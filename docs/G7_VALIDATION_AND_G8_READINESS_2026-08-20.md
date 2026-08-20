# G7 Validation and G8 Readiness — 2026-08-20

- **Worktree:** `D:\Vit_DAW_worktrees\g7-branch-worktree-multi-agent`
- **Branch:** `codex/g7-branch-worktree-multi-agent`
- **Baseline:** `5b2b9cfe6197fbfb813db8d8c01e60034515bc7b`
- **Implementation contract:** `docs/G7_BRANCH_WORKTREE_MULTI_AGENT_COLLABORATION_V1.md`

## 1. Audit findings

G7 reused the existing content-addressed Project History, Conversation Graph,
Working Sessions, Saved Generations, Free-State Experiment Runtime, Full Project
Access, Stop/Checkout Guard, Kernel Audition Preview, User Judgment, and
Candidate Inspect/Apply.

The principal pre-G7 gap was that `version.branch_create` always activated the
new Branch while the backend running-Turn Guard did not classify Branch creation
as an Active Project Plane operation. Reservation, Parent/Child task ownership,
and governed cross-Worktree Candidate preparation were absent.

## 2. Implemented model and lifecycle

Implemented schema `vit.worktree_collaboration.v1` with:

- `WorktreeReservation`;
- `ChildTask`;
- project-scoped Snapshot/Restore;
- single-writer key `(project_uuid, worktree_ref)`;
- Agent/Goal/Run/Conversation owner validation;
- Reserve, List, Renew, Release, Takeover, Disposition, Abandon, Fail;
- lease expiration and stale recovery;
- Candidate, Artifact, and Settlement audit references;
- dispositions `retain`, `promote`, `continue`, `release`, `abandon`, `failed`.

No lifecycle operation deletes a Branch, Worktree, Project History object, or
Candidate Artifact.

## 3. Fork and Checkout evidence

Automated tests prove:

- two inactive Branches share one Pivot Commit and later diverge independently;
- inactive Branch creation does not modify project bytes, Active Branch, HEAD,
  or Active Conversation Node;
- Branch name collisions do not overwrite refs;
- Worktree collisions add a deterministic short suffix;
- identical managed restart requests reuse their existing Worktree rather than
  creating a duplicate;
- Worktree creation remains non-active;
- Full Project Access can create inactive Branches and reserved Worktrees
  without per-operation confirmation;
- Manual Confirmation still returns `needs_confirmation`;
- active Branch creation and all Checkout/Restore operations are rejected while
  a Turn is running;
- Stop allows normal Checkout;
- Child tasks cannot Checkout the Parent Active Project Plane.

## 4. Reservation and multi-agent isolation evidence

Automated tests prove:

- a second writer cannot reserve an occupied Worktree;
- a non-owner DAW mutation is rejected;
- a reserved Child cannot target the Parent project path;
- two independent Worktrees can be written concurrently without changing the
  Parent `.vit` file;
- Parent-to-Child writer transfer binds the exact Child Agent/Goal/Run;
- Child failure marks its Reservation failed and leaves Parent project bytes
  unchanged;
- Child completion records Candidate/Artifact/Settlement evidence and releases
  the writer;
- explicit Takeover and all result dispositions preserve Worktree state;
- project restart restores Reservations and Child Tasks;
- expired leases are recovered through the production restore/list/recover
  paths, release writer ownership, and retain Worktree history;
- a Reservation whose Goal is no longer active restores as stale rather than an
  active writer.

## 5. Cross-Worktree Candidate and real Kernel A/B evidence

`collaboration.candidate_prepare`:

- requires a real local audio file;
- supports engineering sources `checkpoint`, `branch`, `worktree`, and
  `preview_state`;
- validates Commit, project UUID/path/revision, Worktree, owner, and Reservation;
- computes content SHA-256, render revision, preview revision, and Artifact ref;
- emits a playable `source_kind=audio_file` Candidate while retaining
  engineering provenance;
- rejects unsupported full-project scope.

`collaboration.audition_prepare`:

