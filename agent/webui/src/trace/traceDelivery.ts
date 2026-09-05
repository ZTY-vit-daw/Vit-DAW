import type { AgentEvent } from "../types";
import type { ChatMessage } from "../types";
import type { TrajectoryState } from "../trajectory";

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
    const isChainTerminal = Boolean(payload.scheduler_chain) && (type === "turn.completed" || type === "turn.failed");
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

// 空结算块隐藏（GUI-F7 / 原 A 方案）：0 轨迹节点且终态 completed 的回合不再
// 渲染独立轨迹块——调度链收尾切片（128ms、无工具调用）的"0 步·执行完成"
// 块是误导，终局结果由链终局消息气泡承载。failed/stopped 与 live 回合保留
// 块：异常与进行中信息优先可见。
export function shouldRenderTraceBlockForTurn(state: TrajectoryState, turnId: string): boolean {
  const turn = state?.turns?.[turnId];
  if (!turn) {
    return false;
  }
  const status = String(turn.status ?? "").toLowerCase();
  if (status !== "completed") {
    return true;
  }
  const hasNodes = (turn.nodeIds ?? []).some((id) => Boolean(state.nodes?.[id]));
  return hasNodes;
}
