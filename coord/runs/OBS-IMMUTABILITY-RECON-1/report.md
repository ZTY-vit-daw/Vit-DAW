# OBS-IMMUTABILITY-RECON-1：观察产物共享引用穿透勘察报告

- 执行卡：`coord/cards/todo/2026-09-29-OBS-IMMUTABILITY-RECON-1.md`
- 日期：2026-09-27（会话日期；卡面标注 2026-09-29）
- 性质：只读勘察，零代码改动。HEAD=`13e58d83`（领取时 main，`git pull` already up to date）。
- 输入：MAT-D3 取证（`coord/runs/MAT-D3-1/forensics/README.md`）+ 当前工作树代码。
- **工作树并行改动申报（§12 纪律）**：领取后发现 materialize 域存在并行未提交改动
  （`materialize_shadow.go`/`adapters.go`/`metrics.go`/`recompute.go` 修改 + 新增
  `timing_carried.go`/`timing_carried_test.go`）——MAT-D4 timing-carried 登记实现
  （裁定②：时序两态分歧登记排除，对账改走 `ReconcileShadowWithTiming`，
  `materialize_shadow.go:121`）。本卡未触碰这些文件；锚点行号以**当前工作树**
  （含该并行改动）为准。该实现与本报告结论互为印证：它登记的"观察投影=finalize
  世代 vs 物化行=尾挂世代"正是本报告定案的 W1 两态（§0）；登记是观测面止血，
  本报告 §4.2 R1 是根因侧修复方向。

## 0. 一句话结论（定案）

**MAT-D3 §3.4 的两个候选机制（遥测 ingest 原地穿透、finalize 内 strip 触发）全部否定。**
真机制是：**v2 持久化证据投影在 finalize 之后、返回/落盘之前，对深拷贝包整体替换
`GlobalSummary["feature_snapshot"]`（递归剥离 `time_segments` 等载荷键）**——
`agent/internal/mixboard/persistence_v2.go:42 → 107-163`。观察包因此存在两个态：

- **State A（finalize 态）**：inline snapshot 含 `time_segments`；DOM/MOM/TIM/FXM/COM
  投影与 Digest/Catalog 全部从它计算（`dom_projection.go:25`）。
- **State B（持久化/返回态）**：`cloneObservationPacket` 深拷贝后，inline snapshot 被
  `persistentFeatureProjection` 替换——`time_segments`/`time_energy_rows`/
  `spectrogram_tile_rows`/payload 类键递归删除、数组 512 截断、注入 `evidence_ref`
  （`persistence_v2.go:126-163`）。

**返回给调用方与尾挂物化的 `WriteResult.Observation` 是 State B；`obs.DOMProjection`
字段却是 State A 的产物。** 尾挂 `materializeDepInputs` 用 State B 重算 DOM（fresh），
对账 stored 侧 `DOMRowFromObservation` 用 State A 派生的 `obs.DOMProjection`
（`materialize/measurement_carried.go:54-58`）——两态必然分歧且各自确定性恒定，
与 MAT-D3 真栈 S2 四轮"fresh/stored 各自恒定、payload 相等"的形态完全吻合。
"共享引用穿透"（compact 嵌套浅引用）**存在但当前无写入者触发**，不是 S2 的原因（§2.2）。

### 0.1 对 MAT-D3 证据的逐条吻合

| MAT-D3 观察 | State A/B 机制解释 |
|---|---|
| 尾挂 deps 侧 `waveform_envelope` 顶层键在场、`time_segments=false` | 投影替换不删顶层键，只递归删 `time_segments`（`persistence_v2.go:144`） |
| deps 侧 band 行 `band_dynamics` 在场、keys=44 | `bands`/`band_dynamics` 不在剥离键表 |
| 观察侧投影有 `segment_peak_dbfs_distribution`/`activity_structure=ready`，物化侧 missing | 这些字段由 `time_segments` 派生；State A 有、State B 无 |
| 三角重放"同一 obs 包"重放 `dom.Build ≠ obs.DOMProjection` | 重放输入=State B 包，投影=State A 产物 |
| 合成测试绿、真栈 S2 红 | 合成/legacy 路径不进 v2 投影（§1 步骤 6 的环境分支），无两态 |

## 1. 穿透路径全景：观察产物从 finalize 到消费的全生命周期

