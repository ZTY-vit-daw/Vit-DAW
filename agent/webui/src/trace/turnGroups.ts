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

/**
 * 渲染序列中最后一个「会渲染轨迹块」的回合 id——任务详情锚定位（GUI-F2）。
 * chat 响应的 turn_id（turn_ 域）与 trajectory 事件的 turn id（run_ 域）可能不同：
 * 只有存在于 trajectory.turns 的组 turnId 才渲染块，其余组（如 chat 回复组）不入序列。
 */
export function latestRenderedTurnId(groups: MessageTurnGroup[], knownTurnIds: Set<string>, orphanTurnIds: string[]): string {
  const sequence = [
    ...groups.flatMap((group) => (group.turnId && knownTurnIds.has(group.turnId) ? [group.turnId] : [])),
    ...orphanTurnIds
  ];
  return sequence[sequence.length - 1] ?? "";
}

/** 活动是否属于非回合类（上传/调用等）——这些留在活动线，回合内活动并入轨迹思考行 */
export function isUnboundActivity(
  activity: ChatMessage,
  knownTurnIds: Set<string>,
  roundBoundKeys?: Set<string>
): boolean {
  const turnId = (activity.turn_id ?? "").trim();
  if (turnId && knownTurnIds.has(turnId)) {
    return false;
  }
  // TRAJ-IMPL-2（设计 §2.1-5 活动线去重）：已并入回合块的 item 活动不再进流底活动线。
  // 跨命名空间时活动的 turn_id（chat 域 run_/turn_ 混写）判不出归属——收口判据因此换成
  // **事件身份**（逻辑消息 id / 活动 id，roundActivityBoundKeys 产出），与「这个 item 步
  // 已经落在某个回合容器里」是同一件事。无回合归属的活动（上传等）照旧留在 lane。
  if (roundBoundKeys && roundBoundKeys.size > 0) {
    for (const key of [activity.logical_message_id, activity.id, activity.source_id]) {
      const value = (key ?? "").trim();
      if (value && roundBoundKeys.has(value)) {
        return false;
      }
    }
  }
  return true;
}

/**
 * FIX-AUDITION-TRAIL-1（2026-09-29 M1 手测缺陷①取证定性）：audition 族活动行的
 * 身份判定。内核遥测（audition.prepare.started / audition.candidate.ready /
 * audition.ready）的会话快照不带 turn 域（内核 audition::Session 无该字段，
 * 2026-09-05 fixture seq27/28 实证），在 agent 侧 enrich 事件（同会话首个带
 * turn 域者，mount 完成后才发）到达前以未绑定身份落流底活动线；族内每个
 * eventType 各占一条逻辑消息（audition:{sid}:{type}），upsert 不折叠——处理期
 * 输出内容底下出现多条「Kernel audition」重复行（M1 手测原述）。AUDITION-LANE-1
 * 的裁定是**该族呈现面=判定卡**（每会话一卡、随状态更新、判定后定型），此处按
 * 身份（而非回合绑定）排除族内一切活动行，判定卡即「至多一条」的单表面。
 */
export function isAuditionFamilyActivity(activity: ChatMessage): boolean {
  const logical = (activity.logical_message_id ?? "").trim();
  if (logical.startsWith("audition:")) {
    return true;
  }
  const source = (activity.source_id ?? activity.id ?? "").trim();
  return source.includes("agent_event_goal_audition:");
}

/** 流底活动线的可见集合（App 渲染唯一消费口）：未并入回合/会话域，且不属于 audition 族 */
export function laneVisibleActivities(
  activities: ChatMessage[],
  laneBoundTurnIds: Set<string>,
  roundBoundKeys?: Set<string>
): ChatMessage[] {
  return activities.filter(
    (activity) => isUnboundActivity(activity, laneBoundTurnIds, roundBoundKeys) && !isAuditionFamilyActivity(activity)
  );
}
