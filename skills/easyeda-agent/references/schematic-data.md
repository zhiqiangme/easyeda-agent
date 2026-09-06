# 原理图数据与 SCH Apply（1.4）

适用于从本地 JSON 绘图、整理已有原理图、修正位号和验证转换结果。
CLI 参数以 `easyeda sch <command> --help` 为准；电路选型依据具体器件的数据手册，
通过 `lib device get` 的属性可取得 Datasheet 地址。转换器不会推断缺失的外围电路。

## 数据分层

| 对象 | 权威数据与约束 |
|---|---|
| `component` | `id` 为稳定、不透明实例 ID；`ref` 为显示位号；可选 `role` 保存功能名。`device.libraryUuid/deviceUuid` 是器件库身份，不能用 16 位放置实例 UUID 替代 32 位库 UUID。 |
| `pin` | `number` 是完整物理引脚编号，`name` 是符号脚名；明确 NC 用 `noConnected:true`。保留器件所有物理引脚。 |
| `net` | 稳定 `id`、名称 `name`、可选 `scope/role`。电源与地通常为 `global/power|ground`，信号通常为 `local/signal`；作用域是规划提示，不替代连接记录。 |
| `connections` | 每条为 `{componentId,pinNumber,netId,kind}`；导出使用 `kind:"netlist"`，它不表示导线几何。通过稳定 ID 引用对象。 |
| `modules` | Lib 的 `id/name/coreComponents/peripheralComponents/internalNets/ports`。核心和外围列表引用组件 ID，不引用 primitiveId 或从 ID 截取位号。 |
| 几何 | 器件位置、朝向、实测 bbox/引脚位置、导线、标记、框与文字。允许重算，但不能暗改连接核心。 |

Connectivity JSON 顶层为 `schemaVersion:"1.4"`、`projectId/documentId`、
`components/nets/connections`，可附 `modules/issues`。一个参与绘图的引脚应恰好对应
一个网络或明确 NC；未知、缺失证据需要修复，不能自动填 NC。多个同功能脚也要逐脚连接。

实例通过 `otherProperty["EasyEDA Agent Component ID"]` 保留 canonical ID，
`otherProperty["EasyEDA Agent Role"]` 保留可选功能角色。重新导出优先读取绑定；
未绑定旧图兼容用 `cmp-<ref>` 初始化一次。改名之后保留旧 ID，不重新造 ID，
不覆盖原生 `uniqueId`（它可能关联 PCB）。

## 选择转换入口

| 需求 | CLI 与边界 |
|---|---|
| 读取连接图 | `sch connectivity [--page <page> | --all-pages]`；跨页导出逐页激活读取，避免只取得浅层引脚信息。 |
| 本地连接差异 | `sch connectivity-diff before.json after.json`；检查组件/网络增删、连接及 NC 差异，不代替器件库身份与几何校验。 |
| 非标准位号修复 | `sch designators allocate` 分配，`plan` 编译原地修改队列，`verify` 执行前后校验。 |
| 完整 Lib 图面 | `sch compose`：完整连接核心与局部几何 → 单页布局与受保护 Apply。 |
| 基础放置 | `sch materialize`：已知库身份和 placement → 放件队列，可选逐脚标记。它不是完整模块绘图器。 |
| 明确的标记增量 | `sch plan before.json after.json`：仅新增指定 kind 的电源/地/网络端口连接；不支持任意器件更改、删网或重接。 |
| 只画框和标题 | `sch frame apply/check --from frames.json`；字段见 `sch frame --help` 与 [actions.md](actions.md)。 |
| 执行队列 | `sch apply plan.json`，顺序等待 WebSocket 响应并记录 journal。 |

## 位号修复

正常位号使用英文字母前缀加数字。保留已有合法编号的大小写、前导零和声明顺序；
`U_RF`、`J_AUDIO_MOD` 属于功能名，放入 role，不当成原始合法编号继续重放。
官方库默认前缀可能是 `U?`、`CN?` 等，不能假定端子必定是 `J`。

1. 用 `lib device get --uuid <deviceUuid> --library <libraryUuid>` 查询官方记录，保存响应。
2. 将 `result.device.property.designator` 汇成 `prefixes.json`，格式为
   `{"<libraryUuid>/<deviceUuid>":"CN?"}`。不能将库的 `Designator:"CN?"` 占位属性写回已有实例。
3. 在全工程连接图上分配编号，再逐页编译修改计划：

```bash
easyeda sch designators allocate project.json --prefixes prefixes.json \
  --out numbered.json --changes changes.json

easyeda sch list --project <project> --page <page> --stay \
  --include-pins --include-bbox --include-wires --include-device-identity > before.json

easyeda sch designators plan numbered.json --before before.json --out rename.json
easyeda sch apply rename.json --dry-run
easyeda sch apply rename.json --yes
```

分配只改非标准项，按输入顺序跳过全工程已占用数字。plan 固定源文件 SHA，核对库身份和
原始引脚/网络，只对原有实例写 ref 与 ID/role 绑定；不清页、不摆件、不画线。
全工程守卫逐页加载；页面枚举、读取或原页恢复失败时停止，不能把不完整清单当成全量证据。
前后检查保护 primitiveId、uniqueId、属性、引脚/网络/NC、位置和导线，成功后同步本页
组成员及 role 引用并保存。再次生成计划时，已匹配的绑定不再写入。

源 composition 的 `placements[].designator`、`terminals[].designator`、组和规格书中的
ref 引用也要按组件 ID 同步；不要对 JSON 做全局字符串替换，网络名和稳定 ID 不随之改名。
`compose/materialize` 在生成放置队列前拒绝非标准 ref，防止错误源数据写回画布。

