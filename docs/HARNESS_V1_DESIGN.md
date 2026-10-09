# HARNESS_V1_DESIGN — L1-5 新 harness 骨架（pull 模式 · 相位降级冷启动 · 能力层快路径）

> 卡：coord/cards/doing/2026-10-09-L1-5-DESIGN-1（决策侧 L2 设计面，/morning 单点确认）。
> 来源：AGENTIC_OBSERVATION_ROADMAP_2026-09-25 D15/§3 线 1 L1-5 行/§4 质量门 G3 + CONTEXT_LAYERING_V1_DESIGN §7 衔接面/§10 OQ-1。
> 性质：零代码设计档。行号锚 2026-10-09 早窗 HEAD=8abbccd1 复核（helpers.go 剖面面沿 L1-4 卡锚本轮已复核）。

---

## 0. 设计定位

L1-5 是 harness 线关键路径最后一段地基：**把"每轮全量快照推给模型"的 push 形态，换成"模型按需拉取观察"的 pull 形态**，同时完成 D15 裁定的两项降级（相位轮次→冷启动第一轮；能力层→快路径库）。前置全齐：L1-2 物化（MAT 链）、L1-3 查询面（ref.query/ref.diff，接口冻结）、L1-4 四层前缀+退场（A/B/C/D 全合入）。本设计不实现，只定骨架接口与 G3 A/B 方案，产出实施卡排程。

**非目标**：编译器三层（L2-1 起）、质询协议（L2-3）、memory（D14）、FS0-FS9 统一收敛（T10 答辩后）——只定调用面占位。旧 harness（agentloop/chat 双入口）零破坏：A/B 裁定切换前生产默认 push。

---

## 1. 现役基线（勘察事实，本轮亲核）

### 1.1 push 模式两入口的同构循环

| 面 | agentloop（goal 运行） | chat（会话入口） |
|---|---|---|
| 主循环 | `MessageLoop.loop`（message_loop.go:400）：pendingToolQueue → 确定性 preflight 链（~10 项，:1003-:1976 一带）→ checkpoint → checkTurnBudget → buildContextSnapshot 全量重建 → assembleWithReport → LLM → executeMessageLoopToolCalls（:864） | server.go 会话管线（PrefixService 注入 :71/:512/:5752-5759） |
| 装配 | 每 entry 经 `assembleWithReport`（message_loop.go:4061-4068）走 PrefixService——L1-4 后稳定段/动态区物理分离已落 | `buildAssemblyWithReport`（server.go:5752 起）同型 |
| 快照 | `buildContextSnapshot`+`buildModelContextSnapshot` 逐轮全量；`ModelContextOverflow` fail-closed（不静默截断） | 同族 |

**push 的结构性成本**：快照逐轮全量重建（事实稳定前缀靠 L1-4 的字节级恒等治理兜住，但语义上模型每轮收到"当前世界全量"而非"所需增量"）；确定性 preflight 链在模型轮**之前**拦截（顺序耦合：preflight 命中即短路出结果，模型不参与）。

### 1.2 已就位地基（本设计的消费面，全冻结）

1. **PrefixService**（promptruntime/prefix_service.go）：AssemblyReport=层报告+前缀指纹+断裂归因（封闭枚举六值）+双轨一致性；BreakReason.PrefixBreaking 判 P2。注入缝已按"依赖注入点"设计（§7.1 语境）。
2. **退场执行器**（contextruntime/exit_executor.go:102/:137）：`OnTurnBoundary(TurnBoundaryEvent{TurnID, Window, Budget, Now})` 确定性零 LLM。生产接线两处：run 终态漏斗（agentloop/exit_retain.go:56，TurnID=RunID，观察结论 retain 进 L4 账本+语句级去重）与 history refs 伴随索引（contextruntime/history_refs.go:278）。**轮内逐 turn 触发未接**——本设计的接线面。
3. **查询读面**（L1-3 冻结）：`ref.query`/`ref.diff`（tools/catalog.go:727/:802）+ `ccb.observation_catalog`/`ccb.observation_request`（CCB 显式 view 请求）。重拉=query+Expand 组合，零动词新增义务延续。
4. **四层载体**（L1-4-IMPL-B）：ruleset manifest（embed）/user_profile/env_instance/L4 账本（append-only+链式 hash+genesis 头部段）。
5. **物化读侧**（L1-2）：MaterializedStore.Resolve/Query（F9 三函数契约冻结）；生产默认 off，A/B 期间读侧形态不变。

