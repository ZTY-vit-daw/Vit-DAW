# SHELL-2 执行进度（断点续跑协议）

- 卡：coord/cards/doing/2026-11-15-SHELL-2.md
- owner：GLM-5.3-Flash · PC · ZCode（freeflash 量产执行流，主管会话派发）
- worktree：主仓 `D:/Vit_DAW_wt_shell2`（port/shell-2 @ 8c387721 = origin/main）；前端 `D:/Godot/project/vit-daw-frontend_wt_shell2`（port/shell-2 @ b578525 = 前端 main）
- 断点恢复指引：先读卡面，再读本文件最后一条「下一步」；按文件存在性判断产物，不重做已完成段。

## 勘察结论（M0 前置，已完成）

1. **既有链摸清**：
   - 348 桥语义：React `ask_vit_launcher`（launcher_directive/launcher_choose_file/launcher_surface_ready/launcher_result_ack）→ 宿主 → `vit_launcher_result`（opened/failed/canceled/unsupported/invalid/dialog_shown）。React 消费面（LauncherSurface/launcherBridge/launcherData）现成，烟测接缝 `launcher_smoke=1` URL 参数现成。
   - Godot 旧宿主链（start_page.gd）：`open_project_by_path`（公共）→ `_send_open_project_ipc`（内核 UDP 4445 IPC open_project）→ `_try_enter_main`（notify agent `version.project_opened` @ /agent/invoke + `_enter_main_scene` 切 VitDockRoot）。治理=`_launcher_governance_decision`：/agent/state current_project 空→零摩擦直行；非空/未知→三键 Godot 弹窗（save 走 save_project/save_as_project IPC）。
   - 343 窗机：`VIT_AGENT_WINDOW_MODE=autoload` 才启用；缺省休眠（mode.off）——VitShell 直启不设该 env 即保持休眠。
   - Godot autostart：kernel/hub/agent 拉起条件=非 `VIT_FROM_LAUNCHER=1` 且非 editor-without-dev-autostart；**VitIpcClient._exit_tree 会 OS.kill 已注册 kernel/hub/agent 子进程**——Godot 退出=kernel/hub 随之退出（355 全树语义在 Godot 侧的现有实现）。
   - agent 侧：/agent/invoke 有 save_project/open_project 工具（catalog.go 1064/1067/1070），authority 门对无 authority_mode 断言的请求放行（turn_control.go validateInvokeAuthority）。
2. **SHELL-2 打开链设计定案**（依据设计定稿时序「治理确认→agent/内核装载→VitShell 拉 Godot 直启」+ 卡面任务 4「Godot 直启 boot 装载指定工程（复用 open_project_by_path 公共链）」）：
   - React（VitShell WebView2）→ VitShell 收 ask_vit_launcher → 治理查 /agent/state →（有在开工程）发 `vit_governance_required` → React 页内三键卡（对话流线程项）→ `launcher_governance_choice` 回投 → VitShell 执行：cancel→result canceled；save→POST /agent/invoke {tool:"save_project",confirmed:true}（既有工具零新协议；require_path→IFileSaveDialog→save_as_project invoke）；switch/直行→关旧 DAW（WM_CLOSE→等退→kill 兜底；Godot 退出自带 kernel/hub 清理=355「退出重进」降级维持）→ 拉起 Godot 直启 → 等 Godot launch-result 回执 → vit_launcher_result{opened|failed}。
   - **Godot 直启契约（前后端自定，env 面）**：`VIT_SHELL_DIRECT=1` + `VIT_SHELL_LAUNCH_ACTION=open|new` + `VIT_SHELL_OPEN_PROJECT=<path>`（open）/`VIT_SHELL_PROJECT_NAME=<name>`（new）。start_page.gd `_ready` 检测 VIT_SHELL_DIRECT=1 → 跳过启动页 UI（根节点 visible=false，无 start_page 渲染）→ 走既有 bootstrap（kernel/hub 若缺自启、agent ping 到 VitShell 已启的 7878 即跳过）→ 公共链 open_project_by_path/_create_new_project → 切 dock。
   - **launch-result 回执契约**：Godot 直启成功/失败 POST `http://127.0.0.1:7879/vitshell/launch-result`（VitShell status 端点新增 POST 面）→ VitShell 据此回 React 终态 + 发 `vit_surface project`（进工程）。
   - **关 DAW 回主窗**：VitShell 以进程句柄 wait 感知 Godot 退出 → 发 `vit_surface launcher`（entrySurfaceBridge 既有面，React 同窗 setState，navigate 不增）。
   - 343 休眠：直启进程 env 不设 VIT_AGENT_WINDOW_MODE。
3. **烟测驱动设计**：Playwright CDP 连 VitShell WebView2（`WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS=--remote-debugging-port=<port>`）断言真实 DOM；原生 IFileOpenDialog 人工域=烟测 env `VIT_SHELL_SMOKE_FILE_DIALOG=<path>` 旁路（生产零行为，真实对话框留手测 v6/SHELL-3）；治理选择 smoke env/URL 参数自动回投（镜像 Godot 侧 VIT_SMOKE_LAUNCHER_GOVERNANCE 模式）。
4. **栈占用**：2026-11-15 01:59 净空亲核（7878/7879/8787/4445 监听=0，Vit 系进程=0）后按 §2.2 登记占用推 main。注意：SHELL-1 占用行（记录 L4）未见显式释放行（PROGRESS/coord commit 声明释放），本卡以物理亲核净空接管，已如实记录。
5. 停止条件核对：治理链用既有 agent 工具（save_project invoke）+ 新宿主↔React 消息类型（非 agent 协议字段）→ 不触发「agent 新协议字段」停止条件；直启不依赖 start_page UI 初始化（bootstrap 函数复用）→ boot 冲突风险低，实证于 M3。

## 里程碑

- [x] M0 领取：双 worktree + 卡 todo→doing + 本 PROGRESS 建立
- [ ] M0b 环境准备：webui npm install + Playwright 钉 1.65.0+SKIP_DOWNLOAD
- [ ] M1 抽屉手动按钮（React「＋新建」「打开…」）+ VitShell IFileOpenDialog + choose_file→打开链续链
- [ ] M2 页内治理三键确认卡（React）+ VitShell 治理流程（/agent/state + save invoke）
- [ ] M3 Godot 直启参数（start_page.gd 直启分支）
- [ ] M4 打开链闭环 + 关 DAW 回主窗（spawn/exit watch + vit_surface + launch-result 端点）
- [ ] M4b 单测：webui vitest（三门 563 不回退+新测）+ VitShell CMake 编译 exit 0
- [ ] M5 真栈全旅程烟测 exit 0（RUNBOOK 先行 + §10 recents 快照还原）
- [ ] M6 回执：卡面回填 → done → coord-only 推送；双仓 port 留证

## 状态：进行中

下一步：M0b webui 环境准备 + M1 实现。
