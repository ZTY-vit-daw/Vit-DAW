import type { ChatMessage } from "../types";
import type { TrajectoryState } from "../trajectory";
import { trajectoryTurns } from "../trajectory";
import { groupMessagesByTurn, turnIsAnchored } from "./turnGroups";
import { isChainResultChatMessage, shouldRenderTraceBlockForTurn } from "./traceDelivery";
import type { TurnEventMetaMap } from "./turnEventMeta";

// GUI-1/G2（渲染计划抽取）：MessageStream 的「回合分组 + 轨迹块锚定 + 孤儿块 +
// 终局消息归位」从 JSX 里抽为纯函数，固化目标顺序（GUI-F8 定案）：
//
//   用户消息 → 中间汇报 → 回合轨迹块（锚定）→ … → 实验轨迹块（孤儿）→ 终局回复
//
// 计划只描述「渲染什么、按什么顺序」，不持有 React 状态；判定卡/乐观条/活动线
// 等交互态渲染仍留在 MessageStream 组件，按各自既有位置插入。

export type MessageStreamEntry =
  | { kind: "messages"; key: string; messages: ChatMessage[] }
  | { kind: "trace"; key: string; turnId: string };

export interface MessageStreamRenderPlan {
  /** 有序渲染条目（不含流尾终局回复——见 chainResultMessages） */
  entries: MessageStreamEntry[];
  /** 未被消息组锚定、追加渲染在条目序列尾的轨迹回合 */
  orphanTurnIds: string[];
  /** 调度链终局消息：渲染在整个条目序列（含孤儿块）之后 */
  chainResultMessages: ChatMessage[];
}

export function buildMessageStreamRenderPlan(options: {
  messages: ChatMessage[];
  trajectory: TrajectoryState;
  turnEventMeta?: TurnEventMetaMap;
}): MessageStreamRenderPlan {
  const { messages, trajectory, turnEventMeta } = options;
  const chainResultMessages = messages.filter(isChainResultChatMessage);
  const flowMessages = chainResultMessages.length > 0 ? messages.filter((message) => !isChainResultChatMessage(message)) : messages;
  const groups = groupMessagesByTurn(flowMessages);

  const entries: MessageStreamEntry[] = [];
  for (const group of groups) {
    const userMessages = group.messages.filter((message) => message.role === "user");
    if (userMessages.length > 0) {
      entries.push({ kind: "messages", key: `${group.key}:user`, messages: userMessages });
    }
    if (group.turnId && shouldRenderTraceBlockForTurn(trajectory, group.turnId, turnEventMeta)) {
      entries.push({ kind: "trace", key: `${group.key}:trace`, turnId: group.turnId });
    }
    const turnMessages = group.messages.filter((message) => message.role !== "user");
    if (turnMessages.length > 0) {
      entries.push({ kind: "messages", key: `${group.key}:rest`, messages: turnMessages });
    }
  }

  // 轨迹事件先于回合消息到达时，轨迹块挂流尾防丢；同样受 M12 谓词约束
  const orphanTurnIds = trajectoryTurns(trajectory)
    .filter((turn) => turn.nodeIds.length > 0 && !turnIsAnchored(groups, turn.id) && shouldRenderTraceBlockForTurn(trajectory, turn.id, turnEventMeta))
    .map((turn) => turn.id);
  for (const turnId of orphanTurnIds) {
    entries.push({ kind: "trace", key: `orphan:${turnId}`, turnId });
  }

  return { entries, orphanTurnIds, chainResultMessages };
}
