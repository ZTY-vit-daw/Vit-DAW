import { describe, expect, it } from "vitest";
import { buildMessageStreamRenderPlan } from "./renderPlan";
import { emptyTrajectoryState, reduceTrajectoryEvents, type TrajectoryState } from "../trajectory";
import { reduceTurnEventMeta, type TurnEventMeta } from "./turnEventMeta";
import type { AgentEvent, ChatMessage } from "../types";

function chat(partial: Partial<ChatMessage> & Pick<ChatMessage, "id" | "role" | "content">): ChatMessage {
  return { createdAt: 0, ...partial } as ChatMessage;
}

function chainResult(id: string, content: string): ChatMessage {
  return chat({ id, role: "assistant", content, source_id: id });
}

function meta(...turns: Array<{ id: string; itemActivityCount?: number }>): Record<string, TurnEventMeta> {
  const out: Record<string, TurnEventMeta> = {};
  for (const turn of turns) {
    out[turn.id] = {
      turnKind: "",
      itemActivityCount: turn.itemActivityCount ?? 0,
      itemActivityKeys: [],
      startedAt: 0,
      endedAt: 1000
    };
  }
  return out;
}

function stateWithTurn(id: string, events: AgentEvent[]): TrajectoryState {
  return reduceTrajectoryEvents(emptyTrajectoryState(), events);
}

