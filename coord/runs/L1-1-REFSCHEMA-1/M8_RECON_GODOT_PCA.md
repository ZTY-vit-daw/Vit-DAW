# M8 勘察补腿：Godot 前端写者 + PCA 层（L1-1 迁移计划 M8 项）

- 任务来源：[G1 终审记录 §4 M8](G1_FINAL_REVIEW.md)；背景盘点 [EVIDENCE_REFS_INVENTORY.md](../L1-1-RECON-1/EVIDENCE_REFS_INVENTORY.md) §6 两处未覆盖。
- 性质：只读勘察，零代码改动；本报告为唯一写入物。
- 日期：2026-10-07 晚（闲时任务）；注册表基线=main fbd243d8 时的 `agent/internal/agentprotocol/refschema.go`——**15 条** legacy 前缀（盘点时 14 条+REFSCHEMA-M1 当日补入 `observation:`）。
- 勘察域：`D:\Godot\project\vit-daw-frontend`（206 个 .gd）+ `agent/internal/processorattestation`（10 文件）+ `agent/internal/pluginsemantics`（2 文件）。行号以本次实读为准。

---

## 0. 执行摘要

1. **前端是两条已注册族的活跃写者**：`kernel_prepared_` 第三写者（FIX-F5 结论复核成立）与 `mixboard_` 第二写者（**新事实**：盘点 G1 只记了 agent 侧 harness.go 一处；前端 mix_client.gd 也产 `mixboard_<msec>` 并写进同一桥快照行族）。两者均命中注册表=legacy 可翻译，无需补条。
2. **前端另发现三个未注册 IPC 请求 id 前缀**（`ipc_` / `library_plugin_list_` / 裸微秒时间戳）——均属前端内部命令信封域，不进 EvidenceRefs 与 mixboard 桥快照白名单行，建议**登记不动作**。
3. **PCA 层比盘点预期更干净**：两包零 `EvidenceRefs []string` 写点。processorattestation 用**结构化** `EvidenceRef{ReceiptID,Kind,SHA256,ObservedAt,CorpusRecord}`（非字符串拼接），ReceiptID 值逐字复制自上游 CCB 审计回执/观察 id；pluginsemantics 的 `Evidence` 是纯评分字段族无 ref 字符串。
4. **衍生发现一个未注册身份族**：`ccbr_` / `ccbr_rejected_`（CCB 审计回执 receipt_id，capabilitycontext 域——非本卡域，如实登记移交）。

---

## 1. 任务 A 清单：Godot 前端写者

### A-1. `kernel_prepared_waveform_envelope_<clip>` —— G 类，命中 `kernel_prepared_` ✅

- **锚点**：`app/kernel/autoloads/telemetry_manager.gd:1963`
- **真实摘录**：`var request_id := "kernel_prepared_waveform_envelope_%s" % clip_id.strip_edges()`
- **生成时机**：前端收到内核 `audio_feature_data_ready` 遥测、代内核物化波形包络特征时（packet 含 `status:"materialized"` / `lifecycle:"kernel_prepared_materializer"` / `kernel_event_cmd:"audio_feature_data_ready"`）。
- **唯一性来源**：clip_id（拓扑身份）——非时间戳非随机；同 clip 重复物化同 id。
- **归类**：G 类快照 request_id 族；注册表 `kernel_prepared_`（slot=snapshot）命中，legacy 可翻译。
- **同文件邻域**（:1033/:1034/:1967/:1968/:2513）：`kernel_prepared_telemetry` / `kernel_prepared_l3_telemetry` / `kernel_prepared_materializer` 等是行字段 `source_kind`/`materialized_by`/`lifecycle` 的**判别常量值**（agent 侧 harness.go:7337 前缀判族的兄弟面），不是 request_id 本身——登记不计入生成点。

### A-2. `mixboard_<msec>`（含每特征变体 `mixboard_<msec>_<feature>`）—— G 类，命中 `mixboard_` ✅【新事实】

- **锚点**：`app/kernel/clients/mix_client.gd:36-40`（族 id）与 `:66`（每特征变体）
- **真实摘录**：
  ```gdscript
  func request_observation_features(..., request_id_prefix: String = "mixboard") -> Dictionary:
      var request_id := request_id_prefix.strip_edges()
      if request_id.is_empty(): request_id = "mixboard"
      request_id += "_" + str(Time.get_ticks_msec())
      ...
      var feature_request_id := request_id + "_" + feature
  ```
