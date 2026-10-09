# L1-5-IMPL-A-CANCEL-FIX-1：PullLoop 取消边界修复（已授权可领取）

- 发卡：Codex 辅助决策流 / harness 共享池审查会话 / 2026-10-09（Asia/Shanghai）。
- 派发确认：已确认。用户核查回执后本轮明确“可以继续推任务”，授权继续派发本缺陷修复；独立执行流按用户指定卡 ID 领取。2026-10-09 修订保留原未授权提交 d45f6b17，不追改历史。
- 验收负责人：现有主管决策流；A 原卡最终裁定、修复合入和 B 解锁仍归主管。
- 目标仓库：D:\Vit_DAW（或 Mac 同仓独立 worktree）。
- 优先级 / 预估 / 依赖：P1 / 30-60 分钟 / 用户本轮授权已满足；基于被审 A=de1d876c 修复，不依赖 A 已合入。首选可执行修复卡，原 A 最终验收仍待主管；不解锁 B。
- 文件域：agent/internal/pullharness/loop.go、loop_test.go（或同包新增取消测试），以及 coord/runs/L1-5-IMPL-A-CANCEL-FIX-1/ 回执/日志。不改 budget.go/mode.go、冻结三包、原测试期望或 B/C/D 设计；新增总计不超过五个实现/测试/回执文件。
- 并行与资源：与 B 同包/loop.go 重叠，禁止与 B 并行编辑。C 只读复核域独立；零真实栈，本卡不启动运行栈。

## 已证事实与待定边界

- 被审 A=de1d876c，零生产消费；三取消反例经执行复核与发卡侧独立复跑两份证据坐实。
- 依据：coord/runs/L1-5-IMPL-A-REVIEW-1/REPORT.md @7b567089；辅助核查 coord/reports/2026-10-09-L1-5-AC-review-audit.md；独立反例 coord/runs/L1-5-REVIEW-AUDIT-20261009/a-cancellation.txt。
- 已取消入场仍 Route；Complete 取消后 error=nil 仍执行工具；Execute 取消后落 budget_exhausted 并 T2。原卡要求中断保持 interrupted、不触发终局 T2。
- 本次窄修口径：取消被观察到之后结果必须 interrupted、禁止启动下一阶段、禁止走 finish/T2。已返回的工具结果/成本/模型回复按既有 Result.Conversation、Reply、ProbeSpent、Trace 保留；不伪称取消能撤销已经执行的动作。取消批不新增“已完成轮次”或 T1（设计 §2 暂停/续跑不触发），此前正常完成批的 T1/计数保持；正常无取消路径的原预算与终局规则保持。该口径只覆盖骨架边界，真实批内执行进度/continuation 幂等归适配设计，不新增字段或改接口。

### 取消路径验收表（执行前冻结）

| 路径 | 必须断言 |
|---|---|
| Run 入场 ctx 已取消（Router 非 nil） | Route/LLM/Execute 调用数均0，Outcome=interrupted，T1/T2均0 |
| Complete 取消 ctx 后仍返回 reply/error=nil | 保留已收到的 reply/会话行；不执行新工具，interrupted，无新 T1/T2 |
| Plan 阶段取消 ctx | Execute 调用数0，interrupted，无新 T1/T2 |
| Execute 取消并返回空结果、预算恰达到上限 | interrupted，不改为 budget_exhausted；无新 T1/T2、不计完成轮次 |
| Execute 取消并返回已执行结果及非零成本 | 已返回 ModelLine/成本/ID 有可回查载体，interrupted，不丢已执行事实；无新 T1/T2、不计完成轮次 |
| Route 执行中取消并返回 hit/Reply/非零成本 | 保留返回 Reply 与成本，interrupted，无T2；不得重新执行 Route 或 Tools |

取消检查只能阻止其后新阶段，不承诺与外部取消完全无竞态；执行器仍有自己的取消与权限职责。不得在本卡新建回滚、退款或恢复协议。

## 实施与验收

