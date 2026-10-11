# GEN-UNIFY-CTX-DESIGN-1：工程统一化·音乐上下文中枢设计基线（总览）

- 执行：GLM-5.3 ZCode 执行流会话（Mac）/ 2026-10-11；领取 origin/main=cf46f950（rebase 后领取提交 12105183）；worktree `~/Documents/Vit-DAW-wt-genctx` / 分支 `port/gen-unify-ctx-design-1`
- 性质：设计卡，零生产代码（全部产出=本目录设计件；对仓库源码零 diff）
- 定位：生成/编曲/VitReel 的公共脊柱（用户 2026-10-11 晚窗裁定："工程级音乐上下文中枢——按需开发原则，进入生成能力开发阶段所以现在开发"）

## 1. 设计五件（索引）

| 件 | 文件 | 内容 |
|---|---|---|
| 一 | [01-context-inventory.md](01-context-inventory.md) | 七项上下文清单（速度/拍号/调性/和声/段落/能量/风格）×四级分级契约（hard/hard-when-set/soft/inject-only）×四消费方取用面（生成 API/生成插件/编曲/VitReel）+ 快照数据模型 music.context.v1 |
| 二 | [02-disclosure-surface.md](02-disclosure-surface.md) | get_project_state 增 music_context 披露键（锚点亲验+恒真值证明）；调性/风格工程级字段（vit_* 属性裁定——含否决原生 PitchSequence 的实证理由）+ 两命令 + AGENTS §11 全语义 |
| 三 | [03-injection-contract.md](03-injection-contract.md) | 生成提示词四块模板（MUSIC CONTEXT/CONSTRAINTS/GUIDANCE/USER）+ 单一渲染引擎（"生成插件壳"形式化）+ RenderProfile + 机检矩阵（产物三分：note 序列/外部 MIDI/外部音频）+ 回读二道门 + agent 侧写前门实证（内核只钳制不拒） |
| 四 | [04-scene-boundary.md](04-scene-boundary.md) | scene 三枚举（in_project 强制默认/free_creation 豁免/experiment_control 对照臂）；预答辩创作实验（ARTIST-AUDITION）消费映射；产物落点判据；豁免不可叠加纪律 |
| 五 | [05-impl-card-specs.md](05-impl-card-specs.md) | 实施卡规格 4 张（可直接发卡）+ 序列并行图 + 互斥文件清单 |

## 2. 卡面验收项逐条结论

| # | 验收要求 | 结论 |
|---|---|---|
| 1 | 设计五件齐 | ✅ 本目录五件+本总览 |
| 2 | 披露面锚点（文件:行或 grep 实证） | ✅ 件二 §1.3：`handleGetProjectState`=**CommandDispatcher.cpp:3291-3448**（卡面"3279-3437 一带"亲验成立；披露键现状=该函数内**无** tempo/拍号键——`grep -n tempo CommandDispatcher.cpp` 全部命中为 beat↔time 换算与 set_tempo，无披露）；set_tempo=:2514 注册/:3451-3481 实现；读侧 API 同面 :3468 |
| 3 | 回读校验面锚点 | ✅ 件三 §3/§5：写命令注册 CommandDispatcher.cpp:2609-2657；argspec catalog.go:538-545（time_unit:beats）；风险级 catalog.go:1003-1009（写=RiskConfirm/读=RiskDirect）；**内核钳制不拒绝** MidiService.cpp:822-1233（jlimit 8 处）→ 机检必须 agent 侧写前门的实证依据；回读面 get_midi_clip_notes=:2645 |
| 4 | 与 VIS-FILM 设计件交叉引用 | ✅ 件一 §3（VitReel 取用面：成片数据面永远取最终混音，VIS-FILM 01-film-contract.md:111-113）；README §3 裁定 6（film/ 目录先例）借鉴于字段落位讨论 |
| 5 | 与 MIDI 命令面交叉引用 | ✅ 件三 §1.1（MUST 1=clip 相对 beats 坐标系，GEN-MAC-BASELINE-1 report.md §二.3）；§5（mutate by-id 形态 MidiService.cpp:1125-1144+MIDI-ARGSPEC-ALIGN-1 卡在案；SMF chunk 静默容忍 §二.4） |
| 6 | 实施卡规格达可直接发卡 | ✅ 件五：每卡目标/文件域/验收（命令+断言）/停止条件/依赖/工作量+互斥清单 |

## 3. 关键设计裁定汇总（主管验收聚焦点）

