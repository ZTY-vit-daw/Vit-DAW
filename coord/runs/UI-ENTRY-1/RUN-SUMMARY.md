# UI-ENTRY-1 真栈烟测 run 记录

- run ID（最终 PASS）：`run_20261114_233821`，exit 0，2026-11-14 23:38:21–23:4x +0800
- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_uientry1/scripts/ui_entry_smoke.ps1`
- 被测：主仓 worktree `D:/Vit_DAW_wt_uientry1` @ port/ui-entry-1（9bcea3f5+脚本修复轮）；前端 worktree `D:/Godot/vit-daw-frontend_wt_uientry1` @ port/ui-entry-1（ae24b52：extension SW_HIDE 主窗 face + gd 编排；双变体 dll 重编）
- 内核：`D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe`（零内核改动只读复用；kernel_pid=18436）
- agent：现场构建隔离二进制（agent_pid=30860，sha256 ADD96F5B…4025）；HTTP 7878 + UDP 4445 bridge
- Godot：`D:/Godot/Godot_v4.6.1-stable_win64.exe --path D:/Godot/vit-daw-frontend_wt_uientry1`（autoload_pid=8448 / fallback_pid=28624 / legacy_pid=25788；VIT_AGENT_WINDOW_MODE=autoload + VIT_SMOKE_AUTO_OPEN_PROJECT=<隔离 fixture 副本> + VIT_SKIP_DEV_AUTOSTART=1）
- fixture：912.vit 复制到 run 目录（源 sha256 F6CA36FA…0BD 运行前后一致亲证=零污染；recents 快照还原）
- 占用登记：da674ce1（§2.2 全程：净空亲核 23:24 → 登记 → owner 重读确认 → ≤60s 复核 → 跑 → finally 拆栈复核五 TCP=0+进程=0）

## 断言结果（PASS 轮，全部 true）

| 断言 | 证据 |
|---|---|
| 回退腿（真链失败注入：无 agent） | health.failed → unavailable 行 → main_window.restore(reason=unavailable) → Godot 主窗可见 + chat 窗隐藏 |
| **启动态单窗** | main_window.hidden(via=extension_showwindow) 后轮询：可见 VitWebViewHostChildWindow 恰 1（hwnd=1444096）+ Godot 主窗 IsWindowVisible=false |
| 启动页路由 | page_loaded url=http://127.0.0.1:7878/app/?surface=launcher |
| 进工程展开 | smoke.auto_open.done scene=VitDockRoot + project_stage.presented(surface_posted=true) + 主窗恢复可见（mode=maximized 还原） |
| **React 真收切面** | surface.ack surface=project（React 收 vit_surface 后回执，宿主日志留痕） |
| **同 HWND 跨进工程** | child_hwnd=1444096 在 boot/hidden/visible/scene_added/auto_open.done/project_stage.presented 行全同 |
| **同窗不重载** | navigate_count=1（boot 一次；surface 切换零导航——343 不变量） |
| URL 不变 | page_url=launcher URL 跨 scene_added/presented/auto_open.done 三处一致 |
| 工程态双窗 | 主窗可见 + chat 窗 1 个可见（EnumWindows） |
| legacy 腿 | 零 boot 行 + 零 main_window.hidden 行 + 进程存活（flag 不设零行为变化） |

## §8 运行历史（同代码版本族；每轮根因明确）

- run_20261114_231928（exit 1）：**装置**——PS1 无 BOM UTF-8 中文注释被 PS5.1 按 ANSI 误解码（UI-BRIDGE-1 同款）；加 BOM 修复
- run_20261114_232032（exit 1）：**装置**——EnumWindows delegate 内 `$pid` 与 PowerShell 只读自动变量冲突；改名 $procId 修复
- run_20261114_232223（exit 1）：**引擎限制发现轮**——`main_window.hidden` 在场但主窗仍可见：Godot 4.6.1 主窗 hide 被窗口状态机 restored 链反转（window.cpp:939）；修复假设=先落回 windowed 再 hide
- run_20261114_232520（exit 1）：**引擎限制确认轮**——deferred 两帧后 hide 仍失效；probe（232728/232856）铁证：**Godot 对主窗 hide 降级为最小化**（IsIconic=true、IsWindowVisible 恒 true、任务栏图标残留）——GDScript 层无真隐藏路径
- run_20261114_233618（exit 1）：**修复落地轮+脚本竞速暴露**——extension `set_parent_window_shown`（SW_HIDE 主窗 HWND）生效（hidden 行 via=extension_showwindow）但 startup 断言窗口（page_loaded 后 2s）与 auto_open 竞速，restore 已先行
- **run_20261114_233821（exit 0 PASS）**：断言锚点前移（hidden 行后轮询静置窗口），三腿全绿

## 端测覆盖边界

- 覆盖：真实三件套三腿全链（启动隐藏→launcher 路由→真实 open 链→进工程展开→同窗切面→回退→legacy）；Win32 窗口面（EnumWindows 可见性/数量/HWND 一致性）；React 侧真实收面（surface.ack 回执通道）
- 未覆盖：人工目检（启动瞬间残余闪现程度——Godot 引擎启动序列在 deferred hide 前显示主窗，boot splash 短暂可见，手测 v4 观测）；React project surface 渲染细节（LoadingGate 放行后 DOM 面=346/347 既有渲染面，本卡只证切换消息真收）；任务栏语义（启动态主窗隐藏后任务栏无 DAW 图标=SW_HIDE 真隐藏语义，任务栏交互手测 v4）；多屏/DPI
- legacy 腿为进程存活+日志面断言（UI-HOST-1 同口径）

## 工件

- `run_20261114_233821/artifacts/`（全断言 report+PID+哈希+log 副本）、失败轮 231928/232032/232223/232520/233618 artifacts（report+log 取证）、probe_20261114_232728/232856（引擎限制时间线）
- 分支：port/ui-entry-1（主仓+前端仓）；bin/fixture 工作区不入库（agent 二进制 sha256 在 report，fixture 源哈希前后亲证）
