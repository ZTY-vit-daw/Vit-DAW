import type { AgentEvent, JsonRecord } from "../types";
import type { TrajectoryNode, TrajectoryTurn } from "../trajectory";
import { SETTLE_SLICE_MAX_LIFESPAN_MS, SETTLE_SLICE_TURN_KIND } from "./traceDelivery";
import { trajectoryTurnIdOfEvent, type TurnEventMeta } from "./turnEventMeta";
import { stepLabel, stepStatusFamily } from "./stepLabels";

// TRAJ-IMPL-2（设计 docs/TRAJECTORY_PRESENTATION_REDESIGN_V1.md §2.1-1/3 + §7 裁定 A，
// 2026-09-14 定稿）：**回合步归约器**——把 item 工具步骤还原成「回合内进度步」，
// 让回合容器的存在性不再绑在实验准入上（问题①可见性错绑的修法）。
//
// 根因（F1+F2）：item.* 事件既不投影为轨迹节点（2026-09-05 取证定案），跨命名空间的
// 归属又让它连回合附属位都拿不到——执行期用户看不到任何进度。本模块与
// reduceTrajectoryEvents / reduceTurnEventMeta **同键**（trajectoryTurnIdOfEvent：
// source_turn_id 优先），因此 B9「一轮对话一个轨迹块」不破：item 步落在该回合自己的
// 容器里，不会另开一个块。
//
// 纯函数 + 增量归约：与轨迹/meta 两个归约器同风格（按 footprint 键去重，轮询重复投递
// 不虚增；生命周期时间戳 min/max 折叠）。协议与 trajectory.ts / turnEventMeta.ts 零改动。

/** 步状态（与 TrajectoryStatus 的显示族对齐：TraceStep 直接消费） */
export type RoundStepStatus = "running" | "completed" | "failed" | "pending";

/** 步种类：item 工具步（kind=activity） / 审批步 / 回合失败步 */
export type RoundStepKind = "activity" | "approval" | "turn";

/**
 * 回合内一步（item 工具步为主）。
 *
 * key 与 turnEventMeta.activityFootprintKey 同源（logical_message_id 优先）：
 * 起止两条事件折叠成一步，且与 meta 的「item 活动足迹」记账一一对应。
 */
export interface RoundStep {
  /**
   * 步身份键（每次工具调用一步）：足迹键 + 起始 seq。真栈形态下**同一次 run 的多次
   * 调用共用 item_id / logical_message_id**（取证：2026-09-11 与 2026-09-13 两份
   * fixture 里三次不同调用都是 agent_item:<run>:tool_step_1），因此足迹键不能当步身份，
   * 只有事件 seq 能把「第几次调用」分开。
   */
  key: string;
  /**
   * 配对足迹键（item_id 优先，回退 logical_message_id / type:seq）：起止两条事件据此
   * 成对折叠，活动线去重也认它（与 turnEventMeta 的 item 活动足迹同指一次调用）。
   */
  footprint: string;
  /** 回合键（trajectoryTurnIdOfEvent，source_turn_id 优先） */
  turnId: string;
  kind: RoundStepKind;
  /** 原始事件类型（item.started / item.completed / item.failed / approval.requested / turn.failed） */
  eventType: string;
  /** 原始标识符（payload.command_name / payload.tool / …）；未命中映射时它就是标题 */
  identifier: string;
  /** 显示标题：映射命中=人话整句，未命中=标识符原样（裁定 A） */
  title: string;
  /** 是否命中人话映射表（false = 待补登记） */
  mapped: boolean;
  status: RoundStepStatus;
  /** 首次目击时刻（步序按它混排；起止折叠时不被完成事件改写） */
  createdAt: number;
  /** 步序 seq（= 起始 seq，独立收口步 = 自身 seq；与轨迹节点 seq 同域） */
  seq: number;
  /** item.started 的 seq（未收口步据此配对；重复投递据此幂等） */
  startSeq?: number;
  /** item.completed / item.failed 的 seq（同上） */
  closeSeq?: number;
  /** 活动线去重钥匙①：与 App 的 agentEventSourceID 同形的活动 id */
  activityId: string;
  /** 活动线去重钥匙②：事件的 logical_message_id */
  logicalMessageId: string;
}

