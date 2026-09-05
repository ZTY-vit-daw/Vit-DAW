import { describe, expect, it } from "vitest";
import { chatMessageFromAgentEvent } from "./App";
import { chainResultMessagesFromEvents, shouldRenderTraceBlockForTurn } from "./trace/traceDelivery";
import type { AgentEvent } from "./types";
import type { TrajectoryState, TrajectoryTurn, TrajectoryNode } from "./trajectory";

function chainResultEvent(extras: Partial<AgentEvent> = {}): AgentEvent {
  return {
    seq: 14,
    type: "turn.completed",
    goal_id: "goal-f7",
    run_id: "run-f7",
    item_id: "chain_result",
    status: "completed",
    body: "在已声明的观察范围内没有发现可信改善候选。",
    payload: { scheduler_chain: true, stop_reason: "no_candidate_found" },
    ...extras
  } as AgentEvent;
}

// GUI-F7（2026-09-05 手测实锤）：调度链纯后台收尾的终局回复只经事件路径交付，
// 但 reduceAgentEventActivities 对 turn.completed 只清场不产消息——F6 的合成
// 分支集成不可达。修复：轮询器把 scheduler_chain 终局事件的合成消息直接入
// messages（本模块的提取 helper），不经活动列表（活动会被回合清退）。
describe("chain result messages from events (GUI-F7)", () => {
  const extract = (events: AgentEvent[]) => chainResultMessagesFromEvents(events, "default", chatMessageFromAgentEvent);

  it("scheduler_chain turn.completed 提取为终局消息", () => {
    const messages = extract([chainResultEvent()]);
    expect(messages).toHaveLength(1);
    expect(messages[0]?.role).toBe("assistant");
    expect(messages[0]?.content).toContain("没有发现可信改善候选");
    expect(messages[0]?.id).toBe("agent_event_goal-f7_chain_result");
  });

  it("scheduler_chain turn.failed 提取为系统错误消息", () => {
    const messages = extract([chainResultEvent({
      type: "turn.failed",
      status: "failed",
      body: "chain failed at settle"
    })]);
    expect(messages).toHaveLength(1);
    expect(messages[0]?.role).toBe("system");
    expect(messages[0]?.status).toBe("error");
  });

  it("同 goal 重复事件只保留一条（去重）", () => {
    const messages = extract([
      chainResultEvent({ seq: 14 }),
      chainResultEvent({ seq: 20, body: "重复投递的终局事件" })
    ]);
    expect(messages).toHaveLength(1);
    expect(messages[0]?.content).toContain("没有发现可信改善候选");
  });

  it("HTTP 路径（无 scheduler_chain）与空 body 不产消息", () => {
    expect(extract([chainResultEvent({ payload: {} })])).toHaveLength(0);
    expect(extract([chainResultEvent({ body: "" })])).toHaveLength(0);
  });
});

// 空结算块隐藏（GUI-F7 / 原 A 方案）：0 轨迹节点且终态 completed 的回合不再
// 渲染独立轨迹块（"0 步·执行完成"误导；终局结果由消息气泡承载）。failed/
// stopped 与 live 回合的块保留——异常与进行中信息优先可见。
describe("shouldRenderTraceBlockForTurn (GUI-F7)", () => {
  const state = (turn: TrajectoryTurn): TrajectoryState => {
    const nodes: Record<string, TrajectoryNode> = {};
    for (const id of turn.nodeIds ?? []) {
      nodes[id] = { id, kind: "tool", title: "", summary: "", status: "completed" } as TrajectoryNode;
    }
    return {
      turns: { [turn.id]: turn },
      nodes
    } as TrajectoryState;
  };
  const turn = (status: string, nodeIds: string[] = []): TrajectoryTurn =>
    ({ id: "turn-x", status, nodeIds } as TrajectoryTurn);

  it("0 节点 completed 回合不渲染块", () => {
    expect(shouldRenderTraceBlockForTurn(state(turn("completed")), "turn-x")).toBe(false);
  });
  it("0 节点 running 回合保留块（进行中指示）", () => {
    expect(shouldRenderTraceBlockForTurn(state(turn("running")), "turn-x")).toBe(true);
  });
  it("0 节点 failed/stopped 回合保留块（异常可见优先）", () => {
    expect(shouldRenderTraceBlockForTurn(state(turn("failed")), "turn-x")).toBe(true);
    expect(shouldRenderTraceBlockForTurn(state(turn("stopped")), "turn-x")).toBe(true);
  });
  it("有节点的 completed 回合照常渲染", () => {
    expect(shouldRenderTraceBlockForTurn(state(turn("completed", ["n1"])), "turn-x")).toBe(true);
  });
});
