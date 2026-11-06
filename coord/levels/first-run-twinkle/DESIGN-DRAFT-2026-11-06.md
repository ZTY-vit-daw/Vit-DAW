# 《小星星》乐局（first-run-twinkle）乐局卡 v0 格式与内容——设计先行稿

- 卡：[2026-11-05-EDU-LEVEL-CONTENT-1.md](../../cards/doing/2026-11-05-EDU-LEVEL-CONTENT-1.md)（池序 279，乐局 2）
- 执行：GLM 主会话 L2-worker 子代理（ZCode / PC）/ 2026-11-06；**先行稿轮**（draft）——纯设计文档，零 agent 代码、零素材二进制、不碰 capability_pack
- 领取基线：origin/main `9eebccdf`；worktree `D:/Vit_DAW_wt_279impl` @ `port/edu-level-content-1`
- 依据链：[rulings 2026-11-05-EDU-DEMO-SCOPE-RULINGS-1](../../rulings/2026-11-05-EDU-DEMO-SCOPE-RULINGS-1.md)（§一-4 专家可写 / §五-8/10/12/16/17/18/19 / §六 / §七-20/21/22）+ [257 报告](../../reports/2026-11-05-edu-runtime-recon-1.md)（构建块 2 修订三条 + 点 3 接口注意点）+ 276 edu 包实况（`agent/internal/edu/`：edu.go / transform.go / judge.go / command.go，段 1+2 已验收 pass）+ doc 1 §3.3/3.4（关卡格式与版权口径谱系）+ doc 2 §4（课时卡草案谱系）
- **定稿前置（未满足前本稿不是定稿）**：①裁定 18 终局判定修订版经用户确认；②276（EDU-SYMBOL-CORE-1）段 3 终验后检测核返回形态冻结；③素材版权口径经用户/懂版权者核准。详见 §6 开放问题清单。

---

## 0. 文档定位与阅读顺序

本稿是 279 的先行稿交付物，自包含四件事：

1. **乐局卡 v0 schema 草案**（§1）——与 276 检测核可判定字段逐项对齐，与 280（EDU-LEVEL-RUNTIME-A，lesson_card 校验门）需求前置对齐；
2. **一局实例素材规格**（§2）——《小星星》12 小节主题的原型片段、改错版、工程模板轨布局、学生任务描述、判定参数草案（完整 level.yaml 草案实例在 §2.5）；
3. **内容工件目录结构提案**（§3）；
4. **裁定 18 闭合判据三锚点的实例映射草案**（§4）+ 开放问题清单（§6）+ 定稿轮差额工作清单（§7）。

命名谱系说明：doc 1 用 `vit.level.v0`、doc 2 用 `vit.lesson.v0`；本稿统一为 **`vit.level_card.v0`**（乐局卡=乐局游戏的内容包实例，rulings §五-16 词层：乐局>关卡>关卡运行时）。

---

## 1. 乐局卡 v0 schema 草案

### 1.1 顶层结构总览

```yaml
schema_version: vit.level_card.v0   # 必填；校验门对未知版本 fail-closed（restore 同型）
id: first-run-twinkle               # 必填；目录名一致
title: …                            # 必填
story: …                            # 选填；叙事引入（人格渲染素材）
lead_default: model                 # 必填；会话级主导极性默认（rulings §五-17；可逐节点覆写；advisory）
players: [human, model]             # 必填
timing: …                           # 必填；§1.4（257 修订②）
persona_slot: …                     # 必填；§1.6（rulings §五-19）
assets: …                           # 必填；素材清单（含哈希位）
notes: …                            # 必填（当任何判定引用 asset 参照时）；内嵌判定参照音符表
objectives: …                       # 选填；教学目标/检查挂接（rulings §五-10"乐局卡=关卡节点+教学目标/检查挂接"）
nodes: …                            # 必填；有序节点数组
guidance: …                         # 必填；拉回配置（§1.8；281 消费）
write_boundary: …                   # 必填；§1.5（257 修订③）
finale: …                           # 必填；§1.7（rulings §五-18）
budget: …                           # 演示局必填；§8 止损
```

### 1.2 done_when 条件文法（判定器枚举=首版 3 个）

每节点 `done_when` 是一棵小布尔表达式，叶子三族 + 组合子，全部闭枚举（校验门逐项拒绝未知名，见 §5）：

```
done_when ::= <leaf> | { all: [<leaf>…] } | { any: [<leaf>…] }

<leaf> ::= 判定器条件   { judge: <name>, target: <range>, reference: <range>, …判定器参数 }
         | 交互事件条件 { event: <name>, key?: <str>, min_count?: <int> }
         | 结构谓词条件 { structure: <name>, clip?/track?/key?: <str> }
```

**判定器枚举（首版 3 个，闭集；rulings §五-9 实现倾向 + 280 卡目标 2）**：

| judge 名 | 语义基线（对齐 276 已就位的核心） | 关键参数 |
|---|---|---|
| `note_sequence_match` | 学生片段 vs 原型片段的**音程序列**逐位比对（276 `JudgeAgainstPrototype` 语义）；返回 match / transposed_match / pitch_mismatch / length_mismatch 四态 + 逐音错位（note_id + actual/expected/prototype pitch） | `accept_transposed: bool`（默认 false——276 judge.go 语义说明原文："对「模进练习」是正确变体，对「原调重复练习」是否算对由乐局卡裁决"，本字段即乐局卡裁决落点）；`target`/`reference` |
| `pitch_set_match` | 两段音高**多重集**相等；276 核未覆盖（集合比较非序列比较），语义由 280 定案 | 提案：`set_mode: pitch_class`（默认，容八度错位）\| `absolute`；**280 对齐项**（§6-5） |
| `rhythm_match` | 两段节奏逐位相等 = IOI（inter-onset）+ 逐音长度（276 `rhythmEqual` / `Judgment.rhythm_matched` 同语义）；音数不等即 fail | `target`/`reference` |

**交互事件枚举（运行时提供，闭集；进 280 校验门为新增对齐项）**：
`human_replied` / `user_confirmed(key)` / `user_chose(key)` / `user_feedback_given` / `user_identified(key)` / `export_completed`

