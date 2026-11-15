# vit.legame_design.v0 乐局设计源契约（冻结草案 W01，候审）

- 状态：**冻结草案（候主管过目；§7 六项候用户裁定，未定夺前不得按草案默认值开工实施卡）**
- 卡：G0-LEGAME-CONTRACT-1（池序 370，P0）；执行=GLM-5.3-Flash 免费车道子代理，2026-11-15 深夜
- 输入权威：
  - 设计源实例：`coord/lesson-designs/music-creation-unit2/design.v1.yaml`（revision 3）
  - 现行运行时格式：`agent/internal/edu/lesson_card.go`（`vit.level_card.v0`，KnownFields fail-closed）
  - 裁定口径：`coord/decisions/2026-11-11-legame-demo-experience-first.md` §13/§14/§15
  - 审计依据：`coord/reports/2026-11-12-demo-execution-audit/README.md` H1/H2/H3、`WORK-PACKAGES.yaml` W01
- 配套工件：schema 结构草案 `coord/runs/G0-LEGAME-CONTRACT-1/legame-design-v0.schema-draft.yaml`；覆盖性核验 `design-source-coverage-check.md`；实施卡草案 `impl-cards-draft.md`
- 本文档不改生产代码、不实现解析器；本文档被主管验收后即 W01 实施卡的格式权威。

---

## 1. 范围与非目标

**范围**：冻结 `vit.legame_design.v0`（乐局设计源）schema 分层定义、字段契约、校验门规则、与 `vit.level_card.v0` 的单向编译/适配契约（含缺项诊断与卡缓存失效）、候用户裁定项清单、实施卡拆分草案。

**非目标**：不实现解析器/编译器；不修改 `lesson_card.go` 或任何生产代码；不定义非乐局意图文档的完整格式（仅冻结判层机制与保留值，见 R1）；不做 design←card 反向迁移；YuE2 方言映射、谱面库选型（369 卡）、单栈验收口径（371 卡）不在本文档域内。

## 2. 两个文档、两种角色（关系定性）

| | 乐局设计源 `vit.legame_design.v0` | 乐局卡 `vit.level_card.v0` |
|---|---|---|
| 角色 | **设计源**：课程意图/材料/流程的人类可读权威（§15 工程包的意图文档） | **运行时卡**：引擎可执行的判定/资产/节点契约 |
| 作者/入口 | 外部编辑（M1 入口=导入已定稿 YAML+启动页文本微调，§15 方案 A） | 由编译器产生，或专家手写（存量 first-run-twinkle） |
| 校验 | 语义校验门（本文档 §5；E/W/I 诊断） | 现行 12+6 拒绝路径（lesson_card.go，不改） |
| 权威性 | **源**（手改卡无效，见 §6.5） | **派生物或独立手写产物** |

定性（三条纪律，实现卡与评审均以此为准）：

1. **单向编译**：design→card 是唯一合法降级路径（§6）；不支持反向，不做继承/子类型关系——两者字段集重叠是巧合不是合同。
2. **不自动把自然语言变代码**（审计 H1 明言）：编译器只做封闭映射表翻译；映射表未覆盖的判据一律降级 advisory 并出 W 诊断，禁止"文本理解"式生成判定器。
3. **存量卡不受影响**：`level_card.v0` 继续按现行校验门独立加载（审计 W01 first_slice"保留旧卡加载"）；本契约不改变其任何行为。

## 3. 分层 schema（§15 裁定落实）

### 3.1 两层定义

| 层 | 适用 | 必填 | 可选字段 |
|---|---|---|---|
| **L0 最小意图层** | 一切 Vit 工程包可携带的意图文档（乐局与非乐局） | 仅结构性判层键 `schema_version`（见 3.3 解读） | id, revision, title, brief, workspace.timing, guidance, budget, pending_questions（全部可选） |
| **L1 乐局层** | `schema_version: vit.legame_design.v0` 的文档 | 追加五个硬必填段：**objectives, materials, checks, flow**（非空列表）+ **workspace.regions**（非空列表）；node 级 `complete_when` 非空 | 追加可选段：workspace.tracks, capability_requests, evidence_recording, submission, completion |

必填段口径（§15"objectives/materials/regions/checks/flow 等必填段"的落法）：

- 五个必填段缺失或为空列表 → E 诊断（编译拒绝）。§15 列名的"regions"≙设计源的 `workspace.regions`（regions 嵌在 workspace 下，不提升顶层，与设计源实例一致、避免无谓迁移）。
- L1 可选段缺省语义见 §4 逐字段表（每字段有缺省声明，满足 §11 持久化兼容纪律）。
- evidence_recording / submission / completion 对乐局证据链是强语义，v0 仍列为可选段：缺省时编译模板注入保守缺省（§6.4 模板表），但缺省注入逐项出 I 诊断，评审可辨。

