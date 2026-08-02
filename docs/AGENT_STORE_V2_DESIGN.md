# Agent 工程存储格式 v2 设计

日期:2026-08-01
基线:`39e4c78`(`codex/eq-structural-matrix-unification`)
状态:Part 1 已实现并通过存储/保存生命周期自动化验收;Part 2(L3 P1–P5)未开始

实现校验摘要(2026-08-01):

- Go 全仓测试:`go test ./...`;
- kernel Release 构建:`cmake --build VitApp/build_release --config Release --target VitApp`;
- kernel 独立临时目录烟测:`scripts/project_store_v2_kernel_smoke.py`，覆盖普通 Save As、`reference_only`、`copy_referenced_audio`、导出期间 active identity 不变及三类工程重开;
- v2 生命周期测试覆盖普通保存不创建 package、Save As fork/rebind 后切换 active、文件夹导出不切换 active、源 manifest/Journal 零变化、旧 package 只读发现、Workspace 权威路径零新增、重启恢复和容量降级;
- L3 仅完成 Part 1 所需的单写者收口:kernel 只发布事件，由 Agent 写 `.vit_derived/<uuid>/l3_feature_log.jsonl`;未改动 L3 缓存、重放、并发、优先级或任务泵。

## 0. 文档定位与任务边界

本文档是移交实现的完整规格,包含两条任务线:

- **Part 1 — Agent 工程存储格式 v2**(纯 agent/Go 线)。根治当前失控的数据链:运行时缓存充当工程权威、Observation 嵌入完整原始结果、Journal 单文件整体重写。
- **Part 2 — 观察层生产侧优化**(kernel C++ 为主 + agent 小改,独立任务包)。解决 L3 内容更新慢、刷新机制不明确的问题。不阻塞 Part 1,但包含一项 Part 1 契约的 kernel 落地。

**实施顺序**:Part 1 先落地并通过其烟测合同,再做 Part 2。两条线独立提交、独立验证、独立回滚。同一实现者执行,不得混批,否则烟测回归无法归因(Part 2 的 P3 会改变 L3 就绪时序,直接影响 acoustic package ready 时机)。

**冻结声明**:旧 Agent 工程数据与旧运行时数据只读归档,不迁移、不覆盖、不删除;保留 `39e4c78` 以来的 A–F、B4、C1 全部代码成果;不得 reset、clean、checkout 或覆盖用户成果。v2 不改变既有工具语义与既有 API;只改存储落点/载荷形状,并增量新增显式文件夹导出命令。

---

# Part 1 — Agent 工程存储格式 v2

## 1. 目标(与作者确认的 8 项)

1. 工程级隔离 Agent 状态、观察证据、Mixboard 和 Journal。
2. 运行时缓存(`VitApp/Workspace`)不再作为工程权威数据。
3. Observation 保存紧凑投影和证据引用,不嵌入完整原始结果。
4. Journal 改为紧凑、追加/分片格式,不保存完整 `project.state`/`mix.observe`。
5. 所有对象绑定 `project_uuid`,并设置体积和数量门禁。
6. 保存、另存为工程、另存为文件夹、重启恢复使用同一套 v2 工作区;普通另存为与文件夹导出的媒体语义严格分离。
7. 烟测使用独立目录和独立 Agent 存储,不污染正式工作区。
8. 从新建实验工程验证 A5→B1→B2→B4→C1,并覆盖保存、重启、继续处理。

里程碑口径(作者确认):A5=段落地图(`project.markers.apply_section_markers`);B1=Gain Staging;B2=静态主次与音量平衡(`track_gain_adjust`);**B4=低频关系(`low_end_relation`),经插件 EQ 治理作为执行手段**;C1=频段清理(使用插件 EQ)。后续 C2 将新增压缩器能力,v2 的 evidence kind 注册机制不得写死现有类型。

## 2. 现状盘点(2026-08-01 实测)

| 存储 | 位置 | 实测体积 | 问题 |
|---|---|---|---|
| 对话图+运行时状态 | `.vit_history/`(工程旁) | 0.3 MB | 健康,v2 复用 |
| 派生分析 | `.vit_derived/<uuid>/` | 按需 | 已是 v2 形态,保留 |
| Mixboard+Observation | `VitApp/Workspace/Artifacts/mixboard/` | **3.1 GB / 14299 文件** | 全局运行时目录充当权威;单 observation 72–74 MB |
| Journal | `VitApp/Workspace/Logs/agent_journal.json` | **1.2 GB 单文件** | 每次 Record 整体重写;result 内嵌完整 `project.state`/`mix.observe` |

关键实证:

- 单个 observation JSON 中 `feature_snapshot` 约 28 MB 被**内嵌两份**(`global_summary` 与 `environment_package`),其中 `spectrogram_tile_rows` 占 27.6 MB。而 `mom/context_pack.go:370` 构建 LLM 上下文时本就剥离 `spectrogram_tiles/tile_payload`——LLM 从不消费这些大行。
- `journal.go:186`(`persistLocked`)每次写入都 `MarshalIndent` 整个 actions 数组(上限 500 条)并整文件替换。
- `mixboard.go:880`(`DefaultRoot`)与 `acousticpackage/status.go:118`(`DefaultStorePath`)默认落在 `VitApp/Workspace`,harness 有 4 处 `mixboard.NewStore("")` 调用点(`harness.go:2536/3334/3348/3376`)。
- 紧凑化机制已存在但未被用于落盘:`mixboard/catalog.go` 的 `BuildDigest`/`BuildCatalog`。

## 3. 设计原则

**权威 / 可再生二分**(写入 manifest,作为淘汰判定依据):

- 权威(不可自动淘汰):journal 审计、mixboard decision ledger、conversation graph、agent runtime state。
- 可再生(可安全淘汰):evidence blob(重新 `mix.observe` 即可再造)、context pack(可由 observation 重建)。

**三条红线**:

1. Agent 存储门禁永远不能阻塞 `.vit` 工程本身的保存,不能丢用户工作;门禁失败只降级 Agent 记录。
2. 权威数据不被自动淘汰静默删除。
3. 可再生数据淘汰不需要询问用户。

## 4. 目录布局(三根,全部与 `.vit` 同级、按 uuid 隔离)

```
工程目录/
├─ MySong.vit, MySong_Media/
├─ .vit_history/<uuid>/          # 草稿管理类(小,KB–低 MB)
│  ├─ conversation_graph.json        # 现有
│  ├─ state/agent_runtime_state.json # 现有
│  └─ (commits/objects/worktrees)    # 现有
├─ .vit_agent/<uuid>/            # 追加共享类(大,门禁管理)——新增 v2 根
│  ├─ manifest.v2.json
│  ├─ journal/000001.jsonl …
│  ├─ observations/obs_<id>.json
│  ├─ evidence/objects/<sha256[:2]>/<sha256>.json
│  ├─ evidence/index.json
│  └─ mixboard/sessions/<id>/(current.json, context_pack.json)
│  └─ mixboard/decisions/(decisions/, decision_events/, current_decisions.json)
└─ .vit_derived/<uuid>/          # 派生分析类(现有,扩容)
   ├─ analysis_manifest.json         # 现有
   ├─ l3_feature_log.jsonl           # 现有(唯一写者改为 agent,见 Part 2.7)
   └─ acoustic_package_status.json   # 从 VitApp/Workspace/Artifacts 迁入
```

**布局决策记录——为什么追加类不并入 `.vit_history/<uuid>/`**:

1. `working_session.go:74` 开会话时**整个 HistoryDir 被复制**为草稿;`saved_generation.go:395` 每次保存又整目录复制一份存代。大对象放里面会导致每次开工程/保存复制几百 MB。
2. `saved_generation.go:402` `replaceWorkspaceFromPrepared` 的语义是**旧目录整体改名→删除,新目录整体替换**。追加类若不参与草稿,保存提交时会被静默删除。
3. 修改这三条复制路径(排除+保留)风险高且语义别扭(草稿不再自包含)。沿用已验证的 `.vit_derived` 兄弟根模式 + `ForkDerived` 同款 copy+rebind,对 history 包保存路径的改动为零。

**媒体目录不是第四个 Agent 权威根**:`MySong_Media/` 只保存工程引用的媒体资产。普通保存与普通另存为不因 v2 自动复制媒体;只有“另存为文件夹”在用户明确选择复制媒体时才创建/填充该目录。Agent 三根的 fork/rebind 与媒体复制是两条独立事务步骤,由同一次文件夹导出协调提交。

## 5. 对象格式规格

所有 v2 对象头部必须携带 `schema_version` 与 `project_uuid`;读取时校验(沿用 `LoadAnalysisManifest` 的 identity mismatch 拒绝模式)。schema id:manifest=`vit_agent_store_manifest.v2`,journal 记录=`journal_action.v2`,observation=`mix_observation.v2`,evidence 索引=`evidence_index.v1`。

### 5.1 manifest.v2.json