- requires two distinct target-scope audio files;
- calls Kernel `audition.prepare`;
- does not Checkout, Render, open a project, reload the active project, or change
  the Agent writer;
- binds the ready Kernel session to the existing Free-State/User Judgment flow.

Kernel C++ tests use two different real WAV files, decode and warm both buffers,
switch A/B in the Audition Preview Plane, and confirm the Active Project Plane
diagnostic counters remain unchanged.

Candidate protocol and Kernel session serialization retain:

```text
engineering_source_kind
engineering_source_ref
branch_ref
worktree_ref
checkpoint_ref
commit_id
project_path
project_uuid
project_revision
render_revision
preview_revision
owner_agent_id
reservation_id
artifact_ref
```

Active Project revision change marks the Audition session stale before Select.
A released Reservation does not invalidate an immutable, content-addressed audio
Candidate; later Worktree changes require preparation of a new Candidate.

## 6. Inspect, Judgment, and Apply evidence

Existing explicit Inspect/Apply behavior remains in force. Tests prove:

- Inspect does not adopt;
- Inspect failure restores the safety Checkpoint;
- A/B Select does not record Judgment;
- Judgment does not mutate the project;
- Apply requires exact preferred-candidate Judgment evidence;
- Apply uses `explicit_checkout`, creates safety/adoption Checkpoints, refreshes
  Shadow, requests CCB re-observation, settles the Experiment, writes a durable
  Conversation Node, and records a Project Change Receipt;
- winning Worktree Reservation becomes `promote/released` and retains Candidate,
  Artifact, and Settlement references.

No generic DAW merge or opaque plugin-state merge was introduced.

## 7. GUI evidence

The existing HistoryPane was minimally extended to show:

- non-active Worktree creation and separate explicit Open action;
- Reservation status and disposition;
- owner Agent and purpose;
- linked Child Task state;
- running Checkout Guard state.

The existing Audition card now shows engineering source, Branch/Worktree,
owner, and Reservation. The browser remains a control/presentation client and
does not render authoritative audio.

## 8. Validation commands and results

### Go targeted packages

All passed:

```text
go test ./internal/history -count=1
go test ./internal/runtime -count=1
go test ./internal/chat -count=1
go test ./internal/harness -count=1
go test ./internal/experiment -count=1
go test ./internal/kernel -count=1
go test ./internal/trajectory -count=1
go test ./internal/projectworkspace -count=1
go test ./internal/collaboration -count=1
```

### Go full repository

Passed:

```text
go test ./... -count=1
```

### WebUI

Passed:

```text
npm test
# 6 test files, 34 tests passed

npm run build
# TypeScript check and Vite production build passed
```

### Kernel Audition

Generator and compiler:

```text
Visual Studio 18 2026
MSVC 19.50
Windows SDK 10.0.26100.0
```

CTest discovery:

```text
Test #1: VitAuditionPreviewAudioPlaneTests
Test #2: VitAuditionPreviewStateTests
Total Tests: 2
```

Build and test result:

```text
VitAuditionPreviewAudioPlaneTests  Passed
VitAuditionPreviewStateTests       Passed
100% tests passed, 0 failed
```

The build emitted pre-existing Tracktion/JUCE code-page/deprecation and temporary
build-directory warnings; no compile or test failure occurred.

### Repository checks

```text
gofmt -w <changed Go files>
git diff --check
```

Both completed without formatting or whitespace errors.

## 9. Remaining unsupported scope

The following remain intentionally unsupported:

- full-project audition rendering;
- local-bus/full-project Kernel audio preparation beyond the validated target
  audio-file path;
- generic DAW merge;
- automatic field-level merge of opaque plugin state;
- WebUI/browser audio rendering;
- C3 spatial/depth work;
- preference-skill distillation;
- G8 Stems product validation;
- automatic deletion of Branches or Worktrees.

## 10. G8 readiness conclusion

G7 is ready as the collaboration and recovery foundation for G8.

Real Stems may be used for **target-scoped canary validation** when each compared
direction produces a real prepared audio file. Full-project Stems product
validation is **not yet ready**, because full-project Candidate rendering remains
fail-closed and is explicitly outside G7 scope.