单轮 `mix_request_observation` 时序（真栈 v2 工程，锚点按执行序）：

1. **入口**：`harness/harness.go:3443` `requestMixObservation` →
   `mixboard.NewStore("").RequestObservation`。
2. **私有快照解析**：`mixboard/mixboard.go:1087` `loadFeatureSnapshot`
   （`mixboard.go:1455-1504`）。两条路径（args 注入 / 快照文件读）都经
   JSON marshal/unmarshal——**每次观察的快照行都是全新对象，不与其他持有者共享**。
3. **State A 组装**：`mixboard.go:1088` `buildObservation` → `mixboard.go:1252`
   `compactFeatureSnapshot(*featureSnapshot)` 进 `GlobalSummary["feature_snapshot"]`。
   compact 新建顶层 map，但嵌套值（`time_segments`/`bands`/`band_dynamics` 等）按引用拷贝
   （`mixboard.go:2892-2900` `compactFeatureRow` `out[key] = value`；
   `mixboard.go:2902-2911` `compactTrackFeatureRows`）——**obs 包 inline snapshot
   与私有解析快照共享嵌套行**（窗口分析见 §2.2）。
4. **before/after**：`mixboard.go:1089` `applyBeforeAfterDelta`
   （`mixboard.go:3233-3266`）——只写 `MixPackage`（delta/ab_result/caps/missing），
   不碰 snapshot 行。
5. **finalize**：`mixboard.go:1090` `FinalizeObservationContext`
   （`mixboard/catalog.go:59-92`）：DOM（早跑/兜底）→ MOM → TIM → FXM → COM →
   Digest → Catalog →（仅 band-stereo wants 时）filter+`stripObservationRawKeys`。
   各投影从 State A 读（`dom_projection.go:25`）；**全部只读**
   （dom/com/mom/tim/fxm projection 文件对 snapshot 无写入，均为局部变量绑定）。
6. **两态分叉点**：`mixboard.go:1091` `projectObservationForPersistence`
   （`persistence_v2.go:13-75`）：
   - `persistence_v2.go:14-16`：**legacy/dev 早退**（`VIT_MIXBOARD_ROOT` 设置或
     `projectstore.Current()` 未激活）——返回原 State A 对象，无深拷贝无剥离；
   - v2 路径：`:28` `cloneObservationPacket`（JSON round-trip，State A→全新容器，
     共享引用链在此关闭）→ `:34` `PutEvidence` 把**全量 State A raw snapshot
     （含 time_segments）**入证据库 → `:42` `applyObservationEvidenceProjection`
     （`:107-124`）**替换** `GlobalSummary["feature_snapshot"]` 为剥离投影
     （`persistentFeatureProjection` `:126-136` + `stripLargeObservationValue`
     `:138-163`，剥离键表见 `:144`：`tile_payload/payload/samples/sample_values/
     pcm/waveform/spectrogram_tile_rows/time_segments/time_energy_rows/raw_rows`，
     数组 512 截断）；`EnvironmentPackage`/`DeepPackage` 的 `feature_snapshot`
     删除改 `feature_snapshot_ref` → 超预算时 `:52→:165-194`
     `applyObservationHardProjection` 把 snapshot 整体降级为 `{status:projected}` →
     `:62-66` 落盘 v2 canonical `observations/<id>.json`（State B）。
   - **返回值是 State B**（`mixboard.go:1125-1133` `WriteResult.Observation =
     persistedObservation`）。
7. **board/context_pack**：`mixboard.go:1114/1119` 从 `persistedObservation`（State B）
   构建落盘。ContextPack 只装投影的 Context 形态，不含 feature_snapshot。
8. **尾挂物化**（S2 分歧现场）：`harness.go:3468-3470`
   `materializeObserveRound(&result.Observation)`——拿的是 State B 指针：
   - `harness/materialize_shadow.go:226`
     `deps.FeatureSnapshot = obs.GlobalSummary["feature_snapshot"]`（**State B**）；
   - `materialize/recompute.go:122` `RecomputeLazy` →
     `materialize/adapters.go:315/346` DOMAdapter 以 deps.FeatureSnapshot 合成观察包重算
     → **fresh 行=State B 重算**；
   - `materialize_shadow.go:113` `DOMRowFromObservation` →
     `materialize/measurement_carried.go:54-58` 行化 `obs.DOMProjection`
     → **stored 行=State A 派生**；
   - `materialize_shadow.go:121` `ReconcileShadowWithTiming` 对账 → 分歧
     （MAT-D4 并行改动后：该时序世代差分歧走 timing_carried 登记排除；
     HEAD 版此行为 `ReconcileShadowDetailed`）。
