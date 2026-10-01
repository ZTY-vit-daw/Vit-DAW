import { describe, expect, it } from "vitest";
import {
  durableMessagesForStorage,
  historyMessageProtocol,
  messageProtocolIdentityKeys,
  mergeMessageCollections,
  proposalActionIdentity,
  proposalDecisionTranscript,
  reduceAgentEventActivities,
  resolveCompletedTurnProposals,
  resolveSupersededMessages,
  responseMessageProtocol,
  settleSupersededInteractionFamilies,
  settleTerminatedTurnInteractions,
  terminalTurnIDsFromEvents,
  transientMessage,
  TURN_EXPIRED_RESOLVED_ACTION,
  SUPERSEDED_BY_NEWER_RESOLVED_ACTION
} from "./messageLifecycle";
import { chatMessageFromAgentEvent } from "./App";
import { reduceTurnEventMeta } from "./trace/turnEventMeta";
import type { AgentEvent, ChatMessage, ChatResponse } from "./types";

function activityFromEvent(event: AgentEvent): ChatMessage {
  return transientMessage({
    id: `agent_event_${event.goal_id}_${event.item_id}`,
    source_id: `agent_event_${event.goal_id}_${event.item_id}`,
    role: "assistant",
    content: event.title || "正在执行",
    createdAt: event.seq,
    status: "pending"
  });
}

