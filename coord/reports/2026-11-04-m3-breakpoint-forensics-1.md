# M3-BREAKPOINT-FORENSICS-1 勘察报告：组 1 首绿三层断点（终局驱逐 / misroute 软预问 / continuation 前台吞）

- 执行：GLM-5.3 Flash（ZCode PC 执行会话 2026-11-04）；worktree `D:\Vit_DAW_wt_244work`（分支 `port/m3-breakpoint-forensics-1`，基线 06cdc6cc=领取提交）
- 依据：卡面 `coord/cards/doing/2026-11-04-M3-BREAKPOINT-FORENSICS-1.md` + 裁定 `coord/rulings/2026-11-04-M3-WRITE-GATE-TRIGGER-CLARIFY-1-pass.md`
- 红线遵守：**零代码改动**（本分支 diff 仅本报告）；**零 LLM 调用**（全部结论来自既有 writegate1 工件+源码静读）
- 载荷坐标口径：`coord/runs/M3-WRITE-GATE-TRIGGER-CLARIFY-1/writegate1/logs/agent_llm_payload_capture.jsonl` 共 168 条记录；分组 r1=记录 2–44（43 条）、r2=45–128（84 条）、r3=129–162（34 条），其余为 setup/smoke。卡面 r1[42]/r2[83]/r3[33]=轮内 0 基索引=各轮末条=本报告记录 44/128/162。消息结构恒为 3 条：`msg[0]` system（17670 B）+`msg[1]` gate 短消息（164/213 B）+`msg[2]` 大上下文 user（~40 KB，节区每轮重组）。@offset 指 msg[2] 内字节偏移。

---

## 面一：终局轮上下文驱逐机制（基础设施面，主攻）

### 1.1 载荷对照表（needle 亲扫，逐记录）

| 轮 | 记录范围 | 披露通道 | 披露在场记录 | 终局轮（末条）披露 | 终局轮申报 |
|---|---|---|---|---|---|
| r1 | 2–44 | **ledger `automation_surfaces`**（含 plugin_id 1011/Volume/0.740818/范围 0–1） | 7–43（36 条连续） | **消失**（记录 44 三节全无披露） | capability_blocked `missing_resolved_parameter_identity_and_time_span` |
| r2 | 45–128 | **单槽 `recent_goal_context.recent_observation`**（get_plugin_parameters 回执恰停在槽内；ledger 全轮无 automation_surfaces——r2 整轮即 continuation 模式） | 49–128（仅记录 77 单条缺口=该请求连 recent_observation 键都不存在，装配分支未逐点定位） | **在场**（记录 128 @6097 `0.740818`、@6297 `1011`，走槽不在 ledger） | needs_experiment + 软预问（面二） |
| r3 | 129–162 | ledger `automation_surfaces` | 135–161 | **消失**（记录 162 三节全无披露；snapshot=track.time_dynamics+transient 无参数） | needs_experiment `needs_resolution`（缺可写标识+取值范围） |

needle 计数：v2 句（"do not pre-ask in text"）在 157/161 条 llm_route 载荷在场（缺席=三条 2896 B 引导请求记录 2/45/129+记录 77）；v1 旧句 "before any write" 0 命中——与裁定 157/0 完全吻合，**三轮终局轮 v2 在场亲证成立**（记录 44/128/162 的 msg[0]+msg[2] 双消息均携带）。

终局轮 ledger 换面对照（同轮内相邻记录 diff，`Free-state reasoning ledger JSON` 单行整体替换）：

- r1 记录 43（fresh 模式 ledger 键集）：`active_intent, automation_surfaces, cycle, decision_phase, loop_id, max_cycles, original_intent, plugin_candidate_disclosure, requires_post_action_observation, schema_version, status, target_ref`
- r1 记录 44（终局=continuation 模式键集）：上表去掉 `automation_surfaces`，加入 `continuation_budget:6, continuation_used:1, latest_decision, observation_ids, priority_queue`；同请求追加指令行 `TERMINAL TURN (mechanical runtime state): closure rounds remaining: 4/6; continuations remaining: 5/6. This is the final turn: return a final decision now — ...`；`Remaining tool calls this run: 71→80`（终局片拿满新工具预算）。
- r3 161→162 同型；r2 全轮为 continuation 模式（127→128 仅 `continuation_used:3→4` + 预算警告句→TERMINAL TURN 替换）。

### 1.2 因果链（触发→裁剪→终局轮缺披露，可直接拆修复卡）