### 3.2 非乐局文档的处理边界（M2+ 后置）

非乐局意图文档：允许携带、发现、按 L0 子集解析（未知字段仍 fail-closed）；其每轮注入**后置 M2+，不进 11/19 关键路径**（§15 原文）。v0 只冻结：判层机制、L0 字段集、保留值 `vit.project_intent.v0`（候选值，R1）。在 R1 裁定前，实现卡只须保证"未知 schema_version fail-closed"，不必实现非乐局路径。

### 3.3 判层机制与"全可选"解读（R2 候裁定）

- `schema_version` 是判层键：`vit.legame_design.v0` → 按 L0+L1 全量校验；未知值 → fail-closed 拒绝（与 level_card 拒绝路径 11 同型）。
- "最小意图层字段全可选"（§15 原文）解读为：**L0 的意图字段全可选；schema_version 是判层机制的结构键，不属于"意图字段"，必须存在**。没有它无法判层、无法 fail-closed。此解读若被否决（例如要求无 schema_version 的裸 YAML 也合法），需给出替代判层机制再冻结——故列为 R2。
- 文档类型判定只看 schema_version，**不看段落存在性**（不做"有 objectives 就当乐局"的猜测——与 CCB 不做自动 view 选择的仓库纪律同款）。

## 4. 字段契约（全字段覆盖）

类型记法：`str` 非空串 / `str?` 可空 / `int` / `float` / `bool` / `list[T]` / `map`。必填性：L0 段内字段一律可选；L1 必填段内字段按表标注（`必填`/`可选`）。"枚举"为 v0 闭集，**新增值须先扩契约再扩实现**（与 write_boundary.confirm_downgrade v0 仅认 none 同款纪律）。

### 4.1 顶层（17 段）

| 字段 | 层/必填 | 类型 | 枚举/格式/引用 | 缺省语义 |
|---|---|---|---|---|
| schema_version | 结构键（必填） | str | 枚举 {vit.legame_design.v0}；保留值 vit.project_intent.v0（R1） | 无（判层键） |
| id | L0 可选 | str | 卡内全局唯一；乐局编译目标须与包目录一致（加载口收口） | 未命名工程 |
| revision | L0 可选 | int | ≥1；内容哈希的一部分（§6.5 缓存键） | 0（未修订跟踪） |
| title | L0 可选 | str | 编译目标须非空（卡拒绝空 title） | 空（E if 编译） |
| brief | L0 可选 | map | 4.2 | 空 |
| objectives | L1 必填 | list[map] | 4.3 | （乐局缺→E） |
| materials | L1 必填 | list[map] | 4.4 | （乐局缺→E） |
| workspace | L0/L1 混合 | map | 4.6 | 空 map（timing 全缺省；乐局缺 regions→E） |
| capability_requests | L1 可选 | list[map] | 4.7 | 空表=无能力声明（编译写边界只含内建只读面） |
| checks | L1 必填 | list[map] | 4.8 | （乐局缺→E） |
| flow | L1 必填 | map | 4.9 | （乐局缺→E） |
| guidance | L0 可选 | map | 4.10 | 全部未声明（编译模板注入 §6.4） |
| evidence_recording | L1 可选 | map | 4.11 | 模板缺省（§6.4，I 诊断） |
| submission | L1 可选 | map | 4.12 | 模板缺省（同上） |
| completion | L1 可选 | map | 4.13 | 模板缺省（同上） |
| budget | L0 可选 | map | 4.14 | null 语义=运行时决定（编译时 E+覆盖参数，R5） |
| pending_questions | L0 可选 | list[str] | 自由文本；编译回执原样罗列（I） | 空表 |

### 4.2 brief（L0，全子字段可选）

| 字段 | 类型 | 说明/校验 |
|---|---|---|
| audience | str | 无枚举（自由文本） |
| music_task | str | 无枚举 |
| final_deliverable | str | 无枚举 |
| prerequisites | list[str] | 自由文本（v0 不做先修结构化） |
| estimated_minutes | int? | **可空**；null=不以课时限制（设计源既定）；仅 I 诊断提示 |

### 4.3 objectives[]（L1 必填段）

| 字段 | 必填 | 类型 | 校验 |
|---|---|---|---|
| id | 必填 | str | 段内唯一（E） |
| kind | 必填 | str | 枚举 {learning, music_task}（E） |
| description | 必填 | str | 非空（E） |
| evidence_checks | 必填 | list[str] | 每项 ⊆ checks[].id（E，规则 REF-OBJ）；非空（乐局目标无证据钩→W） |

