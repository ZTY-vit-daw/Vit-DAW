# TIM 结构断言器 v1 设计（音频编译器第一层承载者）

> **状态：规划（未实现）**。本文档是 L2-1-TIM-DESIGN-1 的产出：路线图 [D8 编译器第一层](AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md)「结构合法性断言」的 v1 设计——断言清单、五要素定义、实现锚点与测试计划。实现由后续卡（L2-1-TIM-1）执行；本文档不代表当前系统现状。
>
> 版本口径：设计对象是现行 TIM 投影。**事实更正：TIM 现行 schema 是 `tim.projection.v0`**（`agent/internal/tim/types.go:4-5`）；任务卡所写 `tim.projection.v0_2` 是 TOM 的版本号（`agent/internal/tom/types.go:5`）。本文全部按 v0 表述。

## 0. 定位与硬原则

- **编译级、确定性、不用听**（D8 第一层）：每个断言器是纯谓词，同输入同输出，不依赖 LLM、不依赖听感。
- **裁判不下场（硬规则，写入实现）**：断言器只做红绿灯——fail 报警、pass 记录、数据缺失报 not_evaluable；**不做任何修正动作**。修正一律走 agent 闭环（诊断→调整→复检）。实现上断言器无写路径：输入只读，输出只有 Issue/Limitation/日志行。
- **v1 失败动作全部 warn**（不 block）。block 语义留给 v2+ 交付门，且必须单独决策。
- **三态而非两态**：`pass / fail / not_evaluable`。数据缺失 = not_evaluable（登 Limitations 码），**绝不静默记 pass**——与 AGENTS §7 证据纪律和 CCB「缺失/partial 原样保留不升级」原则同构。
- **数据可得性优先**：v1 只收编现有数据能支撑的断言；需内核新增输出的一律进 §5 缺口清单（标注「内核腿」），留给决策侧排卡。**v1 零内核改动，全部断言在 agent 侧判定**（mac 可独立完成实现卡）。

## 1. TIM 现状盘点（锚点）

### 1.1 结构（`agent/internal/tim/`，全包 1404 行）

| 部件 | 锚点 | 内容 |
|---|---|---|
| schema 常量 | `types.go:4-5` | `Version="v0"`、`SchemaVersion="tim.projection.v0"` |
| 输入 | `types.go:8-20` | `Input{ProjectPackage, AcousticPackageStatus, SourceCapabilities, AuthoritativeState}`；AuthoritativeState 内部只读、不进模型投影 |
| 投影主体 | `types.go:22-37` | `Projection{Status, TechnicalSummary, Coverage(7 项), RiskSummary, Issues[], TrackFacts[], EvidenceRefs, Limitations, LLMContext}` |
| TrackFact | `types.go:98-118` | 含 `PeakDBFS/RMSDBFS/HeadroomDB *float64`（来自 acoustic 行）、`SampleRateHz`、`AcousticStatus` |
| 构建入口 | `projection.go:28` (`Build`) | 从 `ProjectPackage["tracks"]` 逐轨生成 TrackFact+Issues |
| 权威态调和 | `projection.go:96-161` | `reconcileAuthoritativeSourceState`——compact 行缺失不升级为 missing；合并键表 `:151`（status/rms/peak/headroom/crest/sample_rate/channel/bit_depth/duration，**不含 nan_count/inf_count**） |
| 现有 issue 码 | `projection.go:365-408` | `empty_track / source_path_missing / playback_source_invalid / abnormally_short_clip / compressed_source_format / acoustic_package_missing_or_partial / possible_clipping_or_no_headroom / possible_silence` |
| 状态机 | `projection.go:13-18, 598-611` | status: ready/partial/missing/suspect/not_applicable；severity: error/warning/notice |
| LLMContext | `projection.go:645-696` | CompactFacts 四层（summary/coverage/risk/issue_excerpt），`do_not_include_raw_package=true` |
| 导入种子 | `import.go:19-89` | `BuildFromImportRows`（注意：`ImportInput.AudioSettings` 字段存在但**当前未使用**——恰好是 v1 采样率断言的现成输入位） |

