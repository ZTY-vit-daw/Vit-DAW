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

- 时间：2026-11-14
- origin/main hash：e854944621f6b9ec420bfd1cb9c2f71df64ede15（已 fetch 复核，344 基座 38798da7+fb50912f 在卷）
- owner：freeflash（GLM-5.3-Flash 免费车道子代理）· PC · ZCode 主管会话派发
- 分支：port/ui-launcher-1
- worktree：D:/Vit_DAW_wt_uilaunch1
- 领取时工作树状态：`git status --short` 仅本卡工件 PROGRESS.md（未跟踪），无其他改动
- 领取提交：1ea83f48（todo→doing，已推 main：e8549446..1ea83f48 fast-forward）

## 回执

- 实现分支：port/ui-launcher-1（基线 e8549446）；实现提交：c8582b35（M1–M3：surface+API+17 单测）、8cb87a67（M4/M5：渲染烟测+工件）、5bb87169（PROGRESS 终态）；执行侧自验完成日期 2026-11-14
- 命令退出码（agent/webui，2026-11-14）：
  - `npm run test` → 0（498 passed = 481 现有 + 17 新增 launcher 单测）
  - `npm run build` → 0（tsc --noEmit + vite build）
  - `npm run test:render` → 0（gallery RESULT=PASS + launcher LAUNCHER_RESULT=PASS）
- 截图/报告工件（分支 port/ui-launcher-1 在卷）：
  - coord/runs/UI-LAUNCHER-1/screens/launcher-viewport-1440x900.png
  - coord/runs/UI-LAUNCHER-1/screens/launcher-full.png
  - coord/runs/UI-LAUNCHER-1/render-smoke-launcher-report.json
  - coord/runs/UI-LAUNCHER-1/PROGRESS.md（全里程碑+命令记录）
- 分发冒烟（一次性临时脚本不入库；vite dev 代理指向关闭端口 127.0.0.1:9，零真实栈接触）：?surface=launcher → LauncherSurface 挂载 + agent 不可用 banner + 重试（.app-shell 不渲染）；?surface=main → .app-shell app-shell-main-surface 照常渲染；exit 0
- 端测覆盖边界声明：**浏览器渲染面已覆盖**——launcher DOM 断言（标题栏只写 Vit 文本且无 i-vit/工程标、抽屉真实 recent 数=声明数=DOM 行数、建议 3–4、输入框、加载/错误/空态、directive 恒 pending 占位不虚报、fixture 全标注）+ 正文≥4.5:1/图形≥3:1/字号≥13px/无裁切/零外部请求 + 1440×900 截图；surface 分发经浏览器冒烟；**真实栈端到端未覆盖**（真实 agent 进程 + 真实 recents 数据流），留 343/348；fixture 页全离线（零网络），fixture 工程名带「fixture」后缀自证
- 边界与遗留：
  1. directive/结果卡仅数据展示（「准备打开/新建…」占位），open/new 执行留 UI-BRIDGE-1；
  2. 344 目录工件（coord/runs/UI-FOUNDATION-1/{render-smoke-report.json, screens/gallery-full.png}）被本次共享烟测重生成呈 modified——344 域不提交不回滚，留验收侧处置；
  3. src/gallery/{main.tsx, gallery.css, render-smoke.mjs} 的最小扩展是卡面验收「launcher 渲染面进 test:render 底座」的直接要求；344 已验收断言与工件路径零改动（gallery 断言全绿即证）；
  4. App.tsx 仅 surface 分发最小改动（App→MainApp 更名 + 6 行 wrapper，full/main/workbench 行为零变化）；
  5. 完整最近工程列表经 launcher 聊天 list 意图（"有哪些工程"，确定性零 LLM）获取——greeting 端点只回执首项+计数，无独立全量 recents HTTP 端点。

## 验收

- 裁定文件 / 验收 commit：待主管决策流
