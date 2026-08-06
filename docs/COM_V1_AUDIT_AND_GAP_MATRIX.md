# COM v1 Audit And Implementation Gap Matrix

Status: design audit complete
Date: 2026-08-04

Current implementation progress: COM-1 through COM-4 are implemented and covered by deterministic
package tests plus bounded real-product evidence smokes. COM-5 and all later semantic/C2 phases
remain open.

## 1. Audit Scope

This audit is read-only with respect to runtime code. It examined the current DAD/acoustic package,
MOM v1.5, FXM v0, Mixboard L2 Render Probe and A/B paths, frequency-cleanup post-FX baselines,
real observation artifacts, and the Pro-C 2 compressor reference.

The resulting contract is frozen in `docs/COM_V1_CONTRACT.md`. The original audit target made no
runtime changes; the subsequent COM-1 target added only the isolated `source_only` package. No
compressor recognizer/controller change, semantic planner, product-path wiring, or C2 integration
has been made.

## 2. Evidence-Based Findings

### Existing architecture that should be reused

- Observation models sit above DAD and emit task-oriented compact projections; raw waveform and
  spectrogram payloads do not belong in LLM context.
- MOM already carries observation/session identity, project cut and source/clip/render revisions,
  tap point, freshness, evidence refs, trust gates, compact facts, and raw-payload exclusion.
- FXM v0 already owns generic bypass/processed chain comparison for matching source, window, sample
  format, deterministic quality, and latency compensation. Its current deltas cover RMS, LUFS,
  peak, crest, bands, and latency.
- Mixboard's A/B result already gates before/after comparison on ready probes, same tap, same render
  mode, changed render revision, quality evidence, and raw-payload exclusion.
- `frequencycleanup.TargetPostFXBaseline` provides a reusable pattern for exact target coverage,
  uniform post-FX taps, state fingerprints, before/after baseline identity, and inconclusive results
  when comparability fails.

### Existing evidence limitations

- The inspected real waveform artifact exposes detailed source metadata and five-second
  `time_segments` with RMS, peak, crest, and energy state. This is useful for macro activity and
  section variance, not compressor attack/release behavior.
- The same artifact advertises a 10 ms source analyzer frame duration and a 500-frame waveform
  resolution, but the observation-facing time-energy rows are five-second summaries. The finest
  trace is not currently preserved as a typed COM-ready evidence contract.
- Mixboard caps project waveform time segments at 12 rows, acoustic status caps them at 16, and
  persisted observation compaction removes `time_segments`. This is correct for LLM/storage
  hygiene but means COM must derive compact facts before those lossy boundaries.
- Current L2 Render Probe deliberately rejects raw waveform/time-segment payloads and emits scalar
  level, band, stereo, quality, tap, and revision facts. It supports general A/B, but not aligned
  gain-action, transient, or recovery analysis.
- FXM accepts scalar `Measurement` values. It has no synchronized envelope/event inputs and cannot
  identify compressor-specific time behavior.
- Existing source-only DAD rows are pre-FX source evidence. They cannot establish what a loaded
  compressor did, especially when makeup/output gain or wet/dry mix is active.

## 3. Ownership Decision

COM does not replace or extend MOM into a generic dynamics bucket. MOM remains responsible for mix
relationships and general A/B presentation. COM is a peer projection because compressor behavior
requires a different evidence topology: aligned pre/post time behavior plus explicit
identifiability.

COM also does not replace FXM. FXM remains the canonical generic transformation delta and
comparability receipt. COM consumes compatible FXM identity/quality evidence where possible and
adds compressor-specific typed derivations from synchronized envelope/event evidence.

The practical split is:

```text
DAD/render probe -> typed aligned evidence
                  -> FXM: generic chain transformation
                  -> COM: compressor behavior and identifiability
MOM              -> project relation and general A/B context
future planner   -> intent and parameter decisions using COM + live topology
```

## 4. Capability And Gap Matrix

