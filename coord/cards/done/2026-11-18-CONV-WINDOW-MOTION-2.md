# CONV-WINDOW-MOTION-2：窗口切换进一步放慢至 480ms 级

- 池序 430；P1 用户手测反馈（2026-11-18 22:1x）：429 仍“太快”，要求再慢；并询问业界常见时长。
- owner=`lead/convrail418`（续用 428/429）；分支 `codex/shell-conv-motion-430`；worktree=`C:/Users/timoz/.codex/worktrees/pc430-motion/Vit_DAW`；基线=`origin/main`。

## 参考（2026-11-18 查）

- Material Design 3：进入屏幕 400ms（Emphasized decelerate），离开 200ms；Material 1：移动端过渡通常 300ms，>400ms 可能显慢，但强调/大面转场可到 400–500ms。
- Apple HIG：常用 0.35s；复杂转场可更长。
- Nielsen Norman：简单反馈 100–200ms；内容/页面转场 200–500ms；复杂编排/页面切换 300–500ms。
- 结论：全窗口级切换落在 **400–500ms** 合理；用户仍觉得快，取上沿 **480ms**（不再超过 500ms，避免拖沓）。

## 目标

1. 右侧 surface 流层 active 入场：**480ms** + `cubic-bezier(0.05,0.7,0.1,1)`（emphasized decelerate），opacity 0.15→1 + translateX(12px)；inactive 立即 hidden。
2. 左栏内容：**320ms**（同曲线或 standard），与右侧错开（右侧延迟 40ms 起，避免同时动）。
3. 工程 thread 内容体：**420ms**（同曲线），只动内容体，壳/titlebar/composer 静止。
4. launcher 内容体：420ms；reduced-motion 全部关闭。
5. 动效值抽成 CSS 自定义属性（`--v5-conv-motion-layer/rail/content` + `--v5-conv-motion-ease`），后续调时长一处改；不引入 JS 定时器。

## 文件域（≤5）

`src/entry/conversationWindow.css`、`src/entry/entryShell.css`、`src/project/project.css`、`tests/render-window-presentation.mjs`（ProjectSurface.tsx 若已用 data 属性则无需改）；不动 Go/agent/内核/windowPresentation/fixture-gallery 期望。

## 验收（零栈；亲跑原文）

1. `cd agent/webui && npm ci`、`npm run test`、`npm run build` exit 0；
2. `node tests/render-window-presentation.mjs` exit 0（既有 124 + 更新 computed 断言：layer 480ms/emphasized decelerate、rail 320ms、thread 420ms、launcher 420ms、右侧延迟 40ms、reduced-motion none；无双影/无布局跳动）；
3. `npm run test:render` 全段 PASS；`git diff --check` 0；
4. 回执：commit、文件、命令+exit、更新后的时长/缓动/延迟清单、新增/替换断言原文、边界（零真栈）。

## 停止

同确定性断点两次失败→停；若需结构统一 frame（A+）才能改善观感，停手交主管讨论，不扩域。

## 回执与验收（2026-11-18 晚）

- commit `e2ce71ef`；主管亲跑 755 测/build/RWP 126 检查 0 失败/test:render 全段 PASS；合入 main `c7195e18`；dist=`1F517BCC…`。
- 动效：右侧 480ms（+40ms 错峰）/左栏 320ms/内容体 420ms；token 化一处可调；reduced-motion 全关。
- 边界：结构未变；若仍觉得快，下一步 ~550ms 或动效速度偏好。
- 裁定：[rulings/2026-11-18-CONV-WINDOW-MOTION-2-pass.md](../../rulings/2026-11-18-CONV-WINDOW-MOTION-2-pass.md) PASS；候手测。
