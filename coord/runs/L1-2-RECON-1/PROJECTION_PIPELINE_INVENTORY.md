# 投影计算触发链与依赖图盘点（PROJECTION_PIPELINE_INVENTORY）

- 卡片：L1-2-RECON-1（纯只读勘察，零代码改动）
- 执行侧：mac 会话 / 分支 port/l1-2-recon-1-proj-inventory
- 勘察基线：HEAD=9b85b9f（origin/main=297021b + 本卡领取提交）；静态读码，四路并行勘察 + 承重锚点逐条亲验（抽验 25+ 处全部命中，含 catalog.go 装配顺序 / F5 铃标注 / shadow 受影响域表 / rlm 三源合并 / DAD gate 轮询等）
- 用途：路线图 D4（事件驱动物化+依赖图增量失效）的现状事实清单；G2 质量门（失效正确性专项）的输入
- 拓扑前提：`docs/OBSERVATION_PROJECTION_MANIFEST.md`——九投影为 DAD 之上同级 peer，禁止写成链；本报告所有"依赖边"指数据消费关系（谁读谁的什么），不构成管线

## 0. 执行摘要（一句话版）

**当前不存在任何"事件驱动的投影重算"：九投影全部是请求驱动的惰性现算；内核事件只刷新输入缓存（feature snapshot 文件 / shadow 内存镜像），投影要等下一次观察请求才重建。** 失效机制已存在但分层各异——写侧新鲜度标注（F5 铃）、观察台账按受影响域打 stale、revision 三元组指纹、L2 probe 状态指纹缓存——这些是物化层可直接复用的种子；缺口在于"投影级"的脏标记与"事件→依赖图→受影响投影"的传播链尚不存在（scope 映射表已存在于 shadow 层，见 §5.M4）。

---

## 1. 计算触发链矩阵（九投影 + DAD）

### 1.1 矩阵总表

| 投影 | 触发模式 | 计算入口（文件:行） | 计算时机 | 结果去向 |
|---|---|---|---|---|
| DOM | 请求驱动 | `dom.Build` ← `finalizeDOMProjection` `agent/internal/mixboard/dom_projection.go:11`（Build 在 :16） | mix.observe / ccb.observation_request 请求内 | 挂 `ObservationPacket.DOMProjection` → obs JSON 落盘 → mix_read / CCB bundle |
| MOM | 请求驱动 | `mom.Build` ← `finalizeMOMProjection` `agent/internal/mixboard/mixboard.go:3952`（Build 在 :3956） | 同上 | `obs.MOMProjection` → 同上 |
| TIM | 请求驱动 + 导入回执双路径 | `tim.Build` ← `finalizeTIMProjection` `mixboard.go:3960`（Build :3964）；`tim.BuildFromImportRows` ← `messageLoopTIMProjectionForImportResult` `agent/internal/agentloop/message_loop.go:8535`（Build :8546）与 harness `publicStemsImportResult` `agent/internal/harness/harness.go:11790`（Build :11835） | 观察请求内 / stems 导入工具回执时 / DAD gate 轮询刷新 | obs 落盘 / `result["tim_projection"]` memo 进对话上下文 |
| TOM | 导入回执 + capability 上下文 | `tom.BuildFromImportRows` ← `messageLoopTOMProjectionForImportResult` `message_loop.go:8556`（Build :8567）；`capabilitycontext.BuildProjectTOMProjection` `agent/internal/capabilitycontext/project_tom.go:14`（Build :19） | 导入回执 / B2/B3/C1 canary 上下文获取时 | `result["tom_projection"]` / executionMemory.PendingTrackOrganization `agentloop/execution_memory.go:331` |
| FXM | 请求驱动（测量随请求携带） | `fxm.Build` ← `finalizeFXMProjection` `mixboard.go:3968`（Build :3985；输入几乎全来自 `req.Args["fxm_measurement"]` :3973-3977） | 观察请求内 | `obs.FXMProjection` → 落盘 |
| COM | 请求驱动 + 用户确认执行链 | `com.Build` ← `finalizeCOMProjection` `agent/internal/mixboard/com_projection.go:14`（Build :40-49）；`executeSemanticCompressorTicket` `agent/internal/chat/semantic_compressor_execution.go:575`（change_delta Build :659） | 观察请求内（paired_io/source_only/change_delta）/ 压缩器执行票据确认后 | obs 落盘 / 执行 receipt `com_evaluation`（:663，不落盘） |
| EPM | 导入回执 | `epm.BuildFromImportRows` ← `messageLoopEPMProjectionForImportResult` `message_loop.go:8578`（Build :8589） | stems 导入回执时 | `result["epm_projection"]` / executionMemory.PendingSectionMarkers `execution_memory.go:520` |
| RLM | capability preflight + 确认后链 | `rlm.Build` ← `BuildGainStagingPack` `agent/internal/capabilitycontext/gain_staging.go:80`（Build :87-93） | B1 意图命中时每 run 首轮 preflight（`agentloop/static_mix_gain_staging_context.go:17-23`，message_loop.go:432 调）/ B1/B1.2 确认后（:1069/:1277） | `<capability_context_pack>` 注入对话（LLM 上下文）；内容哈希 ID `rlm/projection.go:107` |
| acousticpackage | 请求驱动 read-first + 后台补采 | `acousticpackage.BuildStatus` `agent/internal/acousticpackage/status.go:337` ← harness `prepareMixObservationAcousticPackage` `harness.go:3642-3666`；后台补采 `requestMixObservationFeaturesBackground` `harness.go:4205`（触发判定 `mixObservationProjectBackgroundFillNeeded` :6895） | 每次观察请求装配时 read-first；L3 缺/stale 时低优先级后台采集 | 落盘 `acoustic_package_status.json`（schema `acoustic_package_status.v0`，status.go:18）→ MOM/DOM/TIM/CCB 消费 |
| masking（观察） | 请求驱动（CCB masking view 专用） | `prepareCCBMaskingObservation` `agent/internal/harness/ccb_masking_observation.go:30` → `CollectL2RenderProbeBatch` `agent/internal/harness/l2_probe_batch.go:44` | 请求 view 含 `mix.masking_relationship` 时（`harness/ccb_observation.go:59-63`） | probe 帧 → feature snapshot → MOM masking 输入 |
| DAD/audioclosure | 事件溯源状态机（外部驱动） | `Driver.*` 全家 `agent/internal/audioclosure/driver.go:24-450`；事件重放 `Fold`/`appendEvent` `audioclosure/events.go:386`（全量重放） | 每个 `/agent/chat` 回合（`chat/goalrunner_chat.go:267/:343`）+ continuation scheduler 250ms tick 续跑（`chat/continuation_scheduler.go:150`） | `MemoryStore`（store.go:18，无磁盘日志）+ workspace Snapshot/Restore（`chat/server.go:6993/:7145`） |
| DAD/frequencycleanup | 请求驱动 read-first 库 | `BuildModel`/`Analyze` `agent/internal/frequencycleanup/model.go:16/:46` ← `BuildFrequencyCleanupPack` `capabilitycontext/frequency_cleanup.go:67`（:82-83 调用） | C1 能力会话命中时（`chat/c1_frequency_cleanup_runtime.go:57`）；明确 0 次 render（:113-114） | WorkflowData / orchestration session / mixboard 决策投影 |

