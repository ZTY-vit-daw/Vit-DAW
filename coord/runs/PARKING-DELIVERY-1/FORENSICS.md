# PARKING-DELIVERY-1 取证报告：停泊/A/B 交接被 final gate 驳回+预算耗尽静默吞掉

- 卡：PARKING-DELIVERY-1（池序 149）；取证会话=GLM-5.3 L2 执行会话 PC/ZCode；2026-10-20 晚窗
- 方法：只读取证——主仓工作树代码（与领取基线 origin/main=c08de366 的代码面一致，a958c7c2→c08de366 及本地 main 独有提交均仅动 coord/）+ 场次工件亲读
- 工件根：`VitApp/Workspace/Logs/`（agent_last.log / agent_message_loop_debug.jsonl / agent_llm_telemetry.jsonl）
- 结论文件本报告 + 拆卡建议 [SPLIT-CARDS.md](SPLIT-CARDS.md)

## 0. 事实基座（先于三问，工件亲读修正实录摘要）

实录 F5 记「两次驳回」（18:21:34/18:21:45）。工件实为 **4 次连续 `free_state_final_gate` 驳回**（agent_message_loop_debug.jsonl 行 23751-23754），每次违规字段不同、reply 正文全部是合格 A/B 交接文案：

| # | 时刻 | 驳回 error（节选） | 违规层 | reply 正文性质 |
|---|---|---|---|---|
| 1 | 18:20:59 | `experiment_materiality: unsupported materiality state ""…attempt >= 1, got 0` + complete object shape 教学 | Validate() | 「请在 DAW 里对 bass 轨单独独奏，和未处理版本做 A/B 试听」 |
| 2 | 18:21:09 | `experiment_target_response: target response evidence_refs are required` | Validate() | 「请你在 DAW 里对 bass 的混响 A/B 试听…」 |
| 3 | 18:21:34 | materiality 形状（同 #1 类，教学在场） | Validate() | 「请在 DAW 里对 bass 轨做整体开关对比（旁通/启用）…交由你的耳朵判断」 |
| 4 | 18:21:45 | `improvement_proposal requires status=improvement_proposal or needs_experiment`（status=needs_action 携带 proposal） | Validate() | 「请你在 DAW 里对 bass 轨做 A/B 试听：开关这条轨上新加的效果…」 |

slice↔turn 对应（agent_last.log timing 行 448/461/471/481/490 + arm 行 447/460/470/480/489）：交互 approve 后 5 个 continuation slice（18:20:49/18:20:59/18:21:09/18:21:35/18:21:45）逐片以 max_turns 结束；驳回 #1-#4 分别落在后 4 片的片内/片尾，即 turn6-9 executed=0 的实义=**每片产出都被 gate 扣下，无一到达执行面**。#1/#3 正是 109 卡腿 D/E 已硬化教学的形状（教学文本在今晚 error 里在场，见 raw）——模型带教学仍连续违规属概率尾部，与 109 收敛轮（rerun5/6 一次教学即收敛）不矛盾，同模型档行为是概率性的。

## 问①：final gate 驳回后为何无第三次修复机会——bounce/预算/门教学在哪层断

**结论：bounce 通道没有断——它无上限、教学在场、每轮真回喂；断的是它通往任何保底出口的两条路：(a) content-shape 驳回根本不进 TIMING-1 反滥用计数，terminal lock 与其配套的「一次强化重试→诚实 settle fallback」从未触发；(b) bounce 循环活到 continuation 预算耗尽，而预算耗尽路径的用户面兜底（delivery gate）被另一处缺陷挡死（见问②）。对照 full_access_autonomy.go 单 bounce 先例，本质差异是：那个 guardrail 有「上限 1 + 上限耗尽后原样投递」的双保险，final gate 的 content-shape bounce 通道既无上限也无投递保底。**

代码锚点与证据链：

