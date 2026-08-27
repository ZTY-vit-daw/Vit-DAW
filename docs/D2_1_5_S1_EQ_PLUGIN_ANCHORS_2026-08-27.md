# D2-1.5-S1 定标：真实 EQ 插件通道锚点（2026-08-27）

为今晚 D2-1.5-S1（实验通道执行层表驱动）定标真实静态 EQ 插件通道。上游输入：任务卡
`queue/todo/2026-08-27-D2-1-5-S1-table-driven-execution-layer.md`「早窗新增输入」节与
`docs/FREE_STATE_PHASE_D_D2_1_STATIC_EQ_2026-08-26.md` 早窗诊断根因 4。本文只做定标与设计建议，
不改代码；一个治理问题只记录不裁决。

诊断会话：GLM L2（2026-08-27 上午）。所有 file:line 均为本会话在 HEAD `26475d3` 上逐条核实。

## 0. TL;DR

- **主锚点 3 个**：`bx_hybrid V2`（promoted、单文件、5-band 连续频点）、`Millennia NSEQ-2`
  （promoted、单文件、4-band 含全参 mid）、`FreeEQ8`（unattested 但有内核层全链工件，固定 8-band ±24 dB）。
  `bx_cleansweep V2` 虽 promoted 但**没有 band gain 参数**，记为负锚点。
- **pluginprobe 与内核 param_id 同源**（FreeEQ8 实证 8/8 一致），可放心用作离核参数面代理。
- **dB 域只能在内核层运行时发现**（display_probe / eq_band_summary）；pluginprobe 只给 normalized 域。
  Millennia 的 Gain Range 开关证明静态硬编码 dB 域不安全。
- 注入建议：**独立文件 `~/.vit/free_state_experiment_plugins.json`**，不要塞 `config.json`
  （其 Save 会用 EngineConfig 重写整文件、抹掉未知键）；端口 instantiate 带 `plugin_path`、
  identifier 留空；写入用 `set_params_batch` + `normalized_value`，回读 `value_text` 物理解析。

## 1. 数据源与快照

| 数据 | 来源 | 快照位置 |
|---|---|---|
| PCA 候选 | `GET http://127.0.0.1:7878/agent/processor-certification/candidates?family=static_eq`（2026-08-27 09:47） | `temp/pca_static_eq_snapshot.json`（工作树，未提交） |
| 语义清单 | `C:\Users\timoz\.vit\plugin_semantics.json`（993 条，eq 家族 148，built_at 2026-07-27） | 原地 |
| 参数面（离核） | pluginprobe HTTP 宿主（127.0.0.1:9318，native worker） | `temp/d2_1_5_s1_pluginprobe/*.json` |
| 参数面（内核层，7 月既有） | `scripts/_eq_probe_diag_out/*_params.json`（2026-07-26） | 原地 |
| param_id 约定样本 | `temp/c2-matrix-final/api_2500.parameters.json`（内核 get_plugin_parameters 真实输出） | 原地 |

PCA 静态 EQ 快照事实：**976 个候选中 11 个 promoted**（status 分布 965 unattested + 11 promoted）。
11 个 promoted = 7 个 Plugin Alliance 单文件 + 4 个 Waves 壳（WaveShell1-VST3 17.1_x64.vst3）。
static_eq 家族无认证 runner（`agent/internal/chat/processor_certification_entry.go:420`
inspect_only 分支；`:300` 对应 capability 状态 "inspect-only"）。

## 2. 候选挑选（PCA × 语义清单交叉）

准则：单文件非壳（内核路径直载可用，见 §4.2）、参数面小、有明确 per-band gain dB 参数、
PCA 有 attestation 优先。交叉结果：976 个 PCA static_eq 候选全部在语义清单中，其中 155 个
语义 eq 家族。