**结构谓词枚举（闭集）**：`clip_exists(clip)` / `track_non_empty(track)` / `tempo_set`

**区间寻址**（target/reference 通用）：

```
target:   { clip: <clip_id 或运行时轨槽名>, range_beats: [start, end) }
reference:{ asset: <asset_id> | clip: <clip_id>, range_beats | range_bars }
```

- 音符坐标统一 **beats**（与内核 `get_midi_clip_notes` / 276 `Note` 一致，257 修订②）；
- `range_beats` 半开区间 `[start, end)`，端点容差 1e-6（与 276 `windowNotes` 同口径）；逆序/零宽 → 校验门拒绝（对齐 276 `parseAnalyzeArgs` fail-closed）；
- `range_bars` 为专家手写友好形态（1-based 小节号对），校验门用 `timing`（§1.4）换算成 beats 并与 `range_beats`（若同给）核对一致性——**小节换算在卡内显式依据上做，判定器不做小节推断**（257 修订②原文）。

### 1.3 与 276 检测核可判定字段的对齐表（教学挂接 evidence_hook 用）

`nodes[].teaching.evidence_hook` 挂接 `midi.analyze_transformations`（276 段 2 命令面，RiskDirect 只读）。卡内可写参数**严格取 276 `parseAnalyzeArgs` 已实现的字段**：

| 卡内字段 | 276 检测核对应 | 对齐要点 |
|---|---|---|
| `clip_id`（运行时绑定为轨槽） | `clip_id` 必填 | 卡内用轨槽名（如 `theme_track`），运行时替换实际 clip id |
| `transformations: [repeat, sequence, inversion, retrograde]` 子集 | `Transformations []Kind` 白名单 | 白名单外值 → 拒绝（276 同款 fail-closed） |
| `key: { tonic: 0-11, mode: major\|省略 }` | `Key *Key` | **sequence 双语义对齐**：`DetectOptions` 没有 quality 请求参数——**提供 key 即启用调内模进（diatonic 档）**，省略 key 只检测半音域关系（chromatic）。`sequence_quality` 是**返回断言字段**（expected_findings 里用），不是请求参数 |
| `min_match_notes`（≥2；省略/0=默认 3） | `MinMatchNotes` | <2 → 拒绝（276 同款） |
| `range_beats` | `range_beats` | 同上容差与半开语义 |

**expected_findings 断言字段**（教学锚点，字段名对齐 276 `Finding`/`AnalyzeFinding`）：`kind`、`sequence_quality`、`proto_bars`/`match_bars`（由 `segments[].bar_start/bar_end` 投影）、`shift_semitones`、`diatonic_steps`、`note_count`。一个节点可挂多条 hook（`evidence_hooks[]`），每条可带 `expected_findings`（须存在）/`expect_absent`（须不存在，差分诊断）/`expect_present`——差分断言供错音诊断类教学锚（§2.5 n3）。断言校验属教学面（agent 复核叙事），不进 280 结构校验门。

**bar 可用性前置**：276 命令面 bar 守门要求 `tempo_setting_count==1` 且拍号可用。乐局流程 n2 显式 `set_tempo` 一次（import 不读 tempo meta，rulings §六拍谱接线定案），故 **evidence_hook 全部挂在 n2 之后的节点**（本实例=n3），流程顺序天然满足守门条件。

### 1.4 timing 块（bpm 与小节起拍显式进卡——257 修订②）

```yaml
timing:
  bpm_default: 100            # n2 确认前的建议默认；n2 确认后 set_tempo 写入工程（唯一一次 tempo 写入）
  time_signature: "4/4"
  beats_per_bar: 4
  pickup_beats: 0             # 弱起拍数（本曲无）
  clip_origins:               # 各 clip 资产的全局起点：bar(n) ↔ 全局 beat [(n-1)*4, n*4)
    theme:           { start_beats_global: 0.0, first_bar: 1 }
    theme_corrupted: { start_beats_global: 0.0, first_bar: 1 }   # 平行对照轨，同起点
  bar_span: { first_bar: 1, last_bar: 12 }
```

校验门据此做"小节存在"检查：一切 bar 引用（evidence_hook/expected_findings/range_bars）必须落在 `bar_span` 且被对应资产音符表覆盖。

### 1.5 write_boundary 块（257 修订③：写入边界声明进卡）

```yaml
write_boundary:
  allow:                      # 本关允许的写入命令族白名单（命令名以 catalog 注册面为准，定稿轮逐名核对）
    - import_midi_to_track
    - create_midi_clip        # create/insert clip 族（rulings §五-17 已核实音符级齐全）
    - add_midi_notes          # 含 bulk
    - mutate_midi_note
    - delete_midi_note
    - apply_midi_note_patch   # operations[]: transpose / quantize / velocity
    - set_tempo
    - project.markers.upsert  # 含 range marker（问询可锚定参照物，rulings §五-8）
  confirm_downgrade: none     # RiskConfirm 是否乐局内降级=用户裁定待下（rulings §六挂账）；先行稿默认不降级
```

### 1.6 persona_slot 块（rulings §五-19：12 月=最小人格槽，提示词块级）

```yaml
persona_slot:
  id: default_producer
  form: prompt_block          # 12 月形态=提示词块；人格包系统=产品线
  source: persona_default.md  # 本内容包内文件（定稿轮从本稿 §2.7 骨架提取成文件）
```

轨道对人格免疫（人格不可替用户确认节点、不可自宣完成、不可染指判定权分布）为**运行时铁律**（rulings §五-19 ③），不写进卡数据。

### 1.7 finale 块（裁定 18 终局判定落卡；先行稿按现文设计）