| COM need | Current capability | Classification | Required implementation response |
| --- | --- | --- | --- |
| source identity/revision | DAD, acoustic package, MOM | reusable | normalize into COM conditions |
| project cut and target identity | MOM/Mixboard | reusable | reuse stable refs, do not invent identity |
| scalar peak/RMS/crest | DAD and L2 probe | available | expose as source/level typed facts |
| macro activity/time energy | five-second DAD segments | partial | source-only macro projection; mark lower scales unavailable |
| exact analysis window | FXM start/end seconds | partial | add sample-index start/end as authority |
| input/output tap identity | L2 tap exists, normally one post-FX tap | missing for pair | add explicit pre-compressor and post-compressor taps |
| exact processor scope | FXM chain identity, compressor topology generation | partial | bind pair to one instance/scope and reject unrelated chain changes |
| deterministic dual render/pair | FXM quality fields | partial | capture or render synchronized pre/post evidence |
| latency alignment | FXM boolean/latency scalar | partial | add alignment offset, method, residual/error proof |
| fine envelope trace | not present in observation contract | missing | new DAD/render evidence artifact, kept outside LLM context |
| onset/event correspondence | none | missing | deterministic event matching over aligned traces |
| gain-action envelope | none | missing | derive from aligned input/output envelopes with silence guards |
| transient response | none | missing | derive bounded matched-event facts at sufficient resolution |
| recovery motion | none | missing | derive return/incomplete-recovery/modulation candidates |
| stereo action relation | stereo summaries only | partial | add channel-aware paired envelope facts; mono is N/A |
| trigger/band relation | band summaries, no time alignment | missing for behavior | optional aligned band-energy/action correlation evidence |
| generic level delta | FXM and MOM A/B | available | reference canonical delta rather than duplicate it |
| before/after state comparison | MOM A/B and baseline patterns | partial | compare two internally ready paired COM projections |
| input equivalence gate | not explicit | missing | add tolerance and attribution gate for change_delta |
| identifiability | no compressor-specific contract | missing | implement per-dimension identified/bounded/unknown rows |
| compact context hygiene | MOM/FXM sanitizers | reusable | reuse forbidden-key/path tests and fact budgets |
| product-path exposure | Mixboard catalog has MOM/TIM/FXM | missing | later add COM catalog/read/context projection |

## 5. What Current Evidence Can Truthfully Support

With no new capture implementation, a prototype projection could truthfully support only
`source_only` macro facts and scalar post-FX references. It could describe that a source is sparse,
dense, high-crest, or macro-variable when the evidence is fresh.

It cannot truthfully support:

- observed gain-reduction depth or duty cycle;
- attack/transient capture behavior;
- release/recovery behavior or pumping;
- compressor input/output attribution;
- parameter-value inference;
- a trustworthy post-control compression behavior delta.

Therefore a first code milestone that merely wraps current RMS/crest fields and calls the result a
ready compressor observation would violate the frozen contract.

## 6. Implementation Phases

### Phase COM-1: typed contract and source-only projection

Implement COM types, deterministic IDs, status/trust/identifiability logic, source-only macro facts,
context sanitization, and deterministic package-level fixtures. No product-path wiring, semantic
planner, or parameter control changes.

Exit gate: `source_only` never promotes compressor behavior and all lower time scales are correctly
reported as unavailable with current five-second evidence.

### Phase COM-2: aligned paired evidence

Add a DAD/render-probe evidence artifact for exact-window pre-compressor and post-compressor
envelopes/events. Freeze tap identities, processor scope, sample-index window, latency alignment,
quality evidence, and raw-payload storage boundaries.

Exit gate: deterministic fixtures and a real Vit product smoke prove same-window dual-tap capture,
nonblank evidence, latency alignment, revision freshness, and no raw leakage to observation context.

Implementation note (2026-08-04): COM-2 is implemented by the kernel command
`compressor_dual_tap_probe`, the raw artifact schema `dad.compressor_dual_tap_evidence.v1`, and the
compact receipt schema `dad.compressor_dual_tap_receipt.v1`. The v1 scope gate accepts only one
recognized `single_band_broadband` compressor in an isolated direct track slot or a transparent,
serial, one-node Vit rack. Active clip processors, other active audio processors, non-unity
volume/pan, rack dry/parallel paths, and ambiguous rack topology are rejected. The input tap is a
same-track `usePlugins=false` render; this is valid only because those gates prove that disabling
track/clip plug-ins removes exactly the selected compressor while all admitted utilities are
transparent.

