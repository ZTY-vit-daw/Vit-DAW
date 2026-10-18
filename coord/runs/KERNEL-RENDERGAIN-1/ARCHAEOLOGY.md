# KERNEL-RENDERGAIN-1（132）考古报告：stock 内核离线渲染 ~2x 增益归因

- owner：DSH PC 执行流 exec-rendergain；worktree `D:/Vit_DAW_wt_rendergain1`，分支 `port/kernel-rendergain-1`（基线 origin/main=c33a5cfe）
- 取证边界：**零生产代码改动**；本目录只放诊断工件/报告；不改既有 scripts
- 报告时间：2026-10-18 15:00 前（考古腿，不占栈）；栈腿（最小复现+差分）待 Lead 放行后补
- 机器：PC（Windows，D:\Vit_DAW 共享主工作树只读；tracktion_engine 子模块 D:\Vit_DAW\tracktion_engine 只读）

## 0. 结论速览（三问）

### ① 注入点归因：**工程 master volume 状态（fader 位置语义），不是 te::Renderer / 写入器，也不是故意 headroom 补偿**

- 注入点 = **Edit 级 master volume 插件**（`te::VolumeAndPanPlugin`，isMasterVolume=true）的 `volume` 属性值 **1.0**。
- 在 pin 住的 tracktion_engine 快照里，`volume` 属性是 **fader 位置（0..1）**，不是线性增益：`volumeFaderPositionToDB(pos) = 20*ln(pos) + 6`，`volumeFaderPositionToGain(pos) = 10^((20*ln(pos)+6)/20)`。
  - **pos=1.0 → +6.000000 dB，gain=1.99526231**（fader 顶到 +6 dB）
  - pos=0.7408182 → 0 dB（unity）；pos=0.6376282 → -3 dB
- 实测对账（K1 既有工件，见 §2）：渲染/源幅度比 = **1.995239 = +5.99990 dB**，与 pos(1.0)=+6.000000 dB 差 0.0001 dB（16bit 量化内）——**吻合**。
- 该 master volume 状态值来自三处硬编码/模板（均为 `volume="1.0"`）：
  1. `VitApp/Source/Core/VitPaths.h:25-29`（`getDefaultProjectXmlTemplate` 内嵌 XML）
  2. `VitApp/Source/Core/VitPaths.h:73-77`（`getBlankProjectXmlTemplate`，`new_project` 用）
  3. 仓库 `VitApp/Workspace/default_project.xml:14-18`（headless 内核启动加载的默认工程；`VitHeadlessService: edit loaded from .../default_project.xml`）
- 渲染侧只是把这个插件节点放进图里：`VitProductionCoordinator::startOfflineRender` 用 `te::Renderer::Parameters`，`useMasterPlugins` 默认 true（`TransportAudioService::handleStartRender` `use_master_plugins` 默认 true），engine 侧 `Renderer.cpp:41/143` 把它映射为 `CreateNodeParams::includeMasterPlugins`，`EditNodeBuilder.cpp:1763` gate、`1785-1786` `createNodeForPlugin(*masterVolPlugin, ...)`。
- 排除项（考古证据）：
  - **te::Renderer 归一化**：`Renderer.h:73-76` `shouldNormalise/shouldNormaliseByRMS` 默认 false，Vit 代码未设置（全仓 vit 源无 shouldNormalise 赋值）；`tracktion_Renderer.cpp` 的 normalising 分支不会走。
  - **写入器 16bit/浮点**：增益在 float 图内已发生，与写出 bitDepth 无关（K1 diag 全为 16bit；栈腿补 16/24/32 对照预期同比值）。JUCE WavAudioFormat 16bit 只做 float→int16×32767 标定。
  - **import/clip/track 增益**：导入路径不设 clip gain/track gain；新鲜 track 的 volume 插件默认 `decibelsToVolumeFaderPosition(0)=0.7408`（0 dB，`tracktion_VolumeAndPan.cpp:107`）；`VolumeAndPanPlugin::create()` 不写 volume 属性 → 走该默认。K1 左独占源 L=0.9976/R=0（逐声道标量、排除声道和）与 master 级标量一致。
  - **CLICKTRACK level 0.6**：click 默认关，不参与离线渲染。
