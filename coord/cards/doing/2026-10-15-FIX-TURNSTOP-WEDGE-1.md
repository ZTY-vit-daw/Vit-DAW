# FIX-TURNSTOP-WEDGE-1：user_stop 半落地——slice/task 永不收敛且二次停止救不回

- 发卡：GLM 主管决策侧 / 2026-10-15 早窗（依据=M2-EXPERIENCE-1 首跑取证，[README](../../runs/M2-EXPERIENCE-1/README.md)）
- 派发确认：**已确认（用户 2026-10-15 晚窗「进入晚窗，可以发卡」）**；验收负责人：GLM 主管决策流
- 池序 116；目标仓库=D:\Vit_DAW；P1 / ≤1 天
- 模型分级：L2 / GLM 执行会话（并发状态机收敛语义）
- **并行注记（2026-10-15 晚窗）**：与 117 并行派发——117 本轮为取证腿（零生产代码合入），与本卡文件域不相交；真栈腿（烟测复现）与 117 取证 run 按栈占用登记串行。

## 缺陷实锤（全部亲证）

1. 用户在 slice 刚启动（active 态）时按停止：`handleTurnStop`（internal/chat/turn_control.go:428）因 `wasActivelyExecuting=true` 走「runner checkpoint 落账」分支（不内联 finalize）——但 runner 无在飞轮询点，**stop 永不落账**：goal/plan=stopped、stop_requested=true，而 task=active、slice=running、goal.updated_at 冻结（10 分钟零变化）。
2. 决策侧二次 POST /agent/turn/stop（此时 goal.status=stopped → 走 `finalizeStoppedTurn` 内联分支）：HTTP 200 ok，但 slice/task **仍 running**，checkpoint_ref 回退为 ask checkpoint（未产出新收口 checkpoint）——设计内恢复路径无效。
3. 用户观感=「无法终止」：UI 终止按钮无效果，轨迹面无在飞可见物。

## 修复目标

- active 态停止在 runner 停摆（无 LLM 在飞、无 checkpoint 将至）时可收敛：停止请求必须有超时兜底或轮询点可观察；
- `finalizeStoppedTurn` 必须把 slice/task 一并终态化（当前只动 goal 层）；
- 二次停止不产生重复副作用（幂等）。

## 文件域

agent/internal/chat/turn_control.go + goalrunner/harness 停止落账路径 + 单测；与 117 卡（ask 前置段）域不相交可并行。

## 验收标准

- 单测：active 态 stop 后 runner 停摆场景 → 限时内 slice/task 收敛 stopped；二次 stop 幂等；真实 settle 不被误伤（109 停泊语义回归：user_judgment_pending 停泊存活不受影响）。
- 真栈：复现场景（ask 后立即 stop）→ 停止后 slice/task 终态、UI 可再发起会话，exit 0（扩展现有 ps1 体系）。
- `go test ./... -count=1` 0 FAIL + gofmt 净。

## 停止条件

停止落账与 109 停泊回放守卫（5ad6b797）语义冲突 → 锚点上交决策侧。

## 领取

- 时间：2026-10-15 18:08 (+08)
- origin/main：e115fd8a458da21e04abf6cd451bf3a775634dc7
- owner：GLM-5.3 L2 执行会话 / PC (Windows) / ZCode 会话（用户 2026-10-15 晚窗派发指令指定）
- 分支：port/fix-turnstop-wedge-1
- worktree：D:/Vit_DAW_wt_tsw1
- 领取提交：本提交（卡面由 todo 迁入 doing——决策侧写卡后尚未推送远端，领取提交基于 origin/main e115fd8a 将本卡连同领取回填首次同步至共享 main；todo/ 中原 untracked 草稿由决策侧工作树自行处置）

## 回执

（待完成后回填）
