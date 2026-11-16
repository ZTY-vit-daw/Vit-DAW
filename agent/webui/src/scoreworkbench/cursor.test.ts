// @vitest-environment jsdom
// SCORE-WORKBENCH-1 M4：游标换算测试（秒→beat→x；锚点插值；范围外不画）。
import { describe, expect, it } from "vitest";

import { cursorPosition, playheadClipBeats, tickToX } from "./cursor";
import type { ScoreModel } from "./model";
import { renderScoreInto, type TickXRow } from "./render";

const model: ScoreModel = {
  timeSignature: { numerator: 2, denominator: 4 },
  notes: [
    { id: "k1", pitch: 72, startBeats: 0, lengthBeats: 0.75 },
    { id: "k2", pitch: 67, startBeats: 0.75, lengthBeats: 0.25 },
    { id: "k3", pitch: 71, startBeats: 1.5, lengthBeats: 0.5 },
    { id: "k4", pitch: 76, startBeats: 2.5, lengthBeats: 0.5 } // 第二小节存在
  ]
};

describe("playhead -> beat -> cursor (M4)", () => {
  it("playheadClipBeats converts seconds to clip-relative beats at constant tempo", () => {
    // 120bpm：1 拍=0.5s；clip 从 2s 起 → playhead 2.5s = clip 内 1 拍
    expect(playheadClipBeats(2.5, 2, 120)).toBeCloseTo(1, 10);
    expect(playheadClipBeats(2.0, 2, 120)).toBe(0);
    expect(playheadClipBeats(1.5, 2, 120)).toBe(-1); // clip 前（120bpm 下 0.5s=1 拍）
  });

  it("tickToX interpolates between symbol anchors and clamps beyond the row", () => {
    const row: TickXRow = [
      [0, 30],
      [4, 100],
      [8, 170]
    ];
    expect(tickToX(row, 0)).toBe(30);
    expect(tickToX(row, 4)).toBe(100);
    expect(tickToX(row, 2)).toBeCloseTo(65, 6); // 中点线性
    expect(tickToX(row, -5)).toBe(30);
    expect(tickToX(row, 99)).toBe(170);
  });

  it("cursorPosition maps a mid-clip beat onto the correct measure row with monotonic x", () => {
    const container = document.createElement("div");
    document.body.appendChild(container);
    const result = renderScoreInto(container, model);
    const c0 = cursorPosition(result, 0, 2);
    const c1 = cursorPosition(result, 1.25, 2);
    expect(c0?.measure).toBe(0);
    expect(c1?.measure).toBe(0);
    expect(c1 && c0 && c1.x > c0.x).toBe(true); // 行内单调
    // 小节边界：beat 2.0 → 小节 1
    const c2 = cursorPosition(result, 2, 2);
    expect(c2?.measure).toBe(1);
    // 渲染范围外 → null（clip 超出已推导小节）
    expect(cursorPosition(result, 99, 2)).toBeNull();
    expect(cursorPosition(result, -0.1, 2)).toBeNull();
  });

  it("cursor x at a symbol tick equals the symbol anchor x (hit precision, spike L3 mode)", () => {
    const container = document.createElement("div");
    document.body.appendChild(container);
    const result = renderScoreInto(container, model);
    const anchorX = result.tickRows.get(0)?.find(([tick]) => tick === 4)?.[1];
    expect(anchorX).toBeDefined();
    const cursor = cursorPosition(result, 1, 2);
    expect(cursor?.x).toBeCloseTo(anchorX as number, 3);
  });
});
