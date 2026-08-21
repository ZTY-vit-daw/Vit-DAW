import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { normalizeTaskTrajectory } from "../taskTrajectory";
import { TaskTrajectoryView } from "./TaskTrajectoryView";

function runtimeSnapshot(state: string, continuation: Record<string, unknown> = {}, interaction?: Record<string, unknown>) {
  return normalizeTaskTrajectory({
    schema_version: "vit.task_runtime_trajectory.v1",
    task: { task_id: "task-gui", goal_id: "goal-gui", run_id: "run-gui", original_intent: "检查一下当前工程有什么问题？", status: "waiting_continuation", updated_at: "2026-08-21T11:00:00Z" },
    run: {
      run_id: "run-gui", current_slice_id: "slice-2", current_turn_id: "turn-2",
      slices: [{ slice_id: "slice-1", sequence: 1, max_turns: 4, status: "waiting_continuation" }, { slice_id: "slice-2", sequence: 2, max_turns: 4, status: "running" }],
      turns: [{ turn_id: "turn-1", slice_id: "slice-1", sequence: 1, source: "user", status: "waiting_continuation" }, { turn_id: "turn-2", slice_id: "slice-2", sequence: 2, source: "automatic_continuation", status: "running" }]
    },
    semantic: { state, revision: 5, project_revision: "revision-5", evidence_refs: ["project.state:revision-5"], pending_interaction: interaction },
    continuation,
    capability_route: { capacity_assessment: { selected_capability: "free_state", capacity_level: "within_free_state" } },
    transitions: [{ revision: 4, event: "diagnostic_completed", to: "diagnostic_complete", summary: "历史诊断", evidence_refs: ["old-evidence"], project_revision: "revision-4", stale_for_current_revision: true, occurred_at: "2026-08-21T10:00:00Z" }]
  });
}

describe("task runtime trajectory GUI", () => {
  it("shows automatic continuation as scheduler-owned work with stable task identity", () => {
    const markup = renderToStaticMarkup(<TaskTrajectoryView snapshot={runtimeSnapshot("observation_in_progress", { continuation_id: "cont-1", status: "pending" })} />);
    expect(markup).toContain("检查一下当前工程有什么问题？");
    expect(markup).toContain("已排入自动续跑");
    expect(markup).toContain("同一 Task、Run 与原始意图继续");
    expect(markup).toContain("Run 的 invocation 切片");
    expect(markup).toContain("保留在自由态");
    expect(markup).not.toContain("继续执行");
    expect(markup).not.toContain("原始 reasoning");
  });

  it("separates a human wait from terminal no-candidate and capability-blocked states", () => {
    const waiting = renderToStaticMarkup(<TaskTrajectoryView snapshot={runtimeSnapshot("human_judgment_required", {}, { interaction_id: "interaction-1", kind: "human_judgment", reason: "需要听感判断" })} />);
    expect(waiting).toContain("等待你的交互");
    expect(waiting).toContain("需要听感判断");

    const noCandidate = renderToStaticMarkup(<TaskTrajectoryView snapshot={runtimeSnapshot("no_candidate_found")} />);
    expect(noCandidate).toContain("未发现候选项");
    expect(noCandidate).not.toContain("任务已完成");

    const blocked = renderToStaticMarkup(<TaskTrajectoryView snapshot={runtimeSnapshot("capability_blocked")} />);
    expect(blocked).toContain("能力受限");
    expect(blocked).not.toContain("任务已完成");
  });
});
