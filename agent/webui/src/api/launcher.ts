/*
 * UI-LAUNCHER-1 launcher 面 API 客户端。
 *
 * 消费真实 agent 端点（agent/internal/chat/launcher_flow.go + server.go）：
 * - GET  /agent/launcher/greeting  门厅开场白（确定性模板，零 LLM）
 * - POST /agent/chat               launcher 聊天（context.launcher=true 主门自报）
 *
 * 契约依据（server.go ChatRequest/ChatResponse + launcher_flow.go）：
 * - 请求体键 `context`，launcher 键 = context.launcher === true（§3.1 非权限声明）。
 * - 回执 launcher_directive：agent 只决定与呈报，requires_governance 恒 true，
 *   执行权在 UI 侧自查——本卡不执行 open/new，仅数据展示。
 * - 回执 project_result_cards[]：kind=launcher_project / type=project_open_candidate。
 * - context_flow："launcher"（launcher 分支回执）/"project"/空。
 *
 * 本文件不 import src/types.ts（避免改动 345 文件域之外的类型面），
 * launcher 相关类型自持；fetch 复用 src/lib/api.ts 的 apiPath（只 import 不改）。
 */

import { apiPath } from "../lib/api";

/** launcher_directive（设计稿 §4.1 冻结 schema 的前端镜像）。 */
export interface LauncherDirective {
  action: string; // open_project | new_project
  file_path?: string;
  project_name?: string;
  reason?: string;
  requires_governance?: boolean;
}

/** 单个最近工程条目（抽屉行）。 */
export interface LauncherProjectEntry {
  /** 显示名（去扩展名）。 */
  name: string;
  /** 完整路径。 */
  filePath: string;
}

/** launcher 结果卡（project_result_cards 元素的前端镜像）。 */
export interface LauncherProjectCard extends LauncherProjectEntry {
  id: string;
  kind?: string;
  type?: string;
  directive?: LauncherDirective | null;
}

/** GET /agent/launcher/greeting 回执。 */
export interface LauncherGreeting {
  status?: string;
  launcher?: boolean;
  reply?: string;
  recent_project?: { name?: string; file_path?: string } | null;
  recent_count?: number;
}

/** POST /agent/chat（launcher 卷）回执中本卡消费的字段。 */
export interface LauncherChatResult {
  conversation_id?: string;
  reply?: string;
  agent_mode?: string;
  context_flow?: string;
  launcher_directive?: LauncherDirective | null;
  project_result_cards?: Array<Record<string, unknown>>;
  error?: string;
  status?: string;
}

/** 请求体（types.ts ChatRequest 的 launcher 子集，context.launcher=true）。 */
export interface LauncherChatRequestBody {
  conversation_id: string;
  message: string;
  context: { launcher: true };
}

async function readError(response: Response, fallback: string): Promise<string> {
  const text = await response.text().catch(() => "");
  if (text) {
    try {
      const data = JSON.parse(text) as { error?: string };
      if (data && typeof data.error === "string" && data.error) return data.error;
    } catch {
      return text.slice(0, 160);
    }
  }
  return fallback || `${response.status} ${response.statusText}`;
}

/**
 * 门厅开场白。agent 不可用 / 非 2xx → 抛错（调用方给明确文案与重试）。
 * recents 不可达时 agent 以 recent_count=0 + 降级模板回执（HTTP 200），
 * 该降级不在此处视为错误——由调用方按 recent_count 呈现空态。
 */
export async function fetchLauncherGreeting(): Promise<LauncherGreeting> {
  const response = await fetch(apiPath("/agent/launcher/greeting"), { method: "GET" });
  if (!response.ok) {
    throw new Error(await readError(response, "launcher greeting 请求失败"));
  }
  const text = await response.text();
  if (!text) throw new Error("launcher greeting 返回了空响应");
  try {
    return JSON.parse(text) as LauncherGreeting;
  } catch {
    throw new Error("launcher greeting 返回了非 JSON 响应");
  }
}

/**
 * launcher 聊天轮次。主门自报 context.launcher=true；agent 端确定性意图路由
 * （零 LLM）在 launcher 卷生效。网络/HTTP 错误抛出，业务错误字段随回执返回。
 */
export async function sendLauncherChat(
  conversationID: string,
  message: string
): Promise<LauncherChatResult> {
  const body: LauncherChatRequestBody = {
    conversation_id: conversationID,
    message,
    context: { launcher: true }
  };
  const response = await fetch(apiPath("/agent/chat"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body)
  });
  if (!response.ok) {
    throw new Error(await readError(response, "launcher 聊天请求失败"));
  }
  const text = await response.text();
  if (!text) throw new Error("launcher 聊天返回了空响应");
  try {
    return JSON.parse(text) as LauncherChatResult;
  } catch {
    throw new Error("launcher 聊天返回了非 JSON 响应");
  }
}

/** 回执结果卡 → 抽屉/卡面条目（容错：缺 name/file_path 的卡跳过）。 */
export function cardsToEntries(cards: Array<Record<string, unknown>> | undefined): LauncherProjectCard[] {
  const out: LauncherProjectCard[] = [];
  for (const card of cards ?? []) {
    const name = typeof card.name === "string" ? card.name.trim() : "";
    const filePath = typeof card.file_path === "string" ? card.file_path.trim() : "";
    if (!name && !filePath) continue;
    const directive =
      card.launcher_directive && typeof card.launcher_directive === "object"
        ? (card.launcher_directive as LauncherDirective)
        : null;
    out.push({
      id: typeof card.id === "string" && card.id ? card.id : `launcher_card_${out.length + 1}`,
      kind: typeof card.kind === "string" ? card.kind : undefined,
      type: typeof card.type === "string" ? card.type : undefined,
      name,
      filePath,
      directive
    });
  }
  return out;
}
