# ADR: Free-State Experiment Runtime and Observable Trajectory v1

- **Status:** proposed / discussion draft
- **Date:** 2026-08-17
- **Owners:** Vit Agent / Vit WebUI
- **Related:** `ADR_AUDIO_CLOSURE_EXPERIMENT_V1.md`, `C2_DYNAMIC_CONTROL_V1.md`, `PROJECT_CHANGE_RECEIPT_V1.md`, `KERNEL_AUDITION_PREVIEW_CONTRACT_V1.md`, `AUDITION_AB_AND_USER_JUDGMENT_V1.md`, `OBSERVABLE_TRAJECTORY_PROTOCOL_V1.md`, `agent_action_workflow_v1_master_plan.md`

## 1. Summary

Vit will extend the existing Project History and Conversation Graph with a
first-class free-state experiment runtime and a user-visible observable
execution trajectory.

This ADR does **not** redesign Project History, Branch, Worktree, Pivot, or
Checkout. Those facilities already exist and remain the recovery and branching
foundation. The new layer runs on top of them and is responsible for expressing
and executing open-ended, evidence-guided audio improvement tasks.

The system must support:

- natural-language requests with or without an explicit audio target;
- multi-round reversible experiments rather than one-shot parameter writes;
- a distinction between insufficient intervention dose and an unsupported
  treatment hypothesis;
- Full Project Access for project-contained, recoverable mutations;
- a compact, progressively disclosed, auditable execution trace instead of
  raw model chain-of-thought;
- durable settlement nodes that integrate completed work back into the
  existing conversation work tree;
- Stems-based product validation with before/after and trajectory evidence.

## 2. Correct Capability Baseline

The current capability route is:

- **C1:** frequency cleanup;
- **C2:** dynamic control;
- **C3:** space and depth;
- **C4:** section automation.

C2 is the recently completed capability milestone. C3 has not yet been
implemented. This ADR describes a horizontal runtime/UI milestone between C2
and C3; it is not a claim that C3 is complete and it is not a replacement for
C3 implementation.

The currently available C1/C2 and processor-control foundations are sufficient
to begin testing free-state improvement experiments. C3 remains the next
vertical audio capability to be added to the same runtime later.

## 3. Existing Foundation (Do Not Rebuild)

The following capabilities are already present and are inputs to this ADR:

### 3.1 Project History and project recovery

Vit already has content-addressed Project History with project snapshots,
checkpoint manifests, branch refs, restore/checkout, saved generations, and
working sessions.

### 3.2 Conversation Graph

A durable `ConversationNode` is bound to a project `CommitID` and carries a
parent node, branch, turn/run identity, message lifecycle, artifacts, and
project result cards. The active conversation is projected from the active
node's ancestor path.

### 3.3 Branch and Worktree operations

The existing History layer supports:

- creating a branch from a checkpoint or conversation node;
- writing a branch marker from the fork node;
- checking out a branch or a conversation node;
- materializing a separate worktree from a checkpoint;
- checking out a worktree with current-state autosave;
- deleting a conversation subtree and unreferenced history objects.

### 3.4 Kernel and Shadow reload

History Checkout and Worktree Checkout already reload the active project in the
Kernel and refresh the Shadow project representation.

### 3.5 Current WebUI History surface

The current `HistoryPane` already exposes worktrees, branches, the Ask Vit
history tree, node selection, branch creation from a node, worktree creation
from a node, and history deletion. This surface is not replaced by the new
trajectory UI.

### 3.6 Existing audio closure and project receipts

`ADR_AUDIO_CLOSURE_EXPERIMENT_V1.md` already defines L1 facts, L2 deterministic
conditions, and L3 evidence-backed improvement hypotheses. It also defines
`needs_experiment`, snapshots, readback, rollback, and technical verification.

This ADR extends that design for repeated free-state execution. It does not
turn an L3 hypothesis into an objectively true mix defect.

### 3.7 Permission and stop-control audit

