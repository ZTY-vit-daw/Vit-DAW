/*
 * SIM-MUSIC-WIDGET-1（池序 441）：装置侧 MIDI 只读解析器测试。
 *
 * 装置 fixture：coord/experiments/tutor-sim-2/assets/theme_twinkle_v0.mid
 * （只读消费；实测锚定 42 noteOn / 48 拍 / 拍号 4/4（nn=4 dd=2）/ 100 BPM /
 * division 480 / format 0 单轨——领取时 node 逐字节亲证）。
 * fail-closed 用例用测试内构造的最小 SMF 字节（不写任何文件）。
 */

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

import { parseMidiScore } from "./midiScore";
import { GRID_PER_BEAT } from "../scoreworkbench/model";

// 仓库根相对 vitest 运行 cwd（agent/webui）：webui→agent→root，再进 coord 装置目录。
const FIXTURE_REL = "../../coord/experiments/tutor-sim-2/assets/theme_twinkle_v0.mid";

function fixtureBytes(): Uint8Array {
  // 只读消费仓库装置 fixture（不写入）。仓库窄声明（vite-env.d.ts）：
  // readFileSync(path, encoding): string——MIDI 是二进制，用 latin1 读
  //（字节↔码位无损映射）再转 Uint8Array，不经文本重解码。
  const text = readFileSync(join(process.cwd(), FIXTURE_REL), "latin1");
  const bytes = new Uint8Array(text.length);
  for (let i = 0; i < text.length; i++) bytes[i] = text.charCodeAt(i) & 0xff;
  return bytes;
}

/* ---- 最小 SMF 构造（fail-closed 用例；只产字节不落盘） ---- */

function vlq(value: number): number[] {
  if (value < 0) throw new Error("negative vlq");
  const bytes = [value & 0x7f];
  let rest = value >> 7;
  while (rest > 0) {
    bytes.unshift((rest & 0x7f) | 0x80);
    rest >>= 7;
  }
  return bytes;
}

type TrackEvent = { delta: number; bytes: number[] };

function buildSMF(options: {
  format?: number;
  division?: number;
  tracks: TrackEvent[][];
}): Uint8Array {
  const format = options.format ?? 0;
  const division = options.division ?? 480;
  const header: number[] = [
    0x4d, 0x54, 0x68, 0x64, // "MThd"
    0, 0, 0, 6,
    (format >> 8) & 0xff, format & 0xff,
    0, options.tracks.length,
    (division >> 8) & 0xff, division & 0xff
  ];
  const parts: number[] = [...header];
  for (const events of options.tracks) {
    const body: number[] = [];
    for (const event of events) {
      body.push(...vlq(event.delta), ...event.bytes);
    }
    body.push(...vlq(0), 0xff, 0x2f, 0); // end of track
    parts.push(
      0x4d, 0x54, 0x72, 0x6b, // "MTrk"
      (body.length >> 24) & 0xff, (body.length >> 16) & 0xff, (body.length >> 8) & 0xff, body.length & 0xff,
      ...body
    );
  }
  return new Uint8Array(parts);
}

const NOTE_ON = (pitch: number, velocity = 90, channel = 0): number[] => [0x90 | channel, pitch, velocity];
const NOTE_OFF = (pitch: number, channel = 0): number[] => [0x80 | channel, pitch, 0];
const TIMESIG = (numerator: number, dd: number): number[] => [0xff, 0x58, 4, numerator, dd, 24, 8];
const TEMPO = (usPerQN: number): number[] => [0xff, 0x51, 3, (usPerQN >> 16) & 0xff, (usPerQN >> 8) & 0xff, usPerQN & 0xff];

/* ---- 装置 fixture 断言（验收锚点：42 音/48 拍/4-4 拍号） ---- */

describe("parseMidiScore — theme_twinkle_v0.mid 装置 fixture", () => {
  const doc = parseMidiScore(fixtureBytes());

  it("摘要：42 音 / 48 拍 / 100 BPM / 4/4", () => {
    expect(doc.summary.noteCount).toBe(42);
    expect(doc.summary.durationBeats).toBe(48);
    expect(doc.summary.tempoBPM).toBeCloseTo(100, 6); // tempo meta 600000 µs/qn
    expect(doc.summary.timeSignatureLabel).toBe("4/4");
  });

  it("模型：4/4 拍号、音符在 16 分网格、id 稳定、按 start 排序", () => {
    expect(doc.model.timeSignature).toEqual({ numerator: 4, denominator: 4 });
    expect(doc.model.notes.length).toBe(42);
    doc.model.notes.forEach((note, index) => {
      expect(note.id).toBe(`m${index + 1}`);
      const ticks = note.startBeats * GRID_PER_BEAT;
      expect(Math.abs(ticks - Math.round(ticks))).toBeLessThan(1e-9);
      const lenTicks = note.lengthBeats * GRID_PER_BEAT;
      expect(Math.abs(lenTicks - Math.round(lenTicks))).toBeLessThan(1e-9);
      expect(note.pitch).toBeGreaterThanOrEqual(0);
      expect(note.pitch).toBeLessThanOrEqual(127);
      expect(note.lengthBeats).toBeGreaterThan(0);
    });
    for (let i = 1; i < doc.model.notes.length; i++) {
      expect(doc.model.notes[i].startBeats).toBeGreaterThanOrEqual(doc.model.notes[i - 1].startBeats);
    }
  });

  it("48 拍 = 12 个 4/4 小节（最后一音不越界）", () => {
    const measureBeats = 4;
    expect(Math.round(doc.summary.durationBeats / measureBeats)).toBe(12);
    const last = doc.model.notes[doc.model.notes.length - 1];
    expect(last.startBeats + last.lengthBeats).toBeLessThanOrEqual(48 + 1e-9);
  });
});

