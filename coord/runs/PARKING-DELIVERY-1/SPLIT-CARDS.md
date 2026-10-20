# PARKING-DELIVERY-1 实现卡拆分建议（交主管定稿）

依据=[FORENSICS.md](FORENSICS.md) 三问归因。拆分原则：根修（交付门谓词泄漏）单卡闭环；验证面收口独立卡（ps1 面，与 Go 面文件域不相交可并行）；「模型 reply 兜底」为产品语义决策，先裁定后立卡。

## 卡 A（建议池序新卡，P1）：BUDGETSTOP-CHAINEND-DELIVERY-1——预算停 settle 清理 goalContinuations legacy 泄漏，恢复 D1-STALL-1 投递

- **修法（主案）**：`agent/internal/chat/goalrunner_chat.go` 预算耗尽 settle 段（3265-3299，`autoContinuationBudgetExhausted=true` 处）补 `delete(s.goalContinuations, durable.GoalID)`——与 3213-3215 的 arm 对称撤除。语义依据：goalContinuations 的语义是「下一自动片的 arm」，budget stop 后该语义不成立（loop 已 blocked、全记录 completed、不再排片）。
- **不改谓词总判定的理由**：`goalHasLiveContinuationOwner` 的 "unknown arm shapes keep counting as live" 是有意设计（FIX-F2-SURFACE-REPLY 先例=对确认形态加例外而非改总判定）；改总判定影响 turn_close_guard 与 scheduler 两个消费点，风险面大。budget stop 处撤 arm 是更窄、语义更准的修复。
- **领取侧核对义务（卡内写明）**：①`goalContinuations` 的全部读点（含 bare「继续」续跑入口 shouldResumeGoalFromStatus 相关路径）确认 delete 后用户「继续」仍可续跑（goal=waiting_continue 承认的入口不依赖该 map 条目——预期如此，须核对证实）；②对照 3315-3379（res.Continuation==nil 分支）的同类清理（3357 有 delete）确认两分支清理对齐。
- **文件域**：`agent/internal/chat/goalrunner_chat.go` + `agent/internal/chat/d1_stall_settle_test.go`（或同包新测试文件）。
- **验收面**：
  1. 单元红→绿：复刻 recordGoalResult budget-stop 形态（loop active、res.Continuation 非 nil、used>budget 且 lock 已存在）后断言 ①`goalHasLiveContinuationOwner(goalID)==false` ②`settleAndDeliverContinuationChainEnd` 经真实 chainEnded 求值（或 runDurableContinuation 形状）投递 chain_result 事件且 body 含「继续」/「还没做完」needle（即今晚缺失的 D1-STALL-1 回执）；既有 d1_stall_settle / judgment_park_delivery / scheduler_terminal_delivery / scheduler_failure_delivery 测试全绿；
  2. `go test ./... -count=1` 0 FAIL + gofmt 净（LF blob 口径）；
  3. 真栈腿（按 AGENTS §5）：`scripts/d1_stall_repro_smoke.ps1` 复现——到达 budget stop 形态时分类=parked_owe（exit 0）而非 timeout；**到达方式的概率性如实声明**：真实 LLM 连续不收敛是尾部事件，若现成配置可注入低 continuation budget（`loop.ContinuationBudget` override 或小预算话术）则用确定性注入，否则按 §8 概率性口径写明轮次与止损线。端测边界声明照 109 卡先例（覆盖到哪个 settle 边界、未覆盖哪段下游）。
- **预估**：0.5 天（单元+真栈各半）；依赖=无。

## 卡 B（P2，可与 A 并行，文件域不相交）：STALLSMOKE-NO-DELIVERY-CLASS-1——停摆烟测给「零 chain_result」形态独立缺陷分类

- **修法**：`scripts/d1_stall_repro_smoke.ps1` 判定循环（223 行段）新增形态：`goal=waiting_continue && live==0 && chain_result==0` 稳定持续 ≥N 个轮询周期（建议 N=3，与 KernelDwell 类参数化）→ 新分类 `parked_owe_no_delivery` → **exit 1**（缺陷类）。既有 parked_owe/parked_owe_silent 语义不变。
- **理由**：现状该形态落 timeout(exit 2 环境类)，交付面死亡被环境分类吞掉（问③-(b)）。109 卡 §8 纪律要求环境/缺陷分开记录，本卡把「链停且零投递」从环境类挪回缺陷类。
- **文件域**：`scripts/d1_stall_repro_smoke.ps1`（+其自验方式：与卡 A 真栈腿同一栈复用，或注入复现）。
- **验收面**：脚本对 healthy 停摆仍 exit 0（回归）；对零投递形态 exit 1 且 verdict 行含新分类名（工件=console+verdict 行）。**建议排在卡 A 之后跑**（A 修复后 healthy 路径才可复现 exit 0）。

## 卡 C（裁定备忘，先裁定后决定是否立卡）：预算耗尽回执是否附「模型最后被拒 reply」

- 两案并列交主管（FORENSICS.md 问②尾部）：
  - **案 1（现状设计，卡 A 修复后即生效）**：只投五要素欠账回执（d1OwedEvaluationReply），不附被拒 reply。优点：治理口径单一（回执明说 A/B 卡未建、「继续」入口）；缺点：模型的 A/B 操作指引（旁通/启用对比的具体听法）不达用户。
  - **案 2（增强）**：回执后附引文块「模型最后的交接说明（被系统形状门拒收，未经治理验证）」。优点：用户拿到可执行试听指引（干预确实已应用，操作层试听可行）；缺点：prose 与治理状态并置有「两个声音」风险，且被拒决策对象与 prose 的一致性无验证。
  - 执行侧倾向：案 1 先行观察（卡 A 已恢复交付），用户反馈「回执不够用」再议案 2——不入本批拆卡。
- 若裁定立案：文件域=`agent/internal/chat/continuation_scheduler.go`（owedEvaluationChainReply 附近）+ 对应测试；验收面=回执文本含引文块断言+sanitize 守卫（stripInternalTerminalTerms）不漏内部代号。

## 次生面（不立卡，随卡 A 自然恢复）

- turn 事件面（turn_trajectory_events.go:40-54 waiting_continue turn.completed 的状态映射）被同一 legacy 泄漏挡；卡 A 修复后该面自动恢复，无需单独改动。d1_stall_repro_smoke.ps1 的 turn_terminal_events 证据行会在真栈腿顺带验证。

## 排序建议

A（根修，P1）→ B（验证面收口，P2，真栈腿可与 A 验收栈复用）→ C（观察后议）。A+B 合计 ≈1 天；均不触碰 FS 状态机收敛面（T10 边界外）。
