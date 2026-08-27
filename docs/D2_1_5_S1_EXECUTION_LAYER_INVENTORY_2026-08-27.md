# D2-1.5-S1 执行层表驱动勘察清单（2026-08-27）

- 性质：只读勘察产物，为今晚 D2-1.5-S1（实验通道执行层表驱动）提供逐点 diff 锚点。
- 方法：逐文件枚举"按域变化"的代码点——新增第三个域时必须手改、或必须由域表覆盖的位置。每点给 `file:line` 与 track_gain / static_eq 两域各自取值。
- 输入：任务卡 `C:\Users\timoz\.zcode\workspace\default\queue\todo\2026-08-27-D2-1-5-S1-table-driven-execution-layer.md`（含"早窗新增输入"五要点，本文不重复挖掘，只在锚点处回指）。
- 边界：结论限于锚点枚举，不做设计裁决。五个目标文件组全部存在且各节齐全；另补 3 个文件外关联锚点（§6），因目标文件引用的符号定义在那里。
- 行号基线：分支 `codex/g1-g7-runtime-remediation`，HEAD `26475d3`（2026-08-27 勘察时点）。

## 0. 早窗五要点的行号落点（回指，非新结论）

| 任务卡要点 | 精确锚点 |
|---|---|
| 1. `juce_eq` 占位符不存在 | `chat/free_state_d1_runtime.go:38`（`defaultD1StaticEQPluginIdentifier`），注入点在 `:171-174` |
| 2. 值语义（归一化 0..1 vs 原始 dB） | `executionports/staticeq_vsp.go:133-148`（set 发原始 dB `value`）与 `:159-161`、`:243-275`（回读取 `value/current_value/normalized_value` 或解析 `value_text` 后与 dB 目标比） |
| 3. `band_%d_gain` 约定对真实插件不成立 | `chat/free_state_d1_runtime.go:164-170`（paramID 组装），校验消费在 `executionports/staticeq_vsp.go:51-53` |
| 4. PCA 载入门不守 VSP 通道 | 无代码锚点（实测结论），影响面在 `staticeq_vsp.go` 全部三个命令 |
| 5. 三个已修 bug 勿回退 | D1 回执投影域：`chat/free_state_d1_receipt.go:123-127`；域枚举与原生域路由在 chat/agentprotocol 包其它文件，不在本次五文件范围内 |

## 1. `agent/internal/chat/free_state_d1_runtime.go`（457 行）

### 1.1 域常量（新增域必加一组）

| 锚点 | track_gain 取值 | static_eq 取值 |
|---|---|---|
| `:25` 域名常量 | 无独立常量，走 `experiment.D1S1ActionDomain` | `d1StaticEQDomain = "static_eq"` |
| `:26` 动作种常量 | `experiment.D1S1ActionKind`（"track_gain_adjust"，定义在 experiment 包） | `d1StaticEQKind = "static_eq_band_adjust"` |
| `:32` CapabilityID | `staticBalanceCapabilityID`（**定义在文件外** `chat/capability_runtime_canary.go:23`，值 `static_mix.static_balance.v0`） | `d1StaticEQCapabilityID = "static_mix.static_eq.v0"`（audit-only，无 registry 注册，见 `:28-31` 注释） |
| `:38` 插件标识默认值 | 不适用（无插件概念） | `defaultD1StaticEQPluginIdentifier = "juce_eq"`（早窗要点 1 的占位符） |

### 1.2 `d1JournalMutationPort`（journal Command 形状）

