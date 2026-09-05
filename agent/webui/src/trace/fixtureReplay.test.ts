import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { emptyTrajectoryState, reduceTrajectoryEvents, trajectoryTurns } from "../trajectory";
import { reduceAgentEventActivities, eventTurnID } from "../messageLifecycle";
import { chatMessageFromAgentEvent } from "../App";
import { chainResultMessagesFromEvents, shouldRenderTraceBlockForTurn } from "./traceDelivery";
import { buildMessageStreamRenderPlan } from "./renderPlan";
import { reduceTurnEventMeta } from "./turnEventMeta";
import type { AgentEvent, ChatMessage } from "../types";

const fixturesDir = new URL("./__fixtures__/", import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1");

function loadFixture(name: string): AgentEvent[] {
  const raw = JSON.parse(readFileSync(fixturesDir + name, "utf-8")) as { events?: AgentEvent[] };
  return raw.events ?? [];
}

function chat(partial: Partial<ChatMessage> & Pick<ChatMessage, "id" | "role" | "content">): ChatMessage {
  return { createdAt: 0, ...partial } as ChatMessage;
}

const activityFactory = (event: AgentEvent) => chatMessageFromAgentEvent(event, "default");

// CONTRACT-1 C0 双写后的新流形态：fixture 旧事件 + 双写字段注入（仿真冒烟链实测形态）
function withContract1DualWrite(events: AgentEvent[]): AgentEvent[] {
  return events.map((event) => ({
    ...event,
    trajectory_turn_id: event.turn_id ?? event.run_id,
    source_turn_id: event.run_id ?? event.goal_id,
    payload: event.payload && (event.payload as Record<string, unknown>).scheduler_chain
      ? { ...event.payload, turn_kind: "settle_slice" }
      : event.payload
  }));
}

// 2026-09-05 M12 取证复现流（webui_mto15xxx）：item-only 观察轮 + 链收尾切片。
// 取证定案：完成前后块都渲染过、终局消息正常送达，唯回合结束后整个轨迹块消失
// ——F8 谓词把「只有 turn 生命周期节点」的 item-only 回合同判为结算切片隐藏。
describe("fixtures 组合回放：mto15xxx 15 事件（M12 复现形态）", () => {
  const events = loadFixture("2026-09-05-webui-mto15xxx-15events.json");
  const runId = "run_454d82f03c445042";

  it("事件流形态与取证记录一致：item-only 观察轮（5 个 item、无实验节点）", () => {
    const itemTurnIds = new Set(events.filter((event) => event.type.startsWith("item.")).map(eventTurnID));
    expect(itemTurnIds.has(runId)).toBe(true);
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), events);
    const turn = state.turns[runId];
    expect(turn).toBeDefined();
    // 唯一轨迹节点是 turn 生命周期节点（取证：item 从不投影为轨迹节点）
    const stepNodes = turn.nodeIds.filter((id) => state.nodes[id]?.kind !== "turn");
    expect(stepNodes).toHaveLength(0);
  });

  it("M12 回放：item 活动足迹记账后，完成的观察轮轨迹块保留渲染（无消失复现）", () => {
    const meta = reduceTurnEventMeta({}, events);
    expect(meta[runId]?.itemActivityCount).toBe(5);
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), events);
    expect(shouldRenderTraceBlockForTurn(state, runId, meta)).toBe(true);
  });

  it("M12 回放（C0 新流形态）：双写+settle_slice 注入后观察轮同样不隐藏", () => {
    const meta = reduceTurnEventMeta({}, withContract1DualWrite(events));
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), withContract1DualWrite(events));
    // 链终局与主回合共享 run id：settle_slice 标记不得压过 item 活动证据
    expect(meta[runId]?.turnKind).toBe("settle_slice");
    expect(shouldRenderTraceBlockForTurn(state, runId, meta)).toBe(true);
  });

  it("终局链路：链收尾切片合成一条终局消息，活动归约后回合内活动被清退", () => {
    const chainMessages = chainResultMessagesFromEvents(events, "default", chatMessageFromAgentEvent);
    expect(chainMessages).toHaveLength(1);
    expect(chainMessages[0]?.content).toContain("没有发现可信改善候选");
    const activities = reduceAgentEventActivities([], events, activityFactory);
    // turn.completed 后活动清场（item.started/completed 均已 dismiss）
    expect(activities.filter((activity) => (activity.turn_id ?? "").trim() === runId)).toHaveLength(0);
  });

  it("渲染计划回放：用户消息+中间汇报 → 轨迹块 → 终局回复，全程不丢轨迹条目", () => {
    const meta = reduceTurnEventMeta({}, events);
    const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), events);
    const messages = [
      chat({ id: "u1", role: "user", content: "检查一下当前工程有什么问题", turn_id: runId }),
      chat({ id: "a1", role: "assistant", content: "我还在继续处理这个任务，完成后再向你汇报。", turn_id: runId })
    ];
    const plan = buildMessageStreamRenderPlan({
      messages: [...messages, ...chainResultMessagesFromEvents(events, "default", chatMessageFromAgentEvent)],
      trajectory,
      turnEventMeta: meta
    });
    const shape = plan.entries.map((entry) => entry.kind === "trace" ? `trace:${entry.turnId}` : `messages:${entry.messages.map((message) => message.id).join(",")}`);
    expect(shape).toEqual(["messages:u1", `trace:${runId}`, "messages:a1"]);
    expect(plan.chainResultMessages).toHaveLength(1);
  });
});

