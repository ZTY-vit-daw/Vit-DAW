# 执行轨迹呈现重设计 V1：回合单活动面（TRAJECTORY-DESIGN-1 设计定稿）

- 状态：**已定稿（2026-09-14 用户裁定四点）**——A 工具步骤人话映射（§2.1-3 增补）/ B 等待行暂不加「继续」按钮（§2.4-2 增补，增量小随时可补）/ C 收据行一行先做出来看（§2.3 维持）/ D 三面维持「一条流（回合块+气泡）+一个锚（PlanBar）」分层，收据并入气泡头部记入分期二可选。实现卡按分期一 ③→①→② 拆发。
- 任务卡：`queue/doing/2026-09-13-TRAJECTORY-DESIGN-1-single-live-surface.md`（用户裁定 2026-09-13；两轮手测证据与决策侧四问题诊断在卡内，本稿直接引用不重查）。
- 勘察基线：HEAD `9c22fd9`（2026-09-14 09:18 领取时），只读勘察 `agent/webui/src` 与事件流协议文档，零生产代码改动（本卡承诺）。
- 参照系：DSH/ZCode 型单活动面四原则（决策侧归纳，见 §3 映射表）。
- 文件域约束：与 chat 域执行会话并行，本设计与分期一全部改动落在 `agent/webui/src`；`agent/internal/chat` 零接触。

---

## 1. 现状结构（锚点勘察）

当前「执行过程呈现」由三个面加两个辅助机制构成：

| 面/机制 | 组件（锚点） | 数据源 | 生命周期 | 粒度 |
|---|---|---|---|---|
| PlanBar 任务规划条 | `composer/PlanBar.tsx` | `vit.task_runtime_trajectory.v1` 快照（`/agent/runtime/status` 8s 轮询）+ `uiState.goal`/`agent_plan` 回退 | 跨回合常驻锚；PLANBAR-1 后终态/陈旧态有限时间收起 | 任务级（goal/run） |
| 回合轨迹块 TraceBlock | `trace/TraceBlock.tsx` + `trace/renderPlan.ts` | `trajectory.*` 事件（`vit.observable_trajectory.v1`，协议定为 transient/persistence=none） | 回合级：live→终态坍缩回执（0.9s 后自动收起） | 回合步（实验轨迹节点） |
| 对话气泡 | `App.tsx` MessageStream | durable 消息（localStorage `ask_vit_conversation_messages.v2` + Project History 水合） | 持久 | 语义内容（用户消息/最终回复/判定卡） |
| 乐观占位 | `trace/TraceBlock.tsx` `OptimisticTraceBlock` | `isSending && 无 live 真块 && 流尾是本轮乐观用户消息`（`shouldShowOptimisticTrace`） | 发送窗口 | 单行「正在处理」 |
| 活动线+瞬态活动 | `MessageStream` activity-lane + `messageLifecycle.reduceAgentEventActivities` | `item.*`/`approval.requested` 等 → transient 活动行 | 回合终局事件清退；**未绑定轨迹回合的活动渲染在流底活动线**（`turnGroups.isUnboundActivity`） | 单行一次性活动 |
| 回合事件足迹 meta | `trace/turnEventMeta.ts` | 原始事件流：item 足迹计数 + startedAt/endedAt/residencyEndedAt（CONT-STALL-1 工作/驻留拆分唯一真源） | 内存，随事件回放重建 | 只作证据与时长，不渲染步 |

关键事实（后续引用）：

- **F1**：回合块的渲染存在性 = `trajectoryTurns(trajectory).filter(turn => turn.nodeIds.length > 0 && shouldRenderTraceBlockForTurn(...))`（`renderPlan.ts:111-113`）——即「回合有没有实验轨迹节点」。`trajectory.*` 事件族只由实验环/自由态链发射（`OBSERVABLE_TRAJECTORY_PROTOCOL_V1` §4 事件命名空间全是实验语义节点）。
- **F2**：纯 LLM/观察回合零 trajectory 节点 → 被过滤 → 无块。其间到场的 `item.*` 事件被 transient 化为活动行，其回合归属 `turn_id`（chat `turn_*` 域）与轨迹回合键（`source_turn_id` 优先 → `run_*` 域，`trajectoryTurnIdOfEvent`）不同命名空间 → `isUnboundActivity` 恒真 → 活动落**流底**活动线（正是 composer 浮层遮挡带，同 `.notice.warning` 已知缺陷族）。
- **F3**：回合块三态数据（trajectory state / turnEventMeta / activities）全部内存态，重建依赖 `/agent/events` 自 seq 0 回放；协议把 trajectory 事件定为 transient（§2：durable 用户可见输出走 Project History ConversationNode）。决策侧探针已证重载后 traceIndexes 空——「用活态当持久层」的结构结果。
- **F4**：CONT-STALL-1 已把工作/驻留拆分记账（`turnDurationSplit` → workMs/parkMs），但只在**终态** meta 呈现；驻留期 live 无任何呈现（块停在「正在处理」+陈旧思考行）。
- **F5**：消息面持久化双轨：localStorage（durable 过滤后存，上限 120 条）+ Project History 服务端会话图水合。刷新后气泡在、块不在——与用户手测证据一致。

