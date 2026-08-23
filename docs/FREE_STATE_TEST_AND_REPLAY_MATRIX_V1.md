# Free-State Test and Replay Matrix v1

Status: 已实现（Phase C 收口，2026-08-23）。L1 全部落地（M01–M10，见各行文件路径）。L2 回放 M11–M17：agent/internal/agentloop/free_state_replay_test.go + fixture 家族 agent/internal/agentloop/testdata/free_state_replay/（每份记录来源 artifact sha256 与变换类型；生成脚本 scripts/gen_free_state_replay_fixtures.py）。L4 M19：agent/internal/experiment/audio_outcome_test.go（五层分离红线）。L3 M18：scripts/run_free_state_open_intent_acceptance.ps1（observation_within_budget 为 /agent/events 实际 ccb.observation_request 计数（预算 24），终态经运行时接口判定（continuations 的 free_state_decision_status/free_state_stop_reason/free_state_current_phase/free_state_limitations + goal stop_reason），日志正则仅诊断）。回放零网络零真模型；封存断言不硬编码易变标识。

Date: 2026-08-23

行文原则参照 `docs/COM_V1_TEST_CONTRACT.md` §1：**确定性 fixture 与 product-path smoke 分离**——确定性 fake 测试证明 Runtime 契约，真模型/真栈冒烟证明协议不变量；两者互不替代。四层划分来自 ADR §11。

## 1. 现有 fake 基础设施盘点（已核对）

- `freeStateTestExecutor`（agent/internal/agentloop/free_state_reasoning_test.go:17-58）：可编程工具执行 fake。
- `fakeMessageCompleter`（agent/internal/agentloop/message_loop_test.go:21-27）：确定性模型输出 fake。
- `free_state_reasoning_test.go` 现有 **50** 个测试函数（78–1503 行），已覆盖：CCB 观察前置（144、844）、rejected view set 不可重试（281、889、935、982、1119）、观察 ledger 披露（1459）、同族/总量 action 上界（603、676）、candidate frontier target-level 门（1241、1289、1324、1363）、usable project 观察不可重复（1439）、mutation 工具拒绝（1415）、diagnostic-only 边界（417–567）等。
- audioclosure：`driver_test.go`（State/Event/Store 路径）；experiment：`runtime` 侧 Turn/Round 生命周期测试。

## 2. 测试矩阵

层=L1 runtime contract / L2 transcript replay / L3 real-model open-intent / L4 audio outcome。

| # | 用例 | 层 | Phase B/C 落地位置 | 通过标准 |
|---|---|---|---|---|
| M01 | FS 相位正向全链：FS0→FS9 合法路径逐相位转移 | L1 | `agent/internal/agentloop/free_state_phase_test.go`（新） | 每次转移产生 `phase_transition` 事件且守卫结果为真 |
| M02 | 非法转移拒绝（产物 1 §2 示例 5 条逐条） | L1 | 同上 | 转移被拒且决策被降级为 needs_observation/blocked |
| M03 | 优先队列默认序 + priority_reason 重排 + skip 理由枚举校验 | L1 | `agent/internal/agentloop/free_state_priority_queue_test.go`（新） | 无理由 skip 被拒；理由枚举外值被拒 |
| M04 | 同 target/view set 重请求三准入（新证据/新 revision/记录矛盾） | L1 | 同上 | 三准入外重复请求被拒（推广 1439 号测试语义） |
| M05 | 队列耗尽 ⇒ no_candidate_found，不得暗示工程完美 | L1 | 同上 | 终态 status=no_candidate_found 且 reply 无「完美/无问题」类表述断言（模式断言，不硬编码） |
| M06 | needs_experiment 新门 G1–G7 逐项缺一即拒绝 | L1 | `agent/internal/agentloop/free_state_gate_test.go`（新） | 7 个变体各拒绝；门替换后 574-576 弱门测试改写 |
| M07 | 门通过 ⇒ Admission 构造且 Validate 通过；门不通过唯一出口 needs_observation | L1 | 同上 | 状态断言 |
| M08 | 一轮实验 classification/disposition 合法组合表（产物 2 §2 机检规则） | L1 | `agent/internal/experiment/receipt_schema_test.go`（新） | 非法组合（如 ambiguous+continue_once）被拒 |
| M09 | continue_once 全局至多一次 | L1 | 同上 | 第二次被拒 |
| M10 | continuation：waiting_continue 内部有界、budget 耗尽停机、重启从 phase/round/ledger/frontier 恢复 | L1 | `agent/internal/chat/free_state_continuation_test.go`（新，扩展 continuation_scheduler 既有测试） | 恢复后状态与宕机前一致 |
| M11 | replay：valid transcript 全程回放达到同一终态 | L2 | 已实现：`agent/internal/agentloop/free_state_replay_test.go` TestM11ReplayValidReachesSameTerminal + `testdata/free_state_replay/valid.json` | 无新模型请求；终态一致 |
| M12 | replay：malformed（截断 JSON、未知 status、坏 view id） | L2 | 已实现：TestM12ReplayMalformedRejectedAndAuditable + 3 份 malformed_*.json | 拒绝且错误可审计（protocol repair / final_gate trace / 保留 rejected 回执） |
| M13 | replay：repetitive（同 view set 重复请求） | L2 | 已实现：TestM13ReplayRepetitiveRejectedFromSecondRequest + repetitive.json | 第二次起被 M04 规则拒绝（每个 view set 恰执行一次） |
| M14 | replay：partial（bundle partial/omission） | L2 | 已实现：TestM14ReplayPartialPreservedReadinessNotUpgraded + partial.json | partial 原样保留，readiness 不升级 |
| M15 | replay：stale（project revision 变化后旧 refs） | L2 | 已实现：TestM15ReplayStaleRefsRejectedByGate + stale.json | G7 拒绝 stale refs（final_gate trace 含 G7_fresh_revision_bound_refs） |
| M16 | replay：contradictory（轮次间证据冲突） | L2 | 已实现：TestM16ReplayContradictionRevisitedNotSilentlyPassed + contradictory.json | 触发 discrimination 重访，不静默通过（不得 settled satisfied） |
| M17 | replay：transient-error（transport/LLM 瞬时失败后恢复） | L2 | 已实现：TestM17ReplayTransientErrorRecoversWithoutRepeatingObservation + transient_error.json | 恢复续跑（continuation 恢复 loop 状态），不重复已执行观察 |
| M18 | 真模型开放意图验收（首条消息固定，零注入） | L3 | `scripts/run_free_state_open_intent_acceptance.ps1` + `scripts/free_state_open_intent_acceptance.py`（已实现，见产物 6） | 协议不变量成立且脚本退出码 0 |
| M19 | 音频结果五层分离（产物 4 §3 红线） | L4 | 已实现：`agent/internal/experiment/audio_outcome_test.go`（M06 门已落地，启用条件满足） | 分析性变化不得产生可听改善表述 |

