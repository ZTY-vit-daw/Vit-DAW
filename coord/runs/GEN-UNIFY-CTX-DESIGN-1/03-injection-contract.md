# 设计件三：注入契约（提示词模板结构 + 硬约束机检）

- 归属：GEN-UNIFY-CTX-DESIGN-1 / 2026-10-11
- 卡面硬要求：提示词模板结构=自动上下文块+用户自由块，与生成插件壳的模板引擎**共用同一契约**；硬约束机检=生成产物 MIDI 回读校验速度/调性，不合格拒写入。
- "生成插件壳"形式化：仓库无该实体（grep coord/ 全库仅本卡提及——用户口径概念）。其雏形=AIGC-DRUM-GEN-1 卡面"固定 prompt 模板"（coord/cards/todo/2026-10-10-AIGC-DRUM-GEN-1.md:17）。本件把"壳"落为**共用模板引擎**（§1 渲染器）+**快照**（件一 §4）+**机检门**（§4）：任何生成消费方（内置编排/外部 API 适配器/VitReel/编曲）产提示词必须经同一渲染器，产物必须过同一机检矩阵——这就是"共用同一契约"的可验收形态。

## 1. 模板结构（generation prompt v1）

```
<generation_prompt> :=
  [MUSIC CONTEXT 块]        ← 自动上下文块：快照渲染，只读，消费方不可增删改
  [CONSTRAINTS 块]          ← 硬约束声明：从 hard/hard-when-set 字段派生的 MUST 列表
  [GUIDANCE 块]             ← soft/inject-only 字段的建议性/中性描述（有值才出现）
  ---
  [USER 块]                 ← 用户自由块：用户原文，永远置尾、永不加料
```

渲染示例（快照=件一 §4 示例值，scene=in_project）：

```
== MUSIC CONTEXT (project, music.context.v1, revision ab12cd) ==
tempo: 120.0 BPM (initial; automation: none)
time signature: 4/4
key: C major (set by user)
style note: 暖色钢琴主导，克制
sections: verse [0-64 beats]
harmonic: (not provided)
energy: (not provided)

== CONSTRAINTS (machine-checked before write) ==
MUST 1: note times are in beats (clip-relative, beat 0 = clip start), time_unit:"beats"
MUST 2: rhythm positions land on the 4/4 grid (16th-note resolution; tolerance ±1/64 beat)
MUST 3: all MIDI pitches are in the C major pitch set {60,62,64,65,67,69,71} + octaves
MUST 4: velocity 1-127, length > 0

---
<user free text verbatim>
```

### 1.1 渲染规则

- **单一渲染函数**（模板引擎本体）：`RenderGenerationPrompt(snapshot MusicContextSnapshot, userFreeText string, profile RenderProfile) string`。消费方只传 profile（声明消费哪些块/哪些字段，见 §2），不允许传自定义模板文本（防口径漂移——模板演进=引擎版本演进）。
- **未设置项不静默消失**：MUSIC CONTEXT 块内未设置项渲染为 `(not set)` / `(not provided)` 行（审计可见——生成失败时能区分"没注入"与"注入了但未设置"）；GUIDANCE 块有值才出现（token 经济）。
- **用户块契约**：原文置尾、去首尾空白外零处理、永不与自动块拼接改写。价值主张落点：自动块承载一切工程约束，用户块=纯创作意图表达。
- **坐标系注记强制入模板**（MUST 1）：MIDI note start=**clip 相对 beats**（非时间线绝对拍——GEN-MAC-BASELINE-1 report.md §二.3 实测注记；catalog.go:538-539 argspec `time_unit:beats` 亲验）。这是生成线对 LLM 最易错的约定，入硬约束声明。
- **分级语气映射**：hard→MUST（编号列表，逐条可机检对应）；soft→"style note:"前缀陈述；inject-only→中性键值行。禁止 soft 项使用 MUST 措辞（防止 LLM 把建议当约束）。

