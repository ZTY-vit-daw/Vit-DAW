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
- 回执：（commit hash / WARN 行样例 / 端测边界声明=纯日志加法）
- 验收：（裁定文件 / 验收 commit）
