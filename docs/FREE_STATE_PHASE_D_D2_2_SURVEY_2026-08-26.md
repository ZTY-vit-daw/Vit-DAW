# D2-2 前置勘察：多轮实验路径只读调研（2026-08-26）

- 状态：只读勘察产出（无代码改动）
- 仓库：D:\Vit_DAW，分支 codex/g1-g7-runtime-remediation
- 执行卡：queue/doing/2026-08-26-D2-2-presurvey-multiround-experiment.md
- 上游裁定：todo/2026-08-27-D2-prep-free-state-loop-expansion.md（顺序 D2-0 → D2-1 → D2-2；D2-2 风险最高，ADR §8 停止规则边界，必须最后开）

## 0. 勘察基线与本卡边界执行情况

- 本卡为纯只读勘察：未改任何代码/测试，未 push，未做 git reset/clean。
- **观测基线（写稿时 git status --short，非本卡产物）**：`M VitApp/Workspace/default_project.xml`、`M agent/internal/chat/free_state_d1_runtime.go`、`M agent/internal/chat/free_state_experiment_runtime.go`、`M agent/internal/chat/mix_tick_confirmation.go`、`?? agent/internal/executionverifiers/staticeq.go`、`?? docs/TEMPDIR_FLAKY_EVIDENCE_2026-08-26.md`。其中 chat 三文件与 staticeq.go 属并行会话（D2-1-S2b 执行链开发）工作树改动，本勘察未触碰。
- **重要稳定性警告**：勘察期间 `agent/internal/chat/free_state_experiment_runtime.go` 被并行会话实时编辑（同一函数行号在两次读取间从 337→354 漂移；`freeStateExperimentAdmission` 已从"仅 track_gain"演进为"D2-1 域表准入"，见 §1）。本文档对 chat/ 文件的行号以写稿时最后一次 read 校验为准，引用前需复验；`experiment/`、`audioclosure/`、`agentloop/`、`scripts/` 在勘察期间未变化，行号可信。D2 prep 卡引用的 `free_state_experiment_runtime.go:340` 已过期（现 361-362）。

---

## 1. 多轮路径现状

### 1.1 DecisionNextRound（insufficient_dose 分支，非 D1 才走）

- 决策枚举：`agent/internal/experiment/runtime.go:88-99` 定义八种 RoundDecision，`DecisionNextRound = "next_round"`（runtime.go:91）。
- **chat 侧自动路径**：`agent/internal/chat/free_state_experiment_runtime.go:358-372`（recordFreeStateExperimentDecision）：materiality 为 `EvaluationInsufficientDose` **且 `!loop.Experiment.Admission.IsD1S1()`**（361 行）时，才自动 `DecideRound(DecisionNextRound, "insufficient dose; calibrate in next round")`（362 行）并 `StartRound` 开下一轮（368 行）。
- **D1 拦截点**：361 行的 `IsD1S1()` 守卫即 D1 域表拦截；此外 `experiment/runtime.go:560-567` `StartRound` 对 D1-S1 拒绝第二轮（"D1-S1 permits one round"），`runtime.go:665-667` `ApplyIntervention` 对 D1-S1 拒绝同轮第二次前向变更（"D1-S1 permits one forward mutation per round"）。模型侧提示同样禁止：`agent/internal/agentloop/ccb_model_prompt.go:70`（subthreshold "MUST NOT request next_round or another mutation"）与 `ccb_model_prompt.go:75`（"Never return next_round, continue_once, or a second treatment"）。
- **运行时层（非 D1）多轮能力存在**：`StartRound` 仅对 D1-S1 限一轮（runtime.go:560-567）；`Round` 注释明言一轮可含多个 Intervention 用于剂量校准（runtime.go:423-424）；`ApplyIntervention` 以 `InterventionCount() >= ExperimentBudget` 为预算上限（runtime.go:668-670、932-938）。单元测试 `TestMultiRoundExperimentCalibratesDoseThenAdvances`（experiment/runtime_test.go:366-404）验证：同轮两次剂量尝试（attempt 1 不足 → attempt 2 达成）→ `DecideRound(NextRound)`（395）→ 第二轮 StartRound（398）→ Rounds==2（401）。
- **audition 判定侧 next_round**（非 D1）：`agent/internal/chat/audition_events.go:690-704` dispositionForUserJudgment 中 heard_difference!=yes（692）、preference neither/unsure（700、702）均返回 `DecisionNextRound, Continue:true`；`audition_events.go:779-801` applyFreeStateJudgmentOutcome 对非 D1 执行 DecideRound(NextRound)（785）+ StartRound（792）+ resumeTaskExperimentAfterJudgment（800）。**D1 域被 d1DispositionForUserJudgment 转换为 stopped**（audition_events.go:706-712，708 行 NextRound→Stopped/NeedsJudgment；执行分支在 802-821）。
- **生产可达性结论**：所有生产准入都被 D1-S1 域表绑定（见 §1.3），因此多轮路径**当前在生产链路上不可达**（仅 experiment 包 API 与单测可达）。这是 D2-2 要打开的缝。

### 1.2 continue_once

