# 设计件三：模板引擎 + GUI 引导（结构化输入 → 稳定专业模板）

- 归属：GEN-PLUGIN-SHELL-DESIGN-1 / 2026-10-12
- 卡面对应：设计五件之 3——"结构化输入→稳定专业模板；按场景模板库；与 genctx RenderGenerationPrompt 的 gen_plugin profile 对接——设计件三 §2"（统一化件三 §2=RenderProfile）。
- 设计前提 1："可自调/按场景设计提示词模板；GUI 引导结构化输入固化稳定专业模板格式"。

## 1. 核心裁定：模板=USER 块编排器，永不触碰自动块

统一化件三 §1.1 钉死：单一渲染函数 `RenderGenerationPrompt(snapshot, userFreeText, profile)`，消费方**不允许传自定义模板文本**（防口径漂移）。本设计的解法（两不违）：

- **壳的"提示词模板"定义域=USER 块内部结构+生成参数映射**：模板把结构化字段（乐器/段落数/密度/情绪/参考……）编排成**专业格式的自由文本**（如分节陈述："段落：verse 8 小节 + chorus 8 小节；节奏：……"），加上用户原文，合成一个 `free_text` 串交渲染器置尾。
- **自动块（MUSIC CONTEXT/CONSTRAINTS/GUIDANCE）恒由 genctx 渲染器独家产出**——壳代码中不存在任何自动块拼装路径（件一 §6 单一性核对）。
- 价值主张落点（前提 1"上下文自动封装越丰富用户自由发挥范围越大"）：自动块承载一切工程约束（引擎管），模板管"用户怎么把意图说专业"（模板管）——**两层分离正是"自动封装丰富度只放不放用户自由"的机制形态**（统一化件一 §2 前提②核对同口径）。

模板产物（每次提交）：

```
free_text = compose(template, form_values) + "\n\n" + trim(user_raw_text)
gen_params = map(template.params_map, form_values)   // duration/count/style_tags/instrumental/title
```

`compose` 纯函数、确定性（同表单值同产物——可复现性对齐 AIGC-DRUM-GEN-1 的固定模板+固定种子纪律精神）。

## 2. 模板 schema（vit.genshell.template.v1）

```json
{ "schema_version": "vit.genshell.template.v1",
  "id": "song_segment_v1", "name": "歌曲片段",
  "description": "按段落结构生成完整编曲片段",
  "default_scene": "in_project",
  "fields": [
    { "id": "sections", "label": "段落结构", "type": "text",  "default": "verse 8 bars + chorus 8 bars" },
    { "id": "energy",   "label": "能量走向", "type": "single_choice",
      "choices": ["build-up", "steady", "strip-down"], "default": "build-up" },
    { "id": "density",  "label": "密度",     "type": "single_choice",
      "choices": ["sparse", "moderate", "dense"], "default": "moderate" },
    { "id": "mood",     "label": "情绪",     "type": "text", "default": "" }
  ],
  "user_block_composer": "sections→'Sections: {sections}'; energy→'Energy: {energy}'; density→'Density: {density}'; mood→'Mood: {mood}'",
  "gen_params_map": { "duration_seconds": { "from": "bars_field_or_default", "default": 60 }, "count": { "default": 2 } } }
```

- 字段类型 v1 收敛四种：`text / single_choice / number / duration`（受控词表=choices 数组——"固化稳定专业模板格式"的落点：选项词表化，防自由输入漂移；text 型仍留给创作意图）。
- `user_block_composer` 是**声明式编排规则**（字段→行格式），不是任意代码（壳按规则拼行——模板文件不可注入逻辑）。
- 存储：内置模板编译进壳（只读基准集）+用户模板 `~/.vit/gen_shell_templates.json`（`{"templates":[...]}`，可覆盖同 id/新增）；未知 `schema_version`=fail-closed（模板文件整份拒绝+GUI 明示，不静默部分加载——对齐件三未设置项不静默消失纪律）。
- 用户自调边界：模板可改字段/词表/编排规则（USER 块内自由）——**不可**声明自动块内容、不可降 hard 字段级别（schema 无此入口，统一化件一 §1 属性级常量纪律的机制保证）。

