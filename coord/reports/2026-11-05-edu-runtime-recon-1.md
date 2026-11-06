# EDU-RUNTIME-RECON-1 勘察报告：乐局运行时面（关卡运行时挂载 / 前端复用 / 挂起定级）

- 卡：[2026-11-05-EDU-RUNTIME-RECON-1.md](../cards/todo/2026-11-05-EDU-RUNTIME-RECON-1.md)（池序 257，P1 / 0.5 天）
- 执行：ZCode 执行流（PC）/ 2026-11-05；只读勘察，零生产代码改动、零 LLM 调用、不占真实栈
- 领取基线：HEAD `b46a9096`（main）；工作树含其他卡未提交改动，未触碰
- 设计语境：[2026-11-05-EDU-DEMO-SCOPE-RULINGS-1.md](../rulings/2026-11-05-EDU-DEMO-SCOPE-RULINGS-1.md)（尤其 §五.8 问询坐标系 / §五.10 实例先行·游戏即教学 / §三 `agent/internal/edu/` 代码位置）

---

## 点 1：关卡运行时挂载面

### 1.1 现有消息流分层（勘察事实）

chat 主循环入口 `runAgentLoopChat`（agent/internal/chat/goalrunner_chat.go:130）从用户消息到 controller 运行分五层：

| 层 | 锚点 | 现状 |
|---|---|---|
| ① 确定性拦截面（语义入口之前） | goalrunner_chat.go:180-191 | 两个先例：mix workflow queue 读写（`handleMixWorkflowQueueChat`，mix_workflow_queue_view.go:690-725）与 send topology 请求。设计注释明言"run BEFORE semantic-entry classification: …must not depend on the classifier's mood" |
| ② 语义入口分类 | `planSemanticEntry`（semantic_entry.go:133），route 五值 discussion/observation/explicit_control/open_semantic/other（semantic_entry.go:23-28） | 模型分类，fail-open + 本地容量事实短路（goalrunner_chat.go:261-298） |
| ③ 容量路由 | `planObservationFirstCapabilityRoute`（capability_routing.go:269-327） | `assessFreeStateCapacity`（capability_routing.go:574-620）在 free_state / mixing_capability_layer 二选一（常量 capability_routing.go:25-26）；`validCapacityAssessment` 白名单锁死这两个值（capability_routing.go:894-904） |
| ④ controller 选择与独占 | orchestrationcontroller/types.go:18-23 四 Kind（ordinary_conversation / direct_typed_action / minimal_audio_closure / project_mix_workflow）；Registry 单 owner（types.go:141-164，Acquire 遇异 Kind active 直接拒绝 types.go:159）；`prepareProjectMixController` 冲突报"conversation is already owned by X"（orchestration_controller_host.go:85-88） | **ProjectMixWorkflow 先例：路由层已接入（capability_routing.go:292-294、445-447；restore 白名单 capability_routing.go:919）但运行时未注册**——`projectMixWorkflowV1Available` 恒 false（orchestration_controller_host.go:127-129），落 `projectMixUnavailableResponse` 停在能力边界 fail-closed（orchestration_controller_host.go:131-165） |
| ⑤ controller 准备与运行 | 三连 prepare：goalrunner_chat.go:306-317（`prepareProjectMixController` → `prepareAudioClosureContext`（audio_closure_controller.go:56）→ `prepareFreeStateReasoningContext`（free_state_reasoning_loop.go:304））；messageLoop.Start/Continue 三分支 goalrunner_chat.go:399-499；后处理链 goalrunner_chat.go:509-619 | free_state 触发条件 = controller==MinimalAudioClosure && route==open_semantic（free_state_reasoning_loop.go:285-302）；audio closure = MinimalAudioClosure 的闭包账本；乐局若做 controller 必须进此链 |

另有两层"非 controller"的会话级持久状态先例（乐局同型的最近参照物）：

- **freeStateLoop**：`storeFreeStateLoop` + `persistCurrentProjectWorkspace`（free_state_reasoning_loop.go:1142、380），workspace 快照持久化、会话级、跨轮存活——judgment park（待人耳 A/B 驻留态）就驻在它上面（judgment_park_continuation.go:31-40）。
- **mix workflow queue 节点标注**：确定性 chat 面 + executor 旁路工具（mix_workflow_queue_view.go:690 chat 面 / :743 `invokeMixWorkflowQueueTool`"chat-side projection, not a harness command"）；restore 走 fail-closed 白名单校验，未知节点/非法迁移丢弃计数（`restoreMixWorkflowQueueAnnotations`，mix_workflow_queue_view.go:670-681）。

### 1.2 结论：横切会话状态机最小而正；不做第五 controller Kind

