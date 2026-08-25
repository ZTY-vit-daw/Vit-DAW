# Free-State Phase D D1-S1 Paused Handoff

Status: development paused by the user; uncommitted working-tree handoff, not a D1 closeout. Date: 2026-08-23.

## 1. Purpose and epistemic boundary

This document records the state of Phase D when the development conversation was intentionally stopped because the available token budget was exhausted. It exists so a later conversation can resume from verified evidence instead of reconstructing the work from memory.

The distinction below is mandatory:

- the D1-S1 production implementation and deterministic/code-level gates are present in the working tree and were reported passing;
- the real `spv1_p01` open-intent run did not autonomously select `track_gain`, so the D1 exercised product-path gate is still open;
- Phase D and D1-S1 are therefore **not complete**;
- no claim that the mix "sounds better" is made or permitted as an automatic success condition.

This handoff is a point-in-time historical record. A continuation must re-check the branch, HEAD, working tree, processes, artifacts, source, and tests before relying on it.

## 2. Frozen baseline and working tree

At pause time:

- branch: `codex/g1-g7-runtime-remediation`;
- HEAD: `f579e08d872555df28f9227fa27ec2975a5ec9e6` (`feat: close free-state phase c runtime and acceptance`);
- no Phase D commit was created;
- the D1-S1 implementation and tests remain uncommitted in the working tree;
- the pause operation did not create a branch, discard changes, or commit code.

The observed working tree contained 23 modified tracked files and 7 untracked files. The untracked Phase D files were:

- `agent/internal/chat/free_state_d1_receipt.go`;
- `agent/internal/chat/free_state_d1_render.go`;
- `agent/internal/chat/free_state_d1_runtime.go`;
- `agent/internal/chat/free_state_d1_runtime_test.go`;
- `agent/internal/harness/render_wait_test.go`;
- `scripts/free_state_d1_smoke.py`;
- `scripts/run_free_state_d1_smoke.ps1`.

The tracked changes span the existing free-state prompt/admission path, `audioclosure`, experiment runtime/receipt, chat/audition lifecycle, static-balance VSP port and verifier, harness render telemetry, orchestration receipt details, tests, the phase transition document, and the existing development smoke launcher. Treat every current working-tree change as authoritative input; do not reset, clean, or replace it from HEAD.

## 3. D1-S1 scope implemented in the working tree

The implemented slice is deliberately narrow:

- action domain: `track_gain`;
- action kind: `track_gain_adjust`;
- exactly one observation-bound track target;
- non-zero bounded delta within `+/-2 dB`;
- `experiment_budget=1` and one forward mutation;
- one D1 round; no automatic second dose or `continue_once`;
- Full Project Access still executes only an admitted bounded experiment;
- Project History, Conversation Graph, Worktree, VSP, rollback, audition, journal, and execution coordination are reused rather than rebuilt.

The production path implemented in the working tree is:

```text
fresh observation-backed proposal and G1-G7 admission
-> frozen single-action plan and strong ProjectCut
-> before render
-> executionruntime
-> executionports.StaticBalanceVSPPort
-> VSP CAS mutation and revision advance
-> exact track gain readback
-> fresh post-action CCB evidence bound to the new revision
-> acoustic materiality / target-response separation
-> after render
-> real A/B audition preparation
-> structured human judgment
-> retain, exact rollback, or terminal ambiguous settlement
-> durable D1 receipt and restart projection
```

Implemented safety and audit properties include:

- VSP `base_revision`, project epoch, stable request/idempotency identity, transaction identity, before/after revision, requested target, actual readback, and `readback_verified` details;
- non-advancing revision and missing/mismatched readback fail closed;
- post-action observation must be fresh and its revision must match the mutation after-revision;
- before and after audition candidates carry distinct real audio provenance;
- `audition.select` remains preview-only and does not mutate or settle the project;
- retain, rollback, and ambiguous human outcomes are separated from playback selection;
- ambiguous judgment is terminal for D1-S1 automatic execution and cannot cause another forward mutation;
- rollback delegates to the existing governed rollback path and targets the current D1 intervention, even when a later unrelated journal action exists;
- journal replay with the same action ID does not append a second forward-mutation record;
- restart projection does not duplicate the intervention or post-action observation;
- the D1 receipt exposes `parameter_applied`, `readback_verified`, `evaluation_ready`, `human_audition_ready`, `human_confirmed`, `ambiguous`, `rolled_back`, and `settled` independently.

## 4. Tests and validation reported before pause

The development conversation reported the following results before it was stopped:

- focused Go tests: PASS;
- `go test ./... -count=1`: PASS;
- WebUI: 8 files / 39 tests PASS;
- WebUI build: PASS;
- PowerShell and Python smoke-script syntax checks: PASS;
- `git diff --check`: PASS;
- real VitApp + Godot + rebuilt Go agent startup and basic smoke: PASS.

Added/extended tests cover at least:

