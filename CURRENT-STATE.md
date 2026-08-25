# CURRENT-STATE.md — docs/ 文档三态索引

本文件是 `docs/` 全部文档的**现行清单权威索引**（T6 产出，2026-08-23）。AGENTS.md §4 的规则以此为准：**只把标记为「现行」的文档当作现状描述来读**。

三态定义：

- **现行**：内容与当前实现一致，可作为现状/契约依据直接阅读。
- **历史记录**：基线、验证记录、审计、ADR、设计草案快照等纯记录性文档。保留原位，可查证当时的设计决策，**不得当作现状**。
- **已废弃**：内容已被实现超越且会误导 agent 的文档，已迁至 `docs/archive/`。

汇总：`docs/` 顶层共 117 份现行/历史文档（现行 51 / 历史记录 66），另有 `docs/archive/` 已废弃文档 4 份；两者合计 121 份。`docs/vsp/` 子目录 13 份文档单独索引（见末节）。

---

## 一、现行（51）

### 观察投影与上下文

| 文档 | 说明 |
|---|---|
| [OBSERVATION_PROJECTION_MANIFEST.md](docs/OBSERVATION_PROJECTION_MANIFEST.md) | 观察投影拓扑 ground truth（DAD + peer 投影，AGENTS.md 白名单收录） |
| [COM_V1_CONTRACT.md](docs/COM_V1_CONTRACT.md) | Compression Observation Model 契约（`com.projection.v1`，COM-1 已实现） |
| [COM_V1_TEST_CONTRACT.md](docs/COM_V1_TEST_CONTRACT.md) | COM v1 测试契约（COM-1–COM-5 测试已实现） |
| [DOM_V1_CONTRACT.md](docs/DOM_V1_CONTRACT.md) | Dynamics Observation Model 契约（`dom.projection.v1`，source_only 已落地） |
| [MOM_FREQUENCY_RELATIONSHIP_V1.md](docs/MOM_FREQUENCY_RELATIONSHIP_V1.md) | MOM 频率关系投影（`mom.frequency_relationship.v1`，MOM v1.5 已实现） |
| [CONTEXT_GRAPH_V1_DESIGN_DRAFT.md](docs/CONTEXT_GRAPH_V1_DESIGN_DRAFT.md) | Graph Context v1 权威设计记录（v1 已实施 2026-08-13） |
| [AGENT_STORE_V2_DESIGN.md](docs/AGENT_STORE_V2_DESIGN.md) | Agent 工程存储格式 v2（Part 1 已实现并通过验收；Part 2 未开始） |

### 自由态与语义循环

