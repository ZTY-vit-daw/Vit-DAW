# WEBUI-SESSION-SEMANTICS-1：主对话流语义——webui 启动自动建主会话+命名 / 执行轨迹随会话切换清空（P2）

- 池序 9；目标仓库=D:\Vit_DAW（agent/webui）；来源=[decisions/2026-10-03-user-manual-test-feedback.md](../../decisions/2026-10-03-user-manual-test-feedback.md) 问题 4+6；与 NOTESTREAM-2（note 副会话线）同族不同文件域，注意避让 webui 渲染段
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / webui IA 一期侧边栏已合 main（SessionFlowSidebar/sessionFlow 注册表）
- 模型分级：L1 / GLM 或 flash 可接
- 已核实事实：
  1. 现状：webui 打开后不主动新建会话，用户落第一条消息才落到某会话（或沿旧会话）；用户裁定产品逻辑=**note 流是副（子代理），webui 流是主**，主会话应启动即建并命名（或留白给用户命名——实现取轻：默认命名+可改名）。
  2. 症状 6（轨迹不随会话切换）：note 中输入 hi 后，webui 新建对话流继续工作，输入框上方执行轨迹仍显示 hi——轨迹面板未随 active session 切换清空/重绑。IA 一期 E2E 断言过"会话切换后任务轨迹清空是合法行为"（green 首轮 IA3 采样位修正记录在案），说明机制存在但 note→webui 新建流场景未触发（跨会话类型 or 事件时序）。
  3. **手测三号场实锚（2026-10-03 晚）**：用户见 webui 侧边栏三条未命名对话流（本栈主流+疑似跨栈遗留条目+便签 chat_ff99… 流）——跨栈遗留累积形态待本卡取证确证（localStorage 注册表生命周期）；[FORENSIC-3](../../runs/MANUAL-TEST3-20261003/FORENSIC.md)。
  4. **命名协议定稿（用户裁定 2026-10-03，[decisions/2026-10-03-session-naming-and-scope-context.md](../../decisions/2026-10-03-session-naming-and-scope-context.md)）**：主对话流默认名=「主对话流 · <工程名>」（每次拉栈自动建、同工程重开沿旧名；出生即命名、用户改名最高且持久；LLM 增强二期不做）。
- 目标：
  1. 主对话流：webui 启动（或工程打开）自动创建主会话，默认命名（协议与 NOTESTREAM-2 命名协议同族，如「主对话流」+日期/序号），侧边栏可见、可改名；用户后续新建的会话平级列出。
  2. 轨迹绑定：执行轨迹/PlanBar 数据源随 active session 切换即时清空或重绑对应会话轨迹；补 webui 测试（切换会话→轨迹清空/换绑断言）+E2E 组扩展（note 会话→webui 新建会话场景）。
  3. 旧消息/旧会话兼容（AGENTS §11）：注册表 fail-open 既有语义零回退。