## Lib 组合输入

`compose` 输入顶层使用 `schemaVersion:1`，内含上述 `connectivity`（版本仍是字符串 `"1.4"`）、
实测 `sheet`、`keepouts` 和按阅读顺序排列的几何 `modules`。

| 字段 | 格式 |
|---|---|
| `sheet` | `{minX,minY,maxX,maxY}`，目标纸张实际 bbox。坐标单位 raw = 0.01 inch，y 向上。 |
| `keepouts` | bbox 数组，例如图签；从 `sch sheet-geometry --json` 取得，保留其来源与警告。空数组表示已确认没有禁放区。 |
| `modules[]` | `id/title/placements/wires/flags`，可附 `terminals/titleMetrics`；与 connectivity 的 Lib 成员逐项对应，每件只归属一个模块。 |
| `placements[]` | `designator/value/x/y/rotation/mirror/bbox/pins`；bbox 与引脚位置来自官方实测，器件和引脚坐标落在 5 raw 网格。 |
| `pins[]` | 完整 `{number,name,net,x,y}`；NC 的 `net` 为空，与连接核心的 NC 状态一致。 |
| `wires[]` | `{net,points:[[x,y],...]}`，正交、非零、落网格。JSON 网名本身不能给实际线树命名，须连接对应引脚/标记。 |
| `flags[]` | `{net,kind,pinX,pinY,direction,offset}`；kind 为 `power/ground/net_port_in/net_port_out/net_port_bi`，方向 `up/down/left/right`，offset 为正的网格长度。转换生成真实引线。 |
| `terminals[]` | 可选 `{designator,pin,direction,kind?}`，默认 `net_port_bi`；引用本模块已测引脚，选择向外直出方向，不能与已有接线重复。 |
| `titleMetrics` | 可选 `{title,fontSize,width,height}`，须与标题、字高相符的实测值；没有时采用保守预测，Apply 仍检查实际文字边界。 |

局部外围沿核心引脚方向设计，电容/电阻用短线直接连接。电源/地就近重复放置可减少环绕线；
跨模块信号使用网络端口。端子直出按 10～300 raw、5 raw 步进寻找最短合法直线，
邻标签通过错落长短避让。无法直出就修源几何，不自动回退折线。

组合器从模块上下空档选择标题位置，再从左上以 Z 字排列，使用最大模块高度统一行高。
页边、框内最小边距、模块间距及标题内缩固定 10 raw，标题净距 5 raw；
框为粉色 `#AA00AA` 虚线、无填充，标题为 20 raw。本版本无独立 Notes。
它平移器件、引脚和线路，但不推断器件朝向或缩放符号。

## 从计算到现场

```bash
# 离线检查输入并计算布局
easyeda sch compose --from composition.json --out composition-plan.json

# 读取将要操作的已有页
easyeda sch list --project <project> --page <page> --stay \
  --include-pins --include-bbox --include-wires --include-device-identity > before.json

# 已授权重建不同图面时加 --replace；已匹配图面不需要该参数
easyeda sch compose --from composition.json --out composition-plan.json \
  --before before.json --replace --playbook apply.json
easyeda sch apply apply.json --dry-run
easyeda sch apply apply.json --yes
```

生成队列已固定工程与页 UUID；执行时沿用其目标，不用名称覆盖 `--project/--doc`。
目标已有图面与计划不同且未加 `--replace` 时拒绝生成队列。
完全匹配时保留电路，仅执行验证、组登记、模块框与保存；几何匹配但未接线时，
只有明确的空导线/空标记/无冲突 NC 证据才能复用器件接线。其他重建路径清目标页、
保留纸张，再枚举全部图元确认无残余。

新建器件先零旋转 place，再绝对 modify 写入实测朝向、位置及 canonical 绑定。
当前平台 create 与存储旋转存在符号差异，不应直接将测量角度传给 create。
回读全部引脚几何通过后才恢复 NC、导线和标记；最终核对 pin→net/NC、线树、模块框，
通过 `sch gate --strict` 后显式保存。`buses/shortSymbols` 计数缺失或非零会阻止图面匹配。

所有页使用各自的完整器件子集、不同 `documentId`，跨页位号不重复；组合器不自动创建、
合并或删除页面。默认先做一页，放不下时根据功能及用户已有授权拆页。
跨页迁入前还需安排源页处置：源页若仍有相同 ref，全工程位号守卫会拒绝目标重建。
先保存完整源数据并列出器件的目标页及源页处置步骤，再在已授权范围内执行迁移；
当前没有跨页迁移事务，不能只执行目标 compose，或为消除冲突盲清其余页面。

## 验证与恢复

- `sch apply --dry-run` 只校验队列，不证明现场状态与电路正确。
- `requireFullExecution:true` 或含连接守卫的队列须完整执行，不能续跑/跳步或更换目标。
  前检失败、写入超时或部分成功后，保留日志、读取实际状态，再生成新队列。
- Apply 不提供事务回滚。`checkpoint` 只是日志标记，只有 save 动作才保存。
- `sch connectivity-diff` 以稳定 ID 对账；位号修复还须核对 ref 映射，重新布局还须核对
  库 UUID、完整引脚、真实导线和框。DRC 单项结果不替代这些检查。
- 官方器件 bbox 不完整包含外置位号/型号文字；用官方 `sch export-image` 复核可读性，
  将必要净距写回源数据。短位号可能缩小文字预留并改变重新 compose 的紧凑排版；
  单独修位号不应附带重排。
- 报告未解决的 WARN、未知数据与未运行的验证。失败队列不会自动执行末尾 save；
  如需保留已核实的局部成果，先回读确认后另行保存，不把局部完成称为整板通过。
