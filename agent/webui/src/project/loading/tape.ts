/*
 * UI-LOADING-1（347）P2 装载状态卡 · 磁带 icon（tapeSVG）。
 *
 * 来源冻结：`design/main-window-v3/SPEC.md` §3 tapeSVG 逐字移植（结构不动——
 * rect/circle/line 序列、坐标公式、描边宽度全部保持原样；只改尺寸与参数，
 * SPEC-V5 §7.4 红线）。原稿依赖的外部 rgba 助手在 SPEC.md 未附，按其调用形
 * rgba(a.c1, 0.62)（c1 = [r,g,b]）补常规实现。
 *
 * 带量语义（SPEC-V5 §7.4）：a.L / a.R 控制左右带盘带量：
 * - 待机：L=1, R=0.2
 * - 进行中（进度 p）：L=1-0.8p, R=0.2+0.8p
 * - 完成：L=0.2, R=1
 * a.c1 = 工程氛围色（[r,g,b]）；无氛围色（如《ProQ3 裸控实验》）用 [200,200,196]。
 * 磁带身份色只在磁带内部，不外溢（SPEC-V5 §4.5）。
 */

export interface TapeAtmos {
  /** 左带盘带量 0..1。 */
  L: number;
  /** 右带盘带量 0..1。 */
  R: number;
  /** 工程氛围色 [r,g,b]。 */
  c1: [number, number, number];
}

/** SPEC 外部依赖的常规助手：[r,g,b] + alpha → CSS rgba() 字符串。 */
function rgba(c: [number, number, number], alpha: number): string {
  return "rgba(" + c[0] + "," + c[1] + "," + c[2] + "," + alpha + ")";
}

var INK = "#090B0E",
  WIN = "#FBF9F1",
  HUB = "#FAFAF2",
  ACC = "#1A1A1F",
  TINT = "#E6E6E6";

/** tapeSVG 逐字移植（来源见头注释；仅允许改 w/h/a 参数，不得改结构）。 */
export function tapeSVG(w: number, h: number, a?: TapeAtmos | null): string {
  var bx = 2,
    by = 2,
    bw = w - 4,
    bh = h - 4;
  var wx = bx + 8,
    wy = by + 8,
    ww = bw - 16,
    wh = bh - 26;
  var cy = wy + wh / 2,
    c1x = wx + ww * 0.28,
    c2x = wx + ww * 0.72;
  var r = Math.min(wh * 0.3, 22);
  var d1 = a ? r * (0.34 + 0.5 * a.L) : r,
    d2 = a ? r * (0.34 + 0.5 * a.R) : r;
  var labelY = by + bh - 12;
  var bodyFill = a ? rgba(a.c1, 0.62) : TINT;
  var s =
    '<svg width="' + w + '" height="' + h + '" viewBox="0 0 ' + w + " " + h + '">';
  s += '<rect x="' + bx + '" y="' + by + '" width="' + bw + '" height="' + bh + '" fill="' + bodyFill + '"/>';
  s += '<rect x="' + bx + '" y="' + by + '" width="' + bw + '" height="' + bh + '" fill="none" stroke="' + INK + '" stroke-width="2"/>';
  s += '<rect x="' + wx + '" y="' + wy + '" width="' + ww + '" height="' + wh + '" fill="' + WIN + '"/>';
  s += '<rect x="' + wx + '" y="' + wy + '" width="' + ww + '" height="' + wh + '" fill="none" stroke="' + INK + '" stroke-width="2"/>';
  [
    [c1x, d1],
    [c2x, d2]
  ].forEach(function (reel) {
    s += '<circle cx="' + reel[0] + '" cy="' + cy + '" r="' + reel[1] + '" fill="' + ACC + '"/>';
    s += '<circle cx="' + reel[0] + '" cy="' + cy + '" r="' + r * 0.32 + '" fill="' + HUB + '"/>';
    s += '<circle cx="' + reel[0] + '" cy="' + cy + '" r="' + r + '" fill="none" stroke="' + INK + '" stroke-width="2"/>';
  });
  s += '<line x1="' + (bx + 10) + '" y1="' + labelY + '" x2="' + (bx + bw - 10) + '" y2="' + labelY + '" stroke="' + INK + '" stroke-width="2"/>';
  s += '<line x1="' + (bx + 10) + '" y1="' + (labelY + 6) + '" x2="' + (bx + bw * 0.55) + '" y2="' + (labelY + 6) + '" stroke="rgba(31,36,43,0.6)" stroke-width="1"/>';
  return s + "</svg>";
}

/** 带量帧映射（SPEC-V5 §7.4）：phase/progress → {L,R}。未知进度不伪造——无 p 停待机帧；失败冻结当前帧不回卷。 */
export function tapeReelAmounts(
  phase: "idle" | "loading" | "completed" | "failed",
  progress: number | null
): { L: number; R: number } {
  if (phase === "completed") return { L: 0.2, R: 1 };
  const hasProgress = (phase === "loading" || phase === "failed") && progress !== null && Number.isFinite(progress);
  if (!hasProgress) return { L: 1, R: 0.2 }; // 待机帧（未知/无进度时静止，不伪造推进）
  const p = Math.min(1, Math.max(0, progress as number));
  return { L: 1 - 0.8 * p, R: 0.2 + 0.8 * p };
}
