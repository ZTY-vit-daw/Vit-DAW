# MIX-CAP-RECON-1 勘察回执：混音能力层现状盘点（PC 线启动勘察）

- 执行：GLM-5.3 flash（ZCode PC 会话）/ 2026-10-10 夜 / 零代码只读
- 领取基线：origin/main=1c1a447a；勘察期间工作树无本卡代码改动
- 方法：全仓 grep 定位 + 关键文件精读 + 两个只读 Explore 子代理（底盘面/基材面）+ 前序勘察卡交叉引用（MIXLAYER-RECON-1 已验收矩阵作 §3 基线）
- 配套：[GREP-APPENDIX.md](GREP-APPENDIX.md)（原始 grep 输出摘录）

---

## §1 固定 runtime 能力清单

### 1.0 总进入模型（一句话版）

chat 消息进入 `handleChat` 后第一道能力 seam 是 `handleCapabilityRuntimeCanary`（`agent/internal/chat/server.go:2467`，先于语义入口路由）；它经 `resolveCapabilityOwner`（`capability_owner.go:24`）解析 capability ID——三来源：①请求上下文显式 `capability_id`；②话术推断 `inferProjectAwareCapability`（`capability_owner.go:206-247`）；③本会话活动 v1 Session 粘滞（`capability_owner.go:160-172`）。解析出 ID 后 `OwnerPolicy.Select(explicitV1=true, …)`（`orchestration/owner_policy.go:62-66`）恒判 EngineV1，按 ID 分发到六个固定编排 runtime（`capability_runtime_canary.go:109-126`）。解析不出 ID → 返回 handled=false，消息落回普通自由态循环。**rollout 环境变量 `VIT_CAPABILITY_RUNTIME_V1_ROLLOUT` 的策略表只登记了 b2/b3（`owner_policy.go:31-45`），但因 explicitV1 恒为 true，该表在当前调用点不构成额外门**。

### 1.1 固定编排 capability runtime（canary seam 直达，共 6 现役 + 1 退役）

