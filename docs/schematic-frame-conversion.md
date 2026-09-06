# 模块框与标题的数据转换

1.4 将模块框和标题纳入本地布局 JSON。电气模型保持不变，绘图适配器负责把坐标、
单位与样式转换为官方 API 参数，再以回读结果验证转换是否正确。独立 Notes 功能
退出本版；模块标题保留。

## 数据契约

```json
{
  "schemaVersion": 1,
  "documentId": "<page-uuid>",
  "frames": [{
    "id": "POWER",
    "title": "POWER / AMS1117-3.3",
    "rect": {"minX": 100, "minY": 510, "maxX": 555, "maxY": 780},
    "titleX": 110,
    "titleY": 770,
    "fontSize": 20,
    "color": "#AA00AA",
    "lineType": 1
  }]
}
```

- 坐标为 0.01 inch，y 向上；0.2 inch 标题对应 `fontSize=20`。
- `id` 是页面内稳定的模块标识。`rect` 是几何包围盒；官方矩形起点转换为
  `(minX,maxY)`，宽高分别为 `maxX-minX` 与 `maxY-minY`。
- 标题以左上角对齐，向右、向下展开。默认粉色 `#AA00AA`，框同色、无填充，
  `lineType=1` 是官方 `DASHED`。
- 规划器需纳入器件本体、引脚、导线、电源符号及文字留白，再向外取整到网格。
  文字留白估算必须与实际渲染回读区分；越出纸张时拒绝计划，不自动分页。
- 模块按功能顺序从左上角向右排，行满进入下一行；所有行采用最大模块高度，
  框的上下边对齐。计算出的位移同时作用于模块内所有器件、引脚、导线及标记。
  `power-layout` 默认使用左上起排；`--at` 明确指定核心坐标时保留该位置，
  `--frames-only` 使用现有位置。五模块双行规划已有离线回归，尚不代表多模块完整 Apply。

## CLI 与 Apply

```bash
easyeda sch frame check --from plan.json --project <project>
easyeda sch frame apply --from plan.json --project <project>

easyeda sch power-layout --from geometry.json --out plan.json --playbook apply.json
easyeda sch apply apply.json --dry-run
easyeda sch apply apply.json --yes

# 电路位置和网表已正确，只补呈现层；不生成移件、删线或重连操作
easyeda sch power-layout --from geometry.json --out plan.json \
  --frames-only --playbook frames-apply.json
easyeda sch apply frames-apply.json --yes
```

`sch frame apply/check` 也接受 `--data` 内联 JSON，供 Apply 的 `run` 步骤使用。
框适配器适用于任意模块；当前 `power-layout` 的器件规划范围仍为固定 LDO 四器件。
首次转换前核对页面身份；队列在修改前后核对全部器件与 pin→net，图形回读通过后保存。
生成队列带 `requireFullExecution`，禁止 `--resume/--from/--to` 跳过校验及更换目标页面。
失败后回读、重新生成并完整执行，已有正确框会通过幂等比较保留。

每个页面、每个模块分别记录本工具的框/标题 ID。重复执行时核对完整几何和样式，
已有目标保持不变；只替换工具记录的旧图形，不根据颜色或文字模糊删除用户图形。
失败后先回读实际状态，不盲重试写操作；回读缺失或样式不符均不能报告验证通过。

## 验证边界

离线回归验证多组位置、尺寸、旋转输入下的包络和队列生成，并包含错网、缺字段、
出纸张、字号/颜色/线型不一致等负样例。现场验证同一数据首次 Apply 和重复 Apply，
核对框与文字、引脚网表和严格电气门禁，最后用官方导图辅助检查阅读效果。
该流程验证软件转换机制，不代表整板制造或电源稳定性验证。

EasyEDA 3.2.186 的实测适配：矩形 `fillStyle` 即使传入 `None` 仍回读 null，
因此显式传 `fillColor="none"` 并核实无填充；文字请求 `LEFT_TOP=1` 后回读枚举 2，
但实际 bbox 左上角与目标一致。因此对齐以真实 bbox 的 `minX/maxY` 验证，
同时核对 fontSize、位置、旋转、颜色和内容。首次旧转换请求被校验拦下，修正后全队列通过。
