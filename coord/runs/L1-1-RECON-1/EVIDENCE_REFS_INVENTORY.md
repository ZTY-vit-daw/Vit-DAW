# L1-1-RECON-1：现存 evidence refs 格式全库盘点报告

- 执行侧：PC 会话（GLM-5.3 / L1 勘察）
- 领取时 HEAD：`911e331275206090126f79a1cf514604a8f9a1af`
- 领取时 `git status --short`：` M VitApp/Workspace/default_project.xml`（运行时工程状态，不动）+ 12 个 untracked `coord/runs/*` 工件目录（FIX-PCA-AUTOSWEEP-1 / FIX-PCA-EQCHANNEL-1 / PORT-WIN-HYGIENE-SMOKE-1，均为领取前已有，未触碰）
- 性质：只读勘察，零代码改动；所有样例为真实摘录，`文件:行` 以领取时 HEAD 为准
- 日期：2026-09-27

---

## 0. 执行摘要

1. **全库没有统一的 evidence ref 类型**。所有层的引用统一落在 `EvidenceRefs []string`（自由字符串）上：LLM 提案（`agentprotocol`）、九个投影包、DAD 记录、CCB 披露、执行回执全部如此。格式的约定全部隐含在各生成点的字符串拼接里。
2. **盘点到 31 处生成点**（Go 侧 26 + 内核 C++ 侧 5），归并为 8 类身份/引用方案（§2）。同类语义至少 7 组不一致（§3，验收线 ≥5 已超额）。
3. **D1 五元组（projection_kind/scope_id/time_window/snapshot_id/content_hash）没有一个字段拥有全库统一名字**；最接近的结构化键是 `audioclosure.ObservationKey`（§4）。
4. **格式不一的痛点有四组实证**（§5）：F5 快照 request_id 双族分叉、plugin_prep_worker 防御性解析、frequencycleanup 别名归一、mixboard ref 前缀剥离。前两组各自烧掉过整张修复卡。

---

## 1. 数据面总事实：refs 是自由字符串

消费端类型定义（全部 `[]string`，无内嵌结构）：

- `agent/internal/agentprotocol/types.go:131`（LLM 提案）：`EvidenceRefs []string `+"`json:\"evidence_refs\"`"+``，且 `Validate()`（types.go:153）只查非空：`if len(p.EvidenceRefs) == 0 { return fmt.Errorf("evidence_refs are required") }`——**不校验格式**
- `agent/internal/agentprotocol/types.go:231`（PendingCandidate）：`EvidenceRefs []string `+"`json:\"evidence_refs,omitempty\"`"+``
- `agent/internal/audioclosure/types.go:187`（DAD Candidate）：`EvidenceRefs []string `+"`json:\"evidence_refs,omitempty\"`"+``
- `agent/internal/journal/journal.go:51`、`agent/internal/trajectory/types.go:101`、`agent/internal/pendingmanager/manager.go:150`、`agent/internal/agentloop/execution_memory.go:108/145/171`：全部同型透传
- MOM 是唯一把 refs 织进每个嵌套层的投影（`internal/mom/types.go`：ProjectStructure/Layers/TrustQuality/LLMContext/StaticLevelTrack/FrequencyRelationship/MaskingRelationship 共 9 处 `EvidenceRefs []string`）

LLM 生成侧的"格式规范"只是一句 prompt 占位符（`agent/internal/agentloop/ccb_model_prompt.go:109`）：

```
"evidence_refs":["<exact observation id>"]
```

——"exact observation id" 指什么格式，prompt 未定义，模型实际回流什么就靠下游防御解析兜底（§5 痛点②③）。

---

## 2. 生成点清单（31 处，8 类方案）

每条记录：字段结构（真实摘录）/ 生成时机 / 唯一性来源 / 不变性 / 可重拉性。

### A 类：投影内容 ID——`stableProjectionID` 四变体 + 三个无 ID 投影

**A1. DOM**（`agent/internal/dom/projection.go:530-537`）

```go
func stableProjectionID(projection Projection) string {
	copy := projection
	copy.ProjectionID = ""
	copy.GeneratedAt = ""
	copy.LLMContext = LLMContext{}
	data, _ := json.Marshal(copy)
	sum := sha256.Sum256(data)
	return "dom_" + hex.EncodeToString(sum[:])[:20]
}
```

