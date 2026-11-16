import { Icon } from "./icons";
import { StatusRing } from "./StatusRing";

/**
 * 状态卡（SPEC-V5 §8.2 + §4.2.1）：Vit 正在做一件多步的事，以执行轨迹呈现。
 * 已完成步骤收成轨迹行（状态图标 + 步骤名 + 耗时）；当前步骤动效 + 小面积黄；
 * 最多同时显示 3 步。进度只用一种表达：进度条。
 */
export type StatusCardState = "running" | "completed" | "failed";

export interface TraceStep {
  id: string;
  label: string;
  status: "done" | "current";
  /** 已完成步骤的耗时（mono 13）。 */
  duration?: string;
}

export interface StatusCardProps {
  /** 标题直接写正在做什么，如「正在打开《鼓组混音 v2》」。 */
  title: string;
  steps: TraceStep[];
  state: StatusCardState;
  /** 0..1 进度（running 态显示进度条）。 */
  progress?: number;
  failureReason?: string;
  onRetry?: () => void;
  className?: string;
}

/** 已完成卡收起为一行回执；轨迹可展开（默认展开最近三步）。 */
export function StatusCard({
  title,
  steps,
  state,
  progress,
  failureReason,
  onRetry,
  className,
}: StatusCardProps) {
  const cls = ["v5-status-card", `v5-status-card--${state}`, className]
    .filter(Boolean)
    .join(" ");
  const visible = steps.slice(-3);
  return (
    <section className={cls} data-status-card-state={state}>
      <header className="v5-status-card-head">
        <h3 className="v5-status-card-title">{title}</h3>
        {state === "running" ? (
          <span className="v5-status-card-progress" aria-label="进度">
            <span
              className="v5-progress-track"
              role="progressbar"
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={Math.round((progress ?? 0) * 100)}
            >
              <span
                className="v5-progress-fill"
                data-graphic="progress-fill"
                style={{ width: `${Math.round((progress ?? 0) * 100)}%` }}
              />
            </span>
          </span>
        ) : null}
      </header>
      {state === "failed" && failureReason ? (
        <p className="v5-status-card-reason">{failureReason}</p>
      ) : null}
      {state !== "completed" ? (
        <ul className="v5-trace">
          {visible.map((s) => (
            <li
              key={s.id}
              className={
                s.status === "current"
                  ? "v5-trace-step v5-step-current"
                  : "v5-trace-step"
              }
            >
              {s.status === "current" ? (
                <span className="v5-current-chip" data-graphic="current-chip">
                  <StatusRing state="busy" size={16} />
                </span>
              ) : (
                <Icon name="i-done" size={16} className="v5-trace-done" />
              )}
              <span className="v5-trace-label">{s.label}</span>
              {s.duration ? (
                <time className="v5-trace-time">{s.duration}</time>
              ) : null}
            </li>
          ))}
        </ul>
      ) : null}
      {state === "failed" && onRetry ? (
        <div className="v5-status-card-actions">
          <button type="button" className="v5-btn v5-btn--secondary" onClick={onRetry}>
            <span>重试</span>
          </button>
        </div>
      ) : null}
      {state === "completed" ? (
        <p className="v5-status-card-receipt">
          <Icon name="i-done" size={16} className="v5-status-card-done" />
          <span>已完成，轨迹已存入执行记录</span>
        </p>
      ) : null}
    </section>
  );
}
