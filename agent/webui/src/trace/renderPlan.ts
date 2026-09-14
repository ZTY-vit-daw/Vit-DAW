import type { ChatMessage } from "../types";
import type { TrajectoryState, TrajectoryTurn } from "../trajectory";
import { trajectoryTurns } from "../trajectory";
import { groupMessagesByTurn, type MessageTurnGroup } from "./turnGroups";
import { isChainResultChatMessage, shouldRenderTraceBlockForTurn } from "./traceDelivery";
import { roundStartSeq, roundStepTurnIds, shouldRenderRoundContainer, type RoundStepMap } from "./roundSteps";
import type { TurnEventMetaMap } from "./turnEventMeta";
import { liveTurnIdsOf, receiptsForHydration, type TurnReceipt } from "./turnReceipts";

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
//
// UI-FOLLOW-2（2026-09-13 真栈水合实测，E2E-WEBUI-1 渲染面红 + 归档会话复算）：
// 上面两条判据在真栈数据上各有一个独立缺陷，都会产出同一个用户症状「块坠到
// 对话流最底」：
//
//   ① **水合消息时刻比回合起始晚 2 ms**：用户消息的 createdAt 走 Project History
//      水合路径（服务端图节点盖章，实测 2026-09-13T04:26:17.1423557Z），而同回合
//      的 turnEventMeta.startedAt 来自事件流（trajectory.turn.started，
//      2026-09-13T04:26:17.1403545Z）。严格 `<= ` 判据因此把该回合**自己**的开启
//      用户消息排除在候选之外：单回合会话退化成零候选 → 流尾孤儿块；多回合会话
//      静默往前滑一格（挂到上一个回合的用户消息下）。容差
//      TURN_SLOT_ANCHOR_TOLERANCE_MS 只吸收「同一回合两条人机同源时间戳的盖章
//      先后差」，不把下一回合的用户消息拉进来。
//   ② **同一 run_id 承载多条用户消息**：真栈实测 run_bab2dcdacb41bdc1 同时挂在
//      12:26:17 / 12:27:15 / 12:27:50 三条 ask 节点上（同一 goal 的连续追问共用
//      run）。groupMessagesByTurn 把它们并成一个渲染组（B9 语义不变），但旧实现
//      把块放在「组内全部用户消息之后」——组内的后两条追问因此被排到块上方，
//      块连带坠到整条对话流的最后一条用户消息之下（DOM 实证：块 flowIndex 11，
//      最后一条用户消息在 10）。修法是**组内槽位细分**：块只跟组内开启该回合的
//      那条用户消息，组内其余用户消息（以及组内汇报）落到块之后。
//
// 两条修法都不放松既有语义：身份锚定仍然优先、无证据仍不猜（回落流尾）、
// M12 谓词与 B9「一组一块」不变。
//
// TRAJ-IMPL-2（2026-09-14，设计 §2.1-2 问题①可见性错绑）：上面全部修法都建立在
// 「该回合确实有一个轨迹块要渲染」之上，而旧谓词把**容器的存在性**绑在实验准入
// 的产物上（trajectory 回合 + 非空节点）——纯 chat 回合执行 31s、item.* 明明在场，
// 却连容器都没有，进度只能落在流底活动线（F2）。本卡把谓词放宽为「回合有任一活动
// 证据即有容器」：轨迹壳节点 / 轨迹步 / item 步 / turnEventMeta.startedAt 任一条
// 成立即出块；实验轨迹步降格为步来源之一。M12 settle_slice 隐藏谓词
// （shouldRenderTraceBlockForTurn 整段）**逐字保留、零改动**——放宽不得让结算切片
// 噪音回归（验收含「settle_slice 仍隐藏」钉）。
//
// TRAJ-IMPL-3（2026-09-14，设计 §2.3 分期一② 问题③活态/水合不一致 + §7 裁定 C）：
// 上面两级锚定解决的是「活态块落在哪」，而刷新后活态本身不存在（trajectory 事件是
// transient，重载后 traceIndexes 空）——块整块消失。本卡把**终局回执行台账**
// （trace/turnReceipts.ts，localStorage `vit.turn_receipts.v1`）里的行当作同一类
// 候选：活态中不存在的回合渲染一行**静态收起回执行**（kind="receipt"）。
//
// 三条不变量：
//   ① **落位复用同一条两级锚定**（身份锚定优先 → 槽位时刻 + 50ms 容差），不收据另造规则；
//   ② **同键活态优先**：回合键在活态（trajectory.turns ∪ roundSteps）里存在时不渲染收据行
//      （去重在 receiptsForHydration 入口完成，见 trace/turnReceipts.ts）；
//   ③ **一个渲染组只承载一个条目**（B9）：活态块候选先占组，收据抢不到组时退流尾
//      （orphanReceiptTurnIds）——不丢行、不挤掉活态。