## 3. 内置场景模板库（v1 四张起步）

| id | 场景 | 关键字段 | 默认参数 |
|---|---|---|---|
| `song_segment_v1` | 歌曲片段（完整编曲） | 段落结构/能量/密度/情绪 | count=2, duration 60s |
| `pad_bed_v1` | 氛围垫（长音背景层） | 明亮度/起伏/长度 | count=1, duration 120s, instrumental |
| `rhythm_loop_v1` | 节奏 loop（音频型） | 风格/小节长度/复杂度 | count=2, duration 15s |
| `reference_continue_v1` | 参考续写（风格关键词驱动） | 参考描述/延续方向 | count=2 |

（鼓点 pattern **不在**此库——"生成≠MIDI；编曲才需要 MIDI"用户裁定 2026-10-11，AIGC-DRUM-GEN-1 卡面状态修订在案；本库全音频产物模板。）

## 4. GUI 结构（VSTGUI，六区）

```
┌ GenShell ────────────────────────────────────────────────┐
│ ① 模板选择（下拉：内置+用户模板）                          │
│ ② 结构化字段表单（按模板 fields 动态渲染：文本/选择/数字/时长）│
│ ③ 自由文本区（用户原文，=USER 块尾段）                      │
│ ④ 工程上下文面板（context_snapshot 只读展示：tempo/拍号/调性/ │
│    风格/段落+injection_ready 标记）+ 提示词预览（dry-run）    │
│ ⑤ 生成参数区：provider/模型下拉（provider_catalog）+ scene    │
│    选择（默认 in_project；free_creation 带警示）+ 区域        │
│    （track=本轨锁定，start/长度拍数）+ [生成] [取消]          │
│ ⑥ 作业区：状态/进度/失败类别+重试按钮（Retryable 才亮）/候选   │
│    take 列表+切换/试听（Bypass）/已落轨 clip 引用             │
└──────────────────────────────────────────────────────────┘
```

- **scene 消费映射（统一化件四边界的壳侧落实）**：默认 `in_project`（强制注入+落轨义务）；`free_creation` 可选（GUI 警示"零工程上下文注入，产物不落轨——下载至缓存目录，可手动导入"——件四 §3 判据=产物落点，free_creation 下 runner 不发 ingest，产物路径经 job_terminal 回执显示）；`experiment_control` **不进 GUI**（实验主持侧专用枚举，防实验流量伪装，件四 §1）。
- **提示词预览=dry-run**：④ 区"预览"按钮 → 请求经宿主→agent `render_only` 路径（runner 同一渲染函数、不调 genapi）→ 回显渲染终文（四块全貌）。**预览=实发**的保证：同一条 agent 代码路径，壳无本地拼装（件一 §6）。
- GUI 刷新：500ms 定时器轮询宿主服务（进程内，件二 §1）；作业事件另有 T2 推送时即时刷新（优化非依赖）。
- 手测边界（AGENTS §5）：插件 GUI 视觉验收=用户经 Godot 前端拉起栈+`show_plugin_editor` 打开壳编辑器人工目检；自动化端测只覆盖数据面（件五各卡验收面）。

## 5. 与 genctx 的对接清单（实现卡引用）

| 对接点 | 契约出处 | 壳侧义务 |
|---|---|---|
| RenderProfile | 统一化件三 §2 | 壳传 `ConsumerID="gen_plugin"`，IncludeFields/Blocks 按模板声明裁剪（默认全量）；无自定义模板文本入口 |
| scene | 统一化件四 §1 | 每次请求显式携带；free_creation 不落轨（§4） |
| 用户块保真 | 统一化件三 §1.1 | free_text 原文置尾零加料（编排发生在合成前，合成体整体作为用户块） |
| 未设置项可见 | 统一化件三 §1.1 | GUI 上下文面板把 `(not set)` 原样显示（不隐藏未设置项） |
| 机检面 | 统一化件三 §3 | 音频产物 C 形态无机检（如实）；MIDI artifact 不落轨（件二 §3） |
| 快照来源 | 统一化件二 §2.5 | 壳不直连 get_project_state；快照经 agent 组装→宿主→context_snapshot 展示副本 |