| 锚点 | track_gain 取值 | static_eq 取值 |
|---|---|---|
| `:51` | `staticEQ bool` 零值 | `staticEQ: true`（由 `executeD1StaticEQ:386` 注入） |
| `:60-68` vs `:69-76` journal record | Summary `"D1-S1 bounded track gain adjustment"`；Tool/CommandName `track_gain_adjust`；Command `{cmd: "set_volume", track_id, db: Args["target_db"]}` | Summary `"D2-1 bounded static EQ band adjustment"`；Tool/CommandName `set_plugin_param`；Command `{cmd: "set_plugin_param", track_id, plugin_id: Args["plugin_id"]‖Args["plugin_identifier"], param_id, value: Args["target_value"]}`（注意：plan 只放 `plugin_identifier`，故此处 plugin_id 键实际承载 identifier 值） |
| 两域相同 | Domain `"daw"`、Source `"free_state_d1_s1"`、RiskLevel `"confirm"`、RequiresConfirmation、ConfirmationStatus、Status 流转 | 同左 |

### 1.3 `d1TrackGainPlan`（:87-137）vs `d1StaticEQPlan`（:143-203）逐项差异

| 差异项 | track_gain（行号） | static_eq（行号） |
|---|---|---|
| 域/_kind 校验 | 无显式域校验；仅 `candidate.Operation == experiment.D1S1ActionKind`（`:94`） | 额外查域表：`D1S1DomainSpecFor` 且 `spec.ActionDomain/ActionKind` 等于 `:25/:26` 常量（`:150-152`） |
| 候选参数校验 | TrackID 非空 + `DeltaDB != 0` 且 `|DeltaDB| ≤ 2`（`:94`） | 仅 TrackID 非空（`:153`）；增益界在 admission 侧查 `gain_db`（`:156-159`） |
| admitted 值匹配 | `candidate.DeltaDB` vs admitted `delta_db`，容差 0.0001（`:97-100`） | 不匹配候选（候选不带增益），只读 admitted `gain_db`（`:156`） |
| Before 值读取 | 计划层从状态快照读 `currentDB`（`d1TrackGain:105-108`，helper `:205-213`，键序 `volume_db/fader_db/gain_db/db`），读不到即阻塞 | 计划层不读（注释 `:139-142`）；由端口 Preflight 读（见 §4） |
| 参数组装 | 无 | `bandIndex`（TypedAction `band_index`，默认 0）→ `paramID = "band_%d_gain"`（`:164-170`）；`pluginIdentifier` = TypedAction `plugin_identifier` ‖ `juce_eq` 默认（`:171-174`） |
| TargetFingerprints | `track:%s:fader:%g`（`:115`） | `track:%s:eq:%s:pending`（`:181`） |
| ContractVersions | `{"free_state:d1_s1", "action:track_gain_adjust"}`（`:116`） | `{"free_state:d1_s1", "action:static_eq_band_adjust"}`（`:182`） |
| BeforeFingerprint | `track:%s:fader_db:%g`（`:123`） | `track:%s:eq:%s:pending`（`:189`，"pending" 因为 instantiate 前不可知） |
| Args 键 | `{delta_db, target_db}`（`:124`） | `{plugin_identifier, param_id, target_value}`（`:190`） |
| Action.Command | `experiment.D1S1ActionKind`（`:122`） | `d1StaticEQKind`（`:188`） |
| actionID 后缀 | `"_gain"`（`:121`） | `"_eq"`（`:187`） |
| ActionSet/Proposal CapabilityID | `staticBalanceCapabilityID`（`:125`、`:134`） | `d1StaticEQCapabilityID`（`:191`、`:200`） |
| 两域相同 | sessionID/actionSetID/proposalID/contextBundleID 组装、round/previousObservationID 回溯（`:121-135` ↔ `:187-201`）、TargetScope/Risk/VerificationRef/CapabilityVer `"v0"` | 同左 |

### 1.4 `executeD1TrackGain`（:221-305）vs `executeD1StaticEQ`（:311-395）

两函数的 session 加载/重启恢复/before render 校验/snapshot/authorize 块逐字节等价（`:222-289` ↔ `:312-379`）。仅三处差异：

