# 原理图布局与模块标题

新设计先读 [schematic-data.md](schematic-data.md)，按 [auto-layout-sop.md](auto-layout-sop.md)
生成并验证 Apply。本文件说明布局计算和仍可用于存量图的工具，不要求每张图都跑自动布局。

## 计算依据

- 使用 `sch sheet-geometry --json --doc <page>` 的实测 sheet bbox 与 `keepouts[]`。
  当前图签比例仅标定 A4 横版；`fallback-ratio` 是近似，不能当作精确边界。
  缺少图纸或器件几何时先补测，不扩展坐标到纸外。
- 原理图坐标 y-UP，1 raw = 0.01 inch；器件 anchor 使用 5 raw 网格。
  矩形左上点为 `(minX,maxY)`，高度向下展开。不要混用 PCB 的 mil 单位。
- 从真实 bbox、引脚号和坐标确定朝向。引脚号不等于左右位置，库符号不能共用猜测的旋转表。
  `rotation/mirror` 是设计输入；改变它们时同步计算全部引脚位置。
- 电气连接来自 canonical 图；已有导线只在保留布局或建立执行基线时使用，不能反过来决定目标网络。

## Lib 内部与单页组合

先放核心器件，再按连接引脚所在侧安排电容、电阻和接口。VIN → 核心 → VOUT 的功能流尽量
从左向右，去耦靠近电源脚。外围的连线方向与引脚方向一致；电源通常上引、地通常下引，
若库符号或通道要求其他方向，应以真实几何避让和官方导图为准。

`sch lib-layout` 从连接图、实测引脚和明确网络策略计算核心与外围；输入契约见
[schematic-data.md](schematic-data.md)。它沿真实连接放件，包括串联外围，保持实测姿态。

**`sch compose` 只组合已设计好的模块几何。** 它整体平移模块中的器件、引脚、导线
和标记，不会从任意网表自动推导核心与外围的位置，也不会自动旋转符号或分页。

自编局部计算应保存脚本与参数，从保留的测量源生成目标 JSON，再运行 `compose` 验证；
不要只留下手填坐标的结果。平移需同步 anchor、bbox、全部 pin 及相连线端/标记，
并独立核对稳定 ID、ref、库身份和 pin→net/NC。输出注明哪些是实测、哪些由计算推导。
修改约束后重跑同一转换，避免每次重新编写一次性改图代码。

组合规则：

1. 先将每个模块压成紧凑包络，再按输入功能顺序从纸张左上向右排，行满后回到左侧下一行，
   形成 Z 字阅读顺序。
2. 每框按内部内容紧凑包围，同行仅顶对齐；下一行按上一行最高框推进，不拉高短框。
   有 `sheetBorder` 时虚线笔画距实测图纸内边框至少 10 raw，输出 `usableBounds` 和边界来源；
   缺少时兼容使用纸张 bbox，不能当成红框净距已验证。模块间距固定为 10 raw；
   框内至少 10 raw、标题净距 5 raw，图签 keepout 不占用。
3. 同模块外围用短正交线，减少折点和绕行；GND/VCC 允许多处就近符号。
   `terminals` 可引用已有实测引脚，沿指定方向生成最短合法直桩并用不同长度避让标签。
   找不到直线路径时调整输入位置或朝向，不自动绕长线。
4. 计算和校验同时考虑器件、引脚、导线段、标记本体和文字带；同网导线也不能穿过文字。
   本体 bbox 不含所有外置位号/型号，导图发现的文字避让仍应写回源数据。
5. 装不下时先依据包络诊断调整模块几何；再按功能拆页。多页转换各自提供完整单页子集，
   不得复制后把同位号的原器件留在源页。

字段、完整命令和重建守卫集中在 [schematic-data.md](schematic-data.md)。

## 紧凑方框与标题

1.4 只生成模块方框和标题，不生成独立 Notes 或预留 Notes 带。当前样式是粉色
`#AA00AA`、虚线 `lineType:1`、0.2 inch 标题 `fontSize:20`。

标题在上、下空档中搜索；先最小化框高度，再比较面积，平局优先左对齐、顶部。
能嵌入现有空白就不扩高，放不下才最小扩边。`titleMetrics` 可提供官方实测宽高；
无实测时使用保守估算，输出的 `titleLayout` 保存包络、障碍和净距，用于落地核验。

```bash
easyeda sch frame apply --from frames.json --project <project> --doc <page>
easyeda sch frame check --from frames.json --project <project> --doc <page>
```

