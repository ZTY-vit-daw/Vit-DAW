import { ChevronRight } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { AuthorityMode, ChatMessage } from "../types";
import type { TrajectoryNode, TrajectoryState, TrajectoryTurn } from "../trajectory";
import { nodeKindLabel, phaseLabel, statusLabel } from "../trajectory/TrajectoryView";
import { StateIcon, terminalClass } from "../taskTrajectory/details";
import { mergeTraceAndRoundSteps, type RoundStep } from "./roundSteps";
import { turnDurationSplit, type TurnEventMeta } from "./turnEventMeta";
import "./trace.css";

/** 完成后自动收起的停留时长（ms）——先让人看清完成态再收 */
export const autoCollapseDelayMs = 900;

/** live 计时器节拍（ms，TRAJ-IMPL-1 §2.4-1/2）：每秒推进「已工作 / 已等」 */
export const liveClockTickMs = 1000;

/**
 * 每秒时钟（TRAJ-IMPL-1）。live 期驱动「已工作 Xs / 已等 Xs」推进；返回取消器，
 * 组件卸载（或 live 退场）时必须调用——不留悬空 interval。
 * 不依赖 DOM（用全局 setInterval），node 环境可用 fake timers 直接钉节拍与清理。
 */
export function startSecondClock(
  onTick: (nowMs: number) => void,
  intervalMs: number = liveClockTickMs
): () => void {
  const timer = setInterval(() => onTick(Date.now()), intervalMs);
  return () => clearInterval(timer);
}

export function isLiveStatus(status: string): boolean {
  return status === "running" || status === "pending";
}

/** 非直播态的收起初值：已完成/等待判断/失败 → 收起为回执 */
export function defaultCollapsedForStatus(status: string): boolean {
  return !isLiveStatus(status);
}

function formatSeconds(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return "--";
  return `${(ms / 1000).toFixed(1)}s`;
}

/** 整秒时长（live 计时器口径，TRAJ-IMPL-1 §2.4-1/2）：向下取整——每秒推进一格。
 *  负值（时钟回拨 / 开始时刻在未来）与非法值钳到 0s，不虚报负时长。 */
export function formatElapsedSeconds(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return "0s";
  return `${Math.floor(ms / 1000)}s`;
}

/** 驻留等待对象（§2.4-2 文案分支，2026-09-14 用户澄清）：同回合有未决 A/B 判定卡
 *  → 等的是人的试听判定；其余 → 等的是续跑（欠收尾评估 / 手动停止后恢复）。 */
export type ResidencyKind = "audition_judgment" | "continuation";

/**
 * 驻留判定（§2.4-2，只读消费 turnEventMeta，归约器零改动）：live 且工作片终点
 * 已到（切片以 waiting_continue 收尾、turn.completed 落地）而回合终局事件未到
 * → 驻留期。驻留终点（turn.stopped）已到的不算驻留——那一段等待已经结束。
 */
export function residencyActive(options: { live: boolean; turnMeta?: TurnEventMeta }): boolean {
  if (!options.live) return false;
  const endedAt = options.turnMeta?.endedAt;
  if (endedAt === undefined || !Number.isFinite(endedAt)) return false;
  const residencyEndedAt = options.turnMeta?.residencyEndedAt;
  return residencyEndedAt === undefined || residencyEndedAt <= endedAt;
}

/**
 * live 头部「已工作」时长（§2.4-1，startedAt 为唯一真源）：
 *  - 工作片未收尾（endedAt 未到）：now - startedAt，每秒推进；
 *  - 工作片已收尾（驻留/等待中）：冻结在 endedAt - startedAt——等待不得计成工作
 *    （CONT-STALL-1 口径延伸：驻留墙钟不得冒充执行时长）；
 *  - 无 startedAt 证据：null（不显示、不造默认值）。
 */
export function liveWorkElapsedMs(options: { turnMeta?: TurnEventMeta; nowMs: number }): number | null {
  const startedAt = options.turnMeta?.startedAt;
  if (startedAt === undefined || !Number.isFinite(startedAt)) return null;
  const endedAt = options.turnMeta?.endedAt;
  if (endedAt === undefined || !Number.isFinite(endedAt)) return options.nowMs - startedAt;
  return endedAt - startedAt;
}

/**
 * 同回合未决 A/B 判定卡（§2.4-2 文案分支的判据，只读消费该回合的轨迹节点）：
 * user_judgment 节点处于等待/运行态，且没有同一 audition 会话的落账节点
 * （trajectory.user_judgment.recorded / completed）把它解掉。判定卡与轨迹回合
 * 同键（audition.ts 的 turnID 同样取 source_turn_id），所以「同回合」= 同 nodeIds。
 */
