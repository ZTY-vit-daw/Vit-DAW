/*
 * CONV-RAIL-1（418，B2a）：对话流左栏纯数据层。
 *
 * 输入 = /agent/ui/state 的 project_history（Go historyState 投影：branches /
 * active_branch / active_node_id / conversation_graph.nodes），输出 = 左栏树行与
 * 只读预览消息。纯函数、零 React、零请求：渲染面（ConversationRail /
 * ProjectSurface）与单测共用同一语义，避免「树行标题」和「预览路径」两处各算。
 *
 * 语义锚点（卡面 §数据与语义）：
 * - 分支行标题 = 分支自身片段（node.branch === 分支名）里首条有意义用户消息摘要
 *   （复用 lib/util 的 conversationTitleCandidate + compactConversationTitle）；
 *   新建但尚无用户消息的分支（只有 branch_marker）显示「新对话」。
 * - 分支行副标 = 分支名 · 最近活动（活动时间取 head 节点 created_at）。
 * - 只读预览消息 = 从分支 head 节点回溯 parent 链（root → head），仅 kind=ask/vit
 *   投影为 user/vit 消息（与 agent 侧 conversationMessagesForGraph 同口径）；
 *   非当前分支也能预览——不调用任何 checkout。
 * - 当前分支 head 用 active_node_id（当前对话位置）；其它分支 head 用
 *   branches[name] commit 对应的分支节点，缺省退回分支最后一个节点。
 *
 * 边界：B2a 只服务当前工作树；跨工作树浏览/打开留 B2b（419）。
 */

import type { AgentUIState, JsonRecord } from "../types";
import {
  asRecord,
  compactConversationTitle,
  conversationTitleCandidate,
  firstArray,
  textValue,
  truthy
} from "../lib/util";
import { historyWorktreeRows } from "../history/tree";

/** 只读预览消息（ProjectSurface 用 Message 组件逐条渲染）。 */
export interface ConversationPreviewMessage {
  id: string;
  author: "user" | "vit";
  text: string;
  createdAt: string;
}

/** 左栏分支行（当前工作树内）。 */
export interface ConversationBranchSummary {
  /** 分支名（即 version.checkout {branch} 的取值）。 */
  name: string;
  /** 行标题：首条有意义用户消息摘要；无则「新对话」。 */
  title: string;
  /** 行副标：`分支名 · 最近活动/当前对话`。 */
  subtitle: string;
  /** 分支 head commit（history.branches[name]；缺省取 head 节点 commit）。 */
  head: string;
  /** 分支 head 节点 id（预览回溯起点；空=该分支无节点）。 */
  headNodeId: string;
  /** 最近活动时间（ISO 字符串；空=未知）。 */
  updatedAt: string;
  /** 是否当前活动分支（左栏高亮）。 */
  active: boolean;
  /** 分支自身节点数（含分支点标记；仅展示用）。 */
  nodeCount: number;
  /** 是否有可回溯的 head 节点（无=只读预览为空）。 */
  previewAvailable: boolean;
}

/** 左栏当前工作树分组行。 */
export interface ConversationWorktreeSummary {
  label: string;
  path: string;
  projectFilePath: string;
  active: boolean;
}

/** 左栏树完整视图。 */
export interface ConversationRailView {
  worktree: ConversationWorktreeSummary | null;
  branches: ConversationBranchSummary[];
  /** 当前活动分支名（无=空）。 */
  activeBranch: string;
  /** 当前活动节点 id（新建对话分叉起点；空=不可新建）。 */
  activeNodeId: string;
  /** 是否有可展示的对话流（空=「暂无历史对话」）。 */
  hasHistory: boolean;
}

const BRANCH_MARKER_KIND = "branch_marker";

function conversationGraph(history: JsonRecord): JsonRecord {
  return asRecord(history.conversation_graph);
}

/** conversation_graph.nodes 里带 id 的节点（保持 graph 顺序=追加顺序）。 */
export function conversationNodes(history: JsonRecord): JsonRecord[] {
  return firstArray(conversationGraph(history).nodes)
    .map(asRecord)
    .filter((node) => Boolean(textValue(node.id, "")));
}

function conversationNodeKind(node: JsonRecord): string {
  return textValue(node.kind, "").toLowerCase();
}

function conversationNodeText(node: JsonRecord): string {
  // Go 侧 text 是完整正文、text_preview 是压缩预览；旧记录可能只有其一。
  return textValue(node.text, "") || textValue(node.text_preview, "") || textValue(node.message, "");
}

function conversationNodeIndex(history: JsonRecord): { nodes: JsonRecord[]; byID: Map<string, JsonRecord> } {
  const nodes = conversationNodes(history);
  const byID = new Map<string, JsonRecord>();
  nodes.forEach((node) => byID.set(textValue(node.id, ""), node));
  return { nodes, byID };
}

