# COM v1 Test Contract

Status: frozen; COM-1 through COM-5 tests implemented
Date: 2026-08-04

## 1. Test Principle

COM tests verify observable claims, comparability, and non-claims. Passing is not defined as
producing many metrics. Passing means the projection promotes only facts justified by its evidence
resolution and identity gates.

All algorithm tests use deterministic PCM or typed envelope fixtures with known expected
relationships. Product-path smoke adds real Vit/Godot evidence but does not replace deterministic
fixtures.

## 2. Required Fixture Families

| Fixture | Purpose |
| --- | --- |
| steady tone/noise with fixed gain | prove zero dynamic action and scalar level change separation |
| isolated impulses with known gain envelope | transient capture and latency alignment |
| repeated drum-like events | event consistency and inter-hit recovery |
| step/hold/release envelope | bounded attack/recovery observation |
| sustained phrase with slow gain motion | phrase-level action and macro variance |
| sparse source with silence | silence guards and active coverage |
| stereo asymmetric and linked pairs | channel action relation and mono N/A |
| makeup-compensated compression | prove scalar RMS cannot replace gain-action evidence |
| parallel wet/dry mix | prove ambiguous action is bounded/not identifiable |
| low-frequency-correlated action | optional trigger relation without causal overclaim |
| insufficient-resolution envelope | prevent false transient/recovery readiness |
| limiter/multiband/unknown scope | enforce broadband compressor boundary |

Fixtures must record sample rate, channel layout, exact sample window, input/output tap names,
latency, analyzer version, processor scope, state hash, and expected identifiability.

## 3. Source-Only Contract Tests

1. Fresh source evidence produces a stable `com.projection.v1` ID and source-dynamics facts.
2. Five-second DAD segments can make macro activity ready or partial but leave micro transient,
   gain action, and recovery behavior `not_identifiable`.
3. Source-only never sets `can_support_behavior_observation`, semantic compressor behavior, or
   post-action evaluation to true.
4. Peak/RMS/crest presence alone never creates a gain-action projection.
5. Stale source revision remains stale; missing time segments do not become approximate timing.
6. Silent or nearly silent input reports coverage limits and no invented event statistics.
7. Reordered input maps/lists produce the same stable projection where order is non-semantic.

## 4. Paired-I/O Comparability Tests

The happy-path fixture must pass all gates and produce `paired_io` behavior facts. Each following
mutation must independently block or downgrade behavior attribution:

- source revision mismatch;
- start/end sample mismatch;
- sample rate or channel-layout mismatch;
- missing/unknown input or output tap;
- reversed tap order;
- processor scope or topology-generation mismatch;
- nondeterministic render;
- missing or failed latency alignment;
- residual alignment error above tolerance;
- analyzer-version mismatch;
- NaN/Inf or zero/non-silent coverage failure;
- missing evidence ref;
- unsupported limiter, multiband, or ambiguous multi-stage scope;
- insufficient time resolution for the requested typed dimension.

One failed gate must not erase trustworthy source-only facts. It must prevent the affected behavior
subprojection from being promoted.

A repeated capture of the same frozen state must receive a new `pair_id` and `job_id` while being
allowed to retain the same `render_revision`. The capture IDs prove freshness; the render revision
proves state/content identity. Requiring the render revision to change belongs only to a real
state-change comparison in `change_delta`.

## 5. Behavior Derivation Tests

### Gain action

- known gain-envelope depth and duty-cycle quantiles are recovered within frozen tolerances;
- fixed output gain is separated from time-varying gain action;
- makeup-compensated output still reports action when aligned traces prove it;
- scalar-only input reports gain action as not identifiable;
- silence and near-zero denominators do not generate extreme false reduction.

### Transient response

- matched impulses distinguish preserved, attenuated/rounded, mixed, and unchanged-within-tolerance;
- latency-shifted but correctly aligned events match; unaligned events do not;
- event count and resolution gates are enforced;
- no fixture causes COM to emit an inferred numeric attack control value.

### Recovery motion

- known complete and incomplete returns are distinguished;
- repeated-event fixtures detect lack of recovery without calling every long decay pumping;
- periodic modulation is a candidate with confidence and alternatives, not a causal diagnosis;
- no fixture causes COM to emit an inferred numeric release or auto-release value.

### Stereo and trigger relation

- mono yields `not_applicable` stereo behavior;
- linked and asymmetric stereo fixtures produce bounded observed relations;
- band/action correlation can be reported only with aligned time evidence and never becomes an
  inferred sidechain-filter value.

## 6. Change-Delta Tests

1. Two paired projections with identical conditions, equivalent input, and changed state/revision
   produce a typed behavior delta with both child IDs.
2. Identical projection IDs or unchanged render/state revision are stale, not a zero-effect proof.
3. Changed source/window/taps/render mode/sample format/analyzer/topology are non-comparable.
4. Input-equivalence failure makes behavioral attribution suspect even when outputs differ.
5. A behavior dimension unavailable in either child remains unavailable in the delta.
6. Unchanged-within-tolerance is distinct from missing and non-comparable.
7. Generic level deltas cite FXM/MOM evidence where supplied and do not fork a conflicting
   canonical value.
8. A parameter-only state change must preserve the structural chain hash while changing both the
   processor-state hash and render revision; a topology/connection change must not preserve it.
9. The compact pre-compressor input identity hashes quantized envelope values and metadata only;
   it never embeds the frame trace in the projection or LLM context.

## 7. Trust And Identifiability Tests