describe("Vit message lifecycle v1", () => {
  it("never persists transient or legacy processing rows", () => {
    const messages: ChatMessage[] = [
      {
        id: "proposal",
        role: "assistant",
        content: "B2 方案",
        createdAt: 1,
        lifecycle: "durable",
        persistence: "project_history",
        message_kind: "proposal"
      },
      transientMessage({ id: "activity", role: "assistant", content: "正在观察", createdAt: 2 }),
      { id: "agent_event_goal_item", role: "assistant", content: "旧即时事件", createdAt: 3, status: "sent" },
      { id: "interaction_processing_1", role: "assistant", content: "旧执行进度", createdAt: 4, status: "sent" }
    ];

    expect(durableMessagesForStorage(messages).map((message) => message.id)).toEqual(["proposal"]);
  });

  it("closes item activity on approval and closes the complete turn", () => {
    const started: AgentEvent = {
      seq: 1,
      type: "item.started",
      goal_id: "goal-1",
      run_id: "run-1",
      item_id: "item-1",
      logical_message_id: "agent_item:run-1:item-1",
      title: "正在读取工程"
    };
    const approval: AgentEvent = {
      ...started,
      seq: 2,
      type: "approval.requested",
      title: "等待确认"
    };
    const completed: AgentEvent = {
      ...started,
      seq: 3,
      type: "turn.completed",
      item_id: "",
      logical_message_id: "agent_turn:run-1"
    };

    const active = reduceAgentEventActivities([], [started], activityFromEvent);
    expect(active).toHaveLength(1);
    expect(active[0]).toMatchObject({ lifecycle: "transient", persistence: "none", message_kind: "activity", turn_id: "run-1" });
    expect(reduceAgentEventActivities(active, [approval], activityFromEvent)).toEqual([]);
    expect(reduceAgentEventActivities(active, [completed], activityFromEvent)).toEqual([]);
  });

  it("reuses the Project History identity for the durable Proposal response", () => {
    const response: ChatResponse = {
      reply: "B2 静态平衡方案",
      needs_confirmation: true,
      run_id: "run-2",
      project_history: {
        conversation_messages: [
          { role: "user", content: "进行 B2", node_id: "ask-1" },
          {
            role: "assistant",
            content: "B2 静态平衡方案",
            node_id: "proposal-node",
            logical_message_id: "proposal-node",
            message_kind: "proposal"
          }
        ]
      }
    };

    expect(responseMessageProtocol(response, response.reply || "")).toMatchObject({
      source_id: "proposal-node",
      logical_message_id: "proposal-node",
      message_kind: "proposal",
      lifecycle: "durable",
      persistence: "project_history",
      turn_id: "run-2"
    });
  });

  it("gives the HTTP Proposal and history row a shared merge identity", () => {
    const local: ChatMessage = {
      id: "local-proposal",
      source_id: "proposal-node",
      logical_message_id: "proposal-node",
      role: "assistant",
      content: "B2 静态平衡方案",
      actions: [{ kind: "proposal_approval" }],
      createdAt: 1,
      message_kind: "proposal"
    };
    const historyProtocol = historyMessageProtocol({
      node_id: "proposal-node",
      logical_message_id: "proposal-node",
      message_kind: "proposal"
    }, "assistant");
    const history: ChatMessage = {
      id: "history-proposal-node",
      role: "assistant",
      content: "B2 静态平衡方案",
      createdAt: 1,
      ...historyProtocol
    };

    const localKeys = new Set(messageProtocolIdentityKeys(local));
    const shared = messageProtocolIdentityKeys(history).filter((key) => localKeys.has(key));
    expect(shared).toContain("logical:proposal-node");

    const merged = mergeMessageCollections(
      [local],
      [history],
      messageProtocolIdentityKeys,
      (existing, incoming) => ({
        ...incoming,
        ...existing,
        source_id: existing.source_id || incoming.source_id,
        logical_message_id: existing.logical_message_id || incoming.logical_message_id,
        actions: [...(existing.actions ?? []), ...(incoming.actions ?? [])]
      })
    );
    expect(merged).toHaveLength(1);
    expect(merged[0].actions).toEqual([{ kind: "proposal_approval" }]);
  });

  it("treats legacy Project History rows as durable without rewriting them", () => {
    expect(historyMessageProtocol({ node_id: "legacy-node" }, "assistant")).toMatchObject({
      lifecycle: "durable",
      persistence: "project_history",
      message_kind: "assistant",
      logical_message_id: "legacy-node"
    });
  });

  it("keeps a durable Proposal but closes its actions when a Receipt supersedes it", () => {
    const proposal: ChatMessage = {
      id: "proposal",
      logical_message_id: "proposal-42",
      role: "assistant",
      content: "B2 proposal",
      actions: [{ kind: "proposal_approval", status: "waiting_for_user", actions: [{ id: "approve" }] }],
      createdAt: 1,
      lifecycle: "durable",
      persistence: "project_history",
      message_kind: "proposal"
    };
    const receipt: ChatMessage = {
      id: "receipt",
      role: "assistant",
      content: "B2 complete",
      createdAt: 2,
      supersedes: ["proposal-42"],
      lifecycle: "durable",
      persistence: "project_history",
      message_kind: "execution_receipt"
    };

    const resolved = resolveSupersededMessages([proposal, receipt]);
    expect(resolved[0].content).toBe("B2 proposal");
    expect(resolved[0].actions?.[0]).toMatchObject({ status: "completed", actions: [] });
    expect(resolved[1]).toEqual(receipt);
  });

  it("closes a restored generic confirmation when the same turn has a later execution receipt", () => {
    const proposal: ChatMessage = {
      id: "b1-proposal",
      role: "assistant",
      content: "B1 clip gain proposal",
      actions: [{
        id: "interaction-b1",
        kind: "confirmation",
        status: "waiting_for_user",
        actions: [{ id: "approve" }, { id: "cancel" }]
      }],
      createdAt: 1,
      turn_id: "run-b1",
      message_kind: "proposal"
    };
    const receipt: ChatMessage = {
      id: "b1-receipt",
      role: "assistant",
      content: "B1 clip gain applied and verified",
      createdAt: 2,
      turn_id: "run-b1",
      message_kind: "execution_receipt"
    };

    const resolved = resolveCompletedTurnProposals([proposal, receipt]);
    expect(resolved[0].actions?.[0]).toMatchObject({
      status: "completed",
      stage: "completed",
      resolved_action_id: "turn_terminal_receipt",
      actions: []
    });
  });

  it("keeps a later staged Proposal actionable when no terminal receipt follows it", () => {
    const firstProposal: ChatMessage = {
      id: "stage-one",
      role: "assistant",
      content: "Stage one proposal",
      actions: [{ id: "confirm-one", kind: "confirmation", actions: [{ id: "approve" }] }],
      createdAt: 1,
      turn_id: "run-staged",
      message_kind: "proposal"
    };
    const receipt: ChatMessage = {
      id: "stage-one-receipt",
      role: "assistant",
      content: "Stage one complete",
      createdAt: 2,
      turn_id: "run-staged",
      message_kind: "execution_receipt"
    };
    const nextProposal: ChatMessage = {
      id: "stage-two",
      role: "assistant",
      content: "Stage two proposal",
      actions: [{ id: "confirm-two", kind: "confirmation", status: "waiting_for_user", actions: [{ id: "approve" }] }],
      createdAt: 3,
      turn_id: "run-staged",
      message_kind: "proposal"
    };

    const resolved = resolveCompletedTurnProposals([firstProposal, receipt, nextProposal]);
    expect(resolved[0].actions?.[0]).toMatchObject({ status: "completed", actions: [] });
    expect(resolved[2].actions?.[0]).toMatchObject({ status: "waiting_for_user", actions: [{ id: "approve" }] });
  });

  it("closes a restored B4 plug-in selector when the same turn later completed", () => {
    const selector: ChatMessage = {
      id: "b4-plugin-selection",
      role: "assistant",
      content: "Choose the exact EQ plug-in for B4",
      actions: [{
        id: "interaction-b4-plugin",
        kind: "b4_plugin_selection",
        status: "waiting_for_user",
        actions: [{ id: "select_plugin_candidate_39" }, { id: "cancel" }]
      }],
      createdAt: 1,
      turn_id: "run-b4",
      message_kind: "assistant"
    };
    const verification: ChatMessage = {
      id: "b4-verification",
      role: "assistant",
      content: "B4 project batch completed and verified",
      createdAt: 2,
      turn_id: "run-b4",
      message_kind: "verification"
    };

    const resolved = resolveCompletedTurnProposals([selector, verification]);
    expect(resolved[0].actions?.[0]).toMatchObject({
      status: "completed",
      stage: "completed",
      resolved_action_id: "turn_terminal_receipt",
      actions: []
    });
  });

  it("does not close a persistent result card just because the turn completed", () => {
    const result: ChatMessage = {
      id: "result-card",
      role: "assistant",
      content: "Project result",
      actions: [{ id: "result", kind: "project_result", actions: [{ id: "open_report" }] }],
      createdAt: 1,
      turn_id: "run-result",
      message_kind: "assistant"
    };
    const verification: ChatMessage = {
      id: "result-verification",
      role: "assistant",
      content: "Verified",
      createdAt: 2,
      turn_id: "run-result",
      message_kind: "verification"
    };

    expect(resolveCompletedTurnProposals([result, verification])[0]).toEqual(result);
  });

  it("gives live and history-restored actions the same Proposal identity", () => {
    const live = {
      id: "interaction-live",
      kind: "proposal_approval",
      payload: { proposal_id: "proposal-99", proposal_presentation: { proposal_id: "proposal-99" } }
    };
    const restored = {
      id: "confirmation-plan",
      kind: "proposal_approval",
      plan_id: "plan-99",
      payload: { proposal_id: "proposal-99", plan_id: "plan-99" }
    };
    expect(proposalActionIdentity(live)).toBe("proposal-99");
    expect(proposalActionIdentity(restored)).toBe("proposal-99");
  });

  it("uses the durable conversational wording for Proposal decisions", () => {
    expect(proposalDecisionTranscript("approve", "确认执行")).toBe("可以执行");
    expect(proposalDecisionTranscript("cancel", "取消")).toBe("取消");
  });

  it("classifies a completed capability response as Verification", () => {
    const response: ChatResponse = {
      reply: "B2 已执行并验证通过",
      workflow: "capability_runtime_v1",
      workflow_data: {
        canary_stage: "executed_verified",
        execution_id: "execution-1",
        verification_result: { status: "pass" }
      }
    };
    expect(responseMessageProtocol(response, response.reply || "").message_kind).toBe("verification");
  });
});