- 文件域：agent/webui/src/（SessionFlowSidebar/sessionFlow/App.tsx 会话初始化与轨迹绑定段）+测试。
- 约束：与 SETTLE-DELIVER-1 的 webui 渲染段改动如相遇，串行（SETTLE-DELIVER P1 先）；E2E 真栈排他（泊位）。
- 验收标准：npm test 全绿+新用例（主会话自动建+轨迹切换）+E2E-WEBUI-1 扩组 exit 0+用户目检复验。
- 停止条件：主会话语义需 agent 侧会话接口支持而现接口缺失 → 实锚清单上交定方案。
- 领取：2026-10-04 10:18 / origin/main=d2862b59 / 分支 port/webui-session-semantics-1 / **worktree=D:\Vit_DAW_wt_sess_sem_1**（主树被他卡 REGION-OP-RECON-1 占用，按协议独立 worktree 开工）
- 回执：
  - **实现 commit=45840390**（port/webui-session-semantics-1，已推远端；8 文件 +548/-10：App.tsx/SessionFlowSidebar.tsx/sessionFlow.ts(.test)/taskTrajectory.ts(.test)/composer/PlanBar.test.tsx/scripts/webui_rendered_dom_smoke.mjs）
  - **主会话命名协议（落地面）**：`sessionFlow.ts` 新增每 scope 主会话身份键 `ask_vit_session_main.v1:<scope-slug>`（fail-open，与注册表同语义：损坏/无 window 读空、空 id 不吞旧值、写失败 best-effort）+确定性模板 `mainConversationDefaultTitle`——「主对话流 · <工程名>」（工程名=scope 工程路径尾段，未保存退化「主对话流」）；App 在 scope 物化三分支（initial/switch、evolution 采纳、evolution 迁移）**首见即钉**主会话 id（幂等：已钉沿旧身份=同工程重开沿旧名；新建/切换会话只改写既有锚定、不触碰主身份）；`SessionFlowSidebar` 对主会话行幂等登记默认名——**upsert 只填未命名行：用户改名最高且持久、归档态零覆写**；用户后续新建会话不经此路径（平级列出，沿首条消息推导或「未命名会话」）。
  - **轨迹绑定锚点**：①`taskTrajectory.ts` 新增 `reduceTaskTrajectoryForConversation`——`/agent/runtime/status` 全局 `task_trajectory` 投影按 `task.conversation_id` 归属裁决（Go 侧载荷本就携带，chat/server.go taskRuntimeTrajectoryProjection；**无需 agent 改动，文件域守住 webui**）：异会话快照即时清空、载荷缺位保留现状（不算异会话证据）、monotonic 语义保持；②App `refreshState` 合并位换用之（useCallback 依赖补 conversationID）；③PlanBar 的 goal/plan 全局兜底与链活信号按会话归属门控（`planBarOwnLive=trajectoryLive||(agentTurnRunning&&属主快照在位)`）——异会话任务在跑时兜底置空，栏随快照清空**立即卸载**，不再以无快照兜底形态滞留或泄漏他流意图文本。修复过程自纠一个合成 bug：归约吃原始载荷（已归一化对象二次归一化得 null→恒空态），以插桩取证定位后修正。
  - **验证**：vitest 435/435 全绿（新增 10：会话绑定归约 4+主会话身份/命名 5+PlanBar 源码钉同步 1）；`npm run build` exit 0；**E2E-WEBUI-1 扩组 exit 0**——新增 SS1-SS3 断言组（SS1 启动即建+命名+主身份键持久+active；SS2 note 会话→切换清场+webui 新建对话流不复活+新流不被误名；SS3 切回属主重绑+用户改名跨 reload 持久），note 行经 `/agent/ui/state` 同源自校准注入，真 agent+Playwright **即起即拆**（隔离 7897 泊位，未占用户真栈），同版本连续两轮 verdict=pass：`artifacts/e2e_webui1/20261004_111745`、`20261004_112210`（worktree 内）。
  - **端测覆盖边界声明**：①全局 runtime 投影同一时刻只报一个 goal——并发双任务（主流+note 同时在跑）时属主快照可能被顶掉，主流 PlanBar 诚实清空而非显示错任务；按会话查询任务投影需 agent 侧端点扩展（越域，未做）；②注册表既有跨栈遗留未命名行**不回填命名**（fail-open 零回退），可手工改名/归档；全新浏览器上 URL 深链会话被视为该 scope 主会话（首见即钉）；③E2E note 行为网络层注入（PLANBAR-1 同模式），note 真链路（NOTESTREAM-2）不在此组覆盖；④[等待用户：真栈目检复验]——按手测途径（Godot 前端拉起）验证：启动后侧边栏出现「主对话流 · <工程名>」、note 输入后 webui 新建会话输入框上方不再残留旧轨迹。
  - **事故披露（已自纠）**：E2E 脚本首版误写入**主树** `scripts/webui_rendered_dom_smoke.mjs`（路径笔误）；核实主树该文件 diff 仅含本人三处误编辑（会话开始时 scripts/ 干净）后以 `git checkout -- <该文件>` 还原，主树 scripts/ 现为干净；正确改动已落在 worktree 版本。
- 验收：conditional pass（[rulings/2026-10-04-WEBUI-SESSION-SEMANTICS-1-conditional.md](../../rulings/2026-10-04-WEBUI-SESSION-SEMANTICS-1-conditional.md)，2026-10-04）/ 验收 commit=merge 2f5e57d1（合 main）+主树 dist 11:39:53 收口重建；我方复跑 verify_pc（vitest 435/435+build 0+E2E 复轮 pass SS1-SS3 True）。转正挂用户真栈目检两点。
