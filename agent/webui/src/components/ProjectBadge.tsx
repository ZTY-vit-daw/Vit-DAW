import type { ReactElement } from "react";
import { Icon, type IconSize } from "./icons";
import "../styles/tape.css";

/**
 * 磁带小标志 i-project（工程对象：工程卡、工程抽屉、标题栏、任务栏工程条目）。
 * SPEC-V5 §7.3.1 v5.1 修订：48px 以下使用本标志；≥48px 才可用冻结的 tapeSVG。
 * 状态叠加 = 右下角小状态点，不把整个标志染成状态色。
 *
 * UI-REC-TAPE-1（365）场用位修订（用户裁定 2026-11-15 第 1 条）：启动台工程列表
 * （recents 行 + 建议卡工程位）不适用 ≤48px 简化标志规则，改用下方 {@link TapeBadge}
 * 磁带 cassette 造型头像；本组件（i-project）保留给其余 ≤48px 场用位，清单见
 * SPEC-V5 §7.3.1 修订段。
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

/* =====================================================================
 * UI-REC-TAPE-1（365）：启动台磁带 cassette 造型头像（TapeBadge）。
 *
 * 造型冻结源 = `main-window-v3/SPEC.md` §3 `tapeSVG(w,h,a)`（逐字核对在卷，
 * 2026-11-15 领取时）：壳=内缩 2px·tint/氛围填充·2px 墨描边；窗=内缩 8px·高-26·
 * 暖纸·2px 墨描边；带轮=窗宽 28%/72%·r=min(窗高*0.3,22)·深盘·毂 0.32r·墨环；
 * 带量公式 d=r*(0.34+0.5*fill)（TapeReel）；底标=墨线 2px+浅墨 1px。
 * 冻结源几何在 20px 直绘时窗口宽=0（ww=bw-16<0），坐实 SPEC-V5 §7.3「<48px
 * 读不清」——小尺寸按卡面授权走「同造型等比简化档」（同拓扑：壳/窗/双带轮+毂/
 * 底标双线，令牌与带量公式逐字保留；比例锚定冻结源 48px 样张；24 网格·结构
 * 笔触 1.5px，符合 §7.1「16px 最小 1.5px」）。
 *
 * 氛围系统 = `tape_atmosphere.gd` 确定性移植（`launcher-route-mondrian-v2.html`
 * :474-499 逐字）：工程名 FNV-1a 哈希（含非 ASCII 补轮）→ 6 色对 → c1/c2/mid +
 * L/R 带量；壳填充 rgba(c1,0.62)（冻结公式）；无工程名=中性灰壳满盘（冻结源
 * null 氛围语义；§7.4 中性色语义在 §7.4 场景保留）。§7.4 三态带量（待机/
 * 进行中/完成）为后续磁带动效卡域，本卡只消费确定性 L/R。
 * ===================================================================== */

/** 冻结令牌（main-window-v3/SPEC.md §3 逐字）。 */
const TAPE_INK = "#090B0E";
const TAPE_WIN = "#FBF9F1";
const TAPE_HUB = "#FAFAF2";
const TAPE_ACC = "#1A1A1F";
const TAPE_TINT = "#E6E6E6";

type TapeRGB = readonly [number, number, number];

/** rgba 拼接（v2 rgba(c,a) 逐字移植）。 */
function tapeRGBA(c: TapeRGB, a: number): string {
  return `rgba(${c[0]},${c[1]},${c[2]},${a})`;
}

/** 氛围色对（tape_atmosphere.gd 逐字移植，launcher-route-mondrian-v2.html:474-480）。 */
const TAPE_ATMOS_PAIRS: readonly (readonly [TapeRGB, TapeRGB])[] = [
  [
    [0x73, 0xb8, 0xf5],
    [0x06, 0x5b, 0xc7],
  ],
  [
    [0xfa, 0xcc, 0x1a],
    [0x73, 0xb8, 0xf5],
  ],
  [
    [0x06, 0x5b, 0xc7],
    [0xfa, 0xcc, 0x1a],
  ],
  [
    [0x73, 0xb8, 0xf5],
    [0xfa, 0xcc, 0x1a],
  ],
  [
    [0x06, 0x5b, 0xc7],
    [0x73, 0xb8, 0xf5],
  ],
  [
    [0xfa, 0xcc, 0x1a],
    [0x06, 0x5b, 0xc7],
  ],
];

/** 供单元测试断言色对域（不进渲染路径）。 */
export const TAPE_ATMOS_PALETTE = TAPE_ATMOS_PAIRS;

/**
 * FNV-1a 含非 ASCII 补轮（v2 stableHash 逐字移植；tape_atmosphere.gd 同源）。
 * 同一工程名 → 同一氛围（确定性，无随机源）。
 */
