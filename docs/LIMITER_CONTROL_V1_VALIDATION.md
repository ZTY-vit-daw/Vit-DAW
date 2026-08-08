# Limiter Control V1 Validation

Date: 2026-08-06

## Current implementation verdict

The current Limiter V1 implementation passes the disclosed topology matrix,
typed apply/readback/restore smokes, failure-injection rollback tests, and the
full Go test suite. It preserves the broadband Compressor recognizer and uses
independent `l1cr1_` / `l1rr1_` authority references.

## Disclosed matrix

Latest artifact:

`VitApp/Workspace/Artifacts/smoke/limiter_control/20260806_163422`

| Case | Expected boundary | Result |
| --- | --- | --- |
| L1 limiter Stereo | threshold-ceiling-time | pass; two identical generations |
| ZaMaximX2 | no proved limiter stage | pass; `unsupported_broadband_compressor` |
| L2 Stereo | threshold-ceiling-time | pass; two identical generations |
| bx_limiter True Peak | drive-ceiling-time | pass; two identical generations |
| HUM Audio Devices LAAL | indexed multi-stage limiter | pass; two stages; two identical generations |
| bx_clipper | clipper boundary | pass; `unsupported_clipper` |
| Pro-L 2, post-blind regression | drive/peak-output-ceiling/time | pass; two identical generations |
| synthetic broadband compressor | compressor boundary | pass by unit fixture |
| synthetic multiband dynamics | multiband boundary | pass by unit fixture |

The known-sample typed write smoke passed on L1 limiter Stereo, bx_limiter
True Peak, and the post-blind Pro-L 2 regression. Each case selected a measured
target, returned `exact`, preserved topology generation, restored the `l1rr1_`
preimage, and ended with zero normalized parameter differences.

## Sealed one-shot run

Artifact:

`VitApp/Workspace/Artifacts/smoke/limiter_control/20260806_162804`

Freeze facts:

- parameter surfaces read before freeze: 0;
- execution count per case: 1;
- manifest SHA-256:
  `d46528562034d22898ade17cb80b9991c1dfe284eb566a55baa2bc708b817879`;
- frozen recognizer/control composite SHA-256:
  `6906531ebf5ecd538c0a68ac473be28a24b2a639bd52117680716eaf434ac5f3`;
- freeze intact before and after execution: true;
- disposable tracks were deleted.

| Sealed case | One-shot result |
| --- | --- |
| L4 Ultramaximizer Stereo | pass: `threshold_ceiling_time`; generation stable across two reads; threshold apply exact; exact full preimage restored |
| Pro-L 2 | fail: `unresolved_limiter_surface` |

The Pro-L failure was caused by a real missing structural rule. Its surface
exposes `Gain`, `Output Level` measured in dBTP, `Lookahead`, and `Release`, but
does not expose a parameter containing the word Limiter. The frozen recognizer
dropped both unqualified Gain and Output Level. After the one-shot run, Pro-L
was moved to regression. The current rule accepts a measured peak-unit output
as ceiling evidence, permits the same stage's unqualified drive, and still
rejects ratio/knee compressor transfer unless an explicit limiter stage exists.

The sealed Pro-L execution was not repeated. Therefore the historical sealed
report verdict remains failed even though the current implementation passes
the exact disclosed topology and a subsequent live typed apply/restore
regression. These are separate claims.

## Transaction and authority evidence

Automated tests cover:

- stable generation independent of product identity and current values;
- threshold-ceiling-time, drive-ceiling-time, indexed stages, and shared controls;
- compressor, clipper, multiband, and gate/expander boundaries;
- peak-unit output ceiling recovery without product identity;
- target, generation, section, role, parameter, and checksum-bound control refs;
- stale ref and unit mismatch rejection before writes;
- one atomic batch for typed writes;
- exact normalized restore refs;
- full preimage rollback after batch failure or unplanned parameter changes;
- Limiter/Compressor control and restore refs are not interchangeable;
- generic writes are blocked only for Limiter-owned bindings;
- typed inspect/apply workflow calls do not escape to the generic executor;
- read-only inspect versus undoable typed apply catalog/policy classification.

Verification commands:

```powershell
cd D:\Vit_DAW\agent
go test ./...

cd D:\Vit_DAW
python -m py_compile scripts/limiter_blind_smoke.py scripts/limiter_matrix_smoke.py scripts/limiter_known_apply_smoke.py
```

Both completed successfully after the Pro-L correction.

## Residual claim boundary

The current implementation is validated as a Limiter V1 control surface. It
does not make Maximizer a processing primitive, does not control clipper or
multiband stages, does not create product mappings, and does not claim a
successful sealed Pro-L blind pass. A future blind round must use a new,
previously undisclosed product; Pro-L 2 and L4 are now regression samples.