---

## 2. 四个结构性问题逐条回应

### 2.1 问题①：可见性错绑（实验准入 ≠ 回合活动）

**根因**（F1+F2）：回合块把「容器的存在性」绑在「实验准入的产物」上。进度可见性应当绑定「回合有活动」，实际绑定「回合进了实验环」。item.* 明明在场，却被两条规则排除在回合内呈现之外：不投影为轨迹节点（2026-09-05 取证定案），且跨命名空间归属使它连回合附属位都拿不到。

**修法：回合容器与实验轨迹步解耦**（DSH 原则 1 落点）。

1. 新增**回合步归约器**（`trace/roundSteps.ts`，纯函数+增量归约）：`item.started` / `item.completed` / `item.failed`（附加 `approval.requested`、`turn.failed`）→ 回合内步（key/title/status/createdAt），回合键沿用 `trajectoryTurnIdOfEvent`（`source_turn_id` 优先）——与 `reduceTrajectoryEvents`、`reduceTurnEventMeta` 同键，B9「一轮对话一个轨迹块」不破。每回合步数设上限（滚动窗口留最近 ~50 步 + 总计数）。
2. **渲染谓词放宽**：回合有任一活动证据（trajectory 壳节点 / trajectory 步 / item 步 / turnEventMeta.startedAt）即有容器；实验轨迹步降格为步来源之一。M12 settle_slice 隐藏谓词（`shouldRenderTraceBlockForTurn` 证据链：settle_slice 标记 + 无步无活动足迹 + 短寿命回退）**逐字保留**——渲染放宽不得让结算切片噪音回归。
3. **步序混排**：item 步与轨迹步按 createdAt 排序内联同一容器，行形态复用 `TraceStep`（item 步 kind=activity）；live 态思考行语义不变。
4. **归属锚定零服务端改动**：沿用 UI-FOLLOW-1/2 两级锚定（身份锚定 → 用户消息槽位时间戳锚定，50ms 容差）。回合容器键与消息 `turn_id` 跨命名空间时由槽位锚定兜住——机制已在产，不需新发明。
5. **活动线去重**：已并入回合块的 item 活动不再进流底活动线（lane 过滤条件扩一步：活动在 roundSteps 已有归属即视为 bound）；无回合归属的活动（上传等）照旧。

**效果**：第二请求 31s 场景——发送即乐观占位 → `turn.started`/首批 `item.*` 到场，回合容器接管 → 工具调用逐条内联为进度步。进度可见性从此与实验准入无关。

### 2.2 问题②：三面并存，无主次

**裁定：不砍面，分层定责**——三个面各有唯一职责与唯一时序位，互相不再抢戏：

| 面 | 唯一职责 | 时序位 |
|---|---|---|
| PlanBar | 任务级锚定状态行：这个任务在做什么、现在什么状态（DSH 原则 2 落点） | 跨回合常驻锚（composer 停靠列） |
| 回合块 | 回合级单活动面：回合进行中一切活动内联；终局坍缩为本回合回执行（DSH 原则 1/3 落点） | 用户消息之后的回合附属位 |
| 对话气泡 | 语义内容：提问、最终回复、判定卡、回执消息——**不含中间态进度** | 消息流本身 |

**主次**：回合进行中，用户看回合块（唯一活动区）；PlanBar 提供全局锚但不承担回合内进度细节；气泡承载结果不承载过程。PlanBar 与回合块是任务级/回合级两个分辨率，不是竞争面——用户「不知道看哪个」的根源是回合块经常缺席（问题①），不是 PlanBar 多余。PlanBar 本卡不动（PLANBAR-1 刚验收、用户已确认）。活动线在分期一收窄为无回合归属活动的兜底，答辩后评估整体退役。

### 2.3 问题③：活态/水合不一致（刷新后整块消失）

