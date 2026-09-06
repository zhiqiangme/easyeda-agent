# EasyEDA 原理图

1.4 的工作对象是器件、引脚、网络和连接组成的数据图。先把电路和 Lib 的局部几何设计准确，
再编译为顺序执行的 SCH Apply。模块内部的核心器件与外围用短导线连成完整电路；电源、地及
跨模块信号按需要使用局部标记。截图用于最后的视觉复核。

按任务读取：

| 任务 | 参考 |
|---|---|
| IR、位号、Lib 合页与转换边界 | [schematic-data.md](schematic-data.md) |
| XY、引脚方向、紧凑标题、存量布局工具 | [schematic-placement.md](schematic-placement.md) |
| 计算 → Apply → 验证的操作顺序 | [auto-layout-sop.md](auto-layout-sop.md) |
| 局部接线、端口与断连 | [schematic-wiring.md](schematic-wiring.md) |
| 手写 Apply 或需要了解 action 返回值 | [actions.md](actions.md) |
| 原理图到 PCB 的整板阶段 | [design-flow.md](design-flow.md)、[pcb.md](pcb.md) |

## 目标与快照

```bash
easyeda health
easyeda doc ls --project <project> --json
easyeda sch connectivity --all-pages --project <project> > project-connectivity.json
easyeda sch list --project <project> --doc <page-uuid> \
  --include-device-identity --include-pins --include-bbox --include-wires > page-before.json
easyeda sch sheet-geometry --project <project> --doc <page-uuid> --json
```

- 每次读写绑定 `--project` 与 `--doc`，同名页使用 UUID。`--doc` 会确认并激活目标页；
  `windowId` 会随连接器重连改变，通常按工程路由更稳。
- `sch connectivity --all-pages` 逐页激活并恢复起始页。普通 `sch list --all-pages`
  可能只得到已加载页面的浅数据，不能用空 pins 证明无引脚或 NC；精确几何须逐页读取。
- `sch read` 是语义检查快照；`sch list` 加上述选项才是 compose 所需的完整实测基线。
  两者直接输出 JSON，没有 `--json` 参数。`--include-wires` 同时带活动页连接图元计数。
- 放置需要官方器件库 UUID；实例的 `primitiveId` 是修改句柄，`uniqueId` 是 sch↔PCB
  关联键，二者均不能当库 UUID。无法解析 device identity、pins 或网络时先补数据，不能猜测。
- 临时 JSON、计划和回读结果放入项目已忽略的临时目录。保留原始快照，在副本中设计目标。

## 器件与典型电路

先查 [standard-parts.json](standard-parts.json) 和 `easyeda blocks show <id>`。
已知 C 号用 `easyeda lib by-lcsc --lcsc C…` 精确解析；搜索结果仍须核对型号、封装与 C 号。
从官方器件记录的 Datasheet 地址查典型应用、引脚和外围参数，不凭外观或记忆补接线。
新选型可用 [parts-add.py](../scripts/parts-add.py) 写回标准器件库。

`sch resolve-lcsc` 与换件前的旧器件解析使用同一封装身份规则：双方有封装 UUID 时精确
匹配，双方同时提供封装库 UUID 时也须一致；缺失 UUID 才按去首尾空白、忽略大小写的
封装名称回退。只有 UUID、没有名称的实例仍要过滤封装。兼容库记录的嵌套 `footprint`
及旧版 `footprintName/footprintUuid` 字段。批量解析仅对完整查询条件相同的实例复用结果，
包括型号、项目库器件名、封装 UUID/库 UUID/名称，不能因型号与封装同名就跨身份复用。

`sch block-apply` 仍适用于已验证的电路块，支持相对关系模板和存量绝对偏移模板；它会读取
真实引脚、验证几何和连通，并登记功能子组。模板的功能引脚名必须与实际符号引脚表对应。
`compose` 消费已经设计好的 Lib 几何，不能代替任意器件的典型电路设计。

正常位号使用字母前缀加数字。库的 `U?` 等提供分配前缀，功能名称存 `role`。
历史非标准位号通过 `sch designators allocate/plan` 修复，稳定组件 ID 保持不变；
完整规则见 [schematic-data.md](schematic-data.md)。

<a id="easyeda-electrical-rules-load-bearing"></a>

## 必须保留的电气规则

