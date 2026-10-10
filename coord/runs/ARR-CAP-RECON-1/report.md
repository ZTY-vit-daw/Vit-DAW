# ARR-CAP-RECON-1：编曲能力线现状勘察报告（Mac 线候选②）

- 任务卡：`coord/cards/doing/2026-10-10-ARR-CAP-RECON-1.md`（池序 57；验收负责人=GLM 主管决策流）
- 执行侧：GLM-5.3-Flash（ZCode flash 执行会话，PC win32）；只读勘察零代码
- 勘察基线：origin/main = `e79cdc97`（领取时）；报告写就时 main 已被并行流推进至 `15a7264b`（核过 e79cdc97..15a7264b 仅 coord/ 卡面变更，代码锚点不受影响）
- 勘察日期：2026-10-10
- 性质声明：纯只读——零代码、零探针、未触碰工作树预存改动（VitApp/Workspace 两 XML）与其他 runs 目录；主输入=四个前产报告（L2-2-SEG-RECON-1 / L2-2-SEG-SMOKE-1 / REGION-OP-RECON-1 / ARRANGE-RECON-1）+ 五张 done 卡（SEG-1/SEG-2/SEG-SMOKE-1/MIDI-CMD-REGISTER-1/REGION-INTENT-WIRE-1）+ 两张 region 卡（GOAL-1 done / IMPL-1 冻存）+ 当前代码锚点亲核。
- 锚点口径：凡标注"亲核"的行号为领取基线当日 grep/sed 实读；承自前产报告且未复跑的锚点在引用处标"（前产）"。抽样亲核命中：harness.go:144/:3958/:3971/:4105-4112、internal/segmentation 零 importer、refschema.go:131-206、catalog.go:870-874/:1001、goalrunner_chat.go:4323-4328、epm/projection.go:236/:389、VspKernelReference.cpp:92/:111、CommandDispatcher.cpp:2390-2394。

---

## 〇、总览（一句话版）

**编曲线的两块地基——"结构输入"与"操作面"——状态不对称**：操作面（clip/范围/MIDI/markers/时间轴命令）大部分已就绪甚至已过真栈验证，而结构输入链（段落投影消费链）在内核产出与 agent 算法两端都已做完并验收，**唯独中间的 harness 摄入白名单没接**——`segmentation_primitives` 事件到 agent 即被丢弃，导致 SEG-2 切分算法成为零消费孤儿、EPM 只有启发式段落图可用、ref schema 无段落族可寻址。第一张卡的方向已被两次域外登记预告过（白名单扩容），本报告给出整链缺口清单与文件域冲突评估。

---

## 一、段落投影消费链现状（编曲的结构输入）

### 1.1 整链六段现状

```
内核 DSP 原语 ──✅──> VSP 事件通道 ──❌白名单──> agent 摄入/落盘 ──❌零消费──> 段落算法(agent) ──❌未接线──> 投影层 ──❌无族──> ref schema(模型可寻址)
```

