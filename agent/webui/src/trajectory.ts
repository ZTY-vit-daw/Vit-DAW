import type { AgentEvent, JsonRecord } from "./types";

export const trajectorySchemaVersion = "vit.observable_trajectory.v1" as const;

export type TrajectoryStatus = "pending" | "running" | "completed" | "stopped" | "failed" | "waiting_for_user";
export type TrajectoryEvaluation =
  | "not_ready"
  | "insufficient_dose"
  | "agent_evaluable"
  | "human_audition_ready"
  | "human_confirmed"
  | "ambiguous"
  | "unsupported_hypothesis"
  | "rolled_back";

export type TrajectoryNodeKind =
  | "turn"
  | "intent"
  | "observation"
  | "hypothesis"
  | "action"
  | "materiality"
  | "verification"
  | "decision"
  | "rollback"
  | "branch_or_worktree"
  | "user_judgment"
  | "settlement"
  | "error";

export interface TrajectoryPayload {
  schema_version?: string;
  trace_node_id?: string;
  parent_node_id?: string;
  turn_id?: string;
  round_id?: string;
  node_kind?: TrajectoryNodeKind | string;
  phase?: string;
  status?: TrajectoryStatus | string;
  summary?: string;
  evidence_refs?: string[];
  action_refs?: string[];
  project_revision?: string;
  checkpoint_ref?: string;
  branch_ref?: string;
  worktree_ref?: string;
  materiality?: TrajectoryEvaluation | string;
  target_response?: string;
  next_decision?: string;
  outcome?: TrajectoryEvaluation | string;
  details?: JsonRecord;
}

export interface TrajectoryEvent extends AgentEvent {
  type: `trajectory.${string}` | string;
  item_type?: "trajectory" | string;
  payload?: TrajectoryPayload & JsonRecord;
}

export interface TrajectoryNode {
  id: string;
  turnId: string;
  roundId: string;
  parentId: string;
  kind: TrajectoryNodeKind | string;
  phase: string;
  status: TrajectoryStatus | string;
  title: string;
  summary: string;
  createdAt: number;
  seq: number;
  materiality: TrajectoryEvaluation | string;
  targetResponse: string;
  outcome: TrajectoryEvaluation | string;
  nextDecision: string;
  evidenceRefs: string[];
  actionRefs: string[];
  projectRevision: string;
  checkpointRef: string;
  branchRef: string;
  worktreeRef: string;
  details: JsonRecord;
  eventType: string;
}

export interface TrajectoryRound {
  id: string;
  turnId: string;
  status: TrajectoryStatus | string;
  phase: string;
  nodeIds: string[];
  activeNodeId: string;
  evaluation: TrajectoryEvaluation | string;
  decision: string;
}

export interface TrajectoryTurn {
  id: string;
  status: TrajectoryStatus | string;
  phase: string;
  nodeIds: string[];
  roundIds: string[];
  activeNodeId: string;
  activeRoundId: string;
  outcome: TrajectoryEvaluation | string;
  stopped: boolean;
}

export interface TrajectoryState {
  schemaVersion: string;
  turns: Record<string, TrajectoryTurn>;
  rounds: Record<string, TrajectoryRound>;
  nodes: Record<string, TrajectoryNode>;
  eventKeys: string[];
  nextSeq: number;
}

export function trajectoryEventKey(event: AgentEvent): string {
  const payload = record(event.payload);
  const id = text(payload.trace_node_id) || text(event.item_id);
  const turn = text(payload.turn_id) || text(event.turn_id) || text(event.run_id) || text(event.goal_id);
  const seq = Number.isFinite(event.seq) ? String(event.seq) : "";
  return [turn, id, text(event.type), seq].filter(Boolean).join(":");
}

export function isTrajectoryEvent(event: AgentEvent): event is TrajectoryEvent {
  const payload = record(event.payload);
  return text(event.type).startsWith("trajectory.") && text(payload.schema_version) === trajectorySchemaVersion;
}

export function emptyTrajectoryState(): TrajectoryState {
  return {
    schemaVersion: trajectorySchemaVersion,
    turns: {},
    rounds: {},
    nodes: {},
    eventKeys: [],
    nextSeq: 0
  };
}