- 器件引脚与 flag 重叠不构成连接。用“pin → 非零导线 → flag/port”；零长导线会被忽略。
- 同模块相邻外围优先真实正交线，尤其 VIN/VOUT 与电容。不要把连续外围拆成满页同名标签。
- GND/VCC 可就近重复放符号，减少环绕母线；普通信号用 netport，不伪装成电源 flag。
- 导线不能经过无关引脚或异网线。多脚连接应形成有实际锚点的连续线树，不依赖未验证的空中结点。
- 显式 NC 表示设计上不用的引脚。缺少网络信息不是 NC，也不能用删除器件引脚消除告警。
- `sch autoconnect` 会识别已连目标网；`sch connect` 不幂等，重发可能叠加导线和标记。
- `sch disconnect` 可能影响共享线树；返回的 `alsoDisconnectedPins[]` 必须逐个恢复。
  `partial`、`survivedIds`、`notApplied` 表示删除未完全生效，不能直接当作断开成功。
- 删除器件默认清理独占桩线/标记，保留其他器件共用的线树。与该器件无关的残线需另行识别。
  `bridge-check` 的 `orphan-tree` 才能发现“flag 连着线、整棵线树不接任何器件”的残留。

## 修改与恢复边界

1.4 重建优先修改源数据并重新 compose。已有连线的小范围移动用 `sch group-move` 等
携带连接的工具；单独 `sch modify`、`align`、`distribute` 只动器件，不能视为带线移动。
换型号/符号/封装会重建实例，应重新取 primitive ID，检查 `pinDiff` 并验证网络。

`sch modify` 的 `otherProperty` 与兼容别名 `customAttributes` 二选一。连接器合并保留
现有属性，并回读检查；`partial`/非空 `notApplied` 是失败，`verified:false` 是未经确认。
回放 `propertiesBefore` 只能恢复覆盖值，不能靠 merge 删除本次新增的键。不要把库记录的
`Designator`、`Unique ID` 等投影字段整包写入实例。

清页先看 `sch clear --dry-run`，只在已授权的重建范围内执行；默认保留 sheet。
清后用 `sch clear --dry-run --expect-empty` 核对所有非保留图元，读取失败不是空页。
新 frame 和旧 zone-draw 分别拥有自己的图元，清旧标注用其对应命令，不按类型删除用户图形。

放置或修改超时不代表未落地。先按新鲜快照核实；`ACTION_ABANDONED` 或无法确认的回读
不得触发盲重试。`QUEUE_OVERFLOW` 表示该请求未执行。队列序号只证明 handler 顺序，不能
证明文档已保存。恢复后仍有 `partial`/`stillBroken` 就报告残余状态，再从实际数据规划。

变更先在 EasyEDA 内存中生效。daemon 的防抖 autosave 是兜底；通过阶段验证后仍显式
`sch save` 并确认 `saved:true`。若数据读回互相矛盾，先保存和检查连接器状态，必要时
`doc reload` 后再 `doc switch <uuid>`，重新取基线，不依据可疑读数删除电路。

## 验证与交付

- 每页运行 `sch gate --strict`：依次 layout-lint → check → bridge-check → SDK DRC。
  `pass` 才是通过；`fail` 表示电路有阻塞项；`blocked` 表示检查未完成，先修环境。
- gate 不能证明设计意图。另将实际 connectivity 的组件 ID、pin→net 与 NC 对照目标图。
- `layout-lint --strict` 要求完整的单页本体、引脚和图纸几何，不能和 `--all-pages`
  合用。它不覆盖所有外置位号/型号文字；marker 检查和官方导图仍有必要。
- `sch check --json` 的逐条问题在 `result.findings`。SDK DRC 可能只返回布尔/聚合值，
  不能单凭它宣称官方 UI 所有警告已清除；跳过的 gate 阶段仍需补验。
- 用 `sch export-image` 导整页或指定 `--ids`；这是文档渲染，不依赖前台视口刷新。
  产物路径以响应 `artifacts[].path` 为准。BOM 和网表另用 `bom export`、`sch netlist`。

## 接口不足时

先查 `easyeda <command> --help` 和 `easyeda actions`。确无现成能力时，
`easyeda api search <query>` 可离线查询官方 API 索引；优先组合现有 typed actions。
必要的 `debug.exec_js` 只用于任务范围内的临时探测，输出须可 JSON 序列化。
重复使用的操作应落实为 typed action 与 CLI，再同步 Skill。网表读取用
`sch_ManufactureData.getNetlistFile()`，不要使用已废弃且可能挂起的 `sch_Netlist.getNetlist()`。
