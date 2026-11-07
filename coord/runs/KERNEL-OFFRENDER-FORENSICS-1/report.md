# KERNEL-OFFRENDER-FORENSICS-1 取证报告——内核离线渲染数字静音（277 seg3a_r2 转卡）

- 执行：GLM L2-worker 子代理（ZCode/PC）@ worktree `D:/Vit_DAW_wt_297forensic` 分支 `port/kernel-offrender-forensics-1`，领取基线 origin/main=a93dd97c（worktree 全净）。
- 性质：源码级取证，零栈零 LLM 零源码改动；seg3a_r2 工件与 277 卡面只读。
- 输入：`coord/runs/ARR-SRCLOAD-1/seg3a_r2_20261107_1020/`（evidence.md/summary/fixture 主文件）+ 277 卡 done 回执段 3a-r2。
- 引擎源码树定位：被测 kernel（wt_228 构建，SHA256 F9E3DA0A…）的 CMakeCache `VIT_TRACKTION_ENGINE_DIR:PATH=D:/Vit_DAW/tracktion_engine`（wt_228/VitApp/build_release/CMakeCache.txt:429 亲读）——本报告全部引擎锚点取自该树（te 快照 b4397498，本地零改动）。本卡 worktree 下 `tracktion_engine/` 为空目录（worktree 不带未跟踪内容），已核非取证输入。

## 0. 事实基线（seg3a_r2，不重复取证）

装载链全实证（rack.add_node ok/plugin 1013/rack_item 1012/track 1007/修形断言全过/clip 1014 十六音回读一致）→ start_render job 780f1b51 → render_done（非 fallback）→ wav 8.0s=352800 frames 全零（1,411,200 数据字节 nonzero=0，python wave 独立核验）。活栈拆栈前探针排除：轨 mute/solo/fader/pan/输出解析干净；rack 图模型 edges 4 条（`RACK_INPUT|0→1013|0` MIDI pin0 + `1013|1|2→RACK_OUTPUT|1|2` 音频 L/R + `1013|0→RACK_OUTPUT|0` MIDI out）、audio_reachable_from_rack_input=true、vit_effective_in_output_path=true、load_state enabled/processing=true load_error 空。8.0s 长度=edit->getLength() 缺省 range 证活 edit 在场。

## 1. 渲染执行路径全链锚点（源码亲读）

### 1.1 VitApp 入口层

| # | 锚点 | 事实 |
|---|---|---|
| A1 | `VitApp/Source/Service/TransportAudioService.cpp:1185-1223` handleStartRender | file_path 必填；bit_depth→jmax(16,·)；缺省 range=[0, edit->getLength()]；useMasterPlugins 缺省 **true**；tracksToDo 传空 BigInteger |
| A2 | `VitApp/Source/Service/VitProductionCoordinator.cpp:828-840` | Leg B 门放行（MIDI clip+轨上 takes-MIDI/enabled 插件在场即 true）——run 实证放行 ✓ |
| A3 | `VitProductionCoordinator.cpp:857-871` | Renderer::Parameters：time=[0,8s]、wav/16bit、useMasterPlugins=true、tracksToDo 空=全轨；sampleRateForAudio=设备现行 44100（:868-872，与产物 44100 一致） |

### 1.2 引擎渲染图构建（离线与实时同一构建器）