export function tapeAtmosHash(s: string): number {
  let h = 0x811c9dc5;
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    h = (h ^ (c & 0xff)) >>> 0;
    h = Math.imul(h, 0x01000193) >>> 0;
    if (c > 0xff) {
      h = (h ^ ((c >> 8) & 0xff)) >>> 0;
      h = Math.imul(h, 0x01000193) >>> 0;
    }
  }
  return h >>> 0;
}

/** 磁带氛围（v2 atmos 逐字移植）：c1/c2 色对 + mid 混合 + L/R 带量（§7.4）。 */
export interface TapeAtmos {
  c1: TapeRGB;
  c2: TapeRGB;
  mid: TapeRGB;
  L: number;
  R: number;
}

/** 工程名 → 确定性磁带氛围（FNV-1a 哈希 → 6 色对 + L/R 带量）。 */
export function tapeAtmos(name: string): TapeAtmos {
  const h = tapeAtmosHash(name);
  const p = TAPE_ATMOS_PAIRS[h % 6];
  const c1 = p[0];
  const c2 = p[1];
  const mid: TapeRGB = [
    Math.round(c1[0] + (c2[0] - c1[0]) * 0.55),
    Math.round(c1[1] + (c2[1] - c1[1]) * 0.55),
    Math.round(c1[2] + (c2[2] - c1[2]) * 0.55),
  ];
  const clamp = (v: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, v));
  const L = clamp(0.25 + (h % 56) / 100, 0.25, 0.8);
  const R = clamp(1.05 - L, 0.25, 0.8);
  return { c1, c2, mid, L, R };
}

/** 单带轮 = 深盘 + 浅毂 + 墨环（冻结源 forEach 结构，顺序逐字）。 */
function tapeReel(
  key: string,
  cx: number,
  cy: number,
  r: number,
  discR: number
): ReactElement {
  return (
    <g key={key}>
      <circle cx={cx} cy={cy} r={discR} fill={TAPE_ACC} data-tape-reel />
      <circle cx={cx} cy={cy} r={r * 0.32} fill={TAPE_HUB} />
      <circle
        cx={cx}
        cy={cy}
        r={r}
        fill="none"
        stroke={TAPE_INK}
        strokeWidth={TAPE_STROKE_STRUCTURE}
        data-graphic="tape"
      />
    </g>
  );
}

/** 结构笔触（小尺寸等比简化档：24 网格 1.5px，符合 §7.1「16px 最小 1.5px」）。 */
const TAPE_STROKE_STRUCTURE = 1.5;
/** 底标副线笔触（冻结源 1px 浅墨）。 */
const TAPE_STROKE_DETAIL = 1;

/**
 * 小尺寸等比简化档（<48px 场用位）：同拓扑（壳/窗/双带轮+毂/底标双线），
 * 比例锚定冻结源 48px 样张——壳内缩 1.5（冻结 2/44≈4.5%）、窗宽 66.7% 壳宽
 * （冻结 28/44≈63.6%）、带轮 r=min(窗高*0.26,22)（冻结 0.3 系数按两轮环间隙
 * 校准，避免小网格下双环叠压失形）、底标主/副线间距 3（冻结 6/44≈13.6%）。
 * 令牌、带量公式 d=r*(0.34+0.5*fill)、毂比 0.32r、rgba(c1,0.62) 壳填充逐字保留。
 */
function tapeSVGSmall(w: number, h: number, a: TapeAtmos | null): ReactElement {
  const s = 24;
  const bx = 1.5;
  const by = 1.5;
  const bw = s - 3;
  const bh = s - 3;
  const wx = 5;
  const wy = 5;
  const ww = 14;
  const wh = 10;
  const cy = wy + wh / 2;
  const c1x = wx + ww * 0.28;
  const c2x = wx + ww * 0.72;
  const r = Math.min(wh * 0.26, 22);
  const d1 = a ? r * (0.34 + 0.5 * a.L) : r;
  const d2 = a ? r * (0.34 + 0.5 * a.R) : r;
  const labelY = by + bh - 6;
  const bodyFill = a ? tapeRGBA(a.c1, 0.62) : TAPE_TINT;
  return (
    <svg width={w} height={h} viewBox={`0 0 ${s} ${s}`} data-tape-svg role="presentation">
      <rect
        x={bx}
        y={by}
        width={bw}
        height={bh}
        fill={bodyFill}
        stroke={TAPE_INK}
        strokeWidth={TAPE_STROKE_STRUCTURE}
        data-tape-shell
        data-graphic="tape"
      />
      <rect
        x={wx}
        y={wy}
        width={ww}
        height={wh}
        fill={TAPE_WIN}
        stroke={TAPE_INK}
        strokeWidth={TAPE_STROKE_STRUCTURE}
        data-tape-window
        data-graphic="tape"
      />
      {tapeReel("L", c1x, cy, r, d1)}
      {tapeReel("R", c2x, cy, r, d2)}
      <line
        x1={bx + 4}
        y1={labelY}
        x2={bx + bw - 4}
        y2={labelY}
        stroke={TAPE_INK}
        strokeWidth={TAPE_STROKE_STRUCTURE}
        data-tape-label
        data-graphic="tape"
      />
      <line
        x1={bx + 4}
        y1={labelY + 3}
        x2={bx + bw * 0.55}
        y2={labelY + 3}
        stroke="rgba(31,36,43,0.6)"
        strokeWidth={TAPE_STROKE_DETAIL}
        data-tape-sublabel
      />
    </svg>
  );
}