### 1.2 数据流与装配时机

**无轮询/无事件刷新 TIM 本体；TIM 在每次 observation 物化时同步重建**：

```
内核 get_project_state（VSP 别名 project.snapshot.get，VspKernelReference.cpp:86）
  │  ZMQ REQ/REP 5555（kernel/client.go:39-55）
  ▼
shadow 刷新 refreshShadowWithStatus（harness.go:12456；优先 VSP 快照 :12481，回退 get_project_state :12503）
  │  shadow 保留每轨 rack（shadow/shadow.go:678）与 audio_settings
  ▼
mixboard.Store.RequestObservation（mixboard.go:1071）→ FinalizeObservationContext（catalog.go:59）
  → finalizeTIMProjection（mixboard.go:3960-3966）→ tim.Build
  输入装配 timInputFromObservation（mixboard.go:4017-4032）
  权威态 authoritativeTIMState（mixboard.go:4038-4105，只取 source 状态键+DAD 计数）
  ▼
ObservationPacket.TIMProjection（mixboard.go:101）
```

- DAD 声学事实走 ZMQ SUB 事件 `audio_feature_data_ready`（harness.go:5329-5370）+ projectstore 持久化快照。
- 导入路径：`BuildFromImportRows` 即时种子（harness.go:11834-11841）→ DAD 就绪后择优替换（message_loop.go:7815/7830）。
- 触发命令 `mix_request_observation`：harness.go:2424-2425；chat mix 轮 mix_session_workflow.go:3010；CCB 观察 ccb_observation.go:65。

**内核侧数据源要点**（`get_project_state`，CommandDispatcher.cpp:3268-3401，注册 :2491）：

- clips 行：`file_path/current_source_path/playback_source_valid`（:966-970，构造 :921-978）。
- 每轨 `rack`：nodes（`node_id/plugin_id/zone_id/enabled/audio_reachable_from_rack_input/vit_orphan_bypass_candidate/plugin_path/plugin_format`，:1820-1857）+ edges（`source_id/dest_id/...`，含 RACK_INPUT/OUTPUT 哨兵，:1868-1885），经 :3314 挂出；可达性/孤儿由内核预标注（VitClipRouteRegistry.cpp:477-515）。
- `audio_settings`（:3377-3382）与 `project_health` 问题码（:3397-3398，VitProjectHealthCheck.cpp:44-94：missing_asset/warp_unaligned/pending_job/invalid_connector_profile）。
- L3 声学证据：`nan_count/inf_count/max_abs/sum_abs/nonzero_count/coverage_ratio`（L3AcousticAnalyzer.cpp:441-455，公共戳 :509-531）；响度 `peak_dbfs/rms_dbfs/approximate_lufs`（:697-722，**近似 LUFS 无 BS.1770、无 true peak**）。**nan/inf 键已在 agent 观察装配透传键表内**（mixboard.go:2893），COM/FXM 已消费同源（com_projection.go:151、fxm/types.go:83）——只是 TIM 未读。

### 1.3 消费方

| 消费方 | 锚点 | 形态 |
|---|---|---|
| mix_read 键 `observation.tim_projection` | catalog.go:247, 485-489 | 缺失返回 `{status:missing}` |
| ContextPack | mixboard.go:4766-4768 | `tim_projection` + `technical_integrity_context`（LLMContext） |
| CCB `project.structure` view | free_state_observation.go:427, 591-593, 703-705 | TIM 为**可选补充投影**；裁剪为 8 字段子集（**不含 llm_context、不含 issues、不含 assertions 位**）；缺失不降级视图 |
| contextruntime 模型投影 | model_projection.go:1265-1266 | `project.structure` 分支：status/technical_summary/risk_summary/coverage |
| TOM（peer 引用） | tom/projection.go:565-571, 1005 | 仅 status/risk digest + limitation 码 |
| EPM | epm/projection.go:89 | 仅 import 输入，不进 LLMContext |
| 用户面报告行 | message_loop.go:8497, 8421-8476, 7504 | 「A2 TIM 技术完整性检查…」；注意 `terminal_report_wording.go:62` 黑名单剥 `TIM` 字样——**用户面文案必须用「技术完整性/结构检查」措辞** |