// 2026-09-05 实验流（webui_mtny2v9x）：run 级 turn 与实验 turn:free_state_* 双域并存。
describe("fixtures 组合回放：mtny2v9x 33 事件（实验流）", () => {
  const events = loadFixture("2026-09-05-webui-mtny2v9x-33events.json");

  it("双域并存形态：run 级与实验级 turn 都进轨迹状态", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), events);
    const turnIds = trajectoryTurns(state).map((turn) => turn.id);
    expect(turnIds.some((id) => id.startsWith("run_"))).toBe(true);
    expect(turnIds.some((id) => id.startsWith("turn:free_state_") || id.startsWith("turn:"))).toBe(true);
  });

  it("实验轮节点完整保留（intent/observation/hypothesis 等），回合完成后照常渲染", () => {
    const meta = reduceTurnEventMeta({}, events);
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), events);
    const experimentTurn = trajectoryTurns(state).find((turn) => turn.id.includes("free_state"));
    expect(experimentTurn).toBeDefined();
    expect(experimentTurn!.nodeIds.length).toBeGreaterThan(1);
    expect(shouldRenderTraceBlockForTurn(state, experimentTurn!.id, meta)).toBe(true);
  });

  it("C0 新流形态回放全绿：双写字段注入不改变归约结果", () => {
    const dual = withContract1DualWrite(events);
    const before = reduceTrajectoryEvents(emptyTrajectoryState(), events);
    const after = reduceTrajectoryEvents(emptyTrajectoryState(), dual);
    expect(trajectoryTurns(after).map((turn) => turn.id)).toEqual(trajectoryTurns(before).map((turn) => turn.id));
    expect(Object.keys(after.nodes)).toEqual(Object.keys(before.nodes));
  });
});

// 旧事件流回退谓词（GUI-1 RED ②）：三条件缺一不隐藏。
describe("回退谓词三条件（无 settle_slice 标记的旧事件流）", () => {
  const turnOnlyEvents = (seq: number, type: string, at: string): AgentEvent[] => [
    { seq, type, run_id: "run_x", turn_id: "run_x", item_id: `turn:run_x`, created_at: at, payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "turn", turn_id: "run_x", trace_node_id: "turn:run_x", status: type.endsWith("completed") ? "completed" : "running" } } as AgentEvent
  ];

  function stateFor(events: AgentEvent[]) {
    return reduceTrajectoryEvents(emptyTrajectoryState(), events);
  }

  it("0 步 + 无活动 + 128ms 生命周期 → 隐藏（真结算切片特征，F8 原意保留）", () => {
    const events = [...turnOnlyEvents(1, "trajectory.turn.started", "2026-09-05T15:00:00.000Z"), ...turnOnlyEvents(2, "trajectory.turn.completed", "2026-09-05T15:00:00.128Z")];
    const meta = reduceTurnEventMeta({}, events);
    expect(shouldRenderTraceBlockForTurn(stateFor(events), "run_x", meta)).toBe(false);
  });

  it("缺条件②（有 item 活动记录）→ 渲染", () => {
    const events = [
      ...turnOnlyEvents(1, "trajectory.turn.started", "2026-09-05T15:00:00.000Z"),
      { seq: 3, type: "item.started", run_id: "run_x", turn_id: "run_x", item_id: "t1" } as AgentEvent,
      ...turnOnlyEvents(2, "trajectory.turn.completed", "2026-09-05T15:00:00.128Z")
    ];
    const meta = reduceTurnEventMeta({}, events);
    expect(shouldRenderTraceBlockForTurn(stateFor(events), "run_x", meta)).toBe(true);
  });

  it("缺条件③（生命周期 3s 不短）→ 渲染", () => {
    const events = [...turnOnlyEvents(1, "trajectory.turn.started", "2026-09-05T15:00:00.000Z"), ...turnOnlyEvents(2, "trajectory.turn.completed", "2026-09-05T15:00:03.000Z")];
    const meta = reduceTurnEventMeta({}, events);
    expect(shouldRenderTraceBlockForTurn(stateFor(events), "run_x", meta)).toBe(true);
  });

  it("证据不足（无 meta）→ 保守渲染", () => {
    const events = [...turnOnlyEvents(1, "trajectory.turn.started", "2026-09-05T15:00:00.000Z"), ...turnOnlyEvents(2, "trajectory.turn.completed", "2026-09-05T15:00:00.128Z")];
    expect(shouldRenderTraceBlockForTurn(stateFor(events), "run_x")).toBe(true);
    expect(shouldRenderTraceBlockForTurn(stateFor(events), "run_x", {})).toBe(true);
  });

  it("新流 settle_slice 标记替代生命周期条件：0 步 + 无活动 + 标记 → 隐藏（不看寿命）", () => {
    const events = [
      ...turnOnlyEvents(1, "trajectory.turn.started", "2026-09-05T15:00:00.000Z"),
      ...turnOnlyEvents(2, "trajectory.turn.completed", "2026-09-05T15:00:05.000Z"),
      { seq: 3, type: "turn.completed", run_id: "run_x", turn_id: "run_x", item_id: "chain_result", payload: { scheduler_chain: true, turn_kind: "settle_slice" } } as AgentEvent
    ];
    const meta = reduceTurnEventMeta({}, events);
    expect(meta["run_x"]?.turnKind).toBe("settle_slice");
    expect(shouldRenderTraceBlockForTurn(stateFor(events), "run_x", meta)).toBe(false);
  });
});
