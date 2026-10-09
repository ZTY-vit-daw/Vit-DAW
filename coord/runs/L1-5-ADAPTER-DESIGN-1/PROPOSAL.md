# L1-5-ADAPTER-DESIGN-1：A/C 生命周期适配提案

**提案，未定版。** 2026-10-09，Asia/Shanghai。执行流：Codex GPT-6 / Windows ZTY / 会话 `01a11f3f-41df-7411-acad-4b9a90cb5d0d`。主管负责设计裁决、C 阶段验收、接口解冻及后续发卡。本文不解锁 B/D，不合入 A/C，不新增用户手测义务。

建议选择 **agentloop 持有一个长期 state 的 adapter + 中性会话协议**。保留 C 的宿主执行体及注册顺序；pull 只驱动循环，宿主负责执行事实、恢复状态和最终生命周期。必须修订 A 的占位协议，不能把 C 的 stopped 布尔值包装成 A 的 hit，也不能让 C 返回后再次调用 Tools.Execute 或 finish/T2。

## 1. 版本与取证边界

| 对象 | 实际 inspected 版本 / 事实 |
|---|---|
| 执行工作树 | `C:/Users/timoz/.codex/worktrees/l1-5-adapter-design/Vit_DAW`，`port/l1-5-adapter-design-1`，创建 HEAD=`7e230ab92924295cba5f01cd812f7afbb10e4104`；领取前 status/diff stat 均空 |
| 正式领取 | 协调同步基线 `b07da14ccf31930a318f5cf9c2782011768a1692`；领取提交 `6b7c088fa5c0680244e7c812d2f831e30d972f74` 已推 main，重读 owner 一致 |
| A 原实现 | `de1d876c8b8e3e453d69b138e22bf7425df4f558`，用 git show 读取；未将实现应用到本卡工作树 |
| C 实现 / 回执 | `c488b811` / `89fee703e106e6dd2cc1da47dde64611ddababa6`，源码以 c488b811 行号定位；没有依据主树缺包判定实现不存在 |
| A/C 独立复核 | `7b567089` 的 A REPORT.md；`576c8ca8` 的 C REPORT.md；main 上 `coord/reports/2026-10-09-L1-5-AC-review-audit.md` |
| 规格 | `7e230ab9:docs/HARNESS_V1_DESIGN.md` §2/§3/§5/§9/§10；CURRENT-STATE 将其列为现行规划，不视为完成实现 |
| 并行取消修复 | 领取时另一 owner 正在执行；取证期间 main 推进至 `3f6ba5fb`，修复转 done 待审，代码 `47ce5d75`。已读其相对 `231a4ff3` 的 loop.go diff：新增取消检查和结果保留，不改接口；不将其自验报告当最终验收 |
| 停止条件 | 收口 15:07 +08:00 再次 fetch：origin/main=`3f6ba5fbd787ad80820c532b6423d528b6c90bb2`，本卡 doing owner 一致，C 仍 done 待审、无 C 最终返工/修订 ruling，origin/port/l1-5-impl-c 仍 `89fee703`；没有触发旧提案停止条件 |

主树既有 Workspace XML、history_refs.go 及未跟踪工件没有带入或处理。源码、其他卡、rulings、docs、资源记录只读。本卡没有编译/测试生产实现、没有真实栈或 GUI 操作。

### 1.1 可重放源码锚点

以下路径均相对仓库根；`A:`=de1d876c，`C:`=c488b811。亲读返回点与副作用，不只采用复核报告结论。同行多位置代表分别核查的语句。