export type MessageStreamEntry =
  | { kind: "messages"; key: string; messages: ChatMessage[] }
  | { kind: "trace"; key: string; turnId: string }
  // TRAJ-IMPL-3 §2.3：活态缺失回合的静态收起回执行（水合路径产物）
  | { kind: "receipt"; key: string; turnId: string; receipt: TurnReceipt };

export interface MessageStreamRenderPlan {
  /** 有序渲染条目（不含流尾终局回复——见 chainResultMessages） */
  entries: MessageStreamEntry[];
  /** 未能挂进任何回合组、追加渲染在条目序列尾的轨迹回合 */
  orphanTurnIds: string[];
  /** 同上，但落位的是水合收据行（TRAJ-IMPL-3）：与轨迹块分开记账，孤儿轨迹语义不变 */
  orphanReceiptTurnIds: string[];
  /** 调度链终局消息：渲染在整个条目序列（含孤儿块）之后 */
  chainResultMessages: ChatMessage[];
}

/**
 * 回合槽位锚定的时钟容差（UI-FOLLOW-2，2026-09-13 真栈实测 +2 ms）。
 *
 * 同一回合的两条时间戳来自同一 agent 进程但不同盖章点：回合起始取事件流
 * （trajectory.turn.started.created_at），用户消息取会话图节点
 * （graph node created_at，比前者晚约 2 ms）。50 ms 是「同一回合内盖章先后差」
 * 的量级（实测 2 ms，留两个数量级余量），远小于两回合之间的间隔（用户输入
 * 间隔为秒级），因此不会把下一回合的用户消息误判成本回合的开启消息。
 */
export const TURN_SLOT_ANCHOR_TOLERANCE_MS = 50;

/** 对话回合槽位：一个用户消息 = 一个回合开启点（锚定位落在该消息的渲染位置上） */
interface MessageRoundSlot {
  /** 该用户消息所在渲染组的键——轨迹块插到这条消息之后 */
  groupKey: string;
  /** 该用户消息在组内「用户消息」序列中的序号（0 = 组内第一条用户消息） */
  userIndex: number;
  /** 用户消息落库时刻（服务端图节点盖章；agent 与本机同源，可与回合事件时间直接比较） */
  startedAt: number;
}

/** 轨迹块落位：渲染组 + 组内用户消息序号（-1 = 组内无用户消息，块落在组首） */
interface MessageRoundAnchor {
  groupKey: string;
  userIndex: number;
  turnId: string;
}

