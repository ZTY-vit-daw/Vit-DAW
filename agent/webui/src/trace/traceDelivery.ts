import type { AgentEvent } from "../types";
import type { ChatMessage } from "../types";
import type { TrajectoryState } from "../trajectory";
import { trajectoryTurnIdOfEvent, type TurnEventMetaMap } from "./turnEventMeta";

// GUI-F7（2026-09-05 手测实锤）：调度链纯后台收尾的终局回复只经事件路径交付，
// 而 reduceAgentEventActivities 对 turn.completed 事件只清场不产消息——F6 在
// chatMessageFromAgentEvent 里的合成分支在轮询集成路径上不可达。本模块把
// scheduler_chain 终局事件的合成消息直接路由进 messages 列表（正式气泡），
// 不经活动列表（活动会被回合清退）。合成逻辑仍以 App 的 chatMessageFromAgentEvent
// 为单一来源，通过注入传入，避免 leaf 模块反向依赖 App。

export function chainResultMessagesFromEvents(
  events: AgentEvent[],
  mode: string | undefined,
  factory: (event: AgentEvent, mode?: string) => ChatMessage | null
): ChatMessage[] {
  const out: ChatMessage[] = [];
  const seen = new Set<string>();
  for (const event of events) {
    const payload = (event.payload ?? {}) as Record<string, unknown>;
    const type = String(event.type ?? "");
    // GUI-1：turn.stopped 终局补齐（CONTRACT-1 服务端已保证 stopped body 非空）
    const isChainTerminal = Boolean(payload.scheduler_chain) &&
      (type === "turn.completed" || type === "turn.failed" || type === "turn.stopped");
    if (!isChainTerminal) {
      continue;
    }
    const message = factory(event, mode);
    if (!message) {
      continue;
    }
    const key = message.id || message.source_id || "";
    if (key && seen.has(key)) {
      continue;
    }
    if (key) {
      seen.add(key);
    }
    out.push(message);
  }
  return out;
}

export function appendChainResultMessages(current: ChatMessage[], incoming: ChatMessage[]): ChatMessage[] {
  if (incoming.length === 0) {
    return current;
  }
  const existing = new Set(current.map((message) => message.id || message.source_id || ""));
  const additions = incoming.filter((message) => {
    const key = message.id || message.source_id || "";
    return key && !existing.has(key);
  });
  if (additions.length === 0) {
    return current;
  }
  return [...current, ...additions];
}

// F3 面②：链终局到达是"链已收尾"的最早权威信号。忙态（agentTurnRunning）派生
// 自 runtime status 的 goal/continuations 投影，只靠 8s 周期轮刷新时终局后停止
// 按钮最长滞留 8s（B1 手测"stop=done 后转圈"的呈现面放大器）；轮询器据此谓词
// 立即刷新 runtime status，忙态数秒内退场。只认 scheduler_chain 终局三型：
// HTTP 路径的 turn.completed 已由响应体交付并自带 refreshState，不重复触发。
export function hasChainTerminalDeliveryEvent(events: AgentEvent[]): boolean {
  for (const event of events ?? []) {
    const type = String(event?.type ?? "");
    if (type !== "turn.completed" && type !== "turn.failed" && type !== "turn.stopped") {
      continue;
    }
    const payload = (event.payload ?? {}) as Record<string, unknown>;
    if (Boolean(payload.scheduler_chain)) {
      return true;
    }
  }
  return false;
}

// GUI-F8：终局消息的渲染位置谓词——scheduler_chain 终局消息在消息流中移到
// 孤儿轨迹块（实验轮块）之后渲染：用户消息 → 中间汇报 → 实验轨迹块 → 终局回复。
export function isChainResultChatMessage(message: ChatMessage): boolean {
  const key = String(message?.source_id ?? message?.id ?? "");
  return key.startsWith("agent_event_") && key.endsWith("_chain_result");
}

// M12 谓词（GUI-1，2026-09-05 取证定案修复 F8 过宽隐藏）：F8 的「completed 且
// 无非 turn 节点即隐藏」把 item-only 观察轮（有活动、无轨迹节点投影、142s 生命
// 周期）同判为结算切片——回合完成后整个执行痕迹消失。修复后隐藏需要证据：
//
// ① 新事件流：服务端 turn_kind=settle_slice 标记（CONTRACT-1）叠加「无非 turn
//    节点 ∧ 无 item 活动足迹」→ 隐藏（标记是服务端权威，替代寿命猜测）；
// ② 旧事件流回退谓词三条件：无非 turn 节点 ∧ 无 item 活动记录 ∧ 生命周期短于
//    阈值 → 隐藏。缺任何一项证据即渲染；meta 缺失（证据不足）保守渲染。
// failed/stopped 与 live 回合照旧保留块。
//
// 注意链终局与主回合共享 run id（fixture 实证）：settle_slice 标记会落在有
// item 活动的观察轮终局事件上，故标记单独不足以隐藏，活动足迹证据始终优先。

export const SETTLE_SLICE_TURN_KIND = "settle_slice";
// 取证基准：真结算切片生命周期 ~128ms；item-only 观察轮 ~142s。2s 阈值两端均
// 有量级余量，仅用于旧事件流（无标记）的回退判定。
export const SETTLE_SLICE_MAX_LIFESPAN_MS = 2000;

export function shouldRenderTraceBlockForTurn(
  state: TrajectoryState,
  turnId: string,
  turnEventMeta?: TurnEventMetaMap
): boolean {
  const turn = state?.turns?.[turnId];
  if (!turn) {
    return false;
  }
  const status = String(turn.status ?? "").toLowerCase();
  if (status !== "completed") {
    return true;
  }
  const hasStepNodes = (turn.nodeIds ?? []).some((id) => {
    const node = state.nodes?.[id];
    return Boolean(node) && String(node.kind ?? "") !== "turn";
  });
  if (hasStepNodes) {
    return true;
  }
  const meta = turnEventMeta?.[turnId];
  // 条件②（item 活动足迹）：足迹未知按有活动处理——证据不足不隐藏
  if (!meta || meta.itemActivityCount > 0) {
    return true;
  }
  // 新流：服务端 settle_slice 标记为隐藏许可
  if (meta.turnKind === SETTLE_SLICE_TURN_KIND) {
    return false;
  }
  // 旧流回退条件③：生命周期短于阈值；时间戳缺失视为不满足
  if (meta.startedAt === undefined || meta.endedAt === undefined) {
    return true;
  }
  return meta.endedAt - meta.startedAt >= SETTLE_SLICE_MAX_LIFESPAN_MS;
}

/** 事件流中带 settle_slice 标记的轨迹归属 turn（诊断/测试用） */
export function settleSliceTurnIds(events: AgentEvent[]): string[] {
  const out: string[] = [];
  for (const event of events) {
    const payload = (event.payload ?? {}) as Record<string, unknown>;
    if (payload.turn_kind === SETTLE_SLICE_TURN_KIND) {
      const turnId = trajectoryTurnIdOfEvent(event);
      if (turnId && !out.includes(turnId)) {
        out.push(turnId);
      }
    }
  }
  return out;
}