### 1.2 请求驱动主链（五投影共用装配管线）

统一编排点 `FinalizeObservationContext` `agent/internal/mixboard/catalog.go:59-86`（亲验）：

1. `req.Args["dom_mode"]` 非空 → 先 `finalizeDOMProjection`（:66-69）；
2. band/stereo 模式 → `applyBandStereoProjection`（:70-72，重写 Global/Mix/Deep 包并清 TimelineDigest，mixboard.go:3946-3949）；
3. 顺序 `finalizeMOMProjection` → `finalizeTIMProjection` → `finalizeFXMProjection`（:73-75）；
4. `finalizeCOMProjection`（:76，`com_mode` 为空直接 return，com_projection.go:18-21）；
5. 无 DOM 无 band/stereo → 兜底再跑一次 DOM（走默认 ModeSourceOnly，:77-79，dom_projection.go:21-24）；
6. `BuildDigest` + `BuildCatalog`（:80-81）。

`FinalizeObservationContext` 只有两个调用方：`Store.RequestObservation`（`mixboard.go:1089`，落盘主路径）与 `BuildObservation`（`mixboard.go:1290` 纯内存，DAD gate 刷新用）。`RequestObservation` 的输入装配（mixboard.go:1085-1089，亲验）：`readPreviousObservation`（上一份 obs，供 before/after delta）→ `loadFeatureSnapshot`（feature snapshot 文件）→ `buildObservation` → `applyBeforeAfterDelta` → finalize。

**触发者全集**（谁发 mix.observe / ccb.observation_request）：
- HTTP 工具面：`POST /agent/invoke` → `harness.Invoke` case `"mix_observe","mix_request_observation"`（`harness.go:2424-2426`，亲验）与 case `"ccb_observation_request"`（:2438-2440 → `harness/ccb_observation.go:20`）；
- LLM 会话轮：agentloop 执行器发 mix.observe 工具调用（`chat/goalrunner_chat.go:780-831`）；
- mix session 轮：`requestMixObservationRound`（`chat/mix_session_workflow.go:3010`）被 revise/tuning/initial-observation 四处调用；
- CCB 路由：free-state 模型逐轮显式请求（`ccb_model_prompt.go:62-179`，`free_state_reasoning.go:959-997` 校验，reject+guide 不自动选 view，ccb_observation.go:42-56）；
- 离线 CLI：`cmd/mixlab/main.go:103`、`cmd/comderive/main.go:28`、`cmd/domreadiness/main.go:49`。