**根因**（F3+F5）：回合块消费的全是 transient 事件派生态，刷新后能否重建取决于服务端事件缓冲可否回放——而协议哲学本来就是「活态不持久，durable 的只有 Project History 节点」。块消失是拿活态当了持久层。

**修法：持久的是摘要，不是活态**（DSH 原则 3 落点）。

- **分期一（webui 域）**：新增**终局回执行台账** `vit.turn_receipts.v1`（localStorage，per conversation+scope，bounded ~200 条）。回合终态收口时写入一行摘要：回合键、状态、步数、活动数、执行时长、等待时长、started_at。水合路径（conversationID/scope 恢复效应）读取台账，为**活态中不存在**的回合渲染静态收起回执行（槽位锚定落位）；活态存在同键回合时活态优先、回执不重复渲染。刷新后用户看到的是「执行完成 · N 步 · 执行 Xs」收起行，不是活块——这正是原则 3 的「持久的是摘要不是活态」。
- **分期二（服务端，chat 域另拆卡）**：终局回执落 Project History 会话图 durable 节点（`message_kind=turn_receipt`），与「completed user-visible output 走 ConversationNode」的协议既有哲学对齐；跨浏览器/跨设备；webui 台账降级为缓存。
- **否决项**：把 `/agent/events` 缓冲做成持久回放——把活态当持久层，治标、chat 域大改、与协议 transient 语义冲突。

**边界（如实声明）**：localStorage 台账换浏览器/清缓存即丢。可接受——最终回复本来就在气泡里（Project History 水合），丢的只是「这回合干了多久几步」的回执行，非语义内容丢失；分期二根治。

### 2.4 问题④：时长语义不直观（「执行了 220 秒」误读）

**根因**（F4）：拆分已在后端记账，呈现只有终态一处且句式可被读成总执行时长；驻留期 live 零呈现。

**修法三件**（DSH 原则 2/4 落点）：

1. **live 态头部 meta 改「已工作 Xs」**：`now - startedAt` 每秒推进（startedAt 已有，turnEventMeta 唯一真源地位不变）；不再只有「N 步 / --」。
2. **驻留显式行**：live 且 `endedAt` 已到（切片以 waiting_continue 收尾）而回合终局事件未到 → 块内显式行「等待续跑（已等 Xs）」——不出现空白、悬置或把等待冒充工作。判定全部消费既有 meta 字段，归约器零改动。
3. **终态句式统一**「执行 Xs · 等待续跑 Ys」（`traceMetaParts` 已产出「执行 Xs，等待续跑 Ys」，冻结该句式为契约）；无驻留段的回合不多个词（既有缺省不渲染语义保留）。PlanBar 状态行语义（「已工作」）本卡不动。

---

## 3. DSH/ZCode 四原则逐条映射

| DSH/ZCode 原则 | 我方落点 | 现状差距（锚点） | 分期 |
|---|---|---|---|
| 1. **单活动面**：每回合一个流式活动区，一切活动按时间序内联；可见性绑定「回合有活动」 | 回合块扩展为回合单活动面：容器存在性=任一活动证据；item 步+轨迹步+思考行同面混排（§2.1） | 容器存在性=实验轨迹节点（`renderPlan.ts:111-113`）；item 活动落流底 lane（F2） | 分期一① |
| 2. **状态行=已工作时长**：锚定常驻，语义「已工作 xx 分」 | PlanBar 常驻锚（已达成，PLANBAR-1）；回合块 live meta 补「已工作 Xs」（§2.4-1） | 回合块 live meta 只有步数/--，无时长（F4） | 分期一③ |
| 3. **终局=一行可持久摘要**：坍缩为摘要行，刷新后仍在 | 终态坍缩回执行已有（autoCollapse+`traceMetaParts`）；缺持久层 → 回执行台账/分期二 durable 节点（§2.3） | 回执行随内存态一起消失（F3/F5，探针 traceIndexes 空） | 分期一② + 分期二 |
| 4. **等待=显式行**：驻留/等待续跑同面显式「等待续跑（已等 Xs）」 | 驻留显式行 + 终态双段句式（§2.4-2/3） | 驻留期零呈现，终态混示被读作总执行时长（用户手测：220+ 秒） | 分期一③ |

---

## 4. 现有组件取舍清单

