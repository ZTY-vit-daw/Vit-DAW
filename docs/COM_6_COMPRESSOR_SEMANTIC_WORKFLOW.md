# COM-6 Compressor Semantic Workflow

Status: implemented contract
Date: 2026-08-04

## Purpose

COM-6 adds the planning portion of the ordinary Agent compressor semantic workflow. It connects the
existing live broadband-compressor recognizer/controller boundary with COM evidence and LLM musical
judgement, but it does not authorize or execute a parameter mutation.

The workflow is a structured, evidence-driven and governed semantic control workflow. It is not an
LLM chain-of-thought record and it is not C2.

```text
abstract compressor request
-> audio processor semantic identity card
-> intent / semantic-axis / evidence plan
-> COM observation
-> dimension-filtered COM decision evidence
-> selected-axis control brief
-> compact LLM control decision
-> deterministic absolute planning-only compressor plan assembly
-> deterministic reachability check
-> at most one constrained LLM revision
-> stop before confirmation or mutation
```

## Processor Identity Card

Schema: `audio_processor.semantic_identity_card.v1`
Short name: processor card
Maximum encoded size: 2048 bytes

The card answers only:

1. Who is the current processor?
2. What generic interaction archetype does its live topology support?
3. What are its most important present and absent control boundaries?

It contains:

- product name and optional manufacturer;
- `broadband_compressor` family;
- topology-derived interaction style;
- at most five signature semantic roles;
- at most eight hard boundaries;
- live structural classification, confidence and topology generation.

It must not contain `control_ref`, parameter IDs, normalized values, sampled curves, GUI coordinates,
presets, recommended settings or product-specific execution mappings. Product identity activates an
LLM prior; the local live topology constrains that prior and remains the execution fact.

An unknown product is valid. It uses `Unknown broadband compressor`, `identity_status=unknown`, and
the live archetype. A known product with no locally supplied manufacturer uses `name_only`.

## Semantic Axes

The closed v1 axis vocabulary is:

- `activation_intensity`
- `transfer_severity`
- `transient_timing`
- `recovery_motion`
- `detector_focus`
- `output_normalization`
- `parallel_balance`
- `character`

The phase-one LLM output is `semantic_effect.compressor_intent_plan.v1`. It selects one to four axes,
preserves negative constraints and requests `paired_io`, `source_only` or `not_needed` COM evidence.
This phase sees the processor card before it sees COM or any control surface.

## Progressive Disclosure

COM-6 uses three distinct disclosures:

1. Processor card: identity, archetype, signature roles and hard boundaries.
2. COM decision evidence: only phase-one-requested dimensions from the compact typed observation,
   plus the minimum trust, identifiability, summary, limitation and evidence-reference facts.
3. Candidate control brief: only path/role controls that can serve the selected semantic axes, with
   current physical presentation and bounded reachable summaries.

The brief schema is `compressor.control_brief.v1`. It omits parameter ID, `control_ref`, normalized
curves and every unrelated control. The full live surface remains local.

The decision-evidence schema is `semantic_effect.compressor_decision_evidence.v1`. The complete COM
context remains the workflow and audit record, but it is not copied into the phase-two prompt. The
decision projection removes duplicate full behavior, identifiability, trust and `llm_context`
representations and retains only the compact facts whose layer or dimension was requested by phase one.

## Planning Contract

Schema: `semantic_effect.compressor_plan.v1`

The plan carries:

- exact track/plugin target;
- exact user goal and negative constraints;
- identity-card ID and live topology generation;
- selected semantic axes;
- bounded COM evidence statement;
- one to six absolute physical/enum control proposals;
- a frozen evaluation contract;
- a revision record.

Every proposed control states its semantic axis, path, role, absolute target, acoustic purpose,
evidence references and confidence.

The phase-two LLM does not emit this complete plan directly. Its private boundary is
`semantic_effect.compressor_control_decision.v1`, containing only:

- one to six axis/path/role/absolute-target decisions with concise purpose and confidence;
- one compact future COM evaluation row per served axis;
- bounded limitations.

The deterministic host then supplies the exact target, frozen goal and negative constraints,
identity-card ID, topology generation, COM evidence identity and summary, stable proposal IDs,
revision metadata, level-match policy, success policy and no-auto-iteration rule. The assembled public
plan still passes the unchanged `semantic_effect.compressor_plan.v1` validator before reachability or
COM-7 materialization. The model never emits a parameter ID, normalized value or `control_ref`.

Normative invariants:

- `planning_only=true`;
- `mutation_authorized=false`;
- output/makeup gain can serve only `output_normalization`;
- mix/wet/dry can serve only `parallel_balance`;
- a role absent from the selected-axis brief is unreachable;
- source-only evidence cannot prove current gain action, transient response or recovery behavior;
- local hard boundaries override product-name memory;
- CLA-2A-like amount-driven surfaces cannot acquire invented threshold, ratio, attack or release.

## Evaluation Contract

Schema: `semantic_effect.compressor_evaluation_contract.v1`

The plan freezes the exact user goal and one evaluation row per selected axis. Each row states the
desired bounded behavioral direction, relevant COM dimensions and a future acceptance condition.
The policy is always `bounded_com_change_delta_plus_user_acceptance` and `no_auto_iteration=true`.

COM change-delta may establish that behavior changed in the intended bounded direction. It does not
prove artistic quality or user acceptance. COM-6 does not execute the plan and therefore does not
evaluate it.

## Rejection And Revision

The deterministic reachability seam produces
`semantic_effect.compressor_materialization_rejection.v1`. A rejection identifies the unreachable
proposals, reason, and currently reachable path/role alternatives. It proves
`mutation_performed=false` and permits exactly one constrained LLM revision.

The revised plan must retain the exact goal, target, card ID, topology generation and selected axes;
it binds the rejection ID. A second unreachable result stops the workflow. There is no retry loop.

## Product Path

The ordinary Agent routes abstract compressor actions into COM-6 before its general tool loop. Exact
requests such as `set Threshold to -12 dB and Ratio to 4:1` remain on the existing typed compressor
control path. Discussion-only requests do not produce plans. Limiters and multiband compressors stay
outside the route.

The Godot product-path smoke must prove:

- the production Agent binary was rebuilt and launched by Godot;
- the processor card reaches phase one before COM;
- a COM observation or explicit bounded missing/fallback state reaches phase two;
- only the selected-axis control brief is disclosed;
- `semantic_effect.compressor_plan.v1` is returned;
- no item/tool execution for compressor parameter writes occurs;
- all live plugin parameters are byte-for-byte/numerically unchanged before and after planning.

## Non-goals

COM-6 does not implement confirmation, mutation, readback, rollback, post-action automatic
iteration, C2, limiters, multiband compression, Profile, VPS, SPAL or product-specific mappings.
