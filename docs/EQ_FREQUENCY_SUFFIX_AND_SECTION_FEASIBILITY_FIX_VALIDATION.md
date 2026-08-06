# EQ 频率后缀与 section 可行性修复验证

日期：2026-07-28
分支：`codex/eq-structural-matrix-unification`

## 修复范围

本轮只修复两条与插件身份无关的通用规则：

1. Frequency/Hz 物理域支持显示值中的裸 `k/K` SI 后缀，并统一用于 probe、topology 和执行 readback；Gain、Q 等其他物理域不会应用该倍率。
2. typed section planner 在按频率和当前状态排序前，先对每个结构候选无写入地检查 Shape、Frequency、Gain、Q、Slope 与 Activation；只有能满足整个原子请求的 section 才参与最终选择。

没有加入插件名称分支、音频主动探测、已退役映射链路、B4 或 `1/2` 通道猜测。

## 自动化验证

- `go test ./internal/workflows/plugingrabber ./internal/chat ./internal/agentloop ./internal/tools ./cmd/eqtopologyreplay`：通过。
- 历史 16 capture：14 个建模、14 个可执行；相对既有 replay 为 0 回退。
- Waves 50 capture：44 个建模、38 个可执行；相对既有 replay 为 0 回退。

离线产物：

- `artifacts/eq_generic_gap_fix/20260728/legacy_replay_after.json`
- `artifacts/eq_generic_gap_fix/20260728/legacy_comparison.csv`
- `artifacts/eq_generic_gap_fix/20260728/waves_replay_after.json`
- `artifacts/eq_generic_gap_fix/20260728/waves_comparison.csv`

## Plugin Alliance 真实双层烟测

完整 Godot → Kernel → Agent 产物：

- `artifacts/plugin_alliance_eq_two_level/20260728_111246/summary.json`
- `artifacts/plugin_alliance_eq_two_level/20260728_111246/run_manifest.json`

| 插件 | 参数级 | Shape 级 | 结论 |
|---|---|---|---|
| bx_digital V3 | Bell 3400 Hz / -3 dB / Q 0.5 exact | Low Cut 安全拒绝 | 裸 `k` 三链修复生效；Cut 仍有独立结构缺口 |
| AMEK EQ 200 | Bell 3400 Hz / -3 dB / Q 0.5 exact，选择 HMF 1 | High Shelf 安全拒绝 | 全显式字段候选筛选生效；Shelf 拒绝保持 |
| SPL PQ | 安全拒绝 | 安全拒绝 | Q 下限与 Shape 边界均保持正确拒绝 |
| Lindell 80 Channel | quantized 3200 Hz / -3 dB | Low Cut quantized 70 Hz | 原有双层能力未回退 |

所有成功事务均通过正式 `operation_ref` undo 恢复；所有拒绝和最终分页均为零漂移。受保护运行时文件 byte-identical，端口 5555、5556、7878、8787 已释放。审计计数为 0 audio probe、0 B4；已退役映射链路不在运行时能力面中。

## 后续结构状态

以下结构已在后续 `property_sentinel` 修复中解决，验证见 `docs/EQ_PROPERTY_SENTINEL_CUT_FIX_VALIDATION.md`。

`bx_digital V3` 的 Cut 拒绝已证明不是频率单位问题。修复后其 High-pass/Low-pass 频率曲线正确恢复到 Hz/kHz 数量级，但：

- 名称被分词为 `high pass 1` / `low pass 1`，现有 dedicated-cut 词法归一化只覆盖无尾缀或单 token 的 `highpass` / `lowpass`；
- 同 section 的 Slope 枚举为 `Off / 6 dB / 12 dB`，它同时承担 Activation 与 Slope，当前模型只将其作为 Slope 绑定。

后续实现将两者分别处理为现有 Cut 词法归一化补全和通用 `property_sentinel` activation，并已通过 bx_digital V3 的 Cut/undo 真实烟测。