**无轮询**：未发现任何定时器/Ticker 周期性重建投影；唯一周期结构是 audioclosure 的 continuation scheduler（250ms，驱动状态机续跑而非投影重算）与 DAD gate 的轮询等待（见 §3.E9）。

### 1.3 事件只到输入缓存为止（现状关键事实）

内核遥测事件驱动的是**输入缓存刷新**，不是投影计算：ZMQ SUB → `bridge.normalizeTelemetry`（`agent/internal/bridge/bridge.go:400-430`，亲验）→ `TelemetryHook`（`cmd/vitagent/main.go:65` 注入 `chatServer.HandleKernelTelemetry`）→ `Harness.IngestKernelTelemetry`（`harness.go:3890-3906`，亲验，仅 4 个分支：render topic / waveform / spectral tile / L3 acoustic）→ `writeMixboardReady*Snapshot` 家族写 feature snapshot 文件（`harness.go:6021+`，原子写 :7155-7174）。投影等下一次请求读该文件才算。这是 D4 要改变的核心现状。

---

## 2. 投影间依赖边清单（17 条，两端锚点）

标注约定：**显式**=代码里可指的调用/字段传递；**时序隐式**=下游读到的数据新鲜度依赖上游写入顺序，附时序证据。

### 2.1 peer 投影间（manifest 允许的显式可选引用）

- **E1（显式·可选）TOM ← TIM**：`tom.ImportInput.TIMProjection` 传值 `message_loop.go:8570`；消费端 `tom.BuildLLMContext(proj, timProjection ...map[string]any)` `agent/internal/tom/projection.go:951`（亲验），压缩为一条 `tim_projection` digest fact。manifest §4：缺失不影响 TOM readiness。
- **E2（显式·仅 import 输入）EPM ← TIM+TOM**：`epm.ImportInput.TIMProjection/TOMProjection` 传值 `message_loop.go:8592-8593`（亲验）；manifest §4 确认不进 EPM LLMContext，不构成判定依赖。

### 2.2 投影 ← 数据源/证据