### 4.4 materials[]（L1 必填段）

| 字段 | 必填 | 类型 | 校验 |
|---|---|---|---|
| id | 必填 | str | 段内唯一（E） |
| kind | 必填 | str | 枚举 {text, image, symbolic_reference, midi_or_audio, midi, material_group}（E） |
| file | 可选 | str? | null 合法=未备料；非 null 时为**相对设计源 YAML 所在目录的包内相对路径**（POSIX 斜杠；禁止绝对路径/`..` 越出包根→E，规则 MAT-PATH）；校验门不查文件存在性（设计期可在包外编辑），**文件存在性+哈希对账在装配/编译期收口**（E，规则 MAT-FILE，§6.3） |
| purpose | 必填 | str | 非空 |
| source | 必填 | str | 非空（出处声明；未核材料须如实标注"待转写/未听核"——来源声明不实属内容面 lint，W） |
| locator | 可选 | map | pdf_pages list[int]、printed_pages list[int] 可空可缺；music_region：str 或 list[str] 或 null（**单/复值并存为设计源既定形态，schema 接受两态；归一方向 R6**）；material_group 的 music_region 列表项语义=组内条目 ID（按 ID 加载是拟议语义，实现属准备层——pending_questions 既定，不因 schema 收编而视为已实现） |

材料核验状态：**v0 schema 不新增 status 字段**。未听核/未转写状态由 `assets/catalog.yaml`（材料库索引）承载（TEXTBOOK-ALIGNMENT §3 既定结构）；编译期按 catalog 状态决定资产准入（§6.3）。是否把核验状态提升进设计源 schema → R6 附带项。

### 4.5 （无独立段）材料→资产的方向声明

设计源材料 ≠ 运行时资产。只有**file 非空、可播放（kind ∈ {midi, midi_or_audio}）、catalog 核验状态达标**的材料才在编译期成为卡资产（sha256 补齐）；其余一律不进卡并出 W（待备料清单）。设计源里 file:null 的条目（如 listening-xiaocao）是合法的"登记未备料"，不阻塞编译（W），但**不得**被编译成可播放资产——禁止用占位 MIDI 冒充（设计源注释红线）。

### 4.6 workspace

**workspace.timing**（L0 可选；§13/§14 两层计量裁定的落点）：

| 字段 | 类型 | 校验/缺省 |
|---|---|---|
| time_signature | str? | 格式 `N/D`（N,D 正整数；E if 违例）；缺省=内核内建默认（不写工程） |
| key_signature | str? | **v0 词表候裁定 R6**；草案按 token `^[A-G][#b]?$`（大调名）收；消费方=记谱拼写 + YuE2 ABC `K:` 默认上下文；**材料可自带覆盖**（§14 裁定原文）；缺省=C |
| tempo_bpm | float? | 可空（null=未定，编译 E+覆盖参数 R5）；>0 |
| pickup_beats | float? | ≥0（缺省 0） |

两层计量纪律（§14，写死进编译契约）：**工程拍号只由本段声明**（创建工程时应用、回读一致）；Tap 控件内"拍号选择"是 agent 教学演示/节拍器局部状态，不写工程、不进检查计量——schema 不为 Tap 局部拍号提供字段（R-附加：是否需要结构化字段→见 §7 R5 附注，v0 不加）。

**workspace.tracks[]**（L1 可选）：

| 字段 | 必填 | 类型 | 校验 |
|---|---|---|---|
| id | 必填 | str | 段内唯一（E）；regions[].track 引用键 |
| name | 必填 | str | 非空（显示名） |
| role | 必填 | str | 枚举 {reference, user_work, assist_generated, generated_result}（E） |
| initial_material | 可选 | str? | null 合法；非 null 时 ⊆ materials[].id（E，规则 REF-MAT） |
| editable_by_user | 可选 | bool | 缺省 false |

**workspace.regions[]**（L1 必填）：

| 字段 | 必填 | 类型 | 校验 |
|---|---|---|---|
| id | 必填 | str | 段内唯一（E）；checks[].target 引用键 |
| track | 必填 | str | ⊆ tracks[].id（E，规则 REF-REGION） |
| locator | 必填 | map | 联合形态（下表；恰一种 scope 语义，混写→E，规则 REG-LOC） |
| purpose | 必填 | str | 非空 |

locator 联合形态（设计源实例归纳，闭集）：

