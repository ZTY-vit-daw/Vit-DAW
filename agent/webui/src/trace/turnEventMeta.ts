import type { AgentEvent, JsonRecord } from "../types";

// GUI-1（M12 修复的证据层）：item 活动在回合结束后会被 reduceAgentEventActivities
// 清退、且 item 从不投影为轨迹节点（2026-09-05 取证定案），所以「这个回合发生过
// 什么」在完成后只剩事件流本身可考。本归约器从原始事件流为每个轨迹 turn 记账：
// item 活动足迹、生命周期时间、服务端 turn_kind 标记（CONTRACT-1 settle_slice），
// 供 shouldRenderTraceBlockForTurn 的回退谓词消费。纯函数、增量归约、按
// logical_message_id/item_id 去重（轮询重复投递不虚增）。

export interface TurnEventMeta {
  /** 服务端 turn_kind 标记（如 settle_slice）；无标记为空串 */
  turnKind: string;
  /** 该 turn 内去重后的 item 活动条数 */
  itemActivityCount: number;
  itemActivityKeys: string[];
  startedAt?: number;
  /**
   * 工作片终点：turn.completed / turn.failed（及其轨迹孪生）的 created_at。
   * 这是「执行时长」的终点，不是回合在时间轴上的最后一点。
   */
  endedAt?: number;
  /**
   * 驻留终点：turn.stopped / trajectory.turn.stopped 的 created_at。
   *
   * CONT-STALL-1（2026-09-12 22:53 真栈，goal_5b9cb1a9e48ace5b）：切片在
   * 22:54:06 以 waiting_continue 结束并在驻留里空了 203.5 s，用户 22:57:30
   * 手动停止才补上 trajectory.turn.stopped。该事件与 turn.completed 同属一个
   * turn（source_turn_id 相同），此前被并进 endedAt 取 max，于是
   * endedAt-startedAt = 258.3 s 被当成执行时长呈现（「258 秒执行记录」），
   * 203 s 的驻留等待被计成工作。驻留是等待，不是执行：工作片终点与驻留终点
   * 必须分开记账，消费侧才能「执行 55s，等待续跑 203s」地分行呈现。
   */
  residencyEndedAt?: number;
}

/**
 * 工作/驻留时长拆分（唯一真源）。口径：
 *  - 有工作终局：workMs = endedAt - startedAt；驻留终点更晚时
 *    parkMs = residencyEndedAt - endedAt。
 *  - 无工作终局但有驻留终点（真·运行中被停止）：整段到停止为止都是工作，
 *    parkMs = null。
 *  - 两者皆无：无法记账，两个字段都是 null（不虚报 0）。
 */
export interface TurnDurationSplit {
  workMs: number | null;
  parkMs: number | null;
}

export function turnDurationSplit(meta?: TurnEventMeta): TurnDurationSplit {
  if (!meta || meta.startedAt === undefined) {
    return { workMs: null, parkMs: null };
  }
  const started = meta.startedAt;
  if (meta.endedAt !== undefined) {
    const workMs = Math.max(0, meta.endedAt - started);
    if (meta.residencyEndedAt !== undefined && meta.residencyEndedAt > meta.endedAt) {
      return { workMs, parkMs: meta.residencyEndedAt - meta.endedAt };
    }
    return { workMs, parkMs: null };
  }
  if (meta.residencyEndedAt !== undefined) {
    return { workMs: Math.max(0, meta.residencyEndedAt - started), parkMs: null };
  }
  return { workMs: null, parkMs: null };
}

export type TurnEventMetaMap = Record<string, TurnEventMeta>;

/**
 * 轨迹归属键（B9 统一面）：优先 source_turn_id（轮次域=拥有该事件的 run，
 * CONTRACT-1 C0 消息归属域）——与 reduceTrajectoryEvents 的轮次键对齐，
 * 保证 meta 记账（活动足迹/turn_kind/生命周期）与轨迹回合同键。缺失回退
 * trajectory_turn_id / payload.turn_id / 旧 turn_id / run_id / goal_id（旧流
 * 行为不变）。
 */