| 差异点 | track_gain（行号） | static_eq（行号） |
|---|---|---|
| plan 函数调用 | `d1TrackGainPlan`（`:263`） | `d1StaticEQPlan`（`:353`） |
| verifier 构造 | `executionverifiers.StaticBalance{...}`（`:290-291`，Acoustic 参数两域同构） | `executionverifiers.StaticEQ{...}`（`:380-381`） |
| 端口选择 | Reconcile 用 `StaticBalanceVSPPort`（`:294`）；Execute 用 journal port 包 `StaticBalanceVSPPort`、无 `staticEQ`（`:296`） | Reconcile 用 `StaticEQVSPPort`（`:384`）；Execute 用 journal port 包 `StaticEQVSPPort`、`staticEQ: true`（`:386`） |

### 1.5 `projectD1Execution`（:397-452，两域共享）

| 锚点 | 说明 |
|---|---|
| `:407-408` | admissionDomain/admissionKind 从 `TypedAction` 投影、回落 `experiment.D1S1*` 常量——已是数据驱动 |
| `:417-418` | `RequestedViewIDs/ExecutedViewIDs` 硬编码 `"mix.multitrack_relationship"`；两域现值相同，但这是按域可变点（第三域若需不同 CCB 视图须覆盖） |
| `:441-444` | 人类回复文案按域二分支：默认 track_gain 文案；`admissionDomain == d1StaticEQDomain` 时换 static_eq 文案 |
| `:435-440` | `parameter_applied/readback_verified` 等回执字段两域同键消费（Details 键名差异见 §4） |

`d1BlockedResponse`（:454-457）域无关。

## 2. `agent/internal/chat/mix_tick_confirmation.go`（1225 行）

| 锚点 | 差异类别 | track_gain / 通用取值 | static_eq 取值 |
|---|---|---|---|
| `:177-190` | 域分发点 | static_eq 判定在 `:180`（读 `Admission.TypedAction` 的 `action_domain/domain` 与 `d1StaticEQDomain` EqualFold）→ 走 `executeD1StaticEQ`（`:181`）；否则落到通用 `executeD1TrackGain`（`:187`）。注意分发键是 admission 域而非 candidate.Operation | 同左（命中 `:180` 分支） |
| `:370-374` | Operation 白名单 | `validatePendingMixTickCandidate` switch 允许 `track_gain_adjust, track_pan_adjust, track_pan_set, d1StaticEQKind`；错误文案 `:373` 同步枚举四个名字（文案与白名单双处硬编码） | `d1StaticEQKind` 已在列 |
| `:378-391` | 按操作参数界 | track_gain 查 `delta_db` ±2、pan 两域各有界 | **无 static_eq case**（增益界由 plan 层 `:156-159` 把守；此处对 static_eq 不查参数） |
| `:392-394` | 重复兜底 | track_gain 的 DeltaDB==0 再拒一次（static_eq 不经过） | 不适用 |
| `:472-487` | 人类摘要文案 | `pendingMixTickHumanSummary`：pan 两 case + default 电平文案（`:485`） | `case d1StaticEQKind`（`:482-483`）输出"有界的静态 EQ 频段增益调整" |
| `:489-491` | 事件正文 | 域无关（包装 humanSummary） | 同左 |
| `:493-505` | 动作短文案 | `pendingMixTickEventActionText` 无 static_eq case，落入 default 电平文案 | **缺 case**（该函数未发现非测试调用方，影响待核） |
| `:515-537` | 事件 payload | `pendingMixTickEventPayload` default 分支放 `delta_db` | static_eq 落 default，payload 带无意义的 `delta_db`（cosmetic） |
| `:120-160` | 上下文重建 | `pendingMixTickCandidateFromContext`：Operation 缺失时默认 `"track_gain_adjust"`（`:155-157`）——按域可变的默认值 | 同一默认（static_eq 候选重建依赖 payload 里带 operation） |
| `:640-688` | 下一候选生成 | `nextPendingMixTickCandidateFromReobserve` 硬编码 `Operation: "track_gain_adjust"`（`:677`）与 `DeltaDB: -1`（`:680`）：链式建议只产 track_gain | 不产出 static_eq |

