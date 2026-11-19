# 裁定：CONV-CROSS-COMBO-1（422）PASS

- 日期：2026-11-19 19:13 +0800
- 卡：422 CONV-CROSS-COMBO-1（B2b 跨工作树真栈组合验证）
- 裁定：**真栈 PASS，装置接受，卡收口**。
- 基线：main `b9c9d943`；分支 `codex/conv-cross-combo-422` @ `80c03f12` / `b971b1cd` / `218dff3c`；验收合入 main `99b21d1b` / `d6ee2742` / `f6d33a98`。
- 正式轮：[run_20261119_1928/summary.json](../runs/CONV-CROSS-COMBO-1/real-stack/run_20261119_1928/summary.json)；`status=passed`、166 断言 0 失败、S0–S6 全 true、`single_reload=1`、`conversation_open_open_project=1`、`target_path_seen=true`、recents/authority 还原 true、零 LLM 遥测。
- 栈：teardown 后 Vit 系进程 0、TCP 5555/5556/7878/7879/8787/9333 与 UDP 4445 全 0；recents `63D0F3F7…`、`Settings.xml`、`default_project.xml` 均还原一致。

## 验收事实

- S1：真实工程 A 创建/保存；save_as 返回 `project_workspace`（本轮无 warning）；`.vit_agent/<uuid>/manifest.v2.json` 身份一致。
- S2：`version.worktree_create` 建 B；`version.branch_create` 建非 main 目标分支/节点；`project.open A` 复位；单次 setup reload 断言通过。
- S3：`version.status {project_path:B}` 只读浏览；内核仍 A、零 open_project、A/B 文件哈希不变。
- S4：`version.conversation_open {project_file_path:B, branch, node_id}`：目标路径/分支/节点/HEAD 激活；`target_materialized=true`、`kernel_reloaded=true`、`same_worktree=false`、顶层 `worktree_is_root=false`；**单次 open_project=1**；UI/shadow 工程路径与 DAW 投影一致；A 文件未变。
- S5：未知/失效 worktree、未知 node/branch：HTTP 错误、`target_materialized=false`、`kernel_reloaded=false`、零 reload、当前工程不变。
- S6：recents/authority 快照还原哈希亲证；装置自检只写 RunRoot；栈由 owner 停后全零。

## 轮次与装置修（如实记录）

1. **1905 首轮 S1 失败**：`project.save_as` status=ok 但 `project_workspace_warning="rename ... Access is denied"`；属已知瞬态 Windows rename（`history/working_session_test.go:447` 同型），脚本误把 warning 当失败。装置修 `b971b1cd`：记录 warning（分类 `windows_rename_transient`），改断言目标 `.vit_agent/<uuid>/manifest.v2.json` 可用且身份一致；不可用才 fail。
2. **1920 第二轮 S4 失败**：脚本读 `result.worktree.is_root`（raw row 无此键→默认 true），产品实际字段是顶层 `result.worktree_is_root=false`。装置修 `218dff3c`；其余 S4 断言未放宽。
3. **1928 第三轮 PASS**。

## 关键契约更正

- `version.status` 的真实参数是 `project_path`（非卡面写的 `project_file_path`）；已在 RUNBOOK/脚本按实现登记，产品未改。
- `conversation_open` 的根判据是顶层 `worktree_is_root`；`result.worktree` 是 raw Project History row，不保证含 `is_root`。

## 覆盖边界

- 物化后 **reload 失败**无法在真栈确定性注入，仅 419 fake-kernel 单测覆盖；未在本轮制造。
- `target_materialized="unknown"` 未覆盖。
- Godot GUI 像素不驱动；只断言 agent-visible 投影（UI state/shadow/项目路径/DAW 计数），无截图。
- 目标节点是产品 seed + `branch_marker` 节点（零 LLM），不是真人 chat 轮。
- 既有草稿在 RunRoot 外（`%LOCALAPPDATA%\Vit\ProjectHistory\drafts`）仅记录 warning，不改写用户草稿。

## 结论

- 422 done；B2b 跨工作树真栈组合验证收口。
- 下一首排：432 W02-A1b（有界课程上下文注入，先定 prompt 路径）/ A1c（真实栈选中 clip + LLM 反馈）；用户节点 390/384/材料听核按用户时间。