```yaml
finale:
  completion_before_export: true     # 完成时刻在导出之前；导出=确认后的仪式动作，非终审证据
  closure_check:                     # 前置锚点=闭合判定（agent 可评估、达阈值主动触发问询）
    structure_complete:              # 锚点①
      all_nodes_done: true           # 全节点 done_when（设计局口径）
      no_placeholder_blanks: [accompaniment_blank]   # 待填轨非空（豁免口径=开放问题 §6-11）
      whole_song_playable: { from_beats: 0.0 }       # 整曲可从头播到尾（至最后 clip 末端）
    work_surface_quiet:              # 锚点②
      no_pending_proposals: true
      no_unanswered_queries: true
      quiet_turns_min: 2             # 近 2 轮无实质工程修改（只听不改）
    soft_signals:                    # 锚点③
      user_closing_language: { role: weighted, required: false }   # 加权项非必要条件
  ask:
    semantic_template: closure_inquiry   # 闭合达阈值→agent 携证据、用可锚定坐标问询"要收吗"；运行时发语义模板，人格渲染（§五-19 ②）
  user_direct_export: skip_ask       # 用户主动要导出=用户侧闭合信号，免问询直走仪式
  ritual: [name_together, export, settle]   # 命名+导出（终局仪式）
  settle:                            # 沉淀
    memory_writes: [user.display_name, model.display_name, user.taste.variation_choice,
                    user.speaking_style_feedback, user.music_experience]
    objectives_check: all            # 学习目标检查（obj1-obj3 状态入轨迹）
    trajectory_archive: true
  edges:
    completion_neq_stop: true        # 挂起是合法出口但非完成
    completion_neq_freeze: true      # 判定对象是局非工程；局后继续修改正常，不回滚完成状态
```

budget 块（演示局 §8 止损，数值定稿轮定）：

```yaml
budget:
  max_turns: 60
  max_llm_calls: 120
  on_stop_loss: 保留现场上交   # 止损只停止投入；不把未发生的成功改写为成功，不把环境错误归因模型
```

### 1.8 guidance 块（拉回配置）

```yaml
guidance:
  pullback_after_turns: 4      # 多轮未推进当前节点→运行时重新注入当前节点目标（doc 1 §3.2；281 卡目标 2 消费）
  soft: 鼓励模型分享推理、询问感受；用户打字多是好事   # 软引导=人格层（advisory）
  avoid: [重复询问画像已有信息, 让用户报模型能读到的数据, 同一问题换说法反复问]
```

### 1.9 校验门需求清单（给 280 的对齐面——拒绝路径枚举）

280 校验门（lesson_card.go）按本稿需覆盖的拒绝路径：

1. 未知 `judge` 名 → 拒绝（280 卡已定"专家写不出不存在的检查方式"）；
2. 未知 `event` 名 → 拒绝（**新增对齐项**，枚举见 §1.2）；
3. 未知 `structure` 谓词 → 拒绝（新增对齐项）；
4. bar 引用越界（超出 `bar_span` 或不被资产音符表覆盖）→ 拒绝（"小节存在"）；
5. `range_beats` 逆序/零宽 → 拒绝（对齐 276）；`range_bars` 与 `range_beats` 同给但不一致 → 拒绝；
6. `min_match_notes < 2` → 拒绝（对齐 276）；省略/0 = 默认 3；
7. `key.tonic` 越界 0-11 / `mode` 非 major → 拒绝（对齐 276，首版仅大调）；
8. `transformations` 白名单外 → 拒绝（对齐 276）；
9. `node.id` 重复 → 拒绝（jam_state 节点数组寻址前提）；
10. assets 引用不存在的 id / 缺 sha256 → 拒绝；
11. `schema_version` 未知 → 拒绝（restore fail-closed 同型，mix_workflow_queue_view.go:670 模式）；
12. `objectives[].check` 引用未知 judge/event → 拒绝（`verbal_rubric` 为显式豁免标记：教学侧非判定器，不进门）。

---

## 2. 一局实例素材规格（first-run-twinkle，小星星级起步难度）

### 2.1 主题原型片段（公版旋律，Vit 原创呈现）

《小星星》12 小节单旋律，C 大调，4/4，全四分音符进行、每两小节乐句末音拖两拍（*Ah! vous dirai-je, maman*，18 世纪法国民歌，公版旋律；doc 1 §3.4 版权口径）：

```
| C C G G | A A G– | F F E E | D D C– | G G F F | E E D– | G G F F | E E D– | C C G G | A A G– | F F E E | D D C– |
  bar 1-2 (A1)      bar 3-4 (A2)      bar 5-6 (B1)      bar 7-8 (B2)      bar 9-10 (A'1)    bar 11-12 (A'2)
```

MIDI 音高（C4=60）：C=60 D=62 E=64 F=65 G=67 A=69。42 音符全表（pitch / start_beats / length_beats，beats 域，判定参照权威=§2.5 卡内 `notes.theme` 内嵌表）：

| 段 | 音符（pitch@start/len） |
|---|---|
| A1（bars 1-2） | 60@0/1, 60@1/1, 67@2/1, 67@3/1, 69@4/1, 69@5/1, 67@6/2 |
| A2（bars 3-4） | 65@8/1, 65@9/1, 64@10/1, 64@11/1, 62@12/1, 62@13/1, 60@14/2 |
| B1（bars 5-6） | 67@16/1, 67@17/1, 65@18/1, 65@19/1, 64@20/1, 64@21/1, 62@22/2 |
| B2（bars 7-8） | 67@24/1, 67@25/1, 65@26/1, 65@27/1, 64@28/1, 64@29/1, 62@30/2 |
| A'1（bars 9-10） | 60@32/1, 60@33/1, 67@34/1, 67@35/1, 69@36/1, 69@37/1, 67@38/2 |
| A'2（bars 11-12） | 65@40/1, 65@41/1, 64@42/1, 64@43/1, 62@44/1, 62@45/1, 60@46/2 |

**结构可教学性（276 检测核机械验证口径——见附录 A，min_match_notes=7 + key=C 大调、全曲窗口）**，全曲恰好 6 条乐句级 findings：

| 关系 | 类型 | 断言字段 |
|---|---|---|
| B1→B2（bars 5-6→7-8） | repeat | shift_semitones=0，7 音/段 |
| A→A'（bars 1-4→9-12） | repeat | shift_semitones=0，**14 音/段**（A1→A'1 与 A2→A'2 两处 7 音子重复被极大性过滤合并为一条整段报告——核心语义：同 kind 更长段包含子段时子段不报） |
| A2→B1（bars 3-4→5-6） | sequence(diatonic) | diatonic_steps=+1、shift_semitones=+2（半音差逐位不等，chromatic 档检不出——**必须带 key**，已验证 no-key 时此对关系漏检） |
| A2→B2（bars 3-4→7-8） | sequence(diatonic) | diatonic_steps=+1 |
| B1→A'2（bars 5-6→11-12） | sequence(diatonic) | diatonic_steps=−1、shift_semitones=−2（B 段下行一级回到 A'2——主题的音级链条 F 系→G 系→F 系，天然对称） |
| B2→A'2（bars 7-8→11-12） | sequence(diatonic) | diatonic_steps=−1 |

