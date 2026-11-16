import { describe, expect, it } from "vitest";
import { chatMessageFromAgentEvent } from "./App";
import type { AgentEvent } from "./types";

function chainEvent(extras: Partial<AgentEvent>): AgentEvent {
  return {
    seq: 1,
    type: "turn.completed",
    goal_id: "goal-f6",
    run_id: "run-f6",
    item_id: "chain_result",
    status: "completed",
    body: "本轮观察结束：未发现值得处理的候选项。",
    payload: { scheduler_chain: true },
    ...extras
  } as AgentEvent;
}

// AGENT-F6（2026-09-04 手测）：调度侧续跑链的终片结果此前被静默丢弃——
// 多轮执行后用户看不到任何结果。修复后终片 turn.completed 带
// scheduler_chain 标记与最终回复，UI 合成助手结果消息；HTTP 路径的
// turn.completed 不产消息（回复已由 HTTP 响应交付，再产会双份）。
describe("scheduler chain result message (GUI-F6)", () => {
  it("scheduler_chain 标记的 turn.completed 合成助手结果消息", () => {
    const message = chatMessageFromAgentEvent(chainEvent({}), "default");
    expect(message).not.toBeNull();
    expect(message?.role).toBe("assistant");
    expect(message?.content).toContain("未发现值得处理的候选项");
    expect(message?.status).toBe("sent");
    expect(message?.id).toBe("agent_event_goal-f6_chain_result");
  });

  it("HTTP 路径的 turn.completed 不产消息（防双份交付）", () => {
    const message = chatMessageFromAgentEvent(chainEvent({ payload: {} }), "default");
    expect(message).toBeNull();
  });

  it("scheduler_chain 的 turn.failed 合成系统错误消息", () => {
    const message = chatMessageFromAgentEvent(chainEvent({
      type: "turn.failed",
      status: "failed",
      body: "chain exploded in a test"
    }), "default");
    expect(message).not.toBeNull();
    expect(message?.role).toBe("system");
    expect(message?.status).toBe("error");
    expect(message?.content).toContain("chain exploded");
  });

  it("空回复的 scheduler_chain 事件不产空消息", () => {
    const message = chatMessageFromAgentEvent(chainEvent({ body: "" }), "default");
    expect(message).toBeNull();
  });
});
