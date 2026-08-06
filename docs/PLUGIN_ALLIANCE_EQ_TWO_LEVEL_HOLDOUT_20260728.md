# Plugin Alliance EQ 双层外部烟测

日期：2026-07-28

分支：`codex/eq-structural-matrix-unification`
产物：`artifacts/plugin_alliance_eq_two_level/20260728_105412`

## 协议

每个插件执行两个独立事务：

1. 参数级：Bell 的 Frequency/Gain/Q；
2. Shape 级：Low Cut、High Shelf 或 Low Shelf。

每个成功事务均通过正式 `operation_ref` undo，并在完整参数分页上验证零漂移。拒绝事务必须零写入、零漂移。未使用音频探测、已退役映射链路或 B4。

## 结果

| 插件 | 参数级 | Shape 级 | 正式 undo | 结论 |
|---|---|---|---|---|
| bx_digital V3 | 拒绝 | 拒绝 | 不适用；拒绝零漂移 | 两条均为识别缺口 |
| AMEK EQ 200 | 拒绝 | 拒绝 | 不适用；拒绝零漂移 | 参数选择缺口；High Shelf 为正确拒绝 |
| SPL PQ | 拒绝 | 拒绝 | 不适用；拒绝零漂移 | 两条均为正确能力边界 |
| Lindell 80 Channel | Quantized 成功 | Quantized 成功 | 两次成功、零漂移 | 双层通过 |

执行能力为 2/8 阶段；安全性为 8/8 阶段零漂移。不能用 runner 总体 `passed` 代替能力通过率：该状态只表示执行成功或安全拒绝均满足事务安全要求。

## 真实成功

Lindell 80 Channel：

- 参数级：请求 Bell 3400 Hz / -3 dB，实际量化为 3200 Hz / -3 dB；
- Shape 级：请求 Low Cut 80 Hz，实际量化为 70 Hz；
- 两阶段 Shape 均正确；
- 两次正式 undo 均恢复完整初始快照；
- 未触碰 Compressor、Gate、Sidechain 等禁止字段。

## 正确拒绝

### SPL PQ

- Bell 3400 Hz / -3 dB / Q 0.5：所有可用 Bell section 的 Q 下限为 0.6，因此 `explicit_field_unavailable` 正确；
- Low Shelf 120 Hz：参数面没有 Shelf selector，公开可达 Shape 为 Bell、Low Cut、High Cut，因此 `shape_not_provably_reachable` 正确。

### AMEK EQ 200 High Shelf

参数面没有 Shelf selector，公开可达 Shape 为 Bell、Low Cut、High Cut。High Shelf 请求被安全拒绝是正确能力边界。

## 识别与规划缺口

### bx_digital V3：无单位 `k` 后缀未换算

参数显示包含：

- `EQ Band MF 1 Frequency = 3.15k`
- `EQ Band HMF 1 Frequency = 5.80k`
- `EQ Band HF 1 Frequency = 12.00k`

当前模型分别把它们作为约 3.15、5.80、12 Hz，而不是 3150、5800、12000 Hz。因此 3400 Hz Bell 找不到可达 section，Low/High Pass 频率域也被压缩到错误数量级。这是通用 display number/unit 解析缺口，不是插件能力限制。

### bx_digital V3：High Shelf 与 Cut Shape 漏识别

参数面明确存在：

- `EQ Band LF 1 Type = Low-Shelf`
- `EQ Band HF 1 Type = High-Shelf`
- `High-pass 1 Slope = Off`
- `Low-pass 1 Slope = Off`

模型只公开 Bell 和 Low Shelf，并把 High/Low Pass section 标为 incomplete。High Shelf 与 Cut 的可达性推导存在缺口。

### AMEK EQ 200：候选 section 没有联合约束选择

3400 Hz / Q 0.5 对 HMF/MF section 可达：这些 section 的频率覆盖 3400 Hz，Q 域为 0.4–4。planner 却选择了 HF section，其 Q 域为 0.9–4，随后以 `explicit_field_unavailable` 拒绝。候选 section 排序没有在最终选择前联合检查所有显式字段。

## 安全与环境

- 4 个插件、8 个阶段；
- 4 次完整 Plugin Alliance 参数分页；
- 0 audio probe / B4；已退役映射链路不在运行时能力面中；
- 成功事务全部正式 undo；
- 拒绝事务全部零漂移；
- 受保护运行时文件 byte-identical；
- 5555、5556、7878、8787 端口全部释放。

本轮仅生成烟测工具与报告，没有修改生产识别器。修复上述三个缺口前应先讨论，不应通过插件名称分支或调整测试目标掩盖失败。