### 1.4 现有 WARN/监督报警路径（v1 挂接点）

1. **agent 日志**：`logx.Warn` → `[WARN]` 行（logx/logger.go:43-45, 56），恒回显 stdout（:68-70），落 `VitApp/Workspace/Logs/agent_last.log`。惯用 `[域] key=value` 前缀（如 `[continuation.stall]`）。
2. **投影内**：TIM `Issues{severity:warning}` + `RiskSummary{by_severity, primary_codes}` + Limitations 码——LLM 经 `project.structure` view（risk_summary 汇总可见）与 mix_read（全量可见）消费，**零新增管道**。
3. **内核侧范式**（v1 不用但记录）：渲染看门狗 `WARN [render_watchdog]` + publishMessage 事件（VitProductionCoordinator.cpp:1007-1022）→ ZmqLogger（ZmqLogger.cpp:38-49）→ monitor.py SUB 5557 → system_journal.jsonl。
4. **烟测参照**：cont_stall_repro_smoke.ps1:440（agent 日志前缀过滤断言）、d1_stall_repro_smoke.ps1:99-110（日志基线行数差分）、journey1 plugin_instance_ready 断言（journey1_demo_journey_smoke_mac.sh:1208）。

## 2. 断言清单 v1（5 个，范围钉死）

每个断言器五要素：谓词（精确到可写测试）/ 作用域 / 数据源 / 判定位置 / 失败动作。命名：asserter=`<域>`，check=`<谓词名>`，fail 码 `assert_<check>`，not_evaluable 码 `assert_<check>_not_evaluable`（Limitations）。

### AS-SIG 信号卫生断言器（asserter=`signal_hygiene`）

| 要素 | 定义 |
|---|---|
| 谓词 | **P1 nonfinite**：`acoustic.nan_count + acoustic.inf_count == 0`（每轨；两键均缺 → not_evaluable）。**P2 clipping_headroom**：`peak_dbfs < -0.1` 且（headroom_db 缺或 `> 0.1`）；两值均缺 → not_evaluable。P2 谓词与现有 issue `possible_clipping_or_no_headroom`（projection.go:398-403）同语义——v1 **沿用现有码**（不双码一个问题），断言器只是把该码纳入管辖并在 assertions 块登记 |
| 作用域 | 轨 |
| 数据源 | L3 声学证据：nan/inf（L3AcousticAnalyzer.cpp:450-452；透传键表 mixboard.go:2893；TIM 未读——**agent 侧腿：tim 读键**）；peak/headroom（TIM 已读 projection.go:325-327）。无内核腿 |
| 判定位置 | agent 侧（tim.Build 内） |
| 失败动作 | warn（fail 码：P1 `assert_signal_nonfinite` 新增；P2 沿用 `possible_clipping_or_no_headroom`） |

### AS-PEAK 电平上限断言器（asserter=`level_ceiling`，true peak v1 单边版）

| 要素 | 定义 |
|---|---|
| 谓词 | 每轨：`peak_dbfs ≤ ceiling_dbfs`（ceiling 默认 `-1.0`；RLM profile 就位后取 `true_peak_max`，无 profile 用默认）。缺 peak_dbfs → not_evaluable。**单边语义明示**：sample peak > ceiling ⇒ 真峰值必超（true peak ≥ sample peak），fail 判定有效；sample peak ≤ ceiling **不保证** true peak 合规——Limitations 常驻 `assert_level_ceiling_sample_peak_only` |
| 作用域 | 轨（v1）；v2 扩总线/render 产物 |
| 数据源 | acoustic `peak_dbfs`（现成）+ ceiling（v1 常量默认；L2-1-RLM-1 profile 产出后接入——**无依赖阻塞**） |
| 判定位置 | agent 侧 |
| 失败动作 | warn（`assert_level_ceiling_exceeded`） |
| 内核腿 | true peak 需 4x 过采样，内核现无（§5-1） |