export function buildMessageStreamRenderPlan(options: {
  messages: ChatMessage[];
  trajectory: TrajectoryState;
  turnEventMeta?: TurnEventMetaMap;
  /** TRAJ-IMPL-2：回合内 item 步账（谓词放宽的第二个证据源；缺省时行为逐字不变） */
  roundSteps?: RoundStepMap;
  /**
   * TRAJ-IMPL-3 §2.3：终局回执行台账行（刷新后活态缺失回合的静态收起回执行）。
   * 缺省/空表时行为逐字不变；同键活态优先由 receiptsForHydration 在入口处去重。
   */
  receipts?: TurnReceipt[];
}): MessageStreamRenderPlan {
  const { messages, trajectory, turnEventMeta, roundSteps, receipts } = options;
  const chainResultMessages = messages.filter(isChainResultChatMessage);
  const flowMessages = chainResultMessages.length > 0 ? messages.filter((message) => !isChainResultChatMessage(message)) : messages;
  const groups = groupMessagesByTurn(flowMessages);

  // 渲染哪些回合：谓词逐字沿用（M12 item 活动足迹 / settle_slice 标记证据链）
  const renderTurns = renderTurnCandidates(trajectory, turnEventMeta, roundSteps);

  // 锚定（UI-FOLLOW-2：落位从「组」细化为「组 + 组内用户消息序号」）：
  //   ① 身份锚定优先（消息组携带该回合 id，既有语义逐字保留）；
  //   ② 身份不同源时按回合槽位（不晚于回合起始 + 容差的最后一条用户消息）入位。
  // 水合收据候选（TRAJ-IMPL-3 §2.3）：活态中**不存在**的回合才补静态收起回执行
  // （同键活态优先，去重发生在入口 receiptsForHydration）。它们与轨迹块候选共用
  // 同一套两级锚定——收据行的落位不是第二套规则，是 UI-FOLLOW-1/2 同一条路径。
  const receiptRows = receiptsForHydration({ receipts, liveTurnIds: liveTurnIdsOf(trajectory, roundSteps) });
  const receiptByTurnId = new Map(receiptRows.map((row) => [row.turnId, row]));

  const slots = messageRoundSlots(groups);
  const anchorByGroupKey = new Map<string, MessageRoundAnchor>();
  const orphanTurnIds: string[] = [];
  const orphanReceiptTurnIds: string[] = [];
  for (const turnId of renderTurns) {
    const anchor = anchorForTurn(groups, slots, turnId, turnStartedAtMs(turnEventMeta, turnId));
    if (!anchor || anchorByGroupKey.has(anchor.groupKey)) {
      orphanTurnIds.push(turnId);
      continue;
    }
    anchorByGroupKey.set(anchor.groupKey, anchor);
  }
  // 收据候选排在轨迹块候选**之后**：一个渲染组只承载一个块（B9），活态块先占位，
  // 收据抢不到组时退流尾（不丢，也不挤掉活态——同键去重之外的第二道保护）。
  for (const row of receiptRows) {
    const anchor = anchorForTurn(groups, slots, row.turnId, receiptStartedAtMs(row));
    if (!anchor || anchorByGroupKey.has(anchor.groupKey)) {
      orphanReceiptTurnIds.push(row.turnId);
      continue;
    }
    anchorByGroupKey.set(anchor.groupKey, anchor);
  }
  const anchoredEntry = (turnId: string, groupKey: string): MessageStreamEntry => {
    const receipt = receiptByTurnId.get(turnId);
    return receipt
      ? { kind: "receipt", key: `${groupKey}:receipt`, turnId, receipt }
      : { kind: "trace", key: `${groupKey}:trace`, turnId };
  };

  const entries: MessageStreamEntry[] = [];
  for (const group of groups) {
    const anchor = anchorByGroupKey.get(group.key);
    const userMessages = group.messages.filter((message) => message.role === "user");
    const restMessages = group.messages.filter((message) => message.role !== "user");
    if (!anchor) {
      if (userMessages.length > 0) {
        entries.push({ kind: "messages", key: `${group.key}:user`, messages: userMessages });
      }
      if (restMessages.length > 0) {
        entries.push({ kind: "messages", key: `${group.key}:rest`, messages: restMessages });
      }
      continue;
    }
    // 组内没有用户消息可依附（纯汇报组，身份锚定命中）：块落在组首，既有行为不变。
    if (userMessages.length === 0) {
      entries.push(anchoredEntry(anchor.turnId, group.key));
      if (group.messages.length > 0) {
        entries.push({ kind: "messages", key: `${group.key}:rest`, messages: group.messages });
      }
      continue;
    }
    // 锚定用户消息及其之前的组内用户消息在块上方；其余（组内后续用户消息 + 汇报）
    // 落到块下方。单用户消息组与既有输出逐字一致（tail 只剩原 rest）。
    const splitAt = Math.min(Math.max(anchor.userIndex, 0), userMessages.length - 1) + 1;
    entries.push({ kind: "messages", key: `${group.key}:user`, messages: userMessages.slice(0, splitAt) });
    entries.push(anchoredEntry(anchor.turnId, group.key));
    const tailMessages = [...userMessages.slice(splitAt), ...restMessages];
    if (tailMessages.length > 0) {
      entries.push({ kind: "messages", key: `${group.key}:rest`, messages: tailMessages });
    }
  }

  // 兜底：确实没有回合附属位的轨迹块挂流尾防丢（无起始时刻证据 / 槽位已占）
  for (const turnId of orphanTurnIds) {
    entries.push({ kind: "trace", key: `orphan:${turnId}`, turnId });
  }
  for (const turnId of orphanReceiptTurnIds) {
    const receipt = receiptByTurnId.get(turnId);
    if (receipt) {
      entries.push({ kind: "receipt", key: `orphan:receipt:${turnId}`, turnId, receipt });
    }
  }

  return { entries, orphanTurnIds, orphanReceiptTurnIds, chainResultMessages };
}