| 文档 | 说明 |
|---|---|
| [FREE_STATE_MINIMUM_IMPROVEMENT_WORKFLOW_UPDATE_2026-08-22.md](docs/FREE_STATE_MINIMUM_IMPROVEMENT_WORKFLOW_UPDATE_2026-08-22.md) | 自由态工作流修订（AGENTS.md 白名单收录） |
| [FREE_STATE_CCB_OBSERVATION_PROTOCOL_V1.md](docs/FREE_STATE_CCB_OBSERVATION_PROTOCOL_V1.md) | 自由态 CCB 只读观察协议（模型是决策所有者） |
| [FREE_STATE_DIAGNOSTIC_ONLY_V1.md](docs/FREE_STATE_DIAGNOSTIC_ONLY_V1.md) | 自由态只读诊断边界契约 |
| [FREE_STATE_PHASE_TRANSITION_TABLE_V1.md](docs/FREE_STATE_PHASE_TRANSITION_TABLE_V1.md) | 自由态 FS0–FS9 相位转移表与四层状态机映射（implemented 且生产可达（Phase B 接线收口）：audioclosure/phase.go 相位机（守卫由 DerivePhaseGuardInput 从 State 实证推导）+ events.go phase_transition 事件 + chat 轮边界 AdvancePhase 生产驱动 + free_state_phase 上下文键；M01/M02 及接线测试已落地） |
| [FREE_STATE_DIAGNOSTIC_ROUND_AND_PRIORITY_QUEUE_SCHEMA_V1.md](docs/FREE_STATE_DIAGNOSTIC_ROUND_AND_PRIORITY_QUEUE_SCHEMA_V1.md) | 诊断轮次与优先队列 schema（implemented 且生产可达（Phase B 接线收口）：audioclosure/diagnostic_round.go schema + 三准入（准入 1 需新 unresolved question）+ 轮闭合时生产写入事件流（G4 数据源接通）；chat 循环持 current_round_id/current_phase/priority_queue/budget（队列由 audioclosure.QueueFromRounds 从事件流派生、syncFreeStateSpine 镜像，malformed fail-closed）；M03–M05 及队列接线/负例测试已落地） |
| [FREE_STATE_NEEDS_EXPERIMENT_GATE_V1.md](docs/FREE_STATE_NEEDS_EXPERIMENT_GATE_V1.md) | needs_experiment 七项准入契约（implemented 且生产可达（Phase B 接线收口）：agentloop/free_state_gate.go G1–G7 整体替换弱门，门失败唯一出口 needs_observation；G4/G7 生产数据源（diagnostic round 事件流 + ledger project_binding/freshness 透传）已接通；M06/M07 已落地） |
| [FREE_STATE_IMPROVEMENT_EXECUTION_RECEIPT_V1.md](docs/FREE_STATE_IMPROVEMENT_EXECUTION_RECEIPT_V1.md) | 最小改进执行回执 schema（已实现（Phase C 收口）：experiment/receipt.go 类型与机检校验（含 ambiguous 不得 continue_once、continue_once 全局一次、五层分离红线）；M08/M09/M19 测试落地；真实音频 Apply/Rollback/Settlement 留待下一阶段） |
| [FREE_STATE_TEST_AND_REPLAY_MATRIX_V1.md](docs/FREE_STATE_TEST_AND_REPLAY_MATRIX_V1.md) | 自由态四层测试与回放矩阵（已实现（Phase C 收口）：L1 M01–M10、L2 回放 M11–M17（free_state_replay_test.go + testdata/free_state_replay/ fixture 家族）、L4 M19 全部落地；L3 验收脚本按 journal/events 实际观察计数 + 运行时接口终态（含 free_state_limitations 终态成因）） |
| [SEMANTIC_ENTRY_V1.md](docs/SEMANTIC_ENTRY_V1.md) | 模型侧语义入口决策（`semantic_entry_decision.v1`，现行入口路径） |
| [SEMANTIC_PRE_FAMILY_DISCLOSURE_V1.md](docs/SEMANTIC_PRE_FAMILY_DISCLOSURE_V1.md) | 语义 pre-family 披露边界 |
| [SEMANTIC_PROCESSOR_INTENT_V1.md](docs/SEMANTIC_PROCESSOR_INTENT_V1.md) | `semantic_processor_intent.v1` 观察目标→处理器族交接 |
| [SEMANTIC_PROGRESSIVE_DISCLOSURE_V1.md](docs/SEMANTIC_PROGRESSIVE_DISCLOSURE_V1.md) | 语义渐进披露共享工作流边界 |
| [DUAL_MODE_ARCHITECTURE_V2.md](docs/DUAL_MODE_ARCHITECTURE_V2.md) | 双模式架构（自由对话 + 编排混音），Active |
| [DUAL_MODE_QUICK_START.md](docs/DUAL_MODE_QUICK_START.md) | 双模式架构快速上手指南 |
| [OBSERVABLE_TRAJECTORY_PROTOCOL_V1.md](docs/OBSERVABLE_TRAJECTORY_PROTOCOL_V1.md) | 可观察轨迹协议（`vit.observable_trajectory.v1`，已实现/mock reducer） |
| [PROJECT_CHANGE_RECEIPT_V1.md](docs/PROJECT_CHANGE_RECEIPT_V1.md) | `project_change_receipt.v1`：Shadow/Mixboard ↔ 自由态 agent 的变更回执 |
| [AUDITION_AB_AND_USER_JUDGMENT_V1.md](docs/AUDITION_AB_AND_USER_JUDGMENT_V1.md) | Audition A/B 与用户判断（G0 设计冻结） |
| [KERNEL_AUDITION_PREVIEW_CONTRACT_V1.md](docs/KERNEL_AUDITION_PREVIEW_CONTRACT_V1.md) | 内核试听预览契约（G0 设计冻结，Kernel/JUCE 拥有渲染） |

