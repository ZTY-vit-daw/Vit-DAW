# Generic Broadband Compressor Control v1.1 Validation

## Outcome

Generic broadband compressor control v1.1 passed its implementation and product-path gates on
2026-08-04. It does not implement multiband compressor, limiter, gate/expander, de-esser, or
spectral-dynamics control. It identifies those adjacent topologies explicitly and leaves their
parameters untouched.

The production Godot-owned Agent binary is:

`174A2D42766C1AD4DE867251C1A8D139F43D973FAD8D43EF3C972D4B1AFC928B`

## Implemented Generic Fixes

- Section-aware frequency-band evidence separates sidechain EQ from gain-reduction bands.
- Named sequential broadband stages such as Optical and Discrete are emitted as separate control
  paths (`multi_stage_serial_control`); this is distinct from frequency bands.
- Named Low/Mid/High compressor cells and indexed spectral threshold/frequency arrays have distinct
  unsupported boundaries.
- Limiter and gate auxiliary evidence vetoes weak threshold/timing or input/timing surfaces, while a
  separately provable broadband stage remains controllable in hybrid products.
- Input-driven fixed-transfer levelers use response/mode plus corroborating structural evidence.
- `-INF` display endpoints are excluded from finite physical inversion.
- Successful apply returns an exact normalized `restore_ref`; rounded physical text is not used as
  an undo preimage.
- Broken/context-dependent `value_to_string` surfaces use a bounded apply-time local direction probe
  with probe restore, local bracketing, final readback, and full rollback on failure.

No plugin/vendor identity allowlist, Profile, VPS, or SPAL mapping was added.

## Verification

| Gate | Result | Evidence |
| --- | --- | --- |
| Go test suite | passed | `go test ./... -count=1` |
| Original training topology matrix | 22/22 positive, 2/2 boundary | `compressor_control/v11_training_matrix/summary.json` |
| Representative real apply/restore | 4/4 exact restore | `compressor_control/v11_apply_matrix.json` |
| Disclosed Plugin Alliance round-1 regression | 18/18 | `compressor_control/plugin_alliance_regression_v11/summary.json` |
| Godot-owned product path | passed | `product_path_20260804_094717/summary.json` |
| Vit natural-language explicit control | passed | `compressor_control/v11_nl_control.json` |

All artifact paths above are relative to:

`D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\product_path_20260804_094717`

The regression run reports zero leftover temporary tracks and zero reads from the sealed reserve
set. The original round-1 blind report and evidence remain unchanged; v1.1 does not claim that its
post-fix regression is blind.

## Regression Boundaries

The six disclosed negative cases now resolve as:

| Surface | Boundary |
| --- | --- |
| bx_limiter True Peak | `unsupported_limiter` |
| Lindell MBC | `unsupported_multiband_compressor` |
| SPL De-Esser Dual-Band | `unsupported_de_esser` |
| SPL Transient Designer Plus | `not_compressor` |
| Unfiltered Audio G8 | `unsupported_gate_expander` |
| Pro Audio DSP DSM V3 | `unsupported_spectral_dynamics` |

The next compatibility expansion must add a new topology module rather than weakening these
broadband-compressor boundaries.
