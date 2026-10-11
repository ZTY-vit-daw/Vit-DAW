# 设计件一：上下文清单与分级契约（music.context.v1）

- 归属：GEN-UNIFY-CTX-DESIGN-1 / 2026-10-11
- 设计前提（用户已定，本件在此之上定稿细节）：速度/拍号=硬约束（产物机检 fail-closed）；调性=起步用户手填工程级字段（分析推断留 v2）；风格/气质=软引导。
- 交叉引用：`../../runs/GEN-MAC-BASELINE-1/report.md`（MIDI 命令面坐标系注记 §二.3）；`../../decisions/2026-09-29-thesis-positioning-decision.md:16-17`（统一性合规率实验与生成线三层愿景）；`../VIS-FILM-DESIGN-1/01-film-contract.md:111-113`（VitReel 咬合点）。

## 1. 分级语义（先定义分级，再逐项套用）

| 级别 | 语义 | 义务链 |
|---|---|---|
| **hard**（硬约束） | 产物**必须**符合；不符即拒写入（fail-closed） | 披露恒有值 → 注入 MUST 声明 → 产物机检 → 不合格拒写+失败分类记录 |
| **hard-when-set**（设置即硬约束） | 字段已设置时按 hard 执行；未设置时无该约束（用户未约束即无违反），注入块明示"未设置" | 同上，但存在未设置态 |
| **soft**（软引导） | 影响提示词语气与生成倾向，不做产物验收门 | 有值注入建议性描述；无机检 |
| **inject-only**（仅注入） | 有值才注入、无值省略；中性陈述，无 MUST 无建议语气 | 仅注入；无任何强制 |

分级是**消费方无关的属性级常量**（同一字段在所有消费方同一级别）——不允许某消费方私降级别（防止"某插件悄悄把调性从硬约束降为软引导"的统一性漏洞）。消费方只能选择**是否消费**（取用面，§3），不能选择**以何级别消费**。

## 2. 上下文清单（七项定稿）

> "最小集"=v1 落地面（件二/件三的键与字段）；"完整集"=分层扩展方向。v1 不实现项标 v2。

| # | 项 | v1 级别 | v1 数据源 | v1 状态 | v2 方向 |
|---|---|---|---|---|---|
| 1 | 速度 tempo_bpm | **hard** | 内核 `edit->tempoSequence.getTempo(0)->getBpm()`（读取侧与 `set_tempo` 写入侧同 API，CommandDispatcher.cpp:3468 亲验） | 恒有值（引擎缺省自动补 120，见件二 §1.2） | tempo 自动化（多 tempo setting）按段生效 |
| 2 | 拍号 time_signature | **hard** | 内核 `edit->tempoSequence.getTimeSig(0)`（`getStringTimeSig()`/`numerator`/`denominator`，tracktion_TimeSigSetting.h:33,50 亲验） | 恒有值（引擎缺省自动补 4/4） | 拍号变化段（time_sig_setting_count>1 的分段披露） |
| 3 | 调性 key_signature | **hard-when-set** | **用户手填工程级字段**（新建 `vit_music_key_signature` edit state 属性，件二 §2） | 可缺省（未设置） | 分析推断（音频→调性估计，写回同一字段并标 `set_by:"analysis"`） |
| 4 | 和声 harmonic | **inject-only**（v1） | 生成调用时的用户自由文本（不持久化） | 可空 | 和声进行投影（分析派生）；编曲线正典前置消费 |
| 5 | 段落结构 section_structure | **inject-only**（v1） | 既有 markers 披露（get_project_state `markers` 键，CommandDispatcher.cpp:3421 亲验，零新面） | 可空（无 marker 即无） | 段落投影/编曲线段落自动化（C4 前置） |
| 6 | 能量曲线 energy_curve | **inject-only**（v1） | 生成调用时的用户自由文本（不持久化） | 可空 | DOM/RLM 派生能量包络（分析产物，v2 挂观察投影体系） |
| 7 | 风格/气质 style | **soft**（v1） | 用户手填工程级字段（新建 `vit_music_style_note`，与调性同壳，件二 §2.4） | 可缺省（空串） | 风格标签受控词表+参照曲embedding（远期，集合商原则下不自研） |

**分级与前提的对应核对**：前提①"期望产物与工程音乐特征同步"→ #1/#2/#3 hard 链；前提②"自动封装越丰富用户自由块越大"→ #4-#7 注入面丰富度只加不减用户自由；前提③分级原案逐字落实（速度拍号 hard / 调性用户手填 / 风格 soft）。