The inspected current WebUI exposes the existing conversational/goal/plan
interaction modes and pending-interaction cancellation paths. A generic
composer-level Full Project Access selector and a universal running-Turn Stop
button are design requirements for the new runtime/UI layer; they are not
silently treated as already implemented by the current WebUI source. This ADR
preserves the existing cancellation behavior and defines the new explicit
permission/stop controls as implementation work for the next development
phase.

## 4. Problem

Coding-style task loops are not sufficient for DAW audio improvement.

For coding, a small mutation commonly produces a deterministic, inspectable
state difference. For audio, a parameter delta is not a reliable proxy for a
meaningful sonic intervention:

- a 1 dB EQ change may be masked by bandwidth, arrangement, or level;
- a compressor threshold or ratio may change while gain reduction remains
  effectively zero;
- a limiter ceiling may change while no material peaks hit the limiter;
- a dynamic processor may technically run while the target relationship does
  not change;
- a target may improve while a protected musical dimension regresses.

Therefore, the runtime must not treat “parameter changed” as “experiment made
progress,” and it must not treat “no audible improvement” after a subthreshold
intervention as proof that the hypothesis is wrong.

A second problem is presentation. The user needs progress visibility and
control, but raw model chain-of-thought is neither a stable product contract
nor a useful audit record. The UI must expose verified work, evidence, actions,
and decisions instead.

## 5. Terminology

### 5.1 Session

A continuing creative context containing the user's broad intent, protected
preferences, accepted results, rejected directions, and active Project
History scope.

### 5.2 Turn

One user request and its Agent response lifecycle. A Turn may contain several
experiment rounds and may settle, pause, branch, or request user judgment.

### 5.3 Experiment Round

A bounded cycle of observation, hypothesis, intervention, verification, and
decision. A Round may contain multiple dose-calibration attempts.

### 5.4 Intervention

One typed, reversible treatment attempt against a target track, bus, plugin,
relationship, or project scope.

### 5.5 Observable Trajectory

A user-facing projection of meaningful task progress. It includes intent,
observations, hypotheses, actions, materiality, target response, side effects,
rollback, and settlement. It never means raw private chain-of-thought.

### 5.6 Pivot

An existing durable conversation/history node with a recoverable project
Commit that can be used as the origin of a new branch or worktree.

### 5.7 Durable Settlement Node

The compact, durable Conversation Node written after a Turn or Round reaches
a meaningful terminal state. Intermediate live trace events remain transient
unless explicitly promoted to durable history.

## 6. Decisions

### 6.1 Reuse the existing Project History and Conversation Graph

The new runtime will not create a second branch system. It will use the
existing project-bound Conversation Graph, CommitID binding, Branch, Worktree,
Pivot, and Checkout semantics.

A completed experiment that is retained should produce a normal durable
Project History checkpoint and a durable Conversation Node. A user can then
use the existing History UI to create a branch or worktree from that node.

### 6.2 Separate live trajectory from durable conversation history

Live execution stages are transient runtime state and should not append a
permanent conversation node for every internal update.

The runtime may emit transient events such as:

```text
turn.started
intent.framed
observation.started
observation.recorded
hypothesis.proposed
dose.applied
technical.readback
materiality.evaluated
target.response.evaluated
side_effect.detected
round.advanced
rollback.completed
turn.settled
```

The durable conversation granularity remains one completed user-visible
conversation output as one Conversation/History node. A long free-state Turn
may contain many internal Experiment Rounds, but those rounds are not
implicitly promoted to conversation nodes. When the Agent completes a visible
output, the output receives the current Project Commit and becomes the next
durable node; internal round detail is referenced from its settlement payload
or retained in the trajectory/runtime record.

Project History safety checkpoints may be finer-grained than Conversation Nodes
when needed for recovery. That distinction is intentional:

```text
Conversation Node  = completed visible output / durable dialog pivot
Round Checkpoint   = recoverable engineering state inside a long Turn
Live Trace Event   = transient execution progress
```

