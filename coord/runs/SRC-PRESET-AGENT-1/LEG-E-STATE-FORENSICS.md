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

## §4 内容实证层（2026-11-16 21:0x 腿 D 闸门判定：binary_309_identical——版本差假说证伪）

**腿 D 闸门实测**（主管授权 10 分钟栈时；run=coord/runs/SRC-PRESET-AGENT-1/legd_gate_20261116_2115/，attempt3 PASS，占用 21:02→21:10）：

- 现行内核（VitApp.exe sha256 `a5947361…`，374/377 同一二进制复用+零内核改动）XML-boot → add_track → instantiate Surge XT（776 参=298 全等）→ `VIT_DUMP_CLEAR_XML=1` sidecar 通道取明文样本（attempt1 `save_project{}`=require_path、attempt2 `save_as .xml` 被 normalizeProjectPathForSave 强改 .vit+加密——两轮 probe_defect 装置域，产品面全程 ok）。
- **判定：`binary_309_identical`**——现行内核产出 state 解码后与 309 模板工件头 32 字节逐字相同，init-patch 全等（67294B，blob sha256 `f245cce2…`）。**「11-09 构建与现行子模块版本差」假说被证伪：高熵二进制是现行内核的本征 state 形态。**
- 附加取证：`set_plugin_param` Global Volume 0.9（读回 ok）→ 再存 → state **逐字不变**——疑 master volume 在 Surge patch 序列化之外（腿 D 后续做 patch 内参数差分时须选 patch 内参数）。

**§4 结论重写**：

1. 309 工件非陈旧产物；`58 d9 0c` 形态=现行内核本征。腿 E 此前据内核源码推出的「XML 包裹明文」预期与该二进制行为**不符**——即本仓 tracktion 子模块检出的 JUCE 源码与被测二进制在此路径上存在未解释的出入（构建期源码态或未核对的包装层），**源码级 XML 假说降级为「未证伪但被实测压倒」**，按实测为准。
2. **腿 D 路径裁定依据（主管预设分支 B）**：仍高熵 → **腿 D 走内核亲存 state 路径（309 先例：真栈装载→设预设→save→sidecar 提取 state 入模板）**；fxp→state 离线合成不可行（内容层结构未解+无内核侧 setState 通道）。
3. state 随补丁内容变化的确认留腿 D（选 patch 内参数做差分；Global Volume 已证不在序列化内）。

## 解闸动作——已完成（2026-11-16 21:10）

## 工件与证据索引

- 解码/回环/双样本对比脚本与输出：本报告成文过程中的命令记录在卡面回执（一次性实验，未留脚本文件；字母表与格式已写入腿 A 包内 none——按「报告不落代码」口径，算法细节以本报告为准）。
- 样本：`coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/template.v1.xml`（state 属性）；`coord/runs/WAVESHELL-OFFRENDER-FIX-1/stackleg_20261109_1906/e2a_baseline_gr|e2b|e3…/*.vit`、`e4_surge_xt/*.vit`。
- 源码锚点：全部在正文行号在卷。
