// SCORE-WORKBENCH-1 M1：模型单元测试（网格校验 fail-honest/序列化往返/单位换算）。
// 环境注释见文件头——vitest 默认 node env；本文件不需 DOM。
import { describe, expect, it } from "vitest";

import {
  ModelError,
  beatsPerMeasure,
  beatsToTicks,
  deserializeModel,
  measureLengthTicks,
  serializeModel,
  ticksToBeats,
  validateModel,
  type ScoreModel
} from "./model";

function note(id: string, pitch: number, startBeats: number, lengthBeats: number): ScoreModel["notes"][number] {
  return { id, pitch, startBeats, lengthBeats };
}

const model219: ScoreModel = {
  timeSignature: { numerator: 2, denominator: 4 },
  notes: [note("k1", 72, 0, 0.75), note("k2", 67, 0.75, 0.25), note("k3", 71, 1.5, 0.5)]
};

describe("score model units", () => {
  it("beats<->ticks conversion is exact on the 16th grid", () => {
    expect(beatsToTicks(0.75)).toBe(3);
    expect(beatsToTicks(1.5)).toBe(6);
    expect(ticksToBeats(3)).toBe(0.75);
    expect(ticksToBeats(6)).toBe(1.5);
  });

  it("beatsPerMeasure supports denominator=4 family (2/4, 3/4, 4/4)", () => {
    expect(beatsPerMeasure({ numerator: 2, denominator: 4 })).toBe(2);
    expect(beatsPerMeasure({ numerator: 3, denominator: 4 })).toBe(3);
    expect(beatsPerMeasure({ numerator: 4, denominator: 4 })).toBe(4);
    expect(measureLengthTicks({ numerator: 4, denominator: 4 })).toBe(16);
  });

  it("fails honest on denominator != 4 (DERIVATION_RULES §0)", () => {
    expect(() => beatsPerMeasure({ numerator: 6, denominator: 8 })).toThrowError(ModelError);
  });

  it("validateModel rejects off-grid start/length instead of rounding", () => {
    const bad: ScoreModel = {
      timeSignature: { numerator: 2, denominator: 4 },
      notes: [note("x", 60, 0.1, 0.5)]
    };
    expect(() => validateModel(bad)).toThrowError(/grid_misaligned/);
  });

  it("validateModel rejects out-of-range pitch, negative span, empty id", () => {
    expect(() => validateModel({ timeSignature: { numerator: 2, denominator: 4 }, notes: [note("p", 128, 0, 1)] })).toThrowError(
      /bad_pitch/
    );
    expect(() => validateModel({ timeSignature: { numerator: 2, denominator: 4 }, notes: [note("n", 60, 0, -1)] })).toThrowError(
      /negative_length/
    );
    expect(() => validateModel({ timeSignature: { numerator: 2, denominator: 4 }, notes: [note("", 60, 0, 1)] })).toThrowError(
      /empty_id/
    );
  });

  it("accepts the canonical lesson fixture (dotted eighth + sixteenth + rests)", () => {
    expect(() => validateModel(model219)).not.toThrow();
  });

  it("serialize -> deserialize round-trips byte-stable (model identity)", () => {
    const json = serializeModel(model219);
    const back = deserializeModel(json);
    expect(serializeModel(back)).toBe(json);
    expect(back.notes.map((item) => item.id)).toEqual(["k1", "k2", "k3"]);
  });

  it("deserialize fails honest on malformed payload", () => {
    expect(() => deserializeModel("{not json")).toThrow();
    expect(() => deserializeModel(JSON.stringify({ timeSignature: { numerator: 2, denominator: 4 } }))).toThrowError(ModelError);
  });
});
