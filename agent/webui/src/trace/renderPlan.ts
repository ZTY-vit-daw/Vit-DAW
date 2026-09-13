import type { ChatMessage } from "../types";
import type { TrajectoryState } from "../trajectory";
import { trajectoryTurns } from "../trajectory";
import { groupMessagesByTurn, type MessageTurnGroup } from "./turnGroups";
import { isChainResultChatMessage, shouldRenderTraceBlockForTurn } from "./traceDelivery";
import type { TurnEventMetaMap } from "./turnEventMeta";

// GUI-1/G2（渲染计划抽取）：MessageStream 的「回合分组 + 轨迹块锚定 + 孤儿块 +
// 终局消息归位」从 JSX 里抽为纯函数，固化目标顺序（GUI-F8 定案）：
//
//   用户消息 → 中间汇报 → 回合轨迹块（锚定）→ … → 实验轨迹块（孤儿）→ 终局回复
//
// 计划只描述「渲染什么、按什么顺序」，不持有 React 状态；判定卡/乐观条/活动线
// 等交互态渲染仍留在 MessageStream 组件，按各自既有位置插入。
//
// UI-FOLLOW-1（2026-09-12 用户产品裁定）：轨迹块与状态行跟随对话——轨迹块是
// **每回合消息的附属组件**（回合内实时更新、回合结束定格为该回合的终态行），
// 只有确实「没有回合附属位」时才允许流尾兜底。缺陷形态（真栈手测命中，用户
// 截图实证）：轨迹回合的身份键与消息携带的 turn_id 常不在同一命名空间（轮次域
// run_*/turn:free_state_* 对 chat 的 turn_* 域；失败链更可能只有一条不带任何
// turn_id 的乐观用户消息），身份匹配失败时旧实现把轨迹块一律追加到条目序列尾
// ——「执行完成 N 步」这一行连着它的步数轨迹坠到整条对话流最底部、被输入框浮层
// 压住，既不随新回合下移、也不属于任何回合。因此锚定分两级：
//
//   ① 身份锚定（既有语义，逐字保留）：消息组的 turn_id === 轨迹回合 id；
//   ② 回合槽位锚定（UI-FOLLOW-1 新增）：以用户消息为界切出的对话回合槽位，
//      取「起始时刻不晚于该回合起始的最后一个用户消息」所在组。用户消息是回合
//      的唯一开启者（乐观消息先于该回合 turn.started 落地），因此这是身份不同源
//      时唯一可判定的归属证据。回合起始时刻只认 turnEventMeta（turn.started /
//      trajectory.turn.started 的 created_at）——**证据不足时不猜**，保持流尾
//      兜底（无 meta 的旧调用点行为逐字不变）。
//
//   一个渲染组只承载一个轨迹块（B9「一轮对话一个轨迹块」不变）；同一槽位出现
//   多个候选回合时（B9 之前的双域形态），最早开始的回合占位，其余保持流尾兜底。

export type MessageStreamEntry =
  | { kind: "messages"; key: string; messages: ChatMessage[] }
  | { kind: "trace"; key: string; turnId: string };

export interface MessageStreamRenderPlan {
  /** 有序渲染条目（不含流尾终局回复——见 chainResultMessages） */
  entries: MessageStreamEntry[];
  /** 未能挂进任何回合组、追加渲染在条目序列尾的轨迹回合 */
  orphanTurnIds: string[];
  /** 调度链终局消息：渲染在整个条目序列（含孤儿块）之后 */
  chainResultMessages: ChatMessage[];
}

/** 对话回合槽位：一个用户消息 = 一个回合开启点（锚定位落在该消息的渲染组上） */
interface MessageRoundSlot {
  /** 该用户消息所在渲染组的键——轨迹块插到这一组的用户条目之后 */
  groupKey: string;
  /** 用户消息落库时刻（客户端时钟；agent 与本机同源，回合事件时间与之可直接比较） */
  startedAt: number;
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