- 生成时机：投影构建完成时；唯一性：全投影 JSON 内容哈希；不变性：**内容变 ID 必变，时间戳不参与**（GeneratedAt 置空）；可重拉性：无按 ID 注册表，重拉=重算哈希比对。

**A2. FXM**（`agent/internal/fxm/projection.go:207-213`）

```go
func stableProjectionID(value Projection) string {
	copy := value
	copy.ProjectionID = ""
	copy.LLMContext = LLMContext{}
	data, _ := json.Marshal(copy)
	sum := sha256.Sum256(data)
	return "fxm_" + hex.EncodeToString(sum[:])[:20]
}
```

- 与 DOM 的差异：**不置空 GeneratedAt**——同一内容两次生成（时间戳不同）得到**不同 ID**。A/B 对比投影因此天然是"实例身份"而非"内容身份"。

**A3. COM**（`agent/internal/com/projection.go:467-473`）：与 DOM 同构（置空 ProjectionID/LLMContext/GeneratedAt），`com_`+20 hex，内容身份。

**A4. RLM**（`agent/internal/rlm/projection.go:738-749`）

```go
seed := strings.Join([]string{
	proj.SchemaVersion, proj.SelectedMetric, proj.Mode,
	fmt.Sprintf("%d", proj.Summary.TrackCount),
	fmt.Sprintf("%d", proj.Summary.ActionCount),
	strings.Join(proj.EvidenceRefs, "|"),
	proj.GeneratedAt,
}, "\x00")
sum := sha256.Sum256([]byte(seed))
return "rlm_" + hex.EncodeToString(sum[:])[:16]
```

- 第三种方案：**字段拼接种子**（非全量 JSON），**GeneratedAt 参与种子**（实例身份），哈希取 **16** hex（其余三包是 20）。

**A5-A7. MOM/TIM/TOM/EPM：无自生成 ID**。身份完全靠外部传入的 `ObservationID`（`internal/mom/types.go:50`、`internal/tim/types.go:25`、`internal/tom/types.go:21`、`internal/epm/types.go:22`，均为 `json:"observation_id,omitempty"`）。内容变了 observation_id 不变——与 DOM/COM 的"内容变 ID 变"语义**相反**。

### B 类：观察/会话身份 ID

**B1. mixboard 观察票 ID**（`agent/internal/mixboard/mixboard.go:1262`）

```go
ObservationID: "obs_" + time.Now().UTC().Format("20060102T150405") + "_" + randomID(),
```

- 唯一性：时间戳+随机数；不变性：与内容**无关**（同内容两轮观察两个 ID）；可重拉性：**好**——落盘 `<sessionDir>/observations/<ID>.json`，`readObservationByID`（mixboard.go:3721）按 ID 读回。

**B2. audioclosure 观察指纹**（`agent/internal/audioclosure/driver.go:491-504`）

```go
func ObservationFingerprint(key ObservationKey) (string, error) {
	...
	digest := sha256.Sum256(raw)
	return "audio_observation:" + hex.EncodeToString(digest[:16]), nil
}
```

