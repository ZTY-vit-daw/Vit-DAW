# TIM-KERNEL-GAPS-1：TIM 断言器内核腿缺口八项细化报告（设计 §5 展开）

- 执行侧：Mac 夜间托管会话（纯只读勘察，零代码改动）
- 领取时 origin/main：`83473dd`（领取提交 `b104ca3`）
- 输入：`docs/TIM_ASSERTER_V1_DESIGN.md` §5（:190-201，8 项清单）——**停止条件核查通过**：文档在位、8 项与卡面预期一致
- 领取时 `git status --short`：`M VitApp/Workspace/Settings.xml`、`M VitApp/Workspace/default_project.xml`、`?? .zcodeignore`、`?? VitApp/Workspace/Artifacts/`、`?? coord/runs/FIX-PCA-AUTOSWEEP-1/20260925_mac/`——领取前已有，未触碰
- 日期：2026-09-27；行号以本卡工作 HEAD 为准
- **锚点漂移声明**：设计文档 §5 引用的 L3AcousticAnalyzer 行号（:697-722/:713-716）相对当前 HEAD 已漂移约 +70 行（TIM 合入后）；本报告全部使用当前 HEAD 复核过的锚点

---

## 0. 执行摘要

1. 八项逐项四要素齐（§2）；**两项快赢**（DC offset 2-4h、hygiene 回传 1-2h 内核工时）、**一项能力面修正**：设计文档写"aux\_send 全库零命中"——实际 Tracktion 引擎自带 AuxSendPlugin/AuxReturnPlugin（`tracktion_engine/.../tracktion_AuxSend.h:14+`），缺的只是 VitApp 披露面，不是路由能力。
2. 共性前置三组（§3）：**A 组"get\_project\_state 披露扩展模式"**（items 3/4/5 共享，建议一张准备卡先立扩展点）、B 组"L3 证据字段扩展模式"（items 1/2/6）、C 组"agent 断言器 v2 扩展模式"（全部共享，框架已在 agent/internal/tim/asserter.go 落地）。
3. 风险最高两项：BS.1770 LUFS（正确性需 EBU 测向量，建议 D9 排期 gate）、实时削波持久化（音频线程实时安全约束）。
4. 排卡建议六张（§2 逐项+§4 汇总），总量级：内核 ~40-65h + agent ~15-22h（含测试），不含跨仓取证。

## 1. 消费面与披露面（所有项的共同上下文）

- **agent 消费面**：TIM 断言器输入三腿=TrackFacts/RackSummaries/RackEdges（`agent/internal/tim/asserter.go:79-106`，自 project state 的 tracks/plugins/rack 块装配）；acousticAssertionRefs/rackAssertionRefs（:117-118）。
- **内核披露面**：`get_project_state`（`VitApp/Source/Service/CommandDispatcher.cpp:2491` 注册、:3268+ 实现）逐轨出 plugins/clips/rack 块（:3312-3314）——items 3/4/5/7 的数据出口；`L3AcousticAnalyzer`（1134 行）的 evidence 结构与 bake JSON——items 1/2/6 的数据出口。
- 文件规模（工作量依据之一）：L3AcousticAnalyzer.cpp 1134 行 / PluginRackControlService.cpp 3591 / VitHeadlessService.cpp 1694 / TransportAudioService.cpp 1571 / PluginListHygiene.cpp 100。

## 2. 八项逐项展开（四要素）

### Item 1：true peak（4x 过采样）→ AS-PEAK v2

