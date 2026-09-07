# 1.4 发布与验证

版本 **v1.4.2**。v1.4.1 已用于公开预览，不能覆盖；本次统一 CLI 构建标记、
连接器 manifest/npm 元数据及 Skill 到 1.4.2。变更内容见
[Changelog](../extension/CHANGELOG.md)。发布状态与下载以
[GitHub Release](https://github.com/zhoushoujianwork/easyeda-agent/releases/tag/v1.4.2) 为准。

## 本地检查与构建

```bash
make test
make lint-test
npm --prefix extension test
make release-script-test
make release-check VERSION=v1.4.2
make release-build VERSION=v1.4.2
```

`release-build` 在 `dist/` 生成五平台 CLI、连接器 `.eext`、`skills.tar.gz`、安装器
及八项资产的 `checksums.txt`，核对版本、UUID、校验和和本机 CLI。
它不会安装候选二进制、修改版本、提交代码、创建 tag 或上传资产。

Skill 只打包 Git 已跟踪/已暂存文件的当前内容；新增公共参考需要先暂存。
本地草稿不会入包。`make skill-check` 检查链接在独立安装目录内仍然有效。
GitHub、ClawHub 和 SkillHub 使用同一份受控文件清单。

后续换版本需显式同步 `extension/extension.json`、`extension/package.json`、
`extension/package-lock.json` 的根版本及 `packages[""].version`，
并用 `python3 scripts/sync-skill-version.py <x.y.z>` 同步 Skill，补对应 Changelog。
`release-check` 会拒绝任一版本漂移；发布时不再自动改版本或暗中提交。

完成候选验收、提交所需源码并获得发布指令后，执行
`make release VERSION=v1.4.2`。该命令才会创建 tag、推送并发布；已存在 tag 会被拒绝。
连接器使用稳定 UUID，安装步骤见
[环境说明](../skills/easyeda-agent/references/environment-setup.md)。

## 验证范围

2026-09-06 本地准备检查通过：Go 全量测试、202 项连接器测试、18 项发布/安装态
脚本测试、lint 回归、Agent Skills 规范及包内链接检查。五平台构建、连接器版本/UUID、
本机 CLI 与八项资产校验和通过；包内 56 个文件逐字节匹配公共源码。

隔离解压后，辅助脚本成功读取 37 个内嵌块、128 种器件的引脚引用并与仓库源数据一致；
随包 lint 用例通过，可选的仓库 TS 源码比对明确跳过。两种 Skill 路由推演覆盖原地改号
及跨页合并，不算现场 Apply。入口 223→90 行，公共 Markdown 528,097→314,232 字节，
减少 40.5%；统计不含用户本地草稿。当前产物位于 `dist/`。

离线验证覆盖 Skill 路由/包内依赖、CLI 合同、连接图/几何转换、位号守卫、
连接器单元测试和发布资产。现场证据见[组合验证](schematic-composition-validation.md)：
两页转换与随后独立的位号修复分别完成回读，23 器件、165 引脚、84 连接、81 NC 保持。

尚存边界：宏恩工程有 15 条既有网络 WARN；官方 DRC 可能只给全工程聚合数。
完整 compose 可因短位号收紧包络而重新排版，不能把原地改号验证当成重排验证。
本轮没有重跑 `ceshi` 从原始需求到 PCB 的整板验收，也没有在编辑器加载 1.4.2 候选包；
这些未执行项不记为通过。发布准备完成不表示整板功能或制造交付已经验收。

## 分支与合并

发布前合入 #197（封装匹配）与 #198（铜层计数/叠层保护），并补充封装身份缓存、
层数证据缺失及预览一致性的回归。保留贡献提交与 PR 历史。
四个旧本地分支及 `origin/feat/sch-zoned-layout-opt` 的提交均已在 main，清理其引用；
含未提交文件的旧 1.3.1 临时工作区保留。新增 GitHub CI 执行 CLI、连接器和 Skill 检查。

## 2026-09-07 安装与终端复验

使用从 GitHub Release 重新下载的 v1.4.2 原包，八项资产 SHA-256 全部匹配。
Skill 包含 56 个文件、79 个有效本地链接，连接器 manifest 为 1.4.2。

| 验证 | 结果与边界 |
|---|---|
| macOS ARM64 | 原包在空工作目录执行 15 项 CLI 离线检查通过 |
| macOS Intel 包 | 在 Apple Silicon 兼容层执行同样 15 项检查通过，未在 Intel 硬件运行 |
| bash、zsh 新终端 | 隔离安装后 PATH 解析、版本和 compose 帮助通过；安装下载用原包本地回放 |
| Linux amd64/arm64、Windows amd64 | 发布资产完整性通过，尚未在这些系统实际运行 |
| Skill | 包内路径、版本、辅助脚本执行权限通过；未测 AI 客户端实际发现/加载界面 |
| 原理图转换 | compose 正例、确定性、稳定 ID/位号/网络/NC 保留和错误输入拒绝通过；未调用 EDA |

三个 subagent 分别完成安装链反证、跨平台 smoke 和独立代码审查。原版可复现：
自定义客户端目录被忽略、升级失败返回成功、旧 Skill 文件残留、preserve 模式冒报新版本、
daemon 启动将 Skill 升到与自身不同的 latest。源码已补充安装前校验、目录切换与失败回滚、
版本绑定、精确版本匹配和非零失败；CLI 与不同客户端仍逐项更新，不是整体事务。
这些修复尚未发布，下载原 v1.4.2 不会自动获得它们。

可重复验证入口：

```bash
make release-smoke VERSION=v1.4.2 DIST=/absolute/path/to/downloaded-assets
make release-script-test
make test
make lint-test
make skill-check
```

本地通过 40 项发布/安装脚本测试、Go 全量测试、selfupdate race 检查和 lint 规则回归。
另从已提交源码独立编译 `v1.4.2-install-validation` 候选 CLI，联网下载正式 Skill 包，
在两个自定义客户端目录完成升级、删除旧文件和再次版本检查；错误客户端返回退出码 1。
原生可执行文件的下载、精确版本核验和临时文件替换专项测试在 macOS 通过。
新增 Ubuntu/macOS/Windows 原生 CLI smoke 及下载加载 CI；未推送执行的 CI 不计为通过。
详细本地证据位于 Git 忽略目录 `tmp/install-validation-v1.4.2/`。

## 2026-09-07 Agent 操作行为验证

验证对象是 Agent 阅读 Skill 后实际选择的命令和产物，不以安装成功或流程复述代替。
三个未继承本次开发对话的 subagent 只读取正式 v1.4.2 Skill 和真实发布 CLI，
分别完成以下离线任务；命令包装器记录调用并阻止现场写入。测量输入来自历史样本，
工程/页身份已隔离；版本差异样本为人工注入，不冒充现场导出。

| 输入任务 | 实际行为与结论 |
|---|---|
| `esp32MiniRequire.md` 第一节原始需求 | 查询 7 个电路块并准备候选设计；识别块内已有选型失败记录。官方库查询因离线限制未完成，没有补造完整引脚、NC、测量或 Apply。只验证需求准备行为，未验证新设计绘制完成。 |
| 已有 POWER 模块紧凑布局 | 先修改本地几何，再真实执行 compose；保留 4 器件、10 引脚网络、正常位号及库身份。显式线长 320→225 raw；含标记引线为 520→425 raw。没有新鲜页快照时不生成现场队列。 |
| 本地与导出数据版本核对 | connectivity-diff 返回 `{}` 后补查字段，发现 C3→C30、器件/两脚 x 各增加 25 raw，以及 bbox 未同步的证据疑点；没有误报同步或覆盖任一输入。 |

本次暴露的配套缺口是局部计算的复现性：初次布局留下了目标 JSON，但没有保存可重跑
计算脚本。源码 Skill 已明确保留测量源、脚本和参数，区分 Lib 内部设计与 compose 的
整页组合，并将位号分配明确为缺失/非法编号时的条件步骤。版本核对说明补充了逐域
证据和同步方向。这些说明调整尚未发布，不属于下载原 v1.4.2 时已有的改动。

复测时让独立 Agent 只读候选 Skill，接收原始模块和模拟空白页快照，产出本地设计、
计算脚本、队列及 dry-run 证据；不提供上轮坐标或报告。完整本地证据保存在
`tmp/agent-workflow-eval-v1.4.2/` 的 `new-design/`、`power-edit/`、`version-review/`
和 `candidate-power/`。重跑应冻结 Skill/CLI 版本，保留输入哈希、所读参考、CLI 输出
与退出码，由另一审查者按 ID/ref/引脚/网络、几何及证据范围核查产物；不要将评分答案
交给执行 Agent。来源缺失、工具缺功能和行为偏离分别记录。

候选 Skill 复测已完成：产出 `recompute.py`、参数和重跑入口，编译 35 步受保护队列，
正式 CLI dry-run 通过；含标记引线总长 520→415 raw。父级独立重跑两次，目标与计划
哈希一致；增大一个输出电容间距后重新 compose 成功，连接数据不变。脚本仍专用于
本次模块，不能称为通用布局求解器。另一审查 Agent 对初次 POWER 产物完成 35 项
离线检查与正式 CLI 重算，未发现电气/身份不变量或几何同步错误。包内链接与官方
`skills-ref` 验证通过；这些检查不代替 Agent 行为证据或现场验收。

行为样本说明 Agent 能找到并使用数据路径；不能推断所有模型或客户端必然遵循。
1.4.2 的通用 Lib 内部位置/布线仍需 Agent 编制本地计算，完整设计 diff 仍需补充字段比较。
本轮没有执行真实 EDA Apply、回读、DRC、官方导图与保存，亦未完成 `ceshi` 整板端到端验收。