| # | Capability ID（段位） | 功能 | 入口（话术/上下文） | 接回自由态现状 | 退役候选 |
|---|---|---|---|---|---|
| 1 | `static_mix.static_balance.v0`（B2） | 全轨静态 fader 平衡：mix.observe+TOM 投影→solver 候选→冻结 ActionSet→授权→`StaticBalanceVSPPort` 执行+推子回读+fresh MOM 验证（`capability_runtime_canary.go:131-252,307-374`） | 话术「b2/static balance/静态平衡/推子平衡/静态主次/突出主唱」等（`capability_owner.go:212-219`；goalrunner 话术面 `goalrunner_chat.go:3967-3975`）+显式 capability_id+会话粘滞 | **仅编排入口**。循环内另有同名工具包 `static_mix_static_balance`（`goalrunner_chat.go:4266-4271`，含 mix.propose_tick/apply_tick/rollback_tick 小步通道）——工具面循环内可达，v1 solver/冻结编排面不可达 | 否（B2/B3 迁移 Phase A-F 已完成，`PROJECT_AWARE_CAPABILITY_ORCHESTRATION_ARCHITECTURE_V1.md:509`） |
| 2 | `static_mix.pan_layout.v0`（B3） | 声像布局：中心元素/左右展开/宽度/mono 风险 | 话术「b3/pan layout/声像布局/声场布局」（`capability_owner.go:221-224`；`goalrunner_chat.go:3977-3985`） | **仅编排入口**（`pan_layout_runtime_canary.go:21`）；同上，循环内有 pan 小步 tick 工具面 | 否（同 B2） |
| 3 | `static_mix.low_end_relation.v0`（B4） | 全工程低频关系诊断→generic-EQ 装载批+原子参数处理（经普通 Agent EQ runtime）：`b4.eq_plugin_load_batch.governed`（`b4_eq_runtime.go:26,258`）+`b4.semantic_eq_batch.governed`（`:618`） | 话术「b4/low end relation/低频关系/低频遮蔽/低频占用」（`capability_owner.go:226-232`）；**选轨低频话术不劫持**——须显式 b4 标记才进 B4（`capability_owner.go:141-144` explicitB4InvocationMarker） | **仅编排入口**（`low_end_relation_runtime_canary.go:19`） | 否（`B4_EQ_CLOSED_LOOP_SMOKE_CONTRACT.md` 现行） |
| 4 | `fine_mix.frequency_cleanup.v1`（C1） | 全工程频段清理：逐轨分类→静态 EQ 确认制处理（共享普通 Agent EQ runtime）：`c1.eq_plugin_load_batch.governed`（`c1_frequency_cleanup_runtime.go:27,365`）+ c1.semantic_eq_batch（`agentprotocol/refschema.go:204`） | 话术「c1/frequency cleanup/频段清理/频率清理/全工程eq清理」（`capability_owner.go:235-240`）；选轨 EQ 上下文不劫持（`:145-147` explicitC1InvocationMarker 反门） | **仅编排入口**（`c1_frequency_cleanup_runtime.go:57`） | 否（`C1_FREQUENCY_CLEANUP_V1.md` 现行） |
| 5 | `fine_mix.dynamic_control.v1`（C2） | 全工程动态控制批：观察→目标假设→PCA 候选→装载批+参数批：`c2.dynamic_plugin_load_batch.governed`（`c2_dynamic_batch.go:26,207`）+`c2.dynamic_parameter_batch.governed`（`:471`）；执行经 harness `rack.add_node`（`:660`） | **仅显式标记**「c2/fine_mix.dynamic_control」——注释明言 C2 是 project capability 入口，不做选轨动态话术的词法后继（`capability_owner.go:242-247`） | **仅编排入口**（`handleDynamicControlRuntime`→`handleC2ProjectBatchRuntime`，`c2_dynamic_batch.go:50,57`；插件选择确认卡=server.go:4300 交互续接） | **legacy 单目标 runtime 是退役候选**：`handleLegacyC2SingleTargetRuntime` 仅存续反序列化旧持久化 Session，新调用不可达（`c2_dynamic_control_runtime.go:30-33`） |
| 6 | `agent.effect.eq_control.v0`（semantic EQ） | 选轨语义 EQ：冻结构拓扑+preimage+确认+回读+回滚+验证（`semantic_eq_runtime.go:810`） | canary seam 分发（`capability_runtime_canary.go:121-123`）+**自由态循环内可达**：agentloop 语义 EQ 意图门+工具白名单（`agentloop/message_loop.go:2625,4867-4897,5406-5412`）——full_project 范围 EQ 走 canary，选轨 EQ 留在普通循环 | **双入口：循环内可达+编排入口**（六者中唯一循环内可达的插件参数面） | 否 |
| 7 | `static_mix.focus_position.v0`（旧 B5） | 核心元素定位（历史） | 显式 ID 兼容重定向→B2（`capability_owner.go:19-20`；`capability_runtime_canary.go:28`） | 不适用 | **已退役**（`static_mix_capability_contract_v0.md:11`；`orchestration/builtins.go` 无注册） |

注册面（声明层）：`orchestration/builtins.go:5-63` 六个 CapabilityDefinition（static_balance/pan_layout/low_end_relation/frequency_cleanup v1/dynamic_control v1/eq_control v0）——与上表 1-6 一一对应；适配器所有权留在各域包，builtins 只持有声明。

### 1.2 确定性快路径 FastPathRouter（自由态循环内 preflight，共 10 项）

注册清单（`fastpath/router.go:79-90`，完整性契约 `DefaultEntryNames`；挂点 `agentloop/message_loop.go:444` 一带；A4 scope note 明言 fastpath preflight 是 harness plumbing 不在固定编排标记集内——`fs_largeproject_smoke.ps1:1314`）：

| 入口 | 功能 | 对应段位 |
|---|---|---|
| static_mix_capability_contract | 静态混音能力契约 preflight | B 族 |
| project_blackboard_status | 工程黑板状态汇报（blackboard status_report） | 跨段 |
| clip_fade_gain_set / clip_fade_gain_read | clip fade/gain 读写 | A4 |
| strip_silence_suggest | Strip Silence 推荐（分析→pending apply，不直接改工程） | A4 |
| clip_range_split | clip 范围切分 | A4 |
| stems_folder_import | 分轨文件夹导入 | A1 |
| pending_section_markers_apply | 段落 marker 待确认写入 | A5 |
| natural_mix_observation | 自然混音/音频观察意图匹配（`fastpath/natural_mix.go:10,33`） | 观察 |
| static_mix_gain_staging_context_pack | B1 增益架构上下文包 | B1 |

