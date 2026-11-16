// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2h）：App.tsx ProjectPane/TracksPane/PluginsPane 出域（§3.2 daw/ 2h 并入）。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
import { Plug } from "lucide-react";
import type { AgentUIState, JsonRecord } from "../types";
import { asRecord, firstArray, lastPathPart, textValue, truthy } from "../lib/util";
import { statusLabel, localizeDisplayText } from "../actions/labels";
import { EmptyState, InfoGrid, KeyValue } from "../topbar/diagnostics";

export function ProjectPane({ uiState }: { uiState: AgentUIState | null }) {
  const project = asRecord(uiState?.project);
  const goal = asRecord(uiState?.goal);
  const plan = asRecord(uiState?.agent_plan);
  const capabilities = asRecord(uiState?.capabilities);
  return (
    <div className="pane-body">
      <InfoGrid
        rows={[
          ["工程", lastPathPart(textValue(project.project_path, "Vit Project"))],
          ["轨道", textValue(project.user_track_count ?? project.track_count, "0")],
          ["图修订", textValue(project.graph_revision, "-")],
          ["工具", textValue(capabilities.tools, "-")]
        ]}
      />
      <section className="work-section">
        <h2>目标</h2>
        <KeyValue label="状态" value={statusLabel(goal.status ?? plan.status, "空闲")} />
        <KeyValue label="步骤" value={localizeDisplayText(textValue(goal.current_step ?? plan.current_step, "无")) || "无"} />
        <KeyValue label="停止" value={localizeDisplayText(textValue(goal.stop_reason ?? plan.stop_reason, "无")) || "无"} />
      </section>
    </div>
  );
}

export function TracksPane({ uiState }: { uiState: AgentUIState | null }) {
  const tracks = uiState?.tracks ?? [];
  return (
    <div className="pane-body">
      <section className="work-section">
        <h2>轨道</h2>
        <div className="track-list">
          {tracks.length === 0 && <EmptyState label="暂无轨道" />}
          {tracks.map((track, index) => (
            <TrackRow key={textValue(track.id ?? track.track_id, `track-${index}`)} track={track} />
          ))}
        </div>
      </section>
    </div>
  );
}

function TrackRow({ track }: { track: JsonRecord }) {
  const name = textValue(track.name ?? track.track_name, "Track");
  const type = textValue(track.type ?? track.kind, "audio");
  const selected = Boolean(track.selected);
  return (
    <div className={`track-row ${selected ? "selected" : ""}`}>
      <span className="track-color" />
      <div>
        <strong>{name}</strong>
        <span>{type}</span>
      </div>
      <div className="track-flags">
        <b className={truthy(track.mute) ? "on red" : ""}>M</b>
        <b className={truthy(track.solo) ? "on yellow" : ""}>S</b>
        <b className={truthy(track.arm ?? track.armed) ? "on blue" : ""}>R</b>
      </div>
    </div>
  );
}

export function PluginsPane({ uiState }: { uiState: AgentUIState | null }) {
  const rack = asRecord(uiState?.plugin_rack);
  const plugins = firstArray(rack.plugins, rack.items, rack.chain, rack.rack);
  return (
    <div className="pane-body">
      <section className="work-section">
        <h2>Plugin Rack</h2>
        {plugins.length === 0 && <EmptyState label="No plugins loaded" />}
        {plugins.map((raw, index) => {
          const plugin = asRecord(raw);
          return (
            <div className="plugin-row" key={textValue(plugin.id ?? plugin.plugin_id, `plugin-${index}`)}>
              <Plug size={16} />
              <div>
                <strong>{textValue(plugin.name ?? plugin.title, `Plugin ${index + 1}`)}</strong>
                <span>{textValue(plugin.vendor ?? plugin.status ?? plugin.kind, "rack node")}</span>
              </div>
            </div>
          );
        })}
      </section>
    </div>
  );
}