| # | 锚点 | 事实 |
|---|---|---|
| B1 | `tracktion_Renderer.cpp:679-717` EditRenderer::render | `ScopedRenderStatus(edit,false)`（渲染开始即 `transport.freePlaybackContext()`，tracktion_Edit.cpp:779-787——释放实时图但**不卸载插件实例**；参数 false=渲染后不重建实时图）→ createRenderTask → 渲染线程循环 runJob |
| B2 | `tracktion_Renderer.cpp:23-54` createRenderTask | cnp.**forRendering=true**、includePlugins=usePlugins（默认 true）、includeMasterPlugins=true、includeBypassedPlugins=false → `createNodeForEdit(edit,cnp)`——**离线图与实时图共用同一 EditNodeBuilder**，forRendering 只跳过 live-input/freeze/输入设备 send 等旁路（EditNodeBuilder.cpp:1146/1282/1424/1511/1605/1837），**不跳过 clips MIDI 源、rack、插件链** |
| B3 | `EditNodeBuilder.cpp:1970-2008` createNodeForEdit(Edit&,) | 轨筛选：仅「output 到 device 且不喂其他轨」的轨（:1984-1993）——探针 output_to_device_resolved=true ✓；`createNodeForTrack:1599` `createsOutput()`=isAudioTrack()（Track.h:118，AudioTrack 恒 true）✓；顶层=SummingNode(轨)→master plugins（useMasterPlugins=true）→master fade→`createRackNode(edit.getRackList())`（:2005 rack 总线节点 join 主 sum） |
| B4 | `EditNodeBuilder.cpp:1418-1521` createNodeForAudioTrack | clips 节点→TrackMutingNode(clips)→（无 listeners 不包 LiveMidiOutput）→**轨 pluginList 迭代**（:1504 createPluginNodeForTrack）→TrackMutingNode(track) →输出。轨无 clips 且有 synth 时的 SilentNode(2) stub 分支（:1459-1473）本场景不适用（clip 在场） |

### 1.3 MIDI 源与透传链（候选1 核查面）

| # | 锚点 | 事实（MIDI 是否通过） |
|---|---|---|
| C1 | `EditNodeBuilder.cpp:663-725` createNodeForMidiClip | MidiClip→LoopingMidiNode(beatsRaw)/MidiNode(seconds)；通道=clip.getMidiChannel()（MidiList 级默认 1——VitApp `MidiService.cpp:828-833` addNote 第 5 参为 colourIndex 非 channel，note 通道随 list，无错配面）；muteLambda 在 audible 时返回 false（不静音） |
| C2 | `EditNodeBuilder.cpp:891-951` createNodeForClips | CombiningNode 按 clip 时间窗（0–8s）包装 |
| C3 | `tracktion_TrackMutingNode.cpp:144-169` | audible（=isAudible，mute/solo 决定，Track.cpp:182-193）→ `pc.buffers.midi.copyFrom` **透传** ✓（探针 mute=false/solo=false） |
| C4 | `EditNodeBuilder.cpp:1277-1324` createPluginNodeForList | 轨 pluginList=[RackInstance("New Rack"), Volume&Pan, LevelMeter] 逐个 wrap；Volume&Pan/LevelMeter 不吃 MIDI |
| C5 | `tracktion_RackInstanceNode.cpp:99-105` | **"Always copy MIDI"（无条件透传）** ✓ |
| C6 | `tracktion_TestNodes.h:459-531` SendNode | process 把 input MIDI swap/copy 到输出（:517-523）——总线信号源 ✓ |
| C7 | `tracktion_TestNodes.h:536-743` ReturnNode | **transform 阶段** findSendNodes 把同 busID 的 SendNode 连接为 input（:660-742）；process copyFrom MIDI（:651）。transform 在渲染 player 的 prepareToPlay 内执行：`NodePlayerUtilities.h:110-135`→`createNodeGraph`（Node.h:779-800）→`transformNodes`（Node.h:437-467）——**渲染图的总线连接会建立** ✓ |
| C8 | `nodes/tracktion_ConnectedNode.h:137-162/231-268` | rack 内插件输入节点：addMidiConnection；process `connectMidi→mergeFrom`（:247-248）✓ |
| C9 | `nodes/tracktion_SummingNode.h:174-192` | MIDI mergeFrom（:188）✓ |

**候选1 结论（源码层）**：离线渲染图的 MIDI 馈入链九个环节语义全部贯通，无「离线不馈 VSTi MIDI」的结构分支或设计边界。

### 1.4 rack 结构层

