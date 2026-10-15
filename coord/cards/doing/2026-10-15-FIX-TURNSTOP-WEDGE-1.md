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

- **实现 commits**（port/fix-turnstop-wedge-1，已推远端）：
  - `846feb63` fix(FIX-TURNSTOP-WEDGE-1)：runtime.MarkStopped 补 syncTaskStatus（与 Complete/SetStatus 簿记纪律对齐）+ handleTurnStop active 分支限时观察窗（默认 2s/50ms 轮询，`turnStopSettleWindowOverride` 注入）超时内联 finalizeStoppedTurn 幂等兜底 + 单测四景 + 烟测 probe（py `--immediate-stop-probe` / ps1 `-ImmediateStopProbe`）
  - `74e6dd18` runs(FIX-TURNSTOP-WEDGE-1)：真栈烟测 run1 工件入库（§8 声明+report+console+agent log）
  - 领取提交 `686e0bc5`（rebase 后含于 846feb63 祖先链）
- **单测**（红绿已验：基线 e115fd8a 三景行为级 FAIL——字段无关版，修复版全 PASS）：
  - `go test ./internal/chat -run 'TestTurnStop' -count=1` → ok（四景：停摆窗口兜底收敛 / 二次 stop 幂等（EventTurnStopped 去重+checkpoint 保持）/ runner 窗口内落账不误伤（真实 settle 保护）/ 窗口解析）
  - 109 停泊回归：`TestParkedReplay|TestAudioClosure` → ok（audioclosure 零改动）；核心四包（chat/runtime/agentloop/harness）→ ok
  - 全量 `go test ./... -count=1` → **exit 0（36 包全 ok，0 FAIL）**；gofmt 净（LF 规范化后 4 文件 CLEAN；工作树 CRLF 为 checkout 行尾，入库 LF）
- **真栈烟测**：`run_free_state_d1_smoke.ps1 -ImmediateStopProbe -SkipBuild -KernelExe <主检出>` → **exit 0 首跑 PASS**（run ID 20261015_183315；被测 agent SHA256 6119876A26FB2700 + 内核主检出 B6565DCF85D1DA86 C++ 零改动）。probe 实录：stop 前 prior=running/active（active 窗口命中）；收敛三元组 task=settled、running_slices=[]、新收口 checkpoint `c_20261015T103449_d68a0d9d`（非 ask 回退）；二次 stop 幂等；同会话新 goal `goal_b9c10845e9eabab3` + 完整响应（degraded=False）；closure 诚实结算 owner_turn_closed（日志锚点 `[turn.stop] request received`×2 在卷）。占用 19f7fb08 / 释放见 PC-RUNTIME-STACK.md（18:37 拆栈四端口+进程零复核）。
- **端测边界声明**：①真栈 stop 到达时 goal 已 waiting_continue（LLM 5s 快速完成）→ 真栈验证的主路径=finalizeStoppedTurn 内联分支（=M2 症状 2「二次停止救不回」同款分支，修复一在其中生效）；active 分支两条路径（runner 窗口内落账不误伤 / 停摆 2s 兜底）由单测景三/景一钉死，`fallback_anchor_seen=False`（兜底路径未被真栈触发——117 修复后无稳定停摆窗可构造）。②新 ask 断言含执行前声明的降级口径（240s 超时→持久面新 goal 绑定），本次未触发降级（完整响应）。③webui 渲染面未测（本卡无渲染改动）。
- **并发登记竞态备忘**：本卡占用登记（19f7fb08）与 FE-CEF-UPGRADE-1(116) 七项回归登记（8dcf1a77）几乎同时落库；物理面无冲突（详见 PC-RUNTIME-STACK.md 竞态备忘条），116 后续启栈前已提示重核 7878。
- **worktree/分支清单**：worktree `D:/Vit_DAW_wt_tsw1`；分支 `port/fix-turnstop-wedge-1`（@74e6dd18，已推远端）+ 协调分支 `coord/tsw1-stack`（本收尾提交）；run 重件留盘 `artifacts/free_state_d1_s1/20261015_183315/`。
- 报告链接：[coord/runs/FIX-TURNSTOP-WEDGE-1/README.md](../../runs/FIX-TURNSTOP-WEDGE-1/README.md)