// AUDITION-LANE-1（2026-09-14 手测命中）：audition.* 事件不携带事件级 turn 字段
// （服务端 emitAuditionEvent 不设 GoalID/RunID/TurnID，emitAgentEvent 回退链全落空），
// 活动行回合归属判 unbound → 全落流底 lane 且永不清退（探针实测 48 元素）。
// 真栈形态（2026-09-05 fixture webui_mtny2v9x）：telemetry 类事件（prepare.started /
// candidate.ready）的 payload.session 无 turn_id，只有 agent 侧发出的事件带——回合域
// 要按会话记忆继承 + 回填（audition.ts:93 previous?.turnID 的活动侧对应）。
describe("AUDITION-LANE-1：audition 活动归属会话回合域（不落流底 lane）", () => {
  const SID = "audition:turn:free_state_x:round-1";
  const SESSION_TURN = "turn:free_state_x";

  function auditionEvent(seq: number, type: string, options: { sessionTurn?: string; sourceTurn?: string } = {}): AgentEvent {
    const session: Record<string, unknown> = { session_id: SID, conversation_id: "c1", status: "preparing" };
    if (options.sessionTurn) {
      session.turn_id = options.sessionTurn;
    }
    return {
      seq, type, item_id: SID, item_type: "audition", status: "preparing", title: "Kernel audition",
      logical_message_id: `audition:${SID}:${type}`,
      created_at: new Date(Date.UTC(2026, 8, 14, 12, 0, 0) + seq * 1000).toISOString(),
      payload: { schema_version: "vit.kernel_audition.v1", session },
      ...(options.sourceTurn ? { source_turn_id: options.sourceTurn } : {})
    } as AgentEvent;
  }

  it("同批事件：首个携带会话回合域的事件回填早到活动，整族同键", () => {
    const events = [
      auditionEvent(27, "audition.prepare.started"),
      auditionEvent(28, "audition.candidate.ready"),
      auditionEvent(29, "audition.ready", { sessionTurn: SESSION_TURN })
    ];
    const activities = reduceAgentEventActivities([], events, (event) => chatMessageFromAgentEvent(event, "default"));
    expect(activities).toHaveLength(3);
    for (const activity of activities) {
      expect(activity.turn_id).toBe(SESSION_TURN);
    }
  });

  it("晚到无回合域事件继承会话回合记忆（跨轮询批次）", () => {
    const first = reduceAgentEventActivities(
      [],
      [auditionEvent(27, "audition.prepare.started"), auditionEvent(29, "audition.ready", { sessionTurn: SESSION_TURN })],
      (event) => chatMessageFromAgentEvent(event, "default")
    );
    const second = reduceAgentEventActivities(first, [auditionEvent(30, "audition.selected")], (event) => chatMessageFromAgentEvent(event, "default"));
    expect(second).toHaveLength(3);
    for (const activity of second) {
      expect(activity.turn_id).toBe(SESSION_TURN);
    }
  });

  it("source_turn_id 优先（B9 判定卡同键）：服务端补轮次域后按 run 绑定", () => {
    const activities = reduceAgentEventActivities(
      [],
      [auditionEvent(31, "audition.prepare", { sessionTurn: SESSION_TURN, sourceTurn: "run_9" })],
      (event) => chatMessageFromAgentEvent(event, "default")
    );
    expect(activities[0]?.turn_id).toBe("run_9");
  });

  it("C0 判定事件（turn 域跨命名空间）按 source_turn_id 绑定 run，轮次终局照旧清退", () => {
    const judgment: AgentEvent = {
      seq: 40, type: "trajectory.user_judgment.requested",
      goal_id: "goal_1", run_id: "run_9", turn_id: SESSION_TURN, source_turn_id: "run_9",
      item_id: "trace-j1", item_type: "trajectory", status: "waiting", title: "User judgment",
      payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "trace-j1", turn_id: SESSION_TURN, node_kind: "user_judgment", status: "waiting" }
    } as AgentEvent;
    const terminal: AgentEvent = {
      seq: 41, type: "trajectory.turn.completed",
      goal_id: "goal_1", run_id: "run_9", turn_id: "run_9", source_turn_id: "run_9",
      item_id: "turn:run_9", item_type: "trajectory", status: "completed",
      payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "turn:run_9", turn_id: "run_9", node_kind: "turn", status: "completed" }
    } as AgentEvent;
    const activities = reduceAgentEventActivities([], [judgment], (event) => chatMessageFromAgentEvent(event, "default"));
    expect(activities[0]?.turn_id).toBe("run_9");
    expect(reduceAgentEventActivities(activities, [terminal], (event) => chatMessageFromAgentEvent(event, "default"))).toHaveLength(0);
  });

  it("零回退：不带回合域的普通活动仍无 turn 绑定（照旧留 lane）", () => {
    const plain: AgentEvent = { seq: 50, type: "item.started", item_id: "upload-1", item_type: "upload", status: "running", title: "上传素材" } as AgentEvent;
    const activities = reduceAgentEventActivities([], [plain], (event) => chatMessageFromAgentEvent(event, "default"));
    expect(activities).toHaveLength(1);
    expect(activities[0].turn_id ?? "").toBe("");
  });
});

