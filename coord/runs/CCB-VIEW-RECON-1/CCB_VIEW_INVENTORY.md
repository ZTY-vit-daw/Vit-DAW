# CCB-VIEW-RECON-1：CCB view 目录与参数化缺口勘察报告

- 执行侧：Mac 夜间托管会话（纯只读勘察，零代码改动）
- 领取时 origin/main：`a5196303f6c756f7baec8eb66a48582c73dc5fec`（领取提交 `2a7ad99`）
- 领取时 `git status --short`：`M VitApp/Workspace/Settings.xml`、`M VitApp/Workspace/default_project.xml`、`?? .zcodeignore`、`?? VitApp/Workspace/Artifacts/`、`?? coord/runs/FIX-PCA-AUTOSWEEP-1/20260925_mac/`——均为领取前已有运行时状态/工件，未触碰
- 交叉引用：`coord/runs/L1-1-RECON-1/EVIDENCE_REFS_INVENTORY.md`（下称 L1-1 报告）
- 日期：2026-09-27；行号以本卡工作 HEAD 为准

---

## 0. 执行摘要

1. **view 目录全集 = 18 个语义 view**，单一定义点 `agent/internal/capabilitycontext/free_state_observation.go:421-469`（`freeStateViewDefinitions`），分五个域：project（2）/ track（9）/ mix（3）/ processor（3）/ comparison（1）。
2. **披露经过两层独立裁剪**：第一层在 capabilitycontext 装配（每 view 绑定数据键族 + 逐 view 字段白名单 + sanitize），第二层在 contextruntime 模型投影（hot/warm/cold 分层预算 + 每 view digest 化 + 禁键过滤）。两层都对"参数化"封闭——所有裁剪规则都是编译期硬编码。
3. **参数化基线结论：view 选择面是参数化的（view_ids/target/freshness_class/max_disclosure_bytes/max_items），观察尺度面是零参数化的**——无时间窗、无轨道谓词过滤、无每 view 粒度档、无 topK；TemporalResolution 只是目录里的静态描述字符串。确认卡面预判的"零参数化基线"，但需精确表述为"尺度零参数化、选择面有五个粗旋钮"。
4. 盘出 **7 条参数化缺口**（§3，验收线 ≥6），每条带锚点；其中 view 键 vs evidence ref 两套命名法（L1-1 报告 §2.F）是即席查询谓词（L1-3）落地的最大结构性前置。

---

## 1. view 目录全集（18 view 逐一）

目录结构 `FreeStateObservationView`（`free_state_observation.go:20-32`）：view_id / questions / supported_target_kinds / **temporal_resolution（静态描述串）** / availability / cost_latency_class / quality_ceiling / limitations / required_dependencies / diagnostic_dimensions / interpretation_guidance。目录整体由 `FreeStateObservationCatalogFor`（:121-139）发布，boundary=`semantic_views_only`，四条 exclusions（raw PCM/波形谱阵列/完整 DAD 包/处理器选择与 mutation 权限，:132-137）。

每 view 的"数据源投影"= 绑定的**数据键族**（`keys`，装配时从 mixboard store 读这些键）；"裁剪规则"= 第一层 `compactProjectionForView`（:671-742）的分派。

