# HYGIENE-GOALPERSIST-WARN-1：goal 边界持久化失败补专用 WARN（观测面小缺口，BOUNDARY-PERSIST 观察项 A）

- 发卡：GLM 主管决策侧 / 2026-10-10 晚窗（依据=[EVENING-IDLE-FINAL rulings §1](../../rulings/2026-10-10-EVENING-IDLE-FINAL-rulings.md) 观察项 A 处置）
- 派发确认：已确认（主管裁定立卡）
- 验收负责人：GLM 主管决策流
- 池序 48；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P3 / 0.5h / 无
- 模型分级：L0 / flash 可接（一行日志加法）

## 缺口（BOUNDARY-PERSIST-1 取证实证）

`recordGoalResult` 的 persist 失败降级分支（`agent/internal/chat/goalrunner_chat.go:2037-2042`）只在 timing 行留 `stop=durable_checkpoint_persist_failed`，**底层 error 全文（含租约 owner/expires_at）既不入日志也不入 summary 工件**——事后取证只能靠同窗旁证重建。对照两处既有 WARN 先例：`capability_routing.go:679`（deferred WARN）与 `server.go:7192-7194`（写盘失败 WARN）。

## 目标

降级分支加一行 WARN：`[goal.persist] failed conversation=%s goal=%s error=%v`（error=%v 携 `res.Error()` 全文）。**纯日志加法，零行为语义变化**（降级/终态/Continuation 清空逻辑零改动）。

## 文件域

`agent/internal/chat/goalrunner_chat.go` 单文件（+如需日志断言则测试文件）；越域即停。

## 验收标准

`go build ./...` + `go test ./internal/chat -count=1` 全绿 + 全量 `go test ./... -count=1` 0 FAIL + 触碰文件 blob 级 gofmt 净；回执附 WARN 行样例（含错误全文形态）。

## 停止条件

降级分支在领取时已非此形态（锚点漂移）→ 上交。

## 并行与资源

纯单测域，不占真栈；与在池卡文件域不相交（chat 包 goalrunner 面，M4B/PROBE-METER/前端卡均不碰）。

- 领取：2026-10-10 18:14 / origin/main=3c275461e49cbfd39303225269cfdc818c5438e1 / owner=GLM-5.3-Flash（ZCode flash 执行流会话，PC 端）/ 分支=port/hygiene-goalpersist-warn-1 / worktree=D:/Vit_DAW_wt_goalpersist_warn_1 / 领取提交=（本提交）
- 领取时锚点核对：`goalrunner_chat.go:2037-2042` 降级分支形态与卡面一致，未漂移；两处 WARN 先例（capability_routing.go:679、server.go:7192-7194）核验在案，`s.logger` nil 守卫模式与 `goal=%s=res.GoalID` 取值沿用同函数 2198 行先例。
- 回执：实现 commit=9ceb62ea（分支 port/hygiene-goalpersist-warn-1，已推 origin；单文件 agent/internal/chat/goalrunner_chat.go +7 行=4 行注释+3 行 nil 守卫 WARN，降级/终态/Continuation 清空逻辑零改动，diff 全文见分支）。WARN 行样例（真实 logx.Warn 渲染，scratch 程序用后即删未入库）：
  - `2026-10-10T18:21:36 [WARN] [goal.persist] failed conversation=conv_ab12cd34 goal=goal_mix_free_state_1 error=persist project workspace: write D:\projects\demo_set\.vit\agent_runtime_state.json: Access is denied.`
  - `2026-10-10T18:21:36 [WARN] [goal.persist] failed conversation=conv_ab12cd34 goal=goal_mix_free_state_1 error=durable continuation lease held by owner=agent@PC-MAIN pid=4128 expires_at=2026-10-10T18:31:07Z: checkpoint store busy`（含租约 owner/expires_at 的错误全文形态，即 BOUNDARY-PERSIST-1 取证时只能旁证重建、现可直接入日志的那类文本）
  - 验收记录：`go build ./...` exit 0；`go test ./internal/chat -count=1` ok 94.7s；全量 `go test ./... -count=1` 92 包 ok / 0 FAIL / EXIT=0；触碰 blob（LF 净内容）gofmt -l/-d 均空。备注：工作树 core.autocrlf=true 使 gofmt -l 对包内全部文件报 CRLF 假阳性，按卡面「blob 级」口径以去 CR 后内容核对，改动前后 blob 均 gofmt 净。
  - 端测边界声明：纯日志加法零行为语义变化，验收=编译+单测+全量+gofmt（卡面"纯单测域，不占真栈"），未做端侧真栈烟测，与卡面资源条款一致。
- 验收：**pass（2026-10-10 晚窗主管）**——[rulings/2026-10-10-EVENING-BATCH-rulings.md](../../rulings/2026-10-10-EVENING-BATCH-rulings.md) §2；cherry-pick 9ceb62ea→main 6d31fb4f；diff 亲读（nil 守卫+error 全文）。观察项 A 销项。
