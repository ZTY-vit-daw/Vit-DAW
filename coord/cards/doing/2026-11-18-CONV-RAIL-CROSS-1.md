# CONV-RAIL-CROSS-1（B2b）：跨工作树只读浏览 + 打开工作树并继续对话

- 池序 421；P1；PC；发卡/验收=本会话 PC 主管（lead）；派发确认=已确认（用户 2026-11-18 晚确认 B 方向与交互模型；419 组合工具已合入 main 8d9b8f4e，421 等 426 合入后发卡）。
- owner=`lead/convrail418`（续用 B2a/426 上下文）；分支 `codex/shell-conv-rail-cross-421`；worktree=`C:/Users/timoz/.codex/worktrees/pc421-cross/Vit_DAW`（主仓）；基线=`origin/main@22a50f65`。
- 目标：左栏从“当前工作树”升级为**工作区→工作树→分支**：其他工作树只读浏览（懒加载 `version.status`）、分支归属过滤、只读预览+输入蒙版、动作卡【打开此工作树并继续此对话】→ 组合工具 `version.conversation_open`（单次工程重载）；当前工作树仍用 B2a 的【继续此对话】。

## 数据与交互（已核）

- 工作树列表：`entryUIState.project_history.worktrees`（`historyWorktreeRows` 现有纯函数；行含 name/project_file_path/head 等，实现前先读实际字段）。
- 其他工作树展开时懒加载：`/agent/invoke` 调 `version.status`，`{project_path: worktree.project_file_path, confirmed:false}`（只读，不调 checkout、不 open_project、不重载 DAW）；结果含 branches/conversation_graph/active_branch/active_node_id；按 worktree 缓存，展开一次只取一次（可手动刷新）。
- **分支归属过滤（B2a 边界收口）**：同一分支只出现在一个工作树组下——优先归“该分支为 active_branch”的工作树；无 active owner 时归 head commit==worktree head 的工作树；仍无法归属的分支进工作区级「其他分支」组（只读）。当前工作树组同样过滤掉已归属其它工作树的 active 分支，避免重复。
- 预览：复用 B2a 的 branch head→parent 链只读消息与蒙版；浏览不触发任何写操作；点击当前工作树分支=现有【继续此对话】（`version.checkout`）；点击其它工作树分支=【打开此工作树并继续此对话】。
- 跨工作树继续：`window.confirm` 后调 `version.conversation_open`，参数 `{project_file_path: 目标工作树副本路径, branch: 目标分支, node_id: 该分支 head 节点 id, confirmed:true}`（传 branch+head 节点可走非 detached 分支路径，419 已证）；loading 中禁用重复；成功后 `fetchUIState` 刷新，左栏切到新 active 工作树/分支并清预览。
- 失败：status 拉取失败→该组显示“无法读取”+重试；conversation_open 失败→保留预览+原因+可重试，不自动重试；`target_materialized=true` 等披露字段如实展示；turn 运行中/`checkout_blocked` 禁用并说明。

## 文件域（≤10）

`agent/webui/src/App.tsx`（entryUIState/刷新注入）、`src/entry/ConversationRail.tsx`、`src/entry/conversationData.ts`(+test)、`src/entry/conversationRail.css`、`src/project/ProjectSurface.tsx`（动作卡文案/蒙版复用）、`src/lib/api.ts`（如需 status invoke helper 复用）、`tests/render-window-presentation.mjs`；可选新增 `src/entry/conversationCross.test.ts`。**不动** Go/agent/内核/Godot、windowPresentation 协议、fixture/gallery 期望。

## 验收（零真栈；亲跑原文）

1. `cd agent/webui && npm ci`、`npm run test`（746 基线+新增纯函数/交叉测试）、`npm run build` exit 0；
2. `node tests/render-window-presentation.mjs` exit 0（既有 75 + 新增：其它工作树组展开/懒加载只取一次/只读预览无写调用/动作卡文案与 confirm/invoke 参数精确/loading 禁用/失败保留预览+原因/成功后刷新并高亮新 active/分支归属唯一性/checkout_blocked 守卫）；
3. `npm run test:render` exit 0（全段 PASS，fixture/gallery 零回归）；`git diff --check` exit 0；
4. 回执给出新增断言原文、mock `/agent/invoke` 调用记录（version.status 与 version.conversation_open 实参）、归属规则测试、边界（零真栈；真实栈组合候 422）。

## 停止

同确定性断点两次失败→停；需改 agent/Go/内核/协议→停报；真实栈行为留 422。
