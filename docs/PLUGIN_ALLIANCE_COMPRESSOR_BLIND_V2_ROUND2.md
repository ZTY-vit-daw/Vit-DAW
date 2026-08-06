# Plugin Alliance Compressor Control v1.1 Blind Round 2

## Verdict

The independent frozen round-2 verdict is **failed**.

- Total: 8/8 executed.
- Passed: 5.
- Failed: 3.
- Pure compressor pass rate: 3/5.
- Strict negative pass rate: 0/1.
- Hybrid boundary contract pass rate: 2/2.

The result is evaluation evidence only. No recognizer, control implementation, manifest
expectation, or runner behavior changed after freeze. The opened eight products are no longer
blind and may only be used as disclosed regression evidence.

## Freeze Integrity

| Item | SHA-256 |
| --- | --- |
| Manifest | `f3708c22660060047da1dbdf0831aa048321cb99cd000ba0fb08443efb6ba799` |
| Runner | `408ac64422da1fa12463a5dfe7fcd0efab7e89da77b9e8c91f25bd8ae5d4c10c` |
| Production Agent | `174a2d42766c1ad4de867251c1a8d139f43d973fad8d43ef3c972d4b1afc928b` |
| Recognizer/control source set | `bf55da91e598a238b69cc56a870727cbf88161f54f6199fbd46a1e9711b93cfd` |
| Source v1 reserve manifest | `a9f62a7cda18901f711109a9c625011619efb5718004cac990018627dd3d1e22` |

Freeze verification passed before and after execution with zero mismatches. Preflight uniquely
resolved only the eight main identities and read zero parameter surfaces. The four remaining
reserve plug-ins were not loaded and retain a recorded parameter-read count of zero.

## Results

| Case | Frozen expectation | Result | Finding |
| --- | --- | --- | --- |
| bx_opto | compressor | pass | `amount_driven`; Reduction Amount/Mix exact apply and restore |
| NEOLD U2A | compressor | fail | False negative: `not_compressor` |
| Millennia TCL-2 | compressor | pass | `threshold_driven`; Ratio/Attack exact apply and restore |
| Kiive XTComp | compressor | pass | `multi_path_channel_control`; Input/Ratio exact apply and restore |
| Lindell MU-66 | compressor | fail | Rejected `unsupported_multiband_compressor`; frozen oracle was too broad |
| bx_clipper | not compressor | fail | False positive: `input_driven_fixed_transfer`, confidence 0.96 |
| ADPTR Sculpt | hybrid boundary | pass | Separate broadband stage accepted; Threshold/Ratio exact apply and restore |
| bx_masterdesk Pro | hybrid boundary | pass | Safe rejection `unsupported_de_esser`; snapshot unchanged |

All four accepted apply cases returned actual readback, an exact `restore_ref` restore, and zero
complete-snapshot drift. No failed case invoked apply. All eight temporary tracks were deleted and
the final cleanup audit found no `PA blind v2` track.

## Failure Taxonomy

### NEOLD U2A: genuine minimal-leveler false negative

The 14-parameter surface exposes Peak Reduction, Recovery, Compress/Limit Mode, Gain, Drive and
Mix. Despite the explicit amount and timing evidence, no supported control path was emitted. This
is a generic fixed-transfer/amount-driven corroboration gap, not justification for an identity
allowlist.

### Lindell MU-66: correct multiband boundary, incorrect frozen label

The disclosed surface contains separate Low and High Input, Threshold, DC Threshold, Time
Constant and Output families plus a Crossover control. The current `unsupported_multiband_compressor`
result is consistent with the v1.1 boundary. This frozen failure must remain in the verdict, but a
future disclosed regression should classify the product as multiband rather than weaken the
broadband recognizer.

### bx_clipper: genuine clipper false positive

Two indexed channels expose Input Level, Type, Knee, Ceiling, Mix and Output. There is no
threshold, ratio or compression-amount control. The recognizer treated input plus transfer-like
mode/knee evidence as `input_driven_fixed_transfer`; clipper/ceiling evidence needs to veto that
weak inference unless another independently provable broadband compression stage exists.

## Sealed Reserve

The following products remain unopened in this round:

- HUM Audio Devices LAAL
- Shadow Hills OptoMax
- elysia nvelope
- Lindell 354E

They must not be used during development of fixes inferred from this round if they are intended to
remain an independent follow-up set.

## Evidence

Authoritative artifacts are under:

`D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\product_path_20260804_101930\compressor_control\plugin_alliance_blind_v2`

- `freeze.json`: frozen identities, policy, hashes and zero-read declarations.
- `run/summary.json`: machine-readable verdict and all case outcomes.
- `run/cases/*/evidence.json`: raw load, parameter, inspect, apply, restore and cleanup evidence.
- `conclusion.json`: frozen-result interpretation and failure taxonomy.
- `evidence_index.json`: SHA-256 index of the complete evidence set.
