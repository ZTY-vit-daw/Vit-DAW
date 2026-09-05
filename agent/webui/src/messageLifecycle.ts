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

export function eventTurnID(event: AgentEvent): string {
  return text(event.turn_id ?? event.run_id ?? event.goal_id);
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
    const turnID = eventTurnID(event);
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
    const created = factory(event);
    if (!created) {
      continue;
    }
    const message = transientMessage({
      ...created,
      turn_id: created.turn_id || turnID,
      logical_message_id: created.logical_message_id || logicalID
    });
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
