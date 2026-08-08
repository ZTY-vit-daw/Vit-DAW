# Processor Control Attestation v2

PCA v2 is a separate admission vocabulary for the identity-free typed
controllers introduced after PCA v1. PCA v1 remains unchanged and continues
to cover only `static_eq` and `broadband_compressor`.

## Families

The v2 family names are:

- `limiter`
- `gate_expander`
- `de_esser`
- `transient_shaper`
- `multiband_dynamics`

`Spectral Dynamics` is intentionally not a v2 family. Its current surface is
inspect-only and does not produce controller references.

## Coverage Contract

Coverage is action-only. Every v2 item uses `action=adjust`, has no `shape`,
and contains one semantic `axis`. Axes are evidence-backed capabilities, not
parameter names, mappings, profiles, or execution plans.

| Family | v2 axes |
| --- | --- |
| `limiter` | `detector_latency`, `input_drive`, `output_ceiling`, `output_normalization`, `peak_mode`, `protection_intensity`, `recovery_motion` |
| `gate_expander` | `activation_threshold`, `attenuation_floor`, `detector_focus`, `direction_mode`, `output_normalization`, `parallel_balance`, `state_timing` |
| `de_esser` | `detector_focus`, `output_normalization`, `parallel_balance`, `recovery_motion`, `sibilance_reduction`, `split_scope`, `threshold_sensitivity` |
| `transient_shaper` | `detector_focus`, `envelope_emphasis`, `envelope_timing`, `output_normalization`, `parallel_balance`, `shape_mode` |
| `multiband_dynamics` | `band_dynamics`, `band_timing`, `crossover_layout`, `detector_focus`, `output_normalization`, `parallel_balance` |

The mapping from observed typed roles to these axes is deterministic and
identity-free. A badge never records `param_id`, normalized values,
`topology_generation`, Profile, VPS, SPAL, or product-specific mappings.

## Evidence Gate

A v2 badge requires a strong receipt produced by the disposable-track
certification runner:

1. exact installed subject and current binary fingerprint;
2. two stable live inspections with the same topology generation;
3. at least one reversible typed control selected from observed physical or
   reachable enum evidence;
4. one atomic typed apply with complete actual readback;
5. typed restore, preferably by the returned `restore_ref`;
6. complete parameter snapshot equality after restore;
7. temporary track deletion, including the last-track cleanup guard;
8. zero natural-language or LLM calls.

Failed, unresolved, adjacent, or read-only surfaces are not admitted. A badge
proves only the listed family and action axes for the exact current binary;
live topology inspection remains the execution authority after load.

## Storage and CLI

v2 badges use the independent store
`~/.vit/processor_control_attestations.v2.json` and the `pca2_` attestation
identifier prefix. The v1 store is not migrated or rewritten by v2 commands.

The deterministic CLI surface is:

- `pcactl certify-processor`
- `pcactl import-v2`
- `pcactl list-v2`
- `pcactl query-v2`

No recommendation or semantic planner may treat a v2 candidate as eligible
unless the current binary fingerprint and every required action axis query as
promoted and covered.