这 10 项**全部自由态循环内可达**（每循环单点尝试、按注册序短路）。

### 1.3 循环工具面（goalrunner 工具包 + tools catalog，自由态可达）

- **B1 gain_staging**：话术路由「b1/gain staging/增益结构/电平健康/全工程电平检查」（`goalrunner_chat.go:3940-3965`）→ 工具包 project.state/mix.observe/mix.read/mix.derive/clip.gain.set(set_batch)/track.group.apply_control 等（`:4256-4264`）。无独立固定编排 Session（与 MIXLAYER-RECON-1「B1 无 Session 形态」读数一致）。
- **B2/B3 小步通道**：`static_mix_static_balance`/`static_mix_pan_layout` 工具包含 mix.propose_tick→apply_tick→rollback_tick（`goalrunner_chat.go:4266-4271` 起）；执行工具 `mix_apply_static_balance_batch`/`mix_apply_pan_layout_batch`（`tools/catalog.go:951-952`）。
- **A4 clip 族工具**：clip.strip_silence.analyze/suggest/apply/apply_batch（`tools/catalog.go:996-999`；system prompt 注入 `server.go:5892`）。
- **mixboard 决策账本**：`mix_decision_record.v1` 投影与 needs_review 联动（`MIXBOARD_DECISION_LEDGER_V1.md` 现行；`mixboard.go:3232` gain_staging_observation 等维度键）。

### 1.4 legacy mix session 工作流（休眠面）

`mix_session_workflow.go`（mix_session_entry→planner prep→control surface→fast tick→review，auto_mix/co_mix 双模式，`:16-100` 常量面）——**默认关闭**：`legacyMixSessionWorkflowEnabled` 需 env `VIT_ENABLE_LEGACY_MIX_SESSION_WORKFLOW=1` 或显式上下文旗标（`:464-470`）；入口 `runMixSessionEntryChat`（`:129`）在 main 生产代码零调用者（仅测试调用）。**退役候选：是**（整面 legacy 门控+入口无人调用；但 mix tick/audition 的自由态站面另有在用实现，不随本面退役）。

### 1.5 自由态实验域表（D1/D2/FAM 标签，循环本体）

experiment 域表九族全部 PluginBound 且 Journal 工具=set_plugin_param：static_eq（`experiment/d1s1_domains.go:155`）/broadband_compression（`:214`）/de_esser（`:257`）/transient_shaper（`:307`）/limiter（`:358`）/gate_expander（`:408`）/multiband（`:463`）/pan（`:525`）+ track_gain（D1-S1 形态，`chat/free_state_d1_runtime.go:101`）。路由标签 D1-S1/D2-1.5/FAM1-S1/FAM2-S1/FAM4-S1（`free_state_d1_runtime.go:44-60`）。**这是自由态循环的实验面本体，天然循环内可达**——与 §2-B 时域效果器参数写回通道共用。

### 1.6 支撑模型/投影包（非编排入口，清单附注）

staticbalance/panlayout/lowendrelation（确定性 solver/模型）、mixstyle（.vms 风格权重）、mixdiagnosis、frequencycleanup/dynamiccontrol 计划类型、queryengine route.go、COM/masking 观察投影。均为上述编排面或循环面的被调方。

---

## §1-A 交叉核对：runtime 清单 × fastpath/A4 标记（零遗漏）

A4 断言=自由态运行面上固定编排**零命中**（`fs_largeproject_smoke.ps1:1322-1341`，Phase 8 zero-hit audit；判定组 `$a4 = $fixedOrchestrationAudit.zero_hits`，`:1324`）。文本标记实际 **6 枚**（卡面「五标记」=原始 5 枚，第 6 枚 `c2.dynamic_plugin_load.governed` 系 review A note 1 增补，脚本注释 `:1276-1278`）。逐标记→生产者：