**min_match_notes 校准注记（机械实测，附录 A）**：全曲窗口 findings 数随阈值变化 min=3→30 条、min=4→20 条、min=5→8 条、min=6→6 条、min=7→6 条。低阈值噪声根源是**成对音细胞结构**：主题由 X X Y Y 细胞（C C / G G / A A / F F / E E / D D）拼接，任意两条同音程轮廓的细胞链都构成 chromatic sequence（如 "E E D D→G G F F" shift+3、"A A G– F F→E E D– C C" shift−5）。**教学挂接不靠阈值硬滤，靠窗口+阈值双管**：概念讲解窗口取 **bars 3-8（range_beats [8,32)）+ min=7**，实测 min∈[5,7] 均稳定输出恰 3 条（repeat B1→B2 + diatonic A2→B1/B2）；错音诊断挂接取全曲 + min=7（§2.2）。此校准以 276 核心源码离线复算为准；定稿轮仍须真栈复跑（内核→命令面多一层解析，§6-9）。

### 2.2 改错版规格（故意错一个音）

- **默认损坏点（starter 难度）**：A'1 第 6 音 `@37`（bar 10 beat 2）A4(69) → A♭4(68)。半音外音紧邻还原 A，听感显著（小星星级起步可听性）；只影响 A↔A' 的 repeat 关系，B 段教学材料（B1↔B2 repeat、A2→B1/B2 diatonic sequence）完好（已验证：改错轨 B 窗口 [8,32) findings 与完好轨逐条一致，附录 A）。
- **判定行为预期**（276 核机械验证，供 280/281 测试校准）：
  - `note_sequence_match(target=改错轨 [0,48), reference=theme [0,48))`：pitch_status=`pitch_mismatch`、rhythm_matched=true，**恰一处** mismatch（全曲口径 attempt_index=33、windowed [32,48) 口径 attempt_index=5；actual=68、expected=69、prototype=69；其后 G 为恢复点不误报——judge.go 恢复点逻辑）；
  - 改错轨全曲 `analyze(transformations=[repeat,sequence], key=C, min=7)` 与完好轨的**差分**（附录 A）：完好轨 6 条中 `repeat bars 1-4→9-12（14 音）` **消失**，代之以 `repeat bars 2-4→10-12（8 音）`——即 14 音整段 repeat 被错音**劈成前后两截**，min=7 时只剩越过错音区的 8 音后截（"G– F F E E D D C–" 后半句整体前移重复；min≤5 时另见 5 音前截 C C G G A→C C G G A）。差分信号=「整段 A↔A' 重复缺失」——错音存在的分析面证据；
  - 错音精确定位以 note_sequence_match 的 mismatch（note_id+expected_pitch）为准，analyze 差分作教学叙事（"这段和开头对不上了"）。
- **难度变体（定稿轮可选）**：hard = B2 第 2 音 `@25` G(67)→A(69)（同时破坏 repeat 与 diatonic sequence 两条关系，且为调内音、更难听出）；diatonic 档变体 = 默认点改 69→71（B，调内、更隐晦）。

### 2.3 工程模板轨布局

| 轨 | 名 | 来源 | 角色 | 初始态 |
|---|---|---|---|---|
| 1 | 旋律（你的谱） | n0 导入（OMR 或 fallback 直接导 theme 资产） | workspace（n1 纠错对象） | 空轨（n0 建 clip） |
| 2 | 旋律（改错版） | 模板预置 theme_corrupted | n3 找错音对象 | **muted**（防剧透，n3 解除） |
| 3 | 伴奏 | 模板预置空白轨 | `blank_to_fill`（n5 共创填充；finale 锚点① no_placeholder_blanks 检查对象） | 空轨 |

tempo 不随 import 进入（import 不读 tempo meta）：工程初始无 tempo，n2 显式 set_tempo 一次 → `tempo_setting_count==1`，后续 analyze bar 守门天然满足。

### 2.4 学生任务描述（每节点一句人话）

| 节点 | 对学生说的（人格渲染前语义） |
|---|---|
| n0 拍谱开局 | "把你手里这份谱给我（拍照或文件），我把它变成工程里的 MIDI 弹给你听。" |
| n1 读谱纠错 | "听听看，有哪个音和我识别得不一样？在卷帘里改对它，改完告诉我。" |
| n2 定速度 | "我建议 100 BPM 左右——你觉得呢？定了我们就锁定速度。" |
| n3 找错音 | "第二轨里有 deliberate 的一个错音。先听全曲找找在哪一段，我再陪你对比着听，找到改对它。顺便听听 B 段：两句一模一样的是重复，跟开头那句像又不完全一样的是模进。" |
| n4 选变奏方向 | "轮到我们改它了：换小调、改节奏、加装饰音、换风格——我先各写一小段给你听，你选。" |
| n5 共同完成 | "我把你选的方向做成完整变奏、配上伴奏，哪里不对劲尽管提。" |
| n6 命名+导出 | "收工之前——给它起个名字吧，然后我们导出属于你们的这一版。" |

### 2.5 完整 level.yaml 草案实例（先行稿版；定稿轮以此为蓝本产出真文件）

