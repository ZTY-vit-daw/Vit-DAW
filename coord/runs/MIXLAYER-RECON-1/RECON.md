# MIXLAYER-RECON-1 勘察回执：A-F 能力层实现度盘点

- 日期：2026-10-04；执行侧：GLM-5.3 flash（ZCode，只读勘察）
- 领取基线：`4de389a5900c54ec485d2802e6ecc32c72097b30`（工作树已有与本卡无关的 Settings.xml/default_project.xml 改动与 coord/runs 未跟踪目录，未触碰）
- 约束遵守：零代码改动、零探针写源码树、未动 sealed fixture；全部证据来自源码通读与 grep 锚点
- 依据：`coord/decisions/2026-10-04-capability-layer-harness-ruling.md` 裁定 3；ground truth=`docs/agent_action_workflow_v1_master_plan.md` §6 + `docs/static_mix_capability_contract_v0.md` + `docs/MIXBOARD_DECISION_LEDGER_V1.md`

---

## 问 1：黑板 / 账本 / queue / mix.report 四件实现度

| 件 | 判定 | 核心锚点 |
|---|---|---|
| `mix_workflow_queue.v0` | **纯设计（代码零命中）** | master_plan §6.1/§6.2 定义节点队列+9 态枚举；`grep mix_workflow_queue agent/` 全仓 0 命中；无持久化节点队列、无节点状态机 |
| `mix_decision_record.v1`（决策账本） | **已实现且已接线（B2/B3/B4/C1/C2 五族）** | `agent/internal/mixboard/decision_ledger.go`（832 行）+ `agent/internal/chat/mixboard_decision_projection.go` 读写两侧接线（详见下） |
| `mix.report`（`mix_report.v1`） | **已实现** | 工具注册 `tools/catalog.go:943`（RiskDirect 只读）+ alias `catalog.go:103` + 只读 guard 白名单 `agentloop/read_only_observation_guard.go:114`；实现 `harness/harness.go:4505 requestMixReport`（读账本 board + VSP snapshot 构当前 Project Cut + export_readiness 三态） |
| `project.blackboard.status_report.v0` | **部分实现（行为级，非契约级）** | 契约名全仓 0 命中；实为 `agentloop/project_blackboard_report.go` 的 NL 触发行为报告，每回合现算，无持久化（详见下） |

### 1a. 账本（最完整的一件）

- **实现**：`mixboard/decision_ledger.go` 四 schema 齐（`mix_decision_record.v1` / `mix_decision_board.v1` / `mix_decision_state_event.v1` / `mix_report.v1`，`:16-19`）；不可变记录按 Session ID 幂等（`decisionRecordsEqual :737`）；board 每次读时重算（`buildProjectDecisionBoard :403`）。
- **needs_review 影响联动：存在**——同能力 verified 相交 → supersede（`:444-449`）；后续写入与旧记录 `RecheckOn` 相交 → needs_review（`:450-455`）；authoritative reverted 事件停止写入传播（`RecordDecisionReversion :232`）。测试覆盖烟测合同 1-9：`mixboard/decision_ledger_test.go`（TestProjectDecisionBoardPropagatesRelevantCapabilityEffectsOnly、TestNewVerifiedDecisionSupersedesSameCapability、TestDecisionReversionIsExplicitAndStopsRevertedWritePropagation、TestRelatedDecisionRefsAreBoundedAndFlagUnsettledState 等 11 个）。
- **接线（写侧）**：`chat/mixboard_decision_projection.go:73 attachMixboardDecisionProjection` 挂在终态 CapabilitySession——B2 `chat/capability_runtime_canary.go:372`、B3 `chat/pan_layout_runtime_canary.go:240`、B4 `chat/b4_eq_runtime.go:825` + `chat/low_end_relation_runtime_canary.go:102`、C1 `chat/c1_frequency_cleanup_runtime.go:549`（另有 :129/:213 取消分支）、C2 `chat/c2_dynamic_control_runtime.go:872` + `chat/c2_dynamic_batch.go:618` + `chat/c2_dynamic_plugin_load.go:167`。C1 只读/无动作 Session 不落账（`mixboard_decision_projection.go:80-90`）。
- **接线（读侧）**：`attachMixboardDecisionContext`（bounded refs + requires_revalidation 进 ContextBundle 披露）挂在 C1/B2/B4/B3 四个 runtime（c1_frequency_cleanup_runtime.go:115、capability_runtime_canary.go:207、low_end_relation_runtime_canary.go:73、pan_layout_runtime_canary.go:91）。
- **缺口**：impact 契约 `capabilityImpactContract`（decision_ledger.go:607）仅覆盖 5 个 ID（B2/B3/B4/C1/C2）；**B1 无 impact 契约且 B1 根本不走 CapabilitySession**（走普通 agent 动作：track.group.apply_control / clip.gain.set_batch / mix tick），故 **B1 的决策从不进账本**。其余能力同理没有账本投影。

