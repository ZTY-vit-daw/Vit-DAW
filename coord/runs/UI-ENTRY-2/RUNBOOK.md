# UI-ENTRY-2 RUNBOOK（真栈腿跑前冻结）

- 冻结时点：2026-11-15 凌晨（真栈跑之前提交；跑后修改仅限追加运行历史）
- 卡：UI-ENTRY-2（池序 355，P0）；owner=freeflash·PC；分支 port/ui-entry-2 双仓
- 语义基线：354（ae24b52+主仓 ui_entry_smoke.ps1）+ 手测 v4 用户裁定——启动页门面退役

## 腿与断言面

一次运行四腿（每腿独立 Godot boot；leg3 全栈经 start_page autostart 链拉起）：

1. **失败注入腿（agent 不可达，去 Godot 化）**（autoload + VIT_SKIP_DEV_AUTOSTART=1 + 无 agent 无内核）：
   - boot 行在场（React 窗建成）；main_window.hidden 行在场（启动隐藏照常）；
   - health.failed + unavailable 行在场（真链失败注入）；
   - **零 `main_window.restore` 行**（全程——354 的 unavailable 回退链已退役）；
   - navigate 行在场（错误态导航已尝试；窗内错误态=WebView2 连接失败页）；
   - EnumWindows 静置轮询：Godot 主窗不可见（**Godot 启动页零出现**）+ VitWebViewHostChildWindow 恰 1 可见（React 窗保持在场呈现错误态）。
2. **WebView2 不可用腿**（autoload + VIT_SMOKE_WEBVIEW2_FAIL=1 + VIT_SKIP_DEV_AUTOSTART=1）：
   - webview2.unavailable 行在场 + **零 boot 行**（建窗路径中止）；
   - EnumWindows：本进程 **#32770 原生错误对话框出现**（OS.alert，MB_ICONERROR）；
   - 脚本对该对话框投递 WM_CLOSE（窗口消息自动化，354 UI-OS-1 同先例，非主机控制）→
   - **Godot 进程退出**（≤15s）+ 全程零 VitWebViewHostChildWindow。
   - 边界声明：①对话框**文案/图标内容**不做进程内断言（原生 MessageBox 内容不可枚举），
     断言面=对话框类名+归属 pid+退出链；②OS.alert 在 autoload _ready 内阻塞（先于
     deferred 主窗隐藏）——引擎启动序列在此前已预显主窗（354 已证实的引擎限制面），
     故本腿不断言主窗可见性（进程随后退出，闪现归手测 v5 splash 残余观测项）。
3. **autoload 全链腿（关窗=全栈退出）**（autoload + autostart 开 + VIT_SMOKE_AUTO_OPEN_PROJECT）：
   - 前段同 354：boot→main_window.hidden→静置单窗轮询（可见 VitWebViewHostChildWindow 恰 1
     且 Godot 主窗不可见）→page_loaded(url=?surface=launcher)→smoke.auto_open.done→
     scene_added(VitDockRoot)→project_stage.presented→surface.ack(surface=project)→
     主窗恢复可见+同 child_hwnd+navigate_count==1+URL 不变（343 不变量）；
   - **新断言（本卡核心）**：pid 经端口归属实测（7878 owner=agent、5555 owner=kernel）后，
     对 child hwnd 投递 WM_CLOSE → 轮询断言：
     Godot 进程退出 + **agent 进程退出 + kernel 进程退出**（VitIpcClient._exit_tree
     终止 autostart 登记子进程）+ 五 TCP 端口释放；
   - 经此腿**不经 finally 杀进程即无残留**（finally 仅兜底）。
4. **legacy 回归腿**（flag 不设 + VIT_SKIP_DEV_AUTOSTART=1 + agent 存活[与 354 环境等价]）：
   - 进程存活 + 零 boot 行 + 零 main_window.hidden 行 + 零 app.exit 行（354 断言面+本卡新增零变）；
   - 环境边界（诚实记录）：本腿跑在 leg3 全栈退出之后——kernel 不在场（IPC 状态行与 354
     不同），断言面不受影响（354 legacy 断言只看 chat 窗编排行与进程存活）。

## M3 工程态往返——停止条件命中（降级声明）

- 卡面停止条件：「工程关闭语义在内核/agent 侧无既有命令（关闭工程=仅能退出应用）→ 记录降级，交主管裁定」。
- 勘察证据：VitApp/Source/Service/CommandDispatcher.cpp handlers 仅 new_project(2504)/
  open_project(2664)/save_project(3275)——无 close；前端 project_client.gd 同（new/open/save/save_as/get_state）；
  vit_dock_root.gd 无 dock→start_page 场景返回路径（launcher_directive 在 dock 阶段明确降级 unsupported）。
- 处置：M3 不实现；round-trip 腿不入 RUNBOOK；卡面回执记录降级待主管裁定是否另开内核卡。

## 失败分类与止损

- exit 0=PASS；exit 1=断言失败；exit 2=环境（端口/进程冲突）；exit 4=装置链路（脚本自身缺陷）。
- 同一代码版本同一确定性断点连续失败两次 → 停止原样重跑（AGENTS §8）；每轮修复须有根因记载。
- OS.alert 为阻塞模态：若 #32770 轮询超时（15s），按装置失败记录并 stop 进程取证，不跳过断言。

## 环境契约

- 内核：D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe（零内核改动只读复用；
  leg3 经 autostart 链拉起，VIT_PROJECT_XML 指 run 目录副本隔离 workspace）；
- agent：**worktree 隔离构建** agent/bin/VitAgent.exe（go build，sha256 记录；autostart
  freshness 排序自动选中 worktree 最新构建；agent 语义零改动）；
- Godot：D:/Godot/Godot_v4.6.1-stable_win64.exe --path D:/Godot/project/vit-daw-frontend_wt_uientry2
  （VIT_AGENT_WINDOW_MODE / VIT_SMOKE_AUTO_OPEN_PROJECT / VIT_SMOKE_WEBVIEW2_FAIL /
  VIT_SKIP_DEV_AUTOSTART 按 leg 设置；dll 已提交入库零重编零 junction）；
- fixture：912.vit 复制到 run 目录，源 sha256 前后亲证零污染；recents
  global_project_config.json 快照→还原哈希亲证（AGENTS §10）；
- 日志隔离：VIT_AGENT_LAST_LOG_PATH/VIT_HISTORY_DRAFT_ROOT 指 run 目录（autostart
  子进程经 _set_environment_if_empty 尊重预置值）；
- run 根：coord/runs/UI-ENTRY-2/run_<stamp>/（logs/ + artifacts/report.json）；
- 占用：PC-RUNTIME-STACK 登记→推 main→重读确认→≤60s 复核进程端口→启栈；
  finally 拆栈+五 TCP 监听=0+进程=0 复核→释放记录提交。

## 断言不可伪造纪律

- Windows 断言全部 EnumWindows/IsWindowVisible 实测（FindWindowExW 跨进程不可用先例）；
- 进程退出断言=pid HasExited 实测+端口释放实测，不以脚本单方 kill 充当；
- React 侧证明只认宿主日志行（surface.ack 族）；失败轮原始日志全保真，不凑断言值。
