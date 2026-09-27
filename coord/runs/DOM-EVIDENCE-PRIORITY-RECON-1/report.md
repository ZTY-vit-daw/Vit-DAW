# DOM-EVIDENCE-PRIORITY-RECON-1 勘察报告：band 证据来源优先级语义（MAT-D 裁定③决策输入）

- 卡片：`coord/cards/todo/2026-09-29-DOM-EVIDENCE-PRIORITY-RECON-1.md`
- 性质：只读勘察，零代码改动。
- 领取：2026-09-29（执行侧 PC 会话）/ main 已 `git pull`（Already up to date）/ 工作树既有改动 = `VitApp/Workspace/default_project.xml`（内核运行时状态，非本卡域，未触碰）+ 他卡 coord/runs 未跟踪工件。
- HEAD：`58180a41`。

---

## 0. 摘要（裁定③的事实输入）

1. **回退轴现状**：`domInputFromObservation` 的 band/waveform 回退序为「快照单数键 → **MixPackage.current_metrics** →（target 不匹配时）快照复数键 → current_metrics 兜底」。MixPackage 插在快照复数键**之前**——这是 MAT-D S2 分歧轴（`agent/internal/mixboard/dom_projection.go:25-48`）。
2. **语义精确化（裁决关键）**：current_metrics.waveform/band_energy 并非独立新测量——观察轮组装时由**同一个** featureSnapshot 的单数行派生（`mixboard.go:1219` `buildWaveformMetrics` / `mixboard.go:1239` `buildBandEnergySummary`）。所谓"测量遮蔽持久证据"实为「**单数行的轮内派生视图** 遮蔽 **复数键的目标轨持久行**」——两分支都是持久证据的投影，但行归属/时效/构建器不同。真正 F9 谱系的"请求携带测量"是 fxm_measurement / com paired 工件，不经此轴。
3. **消费面**：dom 投影五个维度（peak_structure/activity/frequency_time_events/transient/band_dynamics）全部由该 band/waveform 输入派生，进 CCB 五个 view、LLM ContextPack CompactFacts（含具体 dB 数值）、mix_read 全投影披露、物化对账 contentIdentityHash。回退序改快照优先 = 这些面的证据形态从"单数行轮内视图"变为"复数键目标行"。
4. **激活频率**：MAT-D-1 三个真栈 run、共 6 轮观察、6 次 dom reconcile、6 次分歧——**烟测场景 100% 激活**（方法与边界见 §3；单轨 fixture 样本，多轨工程无工件，不做外推精确数）。
5. **宪法对照**：修正③（快照优先、MixPackage 殿后）与 F9 裁定方向一致（请求携带量不入可预计算输入域），不违反"原样保留不升级 readiness"宪法；但有时效性注意点（复数键取行不选"最佳行"，见 §5）。

---

## 1. 回退序现状全景

### 1.1 dom 的 band/waveform 回退序逐分支

装配入口 `domInputFromObservation`（`agent/internal/mixboard/dom_projection.go:20-48`），band 与 waveform 完全对称：

| 支 | 代码锚点 | 触发条件 | 数据来源与语义 |
|---|---|---|---|
| ① 快照单数键 | `dom_projection.go:25-26`（waveform_envelope）、`:31`（band_energy_summary） | 行非空（`len>0`）即用 | `obs.GlobalSummary["feature_snapshot"]` 内的**全局单数行**（观察轮组装时 `compactFeatureSnapshot` 投影，`mixboard.go:2764-2792`）。DAD 持久特征快照（L3 物化行经遥测/落盘汇入 `mixboard_feature_snapshot.json`，`mixboard.go:1443-1489`），单数键经 `promoteBestL3FeatureRows` 从单/复数候选中晋升"最佳行"（`mixboard.go:1614-1628`，按 status/target 匹配/完整度打分，锚点 `realtimeFeatureRowBeats` `mixboard.go:1669-1681`） |
| ② MixPackage.current_metrics | `dom_projection.go:28-30`（waveform）、`:32-34`（band） | ①为空（`len==0`） | `obs.MixPackage["current_metrics"]`（`role:"realtime_mix_loop"`、`round:req.Round`，`mixboard.go:3190-3196`）。**注意**：其 waveform/band_energy 由观察轮组装时从同一 featureSnapshot 单数行派生——`waveformMetrics = buildWaveformMetrics(featureSnapshot.WaveformEnvelope)`（`mixboard.go:1219`，builder 定义 `:2958-2986`）、`bandEnergy = buildBandEnergySummary(featureSnapshot.BandEnergySummary)`（`mixboard.go:1239`，builder 定义 `:2988-3030`：拷贝 track_id/clip_id/request_id/测量条件，做 silence 确认与近似换算）。即**单数持久行的轮内投影视图**，非独立测量 |
| ③ 快照复数键（target 修正） | `dom_projection.go:35-42` | target kind ∈ {track, clip}（`targetNeedsMatchingRow`，`:239-246`）且当前行（①或②的结果）不匹配 target（`featureRowMatchesTarget` 查 track_id/id/clip_id，`:248-260`） | `featureRowForTarget(snapshot["track_waveform_envelopes" / "band_energy_summaries"], target)`（`:262-274`）——**per-track 持久行**，取第一条匹配行；无匹配返回 nil（会覆盖掉②的结果） |
| ④ current_metrics 兜底 | `dom_projection.go:43-48` | ③之后仍为空 | 同②。**兜底后不再做 target 匹配检查**——④的行可以不匹配目标 |