| # | 链段 | 状态 | 锚点与证据 |
|---|---|---|---|
| S1 | **内核产出**（SEG-1，已合入+验收 pass） | ✅ 完成 | `L3AcousticAnalyzer.cpp` `deriveSegmentationPrimitives`/`publishSegmentationPrimitives`：载荷 `dad_l3_segmentation_primitives.v1` = onset_events（detected/published/truncated 诚实披露，发布上限 1024）+ onset_density（逐帧，与既有 L3 特征同网格）+ energy_novelty（逐帧+max/mean 汇总）；确定性三常量编译期固定并透出（kSegOnsetRiseDb=3.0/窗口 1.0s）；degenerate 源显式 not_evaluable。`AudioFeatureService.cpp` requestBake 路由含 SegmentationPrimitives（前产 SEG-1 回执亲核段） |
| S2 | **真栈产出已证**（SEG-SMOKE-1，pass） | ✅ 完成 | `seg_primitives_smoke.ps1` 双 run PASS（20260926_231942/232440）：A1 载荷 ready、A2 数值合法（onset 26/26 未截断+density/novelty 2679 帧）、A3 两轮排除 updated_at/request_id 后 canonical JSON 逐字节一致（484,377 字节，**跨 run 哈希一致**，决策侧 sha256 复核）；工件 `coord/runs/L2-2-SEG-SMOKE-1/` |
| S3 | **agent 摄入/落盘** | ❌ **主缺口（本报告核心）** | `harness.go:3958` IngestKernelTelemetry 四分支只认 render/waveform_envelope/spectral_field/L3 三摘要；`:4105-4112` `isL3AcousticSummaryFeature` = band_energy_summary/stereo_relation_summary/loudness_summary 三型（**亲核**）——segmentation_primitives 事件进 agent 即丢。落盘白名单同断：`l3_feature_log.jsonl`/`mixboard_feature_snapshot.json` 不收录该型（前产 SEG-SMOKE 发现②，本次复核当前代码仍如此） |
| S4 | **查询/请求面** | ❌ 缺 | `harness.go:144`（**亲核**）`mixboardObservationFeatureTypes = ["waveform_envelope","spectral_field","l3_acoustic_summary"]`——mix_observe 后台特征请求面不会请求该型。内核触发面本身是通的（warm_waveform_bake 带 feature_type 参数，mixboard 后台填充路径 `harness.go:4387-4399` 透传；SEG-SMOKE 探针直发 feature_type=segmentation_primitives 两轮 ready 实证）——**断点纯在 agent 侧摄入与请求清单，不在内核** |
| S5 | **段落算法**（SEG-2，已合入+验收 pass） | ⚠️ 完成但孤儿 | `agent/internal/segmentation/`（segmentation.go ~330 行）：`ParsePrimitives`(:153)/`DetectBoundaries`(:172)/`BuildApplySectionMarkersCommand`(:333)——确定性线性阈值切分（绝对参考刻度 4.0 onsets/s+0.05 线性 RMS、阈值 0.65、8s 最短段、置信三档、NE 五态）、中性 ID S1…Sn、命令构造 source=segmentation_primitives_v1 过滤替换。**全仓 grep 零 importer（亲核）**——组件级绿但无生产消费 |
| S6a | **投影层** | ❌ 无段落 peer | `docs/OBSERVATION_PROJECTION_MANIFEST.md` §2：8 peers（DOM/MOM/TIM/TOM/FXM/COM/EPM/RLM），无 segmentation 投影。现存唯一"段落"投影=EPM section_map（`epm/projection.go:236` buildSectionMapProjection、`:389` buildSectionCandidatesFromProfiles，**亲核**）——但它是**启发式代理**（输入=clip timing map+DAD 轻量波形包络+TOM group context，三策略 status 均为 "planned"），**不消费 segmentation primitives**。即当前存在两条并行未合流的段落来源：EPM 启发式（活、模型可见）vs SEG-2 确定性（孤、模型不可见） |
| S6b | **ref schema 模型可寻址** | ❌ 无族 | `internal/agentprotocol/refschema.go` 注册表（:131-206，**亲核**，M5 后 27 词条）无任何 segmentation/section/marker 族。技术上 `dad.l3.` 前缀族（:144，TargetKind=dad.l3，slot=scope）可容纳 `dad.l3.segmentation_primitives...` URI——但零生产者（S3 断则 S6b 无米下锅）。CCB v1 视图目录亦无段落/结构 view（manifest §3；EPM 明注"CCB v1 目录不含执行投影 view"） |
| S7 | **写回/执行面**（markers） | ✅ 完成 | 内核五命令 `project.markers.list/upsert/apply_section_markers/rename/delete`（`VspKernelReference.cpp:90-92` 区段；`CommandDispatcher.cpp:2390-2394` handler，**亲核**；ProjectMarkerService handleApplySectionMarkers 按 source 过滤替换+section_id/confidence 字段承载，前产 SEG-RECON P1）。agent 工具面五工具注册（`catalog.go:870-874`，**亲核**：list=RiskDirect 可自由读，apply_section_markers=RiskConfirm）；goal loop 白名单 `agentLoopProjectMarkerTools`（`goalrunner_chat.go:4323-4328`，**亲核**）；**agentloop 确认流已在**：`agentloop/execution_memory.go:574-576` pending_section_markers → 用户确认 → `message_loop.go:1722/:1767-1773` 路由 apply_section_markers + `:4348` prompt 提示——但该流当前只接 EPM 启发式段落图（"confirmed A5/EPM section map"） |

