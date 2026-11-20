// SCORE-GUI-FOLLOW-1 (398)：跟随状态机纯单元用例（边沿语义/防抢回/busy 延迟/跨工程清簿记）。
import { describe, expect, it } from "vitest";
import { FOLLOW_CONFIRM_POLLS, guiSelectedClipID, guiSelectedTrackID, initialFollowState, markSelect, nextFollow } from "./selection";
import { followTargetFor, scoreCatalog } from "./binding";
import type { AgentUIState } from "../types";

const fixture = (path = "P:/first.vit", extra: Partial<AgentUIState> = {}): AgentUIState => ({
  project: { project_path: path },
  tracks: [
    { id: "a", name: "甲", clips: [{ id: "a1", name: "主题甲", clip_type: "midi" }] },
    { id: "b", name: "乙", clips: [{ id: "b1", name: "主题乙", clip_type: "midi" }, { id: "b2", name: "尾段乙", clip_type: "midi" }, { id: "w", clip_type: "wave" }] }
  ],
  ...extra
});

const input = (over: Partial<Parameters<typeof nextFollow>[1]> = {}) => ({
  projectKey: "P:/first.vit",
  guiTrackID: "",
  guiClipID: "",
  choiceProjectKey: "",
  choiceTrackID: "",
  busy: false,
  ...over
});

describe("398 GUI selection readers", () => {
  it("prefers the resolved selected_track row and falls back to ui_context read order", () => {
    expect(guiSelectedTrackID(fixture())).toBe("");
    expect(guiSelectedTrackID(fixture("P:/first.vit", { selected_track: { track_id: "a" } }))).toBe("a");
    expect(guiSelectedTrackID(fixture("P:/first.vit", { selected_track: { id: "row-id" } }))).toBe("row-id");
    expect(guiSelectedTrackID(fixture("P:/first.vit", { selected_track: null, ui_context: { selected_track_id: "b" } }))).toBe("b");
    expect(guiSelectedClipID(fixture("P:/first.vit", { ui_context: { selected_clip_id: "b1" } }))).toBe("b1");
    expect(guiSelectedClipID(fixture())).toBe("");
  });
});

