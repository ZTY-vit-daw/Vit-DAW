# 146 SEMANTIC-TIMEFX-BUS-CONVENTION-1 取证腿收尾报告

- 执行：GLM-5.3 L2 执行会话（PC/ZCode，本会话）；基线=358b3de7（领取推送 3c4176b7）；2026-10-20 晚窗
- 输入：M2 体验场次①实录（`coord/reports/2026-10-20-m2-experience-session-1.md`）+ 源码（358b3de7 干净 worktree）+ 当晚运行工件（主检出 Logs/ 与会话图、运行时状态）
- 本卡不改生产代码；本报告为取证结论，设计定稿与实验方案见同日设计稿

## TL;DR

1. **F4 读法需修正**：模糊意图第二轮不是"提案被准入门驳回"——**模型自己申报了 capability_blocked**（理由=人声轨身份未披露，拒绝改错轨），admission receipt `boundary=proposal_missing`、`proposal_present=false`（**根本没有提案**）。用户看到的「本次有界改进提案未通过自由态准入门（G3…G8）」是回复面构造器 `freeStateCapabilityBlockedReply`（`internal/chat/goalrunner_chat.go:2098-2107`）在 boundary=proposal_missing 时仍优先拼 failed_gate_ids 造成的**事实错误文案**——它同时压掉了模型 summary 里可操作的阻塞原因（「工程信息没有披露人声轨…唯一可见轨道是 bass(id 1007)」）。
2. **门语义不过严**：G 门审计的是空循环（本 turn 零观察+新循环 ledger 零结转），G3/G5/G6/G7/G8 悬空是设计行为；G1/G2/G4 通过（G4 走会话级 closure 已闭合诊断维度，`free_state_gate.go:259-267`）证明门在正常工作。问题全在门上游。
3. **路由在 G 门之前且已正常工作**：server 侧语义入口分类（`semantic_entry.go`）+ 容量评估在模型 turn 前完成，「给人声加点空间感」被正确放行进 open_semantic 自由态循环（loop created_at=18:22:24.6，original_intent 原文在卷）。断裂点在**路由之后、提案之前**：观察行为与证据结转。
4. **零层持有装载位置决策**：plugin_recommendation 链所有 track 引用是 `requestContext.selected_track_id` 直通投影（`plugin_recommendation.go:125/286/528/568/814/843`）——"inline 于 UI 选中轨"是链上**唯一可表达**的形态，不是任何一层的决策。总线拓扑在当前语义链中不可表达。
5. **例外面与内核命令面均就绪**：inline 例外清单可落地（§5）；`track.send.*` 五命令+建轨+装载足够表达 bus 拓扑（K2 工作流已在用）——停止条件两条均未触发。

## ① 准入门拒绝根因分层（待证 1 收尾）

### 证据链（全部工件可回指）

| # | 事实 | 锚点 |
|---|---|---|
| E1 | 第二轮复用同一 goal（goal_98d6813ec6100767），非新 goal | `agent_last.log:492-495` |
| E2 | turn10 一次 LLM 调用 8.8s，messages=3（system 14331 字符 + 新话术 131 + 历史 blob 41308），**零工具调用** | `agent_last.log:492`；遥测末条（prompt_fingerprint=75e91e8d…） |
| E3 | ledger boundary：`stored_receipts=0`、`ccb_observations=1 ids=[obs_20261020T102049…]`、`snapshot_loop=true continuation_loop=false`——第一轮三次观察**未结转**进第二轮新循环 | `agent_last.log:493` |
| E4 | 模型申报：status=**capability_blocked**，summary=「工程信息始终没有披露人声轨身份…唯一可见且可指定的轨道是 bass（id 1007）…在人声轨身份未确认前，无法确定短衰减空间处理应该施加到哪一轨」；semantic_processor_intent 拒绝码=**target_identity_not_disclosed**；limitations 三条（含「目标轨确认前不做任何改动」） | 会话 `agent_runtime_state.json` → `free_state_reasoning_loops[webui_mvgj5jdz].latest_decision` |
| E5 | admission receipt：`boundary=proposal_missing`、`proposal_present=false`、`proposal_valid=false`、failed_gate_ids=[G3,G5,G6,G7,G8]、gate_results G1/G2/G4=pass | 同上 → `.admission_receipt`（recorded_at=18:22:33.78） |
| E6 | 用户收到=门代号文案（「本次有界改进提案未通过自由态准入门（G3_project_scan…G8_target_consistency）…」），非模型 summary；reply_len=267 与该文案吻合 | 会话图 node[9]；`agent_last.log:498` |
| E7 | boundary 分层逻辑存在且正确区分 proposal_missing / proposal_invalid / admission_gate_failed / capability_boundary | `internal/chat/free_state_reasoning_loop.go:732-744`（recordFreeStateAdmissionReceipt） |
| E8 | 回复面构造器**不看 boundary**，failed_gate_ids 一律优先于 decision.Summary | `internal/chat/goalrunner_chat.go:2098-2107`（freeStateCapabilityBlockedReply） |

