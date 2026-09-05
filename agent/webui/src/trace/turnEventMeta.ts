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
  endedAt?: number;
}

export type TurnEventMetaMap = Record<string, TurnEventMeta>;

/**
 * 轨迹归属键（CONTRACT-1 C0 双读期）：优先 trajectory_turn_id，缺失回退
 * 旧 turn_id / run_id / goal_id（双写期 trajectory_turn_id 与 turn_id 等值，
 * 显式优先读新字段是为收窄期铺轨）。payload.turn_id 仍最优先——实验级事件
 * 的轨迹归属写在 payload 内。
 */
export function trajectoryTurnIdOfEvent(event: AgentEvent): string {
  const payload = event.payload as JsonRecord | undefined;
  const payloadTurnID = payload && typeof payload === "object" ? text(payload.turn_id) : "";
  return text(payloadTurnID) || text(event.trajectory_turn_id) || text(event.turn_id) || text(event.run_id) || text(event.goal_id);
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
      type === "turn.completed" || type === "turn.failed" || type === "turn.stopped" ||
      type === "trajectory.turn.completed" || type === "trajectory.turn.failed" || type === "trajectory.turn.stopped"
    ) {
      const at = eventCreatedAt(event);
      if (at !== undefined && at !== meta.endedAt) {
        meta = { ...meta, endedAt: meta.endedAt !== undefined ? Math.max(meta.endedAt, at) : at };
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
