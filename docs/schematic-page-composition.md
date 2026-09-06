# 单页 Lib 组合与 SCH Apply

`easyeda sch compose` 把完整的 1.4 电气模型和各 Lib 已设计的局部几何合成一页原理图，
输出布局 JSON，并可编译顺序执行的 `sch apply` 队列。它负责模块组合和数据转换；
核心器件与外围电路的连接、局部摆放及朝向必须已在输入中确定。

## 输入契约

```text
schemaVersion: 1
connectivity: 完整的 1.4 IR
sheet: {minX, minY, maxX, maxY}
keepouts: [{minX, minY, maxX, maxY}, ...]
modules: [
  {id, title, titleMetrics?, placements, wires, flags}, ...
]
```

| 字段 | 约束 |
|---|---|
| `connectivity` | 必须有目标 `projectId/documentId`、稳定器件/网络 ID、全部物理引脚及 `connections`。每个引脚恰好有一个网络或明确的 `noConnected:true`；不能用 NC 掩盖缺失的连接数据。 |
| `components[].device` | `libraryUuid/deviceUuid` 必须来自已解析的器件库身份。放置实例 ID 不能当库 UUID 重放。 |
| `connectivity.modules` | 用 `coreComponents/peripheralComponents` 引用器件 ID；与几何模块成员逐项对应，每件只归属一个模块。 |
| `sheet` | 目标页实际纸张 bbox；Apply 前再次核对。坐标单位为 0.01 inch，y 向上。 |
| `keepouts` | 纸内禁止占用的区域，例如图签。记录其来源和可见状态；空数组表示确实没有禁放区域。 |
| `modules` | 数组顺序就是功能阅读顺序，不按旧页面或旧 XY 排序。 |
| `placements[]` | `designator/value/x/y/rotation/mirror/bbox/pins`。器件与引脚坐标落在 5 raw 网格；bbox 使用官方实测几何。`pins` 为完整的 `{number,name,net,x,y}` 数组，NC 引脚的 `net` 为空。旧 `primitiveId` 不作为新实例身份。 |
| `wires[]` | `{net,points:[[x,y],...]}`；正交折线会拆成单段。网名用于本地校验，实际线树由相连的电源符号或网络端口命名。 |
| `flags[]` | `{net,kind,pinX,pinY,direction,offset}`；`kind` 使用 `power/ground/net_port_in/net_port_out/net_port_bi`，方向为 `up/down/left/right`，偏移为正的网格长度。转换会生成真实引线。 |
| `titleMetrics` | 可选 `{title,fontSize,width,height}`，仅接受同标题、同字高的实测数据。没有实测时使用保守字宽预测，Apply 仍检查实际文字 bbox。 |

连接模型见 [概念约定](concepts.md)，框与标题字段见
[模块框转换](schematic-frame-conversion.md)。局部电路应先用真实引脚几何设计：
外围电容、电阻直接连接核心器件，电源/地就近放标记，边界信号可用网络端口。
每个有网络的引脚都必须经真实线段到达同名标记；在 JSON 导线中填写网名本身不构成连接。

## 排版与固定贴边尺寸

页面边距、框内最小边距、模块间距、行间距、标题内缩及图签净距均为
**10 raw = 0.1 inch = 2.54 mm**；标题与内容净距为 5 raw。
这些尺寸由共享常量定义，当前 CLI 不提供逐模块调整参数。

先在模块上方、下方寻找标题空档，选择合法且总高度较小的包络；标题保持
20 raw（0.2 inch），粉色 `#AA00AA`，外框同色、虚线、无填充。再从左上按 Z 字排列，
右侧放不下才换行；所有行和同行框采用最大模块高度。平移同时作用于器件、引脚、
导线、标记与标题，不改变器件朝向及任何电气连接。网格取整与统一行高可以增加留白。

图签应通过 `sch sheet-geometry --json` 获取，并保留 `source/warnings`。
运行时规则在 `internal/app/cmd_sch_sheet.go`，Skill 的 `sheet-templates.json` 是镜像。
1170 × 825 的 A4 横向图纸、显示默认图签时，当前校准区域为
`{minX:468,minY:0,maxX:1170,maxY:198}`。这是基于实测纸张和已校准模板的推导，
不能把它称为独立图签图元的 API 实测 bbox，也不能把旧的 0.22 × 0.14 比例用于验收。

## CLI 闭环

准备输入后，布局和队列生成均在本地执行：