### 1.3 相位机器与披露位（回收对象）

- **推进面**：`audioclosure.Driver.AdvancePhase`（phase.go:340）——FS0 语义入口→FS1 工程绑定→FS2 容量→FS3 扫描→FS4 诊断轮→FS5 候选前沿→FS6 目标确认→…（合法前移链，guard 派生证据逐级放行）。消费方=chat/audio_closure_controller + chat/free_state_reasoning_loop（相位门已随 TIMING-1 退役，现役角色=推进+终态判定语境）。
- **披露位三处**（ccb_model_prompt.go）：GATE PATH 预披露（:220/:273——needs_experiment 准入缺证时指引先走 ccb.observation_request 补证）；TERMINAL TURN（:358——终态轮机械状态头）；G1-G8 证据门指引（:582）。
- **剖面选择**（helpers.go:116-129 `messageLoopModelContextProfile`）：由 free-state 上下文 `decision_phase` 驱动快照分区裁剪（selection/materialize/post_action 三剖面）——L1-4 已裁"正交不动"。

### 1.4 快路径现状（原型已在产）

message_loop 确定性 preflight 族≈快路径库的**无声明原型**：模式化高频任务（clip fade/gain 读写、strip silence 建议、clip range split、stems 导入、section markers 应用、project blackboard 状态、静态混音能力契约……）在模型轮前匹配用户意图→直接产出确定性 tool calls 或回复，零 LLM 成本。现状问题：与主循环顺序耦合（diagnostic-only 轮需整体旁路，message_loop.go:417-441 注释）、散在 message_loop 巨文件内、无统一注册/路由面。

---

## 2. OQ-1 裁决：turn 语义 = 工具循环节

**裁决**：pull loop 的"轮次"（turn）= **一次模型响应及其工具执行批**（工具循环节）；run 终态是一个额外的终局边界（既有先例 TurnID=RunID）。逻辑任务节**不进 v1**。

**依据**：

1. **机械可界定**：工具循环节的边界是客观事件（工具批执行完毕、下一模型调用前）——退场判据要求确定性零 LLM（exit_executor.go:138 注释明文），"逻辑任务完成"依赖语义判断，让 LLM 参与边界判定违反该纪律。
2. **执法粒度对齐既有机械面**：checkTurnBudget 按模型轮执法、chat 截尾 limit=12 按消息对、观察账本窗口 24 对——预算与窗口的既有机械粒度全部落在工具循环节一级；逻辑任务节粒度会在任务内多轮爆预算而无执法点。
3. **任务级语义已有承载**：任务结论/确权事实按语义事件入 L4 账本（Kind=observation_conclusion 等，append-only），不需要边界事件承载"任务完成"。
4. **接口零改动**：TurnBoundaryEvent 的 TurnID 是字符串——命名空间定 `cycle:<n>`（run 内递增）即可，run 终局沿用 RunID（exit_retain.go 先例），L1-4 冻结接口不动。

**触发点映射**（新 harness 内，每循环恰好一次）：

```
模型响应 → 工具批执行完毕 → [T1] OnTurnBoundary(cycle:<n>, window, budget)
        → （若继续）装配下一轮 → 模型调用 → …
run 终态（终局判定/止损/预算尽）→ [T2] OnTurnBoundary(RunID) + retain 收尾（既有语义）
```

T1 的退场动作=该轮观察原始内容出窗（retain/ref/drop 三态，判据沿 CONTEXT_LAYERING §4 既有机械规则）；T2 维持 exit_retain 既有行为（观察结论 retain 进工程账本）。暂停/续跑面不触发（账本随 continuation 存续——既有语义）。

---

## 3. pull loop 骨架

### 3.1 组件与循环结构

新包 `agent/internal/pullharness`（纯新增，零旧面改动——A/B 并存前提）：

```go
type PullLoop struct {
    LLM       llm.Client            // 既有客户端面
    Prefix    promptruntime.PrefixService // 注入，不自建装配报告/指纹
    Tools     ToolExecutor          // 既有动词面的执行适配（零动词新增）
    Exit      contextruntime.ExitExecutor  // T1/T2 触发
    Router    FastPathRouter        // §5 快路径路由
    Budget    ObservationBudget     // §3.4
}

func (p *PullLoop) Run(ctx, in GoalInput) Result
```