/**
 * 渲染候选回合（TRAJ-IMPL-2 谓词放宽，设计 §2.1-2）：容器存在性 = 回合有**任一活动证据**。
 *
 *   ① 轨迹回合：nodeIds 非空 + M12 证据链（shouldRenderTraceBlockForTurn）——逐字保留；
 *   ② item 步回合（无 trajectory 回合记录，如纯 chat 回合的 31s 执行）：活动足迹
 *      （item 步）或回合起始时刻即证据；M12 的同一证据链由 shouldRenderRoundContainer
 *      重述（settle_slice 标记 + 无足迹 + 短寿命回退，常量取自 traceDelivery 不另立口径）。
 *
 * 两类候选按**回合起始 seq** 混排（同一事件流 seq 域）——锚定顺序因此与事件序一致，
 * 单回合只占一个渲染位（B9「一轮对话一个轨迹块」），后到者保持流尾兜底。
 */
function renderTurnCandidates(
  trajectory: TrajectoryState,
  turnEventMeta: TurnEventMetaMap | undefined,
  roundSteps: RoundStepMap | undefined
): string[] {
  const candidates: Array<{ turnId: string; order: number }> = [];
  for (const turn of trajectoryTurns(trajectory)) {
    if (turn.nodeIds.length > 0 && shouldRenderTraceBlockForTurn(trajectory, turn.id, turnEventMeta)) {
      candidates.push({ turnId: turn.id, order: turnStartSeq(trajectory, turn) });
    }
  }
  for (const turnId of roundStepTurnIds(roundSteps)) {
    // 有轨迹回合记录的回合由 ① 按 M12 谓词裁定（含 settle_slice 隐藏），不重复候选。
    if (trajectory.turns?.[turnId]) {
      continue;
    }
    if (!shouldRenderRoundContainer({ round: roundSteps?.[turnId], meta: turnEventMeta?.[turnId] })) {
      continue;
    }
    candidates.push({ turnId, order: roundStartSeq(roundSteps![turnId]) });
  }
  return candidates.sort((left, right) => left.order - right.order).map((candidate) => candidate.turnId);
}

/** 轨迹回合起始 seq（与 item 步账同域；无节点证据的回合排到最后） */
function turnStartSeq(state: TrajectoryState, turn: TrajectoryTurn): number {
  let minimum = Number.MAX_SAFE_INTEGER;
  for (const nodeId of turn.nodeIds ?? []) {
    const seq = state.nodes?.[nodeId]?.seq;
    if (typeof seq === "number" && Number.isFinite(seq) && seq < minimum) {
      minimum = seq;
    }
  }
  return minimum;
}