### 1b. 黑板（部分实现的关键短板）

- 实现形态：`agentloop/project_blackboard_report.go`——NL 触发词路由（`:10 messageLoopProjectBlackboardStatusRequest`）→ 确定性文本报告，**每回合从 project.state 快照 + 会话内执行记忆现算**（`state.executionMemory.PendingTrackOrganization/PendingSectionMarkers`、`state.recentObservation`，`:339-347`）。
- **A-F 状态矩阵是硬编码**：C/D/E/F 恒为"尚未记录为已执行"字符串（`:323-326`）；B 族状态读 context 键 `static_mix_capabilities` / `static_mix_capability_status` / `capability_statuses`（`agentloop/static_mix_capability_contract.go:177`）——**这三个键全仓无任何生产者**（grep 仅命中该消费者；`staticbalance/model.go:419` 只是把 `project_blackboard` 当可能的上下文容器去读 TOM）。实际结果：B 状态恒回落 not_started，仅当本会话存在 pending mix tick 时按操作类型推断 pending_confirmation（`static_mix_capability_contract.go:145-157`）。
- 导入变体：`messageLoopProjectBlackboardReportFromImport`（`:113-236`）——A1-A5 行来自 stems 导入数据 + TIM/TOM/EPM 投影，是一次性导入报告，非持久黑板。
- **结论：黑板当前没有跨会话持久层；"A-F 当前状态/已知用户目标/风险"只能看到本会话内存与当前快照。**

### 1c. mix.report 的边界

报告内容=账本投影（capability_summary / decision_timeline / verification_summary / unresolved / export_readiness，`decision_ledger.go:327 BuildMixReport`）。账本只收 B2/B3/B4/C1/C2 终态 Session，故报告覆盖面=账本覆盖面；`FinalMeasurements` 无生产者（只能由调用方传入），不传时 export_readiness 恒 `not_assessed`。

---

## 问 2：A-F 23 节点契约接线矩阵

图例：**已备**=投影/命令面在位，只差契约接线（编排类）；**部分**=只读观察/底层 route 在位，能力 Session 缺；**缺**=需开发（typed route 或能力本体）。

