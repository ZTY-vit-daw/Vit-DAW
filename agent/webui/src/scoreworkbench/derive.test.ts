// SCORE-WORKBENCH-1 M1：确定性记谱推导测试（DERIVATION_RULES.md v1 锚定）。
// 断言族=369 spike L1/L2 亲证集（27 断言中的模型/推导面）+ 教材四要素矩阵
// （DERIVATION_RULES §5）+ 4/4、3/4 拍号面（DG1-A A4 的模型侧前置）。
import { describe, expect, it } from "vitest";

import { deriveNotation, eventsAtMeasure } from "./derive";
import { GRID_PER_BEAT, type KernelNote, type ScoreModel } from "./model";

const EIGHTH = 2;
const DOTTED_EIGHTH = 3;
const QUARTER = 4;
const SIXTEENTH = 1;

function kernelNote(id: string, pitch: number, startBeats: number, lengthBeats: number): KernelNote {
  return { id, pitch, startBeats, lengthBeats };
}

// 369 spike 同一测试内容（3 小节 2/4，四要素全覆盖）。
const spikeModel: ScoreModel = {
  timeSignature: { numerator: 2, denominator: 4 },
  notes: [
    kernelNote("k1", 72, 0.0, 0.75), // C5 附点八分（前长后短首半拍）
    kernelNote("k2", 67, 0.75, 0.25), // G4 十六分
    // 1.0-1.5 空档 → 八分休止
    kernelNote("k3", 71, 1.5, 0.5), // B4 八分
    kernelNote("k4", 76, 2.5, 1.0), // E5 弱起跨拍 → 八分+八分延音（切分）
    kernelNote("k5", 79, 3.5, 1.0) // G5 跨小节线延音
    // 4.5-6.0 空档 → 休止
  ]
};

function restsOf(model: ScoreModel): Array<[number, number]> {
  return deriveNotation(model)
    .events.filter((event) => event.kind === "rest")
    .map((event) => [event.tick, event.sizeSixteenths]);
}

describe("derivation R1-R3 (DERIVATION_RULES v1)", () => {
  const d0 = deriveNotation(spikeModel);

  it("L1-dotted: 拍内附点归并——附点八分单符号 @tick0（教材「前长后短」首半拍）", () => {
    const dotted = d0.events.filter((event) => event.kind === "note" && event.sizeSixteenths === DOTTED_EIGHTH);
    expect(dotted).toHaveLength(1);
    expect(dotted[0].kernelId).toBe("k1");
    expect(dotted[0].tick).toBe(0);
  });

  it("L1-rest-1/2: 休止填充表——1.0-1.5 拍空档=八分休止、4.5 拍起空档有八分休止", () => {
    const rests = d0.events.filter((event) => event.kind === "rest").map((event) => [event.tick, event.sizeSixteenths]);
    expect(rests).toContainEqual([4, EIGHTH]);
    expect(rests).toContainEqual([18, EIGHTH]);
  });

  it("L1-ties: 弱起切分延音链——延音组 2 个（k4 跨拍、k5 跨小节线）", () => {
    expect(d0.tieGroups).toHaveLength(2);
  });

  it("L1-tie-barline: k5 延音组跨小节线（tick14 在小节1、tick16 在小节2）", () => {
    const k5Group = d0.tieGroups.find((group) => group.length === 2 && group[0] === 14);
    expect(k5Group).toBeDefined();
    const at14 = d0.events.find((event) => event.tick === 14);
    const at16 = d0.events.find((event) => event.tick === 16);
    expect(at14?.measure).toBe(1);
    expect(at16?.measure).toBe(2);
    expect(at14?.tieNext).toBe(true);
    expect(at16?.tiePrev).toBe(true);
  });

  it("R1.3: 每段一个符号——全部事件 sizeSixteenths ∈ {1,2,3,4} 且段与段无缝覆盖", () => {
    for (const event of d0.events) {
      expect([1, 2, 3, 4]).toContain(event.sizeSixteenths);
    }
    // 覆盖检查：总 tick 覆盖 3 小节 × 8 tick
    const covered = new Set<number>();
    for (const event of d0.events) {
      for (let tick = event.tick; tick < event.tick + event.sizeSixteenths; tick++) {
        expect(covered.has(tick)).toBe(false);
        covered.add(tick);
      }
    }
    expect(covered.size).toBe(3 * 2 * GRID_PER_BEAT);
  });

  it("R3.3: 休止永不参与延音链", () => {
    for (const event of d0.events) {
      if (event.kind === "rest") {
        expect(event.tieNext).toBeUndefined();
        expect(event.tiePrev).toBeUndefined();
      }
    }
  });

  it("确定性：同输入同输出（两次推导深度相等）", () => {
    const again = deriveNotation(spikeModel);
    expect(again).toEqual(d0);
  });

  it("空模型=1 小节全休止（DERIVATION_RULES §4）", () => {
    const empty = deriveNotation({ timeSignature: { numerator: 2, denominator: 4 }, notes: [] });
    expect(empty.measures).toBe(1);
    expect(empty.events.every((event) => event.kind === "rest")).toBe(true);
    expect(empty.events.map((event) => event.sizeSixteenths)).toEqual([4, 4]);
  });
});