1. **分级四级制**：hard / hard-when-set（调性专属——已设置才机检，未设置不造约束）/ soft / inject-only；分级是属性级常量，消费方只能裁剪消费范围不能降级（防统一性漏洞）。
2. **披露=单对象键 music_context**：不散铺扁平键；tempo/拍号恒有引擎真值（TempoSequence::setState 空列表自动补缺省，tracktion_TempoSequence.cpp:141-144 实证）——与 block_size"无值键缺省"先例（TIM-KERNEL-DISCLOSE-1）性质不同但同纪律合规。
3. **调性否决原生 PitchSequence**：空序列自动插 pitch=60（C 大调缺省，tracktion_PitchSequence.cpp:101-102 实证）→"已设置/未设置"不可区分，hard-when-set 无法成立；改用 vit_* edit state 属性（vit_project_uuid 同款先例），值域与原生对齐（root_pitch 0-127+11 scale 枚举名）留 v2 迁移无损通道。
4. **机检=agent 侧写前门（唯一 fail-closed 面）**：内核写路径 jlimit 钳制不拒绝（8 处实证）→ 内核不可能承担拒写；门=生成编排器内 ValidateNotes/ValidateMidiFile→通过才发写命令；写后回读二道门（数学一致+音级集合复验）。
5. **tempo 硬约束的承载澄清**：note 序列（beats 坐标系）形态下速度违规结构上不可能（beats 即工程速度）——tempo 显式机检对象=外部 MIDI（SMF meta vs 工程值）与 v2 音频分析；此澄清不降级别，是级别的正确落法。
6. **音频产物 v1 无机检面**（如实声明不冒充）：Suno/MusicGPT 类产物只注入 MUST 声明不做产物分析；v2 候选=Basic Pitch 转 MIDI 后复用同门（工具选型已在 2026-09-29 论文定位裁定记录）。
7. **scene=调用级参数、默认即强制**：不设全局开关不落工程字段（防豁免泄漏成默认）；experiment_control 独立枚举承载论文合规率实验的"关"臂（提示词面板开/关对照的机制化），与 free_creation 生产豁免隔离。
8. **注入引擎单一化**："生成插件壳"落为共用 RenderGenerationPrompt（消费方只传 profile 无自定义模板文本）；未设置项渲染为可见占位（不静默消失）；用户块原文置尾永不加料。
9. **agent 侧新包 genctx 零踩踏**：不碰 capabilitycontext/promptruntime（chat 装配与生成渲染分离的理由见件三 §1.2）；capabilitycontext 避让纪律延续 RLM-PROFILE-2 先例。
10. **拆卡 4 张序列**：DISCLOSE(0.5d)→KEYFIELD(1d，同文件域串行)→INJECT(1d，可与 KEYFIELD 并行窗口)→CONSUME(0.5-1d，依赖 AIGC-DRUM-GEN-1)；外部 API/VitReel/编曲消费不另开卡（归各线首批卡验收项，引用 profile 契约即可）。

## 4. 交叉引用与依据清单

- 卡面：coord/cards/doing/2026-10-11-GEN-UNIFY-CTX-DESIGN-1.md（领取回填在案）
- MIDI 命令面首勘：coord/runs/GEN-MAC-BASELINE-1/report.md（五命令真栈 30/30 门；clip 相对 beats 坐标系 §二.3；argspec 漂移 §二.2；SMF 静默容忍 §二.4）
- VIS-FILM 设计基线：coord/runs/VIS-FILM-DESIGN-1/（README+01-film-contract.md——VitReel 咬合=成片数据面取最终混音 :111-113；设计件风格先例）
- 论文定位裁定：coord/decisions/2026-09-29-thesis-positioning-decision.md:16-17（统一性合规率实验=提示词面板开/关对照；生成线三层愿景=编曲工具面/生成集成面/工程统一性底座——本设计=底座件；集合商原则与 Basic Pitch 选型）
- 预答辩创作实验：coord/cards/todo/2026-10-10-ARTIST-AUDITION-1.md（豁免场景直接消费方，已后置于生成线首批+编曲前置）
- 生成首卡：coord/cards/todo/2026-10-10-AIGC-DRUM-GEN-1.md（"固定 prompt 模板"=生成插件壳雏形；回执 schema 承载违例记录的先例）
- 生成能力勘测：coord/runs/GEN-CAP-RECON-1/report.md:73（生成编排全 agent 侧、内核零改动的形态裁定——本设计机检门同形态）

## 5. 工件清单

```
coord/runs/GEN-UNIFY-CTX-DESIGN-1/
  README.md                    ← 本文件
  01-context-inventory.md      设计件一：清单与分级契约+快照模型
  02-disclosure-surface.md     设计件二：披露键+工程级字段+§11 语义
  03-injection-contract.md     设计件三：注入模板+机检矩阵+回读门
  04-scene-boundary.md         设计件四：场景边界与豁免
  05-impl-card-specs.md        设计件五：拆卡规格（4 张）
```

## 6. 端测边界声明（AGENTS §5）

本卡零生产代码：无 webui/渲染面/用户旅程改动，无 agent 行为改动；全部产出为设计文档。锚点均为只读勘察（文件:行 + grep 实证，出处逐条内嵌于各件）。无真栈执行需求（GEN-MAC-BASELINE-1 run4 已提供命令面行为证据，本卡引用不重跑）。
