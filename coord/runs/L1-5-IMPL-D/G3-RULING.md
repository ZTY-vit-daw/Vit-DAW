# L1-5-IMPL-D G3 A/B 裁定报告（harness 线收口）

- 卡：L1-5-IMPL-D（腿4 G3 A/B 执行）
- 执行：GLM-5.3 执行会话（PC / ZCode），2026-10-09 晚窗
- run：`coord/runs/L1-5-IMPL-D/20261009_195558/`（SCRIPT-LASTEXITCODE=0）
- 被测代码：port/l1-5-impl-d @ 18dc5c87+（腿1-3 已提交；遥测面小改随腿4 提交）
- 栈占用记录：`coord/resources/PC-RUNTIME-STACK.md`（占用 19:47 登记 ae351b20，运行后释放）

## 1. 运行设定（§8 概率纪律预写面，与场景脚本内注释一致）

- 轮数：每模式 3 轮（`-HarnessAbRounds 3` 缺省），固定话术（J3 同源 base64：主唱/Bass 双 stem 提升响度清晰度意图），每轮新会话；A/B 变量=harness 模式（env `VIT_DAW_HARNESS`，agent 按模式重启，内核与工程全程同一 berth）。
- fixture：J1 双 stem 配方（Lead Vocal 440Hz + Bass 110Hz，DAD 烘焙 ready 后持久化重开）→ 权限 full_project_access。三腿（fixture/open/authority）为确定性门槛（失败即 throw——本次全过）。
- 成功条件（逐轮分类，记录不抛）：judgment_terminal（终局判定）/ needs_user_*（健康链路待用户）。
- 失败分类：stalled_slice_limit / budget_exhausted（pull 单列）/ infra_error_llm|transport|protocol / environment_interrupt（分记）。
- 止损线：同模式连续 2 轮 infra_error_* → 停该模式；轮后 agent 健康失败 → 全 run 止损。本次两线均未触发。

## 2. 指标三层（工件：harness_ab_metrics_{push,pull}.json / harness_ab_summary.json / harness_ab_telemetry_{push,pull}.jsonl）

### 2.1 判定链 outcome 分层

| 模式 | 轮1 | 轮2 | 轮3 | 失败类（no_candidate/capability_blocked/budget_exhausted/infra） |
|---|---|---|---|---|
| push | judgment_terminal（limit_reached→nudge→completed/no_pending_mix_tick_candidate） | 同左 | 同左 | 0 |
| pull | needs_user_confirmation（首轮提案→waiting_confirmation） | 同左 | 同左 | 0 |

两模式三轮全部落在健康类（零失败类、零 budget_exhausted、零 infra_error、零 panic——两模式 agent 日志 panic/fatal 计数=0）。但 outcome 分布**系统性不同**：push 每轮 2 turn（首 turn 触 max_turns 暂停→nudge→确定式完成回 `no_pending_mix_tick_candidate`），pull 每轮 1 turn 直达确认卡（模型在精简装配下选择首轮提案待确认）。completed≠judgment_ok（§11.2）：push 的 completed 是确定式回退完成，**两模式均未产出结构化 judgment_ok 判定**——质量对比按分层如实记录，不据此判 pull 劣化。

### 2.2 成本（LLM 调用轮数+字节量；probe 成本=unknown——执行面无计量源，账本如实标 unknown 不填 0）

| 模式 | LLM 调用 | messages_total | prefix_bytes/调用 | dynamic_bytes/调用 |
|---|---|---|---|---|
| push | 5（≈1.7/轮） | 10 | ≈14,207 | **≈38,637**（全量快照注入） |
| pull | 13（≈4.3/轮） | 174 | ≈33,642（含冷启动底座+协议段） | **≈897**（动态区仅状态+预算） |

pull 的**逐 turn 注入语义达成**（动态区 42× 缩小：模型每轮不再收到"当前世界全量"）；但轮级 LLM 用量上升（13 vs 5 调用）——pull 链在精简装配下走多轮对话推进（历史在会话内累积，messages_total 174 vs 10）。**成本显著改善（轮数或字节量下降）未达成**：轮数上升；字节量按"每 turn 注入"口径大幅下降、按"每轮总注入"口径上升。

### 2.3 前缀命中（P1-P3）

| 模式 | p2_break_calls（断裂归因） | p3_isolated_calls（动静物理分离） | p1（前缀字节恒等） |
|---|---|---|---|
| push | 0/5 | 5/5 | 指纹级不可判（见下） |
| pull | 0/13 | 13/13 | 指纹级不可判（见下） |

两模式零前缀断裂（P2 判据全过）；动静分离结构性成立（P3 全过）。P1（跨 turn 字节级 starts-with）按现有指纹不可判：PromptFingerprint 哈希**整装配**（含动态区），跨轮恒等天然不成立（push 5 调用 5 指纹、pull 13 调用 13 指纹，与断裂无关）。P1 需前缀级指纹供给（见归因卡②）。

## 3. G3 裁定建议

**建议：不切换（生产缺省维持 push）+ 两轴差异归因卡。**依据（§6 门槛逐条）：

1. 质量不劣化：失败类零发生（无系统性回退证据）；但 outcome 分布系统性位移（确定式完成↔首轮提案确认）构成行为差异，需归因后方可评估"判定链成功率≥push 基线"。
2. 成本显著改善：未达成（轮级 LLM 调用 4.3 vs 1.7；仅逐 turn 注入口径改善）。
3. budget_exhausted 单列：0 轮（pull 未配置 MaxCycles 上限，本轮由确认卡自然暂停——预算校准面归归因卡①一并覆盖）。

### 差异归因卡建议两轴

- **①行为差异归因**：pull 精简装配（动态区仅 897B 状态行+预算披露）下模型首轮倾向提案待确认；push 全量快照注入下模型走满 turn 预算后由确定式回退完成。需在同类 fixture 上加确认往返（用户确认→执行→A/B 判定）与多话术采样，判定两种形态在"到达结构化判定"上的等价性；同时校准 pull 观察预算（MaxCycles/probe 计量源接入）。
- **②P1 计量判据供给**：PromptFingerprint 需增前缀级（仅稳定段）指纹或 AssemblyReport 暴露前缀段独立哈希，P1 判据（prefix_bytes 跨 turn starts-with 恒等）方可机械判定。

## 4. 覆盖边界（诚实声明）

- 真栈双模面：工程打开→权限→LLM 实验链→（pull：提案确认卡）已覆盖；**确认往返（用户确认→执行→A/B 判定站）未在真栈行使**（pull 轮止于确认卡；该面由 agentloop 单测覆盖：TestPullEntryConfirmationResumeSameSession）。归因卡①补真栈确认往返。
- probe 物理成本：执行面无统一计量源（提案 §5.1 已取证），账本 ProbeCostKnown=false 如实披露；MaxCycles 本轮未配置（uncapped）。
- 冷启动底座：chat 目标上下文未携带 engine_snapshot，pull 冷启动按 absent 显式处理（§4.1 fail-open 但显式）——底座真实供给面归归因卡①。
- 会话入口：仅 goal 面（messageLoop 三入口）分流；chat 直连管线未迁移（OQ-H3，归 G3 后切换卡）。

## 5. 结论

D 卡腿4 验收面齐：真栈双模场景 SCRIPT-LASTEXITCODE=0、指标三层工件落位、G3 裁定建议成文（不切换+差异归因）。harness 线（A/B/C/D）至此全部落地；pull 模式经 env 开关可用（缺省 push 零变化），切换决策待归因卡清偿后由主管裁定。
