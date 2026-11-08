# JOURNEY-4 探针腿报告：确定性 B2 链不挂 audition（停止条件①触发）

- 卡：coord/cards/blocked/2026-10-07-JOURNEY-4.md（原 todo，2026-10-07 21:04 领取，commit ad4ed1f2）
- 运行：`powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev_agent_smoke.ps1 -Scenario journey_first -StartKernel`，退出码 0
- HEAD=ad4ed1f2（head.txt 同），工作树 diff=scripts/dev_agent_smoke.ps1 探针段（本卡新增）+ VitApp/Workspace 两文件 M（领取前已有）
- 泊位：journey berth 自起自拆（kernel pid 27796 / agent pid 8152，teardown 两行 ok），领取前 netstat 5555/7878/5556 空闲

## 结论

**J1 确定性 B2 链（capability_runtime_v1 propose→approve→executed_verified）不挂任何 audition 面。**
J4 判定腿的确定性装卡入口不存在于现有零-LLM 面——按卡面停止条件①转上交定方案（设计 JOURNEYS.md §3：涉 agent 侧新观测/驱动端点=越 scripts 域）。

## 探针证据（本 run，journey_propose_events.json + journey_summary.json）

- propose 会话 `dev_journey_first_probe_20261007_210806_1` 事件流 5 条，覆盖 B2 全链：
  1. turn.started
  2. trajectory.turn.started（turn:run_f05d66cef9570eb7）
  3. turn.completed（status=waiting_confirmation = proposal 卡挂出）
  4. trajectory.turn.completed（waiting_for_user）
  5. turn.completed（completed = approve 执行收口）
- **audition.\*=0；audition.candidate.ready=False；audition_session_id=False；mix_tick=0；applied/intervention 类=0**
- 对照（R4=JOURNEY-3 20261007_202322，自由态 NL 面，journey_nl_events_after_hop.json）：hop 后新增
  trajectory.intervention.applied → audition.prepare.started → audition.candidate.ready → audition.ready(×2)，
  会话号 `audition:turn:free_state_941daf021bf411ff:round-1-905bff1720a328c2`（自由态 prepareFreeStateAudition 构造形态）。

## 锚点清单

### A. audition prepare 触发源（agent 侧唯一装配路径=自由态实验 runtime）

| # | 锚点 | 说明 |
|---|---|---|
| A1 | `agent/internal/chat/audition_events.go:585` `prepareFreeStateAudition` | 唯一 agent 侧装配函数；sessionID=`audition:<Experiment.ID>:<round.ID>`；前置=freeStateReasoningLoop 携 Experiment |
| A2 | `agent/internal/chat/free_state_experiment_runtime.go:742` | settle 记录 target response outcome=EvaluationHumanAuditionReady 时触发（J3/R4 链的实际触发点） |
| A3 | `agent/internal/chat/free_state_d1_runtime.go:750` | D1 runtime 同型触发 |
| A4 | `VitApp/Source/Service/AuditionPreviewService.cpp:186`（audition.prepare.started）、`:509-510`（audition.candidate.ready） | 内核遥测源；经 `agent/internal/chat/server.go:545` ingestAuditionTelemetry → `audition_events.go:168` 重发到会话流（需 conversation_id） |

### B. J1 链与 R4 自由态链差异面

| # | 差异点 | 证据 |
|---|---|---|
| B1 | J1 B2 直驱链走 capability_runtime_v1 canary：/agent/chat context{capability_id=static_mix.static_balance.v0, interaction_mode=propose, capability_runtime_v1=true} → proposal 卡 → /agent/interaction/respond approve → 写租约执行 → executed_verified | capability_runtime_canary.go / capability_conversation.go / capability_recovery.go / mix_session_workflow.go 四文件 grep 零 audition/AuditionPrepare 调用；探针实证 applied_like=0 |
| B2 | J1 链无 freeStateLoop、无 Experiment round、无 trajectory.intervention.applied 事件 | 探针事件流 5 条全量在案（上表）；R4 对照有 intervention.applied |
| B3 | R4 链=goalrunner NL 面：startFreeStateExperiment（`free_state_experiment_runtime.go:514`，Experiment 挂 loop 的唯一入口，由 agentloop FreeStateDecision/ImprovementProposal 驱动=LLM 面）→ applied → target response human_audition_ready → A2 → A1 → 内核 prepare → 遥测三事件 | R4 工件 + 代码锚点 |
| B4 | J4 判定腿断言面全部要求自由态 loop 在场：/agent/audition/status 有效会话、user_judgment.requested（`audition_events.go:768` requestAuditionJudgment 要求 loop.Experiment）→recorded（judgment POST recordFreeStateAuditionJudgment 要求 loop 身份匹配） | 代码锚点 |
| B5 | mix_tick A/B 支路（`mix_tick_audition.go`，B12-2）不写 experiment evidence、不产生 user_judgment 事件（server.go handleAuditionJudgment 注释明示），不满足 J4 断言面 | 代码锚点 |

### 转上交的含义

J4 判定腿要在零-LLM 前提下坐实，需要决策侧二选一定方案：
1. agent 侧新增确定性驱动/观测端点（把自由态 audition 装配暴露为 HTTP 直驱面）——越 scripts 域，需另立卡；
2. 或把 J4 判定腿改挂 J3 概率面产物之上（放弃零-LLM 前提，改验收口径为概率面）——改设计。

停止条件②未触发：判定腿未开工，无新观测端点诉求被触碰。

## 端测边界声明

本卡改动仅 `scripts/dev_agent_smoke.ps1` journey_first 场景段（record-only 事件采集+summary 字段，无既有断言改动）；无 agent/webui 代码改动；断言只落服务端/事件拥有面，无渲染面与用户旅程面改动。
