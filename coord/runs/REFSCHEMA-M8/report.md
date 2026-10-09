# REFSCHEMA-M8：Godot 写者 + PCA 层 refs 勘察报告（只读，零代码）

- 卡：`coord/cards/done/2026-10-09-REFSCHEMA-M8.md`（池序 33，G1 终审 §4 M8 行）
- 勘察时间：2026-10-09 晚；agent 仓 HEAD=`afddcea3`（领取 commit，基于 origin/main 45ae7591）；前端仓 `D:\Godot\project\vit-daw-frontend` 仓库外只读扫描（未取 git 状态）。
- 注册表基线：`agent/internal/agentprotocol/refschema.go:130-159` 实读——**15 条** legacy 前缀（M1 `observation:` 后无新增；M2X 迁移生成点不改注册表）。
- **与既有产出的关系**：10-07 闲时任务已产出 [M8_RECON_GODOT_PCA.md](../L1-1-REFSCHEMA-1/M8_RECON_GODOT_PCA.md)（覆盖 processorattestation + pluginsemantics + 前端两族）。本报告**复核其锚点（两日后行号漂移实测）并补齐本卡点名的 processorauthority / processorregistry 两包**——该两包正是旧产出未覆盖面，且本腿在 processorauthority 发现了**真实生成点**（`pcr1_`，与旧产出"PCA 层非生成者"的结论在包级精度上需要修正）。

## 0. 执行摘要

1. **前端证据面写者维持两族两写者**（旧勘察结论复核成立，行号小幅漂移）：`kernel_prepared_waveform_envelope_<clip>`（telemetry_manager.gd:1963，锚点未漂移）与 `mixboard_<msec>`（mix_client.gd，id 构造从 :36-40/:66 漂到 **:39-42/:65**）。均命中注册表=legacy 可翻译。
2. **前端 IPC 信封域扩大**（10-07 后新增或旧勘察未及）：`vsp_transport_`、`track_lane_2d_`、`remove_clips_`、`startup_probe_` 四个生产前缀 + `gui_save_probe_`/`gui_vsp_probe_` 两个诊断前缀——**agent 仓零消费**（grep 实证，附录 A4），不进证据面，登记不动作。
3. **前端 `vit://` 字面量不再是零**：3 处（gui_architecture_audit_probe.gd:355-356 合成审计 fixture、browser_panel.gd:614 WebView 测试地址）——非 refschema 面；但 M6 迁移后前端 `vit://` grep 会命中它们，**验收断言需排除这两文件**（前瞻登记）。
4. **PCA 三包中 processorauthority 是生成者**（本卡点名包、旧产出未覆盖）：`evidenceRef()` 产 `pcr1_<sha256[:24]>` ReceiptID（内容寻址），三个调用点。**processorregistry 零 ref 面**（纯数据表）。processorattestation 维持"结构化持有者"复核成立。
5. **agent 侧域外衍生三个新发现**（盘点 §6 与旧 M8 产出均零覆盖）：`mixboard_l2_render_probe_<track>_<ts>`（harness.go:3609，注册表命中的第三 mixboard_ 写者）；`mix-report:<uuid>:<hash>` 与 `mixboard-project:<uuid>`（harness.go:4548-4549 + chat/mixboard_decision_projection.go:100，ref 形态响应字段、无下游读取者、未注册）；注册表 `mixboard_` 条目 Anchor 已漂移（harness.go:6848-6849 → 实际写者在 :6923）。

---

## 1. 清单一：Godot 前端写者族（四栏）

### 1a. 证据面写者（命中注册表）

| # | 前缀族 | 写出处 | 形态样例 | 消费方 | vit:// 对应物 |
|---|---|---|---|---|---|
| F1 | `kernel_prepared_waveform_envelope_<clip>` | `app/kernel/autoloads/telemetry_manager.gd:1963` | `"kernel_prepared_waveform_envelope_%s" % clip_id.strip_edges()` | 前端代内核物化波形包络（`audio_feature_data_ready` 遥测行，status=materialized）→ 桥快照行族 → agent harness 消费面 `SnapshotRow`（harness.go:4067/:4093/:4433/:4446）与 `latest_request`（harness.go:3760/:3786/:6110） | 无（legacy；注册表 `kernel_prepared_` slot=snapshot 可翻译）。唯一性=clip_id 拓扑身份，同 clip 重复物化同 id |
| F2 | `mixboard_<msec>`（+特征变体 `mixboard_<msec>_<feature>`） | `app/kernel/clients/mix_client.gd:39-42`（族 id）、`:65`（特征变体）、`:71`（`mixboard_request_id` 信封字段） | `request_id = "mixboard" ; request_id += "_" + str(Time.get_ticks_msec())` | 前端 mixboard 观察特征请求 → 内核命令 `mixboard_request_observation_features` → agent harness.go:4395/:4943/:5071 读 `mixboard_request_id` → 桥快照 `latest_request` 行族 | 无（legacy；注册表 `mixboard_` 命中）。**弱唯一性**：毫秒粒度，同毫秒两请求同 id（agent 侧为纳秒时间戳格式 20060102T150405.000000000） |

