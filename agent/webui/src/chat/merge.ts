// T9-PRE-1A-1（施工图 UI-CONVERGE-RECON-1 §3.1 批 1 / 步 1b-ii）：App.tsx 消息归并层（R 域 merge 段）出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
// 与 ./storage 互引为既有双向耦合（两侧均为函数声明+函数体内取值，ESM 安全）。
// App 侧引用的 15 个判定/payload 函数（isMixBoardAction/isComposerInteraction/confirmation* 等）
// 属动作卡域（步 1e 落点），经 App 兼容层引用（函数声明，循环安全），1e 移动后剪断。

import type { AgentUIState, ChatMessage, ChatResponse, JsonRecord } from "../types";
import {
  durableMessage,
  historyMessageProtocol,
  inferMessageKind,
  isDurableMessage,
  mergeMessageCollections,
  messageProtocolIdentityKeys,
  proposalActionIdentity,
  resolveCompletedTurnProposals,
  resolveSupersededMessages,
  settleSupersededInteractionFamilies
} from "../messageLifecycle";
import { isConsumedInteractionStatus } from "../interactionGuard";
import { asRecord, firstArray, textValue, truthy, uniqueID } from "../lib/util";
import { hasMeaningfulChatMessages, initialMessage, messagesOrIntro } from "./storage";
import {
  actionRenderID,
  artifactSummariesFrom,
  confirmationFallbackActionsFromResponse,
  interactionActionsFrom,
  isCapabilityProposalInteraction,
  isComposerInteraction,
  isConfirmationAction,
  isDisposableConfirmationPromptMessage,
  isDisposableConfirmationPromptText,
  isMixBoardAction,
  isPendingInteractionAction,
  isProjectResultAction,
  mergeArtifacts,
  resolveInteractionInMessages,
  interactionPayload
} from "../App";

// VITNOTE-NOTESTREAM-2：切换到 note 会话时的消息种子——note 问答不经 webui 发送
//（本地桶无消息、/agent/events 无轨迹），从 note_sessions 投影行回放问答对；非
// note 会话/无投影行回落常规开机问候（既有恢复效应接管）。fail-open：异形行按空处理。
export function noteSessionSeedMessages(targetConversationID: string, noteSessions: JsonRecord[] | undefined): ChatMessage[] {
  const intro = () => [{ ...initialMessage, id: uniqueID("intro"), createdAt: Date.now() }];
  const target = targetConversationID.trim();
  if (!target || !Array.isArray(noteSessions)) {
    return intro();
  }
  const row = noteSessions.find((candidate) => textValue(candidate.conversation_id, "") === target);
  if (!row) {
    return intro();
  }
  const messages: ChatMessage[] = [];
  for (const raw of firstArray(row.messages)) {
    const record = asRecord(raw);
    const content = textValue(record.content ?? record.text, "");
    const roleText = textValue(record.role, "").toLowerCase();
    if (!content || (roleText !== "user" && roleText !== "assistant")) {
      continue;
    }
    const created = Date.parse(textValue(record.created_at, ""));
    messages.push({
      id: uniqueID("vitnote"),
      role: roleText === "user" ? "user" : "assistant",
      content,
      createdAt: Number.isFinite(created) ? created : Date.now(),
      lifecycle: "durable",
      persistence: "project_history"
    });
  }
  return messages.length > 0 ? messages : intro();
}

