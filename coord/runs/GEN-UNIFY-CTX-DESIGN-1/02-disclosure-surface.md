# 设计件二：披露面设计（内核 tempo/拍号披露键 + 调性/风格工程级字段）

- 归属：GEN-UNIFY-CTX-DESIGN-1 / 2026-10-11
- 卡面锚点亲验结论：卡面写"CommandDispatcher.cpp:3279-3437 一带"——**实测成立**，`handleGetProjectState` 函数体=**CommandDispatcher.cpp:3291-3448**（3279-3289 是其前一个函数 krev1 digest 的收尾，属"一带"范围）。披露面=函数内 `response->setProperty(...)` 块（:3389-3447）。与 ARR 勘察"小改"定级相符：本件全部改动落在该函数披露块+两个新命令 handler，**零既有键变更**。

## 1. tempo/拍号披露键（music_context 块）

### 1.1 键形态

`get_project_state` 响应新增**单一结构化对象键** `music_context`（不散铺扁平键——顶层已有 40+ 键，散铺污染+演进困难；单对象给后续字段留一个命名空间）：

```json
"music_context": {
  "tempo_bpm": 120.0,
  "tempo_setting_count": 1,
  "time_signature": "4/4",
  "time_signature_numerator": 4,
  "time_signature_denominator": 4,
  "time_sig_setting_count": 1,
  "key_signature": null,
  "style_note": ""
}
```

### 1.2 读侧与"恒有真值"证明（锚点）

- tempo 读：`edit->tempoSequence.getTempo(0)->getBpm()`——与 `set_tempo` 写入侧同 API 面（CommandDispatcher.cpp:3468-3469 亲验：`getTempo(0)` + `setBpm`）。
- 拍号读：`edit->tempoSequence.getTimeSig(0)` → `getStringTimeSig()`（"4/4" 形态，tracktion_TimeSigSetting.h:33）/ `numerator` / `denominator`（CachedValue，:50）。
- **恒有真值**：`TempoSequence::setState`（tracktion_TempoSequence.cpp:125-160 亲验）在 tempos/timeSigs 空列表时**自动插入缺省**（`insertTempo({},nullptr)` / `insertTimeSig({},nullptr)`，:141-144）→ 只要 edit 已加载，`getTempo(0)`/`getTimeSig(0)` 恒非空。因此 `tempo_bpm`/`time_signature*` 键在 status=ok 的响应中**恒存在且为引擎真值**——不违反"无真值不造默认"披露纪律（先例：block_size 披露 TIM-KERNEL-DISCLOSE-1，CommandDispatcher.cpp:3405-3416——该处无设备即键缺省；本处引擎保证有值，性质不同且合规）。
- `*_setting_count`>1 = tempo/拍号自动化存在（tempo track）。v1 机检按 `getTempo(0)`（曲首值）执行；count>1 时注入块附注记"tempo automation present, using initial tempo"（如实披露不升级处理，对齐"缺失/partial 原样保留"精神）。

### 1.3 挂点与实现形态（供实现卡）

- 插入位置：`handleGetProjectState` 内、`response->setProperty("tracks", ...)`（:3420）之前的工程级键区（:3389-3399 一带，紧邻 project_uuid/scope 等工程级键）。
- 形态：一个 `appendMusicContextDisclosure(juce::DynamicObject& response, te::Edit& edit)` 辅助函数（模块内 static，与 `appendGraphRevisionProperties` 同风格，:3447 先例）——保持 handleGetProjectState 主体可读。
- 键缺省语义：edit 未加载时整个命令已走 "No active project" 错误路径（:3295-3296），music_context 随响应不存在，无需单独缺省分支。

## 2. 调性工程级字段（含 §11 全语义）

### 2.1 存储裁定：vit_* edit state 属性，**不用**原生 PitchSequence

- **存储**：edit state 属性 `vit_music_key_signature`，值为 JSON 字符串。先例（同函数内亲验）：`vit_project_uuid`/`vit_project_parent_uuid`/`vit_agent_history_generation`/`vit_analysis_manifest_json`（CommandDispatcher.cpp:3391-3396 的读取模式——`edit->state.getProperty("vit_...").toString()`）。
- **否决原生 PitchSequence 的实证理由**：`PitchSequence` 在列表空时**自动插入 pitch=60（C 大调缺省）**（tracktion_PitchSequence.cpp:101-102 亲验：`if (getNumPitches() == 0) insertPitch({}, 60)`）——用户显式设置"C 大调"与"从未设置"在原生面**不可区分**，hard-when-set 语义（未设置=不机检）无法成立。vit_* 属性的缺省=属性不存在=未设置，语义干净。
- 原生 `PitchSetting` 的对齐关系保留为**值域对齐**（root_pitch 用 MIDI 音高数 0-127 与 `PitchSetting::getPitch()` 同域；scale 名与 `Scale::ScaleType` 11 枚举名一一对应，tracktion_Musicality.h:77-90）——v2 分析推断若迁入引擎侧（写原生 PitchSequence），值可无损搬运。
- 风格字段 `vit_music_style_note`：纯字符串属性（非 JSON），同壳实现。