### 处理器控制契约（PCA 体系）

| 文档 | 说明 |
|---|---|
| [PCA_PLUGIN_LOAD_GATE_V1.md](docs/PCA_PLUGIN_LOAD_GATE_V1.md) | PCA 约束的插件加载门（AGENTS.md 白名单收录） |
| [PROCESSOR_CONTROL_ATTESTATION_V1.md](docs/PROCESSOR_CONTROL_ATTESTATION_V1.md) | PCA v1 准入库（static_eq / broadband_compressor） |
| [PROCESSOR_CONTROL_ATTESTATION_V2.md](docs/PROCESSOR_CONTROL_ATTESTATION_V2.md) | PCA v2 身份无关 typed 控制器准入词汇表 |
| [PROCESSOR_CERTIFICATION_ENTRY_V1.md](docs/PROCESSOR_CERTIFICATION_ENTRY_V1.md) | 处理器能力认证产品入口（Plugin Manager 设置页） |
| [COMPRESSOR_CONTROL_V1.md](docs/COMPRESSOR_CONTROL_V1.md) | 通用宽带压缩控制 v1.1（已过实现与产品路径门） |
| [COMPRESSOR_REFERENCE_PRO_C_2.md](docs/COMPRESSOR_REFERENCE_PRO_C_2.md) | 压缩语义规划权威示例（Pro-C 2 worked example） |
| [GATE_EXPANDER_CONTROL_V1.md](docs/GATE_EXPANDER_CONTROL_V1.md) | 门/扩展器控制 V1 |
| [LIMITER_CONTROL_V1.md](docs/LIMITER_CONTROL_V1.md) | 限制器控制 V1 |
| [MULTIBAND_DYNAMICS_CONTROL_V1.md](docs/MULTIBAND_DYNAMICS_CONTROL_V1.md) | 多段动态控制 V1 |
| [TRANSIENT_SHAPER_CONTROL_V1.md](docs/TRANSIENT_SHAPER_CONTROL_V1.md) | 瞬态塑形控制 V1 |
| [SPECTRAL_DYNAMICS_CONTROL_V1.md](docs/SPECTRAL_DYNAMICS_CONTROL_V1.md) | 频谱动态控制 V1（inspect-only 边界冻结） |
| [COM_6_COMPRESSOR_SEMANTIC_WORKFLOW.md](docs/COM_6_COMPRESSOR_SEMANTIC_WORKFLOW.md) | COM-6 压缩语义工作流（已实现契约） |
| [COM_7_COMPRESSOR_EXECUTION_CONTRACT.md](docs/COM_7_COMPRESSOR_EXECUTION_CONTRACT.md) | COM-7 压缩语义执行契约 |

### 混音能力与编排

| 文档 | 说明 |
|---|---|
| [C1_FREQUENCY_CLEANUP_V1.md](docs/C1_FREQUENCY_CLEANUP_V1.md) | C1 频率清理能力（`fine_mix.frequency_cleanup.v1`） |
| [C1_FREQUENCY_CLEANUP_SMOKE_CONTRACT.md](docs/C1_FREQUENCY_CLEANUP_SMOKE_CONTRACT.md) | C1 冒烟契约（安全边界：fixture/临时存储） |
| [C2_DYNAMIC_CONTROL_V1.md](docs/C2_DYNAMIC_CONTROL_V1.md) | C2 动态控制能力（`fine_mix.dynamic_control.v1`） |
| [B4_EQ_CLOSED_LOOP_SMOKE_CONTRACT.md](docs/B4_EQ_CLOSED_LOOP_SMOKE_CONTRACT.md) | B4 抽象 EQ 闭环冒烟契约 |
| [MIXBOARD_DECISION_LEDGER_V1.md](docs/MIXBOARD_DECISION_LEDGER_V1.md) | Mixboard 决策账本与混音报告 v1（Normative） |
| [PROJECT_AWARE_CAPABILITY_ORCHESTRATION_ARCHITECTURE_V1.md](docs/PROJECT_AWARE_CAPABILITY_ORCHESTRATION_ARCHITECTURE_V1.md) | 项目感知能力编排架构（Normative v1） |