- **E3（显式）MOM ← acousticpackage 状态**：`mom.sourceIdentity` 优先读 `AcousticPackageStatus.source_identity`（`mom/projection.go:881-892`，亲验）；装配侧 `readFrequencyAcousticSnapshot` `mixboard.go:495-538` 从 store 落盘文件读。revision 三元组（source/clip/render）进 `TrustQuality`（projection.go:431-480）。
- **E4（显式+时序隐式）DOM/MOM/TIM/COM ← feature snapshot 文件**：读端 `loadFeatureSnapshot` `mixboard.go:1086`；写端遥测事件链（§1.3）。时序风险已由 F5 前台铃治理（§5.M1）：行级 freshness 由写侧同一笔原子写入标注，读侧不推断。
- **E5（显式）COM paired ← 内核双 tap 工件**：`readCOMPairedArtifact(req.Args["com_artifact_path"])` `com_projection.go:33`；工件由 `prepareMixObservationCOMEvidence` 现场采集（内核命令 `compressor_dual_tap_probe`，`harness/com_observation.go:20/:39/:243`），路径 `VitApp/Workspace/Artifacts/com_evidence/<pairID>/<pairID>.json`（:295-302）。配对原理：`PairedEnvelopeFrame` 每帧同时携带 Input/Output Peak/RMS（`com/paired_types.go:63-70`）——bypass/processed 对齐发生在工件内部，非两次 render 外部拼接；`PairedConditions` 用 RenderRevision+Determinism proof 保证可比（:33-60）。
- **E6（显式+锚定校验）COM change_delta ← 写前/写后两次 paired 投影**：`semantic_compressor_execution.go:602/:648` 参数写入前后各采集一次，`:659` 内联 `ChangeDeltaInput{Before, After}` Build。跨窗锚定靠 `PairedEvidenceExpectation{TrackID, ClipID, PluginInstanceID, TopologyGeneration, StartSample, EndSample}` 校验（com_observation.go:77-86），且 chat 侧强制精确采样窗（semantic_compressor_execution.go:391-393）。
- **E7（显式）FXM ← 请求携带测量**：`fxm.Input` 几乎全来自 `req.Args["fxm_measurement"]` JSON 反序列化（`mixboard.go:3972-3978`，亲验），obs 只补 ObservationID/MixSessionID/CreatedAt/TargetRef（:3979-3983）。A/B 锚定在测量自带的 render 身份里，不在装配层。
- **E8（显式+时序隐式⚠）RLM ← 三源工具结果合并**：`buildReferenceRows` 固定顺序合并 `project.state:tracks` → `mix.observe:project_package.tracks` → `project.audio_analysis_status:track_waveform_envelopes`（`rlm/projection.go:128-134`，亲验）。**时序证据**：`mergeSourceRows`（projection.go:170-192）对同 key 行做字段级覆盖合并（`mergeMap(out[idx].Data, row.Data)` :180），后组合覆盖先组合，**无任何 revision/新鲜度比较**——三个来源是不同时刻抓取的 `state.executed` 工具结果（装配于 `static_mix_gain_staging_context.go:114-123`），若工程在 mix.observe 之后又变更，RLM 行会混拼两个 revision 的字段且无标注。这是 G2 失效正确性专项应重点覆盖的既有隐患。
- **E9（时序隐式，代码显式治理）TIM 导入投影 ← DAD 声学行**：时序证据链——① 导入回执先到：`publicStemsImportResult`（harness.go:11790，Build :11835）在内核导入命令应答内即时建 TIM；② 此时声学必缺：初始行 `acoustic.status=missing, reason="dad_acoustic_analysis_pending_after_import"`（`tim/import.go:58-64`，亲验）；③ DAD 分析后到：`dad_readiness_gate` 轮询 `project.audio_analysis_status`（`message_loop.go:7548-7593`，亲验：合并 :7585、刷新 :7589、ready 判定 :7555）；④ 覆盖度比较替换：`messageLoopRefreshImportTIMFromDADSnapshot`（:7815-7828）+ `messageLoopTIMProjectionShouldReplace`（:7830-7849，ready 数/覆盖数/总数三序比较）。整条"等上游"逻辑是显式编码的，但它活在 agentloop 会话流里，不是可复用的失效基础设施。
- **E10（显式·时间边）observation before_after ← 上一份 observation**：`readPreviousObservation` `mixboard.go:1085` → `applyBeforeAfterDelta` :1088（亲验）。跨观察的 delta 边，比较门槛在同 tap+render_revision（`abResultQualityGate` mixboard.go:3335-3336）。

### 2.3 观察装配/失效链

- **E11（显式）CCB view ← 已落盘 observation**：`ccbObservationRequest`（`harness/ccb_observation.go:20-101`，亲验）：无 observation_id 先 `requestMixObservation` 物化（:65）→ `store.Read` binding（:77）→ `store.Read` view keys（:88，`FreeStateObservationReadKeys` `capabilitycontext/free_state_observation.go:252-264`）→ `AssembleFreeStateObservation`（:97）。**CCB 只装配不重建投影**——投影的现算全部发生在 mixboard 物化那一步。
- **E12（显式·失效传播）free-state 观察台账 ← shadow ChangeReceipt**：变更 receipt 的 `affected_scopes` 命中即把台账 view 行打 `stale + invalidated_by_change_id`（`chat/free_state_reasoning_loop.go:1725-1759`，亲验；触发点 :2366-2368 与 `chat/audition_candidate_adoption.go:527`）。**这是现存唯一一条"变更→受影响观察面"的传播链，即 D4 脏传播的雏形**（scope 映射表见 §5.M4）。
- **E13（显式·跨时间缓存边）acousticpackage ← 落盘旧包**：`MergeStatus(stored, derived)`（`status.go:394-421`，亲验）derived 为基底，stored 仅在 `shouldMergeStoredFeature`（:423-452：非空、非 stale 实时层、身份不 mismatch、优先级/完整度不劣）时回填；Upsert 时同 target 异 revision 旧包直接 `MarkStale`（:214-221）。
- **E14（显式+两层新鲜度）投影输入 ← shadow 镜像**：delta_update 到达即 `shadow.ApplyDelta`（`bridge.go:405-408`，亲验）但 receipt 保持 `pending_authoritative_refresh`，直到权威快照确认升 `current_snapshot`（`shadow/shadow.go:110-130`）；观察前强制 `refreshShadowWithStatus` 全量背书（harness.go:3286/3317/12456-12536）。
- **E15（显式）frequencycleanup ← L2 probe + 声学快照（read-first）**：`AssembleFrequencyContext` `mixboard.go:187` 只装配已物化证据；target baseline 经 `CollectL2RenderProbeBatch`（`harness/l2_probe_batch.go:44`，缓存键=track_state_fingerprint :183），验证阶段 `ForceFresh:true` 重采（c1_frequency_cleanup_runtime.go:501）。
- **E16（时序隐式⚠）B1 pack ← 会话内记忆的工具结果**：pack 三源取 `state.executed` 最近结果（`static_mix_gain_staging_context.go:114-123`），同 run 只建一次（:21-23 + `messageLoopHasGainStagingContextPack` :1847-1857）。**时序证据**：三源各自抓取时刻不同，建包后工程变更不触发重建（无失效钩子），pack 生命周期=run 生命周期——与 E8 同族的"记忆型输入无新鲜度门"。
- **E17（显式）masking 观察 ← L2 probe 批**：`ccb_masking_observation.go:30` / `chat/b4_eq_runtime.go:334` 同走 probe 批，帧级 masking_frames 随 probe payload（harness.go:5426-5431 过滤 `l2_render_probe_ready`）。

