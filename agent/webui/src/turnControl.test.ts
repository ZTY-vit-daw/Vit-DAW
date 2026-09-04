import { describe, expect, it } from "vitest";
import { authorityContext, checkoutBlockedByState, continuationChainLive, isAgentTurnRunning } from "./turnControl";

 describe("turn control", () => {
  it("maps explicit full access into Agent context without implicit action flags", () => {
    expect(authorityContext("full_project_access")).toEqual({ authority_mode: "full_project_access", authority_mode_explicit: true });
  });

  it("shows Stop Turn only for active execution states or an in-flight request", () => {
    for (const status of ["running", "processing", "executing", "cancelling"]) expect(isAgentTurnRunning(status)).toBe(true);
    for (const status of ["stopped", "completed", "failed"]) expect(isAgentTurnRunning(status)).toBe(false);
    expect(isAgentTurnRunning("", true)).toBe(true);
  });

  it("projects checkout guard from authoritative server state", () => {
    expect(checkoutBlockedByState({ checkout_blocked: true }, null)).toBe(true);
    expect(checkoutBlockedByState(null, { checkout_blocked: false })).toBe(false);
  });
});

// GUI-F3：切片制下首条 HTTP 响应只是第 1 片边界（waiting_continue + 自动
// 续跑排队），链在后台继续跑——忙态必须覆盖，否则输入提前解锁、确认卡等待
// 期间权限开关可误点（服务端会拒）。waiting_confirmation 单独不算运行，
// 但配上活 continuation（waiting_interaction park）就是回合仍在进行。
describe("continuation chain liveness (GUI-F3)", () => {
  const liveStatuses = ["pending", "claimed", "running", "waiting_interaction"];
  const deadStatuses = ["completed", "cancelled", "failed", ""];

  it("waiting_continue with a live continuation keeps the turn running", () => {
    for (const status of liveStatuses) {
      expect(continuationChainLive([{ status }], "waiting_continue")).toBe(true);
    }
  });

  it("waiting_confirmation parked at an interaction keeps the turn running", () => {
    expect(continuationChainLive([{ status: "waiting_interaction" }], "waiting_confirmation")).toBe(true);
  });

  it("terminal continuations never keep the turn running", () => {
    for (const status of deadStatuses) {
      expect(continuationChainLive([{ status }, { status: "running" }], "waiting_continue")).toBe(true);
      expect(continuationChainLive([{ status }], "waiting_continue")).toBe(false);
    }
  });

  it("terminal goal status never keeps the turn running even with live-looking rows", () => {
    expect(continuationChainLive([{ status: "running" }], "completed")).toBe(false);
    expect(continuationChainLive([{ status: "pending" }], "idle")).toBe(false);
  });

  it("empty or missing continuation list is not live", () => {
    expect(continuationChainLive([], "waiting_continue")).toBe(false);
    expect(continuationChainLive(null, "running")).toBe(false);
    expect(continuationChainLive(undefined, "waiting_confirmation")).toBe(false);
  });
});
