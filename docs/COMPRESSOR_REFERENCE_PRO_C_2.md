# Compressor Reference Exemplar - FabFilter Pro-C 2

## Purpose

This document is the canonical worked example for generic broadband-compressor semantic planning.
It plays the same explanatory role that the historical Q10 reference played for EQ: one complete,
measured product makes the public control vocabulary, topology, physical domains, planning rules,
and verification boundary concrete.

It is not a Profile, VPS, SPAL mapping, product allowlist, or executable parameter table. Runtime
execution must always use the live `plugin_grabber.inspect_compressor` result and its generation-
scoped `control_ref` values. The product name and parameter IDs below are evidence labels only.

The semantic layer owns decisions such as "compress more", "keep the vocal steady", or "preserve
the transient". The deterministic tool layer owns only topology discovery, explicit physical or
enum control, atomic execution, readback, rollback, and exact restore.

## Why Pro-C 2 Is The Main Exemplar

Pro-C 2 exposes a nearly complete single-path compressor surface:

```text
compressor_stage
  main path
    detector: sidechain filters, mode, channel link
    operating_point: threshold, input_drive
    transfer: ratio, knee, transfer_mode
    timing: attack, release, auto_release, hold, lookahead
    gain_action: reduction_range, auto_makeup
  output: output_gain, mix, wet_gain, dry_gain
```

The measured surface is `threshold_driven`, one shared path, confidence 0.96, with topology
generation `c2t1_6b96b6b6969df9a2ddbf22af`. The generation is historical evidence, not a value to
reuse against a new instance.

Source evidence:

`D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\product_path_20260804_094717\compressor_control\v11_training_matrix\pro_c_2.json`

The same product passed explicit natural-language control of Threshold = -12 dB and Ratio = 4:1
with exact typed readback and no generic parameter-write fallback.

## Measured Reference Surface

The following compact table records the roles relevant to semantic planning. The live inspect
result remains authoritative.

| Section | Public role | Measured Pro-C 2 control | Observed domain |
| --- | --- | --- | --- |
| operating point | `threshold` | Threshold | -60 to 0 dB |
| operating point | `input_drive` | Input Level | -18 to +36 dB |
| transfer | `ratio` | Ratio | 1:1 to 100:1, measured curve |
| transfer | `knee` | Knee | 0 to 72 dB |
| transfer | `transfer_mode` | Style | Clean, Classic, Opto, Vocal, Mastering, Bus, Punch, Pumping |
| timing | `attack` | Attack | 0.005 to 250 ms |
| timing | `release` | Release | 10 to 2500 ms |
| timing | `auto_release` | Auto Release | Off, On |
| timing | `hold` | Hold | 0 to 500 ms |
| timing | `lookahead` | Lookahead | 0 to 20 ms |
| gain action | `reduction_range` | Range | 0 to 60 dB |
| gain action | `auto_makeup` | Auto Gain | Off, On |
| detector | `sidechain_filter` | Low/Mid/High SC frequency | 20 Hz to 20 kHz |
| detector | `channel_link` | Stereo Link | 0 to 100% |
| output | `output_gain` | Output Level | -18 to +36 dB, finite measured anchors |
| output | `mix` | Mix | 0 to 200% on this product; never assume 0 to 100% |
| output | `wet_gain` | Wet Gain | -18 to +36 dB, with a non-finite endpoint |
| output | `dry_gain` | Dry Gain | -18 to +36 dB, with a non-finite endpoint |

Not every observed control is automatically a good semantic target. Detector audition, external
sidechain selection, channel routing, and style changes can alter the meaning of the processor or
require user intent that is not implied by a generic dynamics request.

## Canonical Semantic Axes

The semantic planner should reason in orthogonal acoustic axes first, then project those axes onto
the roles actually present in the selected topology.

