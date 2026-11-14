/*
 * UI-PROJECT-1（346）工程内 surface · live 服务实现（真实 agent 端点）。
 *
 * 消费既有端点（零 agent 改动；import-only 复用 lib/api.ts）：
 * - fetchUIState() → GET /agent/ui_state：project.project_path → 工程名；
 *   tracks 长度 → 上下文行（一行元信息最多两项）。
 * - sendChat() → POST /agent/chat：本 surface 独立会话（webui_project_* 前缀，
 *   同 345 launcher 模式，不触碰主对话/历史状态）。
 * - stopTurn() → POST /agent/stop_turn：回复中停止。
 *
 * 边界（卡面 fixture 例外 + 停止条件）：宏快照应用/撤销、A/B 渲染在 agent 侧
 * 无现成协议——live 模式不提供这三项（调用即抛错，由 surface 拦截不入口），
 * 不得伪造「已应用」；如需协议走停止条件交主管另开卡。
 */

import { fetchUIState, sendChat, stopTurn } from "../lib/api";
import type { ProjectContextView, MacroApplyOutcome, ProjectService } from "./projectData";

export function createProjectConversationID(now: number = Date.now()): string {
  return `webui_project_${now.toString(36)}`;
}

function lastPathPart(path: string): string {
  const normalized = path.replace(/\\/g, "/");
  const parts = normalized.split("/").filter(Boolean);
  return parts.length > 0 ? parts[parts.length - 1] : path;
}

function countTracks(uiState: { tracks?: unknown[] } | null | undefined): number {
  return Array.isArray(uiState?.tracks) ? uiState.tracks.length : 0;
}

export function projectContextFromUIState(uiState: Awaited<ReturnType<typeof fetchUIState>>): ProjectContextView | null {
  const project = uiState?.project;
  if (!project || typeof project !== "object") return null;
  const record = project as Record<string, unknown>;
  const path = typeof record.project_path === "string" ? record.project_path.trim() : "";
  if (!path) return null;
  const trackCount = countTracks(uiState);
  return {
    name: lastPathPart(path),
    source: "agent",
    detail: trackCount > 0 ? `${trackCount} 条轨道` : undefined
  };
}

export function createProjectLiveService(): ProjectService {
  const conversationID = createProjectConversationID();
  return {
    async fetchContext(): Promise<ProjectContextView> {
      const uiState = await fetchUIState();
      const view = projectContextFromUIState(uiState);
      if (!view) throw new Error("agent 尚未装载工程上下文");
      return view;
    },
    async sendMessage(text: string): Promise<string> {
      const response = await sendChat({ conversation_id: conversationID, message: text });
      if (response.error) throw new Error(response.error);
      const reply = (response.reply ?? "").trim();
      return reply || "Vit 没有返回内容——换个说法再试一次。";
    },
    async stopReply(): Promise<void> {
      await stopTurn({ conversation_id: conversationID, reason: "webui_project_surface_stop" });
    },
    async applyMacro(): Promise<MacroApplyOutcome> {
      throw new Error("宏快照应用暂无 agent 协议（未接入，不伪造已应用）");
    },
    async undoSnapshot(): Promise<void> {
      throw new Error("快照撤销暂无 agent 协议（未接入，不伪造撤销）");
    },
    async prepareAB(): Promise<{ bypassRef: string; processedRef: string }> {
      throw new Error("A/B 渲染暂无 agent 协议（未接入，不伪造就绪）");
    }
  };
}