```yaml
schema_version: vit.level_card.v0
id: first-run-twinkle
title: 一个旋律的变奏（小星星乐局）
story: >
  莫扎特用《小星星》的旋律写了 12 段变奏（K.265）。这一次，轮到你和 {model_name}。
lead_default: model
players: [human, model]

timing:
  bpm_default: 100
  time_signature: "4/4"
  beats_per_bar: 4
  pickup_beats: 0
  clip_origins:
    theme:           { start_beats_global: 0.0, first_bar: 1 }
    theme_corrupted: { start_beats_global: 0.0, first_bar: 1 }
  bar_span: { first_bar: 1, last_bar: 12 }

persona_slot:
  id: default_producer
  form: prompt_block
  source: persona_default.md

assets:
  - id: theme
    kind: midi_reference            # 判定参照 + 可导入
    file: assets/theme_twinkle_v0.mid
    sha256: <定稿轮计算入档>
    key: { tonic: 0, mode: major }
    notes_ref: notes.theme
  - id: theme_corrupted
    kind: midi_reference
    file: assets/theme_twinkle_corrupted_v0.mid
    sha256: <定稿轮计算入档>
    derived_from: theme
    deviations:
      - { at_beat: 37, pitch: 68, replaces: 69 }    # 默认损坏点（§2.2）
  - id: score_sheet
    kind: score_image               # 拍谱腿输入（286 OMR）；谱面=Vit 原创排版（§6-7 版权口径）
    file: assets/score_twinkle_v0.pdf
    sha256: <定稿轮计算入档>
  - id: accompaniment_blank
    kind: blank_track               # 模板空白伴奏轨（finale 锚点①检查对象）

notes:                             # 内嵌判定参照（beats 域；id 由加载器合成 asset:theme#k）
  theme:                           # 42 音符，全表见 §2.1；此处列 A1+A'1 示意，定稿轮生成全表
    - { pitch: 60, start: 0.0,  length: 1.0 }
    - { pitch: 60, start: 1.0,  length: 1.0 }
    - { pitch: 67, start: 2.0,  length: 1.0 }
    - { pitch: 67, start: 3.0,  length: 1.0 }
    - { pitch: 69, start: 4.0,  length: 1.0 }
    - { pitch: 69, start: 5.0,  length: 1.0 }
    - { pitch: 67, start: 6.0,  length: 2.0 }
    # … A2/B1/B2 段同构（§2.1 全表），A'1 段：
    - { pitch: 60, start: 32.0, length: 1.0 }
    - { pitch: 60, start: 33.0, length: 1.0 }
    - { pitch: 67, start: 34.0, length: 1.0 }
    - { pitch: 67, start: 35.0, length: 1.0 }
    - { pitch: 69, start: 36.0, length: 1.0 }
    - { pitch: 69, start: 37.0, length: 1.0 }
    - { pitch: 67, start: 38.0, length: 2.0 }
    # … A'2 段（§2.1 全表）

objectives:
  - id: obj1
    text: 能听辨旋律中的模进与原样重复
    check: { event: user_identified, key: transformation_kind }   # 正误仲裁=模型对 analyze findings（符号证据在人耳答案之后）
  - id: obj2
    text: 能说出模进是「动机在不同音高上重复」
    check: { kind: verbal_rubric, rubric: [提到重复, 提到音高位置改变, 区分于原样重复] }   # 非判定器：教学侧，不进校验门
  - id: obj3
    text: 能把听出的错音改对（使重复关系恢复）
    check: { judge: note_sequence_match, node: n3_find_wrong_note }

nodes:
  - id: n0_photo_score_open
    title: 拍谱开局
    lead: model
    goal: 把你手里的谱变成工程里的 MIDI：给谱 → Vit 识别导入 → 回放给你听
    model_does: [score_omr_import, play]
    needs_human: true
    fallback: 直接导入 theme 资产（OMR 不可用/识别失败；286 未部署时默认走此路）
    done_when:
      all:
        - { structure: clip_exists, clip: melody_imported }
        - { event: human_replied }
    teaching: { note: 录入即问询即教学（rulings §五-12） }

  - id: n1_read_and_fix
    title: 读谱纠错
    lead: human
    goal: 回放里听着不对的地方（识别错的音），在卷帘里改对它
    model_does: [play_loop, analyze_midi_transformations 定位候选小节, 指出期望音]
    needs_human: true
    done_when:
      judge: note_sequence_match
      target:    { clip: melody_imported, range_beats: [0.0, 48.0] }
      reference: { asset: theme, range_beats: [0.0, 48.0] }
      accept_transposed: false
    hint_ladder: [先让听全曲, 再循环可疑小节, 再并排演奏两版同小节, 最后指出小节范围请学生指音]   # 提示阶梯（doc 2 §3 组件5；advisory，人格渲染）
    teaching: { objectives: [obj3] }
    note: 判定参照绑定 canonical 主题资产（本局=小星星实例）；任意谱通用化时本节点降级 event-only——定稿轮注意

  - id: n2_set_tempo
    title: 定速度
    lead: human                     # 硬护栏：模型估的 BPM 只能当建议（doc 1 §3.2）
    goal: 一起定下这首歌的速度
    model_does: [给出 90-108 BPM 建议并说明理由]
    needs_human: true
    done_when:
      all:
        - { event: user_confirmed, key: tempo }
        - { structure: tempo_set }
    then: [set_tempo]               # 唯一一次 tempo 写入（bar 守门前提）

  - id: n3_find_wrong_note
    title: 找错音（听辨模进/重复）
    lead: human                     # 耳朵判定权在人（rulings §五-7 人耳=音频域符号源）；符号复核在模型
    goal: 第二轨旋律有一个音被改错了——先听出在哪一段，再找到它改对
    model_does:
      - project.markers.upsert 标注 原型/A 段/B 段（可锚定参照物）
      - 讲解 repeat 与 sequence（调内模进），用 analyze findings 作证
      - A/B 循环两轨（transport_set_loop + solo 切换）
      - note_sequence_match mismatches 定位错音（expected_pitch 口径）
    needs_human: true
    done_when:
      judge: note_sequence_match
      target:    { clip: melody_corrupted, range_beats: [0.0, 48.0] }   # 全曲范围（难度由 hint_ladder 收敛，不缩判定域）
      reference: { asset: theme, range_beats: [0.0, 48.0] }
      accept_transposed: false
    hint_ladder: [听全曲自由找, 循环 9-12 小节, 并排演奏 bars 1-2 vs 9-10, 指出 bar 10 请学生指音]
    teaching:
      objectives: [obj1, obj2, obj3]
      evidence_hooks:                   # 双挂接：概念讲解窗口 + 错音诊断差分（参数与预期均经 276 核离线复算，附录 A）
        - id: concept_b_section         # 概念讲解：B 段窗口恰 3 条（min∈[5,7] 稳定）
          analyze: midi.analyze_transformations
          params: { clip_id: <theme_track>, range_beats: [8.0, 32.0], transformations: [repeat, sequence], key: { tonic: 0, mode: major }, min_match_notes: 7 }
          expected_findings:
            - { kind: repeat,   proto_bars: [5, 6], match_bars: [7, 8],  shift_semitones: 0 }
            - { kind: sequence, sequence_quality: diatonic, proto_bars: [3, 4], match_bars: [5, 6], diatonic_steps: 1, shift_semitones: 2 }
            - { kind: sequence, sequence_quality: diatonic, proto_bars: [3, 4], match_bars: [7, 8], diatonic_steps: 1, shift_semitones: 2 }
        - id: diagnose_a_prime          # 错音诊断：改错轨全曲 vs 完好轨差分（§2.2）
          analyze: midi.analyze_transformations
          params: { clip_id: <corrupted_track>, transformations: [repeat, sequence], key: { tonic: 0, mode: major }, min_match_notes: 7 }
          expect_absent:                # 完好轨有、改错轨无
            - { kind: repeat, proto_bars: [1, 4], match_bars: [9, 12], note_count: 14 }
          expect_present:               # 改错轨特有（8 音后截）
            - { kind: repeat, proto_bars: [2, 4], match_bars: [10, 12], note_count: 8 }

  - id: n4_variation_direction
    title: 选变奏方向
    lead: human
    goal: 决定变奏往哪儿走（换小调 / 改节奏 / 加装饰音 / 换风格）
    model_does: [按画像给 2-3 个方向，各写 8 小节试听片段]   # 生成后端可插拔双轨（rulings §七-20/21：编曲线或变换兜底 1b）
    needs_human: true
    done_when: { event: user_chose, key: variation_direction }

  - id: n5_make_it_ours
    title: 共同完成
    lead: model
    goal: 把选定方向发展成完整变奏，你至少提一次修改意见
    model_does: [MIDI 编写, 配伴奏填充空白轨, 基础混音]
    needs_human: true
    done_when:
      all:
        - { event: user_feedback_given, min_count: 1 }
        - { event: user_confirmed, key: final }
    advisory_checks:               # 建议性符号证据（非 done_when 判定器）：变奏段宜含至少一处变换族关系（教学叙事自洽）
      - analyze 变奏段 transformations 全族

  - id: n6_name_and_export
    title: 命名+导出（终局）
    lead: human
    goal: 给这首曲子起个名字，导出属于你们的版本
    done_when: { event: export_completed }
    finale_ref: finale             # 终局判定与仪式见 finale 块（§1.7）

guidance:
  pullback_after_turns: 4
  soft: 鼓励模型分享推理、询问感受；用户打字多是好事
  avoid: [重复询问画像已有信息, 让用户报模型能读到的数据, 同一问题换说法反复问]

write_boundary:
  allow: [import_midi_to_track, create_midi_clip, add_midi_notes, mutate_midi_note,
          delete_midi_note, apply_midi_note_patch, set_tempo, project.markers.upsert]
  confirm_downgrade: none          # 用户裁定待下，默认不降级（§6-4）

finale:                            # 全文见 §1.7（此处同文省略）
  ...

budget:
  max_turns: 60
  max_llm_calls: 120
  on_stop_loss: 保留现场上交
```

