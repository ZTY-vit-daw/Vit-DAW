# M3-CHAT-AUTH-STAMP-1（池序 223）真栈腿回执

- run：`coord/runs/M3-CHAT-AUTH-STAMP-1/chat_auth_stamp_20261031_1`（round 1，红）+ `chat_auth_stamp_20261031_1_r2`（round 2，重启腿红=出域缺陷定案）
- 被测：worktree `D:/Vit_DAW_wt_223impl` @ `fefbc4fd`（port/m3-chat-auth-stamp-1，agent 面=turn_control.go bindChatAuthorityMode 重写+测试；ps1 在 worktree 根自建 agent）
- kernel：主检出 `D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe` 只读复用，sha256 `c1e32dfe…` 运行前亲核=217/218/220/221 同源零漂移（本卡 C++ 零改动）；无 UI；vsphub 不需要（agent 直连内核 ZMQ）
- 隔离：VIT_PROJECT_XML=ps1 共享工程隔离段 git 干净 run 副本（-ChatAuthStampSmoke 已入条件列表）；draft root=per-run 临时目录
- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 -StartKernel -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -ChatAuthStampSmoke -KernelExe <主检出 kernel> -RunArtifactsDir <run 目录>`
- 退出码：round 1 = 1（after 相第一腿红）；round 2 = 1（同腿红，真因定案）

## 一、绿的腿（两轮一致）

| 腿 | 断言 | 结果 |
|---|---|---|
| setup | 隔离工程+3 轨+轨级 volume 插件+基线曲线空 | PASS（tracks 1007/1010/1013） |
| switch_full | POST /agent/authority full → 200 + authority_mode=full + **persisted=true + workspace_ready=true** | PASS |
| full_direct | **裸话术**（body/ctx 无任何 authority 键）「0 到 30 秒，让这条轨的存在感慢慢强起来」→ 无确认卡（interaction_requests 空+needs_confirmation=false）+ full_access_auto_authorized/confirmation_bypassed=full_project_access/interaction_card_issued=false + automation_choreo_settled + written=2 + mix_decision recorded + C4 in_progress + 回执含「完全访问权限下编舞写入已自主执行」+ 内核 get_curve 逐点 (0,0.30)/(30,0.60) | PASS |

→ 卡面标的（chat 链 stamp 从 server 状态直达编舞桥、完全权限直执无确认卡）**真栈成立**；manual/伪造负例腿在 r2 因 after 相早退未执行（单测面已钉：chat_authority_stamp_test.go 五探针全绿）。

## 二、红的腿与定案

**leg restart_restore：agent 真重启后 GET /agent/authority = manual_confirmation（want full_project_access）**，两轮同形。

- round 1 红因=断言体腿序缺陷（已修）：switch 先于 agent 绑定新工程身份（shadow 滞后 ~5s）→ full 持久化到旧身份 workspace；随后的身份切换按设计掉闩采纳新工程 manual。**该窗口忠实复现 219 面 5 的身份切换丢失窗（产品正确行为：per-workspace 权限+防泄漏）**。修复=断言体两相加 wait_shadow_tracks 身份绑定等待（fefbc4fd 之后的脚本修正 commit）。
- round 2 定案（**出域缺陷，本卡不修**）：switch 已落对工程（session_20261030T124721 state=full_project_access 在卷），但重启进程 `RecoverWorkingSession` 因 `runtimeStateHasPendingWork=false`（无活 continuation/非终态 goal）不恢复前一 session → `EnsureWorkingSession` 新建 session 并从 **canonical HistoryDir/存档代播种**（copyDir canonical→workspace）——前一 session workspace 的 agent 运行时状态（含 authority_mode）**从不被带入**。日志锚点：r2 agent_last.log `restore adopted disk mode=manual_confirmation` + `[workspace] activated … recovered=false`（同工程同 UUID）。
- 代码锚点：`agent/internal/history/working_session.go:94-101`（恢复需 pending work）+ `:165-215`（新 session 播种源=canonical/存档代，非前一 session workspace）+ `agent/internal/chat/server.go:7517-7548`（激活链）；持久化面 `persistCurrentProjectWorkspaceChecked`→`WriteAgentRuntimeState` 落 session-bound StateDir（`history.go:1176-1200`）。
- 与用户事故的关系：=219 面 5 记录的 2026-10-29 事故（"webui 切完全权限→下一次 agent 启动不恢复"）同一链第三丢失窗的真身；面 5 只静默关了窗 (i)（persisted 披露），窗 (ii)/(iii) 当时就只诊断未修。本腿把 (ii) 的机制钉到代码行。
- 附带事实：泄漏的 full 只落 per-run 临时 draft root 与 run 目录内 session 状态（随跑隔离，无共享面污染，亲证）。
- 未测开放问题（供修复卡）：切换后再 save 工程（存档代播种源）是否携带 authority 跨重启——本轮未覆盖。

## 三、判定与上交

- 卡面验收线（真栈腿 exit 0）未达：restart 子腿红，根因在 workspace/history 层=本卡文件域（turn_control.go+chat 入口 stamp 点+测试+scripts）之外。按 AGENTS §11 越域即停。
- 上交决策侧处置三选：①扩域修 `RecoverWorkingSession`/播种链（runtime state 跨干净重启携带）另开卡（建议：与 219 面 5 窗口族合并为一张修复卡）；②本卡按「stamp 交付+端测边界声明」验收（stamp 面单测+真栈 phase-1 全绿；重启腿移交新卡）；③本卡返工并入修复。
- 工件：两轮 summary/console/agent 日志在卷（运行时工程文件按 218 先例剔除）；port/m3-chat-auth-stamp-1 分支含实现+脚本+工件三 commit。
