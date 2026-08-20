# G7 Branch, Worktree, and Multi-Agent Collaboration v1

- **Status:** implemented and validated
- **Date:** 2026-08-20
- **Branch:** `codex/g7-branch-worktree-multi-agent`
- **Baseline:** `codex/integration-g7-preflight` at `5b2b9cf`

## 1. Authority boundaries

G7 reuses Project History as the only engineering-history authority. It does not
introduce a second commit graph and does not implement a generic DAW merge.

The state planes remain separate:

- **Active Project Plane:** the Branch/Worktree currently opened by Kernel/JUCE
  and targeted by DAW writes.
- **Audition Preview Plane:** immutable, prepared audio candidates selected only
  for listening.
- **Collaboration Control Plane:** Worktree reservations, Parent/Child tasks,
  owner identity, leases, disposition, and restart recovery.

A/B selection never checks out a Branch or Worktree. Inspect and Apply remain
explicit Active Project Plane operations.

## 2. Fork granularity

### Round Checkpoint

Round Checkpoints remain owned by the Free-State Experiment Runtime. They are
used for dose calibration, rollback, and short-lived recovery, and do not become
long-lived branches by default.

### Branch

`version.branch_create` now supports `activate:false` (or `checkout:false`). An
inactive create writes a Branch ref and optional non-active Branch Marker but
does not materialize the project, change HEAD, change Active Branch, or change
the active Conversation Node.

Activating Branch creation remains backward compatible and is protected by the
running-Turn Checkout Guard. Existing Branch refs are never overwritten.

### Worktree

`version.worktree_create` materializes an independent Project History Worktree
without opening it. WebUI creation no longer performs an implicit
`version.worktree_checkout`; opening is a separate explicit action.

Worktree metadata records display name, safe name, Worktree ref, source Commit,
source Branch/Worktree, parent node, project identity/revision, Goal/Run,
Conversation, owner, purpose, hypothesis, and Reservation ID.

Name collisions append a deterministic six-character suffix. Worktree creation
never overwrites an existing Worktree. A repeated managed request with the same
Reservation or Owner/Goal/Run/source Commit reuses the existing Worktree, so a
restart cannot silently create a duplicate direction.

## 3. Reservation model

Schema: `vit.worktree_collaboration.v1`

A `WorktreeReservation` contains:

- Project UUID, Worktree ref, and Worktree project path;
- Parent node and source Commit;
- source project revision and Branch ref;
- owner Agent, Goal, Run, and Conversation;
- purpose, hypothesis, and display name;
- lifecycle timestamps and optional lease expiration;
- Candidate, Artifact, and Settlement references;
- writer status and result disposition.

Writer statuses:

```text
reserved -> active -> released
                   -> abandoned
                   -> failed
                   -> stale
```

Dispositions:

```text
retain | promote | continue | release | abandon | failed
```

A stale or released Reservation does not delete its Worktree, Branch, Project
History, Candidate, or Artifact.

Supported operations:

```text
collaboration.worktree_reserve
collaboration.worktree_list
collaboration.worktree_renew
collaboration.worktree_recover_stale
collaboration.worktree_release
collaboration.worktree_takeover
collaboration.worktree_disposition
collaboration.worktree_abandon
collaboration.worktree_fail
```

The concurrency key is `(project_uuid, worktree_ref)`. At most one reserved or
active writer is allowed. Owner checks bind Agent, Goal, and Run. Explicit
Takeover is confirmation-gated and preserves all history.

## 4. Parent and Child tasks

A Child Task binds:

- Parent Agent;
- Child Agent, Goal, Run, and Conversation;
- Worktree and Reservation;
- hypothesis, allowed scope, and allowed capabilities;
- structured status, Candidate result, Settlement, and error.

Supported operations:

```text
collaboration.child_register
collaboration.child_update
collaboration.child_list
```

Registering a Child transfers the Reservation writer identity from the Parent
to the Child. A live Child cannot Checkout, Restore, or otherwise change the
Active Project Plane. A reserved writer cannot target another Worktree or the
Parent project path. Terminal Child states update the Reservation:

