# D11-ROLE-RECON-1：角色身份层前置勘察——轨道名语义与声学身份证据源盘点报告

- 任务卡：`coord/cards/todo/2026-09-27-D11-ROLE-RECON-1.md`
- 执行侧：PC 会话（GLM-5.3，L1 只读勘察档）
- 领取时 HEAD：`2397a94eb929693ce3e449881647ca562f1dd6ed`（origin/main 已同步）；勘察期间并行会话在同一工作树将 main 推进至 `2ef350dd`（L1-3-DESIGN-1 回执提交，仅动 coord/cards 与 docs/QUERY_ENGINE_V1_DESIGN.md，非本卡文件域）——**本报告行号以勘察基准 2397a94 为准**，L3/CommandDispatcher 等处行号已按当前树逐一实取（SEG-RECON 报告的 L3 行号在当前树已漂移约 20 行，引用其结论时以本报告行号为准）
- 领取时工作树：`M VitApp/Workspace/default_project.xml`（运行时工程状态）+ 若干 untracked `coord/runs/*`（其他卡工件）——本卡零代码改动、零触碰
- 勘察日期：2026-09-27
- 交叉引用：`coord/runs/L2-3-PROTOCOL-RECON-1/CHALLENGE_CONFIRM_INVENTORY.md`（下称 PROTOCOL-RECON，§6 已判"角色身份＝role_guess 无确权回路"）、`coord/runs/CCB-VIEW-RECON-1/CCB_VIEW_INVENTORY.md`（CCB-VIEW）、`coord/runs/L2-2-SEG-RECON-1/SEGMENT_DSP_INVENTORY.md`（SEG-RECON）；待跑的 MEMORY-RECON-1 卡（工程事实载体＝决策账本/mixboard ledger/projectstore append-only 面，本报告 §5 互证）
- 路线图锚：D11（寻址＝轨道名/segment ID 零判定；身份＝证据生成假设→质询→确权→固化）

---

## 0. 执行摘要

1. **轨道名角色推断已经是生产链路的一等公民，但全部是 Go 侧硬编码词表、无 LLM 参与、无确权回路**：`role_guess` 由 project package 构建时从轨道名现场猜出（`mixboard/project_package.go:116`），经四级解析链（explicit_role 0.90 → tom_assignment → mom_role_guess 0.65 → track_name_inference 0.72）进入静态平衡能力包，角色未解析的轨**直接被排除出该能力**（`staticbalance/model.go:362-367`）。三套独立词表并存（9 类/15 类/11 组，互不完全一致）。
2. **四声学特征：瞬态与立体声宽度代理现成可用；频谱重心有六频段粗粒度替代；基频零存量**。真实 D1 工件实测：`correlation_estimate 0.751 / correlation_state "coherent" / phase_deviation 0`、六频段（20-20k）unit_energy 全 ready；全库无任何音频 F0 分析（仅 MIDI 音符号）。
3. **上下文证据面"内核完整、观察投影裁剪"**：内核 `get_project_state` 暴露 `track_groups`（**带组级 color** 的 vit_track_groups.v1 持久树）、完整 folder 父子（parent/depth/children/folder_behavior）；但 mixboard project package 只拷贝子向 folder 布尔字段，**不拷 parent_track_id/depth/track_groups**；单轨颜色不暴露（全库 grep 零命中）。
4. **多标签承载无障碍**：`[]string` 标签字段是仓库惯用形态（ReasonTags/Tags/RiskTags/ClusterKeys/Llabels 五处先例），TOM 的 `ClusterKeys` 就是"一轨多簇键"的直接先例；现存 role_guess 是单串字段，与多标签承载无结构冲突。
5. **固化落点推荐双层**：agent 侧 projectstore 证据账本（`.vit_agent/` append-only、kind+SHA256+refs、PutEvidence 已活用）+ 内核侧 ValueTree 工程账本（VIT_TRACK_GROUPS 的 `origin` 字段与 VIT_PROJECT_MARKERS 的 `source/confidence` 字段天然承载"发现层写入、可被命名层覆盖"语义）。与 MEMORY-RECON 卡的工程事实载体清单互证一致。

---

