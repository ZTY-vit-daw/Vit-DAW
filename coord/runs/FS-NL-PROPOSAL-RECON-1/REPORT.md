# FS-NL-PROPOSAL-RECON-1 勘察报告（JOURNEY-3 止损上交产物取证）

- 执行：PC 执行侧（值班会话），2026-10-07 20:42–21:1x，HEAD=f47172de（领取 commit；领取时 origin/main=a42c87e1，主树 main 单流）
- 卡：`coord/cards/done/2026-10-07-FS-NL-PROPOSAL-RECON-1.md`
- 性质：纯取证，零生产代码改动。本目录为唯一写入面（外加卡片状态变更）。

## 0. 结论速览

**R3 的缺口不是"模型没给结构化提案"，而是"模型给了结构化有效提案，被 G6/G8 准入门拒绝后，capability_blocked 状态没有表面化——响应面把模型的散文终答当成功终局投递"。**

| 项 | 结论 |
|---|---|
| H1 入口协议缺口 | **排除**（prompt 面强约束在位，且 R3 receipt 证明模型确实返回了结构化提案） |
| H2 协议修复面漏拦 | **采信（主因）**——机制精化：漏拦点不在 messageLoop 弹回（那层在位），在 gate 拒绝后的响应路由（capability_blocked 落 plain 分支，散文终答被当 done 投递） |
| H3 模型概率行为 | **采信（触发器）**——G6/G8 引用一致性是模型概率面；但严重度（治理面零挂出+成功形终答）由 H2 决定 |
| 采信序 | **H2 + H3 复合 > H1（排除）** |
| 抖动率 | N=4 机制分解：成功 1、gate 拒绝形 1、预算耗尽形 1、驱动缺陷（已修）1；纯模型分支约贡献 2/4 的触发概率，但其中 gate 拒绝形有确定性兜底空间 |
| 修复卡 | **建议立卡（P2）**：修复面兜底腿为主（改动面≈1 个分支）；nudge 重试腿可选；prompt 措辞腿收益不确定 |

## 1. 证据清单（全部亲读）

### 1.1 运行工件

| 轮 | 目录 | 关键工件 |
|---|---|---|
| R3（失败·主样本） | `coord/runs/JOURNEY-3/20261007_201400/` | journey_nl_summary.json、journey_nl_events.json（11 事件全文）、journey_nl_chat_1.json（workflow_data.free_state_reasoning_loop）、journey_nl_runtime_status_final.json（task_semantic_state.history + continuations[].free_state_admission_receipt） |
| R2（失败·对照） | `coord/runs/JOURNEY-3/20261007_200603/` | 同构工件；无 admission receipt |
| R1（驱动缺陷·已修） | `coord/runs/JOURNEY-3/20261007_194840/` | receipt=admitted |
| R4（成功·决策侧复跑） | `coord/runs/JOURNEY-3/20261007_202322/` | receipt=admitted 全门过；mix_tick 卡→hop→applied→audition 全链 |

### 1.2 代码锚点（只读核对）

- `agent/internal/chat/improvement_proposal_workflow.go:22-199` — improvementProposalResponse：挂卡/自动授权路由的完整条件链
- `agent/internal/chat/free_state_reasoning_loop.go:800` recordFreeStateDecision；`:959` applyFreeStateDecisionSemantic（语义转换先于 gate 判定）；`:1013-1121` needs_experiment/improvement_proposal 分支（G 门审计、FS7 推进、capability_blocked 出口）；`:1054-1077` gate 拒绝→capability_blocked
- `agent/internal/chat/task_semantics.go:127-199` — 语义转换映射（rev2/rev3 的 reason 串唯一来源：`:169/:174`）
- `agent/internal/chat/goalrunner_chat.go:463-512` — **响应路由门（本缺口的核心漏拦点）**：`:474-477` awaitingAction/awaitingExperiment 判定、`:478-488` plain 分支、`:508` improvementProposalResponse 调用
- `agent/internal/agentloop/free_state_gate.go` — G1-G8 全实现（G6 `:311-364`、G8 `:443-527`、审计入口 `:79-92`）
- `agent/internal/agentloop/free_state_reasoning.go:1041-1157` — messageLoop in-loop 门与 bounce；`:1184-1186` diagnostic_complete 不能结算 improvement 合同
- `agent/internal/agentloop/ccb_model_prompt.go:36,100,108,121,138-143,312` — prompt 面的强约束指令与 GATE PATH 预披露
- `agent/internal/chat/turn_trajectory_events.go:326-378` — emitSchedulerChainResultEvent（settle_slice 终局事件投递）