1. 领取前 fetch 并复查 A 修订版本、B 是否在飞。从最新共享 main 建立独立 worktree（建议 port/l1-5-impl-a-cancel-fix-1）；若 A 尚未合入，仅应用 de1d876c 为实现底座，记录对应本地提交，再追加本卡修复 commit。禁止把原 A 未验收实现推 main，主管按原 A→修复的依赖顺序验收合入。
2. 在 Router 前、模型返回后/工具执行前、工具批返回后的生命周期边界补足取消分流；中断优先于预算/终局分类。明确批完成事实如何保留。不得以 finish 无条件改判造成真实结果被覆盖；代码契约有争议即上交。
3. 将上表六路径转为正式回归，用同步 fake 在指定阶段 cancel，不靠 sleep；新测试修前必须断言级红（不得编译错误代替），修后全绿。正常 Router/无工具终局/模型错误/工具批 T1/预算 T2 原测试继续通过，不改变原期望。
4. 在 agent/ 执行 go build ./...、go test ./internal/pullharness -count=1 -v、go test ./... -count=1，直接退出码均0；提交 blob gofmt 净，附被测 HEAD/status、修复 diff、反例修前红/修后绿、正常路径回归和结果保留口径。
5. 回执明确本卡只修骨架取消路径，未证明生产中断恢复、A/C 适配或 G3 真栈。零生产入口消费须重新核对；若已有生产接线，本卡既定单元级边界失效，上交主管安排真实栈门槛。

- 停止条件：上述结果保留口径无法通过既有字段表达，或必须改 ToolExecutor/FastPathRouter/GoalInput/Result/continuation 持久化；新证据推翻取消批规则；A 版本变化；文件域与 B 冲突。附最小反例与两种方案，上交，不自行扩域；当前窄修不解决 A/C 全量适配。
- 领取：用户本会话明确“执行 L1-5-IMPL-A-CANCEL-FIX-1，按最新卡面和共享协议领取”；2026-10-09 14:02 +08:00；origin/main=7e230ab92924295cba5f01cd812f7afbb10e4104；owner=Codex GPT-6 / Windows ZTY / 会话 01a11f3f-20bf-7092-811f-582e67f23c0f（独立执行流）；分支 port/l1-5-impl-a-cancel-fix-1；worktree C:/Users/timoz/.codex/worktrees/l1-5-cancel-fix/Vit_DAW；领取基线 HEAD=7e230ab92924295cba5f01cd812f7afbb10e4104，status --short 与 diff --stat 均空；协调 checkout= C:/Users/timoz/.codex/worktrees/l1-5-cancel-fix-coord/Vit_DAW；同步提交=本条 owner 记录所在提交（后续回执回填 hash）。A 尚未合入，仅 cherry-pick de1d876c 作为实现底座；未触碰主工作树已有改动；B 在 todo，无 doing 文件域冲突。
- 领取同步确认：b07da14ccf31930a318f5cf9c2782011768a1692 已成功推 main，fetch 重读远端 owner 与本会话一致。
- 回执：执行自验完成，待主管最终验收；修复 commit **47ce5d75**（已推 origin/port/l1-5-impl-a-cancel-fix-1）；原 A=de1d876c 的本地 cherry-pick 底座=231a4ff3，主管按原 A→47ce5d75 顺序验收合入，不重复合入底座副本。修前六条取消路径+此前正常批回归均断言级红（exit 1），修后 `go test ./internal/pullharness -count=1 -v`、`go build ./...`、`go test ./... -count=1` 均 exit 0；staged blob gofmt 净，源码/回执 diff check exit 0，冻结三包 diff 空。详见执行分支 `coord/runs/L1-5-IMPL-A-CANCEL-FIX-1/receipt.md`、red.txt、validation.txt。取消批保留 Reply/Conversation/ModelLine/ProbeSpent，返回工具 ID/Tool/Status/单笔成本记录在 Trace；不增加完成轮次、不新增 T1/T2，此前正常批保持；取消优先于预算终局。零包外生产消费复查通过，未启动真实栈；只证明骨架取消路径，未证明生产中断恢复、A/C 适配或 G3 真栈。本卡五个实现/测试/证据文件；主工作树已有改动未触碰；实现 worktree 提交后 status 空。原 A 最终验收与 B 解锁仍归主管。
- 辅助核查：Codex 发卡流已亲读 diff、独立复现修前断言级红/修后21测试绿与原三反例绿；建议主管通过本窄修，详见 coord/reports/2026-10-09-L1-5-cancel-adapter-audit.md。非最终 ruling、未合入、不解锁 B。
- 验收：（主管裁定 / 修复合入 / A 原卡与 B 依赖状态另记）