循环（与 §2 触发点映射对应）：

```
冷启动底座装配（§4，一次性） → for {
  ① 快路径路由尝试（§5：命中→确定性执行→直接产出/短路，零模型轮）
  ② 装配：PrefixService（四层稳定前缀）+ 动态区（会话状态+预算披露，§4.2）
  ③ 溢出预检：ModelContextOverflow fail-closed（复用既有面）
  ④ 模型调用 → 无工具调用 → 终态判定（含失败分类，§3.4）
  ⑤ 工具批执行（读面 ref.query/ref.diff/ccb.observation_*；写面既有执行动词）
  ⑥ [T1] OnTurnBoundary(cycle:<n>)
  ⑦ 预算检查：exhausted → 止损终态 budget_exhausted
}
[T2] 终局 retain（exit_retain 语义同型移植）
```

**与 push 的语义差异**（G3 对比的本质变量）：无逐轮全量快照注入——模型的世界认知=四层前缀（稳定事实）+动态区（会话状态）+主动查询结果（ref.query/ccb.observation_* 按需拉取）。观察是模型发起的动作，不是环境推送。

### 3.2 装配与 PrefixService 注入

- 四层前缀作为 static/session Section 族挂载（CONTEXT_LAYERING §7.1 字面执行）；pull loop 每轮只重算动态区。
- PrefixService 注入"轮次边界与预算状态"：AssemblyReport 按会话键持续（P1 append-only 判据跨 turn 可测）；预算状态进动态区披露（§4.2），不进前缀。
- 冷启动底座（§4.1）作为 session 层 Section 在会话首装时挂载，之后字节恒定。

### 3.3 工具面消费（零动词新增）

pull 模式所需动词全部已在产：读=ref.query/ref.diff/ccb.observation_catalog/ccb.observation_request；写/执行=既有 mix.*/midi.*/vsp 执行面。ToolExecutor 是适配层不是新工具层；成本分级标注沿 D5（index 级零成本、render/probe 级计入预算）。

### 3.4 观察预算与止损线（路线图"第一天进设计"义务）

```go
type ObservationBudget struct {
    MaxCycles   int     // 工具循环上限（对应既有 checkTurnBudget）
    ProbeCost   float64 // render/probe 物理成本账户（D5 分级计量）
    MaxProbeCost float64
}
```

- 超限=止损终态 `budget_exhausted`——**独立失败分类**，不与 no_candidate_found/capability_blocked 混记（AGENTS §8 分开记录义务）。
- 动态区在接近阈值时披露预警（预算指令披露位迁移的落点，§4.2）。
- 终态判定（无工具调用轮）产出的失败分类沿 push 既有口径（诚实分记），A/B 对比按分类分层统计（§6）。

---

## 4. 相位降级冷启动 + 披露位迁移

### 4.1 冷启动第一轮（相位轮次的降级形态）

D15 裁定：相位轮次→"冷启动第一轮（保证每会话共同底座）"。落地形态：

- **冷启动底座**=会话首装时由装配器从投影只读面渲染的一次性机械事实段：工程绑定/容量/扫描的机器可得部分（FS1-FS3 的语义对应物）+TOM 概览/总线拓扑/交付目标（**与 L4 genesis 头部段同源同事实组**——GENESIS 卡已建的三事实组只读渲染面直接复用，不新造第二事实源）。
- **不再有相位推进链**：新 harness 不消费 `AdvancePhase`（audioclosure 相位机器保留给旧 harness 并存期使用；FS 状态机统一收敛是 T10 域）。"每会话共同底座"由底座段字节恒定保证，不靠状态机逐级放行。
- **L4 账本 `decision_phase` 元数据**：写入侧继续注记（append-only 元数据语义不变，L1-4 §7.2 已裁），新 harness 读侧不驱动任何行为。
- **剖面选择**（helpers.go 同型逻辑）：新 harness 无相位驱动剖面切换——剖面裁剪归动态区状态（预算/终态语境）决定，与相位正交（L1-4 裁定延续）。

### 4.2 披露位迁移处置表（ccb_model_prompt.go 三处 + 预算）

