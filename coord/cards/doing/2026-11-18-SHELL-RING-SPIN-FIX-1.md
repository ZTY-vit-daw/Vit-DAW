# SHELL-RING-SPIN-FIX-1：状态环原地自转修复（加载卡 16px 轴心偏移）

- 池序 416；P2；PC；发卡/验收=本会话 PC 主管（lead）；派发确认=已确认（用户手测反馈：一定是原地自转才是加载动效的合理逻辑）。
- owner=`lead/ringfix416`（DeepSeek-V4.1-Flash，PC）；分支 `codex/shell-ring-spin-fix-416`；worktree=`C:/Users/timoz/.codex/worktrees/pc416-ringfix/vit-daw-frontend`；基线=前端 `origin/main@8c3801f`。
- 缺陷（用户手测 2026-11-18）：新建/加载工程卡片里的黄色状态环不是原地自转，而是绕一个轴点环绕。
- 根因（主管已定位）：`src/design/sprite.css` 的 `.v5-ring--idle/.v5-ring--busy` 旋转动画把 `transform-origin` 固定写为 `12px 12px`（24px 图标坐标），而加载卡 `StatusRing` 用 `size=16`（16×16 SVG），轴心应为中心 8px/8px → 偏 4px 形成绕轴环绕。另 `src/project/loading/loading.css` 对 `.v5-loading-chip .v5-icon` 又设置 `v5-loading-advance`（translateY）动画，与 ring spin 争夺同一 `animation` 属性，动效语义漂移。

## 修复要求

1. `.v5-ring--idle/.v5-ring--busy` 的旋转改为**几何中心**（如 `transform-origin: center`；必要时配 `transform-box`），确保 16/20/24 三档尺寸均原地自转。
2. 移除/收编 loading chip 的 translateY 覆盖：`v5-loading-advance` 动画及其 keyframes/reduced-motion 引用一并清理，让「进行中」按 SPEC 保持 `v5-ring-spin` 1.6s 快转；不得影响其他使用 `StatusRing` 的位置（trace/busy 环等一并复核）。
3. 保留 reduced-motion 全局禁用语义。
4. 在既有渲染装置（`agent/webui/tests/render-window-presentation.mjs` 或 `test:render` 既有段）增加 computed-style 回归断言：加载卡 pending 态下 `.v5-loading-chip .v5-ring` 的 `animation-name` 为 `v5-ring-spin`、`transform-origin` 落在元素中心（16px 档期望 8px 8px / 50% 50%），并断言不存在 translateY 覆盖；不新增测试机制。

## 文件域（仅）

`agent/webui/src/design/sprite.css`、`agent/webui/src/project/loading/loading.css`、`agent/webui/tests/render-window-presentation.mjs`（或既有 render 断言文件）。不动 React 组件、App、launcher/project 结构与主仓。

## 验收（无真栈）

1. `cd agent/webui && npm ci` exit 0；
2. `npm run test` exit 0；`npm run build` exit 0；
3. `node tests/render-window-presentation.mjs` exit 0（含新增 computed-style 断言，输出实际 animation-name/transform-origin 原文）；
4. `npm run test:render` exit 0；
5. `git diff --check` exit 0。

## 停止与回执

同确定性断点两次失败→停；需改 React 结构或新增测试机制→停报。回执：commit、文件、命令+exit、computed-style 原文、边界（零真栈；实际视觉待用户目检）。