### 基线与仓库治理

| 文档 | 说明 |
|---|---|
| [G0_C2_BASELINE_2026-08-17.md](docs/G0_C2_BASELINE_2026-08-17.md) | G0 基线记录；AGENTS.md §6 健康检查命令的权威来源（白名单收录） |
| [REPO_OPTIMIZATION_TODO_2026-08-22.md](docs/REPO_OPTIMIZATION_TODO_2026-08-22.md) | 本轮仓库优化任务清单 T1–T11（白名单收录） |

### 跨端契约与运维

| 文档 | 说明 |
|---|---|
| [VIT_IPC_CONTRACT.md](docs/VIT_IPC_CONTRACT.md) | Godot ↔ Bridge ↔ ZMQ IPC 契约（UDP 4444/4445 桥仍在现役链路中） |
| [VIT_MESSAGE_LIFECYCLE_COPY_SYSTEM_V1.md](docs/VIT_MESSAGE_LIFECYCLE_COPY_SYSTEM_V1.md) | WebUI 消息生命周期与复制系统（messageLifecycle 测试对应契约） |
| [ROUTING_CONSTITUTION.md](docs/ROUTING_CONSTITUTION.md) | Vit 路由宪法（横线=机架 DAG、竖线=Clip 事件作用域） |
| [Vit-DAW 发布模式一键启动配置清单.md](docs/Vit-DAW%20发布模式一键启动配置清单.md) | 发布模式一键启动配置 |
| [Vit-DAW_项目白皮书.md](docs/Vit-DAW_项目白皮书.md) | 项目白皮书（定位与三层架构总述） |

---

## 二、历史记录（66，保留原位，不得当作现状）

### ADR / 决策记录

| 文档 | 说明 |
|---|---|
| [ADR_AUDIO_CLOSURE_EXPERIMENT_V1.md](docs/ADR_AUDIO_CLOSURE_EXPERIMENT_V1.md) | Audio Closure 实验 ADR（2026-08-15，accepted/planned） |
| [ADR_FREE_STATE_EXPERIMENT_RUNTIME_AND_OBSERVABLE_TRAJECTORY_V1.md](docs/ADR_FREE_STATE_EXPERIMENT_RUNTIME_AND_OBSERVABLE_TRAJECTORY_V1.md) | 自由态实验运行时 + 可观察轨迹 ADR（proposed 草案） |
| [SPAL_VPS_RETIREMENT_ADR.md](docs/SPAL_VPS_RETIREMENT_ADR.md) | SPAL/VPS 退役 ADR（accepted & implemented） |

### 验证 / 验收 / 盲测记录

