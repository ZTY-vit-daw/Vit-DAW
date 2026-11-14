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

- [x] M0 领取：双 worktree + 卡 todo→doing + 本 PROGRESS 建立（领取提交 28cc945c 已推 main，owner 亲证）
- [x] M0b 环境准备：webui npm install exit 0（Playwright 钉 1.65.0，SKIP_DOWNLOAD）
- [x] M1-M4 VitShell C++（前端仓 port/shell-2 @ fa6f754→ec71067→4deeb34）：launcher_bridge（choose_file/directive/治理/单飞状态机/线程模型：worker+UI dispatcher+三 rendezvous）+ file_dialog（IFileOpen/SaveDialog）+ godot_manager（直启 env 契约+exit watch+独立 kill-on-close Job+close-daw 进程子树 WM_CLOSE）+ status_server（surface/chain_state/godot_pid/launch_count + POST /vitshell/launch-result + POST /vitshell/close-daw）+ main 接线；CMake 编译 exit 0
- [x] M1-M2 React（主仓 port/shell-2 @ a817c9bc→439f63e7→治理门控修复）：抽屉头部「＋新建」+「打开…」（空 recents 态主路径突出）；页内治理三键卡；launcherBridge 治理消息面（订阅/回投/烟测自动应答）；vitest 570 全绿零回退；tsc+vite build exit 0
- [x] M3 Godot 直启（前端仓 port/shell-2 @ b2b7243→4deeb34）：start_page.gd 直启分支（无启动页渲染→复用 bootstrap+open_project_by_path/_create_new_project 公共链→切 VitDockRoot）+launch-result 幂等回执（Engine.get_main_loop 离树修复）+VIT_SHELL_STATUS_PORT 契约
- [x] M5 真栈全旅程烟测 **exit 0 PASS**（run_20261115_032326；RUNBOOK 先行；L2-L7 全腿绿：抽屉按钮 DOM→CDP 真实点击→choose_file 旁路→零摩擦首开→Godot 直启（kernel/hub 自启+dock 场景+无 start_page 渲染结构性证据）→/agent/state current_project=fixture_a→关 DAW→同窗回 launcher（哨兵完好）→治理三键卡呈现→真实点击直接切换→fixture B 直启→关主窗全退（进程=0 端口=0）→recents/运行态快照还原+fixture 源哈希一致）
- [x] M6 回执：卡面回填 → done → coord-only 推送；双仓 port 留证

## 契约终稿（烟测按此执行）

- **React ↔ VitShell**（channel ask_vit_launcher）：launcher_surface_ready / launcher_directive{request_id,directive} / launcher_choose_file{request_id} / launcher_result_ack / launcher_governance_choice{request_id,choice}；回执 vit_launcher_result{request_id,status,action,message,file_path,project_name} + vit_governance_required{request_id,action,current_project} + vit_surface{surface:project|launcher}（entrySurfaceBridge 既有面，surface_ack 双向留痕）。
- **VitShell ↔ Godot 直启**（env）：VIT_SHELL_DIRECT=1 + VIT_SHELL_LAUNCH_ACTION=open|new + VIT_SHELL_OPEN_PROJECT=<path> / VIT_SHELL_PROJECT_NAME=<name> + VIT_SHELL_STATUS_PORT=<port>；args `--path <前端工程>`；不设 VIT_AGENT_WINDOW_MODE（343 休眠实证：日志 mode.off）/VIT_FROM_LAUNCHER（kernel/hub 自启、agent 已答跳过）。
- **Godot → VitShell**：POST /vitshell/launch-result {"ok","scene","detail","action","project_path"}（幂等一次）；烟测关 DAW 面：POST /vitshell/close-daw（graceful WM_CLOSE 进程子树投递=用户 X 同路径）。
- **烟测旁路**（生产零行为）：VitShell `--smoke-file-dialog <A> --smoke-file-dialog-b <B>`（IFileOpenDialog 人工域旁路，首链 A/后续链 B）；WebView2 CDP `--remote-debugging-port`（Playwright 1.65.0 connectOverCDP：React DOM 断言+真实点击+no-reload 哨兵）。

## 真栈 run 记录（§8/§9 对账）

- **run_20261115_032326 PASS exit 0**（工件=coord/runs/SHELL-2/run_20261115_032326/，随主仓 port/shell-2 入库；console 全量=smoke_console.log）。
- 领取 HEAD：主仓 8c387721 / 前端 b578525；被测树状态=port/shell-2 实现+coord 工件（HEAD/哈希见 run 内 *_head.txt+meta.json）。
- 进程链亲证：VitShell(24728 代)→VitAgent→（直启）Godot 包装+编辑器子进程→VitApp+VspHub；关主窗 Job 全灭（L7 亲证=0）。
- **稳定性口径**：单 run exit 0=存在一条成功路径（卡面验收标准为全旅程 exit 0 单腿达标）；重复成功比例未另规——留 SHELL-3 放行面。
- 前置失败 run（取证在卷，均已拆栈）：run_023349（PS5.1 stderr 终止错误，装置缺陷）；run_023650（go build 行控制字节污染，装置缺陷）；run_023936（CDP 模块解析，装置缺陷）；run_024518（launch-result 离树 get_tree() null——**真缺陷修复 Engine.get_main_loop**）；run_025439（快照文件名清洗符，装置缺陷）；run_031129（治理卡 bridgeBusy 禁用——**真产品缺陷修复**）；run_031749（瞬态 answered 断言竞速，装置断言降级）。真缺陷两次均「代码锚点+最小修复」后前进，无原样重跑凑过。

## 状态：DONE（执行侧自验完成，终验归 GLM 主管决策流）

- 实现：前端 port/shell-2 @ 4deeb34；主仓 port/shell-2 @（见卡面回执）；双仓留 port 候验收不合 main。
- 断点续跑记录：本任务全程单会话执行，无重派中断；coord 推送无冲突。
- 详见卡面回执（done/2026-11-15-SHELL-2.md）与 run 工件。