// FIX-CONFIRM-CARD-1 ②（M1 手测缺陷②）：确认卡生命周期与任务态绑定——
// 任务终结（完成/失败/停止）或被同族新卡替代时，未应答卡立即转入不可交互
// 终态（摘除子按钮），不残留可点击的"过期"卡（点击才揭示 server 4022）。
function waitingConfirmationAction(interactionID: string, extras: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    _ui_source: "interaction",
    id: interactionID,
    interaction_id: interactionID,
    kind: "mix_tick_confirmation",
    type: "mix_tick_confirmation",
    workflow: "mix_tick",
    title: "混音单步待确认",
    status: "waiting_for_user",
    actions: [
      { id: "approve", label: "确认执行", style: "primary" },
      { id: "cancel", label: "取消", style: "secondary" }
    ],
    ...extras
  };
}

function interactionCarrierMessage(id: string, actions: Record<string, unknown>[], extras: Partial<ChatMessage> = {}): ChatMessage {
  return {
    id,
    role: "assistant",
    content: "需要确认后执行。",
    mode: "default",
    artifacts: [],
    actions: actions as ChatMessage["actions"],
    createdAt: 1000,
    status: "sent",
    lifecycle: "durable",
    persistence: "project_history",
    message_kind: "proposal",
    ...extras
  } as ChatMessage;
}