| 文档 | 说明 |
|---|---|
| [FREE_STATE_PHASE_D_D1_CLOSEOUT_2026-08-25.md](docs/FREE_STATE_PHASE_D_D1_CLOSEOUT_2026-08-25.md) | Phase D D1-S1 执行链收口：两次真实栈 PASS 证据、修复层提交映射、明确未竟事项与非目标 |
| [FREE_STATE_PHASE_D_D1_POSTACTION_CHAIN_FIXES_2026-08-25.md](docs/FREE_STATE_PHASE_D_D1_POSTACTION_CHAIN_FIXES_2026-08-25.md) | D-B1~B5 post-action 链修复记录（blocked 门/FS8 评估准入/交互 park/报告入账）与剩余缺口定位 |
| [FREE_STATE_PHASE_D_D1_FIX_A_ROUNDBOUNDARY_HANDOFF_2026-08-25.md](docs/FREE_STATE_PHASE_D_D1_FIX_A_ROUNDBOUNDARY_HANDOFF_2026-08-25.md) | Fix A（capability route 竞态）与 round 记录修复交接（ZCode/DeepSeek 会话间） |
| [FREE_STATE_PHASE_D_D1_DIMENSION_MAPPING_FIX_2026-08-25.md](docs/FREE_STATE_PHASE_D_D1_DIMENSION_MAPPING_FIX_2026-08-25.md) | CCB 维度映射三阶段修复设计（A 结构/B 措辞/C 模式识别 prompt） |
| [FREE_STATE_PHASE_D_D1_S1_CONTINUATION_2026-08-25.md](docs/FREE_STATE_PHASE_D_D1_S1_CONTINUATION_2026-08-25.md) | Phase D D1-S1 中续交接（confirmation 一致性修复后、execution smoke 前） |
| [PHASE_D_TEST_GUIDE.md](docs/PHASE_D_TEST_GUIDE.md) | 维度映射修复的快速验证与 D1 烟测操作指南 |
| [FREE_STATE_PHASE_D_D1_S1_PAUSED_HANDOFF_2026-08-23.md](docs/FREE_STATE_PHASE_D_D1_S1_PAUSED_HANDOFF_2026-08-23.md) | Phase D D1-S1 主动暂停交接：未提交实现、已报告验证、`spv1_p01 not_exercised` 缺口与续开发入口（非 D1 closeout） |
| [FREE_STATE_PHASE_C_CLOSEOUT_2026-08-23.md](docs/FREE_STATE_PHASE_C_CLOSEOUT_2026-08-23.md) | Phase C 自由态运行时与真实开放意图验收收口记录（本文件） |
| [COMPRESSOR_CONTROL_V11_VALIDATION.md](docs/COMPRESSOR_CONTROL_V11_VALIDATION.md) | 压缩控制 v1.1 验证（2026-08-04 通过） |
| [COMPRESSOR_CONTROL_V12_VALIDATION.md](docs/COMPRESSOR_CONTROL_V12_VALIDATION.md) | 压缩控制 v1.2 验证（关闭 round-2 三项失败） |
| [COMPRESSOR_OPEN_SEMANTIC_ROUTING_V1_VALIDATION.md](docs/COMPRESSOR_OPEN_SEMANTIC_ROUTING_V1_VALIDATION.md) | 压缩开放语义路由验证（通过） |
| [COM_6_VALIDATION.md](docs/COM_6_VALIDATION.md) | COM-6 验证（通过） |
| [COM_V1_AUDIT_AND_GAP_MATRIX.md](docs/COM_V1_AUDIT_AND_GAP_MATRIX.md) | COM v1 审计与差距矩阵（进度快照已过时） |
| [GATE_EXPANDER_CONTROL_V1_VALIDATION.md](docs/GATE_EXPANDER_CONTROL_V1_VALIDATION.md) | 门/扩展器验证 |
| [LIMITER_CONTROL_V1_VALIDATION.md](docs/LIMITER_CONTROL_V1_VALIDATION.md) | 限制器验证（2026-08-06） |
| [MULTIBAND_DYNAMICS_CONTROL_V1_VALIDATION.md](docs/MULTIBAND_DYNAMICS_CONTROL_V1_VALIDATION.md) | 多段动态验证 |
| [SPECTRAL_DYNAMICS_CONTROL_V1_VALIDATION.md](docs/SPECTRAL_DYNAMICS_CONTROL_V1_VALIDATION.md) | 频谱动态验证（冻结证据矩阵） |
| [TRANSIENT_SHAPER_CONTROL_V1_VALIDATION.md](docs/TRANSIENT_SHAPER_CONTROL_V1_VALIDATION.md) | 瞬态塑形验证（2026-08-07） |
| [EQ_AUXILIARY_EXCLUSION_FIX_VALIDATION.md](docs/EQ_AUXILIARY_EXCLUSION_FIX_VALIDATION.md) | EQ 辅助参数污染修复验证（2026-07-28） |
| [EQ_DECREASING_SENTINEL_CURVE_FIX_VALIDATION.md](docs/EQ_DECREASING_SENTINEL_CURVE_FIX_VALIDATION.md) | EQ 递减曲线/inactive sentinel 修复验证 |
| [EQ_FREQUENCY_SUFFIX_AND_SECTION_FEASIBILITY_FIX_VALIDATION.md](docs/EQ_FREQUENCY_SUFFIX_AND_SECTION_FEASIBILITY_FIX_VALIDATION.md) | EQ 频率后缀与 section 可行性修复验证 |
| [EQ_LEXICAL_CLOSURE_VALIDATION.md](docs/EQ_LEXICAL_CLOSURE_VALIDATION.md) | EQ 通用词法层收尾验证 |
| [EQ_PROPERTY_SENTINEL_CUT_FIX_VALIDATION.md](docs/EQ_PROPERTY_SENTINEL_CUT_FIX_VALIDATION.md) | EQ property-sentinel cut 修复验证 |
| [SEMANTIC_EQ_COMPRESSOR_GUIDANCE_AUDIT_BASELINE.md](docs/SEMANTIC_EQ_COMPRESSOR_GUIDANCE_AUDIT_BASELINE.md) | 语义入口变更前行为基线（自声明 historical） |
| [PLUGIN_ALLIANCE_COMPRESSOR_BLIND_V1_ROUND1.md](docs/PLUGIN_ALLIANCE_COMPRESSOR_BLIND_V1_ROUND1.md) | PA 压缩盲测 round 1（failed，冻结判定） |
| [PLUGIN_ALLIANCE_COMPRESSOR_BLIND_V2_ROUND2.md](docs/PLUGIN_ALLIANCE_COMPRESSOR_BLIND_V2_ROUND2.md) | PA 压缩盲测 round 2（failed，冻结判定） |
| [PLUGIN_ALLIANCE_COMPRESSOR_BLIND_V3_FINAL.md](docs/PLUGIN_ALLIANCE_COMPRESSOR_BLIND_V3_FINAL.md) | PA 压缩盲测 final round 3（passed） |
| [PLUGIN_ALLIANCE_EQ_TWO_LEVEL_HOLDOUT_20260728.md](docs/PLUGIN_ALLIANCE_EQ_TWO_LEVEL_HOLDOUT_20260728.md) | PA EQ 双层外部烟测记录 |
| [PLUGIN_ALLIANCE_EQ_THIRD_HOLDOUT_20260728.md](docs/PLUGIN_ALLIANCE_EQ_THIRD_HOLDOUT_20260728.md) | PA EQ 第三轮分层随机烟测记录 |
| [PLUGIN_ALLIANCE_EQ_FINAL_LIKELY_HOLDOUT_20260728.md](docs/PLUGIN_ALLIANCE_EQ_FINAL_LIKELY_HOLDOUT_20260728.md) | PA EQ 最终高通过概率烟测记录 |
| [SEMANTIC_PROCESSOR_OPEN_EXPERIMENT_V1.md](docs/SEMANTIC_PROCESSOR_OPEN_EXPERIMENT_V1.md) | 语义处理器开放实验（completed with failures） |
| [SEMANTIC_PROCESSOR_AGENT_PROJECT_SMOKE_V1.md](docs/SEMANTIC_PROCESSOR_AGENT_PROJECT_SMOKE_V1.md) | 语义处理器项目烟测（设计冻结，被观察门阻塞） |
| [G4_REAL_AUDITION_AUDIO_SPIKE_V1.md](docs/G4_REAL_AUDITION_AUDIO_SPIKE_V1.md) | G4 真实内核试听音频 spike 记录 |
| [G4_REAL_AUDITION_AUDIO_VALIDATION_2026-08-19.md](docs/G4_REAL_AUDITION_AUDIO_VALIDATION_2026-08-19.md) | G4 真实试听音频验证记录 |
| [OBSERVATION_V1_ACCEPTANCE.md](docs/OBSERVATION_V1_ACCEPTANCE.md) | 观察系统 v1 验收（2026-06-28） |
| [phase-2.1-real-e2e-validation.md](docs/phase-2.1-real-e2e-validation.md) | Phase 2.1 真实 E2E 验证门（2026-06-20） |
| [V0_3_AGENT_TOOLS_CLOSEOUT.md](docs/V0_3_AGENT_TOOLS_CLOSEOUT.md) | v0.3 工具收口清单（2026-05-20） |