- **实现锚点**：现状只有 sample peak——L3Evidence 结构 `peakAbs` 字段（L3AcousticAnalyzer.cpp:93-130）与主采样循环累积（:1005-1030，observe 调用 :1010-1012、peakAbs :1024-1025，逐样本 `std::abs` 最大值，无过采样）。**juce\_dsp 模块已链接**（现有用法：`juce::dsp::FFT`/`WindowingFunction` :969-971；TiledSpectrogramBaker/VitProductionCoordinator 亦用）——`juce::dsp::Oversampling` 可直接引入，无需新依赖。
- **工作量**：内核 10-16h + agent 4h。依据：过采样器接入离线分析通道（构造 4x oversampling、逐块 process、对过采样流取峰值）+ evidence 结构/bake JSON 出 true\_peak\_dbtp 字段（模式同 :516-517/:595-596 的 sum\_abs/max\_abs 出口）+ 红绿测试（正弦过载样本：sample peak ≤0dBFS 但 true peak >0 的构造用例）；复杂度参照现有 FFT 段接入（:969-971 同等 wiring 面）+ 测试。
- **依赖**：内核设施已存在（juce\_dsp）；离线 bake 上下文，不碰实时路径。
- **排卡建议**：独立卡（B 组模式）；**必须带"算法诚实标注"决策点**（见风险）。
- **风险**：Technical——BS.1770-4 的 true peak 定义绑定特定 4x 过采样滤波器；JUCE Oversampling 用半带多相滤波，是近似而非规范滤波器。若标注 "BS.1770 compliant" 是过度声明，建议算法标签诚实命名（如 `true_peak_oversampled_4x_v1`）并保留 approximate 语义直到实现规范滤波器。对内核实时性无影响（L3 是离线分析）。

### Item 2：DC offset（带符号均值）→ AS-SIG v2

- **实现锚点**：现状只有无符号 `sumAbs`（L3AcousticAnalyzer.cpp:103 字段、:127 累积）——无法判直流偏移。
- **工作量**：内核 2-4h + agent 1-2h。依据：在 observe() 累积处加一行带符号和（:127 旁）、evidence/bake 出 dc\_offset（线性值+比值 dc\_offset\_ratio=|sumSigned|/frameCount）两字段；改动面 <30 行+测试。**八项中最小**。
- **依赖**：无（纯增量）。
- **排卡建议**：与 item 5 合一张"内核卫生小卡"（合计 ~1 天）。
- **风险**：近零（带符号均值数学平凡）；只需注意与 all\_zero 判定（:442）交互的测试。

### Item 3：块长一致性入 project state → AS-SR v2

- **实现锚点**：`current_buffer_size` 已在 `get_audio_device_status` 命令输出（TransportAudioService.cpp:719，含 sample rate :718）——但不在 get\_project\_state；agent 侧 AS-SR 输入装配拿不到它就无从对照。
- **工作量**：内核 2-4h + agent 1-2h。依据：在 handleGetProjectState（CommandDispatcher.cpp:3268+）响应加 audio\_settings 摘要块（数据源同 :719 的 device setup 读取，模式复制现有块拼装）。
- **依赖**：无新设施；建议与 item 4/5 共用 A 组扩展点一次成型。
- **排卡建议**：与 item 4 合一张"project state 披露扩展"卡。
- **风险**：语义——buffer size 随设备切换运行时可变，快照语义需钉住（沿用 graph\_revision/observed\_at 戳记模式，断言指称"哪一刻"的块长）。

### Item 4：per-instance 插件装载态出内核 → AS-PLUGIN v2

- **实现锚点**：`describeExternalPluginLoadState`（PluginRackControlService.cpp:1335-1352）已计算 name/id/format/path/enabled/processing/async/load\_error 八字段**但只进日志**；装载回执仅 rack add 命令时（:3161-3172 的 plugin\_load\_state 属性）；get\_project\_state 的 plugins 数组（createProjectStatePluginsArray，CommandDispatcher.cpp:897 定义、:3312 挂接）不含装载态。
- **工作量**：内核 3-5h + agent 2-3h。依据：把 describe 的字符串输出重构为字段、并入 createProjectStatePluginsArray 每插件行（+8 字段）；改动集中在状态拼装，不碰装载逻辑。
- **依赖**：无；A 组共享。
- **排卡建议**：与 item 3 合卡。
- **风险**：时序——async 初始化是瞬态，披露与消费之间可能变化；输出行需带 observed\_at/revision 戳，agent 侧断言区分 "async pending" 与 "failed"（v1 已有 fail-closed 语义可沿用）。