| 形态 | 字段 | 说明 |
|---|---|---|
| scope 型 | `scope: str`（枚举 {selected_reference_excerpt, four_clips, prototype_and_variant, generated_audio_versions}）+ 可选 `clip_aliases: list[str]` | 抽象范围声明；编译期需绑定桥落到具体 clip/轨（W01-C） |
| clip 型 | `clip_alias: str` + 可选 `time_signature`、`bars: int` | 单 clip 寻址；clip_alias 全局唯一（跨 regions 重复→E，规则 REG-ALIAS；这是 H3"按真实 ID 读取、不串作业"的 schema 侧根基） |
| 多 clip 型 | `clip_aliases: list[str]`（scope=four_clips 时）+ `bars_per_clip: int` | four_clips 专用 |
| 计量覆盖 | `time_signature`（region 级） | 覆盖工程默认计量？——**否**。设计源实例中 region 级 2/4 与工程一致；v0 规则：region 级 time_signature 须等于 workspace.timing.time_signature（不一致→E，规则 REG-METER；材料自带 4/4/3/4 演示属材料层，不经 region 声明——§13"讲解其他拍号由 agent 写示例"） |

### 4.7 capability_requests[]（L1 可选）

| 字段 | 必填 | 类型 | 校验 |
|---|---|---|---|
| id | 必填 | str | 段内唯一；flow.nodes[].uses_capabilities 引用键 |
| request | 必填 | str | 自由文本（意图声明，**不**直接编译为命令——写边界须经绑定闭包表，§6.3 规则 CAP-BRIDGE，禁止文本猜测） |
| required | 必填 | bool | — |
| fallback | 可选 | str? | null 合法=无 fallback（required=true 且 fallback=null 的能力缺失→运行期 pause_and_report 语义，属装配/运行面） |

### 4.8 checks[]（L1 必填段）

| 字段 | 必填 | 类型 | 校验 |
|---|---|---|---|
| id | 必填 | str | 段内唯一 |
| purpose | 必填 | str | 枚举 {task_progress, learning_evidence} |
| method | 必填 | str | 枚举 {user_judgment, rubric, deterministic} |
| target | 必填 | str | ⊆ regions[].id ∪ {"submission_snapshot"}（E，规则 REF-CHK；submission_snapshot 是唯一非 region 合法目标） |
| criterion | 必填 | str | 非空（自然语言判据；**编译器不把它变成代码**，§2 纪律 2） |
| evidence | 必填 | str | 非空 |
| on_unavailable | 必填 | str | 枚举 {request_review, pause_and_report} |

### 4.9 flow（L1 必填段）

flow.progression：枚举 {ordered}（v0 仅 ordered；扩值须扩门）。

**flow.nodes[]**：

| 字段 | 必填 | 类型 | 校验 |
|---|---|---|---|
| id | 必填 | str | 段内唯一（E，同卡拒绝路径 9 同型） |
| title | 必填 | str | 非空 |
| objective_refs | 必填 | list[str] | 每项 ⊆ objectives[].id（E，规则 REF-NODE-OBJ）；空表合法（允许过渡节点） |
| task | 必填 | str | 非空；编译→卡 node.goal |
| lead | 必填 | str | 枚举 {human, model} |
| uses_materials | 可选 | list[str] | ⊆ materials[].id（E，规则 REF-NODE-MAT） |
| uses_regions | 可选 | list[str] | ⊆ regions[].id（E，规则 REF-NODE-REG） |
| uses_capabilities | 可选 | list[str] | ⊆ capability_requests[].id（E，规则 REF-NODE-CAP） |
| human_decides | 可选 | list[str] | 自由文本（每轮注入语境，W02/H2 域；不进卡） |
| model_may_do | 可选 | list[str] | 自由文本；编译→卡 model_does（直拷） |
| freedom | 可选 | map{fixed: list[str], open: list[str]} | 自由文本（不进卡；注入语境 W02） |
| guidance | 可选 | map | introduction str / teaching_points list[str] / hint_ladder list[str] / on_stuck str；hint_ladder→卡 node.hint_ladder（advisory）；teaching_points→卡 teaching（join 规则 §6.2 表 T-14） |
| complete_when | 必填 | list[str] | 非空（E，规则 FLOW-CW）；每项 ⊆ checks[].id（E，规则 REF-NODE-CHK） |
| collect_evidence | 可选 | list[str] | ⊆ checks[].id（E） |
| leaves_behind | 可选 | str | 自由文本（不进卡；I 摘要） |
| transition | 可选 | str | 自由文本过渡语；**不迁移**为卡 then（卡 then=命令名序列，语义不同，§6.2 表 T-16） |

### 4.10 guidance（L0 顶层，全可选）

approach / help_policy / exploration_policy / learner_decision_policy：均 str。语义=教学策略声明（每轮注入语境 + 编译模板软约束来源，§6.2 表 T-18）。无枚举。

