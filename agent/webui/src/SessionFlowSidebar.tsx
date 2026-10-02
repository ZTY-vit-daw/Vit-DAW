import { Archive, ArchiveRestore, PanelLeftClose, PanelLeftOpen, Pencil, Plus, Settings } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import type { HistoryScopeParts } from "./historyScope";
import {
  loadSessionFlowCollapsed,
  loadSessionFlowRegistry,
  renameSessionFlowEntry,
  saveSessionFlowCollapsed,
  saveSessionFlowRegistry,
  sessionFlowRows,
  sessionFlowServerHints,
  setSessionFlowArchived,
  upsertSessionFlowEntry,
  type SessionFlowEntry
} from "./sessionFlow";

// WEBUI-IA-REDESIGN-1：左列会话流侧边栏（替换原 Ask Vit 模式轨）。
// 用户裁定：轻量会话操作（新建/切换/重命名/归档/折叠），会话键=conversation id，
// 不建工作树不建分支（工程级深操作在历史界面）；note 流不进侧边栏
//（NOTESTREAM-2 契约：note 自含面板）。展示编号=纯线性数字（1/2/3…，无前缀
// 无哈希，STATUSBAR-ID-1 同裁定），完整会话 id 不上 UI，调试走 DOM/网络面板
//（行节点保留 data-conversation-id 属性=零成本 DOM 查询面）。

