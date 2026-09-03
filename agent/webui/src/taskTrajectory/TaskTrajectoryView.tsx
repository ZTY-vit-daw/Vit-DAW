import { ChevronDown, ChevronRight, CircleCheck, CircleDashed, Clock3, LoaderCircle, Route, ShieldAlert } from "lucide-react";
import { useState } from "react";
import type { JsonRecord } from "../types";
import { record, records, strings, text, type TaskTrajectorySnapshot } from "../taskTrajectory";
import "./taskTrajectory.css";

const semanticLabels: Record<string, string> = {
  observation_in_progress: "正在观察工程",
  diagnostic_complete: "诊断完成",
  no_candidate_found: "未发现候选项",
  improvement_proposal: "已形成改善建议",
  needs_experiment: "需要受控实验",
  human_judgment_required: "等待你的判断",
  capability_blocked: "能力受限",
  settled: "任务已完成",
  cancelled: "任务已取消",
  failed: "任务失败"
};

const continuationLabels: Record<string, string> = {
  pending: "已排入自动续跑",
  claimed: "调度器已认领",
  running: "自动续跑中",
  waiting_interaction: "等待你的交互",
  completed: "续跑已完成",
  cancelled: "续跑已取消",
  failed: "续跑失败"
};

function semanticLabel(state: string): string {
  return semanticLabels[state] ?? (state || "等待运行时状态");
}

function terminalClass(state: string): string {
  if (["failed", "capability_blocked"].includes(state)) return "is-warning";
  if (["settled", "no_candidate_found", "cancelled"].includes(state)) return "is-terminal";
  if (state === "human_judgment_required") return "is-waiting";
  return "is-active";
}

function formatTime(value: unknown): string {
  const date = new Date(text(value));
  if (Number.isNaN(date.valueOf())) return "";
  return new Intl.DateTimeFormat("zh-CN", { hour: "2-digit", minute: "2-digit", month: "numeric", day: "numeric" }).format(date);
}

function StateIcon({ state }: { state: string }) {
  if (["failed", "capability_blocked"].includes(state)) return <ShieldAlert size={15} aria-hidden="true" />;
  if (["settled", "no_candidate_found", "cancelled"].includes(state)) return <CircleCheck size={15} aria-hidden="true" />;
  if (state === "human_judgment_required") return <Clock3 size={15} aria-hidden="true" />;
  return <LoaderCircle size={15} className="spin" aria-hidden="true" />;
}

function EvidenceRefs({ refs, stale = false }: { refs: string[]; stale?: boolean }) {
  if (!refs.length) return null;
  return <div className={`task-trajectory-evidence ${stale ? "is-stale" : ""}`} aria-label={stale ? "历史证据引用" : "证据引用"}>
    <span>{stale ? "历史证据" : "证据"}</span>{refs.map((ref) => <code key={ref}>{ref}</code>)}
  </div>;
}

function CapabilityRoute({ route }: { route: JsonRecord }) {
  if (!Object.keys(route).length) return null;
  const assessment = record(route.capacity_assessment);
  const entryPlan = record(route.entry_plan);
  const capability = text(assessment.selected_capability);
  const level = text(assessment.capacity_level);
  return <div className="task-trajectory-route">
    <Route size={15} aria-hidden="true" />
    <div><strong>{capability === "mixing_capability_layer" ? "已转入混音能力层" : "保留在自由态"}</strong>
      <span>{level || "运行时容量评估"}{text(entryPlan.stage) ? ` · ${text(entryPlan.stage)}` : ""}</span></div>
  </div>;
}

function ContinuationState({ continuation, semantic }: { continuation: JsonRecord; semantic: JsonRecord }) {
  const pending = record(semantic.pending_interaction);
  const continuationInteraction = record(continuation.pending_interaction);
  const interaction = Object.keys(pending).length ? pending : continuationInteraction;
  if (Object.keys(interaction).length) {
    return <div className="task-trajectory-interaction" role="status">
      <Clock3 size={15} aria-hidden="true" /><div><strong>等待你的交互</strong><span>{text(interaction.reason) || text(interaction.kind) || "需要确认、澄清、判断或权限。"}</span></div>
    </div>;
  }
  const status = text(continuation.status);
  if (!status) return null;
  return <div className={`task-trajectory-continuation status-${status}`} role="status">
    <LoaderCircle size={15} aria-hidden="true" /><div><strong>{continuationLabels[status] ?? status}</strong><span>{status === "pending" || status === "claimed" || status === "running" ? "达到本次 invocation 边界后，系统将沿用同一 Task、Run 与原始意图继续。" : "该状态由持久化调度器维护。"}</span></div>
  </div>;
}

