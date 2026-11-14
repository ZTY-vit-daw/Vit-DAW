# SHELL-3 执行进度（断点续跑协议）

- 卡：coord/cards/doing/2026-11-15-SHELL-3.md（自 todo/2026-11-15-SHELL-3.md 移入）
- owner：GLM-5.3-Flash · PC · ZCode（freeflash 量产执行流，主管会话派发）
- worktree：主仓 `D:/Vit_DAW_wt_shell3`（port/shell-3 @ 25f68e95 = origin/main）；前端 `D:/Godot/project/vit-daw-frontend_wt_shell3`（port/shell-3 @ 14d1f27 = 前端 main，SHELL-1+2 已合入）
- 文件域：`scripts/`（新增烟测脚本，零生产改动）+ `coord/runs/SHELL-3/`；实现提交留 port/shell-3 候主管合入，coord-only 提交推 main
- 断点恢复指引：先读卡面，再读本文件最后一条「下一步」；按文件存在性判断产物，不重做已完成段。

## 勘察结论（M0 前置）

1. **SHELL-2 装置已摸清**（`scripts/vitshell_open_chain_smoke.ps1` + `scripts/vitshell_open_chain_cdp.mjs` + RUNBOOK）：L2-L7 全腿（抽屉 DOM→CDP 真点→choose_file 旁路→首开→直启→关 DAW 同窗回 launcher→治理三键→关主窗全退）；§2.2 占用+≤60s 复核；§10 recents+双根 Workspace 运行态快照还原+fixture 哈希对账。本卡在此装置模式上补腿，不另起一套。
2. **本卡新增腿设计**：
   - 冷启动零 Godot：VitShell 启动后、首开前进程级断言 `Godot*=0` + status `godot_running=false`（splash 缺席的进程级证据）。
   - 主窗真数据：launcher greeting（GET /agent/launcher/greeting 真端点，recents 兜底文案）→ CDP 断言 `data-launcher-state=ready`+`data-launcher-bridge=available`+无 `data-fixture`+greeting 线程帧在场。
   - 工程内对话往返：project surface（346）CDP 真实键入 composer（`.v5-project-composer input`）+发送→断言 user 帧（`data-message-author="user"`）+assistant 帧（`data-message-author="vit"`）出现在 `[data-project-thread]`。LLM 计费面：对 VitShell 进程注入 `VIT_AGENT_LLM_BASE_URL=http://127.0.0.1:1`（agent config.go applyExplicitEnvOverrides 显式 env 覆盖 config 文件，~/.vit/config.json 有真 key 不被动用）→ LLM 不可用时 agent 走确定性 fail-safe（225 裁定：219 三面固定问句保留为 LLM 不可用降级路径）→ 回执仍是真 agent 帧。回包形态（reply vs error）M1 用独立 agent 实证后冻结断言。
   - 失败注入腿：①坏二进制（--agent-exe 指向不存在路径→agent.Start 失败→phase=error 窗内错误页）②端口占位（7878 预占 TcpListener→agent 起不来→`--agent-ready-timeout-ms 15000` 短超时→phase=error）③WebView2 禁用（WEBVIEW2_BROWSER_EXECUTABLE_FOLDER 指向空目录→ProbeWebView2Runtime 失败→系统错误框 MessageBoxW+exit；装置可注=真注，实证不支持则声明留手测）。零 Godot 回退断言三腿共用。
   - 稳定性口径（AGENTS §8）：主腿 3 轮，记录成功比例；RUNBOOK 跑前冻结。
   - legacy/编辑器路径回归（声明性）：Godot `--import --check-only` exit 0 + start_page.gd 直启分支 env 门（VIT_SHELL_DIRECT）静态核证+F5 路径未改声明。
3. **VitShell 契约面**（main.cpp/status_server.cpp @14d1f27）：status `phase=starting|agent_ready|ready|error` + `error` 字段 + `godot_running/launch_count/chain_state`；`--agent-ready-timeout-ms` 参数在卷；POST /close=WM_CLOSE（error 相位窗口已建，可用）；POST /vitshell/close-daw+/vitshell/launch-result。
4. **栈占用**：见 coord/resources/PC-RUNTIME-STACK.md 顶部本卡占用行（登记提交后 ≤60s 即时复核，S2.2③）。

## 里程碑

