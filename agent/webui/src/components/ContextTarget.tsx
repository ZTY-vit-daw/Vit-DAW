import type { SpriteSymbolId } from "../design/sprite";
import { Icon } from "./icons";

/**
 * 上下文目标控件（对话里被引用的对象，如「鼓组总线」）。
 * 默认 = 浅底 1px 边；选中 = 浅黄底 + 2px 墨框 + 1px 外环，投影更低（v5.1）。
 */
export interface ContextTargetProps {
  label: string;
  icon?: SpriteSymbolId;
  selected?: boolean;
  onClick?: () => void;
  className?: string;
}

export function ContextTarget({
  label,
  icon = "i-bus",
  selected,
  onClick,
  className,
}: ContextTargetProps) {
  const cls = ["v5-context-target", selected ? "is-selected" : "", className]
    .filter(Boolean)
    .join(" ");
  return (
    <button type="button" className={cls} onClick={onClick} aria-pressed={selected}>
      <Icon name={icon} size={16} />
      <span className="v5-context-target-label">{label}</span>
    </button>
  );
}
