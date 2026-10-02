# WEBUI-IA-REDESIGN-1：webui 信息架构一期——删 DAW 简易视窗三件套+会话流侧边栏（codex 三区布局落成，P2）

- 池序 36；目标仓库=D:\Vit_DAW（agent/webui 域）；来源=[decisions/2026-10-02-webui-ia-vision.md](../../decisions/2026-10-02-webui-ia-vision.md)（用户裁定：删+三步走+独立控制台愿景）；**STATUSBAR-ID-1（池序 17）并入本卡一期执行**
- 优先级 / 预估 / 依赖：P2 / 1 天 / 无；webui 渲染域（与 chat 域/Godot 域在途卡不同域可并行；E2E 真栈排他注意与他卡错峰）
- 模型分级：L1 / GLM 首选（App.tsx 12k 行内大手术+E2E 断言组新建）
- 已核实事实（决策侧本轮亲核）：
  1. 简易视窗三件套=App.tsx 内 `daw-panel-body`（:2589 时间线/:2720/:2868 机架 lanes×2/:3244 MIDI workspace+`:3278 midi-editor-preview`）+daw-timeline/daw-toolbar/daw-panel-header/actions 一族；左面板有收起按钮（:2478）。
  2. `conversation-panel`/`history-rail-panel`/右面板（--right-panel-width）已在——三区骨架现成，改左列即可。
  3. webui 无会话流侧边栏；新建对话依赖历史界面分支/工作树（用户裁定解耦：侧边栏不建工作树）。
- 目标：
  1. **删三件套**：daw 时间线/机架 lanes/MIDI workspace 常驻镜像移除（连 state 同步与样式死码）。**删前三核对**：①测试/E2E 对这些组件的断言引用（同步更新）；②midi-editor-preview 是否被轨迹详情/证据链复用（复用则删常驻保组件，如实申报）；③demo-capture SOP 素材清单镜头引用（登记不阻塞）。
  2. **会话流侧边栏（左列）**：当前会话列表+新建/切换/重命名/归档；**不建工作树、不建分支**（会话键=conversation id；工程分支/工作树操作保留在历史界面=深操作）；note 流不进侧边栏（NOTESTREAM-2 契约：note 自含面板）；折叠 toggle（折叠态窄条）。
  3. **STATUSBAR-ID-1 并入**：执行状态栏哈希类会话 id 移除（需要区分占位则纯线性数字 1/2/3…，无前缀无哈希；完整 id 走 DOM/网络面板查）——原卡验收标准沿用。
  4. **布局收口**：三区=左（可折叠侧边栏）/中（conversation-panel 不动）/右（证据抽屉保留不动）；左列删除后中间对话流自然加宽。
  5. **回归与断言**：npm run test 全绿+tsc build+E2E 既有组全绿+**新增断言组**（侧边栏存在性/新建切换会话生效/简易视窗节点不再渲染/状态栏 id 形态）；前后截图对比入回执。
- 文件域：agent/webui/src/（App.tsx+组件/样式+测试/E2E 脚本）——实锚后申报具体清单。
- 约束：worktree 纪律（主工作树归决策侧）；E2E 真栈执行与他卡错峰+泊位声明；持久化兼容（AGENTS §11：会话列表形态 fail-open，旧数据加载零破坏）。
- 验收标准：目标 1-4 落地+E2E（含新增组）exit 0+用户目检复验（三区布局/侧边栏操作/视窗已删）。
- 二期钩子（登记不实施）：工程概览证据卡（TOM/ui-state 只读）+A/B 卡内嵌可播放渲染对比（decision 文件 §三步走）。
- 停止条件：删除发现组件被证据链深度耦合（删常驻仍破坏功能）→ 保留组件+如实申报耦合点上交；侧边栏与 conversationIdentity/historyScope 既有语义冲突 → 带锚点上交。
- 领取：2026-10-02 20:43 / origin/main=1ed78d7d / 分支 port/webui-ia-redesign-1（独立 worktree D:\Vit_DAW_wt_webui_ia_1，PC 执行侧 GLM-5.3）
- 回执：（commit hash / 删减行数 / 三核对结论 / E2E 结果 / 截图）
- 验收：（裁定文件 / 验收 commit）