| A4 文本标记 | 生产者（固定编排） | 锚点 |
|---|---|---|
| semantic_eq_batch | B4 `b4.semantic_eq_batch.governed`（c1 端口同源复用） | b4_eq_runtime.go:618；refschema.go:202-204 |
| eq_plugin_load_batch | B4 `b4.eq_plugin_load_batch.governed` + C1 `c1.eq_plugin_load_batch.governed` | b4_eq_runtime.go:26；c1_frequency_cleanup_runtime.go:27 |
| dynamic_plugin_load_batch | C2 `c2.dynamic_plugin_load_batch.governed` | c2_dynamic_batch.go:26 |
| dynamic_parameter_batch | C2 `c2.dynamic_parameter_batch.governed` | c2_dynamic_batch.go:27,471 |
| c2_dynamic_plugin_selection | C2 插件选择交互 workflow | c2_dynamic_control_runtime.go:625 |
| c2.dynamic_plugin_load.governed | C2 单轨受治装载命令 | c2_dynamic_plugin_load.go:20（分发 c2_dynamic_control_runtime.go:520 一带） |

遥测 LLM source 标记 **8 枚**（`fs_largeproject_smoke.ps1:1219-1228`）：b4_project_eq_planner / b4_project_eq_instance_selection / b4_project_treatment_planner（b4_eq_planner.go:180/43/124）、c1_project_eq_planner / c1_project_treatment_planner（c1_frequency_cleanup_planner.go:108/41）、c2_project_candidate_planner / c2_project_target_planner / c2_project_treatment_planner（c2_dynamic_control_runtime.go:308/214、c2_project_treatment.go:114）。

**核对结论**：①6+8 枚标记全部归属 B4/C1/C2 三族固定编排，无一流出；②B2/B3 不产生任何标记（其执行走 track_gain_adjust/pan tick 命令与 StaticBalanceVSPPort，无 governed batch 命令）——与脚本 scope note「capability 广告面/工具目录不计、fastpath preflight 不计」一致；③卡面所列 11 个名字（c1_frequency_cleanup/c2_dynamic_batch/c2_dynamic_control/b4_eq/semantic_eq/static_balance/pan_layout/low_end_relation/strip_silence/clip_fade_gain/gain_staging）全部在 §1.1-1.3 落位，无遗漏、无幽灵项。**零遗漏成立。**

---

## §2 能力底盘面

### 2.A 插件控制链（PCA 面）——已收口

- **契约**：`docs/PCA_PLUGIN_LOAD_GATE_V1.md`（现行，CURRENT-STATE.md:77）。双道独立 fail-closed：①推荐层只暴露 promoted+指纹匹配+family 匹配+coverage 完整的精确标识（文档 :5-11）；②Harness 内核派发前二次复核（授权对象/track/path 精确比对/重算指纹/重读 PCA，:12-15）。门只管 Agent 发起的装载；DAW/UI 手动装载不受影响（:21-23）。
- **attestation vs certification 分工**：attestation=准入库（issued/promoted/stale/revoked 状态机，`processorattestation/store.go:59-170`；v2 身份无关族 limiter/gate_expander/de_esser/transient_shaper/multiband_dynamics，`v2.go:16-22`）；certification=产线认证 runner（临时轨+快照+typed apply/readback/restore+清理，`processorauthority/local_processor_certification.go:36`），回执经 Import 入库；认证装载例外=短时单次 token（`chat/processor_certification_entry.go:107-129` 四重校验）。
- **族注册表**：`processorregistry/registry.go:155,186`（PCARequiredCoverage）。语义索引：`pluginsemantics/index.go`。
- **三条编排路径接入**：C2 候选二次复核（`chat/c2_dynamic_plugin_load.go:66-103`）；B4 装载 plan 准入复核（`chat/plugin_recommendation.go:776-780`）；自由态 D1 processor_selection 相位（`agentloop/free_state_reasoning.go:1265,2464`）+机器本地白名单绑定（`chat/free_state_d1_runtime.go:436`）。
- **最终门**：`harness/harness.go:1955` enforceAgentProcessorLoadGate（TOCTOU 防护注释 :1949-1954）。
- **内核无感知**：VitApp 源码零 `dynamic_plugin_load`/`pca_load_gate` 字符串；接收端=rack_add_node 命令处理器（`VitApp/Source/Service/CommandDispatcher.cpp:3071`）。门控完全在 agent 侧。

### 2.B 时域效果器接入面——参数写回已打通，差分采集缺生产者

