# FIX-TURNSTOP-WEDGE-1 真栈烟测工件

## §8 概率性运行声明（执行前写明，2026-10-15 18:33 +0800）

- **runs**: 1 次完整链尝试（`run_free_state_d1_smoke.ps1 -ImmediateStopProbe`）；仅分类为环境中断（端口/内核启动失败）才允许重跑，功能性失败禁止盲跑。
- **成功条件**: 脚本 exit 0（D1-S1 IMMEDIATE_STOP PASS）——ask 在飞时 POST /agent/turn/stop 后 goal/task/slice 三元组收敛（stopped/非 active/无 running slice）、二次 stop 幂等、同会话可再发新 turn（新 goal_id）。
- **失败分类**: `immediate_stop_not_stopped`（stop 响应非 stopped）/ `wedge_not_converged`（三元组 30s 内不收敛=M2 楔死残存）/ `second_stop_diverged`（幂等破坏）/ `conversation_wedged`（新 ask 无新 goal）/ `anchor_missing`（日志锚点缺失）/ `timeout` / `env_failure`（栈启动/装载失败）。
- **止损线**: 同签名两连败即停，保留工件上交决策侧；不弱化断言。
- **降级证据口径**: 新 ask 的 HTTP 响应在 240s 预算内未返回时（117 卡的 ask→slice 静默缺陷在飞），主断言切换为持久面 conversation_goals 新 goal 绑定（记录 `post_stop_degraded=true`）——这是执行前声明的口径，非事后放宽。

## 被测与基线

- agent: worktree `D:/Vit_DAW_wt_tsw1` 分支 `port/fix-turnstop-wedge-1@846feb63`，VitAgent.exe SHA256 `6119876A26FB2700123E5C42DA73C8B0B084B8775C6CD76DC4A91B6984233B19`（含本卡修复）
- 内核: 主检出 VitApp.exe SHA256 `B6565DCF85D1DA867E16333052A7B928A5413565940603ADBCF4ABD59DD3123B`（C++ 面零改动）
- 前端: Godot 主检出（-StartUI）
- 占用登记: 19f7fb08（2026-10-15 18:31）
- 单测腿（先行）: 新四景 PASS + 停泊回归绿 + `go test ./... -count=1` exit 0（36 包）+ gofmt 净（LF 规范化）；红绿验证=基线 e115fd8a 三景行为级 FAIL、修复版全 PASS

## run 记录

### run1（2026-10-15 18:33–18:37 +0800）—— **PASS exit 0**

- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File scripts/run_free_state_d1_smoke.ps1 -RepoRoot D:/Vit_DAW_wt_tsw1 -PublicManifest <主检出 spv1 fixture 80085263a651cf20> -ImmediateStopProbe -SkipBuild -KernelExe <主检出 VitApp.exe>`
- 退出码 **0**（D1-S1 IMMEDIATE_STOP PASS）；report=`d1_smoke_report_run1.json`（原位 artifacts/free_state_d1_s1/20261015_183315/，重件留盘 worktree）
- probe 结果：conversation `d1_s1_20261015T103434611872`；stopped goal `goal_a3ef7aab84fcb365`；stop 前读数 prior_status=running/prior_task_status=active（active 窗口命中）；**收敛三元组：task_status_after=settled、running_slices_after=[]、checkpoint_ref=c_20261015T103449_d68a0d9d（新收口 checkpoint，非 ask 回退）**；二次 stop goal_status=stopped（幂等）；新 turn goal_b9c10845e9eabab3 ≠ 停止 goal、完整响应（post_stop_degraded=False，reply_len=21）；首 ask 腿 error 空、goal_status=waiting_continue。
- 日志锚点（agent_last_run1.log）：`[turn.stop] request received`×2（18:34:49 first / 18:35:03 second）+ `[turn.stop] settled stopped-turn closure=audio_closure_544877568eba8a95 reason=owner_turn_closed`（closure 诚实结算）。
- **路径归因（如实）**：stop POST 到达时 goal 已 waiting_continue（LLM 5s 快速完成 turn）→ 走 **finalizeStoppedTurn 内联分支**——正是 M2 症状 2（二次停止 200 ok 纹丝不动）的同款分支，修复一（MarkStopped→syncTaskStatus）在其内生效收敛 task/slice。`fallback_anchor_seen=False`：settle-window 兜底未被真栈触发（无停摆窗口），active 分支两条路径由单测景一（停摆兜底）/景三（runner 落账不误伤）钉死——**端测边界声明见卡面回执**。
- 环境：VitAgent 9988 / VitApp 11508 / Godot 2104；18:37 手动 teardown，复核四端口 5555/5556/7878/8787 监听=0、VitApp/VitAgent/vsphub/Godot 进程=0。
- run0（console_run0_param_miss.log）：执行侧参数错误腿——协调分支上误用旧版脚本致 `-ImmediateStopProbe` 参数不识别，未启栈（ports=0）零副作用，非 §8 功能性失败，不占 run 计数。

