import type {
  AgentEvent,
  AgentMode,
  ChatMessage,
  ChatResponse,
  JsonRecord,
  MessageKind,
  MessageLifecycle,
  MessagePersistence
} from "./types";

const durableLifecycle: MessageLifecycle = "durable";
const transientLifecycle: MessageLifecycle = "transient";
const projectPersistence: MessagePersistence = "project_history";
const localPersistence: MessagePersistence = "local";
const noPersistence: MessagePersistence = "none";

const legacyTransientPrefixes = [
  "agent_event_",
  "interaction_processing_"
];

export function durableMessage(
  message: ChatMessage,
  defaults: { kind?: MessageKind; persistence?: MessagePersistence } = {}
): ChatMessage {
  return {
    ...message,
    lifecycle: durableLifecycle,
    persistence: message.persistence === noPersistence ? defaults.persistence ?? localPersistence : message.persistence ?? defaults.persistence ?? localPersistence,
    message_kind: message.message_kind ?? defaults.kind ?? inferMessageKind(message)
  };
}

export function transientMessage(message: ChatMessage): ChatMessage {
  return {
    ...message,
    lifecycle: transientLifecycle,
    persistence: noPersistence,
    message_kind: "activity"
  };
}

export function isDurableMessage(message: ChatMessage): boolean {
  if (message.lifecycle === transientLifecycle || message.persistence === noPersistence) {
    return false;
  }
  return !isLegacyTransientMessage(message);
}

export function isTransientMessage(message: ChatMessage): boolean {
  return !isDurableMessage(message);
}

export function durableMessagesForStorage(messages: ChatMessage[]): ChatMessage[] {
  return messages.filter(isDurableMessage);
}

export function isLegacyTransientMessage(message: ChatMessage): boolean {
  const ids = [message.id, message.source_id ?? ""];
  if (ids.some((id) => legacyTransientPrefixes.some((prefix) => id.startsWith(prefix)))) {
    return true;
  }
  return message.message_kind === "activity" || (message.status === "pending" && message.lifecycle !== durableLifecycle);
}

export function inferMessageKind(message: Pick<ChatMessage, "role" | "status" | "actions" | "message_kind">): MessageKind {
  if (message.message_kind) {
    return message.message_kind;
  }
  if (message.status === "error") {
    return "error";
  }
  if (message.role === "user") {
    return "user";
  }
  if ((message.actions ?? []).some(isProposalAction)) {
    return "proposal";
  }
  if (message.role === "system") {
    return "system";
  }
  return "assistant";
}

export function inferResponseMessageKind(response: ChatResponse): MessageKind {
  if (response.message_kind) {
    return response.message_kind;
  }
  const status = text(response.goal_status ?? response.status).toLowerCase();
  if (response.error || status === "error" || status === "failed") {
    return "error";
  }
  const proposal = record(response.proposal_presentation ?? record(response.workflow_data).proposal_presentation);
  if (response.needs_confirmation || Object.keys(proposal).length > 0 || status.includes("confirmation")) {
    return "proposal";
  }
  const workflowData = record(response.workflow_data);
  const verificationResult = record(workflowData.verification_result);
  const workflowStage = text(workflowData.canary_stage).toLowerCase();
  if (Object.keys(verificationResult).length > 0 || text(workflowData.verification) || workflowStage.startsWith("executed_")) {
    return "verification";
  }
  if ((response.executed_kernel_reply?.length ?? 0) > 0 || (response.project_result_cards?.length ?? 0) > 0) {
    return "execution_receipt";
  }
  return "assistant";
}

export function proposalDecisionTranscript(actionID: string, fallback: string): string {
  const decision = actionID.trim().toLowerCase();
  if (["cancel", "reject", "deny"].includes(decision)) {
    return "取消";
  }
  if (["approve", "allow", "confirm", "yes"].includes(decision)) {
    return "可以执行";
  }
  return fallback;
}

export function responseMessageProtocol(response: ChatResponse, content: string): Partial<ChatMessage> {
  const identity = responseHistoryIdentity(response, content);
  return {
    source_id: identity.sourceID,
    lifecycle: response.lifecycle ?? durableLifecycle,
    persistence: response.persistence ?? (identity.sourceID ? projectPersistence : localPersistence),
    message_kind: inferResponseMessageKind(response),
    turn_id: response.turn_id ?? text(response.run_id ?? response.goal_id),
    logical_message_id: response.logical_message_id ?? identity.logicalMessageID ?? identity.sourceID,
    supersedes: response.supersedes
  };
}

