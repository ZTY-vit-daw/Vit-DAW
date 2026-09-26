# L2-2-SEG-RECON-1：段落发现层 DSP 前置勘察报告

- 任务卡：`coord/cards/todo/2026-09-27-L2-2-SEG-RECON-1.md`（本报告入库后随回执入 done）
- 执行侧：PC 会话（GLM-5.3，L1 只读勘察档）
- 领取时 HEAD：`297021b1a638f1b7edb8959f5f7f14f020f78cce`（origin/main 已同步）
- 领取时工作树：`M VitApp/Workspace/default_project.xml`（运行时工程状态，非本卡触碰）+ 若干 untracked `coord/runs/*` 工件目录（其他卡遗留）——本卡零代码改动、零触碰
- 勘察日期：2026-09-27
- 路线图锚：`docs/AGENTIC_OBSERVATION_ROADMAP_2026-09-25.md` §D10（发现与命名分离：发现层确定性 DSP，边界=可复核事实；命名层中性 ID S1…Sn + 可竞争假设）

---

## 0. 结论速览

1. **onset 密度：现成可用**（三处存量直出事件/短窗能量），缺口仅是事件 128 上限与密度统计本身。
2. **novelty：部分现成**——能量域 novelty 可用已物化的波形包络瓦片差分；谱域 novelty 需补（L3 帧序列不发布 / 谱瓦片 agent 未物化）。
3. **self-similarity：需补计算**（全仓库零 SSM 存量），但喂它的**帧特征序列生产路径现成**（内核 L3 帧序列已在内存逐帧计算、谱瓦片已按 10ms×336 bin 产出），缺的是序列级发布/物化与 SSM 计算。
4. **计算位置推荐内核 C++**：`AudioFeatureType::SegmentationPrimitives` 槽位已注册（枚举+别名+版本串 `segmentation_primitives.v1`），内核有四项确定性先例（串行分析池/参数限幅/双渲染 determinism 校验/工件 SHA256）；agent 侧无 WAV 解码器且物化链路存在世代更替/prune 不确定因素。
5. **中性 ID S1…Sn 承载首选 marker 树**：`project.markers.apply_section_markers` 命令现成，`section_id`/`source`/`confidence` 字段天然承载"发现层写入、可被命名层假设覆盖"的语义；tracktion ARRANGERTRACK 无命令面（不可达路线）。
6. 三算法均**无"完全无落点"**情形，不触发停止条件；缺口清单见 §6。

---

## 1. 内核侧（VitApp C++）DSP/分析存量盘点

### K1. L3 离线声学分析器——FFT 帧特征序列（与三算法最相关的存量）

`VitApp/Source/Service/L3AcousticAnalyzer.cpp`

- **特征类型**：逐帧 `FrameObservation`（:37-44）= `{startSeconds, endSeconds, rmsDbfs, peakDbfs, bandDbfs[6]}`；6 频段定义 sub 20-60 / bass 60-250 / low_mid 250-500 / mid 500-2k / presence 2-6k / air 6-20k Hz（:112-119）。
- **窗口/粒度**：fftOrder=12 → 4096 点 Hann 窗、**非重叠**（hop=窗长，:772-776）→ ~92.9ms/帧 @44.1kHz（`frameWindowMs`/`frameHopMs` 随发布透出，:632-633）。
- **计算过程**：mono 下混 `0.5*(L+R)`（:840）→ Hann（:843）→ 实 FFT → bin 能量累入频段（:846-864）；输入经 `OfflineAudioReadCoordinator::acquire` 读锁（:729）。
- **确定性设计**：分析线程池**单线程串行**，注释明说"Full-source FFT analysis is intentionally serialized…重复 4GB 工程 smoke 的完成顺序不确定"是其动机（:893-903）。
- **数据去向**：帧序列只在内存；发布时归约为三条 summary JSON——BandEnergySummary（:538-670，含 bands/noise_floor/frequency_time_events/transient_events/band_dynamics）、StereoRelationSummary（:672-695，L/R 能量/平衡/相关）、LoudnessSummary（:697-722，approximate RMS-LUFS）。身份戳 source_revision/clip_revision/render_revision + `dad_l3_offline_analyzer.v1`（:18, stampCommon :459-536）。
- **对三算法的意义**：`analysis.frames` 就是现成的"帧特征序列"（6 频段 dB + RMS + peak @93ms），SSM/novelty 需要的输入**已经在内核逐帧算出来**，只是没有序列级出口。