| 现役披露位 | 语义 | 新 harness 去向 |
|---|---|---|
| GATE PATH 预披露（:220/:273） | needs_experiment 缺证准入指引：先 ccb.observation_request 补证再提案 | **冷启动底座**（一次性指引）+ ref.query/observation 响应内的 machine-readable gap 结构（既有语义保留）；不再逐轮复现 |
| G1-G8 证据门指引（:582） | 准入只由证据门决定，勿抢提案 | **冷启动底座**（规则层内容，进 L1 ruleset 域——内容盲审查照 T-C1 键） |
| TERMINAL TURN（:358） | 终态轮机械状态头 | **动态区**逐轮状态指令（迁移实现面，语义不变） |
| 预算指令 | （现役散布） | **动态区** BudgetState 披露（§3.4 预警落点；pull 模式预算可见性是止损线的一部分） |

旧 harness 披露位零改动（A/B 并存期各自独立）。

---

## 5. 能力层快路径挂接

- **快路径库**=message_loop preflight 族（§1.4 清单）的归并重构：`FastPathRouter` 统一注册面（模式匹配器+确定性产出），每循环 ① 位尝试（pull 骨架内），命中→确定性执行短路，零模型轮。
- **路由由 planner 决定**（D15 原文）的 v1 形态：路由=纯模式匹配前置层（非 LLM 调用）+未命中才进 LLM 轮；`LLMPlanner.Next`（planner.go:135）不在快路径内——快路径是它的**前置分流**不是它的内部件。模式库=现役 preflight 匹配器平移（匹配语义零变化，位置从 message_loop 主循环前移到 Router 注册面）。
- **能力层执行体不动**：capabilityadapters/orchestration 编排链是快路径命中后的执行体（模式化高频任务仍走编排），A/B 期间零内部改动；快路径库只是调用面的收敛与可声明化（注册清单=可审计面）。
- **CCB 实验边界保留**：diagnostic-only 轮旁路快路径的既有语义（message_loop.go:417 注释）平移为 Router 的显式开关（diagnostic 轮禁用快路径——防确定性旁路制造实验边界外的终态）。

---

## 6. G3 A/B 方案（质量门，可执行化）

- **flag 面**：harness 选择=env `VIT_DAW_HARNESS`（pull|push，缺省 push）+ `agent_runtime_config.json` 键 `"harness_mode"`（env wins、会话启动读取、运行中不改——**与 blind 配置面同型先例**，audition_events.go 配置面范式；该文件 BOM 容错归 CONFIG-BOM-1 卡）。缺省 push=旧路径零变化（§11 兼容义务）。
- **选样**：一个完整工程=旅程烟测已覆盖的 demo 工程（JOURNEY 1-5 站点全：工程打开→权限→实验→装载→A/B 判定），双模式各跑全流程，≥3 轮/模式（§8 概率运行义务：轮数、成功条件、失败分类、止损线预写）。
- **指标口径**：
  - 质量=判定链 outcome 分层分布（judgment_ok/no_candidate_found/capability_blocked/budget_exhausted 分记）+G1-G8 证据门通过率；
  - 成本=LLM 调用轮数+model_snapshot_bytes 口径字节量+probe 物理成本账户；
  - 前缀命中=AssemblyReport P1-P3（prefix_bytes 跨 turn 字节级 starts-with、断裂归因分布、动态区隔离三判据）。
- **裁定门槛**：质量不劣化（判定链成功率≥push 基线且无系统性回退）且成本显著改善（轮数或字节量下降）→ 建议切换；任何质量回退→不切换+立差异归因卡。budget_exhausted 轮单独归因（预算校准问题≠模式质量问题）。
- **执行载体**：dev_agent_smoke.ps1 体系新增场景（harness_ab 或分场景 pull/push 各一），退出码 0 为门槛（AGENTS §5 真实栈纪律）；旅程面覆盖归 JOURNEY 既有场景族扩展。

---

## 7. 段落投影消费接口预留（汇合段占位）

- L2-2 段落发现层 DSP 产出=中性坐标+假设标签（路线图 L2-3 命名层再确权）。L1-5 只定消费形态：**段落投影以 vit:// ref 族暴露**（D2 对象类型轴预留位），模型经 ref.query 按需拉取——**不进前缀**（动态查询面，防过期前缀污染每一轮，D7 立论同源）。
- 冷启动底座预留"曲式概览"slot（届时从段落投影渲染一行概览，装配器渲染、只读面）。
- 编曲/AIGC 汇合段的工具面（长时程大颗粒任务）不在本设计展开——汇合卡另行立卡。