This preserves the existing History rule that transient activity must not be
written as durable Project History while still giving the user a live DSH-like
trajectory.

### 6.3 Treat `needs_experiment` as a first-class runtime disposition

`needs_experiment` means:

> The Agent has an evidence-linked, bounded improvement hypothesis with a
> typed execution path, a valid comparison plan, and a recoverable mutation
> path. The hypothesis is not asserted to be objectively true.

Admission must include, at minimum:

```text
ExperimentAdmission {
  target_ref
  evidence_refs
  hypothesis
  typed_action
  diagnostic_dose_bounds
  retained_dose_bounds
  experiment_budget
  expected_effect
  protected_dimensions
  verification_plan
  checkpoint_ref
  rollback_plan
  authority_mode
}
```

An L3 experiment is not rejected merely because the current mix cannot be
proved objectively wrong. It is rejected when the target, action, comparison,
recovery, or authority fields are missing or invalid.

### 6.4 Add an Intervention Materiality Gate

An intervention may not be used to classify a hypothesis as effective or
unsupported until it passes a materiality gate.

The evaluation chain is:

```text
technical_application
  -> acoustic_materiality
  -> target_response
  -> net_improvement
```

The runtime must distinguish at least:

```text
technical_application:
  applied | failed | ambiguous

acoustic_materiality:
  none | subthreshold | material

target_response:
  absent | directional | sufficient | ambiguous

net_outcome:
  improved | neutral | regressed | user_judgment_required
```

A technically correct but subthreshold intervention is `insufficient_dose`.
It is not `no_progress` and it is not evidence that the hypothesis is false.

The Materiality Gate must reuse the existing free-state observation system. CCB
already exposes a model-selectable catalog of semantic views and assembles only
the view set requested by the Agent. The runtime must preserve the requested
view set, executed view set, freshness, limitations, and evidence references;
it must not replace the Agent's observation choice with a hard-coded
processor-to-metric table.

The Agent remains responsible for deciding which observations are relevant to
the current target and hypothesis. CCB remains read-only and never chooses a
processor, parameter, treatment, or musical action.

`NoProgressStreak` may advance only after a valid, material intervention has
been evaluated.

### 6.5 Use response-based dose, not parameter delta

The system must not define an effective experiment using a universal value
such as “1 dB.” Effective dose is context-dependent and must be inferred from
processor response and rendered consequences.

Evidence may include, according to the Agent-selected CCB views and the
current hypothesis:

- project structure and project change delta;
- track energy, time dynamics, timbre/frequency, peak, activity, frequency-time
  events, transient, band-dynamics, and stereo-space views;
- multitrack, frequency, and masking relationship views when available;
- processor identity/controls, processor behavior, and processor change delta;
- the CCB observation receipt proving requested/executed view-set equality,
  freshness, limitations, and evidence references;
- existing analytical before/after Render Probe and `ABResult` evidence;
- protected-dimension regression checks;
- user audition judgment when observation evidence cannot settle the choice.

This list describes available evidence, not a mandatory fixed checklist. The
Agent may request a flexible subset and may request additional views during a
later Round when the current bundle is insufficient.

Parameter readback proves application. It does not prove materiality or
improvement.

The trajectory must retain a structured perceptual-dose record rather than only
the requested parameter delta:

```text
PerceptualDoseRecord {
  processor_family
  target_scope
  parameter_domain
  requested_delta
  achieved_delta
  processor_response
  rendered_effect_summary
  calibration_profile
  materiality_status: below_threshold | evaluation_ready | human_confirmed
  evidence_refs
}
```

A calibration profile may provide family-, frequency-, bandwidth-, scope-, and
program-dependent priors for selecting an evaluation-ready dose. It is a
product calibration aid, not a universal claim that a particular parameter
amount is always audible. The first implementation applies to all currently
admitted processor/effect families available to free-state execution, rather
than a fixed EQ/gain/compressor/limiter subset.