### 研究 / 设计阶段记录

| 文档 | 说明 |
|---|---|
| [EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md](docs/EQ_CONTROL_TOPOLOGY_MONOTONIC_REDESIGN_RESEARCH.md) | EQ 拓扑单调重做研究（生产识别器未按此修改） |
| [WAVES_EQ_CONTROL_TOPOLOGY_CENSUS_PHASE1.md](docs/WAVES_EQ_CONTROL_TOPOLOGY_CENSUS_PHASE1.md) | Waves EQ 拓扑普查第一阶段 |
| [WAVES_STATIC_EQ_RECOGNIZER_PHASE2.md](docs/WAVES_STATIC_EQ_RECOGNIZER_PHASE2.md) | 通用静态 EQ 识别器设计第二阶段 |
| [WAVES_STATIC_EQ_PRODUCTION_PHASE3.md](docs/WAVES_STATIC_EQ_PRODUCTION_PHASE3.md) | 通用静态 EQ 生产实现第三阶段 |
| [SPECTRAL_DYNAMICS_DEVELOPMENT_TASKS_V1.md](docs/SPECTRAL_DYNAMICS_DEVELOPMENT_TASKS_V1.md) | 频谱动态开发任务清单 |
| [ROUTING_ENGINE_DIAGNOSIS.md](docs/ROUTING_ENGINE_DIAGNOSIS.md) | V0.5 路由引擎架构审计与诊断（2026-04-18） |
| [ROUTING_IMPLEMENTATION_PLAN_FINAL.md](docs/ROUTING_IMPLEMENTATION_PLAN_FINAL.md) | `te::RackType` 终极重构方案（已实施架构的决策记录） |
| [ROUTING_EXECUTION_TASKBOOK.md](docs/ROUTING_EXECUTION_TASKBOOK.md) | V0.5 路由引擎执行任务书 |
| [DAW_NATIVE_AGENT_HARNESS_PLAN.md](docs/DAW_NATIVE_AGENT_HARNESS_PLAN.md) | DAW 原生 agent harness 架构规划（v0.2+ 时期） |
| [agent_action_workflow_v1_master_plan.md](docs/agent_action_workflow_v1_master_plan.md) | Agent Action Workflow v1 总规划（草案 v0.3，后被能力族表述取代） |
| [automation_envelope_tool_layer_v0.md](docs/automation_envelope_tool_layer_v0.md) | Automation/Envelope 工具层 v0 架构设计稿 |
| [project_prep_capability_layer_v0.md](docs/project_prep_capability_layer_v0.md) | 项目准备能力层 v0（A 阶段的能力族重表述） |
| [static_mix_capability_contract_v0.md](docs/static_mix_capability_contract_v0.md) | Static Mix 能力族契约 v0 |
| [mix_benchmark_manifest_v0.md](docs/mix_benchmark_manifest_v0.md) | 本地真实工程素材登记册（2026-06-29） |
| [contextruntime_fix_spec.md](docs/contextruntime_fix_spec.md) | ContextRuntime 上下文瘦身修复规格（2026-08-11，修复已完成） |
| [PROJECT_AWARE_CAPABILITY_RUNTIME_V1_GATES.md](docs/PROJECT_AWARE_CAPABILITY_RUNTIME_V1_GATES.md) | 能力运行时 v1 迁移门（2026-07-13 快照，迁移已完成） |