export function historyMessageProtocol(row: JsonRecord, role: ChatMessage["role"]): Partial<ChatMessage> {
  const sourceID = text(row.node_id ?? row.id);
  return {
    source_id: sourceID,
    lifecycle: normalizeLifecycle(row.lifecycle, durableLifecycle),
    persistence: normalizePersistence(row.persistence, projectPersistence),
    message_kind: normalizeMessageKind(row.message_kind, role === "user" ? "user" : "assistant"),
    turn_id: text(row.turn_id ?? row.run_id ?? row.goal_id),
    logical_message_id: text(row.logical_message_id) || sourceID,
    supersedes: stringArray(row.supersedes)
  };
}

export function messageProtocolIdentityKeys(message: ChatMessage): string[] {
  const keys: string[] = [];
  if (message.logical_message_id) {
    keys.push(`logical:${message.logical_message_id}`);
  }
  if (message.source_id) {
    keys.push(`source:${message.source_id}`);
  }
  const kind = inferMessageKind(message);
  if (kind === "proposal") {
    const content = normalizeContent(message.content);
    if (content) {
      keys.push(`proposal:${message.role}:${content}`);
    }
  }
  return keys;
}

export function proposalActionIdentity(value: unknown): string {
  const action = record(value);
  const payload = record(action.payload ?? action.data);
  const presentation = record(payload.proposal_presentation ?? action.proposal_presentation);
  return text(
    presentation.proposal_id ??
      payload.proposal_id ??
      action.proposal_id ??
      payload.plan_id ??
      action.plan_id
  );
}

export function mergeMessageCollections(
  current: ChatMessage[],
  incoming: ChatMessage[],
  keysForMessage: (message: ChatMessage) => string[],
  mergeMessages: (existing: ChatMessage, incoming: ChatMessage) => ChatMessage
): ChatMessage[] {
  const seen = new Map<string, number>();
  const merged: ChatMessage[] = [];
  for (const message of [...current, ...incoming]) {
    const keys = keysForMessage(message);
    const existingIndex = keys.map((key) => seen.get(key)).find((index) => index !== undefined);
    if (existingIndex !== undefined) {
      merged[existingIndex] = mergeMessages(merged[existingIndex], message);
      keysForMessage(merged[existingIndex]).forEach((key) => seen.set(key, existingIndex));
      continue;
    }
    keys.forEach((key) => seen.set(key, merged.length));
    merged.push(message);
  }
  return merged.sort((left, right) => left.createdAt - right.createdAt);
}

export function resolveSupersededMessages(messages: ChatMessage[]): ChatMessage[] {
  const superseded = new Set(messages.flatMap((message) => message.supersedes ?? []).filter(Boolean));
  if (superseded.size === 0) {
    return messages;
  }
  return messages.map((message) => {
    if (!message.logical_message_id || !superseded.has(message.logical_message_id) || !message.actions?.length) {
      return message;
    }
    return {
      ...message,
      actions: message.actions.map((actionValue) => {
        const action = record(actionValue);
        if (!isProposalAction(action)) {
          return action;
        }
        return {
          ...action,
          status: "completed",
          stage: "completed",
          resolved_action_id: "superseded",
          actions: []
        };
      })
    };
  });
}

// Project History v1 appends the execution receipt but does not rewrite the
// earlier interactive node. When a UI state refresh restores both rows, an old
// waiting_for_user interaction must be treated as consumed. This includes
// staged selectors such as B4 plug-in selection, not only Proposal approval.
// The receipt must be later in the same turn so a genuinely new interaction in
// a multi-stage turn remains actionable.
export function resolveCompletedTurnProposals(messages: ChatMessage[]): ChatMessage[] {
  const lastTerminalIndexByTurn = new Map<string, number>();
  messages.forEach((message, index) => {
    const turnID = text(message.turn_id);
    const kind = inferMessageKind(message);
    if (turnID && (kind === "execution_receipt" || kind === "verification" || kind === "error")) {
      lastTerminalIndexByTurn.set(turnID, index);
    }
  });
  if (lastTerminalIndexByTurn.size === 0) {
    return messages;
  }
  return messages.map((message, index) => {
    const turnID = text(message.turn_id);
    const terminalIndex = turnID ? lastTerminalIndexByTurn.get(turnID) : undefined;
    if (terminalIndex === undefined || terminalIndex <= index || !message.actions?.some(isTurnBoundInteractionAction)) {
      return message;
    }
    const terminalKind = inferMessageKind(messages[terminalIndex]);
    const resolvedStatus = terminalKind === "error" ? "failed" : "completed";
    return {
      ...message,
      actions: message.actions.map((actionValue) => {
        const action = record(actionValue);
        if (!isTurnBoundInteractionAction(action)) {
          return action;
        }
        return {
          ...action,
          status: resolvedStatus,
          stage: resolvedStatus,
          resolved_action_id: "turn_terminal_receipt",
          actions: []
        };
      })
    };
  });
}

