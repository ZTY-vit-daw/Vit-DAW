# Observation Projection Manifest (v1)

状态: 已审计 (Graph Context v1 Step 0 产出)
日期: 2026-08-13
权威来源: `docs/CONTEXT_GRAPH_V1_DESIGN_DRAFT.md` §2.2, `agent/internal/capabilitycontext/free_state_observation.go`, 各投影包源码

本清单是拓扑审计的 ground truth。实现与文档一律以本清单为准: **所有投影是 DAD 之上的同级 peer, 各自从 DAD 事实派生自己的 projection, 禁止把 peer 写成链。**

## 1. 权威拓扑

```
DAD Evidence Layer ──> DOM / MOM / TIM / TOM / FXM / COM / EPM / RLM  (peer projections, 各自独立)
peer projections ──> CCB (view catalog + bounded disclosure, 按显式 view 请求披露)
CCB ──> Context Graph (hot/warm/cold) ──> LLM
```

- 不存在 `DAD -> DOM -> MOM -> CCB` 管线;
- 每个投影自带 `LLMContext`, 多数 `do_not_include_raw_package=true`; Context Runtime 优先消费投影的 `LLMContext`, 不得展开 raw package;
- CCB 不做自动 view 选择; 缺失/partial/stale/suspect 原样保留, 不升级 readiness。

## 2. 投影清单 (Ground Truth)

| 投影 | Schema | LLMContext | Readiness 归属 | CCB v1 view | 显式 peer 交叉引用 |
|---|---|---|---|---|---|
| DOM | `dom.projection.v1` | ✅ | DOM 各 dimension 独立判定 | `track.peak_structure`, `track.activity_structure`, `track.frequency_time_events`, `track.transient_structure`, `track.band_dynamics` | 无 |
| MOM | `mom.frequency_relationship.v1`, `mom.static_level_relationship.v1` | ✅ | MOM 自己按 tap/scope/revision 判定 | `mix.multitrack_relationship`, `mix.frequency_relationship`, `project.structure`(嵌套 project_structure) | 引用 track observation 的 evidence refs, 不依赖 DOM 输出 |
| TIM | `tim.projection.v0` | ✅ | TIM 自己 | `project.structure`(optional supplement) | 无 |
| TOM | `tom.projection.v0_2` | ✅ | TOM 自己 | 无 (CCB v1 目录不含轨道组织 view) | `BuildLLMContext` 显式可选接收 TIMProjection → 登记保留 (见 §4) |
| FXM | `fxm.projection.v0` | ✅ | FXM 自己 | 无 (CCB v1 目录不含 A/B 变换 view) | 无 |
| COM | `com.projection.v1`, `com.source_dynamics.v1` | ✅ | COM 自己 | `track.time_dynamics`, `processor.identity_and_controls`, `processor.behavior`, `processor.change_delta` | 无 |
| EPM | `epm.projection.v0` | ✅ | EPM 自己 | 无 (CCB v1 目录不含执行投影 view) | ImportInput 可接收 TIM/TOM, 但**不进入 LLMContext**, 不构成依赖 |
| RLM | `reference_level_model.projection.v0` | ❌ 缺口 (v1 未实现, 见 §5) | RLM 自己 | 无 (CCB v1 目录不含参考电平 view) | 无 |

## 3. CCB v1 视图 → 投影映射 (源码核对)

来源: `capabilitycontext.freeStateViewDefinitions`。每视图至多一个权威投影, 其余为 DAD 派生事实或显式 optional supplement。

| CCB view | 权威投影 | 事实 keys | Optional supplement |
|---|---|---|---|
| `project.structure` | MOM (project_structure) + project state | project.static.summary, project.tracks.summary, observation.mom_projection | observation.tim_projection ✅(缺失不降级) |
| `track.basic_energy` | DAD 派生 | track.{id}.static.identity, track.{id}.fast.levels | — |
| `track.time_dynamics` | COM (source_dynamics) | track.{id}.slow.time_energy.summary, observation.com_projection | — |
| `track.timbre_frequency` | DAD 派生 | track.{id}.slow.band_energy.summary | — |
| `track.peak_structure` / `activity_structure` / `frequency_time_events` / `transient_structure` / `band_dynamics` | DOM | observation.dom_projection (每 dimension 独立 status) | — |
| `track.stereo_space` | DAD 派生 | track.{id}.slow.stereo.summary | — |
| `mix.multitrack_relationship` | MOM (multitrack_relation) | observation.mom_projection, project.relationship_inputs | project.rankings.*, project.risks.headroom, project.attention.first ✅ |
| `mix.frequency_relationship` | MOM (frequency_relationship) | observation.mom_projection, project.frequency_relationship_inputs | — |
| `mix.masking_relationship` | — (v1 deferred/unavailable) | — | — |
| `processor.identity_and_controls` | COM (processor_scope) | observation.com_projection | — |
| `processor.behavior` | COM (paired_io) | observation.com_projection | — |
| `processor.change_delta` | COM (change_delta) | observation.com_projection | — |
| `comparison.before_after` | MixBoard 历史 | observation.before_after.latest | — |

审计确认: 不存在把投影 A 的输出作为投影 B 的判定输入的链。`observation.tim_projection` / `project.rankings.*` 等在 `optionalSupplementalProjection` 中显式登记为可选补充, 缺失时 view 不降级。

## 4. TOM/TIM 交叉引用审计结论 (草案 §11 开放问题 4)

**结论: 保留, 登记为显式可选 peer 引用, 不构成链依赖。**

