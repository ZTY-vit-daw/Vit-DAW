/*
 * SIM-MUSIC-WIDGET-1（池序 441）：装置侧 MIDI 材料文件只读解析器（纯函数）。
 *
 * 数据源裁定（卡面领取节）：tutor workflow_data.materials 只含摘要（Go 侧
 * tutorsim.go 不下发音符序列），按卡面授权走「装置侧材料文件只读解析」——
 * 本文件是 UI 侧（webui 域内）的解析面，不落新后端端点、不改 Go/内核；
 * live 装载链内核回读（createAgentScoreBridge.reload，374 既有协议）由调用方
 * 接线（ProjectSurface tutorScoreLoader 接缝），后续真栈卡接入。
 *
 * 语法面：与 agent/internal/tutorsim/midi.go 同款既证集（SMF format 0/1、
 * metrical division、delta VLQ + running status、note on/off 配对），另解析
 * 拍号 meta 0x58 与 tempo meta 0x51（Go summary 不含拍号，谱面渲染需要）。
 * 确定性：beats=ticks/division；音符按 (start, pitch) 排序（同 Go），id 用
 * 排序序号（m1..mN）；网格/音域/时值校验复用 scoreworkbench/model.ts 的
 * validateModel（fail-honest），单声部守门同 kernel.reload（同刻多音 fail-closed）。
 */

import { GRID_PER_BEAT, validateModel, type ScoreModel } from "../scoreworkbench/model";

export type MidiScoreSummary = {
  noteCount: number;
  durationBeats: number;
  tempoBPM: number;
  timeSignatureLabel: string;
};

export type MidiScoreDocument = {
  model: ScoreModel;
  summary: MidiScoreSummary;
};

const DEFAULT_TEMPO_BPM = 120; // SMF 缺省（500000 µs/qn），与 tutorsim/midi.go 同缺省

function u16(data: Uint8Array, pos: number): number {
  return (data[pos] << 8) | data[pos + 1];
}

function u32(data: Uint8Array, pos: number): number {
  return ((data[pos] << 24) | (data[pos + 1] << 16) | (data[pos + 2] << 8) | data[pos + 3]) >>> 0;
}

function ascii(data: Uint8Array, start: number, end: number): string {
  let out = "";
  for (let i = start; i < end; i++) out += String.fromCharCode(data[i]);
  return out;
}

// readVLQ：delta time 可变长数量（≤4 字节），失败 fail-closed。
function readVLQ(data: Uint8Array, pos: number): [value: number, next: number] {
  let value = 0;
  for (let i = 0; i < 4; i++) {
    if (pos >= data.length) throw new Error("midi score: VLQ 截断");
    const byte = data[pos];
    pos += 1;
    value = (value << 7) | (byte & 0x7f);
    if ((byte & 0x80) === 0) return [value, pos];
  }
  throw new Error("midi score: VLQ 超过 4 字节");
}

