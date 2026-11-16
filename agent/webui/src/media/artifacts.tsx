// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2d）：App.tsx artifact 工具域（T）出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
import { Activity, Archive, File, FileAudio, FileImage, FileText, Film, Globe, Plug, X } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { AgentUIState, Artifact, ArtifactSummary } from "../types";
import type { KeyboardEvent as ReactKeyboardEvent } from "react";
import { artifactFileURL } from "../lib/api";
import { asRecord, clampNumber, firstArray, formatBytes, lastPathPart, textValue } from "../lib/util";

type ArtifactContextVariant = "compact" | "message";

export function ArtifactContextCard({
  artifact,
  variant = "message",
  onSelect,
  onRemove
}: {
  artifact: ArtifactSummary;
  variant?: ArtifactContextVariant;
  onSelect: () => void;
  onRemove?: () => void;
}) {
  const presentation = artifactPresentation(artifact);
  const isImage = artifactIsImage(artifact);
  const handleKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onSelect();
    }
  };
  return (
    <div
      className={`artifact-context-card ${variant} ${presentation.kind}`}
      role="button"
      tabIndex={0}
      title={artifactAddress(artifact)}
      onClick={onSelect}
      onKeyDown={handleKeyDown}
    >
      <div className="artifact-context-thumb" aria-hidden="true">
        {isImage ? (
          <img src={artifactFileURL(artifact.id)} alt="" loading="lazy" />
        ) : (
          <presentation.Icon size={variant === "compact" ? 16 : 18} />
        )}
      </div>
      <div className="artifact-context-main">
        <span className="artifact-context-type">{presentation.label}</span>
        <strong>{artifactLabel(artifact)}</strong>
        <small>{presentation.detail}</small>
      </div>
      {onRemove && (
        <button
          className="artifact-context-remove"
          type="button"
          title="Remove"
          onClick={(event) => {
            event.stopPropagation();
            onRemove();
          }}
        >
          <X size={14} />
        </button>
      )}
    </div>
  );
}

export function artifactSummariesFrom(...values: unknown[]): ArtifactSummary[] {
  const seen = new Set<string>();
  const out: ArtifactSummary[] = [];
  values.forEach((value) => {
    firstArray(value).forEach((raw) => {
      const record = asRecord(raw);
      const id = textValue(record.id, "");
      if (!id || seen.has(id)) {
        return;
      }
      seen.add(id);
      out.push({ ...record, id } as ArtifactSummary);
    });
  });
  return out;
}

// artifact 定位上下文用的全量 scope 键（含 state_dir/worktree/branch 等晚物化
// 字段，与服务端 artifactHistoryScopeKey 语义对齐）。CONTRACT-3 起 history-sync
// 与会话/消息存储桶改用 historyScope.ts 的稳定身份键，不再消费本函数。
export function artifactHistoryScopeKeyFromUIState(uiState: AgentUIState | null): string {
  if (!uiState) {
    return "";
  }
  const projectHistory = asRecord(uiState.project_history);
  const project = asRecord(uiState.project);
  const uiContext = asRecord(uiState.ui_context);
  const projectPath = textValue(
    projectHistory.project_path ?? projectHistory.current_project_path ?? project.project_path ?? uiContext.project_path ?? uiContext.current_project_path,
    ""
  );
  const rootProjectPath = textValue(projectHistory.root_project_path, "");
  const stateDir = textValue(projectHistory.state_dir, "");
  const historyDir = textValue(projectHistory.history_dir, "");
  const activeWorktree = textValue(projectHistory.active_worktree, "");
  const activeBranch = textValue(projectHistory.active_branch, "");
  const projectIdentity = projectPath || rootProjectPath || textValue(project.project_path, "") || "unsaved";
  return [
    projectIdentity,
    rootProjectPath || "root",
    stateDir || historyDir || "state",
    activeWorktree || "main",
    activeBranch || "main"
  ].join("::");
}

export function renderArtifactPreview(artifact: ArtifactSummary | Artifact, options: { imageZoom?: number } = {}) {
  const kind = (artifact.kind ?? "").toLowerCase();
  const mime = (artifact.mime ?? "").toLowerCase();
  const fileURL = artifactFileURL(artifact.id);
  if (kind === "audio" || mime.startsWith("audio/")) {
    return <audio className="media-player" controls src={fileURL} />;
  }
  if (kind === "video" || mime.startsWith("video/")) {
    return <video className="media-player video" controls src={fileURL} />;
  }
  if (artifactIsImage(artifact)) {
    const zoom = clampNumber(options.imageZoom ?? 1, 0.35, 4);
    const imageStyle = zoom <= 1
      ? { maxWidth: "100%", maxHeight: "100%" }
      : { width: `${zoom * 100}%`, maxWidth: "none", maxHeight: "none" };
    return (
      <img
        className="image-preview"
        src={fileURL}
        alt={artifactLabel(artifact)}
        draggable={false}
        onDragStart={(event) => event.preventDefault()}
        style={imageStyle}
      />
    );
  }
  if ("text" in artifact && artifact.text) {
    return <pre className="text-preview">{artifact.text}</pre>;
  }
  if (mime === "application/pdf" || mime === "text/html" || mime.startsWith("text/")) {
    return <iframe className="document-frame" title={artifactLabel(artifact)} src={fileURL} />;
  }
  return (
    <div className="file-preview">
      <File size={36} />
      <span>{artifact.mime || artifact.kind || "artifact"}</span>
    </div>
  );
}