- 唯一性：`ObservationKey`（types.go:133-141：ProjectUUID/ProjectRevision/Scope.Kind+ID/TargetRef/ViewIDs/ObservationMode/Tap/**TimeWindow string**）规范化后哈希；用途是**幂等去重键**（driver.go:190 `if _, duplicate := state.Observations[fingerprint]; duplicate`），非内容指针；16 hex。这是全库最接近 D1 五元组的结构化键，但 TimeWindow 是自由字符串。

**B3. CCB 观察束 ID**（`agent/internal/capabilitycontext/free_state_observation.go:279`）

```go
BundleID: "ccbobs_" + compactID(observationID, req.RequestID),
```

- 第四种前缀；唯一性=两 ID 组合哈希；无独立注册表。

**B4. 派生关系包 ID**（`agent/internal/mixboard/catalog.go:356`）

```go
pkg["relationship_id"] = "rel_" + strings.ReplaceAll(obs.ObservationID, "obs_", "") + "_" + safePathName(deriveType)
```

- **字符串手术派生**：对父 ID 做前缀剥离再拼类型（痛点④，§5）。

**B5. capability pack ID**（`agent/internal/capabilitycontext/pack.go:108-113`）

```go
seed := strings.Join([]string{capabilityID, strings.TrimSpace(intent), strings.Join(refs, "|"), generatedAt.UTC().Format(time.RFC3339)}, "\x00")
sum := sha256.Sum256([]byte(seed))
return "cap_pack_" + hex.EncodeToString(sum[:])[:16]
```

- 字段拼接种子+16 hex（RLM 同族做法），generatedAt 参与种子；Pack 仅内存组装（pack.go 无落盘/读盘），重拉=按输入重建。

**B6. C2 处置计划 ID**（`agent/internal/chat/c2_dynamic_control_runtime.go:123`）：`"c2_plan_" + sanitizeCanaryID(sessionID)`——会话派生，非内容派生。

**B7. frequencycleanup 诊断/计划 ID**（`agent/internal/frequencycleanup/treatment.go:86,94`）：`stableID("fci", map[string]any{...})` / `stableID("fcp", ...)`——内容哈希族（含诊断 ID+条目+摘要）。

### C 类：URI 式 evidence ref 字面量（Go 侧）

MOM 专属构造器（`agent/internal/mom/evidence.go`）：

**C1. trackReadKey**（evidence.go:23-25）：`"mix.read:track." + safeKey(targetID(input)) + "." + suffix`
- 真实用例（projection.go:114）：`trackReadKey(input, "fast.levels")` → `mix.read:track.<trackID>.fast.levels`

**C2. acousticFeatureRef**（evidence.go:27-32）：`"acoustic_package_status:" + layer + "." + feature`
- 真实用例（projection.go:127）：`acousticFeatureRef("l3_deep", "band_energy_summary")` → `acoustic_package_status:l3_deep.band_energy_summary`

**C3. observationRef**（evidence.go:34-39）：`"observation:" + input.ObservationID` → `observation:obs_20260921T120000_ab12cd34`
- 注意：B1 的裸 ID 是 `obs_...`，这里又加 `observation:` 方案头——**同一对象两种写法**，下游 normalizeObservationRef（§5 痛点④）专门负责互换。

**C4. render probe ref**（`agent/internal/mom/projection.go:1012-1020`）：优先取行内 `evidence_ref`，缺失时回退 `"dad.l2_render_probe:" + renderRevision`。

**C5. RLM 数据源 ref**（`agent/internal/rlm/projection.go:130-132`）

```go
sourceRows(projectTrackRows(input.ProjectState), "project.state:tracks"),
sourceRows(mixTrackRows(input.MixObservation), "mix.observe:project_package.tracks"),
sourceRows(audioAnalysisRows(input.AudioAnalysisStatus), "project.audio_analysis_status:track_waveform_envelopes"),
```

- 又一族前缀：`project.state:` / `mix.observe:` / `project.audio_analysis_status:`；ref 粒度=整张表，无行级实例身份；行兜底键 `fmt.Sprintf("%s#%d", evidenceRef, i)`（projection.go:162）用**行序号**造键（顺序敏感，非稳定身份）。

**C6. project_package 族**（`agent/internal/mixboard/project_package.go`）：
- :348 `"dad.frequency_evidence:" + trackID + ":" + requestID`（track+request 双段）
- :685 `"project.state:track_level:" + trackID`
- :1240 `"project_package.project_band_occupancy." + bandID`（**点分方案，无冒号**）
- :1271/:1302 `"project_package.project_stereo_spread"`（**裸常量，无任何实例段**——跨工程跨轮次全部同名）

**C7. 无实例投影 ref**（TIM/TOM/EPM 硬编码）：
- `agent/internal/tim/projection.go:57`：`EvidenceRefs: []string{"mix.read:project.tracks.summary"}`；:83 追加 `"dad:track_waveform_envelopes"`（裸 scheme，连路径都没有）
- `agent/internal/tom/projection.go:232` 与 `agent/internal/epm/projection.go:89`：`evidenceRefs("import:track_refs", "tim:projection", "tom:projection", "dad:track_waveform_envelopes")`——`tim:projection` 指向**整个投影类**而非某次投影实例，D1 的 snapshot_id 维度在此完全缺失。

### D 类：内核 C++ 侧 evidence_ref（同名字段、独立生成的格式族）

**D1.** `VitApp/Source/Service/VitProductionCoordinator.cpp:146`（及 :753）：`"dad.l2_render_probe:" + request.renderRevision`
**D2.** `VitApp/Source/Service/VitProductionCoordinator.cpp:526`（及 :953）：`"dad.compressor_dual_tap:" + request.pairId`
**D3.** `VitApp/Source/Service/L3AcousticAnalyzer.cpp:465`：

```cpp
const auto evidenceRef = "dad.l3." + featureName + ":"
    + (request.sourceRevision.isNotEmpty() ? request.sourceRevision : request.filePath);
```

- **回退到文件路径**：无 revision 时 ref 含 `filePath`——路径不稳定（移动/重命名即失效），且跨 Windows/Unix 分隔符不定。
**D4.** `L3AcousticAnalyzer.cpp:573`：`"dad.l3.band_energy_summary:" + juce::String(band.definition.name)`——以**频段名**为实例段。
**D5.** `L3AcousticAnalyzer.cpp:587/610/634/660`：`evidence_refs = {"dad.l3.noise_floor"}` 等——**裸常量无实例段**（与 D3/D4 同文件混用两种粒度）。

### E 类：执行回执 refs

- `agent/internal/chat/b4_eq_runtime.go:618`：`EvidenceRefs: []string{spec.SourcePrefix + ".semantic_eq_batch:" + idempotencyKey}`（:525/:564/:678 加 `.reconcile` 变体）
- `agent/internal/chat/c2_dynamic_batch.go:686`：`EvidenceRefs: []string{"c2.dynamic_plugin_load_batch:" + key}`
- `agent/internal/harness/com_observation.go:99-103`：回执透传 `pair_id`/`evidence_ref`/`artifact_sha256`（含内容哈希，全库唯一把内容哈希与 ref 并排的写法）

### F 类：CCB view 键（披露层的"引用单位"）

`agent/internal/capabilitycontext/free_state_observation.go:421-462` 的 `freeStateViewDefinitions`：view_id（如 `project.structure`、`track.basic_energy`、`mix.masking_relationship`）+ 每视图绑定**数据键族**（如 `track.<targetID>.static.identity`、`observation.dom_projection`、`project.relationship_inputs`）。
- view 键体系（`<域>.<语义>` 点分）与 evidence ref 体系（`<scheme>:<路径>` 冒号）是**两套正交命名法**，靠装配代码手工对应（free_state_observation.go:265-283 从 binding 里捞 `observation_id`/`mix_session_id`）。

### G 类：快照 request_id 族（F5 痛点主角）

**G1.** `agent/internal/harness/harness.go:6848-6849`：

```go
requestID := firstNonEmpty(firstString(cmd, "mixboard_request_id", "request_id"), "mixboard_"+time.Now().UTC().Format("20060102T150405.000000000"))
```

**G2.** `agent/internal/harness/harness.go:4188-4200`：`kernelFeatureMaterializerRequestID` → `"kernel_prepared" + "_" + featureType + "_" + clipID`（段间下划线连接）。

两族 ID 写进**同一个**桥快照文件的 `latest_request`/行 `request_id`（§5 痛点①）。

---

## 3. 跨层不一致矩阵（验收线 ≥5，实际 7 组）

| # | 同类语义 | 端 A | 端 B | （端 C） |
|---|---|---|---|---|
| I1 | **投影身份方案** | DOM/COM：全量 JSON 哈希、时间戳置空、20 hex（dom/projection.go:530, com/projection.go:467） | FXM：同构但**时间戳参与**（fxm/projection.go:207） | RLM：字段拼接种子+时间戳参与+16 hex（rlm/projection.go:738）；MOM/TIM/TOM/EPM 干脆无 ID |
| I2 | **时间窗表达** | audioclosure：`TimeWindow string`（audioclosure/types.go:140） | FXM：`MeasurementWindow` 结构体（start_seconds/end_seconds 浮点，fxm/types.go:48-57） | mixboard：`TimeRuler`（DurationSeconds/SegmentSeconds/FrameSeconds，mixboard.go:1272 附近）；多数 ref 字符串里**根本无时间窗段** |
| I3 | **轨道引用嵌入** | MOM 嵌在 ref 路径中段：`mix.read:track.<id>.fast.levels`（mom/evidence.go:24） | project_package 嵌在尾部：`project.state:track_level:<trackID>`（project_package.go:685） | CCB view 键用独立键：`track.<targetID>.static.identity`（free_state_observation.go:429） |
| I4 | **实例段有无** | `dad.l3.<feature>:<revision或filePath>`（L3AcousticAnalyzer.cpp:465，有实例段） | `dad.l3.noise_floor`（:587，裸常量） | `tim:projection`/`tom:projection`（tom/projection.go:232，指整类投影） |
| I5 | **分隔符方案** | 冒号 scheme：`mix.read:`、`dad.l2_render_probe:` | 点分无冒号：`project_package.project_band_occupancy.<bandID>`（project_package.go:1240） | 裸 ID 加/不加方案头：`observation:<obs_id>`（mom/evidence.go:38）vs `<obs_id>`（mixboard 落盘名） |
| I6 | **snapshot 身份载体** | `request_id`（桥快照，harness.go:6849） | `render_revision`（probe ref 的实例段，mom/projection.go:1017 与 VitProductionCoordinator.cpp:146） | `observation_id`/`pair_id`/`PackID` 各自承担同类角色；全库**无 `snapshot_id` 字段名**（grep 零命中非测试代码） |
| I7 | **内容哈希有无** | 投影 ID=内容哈希（dom/fxm/com/rlm） | 观察票 ID=时间+随机（mixboard.go:1262），内容哈希仅出现在别处字段（`source_hash`/`artifact_sha256`，acousticpackage/status.go:37、com_observation.go:100） | capability pack ID 含 refs 但不含投影内容（pack.go:108） |

---

## 4. D1 五元组缺位对照

| 五元组字段 | 现状最接近物 | 缺位情况 |
|---|---|---|
| projection_kind | ref 前缀隐含（`dad.l3.` / `dom_` / `rlm_`）但**仅 4 包有**；字符串内无标准字段 | MOM/TIM/TOM/EPM 无任何 kind 段 |
| scope_id | 轨道 ID 嵌在字符串三种位置（矩阵 I3）；audioclosure `Scope.Kind+ID` 是唯一结构化实现（types.go:125-129） | 无统一位置/命名 |
| time_window | 三种结构+一种字符串（矩阵 I2） | 绝大多数 ref 无时间窗段——同 track 不同窗的观察共享同一 ref 字面量 |
| snapshot_id | 由 request_id / render_revision / observation_id / pair_id 分头扮演 | 字段名 `snapshot_id` 全库非测试代码零命中 |
| content_hash | 投影 ID 即哈希（4 包）；`artifact_sha256`（com_observation.go:100）、`source_hash`（acousticpackage/status.go:37）散落 | ref 字符串本身从不携带内容哈希；`obs_` 族 ID 与内容完全解耦 |

结构化程度最高的是 `audioclosure.ObservationKey`（audioclosure/types.go:133-141）——它已具备 project/scope/target/views/tap/time_window 六段，但 TimeWindow 是自由 string，且该键只活在闭包驱动内部，没有外溢为跨层引用格式。

---

## 5. 痛点实证回溯（验收线 ≥2，实际 4 组）

**痛点①：F5 快照新鲜度——同快照双 request_id 族分叉（烧掉整张 FIX-F5-SNAPSHOT-FRESHNESS 卡）**

- 事实链（`coord/cards/done/2026-09-21-FIX-F5-SNAPSHOT-FRESHNESS-freshness-bell.md` 定位报告节）：路径 B 用 `mixboard_<UTC纳秒>`（harness.go:6849）戳记、路径 A 用 `kernel_prepared_<feature>_<clip>`（harness.go:4188）材化，两族 ID 无条件覆盖同一 `latest_request`，且 `normalizeMixboardBridgeRowForPacket` 写时把 kernel 族旧行 request_id "过继"洗成新 packet id——断言两次撞上 `band_energy_summary.request_id=mixboard_时间戳 vs latest_request=kernel_prepared_*` 分叉。
- 代码锚点：`agent/internal/harness/harness.go:6848-6849`（mixboard 族）、`harness.go:4188-4200`（kernel_prepared 族）、`harness.go:7337`（`strings.HasPrefix(rowRequestID, "kernel_prepared_")` ——下游**靠字符串前缀判族**，格式即协议）。
- 根因归属：request_id 承担 snapshot_id 职责却无统一 schema，跨写者不可比。

**痛点②：plugin_prep_worker 防御性 ref 解析（格式不可预测的兜底税）**

- `agent/internal/chat/plugin_prep_worker.go:1328-1356` `pluginPrepWorkerNormalizeObservationID`：依次剥离 `observation:`、`observation_id:`、截取 `obs_` 起始、剥引号/逗号/分号/空白、剥 `.json` 后缀，最后**非 `obs_` 前缀直接判空丢弃**。
- 这段代码的存在本身即实证：模型/上游回流的 observation 引用至少以 5 种形态到达（带 scheme 头/带 id 头/裸 ID/文件名/混入标点），且与 C3（`observation:` 加头）正面冲突——MOM 披露时加头，消费端再剥头。

**痛点③：frequencycleanup 证据别名归一（`.`/`_` 分隔符漂移）**

- `agent/internal/frequencycleanup/treatment.go:100-129`：`resolveSuppliedEvidenceRef` + `evidenceRefAliasKey` 把最后一个 `:` 之后的 `.` 和 `_` 统一折叠为 `~` 再做别名匹配，匹配成功后**回写 canonical 形态**（treatment.go:66-72）。
- 实证：模型回流 `mix.read:track_x.fast_levels` 会被宽容接受并改写成 `mix.read:track_x.fast.levels`——披露格式与回流格式的分隔符不统一已进入"必须写归一器"的阶段。

**痛点④：mixboard ref 前缀剥离与字符串手术派生**

- `agent/internal/mixboard/mixboard.go:3699-3712` `normalizeObservationRef`：剥 `observation:` / `observation_id:` / `.json`——与痛点②同款防御逻辑在第二个消费方重复出现（两处独立实现，行为近似但不共享）。
- `agent/internal/mixboard/catalog.go:356`：`"rel_" + strings.ReplaceAll(obs.ObservationID, "obs_", "") + "_" + deriveType`——用 `ReplaceAll` 全串替换前缀来派生关系包 ID，父 ID 格式一旦调整（比如换前缀）即静默产出双前缀畸形 ID。

---

## 6. 盘点边界与未覆盖项

- **Godot 前端仓（D:\Godot\project，仓库外）未盘点**：FIX-F5 卡已证实前端 `telemetry_manager.gd` 是 `kernel_prepared_waveform_envelope_<clip>` 铃的第三写者，其 ref/request_id 生成格式属 mac/前端域，本卡（PC 域只读）未跨仓取证。
- **PCA 层**（processorattestation/pluginsemantics）只按目录名扫过 EvidenceRefs 命中，未逐文件展开；其准入证据链若复用 `obs_`/`capability-pack:` 方案则已被本报告覆盖，若有独立方案需 G1 前补一小节（预计 ≤0.5h）。
- `harness.go` 桥快照行字段白名单（mixboard.go:2893 一行 140+ 键）里 `request_id`/`evidence_ref`/`render_revision` 并存于同一行——行级"哪个字段才是权威身份"未在本次判定，属 G1 评审议题而非勘察缺口。
- 样例行号以 HEAD `911e3312` 为准；harness.go 行号在 FIX-F5 卡记录（4104/6763）与当前 HEAD（4188/6848）间已有漂移，引用时以本报告为准。

## 7. 验收对照

| 卡面验收项 | 状态 |
|---|---|
| ① 矩阵覆盖全部指定包（九投影+DAD 两包+CCB+消费方），生成点 ≥15 逐一带样例 | ✅ dom/mom/tim/tom/fxm/com/epm/rlm/acousticpackage + audioclosure/frequencycleanup + contextruntime(CCB view) + mixboard/journal/trajectory/pendingmanager/agentprotocol 全覆盖；生成点 31 处（§2 A-G 类），每类带真实代码摘录 |
| ② 不一致清单 ≥5 条，每条两端锚点 | ✅ 7 组（§3 I1-I7），每条 ≥2 端文件:行 |
| ③ 痛点实证 ≥2 例 | ✅ 4 组（§5），含 1 组整卡级事故回溯 |
| ④ 报告入库 coord/runs | ✅ 本文件 `coord/runs/L1-1-RECON-1/EVIDENCE_REFS_INVENTORY.md` |
| 零代码改动 | ✅ 仅新增本报告文件；工作树无源码改动 |
