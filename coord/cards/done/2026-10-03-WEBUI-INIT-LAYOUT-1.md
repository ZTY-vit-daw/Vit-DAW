# WEBUI-INIT-LAYOUT-1：webui 初启布局比例挤压——一次对话后自行展开（P2）

- 池序 11；目标仓库=D:\Vit_DAW（agent/webui）；来源=[decisions/2026-10-03-user-manual-test-feedback.md](../../decisions/2026-10-03-user-manual-test-feedback.md) 问题 5；**截图证据**：[runs/MANUAL-TEST-20261003/webui_initial_layout_squeezed.png](../../runs/MANUAL-TEST-20261003/webui_initial_layout_squeezed.png)（用户 10-03 09:30:12 截屏：三区+侧边栏全部比例缩在一起；一轮对话后自行展开，原因未明）
- 优先级 / 预估 / 依赖：P2 / 0.25 天 / IA 一期布局已合 main
- 模型分级：L0/L1 / flash 可接
- 已核实事实：截图在案——初启时整体挤压（疑似容器宽度/flex 基准未初始化或首次渲染用了降级宽度，一轮消息触发重排后恢复）；"对话后自行展开"提示状态驱动的样式切换或测量时序问题。
- 目标：
  1. 复现：dev 起服+浏览器首开（或 E2E headless 首屏截图断言）复现初启挤压形态；定位是 CSS 初始值/首帧测量/条件类名哪一层。
  2. 修复：初启即呈现与对话后一致的展开布局。
  3. E2E：webui_rendered_dom_smoke 加首屏布局断言组（初启主区/侧边栏宽度比例在阈值内）。
- 文件域：agent/webui/src/ 布局/样式段 + scripts/webui_rendered_dom_smoke.mjs（加组）。
- 验收标准：npm test 全绿+E2E 新组 exit 0+用户目检复验（初启即展开）。
- 停止条件：headless 无法复现初启形态（首帧依赖真实浏览器测量）→ 以有头截图取证+人工验证收口，如实申报端测边界。
- 领取：2026-10-03 18:14（PC 执行侧 flash 会话）/ origin/main c7f55d0d06787de0c453fa7a14b160b01aa51c51 / 分支 port/webui-init-layout-1。领取时工作树既有改动：`VitApp/Workspace/default_project.xml`（M，运行时状态，非本卡域，不触碰）、`.zcodeignore` 与 coord/runs/SETTLE-DELIVER-SMOKE-*（??，不触碰）。
- 回执：
  - **commit**：实现 `65343834` @ `port/webui-init-layout-1`（已推送，待决策验收 cherry-pick 合 main）；改动=agent/webui/src/styles.css（2 处 grid-template-rows）+scripts/webui_rendered_dom_smoke.mjs（+141 行 L1 组）；dist 未入库（本地重建 18:20 与烟测 18:26 各一次）。
  - **复现方式**：headless 可完整复现，无需有头兜底——静态服务 `agent/webui/dist` + playwright-core/msedge 三视口（1089×578@1.75=用户截图推算视口 / 1270×674@1.5 / 1600×900@1），修复前 workspace bottom≠视口底（463.3/495.3/495.3），挤压形态与用户截图逐区吻合（侧栏 196px 媒体查询命中/右 rail 折叠/composer 悬浮中部/底部死区=app-shell 米色底）；机制注入验证：对话内容增长→auto 行撑大至满窗="一轮对话后自行展开"的实证。证据：`coord/runs/WEBUI-INIT-LAYOUT-1/`（RUN_LOG.md+复现脚本+RED 抽验+烟测工件副本 smoke_20261003_182628/）。
  - **根因层**：**CSS 初始值层**（非首帧测量、非条件类名）——`.app-shell` 声明 3 行网格 `auto auto minmax(0,1fr)` 但自 8df48982（2026-09-03 副注条移除）起仅 2 个子元素：空的第 3 行吃掉全部 1fr 剩余空间，main-workspace 滑入 auto 行随内容收缩（初启 ~423px），窗口越矮越显著（用户 175% DPI 视口 ~1089×578）；8df48982 回执"空 auto 行 0px 无害"的几何实证只验了 composer 在视口内（恒真），未验不满窗。修复：两处改 2 行+约束注释（行数=子元素数）。
  - **端测边界声明**：headless 复现充分（RED 抽验：addStyleTag 注入旧 3 行值→L1 组红出 tracks 3≠2 children + fill slack 436.7px 双钉，计算值与修复前逐字节一致）；缺陷与修复均为纯 webui 布局层，零 agent 侧改动。**用户目检复验（初启即展开）按卡留待用户**。
  - **门**：`npm run test` 36 文件 420/420 绿（18:26）；`scripts/run_webui_rendered_dom_smoke.ps1` 真栈（隔离 agent pid=7628@7897，工作树含本卡 diff）run root `D:\Vit_DAW\artifacts\e2e_webui1\20261003_182628` **smoke_exit=0 verdict=pass failed_groups=空**：新增 `initial-layout-L1` PASS（tracks[72px 828px]=2children 1:1/workspace bottom=900==innerHeight/侧栏满高/composer 坐底 slack 22px/右面板贴边 right=1600），既有组（IA1-IA3/msg-order-M1-M2/msgrevive-H1/bucket-save-race-F1 等）全 PASS。
- 验收：（裁定文件 / 验收 commit）