| # | 原提交路径 / 行号 | 核查到的事实 |
|---|---|---|
| 1 | A:agent/internal/pullharness/loop.go:73、81、87、112 | Route 入参是原 GoalInput；结果仅 Reply/Outcome/ProbeCost，无暂停/更新协议 |
| 2 | A:agent/internal/pullharness/loop.go:151、169、179 | history 是局部副本；Route 每轮取原 in；命中直接 finish |
| 3 | A:agent/internal/pullharness/loop.go:240、253、260、382 | 模型工具批计数/T1/预算；finish 另发 T2，不能叠在旧 result 之后 |
| 4 | A:agent/internal/pullharness/budget.go:20、51、56 | MaxCycles 与累计 probe 两账户；正上限时 >= 用尽，<=0 不设限 |
| 5 | C:agent/internal/fastpath/router.go:52、60 | 开关整体旁路；stopped 透传；全 false 丢弃 Result 零值，但 state 指针仍被改 |
| 6 | C:agent/internal/agentloop/message_loop.go:405、418、436、449 | 固定十项注册；配置漂移 panic；先 pending 队列，再每轮 diagnostic 判定，再 Router |
| 7 | C:agent/internal/agentloop/runner.go:68、98、350 | Continuation/Result 公共，runState 私有；Result 不携完整活状态 |
| 8 | C:agent/internal/agentloop/runner.go:192、240、263、323、344 | Continue 重建 slice；确认恢复需 pending，确认工具之后才回 loop |
| 9 | C:agent/internal/agentloop/runner.go:505、513、521、547、551 | 已直接调用执行器、计调用数、回写工程与观察状态 |
| 10 | C:agent/internal/agentloop/runner.go:555、576、599、601 | 交互暂停可能无 pending；普通确认有 pending；不能用有无 pending 单独分类 |
| 11 | C:agent/internal/agentloop/runner.go:603、606、611、636、648 | verifier/executionMemory/观察同步；mutation barrier 清队列并重规划；错误可失败，checkpoint 可暂停 |
| 12 | C:agent/internal/agentloop/runner.go:657、673、681、685 | timeout 暂停，显式 stopped/cancelled 终局，插话暂停，原因不同 |
| 13 | C:agent/internal/agentloop/runner.go:690、697、704、713、730 | 旧 turn/tool 上限暂停；complete/fail 已在调用 result 前修改 runtime |
| 14 | C:agent/internal/agentloop/runner.go:739、749、754、782、787 | 所有正常 result 返回都可能 EndTurn；仅四种终态 cont=nil 并 retain |
| 15 | C:agent/internal/agentloop/exit_retain.go:56、70、80、93 | 结论单元用 RunID；已有语句去重；retain 失败留 trace，不能改成成功证据 |
| 16 | C:agent/internal/agentloop/message_loop.go:1066、1089、1130 | 缺 clip 是澄清；disallowed 可写会话/final_gate 后 false |
| 17 | C:agent/internal/agentloop/message_loop.go:1199、1204、1243、1251 | 拆分通过 trace 判已尝试；确认后队列在返回 Result.Continuation 上补写，不能仅序列化 state |
| 18 | C:agent/internal/agentloop/static_mix_gain_staging_context.go:17、44、68、88、119、163、177 | 可执行多次观察并注入 context pack 后 false；pack 依据工程版本判断 stale |
| 19 | C:agent/internal/agentloop/message_loop.go:514、521、775 | transient LLM 暂停与澄清真实返回点；不是终局失败 |
| 20 | C:agent/internal/agentloop/free_state_reasoning.go:902、915、916、1678 | fallback fail 后覆盖专用 StopReason；root/nested 两诊断键同源判定 |
| 21 | C:agent/internal/agentloop/interaction_pause.go:27、46、52 | 正式交互卡从工具结果选等待澄清/确认，确认卡可无 pending tool |
| 22 | C:agent/internal/runtime/runtime.go:14 | 13 个 GoalStatus；并非每个枚举都是当前 Result 返回状态 |
| 23 | C:agent/internal/executor/executor.go:45、57、66 | Harness.Invoke 传 Confirmed/上下文/GoalID/RunID/ToolCallID；adapter 不能直调内核 |
| 24 | C:agent/internal/harness/harness.go:879、930 | 既有 Invoke 及 processor load gate 仍留在执行链 |
| 25 | C:agent/internal/executionruntime/coordinator.go:67、78、115、130、223、259 | proposal/project-cut 冻结校验；稳定动作键；恢复走 Reconcile |
| 26 | C:agent/internal/executionports/staticbalance_vsp.go:79、89、94、113 | 示例 VSP port 要稳定 request key/base_revision/epoch 并回读；不是所有工具都已证明同等幂等 |

`inspect.go` 使用 Go parser/AST 从 Git blob 定位 41 个声明，枚举 runtime 13 状态和 agentloop 非测试文件的生命周期调用点；不运行这些函数、不写 fixture。它只是锚点与调用面复查，不是语义等价或生产验收测试。执行入口（仓库根）：

```powershell
go run ./coord/runs/L1-5-ADAPTER-DESIGN-1/inspect.go
```

本次直接退出码 0，anchor_count=41，13 个枚举与下表逐项一致，108 个 agentloop 生命周期调用点。工具输出 stdout，不额外创建第二份工件。初始只读命令遇到 PowerShell `R` 与 Invoke-History 别名冲突，已用 Read-Part 纠正后读取；不将命令设置失败当源码失败。

## 2. 状态、结果与边界矩阵

**三轴分开**：Lifecycle（继续/暂停/真实终局）、Route（快路径/模型/恢复）、Outcome（质量或失败原因）。completed 只表生命周期结束，不能等价 judgment_ok：project.state disallowed 也可能 r.complete（锚 16 的同族，message_loop.go:1040）。没有结构化成功证据时保留旧 Status/StopReason 和错误信息，标 outcome 待分类，禁止按回复关键词造成功。

T1 仅完整模型工具批结束触发；T2 指 run 终局的结论 retain。runtime EndTurn 是 task/slice turn 结算，不是 T2：旧 result 在暂停也 EndTurn。下表的 EndTurn 均由宿主唯一 owner 负责，新 pull 不再调用一次。