Each capture uses one sample-index-authoritative window and fixed sample format. It renders the
input once and the processed output twice. Determinism is proven at the observation scale by
repeat-render correlation and peak/RMS tolerance; sample-exact identity is recorded diagnostically
but is not required because analog-modelled compressors may contain bounded stochastic behavior.
`pair_id` and `job_id` identify the individual capture and therefore change on a repeat capture;
`render_revision` identifies the frozen source/window/processor render state and therefore remains
stable when that state is unchanged. Capture freshness is checked with the former identities, not
by requiring an unchanged state hash to mutate.
Cross-correlation measures the integer alignment offset after the renderer's latency compensation.
The aligned per-channel envelope and input onset candidates remain only in the raw artifact;
events expose an opaque evidence ref, SHA-256, byte count, quality, scope, revision, and alignment
receipt. COM-2 deliberately performs no gain-action, attack, release, recovery, or semantic
derivation.

### Phase COM-3: paired behavior derivation

Implement gain-action, matched transient, recovery-motion, level-effect references, optional stereo
behavior, and per-dimension identifiability. Keep algorithms deterministic and return bounded or
not-identifiable rather than guessing.

Exit gate: synthetic fixtures with known envelope behavior pass; confounded makeup/mix, silence,
insufficient resolution, and mismatched events do not produce false ready claims.

Implementation note (2026-08-04): COM-3 is implemented in `agent/internal/com` as a read-only
consumer of `dad.compressor_dual_tap_evidence.v1`. The derivation removes an observed steady gain
baseline before reporting effective time-varying gain action; this keeps fixed output/makeup gain
separate from action without claiming the plug-in's makeup parameter or gain-reduction meter.
Matched onset candidates produce bounded leading-edge/body classifications, event-consistency
facts, and observed recovery-time bands. Periodic modulation is only a bounded candidate with
confidence, program-rhythm alternatives, and explicit non-causality. Stereo facts report channel
action coherence/asymmetry while envelope-only image/phase change remains not identifiable.

Every typed behavior has a separate identifiability row. Known parallel blend and ambiguous gain
stage constraints downgrade affected behavior to bounded; silence, coarse resolution, invalid
scope/alignment/identity, discontinuous traces, and invalid events cannot promote behavior. The
compact projection retains only distributions, counts, bounded labels, identities, trust, and
opaque evidence refs. Raw frame/event arrays remain solely in the COM-2 artifact. COM-3 adds no
control values, mutation authority, change-delta logic, product-path wiring, or C2 behavior.

### Phase COM-4: behavioral change delta

Compare two ready/partial paired projections using strict identity and input-equivalence gates.
Reuse FXM/MOM identity patterns without replacing their generic deltas.

Exit gate: changed control state with equivalent input produces typed behavior changes; input,
window, tap, analyzer, topology, or unrelated-chain mismatches are suspect/stale/inconclusive.

Implementation note (2026-08-04): COM-4 accepts exactly two distinct ready/partial `paired_io`
children. It requires equal processor target, topology generation, structural chain hash, source
and clip revisions, sample window and format, taps, render/tail/latency policies, analyzer and
resolutions, plus an equal compact SHA-256 identity of the quantized pre-compressor peak/RMS
envelope. The processor-state hash and render revision must both change. Failed comparability
never becomes a zero-effect claim: all behavior dimensions become `non_comparable`, with stale
reserved for identity/revision failures and suspect used for untrustworthy children or unequal
input traces.

The delta classifies gain action, transient response, recovery motion, absolute level effect,
stereo behavior, and the currently unavailable trigger relation as `stronger`, `weaker`,
`unchanged_within_tolerance`, `mixed`, or `non_comparable`. Each axis defines stronger locally;
notably, a stronger level effect means a larger absolute scalar level change and does not mean
more compression. COM-4 never infers which parameter changed or any control value.

The real CLA-2A smoke performs one reversible typed `reduction_amount` change, captures both COM-2
children, derives the delta, then restores from the exact `restore_ref` and verifies the complete
parameter snapshot. Its control target is an interior measured curve point, because endpoint
stress belongs to controller testing rather than observation comparability. This smoke exposed and
closed an evidence-identity defect: COM-2 `chain_hash` is now a structural graph hash (slots,
nodes, positions, enabled states, and connections), while parameter state remains exclusively in
`processor_state_hash` and the encompassing `scope_revision`.

