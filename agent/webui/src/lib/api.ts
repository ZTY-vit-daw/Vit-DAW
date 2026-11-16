import type {
  AgentConfigResponse,
  AgentEventsResponse,
  AgentInvokeRequest,
  AgentInvokeResponse,
  AgentUIState,
  AuthorityMode,
  Artifact,
  ArtifactListResponse,
  ArtifactReadResponse,
  ChatRequest,
  ChatResponse,
  EngineConfig,
  HealthResponse,
  InteractionRespondRequest,
  JsonRecord,
  MacroControl,
  MacroControlSetResponse,
  ResourceIntakeResponse,
  RuntimeStatusResponse
} from "../types";

const configuredBase = import.meta.env.VITE_AGENT_API_BASE?.replace(/\/$/, "") ?? "";

export function apiPath(path: string): string {
  return `${configuredBase}${path}`;
}

async function requestJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(apiPath(path), {
    ...init,
    headers: {
      ...(init?.body instanceof FormData ? {} : { "Content-Type": "application/json" }),
      ...init?.headers
    }
  });
  const text = await response.text();
  let data = {} as T;
  if (text) {
    try {
      data = JSON.parse(text) as T;
    } catch {
      const preview = text.trim().slice(0, 120);
      if (response.status === 404 && (path.startsWith("/agent/resource") || path.startsWith("/agent/downloads"))) {
        throw new Error("Agent 资料摄取接口未就绪，请重启 VitAgent 或从 Start Page 重新运行测试。");
      }
      throw new Error(preview || response.statusText || "Agent 返回了非 JSON 响应");
    }
  }
  if (!response.ok) {
    const error = data && typeof data === "object" && "error" in data ? String(data.error) : response.statusText;
    throw new Error(error);
  }
  return data;
}

export function fetchHealth(): Promise<HealthResponse> {
  return requestJSON<HealthResponse>("/health");
}

export function fetchRuntimeStatus(): Promise<RuntimeStatusResponse> {
  return requestJSON<RuntimeStatusResponse>("/agent/runtime/status");
}


export function setAuthorityMode(authorityMode: AuthorityMode): Promise<{ status?: string; authority_mode?: AuthorityMode; error?: string }> {
  return requestJSON("/agent/authority", { method: "POST", body: JSON.stringify({ authority_mode: authorityMode }) });
}

export function stopTurn(payload: { conversation_id: string; goal_id?: string; run_id?: string; turn_id?: string; reason?: string }): Promise<{ status?: string; goal_status?: string; checkpoint_ref?: string; error?: string }> {
  return requestJSON("/agent/turn/stop", { method: "POST", body: JSON.stringify(payload) });
}

export function fetchAgentEvents(conversationID: string, since = 0, limit = 120): Promise<AgentEventsResponse> {
  const query = new URLSearchParams({
    conversation_id: conversationID,
    since: String(Math.max(0, since)),
    limit: String(Math.max(1, limit))
  });
  return requestJSON<AgentEventsResponse>(`/agent/events?${query.toString()}`);
}

export function selectAudition(conversationID: string, sessionID: string, candidateID: string): Promise<{ status?: string; session?: JsonRecord; error?: string }> {
  return requestJSON("/agent/audition/select", { method: "POST", body: JSON.stringify({ conversation_id: conversationID, session_id: sessionID, candidate_id: candidateID }) });
}

export function stopAudition(conversationID: string, sessionID: string): Promise<{ status?: string; session?: JsonRecord; error?: string }> {
  return requestJSON("/agent/audition/stop", { method: "POST", body: JSON.stringify({ conversation_id: conversationID, session_id: sessionID }) });
}

export interface AuditionJudgmentPayload {
  conversation_id: string;
  turn_id: string;
  round_id: string;
  audition_session_id: string;
  project_revision: string;
  heard_difference: "yes" | "no" | "unsure";
  preference: "a" | "b" | "neither" | "equal" | "unsure";
  reason_tags?: string[];
  free_text?: string;
}

export function submitAuditionJudgment(payload: AuditionJudgmentPayload): Promise<{ status?: string; evidence?: JsonRecord; error?: string }> {
  return requestJSON("/agent/audition/judgment", { method: "POST", body: JSON.stringify(payload) });
}

export function inspectAuditionCandidate(conversationID: string, sessionID: string, candidateID: string): Promise<{ status?: string; receipt?: JsonRecord; error?: string }> {
  return requestJSON("/agent/audition/inspect_candidate", { method: "POST", body: JSON.stringify({ conversation_id: conversationID, audition_session_id: sessionID, candidate_id: candidateID }) });
}

export function applyAuditionCandidate(conversationID: string, sessionID: string, candidateID: string, judgmentEvidenceID: string): Promise<{ status?: string; receipt?: JsonRecord; error?: string }> {
  return requestJSON("/agent/audition/apply_candidate", { method: "POST", body: JSON.stringify({ conversation_id: conversationID, audition_session_id: sessionID, candidate_id: candidateID, judgment_evidence_id: judgmentEvidenceID }) });
}

export function fetchUIState(): Promise<AgentUIState> {
  return requestJSON<AgentUIState>("/agent/ui/state");
}

export function fetchAgentConfig(): Promise<AgentConfigResponse> {
  return requestJSON<AgentConfigResponse>("/agent/config");
}

export function saveAgentConfig(config: EngineConfig): Promise<AgentConfigResponse> {
  return requestJSON<AgentConfigResponse>("/agent/config", {
    method: "POST",
    body: JSON.stringify(config)
  });
}