describe("FIX-CONFIRM-CARD-1 ②：terminalTurnIDsFromEvents 回合真终结提取", () => {
  it("完成/失败/停止入列；waiting_continue 切片驻留不是终结", () => {
    const events: AgentEvent[] = [
      { seq: 1, type: "turn.started", run_id: "run_a", status: "running" } as AgentEvent,
      { seq: 2, type: "turn.completed", run_id: "run_a", status: "waiting_continue" } as AgentEvent,
      { seq: 3, type: "turn.completed", run_id: "run_a", status: "completed" } as AgentEvent,
      { seq: 4, type: "turn.failed", run_id: "run_b", status: "failed" } as AgentEvent,
      { seq: 5, type: "turn.stopped", run_id: "run_c", status: "stopped" } as AgentEvent,
      { seq: 6, type: "trajectory.turn.completed", run_id: "run_d", status: "completed" } as AgentEvent
    ];
    expect(terminalTurnIDsFromEvents(events)).toEqual(["run_a", "run_b", "run_c", "run_d"]);
  });

  it("非终结事件与无回合域事件不入列；同回合重复终结去重", () => {
    const events: AgentEvent[] = [
      { seq: 1, type: "item.completed", run_id: "run_a", status: "completed" } as AgentEvent,
      { seq: 2, type: "turn.completed", status: "completed" } as AgentEvent,
      { seq: 3, type: "turn.completed", run_id: "run_a", status: "completed" } as AgentEvent,
      { seq: 4, type: "turn.completed", run_id: "run_a", status: "completed" } as AgentEvent
    ];
    expect(terminalTurnIDsFromEvents(events)).toEqual(["run_a"]);
  });
});