| # | 锚点 | 事实 |
|---|---|---|
| D1 | `VitApp/Source/Service/PluginRackControlService.cpp:3560-3624` rack_add_node | `ensureRackInstanceOnTrack`（:1152-1208，tracktion 默认 RackInstance）→ `rack->type->addPlugin(plugin,{x,y},autoConnect=true)`；VSTi 为 **rack 内插件**（轨 pluginList 挂 RackInstance）——探针 track_plugin_names=[New Rack, Volume&Pan, LevelMeter] 同构 |
| D2 | `tracktion_RackType.cpp:495-536` addPlugin | autoConnect=canAutoConnect && **rack 空**（本场景首插件 ✓）；建音频 in/out 连线（:524-525）+ **MIDI 连线 `addConnection({},0,p->itemID,0)` 与 `(p->itemID,0,{},0)`（:528-529）**——与活栈探针 edges 4 条**同源同形**（探针读的就是这套 rack connections 的投影） |
| D3 | `tracktion_RackNode.cpp:381-504` createRackNodeConnectedNode | 遍历 `rack.getConnections()`：Input→item 的 (sourcePin==0&&destPin==0)→`destConnectedNode.addMidiConnection(inputNode)`（:428-446）；item→output（:447-463）——MIDI 进/出 VSTi 的连接按 D2 必然在场 |
| D4 | `EditNodeBuilder.cpp:1629-1669` | rack 侧节点=ReturnNode(rackIn)→RackNodeBuilder 链→SendNode(rackOut)→SinkNode；`createNodesForRacks:1648` 仅当 rack 有 enabled instance（探针 rack 节点 enabled ✓）；轨侧 wrap=EditNodeBuilder.cpp:1242-1275（SendNode(rackIn)/ReturnNode(rackOut)/RackReturnNode） |
| D5 | `tracktion_RackInstance.cpp:76-110` | RackInstance 默认 **wet=1.0**/dry=0/针脚 leftTo=1,rightTo=2,leftFrom=1,rightFrom=2（VitApp 未改写）——wet 通道（VSTi 输出）不被哑掉 |
| D6 | `tracktion_RackReturnNode.cpp:105-156` | wet/dry 混合；MIDI always copy wet（:120） |

### 1.5 VSTi 处理链（候选2 核查面）

| # | 锚点 | 事实 |
|---|---|---|
| E1 | `tracktion_PluginNode.cpp:31-50/276-283` | 渲染图构建时（rack 内）PluginNode 构造即 `plugin->baseClassInitialise({0,sr,bs})`——与实时图**共享同一 te::ExternalPlugin 实例** |
| E2 | `tracktion_Plugin.cpp:447-493` baseClassInitialise | sr/bs 变化才重 initialise；渲染 blockSize=Parameters 默认 512（Renderer.h:61），与设备块同量级 |
| E3 | `tracktion_ExternalPlugin.cpp:1140-1191` initialise | 有实例→prepareToPlay（不 release，注释明言 VST3 release 会杀 MIDI bus）；**实例为 null→静默置 isInstancePrepared=false（:1184-1190）不加载**——实例加载只在 doFullInitialisation（:950-974，processing&&shouldLoadPlugin 门） |
| E4 | **`tracktion_ExternalPlugin.cpp:1336-1342` applyToBuffer** | **两道 fail-silent 守卫：`!hasLoadedInstance` / `!isInstancePrepared` → 无日志直接 return**——输出保持输入拷贝（VSTi 无干声输入=数字静音），渲染照常 render_done。**与 seg3a_r2 全部观测（图完整+无错误+8.0s 全零）完全兼容的机理** |
| E5 | `tracktion_PluginNode.cpp:145-273` process | input MIDI 按 subBlock 切片进 midiMessageArray→`applyToBufferWithAutomation`（:229-232）；context.isRendering=isRendering（:285-294）；enabled 才处理（:179，探针 enabled ✓） |
| E6 | `tracktion_ExternalPlugin.cpp:1235-1334/1456-1499` | prepareIncomingMidiMessages（note 追踪/panic）→processPluginBlock→`pi->processBlock(asb, midiBuffer)`——JUCE 标准调用 |
| E7 | `tracktion_NodeRenderContext.cpp:18-34` findAllPlugins | **专门展开 rack 内插件**进渲染插件清单（RackInstance→type->getPlugins）——setNonRealtime/flush 覆盖 VSTi ✓ |
| E8 | `tracktion_NodeRenderContext.cpp:39-151/179-325` | 渲染循环：precount≈0.5s 空块 warm-up（:117/131）；`setAllPluginsRealtime(plugins, r.realTimeRender=false)`→**VST3 setNonRealtime(true)**（Renderer.cpp:345-353）；flushAllPlugins 预热（Renderer.cpp:307-343）；playHead playSyncedToRange 驱动（:216）；leafNodesReady 门（:241-256，不 ready 只会反复重试→watchdog 超时——本 run render_done 正常完成，证明未卡此门）；写盘按时间轴 samplesToWrite（轨空也写满 8.0s 零） |

