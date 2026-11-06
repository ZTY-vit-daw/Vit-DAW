# CLOSURE-ROUTE-ADAPT-1 勘察腿产出：路由面取证 + 适配方案设计（读档零 LLM 零代码改动）

- 勘察基线：origin/main `a8be057ed772f9030ab148eb9a6b704d0f07e064`（worktree `D:/Vit_DAW_wt_271impl` @ `port/closure-route-adapt-1`）
- 证据来源：276 rerun1 工件（`D:/Vit_DAW_wt_276impl/coord/runs/EDU-SYMBOL-CORE-1/smokerun1/rerun1/`）+ 主仓代码锚点 + git 历史
- 上游裁定依据：[276 终验裁定](../../rulings/2026-11-07-EDU-SYMBOL-CORE-1-pass.md) §二三因素；[2026-10-30 判断权归属裁定](../../decisions/2026-10-30-judgment-ownership.md)（全文已读，边界标注见各案）
- 状态：勘察腿完成，**未动任何代码**；本文档所有「方案」均待主管裁定，不构成实现授权

---

## §1 路由面取证：semantic_entry LLM 分类 → minimal_audio_closure 武装入口判据链（6 环）

### 环 1：分类触发门（`goalrunner_chat.go:251-258`）

`classificationRequired` = 非 plan 模式 ∧ 非 durable continuation ∧ 非「继续」消息 ∧ 非 `free_state_diagnostic_only` ∧ 无活跃 free-state loop ∧ 无活跃 orchestration controller。新用户消息默认跑分类。

### 环 2：LLM 分类（`semantic_entry.go:133-173 planSemanticEntry`）

单次 LLM 调用（预算 12s 总额 + 一次 decode-repair 重试），schema `semantic_entry_decision.v1`，输出 route/target_scope/control_mode/user_authorization/confidence。**分类是概率行为**：276 rerun1 实证同一话术两轮输出 token 数不同（201 vs 168，telemetry 行 0），分类结果可不同。

### 环 3：路由语义与 controller 缺省分派（`semantic_entry.go:72-102` system prompt + `:279-290 defaultSemanticEntryController`）

| route | prompt 判据（:80-89） | controller（:281-286） |
|---|---|---|
| discussion | 想要解释/比较/推理/建议，**不请求观察**或工程变更 | OrdinaryConversation |
| observation | 想要 Agent **检查/评估证据**，不改变工程 | **MinimalAudioClosure** |
| explicit_control | 请求具体控制操作 | DirectTypedAction |
| open_semantic | 请求可听/声学结果，方法留给 Agent | **MinimalAudioClosure** |
| other | 声学仲裁外的普通请求 | OrdinaryConversation |
| unresolved | 置信 <0.50 或歧义 | none（澄清停轮） |

**判据摇摆点（276 根因）**：「分析这段 MIDI 的变换关系」同时满足 discussion（想要分析/推理）与 observation（想要检查/评估证据）的字面判据——两个 route 都是只读，但 controller 分派**完全不同**（OrdinaryConversation vs MinimalAudioClosure）。LLM 分类在这条边界上的非确定性直接表现为：同话术一轮走普通对话、一轮进 closure 治理路径。

### 环 4：容量路由无条件加固（`capability_routing.go:269-327 planObservationFirstCapabilityRoute`）

observation/open_semantic（:278）→ 观察工程结构事实 + 容量评估（:279-290）。三个出口：
- `SelectedCapability == projectMix` → ProjectMixWorkflow（:292-294）
- `capacityFactShortCircuit` 非空（AGENT-F3 窄门，见 §2）→ 直接作答，不建闭包（:295-301）
- **其余一律 `entry.Controller = MinimalAudioClosure`（:308，无条件）**

`promoteOpenImprovementEntry`（:363-371）只把「工程词+问题/改善词」（`openProjectImprovementIntent`，:373-383，**Go 词表**）的 observation 提升 route 标签为 open_semantic（进自动化轮），**不改 controller**。「分析这段 MIDI 的变换关系」不含问题/改善词 → 保持 observation 标签，但 controller 仍是 MinimalAudioClosure。

### 环 5：closure 武装入口（`audio_closure_controller.go:56-181 prepareAudioClosureContext`）

