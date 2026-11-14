import { Icon, type IconSize } from "./icons";

/**
 * 磁带小标志 i-project（工程对象：工程卡、工程抽屉、标题栏、任务栏工程条目）。
 * SPEC-V5 §7.3.1 v5.1 修订：48px 以下使用本标志；≥48px 才可用冻结的 tapeSVG。
 * 状态叠加 = 右下角小状态点，不把整个标志染成状态色。
 */
export interface ProjectBadgeProps {
  size?: IconSize;
  /** 右下角状态点（可选，颜色按 SPEC §4.2）。 */
  state?: "idle" | "busy" | "done" | "error";
  className?: string;
}

export function ProjectBadge({ size = 20, state, className }: ProjectBadgeProps) {
  const cls = ["v5-project-badge", className].filter(Boolean).join(" ");
  return (
    <span className={cls} style={{ width: size, height: size }} data-project-badge={state ?? "none"}>
      <Icon name="i-project" size={size} />
      {state ? (
        <span className={`v5-id-dot v5-id-dot--${state}`} data-graphic="state-dot" aria-hidden="true" />
      ) : null}
    </span>
  );
}