## 1. 轨道名语义的现存使用面（目标 1，锚点 10 处）

### 1.1 role_guess 的生产—解析—消费全链

**生产者（唯一）**：project package 构建时逐轨现场猜测。

- `agent/internal/mixboard/project_package.go:116`——`"role_guess": guessTrackRole(name, trackType)`；同函数 :109-135 建立轨道摘要（name/track_name/user_label 三名同源 :111-113，`stereo_position` :126，folder 布尔族 :129-134）。
- 词表：`agent/internal/mixboard/project_package.go:1571-1595 guessTrackRole`——9 类（master/vocal/kick/snare/drums/bass/guitar/keys_or_synth/bus），`strings.Contains` 子串命中，中英简繁词形（vocal/vox/主唱/人声/歌声/声乐；kick/bd/底鼓/大鼓；bass/low end/sub/贝斯/低音/低频…），默认 `unknown`；**注意 "lead" 单词不在此词表**——"Lead" 名的轨此层判 unknown（对照 1.2 的 inferRole 词表，同轨两层结论不同，见 §6 G1）。
- 插件侧平行体系：`project_package.go:1551-1569 guessPluginRole`（eq/dynamics/space/utility 四类，:1544 进 plugin_chain_summary）。

**解析链（消费者 1：静态平衡能力）**：四级优先链，带置信度与门槛。

- `agent/internal/staticbalance/model.go:297-309 buildTrack` 内：① 行内 `role/role_hypothesis/role_guess` 显式值 → `explicit_role` 0.90（:297-299）；② TOM 角色（`collectTOMRoles` :428-449 从 group_proposals/manifest 收集）→ `tom_assignment`（:300-301，:447/:458）；③ MOM 行 role_guess → `mom_role_guess` 0.65（:302-303）；④ `inferRole(TrackName)` → `track_name_inference`（:304-308）。
- 披露面：`agent/internal/capabilitycontext/static_balance.go:155-167`——`role_source_priority: ["explicit_role","tom_assignment","mom_role_guess","track_name_inference"]`（:166）与固定六函数集 `foreground/rhythm_anchor/low_end_anchor/harmonic_bed/support/effects`（:165）进 RoleModel；`tom_roles` 覆盖率进 EvidenceStatus（:136）。
- **角色是能力门**：`staticbalance/model.go:362-367 trackEligibility`——Role 为空/unknown → `role_unresolved` 排除；置信度 <0.60 → `role_confidence_low` 排除。**角色身份不解决，轨就进不了静态平衡**——D11 身份层的现行业务后果。
- 函数映射：`staticbalance/model.go:599-616 roleFunction`；同义词归一：:618-636 `normalizeRole`（vocal/vocals/lead_voice/voice/main_vocal→lead_vocal 等）。

**解析链（消费者 2：TOM 投影）**：命名优先→技术回退，两级裁决。

- `agent/internal/tom/projection.go:83-143 roleRules`——11 命名组（backing_vocals 0.94 / vocals 0.92 / drums 0.91 / bass 0.90 / strings 0.88 / synths 0.86 / guitars 0.86 / keys 0.86 / fx 0.84 / returns 0.82 / buses_prints 0.78），每组 tokens/phrases/substrings 三层匹配面；:151-155 `ignoredNameTokens`（audio/track/trk/clip/take/stem/wav/aiff… 先剔除无语义词元）。
- `tom/projection.go:331-338 decideAssignment`：命名命中即采（附技术+声学证据）；:340-388 `matchNamingRole`——三种证据形态 `name_phrase`（+0.03 加成）/`name_token`/`name_compact`（去空格复合词），多证据形态再加 0.03（:369-371）；:390-426 `technicalFallback`——静音（-80dBFS）/过热（近 0dBFS）/mono/短 clip（<15s）/长立体声 stem（≥95% 全长+2ch）五类技术分组，**全部 needsReview=true**，角色带 `_unknown` 后缀（mono_source_unknown 等）。
- 策略声明：`tom/projection.go:166`——`PrimaryStrategy: "name_id_first_then_technical_fallback"`；披露分级 :866-920 `buildDisclosurePlan`（命名覆盖 ≥0.70 且技术回退 ≤max(2,20%) → naming_id 首阶；弱名 → technical/DAD 轻量事实阶）。
- **角色标签的自我声明**：`tom/projection.go:1130-1145 summaryMD`——"Role labels are hypotheses; folder proposals require user confirmation."——现行口径已把角色定位为**假设**，组方案要求用户确认（`GroupProposal.NeedsConfirmation` types.go:137）。