### AS-SR 采样率一致性断言器（asserter=`sample_rate`）

| 要素 | 定义 |
|---|---|
| 谓词 | 每轨：`sample_rate_hz == project_settings.sample_rate_hz`（轨值缺失 → not_evaluable；工程 settings 缺失 → 整断言器 not_evaluable）。语义：源采样率≠工程采样率=内核实时转换，报告质量/负载风险，不禁转换 |
| 作用域 | 轨 → 全局汇总 |
| 数据源 | 轨 `sample_rate_hz`（TIM 已读）；工程 `audio_settings.sample_rate_hz`（get_project_state 附带 CommandDispatcher.cpp:3377-3382；shadow 已有——**agent 侧腿：timInputFromObservation 带 audio_settings**，`ImportInput.AudioSettings` 字段已预留未用）。块长一致性延后（§5-3） |
| 判定位置 | agent 侧 |
| 失败动作 | warn（`assert_sample_rate_mismatch`） |

### AS-ROUTE 路由完整性断言器（asserter=`routing`）

| 要素 | 定义 |
|---|---|
| 谓词 | **P1 dead_end**：每轨 rack 内 `enabled && !audio_reachable_from_rack_input` 的节点数 == 0（启用但信号不可达=死路；内核预标注，agent 只汇总）。**P2 cycle**：全轨 rack edges 并集有向图无环（agent 侧 DFS；内核写入时已有 VitDagChecker.cpp:9-51 + VitGraphValidator.cpp:58-99 防环，运行态出现环=状态腐坏——正是编译级断言价值）。rack 数据缺 → not_evaluable |
| 作用域 | 轨 rack（P1）/ 全局图（P2） |
| 数据源 | get_project_state 每轨 rack nodes/edges（createRackState CommandDispatcher.cpp:1793-1897；shadow.go:678 保留；timInputFromObservation 未消费——**agent 侧腿：TIM 输入装配扩展路由摘要**）。轨间 send/aux 拓扑无数据，v1 不做（§5-8） |
| 判定位置 | agent 侧 |
| 失败动作 | warn（`assert_routing_dead_end` / `assert_routing_cycle`） |

### AS-PLUGIN 插件结构合法性断言器（asserter=`plugin_legality`）

| 要素 | 定义 |
|---|---|
| 谓词 | 每 rack 插件节点：`plugin_path ∈ 当前插件表`（knownPluginList/pluginsemantics 索引）。plugin_path 空 → not_evaluable；不在表 → fail（插件已卸载/移动/未扫描——装载必败或静默失效） |
| 作用域 | 轨 rack / 全局 |
| 数据源 | rack 节点 `plugin_path`（CommandDispatcher.cpp:1841-1857）；插件表两源候选：内核 `list_plugins`（PluginRackControlService.cpp:2685-2711）或 **agent 侧 pluginsemantics 索引（index.go:26-42，倾向——agent 现成、免新命令）**，实现卡定夺。per-instance 运行装载态（`plugin_instance_ready/plugin_load_state`）现仅 rack_add_node 回执（:3155-3172）不出 get_project_state——**内核腿**（§5-4），v1 不做 |
| 判定位置 | agent 侧 |
| 失败动作 | warn（`assert_plugin_unknown_path`） |

**停止条件核对**：五个断言器数据可得性全部核证（§1.2 与上表），无「全部存疑」情形——设计成立，无需走盘点上交分支。

## 3. 实现锚点

### 3.1 落点文件（实现卡文件域）

