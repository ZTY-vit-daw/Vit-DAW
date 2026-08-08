# Multiband Dynamics Control v1

## Scope

This primitive controls a proven filterbank dynamics stage. It does not own
static EQ, dynamic EQ, de-essing, spectral dynamics, transient shaping,
clipping, or a maximizer/limiter stage unless that stage is separately
provable and exposed through its own typed primitive.

The recognizer is identity-free. Vendor, product, category, and shell names
are inventory metadata only. The topology generation is based on parameter
roles, band membership, physical display evidence, and stage structure.

## Positive Rule

A surface is accepted only when all of the following are true:

1. At least one shared parameter family proves a filterbank partition through
   `crossover`, `cross over`, `xover`, or an equivalent split-frequency role
   such as `Low-Mid Freq` and `Mid-High Freq`.
2. The shared boundary values are physically readable and strictly ordered in
   their exposed slot order. An unordered or unverifiable set is
   `unresolved_crossover_order`.
3. At least two repeated, independently addressable band cells exist, and at
   least one host-controllable shared modifier exists outside those cells.
4. Each counted cell has an operating point (`threshold`, `thresh`, reduction
   amount, or range), at least one transfer or gain-action control, and timing
   evidence (`attack`, `release`, `recovery`, `hold`, or `lookahead`). Sidechain
   EQ bands do not count.

Band-local edge pairs such as `Band N Low Crossover` plus `Band N High
Crossover` are not interchangeable with a shared ordered vector in v1. They
return `unresolved_band_local_crossovers` until a separate edge-topology
contract exists. This is the current FabFilter Pro-MB boundary.

The retained structural exemplar is Waves C6 Stereo: three crossovers and six
repeated cells, each with threshold, gain, range, attack, and release. C4 has
four cells and uses the `Thresh` spelling; LinMB has four ordered boundaries
and five cells. Lindell MBC exposes two shared split frequencies and
Low/Mid/High cells; Lindell 354E exposes two `XOver` boundaries and the same
named-cell shape. The retained C6 evidence is
[c6_multiband_negative.parameters.json](/D:/Vit_DAW/temp/c2-matrix-negatives-final2/c6_multiband_negative.parameters.json).

## Control Boundary

v1 controls only bindings returned by `plugin_grabber.inspect_multiband`:

- crossover frequency (`value_hz`);
- cell threshold/reduction/range/gain (`value_db`);
- transfer ratio (`ratio`);
- attack/release/hold/lookahead (`value_ms`);
- measured display or exact reachable enum values where the display probe proves them.

Every request is atomic. All crossover targets are planned first, then checked
as one strictly ascending set. Invalid ordering returns
`invalid_crossover_order` and `parameters_changed:false` before a preimage or
write transaction is started. Execution uses the existing full-preimage,
bounded-correction, readback, and restore engine.

Unsupported or unresolved controls are rejected. No v1 mapping is inferred
from a product name, and no generic `plugin.set_parameter` write is allowed to
own a parameter claimed by the typed multiband topology.

## Adjacent Boundaries

| Surface | Result |
| --- | --- |
| Static or dynamic EQ without a crossover filterbank | `not_multiband_dynamics` or `unsupported_dynamic_eq` |
| Sidechain EQ family | ignored as processed bands; no multiband claim |
| De-esser | `unsupported_de_esser` |
| Multiband maximizer/limiter stage | `unsupported_multiband_maximizer` |
| Spectral dynamics | `unsupported_spectral_dynamics`; no multiband control ownership |
| Clipper | `unsupported_clipper`; remains an independent future primitive |
| Broadband compressor | remains owned by the existing compressor recognizer |

## Local Candidate Census

The isolated read-only runner is [multiband_pluginprobe_census.py](/D:/Vit_DAW/scripts/multiband_pluginprobe_census.py), configured by [multiband_control_matrix.json](/D:/Vit_DAW/scripts/multiband_control_matrix.json). It uses the loopback PluginProbe worker, which has no Vit project authority and no parameter-write operation. The legacy disposable-track runner remains [multiband_matrix_smoke.py](/D:/Vit_DAW/scripts/multiband_matrix_smoke.py) for the Godot-owned lifecycle. Neither runner writes a plugin parameter.

Current inventory metadata:

- FabFilter inventory contains one multiband candidate, `Pro-MB`; the captured
  surface has six repeated cells but twelve band-local edge parameters with
  duplicate default boundaries, so it is unresolved for this shared-vector v1.
  Pro-Q 3 and Pro-DS are captured adjacent negatives.
- Waves inventory contains 30 entries in the installed `Fx|Dynamics -
  Multiband` category (mono/stereo variants counted separately). Eight
  inspected entries form the confirmed positive intersection: C4 Mono/Stereo,
  C6 Mono/Stereo, C6-SideChain Mono/Stereo, and LinMB Mono/Stereo. Four IDX/IDX
  LIVE entries remain sealed and were not opened. The remaining non-sealed
  inventory is classified from structure as follows:

  | inventory family | observed structural result |
  | --- | --- |
  | L3 MultiMaximizer, L3 UltraMaximizer, L316, L3-LL Multi, L3-LL Ultra | rejected as a multiband limiter/maximizer surface (`Ceiling`/no dynamics-cell intersection) |
  | TransX Multi | rejected: range cells have no per-cell timing/transfer intersection |
  | MannyM Tone Shaper | unresolved/non-dynamics: no ordered crossover and no complete dynamics cells |
  | MannyM TripleD | rejected as composite de-esser stages; not a filterbank dynamics surface |
  | Vitamin | unresolved/non-dynamics: ordered crossovers but no repeated dynamics cells |

  These are structural outcomes, not mappings from the product names. The
  inspected Waves result is therefore 8 confirmed positive instances, 18
  rejected/unresolved non-sealed instances, and 4 sealed instances; no claim is
  made about an unobserved surface beyond its inventory metadata.
- Regression boundaries: Lindell MBC and Lindell 354E are previously disclosed
  Plugin Alliance samples, therefore not blind.

The authoritative PluginProbe evidence is
[temp/multiband-pluginprobe-census/summary.json](/D:/Vit_DAW/temp/multiband-pluginprobe-census/summary.json)
and its per-case snapshots. Its structural summary reports `ordered=true`,
complete-cell counts, and shared-modifier evidence without assigning a type
from a product name.
