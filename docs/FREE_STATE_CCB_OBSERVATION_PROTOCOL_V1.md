# Free-State CCB Observation Protocol v1

## Purpose

This protocol gives the ordinary-Agent path a bounded, repeatable, read-only way to ask CCB for observation evidence. The top-level LLM remains the decision owner. CCB only catalogs semantic views and assembles requested evidence; it never chooses a processor, parameter, treatment, or musical action.

The fixed B2/B3/B4 capability path is unchanged. Its Context Manifests, readiness gates, deterministic solvers, proposals, confirmation, execution, and verification remain capability-owned.

## Tools

- `ccb.observation_catalog` returns `ccb_observation_catalog.v1`. Entries describe questions answered, target kinds, temporal resolution, availability, cost/latency, quality ceiling, limitations, and dependencies.
- `ccb.observation_request` returns `ccb_observation_bundle.v1`. It can create an authoritative MixBoard observation or reuse an explicit `observation_id`, then read only stable summary/projection keys.

Both tools are `RiskDirect`, set `mutates_project=false`, require no confirmation, and carry no mutation authority.

## Semantic Views

v1 catalogs these stable view IDs:

- `project.structure`
- `track.basic_energy`
- `track.time_dynamics`
- `track.timbre_frequency`
- `track.stereo_space`
- `mix.multitrack_relationship`
- `mix.frequency_relationship`
- `mix.masking_relationship`
- `processor.identity_and_controls`
- `processor.behavior`
- `processor.change_delta`
- `comparison.before_after`

`mix.masking_relationship` remains explicitly deferred. `processor.identity_and_controls` can expose COM processor scope, but a live semantic control surface still requires the separate read-only processor inspector; CCB v1 must report this as partial rather than infer controls.

## Bundle Contract

Every bundle includes:

- observation/session identity;
- actual target binding;
- project UUID, epoch, revision, and state hash when available;
- freshness status and observation time;
- requested semantic views;
- bounded evidence values and evidence references;
- limitations and per-view omission states;
- exact disclosure byte count and budget;
- `read_only=true` and `mutation_authority=false`.

Unsupported view IDs are `forbidden`. Missing/deferred evidence is `unavailable`, stale evidence is `stale`, and valid evidence that does not fit is `omitted_budget`.

## Decision-Phase Boundary

Free-state reasoning separates three roles:

- `processor_selection` uses source and mix evidence to choose the next treatment family. Source-only macro dynamics can support this decision even though they cannot identify compressor behavior or parameter values.
- `processor_materialization` belongs to the governed EQ/compressor workflow, which inspects the live processor, freezes a reachable proposal, confirms it, executes it, and reads it back.
- `post_action_evaluation` requires a fresh CCB observation before the original intent can be declared satisfied.

Missing paired processor I/O before a processor action is expected and is not a selection-stage blocker. Missing micro-transient detail limits confidence and must be carried as a conservative transient-preservation constraint. COM's `can_support_semantic_planning` field describes downstream compressor behavior/parameter planning; it is not a global treatment-family selection gate.

Relationship views define their own observation scope. A `mix.*` request cannot be narrowed to `selected_track`; a request combining track and mix views uses `full_project_with_focus_track`.

## Disclosure Boundary

CCB v1 may reuse compact MixBoard, MOM, TIM, TOM-adjacent project identity, and COM projections. It must not disclose raw PCM, audio buffers, waveform/spectrogram arrays, full DAD packages, evidence blobs, or internal persistence paths. Arrays and recursive values are bounded before disclosure.

## Non-Goals

This protocol does not choose processors, plan parameters, mutate the project, add DAD metrics, or change fixed capability manifests. The persistent free-state loop consumes this read-only protocol, while all EQ/compressor mutation remains in the existing governed workflows.
