# Compressor Open Semantic Routing v1 Validation

Status: passed
Date: 2026-08-04

## Scope

This record validates target 1: connecting the qualified single-band broadband
compressor workflow to Vit's existing open semantic treatment route.

It does not add a new top-level intent router. It extends the existing open
policy so that the Agent can choose compression, inspect a compact processor
semantic identity card, acquire COM evidence, plan reachable controls, and
execute only after confirmation.

This target does not cover target-2 training experiments, limiters, multiband
compression, C2, Profile, VPS, SPAL, or product-specific mappings.

## Stable Runtime Contract

The open policy may disclose a loaded processor as:

```text
processor_type=compressor
qualification_status=broadband_compressor_qualified
next_planner=semantic_compressor
```

The semantic identity card is compact context for the LLM, not parameter-write
authority. A qualified existing compressor routes to semantic compressor
planning. If no suitable processor is loaded, the policy may recommend one,
but plugin-load approval and later parameter-write approval remain separate.

The runtime workflow names are `semantic_compressor_planning` and
`semantic_compressor_execution`. `COM` remains the observation projection model;
historical development labels such as COM-6 and COM-7 are not runtime workflow
identities.

## Product-Path Experiment

The acceptance command was:

```powershell
D:\Vit_DAW\scripts\run_vit_product_path_smoke.ps1 `
  -RepoRoot D:\Vit_DAW `
  -GodotProjectRoot D:\Godot\project\vit-daw-frontend `
  -GodotExe D:\Godot\Godot_v4.6.1-stable_win64.exe `
  -CompressorOpenSemanticRoutingAgentOnly
```

Godot launched the release Kernel, VSP Hub, and production Agent. Binary path,
hash, port ownership, Hub health, and the live official Agent session were
verified before the semantic cases ran.

Evidence of record:

`D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\product_path_20260804_215433\compressor_open_semantic_routing_report.json`

Production Agent:

```text
D:\Vit_DAW\agent\bin\VitAgent.exe
SHA256 856155591EC7C8338C596A87FC0144FCA36207B5CB34A906B2B552A13B08855F
```

## Results

### Existing Instance

- The smoke did not disclose `selected_plugin_id` to the Agent.
- The open policy found the live Pro-C 2 instance and qualified it as a
  single-band broadband compressor.
- The proposal performed zero parameter writes and required confirmation.
- Confirmation executed successfully and changed four intended parameters.
- The temporary track was deleted.

### Empty Rack and Post-Load Handoff

- The open policy selected the compressor category and recommended Pro-C 2.
- Plugin loading required its own confirmation.
- The newly loaded instance was read live and qualified as
  `threshold_driven` before parameter planning continued.
- Parameter execution required a second, distinct confirmation.
- Confirmation executed successfully and changed four intended parameters.
- The temporary track was deleted.

Both paths returned `status=ok`.

## Defects Found and Closed

The first product run exposed two generic routing defects:

1. Open-policy arbitration used a stale shadow plugin graph. Arbitration now
   reads live Kernel project state first and falls back to the cached state only
   when the live read fails.
2. Pro-C 2 exposes both a complete broadband compressor surface and an embedded
   sidechain Mid EQ surface. The classifier previously accepted the auxiliary
   EQ first. A complete broadband compressor identity card now takes precedence
   over an embedded EQ peripheral. This is topology precedence, not a product
   or vendor-name exception.

A regression fixture explicitly combines a qualified broadband compressor with
an adjustable sidechain EQ and verifies that the processor identity remains
`compressor`.

## Verification

The implementation and acceptance audit passed:

- `go test ./internal/chat -count=1`
- `go test ./internal/workflows/plugingrabber -count=1`
- `go test ./... -count=1`
- Python bytecode compilation for
  `scripts/compressor_open_semantic_routing_smoke.py`
- PowerShell AST parsing for `scripts/run_vit_product_path_smoke.ps1`
- `git diff --check` on the target files

`go vet ./...` retains two pre-existing Windows bridge warnings at
`internal/harness/harness.go:5188` and
`internal/vsphub/audio_feature_shm_windows.go:52`, both concerning
`unsafe.Pointer`. Neither location was introduced or changed by this target.

## Acceptance

Target 1 is accepted. Open natural-language treatment can now reach the
qualified compressor workflow with a loaded processor or through a separately
authorized recommendation/load handoff. The deterministic tool layer retains
parameter reachability, qualification, confirmation, atomic execution, and
readback responsibilities; semantic intent remains an Agent/LLM decision.

Target 2 has since been executed as a separate opaque compressor-only and mixed
EQ/compressor experiment. See `docs/SEMANTIC_PROCESSOR_OPEN_EXPERIMENT_V1.md`;
its observed product failures do not alter target 1's routing acceptance.
