# GEN-CAP-RECON-1：生成能力线现状勘察报告

- 执行：GLM-5.3 flash（PC 执行流）@ D:\Vit_DAW / 2026-10-10 夜
- 基线：origin/main=1c1a447a（领取时）；只读勘察零代码，零 commit 实现
- 方法：仓库源码/卡池/裁定文件只读核 + hub queue 冻存卡只读引用 + SSH 只读核查 Mac 仓（2 次：git 状态/merge-base/构建产物路径）
- 发卡语境：用户裁定三线放行 + Mac=生成/编曲候选待讨论；本报告为该讨论的 Mac 生成线输入

---

## 一、生成链底盘

### 1.1 GeneratedAssetService 能力面（生成什么/命令面/落盘链）

**定性：内核侧不生成任何内容——它是"外部生成音频资产的入库-入轨-多候选管理"服务面。生成动作本体在 agent 侧（LLM 编排）/外部生成器。**

内核命令三条，全部已在 CommandDispatcher 注册（无 K2 型缺口）：

| 内核命令 | 注册锚点 | agent 工具别名 | 风险级 |
|---|---|---|---|
| `bridge_ingest_generated_asset` | CommandDispatcher.cpp:2758 | catalog.go:1062 `assets.ingest_generated_asset` | RiskConfirm |
| `switch_asset_take` | CommandDispatcher.cpp:2763 | catalog.go:1063 `assets.switch_take` | RiskConfirm |
| `set_async_ghost_state` | CommandDispatcher.cpp:2768 | catalog.go:1064 `assets.set_async_ghost_state` | RiskUndoable |

`bridge_ingest_generated_asset` 落盘链（GeneratedAssetService.cpp:160-308，逐步锚点）：

1. 入参校验：file_path/track_id/start_time 必填、目标 AudioTrack 存在、源文件存在且为有效非零长音频（:167-200）
2. `VitMediaPoolManager::ingestGeneratedAsset`：资产入媒体池（bucket 分类、asset_ref、source_hash、relative_path/absolute_path）（:211）
3. `ImportService::insertWaveClipWithUndoAndStartBake`：插 wave clip + undo + bake（:219-228）
4. clip 注解（undo 事务）：`vit_asset_ref/hash/relative_path/bucket/kind/state/lifecycle_state/job_id` + warp 描述符（origin_bpm/origin_key/warp_target_bpm/warp_mode/warp_state，VitWarpBridgeNode::fromRequest）+ take 栈三元组（:119-145、:244-251）
5. saveProject → `VitMediaPoolManager::registerClipReference`（引用计数）→ `VitTakeHistoryStack::addTake`（多候选栈）→ `VitAIGCJobRuntime::attachImportedAsset`（job_id→资产/轨/clip 关联）（:259-277）
6. JSON 回执：含 track_id/clip_id/asset_ref/take_stack_id/ghost_state/warp 五元组 + `generated_assets`/`jobs`/`take_histories` 三快照（:279-307）

`switch_asset_take`（:310-385）：clip_id+take_id → take 栈查证（`VitTakeHistoryStack::getTake`）→ 换源文件引用 → 注解更新（vit_asset_ref/active_take_id/asset_state/warp_state）→ `VitGraphSwapCoordinator::publishGraphChange`（图修订发布，take_switch 类）→ setActiveTake → 回执含 take_histories 快照与图修订八属性。

`set_async_ghost_state`（:387-448）：clip 或 job 维度设置 ghost 状态（异步生成中"幽灵预览"策略，`VitAsyncGhostPolicy::normalise` 归一）→ publishGraphChange（ghost_policy_change 类）。

**配套组件**（Core 侧，include 面实证 GeneratedAssetService.cpp:5-13）：VitMediaPoolManager（媒体池）/VitTakeHistoryStack（take 栈）/VitAIGCJobRuntime（AIGC 作业账本：attachImportedAsset/setGhostState/snapshotProjectJobs）/VitAsyncGhostPolicy/VitWarpBridgeNode（BPM/key warp）/VitGraphSwapCoordinator（图修订）/VitAudioInjectorNode（注入节点）/VitBridgeNode（默认 assetKind/nodeRole）。

