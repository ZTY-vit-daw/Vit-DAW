import { describe, expect, it } from "vitest";
import { agentEventPollBusy, createAgentEventPollIdleGate } from "./eventPolling";
import { continuationChainLive, isAgentTurnRunning } from "./turnControl";

// GUI-F5（2026-09-04 手测铁证，会话 webui_mtmwwfax）：链终片结算回合静默 2m13s，
// 轮询器 2 秒空闲即休眠，scheduler_chain 终局回复落在服务端无人来取。忙态判定
// 必须纳入 agentTurnRunning（GUI-F3 的 continuationChainLive 已算好），链活期
// 空轮询不停止；回合终态后仍按 4 拍休眠，不无限轮询。
const idleFlags = { isSending: false, respondingActionID: "", auditionWaiting: false };

describe("事件轮询空闲门（GUI-F5）", () => {
  it("链活期（waiting_continue+活 continuation）空轮询不停止", () => {
    const agentTurnRunning =
      isAgentTurnRunning("waiting_continue") ||
      continuationChainLive([{ status: "running" }], "waiting_continue");
    expect(agentTurnRunning).toBe(true);
    expect(agentEventPollBusy({ ...idleFlags, agentTurnRunning })).toBe(true);
    const gate = createAgentEventPollIdleGate();
    for (let tick = 0; tick < 20; tick += 1) {
      expect(gate.tickIdle(true)).toBe(false);
      expect(gate.idleTicks).toBe(0);
    }
  });

  it("回合终态后仍按 4 拍休眠（不无限轮询）", () => {
    const agentTurnRunning =
      isAgentTurnRunning("completed") ||
      continuationChainLive([{ status: "completed" }], "completed");
    expect(agentTurnRunning).toBe(false);
    expect(agentEventPollBusy({ ...idleFlags, agentTurnRunning })).toBe(false);
    const gate = createAgentEventPollIdleGate();
    for (let tick = 1; tick <= 3; tick += 1) {
      expect(gate.tickIdle(false)).toBe(false);
    }
    expect(gate.tickIdle(false)).toBe(true);
  });

  it("事件到达清零空闲拍，休眠计数重新起算", () => {
    const gate = createAgentEventPollIdleGate();
    gate.tickIdle(false);
    gate.tickIdle(false);
    gate.markActive();
    expect(gate.idleTicks).toBe(0);
    for (let tick = 1; tick <= 3; tick += 1) {
      expect(gate.tickIdle(false)).toBe(false);
    }
    expect(gate.tickIdle(false)).toBe(true);
  });

  it("忙态覆盖其余三路：isSending/respondingActionID/auditionWaiting 任一为真不休眠", () => {
    expect(agentEventPollBusy({ ...idleFlags, isSending: true })).toBe(true);
    expect(agentEventPollBusy({ ...idleFlags, respondingActionID: "action-1" })).toBe(true);
    expect(agentEventPollBusy({ ...idleFlags, auditionWaiting: true })).toBe(true);
    expect(agentEventPollBusy(idleFlags)).toBe(false);
  });

  it("连续失败按 3 拍休眠，忙态下失败不累积", () => {
    const gate = createAgentEventPollIdleGate();
    expect(gate.tickError(true)).toBe(false);
    expect(gate.idleTicks).toBe(0);
    expect(gate.tickError(false)).toBe(false);
    expect(gate.tickError(false)).toBe(false);
    expect(gate.tickError(false)).toBe(true);
  });
});
