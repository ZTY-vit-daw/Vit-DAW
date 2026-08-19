import { ChevronDown, ChevronRight, CircleCheck, CircleDashed, CircleDot, RotateCcw, Square, TriangleAlert } from "lucide-react";
import { useState, type ReactNode } from "react";
import type { TrajectoryNode, TrajectoryRound, TrajectoryState, TrajectoryTurn } from "../trajectory";
import { trajectoryNodesForRound, trajectoryRounds, trajectoryTurns } from "../trajectory";

export function statusLabel(status: string): string {
  switch (status) {
    case "running":
      return "进行中";
    case "completed":
      return "已完成";
    case "stopped":
      return "已停止";
    case "failed":
      return "失败";
    case "waiting_for_user":
      return "等待判断";
    case "pending":
      return "等待中";
    default:
      return status || "未开始";
  }
}

export function evaluationLabel(evaluation: string): string {
  switch (evaluation) {
    case "insufficient_dose":
      return "处理量不足";
    case "agent_evaluable":
      return "方向成立";
    case "human_audition_ready":
      return "等待试听";
    case "human_confirmed":
      return "用户已确认";
    case "unsupported_hypothesis":
      return "假设未获支持";
    case "rolled_back":
      return "已回退";
    case "ambiguous":
      return "需要判断";
    case "not_ready":
      return "尚未就绪";
    default:
      return evaluation || "未评估";
  }
}

export function phaseLabel(phase: string): string {
  const labels: Record<string, string> = {
    framing: "建立目标",
    observing: "观察证据",
    admitted: "实验已准入",
    treating: "执行处理",
    materiality_evaluating: "确认变化量",
    target_response_evaluating: "检查目标响应",
    deciding: "决定下一步",
    rolled_back: "已回退",
    settled: "已收口",
    stopped: "已停止"
  };
  return labels[phase] ?? (phase || "处理中");
}

export function targetResponseLabel(response: string): string {
  const labels: Record<string, string> = {
    directional: "方向成立",
    none: "未观察到响应",
    adverse: "出现副作用",
    ambiguous: "响应不明确"
  };
  return labels[response] ?? (response || "未记录");
}

export function decisionLabel(decision: string): string {
  const labels: Record<string, string> = {
    increase_dose: "提高处理量",
    reduce_dose: "降低处理量",
    keep: "保留当前处理",
    rollback: "回退到恢复支点",
    stop: "停止任务"
  };
  return labels[decision] ?? (decision || "未决定");
}
export function nodeKindLabel(kind: string): string {
  const labels: Record<string, string> = {
    intent: "目标",
    observation: "观察",
    hypothesis: "假设",
    action: "处理",
    materiality: "变化量",
    verification: "验证",
    decision: "决策",
    rollback: "回退",
    branch_or_worktree: "分支",
    user_judgment: "用户判断",
    settlement: "收口",
    error: "错误",
    turn: "任务"
  };
  return labels[kind] ?? (kind || "节点");
}

function nodeIcon(node: TrajectoryNode): ReactNode {
  if (node.kind === "rollback" || node.outcome === "rolled_back") {
    return <RotateCcw size={15} aria-hidden="true" />;
  }
  if (node.kind === "error" || node.status === "failed") {
    return <TriangleAlert size={15} aria-hidden="true" />;
  }
  if (node.status === "running") {
    return <CircleDot size={15} aria-hidden="true" />;
  }
  if (node.status === "stopped") {
    return <Square size={12} aria-hidden="true" />;
  }
  return <CircleCheck size={15} aria-hidden="true" />;
}

function nodeStatusClass(node: TrajectoryNode): string {
  if (node.kind === "rollback" || node.outcome === "rolled_back") return "is-rollback";
  if (node.kind === "error" || node.status === "failed") return "is-error";
  if (node.status === "running") return "is-running";
  if (node.status === "stopped") return "is-stopped";
  return "is-complete";
}

function hasNodeDetails(node: TrajectoryNode): boolean {
  return Boolean(
    node.phase ||
      node.eventType ||
      node.evidenceRefs.length ||
      node.actionRefs.length ||
      node.checkpointRef ||
      node.projectRevision ||
      node.branchRef ||
      node.worktreeRef ||
      Object.keys(node.details).length
  );
}