- **receipt 层（schema 校验）**：`agent/internal/experiment/receipt.go:36` `DispositionContinueOnce = "continue_once"`；`MaxContinueOnce = 1`（receipt.go:17）；`ValidateImprovementReceiptSet` 全局最多一次（receipt.go:209-225，218-221 超限报错）；**红线：classification=ambiguous 禁止 continue_once**（receipt.go:173-180，175-177；注释直引 ADR §8）。
- **FS8 相位层（继续额度）**：`agent/internal/audioclosure/phase.go:133-135` PhaseGuardInput.ContinueOnceRemaining（"the one global continue_once allowance"）；FS8 自环守卫要求其仍为 true（phase.go:179-185，183-184 已花则拒）；该值由 `state.ActionAttempts < policy.MaxActionAttempts` 派生（phase.go:325），默认 `MaxActionAttempts: 1`（audioclosure/types.go:70、77），ActionAttempts 在能力启动时 +1（audioclosure/events.go:258），超限直接 settle（audioclosure/driver.go:272-275）。即**默认策略下 FS8 自环结构上不可能发生**。
- **chat/agentloop 层**：`agent/internal/agentloop/ccb_model_prompt.go:75` 明令模型不得返回 continue_once；chat 包内无任何生产代码产生 continue_once disposition（grep 全包无 DispositionContinueOnce 产出点）。
- **测试锁定**：`experiment/receipt_schema_test.go:113-132` TestM09ContinueOnceAtMostOnceGlobally（一次合法、二次拒绝、continue_once 后接 retain 合法）；`agent/internal/agentloop/free_state_phase_test.go:128-129`（FS8 自环用尽额度被拒）与 232-235（spent allowance 显式拒绝）。

### 1.3 experiment_budget>1 的 admission

- 字段：`experiment/runtime.go:144`；通用校验仅要求 >0（runtime.go:178-180）。
- **D1-S1 门强制 ==1**：`runtime.go:209-211`（"D1-S1 experiment_budget must be 1"），且两 dose scope 的 max_action_attempts==1（runtime.go:220-228）。
- **chat 生产入口三重封死**：① `freeStateExperimentAdmission` 硬编码 `budget := 1`（free_state_experiment_runtime.go:43）；② 提案路径只认 D2-1 域表（track_gain/static_eq，free_state_experiment_runtime.go:28-30）；③ `startFreeStateExperiment` 对模型直接下发的 `ExperimentAdmission` 也强制 `ValidateD1S1()`（free_state_experiment_runtime.go:283-291 的 285 行、302-304 的 302 行）。
- **结论**：`experiment_budget>1` 在生产不可达；只有实验包 API 直调（测试 testAdmission，runtime_test.go:19-20 等）能构造。D2-2 需新增一条受控的 D2-2 准入变体（ValidateD1S1 的预算/轮次参数化），见 §6。

---

## 2. 剂量校准流

- **触发**：`EvaluateMateriality` 收到 `MaterialitySubthreshold` 时轮状态置 `RoundDoseCalibrating`（experiment/runtime.go:710-712）；subthreshold 必须带 `EvaluationInsufficientDose`（runtime.go:385-387）；"insufficient dose 不证伪假设"（runtime.go:355-356 注释、770-772 决策层禁止 subthreshold+plateau，TestInsufficientDoseDoesNotDisproveHypothesis runtime_test.go:217-249）。
- **DoseBounds 语义**：`DiagnosticDoseBounds`/`RetainedDoseBounds`（runtime.go:142-143）是 admission 级不透明 map，通用校验仅要求非空（runtime.go:175-177）；D1-S1 按域校验：track_gain 的 delta_db 非零且 |delta|<=2（experiment/d1s1_domains.go:28-34）、static_eq 的 gain_db 非零且 |gain|<=2（d1s1_domains.go:61-67）、两 scope max_action_attempts==1（runtime.go:220-228）。**多轮语义**：无任何跨轮累计剂量检查——下一轮复用同一 Admission（free_state_experiment_runtime.go:362-368 直接以原 admission 的 CheckpointRef/RequestedViewIDs 开新轮），剂量上限只在单次 admission 校验时生效。
- **是否存在自动加剂量路径（ADR §8 红线核查）**：
  - insufficient_dose（≠ambiguous）自动 next_round：**存在但仅限非 D1**（free_state_experiment_runtime.go:361），且它只是"开新轮"，不修改 dose bounds、不产生新的 admission——"加剂量"实际要由下一轮模型新提案/新 admission 完成。
  - ambiguous 自动加剂量：**receipt 层明确禁止**（receipt.go:173-180）；但**运行时层存在一条非 D1 的冲突路径**：ambiguous audition 判定 → `DecideRound(NextRound, "ambiguous audition judgment; continue with a new point or dose")`（audition_events.go:785，摘要原文即"继续新点或新剂量"）→ 自动开新轮（792-796）。该路径当前因生产全 D1 而不可达（d1DispositionForUserJudgment 将其转 stopped，audition_events.go:706-712），但 **D2-2 一旦放开非 D1 多轮，此路径即与 ADR §8"ambiguous 不得自动加剂量"直接冲突——待 GLM 裁定**（选项：改为 stopped/needs_judgment 终局，或要求新 admission 才可继续）。
  - `MaterialityEvaluation.CalibrationProfile`（runtime.go:364）字段存在但**全仓无消费者**（grep 仅命中定义处），说明剂量曲线目前只是记录位，没有驱动逻辑。