**调性音级集合的确定性定义**（机检依据，件三 §4 引用）：调性字段展开为**音级集合**（pitch-class set）= 根音 pitch-class + `Scale::ScaleType` 步进表（对齐 tracktion 枚举名，tracktion_Musicality.h:77-90 亲验：major/minor/ionian/dorian/phrygian/lydian/mixolydian/aeolian/locrian/melodicMinor/harmonicMinor 共 11 种）。agent 侧维护等价半音步进硬表（不入内核依赖；`Scale::getSteps` 形态对照 tracktion_Musicality.h:125）。机检只做**集合成员判定**（确定性可判定），不做旋律性/和声美感等启发式判定。

## 3. 四消费方取用面

四消费方=生成 API（外部适配器）/生成插件（内置编排）/编曲/VitReel（用户口径四家，2026-10-11 晚窗）。取用面=消费哪些字段+以什么形式拿（注入契约渲染细节归件三）。

| 消费方 | 消费字段 | 形式 | 机检面 |
|---|---|---|---|
| **生成 API**（MusicGPT/Stability/Gemini/MusicGen 官方 API 型 + Suno 会话型，集合商原则下的适配器） | 全七项 | 完整注入提示词（自动块+用户块，适配器只转协议不改内容） | MIDI 产物（文件导入路径）全机检；**音频产物 v1 无机检面**（边界声明：注入 MUST 声明带 tempo/调性，产物不做音频分析——v2 候选=Basic Pitch 转 MIDI 后再检，thesis-positioning-decision.md:17 工具首选已定） |
| **生成插件**（内置编排，AIGC-DRUM-GEN 系；"生成插件壳"的实体形态=共用模板引擎+编排器，件三 §1 形式化） | 全七项（按插件 profile 选块） | 共用模板引擎渲染（同一契约，卡面硬要求） | 产物 note 序列全机检（写前门+回读，件三 §4） |
| **编曲**（arrange 线） | 全七项，**#4 和声/#5 段落主消费方** | 注入契约+编曲正典前置（汇合段重构后吸收，AIGC-DRUM-GEN-1 卡面注记对齐） | 编曲产物（MIDI）同机检 |
| **VitReel**（成片） | #7 风格、#6 能量、#5 段落为主；#1/#2/#3 作元数据 | 注入契约渲染进 film 生成提示词；成片**数据面**永远取最终混音（VIS-FILM 01-film-contract.md:111-113——上下文只影响"画面气质"，不影响数据面来源） | 无新增机检（VIS-FILM 既有校验器体系管辖画面合规；音乐统一性由上游生成/混音环节机检承载） |

混音=v2 可选消费者（设计前提①），v1 不定义取用面，仅保证快照 schema 字段命名不与之冲突。

## 4. 快照数据模型（music.context.v1）

注入的**唯一权威中间形态**：所有消费方从同一快照渲染，禁止各自直连内核字段旁路（防口径漂移）。

```json
{
  "schema_version": "music.context.v1",
  "captured_at": "<RFC3339>",
  "project_revision": "<graph revision，对齐 get_project_state 既有 appendGraphRevisionProperties 语义>",
  "scene": "in_project | free_creation | experiment_control",
  "tempo_bpm": 120.0,
  "tempo_setting_count": 1,
  "time_signature": "4/4",
  "time_signature_numerator": 4,
  "time_signature_denominator": 4,
  "time_sig_setting_count": 1,
  "key_signature": {
    "root_pitch": 60,
    "root_name": "C",
    "scale": "major",
    "set_by": "user",
    "set_at": "<RFC3339>"
  },
  "style_note": "暖色钢琴主导，克制",
  "harmonic_note": "",
  "sections": [ {"name": "verse", "start_beat": 0.0, "end_beat": 64.0} ],
  "energy_note": ""
}
```

- `key_signature: null` = 未设置（hard-when-set 的未设置态）；`sections: []` = 无 marker；空串 = 文本项未填。
- `harmonic_note`/`energy_note`/`sections` 的 provenance 在 v1 分别为"调用入参/调用入参/内核 markers"——快照不区分 provenance 字段（v2 分析项加入时再扩 `provenance` 面，schema_version 升 v2，消费方 fail-closed 拒绝未知版本）。
- 字段来源映射：tempo/拍号/count ← music_context 披露键（件二）；key_signature/style_note ← music_context 披露键（内核字段）；sections ← markers 键换算（marker 名→段落名、时间→beat）；harmonic_note/energy_note ← 生成调用参数（scene 见件四）。
