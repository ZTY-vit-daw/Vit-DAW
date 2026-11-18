# SHELL-ENTRY-RAIL-1（B1）：入口壳层 + 常驻左栏

- 池序 417；P1；PC；发卡/验收=本会话 PC 主管（lead）；派发确认=已确认（用户 2026-11-18 晚：B 方向，A 可跳过；先 B1 结构再 B2 内容）。
- owner=`lead/railb1`（DeepSeek-V4.1-Flash，PC）；分支 `codex/shell-entry-rail-417`；worktree=`C:/Users/timoz/.codex/worktrees/pc417-rail/Vit_DAW`（**主仓**，webui 在 Vit-DAW 主仓）；基线=`origin/main`。
- 目标：给 live 入口 App 加窗口级常驻左栏（280px，沿用现有 launcher drawer 视觉/收展），启动页工程列表迁入左栏；工程面先放「对话流（下一步接入）」占位。surface 切换时左栏容器保持、内容切换；gallery/fixture 路径不变。

## 实现约束（推荐方案，可等价替代但须保行为）

1. `src/App.tsx`：在 `WindowPresentationProvider` 内包一层 `EntryShell`（新组件）：横向布局 = 左栏（`data-entry-rail`，280px，可收起 180ms）+ 右主区（两个 hidden 切换的 surface div 原样放入）。左栏头 = 「Vit」品牌 + 当前面标题（启动页=工程列表；工程面=对话流）+ 收起/展开按钮（复用 WindowPresentation `sidebarOpen/setSidebarOpen`）+ `NativeFocusHint`。左栏体 = `data-entry-rail-slot` 门户容器。
2. 新建 `src/entry/`：`EntryShell.tsx`、`entryRail.tsx`（React context：`{host, surface, collapsed}`）、`entryShell.css`（280px、collapse 过渡、reduced-motion、主区 min-width:0）。
3. `ConvergedLauncher.tsx`：把现有 `<aside data-launcher-drawer>` 内容抽成同文件内组件（保持现有 props/状态/回调/收展行为），live 且有 rail host 时用 `createPortal` 渲染到 `data-entry-rail-slot`（仅 surface=launcher 且未收起）；fixture/gallery 无 rail context 时保持现有内嵌 drawer 与标题栏行为完全不变。live 模式下标题栏里的旧收起按钮隐藏（由左栏头接管），`NativeFocusHint` 保留不重复。
4. `ProjectSurface.tsx`：live 且有 rail host 时 portal 渲染工程面左栏占位（标题「对话流」+ 空态说明「对话流列表将在下一步接入」）；不改工程面主区/输入/历史行为。
5. 收展状态跨 surface 保持；surface 切换时左栏 DOM 容器不重建（同一条 `data-entry-rail` 节点），只切换内容（B2 再接动画）。

## 文件域（≤10）

`agent/webui/src/App.tsx`、`agent/webui/src/launcher/ConvergedLauncher.tsx`、`agent/webui/src/project/ProjectSurface.tsx`、新 `agent/webui/src/entry/*`、`agent/webui/src/styles/launcher.css`/`converged.css`（必要的 rail 样式泛化）、既有渲染断言文件（`src/gallery/render-smoke.mjs` 或 `tests/render-window-presentation.mjs` 的 APPENTRY/App 段）。不动 windowPresentation 协议、不新增测试机制、不改 fixture/gallery 期望。

## 验收（无真栈；亲跑原文）

1. `cd agent/webui && npm ci`、`npm run test`、`npm run build` 全 exit 0（722 测保持）；
2. `node tests/render-window-presentation.mjs` exit 0（35 检查保持）；
3. `npm run test:render` exit 0（全段 PASS；gallery/fixture 面不回归）；
4. 新增渲染断言（放既有装置）：live App 存在 `[data-entry-rail]` 且启动页含 `[data-launcher-projects]`；收起后 rail 宽度为 0 且主区可用；模拟 surface 切到 project 后同一 rail 节点仍在、出现工程面占位、启动页工程列表不再渲染；切回后恢复；
5. `git diff --check` exit 0。

## 停止与回执

同确定性断点两次失败→停；需改 windowPresentation/协议/内核/agent→停报。回执：commit、文件、命令+exit、渲染断言原文、fixture/gallery 未回归证据、边界（零真栈）。
