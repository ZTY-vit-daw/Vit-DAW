export type AgentMode = "default" | "plan" | "goal";

export type JsonRecord = Record<string, unknown>;

export interface HealthResponse {
  status?: string;
  service?: string;
}

export interface RuntimeStatusResponse {
  status?: string;
  service?: string;
  pid?: number;
  checked_at?: string;
  kernel?: JsonRecord;
  shadow?: JsonRecord;
}

export interface AgentEvent {
  seq: number;
  type: string;
  conversation_id?: string;
  goal_id?: string;
  run_id?: string;
  item_id?: string;
  item_type?: string;
  status?: string;
  title?: string;
  body?: string;
  payload?: JsonRecord;
  created_at?: string;
}

export interface AgentEventsResponse {
  status?: string;
  events?: AgentEvent[];
  next_seq?: number;
  error?: string;
}

export interface MultimodalRouteConfig {
  enabled?: boolean;
  provider?: string;
  baseUrl?: string;
  apiKey?: string;
  model?: string;
  priority?: number;
  fallback?: string;
}

export interface BrowserConfig {
  enabled?: boolean;
  mode?: string;
  profilePath?: string;
  rememberSession?: boolean;
}

export interface EngineConfig {
  baseUrl?: string;
  apiKey?: string;
  defaultModel?: string;
  multimodalRoutes?: Record<string, MultimodalRouteConfig>;
  browser?: BrowserConfig;
}

export interface AgentConfigResponse {
  status?: string;
  path?: string;
  config?: EngineConfig;
  hasApiKey?: boolean;
  routeApiKeys?: Record<string, boolean>;
  error?: string;
}

export interface ArtifactSummary {
  id: string;
  kind?: string;
  title?: string;
  source?: string;
  path?: string;
  url?: string;
  mime?: string;
  size_bytes?: number;
  status?: string;
  summary?: string;
  metadata?: JsonRecord;
  created_at?: string;
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
}

export interface Artifact extends ArtifactSummary {
  text?: string;
}

export interface AgentUIState {
  status?: string;
  project?: JsonRecord;
  transport?: JsonRecord;
  tracks?: JsonRecord[];
  selected_track?: JsonRecord | null;
  selected_plugin?: JsonRecord | null;
  plugin_rack?: JsonRecord | null;
  ui_context?: JsonRecord | null;
  goal?: JsonRecord | null;
  agent_plan?: JsonRecord | null;
  artifacts?: ArtifactSummary[];
  project_history?: JsonRecord | null;
  capabilities?: JsonRecord | null;
  macro_controls?: MacroControl[];
}

export interface MacroControlBinding {
  binding_id?: string;
  control?: string;
  track_id?: string;
  plugin_id?: string;
  param_id?: string;
  param_name?: string;
  source_min?: number;
  source_max?: number;
  target_min?: number;
  target_max?: number;
  enabled?: boolean;
  unit?: string;
}

export interface MacroControl {
  macro_id: string;
  id?: string;
  name?: string;
  track_id?: string;
  plugin_id?: string;
  role?: string;
  control?: string;
  control_type?: string;
  value?: number;
  min?: number;
  max?: number;
  unit?: string;
  bindings?: MacroControlBinding[];
  binding_count?: number;
  source?: string;
  status?: string;
  enabled?: boolean;
  updated_at?: string;
  updated_at_unix?: number;
}

export interface ChatRequest {
  conversation_id: string;
  message: string;
  context?: JsonRecord;
  artifact_refs?: string[];
}

export interface ChatResponse {
  status?: string;
  message?: string;
  conversation_id?: string;
  goal_id?: string;
  run_id?: string;
  reply?: string;
  agent_mode?: string;
  agent_plan?: JsonRecord;
  needs_confirmation?: boolean;
  plan_id?: string;
  preview?: string;
  workflow?: string;
  workflow_data?: JsonRecord;
  plugin_learning?: JsonRecord;
  mix_session?: JsonRecord;
  interaction_requests?: JsonRecord[];
  typed_events?: JsonRecord[];
  acoustic_package_status?: JsonRecord;
  acoustic_package_status_path?: string;
  commands?: JsonRecord[];
  executed_kernel_reply?: JsonRecord[];
  project_result_cards?: JsonRecord[];
  goal_status?: string;
  goal_summary?: string;
  current_step?: string;
  completed_steps?: number;
  stop_reason?: string;
  limit_type?: string;
  project_history?: JsonRecord;
  artifacts?: ArtifactSummary[];
  side_panel_request?: {
    view?: string;
    tab?: string;
    artifact_id?: string;
    macro_panel_id?: string;
  };
  error?: string;
}

export interface InteractionRespondRequest {
  interaction_id: string;
  decision?: string;
  action_id?: string;
  payload?: JsonRecord;
}

export interface AgentInvokeRequest {
  tool?: string;
  args?: JsonRecord;
  command?: JsonRecord;
  context?: JsonRecord;
  source?: string;
  confirmed?: boolean;
  run_id?: string;
  goal_id?: string;
  tool_call_id?: string;
}

export interface AgentInvokeResponse {
  status?: string;
  agent_action_id?: string;
  tool?: string;
  command_name?: string;
  risk_level?: string;
  requires_confirmation?: boolean;
  preview?: string;
  undo_label?: string;
  result?: JsonRecord;
  acoustic_package_status?: JsonRecord;
  acoustic_package_status_path?: string;
  project_history?: JsonRecord;
  error?: string;
}

export interface MacroControlSetResponse extends AgentInvokeResponse {}

export interface ChatMessage {
  id: string;
  source_id?: string;
  role: "user" | "assistant" | "system";
  content: string;
  mode?: AgentMode | string;
  pluginLearningProgress?: {
    sessionID: string;
    title?: string;
    targetName?: string;
    live?: boolean;
  };
  artifacts?: ArtifactSummary[];
  actions?: JsonRecord[];
  createdAt: number;
  status?: "pending" | "sent" | "error";
}

export interface ArtifactReadResponse {
  status?: string;
  artifact?: Artifact;
  error?: string;
}

export interface ArtifactListResponse {
  status?: string;
  artifacts?: ArtifactSummary[];
  root?: string;
  error?: string;
}

export interface ResourceIntakeResponse {
  status?: string;
  artifact?: ArtifactSummary;
  artifacts?: ArtifactSummary[];
  download_dirs?: string[];
  change_token?: string;
  changed?: boolean;
  count?: number;
  error?: string;
}