export function hasPendingAuditionJudgment(state: TrajectoryState, turn: TrajectoryTurn): boolean {
  const nodes = turn.nodeIds
    .map((id) => state.nodes[id])
    .filter((node): node is TrajectoryNode => Boolean(node))
    .filter((node) => node.kind === "user_judgment");
  if (nodes.length === 0) return false;
  const waiting = (node: TrajectoryNode) => ["waiting_for_user", "pending", "running"].includes(node.status.toLowerCase());
  const settled = (node: TrajectoryNode) => node.eventType === "trajectory.user_judgment.recorded" || node.status.toLowerCase() === "completed";
  const sessionOf = (node: TrajectoryNode) => {
    const value = node.details?.audition_session_id;
    return typeof value === "string" ? value.trim() : "";
  };
  return nodes.some((node) => {
    if (!waiting(node)) return false;
    const session = sessionOf(node);
    return !nodes.some((other) =>
      settled(other) && other.seq >= node.seq && (session === "" || sessionOf(other) === session)
    );
  });
}

/** 驻留显式行文案（§2.4-2 + 2026-09-14 用户澄清）：等待对象决定用词；只做显示行，
 *  不加「继续」按钮（用户裁定 B，设计 §7）。 */
export function residencyLineText(options: { kind: ResidencyKind; elapsedMs: number }): string {
  const label = options.kind === "audition_judgment" ? "等待你的试听判定" : "等待续跑";
  return `${label}（已等 ${formatElapsedSeconds(options.elapsedMs)}）`;
}

/**
 * 步种类脚注（TRAJ-IMPL-2）：轨迹节点族沿用 trajectory 域的 nodeKindLabel；item 步
 * （kind=activity）是本卡新增的回合内工具步，给它一个人话脚注——nodeKindLabel 表在
 * trajectory 域，本卡文件域不含它，因此在这里补一张最小的补充表（其余 kind 逐字不变）。
 */
const extraKindLabels: Record<string, string> = { activity: "工具步骤" };

function stepKindLabel(kind: string): string {
  return extraKindLabels[kind] ?? nodeKindLabel(kind);
}

function stepDurationMs(node: TrajectoryNode, next: TrajectoryNode | undefined, live: boolean): number | null {
  if (node.status === "running" || node.status === "pending") return null;
  const end = next ? next.createdAt : live ? Date.now() : node.createdAt;
  return end - node.createdAt;
}

function turnStepNodes(state: TrajectoryState, turn: TrajectoryTurn): TrajectoryNode[] {
  return turn.nodeIds
    .map((id) => state.nodes[id])
    .filter((node): node is TrajectoryNode => Boolean(node))
    .filter((node) => node.kind !== "turn")
    .sort((left, right) => left.seq - right.seq);
}

function turnSpanMs(nodes: TrajectoryNode[], live: boolean): number {
  if (nodes.length === 0) return 0;
  const first = nodes[0].createdAt;
  const last = nodes[nodes.length - 1].createdAt;
  return (live ? Date.now() : last) - first;
}

/** 顶部 meta 语义（B3，真栈床 R3 取证驱动）：
 *  - live：步数随轨迹步事件流式增量（≥1 后不回落显示 0）；尚无步节点保持 --，不虚报 0 步；
 *  - 终态有步：N 步 + 时长（回执语义不变）；
 *  - 终态零步：壳节点（kind=turn）不是步——带 item 活动足迹（M12 证据块）显
 *    「N 项活动 + 真实时长」，纯壳显 --。任何形态不再驻留「0 步 0.0s」。 */