/** 一个回合的步账（含滚动窗口与总计数） */
export interface RoundStepTurn {
  turnId: string;
  /** 滚动窗口内的步（最近 ROUND_STEP_WINDOW 步，按 seq 升序） */
  steps: RoundStep[];
  /** 总计数：窗口截断前见过的步数（头部显示的是它，不因窗口而缩水） */
  totalStepCount: number;
  /** 被滚动窗口丢掉的步数（= 总计数 - 窗口内步数） */
  droppedStepCount: number;
  /** turn.started / trajectory.turn.started 的最早时刻 */
  startedAt?: number;
  /** turn.completed / turn.failed（及其轨迹孪生）的最晚时刻——工作片终点 */
  endedAt?: number;
  /** 回合失败（turn.failed / trajectory.turn.failed / 失败态收尾） */
  failed: boolean;
  /** 回合被停止（turn.stopped / trajectory.turn.stopped） */
  stopped: boolean;
  /** 末次收口事件的原始类型（判活/判终态用；空 = 从未收口 = live） */
  terminalType: string;
  /** 末次收口事件的原始状态（如 waiting_continue / completed / failed） */
  terminalStatus: string;
  /** 末次收口事件的 seq（后到者胜；重复投递幂等） */
  terminalSeq: number;
  /** 本回合任一事件的最小 seq（与轨迹回合的节点 seq 同域，供渲染计划排序） */
  firstSeq?: number;
  /** 本回合任一事件的最大 seq */
  lastSeq?: number;
}

export type RoundStepMap = Record<string, RoundStepTurn>;

/**
 * 每回合滚动窗口（设计 §6.2「item 步量爆炸」的对策）：窗口内留最近 50 步，
 * 头部显示总计数。终态坍缩后只显计数与时长，不铺开全部步。
 */
export const ROUND_STEP_WINDOW = 50;

const STEP_EVENT_TYPES = ["item.started", "item.completed", "item.failed", "approval.requested", "turn.failed"];
const LIFECYCLE_START_TYPES = ["turn.started", "trajectory.turn.started"];
const LIFECYCLE_END_TYPES = ["turn.completed", "trajectory.turn.completed", "turn.failed", "trajectory.turn.failed"];
const LIFECYCLE_STOP_TYPES = ["turn.stopped", "trajectory.turn.stopped"];

export function emptyRoundStepMap(): RoundStepMap {
  return {};
}

export function emptyRoundStepTurn(turnId: string): RoundStepTurn {
  return {
    turnId,
    steps: [],
    totalStepCount: 0,
    droppedStepCount: 0,
    failed: false,
    stopped: false,
    terminalType: "",
    terminalStatus: "",
    terminalSeq: 0
  };
}

/**
 * 增量归约（纯函数）：item.started/completed/failed + approval.requested + turn.failed
 * → 回合内步；turn 家族生命周期 → 容器存活证据与收口状态。
 */
