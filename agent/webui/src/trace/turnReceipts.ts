import type { TrajectoryState, TrajectoryTurn } from "../trajectory";
import { emptyTrajectoryState, trajectoryTurns } from "../trajectory";
import {
  roundStepTurnIds,
  roundTurnStatus,
  shouldRenderRoundContainer,
  type RoundStepMap,
  type RoundStepTurn
} from "./roundSteps";
import { shouldRenderTraceBlockForTurn } from "./traceDelivery";
import { turnDurationSplit, type TurnEventMeta, type TurnEventMetaMap } from "./turnEventMeta";

// TRAJ-IMPL-3（设计 docs/TRAJECTORY_PRESENTATION_REDESIGN_V1.md §2.3 分期一② + §7 裁定 C，
// 2026-09-14 定稿）：**终局回执行台账**——「持久的是摘要，不是活态」（DSH 原则 3）的落地件。
//
// 问题③根因（F3+F5）：回合块消费的全是 transient 事件派生态（协议把 trajectory 事件定为
// persistence=none），刷新后 traceIndexes 为空——块整块消失，因为拿活态当了持久层。
// 修法：把**终局摘要**（回合键/状态/步数/活动数/执行时长/等待时长/started_at）落进
// localStorage 台账 `vit.turn_receipts.v1`，水合路径为「活态中不存在」的回合渲染
// 静态收起回执行（槽位锚定复用 UI-FOLLOW-1/2 两级锚定，见 renderPlan）。
//
// 三条纪律（卡面约束）：
//   ① **只在终态收口路径写入**：写入判据复用 TRAJ-IMPL-2 的 roundTurnStatus（设计 §4
//      收口双轨规则）——waiting_*（切片边界）算没结束，live 一律不写；
//   ② **只承载状态/计数/时长，不复述回复正文**（正文归气泡）；行字段是白名单（见
//      serializeTurnReceiptLedger），序列化结构上就没有正文位；
//   ③ schema 带版本号、未知字段容错读；未知版本 / 未知状态枚举 **fail-closed**
//      （读不出就不猜，宁可不显回执行也不虚报）。
//
// 丢失边界（设计 §2.3 明文声明）：换浏览器/清缓存台账即丢，最终回复本来就在气泡里
// （Project History 水合），丢的只是「这回合干了多久几步」的回执行；分期二把回执行落
// 服务端 durable 节点根治。**本卡不做任何服务端改动**。

/** 台账 schema 版本（同时是 localStorage 键前缀；键上带版本，换代不误读旧行） */
export const TURN_RECEIPTS_SCHEMA_VERSION = "vit.turn_receipts.v1";
export const TURN_RECEIPTS_STORAGE_PREFIX = "vit.turn_receipts.v1";

/**
 * 每 conversation+scope 的滚动上限（设计 §2.3「bounded ~200 条滚动」）：
 * 超出即丢最旧（插入序 = 收口序），台账不得无界增长。
 */
export const TURN_RECEIPTS_LIMIT = 200;

/** 回执行状态（只认这三种终态；其余一律不写行） */
export type TurnReceiptStatus = "completed" | "failed" | "stopped";

/**
 * 一行终局回执行。字段即卡面契约：回合键/状态/步数/活动数/执行时长/等待时长/started_at。
 * 时长口径：workMs/parkMs 取 turnDurationSplit（CONT-STALL-1 工作/驻留拆分唯一真源），
 * null = 无证据（不虚报 0）。
 */
export interface TurnReceipt {
  turnId: string;
  status: TurnReceiptStatus;
  stepCount: number;
  activityCount: number;
  workMs: number | null;
  parkMs: number | null;
  /** 回合起始时刻（epoch ms，与 UI-FOLLOW 槽位锚定同域）；0 = 无证据（只走身份锚定） */
  startedAt: number;
}

export interface TurnReceiptLedger {
  schemaVersion: string;
  conversationId: string;
  scope: string;
  savedAt: string;
  receipts: TurnReceipt[];
}

/** 台账键：per conversation + scope（与消息存档同一条「会话×工作区」分桶语义） */
export function turnReceiptsStorageKey(conversationId: string, scope: string): string {
  return `${TURN_RECEIPTS_STORAGE_PREFIX}:${encodeURIComponent(conversationId || "latest")}:${encodeURIComponent(scope)}`;
}