### Item 5：PluginListHygiene Report 回传 → 卫生监督

- **实现锚点**：hygiene 报告结构已有（typesRemoved/blacklistRemoved/removedTypePaths 等，PluginListHygiene.cpp:57-68 使用）——但只写一行日志（:64-68）。
- **工作量**：内核 1-2h + agent 1h。依据：report 结构化字段已存在，加一个查询命令（或并入 get\_project\_state）返回 JSON 即可，~40 行。
- **依赖**：无；A 组共享。
- **排卡建议**：与 item 2 合"内核卫生小卡"。
- **风险**：近零；只需定 JSON 形状（counts+截断路径摘要，对齐日志的防洪水策略 :57-59 注释）。

### Item 6：BS.1770 LUFS → 响度断言（D9 协同）

- **实现锚点**：现状近似 `rmsDb - 0.691`（L3AcousticAnalyzer.cpp:781-784，已标 `approximate_rms_lufs_v1` + approximate\_lufs 字段——诚实标注在位）。
- **工作量**：内核 12-20h + agent 2h。依据：真 BS.1770-4 = K 计权双二阶级联（高频架+高通）+ 400ms 滑窗方块和 + 绝对门（-70 LUFS）+ 相对门（-10 LU）两段门控；JUCE 无现成封装需手写；测试需 EBU R128 参考量向量（构造或取公开集）。
- **依赖**：无内核前置；**排期依赖 D9**（响度断言的消费方）。
- **排卡建议**：独立卡，**D9 排期 gate**（论文裁定：bonus 不是依赖——11-17 freeze 前若 D9 不开跑则顺延）。
- **风险**：八项中正确性风险最高——门控算法与滤波系数任一偏差都产出系统性错误 LUFS，直接污染 D9 响度断言；实现卡必须带参考向量红绿测试，算法标签区分 measured vs approximate（沿用 :784 的标签模式）。

### Item 7：实时削波持久化 → 播放中监督

- **实现锚点**：meters 轮询语义 `getAndClearOverload`/`getAndClearPeak`（VitHeadlessService.cpp:991）——读即清、不落盘，两次轮询之间的削波事件丢失。
- **工作量**：内核 4-8h + agent 2h。依据：在过载产生点（VSP client 的 overload 置位点上游）加粘性原子计数器（per-track clip\_count/last\_clip\_at），新增只读查询（不清零）或并入 meters 输出；持久化决策（会话内存计数 vs ValueTree 落盘）需在卡内定。
- **依赖**：需先定位 overload 的**产生点**（VSP client 侧实现，本卡未展开——实现卡第一步）。
- **排卡建议**：独立卡。
- **风险**：**音频线程实时安全**——计数器只能用 atomic，禁止锁/分配；粘性语义决策（按播放段清零？显式 reset 命令？）影响断言语义，需卡内裁决。

### Item 8：轨间 send/aux 拓扑 → AS-ROUTE v2 全量版

- **实现锚点**（对设计文档的重要修正）：**Tracktion 引擎自带 AuxSendPlugin**（`tracktion_engine/modules/tracktion_engine/plugins/internal/tracktion_AuxSend.h:14`——getGainDb/setMute/getBusNumber/getBusName/静态 getBusNames 全套 API；AuxReturn 同目录）——VitApp/Source 内 `aux_send` 零命中的含义是**披露面缺失**而非能力缺失。现 rack 披露在 createRackState（CommandDispatcher.cpp:1793 定义、:3314 挂接），AuxSend 实例未被枚举为路由边。
- **工作量**：内核 4-6h + agent 3-4h。依据：枚举 AuxSendPlugin 实例进 rack 路由边（字段现成：bus/gain/mute；模式同现有插件枚举）；agent 侧 RackEdges 加 send 类型边+环检测（意外反馈环全量版）。
- **依赖**：内核设施已存在；**跨仓前置**——DAW 前端（仓库外）是否实际创建 aux send 未取证，若前端从不创建则断言覆盖恒空（仍有负证据价值：拓扑完整性=无 send）。
- **排卡建议**：独立卡或并入 AS-ROUTE v2；实现前决策侧补一步前端用法取证（跨仓，本卡不做）。
- **风险**：空转风险（前端未用 send 则断言恒 vacuous-ready）；环检测在合并图（rack 边+send 边）上的复杂度小但测试面增长。