```json
{
  "schema_version": "vit_agent_store_manifest.v2",
  "project_uuid": "vitproj_…",
  "project_path": "…",
  "created_at": "…", "updated_at": "…",
  "budgets": {
    "observation_max_bytes": 2097152,
    "observation_max_count": 500,
    "evidence_max_bytes": 419430400,
    "journal_shard_max_bytes": 4194304,
    "journal_shard_max_records": 1000,
    "journal_max_shards": 50,
    "store_total_max_bytes": 536870912
  },
  "counters": {
    "journal_bytes": 0, "journal_records": 0, "journal_shards": 0,
    "observations_bytes": 0, "observations_count": 0,
    "evidence_bytes": 0, "evidence_count": 0,
    "mixboard_bytes": 0, "total_bytes": 0
  },
  "classes": {
    "authoritative": ["journal", "mixboard_decisions"],
    "renewable": ["evidence", "context_packs"]
  },
  "degraded": { "evidence_off": false, "reason": "" },
  "gate_audit": [
    { "at": "…", "action": "evict_evidence", "detail": "…", "freed_bytes": 0 }
  ]
}
```

`gate_audit` 只保留最近 100 行。counters 增量维护,打开工程时扫描校准一次。开发期靠 counters 分布校准阈值默认值(见 §11 开放项)。

### 5.2 Journal v2(追加 JSONL + 分片)

每行一条 `journal_action.v2`:

```json
{
  "schema_version": "journal_action.v2",
  "project_uuid": "vitproj_…",
  "agent_action_id": "act_…", "run_id": "…", "goal_id": "…", "tool_call_id": "…",
  "created_at": "…", "updated_at": "…", "duration_ms": 0,
  "domain": "daw", "source": "…",
  "tool": "mix.observe", "command_name": "…",
  "args_summary": "…(≤512 字符;大输入写 hash 引用)",
  "risk_level": "…", "requires_confirmation": false, "confirmation_status": "…",
  "status": "succeeded", "error": "…(≤1KB)",
  "project_revision": "…",
  "result_summary": { "status": "ok", "counts": {} },
  "result_ref": "evidence://<sha256>(可选)",
  "evidence_refs": [], "decision_refs": [],
  "undo_label": "…", "target_action_id": "…", "rollback_action_id": "…", "rollback_state": "…",
  "version_commit_id": "…",
  "workspace_rollback": { "path": "…", "reverse_old_text": "…", "reverse_new_text": "…" }
}
```

白名单规则(依据消费方实证):

- **禁止内联**:`project.state` 的 tracks/clips 数组、`mix.observe` 完整结果、`feature_snapshot`、context pack。需要时按 `result_ref` 取或重新执行。
- `workspace_rollback` 仅 `domain=workspace` 且 `command_name=workspace_apply_edit` 时填写——`rollback/rollback.go:53` 只读这三个字段;`domain=daw` 的回滚走 kernel `undo`+`target_action_id`,不读 Result。
- 其余消费方(`server.go:1182` HTTP actions 端点、`server.go:5617` devActionsReply)只用 ID/tool/status/rollback_state,接口保持不变。

分片:当前片写满(4 MB 或 1000 条)即封存开新片;`Record()` 从"整文件重写"变为"追加一行"。读取:`Recent(limit)` 只从尾部片反向读;`Get(actionID)` 先内存索引(打开时扫描各片头尾 id 建立),miss 再扫片。

### 5.3 Observation v2(紧凑投影 + 证据引用)

保留内联:`observation_id/mix_session_id/round/status/target_ref/mix_objects/listen_scope/time_ruler/digest/catalog/`、紧凑 packages(global/project/mix/deep 的 compact 版)、`fxm/mom/tim` 投影(本身已紧凑)、`section_candidates/hotspots/source_capabilities/acoustic_package_status(紧凑)/notes/created_at`、`evidence_refs`。

抽出到 evidence(按内容 SHA-256 寻址):`feature_snapshot` 全量、`spectrogram_tile_rows`、waveform envelope 数组、time-energy 大行组、L2 render probe 原始行。**同一份大行组被多轮观察共享时只存一个 blob**(before/after 观察常见)。

读取路径:`catalog.go` 的 `Read`/`Derive` 与 `mixboard.go:268` 频率恢复改为经 evidence store 懒加载,API 不变。

### 5.4 Evidence blob 与 index

`evidence/objects/<sha256[:2]>/<sha256>.json` 存内容;`evidence/index.json`:

```json
{
  "schema_version": "evidence_index.v1",
  "project_uuid": "vitproj_…",
  "entries": {
    "<sha256>": {
      "kind": "feature_snapshot|spectrogram_tile_rows|waveform_envelope|time_energy|l2_render_probe|journal_result",
      "size": 0, "created_at": "…", "last_referenced_at": "…",
      "refs": ["obs_…", "act_…"], "renewable": true
    }
  }
}
```

`kind` 是开放注册表(C2 压缩器等新能力可新增 kind,不需改 schema)。

### 5.5 Mixboard 迁入