| 组件 | 取舍 | 依据 |
|---|---|---|
| `composer/PlanBar.tsx` | **保留，本卡不动** | 任务级锚职责正确；PLANBAR-1 刚验收、用户确认正常。原则 2 已由它承担 |
| `trace/TraceBlock.tsx` 组件族 | **改造保留**：扩展 props（itemSteps/receipt 态），成为回合单活动面 | 原则 1/3/4 的承载者；B9 终局并入原块、M12 谓词、UI-FOLLOW 锚定全部复用 |
| `OptimisticTraceBlock` | **保留** | 发送窗口兜底；回合容器接管更快（turn.started 即可接管）后自然退场，`shouldShowOptimisticTrace` 谓词不变 |
| 活动线 activity-lane | **分期一收窄**（回合内 item 不再落 lane）；**分期二评估退役** | 原则 1 之下回合内活动应内联；lane 只剩无回合归属活动（上传等） |
| `trace/turnEventMeta.ts` | **保留，地位升格**：时长拆分唯一真源，live/驻留/终态三处消费 | CONT-STALL-1 成果；归约器零改动，只扩消费侧 |
| `trajectory.ts` 归约器 + `vit.observable_trajectory.v1` 协议 | **不动** | 协议与 B9 语义不变；回合容器在其上叠加，不侵入 |
| 回合终局收口 | **新增显式双轨规则**：实验回合（roundScoped）仍只由 `trajectory.turn.*` 终态收口（B9 既有）；非实验回合由 chat 域 `turn.completed/failed/stopped` 收口；settle_slice 按 M12 隐藏 | 纯 chat 回合没有 trajectory 终态事件，必须有自己的收口，否则永不满 live |

---

## 5. 分期

### 5.1 分期一：demo 冻结窗内最小改（全部 `agent/webui/src` 文件域，零 chat/协议改动）

| # | 项 | 对应候选/问题 | 文件域 | 粒度 |
|---|---|---|---|---|
| ③ | 驻留显式行 + 已工作时长 | 候选③ / 问题④ / 原则 2、4 | 改 `trace/TraceBlock.tsx`（每秒计时器+等待行+live meta）；消费 `turnEventMeta`（归约器不动）+测试 | **小** |
| ① | item 事件渲染为回合内进度步 | 候选① / 问题①② / 原则 1 | 新 `trace/roundSteps.ts`(+test)；改 `trace/renderPlan.ts`（谓词放宽+步混排）、`trace/TraceBlock.tsx`（itemSteps 渲染）、`App.tsx`（轮询接线+传参）、lane 去重（`turnGroups.ts` 或 App 过滤）+测试 | **中** |
| ② | 终局摘要行持久化 | 候选② / 问题③ / 原则 3 | 新 `trace/turnReceipts.ts`(+test)、新 `trace/TurnReceiptRow.tsx`(+test)；改 `trace/renderPlan.ts`（回执锚定）、`App.tsx`（终态写入 effect+水合读取）+测试 | **中** |

- **顺序建议**：③ → ① → ②。③ 最小独立可交付；① 是主结构；② 依赖 ① 的回合收口规则（哪些回合算终局、要不要写台账）。
- **冻结窗冲突检查**：三项全在 webui 渲染面，不触协议、不触 chat、不触服务端接口——与 demo 冻结窗无冲突，停止条件（只出分期一）不触发。
- **验收口径**（实现卡细化）：webui vitest 全绿 + 渲染面端侧烟测（E2E-WEBUI-1 渲染门）至少覆盖三个断言面：无实验回合的执行期出现回合块且 item 步内联；刷新后终局回执行仍在（活态缺失时台账水合）；驻留期出现显式「等待续跑（已等 Xs）」行。

### 5.2 分期二：答辩后全量

| 项 | 内容 | 文件域 |
|---|---|---|
| 服务端终局回执 | 回执行落 Project History 会话图 durable 节点（`message_kind=turn_receipt`），跨浏览器/设备；webui 台账降级缓存 | `agent/internal/chat`（另拆卡，与并行会话协调） |
| 回合壳服务端统一 | 非实验回合也发射回合壳事件族（`trajectory.turn.*` 扩展或新回合壳域），消灭跨命名空间拼接与 §4 双轨收口规则 | chat/agentloop（另拆卡） |
| 活动线退役评估 | 回合容器全覆盖后 lane 是否还有存在必要 | webui |
| 回合步回放审计 | 全回合步可回放（事件持久化的正式形态） | 服务端+webui |
| T9 联动 | App.tsx 拆分时 PlanBar/回合块信息层级复核（答辩反馈驱动） | webui |

---

## 6. 风险与边界（如实）

