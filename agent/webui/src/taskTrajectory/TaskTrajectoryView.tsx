import { ChevronDown, ChevronRight } from "lucide-react";
import { useState } from "react";
import { text, type TaskTrajectorySnapshot } from "../taskTrajectory";
import { formatTime, semanticLabel, StateIcon, TaskTrajectoryDetails, terminalClass } from "./details";
import "./taskTrajectory.css";

/**
 * 任务运行轨迹：默认收起为安静单行状态条（GUI-T5），点击展开完整审计体。
 * GUI-F2 起顶部挂载已退出对话面板，视图保留供测试与演示；展开体内容与
 * 对话流回合块「任务详情」小节共用 details.tsx 实现。
 */
export function TaskTrajectoryView({ snapshot, defaultOpen = false }: { snapshot: TaskTrajectorySnapshot | null; defaultOpen?: boolean }) {
  const [open, setOpen] = useState(defaultOpen);
  if (!snapshot) return null;
  const task = snapshot.task;
  const semantic = snapshot.semantic;
  const state = text(semantic.state);
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
      <TaskTrajectoryDetails snapshot={snapshot} />
    </div></div>
  </section>;
}
