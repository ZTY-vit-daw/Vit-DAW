import type { AgentEvent, ChatMessage, JsonRecord, RuntimeContinuation } from "./types";

// F3（2026-09-10）：webui 呈现面三症的守卫层。
//
// 面①（死卡守卫）：Project History 的 conversation_messages 是发送时刻的快照，
// 已应答的 interaction 请求仍带 waiting_for_user；server 防线
//（chat/server.go:4022 "这个交互已处理或已过期"）只在重复提交时兜底，呈现面
// 会把水合快照重新渲染为可交互卡（B1 18:59 二次 approve 形态、E2E R5 死卡半段）。
// 本模块维护一个持久的"已消费交互台账"（localStorage，ID 全局唯一故不分会话），
// 并提供 stampConsumedInteractionActions 在水合/恢复/合并边界把已消费动作标记
// resolved + 摘除子按钮——渲染层（StandardActionCard.isInteractive = id 非空 ∧
// 子按钮非空）与 composer 拾取（isComposerInteraction 状态排除集）因此都拿不到
// 可交互形态。台账在点击应答时乐观写入（应答失败回滚），并由 AGENT-F5 的
// interaction.resolved 事件（server 权威撤卡信号）补充。
//
// 面③（水合隔离）：runtime status 的 continuation pending_interaction 投影是
// 全局的，新浏览器会话会把他会话的 waiting_interaction 卡投影进自己的 composer
// 并挤占之（E2E run 213930 R2 阻断成因）。pendingInteractionFromContinuations
// 按 conversation_id 归属过滤；缺失归属的行按隔离目标 fail-closed 不投影。
//
// 兼容口径：台账损坏/旧形状按空台账处理（fail-open 只影响守卫强度，不影响
// 功能）；盖章是幂等的纯函数变换；capability proposal 卡有独立生命周期
// （proposal_presentation 驱动渲染），不在本守卫盖章范围内。

const consumedInteractionStorageKey = "ask_vit_consumed_interactions";
const consumedInteractionLedgerLimit = 200;

// server 4022 口径的已消费/失效状态词根。与 isComposerInteraction 的
// complete/done/cancel/fail/error 排除集互补（"resolved" 不含那些子串）。
const consumedStatusTokens = ["resolv", "respond", "expir", "consum", "answer"];

export const CONSUMED_INTERACTION_RESOLVED_STATUS = "resolved";
export const CONSUMED_INTERACTION_GUARD_ID = "consumed_interaction_guard";

export function isConsumedInteractionStatus(status: unknown): boolean {
  const normalized = String(status ?? "").trim().toLowerCase();
  if (!normalized) {
    return false;
  }
  return consumedStatusTokens.some((token) => normalized.includes(token));
}

function recordText(value: unknown): string {
  return typeof value === "string" ? value.trim() : value == null ? "" : String(value).trim();
}