describe("buildMessageStreamRenderPlan（GUI-1/G2：固化渲染顺序）", () => {
  it("no_candidate 流：用户消息 → 回合轨迹块 → 中间汇报 → 终局回复（轨迹完成后保留可回看）", () => {
    const messages = [
      chat({ id: "u1", role: "user", content: "检查一下当前工程有什么问题", turn_id: "run_1" }),
      chat({ id: "a1", role: "assistant", content: "我还在继续处理这个任务，完成后再向你汇报。", turn_id: "run_1" })
    ];
    const trajectory = stateWithTurn("run_1", [
      { seq: 1, type: "trajectory.turn.started", run_id: "run_1", turn_id: "run_1", item_id: "turn:run_1", payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "turn", turn_id: "run_1", trace_node_id: "turn:run_1" } },
      { seq: 9, type: "trajectory.turn.completed", run_id: "run_1", turn_id: "run_1", item_id: "turn:run_1", payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "turn", turn_id: "run_1", trace_node_id: "turn:run_1", status: "completed" } }
    ] as AgentEvent[]);
    const chain = chainResult("agent_event_goal_chain_result", "没有发现可信改善候选。");
    const plan = buildMessageStreamRenderPlan({
      messages: [...messages, chain],
      trajectory,
      turnEventMeta: meta({ id: "run_1", itemActivityCount: 5 })
    });
    const shape = plan.entries.map((entry) => entry.kind === "trace" ? `trace:${entry.turnId}` : "messages");
    expect(shape).toEqual(["messages", "trace:run_1", "messages"]);
    expect(plan.entries[0].kind === "messages" && plan.entries[0].messages[0]?.id).toBe("u1");
    expect(plan.entries[2].kind === "messages" && plan.entries[2].messages[0]?.id).toBe("a1");
    // 终局回复在孤儿块之后、且不入组内
    expect(plan.chainResultMessages).toHaveLength(1);
    expect(plan.entries.filter((entry) => entry.kind === "messages").at(-1)?.kind === "messages" &&
      plan.entries.filter((entry) => entry.kind === "messages").at(-1)!.messages[0]?.id).toBe("a1");
  });

  it("实验流：用户消息 → 中间汇报 → 实验轨迹块（孤儿）→ 终局回复", () => {
    const messages = [
      chat({ id: "u1", role: "user", content: "把军鼓往上提", turn_id: "run_1" }),
      chat({ id: "a1", role: "assistant", content: "正在实验。", turn_id: "run_1" }),
      chainResult("agent_event_goal_chain_result", "已提升 1.5dB。")
    ];
    const trajectory = stateWithTurn("turn:fs_1", [
      { seq: 2, type: "trajectory.intent.framed", run_id: "run_1", turn_id: "turn:fs_1", item_id: "n1", payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "intent", turn_id: "turn:fs_1", trace_node_id: "n1", status: "completed" } }
    ] as AgentEvent[]);
    const plan = buildMessageStreamRenderPlan({ messages, trajectory, turnEventMeta: meta({ id: "run_1", itemActivityCount: 2 }) });
    const shape = plan.entries.map((entry) => entry.kind === "trace" ? `trace:${entry.turnId}` : "messages");
    // 组内：用户消息条目 + 中间汇报条目（run_1 无轨迹 turn 记录不入块）；实验块为孤儿条目
    expect(shape).toEqual(["messages", "messages", "trace:turn:fs_1"]);
    expect(plan.orphanTurnIds).toEqual(["turn:fs_1"]);
    expect(plan.chainResultMessages.map((message) => message.id)).toEqual(["agent_event_goal_chain_result"]);
  });

  it("settle_slice 切片回合（0 步、无活动、带标记）不产生轨迹条目，终局回复保留", () => {
    const messages = [
      chat({ id: "u1", role: "user", content: "继续", turn_id: "run_1" }),
      chainResult("agent_event_goal_chain_result", "收尾完成。")
    ];
    const trajectory = stateWithTurn("run_1", [
      { seq: 1, type: "trajectory.turn.started", run_id: "run_1", turn_id: "run_1", item_id: "turn:run_1", payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "turn", turn_id: "run_1", trace_node_id: "turn:run_1" } },
      { seq: 2, type: "trajectory.turn.completed", run_id: "run_1", turn_id: "run_1", item_id: "turn:run_1", payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "turn", turn_id: "run_1", trace_node_id: "turn:run_1", status: "completed" } }
    ] as AgentEvent[]);
    const sliceMeta = meta({ id: "run_1" });
    sliceMeta["run_1"] = { ...sliceMeta["run_1"]!, turnKind: "settle_slice", startedAt: 0, endedAt: 128 };
    const plan = buildMessageStreamRenderPlan({ messages, trajectory, turnEventMeta: sliceMeta });
    expect(plan.entries.map((entry) => entry.kind)).toEqual(["messages"]);
    expect(plan.chainResultMessages).toHaveLength(1);
  });

  it("终局回复消息不进任何回合组（不产生 loose 碎片组）", () => {
    const plan = buildMessageStreamRenderPlan({
      messages: [chainResult("agent_event_goal_chain_result", "终局。")],
      trajectory: emptyTrajectoryState()
    });
    expect(plan.entries).toHaveLength(0);
    expect(plan.chainResultMessages).toHaveLength(1);
  });

  it("纯函数：相同输入产出相同计划（无隐藏状态）", () => {
    const messages = [chat({ id: "u1", role: "user", content: "检查", turn_id: "run_1" })];
    const trajectory = emptyTrajectoryState();
    const a = buildMessageStreamRenderPlan({ messages, trajectory });
    const b = buildMessageStreamRenderPlan({ messages, trajectory });
    expect(a).toEqual(b);
  });
});

describe("renderPlan × fixtures 组合回放前的 meta 接线", () => {
  it("reduceTurnEventMeta 产出的 meta 直接可喂给渲染计划", () => {
    const events: AgentEvent[] = [
      { seq: 1, type: "item.started", run_id: "run_1", turn_id: "run_1", item_id: "t1", logical_message_id: "agent_item:run_1:t1" } as AgentEvent
    ];
    const turnEventMeta = reduceTurnEventMeta({}, events);
    const plan = buildMessageStreamRenderPlan({
      messages: [chat({ id: "u1", role: "user", content: "检查", turn_id: "run_1" })],
      trajectory: stateWithTurn("run_1", [
        { seq: 2, type: "trajectory.turn.completed", run_id: "run_1", turn_id: "run_1", item_id: "turn:run_1", payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "turn", turn_id: "run_1", trace_node_id: "turn:run_1", status: "completed" } }
      ] as AgentEvent[]),
      turnEventMeta
    });
    const shape = plan.entries.map((entry) => entry.kind === "trace" ? `trace:${entry.turnId}` : "messages");
    expect(shape).toEqual(["messages", "trace:run_1"]);
  });
});
