# Free-State CCB Observation Protocol v1

## Purpose

This protocol gives the ordinary-Agent path a bounded, repeatable, read-only way to ask CCB for observation evidence. The top-level LLM remains the decision owner. CCB only catalogs semantic views and assembles requested evidence; it never chooses a processor, parameter, treatment, or musical action.

The fixed B2/B3/B4 capability path is unchanged. Its Context Manifests, readiness gates, deterministic solvers, proposals, confirmation, execution, and verification remain capability-owned.

## Tools

- `ccb.observation_catalog` returns `ccb_observation_catalog.v1`. Entries describe questions answered, target kinds, temporal resolution, availability, cost/latency, quality ceiling, limitations, and dependencies.
- `ccb.observation_request` returns `ccb_observation_bundle.v1`. It can create an authoritative MixBoard observation or reuse an explicit `observation_id`, then read only stable summary/projection keys.

Both tools are `RiskDirect`, set `mutates_project=false`, require no confirmation, and carry no mutation authority.

For an abstract acoustic request, the first `needs_action` is invalid until at least one model-requested `ccb.observation_request` has returned a usable `ready` or `partial` bundle. A usable bundle must be read-only, carry no mutation authority, and contain at least one view. Failed, empty, or malformed bundles do not authorize a processor decision. Explicit parameter commands remain on the separate typed-control path and do not start this free-state requirement.

When the model emits both `free_state.requested_view_ids` and a `ccb.observation_request`, their normalized view sets must match exactly. The runtime rejects added, removed, replaced, malformed, or duplicate view IDs; it never supplies a default view or infers one from user wording. If the model omits the tool call or emits legacy `mix.observe`, the runtime may materialize one CCB request using exactly the model's list and nothing else.

## Semantic Views

v1 catalogs these stable view IDs:

- `project.structure`
- `project.change_delta`
- `track.basic_energy`
- `track.time_dynamics`
- `track.timbre_frequency`
- `track.peak_structure`
- `track.activity_structure`
- `track.frequency_time_events`
- `track.transient_structure`
- `track.band_dynamics`
- `track.stereo_space`
- `mix.multitrack_relationship`
- `mix.frequency_relationship`
- `mix.masking_relationship`
- `processor.identity_and_controls`
- `processor.behavior`
- `processor.change_delta`
- `comparison.before_after`

`mix.masking_relationship` remains explicitly deferred. `processor.identity_and_controls` can expose COM processor scope, but a live semantic control surface still requires the separate read-only processor inspector; CCB v1 must report this as partial rather than infer controls.

`project.change_delta` is the deterministic Shadow Project engineering-change
view. It reports the latest project change receipt and refresh scopes; it does
not establish an acoustic outcome and must not be used as a substitute for a
fresh acoustic view.

The five DOM-backed `track.*` views are acoustic questions, not processor-family labels. CCB reads a DOM dimension only when the LLM explicitly requests its view, and a view response contains only that dimension. CCB has no natural-language keyword rule that chooses a view or processor on the LLM's behalf.

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

Every `ccb_observation_bundle.v1` also carries an embedded `ccb_observation_receipt.v1`:

- `requested_by` is `model` for an Agent/LLM tool request and `caller` for a direct non-agent caller;
- `model_requested_view_ids` preserves the raw view list received from the model/tool call;
- `actual_executed_view_ids` records the canonical acoustic view IDs actually sent to the CCB source-read path;
- `view_set_matches` must be true for an executable free-state observation, proving that the server did not add, remove, replace, or infer a view;
- `scope` records the authoritative scope derived from the requested view set and target binding;
- `freshness` is copied from the observation binding; and
- `rejection_reasons` records malformed, unknown, unavailable, stale, budget, or execution refusal details.

An empty, duplicated, malformed, or unknown view request is rejected. The server never defaults an
empty request to `project.structure`. A rejected receipt is still returned when observation
materialization or source reads fail, so refusal remains auditable without fabricating evidence.

## Decision-Phase Boundary

Free-state reasoning separates three roles:

- `processor_selection` uses source and mix evidence to choose the next treatment family. Source-only macro dynamics can support this decision even though they cannot identify compressor behavior or parameter values.
- `processor_materialization` belongs to the governed EQ/compressor workflow, which inspects the live processor, freezes a reachable proposal, confirms it, executes it, and reads it back.
- `post_action_evaluation` requires a fresh CCB observation before the original intent can be declared satisfied.

Missing paired processor I/O before a processor action is expected and is not a selection-stage blocker. Missing micro-transient detail limits confidence and must be carried as a conservative transient-preservation constraint. COM's `can_support_semantic_planning` field describes downstream compressor behavior/parameter planning; it is not a global treatment-family selection gate.

After family selection, ordinary abstract EQ planning must retain the exact CCB observation binding and use `basis=observation|both`; `user_report/not_needed` alone is not admissible. Compressor evidence mode and dimensions remain model-selected, while deterministic code materializes that request. Compressor planning may use only the disclosed live identity card, control brief, and observation evidence as processor-specific authority; product or vendor identity is not a behavioral prior.

Relationship views define their own observation scope. A `mix.*` request cannot be narrowed to `selected_track`; a request combining track and mix views uses `full_project_with_focus_track`.

## Disclosure Boundary

CCB v1 may reuse compact MixBoard, MOM, TIM, TOM-adjacent project identity, COM, and DOM projections. It must not disclose raw PCM, audio buffers, waveform/spectrogram arrays, full DAD packages, evidence blobs, or internal persistence paths. Arrays and recursive values are bounded before disclosure.

## Non-Goals

This protocol does not choose processors, plan parameters, mutate the project, add DAD metrics, or change fixed capability manifests. The persistent free-state loop consumes this read-only protocol, while all processor mutation remains in governed typed workflows.
