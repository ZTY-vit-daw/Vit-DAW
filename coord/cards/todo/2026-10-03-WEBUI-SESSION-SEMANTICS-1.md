# WEBUI-SESSION-SEMANTICS-1：主对话流语义——webui 启动自动建主会话+命名 / 执行轨迹随会话切换清空（P2）

- 池序 9；目标仓库=D:\Vit_DAW（agent/webui）；来源=[decisions/2026-10-03-user-manual-test-feedback.md](../../decisions/2026-10-03-user-manual-test-feedback.md) 问题 4+6；与 NOTESTREAM-2（note 副会话线）同族不同文件域，注意避让 webui 渲染段
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / webui IA 一期侧边栏已合 main（SessionFlowSidebar/sessionFlow 注册表）
- 模型分级：L1 / GLM 或 flash 可接
- 已核实事实：
  1. 现状：webui 打开后不主动新建会话，用户落第一条消息才落到某会话（或沿旧会话）；用户裁定产品逻辑=**note 流是副（子代理），webui 流是主**，主会话应启动即建并命名（或留白给用户命名——实现取轻：默认命名+可改名）。
  2. 症状 6（轨迹不随会话切换）：note 中输入 hi 后，webui 新建对话流继续工作，输入框上方执行轨迹仍显示 hi——轨迹面板未随 active session 切换清空/重绑。IA 一期 E2E 断言过"会话切换后任务轨迹清空是合法行为"（green 首轮 IA3 采样位修正记录在案），说明机制存在但 note→webui 新建流场景未触发（跨会话类型 or 事件时序）。
- 目标：
  1. 主对话流：webui 启动（或工程打开）自动创建主会话，默认命名（协议与 NOTESTREAM-2 命名协议同族，如「主对话流」+日期/序号），侧边栏可见、可改名；用户后续新建的会话平级列出。
  2. 轨迹绑定：执行轨迹/PlanBar 数据源随 active session 切换即时清空或重绑对应会话轨迹；补 webui 测试（切换会话→轨迹清空/换绑断言）+E2E 组扩展（note 会话→webui 新建会话场景）。
  3. 旧消息/旧会话兼容（AGENTS §11）：注册表 fail-open 既有语义零回退。
- 文件域：agent/webui/src/（SessionFlowSidebar/sessionFlow/App.tsx 会话初始化与轨迹绑定段）+测试。
- 约束：与 SETTLE-DELIVER-1 的 webui 渲染段改动如相遇，串行（SETTLE-DELIVER P1 先）；E2E 真栈排他（泊位）。
- 验收标准：npm test 全绿+新用例（主会话自动建+轨迹切换）+E2E-WEBUI-1 扩组 exit 0+用户目检复验。
- 停止条件：主会话语义需 agent 侧会话接口支持而现接口缺失 → 实锚清单上交定方案。
- 领取：（时间 / origin/main hash / 分支名）
- 回执：（commit hash / 主会话命名协议 / 轨迹绑定锚点 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