| 旧 Result 状态 / 原因 | 真实产生点（C） | 拟映射 | pending / continuation | T1 / T2 与 owner |
|---|---|---|---|---|
| completed / done | runner.go:710；message_loop.go:1011、1107、1258 | Terminal；route=fastpath 或 model；质量另判 | pending 清；cont=nil | 当前完整模型批可有 T1；宿主 T2 一次，EndTurn 一次 |
| failed / failed | runner.go:724，message_loop.go:474、544 | TerminalFailure；保留 Error/FailureReason | cont=nil；未执行动作不能重新播 | 宿主 T2/EndTurn 各一次；失败批不冒充完整批 |
| failed / model_protocol_failure | runner.go:721、724；message_loop.go:560 设置 failure | TerminalFailure，协议失败独立 | cont=nil | 同上，不混 capability_blocked |
| failed / free_state_terminal_turn_unparseable 或 free_state_terminal_turn_gate_rejected | free_state_reasoning.go:915-916 | TerminalFailure；保留覆写后的 StopReason 和原始诊断 refs | cont=nil | 同上；读取 handler 最终返回，不能在 r.fail 内提前固定分类 |
| waiting_confirmation / needs_confirmation | runner.go:601；interaction_pause.go:52 经 runner.go:569 | SuspendConfirmation | 普通 pending 非 nil；正式交互卡可 nil；必须保存 cont 与交互标识 | 暂停批无新增 T1/T2；宿主可 EndTurn 当前 slice 一次 |
| waiting_clarification / needs_clarification | message_loop.go:1066、775；runner.go:569 | SuspendClarification | 多为无 pending；cont 非 nil；NeedsClarification/Question 保留 | 无新增 T1/T2；宿主 EndTurn slice 一次 |
| waiting_continue / limit_reached，max_turns/max_tool_calls/timeout | runner.go:662、692、699 | SuspendSliceLimit（旧切片限额） | 队列可存在，cont 非 nil | 不触发 T2；已完成旧批不重复 T1；宿主 EndTurn slice 一次 |
| waiting_continue / interjection | runner.go:685 | SuspendInterjection | cont 非 nil；用户新输入由原确认/continue 入口处理 | 同暂停；不能直接吞插话后自动继续 |
| waiting_continue / transient_llm_error | message_loop.go:521 | SuspendInfrastructure | cont 非 nil；保留错误原因；真实 LLM 调用次数与旧 turnsUsed 退款分记 | 无新增 T1/T2；宿主 EndTurn slice 一次 |
| stopped / cancelled | runner.go:673（StopRequested） | TerminalCancelled，显式用户 stop | cont=nil，清未决执行但保留已执行事实 | 宿主原取消终局 T2/EndTurn 一次；不当成 ctx 暂停 |
| cancelled / cancelled | runner.go:681（CancelRequested） | TerminalCancelled，显式用户 cancel | cont=nil | 同上 |
| idle/running/processing/executing/cancelling/stable | runtime.go:14-24 定义；runner.go:392、412、645 等 SetStatus 是运行过程 | Continue/过程快照；**没有发现这些枚举作为当前 result 漏斗的明确终局返回点** | 活 state，不用 cont=nil 猜终局 | 无 T2；无状态变化就造边界 |
| Router stopped=false / Result{} | fastpath/router.go:74；gain_staging_context.go:177 | Continue，无论有无副作用 | 同一 state；即使 Result 为空也重新取 Frame | 无 T2，Router-only 不新增 T1 |
| ctx.Err（不是旧 GoalStatus 枚举） | A:loop.go:397；修复 47ce5d75 各阶段取消分流 | SuspendInterrupted；优先于尚未提交的终局/预算结果 | 新恢复载体必需；旧 Result.Conversation 不充分 | 当次中断无新增 T1/T2；保留已完成先前 T1；宿主结算 slice 可单次 EndTurn |
| pull 全 run 观察预算用尽（旧 Result 无同义状态） | A:budget.go:51、56 | TerminalFailure / budget_exhausted；不能翻译为旧 limit_reached | 新终局不自动续片补额度 | 完整模型批已有 T1；宿主 T2 一次；需主管批准映射到 failed+专用 stop reason |

未知 Status/StopReason 组合：fail-closed 适配错误，保留原数据，零新工具；不能缺省映射 fastpath/judgment_ok。stable 不因为名字像稳定就当完成。ctx 暂停与明确用户 cancel 的差异保持；若二者同时到达，先只读 runtime flags，已明确用户取消按取消终局处理，否则 ctx 按暂停。已提交终局之后收到取消不能逆转历史，只拒绝下一阶段。

## 3. 方案比较与推荐签名

| 方案 | state / import / 私有类型 | 迁移域与成本 | 判断 |
|---|---|---|---|
| S1 宿主持有 state + 中性会话协议 | agentloop → pullharness → 冻结基础包；fastpath 仍泛型、不 import agentloop。只有宿主持有 *runState/Runner/MessageLoop，pull 看有版本的只读 Frame | pull 接口/循环 + 新宿主 adapter + runner 生命周期挂点/恢复载体。保留十 handler，需少量旧漏斗改动，成本中 | **推荐**；消除状态镜像与双执行 owner，适配成本可逐卡验 |
| S2 接口化整个 preflight 状态/执行能力 | 新中性 contracts 包，agentloop 与 fastpath 同依赖它；将 input/trace/goal/executionMemory 等改 getter/mutator，再迁 handler | 十 handler 及众多 helper/continuation 均需改；现 C 的逐字等价基线失效；多个重叠文件，成本高 | 可作后续解耦；不是此次 D 的最小前置 |
| S3 每次 Route 输入/输出 state delta 或完整公共镜像 | pull 不持私有类型，但每轮在 agentloop 镜像 runState；结果要新增 continuation/lifecycle/cost/observations 字段 | JSON/map 镜像难表达返回后补队列、Runtime side effects、私有缓存与局部窗口；需版本校验与显式合并规则 | 单纯结果协议解决不了活 state；若采用完整快照协议，成本接近 S2，且更易漏副作用 |

