import { Icon } from "./icons";

/**
 * 动作卡（SPEC-V5 §8.4）：Vit 准备做或已经做了一件改动。
 * 必须表达后果等级：可撤销（蓝）/ 需确认（黄，附确认取消）/ 不可逆（红确认按钮）。
 * 用状态图标加一个词表达等级，不用色条。
 */
export type Consequence = "reversible" | "confirm" | "irreversible";

export interface ActionCardProps {
  title: string;
  description: string;
  consequence: Consequence;
  confirmLabel?: string;
  onConfirm?: () => void;
  onCancel?: () => void;
  className?: string;
}

const CONSEQUENCE_META: Record<
  Consequence,
  { icon: "i-done" | "i-busy" | "i-alert"; word: string; tag: string }
> = {
  reversible: { icon: "i-done", word: "可撤销", tag: "v5-action-tag--blue" },
  confirm: { icon: "i-busy", word: "需要你确认", tag: "v5-action-tag--yellow" },
  irreversible: { icon: "i-alert", word: "不可逆", tag: "v5-action-tag--red" },
};

export function ActionCard({
  title,
  description,
  consequence,
  confirmLabel = "确认",
  onConfirm,
  onCancel,
  className,
}: ActionCardProps) {
  const meta = CONSEQUENCE_META[consequence];
  const cls = ["v5-action-card", `v5-action-card--${consequence}`, className]
    .filter(Boolean)
    .join(" ");
  return (
    <section className={cls} data-action-consequence={consequence}>
      <h3 className="v5-action-title">{title}</h3>
      <p className="v5-action-desc">{description}</p>
      <span className={`v5-action-tag ${meta.tag}`}>
        <Icon name={meta.icon} size={16} />
        <span>{meta.word}</span>
      </span>
      <div className="v5-action-buttons">
        {consequence === "reversible" ? (
          <button type="button" className="v5-btn v5-btn--primary" onClick={onConfirm}>
            <span>{confirmLabel}</span>
          </button>
        ) : consequence === "confirm" ? (
          <>
            <button type="button" className="v5-btn v5-btn--primary" onClick={onConfirm}>
              <span>{confirmLabel}</span>
            </button>
            <button type="button" className="v5-btn v5-btn--text" onClick={onCancel}>
              <span>取消</span>
            </button>
          </>
        ) : (
          <>
            <button type="button" className="v5-btn v5-btn--danger" onClick={onConfirm}>
              <span>{confirmLabel}</span>
            </button>
            <button type="button" className="v5-btn v5-btn--text" onClick={onCancel}>
              <span>取消</span>
            </button>
          </>
        )}
      </div>
    </section>
  );
}