// parseMidiScore：SMF 字节 → 谱面模型 + 确定性摘要。
// 只读纯语法映射，不做音乐推断；不支持 format 2 / SMPTE division / 未闭合音符
// / 网格外时值 / 非单声部（全部 fail-closed，显形不乱补）。
export function parseMidiScore(data: Uint8Array): MidiScoreDocument {
  if (data.length < 14 || ascii(data, 0, 4) !== "MThd") {
    throw new Error("midi score: 缺 MThd 头");
  }
  const headerLen = u32(data, 4);
  if (headerLen < 6 || 8 + headerLen > data.length) {
    throw new Error("midi score: header 长度非法");
  }
  const format = u16(data, 8);
  const division = u16(data, 12);
  if (format > 1) {
    throw new Error(`midi score: 不支持 format ${format}（只认 0/1）`);
  }
  if (division === 0) {
    throw new Error("midi score: division=0");
  }
  if ((division & 0x8000) !== 0) {
    throw new Error(`midi score: 不支持 SMPTE division（0x${division.toString(16).padStart(4, "0")}）`);
  }

  let pos = 8 + headerLen;
  let tempoUSPerQN = 0;
  let timeSignature: { numerator: number; denominator: number } | null = null;
  let tracks = 0;
  const raw: RawNote[] = [];
  while (pos < data.length) {
    if (pos + 8 > data.length) {
      throw new Error("midi score: chunk 头截断");
    }
    const chunkType = ascii(data, pos, pos + 4);
    const chunkLen = u32(data, pos + 4);
    pos += 8;
    if (pos + chunkLen > data.length) {
      throw new Error(`midi score: chunk ${chunkType} 长度越界`);
    }
    const bodyStart = pos;
    const bodyEnd = pos + chunkLen;
    pos = bodyEnd;
    if (chunkType !== "MTrk") continue; // 未知 chunk 跳过（SMF 规范允许）
    tracks += 1;
    const track = parseMidiTrack(data, bodyStart, bodyEnd, division);
    raw.push(...track.notes);
    if (tempoUSPerQN === 0 && track.tempoUSPerQN > 0) {
      tempoUSPerQN = track.tempoUSPerQN;
    }
    if (track.timeSignature) {
      if (timeSignature && (timeSignature.numerator !== track.timeSignature.numerator || timeSignature.denominator !== track.timeSignature.denominator)) {
        throw new Error("midi score: 多段拍号（v1 谱面渲染需恒定拍号，fail-closed）");
      }
      timeSignature = track.timeSignature;
    }
  }
  if (tracks === 0) {
    throw new Error("midi score: 无 MTrk 轨道");
  }

  const notes = raw.sort((a, b) => a.startBeats - b.startBeats || a.pitch - b.pitch);
  // 单声部守门（同 kernel.reload：同刻多音 = 和弦，v1 不支持）。
  const byTick = new Map<number, number>();
  for (const note of notes) {
    const tick = Math.round(note.startBeats * GRID_PER_BEAT);
    byTick.set(tick, (byTick.get(tick) ?? 0) + 1);
  }
  for (const [tick, count] of byTick) {
    if (count > 1) {
      throw new Error(`midi score: 非单声部（tick ${tick} 有 ${count} 个同刻音符）——v1 不支持和弦`);
    }
  }

  const model: ScoreModel = {
    timeSignature: timeSignature ?? { numerator: 4, denominator: 4 }, // SMF 无拍号 meta 时按 4/4 惯例
    notes: notes.map((note, index) => ({ id: `m${index + 1}`, pitch: note.pitch, startBeats: note.startBeats, lengthBeats: note.lengthBeats }))
  };
  validateModel(model); // 网格/音域/时值 fail-honest 校验（scoreworkbench/model.ts）

  let duration = 0;
  for (const note of model.notes) {
    if (note.startBeats + note.lengthBeats > duration) duration = note.startBeats + note.lengthBeats;
  }
  const tempoBPM = tempoUSPerQN > 0 ? 60_000_000 / tempoUSPerQN : DEFAULT_TEMPO_BPM;
  return {
    model,
    summary: {
      noteCount: model.notes.length,
      durationBeats: duration,
      tempoBPM,
      timeSignatureLabel: `${model.timeSignature.numerator}/${model.timeSignature.denominator}`
    }
  };
}

type RawNote = { pitch: number; startBeats: number; lengthBeats: number };