describe("FIX-CONFIRM-CARD-1 ②：settleTerminatedTurnInteractions 回合终结收卡", () => {
  it("终结回合内未应答卡转不可交互终态（摘按钮+turn_expired 标识）", () => {
    const messages = [interactionCarrierMessage("m1", [waitingConfirmationAction("interaction_1")], { turn_id: "run_a" })];
    const settled = settleTerminatedTurnInteractions(messages, ["run_a"]);
    const action = settled[0]?.actions?.[0] as Record<string, unknown>;
    expect(String(action.status)).toBe("completed");
    expect(String(action.resolved_action_id)).toBe(TURN_EXPIRED_RESOLVED_ACTION);
    expect(action.actions).toEqual([]);
  });

  it("其它回合与他回合卡不波及；已结算卡（按钮已摘）不二次改判", () => {
    const approved = { ...waitingConfirmationAction("interaction_done"), status: "completed", resolved_action_id: "approve", actions: [] };
    const messages = [
      interactionCarrierMessage("m0", [waitingConfirmationAction("interaction_other_turn")], { turn_id: "run_z" }),
      interactionCarrierMessage("m1", [approved, waitingConfirmationAction("interaction_open")], { turn_id: "run_a" })
    ];
    const settled = settleTerminatedTurnInteractions(messages, ["run_a"]);
    const actions = settled[1]?.actions ?? [];
    expect((actions[0] as Record<string, unknown>).resolved_action_id).toBe("approve");
    expect((actions[1] as Record<string, unknown>).resolved_action_id).toBe(TURN_EXPIRED_RESOLVED_ACTION);
    expect((settled[0]?.actions?.[0] as Record<string, unknown>).status).toBe("waiting_for_user");
  });

  it("无匹配时引用相等（零拷贝）", () => {
    const messages = [interactionCarrierMessage("m1", [waitingConfirmationAction("interaction_1")], { turn_id: "run_a" })];
    expect(settleTerminatedTurnInteractions(messages, ["run_b"])).toBe(messages);
    expect(settleTerminatedTurnInteractions(messages, [])).toBe(messages);
  });
});

describe("FIX-CONFIRM-CARD-1 ②：settleSupersededInteractionFamilies 同族替代收卡", () => {
  it("同 plan 出现更新的待应答卡时旧卡结算为已替代；最新卡保持可交互", () => {
    const messages = [
      interactionCarrierMessage("m1", [waitingConfirmationAction("interaction_old", { kind: "confirmation", type: "confirmation", payload: { plan_id: "plan_1" }, workflow: "" })]),
      interactionCarrierMessage("m2", [waitingConfirmationAction("interaction_new", { kind: "confirmation", type: "confirmation", payload: { plan_id: "plan_1" }, workflow: "" })])
    ];
    const settled = settleSupersededInteractionFamilies(messages);
    expect((settled[0]?.actions?.[0] as Record<string, unknown>).resolved_action_id).toBe(SUPERSEDED_BY_NEWER_RESOLVED_ACTION);
    expect((settled[0]?.actions?.[0] as Record<string, unknown>).actions).toEqual([]);
    expect((settled[1]?.actions?.[0] as Record<string, unknown>).status).toBe("waiting_for_user");
  });

  it("mix_tick 同 workflow 归族（每会话至多一张待确认）；不同族互不影响", () => {
    const messages = [
      interactionCarrierMessage("m1", [waitingConfirmationAction("interaction_tick_old")]),
      interactionCarrierMessage("m2", [
        waitingConfirmationAction("interaction_tick_new"),
        waitingConfirmationAction("interaction_treat", { kind: "mix_treatment_confirmation", type: "mix_treatment_confirmation", workflow: "mix_treatment" })
      ])
    ];
    const settled = settleSupersededInteractionFamilies(messages);
    const older = settled[0]?.actions?.[0] as Record<string, unknown>;
    const newerTick = (settled[1]?.actions ?? [])[0] as Record<string, unknown>;
    const treatment = (settled[1]?.actions ?? [])[1] as Record<string, unknown>;
    expect(String(older.resolved_action_id)).toBe(SUPERSEDED_BY_NEWER_RESOLVED_ACTION);
    expect(String(newerTick.status)).toBe("waiting_for_user");
    expect(String(treatment.status)).toBe("waiting_for_user");
  });

  it("capability proposal 卡（proposal_approval+presentation）同样按 plan 族替代结算", () => {
    const presentation = { schema_version: "vit.proposal_presentation.v1", proposal_id: "prop_1", title: "方案" };
    const proposalAction = (interactionID: string, revision: number) => waitingConfirmationAction(interactionID, {
      kind: "proposal_approval", type: "proposal_approval", workflow: "capability_runtime_v1",
      payload: { plan_id: "prop_1", proposal_presentation: { ...presentation, proposal_revision: revision } }
    });
    const messages = [
      interactionCarrierMessage("m1", [proposalAction("interaction_rev0", 0)]),
      interactionCarrierMessage("m2", [proposalAction("interaction_rev1", 1)])
    ];
    const settled = settleSupersededInteractionFamilies(messages);
    expect((settled[0]?.actions?.[0] as Record<string, unknown>).resolved_action_id).toBe(SUPERSEDED_BY_NEWER_RESOLVED_ACTION);
    expect((settled[1]?.actions?.[0] as Record<string, unknown>).status).toBe("waiting_for_user");
  });
});