- completed: retain Candidate evidence and release writer;
- stopped: release writer;
- failed: mark Reservation failed;
- abandoned: mark Reservation abandoned.

## 5. Checkout Guard

The Server and Harness both treat these as Active Project Plane changes:

```text
version.checkout
version.node_checkout
version.worktree_checkout
version.restore
version.branch_create with activate=true or omitted
```

`version.branch_create activate=false` and `version.worktree_create` do not
change the Active Project Plane and remain available for isolated alternatives.
A Running/Processing/Executing/Cancelling Turn blocks all active-plane changes.
Stop/Completed/Failed states allow normal Checkout.

## 6. Cross-Worktree Candidate preparation

`collaboration.candidate_prepare` validates a recoverable engineering source and
a real audio file. It accepts engineering source kinds:

```text
checkpoint | branch | worktree | preview_state
```

The playable Kernel source is always:

```text
scope=target
source_kind=audio_file
```

The Candidate retains engineering provenance, Commit, Branch, Worktree,
project path/UUID/revision, render revision, preview revision, owner,
Reservation, and content-addressed Artifact ref.

Worktree Candidates require a matching active or retained Reservation. Project,
Worktree, path, and owner identities are checked. `full_project` preparation
fails closed.

`collaboration.audition_prepare` requires two distinct prepared audio files and
calls Kernel `audition.prepare`. It does not Checkout, Render, reload the
project, or change the Agent write target. A ready session is bound back to the
Free-State loop so existing User Judgment and Inspect/Apply paths remain in
force.

## 7. Preview validity policy

A prepared audio file becomes a content-addressed immutable Candidate Artifact.
Releasing its Reservation does not invalidate the already-prepared audio; the
user may continue comparing retained directions. The Candidate still points to
the exact Commit and project/render/preview revisions that produced it.

Changing the Active Project revision makes the Audition session stale before a
new Select. The server issues `audition.stale` and rejects selection. Subsequent
changes to a source Worktree do not mutate an already-hashed Candidate Artifact;
a new Candidate must be prepared to represent the new Worktree revision.

## 8. Inspect and Apply

Inspect continues to:

1. create a safety Checkpoint;
2. explicitly Checkout Candidate Branch/Worktree/Commit;
3. reload Kernel and refresh Shadow;
4. verify the resulting project plane;
5. restore the safety point on failure;
6. preserve an Inspection Receipt without adoption.

Apply uses `apply_strategy=explicit_checkout`. It requires immutable User
Judgment evidence, creates safety and adoption Checkpoints, refreshes Shadow,
requests CCB re-observation, settles the Experiment, records the adoption
receipt, promotes/releases the winning Reservation, and preserves Candidate and
Artifact references.

Typed Adoption remains available only where an independently validated typed
transaction exists. G7 does not copy opaque plugin state and does not claim a
generic merge.

## 9. Persistence and recovery

The collaboration snapshot is stored inside the existing project-scoped
`agent_runtime_state.json`. It is restored with the Goal Runtime and Free-State
loops. Reservations whose Goal is no longer active become stale; expired leases
can also be recovered as stale. Child tasks attached to stale Reservations stop
without deleting project state.

## 10. GUI projection

The existing HistoryPane remains authoritative for Branch, Worktree, and
Conversation history. Its minimal G7 extension displays:

- selected Worktree and explicit Open action;
- Reservation status and disposition;
- owner Agent and purpose;
- linked Child Task status;
- running Checkout Guard message.

The Audition card displays Candidate engineering source, Branch/Worktree,
owner, and Reservation. WebUI never renders or plays authoritative audio.

## 11. Unsupported scope

G7 intentionally does not implement:

- generic DAW merge or plugin-state three-way merge;
- full-project audition rendering;
- browser/WebUI audio rendering;
- C3 spatial/depth processing;
- preference-skill distillation;
- G8 Stems product validation;
- automatic deletion of user Branches or Worktrees.