The final handoff to a human must be based on an actual meaningful audition A/B
or equivalent evidence, not on a numeric delta alone.

### 6.6 Distinguish analytical A/B from audition A/B

Vit already has an analytical before/after path. CCB exposes
`comparison.before_after`; MOM projects `ABResult`; L2 Render Probe evidence can
carry same-tap before/after revisions and compact level, band, stereo, and risk
deltas; the current Project Result UI can display AB readiness, tap point,
render mode, and delta summary.

This existing path answers whether a valid analytical comparison exists. It is
not yet a user-facing mechanism for switching the audible project between two
candidate states.

The new free-state GUI must add an audition A/B surface for human evaluation.
Comparison scope may be:

1. a selected target or valid tap point;
2. a target-related local bus or relationship scope;
3. the full project when processing interactions make a local comparison
   misleading or incomplete.

Every audition candidate must identify its source checkpoint, Branch or
Worktree, project/render revision, comparison scope, and loudness/reference
handling. The system must never present an analytical delta as if it were a
human preference result.

#### Zero-latency audition invariant

When an A/B control is visible and enabled, both candidates must already be
prepared for playback. A user click must not start a Render, open a project,
Checkout a Branch/Worktree, or wait for Kernel reload. It only selects between
prepared playback sources, preserving timeline position and using a bounded
crossfade or block-boundary handoff where necessary.

Candidate preparation is therefore a background stage of the Agent Turn.
Kernel/JUCE owns the candidate render, preview graph or preview buffer, audio
cache, Transport alignment, and playback-source switching. The runtime may
render a selected region, local scope, or full project, prepare the preview
state, cache loudness/reference metadata, and warm both playback candidates
before emitting `audition.ready`. If both candidates are not ready, the GUI
shows a preparation state and does not present a misleading clickable A/B
control.

The WebUI is a control and presentation client only. It sends preview commands,
shows readiness and active-candidate state, and records user judgment; it is
not the DAW render authority and is not the authoritative audio clock. A
browser/WebUI audio element may be useful for inspecting an already-exported
artifact outside the A/B path, but it is not the product A/B implementation.

The default experience has two explicit planes:

```text
Active Project Plane
  current Branch/Worktree, editable DAW state, Agent mutation target

Audition Preview Plane
  prepared A/B audio candidates, playback selection, temporary comparison state
```

Switching A/B changes only the Kernel-owned Audition Preview Plane. The DAW
Transport remains the authoritative clock, and the Kernel performs the source
handoff at a safe audio-block boundary or through a bounded crossfade while
preserving the current timeline position. The DAW GUI remains on the current
active project and displays a small preview indicator if needed.

The DAW GUI follows a candidate only after the user explicitly chooses an
inspect/apply action, which may perform a normal Branch/Worktree Checkout and
can legitimately take time. This prevents a fast A/B click from making the GUI,
Agent context, and editable project disagree.

### 6.7 Support diagnostic dose and retained dose

The Agent may use a stronger but bounded reversible diagnostic dose to test
causal direction, then reduce the treatment to a lower retained dose.

Within one Experiment Round the Agent may:

1. apply a safe diagnostic dose;
2. verify that the target relationship responds materially;
3. reduce the dose while preserving useful improvement;
4. retain the lower-dose result or roll back.

The diagnostic dose must remain inside the project's recoverability and
listening-safety boundaries. Dose calibration attempts are shown in the live
trajectory but do not automatically become user-visible branches.

### 6.8 Define progress as evidence, uncertainty reduction, or risk reduction

The runtime recognizes three kinds of progress:

1. **Improvement progress:** the target outcome improves without unacceptable
   regression.
2. **Uncertainty reduction:** a valid experiment rules out or supports a
   treatment direction.
3. **Risk reduction:** a direction is shown to damage a protected dimension and
   is rolled back or bounded.

