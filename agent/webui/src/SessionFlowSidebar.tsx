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
  noteSessionServerHints,
  setSessionFlowArchived,
  upsertSessionFlowEntry,
  type SessionFlowEntry
} from "./sessionFlow";

// WEBUI-IA-REDESIGN-1：左列会话流侧边栏（替换原 Ask Vit 模式轨）。
// 用户裁定：轻量会话操作（新建/切换/重命名/归档/折叠），会话键=conversation id，
// 不建工作树不建分支（工程级深操作在历史界面）。展示编号=纯线性数字（1/2/3…，
// 无前缀无哈希，STATUSBAR-ID-1 同裁定），完整会话 id 不上 UI，调试走 DOM/网络面板
//（行节点保留 data-conversation-id 属性=零成本 DOM 查询面）。
// VITNOTE-NOTESTREAM-2（2026-10-03 命名裁定取代 IA 期"note 流不入侧边栏"意见）：
// note 会话经 note_sessions 服务端提示入侧边栏——服务端默认名（「便签 N3 · 时间线
// 55% · 机架 30%」式）随行，本地可改名（注册表权威）；note 删除=行转归档组。

export function SessionFlowSidebar({
  scopeKey,
  scopeParts,
  currentConversationID,
  continuations,
  noteSessions,
  mainConversationID,
  mainFlowTitle,
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
  /** VITNOTE-NOTESTREAM-2：/agent/runtime/status note_sessions 投影（note 会话提示源） */
  noteSessions: unknown;
  /** WEBUI-SESSION-SEMANTICS-1：本 scope 主对话流会话 id（App 在 scope 物化时钉定） */
  mainConversationID: string;
  /** 主对话流默认名（裁定 1：「主对话流 · <工程名>」；确定性模板，零 LLM 依赖） */
  mainFlowTitle: string;
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

  // WEBUI-SESSION-SEMANTICS-1：主对话流出生即命名——主会话行随 App 钉定的
  // mainConversationID 幂等登记默认名「主对话流 · <工程名>」。upsert 只在行未命名时
  // 落默认名：用户改名最高且持久（裁定 1），归档态同样零覆写；用户后续新建的会话
  // 不经此路径（无主会话身份），平级列出、沿消息推导或"未命名会话"展示。
  useEffect(() => {
    if (!scopeKey || !mainConversationID || !mainFlowTitle) {
      return;
    }
    setEntries((current) => upsertSessionFlowEntry(current, { conversationID: mainConversationID, title: mainFlowTitle }));
  }, [scopeKey, mainConversationID, mainFlowTitle]);

  const serverHints = useMemo(
    () =>
      // VITNOTE-NOTESTREAM-2：note 会话提示叠加在 continuations 提示上（键空间不相交：
      // note 会话 id 恒 note_ 前缀；note 行带默认名+归档态，见 sessionFlow.ts 合并注释）。
      new Map([
        ...sessionFlowServerHints(continuations, {
          projectPath: scopeParts.projectPath,
          rootProjectPath: scopeParts.rootProjectPath,
          projectUUID: scopeParts.projectUUID
        }),
        ...noteSessionServerHints(noteSessions, {
          projectPath: scopeParts.projectPath,
          rootProjectPath: scopeParts.rootProjectPath,
          projectUUID: scopeParts.projectUUID
        })
      ]),
    [continuations, noteSessions, scopeParts.projectPath, scopeParts.rootProjectPath, scopeParts.projectUUID]
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
