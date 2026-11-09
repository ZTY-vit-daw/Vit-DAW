# recon-design.md — WAVESHELL-OFFRENDER-FIX-1（309）取证腿（零栈）

- 执行：ZCode GLM-5.3 L2-worker PC 子代理（309 取证腿）@ worktree `D:/Vit_DAW_wt_309recon` 分支 `port/waveshell-offrender-fix-1`，领取基线 origin/main=8c68c186。
- 性质：**零栈、零 LLM、零源码改动、纯读档**。无 run ID，无计费。
- 输入：301 工件（`D:/Vit_DAW_wt_301repro/coord/runs/KERNEL-OFFRENDER-REPRO-1/`，只读）+297 报告（`D:/Vit_DAW_wt_297forensic/coord/runs/KERNEL-OFFRENDER-FORENSICS-1/report.md`，只读）+引擎源码树 `D:/Vit_DAW/tracktion_engine` @ 60ab1a49（=te 快照 b4397498+301 插桩独立提交，只读）+VitApp 源（主仓只读）。

## 0. 取证方法清单

1. 引擎渲染循环 non-realtime 全链源码亲读：tracktion_Renderer.cpp / tracktion_NodeRenderContext.cpp / tracktion_PluginNode.cpp / tracktion_Plugin.cpp / tracktion_ExternalPlugin.cpp / vendored JUCE（juce_AudioProcessor.cpp、juce_VST3PluginFormatImpl.h、VST3_SDK ivstaudioprocessor.h）。
2. VitApp 渲染入口层参数面亲读：VitProductionCoordinator.cpp / TransportAudioService.cpp / CommandDispatcher.cpp 命令面。
3. 301 工件观测对账：M2 内核日志实况设备参数（44100/**bs=480**）、WaveShell 共享容器证据（Grand Rhapsody 挂 WaveShell1-VST3 17.1_x64.vst3）、guard 零命中与链路绿证据（evidence.md §3）。
4. 实时电平观测面勘察：LevelMeterPlugin/LevelMeasurer API、ZMQ 命令面（play/stop/seek/start_recording 等）、发布通道——为 M1 设计取证。
5. 零 LLM 声明：本腿无任何模型调用（无 telemetry 文件产生）；零栈声明：未启动任何进程，VitApp/tracktion_engine 树只读未触碰。

## 1. 核心发现：离线渲染对 VST3 的 processMode 信令链（S1 主嫌疑机理，全链钉死）

### 1.1 观测基线（301 工件，不重证）

- M2 内核日志：设备=「扬声器 (USB AUDIO CODEC)」Windows Audio，**sampleRate=44100.00 bufferSize=480**。
- VitApp 渲染参数（`VitProductionCoordinator.cpp:857-871`）：`sampleRateForAudio`=设备实况 44100（:868-871）；**`blockSizeForAudio` 不设置**→`tracktion_Renderer.h:61` 默认 **512**；**`realTimeRender` 不设置**→`Renderer.h:80` 默认 **false**。
- Grand Rhapsody Piano Stereo=WaveShell 共享容器内子插件（M2 日志 file_or_identifier=`C:\Program Files\Common Files\VST3\WaveShell1-VST3 17.1_x64.vst3`；引擎对该形态有感知面 `tracktion_ExternalPlugin.cpp:1174` `desc.hasSharedContainer`）。
- 301：两静音腿 VIT301_GUARD 零命中→渲染期 `hasLoadedInstance`/`isInstancePrepared` 均真、`applyToBuffer` 越过守卫直达 `processPluginBlock`（守卫后仅剩 enabled 门 `tracktion_ExternalPlugin.cpp:1369-1375`，301 已证 enabled）→ **`pi->processBlock` 确被调用而输出全零**。

### 1.2 渲染初始化时序（每步锚点亲读）

| # | 时刻 | 事件 | 锚点 | 对插件可见效应 |
|---|---|---|---|---|
| T1 | 装载期 | 实例创建（用**设备** sr/bs 44100/480）；processing→true→装载→首次 prepareToPlay | `tracktion_ExternalPlugin.cpp:1820-1864`（:1834/:1857-1859 设备参数；:1866-1896 创建完成无 prepare）+`:1158-1162` | wrapper 首次 `setupProcessing(processMode=kRealtime)` |
| T2 | start_render 建图 | `createNodeForEdit`（callBlocking 消息线程）→ PluginNode ctor → `baseClassInitialise({44100,512})`；sr/bs 变（480→512）→ `ExternalPlugin::initialise` re-prepare | `tracktion_Renderer.cpp:46`；`tracktion_PluginNode.cpp:31-50/:276-278`；`tracktion_Plugin.cpp:450-461`；`tracktion_ExternalPlugin.cpp:1163-1166` | **再次 `setupProcessing(kRealtime)`**——此刻 setNonRealtime 标志仍为 false（置位在 T3）+ `reset()`（见 T5b） |
| T3 | 渲染 init | `setAllPluginsRealtime(plugins, realTimeRender=false)` → `p->setNonRealtime(true)` | `tracktion_NodeRenderContext.cpp:137`；`tracktion_Renderer.cpp:345-353`；**JUCE 基类实现仅置标志、无任何 VST3 调用** `juce_audio_processors_headless/processors/juce_AudioProcessor.cpp:410-413` | 无 |
| T4 | 渲染 init | `nodePlayer->prepareToPlay(44100,512)` → PluginNode::prepareToPlay **不再触碰插件 prepare**（无 baseClassInitialise） | `tracktion_NodeRenderContext.cpp:138`；`tracktion_PluginNode.cpp:88-132`（全函数无重初始化） | 无——**`setupProcessing(kOffline)` 永不发送** |
| T5a | 渲染 init | `flushAllPlugins`：`ep->reset()` + 至多 20s 静音块 warm-up；VSTi 首块静音输出即 `isAudioDataAlmostSilent` 早断（实际≈1 块）；flush 上下文 `rendering=true`、无 MIDI、playing=false | `tracktion_NodeRenderContext.cpp:139`；`tracktion_Renderer.cpp:307-343`（:319 reset；:325 blocks；:333-335 静音块；:337-338 早断）；PluginRenderContext 参数序 `tracktion_Plugin.h:49-54` | 首个 `process()` 收到 **processMode=kOffline**（静音 buffer，无音符） |
| T5b | 同上 | wrapper `reset()` = `setProcessing(false)→setActive(false)→setActive(true)→setProcessing(true)` 循环，**无 setupProcessing**（且 T2 的 baseClassInitialise 在 sr/bs 变化时同样调 reset——两次循环） | `tracktion_ExternalPlugin.cpp:1207-1215`；`tracktion_Plugin.cpp:488-492`；`juce_VST3PluginFormatImpl.h:2972-2984` | 状态循环但 setup 级 mode 不变 |
| T6 | 渲染循环 | 每次 `processPluginBlock→pi->processBlock→processAudio`：**`ProcessData.processMode = isNonRealtime() ? kOffline : kRealtime`——逐调用置 kOffline** | `tracktion_ExternalPlugin.cpp:1456-1499`（297 E6）；`juce_VST3PluginFormatImpl.h:2636-2672`（:2647-2648） | **setup 级 kRealtime vs per-call kOffline 的不一致信令贯穿整个渲染** |
| T7 | 渲染收尾 | `setAllPluginsRealtime(plugins, true)` 恢复标志 | `tracktion_NodeRenderContext.cpp:161/:196` | 下一次 process 起恢复 per-call kRealtime |

### 1.3 VST3 规范依据（vendored SDK 亲读）

`VST3_SDK/pluginterfaces/vst/ivstaudioprocessor.h:138-143`（ProcessModes 文档注释）：

- kRealtime↔kPrefetch 切换**可在实时线程内 per-call 完成**，插件应检查 `ProcessData.processMode`；
- **切换到 kOffline「requires that the host calls IAudioProcessor::setupProcessing」**。

即：per-call 翻 kOffline 而不重发 setupProcessing，不在规范许可路径上。JUCE wrapper（基类 setNonRealtime 只置标志 + prepareToPlay 早退守卫 `juce_VST3PluginFormatImpl.h:2489-2493`「isActive 且同 sr/bs 则直接 return」+ 逐调用上报 `:2647-2648`）组合出的行为=**插件 setup 级从未被告知 offline，却逐块被告知 offline**。

### 1.4 与 301 三 run 的吻合度

| 观测 | S1 预测 | 吻合 |
|---|---|---|
| M0a2（rack+GR）8.0s 数字静音、data sha 与 seg3a_r2 同款 | WaveShell 对不一致 mode 信令静音输出 | ✓ |
| M2（直挂 GR）仍静音 | mode 信令与挂载方式无关 | ✓ |
| M3（直挂 Surge XT）出声 -13.63 dBFS | Surge XT 开源、对 per-call mode 容错 | ✓ |
| VIT301_GUARD 全零 | 实例在/prepared/process 被调——静音发生在插件内部 | ✓ |
| 无错误日志、render_done 正常 | VST3 `process()` 无返回码，静默输出对宿主合法 | ✓ |
| 渲染图/MIDI 馈入/合路链全绿（297+301） | 与 S1 正交，S1 不依赖图缺陷 | ✓ |

## 2. 嫌疑分列（含次要项）

| 嫌疑 | 机理 | 代码锚点 | 三 run+守卫+日志吻合度 | 判别手段 | 评级 |
|---|---|---|---|---|---|
| **S1 host 信令不一致（per-call kOffline 无 setupProcessing 切换）** | §1.2 全链 | §1.2 表 T1-T7 | 全部观测逐项吻合（§1.4），且是唯一源码层完整钉死时序的机制 | 插桩腿 mode 遥测（§4.2）/一致性化判别实验 | **主嫌疑** |
| S2 WaveShell kOffline 模式真缺陷（即便信令一致也静音） | 闭源不可读；Waves 类离线渲染兼容问题的已知类别 | —（黑箱） | 与 S1 观测不可区分（都只在离线触发） | 判别实验（强制一致 kOffline 或 per-call 改 setup 级 mode） | 次主（黑箱族，含 license/离线检测子形态） |
| S3 reset/setActive 循环状态清洗 | T5b 两次循环若致 WaveShell 壳内状态/程序丢失→哑 | `tracktion_Plugin.cpp:488-492`；`tracktion_Renderer.cpp:319`；`Impl:2972-2984` | 吻合（离线特异：live sr/bs 不变不 reset）；但需壳状态清洗后完全哑才解释 8s 全零，不如 S1 简约 | M1（实时无 reset→出声即弱化）+渲染前后 getStateInformation 状态快照插桩 | 次 |
| S4 flush 预热早断（VSTi 1 块即断） | 壳若需 N 块 process 才完成内部初始化 | `tracktion_Renderer.cpp:337-338` | 中低（8s 全零与「晚起音」期望形态不符，除非壳以完全静音实现未初始化态） | 延长预热实验腿（去早断+泵 20s） | 弱 |

- S1 有两个子形态殊途同归（setup 级都停在 kRealtime）：**S1a** 同 sr/bs 渲染→wrapper 早退守卫 `Impl:2489-2493` 直接吞掉 re-prepare（一般 JUCE 宿主行为，适用任何同速率渲染）；**S1b** bs 变化触发 re-prepare 但发生在 node build（setNonRealtime 置位之前）→ setupProcessing 又发一遍 kRealtime（**301 实测环境 480→512 走此路径**）。
- 停止条件评估：S1/S1a/S1b 的干预面全部在 host 侧可修可观测（vendored JUCE wrapper / tracktion 调用序 / VitApp 参数）；S2 闭源不可修但**不需要修**（修法 B/C 均绕开 kOffline）。**未触发「根因进入闭源不可修面」停止条件**——无需转纯策略裁定；但若 M1+插桩证伪 S1 且 S2/S3 成立，B 仍是有效修法（见 §5）。

## 3. M1 实时对照腿设计（只设计不起栈）

**问题**：GR 实时播放是否出声——分野「non-realtime 特异」（S1/S2/S4 族）vs「WaveShell 更深层问题」（S3/装载面）→决定修法面。

### 3.1 观测面勘察结论（零插桩不可行）

- ZMQ 命令面有 `play/stop/seek/return_to_zero`（`CommandDispatcher.cpp:3206+` handlers 表），`start_recording` 只录**输入轨**非主输出（`TransportAudioService.cpp:1074-1112`）；
- transport 回复无电平字段（`buildTransportReply` `:232-248`），无周期性电平发布通道；
- **但**轨上默认挂 LevelMeter（297 D1），`LevelMeterPlugin::measurer` 为公开成员（`tracktion_LevelMeter.h:46`），`LevelMeasurer::getLevelCache()` 可读左右电平（`tracktion_LevelMeasurer.h:113`）→ **VitApp-only 最小插桩可行**（engine 备选：SharedLevelMeasuringNode 一行 tap）。

### 3.2 腿设计（复用 301 M2 直挂装置最省）

- **装置**：301 M2 fixture verbatim（GR 直挂 track.plugins+16 音 clip 0-8s+gui_project_isolation）；驱动器=扩展 `kernel_offrender_repro_smoke.py` 增加 `--realtime-probe` 模式（play→采样→stop→判读内核日志）；插桩=VitApp 侧 VIT309_RTLEVEL 定时 tap（~250ms 周期读 measurer levelCache，TRACKTION_LOG 同 301 管线），与 §4.2 mode 遥测**同一插桩内核一次栈占用**合并跑。
- **序列**：装载→visibility 预检（复用 M2 断言）→return_to_zero→play→采样 ≥4s（覆盖音符段）→stop→return_to_zero。
- **断言**（py 侧，退出码语义与 301 同型）：
  - RT-SOUND：≥1 条 VIT309_RTLEVEL level_db > -60 → exit 0 + `realtime_verdict:"sound"`；
  - RT-SILENT：全部 ≤ -90 → exit 0 + `realtime_verdict:"silent"`（**两种都是有效判定结果**，非失败）；
  - 环境中断（栈/端口/日志管线空——用 301 同款三重亲证法校验管线）→ exit 3 类环境中断。
- **判定矩阵**：
  - RT-SOUND + 离线静音 → **non-realtime 特异确认**（S1/S2/S4 族；S3 弱化——实时路径无 reset）→ 修法面=离线 mode 信令（B 立即可用/C 待插桩证 S1）；
  - RT-SILENT → GR 本环境任何路径哑 → S3/装载面（license/壳装载）升格，B 单独不再充分 → 需装载取证另立+黑名单纯警告路径。
- **§8 跑前口径**：单 run；零 LLM（直 ZMQ，telemetry 不创建=亲证）；失败分类=环境中断/断言失败/管线失效/判定性结果；止损=单 run 无重跑，同型断言失败两轮先补锚点。
- **栈协议**：候栈排队 **296-R2/310 之后**；净空亲核→登记推 main→≤60s 复核→启栈→TEARDOWN_ALL_ZERO 亲核释放；工件目录 `coord/runs/WAVESHELL-OFFRENDER-FIX-1/m1_realtime_<ts>/`（console+summary+kernel_log+fixture 留盘 gitignore 同 301）。

## 4. 插桩加密观测腿设计（卡面目标 2 的取证件，与 M1 同栈同内核）

### 4.1 遥测点（一次性日志，VIT309 标记，log-only 不改行为）

1. `ExternalPlugin::initialise` 在调 `pi->prepareToPlay` 处记 sr/bs+`isNonRealtime()`（`tracktion_ExternalPlugin.cpp:1158-1166` 旁）；
2. VST3 Impl `prepareToPlay` 发 `setupProcessing` 处记 processMode 值+早退守卫是否命中（`juce_VST3PluginFormatImpl.h:2489-2508` 旁；vendored JUCE 同 engine 子模块提交规则，随 301 插桩同型处置）；
3. 每插件首个 `processAudio` 记 per-call processMode（`Impl:2647-2648` 旁）。

### 4.2 判别力

- 运行时证实「setup 级 kRealtime + per-call kOffline」=S1 链闭环（预期命中 S1b 时序：两次 kRealtime setup + kOffline per-call）；意外见 setup 级 kOffline=S1 证伪→S2 升格。
- **判别实验（可选加做，同一次栈占用）**：临时把 `Impl:2648` 一致化（per-call 上报 setup 级实际 mode）→渲染 GR：出声=S1 证实+S2 排除（且该改动即修法 C 原型）；仍静音=S1 排除+S2/S3 成立。一次实验同时完成判别与 C 案原型验证。

## 5. 修法三案评估（最小干预原则排序）

| 案 | 机理 | 证据支撑度 | 干预面（文件域） | 风险 | 排序 |
|---|---|---|---|---|---|
| **B 黑名单+替代路径**：start_render 时检出 edit 含 WaveShell 类插件（fileOrIdentifier 匹配，如含 `WaveShell`）→ `params.realTimeRender=true`（`Renderer.h:80`）+警告日志 | `setAllPluginsRealtime(plugins,true)`→`setNonRealtime(false)`→per-call kRealtime+setup 级 kRealtime **完全一致**=WaveShell 看到的与实时完全相同；块间停顿（`NodeRenderContext.cpp:220-228`）使渲染按实时速率走 | **强**（对 S1/S2 皆免疫——只要 M1 实时出声 B 即出声；M1 待跑是唯一前置不确定项） | **VitApp-only**：`VitProductionCoordinator.cpp:857-871` ~5-15 行+谓词；零 engine 改动 | 触发时整渲染限速至实时（8s 内容≈8s+ 渲染）；谓词维护（Waves 换壳名）；不修根因；列表外 shell 类插件不受益 | **1** |
| **C 包装器 workaround（host 信令一致性化）**：Impl 增记 setup 级 mode 成员，`processAudio` 上报 setup 级 mode 而非实时 isNonRealtime()（规范对齐：per-call 翻转仅许可 RT↔Prefetch） | 离线渲染时 per-call=kRealtime（与 setup 一致），WaveShell 以实时模式处理——效果同 B 的信令面但**不限速、per-plugin 精度** | 中强（S1 机制源码全钉死，待 §4.2 运行时确证；**S2 成立时 C 无效**——一致 kOffline 仍静音） | vendored JUCE：`juce_VST3PluginFormatImpl.h` ~3-5 行（engine 子模块提交） | 全体 VST3 host 面语义变化（依赖 JUCE 旧行为的插件理论上存在）；vendored 树升级合并面；引擎重编+全包回归 | **2**（兼作 §4.2 判别实验原型） |
| **A 预热策略**：渲染前 dummy process 预热（延长 flush/去早断/预滚 MIDI） | 仅对 S4（预热不足）有效 | 弱（8s 全零与预热不足的「晚起音」形态不符；既有 flush+0.5s precount 未奏效） | `tracktion_Renderer.cpp:307-343` 或 `NodeRenderContext.cpp`（engine） | 渲染耗时倍增；对 mode 信令无修复；S4 为弱嫌疑 | **3**（不推荐单用；其「kRealtime 信号下预热」变体已被 C 覆盖） |

- 强变体剔除说明：C 的「setNonRealtime 变化即重发 setupProcessing」强变体因线程/激活时序风险（setNonRealtime 可在渲染线程调用而 setupProcessing 须消息线程）不做。
- B 的引擎级 per-plugin 变体（`setAllPluginsRealtime` 内对 WaveShell 类跳过）列为 B'：精准但 engine diff，B（VitApp 参数级）更小更稳，先 B。
- 若 M1=RT-SILENT：B 降级为「黑名单+用户可见警告」（渲染照走但显著警告），修法面转向装载取证——M1 是三案排序的前置分野。

## 6. 渲染前自检断言形态建议（301 处置转移项，交主管定形态）

| 形态 | 机理 | 利 | 弊 |
|---|---|---|---|
| 1 渲染前 mode-probe 纯警告 | 渲染前对乐器类插件跑小探针（note-on 短促+~0.2s process+测峰）仅告警 | 提前告知不阻断 | 探针扰动插件状态（注入 note-on）；误报面（合法静音探针结果：长起音/门限类）；内核新增探测代码路径自身要验证 |
| 2 渲染前门禁阻断 | 同上探针，失败即拒绝渲染 | 防白跑 | 误报即阻断合法渲染（最差组合）；同上扰动与代码面 |
| **3 渲染后静音断言+自动降级（推荐）** | render 完成后若 `resultMagnitude`≈floor（`NodeRenderContext.cpp:126/:156-158`/`Renderer.h:92`，或 wav 物理计量——301 py 已有 `wav_max_abs` 同型）**且** edit 含 enabled 乐器插件（producesAudioWhenNoAudioInput）→自动以 `realTimeRender=true` 重渲一次+日志+summary 标记 `degraded_to_realtime_render` | 结果导向（覆盖一切静音成因，非仅 WaveShell）；零探针误报（仅实际全零触发，乐器在场时全零几乎必为异常）；与修法 B 天然协同；信号已存在无需新探测 | 静音时首渲白跑一遍；自动重渲语义需在 reply/journal 披露；RT 也哑时重渲仍静音→需升级为显式错误码 |

**建议**：形态 3 为主 + 轻量提示日志（start_render 检出 shell 类插件时回复附 hint 字段+日志，不探测不阻断）；形态 1/2 待命不默认。

## 7. 裁定请求清单（交主管）

1. **M1+插桩腿合并执行授权**：§3+§4 一次栈占用（同一插桩内核）跑两类观测；候栈排 296-R2/310 之后。
2. **修法形态**：建议 B 先行（VitApp-only，M1=RT-SOUND 即可落地）；C 视 §4.2 判别实验结果定（S1 证实且渲染限速不可接受时上 C）；A 不单用。
3. **自检断言形态**：建议形态 3+提示日志；请裁定。
4. **插桩落点许可**：VIT309 mode 遥测落在 vendored JUCE Impl（engine 子模块，同 301 插桩处置规则——独立提交保留）；VIT309_RTLEVEL 落 VitApp（零 engine 改动优先）。
5. 若 M1=RT-SILENT：B 降级为纯警告、装载取证另立卡的建议是否采纳。

## 8. 端测边界声明

本腿零栈零改动，无端测义务。后续腿（M1/插桩/验证）端测=agent ZMQ 面真栈驱动+内核日志留痕+（渲染腿）wav 物理计量；不覆盖 webui 渲染面与 Godot 旅程（无该域改动）。
