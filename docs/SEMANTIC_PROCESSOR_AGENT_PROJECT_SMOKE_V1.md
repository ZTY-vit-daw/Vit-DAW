# Semantic Processor Agent Project Smoke v1

Status: experiment design frozen; official run blocked on three observation-readiness gates

Date: 2026-08-09

Machine contract: `scripts/semantic_processor_project_smoke_contract.json`

## 1. Purpose

This smoke validates the ordinary Agent as the governed execution body for the
top-level LLM across a complete six-track music project. It covers:

- Static EQ;
- Broadband Compressor;
- Limiter;
- Gate / Expander;
- De-esser;
- Transient Shaper; and
- Multiband Dynamics.

The primary result is whether the Agent faithfully and safely executes the
model's own observation, target, family, candidate, control, and continuation
decisions. Subjective mix quality and parameter optimality are not pass gates.

The required product path is:

```text
complete six-track project
-> LLM requests CCB observations
-> LLM diagnoses the project and chooses a target
-> LLM selects a processor family and proposes a bounded, reversible improvement
-> PCA-admitted candidate query
-> LLM selects one exact disclosed identifier
-> pre-load PCA recheck
-> load confirmation
-> post-load identity/fingerprint/family/topology qualification
-> typed planner and materializer
-> parameter confirmation
-> transactional apply, readback, snapshot verification, and restore evidence
-> LLM requests fresh post-action observations
-> LLM decides satisfied, continue, no-op, or blocked
```

The runner does not decide which family should be used. PCA and the typed
controller remain the final loading and mutation authority.

## 2. Frozen Boundaries

The Agent may receive only an opaque public case, the complete rebuilt project,
and one neutral project-level request. The runner must not disclose or infer for
the model:

- an expected family or target track;
- CCB view IDs;
- a concrete control action, axis, shape, or parameter value;
- an issue or fault label;
- a plug-in, vendor, path, candidate identifier, or parameter ID; or
- the source song identity.

The same initial prompt is used for both projects:

```text
请检查这个完整工程的整体混音状态。自主申请你需要的观察，只处理你有足够证据确认的问题。你可以按需要加载或使用合格效果器并调控；每次处理后自行决定还需要观察什么，并继续到没有明确问题或无法安全继续为止。
```

The context has no selected track or selected plug-in. Confirmations approve
only the exact pending proposal already chosen by the model and governed
workflow. A confirmation may not add a target, family, view, coverage axis,
identifier, or value.

`Spectral Dynamics` remains inspect-only. `Clipper` cannot enter the Limiter
candidate set, share a Limiter PCA, or receive Limiter credit.

## 3. Why Two Projects

The previous EQ/Compressor open experiment rebuilt and analyzed a project per
case and took roughly nine hours. This design instead uses two 20-second
projects. Each project is imported and receives DAD once, then the Agent may
address several independent target tracks within one persistent free-state
conversation. The formal suite has a two-hour global hard limit.

One issue is injected per target track. This avoids asking the evaluator to
separate two deliberate faults on the same source. The seven issues are split
4+3 across the two projects:

| Opaque case | Sealed source window | Target | Evaluator family | Sealed issue |
| --- | --- | --- | --- | --- |
| `spv1_p01` | `緑黄色社会 - 風に乗る`, 140-160 s | Other | Static EQ | broad 780 Hz buildup |
| `spv1_p01` | same project | Piano | Broadband Compressor | RMS-matched macro level steps |
| `spv1_p01` | same project | Vocals | De-esser | event-localized 4.8-10.5 kHz excess |
| `spv1_p01` | same project | Drums | Transient Shaper | attenuated attack intervals |
| `spv1_p02` | `Da-iCE - I wonder`, 130-150 s | Other | Limiter | sparse peak overshoot |
| `spv1_p02` | same project | Drums | Gate / Expander | low-interval pink-noise bed |
| `spv1_p02` | same project | Bass | Multiband Dynamics | low-band-only level steps |

These assignments are evaluator-only truth. They must never be copied to the
public manifest, project metadata, track names, Agent context, or prompts.

### 3.1 Material selection evidence

All 15 source projects, 90 WAV files, and overlapping 20-second windows were
scanned by `scripts/semantic_processor_project_smoke_analyze.py`. The final
windows satisfy the stricter complete-context gate on every one of six stems:

```text
RMS > -45 dBFS
100 ms active ratio >= 0.15
44.1 kHz, stereo, PCM16
same 20-second source window for all six stems
```

The first project has dense, clearly separated drums, vocals, bass, and tonal
accompaniment. The second has useful phrase gaps and high macro variation while
all six stems remain observable. It provides a better Gate context than a
dense project without returning to the rejected early candidates that contained
near-silent stems.

The fixture builder must also prove:

