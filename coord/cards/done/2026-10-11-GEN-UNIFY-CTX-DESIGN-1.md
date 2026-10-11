# GEN-UNIFY-CTX-DESIGN-1：工程统一化·音乐上下文中枢设计（生成线首位设计卡）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚窗（依据=用户三线同步裁定："它应该被设计成工程级音乐上下文中枢——对的，只不过按需开发原则，进入生成能力开发阶段所以现在开发"）
- 派发确认：已确认（用户 2026-10-11 晚窗同步）
- 验收负责人：GLM 主管决策流
- 池序 70；目标仓库=D:\Vit_DAW；设计卡零生产代码
- 归属：**Mac 生成线首位**（生成/编曲/VitReel 的公共脊柱；执行端=Mac 会话，或用户指定的 PC 设计会话——VIS-FILM 先例）
- 优先级 / 预估 / 依赖：P1 / 设计 1-1.5 天 / 无（生成线后续实现卡依赖本卡契约）
- 模型分级：L1 / GLM 执行会话

## 设计前提（用户已定，不再讨论）

1. 目标：生成/编曲/VitReel 的提示词**由工程发出时携带工程统一性内容**，期望产物与工程音乐特征同步；混音为 v2 可选消费者。
2. 价值主张（生成插件同源）：上下文自动封装越丰富/详细/准确，用户可自由发挥的提示词范围越大。
3. 起步分级（主管提议、设计卡内定稿）：速度/拍号=硬约束（产物机检，fail-closed 拒写）；调性=起步用户手填（工程级字段新建）+分析推断留 v2；风格/气质=软引导。

## 设计五件

1. **上下文清单与分级契约**：最小集→完整集分层数据模型（速度/拍号/调性/和声/段落结构/能量曲线/风格），逐项标硬/软/仅注入，及各消费方（生成 API/生成插件/编曲/VitReel）的取用面。
2. **披露面设计**：内核 tempo/拍号披露键（get_project_state 增披露——ARR 勘察定级"小改"，CommandDispatcher.cpp:3279-3437 一带，领取/设计时亲验行号）；调性=工程级新字段（含旧工程缺省语义+持久化往返，AGENTS §11）。
3. **注入契约**：提示词模板结构=自动上下文块+用户自由块（与生成插件壳的模板引擎**共用同一契约**）；硬约束机检设计（生成产物 MIDI 回读校验速度/调性，不合格拒写入）。
4. **场景边界**：工程内生产=强制携带；自由创作/艺术家自主场景=豁免机制（预答辩创作实验直接消费此边界）。
5. **最小实现拆卡建议**：披露键小卡/字段持久化卡/注入与机检卡/生成卡接入改造——序列与工作量估算，达可直接发卡程度。

## 文件域

设计零生产代码；产出落 coord/runs/GEN-UNIFY-CTX-DESIGN-1/（契约文档+数据模型+拆卡规格）。

## 验收标准

设计五件齐+锚点（披露面/回读校验面给到文件:行或 grep 实证）；与 VIS-FILM 设计件、MIDI 命令面（GEN-MAC 首勘 report）交叉引用；实施卡规格达到可直接发卡。

## 停止条件

设计中发现披露面/命令面与既有勘察锚点重大不符（上游已变）→ 锚点上交。

- 领取：2026-10-11 晚 / origin/main cf46f950 / GLM 执行会话 ZCode Mac（worktree Vit-DAW-wt-genctx）/ 领取提交=本提交（coord-only 直推 main）
- 回执：**设计基线 [coord/runs/GEN-UNIFY-CTX-DESIGN-1/README.md](../../runs/GEN-UNIFY-CTX-DESIGN-1/README.md)（总览+验收对照+10 条关键裁定）+ 设计件一至五 + 锚点机检底账（anchor-verification.txt，39 项 file:line 断言全 PASS）**。核心结论：①分级四级制（hard/hard-when-set/soft/inject-only）——级别是属性级常量，消费方只能裁剪消费范围不能降级；调性专属 hard-when-set（未设置=不机检+注入块明示）。②披露=单对象键 music_context（tempo/拍号恒有引擎真值——TempoSequence::setState 空列表自动补缺省实证 :141-144；对齐 block_size"无值键缺省"先例纪律）。③**调性否决原生 PitchSequence**（空序列自动插 pitch=60 →"已设置/未设置"不可区分，hard-when-set 无法成立），改用 vit_* edit state 属性+两命令 set_project_key/set_project_style_note，值域与原生对齐留 v2 迁移通道；§11 四要求逐条落位（含 .vit 加密容器双路径往返验收项）。④**机检=agent 侧写前门（唯一 fail-closed 面）**——内核写路径 jlimit 钳制不拒绝（MidiService 8 处实证）→ 内核不可能承担拒写；门=编排器内 ValidateNotes/ValidateMidiFile→通过才发写命令+写后回读二道门。⑤tempo 硬约束承载澄清：note 序列（beats 坐标系）形态下速度违规结构上不可能——显式机检对象=外部 MIDI（SMF meta vs 工程值）与 v2 音频分析；不降级别，是级别的正确落法。⑥音频产物 v1 无机检面（如实声明）；v2 候选=Basic Pitch 转 MIDI 复用同门。⑦scene 三枚举（in_project 强制默认/free_creation 豁免/experiment_control 论文合规率实验"关"臂）——调用级参数不落工程字段，防豁免泄漏成默认；ARTIST-AUDITION 原则逐条映射消费。⑧注入引擎单一化（"生成插件壳"落为共用 RenderGenerationPrompt，消费方只传 profile 无自定义模板入口；未设置项可见占位不静默；用户块原文置尾永不加料）。⑨agent 新包 genctx 零踩踏（不碰 capabilitycontext/promptruntime）。⑩拆卡 4 张（DISCLOSE 0.5d→KEYFIELD 1d 同文件域串行→INJECT 1d 可并行→CONSUME 0.5-1d 依赖 AIGC-DRUM-GEN-1；外部 API/VitReel/编曲消费归各线首批卡验收项）+互斥文件清单。零生产代码零真栈（锚点全只读勘察，GEN-MAC-BASELINE-1 run4 证据引用不重跑）；卡面锚点亲验：3279-3437"一带"成立（实测函数体 3291-3448）；MIDI-ARGSPEC-ALIGN-1 转引行号漂移已修正（1125-1144→1181）。
- 验收：（裁定文件）