## 3. `agent/internal/chat/free_state_experiment_runtime.go`（511 行）

`freeStateExperimentAdmission`（:20-86）内的按域构建分支：

| 锚点 | track_gain 取值 | static_eq 取值 |
|---|---|---|
| `:28-30` | 域表准入门（两域共用，已表驱动）：`D1S1DomainSpecFor` 要求 domain+kind 双匹配，否则拒绝并列出 `D1S1AdmittedDomains()` | 同左 |
| `:43` | `budget = 1`（域无关） | 同左 |
| `:44-45` TypedAction | `{action_domain, action_kind, target_db: ParameterBounds["target_db"], delta_db}`（delta_db 从 `delta_db/db_delta/gain_delta_db` 三键取） | — |
| `:46-47` dose bounds | diagnostic/retained 两份 `{source, bounds, delta_db, max_action_attempts: 1}` | — |
| `:48` 分支条件 | — | `EqualFold(proposal.ActionDomain, d1StaticEQDomain)` |
| `:53-59` TypedAction | — | `{action_domain, action_kind, gain_db}`；再从 ParameterBounds 透传可选 `frequency_hz/q/band_index/plugin_identifier`（`:55-59`，透传键清单硬编码） |
| `:60-61` dose bounds | — | 同形但 `gain_db` 替代 `delta_db` |
| `:79-85` | `ValidateD1S1` + `validateD1FreshObservedTarget`（域无关） | 同左 |

即：TypedAction 组装、剂量键名（`delta_db` vs `gain_db`）、透传键清单三处按域硬编码；其余（checkpoint、authority、预算、验证）域无关。

## 4. `agent/internal/executionports/staticbalance_vsp.go`（220 行）vs `staticeq_vsp.go`（299 行）

### 4.1 动作校验（Preflight 前半）

| 锚点 | StaticBalanceVSPPort | StaticEQVSPPort |
|---|---|---|
| 命令名校验 | `action.Command != "track_gain_adjust"`（`staticbalance_vsp.go:47`） | `action.Command != "static_eq_band_adjust"`（`staticeq_vsp.go:45`，任务卡点名的硬编码点） |
| 参数存在性 | numeric `target_db`（`:50-52`） | numeric `target_value`（`:48-50`）+ 非空 `param_id`（`:51-53`）+ `plugin_id` 或 `plugin_identifier`（`:54-56`） |
| 公共前置 | Client 非空、cut 可执行且哈希匹配、baseRevision > 0、动作集非空、快照 epoch/revision 匹配（两文件逐字等价：`sb:32-63` ↔ `se:31-64`） | 同左 |

### 4.2 preflight before 值读取

| 锚点 | track_gain | static_eq |
|---|---|---|
| 读取方式 | VSP 状态快照 `tracks` 行取 fader：`trackVolume`（`staticbalance_vsp.go:65-69`，helper `:196-213`，键序 `volume_db/fader_db/gain_db/db`） | 仅当 action 已带 `plugin_id` 时经 `readPluginParam`（get_plugin_parameters）读（`staticeq_vsp.go:67-80`）；待 instantiate 的动作 before 记为缺失（`:69-73`） |
| 存储键 | `beforeGainDB[targetRef]`（track 维） | `beforeValues[eqBeforeKey(pluginID,paramID)]`（`plugin\x1fparam` 维，`:288-290`）+ `pluginIDByRef[actionID]`（`:27`） |

### 4.3 Apply 命令序列与回执