  // 渲染哪些回合：谓词逐字沿用（M12 item 活动足迹 / settle_slice 标记证据链）
  const renderTurns = trajectoryTurns(trajectory).filter(
    (turn) => turn.nodeIds.length > 0 && shouldRenderTraceBlockForTurn(trajectory, turn.id, turnEventMeta)
  );

  // ① 身份锚定：消息组的 turn_id 命中轨迹回合 id
  const turnIdByGroup = new Map<string, string>();
  const anchoredTurnIds = new Set<string>();
  for (const group of groups) {
    if (!group.turnId) {
      continue;
    }
    const turn = renderTurns.find((candidate) => candidate.id === group.turnId);
    if (turn && !anchoredTurnIds.has(turn.id)) {
      turnIdByGroup.set(group.key, turn.id);
      anchoredTurnIds.add(turn.id);
    }
  }

  // ② 回合槽位锚定：身份不同源时按「回合起始时刻所属的对话回合」入位
  const slots = messageRoundSlots(groups);
  const occupiedGroupKeys = new Set(turnIdByGroup.keys());
  const orphanTurnIds: string[] = [];
  for (const turn of renderTurns) {
    if (anchoredTurnIds.has(turn.id)) {
      continue;
    }
    const groupKey = anchorGroupKeyForTurn(slots, turnStartedAtMs(turnEventMeta, turn.id));
    if (!groupKey || occupiedGroupKeys.has(groupKey)) {
      orphanTurnIds.push(turn.id);
      continue;
    }
    turnIdByGroup.set(groupKey, turn.id);
    occupiedGroupKeys.add(groupKey);
  }

  const entries: MessageStreamEntry[] = [];
  for (const group of groups) {
    const userMessages = group.messages.filter((message) => message.role === "user");
    if (userMessages.length > 0) {
      entries.push({ kind: "messages", key: `${group.key}:user`, messages: userMessages });
    }
    const turnId = turnIdByGroup.get(group.key);
    if (turnId) {
      entries.push({ kind: "trace", key: `${group.key}:trace`, turnId });
    }
    const turnMessages = group.messages.filter((message) => message.role !== "user");
    if (turnMessages.length > 0) {
      entries.push({ kind: "messages", key: `${group.key}:rest`, messages: turnMessages });
    }
  }

  // 兜底：确实没有回合附属位的轨迹块挂流尾防丢（无起始时刻证据 / 槽位已占）
  for (const turnId of orphanTurnIds) {
    entries.push({ kind: "trace", key: `orphan:${turnId}`, turnId });
  }

  return { entries, orphanTurnIds, chainResultMessages };
}

/** 对话回合槽位：用户消息是回合的唯一开启者（相邻无 turn_id 消息并组语义不变） */
function messageRoundSlots(groups: MessageTurnGroup[]): MessageRoundSlot[] {
  const slots: MessageRoundSlot[] = [];
  for (const group of groups) {
    for (const message of group.messages) {
      if (message.role !== "user") {
        continue;
      }
      const startedAt = Number(message.createdAt);
      if (Number.isFinite(startedAt)) {
        slots.push({ groupKey: group.key, startedAt });
      }
    }
  }
  return slots;
}

/** 轨迹回合起始时刻（唯一证据源：turnEventMeta.startedAt；缺证据返回 NaN） */
function turnStartedAtMs(turnEventMeta: TurnEventMetaMap | undefined, turnId: string): number {
  const startedAt = turnEventMeta?.[turnId]?.startedAt;
  return typeof startedAt === "number" && Number.isFinite(startedAt) ? startedAt : Number.NaN;
}

/** 回合槽位锚定：起始时刻之前的最后一个用户消息所在组（无证据/无槽位 → 空串） */
function anchorGroupKeyForTurn(slots: MessageRoundSlot[], startedAt: number): string {
  if (!Number.isFinite(startedAt)) {
    return "";
  }
  let groupKey = "";
  for (const slot of slots) {
    if (slot.startedAt <= startedAt) {
      groupKey = slot.groupKey;
    }
  }
  return groupKey;
}
