# D2-0 模型命中率：prompt/路由侧改动记录

Date: 2026-08-26 晚窗。Status: 执行记录（GLM L2）。

## 目标与边界

从 prompt/路由侧提升模型自主形成 track_gain 类有界提案的比率（基线 NOT_EXERCISED ≈ 50%）。硬边界遵守：未改任何 G1–G7 判据、相位机（`audioclosure.phaseDecisionPolicies`）、准入校验或 continuation 预算；未改 experiment/chat 语义；判定/守卫全部原样。

## 归因（先只读审计）

1. not_exercised 轮的 continuation 轨迹：6 个 continuation 全部停在 fs4_diagnostic_round 的 needs_observation，耗尽预算后被控制器以 "closure observation round boundary reached" 结算。
2. `agent_message_loop_debug.jsonl` 晚间记录揭示真正瓶颈：模型多次返回 `needs_experiment`，但 `free_state_final_gate` 以 "closure host is in phase fs4_diagnostic_round which does not admit decision status needs_experiment" 弹回——宿主相位机在 fs4/fs5 只放行 needs_observation；`needs_experiment` 要等 frontier（候选行来自 `mix.multitrack_relationship` / `mix.frequency_relationship` 观察）+ target 确认（候选轨 track.* 观察）推进到 fs6 后才合法。
3. 因此 not_exercised ≠ 模型不愿提案，而是模型没有走上能推进相位机的观察路径（维度遍历/结构视图反复消耗预算）。

## 改动（全部在 prompt/披露层）

- `agent/internal/agentloop/ccb_model_prompt.go`
  - improvement 契约前缀新增 `freeStateImprovementConvergenceGuidance`：披露 continuation 预算语义 + "最快合法门路径"三步序列（project.structure → mix.* 关系视图 → 候选轨 track.* 视图 → fs6 后 needs_experiment），并明确"提案证据只需 plausible/reversible/cited"。
  - 新增 `messageLoopFreeStateContinuationBudgetDirective`：相位感知的动态压力指令——预算过半时给出最快门路径警告；预算临界且相位 ≥ fs6 时强制 `final=true needs_experiment`；预算临界但仍 < fs6 时明确警告"此刻提案会被相位门弹回"并指向唯一合法快路径。已进入实验阶段（latest_decision=needs_experiment 等）不施压。
- `agent/internal/agentloop/free_state_reasoning.go`
  - `messageLoopFreeStatePromptContext` 的 compact keys 增加 `continuation_budget` / `continuation_used`（纯披露，模型此前看不到总预算）。
- 新增回归测试：`improvement_proposal_test.go`（指引在场/不泄漏到非 improvement 契约、指令分级、实验阶段不施压、预算披露）、`prompt_wire_test.go`（Context→系统 prompt 装配链路含 CRITICAL 指令）。

## 中间迭代（保留为证据）

- 批次 A（仅静态"尽早收敛"指引）：1 pass / 3 not_exercised——静态指引不足以改变行为。
- 批次 B（催早交卷版动态指令）：0 pass / 4 not_exercised——模型在 fs4 提案被相位门弹回，劣化。此结果直接证实归因 2，并导致批次 C 的相位对齐设计。

## 验收

- 真实栈烟测（spv1_p02 默认模式 -SkipBuild，批次 C）：3 pass / 1 fail / **0 not_exercised**（4/4 进入实验链）；fail 轮与午间基线同类（未达 human_audition_ready，非 prompt 回归）。统计记录见 `docs/FREE_STATE_D1_SMOKE_HISTORY_STATS.md` §4.1。
- 全量 `go test ./...` 通过（含 chat/agentloop/experiment/taskstate）；未改 webui。
- 未动任何运行时判据/守卫/预算。

## 遗留

- 样本量 4 轮；后续 D2-1/D2-2 烟测可顺带累积同口径数据复核 0 not_exercised 的稳定性。
- fail 面（human_audition_ready 未达）成为下一个非 pass 主导面，与 prompt 改动无关，属 D2 后续观察项。