describe("398 follow state machine", () => {
  it("baselines the first poll of a project without following (mount and cross-project)", () => {
    const first = nextFollow(initialFollowState(), input({ guiTrackID: "a", choiceProjectKey: "P:/first.vit", choiceTrackID: "b" }));
    expect(first.decision).toEqual({ kind: "none" });
    expect(first.state).toEqual({ projectKey: "P:/first.vit", lastSeenTrackID: "a", awaitConfirm: "", awaitProjectKey: "", awaitPolls: 0 });
    const cross = nextFollow(first.state, input({ projectKey: "P:/second.vit", guiTrackID: "a", choiceProjectKey: "P:/first.vit", choiceTrackID: "b" }));
    expect(cross.decision).toEqual({ kind: "none" });
    expect(cross.state.lastSeenTrackID).toBe("a");
    expect(cross.state.projectKey).toBe("P:/second.vit");
  });

  it("follows a GUI edge exactly once and never re-pushes the same target (poller cannot grab back)", () => {
    const base = nextFollow(initialFollowState("P:/first.vit"), input({ guiTrackID: "a" })).state;
    const edge = nextFollow(base, input({ guiTrackID: "b", guiClipID: "b1", choiceProjectKey: "P:/first.vit", choiceTrackID: "a" }));
    expect(edge.decision).toEqual({ kind: "follow", trackID: "b", clipID: "b1" });
    for (let i = 0; i < 3; i++) {
      const repeat = nextFollow(edge.state, input({ guiTrackID: "b", guiClipID: "b1", choiceProjectKey: "P:/first.vit", choiceTrackID: "a" }));
      expect(repeat.decision).toEqual({ kind: "none" });
    }
  });

  it("defers while busy without consuming the edge, then follows after busy ends", () => {
    const base = nextFollow(initialFollowState("P:/first.vit"), input({ guiTrackID: "a" })).state;
    const deferred = nextFollow(base, input({ guiTrackID: "b", choiceProjectKey: "P:/first.vit", choiceTrackID: "a", busy: true }));
    expect(deferred.decision).toEqual({ kind: "defer" });
    expect(deferred.state.lastSeenTrackID).toBe("a");
    const after = nextFollow(deferred.state, input({ guiTrackID: "b", choiceProjectKey: "P:/first.vit", choiceTrackID: "a", busy: false }));
    expect(after.decision).toEqual({ kind: "follow", trackID: "b", clipID: "" });
  });

  it("keeps the last edge reference across a transient empty GUI disclosure", () => {
    const base = nextFollow(initialFollowState("P:/first.vit"), input({ guiTrackID: "a" })).state;
    const empty = nextFollow(base, input({ guiTrackID: "" }));
    expect(empty.decision).toEqual({ kind: "none" });
    expect(empty.state.lastSeenTrackID).toBe("a");
    const again = nextFollow(empty.state, input({ guiTrackID: "a" }));
    expect(again.decision).toEqual({ kind: "none" });
    const moved = nextFollow(empty.state, input({ guiTrackID: "b", choiceProjectKey: "P:/first.vit", choiceTrackID: "a" }));
    expect(moved.decision).toEqual({ kind: "follow", trackID: "b", clipID: "" });
  });

  it("confirms when the GUI echoes our own select and when the GUI lands on the held choice", () => {
    const selected = markSelect(initialFollowState("P:/first.vit"), "b", "P:/first.vit");
    const echo = nextFollow(selected, input({ guiTrackID: "b", choiceProjectKey: "P:/first.vit", choiceTrackID: "b" }));
    expect(echo.decision).toEqual({ kind: "confirm" });
    expect(echo.state.awaitConfirm).toBe("");
    const base = nextFollow(initialFollowState("P:/first.vit"), input({ guiTrackID: "a" })).state;
    const same = nextFollow(base, input({ guiTrackID: "b", choiceProjectKey: "P:/first.vit", choiceTrackID: "b" }));
    expect(same.decision).toEqual({ kind: "confirm" });
  });

  it("never confirms across a project boundary (stale await is inert)", () => {
    const selected = markSelect(initialFollowState("P:/first.vit"), "a", "P:/first.vit");
    const switched = nextFollow(selected, input({ projectKey: "P:/second.vit", guiTrackID: "a", choiceProjectKey: "P:/first.vit", choiceTrackID: "a" }));
    expect(switched.decision).toEqual({ kind: "none" });
    expect(switched.state.awaitConfirm).toBe("");
    const echoedInNewProject = nextFollow(switched.state, input({ projectKey: "P:/second.vit", guiTrackID: "a", choiceProjectKey: "P:/second.vit", choiceTrackID: "a" }));
    expect(echoedInNewProject.decision).toEqual({ kind: "none" });
  });

  it("counts await polls for the bounded unconfirmed notice and clears on confirm", () => {
    const selected = markSelect(initialFollowState("P:/first.vit"), "a", "P:/first.vit");
    let state = selected;
    for (let i = 1; i < FOLLOW_CONFIRM_POLLS; i++) {
      state = nextFollow(state, input({ guiTrackID: "" })).state;
      expect(state.awaitPolls).toBe(i);
    }
    state = nextFollow(state, input({ guiTrackID: "" })).state;
    expect(state.awaitPolls).toBe(FOLLOW_CONFIRM_POLLS);
    const confirmed = nextFollow(state, input({ guiTrackID: "a", choiceProjectKey: "P:/first.vit", choiceTrackID: "a" }));
    expect(confirmed.decision).toEqual({ kind: "confirm" });
    expect(confirmed.state.awaitPolls).toBe(0);
  });

  it("a user-driven edge supersedes a pending await instead of confirming", () => {
    const selected = markSelect(initialFollowState("P:/first.vit"), "b", "P:/first.vit");
    const superseded = nextFollow(selected, input({ guiTrackID: "c", choiceProjectKey: "P:/first.vit", choiceTrackID: "b" }));
    expect(superseded.decision).toEqual({ kind: "follow", trackID: "c", clipID: "" });
    expect(superseded.state.awaitConfirm).toBe("");
  });
});

describe("398 follow target guard", () => {
  it("honors only MIDI hints on the followed track; otherwise single-clip auto or explicit wait", () => {
    const catalog = scoreCatalog(fixture("P:/first.vit"));
    expect(followTargetFor(catalog, "b", "b1")?.clipID).toBe("b1");
    expect(followTargetFor(catalog, "b", "w")).toBeNull(); // audio hint rejected
    expect(followTargetFor(catalog, "b", "missing")).toBeNull();
    expect(followTargetFor(catalog, "missing", "b1")).toBeNull();
    const single = scoreCatalog({ project: { project_path: "P:/first.vit" }, tracks: [{ id: "x", name: "单", clips: [{ id: "x1", name: "唯一", clip_type: "midi" }] }] } as AgentUIState);
    expect(followTargetFor(single, "x", "")?.clipID).toBe("x1");
    expect(followTargetFor(catalog, "b", "")).toBeNull(); // multi-clip waits for explicit pick
  });
});
