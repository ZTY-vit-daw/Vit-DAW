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
  /** 当前活动节点 id（新建对话分叉起点；空=无活动节点，可走当前 HEAD 分叉）。 */
  activeNodeId: string;
  /** 是否有工程身份（project_path/active branch/head 任一可得）——无身份才禁用「新对话」。 */
  hasProjectIdentity: boolean;
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

/** 单个分支行摘要（标题/副标/head 节点/活动时间；历史缺失时降级为「新对话」）。 */
export function conversationBranchSummary(history: JsonRecord, branchName: string): ConversationBranchSummary {
  const name = branchName.trim();
  const headNode = name ? conversationBranchHeadNode(history, name) : null;
  const updatedAt = headNode ? textValue(headNode.created_at, "") : "";
  const active = Boolean(name) && name === conversationActiveBranchName(history);
  const activity = active
    ? "当前对话"
    : updatedAt
      ? `最近活动 ${formatConversationActivity(updatedAt)}`
      : "暂无活动";
  return {
    name,
    title: name ? conversationBranchTitle(history, name) : "新对话",
    subtitle: `${name} · ${activity}`,
    head: textValue(asRecord(history.branches)[name], "") || (headNode ? textValue(headNode.commit_id, "") : ""),
    headNodeId: headNode ? textValue(headNode.id, "") : "",
    updatedAt,
    active,
    nodeCount: name ? conversationNodes(history).filter((node) => textValue(node.branch, "") === name).length : 0,
    previewAvailable: Boolean(headNode)
  };
}

/** 左栏分支行（顺序=分支名排序；active 高亮只标当前活动分支）。 */
export function conversationBranchRows(history: JsonRecord): ConversationBranchSummary[] {
  return conversationBranchNames(history).map((name) => conversationBranchSummary(history, name));
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

/**
 * 工程身份存在性（SHELL-PROJECT-STREAM-FIX-1/427）：无活动节点但能确定当前工程/
 * 分支/HEAD 时，新对话仍可从当前 HEAD 分叉——只有「无工程身份」才禁用。
 */
export function conversationHasProjectIdentity(history: JsonRecord): boolean {
  return Boolean(
    textValue(history.project_path ?? history.current_project_path, "") ||
      conversationActiveBranchName(history) ||
      textValue(history.head, "")
  );
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
    hasProjectIdentity: conversationHasProjectIdentity(history),
    hasHistory: branches.length > 0
  };
}

/* ============================================================================
 * CONV-RAIL-CROSS-1（421，B2b）：工作区 → 工作树 → 分支 树与分支归属
 *
 * 数据面：
 * - 工作树清单 = historyWorktreeRows(history)（现有纯函数；行含 name/label/path/
 *   project_file_path/head commit/active）。当前工作树的历史 = 当前 ui state；
 *   其它工作树只在展开时经 version.status 懒加载（ProjectSurface 持有缓存态），
 *   本层接收 statusByKey 纯数据。
 * - 分支归属（同一分支只出现在一个工作树组，B2a 边界收口）优先级：
 *   ① 该分支是某工作树的 active_branch（当前工作树优先，然后其它已加载/未加载行）；
 *   ② 分支 head commit == 某工作树 head（唯一命中才归属，多个命中视为无法归属）；
 *   ③ 已加载的其它工作树 conversation_graph 里存在该分支节点；
 *   ④ 当前工作树 conversation_graph 里存在该分支节点（B2a 连续性）；
 *   ⑤ 仍无法归属 → 工作区级「其他分支」组（只读，无继续动作）。
 * - 预览/继续由渲染面料消费 branchTarget（含归属工作树、该工作树 history 与 head
 *   节点 id）：当前工作树走 version.checkout，其它工作树走 version.conversation_open。
 * ========================================================================== */

/** 其它工作树懒加载状态（缓存于 ProjectSurface；rail 只读消费）。 */
export interface ConversationWorktreeStatus {
  loading?: boolean;
  error?: string;
  history?: JsonRecord | null;
}

export type ConversationWorktreeStatusMap = Record<string, ConversationWorktreeStatus>;

/** 左栏可点的分支行 = 分支摘要 + 归属工作树 + 预览所需 history/head 节点。 */
export interface ConversationBranchTarget extends ConversationBranchSummary {
  /** 分支名（=name；继续/打开动作的 branch 实参直接用该字段）。 */
  branch: string;
  /** 归属工作树 key（工作区级「其他分支」为固定值 OTHER_BRANCHES_KEY）。 */
  worktreeKey: string;
  worktreeLabel: string;
  projectFilePath: string;
  /** 是否当前活动工作树（决定【继续此对话】vs【打开此工作树并继续此对话】）。 */
  isActiveWorktree: boolean;
  /** 工作区级未归属分支：只读浏览，无继续动作。 */
  readOnly: boolean;
  /** 该分支所属工作树的 history（只读预览回溯源；不触发任何写操作）。 */
  history: JsonRecord;
}

