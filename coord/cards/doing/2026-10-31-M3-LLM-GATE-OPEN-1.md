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

- 2026-10-31 / owner=ZCode GLM-5.3 PC 执行会话（227 卡会话）/ 分支 port/m3-llm-gate-open-1 @ worktree D:/Vit_DAW_wt_227
- **领取基线与共享 main 追平**：origin/main（tip=b42c93c7=228 领取提交）不含 222 实现合入（实现在主管本地未推分叉 d753565c=发卡基线）。本卡渲染面以 222 交付物（neutral_automation_route.go+指引段）为基座，处置=merge d753565c 进工作分支（ac602287，唯一冲突=222 done 卡取主管含验收裁定版）；领取提交推送后共享 main 即含 222 实现+227/228 卡面+本领取，主管下次推送=fast-forward。
- 领取时 status：worktree tracked 零改动（merge 后亲核）；合并基线 go build ./... 全绿。主检出 D:/Vit_DAW 的既有 M 文件（VitApp/Workspace xml+coord/runs 日志）=登记前遗留，非本卡域零触碰。
- 文件域核对：228 域=VspKernelReference.cpp+capabilitycontext/free_state_observation.go——本卡域=agentloop+ruleset 骨架 twin（embed resource+legacy 常量，卡面标的 1 授权的骨架改写物理载体）+测试+scripts，零相交。
- 真栈错峰：当前栈空闲（登记面最新=222 于 10-30 21:46 释放，其后无占用；开发面无真栈需求）。重验腿等 228 合入后执行，天然错峰；若届时 228 在跑须再核登记面。
- §12 纪律照旧；LLM 轮次计费回执披露（重验腿）。

## 回执（2026-10-30 23:0x，开发面完成+重验腿执行+闸 2 深层锚点上交）

- 实现 commit：port/m3-llm-gate-open-1 `4a439588`（闸 1 五点+闸 2 headroom）→ `0369fe75`（merge 228 port 分支=重验前置）→ `8c06c815`（真栈登记）→ 本批（工件+释放+回执），已推 origin。
- **闸 1 开发面兑现**：五点（准入 union 四入口/显示过滤器+目录四行/needs_observation 校验放行+同轮单 mutation 限/执行守卫贯穿式——read-only barrier 等通用安全语义全保持/骨架 twin 治理门语义——字节级 parity 锁定在场）。chat 侧 ccb-only 授予不动；词表快道零触碰；诊断专用轮不开放。
- **闸 2**：`noteFreeStateAutomationEngagement`（命中即 +4 turn，settle+4 先例）——重验腿证明**触发器过晚**（见下），agentloop 域内无更早合法触发面（意图预判=词表禁区）。
- **闸 3 定案**：决策轮内全链可达（needs_confirmation 暂停=治理门→ResumeAfterConfirmation→写入→读回→终态），settle 面无需补轮；零裸写治理链单测端到端钉死（模型 set_points→executor 首见 Confirmed=false→确认恢复才 Confirmed=true）。
- **测试**：本卡十面+agent 全量 96 包 ok+chat/harness/tools/ruleset/carriers/promptruntime 全绿；222 同源锁定测试未触仍绿。
- **重验腿**（llmroute3_20261031_1，ps1 exit 0，220 断言体 v2 原口径未改，228 前置经 merge port 分支+kernel f9e3da0a 亲核只读复用）：**部署面亲证**——render 8/8 含六工具 Allowed tools 行+治理门骨架语义（system 15,468→17,045B），222 的指引/骨架同场冲突消除。**组 1 三连 no_route 止损（新形态）**：r1=1 模型轮即 limit_reached→「继续」→观察边界确定性 settle；r2/r3=前台 1 轮+自动续跑后台轮后「继续」被吞；工具面零 automation.*（模型未达执行相）。**根因锚点=`chat/audio_closure_controller.go:438-447` audioClosureMessageLoopBudget：closure 相位 slice MaxTurns 压 1-2+异步续跑泄流+观察边界 settle——闸 2 深层真身（222 的「MaxTurns 8/slice」描述在此修正），chat 域越界**。定性=停止条件 2 形态（接线需动 chat 核心编排→锚点上交扩域裁定）；**非**停止条件 3（非模型能力面：三前提之二 render 亲证在场，模型行为合规 catalog-first，从未获得写机会轮）；非闸 1 布线缺陷。本 run 一次到位无迭代轮（§8：同版本同断点三连同型不重跑）。
- **LLM 计费披露**：11 调用 / 124,621 in / 11,639 out（+29,696 cache read），deepseek-flash，逐调用在 run/logs/agent_llm_telemetry.jsonl。
- 端测覆盖边界：真栈覆盖=HTTP 面真实 LLM 轮次+render 取证+内核基线；不覆盖=轮结构修复后的组 1 通过形态/组 2-3/choreo pending 腿/set_points 确认链真栈端到端（单测级已验）/webui 渲染面。
- 真栈占用：登记 8c06c815（22:43，先于启动，占用前全零亲核）/释放 22:50 同批；拆栈 pid 路径亲核+复核全零。工件：coord/runs/M3-LLM-GATE-OPEN-1/{RESULTS.md,llmroute3_20261031_1}。
- **卡状态=doing 保留**：闸 1 面（主闸开放）已完成待验收；重验腿的「组 1 no_route 应消失」未达成——残差=闸 2 深层（chat 域轮结构），按停止条件上交，建议处置三选：①扩域裁定另开修复卡（closure 轮预算与 automation 诉求配合——候选方向在 RESULTS §四）；②本卡按「闸 1 交付+闸 2 锚点上交」验收合入，重验通过判定移后续卡；③返工。
