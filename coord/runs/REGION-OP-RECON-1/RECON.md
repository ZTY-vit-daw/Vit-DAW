# REGION-OP-RECON-1 · RECON：「范围即操作目标」能力盘点（只读勘察）

- 执行侧会话，2026-10-04；基线 08031102（领取 commit 832c28d7/d2862b59）
- 约束遵守声明：纯只读——零代码改动、零探针写源码树、未触碰 sealed fixture、未触碰工作树预存改动（VitApp/Workspace 两处 XML）与其他 runs 目录。唯一产出为本文件 + 卡片状态变更。
- 路径建议均为**建议**，不代决——设计卡与分期由决策侧/用户拍板。

---

## 总览（一句话版）

**"框选 clip 局部→拆出来→单独调改"的管线骨架已经全部在位**：内核有 `clip.split`（带子 clip id 回传），前端有 range 工具框选（`selected_clip_ranges` 第一公民数据结构，含全局+clip 内双时间界），agent 工具面已注册且主对话上下文已有 `selected_clip_ranges` 键。真正的缺口只有三个：①主对话意图层不消费 ranges（split 切点目前只来自口述时间/播放头）；②范围界定的 **DSP 处理**命令不存在（处理命令全是无时间界的，范围只在渲染/探针面与 strip_silence 结构操作面存在）；③note 辖区与主流双向隔离，无"升级为主任务"通道。治理上，拆分+调改走普通确认制轨道不撞 D1；若纳入自由态实验轨道则同轮两 mutation 违规，需两轮制或拆分定性裁定。

---

## 问 1：内核 clip 拆分能力——**有，五点全链在位**

### 1.1 VSP 命令表（锚点：`VitApp/Source/Service/VspKernelReference.cpp:111`）

```cpp
{ "clip.split", { "split_clip", true } },   // true = mutation
```

同族命令：`clip.move`(109)、`clip.resize`(110)、`clip.remove`(112)、`clip.fade.set/read`(113-114)、`clip.gain.set/read`(115-116)。

### 1.2 内核编辑面实现（锚点：`VitApp/Source/Service/ClipService.cpp:1496-1577`，split 处理器）

- 参数：`clip_id` 必填；切点 `split_time_seconds`（beats 单位下接受 `split_time_beats`/`split_time`/`time` 别名，:1521-1527）。
- 校验：切点必须落在 clip 内部且两侧留存活音频（:1545-1548），最小存活长度 `kMinimumSurvivingClipLengthSeconds = 0.01` 秒（:29）——**极窄范围（<1cm 拖选等效）会被拒**。
- 执行：tracktion engine `clipTrack->splitClip(*clip, te::TimePosition::fromSeconds(splitTimeSeconds))`（:1553），undo 事务名 "Split clip"（:1550），拆后恢复边界 fade（`restoreSplitBoundaryFades`，:1554）。
- **响应闭环**（:1566-1576）：回传 `left_clip_id`/`right_clip_id`/`new_clip_id`（=right）+ 左右各自的 `start_seconds`/`length_seconds`——agent 拆完即可直接拿到子 clip id 继续调改，无需二次查询。

### 1.3 agent 工具面（已注册）

- `agent/internal/tools/catalog.go:987`：`spec("split_clip", "clip.split", "clip", "Split a clip at a timeline position.", RiskConfirm, true, true, true, true, "track_id", "clip_id")`——LLM agent 循环可直接调用，RiskConfirm 确认制。
- `agent/internal/chat/goalrunner_chat.go:4117-4124`：`agentLoopClipTools()` 白名单含 `clip.split`（同族 gain/fade/strip_silence 全在）。

### 1.4 主对话快速意图（已存在，见缺口）

- `agent/internal/conversation/intent.go:93-112`：切分话语 → `clip.split`，`split_time` 取口述秒数（`firstLocalSeconds`）或播放头（`mentionsPlayheadText`）。
- **缺口**：`selectedClipArgs`（intent.go:248-259）只取 `selected_clip_ids`/`piano_roll_focus_clip_id`/`selected_clip_id` 等 **clip id 键**，不取 `selected_clip_ranges`——快速意图的 split 切点目前与框选范围**无关**。

