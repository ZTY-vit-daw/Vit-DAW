# EDU-CAP-RECON-1 勘察报告：乐局能力面（符号分析归宿 / 参照物清单 / 数据流通 / UX-AB 复用线）

- 卡：`coord/cards/todo/2026-11-05-EDU-CAP-RECON-1.md`（池序 256）
- 执行：ZCode 执行流（GLM-5.3）/ 2026-11-05；只读勘察，零生产码改动、零真栈占用、零 LLM 调用
- 领取基线：HEAD `b46a9096406fcde6fcfd29013bccd724ae938fb5`（main 主检出；工作树他卡改动未触碰）
- 结论口径：全部来自代码阅读 + grep 锚点，无运行时取证；锚点均为本日工作树状态

---

## 点① 符号分析归宿验证：结论 = **工具命令为主，返回结构借 EPM 惯例（命令 + 投影字段分层）**

### 两条路径的机制对照

| 维度 | agent 工具目录命令 | EPM peer 投影 |
|---|---|---|
| 注册/生产 | `catalog.go` spec 注册（`agent/internal/tools/catalog.go:965` spec 函数签名：command/tool/category/description/risk/mutates/undo/confirm/refresh/ids...；注册表 `defaultSpecs()` 1040-1225；别名 `AddAlias` 143-158） | 事件伴生构建：import 结果触发 `epm.BuildFromImportRows`（`agent/internal/agentloop/message_loop.go:8605-8618`）→ `result["epm_projection"]` |
| 触发方 | **模型显式调用**（对话内按需） | 系统在状态变化（import 等）后自动构建、随结果披露 |
| 返回组织 | `CommandSpec`（catalog.go:26-44）+ 内核 JSON 回复 + agent action 元数据 | `epm.Projection` 顶层 `EvidenceRefs/Limitations/LLMContext`（`agent/internal/epm/types.go:19-32`）；`LLMContext{SummaryMD, CompactFacts, DoNotIncludeRawPackage, EvidenceRefs, LimitationNotes, SuggestedNextStep}`（types.go:148-155） |
| 置信度惯例 | 无内建（各命令自定义） | `SectionCandidate.Confidence`(string) + `ConfidenceScore`(float64) + `Basis[]` + `RequiresConfirm`（types.go:128-139）双字段惯例 |
| 进入 LLM 的路径 | 工具结果直接进对话轮 | 投影 map 附在 result + CCB bounded disclosure 体系 |
| 落地成本 | 低：一个 spec 行 + handler + policy 分类 + capability_pack 引导 | 高：需进 `docs/OBSERVATION_PROJECTION_MANIFEST.md` peer 体系 + CCB view catalog + 披露纪律 |

### 归宿判定

符号分析（重复/模进/倒影/逆行检测）的使用形态 = **乐局对话内模型主动查询**（"看看这段有什么结构"、课时编译时的证据生产、学生写完后判定），输入确定性（MIDI notes 已有读取命令）。这与工具命令路径的触发模型（模型显式、按需、带证据返回）吻合，与 EPM 投影的触发模型（import 等状态事件伴生、自动披露）不吻合。**与裁定记录 §五9 的实现倾向（工具目录命令）一致，验证成立。**

EPM 的价值在**返回结构惯例**而非归宿：符号分析返回应沿用 `LLMContext` 字段族（SummaryMD/EvidenceRefs/LimitationNotes/SuggestedNextStep）与 Confidence 双字段惯例（confidence 字符串 + 数值分 + basis），使音频域"带置信标注的猜测"（裁定 §五7）与符号域"确定性判定"共用同一表达形态。首版**不做** peer 投影（不进 OBSERVATION_PROJECTION_MANIFEST/CCB），后位需要（如创作模式识别要常驻观察）再升级。

### 注册面草案（不写生产代码）

**命令注册**（catalog.go `defaultSpecs()` 加一行，模式对照 `get_midi_clip_notes` catalog.go:1148）：

