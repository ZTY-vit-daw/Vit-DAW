# A1-MAC-FORM-1：instrument 装载通道 Mac 形态勘察报告（A1 产品形态用户裁定项的事实供给）

- 执行：GLM-5.3 Mac 执行流（ZCode 会话）/ 2026-10-11 晚
- worktree：`/Users/timozty/Documents/Vit-DAW-a1form` / 分支 `port/a1-mac-form-1` / 领取提交 19cdb0a5（origin/main=cf46f950，coord 卡状态直推 main）
- 性质：只读勘察零代码零真栈运行；方法=①消费 PORT-C1/C3 扫描先例数据与本机语义索引全量核对 ②kernel/agent 两侧代码锚点核证 ③本机插件目录/内容库/合成工具清点
- 领取时 HEAD：cf46f950（worktree 干净，零源码改动）

---

## 一、Mac 插件库 instrument 占比

**数据源**（卡面指定 PORT-C1/C3 先例）：
- 扫描路径先例=PORT-C1（`plugin_scan_paths_darwin.go`：macOS VST3 标准两目录 `~/Library/Audio/Plug-Ins/VST3` + `/Library/Audio/Plug-Ins/VST3`，验收 pass 2026-09-16）；
- 扫描结果先例=本机语义索引 `~/.vit/plugin_semantics.json`（built_at 2026-10-03，source=scan_plugins；即 GEN-CAP-RECON §3.1 所引"Waves-only 719 主体"的同源数据，PORT-C3 语义索引链产物）。

**计数事实**（全量核对，工件 `scan_summary.json`）：

| 口径 | 数 | 占比 |
|---|---|---|
| 主体总数 | 719 | 100% |
| 其中 instrument（`is_instrument=true`，权威口径） | **14** | **1.9%** |
| 其中 instrument（语义分类 `primary_type=instrument`，宽松口径） | 22 | 3.1% |
| 效果器（Fx） | 705 | 98.1% |

- 719 主体**全部** manufacturer=Waves、format=VST3，物理形态=**两个 shell bundle 内嵌多主体**（`WaveShell1-VST3 17.1.vst3` 718 个 + `WaveShell1-VST3-ARA 17.1.vst3` 1 个；每个"主体"是 shell 内一个 VST3 子入口，非独立 bundle）。
- 宽松口径 22−14=8 个为**语义误判**（名称启发式把鼓/钢琴**处理效果器**猜成 instrument）：CLA Drums×2、JJP-Drums×3（类别实为 `Fx|Drums`）、GW PianoCentric×3（`Fx|Effects`）。`is_instrument` 标记双源一致：JUCE descriptor 的 `desc.isInstrument`（kernel 披露 `VspKernelReference.cpp:543`，`PluginRackControlService.cpp` `pluginDescriptionToJson` 直出）+ 类别含 "instrument"（agent `pluginsemantics/index.go:168`），14 个全部 `category=Instrument|Waves`、confidence=100。
- **零鼓源效果器 ≠ 零鼓源乐器**的澄清：`Fx|Drums` 11 个（CLA Drums、JJP-Drums 等）全是鼓总线动态/效果处理，不是发声源。
- **装载/探针先例**：PORT-C3 的 24 个 promoted 主体（`~/Documents/vit-c3-artifacts/pluginprobe_reprobe_mac_20260918-202010/mac_fingerprints.json`：C1 comp/DeEsser/RDeEsser/Sibilance/PSE/L1/L2/C4/C6/LinMB/Smack Attack/TransX Wide ×mono/stereo）**全部是效果器**——Mac 至今零 instrument 实例化记录。
- 扫描面边界：AU 版 WaveShell（`/Library/Audio/Plug-Ins/Components/WaveShell1-AU 17.1.component`）本机存在但**不在扫描面**（扫描=VST3 标准路径；AU 仅在 `audiounit:` 前缀的格式名推断中出现，`PluginRackControlService.cpp:1236`，hosting 未验证）——不改变主体池（同 Waves 生态）。

## 二、候选实列：可作鼓源的 instrument 前三 + 装载到轨命令面现状