## 2. R3 因果链（逐步，带工件行证据）

时间轴（R3，conversation=dev_journey_fs_nl_20261007_201404）：

1. **20:14:19** turn.started（NL 话术入栈，goal_471bd3e2bfe61a39）。
2. **20:14:25** 第 1 轮：ccb.observation_catalog → turn.completed stop=limit_reached，waiting_continue（chat_1.json: checkpoint_status=waiting_continue, continuation cont_096999…）。
3. **20:14:31** 续算第 2 轮：ccb.observation_request [project.structure, mix.multitrack_relationship, **mix.frequency_relationship**]（工程级）→ obs_20261007T121432 status=ready。
4. **20:14:42** 第 3 轮：ccb.observation_request [track.timbre_frequency] target=track 1012（Lead Vocal）→ obs_20261007T121443 status=ready。
5. **20:15:07.376/.389** settle 轮 LLM 返回 free_state_decision{status∈{needs_experiment,improvement_proposal}, improvement_proposal={proposal_c06e0c3f…, target=track 1012, requires_experiment=true}}——语义转换 rev2 `diagnostic_completed`("candidate diagnosis established") + rev3 `improvement_proposed`("runtime admitted bounded improvement proposal")（task_semantics.go:169/174 的 reason 串，唯一来源）。
6. **20:15:07.402** `free_state_admission_receipt`（journey_nl_runtime_status_final.json → continuations）：
   ```
   boundary=admission_gate_failed  status=capability_blocked
   failed_gate_ids=[G6_target_evidence, G8_target_consistency]
   其余 6 门全 pass；proposal_present=true  proposal_valid=true
   target_evidence_ref=obs_20261007T121443
   ```
   → recordFreeStateDecision 走 `:1054-1077` capability_blocked 出口（stop_reason=free_state_admission_gate_failed）：无 FS7 推进、无 pending candidate upsert、loop.Status=capability_blocked。
7. **20:15:07.4xx** 响应构建（goalrunner_chat.go:474-488）：`awaitingAction=false、awaitingExperiment=false`（loop 已 blocked）→ **plain 分支** chatResponseFromAgentLoopResult：resp.Reply=模型散文终答（"我准备对主唱轨做一次有界实验：约 350Hz / -3.0dB / Q≈1.0 …执行后请直接 A/B 试听…"）、goal_status=completed、stop_reason=done。**improvementProposalResponse（:508）从未被调用。**
8. **20:15:07.440** turn_close_guard 记录语义 rev4 `owner_turn_closed`（reason="chat turn closed without governed experiment"）。
9. **20:15:07.477** emitSchedulerChainResultEvent 投递 turn.completed（item_id=chain_result, turn_kind=settle_slice）——治理面零卡：audition=0 / mix_tick=0 / applied_like=0（11 事件中）；探针分类 no_candidate_found。

要点：模型散文终答**承诺了一个永远不会执行的实验**（"执行后请直接 A/B 试听"），而系统把它当成功终局投递——这是比"没提案"更坏的形态：用户面成功、内核面 blocked。

## 3. R2 因果链（治理面同形、机制不同形）

- 语义态终局 rev1 `observation_in_progress`——**从未出现诊断/提案转换**；runtime_status 中无 admission receipt（模型从未返回提案决策）。
- 事件链：catalog → 工程级扫描 → 人声轨 basic_energy → 伴奏轨 basic_energy（连续 3 轮全 needs_observation，每轮决策合法）。
- continuations：7 行全 status=completed，free_state_continuation_used=**7** / budget 6（used 超预算的计数口径待考，如实记录）；终态 goal completed，settle reason=goal_terminal，探针分类 `other_terminal:limit_reached`。
- 机制定性：**观察预算耗尽形**——模型概率行为（连续选择观察），非协议缺口。prompt 面已有 observation saturation 提示（ccb_model_prompt.go:533）。

**修正卡面前提**：卡面称"R2/R3 同形终态"——治理面同形（无卡、goal completed），但内部机制**不同形**（R2=从未提案；R3=提案被 G6/G8 拒）。修复面兜底只救 R3 形，不救 R2 形；R2 形的缓解要么提预算、要么在预算将尽时注入"最后一轮必须给终局决策"的强指令。

## 4. R4 成功对照（决策侧 20261007_202322）

