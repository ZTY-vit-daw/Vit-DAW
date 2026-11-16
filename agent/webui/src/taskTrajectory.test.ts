import { describe, expect, it } from "vitest";
import { emptyTaskTrajectoryState, normalizeTaskTrajectory, reduceTaskTrajectory, reduceTaskTrajectoryForConversation } from "./taskTrajectory";

function snapshot(revision: number, updatedAt = "2026-08-21T10:00:00Z", projectRevision = "project-r1", conversationID = "") {
  return {
    schema_version: "vit.task_runtime_trajectory.v1",
    task: { task_id: "task-1", goal_id: "goal-1", run_id: "run-1", conversation_id: conversationID, original_intent: "检查当前工程", status: "active", updated_at: updatedAt },
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

// WEBUI-SESSION-SEMANTICS-1：PlanBar 快照按 active session 绑定——全局
// task_trajectory 投影不得把异会话（note 会话/其他 webui 流）的任务渲染到本会话。
describe("conversation-scoped task trajectory binding", () => {
  it("属主会话的快照正常合并（monotonic 语义保持）", () => {
    const merged = reduceTaskTrajectoryForConversation(
      emptyTaskTrajectoryState(),
      snapshot(3, "2026-08-21T10:03:00Z", "project-r1", "webui_main"),
      "webui_main"
    );
    expect(merged.snapshot?.identity).toBe("task-1:goal-1:run-1");
    const replayed = reduceTaskTrajectoryForConversation(
      merged,
      snapshot(2, "2026-08-21T10:09:00Z", "project-r1", "webui_main"),
      "webui_main"
    );
    expect(replayed.snapshot?.semanticRevision).toBe(3);
  });

  it("异会话快照即时清空（note 会话任务不渲染到 webui 流）", () => {
    const live = reduceTaskTrajectoryForConversation(
      emptyTaskTrajectoryState(),
      snapshot(3, "2026-08-21T10:03:00Z", "project-r1", "webui_main"),
      "webui_main"
    );
    expect(live.snapshot).not.toBeNull();
    // note 会话任务成为全局投影 → 当前 webui 会话的 PlanBar 必须清空
    const noteTask = reduceTaskTrajectoryForConversation(
      live,
      snapshot(4, "2026-08-21T10:04:00Z", "project-r1", "note_r-abc"),
      "webui_main"
    );
    expect(noteTask.snapshot).toBeNull();
  });

  it("切到异会话清空、切回属主即重绑（换绑断言）；正在查看的会话有活任务则显示该任务", () => {
    const mainTask = snapshot(3, "2026-08-21T10:03:00Z", "project-r1", "webui_main");
    const noteTask = snapshot(4, "2026-08-21T10:04:00Z", "project-r1", "note_r-abc");
    const live = reduceTaskTrajectoryForConversation(emptyTaskTrajectoryState(), mainTask, "webui_main");
    expect(live.snapshot).not.toBeNull();
    // note 会话任务占据全局投影、当前仍停留在 webui 主流 → 清空（症状 6 的泄漏形态）
    const cleared = reduceTaskTrajectoryForConversation(live, noteTask, "webui_main");
    expect(cleared.snapshot).toBeNull();
    // 切回 webui 主流（全局投影回到属主任务）→ 重绑
    const rebound = reduceTaskTrajectoryForConversation(cleared, mainTask, "webui_main");
    expect(rebound.snapshot?.identity).toBe("task-1:goal-1:run-1");
    // 用户切到 note 流查看：活任务属于 note 流 → 显示 note 流自己的轨迹（重绑语义）
    const noteView = reduceTaskTrajectoryForConversation(cleared, noteTask, "note_r-abc");
    expect(noteView.snapshot).not.toBeNull();
  });

  it("空会话 id（scope 未物化）一律清空；载荷缺位保留现状（不算异会话证据）", () => {
    expect(reduceTaskTrajectoryForConversation(emptyTaskTrajectoryState(), snapshot(3), "").snapshot).toBeNull();
    const live = reduceTaskTrajectoryForConversation(
      emptyTaskTrajectoryState(),
      snapshot(3, "2026-08-21T10:03:00Z", "project-r1", "webui_main"),
      "webui_main"
    );
    expect(reduceTaskTrajectoryForConversation(live, null, "webui_main").snapshot).not.toBeNull();
    expect(reduceTaskTrajectoryForConversation(live, undefined, "webui_main").snapshot).not.toBeNull();
    // 异形载荷（schema 不符）同"缺位"处理：保留，不误清
    expect(reduceTaskTrajectoryForConversation(live, { schema_version: "wrong" }, "webui_main").snapshot).not.toBeNull();
  });
});
