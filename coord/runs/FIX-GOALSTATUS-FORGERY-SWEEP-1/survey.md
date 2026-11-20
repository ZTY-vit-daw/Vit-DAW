# FIX-GOALSTATUS-FORGERY-SWEEP-1 普查表（全树 goal_status 伪造面）

- 执行：forge334 / 2026-11-20 / 基线 main 948d7b1c（行号以该基线为准）
- 伪造面判定口径（沿 327/330 家族）：**决策应答面上，决定未执行/状态丢失，却把 goal_status 写成 completed（或经投影空回填伪造 completed）**；文案歧义（罐头「已处理或已过期」）为同族核心形态。轮次终态语义（一次 guidance turn 完成=completed）且文案明示未执行者，判合法或登记。

## 一、真伪造（同型，本卡修，5 处）

| # | 锚点（main 948d7b1c） | 触发路径 | 伪造形态 |
|---|---|---|---|
| F1 | chat/server.go:4959-4963（automation choreo 分支兜底；330 登记行号 4834） | interaction respond → 段落包络卡被 take（tombstone consumed）→ automationChoreoForConversation 无候选（候选过期/随工作区切换丢失）→ 兜底 | 罐头「这个段落包络确认已处理或已过期。」+ GoalStatus=StatusCompleted；决定未执行 |
| F2 | chat/server.go:5008-5016（mix tick 分支兜底；decision 非显式 apply 词时可达） | interaction respond → handlePendingMixTickChat 返回 !handled（pending mix tick 候选丢失） | 罐头「这个混音单步确认已处理或已过期。」+ completed；决定未执行 |
| F3 | chat/server.go:5079-5087（mix treatment 分支兜底） | interaction respond → handlePendingMixTreatmentChat !handled | 罐头「这个混音确认已处理或已过期。」+ completed；决定未执行 |
| F2b | chat/mix_tick_confirmation.go:87-95（no_pending_mix_tick_candidate；普查首轮 grep 被 head 截断漏收，红跑前可达性分析补全） | interaction respond 显式 apply 词（「可以执行」在 messageExplicitMixTickApply 表内）+ 无候选 → handled=true 直通；F2 同面同触发的孪生分支（approve+无候选的实际活体路径） | 文案明示但 goal_status="completed"；决定未执行 |
| F4 | chat/server.go:5165-5167 回填 + 5176-5193 CompleteGoal switch default（interaction 面第二伪造面，镜像 330 红跑揭示的 plan 面空回填） | 任何经 finalizeInteractionChatResponse 的空 goal_status 应答 → inferredGoalStatus 默认 completed → switch default s.harness.CompleteGoal(goalID, nil) 把 goal 运行时标成完成 | 回填伪造+运行时伪造：修 F1-F3/F2b 后回执置空 goal_status 会被此路径再伪造回 completed（响应面+goal runtime 双面） |

修复形态（对齐 327/330）：F1-F3 → 新 helper `interactionContinuationMissingReceipt`（WARN 留痕、Reply 明示「候选丢失/决定未执行/目标未因此完成」、GoalStatus 空串、StopReason=`interaction_continuation_missing`、workflow_data 带 decision_executed=false/mutation_performed=false/reason_candidates；保留原 payload 透传+结构化字段覆盖）；tombstone 已由 takePendingInteraction 的 "consumed" 记录（327 机制），重复应答走 catch-all already_retired。F2b → GoalStatus 置空、文案与 stop_reason 不变（测试仅钉 stop_reason 路由，亲证不钉 goal_status）。F4 → 豁免谓词 `receiptCarriesNoGoalStatus`（plan_already_retired/plan_not_found/interaction_already_retired/interaction_not_found/interaction_continuation_missing/no_pending_mix_tick_candidate）守卫 5165 回填与 5176 switch（豁免回执不触 goal runtime，镜像 plan 面 5513-5522 不写 goal 的先例）；旧谓词 pendingPlanReceiptCarriesNoGoalStatus 并入新谓词，3423 调用点随改。

## 二、语义合法（登记理由，不修，抽样列主簇）

