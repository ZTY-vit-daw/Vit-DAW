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
- 领取：（时间 / origin/main hash / 分支名）
- 回执：（commit hash / 复现方式 / 根因层 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