**同一装配的第二个调用方**：物化层 `DOMAdapter.Build` 合成 ObservationPacket（仅 `GlobalSummary.feature_snapshot` + `project_revision`，MixPackage 恒空）调同一 `domInputFromObservation`（`agent/internal/materialize/adapters.go:307-334`；导出包装 `agent/internal/mixboard/dom_materialize.go:16-18`）。影子对账时两路径的差异**只来自 MixPackage 有无**（`agent/internal/harness/materialize_shadow.go:107-147` 对账 + `:188-213` DepInputs 装配不含 MixPackage）。

### 1.2 回退轴激活的触发条件（分歧机制）

物化对账两侧同一 snapshot、同一 target，唯一变量是 MixPackage。激活（且产生 hash 分歧）需要同时满足：

1. 单数键行为空（`compactFeatureRow` 返回空 map——输入行无任何白名单键，`mixboard.go:2892-2900`；normalize 只对 nil 填 missing stub，`mixboard.go:1506-1514`）**或**②取到的行不匹配 target 而复数键有目标行（此时观察路径在②保留 current_metrics 视图、物化路径走③取复数键行——两侧取到**快照中不同的行/形态**）；
2. `MixPackage.current_metrics.band_energy` 非空（真栈观察轮恒非空：`buildMixPackage` 无条件填 current_metrics，`mixboard.go:3190-3202`）；
3. 观察路径产物进 dom 行 reconcile（target=track，`materialize_shadow.go:108-110`）。

补充：持久化投影会进一步改变观察包内快照键集（v2 落盘 strip `waveform`/`time_segments` 等键，`agent/internal/mixboard/persistence_v2.go:126-163`；预算超限时 feature_snapshot 整体替换为 `{"status":"projected"}`，`:165-194`）——reload 的历史观察在此轴上的行为与新鲜观察不同，属于次生形态，本次未展开。

单数行另有请求新鲜度门：行不匹配当前请求会被判 `stale_feature_snapshot_for_current_request` 换成 missing stub（`mixboard.go:2251-2285`，基线判定 `:2227-2249`）——这是"单数键为空/不匹配"的常见成因之一。

### 1.3 其他投影对照盘点（"测量遮蔽持久证据"同款形态）

| 投影 | 输入装配锚点 | 回退形态 | 对照结论 |
|---|---|---|---|
| **com** | `agent/internal/mixboard/com_projection.go:97-102` | `snapshot["waveform_envelope"]` 为空 → `MixPackage.current_metrics.waveform`（两支，仅 waveform；无 target 修正、无复数键） | **同款形态存在**（snapshot 优先、MixPackage 殿后），但 com 在物化层是**登记型**（F9：`materialize/adapters.go:405-417` COMRowsFromObservation 只登记观察产物，无重算对账）——不产生 ShadowDivergence，但登记行的证据来源语义同样受回退序影响 |
| **mom** | `mixboard.go:3990-4016`（整包透传 MixPackage）；消费面 `agent/internal/mom/projection.go:98-276` | 设计语义就是消费轮内 current_metrics（`buildBasicEnergy` `:97-116`、`buildTimbreFrequency` `:118-170` intent 驱动 l3/l2 选择）；唯一反向兜底：waveform missing 时用 GlobalSummary 顶层 peak/rms（`:101-104`）——**测量缺失→持久概要兜底，方向相反** | 无同款遮蔽形态（MOM 的宪法就是"混音测量观察"） |
| **tim** | `mixboard.go:4018-4042`（timInput 不含 MixPackage）；`authoritativeTIMState` `:4106-4173` | 消费 project state + L1 analysis manifest + acoustic package status（只读持久事实） | 无该形态 |
| **fxm** | `mixboard.go:3969-3988` | 输入= `req.Args["fxm_measurement"]`（A/B 测量，F9 登记型），不读 feature_snapshot | 本身即测量型投影，无回退轴 |
| **tom/acp**（物化 precomputable 同侪） | `materialize/adapters.go:136-298` | 输入域 = shadow summary / acoustic store，不消费 MixPackage | 无该形态 |

