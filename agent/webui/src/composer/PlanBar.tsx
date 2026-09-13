import { ChevronDown, ChevronRight, CircleSlash } from "lucide-react";
import { useEffect, useState } from "react";
import { localizeDisplayText, statusLabel } from "../App";
import type { JsonRecord } from "../types";
import { record, text, type TaskTrajectorySnapshot } from "../taskTrajectory";
import { StateIcon, TaskTrajectoryDetails, semanticLabel, terminalClass } from "../taskTrajectory/details";
import "./planbar.css";

/**
 * PlanBar 规划条（GUI-T6）：挂在输入框上方的任务规划窄条。
 * 收起单行：状态 + 意图摘要 + 当前步骤 + Task id；展开：TaskTrajectoryDetails
 * （意图/容量评估/状态原因/工程修订/证据引用）。轨迹块只放轨迹，规划内容归此。
 *
 * PLANBAR-1（2026-09-13 手测命中·用户裁定）两条修法：
 *   ① 位置：挂载进 .composer-dock 停靠列（与 composer 同列、栏在其前），
 *      不再留在面板流内被输入框浮层压住（见 planbar.css 停靠列注释）。
 *   ② 生命周期：执行结束后栏要停掉——终态与「无活链的非终态」在有限时间内
 *      淡出并卸载；不带活链的非终态也不得显示「正在观察」类 live 标签。
 */

/** 终态/陈旧态停留时长：够用户读到收尾状态，又不至于永久占屏 */
export const planBarDwellMs = 6000;
/** 淡出时长：先淡出再卸载（两步时间表由 planBarRetracted 判定） */
export const planBarFadeMs = 400;

/** 呈现相位：live=有活链的非终态；stale=无活链支撑的非终态（陈旧恢复形态） */
export type PlanBarPhase = "live" | "waiting" | "warning" | "terminal" | "stale";

const phaseClasses: Record<PlanBarPhase, string> = {
  live: "is-active",
  waiting: "is-waiting",
  warning: "is-warning",
  terminal: "is-terminal",
  stale: "is-stale"
};

/**
 * 相位分类：警示/终态/等待沿用 taskTrajectory/details 的既有状态词汇
 * （terminalClass），仅非终态再按「有没有活链支撑」细分——有链=live（旋转
 * 图标 + 状态标签），无链=stale（灰态「已中断」，不再声称「正在观察」）。
 * 链活是调用侧信号（App：agentTurnRunning || trajectoryLive）。
 */
export function planBarPhase(state: string, chainLive: boolean): PlanBarPhase {
  const kind = terminalClass(state);
  if (kind === "is-warning") return "warning";
  if (kind === "is-terminal") return "terminal";
  if (kind === "is-waiting") return "waiting";
  return chainLive ? "live" : "stale";
}

/** 上收起草表的两种相位：执行已经结束（终态 / 无活链陈旧态） */
export function planBarRetracts(phase: PlanBarPhase): boolean {
  return phase === "terminal" || phase === "stale";
}

/** 有限时间收起：过了停留 + 淡出窗即卸载；live/waiting/warning 永不被时间清场 */
export function planBarRetracted(phase: PlanBarPhase, elapsedMs: number): boolean {
  return planBarRetracts(phase) && elapsedMs >= planBarDwellMs + planBarFadeMs;
}

export function PlanBar({ snapshot, goal = null, plan = null, defaultOpen = false, chainLive = true }: {
  snapshot: TaskTrajectorySnapshot | null;
  /** uiState.goal 回退：status/summary */
  goal?: JsonRecord | null;
  /** uiState.agent_plan 回退：current_step/status/summary */
  plan?: JsonRecord | null;
  defaultOpen?: boolean;
  /** 链活信号（agentTurnRunning || trajectoryLive）；缺省视为有活链，显式 false 才降级为灰态「已中断」 */
  chainLive?: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);
  // 用户交互计数：点开/收起后重新计时，读详情时不被清场打断
  const [dwellTick, setDwellTick] = useState(0);
  const [elapsedMs, setElapsedMs] = useState(0);
  const goalRec = record(goal);
  const planRec = record(plan);
  const task = snapshot ? snapshot.task : null;
  const semantic = snapshot ? snapshot.semantic : null;
  const statusRaw = semantic ? text(semantic.state) : (text(goalRec.status) || text(planRec.status));
  const intent = task ? text(task.original_intent) : (text(goalRec.summary) || text(planRec.summary));
  const currentStep = localizeDisplayText(text(goalRec.current_step) || text(planRec.current_step));
  const phase = planBarPhase(statusRaw, chainLive);
  // 相位/快照/用户交互任一变化都重开时间表（新任务、新状态不再继承上一次的倒计时）
  const episodeKey = `${snapshot?.identity ?? ""}|${snapshot?.semanticRevision ?? 0}|${statusRaw}|${phase}|${dwellTick}`;

  useEffect(() => {
    setElapsedMs(0);
    if (!planBarRetracts(phase)) return undefined;
    const fadeTimer = window.setTimeout(() => setElapsedMs(planBarDwellMs), planBarDwellMs);
    const goneTimer = window.setTimeout(() => setElapsedMs(planBarDwellMs + planBarFadeMs), planBarDwellMs + planBarFadeMs);
    return () => {
      window.clearTimeout(fadeTimer);
      window.clearTimeout(goneTimer);
    };
  }, [episodeKey, phase]);

  // 空任务（无快照且 goal/plan 无内容，idle 视为无内容）→ 整条不渲染
  const idleStatus = statusRaw === "" || statusRaw === "idle" || statusRaw === "empty";
  if (!snapshot && !intent && !currentStep && idleStatus) return null;
  // 终态/陈旧态停够时间 → 清场（用户裁定：执行结束后栏要停掉，不是常驻）
  if (planBarRetracted(phase, elapsedMs)) return null;

  const retiring = planBarRetracts(phase) && elapsedMs >= planBarDwellMs;
  // 无活链的非终态：不显示「正在观察」类 live 标签，只陈述已中断
  const stateLabel = phase === "stale"
    ? "已中断"
    : semantic ? semanticLabel(statusRaw) : (statusLabel(statusRaw, "") || semanticLabel(statusRaw));
  const taskID = task ? text(task.task_id) : "";
  return (
    <section
      className={["plan-bar", phaseClasses[phase], open ? "is-open" : "", retiring ? "is-retiring" : ""].filter(Boolean).join(" ")}
      aria-label="任务规划"
      data-plan-phase={phase}
      data-task-id={taskID || undefined}
    >
      <button
        type="button"
        className="plan-bar-head"
        onClick={() => {
          setOpen((value) => !value);
          setDwellTick((value) => value + 1);
        }}
        aria-expanded={open}
        aria-controls="plan-bar-body"
      >
        {phase === "stale"
          ? <CircleSlash size={15} aria-hidden="true" />
          : <StateIcon state={statusRaw} />}
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