### 1.2 MIDI 生成链现状（MIDI-RECON-1 / MIDI-CMD-REGISTER-1 产出后的命令注册面）

- **内核声明面**：MidiService.h 九 handler（handleAddMidiNotes / AddMidiNotesBulk / MutateMidiNotes / DeleteMidiNotes / ApplyMidiNotePatch / GetMidiClipNotes / GetClipData / ImportMidiToTrack / InsertMidiClip，MidiService.h:20-29）。
- **注册面**（K2 修复后，CommandDispatcher.cpp:2609-2664）：add_midi_notes(:2609) / add_midi_notes_bulk(:2615) / apply_midi_note_patch(:2633) / import_midi_to_track(:2639) / get_midi_clip_notes(:2645) / get_midi_clip_data(:2651) / insert_midi_clip(:2657)。K2 补的 apply_midi_note_patch/import_midi_to_track 已在（main 9f53bd4e，2026-10-06 pass）。
- **通道**：写命令经 VSP `legacy.command` 逃生舱可达内核；不带 base_revision 直接放行，无 CAS 面（MIDI-CMD-REGISTER-1 回执）。只读白名单：get_midi_clip_notes/get_midi_clip_data 在 VSP 白名单段（CommandDispatcher.cpp:2221-2222）。
- **agent 侧**：catalog.go midi.* 别名族（midi.write_clip_notes/add_notes_bulk 为 LLM 面别名，翻译层 harness.go:9509-9538 纯 insert patch→add_midi_notes 兼容路径保留）；import_midi_to_track/insert_midi_clip 为 RiskConfirm spec；intent 路由触发词族现役（goalrunner_chat.go:3715 case "midi"、:3794 与 :3867-3868 触发词 midi/音符/旋律/和弦/鼓/量化/转调/力度——MIDI-RECON-1 卡引 :3699 有约 16 行正向漂移，功能在）。
- **真栈验证**：MIDI-RECON-1 双轮（20261006_190707/191932：import→确认→写入→回读一致→混合 op→转调反例→插件清单→rack→authority→render）；K2 烟测 run 20261006_201241 exit 0（import_midi_to_track+apply_midi_note_patch 双 completed，回读数学自洽）。
- **端到端出声两腿现状**：
  - **K1 渲染冻结：已修复**。KERNEL-RENDER-FREEZE-1 取证 pass（ABBA 死锁坐实：完成回调 join 渲染线程 vs 渲染线程收尾 callBlocking 等消息线程）→ KERNEL-RENDER-FREEZE-FIX-1 pass（cherry-pick 4984ef66→main 79b8ecdc；烟测 17 断言组双轮绿+midi_register 回归绿；新内核 sha256 b6565dcf…）。start_render（catalog.go:1065，render.start，RiskConfirm）恢复可用；JOURNEY-1 渲染环节解锁。附加发现：`checkNodesForAudio`/`props.hasAudio` 按全工程判定——无音频内容→静音 render_done 而非失败（对生成线"空轨渲染"语义有影响）。
  - **A1 instrument 装载通道：仍结构性缺失，维持"上交待用户定形态，不立项"**（coord/decisions/2026-10-06-midi-line-brake-and-l11-priority.md 裁定 1）。PCA 家族枚举（processorregistry/registry.go:192-197+）为混音效果器六族（static_eq/broadband_compressor/limiter/gate_expander/de_esser/transient_shaper），**零 instrument/synth/drum 族**。pluginsemantics 有 is_instrument 识别面（index.go:35/168/184，metadata 标记+类别判断，权重 70）——发现/标记层可用，装载通道不在。

### 1.3 素材/stem 生成相关 Service 清点（VitApp/Source/Service/）