function isTurnBoundInteractionAction(value: unknown): boolean {
  if (isProposalAction(value)) {
    return true;
  }
  const action = record(value);
  const kind = text(action.kind ?? action.type).toLowerCase();
  if (["project_result", "mixboard", "mix_board"].includes(kind)) {
    return false;
  }
  const status = text(action.status ?? action.stage).toLowerCase();
  const childActions = Array.isArray(action.actions) ? action.actions : [];
  return childActions.length > 0 && (
    status === "" ||
    ["waiting_for_user", "waiting_confirmation", "waiting_clarification", "pending", "requested"].includes(status)
  );
}

// FIX-CONFIRM-CARD-1 ②：交互卡"已结算"判据——子按钮被摘除即不可再点击，
// 无论 status 字段处于何种形态（resolveInteractionInMessages / guard 盖章 /
// superseded 结算都遵循"结算必摘按钮"的写入口径，这里按同一口径读）。
function isSettledInteractionAction(value: unknown): boolean {
  const action = record(value);
  const childActions = Array.isArray(action.actions) ? action.actions : [];
  if (childActions.length === 0) {
    return true;
  }
  const status = text(action.status ?? action.stage).toLowerCase();
  return Boolean(status) && ["resolved", "completed", "complete", "cancelled", "canceled", "failed", "superseded"].includes(status);
}

export const TURN_EXPIRED_RESOLVED_ACTION = "turn_expired";
export const SUPERSEDED_BY_NEWER_RESOLVED_ACTION = "superseded_by_newer";

function settleInteractionAction(action: JsonRecord, resolvedActionID: string): JsonRecord {
  return {
    ...action,
    status: "completed",
    stage: "completed",
    resolved_action_id: resolvedActionID,
    actions: []
  };
}

// FIX-CONFIRM-CARD-1 ②：回合终结（完成/失败/停止）时，该回合内仍未应答的
// 交互卡立即转入不可交互终态——不再残留可点击的"过期"卡（点击才揭示
// server 4022 的缺陷形态）。与 resolveCompletedTurnProposals 的
// turn_terminal_receipt（按终局回执消息推导）互补：这里按事件流的回合终
// 结信号驱动，覆盖终局无回执消息入流的形态（用户停Turn、失败无消息等）。
// waiting_continue 等切片驻留态不是终结，不在本函数口径内（见
// terminalTurnIDsFromEvents 的状态过滤）。
export function settleTerminatedTurnInteractions(messages: ChatMessage[], turnIDs: string[]): ChatMessage[] {
  const turns = new Set(turnIDs.map((id) => text(id)).filter(Boolean));
  if (turns.size === 0 || messages.length === 0) {
    return messages;
  }
  let changed = false;
  const next = messages.map((message) => {
    if (!message.actions?.length || !turns.has(text(message.turn_id))) {
      return message;
    }
    let messageChanged = false;
    const actions = message.actions.map((actionValue) => {
      const action = record(actionValue);
      if (!isTurnBoundInteractionAction(action) || isSettledInteractionAction(action)) {
        return actionValue;
      }
      messageChanged = true;
      changed = true;
      return settleInteractionAction(action, TURN_EXPIRED_RESOLVED_ACTION);
    });
    return messageChanged ? { ...message, actions } : message;
  });
  return changed ? next : messages;
}