### 1.2 整链缺口清单（供排卡）

| # | 缺口 | 层 | 锚点 | 修复路径 |
|---|---|---|---|---|
| A-G1 | 摄入白名单不含 segmentation_primitives（事件丢弃+落盘缺失） | agent/harness | harness.go:3971/:4105-4112 | 白名单扩容+落盘行——即 SEG-SMOKE"域外登记"与 SEG-2"通道现状声明"两次预告的欠账 |
| A-G2 | 观察请求面不请求该型 | agent/harness | harness.go:144 | mixboardObservationFeatureTypes 扩容（可与 A-G1 同卡，同文件同性质） |
| A-G3 | segmentation 包零消费（SEG-2 孤儿） | agent | internal/segmentation（零 importer） | A-G1/G2 通后由消费方接线（新投影或 EPM 输入源） |
| A-G4 | 无段落 peer 投影；EPM 启发式与 SEG 确定性未合流 | 观察层 | manifest §2 + epm/projection.go:236/:389 | 新 peer（拓扑正确，仿 TOM disclosurePlan 模式）或 EPM 输入源切换——**设计决策项**，两案文件域不同（见 §三） |
| A-G5 | ref schema 无段落/section 族（模型不可寻址） | agentprotocol | refschema.go:131-206 | 注册新族（evidence_scheme_uri 型，M4/M5 先例：追加行+独立测试文件） |
| A-G6 | CCB 无段落 view | capabilitycontext | manifest §3 freeStateViewDefinitions | 若模型需按 view 披露段落再扩；EPM 走 LLMContext 不经 CCB 也成立，先不做 |
| A-G7 | marker 写入后无观察消费（写完即"消失"，不回流投影） | 观察层 | REGION-OP-RECON RECON.md:74（前产） | 投影消费 marker state——段落线后期项 |

### 1.3 附带容量事实（前产 SEG-SMOKE 登记，设计时需知）

- 内核 AudioFeatureService 30s merge 窗口节流（同 merge key 重复请求静默丢弃，无发布）——重复分析请求须隔 ≥30s 或变 key（AudioFeatureService.cpp:161 shouldMergeRecentAudioFeatureRequest，前产）。
- L3 分析单线程串行池：61-stem 全量导入时每 clip 一个整曲 bake，积压数十分钟（前产 run 224515 实证）——段落分析如需整曲多轨，预算面要按串行池排。

---

## 二、编曲操作面现状

### 2.1 clip/region 操作命令面——**基本齐备，范围→split 意图已接线**

**已有可复用（真栈或验收背书）**：

| 能力 | 锚点 | 状态 |
|---|---|---|
| clip.split（子 clip id 回传 left/right，0.01s 最小存活，undo 事务） | VspKernelReference.cpp:111（亲核）；ClipService.cpp:1496-1577（前产 A2） | 内核在；真栈由 INTENT-WIRE 场景 B 双轮 exit 0 背书 |
| clip.move/resize/remove/fade.set/gain.set 族 | VspKernelReference.cpp:109-116（前产 A1） | 内核在 |
| agent 工具：split_clip/move_clip/resize_clip/clone_clip/remove_clips/import_audio/import_media_to_track | catalog.go（split_clip=RiskConfirm，前产 A3；本次族分布清点 9 clip.*+单条族共 206 工具） | 已注册 |
| **框选范围→意图→split 全链**（REGION-INTENT-WIRE-1，已合入转正 pass） | conversation/intent.go 范围形态（两次 split 先终点后起点；8 新用例；rulings 2026-10-06 转正） | ✅ 主对话"把这段拆出来"全链通 |
| selected_clip_ranges 第一公民数据结构（全局+clip 内双时间界） | server.go:2171-2175 白名单/:5119-5128 校验（前产 A18） | 在 |
| 范围消费先例两条：strip_silence selected_ranges / semantic compressor | message_loop.go:1253-1281+catalog.go:473-479 / semantic_compressor_workflow.go:380-390（前产 A8-A10） | 在 |
| 前端 range 工具/cut 工具（同内核命令面） | vit_track_lane_2d.gd:2016-2024 / TimelineInputArbitrator.gd:660,847-874（前产 A6/A17） | 在 |