武装判据 = `orchestrationControllerDecision.Controller == MinimalAudioClosure`（:108-109）∧ verified semantic entry（:112-114，无则报错拒绝武装——**fail-open 安全**：semantic entry 服务失败时无 verified decision，closure 不武装，走普通对话）。模式分派：`entry.Route == observation → ModeDiagnostic`，否则 ModeTreatment（:116-119）。

### 环 6：模式身份双向映射（`audio_closure_controller.go:356-367`）

`ModeDiagnostic ↔ observation route ↔ observe_only authorization`；`ModeTreatment ↔ open_semantic ↔ action_requested`。**ModeDiagnostic 是 observation 路由在 closure 内的合法持久身份**（restore 时反推回同一 controller decision，:378-385）——「分析类请求进 closure」在现行设计下不是漏洞，是 observation 路由的既定投影。

---

## §2 路由面取证：「分析类请求」（只读分析、无音频编辑意图）归类现状

1. **无「只读分析」专用类**。六枚举中最接近的是 discussion（不进 closure）与 observation（进 closure ModeDiagnostic），判据边界如 §1 环 3——摇摆实证即 276 两轮。
2. **现存 Go 词表窄门（AGENT-F3，`capability_routing.go:190-236`）**：`capacityFactShortCircuit` 只放两扇门直接作答——①空工程逃生（TrackCount=0 且 revision 已绑定）；②纯计数/存在性事实问题（`capacityFactQuestion` :125-158：疑问词「多少/有没有/是否有/how many…」×结构名词「轨道/插件/分组/自动化/发送/时长」，且被 `capacityDiagnosticIntentMarkers`（:114-117：「问题/毛病/风险/浑浊/改善/优化/improve/fix…」）排除诊断改善意图）。注释明言："**Everything else still routes into the closure machine**"。「分析这段 MIDI 的变换关系」不是计数问题 → 不短路 → 进 closure。
3. **同类词表先例**：`openProjectImprovementIntent`（:373-383）用「工程+问题/改善词」识别可提升为 open_semantic 的观察请求；`explicitProjectMixInvocation`（`semantic_entry.go:175-187`）用固定词表识别 ProjectMix 显式调用。这些都是裁定前存在、裁定后保留为「路由/缓存识别级」的实例。
4. **fail-open 路径**（`goalrunner_chat.go:265-279`）：semantic entry LLM 不可用时，仅容量事实短路可作答（`semanticEntryUnavailableCapacityAnswer`，`capability_routing.go:227-253`，回答附「意图分类服务本次不可用」声明）；否则无 verified decision → closure 拒绝武装 → 普通对话。**LLM 分类失败不会误进 closure；LLM 分类成功但摇摆才会**。

---

## §3 路由面取证：closure 武装入口的意图过滤现状

- **准入侧没有只读/编辑意图的过滤**：武装判据只有 controller 类型 + verified entry（§1 环 5）。observation（只读）与 open_semantic（动作）都建闭包，唯一区分是 `Mode` 字段（Diagnostic/Treatment）。
- ModeDiagnostic 闭包的内部差异面：
  - authorization=observe_only（:356-361），source_route=observation（:362-367）；
  - **不启动 free-state 自动化轮**：`shouldStartFreeStateReasoningLoop`（`free_state_reasoning_loop.go:298-315`）要求 route==open_semantic ∧ control_mode==semantic_loop ∧ authorization==action_requested——observation 闭包永远不带自动化工具面/48-turn 大预算；
  - 预算被 clamp 到 1-2 turns（见 §4）。
- **工具可达性**：276 rerun1 卷内实证 closure slice 披露面**已含** `analyze_midi_transformations`（`agent_prompt_render.jsonl` 披露行）——工具看得到，模型 1-turn 内未调（86 tok 进度注记，零工具调用，telemetry 行 1：in 13937/out 86）。**失败因素不是工具不可达，是 1-turn 预算 × 模型单轮行为**。

---

## §4 预算面取证：`audio_closure_controller.go:529` MaxTurns=1 的设计来源

### 设计文本（现行 + baseline 逐字同源）