| 锚点 | 定性 |
|---|---|
| chat/server.go:5430 / 5637（resolveAgentLoopConfirm 成功路径） | typed_events=consumed、replies 在卷，真实执行后 completed |
| chat/c2_dynamic_plugin_load.go:263 | execution receipts 验证 applied 后 completed |
| chat/c3_space_depth_runtime.go:155、pan_layout_runtime_canary.go:287 | 只读观察/分析真实产出后 completed |
| chat/full_access_semantic_chain.go:264 | mutation_performed=true 真实执行 |
| chat/mix_tick_audition.go:890/901 | 真失败→failed / 真判定→completed |
| chat/mix_tick_confirmation.go:124 / mix_treatment_confirmation.go:60（rejected 路径） | 决定（取消/拒绝）本身已被真实执行（transition+expire 在卷），completed=轮次收束 |
| chat/mix_treatment_confirmation.go:124（resolved） | resolver 已产出决策回执，轮次语义 |
| chat/goalrunner_chat.go:3124/3130/3140、turn_control.go、harness.go:1033/2856、plugin_prep_continuation.go:77、semantic_treatment_strategy.go:1752 | 透传真实 status（cancelled/failed/goal.Status） |
| chat/server.go:2794（主 chat 路径 inferredGoalStatus 空回填） | 轮次终态设计语义；receipts 不经此路径（2898 plan 路径直接 writeChat，已有 330 豁免） |
| capabilityCanaryBlockedResponse（capability_runtime_canary.go:680，六处调用） | 显式负回执 + GoalStatus=failed + Error，诚实 |
| chat/b4_eq_runtime.go:175 | WaitingClarification + 明示「没有加载或写入」 |
| chat/edu_jam_chat.go:223（eduJamBaseResponse）、mix_workflow_queue_view.go:704（queue 面 base，失败置 failed） | 轮次/工作流面语义，非决策伪造 |
| chat/agent_plan.go:150-152、interaction_retirement.go 全部 | 透传/327 修复物 |

## 三、语义存疑但非同型（登记候裁定，不扩域）

| 锚点 | 形态 | 不修理由 |
|---|---|---|
| chat/mix_tick_confirmation.go:235-242（expired_pending_mix_tick_candidate） | approve 被验证拒绝（决定未执行）但 completed + Error 同卷——字段自相矛盾（runtime 侧 Error→CompleteGoal(err) 诚实，仅响应字段谎） | 文案/stop_reason/Error 结构化字段均明示失败，对齐 failed/空属状态词汇裁定；同面 F2/F2b 已修 |
| chat/mix_treatment_confirmation.go:107-116（refused_d1_admission_mismatch） | D1 准入拒绝 + StatusCompleted + mutation_performed:false | 结构化 refused 标记在卷、文案明示；同上候裁定 |
| chat/mix_tick_confirmation.go:169-176 / mix_treatment_confirmation.go:127-134 / mix_tick_audition.go:869-878（三个 ambiguous 澄清轮） | 澄清轮 + completed（WaitingClarification 更诚实） | 文案明示未改；confirm_chat_1/server_test 钉死该流；改状态属语义裁定 |
| chat/server.go:3509-3531 legacyPendingPlanBroadMixBlockedConfirmResponse | 宽泛混音拦截终点：「已拦截…被拦截动作：…」+ completed + blocked:true + CompleteGoal(nil) | 文案显式负状态（非罐头歧义），是设计上的策略拦截收束；改收束语义属产品裁定 |
| chat/mix_session_workflow.go:447-462 staleMixActionResponse | 旧 MixBoard 动作失效恢复：「本次没有执行 DAW 操作」+ completed | 文案明示未执行（非歧义罐头）；legacy flag 门控路径；同上候裁定 |
| chat/mix_session_workflow.go:202/490 | plan mode 入口/停用入口回执 completed | 明示「没有执行 DAW 操作」；入口轮次语义 |
| chat/server.go:3381-3393 chatResponseFromPendingPlanDecision 空 map/空 reply 防御分支 | 「确认已处理。」+completed（空 map）/reply 兜底 | 当前调用图不可达（resolvePendingPlanDecision 恒返回非空 map）；goal_status 走透传+330 豁免 |

## 四、域外登记（不扩域）

- agent/webui/src/interactionGuard.ts + StandardActionCard.tsx：呈现面客户端台账引用旧罐头口径做重复提交防线——属 441 webui 域；修后 stop_reason 结构化字段可用，登记候 441 决定是否对齐。
- 普查范围：agent/internal 全树 `GoalStatus`/`"goal_status"`/罐头变体（已处理/已过期/失效）grep；chat 为唯一伪造面写入域；harness/goalrunner（chat 内）/agentloop/orchestration 均为透传或真实路径。