- **是故意 headroom 补偿还是缺陷 → 缺陷（模板默认值语义陈旧）**，不是 headroom 设计：
  - 引擎自身语义决定了 1.0 是 fader **最大值**（+6 dB），不是 unity；工程内两处独立证据表明**预期默认 master = -3 dB**：`VitKernelUtils.cpp:128-140` 传入 `Edit::Options{.., defaultMasterVolumedB=-3.0f}`（engine `Edit.cpp:951-952` 仅在没有 MASTERVOLUME 子树时生效）；`scripts/gen_vit_project.py:174` 生成 `volume="0.6376281380653381"`（= `decibelsToVolumeFaderPosition(-3.0)`，即 -3 dB，与 fader 语义自洽）。
  - 上述三处 `volume="1.0"` 与"预期 -3 dB"相差 9 dB；最合理解释=早期模板按"线性增益 1.0=unity"的旧心智/旧格式写值，pin 后的引擎按 fader 位置解释 → 变成 +6 dB。engine 的旧工程迁移（`tracktion_OldEditConversion.h:274-281`）只迁移了旧 AuxSend `gain`→`auxSendSliderPos`，**未迁移 volume 属性**（该迁移面本来按"新语义早已生效"设计）。
  - 无任何 Vit 代码/注释声明 +6 dB 或 master headroom 补偿；全仓无 master volume 命令面（见下）。结论：**缺陷（默认工程/空白工程模板值陈旧）+ 引擎属性语义迁移缺失**。

### ② 用户面影响（初步判定，栈腿补实测）

- **不是"导出比监听热 6 dB"**：master volume 节点同时存在于**播放（监听）图**与**渲染图**——播放图 `CreateNodeParams::includeMasterPlugins` 默认 true（`tracktion_EditNodeBuilder.h:31`），渲染图随 `useMasterPlugins=true`（默认）。→ 监听与导出**同热 +6 dB**，两者一致。
- 实际用户面问题：**所有从模板/既有工程继承的工程，master 总线恒定 +6 dB（过 unity）**：
  - 渲染/导出比工程内"轨道电平表/源幅度"期望高 6 dB → 16bit 导出**提前 6 dB 削波**（K1 run1 的 `ratio 1.01≠2.0` 失败即此：双轨 0.5 源和已越过 16bit 线性区；这正是本卡上游 K1 发现削波非线性的机理）。
  - 轨道电平表在 master volume 之前，用户按轨道表混到 -6 dBFS 时实际输出已 0 dBFS。
  - **无 master volume 命令面/UI 面**：kernel CommandDispatcher 无 master volume 命令（只有 track/clip/aux 增益命令）；Godot 前端 grep `MASTERVOLUME|master_volume|masterVolume` **零命中** → 用户不可见、不可控。
  - 工程来源不同电平不一：`new_project`（blank 模板）与默认工程=+6 dB；`gen_vit_project.py` 生成的工程=-3 dB；无 MASTERVOLUME 的 XML=-3 dB（Edit::Options 缺省）→ **跨工程来源 9 dB 电平不一致**。
  - 仓库内 16/16 个项目/工件 XML 的 MASTERVOLUME 均为 `volume="1.0"`（仅 gen_vit_project 产物 2 份为 -3 dB 位置）→ 存量工程全部受影响。
- **P2 卫生 or P1**：建议 Lead 按 P1 候选裁定——影响面=所有新建/存量工程的导出与监听绝对电平（系统性 +6 dB、削波提前、表头不一致），且无 UI 暴露；修复面小（模板值改 0.7408182 或 -3 dB 位置 + 迁移/回归），但属渲染/监听主链电平契约，需裁定口径（改默认值 vs 记录为已知行为 vs 加 master 控制面）。本报告只取证，不擅自改。

### ③ 与既有烟测断言的关系

