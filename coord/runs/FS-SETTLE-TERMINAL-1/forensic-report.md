# FS-SETTLE-TERMINAL-1 取证报告：d1 欠账结算尾段非法相位迁移 fs7→fs9

- 取证时间：2026-09-30 晚窗（领取 20:17）
- 取证基础：活栈残留工件直接回放（非推测）——失败 run 的闭包事件流（audio_closure_b78b9a9229222c35，30 事件全量）、loop 持久态、4 条 durable continuation 状态、journal 18 条动作记录（`C:\Users\timoz\AppData\Local\Vit\ProjectHistory\drafts\draft_20260930T120722_d2ca6bdd\`）+ 决策侧已钉死的 final-gate-error.jsonl / ui-state-failed.json。
- agent 日志不可用：agent_last.log 已被 authority restore 轮询刷屏覆盖（AUTH-RESTORE-LOGSPAM-1，仅存 20:14:42 后 500 行），失败窗口 20:09:4x-5x 无日志残片。事件流回放补齐了这一缺口。

## 1. 迁移序列时序（事件级回放，本地时间 +08:00）

| 时刻 | 事件 | 说明 |
|---|---|---|
| 20:08:21 | closure_started；fs0→fs1 | 观察问答轮起（goal_4e4c14a9 "检查当前工程有什么问题"），round 1 |
| 20:08:43 | obs_120843 记录；frontier 建立 4 候选（progress=true）；fs2→fs3→fs4 | round 2，streak 清零 |
| 20:08:57-58 | obs_120858（track 1007 bass）| round 3 的目标级观察 |
| 20:09:18 | frontier 选定 candidate:054d（progress=true）；task_state→needs_experiment(rev4) | round 3：LLM 首次 needs_experiment 被准入。**注意：闭包相位停在 fs4**——此刻 DimensionClosed 未成立，admission 的两次 AdvancePhase（GatePassed / AdmissionValid 分别断言）都走不过 fs4→fs5 的 guard |
| 20:09:22-23 | set_plugin_param（EQ band0 -2dB，bx_hybrid）succeeded | D1 实验干预应用 |
| 20:09:25 | governed_revision_booked rev 2→4 | 受治修订入账 |
| 20:09:25-26 | obs_120926（post_action freshness）| 确定性 post-action 观察预订 |
| 20:09:26.4 | cont_bf9ea73e 装甲（第 1 个 d1 owed-settlement tail）| 应用于 applied boundary |
| 20:09:28 | round 4 开始；**fs4→fs5→fs6**（派生 guard 走链，GatePassed=false 到不了 fs7） | 第 1 个 settle tail 切片 |
| 20:09:40.221 | **free_state final gate 拒绝二次 needs_experiment 准入**（final-gate-error.jsonl 原文） | LLM 把同一 proposal 原样重交（final=true needs_experiment）；gate 要求先出 settle report |
| 20:09:40.238-239 | obs_120926 记入闭包（round 4）；diagnostic round r_e77b0ee2；round 4 completed（frontier 无进展，**streak=1**） | 切片内模型改为执行 post-action 观察；round 4 仍欠结算报告 |
| 20:09:40.252 | cont_bf9ea73e completed；**cont_dd329a1d 装甲**（第 2 个 settle tail，slice_ddac235345ff2526） | completed-over-owed boundary 再武装 |
| 20:09:41 | round 5 开始 | 第 2 个 settle tail 切片 |
| 20:09:58.385 | **event30 fs6→fs7**（gate_passed=true） | LLM 产出 settle report（needs_experiment 携带 experiment_materiality=insufficient_dose + experiment_round_decision=user_judgment_pending）。recordFreeStateDecision 对携带实验报告的决策跳过 G 审计（carriesExperimentReport）并断言 GatePassed——**脊柱此刻才进 fs7，fs8 从未被进入** |
| 20:09:58.406-445 | audition checkpoint + A/B 渲染（after_revision_4.wav / before_revision_2.wav）| 人耳试听面挂载 |
| 20:09:58.514 | loop 入 judgment park：status=blocked，experiment=waiting_for_user，last_error="experiment round is waiting for the human judgment boundary" | 轮的欠账结算进**人耳判断边界**（合法驻留态） |
| 20:09:58.5xx | **失败点**（详见 §2） | 同一请求的收尾 recordAudioClosureRound |
| 20:09:58.534 | cont_dd329a1d status=failed，last_error=失败信封 | 调度器收口 |
| 20:09:58.537 | goal_4e4c14a9 status=failed | 用户面失败 |

## 2. 失败点构造链（代码锚）

失败发生在 round-5 切片收尾（park 后 20ms 内），`recordAudioClosureRound(audio_closure_controller.go:427)` 内：

1. 观察循环（:483-533）：3 个观察全部 observation_id 去重跳过（journal 证实全程只有 3 次 CCB 调用，无新观察）。
2. frontier 更新（:534）：settle report 不改变 frontier → 无进展。
3. round 收尾（:581-610）：CompleteRound → streak 1→2（内存事件 31，因后续出错未落盘——持久流止于 event30）。
4. **no-progress 边界（:612-614）**：`NoProgressStreak(2) >= MaxNoProgressRounds(2) && frontier 候选>0` → `settleTaskAtAudioClosureBoundary(:649, "closure made no material progress…")`。
5. 边界内守卫全部放行：
   - `freeStateTerminalTurnGuarded`（:669）要求 `loop.Experiment == nil`——实验运行时存活的循环（含 judgment park）**不在保护范围**；
   - 队列守卫（:684）只覆盖 `queueOpen && !frontierOpen`——frontier 有候选，不停。
6. **:700-707 显式 `TransitionPhase(fs7 → fs9)`**——这是全仓唯一一处显式 fs9 迁移请求。`EvaluatePhaseGuard`（audioclosure/phase.go:148）按邻接表（:93 fs7 后继={fs8,fs4}）拒绝："illegal phase transition fs7_improvement_proposal -> fs9_terminal"。
7. 错误回传 goalrunner_chat.go:482 → `audioClosureControllerErrorResponse`（audio_closure_controller.go:1584，GoalStatus=failed）→ 调度器信封 "durable continuation failed: minimal audio closure controller failed: …"。

## 3. 根因定性（回答卡面问题）

**不是**"结算尾段主动把 next phase 定为 fs9"的路径选择，也**不是**欠账记账错账（两个 settle tail 的武装/完成时序全部正确，停止条件不触发）。定性为**复合缺陷，三层叠加**：

1. **边界终止语义错用（直接原因）**：`settleTaskAtAudioClosureBoundary` 假设任意 FS 相位可直达 fs9（fs0-fs6 邻接表确实如此），对 fs7 构造了非法显式迁移。**校验器正确、缺陷在请求方**（与决策侧判断一致）。合法的终态通道其实并存两条：显式 `TransitionPhase(→fs9)`（受邻接表约束）与 `driver.Settle → EventSettled`（fold 语义 "settled closure is FS9"，**不受**邻接表约束——goalrunner_chat.go:306 的内部续跑收尾用的就是这条）。边界路径选了前者且没检查邻接合法性。
2. **守卫缺口（触发时机）**：no-progress 边界击中的时刻恰是实验轮刚结算进 human-judgment park（人耳 A/B 已渲染、loop blocked 等 judgment POST）。全局设计中"唯一能 settle 该等待的是 guarded audition judgment POST"（freeStateLoopOwesAutoSettlement 注释、freeStateLoopOwesAutoSettlement 对 judgment park 减除），但 settleTaskAtAudioClosureBoundary 的两个 defer 守卫都不认识这个驻留态——机器在把裁决权交给用户的那一拍把闭包终止了。即使迁移合法，此刻终止也会摧毁待决判断。
3. **相位脊柱滞后实验生命周期（背景缺陷，解释为何停在 fs7）**：实验的 admission→apply→post-action 全程发生在闭包 fs4→fs6 期间（admission 时刻 DimensionClosed 未成立，GatePassed 与 AdmissionValid 在不同边界分别断言，AdvancePhase 走不过 fs4→fs5→…→fs7/fs8 链）；settle report 才把脊柱推到 fs7（且刻意不断言 AdmissionValid，free_state_reasoning_loop.go:1092-1096）。**fs8（实验验证相）从未被进入**——"跳过 fs8"不是结算尾段的决定，而是脊柱从未跟踪实验生命周期。若为收尾而强行补走 fs7→fs8→fs9，等于伪造一个从未发生的验证相——不可取。

**gate 拒绝与续跑终止的交互（卡面第 2 问）**：final gate 拒二次准入（20:09:40）→ LLM 改出 settle report（20:09:58）→ 报告把轮结算进 judgment park → 同一请求收尾命中 no-progress 边界 → 机器试图终止刚交给人耳的闭包。gate 语义（要求 settle report 先行）被正确执行；错在边界结算不认识 judgment park。

## 4. 修复方案（修请求方，迁移表不动）

1. **judgment/欠账窗口的边界豁免**：`settleTaskAtAudioClosureBoundary` 增加 defer 守卫——loop 带 Experiment 且命中 `freeStateJudgmentBoundary` / `freeStateLoopRoundPendingSettlement` / `freeStateLoopRoundOwesIntervention` 三窗口之一时返回原状态不结算（镜像 admitAudioClosureRound:352-354 已有的轮扩展窗口三件套；no-progress/证据上限两个调用方此前无此保护）。人耳判断 POST（或用户显式续跑）仍是唯一 settle。
2. **fs7 的合法终止构造**：`LegalPhaseTransition(from, fs9)` 成立时保留可审计的显式 TransitionPhase（fs0-fs6、fs8 行为不变）；不成立（fs7）时不再构造 phase_transition 事件，走下方既有的 task-semantic 结算 + `audioClosureSettleFromResult`（EventSettled 通道，fold 落 fs9）——不伪造 fs8 通过，不非法放行 fs7→fs9 邻接。

预期效果（本场景）：round-5 收尾不再出错，round_completed 落盘，loop 保持 judgment park（waiting_continue + A/B 卡呈现），goal 不再 failed——"合法收尾或明确 settle 边界呈现给用户"。

## 5. 越域申报

卡面文件域为 `agent/internal/audioclosure/ + agent/internal/agentloop/`（实锚后申报）。实锚：**修复点在 `agent/internal/chat/audio_closure_controller.go`（settleTaskAtAudioClosureBoundary）+ chat 包测试**；audioclosure 与 agentloop 零改动（校验器与 gate 行为均正确，勿动）。chat 是续跑/结算路径的实际宿主（goalrunner_chat/continuation_scheduler/audio_closure_controller 均在此包），符合卡面"续跑/结算路径"的范围意图，特此申报。