| 插件 | PCA identifier | status / attestation | plugin_path | 判定 |
|---|---|---|---|---|
| bx_hybrid V2 | `VST3-bx_hybrid V2-d0ef306f-c141eb4b` | promoted / `pca1_fb62975730a6e717fdcf36aa` | `C:\Program Files\Common Files\VST3\Plugin Alliance\bx_hybrid V2.vst3` | **主锚点 A** |
| Millennia NSEQ-2 | `VST3-Millennia NSEQ-2-a4bdda31-2a71015c` | promoted / `pca1_9ec6e7b40f92151bf0ec62b8` | `...\Plugin Alliance\Millennia NSEQ-2.vst3` | **主锚点 B** |
| FreeEQ8 | `VST3-FreeEQ8-3b344656-cf7df7de`（bundle 根）/ `VST3-FreeEQ8-c1cbccf4-cf7df7de`（内层二进制） | unattested / — | `C:\Program Files\Common Files\VST3\FreeEQ8.vst3`（目录） | **主锚点 C**（内核层工件最全） |
| Knif Audio Soma | `VST3-Knif Audio Soma-2af06274-d4f55bc1` | promoted / `pca1_c8c63dc7d301ee83ea6370d1` | `...\Plugin Alliance\Knif Audio Soma.vst3` | 备用（参数面最小） |
| bx_cleansweep V2 | `VST3-bx_cleansweep V2-7342ab36-ec8752c7` | promoted / `pca1_276879ffe78479ac2f48b0fb` | `...\Plugin Alliance\bx_cleansweep V2.vst3` | **负锚点**：仅 HP/LP cutoff + Input Gain，无 band gain |
| Marvel GEQ | `VST3-Marvel GEQ-fe15cf0c-13159728` | unattested / — | `C:\Program Files\Common Files\VST3\Marvel GEQ.vst3` | 备用（16-band ±12 dB，有 7 月内核工件） |

排除 Waves 壳 4 个 promoted（API-550A/560、EMO-F2、SSL EV2）：壳路径对内核路径直载
是多插件歧义（§4.2 `:1212-1213`），且 Waves 参数无稳定 ID（§3.1），不适合今晚第一跳。

## 3. 参数面证据（pluginprobe 离核 dump，2026-08-27）

### 3.1 param_id 约定（内核/Tracktion 层）

三份独立证据收敛出的规则——**param_id 来自 JUCE hosted parameter ID，不做任何 `band_%d_gain` 式假设**：

1. **稳定大数字串**：插件声明稳定 VST3 参数 ID 时，param_id 就是该 ID 字符串。
   FreeEQ8 内核层 band gain param_id = `422259119`…`622663176`（`scripts/_eq_probe_diag_out/FreeEQ8_params.json`
   explain.eq_band_summary），与 pluginprobe 的 `id` **8/8 完全一致**（代理验证，本会话）。
   pluginprobe 侧该派生逻辑在 `PluginProbe/native-host/src/main.cpp:353-366`
   （`HostedAudioProcessorParameter::getParameterID()`，否则回落 `host-index:N`，`:365`）。
   Plugin Alliance 全系候选 `stable_id=true`、`id_provenance="vst3_hosted_parameter_id"`。
2. **数字索引**：无稳定 ID 的插件回落字符串化索引。api_2500（Waves 壳）实测 `"0"`…`"15"`
   （`temp/c2-matrix-final/api_2500.parameters.json`，param Thresh/Attack/Ratio…）。
3. **小写名**：宿主按名注册的参数用小写名，如 api_2500 的 `"dry level"` / `"wet level"`；
   `"10000001"` 是 bypass 约定 ID。

跨插件观察：Plugin Alliance 系列共享同一 param_id 空间（如 `1282360881` 在 NSEQ-2 与
Soma 中都是 "Low Frequency 1"）——param_id 只在插件内解释，不可跨插件复用。

### 3.2 各锚点 band-gain / 频点参数（ch1；ch2 的 id 通常 +1 或独立段）

**bx_hybrid V2**（v2.14.0.0，103 参数；ch2 gains `843867719/843869511/843933255/843607367/843605575`）：

| band | gain param_id | gain 名 | freq param_id | freq 默认 | Quality（形状） |
|---|---|---|---|---|---|
| Lf | `827090503` | 1 Lf Gain | `827090502` | 63 Hz | `827090513` LoShelf |
| Lmf | `827092295` | 1 Lmf Gain | `827092294` | 315 Hz | `827092305` = 0.5 |
| Mf | `827156039` | 1 Mf Gain | `827156038` | 3.15k Hz | `827156049` = 0.7 |
| Hmf | `826830151` | 1 Hmf Gain | `826830150` | 6.30k Hz | `826830161` = 0.3 |
| Hf | `826828359` | 1 Hf Gain | `826828358` | 11.20k Hz | `826828369` HiShelf |

