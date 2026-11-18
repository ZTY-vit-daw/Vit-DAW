# CONV-WINDOW-MOTION-3：左侧内容切换单独拉到 800ms（试验）

- 池序 431；P1 用户手测反馈（2026-11-18 22:1x）：重点在左侧列表内容切换速度，要求先加到 800ms 试试。
- owner=`lead/convrail418`；分支 `codex/shell-conv-motion-431`；worktree=`C:/Users/timoz/.codex/worktrees/pc431-motion/Vit_DAW`；基线=`origin/main`。

## 目标（最小改动，只调左栏）

1. `--v5-conv-motion-rail` 由 320ms 改为 **800ms**（只影响 EntryShell 左栏槽内容：启动页工程列表 ↔ 工程对话流）；
2. 其余保持 430 值：右侧 layer 480ms + 40ms 错峰、内容体 420ms、缓动 `cubic-bezier(0.05,0.7,0.1,1)`、reduced-motion 全关；
3. 同步更新 `tests/render-window-presentation.mjs` 中 rail 内容 computed 断言为 800ms；
4. 不改结构/不动 windowPresentation/agent/Go/内核/fixture-gallery 期望。

## 验收（零栈；亲跑原文）

1. `cd agent/webui && npm ci`、`npm run test`、`npm run build` exit 0；
2. `node tests/render-window-presentation.mjs` exit 0（126 checks 基础上把 rail 内容断言更新为 800ms；其余不变）；
3. `npm run test:render` 全段 PASS；`git diff --check` 0；
4. 回执：commit/文件/命令+exit/更新断言原文/边界（零真栈）。

## 说明

- 800ms 已超出常规 200–500ms 区间，用户明确要求先试；只作为试验值，后续可按手测回调 token。

## 回执与验收（2026-11-18 晚，试验值）

- commit `4d9190d2`；主管亲跑 755 测/build/RWP 126 检查 0 失败；合入 main `4dee0d62`；dist=`FA293DB3…`。
- `--v5-conv-motion-rail`=800ms（仅左栏内容）；其余 430 值不变。
- 裁定：[rulings/2026-11-18-CONV-WINDOW-MOTION-3-pass.md](../../rulings/2026-11-18-CONV-WINDOW-MOTION-3-pass.md) PASS；候手测回调。
