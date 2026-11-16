// T9-PRE-1A-1（施工图 UI-CONVERGE-RECON-1 §3.1 批 1 / 步 1b-i）：App.tsx 会话存储/身份层（R 域 storage 段）出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
// 附带迁移：头部常量 conversationMessagesStoragePrefix/conversationScopedIDStoragePrefix 与
// initialMessage（storage/merge 两域共用的开机问候种子，App 其余使用经兼容层引回）。
// hasPendingComposerInteraction/artifactSummariesFrom 属 App 动作卡/响应解析域（1e/2b 落点），
// 经 App 兼容层引用（函数声明，循环安全）；与 ./merge 互引为既有双向耦合（storage↔merge），
// 两侧均为函数声明，ESM 实例化期初始化，安全。

import type { AgentMode, ChatMessage, JsonRecord } from "../types";
import type { HistoryScopeChangeKind, HistoryScopeParts } from "../historyScope";
import { durableMessage, durableMessagesForStorage, isDurableMessage, isLegacyTransientMessage } from "../messageLifecycle";
import { appendChainResultMessages, isChainResultChatMessage } from "../trace/traceDelivery";
import { loadTurnReceipts, mergeTurnReceipts, persistTurnReceipts } from "../trace/turnReceipts";
import { asRecord, firstArray, textValue, stableIDPart, uniqueID } from "../lib/util";
import { mergeChatMessages, removeDisposableConfirmationPromptMessages } from "./merge";
import { artifactSummariesFrom, hasPendingComposerInteraction } from "../App";

const conversationMessagesStoragePrefix = "ask_vit_conversation_messages";
const conversationScopedIDStoragePrefix = "ask_vit_conversation_id_scope";


const initialMessage: ChatMessage = {
  id: "intro",
  role: "assistant",
  content: "Ask Vit 就绪。",
  createdAt: Date.now(),
  lifecycle: "durable",
  persistence: "local",
  message_kind: "assistant"
};



// CONTRACT-3/C3：history-sync 的消息解析纯函数（恢复优先级统一实现）——
// 任务状态以 runtime snapshot 为权威、过程轨迹以事件流为权威、终局以
// Project History 为恢复权威：
// - switch：真工程切换照旧清流（回落问候语），旧工程终局不带入新工程；
// - initial：刷新/首锚以 Project History 为恢复权威并幂等带回事件路径终局
//   （GUI-1 ② 语义）；但空 Project History 不得覆盖已恢复/已回放的终局流
//   （C3：刷新后终局消息不消失——终局由服务端事件重放先行交付时锚定滞后）；
// - evolution/none：同会话演进只合并（保留在流消息与事件 seq 投影）。
export function resolveHistorySyncMessages(input: {
  changeKind: HistoryScopeChangeKind;
  current: ChatMessage[];
  historyMessages: ChatMessage[];
}): ChatMessage[] {
  const hasPendingInteraction = hasPendingComposerInteraction(input.current);
  const baseMessages = hasPendingInteraction ? input.current : removeDisposableConfirmationPromptMessages(input.current);
  if (input.changeKind === "switch") {
    return messagesOrIntro(input.historyMessages);
  }
  if (input.changeKind === "initial") {
    if (!hasMeaningfulChatMessages(input.historyMessages) && hasMeaningfulChatMessages(input.current)) {
      return input.current;
    }
    return messagesOrIntro(appendChainResultMessages(input.historyMessages, input.current.filter(isChainResultChatMessage)));
  }
  return mergeChatMessages(baseMessages, input.historyMessages);
}

