// T9-PRE-1A-1（施工图 UI-CONVERGE-RECON-1 §3.1 批 1 / 步 1f-B）：App.tsx settings config/上下文构建域（S 段）出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
// 附带迁移：multimodalRouteMeta（设置域元数据，App 头部 const 原位迁入，App 其余使用经兼容层引回，
// 消除 settings→App 常量循环的 TDZ 风险）。artifactHistoryScopeKeyFromUIState/artifactLabel 属
// App 响应解析/资料库域（2b/2d 落点），经 App 兼容层引用（函数声明，循环安全）。

import type { AgentMode, AgentUIState, ArtifactSummary, AuthorityMode, EngineConfig, JsonRecord, MacroControl, MultimodalRouteConfig } from "../types";
import { authorityContext } from "../turnControl";
import { asRecord, firstArray, textValue, truthy } from "../lib/util";
import { isSystemRackPluginRecord, selectedClipIDFromUIState, selectedClipIDsFromUIState, selectedTrackIDFromUIState } from "../daw/parse";
import { macroControlID, macroControlLabel, macroControlValue, macroControlsFromUIState, normalizeMacroControl } from "../macro/normalize";
import { artifactHistoryScopeKeyFromUIState, artifactLabel } from "../App";

export const multimodalRouteMeta: Array<{ key: string; label: string; hint: string }> = [
  { key: "text", label: "文本 LLM", hint: "Agent 对话与规划" },
  { key: "image_understanding", label: "图片理解", hint: "图片、截图、视觉素材分析" },
  { key: "image_generation", label: "图片生成", hint: "生成图像与设计稿" },
  { key: "audio_understanding", label: "音频理解", hint: "音频内容和素材分析" },
  { key: "transcription", label: "音频转写", hint: "语音、采样说明、口述转写" },
  { key: "video_browser", label: "视频/浏览器理解", hint: "WebView2 页面与视频上下文" },
  { key: "embeddings", label: "Embedding / 检索", hint: "历史、媒体和工程语义检索" }
];


export function defaultSettingsConfig(): EngineConfig {
  return {
    baseUrl: "",
    apiKey: "",
    defaultModel: "",
    multimodalRoutes: Object.fromEntries(
      multimodalRouteMeta.map((meta, index) => [
        meta.key,
        {
          enabled: meta.key === "text",
          provider: "default",
          baseUrl: "",
          apiKey: "",
          model: "",
          priority: (index + 1) * 10,
          fallback: ""
        } satisfies MultimodalRouteConfig
      ])
    ),
    browser: {
      enabled: false,
      mode: "webview2_companion",
      profilePath: "",
      rememberSession: true
    }
  };
}

export function mergeSettingsConfig(config?: EngineConfig): EngineConfig {
  const defaults = defaultSettingsConfig();
  return {
    ...defaults,
    ...(config ?? {}),
    multimodalRoutes: {
      ...(defaults.multimodalRoutes ?? {}),
      ...(config?.multimodalRoutes ?? {})
    },
    browser: {
      ...(defaults.browser ?? {}),
      ...(config?.browser ?? {})
    },
    apiKey: ""
  };
}

// 保存成功后 response.config 是 RedactSecrets 的产物（主/路由 key 均无明文）；
// 以 merged 为基准、沿用 current 的 key 输入态，其余字段以服务端回传为准。
export function retainSecretInputs(merged: EngineConfig, current: EngineConfig): EngineConfig {
  const mergedRoutes = merged.multimodalRoutes ?? {};
  const currentRoutes = current.multimodalRoutes ?? {};
  const multimodalRoutes: Record<string, MultimodalRouteConfig> = { ...mergedRoutes };
  for (const key of Object.keys(currentRoutes)) {
    multimodalRoutes[key] = { ...(mergedRoutes[key] ?? currentRoutes[key]), apiKey: currentRoutes[key].apiKey ?? "" };
  }
  return { ...merged, apiKey: current.apiKey ?? "", multimodalRoutes };
}

// 状态行区分两种保存情形：已输入 key（输入态保持）与留空（沿用已存 key）。
export function settingsSavedStatus(apiKey: string, hasApiKey: boolean): string {
  if (apiKey.trim()) {
    return "已保存。API Key 已更新，输入框保留所输内容；下一条消息会使用新配置。";
  }
  if (hasApiKey) {
    return "已保存。API Key 留空=沿用已存 key；下一条消息会使用新配置。";
  }
  return "已保存。下一条消息会使用新配置。";
}

