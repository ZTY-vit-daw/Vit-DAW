# M3-LLM-GATE-OPEN-1：变体话术 LLM 通道闸门开放——free-state 决策轮 automation 工具面准入（终态架构第一步，词表退役路线起点）

- 发卡：GLM 主管决策流 / 2026-10-31（依据=222 三道闸定案+用户终态裁定「可以开卡」：[2026-10-30 判断权归属决策·补充裁定](../../decisions/2026-10-30-judgment-ownership.md)——主流对标定案意图层全 LLM，词表退役四阶段，本卡=阶段①）
- 派发确认：已确认（用户 2026-10-30 深夜对标讨论后「可以开卡」）
- 验收负责人：GLM 主管决策流
- 池序 227；目标仓库=**主仓 agent**；P1 / 1 天 / 依赖=222 已合入 ✓、**重验腿前置=228 合入**（观察面轨级插件披露，模型解析 plugin_id 前提——开发面可先行，重验腿等 228）；L2 / PC 执行会话（free-state 安全面+LLM 判读）

## 设计前提（终态架构，卡内不重议）

用户输入→LLM（全部）→模型自主（调工具/反问/提议）→治理门（权限模式+RiskConfirm）→确定性执行→读回→继续或汇报。本卡只开闸不动词表快道（快道退役=后续阶段卡，单卡单入口行为变更）。

## 标的（222 三道闸定案，不重考）

1. **闸 1（主闸）**：free-state 决策轮（neutral 装配）allowed tools=ccb.* only+骨架规则「Do not call any mutation tool in this phase」——指引与骨架同场冲突，模型合法动作集不含 automation。
2. 闸 2：观察探索耗尽 MaxTurns（8/slice）→waiting_continue 截停。
3. 闸 3：settle 面无 LLM 轮（message_loop_system 完整装配未被变体话术触达）。

## 目标

1. **闸 1 开放**：free-state 决策轮工具面准入 automation 三工具——**只读工具直接放行**（get_plugin_parameters）；**mutation 工具经治理门**（set_points→RiskConfirm 确认卡/完全权限直执，223 stamp 已通——**触发权=模型工具调用**，与 Claude Code permission gate 同构，词表确认链不动）。骨架规则改写：mutation 禁令改为「mutation 必经治理门」语义（不再一刀切禁）。
2. **闸 2 配合**：automation 诉求在决策轮内可达执行——预算分配方案勘察定（automation 调用不计入观察探索预算/或诉求命中即预留轮次），不破坏既有观察 slice 语义。
3. **闸 3 随动**：闸 1 开放后 settle 补轮必要性勘察——若决策轮内即可完成「调工具→治理门→执行→读回→汇报」全链则闸 3 无需另开；否则最小补轮方案上交。
4. **重验腿（228 合入后跑）**：220/222 断言体 v2（automation_llm_route_smoke.py）组 1 先行三组×5 轮，口径承 220 §8 成文不得改——**组 1 no_route 应消失**（指引在场+工具面在场+观察面可见 plugin_id 三前提齐）；set_points 确认链同构可用（确认卡→确认→写入→读回）首次真实验证。

## 安全面（硬约束）

- set_points 在 free-state 轮的每一条路径必须经 RiskConfirm/权限链——**零裸写**（负例测试：任何绕治理门的 mutation 调用即红）；
- 完全权限直执承 223 stamp 形态；manual 模式确认卡字节形态不变；
- FS 状态机既有安全语义零回退（stop-pending/隔离/权限闩）。

## 文件域

`agent/internal/agentloop/`（neutral 装配工具面+骨架规则）+free-state 决策轮工具准入（harness/agentloop 勘察定）+测试+scripts（断言体复用）。越域即停上交。

## 停止条件

- mutation 工具准入与 FS 状态机安全语义冲突不可调和（FS 体系设计约束）→锚点+方案上交；
- 治理门接线需动 chat/harness 核心状态机→锚点上交扩域裁定；
- 重验组 1 指引+工具+观察三前提齐备仍三连 no_route→属模型能力面，锚点上交转模型/指引设计讨论，不盲目迭代。

## 领取

（待领取：记录 HEAD/status/diff --stat；真栈错峰登记——228 若在跑真栈须错峰；§12 纪律照旧；LLM 轮次计费回执披露）
