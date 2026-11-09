# WAVESHELL-LOADING-FORENSICS-1 取证报告——WaveShell 装载/license/壳层静音取证（309 M1=RT-SILENT 判别转卡 329）

- 执行：ZCode GLM-5.3 L2-worker PC 子代理（329 取证腿）@ worktree `D:/Vit_DAW_wt_329impl` 分支 `port/waveshell-loading-forensics-1`，领取基线 origin/main=e8ff96c1（worktree 全净）。
- 性质：**零栈、零 LLM、零源码改动、只读取证**。未启动任何进程；Waves 侧全部为文件系统/注册表/二进制静态勘察；引擎树（主检出 `D:/Vit_DAW/tracktion_engine` @ 60ab1a49 与 `D:/Vit_DAW_wt_309recon/tracktion_engine` @ 9b8cc8a/vit309-stackleg）全程只读。
- 输入：三代前情（297 报告 @ port/kernel-offrender-forensics-1、301 evidence @ port/kernel-offrender-repro-1、309 全套 @ 主检出 coord/runs/WAVESHELL-OFFRENDER-FIX-1/）+ vendored JUCE/tracktion 源 + 本机 Waves 安装面。

## 0. 方法清单

1. 三代取证工件归并（只读读档：297 report / 301 evidence / 309 recon-design+RUNBOOK+stackleg evidence）。
2. host 层装载路径源码亲读：VitApp（CommandDispatcher instantiate_plugin / PluginRackControlService）→ tracktion（ExternalPlugin doFullInitialisation/initialise）→ vendored JUCE（juce_VST3PluginFormatImpl.h 全装载链，含本仓既有 `[WavesVST3UI]` 留痕面）。
3. Waves 闭源二进制静态勘察（strings 级，不运行）：WaveShell1-VST3 17.1_x64.vst3、GrandRhapsody.dll、WavesLicenseEngine.dll、InnerProcessDictionary2.dll。
4. 本机 Waves 安装面盘点（只读）：VST3 公共目录、Waves 安装目录、ProgramData（Licenses/Demo Mode/License Notifications/Modules/WavesPluginServer/Central）、注册表、文件 mtime 时间线、license 文件内容。
5. 零 LLM 声明：本腿无任何模型调用、无 telemetry；零栈声明：未启动 VitApp/agent/Waves 任何组件。

## 1. 三代归并表（297→301→309→329 前提）

| 代 | 卡 | 问题 | 方法 | 判决 | 对本卡的意义 |
|---|---|---|---|---|---|
| 297 | KERNEL-OFFRENDER-FORENSICS-1 | seg3a_r2 离线渲染 8.0s 数字静音根因 | 源码级全链取证（渲染图/MIDI 馈入九环节/rack/合路/VSTi 处理链） | 三候选并陈：①MIDI 馈入（结构性排除度最高）②fail-silent 守卫（唯一不可排除）③合路面（排除）；WaveShell 特异行为列为②的次要保留 | 建立全部 host 侧链路锚点；「WaveShell 类 VST3 在 setNonRealtime+独立 prepareToPlay 下的插件特异行为」首次入卷 |
| 301 | KERNEL-OFFRENDER-REPRO-1 | 守卫复现+插件特异性 | VIT301_GUARD 插桩内核+四腿真栈（M0a2/M2/M3） | 守卫零命中→候选②实证排除；M2 直挂 GR 仍静音+M3 Surge XT 出声（-13.63dBFS）→**定性=插件特异性（WaveShell 离线渲染静音）**；rack/合路/MIDI 出罪 | 唯一变量法钉死插件面；实例在+prepared+process 被调而输出全零=静音发生在闭源插件内部 |
| 309 | WAVESHELL-OFFRENDER-FIX-1 | S1 信令机理+M1 实时对照+修法 | 取证腿（S1 时序全链钉死）+栈腿八 run（E1/E1ctrl/E2a/E2b/E3/E4/E5） | **M1=RT-SILENT**（GR 实时也全零，E1ctrl Surge 同装置 -10.97dB 对照）→「non-realtime 特异」推翻，静音归属=装载/license/壳层（S2/S3 族）；S1 信令不一致运行时确证但**非 GR 成因**（E2b C 一致化仍静音）；修法 B warn_only+形态 3 落地；Surge 不回退 PASS | **本卡直接前提**：GR 在四 harness（chat-rack M0a2/M2 直挂渲染/离线渲染/实时播放）一致全零静音——静音必然在「装载后、音频处理前」的壳内初始化/license 面，与渲染模式（realtime/offline）无关 |
| 329 | 本卡 | 静音归属=装载/license/壳层的细化取证 | 本报告：host 装载路径锚定+闭源二进制静态勘察+本机安装面/license 链盘点+三假设判别设计 | （取证腿）新事实见 §3-§4；判别设计 §5；两出口判据 §6 | — |