频点连续可调（is_discrete=false），unit=Hz 已声明；另有 EQ On/Off `1165053806`。

**Millennia NSEQ-2**（v1.12.0.0，48 参数）：

| band | gain param_id | freq param_id | freq 特性 | Q | 备注 |
|---|---|---|---|---|---|
| Low | `1282361137` | `1282360881` | 56 Hz 离散 | — | Shelf 开关 `1282364209` |
| Low-Mid | `1280132913` | `1280132657` | 连续（X10 档 `1280137265`） | `1280135473` | 全参 bell |
| High-Mid | `1213024049` | `1213023793` | 连续（X10 档 `1213028401`） | `1213026609` | 全参 bell |
| High | `1214859057` | `1214858801` | 16.0 kHz 离散 | — | Shelf 开关 `1214862129` |

⚠️ **Gain Range 开关**（`1382967089`，默认 "10"）改变各 band 增益的实际 dB 域——dB min/max
是开关状态的函数，不可静态硬编码。

**FreeEQ8**（内核层 7 月工件，explain.eq_band_summary）：固定 8 band，
80/250/500/1000/2000/4000/8000/12000.001 Hz，gain 域 ±24 dB linear，
gain param_id `422259119/450888270/479517421/508146572/536775723/565404874/594034025/622663176`。
频点固定 → admission 的 `frequency_hz` 直接映射最近 band，`band_index` 语义 1:1。

**Knif Audio Soma**（备用，v1.2.0.0，59 参数）：boost `1282359857`(LF,120Hz)/`1280131633`(LM,470Hz)/
`1213022769`(HM,2k7)/`1214857777`(HF,6k8)，unit=dB 已声明；Trim `1416785201`；频点全离散；
EQ On `1162956593`。

**bx_cleansweep V2**（负锚点）：9 参数 = HP cutoff `1213220466` / LP cutoff `1280329330` /
Input Gain `1197566318` + 模式开关。**无任何 per-band gain**——任何"promoted 即可用"的挑选
逻辑都会踩进这个坑。

### 3.3 display domain 约定

- **pluginprobe 层（离核）只给 normalized 域**（0..1 scale=normalized）+ 当前 display_text；
  它是 observation-only（无参数写），不做 value→text 采样（`agent/internal/pluginprobe/types.go`
  SurfaceParameter；`adapter.go:1-2` 注释"no parameter-write"）。
- **dB min/max 在内核层运行时发现**，两条已实现的通道：
  1. `plugin.get_parameters` 的 display_probe：内核在 normalized 0/0.25/0.5/0.75/1 采样
     value_to_string，推断 `display_domain_candidate`（min/max/unit/scale/confidence）。
     api_2500 实证：19 参数全部有候选，18 个高置信（如 Dry Level "-12~0 dB" linear）。
  2. `plugin_grabber.explain_controls` 的 `eq_band_summary.gain_domain`（FreeEQ8 实证 ±24 linear）。
- 成熟换算机器（今晚直接复用）：`agent/internal/chat/plugin_eq_control.go:1703` `eqNormalizedFromCurve`
  （曲线反解 normalized，支持非线性/单调递减），`:618` `physicalReadbackForRole`（value_text 物理回读），
  `plugin_compressor_apply.go:200-203` 事务探测 `plugin.set_params_batch` + `normalized_value` + readback。

## 4. 今晚 S1 注入设计建议

### 4.1 插件引用来源：独立机器本地文件（不要 config.json）

- 建议新增 `~/.vit/free_state_experiment_plugins.json`（形状自定，例如
  `{"static_eq": {"plugin_path": "...", "param_binding": {...}}}`）。机器特定路径只落这里，
  agent 代码零路径（任务卡红线）。
- **不要塞 `~/.vit/config.json` 顶层键**：config.Load 只 unmarshal 进 `EngineConfig`
  （`agent/internal/config/config.go:166-169`），而 config.Save 用 `json.MarshalIndent(cfg)`
  重写整个文件（`:197-202`）——任何非 EngineConfig 的顶层键会在下一次设置保存时被静默抹掉。
