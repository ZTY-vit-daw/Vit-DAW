# M3-FS-GATE-EVIDENCE-WIRE-1：FS 准入 G6/G8 证据接线+capabilityboundary family 断线+taskstate 幂等重入（229 验收门第五承，三合一修复）

- 发卡：GLM 主管决策流 / 2026-10-31（依据=232 验收定案+用户裁定「发 233 三合一修复卡」：两生产缺陷主管独立亲证——A=G6/G8 对在场的 target_evidence_ref 判不可用+capability_boundary family=None 断线（g1_r1 回执亲读：G1-G5+G7 全 pass+proposal_present/valid=true 仍 fail；工具名匹配面已排除低级错误）；B=taskstate capability_blocked→capability_blocked 幂等重入被拒→goal failed（state.go:515 亲核）。模型行为面五卡零反例，G1-G5+G7 全过=只差最后一道接线的门）
- 派发确认：已确认（用户 2026-10-31）
- 验收负责人：GLM 主管决策流
- 池序 233；目标仓库=**主仓 agent agentloop/capabilityboundary/taskstate 域**；P1 / 0.5-1 天 / 依赖=232 已合入（e32de775）✓；L2 / PC 执行会话（准入语义+状态机判读）

## 标的（232 定案，不重考）

G6（frontier 候选轨覆盖+可用轨级观察）与 G8（提案目标∈frontier+ledger 同轨绑定）对在场证据误判不可用：target_evidence_ref=obs_…非空、proposal_valid=true 仍 fail，frontier 折空 blockers=[G6,G8]；capability_boundary 输入 family=None（提案为音量自动化，家族字段应解析为 volume 族）→family_unresolved+certification_entry_unavailable+no_loaded_instance 三连锁全因断线；taskstate 重复 settle 同型终态被 invalidTransition 拒绝把 goal 标 failed。锚点：`agent/internal/agentloop/free_state_gate.go:312`（gateG6）/`:444`（gateG8）、`agent/internal/agentloop/free_state_reasoning.go:2687`（CCB 观察名匹配——已排除）、`agent/internal/capabilityboundary/resolver.go:18`（unavailable 渲染，输入断线在上游）、`agent/internal/chat/free_state_reasoning_loop.go:3602`（chat 侧 gate_results 审计表）、`agent/internal/taskstate/state.go:515-517`（invalidTransition）。232 run=coord/runs/M3-AUTOMATION-SLICE-BUDGET-OPEN-1/slicebudget_1（失败轮结构化回执在卷，g1_r1/g1_r3/g2_r1-r3 皆有 admission receipt）。

## 目标

1. **A1 诊断+修复（先诊断后修，断点成文）**：以 slicebudget_1 g1_r1 的实际回执形态为 fixture 复现 G6/G8 fail，钉死精确断点——候选 A：`usableTargetTrackID` 对观察 summary 的形状要求（status/target_ref 形态）与实际 CCB 观察回执不匹配；候选 B：frontier 候选的 track_ids 覆盖与提案目标轨（1010）不对账。断点+修复+单测（用真实回执形状做往返，不得为过门改测试期望）。**红线：G6/G8 的 fail-closed 语义不弱化——修的是接线/对账，不是删检查**。
2. **A2 family 断线修复**：勘察 `capabilityboundary.Resolve` 的 `Input.Family` 来源链，从提案（音量自动化→volume 族）正确解析填充；family 有值后 unavailable_reasons 三连锁应收敛为真实语义（有同族候选则给候选，无则如实 unavailable）。单测：提案→family 解析往返。
3. **B taskstate 幂等重入**：同型终态重复 settle（capability_blocked→capability_blocked）改为幂等 no-op（成功返回，不产生新转移），**非同型的转移表语义零改动**；单测：重复 settle 往返+异型转移仍拒绝。
4. **重验腿（本卡验收门，229 原口径第五承）**：组 1「这段让它越来越突出一点」——**组 1 no_route 消失+set_points 确认链（确认卡→确认→写入→get_curve 读回）首次端到端全绿**；组 2/3 按 220 口径跑满。232 的仪器面（等待窗/确认卡驱动/in_progress_timeout）已就位，判定逻辑零改动。