`frames.json` 需含 `schemaVersion:1`、`documentId` 和 `frames`；也可用 `--data` 内嵌
到 Apply。矩形与标题锚点都按 y-UP 解释，标题使用 `LEFT_TOP`。只替换该页该 frame ID
登记的图元，匹配时不重复写；用户图形和 zone-draw 的旧框独立保留。结果不明会停止并
保留恢复记录。`check` 按实际文本 bbox 验证净距/边界，估算通过不等于实际标题已通过。

## AMS1117 的四器件验证入口

`sch power-layout` 是一个固定范围的离线规划器：一颗固定输出 AMS1117、一个输入电容、
两个输出电容。它不是通用电源选型或 DC-DC 布局器。

```bash
easyeda sch power-layout --from geometry.json --doc <page-uuid> \
  --core U1 --input-cap C2 --output-caps C1,C3 \
  --out power-plan.json --playbook power-apply.json
```

输入必须是带真实 bbox/pins/rotation/mirror、连接图元清单和权威 pin→net 的 list 快照。
核心需测得 VIN 在左、pin 4 在右、GND 在 pin 2/VIN 下侧；电容依据真实引脚枚举朝向。
局部 VIN/VOUT 直接连接电容，GND 就近下引，重复输出脚按其实际位置连接。

完整 Apply 还要求旧导线 ID；普通 `list --include-wires` 只有线段几何，不能假造 ID
或将缺失清单当成空页。该入口在移除旧线/标记后移动器件，核验全部实测引脚再接线。
`--at x,y` 可指定核心 anchor；默认模块放在左上。`--frames-only --playbook` 用于
电路已匹配时只补框标题，保留器件/导线位置。精确输入和限制以 `sch power-layout --help` 为准。

## 存量布局工具：按需使用

这些命令仍存在，但不是 1.4 compose 的前置步骤。先确定需要保留还是重新生成图面，
不要交替调用多个规划器覆盖同一份目标坐标。

| 场景 | 命令与边界 |
|---|---|
| 整簇刚移 | `sch group-move --group <id> --dx … --dy …`；多个相关子组用 `--groups` 一次移动，携带局部线和标记 |
| 组/区/页整理 | `sch group tidy`、`sch zone tidy/relayout/move`、`sch sheet tidy`；tidy/relayout 默认规划，`--apply` 执行 |
| 已有功能区重新排布 | `sch zone-arrange --json`，A4 范围；`--apply` 会清扫并重建连接，须审阅计划 |
| 按旧 spec 摆放未连线器件 | `sch autolayout --spec layout.json --dry-run`；只移动已存在器件，不创建设计缺件 |
| 未连线散件填空 | `sch autoplace-free --dry-run`；`--designators` 限定器件，`--all` 重排整页，仅移动器件 |
| 局部对齐 | `sch align/distribute` 只动器件；部分覆盖虚拟组会拒绝，不能借此拆散已接线 Lib |
| 清理标记拥挤 | `sch destagger` 默认预览，`--apply` 携带桩线处理，不能只挪 flag |

`group` 是按页面持久化的虚拟组，不是平台原生分组。成员以位号登记，位号迁移通过
`sch designators plan` 同步。`group list`/`zones status` 可查名称、成员和失效引用。

存量 move 内核会快照、清线、移动、重连并对账；它会尝试恢复失败状态，但不具备事务原子性。
检查 `moveReport.partial/stillBroken/FreeConnected`：已连接脚恢复了不代表原几何完全恢复。
`zone-arrange` 的 `blocked` 应区分无落点与搜索预算耗尽，后者并未证明装不下。

`autolayout --engine template --apply` 在规划和写入前都要求活动页 wire/bus/marker 为零、
完整 bbox/pins，并检查快照未变；拒绝 `--all-pages`，`--rewire` 也不能绕过此门。
官方引擎是整页重排的备用能力，需要前台、耗时较长；有连线时必须 `--rewire`，总线拒绝，
重建连接仍可能失败，没有事务回滚。不要把它当作已完成 Lib 的美化按钮。

旧 `zone-plan`/`zone-draw` 使用活体分区，不消费 1.4 frame JSON。需要维护旧框时先
`sch zone-plan --json`，所有验证项通过后 `sch zone-draw`；清旧框用 `--clear`。
`--mode zones` 已失效。旧框的 22 字号/标题带规则不用于新的紧凑 frame。

完成布局后逐页跑 `sch gate --strict`、目标 pin→net/NC 对账与 `sch export-image`。
视觉检查重点是外围是否直接连核心、同排方向是否一致、文字是否压线、方框是否完整包含模块。