---

## 3. 停止规则全景（RoundDecision 八种决策触发条件矩阵）

运行时裁决闸 `DecideRound`：`experiment/runtime.go:756-786`；决策枚举校验 `validateDecision`（runtime.go:979-986）。**触发条件矩阵**（含决策与目标响应/材质态的关系）：

| 决策 | 允许前置条件（runtime.go:764-772） | 触发者/路径 | 结算映射 |
|---|---|---|---|
| next_round | 无 target_response 也可（仅当 materiality==subthreshold；764 行），或带 target_response | ① chat 非 D1 insufficient_dose 自动（free_state_experiment_runtime.go:361-362）；② 非 D1 ambiguous audition（audition_events.go:785）；③ 模型 ExperimentRoundDecision 直报（free_state_experiment_runtime.go:389-391） | 不结算；开新轮（368） |
| retained | 必须 TargetSufficient（767-769） | audition 判定偏好 B（audition_events.go:696）；判定边界 settle 族（free_state_reasoning_loop.go:993-999）；模型直报 | OutcomeImproved（Settle 时按 target 映射，free_state_reasoning_loop.go:920-925；runtime.go:887-888） |
| rolled_back | 无需 target_response（764 排除） | audition 判定偏好 A（audition_events.go:698）；MarkRollback 通道（runtime.go:788-808）；rollback 后自动执行（free_state_experiment_runtime.go:393-399） | OutcomeRolledBack（runtime.go:891-892） |
| user_judgment_pending | 无需 target_response（764 排除） | 模型直报（FreeStateDecision.ExperimentRoundDecision，需 status=needs_experiment：free_state_reasoning.go:165-167）；**落定后进入人类判定边界**（free_state_reasoning_loop.go:909-917、973-987 freeStateJudgmentBoundary） | 不结算；park 等待 audition（RequestUserJudgmentForSession 要求 human_audition_ready + canonical task human_judgment_required：experiment/user_judgment.go:105-140） |
| plateau | 需 target_response；**subthreshold 禁止**（770-772，"insufficient_dose must not disprove the hypothesis"） | 模型直报 | OutcomePlateau → EvaluationUnsupportedHypothesis（runtime.go:895-896） |
| blocked_by_observation / blocked_by_capability | 无需 target_response（764 排除） | 模型直报；chat 侧 status=blocked 自动映射（free_state_reasoning_loop.go:926-939：StopReason 含 capability → BlockedCapability） | OutcomeBlockedObservation/BlockedCapability，需 canonical task capability_blocked（runtime.go:847-848、893-894；task_semantics.go:324） |
| stopped | 无需 target_response（764 排除） | ① D1 ambiguous audition 终局（audition_events.go:804-805，"no further mutation permitted"）；② Turn.Stop（runtime.go:901-916，需 canonical task cancelled） | OutcomeStopped（runtime.go:851-852、893-894）或 D1 场景 OutcomeNeedsJudgment（audition_events.go:815） |

**判定边界（多轮交互核心）**：`freeStateJudgmentBoundary`（free_state_reasoning_loop.go:973-987）在 round 决策为 user_judgment_pending / 已请求判定 / 已录判定时置位；此后模型只能报 settle 族决策（retain/rollback/stopped）——最终门 `messageLoopFreeStateJudgmentBoundaryIssue`（agentloop/free_state_reasoning.go:518-529，522-527 白名单）与 chat 侧 `freeStateJudgmentSettleDecision`（free_state_reasoning_loop.go:993-999）双重拦截 revival（needs_observation/needs_action/新 admission 均拒）。调度器同时把 user_judgment_pending / human_audition_ready 判为需人工交互、停止自动继续（chat/continuation_scheduler.go:127-153，145、148 行）。**多轮下注意**：判定边界是按 round 存的（round.UserJudgmentRequested / Decision），新一轮 round 会清掉该 round 的状态位——D2-2 需裁定判定边界是否应跨轮延续（待 GLM 裁定）。

**与 8 种决策相关的状态机出处**：ADR §7（docs/ADR_FREE_STATE_EXPERIMENT_RUNTIME_AND_OBSERVABLE_TRAJECTORY_V1.md:570-590）列出 deciding→八决策；§8 Terminal Outcomes（603-618）列出十种结算 outcome 且"technically_verified 不等于 improved"（618）。

---

## 4. 封存断言缺口

### 4.1 现有覆盖（多轮相关）

- experiment 包（27 个测试）：D1-S1 单轮/单变更（runtime_test.go:45、70、89、123）；多轮剂量校准（366-404）；budget 限次（406-422）；insufficient dose 不证伪（217-249）；第二轮 post-action 拒绝（308-348，D1 域）；rollback（273-301）；stop 后拒绝后续活动（501）；receipt M08/M09（receipt_schema_test.go:32-111、113-132）。
- chat 包：D1 判定 retain/ambiguous 结算（free_state_d1_runtime_test.go:475-505）、server 判定处理器（545）、rollback（695）、重启投影不重复变更（717）；实验报告字段摄入（free_state_experiment_runtime_test.go:121-196）、判定边界拒绝 revival（197-255）与放行 settle（256-306）；audition 无差异开新轮（audition_events_test.go:275-296，非 D1）、disposition 策略表（298-323）；M10 继续预算（free_state_continuation_test.go:40-75、118-215）；agentloop FS8 自环（free_state_phase_test.go:128-129、232-235）与判定边界（346-381）。
- **缺口盘点**：生产路径多轮 = 零覆盖（所有多轮测试都走 experiment 包直调或非 D1 chat 直构）；budget>1 的 chat 级准入无测试；跨轮累计剂量无测试；ambiguous→next_round 冲突路径无任何"该被禁"的测试（现有测试反而断言其可执行：audition_events_test.go:275-296）。