**缺**：

1. **时间限段 DSP 处理命令**（工程参数面）：VSP 命令表无任何带时间界的处理命令；范围只在渲染/探针面存在（render.start range:[s,e] TransportAudioService.cpp:1197-1205；l2_render_probe range :1268-1273，前产 A11/A12）。范围调改现状=物理拆（2a 路径，全链在位）或渲染面对比；"不拆 clip 的限段插件处理"需新内核命令（大改，决策项）。
2. note 辖区→主任务交接机制不存在（note_sessions.go:388-411 只读观察者，前产 A16）。
3. audition 面无时间界字段，但子段 A/B 可经 range 渲染→audio_file candidate 组合实现（零 audition 面改动，前产 §3.3）。

### 2.2 MIDI 操作面——**命令族全通（含 2026-10-06 注册补线）**

| 能力 | 锚点 | 状态 |
|---|---|---|
| 内核 MIDI 命令族：add_midi_notes(_bulk)/mutate_midi_notes/delete_midi_notes/get_midi_clip_notes/get_midi_clip_data/insert_midi_clip/create_midi_clip | CommandDispatcher.cpp:2609-2651 区段（前产 ARRANGE-RECON §1.1） | 在 |
| **import_midi_to_track + apply_midi_note_patch 注册补线**（MIDI-CMD-REGISTER-1，pass，main 9f53bd4e） | 真栈 run 20261006_201241 exit 0：import 落轨回读逐项一致+混合 op patch（transpose/delete/insert）回读一致 | ✅ 真栈验证 |
| agent midi.* 工具族 12 条（含 apply_midi_note_patch/read_notes） | catalog.go:883-891/:1003-1004（亲核部分） | 已注册 |
| 前端钢琴卷帘（实发 add_midi_notes_bulk/mutate_midi_notes） | PianoRollView.gd:801/:929,:1811（前产 §1.2） | 在 |
| NL→音符写轨（LLM 工具面直产 note patch，无需新执行链） | ARRANGE-RECON §5.1（前产） | 支持 |

**缺**：

1. **"听"面断点（编曲线最大体验缺口，决策项）**：乐器被 PCA 认证显式排除（processor_certification_entry.go:235）+ MIDI 路由仅 advice 不自动接线（VitZoneBufferAdapter.cpp:23-31）+ C2 动态装载硬编码 Effect/Z3（c2_dynamic_plugin_load.go:213）——**MIDI 写入后轨上无乐器则无声**。三选一需裁定：扩乐器认证/前端固定乐器链/接受静默写入+手动接线（前产冲突事实 1）。
2. 孤儿命令 trigger_note_on/off（前端发、内核零 handler，前产 §1.2 缺口 1，小收尾）。
3. MIDI 轨菜单禁用（实建只走 hybrid，前端域小改）。
4. AMT/OMR 集成（Basic Pitch 首选已勘察可对接，Apache-2.0+.mid 直出，前产 §1.4）——生成线候选，编曲线按需取用。

### 2.3 时间轴操作面——**标注/走带在，工程参数半缺**

