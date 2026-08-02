# Project-aware Capability Runtime v1 — migration gates

Updated: 2026-07-13

## Authority invariants

- A `PlanningSession` has one immutable `engine_owner`. Historical records may still decode `legacy`, but B2/B3 product routing can now create only `capability_runtime_v1` Sessions.
- A v1 Chat session never writes legacy pending state.
- Readiness, Candidate, Proposal, frozen ActionSet, Authorization, Execution and Verification are separate records.
- Shadow planning cannot mutate the project or create authorization.
- Execution requires an exact persisted `FrozenPlan`: Proposal + ActionSet + ProjectCut + Context Bundle identity + previous observation identity.
- A hash-only Proposal is not executable.
- Mutation requires a negotiated `command.base_revision_cas` Kernel feature and a `strong` ProjectCut.
- Mutation requires a durable orchestration Store and a Project History baseline reference.
- One stable request ID is used per action. Kernel receipt caching provides effectively-once replay for the same Session/request ID.
- A cancelled execution cannot dispatch another action; any in-flight receipt is persisted first.
- Crash recovery reconciles missing receipts from real project state and never blindly retries mutation.
- Process-kill tests cover prepared-before-apply, apply-before-receipt, partial batch, receipts-before-verify and verify-before-finalize persisted states.
- Structural verification, fresh acoustic/MOM verification and user aesthetic acceptance are distinct. User acceptance defaults to `unknown`.
- `ExecutionRecord.verification_result` durably preserves status, structural result, acoustic result, user acceptance, summary and evidence refs; the legacy scalar `verification` remains only as a backward-compatible compact status.
- Typed in-process observation payloads (notably `*mom.Projection`) are normalized at the protocol boundary before CCB admission and verification. HTTP JSON serialization is not relied on to make typed evidence visible.

## Context admission

Every v1 model-facing call is bounded by one `ContextEnvelope` containing:

1. system/runtime contract;
2. capability registry top-N;
3. tool schema top-N;
4. compact typed PlanningSession;
5. bounded ContextBundle disclosure and artifact/evidence references;
6. a bounded recent-turn window;
7. an explicit output-token reserve.

The omission manifest uses:

- `not_requested`
- `omitted_budget`
- `available_by_ref`
- `unavailable`
- `stale`
- `forbidden`

Full derived models do not enter default disclosure. Tests cover a 10,000-turn history and prove that admitted conversation content and omission-manifest cardinality remain bounded.

## B2 v1 route

The `static_mix.static_balance.v0` route now uses:

```text
Chat
→ v1 PlanningSession
→ CCB-owned project.state + project.audio_analysis_status + mix.observe (read-only)
→ deterministic TOM reconstruction from the authoritative project snapshot
→ full B2 model/readiness/solver
→ bounded ContextBundle + ContextEnvelope
→ frozen Proposal/ActionSet/ProjectCut
→ version-bound Authorization
→ Project History baseline
→ CAS/idempotent VSP Execution Coordinator
→ structural fader readback
→ fresh mix.observe/MOM verification
```

Client-supplied `project_state`, `mix_observation`, `MOM` or `audio_analysis_status` is not treated as authoritative execution evidence.

The B2/B3 Context Manifest requires track-role relationships. CCB therefore rebuilds the TOM projection from the same authoritative `project.state` snapshot whenever a persisted TOM projection is absent. This is read-only context assembly: it neither applies folder organization nor creates pending execution authority. The TOM identity is included in the ProjectCut dependency set so the frozen plan records the role evidence used by the solver.

## B3 v1 route

The permanent `static_mix.pan_layout.v0` route uses the same shared control plane and lifecycle. Its capability-specific pieces are limited to:

- Pan Layout context pack/model/solver adapter;
- `track_pan_set` Action compiler;
- CAS/idempotent `set_pan` execution port;
- pan postcondition verifier;
- the same fresh full-project mix.observe/MOM verifier.