## 文件域

`agent/internal/agentloop/`（free_state_gate.go/free_state_reasoning.go）+`agent/internal/capabilityboundary/`+`agent/internal/taskstate/`+`agent/internal/chat/free_state_reasoning_loop.go`（审计表同步）+测试。scripts 零改动。越域即停上交。

## 安全面（硬约束）

- G6/G8 fail-closed 语义、治理门、每轮一 mutation 限速、MaxClosureRounds、223 stamp、232 预算 floor 零改动；
- taskstate 仅同型幂等重入放行，转移表其余语义零回退；
- §11：taskstate 若涉持久化格式，旧工件缺省语义+往返测试。

## 停止条件

- 诊断发现 G6/G8 语义要求 automation 族结构上无法提供的东西（设计意图如此）→锚点+两案上交设计裁定（不得强改过门）；
- **断点=模型自主选择的目标轨与确定性 frontier 提名轨集不一致**（判断权归属设计问题：目标选择权在 LLM 判断还是确定性扫描——触及 decisions/2026-10-30-judgment-ownership.md）→**停止不动语义**，锚点+形态数据+两案（A=frontier 提名逻辑扩展纳入模型提案轨=确定性侧让渡；B=指引面引导模型在 frontier 提名集内提案=LLM 侧约束）上交用户裁定，执行会话不得自行择案修；
- B 修复与状态机不变量冲突→两案上交；
- 修复后组 1 证据齐（G1-G8 全过+确认卡弹出）但确认驱动/写入/读回任一环失败→write_fail 如实锚点上交；
- 全修后组 1 仍 capability_blocked/no_route 三连→**模型能力面终局上交**（五卡证据链+本轮全量工件，转模型/指引设计讨论，不再折卡）；
- LLM 面栈不可用→环境中断如实记录。

## 领取

- 领取会话：PC 执行会话（GLM-5.3，ZCode）/ 2026-10-31
- worktree：`D:\Vit_DAW_wt_233impl`，分支 `port/m3-fs-gate-evidence-wire-1`
- 领取基线：`git rev-parse HEAD` = `e50a2ae91c76ae97d0110cf042bafaa54baaca52`（=origin/main）；`git status --short` 空；`git diff --stat` 空（干净起点）

### A1 断点诊断（先诊断后修，g1_r1 fixture 亲读成文）

以 `coord/runs/M3-AUTOMATION-SLICE-BUDGET-OPEN-1/slicebudget_1/rounds/g1_r1.json` 为 fixture 走查（G1-G5+G7 pass、G6/G8 fail、target_evidence_ref=obs_20261031T120646_69938d440c7d 非空、proposal_valid=true、gap 条件 G6=no_selected_candidate / G8=target_not_from_frontier_candidate）：

1. **frontier 折空（候选 B 面）**：`minimal_audio_closure.hypothesis_frontier` 实际形状 = `{"blockers":["G6_target_evidence","G8_target_consistency"]}`——无 `candidates`、无 `candidate_id`。同 slice lag window（fold 在 slice 边界执行），本身合法，正是 FIX-GATE-FRESHNESS-1 fallback 要救的场景。
2. **fallback 数据通道断线（主断点）**：G6 allowedTracks 空窗与 G8 assertion 1 的共同救援 `messageLoopFreshTrackObservationTarget(state)` 只读 `state.recentObservation`（运行时内存字段）。chat 侧审计路径（g1_r1 实际评估点 `chat/free_state_reasoning_loop.go:1454` → `agentloop.AuditFreeStateNeedsExperimentGate`）构造 `state := &runState{input: Input{Context: cloneMap(context)}}`，**recentObservation 恒为 nil** → fallback 恒返回 ""。agentloop 活路径（runner 内评估）不受影响。
3. **gateG6 ledger 终检漏扫 receipts（候选 A 面，但形状与预判相反）**：gateG6 末段 fallback 只扫 `observation_ledger.available_views` 行的 `target_ref` 字段——而真实回执中 available_views 行**不含 target_ref**（track 身份只在 key `track:1010::…` 里）；有完整 `target_ref` map（`{"id":"1010","kind":"track"}`）+ usable status（ready/partial）的 **receipts 行反而没被 G6 检查**（G8 assertion 2 已扫 receipts，两 gate 对同一事实判定不一致，B13-A 镜像问题）。
4. 证据实际在场：receipts 5 行 track 级（obs_…120449/120500/120543/120605/120646），最新 obs_20261031T120646_69938d440c7d status=partial、target_ref={id:1010,kind:track}、proposal evidence_refs 恰指向它（G7 已过）。`latest_observation.summary.target_ref` 是完整 map、status=partial——形状匹配无问题，排除"CCB 回执形状错"假设。
5. 附带发现（越域仅记录不动）：closure `observations` 的 `target_ref` 是纯字符串 "1010"（audioclosure 域），与 CCB 回执 map 形状不同——本卡文件域外，如需统一另开卡。

