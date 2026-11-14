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

- [ ] M0 领取：双 worktree + 卡 todo→doing + 占用登记 + coord-only 推 main
- [ ] M1 装置扩展：scripts/vitshell_full_smoke.ps1 + scripts/vitshell_full_smoke_cdp.mjs（新腿）+ chat 帧实证（独立 agent 单腿）
- [ ] M2 RUNBOOK 冻结（3 轮口径+失败分类+止损线）+ 主腿 3 轮真栈运行（记录成功比例）
- [ ] M3 失败注入腿运行（坏二进制/端口占位/WebView2 禁用）
- [ ] M4 legacy/编辑器路径回归声明核证
- [ ] M5 手测 v6 放行包 coord/runs/SHELL-3/HANDTEST-V6.md
- [ ] M6 回执：卡面回填→todo/doing→done→coord-only 推送；port/shell-3 双仓留证

## 下一步

M0 进行中：worktree 已建，卡移动+占用登记+coord 推送待做。