| 能力 | 锚点 | 状态 |
|---|---|---|
| markers 读写五命令+五工具+goal 白名单+确认流 | 见 §一 S7 | ✅ 时间轴标注位完备 |
| tempo 写命令 set_tempo | CommandDispatcher.cpp:3439-3469（前产 §4.1） | 写在 |
| **tempo 披露键**：get_project_state 不含 tempo/timesig/key/bar——agent 面 tempo 事实=前端透传，MOM 回退分支空转 | CommandDispatcher.cpp:3279-3437（前产亲核） | ❌ 缺一小步（披露键） |
| 拍号（模板写死 4/4 无命令）/调性（工程级不存在）/小节（无 bar 换算） | 前产 §4.1 | ❌ 全链缺 |
| beat↔秒双向换算 | CommandDispatcher.cpp:1100,:1124-1126（前产） | 在 |
| 走带/播放头：play/stop/seek/return_to_zero/toggle_click | catalog 工具清点（本次） | 在 |
| 渲染时间窗：start_render range / l2_render_probe range | TransportAudioService.cpp:1197-1205/:1268-1273（前产） | 在 |
| **自动化曲线**：VSP 命令面零 automation 命令 | REGION-OP-RECON 2b（前产） | ❌ 全缺——PC 混音线 B2/C4 前置，编曲线不要重复立项 |
| arranger 命令面 | CommandDispatcher 全表无（前产 SEG-RECON G7） | ❌ 不可达路线（marker 为唯一承载） |
| 结构级编排复合操作（整段复制/移调一段/段落重排等） | — | ❌ 无现成复合工具，需组合原语（split/clone/move/transpose）或新复合卡 |

### 2.4 编曲操作面小结

- **可复用即开工**：clip 结构操作全链、范围框选→拆分全链、MIDI 全命令族、markers 读写+确认流、range 渲染——"拆一段/写音符/打段落标记"今天就能走通（普通确认制轨道，不撞 D1）。
- **缺的三类**：①结构输入（§一，第一张卡方向）；②"可听"底座（乐器接线+认证，决策项）；③工程参数面（tempo 披露键小改；拍号/调性/自动化大改，按需立项）。

---

## 三、与 PC 混音线的耦合面（并行安全性评估）

### 3.1 FREESTATE-REGION-IMPL-1（冻存卡）与 region/goal 面的交集

- **状态**：冻存待 B2 批次（2026-10-04 晚用户裁定：推迟至自动化曲线开发批，届时按**单轮形态**（写包络=单变更）直接做终态，本卡两轮制降级为设计输入；卡面明示"**不可在此批领取**"）。停工实现保全于 `coord/runs/FREESTATE-REGION-IMPL-1-STOP/`（未入库，B2 时评估复用骨架）。
- **内容与文件域**：loop 状态 range_goal 字段+两轮编舞（轮 1 split×2 断言 settle→轮 2 treatment）。设计锚点地图 D1-D7（`coord/runs/FREESTATE-REGION-GOAL-1/DESIGN.md`）：beginChatGoal（goalrunner_chat.go:32）、agentloop 上下文白名单（helpers.go:143-145）、runtime.Goal(:45)、轮状态机 free_state_reasoning_loop（goalrunner_chat.go:280-330+audio_closure_controller）、D1 prompt（ccb_model_prompt.go:131-133，**零改动红线**）。实现文件域=chat/goalrunner_chat.go+audio_closure_controller.go。
- **交集判定**：region goal 编舞在裁定后**归属 PC 混音线的自动化批次**（自动化=写包络是 B2/C4 的混音面能力；范围 goal 只是它的编舞外壳）。因此：**Mac 编曲线不应规划任何 range_goal/goal 编舞类工作**——那是 B2 激活时的 PC 文件域；Mac 线的范围类工作止步于已合入的意图层（INTENT-WIRE）与普通确认制工具面。
- **命名陷阱（与 MIX-CAP-RECON-1 卡 §3 待核点一致，双线共用）**：`docs/agent_action_workflow_v1_master_plan.md` §6.1 的 B2=静态主次平衡，自动化=文档 **C4**；用户 2026-10-04 裁定语境的"B2 自动化曲线批次"与文档 B2 同名不同义。排卡/排期时须双标注（用户 B2=文档 C4）防两线误读。

### 3.2 文件域冲突清单（Mac 编曲线 × PC 混音线）