### 4.2 D2-2 需要新增的封存断言清单（只列断言，不写代码）

1. **准入断言**：D2-2 多轮 admission（budget>1）经新校验变体通过；D1-S1 校验变体对 budget>1 仍拒绝（防回退）。
2. **轮次/预算断言**：InterventionCount 跨轮累计受 budget 上限约束；超 budget 的 ApplyIntervention 报"budget exhausted"且不产生事件；budget 用尽后的结算必须映射 OutcomeBudgetExhausted ↔ canonical task capability_blocked（runtime.go:847-848）并有 chat 级测试。
3. **剂量边界断言（封存重点）**：跨轮剂量不得突破域表绝对界（track_gain |delta_db|<=2、static_eq |gain_db|<=2，d1s1_domains.go:28-34/61-67）——**若 D2-2 允许累计剂量，需新定义累计判据（每轮界 vs 跨轮累计界，待 GLM 裁定）**；每轮 max_action_attempts 仍为 1（或新域参数化）。
4. **ADR §8 红线断言**：ambiguous（classification=ambiguous 或 target ambiguous）之后**不得**自动 next_round/加剂量——若裁定非 D1 ambiguous 仍须继续，则必须改为"新 admission 显式获批"路径；receipt 层 ambiguous+continue_once 拒绝测试保留（receipt_schema_test.go:70-74 已有）。
5. **continue_once 断言**：全局最多一次（receipt 层已有 TestM09）；FS8 自环额度用尽后任何自环拒绝（agentloop 层已有）；**新增 chat/agentloop 联调级**：额度用尽后的模型继续请求被拒绝且轨迹可见。
6. **判定边界跨轮断言**：多轮场景下 human_judgment_pending 后任何 revival（observation/action/新 admission/下一轮请求）被最终门拒绝；settle 族决策放行（现行为按 round 计，需裁定跨轮语义后锁定）。
7. **重启/幂等断言**：多轮 turn 重启后 Rounds/Intervention 不重复（对齐 D1 的 TestD1S1RestartProjectionDoesNotDuplicateMutation，free_state_d1_runtime_test.go:717）；第二轮 post-action 观察 revision 必须匹配第二次变更 after_revision。
8. **封存纪律**：上述断言不硬编码轨道名（AGENTS.md §5 封存测试纪律）；失败只记失败类型。

---

## 5. 烟测扩展面（run_free_state_d1_smoke.ps1 对多轮的假设）

- 结构：`scripts/run_free_state_d1_smoke.ps1` 参数（3-16 行：PublicManifest/PublicCaseId/AgentHttp/TimeoutSeconds/AdmissionOnly/SettlementProbe 等）→ 真实三件套启动（dev_agent_smoke.ps1：54-68；kernel 5555/agent 7878 探活：70-77；Godot 探测/兜底启动：78-105）→ python 运行器 `scripts/free_state_d1_smoke.py`（107-120）→ 退出码约定：3=NOT_EXERCISED（129-132）、0=PASS（160）、非 0=失败（133-135）；settlement 探针含 agent 重启后 `--verify-settled` 复验（140-158）；报告落 `artifacts/free_state_d1_s1/<stamp>/d1_smoke_report.json`（29-32）。
- **单轮断言点**（free_state_d1_smoke.py）：
  - `experiment_budget` 必须 ==1（431）；
  - diagnostic/retained dose bounds 的 max_action_attempts 必须 ==1（432-434）；
  - **恰好一个 experiment round**（437，"D1 must contain exactly one experiment round"）；
  - 恰好一次前向变更（440）；
  - receipt 必含 before/after_revision、transaction_id、idempotency_key、actual_readback_db、readback_verified（443-448）；
  - 恰好一个 post_action CCB 观察且 fresh+revision 匹配 after_revision（450-453）；
  - materiality/target_response 记录存在（454-455）；
  - d1_receipt 标志位（457-464：human_confirmed 必须 false、settled 必须 false，禁止伪造人判）；
  - audition 会话两候选 audio_file 源（466-474）；
  - **settlement 不得开第二轮**（assert_settled_projection，539-551 的 550 行）；
  - 判定边界重挂（594-598）、settlement 探针（621、654-665）。
- **多轮扩展面**：上述 431/437/440/550 四处是硬单轮假设，D2-2 烟测不能复用同一断言函数；需按 AGENTS.md"复用 ps1 脚本体系、加参数"的模式扩展——建议：`run_free_state_d1_smoke.ps1` 增加 `-MultiRoundProbe` 参数（或新建 `run_free_state_d2_2_smoke.ps1` 同模式），python 侧新增 `validate_d2_multi_round(...)` 断言函数（多轮轮次/预算/剂量界/判定边界/重启幂等），fixture 沿用 p01/p02（D2 prep 卡可复用 fixture 陈述），并保持 sealed 段拒绝（run_free_state_d1_smoke.ps1:25-27）。**是否新建脚本 vs 扩展现有脚本：待 GLM 裁定**。

