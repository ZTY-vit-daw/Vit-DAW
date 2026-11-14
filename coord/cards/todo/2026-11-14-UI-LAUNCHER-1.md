# UI-LAUNCHER-1：P1 启动台 surface（React）

- 发卡：主设计会话（lead）/ 2026-11-14（用户指令：新 UI 设计任务进池）
- 派发确认：已确认（用户 2026-11-14 指令；主管可调整）
- 验收负责人：用户 + 主管决策流
- 池序 345；P1 / 2–3 天 / 依赖=344（基座）；WebUI；浏览器可开发，端到端待 343/348

## 目标

1. 新增 `?surface=launcher`（或等价路由）的启动台 surface：左侧工程抽屉（真实最近工程）、问候语、输入框、3–4 个上下文建议；标题栏只写 "Vit" 文本（V 不进入标题栏）。
2. 消费真实 `/agent/launcher/greeting` 与 launcher 聊天（`context.launcher=true`）；渲染 launcher 结果卡/directive 为**数据展示**，不自行执行 open/new。
3. 保留现有 `full/main/workbench` surface 行为；不改工程内对话、设置、历史树。
4. 空态/加载/agent 不可用/recents 失败要有明确文案与重试；不硬编码示例工程名。

## 文件域

`agent/webui/src/launcher/**`（新增）、`agent/webui/src/api/launcher.ts`（新增）、`agent/webui/src/styles/launcher.css`（新增）、`agent/webui/src/App.tsx`（仅 surface 分发最小改动）；不改工程内 `src/project/**`、`styles.css` 业务规则、agent/Godot。

## 并行与资源

不占真实栈；浏览器渲染烟测（344 的底座）。与 343 文件域不相交；App.tsx 在 345 期间由本卡持有，346 需等本卡合入后再改。

## 验收标准

- `npm run test` / `npm run build` / `npm run test:render` 退出 0；launcher gallery DOM：标题、抽屉、真实 recent 数、建议、输入、加载/错误态。
- 用真实 agent 端点或明确 fixture；fixture 需在卡内标注，不冒充生产数据。
- 无 open/new 执行；directive 仅显示“准备打开…”等占位，不虚报已打开。
- 现有 surface 无回归（至少主对话 surface 冒烟）。

## 停止条件

- 需要 `UI-BRIDGE-1` 才能继续 open/new → 停，不伪造打开；交桥卡。
- App.tsx 冲突/需要工程内组件改动 → 停，交主管拆序。

## 领取

- 时间 / origin/main hash / owner 模型+机器+会话 / 分支 / worktree / 领取提交：待回填

## 回执

- commit hash / 命令退出码 / 截图路径 / 端测覆盖边界声明：待回填

## 验收

- 裁定文件 / 验收 commit：待回填
