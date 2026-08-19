import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { emptyAuditionState, reduceAuditionEvents } from "./audition";
import { emptyTrajectoryState, reduceTrajectoryEvents } from "./trajectory";
import { TrajectoryAuditionPanel } from "./trajectory/TrajectoryAuditionPanel";
import type { AgentEvent } from "./types";

const trajectoryEvent: AgentEvent = { seq: 1, type: "trajectory.turn.started", item_id: "trace-1", payload: { schema_version: "vit.observable_trajectory.v1", turn_id: "turn-1", trace_node_id: "trace-1", node_kind: "turn", status: "running", summary: "started" } };
const auditionEvent: AgentEvent = { seq: 2, type: "audition.ready", item_id: "audition-1", payload: { schema_version: "vit.kernel_audition.v1", session: { session_id: "audition-1", status: "ready", candidates: [{ id: "candidate-a", label: "Before", status: "ready", preview_ref: "a" }, { id: "candidate-b", label: "After", status: "ready", preview_ref: "b" }] } } };

describe("live trajectory and audition projection", () => {
  it("renders real AgentEvent projections together", () => {
    const markup = renderToStaticMarkup(<TrajectoryAuditionPanel trajectory={reduceTrajectoryEvents(emptyTrajectoryState(), [trajectoryEvent])} audition={reduceAuditionEvents(emptyAuditionState(), [auditionEvent])} busySessionID="" onSelect={async () => {}} onStop={async () => {}} />);
    expect(markup).toContain("自由态实验轨迹");
    expect(markup).toContain("A/B AUDITION");
    expect(markup).toContain("Before");
    expect(markup).toContain("After");
  });
});


it("shows the structured judgment form only after Runtime requests a ready session", () => {
  const ready: AgentEvent = { seq: 1, type: "audition.ready", item_id: "audition-ready", conversation_id: "conversation-1", payload: { schema_version: "vit.kernel_audition.v1", session: { session_id: "audition-ready", conversation_id: "conversation-1", turn_id: "turn-1", round_id: "round-1", project_revision: "rev-1", status: "ready", candidates: [{ id: "candidate-a", label: "A", status: "ready", preview_ref: "a" }, { id: "candidate-b", label: "B", status: "ready", preview_ref: "b" }] } } };
  const requested: AgentEvent = { seq: 2, type: "trajectory.user_judgment.requested", conversation_id: "conversation-1", payload: { schema_version: "vit.observable_trajectory.v1", turn_id: "turn-1", round_id: "round-1", details: { audition_session_id: "audition-ready" } } };
  const beforeRequest = renderToStaticMarkup(<TrajectoryAuditionPanel trajectory={emptyTrajectoryState()} audition={reduceAuditionEvents(emptyAuditionState(), [ready])} busySessionID="" onSelect={async () => {}} onStop={async () => {}} onSubmitJudgment={async () => {}} />);
  const afterRequest = renderToStaticMarkup(<TrajectoryAuditionPanel trajectory={emptyTrajectoryState()} audition={reduceAuditionEvents(emptyAuditionState(), [ready, requested])} busySessionID="" onSelect={async () => {}} onStop={async () => {}} onSubmitJudgment={async () => {}} />);
  expect(beforeRequest).not.toContain("你能听出 A 和 B 的区别吗？");
  expect(afterRequest).toContain("你能听出 A 和 B 的区别吗？");
  expect(afterRequest).toContain("都不喜欢");
  expect(afterRequest).toContain("更少刺耳");
  expect(afterRequest).toContain("补充说明（可选）");
});

it("does not show a submit form while either candidate is not ready", () => {
  const preparing: AgentEvent = { seq: 1, type: "audition.prepare", item_id: "audition-preparing", payload: { schema_version: "vit.kernel_audition.v1", session: { session_id: "audition-preparing", conversation_id: "conversation-1", turn_id: "turn-1", round_id: "round-1", project_revision: "rev-1", status: "preparing", candidates: [{ id: "candidate-a", label: "A", status: "ready", preview_ref: "a" }, { id: "candidate-b", label: "B", status: "preparing" }] } } };
  const requested: AgentEvent = { seq: 2, type: "trajectory.user_judgment.requested", payload: { schema_version: "vit.observable_trajectory.v1", turn_id: "turn-1", round_id: "round-1", details: { audition_session_id: "audition-preparing" } } };
  const markup = renderToStaticMarkup(<TrajectoryAuditionPanel trajectory={emptyTrajectoryState()} audition={reduceAuditionEvents(emptyAuditionState(), [preparing, requested])} busySessionID="" onSelect={async () => {}} onStop={async () => {}} onSubmitJudgment={async () => {}} />);
  expect(markup).not.toContain("提交判断");
});