### 2.1 候选池（14 个 instrument 全列，工件 `instruments_14.json` 含装载 identifier）

Bass Fingers / Bass Slapper / Clavinet / Electric Grand 80 / Electric200 / Electric88 / Grand Rhapsody Piano（钢琴/贝斯/电钢系）｜CODEX / Element / Flow Motion（合成器系）｜CR8 Sampler / COSMOS Sample Finder（采样器系）｜OVox Instrument（人声合成）｜StudioVerse Instruments（容器/浏览器）。

**无专用鼓机、无 GM 鼓组 instrument**——Mac 插件库不存在"开箱即用鼓源"。按鼓源适配度排序的前三候选（适配度判断=产品形态常识+索引元数据，**均未在本机 VitApp 实例化过**）：

| # | 候选 | 形态 | 作鼓源的适配与代价 |
|---|---|---|---|
| 1 | **CR8 Sampler Stereo**（`VST3-CR8 Sampler Stereo-10456661-c1e44bc9`） | 采样器 | 最接近鼓源形态（采样映射到键位≈鼓 kit）；**但本机零采样内容**（`~/Library/Application Support/Waves Audio` 全目录仅 ~9MB=Central/Local Server/Plugin Server 三件，无任何乐器内容库）——kit 内容需经 Waves Central 另行下载，是否可用未取证 |
| 2 | **Flow Motion Stereo**（FM 合成器） | 8-op FM 合成 | 纯合成无需内容库；FM 打击乐声设计成熟路线（808 系）；但单实例单音色，GM 式整组鼓需逐声部多实例/多轨，编排成本高 |
| 3 | **Element Stereo**（VA 合成器） | 模拟风格合成器 | 同上（包络合成 kick/snare/hat 可行，非 kit 形态） |

备选：CODEX（wavetable 合成器，同 2/3 代价）。COSMOS 是采样**浏览器**（找样可听非轨上演奏源）；StudioVerse Instruments 为容器入口——均不作鼓源前三。钢琴/贝斯/Clav/OVox 与鼓源无关。

**共同未验证项（如实）**：①14 个主体从未在本机内核内实例化（C3 先例仅效果器）——tracktion 宿主对 Waves instrument VST3 的实际装载行为（含 shell 授权：instrument 是独立产品授权，效果器族装载正常不代表 instrument 已授权/非 demo）未取证；②Waves 授权状态本机未取证。

### 2.2 装载到轨的命令面现状（分层如实：kernel 通 / 编排半通 / 准入不通）

| 层 | 现状 | 锚点 |
|---|---|---|
| **kernel 装载原语** | **通道存在且通用**（无 instrument 限制）：`instantiate_plugin`（track_id + plugin_path/plugin_identifier，经 plugin cache 或 VST3 introspection 解析后 `createNewPlugin` 插入轨 pluginList 首位并发布图修订）；`rack_add_node` 同在 | `CommandDispatcher.cpp:3035/:3071`；`PluginRackControlService.cpp:2769-2833`（handler 全文核读） |
| **kernel 槽位语义** | **instrument 槽位结构预留**：rack zone Z1/Z2/Z3，Z2=synth 专属槽（`getZoneIdForNode` 用 tracktion `plugin->isSynth()` 判定回退 Z2，效果器 Z3） | `VitGraphValidator.cpp:30-34/:52-55`；`PluginRackControlService.cpp:802-829` |
| **kernel 渲染预检** | **instrument-ready**：`editHasRenderableAudioContent` 认"MIDI/Step clip + 轨上有 `takesMidiInput()` 或 `producesAudioWhenNoAudioInput()` 的启用插件"=可渲染；MIDI-only 无 instrument → 同步拒 `no_renderable_audio_content`（拒绝文案明说"load an instrument plugin on the MIDI track before rendering"） | `VitProductionCoordinator.cpp:62-92/:693-712` |
| **agent 发现/标记层** | **可用**：`plugin.list/find` + 语义索引三命令完整披露 14 主体（`is_instrument` 直出） | `catalog.go:94-99/:1014-1018`；`index.go:35/168/184` |
| **agent 编排归一层** | **半通**：catalog 有 `plugin.instantiate`（RiskConfirm）与 `plugin.load_to_rack`→`rack_add_node`；harness 归一化对 `rack_add_node` 做 instrument 感知 zone 分配（instrument→Z2，效果器→Z3） | `catalog.go:96/:1020`；`harness.go:9218-9240`（`resolveRackAddNodeZone`/`applyRackPluginZone`） |
| **agent 准入层（PCA 门）** | **结构性无通道——A1 本体**：`enforceAgentProcessorLoadGate` 对 agent 侧四个装载命令名 fail-closed，唯一准入货币=PCA promoted 记录（track/path/identifier/二进制指纹/家族全覆盖匹配），instrument 无家族（processorregistry 七族全效果器）→ 必拒 `pca_load_gate: autonomous full-access load rejected: no_promoted_attestation_matched`；certification token 例外同样家族绑定（V2 族+comp/static_eq）。**此层无平台分支，PC/Mac 同构** | `harness.go:1955-2020`；`processorregistry/registry.go:192-197`；`docs/PCA_PLUGIN_LOAD_GATE_V1.md`（现行契约） |

