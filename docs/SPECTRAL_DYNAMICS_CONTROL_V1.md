# Spectral Dynamics Control V1

Status: inspect-only boundary frozen; no controller or Profile mapping is defined.

## Evidence rule

The positive cluster is identity-free and requires all of the following from the live parameter surface:

1. A frequency-indexed field with at least three host-controllable frequency bindings.
2. A matching indexed threshold field with at least three bindings sharing the same index set. Indexed Q is supporting evidence, not a substitute for indexed thresholds.
3. One shared, non-indexed dynamics law containing operating point, transfer/direction, and timing evidence. A global threshold plus ratio/expander-ratio/knee and attack/release is the minimum current proof.
4. The surface must not be better explained by repeated EQ gain cells, a filterbank with crossovers, a de-esser focus stage, or a limiter/clipper safety stage.

Capture, Learn, Freeze, Analyzer, and UI state controls are auxiliary evidence only. Product names, vendor names, paths, categories, presets, and the word Capture are never recognition rules.

## Boundaries

- `spectral_field_global_dynamics`: positive inspect result only.
- `unsupported_dynamic_eq`: repeated indexed frequency + gain plus indexed threshold or dynamic-range cells. Q alone is static-EQ evidence and does not prove dynamics.
- `unsupported_multiband_dynamics`: crossover or multiband structure is observable.
- `unsupported_de_esser`: explicit frequency-selective sibilance/de-esser auxiliary structure is observable.
- `unsupported_limiter`: a limiter/maximizer safety stage is observable.
- `unsupported_clipper`: a clipper stage is observable and remains a separate future type.
- `unresolved_spectral_dynamics_surface`: a spectral field is visible but the shared global law is not proven.
- `unsupported_broadband_compressor`: a global law exists without a spectral field.
- `not_spectral_dynamics`: no sufficient spectral evidence.

No apply path is exposed in v1. All positive results carry an observation model requiring an STFT/time-frequency gain field, frequency-bin alignment, time resolution, scope identity, and deterministic read-only probing. Latency, lookahead, channel link, and change-delta are optional observations until fixtures prove their stability.

## Observation model draft

The future observation payload is deliberately separate from COM's broadband taps:

```json
{
  "schema_version": "spectral-dynamics-observation/v1",
  "representation": "stft_gain_field",
  "scope": {"track_id": "...", "plugin_id": "...", "stage_key": "spectral"},
  "frequency_axis": {"centers_hz": [], "bin_count": 0, "mapping": "stable_bin_centers"},
  "time_axis": {"frame_count": 0, "hop_ms": 0, "window_ms": 0, "alignment": "input_output"},
  "gain_field": {"units": "dB", "matrix_ref": "...", "baseline_ref": "..."},
  "determinism": {"repeat_count": 2, "same_input_same_field": false},
  "change_delta": {"available": false, "changed_bins": 0, "changed_frames": 0},
  "latency": {"available": false, "samples": 0},
  "status": "inspect_only"
}
```

An observation is insufficient for future control if the frequency grid is not stable, if input/output alignment is unknown, if the gain field is only a single broadband scalar, or if repeated probes are not deterministic. A display analyzer or a `Capture`/`Freeze` state label cannot substitute for this payload.

## Local evidence

- DSM V3 exposes `Dynamic Mode`, compressor/expander ratios, global threshold, attack/release/knee, and indexed `Frequency 1..3`, `Threshold 1..3`, `Q 1..3`. `Capture` and `Freeze Gain` occur as auxiliary state controls. This is the current training positive candidate, pending audio observation.
- Curves Equator exposes many `Node N Frequency/Gain/Q` cells plus global `Attack`, `Release`, and `Threshold Rider On/Off`. The disclosed surface is an EQ-node field with no independently proven indexed dynamic threshold law, so it remains unresolved/negative for this v1 rule.
- FabFilter Pro-Q dynamic-EQ evidence exposes repeated `Band N Frequency`, `Gain`, `Dynamic Range`, `Threshold`, and `Q` cells. This is a dynamic-EQ negative and must not be promoted by the presence of frequency and threshold alone.
- FabFilter Pro-MB exposes repeated per-band threshold/attack/release cells and is a multiband negative.
- TDR Nova remains a dynamic-EQ negative; its parameter surface is not used as a positive rule source.

Previously disclosed Plugin Alliance products are regression samples, not sealed blind tests. Curves Resolve remains sealed and is not opened in this round.
