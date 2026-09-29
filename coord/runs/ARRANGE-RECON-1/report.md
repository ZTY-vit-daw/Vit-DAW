# ARRANGE-RECON-1 勘察报告：MIDI/音符面 + 生成集成通路 + 资料库 + 工程统一性底座 + agent 集成面 + 选型对比总表

- 卡片：`coord/cards/doing/2026-09-29-ARRANGE-RECON-1.md`（领取：2026-09-29 18:24 / origin/main=bcab6e7dba46dcea68b160e1c1ef7f773fe39c1b / port/arrange-recon-1，worktree `D:/Vit_DAW_worktrees/arrange-recon-1`）
- 代码锚点基线：**bcab6e7d**（=领取时 origin/main；领取 commit 68d7bad5 仅动卡面，代码零 diff）。工作树零代码改动。
- Godot 前端勘察基线：`D:\Godot\project\vit-daw-frontend` 工作树现状（仓库外，无版本锚；行号以当日文件为准）。
- 外部候选事实基线：2026-09-29 web 搜索快照（来源链接见各节；属可变性事实，非仓库锚点）。
- 性质：只读勘察，零代码改动。三态口径：**支持**（机制已在、可直接承载）/ **不支持**（机制缺失，需绕行或新建）/ **需新开面**（有机制但需新工作面或真栈验证）。
- 勘察方法：3 个并行只读 Explore 子代理（内核+Godot 面 / agent 资料库+工具链面 / 插件宿主+PCA 面）+ 主会话亲核承重锚点（tempo 披露链、AIGC 资产管线两处）。

---

## §1 MIDI/音符面盘点

### 1.1 内核 MIDI 数据面：导入/音符 CRUD/轨模型——支持（成熟）

- **SMF 导入**：`VitApp/Source/Service/MidiService.cpp:1816-1823` `juce::MidiFile::readFrom`，仅支持 PPQ（`timeFormat<=0` 报错）；`:1843-1877` 遍历 track 配对 noteOn/noteOff，tick→beat 换算 `startTick/ticksPerQuarter`；`:1903` `insertMIDIClip`、`:1918-1923` `seq.addNote(pitch, BeatPosition, BeatDuration, velocity)`；`:1910-1911` clip 落 `vit_source_file_kind="midi"`+`vit_original_source_path`。前端实发命令 `import_midi_to_track`（`app/kernel/clients/import_client.gd:106-112`）。
- **音符 CRUD 命令面**（`VitApp/Source/Service/CommandDispatcher.cpp:2609-2651`）：`add_midi_notes` / `add_midi_notes_bulk` / `mutate_midi_notes`（含 transpose `MidiService.cpp:733`、velocity `:745`）/ `delete_midi_notes` / `get_midi_clip_notes` / `get_midi_clip_data` / `insert_midi_clip` / `create_midi_clip`。自有音符 ID `vit_note_id`（`MidiService.cpp:20`）。读回序列化 `{pitch,start,length,velocity,note_id}`、time_unit 固定 beats（`MidiService.cpp:1717-1732`）。
- **轨模型**：无自研 Track 类，全部 tracktion `te::Track`；`te::AudioTrack` 映射 kind="hybrid"（`CommandDispatcher.cpp:827-839`）——wave clip 与 MIDI clip 同轨共存。MIDI clip 宿主校验 `dynamic_cast<te::ClipTrack*>`（`MidiService.cpp:1827`）。clip 类型二分 `clip_type = isMidi ? "midi":"audio"`（`VitApp/Source/Core/VitClipRouteRegistry.cpp:19,46,431`）。
- **与音频导入对称**：音频链 `ImportService.cpp:906-978`（`insertWaveClipWithUndoAndStartBake`，undo transaction+flush+波形烘焙+save）；MIDI 链同构（差异仅 `insertMIDIClip` vs `insertWaveClip`）。
- **渲染面**：实时/离线均委托 tracktion（`VitEngineDevice.cpp:12,65-71`；`VitProductionCoordinator.cpp:630` `startOfflineRender` → `te::EditRenderer`）——**产品渲染路径会驱动 MIDI clip→乐器插件**（tracktion 原生能力）。

三态：**支持**。

### 1.2 前端（Godot）钢琴卷帘与编辑现状——支持（存在收尾缺口）