- **生成时机**：前端发起 mixboard 观察特征请求（`mixboard_request_observation_features` 命令经 waveform client 送内核，`mixboard_request_id` 随信封入内核侧行族）。
- **唯一性来源**：毫秒时间戳（+feature 名）；同毫秒两请求同 id（弱唯一性，与 agent 侧 harness.go:6849 的微秒族相比粒度更粗——如实记录）。
- **归类**：G 类；注册表 `mixboard_`（slot=snapshot）命中。
- **意义**：F5 痛点①的"双族分叉"实为**三写者两族**局面——`mixboard_` 族有 agent（harness.go:6849）与前端（mix_client.gd:39）两个写者写同一桥快照 `latest_request` 行。**M6 迁移卡的范围必须同时覆盖两仓两个文件**（ruling #7 的 GDScript 常量导出钩子落点即此）。

### A-3. 前端内部 IPC 请求 id（未注册，不进证据面）—— 登记不动作

| 前缀 | 锚点 | 摘录 | 域 |
|---|---|---|---|
| `ipc_<seq>` | `app/kernel/autoloads/vit_ipc_client.gd:91` | `return "ipc_%d" % _request_seq` | 前端→内核命令信封 request_id（单调序列，会话内唯一） |
| `library_plugin_list_<usec>` | `app/browser/left_library_dock.gd:248` | `"library_plugin_list_%d" % Time.get_ticks_usec()` | 浏览器库面板的 settings 客户端请求 |
| 裸 `<usec>` | `app/browser/cef_browser_view.gd:115` | `var request_id := "%d" % Time.get_ticks_usec()` | CEF 页面捕获的一次性配对 id（仅前端内部 echo 匹配） |

判定：三者是 IPC 信封 id，消费闭环在前端/内核命令层，grep 未见流入 `EvidenceRefs`、mixboard 桥快照白名单行或观察落盘名——**opaque 候选都不是**（不构成 evidence ref），无动作。

### A-4. 其余扫描面（零命中，如实记录）

- `vit://` 前端零出现；`evidence` 词在 .gd 中零出现（除变量名无关项）。
- `obs_` / `acoustic_package_status` / `dad.` / `audio_observation` / `fci_` / `fcp_` 等已注册族：前端零生成点（命中均为变量名/快照文件路径常量 `mixboard_feature_snapshot.json` 类，非 ref 字面量）。
- `tools/diagnostics/mixboard_snapshot_freshness_annotation_probe.gd:19`：`BELL_ID := "kernel_prepared_waveform_envelope_probe_clip_a"`——诊断 fixture 常量（前缀命中 `kernel_prepared_`），F5 链测试工具，非生产写者。

---

## 2. 任务 B 清单：PCA 层

### B-1. processorattestation——结构化 EvidenceRef，零字符串拼接

- **锚点**：`agent/internal/processorattestation/types.go:67-73`
  ```go
  type EvidenceRef struct {
      ReceiptID    string    `json:"receipt_id"`
      Kind         string
      SHA256       string
      ObservedAt   time.Time
      CorpusRecord string    `json:"corpus_record,omitempty"`
  }
  ```
- **与全库 `EvidenceRefs []string` 面的关系**：**不是同一物**——这是 PCA 准入证书（Attestation.Evidence []EvidenceRef）的结构化证据字段，四字段必填校验（types.go:242-243 receipt_id+kind+sha256+observed_at），dedupe 键=`ReceiptID+"\x00"+SHA256`（types.go:322）。v2 同构（v2.go:342-343）。
- **值的来源**：ReceiptID 逐字复制上游——chat/c2 动态控制链的指令面明文要求"copy them verbatim from the target CCB bundle observation_id, evidence_refs, or audit_receipt.receipt_id"（c2_dynamic_control_runtime.go:209 prompt）；pcactl 的 `-receipt` 导入路径收 processorauthority 强回执摘要文件。即 PCA 层是**证据 id 的消费/持有者，非生成者**。
- **归类**：无新前缀；身份复用上游族（observation_id→`obs_`、audit_receipt→下条）。

