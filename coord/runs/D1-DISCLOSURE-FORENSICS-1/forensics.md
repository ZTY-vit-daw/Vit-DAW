# D1-DISCLOSURE-FORENSICS-1 取证报告（读档零 LLM，2026-11-06 早窗）

- 执行：ZCode GLM-5.3 L1 PC 执行会话（258 卡会话）；worktree D:/Vit_DAW_wt_258impl @ port/d1-disclosure-forensics-1（领取提交 05012e5b，基线 92022b7c=origin/main）
- 取证材料：coord/runs/D1-RUNNER-FORENSICS-1/{forensics.md, verifyrun1/}（RUNBOOK+两 d1_report+agent log）+ 留盘大件 D:/Vit_DAW_wt_256impl/artifacts/free_state_d1_s1/{20261105_222048, 20261105_222912}/（journal/conversation store/agent_runtime_state）+ D:/Vit_DAW_wt_256impl/VitApp/Workspace/Logs/{agent_llm_telemetry.jsonl, agent_prompt_render.jsonl}（逐轮分段统计+逐轮 system/directives 载荷）
- 结论先行：**两形态根因均不在 M3 措辞链本体，均在 agent 域内（准入判据面×1+披露投影面×1），卡面停止条件不触发，进入修复腿。**

## 形态②（verifyrun2 20261105_222912：证据新鲜度准入未过→首响应 capability_blocked）

### 事实链（全部工件亲证）

1. **外部写者中途推 revision**：journal 亲证 `track.add_audio`（22:31:02，track 1039）与 `track.delete`（22:31:21）来源 `source=pcactl.certify_compressor`——**非模型**（模型同刻在 turn 2→3 之间的 LLM 调用内）。工程 revision 2→3→4。
2. **观察回执 revision 谱**：seed obs_143052@rev2（22:30:52）→ obs_143105@rev3（22:31:05，pcactl add 后）→ obs_143124@rev4（22:31:24，pcactl delete 后）——CCB 回执由 kernel 铸造，逐枚反映真实当前 revision。
3. **模型提案合规**：admission_receipt 亲证 `proposal_present=true, proposal_valid=true`，evidence_refs=[obs_143124@rev4]（smoke report 三处独立命中），G8 target_consistency=**pass**，G1-G6 全 pass，唯一 fail=G7_fresh_revision_bound_refs。
4. **closure 冻结在 rev2**：agent_runtime_state 持久化亲证 `minimal_audio_closures.audio_closure_dd7d1ce3748044a7.project_revision="2"`（请求入口 22:30:41 创建时锁定；pcactl 22:31:02 才动手）+ `settlement={reason: project_revision_stale, "observation belongs to a different project revision", settled_at 22:32:09.944}`——闭包在落账 rev4 观察时撞跨修订守卫**当场 settle 死亡**，与 admission_receipt recorded_at 22:32:09.942 同刻。
5. **回执自证不一致**：admission_receipt.project_revision=4（来源=loop.LatestProjectChange，free_state_reasoning_loop.go:1254）≠ 门实际准绳 closure revision=2——同一回执内「当前=4」与「要求=2」并存。

### 机理（代码锚点）

- **G7**（agent/internal/agentloop/free_state_gate.go:460-502）：每个 evidence ref 必须解析到 ledger 行且 `freeStateReceiptFreshRevisionBound(row, closureRevision)`——closureRevision 取 `state.input.Context["minimal_audio_closure"]["project_revision"]`。模型最新证据@rev4 对冻结准绳 rev2 永远 stale：**任何新观察都落在真实当前 revision（≥3），永远≠2，准入在本请求内不可满足**。agentloop 侧 turn4/turn5 两次 bounce（反滥用规则 2/3，free_state_reasoning.go:708-761），模型诚实终态 capability_blocked。
- **闭包 revision 只在请求入口 reconcile**：`prepareAudioClosureContext`（chat/audio_closure_controller.go:63-102）仅对比「新请求的 requestRevision」与 closure 追踪值；请求内（单次 /agent/chat 内多 LLM 轮）无任何重验通道。
- **落账守卫把漂移当死信**：`driver.RecordObservation`（audioclosure/driver.go:181-185）观察 revision≠closure 追踪值即 settle StopProjectRevisionStale。外部漂移（无 active capability、非 superseded）与「陈旧回放」未区分——比请求入口语义（同条件下 Revalidate 续命）更严苛，不对称即缺陷。
- **触发归因（诚实边界）**：认证 job 由 agent HTTP 面（/agent/processor-certification/start，唯一内部起点 processor_certification_entry.go:476）或 pcactl CLI 发起；烟测脚本面零调用（全仓 grep 亲证：runner ps1/py、dev_agent_smoke、autosweep 状态均不在场）；两 run 各一次、时点任意（run1=对话结束后 22:27:18，run2=对话中途 22:31:02）→ 最可能=Godot 设置面板手动点击（settings modal 有认证按钮），**未证实**。修复不依赖触发归因：真实用户对话中途编辑工程即同类外部漂移，生产常态。