1. **钉住写入**：`plugin.get_parameters` 成功 → `recordFreeStateAutomationSurface`（`agent/internal/agentloop/free_state_reasoning.go:2231-2258`）把行写入 loop 上下文的**动态键** `loop["automation_surfaces"]`（:2250）+ `loop["observation_ledger"]["automation_surfaces"]` 镜像（:2251-2256）。行带 `project_revision`，读时按 revision 严格前进过期（`freeStateExpireAutomationSurfaceRows` :2347-2358；revision 源 :2366-2378）。writegate1 全程 project_revision=3 无 mutation，过期不触发。
2. **run 内注入**：每轮 prompt 组装经 `messageLoopFreeStatePromptContext`（`free_state_reasoning.go:393-415`），`automation_surfaces` 条件注入（:413-415），读顶层键（`freeStateAutomationSurfaceRowsForState` :2383-2392）。run 内无宿主重入→钉住全程在场（记录 7–43 亲证）——**241（M3-AUTOMATION-SURFACE-PIN-1）的 run 内修复有效**。
3. **run 边界（=驱逐点）**：切片 limit_reached → 宿主边界导入 `res.Continuation.Context` / `res.ContextSnapshot` 的 loop（`agent/internal/chat/free_state_reasoning_loop.go:1199-1210`）→ `freeStateLoopFromAny`（:3400-3413）**`json.Unmarshal` 进 typed struct `freeStateReasoningLoop`（:69-215）——该 struct 无 `AutomationSurfaces` 字段，顶层动态键被静默丢弃**；`mergeFreeStateLoops`（:449-540）逐字段 typed merge 无处安放；续跑请求上下文重建=`freeStateLoopMap(loop)`（:3415-3420，typed 序列化；注入点 :442/:2648/:2757/:2980）。**agentloop 侧导出快照本身保留该键**（`agent/internal/agentloop/helpers.go:32-54` buildContextSnapshot `Context: cloneMap(state.input.Context)`）——丢弃点在 chat 宿主 typed round-trip，非 agentloop。
4. **终局触发**：closure 边界 settle 兜底给受保护观察环预留终局轮——`settleTaskAtAudioClosureBoundary`（`chat/audio_closure_controller.go:915-934`，锁定 reason=`budget_critical`；r1 exchange0 持久化快照亲证 `terminal_turn_locked=True, terminal_turn_reason=budget_critical, stop_reason=limit_reached, goal_status=waiting_continue`）；另有 checkpoint 记账路径 `evaluateFreeStateTerminalTurnTrigger`（`chat/free_state_reasoning_loop.go:3136-3153`）与 `goalrunner_chat.go:3361-3381`（`ContinuationUsed++` 于 durable checkpoint 边界 + LastCheckpoint 兜底锁）。
5. **终局轮渲染**：续跑 run 组装时 ledger=continuation 模式（无 automation_surfaces）+ `TERMINAL TURN` 指令（`agent/internal/agentloop/ccb_model_prompt.go:420-444`；句常量 `freeStateTerminalTurnSentence` `free_state_reasoning.go:792`）要求当场交最终决定→可见面内字面无 1011/0.740818→r1/r3 终局申报缺标识**非模型矛盾，是载体丢弃**（裁定修正②的机制层落地）。

**触发条件结论：不是字节数阈值**。writegate1 三轮上下文尺寸恒定（msg[2] ~40 KB，节区每轮重组而非追加裁剪）。字节机器独立存在且未触发：hot 10 KB/warm 5 KB/full 24 KB（`contextruntime/model_projection.go:18-20`），超预算按确定性顺序把**非阻塞** warm 节区降级为 cold ref（`enforceModelLayerBudgets` :174-230——r2 载荷 snapshot 里 `observation_ledger` 即带 `cold_data:"audit_snapshot_only","degraded":true` 标记；阻塞节区 active_observation/current_selection/recent_goal_context 永不降级 :171-173）；真正超限走 fail-closed（`ModelContextOverflow` :583+，`message_loop.go:513-520` 拒发）。**驱逐=宿主 run 边界 typed round-trip 的键丢失，相位触发，与字节预算正交。**

### 1.3 与 241 的同族性裁定