## 3. 共性基础设施识别（三组）

- **A 组：get\_project\_state 披露扩展模式**（items 3/4/5，item 7 的查询面同型）——"内核状态块拼装（CommandDispatcher.cpp:3268+）→ agent mixboard 键注册 → TIM 输入装配"三段式。**建议一张准备卡先立一次扩展点**（audio\_settings 块+plugins 装载态字段+hygiene 块同一 PR 面），后续项只加数据不动管线。
- **B 组：L3 证据字段扩展模式**（items 1/2/6）——L3Evidence 结构（:93-130）增长+bake JSON 出口（:516-517/:595-596 模式）+evidence\_refs 家族+agent 字段消费；items 1/6 共享"标准合规测量的诚实标注"关切（算法标签+approximate 语义，:784 已有先例）。
- **C 组：agent 断言器 v2 扩展模式**（全部八项）——三态纯谓词框架已落地（agent/internal/tim/asserter.go，485 行，五断言器+三腿装配+§11 fail-closed）；每项新数据=新断言规则+红绿测试，无框架改动。**三组中唯一已建成的前置**。

## 4. 排卡汇总（优先级序）

| 序 | 卡 | 合并项 | 量级 | 优先级理由 |
|---|---|---|---|---|
| 1 | 内核卫生小卡 | item 2+5 | ~1 天 | 最小改动最快收益；A/B 组模式各立一例 |
| 2 | project state 披露扩展 | item 3+4（A 组扩展点） | ~1-1.5 天 | 三断言器（SR/PLUGIN/卫生）同卡解锁 |
| 3 | true peak | item 1（B 组） | ~2-2.5 天 | AS-PEAK v2 主缺口；独立卡带标注决策点 |
| 4 | 实时削波持久化 | item 7 | ~1-1.5 天 | 播放中监督；实时安全 review 是关键路径 |
| 5 | BS.1770 LUFS | item 6（B 组） | ~2-3 天 | **D9 排期 gate**；正确性风险最高需测试向量 |
| 6 | aux 拓扑披露 | item 8 | ~1-1.5 天 | 前置跨仓取证（前端 send 用法）；空转风险 |

## 5. 验收对照

| 卡面验收项 | 状态 |
|---|---|
| ① 设计 §5 清单逐项覆盖（8 项） | ✅ §2 八项全展开 |
| ② 每项四要素齐 | ✅ 锚点/工作量（含依据）/依赖/排卡建议逐项 |
| ③ 共性前置识别 ≥1 组 | ✅ 三组（§3），C 组已建成 |
| ④ 报告入库 | ✅ 本文件 `coord/runs/TIM-KERNEL-GAPS-1/KERNEL_GAP_EXPANSION.md` |
| 零代码改动 | ✅ 仅新增本报告文件 |
| 工作量给依据不拍脑袋 | ✅ 每项标注改动面行数/模式参照/测试构成；文件规模列 §1 |

## 6. 边界与未覆盖项

- item 7 的 overload **产生点**定位（VSP client 内部）未展开——实现卡第一步，预计 ≤1h 勘察。
- item 8 的前端 aux send 用法取证属跨仓（D:\\Godot 前端），留给决策侧派卡。
- 工作量为小时级粗估（卡面口径），非承诺值；实现卡领取时应按届时 HEAD 复核锚点（本卡已实证设计文档锚点会漂移 ~70 行）。
