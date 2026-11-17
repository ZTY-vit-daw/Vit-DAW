import { describe, expect, it } from "vitest";
import { emptyWindowPresentation, parseLaunchProgress, parseWindowObservation, reduceWindowPresentation, isWindowLaunchBusy, type LaunchProgress } from "./windowPresentation";

const message = (extra: Record<string, unknown> = {}) => ({ type: "vit_launch_progress", schema: "vitshell.launch_progress.v1", request_id: "current", seq: 1, phase: "spawning", detail: "", ...extra });
const progress = (seq: number, phase: LaunchProgress["phase"], extra: Partial<LaunchProgress> = {}) => ({ type: "progress" as const, replay: false, progress: { requestId: "current", seq, phase, detail: "", ...extra } });
const begin = () => reduceWindowPresentation(emptyWindowPresentation(), { type: "begin", requestId: "current", title: "fixture project" });

describe("395 native presentation contract", () => {
  it("requires known schema, phase, positive safe sequence and request", () => {
    expect(parseLaunchProgress(message())).toMatchObject({ requestId: "current", seq: 1, phase: "spawning" });
    for (const bad of [null, [], message({ schema: "future" }), message({ phase: "restored_tracks" }), message({ request_id: " " }), message({ seq: 0 }), message({ seq: Number.MAX_SAFE_INTEGER + 1 }), message({ detail: null })]) {
      expect(parseLaunchProgress(bad)).toBeNull();
    }
    expect(parseLaunchProgress(message({ stage: "unknown" }))?.stage).toBeUndefined();
  });
  it("keeps missing windows unknown and orders observations without interpreting release reasons", () => {
    const native = { schema: "vitshell.window_state.v1", foreground: "daw", daw_hwnd: "42", observed_at_tick_ms: 20, release_reason: "workspace_ready" };
    expect(parseWindowObservation(native)?.foreground).toBe("daw");
    expect(parseWindowObservation({ ...native, daw_hwnd: null })).toBeNull();
    expect(parseWindowObservation({ ...native, schema: "future" })).toBeNull();
    let state = reduceWindowPresentation(begin(), { type: "window", observation: parseWindowObservation(native) });
    expect(state.terminal).toBe("");
    state = reduceWindowPresentation(state, { type: "window", observation: parseWindowObservation({ ...native, foreground: "other", observed_at_tick_ms: 10 }) });
    expect(state.window?.foreground).toBe("daw");
    state = reduceWindowPresentation(state, { type: "window", observation: null });
    expect(state.window).toBeNull();
  });
  it("shows only observed phases with unknown progress and at most three steps", () => {
    let state = begin();
    for (const [index, phase] of (["waiting_governance", "saving", "closing_daw", "spawning", "waiting_launch_result"] as const).entries()) {
      state = reduceWindowPresentation(state, progress(index + 1, phase));
      expect(state.view.progress).toBeNull();
      expect(state.view.steps.length).toBeLessThanOrEqual(3);
    }
    expect(state.view.steps.map(s => s.label)).toEqual(["正在关闭原工作台", "正在拉起工作台", "正在等待工作台装载回执"]);
    expect(isWindowLaunchBusy(state)).toBe(true);
  });
  it("ignores duplicate, older, unrelated and unsolicited events", () => {
    const state = reduceWindowPresentation(begin(), progress(4, "saving"));
    expect(reduceWindowPresentation(state, progress(4, "saving"))).toBe(state);
    expect(reduceWindowPresentation(state, progress(3, "spawning"))).toBe(state);
    expect(reduceWindowPresentation(state, progress(99, "opened", { requestId: "previous", stage: "workspace_ready" }))).toBe(state);
    expect(reduceWindowPresentation(emptyWindowPresentation(), progress(1, "saving"))).toEqual(emptyWindowPresentation());
  });
  it("restores only a replayed request and does not turn replay into another step", () => {
    const event = { ...progress(7, "waiting_launch_result"), replay: true };
    const state = reduceWindowPresentation(emptyWindowPresentation(), event);
    expect(state.requestId).toBe("current");
    expect(state.view.steps).toHaveLength(1);
    expect(reduceWindowPresentation(state, event)).toBe(state);
  });
  it("requires explicit workspace_ready even when legacy opened arrives first", () => {
    let state = reduceWindowPresentation(begin(), { type: "result", requestId: "current", status: "opened", message: "opened" });
    expect(state.terminal).toBe("unconfirmed");
    expect(state.view.phase).toBe("loading");
    state = reduceWindowPresentation(state, progress(1, "opened", { stage: "workspace_ready" }));
    expect(state.terminal).toBe("ready");
    expect(state.view.phase).toBe("completed");
    expect(state.view.progress).toBeNull();
    expect(state.view.steps.every(s => s.status === "done")).toBe(true);
    expect(isWindowLaunchBusy(state)).toBe(false);
  });
  it("never upgrades scene_accepted or unknown stage into completed", () => {
    for (const stage of ["scene_accepted", undefined] as const) {
      const state = reduceWindowPresentation(begin(), progress(1, "opened", { stage }));
      expect(state.terminal).toBe(stage ? "accepted" : "unconfirmed");
      expect(state.view.phase).not.toBe("completed");
      expect(isWindowLaunchBusy(state)).toBe(false);
    }
  });
  it("failure and cancel are terminal; unrelated result or late success cannot replace them", () => {
    const state = begin();
    expect(reduceWindowPresentation(state, { type: "result", requestId: "old", status: "failed", message: "old" })).toBe(state);
    const failed = reduceWindowPresentation(state, progress(1, "failed", { detail: "snapshot rejected" }));
    expect(failed.view.failureReason).toBe("snapshot rejected");
    expect(reduceWindowPresentation(failed, progress(2, "opened", { stage: "workspace_ready" })).terminal).toBe("failed");
    const canceled = reduceWindowPresentation(state, progress(1, "canceled"));
    expect(canceled.view.phase).toBe("interrupted");
    expect(isWindowLaunchBusy(canceled)).toBe(false);
  });
  it("next explicit request starts fresh steps but retains instance sequence ordering", () => {
    const old = reduceWindowPresentation(begin(), progress(3, "opened", { stage: "workspace_ready" }));
    const next = reduceWindowPresentation(old, { type: "begin", requestId: "next", title: "next fixture" });
    expect(next.lastSeq).toBe(3);
    expect(next.terminal).toBe("");
    expect(next.view.steps).toHaveLength(1);
    expect(isWindowLaunchBusy(next)).toBe(true);
  });
});