export function artifactIsImage(artifact: ArtifactSummary): boolean {
  const kind = (artifact.kind ?? "").toLowerCase();
  const mime = (artifact.mime ?? "").toLowerCase();
  return kind === "image" || mime.startsWith("image/");
}

function artifactPresentation(artifact: ArtifactSummary): { kind: string; label: string; detail: string; Icon: LucideIcon } {
  const kind = (artifact.kind ?? "").toLowerCase();
  const mime = (artifact.mime ?? "").toLowerCase();
  const extension = artifactFileExtension(artifact);
  const source = artifactSourceLabel(artifact);
  const size = formatBytes(artifact.size_bytes);
  const detail = [source, size].filter((part) => part !== "").join(" / ");
  if (artifactIsImage(artifact)) {
    return { kind: "image", label: "Image", detail, Icon: FileImage };
  }
  if (kind === "audio" || mime.startsWith("audio/")) {
    return { kind: "audio", label: "Audio", detail, Icon: FileAudio };
  }
  if (kind === "video" || mime.startsWith("video/")) {
    return { kind: "video", label: "Video", detail, Icon: Film };
  }
  if (kind === "midi" || extension === ".mid" || extension === ".midi") {
    return { kind: "midi", label: "MIDI", detail, Icon: Activity };
  }
  if (artifactLooksLikeAdapter(artifact)) {
    return { kind: "adapter", label: "Adapter", detail, Icon: Plug };
  }
  if (kind === "web_page" || textValue(artifact.url, "") !== "") {
    return { kind: "web", label: "Web page", detail, Icon: Globe };
  }
  if (kind === "document" || kind === "text" || mime.includes("pdf") || mime.startsWith("text/")) {
    return { kind: "document", label: mime.includes("pdf") ? "PDF" : "Document", detail, Icon: FileText };
  }
  if (kind === "archive" || [".zip", ".rar", ".7z", ".tar", ".gz"].includes(extension)) {
    return { kind: "archive", label: "Archive", detail, Icon: Archive };
  }
  return { kind: "file", label: kind || "File", detail, Icon: File };
}

export function artifactIcon(artifact: ArtifactSummary) {
  const Icon = artifactPresentation(artifact).Icon;
  return <Icon size={16} />;
}

export function artifactLabel(artifact: ArtifactSummary): string {
  return (
    textValue(artifact.title, "") ||
    textValue(artifact.url, "") ||
    lastPathPart(textValue(artifact.path, "")) ||
    artifact.id
  );
}

export function artifactAddress(artifact: ArtifactSummary): string {
  return textValue(artifact.path, "") || textValue(artifact.url, "") || artifactFileURL(artifact.id);
}

export function artifactMetaLine(artifact: ArtifactSummary): string {
  return [artifactPresentation(artifact).label, artifact.status ?? "ready", formatBytes(artifact.size_bytes)]
    .filter((part) => textValue(part, "") !== "")
    .join(" 路 ");
}

function artifactSourceLabel(artifact: ArtifactSummary): string {
  const source = textValue(artifact.source, "").toLowerCase();
  if (source.includes("download")) {
    return "Downloads";
  }
  if (source.includes("upload")) {
    return "Uploaded";
  }
  if (source.includes("browser")) {
    return "Browser";
  }
  if (source.includes("plugin") || source.includes("grabber")) {
    return "Plugin";
  }
  return source || textValue(artifact.status, "");
}

function artifactFileExtension(artifact: ArtifactSummary): string {
  const value = textValue(artifact.path, "") || textValue(artifact.url, "") || textValue(artifact.title, "");
  const clean = value.split(/[?#]/)[0] ?? "";
  const name = lastPathPart(clean).toLowerCase();
  const dot = name.lastIndexOf(".");
  return dot >= 0 ? name.slice(dot) : "";
}

function artifactLooksLikeAdapter(artifact: ArtifactSummary): boolean {
  const metadata = asRecord(artifact.metadata);
  const fields = [
    artifact.kind,
    artifact.source,
    artifact.path,
    artifact.title,
    artifact.mime,
    metadata.resource_origin,
    metadata.profile_path,
    metadata.skill_path
  ]
    .map((value) => textValue(value, "").toLowerCase())
    .join(" ");
  return (
    fields.includes("adapter") ||
    fields.includes("skill") ||
    fields.includes("profile") ||
    fields.includes("plugin_grabber") ||
    fields.includes("capability_pack")
  );
}

export function mergeArtifacts(...groups: ArtifactSummary[][]): ArtifactSummary[] {
  const byID = new Map<string, ArtifactSummary>();
  groups.flat().forEach((artifact) => {
    if (artifact.id) {
      byID.set(artifact.id, { ...byID.get(artifact.id), ...artifact });
    }
  });
  return Array.from(byID.values()).sort((left, right) => textValue(right.created_at, "").localeCompare(textValue(left.created_at, "")));
}