### 旧跨端审计与路线图快照

| 文档 | 说明 |
|---|---|
| [V1_1_CROSS_END_AUDIT_SUMMARY.md](docs/V1_1_CROSS_END_AUDIT_SUMMARY.md) | V1.1 跨端架构审计（Python bridge 时期） |
| [v1.2 cross end audit summary.md](docs/v1.2%20cross%20end%20audit%20summary.md) | V1.2 跨端只读体检提示词 |
| [V1_2_CROSS_END_HEALTH_CHECK.md](docs/V1_2_CROSS_END_HEALTH_CHECK.md) | V1.2 跨端体检结果（2026-05-06） |
| [VIT_KERNEL_SERVICE_BOUNDARY.md](docs/VIT_KERNEL_SERVICE_BOUNDARY.md) | 内核服务边界审计（2026-05-17） |
| [V0.8 plan.md](docs/V0.8%20plan.md) | V0.8 路线图快照 |
| [Vit-DAW_V1.1_Roadmap.md](docs/Vit-DAW_V1.1_Roadmap.md) | V1.1 攻坚阶段蓝图快照 |
| [Vit-DAW 项目功能总览.md](docs/Vit-DAW%20项目功能总览.md) | 已实现功能总览（以代码为准的旧快照） |
| [Vit-DAW 展示版发布前检查清单.md](docs/Vit-DAW%20展示版发布前检查清单.md) | 展示版发布前检查清单（2026-03） |
| [Vit-DAW 调试开关热切指令.md](docs/Vit-DAW%20调试开关热切指令.md) | Godot 运行中热切调试日志指令 |
| [Vit-DAW 频谱漂移问题复盘.md](docs/Vit-DAW%20频谱漂移问题复盘.md) | 频谱漂移问题复盘（2026-03） |

