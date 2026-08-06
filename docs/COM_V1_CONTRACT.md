# Compression Observation Model v1 Contract

Status: frozen design; COM-1 `source_only` foundation implemented
Date: 2026-08-04
Schema: `com.projection.v1`

Implementation note: `agent/internal/com` implements the typed schema, deterministic projection
ID, `source_only` macro projection, trust/readiness, per-dimension identifiability, and compact
context boundary. `paired_io`, `change_delta`, product-path wiring, semantic planning, and C2 remain
unimplemented by design.

## 1. Purpose

COM is the Compression Observation Model. It is a peer observation projection beside MOM, TIM,
TOM, DOM, EPM, and FXM. COM converts typed acoustic evidence into a compact description of source
dynamics and, when paired evidence exists, the behavior of one broadband compressor scope.

COM observes compression behavior. It does not identify a plug-in from its GUI, discover controls,
choose settings, execute parameter writes, or decide what sounds artistically correct.

The v1 supported processor boundary is the existing generic single-band, single-stage broadband
compressor boundary. Limiters, multiband compressors, dynamic EQ, de-essers, gates/expanders,
clippers, spectral dynamics, and unresolved multi-stage or multi-path processors are out of scope.

## 2. Ownership Boundary

| Layer | Owns | Must not own |
| --- | --- | --- |
| DAD / render evidence | signal capture, envelopes, event summaries, revisions, tap identity, quality evidence, evidence refs | musical judgement or compressor parameters |
| FXM | generic bypass/processed chain identity, same-source/window/format comparability, scalar level/spectral/latency deltas | compressor-specific time behavior or semantic intent |
| MOM | project/mix relationships and general post-action A/B presentation | compressor-specific gain-action analysis |
| COM | typed source dynamics, paired input/output compression behavior, identifiability, compact compressor context | topology recognition, parameter planning, mutation, success claims |
| compressor recognizer/controller | live topology and deterministic physical/enum control | acoustic diagnosis or semantic policy |
| future compressor semantic planner | intent-to-axis decision and bounded parameter proposal | evidence fabrication or direct generic writes |
| C2 | workflow orchestration using observation, planner, confirmation, execution, and verification | redefining COM or the controller |

COM may consume an FXM comparability receipt and may cite MOM/FXM evidence. It must not rebuild
FXM's generic chain delta or MOM's general A/B result. COM owns only compression-specific derived
facts that require aligned envelope/event evidence.

## 3. Projection Modes

Every projection declares exactly one `mode`.

### `source_only`

Input is one source/pre-compressor observation. This mode describes the material presented to a
possible compressor: activity, peak/body relation, event density, macro variance, and available
time scales.

It must not claim gain reduction, threshold crossings, ratio, attack, release, pumping, makeup
gain, or audible compressor effect. It may report that a decision axis is not identifiable.

### `paired_io`

Input is an aligned pair for one frozen compressor scope: pre-compressor input and post-compressor
output over the same material window. This is the primary mode for observing compression behavior.

The pair must prove scope identity, source/window identity, sample format compatibility, tap order,
determinism, and latency alignment. A bypass/processed FXM pair can supply part of this proof only
when the processed scope is exactly the selected compressor and both sides also expose the typed
time evidence required by COM.

### `change_delta`

Input is two comparable `paired_io` COM projections, normally before and after a compressor
control-state change. It reports how observed compressor behavior changed. Both children remain
the evidence of record.

This mode is not the generic MOM before/after delta and not FXM bypass/processed delta. It requires
the same source, window, input/output taps, render mode, sample format, latency policy, processor
scope, and analyzer version. The control-state or render revision must change. If the input signal
is not equivalent within a declared tolerance, behavioral attribution is not identifiable.

## 4. Normative Projection Shape

The following shape is normative at field and responsibility level. Concrete Go types may split
the objects, but names and meanings must remain stable.

```json
{
  "schema_version": "com.projection.v1",
  "com_version": "v1",
  "projection_id": "com_<stable hash>",
  "mode": "source_only | paired_io | change_delta",
  "status": "ready | partial | missing | stale | suspect | approximate",
  "observation_id": "optional parent observation",
  "mix_session_id": "session identity",
  "target_ref": {},
  "processor_scope": {},
  "conditions": {},
  "evidence_inputs": {},
  "source_dynamics": {},
  "gain_action": {},
  "transient_response": {},
  "recovery_motion": {},
  "level_effect": {},
  "stereo_behavior": {},
  "trigger_relation": {},
  "behavior_change": {},
  "identifiability": {},
  "trust_quality": {},
  "evidence_refs": [],
  "limitations": [],
  "llm_context": {},
  "generated_at": "RFC3339"
}
```

