# Ruling：FIX-REPAIR-CLARIFY-DEATH-1 pass（2026-09-24）

- 卡：`coord/cards/done/2026-09-24-FIX-REPAIR-CLARIFY-DEATH-1.md`（Mac 决策会话开卡，mac 执行侧执行，Mac 决策会话验收）
- 提交：领取 `f65ab63` / 回执 `dc06edb` / 实现 `54bcfec`（port/fix-repair-clarify-death）经决策侧 cherry-pick 入 main
- 裁定：**pass**——「repair-clarify 协议违约判死」族修复落地：违令先获一次禁-clarify 强化 repair，二次合规轮继续、二次仍违令才判死；F4 判死路径锁定不变

## 决策侧核验（独立复算）

1. **红绿真实性**：红测试以 `model calls=2 want=3 (original + violating repair + one reinforced repair)` **精确红在缺失的强化调用**上（非泛红）；双违令修前 FAIL（无强化路径）→修后 calls=3 暂停待 557 PASS；红2（真畸形不可修复）修前 PASS=F4 判死锁→修后不变 PASS——三态齐。**分支 tip 决策侧 worktree 亲跑 `go test ./internal/agentloop -run 'Repair|Clarif'` ok**。
2. **实现选择（强化 repair）采信**：追加修复点在 message_loop repair 分支（唯一持有 LLM client+会话状态的层）；chat 557 判死路径原样保留为二次违令终态（既有 `TestAudioClosureRepairClarificationBecomesProtocolFailure` 锁定）；剥离降级作为强化 prompt 的诚实出口（两方案最深处汇合）——与卡面倾向一致且理由完备。
3. **两个执行面决定采信**：①违令+强化计一个 repair episode（`MaxModelProtocolRepairs=1` 预算下计 2 会让合规恢复被误 settle——预算法细节的正确处理）；②强化遥测 `source=message_loop_repair_clarify_reinforce` 独立可辨（未来取证直证）。
4. **raw 落盘直证化**：repair 成功分支 `repair_succeeded` 原文+violation/reinforce 行落既有 messageLoopDiagnostic 面——FORENSIC 卡声明的证据边界（repair 原文未落 durable）就此闭合，零 durable schema 改动。
5. **chat 层零 diff**（diff stat 实证）——与"settle/F2 零纠缠"声明一致；contextruntime /var 唯一 FAIL 经基线 worktree 复核归属 CTXSYMLINK 域正确；webui 324/324。
6. **勘误采纳**：卡面与取证报告的 `message_loop.go` 包路径笔误（chat→实为 agentloop）由执行侧更正——**取证报告 FORENSIC_REPORT.md 的同款引用随本 ruling 更正为 `agent/internal/agentloop/message_loop.go`**。

## 遗留移交

- 10 轮 spot 按卡面许可并入下轮统计批观测（新失败族修复后的复发率面）。
- 共享 chat/agentloop 代码单边开发两端合入：PC 侧拉 main 即得；若 PC 统计批观测到同族复发，遥测 source 可直证是否走到强化分支。
- mac 队列剩余：FIX-KERNEL-PLUGINLIST-HYGIENE-1（P2，双端共建）、FIX-GD-TELEMETRY-BELL-1、FIX-TEST-CTXSYMLINK-1；AUTOSWEEP（PC 开发中）合入后 mac 跑首轮 sweep。
