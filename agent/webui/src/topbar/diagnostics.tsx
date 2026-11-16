// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2h）：App.tsx 诊断件（P 域）出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
import type { AgentConfigResponse, AgentUIState, JsonRecord, RuntimeStatusResponse } from "../types";
import { asRecord, lastPathPart, textValue, truthy } from "../lib/util";
import { historyScopeKeyFromUIState } from "../historyScope";

export function InfoGrid({ rows }: { rows: Array<[string, string]> }) {
  return (
    <div className="info-grid">
      {rows.map(([label, value]) => (
        <div key={label}>
          <span>{label}</span>
          <strong>{value}</strong>
        </div>
      ))}
    </div>
  );
}
export function KeyValue({ label, value }: { label: string; value: string }) {
  return (
    <div className="key-value">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

export function buildDiagnosticsRows(runtimeStatus: RuntimeStatusResponse | null, uiState: AgentUIState | null): Array<[string, string]> {
  const kernel = asRecord(runtimeStatus?.kernel);
  const shadow = asRecord(runtimeStatus?.shadow);
  const project = asRecord(uiState?.project);
  const history = asRecord(uiState?.project_history);
  const selectedPlugin = asRecord(uiState?.selected_plugin);
  const uiContext = asRecord(uiState?.ui_context);
  return [
    ["Agent", runtimeStatus?.service ?? "未连接"],
    ["PID", textValue(runtimeStatus?.pid, "-")],
    ["内核", truthy(kernel.connected) || textValue(kernel.status, "").toLowerCase() === "ok" ? "已连接" : textValue(kernel.error ?? kernel.status, "未知")],
    ["影子状态", truthy(shadow.initialized) || truthy(project.initialized) ? "已初始化" : "未初始化"],
    ["轨道", textValue(project.user_track_count ?? project.track_count ?? uiState?.tracks?.length, "0")],
    ["工程", lastPathPart(textValue(history.project_path ?? project.project_path, "未保存工程"))],
    ["历史作用域", historyScopeKeyFromUIState(uiState) || "-"],
    ["当前节点", textValue(history.active_node_id, "-")],
    ["工作树", textValue(history.active_worktree, "main")],
    ["选中插件", textValue(selectedPlugin.plugin_name ?? uiContext.selected_plugin_name, "-")],
    ["插件 ID", textValue(selectedPlugin.plugin_id ?? uiContext.selected_plugin_id, "-")],
    ["状态版本", textValue(shadow.last_delta_seq ?? project.last_delta_seq, "-")],
    ["版本缺口", textValue(shadow.delta_seq_gaps ?? project.delta_seq_gaps, "0")]
  ];
}

export function buildDiagnosticsPayload(
  runtimeStatus: RuntimeStatusResponse | null,
  uiState: AgentUIState | null,
  configResponse: AgentConfigResponse | null
): JsonRecord {
  const project = asRecord(uiState?.project);
  const history = asRecord(uiState?.project_history);
  const uiContext = asRecord(uiState?.ui_context);
  return {
    runtime: runtimeStatus,
    history_scope_key: historyScopeKeyFromUIState(uiState),
    project: {
      project_path: project.project_path,
      track_count: project.track_count,
      user_track_count: project.user_track_count,
      graph_revision: project.graph_revision,
      last_delta_seq: project.last_delta_seq,
      delta_seq_gaps: project.delta_seq_gaps
    },
    project_history: {
      project_path: history.project_path,
      root_project_path: history.root_project_path,
      active_branch: history.active_branch,
      active_node_id: history.active_node_id,
      active_worktree: history.active_worktree,
      head: history.head,
      draft: history.draft,
      initialized: history.initialized,
      commit_count: history.commit_count,
      worktree_count: history.worktree_count,
      warnings: history.warnings
    },
    selected_track: uiState?.selected_track ?? null,
    selected_plugin: uiState?.selected_plugin ?? null,
    ui_context: uiContext,
    configPath: configResponse?.path,
    hasApiKey: configResponse?.hasApiKey,
    routeApiKeys: configResponse?.routeApiKeys
  };
}

export function EmptyState({ label }: { label: string }) {
  return <div className="empty-state">{label}</div>;
}

