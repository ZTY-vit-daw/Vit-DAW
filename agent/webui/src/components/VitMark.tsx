import { Icon, type IconSize } from "./icons";

/**
 * Vit 品牌标 i-vit（SPEC-V5 §7.3.1 v5.1）：
 * 只用于应用图标、任务栏 Vit 条目、系统通知等 ≤32px 的 Vit 身份位置；
 * **不进入对话窗标题栏**（标题栏用 i-project + 工程名，或纯文本「Vit」）。
 * 状态叠加 = 右下角小状态点（变化中时状态点可以动）。
 */
export interface VitMarkProps {
  size?: IconSize;
  state?: "idle" | "busy" | "done" | "error";
  className?: string;
}

export function VitMark({ size = 20, state, className }: VitMarkProps) {
  const cls = ["v5-vit-mark", className].filter(Boolean).join(" ");
  return (
    <span className={cls} style={{ width: size, height: size }} data-vit-mark={state ?? "none"}>
      <Icon name="i-vit" size={size} />
      {state ? (
        <span className={`v5-id-dot v5-id-dot--${state}`} data-graphic="state-dot" aria-hidden="true" />
      ) : null}
    </span>
  );
}