`audioClosureMessageLoopBudget`（:483-546）三分支：
1. **自动化轮**（`audioClosureAutomationRoundContext` :438-452：free-state loop 活跃 ∧ 非 diagnostic-only）→ 抬到 48 turns/80 tool calls（:478-481 常量，M3-LLM-TURN-BUDGET-1/229 + 232 用户裁定「per-slice limit 是接力棒不是边界」推导：链长 8-10 调用 ×~3 turn/调用 ×1.5 余量）；
2. **其余 closure 轮**：`base.MaxTurns = 1`（:529）；frontier 有候选且无 candidate_id → 2（:530-533，注释："Candidate selection is the one closure phase that needs two adjacent model turns: request the bounded target observation, then decide from the returned target evidence"）；terminal_turn_locked → ≥2（:539-541，BOUNDARY-1）；MaxToolCalls clamp ≤6（:542-544）；
3. clamp 注释自述设计假设："The 1-2-turn clamp below was **designed for the pure observation slice (one probe + one verdict)**"（:485-487）。

### git 历史（有意设计证据）

- `git log -S "base.MaxTurns = 1"` 唯一命中 **38ecd24b「baseline: capture C2 and G1 trajectory work」（2026-08-19）**——clamp 及其注释在该 commit 即逐字存在，基线期冻结设计，非近期回归。
- **92533b12（M3-LLM-TURN-BUDGET-1，229 卡）**：为自动化轮开 48/80 豁免时，提交信息明文「**诊断轮与非 FS closure 轮观察 slice 语义零回退**」——即 229 把「诊断轮保持 1-turn clamp」作为有意保留的契约。
- 测试钉：`TestAudioClosureBudgetDiagnosticRoundKeepsClamp`（`audio_closure_turn_budget_test.go:61-78`）显式断言 diagnostic-only 轮 MaxTurns=1/MaxToolCalls=6。

### 欠账形态证据（同一诊断语义，两个预算值并存）

`agentLoopBudgetForContext`（`goalrunner_chat.go:800-807`）：`free_state_diagnostic_only` 轮 base=**5 turns**/6 calls，注释："Catalog discovery, optional structure discovery, one rejected exact view set, one usable diagnostic bundle, and the terminal conclusion **can require five model turns**."。但 budget 叠加顺序（`goalrunner_chat.go:372-378`）是先取 base 再过 closure clamp——**closure 活跃时 5-turn 诊断预算被压回 1**。即：同一个「诊断轮需要多 turn」的语义，非 closure 路径承认 5，closure 路径给 1。这是预算面欠账的直接代码形态（非 closure 的 diagnostic_only 轮当前生产代码无写入点，flag 只从持久化 loop 行恢复——5-turn 分支主要服务 FS 诊断 replay 场景）。

### 276 rerun1 卷内事实（含对 rerun1 evidence.md 的一处修正）

- closure 实际状态（`chat_response.json` → `workflow_data.minimal_audio_closure`）：`mode=diagnostic`、`phase=fs2_capacity_assessed`、**hypothesis_frontier 空（candidates=0，无 candidate_id）**、rounds_started=1、goal_status=waiting_continue、stop_reason=limit_reached、durable continuation `cont_851ad5695b09374396f474cb` 落库 scheduler_eligible。
- **修正**：rerun1 `evidence.md` 称「frontier 有候选=2」，与卷内 JSON 不符（frontier 为空）。因此实际生效预算是 MaxTurns=**1**（:531 条件不满足），与卡面三因素表述一致；「有候选=2」是取证笔误，不改变结论，但实现腿写测试时须以卷内状态为准。
- 失败链完整复述：分析消息 → LLM 分类 observation（摇摆）→ 容量评估非短路 → controller=MinimalAudioClosure → ModeDiagnostic 闭包（fs2_capacity_assessed，frontier 空）→ clamp MaxTurns=1 → 模型 1 turn 产 86 tok 进度注记零工具调用 → limit_reached → waiting_continue 停泊。工具披露面含目标命令但未触达。

---

## §5 适配方案（3 案 + 利弊 + 判断权归属裁定边界标注）

> 边界总纲（2026-10-30 裁定）：确定性层正当域=**意图识别（路由/缓存）+ 执行链 + 安全兜底**；**判断**（问不问/问什么/提议什么/变体话术理解/工具选择）归 LLM，不得新增 Go 固定判断文案。终态=意图理解层 100% LLM、无词表绕道。

### 案 A（路由面，治本向）：observation 拆出「纯分析/诊断」子身份，派 OrdinaryConversation，不进 closure

