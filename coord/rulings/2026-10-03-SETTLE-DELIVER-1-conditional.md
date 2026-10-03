# Ruling：SETTLE-DELIVER-1 — conditional pass（实现面四验全过；单场 exit-0 烟测腿+用户手测复验为转正条件）（2026-10-03 决策会话）

## 验收四层（全部我方亲核/亲跑）

1. **diff 亲核**（3035c8c5@port/settle-deliver-1，产品 6 文件+测试 4 文件+烟测脚本/台账/工件）：
   - 症状 A（agent）：`recordJudgmentSettlementReply`（audition_events.go）——settle 回复直写 history（纯文件 checkpoint+vit 节点 kind=assistant/logical_message_id=judgment_settle:&lt;evidence&gt;；零内核导出零 journal 污染论证合理：settle 片已付变更 checkpoint）；失败仅 WARN 不回滚终态 ✓。webui：`settlementMessagesFromEvents` 事件路由进 messages（GUI-F7 同语义）+活动线去重（messageLifecycle 跳过 settlement_reply）+水合孪生同键合并 ✓。
   - 症状 B：`audioClosureLoopOwnsRound` 所有权门——已 settled/stopped 实验或他 goal 的 loop=历史审计数据不再回放进新闭包；goal 身份缺失时保持历史行为（无证据不猜测）；**守卫零放宽**（真跨修订观察仍 stale，反钉测试在案）；postActionObservationRequired 不再从他人 loop 借位 ✓。
   - 症状 C：renderPlan 无锚组不按 user/rest 切分、整组保 createdAt 序（取证流回放实锤的插队根因）✓。
2. **我方独立复跑**（验证 worktree 亲跑）：go build ✓；chat 全包 107.7s ok ✓；**全仓 go test -count=1 EXIT=0 零 FAIL** ✓；webui vitest **420/420**（36 文件）✓；tsc --noEmit ✓；vite build ✓（4.65s）。
3. **RED 抽验（我方亲做，逐钉摘修复）**：症状 A——还原 main 版 audition_events.go 后 TestJudgmentSettlementReplyPersistsAsConversationMessage FAIL（rows=0），恢复转绿；症状 B——还原 main 版 audio_closure_controller.go 后 TestSettledLoopLedgerDoesNotStaleSettleNextGoalClosure FAIL（Reason=project_revision_stale，**手测实栈症状的精确复现**），恢复转绿。
4. **工件亲读**：run 121934 `judgment.settled` 事件带 `settlement_reply=true`+logical id 同键+结算报告全文；draft 工程 conversation_graph 节点 `n_20261003T042445_85c8934e` kind=vit/message_kind=assistant/logical_message_id 同键/durable/project_history——**症状 A 真栈双面（事件+持久化）独立证实**；烟测台账 9 轮逐轮定性（脚本成熟度与模型随机分支分开记账，无倒推验收）；泊位声明核实（验收时全进程/端口已空）；工程哈希前后一致（零污染）。

## 裁定

- **conditional pass**。转正条件：**用户手测复验**（判定→**对话流可见结算新回复**→新输入正常进治理链执行→**刷新后结算确认仍在**——同场顺带完成 JUDGMENT-SETTLE-STALL-1 与 AB-JUDGMENT/FS-ADOPT 终验）；**单场 exit-0 烟测**可由该手测场替代收口（AGENTS §8：2/9 轮到判定席为上游 LLM 随机分支，非本卡域缺陷；脚本 needle/断言面已由执行侧修复在案，补跑可选）。
- cherry-pick 3035c8c5→**d8b7e2d3** 合 main 已推送。
- 烟测脚本 `scripts/settle_deliver_smoke.ps1`（364 行）与台账入库；后续判定链回归可复用。
- 症状 B 的 Mac 侧同族卡 D1-SETTLE-TAIL-MAC-1 不受影响（本卡修复=回放面所有权门；Mac 卡查调度切片饥饿——两根因正交，Mac 卡继续）。