function isRecord(value: unknown): value is JsonRecord {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

// 盖章范围：composer/流内的 interaction 卡（_ui_source=interaction），排除
// capability proposal（独立渲染与生命周期）。与 App 的 isComposerInteraction
// 输入域对齐但不复制其状态逻辑——状态排除由消费方谓词叠加本模块的状态集。
function isGuardStampableInteractionAction(action: JsonRecord): boolean {
  if (recordText(action._ui_source) !== "interaction") {
    return false;
  }
  if (isCapabilityProposalLike(action)) {
    return false;
  }
  return Boolean(recordText(action.id ?? action.interaction_id));
}

function isCapabilityProposalLike(action: JsonRecord): boolean {
  const payload = isRecord(action.payload ?? action.data) ? (action.payload ?? action.data) as JsonRecord : {};
  const presentation = isRecord(payload.proposal_presentation ?? action.proposal_presentation)
    ? (payload.proposal_presentation ?? action.proposal_presentation) as JsonRecord
    : {};
  const kind = recordText(action.kind).toLowerCase();
  const type = recordText(action.type).toLowerCase();
  return kind === "proposal_approval" || type === "proposal_approval" || recordText(presentation.schema_version) === "vit.proposal_presentation.v1";
}

// 把已消费台账命中的 interaction 动作标记为 resolved 只读形态：
// status/stage → resolved、resolved_action_id → guard 标识、子按钮摘除。
// 幂等；未命中与范围外动作原样返回（保持引用相等以便上层零拷贝短路）。
export function stampConsumedInteractionActions(messages: ChatMessage[], consumedIDs: Iterable<string>): ChatMessage[] {
  const consumed = consumedIDs instanceof Set ? consumedIDs : new Set(consumedIDs);
  if (consumed.size === 0 || messages.length === 0) {
    return messages;
  }
  let changed = false;
  const next = messages.map((message) => {
    if (!message.actions?.length) {
      return message;
    }
    let messageChanged = false;
    const actions = message.actions.map((actionValue) => {
      if (!isRecord(actionValue) || !isGuardStampableInteractionAction(actionValue)) {
        return actionValue;
      }
      const id = recordText(actionValue.id ?? actionValue.interaction_id);
      if (!consumed.has(id)) {
        return actionValue;
      }
      if (isConsumedInteractionStatus(actionValue.status ?? actionValue.stage)) {
        return actionValue;
      }
      messageChanged = true;
      changed = true;
      return {
        ...actionValue,
        status: CONSUMED_INTERACTION_RESOLVED_STATUS,
        stage: CONSUMED_INTERACTION_RESOLVED_STATUS,
        resolved_action_id: CONSUMED_INTERACTION_GUARD_ID,
        actions: []
      };
    });
    return messageChanged ? { ...message, actions } : message;
  });
  return changed ? next : messages;
}

// AGENT-F5：server completePendingInteractionContinuation 发出的权威撤卡事件。
export function resolvedInteractionIdsFromEvents(events: AgentEvent[]): string[] {
  const out: string[] = [];
  const seen = new Set<string>();
  for (const event of events ?? []) {
    if (recordText(event?.type) !== "interaction.resolved") {
      continue;
    }
    const payload = isRecord(event.payload) ? event.payload : {};
    const id = recordText(payload.interaction_id);
    if (!id || seen.has(id)) {
      continue;
    }
    seen.add(id);
    out.push(id);
  }
  return out;
}

// F3 面③：conversation 归属过滤的 pending 交互投影（App 的
// backgroundPendingInteractionAction 收窄为薄包装）。语义与原实现一致：
// 取最新的 waiting_interaction 行；新增会话归属约束——
// conversation_id 缺省视为他行（不可归属），不投影。
export function pendingInteractionFromContinuations(
  continuations: RuntimeContinuation[],
  conversationID: string
): JsonRecord | null {
  const scope = recordText(conversationID);
  if (!scope) {
    return null;
  }
  for (let index = (continuations ?? []).length - 1; index >= 0; index -= 1) {
    const row = continuations[index];
    if (recordText(row?.status).toLowerCase() !== "waiting_interaction") {
      continue;
    }
    if (recordText(row?.conversation_id) !== scope) {
      continue;
    }
    const pending = isRecord(row.pending_interaction) ? row.pending_interaction as JsonRecord : {};
    const interactionID = recordText(pending.interaction_id);
    if (!interactionID) {
      continue;
    }
    const requests = Array.isArray(pending.requests) ? pending.requests.filter(isRecord) as JsonRecord[] : [];
    const request = requests.length > 0 ? requests[0] : {};
    return {
      ...request,
      id: interactionID,
      interaction_id: interactionID,
      kind: recordText(pending.kind) || recordText(request.kind) || "confirmation",
      type: recordText(request.type) || "approval.requested",
      status: "waiting_for_user",
      _ui_source: "interaction"
    };
  }
  return null;
}

// —— 已消费台账（localStorage 持久层；node/测试环境无 window 时安全退化为
//    不持久化的空操作，核心逻辑经 parseConsumedInteractionLedger 纯函数测试）——

export function parseConsumedInteractionLedger(raw: unknown): string[] {
  if (!Array.isArray(raw)) {
    return [];
  }
  const out: string[] = [];
  const seen = new Set<string>();
  for (const entry of raw) {
    let id = "";
    if (typeof entry === "string") {
      id = entry.trim();
    } else if (isRecord(entry)) {
      const value = entry.id ?? entry.interaction_id;
      id = typeof value === "string" ? value.trim() : "";
    }
    if (!id || seen.has(id)) {
      continue;
    }
    seen.add(id);
    out.push(id);
  }
  return out;
}

function readConsumedInteractionStorage(): string[] {
  if (typeof window === "undefined") {
    return [];
  }
  try {
    return parseConsumedInteractionLedger(JSON.parse(window.localStorage.getItem(consumedInteractionStorageKey) || "[]"));
  } catch {
    return [];
  }
}

function writeConsumedInteractionStorage(ids: string[]): void {
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(consumedInteractionStorageKey, JSON.stringify(ids.slice(-consumedInteractionLedgerLimit)));
  } catch {
    // Best-effort ledger; the server 4022 line remains the backstop.
  }
}

export function loadConsumedInteractionIDs(): string[] {
  return readConsumedInteractionStorage();
}

export function recordConsumedInteractions(...idGroups: Array<string | (string | undefined | null)[]>): void {
  const additions = new Set<string>();
  for (const group of idGroups) {
    const ids = Array.isArray(group) ? group : [group];
    ids.forEach((id) => {
      const clean = recordText(id);
      if (clean) {
        additions.add(clean);
      }
    });
  }
  if (additions.size === 0) {
    return;
  }
  const merged = new Set(readConsumedInteractionStorage());
  additions.forEach((id) => merged.add(id));
  writeConsumedInteractionStorage(Array.from(merged));
}

export function forgetConsumedInteraction(...ids: Array<string | undefined | null>): void {
  const removals = new Set(ids.map((id) => recordText(id)).filter(Boolean));
  if (removals.size === 0) {
    return;
  }
  const remaining = readConsumedInteractionStorage().filter((id) => !removals.has(id));
  writeConsumedInteractionStorage(remaining);
}