---

## 6. D2-2 切片建议（草案，2-3 张执行卡）

> 依据：本卡第 1-5 节。D2-2 总目标=在 D1 单轮可信基础上放开多轮，且 ADR §8 红线（ambiguous 不得自动加剂量）与全部现有守卫不退化。每张卡开工前需 GLM 裁定 §2/§3/§4 中标注的开放点。

### 卡 D2-2-S1：运行时准入与预算参数化（experiment 包，先于接线）

- **范围**：新增 D2-2 准入校验变体（如 `ValidateD2MultiRound`，参数化 experiment_budget>1、每轮 max_action_attempts、跨轮累计剂量判据）；`StartRound`/`ApplyIntervention`/`DecideRound` 在 D2-2 准入下放开单轮/单变更限制（保持 D1-S1 门原样，防止回退）；budget 用尽映射 OutcomeBudgetExhausted 的 canonical task 契约测试。
- **边界**：只动 experiment 包与单测；不改 chat 接线、不改烟测、不改模型 prompt。
- **验收命令**：`cd agent; go test ./internal/experiment -count=1`（含新封存断言 4.2 的 1-3、8 条）；`go test ./internal/chat -run 'TestD1S1|TestMultiRound' -count=1` 防回退。
- **风险**：跨轮累计剂量判据定义是主要开放点（每轮界 vs 累计界，待 GLM 裁定）；做错会让 D1-S1 守卫松懈。

### 卡 D2-2-S2：chat 多轮接线与判定边界裁定（chat/agentloop 包）

- **范围**：D2-2 admission 的生产入口（freeStateExperimentAdmission/startFreeStateExperiment 的预算参数化）；insufficient_dose 自动 next_round 在 D2-2 域的开启（现 361 行守卫）；**ambiguous 后处理裁定落地**（§2 红线冲突：非 D1 ambiguous audition→next_round 路径 audition_events.go:779-801 必须改为 stopped/needs_judgment 或"新 admission 获批"）；判定边界跨轮语义；continue_once 额度在 chat 层的消耗/拒绝联调；多轮重启幂等。
- **边界**：不改 experiment 包语义；模型 prompt 更新（ccb_model_prompt.go:50、69-75 的 D2-2 措辞）随本卡；不改烟测脚本。
- **验收命令**：`go test ./internal/chat -run 'Test.*(FreeState|D1S1|Audition|Continuation)' -count=1`；`go test ./internal/agentloop -run 'Test.*(FreeState|Judgment)' -count=1`；`go build ./...`。
- **风险**：最高风险卡——ambiguous 红线与判定边界跨轮若裁定错误，会造成自动加剂量泄漏或判定后复活循环（2026-08-25 21:09 D1 smoke 同类回归）。

### 卡 D2-2-S3：多轮烟测扩展（scripts，最后）

- **范围**：按 §5 扩展面新增 `-MultiRoundProbe` 或新 ps1；python 侧 validate_d2_multi_round 断言（多轮轮次/预算/剂量界/判定边界/重启幂等/预算用尽）；p01/p02 fixture 双用例；NOT_EXERCISED 口径沿用（退出码 3）。
- **边界**：只动 scripts/；**不占用真实烟测环境运行**（留给 S3 会话；本卡仅写脚本与 dry 校验），实际运行按排程窗口。
- **验收命令**：`powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_free_state_d2_2_smoke.ps1`（exit 0）；`git status --short` 仅 scripts/ 与 docs/ 变更。
- **风险**：单轮断言四处（py 431/437/440/550）若被误改会破坏 D1 回归；必须新增断言函数而非就地放宽。

**建议执行顺序**：S1 → S2 → S3；S2 开工前必须完成 GLM 对 §2 ambiguous 路径、§4 累计剂量判据、判定边界跨轮语义三项裁定。

---

## 8. GLM 裁定（2026-08-26 晚窗首审，附于勘察产出）

首审结论：五项勘察的 file:line 证据抽查全部命中（audition_events.go 双分支、runtime.go 三处守卫、receipt.go 红线、insufficient_dose 守卫当前行号复验仍准）；切片建议可行。四项开放点裁定如下，D2-2 各卡按此落地：

1. **§2 ambiguous → next_round 冲突路径**：拆分混淆后分别处理。
   - `heard_difference != yes`（没听出差别）**不是 ambiguous**，是效果不足信号：允许非 D1 自动 next_round（与 free_state_experiment_runtime.go:361 insufficient_dose 路径同级），决策摘要措辞改为 "no audible difference; recalibrate"；剂量变化仍必须经新 admission 过域表校验（现状即如此，不得新增自动加剂量通道）。
   - `preference neither/unsure` 且已听出差别 = **真 ambiguous**：镜像 D1 终局——`DecideRound(Stopped, "ambiguous audition judgment; no further mutation permitted")` + `Settle(OutcomeNeedsJudgment)`，不得自动开新轮。理由：与 D1 行为统一，把 receipt 层红线（ambiguous 禁 continue_once）对齐到运行时层；用户要继续永远可以开新实验（新 turn 新 admission），不是死路。