### K2. L3 瞬态/频段事件（onset 域现成存量）

同文件：

- **瞬态检测**（deriveFineEvidence :263-293）：RMS 帧局部峰判据（当前帧 ≥ 前帧+3dB 且 ≥ 次帧）+ body（后 3 帧均值）/ sustain（第 4-12 帧均值）→ `FineTransientEvent`。发布为 `transient_events`（:613-635）：`onset_seconds/onset_dbfs/body_dbfs/sustain_dbfs/attack_body_contrast_db/sustain_decay_db/window_ms/hop_ms`。
- **频段事件**（:248-261）：频段 dB 局部峰 > p75+4dB → `frequency_time_events`（:590-611），带起止秒+频段范围。
- **硬限制**：两类事件均 **128 条上限**（:248, :254, :263 的 `size() < 128`）；噪底估计 p10 百分位（:216-227）；bandPersistenceRatio 活跃帧占比（:197-214）。

### K3. 谱瓦片烤制器（10ms×336 log-bin 特征场）

`VitApp/Source/Service/TiledSpectrogramBaker.cpp`

- **特征类型**：log 映射谱幅度瓦片，1 frame = **0.01s 固定物理跳步**（:22, :32, :528），4096 线性 FFT bin → max-pool 映射 336 log UI bin（:28-29, 映射 :583-596），1 tile = 500 帧 = 5s，4 通道（RGBA 布局语义）。
- **数据格式**：float32 数组，布局 `(bin * 500 + frame) * 4`（:24, :854-864，对齐 Godot Image 500×336）；SHM 发布段名 `Vit_Waveform_<bakeKey>_g<gen>_<tileIndex>`（:926-932，`createAndMap` 字节数 = tile.size()*sizeof(float)——**载荷是 float 不是 8bit 量化**）。
- **注意（作特征源的偏差源）**：帧级噪门（静音帧整体置零，:666）+ 频段时间平滑（:760）——这两步是为前端渲染做的，作 SSM 特征时静音段相似度会人为抬高。
- **数据去向**：SHM 段（Godot 前端渲染消费）；agent 物化层**不收 spectral**（见 A1）；`audio_feature_data_ready` 事件仍经 VSP 流出（元数据+就绪状态）。

### K4. 波形包络烤制器（4.88ms 短窗能量/峰值）

`VitApp/Source/Service/WaveformEnvelopeBaker.cpp`

- **特征类型**：每帧 6 浮点 `kFeatureStride = 6 // L_min, L_max, R_min, R_max, L_rms, R_rms`（:33）。
- **窗口/粒度**：1 tile = **5.0s**（kTileSeconds :31）× 默认 **1024 帧/tile**（kDefaultFramesPerTile :32，头文件默认参 `framesPerTile=1024`，WaveformEnvelopeBaker.h:23）→ ~4.88ms/帧。
- **数据去向**：SHM 段名 `Vit_AudioFeature_waveform_<bakeKey>_g<gen>_<tileIndex>`（:889-893）；发布状态 JSON 只含**轨道级标量**（rms/peak/crest_db/tile 计数，:393-433）——帧级数据本体走 SHM。命令入口 `warm_waveform_bake`（CommandDispatcher.cpp:2725）；stems 导入自动排队分析（ImportService.cpp:1614-1645）。

### K5. strip silence——能量阈值区域分析（"边界=可复核事实"的协议先例）

`VitApp/Source/Service/ClipService.cpp`