## 形态①（verifyrun1 20261105_222048：轨道标识未披露→提案面未达 NOT_EXERCISED）

### 事实链

1. **ledger 投影里有轨 ID**：loop projection 的 observation_ledger.available_views 亲证 `project.structure` conclusion 含 tracks{bass=1007, drums=1012, guitar=1017, other=1022}，且存在轨键行 `track:1017::track.stereo_space / track.time_dynamics / track.activity_structure`（模型已选定候选轨 1017 并取齐目标级观察，stereo correlation=0.227）。
2. **模型上下文里没有**：telemetry 逐轮 section_stats 亲证 `observation_ledger` 1529B(t1)→2176B(t5)→**139B 桩（t7 起冻结至终局）**，同刻 `compaction_markers` 192→383 出现；终局轮 `active_observation`=1252B（定向视图紧凑载荷，不含轨 ID 表）。turn 指引模板要求 `target.id="<visible track id>"`（agent_prompt_render.jsonl 亲证）。
3. **终局无重取通道**：loop projection 亲证 `terminal_turn_locked=true, terminal_turn_reason=budget_critical`——终局轮工具锁死，模型自述「本轮没有重新取回通道」属实。
4. **G7/G8 fail 是下游**：admission_receipt `boundary=proposal_missing`（无提案→gateG7 len(refs)==0 直接 false，free_state_gate.go:461）——非独立信号。

### 机理（代码锚点）

- **warm 层预算 5KB**（contextruntime/model_projection.go:19 `ModelWarmBudgetBytes=5*1024`），warm 段=recent_goal_context/goal_trace_summary/observation_catalog_ref/**observation_ledger**（model_projection.go:273）。
- **最大者先降级**：`enforceModelLayerBudgets`（model_projection.go:174-230）超预算时按段大小降序把 warm 段替换为 cold ref（CompactionMarker）。活跃观察期 ledger（含结论摘要的 durable 绑定记忆）随新观察增长（t7 前新增 obs_142257/142328 行）为**最大 warm 段**→首降级→139B 桩。预算数学复核：t7 前测算 trace≈2307+ledger(>2307)+catalog 641+recent 543>5120 → ledger（最大）降级 → 总 3630≤5120 ✓ 与遥测逐字节吻合（trace 存活、ledger 死）。
- **ledger 是模型唯一的跨轮观察记忆**：消息结构=[system, user(重建)]——无 assistant 历史（telemetry history_prompt_bytes=0 亲证）；active_observation 随轮轮转。ledger 被降级=绑定记忆（轨 ID×视图×obs id）清零；终局轮 active obs 恰为不含轨 ID 的定向视图 → 模型陈述「当前上下文未披露任何轨道标识」**字面为真**。
- **M3 非根因**：system prompt 在 static 层恒定 17670B（prefix_bytes=18328 不变），warm 压力源=goal_trace_summary 随轮累积（583→3236B）——与措辞链无关。

## 修复方案（按根因最小修；预期三面中的两面：准入判据+披露投影）

1. **修复②A（准入判据面，agentloop/free_state_gate.go）**：G7/G7 gap 的所需 revision=closure 追踪值，**除非** ledger 内存在 kernel 铸造回执携带严格更新的 revision、closure 无 active_capability 且该 revision 不在 superseded 集 → 以最新回执 revision 为准绳（冻结基准落后于 kernel 铸造证据=外部漂移，回执本身即货币性证明）。G1/G8 不动（G1 的 contract==closure 两侧同陈旧仍相容）。
2. **修复②B（chat/audio_closure_controller.go 观察落账循环 ~L719）**：落账前若观察 revision 严格更新且非 superseded、closure 无 active capability、非 parked-judgment、非 post-action 债 → 先 `driver.RevalidateProjectRevision` 治愈再落账（请求入口语义在漂移实际检出点的镜像）→ 闭包不再死于外部漂移，下游（frontier/settle/G1 契约投影）一致。
3. **修复①（披露投影面，contextruntime/model_projection.go enforceModelLayerBudgets）**：warm 降级序中 observation_ledger 排最后（durable 证据记忆后于过程元数据降级）——活跃观察期 trace/catalog 先让位，ledger 存活至提案面；预算值、marker/ref 语义、fail-closed 溢出全部不动。

### 不变量（治理红线核对）

- 降级替换不改内容截断语义（仍 ref+marker，never silent truncation）；G7 fail-closed 本质不变（仅准绳从「陈旧冻结值」回到「kernel 铸造的当前值」）；落账守卫对 parked-judgment 回放（FIX-SETTLE-PARK-REPLAY-RACE-1）与 post-action 过滤语义零改动；RevalidateProjectRevision 为既有审计事件迁移（EventProjectRevisionChanged）；automation_intent 词表/semantic_entry 零触碰；确认链/零裸写零触碰。
- 256 断言面（scripts/free_state_d1_smoke.py 分支容忍）零触碰；修复方向=准入更可达，1-mutation 主链与 0-mutation 边界分支两态均保持合法。