| Semantic axis | Primary roles | Meaning |
| --- | --- | --- |
| activation/intensity | `threshold`, `input_drive`, `reduction_amount`, `low_level_amount`, `high_level_amount` | How often and how deeply the gain-reduction stage engages |
| transfer severity | `ratio`, `direction_curve`, `knee`, `reduction_range` | How strongly level above/below the operating point is reshaped and the maximum allowed action |
| transient timing | `attack`, `lookahead`, `hold` | Whether leading edges pass, are rounded, or are caught before they emerge |
| recovery/sustain | `release`, `recovery`, `time_constant`, `pdr_time`, `auto_release` | How gain returns and whether motion follows phrases, hits, or sustained material |
| detector focus | `sidechain_filter`, `detector_mode`, `channel_link` | Which energy drives compression and how channels interact |
| output normalization | `makeup_gain`, `output_gain`, `auto_makeup` | Post-reduction level compensation; not compression intensity |
| parallel balance | `mix`, `wet_gain`, `dry_gain` | Blend between processed and unprocessed paths; not a substitute for detector/transfer design |
| character | `transfer_mode`, `response`, `transient_emphasis` | Reachable algorithm or response character, used only when the intent justifies changing it |

These axes are not interchangeable. Raising output gain does not mean "more compression". Lowering
mix reduces audible wet contribution but does not weaken the compressor's internal gain reduction.
Changing attack can alter peak passage without changing the nominal threshold or ratio. A semantic
plan must state which acoustic axis it is changing.

## Topology Projection Rules

The same semantic intent must project differently across compressor families. The planner must not
require a role that the live topology does not expose.

| Live classification | Primary intensity actuator | Transfer/timing projection |
| --- | --- | --- |
| `threshold_driven` | lower/raise `threshold`, optionally adjust `input_drive` when gain staging is intentionally part of the plan | use available ratio, knee, attack, release and range roles |
| `input_driven_ratio` | raise/lower `input_drive` against the fixed or selected threshold behavior | ratio and timing remain independent when exposed |
| `input_driven_fixed_transfer` | raise/lower `input_drive` | response/mode/timing may shape behavior; never invent ratio |
| `amount_driven` | raise/lower `reduction_amount` | use recovery/time controls when exposed; never synthesize threshold or ratio |
| `multi_path_level_control` | address `low_level_amount` and/or `high_level_amount` explicitly | preserve independent path intent; do not collapse both into one hidden scalar |
| `multi_path_channel_control` | plan per path or require a provable linked/shared control | do not silently edit only one channel/path for a global request |
| `multi_stage_serial_control` | identify the intended named stage before control | do not treat two serial stages as duplicate knobs |
| `bidirectional_curve` | use the measured operating point and `direction_curve` relative to unity | upward and downward action must be explicit in the plan |

If the requested acoustic axis cannot be represented by reachable roles, the planner must narrow
the proposal, offer an explicit alternative, or return a capability boundary. It must not choose an
unrelated role merely because that role is writable.

## Directional Semantics

The following are semantic tendencies, not unconditional parameter presets:

| Requested change | Typical role direction | Required qualification |
| --- | --- | --- |
| engage more often/deeper | threshold downward, input drive upward, or reduction amount upward | choose exactly the actuator supported by the topology; observation determines magnitude |
| stronger level containment | ratio upward and/or range allowance upward | only when these roles exist; range is a ceiling on action, not the operating point |
| preserve more initial transient | attack slower, lookahead lower | verify peaks are the intended feature rather than unwanted overs |
| catch sharper peaks | attack faster and/or lookahead higher | limiter-like requests remain outside this compressor controller when the selected processor is a limiter |
| recover between hits | release/recovery shorter | avoid audible modulation or distortion; tempo alone is insufficient evidence |
| smooth phrase-level motion | release/recovery longer or auto mode | require envelope evidence that phrase tracking is desired |
| reduce low-frequency pumping | detector high-pass/focus adjustment | do not EQ the audible signal through a detector-only role |
| keep loudness comparable | output/makeup compensation after the dynamics decision | comparison level is an evaluation constraint, not proof of compression quality |
| use parallel compression | design the wet compression first, then set mix/wet/dry balance | lowering mix alone is not a compressor design |

"More" and "less" must be resolved relative to the current physical value, reachable domain, and
measured audio evidence. No fixed dB, ratio, millisecond, or percentage delta is universally valid.

## Observation Contract For Semantic Planning

The semantic layer may use the mature observation layer, but each decision must cite evidence that
matches the axis being changed.

| Decision | Relevant evidence examples |
| --- | --- |
| compression intensity | peak distribution, crest factor, gain-reduction behavior when observable, phrase-level variance |
| attack/lookahead | leading-edge overshoot, transient-to-body relationship, peak duration |
| release/recovery | envelope return time, inter-hit spacing, sustained modulation, pumping evidence |
| detector filtering | correlation between low-frequency energy and unwanted gain movement |
| output compensation | pre/post loudness and peak comparison at the same observation tap |
| channel link/path targeting | stereo image stability and independently observed channel/path behavior |