判族常量（非 id 生成，登记不计入）：telemetry_manager.gd:1033/:1967-1968/:2513 的 `kernel_prepared_telemetry`/`kernel_prepared_materializer` 等是 `source_kind`/`lifecycle` 字段值，与 agent 侧 harness.go:4074/:4101/:4127/:4192-4193 的判族面互为镜像。

### 1b. IPC 信封域（不进证据面；agent 仓消费=零命中，附录 A4 实证）

| 前缀 | 写出处 | 形态 | 域 |
|---|---|---|---|
| `ipc_<seq>` | `app/kernel/autoloads/vit_ipc_client.gd:91` | `"ipc_%d" % _request_seq` | 前端→内核命令信封（单调序列） |
| `library_plugin_list_<usec>` | `app/browser/left_library_dock.gd:248` | `"library_plugin_list_%d" % Time.get_ticks_usec()` | 库面板 settings 请求 |
| 裸 `<usec>` | `app/browser/cef_browser_view.gd:115` | `"%d" % Time.get_ticks_usec()` | CEF 页面捕获一次性配对 |
| `vsp_transport_<usec>_<seq>` | `app/kernel/clients/transport_client.gd:106` | `"vsp_transport_%d_%d" % [usec, seq]` | VSP transport 命令信封 + 前端 inflight 表 |
| `track_lane_2d_<clip>_<msec>` | `app/tracks/track_scene/vit_track_lane_2d.gd:2995` | timeline store pending-commit 追踪 id | 前端 pending 面板域 |
| `remove_clips_<clip>` | `app/rack/Vit_Graph_Rack.gd:4790` | rack 队列请求 id | 前端 rack 命令队列 |
| `startup_probe_<msec>` | `app/startup/start_page.gd:566` | UDP ping payload | 启动连通性探测 |

### 1c. 诊断工具域（tools/diagnostics，非生产写者）

- `gui_save_probe_<msec>`（gui_project_save_matrix_probe.gd:276）、`gui_vsp_probe_<seq>`（vsp_gui_live_probe.gd:40）、fixture 常量 `kernel_prepared_waveform_envelope_probe_clip_a`（mixboard_snapshot_freshness_annotation_probe.gd:19，BELL/F5 链测试工具）。

### 1d. `vit://` 字面量面（非 ref 写者，10-07 后新增）

- `tools/diagnostics/gui_architecture_audit_probe.gd:355-356`：`"vit://audit/synthetic/%s/%03d"`——GUI 架构审计探针的合成插件路径 fixture。
- `app/browser/browser_panel.gd:614`：`"vit://embedded-webview-test"`——内嵌 WebView 地址栏测试默认值。
- 判定：均为合成/展示字面量，不构成 refschema ref；但见 §0 第 3 条的验收前瞻。

---

## 2. 清单二：agent 侧 PCA 三包 evidence refs 生成点（四栏）

### 2a. processorregistry —— 零 ref 面（本卡点名包，旧产出未覆盖）

- **写出处**：无生成点。`registry.go:15-29` `Definition{Family, PCAFamily, Recognizer, CoverageVocabulary, CoverageProofs, Planner, Materializer, TypedExecutor, ReceiptProjector, ObservationViews, InspectOnly}`——纯数据注册表（包注释 registry.go:1-3 自声明 data-only）。
- **与证据的关系**：`CoverageProofs map[string][]processorattestation.Coverage`（registry.go:22）引用 Coverage（action/shape/axis），**无 id、无 ref 字符串**。
- **归类**：非 ref 生成面。

### 2b. processorattestation —— 结构化持有者（旧勘察复核成立，锚点更新）

| 项 | 锚点（本次实读） | 说明 |
|---|---|---|
| 结构定义 | `types.go:67-73` | `EvidenceRef{ReceiptID, Kind, SHA256, ObservedAt, CorpusRecord}`——**非字符串拼接**，与全库 `EvidenceRefs []string` 面不同物 |
| 四字段必填校验 | `types.go:238-243` | receipt_id+kind+sha256+observed_at |
| dedupe 键 | `types.go:322-326` | `ReceiptID + "\x00" + SHA256` |
| v2 同构 | v2.go（IssueSpecV2.Evidence 同类型） | |
| ReceiptID 值来源 | 上游 CCB 审计回执/观察 id 逐字复制 + processorauthority 注入（下条） | 消费/持有者；身份复用上游族（obs_/ccbr_ 邻域） |

### 2c. processorauthority —— **生成者**（本卡点名包，旧产出未覆盖；本腿主发现）

