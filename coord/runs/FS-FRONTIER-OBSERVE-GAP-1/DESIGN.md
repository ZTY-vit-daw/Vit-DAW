# FS-FRONTIER-OBSERVE-GAP-1 修法设计与拆卡建议（卡 156 设计稿）

- 前置阅读：同目录 [RESULTS.md](RESULTS.md)（两缺口归因+锚点清单+工件回指）
- 本文：三案对比（卡面指定：强制观察步 vs ledger 结转 vs frontier 并入用户指名轨）、FS 状态机语义影响、实现卡拆分建议与 §11 持久化兼容回归要求。交主管定稿拆卡。

## 0. 设计总则

1. **诚实边界不动**：blocked 族「永远合法出口」的契约是 147/148/149 一族的既定 posture——模型自报边界是诚实行为，不得被门化（A2 变体因此被否，见 §1）。
2. **agentloop 侧尽量零改动**：守卫（`free_state_reasoning.go:1395-1461`）、G6/G8 全部从 closure/loop 上下文**派生**允许轨集——只要 chat 侧把该进的数据放进 candidates/ledger，agentloop 自动跟随。
3. **内容盲红线的扩展解释须显式裁定**：closure 层红线针对 runtime **发明**领域内容；用户原话指名+机械名字解析属于意图搬运。此解释扩展单列为裁定点（§3-C4），未裁定前 C 案不落笔。
4. **观察供应非平稳**（RESULTS §① 供应不稳定性）：任何依赖「换个观察试试」的修法都不可靠；修复必须给指名轨一条**不依赖 mix 内容巧合**的通道。

## 1. 三案对比

### A 案：强制观察步（两个变体）

**A1=runtime 种子观察（推荐变体）**：`prepareFreeStateReasoningContext` 新循环分支（`free_state_reasoning_loop.go:277-292`）在 store 前，由 server 主动发一枚 G3 合格的 project 级 mix 扫描观察（`mix.multitrack_relationship`，或加 `mix.frequency_relationship`）写入新循环 ledger，loop 记 `seed_observation` 标记。

- 先例：容量评估已在模型 turn 前做确定性工程结构读取（`capability_routing.go:520-524`；`semantic_entry.go:101`「product runtime observes project structure and owns that decision after this turn」）——种子观察=同 posture 的新机制（server 主动 CCB 观察→ledger），不违入口分类器内容盲契约。
- 修什么：新循环不再空账本（G3 即满足、frontier 即有供应）。M2 二轮型与 F-gate 型都受益（有 ledger 至少让 G 门审计有对象、GATE PATH 指引有起点）。
- 不修什么：**不修缺口①**——种子观察的内容仍用户盲；若种子供应的 band 行恰不含指名轨，守卫照旧封锁。
- 代价：每循环 +1 观察延迟；种子视图选择=确定性裁定面（建议 multitrack 单发，频谱面留给模型 GATE PATH 走）。

**A2=结算门槛（否决）**：final gate 拒绝「零观察+零工具调用」的 blocked 族首轮结算。否决理由：动诚实边界契约（总则 1）；真环境缺陷时会把模型困死在弹回循环（实验已实证 4 连弹回后模型只会换话术重试）。146 的 F-gate 判读同样依赖「模型自报边界是合法出口」。

**FS 语义影响：低**（A1 不改状态词表、门语义、守卫；只新增 loop 字段与 server 侧行为）。

### B 案：ledger 结转（前驱→新循环）

落点：`free_state_reasoning_loop.go:265-270` switch 增加 case——persisted 循环存在但 **inactive** 时，新循环 `ObservationLedger` 由前驱 ledger 播种（走既有 `mergeFreeStateLedgers` :529 语义，不新造合并规则），loop 记 `carried_from_loop_id=前驱 LoopID`。