---

## 8. 红线对账

| 纪律 | 遵守方式 |
|---|---|
| 投影 peer 不是链 | pull 工具面=投影只读消费端；无 DAD→前缀链式新语义 |
| CCB 不自动选 view | 观察拉取=模型显式 ref.query/observation_request；装配器不代发 |
| raw package 不展开 | 读面响应沿 LLMContext/摘要行；伴随索引透传 freshness |
| 内容盲（TIMING-2） | 快路径确定性回复与冷启动底座规则段不得含域处理知识；RuleSection.Purpose 机械审查键照 T-C1 |
| L1-3/L1-4 接口冻结 | ToolExecutor 零动词新增；OnTurnBoundary/TurnBoundaryEvent 零改动（TurnID 命名空间约定即可） |
| §8 概率运行 | G3 轮数/成功条件/失败分类/止损线预写成文（§6）；budget_exhausted 独立分类 |
| §11 持久化兼容 | flag 缺省=push 旧行为零变化；新增配置键定义缺省语义（无键=push）；不触碰既有记录格式 |
| 测试不写源码树 | 全部隔离工作区/t.TempDir |
| 观察预算止损 | §3.4 机制化（MaxCycles+ProbeCost 账户+预警披露） |

---

## 9. 实施卡排程建议（决策侧裁剪用）

| 卡 | 内容 | 验收线 | 依赖 |
|---|---|---|---|
| L1-5-IMPL-A | PullLoop 骨架+flag 面（env+config 键）+PrefixService 注入+ObservationBudget——纯新增包，旧面零改动 | 单元测试：循环/边界触发（T1/T2）/预算止损/flag 缺省 push；`go build`+全量 0 FAIL | 无（接口本设计冻结） |
| L1-5-IMPL-B | 冷启动底座装配（GENESIS 三事实组复用）+披露位迁移（表 §4.2）+退场逐 turn 接线（T1） | 底座字节恒定断言；T1 退场三态测试；内容盲审查（T-C1 键扩展） | A |
| L1-5-IMPL-C | FastPathRouter：preflight 族平移+注册面+diagnostic 旁路开关 | 平移前后匹配语义等价测试（现役 preflight 用例回归）；注册清单可审计 | A（与 B 不同文件域可并行） |
| L1-5-IMPL-D | G3 A/B 执行：smoke 场景扩展+双模式全流程+指标采集+裁定报告 | 真实栈 exit 0；§6 指标三层齐；G3 裁定建议成文 | A/B/C |

排序：A 先（骨架独立可测）；B/C 并行；D 收口。每卡真栈门槛按 AGENTS §5；渲染面/旅程边界显式声明义务照旧。

---

## 10. 开放问题与停止条件对照

| # | 开放问题 | 处置 |
|---|---|---|
| OQ-H1 | pull 模式下 continuation/暂停语义与旧 harness 的 continuation 结构归并（消息流/工作树分支面） | IMPL-A 实现卡按 GoalInput/Result 既有形态定最小面，决策侧复核；争议上交 |
| OQ-H2 | 快路径命中产出的终态与模型终态在判定链遥测里的分记口径 | IMPL-C 定（fastpath 终态独立分类，不混入 judgment 统计）；沿用 §8 分记纪律 |
| OQ-H3 | chat 会话入口与 goal 运行入口是否共用同一 PullLoop 实例形态 | IMPL-A 先落 goal 形态（agentloop 同构）；chat 入口迁移归 G3 裁定后的切换卡，不在 L1-5 四卡内 |

**停止条件核验**（卡面两条）：①L1-4 落地形态与设计文档偏差——本轮亲核四处关键锚（PrefixService 注入两入口/exit_retain 终态漏斗/exit_executor 接口/查询动词注册）零偏差，不触发；②pull loop 与未动组件耦合——编译器/质询协议仅为调用面占位（§7）、相位机器不消费（§4.1）、memory 不涉及，骨架可独立成立，不触发。

---

*本文档由 L1-5-DESIGN-1 卡（PC 决策会话，GLM-5.3/L2 设计面，2026-10-09 /morning 单点确认）产出；事实锚本轮 HEAD=8abbccd1 亲核（§1 清单），规格锚路线图 D15/§4-G3 与 CONTEXT_LAYERING_V1_DESIGN §7/§10。*