// 同族交互键：通用确认/mix 单步按 plan_id；mix_tick/mix_treatment 每会话同
// 时至多一张待确认卡（pendingMixTickForConversation 口径），按 workflow 归族。
function interactionFamilyKey(value: unknown): string {
  const action = record(value);
  if (text(action._ui_source) !== "interaction") {
    return "";
  }
  const payload = record(action.payload ?? action.data);
  const planID = text(payload.plan_id ?? action.plan_id);
  if (planID) {
    return "plan:" + planID;
  }
  const workflow = text(action.workflow ?? payload.workflow).toLowerCase();
  if (workflow === "mix_tick" || workflow === "mix_treatment") {
    return "workflow:" + workflow;
  }
  return "";
}

// FIX-CONFIRM-CARD-1 ②"被替代"治理：同族（同 plan / 同 mix workflow）出现
// 更新的待应答交互卡时，旧卡立即结算为"已替代"终态。旧的服务端交互 id 已
// 随替代失效（点击命中 4022 过期口径），呈现面不得再提供可点击形态。
export function settleSupersededInteractionFamilies(messages: ChatMessage[]): ChatMessage[] {
  const latestByFamily = new Map<string, string>();
  messages.forEach((message, messageIndex) => {
    (message.actions ?? []).forEach((actionValue, actionIndex) => {
      const key = interactionFamilyKey(actionValue);
      if (!key || isSettledInteractionAction(actionValue)) {
        return;
      }
      if (!isTurnBoundInteractionAction(actionValue)) {
        return;
      }
      latestByFamily.set(key, `${messageIndex}:${actionIndex}`);
    });
  });
  if (latestByFamily.size === 0) {
    return messages;
  }
  let changed = false;
  const next = messages.map((message, messageIndex) => {
    if (!message.actions?.length) {
      return message;
    }
    let messageChanged = false;
    const actions = message.actions.map((actionValue, actionIndex) => {
      const action = record(actionValue);
      const key = interactionFamilyKey(action);
      if (!key || isSettledInteractionAction(action) || !isTurnBoundInteractionAction(action)) {
        return actionValue;
      }
      if (latestByFamily.get(key) === `${messageIndex}:${actionIndex}`) {
        return actionValue;
      }
      messageChanged = true;
      changed = true;
      return settleInteractionAction(action, SUPERSEDED_BY_NEWER_RESOLVED_ACTION);
    });
    return messageChanged ? { ...message, actions } : message;
  });
  return changed ? next : messages;
}

const TERMINAL_TURN_EVENT_TYPES = new Set([
  "turn.completed", "turn.failed", "turn.stopped",
  "trajectory.turn.completed", "trajectory.turn.failed", "trajectory.turn.stopped"
]);
const TERMINAL_TURN_STATUS_TOKENS = ["completed", "complete", "failed", "stopped", "cancelled", "canceled"];

// FIX-CONFIRM-CARD-1 ②：从事件流提取"回合真终结"的回合 id。turn.completed
// 携带 waiting_continue/waiting_interaction 状态时是切片驻留（回合仍开放，
// 待应答卡仍有效），不属终结；仅完成/失败/停止口径入列。
export function terminalTurnIDsFromEvents(events: AgentEvent[]): string[] {
  const ids: string[] = [];
  const seen = new Set<string>();
  for (const event of events ?? []) {
    const type = text(event?.type);
    if (!TERMINAL_TURN_EVENT_TYPES.has(type)) {
      continue;
    }
    const status = text(event?.status).toLowerCase();
    if (Boolean(status) && !TERMINAL_TURN_STATUS_TOKENS.some((token) => status.includes(token))) {
      continue;
    }
    const turnID = eventTurnID(event);
    if (!turnID || seen.has(turnID)) {
      continue;
    }
    seen.add(turnID);
    ids.push(turnID);
  }
  return ids;
}

export function eventTurnID(event: AgentEvent): string {
  return text(event.turn_id ?? event.run_id ?? event.goal_id);
}

/**
 * AUDITION-LANE-1（2026-09-14）：活动行的回合归属键——与轨迹/判定卡同口径
 * （trajectoryTurnIdOfEvent / audition.ts:93：source_turn_id 优先）。C0 双写后
 * trajectory 系事件的 turn_id 还是实验域而轮次键已是 run 域，活动行按旧
 * eventTurnID 归属就跨命名空间判 unbound，落流底 lane 且轮次终局清退匹配不上。
 * audition.* 事件不带任何事件级 turn 字段（服务端 emitAuditionEvent 不设
 * GoalID/RunID/TurnID），回合域只在会话快照 payload.session.turn_id——按
 * audition.ts:93 同键回退。其余事件保持既有 eventTurnID 链（旧流行为不变）。
 */