### 1.5 前端路径（"选/移/裁/画"编辑）

- "裁"：`D:/Godot/project/vit-daw-frontend/app/clips/clip_drag_command_rules.gd:88-95`——`clip_edge_left/right` 手势 → `resize_clip` 命令；拖拽重叠默认 `trim` 模式（:4）。
- "画/切"：`app/timeline/TimelineInputArbitrator.gd:660 → 847-874`——cut 工具手势 `_request_split_clip_from_cut_tool` → `clip_client.split_clip(payload)`；前端客户端 `app/kernel/clients/clip_client.gd:24-25`。
- **复用结论**：前端与 agent 走**同一内核命令面**（clip.split），复用不是问题，问题只在意图接线（1.4）。

---

## 问 2：局部处理路径三选——可行性结论

### 2a 物理拆（拆出子 clip → 对子 clip 单独处理）——**可行性最高，全链在位**

- 拆：问 1 全链；范围框选的 `start_seconds`/`end_seconds`（问 4 的 range 工具产物）就是现成切点（两次 split 或一次 split+resize）。
- 调：子 clip 独立调改沿用现有 per-clip 命令——`clip.gain.set`（静态增益，catalog.go:996 附近 spec 区）、`clip.fade.set`、`clip.move` 等，均按 `clip_id` 作用。
- 成本/注意：物理拆改变工程结构（undo 事务、结构 revision 变化）；边界 fade 由内核自动恢复（ClipService.cpp:1554）；用户需求字面是"只作用于这个范围"，拆分是手段不是目的——若调改可表达为 clip 级参数（增益/fade），拆分路径完全成立；若调改是插件参数，见 2b。

### 2b 范围界定处理（不拆 clip，处理带时间界）——**agent 消费面先例充分，内核 DSP 执行面缺失**

**已有先例（范围作为操作目标的两条现成管线）**：
1. **strip_silence**：`agent/internal/agentloop/message_loop.go:1253-1281`——`scope=selected_ranges` + `selected_clip_ranges` 直传工具；工具面 `agent/internal/tools/catalog.go:473-479`——`ranges:[{clip_id,track_id,start_seconds,end_seconds}]` + `scope: selected_clip|selected_ranges|all_project`，apply 侧 `strip_regions`（:477）。
2. **semantic compressor**：`agent/internal/chat/semantic_compressor_workflow.go:248,380-390,467`——消费 `selected_clip_ranges`/`time_selection`/`selected_clip_time_range`（`semanticCompressorActiveSecondsRange` 判定范围活跃）。

**内核 DSP 面现状**：
- 命令表（VspKernelReference.cpp:90-146）**无任何时间限段 DSP 处理命令**——处理类全是无时间界的（`plugin.set_parameter`、`track.volume`、`macro.*` 等）。
- **渲染/探针面有时间窗**：`render.start` 接受 `range:[startSec,endSec]`（`VitApp/Source/Service/TransportAudioService.cpp:1197-1205`，`startOfflineRender(..., startSec, endSec, ...)`）；`l2_render_probe` 同样接受 range（TransportAudioService.cpp:1268-1273 附近 `readCommandRange`，无显式 range 时回落 clip 编辑时间范围）。
- tracktion engine 内核本身有 automation 概念（`CommandDispatcher.cpp:1077` detached clip tree remap 提及 automation），但 **VSP 命令面未暴露任何 automation 写入命令**。

**结论**：不拆 clip 的"时间限段处理"目前只能在**渲染/对比证据面**实现（range 渲染两版本对比），不能在**工程参数面**实现（让某插件只作用于某时间窗需要新内核命令，属大改）。若调改是 clip 级参数，2a 与 2b 等价（增益本来就是 clip 上的）；若是插件级参数且要不影响范围外，则当前唯一工程面路径是 2a 物理拆。

