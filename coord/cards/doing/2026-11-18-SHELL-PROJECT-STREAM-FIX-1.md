# SHELL-PROJECT-STREAM-FIX-1：新对话可用 + 工程页消息与加载卡顺序

- 池序 427；P1 用户手测缺陷；PC；发卡=本会话 PC 主管（lead）；2026-11-18 20:45 用户反馈：
  1. 工程加载后左栏【＋ 新对话】灰色无法点击；
  2. 工程页右侧对话流里，加载好的工程卡固定在消息列表下方，发送的用户/agent 消息都显示在它上方（应作为历史记录的一部分，在消息之前）。
- owner=`lead/convrail418`（与 421 同一执行流，串行：先修 427 再续 421）；worktree 复用 `C:/Users/timoz/.codex/worktrees/pc421-cross/Vit_DAW`（分支 `codex/shell-conv-rail-cross-421`）；**427 单独 commit（含卡 ID）后再继续 421 改动**。

## 缺陷 A：新对话禁用

- 现状：`ConversationRail` 的 `newDisabled = busy || checkoutBlocked || !view.activeNodeId`；`ProjectSurface.startNewConversation` 在 `!conversationActiveNode` 时直接报错。新工程/历史尚无活动节点时按钮灰；且普通发送消息后没有刷新 `entryUIState`，左栏 active_node/树保持旧值。
- 修复：
  1. 无活动节点但有工程身份（project_path/active branch/head 可得）时允许新建：`version.branch_create` 只传 `{name}`（不传 from_node_id/commit_id），由当前 HEAD 分叉；有活动节点时保持现有 from_node_id+commit_id 行为。
  2. `ConversationRail` 禁用条件改为：busy / checkoutBlocked / 无工程身份；无活动节点时 hint 改“从当前工程状态新建对话”（不灰）。
  3. `ProjectSurface` 在发送消息完成后（replySettled 后）与工程上下文 loaded 后刷新 `onRefreshConversationState`，让左栏 active node/branches/新对话状态跟上；失败静默保留旧数据。

## 缺陷 B：加载卡在消息之后

- 现状：`ProjectSurface` 把 `<WindowLaunchFeedback/>`（LoadingCard/工程加载卡）渲染在 `state.thread.map(...)` 之后、thread 末尾，导致后续消息都排在它上面。
- 修复：把加载卡移到 `v5-project-thread` 内的**消息列表之前**（作为历史记录中的第一条/加载记录）；保留 `conversationPreview` 优先级（预览时仍显示只读路径）；确保同一时刻只有一个加载卡、无重复。若 launcher surface 的 `renderBridgeExec` 同样存在“加载卡在消息后”的顺序，一并按同一语义修正。
- 渲染断言：`[data-window-launch-feedback]` 必须在消息元素之前（DOM 顺序），发送后 user/vit 消息在其后；加载中仍在顶部可见，不遮挡输入。

## 文件域（≤6）

`agent/webui/src/entry/ConversationRail.tsx`、`src/entry/conversationData.ts`（如需无节点判断）、`src/project/ProjectSurface.tsx`、`src/launcher/ConvergedLauncher.tsx`（仅 launcher 顺序修正时）、`tests/render-window-presentation.mjs`；不动 Go/agent/内核/协议/fixture-gallery 期望。

## 验收（零栈；亲跑原文）

1. `cd agent/webui && npm run test`、`npm run build` exit 0；
2. `node tests/render-window-presentation.mjs` exit 0（既有 75 + 新增：无活动节点时新对话按钮 enabled；点击 mock `version.branch_create` 只带 name；有活动节点时仍带 from_node_id/commit_id；发送后刷新 uiState 被调用；工程页加载卡在消息之前，发送后消息在其后）；
3. `npm run test:render` 全段 PASS；`git diff --check` exit 0；
4. 回执：commit（卡 ID）、文件、命令+exit、新增断言原文、mock invoke 记录、边界（零真栈）；完成后继续 421（同一 worktree 后续 commit）。

## 停止

同确定性断点两次失败→停；需改 agent/Go/内核/协议→停报。