## 2. 装载路径锚定（shell→shellnode→插件体实例化时序）

### 2.1 host 层（源码亲读，全部可观测/可插桩）

| # | 时刻 | 事件 | 锚点 | 备注 |
|---|---|---|---|---|
| L1 | ZMQ 命令 | `instantiate_plugin`→`handleInstantiatePlugin`：track_id+plugin_path/plugin_identifier→`resolveExternalPluginDescription`（knownPluginList 按 `createIdentifierString()` 精确匹配；**shell 成员描述保持**——注释明言 Waves 多成员共享一个 WaveShell，需防 tracktion 用实例化后的 description 覆写所选成员） | `CommandDispatcher.cpp:3365-3370`；`PluginRackControlService.cpp:3216+`（handleInstantiatePlugin）、`:1225+`（resolve） | identifier 含 shell 容器文件路径+成员 uid |
| L2 | 插入轨 | `PluginCache::createNewPlugin(ExternalPlugig, desc)`→`pluginList.insertPlugin` | 同上 | 此时是 te::ExternalPlugin 状态对象，实例未建 |
| L3 | 装载期 | `doFullInitialisation`：`findMatchingPlugin`→`processing && !hasLoadedInstance && shouldLoadPlugin` 门→`callBlocking`（消息线程）`startPluginInstanceCreation` | `tracktion_ExternalPlugin.cpp:950-974`、`:703` | processing 置位时机=装载窗口起点 |
| L4 | JUCE | `createVST3Instance`：工作目录切到插件父目录→`VST3ModuleHandle::create`：DLL load（RefCountedDllHandle）→`IPluginFactory`→`findClassMatchingDescription`（factory 多 class 中选成员；**no_match 时 Waves/WaveShell 特判日志 `[WavesVST3UI] event=class_selection result=no_match`**） | `juce_VST3PluginFormatImpl.h:3790-3833`（createVST3Instance）、`:1300-1380`（VST3ModuleHandle::create+特判） | shell 容器=一个 DLL 暴露 N 个 class（301 日志 file_or_identifier=WaveShell1-VST3 17.1_x64.vst3） |
| L5 | JUCE | `VST3ComponentHolder::initialise`：`component.loadFrom(factory, classID)`→`component->initialize(host)`——**失败面均有 `[WavesVST3UI] component_create result=load_failed/initialise_failed` 留痕**（本仓既有 patch，pinned snapshot bae03310 即在） | `Impl:1762-1833` | host 上下文=VST3HostContextHeadless |
| L6 | JUCE | 实例级 `initialise`：fetchController（controller_lookup 日志）→setComponentHandler→grabInformationObjects→configureParameters→setupIO→`fillInPluginDescription`→**identity_validation**（实例化后 name/uid 与请求不符→return nullptr，有 `[WavesVST3UI] identity_validation result=mismatch` 日志） | `Impl:2355-2415`、`:3806-3826` | shell 成员错装（选错 shellnode）在此拦截 |
| L7 | tracktion | `ExternalPlugin::initialise`（prepareToPlay；shared container 时**不用实例 description 覆写 desc**）→后续 301/309 已钉死的 setupProcessing/processMode 时序（S1，此处不重证） | `tracktion_ExternalPlugin.cpp:1140-1191`（`:1174` hasSharedContainer 分支） | 309 E2a 遥测=本层运行时证据 |

**结论**：host 层四道壳感知拦截面（class_selection no_match / component_create 失败 / identity_validation mismatch / hasSharedContainer 描述保持）全部带 `[WavesVST3UI]` 留痕或 jassert，且 301/309 全部 run 中装载链绿（plugin 1013/status ok）——**host 层装载未失败，GR shellnode 实例化成功**。静音发生在 L5/L6 之后的闭源内部。

