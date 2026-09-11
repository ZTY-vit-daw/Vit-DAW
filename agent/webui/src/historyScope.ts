import type { AgentUIState } from "./types";

// CONTRACT-3（2026-09-05）：会话内 scope 演进 ≠ 工程切换。
// 旧版 scope 键把 state_dir/history_dir/active_worktree/active_branch 等
// 晚物化字段编进身份——链首批 checkpoint 落地后这些字段从空物化为真实值，
// 键中途变化被 history-sync 当作工程切换执行换会话+清流（终审复现：
// 消息流重置为问候语、轨迹块消失、事件 seq 归零）。
// 本模块把 scope 拆成「稳定身份键」与「晚物化值」：身份键只含
// projectPath/rootProjectPath（uuid 只作切换旁证，不进键——uuid 与
// state_dir 同批物化，进键会让每个工程首个 checkpoint 都抖动一次）。

export interface HistoryScopeParts {
  /** 内核状态提供的稳定工程路径（含 project_history/ui_context 兜底链） */
  projectPath: string;
  /** 仅 worktree 工程非空；随首批历史落地物化 */
  rootProjectPath: string;
  projectUUID: string;
  stateDir: string;
  historyDir: string;
  activeWorktree: string;
  activeBranch: string;
}

export type HistoryScopeChangeKind =
  /** 首次观测（挂载后第一拍）：按工程恢复锚定会话 */
  | "initial"
  /** 身份与值都没变化：常规历史合并 */
  | "none"
  /** 同会话内键演进（root 物化、unsaved→具体路径等）：保留消息流与事件 seq，迁移存储桶 */
  | "evolution"
  /** 真工程/工作区切换（或同路径 uuid 更换）：换会话+清流照旧 */
  | "switch";

function recordText(...values: unknown[]): string {
  for (const value of values) {
    if (typeof value !== "string" && typeof value !== "number") {
      continue;
    }
    const text = String(value).trim();
    if (text) {
      return text;
    }
  }
  return "";
}

function recordOf(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" ? (value as Record<string, unknown>) : {};
}

export function historyScopePartsFromUIState(uiState: AgentUIState | null): HistoryScopeParts {
  const projectHistory = recordOf(uiState?.project_history);
  const project = recordOf(uiState?.project);
  const uiContext = recordOf(uiState?.ui_context);
  return {
    projectPath: recordText(
      projectHistory.project_path,
      projectHistory.current_project_path,
      project.project_path,
      uiContext.project_path,
      uiContext.current_project_path
    ),
    rootProjectPath: recordText(projectHistory.root_project_path),
    projectUUID: recordText(projectHistory.project_uuid),
    stateDir: recordText(projectHistory.state_dir),
    historyDir: recordText(projectHistory.history_dir),
    activeWorktree: recordText(projectHistory.active_worktree),
    activeBranch: recordText(projectHistory.active_branch)
  };
}

/** 身份键：稳定身份（projectPath/rootProjectPath）之外的字段一律不进键 */
export function historyScopeKeyFromParts(parts: HistoryScopeParts): string {
  return [parts.projectPath || parts.rootProjectPath || "unsaved", parts.rootProjectPath || "root"].join("::");
}

export function historyScopeKeyFromUIState(uiState: AgentUIState | null): string {
  return historyScopeKeyFromParts(historyScopePartsFromUIState(uiState));
}

function normalizeWorkspacePath(value: string): string {
  return value.trim().replace(/\\/g, "/").replace(/\/+$/, "").toLowerCase();
}

/** 工作区坐标：判「真切换」用的稳定路径（projectPath 优先，root 兜底） */
export function concreteWorkspacePath(parts: HistoryScopeParts): string {
  return normalizeWorkspacePath(parts.projectPath || parts.rootProjectPath);
}

export interface HistoryScopeChangeInput {
  previousKey: string;
  /** 最近一次观测到的具体工作区坐标（"unsaved" 拍不回退，保证 关A→关工程→开B 仍判切换） */
  previousConcretePath: string;
  previousUUID: string;
  nextParts: HistoryScopeParts;
}

export function classifyHistoryScopeChange(input: HistoryScopeChangeInput): HistoryScopeChangeKind {
  if (!input.previousKey) {
    return "initial";
  }
  const nextKey = historyScopeKeyFromParts(input.nextParts);
  const previousConcrete = normalizeWorkspacePath(input.previousConcretePath);
  const nextConcrete = concreteWorkspacePath(input.nextParts);
  // uuid 双侧已知且不同 = 同路径下工程身份被更换（重建/替换）
  const uuidSwitch = Boolean(
    input.previousUUID && input.nextParts.projectUUID && input.previousUUID !== input.nextParts.projectUUID
  );
  if (nextKey === input.previousKey) {
    return uuidSwitch ? "switch" : "none";
  }
  if (previousConcrete && nextConcrete && previousConcrete !== nextConcrete) {
    return "switch";
  }
  // 键变化但工作区坐标一致或单侧未物化 = 会话内演进（物化/晚到身份）
  return "evolution";
}

/**
 * B9 症4（刷新后轨迹消失）：刷新挂载首拍 uiState scope 未物化（unsaved::root）
 * 时，initial 分支会造一个新随机会话 id；下一拍演进到真实 scope 判 evolution
 * （previousConcrete 未物化不满足 switch 条件）并保留该新 id——存档锚定 id 不
 * 被回读，/agent/events 回放打到空缓冲，轨迹块全部消失（终局消息可独立存活：
 * 服务端会话图水合路径不受影响）。evolution 分支在「当前流无有效消息且该 scope
 * 存有锚定会话 id」时采纳存档 id，触发事件回放重建轨迹。有真实对话内容的
 * 现役会话不采纳（不劫持活的 unsaved 会话）。
 */
export function shouldAdoptStoredConversationOnScopeEvolution(input: {
  storedConversationID: string;
  currentConversationID: string;
  hasMeaningfulMessages: boolean;
}): boolean {
  return (
    Boolean(input.storedConversationID) &&
    input.storedConversationID !== input.currentConversationID &&
    !input.hasMeaningfulMessages
  );
}
