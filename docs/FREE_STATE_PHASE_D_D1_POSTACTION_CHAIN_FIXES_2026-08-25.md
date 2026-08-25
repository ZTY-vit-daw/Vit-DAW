# Phase D D1 — post-action 链路修复记录（ZCode 会话，2026-08-25 晚）

接续：`docs/FREE_STATE_PHASE_D_D1_FIX_A_ROUNDBOUNDARY_HANDOFF_2026-08-25.md`（Fix A/C + round 记录修复后的交接）。
本会话在其基础上继续修复 post-action 执行链，共 5 层，每层都经真实栈烟测验证推进。

## 已完成修复（全部在工作树，全量 Go 测试 83 包 ok）

### D-B1：blocked 终局补 post-action fresh observation 门
`internal/agentloop/free_state_reasoning.go` `messageLoopFreeStateOutputIssue` 的 `FreeStateBlocked/CapabilityBlocked` 分支增加与 satisfied 分支对称的检查：`requires_post_action_observation=true` 且本轮无 fresh CCB observation 时拒绝 blocked 终局。
回归：`TestFreeStatePostActionRejectsBlockedWithoutFreshObservation`。
验证：20:38 烟测——模型被顶回完成 post-action 观察（检查通过），失败点推进到 materiality。

### D-B2：post-action prompt 授权语义澄清
`internal/agentloop/ccb_model_prompt.go`：明确"实验已经用户确认并 Apply，原始请求授权不是有效边界"；"return blocked" 收紧为"仅在 fresh post-action observation 无法获得时"。

### D-B3：FS8 准入实验评估报告（三处）
- `internal/audioclosure/phase.go`：FS8 策略补 `NeedsExperiment: true`（评估报告载体；旧策略只允许观察+终局，导致"能观察永远不能汇报"）。更新 `TestFS8VerificationAllowsObservationAndEvaluationButNoSecondAction`。
- `internal/agentloop/free_state_reasoning.go`：携带 experiment 报告字段的 needs_experiment 跳过 G1-G7 准入门（G1/G5/G6 重检 Apply 前状态必然失败），改用 experiment schema 校验器。回归：`TestFS8EvaluationReportSkipsProposalAdmissionGate`。
- 同文件 `FreeStateDecision.Validate`：评估形状（带报告字段）不再强制要求 improvement_proposal（服务端经 admission/round 身份绑定，与 decision 内 proposal 无关）。M02 矩阵断言同步更新。
验证：20:52 烟测——模型评估报告通过 gate，失败点推进到变更计数。

### D-B4：调度器交互边界 park + interaction_id 生命周期
`internal/chat/continuation_scheduler.go`：
- `executeDurableContinuation` 检测 chat 响应停在交互边界（确认/澄清）时，将 checkpoint park 成 `waiting_interaction` 并携带 pending（原先静默 completed，确认交互对 continuation 投影完全不可见）。
- `pendingInteractionFromResult` 补顶层 `interaction_id`（`completePendingInteractionContinuation` 按它匹配；缺失导致 20:17 的孤儿 waiting checkpoint）。
回归：`TestParkClaimedContinuationAtInteractionRetiresOnAnswer`、`TestPendingInteractionFromResultCarriesInteractionID`、`TestInteractionBoundaryChatResponse`。
验证：21:00 烟测——确认/交互链路恢复。

### D-B5：experiment 报告字段与决策状态解耦
`internal/chat/free_state_reasoning_loop.go` `recordFreeStateDecision`：任何状态的决策携带 `experiment_materiality/target_response/round_decision` 都入账（原先仅 needs_experiment/improvement/needs_action 状态入账；capability_blocked + user_judgment_pending 的混合形状报告被丢弃）。blocked 终局 settle 在已带 round decision 时跳过（不覆盖人工判断边界）。
回归：`TestExperimentReportFieldsIngestedOnTerminalDecision`。
验证：21:09 烟测——round 内记录了 post-action 观察 + 评估报告 + user_judgment_pending 被暴露并应答。

## 烟测状态（21:09 最新，`artifacts/free_state_d1_s1/20260825_210913/`）

D1 链路所有环节已分别在真实栈上被走过至少一次：维度闭合→FS5-8→提案→确认→Apply(readback+receipt)→round 内 post_action 观察(rev35)→materiality+user_judgment_pending 提交→人工判断交互暴露并应答。但单次 run 尚未全部串起，当前失败：`D1 requires one post-action CCB observation`——**该 run 的 round 里有 2 条 post_action=true 观察**，validate_d1 要求恰好 1 条。

## 剩余缺口（下一任务，按证据排序）

1. **人工判断边界后的 loop 复活**：user_judgment_pending 被应答后，loop 从 capability_blocked 复活并继续 needs_observation（21:09 的 resp[5-8]，烧掉 6/7、7/7），产生了第 2 条 post-action 观察。人工判断边界应是终局等待，后续模型轮只允许 settle（retain/rollback/ambiguous），不允许再观察。
2. **round 内 post-action 观察幂等**：`RecordObservation(post_action=true)` 同一 round 应拒绝/去重第 2 条（防御性，与 1 互补）。见 `internal/chat/free_state_experiment_runtime.go` 与 experiment 包 `RecordObservation`。
3. 之后按 validate_d1 剩余检查（materiality 记录、target response、A/B audition、settlement receipt、重启一致性）逐项核对该链路是否已具备（多数环节已有实现与测试，见 `d1EvaluatedLoopForTest` 一族 fixture）。

## 测试与构建状态

- `go test ./... -count=1`：83 包 ok；唯一偶发失败 `TestProcessorCertificationStartAcceptsBroadbandCompressorCapability` 是 Windows TempDir 清理竞争（单跑 3/3 过，与本轮改动无关，首次出现于本轮之前的全量跑）。
- `agent/bin/VitAgent.exe`：21:08 构建版本包含全部 5 层修复。
- 本轮所有新回归测试 + 更新的 2 个旧断言（FS8 策略、M02）全绿。

## 边界与禁止事项

沿用前一份交接文档：不 reset/clean/丢弃工作树；不伪造证据；不把 FAIL/NOT_EXERCISED 改写为 PASS；提交仍等烟测全链路通过后统一拆分（建议粒度：D-B1/B2 prompt+gate、D-B3 phase/评估、D-B4 调度器交互、D-B5 报告入账，与前两层修复分开）。