export function sendChat(payload: ChatRequest): Promise<ChatResponse> {
  return requestJSON<ChatResponse>("/agent/chat", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export function respondInteraction(payload: InteractionRespondRequest): Promise<ChatResponse> {
  return requestJSON<ChatResponse>("/agent/interaction/respond", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export function confirmPlan(payload: { plan_id: string; decision: string }): Promise<ChatResponse> {
  return requestJSON<ChatResponse>("/agent/confirm", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export function invokeAgent(payload: AgentInvokeRequest): Promise<AgentInvokeResponse> {
  return requestJSON<AgentInvokeResponse>("/agent/invoke", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export function setMacroControlValue(payload: {
  macro_id: string;
  value: number;
  macro?: MacroControl;
  commit?: boolean;
  source?: string;
}): Promise<MacroControlSetResponse> {
  return invokeAgent({
    tool: "control_set_macro_values",
    args: {
      macro_id: payload.macro_id,
      value: payload.value,
      macro: payload.macro,
      commit: payload.commit ?? true,
      source: payload.source ?? "ask_vit_webui"
    },
    source: payload.source ?? "ask_vit_webui",
    confirmed: true
  }) as Promise<MacroControlSetResponse>;
}

export function renameMacroControl(payload: {
  macro_id: string;
  name: string;
  macro?: MacroControl;
  source?: string;
}): Promise<AgentInvokeResponse> {
  return invokeAgent({
    tool: "control_rename_macro",
    args: {
      macro_id: payload.macro_id,
      name: payload.name,
      macro: payload.macro,
      source: payload.source ?? "ask_vit_webui"
    },
    source: payload.source ?? "ask_vit_webui",
    confirmed: true
  });
}

export function uploadArtifacts(
  files: FileList | File[],
  metadata: {
    conversation_id?: string;
    goal_id?: string;
    run_id?: string;
    project_path?: string;
    root_project_path?: string;
    active_worktree?: string;
    active_branch?: string;
    active_node_id?: string;
    history_scope_key?: string;
    media_scope_key?: string;
	track_id?: string;
    plugin_id?: string;
    plugin_name?: string;
  } = {}
): Promise<ArtifactListResponse> {
  const form = new FormData();
  Array.from(files).forEach((file) => form.append("file", file));
  Object.entries(metadata).forEach(([key, value]) => {
    if (value) {
      form.append(key, value);
    }
  });
  return requestJSON<ArtifactListResponse>("/agent/artifacts/upload", {
    method: "POST",
    body: form
  });
}

export function listArtifacts(options: {
  limit?: number;
  includeInternal?: boolean;
  kind?: string;
  source?: string;
} = {}): Promise<ArtifactListResponse> {
  const query = new URLSearchParams();
  query.set("limit", String(options.limit ?? 80));
  if (options.includeInternal) {
    query.set("include_internal", "true");
  }
  if (options.kind) {
    query.set("kind", options.kind);
  }
  if (options.source) {
    query.set("source", options.source);
  }
  return requestJSON<ArtifactListResponse>(`/agent/artifacts?${query.toString()}`);
}

export function readArtifact(id: string): Promise<ArtifactReadResponse> {
  return requestJSON<ArtifactReadResponse>(`/agent/artifact?id=${encodeURIComponent(id)}`);
}

export function renameArtifact(id: string, title: string): Promise<ArtifactReadResponse> {
  return requestJSON<ArtifactReadResponse>(`/agent/artifact?id=${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify({ title })
  });
}

export function deleteArtifact(id: string): Promise<ArtifactReadResponse> {
  return requestJSON<ArtifactReadResponse>(`/agent/artifact?id=${encodeURIComponent(id)}`, {
    method: "DELETE"
  });
}

export function revealArtifact(id: string): Promise<{ status?: string; path?: string; error?: string }> {
  return requestJSON(`/agent/artifact/reveal?id=${encodeURIComponent(id)}`, {
    method: "POST"
  });
}

export function artifactFileURL(id: string): string {
  return apiPath(`/agent/artifact/file?id=${encodeURIComponent(id)}`);
}

export function captureBrowserPage(payload: {
  url?: string;
  title?: string;
  text?: string;
  selection_text?: string;
  conversation_id?: string;
  goal_id?: string;
  run_id?: string;
  project_path?: string;
  root_project_path?: string;
  active_worktree?: string;
  active_branch?: string;
  active_node_id?: string;
  history_scope_key?: string;
  media_scope_key?: string;
}): Promise<{ status?: string; artifact?: Artifact; error?: string }> {
  return requestJSON("/agent/browser/capture", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export function registerResourceURL(payload: {
  url: string;
  title?: string;
  conversation_id?: string;
  goal_id?: string;
  run_id?: string;
  project_path?: string;
  root_project_path?: string;
  active_worktree?: string;
  active_branch?: string;
  active_node_id?: string;
  history_scope_key?: string;
  media_scope_key?: string;
}): Promise<ResourceIntakeResponse> {
  return requestJSON("/agent/resource/url", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export function scanDownloads(payload: {
  since_minutes?: number;
  limit?: number;
  change_token?: string;
  only_if_changed?: boolean;
  conversation_id?: string;
  goal_id?: string;
  run_id?: string;
  project_path?: string;
  root_project_path?: string;
  active_worktree?: string;
  active_branch?: string;
  active_node_id?: string;
  history_scope_key?: string;
  media_scope_key?: string;
}): Promise<ResourceIntakeResponse> {
  return requestJSON("/agent/downloads/scan", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

export function watchDownloads(payload: {
  change_token?: string;
  timeout_ms?: number;
  poll_ms?: number;
  download_dirs?: string[];
}): Promise<ResourceIntakeResponse> {
  return requestJSON("/agent/downloads/watch", {
    method: "POST",
    body: JSON.stringify(payload)
  });
}
