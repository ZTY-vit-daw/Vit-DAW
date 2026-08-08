# Gate / Expander Control V1 Validation

## Gates

The focused suite is:

```powershell
cd D:\Vit_DAW\agent
go test ./internal/workflows/plugingrabber ./internal/chat ./internal/tools ./internal/toolpolicy -count=1
```

## Real training surfaces

The first live training pass uses four installed products captured through the
read-only parameter bridge and display probes. Product identity is not used by
the recognizer; these names identify the evidence record only.

| Surface | Live evidence | V1 result |
| --- | --- | --- |
| C1 gate Stereo | `Gate Open`, `Gate Close`, `Floor`, `Attack/Hold`, `Release`, `Gate/Expander` | `hard_gate` |
| C1 comp-gate Stereo | Gate-prefixed stage plus `Comp`/`Strap` controls | Gate stage `hard_gate`; compressor controls auxiliary |
| PSE Stereo | `Threshold`, negative-dB `Range`, `Release`, unindexed `HPF Freq`/`LPF Freq` | `downward_expander` |
| Pro-G | Unqualified threshold/ratio path, complete `Upward` threshold/ratio pair, positive-dB `Range`, state timing | `downward_expander`; Upward pair auxiliary |

The C1 composite surface is stage-split before binding. PSE is accepted as a
release-only downward-expander cluster only when the measured Range domain is
negative dB through 0 dB and at least one detector high/low-pass control is
present. Pro-G is accepted through paired-direction structural proof; its
Upward threshold/ratio controls remain outside V1 typed control.

Live disposable-project apply/restore validation passed for both PSE and
Pro-G Threshold controls: apply returned `exact`, restore returned `exact`,
and the final normalized value equaled the complete preimage. Pro-G reports
the Upward pair as `upward_expander` auxiliary evidence and keeps MIDI plus
left/right side-chain controls in unsupported capabilities.

Topology tests cover the G8 threshold/range/state-timing cluster, detector
HPF/LPF bindings, C1 bidirectional compressor rejection, unproven expander
ratio rejection, complete ratio-domain acceptance, selectable Gate/Expander
direction, identity/current-value-independent generation, and fail-closed
MIDI/multichannel capabilities.

Chat tests cover independent control/restore reference prefixes, dB and
detector-frequency physical planning, direction-label rejection, atomic
batch execution, full-preimage rollback after side effects or batch failure,
exact normalized restore, generic-write ownership protection, and agent-loop
inspect/apply routing.

Catalog and policy tests require read-only inspect and undoable mutation apply
entries with the Gate/Expander argument vocabulary.

## Claim boundary

This is a typed control surface, not an acoustic gate detector or semantic
planner. It does not connect COM, PCA, C2, open semantic routing, or generic
product mappings. Expander support remains conditional on direction evidence;
hard-gate support is the V1 default.