### 1.6 合路面（候选3 核查面）

| # | 锚点 | 事实 |
|---|---|---|
| F1 | B3 顶层链 | 轨筛选两道门（output-to-device/createsOutput）均过（探针+A3）；SummingNode→master→fade→rack join→渲染顶层节点——无静默丢轨分支 |
| F2 | `tracktion_Renderer.h:54-81` | useMasterPlugins VitApp 传 true（master volume +6dB 先例 KERNEL-RENDERGAIN-1 只缩放非零信号，不产生静音）；dithering/normalise/trim 均关闭 |
| F3 | `VitApp/Source/Core/VitEngineBehaviour.h:14-21` | VitApp 对 EngineBehaviour 仅定制插件扫描/录音目录——无渲染相关 override |

### 1.7 上游佐证

- `tracktion_Renderer.test.cpp:24` TEST_CASE "Renderer single audio track"——**上游渲染测试仅覆盖音频轨**；MIDI clip+synth 渲染仅出现在 freeze 测试（内置 FourOscPlugin）；**「MIDI clip+rack 内 VSTi」离线渲染无上游测试覆盖**。VitApp 侧先例（KERNEL-RENDERGAIN-1 八腿、GEN 域）全部是音频源渲染——**本路径为 Vit-DAW 首次触达**。

## 2. 结论（三选一）

**③ 证据不足并陈上交**——源码级取证无法在三候选间单一钉死，交主管裁定（处置建议见 §3/§4）。

- **候选1（MIDI 馈入）**：源码语义层**结构性排除度最高**（§1.3 九环节贯通+§1.4 rack 连接同源证实）；「离线渲染本不馈 VSTi MIDI=设计边界」的表述**不成立**——forRendering 不剔除任何 MIDI/rack 环节，引擎设计上支持该路径。
- **候选2（VSTi 离线处理）**：**唯一无法从源码排除的候选**——E4 两道 fail-silent 守卫在「渲染时实例恰好未加载/未 prepare」时产生与本 run 全部观测逐项吻合的结果（图完整、无错误、render_done、8.0s 全零）。反证仅间接：渲染后 3 分钟活栈探针 load_state 仍 enabled/processing/ready（若实例曾被卸载，无日志可查——守卫本身无留痕）。次要保留：WaveShell 类 VST3 在 setNonRealtime(true)+独立 prepareToPlay 下的插件特异行为（源码层不可判）。
- **候选3（合路面）**：源码语义层排除（§1.6 两道轨筛选门探针实证通过；顶层链无静默丢弃分支）。

**缺陷定性建议**（供主管参考，非钉死）：若复现腿证实 E4 守卫命中，缺陷定性为**物化缺陷**（fail-silent 面违反本仓 §5.1「fail-silent 面须留痕」入律口径——守卫无任何 push_warning/日志，最小修建议=守卫处加一次性留痕日志+渲染路径上的实例状态断言，修复卡另立）；若复现腿证实插件特异行为，定性为兼容性缺陷（插件黑名单/预热策略）；若 M1 实时对照也静音，则缺陷在共有面（MIDI 馈入链的运行时状态，候选1 复活）——源码层无法预判。