2. **§4.2-3 跨轮累计剂量判据**：采用"实验基线累计位移界 = 域表单动作绝对界"。track_gain 实验全生命期相对实验基线指纹累计 |Δ| ≤ 2dB；static_eq 每 band 相对基线累计 |Δ| ≤ 2dB；每单次动作界不变。理由：一个实验的总声学效果不超过一次最大有界动作，更强的移动需要新实验（新用户可见提案）。实现锚点：admission 记实验基线指纹（revision + 参数 before 值），跨轮用回执链累计位移；D2-2-S1 落地并加封存断言。
3. **§3/§4.2-6 判定边界跨轮语义**：判定边界按 experiment 作用域持久。任一前轮 `UserJudgmentRequested` 且未落定（无判定记录、未 settle）时，`StartRound`/新 admission/前向变更一律拒绝（错误信息 "judgment pending for this experiment"）；settle 族决策或显式结算后才可继续。理由：防止"向人提问后绕开问题开新轮"；continuation_scheduler 已把 pending 判定停为需人工交互，边界实验作用域化与之对齐。实现建议在 DecideRound/StartRound 层拒绝，不搬 per-round 状态位。
4. **§5 烟测脚本形态**：扩展现有 `run_free_state_d1_smoke.ps1`（加 `-MultiRoundProbe` 参数，三件套自启保持单一路径），python 侧**新增独立** `validate_d2_multi_round(...)` 断言函数；不得就地修改四处硬单轮假设（py 431/437/440/550 对应行）；无参数默认路径逐字节不变。与勘察 §5 建议一致，确认采纳。

附带说明：`MaterialityEvaluation.CalibrationProfile` 全仓无消费者——D2-2-S1 **不得**以它为依赖设计累计剂量判据（用裁定 2 的回执链方案），该字段保持现状。

---

## 7. 本卡验收状态

- 产出：本文档。
- 验收命令（卡内约定）：`cd D:\Vit_DAW; git status --short` —— 本卡未产生代码/测试变更；docs/ 新增本文档，queue 卡已由 todo/ 移入 doing/（完成后置 done/）。工作树其余变更均为并行会话产物（§0 基线），未触碰。

---

## 9. D2-2-S1 执行记录（2026-08-28，flash 会话按卡执行）

- 执行卡：todo/2026-08-27-D2-2-S1-multiround-admission-budget-runtime.md（分支 codex/g1-g7-runtime-remediation，开工 HEAD 20a1c70；卡基线 26475d3 后 experiment 包行号经复验无漂移）。
- 范围：只动 `agent/internal/experiment`（新增 `d2_multiround.go` + `d2_multiround_test.go`，`runtime.go` 三处守卫档位分流与 `Admission.BaselineFingerprint` 字段）。未动 chat/agentloop/scripts/prompt。

### 落地内容与裁定对照

1. **档位显式化（档位陷阱封存）**：`Admission.IsD2MultiRound()`＝域表成员（复用 `D1S1DomainSpecFor`）且 budget∈2..`MaxD2MultiRoundBudget`（包级常量 4，上界由 `TestD2MultiRoundAdmissionBudgetBounds` 封存）；`isD1S1SingleRound()`＝`IsD1S1() && !IsD2MultiRound()`，budget>4 的域成员回落单轮守卫（fail closed）。`IsD1S1()` 与 `ValidateD1S1()` 原语义逐字节未动；封存断言 `TestD2MultiRoundAdmissionTierSplit` 锁定：track_gain+budget=3 的 `IsD1S1()` 仍为 true 且 `ValidateD1S1` 仍拒。
2. **守卫分流**：`StartRound` D1-S1 分支原样保留（第二轮拒、错误文本不变），D2-2 分支每次重跑 `ValidateD2MultiRound` 并放开第二轮；`ApplyIntervention` 每轮单变更守卫对两档同强度（D2-2 档错误文本 "D2-2 permits one forward mutation per round"），总 budget 守卫（InterventionCount 跨轮累计）不变；`RecordObservation` 守卫条件保持 `IsD1S1()`（域成员判定），D2-2 每轮自然受 revision 绑定/恰好一 post-action 观察约束——零改动。
3. **跨轮累计剂量（裁定 2）**：`Admission.BaselineFingerprint`（omitempty，含 revision 锚点，`ValidateD2MultiRound` 必查）；`checkD2MultiRoundCumulativeDisplacement` 以回执链（各轮 applied intervention 的 `AchievedDelta[AdmissionValueKey]`）带符号累计位移，`|累计| > 2dB` 拒（错误可区分 "cumulative ... experiment-lifetime bound"）；每单次动作界不变；failed/ambiguous 占 budget 不计位移；一个 admission 只动一个 admission 钉住的参数（static_eq 单 band），band 级累计在 admission 内与实验级重合。`TestD2MultiRoundCumulativeBoundMatchesDomainAbsoluteBound` 对三域封存"累计界==域表单动作绝对界"。`CalibrationProfile` 未触碰（裁定附带说明）。
4. **判定边界作用域持久（裁定 3 运行时层）**：`experimentJudgmentPending()`＝任一轮 `UserJudgmentRequested` 且无判定记录；`StartRound`/`ApplyIntervention`/`DecideRound`（非 settle 族：retain/rollback/stopped 白名单对齐 agentloop final gate）返回哨兵 `ErrJudgmentPending`（"judgment pending for this experiment"）；不搬 per-round 状态位。判定落定（evidence 录入）后循环重开，heard≠yes 的 next_round 校准路径保持可达。
5. **budget 用尽契约**：逻辑本已存在（runtime.go `OutcomeBudgetExhausted`→`capability_blocked`），补 `TestBudgetExhaustedSettlementRequiresCapabilityBlocked` 与反例测试。

