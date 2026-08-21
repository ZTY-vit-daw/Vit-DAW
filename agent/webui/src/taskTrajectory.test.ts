import { describe, expect, it } from "vitest";
import { emptyTaskTrajectoryState, normalizeTaskTrajectory, reduceTaskTrajectory } from "./taskTrajectory";

function snapshot(revision: number, updatedAt = "2026-08-21T10:00:00Z", projectRevision = "project-r1") {
  return {
    schema_version: "vit.task_runtime_trajectory.v1",
    task: { task_id: "task-1", goal_id: "goal-1", run_id: "run-1", original_intent: "检查当前工程", status: "active", updated_at: updatedAt },
    run: { run_id: "run-1", slices: [], turns: [] },
    semantic: { state: "observation_in_progress", revision, project_revision: projectRevision },
    transitions: []
  };
}

describe("durable task trajectory projection", () => {
  it("accepts only the versioned task projection with stable identity", () => {
    expect(normalizeTaskTrajectory(snapshot(2))?.identity).toBe("task-1:goal-1:run-1");
    expect(normalizeTaskTrajectory({ ...snapshot(2), schema_version: "vit.task_runtime_trajectory.v0" })).toBeNull();
  });

  it("does not regress to an older semantic revision after replay or restart", () => {
    const current = reduceTaskTrajectory(emptyTaskTrajectoryState(), snapshot(3, "2026-08-21T10:03:00Z"));
    const replayed = reduceTaskTrajectory(current, snapshot(2, "2026-08-21T10:04:00Z"));
    expect(replayed.snapshot?.semanticRevision).toBe(3);
    expect(replayed.snapshot?.task.updated_at).toBe("2026-08-21T10:03:00Z");
  });

  it("accepts newer scheduler/runtime state without duplicating the task", () => {
    const initial = reduceTaskTrajectory(emptyTaskTrajectoryState(), snapshot(3, "2026-08-21T10:03:00Z"));
    const refreshed = reduceTaskTrajectory(initial, { ...snapshot(3, "2026-08-21T10:05:00Z"), continuation: { status: "pending", continuation_id: "cont-1" } });
    expect(refreshed.snapshot?.identity).toBe("task-1:goal-1:run-1");
    expect(refreshed.snapshot?.continuation.status).toBe("pending");
  });
});