| # | view_id | 数据源（keys） | 披露内容（quality_ceiling） | 第一层裁剪锚点 |
|---|---|---|---|---|
| 1 | `project.structure` | project.static.summary, project.tracks.summary, observation.tim_projection, observation.mom_projection | compact project and TOM-adjacent identity summary | :676-678 tracks→7 字段/轨（`compactProjectTracksSummaryForStructure` :744-758）；tim→**8 字段子集不含 llm_context/issues**（:703-705，TIM 设计卡 §1.3 之例）；mom→16+7 字段（:708-715 + :760-776）；tim 为可选补充键（`optionalSupplementalProjection` :591-602） |
| 2 | `project.change_delta` | project.change_delta | bounded Shadow Project change receipt | 无专用裁剪（透传，:741 fallthrough） |
| 3 | `track.basic_energy` | track.\<id\>.static.identity, track.\<id\>.fast.levels | bounded level summary | 无专用裁剪（fast.levels 已是紧凑层） |
| 4 | `track.time_dynamics` | track.\<id\>.slow.time_energy.summary, observation.com_projection | COM plus bounded time-energy summaries | :687-688 `compactCOMSourceDynamicsForSelection`（:832-876：levels/activity/macro_dynamics/events 白名单 + time_scale_coverage 只留 macro_program/micro_transient 两档 :842-848 + decision_support 面板） |
| 5 | `track.timbre_frequency` | track.\<id\>.slow.band_energy.summary | broad-band energy only | 无专用裁剪 |
| 6 | `track.peak_structure` | observation.dom_projection | sample-peak structure with explicit true-peak limitation | :691-701 + `compactDOMDimension`（:799-830：单维度字段 + 该维度 readiness 行 + trust_quality 9 字段） |
| 7 | `track.activity_structure` | observation.dom_projection | declared activity states and interval coverage | 同上（dimension=activity_structure，字段表 :1294-1298 contextruntime 侧同构） |
| 8 | `track.frequency_time_events` | observation.dom_projection | bounded time-frequency evidence with explicit omissions | 同上 |
| 9 | `track.transient_structure` | observation.dom_projection | bounded transient evidence with macro-only fallback | 同上 |
| 10 | `track.band_dynamics` | observation.dom_projection | bounded band-dynamics evidence with whole-window fallback | 同上 |
| 11 | `track.stereo_space` | track.\<id\>.slow.stereo.summary | balance/correlation summary | 无专用裁剪 |
| 12 | `mix.multitrack_relationship` | project.relationship_inputs, project.rankings.level, project.rankings.peak, project.risks.headroom, project.attention.first, observation.mom_projection | bounded MOM and project relationship summaries | :716-722 + `compactMOMMultitrackRelation`（:878-922：coverage 计数化；band_conflict_candidates **≤12 行、每行 tracks ≤4** :899-918）；rankings/risks/attention 为可选补充键（:595-602） |
| 13 | `mix.frequency_relationship` | project.frequency_relationship_inputs, observation.mom_projection | compact MOM frequency relationship projection | :723-724 + `compactFrequencyRelationshipInputsForView`（:781-797：6 字段 + 每轨 4 字段，decision_tracks_truncated 透传截断标志） |
| 14 | `mix.masking_relationship` | observation.mom_projection | compact MOM directional masking-risk projection | :725-736（**刻意丢通用 llm_context** 防预算挤占，注释 :726-730）+ `compactMOMMaskingRelationshipForView`（:924-957：conditions 8 字段、coverage 7 字段、candidates **≤12 条**、每条 13 字段） |
| 15 | `processor.identity_and_controls` | observation.com_projection | processor scope only | :680-682 selectFields 11 字段（无 gain_action 等行为面） |
| 16 | `processor.behavior` | observation.com_projection | compact COM behavior projection | :683-684 selectFields 17 字段（含 llm_context） |
| 17 | `processor.change_delta` | observation.com_projection | compact COM change projection | :685-686 selectFields 11 字段 |
| 18 | `comparison.before_after` | observation.before_after.latest | bounded delta summary | 无专用裁剪 |

**通用第二道 sanitize（所有 view 之上）**：`sanitizeFreeStateValue`（:984-1023）——深度 ≤10、列表 ≤ maxItems（默认 12、上限 24，:160-165）；`freeStateForbiddenField`（:1025-1036）剥 raw_\*/\*_samples/pcm/waveform\_array/spectrogram\_tiles/time\_segments/event\_list/\*\_path/file 系全部字段。**披露预算**：`MaxDisclosureBytes` 默认 64 KiB、钳制 [256, 65536]（:149-159）；超预算的 view 以 `OmissionBudget` 逐个剔除（:379-384），bundle 状态 ready→partial→insufficient（:392-396）。

**模型投影层（第二层裁剪，contextruntime/model_projection.go）**——CCB bundle 进入 LLM 上下文前的再投影：