- 观察序列：catalog → [project.structure, multitrack, **masking_relationship**]（工程级，partial）→ [track.basic_energy, **track.timbre_frequency**]（轨道级，ready，obs_20261007T122411）。
- settle 决策 needs_experiment + 提案 evidence_refs=**[obs_20261007T122411]（只引目标轨观察）** → receipt boundary=**admitted**（8/8 门 pass，status=needs_experiment）。
- 语义 rev4 `experiment_required`（"free-state experiment runtime admitted"，experiment_id=turn:free_state_941daf021bf411ff）→ improvementProposalResponse → full_access 自动路由 → 原生域 static_eq → mix_tick.pending×2 → hop 确认 → trajectory.intervention.applied → audition 链（prepare.started→candidate.ready→ready×2）→ ab_card=True，success_strict/policy 双 True。
- R1 同为 admitted（失败是 chain_stall 驱动缺陷，v2 nudge 已修，不计入本缺口）。

**R3 vs R4 唯一分叉**：settle 提案的准入门判定。R3 引用了两个 obs（工程级+轨道级）且 G7（引用解析）过了、G6/G8（前沿候选×目标级证据一致性）双双失败；R4 只引目标轨观察、全过。G6/G8 共同失败的签名与"审计上下文中候选前沿未折叠/不含 1012"一致（G6 `allowedTracks` 空 + G8 断言 1 `target_not_from_frontier_candidate`，而 in-loop 态有 `recentObservation` 回退、审计入口 `AuditFreeStateNeedsExperimentGate`（free_state_gate.go:88）构造的 runState **恒无 recentObservation**——两类入口可判定不一致）。此层为机制推断（内部前沿快照未留痕），不影响 H 裁定：无论 G6/G8 因引用措辞还是因折叠时序而拒，**拒绝后的响应面行为都是同一个缺口**。

## 5. 三假设裁定

### H1 入口协议缺口 —— 排除

证据：
1. R3 receipt `proposal_present=true, proposal_valid=true`：模型在 settle 轮**确实返回了** improvement_proposal.v1 结构化提案（行为证据，强于日志）。
2. prompt 面强约束在位（ccb_model_prompt.go:36）："This is an open improvement contract … You MUST NOT return satisfied. After observation, return needs_experiment with one bounded evidence-backed improvement_proposal…"，附完整 JSON 形状（:100, :138-143）与 GATE PATH 预披露（:312："A proposal whose evidence is not yet in the ledger will be refused by the gate with a machine-readable structured gap; walk the GATE PATH first"）。
3. messageLoop 终轮门（free_state_reasoning.go:1184-1186）把 diagnostic_complete 弹回："convert the confirmed diagnostic into needs_experiment with one bounded improvement_proposal…"——散文式终答若想以 diagnostic_complete 收轮，协议层不允许。

局限（如实申报）：R3 实际 LLM prompt 体**未留痕**——`VitApp/Workspace/Logs/agent_last.log` 为 INFO 级、无 prompt 体，且已被 R4 覆盖（现存 48KB 为 R4 时段）。但 receipt 的 proposal_present/valid 与 R4 行为对照已足以裁定 H1。

### H2 协议修复面漏拦 —— 采信（主因，机制精化）

原假设"messageLoop 的终轮门对散文式提案终答无兜底"不完全准确：
- messageLoop in-loop 门**有**兜底（free_state_reasoning.go:1114-1155）：gate 失败会 bounce 并附结构化 gap 反馈（"return needs_observation with the next bounded observation instead"）。
- 真正的漏拦在 **server 侧 gate 拒绝后的响应路由**：recordFreeStateDecision 把 loop 置 capability_blocked（:1054-1077）后，goalrunner_chat.go:474-488 只路由 awaiting_action/awaiting_experiment 两种 loop 状态——capability_blocked 落入 plain 分支（chatResponseFromAgentLoopResult），投递模型散文终答、stop=done、goal completed。capability_blocked 边界**零表面化**：无边界回复、无卡、无 nudge；free_state_improvement_proposal_missing 分支（:1059）也未触发（提案在场，拒因是 admission_gate_failed）。
- 次级缺口：语义面先应用了 rev3 improvement_proposed（applyFreeStateDecisionSemantic 在 :959 先于 gate 判定运行），53ms 后又 owner_turn_closed——审计留痕诚实，但探针把 R3 分类为 no_candidate_found（**实际有候选提案**，分类与事实不符）。

### H3 模型概率行为 —— 采信（触发器）

