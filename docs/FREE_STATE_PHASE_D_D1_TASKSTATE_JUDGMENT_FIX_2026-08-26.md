# Free-State Phase D D1 taskstate judgment transition fix

Status: current fix record for the D1-S1 non-goal #2 (taskstate 语义迁移缺口). Date: 2026-08-26. Commit: `41477bf`.

## Symptom

PASS run 日志（2026-08-25 终轮起）：

```text
[audition] canonical human judgment transition rejected: semantic transition human_judgment_requested is not allowed from improvement_proposal
```

audition session/render 与验收判据不受影响，但 canonical Task 语义状态从未进入 `human_judgment_required`，判定边界只存在于 experiment 侧投影。

## Root cause

D1-S1 执行链在受控 Apply 后发生 `project_revision_changed`（revision 50→51）。taskstate 的该迁移把语义状态重置回 `observation_in_progress`（作废 revision-bound 证据），但**保留实验身份**（`state.go` Apply 的 experiment-preserving 分支）。post-action 决策随后重新走 `diagnostic_completed → improvement_proposed`，状态停在 `improvement_proposal`（携带 ExperimentID）。audition ready 触发 `requireTaskHumanJudgment`（chat/audition_events.go）时，迁移表只允许 `needs_experiment → human_judgment_requested`，于是被拒。

即收尾记录所述"taskstate 迁移表与实验流的语义对齐"缺口：前向变更后的判定边界合法起点是 `improvement_proposal`（实验身份仍在册），不是 `needs_experiment`。

## Fix

`agent/internal/taskstate/state.go` 的 `EventHumanJudgmentRequested` 分支：

- 合法起点扩为 `needs_experiment | improvement_proposal`；
- 从 `improvement_proposal` 进入时强制快照已绑定 ExperimentID（无实验身份的提案不得请求人工判定）；
- 既有身份校验不变（request 必须带 ExperimentID 且与快照一致）。

修复让迁移合法化，不绕过校验；不伪造人工判断（`human_confirmed` 仍由真实交互推进）。

## Regression locks

`agent/internal/taskstate/state_test.go`：

- `TestHumanJudgmentAfterRevisionCycleKeepsExperimentIdentity`——完整 D1-S1 序列（proposal → experiment_required → project_revision_changed → diagnostic_completed → improvement_proposed → human_judgment_requested），含身份不匹配拒绝；
- `TestHumanJudgmentFromProposalWithoutBoundExperimentIsRejected`——未绑定实验的提案拒绝进入判定。

## Verification

- `go test ./internal/taskstate/ ./internal/chat/ ./internal/experiment/ -count=1` 全绿；
- 真实栈 D1 烟测 `spv1_p02` exit 0（PASS，`artifacts/free_state_d1_s1/20260826_085802/`）：
  - agent log 中该 Warn 0 次（修复前 PASS run 必现）；
  - 落盘 `agent_runtime_state.json` 语义历史：`human_judgment_requested: improvement_proposal → human_judgment_required`，pending_interaction 就绪；
  - 边界保持：`human_confirmed=false`。
