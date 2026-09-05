import { CircleCheck, Clock3, LoaderCircle, Route, ShieldAlert } from "lucide-react";
import type { JsonRecord } from "../types";
import { record, strings, text, type TaskTrajectorySnapshot } from "../taskTrajectory";
import "./taskTrajectory.css";

// 状态集合同时容纳 semantic state（observation_in_progress…）与回合 turn.status
// （running/completed/…）——GUI-F2 起回合回执行与任务详情共用同一套图标语义。
const warningStates = ["failed", "capability_blocked"];
const terminalStates = ["settled", "no_candidate_found", "cancelled", "closed", "completed", "stopped"];
const waitingStates = ["human_judgment_required", "waiting_for_user"];

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
  failed: "任务失败",
  closed: "本轮已收尾"
};

export function semanticLabel(state: string): string {
  return semanticLabels[state] ?? (state || "等待运行时状态");
}

export function terminalClass(state: string): string {
  if (warningStates.includes(state)) return "is-warning";
  if (terminalStates.includes(state)) return "is-terminal";
  if (waitingStates.includes(state)) return "is-waiting";
  return "is-active";
}

export function StateIcon({ state }: { state: string }) {
  if (warningStates.includes(state)) return <ShieldAlert size={15} aria-hidden="true" />;
  if (terminalStates.includes(state)) return <CircleCheck size={15} aria-hidden="true" />;
  if (waitingStates.includes(state)) return <Clock3 size={15} aria-hidden="true" />;
  return <LoaderCircle size={15} className="spin" aria-hidden="true" />;
}

function EvidenceRefs({ refs }: { refs: string[] }) {
  if (!refs.length) return null;
  return <div className="task-trajectory-evidence" aria-label="证据引用">
    <span>证据</span>{refs.map((ref) => <code key={ref}>{ref}</code>)}
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

/** 任务规划展开体（GUI-T6）：仅供输入框上方 PlanBar——轨迹块只放轨迹，轮次记录（invocation 切片/状态变更/续跑态）已随顶栏壳一并退役 */
export function TaskTrajectoryDetails({ snapshot }: { snapshot: TaskTrajectorySnapshot }) {
  const task = snapshot.task;
  const semantic = snapshot.semantic;
  return <>
    <p className="task-trajectory-intent">{text(task.original_intent) || "原始意图尚未可用。"}</p>
    <CapabilityRoute route={snapshot.capabilityRoute} />
    {text(semantic.summary) && <p className="task-trajectory-summary">{text(semantic.summary)}</p>}
    {text(semantic.transition_reason) && <div className="task-trajectory-reason">状态原因：{text(semantic.transition_reason)}</div>}
    {text(semantic.project_revision) && <div className="task-trajectory-revision">工程修订 {text(semantic.project_revision)}</div>}
    <EvidenceRefs refs={strings(semantic.evidence_refs)} />
  </>;
}
