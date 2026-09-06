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