A parameter write, plugin load, or changed number alone is engineering
activity, not acoustic progress.

### 6.9 Provide Full Project Access based on recoverability

Vit will support both:

- **Manual Confirmation:** existing confirmation-oriented behavior;
- **Full Project Access:** autonomous execution for operations within the
  project-contained recoverability boundary.

Full Project Access is a user-selectable Agent permission mode exposed in
the lower-left area of the conversation composer, following the interaction
pattern established by DSH/Codex-style agent products. The mode applies to the
active Agent conversation/session rather than being re-requested for every
parameter write.

In Full Project Access, the Agent does not need per-parameter confirmation for
an admitted reversible experiment. Before mutation, the runtime must still
ensure:

- a checkpoint or equivalent recovery point exists;
- the typed executor is the mutation authority;
- readback is available;
- rollback is deterministic or the operation is isolated in a recoverable
  worktree/branch;
- all actions and outcomes are recorded.

Full Project Access includes autonomous creation of reversible Project History
branches and Worktrees when the Agent decides that a durable alternative or an
independent workspace is useful. The new branch/worktree must be created from
a recorded checkpoint and reported in the observable trajectory.

Full Project Access does not authorize irreversible operations outside the
managed project boundary, such as deleting source media, overwriting external
files, or controlling external hardware. Those actions remain separately
protected.

### 6.10 Use existing branches and worktrees for autonomous alternatives

A normal Experiment Round is not automatically a new branch. Dose calibration
belongs inside the Round, and rollback returns to the Round checkpoint.

However, Full Project Access allows the Agent to create a durable branch or a
new Worktree when a meaningful alternative treatment direction, independent
workspace, or multi-agent task requires it. The Agent does not need a separate
per-branch confirmation for a recoverable operation.

The rules are:

- every new branch/worktree starts from an explicit recoverable checkpoint;
- the parent node, source CommitID, branch/worktree identity, and reason are
  recorded;
- a completed retained result becomes a durable Conversation Node/Pivot;
- the existing HistoryPane remains the long-term branch/worktree UI;
- dose calibration normally remains inside the current Round and does not
  create branch clutter;
- the first implementation does not promise generic DAW state merging;
- adopting a change from another branch means replaying a typed transaction in
  the active branch and re-verifying it.

### 6.11 Capture user judgment as evidence, not as an implicit model rewrite

When technical evidence cannot settle a choice, the Agent should ask the user
for an explicit judgment through a structured interaction rather than infer a
preference from silence or from the next natural-language message.

The interaction should separate at least:

```text
heard_difference: yes | no | unsure
preference: A | B | neither | both_equal | unsure
confidence: optional
reason_tags: optional bounded tags
free_text: optional
```

The raw judgment, the presented A/B candidates, their branch/checkpoint
references, scope, render revision, and loudness/reference handling are stored
as evidence. A later memory or skill system may distill repeated judgments
into a user-specific subjective preference skill, but this ADR does not yet
decide the distillation algorithm, sharing policy, or whether a preference is
global or task-scoped. Raw user evidence must remain available for audit and
correction.

### 6.12 Keep the GUI focused on task progress

The new GUI will be a trajectory layer above the existing HistoryPane.

A Turn should visually expose:

1. intent and inferred/protected goal;
2. current phase;
3. observation summary;
4. hypothesis summary;
5. action and parameter receipt;
6. materiality and target-response result;
7. side effects or rollback;
8. next-round decision;
9. final settlement and branch/pivot references.

Completed rounds collapse by default. The active round remains expanded.
Details are progressively disclosed. The UI never renders raw model
chain-of-thought as the product contract.

`App.tsx` should become a composition/orchestration shell over extracted
trajectory and node renderers rather than accumulate more action-specific
rendering logic.

## 7. Proposed Runtime State Machine

