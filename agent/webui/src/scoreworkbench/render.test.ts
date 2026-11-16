// @vitest-environment jsdom
// SCORE-WORKBENCH-1 M1：VF5 渲染命中测试（jsdom env——369 spike 亲证：无 canvas
// 仅文本测量告警，渲染正常）。断言：SVG 非空/符头计数/行内单调/命中检测/延音渲染。
import { describe, expect, it } from "vitest";

import { deriveNotation } from "./derive";
import type { KernelNote, ScoreModel } from "./model";
import { hitTest, midiPitchToKey, renderScoreInto, rowAtY, snapToGrid, xToTickContinuous } from "./render";

function kernelNote(id: string, pitch: number, startBeats: number, lengthBeats: number): KernelNote {
  return { id, pitch, startBeats, lengthBeats };
}

const spikeModel: ScoreModel = {
  timeSignature: { numerator: 2, denominator: 4 },
  notes: [
    kernelNote("k1", 72, 0.0, 0.75),
    kernelNote("k2", 67, 0.75, 0.25),
    kernelNote("k3", 71, 1.5, 0.5),
    kernelNote("k4", 76, 2.5, 1.0),
    kernelNote("k5", 79, 3.5, 1.0)
  ]
};

function makeContainer(): HTMLDivElement {
  const container = document.createElement("div");
  document.body.appendChild(container);
  return container;
}

describe("VexFlow 5 render (jsdom, thin adapter)", () => {
  it("renders a non-empty SVG score for the lesson fixture", () => {
    const container = makeContainer();
    const result = renderScoreInto(container, spikeModel);
    expect(result.svg.length).toBeGreaterThan(2000);
    expect(container.querySelectorAll("svg").length).toBeGreaterThan(0);
  });

  it("notehead glyph count >= note events (every note symbol draws a head)", () => {
    const container = makeContainer();
    const result = renderScoreInto(container, spikeModel);
    const noteEvents = result.notation.events.filter((event) => event.kind === "note").length;
    const heads = (result.svg.match(/notehead/g) ?? []).length;
    expect(heads).toBeGreaterThanOrEqual(noteEvents);
  });

  it("tie glyph renders for tie groups (cross-barline pair included)", () => {
    const container = makeContainer();
    const result = renderScoreInto(container, spikeModel);
    expect(result.notation.tieGroups.length).toBe(2);
    expect(result.svg).toContain("tie");
  });

  it("tick->x rows are populated per measure and monotonic within a row", () => {
    const container = makeContainer();
    const result = renderScoreInto(container, spikeModel);
    expect(result.tickRows.size).toBe(3);
    for (const row of result.tickRows.values()) {
      expect(row.length).toBeGreaterThan(0);
      for (let i = 1; i < row.length; i++) {
        expect(row[i][1]).toBeGreaterThanOrEqual(row[i - 1][1] - 0.01);
      }
    }
  });

  it("hit test returns grid ticks inside a row and null outside rows", () => {
    const container = makeContainer();
    const result = renderScoreInto(container, spikeModel);
    const row = result.tickRows.get(0) ?? [];
    expect(row.length).toBeGreaterThan(0);
    const firstX = row[0][1];
    const bounds = result.staveBounds.get(0);
    expect(bounds).toBeDefined();
    const hit = hitTest(result, firstX, (bounds?.y ?? 0) + 20);
    expect(hit).not.toBeNull();
    expect(hit?.measure).toBe(0);
    expect(Number.isInteger(hit?.tick)).toBe(true);
    // 远离所有行的 y → null
    expect(hitTest(result, 100, -500)).toBeNull();
    expect(hitTest(result, 100, 100000)).toBeNull();
  });

  it("xToTickContinuous interpolates between anchors and clamps at row ends", () => {
    const container = makeContainer();
    const result = renderScoreInto(container, spikeModel);
    const row = result.tickRows.get(1) ?? [];
    if (row.length < 2) throw new Error("row too sparse for interpolation fixture");
    const [tickA, xa] = row[0];
    const [tickB, xb] = row[1];
    const mid = xToTickContinuous(row, (xa + xb) / 2);
    expect(mid).toBeGreaterThanOrEqual(tickA);
    expect(mid).toBeLessThanOrEqual(tickB);
    expect(xToTickContinuous(row, xa - 1000)).toBe(tickA);
    expect(xToTickContinuous(row, xb + 1000)).toBe(row[row.length - 1][0]);
  });

  it("snapToGrid rounds to integer ticks (16th grid)", () => {
    expect(snapToGrid(2.4)).toBe(2);
    expect(snapToGrid(2.6)).toBe(3);
    expect(snapToGrid(-1)).toBe(0);
  });

  it("midiPitchToKey maps 60=C4 and 72=C5", () => {
    expect(midiPitchToKey(60)).toBe("c/4");
    expect(midiPitchToKey(72)).toBe("c/5");
    expect(midiPitchToKey(61)).toBe("c#/4");
  });

  it("empty measure renders rests only (no crash, one row)", () => {
    const container = makeContainer();
    const result = renderScoreInto(container, { timeSignature: { numerator: 2, denominator: 4 }, notes: [] });
    expect(result.tickRows.size).toBe(1);
    expect(result.notation.events.every((event) => event.kind === "rest")).toBe(true);
    expect(rowAtY(result, (result.staveBounds.get(0)?.y ?? 0) + 10)).toBe(0);
  });

  it("render is deterministic across two renders of the same model (shape parity)", () => {
    const a = renderScoreInto(makeContainer(), spikeModel);
    const b = renderScoreInto(makeContainer(), spikeModel);
    expect(a.notation).toEqual(b.notation);
    expect(a.tickRows.size).toBe(b.tickRows.size);
    expect(deriveNotation(spikeModel)).toEqual(b.notation);
  });
});