### 2.6 素材清单与哈希要求（MANIFEST 规格，验收标准"素材清单带哈希入档"）

```yaml
manifest_schema: vit.level_assets.manifest.v0
level: first-run-twinkle
license:
  melody: public_domain            # Ah! vous dirai-je, maman（18 世纪法国民歌）
  arrangement: vit_original        # 伴奏/编曲/谱面排版=Vit 原创（doc 1 §3.4：不使用现有录音与受保护译配歌词）
  lyrics: none
files:
  - path: assets/theme_twinkle_v0.mid
    sha256: <…>                    # 定稿轮制作后计算
    produced_by: <制作工具与命令（可复现）>
  - path: assets/theme_twinkle_corrupted_v0.mid
    sha256: <…>
    derived_from: theme_twinkle_v0.mid
    deviations: [{ at_beat: 37, pitch: 68, replaces: 69 }]
  - path: assets/score_twinkle_v0.pdf
    sha256: <…>
    note: Vit 原创排版谱面（拍谱腿输入；§6-7）
```

### 2.7 默认人格提示词块骨架（persona_default.md 蓝本；内容终审=定稿轮）

```
你是 {model_name}，{user_name} 的音乐搭档，制作人风格：直接、好奇、爱分享推理。
- 说什么、何时算数由乐局结构管；你只管怎么说（语气、词汇、教学框架）。
- 学生耳朵的事（哪个好听、哪里不对劲）——问，别替人判断。
- 你能读到的（谱面、参数、结构分析）——别问人，直接读。
- 讲结构时用你亲手指得到的坐标（marker、小节号、A/B 段），不说"大概那里"。
- 你不可替用户确认节点、不可自宣完成。
```

---

## 3. 内容工件目录结构提案

```
coord/levels/
  README.md                         # 目录语义+格式索引（定稿轮建；本稿即索引前身）
  first-run-twinkle/
    DESIGN-DRAFT-2026-11-06.md      # 本先行稿（draft 权威文本）
    level.yaml                      # 乐局卡 v0（定稿轮产出，蓝本=§2.5）
    persona_default.md              # 默认人格提示词块（定稿轮从 §2.7 骨架提取）
    assets/
      theme_twinkle_v0.mid          # 主题 MIDI（定稿轮制作）
      theme_twinkle_corrupted_v0.mid
      score_twinkle_v0.pdf          # 拍谱用谱面（定稿轮制作）
      MANIFEST.yaml                 # §2.6 规格（清单+SHA256+出处）
    project_template/               # 可安装工程模板（定稿轮；三轨布局 §2.3 的落成形态）
```

先行稿轮只落 `DESIGN-DRAFT-2026-11-06.md`（本文件）；其余全部为定稿轮工件位。`coord/levels/` 与卡面约束"文件域=coord/levels/（或 docs 同级新目录）"一致，选 coord 侧（内容工件属协作面，随卡池走）。