export function SessionFlowSidebar({
  scopeKey,
  scopeParts,
  currentConversationID,
  continuations,
  settingsOpen,
  onNewConversation,
  onSwitchConversation,
  onOpenSettings,
  readEntryTitle
}: {
  scopeKey: string;
  scopeParts: HistoryScopeParts;
  currentConversationID: string;
  continuations: unknown;
  settingsOpen: boolean;
  onNewConversation: () => void;
  onSwitchConversation: (conversationID: string) => void;
  onOpenSettings: () => void;
  /** 从消息缓存推导未命名会话的展示名（App 注入，避免 App↔sidebar 循环依赖） */
  readEntryTitle: (conversationID: string) => string;
}) {
  const [entries, setEntries] = useState<SessionFlowEntry[]>([]);
  const [collapsed, setCollapsed] = useState(() => loadSessionFlowCollapsed());
  const [renamingID, setRenamingID] = useState("");
  const [renameDraft, setRenameDraft] = useState("");
  const [archivedOpen, setArchivedOpen] = useState(false);
  const renameInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    setEntries(loadSessionFlowRegistry(scopeKey));
    setRenamingID("");
  }, [scopeKey]);

  useEffect(() => {
    if (!scopeKey) {
      return;
    }
    saveSessionFlowRegistry(scopeKey, entries);
  }, [entries, scopeKey]);

  // 当前会话幂等登记（不建工作树：仅注册表行，重命名/归档态保留）
  useEffect(() => {
    if (!scopeKey || !currentConversationID) {
      return;
    }
    setEntries((current) => upsertSessionFlowEntry(current, { conversationID: currentConversationID }));
  }, [scopeKey, currentConversationID]);

  const serverHints = useMemo(
    () =>
      sessionFlowServerHints(continuations, {
        projectPath: scopeParts.projectPath,
        rootProjectPath: scopeParts.rootProjectPath,
        projectUUID: scopeParts.projectUUID
      }),
    [continuations, scopeParts.projectPath, scopeParts.rootProjectPath, scopeParts.projectUUID]
  );
  const { visible, archived } = useMemo(() => sessionFlowRows(entries, serverHints), [entries, serverHints]);

  const toggleCollapsed = () => {
    setCollapsed((value) => {
      saveSessionFlowCollapsed(!value);
      return !value;
    });
  };

  const startRename = (conversationID: string, currentTitle: string) => {
    setRenamingID(conversationID);
    setRenameDraft(currentTitle);
  };

  const commitRename = () => {
    if (renamingID) {
      setEntries((current) => renameSessionFlowEntry(current, renamingID, renameDraft));
    }
    setRenamingID("");
  };

  useEffect(() => {
    if (renamingID) {
      renameInputRef.current?.focus();
      renameInputRef.current?.select();
    }
  }, [renamingID]);

  const archiveEntry = (conversationID: string, nextArchived: boolean) => {
    setEntries((current) => setSessionFlowArchived(current, conversationID, nextArchived));
  };

  const displayName = (row: { title: string; conversationID: string }) =>
    row.title || readEntryTitle(row.conversationID) || "未命名会话";

  if (collapsed) {
    return (
      <aside className="session-sidebar collapsed" aria-label="会话流">
        <div className="session-sidebar-strip">
          <button type="button" className="session-strip-button" title="展开会话流" data-action="expand" onClick={toggleCollapsed}>
            <PanelLeftOpen size={17} />
          </button>
          <button type="button" className="session-strip-button" title="新建会话" data-action="new" onClick={onNewConversation}>
            <Plus size={17} />
          </button>
          <span className="session-strip-count" title={`当前工程 ${visible.length} 个会话`}>
            {visible.length}
          </span>
        </div>
      </aside>
    );
  }

  return (
    <aside className="session-sidebar" aria-label="会话流">
      <header className="session-sidebar-head">
        <strong>会话流</strong>
        <button
          type="button"
          className="session-icon-button"
          title="折叠侧边栏"
          data-action="collapse"
          onClick={toggleCollapsed}
        >
          <PanelLeftClose size={16} />
        </button>
      </header>

      <button type="button" className="session-new-button" data-action="new" onClick={onNewConversation}>
        <Plus size={15} />
        <span>新建会话</span>
      </button>

      <div className="session-list" role="list">
        {visible.length === 0 && <p className="session-empty">暂无会话</p>}
        {visible.map((row) =>
          renamingID === row.conversationID ? (
            <input
              key={row.conversationID}
              ref={renameInputRef}
              className="session-rename-input"
              aria-label="会话标题"
              value={renameDraft}
              onChange={(event) => setRenameDraft(event.currentTarget.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  commitRename();
                } else if (event.key === "Escape") {
                  setRenamingID("");
                }
              }}
              onBlur={commitRename}
            />
          ) : (
            <div
              key={row.conversationID}
              role="listitem"
              className={`session-row${row.conversationID === currentConversationID ? " active" : ""}`}
              data-conversation-id={row.conversationID}
            >
              <button
                type="button"
                className="session-row-main"
                title={displayName(row)}
                onClick={() => onSwitchConversation(row.conversationID)}
              >
                <span className="session-number">{row.displayNumber}</span>
                <span className="session-title">{displayName(row)}</span>
              </button>
              <span className="session-row-actions">
                <button
                  type="button"
                  className="session-icon-button"
                  title="重命名"
                  data-action="rename"
                  onClick={() => startRename(row.conversationID, displayName(row))}
                >
                  <Pencil size={13} />
                </button>
                <button
                  type="button"
                  className="session-icon-button"
                  title="归档"
                  data-action="archive"
                  onClick={() => archiveEntry(row.conversationID, true)}
                >
                  <Archive size={13} />
                </button>
              </span>
            </div>
          )
        )}
      </div>

      {archived.length > 0 && (
        <div className="session-archived">
          <button
            type="button"
            className="session-archived-toggle"
            aria-expanded={archivedOpen}
            onClick={() => setArchivedOpen((value) => !value)}
          >
            已归档 {archived.length}
          </button>
          {archivedOpen &&
            archived.map((row) => (
              <div
                key={row.conversationID}
                className="session-row archived"
                data-conversation-id={row.conversationID}
                data-archived="1"
              >
                <button
                  type="button"
                  className="session-row-main"
                  title={displayName(row)}
                  onClick={() => onSwitchConversation(row.conversationID)}
                >
                  <span className="session-number">{row.displayNumber}</span>
                  <span className="session-title">{displayName(row)}</span>
                </button>
                <span className="session-row-actions">
                  <button
                    type="button"
                    className="session-icon-button"
                    title="取消归档"
                    data-action="restore"
                    onClick={() => archiveEntry(row.conversationID, false)}
                  >
                    <ArchiveRestore size={13} />
                  </button>
                </span>
              </div>
            ))}
        </div>
      )}

      <footer className="session-sidebar-foot">
        <button
          type="button"
          className={`session-foot-button${settingsOpen ? " active" : ""}`}
          title="设置"
          onClick={onOpenSettings}
        >
          <Settings size={16} />
          <span>设置</span>
        </button>
      </footer>
    </aside>
  );
}