/** 其它工作树分组行（折叠态也显示名称/路径/head/已知当前分支）。 */
export interface ConversationWorktreeGroupView {
  key: string;
  name: string;
  label: string;
  path: string;
  projectFilePath: string;
  headCommitID: string;
  active: boolean;
  isRoot: boolean;
  /** 该工作树当前分支（未懒加载时为空=未知，不伪造）。 */
  currentBranch: string;
  /** 是否已取到 version.status 历史。 */
  loaded: boolean;
  loading: boolean;
  error: string;
  branches: ConversationBranchTarget[];
}

/** 左栏跨工作树树视图。 */
export interface ConversationRailGroupsView {
  activeGroup: ConversationWorktreeGroupView | null;
  otherGroups: ConversationWorktreeGroupView[];
  /** 工作区级「其他分支」组（只读）。 */
  otherBranches: ConversationBranchTarget[];
  activeBranch: string;
  activeNodeId: string;
  /** 是否有工程身份（无活动节点时仍可「从当前工程状态新建对话」，427 语义）。 */
  hasProjectIdentity: boolean;
  hasHistory: boolean;
}

/** 工作区级「其他分支」的固定 key（不属于任何工作树）。 */
export const OTHER_BRANCHES_KEY = "workspace:other-branches";

function hasHistoryData(history: JsonRecord | null | undefined): history is JsonRecord {
  return Boolean(history && Object.keys(history).length > 0);
}

function branchHasGraphNodes(history: JsonRecord, branchName: string): boolean {
  return conversationNodes(history).some((node) => textValue(node.branch, "") === branchName);
}

function branchCommitRef(histories: JsonRecord[], branchName: string): string {
  for (const history of histories) {
    const head = textValue(asRecord(history.branches)[branchName], "");
    if (head) {
      return head;
    }
  }
  return "";
}

/** 分支目标行：摘要优先取「有该分支节点」的工作树历史（保证标题/预览一致）。 */
function branchTargetFor(
  branchName: string,
  histories: JsonRecord[],
  fallback: JsonRecord,
  worktreeKey: string,
  worktreeLabel: string,
  projectFilePath: string,
  isActiveWorktree: boolean,
  readOnly: boolean
): ConversationBranchTarget {
  const withNodes = histories.find((history) => branchHasGraphNodes(history, branchName)) ?? null;
  const source = withNodes ?? histories[0] ?? fallback;
  return {
    ...conversationBranchSummary(source, branchName),
    branch: branchName,
    worktreeKey,
    worktreeLabel,
    projectFilePath,
    isActiveWorktree,
    readOnly,
    history: source
  };
}

/**
 * 左栏跨工作树树视图：工作区 → 当前工作树组 + 其它工作树组（折叠/懒加载）+ 其他分支。
 * 纯函数：不发起请求、不读写缓存；statusByKey 由 ProjectSurface 注入。
 */