---

## 4. 裁定 18 闭合判据三锚点——实例映射草案

| 锚点 | 裁定 18 定义 | 本局实例化（可评估形态） |
|---|---|---|
| ① 结构完整 | 乐局目标产物在工程内齐（设计局=全节点 done_when）、无占位空白、整曲可从头播到尾 | n0–n6 全部 done_when 满足（jam_state 节点数组全 done）+ 轨 3「伴奏」非空（`accompaniment_blank` 为 no_placeholder_blanks 检查对象）+ transport 从 beat 0 播至最后 clip 末端（bars 1-12+变奏段）无错误 |
| ② 工作面安静 | 无未决提议/未回复问询、近几轮无实质工程修改（只听不改） | pendingmanager 无 pending 提议/问询；近 `quiet_turns_min=2` 轮 write_boundary 白名单命令零调用（只有读/播/聊） |
| ③ 软信号 | 用户语言收尾倾向（加权项非必要条件） | 用户消息含收尾倾向语义（"就这样吧/差不多了/收工"）→ 加权，`required: false`——无软信号不阻塞闭合，有软信号提高问询触发优先级 |

**流程实例化**：①②达阈值 → agent 携证据问询（语义模板 `closure_inquiry`，人格渲染；证据含可锚定坐标："bars 1-12 主题+变奏段已齐、伴奏轨已填、刚才两轮我们只在听"）→ 用户确认 → 命名（n6 前段）+导出（仪式）→ settle（memory_writes 五项 + obj1-3 检查入轨迹 + 轨迹归档）。用户主动说"导出吧"= 用户侧闭合信号，免问询直走仪式（裁定 18 原文）。完成≠停止（挂起合法）、完成≠冻结（局后改工程不回滚完成态）均以 finale.edges 落卡。

---

## 5. 与 280 校验门的前置对齐清单（互引面）

280 卡验收标准要求"格式与 280 的 lesson_card 校验门需求一致（双方对齐签字=回执互引）"。本稿已覆盖 280 卡目标 3 的全部明示需求（小节存在 ✓ §1.9-4；检查方式有判定器 ✓ §1.2/1.9-1；未知判定器名直接拒绝 ✓ §1.9-1；旧状态缺省语义+restore fail-closed 的 schema 侧前提 ✓ schema_version 拒绝路径 §1.9-11）。**本稿新增、需 280 侧确认的对齐项**：

1. 交互事件枚举（§1.2）与结构谓词枚举进校验门拒绝路径（§1.9-2/3）；
2. `pitch_set_match` 语义定案（276 核未覆盖；本稿提案=音高多重集 + set_mode，§1.2）；
3. `accept_transposed` 作为 note_sequence_match 的卡面裁决字段（276 judge.go 注释预留的"由乐局卡裁决"落点）；
4. done_when 组合子（all/any）嵌套深度建议限一层（本稿实例全部单层；校验门可拒绝深嵌套以保专家可写性）；
5. 内嵌 notes 参照表作为判定输入源（加载器合成 note id `asset:<id>#<k>`）——判定器零工程依赖、校验门零栈可跑的根据；
6. 运行时轨槽名（`melody_imported` / `melody_corrupted` / `theme_track` 等，target.clip 与 evidence_hook.params.clip_id 引用）的声明位与校验深度——本稿暂由 §2.3 轨布局表约定，280 可选择把轨槽声明升为卡字段（如 `runtime_slots:`）并进校验门。

---

## 6. 开放问题清单（定稿前须确认项）

| # | 问题 | 归属 | 先行稿处置 |
|---|---|---|---|
| 1 | **裁定 18（终局判定修订版）用户确认态**——现文"待确认"；若用户翻案修订，finale 块按修订文重做 | 用户+主管 | 按现文设计（§1.7），定稿轮若翻案则 finale 节重写 |
| 2 | **276 段 3 终验后 schema 冻结**——检测核/命令面返回形态若变（bar 换算、字段名、Finding 投影），§1.3 对齐表与 expected_findings 同步修订 | 主管（276 终验） | 以 276 段 1+2 已验收形态为基线 |
| 3 | **素材版权口径**——公版旋律+Vit 原创编曲成立；发布前懂版权者核实（doc 1 §3.4 原句）；MIDI 制作工具与出处标注进 MANIFEST | 用户（+法务核实） | license 块按 §2.6 落 |
| 4 | **RiskConfirm 乐局内写入降级**（rulings §六挂账，主管倾向首版不降级） | 用户裁定 | `confirm_downgrade: none`（默认不降级） |
| 5 | **280 判定器三件语义定案**——pitch_set_match 集合口径、accept_transposed 契约、组合子深度限制、轨槽声明位（§5 六项） | 280 执行+主管 | 本稿提案值 |
| 6 | 拍谱腿（286）依赖 Audiveris/music21 用户部署；OMR 噪声容差（装饰音/自由节奏 artifacts）首版 fail-closed 精确匹配是否够用 | 286 勘察+用户 | n0 带 fallback 直导 theme；n1 精确匹配 |
| 7 | 谱面素材 score_twinkle 排版来源（自排 vs 公版谱影印）——版权口径倾向自排（Vit 原创） | 用户+定稿轮 | 自排（vit_original） |
| 8 | persona_default.md 内容终审（骨架 §2.7）——人格口吻属用户/主管裁量 | 用户+主管 | 骨架先行 |
| 9 | 教学校准已离线机械验证（附录 A：以 276 核心源码只读复制复算，min 阶梯 30/20/8/6/6、B 窗口恰 3 条、差分信号确认）；定稿轮仍须**真栈**复跑 analyze 核对（内核 get_midi_clip_notes→命令面解析层叠加后的端到端一致性，占栈须登记） | 定稿轮（占栈须登记） | §2.1 校准注记 + 附录 A |
| 10 | budget 止损数值（max_turns/max_llm_calls）——按 §8 概率性运行纪律与试跑读数定值 | 定稿轮 | 占位 60/120 |
| 11 | no_placeholder_blanks 豁免口径——用户明确说"伴奏就留白"是否允许闭合（显式豁免事件 vs 不允许） | 用户裁定 | 无豁免通道（预留问题） |
| 12 | 素材 MIDI 的力度/通道约定（全曲固定 velocity 还是表现力度）——影响听感与 rhythm_match 无关但影响试听质量 | 定稿轮 | 建议 velocity=100 固定、channel 0（制作简单可复现） |
| 13 | 是否为 280 互测物化 `level.draft.yaml`（从 §2.5 提取的可解析文件）作为共享测试夹具 | 280 领取时协商 | 本稿自包含；提取成本一行 |