/** 显示用状态文案（与 TraceBlock.turnStatusLabel 的终态分支同词，live 分支不在此列） */
export function turnReceiptStatusLabel(status: TurnReceiptStatus): string {
  switch (status) {
    case "failed":
      return "执行失败";
    case "stopped":
      return "已停止";
    default:
      return "执行完成";
  }
}

/**
 * 行序列化（字段白名单）：正文/标题/摘要都没有位置——「不复述正文」是结构事实，
 * 不是约定。未知扩展字段在这一步被丢弃（读侧容错、写侧不留垃圾）。
 */
export function serializeTurnReceiptLedger(ledger: TurnReceiptLedger): string {
  return JSON.stringify({
    schema_version: TURN_RECEIPTS_SCHEMA_VERSION,
    conversation_id: ledger.conversationId,
    scope: ledger.scope,
    saved_at: ledger.savedAt,
    receipts: ledger.receipts.slice(-TURN_RECEIPTS_LIMIT).map((row) => ({
      turn_id: row.turnId,
      status: row.status,
      step_count: row.stepCount,
      activity_count: row.activityCount,
      work_ms: row.workMs,
      park_ms: row.parkMs,
      started_at: row.startedAt
    }))
  });
}

/**
 * 反序列化（容错读）：
 *  - 未知**字段**：忽略（行只按白名单键读，多余键不参与判定）；
 *  - 未知**版本**（schema_version 不是本代）：fail-closed 返回 null——不拿未来的行当本代读；
 *  - 未知**状态枚举**：丢该行（宁可不显这一回合的回执行，也不把它标成别的状态）；
 *  - 损坏 JSON / 缺字段：丢该行或整表，绝不抛（台账是缓存，读坏不得影响消息面）。
 */
export function parseTurnReceiptLedger(raw: string | null | undefined): TurnReceiptLedger | null {
  if (!raw) {
    return null;
  }
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return null;
  }
  const record = asRecord(parsed);
  const version = text(record.schema_version);
  if (!version || !version.startsWith(TURN_RECEIPTS_STORAGE_PREFIX)) {
    return null;
  }
  const rows: TurnReceipt[] = [];
  for (const value of Array.isArray(record.receipts) ? record.receipts : []) {
    const row = parseTurnReceiptRow(value);
    if (row) {
      rows.push(row);
    }
  }
  return {
    schemaVersion: version,
    conversationId: text(record.conversation_id),
    scope: text(record.scope),
    savedAt: text(record.saved_at),
    receipts: rows.slice(-TURN_RECEIPTS_LIMIT)
  };
}

/**
 * 台账合并（幂等 + bounded 滚动）：
 *  - **同键 = 刷新同一行**（一个回合一行的去重；回合重复收口不新开行）；
 *  - 行序 = 插入序（= 收口序）；超出 TURN_RECEIPTS_LIMIT 丢最旧。
 */
export function mergeTurnReceipts(
  current: TurnReceipt[],
  incoming: TurnReceipt[],
  limit: number = TURN_RECEIPTS_LIMIT
): TurnReceipt[] {
  if (incoming.length === 0) {
    return current;
  }
  const byTurn = new Map<string, TurnReceipt>();
  for (const row of current) {
    if (row.turnId) {
      byTurn.set(row.turnId, row);
    }
  }
  for (const row of incoming) {
    if (row.turnId) {
      byTurn.set(row.turnId, row);
    }
  }
  const rows = [...byTurn.values()];
  return rows.length > limit ? rows.slice(rows.length - limit) : rows;
}

/** 两表逐行同比（写入方据此跳过无变化的 localStorage 写） */
export function turnReceiptsEqual(left: TurnReceipt[], right: TurnReceipt[]): boolean {
  if (left === right) {
    return true;
  }
  if (left.length !== right.length) {
    return false;
  }
  for (let index = 0; index < left.length; index += 1) {
    const a = left[index];
    const b = right[index];
    if (
      a.turnId !== b.turnId ||
      a.status !== b.status ||
      a.stepCount !== b.stepCount ||
      a.activityCount !== b.activityCount ||
      a.workMs !== b.workMs ||
      a.parkMs !== b.parkMs ||
      a.startedAt !== b.startedAt
    ) {
      return false;
    }
  }
  return true;
}