- **FXM 数据结构齐备**：Input{Chain,Conditions,Baseline,Processed}（`fxm/types.go:19-29`）、Measurement RMS/LUFS/Peak/Crest/延迟/频段（`:59-76`）、差分门控 source_revision/window/format 不一致即 blocked/stale（`fxm/projection.go:27-38`）。族无关设计（schema 无 family 字段）。
- **缺口=差分载体生产者**：`fxm.Input` 全仓唯一非测试消费点从调用方 `req.Args["fxm_measurement"]` 反序列化（`mixboard/mixboard.go:3992-4011`）——agent 侧无任何自动采集 bypass/processed render 对喂入 FXM 的生产路径；CCB v1 view 目录不含 FXM（`OBSERVATION_PROJECTION_MANIFEST.md:29,72` 缺口登记）。
- **内核渲染探针现状**：压缩器专属双 tap 离线探针（bypass/processed 成对渲染，唯一）`compressor_dual_tap_probe`（`CommandDispatcher.cpp:3149`→`TransportAudioService.cpp:1405`，render 身份串 com2rp.v1 :1494-1512，窗口上限 120s，作用域强校验 :131-230）；通用链离线渲染 `l2_render_probe`（`CommandDispatcher.cpp:3143`→`TransportAudioService.cpp:1226`）；试听预览=Audition Preview Plane（`AuditionPreviewService.h:28-53`）。**transient/de-esser/limiter 无内核成对渲染探针**。
- **参数写回已打通**（不是只有观测）：域表四族全部 set_plugin_param Journal 通道（§1.5 锚点）；双模式写 legacy set_plugin_param+归一化批写 plugin.set_params_batch（readback+base_revision，`executionports/staticeq_vsp.go:262-277`）；内核 handler 写后 flush+回读（`PluginRackControlService.cpp:1835,1930-1955`）；agent 侧 param_id 需先有参数快照（`harness/plugin_param_guard.go:24-31`）；attestation coverage 轴与域表 SemanticIntentCoverage 一一对应（de_esser=sibilance_reduction 等，`d1s1_domains.go:288-291,338-343,388-393`）。
- **就位度结论**：写回链（白名单+PCA 闸）✅；观测投影结构 ✅（无生产者）；成对差分采集 🟡（仅压缩器探针）；通用时域差分接入 ❌——即 roadmap L2-4「接一个效果器配一套差分断言」的载体就位度=压缩器样板已立，其余族待复制。

### 2.C 自动化曲线面——不存在（可用面），引擎原生能力未暴露

- **内核**：命令注册表（CommandDispatcher.cpp:2259-3210 全量枚举）零 automation/envelope 命令；轨 JSON（createTrackState :841-870）不产出自动化字段；透明性探测把 automationcurve/automationpoints 当**障碍**（拒绝压缩探针作用域，TransportAudioService.cpp:57-61,198）。tracktion_engine 子模块自带 te::AutomationCurve/AutomationCurveList/AutomationRecordManager（`tracktion_engine/modules/.../tracktion_AutomationCurve.h`）——VitApp 应用层从未调用。
- **agent**：AutomationLaneCount 读取点存在（`chat/capability_routing.go:36,496`，capacity 因子 automation_heavy 阈值 24 :545）但上游无数据，**实践中恒 0**（shadow 白名单压缩不保留自动化键，shadow.go:400-416；fail-open 设计）。
- **FREESTATE-REGION-IMPL-1 冻结卡**：解冻条件已随用户勘误改为 **C4 段落自动化批次**（卡面 :3，「原 B2 系误记，按 A-F 官方文档 §6.1 自动化=C4」）；实现保全于 runs/FREESTATE-REGION-IMPL-1-STOP/，代码不进库。
- **文档侧**：automation_envelope_tool_layer_v0.md 在 CURRENT-STATE 归「研究/设计阶段记录（历史）」（CURRENT-STATE.md:192）。
- **结论**：**❌ 不存在**（agent/内核可用面）；底层引擎具备原生能力未暴露；master_plan §6.3 DAW 缺口清单「线性包络线/automation」与裁定 3「真开发项=M3 自动化」一致。

---

## §3 A-F 现状对照表（按主管补注：官方定义=master_plan §6.1，不再推测）