**第二节结论（验收标准口径）**：
- **已有通道**：发现/标记（双源 is_instrument）、kernel 装载原语（instantiate_plugin/rack_add_node 通用）、Z2 synth 槽、渲染预检 instrument 认定、agent 编排 zone 分配。
- **需新通道**：agent 侧 instrument **准入**（PCA 家族扩展或独立 instrument 门+授权语义）——这正是 A1 待用户裁定的产品形态本体（GM 鼓组=内核内置路线则连 kernel 也需新件，本仓 VitApp/Source 零内置合成器，grep ToneGenerator/4OSC/SamplerPlugin 0 命中）。
- **无候选**：专用鼓机/GM 鼓组插件在 Mac 插件库**不存在**；现有 14 主体中最接近的 CR8 又缺本机采样内容——"装上即得鼓声"的零成本选项没有。

## 三、出声腿路径结论：AIGC-DRUM-GEN 系列在 Mac 的最短可达路径

系列出声腿需求：AIGC-DRUM-GEN-1（pattern→MIDI，数据级收卡不出声）→ -2（多候选**试听**→选定）→ SHOW-REPRO-EXP-1（人检**审听**）。两条可能路径：

### 路径 A：MIDI→instrument→渲染/回放 —— **需新通道（等待 A1 用户裁定）**

- 链路：`midi.write_clip_notes/add_notes_bulk`（Mac 实证 ✅ GEN-MAC-BASELINE-1）→ **挂 instrument（堵点）** → `start_render`（渲染预检 instrument-ready，但 instrument 在轨形态从未实测）→ 试听。
- 堵点=agent 准入层（§2.2 末行），PC/Mac 同构；Mac 另有两项未实证：instrument 实例化行为（含 Waves 授权状态）与"instrument 在轨时渲染预检/渲染"端到端。
- 该路径即使裁定通过也非最短：需 新准入通道 + 装载实证 + （CR8 路线还需采样内容下载）三件事。

### 路径 B：外部生成音频→入库→渲染/试听 —— **已有通道，全链 Mac 实证，最短可达**

- 链路与证据（全部 GEN-MAC-BASELINE-1 run4 30/30 门或 JOURNEY-1-MAC 真栈）：
  1. 鼓声 wav 由 **agent 侧外部生成器**产出（生成动作本体在 agent 侧/外部，符合 GEN-CAP-RECON §1.1 定性）——本机现状：**无现成 MIDI→wav 合成 CLI**（fluidsynth/timidity 均未装；`/Library/Audio/Sounds/Banks` 空目录无 DLS 音色；afconvert 不能合成）。即此环需 agent 自带合成脚本（鼓声=确定性信号生成，可零音色库）或经 A1 通道；
  2. `bridge_ingest_generated_asset`（`assets.ingest_generated_asset`）入库入轨 ✅（双 ingest、asset_ref、take 栈快照）；
  3. `start_render` 真音频腿 ✅（legA 3,175,984B RIFF wav；MIDI-only 同步拒语义同为该 run 实证）；
  4. 试听 ✅（audition 面真栈实证=JOURNEY-1-MAC S5；文件级多候选切换 ✅=`switch_asset_take`，-2 的候选承载现成）。