**【2026-11-05 派发后补前提对齐（裁定 §五-16）】** 用户裁定乐局状态=**叠加引导框架层**，与能力域路由（free_state/mixing…）正交、可进出/挂起，非并列隔离模式。本勘察结论与该前提**同向且互证**：横切会话状态机正是"叠加引导框架层"的机制落地——乐局状态在既有路由之外旁路存活，节点/判定/引导是否在跑与"本轮由哪个 capability 接管"解耦；"非并列隔离模式"排除的恰是 controller Kind 路线（见下）。12 月只实现开局→通关单向，enter/exit/suspend 接口注意点见点 3 补节。

**结构性障碍检查：无。** 三类挂载面（确定性拦截 / 后处理钩子 / tools-policy 放行）均有现役先例。以下为方案取舍依据：

- **controller Kind 路线不取**：Registry 是**排他执行权**语义——一局乐局跨几十轮普通对话（学生问"为什么这样写"= discussion/observation；写好了 = 读取判定；示范写入 = explicit_control/open_semantic）。若乐局占 owner，激活期间全部轮次被 types.go:159 拒绝，与裁定 §五.10"游戏就是教学、教学就是游戏（无二分）"直接冲突，也与 §五-16"正交非隔离"相悖。ProjectMixWorkflow 先例恰好证明 controller=排他会话执行权，不是乐局的形状。
- **orchestration PlanningSession capability 层（capability_owner.go:17-104，B2/C1/C2 那族）不取**：那是专项混音能力的 EngineV1 owner policy + sticky 轮次选择（capability_owner.go:66-75），生命周期与乐局跨 controller 长驻不匹配，同属"并列隔离模式"。
- **取横切状态机（=叠加引导框架层）**：乐局状态=会话级持久纯数据状态（freeStateLoop 同型），不占 controllerOwners、不进 capabilityRoutes/restore 白名单；任意 controller 之下的轮次都能推进节点。乐局轮次照常走既有语义入口与 free_state/mixing 路由——与两个既有 capability 的共存方式=**正交叠加，零路由改动**。

### 1.3 挂载方案草案（文件清单 + 预计面，不写代码）

三个挂载点，合计对既有文件侵入 ≈ 12 行插点：

| # | 挂载点 | 文件 | 预计面 |
|---|---|---|---|
| 1 | **确定性拦截面**：入局/出局/乐局状态查询（"开始乐局""现在第几关"） | 新建 `agent/internal/chat/edu_jam_chat.go`；goalrunner_chat.go:180 段插 1 个调用点（与 `handleMixWorkflowQueueChat` 并排，同 `durableInvocationID == ""` 门） | 新文件 ~100 行；插点 ~5 行 |
| 2 | **判定器命令面**：done_when 判定器注册为模型可调用工具（裁定 §五.9 实现倾向"agent 工具目录命令（模型可调用、返回带证据判定）"） | `agent/internal/edu/judge.go`（判定器纯函数）+ `agent/internal/tools/catalog.go` 注册（RiskDirect，消费 get_midi_clip_notes 同族读取）+ policy.go 无需动（catalog spec 直接决定 risk，policy.go:104-120 `decisionForSpec`） | catalog 注册 ~5 行；判定器本体在 edu 包 |
| 3 | **节点推进检查**：判定返回后推进/拉回/局完成 | 拦截面内（形态 i，推荐）或 runAgentLoopChat 后处理段 goalrunner_chat.go:509-619 插 1 个检查点（形态 ii）；推荐 i：学生说"写好了"→ 拦截面跑读取命令 → 判定器 → 回话，零侵入主循环 | 形态 i ~40 行（edu_jam_chat.go 内）；形态 ii 插点 ~7 行 |

**edu 包本体**（`agent/internal/edu/`，裁定 §三 代码位置；答辩后并入 level 包）：

- `jam_state.go`：乐局状态类型——schema_version、conversation_id+project_uuid 双绑定、节点数组（node_id/state/attempt/entered_at）、事件追加表；纯数据，无 controller 引用（~200 行）
- `lesson_card.go`：课时卡加载+校验门（小节存在 / 检查方式有判定器，专家手写与 LLM 起草走同一函数，裁定 §一.4）
- `judge.go`：判定器注册表 + 首版 3 个判定器（note_sequence_match / pitch_set_match / rhythm_match），输入=读取命令返回的 notes 数组（纯数据），确定性零 LLM（~300 行）