// CONTRACT-3：演进后重新锚定存储桶——把旧 scope 桶里的已存消息合并进新桶，
// 会话 id 一并迁移（旧桶保留作回退，不删除）。
export function migrateStoredConversationScope(previousScope: string, nextScope: string, conversationID: string): void {
  if (!previousScope || !nextScope || previousScope === nextScope) {
    return;
  }
  const continuingConversationID = conversationID || loadStoredScopedConversationID(previousScope);
  if (!continuingConversationID) {
    return;
  }
  // CONV-ID-BOOT-1（2026-09-14）：新桶已锚定到「另一个会话」时不得覆写锚定。
  // 首拍 scope 未物化造出的随机会话 id 走到这里，会把真实 scope 桶里用户
  // 上一会话的锚整个抹掉——刷新后裸启动再也回不到原会话（轨迹块/A-B 卡/
  // 收据全灭）。空桶或同值重写照常落锚；消息与台账按 (会话 id, scope) 分桶
  // 迁移，不碰被锚定会话自己的数据。
  const existingNextAnchor = loadStoredScopedConversationID(nextScope);
  if (!existingNextAnchor || existingNextAnchor === continuingConversationID) {
    saveStoredScopedConversationID(nextScope, continuingConversationID);
  }
  // TRAJ-IMPL-3：回执行台账与消息存档同一条迁移语义（scope 演进 = 同一会话换桶，
  // 旧桶保留作回退不删除）。漏了这一步，scope 演进后的刷新会看不见演进前的回执行。
  const previousReceipts = loadTurnReceipts(continuingConversationID, previousScope);
  if (previousReceipts.length > 0) {
    const mergedReceipts = mergeTurnReceipts(loadTurnReceipts(continuingConversationID, nextScope), previousReceipts);
    persistTurnReceipts(continuingConversationID, nextScope, mergedReceipts);
  }
  const previousMessages = loadStoredConversationMessages(continuingConversationID, previousScope);
  if (previousMessages.length === 0) {
    return;
  }
  const existingMessages = loadStoredConversationMessages(continuingConversationID, nextScope);
  const merged = mergeStoredScopeMessages(existingMessages, previousMessages);
  if (merged.length > 0) {
    saveStoredConversationMessages(continuingConversationID, nextScope, merged);
  }
}

function mergeStoredScopeMessages(existingMessages: ChatMessage[], previousMessages: ChatMessage[]): ChatMessage[] {
  return durableMessagesForStorage(mergeChatMessages(existingMessages, previousMessages)).filter(
    (message) => message.id !== "intro"
  );
}

function messagesOrIntro(messages: ChatMessage[]): ChatMessage[] {
  if (messages.length > 0) {
    return messages;
  }
  return [{ ...initialMessage, id: uniqueID("intro"), createdAt: Date.now() }];
}

export function hasMeaningfulChatMessages(messages: ChatMessage[]): boolean {
  return messages.some((message) => {
    // CONV-ID-BOOT-1（2026-09-14）：开机问候由 messagesOrIntro 克隆成
    // intro_<ts>_<rand>（不是整串 "intro"）。旧排除只匹配整串 id，问候被计成
    // 有效消息，演进拍的采纳守卫（不劫持现役会话）因此在裸启动上永远误触：
    // 存档锚定不被回读，fall-through 迁移把真实 scope 桶覆写成新随机会话 id
    // ——刷新后轨迹块/A-B 卡/收据全灭的独占根因。
    if (message.role === "assistant" && (message.id === "intro" || message.id.startsWith("intro_"))) {
      return false;
    }
    return textValue(message.content, "").trim() !== "" || (message.actions?.length ?? 0) > 0 || (message.artifacts?.length ?? 0) > 0;
  });
}

function createConversationID(): string {
  return `webui_${Date.now().toString(36)}`;
}

function conversationScopedIDStorageKey(scope: string): string {
  return `${conversationScopedIDStoragePrefix}:${stableIDPart(scope)}`;
}

function scopedConversationRuntimeKey(scope: string, conversationID: string): string {
  return `${stableIDPart(scope)}:${stableIDPart(conversationID || "latest")}`;
}

export function loadStoredScopedConversationID(scope: string): string {
  if (typeof window === "undefined" || !scope) {
    return "";
  }
  try {
    return window.localStorage.getItem(conversationScopedIDStorageKey(scope))?.trim() ?? "";
  } catch {
    return "";
  }
}

