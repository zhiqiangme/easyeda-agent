# Lib 组合与 Apply 现场验证

2026-09-06，宏恩门禁底座载板；EasyEDA Pro 3.2.186、connector 1.4.1。
验证对象是完整电气 IR 与局部几何经过 `sch compose → sch apply → 回读` 的转换契约。
本次不等同于从需求选型到 PCB 的 `ceshi` 整板回归，也不证明保存的业务电路已经完备。

## 页面与数据

| 页面 | Lib | 器件 | 引脚 | pin→net | NC | 统一行高 |
|---|---|---:|---:|---:|---:|---:|
| POWER_RF_MCU | POWER、RF_MCU | 11 | 69 | 34 | 35 | 380 raw |
| TALK_RF_SD | TALK、RF_FRONTEND | 12 | 96 | 50 | 46 | 485 raw |

四页收敛为两页；删除两个旧页前，`sch clear --dry-run --expect-empty` 全图元枚举均为零。
页边、框内最小边距、模块间距和图签净距固定为 10 raw；标题 20 raw、粉色，框同色虚线。
标题按可用空档放在上方或下方。两页共 35 个水平信号端口。

端子直出修订：27 个端子信号声明由 `compose` 按实测引脚和文字占位计算，
pin→端口实线路长从 3665 降至 1335 raw（减少63.6%），折点从40降至0；
两页全部导线及标记引线从7110降至4860 raw。测量按真实线段图切分交点，
不计符号或文字宽度。密集邻脚采用错落线长；GND/VCC保留短折线与上下旗。
原样位号、前导零、大小写及声明顺序由代码与负例测试保障，Lib 登记不再大写化或排序。
官方导图另发现供电/地线接近外置位号及型号；源数据增加引线净距并下移 SD 端子，
重新计算后再 Apply。API 本体 bbox 不覆盖外置位号文字，此边界不能以数据门通过掩盖。

最终 `sch connectivity --all-pages` 与原始电气 IR 对账：23 器件、165 引脚、
84 条连接、81 NC、28 网络 ID 均保持，器件库身份一致；`sch connectivity-diff` 返回 `{}`。

## 验收结果与边界

- 两页器件库 UUID、bbox、引脚坐标、pin→net/NC、实际导线路径及标记方向均回读一致。
- 两页 `layout-lint`、`clusters`、`bridge-check` 通过，无器件或标记重叠。
  第二页 `check` 另报图签 `Name` 为空；本轮 Apply 前快照已经为空，未改写该字段。
- 四个框及标题回读符合数据；官方 PNG 已复核，两页均已显式保存。
- 新回读重新编译、不传 `--replace` 后，每页只生成 11 个验证/登记/框/保存步骤，
  不含清空、放件、修改器件或接线。执行到严格门前的 9 步通过，框报告 `unchanged:2`；
  前后器件、标记和导线数据完全相同。
- **严格 gate 未通过**：官方 DRC 为 0 fatal、0 error、15 warn。SDK 检查覆盖整个原理图，
  切换两个页面均回报这 15 条，不应累加为 30 条。官方面板确认存在单引脚网络警告，
  例如 VBUS_RF、VBUS_TALK、SPK_SD。保存的原始 IR 本身也有 15 个单引脚网络，
  包括 SPI/SD、音频及 VBUS 信号。未擅自补接或转成 NC，也不为位号格式 INFO 改名。
  第二页严格门还保留上述图签必填项警告。
- 严格门失败后队列正确停止，没有执行其末尾的保存步骤；为保留已验证的转换测试结果，
  在回读后另行显式保存。不能把本次转换验证写成原理图已获严格验收或可转 PCB。

现场暴露的问题已沉淀为代码检查：导线逐段穿越标记本体/文字、同符号不同料号、
缺失/额外总线数据、未接线恢复前的实时 summary 变化，以及 create 旋转方向与实测值不一致。
`make test` 与 `make lint-test` 通过；负例验证失败时不会继续写入。

## 本地证据

测试数据保存在 Git ignored 的 `.easyeda/tmp/merge-pages/`，不把工程实例数据发布到仓库。
最新文件：`page{1,2}-straight-input.json`、`page{1,2}-straight-plan.json`、
`page{1,2}-straight-apply.json` 及日志、`page{1,2}-straight-{before,after}.json`、
`page{1,2}-straight-repeat.json` 及日志、`page{1,2}-straight.png`、
`straight-final-connectivity.json`、`straight-final-connectivity-diff.json`、
`straight-live-verification.json`。`report.json`保存新旧线路逐脚量化结果与输入SHA256；
第二页文字避让的最终队列为`page2-straight-lead-apply.json`，旧`readable`数据保留作比较基线。

`validation-result.json` 列出单引脚网络的稳定 net ID 与具体位号/引脚，
用于下一轮先修电气数据，再重新组合和 Apply。命令契约见 [页面组合](schematic-page-composition.md)。