| 改动 | 文件 | 内容 |
|---|---|---|
| 断言器主体（新） | `agent/internal/tim/asserter.go` | 纯函数 `Evaluate(input AssertInput) []AssertionResult`；`AssertInput{TrackFacts, ProjectSampleRateHz *float64, RackSummaries []RackSummary, KnownPluginPaths map[string]bool, CeilingDBFS float64}`；无 I/O、无写路径（裁判不下场） |
| 谓词与码 | 同上 | 阈值集中一个 `Defaults` 常量块（clipCeiling=-0.1 / headroomFloor=0.1 / defaultCeiling=-1.0 / WARN 行 cap=20） |
| 挂接 | `agent/internal/tim/projection.go` Build 尾部 | `proj.Assertions = Evaluate(...)` 在 `BuildLLMContext` 之前；同步生成 Issue（severity=warning）与 Limitations（not_evaluable 码）；LLMContext 的 technical_risk fact 增补断言计数 |
| 输入装配 | `agent/internal/mixboard/mixboard.go` `timInputFromObservation`(:4017) | 带 audio_settings（采样率）+ rack 摘要（nodes/edges 精简）+ 插件表句柄；`authoritativeTIMState`(:4038) 不动 |
| schema | `agent/internal/tim/types.go` | `Projection.Assertions []AssertionResult json:"assertions,omitempty"`；`AssertionResult{Asserter, Check, Status(pass/fail/not_evaluable), TrackID, Value, Threshold, Code, EvidenceRefs}` |
| WARN 行 | `agent/internal/tim/asserter.go` | `logx.Warn("[tim.assert] asserter=%s check=%s status=%s track=%s value=%v threshold=%v")`，每 fail 一行、每断言器 cap 20 行 |

### 3.2 schema 兼容（AGENTS §11 纪律）

- 旧记录（无 `assertions` 字段）加载：字段 absent = **断言未运行**，不是 pass；不炸、不改变现有字段语义。
- 未知 asserter/check 名：fail-closed——登 limitation `assert_unknown_entity_skipped`，不评估、不吞错。
- 现有 issue 码零改动（TOM digest/CCB 契约不断裂）；断言 fail 新码只增不改。
- 必测：一份真实旧 observation 工件反序列化往返（红测试清单 T7）。

### 3.3 消费面（v1 零 CCB 改动）

- `Assertions` 随 Projection 进 mix_read（全量）、digest（计数）、ContextPack（裁剪版）。
- CCB `project.structure` 的 8 字段裁剪（free_state_observation.go:703-705）**不含 assertions/issues**：LLM 经 `risk_summary.by_severity/primary_codes` 感知 fail 汇总（断言 fail 计入 warning 计数与 primary_codes），明细走 mix_read。改裁剪面为后续评估项（登记 §6），v1 不动 `free_state_observation.go`。
- 用户面：message_loop TIMLines 模式追加一行断言汇总；文案过 `terminal_report_wording.go:62` 黑名单（**不得出现 TIM 字样**，用「结构完整性检查」）。
- 监督烟测观测点：agent_last.log 的 `[tim.assert]` WARN 行（§4 烟测草案）。

## 4. 测试计划

### 4.1 红测试（修前应红；表驱动，风格照 projection_test.go）

| # | 断言器 | 最小构造（修前无 → 红） |
|---|---|---|
| T1 | AS-SIG P1 | track acoustic `{status:ready, nan_count:2}` → 期望 issue 码 `assert_signal_nonfinite` + assertions 登记 |
| T2 | AS-SIG P2 | acoustic `{peak_dbfs:-0.05}` → 现有码已有；期望 assertions 出现 `signal_hygiene/clipping_headroom=fail`（修前无 assertions 块） |
| T3 | AS-PEAK | acoustic `{peak_dbfs:-0.5}` + ceiling -1.0 → `assert_level_ceiling_exceeded` |
| T4 | AS-SR | track `sample_rate_hz:44100` + settings 48000 → `assert_sample_rate_mismatch`；settings 缺 → limitation `assert_sample_rate_not_evaluable` |
| T5 | AS-ROUTE P1 | rack node `{enabled:true, audio_reachable_from_rack_input:false}` → `assert_routing_dead_end` |
| T6 | AS-ROUTE P2 | edges A→B→A → `assert_routing_cycle`（DFS） |
| T7 | AS-PLUGIN | rack node `plugin_path` 不在表 → `assert_plugin_unknown_path` |
| T8 | 三态诚实 | 无 acoustic 证据轨 → `assert_*_not_evaluable` limitation，**不产生 pass** |
| T9 | 兼容 | 旧 observation JSON（无 assertions 字段）Unmarshal 不炸、Assertions==nil；往返一致；未知 asserter 名 → fail-closed limitation |