**修复设计（fail-closed 不弱化）**：① `messageLoopFreshTrackObservationTarget` 增加 ledger fallback——recentObservation 不可用时从 `free_state_reasoning_loop.observation_ledger.receipts` 取最新 usable（ready/partial）track 级 receipt 的 target id（= fold 将消费的同一观察行，与 FIX-GATE-FRESHNESS-1 语义一致；活路径 recentObservation 优先级行为零变化；无 usable track receipt 仍返回 "" 保持 fail-closed）；② gateG6 ledger 终检补 receipts 扫描（`freeStateReceiptUsable(row) && allowedTracks[ledgerRowTrackTarget(row)]`，与 G8 assertion 2 对称）；③ G8 assertion 1 自动受益于 ①。g1_r1 fixture 走查：修复后 G6/G8 均 pass。

### A2 断点诊断

family 只从 `loop.LatestDecision.SemanticProcessorIntent.Family` / `ProcessorType` 两来源解析（`chat/goalrunner_chat.go:2253-2269`）；g1_r1 的 decision 两字段皆空、只带 `improvement_proposal`（action_domain=track_gain）→ family="" → family_unresolved 三连锁。修复：解析链补第三来源 improvement_proposal.action_domain（词表映射：插件域→对应 attestation family；track_gain/clip_gain→volume 族、pan→pan，原生参数域如实映射）；非插件族的 unavailable 收敛为 no_pca_admitted_same/adjacent_family_candidate（真实语义：无插件族候选）。

### B 断点诊断

`taskstate/state.go` `EventCapabilityBlocked` 分支对 `from.Terminal()` 一律 invalidTransition（StateCapabilityBlocked.Terminal()=true）→ 同型重复 settle 被拒 → goal failed。修复：Apply 对 `capability_blocked→capability_blocked` 同型重入提前返回幂等 no-op（成功、零新转移、Revision/History 不变）；其余转移表语义零改动。

- 真栈错峰登记：已完成（PC-RUNTIME-STACK.md 占用→释放，拆栈 TEARDOWN_ALL_ZERO）
- §8 计费披露：见回执节

## 回执（执行侧自验，待主管验收）

- 分支：`port/m3-fs-gate-evidence-wire-1`（领取 e5954e9b / 实现 20bcff64 / 栈登记 0b8faf2e / 回执+证据随后）；worktree `D:\Vit_DAW_wt_233impl`
- run：`coord/runs/M3-FS-GATE-EVIDENCE-WIRE-1/{wire1,wire2}`（EVIDENCE.md 证据锚点在卷）

### 链路进度（§7 分列）