### 2c 前端侧区域处理（L2-2 SEG 的 marker 树/SegmentationPrimitives 衔接）——**分析/标注位，非执行位**

- 链路：内核 `SegmentationPrimitives` payload（feature_type `segmentation_primitives`，SEG-1）→ Go 侧 `agent/internal/segmentation/segmentation.go`（:16-18 消费声明）→ 构建 `project.markers.apply_section_markers` 命令（segmentation.go:331-351，source-filtered replace）→ 内核 `ProjectMarkerService.cpp:356` `handleApplySectionMarkers`（`CommandDispatcher.cpp:2390` 分发；命令表 VspKernelReference.cpp:92 `project.markers.apply_section_markers`）。
- **消费面缺口**：marker 写入后，观察投影层（TOM/DOM/MOM）grep 无直接消费锚点——marker 目前活在内核 project state 里，不直接成为任何处理命令的操作目标。
- **定位**：SEG/marker 线适合当"agent 理解范围语义"的**输入**（段落时间界→建议切点/建议范围），不是执行路径本体；若要让它进执行，需新增"marker 时间界 → 处理参数"的接线（本质还是 2b 的缺口）。

---

## 问 3：治理适配——D1 单轮规则与 audition 形态

### 3.1 D1"单轮一次前向变更"规则锚点

- `agent/internal/agentloop/ccb_model_prompt.go:131`（多轮实验面）：每轮**恰好一次前向 mutation**；同轮内禁第二次 treatment、禁 `continue_once`；action 应用后只允许 retain/rollback/ambiguous human judgment/blocked。
- `ccb_model_prompt.go:133`（D1-S1）：一轮实验 + 一次前向 mutation；post_action_evaluation 里唯一合法输出是 settle 报告或 blocked 边界。

### 3.2 范围操作（拆分+调改两步）的相容形态

范围操作 = 拆分（结构 mutation）+ 调改（处理 mutation）= 两次前向变更。两条治理轨道：

- **普通对话轨道（用户确认制）——不撞 D1**：`clip.split` 与调改命令都是 RiskConfirm（catalog.go:987 等），用户逐条确认执行，不进自由态实验循环。两步两确认即合规。**这是现状即可支撑的形态**。
- **自由态实验轨道（A/B 寻优）——同轮两 mutation 违反 D1-S1/多轮面规则**。相容形态三选（需决策侧裁定，不代决）：
  1. **两轮制**：轮 1 拆分 + 取证 settle；轮 2 调改 + 取证 settle。规则无需改动，流程多一轮。
  2. **拆分定性为准备操作**：裁定"结构拆分不是 treatment mutation"（类比实验的样品制备），prompt 规则明文化（ccb_model_prompt.go 131/133 文案处）。改动小但属治理语义变更，须用户拍板。
  3. **前置拆分**：拆分发生在实验轮之前（用户/主对话先拆好），实验轮只对已存在的子 clip 做单次 treatment——最干净，但把拆分时机交给实验外。

### 3.3 A/B 试听在子段上的形态——audition 会话面不感知时间界，但可经范围渲染接入

- `VitApp/Source/Service/AuditionPreviewState.h`：`Candidate` 只有 `durationSeconds`（无 start/end 界字段）；`TransportAnchor` 只有 `positionSeconds`——**audition Session/StateMachine 无时间界概念**。
- 但 `Candidate.sourceKind` 要求为 `audio_file`（`AuditionPreviewAudioPlane.cpp:339`："real audio spike requires source_kind=audio_file"）——**文件即候选**。
- **子段 A/B 形态**：2b 的 `render.start range:[s,e]` 渲染出范围音频文件 → 作为两个 candidate 的 source 进 `audition.prepare` → `audition.select`/`audition.position` 试听（命令表 VspKernelReference.cpp:133-143）。audition 面无需加时间界字段；非试听的对比证据可用 `l2_render_probe` 的 range（同窗口同路由对比）。此形态与既有 FXM A/B 精神一致，属组合而非新能力。