function historyMessagesFromUIState(uiState: AgentUIState | null): ChatMessage[] {
  const projectHistory = asRecord(uiState?.project_history);
  const rows = firstArray(projectHistory.conversation_messages);
  const artifacts = uiState?.artifacts ?? [];
  const messages: ChatMessage[] = [];
  rows.forEach((raw, index) => {
    const row = asRecord(raw);
    const content = textValue(row.content ?? row.text ?? row.text_preview, "");
    if (!content) {
      return;
    }
    const sourceID = textValue(row.node_id ?? row.id, `history_${index}`);
    const roleText = textValue(row.role ?? row.kind, "assistant").toLowerCase();
    const role: ChatMessage["role"] = roleText === "user" || roleText === "ask" ? "user" : "assistant";
    if (role === "assistant" && isDisposableConfirmationPromptText(content)) {
      return;
    }
    const created = Date.parse(textValue(row.created_at, ""));
    const messageData = asRecord(row.message_data);
    const workflowData = asRecord(messageData.workflow_data);
    const messageInteractions = interactionActionsFrom(messageData.interaction_requests);
    const historyResponse: ChatResponse = {
      reply: content,
      needs_confirmation: truthy(messageData.needs_confirmation),
      plan_id: textValue(messageData.plan_id, ""),
      preview: textValue(messageData.preview, ""),
      workflow: textValue(messageData.workflow, ""),
      workflow_data: workflowData,
      proposal_presentation: asRecord(messageData.proposal_presentation ?? workflowData.proposal_presentation),
      interaction_requests: firstArray(messageData.interaction_requests).map(asRecord),
      message_kind: textValue(row.message_kind, "") as ChatResponse["message_kind"],
      lifecycle: "durable",
      persistence: "project_history"
    };
    const historyFallbackActions = messageInteractions.some(isPendingInteractionAction)
      ? []
      : confirmationFallbackActionsFromResponse(historyResponse);
    const actions =
      role === "assistant"
        ? [
            ...messageInteractions,
            ...historyFallbackActions,
            ...firstArray(row.project_result_cards)
              .map(asRecord)
              .filter((card) => Object.keys(card).length > 0)
              .map((card, cardIndex) => ({
                ...card,
                _ui_source: "project_result",
                kind: "project_result",
                type: "project_result",
                id: textValue(card.id, `history_project_result_${sourceID}_${cardIndex}`)
              }))
          ]
        : [];
    const messageArtifacts = artifactSummariesFrom(row.artifacts);
    const protocol = historyMessageProtocol(row, role);
    const historyMessage = durableMessage({
      id: `history_${sourceID}`,
      role,
      content,
      artifacts: messageArtifacts,
      actions,
      createdAt: Number.isFinite(created) ? created : Date.now() + index,
      status: "sent",
      ...protocol
    }, { persistence: "project_history" });
    if (isDurableMessage(historyMessage)) {
      messages.push(historyMessage);
    }
  });
  return settleSupersededInteractionFamilies(resolveCompletedTurnProposals(resolveSupersededMessages(messages)));
}

function removeDisposableConfirmationPromptMessages(messages: ChatMessage[]): ChatMessage[] {
  return messages.filter((message) => !isDisposableConfirmationPromptMessage(message));
}

function mergeChatMessages(current: ChatMessage[], incoming: ChatMessage[]): ChatMessage[] {
  if (incoming.length === 0) {
    return current;
  }
  const base = current.length === 1 && current[0].id === "intro" ? [] : current;
  return settleSupersededInteractionFamilies(resolveCompletedTurnProposals(mergeMessageCollections(base, incoming, chatMessageKeys, mergeChatMessage)));
}

// FIX-BUCKET-SAVE-RACE-1：restore 合流核（自 restore 效应体抽出，供 reload 桶
// save/restore 管线的回归测试直接驱动）。现流无有效消息（裸启动只有问候克隆）
// 时以桶内容起流；否则桶内容并回现流（去重/收口语义同 mergeChatMessages）。
export function mergeRestoredChatMessages(current: ChatMessage[], storedMessages: ChatMessage[]): ChatMessage[] {
  if (!hasMeaningfulChatMessages(current)) {
    return messagesOrIntro(storedMessages);
  }
  return mergeChatMessages(storedMessages, current);
}

function mergeAssistantMessageIntoChat(current: ChatMessage[], incoming: ChatMessage): ChatMessage[] {
  return messageMixBoardAction(incoming) ? upsertPersistentMixBoardMessage(current, incoming) : mergeChatMessages(current, [incoming]);
}