- `mixboard/sessions/<id>/current.json`(Board,小)与 `context_pack.json`(紧凑、可再生)。
- decision ledger 从 `Artifacts/mixboard/_projects/<uuid>/` 迁至 `.vit_agent/<uuid>/mixboard/decisions/`,保持 `mix_decision_record.v1` 等既有 schema 与幂等语义(`decision_ledger.go` 仅换根)。decision 引用的 observation 被门禁淘汰时不悬空——digest 已内联在 record 内。

### 5.6 acoustic package status

`acousticpackage.DefaultStorePath` 改为 `.vit_derived/<uuid>/acoustic_package_status.json`(它是派生数据,与 analysis manifest 同根)。调用方需能拿到当前工程 uuid——随工程激活注入,无工程上下文时拒绝写(见 §8 env 处置)。

### 5.7 文件夹导出快照与媒体策略

`projectpackage` 保留,但 v2 起只承担**显式导出快照构建器**职责:从目标 `.vit` 与三个 v2 工程根生成可校验的文件夹快照,不再作为打开工程时需要恢复的另一套权威存储。导出的文件夹以后被打开时,其中的 `.vit_history/.vit_agent/.vit_derived` 就是该新工程自己的 canonical 数据。

目标布局:

```text
目标文件夹/
├─ MySong.vit
├─ MySong_Media/
│  └─ Audio/                    # 仅 copy_referenced_audio 时创建/填充
├─ .vit_history/<new_uuid>/
├─ .vit_agent/<new_uuid>/
├─ .vit_derived/<new_uuid>/
└─ package_manifest.v2.json
```

“是否打包音频源文件”是 `save_as_folder` 的独立媒体决策,不得从“文件夹模式”隐式推断。v2 第一版支持两种策略:

| `media_policy` | 行为 | 音频是否自包含 |
|---|---|---|
| `reference_only` | 不复制音频;目标工程继续引用原始媒体位置 | 否 |
| `copy_referenced_audio` | 复制当前工程实际引用的**完整源文件**,保持原格式/采样率/位深;目标工程改写为包内相对路径 | 是(前提是无 missing/copy_failed) |

