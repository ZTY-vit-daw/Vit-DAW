# M3-AUTOMATION-SLICE-BUDGET-OPEN-1：automation 决策轮片预算放开至天然停止点+smoke 终态等待保险丝（229 验收门第四承，用户裁定 2026-10-31）

- 发卡：GLM 主管决策流 / 2026-10-31（依据=231 验收定案+用户裁定：采样协议与异步执行模型错配；限轮位置从每片挪到总量——slice 预算对 automation 轮放开，MaxClosureRounds 总护栏保留，治理门/限速零改动；裁定全文 coord/decisions/2026-10-31-slice-budget-position.md；事实核查=Claude Agent SDK headless 默认 10 轮同款、交互式主流不限轮、边界控制三轨正交共识）
- 派发确认：已确认（用户 2026-10-31「可以发卡」）
- 验收负责人：GLM 主管决策流
- 池序 232；目标仓库=**主仓 agent chat 预算域+scripts**；P1 / 0.5 天 / 依赖=231 已合入（69aec89b）✓；L2 / PC 执行会话（预算语义+LLM 判读）

## 标的（231 定案+用户裁定，不重考）

231 实测：模型每同步窗口仅完成 2-3 工具调用（checkpoint+catalog+observation）即耗尽 10 轮片预算→limit_reached→异步续跑排队文本→no_route×3+拆栈杀续跑。完整链需约 8-10 工具调用（catalog→观察×2-3→get_plugin_parameters→set_points（治理门暂停）→ResumeAfterConfirmation→get_curve→汇报），deepseek-flash 开销率约 3 轮/工具（231 遥测：9-10 轮/3 工具）→链需约 25-30 轮，片预算 10 结构性不够。**每片 10 轮是切片机制不是安全边界**——边界主体（治理门/每轮一 mutation 限速/MaxClosureRounds 总护栏/「取消」打断）已建齐且零改动。

## 目标

1. **生产面：automation 决策轮片预算放开**——`audioClosureMessageLoopBudget` 对 `audioClosureAutomationRoundContext` 命中轮返回**余量预算**：推导成文（建议区间 40-48 turns / 对应 MaxToolCalls 同比放大；依据=231 遥测链长×1.5 余量，推导过程入回执），使一轮请求可跑到**天然停止点**（确认卡 waiting_interaction 暂停/完成汇报/capability_blocked 申报）。**MaxClosureRounds 总护栏、治理门、每轮至多一 mutation 限速、223 stamp、非 automation 观察 slice 语义全部零改动**（229 测试五面零回退）。
2. **仪器面：smoke 终态等待保险丝**——`automation_llm_route_smoke.py`：①进度/排队文本（limit_reached 续跑提示/automatic_continuation_already_scheduled）不再即判 no_route，改为**有界等待终态**（分钟窗，轮询 goal/task 终态）；②终态分派=waiting_confirmation 确认卡在场→**驱动确认**（「确认」话术或 218/226 断言体先例驱动面）→验治理写入+get_curve 逐点读回=success；capability_blocked→按 230 分型；等待窗尽仍无终态→新型 in_progress_timeout 如实分类（不硬塞 no_route）；③判定逻辑其余原口径不动（三组×5 轮、组级 ≥4/5、同型三连止损承 220）。
3. **重验腿（本卡验收门，229 原口径第四承）**：组 1「这段让它越来越突出一点」先行——**组 1 no_route 消失+set_points 确认链（确认卡→确认→写入→get_curve 读回）首次端到端全绿**；组 2/3 按 220 口径跑满。

## 文件域

`agent/internal/chat/audio_closure_controller.go`（预算面，229 同域）+预算来源邻域（勘察定：mode 基础预算的派生面）+测试；`scripts/automation_llm_route_smoke.py`（仪器面）。越域即停上交。

## 安全面（硬约束）

- 治理门/每轮一 mutation 限速/MaxClosureRounds/stop-pending/隔离/权限闩零回退——**预算放开不得成为无治理连写通道**；
- 放开仅限 `audioClosureAutomationRoundContext` 命中轮（229 识别面复用，不新增识别逻辑）；诊断轮/非 FS closure 轮钳制语义零回退；
- §8：LLM 真实计费，轮窗放大后单轮调用数可能上升，回执披露调用数与 token；止损纪律不变。

## 停止条件

- 预算放开与 closure 收敛/调度语义冲突不可调和→锚点+方案上交；
- 放开后模型超余量预算仍不到天然停止点（如 48 轮仍 limit_reached）→**模型能力面证据上交**（verbose 率过高，转模型/指引设计讨论：决策轮换引擎或指引 nudge——届时带实测数据）；
- 等待终态窗内确认卡出现但驱动确认失败→断言体缺陷域内修；治理链写失败→write_fail 如实分类锚点上交；
- LLM 面栈不可用→环境中断如实记录。

## 领取

（待领取：记录 HEAD/status/diff --stat；真栈错峰登记 PC-RUNTIME-STACK.md；§8 计费披露）