S1 不是给现 FastPathOutcome 增加一个 Status 就完成。下面是**待主管授权的签名草案**，替换 A 的生产接线占位面；所有新增类型都只在提案中，不已存在、不承诺兼容旧 fake。LLM、PrefixService、ExitExecutor、工具动词、证据门接口冻结不改。host 同时实现 route/模型结果/执行/返回的会话端口，避免只修 Router、却在 Execute 与终局重新出现双 owner。

```go
// Proposed in pullharness; no import of agentloop/runtime/planner.
type Disposition uint8 // Continue, Suspend, Terminal; unknown is rejected
type ReturnKind uint8  // Done, Failed, Confirmation, Clarification,
                      // SliceLimit, Interjection, Transient, Interrupted,
                      // UserCancelled, ObservationBudgetExhausted
type Frame struct {
    Revision uint64
    GoalID, RunID, PrefixSessionKey string
    Context map[string]any
    Conversation []llm.Message
    Budget LedgerView
}
type LedgerView struct {
    NextCycle uint64
    CompletedCycles, ModelCalls, ToolAttempts int
    ProbeSpent float64
    ProbeCostKnown bool
}
type Step struct {
    Disposition Disposition
    Kind ReturnKind
    Source, Entry, Outcome, Reply, Error string
    Calls []ToolCall // only Interpret may produce an unexecuted model batch
    ReceiptIDs []string // already executed, never turned back into Calls
    DraftID string // host stores full legacy Result, including post-return edits
}
type Session interface {
    Snapshot(context.Context) (Frame, error)
    Attempt(context.Context) (Step, error)
    Interpret(context.Context, string) (Step, error)
    Execute(context.Context, []ToolCall) (Step, error)
    CloseCycle(context.Context, string, contextruntime.ExitExecutor) error
    Return(context.Context, Step) (Result, error)
}

// Proposed in agentloop. The adapter owns one state for the whole invocation.
func (l *MessageLoop) newPullSession(r *Runner, s *runState,
    saved *PullContinuation) (*pullSession, error)
func (s *pullSession) HostResult() Result
// PullContinuation wraps existing Continuation plus versioned pull checkpoint.
// Its public transport/persistence field is a separate supervisor decision.
```

`Frame` 的 map/slice 返回克隆只读视图；pull 不修改后写回，Revision 用来拒绝用旧 Frame 装配下一模型轮。`Interpret` 复用现 parseMessageLoopOutput/guard/finalGate/澄清语义，必须能输出 pause/failure；现 Plan(string) []ToolCall 无法表示这些分支或解析错误，因此也是明确接口裁决点。DraftID 为宿主一次性内存句柄，**不能写入磁盘当 continuation**；跨进程恢复只用版本化 PullContinuation。Step/Frame 不展开 raw package，动态事实仅沿 LLMContext/安全摘要。

### 3.1 字段权威源

| 字段/状态 | 唯一 authority 与回流方式 |
|---|---|
| GoalID/RunID/task/slice/runtime TurnID | 既有 runtime + state.goal/input；PrefixSessionKey 以同一 run/session 绑定，恢复不新造 RunID |
| Context/Conversation/State/trace/plan/executionMemory/最近观察 | 宿主活 state；每次 Attempt/Interpret/Execute 返回后重新 Snapshot，pull 不保留独立 history；所有模型消息先经 Interpret 归档一次 |
| stopped/hit/Entry | C TryMatch 原值；stopped 只决定本轮链停止，Disposition 必须从最终 legacy Result+cause 映射，不从 entry 名推断 |
| DraftID/pending/Preview/UndoLabel/澄清/FreeStateDecision | handler 最后返回的 Result；包括 :1251 的 PendingToolQueue 补写；Return 必须验证 draft 尚未提交 |
| ReceiptIDs/成本/工具计数 | 唯一执行 gateway 中实际返回的 receipt；toolAttempts 与 probe 分开，不能用 trace 长度/模型自报推算 |
| cycle/窗与退场动作 | 宿主 pull checkpoint；CloseCycle 是 T1 唯一调用入口，用注入的 p.Exit 并应用真实 ExitReport；prefix 用调用方提供的同一有状态实例 |
| Outcome | 原结构化 FreeStateDecision/verification/错误类别 + 已批准分类映射；source=fastpath 单独遥测，完成不自动变 judgment_ok |

### 3.2 调用序及生命周期切入

