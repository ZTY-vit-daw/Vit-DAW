# Audio Closure Experiment ADR v1

Status: accepted, planned

Date: 2026-08-15

## Context

Vit can safely mutate a DAW project through typed executors, snapshots,
readback, work-tree history, and rollback.  Those mechanisms answer whether an
edit is authorized and recoverable.  They do not make an open-ended mixing
judgment objectively true.

The existing MinimalAudioClosure path has correctly reached an action-preflight
boundary in blind project smoke tests, but often stops there.  It asks the model
to establish that a candidate relationship is a definite mix defect before it
may select a treatment.  For most full-project mixing requests, that proof does
not exist: "clearer", "more forward", "tighter", and "better balance" depend
on a musical goal, reference, and user preference.

Adding observation views indefinitely will not turn an aesthetic or
goal-dependent question into a deterministic diagnosis.  It also risks an
observation loop that never reaches the executor.

## Decision

Vit distinguishes three kinds of information and authority:

| Level | Meaning | Permitted outcome |
| --- | --- | --- |
| L1 | Objective acoustic and project facts, such as peak level, spectrum, routing, or measured track relationship. | Report facts and cite evidence. |
| L2 | A deterministic engineering condition derived from L1 by a predeclared rule, such as clipping risk or a routing fault. | `needs_action` when the rule and action contract are satisfied. |
| L3 | An evidence-backed mixing improvement hypothesis, such as reducing a likely masking relationship to improve vocal intelligibility. | `needs_experiment`, never a claim that the existing mix is objectively wrong. |

MinimalAudioClosure becomes a minimal *verifiable closure*, rather than a
minimal deterministic-diagnosis closure.  A closure may reach either a
deterministic action or a bounded reversible experiment.

The outer MessageLoop remains responsible for dialogue, turn continuity, and
controller selection.  MinimalAudioClosure and the fixed-phase Orchestrated Mix
workflow are sibling controllers selected for different work:

- MinimalAudioClosure handles one local candidate and one small diagnostic or
  experimental loop.
- Orchestrated Mix handles a predefined, whole-project workflow with its own
  phase ordering and action budget.
- Neither controller directly mutates the DAW.  The typed executor remains the
  only mutation authority.

## Experiment Admission

An L3 proposal may enter `needs_experiment` only when all fields below are
present and pass deterministic policy validation.  A model's free-text claim
that evidence is "sufficient" is not execution authority.

```text
ExperimentAdmission {
  target: stable track, bus, or relationship reference
  evidence_refs: immutable references to relevant L1 observations
  hypothesis: bounded expected improvement, explicitly non-deterministic
  proposed_typed_action: supported processor/action family and target
  parameter_bounds: per-action limits and total experiment budget
  expected_effect: what should be compared after the change
  verification_plan: readback plus technical/A-B/user-or-reference check
  snapshot_id: pre-action recoverable project state
  rollback_plan: deterministic restoration condition and operation
  authority_policy: applicable permission and risk policy
  risk_class: reversible, bounded, non-destructive for v1
}
```

The admission validator rejects a proposal if its evidence does not identify
the target, its action is unsupported or unbounded, it lacks a snapshot or
rollback, or its policy does not permit the operation.  It must not require
proof that the hypothesis is universally correct.

The minimum observation standard for L3 is therefore intervention-relative,
not a global coverage threshold:

1. A target can be located unambiguously.
2. The cited facts plausibly relate to the proposed improvement.
3. The action is bounded to the target and is reversible.
4. A post-action comparison can be performed.
5. The current authority policy permits the experiment.

If any condition is absent, the closure may request one targeted observation
or stop with an auditable reason.  It must not repeatedly collect broad views
without identifying the missing admission field.

## Runtime States

The resulting action lifecycle is:

```text
candidate
  -> target_observed
  -> needs_action | needs_experiment | needs_user_confirmation | stopped

needs_action | needs_experiment
  -> snapshot_created
  -> typed_action_applied
  -> technically_verified
  -> kept | rolled_back | user_judgment_pending | stopped
```

`technically_verified` means that the typed action was applied and read back
correctly and did not violate technical guards.  It does not mean that the
mixing hypothesis was aesthetically proven.

## Permission Policy

In ordinary permission mode, a valid L3 admission returns
`needs_user_confirmation` with the proposed experiment and no project mutation.

In full-access mode, the user delegates a goal and explicit operating bounds.
The agent may autonomously run an admitted experiment only within those bounds.
Full access does not mean unrestricted mutation.  Policy must still define:

- allowed treatment categories and plugin-load permission;
- per-action parameter limits and total action/experiment budget;
- mandatory snapshot, readback, and rollback capability;
- operations that always require confirmation, including destructive,
  irreversible, high-risk, or scope-expanding changes.

All proposal, admission, execution, verification, retention, and rollback
records are appended to the existing auditable project history.

## Context Integrity

The initial user goal and authorization policy are immutable conversation-root
records.  Later continuation prompts are controller metadata, not replacement
user intent.  Every model-visible projection and every closure decision must
retain a reference to that initial goal.

This is required because a smoke replay demonstrated that a continuation
prompt could replace the model-visible `original_intent`, making a valid
full-project goal unavailable during later closure turns.

Evidence storage and projection may be evolved independently, but no context
compaction or HTTP boundary may overwrite or discard a cited L1 fact, the
initial goal, or the active authority policy.

## Consequences

- Blind, open-project optimization is evaluated by whether the agent forms and
  safely closes a reversible hypothesis loop, not by whether it discovers an
  evaluator-private unique defect.
- Deterministic engineering-fault smoke tests remain separate from goal-driven
  mixing smoke tests and open-optimization smoke tests.
- A stronger model may improve hypothesis quality, but changing models cannot
  substitute for the admission contract or policy validator.  Model comparison
  must use identical model-visible evidence bundles.
- The fixed Orchestrated Mix capability is retained.  This ADR does not turn
  all mixing into unconstrained free dialogue.

## Non-Goals

This ADR does not authorize:

- unlimited new observation views or a requirement to prove all L3 hypotheses;
- unbounded autonomous mixing edits;
- direct LLM mutation of the DAW;
- a wholesale Evidence Store or dual-mode rewrite before the vertical slice is
  proven;
- treating technical readback as an aesthetic success verdict.

## Initial Implementation and Acceptance Criteria

The first implementation is one vertical slice, not a broad controller
rewrite:

1. Preserve immutable initial intent and authority policy across continuation
   turns, with an integration test based on the product smoke continuation
   sequence.
2. Define a structured `ExperimentAdmission`, `needs_experiment`, and a
   deterministic admission validator.
3. Connect admitted full-access experiments to snapshot, typed execution,
   readback/A-B recording, and deterministic rollback.
4. In ordinary permission mode, verify that the same admission produces a
   proposal rather than a mutation.
5. Add three separate smoke classes: deterministic engineering fault,
   goal-driven mixing, and blind open optimization.

The vertical slice is accepted only when a full-access goal-driven or open
optimization smoke reaches a real typed, bounded edit and then records a
readback outcome plus either retention or rollback.  It is insufficient to
reach `action_preflight_boundary` alone.