权威定义：`docs/agent_action_workflow_v1_master_plan.md:217-295`（§6.1 Mix Workflow Queue v0：A=project_prep A1-A5 / B=static_mix B1-B4 并列 / C=细混 C1-C5 / D=母带前 D1-D4 / E=母带 E1-E4 / F=审查 F1）。用户方向裁定（`AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md:206`）：A–F 沿纪律序开发，每段完成后能力**全部接回自由态**；F=混音能力层完成的同时**固定编排淘汰**，全部由自由态处理。

基线引用：`coord/runs/MIXLAYER-RECON-1/RECON.md`（2026-10-04 已验收，23 节点矩阵+M1 工作量估算 6-10 天）。下表=该矩阵的**复核+六日增量**，逐节点状态：

| 节点 | 现状 | 关键锚点与增量注记 |
|---|---|---|
| A1 导入接收 | **完成（管道面）** | stems_folder_import fastpath（router.go:86）+L2-2 实证 61 轨导入（seg_primitives_smoke_20260926_231942 exit 0）+fs_largeproject_smoke import 段（tracks_created=61）。无独立 Session 形态（MIXLAYER 读数不变） |
| A2 技术完整性 | **观察面完成；无 Session** | TIM `tim.projection.v0` 现行；路由 ID project_prep.technical_integrity.v0（capability_routing.go:611）；自由态循环消费 TIM。与 MIXLAYER「A 族零 Session」一致 |
| A3 轨道整理 | **部分（观察+话术面）** | TOM `tom.projection.v0_2` 现行；agentLoopTrackOrganizationIntent 话术路由（goalrunner_chat.go:3775 一带）；整理建议/确认执行链未见独立闭环 |
| A4 clip 清理 | **完成（循环内可达）** | 本盘点最完整段：clip_fade_gain_set/read+strip_silence_suggest+clip_range_split 三条 fastpath（router.go:82-85）+clip.strip_silence.* 四工具（catalog.go:996-999）+clip.gain.set_batch+INTENT-WIRE split 先例 |
| A5 段落地图 | **部分** | pending_section_markers_apply fastpath（router.go:87）+project_marker 工具；SEG 段落发现=L2-2 DSP 层（roadmap §L2-2），marker 建议生成面未独立成环 |
| B1 gain staging | **完成（契约+工具面）** | 契约 B1.1 fader unity/B1.2 clip gain 校准（static_mix_capability_contract_v0.md:70-72）；工具包+fastpath context pack（§1.3）；无 Session 形态（与 MIXLAYER 一致） |
| B2 静态平衡 | **完成（全链）** | v1 Session 全链：shadow→solver→freeze→authorize→execute→verify（capability_runtime_canary.go:131-374）；Phase A-F 迁移切面完成（架构文档 :509）；循环内另有 tick 小步通道 |
| B3 声像布局 | **完成（全链）** | 同 B2 批次（pan_layout_runtime_canary.go:21；mix_apply_pan_layout_batch catalog.go:952） |
| B4 低频关系 | **完成（全链）** | EQ 闭环冒烟契约现行；装载批+semantic_eq_batch 双 governed 命令（§1.1 #3） |
| C1 频段清理 | **完成（全链）** | fine_mix.frequency_cleanup.v1 现行（C1_FREQUENCY_CLEANUP_V1.md）；c1 双 governed 命令 |
| C2 动态控制 | **完成（全链）** | fine_mix.dynamic_control.v1 现行（C2_DYNAMIC_CONTROL_V1.md）；批化 runtime 为现役入口（c2_dynamic_batch.go:50）；legacy 单目标=退役候选（§1.1 #5） |
| C3 空间与深度 | **未动（能力级）** | capabilityIDForStage 有映射 `fine_mix.space_depth.v0`（capability_routing.go:619）但**无 runtime 分发、无 Session**（canary seam 六分支无 C3，capability_runtime_canary.go:109-126）；内核 reverb/delay 语义控制面缺（MIXLAYER：plugin_grabber 无 reverb/delay inspector）；FXM 可当差分载体但无生产者（§2.B） |
| C4 段落自动化 | **未动（真开发项）** | 自动化曲线面不存在（§2.C）；FREESTATE-REGION-IMPL-1 解冻条件=C4 批次；tracktion_engine 原生 AutomationCurve 未暴露=起点资产 |
| C5 总线处理 | **未动** | bus/send/FX return 结构化读写缺（MIXLAYER §6.3 第 6 项）；内核 rack_* 是机架图形面非总线处理；track.group.apply_control 可当部分底层 route |
| D1 响度峰值 | **部分（观察）** | mix.observe peak/rms/loudness_ranking/headroom+RLM `reference_level_model.projection.v0`；无能力 Session。裁定 2：D/E 母带线**实现后置**（decisions/2026-10-04 :13） |
| D2 翻译检查 | **部分（观察）** | stereo correlation/mono 风险观察在；无 Session |
| D3 参考曲对比 | **部分（观察）** | RLM 投影（user intent+RenderBindings fail-closed）；无参考曲导入/对比命令；audition.* 家族 10 命令在内核 dispatcher 不在 agent catalog（MIXLAYER 附注） |
| D4 版本输出 | **未动** | export variant/stem 管理缺；内核 start_render 未暴露 agent |
| E1-E4 母带 | **未动（裁定后置）** | 通用插件参数面可当底层 route；无任何能力形态。limiter 域在实验域表（FAM4）=自由态侧已有单点 |
| F1 混音审查 | **部分** | mix_report.v1 账本汇总+Mixboard 决策账本（MIXBOARD_DECision_LEDGER_V1 现行）；mix_workflow_queue.v0 代码零命中纯设计（MIXLAYER）；黑板无跨会话持久层 |