- [x] M0 领取：双 worktree + 卡 todo→doing + 占用登记 + coord-only 推 main（领取 ca097c91 + 占用 5de1ea2c 已推 main 亲证）
- [x] M1 装置扩展：scripts/vitshell_full_smoke.ps1 + scripts/vitshell_full_smoke_cdp.mjs（新腿：probe-launcher-data/probe-project-ready/chat-roundtrip/probe-error-page；旧腿镜像 SHELL-2 语义）+ chat 帧实证 + 五例装置缺陷调试修复（RUNBOOK 调试轮节；取证 run 全保真）
- [x] M2 准备：RUNBOOK 冻结（3 轮口径+失败分类+止损线+零计费 chat 断言口径）
- [x] **M2/M3/M4 正式跑 PASS**：run_full_20261115_043654 **exit 0，主腿 3/3 轮 PASS**（成功比例 3/3）+失败注入三腿全 pass（F1 坏二进制/F2 端口占位/F3 WebView2 禁用=真注系统错误框路径）+legacy 腿 pass（--import --check-only exit 0+start_page env 门结构核证）；每轮含冷启动零 Godot 进程级断言+主窗真数据+抽屉/对话打开工程+治理（首开零摩擦+二次开三键卡逐字断言）+DAW 直启+**工程内 chat 往返真 agent 帧**+关 DAW 同窗回主窗+关主窗全退+S10 还原；§2.2 全程+§10 快照还原零污染（fixture 源哈希前后一致）
- [x] M5 手测 v6 放行包 coord/runs/SHELL-3/HANDTEST-V6.md（主检出 VitShell.exe @14d1f27 现场构建 sha256 bb7066e4…+启动包装 cmd；arg-free 装配缺口如实声明为已知边界）
- [x] M6 回执：卡面回填→doing→done→coord-only 推送；port/shell-3 主仓推 origin 留证（前端仓零改动无 port 提交）

## 三门声明（不回退）

- agent `go build` exit 0（正式跑装置内，run_full_20261115_043654/agent_build.log）
- webui `npm run build`（tsc --noEmit+vite build）exit 0（正式跑装置内）+ **vitest 570/570 全绿**（47 文件，=SHELL-2 基线，04:42 复跑）
- VitShell cmake x64 Release exit 0（正式跑装置内）
- 零生产改动：主仓 diff=scripts/ 新文件+coord 工件；前端仓零 diff（git status 干净）

## M1 实证记录

1. **构建全绿**：agent go build / webui npm install+build / VitShell cmake x64 Release / godot --import 全 exit 0（coord/runs/SHELL-3/build/）。
2. **chat 回包形态（实证+真栈验证）**：`VIT_AGENT_LLM_BASE_URL=http://127.0.0.1:1` 显式 env 覆盖（agent config.applyExplicitEnvOverrides，env 胜 config 文件）→ 零计费。真栈 chat 腿实测（3/3 轮一致）：agent 确定性无 LLM 路径回**真 agent 帧**（「当前工程还没有任何轨道…（意图分类服务本次不可用，以上直接来自工程结构事实。）」）——优于独立单腿探针的悲观预期（单腿无 kernel 时 reply/error 并存）。断言口径=真实 user 帧+真实 HTTP 往返 assistant 帧；LLM 内容生成显式出界。
3. **PS5.1 编码陷阱（装置纪律）**：.ps1 必须带 BOM；Invoke-WebRequest JSON body 必须转 UTF-8 byte。
4. **VitShell 契约面实证**（main.cpp/status_server.cpp @14d1f27）：启动序=WebView2 host.Create(579)→status.Start(679)→StartAgentOnce(702)→失败即 ShowErrorPage（phase=error，窗内错误页含重试/退出按钮）；`--agent-ready-timeout-ms` 在卷；POST /close=WM_CLOSE；WebView2 缺失路径=ProbeWebView2Runtime(504)→MessageBoxW("Vit")（host/status 之前，无 status/agent/Godot）；**arg 解析取首者胜**（重复 flag 隐患）；`VIT_WEBVIEW_DEBUG=1`→stderr 全 step trace（装置已默认开启）。
5. **装置五缺陷**（详见 RUNBOOK 调试轮节）：①status URL 缺 /status 后缀 ②F1 重复 --agent-exe 首者胜 ③F3 注入 env 未清除→后续轮静默死亡（根因链：Start-Process stderr 管道开启句柄继承→VitShell 子进程继承注入 env）④Test-PortBindable TcpListener ctor-bind 泄漏（Stop() 不释放未启动 listener，micro_bind.ps1 实证）⑤F2 occupier 须随 VitShell 退出后释放（子进程句柄继承 bound socket，repro_f2hold 实证）。

## 状态：DONE（执行侧自验完成，终验归用户手测 v6 + GLM 主管决策流）

- 正式 run：coord/runs/SHELL-3/run_full_20261115_043654（3/3 轮+三失败腿+legacy，全工件含 meta/bridge log/CDP 断言/status 快照/godot 日志/哈希对账）
- 调试 run 取证：035009/035933/040758/041804/042646/043247 + 验证 PASS 043539 + 诊断脚本（diag/repro/micro）全保真入库
- 断点续跑记录：本任务全程单会话执行，无重派中断；未发生同断点原样重跑（每次失败均有新根因+修复）
- 详见卡面回执（done/2026-11-15-SHELL-3.md）与 run 工件