**解析链（消费者 3：agent 主循环）**：

- `agent/internal/agentloop/message_loop.go:4188-4204 messageLoopTrackHasVocalEvidence`——人声证据判定：先读 role_guess（vocal/voice/lead_vocal，:4192-4195），**再对轨道名/clip 名做子串匹配**（词表 vocal/voice/lead vocal/lead_vox/vox/主唱/人声/人聲，:4196-4203）——这是"Go 代码直接用轨道名做角色推理"最直白的一处。
- `agent/internal/chat/mix_tick_confirmation.go:1207/:1508/:1538`——用户可见的确认/回复文本按 `name/track_name/user_label/role_guess` 键序取轨道显示名——**role_guess 与轨道名一起进用户对话面**。

**LLM 可见面（模型看到什么）**：

- `agent/internal/contextruntime/model_projection.go:1202-1230 digestProjectTrackIdentities`——project.structure view 在 hot 层强制暴露的轨道身份表，**恰为三字段：track_id / track_name / role_guess**（:1219-1223；CCB-VIEW 报告 §1 注明"否则模型无法继续 track 定向观察"）。
- `agent/internal/capabilitycontext/free_state_observation.go:752`——project.structure 第一层裁剪的 7 字段含 role_guess；:910 masking view 的轨道行也含 role_guess。
- `agent/internal/agentloop/message_loop.go:6693`——MOM multitrack compared_tracks 进消息循环的紧凑键含 track_name/role_guess。
- `agent/internal/agentloop/ccb_model_prompt.go:62`——观察请求示例的 target_ref 用 `label: "<visible track name>"`——轨道名是模型定向观察的寻址词汇。
- **LLM 产出侧无结构化角色输出**：全链无任何"模型声明角色→落证据"的回路（与 PROTOCOL-RECON §6"role_guess 无确权"一致）——模型只消费角色猜测，不生产、不确认。

### 1.2 三套硬编码词表对照（"强先验"的现行形态）

| 词表 | 位置 | 类数 | 匹配法 | 分值/置信 | 词形覆盖 |
|---|---|---|---|---|---|
| guessTrackRole（生产者） | mixboard/project_package.go:1571-1595 | 9 | 子串 | 无分值（直接判） | 中英简繁 |
| inferRole（静态平衡末级回退） | staticbalance/model.go:638-668 | 15 | 子串 | 0.72 | 中英简繁（backing_vocal/kick/snare/percussion/strings/pad/keys/effect/lead_instrument 细于 9 类表） |
| TOM roleRules | tom/projection.go:83-143 | 11 组 | phrase/token/compound 三层+多证据加成 | 0.78–0.94 | **纯英文**（无中文词形） |
| （第四套·人声专用） | message_loop.go:4196-4203 | 1 角色 | 子串 | 布尔 | 中英繁 |

**"什么算强"的现行答案＝子串命中硬编码角色词表即判**：TOM 打 0.78-0.94 并按证据形态数加成；staticbalance 0.72；无正则、无置信分级词表、无用户自定义词表、无词表版本号。同一条轨在不同层可能得到不同角色串（例："Lead" 在 guessTrackRole 判 unknown、在 inferRole 判 lead_instrument 0.72、在 TOM 落 lead_instrument 规则缺失→technical fallback，见 §6 G1）。

### 1.3 命名形态盘点（真实摘录）

