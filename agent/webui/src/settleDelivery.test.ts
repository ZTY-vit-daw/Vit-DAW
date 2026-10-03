import { describe, expect, it } from "vitest";
import { chatMessageFromAgentEvent } from "./App";
import { reduceAgentEventActivities } from "./messageLifecycle";
import { appendChainResultMessages, settlementMessagesFromEvents } from "./trace/traceDelivery";
import { buildMessageStreamRenderPlan } from "./trace/renderPlan";
import { emptyTrajectoryState, reduceTrajectoryEvents } from "./trajectory";
import { reduceTurnEventMeta } from "./trace/turnEventMeta";
import type { AgentEvent, ChatMessage } from "./types";

// SETTLE-DELIVER-1（2026-10-03 手测场取证 coord/runs/MANUAL-TEST-20261003，
// conversation=webui_murptx58，43 事件流）：判定链本体已通，结算后续链断在三处——
// A（结算确认不进对话流）/ B（agent 侧 Go 钉）/ C（二轮消息插队显示在首轮输出
// 上方）。本文件钉 webui 侧两处：A 的 live 投递（事件→正式消息）与 C 的渲染序。

function chat(partial: Partial<ChatMessage> & Pick<ChatMessage, "id" | "role" | "content">): ChatMessage {
  return { createdAt: 0, ...partial } as ChatMessage;
}

// 取证流的 judgment.settled 实形（seq 37）：turn 域=run 域、body=结算报告、
// payload.settlement_reply 标记、logical_message_id=judgment_settle:<evidence>。
function judgmentSettledEvent(extras: Partial<AgentEvent> = {}): AgentEvent {
  return {
    seq: 37,
    type: "judgment.settled",
    conversation_id: "webui_murptx58",
    goal_id: "goal_40e1ebe6ad0d1125",
    run_id: "run_6954b34145002970",
    item_id: "judgment_settle:judgment-2983d2012169bba4",
    item_type: "judgment",
    status: "completed",
    body: "A/B 判定已落账并完成结算：已按你的判定保留改动后状态。",
    payload: {
      schema_version: "vit.kernel_audition.v1",
      judgment_evidence_id: "judgment-2983d2012169bba4",
      experiment_id: "turn:free_state_2c22bedaf4b4acb4",
      experiment_outcome: "improved",
      settlement_reply: true
    },
    created_at: "2026-10-03T09:32:44.7258056+08:00",
    message_kind: "activity",
    turn_id: "run_6954b34145002970",
    source_turn_id: "run_6954b34145002970",
    logical_message_id: "judgment_settle:judgment-2983d2012169bba4",
    ...extras
  } as AgentEvent;
}

describe("settlement messages from events（SETTLE-DELIVER-1 症状 A live 面）", () => {
  const extract = (events: AgentEvent[]) => settlementMessagesFromEvents(events, "default", chatMessageFromAgentEvent);

  it("judgment.settled+settlement_reply 提取为正式助手消息（body+logical 身份）", () => {
    const messages = extract([judgmentSettledEvent()]);
    expect(messages).toHaveLength(1);
    expect(messages[0]?.role).toBe("assistant");
    expect(messages[0]?.content).toContain("A/B 判定已落账并完成结算");
    expect(messages[0]?.logical_message_id).toBe("judgment_settle:judgment-2983d2012169bba4");
  });

  it("无 settlement_reply 标记的 judgment.settled 不提取（非结算确认事件零回退）", () => {
    const messages = extract([judgmentSettledEvent({ payload: { experiment_outcome: "improved" } })]);
    expect(messages).toHaveLength(0);
  });

  it("轮询重复投递按 logical_message_id 去重", () => {
    const messages = extract([
      judgmentSettledEvent({ seq: 37 }),
      judgmentSettledEvent({ seq: 44 })
    ]);
    expect(messages).toHaveLength(1);
  });

  it("追加进 messages 幂等（appendChainResultMessages 复用，不双份）", () => {
    const messages = extract([judgmentSettledEvent()]);
    const once = appendChainResultMessages([chat({ id: "u1", role: "user", content: "把吉他提一点" })], messages);
    const twice = appendChainResultMessages(once, extract([judgmentSettledEvent({ seq: 90 })]));
    expect(twice).toHaveLength(2);
  });

  it("活动线不再承载结算确认（正式消息唯一表面）", () => {
    const activities = reduceAgentEventActivities([], [judgmentSettledEvent()], chatMessageFromAgentEvent);
    expect(activities.filter((message) => message.content.includes("A/B 判定已落账"))).toHaveLength(0);
    // 无标记的 judgment.settled 照旧走活动线（旧流行为不变）。
    const unmarked = reduceAgentEventActivities([], [judgmentSettledEvent({ payload: {} })], chatMessageFromAgentEvent);
    expect(unmarked.filter((message) => message.content.includes("A/B 判定已落账"))).toHaveLength(1);
  });
});