**结构性结论**：物化注册 kind = {tom, acp, dom}（precomputable）+ {fxm, com}（登记型）（`materialize/adapters.go:443-449`）。**dom 是唯一"重算对账型 + 输入装配含 MixPackage 回退"的投影**——MAT-D 分歧只出现在 dom 是结构必然，非偶发。

---

## 2. 消费面影响

### 2.1 band/peak_structure 族字段消费点清单

dom.Build 的五个维度（peak_structure / activity_structure / frequency_time_events / transient_structure / band_dynamics）**全部**由 `input.Source`（含 Bands/TimeSegments/NoiseFloor/Frequency/Transient/BandDynamicsEvidence，即回退轴选中的行）派生（`agent/internal/dom/projection.go:27-38`），随后流向：

| # | 消费面 | 锚点 | 消费内容 |
|---|---|---|---|
| 1 | **CCB 语义 view**（bounded disclosure） | `agent/internal/capabilitycontext/free_state_observation.go:432-436` | track.peak_structure / track.activity_structure / track.frequency_time_events / track.transient_structure / track.band_dynamics 五个 view 挂 `observation.dom_projection` 键；view 状态取对应维度字段 status（`:638-655`）；披露按 view 裁剪（`compactDOMDimension`，`:691-701`） |
| 2 | **LLM ContextPack** | `agent/internal/mixboard/mixboard.go:4826-4828`（buildContextPack，`:4799`） | `dom.ContextProjection` + `DOMProjection.LLMContext`（dynamics_observation_context）。LLMContext 的 CompactFacts **直接携带数值**：peak_dbfs/headroom_db/crest_db（`agent/internal/dom/projection.go:605-611`）、whole_window_band_count（`:615-617`）、band_dynamics band_count（`:621-623`）；read_hints 指路 mix_read（`mixboard.go:4809-4817`） |
| 3 | **mix_read** | `agent/internal/mixboard/catalog.go:481-485`（readObservationKey） | `observation.dom_projection` 返回**完整投影**（含全部 band/peak 族字段）；digest 摘要含 dom 状态（`:171-179`） |
| 4 | **contextruntime 模型投影** | `agent/internal/contextruntime/model_projection.go:1312-1313` | dom 投影 nested facts（维度字段）+ readiness（status/dimension_readiness/trust_quality/evidence_refs/limitations）进模型上下文 |
| 5 | **物化影子对账**（MAT-D 分歧面） | `agent/internal/materialize/adapters.go:361-382` | `contentIdentityHash(projection)` 对**整个投影结构体**取 hash——任何 band/peak 族字段差异都导致 hash 分歧 |
| 6 | **静态平衡/B1 门** | `agent/internal/capabilitycontext/context_manifest.go:277-306`；`agent/internal/capabilitycontext/gain_staging.go` | **不直接消费 dom band**——band 证据走 MOM（mom_projection.band_occupancy，current_metrics 谱系）、level 门走 current_metrics.waveform/track state。回退序修正（只动 dom builder）不影响 B1/B2 门的 band 输入 |

### 2.2 回退序改为"快照优先、MixPackage 殿后"后各消费面的变化

修正③语义：单数键空/不匹配 → **先**取复数键目标行 → 仍无才用 current_metrics。消费面拿到的证据形态变化：

