import type { ChatMessage } from "../types";

export interface MessageTurnGroup {
  key: string;
  /** 非空 = 该组绑定一个 trajectory turn（组内消息携带同一 turn_id） */
  turnId: string;
  messages: ChatMessage[];
}

/**
 * 把消息流按回合分组：携带 turn_id 的消息开启/并入该回合组；
 * 无 turn_id 的消息（如乐观发出的用户消息）自成独立组，保持原始顺序
 * 在后续回合组之前——渲染上即「用户气泡 → 该回合轨迹块 → 回合内卡片」。
 */
export function groupMessagesByTurn(messages: ChatMessage[]): MessageTurnGroup[] {
  const groups: MessageTurnGroup[] = [];
  const byTurn = new Map<string, MessageTurnGroup>();
  for (const message of messages) {
    const turnId = (message.turn_id ?? "").trim();
    if (turnId) {
      const existing = byTurn.get(turnId);
      if (existing) {
        existing.messages.push(message);
        continue;
      }
      const group: MessageTurnGroup = { key: `turn:${turnId}`, turnId, messages: [message] };
      byTurn.set(turnId, group);
      groups.push(group);
      continue;
    }
    const previous = groups[groups.length - 1];
    // 相邻的无 turn_id 消息并成同一独立组，避免碎片化
    if (previous && !previous.turnId) {
      previous.messages.push(message);
      continue;
    }
    groups.push({ key: `loose:${message.id}`, turnId: "", messages: [message] });
  }
  return groups;
}

/** 轨迹 turn 是否已被某个消息组承载（未承载的 turn 需追加渲染，避免丢 live 轨迹） */
export function turnIsAnchored(groups: MessageTurnGroup[], turnId: string): boolean {
  return groups.some((group) => group.turnId === turnId);
}

/** 活动是否属于非回合类（上传/调用等）——这些留在活动线，回合内活动并入轨迹思考行 */
export function isUnboundActivity(activity: ChatMessage, knownTurnIds: Set<string>): boolean {
  const turnId = (activity.turn_id ?? "").trim();
  return !turnId || !knownTurnIds.has(turnId);
}