### 2.2 Waves 闭源内部架构（二进制 strings 静态勘察，不运行）

WaveShell1-VST3 17.1_x64.vst3（10,523,480 B，SHA256 `ee6b1a81…da7bb90`，Common Files\VST3 与 WaveShells V17 两处副本逐位同款）内符号/字符串证据：

| 证据 | strings 原文（节选） | 架构含义 |
|---|---|---|
| Proxy 类族 | `WCPluginInstanceProxy`、`WCWavesAPI3InstanceProxy`、`WCPluginLoader::LoadPlugin`、`WCPluginInstanceProxy::LoadPluginDLL/UnloadPluginDLL`（mangled 符号在卷） | shell 内每个成员=proxy；**真插件 DLL（GrandRhapsody.dll）的装载是与 VST3 component 实例化分离的独立阶段**（可延迟） |
| 延迟装载错误面 | `eFaildToLoadPluginDLL`、`eInitPlugin`、`eInitPluginForTest`、`pluginProxy.UnloadPluginDLL` | 子 DLL 装载失败是显式错误枚举——但对 host 无返回码通道（VST3 process 无错误上报，309 §1.4 已证） |
| license 状态机 | `IsPluginLicensed`、`isLicensed`、`SetIsSessionLicensed`、`IsChainLicensed`、`SaveLicenseStatus`、`UpdateDemoMode`（壳+GrandRhapsody.dll 双侧同款，dll 侧挂 `cALGORITHM` 类与 `WavesAPI3Internal` 请求/响应结构） | license 判定结果写入**会话状态**（Session）；demo 模式可更新/持久化 |
| **demo 静音机制** | **`IsMuteOnDemo`/`isMuteOnDemo`**（壳 proxy 方法符号+dll 内部函数双在卷）、`WP_SetMute`、`WP_SetOutputMute`、`SetMute` | **未授权/demo 时输出静音是 Waves 显式设计行为**——「license 拒绝→静音」有直接机制载体 |
| license 对话框（GUI 面） | `Waves License Error`、`Rescan Licenses`、`Skip All`、`The following Waves plugins do not have licenses.`、`No valid Waves licenses found.`、`Authorization aborted - Canceled by user`、`tmpDisableLicenseDialog.txt` | license 缺失时弹**GUI 对话框**；存在跳过对话框的开关文件（本机未投放，§3.4） |
| license 引擎接口 | `WavesLicenseEngine.bundle`、`WavesLicenseEngine is missing.`、`wle.licenses`、`license_guid` | 壳装载期依赖 ProgramData 的独立 LicenseEngine 模块解析 wle 文件 |
| 插件服务器 | `eWavesPluginServerConfigFolder`、`IPCConnected`/`IPCDisconnected`、`localhost` | V17 壳与 WavesPluginServer（ProgramData 下 PyInstaller/aiohttp 服务，V14.2+V17.1 两版 bundle 在场）有 IPC 面授权/UI 服务 |
| demo UI | `ShowDemoModeWin`、`ShouldShowDemoBanner`、`UpdateViewRectForDemoMode`、`SetDemoConfig`（GrandRhapsody.dll 内） | demo 状态有可见 banner/窗口形态 |