---

## 7. 定稿轮差额工作清单（从本稿到验收）

1. 前置核对：裁定 18 确认态 + 276 段 3 终验结论 + 本稿 §6 开放问题中归属"用户/主管"项的裁定；
2. 产出 `level.yaml`（蓝本=§2.5，notes 全表 42 行生成）+ `persona_default.md`（§2.7 终审版）；
3. 制作素材二进制：theme MIDI（42 音，§2.1 全表）、corrupted MIDI（§2.2 偏差）、score PDF（§6-7 口径）+ MANIFEST 哈希入档；
4. 真栈校准腿（占栈登记）：import 两轨 → set_tempo 100 → analyze(theme轨 B 窗口 [8,32), key=C, min=7) 核对恰 3 条 findings；analyze(两轨全曲, min=7) 核对差分（完好 6 条 / 改错：14 音 repeat 缺失 + 8 音后截出现）；note_sequence_match 全曲（idx 33）/分段（idx 5）两口径核对恰一处 mismatch——预期值全部以附录 A 离线复算为准；
5. YAML 可解析自验 + 与 280 校验门互测（互引回执）；
6. 卡面回执补定稿段 + 提交推送。

---

## 附录 A：判定预期机械验证记录（先行稿轮，2026-11-06）

**方法**：将 276 worktree（`D:/Vit_DAW_wt_276impl` @ `port/edu-symbol-core-1`，commit `f2950cd7`，段 1+2 已验收形态）的检测核三文件（`edu.go`/`transform.go`/`judge.go`）**只读复制**到中立临时模块（`D:/tmp_279_edu_verify/`，不属任何 worktree、不入版本），构造 §2.1 的 42 音主题与 §2.2 改错版（@37→68），直接调用 `edu.DetectTransformations` / `edu.JudgeAgainstPrototype` 复算。零仓库改动、零真栈、零 LLM。两 worktree 均未触碰。

**结果摘要**（完整输出见下）：

| 实验 | 结果 |
|---|---|
| 完好轨全曲（rep,seq, key=C）findings 数 by min | min=3 → 30；min=4 → 20；min=5 → 8；min=6 → 6；min=7 → 6 |
| 完好轨全曲 min=7 的 6 条 | repeat B1→B2(7)；repeat A→A' **14 音整段**；diatonic A2→B1(+1)、A2→B2(+1)、B1→A'2(−1)、B2→A'2(−1) |
| 完好轨 B 窗口 [8,32) | min∈[5,7] 均恰 3 条（repeat B1→B2 + diatonic A2→B1/B2）——概念讲解挂接的稳定区间 |
| no-key 对照（min=4） | 12 条、无任何 diatonic——「调内模进必须带 key」已验证 |
| 改错轨全曲 min=7 差分 | `repeat bars1-4→9-12(14音)` 消失；`repeat bars2-4→10-12(8音)`（"G– F F E E D D C–" 后截）出现；其余 5 条与完好轨一致 |
| 改错轨 B 窗口 [8,32) min=7 | 与完好轨逐条一致（损坏隔离性验证） |
| judge 全曲 [0,48) | pitch_mismatch、rhythm_matched=true、恰 1 mismatch：attempt_index=33、actual=68、expected=69、prototype=69 |
| judge 窗口 [32,48) | 同上但 attempt_index=5（14 音窗口内第 6 音） |

**关键输出摘录**（`go run`，go1.26.2 windows/amd64）：

```
== theme FULL min=7 key (42 notes): 6 findings
   sequence diatonic  proto[7..13]→match[14..20] shift=+2 deg=+1 n=7     # A2→B1  bars3-4→5-6
   sequence diatonic  proto[7..13]→match[21..27] shift=+2 deg=+1 n=7     # A2→B2  bars3-4→7-8
   repeat             proto[14..20]→match[21..27] shift=+0 deg=+0 n=7    # B1→B2  bars5-6→7-8
   repeat             proto[0..13]→match[28..41] shift=+0 deg=+0 n=14    # A→A'   bars1-4→9-12
   sequence diatonic  proto[14..20]→match[35..41] shift=-2 deg=-1 n=7    # B1→A'2 bars5-6→11-12
   sequence diatonic  proto[21..27]→match[35..41] shift=-2 deg=-1 n=7    # B2→A'2 bars7-8→11-12
== theme B-window[8,32) min=7 key (21 notes): 3 findings
   sequence diatonic proto[0..6]→match[7..13] shift=+2 deg=+1 n=7
   sequence diatonic proto[0..6]→match[14..20] shift=+2 deg=+1 n=7
   repeat            proto[7..13]→match[14..20] shift=+0 n=7
== corrupt FULL min=7 key (42 notes): 6 findings
   …（同完好轨 5 条，除 A→A' 14 音 repeat 外）…
   repeat proto[6..13]→match[34..41] shift=+0 n=8                         # 后截 bars2-4→10-12
== corrupt B-window[8,32) min=7 key: 3 findings（与完好轨逐条一致）
judge full  corrupt vs theme: status=pitch_mismatch transposition=0 proto=42 attempt=42 rhythm=true mismatches=1
    mismatch idx=33 note=t33 actual=68 expected=69 proto=69
judge [32,48) corrupt vs theme: status=pitch_mismatch … rhythm=true mismatches=1
    mismatch idx=5 note=t33 actual=68 expected=69 proto=69
```

**修正记录（诚实边界）**：本稿初版曾按手推断言"min=4 恰 5 条 findings、A1→A'1/A2→A'2 分列、改错退化为 5 音前截、mismatch idx=35"——机械复算全部推翻（成对音细胞结构的组合回声 + 极大性合并语义 + 恢复点索引口径），已按实测改写 §2.1/§2.2/§2.5。此为先行稿轮内自纠，未出稿。