function upsertPersistentMixBoardMessage(current: ChatMessage[], incoming: ChatMessage): ChatMessage[] {
  const incomingAction = messageMixBoardAction(incoming);
  if (!incomingAction) {
    return mergeChatMessages(current, [incoming]);
  }
  const incomingSessionID = mixBoardActionSessionID(incomingAction);
  let replaced = false;
  let inserted = false;
  const next = current.flatMap((message) => {
    const actions = (message.actions ?? []).map(asRecord);
    if (actions.length === 0) {
      return [message];
    }
    const mixActions = actions.filter(isMixBoardAction);
    if (mixActions.length === 0) {
      return [message];
    }
    const sessionMatches = mixActions.some((action) => isSameMixBoardSession(action, incomingAction, incomingSessionID));
    const shouldReplaceHere = !replaced && (sessionMatches || !incomingSessionID);
    if (!shouldReplaceHere) {
      const remaining = actions.filter((action) => !isMixBoardAction(action));
      if (remaining.length === 0 && !message.artifacts?.length && !message.content.trim()) {
        return [];
      }
      return [{ ...message, actions: remaining }];
    }
    replaced = true;
    inserted = true;
    const mergedActions = actions.map((action) => (isMixBoardAction(action) ? incomingAction : action));
    return [{
      ...message,
      content: incoming.content || message.content,
      mode: incoming.mode || message.mode,
      artifacts: mergeArtifacts(message.artifacts ?? [], incoming.artifacts ?? []),
      actions: mergedActions,
      status: incoming.status
    }];
  });
  if (inserted) {
    return next;
  }
  const stripped = next.filter((message) => {
    const actions = (message.actions ?? []).map(asRecord);
    return !actions.some(isMixBoardAction);
  });
  return mergeChatMessages(stripped, [incoming]);
}

function messageMixBoardAction(message: ChatMessage): JsonRecord | null {
  return firstArray(message.actions).map(asRecord).find(isMixBoardAction) ?? null;
}

function isSameMixBoardSession(action: JsonRecord, incomingAction: JsonRecord, incomingSessionID: string): boolean {
  if (!isMixBoardAction(action)) {
    return false;
  }
  const existingSessionID = mixBoardActionSessionID(action);
  if (incomingSessionID && existingSessionID) {
    return incomingSessionID === existingSessionID;
  }
  return actionRenderID(action) === actionRenderID(incomingAction);
}

function mixBoardActionSessionID(action: JsonRecord): string {
  const payload = interactionPayload(action);
  const session = asRecord(payload.mix_session ?? action.mix_session);
  const observationEnvelope = asRecord(payload.mix_observation ?? action.mix_observation);
  const observation = asRecord(observationEnvelope.observation);
  const mixboard = asRecord(observationEnvelope.mixboard);
  return textValue(
    session.mix_session_id ??
      observation.mix_session_id ??
      mixboard.mix_session_id ??
      payload.mix_session_id ??
      action.mix_session_id,
    ""
  );
}

function mergeChatMessage(existing: ChatMessage, incoming: ChatMessage): ChatMessage {
  return {
    ...incoming,
    ...existing,
    source_id: existing.source_id || incoming.source_id,
    lifecycle: existing.lifecycle === "durable" || incoming.lifecycle === "durable" ? "durable" : existing.lifecycle ?? incoming.lifecycle,
    persistence: existing.persistence === "project_history" || incoming.persistence === "project_history"
      ? "project_history"
      : existing.persistence ?? incoming.persistence,
    message_kind: inferMessageKind(existing) === "proposal" || inferMessageKind(incoming) === "proposal"
      ? "proposal"
      : existing.message_kind ?? incoming.message_kind,
    turn_id: existing.turn_id || incoming.turn_id,
    logical_message_id: existing.logical_message_id || incoming.logical_message_id,
    supersedes: Array.from(new Set([...(existing.supersedes ?? []), ...(incoming.supersedes ?? [])])),
    artifacts: mergeArtifacts(existing.artifacts ?? [], incoming.artifacts ?? []),
    actions: mergeMessageActions(existing.actions ?? [], incoming.actions ?? []),
    createdAt: Math.min(existing.createdAt, incoming.createdAt),
    status: existing.status === "error" || incoming.status === "error" ? "error" : existing.status ?? incoming.status
  };
}

export function mergeMessageActions(current: JsonRecord[], incoming: JsonRecord[]): JsonRecord[] {
  if (current.length === 0) {
    return incoming;
  }
  if (incoming.length === 0) {
    return current;
  }
  const byKey = new Map<string, JsonRecord>();
  const order: string[] = [];
  [...current, ...incoming].forEach((action, index) => {
    const key = actionMergeKey(action, index);
    const existing = byKey.get(key);
    if (existing) {
      byKey.set(key, mergeActionRecord(existing, action));
      return;
    }
    byKey.set(key, action);
    order.push(key);
  });
  return order.map((key) => byKey.get(key)).filter((action): action is JsonRecord => Boolean(action));
}