---

## 问 4：辖区 → 主任务交接

### 4.1 note 侧现状（只读观察者，与主流双向隔离）

- `agent/internal/chat/server.go:180-181`：`Note *NoteChatPayload` 非 nil 即进入 note 会话模式——会话键=note_id/run 戳、**不走治理链、不写工程单图**。
- `agent/internal/chat/note_sessions.go:388-411`：`buildNoteAssembly` 注入 `note_jurisdiction` 快照（faces：face_id/face_kind/label/selection_share/face_coverage/domain + resolved domain entries：tracks/clips/**time windows**/plugins）；系统 prompt 定位为 "note observer... answer questions asked from a small sticky note"（:410-411）。
- REGION-TIME-1（done 卡 `coord/cards/done/2026-10-03-REGION-TIME-1.md`）正在给辖区载荷 v3.1 加时间维度：每 clip 条目 `range_clip_start`/`range_clip_end` + 顶层 `range_time_span`。
- **note 与主流无交接机制**：note 回答留在 note 会话库，主流 goal 看不到。

### 4.2 主流侧现状——**"范围→操作目标"的现成通道已存在**

1. 前端 range 工具（**框选 clip 局部是第一公民功能**）：`vit_track_lane_2d.gd:2016-2024`（`_finish_clip_range_gesture`，range 工具在 clip 上拖拽，时间钳制到 clip 内、≥0.01s）→ `TimelineEditStore.add_selected_clip_range`（`TimelineEditStore.gd:276-284`）。
2. 规范化数据结构（`TimelineEditStore.gd` `_normalize_clip_range`）：`range_id/clip_id/track_id/start_seconds/end_seconds/duration_seconds/clip_start_seconds/clip_end_seconds/clip_local_start_seconds/clip_local_end_seconds/source`——**全局时间界 + clip 内局部时间界双份**。
3. 流入 agent 主对话：chat context `current_selection.selected_clip_ranges`（`server.go:2171-2175` 白名单、:5119-5128 结构校验；`agentloop/helpers.go:143-145` 上下文键表白名单含 `selected_clip_ranges`/`playhead_seconds` 等）。
4. 已有消费面：message_loop strip_silence 路径（message_loop.go:1268）+ semantic compressor 工作流（semantic_compressor_workflow.go:387）。系统 prompt 已教 LLM "selected clip" 语义（server.go:5758）。

### 4.3 缺口与两形态

- **形态一：主流直接框选指令（现成度高，缺意图接线）**——用户 range 工具框选 → 主流说"把框选的这段拆出来单独降 3dB"。缺口：意图层 `selectedClipArgs`（intent.go:248-259）不取 ranges；split 快速意图的切点来自口述/播放头（intent.go:95-99）而非框选。接线 = 意图层新增 ranges→切点/参数映射（小改，先例是 strip_silence 的 message_loop.go:1268 接法）。
- **形态二：载荷传递（note→主流，需新建机制）**——note 识别结果（含 REGION-TIME-1 时间界）→ 显式"升级为主任务"动作 → 主流 goal 携带范围参数。`goal_text` 通道在位（`mix_session_workflow.go:140,192-193` 等贯穿），但"note 会话 → 主流 goal"的跨会话传递动作目前不存在，属新机制（UI 动作 + 载荷约定）。**不代决**：形态一可先行，形态二等 note 侧使用成熟后再立项。

---

## 三选路径结论（汇总）与建议分期（建议，不代决）

| 路径 | 可行性 | 一句话结论 |
|---|---|---|
| (a) 物理拆子 clip | **高（全链在位）** | split+子 clip 调改命令面齐全，缺的只是意图层消费 ranges |
| (b) 不拆、范围界定处理 | 中 | agent 消费先例充分（strip_silence/semantic compressor）；但内核 DSP 处理命令无时间界——范围处理只能在渲染/对比面，工程参数面需新内核命令（大改） |
| (c) SEG/marker 区域处理 | 低（现位） | SEG→marker 是分析标注位，投影层无执行消费；作范围语义输入合适 |

