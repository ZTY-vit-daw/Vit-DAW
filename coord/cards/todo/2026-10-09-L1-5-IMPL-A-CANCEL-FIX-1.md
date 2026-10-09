# L1-5-IMPL-A-CANCEL-FIX-1：PullLoop 取消边界修复（待主管采信）

- 发卡：Codex 辅助决策流 / harness 共享池审查会话 / 2026-10-09（Asia/Shanghai）。
- 派发确认：待主管确认；用户授权检查复核交付，本文将已证缺陷具体化为修复建议，尚不授权领取或实现。
- 验收负责人：现有主管决策流；A 原卡最终裁定、修复合入和 B 解锁仍归主管。
- 目标仓库：D:\Vit_DAW（或 Mac 同仓独立 worktree）。
- 优先级 / 预估 / 依赖：建议 P1 / 30-60 分钟 / 主管采信 A-REVIEW-1 取消反例并确认修订方案。建议修复先于 A 最终验收；不更改现有池序。
- 文件域：agent/internal/pullharness/loop.go、loop_test.go（或同包新增取消测试），以及 coord/runs/L1-5-IMPL-A-CANCEL-FIX-1/ 回执/日志。不改 budget.go/mode.go、冻结三包、原测试期望或 B/C/D 设计；新增总计不超过五个实现/测试/回执文件。
- 并行与资源：与 B 同包/loop.go 重叠，禁止与 B 并行编辑。C 只读复核域独立；零真实栈，本卡不启动运行栈。

## 已证事实与待定边界

- 被审 A=de1d876c，零生产消费；三取消反例经执行复核与发卡侧独立复跑两份证据坐实。
- 依据：coord/runs/L1-5-IMPL-A-REVIEW-1/REPORT.md @7b567089；辅助核查 coord/reports/2026-10-09-L1-5-AC-review-audit.md；独立反例 coord/runs/L1-5-REVIEW-AUDIT-20261009/a-cancellation.txt。
- 已取消入场仍 Route；Complete 取消后 error=nil 仍执行工具；Execute 取消后落 budget_exhausted 并 T2。原卡要求中断保持 interrupted、不触发终局 T2。
- 待主管定：工具批已部分或全部完成时如何保留结果/成本/已执行事实，以及 T1 是否针对已完成批触发。本卡不得用“ctx 已取消”丢弃实际完成的工具结果，也不发明恢复幂等或持久化新协议。

## 实施与验收

1. 领取前复查授权、A 修订版本、B 是否在飞。用主管指定基线建立独立 worktree；若 A 未合入，从当前 main 仅应用原 A 提交复验，修复为单独追加 commit，禁止把原 A 未验收实现推 main。
2. 在 Router 前、模型返回后/工具执行前、工具批返回后的生命周期边界补足取消分流；中断优先于预算/终局分类。明确批完成事实如何保留。不得以 finish 无条件改判造成真实结果被覆盖；代码契约有争议即上交。
3. 把已证三种反例转为正式回归，增加 Router 执行过程中取消的结果保留测试；均用 fake/t.TempDir，不碰真实工程。正常 Router/无工具终局/模型错误/工具批 T1/预算 T2 原测试继续通过。
4. 在 agent/ 执行 go build ./...、go test ./internal/pullharness -count=1 -v、go test ./... -count=1，直接退出码均0；提交 blob gofmt 净，附被测 HEAD/status、修复 diff、反例修前红/修后绿、正常路径回归和结果保留口径。
5. 回执明确本卡只修骨架取消路径，未证明生产中断恢复、A/C 适配或 G3 真栈。零生产入口消费须重新核对；若已有生产接线，本卡既定单元级边界失效，上交主管安排真实栈门槛。

- 停止条件：解决缺陷必须改 ToolExecutor/FastPathRouter/GoalInput/Result 冻结边界或 continuation 持久化；主管未定义批完成与中断竞争语义；A 版本变化；文件域与 B 冲突。附最小反例与两种方案，上交，不自行扩域。
- 领取：（授权依据 / 时间 / main 基线 / owner 模型+机器+会话 / 分支 / worktree / 同步提交）
- 回执：（修复 commit / 命令退出码 / 红绿证据 / 取消与批结果保留契约 / 端测边界）
- 验收：（主管裁定 / 修复合入 / A 原卡与 B 依赖状态另记）