---

## 3. 变更事件目录（内核→agent，19 项）

通道盘点：**无共享内存事件、无 HTTP 推送**。三条通路——① ZMQ SUB `tcp://127.0.0.1:5556` 遥测 pub-sub（订阅者：bridge `bridge.go:250`、vsphub 中继 `vsphub/telemetry.go:13`、harness 临时采集器 `harness.go:5383`）；② ZMQ REQ `:5555` 命令应答内嵌 revision/resync_hint（`kernel/client.go:22`）；③ VSP Hub HTTP `:8787`（`vsphub/config.go:10`，websocket 分发面）。SHM 只载特征 float 载荷（`vsphub/asset_materialization.go:78`），非事件通道。

agent 侧统一入口：`TelemetryHook=HandleKernelTelemetry`（`chat/server.go:525`）→ `IngestKernelTelemetry`（`harness.go:3890-3906` 四分支）+ `ingestAuditionTelemetry`（`chat/audition_events.go:167`）。

### 3.1 工程/render 变更语义事件（D4 事件源核心候选，#1-#10）

| # | 事件（字面量） | 内核发射锚点 | agent 识别/消费锚点 | 粒度 | 携带信息 | 当前投影响应 |
|---|---|---|---|---|---|---|
| 1 | `type:"delta_update"` | `VitApp/Source/Service/ZmqGateway.cpp:45` | `bridge.go:405-415` → `shadow.ApplyDelta` | 节点/参数级 | target_uid, action(`property_changed:<prop>`), value, seq_id | shadow.nodes_by_uid 更新 + ChangeReceipt（含 affected_scopes，见 §5.M4）→ 台账失效（E12）；投影本身惰性，下次观察才算 |
| 2 | `topic:"recording" subtopic:"recording_stopped"` | `VitApp/Source/Service/VitHeadlessService.cpp:813` | `bridge.go:417-422` → refreshCh → `refreshShadow` | 工程级（take 落地） | —（触发 get_project_state 重拉） | shadow.Initialize 权威快照 → 全部投影输入基准刷新 |
| 3 | `topic:"render" subtopic:"render_done"/"render_failed"` | `VitApp/Source/Service/VitProductionCoordinator.cpp:719` | `harness.go:3897-3937` `ingestKernelRenderTelemetry` + `WaitRender` :3941 | render 作业级 | job_id, status, file_path, message | renderResults 唤醒 waiters → D1 试听 A/B 结算、export、strip silence |
| 4 | `command:"audio_feature_data_ready" feature_type:"waveform_envelope"` | `VitApp/Source/Service/WaveformEnvelopeBaker.cpp:927` | `harness.go:3899/:3974-4001` | clip/track 级 | track_id, clip_id, file_path, shared_memory, float_count, tile_index, request_id | 写 feature snapshot → DOM 波形行 / TIM / acousticpackage / MOM（下次观察消费） |
| 5 | `command:"tile_ready" feature_type:"spectral_field"` | `VitApp/Source/Service/TiledSpectrogramBaker.cpp:980` | `harness.go:3901/:4004-4030` | clip 瓦片级 | tile_index, expected_tiles, shared_memory, float_count | 写 snapshot spectral 行 → DOM/TIM 频谱输入 |
| 6 | `audio_feature_data_ready` L3 `band_energy_summary` | `VitApp/Source/Service/L3AcousticAnalyzer.cpp:468` | `harness.go:3903/:4031-4060`（`isL3AcousticSummaryFeature`） | track 级 | feature_type, track_id, status/quality_status, source_identity, project_uuid | projectstore L3 工作区 + acousticpackage store → MOM frequency_relationship / C1 readiness |
| 7 | 同上 `stereo_relation_summary` | 同上 | 同上 | track 级 | 同上 | 同上（MOM stereo / track.stereo_space 面） |
| 8 | 同上 `loudness_summary` | 同上 | 同上 | track 级 | 同上 | 同上（RLM/电平面） |
| 9 | `command:"l2_render_probe_ready" feature_type:"l2_render_probe"` | （render probe 产出路径，走 REQ 应答+SUB 通知） | `harness.go:5426-5431`（临时 SUB 过滤，**不经 IngestKernelTelemetry**） | track+tap 级 | request_id, track_id, clip_id, tap_point, render_revision, evidence_ref, bands, masking_frames | frequencycleanup baseline / masking 帧 / CCB masking 观察 |
| 10 | `type:"audition.*"`（ready/selected/select.changed/stopped/failed/prepare/blind_disclosure） | （试听引擎） | `chat/audition_events.go:167`（前缀匹配） | 试听会话级 | conversation_id, session{session_id,status,candidates,active_candidate_id} | freeStateLoop.AuditionSessionSnapshot；ready→requestAuditionJudgment → taskstate human_judgment_required |

