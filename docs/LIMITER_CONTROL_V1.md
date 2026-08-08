# Limiter Control V1

## Scope

Limiter V1 is an identity-independent, stage-first topology and typed control
contract. A product name, vendor, category, current parameter value, or the word
`Maximizer` never establishes the type. A maximizer product is accepted only
when its live parameter surface independently proves a limiter stage.

Schema: `limiter-control-topology/v1`.

## Deterministic recognition gate

One control scope is a limiter stage only when the same scope contains all
three structural relationships:

1. an operating point: `threshold` or `input_drive`;
2. a safety bound: `ceiling` or a uniquely promoted main output level inside
   an explicitly marked limiter stage;
3. a time/detector term: `release` or `lookahead`.

An unqualified `Gain` is accepted as input drive only when the scope also has
explicit limiter-stage evidence. An unqualified output is not a ceiling unless
the scope is explicitly a limiter and has no named ceiling. When both Output
Level and Output Trim exist, only the main Output Level may be promoted.

Threshold, drive, ceiling, or release in isolation is insufficient. Ratio or
knee evidence without an explicit limiter stage rejects as broadband
compressor. Clipper evidence, a drive/ceiling/shape surface without time
evidence, and band/crossover dynamics reject into their own boundaries.

## Published topology

Each `limiter_stages[]` row publishes:

- `operating_point`: threshold and input drive;
- `safety`: ceiling;
- `timing`: release, lookahead, and any proved attack/hold control;
- `detector`: true peak and channel link;
- `mode`: auto release, limiter mode, oversampling, stage enable;
- `output`: output gain and mix;
- role capabilities and a stable stage key.

Unindexed controls alongside indexed limiter stages are published as
`shared_controls`. Clipper, compressor, gate/expander, de-esser, and multiband
evidence is retained in `auxiliary_stages`; it is never exposed as a limiter
control binding.

Topology generation is computed from the structural bindings, observed control
domains, curves, reachable values, and auxiliary boundaries. Product identity
and current values are excluded.

## Typed inspect and apply

`plugin_grabber.inspect_limiter` reads the live parameter surface and returns
`l1cr1_` control references bound to track, plugin instance, topology
generation, stage, section, role, and parameter ID.

`plugin_grabber.apply_limiter_controls` accepts only returned references and
exactly one explicit target per control:

- `value_db`: threshold, input drive, ceiling, output gain;
- `value_ms`: release, lookahead, and proved timing controls;
- `percent`: channel link and mix;
- `enum_label`: true peak, limiter mode, oversampling, stage enable, or another
  binding with a measured reachable label;
- `display_value`: measured numeric domains only.

Every request is atomic. Before writing, the executor rereads the complete live
surface, re-runs recognition, rejects stale target/generation/reference/unit
mismatches, and captures the full touched preimage. Writes use measured display
curves or reachable values; ambiguous domains use a reversible transactional
direction probe. Readback failure, unexpected parameter changes, or any write
failure restores the full preimage. Success returns `exact` or `quantized` plus
an `l1rr1_` restore reference.

Generic `plugin.set_parameter` writes are rejected for limiter-owned bindings.
Compressor-owned bindings in a hybrid remain protected by the separate
compressor typed path. The two reference kinds are not interchangeable.

## V1 boundaries

- Maximizer: not a primitive; extract only a proved limiter stage.
- Broadband compressor: ratio/knee/compression-transfer topology without an
  explicit limiter stage is rejected.
- Clipper: clip/type/knee or drive-ceiling-shape without release/lookahead is
  rejected as `unsupported_clipper`.
- Multiband dynamics: band-indexed or crossover-linked dynamics is rejected as
  `unsupported_multiband_dynamics`.
- Gate/expander: gate, expansion, range/hysteresis topology is rejected as
  `unsupported_gate_expander`.
- Unresolved partial surfaces: no typed controls are issued.

## Validation matrix

The authoritative sample split is `scripts/limiter_control_matrix.json`.

- Training: L1 limiter Stereo and ZaMaximX2. Full parameter surfaces may inform
  rules; ZaMaximX2 remains a negative maximizer boundary because its exposed
  Input Gain/Threshold/Release surface does not independently prove a ceiling.
- Regression: L2, bx_limiter True Peak, LAAL, bx_clipper, and synthetic
  broadband-compressor/multiband negatives.
- Sealed blind: Pro-L 2 and L4 Ultramaximizer Stereo. Before freeze only
  identity, path, and binary fingerprint may be read. Parameter surfaces,
  screenshots, and manuals are forbidden. Each case has one post-freeze run.

The blind claim is valid only when source/runner/manifest/agent hashes are
unchanged before and after execution, the disposable project is removed, both
inspect reads return the same topology generation, all applied parameters are
restored to their exact normalized preimage, and each case execution count is
one.

The 2026-08-06 sealed run met the freeze and one-shot requirements. L4
Ultramaximizer Stereo passed inspect twice and completed exact apply/restore.
Pro-L 2 initially returned `unresolved_limiter_surface`; the disclosed
parameter evidence showed a peak-unit `Output Level` ceiling, so that rule was
corrected and Pro-L 2 was then added to regression. The Pro-L blind execution
was not repeated; its original failed result remains authoritative for the
sealed run, while the post-run regression proves the corrected rule and typed
transaction path.
