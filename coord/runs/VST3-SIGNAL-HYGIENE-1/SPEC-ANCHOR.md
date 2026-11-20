# VST3-SIGNAL-HYGIENE-1 规范锚点文档（309 S1 卫生缺陷：host 侧 setup/per-call mode 信令一致化）

- 卡: `coord/cards/todo/2026-11-09-VST3-SIGNAL-HYGIENE-1.md`（池序 331）/ 执行侧 sighyg331 / 2026-11-20
- engine 修复提交: `98fb0200`（engine 子模块分支 `pc331-sighyg`，基线 60ab1a49=301 插桩；主仓 gitlink 保持 b4397498 不动）
- 本文档为验收标准第 1 项「规范锚点文档」。运行时证据与真栈腿要求见末节。

## 1. 规范要求（ground truth）

`modules/juce/modules/juce_audio_processors_headless/format_types/VST3_SDK/pluginterfaces/vst/ivstaudioprocessor.h:138-143`（309 锚点④，亲读原文）：

- kRealtime↔kPrefetch 切换：**允许** per-call（process 调用直接改 `ProcessData::processMode`），插件应在 process 内检查该字段；
- kRealtime（或 kPrefetch）↔**kOffline** 切换：**必须**由 host 调用 `IAudioProcessor::setupProcessing` 通知插件。

字段类型事实：`ProcessSetup::processMode`（:177）与 `ProcessData::processMode`（:243）均为 `int32`；枚举 `ProcessModes { kRealtime=0, kPrefetch=1, kOffline=2 }`。

## 2. host 侧现状链（六锚点，2026-11-20 于 60ab1a49 检出逐字亲核）

| # | 锚点 | 现状 |
|---|---|---|
| ① | `juce_VST3PluginFormatImpl.h` prepareToPlay 守卫（原 :2489-2493） | `isActive && approximatelyEqual(sr) && getBlockSize()==bsp` 即 return——**条件不含 process mode** |
| ② | 同文件 `processAudio`（原 :2648） | `data.processMode = isNonRealtime() ? kOffline : kRealtime;`（per-call） |
| ③ | `processors/juce_AudioProcessor.cpp:410-413` | `setNonRealtime(bool)` 仅置 `std::atomic<bool> nonRealtime` 标志，**零 VST3 调用** |
| ④ | `ivstaudioprocessor.h:138-143` | 见 §1 |
| ⑤ | `tracktion_NodeRenderContext.cpp:136-138` | ctor：`setAllPluginsRealtime(plugins, r.realTimeRender)` → `nodePlayer->prepareToPlay` → `flushAllPlugins`；dtor（:161）恢复 `setAllPluginsRealtime(plugins, true)` |
| ⑥ | `tracktion_Renderer.h`（`blockSizeForAudio=512`、`realTimeRender=false` 默认）+ `VitApp/Source/Service/VitProductionCoordinator.cpp` | VitApp 离线渲染默认不设 realTimeRender（309 修法 B warn_only 已合入：opt-in flip :1046/:1201） |

传播链事实：`Renderer::RenderTask::setAllPluginsRealtime`（`tracktion_Renderer.cpp:345`）对每个 ExternalPlugin 调 `getAudioPluginInstance()->setNonRealtime(!realtime)`——即锚点③的标志位。ExternalPlugin 层无 setNonRealtime override（grep 零命中）。

## 3. 缺陷机理（对 309 卡面描述的精化）

**setup 行本身已是模式感知**（原 :2505-2506 `setup.processMode = isNonRealtime() ? kOffline : kRealtime`）——缺陷不是"setup 硬编码 kRealtime"，而是：

1. 装载期：插件在 realtime 下 prepared → setupProcessing kRealtime（44100/480）；
2. 建图：bs 480→512 变化触发 re-prepare（isNonRealtime 仍 0）→ 第二次 setupProcessing kRealtime（44100/512）；
3. 渲染窗：`setAllPluginsRealtime(false)` 置标志 → `prepareToPlay(44100, 512)` → **守卫①因 isActive+同 sr/bs 早退** → 模式感知的 setup 行不可达 → setup 停留 kRealtime；
4. per-call：`processAudio` 送 `kOffline`（isNonRealtime=true）→ **setup=kRealtime vs per-call=kOffline，违反 §1**。

309 E2a 运行时签名（perCallMode=kOffline/setupMode=kRealtime）与此链逐点吻合。第二个 setupProcessing 调用点 `setupIO()`（原 :3407，实例化期固定 44100/1024/kRealtime）语义正确（扫描语境非 offline），仅需同步维护跟踪。

## 4. 修法评估与定案