- **生成资产链**：GeneratedAssetService / ImportService（insertWaveClipWithUndoAndStartBake；stem 导入 defer_audio_analysis 已改 opt-in）+ §1.1 八个 Core 组件。
- **stem 生成**：**无专门 stem 生成 Service**。stem 相关面=导入链（ImportService；Mac PORT-SMOKE-1/2 的 stems import 已绿）+ OfflineAudioReadCoordinator（离线读并行准备，stem 字样仅注释级）。
- **试听面**：AuditionPreviewService（AUDITION-PLAY-1 现役：audition.select，auto_start 缺省 true，Kernel preview buffer 自含、开预览门不依赖全局 transport——KERNEL_AUDITION_PREVIEW_CONTRACT_V1 现行契约）+ AuditionPreviewAudioPlane/State。
- **渲染面**：start_render → VitProductionCoordinator（K1 修复载体，含看门狗）+ JobEventService（作业事件）。
- **特征/分析面**（生成质量机检可选）：AudioFeatureService/AudioFeatureTypes、L3AcousticAnalyzer、TiledSpectrogramBaker。
- **LLM 工具面齐备度**：assets 三命令 + start_render + midi 九命令全部在 catalog（LLM 可见）；media 域意图路由触发词含 素材/媒体池/.wav/.mid/media/asset/assets（goalrunner_chat.go:3852-3856 add("media")）；midi 域触发词见 §1.2。assets 无专属 case 域（归 media 词表面）。

---

## 二、中枢冻存卡重核