export function reduceTrajectoryEvents(
  current: TrajectoryState,
  incoming: AgentEvent[]
): TrajectoryState {
  let next: TrajectoryState = {
    ...current,
    schemaVersion: trajectorySchemaVersion,
    turns: { ...current.turns },
    rounds: { ...current.rounds },
    nodes: { ...current.nodes },
    eventKeys: [...current.eventKeys]
  };
  const seen = new Set(next.eventKeys);
  next.nextSeq = incoming.reduce((maximum, event) => Math.max(maximum, event.seq || 0), next.nextSeq);
  const events = incoming
    .filter(isTrajectoryEvent)
    .slice()
    .sort((left, right) => (left.seq || 0) - (right.seq || 0));

  for (const event of events) {
    const key = trajectoryEventKey(event);
    if (!key || seen.has(key)) {
      continue;
    }
    const payload = record(event.payload);
    // CONTRACT-1 C0 双读：轨迹归属优先读 trajectory_turn_id（双写期与 turn_id
    // 等值），缺失回退旧字段；payload.turn_id 仍是实验级事件的最优先归属。
    const turnId = text(payload.turn_id) || text(event.trajectory_turn_id) || text(event.turn_id) || text(event.run_id) || text(event.goal_id);
    if (!turnId) {
      continue;
    }
    const roundId = text(payload.round_id);
    const nodeId = text(payload.trace_node_id) || text(event.item_id) || key;
    const previousNode = next.nodes[nodeId];
    const status = text(payload.status) || text(event.status) || "completed";
    const node: TrajectoryNode = {
      id: nodeId,
      turnId,
      roundId,
      parentId: text(payload.parent_node_id),
      kind: text(payload.node_kind) || defaultNodeKind(text(event.type)),
      phase: text(payload.phase),
      status,
      title: text(event.title),
      summary: text(payload.summary) || text(event.body),
      createdAt: event.created_at ? Date.parse(event.created_at) || event.seq || Date.now() : event.seq || Date.now(),
      seq: event.seq || 0,
      materiality: text(payload.materiality),
      targetResponse: text(payload.target_response),
      outcome: text(payload.outcome),
      nextDecision: text(payload.next_decision),
      evidenceRefs: stringArray(payload.evidence_refs),
      actionRefs: stringArray(payload.action_refs),
      projectRevision: text(payload.project_revision),
      checkpointRef: text(payload.checkpoint_ref),
      branchRef: text(payload.branch_ref),
      worktreeRef: text(payload.worktree_ref),
      details: record(payload.details),
      eventType: text(event.type)
    };
    const mergedNode = previousNode ? mergeNode(previousNode, node) : node;
    next.nodes[nodeId] = mergedNode;
    const turn = next.turns[turnId] ?? emptyTurn(turnId);
    next.turns[turnId] = updateTurn(turn, mergedNode, text(event.type), next.nodes);
    if (roundId) {
      const round = next.rounds[roundId] ?? emptyRound(roundId, turnId);
      next.rounds[roundId] = updateRound(round, mergedNode, text(event.type), next.nodes);
    }
    seen.add(key);
    next.eventKeys.push(key);
    next.nextSeq = Math.max(next.nextSeq, event.seq || 0);
  }
  return next;
}

export function trajectoryTurns(state: TrajectoryState): TrajectoryTurn[] {
  return Object.values(state.turns).sort((left, right) => turnFirstSeq(state, left) - turnFirstSeq(state, right));
}

export function trajectoryRounds(state: TrajectoryState, turnId: string): TrajectoryRound[] {
  return Object.values(state.rounds)
    .filter((round) => round.turnId === turnId)
    .sort((left, right) => roundFirstSeq(state, left) - roundFirstSeq(state, right));
}

export function trajectoryNodesForRound(state: TrajectoryState, roundId: string): TrajectoryNode[] {
  return Object.values(state.nodes)
    .filter((node) => node.roundId === roundId)
    .sort((left, right) => left.seq - right.seq);
}

function emptyTurn(id: string): TrajectoryTurn {
  return { id, status: "pending", phase: "", nodeIds: [], roundIds: [], activeNodeId: "", activeRoundId: "", outcome: "", stopped: false };
}

function emptyRound(id: string, turnId: string): TrajectoryRound {
  return { id, turnId, status: "pending", phase: "", nodeIds: [], activeNodeId: "", evaluation: "", decision: "" };
}

