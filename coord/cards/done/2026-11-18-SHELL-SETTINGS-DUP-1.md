# SHELL-SETTINGS-DUP-1：设置入口唯一化（只留左栏 footer）

- 池序 426；P1 缺陷修复；PC；发卡=本会话 PC 主管（lead）；用户 2026-11-18 20:31 手测反馈：工程页对话窗下仍有一个设置按钮，应在左侧列表底部栏且不应重复。
- owner=`lead/convrail418`（DeepSeek 执行流，B2a 原 owner 续用）；分支 `codex/shell-settings-dup-426`；worktree=`C:/Users/timoz/.codex/worktrees/pc426-settings/Vit_DAW`（主仓）；基线=`origin/main@9cd0fe4b`。

## 根因（已定位）

- `App.tsx` entry 分支同时给 `ConvergedLauncher` 与 `ProjectSurface` 传了 `onOpenSettings`，导致：
  - `ProjectSurface` 渲染自己的 footer `[data-project-settings]`（对话窗下方，用户看到的重复）；
  - `ConvergedLauncher` 抽屉在 rail 模式下仍渲染 `[data-launcher-settings]` 抽屉底部设置，与 EntryShell footer 重复。
- EntryShell 左栏 footer `[data-entry-rail-foot]` 已提供唯一设置入口（B2a 418）；应只保留它。

## 修复

1. App entry 分支：移除传给 `ConvergedLauncher`/`ProjectSurface` 的 `onOpenSettings`（`EntryShell` 的 footer 回调保留）；非 entry MainApp 的旧路径不动。
2. 渲染断言更新（`src/gallery/render-smoke.mjs` ENTRY388 段）：设置入口断言从 `[data-launcher-settings]`/`[data-project-settings]` 改为 `[data-entry-rail-foot] [data-entry-rail-settings]`；Escape 关闭后焦点归还该按钮；并新增「entry live 下 data-project-settings=0 且 data-launcher-settings=0」的唯一性断言。
3. `tests/render-window-presentation.mjs` 新增/强化：project surface 无重复设置按钮、footer 设置唯一。

## 文件域（≤4）

`agent/webui/src/App.tsx`、`agent/webui/src/gallery/render-smoke.mjs`、`agent/webui/tests/render-window-presentation.mjs`；无需改 ProjectSurface/ConvergedLauncher 结构（不传 prop 即不渲染）。不动 windowPresentation/agent/Go/内核/入口布局。

## 验收（零栈；亲跑原文）

1. `cd agent/webui && npm run test` 与 `npm run build` exit 0；
2. `node tests/render-window-presentation.mjs` exit 0（73+ 检查；新增唯一性断言）；
3. `npm run test:render` exit 0，ENTRY388 全绿且 fixture/gallery 不回归；
4. `git diff --check` exit 0；回执含设置断言原文、旧断言删除/替换清单、边界（零真栈）。

## 停止

同确定性断点两次失败→停；如需改 EntryShell footer 布局或 windowPresentation 协议→停报。

## 回执与验收（2026-11-18 晚，用户手测缺陷）

- commit `479897dc`；主管亲跑 746 测/build/RWP 75 检查 0 失败/test:render ENTRY388+全段 PASS；合入 main `65142940`。dist 重建为 `A14BE870…`。
- 修复：live 入口唯一设置入口=左栏 footer；工程页/启动页旧 footer 按钮移除；新增唯一性断言。
- 裁定：[rulings/2026-11-18-SHELL-SETTINGS-DUP-1-pass.md](../../rulings/2026-11-18-SHELL-SETTINGS-DUP-1-pass.md) PASS；待用户刷新验证。