1. **驳回分类**：`agent/internal/agentloop/message_loop.go:10868-10911`——`messageLoopFreeStateOutputIssue` 出 issue 后，terminal-locked 分支（10879-10901，BOUNDARY-1 §1.3：一次强化重试→`messageLoopTerminalFallbackResult` 诚实 settle）与非 locked 分支（10909-10911，TIMING-1 记数+bounce 回喂+retry）。
2. **content-shape 永不计数**：`agent/internal/agentloop/free_state_reasoning.go:699-752`（`messageLoopFreeStateNoteAdmissionRejection`）——只当驳回 issue **逐字等于** G 门结构化 gap 消息（710-712：`freeStateNeedsExperimentGateFailureMessage(...) != issue` 即返回）时才计数；注释原文（694-698）："content-shaped rejections (validation, judgment boundary, settle-report shape) return a different issue and **never count**"。今晚 4 次驳回全部是 `Validate()` 层错误（945-947 行 `"invalid free_state decision: " + err.Error()` 路径）→ 计数为 0 → `freeStateAdmissionRejectionLimit` 永不触达 → terminal lock（749-751）从未建立（此推断由代码路径穷尽性支撑：locked 分支的专属诊断 stage `free_state_terminal_fallback` 在今晚 jsonl 中零记录）。
3. **预算耗尽的触发链**：`agent/internal/chat/goalrunner_chat.go:3227-3313`——每次 pending arm 记账 `loop.ContinuationUsed++`（3243）；默认 budget=6（`free_state_reasoning_loop.go:30`）；3252-3262 BOUNDARY-1 last-checkpoint floor：used>budget 且 newly-locked 时把 budget 抬到 used（为 terminal turn 保留一个 checkpoint）；**真耗尽**在 3265-3299：used>budget 且 lock 已存在非 newly（reservation burned）→ loop.Status=blocked、全部活 durable 置 completed、`autoContinuationBudgetExhausted=true`。arm 行实锤：前 4 片 `budget_stop=false` 正常排片，末片 18:21:45 `budget_stop=true lifecycle=completed scheduled=false`（agent_last.log:489）。terminal lock 在 18:21:35 片尾建立、末片为 terminal-only 的细节是代码路径自洽推断（无专属日志行直证），不影响结论。
4. **强化重试成为死信**：#4 驳回（18:21:45.131）若在 locked 片内会获得一次强化重试回喂（message_loop.go:10884-10888；`freeStateTerminalTurnMaxRetries=1`，free_state_reasoning.go:763），但同秒 slice 即以 limit_reached 结束（timing 行 490，total_ms=9697）——重试指令入了会话却没有模型轮可花（telemetry 下一 LLM 调用=18:22:33 用户第二轮）。「无第三次修复机会」的精确表述：**不是机制拒绝给，是 slice 轮次与 continuation 预算同时先耗尽**。
5. **单 bounce 先例对照**：`agent/internal/agentloop/full_access_autonomy.go:40-52`（`fullAccessAutonomyMaxBounces=1`）+ 237-268（`messageLoopFullAccessDirectionClarificationBounce` 返回 false=上限耗尽后**重复请求原样投递用户**，注释："the repeated request is then delivered to the user unchanged… never trap a turn"）。final gate 的 content-shape bounce 没有对称物：驳回→回喂→再驳回→…直至预算耗尽，模型 reply 正文（合格交接文案）被 `parse→gate` 管线扣住从未成为投递候选——这不是缺设计，而是兜底被押到了 delivery gate（问②），而那道兜底失守。

## 问②：owed_outcome=true 的消费面在哪、用户面应交付什么、为何没消费

**结论：owes_outcome 的消费面共三处，设计全部在位——goal 状态修正（工作正常）、链尾投递门的 D1-STALL-1 欠账回执分支（被 `chainEnded=false` 挡死，本卡根修点）、turn 事件面（同一谓词挡，次生）。静默没有任何设计理由；应交付物已有成品文案=五要素欠账回执，只是没送达。**

三处消费面：

1. **goalrunner 预算停 goal 修正（工作正常）**：`goalrunner_chat.go:3443-3458`——budget exhausted 且 owed 时 `SetGoalStatus(waiting_continue)`（D1-STALL-1：预算停的是调度不是实验，「waiting_continue+耗尽预算」≠完成态）。工件实锤：18:21:45 后 goal 全程 waiting_continue，用户第二轮「继续」类入口未断（第二轮实际走了新 goal 的话术，但 resumable 形态在）。
2. **链尾投递门 D1-STALL-1 分支（被挡死=根因）**：`agent/internal/chat/continuation_scheduler.go:1191-1277`（`settleAndDeliverContinuationChainEnd`）第四分支（1258-1276）：goal=waiting_continue + `freeStateLoopOwesExperimentOutcomeFor` + 终态 record → `owedEvaluationChainReply`（1321-1331）→ `d1OwedEvaluationReply`（1342-1354）生成五要素回执——**该文案明确为预算耗尽形态撰写**（1350 行原文：「但这轮还没做完：改动后的评估没有得出结论，自动续跑也已经用完…你回一句“继续”，我就接着把这轮的评估做完」）。
3. **turn 事件面（同一谓词挡，次生）**：`turn_close_guard.go`/`turn_trajectory_events.go:40-54`——`emitChatTurnTrajectory` 用同一 `goalHasLiveContinuationOwner` 决定 waiting_continue turn 事件的处理；legacy 泄漏同样影响该面（turn.completed 的状态映射不执行），卡 A 修复后自然恢复。

**挡死投递门的机制（本卡根因，链条完整）**：