- **缺口（路径 B 剩余，仅一环）**：鼓 wav 生成器本体（agent 侧新做；MIDI 数据级腿与音频腿之间的"音符序列→音频"转换若要在本机离线完成，现无第三方件可复用——要么自带合成，要么留到 A1 后走 instrument）。

### 第三节结论（验收标准口径）

- **最短可达=路径 B（音频文件腿）**：今天零新通道即可出声——ingest→render→audition→take 切换全部 Mac 真栈实证；代价=声音形态是文件级候选（非轨上可编辑音符源），-1 的 MIDI 数据级产物要出声需先经 wav 生成环节。
- **需新通道=路径 A（MIDI 出声腿）**：agent instrument 准入（A1 本体，用户裁定项）；裁定后 Mac 还需补装载实证（含授权）——但 kernel 侧三件（原语/槽位/渲染预检）结构就绪，新通道收敛在准入层一处。
- **无候选项**：本机无 GM/DLS 音色、无合成 CLI、无专用鼓机插件、CR8 无采样内容——"零新代码纯拼现成件"的 MIDI 出声路不存在。

---

## 工件与锚点索引

- 工件：`coord/runs/A1-MAC-FORM-1/scan_summary.json`（719 主体全量计数+类别分布）、`instruments_14.json`（14 主体含装载 identifier）。
- 先例数据源：`~/.vit/plugin_semantics.json`（本机，built 2026-10-03）；`~/Documents/vit-c3-artifacts/pluginprobe_reprobe_mac_20260918-202010/mac_fingerprints.json`（C3 24 主体）。
- 代码锚点：`VitApp/Source/Service/PluginRackControlService.cpp:2769`（instantiate 通用装载）、`:802-829/:1236`（zone/AU 推断）；`VitApp/Source/Core/VitGraphValidator.cpp:30-55`（Z2=synth）；`VitApp/Source/Service/VitProductionCoordinator.cpp:62-92/:693-712`（渲染预检+拒绝语义）；`VitApp/Source/Service/CommandDispatcher.cpp:3035/:3071`（注册）；`agent/internal/harness/harness.go:9218-9240`（instrument zone 分配）、`:1955-2020`（PCA 装载门）；`agent/internal/harness/plugin_scan_paths_darwin.go:15`（扫描路径）；`agent/internal/tools/catalog.go:96/:1014-1020`；`agent/internal/pluginsemantics/index.go:35/168/184`；`agent/internal/processorregistry/registry.go:192-197`；`VitApp/Source/Service/VspKernelReference.cpp:543`（is_instrument 披露）。
- 依赖消费：`coord/runs/GEN-MAC-BASELINE-1/report.md`（可跑面结论+缺口清单 A1 条）；内核二进制 `/tmp/vit-genbase1-build/app/VitApp_artefacts/Debug/VitApp` sha256 `9a70f064…` 核对一致（在位，本卡未用其运行）。

## 诚实边界

- 只读勘察：零代码、零真栈运行（未起内核/agent；所有"实证 ✅"字样均指 GEN-MAC-BASELINE-1/JOURNEY-1-MAC/C3 先例 run 的既有结论，非本卡复跑）。
- 占比数据=2026-10-03 built 的先例索引（卡面指定数据源），非本卡重扫；若插件库此后变过，数字不自动更新。
- "instrument 装载会被 PCA 门拒"=代码结构核证结论（`enforceAgentProcessorLoadGate` 全文核读+家族枚举），未以真 instrument 发令实测拒信；候选三强的适配度排序含产品形态常识判断（索引只有名称/类别/参数元数据），标注为未实证。
- Waves instrument 授权状态、AU hosting、CR8 内容下载可用性三项未取证（只读勘察不装内容不启授权流）。