| 节点 | 契约 ID / 职责 | 投影 | 命令/执行面 | 能力 Session | 账本/黑板接线 | 判定 |
|---|---|---|---|---|---|---|
| A1 工程接收 | `project_prep.import_intake.v0` | DAD+导入计划 | `project.import_preflight` / `project.import_folder_as_stems`（内核+catalog，`tools/catalog.go:867,980`） | 无 | 无 | **已备（接线）** |
| A2 技术完整性 | `project_prep.technical_integrity.v0` | TIM `tim.projection.v0`（catalog 观察键 `observation.tim_projection`，`mixboard/catalog.go:252`） | 只读观察 | 无 | 无 | **已备（接线）** |
| A3 轨道整理 | `project_prep.track_organization.v0` | TOM `tom.projection.v0_2` | `project.apply_track_organization`（catalog.go:128 alias）+ `folder_track.create/set_routing_bus_enabled` + `track.move_to_folder`；pending 方案（executionMemory） | 无 | 无 | **已备（接线）** |
| A4 Clip 清理 | `project_prep.clip_edit_cleanup.v0` | EPM 建议（导入报告 A4 行） | `clip.fade.set/read`、`clip.gain.set/set_batch/read`、`clip.strip_silence.analyze/suggest/apply_batch`、`clip.move/resize/split/clone/remove`（catalog.go:985-998） | 无 | 无 | **已备（接线）** |
| A5 段落地图 | `project_prep.section_marker_map.v0` | EPM 段落推荐 | `project.markers.list/upsert/apply_section_markers/rename/delete`（catalog.go:869-873） | 无 | 无 | **已备（接线）** |
| B1 Gain Staging | `static_mix.gain_staging.v0` | B1 context pack（`capabilitycontext/gain_staging_suggest.go`、`agentloop/static_mix_gain_staging_context.go`）+ capability pack（`tools/capability_pack.go:314`） | track.group.apply_control / clip.gain.set_batch / mix tick（普通 agent 动作） | **无（不走 Session）** | **无 impact 契约→不进账本** | **部分** |
| B2 静态平衡 | `static_mix.static_balance.v0` | TOM+MOM static_level_relationship+project.state | 冻结 ActionSet→Execution Coordinator→fader + VSP 回读 + fresh MOM | ✅ 注册表 `orchestration/builtins.go:8` + `staticbalance` 包 + B2 runtime | ✅ 双侧 | **已落地** |
| B3 声像布局 | `static_mix.pan_layout.v0` | TOM+MOM+stereo 证据 | mix tick pan（小步）；v1 不动宽度 | ✅ `builtins.go:17` + `panlayout` 包 + runtime | ✅ 双侧 | **已落地** |
| B4 低频关系 | `static_mix.low_end_relation.v0` | band_energy+low_end 关系 | 共享 EQ runtime（`b4_eq_runtime.go`） | ✅ `builtins.go:25` + `lowendrelation` 包 + runtime | ✅ 双侧 | **已落地** |
| C1 频段清理 | `fine_mix.frequency_cleanup.v1` | MOM frequency_relationship + masking | PCA EQ 装载门 + 共享 EQ runtime | ✅ `builtins.go:34` + `frequencycleanup` 包 + `c1_frequency_cleanup_runtime.go` | ✅ 双侧 | **已落地** |
| C2 动态控制 | `fine_mix.dynamic_control.v1` | DOM/COM（source_dynamics+paired） | plugin_grabber compressor/de-esser/limiter/spectral/gate/transient inspect+apply（catalog.go:1026-1036） | ✅ `builtins.go:43` + `dynamiccontrol` 包 + c2 三 runtime | ✅ 双侧 | **已落地** |
| C3 空间与深度 | （无契约 ID） | FXM A/B 变换投影在位 | 通用插件装载/参数（instantiate_plugin 等）；**无 reverb/delay 语义控制面**（plugin_grabber 无 reverb/delay inspector） | 无 | 无 | **部分** |
| C4 段落自动化 | （无契约 ID） | 无 | **automation 读写 typed route 本身缺**（§6.3 第 4 项） | 无 | 无 | **缺（M2/M3 真开发）** |
| C5 总线处理 | （无契约 ID） | 无 | bus/send/FX return 结构化读写缺（§6.3 第 6 项）；内核 `rack_*` 命令族是机架图形/连接面，非总线处理 | 无 | 无 | **缺** |
| D1 响度峰值检查 | （无契约 ID） | mix.observe peak/rms/loudness_ranking/headroom + RLM `reference_level_model.projection.v0` | 只读 | 无 | 无 | **部分（观察）** |
| D2 翻译检查 | （无契约 ID） | stereo correlation/mono 风险观察 | 只读 | 无 | 无 | **部分（观察）** |
| D3 参考曲对比 | （无契约 ID） | RLM 投影（user intent + RenderBindings，RLM-PROFILE-2 fail-closed，`rlm/types.go`） | 无参考曲导入/对比命令 | 无 | 无 | **部分（观察）** |
| D4 版本输出检查 | （无契约 ID） | 无 | export variant/stem 管理缺（§6.3 第 7 项）；内核 `start_render` 未暴露 agent | 无 | 无 | **缺** |
| E1 母带 EQ | （无契约 ID） | — | 通用插件参数面可当底层 route | 无 | 无 | **缺（E 线实现后置，裁定 2）** |
| E2 母带动态响度 | （无契约 ID） | — | plugin_grabber limiter inspect/apply 是通用插件控制，无 loudness target/true-peak ceiling 能力 | 无 | 无 | **缺（后置）** |
| E3 母带空间 | （无契约 ID） | — | — | 无 | 无 | **缺（后置）** |
| E4 最终导出质检 | （无契约 ID） | — | export_readiness 仅存在于 mix_report 三态规则，依赖外部传入 FinalMeasurements；无导出能力 | 无 | 无 | **缺（后置）** |
| F1 混音审查 | （无契约 ID） | mix_report.v1 账本汇总（能力汇总/决策时间线/验证计数） | 只读 | 无 | 间接 | **部分** |

