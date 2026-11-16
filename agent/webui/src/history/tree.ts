// T9-PRE-1A-1（施工图 UI-CONVERGE-RECON-1 §3.1 批 1 / 步 1d）：App.tsx historyTree*/historyWorktree* 纯函数群 + History* 类型出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变（HistoryPane 组件留 App，类型经兼容层引回）。
// 树布局算法（historyTreeLayout）为纯函数，可独立测试。

import type { AgentUIState, JsonRecord } from "../types";
import { asRecord, firstArray, lastPathPart, textValue, truthy } from "../lib/util";
import { historyScopeKeyFromUIState } from "../historyScope";

type HistoryTreeNode = {
  index: number;
  id: string;
  nodeID: string;
  parentID: string;
  parentIndex: number;
  kind: string;
  label: string;
  meta: string;
  summary: string;
  branch: string;
  commitID: string;
  createdAt: string;
  active: boolean;
  depth: number;
  raw: JsonRecord;
};

type HistoryBranchRow = {
  name: string;
  head: string;
  active: boolean;
};

type HistoryWorktreeRow = {
  key: string;
  name: string;
  label: string;
  path: string;
  projectFilePath: string;
  commitID: string;
  active: boolean;
  isRoot: boolean;
  raw: JsonRecord;
};

type HistoryLoadedWorktree = {
  key: string;
  history: JsonRecord;
};

type HistoryNodeMenuState = {
  node: HistoryTreeNode;
  x: number;
  y: number;
};

type HistoryTreeLayout = {
  nodes: Array<
    HistoryTreeNode & {
      x: number;
      y: number;
    }
  >;
  connections: Array<{ id: string; points: string }>;
  width: number;
  height: number;
};


function historyTreeNodes(history: JsonRecord): HistoryTreeNode[] {
  const graph = asRecord(history.conversation_graph);
  const rawNodes = firstArray(graph.nodes).map(asRecord).filter((node) => textValue(node.id, "") && textValue(node.commit_id, ""));
  const activeNodeID = textValue(history.active_node_id ?? graph.active_node_id, "");
  const activeBranch = historyActiveBranch(history);
  const byID = new Map<string, JsonRecord>();
  rawNodes.forEach((node) => byID.set(textValue(node.id, ""), node));
  const childrenByParent = new Map<string, JsonRecord[]>();
  rawNodes.forEach((node) => {
    const parentID = textValue(node.parent_node_id, "");
    if (!parentID || !byID.has(parentID)) {
      return;
    }
    const children = childrenByParent.get(parentID) ?? [];
    children.push(node);
    childrenByParent.set(parentID, children);
  });
  const ordered: Array<Omit<HistoryTreeNode, "index" | "parentIndex">> = [];
  const visited = new Set<string>();
  const appendNode = (node: JsonRecord, depth: number) => {
    const id = textValue(node.id, "");
    if (!id || visited.has(id)) {
      return;
    }
    visited.add(id);
    const kind = textValue(node.kind, "checkpoint").toLowerCase();
    const commitID = textValue(node.commit_id, "");
    const branch = textValue(node.branch, activeBranch);
    ordered.push({
      id,
      nodeID: id,
      parentID: textValue(node.parent_node_id, ""),
      kind,
      label: historyKindLabel(kind),
      meta: historyShortID(commitID),
      summary: historyNodeSummary(node, kind),
      branch,
      commitID,
      createdAt: textValue(node.created_at, ""),
      active: id === activeNodeID,
      depth,
      raw: node
    });
    (childrenByParent.get(id) ?? []).forEach((child) => appendNode(child, depth + 1));
  };
  rawNodes.forEach((node) => {
    const parentID = textValue(node.parent_node_id, "");
    if (!parentID || !byID.has(parentID)) {
      appendNode(node, 0);
    }
  });
  rawNodes.forEach((node) => appendNode(node, 0));
  const indexByID = new Map<string, number>();
  ordered.forEach((node, index) => indexByID.set(node.id, index));
  return ordered.map((node, index) => ({
    ...node,
    index,
    parentIndex: node.parentID ? indexByID.get(node.parentID) ?? -1 : -1
  }));
}