| # | 生成点 | 形态样例 | 消费方 | vit:// 对应物 |
|---|---|---|---|---|
| P1 | `receipts.go:349-354` `evidenceRef(path, kind, raw, completedAt)` | `ReceiptID: "pcr1_" + hex(sha256(raw))[:24]`；`Kind`=schema 版本小写；`SHA256`="sha256:"+全量 hex；`CorpusRecord`=回执文件路径 | 调用点三处：`receipts.go:154`（ReadReceipt 主回执，EQ/compressor 各 schema 分支）、`receipts.go:311`（findCaseEvidence，kind=`processor_control_case_evidence.v1`）、`receipts_v2.go:53`（V2 路径）→ `Candidate/CandidateV2.Spec.Evidence` → attestation store（`~/.vit/processor_control_attestations.v1/.v2.json`）→ 消费端：`cmd/pcactl`（`-receipt` 导入 main.go:132/143 + store/query 子命令）与 `internal/chat/processor_certification_entry.go`（HTTP `/agent/processor-certification/candidates|start|status`，chat/server.go:593-595） | 无。未注册（15 条注册表无 `pcr1_`）。**内容寻址**（digest 前 24hex）→ 若未来入字符串面属 hash slot 族语义；当前结构化域内闭环、未流入 `EvidenceRefs []string`，**暂不落 opaque** |

包内其余文件（local_certification.go / local_processor_certification.go）：仅构造 `cases/01_<caseID>` 目录名（:86/:82），非 ref 前缀，登记不计入。

### 2d. 域外衍生（勘察 PCA 消费链时撞见，agent 侧 harness/chat 域——如实登记移交）

| # | 前缀 | 锚点 | 形态 | 消费方 | 注册表 |
|---|---|---|---|---|---|
| D1 | `mixboard_l2_render_probe_<track>_<ts>` | `harness/harness.go:3609` | `"mixboard_l2_render_probe_"+safeRequestIDPart(trackID)+"_"+UTC 纳秒时间戳` | mixboard 特征请求包 request_id（lifecycle=l2_render_probe_ab）→ 桥快照行族 | ✅ 命中 `mixboard_`（前缀包含）。**agent 侧第二 mixboard_ 写者**——盘点 G1 与旧 M8 产出均未记 |
| D2 | `kernel_prepared_<feature>_<clip>` | `harness.go:4262-4278` `kernelFeatureMaterializerRequestID`（G2 锚点 4188-4200 的 id 源，packet 函数仍在原位） | `strings.Join(["kernel_prepared", featureType, clipID], "_")` | 内核遥测事件代物化包 → SnapshotRow（:4067） | ✅ 命中 `kernel_prepared_`。前端 :1963 的 agent 侧兄弟面（盘点 G2 已记，锚点复核成立） |
| D3 | `mix-report:<uuid>:<hash>` / `mixboard-project:<uuid>` | `harness.go:4548-4549`（report_ref / mixboard_decision_board_ref）、`chat/mixboard_decision_projection.go:100` | scheme-URI 形态响应字段 | **无下游读取者**（全库 grep：三写点之外零消费；webui 零命中）——chat 响应 JSON → UI/日志显示面 | ❌ 未注册（`mixboard-project:` 连字符形态与 `mixboard_` 下划线不匹配，最长匹配不劫持）。盘点 §6 零覆盖，新发现 |
| D4 | `ccbr_` / `ccbr_rejected_` | `capabilitycontext/free_state_observation.go:567/:439` | compactID（观察 id+请求 id 哈希短串） | CCB 审计回执域 | ❌ 未注册（旧 M8 产出已登记，维持移交 M4） |

**注册表锚点漂移登记**：`mixboard_` 条目 Anchor="harness.go:6848-6849"（refschema.go:135）——该位置现为 wait 时长函数；经典写者实际在 **harness.go:6923**（`"mixboard_"+time.Now().UTC().Format(...)`，兜底分支）。语义未变，锚点失准。

---

## 3. 迁移建议表（归入 G1 终审 M4-M7 或新行；只建议不实现）