**GrandRhapsody.dll**（1,170,264 B，`C:\Program Files (x86)\Waves\Plug-Ins V17\GrandRhapsody.bundle\Contents\Win64\`，mtime 2026-05-28 02:41）：同源 license 面（`IsPluginLicensed/IsMuteOnDemo/UpdateDemoMode/SaveLicenseStatus/SetIsSessionLicensed` 挂 `cALGORITHM@wvWavesV17_1_42` 与 `WavesAPI3Internal` 请求响应结构）——**license 判定贯穿壳与插件体两层**。

## 3. 本机 Waves 安装面盘点（全部只读）

### 3.1 安装拓扑

- **VST3 公共目录** `C:\Program Files\Common Files\VST3\`：五个 WaveShell 共存——`WaveShell1-VST3 14.12/14.21/16.7/17.1_x64.vst3` + `WaveShell1-VST3-ARA 17.1_x64.vst3`（全部 mtime 2026-07-26 21:16）。**多版本共存**：JUCE knownPluginList 中同产品不同版本的成员并存，identifier 匹配靠 uid 精确性；错版本实例化由 L4/L6 拦截面处理（309 全绿排除）。
- **Waves 主安装** `C:\Program Files (x86)\Waves\`：`Plug-Ins V17\`（263 个 .bundle，含 GrandRhapsody.bundle）、`WaveShells V14-V17`（V17 内 VST3 与公共目录副本同 hash）、`Applications V17`、COSMOS 等。
- **ProgramData** `C:\ProgramData\Waves Audio\`：`Licenses/`、`Demo Mode/`、`License Notifications/`（V14-V17）、`Modules/`（WavesLicenseEngine.bundle、InnerProcessDictionary2）、`WavesPluginServer/`（V14.2+V17.1 bundle，含 WavesPluginServer.exe/WavesPluginClient.exe）、`Central/`（V11-V17 索引）。
- 注册表 `HKLM\SOFTWARE\Waves Audio`：查询无输出（无该键或为空）——Waves V17 授权不依赖注册表（wle 文件制）。

### 3.2 时间线（文件 mtime 亲核）

| 时间 | 事件（物证） |
|---|---|
| 2025-03-23 | `Licenses/0001_7b2c15836a10.wle` 与 `Modules/WavesLicenseEngine.bundle/.../WavesLicenseEngine.dll`（Win32/Win64）mtime 同日——license 文件与解析引擎同期产物 |
| 2026-05-28 02:41-04:54 | `Plug-Ins V17\GrandRhapsody.bundle\...GrandRhapsody.dll`（02:41）与 `WaveShells V17\WaveShell1-VST3 17.1_x64.vst3`（04:54）等 V17 主体文件 |
| 2026-07-26 20:57-21:22 | 授权/安装动作窗：`last.lgn`+`LastAcceptableMACAddress.txt`（20:57）、Central/License Notifications/Demo Mode 目录（21:09-21:15）、公共目录 VST3 副本（21:16）、`Licenses/` 目录定形（21:22） |
| **2026-11-09 19:07** | **`Licenses/LastRefreshLicenseFile.txt` mtime=309 栈腿 E2a 装载 GR 窗口内（19:06 占用→19:29 释放）**——装载时 license refresh 路径真实执行的直接物证（文件 0 字节，仅 touch） |
| 2026-11-09 17:58:04（收腿时顺带观测） | 收腿进程亲核发现 `WavesPluginServer.exe` 常驻（PID 30144，启动 17:58:04，**V14.2 bundle**，非 V17.1，内存 ~108MB）——启动早于 309 栈窗口 19:06，309 工件未记录该进程（当时未观测）；主体（哪个会话拉起）不可考。含义：①309 八 run 期间系统存在 V14.2 server 常驻而 GR=V17 插件——server 版本错配面首次入卷；②D2 判别腿必须记录 server 进程版本/启动时间/端口（见 §5 修订）。本腿未启动任何 Waves 进程（全部命令只读在卷），该进程非本腿产物 |

### 3.3 license 链物证（核心新发现）

`0001_7b2c15836a10.wle`（230,529 B，XML）内容特征：

- `<DeviceID>7b2c15836a10</DeviceID>` + `<DeviceName>**Team V.R**</DeviceName>`——**非本机设备标识的第三方破解组织署名授权文件**；
- `<LastSync>20230101</LastSync>` `<NextSync>99990909</NextSync>`——伪造的永续同步期；
- 主体=大量 `<License>` 条目全部 `Type=Full License`、同一 `LicenseID=20582927` 批量复制、GUID 呈 `ad21-11e0-*` 系列连号——非正常逐产品授权形态。

配套矛盾：`last.lgn`（Waves 引擎写入的本机记录）`DeviceID=08BFB841E625`=`LastAcceptableMACAddress.txt` 内容（本机 MAC）——**本机会话记录与 wle 授权文件 DeviceID 不一致**；`NextSync=20260825` 已过（今日 2026-11-09）。

WavesLicenseEngine.dll（x64，1,658,568 B）strings 证据链：`ReadLicensesFromFile`、`Found valid license file: `/`Found invalid license file: `、`license-expired`、`license-not-began`、`no-license`、`Need To Renew Licenses`、`Found wle at: `/`Could not find wle at: `、`noautowlesync.txt`（跳过自动同步的开关文件）——**wle 校验存在 valid/invalid 判定分支**（校验内容含 DeviceID 绑定/签名/同步期，闭源不可读）。

### 3.4 判别可用的免侵入开关面（strings 发现，均未投放）

- `C:\ProgramData\Waves Audio\...\noautowlesync.txt`（LicenseEngine 检查，在场即跳过 wle 自动刷新）；
- `tmpDisableLicenseDialog.txt`（壳检查，在场即抑制 license 缺失 GUI 对话框）。

二者投放/删除均为可逆文件操作；本腿未做任何投放（只读纪律）。

## 4. 嫌疑重构（309 S2/S3 族细化为 H1/H2/H3）

| 假设 | 机理（本卡取证后形态） | 支持证据 | 反对/削弱证据 | 评级 |
|---|---|---|---|---|
| **H1 license 在非交互上下文拒绝激活** | 本机 wle=第三方破解授权（DeviceID 不符+Team V.R 署名）；壳装载期经 WavesLicenseEngine 校验 wle→若判 invalid/no-license→无 GUI 上下文（headless 内核）license 对话框不可显→`IsMuteOnDemo`/`SetMute` 路径生效→输出全零。demo 静音=全窗静音形态与四 harness 全零逐项吻合 | §3.3 全链物证；`IsMuteOnDemo` 机制载体（§2.2）；`LastRefreshLicenseFile.txt` 19:07 touch=装载期 license 路径执行；VitApp 内核=headless 无 GUI（309 八 run 无任何窗口）；demo 无声形态=持续全零（非周期哑） | 无直接反对（309 数据全部兼容 H1：与挂载方式/渲染模式/实时离线无关的「装载后即静音」正是 license 拒绝的预期形态） | **主嫌疑** |
| H2 shell 延迟实例化在离线窗口外 | proxy 架构=VST3 component 实例化与 `WCPluginLoader::LoadPlugin`（真 DLL 装载）分离；若子 DLL 装载/`eInitPlugin` 是懒执行且依赖 license/IPC 前置（WavesPluginServer localhost 服务未起→IPCDisconnected→初始化不完成），host 侧无任何可观测失败信号（301 守卫绿=JUCE 层实例在，不代表壳内算法就绪） | `LoadPluginDLL/eFaildToLoadPluginDLL/eInitPlugin` 枚举；`IPCConnected/IPCDisconnected`；WavesPluginServer 进程未被任何 run 记录拉起（309 工件进程清单只有 VitApp/VitAgent） | M0a2 装载→渲染间有分钟级 chat 轮次仍静音；E1 实时 play 前亦有装载+预检延时——「窗口外」需要窗口>分钟级才成立；但「license 拒绝导致的永久不就绪」与 H1 合流后形态相同 | 次（与 H1 非正交：H1 成立时 H2 的懒装载也会因 license 拒绝而永不就绪） |
| H3 采样率/块长组合触发壳层哑火 | 特定 sr/bs（如 44100/512 re-prepare 形态）触发壳内部状态清洗/程序丢失 | 309 E2a 观测到 bs 480→512 re-prepare+两次 setupProcessing kRealtime（S1b 路径） | **已被 309 数据大幅削弱**：M1 实时播放（设备 sr/bs 44100/480，无 re-prepare、无 setNonRealtime）同样全零；E2b C 一致化仍静音——sr/bs 与渲染模式两个维度上静音不变 | 弱（保留仅为完备性） |

## 5. 三假设判别实验设计（判别腿候栈执行；§8 口径成文）

装置基线：309 栈腿复用——`dev_agent_smoke.ps1 -WaveshellStacklegSmoke` + `waveshell_stackleg_smoke.py` 参数面（`-WaveshellLegMode {realtime,render}` × `-WaveshellLegPlugin {grand_rhapsody,surge_xt}` × `-WaveshellMountMode {direct,rack}`），内核=含 VIT301_GUARD+VIT309 遥测的 engine 分支续用（`vit309-stackleg` 本地分支在库）。**判别腿跑前按 RUNBOOK 惯例另立跑前口径文件**；本节为设计面。

### D1 GUI 会话对照（H1 判别器，信息量最大）

- **原理**：license 拒绝/demo 态在 GUI 上下文有可见形态（`Waves License Error` 对话框、demo banner=`ShouldShowDemoBanner/ShowDemoModeWin`）；且部分 Waves 版本在 GUI 会话完成授权确认后 `SetIsSessionLicensed` 翻转会话状态。
- **腿**：装载 GR（309 E1 装置）→ `open_plugin_editor`（VitApp 命令面在卷，`CommandDispatcher.cpp:3372+`）→ 人工/截图观测编辑器内容（license 弹窗？demo banner？）→ 关闭编辑器 → 复跑实时播放电平测量（E1v2 tap 装置）。
- **判定**：
  - 出现 license 对话框/demo banner → **H1 直接证实**（无需任何 Waves 侧侵入）；
  - GUI 会话后 RT-LEVEL 由 silent 翻转 sound → H1 证实且给出 host 侧 workaround 出口（GUI 预热）；
  - GUI 干净无弹窗且播放仍静音 → H1 的「GUI 可见形态」分支排除，转 D2/D3。
- **注意**：此腿含一次 GUI 开窗（内核侧 headless 窗口），不属用户手测义务（自动化腿，截图留档）。

### D2 license 状态文件观测（H1 旁证，零侵入）

- **腿**：装载 GR 前后快照 `C:\ProgramData\Waves Audio\` 下 `Licenses/`（含 LastRefreshLicenseFile.txt mtime/内容）、`License Notifications/`、`Demo Mode/` 差分；同步快照进程表（WavesPluginServer.exe/WavesPluginClient.exe：**版本（V14.2 vs V17.1 bundle 路径）/PID/启动时间**）+ localhost 新增监听端口——**基线快照必须在启栈前做**（§3.2 已证系统可有 17:58 启动的 V14.2 常驻，版本错配面须与装载效应分离）。
- **判定**：装载触发 refresh touch（预期，§3.2 已有一次物证）+ 无新授权文件+ 无 V17.1 server 拉起（或拉起即退出）→ 与 H1「校验失败后静默」自洽（非充分判定，作 D1 的环境佐证）。V17.1 server 拉起并存活 → H2 的 IPC 面激活，记录端口与存活期，转 D3 交叉。

### D3 延时对照（H2 判别器）

- **腿**：E1v2 装置加 py 侧 sleep 注入：装载→等 T（T∈{60s}，一档即可）→play→电平采样。
- **判定**：延时后出声 → H2 成立（窗口论）；仍静音 → 「分钟级窗口外」排除，H2 仅存「license 拒绝致永不就绪」合流形态（归 H1 判定管辖）。

### D4 sr/bs 对齐渲染（H3 完备性腿，最低优先）

- **腿**：渲染参数显式 `blockSizeForAudio=480`（与设备一致，避开 480→512 re-prepare）+ 一档 48000sr 变体，GR render。
- **判定**：仍静音 → H3 关闭（预期，M1 已削弱）；出声 → H3 复活且直接给出 host 侧 workaround（bs 对齐）。

### §8 口径（判别腿跑前按此框架在 RUNBOOK 成文定稿）

- **运行次数**：每腿单 run 无重跑（D1/D2/D3/D4 各 1，合计 4 run 预算；D2 附着 D1/D3 run 不独立占栈窗口）。
- **预算**：零 LLM（全部 -NoChatSmoke 直 ZMQ 驱动；telemetry 文件 absent 亲证）。
- **成功条件**：每腿 exit 0 且判定字段在卷（D1=GUI 观测记录+前后电平；D2=文件差分+进程快照；D3=延时后 verdict；D4=wav 计量）；verdict 本身 sound/silent 均为有效判定结果。
- **失败分类**：exit 2=environment（栈/端口/进程互染）；exit 4=command_defect（装置/装载失败/GUI 开窗失败）；GUI 开窗失败单独记录（headless 会话隔离限制）。
- **止损线**：同一确定性断点连续两败停手上交；D1 的 GUI 截图失败一次即转无头判别（不重试）。
- **观测增强（免改内核）**：`[WavesVST3UI]` 全系日志（class_selection/component_create/controller_lookup/identity_validation）在既有内核日志管线（TRACKTION_LOG→VitHeadlessServer log）已覆盖 L4-L6——判别腿直接读取，无需新插桩。

### 停止条件对照（卡面）

- D1（GUI 对照）+D2（文件/进程观测）**不需要** Waves 调试符号、不侵入 license 环境 → 判别可在协议内完成；
- 若 D1-D4 全部无法判定（GUI 无形态、文件无差分、延时无变化、bs 对齐无变化）→ 判别确需 Waves 内部观测（procmon 级注册表/文件追踪或调试符号）→ **触发卡面停止条件**：取证上交转纯策略方案裁定（§6 出口 B 直接生效，黑名单+替代路径维持 309 形态）。

## 6. 结论面预案（两出口判据预写）

### 出口 A：可修（host 侧 workaround）

- **判据**（任一成立）：D1 GUI 会话后出声（license 会话预热 workaround：装载后一次性 `open_plugin_editor` 探测/预热，headless 服务化做法需主管裁形态）；D3 延时后出声（装载就绪等待 workaround：装载后 readiness 轮询再放行渲染/播放）；D4 bs 对齐出声（渲染参数对齐 workaround：`blockSizeForAudio=设备 bs`）。
- **修法形态**：全部 VitApp-only 参数/命令面改动（零 engine）；与 309 修法 B（warn_only）正交可叠加；谓词复用 309 `containsIgnoreCase("waveshell")`。
- **验证矩阵**：M0a2 场景复跑出声+Surge 不回退+全包回归（同 309 E3/E5/E4 面）。

### 出口 B：不可修（闭源二进制内）→ 黑名单+替代路径

- **判据**：D1 GUI 干净或 GUI 后仍静音 + D3/D4 仍静音——静音锁定在闭源 license 校验/壳内初始化，host 无干预点。
- **本腿已给出的定性材料**：本机授权链=第三方破解 wle（§3.3，DeviceID 不符+Team V.R 署名+伪造同步期）与正版 WavesLicenseEngine 校验面的**版本/信任不匹配**是无需调试符号的合理成因链——出口 B 概率因该物证显著上调；「修复本机授权」（正版化/重新激活）超出仓库能力边界，属用户环境域。
- **策略方案**：309 修法 B `warn_only` 默认**维持**（检出+WARN+`shell_class_plugin_hint` 三披露）——建议主管确认为长期产品形态；替代 VSTi 路径=Surge XT（本机 E4/-13.63dBFS 双代证据健康）作为 demo/实验默认乐器；文档面记录「WaveShell 类插件在本授权环境不可用」的产品边界。
- **warn_only 默认维持的产品确认**（卡面目标 4 尾项）：出口 B 生效时建议正式确认；出口 A 生效时 warn_only 仍可保留为兜底（workaround 失败时的可见降级）。

## 7. 端测边界与纪律声明

- 本腿零栈零 LLM 零源码改动：未启动 VitApp/agent/Waves 任何组件；`noautowlesync.txt`/`tmpDisableLicenseDialog.txt` 等开关未投放；wle/license 文件只读未动；两棵 engine 树只读。
- 判别腿（D1-D4）端测=agent ZMQ/命令面真栈驱动+GUI 截图留档+文件系统/进程差分+wav 物理计量；不覆盖 webui 渲染面与 Godot 旅程（本卡无该域改动）。
- 主观边界：本报告对 wle 文件的「非正版」定性基于文件内容物证（第三方署名/设备不符/伪造同步期/批量同 ID），不做超出物证的法律判断；该物证对判别设计的意义=license 校验面的不确定性来源登记。

## 8. 工件与索引

- 本目录：`report.md`（本件）+ `PROGRESS.md`（断点续跑）。
- 引用工件（只读）：297 @ port/kernel-offrender-forensics-1 `coord/runs/KERNEL-OFFRENDER-FORENSICS-1/report.md`；301 @ port/kernel-offrender-repro-1 `coord/runs/KERNEL-OFFRENDER-REPRO-1/evidence.md`；309 @ main `coord/runs/WAVESHELL-OFFRENDER-FIX-1/`（recon-design+RUNBOOK-STACKLEG+stackleg_20261109_1906 八 run）。
- 关键只读锚点清单：`juce_VST3PluginFormatImpl.h:1300-1380/1660-1833/2355-2415/3790-3833`；`tracktion_ExternalPlugin.cpp:950-974/1140-1191`；`CommandDispatcher.cpp:3365-3370`；`PluginRackControlService.cpp:3216+/1225+`；WaveShell 二进制 SHA256 `ee6b1a818fb6392e9c28c1b053ead872c6d12ab654daf079293461721da7bb90`。