- admission TypedAction 覆写已接好：`agent/internal/chat/free_state_d1_runtime.go:171` 已读
  `TypedAction["plugin_identifier"]`，:173 才回落 `defaultD1StaticEQPluginIdentifier="juce_eq"`
  （:38，占位符，20260827_090337 实证内核报 not found）。优先级建议：TypedAction > 本地文件 > 硬失败
  （删掉 juce_eq 回落）。
- 语义清单/PCA 机器（`pca_store_path`、`semantic_index_path`，见 §1 响应字段）**只用作存在性
  与家族校验的交叉验证源**，不做挑选决策——148 个 eq 里自动挑一个是机器相关不稳定决策。

### 4.2 端口如何带 plugin_path

内核解析语义（`VitApp/Source/Service/PluginRackControlService.cpp:1110-1223`
`resolveExternalPluginDescription`）：

- **identifier 非空** → knownPluginList 精确匹配 identifier string，查不到立即失败、不回落路径
  （`:1119-1152`，失败文案 "plugin_identifier not found in known plugin list"）。暂存内核
  knownPluginList 为空 → 任何 identifier 都失败（现状 20260827_090337 的直接原因）。
- **identifier 空 + plugin_path** → 存在性检查（`:1160`，`juce::File::exists()` **接受目录**）→
  缓存未命中则 VST3 格式 `findAllTypesForFile` 直接内省单个插件文件（`:1190-1219`），成功即
  `normaliseAndRegisterExternalPluginDescription` 注册进 knownPluginList（`:1218`）——**免扫描**。
- 多插件（壳）→ 明确拒绝（`:1180-1181` 缓存歧义 / `:1212-1213` 内省歧义）。

落地：`StaticEQVSPPort` 的 instantiate 载荷（`agent/internal/executionports/staticeq_vsp.go:113-118`）
当前只带 `plugin_identifier`；S1 改为注入引用的 `plugin_path` + 空 identifier。
路径形状注意：**内核接受 bundle 目录**（FreeEQ8 根目录可用）；**pluginprobe worker 只认文件**
（`existsAsFile()`，`PluginProbe/native-host/src/main.cpp:272-273`，FreeEQ8 需用
`Contents\x86_64-win\FreeEQ8.vst3`）。PA 单文件插件无此差异。

### 4.3 set / 回读换算方案

- 写入：内核 `set_plugin_param` 原生支持 `normalized_value`（0..1 夹取，
  `PluginRackControlService.cpp:1845-1849` + `:1876-1879` setNormalisedParameter）与
  `value_text`（`:1850-1858` stringToValue）；VSP 侧 `plugin.set_params_batch` 同样三选一
  （`VspKernelReference.cpp:2703-2714`）且参与 base_revision CAS（`:2669`）。**不需要新内核命令。**
- 换算：运行时从 `plugin.get_parameters`（内核 handler `PluginRackControlService.cpp:2865-2910`，
  参数描述符 `:2898`）读 `display_domain_candidate`，linear 域用 `(dB-min)/(max-min)`，
  非线性/可疑域走 `eqNormalizedFromCurve`（plugin_eq_control.go:1703）。当前端口的坏语义：
  set 发原始 dB `value`（staticeq_vsp.go:133-140 的 `"value": target`）+ 回读按 0.001 容差比对
  dB 目标（`:159-161`，读 value/current_value/normalized_value，`:263`）——对归一化插件必失败，
  S1 必须换成 normalized 写 + value_text 物理回读（`new_value_text` 在 set_plugin_param 回执
  `:1906`；batch 回读走 get_plugin_parameters + physicalReadbackForRole 模式）。
- **双通道写形状**：PA 插件 ch1/ch2 是独立参数。一次 static_eq 动作 = 单次 `set_params_batch`
  同时写 ch1+ch2 两个 gain param——原子、单 revision 前进，保持 D1"恰一个前向变异"不变量
  （experiment 判据在 `agent/internal/experiment/d1s1_domains.go:46-75`，gain_db ±2 语义不变）。
