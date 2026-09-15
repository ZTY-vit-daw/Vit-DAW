import { describe, expect, it } from "vitest";
import {
  isConsumedInteractionStatus,
  parseConsumedInteractionLedger,
  pendingInteractionFromContinuations,
  resolvedInteractionIdsFromEvents,
  stampConsumedInteractionActions
} from "./interactionGuard";
import type { AgentEvent, ChatMessage, JsonRecord, RuntimeContinuation } from "./types";

function waitingInteractionAction(interactionID: string, extras: JsonRecord = {}): JsonRecord {
  return {
    _ui_source: "interaction",
    id: interactionID,
    interaction_id: interactionID,
    kind: "improvement_proposal_confirmation",
    type: "improvement_proposal_confirmation",
    title: "改善提案确认",
    status: "waiting_for_user",
    actions: [
      { id: "approve", label: "确认执行", style: "primary" },
      { id: "cancel", label: "取消", style: "secondary" }
    ],
    ...extras
  };
}

function hydratedMessage(actions: JsonRecord[], extras: Partial<ChatMessage> = {}): ChatMessage {
  return {
    id: "history_node_1",
    source_id: "node_1",
    role: "assistant",
    content: "提案等待确认。",
    mode: "default",
    artifacts: [],
    actions,
    createdAt: 1000,
    status: "sent",
    lifecycle: "durable",
    persistence: "project_history",
    message_kind: "proposal",
    ...extras
  } as ChatMessage;
}

// F3 钉①：水合/恢复路径不得把已消费 interaction 卡渲染为可交互。
// server.go:4022 防线口径（"这个交互已处理或已过期"）的呈现面前置：
// 水合快照里已消费的 interaction 动作被标记 resolved + 摘除子按钮，
// 渲染层（StandardActionCard isInteractive = id 非空 ∧ 子按钮非空）与
// composer 拾取（isComposerInteraction 状态排除）因此都拿不到可交互形态。
describe("F3 钉① 已消费 interaction 水合不渲染为可交互", () => {
  it("已消费 ID 的 waiting_for_user 卡被标记 resolved 且摘除子按钮", () => {
    const messages = [hydratedMessage([waitingInteractionAction("interaction_dead1")])];
    const stamped = stampConsumedInteractionActions(messages, ["interaction_dead1"]);
    const action = stamped[0]?.actions?.[0] as JsonRecord;
    expect(String(action.status)).toBe("resolved");
    expect(String(action.resolved_action_id)).toBe("consumed_interaction_guard");
    expect(action.actions).toEqual([]);
    // 渲染不可交互的双重条件：子按钮为空 ∧ 状态命中已消费排除集
    expect(isConsumedInteractionStatus(String(action.status))).toBe(true);
  });

  it("renderID 命中同样摘除（响应路径同时记 id 与 renderID）", () => {
    const action = waitingInteractionAction("interaction_dead2", { title: "另一张卡" });
    const stamped = stampConsumedInteractionActions([hydratedMessage([action])], ["interaction_dead2"]);
    expect(String((stamped[0]?.actions?.[0] as JsonRecord).status)).toBe("resolved");
  });

  it("未消费的同形卡保持原样（不误伤真待确认卡）", () => {
    const action = waitingInteractionAction("interaction_live");
    const messages = [hydratedMessage([action])];
    const stamped = stampConsumedInteractionActions(messages, ["interaction_dead1"]);
    expect(stamped[0]?.actions?.[0]).toBe(action);
    expect(String((stamped[0]?.actions?.[0] as JsonRecord).status)).toBe("waiting_for_user");
  });

  it("非 interaction 来源的动作不盖章（project_result/command/executed 等不波及）", () => {
    const executed = { _ui_source: "executed", id: "interaction_dead1", status: "completed" };
    const stamped = stampConsumedInteractionActions([hydratedMessage([executed])], ["interaction_dead1"]);
    expect(stamped[0]?.actions?.[0]).toBe(executed);
  });

  it("幂等：已 resolved 的卡二次盖章不再变化", () => {
    const first = stampConsumedInteractionActions([hydratedMessage([waitingInteractionAction("interaction_dead3")])], ["interaction_dead3"]);
    const second = stampConsumedInteractionActions(first, ["interaction_dead3"]);
    expect(second).toEqual(first);
  });

  it("已消费状态排除集覆盖 server 4022 口径（resolved/responded/expired/consumed/answered）", () => {
    for (const status of ["resolved", "responded", "expired", "consumed", "answered"]) {
      expect(isConsumedInteractionStatus(status)).toBe(true);
    }
    for (const status of ["waiting_for_user", "awaiting_selection", "pending", ""]) {
      expect(isConsumedInteractionStatus(status)).toBe(false);
    }
  });
});