**六日增量（2026-10-04 盘点后）**：①自由态台账持久化 blocker 修复（FS-LEDGER-PERSIST-1 pass，跨轮 stored_receipts=2 实证）——MIXLAYER 问 4「settle→黑板零桥接」的收据存活半边已解，黑板持久层仍缺；②REFSCHEMA-M5 五词条注册（b4/c1 semantic_eq_batch 主/reconcile+c2.dynamic_plugin_load_batch）——固定编排回执 ref 面收口；③fs_largeproject_smoke A4 标记集增补 c2.dynamic_plugin_load.governed+fsLoopAdmissionStatus 通道；④C3/C4/C5/D/E 零推进（与「真开发项=M2 时域+M3 自动化」的排序一致）。

**两个待核点回执**：
1. **「B2 自动化曲线批次」名实**：已由用户 2026-10-10 勘误解决——原 B2 系误记，官方文档 §6.1 自动化=C4（FREESTATE-REGION-IMPL-1 卡面 :3 勘误注记；本卡 §2.C 锚点链一致）。
2. **authority 归属**：PROJECT_AWARE_CAPABILITY_ORCHESTRATION_ARCHITECTURE_V1 为 runtime authority（迁移 Phase A-G 语义，:459-511——**注意该文档的 Phase A-F 是迁移切面命名空间，与产品 A-F 段位同字母不同义**，对照时勿混）；A-F 族结构按用户指定以 master_plan §6.1 为权威。

**下一批排序建议（讨论输入，不代决）**：MIXLAYER 裁定的 M1 拆卡五序（黑板持久层+供血桥→C3 观察侧收口→D1-D3 观察能力化→F1/queue/黑板三视图对齐等，RECON.md:113-128）仍成立；PC 混音线视角的补充：A–F 纪律序的「接回自由态」验收判据已有现成载体=fs_largeproject_smoke 的 A4 零命中+A3 AB 往返腿，每段开发完成时建议把「该段能力在自由态循环内可达」写成冒测断言（现成模式=§1.2 fastpath 注册清单完整性契约）。

---

## §4 超大工程基材衔接

（Explore 子代理全锚点勘察，本节为其结论的核对收录）

### 4.1 三工程台账

| 工程 | 规模 | 磁盘 | 仓库内登记 |
|---|---|---|---|
| sattelites | 61 轨/3.74GB/48k24b/228.6s | E:\BaiduNetdiskDownload\yingge - sattelites tracks out（已实存验证） | mix_benchmark_manifest_v0.md:16-24（训练/开发集） |
| Weekend Lover | 104 轨/7.22GB/258.6s | E:\BaiduNetdiskDownload\yingge - Weekend Lover tracks out（已实存验证） | manifest :30-37（**封存集**：不用于本阶段调试，master_plan:99,214,488） |
| 长河分轨 | 93 轨（嵌套子目录） | E:\BaiduNetdiskDownload\长河分轨（仅确认文件夹存在，无元数据清单） | **仅 3 处文字提及**（roadmap:206、FS-LARGEPROJECT-SMOKE-1 卡:16、本卡）；manifest 无条目 |