```go
spec("analyze_midi_transformations", "midi.analyze_transformations", "midi",
  "Analyze transformation-family structure (repeat/sequence/inversion/retrograde) in a MIDI clip's note sequence. Read-only; returns bar-anchored findings with evidence refs.",
  RiskDirect, false, false, false, false, "clip_id")
```

可选别名：`c.AddAlias("midi.analyze_sequence", "analyze_midi_transformations")`。

**输入 schema**（对照 catalog.go:558-571 usage 函数的表述惯例）：

```
clip_id:string
optional track_id:string
optional range_beats:[start,end]          # clip 内 beat 域分析窗口，省略=整 clip
optional transformations:one_of[repeat,sequence,inversion,retrograde][]   # 省略=全族
optional min_match_notes:number           # 最小匹配音数，默认 3
```

**返回 schema 草案**（借 EPM LLMContext/Confidence 惯例，epm/types.go:148-155/128-139）：

```json
{
  "status": "ok",
  "analysis_id": "<稳定 id，供 trajectory/课时卡回指>",
  "clip_id": "...", "track_id": "...",
  "time_unit": "beats",
  "music_context_echo": {                  // 回显换算依据（来源=get_project_state music_context，见点②）
    "tempo_bpm": 100.0, "time_signature": "4/4",
    "time_signature_numerator": 4, "time_signature_denominator": 4,
    "tempo_setting_count": 1               // >1 时 bar 字段降级为 null 并写 limitations
  },
  "clip_origin": { "start_beats_global": 0.0 },   // clip 全局 beat 起点（秒×bpm/60 换算，恒定 tempo 下）
  "findings": [{
    "kind": "sequence",                    // repeat | sequence | inversion | retrograde
    "label": "模进（调内，上移 2 半音）",
    "segments": [                          // 参与对比的各段
      { "role": "prototype", "start_beats": 0.0, "length_beats": 8.0,
        "bar_start": 1, "bar_end": 2,      // 小节坐标（1-based，含边界），拍号不可靠时为 null
        "note_ids": ["n1","n2",...] }      // evidence 回指到具体 note id
    ],
    "interval_pattern": [2,2,-1,2,2,2,-1], // 原型音程序列（半音）
    "shift_semitones": 2,                  // sequence 专有；repeat=0
    "confidence": "high",                  // high | medium | low
    "confidence_score": 0.95,
    "basis": ["interval_pattern_exact_match", "rhythm_exact_match"],
    "requires_confirmation": false         // 纯只读分析，恒 false；预留对齐 EPM 惯例
  }],
  "llm_context": {                         // EPM LLMContext 字段族（epm/types.go:148-155）
    "summary_md": "第1-2小节与第3-4小节构成调内模进…",
    "compact_facts": [...],
    "evidence_refs": ["clip:<id>/notes:n1-n14"],
    "limitation_notes": ["单声部假设：多轨合并导入的 clip 未分离声部"],
    "suggested_next_step": "可用 marker 标注原型/模进段并循环播放对比"
  },
  "limitations": ["..."]                   // 顶层（对齐 epm.Projection.Limitations）
}
```

**配套注册**（三处，均为一行级）：
1. policy：`agent/internal/policy/policy.go` directCommands 加 `"analyze_midi_transformations": true`（对照 :39 `get_midi_clip_notes`）——只读分析走 direct 级。
2. capability_pack：midi pack 的 `Tools` 加 `"midi.analyze_transformations"`（`agent/internal/tools/capability_pack.go:264-270`），Preconditions 补一条"结构分析/变换族问题先用 midi.analyze_transformations，再决定 marker/播放动作"。
3. 实现位置：计算逻辑放 `agent/internal/edu/`（裁定 §三：教学逻辑 `agent/internal/edu/`），handler 挂法对照既有 agent-local 命令（如 `mix.propose_tick` catalog.go:1089 是 agent-local 先例）。**零内核改动**（数据全部来自既有 get_midi_clip_notes/get_project_state）。

---

## 点② 参照物清单盘点：参照物矩阵 + 小节坐标结论

