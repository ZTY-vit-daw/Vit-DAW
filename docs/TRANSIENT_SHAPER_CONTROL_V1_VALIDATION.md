# Transient Shaper Control V1 Validation

Validation date: 2026-08-07.

## Automated tests

The focused regression command passed:

```powershell
cd D:\Vit_DAW\agent
go test ./internal/workflows/plugingrabber ./internal/chat ./internal/tools ./internal/toolpolicy -count=1
```

Coverage includes:

- signed `dB` and unitless signed attack/sustain action axes;
- rejection of compressor `Attack` timing in `ms`;
- paired-envelope and single-envelope-range clusters;
- mode-remapped Full Range roles;
- explicit Dual Band and Shelf EQ rejection;
- identity/current-value independent generation;
- auxiliary limiter/clipper isolation;
- generation-scoped control and restore references;
- full-preimage rollback after batch failure or unplanned parameter change;
- catalog, mutation policy, and GoalRunner integration.

## Disposable-project census

Artifact root:

```text
VitApp/Workspace/Artifacts/smoke/transient_shaper_control/20260807_112135
```

Only `plugin.get_parameters` was called on the loaded plugin instances. The runner copied `default_project.xml`, used disposable tracks, made no parameter writes, and deleted the tracks.

Observed training surfaces:

| Case | Structural evidence |
| --- | --- |
| Smack Attack Stereo | Signed `Attack -100..100` plus signed `Sustain -100..100`; per-envelope sensitivity, duration, and shape; `Guard Off/Clip/Limit` isolated as auxiliary |
| TransX Wide Stereo | Signed `Range -24..18 dB`; `Sense -10..10 dB`; `Duration 0.01..500 ms`; `Release 0.5..500 ms` |

## Real inspect matrix

Positive artifact root:

```text
VitApp/Workspace/Artifacts/smoke/transient_shaper_control/20260807_121403
```

| Case | Result | Stability |
| --- | --- | --- |
| Smack Attack Stereo | `paired_envelope_transient_shaper` | same generation on two reads |
| TransX Wide Stereo | `single_envelope_range_transient_shaper` | same generation on two reads |

Boundary artifact root:

```text
VitApp/Workspace/Artifacts/smoke/transient_shaper_control/20260807_121553
```

| Case | Required rejection | Result |
| --- | --- | --- |
| Pro-C 2 | `unsupported_broadband_compressor` | passed twice |
| Pro-MB | `unsupported_multiband_dynamics` | passed twice |
| FabFilter Saturn 2 | `unsupported_multiband_dynamics` | passed twice |
| elysia nvelope (default Dual Band) | `unsupported_dual_band_mode` | passed twice |
| TransX Multi Stereo | `unsupported_multiband_dynamics` | passed twice |

The same regression run loaded `SPL Transient Designer Plus` and recovered the paired signed axes with a stable generation. Named SPL and nvelope Full Range fixtures use their disclosed parameter shapes; the live nvelope default is intentionally a mode-boundary rejection.

The FabFilter results are negative structural boundaries, not identity rules. No dedicated local FabFilter transient-shaper primitive was found.

## Real apply and restore

Artifact root:

```text
VitApp/Workspace/Artifacts/smoke/transient_shaper_control/20260807_121800
```

| Case | Typed write | Apply | Restore |
| --- | --- | --- | --- |
| Smack Attack Stereo | `attack_amount = 25%`, `sustain_amount = -20%` in one atomic request | exact | exact, full snapshot restored |
| TransX Wide Stereo | `transient_range = 3 dB` | exact | exact, full snapshot restored |

The disposable project and tracks were deleted after each case. No user project was opened or changed.
