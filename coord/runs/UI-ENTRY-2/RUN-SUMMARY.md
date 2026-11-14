# UI-ENTRY-2 真栈烟测 run 记录

- run ID（最终 PASS）：`run_20261115_003340`，exit 0，2026-11-15 00:33–00:4x +0800
- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_uientry2/scripts/ui_entry2_smoke.ps1`
- 被测：主仓 worktree `D:/Vit_DAW_wt_uientry2` @ port/ui-entry-2（86915cc8：脚本+coord；agent worktree 隔离构建 agent/bin/VitAgent.exe sha256 见 report，语义零改动）；前端 worktree `D:/Godot/project/vit-daw-frontend_wt_uientry2` @ port/ui-entry-2（83390f5：关窗退出+失败链去 Godot 化+恢复轮+splash 压制+surface 重投鲁棒性；extension cpp 零改动零重编）
- 内核：`D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe`（零内核改动只读复用；L3 经 start_page autostart 链拉起，pid 经 5555 端口归属实测）
- agent：**autostart 链拉起**（start_page OS.create_process，worktree agent/bin 最新构建经 freshness 排序选中；pid 经 7878 端口归属实测）——生产链形态（用户从 Godot 拉起全栈）
- Godot：`D:/Godot/Godot_v4.6.1-stable_win64.exe --path D:/Godot/project/vit-daw-frontend_wt_uientry2`
- fixture：912.vit 复制到 run 目录（源 sha256 F6CA36FA…0BD 前后一致亲证=零污染；recents 快照还原）
- 占用登记：98ed4ed7（§2.2 全程：净空亲核 00:3x → 登记 → 推 main 重读确认 → ≤60s 复核 → 跑 → finally 拆栈复核四 TCP=0+进程=0）

## 断言结果（PASS 轮，四腿全绿）

| 腿 | 断言 | 证据 |
|---|---|---|
| 腿 1 失败注入（agent 不可达，去 Godot 化） | health.failed + unavailable；**零 main_window.restore**；navigate 行在场（错误态导航）；EnumWindows：Godot 主窗不可见+React 窗恰 1 可见 | failinject_leg_pass=true |
| 腿 2 WebView2 不可用 | webview2.unavailable + 零 boot 行 + #32770 原生错误对话框（pid 归属）+ WM_CLOSE 关闭 → **进程退出** + 零 chat 窗 | webview2_leg_pass=true |
| 腿 3 autoload 全链 | 启动态单窗（child_hwnd=13173540）+ launcher URL + 真实 open 链 → VitDockRoot + presented(surface_posted=true) + **surface.ack(surface=project)** + 同 HWND + navigate_count==1 + URL 不变（343 不变量） | autoload_leg_pass=true |
| **腿 3 本卡核心：关窗=全栈退出** | WM_CLOSE 到 child hwnd → app.exit(chat_window_closed) → **Godot/agent/kernel 三进程全部退出**（pid 逐一实测）+ 端口释放 | close_godot/agent/kernel_exited=true + close_ports_released=true |
| 腿 4 legacy 回归 | 零 boot/hidden/app.exit 行 + 进程存活 | legacy_leg_pass=true |
| webui 三门 | 零 diff 回归：vitest 47 文件 **563/563 PASS** + tsc/vite build exit 0 | S7（worktree agent/webui） |

## §8 运行历史（同修复族；每轮根因明确）

- run_20261115_002604（exit 1）：腿 1/2 PASS，腿 3 败于 surface.ack 缺失。**根因（真链取证）**：presented 的 vit_surface 早于 React 完成挂载/订阅（page_loaded#2 晚于 presented=WebView2 冷加载时序），post 进空网丢失；React 随后以 URL 初值 launcher 挂载并宣告 launcher_surface_ready。354 PASS 日志同构形态（双 page_loaded/双 auto_open.begin/failed）仅时序偶发不同——**潜在竞速为 354 交付面固有，非本卡回归**。修复=前端 83390f5：pending surface + React 就绪宣告（launcher_surface_ready）重投 + 5s 兜底重投（post_web_message 面，343 不变量不动；ack 到达即清 pending）。
- run_20261115_003340（exit 0 PASS）：修复落地，四腿全绿（surface.ack 本轮经投递链收敛到达）。

## M3 工程态往返——停止条件命中（降级）

- 卡面停止条件触发：内核 `VitApp/Source/Service/CommandDispatcher.cpp` handlers 仅 new_project(2504)/open_project(2664)/save_project(3275)，**无 close**；前端 `project_client.gd` 同（new/open/save/save_as/get_state）；`vit_dock_root.gd` 无 dock→start_page 场景返回路径（launcher_directive dock 阶段明确降级 unsupported）。
- 处置：**不实现**（禁改内核/agent）；vit_dock_root.gd 零改动；round-trip 腿不入 RUNBOOK。交主管裁定是否另开内核卡（关闭工程命令面）。

## 端测覆盖边界

- 覆盖：真实三件套四腿全链（失败注入去 Godot 化/WebView2 系统错误框+退出/autostart 全栈+关窗全栈退出/legacy 回归）；Win32 窗口面（EnumWindows 可见性/数量/HWND/#32770 对话框归属）；React 真收面（surface.ack）；进程退出断言=pid 逐一 HasExited+端口释放实测
- 未覆盖：人工目检（启动瞬间 splash 压制残余程度——show_image=false 后纯色数帧，手测 v5 观测项；关窗退出瞬间的任务栏/焦点语义）；WebView2 错误框**文案/图标内容**（原生 MessageBox 不可枚举，断言面=类名+pid+退出链）；OS.alert 在 deferred 主窗隐藏前阻塞（引擎预显主窗数帧=354 已证限制面，leg 2 不断言主窗可见性）；WebView2 错误页（agent 不可达时窗内错误态）的 DOM 内容（WebView2 内建错误页，非 React 面）；多屏/DPI
- legacy 腿为进程存活+日志面断言（354 同口径）；kernel settings/cache 写 D:/Vit_DAW/VitApp/Workspace=354 既有行为（recents 快照还原+VIT_PROJECT_XML 隔离 default_project.xml）
- load_error 链不覆盖连接拒绝类失败（extension NavigationCompleted 不查 IsSuccess）——失败链重试经恢复轮实现（agent_chat_window.gd），extension 面缺口如实记录

## 工件

- `run_20261115_003340/artifacts/`（ui_entry2_smoke_report.json+PID+哈希+log 副本）、`run_20261115_002604/`（失败轮 report+logs 取证）
- 分支：port/ui-entry-2（主仓+前端仓）；agent/bin、fixture 工作区不入库（agent sha256 在 report；fixture 源哈希前后亲证）
