import { describe, expect, it } from "vitest";
import type { AgentEvent } from "./types";
import {
  emptyTrajectoryState,
  reduceTrajectoryEvents,
  trajectoryNodesForRound,
  trajectoryRounds,
  trajectoryTurns
} from "./trajectory";
import { mockMultiRoundTrajectoryEvents, mockRollbackTrajectoryEvents, mockStoppedTrajectoryEvents } from "./trajectoryMock";

function event(partial: Partial<AgentEvent>): AgentEvent {
  return {
    seq: 1,
    type: "trajectory.turn.started",
    conversation_id: "conversation-1",
    run_id: "turn-1",
    turn_id: "turn-1",
    title: "开始处理",
    payload: {
      schema_version: "vit.observable_trajectory.v1",
      trace_node_id: "turn-node",
      turn_id: "turn-1",
      node_kind: "turn",
      status: "running"
    },
    ...partial
  };
}

describe("observable trajectory reducer", () => {
  it("groups a multi-round turn and updates a node by trace identity", () => {
    let state = reduceTrajectoryEvents(emptyTrajectoryState(), [
      event({ seq: 1 }),
      event({
        seq: 2,
        type: "trajectory.round.started",
        item_id: "round-1",
        title: "开始第一轮",
        payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "round-1", turn_id: "turn-1", round_id: "round-1", node_kind: "decision", phase: "admitted", status: "running" }
      }),
      event({
        seq: 3,
        type: "trajectory.intervention.materiality",
        item_id: "dose-1",
        title: "处理量不足",
        payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "dose-1", turn_id: "turn-1", round_id: "round-1", node_kind: "materiality", phase: "materiality_evaluating", status: "completed", materiality: "insufficient_dose" }
      }),
      event({
        seq: 4,
        type: "trajectory.round.decision",
        item_id: "round-1-decision",
        title: "提高处理量",
        payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "round-1-decision", turn_id: "turn-1", round_id: "round-1", node_kind: "decision", phase: "deciding", status: "completed", next_decision: "increase_dose" }
      }),
      event({
        seq: 5,
        type: "trajectory.round.started",
        item_id: "round-2",
        title: "开始第二轮",
        payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "round-2", turn_id: "turn-1", round_id: "round-2", node_kind: "decision", phase: "admitted", status: "running" }
      }),
      event({
        seq: 6,
        type: "trajectory.settled",
        item_id: "settlement",
        title: "处理完成",
        payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "settlement", turn_id: "turn-1", node_kind: "settlement", phase: "settled", status: "completed", outcome: "agent_evaluable", checkpoint_ref: "commit-2" }
      })
    ]);
    expect(trajectoryTurns(state)).toHaveLength(1);
    expect(trajectoryRounds(state, "turn-1")).toHaveLength(2);
    expect(trajectoryNodesForRound(state, "round-1")).toHaveLength(3);
    expect(state.rounds["round-1"].evaluation).toBe("insufficient_dose");
    expect(state.turns["turn-1"].outcome).toBe("agent_evaluable");
    expect(state.nodes.settlement.checkpointRef).toBe("commit-2");
  });

  it("deduplicates replayed events and merges a repeated trace node", () => {
    const started = event({ seq: 1, type: "trajectory.observation.recorded", item_id: "obs-1", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "obs-1", turn_id: "turn-1", node_kind: "observation", status: "running", summary: "读取证据" } });
    const completed = event({ seq: 2, type: "trajectory.observation.recorded", item_id: "obs-1", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "obs-1", turn_id: "turn-1", node_kind: "observation", status: "completed", summary: "证据已就绪", evidence_refs: ["obs-ref"] } });
    let state = reduceTrajectoryEvents(emptyTrajectoryState(), [started, started, completed]);
    expect(Object.keys(state.nodes)).toEqual(["obs-1"]);
    expect(state.nodes["obs-1"].status).toBe("completed");
    expect(state.nodes["obs-1"].summary).toBe("证据已就绪");
    expect(state.nodes["obs-1"].evidenceRefs).toEqual(["obs-ref"]);
    expect(state.eventKeys).toHaveLength(2);
  });

  it("does not let a late older event regress a completed node", () => {
    const completed = event({ seq: 4, type: "trajectory.observation.recorded", item_id: "obs-late", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "obs-late", turn_id: "turn-1", node_kind: "observation", phase: "observing", status: "completed", summary: "完成" } });
    const state = reduceTrajectoryEvents(
      reduceTrajectoryEvents(emptyTrajectoryState(), [completed]),
      [event({ seq: 2, type: "trajectory.observation.recorded", item_id: "obs-late", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "obs-late", turn_id: "turn-1", node_kind: "observation", phase: "observing", status: "running", summary: "旧事件" } })]
    );
    expect(state.nodes["obs-late"].status).toBe("completed");
    expect(state.nodes["obs-late"].summary).toBe("完成");
    expect(state.turns["turn-1"].activeNodeId).toBe("obs-late");
  });

  it("projects project revision and keeps stop terminal against later events", () => {
    const stopped = event({ seq: 4, type: "trajectory.turn.stopped", item_id: "stop-terminal", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "stop-terminal", turn_id: "turn-1", node_kind: "turn", phase: "stopped", status: "stopped", project_revision: "revision-4" } });
    const later = event({ seq: 5, type: "trajectory.observation.recorded", item_id: "late-after-stop", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "late-after-stop", turn_id: "turn-1", node_kind: "observation", phase: "observing", status: "completed", project_revision: "revision-5" } });
    const state = reduceTrajectoryEvents(reduceTrajectoryEvents(emptyTrajectoryState(), [stopped]), [later]);
    expect(state.nodes["stop-terminal"].projectRevision).toBe("revision-4");
    expect(state.nodes["late-after-stop"].projectRevision).toBe("revision-5");
    expect(state.turns["turn-1"].status).toBe("stopped");
    expect(state.turns["turn-1"].phase).toBe("stopped");
    expect(state.turns["turn-1"].activeNodeId).toBe("stop-terminal");
  });

  it("does not let an older round decision overwrite a newer settlement", () => {
    const settled = event({ seq: 8, type: "trajectory.settled", item_id: "settled-late", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "settled-late", turn_id: "turn-1", node_kind: "settlement", phase: "settled", status: "completed", outcome: "agent_evaluable" } });
    const older = event({ seq: 3, type: "trajectory.round.decision", item_id: "old-decision", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "old-decision", turn_id: "turn-1", round_id: "round-1", node_kind: "decision", phase: "deciding", status: "completed", outcome: "rolled_back", next_decision: "rollback" } });
    const state = reduceTrajectoryEvents(reduceTrajectoryEvents(emptyTrajectoryState(), [settled]), [older]);
    expect(state.turns["turn-1"].outcome).toBe("agent_evaluable");
    expect(state.turns["turn-1"].activeNodeId).toBe("settled-late");
  });

  it("keeps a stopped turn stopped and does not fabricate a new round", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), [
      event({ seq: 1 }),
      event({ seq: 2, type: "trajectory.turn.stopped", item_id: "turn-stop", title: "已停止", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "turn-stop", turn_id: "turn-1", node_kind: "turn", phase: "stopped", status: "stopped" } })
    ]);
    expect(state.turns["turn-1"].stopped).toBe(true);
    expect(state.turns["turn-1"].status).toBe("stopped");
    expect(trajectoryRounds(state, "turn-1")).toHaveLength(0);
  });

  it("reduces the exported mock scenarios", () => {
    const completed = reduceTrajectoryEvents(emptyTrajectoryState(), mockMultiRoundTrajectoryEvents);
    const rolledBack = reduceTrajectoryEvents(emptyTrajectoryState(), mockRollbackTrajectoryEvents);
    const stopped = reduceTrajectoryEvents(emptyTrajectoryState(), mockStoppedTrajectoryEvents);
    expect(completed.turns["mock-turn"].outcome).toBe("agent_evaluable");
    expect(Object.values(rolledBack.nodes).some((node) => node.kind === "rollback" && node.outcome === "rolled_back")).toBe(true);
    expect(stopped.turns["mock-turn"].stopped).toBe(true);
  });

  it("ignores trajectory events with an unsupported schema", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), [
      event({ seq: 1, payload: { schema_version: "vit.observable_trajectory.v0", trace_node_id: "old", turn_id: "turn-1" } })
    ]);
    expect(trajectoryTurns(state)).toHaveLength(0);
    expect(state.nextSeq).toBe(1);
  });

  it("ignores legacy non-trajectory AgentEvents", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), [
      event({ seq: 1, type: "item.started", item_id: "legacy" }),
      event({ seq: 2, type: "turn.completed", item_id: "legacy-turn" })
    ]);
    expect(trajectoryTurns(state)).toHaveLength(0);
    expect(state.eventKeys).toHaveLength(0);
    expect(state.nextSeq).toBe(2);
  });
});