Absent mode-inapplicable typed projections are omitted. A present typed projection always carries
its own `status`, `evidence_refs`, and `limitations`; an empty object must not imply readiness.

## 5. Identity And Conditions

`processor_scope` contains:

- target track/channel IDs;
- selected plug-in instance ID and stable position/scope identity;
- topology classification and topology generation when available;
- chain hash and processor state hash when available;
- explicit `single_band_broadband` support classification.

`conditions` contains:

- source revision, clip revision, project cut reference, and material reference;
- start/end sample and start/end seconds;
- sample rate, channel count, and channel layout;
- render mode and deterministic flag;
- input and output tap identities;
- latency samples, latency-compensation method, tail policy, and analyzer version;
- analysis resolutions actually used.

Time positions are sample-index authoritative. Seconds are presentation values. Two observations
are not declared aligned from rounded seconds alone.

## 6. Typed Subprojections

### `source_dynamics`

Available in all modes. It describes observable input material, not compressor action.

Required v1 facts when evidence permits:

- analyzed duration and active/silent coverage;
- peak, RMS or active RMS, crest distribution, and macro level range;
- event count/density and inter-event interval distribution;
- transient-to-body contrast distribution;
- section/phrase variance at the available macro scale;
- time-scale coverage and resolution limitations.

### `gain_action`

Only behavior-identifiable in `paired_io` or its `change_delta` child projections. It describes the
aligned input-to-output gain-difference envelope.

Facts may include active-action coverage, reduction-depth distribution, maximum/median/percentile
gain action, event consistency, and gain-action duty cycle. Values are observed effective behavior,
not a plug-in gain-reduction meter and not inferred control values.

### `transient_response`

Describes leading-edge change across matched events: peak attenuation, transient-to-body contrast
change, onset overshoot change, and event-to-event consistency. It may classify only bounded
observations such as `more_preserved`, `more_rounded`, `mixed`, or `indeterminate`.

It must not report a numeric attack parameter. Millisecond behavior estimates require analysis
resolution and event coverage sufficient for the claimed bound.

### `recovery_motion`

Describes post-event gain return: observed return-time bands, incomplete recovery before the next
event, sustained gain movement, and periodic modulation candidates. `pumping_candidate` is allowed
only as an evidence-tagged observation with confidence and alternatives; it is never a diagnosis
from a single crest or RMS delta.

It must not report a numeric release parameter or auto-release state.

### `level_effect`

Describes paired scalar effects already compatible with FXM/MOM: input/output RMS, active RMS,
peak, crest, loudness when genuinely measured, and their deltas. When sourced from FXM, COM stores
the FXM projection/evidence reference and does not recompute a competing canonical delta.

Makeup/output gain can obscure reduction in scalar metrics. Therefore level effect alone never
makes `gain_action`, `transient_response`, or `recovery_motion` identifiable.

### `stereo_behavior`

Optional for stereo evidence. Describes left/right action asymmetry, image/correlation change, and
linked-motion consistency. It must not infer the plug-in's channel-link setting. Mono input yields
`not_applicable`, not `missing`.

### `trigger_relation`

Optional. Describes evidence that gain action co-varies with bounded input features, such as low-
frequency energy. It may support a detector-focus hypothesis but must not infer a sidechain filter
frequency or prove causation from ordinary program material.

### `behavior_change`

Present only in `change_delta`. It contains per-subprojection changes and a list of dimensions that
became stronger, weaker, unchanged within tolerance, mixed, or non-comparable. It must preserve
the two child projection IDs and must not collapse non-comparable facts into zero delta.

## 7. Time-Scale Contract

COM v1 uses named scales; each fact declares the actual window/hop and event count behind it.

| Scale | Intended evidence | Typical question |
| --- | --- | --- |
| `micro_transient` | sample-aligned onset/peak envelope, normally sub-5 ms hop | was the leading edge caught or preserved? |
| `short_gain_motion` | roughly 5-200 ms envelope behavior | how quickly did action build and begin to return? |
| `event_recovery` | roughly 0.2-3 s event/inter-event context | did gain recover between hits or phrases? |
| `macro_program` | multi-second sections/full selection | how variable and active is the material overall? |