function parseMidiTrack(
  data: Uint8Array,
  start: number,
  end: number,
  division: number
): { notes: RawNote[]; tempoUSPerQN: number; timeSignature: { numerator: number; denominator: number } | null } {
  let pos = start;
  let tick = 0;
  let runningStatus = 0;
  let tempoUSPerQN = 0;
  let timeSignature: { numerator: number; denominator: number } | null = null;
  type OpenNote = { startTick: number; velocity: number };
  const open = new Map<number, OpenNote>(); // key=(channel<<7)|pitch
  const notes: RawNote[] = [];

  const closeNote = (key: number, endTick: number) => {
    const opened = open.get(key);
    if (!opened) return; // 未配对 note off 忽略（同 tutorsim/midi.go closeNote）
    let lengthTicks = endTick - opened.startTick;
    if (lengthTicks < 0) lengthTicks = 0;
    notes.push({
      pitch: key & 0x7f,
      startBeats: opened.startTick / division,
      lengthBeats: lengthTicks / division
    });
    open.delete(key);
  };

  while (pos < end) {
    const [delta, afterDelta] = readVLQ(data, pos);
    pos = afterDelta;
    tick += delta;
    if (pos >= end) throw new Error("midi score: 事件状态字节截断");
    const first = data[pos];
    let status: number;
    if (first & 0x80) {
      status = first;
      runningStatus = first;
      pos += 1;
    } else {
      if (runningStatus === 0) throw new Error("midi score: running status 前缺状态字节");
      status = runningStatus;
    }
    if (status === 0xff) {
      // meta：type + VLQ length + data
      if (pos >= end) throw new Error("midi score: meta 事件截断");
      const metaType = data[pos];
      pos += 1;
      const [metaLen, afterMetaLen] = readVLQ(data, pos);
      pos = afterMetaLen;
      if (pos + metaLen > end) throw new Error(`midi score: meta 数据越界（type 0x${metaType.toString(16).padStart(2, "0")}）`);
      if (metaType === 0x51 && metaLen === 3) {
        tempoUSPerQN = (data[pos] << 16) | (data[pos + 1] << 8) | data[pos + 2];
      } else if (metaType === 0x58 && metaLen >= 4) {
        // nn/dd/cc/bb：dd=2 的负对数（0xE0 低半字节域）
        const numerator = data[pos];
        const denominator = 2 ** data[pos + 1];
        if (timeSignature && (timeSignature.numerator !== numerator || timeSignature.denominator !== denominator)) {
          throw new Error("midi score: 多段拍号（v1 谱面渲染需恒定拍号，fail-closed）");
        }
        timeSignature = { numerator, denominator };
      }
      pos += metaLen;
    } else if (status === 0xf0 || status === 0xf7) {
      const [length, afterLength] = readVLQ(data, pos);
      pos = afterLength;
      if (pos + length > end) throw new Error("midi score: SysEx 数据越界");
      pos += length;
    } else if ((status & 0xf0) === 0x90) {
      if (pos + 2 > end) throw new Error("midi score: note on 数据截断");
      const pitch = data[pos];
      const velocity = data[pos + 1];
      pos += 2;
      const key = ((status & 0x0f) << 7) | pitch;
      if (velocity === 0) {
        closeNote(key, tick);
      } else if (open.has(key)) {
        throw new Error(`midi score: 同一音符重复开启（channel ${status & 0x0f} pitch ${pitch}）`);
      } else {
        open.set(key, { startTick: tick, velocity });
      }
    } else if ((status & 0xf0) === 0x80) {
      if (pos + 2 > end) throw new Error("midi score: note off 数据截断");
      const pitch = data[pos];
      pos += 2;
      closeNote(((status & 0x0f) << 7) | pitch, tick);
    } else if ((status & 0xf0) === 0xa0 || (status & 0xf0) === 0xb0 || (status & 0xf0) === 0xe0) {
      if (pos + 2 > end) throw new Error("midi score: 通道事件数据截断");
      pos += 2;
    } else if ((status & 0xf0) === 0xc0 || (status & 0xf0) === 0xd0) {
      if (pos + 1 > end) throw new Error("midi score: 通道事件数据截断");
      pos += 1;
    } else {
      throw new Error(`midi score: 不支持的状态字节 0x${status.toString(16).padStart(2, "0")}`);
    }
  }
  if (open.size > 0) {
    throw new Error(`midi score: ${open.size} 个音符未闭合（fail-closed）`);
  }
  return { notes, tempoUSPerQN, timeSignature };
}
