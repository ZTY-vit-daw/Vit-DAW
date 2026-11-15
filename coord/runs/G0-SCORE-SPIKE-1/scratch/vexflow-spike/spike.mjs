// G0-SCORE-SPIKE-1 / Spike A: VexFlow 5 + 自研 beat-grid 记谱模型
// 零栈：node 无头（jsdom 挂 DOM）；不碰生产 src/；npm 包仅装于本 scratch。
// 断言全部确定性；任何断言失败 -> process.exit(1)。
//
// 腿：
//  L1 模型+记谱推导：附点/切分/休止/延音（含跨小节线延音）的确定性推导
//  L2 编辑->序列化->重解析->重推导->重绘 往返（insert/delete/resize/dot-toggle 四操作）
//  L3 随谱播放游标：tick->x 查找表单调性+命中精度
//  L4 MIDI 往返：模型->SMF(PPQ=48)->解析回模型，音高/起点/长度集合相等

import { JSDOM } from 'jsdom';
import fs from 'node:fs';
import path from 'node:path';
import assert from 'node:assert/strict';

const ART = path.join(import.meta.dirname, 'artifacts');
fs.mkdirSync(ART, { recursive: true });

// ---------- 0. jsdom 环境 ----------
const dom = new JSDOM('<!DOCTYPE html><html><body></body></html>');
globalThis.document = dom.window.document;
globalThis.window = dom.window;

const results = { legs: {}, timings: {}, assertions: 0 };
function ok(name, cond, detail) {
  results.assertions += 1;
  if (!cond) throw new Error(`ASSERT FAIL [${name}] ${detail ?? ''}`);
}

// ---------- 1. 记谱模型（beat-grid，16 分网格；单位=16 分音符数 tick=1/4 beat） ----------
const GRID_PER_BEAT = 4; // 16th 分辨率（教材四类节奏含 16 分：前长后短=附点8+16）
const SIXTEENTH = 1, EIGHTH = 2, DOTTED_EIGHTH = 3, QUARTER = 4, DOTTED_QUARTER = 6, HALF = 8;
// 允许的单一记谱符号长度（16 分数）-> vexflow 时值
const SYMBOL = {
  [SIXTEENTH]: { dur: '16', dots: 0 },
  [EIGHTH]: { dur: '8', dots: 0 },
  [DOTTED_EIGHTH]: { dur: '8', dots: 1 },
  [QUARTER]: { dur: 'q', dots: 0 },
  [DOTTED_QUARTER]: { dur: 'q', dots: 1 },
  [HALF]: { dur: 'w', dots: 0 }, // 仅占位（2/4 内不会用到；半音符在 2/4 为整小节）
};
const SIZES = [HALF, DOTTED_QUARTER, QUARTER, DOTTED_EIGHTH, EIGHTH, SIXTEENTH]; // greedy 从大到小

// 内核形状的音符（与 get_midi_clip_notes 回读对齐：id/pitch/start/length，beats 相对 clip）
function kernelNote(id, pitch, startBeats, lenBeats) {
  return { id, pitch, start: startBeats * GRID_PER_BEAT, length: lenBeats * GRID_PER_BEAT };
}
const model = {
  timeSignature: { numerator: 2, denominator: 4 },
  beatsPerMeasure: 2,
  notes: [
    kernelNote('k1', 72, 0.0, 0.75),  // C5 附点八分（教材"前长后短"首半拍）
    kernelNote('k2', 67, 0.75, 0.25), // G4 十六分
    // 1.0-1.5 空档 -> 八分休止
    kernelNote('k3', 71, 1.5, 0.5),   // B4 八分
    kernelNote('k4', 76, 2.5, 1.0),   // E5 弱起跨拍 -> 八分+八分延音（切分）
    kernelNote('k5', 79, 3.5, 1.0),   // G5 跨小节线延音（第二小节末->第三小节首）
    // 4.5-6.0 空档 -> 休止
  ],
};