1. **渲染放宽 → 结算切片噪音回归**：M12 谓词（settle_slice 标记+活动足迹证据链+短寿命回退）逐字保留；实现卡的测试须含「settle_slice 仍隐藏」钉。
2. **item 步量爆炸**：live 滚动窗口（最近 ~N 步 + 总计数），终态坍缩后只显计数与时长；归约器每回合设上限。
3. **台账与气泡重复呈现**：回执行只承载状态/计数/时长，**不复述回复正文**（正文归气泡）；同键活态优先，去重以 conversation+回合键。
4. **台账丢失边界**：换浏览器/清缓存丢回执行（非语义内容）——可接受，分期二根治（§2.3 边界声明）。
5. **与 chat 域并行会话**：本设计与分期一零 chat 文件改动，事件消费只读；分期二两项必须另拆卡协调，不得顺手动。
6. **本卡零生产代码改动**：以上全部是设计；实现按分期另拆卡，逐卡走渲染面交付门。

---

## 附：锚点索引（勘察引用）

- `agent/webui/src/trace/renderPlan.ts:111-113`（渲染谓词=nodeIds.length>0+M12）、`renderPlan.ts` 全文（UI-FOLLOW-1/2 两级锚定+50ms 容差）
- `agent/webui/src/trace/TraceBlock.tsx`（TraceStep/OptimisticTraceBlock/`traceMetaParts` 驻留句式/autoCollapse 0.9s）
- `agent/webui/src/trace/traceDelivery.ts`（`shouldRenderTraceBlockForTurn` M12 证据链/settle_slice）
- `agent/webui/src/trace/turnEventMeta.ts`（item 足迹/startedAt/endedAt/residencyEndedAt/`turnDurationSplit`）
- `agent/webui/src/trace/turnGroups.ts`（`isUnboundActivity` 跨命名空间判 bound）
- `agent/webui/src/trajectory.ts`（`reduceTrajectoryEvents`/B9 roundScoped 终局并入原块/`trajectoryRoundKeyOfEvent`）
- `agent/webui/src/messageLifecycle.ts`（`reduceAgentEventActivities` transient 化+终局清退/durable 过滤）
- `agent/webui/src/App.tsx:404-493`（事件轮询/空闲门/conversationID 效应重放）、`App.tsx:4201-4400`（MessageStream 渲染+活动线）、`App.tsx:6330-6435`（`chatMessageFromAgentEvent`）
- `agent/webui/src/composer/PlanBar.tsx`（相位/收起时间表/chainLive）
- `agent/webui/src/historyScope.ts`（B9 症4 刷新轨迹消失的水合采纳路径）
- `docs/OBSERVABLE_TRAJECTORY_PROTOCOL_V1.md` §2（transient 哲学）/§4（事件命名空间=实验语义）
- 用户证据与四问题诊断：任务卡 `2026-09-13-TRAJECTORY-DESIGN-1`（本稿不重查）

---

## 7. 用户裁定（2026-09-14 定稿记录）

| 点 | 裁定 | 落点 |
|---|---|---|
| A 工具步骤露脸程度 | **②映射成人话**：item 步标题经映射表转中文人话（如「已完成 频率关系观察」）；映射表落 webui 域（`trace/stepLabels.ts` 或并入 roundSteps），未命中的标识符原样显示并计数（映射表是活表，先覆盖常见族：ccb 观察/mix_tick/trajectory/audition/approval） | 分期一① 增补验收 |
| B 等待行「继续」按钮 | **暂不加**：最常见的等待是「等你 A/B 判定」（解除方式=说判定结论，非「继续」）；先只做显示行，真实使用中出现「它在等、无可点」场面再补（增量小） | §2.4-2 维持显示行形态 |
| C 收据行 | **一行先做出来看**：点到头一行，展开明细属分期二 | §2.3 维持 |
| D 三面形态 | **维持「一条流+一个锚」**（回合块+气泡=同一条对话时间线的一体两面，UI-FOLLOW-1/2 锚定已在产；PlanBar=跨回合任务锚，对应 ZCode/DSH 的 todo 面板——成熟 agent 的通行分工：过程与结果合并成一条流、计划单独锚定）；收据行并入回复气泡头部=分期二可选项 | §2.2 增补参照注 |

等待续跑三场景（供实现与测试对照）：①欠收尾评估（说「继续」跑完欠账）②等 A/B 判定（说判定结论）③手动停止后恢复（说「继续」）。CONT-STALL-1/2 修复后切片预算型等待多数由调度器自动续跑，可见的「等待续跑」行应以 ①②③ 为主。
