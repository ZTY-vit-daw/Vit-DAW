# SHELL-READY-SEAM-1：workspace_ready 真实回执接入 409 产品路径

- 池序 411；P1；PC；发卡/验收=本会话 PC 主管（lead）；派发确认=已确认（用户 2026-11-18 晚批准方案 A）。
- owner=`lead/ready411`（DeepSeek-V4.1-Flash 执行子代理，PC）；分支 `codex/shell-ready-seam-411`；worktree=`C:/Users/timoz/.codex/worktrees/pc411-ready/vit-daw-frontend`；领取基线=前端 `origin/main@bc8daf6`。
- 单目标：在 409 隐藏启动产品路径上给出真实 `workspace_ready`（dock `_ready` 完成 + 首份内核工程快照成功 apply），供 395 呈现「工程已载入」；legacy 路径行为不变。

## 依据

- 402 `b1f9406`：`app/shell/vitshell_launch_readiness.gd`、start_page 接线、`vit_dock_root.gd` hooks、探针。
- 409 `bc8daf6`：`vitshell_window_entry.gd` 的 arm/present/complete 与唯一 launch-result POST；start_page 隐藏启动分支。

## 实现（≤4 生产文件；只准移植+最小适配）

1. 新增 `app/shell/vitshell_launch_readiness.gd`（取 `b1f9406` 版本）。允许差异：`begin` 增加 `post_receipt=false` 选项，由 `vitshell_window_entry` 统一唯一回执；`launch_id` 用 `VIT_SHELL_LAUNCH_ID` 或进程内唯一 fallback。
2. `app/startup/start_page.gd`：**仅** `VIT_SHELL_REQUIRE_HIDDEN_BOOT=1` 分支在 arm window entry 前创建 root-owned readiness 节点并 begin；legacy 分支保持 409 原状。
3. `app/startup/vitshell_window_entry.gd`：`complete()` 在 `present_initial` 之后等待 readiness 有界决策，**只 POST 一次**；合并 payload：`ok` = 409 语义且 readiness 成功；新增 `stage`=`workspace_ready` / `scene_accepted`；失败细节入 `detail`；超时不得报 ready。
4. `vit_dock/scenes/vit_dock_root.gd`：从 `b1f9406` 取 `workspace_mounted/snapshot_applied/snapshot_failed` 接线（无 readiness 节点时 no-op）。

## 文件域（仅）

`app/shell/vitshell_launch_readiness.gd`（新）、`app/startup/start_page.gd`、`app/startup/vitshell_window_entry.gd`、`vit_dock/scenes/vit_dock_root.gd`、必要适配探针（`tools/` 下 402 探针）。不动 VitShell C++/extension/project.godot/webui。

## 验收（无窗）

1. `D:/Godot/Godot_v4.6.1-stable_win64_console.exe --headless --import --check-only --path <worktree>` exit 0；`--headless --import` exit 0。
2. 402 探针适配后成功/快照失败/超时/legacy 四情景 exit 0。
3. 静态断言 hidden-boot 路径只发生一次 launch-result POST（不得 readiness 与 window_entry 双发）。
4. `git diff --check` exit 0。

## 停止条件

需改 C++/协议/真栈→停报；402 契约与 409 语义冲突无法卡内解决→停报；同确定性断点两次失败→停。

## 回执（短）与禁止

commit hash+分支、文件清单、命令+exit 原文、四情景结果、唯一回执静态证据、边界声明（零真栈/零窗口）。禁止 push main、改主检出、启 VitShell/Godot GUI/agent 栈。

## 回执与验收（2026-11-18 晚）

- commit `4784f9b5`（分支 `codex/shell-ready-seam-411`）；主管独立复跑 import check exit 0 与 `tools/run_vitshell_readiness_unit.ps1` exit 0（semantic 21/seam 28/assembly 8），dock file 与 402 版本逐字节一致；合入前端 main `cf33195b` 并推送。
- 裁定：[rulings/2026-11-18-SHELL-READY-SEAM-1-pass.md](../../rulings/2026-11-18-SHELL-READY-SEAM-1-pass.md)。实现面 PASS；真实装配与端到端由 414 组合真栈裁决。
