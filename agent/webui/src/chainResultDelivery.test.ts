import { describe, expect, it } from "vitest";
import { chatMessageFromAgentEvent } from "./App";
import { reduceAgentEventActivities } from "./messageLifecycle";
import {
  chainResultMessagesFromEvents,
  hasChainTerminalDeliveryEvent,
  isChainResultChatMessage,
  shouldRenderTraceBlockForTurn
} from "./trace/traceDelivery";
import { reduceTurnEventMeta } from "./trace/turnEventMeta";
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

// turn.stopped 终局合成（GUI-1/F7 扩展，CONTRACT-1 stopped 终局补齐的消费端）
describe("scheduler_chain turn.stopped 合成（GUI-1）", () => {
  const extract = (events: AgentEvent[]) => chainResultMessagesFromEvents(events, "default", chatMessageFromAgentEvent);

  it("scheduler_chain turn.stopped 提取为系统消息（非 error 态）", () => {
    const messages = extract([chainResultEvent({
      type: "turn.stopped",
      status: "stopped",
      body: "stop_reason_demo",
      payload: { scheduler_chain: true, stop_reason: "stop_reason_demo", turn_kind: "settle_slice" }
    })]);
    expect(messages).toHaveLength(1);
    expect(messages[0]?.role).toBe("system");
    expect(messages[0]?.status).not.toBe("error");
    expect(messages[0]?.content).toContain("stop_reason_demo");
  });

  it("stopped 终局 body 空时 UI 侧兜底文案（双保险，服务端已保底）", () => {
    const messages = extract([chainResultEvent({
      type: "turn.stopped",
      status: "stopped",
      body: "",
      payload: { scheduler_chain: true, turn_kind: "settle_slice" }
    })]);
    expect(messages).toHaveLength(1);
    expect(messages[0]?.content).toContain("已停止");
  });

  it("turn.stopped 与 completed/failed 同样清退回合内活动", () => {
    const started = {
      seq: 1, type: "item.started", goal_id: "goal-f7", run_id: "run-f7", turn_id: "run-f7",
      item_id: "tool_step_1", status: "running", title: "正在执行操作", body: "x"
    } as AgentEvent;
    const stopped = chainResultEvent({ type: "turn.stopped", status: "stopped", body: "回合已停止。" });
    const activities = reduceAgentEventActivities([], [started], (event) => chatMessageFromAgentEvent(event, "default"));
    expect(activities).toHaveLength(1);
    const after = reduceAgentEventActivities(activities, [stopped], (event) => chatMessageFromAgentEvent(event, "default"));
    expect(after).toHaveLength(0);
  });
});