describe("edit -> re-derive roundtrip (spike L2 expectations, model level)", () => {
  it("e2 delete: 延音对消失；空档按拍线落 四分休止+八分休止", () => {
    const next: ScoreModel = JSON.parse(JSON.stringify(spikeModel));
    next.notes = next.notes.filter((item) => item.id !== "k4");
    const rd = deriveNotation(next);
    expect(rd.events.some((event) => event.kernelId === "k4")).toBe(false);
    expect(rd.events.some((event) => event.kind === "rest" && event.tick === 8 && event.sizeSixteenths === QUARTER)).toBe(true);
    expect(rd.events.some((event) => event.kind === "rest" && event.tick === 12 && event.sizeSixteenths === EIGHTH)).toBe(true);
    expect(rd.tieGroups).toHaveLength(1);
  });

  it("e3 resize: 跨小节线延音缩短后消失", () => {
    const next: ScoreModel = JSON.parse(JSON.stringify(spikeModel));
    const k5 = next.notes.find((item) => item.id === "k5");
    if (!k5) throw new Error("fixture broken");
    k5.lengthBeats = 0.5;
    const rd = deriveNotation(next);
    expect(rd.tieGroups).toHaveLength(1); // 只剩 k4
    expect(rd.events.filter((event) => event.kernelId === "k5")).toHaveLength(1);
  });

  it("e4 dot-toggle on the accumulated e1-e3 chain (spike order): 弱起附点 → 八分+十六分延音链 + 附点八分休止填充", () => {
    // spike 同序累积：e1 insert k6 → e2 delete k4 → e3 k5 缩短 → e4 k3 加附点。
    const next: ScoreModel = JSON.parse(JSON.stringify(spikeModel));
    next.notes.push(kernelNote("k6", 74, 1.0, 0.5));
    next.notes = next.notes.filter((item) => item.id !== "k4");
    const k5 = next.notes.find((item) => item.id === "k5");
    if (!k5) throw new Error("fixture broken");
    k5.lengthBeats = 0.5;
    const k3 = next.notes.find((item) => item.id === "k3");
    if (!k3) throw new Error("fixture broken");
    k3.lengthBeats = 0.75;
    const rd = deriveNotation(next);
    const k3Events = rd.events.filter((event) => event.kernelId === "k3");
    expect(k3Events.map((event) => [event.tick, event.sizeSixteenths])).toEqual([
      [6, EIGHTH],
      [8, SIXTEENTH]
    ]);
    expect(rd.tieGroups).toHaveLength(1); // 只剩 k3 延音对（k4 已删、k5 已缩短）
    expect(rd.events.some((event) => event.kind === "rest" && event.tick === 9 && event.sizeSixteenths === DOTTED_EIGHTH)).toBe(true);
    expect(rd.events.some((event) => event.kind === "rest" && event.tick === 12 && event.sizeSixteenths === EIGHTH)).toBe(true);
  });

  it("e4-pristine: 只加附点（无其他突变）时 1 tick 孤立空档落十六分休止", () => {
    const next: ScoreModel = JSON.parse(JSON.stringify(spikeModel));
    const k3 = next.notes.find((item) => item.id === "k3");
    if (!k3) throw new Error("fixture broken");
    k3.lengthBeats = 0.75;
    const rd = deriveNotation(next);
    expect(rd.events.some((event) => event.kind === "rest" && event.tick === 9 && event.sizeSixteenths === SIXTEENTH)).toBe(true);
    expect(rd.tieGroups).toHaveLength(3); // k3 新增延音对 + 原有 k4/k5 两组
  });

  it("e1 insert: 八分休止窗被音符替换", () => {
    const next: ScoreModel = JSON.parse(JSON.stringify(spikeModel));
    next.notes.push(kernelNote("k6", 74, 1.0, 0.5));
    const rd = deriveNotation(next);
    expect(rd.events.some((event) => event.kind === "rest" && event.tick === 4)).toBe(false);
    expect(rd.events.some((event) => event.kernelId === "k6" && event.tick === 4 && event.sizeSixteenths === EIGHTH)).toBe(true);
  });
});

