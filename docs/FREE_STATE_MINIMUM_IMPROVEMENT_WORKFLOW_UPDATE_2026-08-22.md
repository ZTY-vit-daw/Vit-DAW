# Free-State Minimum Improvement Workflow Update

Date: 2026-08-22

Status: design discussion record; no runtime implementation or formal acceptance was performed.

## 1. Scope

This document records the design decisions from the free-state workflow discussion. GUI trajectory rendering and the internal redesign of the fixed A-F mixing capability workflows are out of scope.

The acceptance entry remains an open request such as:

```text
检查一下当前工程有什么问题？
```

The test must not inject a target track, processor, defect, or desired outcome.

## 2. Main Finding

The current free-state runtime has useful local contracts:

- CCB observation requests are cataloged and view IDs are validated exactly.
- Direct mutation tool calls are rejected from free-state decisions.
- `needs_action` requires successful observation evidence.
- `needs_experiment` requires an observation bundle.

The structural gap is that these contracts do not form a Runtime-owned diagnostic spine. CCB views are peer views selected by the model, and the current `needs_experiment` gate can be satisfied by one usable observation bundle. It therefore does not require project binding, capacity assessment, relationship scanning, candidate frontier construction, or target-level evidence closure.

This explains how a run can form a bass improvement proposal after several local observations without proving that the bounded project search is complete.

Relevant implementation points:

- `agent/internal/agentloop/ccb_model_prompt.go`
- `agent/internal/agentloop/free_state_reasoning.go`
- `agent/internal/capabilitycontext/free_state_observation.go`
- `docs/FREE_STATE_CCB_OBSERVATION_PROTOCOL_V1.md`

## 3. Fixed Capability Layer Boundary

The fixed A-F mixing capability layer remains a separate, complete workflow. It is not to be converted into a collection of small tools as part of this free-state work.

The product has two valid entry modes:

```text
open request -> Runtime capacity route -> free-state workflow
open or explicit request -> Runtime capacity/intent route -> fixed A-F capability workflow
```

The Runtime owns the route decision. The model may provide semantic entry information, but it must not delegate merely to avoid making a free-state decision. Changes to the capability layer are not required for the next free-state milestone.

## 4. Free-State Diagnostic Spine

Free-state should be modeled as a host-owned phase machine:

```text
FS0 semantic_entry
  -> FS1 project_bound
  -> FS2 capacity_assessed
  -> FS3 project_scan
  -> FS4 diagnostic_round
  -> FS5 candidate_frontier
  -> FS6 target_confirmed
  -> FS7 improvement_proposal
  -> FS8 experiment_verification
  -> FS9 terminal
```

The model chooses observations inside the current phase, but it cannot skip the phase gates. A model turn is not a diagnostic round: one round may contain multiple tool calls, retries, or continuation invocations.

## 5. Diagnostic Rounds and Priority

The diagnostic round uses a priority queue rather than a fixed “round one, round two” checklist.

The initial default priority may be:

```text
level / headroom
-> frequency / occupancy
-> dynamics
-> stereo / space
-> transient / event
```

This is a default ordering, not a claim that every project has a level problem first. The Runtime may reorder, skip, or revisit a dimension when project evidence, freshness, cost, or a contradiction justifies it.

Each round has exactly one primary dimension and may include only the supporting views required for that dimension. The durable round record should contain at least:

```text
round_id
primary_dimension
priority_reason
views_requested
evidence_status
candidate_refs
unresolved_questions
skipped_dimensions
next_priority
project_revision
```

Skipping a dimension requires a reason such as `not_applicable`, `fresh_evidence_exists`, `already_covered`, `dependency_unavailable`, or `cost_limited`. Repeating the same target and view set requires new evidence, a new project revision, or a recorded contradiction.

## 6. Epistemic Policy: Defect Versus Improvement Hypothesis

Audio work does not have the same deterministic oracle as a coding test. Free-state must not require proof of an objective defect before it can perform a bounded experiment.

The decision states are:

```text
objective_defect
  deterministic evidence supports a governed action

plausible_improvement
  evidence supports a reversible improvement hypothesis but does not prove a defect

no_candidate_found
  the bounded search found no credible improvement opportunity
```

For an open request, the task contract is:

```text
Find at most one high-value, bounded improvement opportunity; if none is found within the search budget, report no_candidate_found.
```

`plausible_improvement` is sufficient for `needs_experiment`. The agent is not required to continue observing until it reaches certainty.