**共存说明**：
- 不改 `assessFreeStateCapacity` / `validCapacityAssessment` / `restoreCapabilityRoutes`（capability_routing.go 三处白名单）——乐局不是 routing 候选。
- done_when 判定读取走 get_midi_clip_notes（RiskDirect，policy.go:39 + catalog.go:1148）零交互读，不与 active controller 的 mutation 写入竞争轮次；判定只在拦截面触发，不抢 free_state 实验轮。
- 持久化走 workspace 快照（storeXxx + persistCurrentProjectWorkspace 同型，free_state_reasoning_loop.go:1142 先例）；restore fail-closed（restoreMixWorkflowQueueAnnotations:670 模式）。
- 12 月课堂写入放行（若需把 add_midi_notes 的 RiskConfirm 在乐局上下文降级）：catalog spec 决定 risk 的机制不动，降级面在 chat 侧 confirm 边界（typed_apply_confirmation_guard 同域），属实现卡注意点，不是本勘察障碍。

---

## 点 2：前端复用面（D:/Godot/project/vit-daw-frontend，只读）

| 项 | 现状 | 锚点 | 结论 |
|---|---|---|---|
| marker 呈现 | 位置+range marker 全套：lane 绘制 + seek/add(位置/命名/range)/rename/delete signal 面 + 列表面板 | vit_dock/views/project_marker_lane.gd:3-9（signal 面）、:48-50（set_markers）；project_marker_list_panel.gd | **现成**。agent 侧 `project.markers.upsert` 支持 range marker（tools/catalog.go:1011，裁定 §二已录），写入即呈现。问询参照物"自写 marker/自设 A-B 段"（裁定 §五.8）两端闭环 |
| 循环播放 | 时间选区循环，transport 按钮+tooltip | app/shell/top_transport_bar.gd:324（"Loop：使用当前时间选区循环播放"）、:74/:318-324 | **现成**。"自设循环区"参照物的呈现/控制面无需新做 |
| A-B 呈现 | 两端现役：Godot 侧 vitnote timeline affordance 声明含"audition A/B 区间试听"（observe 面）；webui 侧 audition 卡/判定卡/盲听面板全套 | app/tracks/vitnote/vit_face_supplier_timeline.gd:37-53；agent/webui/src/audition.ts、src/trajectory/TrajectoryAuditionPanel.tsx | **现成**。12 月演示（前端=钢琴卷帘 + agent 对话面）无需新 A-B 组件 |
| 学生卷帘编辑→agent 读取 | **同源数据面闭环**：前端卷帘 IPC 命令族与 agent 读面同族（add/mutate/delete/get_midi_clip_notes/get_midi_clip_data）；内核返回 clip 全量音符 | app/piano_roll/piano_roll_midi_rules.gd:5-13（IPC_DEBUG_COMMANDS 同族）；VitApp/Source/Service/MidiService.cpp:1685-1734（handleGetMidiClipNotes 遍历 seq.getNotes() 全量返回）；midiNoteToVar MidiService.cpp:310-320 | **零新增通路**。字段=id/start/length/pitch/velocity/channel（beats 坐标）——完整覆盖"学生写了什么"，音符带稳定 id 可寻址（问询坐标可锚定）；裁定 §五.9 符号分析族的输入即此数组 |
| 音符着色 | **一句话：可行且已有双态先例**——per-note ColorRect 按 note_state 分派 predicted（品红半透明）/solid（绿、velocity 明度渐变），教学标注态（如错音高亮）加第三态走同型分支即可 | app/piano_roll/view/PianoRollView.gd:49-51（两色导出）、:135（NOTE_STATE_PREDICTED）、:1389-1410（`_apply_note_polymorphic_visual` 分派 + `_recompute_solid_note_base_color`） | 有余力项，机制现成 |

---

## 点 3：park/挂起定级

**一句定级**：挂起非演示必需（P3 / 答辩后）——乐局状态做成会话级持久纯数据状态（freeStateLoop 同型）后，"挂起"自然退化为"状态留在 workspace 快照里、用户回来接着说"（judgment park Part A 默认采纳同型语义，judgment_park_continuation.go:48-131 先例），12 月零 park 代码。

**状态机接口注意点（不封死并入 level 包与 park/continuation 复用）**：
1. 判定器输入=纯数据（读取命令结果），不含 conversation/goal/controller 引用；推进=纯函数 `(state, observation) → state'`——park 恢复与 level 包复用都不用改判定核心。
2. 状态绑定 conversation_id+project_uuid 双 ID，但**不绑 controller Kind/owner**（不进 orchestrationcontroller 枚举、不进 restoreCapabilityRoutes 白名单）——答辩后若要显式挂起，再以 durable continuation 的 PendingInteraction 形态接入（DurableContinuation，continuation_scheduler.go:209；pendingInteractionFromResult:343 现成）。
3. schema_version + restore fail-closed 校验（未知节点/非法迁移丢弃计数，mix_workflow_queue_view.go:670 模式）；旧状态缺省语义先写进卡（AGENTS §11 持久化兼容红线）。

