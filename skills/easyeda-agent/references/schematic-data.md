# 原理图数据与 SCH Apply（1.4）

适用于从本地 JSON 绘图、整理已有原理图、修正位号和验证转换结果。
CLI 参数以 `easyeda sch <command> --help` 为准；电路选型依据具体器件的数据手册，
通过 `lib device get` 的属性可取得 Datasheet 地址。转换器不会推断缺失的外围电路。

## 数据分层

| 对象 | 权威数据与约束 |
|---|---|
| `component` | `id` 为稳定、不透明实例 ID；`ref` 为显示位号；可选 `role` 保存功能名。`device.libraryUuid/deviceUuid` 是器件库身份，不能用 16 位放置实例 UUID 替代 32 位库 UUID。 |
| `pin` | `number` 是完整物理引脚编号，`name` 是符号脚名；明确 NC 用 `noConnected:true`；已确认悬空用 `connectionState:"unconnected"`。保留器件所有物理引脚。 |
| `net` | 稳定 `id`、名称 `name`、可选 `scope/role`。电源与地通常为 `global/power|ground`，信号通常为 `local/signal`；作用域是规划提示，不替代连接记录。 |
| `connections` | 每条为 `{componentId,pinNumber,netId,kind}`；导出使用 `kind:"netlist"`，它不表示导线几何。通过稳定 ID 引用对象。 |
| `modules` | Lib 的 `id/name/coreComponents/peripheralComponents/internalNets/ports`。核心和外围列表引用组件 ID，不引用 primitiveId 或从 ID 截取位号。 |
| 几何 | 器件位置、朝向、实测 bbox/引脚位置、导线、标记、框与文字。允许重算，但不能暗改连接核心。 |

Connectivity JSON 顶层为 `schemaVersion:"1.4"`、`projectId/documentId`、
`components/nets/connections`，可附 `modules/issues`。一个参与绘图的引脚应恰好对应
一个网络、明确 NC 或显式 `connectionState:"unconnected"`，三者互斥。
悬空状态仅在官方快照明确返回 `net:""` 与 `noConnected:false` 时自动导出，
用于原样重建未完成的图，仍保留 `unconnected-pin` 警告，不代表设计通过。
未知、缺失证据需要修复，不能自动填 NC 或悬空。多个同功能脚也要逐脚连接。

实例通过 `otherProperty["EasyEDA Agent Component ID"]` 保留 canonical ID，
`otherProperty["EasyEDA Agent Role"]` 保留可选功能角色。重新导出优先读取绑定；
未绑定旧图兼容用 `cmp-<ref>` 初始化一次。改名之后保留旧 ID，不重新造 ID，
不覆盖原生 `uniqueId`（它可能关联 PCB）。

旧连接器可能把 16 位放置实例的 `device/footprint.uuid` 与 32 位库资产 UUID 直接比较，
导致同名封装全部报 `package-variant mismatch`。`sch list --include-device-identity` 与
Apply 写前/写后回读共用 CLI 兼容解析：用官方 `getDocumentFootprintSources()` 的
FOOTPRINT `DOCHEAD` 和唯一 `META.source` 证明实例封装到库资产的出处。部分版本在
原理图页返回空数组，此时从官方 `getProjectFile(..., 'epro2')` 的当次工程导出中只读提取
同样的出处；前后工程/页面必须一致，源码必须唯一包含当前页面，ZIP 和解压大小受限。
不读取历史备份替代当前状态，也不在编辑器内保存临时工程。随后请求全部
精确 LCSC 候选，并用 `lib_Device.get` 核对型号/稳定名称及关联封装 UUID、库来源。
唯一匹配才恢复器件库身份；输出 `deviceResolution.via=lcsc-footprint-source` 和原连接器错误。
这证明库资产出处，仍不代替引脚、XY、连线及最终图面回读；不把相同封装名作为重建依据。
两个 32 位资产 UUID 冲突、缺原生出处、来源不符、多候选或查询/证据缺失均拒绝，
不能手改快照清除错误。旧连接器的兼容查询只运行固定官方读取脚本；dry-run 不派发
该 debug 查询，也不降低身份门禁。

## 选择转换入口

| 需求 | CLI 与边界 |
|---|---|
| 读取连接图 | `sch connectivity [--page <page> | --all-pages]`；跨页导出逐页激活读取，避免只取得浅层引脚信息。 |
| 本地连接差异 | `sch connectivity-diff before.json after.json`；检查组件/网络增删、连接及 NC 差异，不代替器件库身份与几何校验。 |
| 本地设计版本差异 | `sch design-diff expected.json actual.json --exit-code`；比较 canonical 或完整 compose 计划，输出稳定 ID 差异、修订哈希和证据覆盖范围。 |
| 从实测引脚计算 Lib 内部 | `sch lib-layout --from layout-input.json --out composition.json`；核心与外围的连接图、实测姿态及网络绘制策略 → 局部器件位置/短线/标记，再交给 compose。 |
| 非标准位号修复 | `sch designators allocate` 分配，`plan` 编译原地修改队列，`verify` 执行前后校验。 |
| 完整 Lib 图面 | `sch compose`：完整连接核心与局部几何 → 单页布局与受保护 Apply。 |
| 基础放置 | `sch materialize`：已知库身份和 placement → 放件队列，可选逐脚标记。它不是完整模块绘图器。 |
| 明确的标记增量 | `sch plan before.json after.json`：仅新增指定 kind 的电源/地/网络端口连接；对应脚原为 `unconnected` 时，目标移除此声明并新增连接。其他器件/引脚/NC 变更、删网或重接均拒绝；逐步回读仍核对明确悬空及 NC。 |
| 只画框和标题 | `sch frame apply/check --from frames.json`；字段见 `sch frame --help` 与 [actions.md](actions.md)。 |
| 执行队列 | `sch apply plan.json`，顺序等待 WebSocket 响应并记录 journal。 |

