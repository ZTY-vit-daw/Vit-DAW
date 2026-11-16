// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2h）：App.tsx TopStatusBar+诊断条（E 域）出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
import { ChevronLeft, ChevronRight, Gauge, Play, Radio, RefreshCw, Square, Volume2 } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useState } from "react";
import type { AgentUIState, JsonRecord, RuntimeStatusResponse } from "../types";
import { asRecord, lastPathPart, textValue } from "../lib/util";
import { statusLabel } from "../actions/labels";
import type { ConnectionStatus } from "../App";

export function TopStatusBar({
  connection,
  healthLabel,
  runtimeStatus,
  uiState,
  conversationTitle,
  onOpenHistory,
  onRefresh,
  onTransportCommand,
  transportBusy
}: {
  connection: ConnectionStatus;
  healthLabel: string;
  runtimeStatus: RuntimeStatusResponse | null;
  uiState: AgentUIState | null;
  conversationTitle: string;
  onOpenHistory: () => void;
  onRefresh: () => Promise<void>;
  onTransportCommand: (tool: string, args?: JsonRecord) => Promise<void>;
  transportBusy: boolean;
}) {
  const [seekDraft, setSeekDraft] = useState("");
  const project = asRecord(uiState?.project);
  const transport = asRecord(uiState?.transport);
  const projectPath = textValue(project.project_path, "Vit Project");
  const projectName = lastPathPart(projectPath);
  const rawPlayState = textValue(transport.play_state ?? transport.state ?? transport.status, connection === "ready" ? "ready" : "offline");
  const playState = statusLabel(rawPlayState, rawPlayState);
  const timecode = textValue(transport.timecode ?? transport.position ?? transport.time, "00:00:00");
  const bpm = textValue(transport.bpm ?? transport.tempo, "--");
  const sampleRate = textValue(transport.sample_rate ?? transport.sampleRate, "--");
  const diagnosticLine = runtimeDiagnosticLine(healthLabel, runtimeStatus, connection);
  const diagnosticTitle = runtimeDiagnosticTitle(runtimeStatus);
  const transportDisabled = connection !== "ready" || transportBusy;
  const submitSeek = () => {
    const seconds = Number(seekDraft);
    if (!Number.isFinite(seconds) || seconds < 0) {
      return;
    }
    void onTransportCommand("transport.seek", { time: seconds });
    setSeekDraft("");
  };

  return (
    <>
      <header className="top-status">
      <div className="brand-block">
        <div className="vit-mondrian-mark" aria-label="Vit"><strong>V</strong><i /><b /></div>
        <div className="conversation-identity">
          <button type="button" className="conversation-title-button" title="打开历史树切换对话流" onClick={onOpenHistory}>
            <strong>{conversationTitle}</strong>
            <ChevronRight size={14} />
          </button>
          <span title={`${projectPath} · ${diagnosticTitle}`}><i className={`connection-dot ${connection}`} />{projectName} · {diagnosticLine}</span>
        </div>
      </div>

      <div className="transport-strip" aria-label="Transport">
        <button className="icon-button" type="button" title="Play" disabled={transportDisabled} onClick={() => void onTransportCommand("transport.play")}>
          <Play size={16} />
        </button>
        <button className="icon-button" type="button" title="Return to zero" disabled={transportDisabled} onClick={() => void onTransportCommand("transport.return_to_zero")}>
          <ChevronLeft size={16} />
        </button>
        <button className="icon-button" type="button" title="Stop" disabled={transportDisabled} onClick={() => void onTransportCommand("transport.stop")}>
          <Square size={15} />
        </button>
        <span className="timecode">{timecode}</span>
        <input
          className="transport-seek-input"
          type="number"
          min="0"
          step="0.25"
          value={seekDraft}
          placeholder="sec"
          title="Seek seconds"
          disabled={transportDisabled}
          onChange={(event) => setSeekDraft(event.currentTarget.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              submitSeek();
            }
          }}
          onBlur={() => {
            if (seekDraft.trim()) {
              submitSeek();
            }
          }}
        />
        <span className="mini-meter">
          <i />
          <i />
          <i />
          <i />
          <i />
        </span>
      </div>

      <div className="status-cells">
        <StatusCell icon={Radio} label={playState} tone={connection === "ready" ? "ok" : "warn"} />
        <StatusCell icon={Gauge} label={`${bpm} BPM`} />
        <StatusCell icon={Volume2} label={sampleRate} />
        <button className="icon-button refresh" type="button" title="Refresh" onClick={() => void onRefresh()}>
          <RefreshCw size={16} />
        </button>
      </div>
    </header>
    </>
  );
}

export function StatusCell({ icon: Icon, label, tone = "neutral" }: { icon: LucideIcon; label: string; tone?: "neutral" | "ok" | "warn" }) {
  return (
    <span className={`status-cell ${tone}`}>
      <Icon size={15} />
      {label}
    </span>
  );
}

export function runtimeDiagnosticLine(healthLabel: string, runtimeStatus: RuntimeStatusResponse | null, connection: ConnectionStatus): string {
  if (connection !== "ready" || !runtimeStatus) {
    return healthLabel;
  }
  const kernel = asRecord(runtimeStatus.kernel);
  const shadow = asRecord(runtimeStatus.shadow);
  const kernelConnected = Boolean(kernel.connected);
  const pieces = [healthLabel, kernelConnected ? "内核已连接" : "内核未连接"];
  const trackCount = textValue(shadow.track_count ?? shadow.user_track_count, "");
  if (trackCount) {
    pieces.push(`${trackCount} 条轨道`);
  }
  const deltaSeq = textValue(shadow.last_delta_seq, "");
  if (deltaSeq) {
    pieces.push(`状态 #${deltaSeq}`);
  }
  return pieces.join(" 路 ");
}

export function runtimeDiagnosticTitle(runtimeStatus: RuntimeStatusResponse | null): string {
  if (!runtimeStatus) {
    return "Agent 运行状态不可用";
  }
  const kernel = asRecord(runtimeStatus.kernel);
  const shadow = asRecord(runtimeStatus.shadow);
  const lines = [
    `Agent: ${runtimeStatus.service ?? "VitAgent"}${runtimeStatus.pid ? ` pid=${runtimeStatus.pid}` : ""}`,
    `内核: ${Boolean(kernel.connected) ? "已连接" : "未连接"}${kernel.endpoint ? ` ${String(kernel.endpoint)}` : ""}`,
    `影子状态: ${Boolean(shadow.initialized) ? "已初始化" : "未初始化"}`
  ];
  const trackCount = textValue(shadow.track_count ?? shadow.user_track_count, "");
  if (trackCount) {
    lines.push(`轨道: ${trackCount}`);
  }
  const deltaSeq = textValue(shadow.last_delta_seq, "");
  if (deltaSeq) {
    lines.push(`最新状态版本 ${deltaSeq}`);
  }
  const kernelError = textValue(kernel.error, "");
  if (kernelError) {
    lines.push(`鍐呮牳閿欒: ${kernelError}`);
  }
  return lines.join("\n");
}
