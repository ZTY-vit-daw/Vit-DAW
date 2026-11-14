# UI-HOST-1 真栈烟测 run 记录

- run ID（最终 PASS）：`run_20261114_184412`，exit 0，2026-11-14 18:44:12–18:44:46 +0800
- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_uihost1/scripts/ui_host_smoke.ps1`
- 被测：主仓 worktree `D:/Vit_DAW_wt_uihost1` @ port/ui-host-1（55852f94，烟测脚本轮）；前端 worktree `D:/Godot/vit-daw-frontend_wt_uihost1` @ port/ui-host-1（e55e901，AgentChatWindow+extension 重编）
- 内核：`D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe`（零内核改动只读复用；kernel_pid=1840）
- agent：现场构建隔离二进制（agent_pid=21928，sha256 1DA0EAAE…）；HTTP 7878 + UDP 4445 bridge
- Godot：`D:/Godot/Godot_v4.6.1-stable_win64.exe --path D:/Godot/vit-daw-frontend_wt_uihost1`（godot_autoload_pid=24240；VIT_AGENT_WINDOW_MODE=autoload + VIT_SMOKE_AUTO_OPEN_PROJECT=<隔离 fixture 副本> + VIT_SKIP_DEV_AUTOSTART=1）
- fixture：912.vit 复制到 run 目录（源 sha256 F6CA36FA…0BD 运行前后一致亲证=零污染）

## 断言结果（PASS 轮，全部 true）

| 断言 | 证据 |
|---|---|
| React 窗先出（boot） | boot 行 main_hwnd=2558138 title=Vit size=1920×1080 |
| 窗口可见 | visible 行 child_hwnd=4523840 visible=true |
| React 页面加载（StartPage 场景期间） | page_loaded 行 url=http://127.0.0.1:7878/app/ scene=StartPage |
| 进入工程（真实 open 链） | smoke.auto_open.done scene=VitDockRoot |
| **同一 HWND 跨场景切换** | child_hwnd=4523840 在 boot/visible/page_loaded/scene_added/auto_open.done 五行全同 |
| **页面未重载** | navigate_count=1（navigate 仅 autoload boot 一次） |
| **URL 未变** | page_url=http://127.0.0.1:7878/app/ 三处一致 |
| 无重复窗口 | EnumWindows 类名 VitWebViewHostChildWindow 恰 1 个；hwnd 与 log 一致 |
| 标题基线 | GetWindowTextW == "Vit" |
| 可见/最小化/召回 | IsWindowVisible=true；SW_MINIMIZE→IsIconic=true；SW_RESTORE→visible 且非 iconic |
| 初始前台探针 | 枚举时刻 GetForegroundWindow()==chat hwnd（foreground_probe_is_chat=true） |
| flag=legacy 回退 | legacy 腿 Godot pid=3368 存活 + 日志零 boot 行（启动页兜底可用） |

## §8 运行历史（同代码版本）

- run_20261114_183806（exit 1）：boot 行缺失——worktree 首跑缺 `.godot/` 缓存（GDExtension/全局类不加载）→ 装置缺陷，脚本补 `--import` 预跑步（装置修复轮，非功能重跑）
- run_20261114_183958（exit 1）：核心断言（HWND 同一/未重载/URL）已过，OS 基线 Win32 枚举 0——FindWindowExW 跨进程返回未命中（装置面）
- run_20261114_184205（exit 1）：CharSet.Unicode 修复仍 0——装置面确认 FindWindowExW 不可用，改 EnumWindows+GetClassName 过滤（探针验证 EnumWindows 可靠）
- run_20261114_184412（**exit 0**）：全断言 PASS

失败轮均为烟测脚本装置缺陷（脚本面修复），非功能缺陷；同断点两连败仅发生在装置面且每轮修复了明确根因。

## 端测覆盖边界

- 覆盖：真实三件套 autoload 腿全链（boot→React 加载→start_page 真实 open 链→场景切换→dock）；Win32 窗口面（枚举/标题/最小化/召回/前台探针）；legacy flag 腿（进程存活+零 boot 行）
- 未覆盖：人工目检（窗口布局/React 视觉/双屏/DPI 125%-150% 缩放观测=R0 §4.5 待真栈观测项，不在本卡）；React 页内交互（launcher surface 不在本卡文件域）；`toggle_chat_window` transport 按钮的手点路径（由 dock 委托 autoload 的代码路径+同窗断言间接覆盖）；多屏/最小化召回的 Windows 键鼠人工复核（脚本 ShowWindow 自动化面已断言）
- 探针约束：初始前台为「枚举时刻前台=chat 窗」的单点探针（SetForegroundWindow 受系统前台锁约束，R0 §4.3 已知）

## 工件

- `artifacts/ui_host_smoke_report.json`（全断言+PID+哈希）
- `logs/godot_autoload.log`（[AgentChatWindow] 全事件行）/ `logs/godot_legacy.log` / `logs/agent_last.log`
- 失败轮：`run_20261114_183{806,958}`、`run_20261114_184205`（report+log 精简件，装置缺陷取证）
- 分支：port/ui-host-1（主仓+前端仓）
