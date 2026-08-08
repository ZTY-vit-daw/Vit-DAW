# Spectral Dynamics Development Tasks V1

These tasks are independent and may be handed to agents with no prior thread context. None of them may use vendor/product identity, `Capture` as a classifier, Profile/VPS/SPAL mappings, or an apply path.

## Task 1: Evidence Census Adapter

Read disclosed parameter snapshots and emit a normalized, identity-free evidence ledger containing parameter name/raw name, host controllability, display unit/domain, index, and role candidates. Preserve sealed samples as opaque identifiers. Add deterministic tests for the DSM, Curves-node, dynamic-EQ, and multiband shapes. Do not alter recognition rules.

## Task 2: Topology Rule Conformance

Audit `DetectSpectralDynamicsModelWithBoundary` against the frozen rule: indexed frequency field plus indexed threshold field plus non-indexed operating/transfer/timing law. Prove that static EQ `Frequency + Gain + Q` is unresolved, dynamic-EQ threshold/dynamic-range cells are vetoed, and repeated band timing cells are multiband vetoes. Add only identity/current-value invariant tests.

## Task 3: Observation Contract

Define and validate `spectral-dynamics-observation/v1`: STFT gain field, frequency-bin alignment, time axis, scope identity, deterministic repeatability, change delta, and optional latency. Specify reject conditions for scalar broadband observations, unstable grids, unknown input/output alignment, and nondeterministic probes. Do not implement control writes.

## Task 4: Read-Only Inspect Workflow

Keep `plugin_grabber.inspect_spectral_dynamics` read-only. Verify one live parameter read, boundary code propagation, generation stability, and absence of controller refs. Add harness tests for kernel failure, malformed parameter surfaces, unresolved surfaces, and successful inspect summaries.

## Task 5: Regression and Sealed-Sample Harness

Run the evidence census and topology projection against all disclosed regression samples. Keep Curves Resolve and one new candidate per collection pass sealed: no parameter surface, screenshot, manual, preset, or identity-derived rule may enter the training corpus. Record false-positive/false-negative outcomes without changing the sealed inputs.

## Task 6: Controller Gate Review

Review whether the observation contract and cross-sample topology evidence are sufficient for any partial control. Until a stable spectral gain-field observation and stage decomposition are proven, the expected result is `unresolved_controller_boundary` and no apply implementation. If the gate later passes, define a new control schema separately from COM and broadband compressor controls.
