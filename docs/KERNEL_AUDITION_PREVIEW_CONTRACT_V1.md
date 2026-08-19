# Kernel Audition Preview Contract v1

- **Status:** proposed / G0 design freeze
- **Date:** 2026-08-17
- **Related:** `ADR_FREE_STATE_EXPERIMENT_RUNTIME_AND_OBSERVABLE_TRAJECTORY_V1.md`, `FREE_STATE_CCB_OBSERVATION_PROTOCOL_V1.md`, `PROJECT_CHANGE_RECEIPT_V1.md`
- **Authority:** Kernel/JUCE owns DAW rendering, preview audio, transport, and playback switching. WebUI is a control and presentation client.

## 1. Purpose

This contract defines the Kernel-owned audition layer for Vit's future free-state
experiment runtime. It is designed for A/B listening in which the user should
not wait after clicking A or B.

The contract separates two state planes:

```text
Active Project Plane
  the editable Branch/Worktree currently opened by the DAW and targeted by Agent mutations

Audition Preview Plane
  prepared candidate audio/state used only for comparison listening
```

Selecting A or B changes only the Audition Preview Plane. It does not perform a
Branch Checkout, Worktree Checkout, project open, project save, Kernel reload,
or Agent-context switch.

## 2. Existing Foundation

The contract reuses existing Vit foundations:

- Project History Commit, Branch, Worktree, and Checkpoint references;
- Conversation Graph and durable ConversationNode binding;
- CCB semantic observation bundles and observation receipts;
- MOM/L2 Render Probe analytical before/after evidence;
- existing AgentEvent sequencing and conversation scoping;
- Kernel Transport and project snapshot/render paths;
- Shadow project refresh after actual project Checkout.

This contract does not replace Project History or create a second project
branching system.

## 3. Hard Invariants

1. **Kernel authority:** DAW rendering, candidate preparation, audio decoding,
   Transport synchronization, and A/B source switching are owned by
   Kernel/JUCE.
2. **WebUI control only:** WebUI renders controls and state, sends audition
   commands, and records user judgment. A browser audio element is not the
   authoritative A/B path.
3. **Ready-before-click:** An A/B control is visible and enabled only after all
   candidates needed for that comparison are ready.
4. **No work on click:** `audition.select` must not start Render, open a
   project, Checkout a Branch/Worktree, reload Kernel, or wait for analysis.
5. **Transport continuity:** A/B selection preserves the current timeline
   position, loop/range context, tempo mapping, and playback state whenever the
   candidate scope supports it.
6. **Active project isolation:** Preview selection does not mutate the active
   editable project or change the active Agent branch.
7. **Explicit adoption:** The DAW GUI follows a candidate only after an
   explicit inspect/apply action.
8. **Stale-safe:** A candidate whose project, render, plugin, media, or
   observation revision is stale cannot remain audition-ready.
9. **Recoverable:** Every candidate is bound to a Checkpoint, Branch, Worktree,
   or equivalent immutable preview artifact.

## 4. Core Entities

### 4.1 AuditionSession

```text
AuditionSession {
  schema_version
  id
  conversation_id
  turn_id
  round_id
  created_at
  status: preparing | ready | playing | stopped | stale | failed | settled
  scope: target | local_bus | full_project
  active_candidate: a | b
  transport_anchor
  candidates
  loudness_reference
  evidence_refs
  active_project_ref
}
```

An AuditionSession is a comparison context, not a ConversationNode and not an
active project Checkout.

### 4.2 Candidate

```text
Candidate {
  id
  label: A | B
  source_kind: checkpoint | branch | worktree | preview_state | render_artifact
  source_ref
  project_uuid
  project_revision
  commit_id
  branch_ref
  worktree_ref
  render_ref
  preview_ref
  scope
  duration_seconds
  sample_rate
  channel_count
  loudness_metadata
  transport_metadata
  evidence_refs
  status: preparing | ready | stale | failed
}
```

A candidate must identify both its engineering source and its playable preview
source. A parameter delta without a recoverable source or playable preview is
not an audition candidate.

### 4.3 TransportAnchor

```text
TransportAnchor {
  position_seconds
  tempo
  time_signature
  loop_start_seconds
  loop_end_seconds
  is_looping
  is_playing
  sample_rate
  timeline_revision
}
```

The Kernel captures the anchor when preparation begins and updates it when the
user selects a candidate or the DAW Transport moves.

## 5. Command Contract

All commands are Kernel-facing commands issued through the existing Agent/WebUI
command boundary. They do not grant WebUI mutation authority over the DAW.

### 5.1 `audition.prepare`

Prepare one AuditionSession and its candidates.

```text
{
  "cmd": "audition.prepare",
  "session_id": "aud_...",
  "scope": "target | local_bus | full_project",
  "candidate_a": { ...source reference... },
  "candidate_b": { ...source reference... },
  "transport_anchor": { ... },
  "loudness_reference": { ... },
  "evidence_refs": [ ... ]
}
```

Preparation may perform background work:

- materialize or resolve candidate state;
- render a target, local, bus, or full-project preview;
- create or reuse a Kernel-owned preview buffer/cache;
- compute and cache loudness/reference metadata;
- warm both playback sources;
- verify transport alignment and media availability.

Preparation must not alter the active editable project.

### 5.2 `audition.status`

Return readiness and failure details without changing audio state.

### 5.3 `audition.ready`

This is an event emitted only when the requested candidate set is playable.
It includes the session ID, candidate IDs, preview refs, render revisions,
scope, transport anchor, and readiness evidence.

