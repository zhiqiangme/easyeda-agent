# PR / Issue 排查（2026-09-08）

本轮读取全部 1 个开放 PR、6 个开放 Issue 及评论；下表为首次排查快照。后续实际修复见本文末尾，尚未向 GitHub 发评论、合并或关闭。

| 项目 | 结论及本轮处理 | 后续验收 |
|---|---|---|
| [PR #199](https://github.com/zhoushoujianwork/easyeda-agent/pull/199) | 请求体上限从 1 MiB 放宽到 32 MiB，方向合理；补丁可应用到当前 main，独立临时副本中 `go test ./internal/daemon` 通过。未合并。 | 现有测试只覆盖 2 MiB 可通过；建议补超限拒绝测试，Skill 说明 base64 后整个 JSON 的大小限制。3D 模型真实导入未验证。 |
| [#192](https://github.com/zhoushoujianwork/easyeda-agent/issues/192) | 本地新增 `pcb modify` / `sch modify --patch-file`，兼容 UTF-8 BOM，与 `--patch` 互斥；同步 Skill。 | 命令级模拟 daemon 测试验证 payload 与无效输入不派发；尚未在 Windows PowerShell 5.1 真机执行。发布后可请报告者复验。 |
| [#191](https://github.com/zhoushoujianwork/easyeda-agent/issues/191) | 仓库 `docs/dev-environment.md` 已有 9 月 5 日的 3.2.186 超时实测；官方接口标注 EDA v4 / BETA，参数签名与当前调用一致。已把兼容性说明补进 Skill。 | 保持开放；受支持宿主上探测并核对连接。不能把换 netport/netflag 当原生 label 已修复。 |
| [#190](https://github.com/zhoushoujianwork/easyeda-agent/issues/190) | 报错发生在 `--doc PCB1` 解析阶段，未进入器件移动。已有维护者评论索要版本、成功/失败后的文档枚举，尚无新增答复。 | 等待同一会话的 health / doc ls 对照，不重复发送相同问题。 |
| [#200](https://github.com/zhoushoujianwork/easyeda-agent/issues/200) | 报告为 3.2.149 重启后 PCB getAll 空读；本机为 3.2.186，不能当同版复现。当前打开的是用户原理图工程，本轮未改动页面。 | 需在受影响宿主核对窗口、活动 PCB、文档 UUID 与 getAll / 引擎计数；读数异常时不能把 0 器件算通过或盲写。附带 cmdKey 症状在 `cmd_sch_block_layout_solve.go` 有网表导出影响命令上下文的历史线索，但未证明本票同源；升级 RPC、密集引脚短接需分别复现。 |
| [#173](https://github.com/zhoushoujianwork/easyeda-agent/issues/173) | 原生 UI 编组 API 未暴露的历史调查已有记录；virtual group 是独立功能开发，需要统一约束全部布局入口。 | 保持开放，不能用只实现 group CRUD 宣称完成。 |
| [#43](https://github.com/zhoushoujianwork/easyeda-agent/issues/43) | 芯片级 N8R8 实机验收；历史 R2 评论已纠正 `pcb check` 假绿，记录了 2 条短路，不能按曾有 0 ERROR 数字关闭。 | 专门运行真实编辑器整板回归，解决短路、RF / 高速约束后再验收；本轮未执行。 |

本轮仅修改 CLI 文件输入与 Skill 说明，未改连接器、布局判据或 autosave；离线测试不代表整板端到端验收。

## 补充验证与提交

修复提交：`6c70816`。后续补齐 Unreleased changelog、两个命令的文件输入示例、
文件缺失/显式零值覆盖/PCB center 冲突/空 inline 与文件互斥的命令级测试。

- `TestModifyPatchFile`：16 个用例通过。
- `TestModifyPatchFileBoundaries`：7 个边界用例通过；拒绝路径均确认没有派发动作。
- `TestBuildModifyPatch`：9 个已有合并语义用例通过。
- `go test ./...`：全量通过（Windows 专属测试在 macOS 跳过）。
- `make lint-test`、`make skill-check`：通过。
- Windows amd64 测试程序交叉编译：通过；不代表 Windows 运行通过。
- `TestModifyPatchWindowsPowerShell51`：本机 macOS 无 PowerShell 5.1，明确跳过。
  已接入 CI native-release-smoke；Windows runner 将通过 Set-Content 生成 BOM JSON，
  再实际调用 powershell.exe → easyeda.exe → 模拟 daemon 验证补丁。该 CI 尚未运行。

本次不升级版本、不发布；保留未完成的宿主复现及整板验收事项。

## 深入修复（后续进展）

- **#190 已有本地修复**：`discoverDocs` 在活动 PCB 被总表漏掉时，调用官方
  `pcb.board.info` 恢复名称与 UUID。项目、文档和类型必须与活动上下文一致。
  补读失败、跨项目、切页或名称不符均拒绝；不移除 `--doc`，不猜名称。
  7 个模拟宿主回归场景通过，相关路径 race 测试通过。尚待受影响宿主实测。
- **PR #199 的补丁已纳入本地实现**：保留原贡献者署名，增加 32 MiB 精确边界
  与多 1 字节拒绝测试，补充 Skill 的 base64 膨胀说明。三个请求体测试路径通过；
  实际 3D 模型导入未测，GitHub PR 状态尚未变更。
- **#200 的根因仍未证实**：源码确认 import-changes 的前后计数也来自 getAll，
  不能称为独立引擎计数。已准备 `scripts/diagnostics/pcb-empty-read.js`，一次只读调用
  对比 getAll 无参数/undefined/分层/ID 枚举与文档前后身份，支持定位参数或上下文差异。
  模拟 ID 非零、实例为空的探针验证通过。当前 autoconnect 的同列引脚硬拒绝与
  批次桩线互斥已有实现，对应回归测试通过；仍需问题工程才能解释报告中的短接。
- **实机阻塞**：本轮一度读取现有 PCB 返回 0，因无独立非空证据未判定为复现；
  已恢复原理图活动页。随后两个连接器均断开，health 确认 windows=[]，
  无法继续实机验证或 ceshi 整板回归。已请求恢复回归工程。

深入修复后的 `go test ./...`、文档恢复/补丁文件 `-race` 测试、`make lint-test`、
`make skill-check` 均通过。Windows PowerShell 5.1 仍待 Windows CI，未以交叉编译替代实测。

## ceshi 定点实测（17:14–17:30，UTC+8）

环境：Chrome 网页版 EasyEDA 3.2.186，ceshi 连接器 1.4.2，开发态 CLI/daemon。
这是针对故障的定点探测，不是 ESP32 客户需求到四层 PCB 的端到端验收。

1. 初始 `doc ls` 返回 null、Board 列表为空。**后来证据证明不能据此称工程为空**：
   `createPcb()` 返回 `d77b816f0ea2a04b`，可打开为 PCB3，但总表、当前 PCB 和
   getPcbInfo 都不可读。`board create --pcb` 返回 Board1，Board 总表仍为空。
2. 第一版“当前 PCB 元数据补读”在该状态下正确拒绝，但不能恢复。补充精确 UUID
   路径：document.current 的结果与响应 context 的 UUID、项目和类型一致才通过。
   实机使用 `--doc d77b816f0ea2a04b` 的读取探针成功执行，名称枚举不可用时仍能定位。
3. 同一探针中 `sameDocument:true`，getAll 无参数/undefined/Top/Bottom/ID 枚举
   全为 0，首尾无变化。没有非空图元证据，所以**尚未复现 #200 的“有器件却空读”**。
4. 测试电阻创建失败：`Cannot convert undefined or null to object`；
   测试丝印创建也失败：`无法创建文本图元`。未成功创建用于连续移动的图元，
   因而 #190/#192 的真实移动/文件补丁/保存重开验证均未完成。
5. UI 曾显示 PCB3，随后在重载/工程入口打开时停留开始页、“暂无数据”或白屏，
   连接器却继续上报旧活动 PCB。不能将连接器在线或活动 UUID 等同于文档数据加载就绪。
6. **清理未确认完成**：删除本轮创建的 PCB 时宿主抛
   `Cannot read properties of null (reading 'data')`；后续受项目身份前置检查保护的
   Board 清理也在该错误处停止。未反复重发删除。宿主恢复后只核查/清理上述 PCB3 UUID
   和本轮 create 返回的 Board1，不批量清空其他文档。

原始本地响应：`/tmp/easyeda-issue-live-20260908/`（create-pcb / add-component / silk-add /
probe / cleanup / cleanup-rest JSON）。当前宿主问题阻塞下，保留错误和清理待办，不宣称验收完成。

补充修复的全量 `go test ./...`、文档守卫 `-race`、Skill lint / package check 全部通过。
身份不一致测试覆盖文档 UUID、类型、项目；修复测试 fixture 的 health 路径后全量复跑通过。

## ceshi 恢复复核（23:04–23:08，UTC+8）

- 当前同名工程 UUID 为 `475cc0f773ed4a6fb7a02336c8a6a67f`，与前轮不同；
  PCB1 为 `7dc1c3e4a818108c`。Board1、PCB 总表及当前 PCB 元数据均恢复可读，
  `doc open` 可打开，UI 可读取 PCB1 编辑器结构。旧工程清理待办不能套用到本工程。
- `pcb add-component` 仍报 `Cannot convert undefined or null to object`；
  `silk-add` 仍报无法创建文本图元。执行 `doc reload`（保存、关闭、重开）并将
  Chrome ceshi 标签激活后，再按名称 `--doc PCB1` 放置，仍为同一错误。
- 只读探针身份前后一致，PCB 元数据正常，器件各枚举与线段枚举全部为 0。
  没有成功放置的已知图元，仍不构成 #200 的非空板空读复现，也不能验收连续移动。
- 本轮未成功创建测试器件，不盲目删除；回读器件仍为 0。连接器为 1.4.2，
  CLI/daemon 为开发构建；版本检查提示最新发行版 1.4.3，未将旧连接器视为同版验收。
- 原始响应保留于 `/tmp/easyeda-issue-live-20260908-recovered/`，包含 add、silk、
  probe、add-after-reload。本轮仅记录实机证据，未修改代码、未重跑已通过的离线测试。