- **特征类型**：10ms 帧（kDefaultStripFrameSeconds=0.010，:35；可调 2.5-100ms，:434 限幅）peak + RMS 双判据（`peak >= thresholdLinear || rms >= thresholdLinear*0.5`，:864-865）→ 活跃段聚合 → **时间区域**（keepSegments/stripRegions，:938-959）。
- **确定性**：默认阈值 -45dBFS/最小静音 0.12s/pad 参数全部限幅（:31-36, :430-434）；输出带 analysisId、clip-local/timeline 双坐标（:434-462）。
- **两段式协议**：`clip.strip_silence.analyze`（事实）与 `clip.strip_silence.apply`（动作）分离，apply **要求传入已确认的 analyze 结果**（:1736-1737）——与 D10 发现/命名分离同构。命令面 CommandDispatcher.cpp:2702-2713。
- **域限制**：clip 域（per-clip 分析），非整曲/轨道域。

### K6. 压缩双取证据——帧包络序列+onset 候选（帧序列持久化先例）

`VitApp/Source/Service/CompressorDualTapEvidence.cpp`

- **特征类型**：帧级 peak 包络序列 `aligned_envelope_frames`（hop 为请求参数 hopSizeSamples，:405）+ onset 候选检测（帧峰局部极大、-48dBFS 底、20ms 不应期、**sample 精确锚点**，:426-441）。
- **数据去向**：**帧序列落盘 JSON 工件** + SHA256 完整性哈希（:423, :447-460），目录 `Workspace/Artifacts/com_evidence/<pairId>/`（TransportAudioService.cpp:1538-1540）——全仓库唯一"帧级序列持久化+完整性"先例。输入 render WAV 落 `%TEMP%/Vit_DAW_CompressorDualTap/`（:1530-1536）。

### K7. render probe 分析 + 双渲染确定性校验（确定性验收的现成范式）

`VitApp/Source/Service/VitProductionCoordinator.cpp` + `TransportAudioService.cpp`

- probe 对离线 render WAV 做 peak/rms/headroom/crest 分析（:104-419），支持频段范围限定（analysisBandLowHz/HighHz/ID，:151, :213）。
- **确定性双渲染校验**：`determinism_rms_delta / determinism_peak_db_delta / determinism_rms_db_delta`（:597-599）——同一请求双渲染逐项 delta 对比的验收模式已在产。
- `l2_render_probe` 入口：TransportAudioService.cpp:1380-1395；render WAV 落 `%TEMP%/Vit_DAW_L2RenderProbe/l2_probe_<uuid>.wav`（:1391-1393，UUID 命名）。

### K8. 特征类型注册表——SegmentationPrimitives 预留槽（实现卡的落点）

`VitApp/Source/Service/AudioFeatureTypes.cpp` / `.h`

- 枚举含 `SegmentationPrimitives`（.h:18），字符串 `segmentation_primitives` + 别名 `segmentation`（.cpp:18, :46-47），产品版本 `segmentation_primitives.v1`（.cpp:85）——**全仓库 grep 仅类型定义两文件命中，零生产者/零消费者**：纯预留槽。
- 同状态的还有 `TimeEnergy`（`time_energy.v1`，.cpp:11, :78）——agent 物化层已声明接受它（A1）但内核无生产者。
- 默认分辨率结构 `AudioFeatureResolution{frameWidth=500, frequencyBins=336, frameSeconds=0.01}`（.h:36-41）；分析版本 `audio_feature.v1.2`（.cpp:70）。
- 烤制分发路由：`AudioFeatureService::requestBake` 按 featureType 分发 → TiledSpectrogramBaker（AudioFeatureService.cpp:166）/ WaveformEnvelopeBaker（:181）/ L3AcousticAnalyzer（:201）。

---

## 2. agent 侧（Go）存量盘点

### A1. SHM 瓦片物化店（agent 唯一的帧级数据入口）

`agent/internal/vsphub/asset_materialization.go`

