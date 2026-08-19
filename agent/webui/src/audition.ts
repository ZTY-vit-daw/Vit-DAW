import type { AgentEvent, JsonRecord } from "./types";

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
      turnID: text(rawSession.turn_id) || text(payload.turn_id) || previous?.turnID || "",
      roundID: text(rawSession.round_id) || text(payload.round_id) || previous?.roundID || "",
      judgmentRequested: event.type === "trajectory.user_judgment.recorded" ? false : previous?.judgmentRequested || event.type === "trajectory.user_judgment.requested",
      judgmentRecorded: previous?.judgmentRecorded || event.type === "trajectory.user_judgment.recorded",
      judgmentEvidence: previous?.judgmentEvidence || (event.type === "trajectory.user_judgment.recorded" ? record(record(payload.details).evidence) : null),
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
  return (session.status === "ready" || session.status === "playing") && candidate.status === "ready" && Boolean(candidate.previewRef);
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
function text(value: unknown): string { return typeof value === "string" ? value.trim() : value == null ? "" : String(value).trim(); }

export function auditionCanInspect(candidate: AuditionCandidate): boolean {
  return Boolean(candidate.projectPath && (candidate.commitID || candidate.checkpointRef || candidate.branchRef || candidate.worktreeRef));
}

export function auditionJudgmentPrefers(session: AuditionSession, candidateID: string): boolean {
  const preference = text(session.judgmentEvidence?.preference);
  return session.judgmentRecorded && ((candidateID === "candidate-a" && preference === "a") || (candidateID === "candidate-b" && preference === "b"));
}