| 形态 | 真实样例 | 来源 |
|---|---|---|
| 数字默认形（Track N + 数字 track_id） | `track_id: "1007", name: "Track 1", role_guess: "unknown", track_type: "hybrid"`（逐字摘录）；烟测工件 Track 1/2/3 | 真实 D1 运行工件 `agent/bin/VitApp/Workspace/Artifacts/mixboard/mix_55dfa435f18e/observations/obs_20260720T123658_95b5cf6b61b0.json`（project_package.tracks[0]）；`cap_v1_plugin_effect_eq_direct_smoke_1784965690_1/context_pack.json` |
| 英文角色名 | "Lead Vocal"、"Vox"、"Bass"、"Drums"、"Kick"、"Gtr"、"Lead"、"Harmony"、"One"、"Two"、"vocals"、"drums"（大小写混用） | agent/internal/harness 与 mixboard 测试 fixture（track_name 字段实测 grep 汇总） |
| 源文件名形 | `"file_path": "D:\\Vit_DAW\\Paper Crown.mp3"`——TOM 的 profile 含 SourcePath/源 basename 词元（tom/projection.go:176 SourcePath 进 profile） | 同上真实工件 acoustic.file_path |
| 中文形 | 词表已备中文词形（主唱/人声/贝斯/底鼓/军鼓/吉他/钢琴/键盘/铺底/和声/效果…），测试中见"通用宏控件"（宏控件名非轨名）——**中文轨道名在两层词表可命中，在 TOM 词表不可**（纯英文） | staticbalance/model.go:644-658、mixboard/project_package.go:1576-1589 实录；TOM roleRules :83-143 无中文 |
| 模板残留形 | Master/Marker/Arranger/Chord/Input 1/Input 2（默认工程 XML） | VitApp/Workspace/default_project.xml name 属性实测 |
| 无语义词元 | track/trk/stem/clip/take/wav/aiff…（TOM 显式忽略表） | tom/projection.go:151-155 |

**判定**：卡面预判的三形态（英文角色名/中文/Track 1007 数字形）全部实证，且真实 D1 工程实际处于**最难形态**——数字默认名+unknown 角色，命名先验完全失效、技术回退接管——这正是 D11 声学特征身份判定的现实需求背景。

---

## 2. 四声学特征可得性矩阵（目标 2）

判定口径：**现成可用**（数据+消费链路都在，锚点）/ **可派生**（现有数据可算出近似值，注明层与公式位置）/ **零存量**（需新增分析）。

| 特征 | 内核数据面 | agent 数据面 | 判定 | 真实摘录（D1 工件实测） |
|---|---|---|---|---|
| **基频范围（F0）** | 无音频 F0 分析（全库仅 MIDI 音符号 VitApp/Source/Service/MidiService.cpp:316 等，非音频分析） | 无任何 pitch/f0 字段（grep 零命中） | **零存量** | — |
| **频谱重心** | 无发布指标；TiledSpectrogramBaker 的 `PoolMode::Centroid`（:59-64, :732-755）是 336 log-bin **bin 内**池化策略（渲染用），非轨级重心值 | 六频段能量全 ready（BandEnergySummary 透传）：sub 20-60/bass 60-250/low_mid 250-500/mid 500-2k/presence 2-6k/air 6-20k Hz，各带 min_hz/max_hz/unit_energy/left/right_unit_energy → **可派生六段加权重心**（agent Go 纯计算，零音频接触）；MOM 已有近似语义：bandTendency（mom/project_relation.go:539-561 低频组/中低/presence/air 计数分级）、dominant_bands（mom/types.go:167）、bandConflictCandidates（:514-537 领袖轨 0.7 能量比） | **可派生（粗粒度）** | `"bass": {min_hz:60, max_hz:250, unit_energy:0.2396, status:"ready"}`（六段全 ready 逐字摘录） |
| **瞬态** | L3 瞬态事件直出：`deriveFineEvidence`（L3AcousticAnalyzer.cpp:241 起；频率事件 :273-279、瞬态 :288，**128 事件上限**）；发布 `transient_events`（:702-703，onset/body/sustain dBFS + attack_body_contrast_db/sustain_decay_db） | DOM 投影 `TransientStructure`（dom/types.go:146-162 TransientEvent 字段表：OnsetSeconds/BodyEnd/SustainEnd/OnsetDBFS/AttackBodyContrastDB/SustainDecayDB）；CCB view `track.transient_structure`（dom/context.go:21）；声学包 time_segments 5s 能量段（crest_db/energy_state） | **现成可用** | `"time_segments":[{end_seconds:5, crest_db:22.547, energy_state:"medium", peak_dbfs:-0.109, rms_dbfs:-22.656, start_seconds:0},…]` |
| **立体声宽度** | StereoRelationSummary 直出（L3AcousticAnalyzer.cpp:740-763）：left/right_energy、left/right_level_db、balance_db/balance_state、**correlation_estimate**（:459 计算：jlimit(-1,1, sumLR/corrDenom)）、correlation_state、**phase_risk、mono_compatibility_risk**（:757-758 由 correlation 派生的风险标签） | 逐轨 `stereo_relation`（project_package.go:166 挂接）；CCB view `track.stereo_space`（track.\<id\>.slow.stereo.summary）；`stereo_position`（pan/balance_db，project_package.go:1513-1529）；MOM stereoOverview（project_relation.go:573-580 center_heavy/off_center/phase_risk 分类）与 project_stereo_spread（:261 含 widest_balance_tracks）；TOM 技术回退用 channel_count+duration（projection.go:405-419） | **代理指标现成可用，无显式"宽度"标量** | `"correlation_estimate": 0.751, "correlation_state": "coherent", "phase_deviation": 0, "phase_negative_ratio": 0, "balance_db": -0.028, "balance_state": "centered"`（逐字摘录） |