function detailValue(value: unknown): string {
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function detailRows(node: TrajectoryNode): Array<[string, string]> {
  const rows: Array<[string, string]> = [];
  if (node.phase) rows.push(["阶段", phaseLabel(node.phase)]);
  if (node.eventType) rows.push(["事件", node.eventType]);
  if (node.evidenceRefs.length) rows.push(["证据引用", node.evidenceRefs.join("、")]);
  if (node.actionRefs.length) rows.push(["动作引用", node.actionRefs.join("、")]);
  if (node.checkpointRef) rows.push(["恢复支点", node.checkpointRef]);
  if (node.projectRevision) rows.push(["工程修订", node.projectRevision]);
  if (node.branchRef) rows.push(["分支引用", node.branchRef]);
  if (node.worktreeRef) rows.push(["工作树引用", node.worktreeRef]);
  for (const [key, value] of Object.entries(node.details)) {
    rows.push([key, detailValue(value)]);
  }
  return rows;
}

export function TraceNode({ node }: { node: TrajectoryNode }) {
  const [detailsOpen, setDetailsOpen] = useState(false);
  const rows = detailRows(node);
  const hasDetails = hasNodeDetails(node);
  return (
    <article className={`trajectory-node ${nodeStatusClass(node)}`} data-node-kind={node.kind} data-node-id={node.id}>
      <div className="trajectory-node-marker">{nodeIcon(node)}</div>
      <div className="trajectory-node-content">
        <div className="trajectory-node-heading">
          <span className="trajectory-node-kind">{nodeKindLabel(node.kind)}</span>
          <strong>{node.title || nodeKindLabel(node.kind)}</strong>
          <span className="trajectory-node-status">{statusLabel(node.status)}</span>
        </div>
        {node.summary && <p className="trajectory-node-summary">{node.summary}</p>}
        {(node.materiality || node.targetResponse || node.outcome) && (
          <div className="trajectory-node-facts">
            {node.materiality && <span className={`trajectory-fact fact-${node.materiality}`}>{evaluationLabel(node.materiality)}</span>}
            {node.targetResponse && <span className="trajectory-fact">目标响应：{targetResponseLabel(node.targetResponse)}</span>}
            {node.outcome && node.outcome !== node.materiality && <span className={`trajectory-fact fact-${node.outcome}`}>结果：{evaluationLabel(node.outcome)}</span>}
          </div>
        )}
        {node.nextDecision && <div className="trajectory-node-next">下一步：{decisionLabel(node.nextDecision)}</div>}
        {hasDetails && (
          <>
            <button
              className="trajectory-node-detail-toggle"
              type="button"
              aria-expanded={detailsOpen}
              onClick={() => setDetailsOpen((open) => !open)}
            >
              {detailsOpen ? <ChevronDown size={13} aria-hidden="true" /> : <ChevronRight size={13} aria-hidden="true" />}
              {detailsOpen ? "收起审计详情" : "查看审计详情"}
            </button>
            {detailsOpen && (
              <dl className="trajectory-node-details">
                {rows.map(([label, value]) => (
                  <div key={`${label}:${value}`}>
                    <dt>{label}</dt>
                    <dd>{value}</dd>
                  </div>
                ))}
              </dl>
            )}
          </>
        )}
      </div>
    </article>
  );
}

export function RoundContainer({ round, nodes, expanded, onToggle }: {
  round: TrajectoryRound;
  nodes: TrajectoryNode[];
  expanded: boolean;
  onToggle: () => void;
}) {
  const active = round.status === "running";
  return (
    <section className={`trajectory-round ${expanded ? "is-expanded" : "is-collapsed"} ${active ? "is-active" : ""}`} data-round-id={round.id}>
      <button
        className="trajectory-round-header"
        type="button"
        onClick={onToggle}
        aria-expanded={expanded}
        aria-controls={`trajectory-round-body-${round.id}`}
      >
        <span className="trajectory-round-chevron">{expanded ? <ChevronDown size={16} /> : <ChevronRight size={16} />}</span>
        <span className="trajectory-round-index">Round {roundIndex(round.id, nodes)}</span>
        <span className="trajectory-round-title">{roundTitle(round, nodes)}</span>
        <span className={`trajectory-round-status status-${round.status}`}>{statusLabel(round.status)}</span>
        {round.evaluation && <span className={`trajectory-round-evaluation evaluation-${round.evaluation}`}>{evaluationLabel(round.evaluation)}</span>}
      </button>
      {expanded && (
        <div className="trajectory-round-body" id={`trajectory-round-body-${round.id}`}>
          {nodes.length > 0 ? nodes.map((node) => <TraceNode key={node.id} node={node} />) : <div className="trajectory-empty">等待轨迹节点…</div>}
          {round.decision && <div className="trajectory-round-decision">下一步：{decisionLabel(round.decision)}</div>}
        </div>
      )}
    </section>
  );
}

function roundIndex(id: string, nodes: TrajectoryNode[]): string {
  const match = id.match(/(\d+)$/);
  return match?.[1] ?? (nodes.length ? "·" : "—");
}

function roundTitle(round: TrajectoryRound, nodes: TrajectoryNode[]): string {
  const firstTitle = nodes.find((node) => node.title)?.title;
  if (firstTitle) return firstTitle;
  if (round.evaluation === "insufficient_dose") return "校准有效处理量";
  if (round.evaluation === "rolled_back") return "撤销副作用过大的处理";
  if (round.status === "running") return "正在进行实验";
  return "实验轮次";
}

function turnNodes(state: TrajectoryState, turn: TrajectoryTurn): TrajectoryNode[] {
  return turn.nodeIds.map((id) => state.nodes[id]).filter((node): node is TrajectoryNode => Boolean(node));
}

function turnTitle(state: TrajectoryState, turn: TrajectoryTurn): string {
  const nodes = turnNodes(state, turn);
  const preferred = nodes.find((node) => node.kind === "turn" && node.title) ?? nodes.find((node) => node.kind === "intent" && node.title);
  return preferred?.title || "自由态改善任务";
}

function turnSummary(state: TrajectoryState, turn: TrajectoryTurn): string {
  const nodes = turnNodes(state, turn);
  return nodes.find((node) => node.kind === "intent" && node.summary)?.summary || "Agent 正在根据可审计证据进行多轮改善。";
}

export function TurnHeader({ state, turn }: { state: TrajectoryState; turn: TrajectoryTurn }) {
  const rounds = trajectoryRounds(state, turn.id);
  const activeRound = rounds.find((round) => round.id === turn.activeRoundId);
  const currentPhase = turn.phase || activeRound?.phase || "";
  return (
    <header className={`trajectory-turn-header status-${turn.status}`}>
      <div className="trajectory-turn-kicker"><CircleDashed size={14} /> 可审计执行轨迹 <span>·</span> {statusLabel(turn.status)}</div>
      <div className="trajectory-turn-title-row">
        <div>
          <h2>{turnTitle(state, turn)}</h2>
          <p>{turnSummary(state, turn)}</p>
        </div>
        <div className="trajectory-turn-meta">
          <span>{rounds.length} 轮实验</span>
          {currentPhase && <span>{phaseLabel(currentPhase)}</span>}
          {turn.outcome && <span className={`trajectory-outcome outcome-${turn.outcome}`}>{evaluationLabel(turn.outcome)}</span>}
        </div>
      </div>
      {turn.stopped && <div className="trajectory-stop-note">用户已停止当前任务。已保留停止前的稳定工程状态。</div>}
    </header>
  );
}

function ContextNodes({ nodes }: { nodes: TrajectoryNode[] }) {
  if (nodes.length === 0) return null;
  return (
    <section className="trajectory-context" aria-label="任务上下文节点">
      <div className="trajectory-section-label">任务上下文</div>
      {nodes.map((node) => <TraceNode key={node.id} node={node} />)}
    </section>
  );
}

function Settlement({ nodes }: { nodes: TrajectoryNode[] }) {
  if (nodes.length === 0) return null;
  return (
    <section className="trajectory-settlement" aria-label="Settlement 收口">
      <div className="trajectory-section-label">Settlement / 收口</div>
      {nodes.map((node) => <TraceNode key={node.id} node={node} />)}
      <div className="trajectory-settlement-footer"><CircleCheck size={16} aria-hidden="true" /> 这次处理已形成可审计收口。</div>
    </section>
  );
}

export function TrajectoryTurnView({ state, turn }: { state: TrajectoryState; turn: TrajectoryTurn }) {
  const rounds = trajectoryRounds(state, turn.id);
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const nodes = turnNodes(state, turn);
  const contextNodes = nodes.filter((node) => !node.roundId && node.kind !== "turn" && node.kind !== "settlement");
  const settlementNodes = nodes.filter((node) => node.kind === "settlement");

  return (
    <section className="trajectory-turn" data-turn-id={turn.id}>
      <TurnHeader state={state} turn={turn} />
      <ContextNodes nodes={contextNodes} />
      <div className="trajectory-rounds">
        <div className="trajectory-section-label">实验轮次</div>
        {rounds.map((round) => {
          const isExpanded = expanded.has(round.id);
          return (
            <RoundContainer
              key={round.id}
              round={round}
              nodes={trajectoryNodesForRound(state, round.id)}
              expanded={isExpanded}
              onToggle={() => setExpanded((current) => {
                const next = new Set(current);
                if (next.has(round.id)) next.delete(round.id); else next.add(round.id);
                return next;
              })}
            />
          );
        })}
        {rounds.length === 0 && <div className="trajectory-empty">当前 Turn 尚未产生 Round。</div>}
      </div>
      <Settlement nodes={settlementNodes} />
    </section>
  );
}

export function TrajectoryView({ state, title = "Observable Trajectory" }: { state: TrajectoryState; title?: string }) {
  const turns = trajectoryTurns(state);
  return (
    <section className="trajectory-view" aria-label="可审计执行轨迹">
      <div className="trajectory-view-bar">
        <div><span className="trajectory-view-eyebrow">VIT / WORK TRACE</span><strong>{title}</strong></div>
        <span className="trajectory-view-version">vit.observable_trajectory.v1</span>
      </div>
      {turns.length > 0 ? turns.map((turn) => <TrajectoryTurnView key={turn.id} state={state} turn={turn} />) : <div className="trajectory-empty trajectory-empty-view">还没有可显示的执行轨迹。</div>}
    </section>
  );
}