export function reduceRoundSteps(current: RoundStepMap, events: AgentEvent[]): RoundStepMap {
  let next: RoundStepMap | null = null;
  const write = (turnId: string, round: RoundStepTurn) => {
    if (!next) {
      next = { ...current };
    }
    next[turnId] = round;
  };

  for (const event of events) {
    const turnId = trajectoryTurnIdOfEvent(event);
    if (!turnId) {
      continue;
    }
    const type = text(event.type);
    const isStep = STEP_EVENT_TYPES.includes(type);
    const isStart = LIFECYCLE_START_TYPES.includes(type);
    const isEnd = LIFECYCLE_END_TYPES.includes(type);
    const isStop = LIFECYCLE_STOP_TYPES.includes(type);
    if (!isStep && !isStart && !isEnd && !isStop) {
      continue;
    }
    let round = (next ?? current)[turnId] ?? emptyRoundStepTurn(turnId);
    const seq = Number(event.seq) || 0;
    if (seq > 0) {
      round = {
        ...round,
        firstSeq: round.firstSeq === undefined ? seq : Math.min(round.firstSeq, seq),
        lastSeq: round.lastSeq === undefined ? seq : Math.max(round.lastSeq, seq)
      };
    }

    // 生命周期记账：min/max 折叠 —— 重复投递不改变结果（幂等）。
    if (isStart) {
      const at = eventCreatedAt(event);
      if (at !== undefined) {
        const startedAt = round.startedAt === undefined ? at : Math.min(round.startedAt, at);
        round = { ...round, startedAt };
      }
    }
    if (isEnd || isStop) {
      const at = eventCreatedAt(event);
      if (at !== undefined) {
        const endedAt = round.endedAt === undefined ? at : Math.max(round.endedAt, at);
        round = { ...round, endedAt };
      }
    }
    // 收口双轨（设计 §4）：非实验回合由 chat 的 turn.completed/failed/stopped 收口，
    // 实验回合的 trajectory.turn.* 终态同样落这里——两者都只是「这个回合不活了」的
    // 证据，本卡不消费它们做协议判断（协议零改动）。
    if ((isEnd || isStop) && seq >= round.terminalSeq) {
      const status = text(event.status);
      const failed = type.endsWith("failed") || isFailedStatus(status);
      const stopped = isStop || status === "stopped" || status === "cancelled" || status === "canceled";
      round = {
        ...round,
        terminalSeq: seq,
        terminalType: type,
        terminalStatus: status || (stopped ? "stopped" : failed ? "failed" : ""),
        failed,
        stopped
      };
    }

    if (isStep) {
      const footprint = stepFootprintKey(event);
      const incoming = roundStepFromEvent(event, turnId, footprint, seq);
      if (incoming) {
        const steps = round.steps.slice();
        let totalStepCount = round.totalStepCount;
        // ① 同一事件二次到达（轮询回放）：就地刷新，不虚增步数（幂等）。
        const sameEvent = steps.findIndex(
          (item) =>
            item.footprint === footprint &&
            ((incoming.startSeq !== undefined && item.startSeq === incoming.startSeq) ||
              (incoming.closeSeq !== undefined && item.closeSeq === incoming.closeSeq))
        );
        if (sameEvent >= 0) {
          steps[sameEvent] = mergeStepRefresh(steps[sameEvent], incoming);
        } else {
          // ② 收口事件：折叠到**同一足迹最近的未收口步**（item.started → item.completed/failed
          //    成对；真栈同一足迹被多次调用复用，配对照样成立）。找不到就独立成步——
          //    只目击到完成事件时（轮询漏了起始）也要有这一步，不假装它没发生。
          const openIndex = incoming.closeSeq === undefined ? -1 : lastOpenStepIndex(steps, footprint);
          if (openIndex >= 0) {
            steps[openIndex] = mergeStepClose(steps[openIndex], incoming);
          } else {
            steps.push(incoming);
            totalStepCount += 1;
          }
        }
        steps.sort((left, right) => left.seq - right.seq);
        // 滚动窗口：留最近 ROUND_STEP_WINDOW 步；总计数保留（头部显示总数）。
        const overflow = steps.length - ROUND_STEP_WINDOW;
        const windowed = overflow > 0 ? steps.slice(overflow) : steps;
        round = {
          ...round,
          steps: windowed,
          totalStepCount,
          droppedStepCount: Math.max(0, totalStepCount - windowed.length)
        };
      }
    }

    write(turnId, round);
  }
  return next ?? current;
}

/** 渲染候选回合键（按回合最早 seq 升序；与轨迹回合的节点 seq 同域可直接混排） */
export function roundStepTurnIds(map: RoundStepMap | undefined): string[] {
  if (!map) {
    return [];
  }
  return Object.values(map)
    .sort((left, right) => roundStartSeq(left) - roundStartSeq(right))
    .map((round) => round.turnId);
}

