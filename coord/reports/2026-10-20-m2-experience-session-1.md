# M2 体验场次①实录（2026-10-20 晚窗，用户手测）

- 场次：用户从 Godot 拉起前端进入六轨工程（stems=bass/drums/guitar/other/piano/vocals，源自 `D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\...\spv1_p06`——**fixture 面，非真实工程新面**）
- 会话：conversation=webui_mvgj5jdz / goal=goal_98d6813ec6100767 / 18:17–18:26；模型=deepseek-flash 思考档；authority=**full_project_access**（18:18:15 switch accepted + restore kept explicit，agent_last.log）
- 二进制：内核 722A79D7+agent/hub 18:15 构建（gate 2026-10-19 已核），与源码一致
- 决策侧事后取证（非现场）：本文所有结论以工件回指为证

## 第一轮（主话术，18:18:16，与 109 验证轮逐字同构）

「有几轨听起来比较干，想给其中一条合适的轨加一点混响（衰减短些），不要用整体增益或 EQ 来解决。请先观察工程，再给一个有界的小步改进建议。」

| 时间 | 事件 | 工件锚点 |
|---|---|---|
| 18:18:21-18:19:00 | 观察 ✓：ccb.observation_catalog + ccb.observation_request×2 | agent_last.log |
| 18:19:12 | version.checkpoint ✓ | 同上 |
| 18:19:28 | plugin.semantic_search（12s，候选=准入面） | 同上 |
| 18:19:35 | **stop=plugin_selection_required**：插件选择交互卡（仅 1 个候选 Pro-R），用户阻塞 60s | 同上；conversation node[2] |
| 18:20:35 | 用户点 select_plugin_candidate_1 → rack.add_node confirmed=true（装载 Pro-R） | interaction_61ce6085a55a2ae9 |
| 18:20:35-18:20:49 | 装载确认卡 + typed controller 确认卡（用户 approve×2）→ 一次参数写入（revision 3→4） | interaction_0748…/50b4… |
| 18:20:59-18:21:45 | turn6-9 连续 4 turn **executed=0**（LLM 空转） | agent_last.log |
| 18:21:34 / 18:21:45 | **两次 free_state_final_gate 驳回**（见 F5） | agent_message_loop_debug.jsonl |
| 18:21:45 | budget_stop=true / lifecycle=completed / **owes_outcome=true**，链终止 | agent_last.log |

- 目标轨=**bass**（模型推理原文「在 bass 轨上形成一层薄薄的空间感」，conversation node[2]）；装载确认卡只写「**当前轨道**」不指名（node[4]）。
- **停泊/A/B 试听卡/判定（清单 §3 步骤 4-6）未到达用户**；用户静默 95 秒后发第二轮。

## 第二轮（146 取证话术，18:22:24）

「给人声加点空间感」→ turn10 零工具调用 → 18:22:34 **stop=capability_blocked**，回复原文：

> 本次有界改进提案未通过自由态准入门（G3_project_scan、G5_frontier_established、G6_target_evidence、G7_fresh_revision_bound_refs、G8_target_consistency），任务在能力边界停止：没有执行任何实验或修改，也不会自动重试。

## Findings

- **F1（缺陷，卡 147）**：full_project_access 未传导语义处理链——三张交互卡（插件选择/装载确认/typed controller 确认）在完全访问下全部弹出。既有契约三处在卷：用户裁定 2026-09-13「完全访问权限应该是它能够自主进行所有执行，我只负责AB试听」（`agent/internal/agentloop/full_access_autonomy.go:10-38` 原文引用，并明言 asking a plug-in instance = behavioral defect）；mix_tick 面 2026-09-11 改判先例（`internal/chat/mix_tick_confirmation.go:531-537` full-access 立即应用不发卡）；harness PCA load gate 的 full-access 通道（`explicitFullProjectAccess`）+ PCA-AUTONOMY-CATALOG-1 准入目录披露。缺口=`internal/chat/plugin_recommendation.go:536-640`（pluginRecommendationSelectionResponse）无任何权限分叉，后续装载/控制器确认面同。
- **F2（措辞失真，卡 148）**：卡面称「当前工程里**唯一可加载**的混响是 FabFilter Pro-R」——实测插件库 994 项含数十种混响（H-Reverb/RVerb/TrueVerb/Abbey Road Plates/MannyM Reverb/ZamVerb/IR1/Lofi Space/ValhallaSupermassive…，BridgeReplies/20261020_181747_plugin_list_available_library）。真实语义=「语义可控准入面仅 Pro-R」。边界诚实但「可加载」表述与事实不符。
- **F3（卡面合同退化，卡 148）**：①装载确认卡不指名目标轨（「当前轨道」vs 清单 §3.2「指名目标轨、参数、小步变化量」）；②typed controller 确认卡英文原文（node[5] "The semantic plan is ready. No parameter has been written. Confirm to execute the typed controller."）；③目标轨选 bass 属模型决策质量（混响上 bass 轨违背常规混音直觉；话术允许自选轨，不断裂，记录为软引导实验变体输入）。
- **F4（146 核心取证）**：模糊意图的实际路径=**到不了任何提案**。server-side 自由态准入门拒绝（stop_reason=free_state_admission_gate_failed，failed_gate_ids=G3/G5/G6/G7/G8，`internal/chat/goalrunner_chat.go:2096-2104`），模型 prose 终答被响应路由扣下，用户收到 G 门代号拼接文案。146 待证 1 的预答：比「规划层零拓扑概念」更早——模糊意图在准入门就断，拓扑决策无从发生；且 turn10 零观察（模型把新话术当延续，未做本轮观察，五门全悬空有内在一致性）。修复 146 的注入设计必须先回答：语义意图路由在 G 门评估之前还是之后分流。
- **F5（主旅程断裂，卡 149）**：模型在 18:21:34 与 18:21:45 **两次产出完整合格的 A/B 试听交接正文**（「请在 DAW 里对 bass 轨做整体开关对比（旁通/启用这条轨的混响）…交由你的耳朵判断」），均被 free_state_final_gate 以决策对象形状违规**整条驳回**：第一次 experiment_materiality 形状违规（state=""、attempt=0——错误信息已含完整 shape 教学）；第二次 status=needs_action 携带 improvement_proposal（要求 improvement_proposal|needs_experiment）。两次驳回后预算耗尽，owes_outcome=true 但用户面零交付。109 线双 exit 0 与今晚逐字同构话术+同模型档，验证面未覆盖该失败路径。
- **F6（卫生，不占卡）**：continuation.park 日志 ~3 条/秒刷屏（18:19:35-18:20:35 共 ~180 行）；后续归入日志卫生池。
- **F7（面差缺口，待用户裁定）**：准入推荐面与探针兼容面之间存在断层——探针兼容面已验证 **42 件可证时域插件**（含 Abbey Road Plates 的 Damper 受控写证据，PLUGGRABBER-TIMEDYN-COMPAT-1，2026-10-16），但准入推荐面（`~/.vit/processor_control_attestations.v2.json`）reverb 族**仅 1 件 promoted（Pro-R，rev78/79 均为 2026-10-12 状态）**。断层项=PCA 认证晋升战役（control case evidence → attestation → admission）。是否开晋升卡由用户裁定。

