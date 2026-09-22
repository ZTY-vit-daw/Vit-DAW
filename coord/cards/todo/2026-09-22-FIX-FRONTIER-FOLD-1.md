# FIX-FRONTIER-FOLD-1：候选名单折叠确定性——合入 GATE-FRESHNESS 兜底 + 修折叠生成/选择不对称

- 优先级 / 预估 / 依赖：P1 / 0.5-1 天 / **FIX-F3-G4-SEMANTICS 分支合入 main（31d35f8d+51686265 待决策验收）**；若届时未合入，从 fix/f3-g4-semantics 分支开工并在回执注明基线。**用户已裁定 1+2 都做（2026-09-22 会话）**
- 模型分级：L2 / GLM-5.3（门语义+折叠语义=准入核心，红测试先行）
- **背景（已定分）**：候选名单（hypothesis_frontier）由 harness 从已入账观察折叠派生（chat/audio_closure_controller.go:865 audioClosureFrontier / :893 audioClosureCandidates / :1240 audioClosureSelectedCandidate；折叠时机=回合边界 recordAudioClosureRound:534-541），AI 无写名单通道，无强引导。已证缺陷：①折叠-准入错位——锁死轮/切片续跑时名单未折，提案撞 G5/G6/G8 三连拒（真栈 20260922_182036 clarify-ask，gap=[G4,G5,G6,G8]），FIX-GATE-FRESHNESS-1（分支 fix/gate-freshness-1 @8653a25a，**未合入 main**）的 G5/G6/G8 空前沿兜底正是解法（G8 兜底=空前沿时提案目标=新鲜轨道级观测轨即一致，其余轨道/跨轨污染照拒）；②折叠生成/选择不对称——audioClosureSelectedCandidate 认"轨道级观察盯住的候选"，但 audioClosureCandidates 要求结论行内嵌轨道 ID 才生成候选：轨道级观察（target_ref=track）结论行无轨道标签→选择器有据可认却无候选可认。
- 目标（两项，用户裁定 1+2）：
  1. **合入 FIX-GATE-FRESHNESS-1**（8653a25a → 现 main 含甲后的基线，冲突重放：甲改 G4 槽位/gateG4ImprovementProposal，其改 G5/G6/G8+新增 freeStateLedgerHasUsableScan/messageLoopFreshTrackObservationTarget，逻辑不重叠仅文本冲突）；合入后原卡 FIX-GATE-FRESHNESS-1（doing/）随本卡验收一并归档，其 ⑤ exit-0 上交项以本卡合并后 ⑤ 1 轮复核为准
  2. **修折叠不对称**：audioClosureCandidates 对 usable 轨道级观察（target_ref.kind=track）在结论行无内嵌轨道 ID 时仍折出候选（轨道=观察 target，候选 ID 沿用 observationID+viewID+issueType+region+trackIDs 派生式）——"证据已入账=事实已登记"，登记窗口与选择器对齐；红测试=轨道级 usable 观察行无轨道标签修前无候选/修后有候选且可被 audioClosureSelectedCandidate 选定
  3. 红测试：item2 修前红/修后绿；合入重放后 GATE-FRESHNESS 红测试组（free_state_gate_freshness_test.go）与 F3 甲/乙测试组（free_state_terminal_adjudication_test.go×2 包）同基线全绿=两侧契约共存证明
- §8：content-blind 红线（fold 派生与 G 门兜底只用结构性字段 target_ref/view_id/轨道 ID/freshness，不引入领域内容）；弹回词表零变动（GATE-FRESHNESS 只加兜底不改词表）；mac 跟随注记（G 门为两端共享 Go 代码，合入即双端生效；mac 侧脚本无 terminal/G 门词表消费，无强制跟随项）
- 文件域：agent/internal/agentloop/free_state_gate.go + free_state_gate_freshness_test.go（自 8653a25a 携带）+ agent/internal/chat/audio_closure_controller.go + 测试；冲突重放不得改动甲的 gateG4ImprovementProposal 语义
- 验收：①②红测试修前红/修后绿（含共存全绿）；③agent 全量+webui 回归绿；④⑤ 各 1 轮真栈（存在性，⑤ clarify-ask 为本族现场）；⑤回执记合入冲突清单与两侧测试组结果
- 停止条件：重放中发现 GATE-FRESHNESS 兜底与甲 OR 语义实质冲突（非文本冲突），或折叠修改需引入领域内容才能绿 → 域外上交裁定
- 领取：
- 回执：
- 验收：
