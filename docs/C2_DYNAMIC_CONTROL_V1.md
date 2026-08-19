# C2 Dynamic Control v1

## Responsibility

C2 (`fine_mix.dynamic_control.v1`) is an independent fine-mix capability for
compression, limiting, gate/expander, de-essing, transient shaping, and
multiband dynamics. It is not a linear successor to C1, C3, or C4.

A C1 `defer_dynamic_processing` classification may become an optional C2
referral. It contributes revision-bound rationale and evidence references only;
it cannot choose a processor family, plugin, coverage axis, control, or action.
C2 remains runnable when C1 has never run or is stale.

The full-project observation stays authoritative but its model-facing context
is a bounded project-digest projection. Raw clips, sample buffers, waveforms,
and other high-volume media payloads remain reachable by their observation
receipt; they are not copied into the C2 decision prompt. This keeps large
projects within the model context budget without changing which project tracks
the fixed project observation covers.

## Reused Leaves

- Broadband compressor: COM-6 planning and COM-7 execution.
- Other admitted dynamic families: `semantic_dynamic_workflow`.
- Exact instances: PCA admission before planner/materializer entry.
- Parameter writes: existing family-specific Typed Executors only.

## C2 Lifecycle

```text
current project + optional peer referral
-> fixed C2 full-project observation pack
   (project MOM digest across every observable track)
-> one model-selected target's dynamic CCB confirmation views
-> C2 decision: no_action_needed | action_proposed | concrete observation block
-> only after action_proposed: PCA resolution of an existing exact instance
   or PCA-admitted local dynamic-plugin selection
-> separate plugin-load confirmation -> rack.add_node -> exact live-instance
   PCA/control-surface admission, with rollback on any failure
-> existing semantic leaf planning and separate parameter confirmation
-> existing typed transaction/readback/rollback
-> Shadow/Mix Board refresh + fresh CCB observation + project delta
-> C2 terminal Mixboard record
```

C2 is a fixed A-F mixing capability, not a free-state improvement-proposal
loop. Its observation scope, evidence gates, terminal states and confirmation
boundaries are C2-owned. `no_action_needed` is a successful terminal outcome:
it means the fixed project observation found no justified dynamic treatment and
does not invite a speculative write.

C2's first vertical slice selects one project target per turn. Multiple targets
remain a later batching concern. A missing loaded instance is not an observation
failure or a `deferred` result: C2 presents only locally available,
PCA-admitted candidates, loads the selected binary only after a dedicated
confirmation, then revalidates PCA against the exact live instance before any
parameter planner or Typed Executor can run.

No raw parameter fallback, tag-based plugin admission, automatic retuning, or
new observer/recognizer/controller is introduced by C2. C2 does not write or
resume the free-state `improvement_proposal` / `needs_experiment` loop; its
post-action refresh, CCB receipt, delta and `needs_review` terminal record stay
within the C2 session.

## Product Smoke

The C2 smoke source is the full `B4完成后` project package, not `p01` and not a
synthetic single-track fixture. The runner copies the complete directory that
contains `B4完成后.vit` into an artifact-owned isolated package before opening
it. It never edits the source project, injects a plugin, or assumes that C2
must write a parameter.

The smoke accepts two valid terminal paths:

- `no_action_needed`: fixed full-project C2 observation concludes that no
  dynamic treatment is justified and proves a zero-write terminal result.
- An action path: choose an existing PCA-admitted live instance or a
  PCA-admitted load candidate, confirm plugin loading separately when needed,
  then confirm the existing typed dynamic leaf. The final receipt must include
  a typed-executor receipt, a fresh CCB observation, project-change delta, and
  a `needs_review` Mix Board record.

The default product-path invocation is:

```powershell
scripts\run_vit_product_path_smoke.ps1 -C2DynamicControlAgentOnly
```