- band 映射：第一晚只动 gain、不动 freq/Q（单变异纪律）。FreeEQ8：frequency_hz → 最近固定
  band；bx_hybrid/NSEQ-2：运行时按 eq_band_summary/参数名匹配频点最近的 band。
  `band_%d_gain` 组装（free_state_d1_runtime.go:170）在真实插件上不成立，S1 下沉为域表
  `D1S1DomainSpec` 的参数绑定字段（d1s1_domains.go:14 现有结构即扩展点）。

### 4.4 建议的今晚注入序列（真实栈晚窗）

1. `~/.vit/free_state_experiment_plugins.json` 写入主锚点 A（bx_hybrid V2，promoted + 单文件 +
   连续频点 + 5 band 覆盖面广）；锚点 B/C 留作烟测变体。
2. 烟测 `scripts/run_free_state_d1_smoke.ps1 -PublicCaseId spv1_p01`（frequency flavor）
   期望 kernel 不再报 identifier not found；回执出现 value_text 物理读数。

## 5. 治理问题（只记录，不裁决）

**D1 static_eq 的 VSP 执行通道绕过 PCA 载入门。** PCA 载入门
（`agent/internal/harness/harness.go:1665` `enforceAgentProcessorLoadGate`）只覆盖 agent HTTP
invoke 通道的 `rack_add_node / rack.add_node / plugin.load_to_rack / instantiate_plugin /
plugin.instantiate`（命令清单 `:1652-1659`，触发条件含 `req.Source != ""` `:1667`），
注释明示"UI/C++ calls do not enter Harness and remain unaffected"（`:1664`）。D1 执行端口经
`SendVSPLegacyCommandWithIDs` 直连内核（staticeq_vsp.go:113-118），不在 Harness 辖内。

实测（2026-08-27 复核）：
- agent invoke 通道 instantiate_plugin → HTTP 400 `pca_load_gate: missing non-forgeable exact
  selection authorization`（对应 harness.go:1672）；
- VSP 通道同日直达内核并收到**内核**错误（20260827_090337 的 "plugin_identifier not found in
  known plugin list: juce_eq"）——门从未触发。

选项（待用户裁定）：
- A：要求 D1 实验装载也走 PCA 授权（例：为有界实验签发一次性 authorization，
  参照 `AuthorizeProcessorCertificationLoad` harness.go:1643-1650 的 narrow-exception 模式）；
- B：维持"有界实验 = 独立治理面"（现有防线：±2 dB 判据、单变异、budget 1、烟测封口、
  环境注入的插件白名单）。

注意叠加关系：active free-state 推理循环的 HTTP invoke 另有一道独立关闭
（`agent/internal/chat/free_state_invoke_guard.go:84-101` 把 instantiate_plugin/
plugin.load_to_rack 列为禁用变异名）——那是模型通道，与上述 VSP 通道是两个面。

## 6. 复现与工件

```powershell
# PCA 快照（栈须在跑）
curl -s "http://127.0.0.1:7878/agent/processor-certification/candidates?family=static_eq" -o temp/pca_static_eq_snapshot.json

# pluginprobe 离核 dump（bundle 目录要用内层二进制；worker 只认文件）
cd agent\bin
.\pluginprobe.exe -worker "..\..\PluginProbe\native-host\build\pluginprobe_vst3_worker_artefacts\Release\pluginprobe_vst3_worker.exe" -listen 127.0.0.1:9318
# 另一终端：POST /v1/plugin/load {"plugin_path":"C:\\...\\bx_hybrid V2.vst3"} → GET /v1/plugin/snapshot

# 内核层参数发现（加载后，agent invoke 只读通道不受 pca_load_gate 影响）
#   plugin.get_parameters {track_id, plugin_id, include_parameters:true}
#   plugin_grabber.explain_controls {track_id, plugin_id}   → eq_band_summary
```

本会话工件：`temp/d2_1_5_s1_pluginprobe/{bx_cleansweep_V2,millennia_nseq2,bx_hybrid_V2,knif_audio_soma,harris_doyle_natalus_dsceq,freeeq8_probe_snapshot}.json`
（工作树，未提交；7 月内核层工件在 `scripts/_eq_probe_diag_out/`）。