### 参照物矩阵（参照物 × 可得性 × 精度 × 引导覆盖）

| 参照物 | 模型从哪拿到（可得性） | 精度 | capability_pack 引导覆盖 | 判定 |
|---|---|---|---|---|
| **marker（读）** | `project.markers.list`（catalog.go:1010；内核 `ProjectMarkerService.cpp:255-270` 返回 `{status, markers[], marker_count}`，marker 行含 marker_id/name/kind(position\|range)/start_seconds/end_seconds/color/source/created_by） | 秒（float64） | ✅ 完整：project_marker pack（capability_pack.go:94-128，StateSlices 明写 markers[] 字段） | **可用** |
| **自设 A-B 段** | `project.markers.upsert`（catalog.go:1011；内核 ProjectMarkerService.cpp:273-330：start_seconds 必填，end_seconds>start → kind=range） | 秒 | ✅ 同上（Examples 有 position marker 用例；range 形态在 Domain 行说明） | **可用**（range marker 即 A-B 段载体） |
| **循环区（loop）** | ❌ **agent 面缺口**：内核有 `transport_set_loop`/`transport_clear_loop`（`VitApp/Source/Service/CommandDispatcher.cpp:3224/3230` → `TransportAudioService.cpp:553-591`：参数 start_seconds/end_seconds，设 loop range + looping=true + playhead 定位到 start），但 **catalog.go 与 policy.go 均未注册**（grep 零命中）→ 模型当前不可调用 | 秒 | ❌ 零引导 | **缺口①：注册即得**（内核现成，agent 侧补 spec + policy 分类 + 引导三行级改动） |
| **轨道/clip 名单** | `project.state` → 内核 `get_project_state`（CommandDispatcher.cpp:2676→handleGetProjectState:3708）：tracks[] + clips[]（clip 行 CommandDispatcher.cpp:943-960：id/name/clip_type/**start_seconds/end_seconds/length_seconds**/offset_in_source_seconds，全秒域） | 秒 + 稳定 id/name | ✅ track pack StateSlices（capability_pack.go:188-195）+ midi pack（:238-243，clips[] 字段清单在卷） | **可用** |
| **transport 时间** | `transport.play/stop/seek/return_to_zero`（catalog.go:1063-1067）；seek 参数 `time:number_seconds`（catalog.go:388） | 秒 | ⚠️ 无 transport 专门 pack；midi/mix pack 零散提及 | **可用，引导薄** |
| **tempo/拍号（读）** | ✅ `get_project_state` 返回的 **music_context 块**（CommandDispatcher.cpp:3817 调 `appendMusicContextDisclosure`:2235-2252）：`tempo_bpm`（float）、`time_signature`（"4/4" 字符串）、`time_signature_numerator/denominator`（int）、`tempo_setting_count`、`key_signature`、`style_note`。注释明示"读 set_tempo 同一引擎真值，绝不伪造默认" | BPM float64；拍号 int | ❌ **零引导**：capability_pack 九个 pack 无一处提及 tempo/拍号/music_context（grep 零命中） | **数据在、引导缺**（缺口②） |
| **tempo（写）** | `set_tempo`（catalog.go:1070；内核 CommandDispatcher.cpp:3869-3907：仅 bpm 字段，写 tempoSequence setting 0） | — | ❌ 无引导 | 已亲核（裁定 §二），不重勘 |
| **拍号（写）** | ❌ **无命令**：内核无 set_time_signature（set_tempo 仅 bpm）；拍号依赖 JUCE 默认 4/4（tempoSequence 自动插入默认 time-sig，CommandDispatcher.cpp:2228-2229 注释） | — | — | **缺口③（结构性，非阻塞）**：《小星星》4/4 可行；非常规拍号乐局需内核新增命令，上交决策侧知悉 |
| **小节（bar）坐标** | 无直接命令；**换算可得**：全局 beat = clip 全局起点秒 × bpm/60 + note.start(clip 内 beat)（note 数据见点③）；bar = floor(全局beat / numerator) + 1 | 恒定 tempo（tempo_setting_count==1）下精确（浮点换算，误差可忽略）；**多段 tempo 下非线性、不可靠** | ❌ 零引导 | 见下方结论 |

