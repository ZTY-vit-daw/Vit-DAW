import type { AgentEvent, JsonRecord } from "./types";

export const auditionSchemaVersion = "vit.kernel_audition.v1";

export interface AuditionCandidate {
  id: string;
  label: string;
  status: string;
  sourceRef: string;
  previewRef: string;
}

export interface AuditionSession {
  id: string;
  status: string;
  activeCandidateId: string;
  candidates: AuditionCandidate[];
  activeProjectRevision: string;
  stateRevision: number;
  lastEventType: string;
  error: string;
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
  incoming.filter(isAuditionEvent).sort((a, b) => (a.seq || 0) - (b.seq || 0)).forEach((event) => {
    const payload = record(event.payload);
    const rawSession = record(payload.session);
    const id = text(rawSession.session_id) || text(event.item_id);
    if (!id) return;
    const key = `${id}:${text(event.type)}:${event.seq || 0}`;
    if (seen.has(key)) return;
    const previous = next.sessions[id];
    const activeProject = record(rawSession.active_project_plane);
    const candidates = Array.isArray(rawSession.candidates)
      ? rawSession.candidates.map(candidateFromAny).filter((candidate) => candidate.id)
      : previous?.candidates ?? [];
    next.sessions[id] = {
      id,
      status: text(rawSession.status) || text(event.status) || previous?.status || "preparing",
      activeCandidateId: text(rawSession.active_candidate_id) || previous?.activeCandidateId || "",
      candidates,
      activeProjectRevision: text(activeProject.project_revision) || previous?.activeProjectRevision || "",
      stateRevision: Number(rawSession.state_revision) || previous?.stateRevision || 0,
      lastEventType: text(event.type),
      error: text(payload.message) || text(payload.error) || (text(event.type) === "audition.failed" ? text(event.body) : "")
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
  return { id: text(row.id), label: text(row.label) || text(row.id), status: text(row.status), sourceRef: text(row.source_ref), previewRef: text(row.preview_ref) };
}
function record(value: unknown): JsonRecord { return value && typeof value === "object" && !Array.isArray(value) ? value as JsonRecord : {}; }
function text(value: unknown): string { return typeof value === "string" ? value.trim() : value == null ? "" : String(value).trim(); }
