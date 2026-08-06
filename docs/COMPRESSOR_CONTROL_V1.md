# Generic Broadband Compressor Control v1.1

## Scope

Compressor Control v1 controls a provable broadband compressor stage from the live host parameter surface.
It provides two typed tools:

- `plugin_grabber.inspect_compressor`: read-only topology recovery.
- `plugin_grabber.apply_compressor_controls`: explicit physical or enum control.

The tool layer does not interpret acoustic intent such as "compress more", punch, glue, or
vocal density. The agent must convert intent into explicit requested values before calling
apply.

This compressor-control layer explicitly identifies but does not control:

- pure limiters (`unsupported_limiter`);
- multiband compressors (`unsupported_multiband_compressor`);
- gates and expanders (`unsupported_gate_expander`);
- de-essers (`unsupported_de_esser`);
- spectral dynamics (`unsupported_spectral_dynamics`);
- pure clippers (`unsupported_clipper`);
- natural-language acoustic decisions;
- Profile, VPS, and SPAL mappings;
- product or vendor identity as recognizer evidence.

Plugin Alliance round 1 was frozen and executed once as the v1 blind set. After disclosure, those
18 cases became a public v1.1 regression cohort. The sealed reserve set remains unread.

## Evidence Boundary

The recognizer reads only observable parameter facts:

- parameter name, raw name, and alias;
- host controllability;
- current display text;
- read-only display probes, discrete labels, and measured domains.

It does not read plugin name, vendor, path, identifier, presets, screenshots, stored mappings,
or product-specific profiles. `DisplayGroup` and pre-existing generic `NormalizedRole` values
are not accepted as compressor role evidence because they can contaminate topology (for
example, a Mix group around an Input control or generic Gain0 fields).

Current values are excluded from the topology generation hash. Measured domains and reachable
labels are included because they define the valid control surface.

## Topology

The recovered graph is:

```text
compressor_stage
  control_paths[]
    detector
    operating_point
    transfer
    timing
    gain_action
  output
  auxiliary_stages[]
```

Path keys represent independently addressable control paths, including shared/main, left/right,
left-mid/right-side, and low/high-level paths. Detector-only or shared modifier branches are
coalesced into the compressor path they control. Shared modifiers are copied across multiple
provable paths rather than emitted as a fake compressor path.

The v1 classifications are structural summaries, not product classes:

| Classification | Required topology evidence |
| --- | --- |
| `threshold_driven` | threshold operating point |
| `input_driven_ratio` | input drive plus ratio |
| `input_driven_fixed_transfer` | input drive plus a fixed-transfer response/mode and corroborating output structure |
| `amount_driven` | peak/reduction/compression amount |
| `bidirectional_curve` | measured ratio/direction domain crosses 1:1 |
| `multi_path_level_control` | distinct low/high level paths |
| `multi_path_channel_control` | multiple channel/control paths |
| `multi_stage_serial_control` | multiple named sequential broadband compression stages |
| `compound_operating_point` | compressor evidence not covered above |

An auxiliary limiter or frequency-selective stage does not invalidate a separately provable
broadband compressor stage. Frequency bands count as multiband evidence only when at least two
bands each expose a compressor operating point plus transfer/timing evidence. Sidechain-EQ bands
remain detector structure. Indexed frequency/Q/threshold arrays without corresponding indexed
compressor cells are rejected as spectral dynamics. UI display switches do not count as input or
output gain evidence.

## Inspect Contract

Inspect returns:

- `schema_version = compressor-control-topology/v1`;
- `mapping_source = generic_structural`;
- topology `generation`;
- classification, confidence, paths, roles, domains, curves, and reachable enum labels;
- a checksum-protected, generation-scoped `control_ref` for every writable binding.

A `control_ref` binds track, plugin instance, generation, stage, path, section, role, and parameter
ID. Apply rejects malformed, target-mismatched, missing, or stale references before any write.

## Apply Contract

Every request is atomic and contains one or more controls. Each control supplies its inspect
`control_ref` and exactly one target:

- `value_db`;
- `ratio`;
- `value_ms`;
- `percent`;
- `display_value`;
- `enum_label`.

Units must agree with both the role and observed domain. Numeric values must be finite and inside
the measured domain. Discrete values are selected only from observed reachable labels or physical
steps. Duplicate parameters in one batch are rejected.

Execution reuses the existing atomic parameter transaction engine:

1. Read the complete parameter preimage.
2. Plan and validate every write before mutation.
3. Execute one batch.
4. Read the complete parameter surface.
5. Reject any unplanned parameter change.
6. Verify physical/enum readback, including bounded physical correction where needed.
7. Restore and verify the complete preimage on any failure.

Successful apply also returns a checksum-protected `restore_ref` containing the exact normalized
preimage for the touched bindings. Passing that token back to the same typed tool restores and
verifies the original normalized values without inverting rounded display text.

When a plug-in returns the current text for every read-only `value_to_string` sample, inspect marks
the binding `transactional_probe_required` but remains read-only. An authorized apply performs a
bounded local direction probe, restores its probe preimage, and then executes normal physical
correction inside the atomic transaction. It never assumes a full `[0,1]` curve across a possible
discontinuity.

Non-finite display endpoints such as `-INF` are sentinels, not finite domain endpoints. They are
excluded from invertible curves; a finite target may never resolve to a non-finite endpoint.