/** 回合起始 seq（无 seq 证据回落窗口内首步，再回落 MAX_SAFE_INTEGER：排到最后） */
export function roundStartSeq(round: RoundStepTurn): number {
  if (round.firstSeq !== undefined) {
    return round.firstSeq;
  }
  return round.steps.length > 0 ? round.steps[0].seq : Number.MAX_SAFE_INTEGER;
}

/** 活动线去重钥匙集合：已归属回合的 item 活动不再进流底活动线（§2.1-5） */
export function roundActivityBoundKeys(map: RoundStepMap | undefined): Set<string> {
  const keys = new Set<string>();
  if (!map) {
    return keys;
  }
  for (const round of Object.values(map)) {
    for (const step of round.steps) {
      for (const value of [step.key, step.footprint, step.activityId, step.logicalMessageId]) {
        if (value) {
          keys.add(value);
        }
      }
    }
  }
  return keys;
}

/** 容器存活证据：窗口内有步（活动足迹始终优先，M12 条件①的口径） */
export function roundStepHasEvidence(round: RoundStepTurn | undefined): boolean {
  return Boolean(round && (round.steps.length > 0 || round.totalStepCount > 0));
}

/**
 * 只有 item 步证据的回合（**无 trajectory 回合记录**）是否出容器。
 *
 * M12 隐藏谓词（shouldRenderTraceBlockForTurn，本卡逐字保留、零改动）的同一证据链，
 * 在这里按「回合单活动面」重述——判据、优先级、阈值与常量全部取自 traceDelivery，
 * 不新造口径：
 *   ① 有活动足迹（步） → 渲染（活动足迹证据始终优先，永不隐藏）；
 *   ② 无足迹但有 settle_slice 标记 → 隐藏（服务端权威标记，结算切片噪音不得回归）；
 *   ③ 无标记：时间戳缺失 → 渲染（证据不足不隐藏）；寿命 < 2s → 隐藏，否则渲染。
 */
export function shouldRenderRoundContainer(options: {
  round?: RoundStepTurn;
  meta?: TurnEventMeta;
}): boolean {
  if (roundStepHasEvidence(options.round)) {
    return true;
  }
  if ((options.meta?.itemActivityCount ?? 0) > 0) {
    return true;
  }
  const meta = options.meta;
  if (!meta) {
    return true;
  }
  if (meta.turnKind === SETTLE_SLICE_TURN_KIND) {
    return false;
  }
  if (meta.startedAt === undefined || meta.endedAt === undefined) {
    return true;
  }
  return meta.endedAt - meta.startedAt >= SETTLE_SLICE_MAX_LIFESPAN_MS;
}

/**
 * 回合容器的回合壳（无 trajectory 回合记录时的 TrajectoryTurn 替身）：
 * 只承载 TraceBlock 需要的身份与状态——nodeIds 恒空（轨迹步为空是事实，不是缺陷）。
 */
export function roundTurnShell(round: RoundStepTurn | undefined): TrajectoryTurn | null {
  if (!round) {
    return null;
  }
  return {
    id: round.turnId,
    status: roundTurnStatus(round),
    phase: "",
    nodeIds: [],
    roundIds: [],
    activeNodeId: "",
    activeRoundId: "",
    outcome: "",
    stopped: round.stopped,
    roundScoped: false,
    terminalStatus: round.terminalType ? roundTurnStatus(round) : "",
    terminalPhase: ""
  };
}

/**
 * 回合状态（收口双轨规则的显示侧口径，设计 §4）：
 *  - 未收口 / 收口状态是 waiting_*（切片边界，续跑未定）→ running（live，容器接管乐观占位）；
 *  - failed / stopped → 对应终态；
 *  - 其余收口 → completed（坍缩为回执行）。
 */
export function roundTurnStatus(round: RoundStepTurn): string {
  if (round.failed) {
    return "failed";
  }
  if (round.stopped) {
    return "stopped";
  }
  if (!round.terminalType) {
    return "running";
  }
  const status = round.terminalStatus.trim().toLowerCase();
  if (status.startsWith("waiting") || status === "pending" || status === "requested") {
    return "running";
  }
  if (status === "stopped" || status === "cancelled" || status === "canceled") {
    return "stopped";
  }
  if (status === "failed" || status === "error") {
    return "failed";
  }
  if (round.terminalType.endsWith("failed")) {
    return "failed";
  }
  if (round.terminalType.endsWith("stopped")) {
    return "stopped";
  }
  return "completed";
}