| # | 冲突面 | Mac 线预计触碰 | PC 线预计触碰 | 风险 | 处置建议 |
|---|---|---|---|---|---|
| C1 | **agent/internal/harness/harness.go** | A-G1/G2 恰好要动：:144 观察特征清单+:3958 摄入分支+:4105 白名单+落盘行 | 同文件 mixboard 快照写入族（:6106-6202 区段）/ccb_masking_observation.go/c1 频清后台特征请求（mixObservationBackgroundFeatureKernelCommand :4387） | **高**——Mac 第一张卡就要动 PC 观察面的同一组清单与同一落盘文件 | A-G1/G2 独立小卡、函数域写死（isL3AcousticSummaryFeature+新分支+清单常量）；领取时 diff 归属核对；与 PC 线烟测腿错峰（§2.2 串行） |
| C2 | **agent/internal/chat/goalrunner_chat.go** | 编曲 goal 工具白名单若扩（:4323 区段同文件） | B2 激活时 range_goal 编舞（IMPL-1 冻存卡文件域主体） | **高**（仅 B2 激活窗口） | 冻结期无冲突；**B2 批次激活期间 Mac 线不领 goalrunner/audio_closure_controller 域卡**（发卡侧把关+卡面写互斥） |
| C3 | **agent/internal/agentloop/message_loop.go** | markers/section case 面+execution_memory 段落接线扩展 | strip_silence ranges/命令翻译层（:9509-9538）/semantic compressor 消费面 | 中——巨型文件多锚区，函数域可不相交 | 卡面写死函数/行区锚点；合并前 diff 归属核对（AGENTS §12） |
| C4 | **agent/internal/conversation/intent.go** | 编曲意图增补 | 混音意图增补（若有） | 中低——各增 case 分支 | 小步提交，冲突可文本合并 |
| C5 | **agent/internal/tools/catalog.go** | 编曲新工具注册 | 混音新工具注册 | 中——同文件 append 区 | 206 工具单文件，提交频次高+小步；必要时按行区拆 hunk |
| C6 | **agent/internal/agentprotocol/refschema.go** | 新段落族注册（A-G5） | 新回执族注册（M4/M5 先例仍在推进期） | 低——append-only 注册表+各自独立测试文件（refschema_m4_test.go 先例） | 各自新增行+测试文件分离命名 |
| C7 | **观察投影包** | 新段落 peer=新目录（零冲突）或改 epm/projection.go（A-G4 二选一） | MOM/COM/masking/acousticpackage（PC 观察主域） | 低（新包案）/中（改 EPM 案） | **建议新包**；若走 EPM 切换案，排期避开 PC 观察面改动窗并与主管裁定 |
| C8 | **agent/internal/capabilitycontext（CCB 视图）** | 若做 A-G6 段落 view | PC 主域（freeStateViewDefinitions/view catalog 扩容在 PC 线高频） | 中（仅当 Mac 做 view） | A-G6 建议不做（§一），冲突自然解除 |
| C9 | **VitApp/Source/Service/CommandDispatcher.cpp** | tempo 披露键/段落命令注册段 | 自动化命令（文档 C4）/混音命令注册段 | 中——同一注册区不同行 | 内核改动两线都走 port 分支+决策侧验收 cherry-pick，天然串行合入；冲突可文本合并 |
| C10 | **真栈占用** | Mac 栈（**尚无资源记录文件**） | PC 栈（coord/resources/PC-RUNTIME-STACK.md 在） | 无跨机冲突 | Mac 线开工首日建 `coord/resources/MAC-RUNTIME-STACK.md`（协议 §2.2）；PC 同机多卡烟测腿天然串行 |

### 3.3 并行安全性结论

**总体成立**：两线主力文件域不相交——Mac 编曲线主力面=internal/segmentation+新投影包+refschema+编曲意图/工具；PC 混音线主力面=chat 固定编排（c1/c2/b4/semantic_eq 族）+mixboard+MOM/COM/masking+PCA/认证。须排卡协调的只有三个窗口：**C1（harness 观察清单，Mac 第一卡就撞，需错峰+窄域）、C2（goalrunner，仅 B2 激活窗口）、C9（内核注册段，靠验收串行化解）**。烟测腿串行已由协议 §2.2 与 PC-RUNTIME-STACK 机制管住。