这一拆分与主流 DAW 的成熟语义一致: Ableton 将 [Collect All and Save](https://help.ableton.com/hc/en-us/articles/209775645-Collect-All-and-Save) 独立于普通 Save/Save As;Logic Pro 将[工程资源复制到项目或保留外部引用](https://support.apple.com/zh-cn/guide/logicpro/lgcpce0d70e7/mac)作为单独设置;REAPER 的 [Save project as](https://www.reaper.fm/userguide.php) 分别提供创建子目录、复制媒体、移动媒体等选项;Cubase 的 [Back up Project](https://www.steinberg.help/r/cubase-pro/15.0/en/cubase_nuendo/topics/project_handling/project_handling_back_up_project_options_r.html) 与 Pro Tools 的 Save Copy In 也把“创建副本”和“是否包含音频”分开。v2 采用其共同最小语义:普通另存为只建立新工程身份,便携文件夹导出再显式询问是否收集音频。

`copy_referenced_audio` 的“referenced”按工程 clip/media source 的真实引用集合枚举,不能只从 acoustic package 或 L3 已分析条目推断,否则未分析媒体会漏包。同一源文件被多个 clip 使用时只复制一次;同内容不同路径可按 SHA-256 去重;不同内容同名时目标文件名追加短 hash,不得相互覆盖。该策略永远是 **copy**,不得移动或删除源文件。

“只复制实际使用的时间片段/裁短源文件”属于 `consolidate_used_ranges`/minimize 能力:它会生成新媒体并涉及 handles、交叉淡化、拉伸、反向、代理文件及舍入边界,不纳入 v2 第一版,也不得把它与 `copy_referenced_audio` 混为一谈。

只要预检发现 `referenced_audio_count > 0` 且请求未携带 `media_policy`,kernel 就返回 `require_media_policy`,并附带 `referenced_audio_count/external_audio_count/missing_audio_count/estimated_copy_bytes` 供 UI 询问用户。这里无论媒体当前位于源工程文件夹内还是外部磁盘,都由用户决定是否复制到**新的目标文件夹**。建议 UI 默认选中 `copy_referenced_audio`,但必须由用户确认;若工程没有音频引用,可直接完成且不弹无意义询问。

UI 至少提供两个清楚选项:“打包工程引用的音频(推荐,约 X GB)”与“不打包音频,保留原位置引用(移动到其他机器后可能离线)”。选择 `reference_only` 时,目标 `.vit` 必须把原本相对源工程目录的媒体引用重新序列化为仍指向原媒体的稳定路径;否则导出完成后在本机就会立即断链。“当前工程内容全部打包”在本规格中指 `.vit` + 三个 v2 根 + 按所选策略处理的工程引用媒体,不包含源目录里未被工程引用的任意文件。

调用示例:

```json
{
  "cmd": "save_as_folder",
  "directory_path": "E:/Portable/MySong",
  "media_policy": "copy_referenced_audio"
}
```

`package_manifest.v2.json` 至少记录:

```json
{
  "schema_version": "vit_project_package_manifest.v2",
  "project_uuid": "vitproj_new",
  "source_project_uuid": "vitproj_source",
  "origin_project_uuid": "vitproj_origin",
  "project_file": "MySong.vit",
  "export_kind": "save_as_folder",
  "media_policy": "copy_referenced_audio",
  "audio_self_contained": true,
  "unbundled_dependency_classes": ["plugins", "sampler_libraries", "video", "external_ir"],
  "package_status": "complete",
  "media": [
    {
      "kind": "audio",
      "source_path": "D:/Audio/vocal.wav",
      "target_path": "MySong_Media/Audio/vocal.wav",
      "size": 582193920,
      "sha256": "…",
      "status": "copied"
    }
  ]
}
```

媒体条目状态至少包括 `referenced/copied/missing/skipped/copy_failed`。`reference_only` 必须写 `audio_self_contained=false`;选择复制但存在 `missing` 或 `copy_failed` 时不得标记 complete。`audio_self_contained=true` 只说明工程引用的音频源已收集,不代表插件、采样器库、视频或外部 IR 已随包携带。第一版默认失败并保留源工程零改动;只有未来加入显式 `allow_incomplete=true` 后才允许发布 `package_status=incomplete` 的包。

文件夹导出使用同盘 staging + 校验 + 原子发布:先写新 `.vit`、fork/rebind 三根,再复制/校验媒体并改写**目标工程**引用,最后发布目标目录。任一步失败都丢弃 staging;不得修改源 `.vit`、源媒体、源三根或当前活动工程。

## 6. 生命周期状态流(同一套 v2 工作区)

草稿管理类走 history 既有机制;追加/派生类按 ForkDerived 模式处理。kernel 命令明确拆为 `save_project`、`save_as_project`、`save_as_folder`;不得通过“传入的是文件还是目录”重载 `save_as_project`。agent 钩子为 `version.project_opened/project_new/project_save_prepare/project_saved`(带 `save_kind`),文件夹导出另发 `version.project_folder_exported`,避免把未切换活动工程的导出误判为 project switch。

1. **新建工程**:draft。草稿类由 history draft 机制管理;追加类落在 draft 派生的 `.vit_agent` 根(精确位置见 §11 开放项),随首次保存被收养。
2. **打开工程 / 重启恢复**:激活时打开三根;manifest counters 扫描校准;journal 追加一行会话标记;`Recent` 只读尾部片;evidence 按需懒加载;`.vit_derived` 走既有 `LoadAnalysisManifest`/`BuildL3AcousticStatuses` 恢复。重启后继续处理所需状态(pending/对话/mixboard decisions)全部可得。
3. **保存(save_kind=save)**:只保存当前 `.vit` 并提交草稿类存代;**不复制** `.vit_agent/.vit_derived`、Agent evidence、媒体源文件或 projectpackage 快照。追加类与派生类不参与提交、不被触碰(它们本就位于 canonical)。
4. **另存为工程(command=`save_as_project`,save_kind=`save_as`)**:输入是明确的目标文件路径。创建当前节点的新工程副本与新 uuid;草稿类走 `history.ProjectFork`,派生类走 `projectworkspace.ForkDerived`(已有),追加类走新增 `ForkAgentStore`:copy + uuid/path rebind。完成后切换当前活动工程到目标工程。媒体不复制;目标 `.vit` 继续引用同一源媒体,相对引用必须重新序列化为仍能解析到原媒体的位置。源工程与源三根保持零改动。
5. **另存为文件夹(save_kind=save_as_folder)**:输入是明确的目标目录与 §5.7 `media_policy`。在目标目录创建独立 `.vit`、新 uuid 与完整 fork/rebind 后的三个 v2 根;按媒体策略决定保留外部引用或复制源音频。它是导出新工程快照,**默认不切换当前活动工程**,后续用户主动打开目标文件夹中的 `.vit` 时才激活该新工程。`projectpackage` 只负责构建/校验该快照,不形成 sidecar 权威副本。
6. **工程切换**:仅新建、打开、普通另存为工程成功时执行 persist 当前 runtime state → activate 新工程 → journal 换绑到新工程根。文件夹导出成功不执行切换。
7. **草稿收养**:沿用 `ProjectSaved` 收养模式,draft 的追加类一并移动+rebind。

源工程在两种 fork 后都保持零改动(现有 `TestExternalSaveAsForksWorkingRuntimeAndKeepsSourceCanonicalFrozen` 的语义扩展到追加/派生类与媒体引用);文件夹导出还必须断言导出后 active project identity 未变化。

## 7. 门禁与容量阶梯

阈值三级可配:全局默认常量 ← manifest `budgets`(工程级覆盖)← 环境变量(烟测用,如 `VIT_V2_STORE_BUDGET_MB=5`)。

- **L0 预算内**:manifest 持续计数。
- **L1 单对象超阈**:写入时处理。observation >2 MB → 强制投影化(大行抽 evidence);evidence 单 blob 超上限(默认 64 MB)→ 拒写该 blob,本次观察降级为仅投影,记审计。
- **L2 类别总量超预算**:自动处理,不问用户。evidence 按"最老且不被最新 observation 引用"淘汰;journal 老分片折叠(丢弃成功的只读轮询记录如 `project.state` 轮询,保留写操作/失败/用户确认,折成 archive 分片);observations 超数量 → 淘汰最老非 pinned(pinned=被 decision 引用)。
- **L3 工程总量硬超且无可淘汰**:**降级不阻断**。切 `evidence_off` 精简模式(新观察只写投影,journal 继续),UI 非阻塞提示,用户三选一:①一键整理(立即 L2+vacuum)②另存延续(slim fork:只带 manifest+decisions+近期 journal 摘要)③提高本工程预算。
- **L4 权威类自身超上限**(紧凑化后实际不可能):只告警,用户决策。

## 8. 兼容边界

**只读冻结**(不迁移、不覆盖、不删除、v2 代码不读取):

- `VitApp/Workspace/Artifacts/mixboard/**`(3.1 GB 旧观察)
- `VitApp/Workspace/Logs/agent_journal.json`(1.2 GB 旧 journal)
- `VitApp/Workspace/Artifacts/acoustic_package_status.json`
- `.vit_history/projects/<path-key>/`(legacy 布局;既有 `migrateLegacyHistoryWorkspace` 迁移器保持原契约)

现有 `.vit_project/` 与 `*.vit_project/` package 继续只读兼容发现,但 v2 新写入不得依赖 `Restore()` 将其还原成工程状态。新“另存为文件夹”直接写出可独立打开的工程目录和 `package_manifest.v2.json`;旧 package 不自动重写、不删除。

**env 处置**:`VIT_MIXBOARD_ROOT`、`VIT_AGENT_JOURNAL_PATH`、`VIT_ACOUSTIC_PACKAGE_STATUS_PATH` 仅在 dev/烟测进程有效;生产运行忽略,默认根一律工程派生。无工程上下文时 v2 store 拒绝写或落显式 temp。

**行为不变承诺**:A–F、B4(低频关系,经插件 EQ 治理)、C1 的工具语义、pending/确认流不变;既有 `save_project`/`save_as_project` API 保持兼容,只增量新增 `save_as_folder` 与 `require_media_policy` 回复。除显式文件夹导出外,v2 只改存储落点与载荷形状。

## 9. 消费方改造清单(精确调用点)

1. `harness.go:138` `journal.NewPersistent(500, defaultJournalPath())` → v2 工程绑定分片 journal,随工程激活换绑(`chat/server.go:6875` `activateCurrentProjectWorkspace` 同点挂接)。
2. `harness.go:2536/3334/3348/3376` 与 `cmd/mixlab/main.go:102` 的 `mixboard.NewStore("")` → 工程派生根(`.vit_agent/<uuid>`);mixlab 等 dev 工具保留显式 root。
3. `acousticpackage/status.go:118` `DefaultStorePath` → `.vit_derived/<uuid>/`(§5.6)。
4. `mixboard/catalog.go` `Read`/`Derive` 与 `mixboard.go:268` `recoverFrequencySnapshotFromPriorObservations` → evidence 懒加载;根换工程级。
5. `internal/artifacts` store(`chat/server.go:6386` `artifactRoot`)→ 归类 triage(见 §11)。
6. `rollback/rollback.go` → 不变(白名单已覆盖)。
7. `server.go:1182/5617` actions 读取 → 接口不变,实现换 v2 journal `Recent`。
8. `projectpackage` 的 `Prepare/Commit/Restore` → 新写路径改为从 `.vit_history/.vit_agent/.vit_derived` 构建 `package_manifest.v2.json`;取消其作为运行时权威恢复源的职责,旧 v1 仅保留只读兼容。
9. `ProjectService.cpp` 当前拒绝目录的 `save_as_project` 保持不变;新增独立 `save_as_folder` handler。媒体枚举必须来自 kernel 当前工程 source graph,并在保存前返回媒体预检摘要。

## 10. 烟测合同

**隔离**:临时目录 `$TEMP/vit-v2-smoke-<guid>/` 内用 fixture 新建实验工程(stems 工程)。v2 三根天然工程相对,全部落在临时目录;agent 存储独立,不污染正式工作区。

**步骤**:A5(段落地图提案→确认→写 markers)→ B1(Gain Staging)→ B2(静态平衡)→ B4(低频关系,经插件 EQ)→ C1(频段清理提案)。

**每步断言**:

1. 权威文件只出现在工程三根内(`.vit_history/.vit_agent/.vit_derived`);
2. `VitApp/Workspace` 前后快照零新增(特指 `Artifacts/mixboard`、`Logs/agent_journal.json`、`Artifacts/acoustic_package_status.json`);
3. 每个新 observation ≤2 MB 且不含 `spectrogram_tile_rows` 等大行内嵌(抽查 JSON key);
4. journal 分片 append-only(尺寸单调,无整文件重写);
5. 所有新对象 `project_uuid` 校验通过。

**生命周期断言**:

- 保存 → 杀 agent 进程 → 重启 → 重开工程 → 继续处理:pending/对话/mixboard decision board/evidence index 完整可用;
- 另存为工程:新 uuid,fork 完整(decisions/manifest 已 rebind),不复制音频,目标成为 active,源工程零改动;
- 另存为文件夹(`reference_only`):新 uuid 与三根均在目标目录,active 仍是源工程,manifest 为 `audio_self_contained=false`,没有音频复制;
- 另存为文件夹(`copy_referenced_audio`):目标音频 hash/尺寸与源一致,目标 `.vit` 仅引用包内相对路径,manifest 为 `audio_self_contained=true`,源音频/源工程/active identity 均不变;
- 媒体预检:存在工程引用音频但缺少策略时返回 `require_media_policy`;存在 missing 或注入 copy failure 时不发布 complete 包,staging 可清理且源零改动;
- 门禁压测:微型预算(如总量 5 MB)下连续观察,断言淘汰审计行、`evidence_off` 降级、`.vit` 保存不受影响。

**出口标准**:以上全绿 + 既有 `project_workspace_persistence_test.go` 等 5 个持久化测试不回归。

## 11. 开放项(实现时确认)

1. `internal/artifacts` store 归类:按工程归属迁入 `.vit_agent/<uuid>/artifacts/`,应用级残留除外——实现前 triage 其内容。
2. 阈值默认值:先用 §7 默认值,开发期以 manifest counters 分布校准。
3. draft 期 `.vit_agent` 根的精确位置(与 history draft 路径派生方式对齐)。
4. 后续是否实现 `consolidate_used_ranges`、视频/采样器样本/外部 IR 等媒体类别;这些能力必须各自显式选择,不得扩张 `copy_referenced_audio` 的 v2 语义。

---

# Part 2 — 观察层生产侧优化(独立任务包)

> 本部分不阻塞 Part 1。观测层 L1=常驻离线内容、L2=实时提取内容、L3=周期提取内容。结论:L1/L2 链路健康,无需改动;L3 有 5 处可优化,其中 P2/P3 直接解释"更新慢、刷新机制不明确"。

## 2.1 L3 链路现状(实证)

导入时每 clip 提交 2 个特征(`ImportService.cpp`:L1 waveform=ImportImmediate + L3=BackgroundWarm)→ `AudioFeatureService` 30 秒合并去重 → L3 进入**单线程池**(`L3AcousticAnalyzer.cpp:647`,`withNumberOfThreads(1)`)→ `analyzeFile` 全文件解码 + 4096 阶 FFT → 三路输出:事件推 agent、kernel 直接 append `.vit_derived/<uuid>/l3_feature_log.jsonl`(`L3AcousticAnalyzer.cpp:37`)、agent `AppendL3Feature` 去重落盘。后台任务泵 `timerCallback` 每 1 秒提交 1 个 clip(`kImportAnalysisAutoSubmitIntervalMs=1000`,`kImportAnalysisFeaturesPerClip=2`)。

## 2.2 P1 — 同源重复分析,无源级缓存(最大浪费)

`analyzeFile` 无缓存键;多 clip 同源(双轨同素材、clip 拆分)对同一文件重复全量 FFT,而导入范围恒为全源,结果只依赖(文件, source_revision)。
**改法**:加源级 single-flight 缓存(key=文件+source_revision;在跑则挂等待、完成则共享结果)。

## 2.3 P2 — "缺失重放"只认 L1,不认 L3(刷新不明确的直接原因)

`ImportService.cpp:1690` `retry_missing` 用 `WaveformEnvelopeBaker::getLatestBakeStatus` 判定——L3 缺失但 L1 ready 时回复"already ready",**L3 缺口永不重放**,只能等 agent 侧 full_project 观察触发的 background fill(`harness.go:5848`)。
**改法**:重放判定按特征类型分别检查(L1/L3 各自算 missing)。

## 2.4 P3 — 单线程池 + FIFO 无优先级(慢的主因)

串行化是当年为 4GB 工程烟测确定性加的(注释原文),但 L3 任务按源独立,磁盘读已有 `OfflineAudioReadCoordinator` 租约。`juce::ThreadPool` 纯 FIFO,`OnDemand` 优先级只影响去重不影响调度——用户在等的 L3 排在全部后台暖场之后。
**改法**:磁盘读串行、FFT 计算并行(N=2~4;结果按源落盘,与顺序无关,确定性不破坏);OnDemand 请求插队到队首。

## 2.5 P4 — 任务泵固定 1 clip/秒

50 clip 工程仅提交需 50 秒才轮到 FFT。**改法**:批量提交(每 tick N 个)或直接全量入池由池限流,泵只负责进度上报。

## 2.6 P5 — 源文件磁盘变更无 watcher

L3 不随 clip 编辑重分析是**正确设计**(源级分析,trim/gain/fade 不改源内容);但源文件在磁盘被替换时无任何路径重求,只能等重新导入。
**改法**:agent 侧 fill 判定加"source_fingerprint 与 L3 行不符则重求"(fingerprint 字段已存在)。

## 2.7 kernel 落盘移除(Part 1 契约的 kernel 落地)

`l3_feature_log.jsonl` 现有**两个写者**:kernel `persistProjectL3Telemetry`(裸 append)与 agent `AppendL3Feature`(去重 + `RebindL3FeatureLogFile` 原子替换)。另存 fork 的 rebind 替换瞬间 kernel 正在 append 的行会丢;agent 内存 digest 对 kernel 后写行不可见,可能出重复行。
**契约:v2 起 agent 是唯一写者**;kernel 只发 `audio_feature_data_ready` 事件,删除 `persistProjectL3Telemetry`。

## 2.8 明确不改的部分(机制合理)

- L1 链路:ImportImmediate 优先级、独立 baker、`invalidateClipBake` 覆盖 clip 删除/拆分/strip silence,失效-重求闭环完整。
- L2 实时链:bridge 实时 metering,与离线链无关。
- L3 源级语义:不随 clip 编辑刷新是正确设计;stale 按 revision 契约 agent 侧已有。
- 重启恢复:`l3_feature_log.jsonl` + `BuildL3AcousticStatuses` 折叠恢复,已是工程级持久化。

## 2.9 验证

- P1/P3/P4:多 clip 同源工程与 50 clip 工程,L3 全量就绪耗时对比(改动前后);
- P2:构造 L3 缺失 + L1 ready,`project.audio_analysis_start retry_missing` 能重放 L3;
- P3 回归:重复 4GB 工程烟测,结果确定性不变;
- 全部:Part 1 烟测合同不回归。

---

## 附录 A — 关键证据索引

| 事实 | 位置 |
|---|---|
| HistoryDir 布局/Open | `agent/internal/history/history.go:177` |
| 会话草稿整目录复制 | `agent/internal/history/working_session.go:74` |
| 存代整目录复制 | `agent/internal/history/saved_generation.go:395` |
| prepared 替换语义(删除陷阱) | `agent/internal/history/saved_generation.go:402` |
| ProjectFork / ProjectSaved / ForkDerived | `agent/internal/history/history.go:717/647`、`agent/internal/projectworkspace/derived.go:110` |
| Journal 整文件重写 | `agent/internal/journal/journal.go:186` |
| mixboard 默认根(运行时) | `agent/internal/mixboard/mixboard.go:880` |
| 紧凑 digest/catalog 已有 | `agent/internal/mixboard/catalog.go:77/205` |
| LLM 上下文剥离大行清单 | `agent/internal/mom/context_pack.go:370` |
| rollback 只读三字段 | `agent/internal/rollback/rollback.go:53` |
| L3 单线程池 | `VitApp/Source/Service/L3AcousticAnalyzer.cpp:647` |
| kernel L3 遥测落盘(双写者) | `VitApp/Source/Service/L3AcousticAnalyzer.cpp:37`、`agent/internal/projectworkspace/l3_feature_log.go:34` |
| retry_missing 只看 L1 | `VitApp/Source/Service/ImportService.cpp:1690` |
| 任务泵 1 clip/秒 | `VitApp/Source/Service/ImportService.cpp:24` |
| invalidateClipBake 不含 L3 重求 | `VitApp/Source/Service/AudioFeatureService.cpp:235`、`ClipService.cpp:1641/1804` |
| agent 观察触发 fill | `agent/internal/harness/harness.go:5848` |
