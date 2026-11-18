# SHELL-PROJECT-STREAM-FIX-1（427）验收：PASS（零栈；用户手测缺陷）

- 卡：`2026-11-18-SHELL-PROJECT-STREAM-FIX-1`（池序 427）；主管=lead；执行=lead/convrail418。
- commit `30449b90`（5 文件 +152/−20）；已合入 main `c0a3ca6f`（后与 421 一起到 `7cea611f`）。

## 修复

- **新对话可用**：禁用条件从 `!activeNodeId` 改为 `!hasProjectIdentity`（busy/checkout_blocked 仍禁用）；无活动节点但工程身份可得时 `version.branch_create` 只传 `{name}`（当前 HEAD 分叉）；发送完成/上下文 loaded 后刷新左栏 uiState。
- **加载卡顺序**：ProjectSurface 的 `WindowLaunchFeedback`（装载记录卡）移到消息列表之前；launcher 同语义；同一时刻一张卡。

## 主管复核（亲跑，含 421 最终树）

| 项 | 结果 |
|---|---|
| `npm run test` | exit 0；63 files / **755 tests** |
| `npm run build` | exit 0 |
| `node tests/render-window-presentation.mjs` | exit 0；**101 checks / 0 failed** |
| `npm run test:render` | exit 0；ENTRY388 + 全段 PASS |
| 新增断言 | 无节点新对话 enabled、branch_create 仅 name、有节点仍带 from_node_id/commit_id、发送后刷新、加载卡在消息前且不遮 composer |

## 边界

- 零真栈；真实手测需刷新/重启入口。
- 无活动节点从当前 HEAD 建分支的语义由 agent `version_branch_create` 既有行为保证（branch_create commit_id 缺省=currentHead）；421 已包含该路径。