// ---------- 2. 确定性记谱推导（mini-W18：内核平音符 -> 记谱符号+延音链） ----------
// 规则（v0，spike 级简化，主开发卡需按教材语汇精化）：
//  R1 切分点：每个跨度先在小节线与拍线处切开（弱起跨拍即产生同音延音链）；
//  R2 段内贪心：用允许符号从大到小填满每段；
//  R3 同一 kernel note 的相邻符号互为延音（tie）；休止无延音。
function deriveNotation(model) {
  const mLen = model.beatsPerMeasure * GRID_PER_BEAT;
  const total = Math.max(...model.notes.map(n => n.start + n.length), 1);
  const measures = Math.ceil(total / mLen);
  // 收集切分点：小节线 + 拍线
  const cutPoints = new Set();
  for (let m = 0; m <= measures; m++) cutPoints.add(m * mLen);
  for (let t = 0; t <= measures * mLen; t += GRID_PER_BEAT) cutPoints.add(t);

  const events = []; // {measure, tick, sizeSixteenths, kind: note|rest, kernelId?, pitch?, tieNext?, tiePrev?}
  const totalTicks = measures * mLen;
  const nextCutAfter = (t) => Math.min(...[...cutPoints].filter(c => c > t));
  // 段落 = 音符跨度与空档跨度，一律在切分点（小节线/拍线）切开
  const spans = [];
  const coveredTick = new Set();
  for (const n of model.notes) {
    let s = n.start;
    const end = n.start + n.length;
    while (s < end) {
      const segEnd = Math.min(nextCutAfter(s), end);
      spans.push({ kernelId: n.id, pitch: n.pitch, s, e: segEnd });
      for (let t = s; t < segEnd; t++) coveredTick.add(t);
      s = segEnd;
    }
  }
  for (let t = 0; t < totalTicks;) {
    if (coveredTick.has(t)) { t++; continue; }
    let e = t;
    while (e < totalTicks && !coveredTick.has(e)) e++;
    let s = t;
    while (s < e) {
      const segEnd = Math.min(nextCutAfter(s), e);
      spans.push({ kernelId: null, pitch: null, s, e: segEnd });
      s = segEnd;
    }
    t = e;
  }
  spans.sort((a, b) => a.s - b.s);
  // 段内贪心：允许符号从大到小
  for (const sp of spans) {
    let t = sp.s;
    while (t < sp.e) {
      const room = sp.e - t;
      const size = SIZES.find(sz => sz <= room) ?? SIXTEENTH;
      events.push({
        measure: Math.floor(t / mLen),
        tick: t,
        sizeSixteenths: size,
        kind: sp.kernelId ? 'note' : 'rest',
        kernelId: sp.kernelId,
        pitch: sp.pitch,
      });
      t += size;
    }
  }
  // 同 kernel 相邻符号连延音链
  const byId = new Map();
  for (const ev of events) if (ev.kernelId) {
    const arr = byId.get(ev.kernelId) ?? [];
    arr.push(ev); byId.set(ev.kernelId, arr);
  }
  const tieGroups = [];
  for (const arr of byId.values()) {
    if (arr.length > 1) tieGroups.push(arr.map(e => e.tick));
    for (let i = 0; i + 1 < arr.length; i++) { arr[i].tieNext = true; arr[i + 1].tiePrev = true; }
  }
  return { events, tieGroups, measures };
}

const d0 = deriveNotation(model);
// L1 断言：附点 1 个（k1=DOTTED_EIGHTH）、休止覆盖 1.0-1.5 与 4.5-6.0、
//          延音组 2 个（k4 跨拍、k5 跨小节线）、k5 延音组 tick 跨 measure 边界
const dotted0 = d0.events.filter(e => e.kind === 'note' && SYMBOL[e.sizeSixteenths].dots === 1);
ok('L1-dotted', dotted0.length === 1 && dotted0[0].kernelId === 'k1' && dotted0[0].tick === 0,
  `dotted=${JSON.stringify(dotted0)}`);