### 3.2 VSP 协议面变更信号（拉取式，#11-#14）

| # | 事件 | 锚点 | 粒度 | 携带信息 | 当前投影响应 |
|---|---|---|---|---|---|
| 11 | VSP `state.delta`（`vsp.state.delta_request.v1` 应答） | `kernel/vsp.go:169/:265` | 工程修订级 | revision, base_revision, ops, changed_tracks, changed_clips, project_epoch | executor delta→shadow（ChangeSourceExecutorDelta `shadow/change.go:17`）→ 同 #1 |
| 12 | VSP `state.resync` / `resync_hint` | `kernel/vsp.go:176/:278` | 工程 epoch 级 | resync, resync_hint, snapshot_hash | 全量快照重拉 + feature snapshot 打 stale（`stale_feature_snapshot_for_current_request` mixboard.go:2262；`normalizeProjectFeatureMaterialFreshness` :2403）→ DOM/MOM/TIM/COM 失效重算 |
| 13 | 命令应答 `revision`/`resync_hint`（`vsp.command.request.v1`） | `kernel/vsp.go:249-263` | 每写命令级 | revision, transaction_id, resync_hint | governed mutation receipt → audioclosure `RecordGovernedMutation`（`chat/audio_closure_controller.go:793`）→ closure revision 记账；CCB 新鲜度绑定 |
| 14 | `get_project_state` 权威快照应答 | `bridge.go:477/:493`；`refreshShadowWithStatus` `l2_probe_batch.go:56` | 工程级 | tracks, plugins, clips, project_uuid/revision/epoch, analysis_manifest | shadow.Initialize（authoritative，`shadow/change.go:15`）→ pending receipt 转正（shadow.go:110）→ 全投影输入基准 |

### 3.3 UI/hub 分发面（对观察投影无独立响应，D4 可排除，#15-#19）

| # | 事件 | 锚点 | 粒度 | 说明 |
|---|---|---|---|---|
| 15 | VSP `event.notification`（hub 广播/回放） | `vsphub/telemetry.go:91`；订阅 `vsphub/event.go:43/:187` | 任意遥测主题 | #3-#9 的 hub 分发面；agent 不直接消费 |
| 16 | `realtime.publish`（meters/spectrum/transport 流） | 发布 `bridge/vsp_realtime.go:127`；hub `vsphub/realtime.go:170/:195/:485` | 轨道子集 latest-only | 纯 webui 电表/走带 |
| 17 | `asset.materialize_request`→`asset.materialized` | `vsphub/asset_materialization.go:146/:160` | clip 瓦片级 | hub 波形物化缓存（源 #4 的 SHM） |
| 18 | `track_duration_ready` | 仅回放白名单 `vsphub/event.go:245` | clip 级 | **agent 内无 handler**（仅 hub 回放前端） |
| 19 | `topic:"levels"` / `topic:"transport"` | `bridge.go:447-448` | 电表/走带 | 无投影响应；shadow 显式滤噪 TRANSPORT（`shadow.go:770`） |

**D4 事件源结论**：真候选=#1-#10、#11-#14（14 项携带工程/render 变更语义）；#15-#19 是 UI 分发面。事件目录已满足 ≥15（19 项全录，便于决策侧裁剪）。

### 3.4 附：agent 内部事件（非内核→agent，防止混淆）

- audioclosure 17 种事件常量（`audioclosure/events.go:14-32`，亲验：closure_started/round_started/observation_recorded/project_change_recorded/project_revision_changed/governed_revision_booked/frontier_updated/round_completed/model_protocol_repair_recorded/capability_started/capability_settled/rollback_started/task_state_projected/closure_policy_extended/phase_transition/diagnostic_round_recorded/closure_settled）——闭包状态机的事件溯源日志，不是投影输入事件；
- observationrouter（`observationrouter/router.go:16/:37`）只把 mix.observe 工具调用/结果翻译成 typed UI 事件（`chat/events.go:193/:243` → SSE），不路由观察数据本身。