/** item 步 → 轨迹步行形态（复用 TraceStep：kind=activity，行外壳与轨迹步逐字同形） */
export function roundStepAsTrajectoryNode(step: RoundStep): TrajectoryNode {
  return {
    id: `round-step:${step.key}`,
    turnId: step.turnId,
    roundId: "",
    parentId: "",
    kind: step.kind === "activity" ? "activity" : step.kind === "approval" ? "user_judgment" : "error",
    phase: "",
    status: step.status,
    title: step.title,
    summary: "",
    createdAt: step.createdAt,
    seq: step.seq,
    materiality: "",
    targetResponse: "",
    outcome: "",
    nextDecision: "",
    evidenceRefs: [],
    actionRefs: [],
    projectRevision: "",
    checkpointRef: "",
    branchRef: "",
    worktreeRef: "",
    details: {},
    eventType: step.eventType
  };
}

/**
 * 步序混排（§2.1-3）：item 步与轨迹步按 createdAt 内联同一容器（同刻按 seq 稳定）。
 * 无 item 步时**原样返回轨迹步数组**——既有调用面零回退。
 */
export function mergeTraceAndRoundSteps(traceNodes: TrajectoryNode[], roundSteps: RoundStep[]): TrajectoryNode[] {
  if (roundSteps.length === 0) {
    return traceNodes;
  }
  return [...traceNodes, ...roundSteps.map(roundStepAsTrajectoryNode)].sort(
    (left, right) => left.createdAt - right.createdAt || left.seq - right.seq
  );
}

/** 未命中映射表的标识符（唯一、去空）——映射表是活表，这是它的待补清单 */
export function unmappedStepIdentifiers(map: RoundStepMap | undefined): string[] {
  const out: string[] = [];
  if (!map) {
    return out;
  }
  for (const round of Object.values(map)) {
    for (const step of round.steps) {
      if (!step.mapped && step.identifier && !out.includes(step.identifier)) {
        out.push(step.identifier);
      }
    }
  }
  return out;
}

function roundStepFromEvent(event: AgentEvent, turnId: string, footprint: string, seq: number): RoundStep | null {
  const type = text(event.type);
  const payload = record(event.payload);
  const kind: RoundStepKind = type.startsWith("item.")
    ? "activity"
    : type === "approval.requested"
      ? "approval"
      : "turn";
  const status = stepStatus(type, text(event.status));
  const identifier = stepIdentifier(event, payload);
  const label = stepLabel({ identifier, status, eventType: type });
  const createdAt = eventCreatedAt(event) ?? 0;
  const opens = type === "item.started";
  const closes = type === "item.completed" || type === "item.failed";
  return {
    key: `${footprint}#${seq}`,
    footprint,
    turnId,
    kind,
    eventType: type,
    identifier: label.identifier,
    // 未命中且没有标识符时，回落到事件自带标题（比空白诚实），仍不算映射命中。
    title: label.title || text(event.title),
    mapped: label.mapped,
    status,
    createdAt,
    seq,
    startSeq: opens ? seq : undefined,
    closeSeq: closes ? seq : undefined,
    activityId: activityID(event, payload),
    logicalMessageId: text(event.logical_message_id)
  };
}

/** 同一步的重复到达：状态与标题按最新事件，身份与时刻保持首次目击（幂等刷新） */
function mergeStepRefresh(previous: RoundStep, incoming: RoundStep): RoundStep {
  return {
    ...previous,
    status: incoming.status,
    title: incoming.title || previous.title,
    identifier: incoming.identifier || previous.identifier,
    mapped: incoming.mapped || previous.mapped,
    eventType: incoming.eventType,
    activityId: incoming.activityId || previous.activityId,
    logicalMessageId: incoming.logicalMessageId || previous.logicalMessageId
  };
}

