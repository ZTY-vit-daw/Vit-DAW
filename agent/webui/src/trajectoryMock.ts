import type { AgentEvent } from "./types";

const base = {
  conversation_id: "mock-conversation",
  goal_id: "mock-goal",
  run_id: "mock-turn",
  turn_id: "mock-turn"
};

export const mockMultiRoundTrajectoryEvents: AgentEvent[] = [
  { ...base, seq: 1, type: "trajectory.turn.started", item_id: "mock-turn", title: "改善主唱清晰度", status: "running", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "mock-turn", turn_id: "mock-turn", node_kind: "turn", phase: "framing", status: "running" } },
  { ...base, seq: 2, type: "trajectory.intent.framed", item_id: "mock-intent", title: "目标已确认", status: "completed", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "mock-intent", turn_id: "mock-turn", node_kind: "intent", phase: "framing", status: "completed", summary: "提高主唱清晰度，不明显增加亮度。" } },
  { ...base, seq: 3, type: "trajectory.observation.recorded", item_id: "mock-observation", title: "观察已完成", status: "completed", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "mock-observation", turn_id: "mock-turn", node_kind: "observation", phase: "observing", status: "completed", evidence_refs: ["ccb:observation:1"] } },
  { ...base, seq: 4, type: "trajectory.round.started", item_id: "mock-round-1", title: "第一轮实验", status: "running", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "mock-round-1", turn_id: "mock-turn", round_id: "mock-round-1", node_kind: "decision", phase: "admitted", status: "running" } },
  { ...base, seq: 5, type: "trajectory.intervention.applied", item_id: "mock-action-1", title: "执行动态让位", status: "completed", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "mock-action-1", turn_id: "mock-turn", round_id: "mock-round-1", node_kind: "action", phase: "treating", status: "completed", action_refs: ["action:1"] } },
  { ...base, seq: 6, type: "trajectory.intervention.materiality", item_id: "mock-materiality-1", title: "处理量不足", status: "completed", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "mock-materiality-1", turn_id: "mock-turn", round_id: "mock-round-1", node_kind: "materiality", phase: "materiality_evaluating", status: "completed", materiality: "insufficient_dose", next_decision: "increase_dose" } },
  { ...base, seq: 7, type: "trajectory.round.started", item_id: "mock-round-2", title: "第二轮实验", status: "running", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "mock-round-2", turn_id: "mock-turn", round_id: "mock-round-2", node_kind: "decision", phase: "admitted", status: "running" } },
  { ...base, seq: 8, type: "trajectory.target.response", item_id: "mock-response-2", title: "目标响应成立", status: "completed", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "mock-response-2", turn_id: "mock-turn", round_id: "mock-round-2", node_kind: "verification", phase: "target_response_evaluating", status: "completed", materiality: "agent_evaluable", target_response: "directional", evidence_refs: ["ccb:observation:2"] } },
  { ...base, seq: 9, type: "trajectory.settled", item_id: "mock-settlement", title: "处理已保留", status: "completed", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "mock-settlement", turn_id: "mock-turn", node_kind: "settlement", phase: "settled", status: "completed", outcome: "agent_evaluable", checkpoint_ref: "commit:mock-final" } }
];

export const mockRollbackTrajectoryEvents: AgentEvent[] = [
  ...mockMultiRoundTrajectoryEvents.slice(0, 5),
  { ...base, seq: 6, type: "trajectory.rollback.completed", item_id: "mock-rollback", title: "副作用过大，已回退", status: "completed", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "mock-rollback", turn_id: "mock-turn", round_id: "mock-round-1", node_kind: "rollback", phase: "rolled_back", status: "completed", outcome: "rolled_back", checkpoint_ref: "commit:mock-baseline" } },
  { ...base, seq: 7, type: "trajectory.settled", item_id: "mock-settlement-rollback", title: "本轮未保留", status: "completed", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "mock-settlement-rollback", turn_id: "mock-turn", node_kind: "settlement", phase: "settled", status: "completed", outcome: "rolled_back", checkpoint_ref: "commit:mock-baseline" } }
];

export const mockStoppedTrajectoryEvents: AgentEvent[] = [
  ...mockMultiRoundTrajectoryEvents.slice(0, 4),
  { ...base, seq: 5, type: "trajectory.turn.stopped", item_id: "mock-stop", title: "用户已停止任务", status: "stopped", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "mock-stop", turn_id: "mock-turn", node_kind: "turn", phase: "stopped", status: "stopped", checkpoint_ref: "commit:mock-stopped" } }
];
