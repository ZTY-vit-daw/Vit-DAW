/*
 * UI-LOADING-1（347）P2 装载状态卡（SPEC-V5 §8.2 / §7.4 / §4.2.1）。
 *
 * 红线：
 * - 进度只用一种表达：磁带带量（tapeSVG，tapeReelAmounts 映射）；未知进度停
 *   待机帧，不伪造倒计时/百分比（无任何时间驱动，见 loadingData.ts）。
 * - 步骤同屏最多 3 条；当前步骤小面积黄 chip（busy 环），不做整卡黄底。
 * - 完成（蓝）收成一行回执，轨迹可展开（原生 details）；失败（红）写明原因
 *   与重试；中断（黄）写明原因与恢复入口。
 * - 磁带身份色只在磁带内部（§4.5）；无氛围色用中性 [200,200,196]（§7.4）。
 */

import { useMemo } from "react";
import { Icon, StatusRing } from "../../components";
import { tapeReelAmounts, tapeSVG } from "./tape";
import { loadingInputDisabled, type LoadingCardView } from "./loadingData";
import "../../styles/tokens.css";
import "../../components/components.css";
import "./loading.css";

/** SPEC-V5 §7.4：无氛围色工程的中性色。 */
export const LOADING_NEUTRAL_ATMOSPHERE: [number, number, number] = [200, 200, 196];

export interface LoadingCardProps {
  view: LoadingCardView;
  /** 失败/中断的恢复入口（省略 = 不放按钮；不放假按钮）。 */
  onRetry?: () => void;
  /** 磁带边长 px（≥48 才可用 tapeSVG，§7.3）。 */
  tapeSize?: number;
  /** 工程氛围色（磁带内部；省略 = 中性色）。 */
  atmosphere?: [number, number, number] | null;
  className?: string;
}

function loadingHeadline(view: LoadingCardView): string {
  const name = view.title.trim();
  if (view.phase === "completed") return name ? `已打开《${name}》` : "已打开工程";
  if (view.phase === "loading") return name ? `正在打开《${name}》` : "正在打开工程…";
  return name ? `正在打开《${name}》` : "正在打开工程";
}

export function LoadingCard({
  view,
  onRetry,
  tapeSize = 64,
  atmosphere,
  className
}: LoadingCardProps) {
  const phase = view.phase;
  const amounts = tapeReelAmounts(phase === "interrupted" ? "failed" : phase, view.progress);
  const tape = useMemo(
    () => tapeSVG(tapeSize, tapeSize, { L: amounts.L, R: amounts.R, c1: atmosphere ?? LOADING_NEUTRAL_ATMOSPHERE }),
    [tapeSize, amounts.L, amounts.R, atmosphere]
  );
  const headline = loadingHeadline(view);
  const disabled = loadingInputDisabled(view);
  const cls = ["v5-loading-card", `v5-loading-card--${phase}`, className].filter(Boolean).join(" ");
  return (
    <section
      className={cls}
      data-loading-card
      data-loading-phase={phase}
      data-loading-input-disabled={disabled ? "true" : "false"}
    >
      <header className="v5-loading-head">
        <div className="v5-loading-head-text">
          <h3 className="v5-loading-title" data-loading-title>
            {headline}
          </h3>
          {phase === "loading" ? (
            <p className="v5-loading-note">完成后可继续输入</p>
          ) : null}
          {phase === "completed" && view.resultNote ? (
            <p className="v5-loading-note" data-loading-result-note>
              {view.resultNote}
            </p>
          ) : null}
        </div>
        <span
          className="v5-loading-tape"
          data-loading-tape
          data-loading-tape-l={amounts.L.toFixed(2)}
          data-loading-tape-r={amounts.R.toFixed(2)}
          dangerouslySetInnerHTML={{ __html: tape }}
        />
      </header>

      {phase === "loading" && view.steps.length > 0 ? (
        <ul className="v5-loading-steps" data-loading-steps>
          {view.steps.map((step) => (
            <li
              key={step.id}
              className={step.status === "current" ? "v5-loading-step v5-loading-step--current" : "v5-loading-step"}
              data-loading-step-status={step.status}
            >
              {step.status === "current" ? (
                <span className="v5-loading-chip" data-graphic="loading-chip">
                  <StatusRing state="busy" size={16} />
                </span>
              ) : (
                <Icon name="i-done" size={16} className="v5-loading-done" />
              )}
              <span className="v5-loading-step-label">{step.label}</span>
              {step.duration ? <time className="v5-loading-step-time">{step.duration}</time> : null}
            </li>
          ))}
        </ul>
      ) : null}

      {phase === "failed" || phase === "interrupted" ? (
        <div className="v5-loading-block" data-loading-failure>
          <p className="v5-loading-reason" data-loading-failure-reason>
            {view.failureReason}
          </p>
          {onRetry ? (
            <button
              type="button"
              className="v5-btn v5-btn--secondary"
              onClick={onRetry}
              data-loading-retry
            >
              {phase === "interrupted" ? "重新打开" : "重试"}
            </button>
          ) : null}
        </div>
      ) : null}

      {phase === "completed" ? (
        <details className="v5-loading-history" data-loading-history={view.steps.length > 0 ? "available" : "empty"}>
          {view.steps.length > 0 ? <summary>装载轨迹</summary> : null}
          {view.steps.length > 0 ? (
            <ul className="v5-loading-steps" data-loading-steps-history>
              {view.steps.map((step) => (
                <li key={step.id} className="v5-loading-step" data-loading-step-status={step.status}>
                  <Icon name="i-done" size={16} className="v5-loading-done" />
                  <span className="v5-loading-step-label">{step.label}</span>
                  {step.duration ? <time className="v5-loading-step-time">{step.duration}</time> : null}
                </li>
              ))}
            </ul>
          ) : null}
        </details>
      ) : null}
    </section>
  );
}