- 同族成立（家族=「有界可见面丢失正面发现」），**机制不同**：241 修的是单槽 `recent_observation` 逐出（240 forensics：单槽被后继观察顶掉）；本面发现的是**钉住载体自身在宿主 run 边界被 typed round-trip 丢弃**——241 的钉住被证明 run 内有效（记录 7–43），但跨边界不存活。
- r2 的「全要素持有」是**单槽时序运气**：终局轮前最后一次工具回执恰好是 get_plugin_parameters（26 次读取的末次），恰好停在永不降级的阻塞节区（`runner.go:589/610/638` 单槽覆写 → `contextruntime/context.go:128` summarizeRecentGoalContext 组装 recent_goal_context）——不是钉住起效。

### 1.4 修复方向分级

- **T1（立即接线，最小侵入）**：给 `freeStateReasoningLoop` struct 增 `AutomationSurfaces []map[string]any \`json:"automation_surfaces,omitempty"\`` 字段 + `mergeFreeStateLoops` 增对应合并分支（沿用行合并+revision 过期语义，上限沿用 `freeStateAutomationSurfaceLedgerLimit=8`）。验尸 needle：终局轮请求 ledger JSON 含 1011/0.740818。备选（不推荐单独做）：投影改读 observation_ledger 镜像——镜像自身过边界的存活未逐点定位（见 1.5），struct 字段是确定性修复。
- **T2（挂证据触发器）**：跨 run 的回执内容累积面（按 track+plugin 钉住的回执册）与 241 卡 T2 memory 触发器同题，不进当前排期（用户 2026-11-03 裁定沿用）。
- **措辞迭代（模型面，附属于本面）**：终局轮换面后若披露仍在（T1 后），TERMINAL TURN 句无需改动即自洽；无需新措辞。

### 1.5 未定位（如实标注）

1. `observation_ledger` 镜像的丢失点：载荷证明 checkpoint 持久化 loop 的 observation_ledger 仅剩 `[available_views, receipt_count, receipts, schema_version, view_observation_count]`（r1 exchange0 亲证），但宿主 ledger merge（`mergeFreeStateLedgers` :634-667、`mergeFreeStateObservationLedger` :2089-2118）均为 clone 式保留未知键，typed round-trip 也应保留 generic map 键——候选=边界导入的 continuation/snapshot 上下文携带钉住前的旧面（`:455` 注释「continuation 携带切片启动时的输入快照」）或 agentloop 导出侧另有重建。未逐点定位，不充当锚点。
2. 锁定发生时 vs 渲染时的计数差：r1 终局轮渲染 `4/6, 5/6` 但锁定 reason=budget_critical——锁来自 closure 边界 settle 兜底（:932），锁定瞬间的计数不在载荷可观测范围。因果链闭合，计数源未逐轮对账。
3. 终局片工具预算 80 的常量来源未查（机制=新 run 满预算，`message_loop.go:3852-3857` 渲染）。
4. 记录 77 整请求形状不同（2 消息、无 recent_observation、无 v2）的具体装配分支未逐点定位（单条，不影响结论）。

---

## 面二：misroute+软预问残留（模型面）

### 2.1 r2[83] 实发形态（载荷亲证，记录 128 response）

`final:true, needs_experiment, evidence_status:plausible` + `improvement_proposal.v1`（`action_domain:"section_automation"`, `action_kind:"automation_envelope_write"`, `parameter_bounds:{plugin_id:"1011", param_id:"Volume", points 5 点 0.741→0.98, region 0–8s}`）+ reply 含条件式软预问 **「如果你的片段并不从 0 秒开始，请在确认卡里核对起止时间，确认后才会真正写入」** + limitations「当前证据窗口未披露该片段的精确起止时间」。**当前值被同轮直接消费**（reply「目前音量停在 +0.00 dB（约 0.74）」=披露在场且被读）。

### 2.2 竞争引导清单（r2[83] 可见面全量盘点，@offset=msg[2] 内字节偏移）