## 7. Minimum Viable Improvement Execution

The minimum executable improvement is one bounded, auditable experiment:

```text
select one diagnostic dimension
-> obtain dimension evidence
-> decide whether a candidate is plausible
-> obtain target-level evidence
-> freeze one hypothesis and expected response
-> apply one small reversible diagnostic dose
-> obtain fresh post-change evidence
-> classify material / subthreshold / ambiguous / unsupported
-> retain, rollback, continue once, or request user audition
```

The experiment contract must include:

```text
target
hypothesis
evidence_refs
action_domain and bounded action_kind
parameter bounds / diagnostic dose
expected response
verification scope
rollback reference
maximum attempts
```

One minimum execution contains one target, one hypothesis, one bounded action, and one verification scope. It must not become a batch of unrelated treatments.

## 8. Observation Loop Stop Rules

The following rules prevent the previous infinite-observation behavior:

- A repeated observation must answer a new unresolved question.
- The same target/view set cannot be requested again without a new reason.
- If evidence is relevant, fresh, target-specific, reversible, and directionally verifiable, the agent may enter `needs_experiment` without proving a defect.
- If the priority queue is exhausted, return `no_candidate_found`; do not imply that the project is perfect.
- If post-change evidence is `ambiguous`, stop automatic escalation and request audition or report ambiguity.
- A bounded search budget must limit diagnostic dimensions, candidate count, experiment attempts, and continuation work.

## 9. Evidence Gate for `needs_experiment`

The current minimum “one usable observation bundle” gate is insufficient for the open improvement contract. The target gate should require:

```text
project binding complete
capacity assessed
project-level scan complete
at least one diagnostic dimension closed
candidate frontier established
selected candidate has target-level evidence
proposal refs are fresh and revision-bound
```

The gate does not require objective proof of failure. If these conditions are not met, the valid outcome remains `needs_observation`.

## 10. Continuation and Persistence

Free-state must persist the semantic round, not only the latest durable snapshot:

```text
goal_id
run_id
round_id
current_phase
priority_queue
candidate_frontier
observation_ledger
project_revision
continuation_budget
```

The Runtime scheduler must own automatic continuation. `waiting_continue` should be an internal bounded state, not a requirement for the user to manually submit another “continue” message. Restart recovery must resume from the phase, round, evidence ledger, and candidate frontier.

The existing continuation infrastructure must be verified for scheduler ownership, durable persistence, restart recovery, and budget semantics; raising `max_turns` alone is not a fix.

## 11. Test Strategy

The next validation set should separate Runtime correctness from model policy quality.

### Runtime contract tests

Use deterministic fake model, fake CCB, and fake executor to cover phase transitions, priority queue updates, evidence gates, duplicate observations, bounded experiments, rollback, continuation, and restart recovery.

### Transcript replay tests

Replay saved real-model JSON/tool transcripts without making new model requests. Cover valid, malformed, repetitive, partial, stale, contradictory, and transient-error paths.

### Real-model open-intent tests

Use the unmodified request `检查一下当前工程有什么问题？`. Do not supply a target, processor, defect, or desired result. Verify that the model can enter the phase machine, select a bounded priority path, form a plausible improvement hypothesis, and stop with a proposal, no-candidate result, or explicit capability boundary.

### Audio outcome tests

Use the existing problem stems only after the Runtime gates pass. Separate technical readback, acoustic materiality, target response, net outcome, and human A/B judgment. A valid receipt must not claim audible improvement from analytical change alone.

## 12. Non-Goals for the Next Milestone

- Do not redesign the fixed A-F capability layer.
- Do not connect or redesign the GUI trajectory surface.
- Do not inject target-specific context into free-state acceptance.
- Do not solve the loop by raising `max_turns` or forcing a catalog view order.
- Do not perform Apply, Rollback, Settlement, or real audio writes during the design phase.

## 13. Next Implementation Artifacts

Before changing runtime code, prepare:

1. A free-state phase transition table.
2. A durable diagnostic-round and priority-queue schema.
3. The revised `needs_experiment` admission contract.
4. The minimum improvement execution receipt schema.
5. A simulation and transcript-replay matrix.
6. A real-model acceptance script beginning with the open request.

The immediate objective is not to make the agent prove that the mix is wrong. It is to make the agent find, test, and clearly stop on one bounded improvement opportunity when the evidence is plausible, while stopping cleanly when no such opportunity is found.