**清单核对**：hub queue `C:\Users\timoz\.zcode\workspace\default\queue\todo\` 2026-10-05 冻结共 **8 张**。卡面所列"五张"实际与生成线直接相关为**四张**（AIGC-DRUM-GEN-1/AIGC-DRUM-GEN-2/SHOW-REPRO-EXP-1/TIMBRE-SWITCH-1）；另四张分属他线：QUANT-LAUNCH-1/2（执行语义：网格预约生效）、TRANSPORT-AGENT-1（对话式传输）、AUTH-RESTORE-LOGSPAM-1（Mac 恢复日志噪声）。其中 QUANT-LAUNCH-1 是 TIMBRE-SWITCH-1 的软联动前置，一并提及。**本卡未动中枢任何文件（只读引用）。**

重核基线（冻结时 2026-10-05 → 今 2026-10-10 的状态差）：①K1 渲染冻结→已修复（79b8ecdc）；②K2 注册缺口→已修复（9f53bd4e）；③A1 instrument 通道→维持待用户裁定（刹车裁定）；④自由态 harness 大幅演进（L1-4/L1-5 收官、HARNESS_V1 落账、执行回执 schema/事件流/needs_experiment 门现役）；⑤三线放行+Mac 线候选（本卡发卡依据，刹车裁定"节目线等 11 月中评估点"出现新的解除通道）。

### 2.1 AIGC-DRUM-GEN-1（鼓点 pattern 生成链，窄面版）

- **卡面目标**：自然语言（固定词表）→ LLM 生成鼓点音符序列 → midi.write_clip_notes/add_notes_bulk → RiskConfirm → 写入鼓轨 → 回读一致；只新做"生成编排"一环。
- **假设逐条核验**：
  - "写入/确认/回读三环现成"——**仍成立，且比冻结时更成立**：K2 补齐注册后真栈 import/apply_patch/回读一致已实证；确认卡/回执/journal 机制现役。
  - "音符映射以 MIDI-RECON-1 结论为准（GM 鼓组或演示工程实际鼓机音源）"——**需修订**：MIDI-RECON-1 结论=挂 instrument 无 agent 通道（A1），A1 至今待用户定形态。生成链的"写入+回读"环（数据级）不依赖 instrument，可先行；**"写入后出声"环在 A1 解决前不可达**。验收面应裁剪为数据级（写入+回读一致），出声腿显式挂 A1。
- **新承载形态建议**：生成编排一环全在 agent 侧（固定 prompt 模板+种子→音符序列→越界 fail-closed），内核零改动；烟测沿用 dev_agent_smoke `-Scenario drum_gen` 参数模式（K2 先例）；执行回执 schema（FREE_STATE_IMPROVEMENT_EXECUTION_RECEIPT_V1）可承载生成参数/种子/候选入账。
- **结论：假设大体仍成立；鼓源小节需修订（出声腿挂 A1）；地基（写入/确认/回读）比冻结时更牢。判定=需修订（小）后可启动。**

### 2.2 AIGC-DRUM-GEN-2（多候选伴奏扩展）

- **卡面目标**：-1 链上加多候选（2-3 个，试听→选定→写入）+ 铺底声部（backing pattern 换参数集）。
- **假设逐条核验**：
  - "候选机制复用 -1 链"——成立（依赖 -1 先行，关系不变）。
  - "候选试听走既有 audition/预览通道；依赖渲染则以 MIDI-RECON-1 第 2 问结论裁剪"——**正向偏差（比冻结时假设好）**：冻结时渲染冻死（K1 未修），今 start_render 已修复+双轮烟测绿；AuditionPreviewService 现役；JOURNEY-1-MAC 的 S5 A-B 试听段实证 audition 面在 Mac 真栈走通。**但鼓候选试听出声仍依赖鼓源 instrument（A1）**；且 K1 修复附加发现提示：无音频内容→静音 render_done（空鼓轨渲染不会失败、只会无声——试听链对无 instrument 鼓轨的行为须在设计时显式处理）。
  - 候选承载形态须明确：GeneratedAssetService 的 take 栈（switch_asset_take）是**文件级**多候选的现成承载（生成音频资产的 A/B 切换）；鼓 pattern 候选是**音符数据级**。两形态不互通，卡面宜写明用哪个（数据级走 MIDI 面+journal；文件级走 take 栈）。若未来"生成的伴奏是音频资产"（外部生成器产 wav），take 栈即正座。
- **结论：假设仍成立；渲染腿从"冻结风险"翻为"已修复可用"；试听出声腿挂 A1；需补"数据级 vs 文件级候选承载"设计决策。判定=需修订（小）。**

### 2.3 SHOW-REPRO-EXP-1（生成链演出级复现实验，LLM 运行纪律卡）

- **卡面目标**：固定 prompt×固定输入 N≥10+种子开放组 N≥5，机检（调性/速度/长度匹配，数据来自 MIDI 回读）+人检审听，产出成功率与时长分布（loop 缓冲设计输入）。
- **假设逐条核验**：
  - 前置 -2 收口——依赖关系不变。
  - 机检数据源（MIDI 回读）——现役成立（get_midi_clip_notes 在 VSP 白名单）。
  - 人检审听——出声依赖 A1（同 2.1/2.2）。
  - §8 运行纪律（次数/成功条件/失败分类/止损四项先行）——与现行 AGENTS §8 完全一致，成立。
  - 实验记录落 queue/reports/——现役路径仍可用；自由态 harness 的 journal/事件流/执行回执 schema 提供了更结构化的新承载选项。
- **结论：假设全部仍成立，无失效前提；承载建议补"回执 schema/journal 复用"一句；出声依赖项与 -1/-2 同源挂 A1。判定=假设仍成立。**

### 2.4 TIMBRE-SWITCH-1（对话式音色切换）

- **卡面目标**："换个更贴合的音色"→候选（本地已装载 instrument 枚举）→试听→确认→量化边界生效。
- **假设逐条核验**：
  - 卡面切片 4 预案："若勘察结论为 instrument 端到端有缺腿：本卡第一腿为补腿"——**缺腿结论已由 MIDI-RECON-1 给出**（agent 侧结构性无通道；PCA 家族零 instrument），但 A1 维持"上交待用户定形态，不立项"（2026-10-06 刹车裁定）。**本卡第一腿实际=用户对 A1 产品形态的裁定**（这是用户决策点，不是执行卡）——卡面"范围待第 2 问结论裁剪"的前提已被消耗。
  - 候选枚举面：pluginsemantics is_instrument 识别现役（发现/标记层）——比冻结时更实；但"已装载 instrument 枚举"依赖先有 instrument 装载能力（同 A1）。
  - 试听：渲染已修（K1），AuditionPreview 通道可用——正向。
  - 量化生效：QUANT-LAUNCH-1 未收口（仍在 hub queue 冻结）——卡面已备"先落即时生效形态"降级路径，仍可用。
  - 切换走确认卡（工程变更不享受免确认）——现役成立。
- **结论：需修订——启动条件从"勘察结论裁剪"变为"等用户对 A1 形态裁定"；候选枚举/试听/确认三腿假设仍成立。判定=需修订（阻塞项在用户裁定，非技术）。**

---

## 三、Mac 面核验

### 3.1 生成线栈面在 Mac 的就绪度

**PORT 系列已覆盖（真栈实证，均为 pass）**：

| 面 | 卡 | Mac 状态 |
|---|---|---|
| 栈形态 | PORT-VSPHUB-1 | 三件套（内核+hub+agent）可用，ws 默认通；两件套烟测体系在位 |
| 素材导入链 | PORT-SMOKE-MAC-1③/-MAC-2② | stems import 绿（4 轨 4 clip、8 jobs queued→running→reopen） |
| DAD shm 面 | PORT-SMOKE-MAC-2④⑦ | POSIX 读取器已修，L3/L2 render probe 真栈绿 |
| 插件面 | PORT-C1/C3/JOURNEY-1-MAC | Waves-only 719 主体扫描+24 promoted（**无静态 EQ 族；装载探针用 C1 comp Mono**）；冷列表需 semantic_build_index 扫描预热 |
| 旅程面 | PORT-JOURNEY-1-MAC | 九断言绿（工程打开→权限→真实 LLM 实验→装载→S5 A-B 试听 audition.*→会话洁净） |
| L2 realtime | PORT-SMOKE-MAC-2⑦ | 绿（FE-L3READY-1 修复后） |

**生成线特定面：PORT 系列 grep 零覆盖**——`ingest_generated/GeneratedAsset/media_pool/take_history/switch_asset` 在全部 PORT done 卡 0 命中。GeneratedAssetService/媒体池/take 栈/AIGC job/MIDI 写命令**从未在 Mac 真栈验证过**。

**关键缺口①：Mac main 缺 K1/K2 修复**（SSH merge-base 实证，2026-10-10 21:2x）：Mac HEAD=1f566a06（card(RLM-PROFILE-2) done），79b8ecdc（K1 修复）与 9f53bd4e（K2 修复）**均非其祖先**。即：Mac 当前基线上，midi.import_file/apply_patch 直发内核会复现 `Unknown command`（K2 前形态），start_render 会复现 ABBA 冻死（K1 前形态）。Mac 线已自推进（Mac 首卡 RLM-PROFILE-2 + CCB-PARAM/MACRO-RECON-1 领取记录在案）。

**关键缺口②：Mac 内核二进制陈旧**：PORT 四卡（SMOKE-MAC-1/2、VSPHUB-1、JOURNEY-1-MAC）统一复用 9-18 23:47 Debug 构建（sha256 fd0d3d84…，VitApp 源码 9-17 0ea259a 后未变）；`VitApp/Export/staging/runtime/VitApp` 在 Mac 仓不存在（Mac 构建产物路径与 PC 不同）。无任何证据表明 Mac 存在包含 K1/K2 的内核构建。

**Mac 插件生态对生成线的影响**：Waves-only 719 主体中 instrument/音源占比**未取证**（PORT-C3/C1 卡无 instrument 字样）。若 Mac 无可用音源插件，A1 在 Mac 的形态比 PC 更窄（装载通道+音源生态双缺）。鼓型生成的"写入+回读"数据级不受影响；出声/试听腿受影响。

**Mac 资源记录**：coord/resources/ 仅 PC-RUNTIME-STACK.md——Mac 栈占用记录未建（协议 §2.2：生成线 Mac 真栈运行前须另建 MAC-RUNTIME-STACK.md）。

### 3.2 Mac 生成线启动前必补清单（按序）

1. **基线对齐**：Mac 仓同步 main（取回 K1/K2 及后续）→ 内核重建 → 新 sha256/mtime 入档（§9 被测二进制纪律）。
2. **生成链 Mac 首勘**：`bridge_ingest_generated_asset` / `switch_asset_take` / `midi.import_file` / `midi.write_clip_notes` / `start_render` 五命令 Mac 真栈各走一遍——PORT 先例表明跨平台风险主要在**脚本/探针腐化层**（SMOKE-MAC-1 七件红中 4 例为跨平台探针腐化，PC 同版必败），内核命令面本身平台差异小，但必须实证。
3. **instrument 生态勘察（A1 的 Mac 形态输入）**：Mac 插件库 instrument 占比+可装载音源实列——A1 本身是用户裁定项，此勘察为其提供 Mac 侧事实。

---

## 四、启动建议输入（若 Mac 选生成）

### 4.1 首批卡建议（2-3 张，供主管决策流裁定）

1. **GEN-MAC-BASELINE-1（新卡，先行）**：Mac 基线对齐+生成链首勘——§3.2 三件事打包（同步 main→重建内核→五命令真栈首勘）；产出 Mac 生成线缺口清单与可跑面结论。这是其余一切 Mac 生成卡的公共前置，且把 PORT 零覆盖面补上。文件域：scripts/（mac 烟测扩展）+coord 工件；内核构建不改源码。
2. **AIGC-DRUM-GEN-1（中枢卡迁移）**：鼓点生成窄面链，按 §2.1 修订后启动——验收面裁剪为数据级（写入+回读一致），出声腿显式挂 A1；生成编排一环全 agent 侧。在 PC 或 Mac 执行均可（若 Mac 执行，前置 GEN-MAC-BASELINE-1）。
3. **A1-MAC-FORM-1（勘察卡，可与①并行）**：instrument 装载通道 Mac 形态勘察（插件库 instrument 占比/候选实列/装载缺腿在 Mac 的具体形态）——为 TIMBRE-SWITCH-1 与出声腿提供 Mac 决策输入；**A1 产品形态本身仍是用户裁定项**，本卡只供事实。

（SHOW-REPRO-EXP-1 按 §2.3 假设仍成立，排 -2 收口后即可跑，不必首批；AIGC-DRUM-GEN-2 依赖 -1。）

### 4.2 中枢卡迁移登记路径（依 PROTOCOL §0/§3）

- **迁移主体=主管决策流**（执行流不迁移——本卡已遵守，未动中枢文件）：
  1. 主管在 coord/cards/todo/ 建迁移卡（按 §5 模板），卡面注明"来源=中枢 queue/todo/<原文件名>（2026-10-05 冻结）+ 迁入依据（三线放行裁定 + 本次 GEN-CAP-RECON-1 重核结论）+ 派发确认状态"；
  2. 中枢原卡移入 queue/done/（或加"已迁出至 coord 池"注记）——**关闭旧池派发入口，禁止双池领取**；
  3. 迁移卡按本次重核结论修订假设段（§2 各卡"需修订"项逐条落入卡面）。
- **Mac 线配套**：生成线 Mac 真栈运行前建 `coord/resources/MAC-RUNTIME-STACK.md`（§2.2 语义）；Mac 执行流领取走同协议（领取提交仅 coord/ 变更推 main）。
- **与在飞 Mac 卡协调**：Mac 线已有 RLM-PROFILE-2（done 待验收）/CCB-PARAM/MACRO-RECON-1 领取记录——新卡文件域须与这些在飞卡不相交。

---

## 五、锚点与工件索引

**代码锚点**：GeneratedAssetService.h:27-29（三 handler 声明）；GeneratedAssetService.cpp:160-308/310-385/387-448（三命令实现）；CommandDispatcher.cpp:2758/2763/2768（assets 注册）、2609-2664（midi 注册）、2221-2222（midi 只读白名单）；MidiService.h:20-29（九 handler）；catalog.go:1062-1065（assets 三命令+start_render spec）、131-146+1002-1005（midi 别名族+RiskConfirm）；goalrunner_chat.go:3715/3794/3852-3868（media/midi 意图路由触发词）；processorregistry/registry.go:192-197+（PCA 六族，零 instrument）；pluginsemantics/index.go:35/168/184（is_instrument 识别）；AuditionPreviewService.h:24-31（AUDITION-PLAY-1 auto_start 语义）。

**卡与裁定**：coord/cards/done/2026-10-05-MIDI-RECON-1.md（pass，三问结论+K1/K2/A1/A2 缺口清单）；2026-10-06-MIDI-CMD-REGISTER-1.md（pass，main 9f53bd4e）；2026-10-06-KERNEL-RENDER-FREEZE-1.md（pass，ABBA 取证）；2026-10-06-KERNEL-RENDER-FREEZE-FIX-1.md（pass，main 79b8ecdc，烟测 17 断言组）；coord/decisions/2026-10-06-midi-line-brake-and-l11-priority.md（刹车裁定：A1 待用户定形态）；coord/cards/done/2026-09-19-PORT-SMOKE-MAC-1/2、PORT-VSPHUB-1、PORT-JOURNEY-1-MAC（Mac 面）。

**真栈工件**：coord/runs/MIDI-RECON-1/20261006_190707|191932/（p01-p17 阶段 JSON）；coord/runs/SMOKE-SCEN-RANGE-1/20261006_201241/（K2 烟测）；coord/runs/KERNEL-RENDER-FREEZE-FIX-1/（修复验证）。

**SSH 只读核查记录（2026-10-10）**：Mac 仓 ~/Documents/Vit-DAW HEAD=1f566a0699449f7de4fb89f190316d780b8d3891；`git merge-base --is-ancestor`：79b8ecdc MISSING、9f53bd4e MISSING；`VitApp/Export/staging/runtime/VitApp` 不存在。

## 六、诚实边界

- 只读勘察零代码零真栈运行（未起 PC 栈、未写 Mac 仓；SSH 仅 2 次只读 git/ls 核查）。
- Mac 内核构建产物的实际路径与最新时间戳未取证（staging 路径不存在；9-18 构建结论从 PORT 卡工件记录推断）。
- Mac 插件库 instrument 占比未取证（无 PORT 卡触及；列为建议卡③的勘察对象）。
- hub queue 卡面"五张"与实际生成线相关"四张"的差异如实记录（§二清单核对）。
- 试听/渲染在 Mac 的 K1 修复后行为未实证（K1 烟测在 PC；Mac 需基线对齐后重验——已列入 §3.2/建议卡①）。

---

## 附录 A：报告定稿时点的范围变化注记（2026-10-10 夜，勘察收尾时出现）

本报告撰写收尾时，main 出现主管两笔提交（47b617cd、15a7264b）：**用户 2026-10-10 裁定 AMV/成片功能并入生成能力线范畴**，VIS-FILM-DESIGN-1 设计卡已入池（契约=frame(t) 纯函数+音频特征数据面/能力面=film 写作 capability+look 式确定性准入门/播放导出=webui 播放器+MP4/分期 v1 锁死；证据=coord/runs/VIS-FILM-OPENFILM-RECON-1）。主管卡面明确"GEN-CAP-RECON-1 第 4 节启动建议合并复核"。

对本报告结论的影响面（如实注记，不改变已勘察事实）：

1. **§一底盘面不受影响**：GeneratedAssetService/MIDI/试听/渲染底盘同样服务成片线（成片=帧序列+音频的渲染导出，其音频腿与生成资产链共底座）；但 **MP4/视频渲染链在本报告范围外**（VitApp 无视频渲染 Service——§1.3 清单可佐证素材面止于音频），VIS-FILM-DESIGN-1 的渲染腿是新建面。
2. **§二冻存卡重核不受影响**：四张冻存卡均为音频生成域，结论维持。
3. **§三 Mac 面新增变量**：成片线的渲染导出（webui 播放器+MP4）在 Mac 的就绪度完全未勘察（webui 内嵌 B7 线遗留+视频编码面均无 PORT 覆盖）——主管合并复核时建议把"成片线 Mac 面"列为 VIS-FILM-DESIGN-1 的分期考量。
4. **§四首批建议合并复核点**：本报告建议①GEN-MAC-BASELINE-1 的"五命令首勘"仍成立且对成片线同样必要（音频腿公共前置）；建议②③与 VIS-FILM-DESIGN-1 的排序（成片设计卡 vs 音频生成窄面链谁先行）属主管池序裁定，本报告不越权预设。

