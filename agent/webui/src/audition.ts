import type { AgentEvent, JsonRecord } from "./types";
import type { TrajectoryState } from "./trajectory";

export const auditionSchemaVersion = "vit.kernel_audition.v1";

export interface AuditionCandidate {
  id: string;
  label: string;
  status: string;
  sourceRef: string;
  previewRef: string;
  checkpointRef: string;
  commitID: string;
  branchRef: string;
  worktreeRef: string;
  projectPath: string;
  projectRevision: string;
}

export type HeardDifference = "yes" | "no" | "unsure";
export type JudgmentPreference = "a" | "b" | "neither" | "equal" | "unsure";

export interface AuditionSession {
  id: string;
  status: string;
  activeCandidateId: string;
  candidates: AuditionCandidate[];
  activeProjectRevision: string;
  projectRevision: string;
  stateRevision: number;
  lastEventType: string;
  error: string;
  conversationID: string;
  turnID: string;
  roundID: string;
  judgmentRequested: boolean;
  judgmentRecorded: boolean;
  judgmentEvidence: JsonRecord | null;
  /** 盲态会话（B12-1）：模式标志，绝不表示哪个标签是哪一版——判定前 UI 必须保持中立措辞 */
  blind: boolean;
  /** 判定落账后的解盲披露（物理指派 + 对应动作结果）；判定前恒为 null */
  blindDisclosure: JsonRecord | null;
  inspectedCandidateId: string;
  adoptedCandidateId: string;
  adoptionStatus: string;
  inspectionReceipt: JsonRecord | null;
  adoptionReceipt: JsonRecord | null;
}

export interface AuditionState {
  sessions: Record<string, AuditionSession>;
  eventKeys: string[];
}

export function emptyAuditionState(): AuditionState {
  return { sessions: {}, eventKeys: [] };
}

export function isAuditionEvent(event: AgentEvent): boolean {
  return text(event.type).startsWith("audition.");
}

export function reduceAuditionEvents(current: AuditionState, incoming: AgentEvent[]): AuditionState {
  const next: AuditionState = { sessions: { ...current.sessions }, eventKeys: [...current.eventKeys] };
  const seen = new Set(next.eventKeys);
  incoming.filter((event) => isAuditionEvent(event) || event.type === "trajectory.user_judgment.requested" || event.type === "trajectory.user_judgment.recorded").sort((a, b) => (a.seq || 0) - (b.seq || 0)).forEach((event) => {
    const payload = record(event.payload);
    const rawSession = record(payload.session);
    const requestedSession = text(record(payload.details).audition_session_id) || text(payload.audition_session_id);
    const id = text(rawSession.session_id) || requestedSession || text(event.item_id);
    if (!id) return;
    const key = `${id}:${text(event.type)}:${event.seq || 0}`;
    if (seen.has(key)) return;
    const previous = next.sessions[id];
    const activeProject = record(rawSession.active_project_plane);
    const candidates = Array.isArray(rawSession.candidates)
      ? rawSession.candidates.map(candidateFromAny).filter((candidate) => candidate.id)
      : previous?.candidates ?? [];
    const trajectoryJudgmentEvent = event.type === "trajectory.user_judgment.requested" || event.type === "trajectory.user_judgment.recorded";
    next.sessions[id] = {
      id,
      status: text(rawSession.status) || (trajectoryJudgmentEvent ? previous?.status : text(event.status)) || previous?.status || "preparing",
      activeCandidateId: text(rawSession.active_candidate_id) || previous?.activeCandidateId || "",
      candidates,
      activeProjectRevision: text(activeProject.project_revision) || text(rawSession.project_revision) || previous?.activeProjectRevision || "",
      projectRevision: text(rawSession.project_revision) || previous?.projectRevision || "",
      stateRevision: Number(rawSession.state_revision) || previous?.stateRevision || 0,
      lastEventType: text(event.type),
      error: text(payload.message) || text(payload.error) || (text(event.type) === "audition.failed" ? text(event.body) : ""),
      conversationID: text(rawSession.conversation_id) || text(event.conversation_id) || previous?.conversationID || "",
      // B9 统一面：判定卡锚定轮次域（source_turn_id=run）——与轨迹回合同键，
      // 卡片才能挂进统一轨迹块所在的回合组；缺失回退会话原生 turn 域。
      turnID: text(event.source_turn_id) || text(rawSession.turn_id) || text(payload.turn_id) || previous?.turnID || "",
      roundID: text(rawSession.round_id) || text(payload.round_id) || previous?.roundID || "",
      judgmentRequested: event.type === "trajectory.user_judgment.recorded" ? false : previous?.judgmentRequested || event.type === "trajectory.user_judgment.requested",
      judgmentRecorded: previous?.judgmentRecorded || event.type === "trajectory.user_judgment.recorded",
      judgmentEvidence: previous?.judgmentEvidence || (event.type === "trajectory.user_judgment.recorded" ? record(record(payload.details).evidence) : null),
      blind: flag(rawSession.blind) || previous?.blind === true,
      // 解盲只可能来自判定落账后的披露：audition.blind_disclosure 事件（payload 顶层）
      // 或恢复快照里的同名字段。判定前两者都不存在。
      blindDisclosure: firstRecord(payload.blind_disclosure, rawSession.blind_disclosure, previous?.blindDisclosure),
      inspectedCandidateId: text(rawSession.inspected_candidate_id) || previous?.inspectedCandidateId || "",
      adoptedCandidateId: text(rawSession.adopted_candidate_id) || previous?.adoptedCandidateId || "",
      adoptionStatus: text(rawSession.adoption_status) || previous?.adoptionStatus || "",
      inspectionReceipt: Object.keys(record(rawSession.inspection_receipt)).length > 0 ? record(rawSession.inspection_receipt) : previous?.inspectionReceipt || null,
      adoptionReceipt: Object.keys(record(rawSession.adoption_receipt)).length > 0 ? record(rawSession.adoption_receipt) : previous?.adoptionReceipt || null
    };
    seen.add(key);
    next.eventKeys.push(key);
  });
  return next;
}