**四特征对身份判定的意义判读**（供 D11 证据融合设计）：

- **人声判定**（lead/backing 分离）：F0 零存量是最大缺口——谐波性/音高范围是人声 vs 乐器的最强判据，现只能靠 presence 频段能量+瞬态密度弱判；
- **贝斯/低音判定**：sub+bass 频段 unit_energy 占比（现成）+ 长持续（time_segments 现成）已是可用组合；
- **鼓组判定**：transient_events 密度（现成，注意 128 上限）+ onset/body 对比度（现成）已是可用组合；
- **铺底/立体声素材判定**：correlation_estimate + phase_negative_ratio（现成）+ channel_count（TOM 现成）可用。

### 2.1 轨道名强先验形态（目标 2 子项）

见 §1.2 对照表——现行"强"＝硬编码词表子串命中；**不存在**正则词表、分级置信、用户自定义词表、词表版本化。D11 若定义"强先验"，建议沿用 TOM 的证据形态分层（phrase>token>compound）与多证据加成模式（tom/projection.go:347-371），它已是仓库内最精细的名字证据模型。

---

## 3. 编组/相邻轨/颜色惯例的上下文证据（目标 3）

### 3.1 编组（控制组）——内核完整持久树，含颜色

- **数据模型**：`VitApp/Source/Service/TrackGroupService.cpp:13-29`——`VIT_TRACK_GROUPS/VIT_TRACK_GROUP` ValueTree，schema `vit_track_groups.v1`（:51）；字段 `group_id/name/**color**/type/origin/enabled/suspended/**member_track_ids**/link_volume/link_pan/link_mute/link_solo/created_at/updated_at`。
- **get_project_state 暴露**：`VitApp/Source/Service/CommandDispatcher.cpp:3385-3388`——`track_groups`（+别名 `groups`、计数 `track_group_count`）随每次 get_project_state 全量快照返回。
- **命令面全套**：`track_group_list/create/update/set_members/delete`（CommandDispatcher.cpp:2403-2435 注册；服务实现 TrackGroupService.cpp:460 起）；建组默认 `origin="user"`（:505）、**组色按序默认调色板**（normalizedColor，:503）；master 轨不可入组（:426-427 校验）；undo 事务。
- **agent 侧消费：未接入观察投影**——mixboard project package 不收 track_groups（buildTrackSummaries 无此键，project_package.go:109-135）；TOM 从导入行+DAD 行构建（BuildFromImportRows tom/projection.go:157），不读组树。bridge 对 get_project_state 透传（agent/internal/bridge/bridge.go:244-247，仅用于初始化 shadow）——**数据可达、投影未收**。

### 3.2 文件夹结构——内核全量暴露，投影面裁剪

