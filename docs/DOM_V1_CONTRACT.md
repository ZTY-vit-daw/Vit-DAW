# Dynamics Observation Model (DOM) v1 Contract

Status: implemented observation infrastructure; `source_only` is materialized from bounded DAD
evidence, while `paired_io` and `change_delta` are reserved and fail closed.

## 1. Purpose

DOM is the Dynamics Observation Model. It is a peer projection beside MOM, TIM, TOM, FXM, and
COM. DOM turns typed acoustic evidence into bounded, family-neutral descriptions that the LLM can
request through CCB when it needs to understand source dynamics.

DOM does not choose a processor family, recommend or load a plug-in, resolve PCA eligibility,
inspect a vendor UI, map an intent to controls, generate parameter IDs, or mutate the project.
The top-level LLM remains responsible for deciding which observations to request and, in a later
phase, which treatment family to propose. PCA and typed controllers remain the execution authority.

`DOM` previously appeared in planning documents as Delivery Observation Model. That reserved name
is now `DelOM`; DOM means Dynamics Observation Model exclusively.

## 2. Ownership Boundary

| Layer | Owns | Must not own |
| --- | --- | --- |
| DAD / feature evidence | signal facts, time rows, band summaries, freshness, quality, evidence refs | family selection or control semantics |
| DOM | compact source dynamics, dimension readiness, trust, limitations, future governed behavior boundary | processor choice, PCA, controls, mutation |
| CCB | neutral view catalog, explicit request, bounded disclosure | automatic view choice or prompt-to-view routing |
| LLM | inspect the catalog, request evidence, reason about the user's goal | identifiers outside governed candidates or parameter mapping |
| PCA resolver | eligible candidates, exact identity, family and coverage authority | acoustic diagnosis |
| typed controller | certified intent-to-axis mapping, transaction, readback, restore, verification | pre-family acoustic reasoning |

COM continues to own the existing single-band broadband-compressor observation contract. DOM v1
must not reinterpret COM evidence as limiter, gate/expander, de-esser, transient-shaper, or
multiband-processor behavior.

## 3. Projection Modes

Every projection declares one mode.

### `source_only`

Implemented in v1. It describes the unprocessed target without binding a processor. It never
contains `processor_scope`, plug-in identity, topology generation, processor state hash, raw time
rows, or parameter identity. Missing fine-grained evidence remains `partial` or `missing`.

### `paired_io`

Reserved for a future aligned pre/post observation of one already-selected, governed processor
scope. v1 returns `missing` and `can_support_behavior_observation=false`. A bound supported family
is required even to represent the reserved scope.

### `change_delta`

Reserved for a future comparison of compatible `paired_io` projections before and after a
governed state change. v1 returns `missing` and
`can_support_post_action_evaluation=false`. It cannot be synthesized from scalar before/after
levels.

Unknown modes and families outside the frozen boundary return `unsupported`.

## 4. Source-Only Dimensions

| Dimension | v1 evidence | Required refusal boundary |
| --- | --- | --- |
| `peak_structure` | sample peak, headroom, crest, bounded segment distributions | sample peak does not prove true peak or clipping |
| `activity_structure` | declared active/low/silent segment coverage, low-energy runs, and bounded noise-floor percentiles when materialized | coarse activity does not prove noise floor or a control threshold |
| `frequency_time_events` | bounded time-localized frequency event rows when materialized; whole-window bands remain labelled as such | whole-window bands do not become sibilance events |
| `transient_structure` | bounded onset/body/sustain event facts when materialized; macro crest remains a limited fallback | macro crest does not prove onset, attack/body, sustain, or decay behavior |
| `band_dynamics` | bounded per-band time distributions and frame-envelope crest distributions when materialized; whole-window bands remain limited | whole-window band energy does not prove multiband dynamics |

The generic L3 DAD analyzer now materializes bounded, source-identity-bound fine evidence for
noise-floor percentiles, time-localized frequency events, transient envelope events, and per-band
time/crest distributions. MixBoard forwards those facts without adding inference. When a producer
does not supply a complete bounded fact, DOM keeps the affected dimension partial or missing and
retains the explicit limitation.

## 5. Status And Trust

Projection and dimension statuses are `ready`, `partial`, `missing`, `stale`, `suspect`, or
`unsupported`. Root `stale`, `suspect`, `missing`, or `unsupported` status cannot be widened by a
child dimension.

`trust_quality` separates four permissions:

- `can_support_source_description`;
- `can_support_family_selection`;
- `can_support_behavior_observation`;
- `can_support_post_action_evaluation`.

Only the first two may become true in `source_only`. Stale or suspect source evidence disables
them. v1 never sets either behavior permission true.

## 6. CCB Views

CCB exposes five neutral, explicitly requested views:

| View | DOM dimension |
| --- | --- |
| `track.peak_structure` | `peak_structure` |
| `track.activity_structure` | `activity_structure` |
| `track.frequency_time_events` | `frequency_time_events` |
| `track.transient_structure` | `transient_structure` |
| `track.band_dynamics` | `band_dynamics` |

View IDs and catalog questions describe observable acoustics, not processor families. A request
for one view discloses only its corresponding DOM dimension and readiness. Other DOM dimensions
are not attached. A non-DOM request does not read or disclose `observation.dom_projection`.

There is no natural-language keyword table, prompt rule, or deterministic user-text-to-view map.
The LLM receives the neutral catalog, chooses which view or views to request, reads the bounded
result, and retains every missing/partial limitation in its reasoning.

## 7. Frozen Future Processor Boundary

Future governed `paired_io` and `change_delta` work may cover these already-recognized controller
families after family selection:

- limiter;
- gate / expander;
- de-esser;
- transient shaper;
- multiband dynamics.

This list constrains post-selection evidence compatibility. It must not appear in the pre-family
CCB view catalog and is not a routing table.

Broadband Compressor remains COM-owned. Spectral Dynamics remains inspect-only. Clipper cannot
enter the limiter boundary. Reverb and delay are outside DOM and will require their own observation
ownership decision when added.

## 8. MixBoard And Persistence

Normal observations materialize `observation.dom_projection`, include it in the catalog and digest,
persist it in the canonical observation packet, expose it through `mix_read`, and add a bounded
context projection to the Context Pack. Raw time/event/spectral payloads remain in evidence
storage. The specialized bounded band/stereo projection does not auto-attach DOM.

## 9. Non-Goals For TODO4

TODO4 does not extend FreeStateDecision families, add semantic intent schemas, query PCA,
select exact plug-in identifiers, connect typed dynamic controllers, write parameters, or implement
the post-action loop. Those are later stages built on this observation contract.

## 10. Acceptance Gates

- source-only projection is deterministic and contains no processor or parameter identity;
- unavailable evidence stays partial/missing;
- reserved modes fail closed;
- stale evidence cannot support selection;
- catalog/read/context/persistence round trips preserve DOM identity;
- each CCB view discloses only the requested dimension;
- unrequested DOM views are not read or loaded;
- view catalog text contains no supported-family routing labels;
- compact context excludes raw rows, plug-in IDs, topology generation, and state hashes.