1. D goal 入口启动时解析一次模式；push 原调用不变。pull 创建单一 session、注入有状态 Prefix 和真实 Exit；恢复先核验 checkpoint/project binding，不能每次 Route 建新 runState。
2. 先处理原 pendingToolCall/queue：确认权限仍经既有 ResumeAfterConfirmation 语义，`Confirmed=true` 只在已有确认授权后传入；resume 路径改为回同一个 pull session，不能落回旧 l.loop。之后才能 Attempt。diag 旁路不悄悄重排原 pending 队列。
3. Attempt 前检查 ctx/runtime 用户 flags/已用尽 run 预算；用当前 state 调 messageLoopFreeStateDiagnosticOnly，并 SetDiagnosticOnly；调用原 ten-entry TryMatch。handler 的工具请求已经在宿主执行，Step.Calls 必为空。
4. Attempt 的 miss 也取新 Snapshot，成本先结算，状态先回流，然后才装配；Continue 时 Prefix + 当前动态区，overflow 用实际装配/窗计算来源，不能反复用最初的静态 ContextSnapshotJSON。
5. 模型返回先保存实际回复/调用计数，检查取消；Interpret 复用原协议验证，输出暂停/终局或待执行批。Execute 通过同一执行 gateway（r.executeTool→Executor→Harness），不能直接发送工具调用绕过原 guards。
6. 完整模型工具批才 CloseCycle，恰一 T1，实际消费 retain/ref/drop 并更新 history refs/模型窗；批中确认/暂停/取消保留 partial，不 CloseCycle；下一轮仍当前 session。Router-only 批不伪装模型工具循环，不增 CompletedCycles、不发 T1；其结果仍进动态窗，最终 T2 或后续正常 T1/热窗规则处理。该 Router-only T1 边界需要主管采信。
7. 任一 Return 前成本与返回原因已定，宿主提交一次 lifecycle；pull 的 finish 不再独立发 T2。Return 对 Suspend 只保存 checkpoint、按原语义 EndTurn slice，零 retain；对 Terminal 复用宿主结论供给+retainRunObservationConclusions，一次 EndTurn/终局操作。Result.Conversation/Trace/计数从权威 state/ledger 投影，不再次累计。

**实施必要条件：旧 helper 先生成 draft，再最终提交。** 当前 complete/fail 在 result 前已 Runtime.Complete；checkpoint 已 MarkStopped/SetStatus；result 内已 EndTurn/retain。不能事后在 adapter 看见 ctx 取消再声称“零 T2”。建议在 Runner 为 pull 注入私有返回策略（缺省 nil 完全旧行为），在 complete/fail/pause/显式 stop-cancel 分支及 result 的生命周期副作用前分流为无提交的 draft；保留已有结果组装能力。handler 返回后修订的 Result 仍作为最终 draft。原工具观察/验证/状态写回照常进行；这里延后的是终局/切片提交，不是撤销已执行事实。

这不是当前 C 行为变更授权。上述 helper 策略只对 pull 路径启用，push 回归必须逐项证明；若必须修改十个 handler 的领域逻辑才可实现，则停下申请范围。终局事务顺序、draft 纯组装是否继续 buildContextSnapshot、ctx 中断 checkpoint 的返回语义交主管裁决，不默认为实现已经支持。

## 4. 五项不可丢失语义

| 语义 | 最小反例（旧/A 接线可触发的形态） | 拟测断言（尚未实现） |
|---|---|---|
| miss 副作用回流 | gain-staging 三次读工具+pack 后 false；A 继续用原 in/local history，下一模型看不到 pack，再观察一次 | Attempt 返回 Continue 后 Frame.Revision 增；首个模型输入含 pack/安全摘要和已结算预算；下一 Attempt 不重建 fresh pack；工具总数不因 false 加倍 |
| confirmation/pause 零 T2 | clip range split 首 cut 返回 waiting_confirmation，被包成 hit，A.finish 发 T2 | 首 proposal 可执行一次但 apply=0；完整 pending 与余队列保留，T2/retain=0；正式交互卡 nil pending 仍 Suspend |
| 已执行工具不重放 | Router clip.gain.set 已走 r.executeTool，适配又把 trace 转 ToolCall 给 p.Tools.Execute | 同 receipt gateway 调用=1，mutation=1；Route Step.Calls 空；模型批才可带 Calls；再次恢复只处理未决项 |
| 终局唯一 owner | C complete→r.result→retain，A hit→finish 再 T2 | 每 RunID terminal commit=1、retain hook=1、同 slice EndTurn=1；终局原 conclusion 单元/RunID 保持；重复 Return 相同 DraftID 返回原提交结果，不新操作 |
| 每轮同源 diagnostic 刷新 | 首轮 root false，工具更新 nested diagnostic_only=true，缓存开关仍 false | 下一 Attempt 所有十 handler 调用=0；root/nested 各两键分别测试，解除后下一轮恢复；计数仍属于模型路径 |

只计调用次数不足以证明保真：拟测需检查实际下一模型消息、context revision、pending queue、工程账本结论和工具 receipt。一切 fake/fixture 用 t.TempDir；禁止用 sealed 轨道名凑用例。

## 5. 成本、恢复与既有安全执行链

### 5.1 同一个 run 账本

观察预算配置仍沿 ObservationBudget；建议 host ledger 是累计实际消费的唯一 authority，pull 只读披露。初始化一次 ProbeCost，恢复用保存值，不能每次 Run 把初值加一次。每真实工具返回成本在 gateway 按稳定 receipt ID 结算一次，Route hit/miss 和模型批均遵守；Route 返回 Step 不再传一个可重复相加的总 ProbeCost。索引请求已证 index 成本可为 0；未提供计量的 render/probe 成本标 unknown，不能填 0 冒充免费。

检查点：入场/恢复、Attempt 前、每个实际工具前、每个实际工具返回后、模型批完成后、最终 Return 前。已超限入场禁止新增工具/模型；未知成本先保存 receipt 与缺口，禁止继续新增有成本 probe，返回独立计量缺口供主管规定分类。现旧 executor.Result 没有统一 ProbeCost 字段，所以具体工具返回何处可提取可靠成本、计量单位（次数/时长/规范化值）是待裁定/取证点；本卡没有证明生产计量源齐备。估算可用于拒绝新请求，最终消费以实际 receipt 为准；不可退款已执行动作。