| # | 可见面文本（载荷锚点） | 代码锚点 | 与 v2 的竞争方向 |
|---|---|---|---|
| 1 | TERMINAL TURN 句：「return a final decision now — **a bounded improvement proposal**, or an explicit terminal statement of where you are blocked」（@34758）——终局二选里**没有「直接调用」选项** | `free_state_reasoning.go:792`；`ccb_model_prompt.go:420-444` | 终局轮形态被指令锁定为 proposal-or-blocked |
| 2 | 窗口预算句：「The evidence you hold is sufficient for a bounded proposal. Use needs_action only when…; when it supports only a bounded improvement hypothesis, **use needs_experiment with one bounded improvement_proposal.v1**」（@34418/34535） | `ccb_model_prompt.go:396` | 终局阶段框架=提案时刻 |
| 3 | 目标级证据工作流规则：「Once the target-level evidence for your chosen candidate is in hand, **return final=true needs_experiment with one bounded improvement_proposal.v1 citing it**」（@23672）——与 v2「时变目标→直接调用」**正面对撞** | 载荷节区（per-turn directives/契约段） | 两条工作流级指令对时变目标给出相反终局动作 |
| 4 | 开放改进契约枚举：「After observation, return needs_experiment …, no_candidate_found …, or capability_blocked …」（@21395）——观察后动作枚举**无自动化直调分支** | 同上 | 三选枚举缺第四形态 |
| 5 | 认识论注记：「Partial or bounded evidence … is sufficient for needs_experiment」「You are NOT required to prove an objective defect before proposing」（@25690-25905） | msg[2] Important Epistemic Notes（组装见 `message_loop.go:3808`/`:3857`） | plausible 即提案的激励 |
| 6 | **格式示例邻接缺陷**：v2「Action Path Selection」段三条之后**唯一**的 `final:true` JSON 示例是 `needs_experiment`+静态域提案（track_gain\|static_eq\|broadband_compression）（@36627-36815）——文字说「直接调用」，紧邻的形态示例说「needs_experiment」 | `ccb_model_prompt.go:130`（v2 文字源）与示例块同段 | 形态示例对输出形状的约束力强于散文指引；无任何「final=true + automation.set_points tool_call」示例 |
| 7 | D1-S1 admitted 域表：9 个静态域枚举（@37612+）**无任何 automation/envelope 域**——时变提案在 admitted 词表内无合法形态，r2 自创 `section_automation/automation_envelope_write` | 同上 | 提案路线对时变目标是「词表外 improvisation」，直调路线才是骨架正道，但可见面未把两者连起来 |

补充定性：骨架明文「a static action_domain proposal is the wrong instrument for it（时变目标）」（@35302/35645）——r2 严格说未违此句字面（其 domain 非静态域），但 needs_experiment+improvement_proposal **正是**该句所指的「静态提案器械」，violated 在器械层而非词表层。

### 2.3 软预问的判定缺口（时间不确定该走 needs_observation 的缺口描述）

1. **「持有 time range」无判据**：v2 直调条件「once you hold every write element (target track, parameter, in-range values, **time range**)」（@35927）未定义 time range 的「持有」是证据背书还是计划假设；r2 的 region 0–8s 是计划假设（limitations 自证「未披露精确起止时间」），模型在「算持有/不算持有」之间无指引，走了中间形态=提案+预问。
2. **不确定情形无出口**：缺 time range 的合法出口应是 needs_observation（取时间证据；r3 后来正是从 track.time_dynamics 能量证据推出 0–6s——证据可取），但可见面把「不确定」导向提案路径的合法槽：`needs_resolution:["<optional missing typed detail>"]`（@36803）+limitations——**提案路线自带「带缺口提案」的合法槽，直调路线没有**，两槽并存诱导混合形态（带 region 假设的提案+文本预问）。
3. **双确认语义邻接无桥**：提案世界有真实用户确认门（msg[0] @11405「The needs_experiment user-confirmation gate is where any mutation is authorized」）——r2 的「请在确认卡里核对…确认后才会真正写入」是对该门的正确预判；直调世界说「do not pre-ask in text」因为运行时门自动拦卡。两个世界的确认语义在可见面相邻，**无一句桥接「时变目标走直调后由运行时拦卡，确认卡里自然可核对时间，无需文本预问」**——软预问是跨方案语义渗漏，非凭空。
4. 预算句附压：「Every needs_observation turn you return consumes one checkpoint, and exhausting it settles the task at the observation boundary without ever exercising an improvement」（@21948，代码 `ccb_model_prompt.go:648`）——把转向 needs_observation 描述为有代价且可能「永不实验」，进一步劝退「再观察一次拿时间」的选项。

### 2.4 修复方向分级（不写新措辞，供主管裁决）