```bash
easyeda sch compose --from composition.json --out composition-plan.json

# 读取目标页最新几何、全部引脚和导线；include-wires 同时读取 connectivitySummary。
easyeda sch list --project <project-uuid> --page <page-uuid> --stay \
  --include-pins --include-bbox --include-wires --include-device-identity > before.json

# 已批准重建目标页时，编译包含前置状态检查的队列。
easyeda sch compose --from composition.json --out composition-plan.json \
  --before before.json --replace --playbook composition-apply.json
easyeda sch apply composition-apply.json --dry-run
easyeda sch apply composition-apply.json --yes

# 使用新回读重新编译，可验证重复执行是否已无需重建。
easyeda sch list --project <project-uuid> --page <page-uuid> --stay \
  --include-pins --include-bbox --include-wires --include-device-identity > after.json
easyeda sch compose --from composition.json --out composition-plan.json \
  --before after.json --playbook verify-apply.json
easyeda sch apply verify-apply.json --yes
```

`--replace` 允许为不同的目标状态编译恢复/重建队列，执行发生在 `sch apply`。
队列先核对项目/页面、工程内位号唯一性和目标旧状态；新增位号通过 `absentParts`
检查其他页也未占用。根据新鲜回读选择以下路径：

| 目标状态 | 执行方式 |
|---|---|
| 器件、引脚、net/NC、导线路径和标记完全匹配 | 保留现有电路，继续模块框及最终验收。仅同网但路径不同不算匹配。 |
| 器件及全部引脚几何匹配，尚无接线（`reuseUnwired`） | 保留器件，复核并保存放置检查点后恢复 NC、导线和标记。 |
| 其他状态 | 清除目标页并保留纸张；检查全部图元无残余，再重新放置和接线。 |

`reuseUnwired` 必须有明确的空导线、空标记和 `connectivitySummary` 证据：
`scope:activePage`、`wires:0`、`buses:0`，已有 NC 也不能与目标冲突。
执行时重新读取并断言这些计数，缺失或变化即停止，不能仅凭编译时快照继续接线。

重新放置按库 UUID 以零旋转创建，再用绝对 `component.modify` 写入最终位置与朝向；
当前 EasyEDA 的 create 会把 90°/270°反向存储，不能将测量角度直接当 create 参数。
画线之前逐项回读 bbox、朝向与全部引脚位置，通过并保存后才恢复 NC、导线和标记。
所有路径均按 Lib 登记幂等虚拟组、应用/核对模块框，检查完整 pin→net/NC、
导线路径与标记、电气及线树，再通过 `sch gate --strict` 后保存。
框/标题的幂等处理及响应丢失恢复规则见
[模块框转换](schematic-frame-conversion.md)。

## 拒绝条件与失败恢复

最终绘图核对还要求当前页 `connectivitySummary` 中 `buses/shortSymbols` 均为零，缺失计数视为未知，拒绝匹配。

规划器复用运行时的标记本体与文字带模型，检查标记之间及标记与器件的重叠，
并逐段检查普通导线和标记引线是否穿过标记本体或文字；同网也不能穿字。
合法引线可以终止在标记锚点，不用折线的整体包络代替真实线段判断。

缺失引脚、未知 NC、错误网络、未命名线树、孤立导线/标记、触及异网或 NC 引脚、
穿越器件、重复或错误模块成员、无法容纳于单页、压住图签都会阻断规划。
局部坐标可以来自不同页面、甚至处于目标纸张之外；最终组合必须全部落在可用纸内。

生成队列使用 `requireFullExecution`，写操作不自动重试。失败后先保存日志并回读实际状态：

1. **规划失败**：修改局部几何、补齐缺失数据或纠正网络；重新运行 `compose`。不缩小图签区域绕过报错。
2. **清空前状态失配**：重新获取 `before.json`，确认目标与源页现状后重新编译。
3. **放置后几何失配**：检查库身份、符号变体、旋转/镜像和实测引脚；修正输入后从新鲜快照重建。
4. **写入中断或响应不明**：先回读已落地内容，再编译完整队列；不能用 `--resume/--from/--to` 跳过校验，也不能更换队列目标页面。

单页容纳不足时，可按功能拆成两页：分别准备完整的单页器件子集及不同的目标
`documentId`，各运行一次 `compose`。命令每次只组合一页，不自动分页或删除源页。
其他页可保留不同位号的器件；同位号跨页重复时前置检查会停止。旧页应由迁移流程
在数据保全和器件归属确认后处理；不能将复制后留有重复器件视为合页完成。
命令也不推断缺失外围电路、不缩放符号，不以框包络检查代替实际文字可读性检查。
软件验收应同时保存离线计划、Apply 日志、完整回读及官方导图；离线通过不能写成现场闭环已通过。