- D1 admission and single bounded action enforcement;
- one round / one forward mutation;
- VSP CAS, revision advance, stable request identity, exact readback, and reconcile without retry;
- fresh post-action observation revision binding;
- subthreshold-to-ambiguous human-audition boundary;
- one forward journal mutation under replay;
- read-only post-action continuation idempotency;
- distinct A/B audio provenance;
- retain, exact rollback, and ambiguous settlement through the Server judgment handler;
- rollback selection in the presence of an unrelated later journal action;
- restart recovery without duplicated mutation state;
- render terminal telemetry caching and failure wake-up.

These results are handoff evidence, not a substitute for rerunning validation after resumption. This documentation-only pause turn did not rerun the full suites.

## 5. Latest real-stack smoke result

The latest report is:

`D:/Vit_DAW/artifacts/free_state_d1_s1/20260823_213149/d1_smoke_report.json`

Key report fields:

```ini
schema_version=vit.free_state_d1_smoke.v1
public_case_id=spv1_p01
status=not_exercised
reason=spv1_p01 open run did not autonomously select track_gain
dad_fact_ready_count=6
dad_fact_total_count=6
ui_context_preflight=clean
```

The smoke used the open request `检查一下当前工程有什么问题？`. It read the public manifest only, rejected sealed paths, and did not inject a target track, defect, processor, or parameter. Its `not_exercised` result is therefore the correct fail-closed outcome rather than a failed implementation assertion.

The report shows that the real `spv1_p01` project was prepared with all six DAD facts ready, but the bounded open run terminated without an autonomous `track_gain` proposal. Consequently the live run did not execute the D1 mutation, A/B, or human-judgment path.

At the final handoff check, the following local listeners were present:

- VitApp: TCP 5555 and 5556;
- VitAgent: TCP 7878 and UDP 4445;
- VspHub: TCP 8787.

This process state is transient and must be rechecked. It is not evidence that the same binaries or project remain active in a future conversation.

## 6. Why D1 DoD remains open

Code-level and deterministic gates passing are insufficient for D1 completion. The missing evidence is a real, no-injection, product-path run in which `spv1_p01` autonomously selects the admitted `track_gain` action and reaches:

```text
one real mutation
-> exact readback
-> fresh new-revision evidence
-> acoustic materiality
-> real A/B
-> human judgment
-> retain or rollback/ambiguous settlement
-> restart-consistent final receipt
```

The current public p01 fixture was historically qualified for static EQ, broadband compression, de-essing, and transient-shaping issues, not for a sealed track-gain issue. The runner must not reveal those labels or force `track_gain`. Therefore `not_exercised` must not be relabelled as PASS, and a script-injected target/action must not be introduced merely to close the gate.

No automatic state may claim "the mix improved" based only on parameter application, device-open evidence, readback, render completion, acoustic materiality, or an A/B selection. Human preference and technical/acoustic evidence remain separate receipt layers.

## 7. Required continuation procedure

The next development conversation should begin with a read-only audit:

1. Read `AGENTS.md`, `CURRENT-STATE.md`, this handoff, and the Phase C closeout.
2. Verify branch and exact HEAD, then inspect all modified and untracked files without discarding anything.
3. Re-read the latest D1 smoke report and confirm whether a newer artifact exists.
4. Recheck real-stack process ownership and active project identity; do not assume the listeners recorded above are still valid.
5. Run the focused D1 tests and inspect failures before changing code.
6. Re-run `go test ./... -count=1`, WebUI test/build, script syntax checks, and `git diff --check` after any repair.
7. Discuss and freeze how to obtain a legitimate exercised `track_gain` case without runtime injection. Valid directions include a separately qualified, public-manifest-only p01-derived gain perturbation whose sealed truth is unavailable to the runner, or a revised first action-family decision based on evidence. Do not silently choose one.
8. Run the real three-piece D1 smoke. Exit code 0 and a genuinely exercised report are required before claiming D1 DoD.
9. Test both retain and rollback/ambiguous settlement plus restart consistency on the real path.
10. Only after all gates pass should the Phase D changes be narrowly reviewed, staged, and committed.

Do not begin D2 free-state-loop expansion or D3 holdout signoff until D1-S1 has real exercised evidence and a complete settlement receipt.

## 8. Resume prompt

```text
Continue Vit-DAW Phase D from docs/FREE_STATE_PHASE_D_D1_S1_PAUSED_HANDOFF_2026-08-23.md.

This is a continuation of uncommitted work. Do not reset, clean, discard, or overwrite the working tree. First perform the document's read-only continuation audit and report the exact branch, HEAD, modified/untracked files, latest smoke result, and current real-stack state.

The D1-S1 code-level gates were previously reported passing, but D1 is not complete: the latest spv1_p01 open-intent smoke returned not_exercised because the model did not autonomously select track_gain. Preserve the no-injection and public-manifest-only boundary. Do not claim that the mix sounds better, do not make audition.select mutate the project, and do not rebuild Project History, Conversation Graph, Worktree, VSP, rollback, audition, journal, or the execution coordinator.

After auditing, discuss and freeze a legitimate exercised track_gain acceptance route before making further behavior changes. Then complete real Apply -> readback -> fresh evidence -> acoustic materiality -> A/B -> human judgment -> retain/rollback/ambiguous settlement -> restart receipt verification. D1 DoD requires a real three-piece smoke with exit code 0.
```