### Phase COM-5: product-path hardening

Wire COM into Godot-started Vit agent observation/read/context paths, persistence/catalog, smoke
entry points, and regression suites. This phase still does not implement abstract semantic control.

Exit gate: the user can request a read-only compressor observation through the real product path,
and logs/artifacts prove compact context and correct readiness.

Implementation note (2026-08-04): COM-5 is implemented as an explicit, read-only peer projection
on `ObservationPacket`. `mix.observe` materializes COM only when `com_mode` is one of
`source_only`, `paired_io`, or `change_delta`; ordinary Mixboard observations retain their frozen
payload behavior. The projection is exposed through Catalog key `observation.com_projection`,
`mix.read`, ContextPack, the project-scoped store, the Mixboard package status/digest, and the
AgentLoop's bounded prompt summary.

`source_only` consumes a compact current waveform evidence row and preserves the standard
track/clip/material freshness checks. A caller-supplied ready COM source snapshot is direct
evidence and cannot be replaced by a weaker read-first acoustic-package cache row. `paired_io`
accepts an allowlisted COM-2 artifact or, when no artifact is supplied, asks the Godot-started
Agent to perform a live `compressor_dual_tap_probe`, validates its terminal receipt and artifact,
and exposes only a compact capture receipt. `change_delta` reads two allowlisted COM-2 artifacts,
builds both `paired_io` children, and retains every COM-4 comparability and input-equivalence gate.

Raw frame/event arrays and artifact paths remain outside observation, ContextPack, Catalog, and
AgentLoop context. The accepted Godot product smoke is
`artifacts/com_product_path/20260804_162738/com5_product_path_report.json`: all 44 COM-5 gates
passed for CLA-2A (`source_only`, live `paired_io`, and `change_delta`), and the prerequisite
COM-4 report passed all 10 capture/change/restore gates. COM-5 adds no natural-language semantic
route, parameter decision, mutation authority, C2 integration, limiter, or multiband behavior.

### Later goals outside COM v1 implementation

Only after COM-1 through COM-5 are accepted:

1. design and implement compressor abstract semantic planning using COM plus live topology;
2. natural-language semantic-control smoke and bounded proposal/confirmation flow;
3. return to C2 observation/readiness/orchestration and closed-loop verification.

Limiter and multiband work remain separate future model/controller phases.

## 7. Recommended Goal-Mode Todo List

| Goal | Scope | Primary deliverable |
| --- | --- | --- |
| G1 | COM-1 | types, source-only projection, trust, identifiability, context and unit tests |
| G2 | COM-2 | aligned dual-tap evidence contract and deterministic capture fixtures |
| G3 | COM-3 | paired behavior derivation and adversarial/confounder tests |
| G4 | COM-4 | change-delta comparability and attribution tests |
| G5 | COM-5 | Mixboard/agent/Godot product-path integration and smoke |
| G6 | semantic design | abstract compressor intent/axis policy, no C2 orchestration yet |
| G7 | semantic implementation | planner, materialization to live controls, confirmation and verification |
| G8 | C2 | C2 observation contract, integration, closed-loop product smoke |

Each goal should be opened separately. G2 is the highest-risk engineering step; G3 must not start
until the paired evidence and alignment contract pass independently.

## 8. Files Examined

- `agent/internal/acousticpackage/status.go`
- `agent/internal/fxm/types.go`
- `agent/internal/fxm/projection.go`
- `agent/internal/fxm/projection_test.go`
- `agent/internal/mixboard/mixboard.go`
- `agent/internal/mixboard/project_package.go`
- `agent/internal/mom/types.go`
- `agent/internal/mom/projection.go`
- `agent/internal/mom/context_pack.go`
- `agent/internal/frequencycleanup/target_baseline.go`
- `docs/OBSERVATION_V1_ACCEPTANCE.md`
- `docs/COMPRESSOR_REFERENCE_PRO_C_2.md`
- real artifacts under `VitApp/Workspace/Artifacts`