/* ---- fail-closed 构造用例 ---- */

describe("parseMidiScore — fail-closed 语法面", () => {
  it("非 SMF 字节：缺 MThd 头", () => {
    expect(() => parseMidiScore(new Uint8Array([1, 2, 3]))).toThrow(/MThd/);
  });

  it("SMPTE division：不支持", () => {
    const bytes = buildSMF({ division: 0xe250, tracks: [[{ delta: 0, bytes: NOTE_ON(60) }, { delta: 480, bytes: NOTE_OFF(60) }]] });
    expect(() => parseMidiScore(bytes)).toThrow(/SMPTE/);
  });

  it("format 2：不支持", () => {
    const bytes = buildSMF({ format: 2, tracks: [[{ delta: 0, bytes: NOTE_ON(60) }, { delta: 480, bytes: NOTE_OFF(60) }]] });
    expect(() => parseMidiScore(bytes)).toThrow(/format 2/);
  });

  it("未闭合音符：fail-closed", () => {
    const bytes = buildSMF({ tracks: [[{ delta: 0, bytes: NOTE_ON(60) }]] });
    expect(() => parseMidiScore(bytes)).toThrow(/未闭合/);
  });

  it("同刻双音（和弦）：非单声部 fail-closed", () => {
    const bytes = buildSMF({
      tracks: [[
        { delta: 0, bytes: NOTE_ON(60) },
        { delta: 0, bytes: NOTE_ON(64) },
        { delta: 480, bytes: NOTE_OFF(60) },
        { delta: 0, bytes: NOTE_OFF(64) }
      ]]
    });
    expect(() => parseMidiScore(bytes)).toThrow(/单声部/);
  });

  it("网格外时值（三连音域）：validateModel fail-honest 显形", () => {
    // division=96：16 分 = 6 ticks；length 3 ticks = 0.03125 beat → 非 0.25 粒度。
    const bytes = buildSMF({
      division: 96,
      tracks: [[
        { delta: 0, bytes: NOTE_ON(60) },
        { delta: 3, bytes: NOTE_OFF(60) }
      ]]
    });
    expect(() => parseMidiScore(bytes)).toThrow(/grid_misaligned/);
  });

  it("多段拍号（值不同）：fail-closed", () => {
    const bytes = buildSMF({
      tracks: [[
        { delta: 0, bytes: TIMESIG(4, 2) },
        { delta: 0, bytes: NOTE_ON(60) },
        { delta: 480, bytes: TIMESIG(3, 2) },
        { delta: 0, bytes: NOTE_OFF(60) }
      ]]
    });
    expect(() => parseMidiScore(bytes)).toThrow(/多段拍号/);
  });

  it("截断 chunk：fail-closed", () => {
    const full = buildSMF({ tracks: [[{ delta: 0, bytes: NOTE_ON(60) }, { delta: 480, bytes: NOTE_OFF(60) }]] });
    expect(() => parseMidiScore(full.slice(0, full.length - 8))).toThrow();
  });
});

/* ---- 缺省与基础面 ---- */

describe("parseMidiScore — 缺省语义", () => {
  it("无拍号 meta：按 4/4 惯例；无 tempo meta：120 BPM", () => {
    const bytes = buildSMF({
      tracks: [[
        { delta: 0, bytes: NOTE_ON(72) },
        { delta: 480, bytes: NOTE_OFF(72) }
      ]]
    });
    const doc = parseMidiScore(bytes);
    expect(doc.model.timeSignature).toEqual({ numerator: 4, denominator: 4 });
    expect(doc.summary.tempoBPM).toBe(120);
    expect(doc.summary.noteCount).toBe(1);
    expect(doc.summary.durationBeats).toBe(1);
  });

  it("x/4 之外的拍号分母（如 6/8）：validateModel fail-honest", () => {
    const bytes = buildSMF({
      tracks: [[
        { delta: 0, bytes: TIMESIG(6, 3) },
        { delta: 0, bytes: NOTE_ON(72) },
        { delta: 480, bytes: NOTE_OFF(72) }
      ]]
    });
    expect(() => parseMidiScore(bytes)).toThrow(/bad_timesig|denominator/);
  });
});