- `VitApp/Source/Service/CommandDispatcher.cpp:767-824 appendTrackTreeState`——每轨附 `parent_track_id/parent_folder_track_id/depth/track_depth/child_track_ids/direct_child_track_ids/descendant_track_ids/child_track_count/descendant_track_count/is_folder_track/is_folder_container/is_submix_folder/folder_behavior`（routing_bus|container）。
- folder 命令族：`folder_track.create/track.move_to_folder/folder_track.set_routing_bus/project.apply_track_organization`（CommandDispatcher.cpp:2526-2556）；TOM 的组方案正是喂 `apply_track_organization` 的上游（agent 侧 pending 流 message_loop.go:5036-5071）。
- **裁剪断言**：mixboard project package 只拷贝**子向**布尔/计数字段（project_package.go:129-134 is_folder_track/is_folder_container/is_submix_folder/has_child_tracks/child_track_count/descendant_track_count），**不拷 parent_track_id/parent_folder_track_id/depth**——观察投影里的 folder 信息只有"是不是容器/有几个孩子"，**没有"我的父组是谁"**。staticbalance trackEligibility 用子向字段排除容器轨（staticbalance/model.go:352-353）；gain_staging 读 `folder_role` 键（capabilitycontext/gain_staging.go:257）但**全库无生产者**——是预埋的空键。

### 3.3 颜色——组级有、轨级无

- 组级 color 见 §3.1（VIT_TRACK_GROUPS 每组一色，默认调色板按序）。
- **单轨颜色不暴露**：CommandDispatcher/TrackService/TrackGroupService 全量 grep `colour|getTrackColour|setTrackColour` 零命中；tracktion Edit 的轨色属性未进任何命令面。**"颜色惯例"上下文证据在轨级为零存量。**

### 3.4 相邻轨——隐含于数组序，无显式字段

- get_project_state 的 tracks 数组按 `te::getAllTracks(*edit)` 遍历序（CommandDispatcher.cpp:3297）＝排列顺序，**数组下标即隐含相邻性**，但无显式 `order/adjacent` 字段；
- agent 侧 `user_track_index` 补位：mixboard project package :115 透传 `user_track_index/index`；message_loop.go:6066-6067 缺省按遍历序补 index；mix_pending_candidate.go:155 进候选卡。**相邻轨推断可用 user_track_index 差分实现，零新增数据面。**

### 3.5 附带发现：轨道自定义属性先例

`VitApp/Source/Service/CommandDispatcher.cpp:850-851 createTrackState` 透出 `vit_type/vit_intent`（track->state 的自定义 ValueTree 属性）——**在轨道 ValueTree 上挂自定义语义属性并随工程持久化，有现成先例**（D11 若把确权身份写到轨级属性，这是内核侧承载形态参照）。

---

## 4. 多标签不互斥的承载设计输入（目标 4）

现存"一对象多属性"的数据形态（全部 `[]string`）：

| 先例 | 位置 | 语义 |
|---|---|---|
| `ReasonTags []string`（+unique 去重） | agent/internal/experiment/user_judgment.go:68, :202 | 盲测判断理由多标签，随 UserJudgmentEvidence 持久化 |
| `Tags []string` | agent/internal/mixstyle/style.go:28 | 混音风格多标签 |
| `RiskTags []string` | agent/internal/mom/types.go:170 | MOM 风险多标签 |
| **`ClusterKeys []string`** | agent/internal/tom/types.go:124（ManifestAssignment）, :153（TrackAssignment） | **一轨多簇键**——命名簇（naming:vocals）与技术簇（technical:mono/acoustic:hot_or_clipping）本可并列的承载形态（当前 decideAssignment 单裁决只填一键，但字段形态是多值的） |
| `Labels []string` | agent/internal/capabilitycontext/pack.go:33 | 能力包标签 |

**结论**：多标签承载是仓库惯用形态、零结构障碍；现存互斥的唯一环节是 role_guess/RoleHypothesis **单串字段**语义（一轨一角色）。D11 多标签身份（如"lead_vocal + 需复核低频"并存）可直接沿用 `[]string` 惯例；TOM ClusterKeys 的"命名簇+技术簇并列"是最贴近的语义先例。

---

## 5. 固化入账本的落点候选（目标 5，验收线 ≥2，实际给出 5 处、推荐 2）