9. **工具结果/事件流**：`harness.go:3471-3485` `out["observation"] = result.Observation`
   （State B）→ agent loop 工具结果序列化进 LLM 上下文与 chat 事件流。

### 1.1 finalize 后对观察包的全部写入点清单（锚点+时序）

| # | 写入点 | 锚点 | 时序（相对 finalize） | 作用对象 | 性质 |
|---|---|---|---|---|---|
| W1 | 证据投影替换 `feature_snapshot` | `persistence_v2.go:113` | finalize 后、返回/落盘前 | 深拷贝包 | **S2 真机制**；v2 路径恒触发 |
| W2 | 硬投影降级 `feature_snapshot` | `persistence_v2.go:179` | 同上，超预算时 | 深拷贝包 | W1 的预算兜底 |
| W3 | band-stereo 投影整体替换 GlobalSummary/MixPackage/Env/Project | `mixboard.go:3856-3960`（`applyBandStereoProjection`，catalog.go:71 调用） | finalize 内、Digest 前 | State A 包 | 仅 band-stereo wants；S2 不触发 |
| W4 | finalize 内 strip（删 raw 键） | `mixboard.go:4299-4334`（catalog.go:88-91 调用） | finalize 末尾 | State A 包 | 仅 band-stereo wants；S2 不触发 |
| W5 | before/after 写 MixPackage | `mixboard.go:3233-3266` | finalize 前 | State A 包 | 不碰 snapshot |
| W6 | 遥测 ingest 四分支 | `harness.go:3937/4011/4036/4072` | 异步于观察（无时序耦合） | **文件态快照**（`harness.go:6057/6086/6119/6151` → `readMixboardFeatureSnapshotFile` `harness.go:7474` 每次新解析 → merge → `writeMixboardFeatureSnapshotFile` `harness.go:7187` marshal 落盘） | **与内存观察包零共享**（§2.1） |

结论：finalize 之后对"返回包"的唯二写入是 W1/W2（persistence 投影）；对"finalize 前包"
的写入全部发生在 finalize 时间窗内（W3/W4/W5），属于 finalize 态自身的组成。

## 2. 共享引用持有者清单与窗口分析（卡片目标 1 续：未定论定案）

### 2.1 遥测 ingest ↔ 观察包：不存在共享引用通路（定案否证）

四分支逐一：

- `ingestKernelRenderTelemetry`（`harness.go:3937`）：只写 `h.renderResults`/waiters，
  不碰任何快照。
- `ingestKernelWaveformTelemetry`（`harness.go:4011`）：collector 在 `h.featureMu` 下
  `AddEvent`+`SnapshotRow`（行对象为 collector 新建）→
  `writeMixboardReadyProjectTrackWaveformSnapshot`（`harness.go:6086`）。
- `ingestKernelSpectralTelemetry`（`harness.go:4036`）→ `harness.go:6119`。
- `ingestKernelL3AcousticTelemetry`（`harness.go:4072`）：`row := cloneAnyMap(event)` →
  `harness.go:6151`。

三个 `writeMixboardReady*Snapshot` 的共同形态：`readMixboardFeatureSnapshotFile(path)`
**每次全新 JSON 解析**（`harness.go:7474-7484`，无缓存单例）→ 在这份私有解析上
merge/stamp/annotate（`harness.go:6060/6089/6122/6154`、`7188-7190`）→
`writeMixboardFeatureSnapshotFile` marshal+落盘（`harness.go:7187-7206`）。
merge 目标行在单次调用内即被序列化，无跨调用存活对象。**遥测写入的是文件态；
每次观察构建又独立解析文件。两边之间没有任何共享的内存行。**

### 2.2 compact 浅引用：真实存在、当前窗口内无写入者（潜在隐患而非现行缺陷）

- 持有者：`obs.GlobalSummary["feature_snapshot"]` 的嵌套行（`mixboard.go:2764/2892/2902`
  浅拷）↔ 步骤 2 的私有解析快照行；同一浅引用族还包括 `MixPackage.current_metrics`
  各 summary 块（`buildBandEnergySummary` 等，`mixboard.go:1239-1244`）。
