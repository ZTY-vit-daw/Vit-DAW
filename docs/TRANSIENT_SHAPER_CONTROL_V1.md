# Transient Shaper Control V1

## Scope

Transient Shaper Control V1 recognizes and controls one identity-independent transient-envelope stage from the live parameter surface. It does not use product or vendor identity, saved mappings, Profile, VPS, SPAL, open semantic routing, or C2.

The contract preserves the broadband Compressor boundary. An `Attack` parameter is transient action evidence only when its observed physical domain is signed around zero and is not a time domain. `Attack` in `ms` is timing and cannot establish a transient-shaper type.

## Accepted topology clusters

### Paired envelope action

Required evidence:

- one host-controllable signed `attack_amount` axis;
- one host-controllable signed `sustain_amount` axis;
- each measured display domain or display curve spans negative and positive values;
- the physical unit is `dB`, `%`, or an observed unitless `-100..100` action scale exposed as `%` by the typed contract.

Optional controls are detector sensitivity, per-envelope duration, per-envelope shape, focus frequency, mix, and output gain.

### Single envelope range

Required evidence:

- one signed `transient_range` axis;
- a detector sensitivity axis;
- both duration and release timing axes.

This cluster covers a single wideband transient envelope without inventing attack/sustain roles that the parameter surface does not publish.

## Mode-remapped roles

A mode parameter is role evidence only when its reachable labels prove a remapping family such as `Full Range / Dual Band / Shelf EQ`.

- `Full Range`: `Attack/Gain H` and `Sustain/Gain L` may map to `attack_amount` and `sustain_amount` when both axes are signed.
- `Dual Band`: reject with `unsupported_dual_band_mode`.
- `Shelf EQ`: reject with `unsupported_eq_mode`.
- unknown current label: reject with `unresolved_processing_mode`.

The v1 apply tool rejects a requested processing-mode change with `mode_change_requires_reinspect`. An external mode change therefore invalidates the supported topology or changes its interpretation before any later typed write can proceed.

## Typed topology

`plugin_grabber.inspect_transient_shaper` returns:

```text
transient_shaper_stage
  envelope_action: attack_amount, sustain_amount, transient_range
  detector: attack_sensitivity, sustain_sensitivity, detector_sensitivity, focus_enable, focus_mode, focus_frequency
  timing: attack_duration, sustain_duration, duration, release
  shape: attack_shape, sustain_shape
  mode: processing_mode
  output: mix, output_gain
```

Every writable binding has a checksum-protected, instance-specific, topology-generation-scoped `control_ref`. Topology generation excludes plugin identity and current continuous values. It includes the semantic mode, observed domains, curves, reachable enum labels, roles, parameter IDs, and auxiliary stage structure.

## Auxiliary stages

Limiter, clipper, and combined limiter/clipper parameters are reported only in `auxiliary_stages`. They never receive a transient-shaper `control_ref` and cannot be written by `plugin_grabber.apply_transient_shaper_controls`.

This includes an output `Limit` switch and compound `Off / Clip / Limit` guard selectors. They require their own typed processor boundary.

## Adjacent boundaries

V1 fails closed with stable codes for:

- `unsupported_broadband_compressor`;
- `unsupported_gate_expander`;
- `unsupported_limiter`;
- `unsupported_de_esser`;
- `unsupported_multiband_dynamics`;
- `unsupported_spectral_dynamics`;
- `unsupported_clipper`;
- `unresolved_transient_shaper_surface`;
- `not_transient_shaper`.

Multiband transient processors are outside v1 even when each band performs transient shaping.

## Apply contract

`plugin_grabber.apply_transient_shaper_controls` requires `atomic:true` and either explicit controls or one `restore_ref`.

Accepted target fields are `value_db`, `percent`, `value_ms`, `frequency_hz`, `display_value`, and `enum_label`. Exactly one target field is allowed per control. Units must match both the role and the observed domain.

All controls are planned against one live topology generation. Execution captures a full parameter preimage, writes through one atomic batch, reads back all parameters, rejects unplanned side effects, and restores the full preimage on any failure. A successful apply returns an exact normalized `restore_ref` for an independent restore call.
