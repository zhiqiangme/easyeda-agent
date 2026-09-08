# PR / Issue 排查（2026-09-08）

本轮读取全部 1 个开放 PR、6 个开放 Issue 及评论；以下为本地处理结果，未向 GitHub 发评论、合并或关闭。

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