- **证据行归属**：从「单数全局行的轮内派生视图（经 buildBandEnergySummary 转换，可能属于 latest_request 解析的目标而非当前观察 target）」变为「复数键中**匹配当前 target** 的持久行」。target 归属更准。
- **时效语义**：两种情形。(a) 分歧场景下单数键为空、复数键有行——修正后 dom 从"单数行视图（promote 晋升的最新全局行，可能与 target 无关）"变为"目标行"，通常是**更新鲜且更对口**的证据；(b) 反向风险：单数键有行但 target 检查不匹配时，现行④兜底的 current_metrics 与修正后③的复数键行之间的新旧关系不确定——**`featureRowForTarget`（`dom_projection.go:262-274`）取第一条匹配行，不做 promoteBestL3FeatureRows 式的"最佳行"比较**，可能取到较旧的复数行。若采③，建议同步评估复数键行选取的最佳性（这是修正的伴生语义决策点，非本卡裁定范围）。
- **数值面**：buildBandEnergySummary 会做 silence 确认（suspect→partial 升格，`mixboard.go:2990-3008`）与 energy_db 近似换算（`:3024`）；复数键原行不经此转换——CCB view 状态、LLM CompactFacts 数值、mix_read 披露的具体值都随之变化（形态差异，非 readiness 升格）。
- **物化对账**：观察路径与物化路径在"快照有行"时自然同源 → ShadowDivergences 归零路径成立（MAT-D 修复③的预期收益）。MixPackage 仅在快照完全无行时兜底——该形态行按 MAT-D2（登记型+measurement-carried 注记+闸门排除）处置，两卡互补。
- **宪法合规**：修正只换证据来源，不把 missing 升格为 ready——不违反"缺失/partial/stale 原样保留不升级 readiness"（`docs/OBSERVATION_PROJECTION_MANIFEST.md:13`、`docs/MATERIALIZATION_V1_DESIGN.md:38`）。

---

## 3. 真实数据分布：激活频率量级（MAT-D-1 工件估算）

**方法**（不做任何编造精确数，全部可复算）：

1. 样本：`coord/runs/MAT-D-1/materialize_shadow_smoke_2026092{7_163619,7_164058,7_164458}/`（真栈三件套 shadow 态、单轨 fixture wav 2s、target=track 1007、每 run 两轮 `mix_request_observation`（set_volume 前后各一），日志 `agent_last.log` + `shadow_round_metrics_lines.txt` + `divergence_lines.txt` + `run_report.json`）。
2. 计数口径：`shadow_round observation_id=` 行数 = 观察轮数；行内 `dom reconcile_rows` 与 `shadow_divergences`（累计值）差分 = 每轮新增分歧数；`divergence ref=` WARN 行 = 分歧明细（run 2/3 有，run 1 取证代码未含明细，以 metrics 行承载）。

**结果**：

- 3 run × 2 轮 = **6 轮观察**；每轮 dom reconcile_rows=1（单轨一行）、shadow_divergences 每轮 +1 → **6/6 = 100% 激活**（同 ref `vit://dom/track:1007/t=all@current`）。
- 第 3 run 探针（`divergence_lines.txt`）：`snapshot_waveform_envelope=true mix_metrics_waveform=true time_ruler_duration=2`——waveform 单数键非空（①支直用，**waveform 轴不回退**）；分歧集中在 band 轴（MAT-D 根因：band 单数键空 → current_metrics 回退）。fresh 投影 evidence_refs = `dad.l3.band_energy_summary:<wav>`——band 证据溯源仍指向 L3 持久特征行（经 current_metrics 派生视图），佐证 §1.1 的"同源不同形态"结论。
- 两轮 fresh 投影除 `project_revision`（3→4，set_volume 效果）与 observation_id/projection_id 外内容相同、但均与 stored 侧 hash 不同——分歧是**来源形态差**，非数据演化。

**量级与边界（如实申报）**：

- 该 100% 是**烟测场景**（单轨、固定 target、显式两轮观察）的量级；多轨/长会话工程无直接工件，本卡不外推精确比例。
- 机制外推（供权重判断，非测量值）：真栈凡走 `requestMixObservation` 的观察轮 current_metrics 恒非空（`buildMixPackage` 无条件填充），物化侧恒无 MixPackage——**该轴在 shadow 对账启用期间结构性暴露**；分歧是否发生取决于"单数键空/不匹配 target 且复数键或 metrics 视图有料"的出现率。烟测里该条件恒真；真实工程的单数键通常经 promote 晋升有行（匹配时①支直用无分歧），预计低于 100%，但**无工件支撑的具体数字，需 MAT-D2 重跑 S2 或后续真栈 run 才能给出**。

---

## 4. 宪法依据原文锚点（裁决的"宪法"输入）

