# D1-SETTLE-TAIL-MAC-1 取证报告：settle 尾段停滞根因

- 取证时间：2026-10-02（Mac 执行侧，port/d1-settle-tail-mac-1，领取基线 origin/main=8c3ff03）
- 证据源：
  - 主证据（已归档）：`coord/runs/D1-EQ-READBACK-550A-1/d1_550a_20261002_182113/`（round_1/round_2 报告、kernel 摘录）
  - 原始工件（只读回指锚）：`~/Documents/vit-d1eq550a-artifacts/d1_550a_20261002_182113/`（round_1/round_2 agent_last.log 环形日志、session runtime state、orchestration 状态、kernel stdout）
  - 被测版本：repo_head=5693f3f（port/d1-eq-readback-550a-1）
- 日志能见度声明：round_1 环形日志 8000 行窗口只覆盖 18:28:40→18:36:17（apply 在 18:26:12，之前证据被遥测刷屏冲出环外——AUTH-RESTORE-LOGSPAM-1 未修的取证代价，本卡按预警用持久化状态+kernel 日志替代定位）。

## 结论（一句话）

轮1 停滞 = 假设②（等待条件永真形态）：干预 apply 落在一个已被 generic mix-tick 可答 park 钳住的 goal 之上，settle 尾段的两条武装路径（applied 边界的 goal==completed 门、recordGoalResult 的 completed-over-owed 门）在该交错下均不可达，settle 检查点永不武装，调度器按设计握着 answerable park 十分钟不放。轮2 与轮1 **不同根**（准入门拒案，设计内 fail-closed）。

## 轮1 事件链（时间戳 UTC，全部可回指工件）

| 时刻 | 事件 | 证据 |
|---|---|---|
| 10:25:39 | evaluator 唯一一次 chat POST（本轮零 interaction 应答、零 continue nudge——responses[] 仅 4 条：chat 回复+3 条合成标签） | round_1 report responses |
| 10:25:48→10:26:00 | 调度切片 1-2 正常：fs2→fs4→fs6 | continuation_timeline |
| 10:26:09.717 | **切片3（cont_8f1a…ee77f8）park 在 mix_tick_confirmation（interaction_9930feec…，track_gain_adjust -2dB @vocals，无 action_domain）**；goal→waiting_confirmation；槽位被占 | session runtime state durable_continuations / goal_continuations |
| 10:26:10 | drain 采样收尾：waiting_interaction+completed×2；evaluator 按设计跳过无 domain 的 generic tick，落 poll_persisted_loop_for_audition 等待 | timeline 最后帧 + evaluator resp[1].interaction_requests（body=“将 Track 1017 的电平调整 -2.00 dB…”） |
| 10:26:12 | D1 orchestration 回合（d1_session_turnfree_state_2d4b61…）服务端自主执行干预：plugin.set_params_batch（API-550A Stereo param2=0.4=-2dB）readback ok | kernel_log_evidence_excerpt.txt + journal "D2-1 bounded static EQ band adjustment" |
| 10:26:12-15 | applied 边界：观察响应链内入账（obs_…102612/102614），requires_post_action_observation=false，reserveD1PostApplySlices 授地板（post_apply_budget_reserved=true，used=3/budget=6），loop→re_evaluating/processor_selection；**armOwedSettlementCheckpointAtAppliedBoundary 因 goal.Status=waiting_confirmation≠completed 直接 return** | 持久化 loop + goalrunner_chat.go:3129 门 |
| 10:26:15→10:36:17 | goal 永不发 result（parked）；调度器每拍 claim 不到任何记录，静默打 `[continuation.park] answerable park holds the chain non_terminal=1 skipped_status=3`（环形日志首尾行同文）；materiality/target_response/judgment 全 null | round_1 agent_last.log 首尾行 + persisted_loop |
| 10:36:17 | evaluator poll_persisted_loop_for_settled_round 600s 窗口耗尽 → require(materiality) → **"acoustic materiality record is missing"** | free_state_d1_smoke.py:1682 |

关键否定证据（排除另外两假设）：
- **非①调度饥饿**：调度器全程 250ms/拍在跑（answerable-park 日志持续到 18:36:17）；预算地板已授（used=3/6，非预算停）；claimNextContinuation 扫描正常，只是无可领记录。
- **非③超时/重试形态**：无重试循环在等待（requires_post_action_observation=false——SETTLE-CHAIN-1 的 6×4s 预订重试从未介入，观察已在响应链内入账）；也没有任何挂起超时。
- **假设②精确形态**：不是事件丢失，是**该交错下不存在武装边界**——parked goal 不产 result、applied 边界只认 completed；"交给回合边界自己扛"的注释假设 park 会被续跑，但 generic 建议"用户就是不理"正是永不续跑的形态（evaluator 的等待姿势=产品契约：mix 建议可不答，实验尾段调度器自理）。

## Mac vs PC 差异定性

非机器时序敏感，而是 **LLM 提案流方差**：Mac 轮1 的模型在 apply 前抛出了一个 generic mix tick（电平建议，恰与提示词"不要用整体增益"相悖，故被 evaluator 过滤不答），把链 park 住；PC 通过轮没有这个"park 先于 apply"交错（goal 在 applied 边界处于 completed 或 running，两门各自可达）。同版本同码不同模型分支即可复现/回避——"Mac 时序敏感"嫌疑不成立，降级为"模型行为触发的窗口期交错缺陷"。

## 轮2 定性（与轮1 不同根）

- error="D1 loop projection is missing" = validate_d1 的 `find_d1_loop(responses) is None`（free_state_d1_smoke.py:1365）：响应流里从未出现带已准入 typed_action 的 loop 投影。
- 实际形态：模型选了 static_eq（selected_domains=[static_eq]）但提案被准入门 **G6_target_evidence + G8_target_consistency** 拒（admission_receipt.failed_gate_ids，status=capability_blocked）→ experiment 从未开轮（rounds=0）→ 无已准入投影。
- 这是 PCA 准入面的设计内 fail-closed（提案证据/target 一致性不过关），属模型提案质量方差，与轮1 的 settle 尾段饥饿无关。550A 锁定场景下它消耗一轮但不指向代码缺陷；止损分类按 error 字段区分 r1/r2 正确（脚本已修，未回退）。

## 修复（见同目录 DIFF 说明与 commit）

根因修在 agent chat/settle 机械域（两处，均在 agent/internal/chat/goalrunner_chat.go）：

1. **applied 边界门扩展**（armOwedSettlementCheckpointAtAppliedBoundary）：goal 为 parked-resumable 形态（waiting_confirmation/waiting_clarification/waiting_continue——shouldResumeGoalFromStatus 全集）时同样武装 settle 检查点；running 仍拒绝（活回合自扛尾段）；completed 形态维持原 SetGoalStatus 修正，parked 形态不动用户可见 park 面。
2. **arm 核心槽位移**（armOwedSettlementCheckpointLocked）：槽占用者映射到非 runnable durable 时不再一票拒绝——answerable park 记录原样存活（应答按 interaction_id 退役，与槽无关），槽位移给 settle 检查点（与 latest-by-goal restore 的回填语义一致）；不可答 legacy shell 按 recalibration-arm 配方取消（防 restore 回灌）；裸占用（armed internal resume=活自动片）与 runnable durable 仍拒绝。

未放宽任何等待、未吞任何错误：停滞面（调度器静默 answerable-park 日志）保持原样，只是机器欠账现在有片可跑。
