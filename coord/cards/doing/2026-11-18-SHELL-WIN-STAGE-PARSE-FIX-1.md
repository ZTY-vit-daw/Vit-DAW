# SHELL-WIN-STAGE-PARSE-FIX-1：宿主顶层 stage 解析修复（414 attempt01 集成缺陷）

- 池序 415；P1；PC；发卡/验收=本会话 PC 主管（lead）；派发确认=已确认（组合真栈实测缺陷，属必要修复，非扩域）。
- owner=`lead/producer410`（续用 410 原执行子代理，DeepSeek-V4.1-Flash，PC）；分支 `codex/shell-stage-fix-415`；worktree=`C:/Users/timoz/.codex/worktrees/pc415-fix/vit-daw-frontend`；基线=前端 `origin/main@cf33195`（含 410 producer + 411 readiness）。
- 缺陷证据：[ATTEMPT-01](../runs/SHELL-WIN-COMBO-1/ATTEMPT-01.md)、run `coord/runs/SHELL-WIN-COMBO-1/run_20261118_184423`。
  - Godot：`readiness.decided/merged stage=workspace_ready`、`launch_result.post_once stage=workspace_ready`。
  - 宿主：`status_opened.chain_detail="scene_accepted: ..."`。410 `launcher_bridge.cpp` 的 `JsonStringValue(result_body,"stage")` 命中 payload 中先出现的**嵌套 observations 样本字段** `"stage":"show_return"`，顶层 `"stage":"workspace_ready"`（在 observations 之后）被遮蔽 → 395 terminal=unconfirmed。

## 修复要求（最小、不改变其他语义）

1. 新增**顶层 key** 字符串读取（深度/字符串转义感知；只接受对象 depth=1 的 key），用于 launch-result body 的 `stage`；顶层缺失 → 返回空（未知，绝不据此报 workspace_ready）。可同时让 `scene`/`detail` 使用同一稳健读取，但不得改变既有行为。
2. 保留 409/410/411 全部既有契约（replay/seq/window_state/唯一回执等不动）。
3. 单元测试（`VitShell/tests/launcher_presentation_test.cpp` 或等价）至少覆盖：
   - payload 先有嵌套 `"stage":"show_return"`（observations 内）再有顶层 `"stage":"workspace_ready"` → 解析结果 `workspace_ready`；
   - 仅嵌套 stage、无顶层 → 空（不得取嵌套值）；
   - 顶层 `scene_accepted` 正常；
   - 转义字符串/截断/缺 key 反例不崩溃且返回空或正确值。
4. 文件域（仅）：`VitShell/launcher_bridge.cpp`、`VitShell/launcher_bridge.h`（如需）、`VitShell/tests/*`。不动 Godot `.gd`、extension、webui、主仓。

## 验收（无窗；亲跑留原文）

1. `cmake -S VitShell -B VitShell/build -G "Visual Studio 18 2026" -A x64 "-DVITSHELL_WEBVIEW2_DIR=D:/Godot/project/vit-daw-frontend/extension/third_party/webview2"` exit 0；
2. `cmake --build VitShell/build --config Release` exit 0；
3. `ctest --test-dir VitShell/build -C Release --output-on-failure` exit 0（新增嵌套遮蔽反例全过）；
4. `git diff --check` exit 0。

## 停止与回执

同确定性断点两次失败→停；需改 411 协议/其他域→停报。回执：commit hash、文件、命令+exit、新测试计数、顶层解析样例、边界（零真栈）。