### 小节坐标结论（重点回答）

**小节可以成为可靠的可问坐标，附三个成立条件：**

1. **首版限定恒定 tempo**：分析/换算前校验 `music_context.tempo_setting_count == 1`；>1 时 bar 坐标记为不可靠（null + limitation 披露），问询坐标退回秒/marker。乐局《小星星》= 单 tempo 场景，条件天然满足。
2. **换算责任放命令层，不要求模型心算**：点① 草案的 `analyze_midi_transformations` 返回直接带 `bar_start/bar_end`（含 music_context_echo 回显换算依据）——模型引用小节 = 引用命令返回的字段，而非自己算。这符合裁定 §五8"坐标系是基础设施"的远期方向（系统预先确定性建好参照系），首版以命令返回形式兑现。
3. **参照物清单进通用引导**（裁定 §五15 同向）：capability_pack 新增（或扩 midi pack）一段"可锚定参照物清单"——marker（position/range）/循环区（注册后）/轨道与 clip id+name/transport 秒/music_context tempo+拍号/分析命令返回的 bar 坐标；并声明用户侧坐标（鼠标/屏幕）须追问转换。**这是纯引导改动，数据面已全部就位。**

### 缺口清单（按修复成本升序）

| # | 缺口 | 修复 | 成本 |
|---|---|---|---|
| 1 | tempo/拍号可读但零引导 | capability_pack 参照物清单段写入 | 引导级（半天内） |
| 2 | 循环区内核有、agent 未注册 | catalog spec + policy 分类 + 引导（transport_set_loop 参数=start_seconds/end_seconds，RiskDirect 不改工程内容——loop 是传输态，建议 direct 级，由决策侧定） | 三行级 + 引导 |
| 3 | 拍号写命令缺失（内核无 set_time_signature） | 需内核新增（非本链阻塞：小星星 4/4；非常规拍号教材曲目时升优先级） | 内核级，上交决策侧 |
| 4 | 多段 tempo 下 bar 换算不可靠 | 首版以 tempo_setting_count 守门 + limitations 披露；内核 beat 换算命令（toBeats）留后位 | 守门零成本 |

---

## 点③ MIDI/符号数据流通路径

### 两路数据形态

**工程内读取：`get_midi_clip_notes`（内核 `MidiService.cpp:1685-1735`）**

- 输入：clip_id（必填）+ optional track_id（给错报错——防串轨校验在卷）。
- 返回：`{status, clip_id, track_id, time_unit:"beats", notes[]}`；每 note（`midiNoteToVar` MidiService.cpp:310-320）：`id`（稳定 note id）/`start`（**clip 内 beat**）/`length`（beats）/`pitch`（0-127）/`velocity`/`channel`。
- 姊妹命令 `get_midi_clip_data`（MidiService.cpp:1737-1775）额外给 clip 级 `start_seconds/length_seconds/offset_in_source_seconds`——**这是 clip 内坐标→全局坐标的桥**。
- **数据面判定：够符号分析。** 音高✓（pitch）、时值✓（length beats，节奏比对天然同域）、定位✓（clip 内 beat；全局小节 = clip 起点换算 + bar 换算，见点②）。变换族检测（音程序列在变换下的相等性）全部在 beat/pitch 域完成，不需要绝对时间。**注意单 clip=单声部假设**：import 合并导入的多轨内容在同一 clip 内不可分离（见下），limitations 需披露。

**文件导入：`import_midi_to_track`（内核 `MidiService.cpp:1784-1935`）**