/**
 * 冻结 tapeSVG 几何（main-window-v3/SPEC.md §3:30-51 逐字移植，结构不动；
 * ≥48px 场用位按 SPEC-V5 §7.3 使用本路径）。带轮 forEach/底标双线/描边宽度
 * （结构 2px）与冻结源逐字一致。
 */
function tapeSVGFrozen(w: number, h: number, a: TapeAtmos | null): ReactElement {
  const bx = 2;
  const by = 2;
  const bw = w - 4;
  const bh = h - 4;
  const wx = bx + 8;
  const wy = by + 8;
  const ww = bw - 16;
  const wh = bh - 26;
  const cy = wy + wh / 2;
  const c1x = wx + ww * 0.28;
  const c2x = wx + ww * 0.72;
  const r = Math.min(wh * 0.3, 22);
  const d1 = a ? r * (0.34 + 0.5 * a.L) : r;
  const d2 = a ? r * (0.34 + 0.5 * a.R) : r;
  const labelY = by + bh - 12;
  const bodyFill = a ? tapeRGBA(a.c1, 0.62) : TAPE_TINT;
  return (
    <svg width={w} height={h} viewBox={`0 0 ${w} ${h}`} data-tape-svg role="presentation">
      <rect
        x={bx}
        y={by}
        width={bw}
        height={bh}
        fill={bodyFill}
        stroke={TAPE_INK}
        strokeWidth={2}
        data-tape-shell
        data-graphic="tape"
      />
      <rect
        x={wx}
        y={wy}
        width={ww}
        height={wh}
        fill={TAPE_WIN}
        stroke={TAPE_INK}
        strokeWidth={2}
        data-tape-window
        data-graphic="tape"
      />
      <g>
        <circle cx={c1x} cy={cy} r={d1} fill={TAPE_ACC} data-tape-reel />
        <circle cx={c1x} cy={cy} r={r * 0.32} fill={TAPE_HUB} />
        <circle cx={c1x} cy={cy} r={r} fill="none" stroke={TAPE_INK} strokeWidth={2} data-graphic="tape" />
      </g>
      <g>
        <circle cx={c2x} cy={cy} r={d2} fill={TAPE_ACC} data-tape-reel />
        <circle cx={c2x} cy={cy} r={r * 0.32} fill={TAPE_HUB} />
        <circle cx={c2x} cy={cy} r={r} fill="none" stroke={TAPE_INK} strokeWidth={2} data-graphic="tape" />
      </g>
      <line
        x1={bx + 10}
        y1={labelY}
        x2={bx + bw - 10}
        y2={labelY}
        stroke={TAPE_INK}
        strokeWidth={2}
        data-tape-label
        data-graphic="tape"
      />
      <line
        x1={bx + 10}
        y1={labelY + 6}
        x2={bx + bw * 0.55}
        y2={labelY + 6}
        stroke="rgba(31,36,43,0.6)"
        strokeWidth={1}
        data-tape-sublabel
      />
    </svg>
  );
}

/**
 * 磁带 cassette 造型头像（启动台工程列表场用位，用户裁定 2026-11-15 第 1 条）。
 *
 * - `<48px`：24 网格等比简化档（同造型可辨：磁带盒轮廓=壳+窗+双带轮）。
 * - `≥48px`：冻结 tapeSVG 几何（SPEC-V5 §7.3）。
 * - `name`：工程名（经 props 传入，启动台消费位传 `entry.name`/`card.name` 或
 *   文件名派生）→ FNV-1a 哈希 → 氛围色（§7.4）；缺省=中性灰壳满盘。
 */
export interface TapeBadgeProps {
  /** 显示尺寸（px）。20/24 为启动台常规档；≥48 自动切冻结几何。 */
  size?: number;
  /** 工程名：氛围色哈希源（缺省 = 中性灰壳满盘）。 */
  name?: string;
  className?: string;
}

export function TapeBadge({ size = 24, name, className }: TapeBadgeProps) {
  const trimmed = (name ?? "").trim();
  const atmos = trimmed ? tapeAtmos(trimmed) : null;
  const cls = ["v5-tape-badge", className].filter(Boolean).join(" ");
  return (
    <span
      className={cls}
      style={{ width: size, height: size }}
      data-tape-badge={atmos ? "atmos" : "neutral"}
      aria-hidden="true"
    >
      {size >= 48 ? tapeSVGFrozen(size, size, atmos) : tapeSVGSmall(size, size, atmos)}
    </span>
  );
}