| # | 落点 | 位置 | 持久性 | 身份适配度 | 互证 |
|---|---|---|---|---|---|
| **L1（推荐·agent 侧）** | **projectstore 证据账本** | `agent/internal/projectstore/store.go:14-16`（`.vit_agent/` 目录族+manifest.v2.json，schema vit_agent_store_manifest.v2）；`evidence.go:20-46`（EvidenceIndex/EvidenceBlob：**kind+SHA256+refs**，evidence_index.v1/evidence_blob.v1）；`PutEvidence` :49——已被活用（mixboard/persistence_v2.go:34 落 feature_snapshot，ref="obs:"+ObservationID） | 按工程、append-only、带预算（Budgets journal/evidence 分项 store.go:43-53） | **高**——确权身份可作新 kind（如 role_adjudication）落 blob，refs 挂证据链；查询面在 agent 近侧 | MEMORY-RECON 卡目标 1"工程事实=projectstore append-only 面"同源 |
| **L2（推荐·内核侧）** | **ValueTree 工程账本**：VIT_TRACK_GROUPS 与 VIT_PROJECT_MARKERS | TrackGroupService.cpp:13-29（**origin 字段**区分 user/agent 写入方）；ProjectMarkerService（SEG-RECON P1：marker 树 source/confidence/section_id 三字段"发现层写入、可被命名层假设覆盖"语义现成，schema vit_project_markers.v1） | 随工程文件持久化、undo 事务、用户可见可手改 | **高**——`origin="role_adjudication"`+member_track_ids 即"已确权角色组"；marker 树已被 SEG-RECON 判为段落承载首选，角色域同构复用 | SEG-RECON §6 P1；PROTOCOL-RECON §3（M4 attestation 的"指纹失配转 stale"再确权模式可借） |
| L3（弱候选） | journal 动作账本 | agent/internal/journal/journal.go:20-56（Action：Source/ConfirmationStatus/Rollback 链） | 按工程、分片限额 | 中——动作级留痕，非身份状态账本；确权"动作"可记此、"身份"不宜 | PROTOCOL-RECON §3 同款 |
| L4（边界候选） | workspace 快照 FreeStateLoops | agent/internal/chat/server.go:161, :6989, :7126 | 跨重启、会话/工作区级 | 低——工作区级非工程级；仅适合作"本会话已确权缓存" | PROTOCOL-RECON §3 M5"持久但不外推"同病 |
| L5（形态参照） | PCA attestation store | agent/internal/processorattestation/store.go（PROTOCOL-RECON M4） | 跨会话持久 | 形态参照——按二进制指纹非按工程；"一次认证终身可用、指纹失配转 stale"的再确权模式是身份失效语义的最佳样板 | PROTOCOL-RECON §3 唯一终身享用样板 |

**推荐组合：L1+L2 双层**——agent 侧账本供观察/推理近侧查询（身份判定的读取面），内核侧树供用户可见/undo/随工程迁移（身份的权威载体）；两者与 MEMORY-RECON 的三记忆划分（工程事实按工程）互证一致，且 L2 的 origin 与 L1 的 kind 都天然隔离"发现层写入 vs 用户手工"（对照 marker 树 source 过滤替换的既有纪律，SEG-RECON §6 P1）。

---

## 6. 缺口清单（供 D11 实现卡排卡）