**建议分期**（供决策侧出设计卡）：
1. **近期小改**：主对话意图层接线 ranges（切点=range start/end，调改对象=拆出的子 clip 或 range 界定的 clip 参数）；范围操作走普通确认制轨道（RiskConfirm 两步确认），不触碰 D1。
2. **中期组合**：子段 A/B 经 `render.start range` + `audition.prepare(audio_file candidate)` 组合实现（零 audition 面改动）；strip_silence 的 `selected_ranges` 模式作为范围工具族的模板扩展到其他结构操作。
3. **远期决策项**：自由态实验轨道内的范围操作（两轮制 vs 拆分定性为准备操作——用户裁定）；"不拆 clip 的时间限段插件处理"若确有需求，属新内核命令面立项；SEG marker 线接入执行（marker 时间界→处理参数）另评估。

## 锚点索引（回查用）

| # | 锚点 | 内容 |
|---|---|---|
| A1 | `VitApp/Source/Service/VspKernelReference.cpp:111` | 命令表 `clip.split`（另 109-116 clip 族、90-92 markers 族、133-143 audition 族） |
| A2 | `VitApp/Source/Service/ClipService.cpp:1496-1577` | split_clip 实现+校验+子 clip id 回传；:29 最小存活 0.01s |
| A3 | `agent/internal/tools/catalog.go:987` | split_clip 工具注册（RiskConfirm） |
| A4 | `agent/internal/chat/goalrunner_chat.go:4117-4124` | agentLoopClipTools 白名单 |
| A5 | `agent/internal/conversation/intent.go:93-112,248-259` | split 快速意图；selectedClipArgs 只取 clip id |
| A6 | `vit-daw-frontend/app/timeline/TimelineInputArbitrator.gd:660,847-874` | 前端 cut 工具→split_clip |
| A7 | `vit-daw-frontend/app/clips/clip_drag_command_rules.gd:88-95` | 前端裁边手势→resize_clip |
| A8 | `agent/internal/agentloop/message_loop.go:1253-1281` | strip_silence selected_ranges 消费 |
| A9 | `agent/internal/tools/catalog.go:473-479` | strip_silence ranges/scope 参数面 |
| A10 | `agent/internal/chat/semantic_compressor_workflow.go:248,380-390,467` | 语义压缩器消费 selected_clip_ranges/time_selection |
| A11 | `VitApp/Source/Service/TransportAudioService.cpp:1197-1205` | render.start range:[s,e] 时间窗 |
| A12 | `VitApp/Source/Service/TransportAudioService.cpp:1226-1273` | l2_render_probe range（回落 clip 范围） |
| A13 | `agent/internal/segmentation/segmentation.go:16-18,331-351` + `VitApp/Source/Service/ProjectMarkerService.cpp:356` | SEG→apply_section_markers 线 |
| A14 | `agent/internal/agentloop/ccb_model_prompt.go:131,133` | D1-S1 单轮一次前向 mutation 规则 |
| A15 | `VitApp/Source/Service/AuditionPreviewState.h` + `AuditionPreviewAudioPlane.cpp:339` | audition 无时间界；candidate source=audio_file |
| A16 | `agent/internal/chat/server.go:180-181,2286-2289` + `note_sessions.go:388-411` | note 载荷分叉/只读观察者/辖区快照 |
| A17 | `vit-daw-frontend/app/tracks/track_scene/vit_track_lane_2d.gd:2016-2024` + `TimelineEditStore.gd:_normalize_clip_range` | range 工具框选→selected_clip_ranges（双时间界） |
| A18 | `agent/internal/chat/server.go:2171-2175,5119-5128,5758` + `agentloop/helpers.go:143-145` | selected_clip_ranges 进主对话上下文与键白名单 |
