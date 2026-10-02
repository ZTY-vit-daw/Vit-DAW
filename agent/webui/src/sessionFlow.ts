// WEBUI-IA-REDESIGN-1（2026-10-02）：会话流侧边栏的会话注册表。
//
// 用户裁定（decisions/2026-10-02-webui-ia-vision.md）：会话流与工程分支/工作树
// 解耦——侧边栏只按 conversation id 组织轻量会话（新建/切换/重命名/归档），
// 不建工作树、不建分支；工程级深操作保留在历史界面。本模块只做三件事：
//   1. 本地注册表（localStorage，按 history scope 分桶）的 fail-open 读写；
//   2. 服务端 continuations 投影行 → 会话提示（跨浏览器可见的服务端会话）；
//   3. 注册表+服务端提示的合并投影（展示行 + 纯线性编号 1/2/3…，无前缀无哈希）。
//
// 持久化兼容（AGENTS §11）：注册表是新增存储，形态 fail-open——损坏 JSON、
// 字段缺型、未知字段一律按行丢弃/忽略，读取失败返回空表，绝不让旧数据或脏
// 数据破坏加载；不改动任何既有存储键的语义。

export interface SessionFlowEntry {
  conversationID: string;
  /** 显式重命名后的标题；""=未命名（由调用侧从消息缓存推导展示名） */
  title: string;
  createdAt: number;
  updatedAt: number;
  archived: boolean;
}

export interface SessionFlowRow extends SessionFlowEntry {
  /** 展示用纯线性编号（1、2、3…）：按列表位次派生，不持久化、无前缀无哈希 */
  displayNumber: number;
  /** 来自服务端 continuations 投影（本浏览器从未打开过的会话也会出现） */
  serverKnown: boolean;
}

export interface SessionFlowScopeParts {
  projectPath: string;
  rootProjectPath: string;
  projectUUID: string;
}

const REGISTRY_PREFIX = "ask_vit_session_flow.v1";
const COLLAPSED_KEY = "ask_vit_session_flow_collapsed.v1";
const REGISTRY_LIMIT = 200;