export function traceMetaParts(options: {
  live: boolean;
  stepCount: number;
  stepSpanMs: number;
  itemActivityCount: number;
  itemActivitySpanMs: number | null;
  /**
   * 驻留等待时长（ms，CONT-STALL-1）。null/缺省/0 时渲染逐字不变：没有驻留段
   * 的回合不多一个词。>0 时工作时长显式标注为「执行」，驻留单列为
   * 「等待续跑」——「执行 55s，等待续跑 203s」就是这两个词。
   */
  parkMs?: number | null;
  /**
   * live 头部「已工作」时长（ms，TRAJ-IMPL-1 §2.4-1）：非负有限值且 live 时在步数
   * 之后追加「已工作 Xs」（整秒，每秒推进）。缺省/null/NaN（无 startedAt 证据）
   * 时 live 输出逐字不变——不造默认值。
   */
  liveWorkMs?: number | null;
  /**
   * 滚动窗口丢弃的步数（TRAJ-IMPL-2 §6.2）：>0 时步数后标注「较早 N 步已省略」——
   * 窗口截断是事实，不能在计数上假装没发生。缺省/0 时输出逐字不变。
   */
  omittedStepCount?: number;
}): string[] {
  const omittedStepCount = options.omittedStepCount ?? 0;
  const stepCountLabel = stepCountText(options.stepCount, omittedStepCount);
  const base = (() => {
    if (options.live) {
      const liveBase = options.stepCount > 0 ? [stepCountLabel] : ["--"];
      const liveWorkMs = options.liveWorkMs ?? null;
      if (liveWorkMs === null || !Number.isFinite(liveWorkMs)) {
        return liveBase;
      }
      return [...liveBase, `已工作 ${formatElapsedSeconds(liveWorkMs)}`];
    }
    if (options.stepCount > 0) {
      return [stepCountLabel, formatSeconds(options.stepSpanMs)];
    }
    if (options.itemActivityCount > 0) {
      return options.itemActivitySpanMs !== null
        ? [`${options.itemActivityCount} 项活动`, formatSeconds(options.itemActivitySpanMs)]
        : [`${options.itemActivityCount} 项活动`];
    }
    return ["--"];
  })();
  const parkMs = options.parkMs ?? null;
  if (options.live || parkMs === null || !Number.isFinite(parkMs) || parkMs <= 0) {
    return base;
  }
  const parkPart = `等待续跑 ${formatSeconds(parkMs)}`;
  const last = base[base.length - 1];
  // 末位是时长时把它标成「执行」，否则（--/仅活动数）只追加驻留段。
  if (/^\d+(\.\d+)?s$/.test(last)) {
    return [...base.slice(0, -1), `执行 ${last}`, parkPart];
  }
  return [...base, parkPart];
}

/** 步数文案：窗口截断时显式标注被省略的更早步数（总计数不因窗口缩水） */
function stepCountText(stepCount: number, omittedStepCount: number): string {
  if (omittedStepCount > 0) {
    return `${stepCount} 步（较早 ${omittedStepCount} 步已省略）`;
  }
  return `${stepCount} 步`;
}

/** 回执行语义标签：live 只显「正在处理」，不加戏（2026-09-03 用户裁定口径） */
function turnStatusLabel(status: string): string {
  switch (status) {
    case "running":
    case "pending":
      return "正在处理";
    case "waiting_for_user":
      return "等待你的判断";
    case "stopped":
      return "已停止";
    case "failed":
      return "执行失败";
    default:
      return "执行完成";
  }
}

function receiptSub(nodes: TrajectoryNode[]): string {
  const withSummary = nodes.find((node) => node.summary);
  return withSummary?.summary ?? "";
}

function detailRows(node: TrajectoryNode): Array<[string, string]> {
  const rows: Array<[string, string]> = [];
  if (node.phase) rows.push(["阶段", phaseLabel(node.phase)]);
  if (node.eventType) rows.push(["事件", node.eventType]);
  if (node.evidenceRefs.length) rows.push(["证据", node.evidenceRefs.join("、")]);
  if (node.actionRefs.length) rows.push(["动作", node.actionRefs.join("、")]);
  if (node.projectRevision) rows.push(["修订", node.projectRevision]);
  if (node.checkpointRef) rows.push(["恢复支点", node.checkpointRef]);
  for (const [key, value] of Object.entries(node.details)) {
    rows.push([key, typeof value === "string" ? value : JSON.stringify(value)]);
  }
  return rows;
}