Persisted legacy B3 pending records are decode-only migration input. On workspace restore they become terminal `rejected` audit candidates and never regain confirmation or mutation authority.

## Final owner cutover and one-way migration

B2/B3 are permanently routed to v1 at the Chat abstraction seam. The historical rollout environment variables remain readable for deployment telemetry compatibility, but no value—including explicit `off`—can recreate a legacy B2/B3 owner in this binary.

- Existing v1 Sessions retain their immutable owner and invocation-scoped ID.
- Terminal Sessions remain audit history; a later invocation gets the next sequence.
- Old `pending_static_balance_plans`, `pending_pan_layout_plans`, legacy execution-memory fields and legacy confirmation interactions are accepted only by a one-way migration reader.
- Restore clears executable legacy fields and drops legacy confirmation entrances. Still-active legacy candidates become `rejected` with `transition_reason=legacy B2/B3 authority retired after capability runtime v1 cutover`; already-terminal `verified`、`committed`、`failed`、`verification_failed`、`blocked` or `rejected` audit truth is preserved exactly.
- Duplicate map/ExecutionMemory records are reconciled by conversation + capability type + plan/context identity, so an already verified legacy execution cannot gain a second synthetic rejected candidate merely because its reconstructed ID lacks the original goal ID.
- Subsequent persistence never writes those legacy authority fields again.
- Rollback after this phase means deploying a prior binary, not toggling an in-process owner switch.

Session IDs remain invocation-scoped (`cap_v1_b2_<conversation>_<n>` / `cap_v1_b3_<conversation>_<n>`).

The read-only Chat command `/capability-runtime/status` reports:

- active v1 Sessions;
- terminal v1 Session count;
- Store inventory failures;
- the backward-compatible `safe_to_disable_legacy_code` certification field. The code is now removed; the field remains for operators and older automation.

The certified executable still supplies the historical environment defaults for telemetry compatibility:

```text
VIT_CAPABILITY_RUNTIME_V1_ROLLOUT=all
VIT_CAPABILITY_RUNTIME_V1_LIVE_VERIFIED=true
VIT_CAPABILITY_RUNTIME_V1_DISABLE_LEGACY_CREATION=true
```

The AgentLoop B2/B3 preflights, decision schemas, candidate builders and Chat confirmation executors have been physically removed. A final result-boundary guard still rejects an impossible stale legacy result instead of persisting it.

## Durable Store

- The default path is `%AppData%\Vit\Agent\orchestration_v1.json`.
- `VIT_ORCHESTRATION_STORE_PATH=memory` is an explicit non-executable development opt-out.
- FileStore holds an OS-backed lock across read/CAS/write and persists through same-directory temp-file replacement, preventing multi-process lost updates and torn files.
- A real subprocess test runs two writers against one Store and proves that all 80 Sessions survive.

## Live evidence — 2026-07-13

Release VitApp was started with `VIT_PROJECT_XML` pointing at an isolated fixture. The dirty repository `VitApp/Workspace/default_project.xml` remained SHA-256 `965D4FFF083DA86741857257BAC1AC90A90C31FA6DA85FA610C24AC08D2BDB8F` before and after every smoke.

1. VSP protocol smoke: `D:\Vit_DAW\artifacts\orchestration_v1_live_smoke\20260713_132453`
   - negotiated `command.base_revision_cas` and `command.idempotency`;
   - rejected an incorrect base revision with `stale_project_cut`;
   - replayed the same Session/request ID with the same Receipt payload.
2. Combined policy-route B3 then B2 smoke: `D:\Vit_DAW\artifacts\orchestration_v1_live_smoke\v1_policy_both_20260713_140002_summary.json` (SHA-256 `8B1CEBEB2DDD552368E832CC75F9B3AD3197A90C5756E31C85A465130EC392DB`)
   - no explicit canary flag and no externally supplied rollout/live-verification environment variables;
   - both Sessions were assigned to v1 by the packaged-agent defaults;
   - each capability persisted two Action Receipts and a Project History baseline reference;
   - each passed structural readback and a new full-project MOM `ready` observation;
   - each persisted acoustic `pass` while keeping user acceptance `unknown`;
   - the final report contained two terminal v1 Sessions, no legacy pending/conflict, and `safe_to_disable_legacy_code=true` after the creation-close default was enabled.