export function auditionSessions(state: AuditionState): AuditionSession[] {
  return Object.values(state.sessions);
}

export function auditionCanSelect(session: AuditionSession, candidate: AuditionCandidate): boolean {
  // AUDITION-UNSTICK-1（2026-09-14）：stopped 会话放行选择——点击即重启播放。
  // 内核 audition.select 拒 stopped（audition_not_ready），服务端在 select 前
  // 用同参 audition.prepare 把会话重落座回 ready 再 select（既有内核命令，
  // 协议与状态机契约不动）；盲态物理指派随候选原样重建不被重抽。
  // stale/failed 是终态，仍拒。
  return (session.status === "ready" || session.status === "playing" || session.status === "stopped")
    && candidate.status === "ready" && Boolean(candidate.previewRef);
}

/**
 * AUDITION-UNSTICK-1：准备期判定——会话仍在 preparing，或任一候选还在
 * preparing（渲染/预热进行中）。此窗口 UI 必须显形「正在准备 A/B 试听…」，
 * 不能留一块无反馈的空白（用户读作执行轨迹计时停了=像死机）。
 */
export function auditionPreparing(session: AuditionSession): boolean {
  return session.status === "preparing" || session.candidates.some((candidate) => candidate.status === "preparing");
}

/**
 * AUDITION-UNSTICK-1：A/B 控件不可用时的显式原因（禁用不许是死点）。
 * 返回空串=控件可用，无需说明。
 */
export function auditionSelectBlockedReason(session: AuditionSession, busy: boolean, settled: boolean): string {
  if (settled) return "判定已落定，试听已结束";
  if (busy) return "正在处理你的上一次操作…";
  if (auditionPreparing(session)) return "正在准备 A/B 试听…";
  if (session.status === "stale" || session.status === "failed") return "试听会话已失效，无法再播放";
  if (!session.candidates.some((candidate) => candidate.status === "ready" && Boolean(candidate.previewRef))) {
    return "候选音频尚未就绪";
  }
  return "";
}

function candidateFromAny(value: unknown): AuditionCandidate {
  const row = record(value);
  return {
    id: text(row.id), label: text(row.label) || text(row.id), status: text(row.status), sourceRef: text(row.source_ref), previewRef: text(row.preview_ref),
    checkpointRef: text(row.checkpoint_ref), commitID: text(row.commit_id), branchRef: text(row.branch_ref), worktreeRef: text(row.worktree_ref),
    projectPath: text(row.project_path), projectRevision: text(row.project_revision)
  };
}
function record(value: unknown): JsonRecord { return value && typeof value === "object" && !Array.isArray(value) ? value as JsonRecord : {}; }
function firstRecord(...values: unknown[]): JsonRecord | null {
  for (const value of values) {
    const row = record(value);
    if (Object.keys(row).length > 0) return row;
  }
  return null;
}
function flag(value: unknown): boolean {
  if (value === true) return true;
  if (typeof value === "string") return ["1", "true", "yes", "on"].includes(value.trim().toLowerCase());
  return false;
}
function text(value: unknown): string { return typeof value === "string" ? value.trim() : value == null ? "" : String(value).trim(); }

export function auditionCanInspect(candidate: AuditionCandidate): boolean {
  return Boolean(candidate.projectPath && (candidate.commitID || candidate.checkpointRef || candidate.branchRef || candidate.worktreeRef));
}

export function auditionJudgmentPrefers(session: AuditionSession, candidateID: string): boolean {
  const preference = text(session.judgmentEvidence?.preference);
  return session.judgmentRecorded && ((candidateID === "candidate-a" && preference === "a") || (candidateID === "candidate-b" && preference === "b"));
}

export type AuditionSettlementOutcome = "improved" | "rolled_back" | "needs_user_judgment" | "";

// The D1 judgment settles the experiment directly (retain / rollback /
// ambiguous terminal); there is no separate apply step afterwards. The raw
// settlement outcome lives on the trajectory settlement node's details, not
// on the turn's evaluation outcome (which maps needs_user_judgment back to
// human_audition_ready).
export function auditionSettlementOutcome(trajectory: TrajectoryState, session: AuditionSession): AuditionSettlementOutcome {
  const turn = trajectory.turns[session.turnID];
  if (!turn) return "";
  for (let index = turn.nodeIds.length - 1; index >= 0; index -= 1) {
    const node = trajectory.nodes[turn.nodeIds[index]];
    if (!node || node.kind !== "settlement") continue;
    const outcome = text(node.details?.outcome);
    if (outcome === "improved" || outcome === "rolled_back" || outcome === "needs_user_judgment") {
      return outcome;
    }
  }
  return "";
}
