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

import { apiPath, fetchUIState, sendChat, startTutorSim, stopTurn } from "../lib/api";
import { cardsToEntries, type LauncherDirective } from "../api/launcher";
import type { ChatResponse } from "../types";
import type {
  ProjectChatOutcome,
  ProjectContextView,
  MacroApplyOutcome,
  ProjectService,
  TutorMaterialLoadOutcome,
  TutorStartOutcome
} from "./projectData";
import { tutorBriefFromWorkflowData, tutorMaterialsFromWorkflowData } from "./projectData";

/**
 * SHELL-6：工程面消费的 chat 回执子集——types.ts ChatResponse 尚未声明
 * launcher_directive（345 域字段），此处局部扩展（域外零改动）。
 */
type ProjectChatResponse = ChatResponse & { launcher_directive?: LauncherDirective | null };

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
    detail: trackCount > 0 ? `${trackCount} 条轨道` : undefined,
    path
  };
}

/** TUTOR-SIM-2（436）：导师开场回执 → 接缝结果；不可用/缺字段一律 unavailable。
 * TUTOR-SIM-4（438）：v1 回执带 workflow_data（brief/materials）；v0 回执没有
 * workflow_data 时输出对象**不含**这两个键（436 回执映射的严格相等面不变）。 */
export function tutorStartOutcomeFromResponse(response: Awaited<ReturnType<typeof startTutorSim>>): TutorStartOutcome {
  if (response.status !== "ok") {
    return { status: "unavailable", reason: response.reason || response.error || response.status };
  }
  const conversationID = (response.conversation_id ?? "").trim();
  if (!conversationID) return { status: "unavailable", reason: "missing_conversation_id" };
  const workflowData = (response as Awaited<ReturnType<typeof startTutorSim>> & {
    workflow_data?: Record<string, unknown>;
  }).workflow_data;
  const outcome: TutorStartOutcome = {
    status: "ok",
    conversationID,
    reply: (response.reply ?? "").trim(),
    stepIndex: response.step_index ?? 0,
    stepTotal: response.step_total ?? 0,
    stepID: response.step_id ?? ""
  };
  if (workflowData !== undefined && workflowData !== null) {
    const materials = tutorMaterialsFromWorkflowData(workflowData);
    outcome.brief = tutorBriefFromWorkflowData(workflowData);
    if (materials !== null) outcome.materials = materials;
  }
  return outcome;
}

/* TUTOR-SIM-4（438）：材料装载回执面（与 lib/api requestJSON 同型；lib/api
 * 的 requestJSON 未导出，此处用同语义本地 helper，域名外零改动）。 */
interface TutorLoadMaterialResponse {
  status?: string;
  reason?: string;
  error?: string;
  failure_class?: string;
  already_loaded?: boolean;
  load?: {
    loaded?: boolean;
    track_id?: string;
    clip_id?: string;
    plugin_id?: string;
    instrument_attached?: boolean;
    instrument_error?: string;
  };
}

async function postTutorLoadMaterial(payload: {
  conversation_id: string;
  material_title: string;
}): Promise<TutorLoadMaterialResponse> {
  const response = await fetch(apiPath("/agent/tutor/load_material"), {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload)
  });
  const text = await response.text();
  let data: TutorLoadMaterialResponse = {};
  if (text) {
    try {
      data = JSON.parse(text) as TutorLoadMaterialResponse;
    } catch {
      throw new Error(text.trim().slice(0, 120) || response.statusText || "Agent 返回了非 JSON 响应");
    }
  }
  if (!response.ok) {
    const message = data && typeof data === "object" && data.error ? String(data.error) : response.statusText;
    throw new Error(message);
  }
  return data;
}

/** 装载回执 → 接缝结果：服务端 ok 即 track/clip 就位（音源 best-effort 如实带回）。 */
export function tutorMaterialLoadOutcomeFromResponse(response: TutorLoadMaterialResponse): TutorMaterialLoadOutcome {
  if (response.status !== "ok") {
    return { ok: false, reason: response.reason || response.error || response.failure_class || "load_failed" };
  }
  const load = response.load ?? {};
  return {
    ok: true,
    alreadyLoaded: response.already_loaded === true,
    trackID: load.track_id ?? "",
    clipID: load.clip_id ?? "",
    pluginID: load.plugin_id ?? "",
    instrumentAttached: load.instrument_attached === true,
    instrumentError: load.instrument_error
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
    async sendMessage(text: string, tutorConversationID?: string): Promise<ProjectChatOutcome> {
      // TUTOR-SIM-2：导师会话命中时用服务端签发的 conversation_id 走 /agent/chat
      // tutor 分支；否则保持本 surface 的普通会话（既有行为不变）。
      const targetConversationID = tutorConversationID?.trim() || conversationID;
      const response = (await sendChat({ conversation_id: targetConversationID, message: text })) as ProjectChatResponse;
      if (response.error) throw new Error(response.error);
      // SHELL-6：完整回执载荷——directive/cards/确认协议位原样透传（呈现与
      // 执行语义归 projectThreadItemsFromOutcome 单点映射）。
      return {
        text: (response.reply ?? "").trim() || "Vit 没有返回内容——换个说法再试一次。",
        directive: response.launcher_directive ?? null,
        cards: cardsToEntries(response.project_result_cards),
        needsConfirmation: Boolean(response.needs_confirmation),
        // TUTOR-SIM-3：workflow 标记 + workflow_data 原样上抛；进度卡只消费结构化字段。
        workflow: response.workflow ?? null,
        workflowData: response.workflow_data ?? null
      };
    },
    async startTutorSim(projectPath: string): Promise<TutorStartOutcome> {
      // 不可用（无 sidecar / LLM 配置缺失 / HTTP 或网络失败）只返回 unavailable：
      // 自动开场是增强行为，任何失败都不得打断普通工程面。
      try {
        const response = await startTutorSim({ project_path: projectPath });
        return tutorStartOutcomeFromResponse(response);
      } catch (error) {
        return {
          status: "unavailable",
          reason: error instanceof Error ? error.message : "tutor_start_failed"
        };
      }
    },
    async loadTutorMaterial(conversationID: string, title: string): Promise<TutorMaterialLoadOutcome> {
      // 失败只返回 ok:false + 原因：装载失败不打断导师循环，也不伪造已装载。
      try {
        const response = await postTutorLoadMaterial({ conversation_id: conversationID, material_title: title });
        return tutorMaterialLoadOutcomeFromResponse(response);
      } catch (error) {
        return { ok: false, reason: error instanceof Error ? error.message : "tutor_load_material_failed" };
      }
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