describe("time signature family (A4 model-side prerequisite)", () => {
  it("4/4：四拍小节切分在每拍线，整小节空=四分休止 ×4", () => {
    const model: ScoreModel = { timeSignature: { numerator: 4, denominator: 4 }, notes: [kernelNote("a", 60, 0, 0.5), kernelNote("b", 62, 0.5, 0.5)] };
    const rd = deriveNotation(model);
    expect(rd.measures).toBe(1);
    expect(rd.events.filter((event) => event.kind === "rest").map((event) => event.sizeSixteenths)).toEqual([4, 4, 4]);
    expect(eventsAtMeasure(rd, 0)).toHaveLength(5);
  });

  it("3/4：三拍小节；跨小节线延音组两端分属小节 0/1", () => {
    const model: ScoreModel = {
      timeSignature: { numerator: 3, denominator: 4 },
      notes: [kernelNote("a", 60, 2.5, 1.0)]
    };
    const rd = deriveNotation(model);
    expect(rd.measures).toBe(2);
    const a = rd.events.filter((event) => event.kernelId === "a");
    expect(a.map((event) => [event.tick, event.measure, event.sizeSixteenths])).toEqual([
      [10, 0, 2],
      [12, 1, 2]
    ]);
    expect(a[0].tieNext).toBe(true);
  });

  it("4/4 原题计量样例：附点/切分/休止/延音四要素在 4/4 下同样成立", () => {
    const model: ScoreModel = {
      timeSignature: { numerator: 4, denominator: 4 },
      notes: [
        kernelNote("n1", 60, 0, 0.75),
        kernelNote("n2", 62, 0.75, 0.25),
        kernelNote("n3", 64, 1.5, 0.5),
        kernelNote("n4", 65, 2.5, 1.0),
        kernelNote("n5", 67, 3.5, 1.0)
      ]
    };
    const rd = deriveNotation(model);
    expect(rd.events.some((event) => event.kind === "note" && event.sizeSixteenths === DOTTED_EIGHTH && event.tick === 0)).toBe(true);
    expect(rd.events.some((event) => event.kind === "rest" && event.tick === 4 && event.sizeSixteenths === EIGHTH)).toBe(true);
    expect(rd.tieGroups.length).toBeGreaterThanOrEqual(2);
  });
});
