# 公共复用模块库

公共数据位于 `library/modules/`，随 Skill 发布，也在 GitHub 接受贡献。它保存功能模块的
脱敏证据、复用入口和成熟度；不保存训练题、客户板或参考工程的逐题副本。

## 与 Blocks 的分工

| 层 | 数据 | 用法 |
|---|---|---|
| Part | `references/standard-parts.json` | 确定器件库身份、封装与采购身份 |
| Block Template | CLI 内嵌 `internal/blocks/data/` | 参数化生成角色拓扑、重绑端口并创建新实例 |
| Lib Module | `library/modules/catalog.json` 及其资产 | 保存实例级连接核心、实测局部几何和 compose 输入 |
| Composition | 用户或项目本地 JSON | 选择多个 Lib，安排一页并生成受保护 Apply 队列 |

Block 不是运行时布局层，也没有停用。已有 Block 的公共 Lib 条目用 `blockTemplate` 引用；
不要复制其 `parts/internal_nets/ports`。只有边界稳定、值得参数化生成的通用拓扑才新增 Block。

## 成熟度

- `draft`：只有脱敏功能事实或布局/验证约束；不可据此画电路。
- `topology_ready`：已有独立来源核实的完整角色拓扑和端口，可实例化连接核心；尚不能声称
  具备准确的 EasyEDA 库身份或局部几何。
- `compose_ready`：具备确定库 UUID、全部物理引脚状态、稳定 ID、实测 bbox/pin/姿态和局部
  导线/标记，关联的 compose 输入可通过 CLI 离线校验。
- `verified`：`compose_ready` 资产已在真实页面 Apply、逐脚回读、检查并保存；硬件/PCB 验证仍按
  各自 evidence 独立记录。

成熟度只能前进到证据实际覆盖的位置。`draft` 与 `topology_ready` 不得传给 `sch compose`。

## 脱敏与来源

允许：通用功能名、独立数据手册/官方参考设计、聚合出现次数、不可识别的布局约束、现有 Block
引用。禁止：原始附件、题目名称或编号、完整 BOM、整板逐网拓扑、指定坐标/分值、截图和能够
重新组合出某份原题的映射。若版权或授权不明确，只提交独立重新表达的工程事实，并另找器件
手册验证；不能把“公开仓库”当成转载许可。

## 工作流

1. 查 `catalog.json` 和 `easyeda blocks search`，合并重复功能。
2. 从本地材料只生成临时提取表；提交时删除逐题映射，只保留聚合候选。
3. 补独立来源、端口、器件身份与逐脚拓扑；已有 Block 就引用它。
4. 从官方 API 测量符号姿态、bbox 和全部引脚，以 `sch lib-layout` 生成局部几何。
5. `python3 scripts/modules-audit.py`；compose 资产另跑 `easyeda sch compose --from ...`。
6. 真页 Apply 后回读 pin→net/NC、库身份、几何、框和标题，再提升验证状态。

目录审计只证明公开元数据合同和脱敏字段通过，不证明电气设计、PCB 或硬件正确。
