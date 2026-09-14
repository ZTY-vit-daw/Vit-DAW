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
  transientMessage
} from "./messageLifecycle";
import { chatMessageFromAgentEvent } from "./App";
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