---

## 四、启动建议（若 Mac 选编曲——首批 2-3 卡）

**卡 1（第一张，卡面指定方向成立）：段落投影消费链打通·摄取段**——A-G1+A-G2：harness 摄入白名单扩容（isL3AcousticSummaryFeature 或平行分支加入 segmentation_primitives）+落盘行（l3_feature_log/mixboard_feature_snapshot 收录）+观察请求清单扩容（harness.go:144）。
- 域窄（harness.go 三处+测试）、真栈验收路径现成（复用 seg_primitives_smoke.ps1 加 agent 侧落盘断言即可）、且是 SEG-SMOKE/SEG-2 两次域外登记的既定欠账——**风险最低、共识最高**。
- 与 PC 线冲突=C1，窄域错峰即可。
- 可选拆分：若主管偏好更小粒度，1a（摄入+落盘）与 1b（请求清单）可拆，但同文件建议同卡一次收口。

**卡 2：段落→模型可寻址闭环**——A-G3+A-G5（+A-G4 的最小面）：消费接线（`internal/segmentation` 接上真实载荷：新段落 peer 投影**或** EPM 输入源切换——**此分叉是设计决策项，建议卡面挂 [等待主管裁定]**，两案文件域见 C7）+ ref schema 新族注册（模型可用 evidence ref 寻址段落分析工件）。
- 依赖卡 1；产出=模型上下文可见确定性分段（S1…Sn+置信+边界锚）而非仅 EPM 启发式，段落成为可引用证据。

**卡 3：编曲首功能·段落 marker 写入真栈腿**——SEG-2 的 `BuildApplySectionMarkersCommand` 命令构造已单测背书但从未上真栈；markers 命令面/工具面/确认流全在（S7）——补"真栈端到端：真实 stems 导入→primitives→切分→确认→apply_section_markers→markers.list 回读核对"烟测腿（复用 seg_primitives_smoke 体系），完成 L2-2 链的最后一环闭环（同时消解 A-G7 的前置观测）。
- 若主管希望首批出"用户可感"功能而非链路收口，替代候选=乐器自动接线（2.2 缺 1）——但那是决策项（三选一未裁定）+跨 PCA/C2 域，不宜作首批；建议维持链路收口优先。

**首批不碰清单**：region goal 编舞（C2/B2 冻存域）、D1 prompt 文案、CCB 视图主域（A-G6 暂缓）、自动化命令面（PC 线文档 C4 专属）、时间限段 DSP 新内核命令（大改决策项）、AMT/生成集成（生成线范畴）。

---

## 五、验收对照与边界声明

| 卡面验收 | 本报告 |
|---|---|
| 四节齐 | §一 段落投影消费链（S1-S7+缺口 A-G1..G7）/ §二 操作面（2.1-2.4）/ §三 耦合面（3.1-3.3，含文件域冲突清单 C1-C10）/ §四 启动建议（卡 1-3+不碰清单） |
| 锚点 | 全文行号锚；亲核/前产口径分标（报告头部口径声明） |
| 耦合面评估给文件域冲突清单（并行安全性输入） | §3.2 表 C1-C10+§3.3 三窗口结论 |

- 只读声明：零代码改动；工作树变化仅本报告与卡面回执（coord/ 下）。未跑真栈（本卡无烟测要求）。
- 前产锚点未逐条复跑（抽样亲核 10 处全命中，含 4 处 2026-09-27 老锚点当前仍准）；实现卡领取时应按惯例复核承重锚点。
- 并行流注记：勘察期间 GEN-CAP-RECON-1/MIX-CAP-RECON-1 两兄弟卡在同一主工作树并行领取（076ce106/f3ddbe2c/e79cdc97 三提交交错），本卡提交采用窄 pathspec/临时索引方式避免触碰其暂存——多执行流同机并行时主工作树 index 是共享踩踏面（协议 §3 已有 worktree 纪律，勘察类零代码卡目前实践上共栖主树，供主管知悉）。
