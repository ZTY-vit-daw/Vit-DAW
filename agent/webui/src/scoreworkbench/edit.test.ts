// SCORE-WORKBENCH-1 M2：编辑缓冲纯函数边界测试（paint 拆分/边界拒绝/dirty/diff/patch）。
import { describe, expect, it } from "vitest";

import {
  bufferDirty,
  bufferDiffSummary,
  buildApplyPatch,
  createEditBuffer,
  dragNote,
  insertRest,
  placeNote,
  setDuration,
  tieEstablish,
  tieRelease,
  toggleDot
} from "./edit";
import type { ScoreModel } from "./model";

const base: ScoreModel = {
  timeSignature: { numerator: 2, denominator: 4 },
  notes: [
    { id: "n1", pitch: 60, startBeats: 0, lengthBeats: 1 },
    { id: "n2", pitch: 62, startBeats: 1, lengthBeats: 0.5 }
  ]
};

describe("edit buffer ops (M2, pure)", () => {
  it("createEditBuffer starts clean; edits mark dirty with diff counts", () => {
    const buffer = createEditBuffer(base);
    expect(bufferDirty(buffer)).toBe(false);
    // tick6=[6,8) 是休止窗（n2=[4,6)），干净落符不覆盖
    const edited = placeNote(buffer, 72, 6, 2);
    expect(bufferDirty(edited)).toBe(true);
    expect(bufferDiffSummary(edited)).toEqual({ added: 1, removed: 0, changed: 0 });
    // 原缓冲不可变（base 不被污染）
    expect(bufferDirty(buffer)).toBe(false);
  });

  it("paint: placing over a spanning note splits it into two keepers", () => {
    const buffer = createEditBuffer(base);
    // n1=[0,4) ticks；在 [2,4) 放八分 → n1 截为 [0,2)
    const edited = placeNote(buffer, 72, 2, 2);
    const n1 = edited.working.notes.find((note) => note.id === "n1");
    expect(n1?.lengthBeats).toBe(0.5);
    expect(edited.working.notes).toHaveLength(3);
  });

  it("insertRest clears the span; deleteNote leaves the gap to rest derivation", () => {
    const buffer = createEditBuffer(base);
    const erased = insertRest(buffer, 4, 2);
    expect(erased.working.notes.some((note) => note.id === "n2")).toBe(false);
    const deleted = insertRest(buffer, 0, 4);
    expect(deleted.working.notes.some((note) => note.id === "n1")).toBe(false);
  });

  it("setDuration/toggleDot cycles via the dot table; 16th dot is a no-op", () => {
    const buffer = createEditBuffer(base);
    expect(toggleDot(buffer, "n2").working.notes.find((note) => note.id === "n2")?.lengthBeats).toBe(0.75);
    expect(toggleDot(createEditBuffer({ ...base, notes: [{ id: "s", pitch: 60, startBeats: 0, lengthBeats: 0.25 }] }), "s").working.notes[0]
      .lengthBeats).toBe(0.25);
    expect(setDuration(buffer, "n2", 4).working.notes.find((note) => note.id === "n2")?.lengthBeats).toBe(1);
  });

  it("tieEstablish rejects non-adjacent or different-pitch pairs (no hidden side effect)", () => {
    const buffer = createEditBuffer(base); // n1(C4)[0,1) 与 n2(D4)[1,1.5)：音高不同
    const rejected = tieEstablish(buffer, "n1", "n2");
    expect(rejected).toBe(buffer);
    const same: ScoreModel = {
      timeSignature: { numerator: 2, denominator: 4 },
      notes: [
        { id: "p", pitch: 60, startBeats: 0, lengthBeats: 0.5 },
        { id: "q", pitch: 60, startBeats: 1.25, lengthBeats: 0.5 }
      ]
    };
    expect(tieEstablish(createEditBuffer(same), "p", "q").working.notes).toHaveLength(2);
  });

  it("tieRelease only splits strictly inside the note", () => {
    const buffer = createEditBuffer(base);
    const n1 = buffer.working.notes.find((note) => note.id === "n1");
    expect(n1).toBeDefined();
    expect(tieRelease(buffer, "n1", 0)).toBe(buffer); // 起点=边界
    expect(tieRelease(buffer, "n1", 4)).toBe(buffer); // 终点=边界
    const split = tieRelease(buffer, "n1", 2);
    expect(split.working.notes.map((note): [string, number, number] => [note.id, note.startBeats, note.lengthBeats]).sort((a, b) => a[1] - b[1])).toEqual([
      ["n1", 0, 0.5],
      ["n1#split-2", 0.5, 0.5],
      ["n2", 1, 0.5]
    ]);
  });

  it("dragNote moves by grid and paints over the destination", () => {
    const buffer = createEditBuffer(base);
    const moved = dragNote(buffer, "n2", 2); // n2 → tick2，与 n1[0,4) 冲突 → n1 截短
    const n1 = moved.working.notes.find((note) => note.id === "n1");
    const n2 = moved.working.notes.find((note) => note.id === "n2");
    expect(n1?.lengthBeats).toBe(0.5);
    expect(n2?.startBeats).toBe(0.5);
  });

  it("buildApplyPatch emits one replace_region covering the clip, notes sorted by start", () => {
    const buffer = createEditBuffer(base);
    const edited = placeNote(buffer, 72, 6, 2);
    const patch = buildApplyPatch(edited, 8);
    expect(patch.op).toBe("replace_region");
    expect(patch.region_start).toBe(0);
    expect(patch.region_length).toBe(8);
    expect(patch.time_unit).toBe("beats");
    expect(patch.notes.map((note) => note.start)).toEqual([0, 1, 1.5]);
    expect(patch.notes.every((note) => !("id" in note))).toBe(true); // 内核自assign id，回读重绑
  });
});
