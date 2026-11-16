# 腿 E 取证报告：.vit PLUGIN state 编码与 fxp↔state 转换可行性（零栈）

- 卡：SRC-PRESET-AGENT-1 腿 E（主管 2026-11-16 晚授权「顺手做」）；零栈零 LLM 零计费。
- 方法：权威源码逐层核对（内核实际链接的 tracktion fork 子模块 @ `60ab1a49` 主仓钉住版本 + 其内嵌 JUCE 子模块 + Surge XT 上游源码）+ 仓内真实工件解码实验（模板 state×1、309 run 同插件 state×3、跨插件 state×1）。
- 样本：melody-drum-v1 模板 Surge state（67294B，309 E4 fixture 逐字血统）；309 run e2a/e2b/e3 三份 Grand Rhapsody state（1780B×3）。

## 结论总览

| 层 | 问题 | 状态 | 依据 |
|---|---|---|---|
| 外层编码 | state 属性字符串格式 | **已解（铁证）** | 见 §1 |
| tracktion 层 | state 是否=插件 chunk 原文 | **已解（源码级）** | 见 §2 |
| 插件契约层 | Surge DAW state 与 .fxp chunk 是否同构 | **已解（源码级）** | 见 §3 |
| 内容实证层 | 内核实际保存工件内容 | **一处未解矛盾** | 见 §4 |

## §1 外层编码（已解，纠正勘察腿的弱结论）

- **格式**：`"<字节数>.<自定义 base64>"`，JUCE `MemoryBlock::toBase64Encoding`。
- **字母表（权威）**：`".ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+"`——**'.' 是值 0**，'A'=1 … 'Z'=26，'a'=27 … 'z'=52，'0'=53 … '9'=62，'+'=63；6-bit 组 MSB-first。编码表与解码表双向源码核对于**内核自己的 JUCE 子模块**（`D:/Vit_DAW/tracktion_engine/modules/juce/modules/juce_core/memory/juce_MemoryBlock.cpp:366-410`，getBitRange MSB-first 亲读）。
- **回环铁证**：按此字母表解码模板 state 得 67294 字节（与声明长度精确一致），再编码回字符串与原文**逐字相同**。
- **勘误**：勘察腿 RECON.md 的「'.'→e/6// 三候选替换」是在字母表未解时的碰巧同长误读（任意双射都保长度）；本腿以内核源码+回环实验定论。**腿 D 实现时必须用本节字母表。**

## §2 tracktion 层（已解，源码级）

内核 tracktion fork（主仓子模块 @ `60ab1a49`）`modules/tracktion_engine/plugins/external/tracktion_ExternalPlugin.cpp:1029-1045`：

```cpp
pi->getStateInformation (chunk);
...
state.setProperty (IDs::state, chunk.toBase64Encoding(), um);
```

还原侧（:1099-1102）：`chunk.fromBase64Encoding (s); pi->setStateInformation (chunk.getData(), chunk.getSize());`。

- **零包装零加密**：state 属性 = JUCE `AudioPluginInstance::getStateInformation` 输出字节的逐字 base64；还原 = 原字节直喂 `setStateInformation`。fork 与上游同形（sparse clone fork HEAD 交叉核对一致）。

## §3 插件契约层（已解，源码级：fxp chunk ≡ Surge DAW state）

Surge XT 上游源码（surge-synthesizer/surge，main 分支）：

- `SurgeSynthProcessor::getStateInformation`（src/surge-xt/SurgeSynthProcessor.cpp:1557）→ `surge->saveRaw(&data)`。
- `SurgeSynthesizer::saveRaw`（src/common/SurgeSynthesizerIO.cpp:739）→ `storage.getPatch().save_patch(data)`——即 `patch_header{tag "sub3", xmlsize LE32, wtsize[2][12] LE32} + patch XML + wavetable 数据`（SurgePatch.cpp:1360+）。
- **.fxp 同构铁证**：`loadPatchByPath`（SurgeSynthesizerIO.cpp:242-324）读 fxp 头（CcnK/FPCh/fxID='cjs3'）后取 `chunkSize` 字节 chunk，**直喂同一 `loadRaw`**——fxp chunk 体与 DAW state 是同一 `save_patch` 序列化。
- **推论（机械定义）**：fxp → 插件可接受 state = 剥掉 fxp 头取 chunk 体即可（chunk 体即 setStateInformation 可接受字节，经 §2 层直通插件）。

## §4 内容实证层（一处未解矛盾——离线合成的最后闸门）

**矛盾**：按 §2+内核现行 JUCE 源码（VST3 宿主 `getStateInformation` = `<VST3PluginState>` XML 包裹 + `copyXmlToBinary`（magic `0x21324356` LE + 明文单行 XML，无压缩；`juce_audio_processors_headless/processors/juce_AudioProcessor.cpp:946-962` 与 `format_types/juce_VST3PluginFormatImpl.h:2987-3005` 亲读），内核保存的 state 解码后应是 `56 43 32 21` 开头的 ASCII XML。

**实际**（全部按 §1 字母表解码）：

- 模板 Surge state 头：`58 d9 0c 85 4b 41 04 00 0f fe …`（高熵二进制，熵≈6.7/8，无 zlib/gzip/raw-deflate 流（偏移 0-39 扫描）、无 'sub3'/XML/任何已知魔数）。
- 同 run 三份 GR state（e2a/e2b/e3）**逐字相同**（sha256 同）→ 确定性序列化，差异区非时间戳。
- 跨插件对比（Surge 67294B vs GR 1780B）：前 3 字节相同（`58 d9 0c`），第 4-6 字节区相异（疑似长度/插件相关字段），随后 ~69 字节两插件**再次共同**，之后完全发散（各自载荷）。该结构与「[3B 常量头][长度区][跨插件共享头][载荷]」的容器形态吻合，但与任何已核对源码层的预期输出都不符。

**候选解释**（未定）：309 run（2026-11-09）的内核二进制所链 JUCE/路径与现行子模块源码不一致（旧构建或不同 pin）；或该构建存在一处未核对的附加变换。当前子模块源码是 301 卡（11 月中）插桩后的版本，与 11-09 构建有时间差。

**闸门含义**：离线「fxp→state 合成」的管道（§1 字母表 + §2 通道 + §3 剥头）已逐段定义，但**尚未端到端实证**——现行内核保存的 state 内容层与源码预期不符，合成件能否被内核 `setStateInformation` 接受未经证明。**腿 D 不得在验证前宣称免内核换血可行。**

## 解闸动作（腿 C/D 栈时顺带，约 10 分钟，不单独占栈）

1. 现行内核装载 Surge XT → save → 提取 state 属性 → 与 §4 预期（XML 包裹）和 309 工件（二进制）对比归层；
2. 若现行内核产出 XML 包裹：309 工件=旧构建产物，管道全通；用任一出厂 fxp 合成 state 写入工程 XML → open → 出声探针验证（腿 D 正路）；
3. 若现行内核仍产出二进制：对二进制层做同法取证（此时需内核二进制 provenance 核对，可能转内核域卡）。

## 工件与证据索引

- 解码/回环/双样本对比脚本与输出：本报告成文过程中的命令记录在卡面回执（一次性实验，未留脚本文件；字母表与格式已写入腿 A 包内 none——按「报告不落代码」口径，算法细节以本报告为准）。
- 样本：`coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/template.v1.xml`（state 属性）；`coord/runs/WAVESHELL-OFFRENDER-FIX-1/stackleg_20261109_1906/e2a_baseline_gr|e2b|e3…/*.vit`、`e4_surge_xt/*.vit`。
- 源码锚点：全部在正文行号在卷。
