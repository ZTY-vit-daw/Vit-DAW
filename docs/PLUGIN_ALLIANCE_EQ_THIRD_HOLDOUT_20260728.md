# Plugin Alliance EQ 第三轮分层随机外部烟测

日期：2026-07-28

分支：`codex/eq-structural-matrix-unification`

初始产物：`artifacts/plugin_alliance_eq_two_level/20260728_115128`
修复后最终产物：`artifacts/plugin_alliance_eq_two_level/20260728_124447`

## 样本冻结

仅使用 VST3 文件名和公开扫描类别，在读取参数前以固定种子 `2026072803` 分层随机锁定五个未测试对象：

| 分层 | 对象 |
|---|---|
| 现代参数 EQ | bx_hybrid V2 |
| 模拟/Mastering EQ | Millennia NSEQ-2 |
| Channel Strip | bx_console SSL 9000 J |
| 固定/半参数 EQ | Lindell PEX-500 |
| 专用滤波器 | bx_cleansweep V2 |

每个对象固定执行 Bell 3400 Hz / -3 dB / Q 0.5，以及 Low Cut 80 Hz。锁定后没有替换对象或调整目标。

## 结果

| 对象 | Bell 参数级 | Low Cut Shape 级 | 判定 |
|---|---|---|---|
| bx_hybrid V2 | 安全拒绝 | 安全拒绝 | 两个通用识别/规划缺口 |
| Millennia NSEQ-2 | 安全拒绝 | 安全拒绝 | Bell 为通用 range-multiplier 缺口；无 Low Cut 为正确拒绝 |
| bx_console SSL 9000 J | 安全拒绝 | exact：80.9 Hz，Shape=`low_cut` | Q 最低 0.7，Bell 请求正确拒绝；Cut 通过 |
| Lindell PEX-500 | 安全拒绝 | 安全拒绝 | 耦合 boost/attenuate 且无 Cut；两项请求均不具备通用可执行语义 |
| bx_cleansweep V2 | 安全拒绝 | exact：80 Hz，Shape=`low_cut` | Cut-only 对象正确拒绝 Bell；Cut 通过 |

能力执行为 2/10；安全性为 10/10。两个成功事务均通过正式 `operation_ref` undo 恢复，所有拒绝与最终分页均为零漂移。

## 正确拒绝

- bx_console SSL 9000 J：3400 Hz 只能落入 High-Mid，其 Q 域为 0.7–2.5；Q=0.5 不可达。
- bx_cleansweep V2：参数面只有 High Cut/Low Cut，没有 Bell/Gain/Q。
- Millennia NSEQ-2：参数面没有 Low Cut。
- Lindell PEX-500：高频 Boost/Attenuation 是不同控制律，不能证明为任意双极 Bell Gain；参数面也没有 Low Cut。`not_static_eq` 的错误码过粗，但没有丢失本轮两个请求的真实可执行能力。

## 新通用缺口

### 1. Quality 多角色控制

bx_hybrid V2 使用同一词根 `Quality` 表达不同物理角色：

- 中频 Bell：数值为 Q，例如 0.3、0.5、0.7；
- HP/LP：值为 6/12/18/24 dB/oct，即 Slope；
- LF/HF：还可出现 LoShelf/HiShelf，即 Shape。

当前词法只识别 Q/Width/Bandwidth，没有根据 section kind 与 reachable-value 物理标签分解 `Quality`，因此 Bell section 丢失 Q binding。这不是增加一个 `quality -> q` 关键词即可安全解决的问题，而是通用的 role-multiplexing topology。

### 2. 零值 sentinel 的可调频率域

bx_hybrid V2 的 HP Frequency 暴露 0–26000 Hz，0 是边界/sentinel，同时存在独立 On/Off。模型已形成 complete Low Cut section，但 section 排序要求正频率锚点，因而拒绝 80 Hz。需要把零 sentinel 与正频率活动域分离，并保持 readback correction；不能把整个零起点域直接当普通线性频率。

