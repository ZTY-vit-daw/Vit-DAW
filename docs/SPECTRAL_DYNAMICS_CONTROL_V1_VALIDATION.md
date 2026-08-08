# Spectral Dynamics V1 Validation

## Frozen evidence matrix

| Surface | Spectral field | Shared global law | Adjacent veto | v1 result |
|---|---|---|---|---|
| DSM V3 | `Frequency 1..3` + `Threshold 1..3` + `Q 1..3` | one global threshold, two global ratios, attack/release/knee | no repeated gain cells or crossover bank disclosed | training positive candidate |
| Curves Equator | `Node N Frequency/Gain/Q` | global attack/release and rider threshold control | static EQ gain/Q cells; no indexed dynamic threshold or range law | `unresolved_spectral_dynamics_surface` |
| FabFilter Pro-Q dynamic surface | repeated `Band N Frequency/Gain/Dynamic Range/Threshold/Q` | any global timing is not sufficient | dynamic-EQ repeated cells with matching dynamic fields | `unsupported_dynamic_eq` |
| FabFilter Pro-MB | repeated per-band threshold/attack/release | shared controls do not replace per-band law | filterbank/repeated multiband cells | `unsupported_multiband_dynamics` |
| TDR Nova | dynamic EQ band controls | no proved shared spectral law | dynamic-EQ negative | `unsupported_dynamic_eq` or unresolved |
| Curves Resolve | sealed | sealed | no parameter surface inspected | sealed blind sample |

## Tests

The read-only evidence census is reproducible with `scripts/spectral_dynamics_evidence_audit.ps1`. It scans disclosed parameter snapshot names only; it does not instantiate plugins, open editors, or write project/plugin state.

The topology package covers:

- positive `spectral_field_global_dynamics` structure;
- generation independence from plugin identity and current values;
- dynamic-EQ rejection from repeated frequency/gain/threshold cells;
- unresolved field without a global law;
- Capture/Freeze-only non-recognition;
- read-only inspect command and catalog/policy registration.

The inspect workflow performs one live `get_plugin_parameters` read and never writes a parameter. No control references or apply command are emitted.

## Sample partition

The local inventory exposes eight Waves Curves-family entries: Equator Mono/Stereo, Equator Live Mono/Stereo, Resolve Mono/Stereo, and Resolve Live Mono/Stereo. This is an inventory count only, not a topology rule. FabFilter contributes no positive Spectral Dynamics candidate in the disclosed set: Pro-Q is a dynamic-EQ negative and Pro-MB is a multiband negative.

- Training: DSM V3 and synthetic identity-free structural fixtures; full parameter surfaces may be reviewed.
- Regression: disclosed Plugin Alliance surfaces, Curves Equator, FabFilter Pro-Q, TDR Nova, and Pro-MB.
- Sealed blind: Curves Resolve and one new non-disclosed candidate per future collection pass. Their parameter surface, screenshots, manuals, and product identity are excluded from rule design.

## Observation boundary

Positive inspection is not sufficient for a controller. A future observation probe must establish an STFT/time-frequency gain field, frequency-bin alignment, time resolution, scope identity, deterministic repeatability, and change-delta behavior. Latency/lookahead and channel-link behavior remain optional until independently observed.

The six independent follow-up task specifications are in `docs/SPECTRAL_DYNAMICS_DEVELOPMENT_TASKS_V1.md`.