### 本地版本与 EDA 回读对账

先按稳定组件 ID/引脚号匹配同一工程与页，再分别检查拓扑、ref/库身份、placement/bbox/pins。
`connectivity-diff` 返回 `{}` 不涵盖后两类；使用 `design-diff` 补查字段。完整 compose
计划还可比较导线、标记、框与标题；只提供 canonical 数据时这些图形必须列为未验证，
不能把导出中未包含的字段当作“相同”。新命令属于后续源码，原发布版 1.4.2 不含此能力。
`status` 为 `synced` / `different` / `wrong-target` / `incomplete`。
`expectedRevision/actualRevision` 是 `coverage.scope` 内规范化内容的哈希；运行态 primitiveId、
库存数组顺序和整条导线的正反遍历不计差异，模块阅读顺序及实际折线路径会比较。
退出码：0 表示比较执行完；有 `--exit-code` 且内容不同为 2；目标不符或 canonical 证据不完整为 3；
非法输入/读文件失败为 1。新建连接图尚未含 placement/bbox/pin XY 时，即使两份图相同也会返回
`incomplete`（3）；这不等于网表错误，电气不变量可用 `connectivity-diff` 比较，几何须经计算或回读补齐。始终检查 `coverage.unverified`，不能只以退出码 0 声称现场完整同步。

```bash
easyeda sch design-diff target-plan.json observed-connectivity.json --exit-code
easyeda sch design-diff previous-plan.json next-plan.json --exit-code
```

模块局部坐标与整页坐标不同，现场应对照 `compose` 输出的目标坐标，不能直接比较源局部坐标。

保留本地目标和新导出两份文件，记录来源及采集时间。文件较新或连接相同不代表已经同步；
按用户已确认的修改意图确定合并方向，只读比较任务不覆盖任一方。坐标与 bbox 等证据互相
矛盾时先补读；缺少的字段列为未验证，不填默认值制造一致。拓扑、身份、几何、官方导图和
保存状态分别报告，离线比较不能确认导出之后的现场状态。

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

## 由引脚计算 Lib 内部

`sch lib-layout --from layout-input.json --out composition.json` 全程离线，输出直接供
`sch compose` 使用，不生成或派发 EDA 操作。输入：

- `schemaVersion:1`、完整 `connectivity`、实测 `sheet`、明确的 `keepouts` 数组。
  可附 `sheetBorder` 指定实测图纸内边框；它不改变原纸张 bbox。
- `measurements[]` 使用下表 `placements` 的实测格式。必须显式含 x/y、rotation/mirror、
  bbox 四边、完整 pins（含 number/net/x/y）；NC 的 net 为空，朝向固定，不能将未知值填 0。
- `layoutModules[]` 含 `id/title/coreComponentId/netPolicies`，覆盖所有 canonical Lib。
  `coreComponentId` 必须是该模块的核心成员之一；其余核心与外围沿已知电气连接展开。
  `netPolicies` 以**稳定 net ID**为键：`direct`/`module_port` 把该网连成真实线树，
  `local_power`/`local_ground` 为每个独立线树就近放电源/地，已直连的外围不再各放一个标记。

可选 `layoutModules[].peripherals[]` 为 `{componentId,pinNumber?,attachTo:{componentId,pinNumber}}`。
前一个 pinNumber 选择外围脚，attachTo 选择同模块核心或外围的几何参考脚；两脚必须已经同网。
例如两个 VOUT 都存在时可指定右侧那一脚摆电容，另一个 VOUT 仍按网络策略保留连接。
没有提示时按已连接网络选择参考，优先内部信号，再考虑电源；纯地关系不推断功能搭档。
串联支路按依赖顺序放置；环形提示、断开的模块或无法避碰的测量姿态返回具体未解决对象。

核心归零后沿参考脚方向搜索，外围可正对或垂直于参考脚，全部 bbox/pin 随器件平移。
候选保持 5 raw 网格，按连接总长递增，同长度优先直线，再检查正交折点；当前外向搜索至 400 raw、横向至 200 raw。
可选 `maxCandidates` 限制整份输入的搜索次数（默认 20000，范围 1..1000000）；耗尽时明确报错，不写出半成品。
这是有界、保持实测姿态的求解器，失败不证明电路在任意朝向下都无解。改变朝向须重新提供
对应可信几何；不能放宽碰撞检查或修改网表来取得通过。已有手工设计好的 Lib 仍可直接 compose。

```bash
easyeda sch lib-layout --from layout-input.json --out composition.json
easyeda sch compose --from composition.json --out plan.json
# 完成现场取证后，按下文加入 --before/--playbook，最后 Apply。
```

## Lib 组合输入

`compose` 输入顶层使用 `schemaVersion:1`，内含上述 `connectivity`（版本仍是字符串 `"1.4"`）、
实测 `sheet`、`keepouts` 和按阅读顺序排列的几何 `modules`。

| 字段 | 格式 |
|---|---|
| `sheet` | `{minX,minY,maxX,maxY}`，目标纸张实际 bbox。坐标单位 raw = 0.01 inch，y 向上。 |
| `sheetBorder` | 可选同格式 bbox，实际图纸内边框；模块虚线笔画在其内最少留 10 raw，考虑半线宽后向内取 5 raw 网格。缺少时输出 `sheet-bbox-fallback`，不能据此声称已验证红框净距。 |
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

组合器从模块上下空档选择标题位置，再从左上以 Z 字排列。每个框按自己的内容保持紧凑高度，
同行顶齐，下一行按上一行最高框推进；`rowHeights` 为各行推进高度，`rowHeight` 仅是最大值诊断。
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
