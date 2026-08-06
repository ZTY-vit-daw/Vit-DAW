# COM-6 Compressor Semantic Workflow Validation

Status: passed
Date: 2026-08-04

## Scope

This record validates the planning-only product path defined by
`COM_6_COMPRESSOR_SEMANTIC_WORKFLOW.md`. It covers the ordinary Agent route from an abstract
single-band broadband-compressor request through processor-card disclosure, COM observation,
selected-axis control disclosure and an absolute semantic plan.

It does not validate or authorize parameter mutation, confirmation, rollback, COM-7, C2, limiters,
multiband compression, Profile, VPS, SPAL or product-specific mappings.

## Product-Path Experiment

The experiment used the production launch path:

```powershell
D:\Vit_DAW\scripts\run_vit_product_path_smoke.ps1 `
  -RepoRoot D:\Vit_DAW `
  -GodotProjectRoot D:\Godot\project\vit-daw-frontend `
  -GodotExe D:\Godot\Godot_v4.6.1-stable_win64.exe `
  -CompressorSemanticPlanningAgentOnly `
  -TimeoutSeconds 300
```

Godot launched the production Kernel, VSP Hub and rebuilt Agent. The lifecycle gate verified all
three child identities, all four expected ports, exact binary paths and hashes, and VSP Hub health
with a live Agent session.

The semantic case then:

1. created a temporary audio track;
2. loaded FabFilter Pro-C 2;
3. imported the deterministic 48 kHz, 10-second transient-burst WAV;
4. captured the complete live parameter snapshot;
5. sent `压得更稳一点，但尽量保留瞬态，不要因为响度变大产生错觉。` through `/agent/chat`;
6. captured the complete live parameter snapshot again;
7. deleted the temporary track.

Evidence of record:

- `D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\product_path_20260804_175238\compressor_semantic_planning_report.json`
- `D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\product_path_20260804_175238\compressor_semantic_source_fallback_report.json`

## Result

The normal report status is `ok`; all 19 gates passed with no failed gate.

- Workflow: `semantic_compressor_planning`
- Processor: `Pro-C 2`
- COM mode/status: `paired_io / ready`
- COM projection: `com_e4963a6e3acc7bd5bdcd`
- Audio window: 480,000 samples
- Plan: four absolute proposals over four selected semantic axes
- Planning contract: `planning_only=true`
- Mutation authority: `false`
- Mutation performed: `false`
- Tool execution events: none
- Parameter identities/count: unchanged
- Every captured normalized parameter value: unchanged
- Temporary track cleanup: passed

The successful paired observation reported bounded gain action, transient response, recovery
motion, level effect and stereo behavior. Trigger relation remained explicitly not identifiable
because aligned band evidence was absent. The Planner retained that limitation instead of inventing
detector behavior.

The second product run deliberately omitted the exact paired sample window while retaining the same
imported source snapshot. The intent still requested `paired_io`; the workflow recorded
`paired_io exact sample window unavailable`, fell back to `source_only / ready`, and produced
projection `com_8d3353886cbb8e026570`. All 20 gates passed, including the explicit fallback gate,
with no tool execution, no parameter change and successful temporary-track cleanup.

## Defect Found During Validation

An earlier product run returned a valid typed `*com.Projection` from `mix.observe`, but the COM-6
caller accepted only `map[string]any`. The caller therefore reported `mix.observe omitted COM
projection`, even though the projection existed. Planning still stopped before mutation, so the
failure did not alter plugin state.

The fix adds a narrow read adapter at the COM-6 workflow boundary. A typed projection is converted
through `com.ContextProjection`, preserving the existing bounded LLM context and excluding raw time
segments, samples and paired-envelope evidence. Map-form projections remain accepted.

The regression test proves that typed source-only projections are accepted as `ready|partial` and
that raw evidence keys are absent from the adapted context.

## Phase-Two Payload Optimization

The first performance optimization retained the two-phase semantic order and all existing plan,
reachability, confirmation and execution gates. It changed only the phase-two LLM boundary:

- full COM workflow evidence is reduced to `semantic_effect.compressor_decision_evidence.v1`,
  filtered by the dimensions selected in phase one;
- the LLM returns `semantic_effect.compressor_control_decision.v1` instead of reconstructing the
  complete public plan;
- the host deterministically assembles and validates `semantic_effect.compressor_plan.v1`.

A normal Godot 4.6.1 product run loaded the existing `Paper Crown` clip and Pro-C 2 instance, submitted
the controller-equivalent semantic request without an explicit time window, immediately approved the
proposal, and completed COM-7 execution. No UI automation or standalone runtime substitution was used.

Measured against the preceding successful run:

| Measurement | Before | Optimized | Change |
|---|---:|---:|---:|
| phase-two input tokens | 5,653 | 2,793 | -50.6% |
| phase-two output tokens | 2,066 | 973 | -52.9% |
| phase-two LLM time | 49.866 s | 18.649 s | -62.6% |
| proposal HTTP time | 65.026 s | 32.095 s | -50.6% |
| decision evidence bytes | about 15,128 | 5,074 | -66.5% |

The optimized response succeeded on its first two-message request; no JSON repair call occurred.
The proposal remained `paired_io / ready`, `waiting_confirmation`, and zero-write. Approval remained
`executed`, `mutation_performed=true`, controller `atomic=true`, parameter audit `pass`, non-target
parameters unchanged, and post-action `change_delta / ready`. Confirmation execution remained about
5.4 seconds because its two real paired captures were intentionally outside this optimization.

## Acceptance

COM-6 is accepted as a planning-only implementation. It demonstrates that the ordinary Agent can
identify the current processor, select semantic axes, acquire real COM evidence, disclose only
reachable controls, produce a deterministically checkable absolute plan and stop with live plugin
state unchanged.

The next phase must be opened separately. This validation grants no mutation authority and does not
change the frozen COM-6 boundary.

## Verification Commands

The final implementation audit passed:

- `go test ./... -count=1`
- `go vet ./internal/chat ./internal/com ./internal/semanticeffect ./internal/workflows/plugingrabber ./internal/mixboard ./cmd/vitagent`
- Python bytecode compilation for the COM-6 smoke and its compatibility-matrix dependency
- PowerShell parser validation for `run_vit_product_path_smoke.ps1`
- JSON parsing for `compressor_semantic_cases.json`
- `git diff --check`

Running `go vet` with `internal/harness` included retains one pre-existing warning at
`internal/harness/harness.go:5188` for the Windows `MapViewOfFile` pointer conversion. Git history
attributes that line to commit `a13419ef` from 2026-06-16. COM-6 neither introduced nor changed that
bridge code; its harness behavior is covered by the passing full Go test suite and product smoke.