const rests0 = d0.events.filter(e => e.kind === 'rest').map(e => [e.tick, e.sizeSixteenths]);
ok('L1-rest-1', rests0.some(([t, s]) => t === 4 && s === EIGHTH), `rests=${JSON.stringify(rests0)}`); // 1.0-1.5beat
ok('L1-rest-2', rests0.some(([t, s]) => t === 18 && s === EIGHTH), `rests=${JSON.stringify(rests0)}`); // 4.5-5.0beat
ok('L1-ties', d0.tieGroups.length === 2, `tieGroups=${JSON.stringify(d0.tieGroups)}`);
const k5g = d0.tieGroups.find(g => g.length === 2 && g[0] === 14);
ok('L1-tie-barline', !!k5g && d0.events.find(e => e.tick === 14).measure === 1 && d0.events.find(e => e.tick === 16).measure === 2,
  `k5group=${JSON.stringify(k5g)}`);
results.legs.L1_derive = 'PASS (附点1/休止窗2/延音组2含跨小节线1)';

// ---------- 3. 渲染（jsdom + VexFlow 5，逐小节一 stave） ----------
const vex = await import('vexflow');
const { Renderer, Stave, StaveNote, Voice, Formatter, Dot, StaveTie, Beam } = vex;

function renderScore(model, outName) {
  const { events, tieGroups, measures } = deriveNotation(model);
  const div = document.createElement('div');
  const renderer = new Renderer(div, Renderer.Backends.SVG);
  renderer.resize(760, 140 * measures + 40);
  const ctx = renderer.getContext();
  const staves = [];
  for (let m = 0; m < measures; m++) {
    const stave = new Stave(20, 20 + m * 140, 720);
    if (m === 0) stave.addClef('treble').addTimeSignature('2/4');
    stave.setContext(ctx).draw();
    staves.push(stave);
  }
  const noteObjs = new Map(); // tick -> StaveNote
  for (let m = 0; m < measures; m++) {
    const evs = events.filter(e => e.measure === m);
    const notes = [];
    for (const ev of evs) {
      const sym = SYMBOL[ev.sizeSixteenths];
      const key = ev.kind === 'rest' ? 'b/4' : midiToKey(ev.pitch);
      const n = new StaveNote({ keys: [key], duration: sym.dur + (ev.kind === 'rest' ? 'r' : ''), clef: 'treble' });
      if (sym.dots > 0 && ev.kind === 'note') Dot.buildAndAttach([n], { all: true });
      noteObjs.set(ev.tick, n);
      notes.push(n);
    }
    // 同拍内相邻 8 分/16 分连 beam（视觉正确性最小集）
    for (let i = 0; i + 1 < notes.length; i++) {
      const a = evs[i], b = evs[i + 1];
      const beamable = (x) => ['8', '16'].includes(SYMBOL[x.sizeSixteenths].dur) && x.kind === 'note';
      if (beamable(a) && beamable(b) && Math.floor(a.tick / GRID_PER_BEAT) === Math.floor(b.tick / GRID_PER_BEAT)) {
        try { new Beam([notes[i], notes[i + 1]]).setContext(ctx).draw(); } catch { /* beam 装饰性，不作为断言 */ }
      }
    }
    const voice = new Voice({ num_beats: model.beatsPerMeasure, beat_value: 4 })
      .setMode(Voice.Mode.SOFT).addTickables(notes);
    new Formatter().joinVoices([voice]).format([voice], 680);
    voice.draw(ctx, staves[m]);
  }
  for (const g of tieGroups) {
    const a = noteObjs.get(g[0]), b = noteObjs.get(g[g.length - 1]);
    new StaveTie({ firstNote: a, lastNote: b }).setContext(ctx).draw();
  }
  const svg = div.innerHTML || div.firstElementChild?.outerHTML || '';
  fs.writeFileSync(path.join(ART, outName), svg);
  return { svg, events, tieGroups, noteObjs };
}