function selectedPluginContextFromUIState(uiState: AgentUIState | null): { track_id: string; plugin_id: string; plugin_name: string } {
  const selectedPlugin = asRecord(uiState?.selected_plugin);
  const uiContext = asRecord(uiState?.ui_context);
  const rack = asRecord(uiState?.plugin_rack);
  const rackTrack = asRecord(rack.track);
  const selectedTrack = asRecord(uiState?.selected_track);
  const plugins = firstArray(rack.plugins, rack.items, rack.chain, rack.rack)
    .map(asRecord)
    .filter((item) => Object.keys(item).length > 0 && !isSystemRackPluginRecord(item));
  const rackSelection = plugins.find((plugin) => truthy(plugin.selected) || truthy(plugin.current) || truthy(plugin.active) || truthy(plugin.focused)) ??
    (plugins.length === 1 ? plugins[0] : {});
  return {
    track_id: textValue(selectedPlugin.track_id ?? uiContext.selected_plugin_track_id ?? uiContext.selected_track_id ?? rackTrack.track_id ?? rackTrack.id ?? selectedTrack.track_id ?? selectedTrack.id, ""),
    plugin_id: textValue(selectedPlugin.plugin_id ?? uiContext.selected_plugin_id ?? rackSelection.plugin_id ?? rackSelection.id ?? rackSelection.plugin_item_id ?? rackSelection.item_id, ""),
    plugin_name: textValue(selectedPlugin.plugin_name ?? uiContext.selected_plugin_name ?? rackSelection.plugin_name ?? rackSelection.name ?? rackSelection.title, "")
  };
}

function buildChatContext(mode: AgentMode, uiState: AgentUIState | null, artifacts: ArtifactSummary[], macroRefs: MacroControl[] = [], authorityMode: AuthorityMode = "manual_confirmation"): JsonRecord {
  const selectedTrack = asRecord(uiState?.selected_track);
  const uiContext = asRecord(uiState?.ui_context);
  const selectedTrackID = selectedTrackIDFromUIState(uiState);
  const selectedTrackName = textValue(selectedTrack.name ?? selectedTrack.track_name ?? uiContext.selected_track_name, "");
  const selectedClipID = selectedClipIDFromUIState(uiState);
  const selectedClipIDs = Array.from(selectedClipIDsFromUIState(uiState));
  const selectedClipTrackID = textValue(uiContext.selected_clip_track_id ?? selectedTrackID, "");
  const pianoRollFocusClipID = textValue(uiContext.piano_roll_focus_clip_id, "");
  const pianoRollFocusTrackID = textValue(uiContext.piano_roll_focus_track_id, "");
  const playheadSeconds = uiContext.playhead_seconds ?? uiContext.current_playhead_seconds ?? uiContext.transport_position_seconds;
  const currentSelection = compactChatContextRecord({
    selected_track_id: selectedTrackID,
    selected_track_name: selectedTrackName,
    selected_clip_id: selectedClipID,
    selected_clip_ids: selectedClipIDs,
    selected_clip_track_id: selectedClipTrackID,
    piano_roll_focus_clip_id: pianoRollFocusClipID,
    piano_roll_focus_track_id: pianoRollFocusTrackID,
    playhead_seconds: playheadSeconds,
    current_playhead_seconds: uiContext.current_playhead_seconds ?? playheadSeconds,
    transport_position_seconds: uiContext.transport_position_seconds ?? playheadSeconds
  });
  const mergedUIContext = compactChatContextRecord({
    ...uiContext,
    ...currentSelection
  });
  const pluginTarget = selectedPluginContextFromUIState(uiState);
  const project = asRecord(uiState?.project);
  const projectHistory = asRecord(uiState?.project_history);
  const scope = artifactScopeMetadata(uiState);
  const macroControls = macroControlsFromUIState(uiState);
  return compactChatContextRecord({
    agent_mode: mode,
    ...authorityContext(authorityMode),
    // WEBUI-IA-REDESIGN-1: focus 视窗族已删，聊天上下文固定对话面
    active_focus: "dialogue",
    history_scope_key: scope.history_scope_key,
    media_scope_key: scope.media_scope_key,
    project_path: textValue(projectHistory.project_path ?? projectHistory.current_project_path ?? project.project_path, ""),
    root_project_path: textValue(projectHistory.root_project_path, ""),
    active_branch: textValue(projectHistory.active_branch, ""),
    active_node_id: textValue(projectHistory.active_node_id, ""),
    active_worktree: textValue(projectHistory.active_worktree, ""),
    selected_track_id: selectedTrackID,
    selected_track_name: selectedTrackName,
    selected_clip_id: selectedClipID,
    selected_clip_ids: selectedClipIDs,
    selected_clip_track_id: selectedClipTrackID,
    piano_roll_focus_clip_id: pianoRollFocusClipID,
    piano_roll_focus_track_id: pianoRollFocusTrackID,
    playhead_seconds: playheadSeconds,
    current_playhead_seconds: currentSelection.current_playhead_seconds,
    transport_position_seconds: currentSelection.transport_position_seconds,
    current_selection: currentSelection,
    ui_context: mergedUIContext,
    selected_plugin_track_id: pluginTarget.track_id,
    selected_plugin_id: pluginTarget.plugin_id,
    selected_plugin_name: pluginTarget.plugin_name,
    macro_refs: macroRefs.map(compactMacroForContext),
    available_macro_controls: macroControls.slice(0, 12).map(compactMacroForContext),
    artifacts: artifacts.map((artifact) => ({
      id: artifact.id,
      kind: artifact.kind,
      title: artifactLabel(artifact),
      mime: artifact.mime
    }))
  });
}