## 用户三问（2026-10-20 晚窗原话）与决策侧答案

1. **完全访问权限为什么会弹出交互卡片？**→F1/卡 147：语义链三处确认面（plugin_recommendation.go:536-640 选择卡+装载确认+typed controller 确认）均无权限模式分叉；完全访问=全链自主的契约（2026-09-13 裁定）在 mix_tick 面与 harness 装载门已实现，唯独本链未接线。
2. **为什么不用 FX 轨而是直接（inline）加载？**→146 主题：整场唯一内核命令=一次 `rack_add_node`，零 `track.send.*`/零建轨。语义链**没有任何拓扑决策层**（146 已钉事实 2/3：规划层零拓扑概念、控制面未建）；send 底座（K1/K2/K3）10-19 才收官，语义面尚未接入。146 的设计方向即「时域族默认 send/return 拓扑、inline 为显式例外」。
3. **为什么说只有 Pro-R？（记忆：之前测试提供过 Waves 板式混响）**→两面之差，非回归：准入推荐面 reverb 仅 Pro-R promoted（F7 上面）；用户记忆的 Abbey Road Plates 来自 10-16 **探针兼容面**（PLUGGRABBER 卡内核 rack_add_node 直连+inspect 放行+Damper 受控写——该面明确「PCA 产品装载门未触碰未绕过」，run1 里 Plates 走 agent 装载面即被 PCA 门拒）。卡面「唯一可加载」措辞失真（F2/卡 148），但候选集本身是准入面的诚实呈现。

## 正向证据（链路做对的部分）

观察先行 ✓；版本检查点 ✓；一次前向变更（装载+一次参数写入，无追加轮）✓；候选来自 attestation 准入面（plugin_recommendation_attestation）✓；authority switch/restore 记录正常 ✓；边界终止不重试、诚实声明无修改 ✓。

## 对 146 的影响（待证 1 预答，已补记入卡）

模糊意图链现状=准入门拒绝+代号文案，无提案形态可言。146 的取证腿转向：①准入门拒绝的根因分层（模型未做本轮观察 vs 门语义过严）；②注入点设计必须考虑「语义意图路由在 G 门之前分流」；③软引导实验的话术集应包含「准入门先过」前置（否则实验测的是门不是路由）。本实录=`coord/reports/2026-10-20-m2-experience-session-1.md`。

## 工件清单

- `VitApp/Workspace/Logs/agent_last.log`（500 行，含 authority/timing/park/budget 全线）
- `VitApp/Workspace/Logs/agent_message_loop_debug.jsonl`（18:21:34 / 18:21:45 free_state_final_gate 两次驳回全文）
- `VitApp/Workspace/Logs/agent_llm_telemetry.jsonl`（turn1-10 遥测；turn8 output=5023tok）
- `VitApp/Workspace/Logs/BridgeReplies/20261020_181747_..._plugin_list_available_library_...json`（994 插件清单）
- `VitApp/Workspace/Logs/mixboard_interaction_diag.jsonl`（三张交互卡的 respond 事件）
- 会话图：`C:\Users\timoz\AppData\Local\Vit\ProjectHistory\drafts\draft_20261020T101744_927d815e\.vit_history\.sessions\vitproj_ae6c4aefac4467dae33ec98aa023778\session_20261020T101745_98f2dc27\workspace\state\conversation_graph.json`（10 节点全文）
- 观察/证据件：同根 `.vit_agent\vitproj_ae6cb4aefac4467dae33ec98aa023778\observations\obs_20261020T101831/101900/102049_*.json`