- 分层预算常量：hot 10 KiB / warm 5 KiB / 全请求 24 KiB / digest 回退上限 2 KiB（:18-24）；预算执法 `enforceModelLayerBudgets`（:174-230）：warm 段最大优先降级为 `audit_snapshot://` 冷引用 → hot 辅助段（daw_state/semantic）→ 仍超则 fail-closed `context_overflow`（:214-228），**内容永不静默截断，只降级为引用**（注释 :119-123）。
- **active_observation（hot 层）逐 view digest 化**：`projectCCBViewDigest`（:1129-1187）——一行 summary（≤120 runes，:1153）+ evidence_refs（≤5 条，>160 runes 转 `evidence_ref_hash:`，:1382-1407）；无 LLMContext 的 view 走字段白名单回退，回退 >2 KiB 转冷引用（:1174-1175）。三个特例：masking 保留 bounded candidates 进 hot（:1139-1145）；`project.structure` 强制暴露 track 身份表（:1161-1165，`digestProjectTrackIdentities` :1202-1230——否则模型无法继续 track 定向观察，注释 :1158-1161）。
- **warm ledger（观察账本）**：`ProjectObservationLedger`（:306-351）——available_views 最新优先留 24 对（:317-319），rejected_view_sets 跨轮持久（durable do_not_retry），receipts 只留当前 delta 窗口（:342-346）；超限行折叠为计数 + 冷引用（`observationLedgerWindow` :474-486）。每 view 结论 digest `ProjectCCBViewConclusion`（:635-663）+ `projectCCBDecisionDigest`（:670-781，逐 view 的 decision-shape，MaxListItems 收到 4）。
- **禁键过滤**：`forbiddenModelProjectionKey`（:1688-1701）——plugin/vendor/manufacturer/product/topology/parameter\_id/recommended\_family/processor\_type/sealed\_truth/expected\_view 等全部键名包含即删：模型面不出现处理器身份与预填倾向（防泄漏的硬边界）。
- **catalog 短索引投影**：`projectCCBCatalog`（:997-1023）只留 view_id/availability/limitations/首条 question，omitted\_fields 明示砍了 supported\_target\_kinds/temporal\_resolution/cost\_latency\_class/quality\_ceiling/required\_dependencies（:1012）。

## 2. 请求与披露协议现状

**请求如何表达（模型→CCB）**：

- 模型侧唯一入口是两个 harness 工具：`ccb.observation_catalog`（发现目录）与 `ccb.observation_request`（显式 view_ids 请求）。工具 schema：`agent/internal/tools/catalog.go:629-676`——request 必填 `view_ids`（1-12 条），可选 target\_kind/target\_id/target\_label、freshness\_class、max\_disclosure\_bytes(256-65536)、max\_items(1-24)、observation\_id/mix\_session\_id/scope；harness 分发 `agent/internal/harness/harness.go:2436-2439`。
- **不是混在 prompt 里的自然语言指示**：协议是结构化的 `needs_observation` 决策 + tool_calls（`agent/internal/agentloop/ccb_model_prompt.go:61-62`），`requested_view_ids` 必须与 `args.view_ids` 精确一致（prompt 规则 :78）。prompt 里注入的是**目录本身**（"Available observation catalog" :97-98）与 view 选择纪律（自选 view、无默认映射、track view 必须显式 target、:71-79）。
- 请求落地管线：`harness/ccb_observation.go:20-102`——view_ids 合法性校验（未知 view 拒绝 :39-41/:165-178；`ValidateFreeStateObservationViewIDs` free\_state\_observation.go:170-189）、track 目标护栏（fresh 请求的 track.\* view 必须带显式 target，reject+guide 不自动选择，:184-197）、deferred/unavailable 目录项在物化前拒绝（:204-224）→ 无 observation\_id 时先物化 mixboard 观察（masking 需先备同窗探针 :59-64）→ 读 observation.binding + view 键族（`FreeStateObservationReadKeys` :252-264）→ `AssembleFreeStateObservation`（:266-419）。
- **语义 view 决定权威观察范围**：`ccbObservationCommand`（:257-305）把请求翻译成 `mix_request_observation`，view 前缀派生 scope（`ccbObservationScope` :307-332）并显式禁止模型把 project 关系 view 缩窄到单轨（注释 :269-271）；mom\_intent/dom\_mode/com\_mode 由请求的 view 集合推导（:284-303）。
- **审计回执**：每次请求产生不可变 `FreeStateObservationAuditReceipt`（free\_state\_observation.go:68-88）：model\_requested\_view\_ids vs actual\_executed\_view\_ids、view\_set\_matches、scope、freshness、project binding 三件套（uuid/epoch/revision，:313-319 进 bundle.Freshness）。chat 侧 `semantic_progressive_disclosure.go:100-130` 以该回执为权威证明（无回执即拒绝，防伪造观察）。