- `RecordTelemetry`（:59-105）：监听 `audio_feature_data_ready` 事件，**只接受 `waveform_envelope` 与 `time_energy`**（:66-68）→ `readSharedMemoryFloat32(shmName, floatCount)`（:78）→ `AudioFeatureTile{Data []float32, TileIndex, TileStartSeconds, TileDuration, Metadata}`（:85-104）。
- 平台读取器：Windows `audio_feature_shm_windows.go:21-56`（OpenFileMappingW + MapViewOfFile 只读拷贝），darwin `audio_feature_shm_darwin.go`。
- API：`Materialize(request)`（:160）/ `Snapshot()`（:290）；hub 路由 `handleAssetMaterializeLocal`（hub.go:133）。VSP 事件缓存白名单含 `audio_feature_data_ready/tile_ready/track_duration_ready`（vsphub/event.go:243-249）。
- **限制**：时间窗 prune（pruneLocked :312，重放需重新物化）；spectral 瓦片不在接受列表。

### A2. 声学包状态聚合（JSON 消费，非计算）

`agent/internal/acousticpackage/status.go`

- `BuildStatus`（:337-390）消费 mixboard feature snapshot 的 waveform_envelope / spectrogram_tiles / band_energy_summary / stereo_relation_summary / loudness_summary / l2_render_probe / realtime_* 行，产特征就绪状态；L1/L3 分层（:566-567）。

### A3. MOM 频段关系推理（JSON 消费 + 可比性判据）

`agent/internal/mom/frequency_relationship.go`

- `frequencyTrackProfile`（:189-243）读 track 行的 `frequency_evidence`/`band_energy`（内核 BandEnergySummary 透传）→ 频段区域/冲突候选/tonal tendencies/persistence 摘要（:420-565）。
- **测量可比性判据**（:331-370）：tap_point/sample_rate/channel_count/start/end/window_ms/hop_ms/analyzer_version/render_mode 全等才可比——这套键正是段落边界"确定性重放一致"验收需要引用的可比性范式。

### A4. 确定性测试信号生成（WAV 写、非读）

`agent/internal/probeaudio/suite.go`

- 7 类 48k/24bit WAV 信号生成（impulse/log-sweep/multitone/stepped-sine/transient-burst/stereo-phase/tail，:89-104），PCM24 手写编码（:186-247）。

### A5. 执行验证器（JSON 域）

`agent/internal/executionverifiers/harness_acoustic.go`

- VerifyStaticBalance/VerifyPanLayout（:35-69）校验 MOM 关系 JSON 字段（如 effective_static_rms_dbfs），不接触音频样本。

### A6（负结果）. agent 无 WAV/PCM 解码器

全仓库无 PCM 解码函数（probeaudio 只有编码写出）。**agent 侧一切音频 DSP 均不可行，除非新增解码器或走内核命令**。audioclosure（FS0-FS9 状态机，types.go :67-278）与 harness_acoustic 均为 JSON/状态机层，无 DSP。

---

## 3. 数据源可达性

| # | 数据源 | 内核可达性 | agent 可达性 | 备注 |
|---|---|---|---|---|
| D1 | stems wav 原始文件 | **全量离线读现成**（L3 :734-742、strip silence :796-803 直接 AudioFormatReader） | 同机文件系统可读但**无解码器**（A6） | 导入按 copy policy 可复制进工程 Media（ImportService）；采样点级访问=内核现成 |
| D2 | render 结果 | 离线 render 现成（start_render/l2_render_probe） | **仅 JSON**（harness CollectL2RenderProbeBatch，chat/c1_frequency_cleanup_runtime.go:169 等）；WAV 本体在 %TEMP% 未消费 | render 内容确定性有 K7 双渲染校验背书；输入含插件状态，比 stems 输入不稳定 |
| D3 | TOM 波形特征 | — | 轨道级标量（RMSDBFS/PeakDBFS，tom/projection.go:318-319；edit_length_seconds :1302） | **非帧序列**，不能直接喂三算法；TOM 无段内粒度轴 |
| D4 | baker 瓦片 | SHM 发布现成 | envelope 瓦片**可物化**（A1 现成链路）；spectral 瓦片未物化 | spectral 含渲染优化偏差（K3）；SHM 有世代更替/回收语义 |
| D5 | L3 JSON 事件流 | publish 现成 | VSP 事件 → mixboard snapshot → acousticpackage/MOM（现成） | 边界事件若走此通道，下游管线零新建 |