/** 对话回合槽位：用户消息是回合的唯一开启者（相邻无 turn_id 消息并组语义不变） */
function messageRoundSlots(groups: MessageTurnGroup[]): MessageRoundSlot[] {
  const slots: MessageRoundSlot[] = [];
  for (const group of groups) {
    let userIndex = 0;
    for (const message of group.messages) {
      if (message.role !== "user") {
        continue;
      }
      const startedAt = Number(message.createdAt);
      if (Number.isFinite(startedAt)) {
        slots.push({ groupKey: group.key, userIndex, startedAt });
      }
      // 序号按「组内用户消息」计数（与渲染切分口径一致），时刻缺失不影响定位。
      userIndex += 1;
    }
  }
  return slots;
}

/**
 * 收据行起始时刻（槽位锚定证据源）：台账行的 started_at（与 turnEventMeta.startedAt
 * 同域同源）。无证据（0/非有限）返回 NaN——**不猜归属**，只走身份锚定，否则退流尾。
 */
function receiptStartedAtMs(receipt: TurnReceipt): number {
  const startedAt = receipt.startedAt;
  return typeof startedAt === "number" && Number.isFinite(startedAt) && startedAt > 0 ? startedAt : Number.NaN;
}

/** 轨迹回合起始时刻（唯一证据源：turnEventMeta.startedAt；缺证据返回 NaN） */
function turnStartedAtMs(turnEventMeta: TurnEventMetaMap | undefined, turnId: string): number {
  const startedAt = turnEventMeta?.[turnId]?.startedAt;
  return typeof startedAt === "number" && Number.isFinite(startedAt) ? startedAt : Number.NaN;
}

/**
 * 轨迹块落位（UI-FOLLOW-1 两级锚定 + UI-FOLLOW-2 容差与组内槽位细分）。
 * 返回 null = 无锚定证据 → 调用方按流尾兜底（不猜归属）。
 */
function anchorForTurn(
  groups: MessageTurnGroup[],
  slots: MessageRoundSlot[],
  turnId: string,
  startedAt: number
): MessageRoundAnchor | null {
  // ① 身份锚定优先：组内消息携带该回合 id（同源命名空间，最强证据）。
  const ownGroup = groups.find((group) => group.turnId === turnId);
  if (ownGroup) {
    const ownSlots = slots.filter((slot) => slot.groupKey === ownGroup.key);
    if (ownSlots.length === 0) {
      // 组内只有汇报消息（无用户消息）：块落在组首（既有行为不变）。
      return { groupKey: ownGroup.key, userIndex: -1, turnId };
    }
    // 同 run 多用户消息（真栈实测：一个 run_id 挂三条 ask 节点）：块只跟组内
    // 开启该回合的那条用户消息——取不晚于「回合起始 + 容差」的最后一条；
    // 全部晚于回合起始时（同一 run 的后续追问）取组内第一条。块绝不越过组内
    // 后续用户消息坠到流尾。
    const withinTurn = Number.isFinite(startedAt)
      ? ownSlots.filter((slot) => slot.startedAt <= startedAt + TURN_SLOT_ANCHOR_TOLERANCE_MS)
      : [];
    const chosen = withinTurn.length > 0 ? withinTurn[withinTurn.length - 1] : ownSlots[0];
    return { groupKey: chosen.groupKey, userIndex: chosen.userIndex, turnId };
  }

  // ② 回合槽位锚定：身份不同源时，取不晚于「回合起始 + 容差」的最后一条用户消息。
  //    无证据（无 startedAt / 无候选槽位）不猜归属 → null → 流尾兜底。
  if (!Number.isFinite(startedAt)) {
    return null;
  }
  let chosen: MessageRoundSlot | null = null;
  for (const slot of slots) {
    if (slot.startedAt <= startedAt + TURN_SLOT_ANCHOR_TOLERANCE_MS) {
      chosen = slot;
    }
  }
  return chosen ? { groupKey: chosen.groupKey, userIndex: chosen.userIndex, turnId } : null;
}