**做法**：semantic entry 层面把「只读分析」与「观察+改善前奏」在 route 语义上分开（schema 枚举细化或 observation 增加子标签，如 `observation.analysis`），`defaultSemanticEntryController` 与 `planObservationFirstCapabilityRoute` 把纯分析身份派 OrdinaryConversation；ModeDiagnostic 闭包恢复链（§1 环 6 双向映射）保持兼容。

- 优点：分析类完全绕开 closure 治理轮与 1-turn clamp；与 276 **run1 成功形态**（普通 message loop + midi 能力词表披露 + direct invoke，smokerun1 已证健康）对齐；把 LLM 摇摆的两个出口（discussion/observation）合并为同一用户可见行为——**非确定性分类不再产生用户可见形态差异**，这是对「分类随机性」最彻底的消解；观察请求中需要真探测/闭包治理的仍留在 closure 内。
- 缺点/风险：route 语义重新划界动 semantic entry prompt 与消费方多处（closure mode 分派 :116-119、audioClosureSourceRoute :362-367、旧闭包 restore、capability route 迁移 `capabilityRouteSemanticEntry` :336-361）；旧持久化 observation 闭包的兼容面要单独测试。
- 🔴 **判断权边界触碰点（须裁定）**：
  1. route 语义的重新划界本身是**语义面改动**——「这条消息是纯分析还是改善前奏」的边界判断必须由 semantic entry LLM 在分类时作出（prompt/schema 层），若实现时用 Go 词表（如沿用/扩张 `openProjectImprovementIntent` 式判据）做「纯分析」识别 → **直接触碰裁定护栏 1（判断写死）**。确定性层只读 LLM 给出的 route 字段做 controller 分派（识别级，正当）。
  2. 现存 `openProjectImprovementIntent`/`capacityFactQuestion` 词表已处于「识别/判断」灰区边缘，本方案**不得**以扩张这些词表的方式实现。
- 停止条件关联：若主管裁定 route 语义属判断权归属裁定域不可动 → 本案转记录边界，不实现。

### 案 B（预算面，最小修，★建议案）：诊断闭包轮的 1-turn clamp 修正为承认诊断轮基础预算（5 turns）或不低于 2-3 turns

**做法**：`audioClosureMessageLoopBudget` 对 ModeDiagnostic 轮（或「非自动化轮中的诊断轮」窄面）不再压回 1：保留 `agentLoopBudgetForContext` 给出的 base（diagnostic_only=5；普通 goal base=10 中取窄面值），MaxToolCalls 维持 ≤6 或按诊断链放宽到与 base 同源；续跑片经同函数继承（229 side 2 既有契约，:510-513 注释已声明续跑片同源）。

- 优点：**零判断权边界触碰**——预算是执行层（确定性层正当域），M3-LLM-TURN-BUDGET-1/232 已有同面先例（automation 轮 48/80 就是预算面适配，词表零参与）；直接消除 276 失败链的死因环节（1-turn × 单轮无调用 → limit_reached）；有现成推导锚点（`agentLoopBudgetForContext` 诊断轮 5-turn 注释，同语义同值即消除「1 vs 5 欠账形态」）；对 treatment 轮与自动化轮零影响（窄面收窄）。
- 缺点/风险：分析请求仍在 closure 治理路径内（合同/admit/no_progress/轮边界开销仍在）——治标不治路由；若模型单轮行为面就是「产注记不调工具」（276 形态），放宽预算只把停轮点后移，不保证成功（但 durable continuation + 续跑兜底已在，用户「继续」可走完，见案 D）；**与测试钉 `TestAudioClosureBudgetDiagnosticRoundKeepsClamp` 冲突**——修改须以 276 卷内证据为「原 clamp 假设在新形态下失效」的依据改测试意图（one probe + one verdict 的单轮假设被 276 实证打破：模型单轮未完成任何 probe），不得静默改期望值。
- 🔴 **判断权边界触碰点**：无——前提是实现保持**结构性判据**（mode/phase 字段，:438-452 注释明言 "The detection is structural …, never a phrase judgment (judgment-ownership ruling: deterministic layer recognizes, the LLM judges)"），不得引入话术判断来决定预算。

### 案 C（工具面/意图面，兜底向）：observation 闭包轮内分析命令准入（进了 closure 也能走分析命令）

**做法**：为 ModeDiagnostic 闭包轮在工具面准入（agentloop `admitFreeStateDecisionRoundToolSurface` 同构面）放行分析类只读命令（如 midi.analyze_transformations），分析结果按观察证据入账。