- **比值断言免疫**（K1 已在卷：`sendroute_kernel_smoke.py` wet ratio 2.0000、disconnect 回基线；K1 裁定采信）。
- 绝对幅度面（受影响）清单（考古 grep 面）：
  - `scripts/sendroute_kernel_smoke.py:53-59,217` 注释与 control 定标：`r440≈0.1995`（0.1 源 × 1.995）；下限断言 `>0.05` 在 +6dB 下仍过，但**削波敏感性**是 run1 失败根因（脚本已把源降到 0.1 规避）。
  - kernel 内 L2 render probe 质量门（`VitProductionCoordinator.cpp` `analyseL2ProbeFile`/`quality_evidence_failed` 面，peak/silence 阈值以渲染文件绝对幅度为准）→ 受 +6 dB 影响；需栈腿/后续卡核对阈值口径。
  - TIM/AS-PEAK 类断言（`scripts/tim_assert_smoke.ps1`）作用于**源文件波形包络**（waveform_envelope baker 读源），不经过渲染图 → 不直接受影响（但若同工程渲染产物被用于对比需注意）。
  - TimeFX/COM dual-tap、MOM/FXM、mix 观察面：多为 baseline/processed 同图比值或源侧特征 → 免疫；绝对 dBFS 展示面（meter/R128）若取自渲染产物则整体 +6 dB 偏移。
  - 结论口径：**比值断言免疫成立；绝对幅度断言/阈值面整体受 +6 dB 偏移**，既有烟测里唯一实际受伤的是"绝对幅度进入 16bit 削波段"的场景（K1 run1 实证）。

## 1. 代码锚点（file:line）

| # | 锚点 | 作用 |
|---|---|---|
| A1 | `tracktion_engine/modules/tracktion_engine/utilities/tracktion_AudioUtilities.cpp:136-156` | `volScaleFactor=20`；`decibelsToVolumeFaderPosition(db)=exp((db-6)/20)`；`volumeFaderPositionToDB(pos)=20*ln(pos)+6`；`volumeFaderPositionToGain(pos)`；**pos=1.0 → +6 dB** |
| A2 | `.../plugins/internal/tracktion_VolumeAndPan.cpp:107` | `volume.referTo(state, IDs::volume, um, decibelsToVolumeFaderPosition(0))`（缺省 0 dB 位置 0.7408182） |
| A3 | `.../plugins/internal/tracktion_VolumeAndPan.cpp:283-296` | `getVolumeDb()=volumeFaderPositionToDB(current)`；`setVolumeDb()` 走位置换算 |
| A4 | `.../playback/graph/tracktion_EditNodeBuilder.cpp:1763,1785-1786` | `includeMasterPlugins` gate；`createNodeForPlugin(*edit.getMasterVolumePlugin(), ...)` |
| A5 | `.../model/export/tracktion_Renderer.cpp:41,143` | 渲染图 `cnp.includeMasterPlugins = r.useMasterPlugins` |
| A6 | `tracktion_engine/modules/tracktion_engine/model/edit/tracktion_Edit.cpp:931-957` | `initialiseMasterVolume`：仅当 state 无 MASTERVOLUME/PLUGIN 时用 `options.defaultMasterVolumedB`；`volume = decibelsToVolumeFaderPosition(default)` |
| A7 | `VitApp/Source/Core/VitPaths.h:25-29 / 73-77` | 两份内嵌工程模板 MASTERVOLUME `volume="1.0"`（=+6 dB） |
| A8 | `VitApp/Workspace/default_project.xml:14-18` | 仓库默认工程 MASTERVOLUME `volume="1.0"`（内核启动加载） |
| A9 | `VitApp/Source/Core/VitKernelUtils.cpp:128-140` | `loadEditFromXmlString` 传 `Edit::Options{..., defaultMasterVolumedB=-3.0f}`（预期默认 -3 dB；被模板 1.0 绕过） |
| A10 | `VitApp/Source/Service/CommandDispatcher.cpp:3961-4007` | `clear_project` 只清 clip/transport，**保留 master volume**（K1 最小复现链保持 +6 dB） |
| A11 | `VitApp/Source/Service/VitHeadlessService.cpp:1111-1116` | `new_project` 加载 `getBlankProjectXmlTemplate()`（+6 dB 模板） |
| A12 | `VitApp/Source/Service/TransportAudioService.cpp:1213-1223` | `start_render`：bit_depth 默认 24（下限 16）、`use_master_plugins` 默认 true |
| A13 | `VitApp/Source/Service/VitProductionCoordinator.cpp:857-871` | `te::Renderer::Parameters`：`useMasterPlugins`、`audioFormat`、`bitDepth`、`sampleRateForAudio`；**不设 shouldNormalise** |
| A14 | `scripts/gen_vit_project.py:173-174` | 生成工程 MASTERVOLUME `volume="0.6376281380653381"`（=-3 dB 位置；与 fader 语义自洽的"预期默认"证据） |
| A15 | `tracktion_engine/.../tracktion_OldEditConversion.h:274-281` | 引擎旧工程迁移仅处理 AuxSend `gain`→`auxSendSliderPos`；volume 属性无迁移 |