function TraceStep({ node, next, live, authorityMode }: {
  node: TrajectoryNode;
  next: TrajectoryNode | undefined;
  live: boolean;
  authorityMode: AuthorityMode;
}) {
  const [open, setOpen] = useState(false);
  const rows = detailRows(node);
  const isMut = node.kind === "action";
  // UI-FOLLOW-1 终局定格（2026-09-12 用户裁定③）：回合终态是服务端权威事实
  // （turn 家族终局事件）。终态回合里仍标着 running/pending 的步，是「没等到
  // 自己终局」的悬置标记——终局后它不得继续转圈/排队，降级为静态「未收口」
  // （is-unresolved 无 CSS 规则 = 基础墨点，不动画；详情仍可点开）。
  const markOpenEnded = node.status === "running" || node.status === "pending";
  const unresolved = markOpenEnded && !live;
  const statusClass = markOpenEnded
    ? unresolved ? "is-unresolved" : node.status === "running" ? "is-running" : "is-pending"
    : node.status === "failed" ? "is-failed" : "is-plain";
  const duration = stepDurationMs(node, next, live);
  const durationLabel = unresolved
    ? "未收口"
    : node.status === "running" ? "进行中" : node.status === "pending" ? "排队中" : formatSeconds(duration ?? 0);
  return (
    <div
      className={["trace-step", statusClass, isMut ? "is-mut" : "", open ? "is-open" : ""].filter(Boolean).join(" ")}
      role={rows.length ? "button" : undefined}
      tabIndex={rows.length ? 0 : undefined}
      aria-expanded={rows.length ? open : undefined}
      onClick={rows.length ? () => setOpen((value) => !value) : undefined}
      onKeyDown={rows.length ? (event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          setOpen((value) => !value);
        }
      } : undefined}
    >
      <span className="trace-node" aria-hidden="true" />
      <div className="trace-srow">
        <span className="trace-act">
          {node.title || node.eventType || stepKindLabel(node.kind)}
          <span className="trace-gloss">{stepKindLabel(node.kind)}</span>
          {isMut && authorityMode === "full_project_access" && <span className="trace-mutg">完全档 · 直接执行</span>}
        </span>
        <span className={`trace-dur ${node.status === "running" && !unresolved ? "is-live" : ""}`}>{durationLabel}</span>
        {rows.length > 0 && (
          <span className="trace-chev" aria-hidden="true"><ChevronRight size={11} /></span>
        )}
      </div>
      {node.summary && <div className="trace-sum">{node.summary}</div>}
      {rows.length > 0 && (
        <div className="trace-detail"><div>
          <div className="trace-detail-in">
            {rows.map(([label, value]) => <div key={`${label}:${value}`}>{label}：{value}</div>)}
          </div>
        </div></div>
      )}
    </div>
  );
}

/** 发送即显的乐观轨迹条：真 trajectory.turn.started 到达前 0ms 覆盖等待期（与真块同容器类，视觉同一） */
export function OptimisticTraceBlock() {
  return (
    <section className="trace-block is-live is-optimistic" aria-label="执行轨迹：正在处理" aria-live="polite">
      <div className="trace-head" role="status">
        <StateIcon state="running" />
        <strong className="th-label">正在处理</strong>
        <span className="th-meta">--</span>
      </div>
      <div className="trace-wrap"><div>
        <div className="trace-inner">
          <div className="trace-think" aria-live="polite">
            <span className="trace-node" aria-hidden="true" />
            <div className="trace-think-line">
              <span>正在处理…</span>
              <span className="trace-cursor" aria-hidden="true" />
            </div>
          </div>
        </div>
      </div></div>
    </section>
  );
}

/** 乐观占位的显隐判定：发送中、尚无 live 真块接管、流尾是本轮乐观用户消息 */
export function shouldShowOptimisticTrace(options: {
  isSending: boolean;
  trajectory: TrajectoryState;
  messages: ChatMessage[];
}): boolean {
  if (!options.isSending) return false;
  const hasLiveTurn = Object.values(options.trajectory.turns).some(
    (turn) => turn.status === "running" || turn.status === "pending"
  );
  if (hasLiveTurn) return false;
  const last = options.messages[options.messages.length - 1];
  return Boolean(last && last.role === "user" && !(last.turn_id ?? "").trim());
}