- **C 原型（vit309-stackleg 9b8cc8a）直接转正：不可行。** ①提交对象本机不可达：栈腿 worktree `D:/Vit_DAW_wt_309recon` 已删除，`vit309-stackleg` 为其子模块独立克隆的本地分支从未推远端（309 裁定①），主模块仓库 `.git/modules/tracktion_engine` 无该分支/对象（cat-file 亲证 fail）；②语义不适：C 为 per-call 强制 kRealtime（与 setup 一致化），但 offline 渲染语义上 host **应** setup 级 kOffline（§1 正方向）；③309 E2b 已证 C 对 GR 无效（转正失据）。
- **定案=setup 级 kOffline（重做）**：守卫①加 `lastSetupProcessMode` 模式感知——`isActive && 同 sr/bs && lastSetupProcessMode==desiredSetupMode` 才允许早退；mode 翻转时走完整 prepare 路径（deactivate→setupProcessing(kOffline)→setBusArrangements→activateBus→setActive），per-call kOffline 与 setup kOffline 一致，且回 realtime 时对称复位。该完整路径与 sr/bs 变化触发的既有 re-prepare 同路径（301 实测 480→512 re-prepare 对全插件工作），唯一新变量是 ProcessSetup.processMode=kOffline——规范要求插件必须支持。
- **env 门 `VIT331_CONSISTENT_SETUP`（默认关）**：真栈 E4 回归腿 PASS 前生产行为逐字节不变；默认翻转由决策侧在栈腿后执行。309 C 原型 env 门控（VIT309_C_CONSISTENCY）同款处置。
- **停止条件**（卡面）：修法引发非 WaveShell 插件回退 → env 门维持关，转纯登记（本卡 §6 起已登记在案）。

## 5. 改动面与防回归断言（engine 98fb0200，1 file +92/-1）

`modules/juce/modules/juce_audio_processors_headless/format_types/juce_VST3PluginFormatImpl.h`（VST3PluginInstanceHeadless）：

1. prepareToPlay 守卫：模式感知（env 门控）；
2. prepareToPlay setup 后：`lastSetupProcessMode`/`vit331SignalTraceArmed` 维护 + `VIT331_SETUP phase=prepareToPlay` 一次性 log；
3. `processAudio`：`VIT331_SIGNAL` 一次性配对断言（armed→exchange(false)），键=`setupMode/perCallMode/consistent/gate/sr/bs`；
4. `setupIO`：跟踪同步 + `VIT331_SETUP phase=setupIO`；
5. 成员：`std::atomic<int> lastSetupProcessMode`（int32 字段同型）、`std::atomic<bool> vit331SignalTraceArmed`、`vit331ConsistentSetupEnabled()`（读 VIT331_CONSISTENT_SETUP，"0"/空=关）、`vit331ProcessModeName(int32)`。

全部 log-only，行为改动仅守卫一处且 env 门控。

**断言面（真栈腿判定键）**：
- 门关（基线复证）：`VIT331_SIGNAL ... setupMode=kRealtime perCallMode=kOffline consistent=0`（309 E2a 签名复现）；
- 门开（修复生效）：离线渲染 `VIT331_SETUP ... mode=kOffline phase=prepareToPlay` + `VIT331_SIGNAL ... setupMode=kOffline perCallMode=kOffline consistent=1`；
- 回归红线（卡面）：Surge XT 同款腿出声不回退（309 E4 同款断言，`-WaveshellLegMode render -WaveshellLegPlugin surge_xt`），且 go/webui 既有面全绿。

## 6. 缺陷登记（负结论分支也成立的在案记录）

host 侧 VST3 信令不一致（setup 级 kRealtime vs 渲染窗 per-call kOffline）为 VST3 规范违反（ivstaudioprocessor.h:138-143），影响面=对 setup/per-call 模式敏感的插件（WaveShell 类实证表现之一，非 GR 静音成因——309 E2b 判别结论维持）。修复以 env 门控形态落地于 engine 分支 `pc331-sighyg@98fb0200`，gitlink 与默认行为不动，待真栈回归后合入。

## 7. 真栈验证腿（lead，留决策侧；本卡禁启栈）

复用 309 装置与 `coord/runs/WAVESHELL-OFFRENDER-FIX-1/RUNBOOK-STACKLEG.md` 模式：栈协议两条登记（占用/释放）+E4 同款 Surge 腿+门开/门关 A/B 两 run 断言 §5 判定键。被测二进制构建命令见 `coord/runs/VST3-SIGNAL-HYGIENE-1/PROGRESS.md` S4 节（configure 已 exit 0 验证；engine dir 指向主模块仓库 pc331-sighyg 检出）。

## 8. 已核对的边界（不扩域）

- Go 侧 `agent/internal/vst3host`、`processorregistry`、`pluginprobe` grep 信令关键词零命中——派发提示词猜测的 Go 代码域不成立，实际信令面=C++ engine（本卡执行按卡面文件域）。
- `setupIO` 的 44100/1024/kRealtime 实例化 setup：扫描语境，模式正确，仅补跟踪。
- VitApp 侧 309 修法 B（warn_only/opt-in flip）已在 main，本卡不触碰 VitApp/Go 源码。
- genapi（并行卡 440）文件域不相交。