```text
framing
  -> observing
  -> hypothesizing
  -> admitted
  -> dose_calibrating
  -> treating
  -> technically_verifying
  -> materiality_evaluating
  -> target_response_evaluating
  -> deciding
       -> next_round
       -> retained
       -> rolled_back
       -> user_judgment_pending
       -> plateau
       -> blocked_by_capability
       -> stopped
```

Branch, Worktree, and Node Checkout are not valid while an Agent Turn is
running. The existing conversation stop button is the user-visible mechanism
for ending the active run; after the run is stopped (or completes normally),
branch/worktree operations become available. This ADR introduces no separate
audio-specific "stop completion" workflow.

Branch switching is therefore not a mid-Turn suspension primitive. After a
user-initiated stop, unfinished transient trajectory state is marked stopped
and must not leak into the newly selected branch. The latest stable project
checkpoint remains the recovery source.

## 8. Terminal Outcomes

The runtime must distinguish:

- `improved`
- `stable`
- `plateau`
- `needs_user_judgment`
- `blocked_by_capability`
- `blocked_by_observation`
- `rolled_back`
- `budget_exhausted`
- `unsafe_to_continue`
- `stopped`

`technically_verified` is not equivalent to `improved`.

## 9. GUI and Protocol Shape

The trajectory should reuse the existing Agent Event transport rather than
introduce a second event bus. Vit already exposes sequenced `AgentEvent` rows
through the agent-events endpoint, with conversation/goal/run identity,
item identity, status, title/body, payload, lifecycle, persistence, message
kind, and turn identity.

The new design adds a typed `trajectory.*` semantic namespace to that existing
transport. It does not require the WebUI to consume internal controller structs
directly.

For example:

```text
trajectory.turn.started
trajectory.intent.framed
trajectory.observation.recorded
trajectory.hypothesis.proposed
trajectory.round.started
trajectory.intervention.applied
trajectory.intervention.materiality
trajectory.target.response
trajectory.round.decision
trajectory.rollback.completed
trajectory.branch.created
trajectory.worktree.created
trajectory.settled
```

The existing event envelope remains responsible for delivery, sequencing,
polling, deduplication, and conversation scoping. The event payload carries a
versioned trajectory projection, for example:

```text
TrajectoryEventPayload {
  schema_version
  trace_node_id
  turn_id
  round_id
  phase
  status
  summary
  evidence_refs
  action_refs
  project_revision
  checkpoint_ref
  branch_ref
  worktree_ref
  materiality
  target_response
  next_decision
}
```

The WebUI owns a trajectory reducer that groups these events by Turn and Round
and renders them as progressive-disclosure nodes. The transport event is not a
durable Conversation Node. At Turn completion, the controller writes one
settlement node to Project History according to the checkpoint policy below.

The initial visual node registry should include:

- IntentNode
- ObservationNode
- HypothesisNode
- ActionNode
- MaterialityNode
- VerificationNode
- RollbackNode
- BranchOrWorktreeNode
- UserJudgmentNode
- DecisionNode
- SettlementNode
- ErrorNode

The projection must be recoverable from durable history plus the current live
runtime state. A UI refresh must not require replaying raw model output.

## 10. Stems Product Validation

The first product benchmark must contain a deliberately poor but recoverable
mix project and exercise the currently admitted processor families, rather than
only a fixed EQ/gain/compressor/limiter subset. It must contain at least:

- one clear frequency or masking issue;
- one dynamic-control issue;
- a relationship between the issues;
- a nontrivial risk of over-processing;
- at least one direction that requires a valid experiment to reject;
- a result that can be compared by blind A/B listening.

Each benchmark fixture must freeze:

- source stems;
- starting `.vit` project;
- initial render;
- plugin/capability inventory;
- observation package;
- allowed authority mode;
- model/runtime version;
- trajectory and Project History evidence.

Acceptance should evaluate both product result and process quality:

- meaningful blind-listening preference for the improved result;
- no unacceptable protected-dimension regression;
- reproducible project state and history;
- correct distinction between insufficient dose and unsupported hypothesis;
- correct rollback behavior;
- understandable trajectory;
- bounded user intervention count;
- no false claim of improvement when the result is neutral or ambiguous.