- top-level readiness follows mode-required fields, not optional projection count;
- `partial`, `approximate`, `suspect`, `stale`, and `missing` remain distinct;
- every behavior fact has an identifiability row and evidence refs;
- confidence is finite and bounded from 0 to 1;
- `identified` requires paired evidence and dimension-specific resolution/coverage;
- `bounded` explicitly records what it does and does not support;
- parameter values for threshold, ratio, knee, attack, release, lookahead, makeup, mix, detector
  filter, and channel link never appear as audio-inferred facts;
- `can_support_semantic_planning` never implies mutation authority.

## 8. Compact Context And Leakage Tests

The LLM context must:

- set `do_not_include_raw_package` true;
- contain no more than 24 compact facts and no more than 6 per typed subprojection;
- contain trust and identifiability before descriptive tags;
- preserve structured evidence refs and limitations;
- exclude raw samples, waveform/envelope arrays, frame traces, event lists, time segments,
  spectrogram tiles, render/audio file paths, shared memory, full parameter dumps, and full history;
- sanitize Windows and Unix absolute paths inside nested values and strings;
- remain useful when raw evidence artifacts are external and referenced only by opaque IDs.

Tests must recursively inspect keys and string values, following the MOM context sanitizer pattern.

## 9. Integration And Product-Path Gates

Before COM v1 is considered implemented:

1. Mixboard returns and persists a COM projection without persisting raw trace payloads.
2. `mix.read`/catalog exposes the projection and compact context with correct freshness.
3. Existing MOM, TIM, FXM, DAD, EQ, and compressor-control regression suites remain green.
4. The Godot-started agent can request `source_only` and `paired_io` observation through the real
   Vit product path.
5. A product smoke records input/output tap, exact window, latency alignment, render/state
   revisions, evidence refs, and COM trust gates.
6. A post-control smoke produces `change_delta` only after a real compressor state change and
   equivalent-input proof.
7. No smoke invokes generic parameter fallback, changes the recognizer, or enters C2.

The smoke artifact summary must include pass/fail per gate and absolute evidence artifact paths for
human inspection, while the LLM context itself must contain only opaque evidence refs.

## 10. Phase Acceptance Commands

Each goal must expose one bounded entry point. COM-1 now uses the concrete command shown below;
later commands remain names to freeze during their implementation goals:

```text
COM-1: cd agent && go test ./internal/com
COM-2: deterministic paired-evidence and latency-alignment smoke
COM-3: cd agent && go test ./internal/com; then run scripts/run_compressor_behavior_derivation_smoke.ps1
COM-4: cd agent && go test ./internal/com; then run scripts/run_compressor_change_delta_smoke.ps1
COM-5: Godot parse + Go regression + real Vit product-path smoke
```

The concrete COM-2 entry point is:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/run_compressor_dual_tap_smoke.ps1
```

Its report records every gate and the absolute artifact path for human inspection. The terminal
kernel receipt is separately checked for raw arrays, shared-memory identifiers, render paths, and
absolute-path leakage. A repeated processed render must pass the frozen observation-scale
determinism gate (`correlation >= 0.999`, absolute peak and RMS deltas `<= 0.10 dB`); the raw PCM
delta is preserved only as diagnostics for analog-modelled processors.

No phase may be accepted solely from a hand-authored JSON example or an LLM response.

The concrete COM-3 product-evidence entry point is:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/run_compressor_behavior_derivation_smoke.ps1
```

It derives a projection from an actual COM-2 artifact and checks paired identity/revisions, all
typed dimensions and identifiability rows, raw-trace exclusion, compact-context bounds, and the
absence of inferred compressor control values. Deterministic fixture coverage remains authoritative
for known action shapes and adversarial confounders; the real artifact smoke proves schema and
evidence-chain compatibility rather than ground-truthing a commercial plug-in's hidden algorithm.

The concrete COM-4 product-evidence entry point is:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/run_compressor_change_delta_smoke.ps1
```

It captures before/after CLA-2A evidence around an interior, reversible typed control change. The
report requires both COM-2 captures, unchanged structural chain identity, changed processor-state
and render revisions, equal pre-compressor input fingerprints, preserved child projection IDs, all
typed change dimensions, and exact full-snapshot restoration. A `partial` top-level delta is valid
when a child dimension is bounded or unavailable (the current real trigger relation is unavailable);
comparability gates themselves must still all pass. The smoke does not use generic parameter
fallback, relax COM-2 alignment/determinism thresholds, or infer the changed parameter from audio.

The concrete COM-5 product-path entry point is:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/run_com_product_path_smoke.ps1 `
  -RepoRoot D:\Vit_DAW -TrackId 1007 -PluginId 1021 -TimeoutSeconds 180
```

It builds the current Agent, launches kernel/Hub/Agent through Godot, waits for the kernel command
and event ports plus Agent and Hub readiness, and runs the reversible COM-4 prerequisite before
checking all three COM projection modes. The smoke verifies projection identity and readiness,
Catalog freshness, `mix.read` identity, project-store persistence, compact fact budgets, live
COM-2 capture identity, change-delta input equivalence and child IDs, and recursive raw/artifact
path isolation in both response and persisted Observation/ContextPack. A copied project has its
relative audio references rebased to verified existing files so test isolation does not change
Tracktion source-path semantics.

Accepted evidence (2026-08-04):

- `artifacts/com_product_path/20260804_162738/com4_prerequisite_report.json`: 10/10 gates passed,
  including full parameter-snapshot restoration;
- `artifacts/com_product_path/20260804_162738/com5_product_path_report.json`: 44/44 gates passed,
  with `source_only`, live `paired_io`, and `change_delta` all `partial` and trustworthy for their
  bounded claims;
- the terminal cleanup left ports 5555, 5556, 7878, and 8787 closed.
