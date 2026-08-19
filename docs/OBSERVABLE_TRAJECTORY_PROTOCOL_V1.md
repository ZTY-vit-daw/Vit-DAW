# Observable Trajectory Protocol v1

- **Status:** implemented protocol / mock reducer
- **Date:** 2026-08-19
- **Schema:** `vit.observable_trajectory.v1`
- **Transport:** existing sequenced `AgentEvent` endpoint
- **Related:** `ADR_FREE_STATE_EXPERIMENT_RUNTIME_AND_OBSERVABLE_TRAJECTORY_V1.md`

## 1. Scope

G1 defines the UI-facing observable trajectory protocol and a deterministic
mock reducer. It does not connect the free-state runtime, create Kernel
Audition candidates, or render the final trajectory GUI.

## 2. Transport Compatibility

Trajectory events use the existing `AgentEvent` envelope:

```text
seq
conversation_id
goal_id
run_id
item_id
item_type = trajectory
status
title
body
payload
created_at
lifecycle = transient
persistence = none
message_kind = activity
turn_id
logical_message_id
```

A trajectory event remains transient. A completed user-visible output is still
written separately as a durable ConversationNode through Project History.

## 3. Versioned Payload

```text
TrajectoryPayload {
  schema_version = vit.observable_trajectory.v1
  trace_node_id
  parent_node_id?
  turn_id
  round_id?
  node_kind
  phase
  status
  summary?
  evidence_refs?
  action_refs?
  project_revision?
  checkpoint_ref?
  branch_ref?
  worktree_ref?
  materiality?
  target_response?
  next_decision?
  outcome?
  details?
}
```

The WebUI reducer ignores unsupported schema versions. The Go projection
validator rejects unsupported event types, node kinds, statuses, evaluation
states, missing Turn identity, and missing Round identity for Round-scoped
events.

## 4. Event Namespace

Implemented v1 event types:

```text
trajectory.turn.started
trajectory.turn.stopped
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
trajectory.user_judgment.requested
trajectory.user_judgment.recorded
trajectory.settled
trajectory.error
```

Audition-specific runtime events remain owned by the later Kernel Audition goal.

## 5. Node Kinds

```text
turn
intent
observation
hypothesis
action
materiality
verification
decision
rollback
branch_or_worktree
user_judgment
settlement
error
```

## 6. Status and Evaluation States

Transport status:

```text
pending
running
completed
stopped
failed
waiting_for_user
```

Evaluation state:

```text
not_ready
insufficient_dose
agent_evaluable
human_audition_ready
human_confirmed
ambiguous
unsupported_hypothesis
rolled_back
```

## 7. Reducer Rules

The WebUI reducer:

1. filters for `trajectory.*` events with schema v1;
2. sorts each incoming batch by `seq`;
3. deduplicates exact replayed events;
4. groups nodes by `turn_id` and optional `round_id`;
5. upserts repeated `trace_node_id` projections;
6. preserves the newest scalar state when an older event arrives late;
7. merges evidence/action references without duplication;
8. preserves stopped Turns;
9. ignores legacy non-trajectory AgentEvents;
10. exposes selectors for ordered Turns, Rounds, and Round nodes.

The reducer stores only projection state. It does not write Project History or
mutate the DAW.

## 8. Mock Scenarios

G1 includes reusable fixtures for:

- a multi-round experiment with `insufficient_dose` followed by a retained
  settlement;
- a rollback path;
- a user-stopped Turn.

These fixtures are intended for G2 trajectory GUI work.

## 9. Implementation Files

```text
agent/internal/trajectory/types.go
agent/internal/trajectory/types_test.go
agent/internal/chat/trajectory_events.go
agent/internal/chat/trajectory_events_test.go
agent/webui/src/trajectory.ts
agent/webui/src/trajectory.test.ts
agent/webui/src/trajectoryMock.ts
```

## 10. G1 Non-Goals

G1 does not:

- emit trajectory events from the real free-state controller;
- add the final trajectory React components;
- split all of `App.tsx`;
- implement Full Project Access or Stop Turn UI;
- implement Kernel Audition or A/B playback;
- change C1/C2 capability behavior;
- change Branch/Worktree behavior.

## 11. Acceptance

G1 is accepted when:

- Go protocol validation tests pass;
- trajectory events are projected through existing transient AgentEvent
  identity/lifecycle rules;
- reducer tests cover Turn/Round grouping, repeated-node updates, duplicate
  replay, late older events, stop, rollback, insufficient dose, settlement,
  unsupported schema, and legacy-event compatibility;
- WebUI TypeScript build passes;
- existing message lifecycle tests remain green.