旧 MaxTurns/MaxToolCalls/Timeout 是可续片限额，与全 run MaxCycles/MaxProbeCost 分开：旧 limit_reached 暂停不补/重置 run 观察预算。MaxCycles 只计完整模型工具批；快路径受 probe 和既有工具次数/timeout 限制，不靠加“假模型轮”限流。Router 执行到限额时须停止其下一个 tool，不能等整个 chain 返回才检查。实际成本不可提前得知时可能最后一个调用越线，记录 overshoot 后终局，不能假称硬预授权上限。

竞争优先级：明确用户取消→取消终局；ctx 中断→保留事实/暂停；普通确认或切片暂停→Suspend；全 run 观察预算用尽→budget_exhausted 终局。若普通确认和全 run 预算用尽同时出现，建议全 run 限额先拒绝后续 apply，保留 pending 证据而不开放自动续跑；暂停展示/终局策略需要主管明确。零成本纯回复遇已用尽预算能否继续属于 Router 豁免裁决；推荐无豁免，统计一致。

### 5.2 需要新增的恢复载体与边界

原 Continuation 已携 Context/Conversation/trace/pending/plan/旧预算/ExecutionMemory/recent observation，但缺 probe ledger/cycle 窗/边界提交状态，且 Continue 会新建 slice、增旧限额并回旧 loop。不能宣称“Conversation 存续=恢复幂等”。建议新增 `agentloop.PullContinuation`（transport 字段由主管确认），包含以下内容：

| 部分 | 必须保存与恢复 |
|---|---|
| 身份 | schema version、harness_mode、GoalID/RunID/task/slice/runtime TurnID、ContinuationID、单次消费 generation、项目 epoch/revision 与 history/worktree 绑定；prefix session key |
| 宿主活状态 | 既有 Continuation 全体；最终 returned Result 上补过的 PendingToolQueue；Executed receipts、mutation barrier/replanAfterTool、consecutiveErrors、协议 repair 状态、影响后续行为的 observePresentation/semanticAction 或明确安全重建依据 |
| pull 状态 | NextCycle 单调序号、完成模型批数、实际模型调用数、ToolAttempts、ProbeSpent/known/上限、已结算 receipt IDs、正在进行批的 BatchID/游标/完成 receipts、尚未执行工具及确认授权关联 |
| 窗与边界 | 当前 hot window/字节/units 与真实 ExitReport 应用状态、T1 已提交 BatchID 集合、terminal commit token、EndTurn 对应 slice 去重状态；未完成批不标已完成 |
| 稳定装配 | 首装冷启动的原字节与四层 section 身份、PrefixService 同会话快照/指纹或受控重新播种信息，完整 AssemblyReport 延续；不是每恢复重新取世界快照当“恒定底座” |

循环 ID：用 `<RunID>:cycle:<NextCycle>` 为**分配身份**；partial 也占一个唯一 ID，但 CompletedCycles 不增加。暂停保存同 BatchID，续跑不重新执行已确认 receipts；完成后一次 T1 再递增完成数。新批分配下一 ID，不能用恢复时从 0 起的 completed count 生成身份。重复 ContinuationID 同 generation 要返回已记录状态或冲突，零新工具/零新计费/零边界；续片不能仅用 r.Continue 无条件新开 slice。

跨进程 crash 的 apply 后 receipt 未保存：必须走已有执行协调器的 Reconcile/回读，不得仅凭 continuation 的 trace 判断未执行然后重放。现 preflight 的普通 legacy tools 未全证明支持稳定 VSP 幂等/恢复；有 unknown apply 时 fail-closed，交主管追加工具腿取证，不承诺 exactly-once。内存 DraftID/PrefixService 指针不能序列化；第一版若只允许同进程恢复，须显式拒绝跨进程 pull checkpoint 并标出产品边界，仍不能重放已执行动作。

持久化兼容：旧 continuation 无 pull 字段一律解释为原 push 语义；新 pull schema 未知版本/枚举、字段缺失或项目绑定不符拒绝执行，保留数据并报具体原因；不默默转 push。需要旧工件加载/往返测试、新版轮回测试。terminal/ref 保留失败不得隐藏为成功；receipt/窗口/边界原子提交不足时需要恢复核对，不声称单靠去重 set 有崩溃安全。

### 5.3 权限、证据、版本与幂等仍留既有调用点

宿主复用 `allowedTool`、`messageLoopToolGuardIssue`、read-only mutation barrier、`checkpoint/checkToolBudget`、`r.executeTool` 的 verifier/executionMemory/recentObservation 同步。实际请求必须经过 `executor.RunToolCall:45` → `Harness.Invoke:879`（Confirmed 和上下文/ID 传递），processor load admission 仍在原 gate；不能把 adapter 的命中当权限授权或凭回复增 readiness。

需要实验的语义仍经原 FreeStateDecision.Validate/证据门，不由 FastPath 名字替代 G1-G8，不扩大 CCB view 选择；缺失/partial/stale 保留。工程变更后沿 r.executeTool 的 mutation barrier 重观察/重绑与 gain-staging pack revision 检查，不能自动沿用旧 AllowedTools/旧请求 cut。