export function saveStoredScopedConversationID(scope: string, conversationID: string): void {
  if (typeof window === "undefined" || !scope || !conversationID) {
    return;
  }
  try {
    window.localStorage.setItem(conversationScopedIDStorageKey(scope), conversationID);
  } catch {
    // Scoped local history is best-effort; the backend Project History remains authoritative.
  }
}

// REFRESH-VANISH-2（2026-09-15）：空 storage 裸开（Godot webview 面板销毁重建丢
// localStorage 的用户形态）时的服务端会话身份兜底。服务端没有「当前会话」直读面，
// 但 runtimeStatus.continuations 投影行携带 conversation_id + project_path/uuid +
// updated_at——按 scope 匹配取最新行即得权威会话身份；事件缓冲（/agent/events）按
// 该 id 键控，采纳它即可经 since=0 全量回放重建轨迹块/A-B 卡（消息面走 uiState
// 服务端历史恢复、不依赖本地 storage，这正是「消息在、轨迹无」的分裂成因）。
export function serverConversationIDFromContinuations(continuations: unknown, parts: HistoryScopeParts): string {
  if (!Array.isArray(continuations)) {
    return "";
  }
  const scopePath = normalizeServerContinuationPath(parts.projectPath || parts.rootProjectPath);
  const scopeUUID = (parts.projectUUID || "").trim().toLowerCase();
  if (!scopePath && !scopeUUID) {
    return "";
  }
  let bestID = "";
  let bestAt = "";
  for (const raw of continuations) {
    const row = asRecord(raw);
    const conversationID = textValue(row.conversation_id, "").trim();
    if (!conversationID) {
      continue;
    }
    const rowPath = normalizeServerContinuationPath(row.project_path);
    const rowUUID = textValue(row.project_uuid, "").trim().toLowerCase();
    const scopeMatched = (scopePath !== "" && rowPath === scopePath) || (scopeUUID !== "" && rowUUID === scopeUUID);
    if (!scopeMatched) {
      continue;
    }
    const updatedAt = textValue(row.updated_at, "");
    // 同刻并列时按会话 id 决胜：选择只由数据决定，不依赖服务端列表顺序。
    if (!bestID || updatedAt > bestAt || (updatedAt === bestAt && conversationID > bestID)) {
      bestID = conversationID;
      bestAt = updatedAt;
    }
  }
  return bestID;
}

function normalizeServerContinuationPath(value: unknown): string {
  return textValue(value, "").trim().replace(/\\/g, "/").replace(/\/+$/, "").toLowerCase();
}

// REFRESH-VANISH-2：scope 会话身份的统一解析链（initial/switch 引导与 evolution
// 锚定候选共用）。优先级：URL 绑定 > 本地锚定 > 服务端兜底 > 新建。服务端兜底
// 只在本地映射缺失时生效（storage 正常形态零行为变化）；evolution 调用侧传
// url="" fresh=""，把链退化为「本地锚定 || 服务端兜底」的锚定候选。
export function resolveScopedConversationID(input: {
  urlConversationID: string;
  storedScopedConversationID: string;
  serverConversationID: string;
  freshConversationID: string;
}): string {
  for (const value of [input.urlConversationID, input.storedScopedConversationID, input.serverConversationID, input.freshConversationID]) {
    const trimmed = value.trim();
    if (trimmed) {
      return trimmed;
    }
  }
  return "";
}

function conversationMessagesStorageKey(conversationID: string, scope = ""): string {
  const id = stableIDPart(conversationID || "latest");
  const scoped = scope ? `:${stableIDPart(scope)}` : "";
  return `${conversationMessagesStoragePrefix}:${id}${scoped}`;
}

export function loadStoredConversationMessages(conversationID: string, scope = ""): ChatMessage[] {
  if (typeof window === "undefined" || !conversationID || !scope) {
    return [];
  }
  try {
    const raw = window.localStorage.getItem(conversationMessagesStorageKey(conversationID, scope));
    if (!raw) {
      return [];
    }
    const parsed = JSON.parse(raw);
    return firstArray(asRecord(parsed).messages ?? parsed).map(sanitizeStoredChatMessage).filter(Boolean) as ChatMessage[];
  } catch {
    // Ignore corrupt local history and let Project History hydrate what it can.
    return [];
  }
}