- **措辞迭代类**（T1.5，速效）：①终局二选句补第三形态（全要素在手→直接 set_points 调用）；②v2 段落邻接补一个「final:true + automation.set_points tool_call」的时变形态示例；③确认语义桥接句（时变目标经运行时拦卡进确认卡，无需文本预问）。均为句级，落点 `free_state_reasoning.go:792`/`ccb_model_prompt.go:130` 邻接示例块，五面 parity 锁链按 243 先例同步。
- **设计裁决类**（主管定）：时变目标是否设 admitted 提案域（如 automation_envelope_write 入词表并给参数 schema），还是维持「时变=永远直调、提案词表不含」并把 #3 目标级规则句加时变豁免。此为裁决项非措辞项。
- **T2**：needs_resolution 槽与直调条件的互斥语义（提案槽不得承载可观察补齐的缺口）挂证据触发器。

---

## 面三：durable continuation 前台吞（基础设施面）

### 3.1 生命周期（三轮同构，载荷亲证）

三轮 exchanges 完全同构（`writegate1/rounds/g1_r{1,2,3}.json`）：

1. 用户首句 → 切片 run（载荷 r1=3..43 / r2=46..127 / r3=130..161）→ `stop_reason=limit_reached, goal_status=waiting_continue`，回执「我还在继续处理这个任务，完成后再向你汇报。」+ `workflow_data.checkpoint_persisted=true, checkpoint_status=waiting_continue, continuation_id=cont_*`（r1 exchange0 持久化 loop 已 `terminal_turn_locked=true, reason=budget_critical, continuation_used=1/budget=6`）。
2. 用户「继续」→ **吞**：`chat/goalrunner_chat.go:194-204`（`isContinueMessage` + `automaticContinuationForConversation` 命中 pending → 回执「任务已在自动续跑队列中；这条消息不会创建新任务或重复执行。」，`stop_reason=automatic_continuation_already_scheduled`，`workflow_data:{attempt:0, status:"pending"}`）。相邻分支 `:206-216` 为交互挂起另一形态（`pending_interaction_requires_response`「单独说'继续'不会越过这个交互边界」）。
3. 终局 run 后台执行（各轮载荷末条）→ capability_blocked/needs_experiment 终态落 goal 状态。**三轮均无第三次 exchange**——终局结果在 chat 面零渲染，只存在于 goal 状态/历史，裁定「能力面零暴露」在载荷层成立。
4. checkpoint 记账在 durable resume 边界：`goalrunner_chat.go:3359-3381`（`ContinuationUsed++`+终局锁评估+超预算 settle `free_state_continuation_budget_exhausted`）；budget 耗尽兜底 `chat/free_state_reasoning_loop.go:2933-2952`。

**smoke 逐轮新对话 × continuation 存活窗口的交互**：三轮用三个新 conversation_id（`llm_route_g1_r{1,2,3}_*`），loop/goal 均按 conversation 键（`s.freeStateLoops[conversationID]`）——跨轮零继承（r2 的 continuation_used 从自身 0 起算到 4，非 r1 的延续）。前台吞是**轮内**现象：同 conversation 内「继续」被 pending 判定吞；跨轮新对话则完全新起 loop。r1 的终局裁决（缺标识）对 r2/r3 不可见——三轮同型重演的机制基础。

### 3.2 回执跨轮投影可见性（80 次观察为何持续不满足）

- **观察量**（载荷逐条统计 tool_calls）：r1=observation_request×40（view：activity_structure×34 为主）+get_parameters×4+get_curve×3；r2=observation_request×80（**track.time_dynamics×78**）+get_parameters×26+get_curve×11；r3=observation_request×31（time_dynamics×17+project.structure×11）+get_parameters×12+get_curve×6。裁定口径 needs_observation 决策 40/80/33 为决策轮计数，与工具调用计数（40/80/31）同量不同面。
- **回执可见性结论：单槽+latest-only，不累积**。每工具覆写单槽（`agentloop/runner.go:589/610/638` → `state.recentObservation`），下一请求仅组装 latest（`contextruntime/context.go:128` `summarizeRecentGoalContext`）；该节区是阻塞节区永不降级（`model_projection.go:171-173`），但**前序回执内容不随行**；observation_ledger 节区（含 receipts 册）在模型面被预算降级为 cold ref（r2 载荷亲证 `cold_data:"audit_snapshot_only"`，机制 `model_projection.go:174-230`），ledger 里只余 `observation_ids`（ID 列表，非内容）。**即：模型每轮只看得到最近一份回执，历史回执内容全部不可见。**
- **持续不满足机制**：模型 needs_observation 申报缺「该轨片段起止时间+参数范围」（r2 载荷 ledger latest_decision @7405 亲证原文）；它连打 track.time_dynamics×78 试图取时间证据，但每轮只见到 latest 一份 digest（视图 digest 截断带 `<truncated>`），无法跨轮比对/累积；r3 才从 time_dynamics 能量证据推出 0–6s（其终局 summary 亲证）。投影层存在 `clip_start_seconds/duration_seconds` 紧凑字段（`internal/mom/projection.go:895`）——能力字段在投影层存在，模型面 digest 的可见性/截断是窄点（未逐轮核验，见 3.4）。
- **可见/机械错位（附发现）**：预算句宣称「每 needs_observation 轮耗一个 checkpoint」（@21948），机械记账实际在 durable resume 边界（`goalrunner_chat.go:3361`）——r2 80 次观察仅耗 4 checkpoint 为机械证明。可见面夸大观察代价，与面二 2.3-4 同向（劝退观察）。