export function activityTurnIDOfEvent(event: AgentEvent): string {
  const sourceTurn = text(event.source_turn_id);
  if (sourceTurn) {
    return sourceTurn;
  }
  const transportTurn = eventTurnID(event);
  if (transportTurn || !text(event.type).startsWith("audition.")) {
    return transportTurn;
  }
  const payload = record(event.payload);
  return text(record(payload.session).turn_id) || text(payload.turn_id);
}

/**
 * WEBUI-MSG-ORDER-2（2026-10-01，M1 复验活态钉尾取证）：内核 audition 会话 id
 * 是会话起源的规范编码——`audition:turn:<原生轨迹域 turn>:round-<n>-<hash>`。
 * 内核 audition::Session 无 turn 字段（2026-09-05 fixture 实证、M1 复验事件流
 * 八条 audition.* 事件 turn 域全空再证），事件面拿不到回合归属；这个编码是
 * webui 侧唯一的会话→回合证据。解析出原生域（补回 `turn:` 前缀，与轨迹事件
 * payload.turn_id 同形），再由消费方经 trajectory 的 nativeTurnIds 账映射到
 * 拥有它的 B9 轮次。不匹配编码的会话 id 返回空串（无证据不猜）。
 */
export function auditionSessionNativeTurnID(sessionID: string): string {
  const parts = text(sessionID).split(":");
  if (parts.length < 4 || parts[0] !== "audition" || parts[1] !== "turn" || !parts[2]) {
    return "";
  }
  return `turn:${parts[2]}`;
}

export function eventLogicalMessageID(event: AgentEvent): string {
  return text(event.logical_message_id) || [eventTurnID(event), text(event.item_id), text(event.type)].filter(Boolean).join(":");
}

export function reduceAgentEventActivities(
  current: ChatMessage[],
  events: AgentEvent[],
  factory: (event: AgentEvent) => ChatMessage | null
): ChatMessage[] {
  let next = current.filter(isTransientMessage);
  for (const event of events) {
    const eventType = text(event.type);
    const turnID = activityTurnIDOfEvent(event);
    const logicalID = eventLogicalMessageID(event);
    // GUI-1：轨迹回合终结事件与顶层 turn 终结同等清场——否则链终局清场后
    // 到达的 trajectory.turn.completed 会再产一条「已完成」残留活动（15 事件
    // fixture 回放实证：seq15 在 seq14 清场后经 factory 产出残留）。
    if (
      eventType === "turn.completed" || eventType === "turn.failed" || eventType === "turn.stopped" ||
      eventType === "trajectory.turn.completed" || eventType === "trajectory.turn.failed" || eventType === "trajectory.turn.stopped"
    ) {
      next = dismissActivitiesForTurn(next, turnID);
      continue;
    }
    if (eventType === "approval.requested") {
      next = dismissActivity(next, logicalID, event);
      continue;
    }
    if (eventType === "item.completed" && isCompletedStatus(event.status)) {
      next = dismissActivity(next, logicalID, event);
      continue;
    }
    // SETTLE-DELIVER-1 症状 A：判定结算确认（settlement_reply）是正式助手消息
    // （settlementMessagesFromEvents 路由进 messages），活动线不再重复承载。
    if (eventType === "judgment.settled" && Boolean(record(event.payload).settlement_reply)) {
      continue;
    }
    const created = factory(event);
    if (!created) {
      continue;
    }
    let activityTurn = turnID;
    if (!activityTurn && eventType.startsWith("audition.")) {
      // AUDITION-LANE-1 会话回合记忆：真栈形态下 telemetry 类 audition 事件
      // （prepare.started/candidate.ready）的会话快照不带 turn_id，只有 agent 侧
      // 发出的事件带（2026-09-05 fixture 实证）——晚到事件继承同会话（同
      // source_id）已绑定的回合域，audition.ts:93 previous?.turnID 的活动侧对应。
      activityTurn = next.find((row) => row.source_id === created.source_id && text(row.turn_id))?.turn_id ?? "";
    }
    const message = transientMessage({
      ...created,
      turn_id: created.turn_id || activityTurn,
      logical_message_id: created.logical_message_id || logicalID
    });
    if (activityTurn && eventType.startsWith("audition.")) {
      // 回填：携带回合域的事件到达前，同会话早到活动已以空回合入账——不回填的话
      // 半族活动仍按 unbound 落流底 lane，整族同键才算归属干净。
      next = next.map((row) =>
        row.source_id === message.source_id && !text(row.turn_id) ? { ...row, turn_id: activityTurn } : row
      );
    }
    next = upsertActivity(next, message);
  }
  return next.sort((left, right) => left.createdAt - right.createdAt);
}