function mergeActionRecord(existing: JsonRecord, incoming: JsonRecord): JsonRecord {
  if (isComposerInteraction(existing) && !isComposerInteraction(incoming)) {
    return existing;
  }
  // FIX-CONFIRM-CARD-1 ②：已结算的交互动作不得从 incoming 复活子按钮。结算
  // 写入口径一律摘按钮（resolveInteractionInMessages / guard 盖章 / 回合终结
  // 与同族替代结算），existing.actions 为空是"已结算"而非"缺按钮数据"；旧
  // 回退取 incoming.actions 会把死卡复活成可点击形态（点击命中 server 4022
  // 过期口径——E2E K1/K2 实证：settle 后的二次同键合并与台账盖章版合并均
  // 触发）。已结算形态以 existing 为权威终稿。
  if (isSettledInteractionRecord(existing)) {
    return existing;
  }
  return {
    ...incoming,
    ...existing,
    actions: firstArray(existing.actions).length > 0 ? existing.actions : incoming.actions
  };
}

function isSettledInteractionRecord(action: JsonRecord): boolean {
  if (textValue(action._ui_source, "") !== "interaction") {
    return false;
  }
  if (textValue(action.resolved_action_id, "") !== "") {
    return true;
  }
  const status = textValue(action.status ?? action.stage, "").toLowerCase();
  return isConsumedInteractionStatus(status) ||
    status.includes("complete") ||
    status.includes("cancel") ||
    status.includes("fail");
}

function actionMergeKey(action: JsonRecord, index: number): string {
  if (isCapabilityProposalInteraction(action)) {
    const proposalID = proposalActionIdentity(action);
    if (proposalID) {
      return `proposal:${proposalID}`;
    }
  }
  return actionRenderID(action) || stableActionIdentity(action) || `${textValue(action._ui_source, "action")}:${index}`;
}

function stableActionIdentity(action: JsonRecord): string {
  const source = textValue(action._ui_source, "action");
  const payload = asRecord(action.result ?? action.payload);
  const tool = textValue(action.tool ?? payload.tool, "");
  const command = textValue(action.command_name ?? action.cmd ?? action.command ?? payload.command_name ?? payload.cmd ?? payload.command, "");
  const status = textValue(action.status ?? payload.status, "");
  const objectID = textValue(
    action.tool_call_id ??
      action.agent_action_id ??
      payload.tool_call_id ??
      payload.agent_action_id ??
      payload.track_id ??
      payload.clip_id ??
      payload.plugin_id ??
      payload.new_clip_id ??
      payload.created_clip_id,
    ""
  );
  if (objectID) {
    return `${source}:${tool || command || "action"}:${objectID}:${status}`;
  }
  const preview = textValue(
    action.preview ??
      action.body ??
      action.title ??
      payload.message ??
      payload.summary ??
      payload.preview,
    ""
  ).slice(0, 120);
  if (tool || command || status || preview) {
    return `${source}:${tool || command || "action"}:${status}:${preview}`;
  }
  return "";
}

function chatMessageKeys(message: ChatMessage): string[] {
  const actions = (message.actions ?? []).map(asRecord);
  const actionKeys = actions
    .filter(isMessageMergeKeyAction)
    .map((action) => actionRenderID(action))
    .filter(Boolean);
  const keys: string[] = messageProtocolIdentityKeys(message);
  if (message.id) {
    keys.push(`id:${message.id}`);
  }
  if (actionKeys.length > 0) {
    keys.push(`actions:${message.role}:${actionKeys.join("|")}`);
  }
  if (!actions.some(isProjectResultAction) && (actionKeys.length === 0 || inferMessageKind(message) === "proposal")) {
    keys.push(`text:${message.role}:${message.content}`);
  }
  return Array.from(new Set(keys));
}

function isMessageMergeKeyAction(action: JsonRecord): boolean {
  if (isProjectResultAction(action)) {
    return false;
  }
  const source = textValue(action._ui_source, "");
  return source === "interaction" || isComposerInteraction(action) || isConfirmationAction(action);
}

export {
  historyMessagesFromUIState,
  removeDisposableConfirmationPromptMessages,
  mergeChatMessages,
  mergeAssistantMessageIntoChat,
  upsertPersistentMixBoardMessage,
  messageMixBoardAction,
  isSameMixBoardSession,
  mixBoardActionSessionID,
  mergeChatMessage,
  mergeActionRecord,
  isSettledInteractionRecord,
  actionMergeKey,
  stableActionIdentity,
  chatMessageKeys,
  isMessageMergeKeyAction
};
