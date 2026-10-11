# Mac 线状态汇报与再同步请求（2026-10-11 晚）

- 发出：Mac 决策侧会话（ZCode，角色跟会话不跟机器）/ 收件：PC 主管决策流
- 性质：Mac 生成线阶段汇报 + **用户最新口径传达 + 卡池一致性再同步请求**（用户将与 PC 侧另行沟通，本文件为沟通底稿与事实底账）

## 一、Mac 线已完成并验收的卡（全部 pass，裁定与工件均在库）

| 卡（完成序） | 裁定 | 交付摘要 |
|---|---|---|
| REFSCHEMA-M2X-1（10-09） | [pass](../rulings/2026-10-09-REFSCHEMA-M2X-1-pass.md) | tim+capabilitycontext 12 条 mix.read: refs 迁 vit://mom（snapshot=observation_id）；红先行 6 FAIL 亲复现；全量 90 包 0 FAIL；范围外族（dad:/project.state:/mix.observe）保留登记 |
| GEN-MAC-BASELINE-1（10-11） | [pass](../rulings/2026-10-11-GEN-MAC-BASELINE-1-pass.md)（Mac 决策侧与 PC 晚窗裁定双合流） | ①基线对齐+内核重建（K1/K2 祖先，内核 sha256 `9a70f064…`）②生成链五命令真栈首勘 30/30 门 exit 0（project 周期/midi 九命令数据级闭环/assets 入库入轨 take 栈/render 双腿含 K1 拒绝语义 Mac 首证）③MAC-RUNTIME-STACK.md 建立+占用释放闭环 ④缺口清单：A1 维持开放、argspec 漂移（→已由 PC 立卡 MIDI-ARGSPEC-ALIGN-1 采纳）、坐标系/SMF 两条注记 |
| GEN-UNIFY-CTX-DESIGN-1（10-11） | [pass](../rulings/2026-10-11-GEN-UNIFY-CTX-DESIGN-1-pass.md) | 设计五件+锚点底账 39 项；八条承重锚决策侧亲验零偏差（含 PitchSequence 自动 pitch60 否决原生调性字段、TempoSequence 恒真值、内核钳制不拒→机检前置）；**实施四卡规格已达可直接发卡**（GEN-CTX-DISCLOSE/KEYFIELD/INJECT/CONSUME） |
| A1-MAC-FORM-1（10-11） | [pass](../rulings/2026-10-11-A1-MAC-FORM-1-pass.md) | Mac 插件库 instrument 占比 14/719（全 Waves VST3 双 shell，无鼓源/GM）；命令面三层如实（kernel 原语通/Z2 槽+编排半通/PCA 准入结构性无通道=A1 本体）；**最短出声=路径 B 音频腿（全链 Mac 真栈已实证，仅缺鼓 wav 生成器一环，零新通道）**；路径 A（MIDI 出声）收敛在准入层待 A1 裁定 |

流程注记：GEN-MAC 出现一次"done 卡先于实现直推 main 致工件短暂缺失"，决策侧 cherry-pick 闭合并已记入 ruling 备忘；本机网络间歇失效（git 以 `-c http.https://github.com.proxy=` 直连兜底）。

## 二、用户 2026-10-11 晚最新口径（Mac 会话内陈述，待与 PC 正式再同步）

> **生成 ≠ MIDI。编曲才需要 MIDI；生成需要的是生成模型的调用能力。**

- 该口径与 2026-09-29 论文定位裁定的三层愿景可以对应：**生成=生成集成面**（生成驱动 CLI 适配器两型：官方 API 型 MusicGPT/Stability/Gemini/自托管 MusicGen、会话型 Suno 自建反代或中转站；集合商原则不自研）；**编曲=编曲工具面**（MIDI 音符级编排）。
- 与 2026-10-11 晚窗三线同步的"**编曲串行后置**"一致。

## 三、发现的卡池不一致（请求 PC 侧处置）

1. **AIGC-DRUM-GEN-1（LLM 生成鼓点音符序列→写 MIDI 轨）按上述口径属编曲工具面**，但卡面至今标"Mac 生成线首批第二张"——10-11 同步只改标了 GEN-VOCAL-SYNTH-1（后移）与 ARTIST-AUDITION-1（不可领），**该卡漏标**。请求：改标编曲串行后置；连带评估 GEN-UNIFY 拆卡 GEN-CTX-CONSUME-1（现依赖 AIGC-DRUM-GEN-1）的依赖关系是否随之调整。
2. **生成集成面首批项（生成插件异步作业模型/便利服务定位/Suno 反代/统一模板）在 todo 池中无对应卡**。请求明确：立卡归属与时间；与"VitReel W1 八卡"（已裁定归 Mac 线推进、尚未拆卡）是何关系；Mac 何时可领。
3. **A1 音源形态裁定建议挂起至编曲线启动前**：A1-MAC-FORM-1 事实包已在库备查；若生成线首批需要音频级试听，路径 B（agent 侧 wav 生成器→ingest→render→audition）零新通道可先行，不依赖 A1 裁定。

## 四、Mac 线当前状态

- doing 空；无在飞卡；执行 worktree 均已清理；MAC-RUNTIME-STACK 空闲。
- 待再同步结论重排 Mac 可领卡序（当前 todo 中 Mac 相关：GEN-CTX 实施四张未发卡、VitReel W1 八卡未拆——两者均以本次再同步结论为前置）。
- 环境事实（后续卡执行会用）：内核二进制 `/tmp/vit-genbase1-build/...`（sha256 `9a70f064…`，重建命令在 GEN-MAC report §二②）；代理 7890 间歇死，git 命令以直连旁路兜底。