### 1.2 与 chat 装配面的关系（不共享装配器，共享风格）

生成提示词**不走** promptruntime 的 chat 装配（prompt.go:12-29 的 Section 五 kind 体系服务于对话流 system/user 装配）；生成面是独立渲染函数。理由：对话面 Section 有 CacheKey/指纹/历史归一化语义，生成面不需要；强行复用会把生成提示词卷入对话缓存演化。风格对齐（块标题+trim 规则）即可。

## 2. RenderProfile（四消费方取用面的机制化）

```go
type RenderProfile struct {
    ConsumerID   string   // "gen_api" | "gen_plugin" | "arrange" | "vitreel"
    IncludeFields []string // 默认全七项；VitReel 可裁 ["style","energy","sections","tempo","key"]
    IncludeBlocks []string  // 默认全四块
}
```

- profile 只能**裁剪**不能**加料**（无自定义文本入口）——级别不可降（件一 §1）、内容不可改，只有消费范围可选。
- 外部 API 适配器（gen_api）拿到渲染终文后只做协议转换（拼入其 API 的 prompt/style/genre 字段），转换不得删改 MUSIC CONTEXT/CONSTRAINTS 块内容（适配器验收项）。

## 3. 机检矩阵（产物形态三分）

**核心实证（机检为什么必须是 agent 侧写前门）**：内核 MIDI 写路径对 pitch/velocity 是**钳制不是拒绝**（MidiService.cpp:822/826/942/947/1076/1081/1228/1233 亲验：`juce::jlimit(0,127,pitch)` / `jlimit(1,127,velocity)`）——越界值被静默改写而非报错。**内核不可能承担 fail-closed 拒写**，机检门必须在 agent 侧生成编排器内、发写命令之前。写命令面=add_midi_notes/add_midi_notes_bulk/apply_midi_note_patch/insert_midi_clip（CommandDispatcher.cpp:2609-2657 注册亲验；catalog.go:1006-1009 RiskConfirm 分级亲验）。

| 产物形态 | 机检内容 | 挂点 | 拒写行为 |
|---|---|---|---|
| **A. 内置编排产物**（note 序列，AIGC-DRUM-GEN 系） | ①调性音级集合（key 已设置时）：全部 pitch ∈ 音级集合（件一 §2 定义，确定性集合判定）②拍号网格：start 对齐分母最小实用 subdivision（默认 1/16）容差 ±1/64 beat ③既有越界面扩展：音域（jlimit 边界=设计上限，如 24-96）、密度上限、长度上限（AIGC-DRUM-GEN-1 卡面"音域/密度/长度上限→明确报错不写入"既有语义归入门内统一）④结构合法：pitch 0-127、velocity 1-127、length>0（**写前拒**——不等内核钳制） | 生成编排器产物→`ValidateNotes(snapshot, notes)`→通过才发写命令 | 任一违例=不发任何写命令，逐项 `constraint_violation{field, expected, actual, note_index}` 记录（AGENTS §8 失败分类纪律；入回执——AIGC-DRUM-GEN-1 卡面 FREE_STATE_IMPROVEMENT_EXECUTION_RECEIPT_V1 schema 承载生成参数/种子/候选，违例记录并入同 schema） |
| **B. 外部 MIDI 文件产物**（import 路径，`import_midi_to_track`，catalog.go:534-535 argspec 亲验） | ①SMF tempo meta vs 工程 tempo_bpm（精确或 ±0.5 BPM，实现卡定）②SMF 拍号 meta vs 工程 time_signature（不一致时 v1=拒导入 fail-closed，附" regenerate with project tempo"指引；**转调选项不提供**——统一性优先，用户确需移调走工程内显式 transpose 命令）③调性（key 已设置时）：导入前解析 note 集合∈音级集合（SMF note 解析在 agent 侧做——导入命令发之前） | 适配器/编排器产物文件→`ValidateMidiFile(snapshot, path)`→通过才发 import 命令 | 同 A：拒发命令+分类记录 |
| **C. 外部音频产物**（WAV/MP3，Suno/MusicGPT 类） | **v1 无机检面**（音频 tempo/调性分析不做——按需开发原则+集合商原则）。注入的 MUST 声明照发（生成端自律约束），产物不机检。 | 无门（边界如实声明） | —（v2 候选：Basic Pitch 音频转 MIDI 后复用 A/B 门——工具选型已在 2026-09-29-thesis-positioning-decision.md:17 记录） |