/** 读取台账（无 window / 键缺失 / 读坏一律空表：不编造回执行） */
export function loadTurnReceipts(conversationId: string, scope: string): TurnReceipt[] {
  if (typeof window === "undefined" || !conversationId || !scope) {
    return [];
  }
  try {
    return parseTurnReceiptLedger(window.localStorage.getItem(turnReceiptsStorageKey(conversationId, scope)))?.receipts ?? [];
  } catch {
    // 隐私模式/配额异常：台账是尽力而为的本地缓存。
    return [];
  }
}

/** 落盘（写侧白名单序列化；失败静默——台账丢不等于消息丢） */
export function persistTurnReceipts(conversationId: string, scope: string, receipts: TurnReceipt[]): void {
  if (typeof window === "undefined" || !conversationId || !scope) {
    return;
  }
  try {
    window.localStorage.setItem(
      turnReceiptsStorageKey(conversationId, scope),
      serializeTurnReceiptLedger({
        schemaVersion: TURN_RECEIPTS_SCHEMA_VERSION,
        conversationId,
        scope,
        savedAt: new Date().toISOString(),
        receipts
      })
    );
  } catch {
    // 同上：Project History 仍是正文的权威，台账只是回执行的本地缓存。
  }
}

/** 读—并—写（scope 迁移与测试用；返回合并后的表） */
export function saveTurnReceipts(conversationId: string, scope: string, incoming: TurnReceipt[]): TurnReceipt[] {
  const current = loadTurnReceipts(conversationId, scope);
  const merged = mergeTurnReceipts(current, incoming);
  if (!turnReceiptsEqual(current, merged)) {
    persistTurnReceipts(conversationId, scope, merged);
  }
  return merged;
}

/**
 * 活态回合键（trajectory 回合 ∪ item 步账回合）：水合收据的**同键去重**依据——
 * 活态存在同键回合时活态优先，收据不重复渲染（设计 §2.3/§6.3）。
 */
export function liveTurnIdsOf(trajectory: TrajectoryState | undefined, roundSteps: RoundStepMap | undefined): Set<string> {
  const ids = new Set<string>();
  for (const turn of trajectoryTurns(trajectory ?? emptyTrajectoryState())) {
    if (turn.id) {
      ids.add(turn.id);
    }
  }
  for (const turnId of roundStepTurnIds(roundSteps)) {
    if (turnId) {
      ids.add(turnId);
    }
  }
  return ids;
}

/** 水合集：活态中**不存在**的回合才渲染收据行（刚刷新、事件还没到时全部渲染） */
export function receiptsForHydration(options: { receipts?: TurnReceipt[]; liveTurnIds: Set<string> }): TurnReceipt[] {
  const rows = options.receipts ?? [];
  if (rows.length === 0) {
    return [];
  }
  return rows.filter((row) => Boolean(row.turnId) && !options.liveTurnIds.has(row.turnId));
}

/**
 * 单个回合的回执行（**终态才产行**；live / 切片边界 / 证据不足一律 null）。
 *
 * 收口规则（设计 §4 双轨）：
 *  - 有 item 步账的回合：roundTurnStatus 说了算（waiting_* → running → 不写）；
 *  - 只有轨迹记录的回合：轨迹回合状态说了算（running/pending/waiting_for_user → 不写）。
 * 状态解析不到已知终态时 fail-closed 返回 null（不猜）。
 */
export function turnReceiptForTurn(options: {
  turnId: string;
  round?: RoundStepTurn;
  meta?: TurnEventMeta;
  trajectoryStatus?: string;
  trajectoryStepCount?: number;
}): TurnReceipt | null {
  const turnId = text(options.turnId);
  if (!turnId) {
    return null;
  }
  const status = terminalStatusOf(options);
  if (!status) {
    return null;
  }
  const split = turnDurationSplit(options.meta);
  const startedAt = nonNegativeOrNull(options.meta?.startedAt) ?? nonNegativeOrNull(options.round?.startedAt) ?? 0;
  return {
    turnId,
    status,
    stepCount: Math.max(0, options.trajectoryStepCount ?? 0) + Math.max(0, options.round?.totalStepCount ?? 0),
    activityCount: Math.max(0, options.meta?.itemActivityCount ?? 0),
    workMs: split.workMs,
    parkMs: split.parkMs,
    startedAt
  };
}

