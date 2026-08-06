# Plugin Alliance Compressor Control v1.2 Final Blind Round 3

## Verdict

The frozen final round-3 verdict is **passed**.

- Total: 4/4 passed.
- Broadband compressor: 1/1 passed.
- Adjacent-type boundaries: 3/3 passed.
- Full-snapshot drift: 0 parameters across all four cases.
- Temporary blind tracks remaining: 0.

This was the only execution of the final four-case reserve. All four products are now disclosed
regression evidence; no sealed Plugin Alliance compressor reserve remains.

## Freeze Integrity

| Item | SHA-256 |
| --- | --- |
| Manifest | `b649c52ff9e0f13c60a7050464e387a71cf222362989fa789c16f3436426c9d6` |
| Runner | `d37e8582f4775a790783e9d07f50c79fac9a001a2f7c8110da14bb40fc11dbc3` |
| Production Agent | `47877c3c79f4ba9e0f8663d4d4cbb9799f78f90bf797cb6b52365311d07a2585` |
| Recognizer/control closure | `be5351847b2f500b68b7cb86b33c7359c40c02bcddf0c2b9b648e6863659a5c5` |

Identity-only preflight uniquely resolved all four products and read zero parameter surfaces.
Freeze verification passed immediately before and after execution with no file mismatch. The
manifest, runner, Agent, recognizer, control implementation, and expectations did not change
during execution.

## Frozen Oracle

Before freeze, the old broad `compressor` label for HUM Audio Devices LAAL was corrected to the
limiter boundary. The correction used only its already disclosed identity and public product
category; the plug-in was not loaded and its parameters were not read. This prevents a known
product-taxonomy mistake from being scored as a recognizer failure, as happened with MU-66 in
round 2.

| Product | Frozen expectation |
| --- | --- |
| Shadow Hills OptoMax | supported broadband compressor |
| HUM Audio Devices LAAL | `unsupported_limiter` |
| elysia nvelope | `not_compressor` |
| Lindell 354E | `unsupported_multiband_compressor` |

## Results

| Product | Result | Control and safety evidence |
| --- | --- | --- |
| Shadow Hills OptoMax | pass: `threshold_driven`, confidence 0.90 | Stable generation; threshold and input drive applied; exact typed readback and `restore_ref`; all 22 parameters restored |
| HUM Audio Devices LAAL | pass: `unsupported_limiter` | No apply; all 36 parameters unchanged |
| elysia nvelope | pass: `not_compressor` | No apply; all 11 parameters unchanged |
| Lindell 354E | pass: `unsupported_multiband_compressor` | No apply; all 42 parameters unchanged |

All four `track.delete` calls succeeded. A separate final `track.list` audit found only the
pre-existing `Track 1` and no `PA blind v3` track.

## Evidence

Authoritative artifacts are stored under:

`D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\product_path_20260804_112622\compressor_control\plugin_alliance_blind_v3`

- `freeze.json`: frozen identities, policies, hashes, Agent health and zero-read declaration.
- `run/freeze_verification_before.json`: pre-execution integrity check.
- `run/cases/*/evidence.json`: complete load, snapshot, inspect, apply/restore and cleanup evidence.
- `run/summary.json`: machine-readable one-shot result.
- `run/freeze_verification_after.json`: post-execution integrity check.
- `conclusion.json`: final interpretation and safety summary.
- `evidence_index.json`: SHA-256 index of the evidence set.

The third-round pass closes the Plugin Alliance blind evaluation phase for generic single-band
compressor control. Future work may use all three rounds as disclosed regression data, but it must
not describe any of these products as an unopened blind set.
