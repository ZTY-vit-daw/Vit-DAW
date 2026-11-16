// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2d）：App.tsx MediaPane+ArtifactPreview（L 域）出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
import { AlertTriangle, Archive, Copy, Globe, Loader2, MoreHorizontal, Plus, RefreshCw, Upload, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { FormEvent, MouseEvent as ReactMouseEvent, WheelEvent as ReactWheelEvent } from "react";
import type { AgentUIState, Artifact, ArtifactSummary } from "../types";
import { readArtifact, registerResourceURL, revealArtifact, scanDownloads, watchDownloads } from "../lib/api";
import { uploadMetadata } from "../settings/config";
import { clampNumber, formatBytes } from "../lib/util";
import { EmptyState, defaultExternalBrowserURL, readonlyAuditMode } from "../App";
import { normalizeBrowserURL, postExternalBrowserMessage } from "../browser/BrowserPane";
import { artifactAddress, artifactIcon, artifactIsImage, artifactLabel, artifactMetaLine, renderArtifactPreview } from "./artifacts";

export type MediaContextMenuState = { artifact: ArtifactSummary; x: number; y: number };

export function MediaPane({
  artifacts,
  selectedArtifact,
  conversationID,
  uiState,
  onSelectArtifact,
  onAttachArtifact,
  onRenameArtifact,
  onDeleteArtifact,
  onUploadClick,
  onRefresh
}: {
  artifacts: ArtifactSummary[];
  selectedArtifact: ArtifactSummary | null;
  conversationID: string;
  uiState: AgentUIState | null;
  onSelectArtifact: (id: string) => void;
  onAttachArtifact: (artifact: ArtifactSummary) => void;
  onRenameArtifact: (id: string, title: string) => Promise<void>;
  onDeleteArtifact: (id: string) => Promise<void>;
  onUploadClick: () => void;
  onRefresh: () => Promise<void>;
}) {
  const [contextMenu, setContextMenu] = useState<MediaContextMenuState | null>(null);
  const [menuStatus, setMenuStatus] = useState("");
  const [listOpen, setListOpen] = useState(true);
  const [resourceURL, setResourceURL] = useState("");
  const [resourceBusy, setResourceBusy] = useState(false);
  const [renameTarget, setRenameTarget] = useState<ArtifactSummary | null>(null);
  const [renameDraft, setRenameDraft] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<ArtifactSummary | null>(null);
  const [artifactActionBusy, setArtifactActionBusy] = useState(false);

  useEffect(() => {
    if (!contextMenu) {
      return;
    }
    const closeMenu = () => setContextMenu(null);
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        closeMenu();
      }
    };
    window.addEventListener("click", closeMenu);
    window.addEventListener("keydown", closeOnEscape);
    return () => {
      window.removeEventListener("click", closeMenu);
      window.removeEventListener("keydown", closeOnEscape);
    };
  }, [contextMenu]);

  const metadata = useMemo(() => uploadMetadata(conversationID, uiState), [conversationID, uiState]);
  const metadataRef = useRef(metadata);
  const downloadChangeTokenRef = useRef("");

  useEffect(() => {
    metadataRef.current = metadata;
  }, [metadata]);

  const scanRecentDownloads = useCallback(async (sinceMinutes = 30, quiet = false, force = false) => {
    if (!quiet) {
      setResourceBusy(true);
    }
    if (!quiet) {
      setMenuStatus("正在扫描下载文件...");
    }
    try {
      const result = await scanDownloads({
        ...metadataRef.current,
        since_minutes: sinceMinutes,
        limit: 80,
        change_token: downloadChangeTokenRef.current,
        only_if_changed: quiet && !force && Boolean(downloadChangeTokenRef.current)
      });
      if (result.change_token) {
        downloadChangeTokenRef.current = result.change_token;
      }
      if (quiet && result.changed === false) {
        return;
      }
      const rows = result.artifacts ?? [];
      if (rows[0]) {
        onSelectArtifact(rows[0].id);
      }
      if (!quiet || rows.length > 0) {
        setMenuStatus(rows.length > 0 ? `发现 ${rows.length} 个新下载` : "没有新的下载文件");
      }
      if (!quiet || rows.length > 0) {
        await onRefresh();
      }
    } catch (scanError) {
      if (!quiet) {
        setMenuStatus(scanError instanceof Error ? scanError.message : "扫描下载失败");
      }
    } finally {
      if (!quiet) {
        setResourceBusy(false);
      }
    }
  }, [onRefresh, onSelectArtifact]);

  useEffect(() => {
    if (readonlyAuditMode) {
      return;
    }
    let cancelled = false;
    const wait = (ms: number) => new Promise<void>((resolve) => window.setTimeout(resolve, ms));
    const watchLoop = async () => {
      await scanRecentDownloads(30, true, true);
      while (!cancelled) {
        try {
          const result = await watchDownloads({
            change_token: downloadChangeTokenRef.current,
            timeout_ms: 25000,
            poll_ms: 1500
          });
          if (cancelled) {
            return;
          }
          if (result.changed) {
            await scanRecentDownloads(30, true, true);
          } else if (result.change_token) {
            downloadChangeTokenRef.current = result.change_token;
          }
        } catch {
          await wait(15000);
        }
      }
    };
    void watchLoop();
    return () => {
      cancelled = true;
    };
  }, [scanRecentDownloads]);

  const openExternalURL = async () => {
    const target = normalizeBrowserURL(resourceURL) || defaultExternalBrowserURL;
    setResourceURL(target);
    const didPost = postExternalBrowserMessage("open_url", { url: target });
    if (!didPost) {
      window.open(target, "_blank", "noopener,noreferrer");
    }
    setMenuStatus("已在系统浏览器打开");
  };

  const saveResourceURL = async () => {
    const target = normalizeBrowserURL(resourceURL);
    if (!target) {
      setMenuStatus("请先输入网址");
      return;
    }
    setResourceBusy(true);
    setMenuStatus("");
    try {
      const result = await registerResourceURL({ ...metadataRef.current, url: target, title: target });
      if (result.artifact) {
        onSelectArtifact(result.artifact.id);
      }
      setMenuStatus("链接已保存到资料库");
      await onRefresh();
    } catch (saveError) {
      setMenuStatus(saveError instanceof Error ? saveError.message : "保存链接失败");
    } finally {
      setResourceBusy(false);
    }
  };

  const openContextMenu = (artifact: ArtifactSummary, event: ReactMouseEvent<HTMLElement>) => {
    event.preventDefault();
    event.stopPropagation();
    const menuWidth = 220;
    const menuHeight = 250;
    const x = Math.max(12, Math.min(event.clientX, window.innerWidth - menuWidth));
    const y = Math.max(12, Math.min(event.clientY, window.innerHeight - menuHeight));
    setContextMenu({ artifact, x, y });
  };

  const copyArtifactValue = async (label: string, value: string) => {
    const text = value.trim();
    if (!text) {
      setMenuStatus(`${label}暂无内容`);
      setContextMenu(null);
      return;
    }
    try {
      await navigator.clipboard.writeText(text);
      setMenuStatus(`已复制${label}`);
    } catch {
      setMenuStatus("复制失败，请手动选择文本");
    }
    setContextMenu(null);
  };

  const openRenameDialog = (artifact: ArtifactSummary) => {
    setRenameTarget(artifact);
    setRenameDraft(artifactLabel(artifact));
    setContextMenu(null);
  };

  const submitRename = async (event: FormEvent) => {
    event.preventDefault();
    if (!renameTarget) {
      return;
    }
    const title = renameDraft.trim();
    if (!title) {
      setMenuStatus("名称不能为空");
      return;
    }
    setArtifactActionBusy(true);
    try {
      await onRenameArtifact(renameTarget.id, title);
      setMenuStatus("已重命名");
      setRenameTarget(null);
    } catch (renameError) {
      setMenuStatus(renameError instanceof Error ? renameError.message : "重命名失败");
    } finally {
      setArtifactActionBusy(false);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) {
      return;
    }
    setArtifactActionBusy(true);
    try {
      await onDeleteArtifact(deleteTarget.id);
      setMenuStatus("已从资料库移除");
      setDeleteTarget(null);
    } catch (deleteError) {
      setMenuStatus(deleteError instanceof Error ? deleteError.message : "删除失败");
    } finally {
      setArtifactActionBusy(false);
    }
  };

  return (
    <div className="pane-body media-pane">
      <div className="media-toolbar">
        <button className="media-pool-title-button" type="button" onClick={() => setListOpen((open) => !open)}>
          <Archive size={18} />
          <span>
            <strong>资料库</strong>
            <small>{artifacts.length} 个文件</small>
          </span>
        </button>
        <div className="media-toolbar-actions">
          <button className="tool-button" type="button" onClick={onUploadClick}>
            <Upload size={16} />
            <span>上传</span>
          </button>
        </div>
      </div>
      <form
        className="media-resource-bar"
        onSubmit={(event) => {
          event.preventDefault();
          void openExternalURL();
        }}
      >
        <label className="media-url-field">
          <Globe size={15} />
          <input
            value={resourceURL}
            spellCheck={false}
            onChange={(event) => setResourceURL(event.currentTarget.value)}
            placeholder="输入网址；留空打开浏览器"
          />
        </label>
        <button className="tool-button primary media-open-button" type="submit" disabled={resourceBusy} title="打开系统浏览器">
          <Globe size={16} />
          <span>打开</span>
        </button>
        <button
          className="tool-button media-icon-button"
          type="button"
          title="保存链接到资料库"
          aria-label="保存链接到资料库"
          onClick={saveResourceURL}
          disabled={resourceBusy}
        >
          <Plus size={16} />
        </button>
        <button
          className="tool-button media-icon-button"
          type="button"
          title="扫描本次会话下载"
          aria-label="扫描本次会话下载"
          onClick={() => scanRecentDownloads(30, false)}
          disabled={resourceBusy}
        >
          {resourceBusy ? <Loader2 className="spin" size={16} /> : <RefreshCw size={16} />}
        </button>
      </form>
      <div className={`media-status-line ${menuStatus ? "" : "empty"}`}>{menuStatus}</div>
      <div className={`media-grid ${listOpen ? "list-open" : "list-closed"}`}>
        <section className="artifact-list" aria-label="资料库文件列表">
          <div className="artifact-list-header">
            <span>文件</span>
            <small>{formatBytes(artifacts.reduce((total, artifact) => total + Number(artifact.size_bytes ?? 0), 0))}</small>
            <button className="artifact-list-close" type="button" title="隐藏文件列表" onClick={() => setListOpen(false)}>
              <X size={14} />
            </button>
          </div>
          <div className="artifact-list-scroll">
            {artifacts.length === 0 && <EmptyState label="暂无资料" />}
            {artifacts.map((artifact) => (
              <div
                key={artifact.id}
                role="button"
                tabIndex={0}
                className={`artifact-list-item ${selectedArtifact?.id === artifact.id ? "active" : ""}`}
                onClick={() => onSelectArtifact(artifact.id)}
                onContextMenu={(event) => openContextMenu(artifact, event)}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault();
                    onSelectArtifact(artifact.id);
                  }
                }}
              >
                <div className="artifact-type-icon">{artifactIcon(artifact)}</div>
                <div className="artifact-row-main">
                  <strong>{artifactLabel(artifact)}</strong>
                  <span>{artifact.kind ?? "artifact"} 路 {formatBytes(artifact.size_bytes)}</span>
                </div>
                <button
                  className="artifact-more-button"
                  type="button"
                  title="更多操作"
                  onClick={(event) => openContextMenu(artifact, event)}
                >
                  <MoreHorizontal size={15} />
                </button>
              </div>
            ))}
          </div>
        </section>
        <ArtifactPreview selectedArtifact={selectedArtifact} onAttachArtifact={onAttachArtifact} />
      </div>
      {contextMenu && (
        <div
          className="media-context-menu"
          style={{ left: contextMenu.x, top: contextMenu.y }}
          role="menu"
          onClick={(event) => event.stopPropagation()}
        >
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              onSelectArtifact(contextMenu.artifact.id);
              setContextMenu(null);
            }}
          >
            预览
          </button>
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              onAttachArtifact(contextMenu.artifact);
              setContextMenu(null);
            }}
          >
            附加到对话          </button>
          <div className="media-menu-separator" />
          <button type="button" role="menuitem" onClick={() => copyArtifactValue("地址", artifactAddress(contextMenu.artifact))}>
            复制地址
          </button>
          <button type="button" role="menuitem" onClick={() => copyArtifactValue("文件名", artifactLabel(contextMenu.artifact))}>
            复制文件名          </button>
          <button type="button" role="menuitem" onClick={() => copyArtifactValue("ID", contextMenu.artifact.id)}>
            复制 ID
          </button>
          <div className="media-menu-separator" />
          <button type="button" role="menuitem" onClick={() => openRenameDialog(contextMenu.artifact)}>
            重命名          </button>
          <button
            className="danger"
            type="button"
            role="menuitem"
            onClick={() => {
              setDeleteTarget(contextMenu.artifact);
              setContextMenu(null);
            }}
          >
            从列表删除          </button>
        </div>
      )}
      {renameTarget && (
        <div className="media-dialog-backdrop" onClick={() => setRenameTarget(null)}>
          <form className="media-dialog" onSubmit={submitRename} onClick={(event) => event.stopPropagation()}>
            <h3>重命名</h3>
            <input value={renameDraft} onChange={(event) => setRenameDraft(event.currentTarget.value)} autoFocus />
            <div className="media-dialog-actions">
              <button type="button" onClick={() => setRenameTarget(null)} disabled={artifactActionBusy}>
                取消
              </button>
              <button type="submit" className="primary" disabled={artifactActionBusy}>
                保存
              </button>
            </div>
          </form>
        </div>
      )}
      {deleteTarget && (
        <div className="media-dialog-backdrop" onClick={() => setDeleteTarget(null)}>
          <div className="media-dialog" onClick={(event) => event.stopPropagation()}>
            <h3>从列表删除</h3>
            <p>{artifactLabel(deleteTarget)}</p>
            <span>只会从资料库移除此记录，不会删除硬盘上的原文件。</span>
            <div className="media-dialog-actions">
              <button type="button" onClick={() => setDeleteTarget(null)} disabled={artifactActionBusy}>
                取消
              </button>
              <button type="button" className="danger" onClick={confirmDelete} disabled={artifactActionBusy}>
                删除
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export function ArtifactPreview({
  selectedArtifact,
  onAttachArtifact
}: {
  selectedArtifact: ArtifactSummary | null;
  onAttachArtifact: (artifact: ArtifactSummary) => void;
}) {
  const [artifact, setArtifact] = useState<Artifact | null>(null);
  const [loading, setLoading] = useState(false);
  const [previewError, setPreviewError] = useState("");
  const [copyStatus, setCopyStatus] = useState("");
  const [imageZoom, setImageZoom] = useState(1);

  useEffect(() => {
    if (!selectedArtifact?.id) {
      setArtifact(null);
      return;
    }
    setLoading(true);
    setPreviewError("");
    void readArtifact(selectedArtifact.id)
      .then((response) => setArtifact(response.artifact ?? null))
      .catch((readError) => setPreviewError(readError instanceof Error ? readError.message : "读取失败"))
      .finally(() => setLoading(false));
  }, [selectedArtifact?.id]);

  useEffect(() => {
    setImageZoom(1);
  }, [selectedArtifact?.id]);

  if (!selectedArtifact) {
    return <EmptyState label="尚未选择资料" />;
  }
  const current = artifact ?? selectedArtifact;
  const isImagePreview = artifactIsImage(current);
  const preview = renderArtifactPreview(current, { imageZoom });
  const location = artifactAddress(current);
  const handlePreviewWheel = (event: ReactWheelEvent<HTMLDivElement>) => {
    if (!isImagePreview) {
      return;
    }
    event.preventDefault();
    const direction = event.deltaY > 0 ? -1 : 1;
    setImageZoom((value) => clampNumber(Math.round((value + direction * 0.12) * 100) / 100, 0.35, 4));
  };
  const copyLocation = async () => {
    try {
      await navigator.clipboard.writeText(location);
      setCopyStatus("已复制地址");
    } catch {
      setCopyStatus("复制失败");
    }
  };
  const revealLocation = async () => {
    try {
      await revealArtifact(current.id);
      setCopyStatus("已打开文件位置");
    } catch (revealError) {
      await copyLocation();
      setCopyStatus(revealError instanceof Error ? `无法打开，已复制地址：${revealError.message}` : "无法打开，已复制地址");
    }
  };
  return (
    <section className="artifact-preview">
      <div className="preview-header">
        <div>
          <h3>{artifactLabel(current)}</h3>
          <span>{artifactMetaLine(current)}</span>
        </div>
        <div className="preview-actions">
          <button className="icon-button" type="button" title="复制地址" onClick={copyLocation}>
            <Copy size={16} />
          </button>
          <button className="icon-button" type="button" title="附加到对话" onClick={() => onAttachArtifact(selectedArtifact)}>
            <Plus size={16} />
          </button>
        </div>
      </div>
      <button className="artifact-location" type="button" title="打开本地文件位置" onClick={revealLocation}>
        {location}
      </button>
      {copyStatus && <div className="preview-copy-status">{copyStatus}</div>}
      {loading && <div className="loading-line"><Loader2 className="spin" size={16} />Loading</div>}
      {previewError && <div className="notice warning"><AlertTriangle size={16} />{previewError}</div>}
      <div
        className={`artifact-preview-body ${isImagePreview ? "image-mode" : ""}`}
        onWheel={handlePreviewWheel}
        onDragStart={(event) => event.preventDefault()}
        onDoubleClick={() => {
          if (isImagePreview) {
            setImageZoom(1);
          }
        }}
      >
        {preview}
      </div>
      {isImagePreview && (
        <div className="image-zoom-status">
          <span>{Math.round(imageZoom * 100)}%</span>
          <button type="button" onClick={() => setImageZoom(1)}>复位</button>
        </div>
      )}
      {current.summary && <p className="artifact-summary">{current.summary}</p>}
    </section>
  );
}


