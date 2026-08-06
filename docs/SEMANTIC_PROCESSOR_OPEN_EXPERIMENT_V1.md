# Semantic Processor Open Experiment v1

Status: completed with observed product failures
Date: 2026-08-05

## Scope

This is target 2 of the ordinary-Agent compressor work. It builds and runs an
opaque compressor-only and EQ/compressor mixed experiment set through the real
Godot -> Kernel -> VSP Hub -> production Agent path.

The experiment does not cover limiter, multiband compression, C2, Profile,
VPS, SPAL, or product-specific mappings. It does not force the expected
processor family. The runner follows the Agent's recommended method, plugin,
load confirmation, parameter confirmation, and supported non-plugin actions.

## Frozen Fixture

Fixture set:

```text
semantic_processor_open_v1_5c151e8bbd47
```

Contract:

- eight opaque cases, six tracks per case, 20 seconds, 44.1 kHz PCM16;
- public case directories contain only problem stems and opaque case metadata;
- issue recipes, expected processors, clean references, fixed references, and
  evaluator truth remain below `sealed`;
- the Agent receives no initial plugin identity;
- each case rebuilds a clean `.vit` project and waits for all six DAD tracks;
- focal-stem and problem/reference RMS are level matched;
- `so_01` is a clean control and accepts only `none`.

Cases:

| Case | Surface | Sealed condition | Expected route |
| --- | --- | --- | --- |
| `so_01` | compressor-only control | clean vocal | none |
| `so_02` | compressor-only | vocal macro dynamics | compressor |
| `so_03` | compressor-only | drum transient overshoot | compressor |
| `so_04` | compressor-only | bass note inconsistency | compressor |
| `so_05` | mixed set | vocal low-mid buildup only | EQ |
| `so_06` | mixed set | vocal macro dynamics only | compressor |
| `so_07` | mixed set | vocal low-mid buildup plus macro dynamics | EQ and compressor |
| `so_08` | mixed set | accompaniment masking plus vocal dynamics | EQ and compressor |

`so_07` and `so_08` run a second neutral turn after the first treatment. The
second prompt asks the Agent to reobserve and continue only if a clear problem
remains; it does not name a processor.

## Product-Path Run

Command:

```powershell
D:\Vit_DAW\scripts\run_vit_product_path_smoke.ps1 `
  -SkipBuild `
  -SemanticProcessorOpenExperimentAgentOnly `
  -TimeoutSeconds 180
```

Evidence of record:

```text
D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\product_path_20260805_085808
```

The launcher verified the exact Godot-owned Kernel, Hub, and Agent binaries,
their port ownership, Hub health, and the official Agent session before the
experiment. All eight projects completed six-track DAD analysis. The blind
runner did not open sealed truth; the separate evaluator opened it only after
the run report was complete.

## Official Results

Official result: 2/8 passed.

| Case | Actual route | Final behavior | Result |
| --- | --- | --- | --- |
| `so_01` | compressor | Pro-C 2 executed with COM evidence | control false positive |
| `so_02` | compressor | Pro-C 2 executed and verified | pass |
| `so_03` | compressor | API-2500 loaded; materialization rejected | observed failure |
| `so_04` | compressor | CLA-2A loaded; materialization rejected | observed failure |
| `so_05` | unobservable | plugin recommendation LLM returned HTTP 502 | observed dependency failure |
| `so_06` | compressor | Pro-C 2 executed and verified | pass |
| `so_07` | EQ | both rounds resolved to observation-only | observed failure |
| `so_08` | compressor | compressor executed; second round stopped without EQ | incomplete eventual coverage |

Failure taxonomy from the sealed evaluator:

```text
route_mismatch                                  1
control_false_positive                         1
confirmation_boundary_missing                  3
post_action_acoustic_evidence_missing          3
processor_execution_failed                     2
materialization_rejected                       2
route_unobservable_due_model_service_error     1
incomplete_eventual_processor_coverage         3
execution_resolution_observation_only          1
```

The two materialization rejections are bounded safety outcomes, not unsafe
writes:

- `so_03`: API-2500 exposes Attack as a discrete enum. The LLM proposed
  `value_ms=3`; the materializer rejected `unit_role_mismatch` instead of
  silently converting it to the `3 ms` enum label.
- `so_04`: CLA-2A produced only `partial` paired-I/O COM evidence. The executor
  requires `paired_io/ready`, so it stopped before parameter confirmation or
  mutation.

Every successful compressor mutation kept plugin loading and parameter writing
as separate confirmations and produced parameter readback plus COM
`change_delta` evidence. Rejected plans stopped after load confirmation and did
not request or perform a parameter write.

## Transport-Error Supplement

Because official `so_05` ended only through an LLM HTTP 502, it was rerun once
as a separately labelled transport-error supplement. It was not substituted
into the official 8-case pass rate.

Supplement evidence:

```text
D:\Vit_DAW\VitApp\Workspace\Artifacts\smoke\product_path_20260805_092251
```

The supplementary run selected EQ, independently confirmed plugin loading and
parameter execution, finished `verified`, and passed sealed evaluation 1/1.
This proves that the EQ route is reachable for the frozen case while preserving
the official run's service-error observation.

## Infrastructure Findings Closed

The experiment closed three test-path defects without changing expected
processor choices:

1. The open-method lexical gate now admits the eight natural hearing-language
   prompts, including `站到前面`, `发闷`, `收稳`, and `更均匀`. This gate only
   enters LLM method arbitration; it does not select EQ or compression.
2. The blind runner now follows structured and explicit natural-language
   confirmations, records utility routes, preserves partial results on failure,
   and reports HTTP endpoint/body diagnostics.
3. Project construction now waits for all six expected DAD tracks. This removes
   a cross-case race where save could occur after only the first asynchronously
   imported track became ready.

## Product Findings Left Open

The experiment intentionally leaves these as evidence for the next development
target:

- clean-control over-treatment despite the model explicitly stating that the
  evidence was insufficient;
- semantic conversion from physical timing values to reachable discrete enum
  labels when an exact label exists;
- policy for useful but `partial` COM evidence on reduced-control optical
  compressors;
- mixed-problem continuation after a successful first processor;
- observation-only EQ treatment resolution when plugin/control identity is
  still unresolved;
- external LLM service reliability and retry policy.

The current compressor control layer is therefore strongest on Pro-C-style
threshold-driven vocal macro-dynamics. Processor-family routing generalized to
drums and bass, but executable semantic materialization is not yet equally
general across discrete-control and reduced-control compressor topologies.
