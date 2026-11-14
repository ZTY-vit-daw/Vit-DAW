import { Icon } from "./icons";
import { StatusRing } from "./StatusRing";

/** 宏卡状态（SPEC-V5 §8.3）：默认 / 试听中（黄）/ 已应用（蓝，带撤销）/ 失败（红，带原因）。 */
export type MacroCardState = "default" | "auditioning" | "applied" | "failed";

export interface MacroOption {
  id: string;
  /** 宏名（22/700）。 */
  name: string;
  /** 一行音乐性描述（14）。不写原始参数数值。 */
  description: string;
  /** 单一频响轮廓采样（0..1，左→右）。只保留这一种可视化。 */
  contour: number[];
}

export interface MacroCardProps {
  /** 对象名，如「鼓组总线」。 */
  object: string;
  /** 宏组名，如「染色」。 */
  name: string;
  /** 恰好 3 个选项（命名参数快照语义）。 */
  options: MacroOption[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  state: MacroCardState;
  /** 失败原因（failed 态必填）。 */
  failureReason?: string;
  onApply?: () => void;
  onUndo?: () => void;
  onAudition?: (id: string) => void;
  onCancelAudition?: () => void;
  onRetry?: () => void;
  onSaveSnapshot?: () => void;
  className?: string;
}

function Contour({ points, selected }: { points: number[]; selected: boolean }) {
  const n = points.length;
  if (n < 2) return null;
  const step = 100 / (n - 1);
  const d = points
    .map((v, i) => `${i === 0 ? "M" : "L"}${(i * step).toFixed(1)} ${(100 - v * 100).toFixed(1)}`)
    .join(" ");
  return (
    <svg
      className="v5-contour-svg"
      viewBox="0 0 100 100"
      preserveAspectRatio="none"
      aria-hidden="true"
    >
      {/* 中线参考（图形，非数据读数） */}
      <line x1="0" y1="50" x2="100" y2="50" className="v5-contour-midline" />
      <path
        d={d}
        className={selected ? "v5-contour-line is-selected" : "v5-contour-line"}
        fill="none"
        vectorEffect="non-scaling-stroke"
        data-graphic="contour-line"
      />
    </svg>
  );
}

/**
 * 宏卡。语义红线（IMPLEMENTATION-HANDOFF）：
 * - 「已选择」与「已应用」分离：切换选项只显示已选择；服务端真实成功后才显示已应用。
 * - 3 个选项、音乐性描述、单一频响轮廓；不暴露底层参数数值。
 * - 试听可取消；失败显示原因与重试，保留未应用事实。
 */
export function MacroCard({
  object,
  name,
  options,
  selectedId,
  onSelect,
  state,
  failureReason,
  onApply,
  onUndo,
  onAudition,
  onCancelAudition,
  onRetry,
  onSaveSnapshot,
  className,
}: MacroCardProps) {
  const selected = options.find((o) => o.id === selectedId) ?? null;
  const cls = ["v5-macro", `v5-macro--${state}`, className].filter(Boolean).join(" ");

  const flag =
    state === "auditioning" ? (
      <span className="v5-macro-flag v5-macro-flag--busy">
        <StatusRing state="busy" size={16} />
        试听中
      </span>
    ) : state === "applied" ? (
      <span className="v5-macro-flag v5-macro-flag--applied">
        <Icon name="i-done" size={16} />
        已应用 · 可撤销
      </span>
    ) : state === "failed" ? (
      <span className="v5-macro-flag v5-macro-flag--failed">
        <StatusRing state="error" size={16} />
        应用失败
      </span>
    ) : selected ? (
      <span className="v5-macro-flag v5-macro-flag--picked">已选择</span>
    ) : null;

  const cycle = (dir: 1 | -1) => {
    if (options.length === 0) return;
    const idx = options.findIndex((o) => o.id === selectedId);
    const next = idx < 0 ? 0 : (idx + dir + options.length) % options.length;
    onSelect(options[next].id);
  };

  return (
    <section className={cls} data-macro-state={state}>
      <header className="v5-macro-header">
        <h3 className="v5-macro-title">
          {object} · {name}
        </h3>
        {flag}
      </header>
      <div className="v5-macro-body">
        <div
          className="v5-macro-well"
          role="radiogroup"
          aria-label={`${object} ${name} 宏选项`}
          onKeyDown={(e) => {
            if (e.key === "ArrowDown" || e.key === "ArrowRight") {
              e.preventDefault();
              cycle(1);
            } else if (e.key === "ArrowUp" || e.key === "ArrowLeft") {
              e.preventDefault();
              cycle(-1);
            }
          }}
        >
          {options.map((o) => {
            const isSel = o.id === selectedId;
            return (
              <button
                key={o.id}
                type="button"
                role="radio"
                aria-checked={isSel}
                className={`v5-macro-option${isSel ? " is-selected" : ""}`}
                onClick={() => onSelect(o.id)}
              >
                <span className="v5-macro-option-name">{o.name}</span>
                <span className="v5-macro-option-desc">{o.description}</span>
                <span className="v5-macro-option-dot" aria-hidden="true" />
              </button>
            );
          })}
        </div>
        <div className="v5-macro-pane" data-graphic="contour">
          <Contour points={(selected ?? options[0]).contour} selected={selected !== null} />
        </div>
        <div className="v5-macro-actions">
          <button
            type="button"
            className="v5-btn v5-btn--primary"
            disabled={state !== "default" || !selected}
            onClick={onApply}
          >
            <span>应用</span>
          </button>
          <button type="button" className="v5-btn v5-btn--text" onClick={onSaveSnapshot}>
            <span>存为我的快照</span>
          </button>
          <span className="v5-macro-aux">
            {state === "auditioning" ? (
              <button type="button" className="v5-btn v5-btn--secondary" onClick={onCancelAudition}>
                <Icon name="i-stop" size={16} />
                <span>取消试听</span>
              </button>
            ) : state === "failed" ? (
              <>
                <span className="v5-macro-reason">{failureReason}</span>
                <button type="button" className="v5-btn v5-btn--secondary" onClick={onRetry}>
                  <span>重试</span>
                </button>
              </>
            ) : (
              <button
                type="button"
                className="v5-btn v5-btn--secondary"
                disabled={!selected}
                onClick={() => selected && onAudition?.(selected.id)}
              >
                <Icon name="i-play" size={16} />
                <span>试听</span>
              </button>
            )}
          </span>
          {state === "applied" ? (
            <button type="button" className="v5-btn v5-btn--text" onClick={onUndo}>
              <span>撤销</span>
            </button>
          ) : null}
        </div>
      </div>
    </section>
  );
}