### 4.11 evidence_recording（L1 可选）

| 字段 | 类型/枚举 | 缺省 |
|---|---|---|
| requested_granularity | list，枚举 {key_action, region, saved_revision} | [key_action, region, saved_revision]（模板缺省） |
| record_initiator_and_executor | bool | true |
| record_help_disclosed | bool | true |
| record_verified_progression | bool | true |
| pin_submission_log_range | bool | true |
| unknown_source | str，枚举 {mark_unverified}（v0 仅此值） | mark_unverified |
| checkpoints | list，枚举 {node_entry, meaningful_revision, submission} | 三值全取 |

缺省=保守全记录（宁多勿丢证据）；显式关闭任一 bool（=false）→ W 诊断（证据链收缩须评审可见）。

### 4.12 submission（L1 可选）

freeze_project_and_evidence_snapshot / teacher_review_required / review_by_objective / retain_previous_submissions / include_raw_evidence_index / summary_claims_require_evidence_refs：bool，缺省 true（保守）。on_return：枚举 {preserve_history_and_continue}（v0 仅此值）。reports：list，枚举 {assignment_report, key_context_summary}。缺省=设计源实例值（§6.4 模板表）。

### 4.13 completion（L1 可选）

required_deliverables：list[str] 自由文本。required_checks / learning_evidence_to_record：list[str] ⊆ checks[].id（E，规则 REF-COMP）。user_confirm_closure：bool 缺省 true（收局须用户确认——设计源实例值，缺省收紧不放松）。export：枚举 {optional}（v0 仅认 optional，设计源实例值；扩值须扩门）。

### 4.14 budget（L0 可选）

max_turns / max_llm_calls：int?，**可空**（null=不限，运行时自行管理——设计源既定）。on_limit：枚举 {preserve_and_pause}（v0 仅此值；缺省该值）。编译目标卡要求两数值 ≥1 → 空值触发 R5 策略（E+覆盖参数，或固定注入表），§6.2 表 T-20。

## 5. 校验门规则（E/W/I 三级）

### 5.1 诊断分级语义

| 级 | 语义 | 出口行为 |
|---|---|---|
| **E（阻断）** | 文档违反 schema/引用完整性/闭集，或编译不可能产出合法卡 | 校验/编译失败，退出码非 0，诊断聚合列出（逐条含字段路径+规则号+修复提示）；**禁止静默纠正**（与 level_card 不静默纠正同口径） |
| **W（降级/待核）** | 可编译，但发生声明性降级（判据降 advisory、未备料排除、核验状态不足等） | 通过，但每条 W 进入回执与编译 manifest；W 清单=验收必读 |
| **I（提示）** | 信息性（pending_questions 数量、estimated_minutes=null、设计层字段不进卡等） | 回执附录 |

### 5.2 规则号清单（实现卡按此编号实现，评审按此对账）

- **E-SCHEMA**：未知 schema_version（fail-closed）；未知字段（yaml KnownFields，任何层）；枚举外取值（§4 全部闭集）；类型错。
- **E-REQ**：L1 五必填段缺失/空；node.complete_when 空；REG-METER / REG-LOC / REG-ALIAS 违例。
- **E-REF**：REF-OBJ / REF-CHK / REF-NODE-OBJ / REF-NODE-MAT / REF-NODE-REG / REF-NODE-CAP / REF-NODE-CHK / REF-COMP / REF-REGION / REF-MAT 引用不存在的 id；各列表 id 重复。
- **E-MAT**：MAT-PATH（绝对路径/越出包根）；MAT-FILE（编译期文件缺失/哈希不符）。
- **E-CAP**：CAP-BRIDGE（capability_request 无登记的命令闭包表条目且 required=true）。
- **W-CHK**：check/complete_when 项无法映射到卡判定面 → 降 advisory_checks（白名单见 §6.2 表 T-8/T-9）。
- **W-MAT**：file:null 材料；catalog 核验状态不达标的可播放材料（排除出卡资产）；source 声明含"未听核/待转写"字样。
- **W-BUDGET**：budget 空值注入缺省（R5 裁定前按草案默认表）。
- **I-NOTE**：设计层字段不进卡（editable_by_user、key_signature、transition、freedom 等）；estimated_minutes=null；pending_questions 计数。

诊断输出形态：聚合列表（同 ValidateLevelCard 聚合风格），每条 `"[E-REF] checks[5].target 'r-nope' 引用未知 region"` 格式，可机读（规则号前缀）+人读（路径+提示）。

## 6. 迁移/适配契约（design → level_card.v0）

### 6.1 编译器形态

