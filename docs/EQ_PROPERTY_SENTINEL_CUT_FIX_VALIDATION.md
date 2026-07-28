# EQ Property-Sentinel Cut 修复验证

日期：2026-07-28
分支：`codex/eq-structural-matrix-unification`

## 规则

本轮将此前发现的 Cut 缺口拆成两个身份无关的修复：

1. 词法归一化把 `HighPass 1`、`High-pass 1`、`High Pass 1` 统一为带原编号的 Low Cut section；Low Pass 对称映射为 High Cut。这不是新的 topology 轴。
2. 当 dedicated Cut 的 Slope 枚举同时包含 inactive 标签和正物理 slope（例如 `Off / 6 dB / 12 dB`）时，推导 `property_sentinel` activation：同一参数同时承担 Slope 和 Activation。

执行语义：

- 已激活且未请求 slope：保留当前 slope；
- 显式请求 slope：Slope 写入同时完成激活，并排在事务末尾；
- 当前为 Off 且未请求 slope：选择最小的正 slope，作为最保守的激活值；
- Disable：写回已证明的 inactive 枚举；
- Remove：resident section 继续拒绝；
- Undo：恢复 Frequency 与复合 Slope/Activation 参数的完整 preimage。

没有使用插件名称分支、音频探测、learn、profile、SPAL、B4 或 `1/2` 通道猜测。

## 自动化验证

- 针对性测试覆盖 compound 编号保持、inactive/active 推导、默认激活、显式 slope、保持现有 slope、Disable 和 activation-last 排序。
- `go test ./internal/workflows/plugingrabber ./internal/chat ./internal/agentloop ./internal/tools ./cmd/eqtopologyreplay`：通过。
- 历史 16 capture：14 建模、14 可执行；相对修复前为 0 回退、0 意外扩展。
- Waves 50 capture：44 建模、38 可执行；相对修复前为 0 回退、0 意外扩展。

离线产物：

- `artifacts/eq_property_sentinel_fix/20260728/legacy/replay_after.json`
- `artifacts/eq_property_sentinel_fix/20260728/legacy/comparison.csv`
- `artifacts/eq_property_sentinel_fix/20260728/waves/replay_after.json`
- `artifacts/eq_property_sentinel_fix/20260728/waves/comparison.csv`

## bx_digital V3 真实烟测

Godot → Kernel → Agent 产物：

- `artifacts/plugin_alliance_eq_two_level/20260728_113453/summary.json`
- `artifacts/plugin_alliance_eq_two_level/20260728_113453/run_manifest.json`

结果：

| 请求 | 写入与 readback | Shape | Undo |
|---|---|---|---|
| Low Cut 120 Hz，Slope 12 dB/oct | 120 Hz exact；12 dB exact | `low_cut` | 恢复 2 参数，零漂移 |
| 打开 Low Cut 80 Hz，不指定 Slope | 80 Hz exact；从 Off 选择 6 dB | `low_cut` | 恢复 2 参数，零漂移 |

模型将 `low cut 1/2` 均推导为 complete、dedicated `low_cut`、activation `property_sentinel(binding_role=slope)`；Upsert、Modify、Disable、Undo 可执行，resident Remove 保持拒绝。

审计：0 audio probe、0 learn、0 profile、0 SPAL、0 B4；受保护运行时文件 byte-identical；5555、5556、7878、8787 端口全部释放。
