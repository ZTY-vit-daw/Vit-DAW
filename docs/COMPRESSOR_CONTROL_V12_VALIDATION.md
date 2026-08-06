# Generic Broadband Compressor Control v1.2 Validation

## Outcome

Generic broadband compressor control v1.2 closes the three disclosed Plugin Alliance round-2
failures without weakening the single-band boundary or adding product/vendor identity rules.

The tested production Agent SHA-256 is:

`47877C3C79F4BA9E0F8663D4D4CBB9799F78F90BF797CB6B52365311D07A2585`

No Profile, VPS, or SPAL mapping was added. The four sealed reserve products were not executed and
their parameter surfaces were not read.

## Generic Fixes

- A fixed-transfer leveler remains provable when `reduction_amount` coexists with an auxiliary
  `input_drive`. Amount evidence is evaluated before the input-driven special branch.
- A pure clipper surface with input, ceiling, and knee/type evidence, but without threshold, ratio,
  reduction amount, or compressor timing evidence, returns `unsupported_clipper`.
- A separately provable broadband compressor stage may still expose an auxiliary ceiling or knee;
  that does not trigger the pure-clipper boundary.
- A continuous control may reserve its normalized endpoints for symbolic states while exposing a
  numeric interior. Three or more monotonic interior samples form the invertible numeric curve;
  interpolation is restricted to those observed anchors and does not extrapolate into the symbolic
  endpoints.

## Verification

| Gate | Result |
| --- | --- |
| Focused topology and apply tests | passed |
| Full Go suite: `go test ./... -count=1` | passed |
| Disclosed round-2 fix regression | 3/3 passed |
| Temporary-track cleanup | clean |
| Sealed reserve parameter reads | 0 |

The repeatable disclosed regression runner is:

```powershell
python D:\Vit_DAW\scripts\compressor_plugin_alliance_fix_v12.py `
  --output-dir D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\product_path_20260804_112622\compressor_control\v12_fix_smoke
```

Evidence is stored under:

`D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\product_path_20260804_112622\compressor_control\v12_fix_smoke`

## Real Product Results

| Product | Expected boundary | Result | Mutation proof |
| --- | --- | --- | --- |
| NEOLD U2A | supported broadband compressor | `amount_driven`, confidence 0.90 | `input_drive` and `reduction_amount` applied exactly; `restore_ref` exact; all 14 parameters restored |
| Lindell MU-66 | multiband boundary | `unsupported_multiband_compressor` | zero writes; all 37 parameters unchanged |
| bx_clipper | clipper boundary | `unsupported_clipper` | zero writes; all 30 parameters unchanged |

All three temporary tracks were deleted. The final independent `track.list` cleanup audit found no
`PA fix v1.2` tracks.

## Godot Runtime Evidence

The product-path runner rebuilt the production Agent and Godot registered and launched Kernel,
VSP Hub, and Agent. The live process chain was:

```text
Godot console (40568)
  Godot runtime (23132)
    VitApp Kernel (44276): 5555, 5556
    VSP Hub (48580): 8787
    VitAgent (39420): 7878
```

Agent and Hub health both returned `ok`; Hub reported a live Agent session and realtime streams.
The lifecycle script itself exited with `Missing Godot lifecycle evidence` because its log parser
did not recognize the emitted registration lines. This is a harness-evidence false negative, not a
runtime or compressor regression; the process ownership, executable paths, ports, health, Agent
hash, and live product regression were verified independently.

The historical round-2 blind report remains unchanged. This v1.2 run uses only the three disclosed
failures as regression data and makes no new blind-test claim.