`internal` 新包（实施卡 W01-B；包名候定 `legamedesign/compile` 或 `educompile`，不进 `edu` 包本体以免混淆两格式）。输入=设计源字节+装配上下文（包目录、catalog 状态、覆盖参数）；输出=（LevelCard 字节, compile manifest）。终检=**复用现行 `ValidateLevelCard`**——编译产物必须通过现行校验门，编译器不得绕过/复制/弱化该门（单向依赖：compile→edu，edu 不反向 import）。

### 6.2 编译映射表 v0（表格=权威；实现卡按表执行，偏差须回本文档修订）

| # | 设计源 | 卡 | 规则 |
|---|---|---|---|
| T-1 | schema_version | 固定输出 `vit.level_card.v0` | 重写 |
| T-2 | id / title | id / title | 直拷；title 空→E |
| T-3 | brief（拼接模板） | story | 模板句（music_task+final_deliverable 摘要）；brief 缺→W+模板兜底句 |
| T-4 | flow.nodes[].lead 全一致 | lead_default | 全 human→human；全 model→model；**混写→E**（要求设计侧统一或显式覆盖参数） |
| T-5 | （固定） | players=[human, model] | 本契约为双人乐局格式，固定 |
| T-6 | workspace.timing.time_signature | timing.time_signature | 直拷；缺→E（乐局无计量不可编译） |
| T-7 | （换算） | timing.beats_per_bar | =time_signature 分子（2/4→2，4/4→4）；确定性换算不做推断 |
| T-8 | workspace.timing.tempo_bpm | timing.bpm_default | null→R5 策略（草案=编译覆盖参数必填，无参数→E）；>0 校验 |
| T-9 | workspace.timing.pickup_beats | timing.pickup_beats | 直拷（缺省 0） |
| T-10 | workspace.timing.key_signature | （无对应卡字段） | 不迁移；I 诊断（消费方=记谱拼写/YuE2 ABC K:，走材料与装配层，§14） |
| T-11 | workspace.regions（clip 型 locator） | timing.clip_origins + bar_span | clip_alias→origin 条目（start_beats_global 按 region 在轨内序推导，bars→first_bar）；scope 型 region 不产生 origin；bar_span=全 clip 型 region 覆盖的最外层小节窗；推导歧义→E |
| T-12 | workspace.tracks[] | runtime_slots | 键=track.id；track_index=R3（草案=声明序 1..n）；kind 映射：reference→reference，user_work→workspace，assist_generated→blank_to_fill，generated_result→workspace（**提案值**；卡不校验 kind 闭集，语义对账归 W01-C lint）；initial_material→preload（材料须已准入资产，否则 E-REF）；initial_state：preload=null→empty，else loaded |
| T-13 | materials（§4.5 准入者） | assets | file 复制进包资产区+sha256 补齐；kind 映射 midi→midi_reference；key 由 workspace.timing.key_signature+mode 推导（tonic 音级表）；material_group/未备料/文本图像类**不进卡资产**（W/I） |
| T-14 | checks[].method | 判定面三分支 | deterministic+判据可表达为卡结构谓词白名单（clip_exists/track_non_empty/tempo_set）→done_when 叶；**白名单外一律降 advisory（W-CHK），禁止从 criterion 文本生成判定**；rubric→objectives[].check kind=verbal_rubric（criterion+evidence→rubric 行拆句）；user_judgment→event 叶映射表 {user_judgment→user_confirmed|user_feedback_given|user_chose 按语境登记}——映射表未覆盖→verbal_rubric 降级（W） |
| T-15 | objectives[] | objectives[] | id 直拷；description→text；evidence_checks 与卡单 check 的多对一→**R4**（草案：长度=1 直映射；>1→拆分为 `id#k` 多卡目标（W 诊断列明拆分））；无 evidence_checks→W |
| T-16 | flow.nodes[] | nodes[] | id/title/lead 直拷；task→goal；guidance.hint_ladder→hint_ladder；guidance.teaching_points→teaching.note（join"；"）；model_may_do→model_does；complete_when→done_when（可编译项；全不可编译→E-FLOW：节点无完成条件不可运行）；collect_evidence→teaching.objectives 关联（经 T-15 拆分对应）；transition/human_decides/freedom/leaves_behind/uses_* → 不进卡（I；uses_* 进每轮注入语境，W02/H2 域）；then 缺省空（设计源无命令序列） |
| T-17 | （固定模板） | write_boundary.allow | **仅由 CAP-BRIDGE 闭包表产生**（每条登记的 capability_request→命令名闭包）；无登记→E-CAP；confirm_downgrade 固定 none（卡 v0 纪律） |
| T-18 | guidance（顶层4字段 join） | guidance.soft | 四字段 join（"；"）；avoid=[]；pullback_after_turns→R5 缺省注入（草案=设计源未声明时注入保守值并 W） |
| T-19 | submission/completion/evidence_recording | finale + settle 模板 | completion.user_confirm_closure=true→finale.closure_check.soft_signals.user_closing_language.required=true；required_checks→settle.objectives_check 引用串；freeze snapshot→settle.trajectory_archive=true；reports→settle.memory_writes 模板行；逐字段模板在实现卡 W01-B 固化（本文档只冻结名目与方向，模板文案实现卡出） |
| T-20 | budget | budget | null→R5（草案=覆盖参数必填否则 E）；on_limit=preserve_and_pause→on_stop_loss="preserve_and_pause" 直拷 |
| T-21 | pending_questions | （不进卡） | 编译回执罗列（I） |