---

## 4. 现存失效/缓存机制盘点（10 项，物化层种子排序）

| # | 机制 | 位置 | 语义 | 物化层可复用度 |
|---|---|---|---|---|
| M1 | **F5 前台铃行级 freshness** | `harness.go:7155-7174`（原子 tmp+rename）+ `annotateMixboardSnapshotFreshness` :7181-7215 + `annotateMixboardRowFreshness` :7217-7248（亲验） | 写侧标注：同 request_id=current；无铃有材料身份=material_reuse+reused_for_request；其它=stale+superseded_by_request；missing 占位行不冒充任何铃。读侧零推断 | ★★★ 直接的"行级脏标注"范本：物化层每行带铃归属即可复用该三分语义 |
| M2 | **free-state 台账按域失效** | `chat/free_state_reasoning_loop.go:1725-1759`（亲验） | 变更 receipt 的 affected_scopes 命中 view → 行打 stale+invalidated_by_change_id，旧行保留审计、不可支撑当前动作；`project.change_delta` view 豁免 | ★★★ 现存唯一变更→观察面传播链；D4 只需把"view 行"换成"投影实例"即得脏传播骨架 |
| M3 | **shadow 受影响域映射表** | `shadow/change.go:200-222`（亲验 addChangeScopes）+ 两层 freshness（`change.go:24-29`：pending_authoritative_refresh/current_snapshot；确认 `shadow.go:110-130`） | gain/level/mute/solo→track.level+mix.*+headroom；pan→stereo+mix.*；plugin_chain→processor.*+before_after+mix.*；clips/lifecycle→structure+track.*+mix.*；默认 project.state。delta 即时可见但未经快照背书 | ★★★ 这张表就是 D4 依赖图的第一版边集（参数路径→观察域），两层 freshness 是"提示/背书"分级原型 |
| M4 | **acousticpackage Store** | `status.go:106-251`（落盘+原子替换+路径级进程内单例）、:337-421（Build/Merge）、:214-221（异 revision 交叉失效）、:1577-1628（source/clip revision 指纹：source_hash+path+size+mtime+duration → `rev_hex8`；clip 再叠 project/track/clip/start） | 落盘状态包：derived 基底+stored 受控回填（优先级 ready=7…deferred=0 + 完整度评分）；同 target 异 revision 即 stale | ★★★ 投影物化的持久层原型：revision 指纹=物化键，Upsert 交叉失效=增量失效的现存实现 |
| M5 | **L2 probe 状态指纹缓存** | `harness/l2_probe_batch.go:105-148`（cachedL2RenderProbeForStateRange）+ :183-198（`track_state_fingerprint`=sha256(project_uuid+track 行)） | 轨道状态指纹不变+行 ready+revision/evidence 非空+range 匹配 → 复用旧 probe；`ForceFresh` 绕过 | ★★☆ "输入指纹→缓存命中"范本，可直接推广为投影物化键 |
| M6 | **revision 三元组+绑定门** | MOM TrustQuality（`mom/policy.go:180-216`、`projection.go:431-480`）；AB 门 `mixboard.go:3335-3336`；观察-动作 revision 门 `agentloop/free_state_gate.go:975-1013`；closure revision 失配 settle `chat/audio_closure_controller.go:67-89`；`taskstate/state.go:191-201`（EventProjectRevisionChanged 清 EvidenceRefs） | source/clip/render 三 revision 各自独立 missing/ready 判定；动作结算前核对观察 revision==当前 revision，失配即 stale 拒用 | ★★☆ 物化层的"消费时校验"腿（与写侧标注互补） |
| M7 | **观察持久化=缓存** | `mixboard/persistence_v2.go:13-75`：feature snapshot 全量进 evidence CAS（`projectstore.PutEvidence` `evidence.go:49-120`，内容寻址 `evidence://<sha256>`，pin 不可逐 :226-236），obs 体瘦身落 `.vit_agent/<uuid>/observations`，预算裁剪/超限拒绝 | 同 observation_id 的 mix_read/CCB 不重算；evidence 内容寻址天然去重 | ★★☆ 物化产物的存储层可直接搭在 evidence CAS 上 |
| M8 | **freshness 归一化 sanitize** | `harness.go:3860-3888`（stale_feature_snapshot_without_request_or_source_revision）；`mixboard.go:2156-2262/:2403` | 请求装配时把无铃/无源 revision 的快照行统一改写 stale/missing，不升级 readiness | ★☆☆ 读侧兜底，物化后可被写侧标注取代大半 |
| M9 | **per-result memo / pack 去重** | `message_loop.go:8539-8541`（result 内 tim_projection memo）；`static_mix_gain_staging_context.go:1847-1857`（同 run pack 只建一次） | 会话内同输入不重算 | ☆☆☆ 生命周期太短，仅参考 |
| M10 | **mutation barrier** | `agentloop/runner.go:604-621` | mutating 工具完成后丢队列、强制重新观察 | ★☆☆ "变更后必失效"的行动级强制，语义可并入事件失效 |

