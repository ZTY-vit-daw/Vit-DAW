// T9-PRE-1A-1（施工图 UI-CONVERGE-RECON-1 §3.1 批 1 / 步 1f-A）：App.tsx 通用工具域出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
// numericValue/finiteNumber 为跨域通用数值助手（daw/macro/browser 桥共用），按 §U
// 「通用工具——全域被依赖」语义归位本模块（物理原址分别在 App 的 browser 桥区与
// history 类型区，见卡面回执偏差记录）。
import type { AgentUIState, ChatMessage, JsonRecord } from "../types";

const panelWidthStoragePrefix = "ask_vit_project_workbench_width";

function numericValue(value: unknown): number {
  const n = Number(value);
  return Number.isFinite(n) ? n : 0;
}

function finiteNumber(value: unknown, fallback: number): number {
  const next = Number(value);
  return Number.isFinite(next) ? next : fallback;
}

function asRecord(value: unknown): JsonRecord {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as JsonRecord) : {};
}

function firstArray(...values: unknown[]): unknown[] {
  for (const value of values) {
    if (Array.isArray(value)) {
      return value;
    }
  }
  return [];
}

function textValue(value: unknown, fallback: string): string {
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (trimmed === "<nil>" || trimmed === "null" || trimmed === "undefined") {
      return fallback;
    }
    return trimmed || fallback;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return fallback;
}

function compactJSON(value: unknown): string {
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function parseJSONRecord(value: unknown): JsonRecord {
  const text = textValue(value, "");
  if (!text) {
    return {};
  }
  try {
    return asRecord(JSON.parse(text));
  } catch {
    return {};
  }
}

function stableIDPart(value: string): string {
  const clean = value.trim().replace(/[^a-zA-Z0-9_-]+/g, "_").replace(/^_+|_+$/g, "");
  return clean.slice(0, 96) || stableTextHash(value);
}

function stableTextHash(value: string): string {
  let hash = 2166136261;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return (hash >>> 0).toString(36);
}

function clampNumber(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function defaultComposerInteractionHeight(): number {
  if (typeof window === "undefined") {
    return 560;
  }
  const saved = Number(window.localStorage?.getItem("ask_vit_composer_interaction_height"));
  if (Number.isFinite(saved) && saved > 0) {
    return clampComposerInteractionHeight(saved);
  }
  return clampComposerInteractionHeight(window.innerHeight * 0.74);
}

function clampComposerInteractionHeight(value: number): number {
  if (typeof window === "undefined") {
    return clampNumber(value, 340, 880);
  }
  return clampNumber(value, 320, Math.max(420, window.innerHeight - 124));
}

function truthy(value: unknown): boolean {
  return value === true || value === "true" || value === 1 || value === "1";
}

function formatBytes(value: unknown): string {
  const bytes = typeof value === "number" ? value : 0;
  if (bytes <= 0) {
    return "-";
  }
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  if (bytes < 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

function lastPathPart(path: string): string {
  const normalized = path.replace(/\\/g, "/");
  const parts = normalized.split("/").filter(Boolean);
  return parts[parts.length - 1] || path || "Vit Project";
}

function uniqueID(prefix: string): string {
  return `${prefix}_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
}

function currentConversationTitle(uiState: AgentUIState | null, messages: ChatMessage[]): string {
  const history = asRecord(uiState?.project_history);
  const graph = asRecord(history.conversation_graph);
  const nodes = firstArray(graph.nodes).map(asRecord);
  const activeNodeID = textValue(history.active_node_id ?? graph.active_node_id, "");
  const byID = new Map(nodes.map((node) => [textValue(node.id, ""), node]));
  const activePath: JsonRecord[] = [];
  const visited = new Set<string>();
  let cursor = activeNodeID ? byID.get(activeNodeID) : undefined;
  while (cursor) {
    const id = textValue(cursor.id, "");
    if (!id || visited.has(id)) {
      break;
    }
    visited.add(id);
    activePath.unshift(cursor);
    cursor = byID.get(textValue(cursor.parent_node_id, ""));
  }
  const latestTaskAsk = [...activePath].reverse().find((node) => {
    if (textValue(node.kind, "").toLowerCase() !== "ask") {
      return false;
    }
    return conversationTitleCandidate(textValue(node.text_preview ?? node.text ?? node.message, ""));
  });
  const latestTaskText = latestTaskAsk ? textValue(latestTaskAsk.text_preview ?? latestTaskAsk.text ?? latestTaskAsk.message, "") : "";
  if (latestTaskText) {
    return compactConversationTitle(latestTaskText);
  }
  const localTaskMessage = [...messages].reverse().find((message) => message.role === "user" && conversationTitleCandidate(textValue(message.content, "")));
  return localTaskMessage ? compactConversationTitle(localTaskMessage.content) : "新对话";
}

function conversationTitleCandidate(value: string): boolean {
  const clean = value.replace(/\s+/g, " ").trim().toLowerCase();
  if (clean.length < 3) {
    return false;
  }
  return !/^(可以|可以执行|执行|确认|确定|同意|继续|好的|好|ok|okay|yes|取消|不用了)[。.!！\s]*$/i.test(clean);
}

function compactConversationTitle(value: string): string {
  const clean = value.replace(/\s+/g, " ").replace(/^[#>*`\-\s]+/, "").trim();
  if (!clean) {
    return "新对话";
  }
  return clean.length > 28 ? `${clean.slice(0, 27)}…` : clean;
}

function panelWidthStorageKey(side: "left" | "right", panel: string): string {
  return `${panelWidthStoragePrefix}_${side}_${panel}`;
}

function loadPanelWidth(side: "left" | "right", panel: string, fallback: number, minimum: number): number {
  if (typeof window === "undefined") {
    return fallback;
  }
  try {
    const stored = Number(window.localStorage.getItem(panelWidthStorageKey(side, panel)));
    return Number.isFinite(stored) && stored >= minimum ? stored : fallback;
  } catch {
    return fallback;
  }
}

function savePanelWidth(side: "left" | "right", panel: string, width: number): void {
  try {
    window.localStorage.setItem(panelWidthStorageKey(side, panel), String(Math.round(width)));
  } catch {
    // Panel sizing is best-effort and must never block the workbench.
  }
}

export {
  numericValue,
  finiteNumber,
  asRecord,
  firstArray,
  textValue,
  compactJSON,
  parseJSONRecord,
  stableIDPart,
  stableTextHash,
  clampNumber,
  defaultComposerInteractionHeight,
  clampComposerInteractionHeight,
  truthy,
  formatBytes,
  lastPathPart,
  uniqueID,
  currentConversationTitle,
  conversationTitleCandidate,
  compactConversationTitle,
  panelWidthStorageKey,
  loadPanelWidth,
  savePanelWidth
};
