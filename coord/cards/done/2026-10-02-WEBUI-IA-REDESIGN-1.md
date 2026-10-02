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
- 回执：实现 commit=**6501e051**（port/webui-ia-redesign-1，9 文件 +1434/−2861；领取提交 8c3ff037 在 main）
  - **删减（目标 1）**：App.tsx 净 −1654 行——DawFocusPanel 壳+DawTracksPanel（时间线）+DawRackPanelV2（机架 lanes）+DawMidiPanel（MIDI workspace+midi-editor-preview）+focus 体系（FocusMode/focusItems/SideRail 轨/focusTitle/FocusSummary/handleFocusChange）+死码 helper 族（rack/midi/mixer/clip-drag 一族）；styles.css 死族 −1291 行（153 规则）+侧边栏新样式；dawTracksFromUIState/DawTrack 模型族保留（确认卡/工程结果卡共用，实锚 ：4059/:6288 等消费者）。
  - **三核对结论**：① E2E/单测对三件套**零断言引用**（webui_rendered_dom_smoke.mjs 内 9 处 "daw" 均为 `item_type:"daw_action"` 消息事件 fixture，与视窗组件无关）→ 既有断言无需同步；② midi-editor-preview 全仓唯一引用=App.tsx:3278 自身，轨迹详情/证据链**零复用** → 整体删除（未触发"删常驻保组件"停止条件）；③ demo-capture SOP 素材清单无三件套镜头引用 → 登记不阻塞。
  - **同族发现（如实申报）**：`daw-panel-body` 家族实为四件+死码——:3427 mixer-strips（DawMixerPanel，per-track 推子镜像；decision §能力边界裁定"手工细调归 DAW GUI"，同属低配 DAW 镜像族）与 :2717 DawRackPanel **V1（无调用点死码）**随三件套一并删除；focus 切换体系整体下线是目标 4（中=conversation-panel 常驻）的必要条件。若决策侧裁定 mixer 应保留，revert 该组件族即独立可回滚（同一提交内边界清晰）。
  - **会话流侧边栏（目标 2）**：新增 SessionFlowSidebar.tsx+sessionFlow.ts（注册表）+sessionFlow.test.ts（14 例）。新建=既有 handleNewConversation 轻量通路（webui_ id）；切换=同语义镜像（scope 暂停+锚定改写+瞬态清空；消息由既有恢复效应自本地桶水合、轨迹由 [conversationID] 效应重放）；重命名=inline input（Enter/Esc）；归档+还原；折叠窄条 toggle（localStorage）；**不建工作树不建分支**（E2E note: "new conversation webui_mur0m1vm registered (no worktree, no branch)"）；note 流零接触；编号=纯线性数字 1/2/3…，完整会话 id 不上 UI（行节点 data-conversation-id 属性=DOM 查询面）。注册表 fail-open：损坏 JSON/异形行丢弃不阻塞加载（AGENTS §11，单测钉）。
  - **STATUSBAR-ID-1（目标 3，并卡）**：PlanBar `Task {taskID}` 可见文本移除；data-task-id 属性保留（调试走 DOM/网络面板）；单任务窄条无区分占位需求，按裁定默认移除式交付；PlanBar.test.tsx 两处断言同步更新（可见文本→属性形态）。
  - **E2E（目标 5）**：新增三组——**ia-redesign-sidebar-IA1**（存在性/线性编号/新建/重命名/切换回原会话/归档还原/折叠展开全操作链，真实点击驱动）、**ia-redesign-viewport-IA2**（12 个死族选择器含 .side-rail/.rail-button 全 0）、**ia-redesign-statusbar-IA3**（plan-bar 头无可见 Task 文本+data-task-id="task_e2e_planbar1" 在）。**RED**（HEAD bundle，run root `artifacts/e2e_webui1/webui-ia-redesign-1-red`）：既有 24 组全绿+唯三新组红（侧边栏未渲染 / .side-rail×1+.rail-button×7 在场 / "Task task_e2e_planbar1" 可见+.plan-bar-task 元素在）——证明新组真能抓住旧形态。**GREEN**（新 bundle，`…-green2`）：**28 组全绿 verdict=pass，脚本 exit 0**。（green 首轮 IA3 误判：采样位在会话切换后，而 [conversationID] 效应清任务轨迹是合法行为——采样位移至挂载后复跑 green2 通过；red/green/green2 三 run root 互不覆盖留档，prereq.txt 含 head/二进制 SHA256/dist 哈希/agent_pid/桥端口全量。）
  - **泊位声明（与他卡错峰）**：HTTP 7899（每轮验空）；桥端口 VIT_AGENT_* 覆盖 5601/5602/4500/4501；agent=工作树构建 VitAgent.ia1.exe（SHA256 在案，agent Go 代码零改动）；draft root=各 run 目录内 agent_drafts；agent 进程每轮 stopped（pid+port_released=7899 在案）；全程未触碰主工作树 D:\Vit_DAW 与其他会话真栈。
  - **截图（前后对比）**：前=`…-red/dom-ia-redesign-mount.png`（旧 SideRail 轨+Task id 可见）；后=`…-green2/dom-ia-redesign-mount.png` / `-ops.png`（侧边栏操作后形态）/ `-collapsed.png`（折叠窄条）。
  - **回归**：npm run test **413/413 全绿**（含 sessionFlow 14 新例）；npm run build（tsc --noEmit+vite）**exit 0**。
  - **运行栈声明**：本卡 E2E 泊位栈（隔离 VitAgent+headless 浏览器）每轮已拆（stopped_agent_pid+port_released 在案）；真实三件套栈（VitApp 内核/Godot 前端）本卡**未启动、无移交**。
  - **端测覆盖边界（AGENTS §5）**：渲染面已过 E2E-WEBUI-1 全量 28 组（浏览器级 DOM 断言+真实交互）；用户目检复验（三区布局/侧边栏操作/视窗已删）留待决策侧安排——验收标准第三项未在本卡闭合。
- 验收：**conditional pass（2026-10-02 决策会话；用户目检复验留待非阻塞）**——[rulings/2026-10-02-WEBUI-IA-REDESIGN-1-conditional.md](../../rulings/2026-10-02-WEBUI-IA-REDESIGN-1-conditional.md)；删除边界+同族超额采信+侧边栏不建树契约+E2E RED 恰三新组红/GREEN2 28-28 工件亲读+我方复跑（413/413+build）+三核对采信；cherry-pick 6501e051=f6640d42 合 main；STATUSBAR-ID-1 随卡关闭