**【enter/exit/suspend 接口注意点（裁定 §五-16 要求；12 月只实现开局→通关单向，切换只定接口不做实现）】**

- **enter（开局）**：唯一 12 月实现面。形态=拦截面确定性命令（"开始乐局《小星星》"）或课时卡注入动作；enter 输入=课时卡内容包实例（裁定 §五-16 乐局游戏=可实例化内容包；无内容 enter=即兴乐局，归产品线不在 12 月）。接口注意：enter 不动 controllerOwners、不改路由——只创建 jam state 行；同一会话已有活跃 jam state 时 enter 幂等拒绝（同 mix queue 标注重写的 fail-closed 姿态）。
- **exit（出局/局终状态退出）**：接口注意点=exit 必须与 controller settle 语义解耦——乐局没有 owner 可 settle，exit 只做三件事：jam state 置终态（含完成/中止原因）、投影一条终局面（复用 trajectory 事件零新增协议）、能力沉淀钩子位（裁定 §五-16"学习→自主创作：符号分析自由态照常认模进"——即 exit 后普通路由照旧，唯一要求是 exit 不留任何阻断面）。12 月局终即 exit，单向不回头。
- **suspend/resume（挂起/恢复）**：12 月零实现；接口留法=suspend 不做任何主动动作（状态天然留在 workspace 快照），resume=下次轮次 prepare 时读到非终态 jam state 即视为 resume（拦截面照常应答"现在第几关"）。将来显式 park 若需要（如跨会话移交），接入点是 DurableContinuation PendingInteraction——届时 suspend=arm pending interaction + jam state 标 suspended，resume=claim 恢复；这两个动作全部在 chat 侧，edu 包状态类型无需预留任何挂起字段，只需 `status` 枚举里留 `suspended` 一个值（枚举扩展属持久化兼容旧值缺省语义，随卡声明）。
- **共通红线**：三个接口都不得触碰 free_state/mixing 的路由白名单与 owner 注册表——乐局进出挂起对能力域路由完全不可见（正交的机制含义），这是并入 level 包时接口不变式。

---

## 构建块估时与格式修订意见

### 构建块 3（关卡运行时）：估 ~1.5-2 天，建议拆 2 卡

- 卡 A（edu 包）：jam_state + judge 注册表 + 首版 3 判定器 + lesson_card 校验门 + 单测 —— 0.5-1 天
- 卡 B（挂载）：chat 拦截面 + goalrunner 插点 + workspace 持久化/restore + 单测 + 真栈烟测腿（扩展现有冒测脚本、HTTP chat 驱动一局最小流程，AGENTS §5 门槛）—— 各 0.5 天

### 构建块 2（乐局卡格式定稿）：修订意见 3 条

1. **判定器枚举要小且前置**：done_when.judge 只允许注册表内名字（首版 3 个），校验门直接拒绝未知判定器——专家写不出"不存在的检查方式"。
2. **小节坐标显式化**：卡内显式给 bpm+小节起拍映射，判定器不做小节推断（裁定 §五.8 坐标系规则："第几小节"的可靠性不归判定器承担）；音符坐标统一 beats（与 midiNoteToVar 返回一致）。
3. **写入放行边界写进卡**：课时卡声明本关允许的写入命令族；RiskConfirm 命令在乐局上下文是否降级为可直接执行，是独立的产品裁定，不藏在卡格式里。

---

## 勘衡量

- 阅读面：chat 包 4 个核心文件全文（capability_routing.go 1067 行、policy.go 175 行、semantic_entry.go 458 行、orchestration_controller_host.go 196 行）+ goalrunner_chat.go 主流程段（130-620）+ audio_closure_controller.go/free_state_reasoning_loop.go/mix_workflow_queue_view.go/judgment_park_continuation.go 关键段 + orchestrationcontroller/types.go 全文 + exit_retain.go 头段 + tools/catalog.go 相关条目 + agentloop 目录面；内核 MidiService.cpp 读取链（1685-1734、310-320）；Godot 前端 6 个文件（marker lane/transport bar/piano roll rules/pitch lane/grid canvas/view/face supplier timeline）。
- 未展开：continuation_scheduler.go 全文（只核对了类型与入口函数签名）；agentloop message_loop preflights 全文（mix queue in-loop carrier 的细节）；webui audition 组件内部（只核对存在性与职责）。均为挂载实现卡开工时的阅读项，不影响本报告结论。
- 上交决策侧事项：无结构性障碍。两点提请：①乐局不做第五 controller Kind 是本勘察的设计建议（依据=排他 owner 语义与裁定 §五.10 冲突），请主管在发实现卡时确认；②12 月课堂写入放行的降级边界（RiskConfirm→direct 在乐局上下文）需产品裁定，建议随构建块 2 格式定稿一并裁定。