**族级读数**：B 族契约 v0 后 Session 形态落地 **3/4**（B2/B3/B4 全链落地，B1 无 Session 形态）。C 族 C1/C2 **不只观察，Session 已落地**；C3 仅观察（FXM+通用插件面），无 Session；C4/C5 全缺。A 族 5 节点=投影/命令面全备、零 Session；D 族观察侧有底子（RLM/rankings/stereo），E 族=通用插件面+无能力，F1=mix.report 雏形。

---

## 问 3：typed route §6.3 七项缺口核对

内核命令表=`VitApp/Source/Service/CommandDispatcher.cpp` handlers 全集（178 个注册名，含新旧别名）；agent catalog=`agent/internal/tools/catalog.go`。

| # | §6.3 缺口项 | 内核侧 | agent catalog 侧 | 判定 |
|---|---|---|---|---|
| 1 | 工程采样率切换与管理 | `project.get_audio_settings` / `project.set_audio_settings` / `project.validate_audio_settings_change` | 同名 spec（catalog.go:400-405） | **在位** |
| 2 | 单轨电平微调稳定 typed route | `set_volume`、`set_volume_batch`、`track.volume.set_batch`、`track.group.apply_control` | `track.volume`（:972）、`mix.propose_tick/apply_tick`（track_gain_adjust）、`track.group.apply_control`（:965） | **在位** |
| 3 | 声像与宽度稳定 typed route | `set_pan`、`set_pan_batch`、`track.pan.set_batch` | `track.pan`（:973）+ mix tick pan | 声像**在位**；**宽度缺**（无 width 命令，B3 v1 也不执行宽度） |
| 4 | 线性包络 / automation 写入与读取 | 178 命令中**无** automation/envelope 命令（内核源码 envelope 命中均为插件瞬态分析，非轨道自动化 lane） | 无（唯一 envelope 命中=plugin transient shaper 检测） | **缺**（=M3 自动化真开发项） |
| 5 | clip gain、fade、静音片段 | `clip.gain.set/set_batch/read` ✅、`clip.fade.set/read` ✅、strip silence ✅；**clip 级静音写无**（`set_mute` 是 track 级；clip mute 只在 `clip.gain.read` 回读中带出） | 同 | gain/fade **在位**；**clip 静音写半缺** |
| 6 | bus / send / FX return 结构化读写 | 仅 `folder_track.create` / `folder_track.set_routing_bus_enabled` / `set_folder_routing_bus`（folder→bus 路由开关）；**无 send、无 FX return 命令** | catalog 无 bus/send/return 命令 | **缺**（folder bus 路由除外） |
| 7 | reference track / export variant / stem 输出管理 | 内核有 `start_render` / `cancel_render` / `export_policy`（渲染面存在）；`project.snapshot_export` 是工程快照非音频导出；**无 stem/variant/reference 管理命令** | `start_render` / `l2_render_probe` / `audition.*` 十命令**均未暴露**给 agent catalog | **缺**（内核渲染面在但 agent typed route 未接；stem/variant/reference 管理本体缺） |

附注：`audition.*` 家族（prepare/inspect_candidate/select/apply_candidate 等 10 命令，人耳盲听 A/B 面）与 `l2_render_probe` 同样只在内核 dispatcher，不在 agent catalog——D3 人耳判定链路与第 7 项的暴露缺口在此交汇。

---

## 问 4：自由态 settle→黑板供血路径现状

**settle 报告是什么**：自由态实验轮终局的 LLM 决策 JSON（materiality + target_response + round_decision，形状锚点 `agentloop/free_state_reasoning.go:1236 freeStateSettleReportExample`），在 fresh post-action 证据记录后 booking（`chat/free_state_experiment_runtime.go:661` 拒收无新证据的 settle）。

**现在写到哪（四层落点，全部非工程键控）**：
1. `experiment.Turn` 控制器状态（`experiment/runtime.go:470`"persisted free-state experiment controller state"）——运行时保存位置是**会话内存 map**：`chat/free_state_reasoning_loop.go:646 storeFreeStateLoop → s.freeStateLoops[ConversationID]`，**无磁盘持久化**（agent 重启即失）；
2. 轨迹事件：`settleFreeStateExperiment`（`free_state_experiment_runtime.go:835`）→ `Experiment.Settle` → `emitFreeStateExperimentEvents`（:314）→ `emitTrajectoryEvent`（`chat/trajectory_events.go:12`，有 task contract 时 BindTaskState 绑任务态）；
3. D1 回执/journal：`free_state_d1_receipt.go:124`（Settled 字段）+ `free_state_d1_runtime.go:91 d1JournalMutationPort`；
4. 任务态事件：`chat/task_semantics.go:327 completeTaskExperimentOutcome → EventTaskSettled`（仅当 goal 带 task semantic contract）。

