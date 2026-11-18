# SHELL-WIN-PRES-MERGE-1：395 呈现实现合入当前 main

- 池序 412；P1；PC；发卡/验收=本会话 PC 主管（lead）；派发确认=已确认（用户 2026-11-18 晚批准方案 A）。
- owner=`lead/presmerge412`（DeepSeek-V4.1-Flash 执行子代理，PC）；分支 `codex/shell-win-pres-412`；worktree=`C:/Users/timoz/.codex/worktrees/pc412-pres/Vit_DAW`（主仓）；基线=`origin/main@738291f6`。
- 单目标：把 `codex/shell-presentation-395` 的四个提交合入当前 main（`d2254f36` / `00f9572f` / `5bd0be40` / `bd72b925`），保持 5 生产文件域，恢复 722 单测 + 31 渲染检查 + 全 render 段通过。消费端协议不改（producer 由 410 按 `vitshell.launch_progress.v1` / `vitshell.window_state.v1` 提供）。

## 文件域（仅）

`agent/webui/src/App.tsx`、`agent/webui/src/launcher/ConvergedLauncher.tsx`、`agent/webui/src/project/ProjectSurface.tsx`、`agent/webui/src/project/loading/windowPresentation.tsx`、`.../windowPresentation.css`、`.../windowPresentation.test.tsx`、`agent/webui/tests/render-window-presentation.mjs`。冲突解决不得触及其他文件。

## 验收（全部亲跑并留原文）

1. `cd agent/webui && npm ci` exit 0（独立安装，不借用主检出 node_modules）。
2. `npm run test` exit 0（含新 9 例，期望 ≥722）。
3. `npm run build` exit 0。
4. `node tests/render-window-presentation.mjs` exit 0（31 检查）。
5. `npm run test:render` exit 0（各段 PASS）。
6. `git diff --check` exit 0。

## 边界与停止

不接真栈；不改断言/降标准；不伪造 producer——无真宿主时渲染脚本沿用既有 host mock，但回执必须声明；不得改 409/389 已验收面。冲突涉及 5 文件之外、渲染环境不可用（记录原因）、同确定性断点两次失败→停报。

## 回执（短）与禁止

commit hash+分支、文件清单、命令+exit 原文、测试/渲染计数与段清单、冲突文件与解决理由、边界声明（零真栈）。禁止 push main、改主检出、启 agent/Godot/VitShell、跨卡文件域。