- 输入：track_id + file_path + optional start_time_beats（全局 beat 定位）+ mode（usage 声明 merge_tracks；实现=**全部源轨合并进单个新 clip**）。
- 约束：**仅 PPQ SMF**（SMPTE timeFormat 拒绝）；tick→beat = timestamp / ticksPerQuarter。
- 副作用：新建 clip（名=文件名去扩展，vit_source_file_kind="midi"），note id 统一 `midi_file_note_N`；**不读 MIDI 文件的 tempo/拍号 meta**（只逐 note 事件）→ 导入后工程 tempo 不变，需另行 `set_tempo` 对齐（拍谱流程内显式做）。
- 返回：`clip_id/new_clip_id/track_id/clip_name/source_file_path/start_time_beats/length_beats/start_seconds/length_seconds/note_count`——**领取绑定齐全**（last_created_clip binding 已在卷，catalog.go:1259）。

### 拍谱最短接线路径图（Audiveris/music21 → 进工程）

```
[用户离线] 教材曲谱图片/PDF
   → Audiveris (OMR) → MusicXML
   → music21 读 MusicXML（校正/人工核对）
   → music21 写出 PPQ Standard MIDI File（.write('midi')；单旋律声部导出）
        ※ music21 写 MIDI 自带 tempo/拍号 meta，但 import 侧不读（上见）——
          tempo 由乐局流程显式 set_tempo；拍号默认 4/4（缺口③知悉）
   → [乐局开局节点] import_midi_to_track {track_id, file_path, start_time_beats:0}
   → get_midi_clip_data + get_midi_clip_notes 回读（note_count 核对 + 数据验证）
   → Vit 回放（transport.play）→ 用户耳纠错（裁定 §五12：录入即问询即教学）
   → analyze_midi_transformations（构建块 1）产出 bar 锚定结构证据
```

- **中间格式落点定案：MusicXML →（music21）→ MIDI →（既有 import_midi_to_track）→ 工程**。不新增 MusicXML 直读命令：music21 转换在用户侧离线完成（裁定 §五12 工具=用户部署、非常驻），工程侧零改动。
- 最短路径总改动 = **零**（全部既有命令）；乐局流程卡只需编排既有命令 + set_tempo。
- OMR vs VLM 小实验（裁定 §五12）与本题正交，不在此勘察范围。

---

## 点④ UX-AB 试听入口复用线：结论 = **机制链可承载，但乐局形态需一次区间渲染挂接；兜底路径更短**

### 既有面盘点（UX-AB-AUDITION-IMPL-1 设计稿结论 + 本日亲核）

UX-AB 设计稿（`coord/reports/2026-10-22-ab-audition-entry-design.md`）§1 已实证 audition 机制链生产就绪，本日亲核关键锚点均成立：

- 内核 audition 命令族：`audition.prepare/select/stop/inspect_candidate/apply_candidate`（CommandDispatcher.cpp:2433/2463/2480-2489，AuditionPreviewService）；渲染期白名单在卷（:2384-2408）。
- agent 事件链：`emitAuditionEvent`（`agent/internal/chat/audition_events.go:157`）、判定 POST 双路由（:546-608，mix-tick 分流 → free-state fallback）、`prepareFreeStateAudition`（:609）。
- webui 消费链零改动候选：audition.* 事件 → session 归约 → TrajectoryAuditionPanel 判定卡（设计稿 §1#3）。
- **渲染粒度**：`start_render` 支持 `range:[startSec,endSec]` 区间渲染（内核 `TransportAudioService.cpp:1185-1215`：file_path + range 数组 + bit_depth + use_master_plugins；startOfflineRender 整工程处理链、按区间截取）。

### 乐局需求 vs 既有形态的差异

UX-AB audition 的候选形态 = **mutation 前后两个整工程渲染 wav**（mix-tick bracket：before render → mutate → after render → mount，设计稿 §1#5）。乐局"重复 vs 模进"对照 = **同一工程内两个时间区间**（第 1-2 小节 vs 第 3-4 小节），无 mutation。差异本质：候选生产方式不同，mount/事件/判定/webui 链完全同构。

### 承载结论：**能承载，两条路径**

