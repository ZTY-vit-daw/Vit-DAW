# UI-OS-1 真栈烟测 run 记录

- run ID（最终 PASS）：`run_20261114_193656`，exit 0，2026-11-14 19:36:56–19:37:32 +0800（36s）
- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_uios1/scripts/ui_os_smoke.ps1`
- 被测：主仓 worktree `D:/Vit_DAW_wt_uios1` @ port/ui-os-1（fed5e3b9 烟测脚本轮；实现=脚本 4 提交）；前端 worktree `D:/Godot/vit-daw-frontend_wt_uios1` @ port/ui-os-1（128477b = 1e0713a extension OS 语义+通知桥 / 9dab9fa gd 生命周期+通知桥 / 128477b dock+start_page，双变体 dll 重编）
- 内核：`D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe`（零内核改动只读复用）
- agent：现场构建隔离二进制（sha256 在 report；HTTP 7878 + UDP 4445 bridge）
- Godot：`D:/Godot/Godot_v4.6.1-stable_win64.exe --path D:/Godot/vit-daw-frontend_wt_uios1`（VIT_AGENT_WINDOW_MODE=autoload + VIT_SMOKE_AUTO_OPEN_PROJECT=<隔离 fixture 副本> + VIT_SMOKE_OS_NOTIFY=1 + VIT_SKIP_DEV_AUTOSTART=1）
- fixture：912.vit 复制到 run 目录；源 sha256 F6CA36FA…0BD 运行前后一致（343 基线哈希同值）=零污染亲证
- 占用：登记 203c33e9 → ≤60s 复核（进程=0/端口=0+重读远端记录）→ 启动；finally 拆栈复核（进程=0/五端口监听=0）→ 释放记录随本文件推送

## 断言结果（PASS 轮，71 断言零意外）

| # | 断言 | 证据 |
|---|---|---|
| A（343 不变量） | 同一 HWND 跨场景切换 + navigate_count==1 + URL 不变 | child_hwnd=526050 在 boot/visible/page_loaded/auto_open.done/scene_added 五行全同 |
| B1 | 任务栏条目可区分（图标/标题） | chat title=="Vit"+GCLP_HICON 非零；同 PID DAW 主窗 title="Vit_DAW_Frontend (DEBUG)"≠"Vit"；枚举恰 1 窗（无重复条目） |
| B2 | 前台门控（打开在前不通知） | N1 notify.request foreground=true → issued=false + os_event notification_skipped |
| B3 | 最小化→任务栏+topmost 撤复 | SW_MINIMIZE→IsIconic=true+WS_EX_TOPMOST 撤销+os_event minimized；召回同 HWND 可见+topmost 复立 |
| B4 | 后台真实通知 | N2 notify.request foreground=false → issued=true + os_event notification_shown（Shell_NotifyIconW 真实触发）+ **notification_visible（NIN_BALLOONSHOW，OS 上屏实证）** |
| B5 | 通知召回同一窗口 | notify.recall.done child_hwnd=526050（同 HWND）+ SW 恢复可见 |
| B6 | 关闭≠最小化（真关不收拢） | PostMessageW(WM_CLOSE)→IsWindow=false+枚举 0+os_event closed（DestroyWindow，非 SW_HIDE） |
| B7 | 关闭后重建 | 新 HWND=461058≠旧+boot_index==2+navigate_count==1（teardown 归零+单次导航）+page_loaded+URL /app/ + 前后前台不变（SW_SHOWNA 不抢焦点）+topmost 复立+枚举恰 1 |
| C | legacy flag 腿 | 进程存活+零 boot 行 |
| 记录项 | DPI | chat_window_dpi=168（系统当前 175% 缩放；GetDpiForWindow 在卷；125%/150% 视觉缩放=人工目检项） |

软断言 `n2_balloon_on_screen_os_confirmed` 实测 **true**（超出软预期：OS 亲证 toast 上屏）。

## §8 运行历史（同代码版本，前端实现零改动）

- run_20261114_193333（exit 1）：boot 行缺 child_hwnd——脚本缺陷（343 的 child_hwnd 在 visible 行，本脚本误从 boot 行取）→ 修复+放宽 JSON 键序匹配（Godot JSON.stringify 键排序实证）
- run_20261114_193526（exit 0 但断言未全 gate）：`recreated_navigate_count_2` 预期与自身设计矛盾（gd/PROGRESS 设计=teardown 归零+重建导航恰一次，实测 1）且该断言漏 throw 门禁 → 校准为 navigate_count==1+补门禁（装置修复轮，非功能重跑）
- run_20261114_193656（**exit 0**）：全断言 PASS

失败轮均为烟测脚本装置缺陷（脚本面修复，每轮明确根因），前端实现三提交跨三轮零改动。

## 端测覆盖边界

- 覆盖：真实三件套 autoload 腿全链（boot→React 加载→start_page 真实 open 链→dock）；OS 行为矩阵脚本面（任务栏区分/前台门控/最小化 topmost 撤复/后台真实通知含 OS 上屏实证/同 HWND 召回/WM_CLOSE 真关/重建新 HWND 不抢焦点/枚举唯一性）；legacy 腿
- 未覆盖（留人工目检/后续卡）：OS toast **实点**（鼠标点击通知本体→NIN_BALLOONUSERCLICK→召回+定位——脚本以同一生产召回路径合成驱动，实点受 computer-use 禁令约束）；通知两动作按钮的 OS 呈现（Win32 balloon 无动作按钮，动作以 ≤2 条元数据契约传递，WinRT toast 需 AppUserModelID=另卡裁定）；i-vit 状态点颜色（OS 版式限制：confirm/error 有系统图标，done 无蓝点）；DPI 125%/150% 视觉缩放与双屏目检；React 页内定位消费（vit_notify_activated 消费面=348 桥域）；触发链（agent 事件→notify）=348 桥域
- 探针约束：N1 前台=page_loaded 时刻单点探针；合成召回与重建经 show_window 生产路径驱动

## 工件

- `artifacts/ui_os_smoke_report.json`（71 断言+PID+哈希+DPI）
- `logs/godot_autoload.log`（[AgentChatWindow] 全事件行含 os_event/notify.*）/ `logs/godot_legacy.log` / `logs/agent_last.log`
- 失败轮：run_20261114_193333 / run_20261114_193526（report+log 精简件，装置缺陷取证）
- 分支：port/ui-os-1（主仓+前端仓，留待主管验收合入）
