# FS-PARK-TURNFAIL-1 取证报告

- 取证人：PC 执行会话①（port/fs-park-turnfail-1，基线 ec93d570）
- 证据源：`coord/runs/M8-FORENSIC-20260930/evidence/`（events-webui-muo6fygb.json / ui-state.json / message-loop-run-e5796736.jsonl）
- 活栈：2026-09-30 22:04–22:08，webui 会话 webui_muo6fygb，goal_6da74bd8 / run_e5796736，草稿工程 draft_20260930T140416（project_revision 演进 37→39）

## 1. 事件级时间线（events 47 条逐 seq 回放）

| 时刻 | seq | 事件 | 事实 |
|---|---|---|---|
| 22:04:59 | 1 | turn.started | 用户输入"检查一下当前工程有什么问题"，turn_id=run_e5796736 |
| 22:05:04 | 5 | turn.completed | 第一轮回答正常落盘（**同一 turn 第 1 次 completed**） |
| 22:05:11–22 | 6–9 | item.*/ccb_observation_request | d1 结算切片继续跑 items（**completed 之后同一 turn 再出 items——生命周期翻转第一段**） |
| 22:05:36–43 | 10–17 | trajectory.* + mix_tick | 自由态实验轮开启（turn:free_state_c2681cbd102907f6），EQ 干预应用（吉他轨 1017，400Hz −0.5dB），post-action 观察落账 |
| 22:06:01 | — | parse_failed | settle 报告第一次产出 JSON 不可解析 |
| 22:06:23 | — | repair_succeeded + **拒①** | 修复层原样重发旧 needs_experiment 提案（无 report 字段）→ final_gate："applied experiment round is pending settlement…emit the settle report" |
| 22:06:37 | 22–33 | round.decision + user_judgment.requested×2 + turn.completed | LLM 补齐 settle 报告：materiality=subthreshold/insufficient_dose，target_response=ambiguous/**human_audition_ready** → 轮决策 **user_judgment_pending** 落账 + kernel audition ready + A/B 判断请求×2 → goal=waiting_continue（turn_kind=settle_slice）。**park 建立，loop.Status=blocked** |
| 22:06:50–58 | 34–45 | audition.select/selected/stopped ×2 | 用户试听了 A/B（kernel 面事件）**但从未 POST judgment** |
| 22:07:20 | 46 | turn.started | 用户第二条消息进入——**复用同一 goal/run/turn id（生命周期翻转第二段：第 2 次 started）** |
| 22:07:38 | — | **拒②** | LLM 对新消息给出合理的 Bass 新实验提案 → needs_experiment 全准入门拒绝（G1 revision 漂移至 39、G7/G8 证据引用不解析），门引导"return needs_observation" |
| 22:08:07 | — | **拒③** + terminal fallback | LLM 按引导返回 needs_observation+ccb.observation_request——但轮已 terminal 锁定，`final:false` 不准入 → `free_state_terminal_turn_unparseable: no admissible final decision after one strengthened retry` |
| 22:08:07 | 47 | **turn.failed** | 用户面"执行失败" |

## 2. 三拒失败链机理（message-loop 六条逐条回放 + 代码锚定）

### 拒①（22:06:23，settled 轮门）
- LLM 产出：重发的吉他轨 needs_experiment（无 report 字段）。
- 拒绝点：`agent/internal/agentloop/free_state_reasoning.go:1048-1055`——轮已应用待结算，二次准入非法，引导"emit the settle report"。
- 结果：LLM 随后补出合规 settle 报告，park 正常建立（22:06:37）。此拒本身是**正确防线**。

### 拒②（22:07:38，G 门结构化 gap）——新消息被续进 park 的 goal
- 路由锚定：`chat/goalrunner_chat.go:36` `beginChatGoal`——`shouldResumeConversationGoal`（goal=waiting_continue 可续用，`shouldResumeGoalFromStatus`）→ 新消息**复用同一 goal/run**。
- 切片上下文锚定：`chat/free_state_reasoning_loop.go:253` `prepareFreeStateReasoningContext`——park 后持久 loop `Status=blocked` → `freeStateLoopActive=false` → 走 `case requestOK: loop = requestLoop` 分支，绑定了 **webui 传输面带回的陈旧 loop 副本**（其 experiment 缺 22:06:37 才落账的轮决策）。
- 后果一：`messageLoopFreeStateJudgmentBoundary`（free_state_reasoning.go:507-531）读不到 user_judgment_requested/轮决策 → **judgment boundary 防线失效**——本应拒以"parked at the human-judgment boundary"的话没有出现，新提案进了 G 门。
- 后果二：G1（task_contract revision 37 vs 活 revision 39）、G7/G8（stale candidate 证据引用不解析）全拒——提案死于陈旧证据绑定，门引导"return needs_observation with the next bounded observation"。
- 后果三：TIMING-1 反滥用计数（`messageLoopFreeStateNoteAdmissionRejection`，计数上限 2）在陈旧副本计数基础上 +1 → **admission_rejections_exhausted → terminal turn 锁定**。

### 拒③（22:08:07，终局轮拒绝一切非终局输出）——LLM 按引导给的合法答案被自己锁死
- LLM 产出：`needs_observation` + ccb.observation_request（**正是拒②引导要求的下一步**）。
- 拒绝点：`free_state_reasoning.go:844-861` `messageLoopFreeStateTerminalTurnIssue`——terminal 锁定轮上一切非（完整提案 needs_experiment / 终态族）输出一律拒绝，**含 tool call**。
- 强化重试一次后仍 needs_observation → `message_loop.go:696` `messageLoopTerminalFallbackResult` → `r.fail`（stop_reason=free_state_terminal_turn_unparseable）。
- 用户面：`chat/server.go:2386-2392` 把 `resp.Error≠""` 映射为 **turn.failed**。

### 50s 尾段归因确认
22:05:22→22:06:37 的 75s 尾段 = ①22:06:01 parse_failed→修复（~22s）；②22:06:23 拒①→settle 报告重生成（~14s）；③事件落账与 audition 准备。**耗时主体确为 LLM 重试与门循环，非 CCB 数据组装**（与入池初判一致，llm telemetry 精确 per-call 已由 message-loop 六条+时间戳钉死）。

## 3. 根因归纳

- **RC1（路由语义缺失，主因）**：park 存活下新用户消息被 `beginChatGoal` 续进同一 goal/run/turn——系统**不存在**"新输入=默认采纳待裁决段+关闭该轮+新 turn id"的语义。停止条件（park 路由架构层无定义需产品裁定）**未触发**：路由本身有定义（续用），缺的是采纳结算，且用户裁定（2026-09-30）已给出语义定案。
- **RC2（陈旧传输副本）**：park 的持久 loop 为 blocked（非 active），切片绑定回退到 webui 回传的陈旧 loop 副本——judgment boundary 防线与计数器都读到旧状态。
- **RC3（终局强制）**：BOUNDARY-1 terminal-turn 机制把"最后一跳必须终局决策"应用到 parked 轮上，LLM 的正确"先观察"答案结构性不可准入 → 诚实 fallback 被分级为执行失败 → turn.failed。

## 4. 修复设计（按用户裁定 2026-09-30，decisions/2026-09-30-park-adoption-and-judgment-card-ruling.md）

**Part A（主修）park+新用户输入=默认采纳**：`handleChat` 在 `beginChatGoal` 前拦截——若会话 loop 驻留 judgment park（`freeStateJudgmentBoundary` 成立且轮上无人耳判断证据）且消息是真实新输入（非"/"命令、非裸"继续"、非待确认提案的确认语）：
1. 轮决策 `adopted_by_continuation`（新 RoundDecision；保留已应用状态，不要求 sufficient target response，不写 UserJudgmentEvidence）；
2. 任务语义 EventTaskSettled（human_judgment_required→settled 为合法迁移，taskstate/state.go:335；自动清 PendingInteraction）；
3. 实验 `Settle(outcome=adopted_by_continuation)`（新 SettlementOutcome；**绝不记 human_confirmed**——UserJudgmentEvidence 为空、TargetResponse 保持 ambiguous/human_audition_ready 原样）；
4. loop.Status=completed、audition 快照 adoption_status=adopted_by_continuation（供 AB 卡终态样式联动）；
5. `CompleteGoal(parked goal)` → 新消息经 `beginChatGoal` 开**全新 goal/run/turn id**（解决生命周期翻转）。

**Part B（防线）park 不强制终局**：`runAgentLoopChat` 结果映射处——失败结果的 stop_reason 属 terminal fallback 族且会话持久 loop 仍驻留 judgment park 时，不投影 turn.failed：改 waiting_continue + 用户可读边界话术 + 重挂 audition 判断交互请求（park 保持可裁决，新输入通道由 Part A 兜底）。

**不改动**：拒①类 settle 引导、G1-G8 准入门、judgment POST 显式结算通道（人耳判断仍是唯一 human_confirmed 来源）、BOUNDARY-1 机制本体（仅 park 场景下旁路其用户面失败投影）。

## 5. 诚实边界

- `adopted_by_continuation` 是新增枚举（RoundDecision/SettlementOutcome/trajectory 评估值），旧记录无此值，消费端按未知值兼容（switch default 不升级）。
- 默认采纳结算的 evidence 链=实验自身观察/干预证据，**不含任何人耳判断证据行**；论文/证据链口径区分"人耳确认"（human_confirmed）与"继续对话默认采纳"（adopted_by_continuation）。