---

## 三、已废弃（4，已迁至 docs/archive/）

| 文档 | 废弃原因 |
|---|---|
| [ROUTING_IMPLEMENTATION_PLAN.md](docs/archive/ROUTING_IMPLEMENTATION_PLAN.md) | 被 `ROUTING_IMPLEMENTATION_PLAN_FINAL.md` 明确取代 |
| [ARCHITECTURE_V1_IMPLEMENTATION_STATUS.md](docs/archive/ARCHITECTURE_V1_IMPLEMENTATION_STATUS.md) | 2026-07-13 实施状态快照，已与现状严重脱节，易误导 |
| [TRAE_MIGRATION_PROMPT.md](docs/archive/TRAE_MIGRATION_PROMPT.md) | Trae 迁移提示词包，一次性工具产物，与现行工作流无关 |
| [TRAE_MIGRATION_SKILL.md](docs/archive/TRAE_MIGRATION_SKILL.md) | 同上（Trae 迁移执行 skill） |

---

## 四、docs/vsp/ 子目录（13 份 + schema/）

VSP（Vit Session Protocol）是现役的会话契约体系，入口见 [docs/vsp/README.md](docs/vsp/README.md)。

| 文档 | 状态 | 说明 |
|---|---|---|
| vsp/README.md | 现行 | VSP v1 Foundation 入口与文档导航 |
| vsp/VSP_V1_FOUNDATION_SPEC.md | 现行 | v1 Foundation 规格（角色/通道/传输/状态/资产/权限） |
| vsp/VSP_V1_EXECUTION_TARGET.md | 现行 | v1 可执行目标说明 |
| vsp/VSP_V1_CONFORMANCE_AND_SMOKE.md | 现行 | 一致性测试与烟测矩阵 |
| vsp/VSP_PHASE1_SCHEMA_AND_CHANNELS.md | 现行 | envelope / 五通道骨架 / legacy IPC adapter 设计 |
| vsp/VSP_TRANSPORT_BINDING_V1.md | 现行 | 传输绑定契约 v1 |
| vsp/VSP_HUB_ARCHITECTURE.md | 现行 | 独立 VspHub 长期架构 |
| vsp/VSP_HUB_TRANSPORTS.md | 现行 | Hub 传输（POST /vsp、/vsp/stream、端口与环境变量） |
| vsp/VSP_HUB_EXTENSION_API.md | 现行 | Hub 第三方扩展接入 API |
| vsp/VSP_MIGRATION_FROM_LEGACY_BRIDGE.md | 现行 | legacy bridge → VSP Hub 迁移边界与收口门槛 |
| vsp/VSP_PHASE0_AUDIT.md | 历史记录 | 迁移前通信链路审计快照 |
| vsp/VSP_HUB_VERIFICATION_2026_07_02.md | 历史记录 | Hub 验证记录（2026-07-02） |
| vsp/VSP_V1_CLOSEOUT_REPORT.md | 历史记录 | v1 Foundation 收口报告 |

---

## 维护规则

1. 新文档合入 `docs/` 时必须同步在本索引登记并标注三态；未登记视为未合入。
2. 文档状态发生实质变化（被实现超越/成为记录）时，更新本索引相应条目。
3. 新增废弃文档一律 `git mv docs/<name>.md docs/archive/<name>.md`，保留历史，并更新第三节。
