// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2f）：App.tsx HistoryPane（O 组件域）出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
import { GitBranch, Plus, RefreshCw, X } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import type { MouseEvent as ReactMouseEvent } from "react";
import type { AgentUIState, JsonRecord } from "../types";
import { asRecord, lastPathPart, textValue } from "../lib/util";
import { invokeAgent } from "../lib/api";
import {
  historyActiveBranch,
  historyBranchRows,
  historyNodeCanCreateFrom,
  historyNodeTitle,
  historyProjectPath,
  historySafeName,
  historyShortID,
  historyStatusIsError,
  historyToolArgs,
  historyTreeLayout,
  historyTreeNodes,
  historyWorktreeCheckoutArgs,
  historyWorktreeRowFromRecord,
  historyWorktreeRows,
  buildHistoryInvokeContext,
  type HistoryBranchRow,
  type HistoryLoadedWorktree,
  type HistoryNodeMenuState,
  type HistoryTreeNode,
  type HistoryWorktreeRow
} from "./tree";
import { EmptyState } from "../topbar/diagnostics";

export function HistoryPane({ uiState, onRefresh, checkoutBlocked }: { uiState: AgentUIState | null; onRefresh: () => Promise<void>; checkoutBlocked: boolean }) {
  const activeHistory = asRecord(uiState?.project_history);
  const worktrees = useMemo(() => historyWorktreeRows(activeHistory), [activeHistory]);
  const activeWorktreeKey = worktrees.find((worktree) => worktree.active)?.key ?? worktrees[0]?.key ?? "";
  const [selectedWorktreeKey, setSelectedWorktreeKey] = useState("");
  const [loadedWorktree, setLoadedWorktree] = useState<HistoryLoadedWorktree | null>(null);
  const selectedWorktree = worktrees.find((worktree) => worktree.key === selectedWorktreeKey) ?? worktrees.find((worktree) => worktree.key === activeWorktreeKey) ?? worktrees[0] ?? null;
  const selectedWorktreeIsActive = selectedWorktree?.active ?? true;
  const displayHistory = selectedWorktreeIsActive
    ? activeHistory
    : loadedWorktree?.key === selectedWorktree?.key
      ? loadedWorktree.history
      : {};
  const branches = useMemo(() => historyBranchRows(displayHistory), [displayHistory]);
  const nodes = useMemo(() => historyTreeNodes(displayHistory), [displayHistory]);
  const treeLayout = useMemo(() => historyTreeLayout(nodes), [nodes]);
  const [selectedBranchName, setSelectedBranchName] = useState("");
  const [selectedNodeID, setSelectedNodeID] = useState("");
  const [busyAction, setBusyAction] = useState("");
  const [status, setStatus] = useState("");
  const [nodeMenu, setNodeMenu] = useState<HistoryNodeMenuState | null>(null);
  const activeNodeID = textValue(displayHistory.active_node_id ?? asRecord(displayHistory.conversation_graph).active_node_id, "");
  const selectedNode =
    nodes.find((node) => node.id === selectedNodeID) ??
    nodes.find((node) => node.id === activeNodeID) ??
    nodes[nodes.length - 1] ??
    null;
  const activeBranch = historyActiveBranch(displayHistory);
  const projectPath = historyProjectPath(displayHistory) || selectedWorktree?.projectFilePath || historyProjectPath(activeHistory);
  const loadingSelectedWorktree = Boolean(selectedWorktree && !selectedWorktreeIsActive && busyAction === `inspect:${selectedWorktree.key}`);

  useEffect(() => {
    if (!selectedWorktreeKey || !worktrees.some((worktree) => worktree.key === selectedWorktreeKey)) {
      setSelectedWorktreeKey(activeWorktreeKey);
      setLoadedWorktree(null);
    }
  }, [activeWorktreeKey, selectedWorktreeKey, worktrees]);

  useEffect(() => {
    setSelectedBranchName(historyActiveBranch(displayHistory));
  }, [displayHistory]);

  useEffect(() => {
    if (nodes.length === 0) {
      setSelectedNodeID("");
      return;
    }
    if (selectedNodeID && nodes.some((node) => node.id === selectedNodeID)) {
      return;
    }
    setSelectedNodeID(activeNodeID || nodes[nodes.length - 1].id);
  }, [activeNodeID, nodes, selectedNodeID]);

  useEffect(() => {
    if (!nodeMenu) {
      return;
    }
    const closeMenu = () => setNodeMenu(null);
    window.addEventListener("click", closeMenu);
    window.addEventListener("blur", closeMenu);
    return () => {
      window.removeEventListener("click", closeMenu);
      window.removeEventListener("blur", closeMenu);
    };
  }, [nodeMenu]);

  const inspectWorktree = async (worktree: HistoryWorktreeRow) => {
    if (!worktree.projectFilePath) {
      setLoadedWorktree(null);
      setStatus("这个工作树还没有可读取的工程文件。");
      return;
    }
    setBusyAction(`inspect:${worktree.key}`);
    setStatus("");
    try {
      const response = await invokeAgent({
        tool: "version.status",
        args: historyToolArgs(activeHistory, { project_path: worktree.projectFilePath }),
        confirmed: false,
        source: "ask_vit_webui_history",
        context: buildHistoryInvokeContext(uiState, activeHistory)
      });
      if (response.error || response.status === "error") {
        throw new Error(response.error || "读取工作树历史失败。");
      }
      const result = asRecord(response.result ?? response.project_history);
      setLoadedWorktree({ key: worktree.key, history: result });
      setStatus(`正在查看工作树：${worktree.label}`);
    } catch (historyError) {
      setLoadedWorktree(null);
      setStatus(historyError instanceof Error ? historyError.message : "读取工作树历史失败。");
    } finally {
      setBusyAction("");
    }
  };

  const selectWorktree = async (worktree: HistoryWorktreeRow) => {
    setSelectedWorktreeKey(worktree.key);
    setSelectedNodeID("");
    setNodeMenu(null);
    if (worktree.active) {
      setLoadedWorktree(null);
      setStatus("");
      return;
    }
    await inspectWorktree(worktree);
  };

  const runHistoryTool = async (
    actionKey: string,
    tool: string,
    args: JsonRecord,
    successMessage: string,
    options: { checkoutCreatedWorktree?: boolean } = {}
  ) => {
    if (busyAction) {
      return null;
    }
    if (checkoutBlocked && ["version.checkout", "version.node_checkout", "version.worktree_checkout", "version.branch_create"].includes(tool)) {
      setStatus("Agent Turn 正在运行；请先 Stop Turn 再切换工程历史。");
      return null;
    }
    setBusyAction(actionKey);
    setStatus("");
    try {
      const response = await invokeAgent({
        tool,
        args: historyToolArgs(displayHistory, args),
        confirmed: true,
        source: "ask_vit_webui_history",
        context: buildHistoryInvokeContext(uiState, displayHistory)
      });
      if (response.error || response.status === "error") {
        throw new Error(response.error || "Project History action failed");
      }
      if (options.checkoutCreatedWorktree) {
        const createdWorktree = asRecord(response.result);
        const checkoutArgs = historyWorktreeCheckoutArgs(historyWorktreeRowFromRecord(createdWorktree, displayHistory));
        if (Object.keys(checkoutArgs).length > 0) {
          const checkout = await invokeAgent({
            tool: "version.worktree_checkout",
            args: historyToolArgs(displayHistory, checkoutArgs),
            confirmed: true,
            source: "ask_vit_webui_history",
            context: buildHistoryInvokeContext(uiState, displayHistory)
          });
          if (checkout.error || checkout.status === "error") {
            throw new Error(checkout.error || "Worktree created, but checkout failed");
          }
        }
      }
      setStatus(successMessage);
      await onRefresh();
      if (selectedWorktree && !selectedWorktree.active && !options.checkoutCreatedWorktree) {
        await inspectWorktree(selectedWorktree);
      }
      return response;
    } catch (historyError) {
      setStatus(historyError instanceof Error ? historyError.message : "Project History action failed");
      return null;
    } finally {
      setBusyAction("");
    }
  };

  const checkoutBranch = async (branch: HistoryBranchRow) => {
    setSelectedBranchName(branch.name);
    if (!selectedWorktreeIsActive || !branch.name || branch.active) {
      return;
    }
    if (!window.confirm(`切换到分支 ${branch.name}？当前工程会恢复到该分支 HEAD。`)) {
      return;
    }
    await runHistoryTool(`branch:${branch.name}`, "version.checkout", { branch: branch.name }, `已切换到分支 ${branch.name}`);
  };

  const createBranchFromNode = async (node: HistoryTreeNode) => {
    if (!historyNodeCanCreateFrom(node)) {
      setStatus("这个节点还没有可用的回溯哈希。");
      return;
    }
    const name = window.prompt("从此节点新建分支", `branch-${historyShortID(node.commitID)}`);
    const cleanName = historySafeName(name ?? "");
    if (!cleanName) {
      return;
    }
    await runHistoryTool(
      `branch-create:${node.id}`,
      "version.branch_create",
      { name: cleanName, commit_id: node.commitID, from_node_id: node.nodeID },
      `已从节点创建分支 ${cleanName}`
    );
  };

  const createWorktreeFromNode = async (node: HistoryTreeNode) => {
    if (!historyNodeCanCreateFrom(node)) {
      setStatus("这个节点还没有可用的回溯哈希。");
      return;
    }
    const name = window.prompt("从此节点新建工作树", `worktree-${historyShortID(node.commitID)}`);
    const cleanName = historySafeName(name ?? "");
    if (!cleanName) {
      return;
    }
    await runHistoryTool(
      `worktree-create:${node.id}`,
      "version.worktree_create",
      { name: cleanName, commit_id: node.commitID },
      `已从节点创建并打开工作树 ${cleanName}`,
      { checkoutCreatedWorktree: true }
    );
  };

  const deleteHistoryNode = async (node: HistoryTreeNode) => {
    if (!node.nodeID) {
      setStatus("这个节点没有可删除的节点 ID。");
      return;
    }
    if (!window.confirm(`删除节点 ${historyNodeTitle(node)} 及其子节点？这会修改工程历史树。`)) {
      return;
    }
    await runHistoryTool(`delete:${node.id}`, "version.node_delete", { node_id: node.nodeID }, `已删除节点 ${historyShortID(node.nodeID)}`);
  };

  const openNodeMenu = (event: ReactMouseEvent, node: HistoryTreeNode) => {
    event.preventDefault();
    event.stopPropagation();
    setSelectedNodeID(node.id);
    setNodeMenu({ node, x: event.clientX, y: event.clientY });
  };

  return (
    <div className="pane-body history-pane" onContextMenu={(event) => event.preventDefault()}>
      <section className="history-rail-panel">
        <div className="history-title-line">
          <div>
            <h2>工程历史</h2>
            <span>{lastPathPart(projectPath || textValue(activeHistory.project_label, "未保存工程"))}</span>
          </div>
          <button className="history-icon-button" type="button" title="刷新历史" onClick={() => void onRefresh()} disabled={Boolean(busyAction)}>
            <RefreshCw size={15} />
          </button>
        </div>

        <div className="history-rail-group">
          <div className="history-section-heading">
          <h2>工作树</h2>
          <span>{worktrees.length}</span>
        </div>
          <div className="history-list">
          {worktrees.length === 0 && <EmptyState label="暂无工作树" />}
          {worktrees.map((worktree) => (
            <button
                className={`history-list-row ${worktree.active ? "active" : ""} ${selectedWorktree?.key === worktree.key ? "selected" : ""}`}
              key={worktree.key}
              type="button"
                onClick={() => void selectWorktree(worktree)}
                disabled={Boolean(busyAction) && busyAction !== `inspect:${worktree.key}`}
              title={worktree.projectFilePath || worktree.path || worktree.label}
            >
                <span className="history-row-dot" />
              <span>
                <strong>{worktree.label}</strong>
                  <small>{historyShortID(worktree.commitID) || lastPathPart(worktree.projectFilePath || worktree.path) || "-"}</small>
              </span>
                <em>{worktree.active ? "当前" : selectedWorktree?.key === worktree.key ? "已选中" : "可查看"}</em>
            </button>
          ))}
          </div>
        </div>

        <div className="history-rail-group">
          <div className="history-section-heading">
          <h2>分支</h2>
          <span>{branches.length}</span>
        </div>
          <div className="history-list">
            {loadingSelectedWorktree && <EmptyState label="正在读取工作树历史..." />}
          {branches.length === 0 && <EmptyState label="暂无分支" />}
          {branches.map((branch) => (
            <button
                className={`history-list-row ${branch.active ? "active" : ""} ${selectedBranchName === branch.name ? "selected" : ""}`}
              key={branch.name}
              type="button"
              onClick={() => void checkoutBranch(branch)}
                disabled={Boolean(busyAction) || checkoutBlocked || !branch.head}
              title={branch.head}
            >
                <span className="history-row-dot" />
              <span>
                <strong>{branch.name}</strong>
                <small>{historyShortID(branch.head) || "-"}</small>
              </span>
                <em>{branch.active ? "当前" : selectedWorktreeIsActive ? "可切换" : "可查看"}</em>
            </button>
          ))}
          </div>
        </div>

        <div className="history-rail-group history-tree-group">
          <div className="history-section-heading">
          <h2>Ask Vit 历史树</h2>
          <span>{nodes.length}</span>
        </div>
          <div className="history-tree-scroll">
            {nodes.length === 0 && <EmptyState label={loadingSelectedWorktree ? "正在读取工作树历史..." : "暂无历史节点"} />}
            {nodes.length > 0 && (
              <div className="history-tree-canvas" style={{ width: treeLayout.width, height: treeLayout.height }}>
                <svg className="history-tree-lines" width={treeLayout.width} height={treeLayout.height} aria-hidden="true">
                  {treeLayout.connections.map((connection) => (
                    <polyline key={connection.id} points={connection.points} />
                  ))}
                </svg>
                {treeLayout.nodes.map((node) => (
                  <button
                    className={`history-node-card ${node.kind} ${node.active ? "active" : ""} ${selectedNode?.id === node.id ? "selected" : ""}`}
                    key={node.id}
                    type="button"
                    style={{ left: node.x, top: node.y }}
                    onClick={() => setSelectedNodeID(node.id)}
                    onContextMenu={(event) => openNodeMenu(event, node)}
                    title={node.summary}
                  >
                    <strong>{historyNodeTitle(node)}</strong>
                    <span>{historyShortID(node.commitID) || node.meta || "-"}</span>
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
        {status && <div className={`history-status ${historyStatusIsError(status) ? "error" : ""}`}>{status}</div>}
      </section>

      {nodeMenu && (
        <div className="history-context-menu" style={{ left: nodeMenu.x, top: nodeMenu.y }} onClick={(event) => event.stopPropagation()}>
          <button type="button" onClick={() => void createBranchFromNode(nodeMenu.node)} disabled={!historyNodeCanCreateFrom(nodeMenu.node) || checkoutBlocked || Boolean(busyAction)}>
            <GitBranch size={14} />
            <span>从此节点新建分支</span>
          </button>
          <button type="button" onClick={() => void createWorktreeFromNode(nodeMenu.node)} disabled={!historyNodeCanCreateFrom(nodeMenu.node) || checkoutBlocked || Boolean(busyAction)}>
            <Plus size={14} />
            <span>新建工作树</span>
          </button>
          <button type="button" className="danger" onClick={() => void deleteHistoryNode(nodeMenu.node)} disabled={!nodeMenu.node.nodeID || Boolean(busyAction)}>
            <X size={14} />
            <span>删除节点</span>
          </button>
        </div>
      )}
    </div>
  );
}

function historyMessageRoleLabel(role: string): string {
  const clean = role.toLowerCase();
  if (clean === "user" || clean === "ask") {
    return "用户";
  }
  if (clean === "assistant" || clean === "vit") {
    return "Vit";
  }
  return role || "消息";
}

