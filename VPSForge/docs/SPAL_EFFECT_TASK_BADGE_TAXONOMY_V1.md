# SPAL Effect Task Badge Taxonomy v1

Status: planning and authoring guidance only. This document does not create a
Credential, Catalog entry, SPAL route, executable schema, or permission to
load a plug-in into Vit.

## Four separate layers

| Layer | Purpose | May SPAL route on it? |
| --- | --- | --- |
| Classification directory | Browse and organize candidate work only. Examples: `spectral_processing`, `dynamics_processing`, `time_based_effects`, `nonlinear_processing`. | No |
| Routeable task badge | A task a user may independently ask for, backed by a local, verified Credential. | Yes, exact badge only |
| Badge feature matrix | The conformed standard actions and optional feature conditions for one plugin under one badge. | Only after exact badge selection |
| VPS vendor special capability | Vendor-specific extension retained in a VPS. It can be summarized when explicitly relevant. | No, unless a separate schema and conformance later create a Credential-backed badge/action |

Directory names and shared implementation components are never parent badges.
For example, neither `time_based_effects` nor `dynamics.v1` may be passed to a
SPAL Resolver as a routeable task.

## Required routing boundary

```text
Natural language
  -> Agent/LLM reasoning and clarification
  -> capability plane forms an explicit task
  -> exact task badge + required feature conditions
  -> SPAL exact Credential/Catalog query
  -> selected VPS mapping
  -> Proposal, authorization, execution, fresh readback, rollback
```

For example, a request to create reverb space becomes:

```json
{
  "task": "reverb.create_space",
  "required_badge": "reverb.v1"
}
```

SPAL may select only a locally verified `reverb.v1` Credential. It must not
query `time_based_effects` and guess between Reverb and Delay. If the user asks
only for “more space”, the Agent may reason, compare alternatives, or ask a
clarifying question before it forms the explicit task; this taxonomy does not
restrict that reasoning stage.

## Candidate roadmap — not current routing authority

| Classification directory | Candidate routeable task badge | First reference plugin |
| --- | --- | --- |
| `spectral_processing` | `equalizer.v2` | FabFilter Pro-Q 3 |
| `dynamics_processing` | `compressor.v1` | FabFilter Pro-C 2 |
| `dynamics_processing` | `gate_expander.v1` | FabFilter Pro-G |
| `dynamics_processing` | `de_esser.v1` | FabFilter Pro-DS |
| `dynamics_processing` | `limiter.v1` | FabFilter Pro-L 2 |
| `time_based_effects` | `reverb.v1` | FabFilter Pro-R |
| `time_based_effects` | `delay.v1` | FabFilter Timeless 3 |
| `nonlinear_processing` | `saturation_distortion.v1` | FabFilter Saturn 2 |

Every entry remains a candidate until that exact task has local human semantic
confirmation and task-scoped conformance. A snapshot, parameter label,
Plugin Learning result, plugin name, or FXM render alone does not grant it.

## Deliberate initial consolidation

- Gate and Expander share `gate_expander.v1`.
- Saturation and Distortion share `saturation_distortion.v1`.
- Multiband dynamics is not a badge in v1. It is initially expressed as
  `compressor.v1` plus `required_features: ["multiband", "per_band_dynamics"]`.

A new `multiband_dynamics.v1` Proposal is appropriate only if experiments show
an independently requested user task and a vendor-neutral, repeatable
conformance contract that cannot be expressed this way.

Compressor, Gate/Expander, De-esser, Limiter, Reverb and Delay must retain
separate routeable badges even if schemas, execution code, DSP primitives or
tests share components.

## Shared components are not badges

`dynamics_common` may share detector, threshold, ratio/curve, attack, release,
knee, range, sidechain, channel-link, lookahead, input/output gain, bypass,
state and rollback. `time_effect_common` may share wet/dry, tempo sync,
stereo, tail, freeze/hold, bypass, state and rollback.

Neither name may appear as a routeable badge or SPAL query key.

## Badge admission rule

A routeable badge requires all of the following for that exact local plugin
version and format:

1. an independently requested user task;
2. vendor-neutral minimum functions and action schema;
3. repeatable, task-scoped conformance;
4. exact SPAL selection without category inference;
5. human semantic confirmation;
6. local write/fresh-readback, state restoration, behavior tests and rollback;
7. explicit Credential issuance under the separate installation authority.

One plugin may hold multiple badges, but each needs independent conformance.
An awarded badge does not award its untested feature matrix rows or vendor
special capabilities.

## VPS Forge preflight output

Preflight produces three badge-review artifacts in each isolated workspace:

- `candidate_task_badges.json`: `agent-inferred`, `candidate_not_granted`, and
  never routeable;
- `required_feature_gaps.json`: feature-condition gaps, not independent badges;
- `badge_conformance_skeleton.json`: planned audit skeleton, never a
  Credential.

Preflight may prepare evidence for a roadmap candidate supplied by the
authoring plan. It never upgrades an observation to `conformed`, changes a
Catalog, enables SPAL, or issues a Credential.