### 3.3 修复方向分级

- **T1（立即接线）**：吞回执升级——`automatic_continuation_already_scheduled` 回执携带 pending continuation 的当前状态+最近一次终局/观察回执摘要（pendingmanager 已有 attempt/status，`goalrunner_chat.go:198` 处可附），并把终局结果补投递到 chat 面（goal 完成时对原 conversation 回执一条终态摘要）。零协议改动，回复负载内接线。
- **T2（挂证据触发器）**：跨轮回执累积面——与面一 T1 同载体（struct 字段化后 observation_receipts 摘要面可按 target 钉住）；及 projection digest 的 clip_start_seconds/duration_seconds 可见性核验后按需扩 needle。
- **措辞迭代**：预算句「every needs_observation turn consumes one checkpoint」改为与机械记账一致的表述（resume 边界计费）。

### 3.4 未定位（如实标注）

1. track.time_dynamics 的模型面 digest 是否携带 clip 起止字段：载荷内 digest 均截断（`<truncated>`），投影层字段存在（`mom/projection.go:895`）但模型实际可见子集未逐轮核验。
2. 终局结果是否存在其他已投递面（webui/goal 面有渲染而 rounds JSON 未记录）——本卡只证 chat exchanges 面零渲染，其他面未查。

---

## 验收对照

- 三面各：锚点（file:line）+载荷/日志证据+分级 ✓（面一 §1.2/1.4；面二 §2.2/2.3/2.4；面三 §3.1/3.2/3.3）
- 终局驱逐面因果链可直接拆修复卡 ✓（§1.2 五步：钉住写入→run 内注入→宿主边界 typed round-trip 丢弃→终局触发→终局渲染缺披露；T1=struct 字段+merge 分支）
- 零代码改动：本分支 `git diff --stat` 仅本报告文件 ✓；零 LLM 调用（纯既有工件+源码静读）✓
- 与卡面前提冲突处：无推翻性冲突；一处精确化——「241 同族」成立但机制非单槽逐出而是宿主 typed round-trip 键丢失（§1.3），与裁定修正②的定性兼容且落地到代码层。

## 关键锚点速查（供拆卡引用）

| 断点 | 首选锚点 |
|---|---|
| 钉住写入（run 内有效） | `agentloop/free_state_reasoning.go:2231-2258`（写），`:393-415`（注入），`:2383-2392`（读） |
| 驱逐点（typed round-trip） | `chat/free_state_reasoning_loop.go:3400-3413`（fromAny），`:69-215`（struct 无字段），`:449+`（merge），`:442`（续跑上下文） |
| agentloop 导出保留键 | `agentloop/helpers.go:32-54` |
| 终局触发 | `chat/audio_closure_controller.go:915-934`；`chat/free_state_reasoning_loop.go:3136-3168`；`chat/goalrunner_chat.go:3359-3381` |
| 终局指令 | `agentloop/free_state_reasoning.go:792`；`agentloop/ccb_model_prompt.go:420-444`（渲染），`:367-398`（预算句），`:130`（v2 文字） |
| 前台吞 | `chat/goalrunner_chat.go:194-204`（吞），`:3359-3381`（checkpoint 记账） |
| 单槽回执 | `agentloop/runner.go:589/610/638`；`contextruntime/context.go:128` |
| 字节预算（正交面） | `contextruntime/model_projection.go:18-20`（常量），`:174-230`（降级），`agentloop/message_loop.go:510-520`（fail-closed） |
| 快照组装 | `agentloop/message_loop.go:3852-3857`（snapshot 行），`:3808`（ledger 行） |