- 窗口：`mixboard.go:1088`（compact）→ `persistence_v2.go:28`（cloneObservationPacket
  深拷断链）。v2 路径窗口必然关闭；legacy 早退路径不 clone，但返回的就是同一对象、
  私有快照出作用域后无人再碰。
- 窗口内写入者：**无**。W3/W4/W5 对包的写入不落在与源快照共享的行上
  （W5 写 MixPackage 新键；W3 整体替换；W4 删的是包内键，且仅在 band-stereo 分支，
  而该分支下 GlobalSummary 已被 W3 换成新 map，不再挂源快照引用）。
- 判定：**该浅引用是防御性缺口**（未来任何人在窗口内"顺手补写"源快照行即穿透），
  但当前 S2 分歧与它无关。MAT-D3 §3.4 候选一的"finalize 后遥测 ingest 原地更新穿透"
  在当前代码里**无通路**。

### 2.3 "遥测穿透 vs strip 触发"定案（卡片目标 1 的显式问题）

**两者皆否，第三机制成立**：

1. 遥测穿透：否——通路不存在（§2.1）。
2. finalize strip（`stripObservationRawKeys`）：否——仅 band-stereo wants 触发
   （`catalog.go:88`，谓词 `mixboard.go:3837-3854`），且它删**顶层**
   `waveform_envelope`/`track_waveform_envelopes` 键——与 MAT-D3 §3.2"deps 侧顶层键
   在场"直接矛盾。
3. **真机制 = 持久化证据投影**（W1，`persistence_v2.go:42/107-163`）：不删顶层键
   （与尾挂观察一致）、恰好递归删 `time_segments`（与 deps 侧 `time_segments=false`
   一致）、不删 `bands`/`band_dynamics`（与 keys=44 行一致）、legacy/合成路径不触发
   （与"合成绿、真栈红"一致）。

## 3. 受影响消费方清单：逐面判定"读到哪个态"

| 消费面 | 入口锚点 | 读到的态 | 判定依据 |
|---|---|---|---|
| obs JSON 落盘重读（v2 canonical） | `persistence_v2.go:62-66` | **State B** | 落盘的就是投影后包 |
| obs JSON（legacy session 路径） | `mixboard.go:1105-1112` | **State A** | 早退返回原对象（环境依赖！） |
| mix_read / Store.Read | `mixboard/catalog.go:299-334`（`findObservation` `:373-449` 文件重读，fresh parse 无共享） | 落盘态：真栈 v2=**State B**，legacy=State A | 内容污染由落盘时机决定：落盘在 W1 之后 |
| mix_read `.raw.` 键的证据回溯 | `catalog.go:317-319/588-611`（`lazyFeatureSnapshot`→`projectstore.GetEvidence`） | **State A raw**（含 time_segments） | 证据 blob 在 W1 前由 `PutEvidence`（`persistence_v2.go:34`）写入 |
| ContextPack（context_pack.json） | `mixboard.go:1119` `buildContextPack(persistedObservation)` | 投影字段=State A **派生值**；不含 inline snapshot | ContextProjection 来自投影结构体（clone 保真） |
| Board（current.json） | `mixboard.go:1114` | State B 摘要面 | 同上 |
| 事件流 / LLM 工具结果 | `harness.go:3471-3485` `out["observation"]` | v2=**State B** | 返回值即投影后包；LLM 看到的 inline snapshot 无 time_segments |
| **尾挂物化 fresh 侧** | `materialize_shadow.go:226` → `adapters.go:315/346` | **State B** | **S2 分歧点** |
| **尾挂对账 stored 侧** | `measurement_carried.go:54-58`（`obs.DOMProjection`） | **State A 派生** | 投影在 W1 前计算，clone 保真携带 |
| 下一轮 before/after | `mixboard.go:1086`（`readPreviousObservation`）+ `:3233` | previous=落盘态（v2=State B）vs current=State A | 比较面是 MixPackage 标量（rms/peak/balance 等），剥离不命中——语义实质等价，但严格说是跨态比较 |
| 观察文件频率行恢复 | `mixboard.go:420-440`（`readFrequencyRowsFromObservations`） | State B，但 band/stereo/loudness 行不被剥离键命中 | **不受影响** |
| TIM finalize 证据 | `mixboard.go:4072`（`timAcousticEvidenceByTrack`） | State A（finalize 期） | `nan_count/inf_count` 不在剥离键表 |
| render probe 观察回读 | `mixboard.go:3372` | State B；`l2_render_probe` 行不被剥离命中 | 不受影响 |
| journal | 不直接存 obs 结构；工具结果 JSON 文本随消息记录 | State B 文本 | 间接面 |