### 分层结论

- **L0 模型行为（直接原因）**：turn10 零观察。系统提示词明确指示「If no track identity is visible yet, request a project view that discloses it」（`internal/agentloop/ccb_model_prompt.go:669`）——模型没有执行（把新话术当已完结目标的延续，turn10 输入带着 41k 历史 blob）。它随后**正确地**拒绝在身份不明的目标上动手：这是诚实边界行为，不是能力失败。
- **L1 系统缺口（上游成因，三个）**：①新循环 ledger 零结转——循环合并只对 active 循环生效（`free_state_reasoning_loop.go:265-270`），第一轮 18:21:45 已 completed/budget_stop 终止，第二轮新循环从空 ledger 开始；②循环启动无强制观察步——模型可以不观察直接结算；③模型上下文中轨道清单不可见（容量评估 observed_facts 只有 track_count=6 计数，`capability_routes.task_d4b27231f7d9406c.capacity_assessment`）——轨道身份须靠本轮 ccb 观察（project.structure 等 view）获取，模型没做。
- **L2 门语义**：**不过严**。门审计空循环五门悬空是按设计工作（G3 要求 delivered scan receipt、G5 要求 frontier candidates、G6 要求目标级证据、G7 要求绑定当前 revision 的 fresh refs、G8 要求提案-目标一致——`internal/agentloop/free_state_gate.go:186-285/312-530/599-623`）。若有提案被拒，receipt 会记 boundary=admission_gate_failed；本例无提案。**「门语义过严」假设被证据否定**。
- **L3 回复面缺陷（新 finding，建议上交）**：`freeStateCapabilityBlockedReply` 在 boundary=proposal_missing（模型自报边界、无提案）时仍输出「提案未通过准入门」——事实错误 + 压掉模型 summary。修复方向：按 E7 的 boundary 分层选文案（proposal_missing/invalid → decision.Summary 分支；admission_gate_failed → 门代号文案）。FS-CAPABILITY-BLOCKED-SURFACE-1 只修了"拒绝包成成功答"，这是同族的反向面。**与 147/148/149 同链，建议并入 147 族或单独小卡，不属本卡修复域**。
- **取证缺口如实记录**：模型 turn10 的完整原文（是否含 prose reply 字段）未在任何工件中找到——遥测无文本、message_loop_debug 无 18:22 条目（该窗口仅 4 条 free_state_final_gate，均属第一轮 18:20-18:21）、会话图只落了门文案节点。latest_decision.summary（E4）是模型申报内容的全部可见部分。

## ② 语义意图路由与 G 门评估时序（注入点核心问题）

**答案：路由在前，且已工作；G 门在后；惯例不能也不应挂在入口分类器上。**

当前管线（锚点）：

```
用户文本
 → [模型 turn 之前·server 侧] 语义入口分类（internal/chat/semantic_entry.go，LLM 分类器
   semantic_entry_decision.v1：route/control_mode/user_authorization/target_scope，12s 总预算 fail-open）
   + 容量评估/能力路由（capability_routing.go；product runtime 确定性选择也在场——
   第一轮 reason="product runtime selected free_state after project structure observation"）
 → open_semantic + semantic_loop + action ⇒ 启动自由态循环
   （free_state_reasoning_loop.go:235-252 shouldStart；:254-316 prepare——新循环 ledger 为空）
 → 模型 turn（agentloop；观察=模型自发 ccb.observation_*；ccb_model_prompt.go 组装系统提示词）
 → 模型申报 FreeStateDecision
 → server：recordFreeStateDecision → recordFreeStateAdmissionReceipt（G1-G8 审计+boundary 分层）
 → 响应面 settledTurnFaceResponse（goalrunner_chat.go:2125-2136）
   → capabilityBlockedBoundaryResponse（:2143+；缺陷见 L3）
 → 过门提案 → plugin_recommendation 链（选插件卡→rack.add_node→typed controller→参数写入）
```

关键事实：

1. **入口分类器契约是内容盲的**（`semantic_entry.go:96-102` 系统提示词原文）：禁止输出/推荐任何 processor family、插件、观察 view、控制轴、参数；「Family selection, if later needed, belongs to a separate model turn after model-selected observation」；「The product runtime observes project structure and owns that decision after this turn」。→ **拓扑惯例注入点不得放入口分类器**（与既有契约直接冲突），应放观察后的族选择/治疗规划层。
2. **第二轮路由成功**：loop created_at=18:22:24.623（用户消息 18:22:24 后 ~0.6s），original_intent=「给人声加点空间感」，authority_mode=full_access——模糊意图被正确分类进 open_semantic 语义循环。**「模糊意图到不了语义链」不成立；到不了的是提案（观察断层）**。
3. **G 门在模型申报之后求值**（`free_state_gate.go:599` ← `free_state_reasoning.go:710/1114` 与 `free_state_reasoning_loop.go:684/693`）——它从不拦截路由，只在结算时审计。真正的 needs_experiment/improvement_proposal 被门拒时有独立计数/锁终端机制（`free_state_reasoning.go:699-734` advisory rule 2/3：同指纹不再入环、第二次 G 拒锁终端）——本例未触发（无提案）。