// F3 钉③：stale pending 卡不挤占新会话呈现（E2E run 213930 R2 阻断成因）。
// runtime status 的 continuation pending_interaction 投影是全局的——新浏览器
// 会话（新 conversationID）不得把他会话的 waiting_interaction 卡投影进自己
// 的 composer。conversation_id 缺失的行无法归属本会话，按隔离目标 fail-closed。
describe("F3 钉③ pending 交互投影的会话隔离", () => {
  const pendingRow = (conversationID: string, interactionID: string): RuntimeContinuation => ({
    continuation_id: `cont_${interactionID}`,
    conversation_id: conversationID,
    status: "waiting_interaction",
    pending_interaction: {
      interaction_id: interactionID,
      kind: "improvement_proposal_confirmation",
      requests: [{ type: "approval.requested", title: "提案确认" }]
    }
  });

  it("他会话的 waiting_interaction 行不投影为本会话 composer 卡", () => {
    const rows = [pendingRow("conv_old_session", "interaction_stale")];
    expect(pendingInteractionFromContinuations(rows, "conv_new_session")).toBeNull();
  });

  it("本会话的 waiting_interaction 行照常投影（现有 R2 路径不回退）", () => {
    const rows = [pendingRow("conv_mine", "interaction_mine")];
    const card = pendingInteractionFromContinuations(rows, "conv_mine");
    expect(card).not.toBeNull();
    expect(String(card?.interaction_id)).toBe("interaction_mine");
    expect(String(card?.status)).toBe("waiting_for_user");
    expect(String(card?._ui_source)).toBe("interaction");
  });

  it("conversation_id 缺失的行不可归属，不投影（fail-closed）", () => {
    const row = pendingRow("", "interaction_orphan");
    expect(pendingInteractionFromContinuations([row], "conv_mine")).toBeNull();
  });

  it("非 waiting_interaction 状态与缺 interaction_id 的行跳过", () => {
    const done: RuntimeContinuation = { continuation_id: "c1", conversation_id: "conv_mine", status: "completed" };
    const noID: RuntimeContinuation = {
      continuation_id: "c2",
      conversation_id: "conv_mine",
      status: "waiting_interaction",
      pending_interaction: { kind: "confirmation" }
    };
    expect(pendingInteractionFromContinuations([done, noID], "conv_mine")).toBeNull();
  });
});

// F3 面②配套：AGENT-F5 interaction.resolved 事件是 server 权威的已消费信号，
// 轮询路径把它记入持久已消费台账（对齐 completePendingInteractionContinuation
// 的撤卡事件），跨标签页/重载都能重建台账。
describe("interaction.resolved 事件提取", () => {
  it("从事件流提取 resolved interaction_id", () => {
    const events: AgentEvent[] = [
      { seq: 1, type: "item.started" } as AgentEvent,
      { seq: 2, type: "interaction.resolved", payload: { interaction_id: "interaction_done", kind: "confirmation" } } as AgentEvent,
      { seq: 3, type: "interaction.resolved", payload: {} } as AgentEvent
    ];
    expect(resolvedInteractionIdsFromEvents(events)).toEqual(["interaction_done"]);
  });
});

// MSG-REVIVE-1 钉②：回放后到的 resolved 事件必须能重盖已渲染流。裸开
//（storage 全灭）形态水合先于回放：图快照的 waiting_for_user 卡已渲染成
// 可交互卡，而台账此时为空；回放把 resolved 事件送达后，若只补台账不重盖
// 流，已 resolve 的交互会复活成死卡（点击报「交互已过期」，2026-09-15
// 用户手测形态）。提取→盖章的组合就是轮询路径的重盖链。
describe("MSG-REVIVE-1 回放补盖：resolved 事件重盖已渲染流", () => {
  it("水合先渲染、回放后到的 resolved 事件把复活卡转为只读", () => {
    const rendered = [hydratedMessage([waitingInteractionAction("interaction_r3_mixtick", { kind: "mix_tick_confirmation", type: "mix_tick_confirmation" })])];
    const events: AgentEvent[] = [
      { seq: 18, type: "interaction.resolved", payload: { interaction_id: "interaction_r3_mixtick", kind: "mix_tick_confirmation" } } as AgentEvent,
      { seq: 19, type: "mix_tick.pending", payload: { track_id: "1007" } } as AgentEvent
    ];
    const resolved = resolvedInteractionIdsFromEvents(events);
    expect(resolved).toEqual(["interaction_r3_mixtick"]);
    const stamped = stampConsumedInteractionActions(rendered, resolved);
    const action = stamped[0]?.actions?.[0] as JsonRecord;
    expect(String(action.status)).toBe("resolved");
    expect(String(action.resolved_action_id)).toBe("consumed_interaction_guard");
    expect(action.actions).toEqual([]);
    expect(isConsumedInteractionStatus(String(action.status))).toBe(true);
  });

  it("resolved id 未命中当前流时零变化（引用相等，正常形态零开销）", () => {
    const rendered = [hydratedMessage([waitingInteractionAction("interaction_live1")])];
    const events: AgentEvent[] = [
      { seq: 1, type: "interaction.resolved", payload: { interaction_id: "interaction_other" } } as AgentEvent
    ];
    const stamped = stampConsumedInteractionActions(rendered, resolvedInteractionIdsFromEvents(events));
    expect(stamped).toBe(rendered);
  });
});

// 台账解析：localStorage 里的消费记录必须是可清洗的（旧形状/脏数据不入集）。
describe("已消费台账解析", () => {
  it("接受字符串数组与 {id} 记录数组，剔除空白与坏行", () => {
    expect(parseConsumedInteractionLedger(["interaction_a", " interaction_b "])).toEqual(["interaction_a", "interaction_b"]);
    expect(parseConsumedInteractionLedger([{ id: "interaction_c" }, { interaction_id: "interaction_d" }, {}, " interaction_e "])).toEqual([
      "interaction_c",
      "interaction_d",
      "interaction_e"
    ]);
    expect(parseConsumedInteractionLedger([42, null, { id: 7 }, ""])).toEqual([]);
    expect(parseConsumedInteractionLedger("junk")).toEqual([]);
  });
});