---

## 4. 三算法 × 数据源可行性矩阵

判定口径：**现成可用**（数据+消费链路都在）/ **需补计算**（注明层面与落点）/ **缺数据源**。

| 算法（需要的输入） | D1 stems 原始 | K1 L3 帧序列（93ms，6band+RMS） | D4a envelope 瓦片（4.88ms，min/max/RMS×2ch） | D4b spectral 瓦片（10ms×336 log-bin） | K2 L3 摘要事件 | D2 render 结果 |
|---|---|---|---|---|---|---|
| **self-similarity**（整曲帧特征序列 × 相似度矩阵） | 需补计算（内核可直接算；agent 需先补 WAV 解码） | **现成计算、未发布**——`analysis.frames` 逐帧已在内存（L3AcousticAnalyzer.cpp:865-873），缺序列级出口 | 可用但维度低（6 维/帧）：粗粒度 SSM 可行（A1 物化后自相关）；重复性检测能力弱于谱特征 | **理想特征源**：336 维/帧@10ms 正是 SSM 文献标准输入；需扩 agent 物化（A1 白名单）或内核直连 | 不够（只有归约摘要，无序列） | 不推荐作主输入（render 成本+输入含插件状态）；FXM 域 A/B 已用此路 |
| **novelty**（帧间距离/能量差分序列） | 同上 | 同上（帧差分=93ms novelty curve，现成计算未发布） | **现成可用**：物化后帧间差分即能量域 novelty（4.88ms 分辨率），纯 agent 侧 Go 计算、零音频接触 | 需扩物化；谱域 novelty（谱通量类）精度最好 | **弱形式现成**：`frequency_time_events`（p75+4dB 谱变化点）+ `transient_events` 即变化点事件；128 上限 | 不推荐（同上） |
| **onset 密度**（瞬态/短窗能量序列） | 同上 | **现成可用**：`transient_events` 直出 onset 事件列表（onset_seconds/dbfs，K2）；密度=事件按窗计数，纯 agent Go | **现成可用**：min/max 帧峰值差分，分辨率 4.88ms 优于 L3 的 93ms | 可用（谱通量 onset 需扩物化） | **现成**（K2 即此） | K6 DualTap onset 候选现成（20ms 不应期、sample 锚点、落盘工件），但绑死压缩探测上下文，可借检测器代码模式、不可直接复用数据 |

**判定汇总**：
- onset 密度：**现成可用**（K2 事件列表 + A1 物化瓦片两条独立路径）；实现卡只需密度统计本身（agent 侧 Go，零音频接触）+ 处理 128 事件上限（G3）。
- novelty：能量域**现成可用**（A1 物化 → 差分）；谱域**需补计算**（内核发布 L3 帧序列 或 扩 A1 物化 spectral）。
- self-similarity：**需补计算**（零存量）；特征序列有三条现成生产路径，推荐内核 L3 帧序列直连（帧已在内存算出，只差发布/在内核内直接算 SSM）。

---

## 5. 计算位置评估（按"确定性重放一致"验收要求）

### 5.1 候选位置对比

| 维度 | 内核 C++ | agent Go | 协作 |
|---|---|---|---|
| 原始音频访问 | AudioFormatReader 全量离线读现成（K1/K5） | 无 WAV 解码器（A6），需新建 | 内核算、agent 消费 |
| 确定性先例 | ①单线程串行分析池（K1 :893-903，动机即重复 smoke 确定性）②参数全限幅（K5 :430-434）③双渲染 determinism delta 校验（K7 :597-599）④工件 SHA256（K6 :447-452） | 物化瓦片是内核确定性产物的拷贝，同机重放一致可达；但供应链含 bake 世代更替、事件到达时机、物化店 prune 三重不确定（A1） | 各取所长 |
| 输入稳定性 | stems 源文件：文件不变则输入不变（source_revision 内容指纹戳现成，K1 :496-504） | 依赖内核 bake 触发与 SHM 存活窗口 | 源文件直读最稳 |
| 发布通道 | audio_feature_data_ready JSON 事件现成（stampCommon K1 :459-536 + VSP 缓存 :245） | 投影层近（peer 拓扑） | — |
| 槽位 | **SegmentationPrimitives 已注册**（K8） | 段落投影是新 peer（拓扑正确） | — |

