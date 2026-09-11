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
  /** B9 统一面：轮次键来自 source_turn_id（CONTRACT-1 C0 消息归属域）的回合。
   * 轮次域回合的终态只由 trajectory.turn.completed/failed/stopped 收口——
   * 中途到达的完成态步节点不提前把回合收成回执（用户裁定：一轮对话一个
   * 轨迹块，终局并入原块，下次输入才出新块）。旧流（无 source 字段）保持
   * 按最新 seq 节点状态推进的既有语义。 */
  roundScoped: boolean;
  /** 轮次域回合的终局事件状态（空 = 轮次仍开放） */
  terminalStatus: string;
  terminalPhase: string;
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

/** B9 统一面（轮次键）：一轮对话一个轨迹块。事件的轮次归属优先读
 * source_turn_id——服务端对 run 级与实验级（free_state 域）事件统一回填拥有
 * 它的 run（CONTRACT-1 C0 消息归属域），因此同一轮的 run 壳与实验轨迹节点
 * 归并到同一轮次键下；缺失时回退既有轨迹归属链（旧流行为不变）。 */
export function trajectoryRoundKeyOfEvent(event: AgentEvent): string {
  const payload = record(event.payload);
  return (
    text(event.source_turn_id) ||
    text(payload.turn_id) ||
    text(event.trajectory_turn_id) ||
    text(event.turn_id) ||
    text(event.run_id) ||
    text(event.goal_id)
  );
}

/** 轮次域回合里是否仍有 running/pending 节点（壳或步）——轮次开放中 */
export function hasLiveTrajectoryTurn(state: TrajectoryState): boolean {
  return Object.values(state.turns).some((turn) => turn.status === "running" || turn.status === "pending");
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
    // B9 统一面：轮次键优先 source_turn_id（run 域），同轮的 run 壳与实验
    // （free_state 域）轨迹节点归并为一个轨迹回合；旧流回退既有归属链。
    const turnId = trajectoryRoundKeyOfEvent(event);
    if (!turnId) {
      continue;
    }
    const roundScoped = Boolean(text(event.source_turn_id));
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
    next.turns[turnId] = updateTurn(turn, mergedNode, text(event.type), next.nodes, roundScoped);
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
  return { id, status: "pending", phase: "", nodeIds: [], roundIds: [], activeNodeId: "", activeRoundId: "", outcome: "", stopped: false, roundScoped: false, terminalStatus: "", terminalPhase: "" };
}

function emptyRound(id: string, turnId: string): TrajectoryRound {
  return { id, turnId, status: "pending", phase: "", nodeIds: [], activeNodeId: "", evaluation: "", decision: "" };
}

function updateTurn(turn: TrajectoryTurn, node: TrajectoryNode, eventType: string, nodes: Record<string, TrajectoryNode>, roundScoped = false): TrajectoryTurn {
  const active = nodes[turn.activeNodeId];
  const isNewer = !active || node.seq >= active.seq;
  const stopped = turn.stopped || (isNewer && (eventType === "trajectory.turn.stopped" || node.status === "stopped"));
  const mayAdvance = isNewer && !turn.stopped;
  const scoped = turn.roundScoped || roundScoped;
  const terminalEventType = eventType === "trajectory.turn.completed" || eventType === "trajectory.turn.failed" || eventType === "trajectory.turn.stopped";
  // 终局事件粘性捕获（不因当时尚未置 scoped 而漏记；仅在轮次域分支消费）
  const terminalStatus = turn.terminalStatus || (terminalEventType ? node.status || "completed" : "");
  const terminalPhase = turn.terminalStatus
    ? turn.terminalPhase
    : terminalEventType
      ? node.phase || node.status || "completed"
      : "";
  let status: string;
  let phase: string;
  if (stopped) {
    status = "stopped";
    phase = "stopped";
  } else if (scoped) {
    if (terminalStatus) {
      // 终局并入原块：轮次只由 trajectory.turn 终态事件收口
      status = terminalStatus;
      phase = terminalPhase;
    } else {
      // 轮次开放中：仍有 running/pending 节点（壳或步）即保持直播；全完成
      // 节点按最新 seq 推进（终态事件未到前不虚报，也不提前收成回执）
      const hasLiveNode = turn.nodeIds.concat([node.id]).some((id) => {
        const candidate = id === node.id ? node : nodes[id];
        return Boolean(candidate) && (candidate.status === "running" || candidate.status === "pending");
      });
      status = hasLiveNode ? "running" : mayAdvance ? node.status || turn.status : turn.status;
      phase = hasLiveNode ? turn.phase || "framing" : mayAdvance ? node.phase || turn.phase : turn.phase;
    }
  } else {
    status = mayAdvance ? node.status || turn.status : turn.status;
    phase = mayAdvance ? node.phase || turn.phase : turn.phase;
  }
  return {
    ...turn,
    roundScoped: scoped,
    terminalStatus,
    terminalPhase,
    status,
    phase,
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