function midiToKey(p) { // 60=C4
  const names = ['c', 'c#', 'd', 'd#', 'e', 'f', 'f#', 'g', 'g#', 'a', 'a#', 'b'];
  return `${names[p % 12]}/${Math.floor(p / 12) - 1}`;
}

let t0 = performance.now();
const r0 = renderScore(model, 'state0-initial.svg');
results.timings.render3bars_ms = Math.round(performance.now() - t0);
const heads0 = (r0.svg.match(/notehead/g) || []).length;
const restGlyphs0 = (r0.svg.match(/rests?\./g) || []).length + (r0.svg.match(/"rest/g) || []).length;
ok('L2-svg-nonempty', r0.svg.length > 2000, `len=${r0.svg.length}`);
const noteEvents0 = r0.events.filter(e => e.kind === 'note').length;
ok('L2-noteheads', heads0 >= noteEvents0, `heads=${heads0} noteEvents=${noteEvents0}`);

// ---------- 4. 编辑->序列化->重解析->重推导->重绘 往返 ----------
const serialize = (m) => JSON.stringify({ timeSignature: m.timeSignature, beatsPerMeasure: m.beatsPerMeasure, notes: m.notes });
const reparse = (s) => JSON.parse(s);

function applyEdit(name, mutate, expects) {
  const json1 = serialize(model);            // 序列化（=写入前快照）
  const m2 = reparse(json1);                 // 重解析（=回读后重建）
  mutate(m2);                                // 真实编辑操作
  const rd = deriveNotation(m2);             // 重推导
  const svgName = `state-${name}.svg`;
  const rr = renderScore(m2, svgName);       // 重绘
  for (const [label, fn] of expects) ok(`L2-${name}-${label}`, fn(rd, rr), JSON.stringify(rd.events.map(e => [e.tick, e.kind, e.sizeSixteenths, e.kernelId])));
  results.legs[`L2_edit_${name}`] = 'PASS';
  Object.assign(model, m2);
}

// e1 插入：D5 @1.0 len0.5 填掉八分休止窗
applyEdit('insert', (m) => { m.notes.push(kernelNote('k6', 74, 1.0, 0.5)); }, [
  ['no-rest-1to15', (rd) => !rd.events.some(e => e.kind === 'rest' && e.tick === 4)],
  ['note-exists', (rd) => rd.events.some(e => e.kernelId === 'k6' && e.tick === 4 && e.sizeSixteenths === EIGHTH)],
]);
// e2 删除：删 k4（切分延音组消失；空档 8→14 按拍线切为 [8,12)+[12,14) -> 四分休止+八分休止）
applyEdit('delete', (m) => { m.notes = m.notes.filter(n => n.id !== 'k4'); }, [
  ['k4-gone', (rd) => !rd.events.some(e => e.kernelId === 'k4')],
  ['rests-fill', (rd) => rd.events.some(e => e.kind === 'rest' && e.tick === 8 && e.sizeSixteenths === QUARTER) && rd.events.some(e => e.kind === 'rest' && e.tick === 12 && e.sizeSixteenths === EIGHTH)],
  ['ties-now-1', (rd) => rd.tieGroups.length === 1],
]);
// e3 改长：k5 len 1.0->0.5（跨小节线延音组消失）
applyEdit('resize', (m) => { const n = m.notes.find(x => x.id === 'k5'); n.length = 0.5 * GRID_PER_BEAT; }, [
  ['no-ties', (rd) => rd.tieGroups.length === 0],
  ['k5-single', (rd) => rd.events.filter(e => e.kernelId === 'k5').length === 1],
]);
// e4 附点切换：k3 len 0.5->0.75（弱起附点跨小节线 -> 八分+十六分延音链=R1 确定性行为；空档 -> 附点八分休止+八分休止）
applyEdit('dottoggle', (m) => { const n = m.notes.find(x => x.id === 'k3'); n.length = 0.75 * GRID_PER_BEAT; }, [
  ['k3-tied-pair', (rd) => {
    const k3 = rd.events.filter(e => e.kernelId === 'k3').map(e => [e.tick, e.sizeSixteenths]);
    return k3.length === 2 && k3[0][1] === EIGHTH && k3[1][1] === SIXTEENTH && k3[1][0] === 8;
  }],
  ['k3-tie-group', (rd) => rd.tieGroups.length === 1],
  ['dotted-rest-fill', (rd) => rd.events.some(e => e.kind === 'rest' && e.tick === 9 && e.sizeSixteenths === DOTTED_EIGHTH) && rd.events.some(e => e.kind === 'rest' && e.tick === 12 && e.sizeSixteenths === EIGHTH)],
]);
results.legs.L2_edit_roundtrip = 'PASS (insert/delete/resize/dot-toggle 四操作，序列化->重解析->重推导->重绘全链)';

// ---------- 5. 游标时钟：tick->x 查找表（按小节分组；行内单调，跨行跳行） ----------
const { noteObjs } = renderScore(model, 'state-final.svg');
const MLEN = model.beatsPerMeasure * GRID_PER_BEAT;
const rows = new Map(); // measure -> [[tick,x]]
for (const [tick, n] of noteObjs) {
  const x = typeof n.getAbsoluteX === 'function' ? n.getAbsoluteX() : n.getBoundingBox().getX();
  const m = Math.floor(tick / MLEN);
  if (!rows.has(m)) rows.set(m, []);
  rows.get(m).push([tick, x]);
}
let totalPts = 0;
for (const [m, arr] of rows) {
  arr.sort((a, b) => a[0] - b[0]);
  totalPts += arr.length;
  for (let i = 1; i < arr.length; i++) ok('L3-monotonic', arr[i][1] >= arr[i - 1][1] - 0.01, `m${m} ${JSON.stringify(arr)}`);
}
const xAt = (beat) => {
  const t = beat * GRID_PER_BEAT;
  const m = Math.floor(t / MLEN);
  const arr = rows.get(m) ?? [[0, 40]];
  for (let i = 1; i < arr.length; i++) if (arr[i][0] >= t) {
    const [t0, x0] = arr[i - 1], [t1, x1] = arr[i];
    return x0 + (x1 - x0) * (t - t0) / Math.max(1, t1 - t0);
  }
  return arr[arr.length - 1][1];
};
const row3 = rows.get(1); // 第二小节含 tick 14（k5）
ok('L3-hit', Math.abs(xAt(3.5) - row3.find(([t]) => t === 14)[1]) < 0.01, '');
ok('L3-interp', xAt(2.625) > row3.find(([t]) => t === 9)[1] && xAt(2.625) < row3.find(([t]) => t === 12)[1], '');
results.legs.L3_cursor = `PASS (tick->x 表 ${totalPts} 项/3 行，行内单调，命中+插值)`;

// ---------- 6. MIDI 往返（SMF PPQ=48，16分=12 tick，全整数） ----------
const PPQ = 48, PER_16 = PPQ / 4;
function toSMF(m) {
  const track = [];
  // tempo 80bpm + 2/4
  track.push([0, [0xFF, 0x51, 3, ...int3b(60000000 / 80)]]);
  track.push([0, [0xFF, 0x58, 4, 2, 2, 24, 8]]);
  const evs = [];
  for (const n of m.notes) {
    evs.push({ t: n.start * PER_16, on: true, p: n.pitch });
    evs.push({ t: (n.start + n.length) * PER_16, on: false, p: n.pitch });
  }
  evs.sort((a, b) => a.t - b.t || (a.on ? 1 : -1));
  let last = 0;
  for (const e of evs) {
    track.push([e.t - last, e.on ? [0x90, e.p, 100] : [0x80, e.p, 0]]);
    last = e.t;
  }
  track.push([0, [0xFF, 0x2F]]);
  const bytes = [];
  const u32 = (v) => [v >>> 24 & 255, v >>> 16 & 255, v >>> 8 & 255, v & 255];
  const vlq = (v) => { const o = [v & 127]; while (v > 127) { v >>>= 7; o.unshift((v & 127) | 128); } return o; };
  bytes.push(0x4D, 0x54, 0x68, 0x64, 0, 0, 0, 6, 0, 0, 0, 1, (PPQ >> 8) & 255, PPQ & 255);
  const trk = [];
  for (const [delta, msg] of track) { trk.push(...vlq(delta), ...msg); }
  bytes.push(0x4D, 0x54, 0x72, 0x6B, ...u32(trk.length), ...trk);
  return Buffer.from(bytes);
}
function int3b(v) { const x = Math.round(v); return [x >> 16 & 255, x >> 8 & 255, x & 255]; }
function parseSMF(buf) {
  let pos = 0;
  const rd = (n) => buf.subarray(pos, pos + n), u32 = () => { const v = buf.readUInt32BE(pos); pos += 4; return v; };
  if (buf.toString('ascii', 0, 4) !== 'MThd') throw new Error('not SMF');
  pos = 4; u32(); const format = buf.readUInt16BE(pos); pos += 2; const ntrks = buf.readUInt16BE(pos); pos += 2; const division = buf.readUInt16BE(pos); pos += 2;
  const notes = []; const open = new Map();
  for (let trk = 0; trk < ntrks; trk++) {
    if (buf.toString('ascii', pos, pos + 4) !== 'MTrk') throw new Error('bad track');
    pos += 4; const len = u32(); const end = pos + len; let tick = 0; let running = 0;
    while (pos < end) {
      let d = 0, b; do { b = buf[pos++]; d = (d << 7) | (b & 127); } while (b & 128);
      tick += d;
      let st = buf[pos];
      if (st < 0x80) st = running; else { pos++; running = st; }
      if (st === 0xFF) { const type = buf[pos++]; let l = 0; do { b = buf[pos++]; l = (l << 7) | (b & 127); } while (b & 128); pos += l; }
      else if (st >= 0xF0) { let l = 0; do { b = buf[pos++]; l = (l << 7) | (b & 127); } while (b & 128); pos += l; }
      else {
        const n = st & 0xF0, ch = st & 0x0F;
        if (n === 0x90 && buf[pos + 1] > 0) { open.set(ch * 128 + buf[pos], { p: buf[pos], t: tick }); pos += 2; }
        else if (n === 0x80 || n === 0x90) { const key = ch * 128 + buf[pos]; const o = open.get(key); if (o) { notes.push({ pitch: o.p, start: o.t, end: tick }); open.delete(key); } pos += 2; }
        else if (n === 0xC0 || n === 0xD0) pos += 1;
        else pos += 2;
      }
    }
    pos = end;
  }
  return { division, notes };
}
const smf = toSMF(model);
fs.writeFileSync(path.join(ART, 'final.mid'), smf);
const back = parseSMF(smf);
ok('L4-ppq', back.division === PPQ, `division=${back.division}`);
const mine = model.notes.map(n => [n.pitch, n.start / GRID_PER_BEAT, n.length / GRID_PER_BEAT]).sort();
const theirs = back.notes.map(n => [n.pitch, n.start / PPQ, (n.end - n.start) / PPQ]).sort();
ok('L4-set-equal', mine.length === theirs.length && mine.every((x, i) => x.every((v, j) => Math.abs(v - theirs[i][j]) < 1e-9)),
  `mine=${JSON.stringify(mine)} theirs=${JSON.stringify(theirs)}`);
results.legs.L4_midi_roundtrip = `PASS (模型->SMF(${smf.length}B)->解析回读：${theirs.length} 音符 pitch/start/length 集合相等)`;

fs.writeFileSync(path.join(ART, 'results.json'), JSON.stringify(results, null, 2));
console.log('SPIKE A (VexFlow 5 + own model): ALL PASS');
console.log(JSON.stringify(results, null, 2));