## 3. Replay fixture 家族与素材来源

fixture 家族 = {valid, malformed, repetitive, partial, stale, contradictory, transient-error}（M11–M17 一一对应）。素材来源 = **产物 6 首跑落盘**的 transcript：`artifacts/free_state_open_intent_acceptance/<时间戳>/`（raw 响应 + 决策 JSON）。素材路径登记（首跑 PASS 产物，供 Phase B 固化为 `agent/internal/agentloop/testdata/free_state_replay/`）：

- `artifacts/free_state_open_intent_acceptance/20260823_120030/`（首跑 PASS：协议不变量成立，终态 capability_blocked；transcript/ + free_state_open_intent_acceptance.json）
- `artifacts/free_state_open_intent_acceptance/20260823_125822/`（门后预算收束运行）
- `artifacts/free_state_open_intent_acceptance/20260823_145032/`（接线后脊柱推进运行：free_state_current_phase=fs2_capacity_assessed）
- 历史基线事实（Phase B 前如实记录，多数已由 Phase B/C 修复）：弱门已被 G1–G7 整体替换；终态 stop reason 现可经 /agent/runtime/status continuation 行（free_state_decision_status/free_state_stop_reason/free_state_current_phase/free_state_limitations）查询，日志正则仅作诊断。
- `artifacts/free_state_open_intent_acceptance/20260823_153758/`（Phase C 收口复跑 PASS：退出码 0，spine_progressed=true，脊柱 fs2_capacity_assessed，终态 capability_blocked（经 /agent/runtime/status continuation 行查询），观察计数 8/24 预算内；capability_blocked 属合法收口——门未全过故无 improvement_proposal，成因经运行时接口可查询）。
- Phase C 固化结果：9 份 fixture（valid + malformed×3 + repetitive/partial/stale/contradictory/transient-error）落在 `agent/internal/agentloop/testdata/free_state_replay/`，每份 `source` 字段记录来源 artifact 路径、sha256 与变换类型（真实 transcript 为 HTTP envelope，模型轮脚本为文档化变换重构；负例家族为 synthetic variant）；再生成用 `scripts/gen_free_state_replay_fixtures.py`。

fixture 化纪律：从 transcript 抽取时不得把易变标识（轨道名等）硬编码进断言（AGENTS.md §5 封存纪律）；transformed fixture 记录变换类型与来源 hash。

## 4. 通过标准总结

- L1/L2：`go test ./...` 全绿（确定性，无网络、无真模型）。
- L3：ps1 脚本退出码 0（PASS 语义=协议不变量成立，不评模型策略质量——ADR §11 分离）。
- L4：仅在 L1 新门实现并通过后进入；评审五层分离红线。
