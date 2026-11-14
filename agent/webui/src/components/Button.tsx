import type { SpriteSymbolId } from "../design/sprite";
import { Icon } from "./icons";

/** SPEC-V5 §4.3：主按钮墨底（不得用红色）、次按钮纸底墨边、危险按钮红底纸字。 */
export type ButtonVariant = "primary" | "secondary" | "danger" | "text";

export interface ButtonProps {
  variant?: ButtonVariant;
  icon?: SpriteSymbolId;
  disabled?: boolean;
  onClick?: () => void;
  children: React.ReactNode;
  className?: string;
}

/** 交互控件圆角 4px；主按钮 = 墨底纸字；红色只用于不可逆动作（danger）。 */
export function Button({
  variant = "secondary",
  icon,
  disabled,
  onClick,
  children,
  className,
}: ButtonProps) {
  const cls = ["v5-btn", `v5-btn--${variant}`, className].filter(Boolean).join(" ");
  return (
    <button type="button" className={cls} disabled={disabled} onClick={onClick}>
      {icon ? <Icon name={icon} size={16} className="v5-btn-icon" /> : null}
      <span>{children}</span>
    </button>
  );
}