export function upsertActivity(current: ChatMessage[], incoming: ChatMessage): ChatMessage[] {
  const message = transientMessage(incoming);
  const key = activityKey(message);
  const index = current.findIndex((item) => activityKey(item) === key);
  if (index < 0) {
    return [...current, message];
  }
  return current.map((item, itemIndex) => itemIndex === index ? { ...item, ...message, createdAt: item.createdAt } : item);
}

export function dismissActivityByID(current: ChatMessage[], messageID: string): ChatMessage[] {
  return current.filter((message) => message.id !== messageID && message.logical_message_id !== messageID && message.source_id !== messageID);
}

export function dismissActivitiesForTurn(current: ChatMessage[], turnID: string): ChatMessage[] {
  if (!turnID) {
    return [];
  }
  return current.filter((message) => message.turn_id !== turnID);
}

function dismissActivity(current: ChatMessage[], logicalID: string, event: AgentEvent): ChatMessage[] {
  const itemID = text(event.item_id);
  return current.filter((message) => {
    if (logicalID && message.logical_message_id === logicalID) {
      return false;
    }
    if (itemID && (message.source_id ?? message.id).includes(itemID)) {
      return false;
    }
    return true;
  });
}

function responseHistoryIdentity(response: ChatResponse, content: string): { sourceID: string; logicalMessageID: string } {
  const history = record(response.project_history);
  const rows = Array.isArray(history.conversation_messages) ? history.conversation_messages.map(record) : [];
  const expected = normalizeContent(content);
  for (let index = rows.length - 1; index >= 0; index -= 1) {
    const row = rows[index];
    const role = text(row.role ?? row.kind).toLowerCase();
    const rowContent = normalizeContent(text(row.content ?? row.text ?? row.text_preview));
    if (role !== "assistant" && role !== "vit") {
      continue;
    }
    if (expected && rowContent !== expected) {
      continue;
    }
    return {
      sourceID: text(row.node_id ?? row.id),
      logicalMessageID: text(row.logical_message_id)
    };
  }
  return { sourceID: "", logicalMessageID: "" };
}

function isProposalAction(value: unknown): boolean {
  const action = record(value);
  const payload = record(action.payload ?? action.data);
  const kind = text(action.kind ?? action.type).toLowerCase();
  return kind === "confirmation" ||
    kind === "proposal_approval" ||
    Boolean(action._synthetic_confirmation) ||
    Object.keys(record(payload.proposal_presentation ?? action.proposal_presentation)).length > 0;
}

function activityKey(message: ChatMessage): string {
  return message.logical_message_id || message.source_id || message.id;
}

function isCompletedStatus(value: unknown): boolean {
  const status = text(value).toLowerCase();
  return status === "" || ["completed", "complete", "done", "ok", "success", "succeeded", "applied"].includes(status);
}

function normalizeLifecycle(value: unknown, fallback: MessageLifecycle): MessageLifecycle {
  return text(value) === transientLifecycle ? transientLifecycle : fallback;
}

function normalizePersistence(value: unknown, fallback: MessagePersistence): MessagePersistence {
  const normalized = text(value);
  return normalized === noPersistence || normalized === localPersistence || normalized === projectPersistence ? normalized : fallback;
}

function normalizeMessageKind(value: unknown, fallback: MessageKind): MessageKind {
  const normalized = text(value) as MessageKind;
  return ["activity", "user", "assistant", "proposal", "execution_receipt", "verification", "warning", "error", "system"].includes(normalized)
    ? normalized
    : fallback;
}

function stringArray(value: unknown): string[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const values = value.map(text).filter(Boolean);
  return values.length > 0 ? values : undefined;
}

function normalizeContent(value: string): string {
  return value.trim().replace(/\s+/g, " ");
}

function record(value: unknown): JsonRecord {
  return value && typeof value === "object" && !Array.isArray(value) ? value as JsonRecord : {};
}

function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : value == null ? "" : String(value).trim();
}