These ranges are routing guidance, not claims that current evidence supports them. The current DAD
five-second `time_segments` support only `macro_program`. They cannot make transient or recovery
behavior ready. Implementations must derive facts from the finest evidence artifact before compact
projection; they must not pass raw traces to the LLM.

## 8. Alignment And Comparability Gates

`paired_io` is behavior-ready only when all required gates pass:

1. exact target and supported processor scope;
2. same source/material revision and exact sample window;
3. same sample rate, channel layout, render mode, and analyzer version;
4. declared pre-processor input tap and post-processor output tap in correct order;
5. deterministic renders or one deterministic dual-tap render;
6. measured/declared latency and successful sample alignment;
7. sufficient non-silent coverage and no NaN/Inf;
8. evidence refs for both sides;
9. required time resolution for each promoted typed fact;
10. no unrelated state change inside the frozen processor scope.

`change_delta` additionally requires comparable child schema versions, same processor/topology,
same conditions and analysis configuration, distinct child projection IDs, changed state/render
revision, and equivalent input evidence. Failed attribution gates make behavior change `suspect` or
`stale`, even if processed-output scalar metrics differ.

## 9. Readiness And Trust

Top-level status follows the requested mode, not the union of every optional field:

- `ready`: all required mode gates pass and every required typed projection for that mode is
  identified at its declared scale; for `source_only`, this means source dynamics, not compressor
  behavior;
- `partial`: trustworthy evidence exists, but one or more requested behavior dimensions are not
  identifiable or only a lower time scale is covered;
- `approximate`: usable bounded estimates exist but their method is explicitly approximate;
- `suspect`: evidence exists but quality, alignment, scope, or attribution is untrusted;
- `stale`: identity or revision no longer matches the requested cut/state;
- `missing`: required evidence is absent.

`trust_quality` contains at least:

```text
overall_status
mode_gates
coverage
same_source / same_window / same_format
tap_order_valid / deterministic / latency_aligned
input_equivalent (change_delta)
can_support_source_description
can_support_behavior_observation
can_support_semantic_planning
can_support_post_action_evaluation
blocked_reasons / approximate_fields / suspect_fields / stale_fields / missing_fields
```

`can_support_semantic_planning` means COM can inform a future planner. It grants no execution
authority. `source_only` may support source description but never behavior observation or post-
action evaluation.

## 10. Identifiability Contract

Every promoted behavior dimension has an identifiability row:

```json
{
  "dimension": "transient_response",
  "status": "identified | bounded | not_identifiable | not_applicable",
  "basis": "paired_trace | paired_events | aggregate_only | source_only",
  "confidence": 0.0,
  "resolution": {"window_ms": 0.0, "hop_ms": 0.0, "event_count": 0},
  "supports": ["semantic_axis:transient_timing"],
  "does_not_support": ["parameter_value:attack_ms"],
  "limitations": []
}
```

COM v1 never identifies numeric threshold, ratio, knee, attack, release, lookahead, range, makeup,
mix, detector filter, or channel-link parameter values from audio. Those values come only from the
live deterministic control surface. COM observes effective behavior and explicitly records when
different parameter combinations are acoustically non-identifiable.

## 11. Compact LLM Context

`llm_context` follows the established projection pattern:

- one short summary;
- no more than 24 compact facts total;
- no more than 6 facts per typed subprojection;
- identifiability and trust facts precede descriptive tags;
- evidence refs and limitations remain structured;
- `do_not_include_raw_package: true` is mandatory.

Forbidden context payloads include raw samples, waveform/envelope arrays, per-frame gain traces,
event lists, spectrogram tiles, render paths, shared-memory references, full plugin parameter dumps,
and full artifact history. Compact quantiles, counts, bounded labels, and evidence refs are allowed.

## 12. Non-Goals And Safety Invariants

COM v1 does not:

- decide whether or how much to compress;
- translate natural language into controls;
- recognize or bind plug-in parameters;
- execute, restore, or verify parameter writes;
- claim audible quality or user acceptance;
- absorb limiter or multiband behavior;
- expose raw audio to the LLM;
- treat output loudness matching as proof of good compression;
- infer hidden parameter values from ambiguous audio behavior.

These boundaries are normative for subsequent implementation targets.