// M12 谓词（GUI-1 修复 F8 过宽隐藏——2026-09-05 取证定案）：
// 隐藏需要「证据」。新事件流消费 turn_kind=settle_slice 标记（叠加 0 步+无活动）；
// 旧事件流回退谓词三条件=无非 turn 节点 ∧ 无 item 活动记录 ∧ 生命周期短于阈值，
// 缺一即渲染；证据不足（无 meta）保守渲染。item-only 观察轮（M12 复现形态）
// 凭活动足迹保住可回看性。
describe("shouldRenderTraceBlockForTurn (GUI-1 / M12)", () => {
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
  const metaFor = (options: { itemActivityCount?: number; turnKind?: string; startedAt?: number; endedAt?: number } = {}) => ({
    "turn-x": {
      turnKind: options.turnKind ?? "",
      itemActivityCount: options.itemActivityCount ?? 0,
      itemActivityKeys: [],
      startedAt: options.startedAt,
      endedAt: options.endedAt
    }
  });

  it("0 节点 completed 回合：证据不足（无 meta）保守渲染——M12 教训，不得凭猜测隐藏", () => {
    expect(shouldRenderTraceBlockForTurn(state(turn("completed")), "turn-x")).toBe(true);
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

  it("回退三条件全满足（0 步+无活动+128ms）→ 隐藏（真结算切片特征）", () => {
    expect(shouldRenderTraceBlockForTurn(
      state(turn("completed")),
      "turn-x",
      metaFor({ startedAt: 0, endedAt: 128 })
    )).toBe(false);
  });
  it("缺条件②：有 item 活动记录 → 渲染（M12 复现形态观察轮）", () => {
    expect(shouldRenderTraceBlockForTurn(
      state(turn("completed")),
      "turn-x",
      metaFor({ itemActivityCount: 5, startedAt: 0, endedAt: 142_000 })
    )).toBe(true);
  });
  it("缺条件③：生命周期 3s 不短 → 渲染", () => {
    expect(shouldRenderTraceBlockForTurn(
      state(turn("completed")),
      "turn-x",
      metaFor({ startedAt: 0, endedAt: 3000 })
    )).toBe(true);
  });
  it("证据残缺：meta 存在但无生命周期时间戳 → 渲染", () => {
    expect(shouldRenderTraceBlockForTurn(
      state(turn("completed")),
      "turn-x",
      metaFor({})
    )).toBe(true);
  });

  it("GUI-F8 原意（修正口径）：只含 turn 生命周期节点 + 三条件满足 → 隐藏", () => {
    const turnOnly = { id: "turn-y", status: "completed", nodeIds: ["t1"] } as TrajectoryTurn;
    const withTurnNode = {
      turns: { "turn-y": turnOnly },
      nodes: { t1: { id: "t1", kind: "turn", title: "", summary: "", status: "completed" } as TrajectoryNode }
    } as unknown as TrajectoryState;
    expect(shouldRenderTraceBlockForTurn(withTurnNode, "turn-y", {
      "turn-y": { turnKind: "", itemActivityCount: 0, itemActivityKeys: [], startedAt: 0, endedAt: 128 }
    })).toBe(false);
    // 同一回合混入一个非 turn 节点则恢复渲染
    const mixed = {
      turns: { "turn-y": { ...turnOnly, nodeIds: ["t1", "n1"] } },
      nodes: {
        t1: { id: "t1", kind: "turn", title: "", summary: "", status: "completed" } as TrajectoryNode,
        n1: { id: "n1", kind: "observation", title: "", summary: "", status: "completed" } as TrajectoryNode
      }
    } as unknown as TrajectoryState;
    expect(shouldRenderTraceBlockForTurn(mixed, "turn-y")).toBe(true);
  });

  it("新流 settle_slice 标记：0 步+无活动+标记 → 隐藏（不看生命周期）", () => {
    expect(shouldRenderTraceBlockForTurn(
      state(turn("completed")),
      "turn-x",
      metaFor({ turnKind: "settle_slice", startedAt: 0, endedAt: 5000 })
    )).toBe(false);
  });
  it("新流 settle_slice 标记不得压过活动证据：标记+有 item 活动 → 渲染（链终局共享 run id 形态）", () => {
    expect(shouldRenderTraceBlockForTurn(
      state(turn("completed")),
      "turn-x",
      metaFor({ turnKind: "settle_slice", itemActivityCount: 5 })
    )).toBe(true);
  });
  it("无标记回退：短寿命但活动未知（meta 缺该 turn）→ 渲染", () => {
    expect(shouldRenderTraceBlockForTurn(
      state(turn("completed", [])),
      "turn-x",
      { "other-turn": { turnKind: "", itemActivityCount: 0, itemActivityKeys: [], startedAt: 0, endedAt: 100 } }
    )).toBe(true);
  });
});

// RED ⑤ 前置：reduceTurnEventMeta 与谓词接线（详见 fixtureReplay.test.ts 组合回放）
describe("turnEventMeta 接线冒烟", () => {
  it("从事件流提取 item 活动足迹供谓词消费", () => {
    const events = [
      { seq: 1, type: "item.started", goal_id: "g", run_id: "run-m", turn_id: "run-m", item_id: "t1", logical_message_id: "agent_item:run-m:t1" },
      { seq: 2, type: "turn.completed", goal_id: "g", run_id: "run-m", turn_id: "run-m", payload: { scheduler_chain: true, turn_kind: "settle_slice" } }
    ] as AgentEvent[];
    const meta = reduceTurnEventMeta({}, events);
    expect(meta["run-m"]?.itemActivityCount).toBe(1);
    expect(meta["run-m"]?.turnKind).toBe("settle_slice");
  });
});

// F3 钉②：终局到达（含迟到路径）忙态数秒清除——链终局事件是"链已收尾"的
// 最早权威信号（消息入流由 GUI-F7 路径承担）；忙态（agentTurnRunning 派生自
// runtime status 的 goal/continuations 投影）要靠立即刷新 runtime status 才能在
// 数秒内退场，否则最长要等 8s 周期轮。谓词只认 scheduler_chain 终局三型：
// HTTP 路径的 turn.completed 已由响应体本身交付并驱动 refreshState，不需再刷。
describe("chain terminal arrival 请求即时 runtime 刷新 (F3)", () => {
  it("scheduler_chain 终局三型任一到达 → true", () => {
    expect(hasChainTerminalDeliveryEvent([chainResultEvent()])).toBe(true);
    expect(hasChainTerminalDeliveryEvent([
      chainResultEvent({ type: "turn.failed", status: "failed", body: "settle failed" })
    ])).toBe(true);
    expect(hasChainTerminalDeliveryEvent([
      chainResultEvent({ type: "turn.stopped", status: "stopped", body: "stop_reason_demo" })
    ])).toBe(true);
  });

  it("HTTP 路径 turn.completed（无 scheduler_chain 标记）→ false", () => {
    expect(hasChainTerminalDeliveryEvent([chainResultEvent({ payload: {} })])).toBe(false);
  });

  it("非终局事件（item/切片边界 ack）→ false（scheduler_chain 标记只在终局投递事件上）", () => {
    const sliceAck = chainResultEvent({ status: "waiting_continue", body: "slice 1 ack", payload: { scheduler_chain: false, turn_kind: "slice_boundary" } });
    const item = { seq: 3, type: "item.completed", goal_id: "g", run_id: "r", item_id: "t1", status: "completed" } as AgentEvent;
    expect(hasChainTerminalDeliveryEvent([sliceAck, item])).toBe(false);
    expect(hasChainTerminalDeliveryEvent([])).toBe(false);
  });
});

describe("chain result message render position (GUI-F8)", () => {
  it("终局消息可被谓词识别（渲染序接线用）", () => {
    const message = chatMessageFromAgentEvent(chainResultEvent(), "default");
    expect(message).not.toBeNull();
    expect(isChainResultChatMessage(message!)).toBe(true);
  });
  it("普通消息不被误判", () => {
    expect(isChainResultChatMessage({ id: "m1", source_id: "m1" } as never)).toBe(false);
  });
});
