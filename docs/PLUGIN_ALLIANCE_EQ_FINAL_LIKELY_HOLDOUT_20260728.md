# Plugin Alliance EQ 最终高通过概率外部烟测

日期：2026-07-28

分支：`codex/eq-structural-matrix-unification`
产物：`artifacts/plugin_alliance_eq_two_level/20260728_125659`

## 样本冻结

本轮不是随机抽样，而是在不读取参数的前提下，仅依据 VST3 文件名与宿主扫描类别，从未消费且不属于既有测试同族的对象中选择五个最可能通过者：

1. Bettermaker EQ232D
2. Knif Audio Soma
3. elysia museq Master
4. Harris Doyle Natalus DSCEQ
5. Black Box Analog Design HG-Q

排除了 AMEK EQ 250、bx 系列变体、Lindell、SPL、Millennia 和 Kirchhoff 等已经消费的型号族。冻结配置为 `scripts/eq_plugin_alliance_final_likely_holdout_smoke.json`。

每个对象固定执行两项请求：Bell 3400 Hz / -3 dB / Q 0.5，以及 Low Cut 80 Hz。锁定后未替换对象或修改目标。

## 实机结果

| 对象 | Bell 参数级 | Low Cut Shape 级 | 结构判定 |
|---|---|---|---|
| Bettermaker EQ232D | 安全回滚 | 安全回滚 | 两项均为 linked-stereo 镜像事务假阴性 |
| Knif Audio Soma | 安全拒绝 | 安全拒绝 | Bell 为词法/物理解析假阴性；80 Hz Cut 正确拒绝 |
| elysia museq Master | 安全拒绝 | 安全拒绝 | Bell 请求因 Q 无数值语义而正确拒绝；Low Cut 为复合 Shape 假阴性 |
| Harris Doyle Natalus DSCEQ | 安全拒绝 | exact：80 Hz | Bell 无 Q，正确拒绝；Low Cut 成功并正式 undo |
| Black Box Analog Design HG-Q | 安全拒绝 | 安全拒绝 | stateful ganged/linked 音色网络，通用层正确拒绝 |

总计：1/10 实际成功，5/10 正确拒绝，4/10 通用假阴性；安全与恢复为 10/10。

## 正确拒绝

- Knif Audio Soma 的 Low Cut 枚举只有 Off / 25 / 33 / 50 Hz，80 Hz 不可达。
- elysia museq Master 的 Bell Q 只有 Wide/Narrow，没有可证明的数值 Q=0.5 映射，因此本次显式 Q 请求不能执行。
- Harris Doyle Natalus DSCEQ 的 High-Mid 只有 Frequency、Gain magnitude 和 Polarity，没有 Q 参数；Q=0.5 请求正确拒绝。
- Black Box HG-Q 当前 Frequency 字段在 Ganged 状态下只显示 `Ganged`，物理频率不可观测；同时采用独立 Boost/Cut 网络及状态切换。按通用静态 EQ 范围，两项请求均应拒绝。

## 通用假阴性

### 1. Linked-stereo 镜像事务

Bettermaker 已正确形成 Bell 与 Low Cut topology，并实际规划了正确参数。插件处于 `CHANNEL=STEREO`：

- 写 EQ2 的 1 侧 Frequency/Gain/Q，会同步改变 2 侧对应参数 29/30/31；
- 写 HPF Frequency 1，会同步改变 HPF Frequency 2（参数 23）。

执行器将这些镜像写入判定为 `unplanned_parameter_change` 并完整回滚。安全行为正确，但执行能力是假阴性。可泛化修复必须由对称重复 section 与显式 linked/stereo 状态推导镜像写集，并对两侧严格读回，不能按插件名放行任意副作用。

### 2. 工程记数、地区小数与 section 缩写

Knif Soma 的目标实际可达：High-Mid Frequency 有 3k3/3k9，Gain 有 -3.0，Bandwidth 有 `0,5`。当前结构未把以下事实统一：

- `HM/LM/LF/HF` 与 High Mid/Low Mid/Low/High section；
- `1k0/2k7/3k3` 工程频率记数；
- `0,5/0,7` 小数逗号。

因此 Bell core 没有合并，3400 Hz 本应量化到 3300 或 3900 Hz 的请求被错误拒绝。这三条均为身份无关的通用词法/物理解析规则。

### 3. Gain magnitude + polarity 与方向性 Type

elysia museq 使用：

- 非负 `Gain 0..15 dB`；
- `Mode = Boost/Cut` 表达增益符号；
- Low section 的 `Type = Shelf/Cut` 表达 Low Shelf/Low Cut；
- Low Frequency 域为 9..200 Hz，80 Hz 可达。

当前识别器没有把 Gain magnitude 与独立 polarity enum 合成为双极 Gain，也没有在方向已由 section 证明时把 `Type=Cut` 投影为 Low Cut。因此 Low Cut 80 Hz 是通用复合控制假阴性。Bell 请求仍因 Wide/Narrow Q 无数值映射而应拒绝。

## 安全审计

- 五插件、十阶段均完成真实 Godot → Kernel → Agent 链和完整参数分页；
- 唯一成功事务通过正式 `operation_ref` undo；所有拒绝和回滚后最终分页均为零漂移；
- 0 audio probe / B4；已退役映射链路不在运行时能力面中；
- 受保护运行时文件 byte-identical；
- 5555、5556、7878、8787 端口全部释放。

本轮只新增冻结配置和测试报告，没有继续修改生产识别器。

## 后续词法层收尾

经讨论后，只对 Knif 暴露的三条通用词法规则进行封顶修复：`HM/LM/LF/HF` section 缩写、`3k3/1k0` 工程频率和 `0,5` 小数逗号。修复后 Bell 请求成功：3400 Hz 量化到 3300 Hz，Gain -3 dB 与 Q 0.5 exact，并通过正式 undo 恢复。80 Hz Low Cut 的正确拒绝保持。

最终证据：`artifacts/plugin_alliance_eq_two_level/20260728_134330`。详细验证见 `docs/EQ_LEXICAL_CLOSURE_VALIDATION.md`。
