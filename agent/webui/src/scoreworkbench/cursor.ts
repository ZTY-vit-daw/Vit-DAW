// SCORE-WORKBENCH-1 M4：随谱游标——playhead 秒→beat→x。
// 单 tempo fail-honest 门：tempo 来自 music_context_echo 且 tempo_setting_count==1
// （kernel.ts 装载门已守），本模块只做换算；无 tempo/无快照=不画游标（不静默错位）。
// tick→x 用符号锚点分段线性（369 spike L3 亲证：行内单调+命中精确+拍间插值）。

import { GRID_PER_BEAT } from "./model";
import type { RenderResult, TickXRow } from "./render";

// playheadClipBeats：全局播放头秒 → clip 内相对拍（恒定 tempo；clip 外可为负/超长，
// 由调用方决定是否显示）。
export function playheadClipBeats(playheadSeconds: number, clipStartSeconds: number, tempoBPM: number): number {
  return ((playheadSeconds - clipStartSeconds) * tempoBPM) / 60;
}

// tickToX：行内 tick→x（锚点分段线性；锚点外夹逼首尾锚点 x）。
export function tickToX(row: TickXRow, tick: number): number {
  if (row.length === 0) {
    return 0;
  }
  if (tick <= row[0][0]) {
    return row[0][1];
  }
  const last = row[row.length - 1];
  if (tick >= last[0]) {
    return last[1];
  }
  for (let i = 1; i < row.length; i++) {
    if (row[i][0] >= tick) {
      const [tickA, xa] = row[i - 1];
      const [tickB, xb] = row[i];
      if (tickB <= tickA) {
        return xb;
      }
      return xa + ((tick - tickA) / (tickB - tickA)) * (xb - xa);
    }
  }
  return last[1];
}

export type CursorPosition = { measure: number; x: number; y: number; height: number; beat: number; tick: number };

// cursorPosition：clip 内相对拍 → 游标位置（小节行+x+y 几何）。beat 超出渲染范围 → null。
export function cursorPosition(
  result: RenderResult,
  clipBeats: number,
  beatsPerMeasureValue: number
): CursorPosition | null {
  if (!Number.isFinite(clipBeats) || clipBeats < 0) {
    return null;
  }
  const measureTicks = beatsPerMeasureValue * GRID_PER_BEAT;
  const tick = clipBeats * GRID_PER_BEAT;
  const measure = Math.floor(tick / measureTicks);
  const row = result.tickRows.get(measure);
  if (!row || row.length === 0) {
    return null;
  }
  const bounds = result.staveBounds.get(measure);
  return {
    measure,
    x: tickToX(row, tick),
    y: bounds?.y ?? 0,
    height: bounds?.height ?? 120,
    beat: clipBeats,
    tick
  };
}