### 6.3 装配绑定契约（H3 的 schema 侧冻结）

1. **CAP-BRIDGE 闭包表**：capability_request.id → 引擎命令名闭包（如 edit-and-audition→{score_edit 写族, play, read_back}）。表由实现侧登记（W01-C），设计源 request 文本不参与命令推导。required=true 且无表项→E。此表是安全边界（写边界来源），修订走契约评审不走代码顺手改。
2. **材料相对路径与包语义**：materials[].file 相对设计源所在目录；工程包导入时整体复制进包（assets 目录结构按 TEXTBOOK-ALIGNMENT §3）；包导出/移动/重开按相对路径解析（W01 单元 5），不依赖原下载目录。
3. **clip_alias 是作业身份**：同轨多作业（四条练习/原型变体/主题版本）一律按 clip_alias+region 寻址，禁止"轨首 clip"式推导（H3 原文）；region 级 clip_alias 全局唯一（E-REG-ALIAS）。
4. **binding 键=工程 UUID**：乐局 binding 按 project UUID 目录隔离（修复 `legameBindingPath` 父目录静态推导的串工程面，H3 审计既定口径；落实归 W01-C）。**缓存键禁用 level_id 单键**（H3 原文）。
5. **学生作品快照归 W02**（§14 裁定原文）：clip alias 解析与版本快照是 W02/W05/W10/W19 共用契约——本文档只冻结"alias 是寻址身份"，快照语义不在本契约域内。

### 6.4 缺项诊断与编译缺省注入表（R5 裁定前的草案默认）

| 缺项 | 草案默认 | 级 |
|---|---|---|
| tempo_bpm=null | 无覆盖参数→E；有参数→注入并 W | E/W |
| budget 两数值 null | 无覆盖参数→E；有参数→注入并 W | E/W |
| guidance.pullback_after_turns 缺 | 注入 6（first-run-twinkle 同型保守值）并 W | W |
| brief 缺（story 兜底） | 模板句"（设计源未提供剧情句）"并 W | W |
| evidence_recording/submission/completion 段缺 | §4.11–4.13 缺省表 + I | I |
| key_signature 缺 | C（§14 裁定"本课默认 C"泛化为缺省 C）+ I | I |

### 6.5 卡缓存失效（H3"卡缓存仅以 level_id 为 key"的修复契约）

- **缓存键四元组**：`(design 源字节 SHA-256, 编译器版本号, 装配输入清单哈希, 目标卡 schema 版本)`。装配输入清单=materials 准入文件的 sha256 集合+catalog.yaml 哈希+覆盖参数。任一变化→失效重编译。
- **manifest 外置**：编译产物=`<id>.compiled.level.yaml` + `<id>.compile.manifest.json`（四元组+W 诊断清单+映射表版本）。卡本体不带 provenance 字段（KnownFields 禁扩，不得往卡里塞源哈希）。
- **源为权威**：运行时载入卡前核对 manifest；键不匹配/manifest 缺失/卡与 manifest 哈希不符→拒绝使用缓存并重编译（或 E）。手改 compiled 卡无效（改了源才有效；手改卡被重编译覆盖属预期行为，回执说明）。
- **revision 字段**：进入源哈希，不单独作键；revision 前进但内容不变的边界情形按"内容不变→哈希不变→缓存有效"处理（注释性修订不强制重编译；若需强制，触碰任何字节即可）。
- **失效动作幂等**：重编译失败时保留旧缓存+阻断使用（fail-closed），不得降级运行旧卡。

## 7. 候用户裁定项清单（未裁定前实施卡不得按草案默认开工）