另记（非观察域但同构）：PluginSnapshotCache（`harness/plugin_param_guard.go:27-76`）、processor attestation Store MarkStale（`processorattestation/store.go:25/:162`）、semantic compressor 执行 freshness（`semantic_compressor_execution.go:452-464`，topology/参数基线指纹变化拒执行）、frequencyAcousticSnapshotCache（`mixboard.go:300-309`，size+mtime 失效）。

---

## 5. 对 D4/G2 的设计输入（事实推出，非方案）

1. **物化的插入点已天然存在**：投影计算全部收口在 `FinalizeObservationContext`（catalog.go:59-86）一处编排，事件→物化的改造面集中；CCB 已是纯读端（ccb_observation.go 只 Read+Assemble，E11）——把 finalize* 从请求内挪到事件驱动不会破坏 CCB 契约。
2. **脏传播的 scope 映射不必新发明**：shadow `addChangeScopes`（change.go:200-222）已给出"参数路径→观察域"映射，free-state 台账（M2）已验证"scope 命中→打 stale"可行；D4 需要补的是 scope→**投影实例**（而非 view 行）的对应及传播闭包。
3. **G2 必测的既有隐患两处（本报告新实证）**：E8 rlm `mergeSourceRows` 字段级覆盖合并无 revision 比较（rlm/projection.go:170-192）；E16 B1 pack 同 run 不失效（static_mix_gain_staging_context.go:21-23）。两处都是"漏标脏传播=静默错误数据"的活样本，应作为失效正确性专项的红测素材。
4. **"等上游"逻辑目前散落在会话流**：DAD gate（E9，message_loop.go:7548-7593）是唯一显式编码的"上游未完成→等→覆盖度比较替换"实现，但它绑定在 stems import 回复路径上；物化层需要把它泛化为依赖图上的就绪门。
5. **事件通道结论**：内核事件经由单点 `IngestKernelTelemetry`（harness.go:3890-3906，仅 4 分支）+ bridge delta 路径 + VSP 拉取信号；l2_render_probe_ready 走临时 SUB 不经统一入口（harness.go:5426-5431）——D4 若做统一事件总线，此处是现存分叉点。
6. **观察请求是唯一重算触发**（§1），因此"物化命中率"在现状下无基线数据；建议 G2 前先加观察请求计数（只计数不改行为）摸清各 finalize 的实际调用频率分布。

## 6. 边界与剩余（诚实申报）

- **内核 C++ 侧发射全集未逐个盘点**：本报告事件目录以 agent 消费侧为准（识别点全锚定），内核侧仅抽查 6 个发射点锚定存在（ZmqGateway.cpp:45 / VitHeadlessService.cpp:813 / VitProductionCoordinator.cpp:719 / WaveformEnvelopeBaker.cpp:927 / TiledSpectrogramBaker.cpp:980 / L3AcousticAnalyzer.cpp:468）。若需"内核侧发射全集+发射条件"（如 delta_update 的节流/合并策略），需另开内核侧勘察卡。
- **vsphub 回放白名单细节**（event.go:245 一行提及 track_duration_ready 等）未逐项展开；对 agent 观察链无影响（#15-#19 已归 UI 面）。
- **L1-1 已登记的两处边界沿用**：前端第三写者与 PCA 层不在本卡范围。
- masking 投影本体（`masking/measurement.go`）为 deferred 能力，本卡只盘了其观察触发链（E17），未盘算法内部。
- 报告全部锚点出自静态读码 @9b85b9f；动态时序（如 F5 铃在并发写下的实际行为）未运行验证，属 G2 专项范畴。

## 7. 验收对照

| 卡面验收项 | 本报告 | 状态 |
|---|---|---|
| ①触发链矩阵覆盖九投影+DAD | §1（九投影+audioclosure/frequencycleanup，另附 masking 观察） | ✅ |
| ②依赖边 ≥10 条两端锚点 | §2 共 17 条，显式/时序隐式分类，隐式均附时序证据 | ✅ |
| ③事件目录 ≥15 个 | §3 共 19 项（另附 agent 内部事件防混淆） | ✅ |
| ④现存机制 ≥3 个语义说明 | §4 共 10 项，含物化种子排序 | ✅ |
| ⑤报告入库 | 本文件，port/l1-2-recon-1-proj-inventory 分支 | ✅ |