**行为耦合判定（卡片目标 3 的前置问题）：没有任何消费方把"穿透后态（State B 的剥离）"
当作语义依赖。** State B 的剥离是 v2 持久化预算机制（observation budget/gate，
`persistence_v2.go:51-61`），不是信息消费需要。唯一"依赖"是结构性巧合：尾挂物化把
State B 当作"与 finalize 同源同态"的输入——这正是缺陷本身，不是合法耦合。因此
把尾挂对齐回 State A 不会破坏任何现有合法消费方。

## 4. 修复方案评估

### 4.1 卡面三案（前提=遥测穿透；该前提已否定）

| 方案 | 改动点 | 性能量级 | 对 S2 分歧 | 行为影响 |
|---|---|---|---|---|
| ① compact 时全深拷 | `mixboard.go:2764-2911`（compact 三函数）或两调用点 `:1252/:3163`；方法=JSON round-trip 或递归拷 | 每轮观察一次 snapshot 单树 round-trip。观察轮=agent 工具调用驱动（活跃会话每分钟个位数；MAT-0 计数器已部署、真栈基线未采集）；v2 路径每轮已有 `cloneObservationPacket` 全包深拷+3 次 marshal 落盘，本方案增量约为全包拷的同阶或更低（snapshot 树 ≈ 全包子集）——每轮毫秒~十毫秒级（MB 级 JSON 量级），不改变现有成本量级 | **无效**。剥离发生在深拷之后的 W1，深拷只断 §2.2 窗口，而窗口内当前无写入者 | 无（当前无依赖窗口内变更的消费者）。价值=封死潜在隐患（防御性），可作为卫生改动独立评估 |
| ② 嵌套行写时复制 | 无现存触发点（窗口内无写入者）；要落地需改 compact 返回类型为写时复制 wrapper（波及全部 compact 消费面）或给未来写入者立纪律 | 零成本直到有写 | **无效**（同①） | 需要持续纪律，不如①一次性断链；不推荐单独采用 |
| ③ 遥测 ingest 改写前克隆 | —— | —— | **无效** | **空操作**：ingest 已在每次全新解析+新建行上工作（§2.1），现状即等价于"改写前克隆" |

三案共同问题：针对的机制不存在。①有防御价值，②③无增量价值。

### 4.2 真机制（W1 两态）对应的修复方向（供决策侧裁定，均超出本卡文件域）

| 方向 | 改动点 | 性能 | 行为影响 | 评估 |
|---|---|---|---|---|
| **R1 尾挂换源到证据 blob（推荐）** | `materialize_shadow.go:225-228`：inline State B 之外优先经 `obs.EvidenceRefs` 的 feature_snapshot 证据 ref 回溯 State A raw（复用 `catalog.go:588` `lazyFeatureSnapshot` 的 `projectstore.GetEvidence` 逻辑） | 每轮一次证据 blob 读+反序列化，与现有 `loadFeatureSnapshot` 同阶 | DOMAdapter 重算输入恢复 time_segments → fresh 与 stored 应收敛（timing_carried 登记计数应归零）；其他消费方零影响；需定义 `ErrEvidenceDisabled` 时的回退（回退 inline 并登记缺口，或跳过本轮） | 改动集中单文件；语义=物化与 finalize 同源同态（G2-D 设计意图）；不动 v2 预算机制；与 MAT-D4 已入池的 timing-carried 登记互补（登记治标、R1 治本，R1 落地后登记面自然归零） |
| R2 包内 finalize 态快照字段 | `ObservationPacket` 增字段 + persistence + materialize_shadow | 每轮多携带一份 snapshot 尺寸负载（或仅引用） | 持久化格式变更——触发 §11 兼容义务（旧记录缺省语义+往返测试） | 比 R1 重，收益相同 |
| R3 持久化投影放宽键表 | `persistence_v2.go:144` 去 `time_segments` | observation 尺寸回归（time_segments 是载荷大头） | 动 v2 预算/gate 机制，与"raw 走 evidence_ref"设计原则相悖 | 不推荐 |
| MAT-D3 §四 B（对账基准换源=尾挂态重算） | materialize 对账语义 | 零 | stored 侧也改 State B 重算→物化行与 `obs.DOMProjection`/LLM 上下文（ContextPack、mix_read dom_projection 仍是 State A 派生）持续不同源——隐藏两态而非修复；且"观察基准定义变更"需裁定 | 不推荐 |
| MAT-D3 §四 C（DepInputs 扩轴） | —— | —— | 已被 MAT-D3 3.3 否决；本次定案补强：缺的是 time_segments（被剥离），不是轴 | 已否决 |