入库纪律：音频素材不进 git（manifest:8）；VitApp 源码零硬编码；唯一隔离副本=run 工件 sattelites_61.vit（FS-LARGEPROJECT-SMOKE-1/fs_largeproject_smoke_20261010_203348）。

### 4.2 导入链就绪度——sattelites 已证，另两部未跑

- fastpath 入口在位（stems_folder_import）；L2-2 实证 61 轨全量导入+L3 管线（run=seg_primitives_smoke_20260926_231942 exit 0，工件 coord/runs/L2-2-SEG-SMOKE-1/）。
- fs_largeproject_smoke 203348 回归：import 段 tracks/clips/readable=61/61/61；preheat 30s 就绪 dad 61/61 ready+122 feature jobs；**A2 站图 PASS**（FS-LEDGER-PERSIST-1 回归轮，rulings/2026-10-10-NIGHT-BATCH pass）。
- Weekend Lover 仅只读 preflight 腿（SMOKE_TESTS.md:910-912），无全量导入 run；长河嵌套子目录形态完全未验证；扩样本卡未立（FS-LARGEPROJECT-SMOKE-1 卡:16「另立扩样本卡」尚在计划）。

### 4.3 A/B 试听链就位度——harness 端端到端可用；大工程上零触发

- 数据面：MOM ABResult（mom/types.go:15,219,251、projection.go:282-288）+mixboard deriveABResult（catalog.go:352,579,708）；单元冒测绿（run_ab_result_smoke.ps1，实跑 exit 0 记录 KERNEL-RENDER-FREEZE-FIX-1/manifest.md:17）。
- HTTP 试听面：/agent/audition/status|select|stop|judgment|inspect_candidate|apply_candidate（chat/server.go:577-582）；盲听开关 VIT_DAW_AUDITION_BLIND（audition_events.go:28）。
- 端到端实证：JOURNEY-4-REV journey_ab_judgment 盲听腿 20261008_200153 = judgment_ok_blind 通过（receipt.md:27-28,41）。
- **缺口**：61 轨大工程上 A/B 往返腿（A3）至今零触发——7 个 FS-LARGEPROJECT-SMOKE run audition_events 全为 0（203348 同样，A3=FAIL not_driven_no_card，定性=模型证据完备性概率面，G3 已过，FS-LEDGER-PERSIST-1:53）。**大工程 A/B 腿走通仍是待办非事实。**

### 4.4 与混音线的规划关系

- 用户验收口径（roadmap:206）：全曲「检查一下当前工程有什么问题」→自由态同款输出→AB 试听链走通→**确认未调用固定编排**——即 §1 的 A4 零命中断言就是该口径的自动化载体。
- master_plan:186-190：用 sattelites 跑通真实工程自动混音最小闭环+benchmark manifest。
- 用户手测节点维持既有裁定：时域效果器第一个可感功能落地时体验（AGENTS 共享卡池节）；共享协作不新增手测义务。

---

## 边界与诚实声明

- 本卡纯只读勘察（零代码、零探针写源码树、零栈启动），不适用端侧烟测门槛；结论以源码锚点+全仓 grep+已验收 run 工件为据。
- 未逐行核实项（继承自子代理不确定性清单）：l2_render_probe 是否含轨插件链未读到行级；plugin.set_params_batch 内核 handler 函数名未定位到；Godot 前端自动化编辑入口未勘察。
- 「fastpath/A4 五标记」按现行脚本实为 6 文本标记+8 遥测标记（第 6 枚系增补），已按 6+8 口径核对（§1-A）。
- 领取提交 076ce106 事故如实记录：该提交除本卡 rename 外扫入了共享索引中并行会话已暂存的 ARR-CAP-RECON-1/GEN-CAP-RECON-1 两张卡 todo→doing 纯改名（100% rename，内容零改动，均在 coord/cards 允许直推面内）；三卡现均于 doing/ 且各自 owner 继续回填，无内容干扰。执行侧领取流程的「共享索引并发 mv」冲突面建议主管在协议层留意（§2.1 协调分支模式可规避）。