/** 当前活动分支（detached 时返回 detached；字段缺失时从 graph/节点回退）。 */
export function conversationActiveBranchName(history: JsonRecord): string {
  if (truthy(history.detached)) {
    return "detached";
  }
  const direct = textValue(history.active_branch, "");
  if (direct) {
    return direct;
  }
  const graphBranch = textValue(conversationGraph(history).active_branch, "");
  if (graphBranch) {
    return graphBranch;
  }
  const nodes = conversationNodes(history);
  for (let index = nodes.length - 1; index >= 0; index -= 1) {
    const branch = textValue(nodes[index].branch, "");
    if (branch) {
      return branch;
    }
  }
  return "";
}

/** 当前活动节点 id（历史字段优先，graph 同值兜底）。 */
export function conversationActiveNodeId(history: JsonRecord): string {
  return textValue(history.active_node_id, "") || textValue(conversationGraph(history).active_node_id, "");
}

/** 当前活动节点的 commit id（新建对话按该节点分叉时作为 branch_create.commit_id）。 */
export function conversationActiveNodeCommitID(history: JsonRecord): string {
  const nodeID = conversationActiveNodeId(history);
  if (!nodeID) {
    return "";
  }
  const node = conversationNodeIndex(history).byID.get(nodeID);
  return node ? textValue(node.commit_id, "") : "";
}

/** 分支名清单：history.branches 键（worker 级，排序稳定）；缺失时从节点回退。 */
export function conversationBranchNames(history: JsonRecord): string[] {
  const names = Object.keys(asRecord(history.branches))
    .map((name) => name.trim())
    .filter(Boolean)
    .sort((left, right) => left.localeCompare(right));
  if (names.length > 0) {
    return names;
  }
  const fromNodes = Array.from(
    new Set(conversationNodes(history).map((node) => textValue(node.branch, "")).filter(Boolean))
  );
  return fromNodes.sort((left, right) => left.localeCompare(right));
}

function pathToRoot(byID: Map<string, JsonRecord>, start: JsonRecord | null): JsonRecord[] {
  const path: JsonRecord[] = [];
  const visited = new Set<string>();
  let cursor: JsonRecord | null = start ?? null;
  while (cursor) {
    const id = textValue(cursor.id, "");
    if (!id || visited.has(id)) {
      break;
    }
    visited.add(id);
    path.unshift(cursor);
    cursor = byID.get(textValue(cursor.parent_node_id, "")) ?? null;
  }
  return path;
}

/**
 * 分支 head 节点：当前分支=active_node_id 指向的节点（当前对话位置）；其它分支=
 * branches[name] commit 对应的分支节点（同名 commit 在多个分支共享时按 branch 过滤），
 * 缺省退回该分支最后一个节点。
 */
export function conversationBranchHeadNode(history: JsonRecord, branchName: string): JsonRecord | null {
  const clean = branchName.trim();
  if (!clean) {
    return null;
  }
  const { nodes, byID } = conversationNodeIndex(history);
  if (clean === conversationActiveBranchName(history)) {
    const activeNode = byID.get(conversationActiveNodeId(history));
    if (activeNode) {
      return activeNode;
    }
  }
  const own = nodes.filter((node) => textValue(node.branch, "") === clean);
  if (own.length === 0) {
    return null;
  }
  const head = textValue(asRecord(history.branches)[clean], "");
  if (head) {
    for (let index = own.length - 1; index >= 0; index -= 1) {
      if (textValue(own[index].commit_id, "") === head) {
        return own[index];
      }
    }
  }
  return own[own.length - 1];
}

/**
 * 分支行标题：分支自身片段的首条有意义用户消息摘要；新分支（只有 branch_marker、
 * 尚无消息）显示「新对话」；旧记录节点无 branch 字段时退回整条路径的首条。
 */
export function conversationBranchTitle(history: JsonRecord, branchName: string): string {
  const head = conversationBranchHeadNode(history, branchName);
  if (!head) {
    return "新对话";
  }
  const { byID } = conversationNodeIndex(history);
  const path = pathToRoot(byID, head);
  const ownAsks = path.filter(
    (node) => textValue(node.branch, "") === branchName && conversationNodeKind(node) === "ask"
  );
  const hasMarker = path.some(
    (node) =>
      conversationNodeKind(node) === BRANCH_MARKER_KIND && textValue(node.branch, "") === branchName
  );
  const pathAsks = path.filter((node) => conversationNodeKind(node) === "ask");
  const candidates = ownAsks.length > 0 ? ownAsks : hasMarker ? [] : pathAsks;
  const meaningful = candidates.find((node) => conversationTitleCandidate(conversationNodeText(node)));
  if (!meaningful) {
    return "新对话";
  }
  return compactConversationTitle(conversationNodeText(meaningful));
}