| # | 族 | 归行建议 | 预计触碰面与风险 |
|---|---|---|---|
| R1 | 前端 `mixboard_` 写者（mix_client.gd:39-42/:65） | **M6**（G 类，维持旧勘察建议；行号按本报告更新） | M6 卡范围须含 agent+Godot 两仓；GDScript 常量导出（ruling #7）落点即此；毫秒弱唯一性在 snapshot 段语义统一后自然消解 |
| R2 | 前端 `kernel_prepared_` 写者（telemetry_manager.gd:1963） | **M6**（G 类） | 同上；与 agent 侧写者（R3/D2）同批，避免迁移后同族双形态并存窗口拉长 |
| R3 | agent `mixboard_l2_render_probe_`（harness.go:3609）+ 经典写者（:6923） | **M6**（G 类，M6 范围追加：agent 侧共两写者） | M6 立卡时按本报告新锚点；**顺手动作建议**：更新注册表 `mixboard_` 条目 Anchor（6848-6849→6923，纯注释字段，零行为） |
| R4 | `pcr1_`（processorauthority） | **新行建议 M9（登记观察项，非迁移）**：现状结构化闭环无字符串面，不急于注册翻译条目；触发条件=PCA receipt_projector 投影面未来把 Evidence 拼成 refs 字符串 | 若触发注册，建议字段 `{LegacyPrefix:"pcr1_", Family:RefFamilyObservationFingerprint 邻域（内容寻址）, TargetKind:"", Slot:RefSlotHash}`。低优先、零当前行为 |
| R5 | `mix-report:` / `mixboard-project:`（harness.go:4548-4549 + chat:100） | **M5**（E 类回执族，机会迁移）：纯响应面 ref 形态字段、无下游读取者 | 不动也无 opaque 积累（不进 EvidenceRefs 面）；若 chat 面长期保留，建议 M5 时一并定去留（注册翻译条目或改字段名消歧 `mixboard-project:` 与 `mixboard_` 的视觉近似） |
| R6 | `ccbr_` / `ccbr_rejected_`（capabilitycontext） | **M4**（B 类身份族，维持旧勘察建议） | 与 ccbobs_/cap_pack_ 同批定承载，避免单条先定 |
| R7 | 前端 IPC 信封 7 前缀 + 诊断工具域 3 项（§1b/§1c） | **登记不动作**（不构成 evidence ref） | 无 |
| R8 | 前端 `vit://` 字面量 3 处（§1d） | **登记不动作**，但作为 M6 验收前瞻：迁移后前端 `vit://` grep 断言须排除 gui_architecture_audit_probe.gd 与 browser_panel.gd | 低风险，防 M6 验收误判 |

---

## 附录：可复跑 grep 命令（Git Bash，2026-10-09 实跑口径）

```bash
# A1. 前端证据面两族写者（写出处+判族常量）
cd /d/Godot/project/vit-daw-frontend && rg -n --glob '*.gd' 'mixboard_|kernel_prepared_' app tools

# A2. 前端 request_id 构造全量扫（新写者发现面）
cd /d/Godot/project/vit-daw-frontend && rg -n --glob '*.gd' 'request_id ?[:=]=? ?.*("%|_\" ?\+|%s|%d|get_ticks)' app tools

# A3. 前端 vit:// 字面量
cd /d/Godot/project/vit-daw-frontend && rg -n --glob '*.gd' 'vit://' app tools

# A4. 前端新前缀的 agent 侧消费核查（零命中=IPC 信封域实证）
cd /d/Vit_DAW/agent && rg -n 'vsp_transport_|track_lane_2d_|remove_clips_|startup_probe_|gui_save_probe_|gui_vsp_probe_|library_plugin_list_' internal/ cmd/

# A5. PCA 生成点与消费链
cd /d/Vit_DAW/agent && rg -n 'pcr1_|evidenceRef\(' internal/ --glob '!*_test.go'
cd /d/Vit_DAW/agent && rg -n 'receipt|HandleFunc' cmd/pcactl/main.go
cd /d/Vit_DAW/agent && rg -n 'processor-certification' internal/chat/server.go

# A6. agent 侧 mixboard_/kernel_prepared_ 写者与桥快照消费面
cd /d/Vit_DAW/agent && rg -n '"mixboard_" ?\+|"mixboard_l2_render_probe_|kernelFeatureMaterializerRequestID|SnapshotRow|latest_request' internal/harness/harness.go

# A7. mix-report:/mixboard-project: 全库消费核查（三写点之外零消费实证）
cd /d/Vit_DAW/agent && rg -n 'mix-report:|mixboard-project:' internal/ cmd/ webui

# A8. 注册表现状（15 条）与两个未注册族的核查
cd /d/Vit_DAW/agent && rg -n 'LegacyPrefix:' internal/agentprotocol/refschema.go
cd /d/Vit_DAW/agent && rg -n 'pcr1_|ccbr_' internal/agentprotocol/refschema.go   # 零命中=未注册

# A9. 盘点/旧产出零覆盖核查（本报告新发现的独立性）
cd /d/Vit_DAW && rg -n 'mix-report|mixboard-project|mixboard_l2_render_probe|pcr1_|ccbr_' coord/runs/L1-1-RECON-1/EVIDENCE_REFS_INVENTORY.md   # 零命中
```

*勘察边界：前端仓+PCA 三包+衍生消费链锚点；行号以 2026-10-09 晚实读为准（agent HEAD afddcea3）。纯只读，两仓库零改动，本报告为唯一写入物。*