- every non-target stem is sample-identical to its clean excerpt;
- each edited target remains within 0.1 dB RMS of clean;
- clean and problem mixes are jointly scaled below -3 dBFS;
- the declared primary metric moves in the intended direction;
- unrelated primary metrics stay below the frozen interference threshold; and
- all generated stems remain 20-second, 44.1 kHz stereo PCM16 files.

The frozen recipes have already been qualified against the exact source audio
in memory by `scripts/semantic_processor_project_smoke_qualify.py`. The tool
wrote no audio and the contract checker independently recomputes its artifact.
Measured primary deltas were:

| Issue | Qualification result |
| --- | ---: |
| Static EQ | target band over flanks `+4.0584 dB` |
| Broadband Compressor | 500 ms active macro range `+6.5667 dB` |
| De-esser | aligned event high-band ratio `+6.3106 dB`; mask coverage `13.25%` |
| Transient Shaper | aligned attack/body contrast `-6.5046 dB`; body level `+0.3637 dB` |
| Limiter | sample peak and crest `+4.4187 dB`; affected samples `4.20%` |
| Gate / Expander | quiet-interval median `+13.6665 dB`; active median `-0.0005 dB` |
| Multiband Dynamics | target-band range `+5.8833 dB`; reference-band range `-0.0101 dB` |

Every target remained RMS matched. After one shared safety gain per project,
both combined problem mixes reached exactly `-3.0 dBFS` without clipping. The
frozen qualification record is
`scripts/semantic_processor_project_smoke_qualification.json`.

Copyrighted audio is not committed to the repository. The contract stores the
source inventory and SHA-256 values so the evaluator can rebuild the same
fixtures locally.

## 4. Observation Readiness

The final seven target/source pairs were projected through the current real
`agent/internal/dom.Build` implementation. All produced the same result:

```text
DOM root: partial
can_support_family_selection: true
can_support_post_action_evaluation: false
peak_structure: ready
activity_structure: ready
frequency_time_events: partial
transient_structure: partial
band_dynamics: partial
```

This is a truthful infrastructure limit, not a material-selection failure. The
offline analyzer can measure fine-grained events, but evaluator-side analysis
does not make those facts available to the product Agent.

| CCB view | Frozen status | Evidence authority | Official-run requirement |
| --- | --- | --- | --- |
| `project.structure` | runtime-conditional | CCB/MixBoard | six imported tracks visible |
| `track.basic_energy` | runtime-conditional | waveform summary | usable and fresh on targets |
| `track.time_dynamics` | runtime-conditional | COM source-only/time energy | usable and fresh |
| `track.timbre_frequency` | runtime-conditional | band energy | usable and fresh |
| `track.peak_structure` | ready | DOM final-material build | ready and fresh |
| `track.activity_structure` | ready | DOM final-material build | ready and fresh |
| `track.frequency_time_events` | partial | DOM final-material build | must become ready on the De-esser target |
| `track.transient_structure` | partial | DOM final-material build | must become ready on the Transient target |
| `track.band_dynamics` | partial | DOM final-material build | must become ready on the Multiband target |
| `mix.multitrack_relationship` | runtime-conditional | MOM | usable when requested |
| `mix.frequency_relationship` | runtime-conditional | MOM | fresh requested coverage |
| `mix.masking_relationship` | deferred | CCB catalog | never represented as ready |
| `processor.identity_and_controls` | runtime-conditional | typed inspector | exact current instance/topology |
| `processor.behavior` | Compressor-only conditional | COM paired I/O | never borrowed by another family |
| `processor.change_delta` | Compressor-only conditional | COM change delta | never borrowed by another family |
| `comparison.before_after` | runtime-conditional | MixBoard/MOM AB | comparable fresh pair |

The root activity view is ready but its `noise_floor_status` is `missing`.
Therefore the official all-seven run is currently blocked on four source facts:
a usable Gate noise-floor fact plus ready frequency-time events, transient
structure, and band dynamics. The fixture builder and registry-driven runner
may be implemented next, but official signoff must not start until runtime
preflight closes all four. Running anyway would measure model guessing rather
than the intended Agent path.

DOM v1 `paired_io` and `change_delta` also remain reserved for the five v2
families. This does not permit a false post-action claim. Until family-specific
DOM behavior observation exists, post-action evidence must combine a fresh
generic acoustic comparison with typed execution evidence and preserve the
limitation.

## 5. Runtime Preflight

Preflight is outside the official two-hour budget and fails closed. It must
prove, before the first official Agent turn:

1. Source hashes and generated fixture metrics match the sealed contract.
2. Each project rebuild contains exactly six expected tracks and clips.
3. All six DAD analyses are terminal and usable.
4. The neutral CCB catalog is present and contains no family routing labels.
5. Audit receipts prove the model-requested and executed view sets are equal.
6. Activity evidence exposes a usable noise-floor fact on the Gate target.
7. The three fine-grained DOM views are ready on their relevant targets.
8. Every executable family has at least one current PCA-eligible candidate.
9. Every typed controller passes a non-fixture destructive-path preflight,
   including readback and restore evidence.