### 3. Frequency range multiplier

Millennia NSEQ-2 的 High-Mid section 含 `Frequency`、`Frequency X10`、Q、Gain 和 In/Out。基础频率域为 250–2500 Hz；X10 后可覆盖 2500–25000 Hz，因此 3400 Hz / Q 0.5 可达，Gain -3 dB可按枚举量化。当前模型未把 X10 绑定为频率换算的一部分，错误地认为 3400 Hz 超域。

这是新的通用 `frequency_transform/range_multiplier` 结构，会同时改变物理域、归一化换算和写入顺序。

Millennia 的 High Frequency 枚举还以无单位的 `4.8...21.0` 表示 kHz，这是额外观察到的单位上下文问题，但不是本轮 3400 Hz Bell 拒绝的必要根因。

## 安全审计

- 5 个外部对象、10 个阶段；
- 0 audio probe / learn / profile / SPAL / B4；
- Channel Strip 未写入 dynamics、compressor、gate、sidechain 或 detector 参数；
- 受保护运行时文件 byte-identical；
- 5555、5556、7878、8787 端口全部释放。

## 通用修复与最终复测

经用户确认后，只针对上述三个缺口加入了身份无关的结构规则，没有加入 Plugin Alliance 或具体插件名称分支：

1. `Quality` 只在局部物理证据充分时投影：数值 Q 当前值与数值 Q 域共同成立时映射为 Q；dedicated Cut 且 reachable label 明示 dB/oct 时映射为 Slope；Shelf 等符号状态保持未决。明确命名为 `Slope` 的旧式 `6 dB / 12 dB` 枚举继续兼容。
2. 零起点频率曲线的正物理采样可参与 section 频率排序；0 本身仍不被当作可写的普通正频率。
3. 同 section 的 `Frequency XN` On/Off 或 In/Out 枚举形成 `frequency_transform` binding。planner 选择可达 factor，以 `effective target / factor` 写基础频率，再写 transform。基础频率读回按插件显示值严格校验，公开结果按有效频率报告；transform 枚举仍独立要求 normalized 与 label 精确读回。

最终结果：

| 对象 | Bell 参数级 | Low Cut Shape 级 | 最终判定 |
|---|---|---|---|
| bx_hybrid V2 | exact：3400 Hz / -3 dB / Q 0.5 | exact：80 Hz | 两个缺口均恢复 |
| Millennia NSEQ-2 | quantized：3396 Hz / -4 dB / Q 0.5；X10=`On` | 安全拒绝 | range multiplier 生效；Gain 按真实枚举量化；无 Cut 拒绝保持 |
| bx_console SSL 9000 J | 安全拒绝 | exact：80.9 Hz | Q=0.5 不可达的拒绝保持 |
| Lindell PEX-500 | 安全拒绝 | 安全拒绝 | 两项结构性拒绝保持 |
| bx_cleansweep V2 | 安全拒绝 | exact：80 Hz | Cut-only 语义保持 |

最终为 5/10 实际执行、5/10 正确安全拒绝、10/10 安全与恢复通过。五个成功事务全部通过正式 `operation_ref` undo；Millennia 一次恢复 Frequency、Frequency X10、Q、Gain 四个参数。所有对象最终分页零漂移。

## 回归与单调性

- 相关 Go 回归：`plugingrabber`、`chat`、`agentloop`、`tools`、`eqtopologyreplay` 全部通过。
- 历史 16 capture：14 recognized / 14 executable，较修复前 0 回退。
- Waves 50 capture：44 recognized / 38 executable，较修复前 0 回退。
- 第三轮 22 snapshot：18 recognized / 18 executable，顶层 capability 0 回退；结构变化体现在歧义 binding 合并和 transform-aware planner。
- 离线对照产物：`artifacts/eq_third_holdout_repair/20260728`。
- 最终实机审计：0 audio probe / learn / profile / SPAL / B4；受保护运行时文件 byte-identical；5555、5556、7878、8787 端口全部释放。