/** 收口折叠：起止合成一步（步序与时刻留在起点，状态/人话标题取完成事件） */
function mergeStepClose(open: RoundStep, incoming: RoundStep): RoundStep {
  return {
    ...open,
    closeSeq: incoming.closeSeq,
    status: incoming.status,
    title: incoming.title || open.title,
    identifier: incoming.identifier || open.identifier,
    mapped: incoming.mapped || open.mapped,
    eventType: incoming.eventType,
    activityId: incoming.activityId || open.activityId,
    logicalMessageId: incoming.logicalMessageId || open.logicalMessageId
  };
}

/** 同一足迹最近的未收口 item 步（真栈同一足迹被多次调用复用 → 只能 LIFO 配对） */
function lastOpenStepIndex(steps: RoundStep[], footprint: string): number {
  for (let index = steps.length - 1; index >= 0; index -= 1) {
    const step = steps[index];
    if (step.footprint === footprint && step.kind === "activity" && step.closeSeq === undefined) {
      return index;
    }
  }
  return -1;
}

/**
 * 配对足迹键：**item_id 优先**（一次调用一个 item 标识；起止两条事件据此配对），
 * 缺失回退 logical_message_id，再回退 type:seq。
 *
 * 注意与 turnEventMeta.activityFootprintKey（logical_message_id 优先）的差别：
 * 那个键回答「这个回合有没有发生过活动」（meta 计数），本键回答「这一步是哪一次调用」
 * ——真栈同 run 的多次调用连 item_id 都复用，所以配对还要叠 LIFO（lastOpenStepIndex）。
 */
function stepFootprintKey(event: AgentEvent): string {
  const item = text(event.item_id);
  if (item) {
    return item;
  }
  const logical = text(event.logical_message_id);
  if (logical) {
    return logical;
  }
  return `${text(event.type)}:${Number(event.seq) || 0}`;
}

/** 步状态：事件类型定基调，事件状态定成败 */
function stepStatus(type: string, status: string): RoundStepStatus {
  const value = status.trim().toLowerCase();
  if (value === "failed" || value === "error") {
    return "failed";
  }
  if (type === "item.failed" || type === "turn.failed") {
    return "failed";
  }
  if (type === "item.started" || type === "trajectory.turn.started") {
    return "running";
  }
  if (type === "approval.requested") {
    return "pending";
  }
  if (type === "item.completed") {
    return "completed";
  }
  const family = stepStatusFamily(status, type);
  return family === "pending" ? "pending" : family;
}

/**
 * 标识符解析顺序（真栈事件形态）：completed/failed 带 command_name，
 * started 带 tool（或 command_raw.tool）；缺 payload 时回落 command / item_type /
 * 事件类型——**不猜语义**，解析不到就交给映射表的未命中分支原样显示。
 */
function stepIdentifier(event: AgentEvent, payload: JsonRecord): string {
  const raw = record(payload.command_raw);
  const candidates = [
    text(payload.command_name),
    text(payload.tool),
    text(raw.tool),
    text(payload.command),
    text(event.item_type),
    text(event.type)
  ];
  for (const candidate of candidates) {
    if (candidate) {
      return candidate;
    }
  }
  return "";
}

/** 活动 id：与 App 的 agentEventSourceID 同形（agent_event_<goal>_<item>），活动线去重钥匙 */
function activityID(event: AgentEvent, payload: JsonRecord): string {
  const itemID = text(event.item_id);
  if (!itemID) {
    return "";
  }
  const goalID = text(event.goal_id) || text(payload.goal_id) || "goal";
  return `agent_event_${goalID}_${itemID}`;
}

function isFailedStatus(status: string): boolean {
  const value = status.trim().toLowerCase();
  return value === "failed" || value === "error";
}

function eventCreatedAt(event: AgentEvent): number | undefined {
  const raw = text(event.created_at);
  const at = raw ? Date.parse(raw) : NaN;
  return Number.isFinite(at) ? at : undefined;
}

function record(value: unknown): JsonRecord {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as JsonRecord) : {};
}

function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : value === undefined || value === null ? "" : String(value).trim();
}