| # | 缺口 | 锚点 | 影响 |
|---|---|---|---|
| G1 | 三套角色词表不统一、同轨异判（"Lead"：mixboard 层 unknown／staticbalance 层 lead_instrument 0.72／TOM 层无 lead 规则落 technical fallback；"One/Two"三层皆 unknown） | project_package.go:1571-1595 / staticbalance/model.go:638-668 / tom/projection.go:83-143 | 身份层若无统一词表（版本化），各层角色串与置信度不可比，证据融合无共同词表基座 |
| G2 | 基频（F0）零存量——人声/乐器判定的最强声学判据缺位 | 全库 grep 实证（仅 MidiService.cpp:316 MIDI 音符号） | D11 声学身份证据的最大单点缺口；落点可循 SEG-RECON 的 SegmentationPrimitives 槽位先例（AudioFeatureTypes 新特征类型+版本串） |
| G3 | 频谱重心无轨级直出（仅六频段可派生+MOM bandTendency 近似） | L3AcousticAnalyzer.cpp 频段定义；mom/project_relation.go:539-561 | 粗判可用、精细判（人声亮度/鼓组金属度）受限；派生计算落 agent Go 即可（零音频接触） |
| G4 | 单轨颜色零暴露；组色只在内核快照、观察投影不收 | §3.1/§3.3 | "颜色惯例"上下文证据链断在投影面；收编需改 project package 字段拷贝清单 |
| G5 | folder 父向信息（parent_track_id/depth）与 track_groups 在 project package 被裁 | project_package.go:129-134（只拷子向）；§3.1/§3.2 | 组上下文进不了观察投影，LLM 看不到"这轨属于哪组" |
| G6 | role_guess 无确权回路（猜测即用、永不回问、无用户修正入口） | PROTOCOL-RECON §6 已判；本报告 §1.1 全链无确认点 | D11 净新增面；质询机制起点=M5 盲测三要素（PROTOCOL-RECON §2 结论） |
| G7 | 观察目录无"身份"专用 view——role_guess 附在 project.structure/能量 view 里，无 track.identity view | free_state_observation.go:421-469 目录 18 view 无身份域 | D11 若要身份证据可观察/可引用，需新 view 或扩 structure view（CCB-VIEW §3 参数化缺口的既定约束适用） |
| G8 | transient/frequency 事件 128 条上限 | L3AcousticAnalyzer.cpp:273/:279/:288 | 密集瞬态轨（鼓组）事件截断；身份判定用密度统计需上限参数化或密度直出（SEG-RECON G3 同款） |
| G9 | `folder_role` 键全库无生产者（预埋空键） | capabilitycontext/gain_staging.go:257 | 若 D11 引入组级角色，此键可作消费面接点（需先建生产者） |

---

## 7. 验收对照（卡面五条）

| 卡面验收 | 本报告 |
|---|---|
| ① 轨道名使用面 ≥3 处锚点 | §1.1 十处锚点（生产 :116／四级链 :297-309／能力门 :362-367／TOM 裁决 :331-426／主循环人声判定 :4188-4204／LLM 身份表 :1202-1230 等）+ §1.2 三套词表对照 |
| ② 四声学特征可得性矩阵 | §2 四特征逐格判定（零存量/可派生/现成×2）+ 真实 D1 工件逐字摘录三段 + 强先验形态 §2.1 |
| ③ 上下文证据面结论 | §3：编组内核完整（vit_track_groups.v1 含色）投影未收；folder 父向被裁；单轨色零暴露；相邻=数组序+user_track_index |
| ④ 固化落点候选 ≥2 | §5 五处、推荐 L1 projectstore 证据账本 + L2 内核 ValueTree 树（origin/source 字段），与 MEMORY-RECON 互证 |
| ⑤ 报告入库 | 本文件 `coord/runs/D11-ROLE-RECON-1/ROLE_IDENTITY_INVENTORY.md` |

停止条件检查：四特征无"完全无数据面"情形（F0 零存量但落点先例现成），上下文证据与落点均有实锚，不触发；缺口以 G1-G9 上交，未硬凑可行性。

零代码改动声明：本卡全程只读勘察，未修改任何源码/测试/脚本；工作树变化仅为本报告与卡面回执（coord/ 下）。

---

## 8. 边界与未覆盖项

- `.claude/worktrees/` 过期快照未采信（grep 时显式排除）；`agent/bin/` 运行工件仅用作**真实数据摘录**证据（只读），不作代码锚点。
- Godot 前端仓（仓库外）的组/颜色 UI 写入面未跨仓取证（与 CCB-VIEW §6 同边界）——组色的用户实际使用惯例（用户起没起过组）无历史数据可考，仅能确认数据面存在。
- webui 呈现面（身份/角色在前端如何显示）未展开，属 E2E-WEBUI-1 域。
- MEMORY-RECON-1 未跑（卡在 todo）——§5 互证以其**卡面目标**为基准；其报告入库后若有出入，以双方报告交叉核对为准。
- spv1_p02 六轨 stems 工程的 .vit 为加密二进制（VIT1 头），未能直接取出轨名清单；命名形态以真实 mixboard 工件+测试 fixture 归纳。