function compactChatContextRecord(record: JsonRecord): JsonRecord {
  return Object.fromEntries(
    Object.entries(record).filter(([, value]) => {
      if (Array.isArray(value)) {
        return value.length > 0;
      }
      if (value && typeof value === "object") {
        return Object.keys(value as JsonRecord).length > 0;
      }
      return textValue(value, "") !== "";
    })
  );
}

function compactMacroForContext(macro: MacroControl): JsonRecord {
  const normalized = normalizeMacroControl(asRecord(macro));
  return {
    macro_id: macroControlID(normalized),
    name: macroControlLabel(normalized),
    track_id: normalized.track_id,
    plugin_id: normalized.plugin_id,
    role: normalized.role,
    control: normalized.control,
    value: macroControlValue(normalized),
    binding_count: normalized.bindings?.length ?? 0,
    bindings: (normalized.bindings ?? []).slice(0, 6).map((binding) => ({
      control: binding.control,
      track_id: binding.track_id,
      plugin_id: binding.plugin_id,
      param_id: binding.param_id,
      param_name: binding.param_name,
      target_min: binding.target_min,
      target_max: binding.target_max,
      enabled: binding.enabled !== false
    }))
  };
}

function uploadMetadata(conversationID: string, uiState: AgentUIState | null): {
  conversation_id: string;
  goal_id?: string;
  run_id?: string;
  project_path?: string;
  root_project_path?: string;
  active_worktree?: string;
  active_branch?: string;
  active_node_id?: string;
  history_scope_key?: string;
  media_scope_key?: string;
} {
  const goal = asRecord(uiState?.goal);
  const plan = asRecord(uiState?.agent_plan);
  return {
    conversation_id: conversationID,
    goal_id: textValue(goal.goal_id ?? plan.goal_id, ""),
    run_id: textValue(goal.run_id ?? plan.run_id, ""),
    ...artifactScopeMetadata(uiState)
  };
}

function artifactScopeMetadata(uiState: AgentUIState | null): {
  project_path?: string;
  root_project_path?: string;
  active_worktree?: string;
  active_branch?: string;
  active_node_id?: string;
  history_scope_key?: string;
  media_scope_key?: string;
} {
  const project = asRecord(uiState?.project);
  const projectHistory = asRecord(uiState?.project_history);
  const uiContext = asRecord(uiState?.ui_context);
  const projectPath = textValue(
    projectHistory.project_path ?? projectHistory.current_project_path ?? project.project_path ?? uiContext.project_path ?? uiContext.current_project_path,
    ""
  );
  const rootProjectPath = textValue(projectHistory.root_project_path, "");
  const activeWorktree = textValue(projectHistory.active_worktree, "");
  const activeBranch = textValue(projectHistory.active_branch, "");
  const activeNodeID = textValue(projectHistory.active_node_id, "");
  const historyScopeKey = artifactHistoryScopeKeyFromUIState(uiState);
  return compactMetadata({
    project_path: projectPath,
    root_project_path: rootProjectPath,
    active_worktree: activeWorktree,
    active_branch: activeBranch,
    active_node_id: activeNodeID,
    history_scope_key: historyScopeKey,
    media_scope_key: mediaScopeKeyFromParts(projectPath, rootProjectPath, activeWorktree, activeBranch, historyScopeKey)
  });
}

function mediaScopeKeyFromUIState(uiState: AgentUIState | null): string {
  return artifactScopeMetadata(uiState).media_scope_key ?? "";
}

function mediaScopeKeyFromParts(projectPath: string, rootProjectPath: string, activeWorktree: string, activeBranch: string, historyScopeKey: string): string {
  if (rootProjectPath) {
    return ["root", rootProjectPath, activeWorktree || "main", activeBranch || "main"].join("::");
  }
  if (projectPath) {
    return `project::${projectPath}`;
  }
  if (historyScopeKey) {
    return `history::${historyScopeKey}`;
  }
  return "";
}

function compactMetadata<T extends Record<string, string>>(metadata: T): Partial<T> {
  return Object.fromEntries(Object.entries(metadata).filter(([, value]) => value.trim() !== "")) as Partial<T>;
}

export {
  selectedPluginContextFromUIState,
  buildChatContext,
  compactChatContextRecord,
  compactMacroForContext,
  uploadMetadata,
  artifactScopeMetadata,
  mediaScopeKeyFromUIState,
  mediaScopeKeyFromParts,
  compactMetadata
};