## 2. 既有工件实测（本次 archaeology 亲读/新算）

实际读取路径（**注意**：Card/任务书写的是 `D:/Vit_DAW/Workspace/Artifacts/smoke/sendroute_20261017_204856/`，实际在 K1 worktree 的 VitApp/Workspace 下）：

- 诊断 wav：`D:/Vit_DAW_wt_sendroute1_build/diag_{render,minimal,48k,stock_48k,left}.wav`（5 件，576784 B）+ 源 `sin440_48k.wav`/`sin440_left.wav`
- K1 run3 烟测工件：`D:/Vit_DAW_wt_sendroute1/VitApp/Workspace/Artifacts/smoke/sendroute_20261017_204856/`（render_control/wet/after_disconnect + sin440/880.wav + summary/console）
- K1 内核会话日志（产生 diag wav 的同一会话）：`D:/Vit_DAW_wt_sendroute1/Export/staging/runtime/Workspace/Logs/VitHeadlessServer2026-10-17_20-36-29.log`

实测（python 直读 16-bit PCM）：

| 文件 | 采样率/声道 | peak(int) | amp440(int) | 归一化 peak | 备注 |
|---|---|---|---|---|---|
| sin440_48k.wav（源） | 48000/2 | 16383 | 16382.84 | 0.49998 | 0.5 幅度正弦 |
| diag_48k.wav（stock 渲染） | 48000/2 | 32689 | 32688.03 | 0.99759 | **比值 1.995239 = +5.99990 dB** |
| diag_stock_48k.wav | 48000/2 | 32689 | 32688.03 | 0.99759 | 同上（stock 内核） |
| diag_left.wav（左独占源渲染） | 48000/2 | L=32689/R=0 | L=32688.03/R=0 | 0.99759/0 | 逐声道标量（排除声道和） |
| sin440.wav（run3 源 44.1k 0.1） | 44100/2 | 3277 | — | 0.09998 | K1 run3 源 |
| render_control.wav（run3 双轨控制渲染） | 48000/2 | 11508 | — | RMS≈0.199488 | 0.1 源 → 0.0997/轨（≈+6 dB；双轨和 RMS 0.1995） |

- 同一会话 `project.state` snapshot_xml（内核日志 20:36 会话，行 604/632）: `<MASTERVOLUME><PLUGIN type="volume" id="1001" enabled="1" volume="1.0">` —— **产生缺陷渲染的会话，master volume 状态实测=1.0**。
- 换算对账：`volumeFaderPositionToGain(1.0)=1.99526231`；实测比值 `32688.03/16382.84=1.995239`（=+5.99990 dB），差 -0.00012 dB。

## 3. 历史考古

- `git blame`：两份内嵌模板 `volume="1.0"`（VitPaths.h:26/74）、仓库 `default_project.xml`、`gen_vit_project.py` 的 -3 dB 值**均来自初始提交 `991cffb6`（2026-04-22）**——问题自 2026-04-22 起存在（K1 判定"预存 stock 行为"吻合），非近期改动引入。
- pin 历史：子模块 2026-10-17 `1cd57f14` 重指自包含镜像 `bae03310`（内容=te@0981e7ce 快照）；旧 pin 也是 te@0981e7ce → **引擎语义未在近期变化**，问题一直在模板值侧。
- 全仓工程/工件 XML 抽查：**16/16** 份的 MASTERVOLUME=`volume="1.0"`（含 `VitApp/Workspace/default_project.xml`、`coord/runs/JOURNEY-3/*/kernel_workspace/default_project.xml`、L2-2-SEG-SMOKE/L4-LEDGER-DIR/OPT-IMPL-2/PORT-WIN-HYGIENE/QUOTA-RECOVERY 的工程副本）；仅 `scripts/generated_project.xml`/`generated_dark.xml` 为 -3 dB 位置。
- 无"headroom 补偿"痕迹：Vit 源无 master volume 设置代码（仅 VitPaths.h 内嵌模板字符串）；无注释/文档声明 +6 dB。