编排执行仍由 executionruntime.Coordinator 校验 FrozenPlan/ActiveProposal/ProjectCut（:67/:78），用 session/action hash 稳定键（:115/:130），恢复走 Reconcile（:223/:259）；VSP port 示例仍做 base_revision/epoch/readback。这里只核实该示例，不把其幂等保证推广到所有 clip/stems handler。本卡不改权限/证据门/工程 CAS/VSP 协议。

## 6. 给 B/D 的最小文件域与建议拆卡

取消修复域=loop.go/loop_test.go（或取消测试）+本卡独立回执；现在修复已 done 待审，依旧不领取或覆盖该域。B 仍 todo，依赖 A 已验收合入；B 原卡容许填 loop.go 槽。下表为主管可用的**建议卡**，本执行流没有发卡授权动作，不修改原 B/D，也不改变排序。每行最多 5 个生产/测试文件；回执工件独立目录，若总粒度超过五文件再拆。

| 阶段 / 建议卡 | 生产与测试最小域（相对 agent/internal，新增文件为计划） | 依赖与验收入口 |
|---|---|---|
| 接口准备 / ADAPTER-PORT | pullharness/session.go、session_test.go（新增）；loop.go、loop_test.go | 主管批准 S1/矩阵/接口解冻；A+取消修复先验收提交。计划 `go test ./internal/pullharness -run TestSession -count=1` +原取消回归；不得 B 并行改 loop |
| 状态适配 / ADAPTER-STATE | agentloop/pull_session.go、pull_session_test.go、pull_continuation.go、pull_continuation_test.go（新增） | PORT+C 阶段已裁定；计划 `go test ./internal/agentloop -run 'TestPull(Session|Continuation)' -count=1`；miss/队列补写/旧工件/重复续跑 |
| 执行 / ADAPTER-EXEC | agentloop/pull_execution.go、pull_execution_test.go（新增）；runner.go | STATE，独占 runner.go；tool gateway 计量/权限/receipt 幂等/unknown apply，零执行层内部改动。计划 `go test ./internal/agentloop -run TestPullExecution -count=1` |
| 生命周期 / ADAPTER-LIFE | agentloop/pull_lifecycle.go、pull_lifecycle_test.go（新增）；runner.go、exit_retain.go；pullharness/loop.go | EXEC 后提交，独占 runner/loop；延迟 draft/唯一 Return/T1/EndTurn/T2、ctx 与用户 cancel。计划 `go test ./internal/agentloop ./internal/pullharness -count=1`，原 push 回归 |
| B 装配与退场配合 | 原计划 coldstart/disclosure/exit_wiring 与各测试需分 B1/B2，每卡 <=5；只消费批准后的 session Frame/CloseCycle 槽，不持第二套 state | B 范围/顺序由主管改卡；冻结三包零改；真实 ExitReport 到模型窗的测试，Prefix 首装+恢复字节恒定。计划入口由拆分卡写实，不虚称存在 |
| D goal 路由 / ADAPTER-ENTRY | agentloop/message_loop.go、pull_entry.go、pull_entry_test.go（后两新增）；pullharness/mode.go、mode_test.go 仅若确需新配置语义 | LIFE+B 收口；依据具体现入口决定是否 mode 两文件必要，最多5。启动读取 flag；Run/Continue/ResumeAfterConfirmation 同模式；chat 不迁移 |
| G3 / D-SMOKE 与 D-JOURNEY | 只在现 scripts/dev_agent_smoke.ps1 体系新增/扩参，分别 <=5 个脚本/断言文件；报告另卡只读 | ENTRY 后，真实栈串行占用记录。计划 harness_ab 入口尚不存在；质量/成本/前缀三层指标、双模式 >=3 轮/模式、确定性断点两次止损 |

所有生产实现卡须有合适 build/局部/全量 checks；已接线改动交付前必须真实栈脚本 exit 0，渲染/旅程覆盖边界显式声明。上表正则是**计划测试名称**，本仓当前没有这些测试，不能用 no-tests-to-run 当通过。D 默认仅 goal 入口，规格 OQ-H3 的 chat 迁移在 G3 之后另卡；不把“双模”误写成已迁两生产入口。

## 7. 提案验收场景表

以下是后续实现必须冻结的反例与断言，不是本卡已通过的测试。计数均为本场景新增量；T2=retain hook，E=runtime EndTurn，M=实际模型调用；工具次数指执行 gateway 调用，提案/观察与实际 mutation 分开计量。除 Router-only 约定，T1 用完整模型工具批定义。