// WEBUI-MSG-ORDER-1（2026-10-01，M8 手测取证）：同一 run 的生命周期翻转重放——
// waiting_continue 续跑让 turn.started/completed/failed 携带同一 logical_message_id
// （M8 证据 agent_turn:run_e5796736a4865570 贯穿 seq1→seq47），且存在同 id 双发对
// （audition.ready 0.5-14ms 错位；trajectory.turn.completed 同 ms 双发但 item_id
// 不同=实验回合与 run 壳各自合法收口）。本组钉活动面/记账面对翻转与重复投递的
// 稳健性：同 id 再 started 不得复活已清退活动、不得重排（原位更新）、重复事件
// 不虚增。
describe("WEBUI-MSG-ORDER-1：同 logical id 生命周期翻转重放（started→completed→items→completed→started→failed）", () => {
  const RUN = "run_e5796736a4865570";
  const GOAL = "goal_6da74bd8211d252f";
  const TURN_LOGICAL_ID = `agent_turn:${RUN}`;
  const base = { conversation_id: "webui_muo6fygb", goal_id: GOAL, run_id: RUN, source_turn_id: RUN, turn_id: RUN };

  function event(seq: number, at: number, type: string, extra: Partial<AgentEvent> = {}): AgentEvent {
    return {
      seq,
      type,
      created_at: new Date(Date.parse("2026-09-30T22:04:59.321+08:00") + at).toISOString(),
      logical_message_id: type.startsWith("audition.") ? `audition:audition:${GOAL}` : TURN_LOGICAL_ID,
      ...base,
      ...extra
    } as AgentEvent;
  }

  // M8 事件序列的取证压缩版：轮次生命周期两次开启 + 两段 item + 双发对。
  const replay: AgentEvent[] = [
    event(1, 0, "turn.started", { title: "正在处理", body: "检查一下当前工程有什么问题" }),
    event(3, 5_008, "item.started", { item_id: "ccb_observation_a", item_type: "daw_action", title: "正在执行操作", logical_message_id: "" }),
    event(4, 5_029, "item.completed", { item_id: "ccb_observation_a", item_type: "daw_action", status: "completed", logical_message_id: "" }),
    event(5, 5_113, "turn.completed", { status: "waiting_continue", title: "处理完成", body: "我还在继续处理这个任务，完成后再向你汇报。" }),
    event(8, 37_299, "item.started", { item_id: "ccb_observation_b", item_type: "daw_action", title: "正在执行操作", logical_message_id: "" }),
    event(9, 37_397, "item.completed", { item_id: "ccb_observation_b", item_type: "daw_action", status: "completed", logical_message_id: "" }),
    // trajectory.turn.completed 同 ms 双发：item_id 不同（实验回合 / run 壳）——合法成对
    event(32, 98_421, "trajectory.turn.completed", { item_id: "turn:free_state_c2681cbd102907f6", status: "waiting_for_user", logical_message_id: "trajectory:turn:free_state_c2681cbd102907f6:turn" }),
    event(33, 98_421, "trajectory.turn.completed", { item_id: `turn:${RUN}`, status: "waiting_for_user", logical_message_id: `trajectory:${RUN}:turn` }),
    // 同 id 双发对：audition.ready 2ms 错位（内核遥测+agent 侧双源形态）
    event(20, 45_315, "audition.ready", { item_id: `audition:turn:free_state_c2681cbd102907f6:round-1`, item_type: "audition", status: "ready", title: "Kernel audition" }),
    event(21, 45_317, "audition.ready", { item_id: `audition:turn:free_state_c2681cbd102907f6:round-1`, item_type: "audition", status: "ready", title: "Kernel audition" }),
    // 同 id 翻转：第二轮输入再次 turn.started（同一 logical id），47s 后 failed
    event(46, 141_624, "turn.started", { title: "正在处理", body: "你能再检查一下Bass轨道，看看它的低频有没有什么问题吗？" }),
    event(47, 188_420, "turn.failed", { status: "failed", title: "执行失败", body: "任务在形成有效结算前失败；失败原因与已有证据已保留。" })
  ];

  it("turn 生命周期事件不产消息、不复活已清退活动；同 id 再 started 后 failed 终局清场干净", () => {
    // 喂到 seq21（第一轮收口 + audition 双发）：item 活动已被 turn.completed 清退，audition 行存活
    const afterFirstSlice = reduceAgentEventActivities([], replay.slice(0, 10), chatMessageFromAgentEvent);
    expect(afterFirstSlice.filter((row) => row.source_id?.includes("ccb_observation"))).toHaveLength(0);
    const auditionRows = afterFirstSlice.filter((row) => row.source_id?.includes("audition"));
    expect(auditionRows).toHaveLength(1);
    // 翻转 + 终局：第二个 turn.started 不复活任何活动，turn.failed 清场后为空
    const afterReplay = reduceAgentEventActivities(afterFirstSlice, replay.slice(10), chatMessageFromAgentEvent);
    expect(afterReplay).toHaveLength(0);
    // 全量一次喂入（轮询窗口可能整段重放）终态一致
    expect(reduceAgentEventActivities([], replay, chatMessageFromAgentEvent)).toHaveLength(0);
  });

  it("同 id 双发不重排：audition 活动原位更新、保留首见 createdAt（不因重复投递换位）", () => {
    const firstAt = Date.parse("2026-09-30T22:04:59.321+08:00") + 45_315;
    const rows = reduceAgentEventActivities([], replay.slice(0, 10), chatMessageFromAgentEvent);
    const auditionRow = rows.find((row) => row.source_id?.includes("audition"));
    expect(auditionRow).toBeDefined();
    expect(auditionRow?.createdAt).toBe(firstAt);
  });

  it("记账面：startedAt 取首次开始（锚定证据不被再 started 拉走），双发终局/重复 item 不虚增", () => {
    const meta = reduceTurnEventMeta({}, replay);
    const turn = meta[RUN];
    expect(turn).toBeDefined();
    expect(turn.startedAt).toBe(Date.parse("2026-09-30T22:04:59.321+08:00"));
    expect(turn.itemActivityKeys).toEqual(["ccb_observation_a", "ccb_observation_b"]);
    expect(turn.itemActivityCount).toBe(2);
    expect(turn.endedAt).toBe(Date.parse("2026-09-30T22:04:59.321+08:00") + 188_420);
  });

  it("终局提取：重复的 trajectory.turn.completed 去重后仍只回报一个回合 id", () => {
    expect(terminalTurnIDsFromEvents(replay)).toEqual([RUN]);
  });
});