Evidence must come from a compatible tap point and render revision. A semantic proposal may be
reasonable without proving its audible result in advance, but it must not claim a diagnosis that
the observation does not support.

## Reference Planning Shape

The future semantic planner should produce a topology-independent decision that can be
materialized into live compressor `control_ref` requests. This illustrative shape is a design
reference, not an implemented schema:

```json
{
  "schema_version": "compressor-semantic-plan/reference-v1",
  "target": {
    "track_id": "live track",
    "plugin_id": "live compressor instance",
    "topology_generation": "generation returned by inspect"
  },
  "intent": {
    "summary": "preserve the leading transient while reducing phrase-level variation",
    "axes": ["activation_intensity", "transient_timing", "output_normalization"]
  },
  "controls": [
    {
      "role": "threshold",
      "target_kind": "physical_absolute",
      "value_db": -12.0,
      "reason": "chosen from current topology and observation evidence"
    },
    {
      "role": "attack",
      "target_kind": "physical_absolute",
      "value_ms": 20.0,
      "reason": "retain the observed leading edge"
    }
  ],
  "constraints": {
    "atomic": true,
    "preserve_unmentioned_roles": true,
    "level_matched_evaluation": true
  },
  "evidence_refs": ["observation reference"],
  "limitations": []
}
```

An implementation must bind each planned public role to a live `control_ref` from the same
generation. It must reject duplicate roles, ambiguous paths, unavailable units, and stale topology
before mutation.

## Worked Projection Examples

### Explicit parameter request on Pro-C 2

User intent: "Set Threshold to -12 dB and Ratio to 4:1."

- This is not abstract acoustic inference.
- Inspect the live compressor.
- Resolve the `threshold` and `ratio` bindings on the intended path.
- Apply both controls atomically with their physical values.
- Require exact/quantized typed readback and retain `restore_ref`.

This path has already passed the real Vit product smoke.

### Abstract steady-vocal request on an amount-driven leveler

User intent: "Make the vocal a little steadier."

- Observation/LLM determines whether reduced phrase variance is appropriate.
- The live topology exposes `reduction_amount` and `recovery`, not threshold and ratio.
- Plan a bounded change to reduction amount; change recovery only if envelope evidence supports it.
- Treat output gain as separate level compensation.
- Do not emulate Pro-C 2 by inventing threshold, ratio, attack, or knee.

### Preserve punch on an input-driven compressor

User intent: "Control it, but keep the attack."

- Use `input_drive` as the supported intensity actuator.
- Use `attack` only if the live topology exposes a physical timing role.
- If attack is fixed, reduce intensity or propose a different processor rather than writing a mode
  that has no proven relationship to transient preservation.

### Unsupported adjacent processor

If inspect returns `unsupported_limiter`, `unsupported_multiband_compressor`,
`unsupported_clipper`, `unsupported_gate_expander`, `unsupported_de_esser`, or
`unsupported_spectral_dynamics`, the semantic compressor planner must stop before materialization.
It may route to a future dedicated controller, but it must not weaken the broadband boundary.

## Materialization And Verification Invariants

1. Select one real loaded instance and freeze its live topology generation.
2. Keep semantic planning separate from parameter binding and execution.
3. Materialize only roles and paths proven reachable by inspect.
4. Preserve all unmentioned controls.
5. Execute the complete request atomically through `plugin_grabber.apply_compressor_controls`.
6. Require actual typed readback for every requested control.
7. On any failure, verify full-preimage rollback.
8. Keep the exact normalized `restore_ref` for user undo or controlled comparison.
9. Refresh observation evidence after mutation before claiming an acoustic result.
10. Report what changed and why without claiming that output gain or mix alone created compression.

## Development Boundary

This reference is sufficient to begin abstract-semantic design, but it does not itself implement a
semantic compressor planner, proposal UI, observation-to-parameter policy, post-apply evaluation,
or C2 workflow integration. Those belong to the next development phase.

Recognizer changes are now evidence-gated. The semantic phase must consume the existing generic
topology rather than add product-specific recognition rules for the examples in this document.