**tempo 硬约束在矩阵中的承载方式澄清**（设计精确性关键）：note 序列以 beats 表达时，tempo 不进入音符数据（时间语义由内核 `tempoSequence.toTime` 换算，ClipService.cpp:158/MidiService.cpp:125 先例）——A 形态的 tempo 合规=**坐标系契约（beats）+注入 tempo 值**共同保证，机检退化为网格对齐（②）；tempo 的**显式机检对象**=B 形态（SMF meta vs 工程值）与 C 形态 v2 分析。此澄清不降低"速度=硬约束"级别——A 形态下速度违规在结构上不可能（beats 坐标系即工程速度）。

## 4. 回读校验（写入后第二道门）

写命令成功后**必须**回读核验（对齐 GEN-MAC-BASELINE-1 的回读一致验收模式——report.md §一.③"最终回读数学一致"）：

- 回读面：`get_midi_clip_notes`（CommandDispatcher.cpp:2645 注册、catalog.go:1003 RiskDirect 亲验——读命令免确认，回读零交互成本）。
- 核验项：note 数量/pitch/start/length/velocity 逐项 vs 发送序列（既有数学一致模式）+ **pitch 集合 ⊆ 音级集合**（若 key 已设置——防内核钳制改写后的集合漂移：写前 pitch=95 合法但被钳制场景已在写前拦截，此处防的是 id 映射/坐标系错位类事故）。
- 回读不一致=写入事故（非生成违例）：立即报告+标记 clip 待人工处置，**不自动删除**（用户工程内已落数据，删除属破坏性操作须显式确认）。

## 5. 坐标系与命令形态注记（消费方接入须知，实测出处）

- note `start`=**clip 相对 beats**（clip 起点为 0；GEN-MAC-BASELINE-1 report.md §二.3 实测：clip2 插在 beat 8，其内 note start=0.0/1.0/…）。
- `mutate_midi_notes` 必须**逐 note 携带 id**（内核拒区域形式，"each note requires a non-empty id"——本卡基线亲验=MidiService.cpp:1181；GEN-MAC-BASELINE-1 report.md §二.2 所引 1125-1144 为其当时 HEAD 行号，上游已漂移；argspec 漂移已另开 MIDI-ARGSPEC-ALIGN-1 卡，todo/ 在案）——机检门后若需修正已有 note，走 by-id 形态。
- SMF chunk id 静默容忍注记（GEN-MAC-BASELINE-1 §二.4）：B 形态解析器不得依赖内核报错信息定位坏文件——导入前 agent 侧预解析（机检①②③本来就要预解析，天然规避）。

## 6. 落位（供拆卡）

全部 agent 侧、新包 `agent/internal/genctx/`（零踩踏既有包——capabilitycontext 避让纪律延续 RLM-PROFILE-2 先例）：

```
agent/internal/genctx/
  snapshot.go        // MusicContextSnapshot + 从 get_project_state 响应组装（件二 §2.5）
  render.go          // RenderGenerationPrompt + RenderProfile（§1-2）
  validate.go        // ValidateNotes / ValidateMidiFile + 音级集合表（§3）
  readback.go        // 回读核验（§4）
```

单测面：音级集合表 11 scale 全枚举×根音采样、网格对齐容差边界、违例分类完整性、渲染未设置项可见性、profile 裁剪边界、用户块原文保真（含尾随空格/换行攻击样例）。