## 3. 最小复现设计（只设计不执行；真栈腿候栈排队另派）

复用 arr_srcload 驱动器（`scripts/arr_srcload_smoke.py`）变体，M1–M3 全免 LLM 直命令路径（仅 M0 复刻 chat，计费口径同 277 ≪¥0.1）。每腿单 run，§8 跑前成文（次数=1/失败分类/止损=单 run）。执行顺序按信息量排序：

- **M0 复现基线**：逐字复刻 seg3a_r2（同 kernel 二进制 wt_228 SHA256 F9E3DA0A…+同 fixture 流程+同 chat 话术）。预期复现全零；不复现→环境/时序敏感（插件实例竞争类），取证方向改运行时观测。
- **M2 rack 二分腿（最大信息量）**：不经 rack——VSTi 直接挂轨 pluginList（先例命令 `instantiate_plugin`，D1 直挂路径）→同 clip→start_render。出声→缺陷在 rack 结构的渲染图运行时（候选1/3 的 rack 子面复活）；仍静音→非 rack 特有（候选2 或共有面）。
- **M3 插件特异性腿**：M2 同构但换 VSTi（如 Surge XT，预检清单在卷）→start_render。出声→Grand Rhapsody/WaveShell 在 non-realtime 大块下的插件特异行为；仍静音→通用路径。
- **M1 实时对照腿（277 未验证过的盲区）**：M0 装载后先实时播放探针（transport play+realtime 电平/l2_render_probe tap）再 start_render。实时出声+渲染全零→「离线 vs 实时」实例状态差异（E4 守卫最强嫌疑）；实时也静音→缺陷在两路径共有面。
- **观测增强（写入腿设计的取证点）**：①渲染期间对 E4 两守卫的命中证据（临时日志腿或修复卡加留痕后复跑——本卡零源码改动不实现）；②记录设备 blockSize 与渲染 Parameters 块（512）是否一致（E2 是否触发 re-prepare）；③渲染前后 `hasLoadedInstance/isInstancePrepared` 等效可观测量（describeExternalPluginLoadState 快照）。

## 4. PD-7 查证结论：journal→.vit 主文件不回写 = by-design

- **产品裁定在卷**：`VitApp/Source/Service/VitWorkingCopyPersistPolicy.h:9-44`（B15，产品裁定 2026-09-11）——「磁盘 .vit=用户最后一次手动 save；**agent 治理变更（VSP envelope）只进版本仓库（blob+commit=Project History/journal），不推进工作副本**」；Godot UI 原生命令仍即时持久化；显式 project.save lifecycle 命令直写。
- **机制锚点**：`CommandDispatcher.cpp:2312-2329` runAutoPersist——GovernanceScope 内（governanceDepth>0）auto-persist 被 defer 并返回 true（注释明言「the deferral is the intended semantics, not a failure」）；:2280-2308 全服务接线 autoPersist funnel。
- **与 277 观测对账**：rack.add_node/insert_midi_clip/add_midi_notes 皆经 VSP envelope（agent 治理）→ defer→只落 journal revision=4；主 .vit mtime 停在 save_as_project（显式 lifecycle 命令直写，10:21:11）——**观测与设计逐项一致，非缺陷**。
- **重开丢 rack+clip 的语义边界**：该状态的恢复入口是 Project History/版本仓库而非主文件；叠加 FIX-START-STATE-RECOVERY-1 用户裁定（2026-10-18 方案一「启动不恢复」），重开主文件确实不含 rack/clip——设计自洽但**用户可感知面**（若期望「agent 改完重开还在」则需显式 save_project 或未来显式恢复入口）——是否增强交主管裁量，非本卡范围。

## 5. 工件与边界

- 本目录（report.md）为唯一工件；源码零改动；未启栈未计费（零 LLM）。
- 引擎源码树=D:/Vit_DAW/tracktion_engine（主工作树，只读；worktree 内同名目录为空已核）。
- seg3a_r2 工件只读引用，未复制未改动。