### 4.2 端侧烟测草案（监督报警路径；实现卡按 AGENTS §5 真栈执行）

脚本模式：扩展现有 journey/smoke 体系（mac 版参照 `journey1_demo_journey_smoke_mac.sh`；日志断言参照 `cont_stall_repro_smoke.ps1:440` 前缀过滤 + `d1_stall_repro_smoke.ps1:99-110` 基线差分）。

1. 真实三件套上，对样例工程触发 `mix_request_observation`；
2. 断言 mix_read `observation.tim_projection` JSON：`assertions[]` 存在，含预期 fail 码（fixture 构造一个触发项，如近满幅轨）或全 pass+not_evaluable 清单；
3. agent_last.log 差分断言 `[tim.assert]` WARN 行出现（谁消费：LLM=risk_summary、烟测=日志、用户=报告行）；
4. **exit 0 = PASS**。

## 5. 数据缺口清单（内核腿，v1 不做，供决策侧排卡）

| # | 缺口 | 现状锚点 | 服务于 |
|---|---|---|---|
| 1 | true peak（4x 过采样） | 无过采样；仅 sample peak（L3AcousticAnalyzer.cpp:697-722） | AS-PEAK v2 |
| 2 | DC offset（带符号均值） | 仅 `sum_abs` 无符号，无法判（L3AcousticAnalyzer.cpp:441-455） | AS-SIG v2 |
| 3 | 块长一致性入 project state | `current_buffer_size` 在另一命令（TransportAudioService.cpp:719） | AS-SR v2 |
| 4 | per-instance 插件装载态出内核 | `describeExternalPluginLoadState` 只进日志（PluginRackControlService.cpp:1335-1352）；回执仅命令时（:3155-3172） | AS-PLUGIN v2 |
| 5 | PluginListHygiene Report 回传 | 只写一行日志（PluginListHygiene.cpp:64-68） | 卫生监督 |
| 6 | BS.1770 LUFS | 近似 `rmsDb-0.691`，已标 approximate（L3AcousticAnalyzer.cpp:713-716） | 响度断言（D9 协同） |
| 7 | 实时削波持久化 | meters `getAndClear` 语义不落盘（VitHeadlessService.cpp:991） | 播放中监督 |
| 8 | 轨间 send/aux 拓扑 | `aux_send` 全库零命中 | AS-ROUTE v2（意外反馈环全量版） |

## 6. v1 非目标与后续

- 不做 block、不做自动修正（裁判不下场；block 语义 v2+ 单独决策）。
- 不改 CCB `project.structure` 裁剪面（明细披露路径后续评估——若 LLM 需在 view 内直接见断言明细，需扩 free_state_observation.go 裁剪键表，走独立卡）。
- 不做总线级/render 产物级断言（v1 数据面=轨级；render 后监督与 FXM 差分回归=D8 第二层范畴）。
- 不动内核（v1 全部 agent 侧；内核腿见 §5）。
- 实现卡（L2-1-TIM-1）预估：asserter.go + 挂接 + 输入装配扩展 + 红绿测试 T1-T9 + 烟测一腿，文件域=`agent/internal/tim/*` + `agent/internal/mixboard/mixboard.go`（timInputFromObservation 一处）+ 烟测脚本。