| 锚点 | track_gain（staticbalance_vsp.go） | static_eq（staticeq_vsp.go） |
|---|---|---|
| 命令序列 | 单命令 `set_volume {track_id, db: target_db, base_revision}`（`:89-94`） | 可选 `instantiate_plugin {track_id, plugin_identifier, base_revision}`（`:111-132`，幂等键后缀 `":instantiate"`）→ `set_plugin_param {track_id, plugin_id, param_id, value: 原始 dB 目标, base_revision}`（`:133-148`）→（值语义缺口，早窗要点 2） |
| 回读 | 状态快照 `trackVolume` 比对目标，容差 0.001（`:103-116`） | `readPluginParamUnlocked`（get_plugin_parameters）比对，容差 0.001（`:159-161`）；取值序 `value/current_value/normalized_value` → 解析 `value_text`（`:263-272`） |
| Details 键 | `before_revision, after_revision, transaction_id, idempotency_key, requested_target_db, actual_readback_db, readback_verified, before_readback_db, before_readback_available`（`:130-135`） | `before_revision, after_revision, transaction_id, idempotency_key, plugin_id, param_id, requested_target_value, actual_readback_value, readback_verified, plugin_instantiated_by_action` + 条件 `before_readback_value`（`:172-181`；**无** `before_readback_available`） |
| 公共不变量 | 单幂等键（`tx_` 前缀）、applied_unreconciled 三出口（快照失败/epoch 变/revision 未推进）、EffectivelyOnce、EvidenceRefs 形状 | 同左（`se:104-108,149-158`） |
| 读命令幂等键 | 不适用 | 自成一套 `"read:track:plugin:param"` / `"tx_read:plugin"`（`:249`） |

### 4.4 Reconcile

| 锚点 | track_gain（`:139-172`） | static_eq（`:192-233`） |
|---|---|---|
| 回读源 | 状态快照 `trackVolume` | `readPluginParam`；**action 无持久 `plugin_id` 时直接判 `not_applied`**（`:211-216`，注释明言不可从持久化 action 复核） |
| Details | 同 Apply 族 + `reconciled: true` | 同 Apply 族 + `reconciled: true` |

### 4.5 共享/独有 helper

- `numeric`（`sb:174-190`）、`trackRows`（`sb:215-220`）：域无关，位于 staticbalance 文件内（static_eq 复用）。
- `actionArgText`（`se:280-286`）、`eqBeforeKey`（`se:288-290`）、`firstReplyText`（`se:292-299`）：static_eq 独有。
- `trackVolume`（`sb:196-213`）：track_gain 族语义（track fader 键序）。

## 5. `agent/internal/executionverifiers/staticbalance.go`（161 行）vs `staticeq.go`（149 行）

| 差异项 | StaticBalance（staticbalance.go） | StaticEQ（staticeq.go） |
|---|---|---|
| 声学接口 | `AcousticVerifier{VerifyStaticBalance}`（`:20-22`） | `AcousticVerifierEQ{VerifyStaticEQ}`（`:28-30`）——为措辞单独立接口 |
| 结构断言（`:59-81` ↔ `:47-68`） | **快照驱动**：读 VSP state、索引 tracks、断言目标轨存在、`Args["target_db"]` vs 轨行 `volume_db/fader_db/gain_db/db`，容差 0.001 | **回执驱动**：插件参数不可见于 VSP 快照（注释 `:14-20`），断言 `receipt.Details["actual_readback_value"]` vs `Args["target_value"]`（容差 0.001）+ `readback_verified == true` + `AppliedRevision == 当前 state.Revision`（`:63-67`） |
| 结构证据引用 | `"vsp.state.snapshot:postcondition"`（`:82`） | `"vsp.receipt.readback:postcondition"`（`:70`） |
| 声学视图绑定 | `HarnessAcoustic.VerifyStaticBalance` → `verifyFreshMOM(..., "static_level_relationship", verifyStaticBalanceMOM)`（**harness_acoustic.go:35-36**） | `HarnessAcoustic.VerifyStaticEQ` → `verifyFreshMOM(..., "static_level_relationship", verifyStaticEQMOM)`（`staticeq.go:104-106`） |
| fail 重试 | VerifyStaticBalance 在 relationship=fail 时追加一次独立 fresh MOM 复核（harness_acoustic.go:37-60，防 B1 收敛抖动） | **无此重试**（`:104-106` 直接返回）——两域行为差异，不止措辞 |
| MOM policy 函数 | `verifyStaticBalanceMOM`（harness_acoustic.go:337-375） | `verifyStaticEQMOM`（staticeq.go:111-149）：逻辑逐分支等价（ready/approx/partial 门、≥2 轨、95% usable 覆盖、目标轨在列），**唯一差异是文案前缀 "B2" vs "D2-1"** |
| 其余 | 回执覆盖/状态检查、Acoustic 结果装配、pass/fail/inconclusive 折叠（`:44-58, 83-108` ↔ `:33-46, 71-96`）逐字等价 | 同左 |