/** 最近活动时间（head 节点 created_at；空=未知）。 */
export function conversationBranchUpdatedAt(history: JsonRecord, branchName: string): string {
  const head = conversationBranchHeadNode(history, branchName);
  return head ? textValue(head.created_at, "") : "";
}

/** ISO/时间串 → 稳定本地展示（MM-DD HH:mm）；非法/空返回空串。 */
export function formatConversationActivity(value: string): string {
  const raw = value.trim();
  if (!raw) {
    return "";
  }
  const date = new Date(raw);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const pad = (part: number) => String(part).padStart(2, "0");
  return `${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

/** 左栏分支行（顺序=分支名排序；active 高亮只标当前活动分支）。 */
export function conversationBranchRows(history: JsonRecord): ConversationBranchSummary[] {
  const names = conversationBranchNames(history);
  if (names.length === 0) {
    return [];
  }
  const nodes = conversationNodes(history);
  const activeBranch = conversationActiveBranchName(history);
  const branches = asRecord(history.branches);
  return names.map((name) => {
    const headNode = conversationBranchHeadNode(history, name);
    const updatedAt = headNode ? textValue(headNode.created_at, "") : "";
    const active = name === activeBranch;
    const activity = active
      ? "当前对话"
      : updatedAt
        ? `最近活动 ${formatConversationActivity(updatedAt)}`
        : "暂无活动";
    return {
      name,
      title: conversationBranchTitle(history, name),
      subtitle: `${name} · ${activity}`,
      head: textValue(branches[name], "") || (headNode ? textValue(headNode.commit_id, "") : ""),
      headNodeId: headNode ? textValue(headNode.id, "") : "",
      updatedAt,
      active,
      nodeCount: nodes.filter((node) => textValue(node.branch, "") === name).length,
      previewAvailable: Boolean(headNode)
    };
  });
}

/** 分支只读预览消息：head 节点回溯 parent 链，ask→user / vit→vit（其余节点跳过）。 */
export function conversationBranchMessages(history: JsonRecord, branchName: string): ConversationPreviewMessage[] {
  const head = conversationBranchHeadNode(history, branchName);
  if (!head) {
    return [];
  }
  const { byID } = conversationNodeIndex(history);
  const messages: ConversationPreviewMessage[] = [];
  const seen = new Set<string>();
  for (const node of pathToRoot(byID, head)) {
    const kind = conversationNodeKind(node);
    const author = kind === "ask" ? "user" : kind === "vit" ? "vit" : null;
    if (!author) {
      continue;
    }
    const text = conversationNodeText(node);
    const id = textValue(node.id, "");
    if (!text || !id || seen.has(id)) {
      continue;
    }
    seen.add(id);
    messages.push({ id, author, text, createdAt: textValue(node.created_at, "") });
  }
  return messages;
}

/** 左栏当前工作树分组（只展示当前工作树；跨工作树留 B2b/419）。 */
export function conversationWorktree(history: JsonRecord): ConversationWorktreeSummary | null {
  if (Object.keys(history).length === 0) {
    return null;
  }
  const rows = historyWorktreeRows(history);
  const row = rows.find((candidate) => candidate.active) ?? rows[0];
  if (row) {
    return {
      label: row.label || row.name || "当前工作树",
      path: row.path,
      projectFilePath: row.projectFilePath,
      active: row.active
    };
  }
  const projectPath = textValue(
    history.project_path ?? history.current_project_path ?? history.root_project_path,
    ""
  );
  if (!projectPath) {
    return null;
  }
  return {
    label: "主工作树",
    path: projectPath.replace(/[/\\][^/\\]+$/, ""),
    projectFilePath: projectPath,
    active: true
  };
}

/** 左栏树完整视图（渲染面唯一数据入口）。 */
export function conversationRailView(uiState: AgentUIState | null): ConversationRailView {
  const history = asRecord(uiState?.project_history);
  const branches = conversationBranchRows(history);
  return {
    worktree: conversationWorktree(history),
    branches,
    activeBranch: conversationActiveBranchName(history),
    activeNodeId: conversationActiveNodeId(history),
    hasHistory: branches.length > 0
  };
}

/**
 * 新对话分支名（确定性、无副作用）：`chat-YYYYMMDD-HHmmss`，撞名追加 -2/-3…
 * 纯 ASCII，避免 Go 侧 ref 与跨平台路径对中文分支名的兼容风险。
 */
export function nextConversationBranchName(existing: string[], now: Date = new Date()): string {
  const pad = (part: number) => String(part).padStart(2, "0");
  const stamp = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}-${pad(
    now.getHours()
  )}${pad(now.getMinutes())}${pad(now.getSeconds())}`;
  const base = `chat-${stamp}`;
  const used = new Set(existing.map((name) => name.trim()).filter(Boolean));
  if (!used.has(base)) {
    return base;
  }
  let suffix = 2;
  while (used.has(`${base}-${suffix}`)) {
    suffix += 1;
  }
  return `${base}-${suffix}`;
}