### 2.2 值结构

```json
{"root_pitch": 60, "root_name": "C", "scale": "major", "set_by": "user", "set_at": "2026-10-11T21:00:00Z"}
```

- `root_pitch`：0-127 MIDI 音高数（权威）；`root_name`：十二音名（"C"/"C#"…，冗余显示字段，由 root_pitch 派生，两者不一致时以 root_pitch 为准——机检与注入只用 root_pitch+scale）。
- `scale`：11 枚举名之一（§2.1 对齐清单）。**未知 scale 值 fail-closed**（写入侧命令拒绝；读取侧未知值=按"未设置"处理并保留原始串到披露的 `key_signature_raw` 调试键——不静默纠正，符合"原样保留不升级"）。
- `set_by`：v1 恒 `"user"`（v2 分析推断写 `"analysis"`，字段从第一天入库防旧记录歧义）。

### 2.3 命令面（用户手填入口）

两个新命令（handler 注册形态对齐 `set_tempo` 先例：CommandDispatcher.cpp:2514 注册 + :3451 实现的单文件模式）：

| 命令 | 参数 | 行为 | 风险级 |
|---|---|---|---|
| `set_project_key` | `root`（音名"C"/"c#"或 int 0-127 二选一）+ `scale`（11 枚举名）+ 可选 `clear:true` | 校验→写 `vit_music_key_signature` 属性→`runAutoPersist()`（对齐 set_tempo 持久化路径 :3473）→回显规范值 | RiskConfirm（agent catalog 侧；工程语义变更） |
| `set_project_style_note` | `text`（≤500 字符）+ 可选 `clear:true` | 写/清 `vit_music_style_note`→auto-persist→回显 | RiskConfirm |

- 校验 fail-closed 项：root 越界（<0 或 >127 或不可解析音名）、scale 非法名、text 超长——均 `makeErrorReply` 拒绝（不钳制不静默截断，对齐披露纪律）。
- agent 侧 argspec/c catalog 注册（agent/internal/tools/catalog.go）随实现卡走。

### 2.4 §11 持久化兼容全语义（AGENTS §11 逐条落位）

| §11 要求 | 本字段语义 |
|---|---|
| 新增持久化字段定义旧状态缺省语义 | **旧工程（无属性）= 调性未设置**：披露 `key_signature:null`、`style_note:""`；注入块明示"调性未设置"；调性机检不启用。零行为变化（不机检≠机检通过，注入块如实写）。 |
| 补旧记录反序列化/往返测试 | 实现卡验收必含：①旧工程（无属性）load→save→load 往返属性仍缺；②设置后 load→save（含 save_as 新路径）→load 值逐字节保真；③`.vit` 加密容器路径（VitHeadlessService.cpp:1211-1233 save 链亲验：edit state 属性随树整体序列化）双验——`runAutoPersist` 与 `save_as_project` 两条落盘路径各往返一次。 |
| 未知枚举值 fail-closed | 未知 scale：写侧拒绝（§2.3）；读侧=未设置+原始串入调试键（§2.2）。JSON 损坏（解析失败）=未设置+`key_signature_parse_error:true` 调试键（不崩溃不造默认值）。 |
| 不得用新增字段悄悄改变旧记录含义 | 属性缺席=旧工程唯一形态，无迁移无改写；披露新键只增不改旧键。 |

### 2.5 agent 侧快照消费（衔接件一 §4）

快照组装器读 `get_project_state` 响应的 `music_context` + `markers` 两键（一次命令调用双取，零额外内核往返）。agent 侧已有 get_project_state 多路径消费先例（chat/free_state_d1_runtime.go:487、continuation_scheduler.go:797、processor_selection.go:260、semantic_treatment_strategy.go:142——均为 `kernel.SendCommand({"cmd":"get_project_state"})` 形态），组装器同模式。

## 3. 披露面不改清单（边界声明）

- **零改动**：既有 get_project_state 全部键、CCB/view catalog（music_context 是命令响应面不是观察投影；升格观察投影=v2 议题）、capabilitycontext（避让先例：RLM-PROFILE-2 同域零触碰纪律）、tempoSequence 写侧（set_tempo 既有行为不动）。
- **不新建**：拍号 set 命令（v1 无用户改拍号需求——拍号写 API 引擎已备（`insertTimeSig`/`setStringTimeSig`），需求出现时另开小卡，本设计不预铺）。