真正的每家族差异（任务卡判断的锚点化）：视图键（两域恰合同为 `static_level_relationship`，压缩族将换 dynamics/COM 视图）、policy 函数实例、fail 重试有无、结构断言模式（快照驱动 vs 回执驱动）。

## 6. 文件外关联锚点（目标文件直接引用，改表驱动时会碰到）

| 锚点 | 内容 |
|---|---|
| `experiment/d1s1_domains.go:14-27` | `D1S1DomainSpec` 现有字段仅 `ActionDomain/ActionKind/PromptParameterHint/ValidateTypedAction/ValidateDoseBounds`——执行层字段（参数绑定、fingerprint 模板、journal 命令形状、contract 后缀、声学视图绑定）尚不存在，是 S1 改动清单 1 的落点 |
| `experiment/d1s1_domains.go:29-76` | 域表两行：track_gain（`:30-41`，无 ValidateTypedAction）与 static_eq（`:42-75`） |
| `experiment/d1s1_domains.go:98-107` | `D1S1DomainSpecFor`：domain+kind 双匹配解析 |
| `chat/capability_runtime_canary.go:23` | `staticBalanceCapabilityID = "static_mix.static_balance.v0"`（track_gain 的 CapabilityID 定义处，`free_state_d1_runtime.go` 只引用） |
| `chat/free_state_d1_receipt.go:123-127` | D1 回执投影 `action_domain/action_kind` 从 TypedAction 取（早窗要点 5 已修 bug 的锁定点）；`syncD1Receipt` 其余域无关 |

## 7. 汇总：新增第三域的触达面（按文件计）

| 文件 | 必须改或表驱动覆盖的点数 |
|---|---|
| `chat/free_state_d1_runtime.go` | 常量 4（§1.1）+ journal 形状 1（§1.2）+ plan 差异 13（§1.3）+ execute 差异 3（§1.4）+ projectD1Execution 域分支 2（`:417-418` 视图、`:441-444` 文案）≈ 23 |
| `chat/mix_tick_confirmation.go` | 白名单+文案 2（`:371/:373`、`:482`）+ 分发判定 1（`:180`）+ 缺口 2（`:493-505` 短文案缺 case、`:378-391` 无参数界 case）+ 默认值/payload 2（`:155-157`、`:515-537`）≈ 7 |
| `chat/free_state_experiment_runtime.go` | TypedAction 组装、剂量键名、透传键清单 3（`:44-61`） |
| `executionports/staticeq_vsp.go` | 动作名校验 1（`:45`）+ 参数存在性清单 1（`:48-56`）+ 命令序列/幂等键后缀 2（`:111-148`）+ Details 键族 1（`:172-181`）+ before 读取策略 1（`:67-80`）+ Reconcile plugin_id 前提 1（`:211-216`）≈ 7（staticbalance_vsp.go 按任务卡原样保留） |
| `executionverifiers/staticeq.go` | 结构断言模式选择 1（`:47-68`）+ 声学视图绑定/policy 2（`:104-106`、`:111-149`）+ fail 重试有无 1 ≈ 4（其中视图绑定与 policy 是压缩族必然变化的家族差异） |
| `experiment/d1s1_domains.go` | 域表新增一行 + S1 扩展的执行描述字段（§6） |

—— 完 ——