/**
 * 当前活态里**所有已收口**回合的回执行（写入方的输入）。
 *
 * 两个来源与 renderPlan.renderTurnCandidates 同序同据——**可见性谓词逐字复用**
 * （轨迹回合走 shouldRenderTraceBlockForTurn 的 M12 证据链，item 回合走
 * shouldRenderRoundContainer 的同一链）：隐藏的回合不得经台账在刷新后复活，
 * 否则 settle_slice 结算切片噪音会绕开 M12 从水合路径回来（设计 §6.1 风险）。
 */
export function collectTerminalTurnReceipts(options: {
  trajectory: TrajectoryState;
  turnEventMeta?: TurnEventMetaMap;
  roundSteps?: RoundStepMap;
}): TurnReceipt[] {
  const { trajectory, turnEventMeta, roundSteps } = options;
  const receipts: TurnReceipt[] = [];
  for (const turn of trajectoryTurns(trajectory)) {
    if (turn.nodeIds.length === 0) {
      // 无轨迹节点的回合由下面的 item 步账来源裁定（与渲染候选同序）。
      continue;
    }
    if (!shouldRenderTraceBlockForTurn(trajectory, turn.id, turnEventMeta)) {
      continue;
    }
    const receipt = turnReceiptForTurn({
      turnId: turn.id,
      round: roundSteps?.[turn.id],
      meta: turnEventMeta?.[turn.id],
      trajectoryStatus: turn.status,
      trajectoryStepCount: trajectoryStepNodeCount(trajectory, turn)
    });
    if (receipt) {
      receipts.push(receipt);
    }
  }
  for (const turnId of roundStepTurnIds(roundSteps)) {
    if (trajectory.turns?.[turnId]) {
      // 有轨迹回合记录的回合由上面按 M12 谓词裁定（含 settle_slice 隐藏），不重复候选。
      continue;
    }
    const round = roundSteps?.[turnId];
    if (!shouldRenderRoundContainer({ round, meta: turnEventMeta?.[turnId] })) {
      continue;
    }
    const receipt = turnReceiptForTurn({ turnId, round, meta: turnEventMeta?.[turnId] });
    if (receipt) {
      receipts.push(receipt);
    }
  }
  return receipts;
}

/** 回合内轨迹步节点数（kind=turn 的壳不算步；与 TraceBlock.turnStepNodes 同口径） */
function trajectoryStepNodeCount(state: TrajectoryState, turn: TrajectoryTurn): number {
  let count = 0;
  for (const nodeId of turn.nodeIds ?? []) {
    const node = state.nodes?.[nodeId];
    if (node && text(node.kind) !== "turn") {
      count += 1;
    }
  }
  return count;
}

function terminalStatusOf(options: {
  round?: RoundStepTurn;
  trajectoryStatus?: string;
}): TurnReceiptStatus | null {
  if (options.round) {
    return knownTerminalStatus(roundTurnStatus(options.round));
  }
  return knownTerminalStatus(text(options.trajectoryStatus).toLowerCase());
}

function knownTerminalStatus(value: string): TurnReceiptStatus | null {
  return value === "completed" || value === "failed" || value === "stopped" ? value : null;
}

function parseTurnReceiptRow(value: unknown): TurnReceipt | null {
  const row = asRecord(value);
  const turnId = text(row.turn_id) || text(row.turnId);
  if (!turnId) {
    return null;
  }
  const status = text(row.status).toLowerCase();
  if (status !== "completed" && status !== "failed" && status !== "stopped") {
    return null;
  }
  return {
    turnId,
    status,
    stepCount: countOrZero(row.step_count ?? row.stepCount),
    activityCount: countOrZero(row.activity_count ?? row.activityCount),
    workMs: nonNegativeOrNull(row.work_ms ?? row.workMs),
    parkMs: nonNegativeOrNull(row.park_ms ?? row.parkMs),
    startedAt: nonNegativeOrNull(row.started_at ?? row.startedAt) ?? 0
  };
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
}

function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : value === undefined || value === null ? "" : String(value).trim();
}

function countOrZero(value: unknown): number {
  const parsed = nonNegativeOrNull(value);
  return parsed === null ? 0 : Math.floor(parsed);
}

function nonNegativeOrNull(value: unknown): number | null {
  const parsed = typeof value === "number" ? value : typeof value === "string" && value.trim() !== "" ? Number(value) : Number.NaN;
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : null;
}