Named endpoint states may also surround a continuous numeric interior. When at least three numeric
samples form a monotonic interior curve, physical inversion uses those measured normalized anchors
only; it does not treat the finite display minimum/maximum as normalized 0/1 or extrapolate through
the named endpoint states.

Results report `exact`, `quantized`, or `rejected`; rejected means no requested state remains.

## Training Matrix

The positive acceptance matrix is fixed at 22 local products:

| Product | Structural classification |
| --- | --- |
| Abbey Road RS124 Stereo | `multi_path_channel_control` |
| API-2500 Stereo | `threshold_driven` |
| C1 comp Stereo | `bidirectional_curve` |
| CLA-2A Stereo | `amount_driven` |
| CLA-3A Stereo | `amount_driven` |
| CLA-76 Stereo | `input_driven_ratio` |
| dbx-160 Stereo | `multi_path_channel_control` |
| DPR-402 Stereo | `multi_path_channel_control` |
| H-Comp Stereo | `threshold_driven` |
| Kramer PIE Stereo | `threshold_driven` |
| Magma StressBox Stereo | `bidirectional_curve` |
| MaxxVolume Stereo | `multi_path_level_control` |
| MV2 Stereo | `multi_path_level_control` |
| OneKnob Pressure Stereo | `amount_driven` |
| PuigChild 670 Stereo | `multi_path_channel_control` |
| RCompressor Stereo | `bidirectional_curve` |
| Renaissance Axx Stereo | `threshold_driven` |
| RVox Stereo | `amount_driven` |
| SSLComp Stereo | `threshold_driven` |
| VComp Stereo | `input_driven_ratio` |
| FabFilter Pro-C 2 | `threshold_driven` |
| ZL Compressor | `threshold_driven` |

Negative boundary fixtures are Waves L2 Stereo (pure limiter) and C6 Stereo (multiband); both must
return `not_compressor`.

Real apply/readback/restore smoke coverage uses API-2500 (threshold plus ratio atomic batch),
CLA-2A (reduction amount), CLA-76 (input drive), and MV2 (low plus high level atomic batch).

## Historical Freeze And Blind Protocol

The recognizer and its fixtures are frozen only after:

- all 22 positive cases expose generic topology and complete `control_ref` coverage;
- both negative cases reject;
- the four representative real apply smokes restore the full preimage;
- `go test ./...` passes.

The v1 freeze and one-shot blind result remain historical evidence. The v1.1 implementation does
not overwrite or relabel that result; it uses the disclosed round-1 cohort only as regression data.

Frozen recognizer/fixture composite SHA-256:

`9b0a6dc28e306b5cdf6708a60edb1489e58a94c44032a22d9a8ff0e59120c830`

Post-freeze blind result: Plugin Alliance `bx_townhouse Buss Compressor` was recognized without
rule changes as one `threshold_driven` path (confidence 0.90) with detector mode, threshold,
ratio, attack, release, makeup gain, and mix bindings. The topology generation was
`c2t1_2befce3dd5dcdf149c6ea010`.

## Godot Product-Path Validation

The focused product-path smoke is:

```powershell
D:\Vit_DAW\scripts\run_vit_product_path_smoke.ps1 `
  -RepoRoot D:\Vit_DAW `
  -GodotProjectRoot D:\Godot\project\vit-daw-frontend `
  -GodotExe D:\Godot\Godot_v4.6.1-stable_win64.exe `
  -CompressorControlAgentOnly -KeepProcesses
```

It rebuilds the production `agent\bin\VitAgent.exe` and VSP Hub, then requires the Godot runtime
to autostart Kernel, Hub, and Agent before testing compressor control through Agent HTTP on port 7878. The focused
branch verifies both compressor-control catalog entries, API-2500 inspect, an atomic threshold-plus-ratio
apply/readback/complete-preimage restore, and L2 pure-limiter rejection. Temporary tracks are
deleted before completion.

The v1.1 product-path run passed on 2026-08-04 with production Agent SHA-256
`174A2D42766C1AD4DE867251C1A8D139F43D973FAD8D43EF3C972D4B1AFC928B`. API-2500 apply and exact
normalized restore both returned `exact`; L2 returned `unsupported_limiter`. Evidence is stored
under `VitApp\Workspace\Artifacts\smoke\product_path_20260804_094717`.

## Vit Natural-Language Validation

With the same Godot-owned lifecycle and a disposable FabFilter Pro-C 2 instance, Vit accepted:

```text
把 Threshold 设置为 -12 dB，Ratio 设置为 4:1
```

Conversation `compressor_nl_final_20260803_2350` completed with exactly two executed items:
`plugin_grabber.inspect_compressor` followed by `plugin_grabber.apply_compressor_controls`.
The apply result was `exact` for both controls and its typed readback was `-12.00 dB` and
`4.00:1`. No generic parameter write, `daw.invoke`, explanation step, retry, or `goal.tick`
was present. A direct generic `plugin.set_parameter` attempt against the same live Pro-C 2
instance returned `typed_compressor_control_required` and left all 45 parameter values unchanged.

The repeatable smoke runner is:

```powershell
python D:\Vit_DAW\scripts\compressor_nl_control_smoke.py `
  --agent-http http://127.0.0.1:7878 `
  --output D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\compressor_nl_smoke.json
```

It resolves the local Pro-C 2 fixture, loads it on a disposable track, sends the real chat
request, validates the event route and typed readback, and deletes the temporary track in a
`finally` block.