### 5.2 推荐分界（确定性论证）

**发现层 DSP（SSM/novelty/onset 曲线 + 边界提取）落内核 `SegmentationPrimitives` 槽位；agent 侧段落投影消费边界事件并分配 S1…Sn；命名层假设挂 marker 由 LLM/用户确权。**

确定性论证链：
1. 输入 = stems 源文件字节，身份 = source_revision 内容指纹（现成字段，K1 :496-504）——同一文件重算输入逐位相同；
2. 计算 = 固定常量（fft 窗长/频段表/帧跳步照抄 K1 模式）+ 限幅参数（照抄 K5 :430-434 模式）+ 串行分析池（照抄 K1 :893-903）——无并发顺序依赖、无浮点归约顺序歧义（单线程顺序累加）；
3. 验收 = 同一 source_revision 二次分析 diff 逐项为 0（照抄 K7 determinism_*_delta 模式）+ 边界工件落盘带 SHA256（照抄 K6 模式）；
4. 版本 = `segmentation_primitives.v1` 版本串已注册（K8），算法改动必须升版本——重放一致以版本串为前提；
5. 边界事件经 `audio_feature_data_ready` 发布（schema 模式 `dad_l3_segmentation_primitives.v1`，stampCommon 现成），下游（acousticpackage/MOM 式消费、A3 可比性键引用 window_ms/hop_ms/analyzer_version）零新建管线。

agent 侧自算路线不推荐的原因（记录在案，非禁止）：需新建 WAV 解码器（A6）或依赖瓦片物化（世代/prune 不确定，重放需重触发 bake）；即使数据到手，重放一致性依赖"内核 bake 重放 + agent 计算重放"两级，验收面更长。

---

## 6. 段落/分段概念存量与衔接点

### P1. VIT_PROJECT_MARKERS marker 树（活跃写入面，S1…Sn 承载首选）

`VitApp/Source/Service/ProjectMarkerService.cpp`