function updateTurn(turn: TrajectoryTurn, node: TrajectoryNode, eventType: string, nodes: Record<string, TrajectoryNode>): TrajectoryTurn {
  const active = nodes[turn.activeNodeId];
  const isNewer = !active || node.seq >= active.seq;
  const stopped = turn.stopped || (isNewer && (eventType === "trajectory.turn.stopped" || node.status === "stopped"));
  const mayAdvance = isNewer && !turn.stopped;
  return {
    ...turn,
    status: stopped ? "stopped" : mayAdvance ? node.status || turn.status : turn.status,
    phase: stopped ? "stopped" : mayAdvance ? node.phase || turn.phase : turn.phase,
    nodeIds: appendUnique(turn.nodeIds, node.id),
    roundIds: node.roundId ? appendUnique(turn.roundIds, node.roundId) : turn.roundIds,
    activeNodeId: mayAdvance ? node.id : turn.activeNodeId,
    activeRoundId: mayAdvance ? node.roundId || turn.activeRoundId : turn.activeRoundId,
    outcome: mayAdvance ? node.outcome || (eventType === "trajectory.settled" ? node.materiality : turn.outcome) : turn.outcome,
    stopped
  };
}

function updateRound(round: TrajectoryRound, node: TrajectoryNode, eventType: string, nodes: Record<string, TrajectoryNode>): TrajectoryRound {
  const active = nodes[round.activeNodeId];
  const isNewer = !active || node.seq >= active.seq;
  return {
    ...round,
    status: isNewer ? node.status || round.status : round.status,
    phase: isNewer ? node.phase || round.phase : round.phase,
    nodeIds: appendUnique(round.nodeIds, node.id),
    activeNodeId: isNewer ? node.id : round.activeNodeId,
    evaluation: isNewer ? node.materiality || node.outcome || round.evaluation : round.evaluation,
    decision: isNewer ? node.nextDecision || (eventType === "trajectory.round.decision" ? node.summary : round.decision) : round.decision
  };
}

function mergeNode(existing: TrajectoryNode, incoming: TrajectoryNode): TrajectoryNode {
  const newer = incoming.seq >= existing.seq ? incoming : existing;
  const older = newer === incoming ? existing : incoming;
  return {
    ...newer,
    createdAt: Math.min(existing.createdAt, incoming.createdAt),
    seq: Math.max(existing.seq, incoming.seq),
    title: newer.title || older.title,
    summary: newer.summary || older.summary,
    phase: newer.phase || older.phase,
    materiality: newer.materiality || older.materiality,
    targetResponse: newer.targetResponse || older.targetResponse,
    outcome: newer.outcome || older.outcome,
    nextDecision: newer.nextDecision || older.nextDecision,
    evidenceRefs: appendMany(existing.evidenceRefs, incoming.evidenceRefs),
    actionRefs: appendMany(existing.actionRefs, incoming.actionRefs),
    projectRevision: newer.projectRevision || older.projectRevision,
    checkpointRef: newer.checkpointRef || older.checkpointRef,
    branchRef: newer.branchRef || older.branchRef,
    worktreeRef: newer.worktreeRef || older.worktreeRef,
    details: { ...older.details, ...newer.details }
  };
}

function turnFirstSeq(state: TrajectoryState, turn: TrajectoryTurn): number {
  return Math.min(...turn.nodeIds.map((id) => state.nodes[id]?.seq ?? Number.MAX_SAFE_INTEGER), Number.MAX_SAFE_INTEGER);
}

function roundFirstSeq(state: TrajectoryState, round: TrajectoryRound): number {
  return Math.min(...round.nodeIds.map((id) => state.nodes[id]?.seq ?? Number.MAX_SAFE_INTEGER), Number.MAX_SAFE_INTEGER);
}

function defaultNodeKind(type: string): string {
  const suffix = type.replace(/^trajectory\./, "");
  if (suffix.includes("intent")) return "intent";
  if (suffix.includes("observation")) return "observation";
  if (suffix.includes("hypothesis")) return "hypothesis";
  if (suffix.includes("materiality")) return "materiality";
  if (suffix.includes("intervention")) return "action";
  if (suffix.includes("rollback")) return "rollback";
  if (suffix.includes("judgment")) return "user_judgment";
  if (suffix.includes("settled")) return "settlement";
  if (suffix.includes("error")) return "error";
  if (suffix.includes("turn")) return "turn";
  return "decision";
}

function record(value: unknown): JsonRecord {
  return value && typeof value === "object" && !Array.isArray(value) ? value as JsonRecord : {};
}

function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : value === undefined || value === null ? "" : String(value).trim();
}

function stringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.map(text).filter(Boolean) : [];
}

function appendUnique(current: string[], value: string): string[] {
  return value && !current.includes(value) ? [...current, value] : current;
}

function appendMany(current: string[], values: string[]): string[] {
  return values.reduce(appendUnique, current);
}
