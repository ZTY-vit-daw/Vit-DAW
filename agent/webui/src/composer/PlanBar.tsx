import { ChevronDown, ChevronRight } from "lucide-react";
import { useState } from "react";
import { localizeDisplayText, statusLabel } from "../App";
import type { JsonRecord } from "../types";
import { record, text, type TaskTrajectorySnapshot } from "../taskTrajectory";
import { StateIcon, TaskTrajectoryDetails, semanticLabel, terminalClass } from "../taskTrajectory/details";
import "./planbar.css";

/**
 * PlanBar 规划条（GUI-T6）：挂在输入框上方的任务规划窄条。
 * 收起单行：状态 + 意图摘要 + 当前步骤 + Task id；展开：TaskTrajectoryDetails
 * （意图/容量评估/状态原因/工程修订/证据引用）。轨迹块只放轨迹，规划内容归此。
 */
export function PlanBar({ snapshot, goal = null, plan = null, defaultOpen = false }: {
  snapshot: TaskTrajectorySnapshot | null;
  /** uiState.goal 回退：status/summary */
  goal?: JsonRecord | null;
  /** uiState.agent_plan 回退：current_step/status/summary */
  plan?: JsonRecord | null;
  defaultOpen?: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const goalRec = record(goal);
  const planRec = record(plan);
  const task = snapshot ? snapshot.task : null;
  const semantic = snapshot ? snapshot.semantic : null;
  const statusRaw = semantic ? text(semantic.state) : (text(goalRec.status) || text(planRec.status));
  const intent = task ? text(task.original_intent) : (text(goalRec.summary) || text(planRec.summary));
  const currentStep = localizeDisplayText(text(goalRec.current_step) || text(planRec.current_step));
  // 空任务（无快照且 goal/plan 无内容，idle 视为无内容）→ 整条不渲染
  const idleStatus = statusRaw === "" || statusRaw === "idle" || statusRaw === "empty";
  if (!snapshot && !intent && !currentStep && idleStatus) return null;

  const stateLabel = semantic ? semanticLabel(statusRaw) : (statusLabel(statusRaw, "") || semanticLabel(statusRaw));
  const taskID = task ? text(task.task_id) : "";
  return (
    <section
      className={["plan-bar", terminalClass(statusRaw), open ? "is-open" : ""].filter(Boolean).join(" ")}
      aria-label="任务规划"
      data-task-id={taskID || undefined}
    >
      <button type="button" className="plan-bar-head" onClick={() => setOpen((value) => !value)} aria-expanded={open} aria-controls="plan-bar-body">
        <StateIcon state={statusRaw} />
        <strong>{stateLabel}</strong>
        {intent && <span className="plan-bar-intent" title={intent}>{intent}</span>}
        {currentStep && <span className="plan-bar-step">{currentStep}</span>}
        {taskID && <span className="plan-bar-task">Task {taskID}</span>}
        <span className="plan-bar-chev" aria-hidden="true">{open ? <ChevronDown size={13} /> : <ChevronRight size={13} />}</span>
      </button>
      <div className="plan-bar-body" id="plan-bar-body"><div className="plan-bar-body-in">
        {semantic ? <TaskTrajectoryDetails snapshot={snapshot!} /> : (
          <>
            {intent && <p className="task-trajectory-intent">{intent}</p>}
            {currentStep && <div className="task-trajectory-revision">当前步骤：{currentStep}</div>}
          </>
        )}
      </div></div>
    </section>
  );
}