- 钢琴卷帘套件齐备：`app/piano_roll/view/PianoRollView.gd`（Scroll 联动/Clip 视口遮罩/框选/群拖/Trim/右键擦除）+ PitchLane/BeatRuler/VelocityCanvas/GridCanvas + `piano_roll_hub.gd`（轨道抽屉/幽灵音符）；MIDI 积木 `app/piano_roll/blocks/MidiBlockCard.gd`、`custom_midi_blocks.json`；时间轴 MIDI clip `app/clips/midi/midi_clip_container.gd`、`midi_timeline_2d_layer.gd`。
- 卷帘实发内核命令：`add_midi_notes_bulk`（PianoRollView.gd:801）、`mutate_midi_notes`（:929,1811）、`resize_clip`（:861,1241）——与 1.1 命令面对接。
- **缺口 1（孤儿命令）**：试听 `trigger_note_on/off`（PianoRollView.gd:1072,1076）在内核全源码零命中，无 handler。
- **缺口 2（菜单禁用）**：轨型枚举 `app/tracks/vit_daw_track_kind.gd:3-10` 注释明言"当前仅 AUDIO_WAVEFORM 已实现"；新建菜单 MIDI 轨/编组/效果返送全部 `set_item_disabled`（`app/legacy/vit_control_v_1.0.gd:267-277`）；实建只走 `add_track("hybrid")`（:309）。

三态：**支持**（编辑面在）；孤儿命令与菜单启用属**需新开面**（小，前端域）。

### 1.3 虚拟乐器装载路径——部分支持（装载同链、认证与接线缺）

- **元数据贯通**：内核 `plugin_list_available` 披露 `is_instrument`（`PluginRackControlService.cpp:1259` `desc.isInstrument`）→ Godot 插件浏览器（`app/browser/left_library_dock.gd:655,878`）。
- **装载与效果器同链**：`instantiate_plugin`（`PluginRackControlService.cpp:2776+`）与 `rack_add_node` 均按 track_id 插插件；rack 节点只创建 `te::ExternalPlugin`（`PluginRackControlService.cpp:3113-3114`，"CLAP is not implemented"）。分区规则：乐器→Z2、效果器→Z3（`agent/internal/chat/plugin_grabber_workflow.go:107-112`；`agent/internal/agentloop/message_loop.go:4317`）。
- **断点 1（认证排除）**：乐器被 PCA 认证管线显式排除（`agent/internal/chat/processor_certification_entry.go:235` `if entry.IsInstrument … { continue }`）；C2 动态装载链硬编码只装 Effect/Z3（`agent/internal/chat/c2_dynamic_plugin_load.go:213`）。
- **断点 2（无自动接线）**：MIDI 路由仅"建议层"——`VitApp/Source/Core/VitZoneBufferAdapter.cpp:23-31` 对 `takesMidiInput()||isSynth()` 给 `advice.mode="native_midi_to_instrument"`（Z1→Z2 passthrough 建议）；内核从不自动为 MIDI clip 挂乐器插件或 MIDI 输出设备。**MIDI 导入后若轨上无乐器则无声**（产品渲染链能力在，接线靠手动）。
- **断点 3（观察宿主听不到）**：pluginprobe render 的 MidiBuffer 每块清空、从不注入事件（`PluginProbe/native-host/src/main.cpp:725-740`）——乐器插件在观察宿主中恒输出静音。

三态：**部分支持**——装载链同构可用；"写入即可听"的乐器自动接线与乐器认证面**需新开面**（决策项，见冲突事实 1）。

### 1.4 集成候选实勘（集合商原则，2026-09-29 复核）——支持（首选确认）

| 候选 | 用途 | 许可 | Windows/CLI | 输出 | 对接面 |
|---|---|---|---|---|---|
| **Basic Pitch**（Spotify） | 音频→MIDI，多声部+弯音 | **Apache-2.0** | `pip install basic-pitch`，CLI `basic-pitch <outdir> <audio>`；另有 npm `basic-pitch-ts`（ONNX，免 Python）；Python 3.12 有安装 issue（#159），3.8–3.10 稳 | `.mid` | 产 .mid → `midi.import_file`（`agent/internal/tools/catalog.go:883-891`）或下载扫描（`.mid/.midi` 已在白名单 `agent/internal/harness/harness.go:9684-9687`）→ `insert_midi_clip` → 卷帘编辑 |
| Omnizart | 多任务 AMT（人声/鼓/和弦/节拍/乐器） | MIT | pip；**~2021 起休眠**（v0.4.x 后无发布） | MIDI/乐谱 | 同上；休眠风险，仅备选 |
| Onsets and Frames（Magenta） | 钢琴 AMT | Apache（研究级） | 旧 TensorFlow 栈；生态已转向 Basic Pitch | MIDI | 不推荐（遗留栈） |
| **Audiveris** | OMR（PDF→乐谱） | **AGPL-3.0** | Windows 安装器（winget/scoop）；CLI `Audiveris -batch -export -output <dir> <pdf>` | **MusicXML 4.x（非 MIDI）** | 产 MusicXML → 需 **MusicXML→MIDI 转换步**（如 music21 脚本）→ 同 Basic Pitch 路径；AGPL 合规姿态=独立进程文件交换、不链接不改性（PianoScore 先例） |