依据:
1. `tom.BuildLLMContext(proj, timProjection ...map[string]any)` 为可变参可选参数; 未提供时不影响 TOM readiness 与 LLMContext;
2. 提供时仅压缩为**一条** `tim_projection` digest fact (status + technical_summary 计数 + risk_summary 计数), 不含 TIM raw package;
3. TOM readiness 由 TOM 自己的输入 (ID/命名/长度/mono-stereo/format/轻量波形特征) 独立判定; TIM fact 是补充上下文;
4. TOM EvidenceRefs 含 `tim:projection` — 属 evidence ref 引用模式, 与 MOM 引用 track observation evidence refs 同构;
5. 移除会破坏 `tom/projection_test.go` 对 compact TIM fact 的既有契约断言。

同类确认: `epm.ImportInput` 的 `TIMProjection`/`TOMProjection` 仅是 import 输入, **不进入 EPM LLMContext**, 无投影间上下文依赖。

## 5. 已知缺口 (登记, v1 不修)

- **RLM 无 `LLMContext` 字段**: `rlm.Projection` 未实现 LLMContext。v1 范围不改 (不属于 CCB v1 视图, 不阻塞预算治理); 后续版本为 RLM 补 LLMContext 时按本清单登记。
- **CCB v1 目录不含 TOM/FXM/EPM/RLM view**: 这些投影存在且可用, 但 free-state CCB 目录当前未披露; 新增 view 时按 §3 表格登记权威投影, 禁止引入链。

## 6. 链式残留审计结果 (草案 §10 第 3 条)

| 位置 | 结论 |
|---|---|
| `agent/**/*_test.go` | 无链式残留 |
| smoke telemetry (product_path_20260813_202548 等) | 无链式残留 |
| `docs/OBSERVATION_V1_ACCEPTANCE.md` §1 数据流描述 | 措辞待修正: 把 `DAD evidence layer -> Observation Model Layer` 标注为 peer 关系 |
| `docs/agent_action_workflow_v1_master_plan.md` (TOM 聚类描述) | 措辞待修正: 标注 TIM 为显式引用而非依赖 |
| 其余 docs | `DOM_V1_CONTRACT.md`, `FREE_STATE_CCB_OBSERVATION_PROTOCOL_V1.md` 已是 peer 表述, 无需修改 |

## 7. 约束边界 (v1)

- 不修改 DAD 原始事实、CCB 事实权威、PCA/typed controller、冻结素材/配方/目标轨道/预期 family/阈值;
- 不修改 Semantic Entry、Prompt 注入策略、A-F 路由;
- 不新建观察模型层; 不引入 RGBA/ViT 图像观察与外部图数据库;
- 不启动正式长时间 Agent 烟测; 验证仅用 Go 单测、Python contract/preflight、短时 runtime preflight。

## 8. 相关文件

- `docs/CONTEXT_GRAPH_V1_DESIGN_DRAFT.md` (§2.2 投影清单为本清单的来源)
- `docs/DOM_V1_CONTRACT.md`, `docs/MOM_FREQUENCY_RELATIONSHIP_V1.md`, `docs/COM_V1_AUDIT_AND_GAP_MATRIX.md`
- `agent/internal/capabilitycontext/free_state_observation.go` (CCB 视图目录与装配)
- `agent/internal/contextruntime/model_projection.go` (模型投影提取)
- 投影包: `agent/internal/{dom,mom,tim,tom,fxm,com,epm,rlm}`

## 9. 观察缺口登记 (v1 观察缺口分支, 2026-08-13)

修复观察缺口不新增投影模型层; 以下登记每个缺口的根因分类与修复状态。

| 缺口 | 根因分类 | 修复状态 |
|---|---|---|
| mix.frequency_relationship partial/suspect | 适配/数据层: tap 不统一 (bass/drums 只有 L2 track_post_fader, 其余 L3 source_file_pre_fx) | ✅ 已修: 通用频率观察统一 L3 (`preferUniformProjectL3FrequencyEvidence` 部分覆盖也统一), L2 render probe 不混入, 缺 L3 轨显式 missing; 采集侧 background fill 已有补 L3 机制 |
| mix.masking_relationship unavailable | DAD/内核能力缺失: masking_analysis=deferred phase 5, 投影/CCB 存在但无数据可装配 | ✅ 披露强化: CCB view limitation 明确为数据层缺口 (非投影缺口); 能力保持 deferred |
| track.time_dynamics partial (micro transient) | 投影层数据映射: L3 已生成 DOM transient_events, 但 COM source_only 未消费 | ✅ 已修: COM SourceEvidence 增加 transient 通道 (L3 帧级), comSourceInput 装配, buildSourceDynamics 生成 micro transient 统计并标 partial (如实声明 FFT 帧级 window/hop) |
| 时间频率持久性 unavailable | DAD 缺数据: L3 未生成 persistence_ratio/active_frame_ratio, MOM 字段已预留 | ✅ 已修: L3 band 行生成两字段 (噪声底+3dB 阈值, active/总帧比), MOM 消费端已就绪零改动 |
| 跨轨 conflict candidates withheld | 适配/数据层可比性: mixed tap 时 measurement conditions 不可比 | ✅ 已修 (烟测实证): (a) CCB 观察路径 (buildObservation) 也统一 L3 tap; (b) MOM 跨轨可比性排除素材 revision (source/clip/render), 只要求测量条件统一 (tap/时间窗/采样格式/analyzer); (c) buildBandEnergySummary 透传 sample_rate/channel_count/analyzer_version/window_ms/hop_ms/analyzed_range 等测量条件字段; 诊断烟测中 mix.frequency_relationship 从 suspect → ready, conflict candidates 恢复生成 |
| 技术元数据 partial/missing | DAD/项目状态缺失暴露: 上游未送 sample rate/bit depth/channel count/playback validity | ✅ 已修: L3 输出 bit_depth (reader->bitsPerSample) 补齐 acoustic 层 fallback; TIM 三层读取保持, 缺失时 limitation 保留 (不伪造) |
