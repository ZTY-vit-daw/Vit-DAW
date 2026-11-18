# SHELL-WIN-PRODUCER-1：409 产品基线的呈现数据源移植

- 池序 410；P1；PC；发卡/验收=本会话 PC 主管（lead）；派发确认=已确认（用户 2026-11-18 晚批准方案 A，授权主管拆卡/派发/验收）。
- owner=`lead/producer410`（DeepSeek-V4.1-Flash 执行子代理，PC）；分支 `codex/shell-win-producer-410`；worktree=`C:/Users/timoz/.codex/worktrees/pc410-producer/vit-daw-frontend`；领取基线=前端 `origin/main@bc8daf6`（tracked clean）；协调基线=主仓 `738291f6`。
- 单目标：在 409 产品窗口基线上补齐 395 消费的只读呈现数据源；不改窗口行为、不启真实栈。

## 依据

- 394-B `e9006de`：`status_server` 可选 window_state reader + `GET /status.window_state`、`main.cpp` 接线。
- 394-D `7bd812f`：`vit_launch_progress`（`vitshell.launch_progress.v1`）+ `ask_vit_shell/window_presentation_ready` 握手/replay。
- 409 `bc8daf6`：产品宿主/守卫现状；395 消费端已按该协议实现（见 `codex/shell-presentation-395`）。

## 实现范围（只准移植+最小适配，禁止新设计）

1. 移植 `e9006de` 的 status reader 机制（status_server.h/.cpp + main.cpp 接线）。
2. 新增 `GodotManager::WindowStateJson()` 的最小 409 适配（**不移植** 394-A/E/E2 的 hidden bootstrap / raw event / loading guard 实现）。契约：
   `{"schema":"vitshell.window_state.v1","foreground":"shell|daw|other|none","daw_hwnd":"<十进制>"|null,"observed_at_tick_ms":<number>}`
   daw 主窗由本进程 Godot 子树 PID 匹配（可新增 EnumWindows 辅助）；foreground 由 `GetForegroundWindow()` 与 shell/reference/daw 对比；无法判定→`none`/null，不编造。可选附加只读字段不改变上述语义。
3. 移植 `7bd812f` 的 progress 发布与 replay：`type/schema/request_id/seq/phase/detail/stage` 照原实现；stage 只来自已核 launch-result 的 `workspace_ready|scene_accepted`；replay 不自增 seq；无记录保持 unknown。
4. 宿主在 window_state 变化时推送 `vit_window_state`：有界轮询（≤250ms，仅变化时发；读失败不得伪造）。焦点事实只能来自实际 Win32 读数。

## 文件域（仅此）

`VitShell/status_server.h`、`status_server.cpp`、`main.cpp`、`godot_manager.h`、`godot_manager.cpp`、`launcher_bridge.h`、`launcher_bridge.cpp`、`VitShell/tests/*`、`VitShell/CMakeLists.txt`。不得改 `extension/`、`app/**`、`project.godot`、主仓。

## 必须排除

394-A/E/E2 的 hidden bootstrap、E2 候选、raw WinEvent、OBS 观察器、394-C launch_id 相关性（除非 409 已需要）。与 409 守卫/窗口行为无关的一切改动不得顺手夹带。

## 验收（无窗；全部亲跑并留原文）

1. `cd <worktree> && cmake -S VitShell -B VitShell/build -G "Visual Studio 18 2026" -A x64 "-DVITSHELL_WEBVIEW2_DIR=D:/Godot/project/vit-daw-frontend/extension/third_party/webview2"` exit 0。
2. `cmake --build VitShell/build --config Release` exit 0。
3. `ctest --test-dir VitShell/build -C Release --output-on-failure` exit 0（移植的 serialization/presentation 纯测试 + 既有 guard 测试）。
4. `git diff --check` exit 0；给出 window_state JSON 样例与 progress 消息样例各一条（测试输出即可）。

## 停止条件

同确定性断点失败两次→停并上交；需要改 extension/gd/主仓/真栈/窗口行为→停报；契约字段无法在无 E/E2 前提下提供→停报。

## 回执（短）与禁止

commit hash+分支、文件清单、每命令 exit 码原文、测试计数、契约样例、边界声明（零真栈/零窗口）。禁止 push main、改主检出、启 VitShell/Godot GUI/agent 栈、跨卡文件域。
