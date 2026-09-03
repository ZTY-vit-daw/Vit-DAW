import { Check, ChevronRight } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { AuthorityMode, ChatMessage } from "../types";
import type { TrajectoryNode, TrajectoryState, TrajectoryTurn } from "../trajectory";
import { nodeKindLabel, phaseLabel, statusLabel } from "../trajectory/TrajectoryView";
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

function receiptTitle(turn: TrajectoryTurn, stepCount: number, authorityMode: AuthorityMode): string {
  const prefix = authorityMode === "full_project_access" ? "自主执行" : "执行";
  const steps = `${stepCount} 步`;
  switch (turn.status) {
    case "running":
    case "pending":
      return `${prefix} · 进行中 · ${steps}`;
    case "waiting_for_user":
      return `等待你的判断 · ${steps}`;
    case "stopped":
      return `${prefix} · 已停止 · ${steps}`;
    case "failed":
      return `${prefix} · 失败 · ${steps}`;
    default:
      return `${prefix}完成 · ${steps}`;
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
  const statusClass =
    node.status === "running" ? "is-running" :
    node.status === "pending" ? "is-pending" :
    node.status === "failed" ? "is-failed" : "is-plain";
  const duration = stepDurationMs(node, next, live);
  const durationLabel = node.status === "running" ? "进行中" : node.status === "pending" ? "排队中" : formatSeconds(duration ?? 0);
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
        <span className={`trace-dur ${node.status === "running" ? "is-live" : ""}`}>{durationLabel}</span>
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
        <span className="th-ic" aria-hidden="true">
          <Check className="th-done" size={14} />
          <span className="th-spin" />
        </span>
        <span className="th-txt"><b>正在处理</b></span>
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

export function TraceBlock({ state, turn, activities, authorityMode = "manual_confirmation" }: {
  state: TrajectoryState;
  turn: TrajectoryTurn;
  /** 该回合的 transient 活动（思考行素材；回合结束后活动已被清退） */
  activities: ChatMessage[];
  authorityMode?: AuthorityMode;
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
  const title = receiptTitle(turn, nodes.length, authorityMode);
  const sub = receiptSub(nodes);
  const meta = live ? "--" : formatSeconds(turnSpanMs(nodes, false));

  return (
    <section
      className={["trace-block", collapsed ? "is-collapsed" : "", live ? "is-live" : ""].filter(Boolean).join(" ")}
      data-turn-id={turn.id}
      aria-label={`执行轨迹：${title}`}
    >
      <button
        className="trace-head"
        type="button"
        aria-expanded={!collapsed}
        onClick={() => setCollapsed((value) => !value)}
      >
        <span className="th-ic" aria-hidden="true">
          <Check className="th-done" size={14} />
          <span className="th-spin" />
        </span>
        <span className="th-txt"><b>{title}</b>{sub && <small>{sub}</small>}</span>
        <span className="th-meta">{meta}</span>
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