// SETTLE-DELIVER-1 症状 C：取证流实形回放（事件 turn 域全在 run 域——CONTRACT-1
// C0 双写后 free_state 实验节点归并进 run_1 轮次块，实验块身份只在 nativeTurnIds
// 账上）。用户目视形态：二轮乐观输入（无 turn_id）与一轮链终局（无 turn_id）并入
// 同一 loose 组，旧实现把组按 user/rest 切分——输入被切到链终局上方。修法
// （WEBUI-MSG-ORDER-3）：无锚组不切分，整组按 createdAt 保序。
describe("SETTLE-DELIVER-1 症状 C：A/B 判定后续场渲染序（取证流身份实形）", () => {
  const T0 = Date.parse("2026-10-03T09:30:50.000+08:00");
  const RUN_1 = "run_6954b34145002970";
  const RUN_2 = "run_7630a1f2f772359f";
  const FS = "turn:free_state_2c22bedaf4b4acb4";

  function forensicIdentityEvents(): AgentEvent[] {
    return [
      { seq: 1, type: "turn.started", run_id: RUN_1, source_turn_id: RUN_1, created_at: new Date(T0 + 100).toISOString(), payload: { turn_kind: "" } },
      { seq: 2, type: "trajectory.turn.started", run_id: RUN_1, source_turn_id: RUN_1, trajectory_turn_id: RUN_1, item_id: `turn:${RUN_1}`, created_at: new Date(T0 + 120).toISOString(), payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "turn", turn_id: RUN_1, trace_node_id: `turn:${RUN_1}` } },
      // free_state 实验事件：source_turn_id=run 域（C0 消息归属域），原生域在 payload.turn_id。
      { seq: 10, type: "trajectory.turn.started", run_id: RUN_1, source_turn_id: RUN_1, trajectory_turn_id: FS, turn_id: FS, item_id: `turn:${FS}`, created_at: new Date(T0 + 43_000).toISOString(), payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "turn", turn_id: FS, trace_node_id: `turn:${FS}` } },
      { seq: 13, type: "trajectory.round.started", run_id: RUN_1, source_turn_id: RUN_1, trajectory_turn_id: FS, turn_id: FS, item_id: "trace-aff98cfddb87b2d8", created_at: new Date(T0 + 46_000).toISOString(), payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "round", turn_id: FS, trace_node_id: "trace-aff98cfddb87b2d8", round_id: "round-1-343c5edf3d91ae8c" } },
      { seq: 32, type: "trajectory.turn.completed", run_id: RUN_1, source_turn_id: RUN_1, trajectory_turn_id: FS, turn_id: FS, item_id: `turn:${FS}`, created_at: new Date(T0 + 109_000).toISOString(), payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "turn", turn_id: FS, trace_node_id: `turn:${FS}`, status: "completed" } },
      { seq: 38, type: "turn.started", run_id: RUN_2, source_turn_id: RUN_2, created_at: new Date(T0 + 134_800).toISOString(), payload: {} },
      { seq: 39, type: "trajectory.turn.started", run_id: RUN_2, source_turn_id: RUN_2, trajectory_turn_id: RUN_2, item_id: `turn:${RUN_2}`, created_at: new Date(T0 + 134_900).toISOString(), payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "turn", turn_id: RUN_2, trace_node_id: `turn:${RUN_2}` } },
      { seq: 43, type: "trajectory.turn.completed", run_id: RUN_2, source_turn_id: RUN_2, trajectory_turn_id: RUN_2, item_id: `turn:${RUN_2}`, created_at: new Date(T0 + 159_000).toISOString(), payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "turn", turn_id: RUN_2, trace_node_id: `turn:${RUN_2}`, status: "completed" } }
    ] as AgentEvent[];
  }

  const shapeOf = (plan: ReturnType<typeof buildMessageStreamRenderPlan>) =>
    plan.entries.map((entry) => entry.kind === "trace"
      ? `trace:${entry.turnId}`
      : entry.kind === "receipt"
        ? `receipt:${entry.turnId}`
        : `messages:${entry.messages.map((message: ChatMessage) => message.id).join(",")}`);

  function planWith(messages: ChatMessage[]) {
    const events = forensicIdentityEvents();
    return buildMessageStreamRenderPlan({
      messages,
      trajectory: reduceTrajectoryEvents(emptyTrajectoryState(), events),
      turnEventMeta: reduceTurnEventMeta({}, events)
    });
  }

  it("用户目视形态（RED 基线）：二轮输入不得插队到一轮链终局上方", () => {
    const messages = [
      chat({ id: "u1", role: "user", content: "帮我看看当前工程", turn_id: RUN_1, createdAt: T0 }),
      chat({ id: "agent_event_goal_40e1ebe6ad0d1125_chain_result", role: "assistant", content: "这一步已经应用好了：Track 1017音量 +3 dB。请试听。", createdAt: T0 + 109_600 }),
      chat({ id: "u2", role: "user", content: "能帮我看看bass轨道的低频怎么样吗", createdAt: T0 + 134_800 }),
      chat({ id: "a2", role: "system", content: "observation belongs to a different project revision", turn_id: RUN_2, createdAt: T0 + 159_000, status: "error" })
    ];
    const plan = planWith(messages);
    expect(shapeOf(plan)).toEqual([
      "messages:u1",
      `trace:${RUN_1}`,
      "messages:agent_event_goal_40e1ebe6ad0d1125_chain_result,u2",
      `trace:${RUN_2}`,
      "messages:a2"
    ]);
  });

  it("A/B 同场景复验（含结算确认消息）：一轮输出（链终局+结算确认）整体在二轮输入之上", () => {
    const settleMessages = settlementMessagesFromEvents([judgmentSettledEvent()], "default", chatMessageFromAgentEvent);
    expect(settleMessages).toHaveLength(1);
    const messages = [
      chat({ id: "u1", role: "user", content: "帮我看看当前工程", turn_id: RUN_1, createdAt: T0 }),
      chat({ id: "agent_event_goal_40e1ebe6ad0d1125_chain_result", role: "assistant", content: "这一步已经应用好了：Track 1017音量 +3 dB。请试听。", createdAt: T0 + 109_600 }),
      ...settleMessages.map((message) => ({ ...message, createdAt: T0 + 114_700 })),
      chat({ id: "u2", role: "user", content: "能帮我看看bass轨道的低频怎么样吗", createdAt: T0 + 134_800 }),
      chat({ id: "a2", role: "system", content: "observation belongs to a different project revision", turn_id: RUN_2, createdAt: T0 + 159_000, status: "error" })
    ];
    const plan = planWith(messages);
    expect(shapeOf(plan)).toEqual([
      "messages:u1",
      `trace:${RUN_1}`,
      "messages:agent_event_goal_40e1ebe6ad0d1125_chain_result,agent_event_goal_40e1ebe6ad0d1125_judgment_settle:judgment-2983d2012169bba4,u2",
      `trace:${RUN_2}`,
      "messages:a2"
    ]);
  });
});
