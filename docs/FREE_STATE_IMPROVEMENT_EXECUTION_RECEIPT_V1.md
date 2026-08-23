# Free-State Minimum Improvement Execution Receipt v1

Status: 已实现（Phase C 收口，2026-08-23）。schema 类型与机检校验（classification/layers 一致性、ambiguous 不得 continue_once、continue_once 全局至多一次=Policy.MaxActionAttempts、五层分离红线）在 agent/internal/experiment/receipt.go；L1 测试 M08/M09：agent/internal/experiment/receipt_schema_test.go；L4 五层分离红线测试 M19：agent/internal/experiment/audio_outcome_test.go。真实音频 Apply/Rollback/Settlement 不在本阶段（Admission 构造+Validate 即边界）。

Date: 2026-08-23

范式参照：`project_change_receipt.v1`（docs/PROJECT_CHANGE_RECEIPT_V1.md，工程变化≠声学结论的分离表述）与 `ccb_observation_receipt.v1`（docs/FREE_STATE_CCB_OBSERVATION_PROTOCOL_V1.md，requested/executed/matches 审计三元组）。

## 1. 实验契约（八字段，ADR §7）

`free_state_improvement_experiment_contract.v1`——即 `experiment.Admission`（agent/internal/experiment/runtime.go:130-145）的字段子集对齐：

```json
{
  "schema_version": "free_state_improvement_experiment_contract.v1",
  "target": {},
  "hypothesis": "",
  "evidence_refs": [],
  "action_domain": "eq | compressor | limiter | gate_expander | de_esser | transient_shaper | multiband_dynamics",
  "action_kind": "",
  "parameter_bounds": {"diagnostic_dose": {}, "retained_dose": {}},
  "expected_response": "",
  "verification_scope": [],
  "rollback_reference": "",
  "maximum_attempts": 1
}
```

一次最小执行恰含 1 target、1 hypothesis、1 bounded action、1 verification scope（ADR §7）；不得成为不相关处理的批量。Admission 既有 `Validate`（runtime.go:147-187）承担 schema 完整性校验。

## 2. 回执本体：`free_state_improvement_execution_receipt.v1`

```json
{
  "schema_version": "free_state_improvement_execution_receipt.v1",
  "receipt_id": "fsx_<hash12>",
  "experiment_contract_ref": "",
  "project_revision": "",
  "classification": "material | subthreshold | ambiguous | unsupported",
  "disposition": "retain | rollback | continue_once | request_audition",
  "layers": {
    "technical_readback": {"status": "applied | failed | ambiguous"},
    "acoustic_materiality": {"status": "none | subthreshold | material"},
    "target_response": {"status": "absent | directional | sufficient | ambiguous"},
    "net_outcome": {"status": "improved | stable | plateau | rolled_back | ..."},
    "human_ab": {"status": "not_requested | pending | decided"}
  },
  "evidence_refs": [],
  "limitations": [],
  "recorded_at": ""
}
```

枚举对齐 experiment runtime 现值：TechnicalApplication（runtime.go:56-60）、AcousticMateriality（63-69）、TargetResponse（72-79）、RoundDecision（81-93，retain/rollback/next_round(=continue_once)/user_judgment_pending(=request_audition)）、SettlementOutcome（96-109）。

机检规则：

- `classification` 与 `layers.acoustic_materiality`/`layers.target_response` 一致（material⇒material+非absent；unsupported⇒target absent 或 technical failed；ambiguous 只允许 disposition=request_audition 或 rollback，不得 continue_once——ADR §8「ambiguous 停止自动升级」）。
- `disposition=continue_once` 全局至多一次（`maximum_attempts` 与 Policy.MaxActionAttempts=1，agent/internal/audioclosure/types.go:70 对齐；放宽须新 policy 记录）。
- `evidence_refs` 必须指向 fresh、revision-bound 的观察回执（同产物 3 G7）。

## 3. 红线（ADR §11，五层分离）

1. **分析性变化不得声称可听改善**：`technical_readback.status=applied` 不得推出 `net_outcome=improved`；工程回执语义同 project_change_receipt.v1 的 pending_authoritative_refresh 原则。
2. **technical readback / acoustic materiality / target response / net outcome / human A/B** 五层各自独立判定、独立字段，禁止一层结果填充另一层。
3. `human_ab.status` 非 `decided` 时，回执与任何上层报告不得出现「可听改善」表述；`request_audition` 是 ambiguous 的合法出口。