- G6/G8 的一致性判定输入（提案 target/引用 vs 前沿×账本）由模型输出与折叠时序共同决定，同 prompt 同代码 R4 全过、R3 双拒——触发是概率性的，R4 证据使 H3 先验升高的判断成立。
- 但 H3 单独不构成完整根因：若修复面在位，R3 形会被 bounce/边界表面化（显式失败可重试），而不是静默变成"成功形终答"。**严重度由 H2 决定，发生概率由 H3 决定。**

## 6. 抖动率评估（N=4，机制分解，不做区间估计）

| 轮 | 形态 | 机制 | 修复面兜底可救？ |
|---|---|---|---|
| R1 | 链停 | 驱动缺陷（已修，v2 nudge） | — |
| R2 | 无提案·预算耗尽 | 模型连续 needs_observation | 否（需预算/指令腿） |
| R3 | 提案被拒·静默成功形 | H3 触发 + H2 放大 | **是**（bounce/边界卡/nudge 任一） |
| R4 | 全链成功 | — | — |

- 聚合成功率 1/4=25%。样本量不足以给置信区间；机制分解显示 2 份失败（R2/R3）各有确定性缓解路径。
- 判断（供决策侧）：当前形态**不可接受**——R3 形的失败不是"没成功"，而是"失败被包装成成功"（散文承诺实验+goal completed+零卡），违反 §7"不得把链路进度写成端到端完成"的验收精神，且用户面无法区分。
- 若仅落修复面兜底（建议 1），R3 形从"静默成功形"变为"显式 admission 拒绝边界"（可重试/可诊断）；若加 nudge 腿（建议 2），有概率直接转为成功形。R2 形需单独考虑。

## 7. 修复建议（按成本升序，建议 1+可选 2）

1. **修复面兜底腿（推荐，最低成本）**：`goalrunner_chat.go:478` plain 分支前，对 `loop.Status=="capability_blocked"` 且 LatestDecision.StopReason ∈ {free_state_admission_gate_failed, free_state_improvement_proposal_missing} 的情况返回显式边界响应（capability_blocked 边界回复/卡，附 receipt 的 failed_gate_ids），不投递模型散文终答。改动面≈1 个分支+文案；不碰 G 门语义、不碰 prompt。验收：journey 烟测中该形态被分类为 admission_rejected/blocked 而非 no_candidate_found/goal completed。
2. **nudge 重试腿（可选，中成本）**：对 admission_gate_failed 的 settle 结果追加一次带结构化 gap 的 nudge 重试（复用 freeStateNeedsExperimentGateFailureMessage 模板，消耗 1 continuation 预算，限 1 次）。收益：R3 形直接转成功形；风险：与现有 anti-abuse 指纹锁的交互需过一遍测试。
3. **prompt 注入点（辅助，收益不确定）**：GATE PATH 块（ccb_model_prompt.go:312）已有；可补一句"improvement_proposal.evidence_refs 必须逐字引用目标轨观察的 observation_id，target.id 必须是该观察的 track id"。但 R3 的确切拒因（引用措辞 vs 前沿折叠时序）未定论，措辞腿收益无法预估。
4. **话术调整：不需要**——同话术 R4 走通，话术不是瓶颈。

**是否立修复卡：建议立（P2）**，主腿=建议 1，可选腿=建议 2；文件域 `agent/internal/chat/`（goalrunner_chat.go + free_state_reasoning_loop.go 边界文案）；锚点已在 §1.2 列全。附带给修复卡的两个待办：(a) 探针分类修正（no_candidate_found→区分 admission_rejected）；(b) 可选 DEBUG 级 prompt/审计上下文留痕开关（本次取证的盲区根因）。

## 8. 停止条件核查与缺口清单

- 卡面停止条件"工件不足以下结论（如终轮 prompt 面未留痕）→ 缺口清单上交"：**未触发**——receipt+语义 history+事件流+代码锚点已闭环定因；但 prompt 体不留痕本身列为取证盲区缺口（§5-H1 局限、§7-4b）。
- 受控复现：**跳过**（卡面条款"若 1/2 已定因则跳过"已满足；且复现无法区分 R3 内部"引用措辞 vs 前沿折叠时序"——两者都不留痕，复现只会再采一个概率样本）。

## 9. 边界声明

- 本卡零生产代码改动；git 工作树中 `VitApp/Workspace/Settings/Settings.xml`、`VitApp/Workspace/default_project.xml` 的改动为领取前运行时状态，未触碰。
- 真栈未启动（取证不需要；泊位 5555/7878/5556 未占用，未申报监听）。
- R2 的 `free_state_continuation_used=7 > budget 6` 计数口径未深究（次要现象，如实记录）。
