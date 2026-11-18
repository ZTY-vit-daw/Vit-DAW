# CONV-WINDOW-SHELL-1（A 方案）：右侧对话窗口壳持久化 + 流内容切换

- 池序 428；P1；PC；发卡/验收=本会话 PC 主管（lead）；用户 2026-11-18 晚确认：先试 A 方案——工程启动/切换时对话流窗口不应“刷新”，应窗口壳不动、内容过渡；左侧列表切换。
- owner=`lead/convrail418`（续用 B2a/B2b/427 上下文）；分支 `codex/shell-conv-window-428`；worktree=`C:/Users/timoz/.codex/worktrees/pc428-window/Vit_DAW`（主仓）；基线=`origin/main`。

## 现状与目标

- 现状：B1 左栏已跨 surface 常驻同一 DOM，但右侧仍是两个独立 surface 组件：launcher 在 surface≠launcher 时卸载；project 首次打开时新挂载并拉 context/历史，故观感=窗口刷新。
- 目标（A V1）：右侧抽出一个常驻 `ConversationWindow` 宿主，launcher/project 作为两个**持久挂载的流层**；surface 切换/工程打开/返回时窗口壳与两层的 DOM 不重建，非活动层隐藏（inert/aria-hidden），活动层 180–240ms crossfade（reduced-motion 关闭）；保留滚动位置、焦点、输入草稿；工程加载/内核 reload 仍发生，但显示为流内加载/骨架，不清空窗口壳。
- **不在本卡范围**：不改 410/412 windowPresentation 协议；不重做统一标题栏/composer（层内沿用现有 frame）；不取消 `open_project` 单次真实 reload（B3 单窗常驻另议）。

## 实现要点

1. App entry：新增 `ConversationWindow`（entry 新文件 + css），包住 launcher/project 两个 surface；用 mounted 标记保证首次进入后**不因 surface 切换卸载**（hidden 而非条件渲染）；两层绝对定位叠放，active 层可见/可交互，inactive 层 inert+aria-hidden+pointer-events none。
2. 切换动效：active 层 opacity/轻微 translateX 过渡（180–240ms，符合既有 windowPresentation 收展时长语言）；reduced-motion 媒体查询关闭过渡；层切换不改变左栏 EntryShell 节点。
3. 工程加载：ProjectLoadingGate/WindowLaunchFeedback 在 project 层内正常显示；返回 launcher 再进 project 时不得重新播放/重复挂载加载卡（层已挂载，状态保留）；首次打开工程仍允许一次真实 context/历史加载（A 方案接受这一次），但窗口壳不重建。
4. 滚动/焦点/输入：切换 surface 时保留各层滚动位置与输入草稿；非活动层不抢焦点；返回后恢复原 scrollTop。
5. fixture/gallery 路径不变（无 EntryShell context 时保持现有渲染）；B1/B2a/B2b 的 101 条渲染断言不回归。

## 文件域（≤8）

`agent/webui/src/App.tsx`、`src/entry/EntryShell.tsx`（右主区挂载 ConversationWindow）、新 `src/entry/ConversationWindow.tsx`、新 `src/entry/conversationWindow.css`、必要时 `src/launcher/ConvergedLauncher.tsx`/`src/project/ProjectSurface.tsx` 的 hidden/inert 适配、`tests/render-window-presentation.mjs`；不动 Go/agent/内核/Godot/windowPresentation 协议/fixture-gallery 期望。

## 验收（零栈；亲跑原文）

1. `cd agent/webui && npm ci`、`npm run test`、`npm run build` exit 0；
2. `node tests/render-window-presentation.mjs` exit 0（101 既有 + 新增：右侧窗口壳跨 launcher→project→launcher 为同一 DOM 节点；launcher/project 层切换后仍挂载；active/inactive inert+aria-hidden；过渡 computed style 180–240ms 且 reduced-motion 关闭；返回后 scrollTop 保留；加载卡不重复挂载/不重播；左栏同一节点不变）；
3. `npm run test:render` 全段 PASS；`git diff --check` exit 0；
4. 回执：commit、文件、命令+exit、新增断言原文、是否保留层内 frame 的说明、边界（首次工程打开仍有一次真实加载；零真栈）。

## 停止

同确定性断点两次失败→停；需改 windowPresentation/agent/Go/内核→停报；若发现必须重做统一 frame 才能达到观感，先交回主管裁定是否升级 B 方案。

## 回执与验收（2026-11-18 晚，A 方案）

- commit `4cb47032`；主管亲跑 755 测/build/RWP 114 检查 0 失败/test:render 全段 PASS；合入 main `7b815d57`；dist=`7C30A2B6…`。
- 交付：ConversationWindow 窗口壳 + 持久流层；管理视图往返不重建、滚动/草稿/加载卡保留；200ms crossfade/reduced-motion 关闭；114 渲染断言语义。
- 边界：首次 launcher→project 仍按 2026-11-15 旧裁定卸载 launcher 层一次（改该裁定需用户确认）；首次工程打开仍有一次真实加载；层内 frame 未统一（B3 另议）。
- 裁定：[rulings/2026-11-18-CONV-WINDOW-SHELL-1-pass.md](../../rulings/2026-11-18-CONV-WINDOW-SHELL-1-pass.md) PASS；候用户手测观感。
