import { ChevronRight } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { AuthorityMode, ChatMessage } from "../types";
import type { TrajectoryNode, TrajectoryState, TrajectoryTurn } from "../trajectory";
import { nodeKindLabel, phaseLabel, statusLabel } from "../trajectory/TrajectoryView";
import { StateIcon, terminalClass } from "../taskTrajectory/details";
import { turnDurationSplit, type TurnEventMeta } from "./turnEventMeta";
import "./trace.css";

/** 完成后自动收起的停留时长（ms）——先让人看清完成态再收 */
export const autoCollapseDelayMs = 900;

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
}): string[] {
  const base = (() => {
    if (options.live) {
      return options.stepCount > 0 ? [`${options.stepCount} 步`] : ["--"];
    }
    if (options.stepCount > 0) {
      return [`${options.stepCount} 步`, formatSeconds(options.stepSpanMs)];
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
          {node.title || node.eventType || nodeKindLabel(node.kind)}
          <span className="trace-gloss">{nodeKindLabel(node.kind)}</span>
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

export function TraceBlock({ state, turn, activities, authorityMode = "manual_confirmation", turnMeta }: {
  state: TrajectoryState;
  turn: TrajectoryTurn;
  /** 该回合的 transient 活动（思考行素材；回合结束后活动已被清退） */
  activities: ChatMessage[];
  authorityMode?: AuthorityMode;
  /** 该回合的事件足迹 meta（M12 证据：item 活动数/生命周期；缺省按无足迹处理） */
  turnMeta?: TurnEventMeta;
}) {
  const live = isLiveStatus(turn.status);
  const [collapsed, setCollapsed] = useState(() => defaultCollapsedForStatus(turn.status));
  const wasLive = useRef(live);

  // 输出完成 → 自动收起（先停留一拍让人看清完成态）
  useEffect(() => {
    if (wasLive.current && !live) {
      const timer = window.setTimeout(() => setCollapsed(true), autoCollapseDelayMs);
      return () => window.clearTimeout(timer);
    }
    wasLive.current = live;
    return undefined;
  }, [live]);

  const nodes = turnStepNodes(state, turn);
  const thinking = live ? activities[activities.length - 1] : undefined;
  const label = turnStatusLabel(turn.status);
  const sub = receiptSub(nodes);
  // 工作/驻留分离（CONT-STALL-1）：item 活动时长只取工作片终点，驻留等待单列。
  const split = turnDurationSplit(turnMeta);
  const metaParts = traceMetaParts({
    live,
    stepCount: nodes.length,
    stepSpanMs: turnSpanMs(nodes, false),
    itemActivityCount: turnMeta?.itemActivityCount ?? 0,
    itemActivitySpanMs: split.workMs,
    parkMs: split.parkMs
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
          {live && (
            <div className="trace-think" aria-live="polite">
              <span className="trace-node" aria-hidden="true" />
              <div className="trace-think-line">
                <span>{thinking?.content ?? "正在处理…"}</span>
                <span className="trace-cursor" aria-hidden="true" />
              </div>
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