- `goalrunner_chat.go:3213-3215`：arm 时 `s.goalContinuations[res.GoalID] = durable.Continuation`（legacy arm 写入）；
- `goalrunner_chat.go:3265-3299`：预算耗尽 settle 把 durable 置 completed、全部活记录置 completed——**但 3214 写入的 goalContinuations legacy 条目从未清理**（3211-3212 的 delete 只在 durable 天生终态时执行，发生在 budget-stop 改写之前）；
- `turn_trajectory_events.go:63-93`（`goalHasLiveContinuationOwner`）：legacy 条目存在且其 durable ≠ waiting_interaction → 判「活」（completed 也算活；注释自认 "Unknown arm shapes keep counting as live"——FIX-F2-SURFACE-REPLY 当时只给 waiting_interaction 加了例外，budget-stop 的 completed 形态是漏网 arm shape）；
- `continuation_scheduler.go:1146` `chainEnded := !goalHasLiveContinuationOwner(...)` → false → 1205 行第一道卫兵 `if !chainEnded { return }` 静默返回，四个投递分支（含 D1-STALL-1）零进入。
- **工件实锤**：agent_last.log:491 `[f6.gate] chainEnded=false status=completed respGoal=waiting_continue runtimeGoal=waiting_continue`——与预算耗尽形态四个分支条件全部吻合、唯独 chainEnded 一票否决。

**用户面应交付什么**：设计裁定已在卷=标准欠账回执（上面五要素），静默无理由（goalrunner 注释原文：「the delivery gate stays the single user-facing authority」）。「模型 reply 正文兜底」（4 次被拒 reply 都是合格 A/B 文案）是可选增强非现状设计——存在语义张力（模型 prose 说「请 A/B 试听」但治理层 A/B 卡未建、判定入口未开；干预已应用故操作层试听可行，但保留/回滚治理链不在），列为待主管裁定项（SPLIT-CARDS.md §C），不建议默认采用。

## 问③：109 线双 exit 0 验证面为何没覆盖该形状失败路径

**结论：分两层——(a) 109 线烟测断言面在状态/记录而非用户面文本，预算耗尽形状在其面上=「materiality missing」exit 1（链路 FAIL 可见，但「FAIL 后用户面交付什么」不在判据）；(b) 用户面文本断言其实已在停摆专项烟测 d1_stall_repro_smoke.ps1 里（parked_owe_silent→exit 1 是一等分类），但其判定前提「chain_result 事件数>0」在本缺陷下不成立——交付门静默死亡时零 chain_result，形态落 timeout(exit 2 环境类) 而非 parked_owe_silent(exit 1 缺陷类)，静默交付缺陷被 timeout 分类吞掉。单元层第三层：D1-STALL-1 投递测试显式传 chainEnded=true，绕过了被污染的谓词求值。**

- **109 线（scripts/free_state_d1_smoke.py）**：exit 0 判据=收敛路径健康——`materiality is not None`（1694 附近）、target_response 非空、post-action 观察=1、d1_receipt 全 flags、human_audition_ready、停泊存活轮询窗。预算耗尽形状在 109 卡 rerun2 实证为 exit 1「acoustic materiality record is missing」（同签名）——**烟测看得见链失败，看不见用户面**；且 109 卡七腿处置史方向全部是「让模型收敛」（腿 D/E 教学硬化）或「修链路产品缺陷」（112 绑定/113 弹回/115 回放竞速），「链失败时用户面回执」从未成为验收对象——rerun2 的预算耗尽轮里用户面是否收到回执无人断言过。
- **停摆专项（scripts/d1_stall_repro_smoke.ps1:223）**：判定条件 `goal=waiting_continue && live==0 && chain_result>0` → 按最后 chain_result body 的「继续/未完成」needle 分 parked_owe(exit 0)/parked_owe_silent(exit 1)。本缺陷下 chain_result=0（deliverSchedulerChainTerminal 未被调）→ 条件不成立 → 轮询至 deadline → timeout exit 2。**用户面断言存在但被自己的前置条件短路。**
- **单元层（agent/internal/chat/d1_stall_settle_test.go:156-167）**：`d1StallChainEnd` 助手显式传 `chainEnded` 布尔（182 行传 true）直调投递门——验证的是「chainEnded=true 时分支行为」，`goalHasLiveContinuationOwner` 的真实求值与 recordGoalResult budget-stop 全链无任何测试覆盖，legacy 泄漏因此三层全部不可见。

## 与停止条件的对照

- 未发现 continuation 预算/budget_stop 行为有更晚用户裁定在卷（在卷裁定=预算耗尽非完成态、goal 保 resumable——现状代码与之一致，缺陷仅在投递面）。
- 未牵出 FS 状态机统一收敛（T10）级结构性问题：断裂点是 goalContinuations 生命周期的一个具体泄漏 + 验证面前置条件，收敛面不涉。