function historyTreeLayout(nodes: HistoryTreeNode[]): HistoryTreeLayout {
  if (nodes.length === 0) {
    return { nodes: [], connections: [], width: 260, height: 160 };
  }
  const cardWidth = 108;
  const cardHeight = 42;
  const rowHeight = 72;
  const laneGap = 92;
  const pad = 8;
  const children = new Map<number, number[]>();
  nodes.forEach((node, index) => {
    if (node.parentIndex < 0 || node.parentIndex >= nodes.length) {
      return;
    }
    const rows = children.get(node.parentIndex) ?? [];
    rows.push(index);
    children.set(node.parentIndex, rows);
  });
  const roots = nodes.map((node, index) => ({ node, index })).filter(({ node }) => node.parentIndex < 0 || node.parentIndex >= nodes.length).map(({ index }) => index);
  const layout = new Map<number, { lane: number; depth: number }>();
  const visiting = new Set<number>();
  let nextLane = 0;
  const assign = (index: number, depth: number): number => {
    const existing = layout.get(index);
    if (existing) {
      return existing.lane;
    }
    if (visiting.has(index)) {
      const lane = nextLane++;
      layout.set(index, { lane, depth });
      return lane;
    }
    visiting.add(index);
    const childRows = children.get(index) ?? [];
    let lane = 0;
    if (childRows.length === 0) {
      lane = nextLane++;
    } else {
      const childLanes = childRows.map((childIndex) => assign(childIndex, depth + 1));
      lane = (childLanes[0] + childLanes[childLanes.length - 1]) / 2;
    }
    visiting.delete(index);
    layout.set(index, { lane, depth });
    return lane;
  };
  (roots.length > 0 ? roots : [0]).forEach((index) => assign(index, 0));
  nodes.forEach((_, index) => {
    if (!layout.has(index)) {
      assign(index, 0);
    }
  });
  const maxDepth = Math.max(0, ...Array.from(layout.values()).map((row) => row.depth));
  const leafCount = Math.max(1, Math.ceil(nextLane));
  const naturalWidth = cardWidth + Math.max(0, leafCount - 1) * laneGap;
  const width = Math.max(260, pad * 2 + naturalWidth);
  const height = Math.max(160, pad * 2 + cardHeight + maxDepth * rowHeight);
  const horizontalOffset = Math.max(pad, (width - naturalWidth) / 2);
  const positioned = nodes.map((node, index) => {
    const row = layout.get(index) ?? { lane: 0, depth: node.depth };
    return {
      ...node,
      x: horizontalOffset + row.lane * laneGap,
      y: pad + row.depth * rowHeight
    };
  });
  const connections = positioned
    .filter((node) => node.parentIndex >= 0 && positioned[node.parentIndex])
    .map((node) => {
      const parent = positioned[node.parentIndex];
      const startX = parent.x + cardWidth / 2;
      const startY = parent.y + cardHeight;
      const endX = node.x + cardWidth / 2;
      const endY = node.y;
      const midY = startY + (endY - startY) * 0.52;
      return {
        id: `${parent.id}:${node.id}`,
        points: `${startX},${startY} ${startX},${midY} ${endX},${midY} ${endX},${endY}`
      };
    });
  return { nodes: positioned, connections, width, height };
}

function historyBranchRows(history: JsonRecord): HistoryBranchRow[] {
  const activeBranch = historyActiveBranch(history);
  const branches = asRecord(history.branches);
  const names = Object.keys(branches).sort((left, right) => left.localeCompare(right));
  if (names.length === 0 && activeBranch) {
    return [{ name: activeBranch, head: textValue(history.head, ""), active: true }];
  }
  return names.map((name) => ({
    name,
    head: textValue(branches[name], ""),
    active: name === activeBranch
  }));
}

function historyWorktreeRows(history: JsonRecord): HistoryWorktreeRow[] {
  const rows: HistoryWorktreeRow[] = [];
  const rootProjectPath = textValue(history.root_project_path, "") || textValue(history.project_path, "");
  if (rootProjectPath) {
    const rootPath = rootProjectPath.replace(/[/\\][^/\\]+$/, "");
    rows.push({
      key: historyWorktreeKey(rootProjectPath, rootPath, "main"),
      name: "",
      label: "主工作树",
      path: rootPath,
      projectFilePath: rootProjectPath,
      commitID: textValue(history.head, ""),
      active: historyActiveWorktree(history) === "main",
      isRoot: true,
      raw: { project_file_path: rootProjectPath, is_root: true }
    });
  }
  const seen = new Set(rows.map((row) => row.key));
  firstArray(history.worktrees).map(asRecord).forEach((row) => {
    const item = historyWorktreeRowFromRecord(row, history);
    if (!item.key || seen.has(item.key)) {
      return;
    }
    seen.add(item.key);
    rows.push(item);
  });
  return rows;
}

function historyWorktreeRowFromRecord(row: JsonRecord, history: JsonRecord): HistoryWorktreeRow {
  const name = textValue(row.name, "");
  const projectFilePath = textValue(row.project_file_path, "");
  const path = textValue(row.path, "");
  const commitID = textValue(row.worktree_head_commit_id ?? row.head ?? row.commit_id ?? row.origin_commit_id, "");
  const isRoot = truthy(row.is_root);
  const label = textValue(row.label, "") || (isRoot ? "主工作树" : name || lastPathPart(projectFilePath || path) || "工作树");
  const key = historyWorktreeKey(projectFilePath, path, isRoot ? "main" : name || label);
  const activeWorktree = historyActiveWorktree(history);
  const active = isRoot ? activeWorktree === "main" : Boolean(name) && name === textValue(history.active_worktree, "");
  return { key, name, label, path, projectFilePath, commitID, active, isRoot, raw: row };
}