- **只结转 ledger，不结转 closure/frontier/candidates**：closure 是任务域假设——结转会把缺口①的用户盲 frontier 原样带进新循环（更糟）；且前驱终态本身往往就是该假设的否定结论。观察 receipts 是工程事实，可跨循环。
- 新鲜度：结转**不预滤**，G7 revision-bound 审计天然约束（工程 revision 变了旧 ref 自动失效）——门管新鲜度，结转管可见性。职责分明，不重复实现。
- 预算语义：结转 receipts **不占**新循环观察 checkpoint 预算（预算按循环计的既有语义保留）；audit 面在 `carried_from_loop_id` 可区分血统。
- 修什么：M2 二轮型（延续断层）直接修复——二轮不再从零开始，G3/G7 有旧证据可用，GATE PATH 指引有实底。
- 不修什么：冷启动首次（前驱不存在时仍空——由 A1 兜底）；不修缺口①。
- §11 面最大：ledger 语义从 per-loop 扩为带血统的 per-conversation——旧 runtime state 兼容必须做（§5）。

**FS 语义影响：中**——loop 生命周期边界语义扩展（终态不再是血统终点），但状态词表、门、守卫均不变。

### C 案：frontier 并入用户指名轨

落点：chat 侧 closure fold（`audio_closure_controller.go:1039` 邻域）——loop 携带 `user_named_target`（track id），fold 时若其不在任何 candidate 轨集，追加合成 candidate（`id="user_named:"+trackID`、`source="user_intent"`、track_ids=[该轨]）。

- **指名轨来源（C-A）**：入口分类器**不可用**——契约内容盲（`semantic_entry.go:96-102`）且 target_scope 词表只有 `none|current_selection|project_context`（:77），无指名槽位；扩分类器输出与既有契约直接冲突（146 §② 已裁定注入点不进入口分类器）。正确来源=**分类器之外的确定性名字解析**：prepare 时用 TOM/内核轨名对 userText 做机械匹配（含中文别名「人声/伴奏/鼓组」类；解析失败/多义 fail-open 到现状）。
- **agentloop 零改动**：守卫允许轨集（`free_state_reasoning.go:1400-1414`）、G6（`free_state_gate.go:316-327`）、G8（:457-470）、披露行（`ccb_model_prompt.go:191-226`）全部从 candidates 派生——合成 candidate 自动打通观察许可、G6 证据、G8 提案目标。选中机制也自洽：模型观察指名轨→`audioClosureSelectedCandidate`（:1435）按观察目标匹配即选中该 candidate。
- 修什么：缺口①本体——指名轨不再依赖 band 行巧合；F-gate 签名消除（A1/A3 复刻话术应能到达提案面）。
- 不修什么：缺口②（空循环仍可零观察结算）。
- 语义代价（需要裁定的两点）：
  - **C4-红线扩展解释**：closure 层 content-blind 红线原义=runtime 不发明 track/plugin/view 内容。用户原话指名+机械解析=意图搬运非发明。此解释**扩展了红线文档语义**，须主管（必要时用户）裁定认可后落笔。
  - **C5-G5 语义修订**：G5「frontier_established」的含义从「机器派生冲突候选存在」扩为「机器候选或用户意图候选存在」——`free_state_gate.go` G5 断言与 GATE PATH 文案（G5 行）需同步措辞；这是三案中唯一动「门审计对象定义」的。

**FS 语义影响：中高**（G5 定义修订+红线扩展解释；守卫/G6/G8 机制不变但允许集来源扩了）。

### 对比总表

| 维度 | A1 种子观察 | B ledger 结转 | C 指名轨并入 |
|---|---|---|---|
| 修复目标 | 冷启动空账本（缺口②半） | 延续断层（缺口②本体） | 指名轨封锁（缺口①本体） |
| 改动层 | chat（prepare+server 观察） | chat（prepare switch） | chat（fold+prepare 解析）+门文案 |
| agentloop 改动 | 0 | 0 | G5 文案行（`ccb_model_prompt.go`），机制 0 |
| FS 状态机语义 | 低 | 中（生命周期血统） | 中高（G5 定义+红线解释） |
| 依赖裁定 | 种子视图选择 | 无 | **C4 红线解释+C5 G5 措辞** |
| §11 兼容面 | loop 新字段×1 | loop 新字段×2+合并语义 | loop 新字段×1+candidate 新 source 枚举 |

## 2. 建议组合与优先序