**partial/stale 的表示与透传**：

- 键级状态由 `semanticItemStatus`（:604-660）从源行 status 解出（含 mom/com/dom 的嵌套字段特判），view 级汇总 `assembleFreeStateView`（:573-582）：ready / missing / stale / partial。
- 缺席透传三通道：`Omissions` map（Forbidden/Unavailable/Stale/Budget 四类，:356-378）、`OmissionReasons` 字符串表、`Limitations`（partial/suspect/approximate 追加 "evidence status is X"，:387-389）——原样保留不升级（与 AGENTS §5 投影纪律一致）。
- 拒绝（rejected bundle）带 rejection\_scope="exact\_view\_set" + blocking/non\_blocking view 拆分（:195-235），prompt 侧规则教模型按 blocking 集重选（ccb\_model\_prompt.go:81-82）。
- 模型投影层再映射：`projectionStatus`（model\_projection.go:1429-1438）partial/stale/suspect/approximate→"partial"，missing/deferred/unavailable/blocked→"omitted"；ledger rejected\_sets 跨轮可见防重复请求（:522-534）。

## 3. 参数化缺口清单（对照路线图 scope×time_window×粒度/topK/谓词）

**基线结论**：请求面的现有参数 = view\_ids（选择什么）+ target\_kind/id（单目标）+ freshness\_class + max\_disclosure\_bytes + max\_items（两个全局体积旋钮）。**尺度参数（时间窗×范围×粒度）、topK、谓词过滤全部缺位**；所有裁剪常数编译期硬编码。逐条：

1. **无时间窗参数**：`TemporalResolution` 是目录静态描述串（"whole window"/"macro and short-window summary"，:429-431 等），请求 schema（tools/catalog.go:655-671）与 `FreeStateObservationRequest`（:53-66）均无 time\_window 字段——同 view 无法换窗观察。互证 L1-1 报告 §3 I2（TimeWindow 三种结构+一种字符串，多数 ref 无时间窗段）与 §4（time_window 是五元组最缺位字段）。
2. **无轨道谓词/多目标选择**：TargetRef 单目标；mix.\* view 的 scope 由 view 类型硬定（ccbObservationScope :307-332），模型不能表达"只比较这 3 条轨的 masking"或"排除人声轨"。frequency\_relationship\_inputs 的 `decision_tracks_truncated`（:785）是投影侧固定截断的**事后标志**，不是可设过滤。
3. **粒度只有两个全局粗旋钮**：max\_items（列表条数，默认 12）与 max\_disclosure\_bytes（总字节）作用在**整个 bundle** 上（:149-165）；每 view 无独立粒度档——band 数、segment 分布 bin、candidates 字段集全部固定（§1 表）。
4. **topK 不存在于请求面**：候选裁剪常数硬编码——masking candidates ≤12（:938-942）、multitrack candidates ≤12 行×4 轨（:899-918）、decision digest MaxListItems=4（:645/:688）、evidence\_refs ≤5（model\_projection.go:1387）。模型不能要 top-20 或"只要 median\_margin\_db 最大的 3 条"。
5. **freshness\_class 是无枚举自由串**：schema 无 enum（tools/catalog.go:661），normalize 仅在空时默认 "current\_observation"（:146-148）——"post\_action"（audition\_candidate\_adoption.go:164 在用）等值靠调用方拼写纪律，语义未校验。
6. **目录查询无参数面**：`ccb.observation_catalog` 仅 target\_kind/target\_id（tools/catalog.go:636-638），不能按 diagnostic\_dimension/availability/cost 过滤；`GetViewsForDimension`（:1143-1155，按维度反查 view）**全库仅测试调用、无生产调用方**（grep 实证）——按维度推荐 view 的钩子存在但未接线，即席查询（L1-3）可复用它做目录谓词入口。
7. **view 键与 evidence ref 两套正交命名法**（互证 L1-1 报告 §2.F/§3 I3/I5）：view 数据键 `track.<id>.fast.levels`（点分）vs evidence ref `mix.read:track.<id>.fast.levels`（冒号 scheme），靠 `FreeStateObservationReadKeys`+装配代码手工对应（:252-263）；跨 view 组合查询若以 ref 为谓词，需先统一两套键（G1 ref schema 的直接输入）。