export function conversationWorktreeGroups(
  history: JsonRecord,
  statusByKey: ConversationWorktreeStatusMap = {}
): ConversationRailGroupsView {
  const emptyView: ConversationRailGroupsView = {
    activeGroup: null,
    otherGroups: [],
    otherBranches: [],
    activeBranch: "",
    activeNodeId: "",
    hasProjectIdentity: false,
    hasHistory: false
  };
  if (!hasHistoryData(history)) {
    return emptyView;
  }
  const rows = historyWorktreeRows(history);
  if (rows.length === 0) {
    // 无 worktree 记录时的单工作树回退（B2a 同款；project_path 推导已在 conversationWorktree）。
    const fallbackWorktree = conversationWorktree(history);
    if (!fallbackWorktree) {
      return emptyView;
    }
    const key = fallbackWorktree.projectFilePath.replace(/\\/g, "/").toLowerCase() || "main";
    const branches = conversationBranchNames(history).map((name) =>
      branchTargetFor(name, [history], history, key, fallbackWorktree.label, fallbackWorktree.projectFilePath, true, false)
    );
    const activeBranch = conversationActiveBranchName(history);
    return {
      activeGroup: {
        key,
        name: "",
        label: fallbackWorktree.label,
        path: fallbackWorktree.path,
        projectFilePath: fallbackWorktree.projectFilePath,
        headCommitID: textValue(history.head, ""),
        active: true,
        isRoot: true,
        currentBranch: activeBranch,
        loaded: true,
        loading: false,
        error: "",
        branches
      },
      otherGroups: [],
      otherBranches: [],
      activeBranch,
      activeNodeId: conversationActiveNodeId(history),
      hasProjectIdentity: conversationHasProjectIdentity(history),
      hasHistory: branches.length > 0
    };
  }
  // 当前工作树行：historyWorktreeRows 的 active 标记缺省为 root(none)；再用
  // project_path 兜底匹配（active_worktree 名称与 worktree 行不一致时不丢当前组）。
  let activeRow = rows.find((row) => row.active) ?? null;
  if (!activeRow) {
    const currentProjectPath = textValue(history.project_path ?? history.current_project_path, "");
    if (currentProjectPath) {
      activeRow =
        rows.find((row) => row.projectFilePath && row.projectFilePath.toLowerCase() === currentProjectPath.toLowerCase()) ??
        null;
    }
  }
  activeRow = activeRow ?? rows[0];
  const orderedRows = [activeRow, ...rows.filter((row) => row !== activeRow)];
  const activeKey = activeRow.key;
  const known = new Map<string, JsonRecord>();
  if (hasHistoryData(history)) {
    known.set(activeKey, history);
  }
  orderedRows.forEach((row) => {
    if (row.key === activeKey) {
      return;
    }
    const status = statusByKey[row.key];
    if (status?.history && hasHistoryData(status.history)) {
      known.set(row.key, status.history);
    }
  });
  const orderedHistories = orderedRows
    .map((row) => known.get(row.key) ?? null)
    .filter(hasHistoryData);
  const names = Array.from(
    new Set(orderedHistories.flatMap((item) => conversationBranchNames(item)))
  ).sort((left, right) => left.localeCompare(right));
  const heads = new Map<string, string>();
  orderedRows.forEach((row) => {
    const rowHistory = known.get(row.key) ?? null;
    heads.set(row.key, textValue(rowHistory?.head, "") || row.commitID);
  });
  const branchOf = (rowKey: string): string => {
    const rowHistory = known.get(rowKey);
    return rowHistory ? conversationActiveBranchName(rowHistory) : "";
  };
  const activeBranch = conversationActiveBranchName(history);
  // ① active_branch 所有者优先（当前工作树先，然后其它行）。
  const owners = new Map<string, string>();
  orderedRows.forEach((row) => {
    const branch = branchOf(row.key);
    if (branch && names.includes(branch) && !owners.has(branch)) {
      owners.set(branch, row.key);
    }
  });
  // ② head commit == 工作树 head（唯一命中才归属；多命中=无法归属）。
  names.forEach((name) => {
    if (owners.has(name)) {
      return;
    }
    const head = branchCommitRef(orderedHistories, name);
    if (!head) {
      return;
    }
    const candidates = orderedRows.filter((row) => heads.get(row.key) === head);
    if (candidates.length === 1) {
      owners.set(name, candidates[0].key);
    }
  });
  // ③ 已加载的其它工作树 conversation_graph 含该分支节点（对话确实发生在那棵工作树）。
  names.forEach((name) => {
    if (owners.has(name)) {
      return;
    }
    const candidate = orderedRows.find((row) => {
      if (row.key === activeKey) {
        return false;
      }
      const rowHistory = known.get(row.key);
      return Boolean(rowHistory && branchHasGraphNodes(rowHistory, name));
    });
    if (candidate) {
      owners.set(name, candidate.key);
    }
  });
  // ④ 当前工作树 conversation_graph 含该分支节点（B2a 连续性：仍在当前组）。
  names.forEach((name) => {
    if (!owners.has(name) && branchHasGraphNodes(history, name)) {
      owners.set(name, activeKey);
    }
  });
  const groupFor = (row: (typeof orderedRows)[number], isActive: boolean): ConversationWorktreeGroupView => {
    const rowHistory = known.get(row.key) ?? null;
    const status = statusByKey[row.key] ?? {};
    const ownedNames = isActive || rowHistory ? names.filter((name) => owners.get(name) === row.key) : [];
    const preferredHistories = rowHistory ? [rowHistory] : orderedHistories;
    const branches = ownedNames.map((name) =>
      branchTargetFor(name, preferredHistories, history, row.key, row.label || row.name || "工作树", row.projectFilePath, isActive, false)
    );
    return {
      key: row.key,
      name: row.name,
      label: row.label || row.name || "工作树",
      path: row.path,
      projectFilePath: row.projectFilePath,
      headCommitID: heads.get(row.key) ?? "",
      active: isActive,
      isRoot: row.isRoot,
      currentBranch: isActive ? activeBranch : rowHistory ? conversationActiveBranchName(rowHistory) : "",
      loaded: Boolean(rowHistory),
      loading: Boolean(status.loading),
      error: textValue(status.error, ""),
      branches
    };
  };
  const activeGroup = groupFor(activeRow, true);
  const otherGroups = orderedRows
    .filter((row) => row !== activeRow)
    .map((row) => groupFor(row, false))
    .sort((left, right) => left.label.localeCompare(right.label));
  const otherBranches = names
    .filter((name) => !owners.has(name))
    .map((name) =>
      branchTargetFor(name, orderedHistories, history, OTHER_BRANCHES_KEY, "其他分支", "", false, true)
    );
  return {
    activeGroup,
    otherGroups,
    otherBranches,
    activeBranch,
    activeNodeId: conversationActiveNodeId(history),
    hasProjectIdentity: conversationHasProjectIdentity(history),
    hasHistory: Boolean(
      activeGroup && (activeGroup.branches.length > 0 || otherGroups.length > 0 || otherBranches.length > 0)
    )
  };
}

/** 目标分支是否=当前活动工作树的当前分支（决定继续动作语义）。 */
export function conversationTargetIsActiveBranch(
  groups: ConversationRailGroupsView,
  target: ConversationBranchTarget
): boolean {
  return Boolean(
    groups.activeGroup &&
      target.worktreeKey === groups.activeGroup.key &&
      target.branch === groups.activeBranch
  );
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