export function saveStoredConversationMessages(conversationID: string, scope: string, messages: ChatMessage[]): void {
  if (typeof window === "undefined" || !conversationID || !scope) {
    return;
  }
  const rows = durableMessagesForStorage(messages)
    .filter((message) => message.id !== "intro")
    .slice(-120)
    .map(sanitizeChatMessageForStorage);
  if (rows.length === 0) {
    return;
  }
  const payload = JSON.stringify({
    schema_version: "ask_vit_conversation_messages.v2",
    conversation_id: conversationID,
    scope,
    saved_at: new Date().toISOString(),
    messages: rows
  });
  try {
    window.localStorage.setItem(conversationMessagesStorageKey(conversationID, scope), payload);
  } catch {
    // Local history is best-effort; Project History remains the durable backend record.
  }
}

function sanitizeChatMessageForStorage(message: ChatMessage): ChatMessage {
  return durableMessage({
    id: textValue(message.id, uniqueID("stored_msg")),
    source_id: textValue(message.source_id, ""),
    role: message.role,
    content: textValue(message.content, ""),
    mode: message.mode,
    artifacts: message.artifacts ?? [],
    actions: (message.actions ?? []).map(asRecord).filter((action) => Object.keys(action).length > 0),
    createdAt: Number.isFinite(message.createdAt) ? message.createdAt : Date.now(),
    status: message.status,
    lifecycle: message.lifecycle,
    persistence: message.persistence,
    message_kind: message.message_kind,
    turn_id: message.turn_id,
    logical_message_id: message.logical_message_id,
    supersedes: message.supersedes
  });
}

function sanitizeStoredChatMessage(value: unknown): ChatMessage | null {
  const row = asRecord(value);
  const roleText = textValue(row.role, "assistant");
  const role: ChatMessage["role"] = roleText === "user" || roleText === "system" ? roleText : "assistant";
  const content = textValue(row.content, "");
  const actions = firstArray(row.actions).map(asRecord).filter((action) => Object.keys(action).length > 0);
  const artifacts = artifactSummariesFrom(row.artifacts);
  if (!content && actions.length === 0 && artifacts.length === 0) {
    return null;
  }
  const createdAt = Number(row.createdAt ?? row.created_at);
  const candidate: ChatMessage = {
    id: textValue(row.id, uniqueID("stored_msg")),
    source_id: textValue(row.source_id, ""),
    role,
    content,
    mode: textValue(row.mode, "") as AgentMode,
    artifacts,
    actions,
    createdAt: Number.isFinite(createdAt) ? createdAt : Date.now(),
    status: chatMessageStatusFromStored(row.status),
    lifecycle: textValue(row.lifecycle, "") as ChatMessage["lifecycle"],
    persistence: textValue(row.persistence, "") as ChatMessage["persistence"],
    message_kind: textValue(row.message_kind, "") as ChatMessage["message_kind"],
    turn_id: textValue(row.turn_id, ""),
    logical_message_id: textValue(row.logical_message_id, ""),
    supersedes: firstArray(row.supersedes).map((item) => textValue(item, "")).filter(Boolean)
  };
  if (isLegacyTransientMessage(candidate) || !isDurableMessage(candidate)) {
    return null;
  }
  return durableMessage(candidate);
}

function chatMessageStatusFromStored(value: unknown): ChatMessage["status"] {
  const status = textValue(value, "sent").toLowerCase();
  if (status === "pending" || status === "error") {
    return status;
  }
  return "sent";
}

export {
  initialMessage,
  mergeStoredScopeMessages,
  messagesOrIntro,
  createConversationID,
  conversationScopedIDStorageKey,
  scopedConversationRuntimeKey,
  normalizeServerContinuationPath,
  conversationMessagesStorageKey,
  sanitizeChatMessageForStorage,
  sanitizeStoredChatMessage,
  chatMessageStatusFromStored
};