- 优点：不动路由与预算语义；分类摇摆仍在但两个出口都能完成分析；分析证据进 closure 证据链（与 DAD/audioclosure 证据层对齐）。
- 缺点/风险：227 卡工具面准入是结构定案（active FS loop + 非诊断专用），给 diagnostic-only 轮开工具面**触碰该定案的边界**，且按「工具面与预算同源」原则（:492-494 "tool surface and turn budget stay sourced identically"）开工具面必须同源开预算——实际=案 C ⊃ 案 B 的改动量；observation 路由不启动 FS loop，需为 diagnostic closure 轮新建准入判据，新增 Go 结构判据有固化引力；改动面在 agentloop 深层，超过「最小修」定位。
- 🔴 **判断权边界触碰点**：准入判据若需区分「这轮该调分析工具还是观察工具」→ 该选择属 LLM 判断权（裁定表格「变体话术理解与工具选择」行），确定性层只做披露（把工具放进披露目录让模型自选）才合规；若实现成 Go 判据替模型选工具 → 触碰裁定。

### 案 D（驱动面，卡面可选腿，非根因方案）：waiting_continue 有界续跑腿

烟测驱动对 waiting_continue 停泊发一条「继续」走完 continuation（276 卡可选后续①）。不消除根因（每次分析类误路由都要多一轮续跑），仅在 A/B 落地前作为验证腿便车；若案 B 落地后仍有模型单轮注记不调工具的形态，续跑腿是兜底逃生门。实现域=冒测驱动脚本，不碰 chat 内核。

### 案别建议

**主推案 B（预算面最小修）**：证据最硬（1 vs 5 欠账形态可代码级复述）、零判断权边界触碰、改动面最小（单函数窄面+测试意图修订）、与 229/232 既有预算适配同构。**案 A 作为中期治本案一并提请裁定**（route 语义划界的授权在主管；裁定通过则 A 与 B 可叠加——A 消解路由摇摆的用户可见差异，B 修复留在 closure 内的诊断轮预算欠账）。案 C 仅在 A/B 均被裁否后作为兜底重估。案 D 独立于主案按需挂验证腿。

---

## §6 裁定请求清单（交主管）

1. **案别裁定**：是否按「B 先行 + A 提请裁定」推进？（B=预算面最小修；A=observation 拆纯分析子身份派 OrdinaryConversation）
2. 🔴 **判断权边界裁定（案 A 前置）**：semantic entry route 语义重新划界（新增「纯分析/诊断」身份或 observation 子标签）是否属于 2026-10-30 裁定域内允许的「LLM 分类语义修订」？若允许，确认实现约束：分类由 LLM 作出，确定性层只消费 route 字段；禁止新增/扩张 Go 词表做「纯分析」识别。
3. 🔴 **测试意图修订裁定（案 B 前置）**：`TestAudioClosureBudgetDiagnosticRoundKeepsClamp` 钉死的「诊断轮 1-turn clamp」契约，是否认可以 276 卷内证据（模型单轮注记零调用 → limit_reached → 命令面未触达）为依据修订为「诊断轮承认基础预算」？此为对 229 提交信息「诊断轮零回退」契约的有意修订，需留痕。
4. **276 尾巴收口方式**：案 B 落地后，276 rerun1 遗留的 durable continuation（`cont_851ad5695b09374396f474cb`）与「分析话术真栈验证」是否搭下一轮真栈 run 便车（卡面验证腿口径），或由案 D 续跑腿覆盖？
5. **evidence.md 笔误处置**：276 rerun1 evidence.md「frontier 有候选=2」与卷内 JSON（frontier 空）不符——是否需要在该卡工件目录追加勘误注记（本卡不越域改他卡工件）？

## §7 勘察腿自查

- 零代码改动（本腿 diff 仅 coord 工件与卡面回执）；零 LLM 调用；未触碰 sealed/holdout fixture；未运行任何测试/构建（纯读档）。
- 代码锚点均在基线 a8be057e 工作树核过；git 历史检索：`-S "base.MaxTurns = 1"`（1 命中）、`-S "candidate selection"`（FIX-FRONTIER-FOLD-1 等，与本卡无冲突）、`-S "one probe + one verdict"`（92533b12）。
- 判断权裁定文档（2026-10-30 全文）、276 终验裁定 §二 已读并纳入边界标注。