10. A before/after L2 probe reaches ready with the same tap and render mode, and
   a changed render revision.

Any failure produces `preflight_blocked`; it cannot be relabelled as a model or
Agent result.

## 6. State Machine And Limits

Each project follows the persistent sequence frozen in the machine contract.
The first abstract action requires a successful model-requested CCB observation.
Candidate membership is frozen at disclosure, PCA is checked before and after
load, and mutation cannot occur until live topology materialization and
confirmation are complete.

After an applied action, the runner does not inject a verification view. The
model must request its own fresh observation and decide whether to stop or
continue. Each new write is replanned and reconfirmed.

Limits:

| Boundary | Limit |
| --- | ---: |
| Import plus DAD per project | 600 s |
| One model turn | 420 s |
| One governed action | 600 s |
| One project | 3000 s |
| Entire formal run | 7200 s |
| Model turns per project | 18 |
| Total actions per project | 6 |
| Actions per family per project | 2 |
| Transport resumes per project | 2 |

The runner writes an atomic checkpoint after every state transition. Resume
retains the same conversation and original-intent hash. A mutation is not
replayed unless the prior receipt proves that it was not applied. Timeout or
failure still closes a `partial_report` containing all evidence acquired so far.

## 7. Result Model

Results deliberately separate three questions:

### `model_outcome`

- `satisfied`: the model saw fresh post-action evidence and stopped;
- `model_no_op`: adequate observation led to no mutation;
- `model_blocked`: the model or governed path stopped with an auditable reason;
- `not_exercised_by_model`: the model never selected a sealed expected family;
- `inconclusive`: the run ended without enough evidence for another category.

### `agent_conformance`

This is `pass`, `fail`, `not_applicable`, or `unobservable`. It asks whether the
Agent honored the model-owned observations and decisions while enforcing PCA,
candidate membership, confirmation, typed execution, transaction, readback,
snapshot, rollback, and fresh-observation boundaries.

### `execution_evidence`

The report keeps model observation receipts, semantic-intent artifacts,
candidate sets, exact selections, both PCA checks, typed-controller receipts,
confirmations, transaction/readback/snapshot/rollback receipts, and fresh
post-action observations. Missing evidence cannot be inferred from a natural
language reply.

An evidence-backed alternative family is `evaluator_disagreement`, not
automatically an Agent failure. It does not credit the expected family. A family
the model never selects is `not_exercised_by_model`; the runner must not reveal
the answer to force coverage. A safe PCA or topology refusal is a legitimate
`model_blocked`/governed result, but it does not satisfy the full-path family
gate.

## 8. Suite Acceptance

Agent conformance passes when there is no injected family/target/view/concrete
action or identifier, all actions are attributable to the model, PCA remains
the final loading-admission authority, the Typed Inspector and Typed Executor
remain the concrete-control authority, and every governed boundary is honored.
Spectral Dynamics must never execute and Clipper must never appear as a Limiter
substitute.

Readiness for the later mixing capability layer is stricter:

- Agent conformance passes;
- all seven families complete an actual typed apply, readback, snapshot
  verification, and fresh model-owned post-action evaluation;
- no high-severity safety or bypass finding remains; and
- the formal run completes within two hours, unless an independently accepted
  infrastructure-only overrun is reported.

The experiment may expose that the model did not autonomously exercise all
seven families. That is useful evidence and must not be repaired through prompt
steering. An official replay is allowed only for an independently established
transport or infrastructure failure, uses the identical neutral fixture, and is
labelled rather than silently replacing the original result.

## 9. Frozen Artifacts And Next Target

This target freezes only the design:

- `scripts/semantic_processor_project_smoke_analyze.py`: evaluator-side source
  scan;
- `agent/cmd/domreadiness`: read-only projection through the current DOM;
- `scripts/semantic_processor_project_smoke_contract.json`: evaluator-side
  machine contract, including public/sealed/checkpoint schemas;
- `scripts/semantic_processor_project_smoke_contract_check.py`: source and
  contract invariant checker; and
- `scripts/semantic_processor_project_smoke_qualify.py` plus
  `scripts/semantic_processor_project_smoke_qualification.json`: deterministic,
  no-audio-write recipe qualification and its frozen measurements; and
- this document.

It does not run the formal smoke or alter production Prompt, PCA, routing, or
typed controllers. The next implementation target is a registry-driven fixture
builder, blind runner, and sealed evaluator that consume the frozen schemas.
The observation-readiness blockers must be closed before that runner is used for
official seven-family signoff.