**推荐组合**：R1 修 S2（G2-D 收敛）+ 可选方案①作卫生改动封浅引用隐患（独立小卡）。
R1 落地后真栈 S2 预期 `ShadowDivergences→0`，但需真栈重跑验证（本卡只读未改码）。

### 4.3 性能量级评估方法学（不编造精确数）

- 频率：MAT-0 per-kind finalize 计数器（`mixboard/observation_metrics.go`，接线
  `MaterializeMetrics.log`，`cmd/vitagent/main.go`）已部署；真栈基线尚未采集
  （done 卡声明"基线采集待真实工程观察会话"）。量级判断依据=观察轮由 LLM 工具调用
  驱动（活跃混音会话每分钟个位数、无人时零），非音频回调节奏。
- 成本：以"同阶既有操作"论证——v2 路径每轮已有全包 JSON round-trip 深拷
  （`cloneObservationPacket`）+ PutEvidence marshal + 3 次落盘 marshal；
  ①/R1 的增量都 ≤ 现有单轮深拷密度，不改变每轮毫秒~十毫秒级（MB 级 JSON）的量级。

## 5. 边界与申报

- 本卡只读：未改任何源码/测试/脚本；未跑真栈（无 run ID）；全部结论来自代码锚点
  推演 + MAT-D3 既有真栈工件交叉验证。
- "真机制=W1"的判定依据是代码路径唯一性（finalize 后对返回包的写入点仅 W1/W2）
  + MAT-D3 四组观察逐条吻合（§0.1）+ 合成/真栈环境分支解释（§1 步骤 6）。
  未做动态复现（属修复卡验证范畴）。
- legacy/dev 路径（`VIT_MIXBOARD_ROOT` 或无 v2 store）无两态——这解释了全部合成
  测试为绿的成因，也意味着修复验证必须在真栈 v2 栈上跑 S2。
- G2-D 状态：本卡不推进闭环；R1 是决策侧待裁定的修复方向。

## 6. 关键锚点索引

| 主题 | 锚点 |
|---|---|
| 观察主流程 | `harness.go:3303`（requestMixObservation）、`:3443`（RequestObservation 调用）、`:3468-3470`（尾挂） |
| RequestObservation 生命周期 | `mixboard.go:1072-1134`（1087 load / 1088 build / 1089 delta / 1090 finalize / 1091 persistence） |
| Finalize | `mixboard/catalog.go:59-92` |
| DOM 输入取 State A | `mixboard/dom_projection.go:25` |
| compact 浅引用 | `mixboard.go:2764`（compactFeatureSnapshot）、`:2892-2900`（compactFeatureRow）、`:2902-2911`（compactTrackFeatureRows） |
| 两态分叉（W1/W2） | `mixboard/persistence_v2.go:13-75`（28 clone / 34 PutEvidence / 42→107-124 替换 / 126-163 剥离 / 165-194 硬投影） |
| 遥测四分支 | `harness.go:3937/4011/4036/4072`；写文件族 `:6057/6086/6119/6151/6201`；文件解析 `:7474`；落盘 `:7187` |
| finalize strip（W4，S2 不触发） | `mixboard.go:4299-4334`；谓词 `:3837-3854`；调用 `catalog.go:88-91` |
| 尾挂取 State B | `harness/materialize_shadow.go:226`；重算链 `materialize/recompute.go:122`、`materialize/adapters.go:315/346` |
| stored 侧 State A | `materialize/measurement_carried.go:54-58` |
| 证据回溯 State A raw | `mixboard/catalog.go:588-611`（lazyFeatureSnapshot）、`persistence_v2.go:34`（PutEvidence） |
| mix_read 文件重读 | `mixboard/catalog.go:299-334/373-449` |