来源：[spotify/basic-pitch](https://github.com/spotify/basic-pitch)、[basicpitch.spotify.com](https://basicpitch.spotify.com)、[basic-pitch issue #159](https://github.com/spotify/basic-pitch/issues/159)、[Audiveris GitHub](https://github.com/Audiveris/audiveris)、[Audiveris CLI 文档](https://bacchushlg.gitbooks.io/audiveris-5-1/content/advanced/cli.html)、[Audiveris 安装文档](https://audiveris.github.io/audiveris/_pages/tutorials/install/binaries)。

三态：**支持**（Basic Pitch 首选成立：Apache-2.0+CLI+.mid 直出，与既有 midi 导入链零改造对接）；OMR 的 MusicXML→MIDI 转换步**需新开面**（小）；AGPL 合规姿态需 DESIGN 固化（见冲突事实 6）。

## §2 生成集成通路

### 2.1 Suno 现状复核——用户裁定成立

2026-09-29 复核一致：**Suno 无公开官方 API**。内部端点 `studio-api.suno.ai` 已 503 关停（击穿一批第三方工具，[OmniRoute issue](https://github.com/diegosouzapw/OmniRoute/issues/14224)）；社区反代方案存续（[Suno-API（Golang）](https://github.com/Suno-API/Suno-API)、[gcui-art/suno-api（Next.js）](https://github.com/gcui-art/suno-api)）；聚合/中转站生态在（[musicapi.ai 经销 Suno/Udio](https://musicapi.ai/udio-api) 等）。即用户裁定（自建反代 vs 中转站采购，自有可行方案）是唯一路径，且**接口面无稳定性承诺**——断点监控与兜底链是硬前置。凭据面（鉴权/本地隔离）为自建面工作量。

三态：**不支持**（官方 API 型适配器对 Suno 不存在）；会话型方案**需新开面**（反代部署/采购+凭据管理+监控）。

### 2.2 提供方适配器两型——官方 API 型候选实勘

| 提供方 | 形态 | 计费/成本 | 能力边界 | 风险 |
|---|---|---|---|---|
| **MusicGPT** | 官方 REST API | PAYG $0.10/曲起；Plus $14.99/月 $0.05/曲 | 常规整曲 | 低；接口最简（[docs](https://docs.musicgpt.com/api-documentation/index/introduction)、[定价](https://musicgpt.com/api)） |
| **Stability Audio** | 官方 REST（api.stability.ai v2beta） | 按量 | Stable Audio 2.5/3.0 在役（3.0 最长 6 分钟）；**2.0 已于 2024-10/2025-04 废弃** | 版本流失先例（[release notes](https://platform.stability.ai/docs/release-notes)） |
| **Gemini（Lyria）** | 官方 API（Gemini API） | 预览计费 | Lyria 3.5 public preview：44.1kHz 立体声、文本/图像提示、含人声整曲；Lyria 2 前代；RealTime 实验 | 预览期变动（[音乐生成文档](https://ai.google.dev/gemini-api/docs/music-generation)、[Lyria 3 公告](https://blog.google/innovation-and-ai/technology/developers-tools/lyria-3-developers)） |
| **MusicGen 自托管** | 本地 CLI（audiocraft） | 免费（硬件） | 代码 **MIT**、权重 **CC-BY-NC 4.0（非商用）**；无真实人声、英文提示、偏短片段 | 离线可用；毕设/研究用途合规，商用需换权重 |

三态：**支持**（四候选皆有可编程面）；适配器统一接口建议"提交提示词/参数 → job 轮询 → 产出本地音频文件"——与内核已有 AIGC job 台账（见 2.5）天然对齐。出站网络面现状=刻意仅回环（pluginprobe CLI 强制 loopback `agent/cmd/pluginprobe/main.go:17-21`；全仓无出站 http.Client，唯一 client 在 processorauthority 指向 agent 自身 127.0.0.1:7878 `local_processor_certification.go:19-38`）——**受控出站面需新开面+toolpolicy 分级**（见冲突事实 4）。

### 2.3 兜底链（适配器失败→外部导入）——支持（可用形态确认）

链路：外部浏览器下载 → 文件落 Downloads → `/agent/downloads/scan`（`agent/internal/chat/server.go:578,834`，按 mtime since 30min）或 `/agent/downloads/watch`（`:579,853`；poll 1500ms `agent/internal/resourceintake/intake.go:128-182`）→ 生成 artifact（source=downloads）→ 前端媒体池预览（`side_panel_request` server.go:810）→ 用户选中成为 `selected_library_file_path` → `clip.import_media_to_track`（catalog.go:859-860）/stems 批量 `project.import_folder_as_stems`（:861）。

- 刷新机制现状：全命令触发式、无索引文件——Places 根每次现扫 `filepath.WalkDir`（上限 6000 文件/深度 8，`harness.go:9975-10020`），**对新文件天然可见**（外部下载的生成物无需任何刷新动作即可被库搜到）。
- 缺口：无"artifact→自动放轨"直连工具（必须经前端选中或显式路径）；agent 自取网络音频不可用（web 工具纯文本 512KB 上限，`agent/internal/tools/webtools.go:36`；二进制上传走 `/agent/artifacts/upload` 64MB，server.go:1359）。

三态：**支持**——"系统不硬依赖 Suno 可用性"的兜底前提今天已成立；自动化直连属增强项**需新开面**（小）。

### 2.4 封装为 VST3 插件的技术面——不支持（作为 v1 通路）

- 现有 VST3 面=观察宿主（pluginprobe 白名单仅 load/snapshot/render/unload 四只读操作，`agent/internal/vst3host/worker.go:194-201`，拒绝一切参数写）+ 产品 rack（只建 `te::ExternalPlugin`）。"把生成引擎做成 VST3 插件"=新造 .vst3 二进制（插件内网络调用、宿主兼容矩阵、离线渲染/实时两模式），且与既有链收益错配：
  - PCA 语义是"对已安装效果器二进制的参数控制"：processorintent families 全为效果器（`agent/internal/processorintent/intent.go:32-42` 硬编码 9 family）；registry 要求确定性 Recognizer/Planner/Materializer/TypedExecutor/ReceiptProjector（`agent/internal/processorregistry/registry.go:15-29`）；attestation 按二进制指纹签发（`agent/internal/processorattestation/eligibility.go:36,68`）。generator 语义需横跨 intent/registry/attestation 三包新增 family 并重写 coverage 证明模型。
  - 乐器已被认证排除（1.3）；插件内网络调用与"仅回环"安全姿态冲突（2.2）。
- **现成的非 VST3 通道**：文件式 AIGC 资产管线（2.5）——"外部引擎产出文件 → 入池 → 插 clip → 正常参与 tracktion 渲染"已跑通。

三态：**不支持**（v1 放弃该通路；若 v2+ 需"宿主内参数联动"再评估）。

### 2.5 agent 触发接入点 + 生成驱动 CLI 公共底座——支持（模式齐备）

- **新本地工具注册成本**：3–4 处——catalog spec（签名 `catalog.go:713`；先例 `render_profile_bind` `catalog.go:947-948`）+ invokeLocal case（`harness.go:2453-2458`）+ 实现文件（`agent/internal/harness/render_profile.go:25`）+（可选）chat 持久化（`chat/server.go:169/:7023`）。模型可见性由 `Catalog.ModelSummary`（catalog.go:274）自动收编。
- **外部进程先例（CLI 底座模板）**：① shelltools allowlist（`agent/internal/shelltools/shelltools.go:14-20` 允许前缀+`:45` 超时+`:74-77` 禁元字符；工具 `shell_run` catalog.go:783 RiskConfirm）；② vst3host worker（崩隔离子进程 `exec.Command`+stdio JSON-RPC，`agent/internal/vst3host/worker.go:78-88,:138`）；③ pluginprobe（观察-only 宿主适配器，`agent/internal/pluginprobe/adapter.go:11-25`）。生成驱动 CLI 底座可仿 worker 模式做成独立二进制：agent 嵌入（工具 exec+轮询）与未来任何插件封装共用同一 CLI——两前端底层同构（用户 2026-09-29 判断成立）。
- **生成物落地面（内核已备）**：
  - `bridge_ingest_generated_asset`（`VitApp/Source/Service/GeneratedAssetService.cpp:160-264`）：入参 file_path/track_id/bucket/job_id/start_time/asset_kind/ghost_state/take_id；校验音频可读非零长 → `VitMediaPoolManager::ingestGeneratedAsset`（bucket 缺省 **"Generated"**，`VitMediaPoolManager.h:20`）→ `insertWaveClipWithUndoAndStartBake` 插轨 → **warp 元数据 originBpm/originKey/warpTargetBpm/warpMode**（`VitWarpBridgeNode.cpp:17,:40`，clip 落 `vit_origin_key` 等属性）→ take 栈+ghost 态（`appendTakePropertiesToClip`）→ 存盘。agent 工具名 `assets.ingest_generated_asset`（catalog.go:942）。
  - 作业台账 `aigc_register_job`（catalog.go:941）→ `JobEventService.cpp:34-61`；记录结构 `VitApp/Source/Core/VitAIGCJobRuntime.h:12-22`：jobId/nodeId/**jobState(idle…)**/**ghostState(bypass)**/statusMessage/requestHash/**source**/assetRef/trackId/clipId，随工程持久化（upsert/attachImportedAsset/setGhostState/snapshotProjectJobs）。
  - take/ghost 操作面：`assets.switch_take`、`assets.set_async_ghost_state`（catalog.go:943-944）。

三态：**支持**——适配器→job 台账→ingest→插轨的完整落地管线已在（这正是"生成集成面"的现成地基）；CLI 底座两前端共用有成熟模板。

## §3 资料库现状

### 3.1 REALSTEMS 澄清与三层素材设施

**REALSTEMS 不是代码设施**——全仓 `realstems` 仅命中流程文档（`coord/cards/done/2026-09-23-PORT-REALSTEMS-MAC-1.md:5`：真实素材由用户从 PC 转交、绝不入仓；`coord/decisions/2026-09-23-realstems-eq1-p1-and-registrations.md`）。真正的素材设施是三层：

1. **DAW 内建 library（"Places"）**：根路径由前端 requestContext 逐次传入（`harness.go:9947-9972` `importSearchRoots` 收 `search_roots/library_places/places/…`；无根报错 `:9975`）；agent 无库根配置/env/常量。
2. **agent artifacts 素材池**：JSONL 追加清单（`agent/internal/artifacts/store.go:70` O_APPEND），root 在 `roots.Agent/artifacts` 或 `VitApp/Workspace/<DefaultDirName>`（`store.go:32-40`）；登记入口 `RegisterAssets`/`IndexAuthorizedFolder`（`agent/internal/resourceintake/intake.go:184,:217`）+对话路径自动登记（`chat/media_reference_artifacts.go:21`）。
3. **下载目录监视**：`ScanDownloads`（intake.go:70）/`WatchDownloads`（:128）。

三态：**支持**（形态=三层按需扫描+一份 JSONL 清单，无常驻 watcher/无索引 DB——刷新见 §2.3）。

### 3.2 装载链与格式面

- 装载：单文件 `clip.import_media_to_track`/`clip.import_audio`（catalog.go:859-860）；文件夹/stems `project.import_preflight`（:750 只读预检）→ `project.import_folder_as_stems`/`project.import_audio_files`（:861，一文件一轨一 clip 单命令）；prompt 硬规则防逐轨模拟（`chat/server.go:5712`）。执行 VSP 优先/legacy ZMQ 兜底（`harness.go:1535,:1551`；`agent/internal/kernel/client.go:21-27`）。
- 格式：音频 9 扩展名（`harness.go:9672-9682`）+ **MIDI `.mid/.midi` 已在白名单**（`:9684-9687`）；无 NI `.stems` 容器格式（"stems"=分轨 wav 文件夹）。元数据轻量文件级（kind/mime/size/mtime，intake.go:371-391）；深度元数据由 kernel preflight/`media.inspect_files` 返回（catalog.go:750-751）。

三态：**支持**。

### 3.3 生成物落库：已有 AIGC 管线与缺口

- **音频生成物**：全链已通（§2.5）——bucket="Generated" 分桶、warp 元数据（originBpm/originKey/warpTargetBpm）、take 栈+ghost 态、job 台账。
- **MIDI 生成物**：走 `midi.import_file`（catalog.go:883-891）→ insert_midi_clip；`.mid` 已在素材池白名单（3.2）。
- **stems 生成物**（多轨分轨）：`project.import_folder_as_stems` 批量链可用。
- 缺口：① 生成 job 状态到前端专属展示面（"生成中/完成"面板）无专道（ghost_state/switch_take 工具在，UI 面未勘察到）；② `source` 字段取值语义未规约（供哪个提供方/适配器回填，DESIGN 需定枚举）；③ 生成物与"参考/素材"在库内的区分仅靠 bucket 字符串。

三态：**支持**（音频/MIDI/stems 三类生成物落库均已有承载面）；展示面与 source 语义**需新开面**（小，DESIGN 定）。

## §4 工程统一性底座

### 4.1 内核工程参数现状

| 参数 | 内核现状 | 锚点 |
|---|---|---|
| tempo/BPM | **在**：写命令 `set_tempo`；模板默认 120 | `CommandDispatcher.cpp:2514,:3439-3469`（`tempoSequence.getTempo(0)->setBpm`）；`VitPaths.h:20,68` |
| 拍号 | **仅模板写死 4/4**，无 set/get 命令（~140 handler 无 timesig） | `VitPaths.h:21,69` |
| 调性 | **工程级不存在**（全源码无 musical key/scale；仅生成资产级 `origin_key` 自由字符串） | `VitWarpBridgeNode.cpp:17,:40` |
| 小节 | **不存在**（时间单位仅 seconds/beats，无 bar 换算） | — |
| beat↔秒 | **双向在**（toTime/toBeats） | `CommandDispatcher.cpp:1100,:1124-1126`；MIDI 导入即用 `MidiService.cpp:1887-1893` |
| 持久化 | tracktion Edit XML → `.vit`（含 TEMPOSEQUENCE/TEMPO/TIMESIG） | `VitProjectFile.h:9,:17-21`；`VitHeadlessService.cpp:660-688` |
| get_project_state 披露 | **不含 tempo/timesig/key/bar**（主会话亲核） | `CommandDispatcher.cpp:3279-3437` |
| 媒体池 BPM | 资产级 origin_bpm/warp_target_bpm | `VitMediaPoolManager.cpp:62-64,187-189` |

### 4.2 agent 观察面可得性

- **MOM 已披露 tempo**：TimeRuler 含 `tempo_bpm`（`agent/internal/mom/projection.go:79`）。但数据源优先 `req.Args.tempo_bpm/bpm`（前端 requestContext 透传），回退 `req.ProjectState.tempo_bpm`——**该回退分支因内核 get_project_state 不出 tempo 键而空转**（主会话亲核：`agent/internal/mixboard/mixboard.go:1178-1180`；kernel 侧无披露）。即 agent 面 tempo 的事实来源=前端透传，非内核权威。chat 侧 bpm/tempo 仅作 requestContext 安全面透传（`chat/server.go:1881`）。
- **TIM 未披露 tempo**：TIM 的既有披露模式（block_size=kernel audio_settings 直读 `tim/projection.go:741-750`；装载态=plugin_load_state 断言 `tim/asserter.go:410-438`）可平移——tempo 接入只需在装配点 `timInputFromObservation`（`mixboard.go:4035-4067`）传入 TimeRuler/ProjectState tempo 并加 Input/Summary 字段+evidence_ref。
- **调性/拍号**：全 Go 侧零命中——需整链新建（kernel 披露键 → ProjectState → tim/mom Input 字段 → 断言/摘要）。

三态：tempo=**部分支持**（写面+MOM 披露在，内核披露键缺一小步）；拍号/调性/小节=**不支持**（需新开面，工作量集中点）。

### 4.3 生成提示词面板固化工程信息的技术落点

- **VST3 插件内表单**：**不支持**（同 §2.4，无插件封装面）。
- **agent 侧提示词装配**：**支持**——requestContext 已透传 bpm/tempo（server.go:1881）；新生成工具的参数面（2.5 注册模式）即可固化工程信息进提示词；tempo 可从 TimeRuler/ProjectState 读。调性/拍号若要进提示词，先补 4.1/4.2 数据链。

三态：**支持**（agent 侧装配为 v1 落点）；插件内表单不支持。

## §5 agent 侧编曲集成（NL→轨道写入）

### 5.1 落点：既有工具面完整——支持

- **Go agent 已有完整 MIDI 工具族**（`agent/internal/tools/catalog.go:883-891`）：`midi.read_notes / read_clip_data / import_file / insert_clip / create_clip / legacy_add_notes(+bulk) / legacy_mutate_notes / legacy_delete_notes / apply_note_patch`（beat 基 patch）；另有别名族 `midi.write_clip_notes / transpose / quantize…`（catalog.go:129-144）。
- 参数归一与路径解析齐备：`midiOperationFromArgs`（harness.go:9624）、`normalizeMidiPatchArgs`（:9137 区域）、`resolveExistingMidiPath/isSupportedMidiPath`（:9841-9859）、选中/附件 MIDI 注入（:9791-9802）。
- agent 侧无 MIDI 事件生成/变换引擎（所有写转 kernel 命令，agent 只做参数结构化）——**NL→音符 = LLM 在工具调用面直接产 note patch 参数**（`apply_note_patch`），不需要新执行链；风险分级用 spec 现成模式（RiskDirect/Confirm，catalog.go:713）。
- 轨道级操作族齐备（track.add/add_audio/rename/volume/pan…，catalog.go:835-855）；批量编排走 executionports（强 ProjectCut 守卫，`agent/internal/executionports/staticbalance_vsp.go:30-37` 先例）。

三态：**支持**——"按用户语言写入轨道"走既有 proposal/工具执行链即可，无需新工具族；发音链缺口同 §1.3。

### 5.2 新工具注册成本——支持（3–4 处，见 §2.5）

### 5.3 与自由态工作流的衔接——支持（门在容量评估/能力路由）

- 新能力进入路径：工具进 catalog → 自由态会话容量评估（`chat/capability_routing.go:55` 起 `FreeStateCapacityAssessment`）→ 路由出 executable path 才过 **G2**（`agentloop/free_state_gate.go:123`；`audioclosure/phase.go:107-135` PhaseGuard）→ 实验类能力另需 **G8** experiment.Admission；无 executable path → `FreeStateCapabilityBlocked` 终态（`agentloop/free_state_reasoning.go:37`）——REALSTEMS 卡实证过（static_eq 白名单缺口→capability_blocked）。
- 即编曲/生成新工具需同步扩能力路由白名单，否则在自由态会话中被 capability_blocked 挡住。

三态：**支持**（FS 门控机制在）；路由白名单扩容=**需新开面**（每工具一小步，DESIGN 列清单）。

## §6 选型对比总表（供 DESIGN 定 v1 范围）

通路×输入矩阵（工作量：S 小 / M 中 / L 大；依赖=新增外部依赖；风险主项）：

| 输入 \ 通路 | A. agent 对话触发（CLI 适配器） | B. VST3 插件封装 | C. 外部导入（兜底） |
|---|---|---|---|
| **NL 写入轨道**（NL→MIDI） | **S**：工具面已在（midi.apply_note_patch），新增生成编排工具 3–4 处；依赖无；风险低（发音链依赖手动乐器接线） | 不适用 | 不适用 |
| **音频转 MIDI**（Basic Pitch） | **M**：引入外部 CLI（pip 或 npm/ONNX）+worker 模式封装；依赖 Python 3.8–3.10 或 ONNX 运行时；风险低（Apache-2.0） | 不做 | **S**：CLI 手动跑+`.mid` 落 Downloads→导入链已通 |
| **OMR**（Audiveris，v2） | **M–L**：AGPL 文件交换姿态+MusicXML→MIDI 转换步+Java 运行时；风险=AGPL 合规（文件交换可守）+识别质量 | 不做 | **S**：同上手动路径 |
| **整曲生成·官方 API 型**（MusicGPT/Stability/Lyria） | **M**：适配器+凭据管理+job 轮询；依赖=受控出站网络面（新开）+账号计费；风险=版本流失（Stability 先例） | **L+高**：PCA 语义错配需重写三包；网络面冲突 | **S**（已可用，§2.3） |
| **整曲生成·Suno 会话型** | **L**：自建反代/中转站采购+断点监控+凭据本地隔离；风险高（端点关停先例）——**兜底链 C 必须先行** | 同上不做 | **S**（唯一可靠路径） |
| **整曲生成·自托管 MusicGen** | **M**：本地 CLI+GPU；风险=CC-BY-NC 权重（毕设非商用 OK） | 不做 | 不适用（本地即产物） |

落地地基（全部通路共用，已在）：AIGC job 台账（VitAIGCJobRuntime）+ `bridge_ingest_generated_asset` 入池插轨+take/ghost+媒体池 "Generated" 分桶+下载扫描刷新。

**供 DESIGN 拍板的建议选项**（非裁定，执行侧勘察建议）：
1. **v1 必选**：NL 写入（零新依赖）+ 外部导入兜底（已可用）——两者构成"不依赖任何外部平台"的最小闭环。
2. **首个集合商集成**：Basic Pitch（Apache-2.0、CLI、.mid 直出、与既有 midi 链零改造对接）。
3. **整曲生成适配器**：官方 API 型先行（MusicGPT 接口最简 / Lyria 3.5 能力最强含人声整曲）；Suno 会话型放后且强制以兜底链为前置。
4. **不做**：VST3 插件封装通路（v1 明确放弃，避免 PCA 三包重写）；OMR 可列 v2（Audiveris 定位与卡面一致）。
5. **底座补齐优先级**：tempo 内核披露键（小）→ TIM/MOM 工程参数披露 → 调性/拍号链（大，若 v1 提示词需要）→ 乐器自动接线（编曲"可听"的前置）。

## 总三态结论表

| 节 | 勘察面 | 三态 |
|---|---|---|
| §1.1 | 内核 MIDI 数据面（导入/CRUD/轨模型/渲染） | 支持 |
| §1.2 | 前端钢琴卷帘/编辑 | 支持（孤儿命令+菜单禁用=需新开面·小） |
| §1.3 | 虚拟乐器装载 | 部分支持（认证排除+无自动接线=需新开面·决策项） |
| §1.4 | 集成候选（Basic Pitch/Omnizart/OaF/Audiveris） | 支持（首选确认；MusicXML→MIDI 转换步=需新开面·小） |
| §2.1 | Suno 官方 API | 不支持（裁定成立：会话型需新开面） |
| §2.2 | 官方 API 型候选 | 支持（四家有面；受控出站网络面=需新开面） |
| §2.3 | 兜底链 | 支持（可用形态确认） |
| §2.4 | VST3 封装 | 不支持（v1 放弃建议） |
| §2.5 | agent 触发+CLI 公共底座+落地管线 | 支持 |
| §3 | 资料库三层+装载+生成物落库 | 支持（展示面/source 语义=需新开面·小） |
| §4 | 工程统一性底座 | tempo 部分支持；拍号/调性/小节不支持（需新开面） |
| §5 | NL→轨道写入+自由态衔接 | 支持（能力路由白名单扩容=需新开面·每工具） |

## 与设计假设冲突/需设计侧裁定的事实（本卡核心产出）

1. **MIDI"听"面断点**：数据/编辑/渲染能力成熟，但乐器接线无自动链（Z1→Z2 仅 advice，`VitZoneBufferAdapter.cpp:23-31`），乐器被 PCA 认证排除（`processor_certification_entry.go:235`），C2 动态装载硬编码 Effect/Z3（`c2_dynamic_plugin_load.go:213`）。编曲线若要"写入即可听"需裁定：扩乐器认证 / 前端固定乐器链 / 接受静默写入+手动接线。
2. **调性/拍号/小节全链缺失**（工程级）：v1 提示词若要固化调性，工作量集中在内核数据链新建（kernel 字段→命令→get_project_state 披露→观察面）。
3. **tempo 权威源缺位**：内核 get_project_state 不出 tempo 键，agent 面 tempo 事实来源=前端 requestContext 透传（mixboard 回退分支空转）——补一个披露键即闭环，建议列入 DESIGN 底座项。
4. **出站网络面刻意仅回环**（pluginprobe 强制 loopback、全仓无出站 http.Client）是既有安全姿态；生成适配器需新开受控出站面+toolpolicy 风险分级，边界（域名白名单？凭据存储位置？）需裁定。
5. **VST3 封装通路与现有架构错配**（观察宿主+效果器参数控制语义+乐器排除）——建议 DESIGN 明确放弃或推迟，避免投入 PCA 三包重写。
6. **Audiveris AGPL-3.0**：集合商原则下合规姿态（独立进程文件交换、不链接不改性、不分发修改版）需在 DESIGN 固化为硬约束。
7. **MusicGen 权重 CC-BY-NC 4.0**：毕设/研究用途合规，文档需声明非商用边界。
8. **能力路由白名单是新工具的隐性前置**：不扩白名单则自由态会话 capability_blocked（REALSTEMS 实证）——DESIGN 的工具清单须同步列路由扩容项。

## 勘察边界声明

- 只读勘察，零代码改动（`git status` 终态仅本报告与卡面变更）。
- 未跑真实栈：本卡无烟测要求；内核命令面/渲染行为结论来自源码静态勘察（tracktion 委托路径未运行验证）。
- 外部候选事实为 2026-09-29 web 搜索快照（来源链接见 §1.4/§2.1/§2.2），属可变性事实；Suno 端点状态等以当日为准。
- Godot 前端为仓库外工作树，无版本锚，行号以当日文件为准。
- 勘察方法：3 个并行只读 Explore 子代理分域扫描 + 主会话亲核两处承重锚点（tempo 披露链、AIGC 资产管线）；子代理锚点未逐条复跑（抽查一致），若 DESIGN 依赖个别行号建议实现卡领取时复核。