| 场景 / 输入 | 工具与上下文预期 | 边界 / 成本预期 | 分类与结果 |
|---|---|---|---|
| 终局命中：能力契约纯回复 | tool=0，M=0，trace 多一 final_gate；同 state | T1=0，T2=1，E=1，probe=0 | Terminal，source=fastpath；质量无未经证实 judgment_ok |
| 终局命中：单 clip set 成功 | proposal/apply 按实际授权；本例已授权 apply=1；Tools 不二播 | T1=0，T2=1，E=1，receipt cost 仅一次 | Terminal fastpath；保留 verification/engine 状态 |
| 暂停命中：工具限额或 timeout | guard 前 tool=0（若前有已执行调用则保留原 receipt）；queue/Context 不丢 | 新 T1/T2=0，E=1，run ledger 不重置 | SuspendSliceLimit，cont 非 nil，不映射 budget_exhausted |
| 确认命中：range split 首 cut proposal | proposal=1，apply=0，M=0；pending call+剩余 cuts+Preview 原样 | T1/T2=0，E=1，proposal 成本记一次 | SuspendConfirmation；confirmed resume 才执行下一合法动作 |
| 正式确认卡无 pending tool | tool 返回正式 interaction 1 次；保存交互上下文，不能走 ResumeAfterConfirmation 强要求 pending | T1/T2=0，E=1，成本不丢 | SuspendConfirmation；使用原交互卡回流入口 |
| miss 有观察副作用：gain-staging pack | 本 fixture 三读调用=3、M 后续1；最新 pack/trace/State 入模型；fresh pack 下一轮不再读 | Router T1/T2/E=0，probe=sum(unique receipts)；后续完整模型批才 T1 | Continue，零 Shortcuts 终局计数，Route 有工作可另记尝试 |
| 无匹配无副作用 | handler 各试一次，tool=0，Frame 不伪增内容；进入一模型轮 | T1/T2/E=0（模型返回前），probe 不变，M=1 | Continue，非 fastpath 成功 |
| ctx 入场已取消 | Route/LLM/tool=0，保留原 state | T1/T2=0，无新物理成本；若没有开启 slice 则 E=0 | SuspendInterrupted，保存恢复身份，不造终局 |
| Router 执行中取消+一 receipt | tool=1、不再下一工具/handler/模型；保留已执行结果和 Context，partial 游标 | 新 T1/T2=0，E<=1 按已开 slice；cost=真实1笔 | SuspendInterrupted 优先于尚未提交 completed/预算 draft |
| 明确用户 cancel/stop | tool=0 after checkpoint，保留先前 receipts，cont=nil | T2=1，E=1，不新增 T1、不退款 | TerminalCancelled，保持与 ctx 中断区别 |
| 重复续跑相同 continuation generation | 已提交 receipt 不二播；第一次确认可 apply=1、第二次新增=0；最新 queue 保留 | 同 BatchID T1<=1、同 terminal T2<=1、同 slice E<=1，probe 不重复 | 原缓存结果或显式冲突；不多开 slice补额度 |
| diagnostic-only 每轮 root/nested 改变 | 下一轮所有 handler=0；pending 原 guard 路径另验；模型走 CCB 显式 view | Router T1/T2=0、tool=0，probe无额外旁路成本 | Continue，两个 root/nested 键均同源；关闭后恢复尝试 |
| 全 run probe 已用尽 / Router 返成本越线 | 入场用尽 tool/M=0；返成本越线保留本调用事实并禁止下一调用 | Router 无 T1，Terminal T2/E=1，probe 如实可能 overshoot | budget_exhausted 单列，不混 no_candidate_found/切片暂停 |
| 模型完整工具批与 partial 恢复 | 完整批按实际工具数，history/窗应用 ExitReport；partial 保存游标不回放 | 完整批 T1=1；暂停=0；后续新批 ID 单调；终局 T2=1 | Continue/Suspend/Terminal 分记，completed cycles 与 allocated IDs 不混 |

## 8. 待主管裁决与本卡交付判断

1. 采纳 S1 及 C 当前“宿主注册面+纯 helper 平移”的阶段边界；三副本与 panic 例外仍由 C 最终验收裁定，本提案不替主管接受。
2. 批准 A 的 Route/GoalInput/Result/ToolExecutor 生产适配协议改动和 B 协作重排；先验收 A 取消修复，任何 B/D 领取仍按已合入依赖。
3. 批准 pull-only draft 策略与宿主唯一终局 owner；runtime EndTurn 在暂停保留，T2 仅真实终局；用户 cancel 与 ctx 中断的竞争口径。
4. 明确 Router-only 不触发模型批 T1、同 run probe 无豁免、切片暂停与 run budget 区分、确认与预算竞争顺序；定可靠计量来源/单位与 unknown 分类。
5. 明确恢复载体 transport/持久化域与第一版同进程/跨进程边界；旧记录缺省 push、未知 pull fail-closed；普通工具 unknown apply 的停止与取证卡。
6. 完成/失败的结构化 Outcome 映射与 fastpath 遥测口径；不能用 completed 一刀切为 judgment_ok，chat 迁移仍另卡。

本卡执行自验标准：六项必交内容齐备、真实返回点及至少8处原提交锚点可回查、两方案以上比较与推荐可拆卡、至少八场景计数/状态/边界/成本齐备，且写域仅 PROPOSAL.md、inspect.go 与本卡状态。机械取证 exit 0；文档 diff check 应 exit 0。未跑全量 Go 或真实栈，符合纯提案卡口径；本交付**只证明提案与取证齐备，不证明 A+C 集成、暂停恢复幂等或 G3 达标**。最终设计采信和后续授权待主管。

执行取证命令 exit 0；必交内容机械检查 exit 0；gofmt -l inspect.go exit 0、无输出。提交前对两个交付文件执行 staged diff check 并在本卡回执记录结果；报告提交仅此目录，不向 main 推报告或源码。主树已有改动未触碰，协调状态提交仅本卡 todo/doing/done 路径。