3. Post-retirement combined smoke: `D:\Vit_DAW\artifacts\orchestration_v1_live_smoke\v1_post_retirement_20260713_143333\evidence\summary.json` (SHA-256 `9693300BB88467293EBED7F55EFD99E4772BB6595858F57A4A22B2F2BC34ACC1`)
   - launched the rebuilt agent with `VIT_CAPABILITY_RUNTIME_V1_ROLLOUT=off` and `VIT_CAPABILITY_RUNTIME_V1_DISABLE_LEGACY_CREATION=false`, without `--explicit-canary`;
   - B3 and B2 still created only v1 Sessions and each persisted two Receipts plus a History baseline;
   - both structural and acoustic verification were `pass`, fresh MOM was `ready`, and user acceptance remained `unknown`;
   - the authority report contained two terminal v1 Sessions and `safe_to_disable_legacy_code=true`;
   - the dirty default project hash remained unchanged.

Targeted v1 packages, Chat routes, recovery tests and cross-process Store tests pass. Repository-wide `go test ./...` still fails only in the pre-migration `internal/agentloop` suite; those failures must not be represented as v1 failures or as repository-wide green status.

### B2 track-role readiness correction — 2026-07-13

A manual run on the current 71-track project reached the v1 Session owner but stopped at `track_role_relationships`. The failure exposed a Context Manifest implementation gap: TOM was declared as required evidence, while the v1 acquisition path supplied project state, MOM and DAD only. The readiness threshold was intentionally kept unchanged. CCB now deterministically reconstructs TOM from authoritative project state for both B2 and B3. `TestProjectTOMProjectionRestoresB2RoleCoverageWithoutCreatingAuthority` verifies at least 95% role coverage and non-empty deterministic B2 candidates without creating authorization or mutation state. A new packaged-agent manual retest remains the product acceptance gate.

### Capability-specific verification and needs-review outcome — 2026-07-13

The same 71-track run then applied all 12 B2 fader actions with effectively-once Receipts and passed structural readback, while the shared acoustic verifier returned `inconclusive` because all 61 non-container tracks lacked optional band/stereo evidence. This exposed a contract mismatch: B2 admission deliberately uses L1/effective level relationships and does not require L3 spectral or stereo evidence, but the old shared verifier required the aggregate MOM relation to be globally `ready`.

- B2 now passes post-execution acoustic verification only when the observation is fresh, `level_distribution` is `ready`, compared-track coverage is at least 95%, every action target remains in the fresh MOM inventory, and the post-MOM effective static levels realize the exact foreground/anchor/support direction declared by the frozen candidate. Missing band/stereo evidence remains an optional limitation; missing effective relationship evidence is `inconclusive`, while movement opposite to the candidate is `fail`.
- B3 independently requires a fresh `stereo_distribution`; it cannot borrow B2's level-only policy.
- A stale, suspect, invalid or explicit structural/acoustic failure still fails the Session.
- Applied Receipts followed by unavailable or incomplete required verification evidence now persist as terminal Session `needs_review` with Execution `verification_inconclusive`. This outcome is not `failed`, cannot automatically retry mutation, and is surfaced to Chat as a review state.

## Remaining non-v1 cleanup

- B2/B3 legacy executable authority cleanup is complete; only decode-only migration structs/converters remain.
- Audit or retire the wider pre-existing `internal/agentloop` failures before claiming repository-wide green status. The current repository-wide run fails only in that package; every other package passes.
- Migrate future capabilities through the same PlanningSession/Context/Execution/Verification protocols rather than adding new side-channel pending authorities.
