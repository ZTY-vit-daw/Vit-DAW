# B4 full-project abstract EQ smoke contract

This contract covers only B4's handoff to the existing ordinary-Agent generic
static-EQ runtime. It excludes profile/learn, SPAL, network search, the retired
`static_mix.focus_position.v0` capability and unrelated stress fixtures.

## Routing

1. An explicit actionable B4 request observes `scope=full_project` and creates
   one `low_end_relation.treatment_plan.v1` covering every justified target.
2. A selected-track ordinary EQ request remains on
   `agent.effect.eq_control.v0`, even if its listening goal mentions low end.
3. A read-only B4 request ends after analysis and leaves no active sticky B4
   owner.

## Instance and loading phases

1. Every treatment target resolves independently to an exact `track_id` plus a
   topology-qualified loaded `plugin_id`.
2. Multiple qualified EQs are selected in one bounded project-level decision;
   no per-track Agent loop is started.
3. If any target has no qualified EQ, the workflow first reports
   `plugin_selection_required.v1` with no mutation.
4. Selecting a local candidate creates one project-level load Proposal for all
   missing targets. Confirmation of that Proposal must not write EQ parameters.
5. Every actual loaded `plugin_id` is rediscovered and topology-qualified.
   Any load/qualification failure deletes every instance added by that batch.

## EQ parameter phase

1. One `semantic_effect_batch.v1` contains exactly one existing
   `semantic_effect_action.v1` per treatment target, in treatment order.
2. Deterministic materialization freezes each exact instance's topology,
   requested edits, exact/quantized preview, planned writes and complete
   parameter preimage before one project-level EQ confirmation.
3. Confirmation executes the existing single-instance generic-EQ executor for
   every leaf. A rejected leaf restores all earlier leaves in reverse order.
4. Receipts preserve requested values, exact/quantized/rejected status, actual
   readback, operation references and the project rollback report.

## Verification

1. Every leaf must report structural readback pass.
2. A fresh full-project `mix.observe` rebuilds B4 relationship evidence and
   reports `improved`, `unchanged`, `worse` or `inconclusive` separately from
   structural status.
3. User acceptance remains `unknown`; fresh evidence must not be described as
   proof that the listening goal was accepted.

The deterministic unit gate is:

```powershell
cd D:\Vit_DAW\agent
go test ./...
```
