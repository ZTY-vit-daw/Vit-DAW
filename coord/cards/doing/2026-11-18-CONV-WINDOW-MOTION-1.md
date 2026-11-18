# CONV-WINDOW-MOTION-1：窗口切换动效调慢/分层过渡

- 池序 429；P1 用户手测反馈（2026-11-18 22:0x）：428 A 方案已不刷新，但切换动效“非常快非常不自然”；左侧列表与右侧内容几乎同时瞬切。
- owner=`lead/convrail418`（续用 428/B2a/B2b 上下文）；分支 `codex/shell-conv-motion-429`；worktree=`C:/Users/timoz/.codex/worktrees/pc429-motion/Vit_DAW`（主仓）；基线=`origin/main`。

## 目标（只改动效/过渡，不改结构）

1. **右侧 surface 流层过渡**：当前 200ms 全层 opacity+translateX 显得像两框同时换；改为 320–360ms（建议 340ms）`cubic-bezier(0.22,0.61,0.36,1)` 的缓出；inactive 层立即隐藏（避免双影），active 层只做轻淡入+8–12px 位移；不要出现两侧同时快速切换的观感。
2. **左侧列表内容过渡**：EntryShell 左栏槽内容在“启动页工程列表 ↔ 工程对话流”切换时加 220–260ms 淡入/轻移（与 180ms 收展区分）；同一时刻只动新内容，避免列表瞬切。
3. **工程内对话流切换**（分支预览/返回当前分支/继续后）：只动 `v5-project-thread` **内容体**（opacity 0→1 + translateY 6–8px，280–320ms），titlebar/composer/窗口壳保持静止；不重播加载卡。
4. **launcher 流内容切换**（greeting/结果卡等）如实现简单，同样只动内容体 280–320ms；否则保持现状并在回执说明。
5. reduced-motion 全部关闭；不引入 JS 定时器，纯 CSS transition/animation；避免动画期间布局跳动/双滚动条。

## 文件域（≤6）

`src/entry/conversationWindow.css`、`src/entry/ConversationWindow.tsx`（若需状态 class）、`src/entry/entryShell.css`（左栏内容过渡）、`src/project/project.css`（thread 内容过渡）、`src/project/ProjectSurface.tsx`（内容切换 class 驱动，若需）、`tests/render-window-presentation.mjs`；不动 Go/agent/内核/windowPresentation 协议/fixture-gallery 期望。

## 验收（零栈；亲跑原文）

1. `cd agent/webui && npm ci`、`npm run test`、`npm run build` exit 0；
2. `node tests/render-window-presentation.mjs` exit 0（既有 114 + 新增：右侧 layer 过渡 computed duration 320–360ms/easing 曲线；左栏内容过渡 220–260ms；工程 thread 内容过渡只作用于内容体、窗口壳/titlebar/composer 无过渡；reduced-motion 全部 none；无重复/双影 class）；
3. `npm run test:render` 全段 PASS；`git diff --check` exit 0；
4. 回执：commit、文件、命令+exit、新增断言原文、各过渡时长/缓动清单、边界（零真栈）。

## 停止

同确定性断点两次失败→停；若必须改结构（统一 titlebar/composer）才能消除右侧切换感，停手交主管讨论 A+。