function SliceRow({ slice, turns, currentTurnID }: { slice: JsonRecord; turns: JsonRecord[]; currentTurnID: string }) {
  const [open, setOpen] = useState(false);
  const sliceID = text(slice.slice_id);
  const sliceTurns = turns.filter((turn) => text(turn.slice_id) === sliceID);
  const title = `Invocation ${text(slice.sequence) || "-"}`;
  return <section className={`task-trajectory-slice status-${text(slice.status)}`}>
    <button type="button" className="task-trajectory-slice-head" onClick={() => setOpen((value) => !value)} aria-expanded={open} aria-controls={`slice-${sliceID}`}>
      {open ? <ChevronDown size={14} aria-hidden="true" /> : <ChevronRight size={14} aria-hidden="true" />}
      <strong>{title}</strong><span>max turns {text(slice.max_turns) || "-"}</span><span className="task-trajectory-state-chip">{text(slice.status) || "unknown"}</span>
    </button>
    {open && <div className="task-trajectory-turns" id={`slice-${sliceID}`}>
      {sliceTurns.map((turn) => <div key={text(turn.turn_id)} className={`task-trajectory-turn ${text(turn.turn_id) === currentTurnID ? "is-current" : ""}`}>
        <CircleDashed size={13} aria-hidden="true" /><span>Turn {text(turn.sequence) || "-"}</span><strong>{text(turn.source) === "automatic_continuation" ? "自动续跑" : text(turn.source) || "任务执行"}</strong><em>{text(turn.status) || "unknown"}</em>
      </div>)}
      {!sliceTurns.length && <span className="task-trajectory-empty">此 invocation 尚未记录 Turn。</span>}
    </div>}
  </section>;
}

function SemanticHistory({ transitions }: { transitions: JsonRecord[] }) {
  const [open, setOpen] = useState(false);
  if (!transitions.length) return null;
  return <section className="task-trajectory-history">
    <button type="button" className="task-trajectory-history-toggle" onClick={() => setOpen((value) => !value)} aria-expanded={open}>
      {open ? <ChevronDown size={14} aria-hidden="true" /> : <ChevronRight size={14} aria-hidden="true" />} 状态变更记录 <span>{transitions.length}</span>
    </button>
    {open && <ol>
      {transitions.map((transition) => {
        const stale = transition.stale_for_current_revision === true;
        return <li key={`${text(transition.revision)}:${text(transition.event)}`} className={stale ? "is-stale" : ""}>
          <div><span>r{text(transition.revision)}</span><strong>{semanticLabel(text(transition.to))}</strong><time>{formatTime(transition.occurred_at)}</time></div>
          {(text(transition.summary) || text(transition.reason)) && <p>{text(transition.summary) || text(transition.reason)}</p>}
          <EvidenceRefs refs={strings(transition.evidence_refs)} stale={stale} />
        </li>;
      })}
    </ol>}
  </section>;
}

/** 任务运行轨迹：默认收起为安静单行状态条（GUI-T5），点击展开完整审计体 */
export function TaskTrajectoryView({ snapshot, defaultOpen = false }: { snapshot: TaskTrajectorySnapshot | null; defaultOpen?: boolean }) {
  const [open, setOpen] = useState(defaultOpen);
  if (!snapshot) return null;
  const task = snapshot.task;
  const run = snapshot.run;
  const semantic = snapshot.semantic;
  const state = text(semantic.state);
  const slices = records(run.slices);
  const turns = records(run.turns);
  const updated = formatTime(semantic.updated_at);
  return <section className={["task-trajectory", terminalClass(state), open ? "is-open" : ""].filter(Boolean).join(" ")} aria-label="任务运行轨迹" data-task-id={text(task.task_id)} data-semantic-state={state}>
    <button type="button" className="task-trajectory-head" onClick={() => setOpen((value) => !value)} aria-expanded={open} aria-controls="task-trajectory-body">
      <StateIcon state={state} />
      <strong>{semanticLabel(state)}</strong>
      <span className="task-trajectory-head-meta">
        <span>Task {text(task.task_id)}</span>
        <span>Run {text(task.run_id)}</span>
        <span>r{text(semantic.revision) || "0"}</span>
        {updated && <span>更新 {updated}</span>}
      </span>
      <span className="task-trajectory-head-chevron" aria-hidden="true">
        {open ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
      </span>
    </button>
    <div className="task-trajectory-body" id="task-trajectory-body"><div className="task-trajectory-body-in">
      <p className="task-trajectory-intent">{text(task.original_intent) || "原始意图尚未可用。"}</p>
      <ContinuationState continuation={snapshot.continuation} semantic={semantic} />
      <CapabilityRoute route={snapshot.capabilityRoute} />
      {text(semantic.summary) && <p className="task-trajectory-summary">{text(semantic.summary)}</p>}
      {text(semantic.transition_reason) && <div className="task-trajectory-reason">状态原因：{text(semantic.transition_reason)}</div>}
      {text(semantic.project_revision) && <div className="task-trajectory-revision">工程修订 {text(semantic.project_revision)}</div>}
      <EvidenceRefs refs={strings(semantic.evidence_refs)} />
      <section className="task-trajectory-run" aria-label="Run invocation 历史"><div className="task-trajectory-section-title">Run 的 invocation 切片 <span>{slices.length}</span></div>
        {slices.map((slice) => <SliceRow key={text(slice.slice_id)} slice={slice} turns={turns} currentTurnID={text(run.current_turn_id)} />)}
        {!slices.length && <div className="task-trajectory-empty">任务已建立，等待首个 invocation。</div>}
      </section>
      <SemanticHistory transitions={snapshot.transitions} />
    </div></div>
  </section>;
}
