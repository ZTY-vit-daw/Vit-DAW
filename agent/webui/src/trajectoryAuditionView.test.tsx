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