**主路径（推荐，进乐局卡）：区间渲染候选 + audition mount 复刻。**
`start_render` 两次（bars→秒换算后各渲一段，range 参数）→ 产出 A/B 两 wav → 复刻 `mix_tick_audition.go` 的 mount 形态自造会话身份（命名域建议 `audition:music_game:<node>`，与 experiment/mix_tick/dynamic 三域隔离，对照设计稿 §3.6 纪律）→ 复用 d1AuditionCandidatesForAssignment + 盲测抽签 + AuditionPrepare → `audition.ready` 事件 → **webui 判定卡零改动出现**。改动面 = agent 侧新挂接文件（乐局内对比问询 bracket，类比 semantic_dynamic_audition 设计）+ 判定第三/四身份通道扩展（audition_events.go 路由邻域，mix-tick 同模式）。成本对照 UX-AB 卡 A（已验收合入的实现形态）——同族工作量，估 1-1.5 天。

**兜底路径（更短，建议首版先行）：注册循环段播放组合。**
`transport_set_loop`（内核已有，点②缺口①）+ `seek` + `play` 三命令编排 = 对话内 A/B 描述（"先听 0:00-0:04 原型，再听 0:04-0:08 模进"）+ 循环段播放，用户耳判后口头/对话回流（user_judgment 类事件）。**零渲染成本、零新机制**，只补点②缺口①的注册（三行级）+ 引导。教学闭环第 4 步"播放对比片段"在兜底路径即可成立；盲测判定卡形态留给主路径（乐局内"哪个是模进"检查题用得上）。

**推荐组合：兜底先行（与缺口①同卡修），主路径随乐局"问询 A/B"节点卡排入**——两者不冲突，主路径的 mount 链复用度极高。

---

## 构建块 1（符号分析引擎）实现面估时意见

原 doc2 §3 估时：侧车 Python 2-3 天 + 模进判定器（Go 或侧车）2-3 天（合计 4-6 天，含 Python 环境与进程面）。裁定 §三已作废"music21 侧车先行"，本报告归宿验证 = Go 命令路线成立，估时修订：

| 工序 | 内容 | 估时 |
|---|---|---|
| 序列核 | 音程序列提取（pitch→interval）+ 变换族统一检测核（repeat/sequence/inversion/retrograde = 音程序列在变换下的相等性，单一匹配核四种映射）+ 节奏比对 | 1-1.5 天 |
| 命令面 | analyze_midi_transformations 命令（点①草案 schema）+ bar 换算（music_context 守门）+ evidence refs（note id 回指）+ 单测（小星星 A/B 段调内模进为黄金用例） | 0.5-1 天 |
| 判定器 | 学生片段 vs 原型的比较（错音定位 = 逐音程序列 diff）——同一序列核复用 | 0.5-1 天 |
| 注册/引导 | spec + policy + capability_pack（midi pack 扩 or 参照物清单段） | 0.5 天 |

**合计 2.5-4 天（Go 单包，`agent/internal/edu/`），零内核、零侧车、零 Python 依赖**；较原路线省约 1-2 天并消除进程管理/离线部署面。验收门槛按 AGENTS §5：真栈烟测腿（chat 驱动 → 命令返回断言）+ 既有测试零回退。

---

## 勘察边界声明

- 只读勘察：零生产码改动、零真栈占用、零 LLM 调用；本报告为工作树唯一新增（除卡面回执节）。
- 全部结论来自代码阅读；`music_context` 披露块、`transport_set_loop`、`start_render range` 等内核面均亲读实现，但**未在真实栈上执行验证**（本卡只读约束）；实现卡须以真栈烟测坐实。
- 上交决策侧的事项（非阻塞，卡面停止条件未触发）：
  1. 拍号写命令缺失（内核无 set_time_signature）——非常规拍号教材曲目时升优先级；
  2. `transport_set_loop` 的 policy 分级建议（direct vs undoable——loop 是传输态不改工程内容，本报告倾向 direct，由决策侧定）；
  3. 参照物清单进通用引导 vs midi pack 专属（裁定 §五15 问询协议通用化待用户裁定，引导落点随之定）。