**参数化要动哪里（逐条对应）**：①②⑤⑥ 改 catalog 结构+请求 schema（`FreeStateObservationView` 加参数声明、request 加字段、tools/catalog.go schema 同步）；③④ 改装配侧 per-view 裁剪函数签名（把常数变成参数）+ 模型投影 digest 限额联动；⑦ 是 G1（ref schema）前置而非 CCB 单侧可解。

## 4. 与 D1/D2 的衔接点（ref 谓词输入的现状）

- **scope\_id 现成载体**：请求侧 `TargetRef`（kind/id/label，结构化）与派生 scope（full\_project/selected\_track/full\_project\_with\_focus\_track 等 5 值，:512-538）已具备 D1 scope 的表达雏形；但进入 bundle 后只剩 target\_ref map + scope 字符串，**没有 track 集合表达能力**（多轨谓词无载体）。
- **time\_window 完全缺位**：请求→bundle→ledger 全链无时间窗字段；freshness map 携带的是 project\_uuid/epoch/revision（:313-319）与 observed\_at，是**版本身份不是观察窗**。与 L1-1 报告 §4 五元组对照结论一致。
- **snapshot 侧**：bundle 带 observation\_id（mixboard 观察票，可按 ID 重拉——L1-1 报告 §2.B1）+ project\_revision；audit 回执含完整 project binding。**ref 谓词若要绑定"哪次观察"，observation\_id 是当前唯一可用句柄**；request\_id 只是幂等键。D2 的"证据引用可校验"在 CCB 面已有 view\_set\_matches 审计（模型请求=服务执行），但 ref 字符串本身的格式校验不存在（agentprotocol Validate 只查非空，L1-1 报告 §1）。
- **两套键统一是 D1/D2 与 L1-3 的共同前置**：CCB view 键（点分）成为查询谓词还是 evidence ref（冒号）成为统一键，决定 G1 schema 的主键选型；本卡事实：CCB 内部以 view 键为权威、ref 只在 bundle 边界出现，**view 键体系更接近查询引擎的天然主键**。

## 5. 验收对照

| 卡面验收项 | 状态 |
|---|---|
| ① view 目录 ≥10 逐一列出（键名/内容/裁剪/锚点） | ✅ 18 个（§1 表），每条带数据键族+裁剪锚点 |
| ② 参数化基线结论 | ✅ §0-3：尺度零参数化；选择面 5 参数（view\_ids/target/freshness\_class/max\_bytes/max\_items）逐一定位 |
| ③ 缺口清单 ≥6 条带锚点 | ✅ 7 条（§3），每条带文件:行 |
| ④ 报告入库 | ✅ 本文件 `coord/runs/CCB-VIEW-RECON-1/CCB_VIEW_INVENTORY.md` |
| 零代码改动 | ✅ 仅新增本报告文件；工作树无源码改动 |

## 6. 边界与未覆盖项

- `agent/internal/capabilitycontext/` 其余文件（context\_manifest/context\_builder/pack/gain\_staging 等）是 capability pack 装配面，不是 CCB view 目录的组成部分（manifest 无 view 概念，本报告 §1 范围以 free\_state\_observation.go 为权威单点）；两者关系在 L1-4 卡的装配链里覆盖。
- prompt 侧与 ccb 观察的**时序治理**（continuation budget / terminal turn / admission gate 披露，ccb\_model\_prompt.go:431-502）属 L1-4（上下文装配）范围，本卡只盘 view 目录与参数化。
- Godot 前端仓（仓库外）无 CCB view 消费面，未跨仓取证（与 L1-1 报告 §6 同边界）。