### 5.4 `audition.select`

```text
{
  "cmd": "audition.select",
  "session_id": "aud_...",
  "candidate": "a | b"
}
```

Preconditions:

- session status is `ready` or `playing`;
- selected candidate is `ready`;
- current session and project identity still match.

Effects:

- select the already-prepared Kernel playback source;
- preserve timeline position and playback state;
- switch at a safe audio-block boundary or bounded crossfade;
- emit the active-candidate event.

It must not render or Checkout.

### 5.5 `audition.position`

Synchronize or query the audition position with the authoritative DAW
Transport. WebUI may request a position update, but Kernel owns the resulting
clock and reports the actual position.

### 5.6 `audition.stop`

Stop the preview playback without changing the active project or its Branch.
It may release or retain warm buffers according to cache policy.

### 5.7 `audition.inspect_candidate`

Explicitly make a candidate inspectable in the DAW GUI. This is not a
zero-latency A/B operation and may perform a normal Branch/Worktree Checkout or
open a candidate workspace. It must clearly report that the Active Project
Plane is changing.

### 5.8 `audition.apply_candidate`

Explicitly adopt a candidate into the active project/branch. The implementation
must use a typed, recoverable operation and produce a Project Change Receipt,
new Checkpoint/Commit as appropriate, and a refreshed observation state.

## 6. Candidate Preparation Pipeline

```text
candidate.requested
  -> source.validated
  -> preview.rendering_or_resolved
  -> loudness_metadata.ready
  -> transport.aligned
  -> audio_source.warmed
  -> audition.ready
```

The Agent Turn may continue while preparation is running. The trajectory shows
preparation progress, but the user-facing A/B control remains disabled until
both candidates are ready.

For a full-project comparison, preparation may be expensive. The latency is
paid during the Agent's background work, not after the user clicks A/B.

## 7. Scope Modes

### 7.1 Target scope

Used when the treatment is isolated and a local preview is musically valid.

### 7.2 Local bus or relationship scope

Used when the target depends on a related set of tracks, such as Vocal Bus,
Drum Bus, Bass/Kick, or a masking relationship.

### 7.3 Full-project scope

Used when processing is coupled through buses, dynamics, routing, or Master
processing and a local preview would misrepresent the result.

The selected scope must be shown in the GUI and included in user judgment
records.

## 8. Loudness and Reference Handling

The Kernel stores loudness/reference metadata with each candidate. The first
version must make the reference policy explicit rather than silently favoring
the louder candidate.

A candidate may use metadata-based playback gain adjustment when that does not
invalidate the comparison. If a reliable comparison requires a new render, it
must happen during preparation, before `audition.ready`.

## 9. Cache and Invalidation

A preview cache key must include enough identity to prevent stale audition:

```text
preview_cache_key = hash(
  project_uuid,
  source_ref,
  project_revision,
  render_revision,
  scope,
  media_revision,
  plugin_state_revision,
  transport_anchor,
  loudness_reference,
  preview_schema_version
)
```

Invalidate a candidate when:

- its source Commit/Branch/Worktree changes;
- the active project or media revision changes;
- a required plugin or processor state changes;
- the render revision is stale;
- the candidate audio is missing or corrupt;
- the requested scope no longer matches the evidence.

Invalidation emits a trajectory event and disables the A/B control until
re-preparation completes.

## 10. DAW GUI Policy

The default GUI displays the Active Project Plane. During preview it may show:

```text
A/B Preview: Candidate B
Active project: main / current Worktree
```

Plugin parameters and track controls shown in the normal DAW GUI remain those
of the active editable project. The GUI does not silently display Candidate B
parameters while Candidate A remains editable.

The user may explicitly choose `inspect_candidate` to view the candidate's
Branch/Worktree. The user may explicitly choose `apply_candidate` to make the
candidate the active editable result.

## 11. Failure and Safety

Preparation failures are explicit:

- `candidate_missing`
- `render_failed`
- `media_missing`
- `plugin_state_unavailable`
- `transport_alignment_failed`
- `loudness_reference_unavailable`
- `stale_candidate`
- `preview_cache_corrupt`

The GUI must not show an enabled A/B control for a failed or stale candidate.

Full Project Access may autonomously prepare candidates and create recoverable
Branches/Worktrees. It may not silently perform an active-project Checkout as
part of `audition.select`.

## 12. Event Projection

The Kernel emits trajectory-compatible events through the existing AgentEvent
transport:

```text
audition.prepare.started
audition.candidate.progress
audition.ready
audition.select.changed
audition.position.changed
audition.stale
audition.failed
audition.stopped
audition.inspect.started
audition.apply.started
audition.apply.settled
```

The WebUI uses these events to update the trajectory and A/B controls. It does
not infer readiness from a text message.

## 13. Acceptance Gates

The contract is ready for implementation when tests prove:

- A/B remains disabled until both candidates are ready;
- `audition.select` performs no Render, Checkout, or Kernel Reload;
- A/B switching preserves Transport position within the defined tolerance;
- switching uses the Kernel audio path rather than a browser audio element;
- stale candidates disable the control;
- full-project candidates can be prepared before user audition;
- DAW GUI remains on the Active Project Plane during ordinary A/B selection;
- explicit inspect/apply is the only path that makes the GUI follow a candidate;
- all candidate references are recoverable and auditable.

## 14. Non-Goals

This contract does not implement:

- general DAW state merge;
- silent project Checkout on A/B click;
- browser-owned DAW rendering;
- a universal loudness or audibility threshold;
- user preference distillation;
- C3 spatial processing.