- **组合=B+A1 同卡（修缺口②）+ C 独立卡（修缺口①，挂 C4/C5 裁定为前置）**。三案无互斥；B 与 A1 同函数族（prepare/new-loop 分支）合并实现最小。
- 优先序：**C 优先于 B+A1**——F-gate 直接压制提案面到达率（实验 3/8）且是 146 实验重排与 NL 旅程消解的硬前置；但 C 需要决策侧裁定，**B+A1 无语义争议可先行开工**。若主管只排一张，建议 C（裁定成本一次性付清后，模糊意图链的用户意图面才算接通）。
- 与在飞卡关系：157（REPLY-FACE-BOUNDARY-LAYER-1，L3 回复面）正交互补——157 修「blocked 被渲染成门代号」，本卡修「不该 blocked」；两者合入后 M2 二轮话术的用户面才完整。154/155（topoplan/writegate）在提案面下游，无文件域冲突。

## 3. 实现卡拆分建议（文件域+验收面）

| 卡 | 内容 | 文件域 | 验收面 |
|---|---|---|---|
| 156-IMPL-CARRY（B+A1） | 终态循环 ledger 结转（carried_from_loop_id）+新循环种子 mix 扫描观察（seed_observation） | `agent/internal/chat/free_state_reasoning_loop.go`（prepare/store 邻域）+种子观察 server 侧发起点（capability_routing.go 邻域或独立 helper）+单测 | 单测（结转往返/种子入账/G3 即满足/§11 旧状态加载）；真栈 chat 腿=M2 二轮话术复刻（二轮 ledger 非空、G3 satisfied、不再零观察结算）；端测边界声明按 AGENTS §5 |
| 156-IMPL-NAMED（C） | user_named_target 确定性解析+合成 candidate 并入+G5 文案同步 | `agent/internal/chat/audio_closure_controller.go`、`free_state_reasoning_loop.go`（解析捕获）、`internal/agentloop/ccb_model_prompt.go`（G5 行措辞）+单测 | **前置=C4/C5 裁定**。单测（指名并入/解析失败 fail-open/多义拒绝/G6-G8 联动/守卫放行）；真栈 chat 腿=A1/A3 复刻话术不再弹回、到达提案面；sealed 纪律=名字解析测试用 fixture 别名表，无硬编码轨名 |
| 156-REG-RESCHEDULE | 146 SOFTGUIDE 实验重排腿（L1+L3 合入后按 §E 原协议） | 驱动脚本/工件目录（非生产码） | 重跑判读面：F-gate 0 计数+提案面到达率（B2 判读预绑定规则已锁硬规则分支，重排主要验证到达率恢复） |

## 4. FS 状态机语义谨慎面与 T10 边界（停止条件细则）

1. **两份 ledger 不得统一 schema**：chat 侧 `loop.ObservationLedger`（持久、血统）与 agentloop 侧 `messageLoopFreeStateContext` 投影（只读视图）是两层。若 B 案实现中发现结转必须统一两者 schema 才能工作——**那是 T10 邻接面，停**：只做 chat 侧搬运，agentloop 继续经 context 消费。
2. **G5 修订只动措辞与候选来源，不动门数量/顺序**；G6/G8/守卫机制零改动。
3. **blocked 族出口永不禁**（总则 1）；种子观察失败（如观察系统不可用）fail-open 到现状空账本，不得阻塞循环启动。
4. 结转不复活终态：前驱 loop 状态保持终态记录，血统只体现在新循环的 carried 字段——不引入「循环复活」语义。

## 5. §11 持久化兼容回归要求（两实现卡共同验收项）

1. loop 新字段（`carried_from_loop_id`、`seed_observation`、`user_named_target`）旧态缺省语义=absent→无结转/无种子/无指名，行为与现状逐字节一致；
2. 旧 `agent_runtime_state.json`（free_state_reasoning_loops 无新字段）加载+序列化往返测试各至少一例（真实旧工件，非合成）；
3. 结转合并必须走既有 `mergeFreeStateLedgers`（:529）路径，不新造合并规则；上限（`freeStateObservationLedgerLimit=24`）行为需显式测试（结转+新观察溢出时旧 receipts 的淘汰语义）；
4. 合成 candidate 的 `source="user_intent"` 为新枚举：门侧未知 source **fail-closed**（不认作机器候选参与 G5），反序列化侧不 crash；
5. 名字解析测试数据=fixture 别名表（含 miss/多义/中文命中），不硬编码真实工程轨名（AGENTS §10 sealed 纪律）。
