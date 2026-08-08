# Gate / Expander Control V1

## Scope

This layer controls one identity-independent, single shared hard-gate or
provably downward-expanding stage from the live host parameter surface. It is
separate from broadband compressor, limiter, de-esser, transient-shaper,
multiband, spectral-dynamics, and clipper control.

The minimum hard-gate graph is:

```text
detector -> threshold -> range/reduction -> attack|hold + release
```

An expander is accepted only when its direction is independently proven by a
reachable Gate/Expander direction enum or by a measured expansion-ratio domain
whose observed values are all at least 1 and include values greater than 1.
An isolated parameter named `Expander Ratio` is not enough.

A paired-direction surface may also prove the downward path when it contains
one unqualified threshold/ratio path, a complete explicitly `Upward`
threshold/ratio pair, a measured positive-dB attenuation-depth Range, and
shared attack/hold plus release timing. The Upward pair remains an auxiliary
stage and never becomes a V1 typed control.

## Evidence boundary

Recognition consumes host-controllable parameter names/raw names/aliases,
read-only display probes, measured physical domains, and reachable enum labels.
It does not consume product identity, vendor, path, presets, screenshots,
DisplayGroup, generic normalized roles, current values, Profile, VPS, or SPAL.

G8-style MIDI trigger and indexed/multichannel detector controls are retained
as `unsupported_capabilities` and never become typed bindings. Unindexed SC
HPF/LPF controls may become detector bindings.

Threshold plus ratio plus attack/release without range, hysteresis, gate
evidence, or a proved direction remains a broadband-compressor boundary. A
direction enum may be written only with a reachable label containing Gate,
Expander/Expansion, or Downward evidence; Compressor and Upward labels reject.

## Inspect and apply

The schema is `gate-expander-control-topology/v1`. Inspect publishes one
`gate_expander_stage`, a structural generation, and checksum-protected
`g1cr1_` control references. Apply accepts only those references and exactly
one of `value_db`, `ratio`, `value_ms`, `frequency_hz`, `percent`,
`display_value`, or `enum_label`. It is atomic-only, verifies every readback,
rejects unplanned parameter changes, and restores the complete normalized
preimage on failure. Successful writes return an exact normalized `g1rr1_`
restore reference. Generic `plugin.set_parameter` is blocked for owned typed
bindings.

V1 controls threshold, range/reduction, hysteresis, attack, hold, release,
lookahead/cycle delay, detector HPF/LPF, direction enum when proven, mix, and
output gain. MIDI trigger, multichannel detector, upward expansion, and
unresolved display domains are not typed controls.