1. **A1 G6/G8 证据接线：完成+真栈生效亲证**。诊断成文（见领取节）：主断点=chat 审计路径 `AuditFreeStateNeedsExperimentGate` 构造 snapshot-only runState（recentObservation 恒 nil），FIX-GATE-FRESHNESS-1 fallback 在该路径结构性失效；次断点=gateG6 ledger 终检漏扫携带完整 target_ref 的 receipts（available_views 行反而不带 target_ref）。修复=双臂 fallback（活观察优先，缺省时 ledger 最新 usable+已交付 track 级 receipt）+gateG6 receipts 扫描（与 G8 assertion2 对称）+`freeStateReceiptDeliveredAnyView` 交付判定守两处新路径（B13-D 全裁 receipt 不绑目标，status 无关）。回退验证：还原修复后 g1_r1 fixture 测试精确复现 `failed=[G6_target_evidence, G8_target_consistency]`。真栈：两 run 11 采样轮 admission_gate_failed 零出现（232 同腿 8/10），组 1/组 2**首次走到 improvement_proposal 确认卡**（PendingCandidate/waiting_user，229 五卡以来第一次 G1-G8 全过+提案接受）。
2. **A2 family 断线：完成**（单测层验证；本轮无 blocked 轮触发该面——门开后不再走到）。`capabilityboundary.FamilyFromImprovementActionDomain` 全词表映射+chat wiring 第三来源；三连锁 family_unresolved 收敛为真实 per-class 语义（volume 族=no_pca_admitted_same/adjacent 等）。
3. **B taskstate 同型幂等重入：完成+真栈生效亲证**。232 的 `capability_blocked is not allowed from capability_blocked` goal failed 形态本轮零出现。
4. 测试：go build 0+全仓 go test 全绿（含 chat 全量 173s）+G0 五面绿；新增三域测试（g1_r1 fixture 复现+audit 接缝+fail-closed 边界+词表全往返+幂等五例）；三处旧测试 fixture 修正=误拒时代构造对齐真拒场景（gateRefusedFreeStateFixture 的 track-202 receipt、TIMING2 advanced、M06 dangling/no_evidence_basis variant——断言意图零改动，附注释）。
5. 判断权分岔核对（主管 amend）：g1_r1 模型提案轨 1010=ledger 最新 track 观察轨 1010，不存在「模型自主选轨 vs frontier 提名不一致」；修复后提案轨≠观察轨仍 fail-closed（单测钉死）。

### 最终验收（第五承未达，按停止条件上交）

- wire2 组 1：[no_route, confirm_broken, confirm_broken, capability_blocked, no_route] successes=0；组 2：[confirm_broken, no_route, ×3 confirm_broken] 止损三连；组 3 skipped。**no_route 消失未达**（残留两轮=观察循环耗尽 turn 预算的新形态，非门拒绝）；**set_points 端到端首绿未达**——断在**确认驱动环**：
  - 机制（g1_r2 全链在卷）：确认卡弹出→驱动「确认写入」×3 全部落入 LLM 纯文本轮（tools=[]），从未路由到 `continueImprovementProposalInteraction` 的 settle。
  - 锚点：`agent/internal/actionworkflow/types.go:255-263`（acceptText 词表无「确认写入/写入/开始写入」族）+`agent/internal/chat/server.go:4035-4050`（isImprovementProposalConfirmationText 需「确认」+「提案|建议|这个」）。对照：choreo 卡按钮印「确认写入」（automation_choreo.go:584 词表含它），improvement_proposal 卡按钮=「确认提案」；驱动脚本 CONFIRM_PHRASE 固定「确认写入」（218 choreo 时代短语）。A1 修复前该卡从未到达，此兼容性从未被测过。
  - **两案上交（不在本卡文件域，不择案）**：A=生产侧 acceptText/isImprovementProposalConfirmationText 补「确认写入」族（越 actionworkflow+chat 路由域）；B=驱动侧 CONFIRM_PHRASE 换「确认提案」（越 scripts 红线域）。附注：确认链修通后单轮链更长，`CHAT_TIMEOUT_SEC=300`（脚本硬编码）可能结构性不足（wire1 即因此中断，见 EVIDENCE §C）。
- §8 计费：wire1 61 调用（908K in/78K out，另 1 canceled）+wire2 167 调用（2.47M in/236K out）=**228 计费调用，约 3.38M in/314K out**（deepseek-flash）。
- 测试命令及退出码：`go build ./...`=0；`go test ./...`=0（全绿）；G0 五面=5×ok；真栈 wire2 ps1 exit 0+wire1 ps1 exit 1（驱动面超时，环境中断分类在卷）。
- 卡留 doing 待主管验收。