function storageScopeKey(scope: string): string {
  // 独立命名空间：键格式自含（trim+压缩分隔符），不依赖 App 内部键编码。
  const slug = scope.trim().replace(/[<>:"/\\|?*\u0000-\u001f]/g, "-").slice(0, 96);
  return `${REGISTRY_PREFIX}:${slug || "unsaved"}`;
}

function validEntry(value: unknown): SessionFlowEntry | null {
  if (!value || typeof value !== "object") {
    return null;
  }
  const row = value as Record<string, unknown>;
  const conversationID = typeof row.conversationID === "string" ? row.conversationID.trim() : "";
  if (!conversationID) {
    return null;
  }
  const createdAt = Number(row.createdAt);
  const updatedAt = Number(row.updatedAt);
  return {
    conversationID,
    title: typeof row.title === "string" ? row.title.trim().slice(0, 120) : "",
    createdAt: Number.isFinite(createdAt) && createdAt > 0 ? createdAt : Date.now(),
    updatedAt: Number.isFinite(updatedAt) && updatedAt > 0 ? updatedAt : Date.now(),
    archived: row.archived === true
  };
}

export function loadSessionFlowRegistry(scope: string): SessionFlowEntry[] {
  if (typeof window === "undefined" || !scope) {
    return [];
  }
  try {
    const raw = window.localStorage.getItem(storageScopeKey(scope));
    if (!raw) {
      return [];
    }
    const parsed = JSON.parse(raw) as unknown;
    const rows = Array.isArray((parsed as { sessions?: unknown[] })?.sessions)
      ? (parsed as { sessions: unknown[] }).sessions
      : Array.isArray(parsed)
        ? parsed
        : [];
    const seen = new Set<string>();
    const entries: SessionFlowEntry[] = [];
    for (const row of rows) {
      const entry = validEntry(row);
      if (!entry || seen.has(entry.conversationID)) {
        continue;
      }
      seen.add(entry.conversationID);
      entries.push(entry);
    }
    return entries;
  } catch {
    // fail-open：损坏/异形数据按空表处理，不阻塞加载（AGENTS §11）
    return [];
  }
}

export function saveSessionFlowRegistry(scope: string, entries: SessionFlowEntry[]): void {
  if (typeof window === "undefined" || !scope) {
    return;
  }
  try {
    const rows = entries.slice(0, REGISTRY_LIMIT);
    window.localStorage.setItem(
      storageScopeKey(scope),
      JSON.stringify({ schema_version: "ask_vit_session_flow.v1", sessions: rows })
    );
  } catch {
    // best-effort：注册表只是展示索引，写失败不阻塞会话功能
  }
}

/** 幂等登记：会话不存在则建行，存在则只刷 updatedAt（保留重命名与归档态） */
export function upsertSessionFlowEntry(
  entries: SessionFlowEntry[],
  input: { conversationID: string; updatedAt?: number; title?: string }
): SessionFlowEntry[] {
  const conversationID = input.conversationID.trim();
  if (!conversationID) {
    return entries;
  }
  const updatedAt = input.updatedAt ?? Date.now();
  let found = false;
  const next = entries.map((entry) => {
    if (entry.conversationID !== conversationID) {
      return entry;
    }
    found = true;
    const title = input.title !== undefined && entry.title === "" ? input.title.trim().slice(0, 120) : entry.title;
    return { ...entry, updatedAt: Math.max(entry.updatedAt, updatedAt), title };
  });
  if (!found) {
    next.push({
      conversationID,
      title: (input.title ?? "").trim().slice(0, 120),
      createdAt: updatedAt,
      updatedAt,
      archived: false
    });
  }
  return next;
}

export function renameSessionFlowEntry(entries: SessionFlowEntry[], conversationID: string, title: string): SessionFlowEntry[] {
  const cleanTitle = title.trim().slice(0, 120);
  return entries.map((entry) => (entry.conversationID === conversationID ? { ...entry, title: cleanTitle } : entry));
}

export function setSessionFlowArchived(entries: SessionFlowEntry[], conversationID: string, archived: boolean): SessionFlowEntry[] {
  return entries.map((entry) => (entry.conversationID === conversationID ? { ...entry, archived } : entry));
}

export interface SessionFlowServerHint {
  conversationID: string;
  updatedAt: number;
  intent: string;
}

function normalizePath(value: unknown): string {
  return typeof value === "string" ? value.trim().replace(/\\/g, "/").replace(/\/+$/, "").toLowerCase() : "";
}

function textOf(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

/**
 * 服务端 continuations 投影行 → 本 scope 的会话提示。匹配口径与
 * serverConversationIDFromContinuations（REFRESH-VANISH-2）一致：project_path
 * 或 project_uuid 命中即算本工程；同一会话多任务切片按最新 updated_at 去重。
 */
export function sessionFlowServerHints(continuations: unknown, parts: SessionFlowScopeParts): Map<string, SessionFlowServerHint> {
  const hints = new Map<string, SessionFlowServerHint>();
  if (!Array.isArray(continuations)) {
    return hints;
  }
  const scopePath = normalizePath(parts.projectPath || parts.rootProjectPath);
  const scopeUUID = parts.projectUUID.trim().toLowerCase();
  if (!scopePath && !scopeUUID) {
    return hints;
  }
  for (const raw of continuations) {
    if (!raw || typeof raw !== "object") {
      continue;
    }
    const row = raw as Record<string, unknown>;
    const conversationID = textOf(row.conversation_id);
    if (!conversationID) {
      continue;
    }
    const rowPath = normalizePath(row.project_path);
    const rowUUID = textOf(row.project_uuid).toLowerCase();
    const matched = (scopePath !== "" && rowPath === scopePath) || (scopeUUID !== "" && rowUUID === scopeUUID);
    if (!matched) {
      continue;
    }
    const parsed = Date.parse(textOf(row.updated_at));
    const updatedAt = Number.isFinite(parsed) ? parsed : 0;
    const intent = textOf(row.original_intent).slice(0, 120);
    const existing = hints.get(conversationID);
    if (!existing || updatedAt >= existing.updatedAt) {
      hints.set(conversationID, { conversationID, updatedAt: Math.max(updatedAt, existing?.updatedAt ?? 0), intent });
    }
  }
  return hints;
}

/**
 * 合并投影：注册表为展示权威（标题/归档/创建时间），服务端提示补充本浏览器
 * 未见过的会话并刷新 updatedAt（取两侧较大值）。输出按 updatedAt 降序（同刻
 * 按 conversationID 决胜，选择只由数据决定），可见行赋纯线性编号 1/2/3…。
 */
export function sessionFlowRows(
  entries: SessionFlowEntry[],
  serverHints: Map<string, SessionFlowServerHint>
): { visible: SessionFlowRow[]; archived: SessionFlowRow[] } {
  const byID = new Map<string, SessionFlowEntry>();
  for (const entry of entries) {
    byID.set(entry.conversationID, { ...entry });
  }
  for (const [conversationID, hint] of serverHints) {
    const existing = byID.get(conversationID);
    if (!existing) {
      byID.set(conversationID, {
        conversationID,
        title: "",
        createdAt: hint.updatedAt,
        updatedAt: hint.updatedAt,
        archived: false
      });
    } else if (hint.updatedAt > existing.updatedAt) {
      byID.set(conversationID, { ...existing, updatedAt: hint.updatedAt });
    }
  }
  const merged = Array.from(byID.values()).sort((left, right) => {
    if (left.updatedAt !== right.updatedAt) {
      return right.updatedAt - left.updatedAt;
    }
    return left.conversationID < right.conversationID ? -1 : 1;
  });
  let numberSeed = 0;
  const rows = merged.map((entry) => ({
    ...entry,
    displayNumber: ++numberSeed,
    serverKnown: serverHints.has(entry.conversationID)
  }));
  return {
    visible: rows.filter((row) => !row.archived),
    archived: rows.filter((row) => row.archived)
  };
}

/** 侧边栏折叠态（全局，不分 scope） */
export function loadSessionFlowCollapsed(): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  try {
    return window.localStorage.getItem(COLLAPSED_KEY) === "1";
  } catch {
    return false;
  }
}

export function saveSessionFlowCollapsed(collapsed: boolean): void {
  if (typeof window === "undefined") {
    return;
  }
  try {
    window.localStorage.setItem(COLLAPSED_KEY, collapsed ? "1" : "0");
  } catch {
    // best-effort
  }
}