## 11. Open Questions for Discussion

The following points remain for the next design pass:

1. **Checkpoint cadence for long free-state Turns:** The existing durable rule
   is one completed conversation output as one Conversation/History node. A
   long free-state execution may contain many internal rounds before producing
   that output. We must decide which internal Round boundaries also receive
   Project History checkpoints for recovery, while keeping them out of the
   durable conversation tree unless explicitly emitted as a completed output.
2. **Evaluation-ready policy:** How should the first version combine the
   Agent-selected CCB evidence bundle, processor/control readback, analytical
   ABResult, and the need for a human audition? The policy should classify
   below-threshold, evaluation-ready, and human-confirmed states without
   pretending to have a universal human threshold.
3. **Audition A/B scope:** When should the GUI offer selected-target, local-bus,
   or full-project A/B switching? How should candidate states be rendered,
   cached, made loudness-comparable, and returned to the active branch?
4. **User judgment interaction:** Which questions and reason tags produce useful
   evidence without turning every free-state round into a questionnaire?
5. **Full Project Access persistence:** The permission is a composer-level
   Agent mode. We still need to decide whether its selected value persists per
   conversation, per project workspace, or per user profile.
6. **Subjective preference distillation:** How should raw user judgments later
   become an editable, task-scoped or shareable subjective preference skill in
   the Hermes-like memory design? This is intentionally deferred; v1 records
   raw evidence first.
7. **Multi-agent worktree policy:** How should multiple Agents announce,
   name, reserve, and settle independent Worktrees without confusing the
   user's active conversation branch?

## 12. Non-Goals

This ADR does not:

- implement C3 space and depth;
- replace the existing Project History, Branch, Worktree, or Checkout design;
- expose raw model chain-of-thought;
- claim a universal numeric “audibility” threshold; the system may still
  maintain calibrated, family- and scope-specific evaluation-dose profiles;
- make a single acoustic metric the definition of a good mix;
- promise generic DAW state merging;
- implement full arrangement or composition automation;
- make every intermediate experiment a permanent conversation node.

## 13. Implementation Sequence

1. Freeze this ADR's existing-foundation and terminology sections.
2. Define the live trajectory projection contract and reducer using mocked
   events.
3. Extract trajectory rendering from `agent/webui/src/App.tsx` without changing
   existing HistoryPane behavior.
4. Add materiality/dose evaluation records that reference Agent-selected CCB
   bundles and existing analytical ABResult evidence.
5. Wire Experiment Round checkpoints and settlement nodes to existing Project
   History.
6. Implement the composer-level Full Project Access mode and its
   recoverability policy.
7. Design the user-facing audition A/B surface, including local and
   full-project comparison states.
8. Design structured user judgment capture and persist raw judgment evidence.
9. Run the first Stems benchmark across all currently admitted processor
   families and capture audio, trajectory, A/B, and user-judgment evidence.
10. Revisit C3 space/depth only after the free-state benchmark passes its
   acceptance gates.

## 14. Initial Acceptance Gates

The ADR is ready to move from `proposed` to `accepted` only when:

- the existing History/Branch/Worktree behavior remains green;
- a mocked multi-round trajectory can render, collapse, stop, and settle;
- a subthreshold intervention is not counted as a failed hypothesis;
- a material but ineffective intervention is correctly classified;
- Agent-selected CCB evidence is preserved with freshness and view-set
  receipts;
- a reversible experiment can run without per-action confirmation in Full
  Project Access;
- rollback restores the exact pre-round project state;
- a retained settlement creates a durable node bound to the correct CommitID;
- Branch/Worktree controls remain unavailable while the Agent is running;
- analytical ABResult and human audition A/B are not conflated;
- structured user judgment is stored with candidate-state evidence;
- the first Stems fixture demonstrates a reproducible, auditable improvement.