| 依据 | 原文锚点 | 与裁定③的关系 |
|---|---|---|
| **F9 裁定**（测量随请求不预计算） | `docs/MATERIALIZATION_V1_DESIGN.md:25`：「F9 \| FXM/COM paired 输入随请求携带（测量/双 tap 工件），不是工程状态——不可事件驱动预计算」 | MixPackage 是请求轮内包；若 dom 行要 precomputable+零分歧，其装配输入应落在 feature snapshot 输入域内。**修正③使 dom 装配与 F9 方向一致**（MAT-D 裁定①"DepInputs 扩轴携带 MixPackage"与 F9 冲突被否决的同一逻辑） |
| F9 登记型细则 | `docs/MATERIALIZATION_V1_DESIGN.md:96`：「fxm（测量随请求携带，E7）、com paired…在物化层是**登记型**…不参与事件重算，不进依赖图」 | MAT-D2（measurement-carried 注记+闸门排除）的文档依据 |
| dom 输入域声明 | `docs/MATERIALIZATION_V1_DESIGN.md:230`：「`dom.Build`/`mom.Build` 吃整包输入（feature snapshot 全量行+acoustic 整包），**仅 fxm 随请求携带（F9，登记型）**」；`agent/internal/materialize/adapters.go:301`（注释：「dom 适配器（输入域：feature snapshot，source_only 档）」） | 文档把 dom 输入域写成 feature snapshot——与代码实际装配（含 MixPackage 回退）存在**文档-实现偏差**，正是裁定③要收口的语义 |
| 输入分层词表 | `docs/MATERIALIZATION_V1_DESIGN.md:167`：「其余 15 条全是"投影←输入源"（**feature snapshot 行、acousticpackage 状态、shadow 域、请求携带测量、双 tap 工件**）」；`:173-176`（Compute: precomputable \| registered(F9)） | "持久证据（前三类）vs 请求携带测量（后两类）"分层的原文——回退序优先级即这两层之间的排序 |
| **宪法（继承裁定）** | `docs/MATERIALIZATION_V1_DESIGN.md:38`：「D2 统一寻址不统一存储引擎；D5 冷热分离与成本分级；投影 peer 非链；缺失/partial/stale 原样保留不升级 readiness」 | 修正③不升级 readiness、不动 peer 拓扑——合规 |
| **peer 拓扑 ground truth** | `docs/OBSERVATION_PROJECTION_MANIFEST.md:10-16`：「所有投影是 DAD 之上的同级 peer…CCB 不做自动 view 选择；缺失/partial/stale/suspect 原样保留，不升级 readiness」 | 同上；另约束消费面变化必须原样呈现（§2.2 的形态差异须如实流入 CCB/LLM，不做升格） |
| freshness 语义 | `docs/MATERIALIZATION_V1_DESIGN.md:127/133`（current：「本 generation 内已重算（或登记）且输入域此后未再变更」） | 若采③，dom 行 freshness 的"输入域"边界随之清晰为 feature snapshot——MixPackage 兜底形态按 MAT-D2 登记型处置 |

---

## 5. 裁决输入汇总（事实 → 权重）

**支持③（快照优先、测量殿后）的事实**：
- dom 是唯一"precomputable+MixPackage 回退"投影，分歧结构性集中于它（§1.3）；
- 修正使 dom 装配与其声明的输入域（feature snapshot，`adapters.go:301`）一致，F9 方向一致（§4）;
- 证据行 target 归属更准（复数键目标行 vs 单数全局行派生视图）（§2.2）;
- 烟测场景 100% 激活——分歧不是边角案例（§3）。

**谨慎/伴生决策点**：
- `featureRowForTarget` 取第一条匹配行、无"最佳行"比较——修正后可能取到较旧复数行（§2.2 时效 (b)）；建议与③同步裁定复数键行选取规则；
- current_metrics 实为单数行派生视图而非独立测量（§1.1 支②语义）——若决策侧认定"派生视图也算轮内测量谱系"，③的理由更充足；若认定"同源不算遮蔽"，则③的收益只剩"物化同源+target 归属"，权重相应调整；
- 多轨真实工程的激活率未知（§3 边界）——建议 MAT-D2 重跑 S2 时顺带采集多轨样本，或采③后以 S2 转绿作闭环验证。

**对其他投影的波及评估**：com 装配有同款两支回退（`com_projection.go:98-102`）但为登记型、无对账面——若裁③为"快照优先"通则，com 是否同步对齐是附带语义决策（低紧迫：com 无分歧面，且其 MixPackage 回退同样实为单数行派生视图）；mom/tim/fxm/tom 无该形态（§1.3）。

---

## 6. 勘察边界申报

- 零代码改动；全部结论来自源码阅读 + MAT-D-1 既有工件复算，未启动真栈。
- 频率估算基于 MAT-D-1 三 run（单轨 fixture）；多轨外推仅给机制判断，未给编造数字。
- 持久化投影（persistence_v2 strip/hard projection）对 reload 观察在此轴上的影响只做机制标注（§1.2 末），未展开验证。
- `git status --short` 变化 = 本报告目录新增（coord/runs/DOM-EVIDENCE-PRIORITY-RECON-1/），源码树零触碰。