## 4. 栈腿差分实验计划（待 Lead 放行；不启 Godot UI）

目标：把"注入点=master volume 状态"从代码考古升为真栈实测，并给用户面判定补实测。

- 环境：`dev_agent_smoke.ps1 -StartKernel` 体系（内核 exe=主检出 `Export/staging/runtime/VitApp.exe`，SHA256 B6565DCF85D1DA867E16333052A7B928A5413565940603ADBCF4ABD59DD3123B；不启 UI）；本卡新增诊断脚本放 `coord/runs/KERNEL-RENDERGAIN-1/`（python + 薄 ps1；不改既有 scripts）；工件全部入该目录。
- 实验矩阵（同一内核会话内尽量完成；需重启内核的变体单独一轮）：
  1. **K1 最小复现**：`clear_project→add_track→import_audio(48k 立体声 0.5 正弦)→start_render(16bit)`→逐声道幅度（预期 0.9976）。
  2. **master 节点隔离 A/B**：同 fixture 渲染两次，仅 `start_render` 参数 `use_master_plugins`: true(默认) vs **false** → 预期 true=+6.0 dB、false=1.0x（engine A5 映射证明 master 节点是唯一差异；同时暴露"渲染关 master 但监听仍含 master"的语义口径）。
  3. **工程状态映射**：`load_project` 三份 fixture XML（仅 MASTERVOLUME 不同：1.0 / 0.7408182207 / 0.6376281516，即 +6/0/-3 dB）→ 渲染比值预期 1.9953 / 1.0000 / 0.7079（精确映射，直接坐实 A1 换算）。
  4. **new_project 路径**：`new_project`（blank 模板）→ add/import/render → 预期同 +6 dB（A7/A11）。
  5. **bitDepth**：16 / 24 / 32 → 比值不变（排除写入器；16bit 高幅度出现削波非线性）。
  6. **源幅度扫描**：0.05/0.1/0.25/0.5 → 比值恒定（线性，直到 16bit 削波）。
  7. **采样率/声道**：48k/44.1k 源、mono/stereo → 比值恒定。
  8. 观测面：`list_tracks`/`project.state` 的 volume_db 读回（master 不在 track 行内→不可见，记录为"无 UI 读回面"证据）；如内核有 meter/telemetry 面则顺带记录。
- §8 声明：LLM=0 调用；成功条件=每项实测在预期值容差内；失败分类=environment_interrupt（栈起不来/端口占用）/assertion_fail（与考古预期不符）；止损=同签名 assertion_fail×2。
- 栈纪律：启动前亲核四端口（7878/5555/5556/8787/4445）监听=0+四类进程（VitApp/VitAgent/vsphub/Godot 含变体名）=0；只杀自起 pid；跑完拆净复核=0；占用/释放记录由 Lead 写 PC-RUNTIME-STACK（协调面单写）。

## 5. 边界与自证

- 本报告为**考古腿**（零生产代码、未启栈、未跑渲染）；所有实测数字来自 K1 既有工件与引擎源码换算，栈腿差分未做。
- `D:/Vit_DAW` 共享主工作树只读（仅 `git fetch` / `git log` / `git show` / `git blame`）；未触碰其他卡 worktree；未 push main；本分支 commit 仅含本目录工件。
- 已知局限：监听路径 = 播放图含 master volume 为**代码推断**（EditNodeBuilder.h:31 默认 true），未真栈实测监听电平；UI 无 master 面为 grep 推断（Godot 前端 + kernel 命令面）；绝对幅度断言清单为脚本源静态 grep，栈腿后可按实测补全。
