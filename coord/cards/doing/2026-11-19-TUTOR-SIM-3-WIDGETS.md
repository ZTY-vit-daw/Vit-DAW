# TUTOR-SIM-3：状态/进度卡 + 步骤输入卡

- 池序 437；P0；PC；发卡=lead（2026-11-19 22:52）；派发确认=用户「可以通过」+ 先前「继续开发控件」；验收负责人=lead。
- 前置：435/436 done；TUTOR-SIM-2 UI 自动开场手测 PASS。
- 目标：在 ProjectSurface 的 tutor sim 流程里加入两个**对话流内网页控件**，替代原始状态文本：
  1. **TutorProgressCard**：显示第 X/Y 步与 completed；数据来自 start/chat 响应的 workflow_data（step_index/step_total/completed），不是 LLM 文本、不是 eduJamRenderStatus 字符串。
  2. **TutorStepCard**：显示当前步骤问题（来自最近 assistant 回复/当前 step）和数字输入框 + 提交按钮；提交走现有 tutor conversation 的 /agent/chat；不推进时保持，通过后更新进度；completed 后禁用并显示完成。
- 约束：无命令词（开始乐局/写好了/乐局状态/乐局事件）；不显示 expect/答案；不改 server/实验 YAML；不做谱面/Tap/音频；普通工程/无 tutor 时零 UI 变化。
- 文件域：agent/webui/src/project/ProjectSurface.tsx、projectData.ts、projectService.ts（如需）、新组件 + 对应 test；不改 agent/Go。
- 验收：vitest——start ok 显示 1/3；错输入仍 1/3；7→2/3；42→3/3；123→completed/输入禁用；step card 提交带 tutor conversation；普通工程无卡片、无原始状态文本；npm run test + build exit 0。
- 停止：若必须改 server 响应/新增协议，停手上报；先在 UI 侧用已有 workflow_data 实现。

## 领取（2026-11-19 22:52）

- owner=`tutorui`（同线续用）；worktree `pc436-ui` / 分支 `codex/tutor-sim-2-ui-436`；基线 main `8d49cc49`（worktree 代码 ca65166d 已合入）。
- 只改 UI 文件域；不启栈、不改 server/实验 YAML；真栈手测由 lead 开窗。