**与黑板/账本的桥接：零**。`chat/free_state*.go` 与 `agentloop/free_state*.go` 中 mixboard/decision_ledger 引用为零（grep 证实）。身份轴错位：settle 状态按 **conversation_id** 键控；账本/黑板按 **project_uuid** 键控。黑板报告（agentloop/project_blackboard_report.go）不读 experiment/settle 状态；mix.report 只读账本。

**距"黑板数据源"差五件事**（裁定 3"供血非替代"的落地缺口）：
1. **键控桥**：conversation→project_uuid 映射。锚点已在：`ensureFreeStateExperimentCheckpoint`（free_state_experiment_runtime.go:322）从 `loop.LatestProjectChange` 取 `checkpoint_ref/commit_id`——工程历史检查点锚已存在于自由态 loop 内，可直接作桥接键。
2. **持久化层**：settle 结论（含用户人耳判定 outcome）落工程级存储（账本 `decisions/` 同级或黑板专属目录），跨会话/跨天可读；当前 Turn 只活内存。
3. **结论单元投影**：把 settle 报告映射为黑板可读单元——与账本 impact 契约对齐（writes/recheck_on 维度）或作独立"自由态结论"黑板块；needs_review 联动沿用账本机制。
4. **黑板消费**：`project_blackboard_report.go` 增读该持久层（现在只读 project.state+会话内存，读不到任何跨会话状态）。
5. **mix.report 覆盖**：报告时间线纳入自由态结论（否则 F1 审查看不见实验线）。

---

## M1 内 A-F 工作量估算（供拆卡参考，不代决）

已落地不需 M1 动的：B2/B3/B4/C1/C2 五族 Session 全链 + 账本读写两侧 + mix.report 工具面。剩余编排类工作块：

| 工作块 | 内容 | 估量（flash/L1 级） |
|---|---|---|
| 黑板持久层+供血桥 | project_uuid 键控黑板（A-F 状态/known goals/风险/unresolved）+ settle 供血桥（Q4 五件事）+ blackboard report 改读持久层 | **1.5-2 天** |
| mix_workflow_queue.v0 | 节点队列+9 态状态机+agent 读写；若定义为黑板的一个投影视图则降为 0.5-1 天 | 1-2 天（视图化则 0.5-1 天） |
| B1 补账本/Session 接线 | B1 impact 契约 + 终态投影（或普通动作 receipt 聚合的轻量账本记录） | 0.5-1 天 |
| A 族契约接线（A1-A5） | 投影/命令面已备：每节点=CCB view 披露+黑板状态回写+非线性调用路由校验 | 1-1.5 天 |
| C3 观察侧收口 | FXM+通用插件面→C3 只读能力 Session（语义 reverb/delay 控制面随节点另补） | 0.5-1 天 |
| D1-D3 观察能力化 | RLM/rankings/stereo→只读能力 pack | 0.5-1 天 |
| typed route 小缺口 | 宽度、clip 静音写、bus-send 结构化读写、导出面暴露——随节点补最小（裁定 3 注脚①） | 各 0.25-0.5 天，合计 1-2 天 |
| F1/queue/黑板三视图对齐 | mix.report 纳入自由态结论与节点队列视图 | 0.5 天 |

**合计（M1 内 A-F 编排类剩余）≈ 6-10 天**——不含 M2 时域、M3 自动化两个真开发项（C4/automation 与 C5 的 route 本体在 M3/M2 范围，不在上表重复计价）。拆卡权在决策侧。

---

## 勘察边界声明

- 全程只读：未运行任何写状态命令；`go test`/构建均未执行（勘察卡无需，且避免在共享工作树落产物）。
- 锚点均为源码行号或 grep 全仓判定（"零命中"类结论以 `grep -rln` 全仓扫描为据，含 `agent/`、`VitApp/Source/`）。
- 未核对 webui 渲染面与 Godot 前端对黑板/queue 的展示面（卡内未要求；黑板报告目前是文本回复形态）。
- `docs/PROJECT_AWARE_CAPABILITY_ORCHESTRATION_ARCHITECTURE_V1.md`（master_plan 规范更新指向的 authority 文档）未逐条核对——账本/Session 侧实现与 `MIXBOARD_DECISION_LEDGER_V1.md` 烟测合同逐条对上了，若决策侧需要再补。