export function trajectoryTurnIdOfEvent(event: AgentEvent): string {
  const payload = event.payload as JsonRecord | undefined;
  const payloadTurnID = payload && typeof payload === "object" ? text(payload.turn_id) : "";
  return (
    text(event.source_turn_id) ||
    payloadTurnID ||
    text(event.trajectory_turn_id) ||
    text(event.turn_id) ||
    text(event.run_id) ||
    text(event.goal_id)
  );
}

export function emptyTurnEventMetaMap(): TurnEventMetaMap {
  return {};
}

export function reduceTurnEventMeta(current: TurnEventMetaMap, events: AgentEvent[]): TurnEventMetaMap {
  let next: TurnEventMetaMap | null = null;
  for (const event of events) {
    const turnId = trajectoryTurnIdOfEvent(event);
    if (!turnId) {
      continue;
    }
    const existing = (next ?? current)[turnId];
    let meta = existing ?? { turnKind: "", itemActivityCount: 0, itemActivityKeys: [] };
    let changed = !existing;
    const type = text(event.type);
    if (type === "item.started" || type === "item.completed" || type === "item.failed") {
      const key = activityFootprintKey(event);
      if (key && !meta.itemActivityKeys.includes(key)) {
        meta = { ...meta, itemActivityKeys: [...meta.itemActivityKeys, key], itemActivityCount: meta.itemActivityCount + 1 };
        changed = true;
      }
    }
    const turnKind = payloadText(event.payload, "turn_kind");
    if (turnKind && turnKind !== meta.turnKind) {
      meta = { ...meta, turnKind };
      changed = true;
    }
    if (type === "turn.started" || type === "trajectory.turn.started") {
      const at = eventCreatedAt(event);
      if (at !== undefined && at !== meta.startedAt) {
        meta = { ...meta, startedAt: meta.startedAt !== undefined ? Math.min(meta.startedAt, at) : at };
        changed = true;
      }
    }
    if (
      type === "turn.completed" || type === "turn.failed" ||
      type === "trajectory.turn.completed" || type === "trajectory.turn.failed"
    ) {
      const at = eventCreatedAt(event);
      if (at !== undefined && at !== meta.endedAt) {
        meta = { ...meta, endedAt: meta.endedAt !== undefined ? Math.max(meta.endedAt, at) : at };
        changed = true;
      }
    }
    // 驻留终局单独记账：turn.stopped 收尾的是一段等待，不是一片工作。
    if (type === "turn.stopped" || type === "trajectory.turn.stopped") {
      const at = eventCreatedAt(event);
      if (at !== undefined && at !== meta.residencyEndedAt) {
        meta = { ...meta, residencyEndedAt: meta.residencyEndedAt !== undefined ? Math.max(meta.residencyEndedAt, at) : at };
        changed = true;
      }
    }
    if (changed) {
      if (!next) {
        next = { ...current };
      }
      next[turnId] = meta;
    }
  }
  return next ?? current;
}

function activityFootprintKey(event: AgentEvent): string {
  const logical = text(event.logical_message_id);
  if (logical) {
    return logical;
  }
  return text(event.item_id) || `${text(event.type)}:${Number(event.seq) || 0}`;
}

function eventCreatedAt(event: AgentEvent): number | undefined {
  const raw = text(event.created_at);
  const at = raw ? Date.parse(raw) : NaN;
  return Number.isFinite(at) ? at : undefined;
}

function payloadText(payload: unknown, key: string): string {
  if (!payload || typeof payload !== "object" || Array.isArray(payload)) {
    return "";
  }
  return text((payload as JsonRecord)[key]);
}

function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : value === undefined || value === null ? "" : String(value).trim();
}