**对注入点设计的直接推论**（详见设计稿）：惯例注入三件套=①循环系统提示词软声明（`ccb_model_prompt.go`）+②提案/装载链的拓扑计划对象（`plugin_recommendation.go` 提案组装处）+③执行写门校验；前置依赖=回复面 boundary 修复（L3）与新循环首轮观察/结转（L1）——后两者不修，惯例只在"模型自觉观察"场景生效。

## ③ 装载位置决策层归属（待证 2 收尾）

**结论：当前没有任何一层持有装载位置/拓扑决策；"inline"是 selected_track_id 的默认投影，非决策产物。**

| 层 | 实际职责 | 拓扑概念 |
|---|---|---|
| `chat/semantic_treatment_strategy.go` | 治疗/族/参数规划（planSemanticTreatment，:645）——**签名吃 trackID**，被喂目标轨 | 无。已钉事实 2 成立：文中 `Topology` 字段是**插件参数拓扑**（generic_eq_topology，:37/63/353），非信号路由拓扑；`SendCommand` 是 Go 客户端方法名。同词异义，特此注明 |
| `chat/plugin_recommendation.go` | 插件推荐/选择交互卡/装载确认/rack.add_node/typed controller | 无。所有 track 引用=`firstStringFromMap(requestContext, "selected_track_id", "selected_plugin_track_id")` 直通（:125/286/528/568/814/843）——**UI 选中轨是唯一可表达装载位置**；「当前轨道」卡面文案（F3①）即此直通的面 |
| `semanticorchestrator/orchestrator.go` | 处理器意图→适配器→控制绑定的渐进披露状态机（AcceptIntent→SelectAdapter→…→SetPhysicalTarget→BindControlRefs→Confirm，:143-261） | 无。SetPhysicalTarget 是披露**阶段**名（控制面/参数绑定），非路由拓扑 |
| `planner/planner.go` | 已装载插件的参数操作提示（explain_controls/apply_eq_edits/set_plugin_param，:215） | 无 |
| 模型 | 第一轮推理「在 bass 轨上形成一层薄薄的空间感」（会话图 node[2]） | 模型可提议目标，但链上落笔仍是 selected_track_id 直通 |

**设计推论**：总线程的断裂精确化为——topology 在提案→装载链上**不可表达**。要给"加混响=上 FX 轨"一个落笔面，必须新增拓扑计划对象并由装载链消费（见设计稿 §2）。

## ④ inline 例外场景清单（待证 3）

以下场景 inline 是正确形态，默认总线规则不适用（或被更高优先级路径接管）：

1. **用户明说插入/inline**：「直接插在这条轨上」「就加在这轨不要建轨」——显式指示优先于惯例（含语义入口 target_scope=current_selection 的单轨选定场景）。
2. **用户显式具名 send 结构（≥2 源轨）**：走 K2 确定性工作流（`send_route_workflow.go:31-52`，`len(sources)<2` 不拦）——显式结构请求是硬面，不属默认惯例的软路由范围。
3. **单轨快速 A/B / 试听探测**：即建即删临时轨模式（M2-TIMEFX 线既有实践）；audition 盲测（VIT_DAW_AUDITION_BLIND 面）在探测轨上 inline 合法——拓扑对即删对象无意义。
4. **实验探测轨**（M2 认证装载）：同上，生命周期=即建即删。
5. **执行细节（非例外但须写进规则）**：工程已有可用时域 FX/return 轨时，惯例应**复用**既有轨（C3 观察面已能读 send/return 结构，`c3_space_depth_runtime.go` 载体 4=track.send.list）而非重复新建；单轨工程不构成例外（send/return 拓扑在母线侧依然成立）。

## 停止条件核对

- 「惯例已部分存在」：**未触发**——③ 确认零层持有拓扑决策（比已钉事实 2 更强：连 implicit 决策都没有，是 context 直通）。
- 「需要内核新命令」：**未触发**——track.send.* 五命令+建轨+装载+宏绑定（K1/K2/K3）覆盖 bus 拓扑全部表达；缺口纯在 agent 语义链（提案/装载/提示词），不落内核。

## 对实录 F4 的修正建议（供主管采择）

实录 F4 与卡补记中「server-side 自由态准入门拒绝…模型 prose 终答被扣」应修正为：「模型自报 capability_blocked（人声轨身份未披露，target_identity_not_disclosed），无提案存在；G 门对空循环的审计结果经回复面 failed_gate_ids 优先分支（goalrunner_chat.go:2098-2107）被错误渲染为『提案未通过准入门』」。语义不变的部分：模糊意图确实到不了提案；准入门先过前置确实必要。变化的部分：根因从「门」移到「观察断层+回复面」；修复责任从门语义移到 L1（首轮观察/结转）+L3（boundary 文案分层）。