| # | 事项 | 草案立场（仅供参考，非裁定） | 影响面 |
|---|---|---|---|
| R1 | 非乐局意图文档 schema_version 保留值（`vit.project_intent.v0`？）与 L0 字段集是否=乐局字段子集 | 保留该值；L0=子集 | M2+ 非乐局链路；不进关键路径 |
| R2 | 判层机制=仅 schema_version；"字段全可选"不含 schema_version 本身 | 是（工程必需解读） | 解析器主入口 |
| R3 | design.tracks→runtime_slots 的 track_index 指派：声明序 1..n（草案） vs 设计源加显式 `index` 字段 | 序数指派+manifest 记录映射 | 编译器 T-12；内核轨序 |
| R4 | objectives 多 evidence_checks→卡单 check：拆分为多卡目标 `id#k`（草案） vs 选主检查+余项 advisory | 拆分（保全证据钩） | 编译器 T-15；W02 证据注入 |
| R5 | budget/tempo 空值编译策略：覆盖参数必填否则 E（草案，fail-closed） vs 固定缺省注入表 | E+显式覆盖参数（不静默替用户定速度/预算） | 编译器 T-8/T-20；T-18 |
| R6 | key_signature 词表封闭度（自由 token `^[A-G][#b]?$` vs 15 大调封闭枚举 vs 含小调）；附带：locator.music_region 单/复值归一方向（草案=接受两态、编译期归一为 list）；材料核验状态是否提升进设计源 schema（草案=不提升，留 catalog） | 草案立场见左 | §4.6 timing；§4.4 locator；§6.3 装配 |

另有两点**非裁定的解释备案**（主管可否决）：①§15"regions"≙workspace.regions 不提升顶层；②卡 kind 映射表 T-12 的 assist_generated→blank_to_fill 为提案值，W01-C 落实时如有语义冲突回本文档修订。

## 8. 与裁定/审计的逐条对照（自证无遗漏）

| 来源 | 条目 | 落点 |
|---|---|---|
| §13 | 工程 tempo/meter 统一，YAML 声明值创建时应用 | §4.6 timing 必填语义、T-6/T-7 |
| §13 | 工程拍号 2/4；讲解其他拍号 agent 写示例 | REG-METER（region 不覆盖工程拍号）；材料层演示不进 schema |
| §13 | 学生节奏稿=旋律轨 clip；鼓轨=assist_generated | 设计源既定 role 枚举；T-12 kind 映射 |
| §14 | key_signature 补入、默认 C、材料可覆盖、记谱+ABC K: 消费 | §4.6 key_signature；T-10；R6 词表 |
| §14 | 拍号两层：Tap 局部拍号不写工程不进计量 | §4.6 两层纪律（schema 不加字段） |
| §14 | 快照归 W02；alias 解析共用契约 | §6.3 第 5 条（边界声明） |
| §15 | 工程包=文件+意图文档+资产+agent 状态一体 | §2 角色表；§6.3 第 2 条 |
| §15 | 统一 YAML 不设双格式；M1=导入定稿 YAML | §2 设计源入口 |
| §15 | 分层 schema：L0 全可选+L1 五必填段；W01 冻结 | §3 全节 |
| §15 | 非乐局携带/发现/解析允许，注入 M2+ | §3.2 |
| §15 | manifest 哈希完整性（防意外损坏非防篡改） | §6.5 manifest（含"非防篡改"边界：手改源=合法编辑） |
| H1 | 冻结最小源语义+编译/适配契约；不承诺自然语言自动变代码 | §2 纪律 2；T-14 白名单 |
| H3 | 同轨多作业按真实 ID/区域；不串作业 | REG-ALIAS；§6.3 第 3 条 |
| H3 | binding 父目录串工程；缓存 level_id 单键 | §6.3 第 4 条；§6.5 四元组键 |
| W01 单元 | 源 schema 校验/材料引用相对路径/映射/修订绑定缓存失效/导出移动重开 | §5/§6.3.2/§6.2/§6.5/§6.3.2 |

## 9. 验收与生效

- 本文档经主管决策流过目 → §7 六项转用户裁定 → 裁定结论回填本文档（§10 修订记录）→ 冻结生效 → 实施卡（`coord/runs/G0-LEGAME-CONTRACT-1/impl-cards-draft.md` 三张）由主管确认发卡。
- 覆盖性自证：`coord/runs/G0-LEGAME-CONTRACT-1/design-source-coverage-check.md` 对设计源 revision 3 全字段路径逐项核验（方法与结果见该文件）。
- 冻结后的修订规则：schema 变更=契约修订卡（改本文档+schema 草案+覆盖核验三件套同轮），禁止实现侧绕过契约改字段。

## 10. 修订记录

| 日期 | 版本 | 说明 |
|---|---|---|
| 2026-11-15 | v0-draft1 | 首版冻结草案（G0-LEGAME-CONTRACT-1） |
