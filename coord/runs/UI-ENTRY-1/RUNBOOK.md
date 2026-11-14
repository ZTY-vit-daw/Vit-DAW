# UI-ENTRY-1 RUNBOOK（真栈腿 §8 跑前冻结）

- 冻结时点：2026-11-14 深夜（真栈跑之前提交；跑后修改仅限追加运行历史）
- 卡：UI-ENTRY-1（池序 354，P0）；owner=freeflash·PC；分支 port/ui-entry-1 双仓

## 腿与断言面

一次运行三腿（内核+agent 单次构建启动；Godot 每腿独立 boot）：

1. **autoload 腿**（主腿，VIT_AGENT_WINDOW_MODE=autoload + VIT_SMOKE_AUTO_OPEN_PROJECT）：
   - boot 后 EnumWindows：可见顶层窗中 VitWebViewHostChildWindow 恰 1 个（React 启动台）
     **且 Godot 主窗不可见**（IsWindowVisible=false，标题含 Vit_DAW_Frontend 或 Godot 主窗类名）；
   - Godot 日志 `main_window.hidden` 行在场（child hwnd 建立后才隐藏）；
   - page_loaded 行 url=http://127.0.0.1:7878/app/?surface=launcher（URL 路由生效）；
   - smoke.auto_open.done scene=VitDockRoot；
   - `project_stage.presented` 行在场 + `surface.ack` 行 surface=project（React 真收证明）；
   - 同一 child_hwnd 跨 boot/visible/page_loaded/scene_added/project_stage.presented 五行全同；
   - navigate_count==1（进工程切换 surface **零导航**，343 不变量）；
   - page_url 三处一致（不因切面变化）；
   - 主窗恢复后 IsWindowVisible=true（进工程展开）。
2. **回退腿**（VIT_AGENT_WINDOW_MODE=autoload + **无 agent 进程**）：
   - Godot 日志 health.failed + unavailable 行在场（真链失败注入）；
   - `main_window.restore` 行在场（reason=unavailable）；
   - EnumWindows：Godot 主窗可见（启动页兜底），VitWebViewHostChildWindow 不可见。
3. **legacy 腿**（flag 不设）：Godot 进程存活 + 日志零 boot 行 + 零 main_window.hidden 行。

## 失败分类与止损

- exit 0=PASS；exit 1=断言失败；exit 2=环境（端口/进程冲突）；exit 4=装置链路（脚本自身缺陷）。
- 同一代码版本同一确定性断点连续失败两次 → 停止原样重跑（AGENTS §8）；
  装置轮修复须有根因记载（UI-HOST-1 先例：每轮修复明确根因后继续）。
- 回退腿 health 等待上限 24×0.45s≈10.8s（autoload 既有常量），超时按断言失败记录。

## 环境契约

- 内核：D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe（零内核改动只读复用，pid 记录）；
- agent：主仓 worktree 现场构建隔离二进制（go build，sha256 记录）；HTTP 7878+UDP 4445；
- Godot：D:/Godot/Godot_v4.6.1-stable_win64.exe --path D:/Godot/vit-daw-frontend_wt_uientry1
  （VIT_AGENT_WINDOW_MODE / VIT_SMOKE_AUTO_OPEN_PROJECT / VIT_SKIP_DEV_AUTOSTART=1）；
- fixture：912.vit 复制到 run 目录，源 sha256 F6CA36FA…0BD 前后亲证零污染；
  recents global_project_config.json 快照→还原哈希亲证（AGENTS §10）；
- run 根：coord/runs/UI-ENTRY-1/run_<stamp>/（logs/ + artifacts/report.json）；
- 占用：PC-RUNTIME-STACK 登记→推 main→重读确认→≤60s 复核进程端口→启栈；
  finally 拆栈+五 TCP 监听=0+进程=0 复核→释放记录提交。

## 断言不可伪造纪律

- Windows 断言全部 EnumWindows/IsWindowVisible 实测（UI-HOST-1 先例：FindWindowExW 跨进程
  不可用，用 EnumWindows+GetClassName 过滤）；
- React 侧证明只认宿主日志行（surface.ack / result_ack 族——React 真收后回投），
  不以宿主单方 post 记录充当 React 在场证明；
- 失败轮原始日志全保真，不凑断言值。
