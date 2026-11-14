import type { SpriteSymbolId } from "../design/sprite";

/** SPEC-V5 §7.1：显示尺寸 16（最小）/ 20（界面默认）/ 24（强调）。 */
export type IconSize = 16 | 20 | 24;

export interface IconProps {
  name: SpriteSymbolId;
  size?: IconSize;
  className?: string;
}

/**
 * 雪碧图引用图标。fill/stroke 契约由 sprite.css 的 .v5-icon 提供；
 * 底面适配：容器需设置 --v5-icon-bg 为自身背景色（knockout/V 标内槽用）。
 * data-graphic 供渲染烟测做 ≥3:1 图形对比度检查。
 */
export function Icon({ name, size = 20, className }: IconProps) {
  const cls = ["v5-icon", className].filter(Boolean).join(" ");
  return (
    <svg
      className={cls}
      style={{ width: size, height: size }}
      data-icon-size={size}
      data-graphic="icon"
      aria-hidden="true"
    >
      <use href={`#${name}`} />
    </svg>
  );
}