function historyWorktreeKey(projectFilePath: string, path: string, name: string): string {
  return (projectFilePath || path || name).replace(/\\/g, "/").trim().toLowerCase();
}

function historyToolArgs(history: JsonRecord, args: JsonRecord): JsonRecord {
  const out: JsonRecord = { ...args };
  if (!textValue(out.project_path, "")) {
    const projectPath = historyProjectPath(history);
    if (projectPath) {
      out.project_path = projectPath;
    }
  }
  return out;
}

function historyWorktreeCheckoutArgs(worktree: HistoryWorktreeRow): JsonRecord {
  const args: JsonRecord = {};
  if (worktree.name && !worktree.isRoot) {
    args.name = worktree.name;
  }
  if (worktree.projectFilePath) {
    args.project_file_path = worktree.projectFilePath;
  }
  return args;
}

function buildHistoryInvokeContext(uiState: AgentUIState | null, sourceHistory?: JsonRecord): JsonRecord {
  const history = sourceHistory && Object.keys(sourceHistory).length > 0 ? sourceHistory : asRecord(uiState?.project_history);
  return {
    project_history: history,
    history_scope_key: historyScopeKeyFromUIState(uiState),
    project_path: historyProjectPath(history),
    root_project_path: textValue(history.root_project_path, ""),
    active_branch: historyActiveBranch(history),
    active_worktree: textValue(history.active_worktree, ""),
    active_node_id: textValue(history.active_node_id, "")
  };
}

function historyProjectPath(history: JsonRecord): string {
  return textValue(history.project_path ?? history.current_project_path ?? history.root_project_path, "");
}

function historyActiveBranch(history: JsonRecord): string {
  if (truthy(history.detached)) {
    return "detached";
  }
  return textValue(history.active_branch, "main");
}

function historyActiveWorktree(history: JsonRecord): string {
  return textValue(history.active_worktree, "") || "main";
}

function historyKindLabel(kind: string): string {
  switch (kind.toLowerCase()) {
    case "ask":
      return "Ask";
    case "vit":
      return "Vit";
    case "branch_marker":
      return "分支点";
    case "checkpoint":
      return "Saved";
    default:
      return kind || "鑺傜偣";
  }
}

function historyNodeSummary(node: JsonRecord, kind: string): string {
  const summary = textValue(node.text_preview ?? node.text ?? node.summary ?? node.message, "");
  if (summary) {
    return summary.length > 92 ? `${summary.slice(0, 89)}...` : summary;
  }
  return `${historyKindLabel(kind)} saved state`;
}

function historyNodeTitle(node: HistoryTreeNode): string {
  const label = node.label || historyKindLabel(node.kind);
  const branch = node.branch && node.branch !== "main" ? ` / ${node.branch}` : "";
  return `${label}${branch}`;
}

function historyNodeCanCreateFrom(node: HistoryTreeNode): boolean {
  return Boolean(node.commitID) && node.kind === "vit";
}

function historyStatusIsError(status: string): boolean {
  const lower = status.toLowerCase();
  return lower.includes("error") || lower.includes("failed") || status.includes("失败") || status.includes("不可用");
}

function historyShortID(value: string): string {
  const clean = value.trim();
  if (!clean || clean === "-") {
    return "";
  }
  return clean.length > 10 ? clean.slice(0, 10) : clean;
}

function historySafeName(value: string): string {
  return value
    .trim()
    .replace(/[\\/:*?"<>|]+/g, "-")
    .replace(/\s+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 64);
}

export type {
  HistoryTreeNode,
  HistoryBranchRow,
  HistoryWorktreeRow,
  HistoryLoadedWorktree,
  HistoryNodeMenuState,
  HistoryTreeLayout
};

export {
  historyTreeNodes,
  historyTreeLayout,
  historyBranchRows,
  historyWorktreeRows,
  historyWorktreeRowFromRecord,
  historyWorktreeKey,
  historyToolArgs,
  historyWorktreeCheckoutArgs,
  buildHistoryInvokeContext,
  historyProjectPath,
  historyActiveBranch,
  historyActiveWorktree,
  historyKindLabel,
  historyNodeSummary,
  historyNodeTitle,
  historyNodeCanCreateFrom,
  historyStatusIsError,
  historyShortID,
  historySafeName
};