export function TraceBlock({ state, turn, activities, authorityMode = "manual_confirmation", turnMeta, itemSteps, totalStepCount }: {
  state: TrajectoryState;
  turn: TrajectoryTurn;
  /** 该回合的 transient 活动（思考行素材；回合结束后活动已被清退） */
  activities: ChatMessage[];
  authorityMode?: AuthorityMode;
  /** 该回合的事件足迹 meta（M12 证据：item 活动数/生命周期；缺省按无足迹处理） */
  turnMeta?: TurnEventMeta;
  /**
   * 回合内 item 步（TRAJ-IMPL-2 §2.1-1/3）：item 工具步还原成的进度步，按 createdAt
   * 与轨迹步混排内联同一容器（行形态复用 TraceStep，kind=activity）。缺省空数组时
   * 渲染逐字不变——既有调用面零回退。
   */
  itemSteps?: RoundStep[];
  /** 该回合步的总计数（滚动窗口外仍计入；缺省=窗口内步数即总数） */
  totalStepCount?: number;
}) {
  const live = isLiveStatus(turn.status);
  const [collapsed, setCollapsed] = useState(() => defaultCollapsedForStatus(turn.status));
  const wasLive = useRef(live);
  // TRAJ-IMPL-1 §2.4-1/2：live 期每秒时钟（驱动「已工作 / 已等」推进）。终态不需要
  // 节拍；卸载或 live 退场即经 startSecondClock 的取消器清理，不留悬空 interval。
  const [nowMs, setNowMs] = useState(() => Date.now());
  useEffect(() => {
    if (!live) {
      return undefined;
    }
    setNowMs(Date.now());
    return startSecondClock(setNowMs);
  }, [live]);

  // 输出完成 → 自动收起（先停留一拍让人看清完成态）
  useEffect(() => {
    if (wasLive.current && !live) {
      const timer = window.setTimeout(() => setCollapsed(true), autoCollapseDelayMs);
      return () => window.clearTimeout(timer);
    }
    wasLive.current = live;
    return undefined;
  }, [live]);

  // 步序混排（§2.1-3）：item 步与轨迹步按 createdAt 内联同一容器；无 item 步时
  // mergeTraceAndRoundSteps 原样返回轨迹步，既有渲染逐字不变。
  const windowSteps = itemSteps ?? [];
  const nodes = mergeTraceAndRoundSteps(turnStepNodes(state, turn), windowSteps);
  const omittedStepCount = Math.max(0, (totalStepCount ?? windowSteps.length) - windowSteps.length);
  const thinking = live ? activities[activities.length - 1] : undefined;
  const label = turnStatusLabel(turn.status);
  const sub = receiptSub(nodes);
  // 工作/驻留分离（CONT-STALL-1）：item 活动时长只取工作片终点，驻留等待单列。
  const split = turnDurationSplit(turnMeta);
  // 驻留期（§2.4-2）：等待是显式的一行，不冒充工作——live 思考行退场、等待行上场；
  // 文案按等待对象区分（同回合未决 A/B 判定卡 → 等待你的试听判定）。
  const residency = residencyActive({ live, turnMeta });
  const waitLine = residency
    ? residencyLineText({
        kind: hasPendingAuditionJudgment(state, turn) ? "audition_judgment" : "continuation",
        elapsedMs: nowMs - (turnMeta?.endedAt ?? nowMs)
      })
    : null;
  const metaParts = traceMetaParts({
    live,
    stepCount: nodes.length,
    stepSpanMs: turnSpanMs(nodes, false),
    itemActivityCount: turnMeta?.itemActivityCount ?? 0,
    itemActivitySpanMs: split.workMs,
    parkMs: split.parkMs,
    liveWorkMs: live ? liveWorkElapsedMs({ turnMeta, nowMs }) : null,
    omittedStepCount
  });

  return (
    <section
      className={["trace-block", terminalClass(turn.status), collapsed ? "is-collapsed" : "", live ? "is-live" : ""].filter(Boolean).join(" ")}
      data-turn-id={turn.id}
      aria-label={`执行轨迹：${label}`}
      aria-live={live ? "polite" : undefined}
    >
      <button
        className="trace-head"
        type="button"
        aria-expanded={!collapsed}
        onClick={() => setCollapsed((value) => !value)}
      >
        <StateIcon state={turn.status} />
        <strong className="th-label">{label}</strong>
        {sub && <small className="th-sub">{sub}</small>}
        <span className="th-meta">{metaParts.map((part) => <span key={part}>{part}</span>)}</span>
        <span className="th-chev" aria-hidden="true"><ChevronRight size={12} /></span>
      </button>
      <div className="trace-wrap"><div>
        <div className="trace-inner">
          {live && !residency && (
            <div className="trace-think" aria-live="polite">
              <span className="trace-node" aria-hidden="true" />
              <div className="trace-think-line">
                <span>{thinking?.content ?? "正在处理…"}</span>
                <span className="trace-cursor" aria-hidden="true" />
              </div>
            </div>
          )}
          {/* 驻留显式行（§2.4-2）：静态墨点 + 每秒推进的已等时长，无转圈/游标，
              不把自己装成正在处理；只做显示行，不加「继续」按钮（用户裁定 B）。 */}
          {waitLine !== null && (
            <div className="trace-wait">
              <span className="trace-node" aria-hidden="true" />
              <div className="trace-wait-line"><span>{waitLine}</span></div>
            </div>
          )}
          {nodes.map((node, index) => (
            <TraceStep
              key={node.id}
              node={node}
              next={nodes[index + 1]}
              live={live}
              authorityMode={authorityMode}
            />
          ))}
          {nodes.length === 0 && !live && <div className="trace-sum">本回合尚未产生轨迹节点。</div>}
        </div>
      </div></div>
    </section>
  );
}