### 验收结果

- `go build ./...` PASS；`go test ./internal/experiment -count=1` PASS（新增 9 个测试函数含对抗用例"每轮合规但累计超界"）；`go test ./internal/chat -run 'TestD1S1|TestMultiRound|TestExperimentBudget' -count=1` PASS（D1-S1 防回退）；`go test ./... -count=1` 全绿。
- 实栈烟测按卡约定未跑（实栈由 GLM 主线独占做 S2d 验收，晚窗统一补 `run_free_state_d1_smoke.ps1` 回归）。
- S2 依赖的本卡档位 API：`IsD2MultiRound()`、`ValidateD2MultiRound()`、`MaxD2MultiRoundBudget`、`Admission.BaselineFingerprint`、`ErrJudgmentPending`。

---

## 10. D2-2-S2 执行记录（2026-08-28，GLM 会话按卡执行）

- 执行卡：todo/2026-08-27-D2-2-S2-chat-wiring-ambiguous-judgment-boundary.md（分支 codex/g1-g7-runtime-remediation，开工 HEAD 1bda158；S2d 已提交、chat 包无未提交改动，串行约束满足）。
- 范围：只动 `agent/internal/chat` + `agent/internal/agentloop`（新增各自 `free_state_d2_multiround.go` 与 `_test`；改 `free_state_experiment_runtime.go`、`audition_events.go`、`free_state_reasoning_loop.go`、`free_state_reasoning.go`、`ccb_model_prompt.go`、`audition_events_test.go`）。未动 experiment 包/scripts/烟测。

### 落地内容与裁定对照

1. **准入档位接线（清单 1）**：档位唯一来源是环境变量 `VIT_FREE_STATE_D2_MULTI_ROUND_BUDGET`（`agentloop.ResolveD2MultiRoundBudget()`，2..4 内生效，越界/畸形回落 1 fail closed；chat 准入与 agentloop prompt 同源读取，档位不会半应用）。`freeStateExperimentAdmission` 保持零参默认单轮（委托 `freeStateExperimentAdmissionWithTier(loop, proposal, 1)`，构建与校验顺序逐字节不变）；tier>1 走 `ValidateD2MultiRound` 并由服务端从新鲜目标观察派生 `BaselineFingerprint`（模型提供的指纹/预算被覆盖——模型不能升级也不能收窄档位，`TestD2MultiRoundAdmissionTierFailsClosedAndBarsModelBudgetUpgrade` 封存）。multi-round admission 创建时记 Info 审计日志。
2. **insufficient_dose 档位化（清单 2）**：`recordFreeStateExperimentDecision` 守卫换 `!freeStateAdmissionRunsSingleRound(admission)`（= `!(IsD1S1() && !IsD2MultiRound())`；单轮 D1-S1 与无注入默认路径行为不变，D2-2 域成员放开）。摘要 "insufficient dose; calibrate in next round" 保持（不含加剂量承诺）。
3. **ambiguous 拆分（裁定 1）**：`dispositionForUserJudgment` 拆为——heard≠yes → `NextRound`（效果不足信号）；已听出差别 + neither/unsure/equal → `Stopped/OutcomeNeedsJudgment`（真 ambiguous，全档终局；equal 属原 :702 default 分支，随裁定覆盖）。`applyFreeStateJudgmentOutcome` default 分支重写：recalibrate 摘要 "no audible difference; recalibrate"（原 :785 "continue with a new point or dose" 冗余删除）；真 ambiguous 走 D1 同款终局（DecideRound(Stopped, "ambiguous audition judgment; no further mutation permitted") + Settle(NeedsJudgment)），不开新轮。recalibrate 的 StartRound 基线 revision 改用活 revision（LatestProjectChange 优先，回落轮自带）——第 2 轮基线是第 1 次变更后的 revision。`TestUserJudgmentDispositionCoversPreferencePolicy` 与 `TestLegacyAuditionJudgmentWithoutDifferenceStartsNextRound`（补摘要断言）同步改写。
4. **判定边界跨轮（裁定 3 chat/agentloop 层）**：`freeStateJudgmentBoundary` 与 `messageLoopFreeStateJudgmentBoundary` 从"当前 round"改为扫描 experiment 全部 rounds（任一轮 `UserJudgmentRequested` 且无判定记录 → 边界成立；当前轮 evidence 已录或 decision=user_judgment 仍 parking；已落定的前轮不 parking 后续校准轮）。`startFreeStateExperiment` 入口守卫：pending 时返回 `experiment.ErrJudgmentPending`（与 S1 运行时哨兵同文案）。settle 白名单（retain/rollback/stopped）与 agentloop 最终门白名单不变。
5. **prompt 档位条件化（清单 5）**：提案形状（原 :51）、subthreshold 规则（原 :72）、轮次/变更限制（原 :77）、domain rule 尾句按档位选择；单轮文本逐字节保留（`TestPromptTierConditionalSingleRoundDefault` 封存原文片段）；多轮变体允许 "one next calibration round" 措辞但仍禁 continue_once、单轮内二次 treatment 与 ambiguous 后任何 round/mutation/dose（`TestPromptTierConditionalMultiRoundByInjection`）。活实验的 admission budget 是 prompt 档位权威（env 仅在无活实验时决定下一 admission 的档位；超界 budget 回落单轮措辞）。
6. **continue_once 锁定（清单 6，不开启）**：chat 无 continue_once 产出点（未新增）；receipt 层 ambiguous+continue_once 拒绝测试与 FS8 自环额度测试原样通过；联测由 `TestJudgmentBoundarySpansRoundsAtFinalGate`（next_round 在边界被最终门拒、settle 放行、落定判定不 parking 校准轮——issue 文本即模型可见轨迹）+ budget 用尽 chat 级测试覆盖。
7. **多轮重启幂等（清单 7）**：`TestD2MultiRoundRestartProjectionDoesNotDuplicate`——多轮 loop 经 `freeStateLoopFromAny(freeStateLoopMap(...))` 持久化往返后重放第 1 轮 action 回执与观察，Rounds/Intervention 不重复，第 2 轮 post-action 观察 revision 匹配第 2 次变更 after_revision。
8. **budget 用尽（清单 8）**：`TestD2MultiRoundBudgetExhaustedSettlesCanonicalTask`——budget=2 花尽后第 3 次 apply 被运行时 budget 守卫拒（不产生事件），`settleFreeStateExperiment(OutcomeBudgetExhausted)` ↔ canonical task `capability_blocked`（experiment TaskState 与 harness RuntimeStatus 双断言，Terminal）。
9. **封存断言（清单 9 = 勘察 §4.2 第 4/5/6/7 项）**：4→`TestD2MultiRoundAmbiguousJudgmentIsTerminalAndBlocksFurtherMutation`（ambiguous 终局 + settle 后 StartRound/ApplyIntervention 拒绝）；5→receipt/FS8 既有测试 + 最终门联测；6→chat `TestD2MultiRoundJudgmentBoundarySpansRoundsAndRefusesAdmission`（revival 拒、新 admission 拒 ErrJudgmentPending、settle 放行）+ agentloop `TestJudgmentBoundarySpansRoundsAtFinalGate`；7→重启幂等测试。断言只落在 rounds/decision/outcome 形状，不硬编码轨道名；失败信息记失败类型。

