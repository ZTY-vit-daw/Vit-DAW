# Audition A/B and User Judgment v1

- **Status:** proposed / G0 design freeze
- **Date:** 2026-08-17
- **Related:** `ADR_FREE_STATE_EXPERIMENT_RUNTIME_AND_OBSERVABLE_TRAJECTORY_V1.md`, `KERNEL_AUDITION_PREVIEW_CONTRACT_V1.md`, `FREE_STATE_CCB_OBSERVATION_PROTOCOL_V1.md`

## 1. Purpose

This document defines how Vit moves from analytical before/after evidence to
human audition and structured user judgment. It does not replace CCB, MOM,
L2 Render Probe, Project History, Branch, Worktree, or Kernel audition.

## 2. Two Different A/B Systems

### 2.1 Analytical A/B

Analytical A/B is evidence about whether two observations or Render revisions
can be compared. Existing Vit foundations include:

- CCB `comparison.before_after`;
- L2 Render Probe before/after evidence;
- MOM `ABResultComparison`;
- before/after evidence references;
- render revisions;
- compact level, band, stereo, and risk deltas;
- readiness, stale, suspect, and missing states.

Analytical A/B is consumed by the Agent and Runtime. It can support a decision,
but it is not a statement of human preference.

### 2.2 Audition A/B

Audition A/B is a Kernel-owned playback session in which the user can hear
candidate A and candidate B without waiting after clicking the control.

Audition A/B requires:

- two prepared Kernel candidates;
- explicit scope;
- transport alignment;
- loudness/reference metadata;
- candidate source/checkpoint/Branch/Worktree references;
- ready playback state;
- user-facing A/B controls.

The WebUI only controls and displays the Kernel audition session.

## 3. Evaluation States

The Runtime separates evidence readiness from user preference:

```text
not_ready
  insufficient, stale, missing, malformed, or not comparable

insufficient_dose
  the action was applied but the resulting intervention is below the
  evaluation threshold; the hypothesis is not rejected

agent_evaluable
  the observation bundle and technical response are sufficient for Agent
  evaluation, without requiring a human preference

human_audition_ready
  analytical evidence is valid and prepared A/B playback is available

human_confirmed
  the user has supplied a structured judgment for the candidates

ambiguous
  evidence or listening result cannot support a stable decision

unsupported_hypothesis
  a material intervention produced no relevant target response or the
  expected direction was contradicted

rolled_back
  the candidate or Round was not retained and the project returned to its
  recoverable prior state
```

These states are not strictly linear. For example, `insufficient_dose` may
return to another intervention in the same Round, and `human_audition_ready`
may become `ambiguous` without becoming `human_confirmed`.

## 4. Entering `evaluation_ready`

The first version must not claim a universal human-perceptible parameter
amount. It should determine whether the current experiment is ready for the
next judgment using a combination of:

1. valid target binding;
2. fresh CCB evidence whose requested and executed view sets match;
3. successful typed action and parameter/readback verification;
4. a material or intentionally diagnostic rendered response;
5. relevant target-response evidence selected by the Agent;
6. protected-dimension checks;
7. valid analytical before/after evidence when available;
8. a prepared audition candidate when human evaluation is required.

The Runtime records which conditions were satisfied. It does not invent an
observation checklist or add CCB views that the Agent did not request.

## 5. Entering `human_audition_ready`

A comparison may enter `human_audition_ready` only when:

- candidate A and B have valid recoverable source references;
- both candidates are prepared by Kernel/JUCE;
- the comparison scope is explicit;
- the candidate Render/project revisions are current;
- the Transport anchor is valid;
- the loudness/reference policy is visible;
- the GUI can switch A/B without starting new work;
- the analytical evidence is not stale or suspect in a way that invalidates
  the proposed comparison.

`human_audition_ready` means “ready for a meaningful human comparison.” It does
not mean the Agent or Runtime already knows which candidate is better.

## 6. Candidate Presentation

The GUI should display one compact comparison card:

```text
A/B 试听已准备
范围：全工程
目标：提高主唱清晰度

A  自然版本       已准备
B  动态让位版本   已准备

[播放 A] [播放 B]
[查看 A] [查看 B]
[采用 A] [采用 B]
```

The card must show or make expandable:

- target and user goal;
- scope;
- candidate labels and short summaries;
- source Branch/Worktree/Checkpoint;
- analytical evidence status;
- loudness/reference handling;
- current audition candidate;
- preparation or stale status;
- inspect/apply actions.