### B-2. 衍生发现：`ccbr_` / `ccbr_rejected_`—— CCB 审计回执族（capabilitycontext 域，未注册）

- **锚点**（勘察 PCA 消费链时撞见，域外如实登记）：`agent/internal/capabilitycontext/free_state_observation.go:567`（`"ccbr_" + compactID(observationID, req.RequestID)`）与 `:439`（`"ccbr_rejected_" + compactID(...)`）。
- **唯一性来源**：compactID（观察 id+请求 id 的sanitize 哈希短串，:1362-1375，80 字符截断）；身份族（会话内配对）。
- **归类**：B 类观察/会话身份族（与已盘点 B3 `ccbobs_` 同族邻域）；**未注册**→ 若入 EvidenceRefs 面则落 opaque。
- **建议**：随 M4（B 类身份族机会迁移）卡面登记处置；本腿只登记不越域动作。

### B-3. pluginsemantics——纯评分字段族，零 ref 面

- **锚点**：`agent/internal/pluginsemantics/index.go:39/49/173-179`——`Evidence []Evidence{Kind, Value, Type, Score, Message}`：语义索引的证据评分行，无 id/ref 字符串拼接，无 EvidenceRefs 写点。
- **归类**：不属八类任何一类（非 ref 生成面）。

---

## 3. 新发现前缀汇总表

| 前缀 | 域 | 类 | 注册表命中 | 建议 |
|---|---|---|---|---|
| `kernel_prepared_waveform_envelope_` | Godot telemetry_manager.gd:1963 | G | ✅ `kernel_prepared_` | 无需补条；**纳入 M6 迁移范围**（前端侧写者） |
| `mixboard_`（+特征变体） | Godot mix_client.gd:39/:66 | G | ✅ `mixboard_` | 无需补条；**纳入 M6 迁移范围**（前端第二写者，ruling #7 常量导出落点） |
| `ipc_` | 前端 vit_ipc_client.gd:91 | IPC 信封 | —（不进证据面） | 登记不动作 |
| `library_plugin_list_` | 前端 left_library_dock.gd:248 | IPC 信封 | —（不进证据面） | 登记不动作 |
| `<usec>` 裸值 | 前端 cef_browser_view.gd:115 | IPC 配对 | —（不进证据面） | 登记不动作 |
| `ccbr_` / `ccbr_rejected_` | agent capabilitycontext:567/:439（域外衍生） | B 类身份 | ❌ 未注册 | 随 M4 登记处置；若需补条建议字段：`{Family: observation_fingerprint 族邻域（B 类会话身份）, Slot: snapshot, TargetKind: ""}`（与 ccbobs_/ccbr 同批定承载，避免单条先定） |

## 4. 与迁移计划 M1-M8 的衔接建议

1. **M6（G 类+BELL/F5 链触发）范围修正**：卡面须同时列 agent 侧 harness.go 两写者与 Godot 侧 telemetry_manager.gd/mix_client.gd 两写者（本腿坐实的三写者两族全貌）；常量导出（ruling #7）= agent `agentprotocol` 常量 + Godot 侧镜像常量（前端无法 import Go 包，导出形态=字符串常量+注释指回权威源，参照内核 RefSchema.h 先例）。
2. **M4（B 类机会迁移）追加登记项**：`ccbr_`/`ccbr_rejected_`（§3 末行建议字段）与盘点既有 `ccbobs_`/`cap_pack_` 同批。
3. **无注册表补条来自本腿两域本身**——A/B 两域的新前缀要么命中要么不进证据面；REFSCHEMA-M1（`observation:`）之后注册表 15 条对本腿发现面已足够。
4. **弱唯一性观察**（供 M6 迁移时顺手评估，不单独立卡）：前端 `mixboard_<msec>` 毫秒粒度同撞可能（agent 侧为微秒+纳秒混合）；迁移 vit:// 时 snapshot 段语义统一后此差异自然消解（时间戳身份→snapshot 段，规范化由迁移卡定）。

---

*勘察边界：两包+前端仓的 ref 生成点；行号以 2026-10-07 晚实读为准（HEAD fbd243d8 / 前端仓现状未取 git 状态——仓库外工程只读扫描）。发现与盘点 §6 预期不符处（PCA 层更干净、mixboard 前端写者系新事实）均如实记录。*