### 红线自查（卡要求的逐条指认）

- **ambiguous 终局**：diff 见 `audition_events.go` disposition default 分支与 applyFreeStateJudgmentOutcome default 重写；测试见 disposition 策略表 + ambiguous 终局封存测试。
- **判定 pending 拒绝**：diff 见 `freeStateExperimentJudgmentPending` + `startFreeStateExperiment` 入口守卫（返回 S1 哨兵）+ 双侧边界扫描重写；测试见 chat/agentloop 两个跨轮边界测试 + 既有 D1 边界测试不回退。
- **无档位注入默认路径零变化**：diff 见 `freeStateExperimentAdmission` 零参委托（tier=1 分支构建/校验顺序原样）、`startFreeStateExperiment` tier=1 分支逐字节等价（模型直供 admission 路径 ValidateD1S1→fresh 顺序不变）、insufficient_dose 守卫在 budget=1 时与 `!IsD1S1()` 等价、prompt 单轮文本常量逐字复制；测试见 fail-closed/防升级测试 + `TestPromptTierConditionalSingleRoundDefault` + 全部既有 D1 测试原样通过。

### 残留与说明（不阻塞本卡）

- `experiment.Turn.DecideRound` 无 ensureLive：settled 后 round decision 字段仍可被覆写（StartRound/ApplyIntervention 有 ensureLive，无新轮/变更/剂量泄漏，仅审计字段污染）。experiment 包语义 S1 已冻结，本卡不改，留待 GLM 决定是否补卡。
- 边界 revival 拒绝会把 loop 存为 `blocked`，`freeStateLoopActive` 对 blocked 短路——后续模型 settle 决策不再处理（既有语义）；settle-family 放行适用于 loop 仍呈现 active 的 parked 状态，blocked 后的生产恢复通道是 audition 判定路径。测试注释已记录。
- recalibrate 路径 StartRound 失败时（如 budget 用尽后开轮失败）错误只静默不记日志——沿既有行为，未改。

### 验收结果

- `go build ./...` PASS；`go test ./internal/chat -run 'Test.*(FreeState|D1S1|Audition|Continuation|MultiRound|Judgment)' -count=1` PASS；`go test ./internal/agentloop -run 'Test.*(FreeState|Judgment)' -count=1` PASS；`go test ./... -count=1` 全绿。
- 实栈烟测按排程未跑（晚窗统一补 D1 默认路径回归）；多轮 `-MultiRoundProbe` 待 S3。
- 本卡交付门槛（上述单测全绿 + D1 默认路径不回退）达成。