A/B click changes only the Kernel Audition Preview Plane. `查看` and `采用`
are explicit project-state operations and may take time.

## 7. User Judgment Interaction

Vit should ask for judgment only when the next decision depends on human
hearing or preference. The question should be short and structured.

### 7.1 Difference question

```text
你能听出 A 和 B 的区别吗？
[能] [不能] [不确定]
```

### 7.2 Preference question

```text
如果能听出，你更偏好哪个？
[A] [B] [都差不多] [不确定]
```

### 7.3 Optional reason tags

```text
[更清晰]
[更自然]
[更有力度]
[更稳定]
[更少刺耳]
[更宽]
[其他]
```

Free text is optional and should not be required for every Round.

The system must keep “heard a difference” separate from “preferred a
candidate.” A click, replay count, or time spent listening is not itself a
preference judgment.

## 8. UserJudgmentEvidence

```text
UserJudgmentEvidence {
  schema_version
  id
  conversation_id
  turn_id
  round_id
  audition_session_id

  candidate_a_ref
  candidate_b_ref
  candidate_a_render_ref
  candidate_b_render_ref
  candidate_a_commit_id
  candidate_b_commit_id
  candidate_a_branch_ref
  candidate_b_branch_ref
  candidate_a_worktree_ref
  candidate_b_worktree_ref

  scope
  transport_anchor
  loudness_reference
  analytical_evidence_refs

  heard_difference: yes | no | unsure
  preference: a | b | neither | equal | unsure
  reason_tags
  free_text
  created_at
}
```

The evidence is immutable after recording except through an explicit correction
record. The candidate references must remain resolvable for later audit.

## 9. Full-Project A/B

For processing whose audible result is coupled across the project, the
AuditionSession may use full-project candidates. The user must not have to
wait after clicking A/B; preparation occurs before `human_audition_ready`.

The full-project candidate may be backed by:

- a Branch Checkpoint;
- a Worktree render;
- a Kernel preview graph;
- a Kernel-owned Render artifact;
- another recoverable candidate source explicitly recorded by the Runtime.

The WebUI must never independently render a DAW candidate. It only sends
`audition.select` and displays Kernel readiness/active-candidate state.

## 10. Candidate Adoption

If the user chooses `查看 A/B`, Vit may perform a normal project-state Checkout
so the DAW GUI follows the selected candidate.

If the user chooses `采用 A/B`, Vit must:

1. create or identify the target recoverable project state;
2. perform a typed, auditable adoption operation;
3. create a Project Change Receipt and appropriate Checkpoint/Commit;
4. refresh Kernel and Shadow state;
5. re-observe affected CCB views;
6. write or update the durable Conversation Settlement.

A/B audition alone never changes the active editable project.

## 11. Raw Judgment and Future Preference Skill

v1 stores raw `UserJudgmentEvidence`. It does not automatically rewrite the
user's model prompt, global style, or long-term preference profile.

A future Hermes-like memory layer may distill repeated judgments into an
editable subjective preference skill. That future layer must decide:

- global versus task-scoped preference;
- conflict and recency rules;
- user editing and deletion;
- sharing/export policy;
- confidence and evidence count;
- how to avoid turning a single A/B choice into a permanent preference.

The raw evidence remains the source of truth and must be linkable from any
future distilled preference.

## 12. Event Projection

The existing AgentEvent transport can expose:

```text
trajectory.audition.preparing
trajectory.audition.ready
trajectory.audition.selectable
trajectory.audition.selected
trajectory.audition.stale
trajectory.audition.failed
trajectory.user_judgment.requested
trajectory.user_judgment.recorded
```

The WebUI reducer uses these events to update the comparison card. Durable
Project History receives the final settlement and user evidence references,
not every playback click.

## 13. Acceptance Gates

The design is ready for implementation when tests prove:

- analytical A/B and audition A/B are represented separately;
- an A/B control is disabled until Kernel reports both candidates ready;
- A/B selection does not invoke Render, Checkout, or Kernel Reload;
- candidate source and Render references are retained;
- the user can answer difference and preference separately;
- a preference cannot be inferred from click counts alone;
- a user judgment references exact candidate and project state;
- full-project A/B uses the Kernel audio path;
- inspect/apply is the only path that makes the DAW GUI follow a candidate;
- raw judgment evidence survives refresh and Project History reload.

## 14. Non-Goals

This document does not define:

- a universal human audibility threshold;
- automatic preference distillation;
- generic DAW project merging;
- browser-owned DAW rendering;
- automatic user preference changes from one judgment;
- C3 space/depth processing.
