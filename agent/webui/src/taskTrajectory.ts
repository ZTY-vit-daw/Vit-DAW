import type { JsonRecord, TaskRuntimeTrajectory } from "./types";

export const taskRuntimeTrajectorySchema = "vit.task_runtime_trajectory.v1" as const;

export interface TaskTrajectorySnapshot {
  schemaVersion: string;
  task: JsonRecord;
  run: JsonRecord;
  semantic: JsonRecord;
  transitions: JsonRecord[];
  continuation: JsonRecord;
  capabilityRoute: JsonRecord;
  identity: string;
  semanticRevision: number;
  updatedAt: number;
}

export interface TaskTrajectoryState {
  snapshot: TaskTrajectorySnapshot | null;
}

export function emptyTaskTrajectoryState(): TaskTrajectoryState {
  return { snapshot: null };
}

// Runtime snapshots are replay-safe: an older semantic revision, or an older
// snapshot for the same revision, must never replace what the UI already saw.
export function reduceTaskTrajectory(
  current: TaskTrajectoryState,
  incoming?: TaskRuntimeTrajectory | JsonRecord | null
): TaskTrajectoryState {
  const next = normalizeTaskTrajectory(incoming);
  if (!next) return current;
  const previous = current.snapshot;
  if (previous && previous.identity === next.identity) {
    if (next.semanticRevision < previous.semanticRevision) return current;
    if (next.semanticRevision === previous.semanticRevision && next.updatedAt < previous.updatedAt) return current;
  }
  return { snapshot: next };
}

// WEBUI-SESSION-SEMANTICS-1：PlanBar 数据源（/agent/runtime/status 的 task_trajectory
// 全局投影）按 active session 绑定——投影 task.conversation_id 与当前会话不符（note
// 会话任务、其他 webui 流任务）时即时清空，不得把异会话的执行轨迹渲染到本会话。
// 载荷缺位（runtime 空闲无任务投影）不算异会话证据，保留现状交给既有 replay-safe
// 归约与 [conversationID] 清场效应。
export function reduceTaskTrajectoryForConversation(
  current: TaskTrajectoryState,
  incoming: TaskRuntimeTrajectory | JsonRecord | null | undefined,
  conversationID: string
): TaskTrajectoryState {
  const active = conversationID.trim();
  if (!active) {
    return emptyTaskTrajectoryState();
  }
  const next = normalizeTaskTrajectory(incoming);
  if (!next) {
    return current;
  }
  if (text(next.task.conversation_id) !== active) {
    return emptyTaskTrajectoryState();
  }
  // 归约吃原始载荷（normalizeTaskTrajectory 只认 schema_version 蛇形键，已归一化
  // 对象二次归一化会得 null——原始输入上幂等，monotonic 规则保持单一出处）。
  return reduceTaskTrajectory(current, incoming);
}

export function normalizeTaskTrajectory(input?: TaskRuntimeTrajectory | JsonRecord | null): TaskTrajectorySnapshot | null {
  const root = record(input);
  if (text(root.schema_version) !== taskRuntimeTrajectorySchema) return null;
  const task = record(root.task);
  const taskID = text(task.task_id);
  const goalID = text(task.goal_id);
  const runID = text(task.run_id);
  if (!taskID || !goalID || !runID) return null;
  const semantic = record(root.semantic);
  return {
    schemaVersion: taskRuntimeTrajectorySchema,
    task,
    run: record(root.run),
    semantic,
    transitions: records(root.transitions),
    continuation: record(root.continuation),
    capabilityRoute: record(root.capability_route),
    identity: `${taskID}:${goalID}:${runID}`,
    semanticRevision: number(semantic.revision),
    updatedAt: time(task.updated_at) || time(semantic.updated_at)
  };
}

export function record(value: unknown): JsonRecord {
  return value && typeof value === "object" && !Array.isArray(value) ? value as JsonRecord : {};
}

export function records(value: unknown): JsonRecord[] {
  return Array.isArray(value) ? value.map(record).filter((item) => Object.keys(item).length > 0) : [];
}

export function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : value === undefined || value === null ? "" : String(value).trim();
}

export function strings(value: unknown): string[] {
  return Array.isArray(value) ? value.map(text).filter(Boolean) : [];
}

export function number(value: unknown): number {
  const parsed = typeof value === "number" ? value : Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

function time(value: unknown): number {
  const parsed = Date.parse(text(value));
  return Number.isFinite(parsed) ? parsed : 0;
}