- 模型：Edit 状态下 `VIT_PROJECT_MARKERS/VIT_PROJECT_MARKER` ValueTree（:13-14），schema `vit_project_markers.v1`（:41）；字段 marker_id/name/start_seconds/end_seconds/kind/color/**source/created_by/confidence/section_id**（:158-177）。
- `handleApplySectionMarkers`：`sections[]{start_seconds, end_seconds, name, section_id, color}` 载荷；`replace_existing` **按 source 过滤**替换（发现层写入与用户手工 marker 隔离）；undo 事务；确定性 marker_id（hash，:107-113）；name 缺省 "Section N"。
- 命令面五条现成：project.markers.list/upsert/apply_section_markers/rename/delete（CommandDispatcher.cpp:2366-2404）。
- **衔接判定**：`section_id` 直接承载中性 ID；`source="segmentation_discovery"`（建议值）过滤隔离；`confidence` 挂边界置信度；`created_by` 区分写入方。发现层边界 → marker 的写入路径**零新建命令**。

### P2. tracktion 原生 MARKERTRACK / ARRANGERTRACK（弱衔接，不可达路线）

工程 XML 恒有两轨（`VitApp/Workspace/default_project.xml:33-34`）；CommandDispatcher 全表（约 160 条命令）**无 arranger 命令**——arranger 承载路线当前 blocked；marker 树（P1）是唯一活跃写入面。

### P3. clip 结构坐标系 + 两段式协议（D10 协议先例）

clip 的 timeline/clip-local 双坐标转换现成（K5 :434-462）；发现层边界输出 timeline 秒坐标即可与 clip/区间对齐。strip silence 的 **analyze（事实）→ apply（动作，要求已确认的 analyze 结果 :1736-1737）** 两段式正是 D10"发现（事实）/命名（解释）分离"的协议级先例，段落投影可沿用同一协议形状。

### P4. TOM 投影粒度（新增轴而非复用）

TOM 当前只有轨道级长度标量（edit_length_seconds，tom/projection.go:1302），无段内粒度轴——段落投影（L2-2-SEG-1）将为观察层**新增**粒度轴；TOM 的 disclosurePlan/evidenceRefs 模式（tom/projection.go:217-232）可仿。段落投影作为新 peer 注册进 `docs/OBSERVATION_PROJECTION_MANIFEST.md` 是实现卡的事。

---

## 7. 缺口清单（供 L2-2-SEG-1 实现卡排卡）

| # | 缺口 | 锚点 | 影响 |
|---|---|---|---|
| G1 | SegmentationPrimitives 槽零实现（枚举+版本串已注册） | AudioFeatureTypes.cpp:18,46,85 | 实现卡主落点 |
| G2 | L3 FrameObservation 帧序列不发布（内存丢弃） | L3AcousticAnalyzer.cpp:865-873 | 内核内直算 SSM 则无需发布序列；agent 算则需扩发布 |
| G3 | transient/frequency events 128 事件上限 | L3AcousticAnalyzer.cpp:248,254,263 | 长曲（>~2 分钟密集瞬态）onset 列表截断；密度统计需上限参数化或密度直出 |
| G4 | agent 无 WAV 解码器 | probeaudio/suite.go（仅编码） | agent 自算路线前置缺口（不推荐路线，记录在案） |
| G5 | spectral 瓦片 agent 未物化 + SHM 世代/prune 供应链 | asset_materialization.go:66-68, :312 | agent 侧谱特征消费的前置 |
| G6 | spectral 瓦片含渲染优化（噪门置零+时间平滑） | TiledSpectrogramBaker.cpp:666,760 | 作 SSM 特征源时静音段相似度被人为抬高 |
| G7 | ARRANGERTRACK 无命令面 | CommandDispatcher.cpp 全表 | arranger 承载路线 blocked；走 marker（P1） |
| G8 | strip silence 是 clip 域非整曲/轨道域 | ClipService.cpp:796-880 | 边界语义/参数限幅/两段式协议可复用，分析域需扩 |
| G9 | 中性 ID 分配器（S1…Sn 编号策略）无存量 | —（纯 agent 侧逻辑） | 段落投影内实现；与 marker section_id 对齐规则需在实现卡定义 |

---

## 8. 验收对照（卡面五条）

| 卡面验收 | 本报告 |
|---|---|
| ① 存量盘点覆盖 agent+内核 ≥8 个分析能力 | 内核 8 项（K1-K8）+ agent 5 项（A1-A5）+ 1 项负结果（A6），共 14 条，每条带文件:行 |
| ② 三算法矩阵每格有具体数据源判定 | §4 矩阵 6 数据源 × 3 算法逐格判定，判定落到具体函数/字段 |
| ③ 计算位置评估含确定性论证 | §5：五步确定性论证链（输入指纹→固定常量+串行池→determinism delta→版本串→事件通道） |
| ④ 段落概念 ≥2 衔接点 | §6 四个（P1 marker 树含 section_id/source/confidence、P3 clip 坐标系+两段式协议为主衔接；P2/P4 为边界说明） |
| ⑤ 报告入库 | 本文件 `coord/runs/L2-2-SEG-RECON-1/SEGMENT_DSP_INVENTORY.md` |

停止条件检查：三算法均无"完全无落点"（onset 现成可用 / novelty 部分现成 / self-similarity 需补计算但特征序列生产路径现成），不触发；缺口以 G1-G9 上交，未硬凑可行性。

零代码改动声明：本卡全程只读勘察，未修改任何源码/测试/脚本；工作树变化仅为本报告与卡面回执（coord/ 下）。
