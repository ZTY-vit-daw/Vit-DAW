// SCORE-WORKBENCH-1 M1：VexFlow 5 渲染薄适配层（风险 R4：锁版本 5.0.0，
// 唯一允许 import vexflow 的文件——API 漂移面收敛于此）。
// 369 spike 亲证：jsdom 无 canvas 只产生文本测量告警，渲染正常（生产浏览器无此问题）。
// 渲染形状：每小节一 stave（单声部）；beam 只连同拍内相邻 8/16 分（装饰性，不承担断言）；
// 延音 tie 用 VF5 camelCase 参数（firstNote/lastNote，369 实证漂移点）。

import { Dot, Formatter, Renderer, Stave, StaveNote, StaveTie, Voice, Beam } from "vexflow";
import { deriveNotation, eventsAtMeasure, SYMBOL_BY_SIZE, type Notation, type NotationEvent } from "./derive";
import type { ScoreModel } from "./model";

export type TickXRow = Array<[tick: number, x: number]>;

export type RenderResult = {
  svg: string;
  notation: Notation;
  // 每小节一行：tick→符号中心 x（命中检测与游标共用，行内单调）。
  tickRows: Map<number, TickXRow>;
  // 每小节 stave 几何（命中检测定位行）。
  staveBounds: Map<number, { x: number; y: number; width: number; height: number }>;
  height: number;
};

export type RenderOptions = {
  width?: number;
  rowHeight?: number;
};

const DEFAULT_WIDTH = 340;
const DEFAULT_ROW_HEIGHT = 120;
const STAVE_LEFT = 12;
// 高音谱号+拍号占掉的谱头宽（首行）——命中插值用拍线锚点而非谱头。
const CLEF_WIDTH = 52;

export function midiPitchToKey(pitch: number): string {
  const names = ["c", "c#", "d", "d#", "e", "f", "f#", "g", "g#", "a", "a#", "b"];
  return `${names[pitch % 12]}/${Math.floor(pitch / 12) - 1}`;
}

// renderScoreInto：把模型推导结果渲染进容器（SVG），返回命中检测所需几何。
export function renderScoreInto(container: HTMLDivElement, model: ScoreModel, options: RenderOptions = {}): RenderResult {
  container.textContent = "";
  const width = options.width ?? DEFAULT_WIDTH;
  const rowHeight = options.rowHeight ?? DEFAULT_ROW_HEIGHT;
  const notation = deriveNotation(model);
  const renderer = new Renderer(container, Renderer.Backends.SVG);
  renderer.resize(width, rowHeight * notation.measures + 8);
  const context = renderer.getContext();

  const staveWidth = width - STAVE_LEFT * 2;
  const tickRows = new Map<number, TickXRow>();
  const staveBounds = new Map<number, { x: number; y: number; width: number; height: number }>();
  const noteByTick = new Map<number, StaveNote>();

  for (let measure = 0; measure < notation.measures; measure++) {
    const stave = new Stave(STAVE_LEFT, 4 + measure * rowHeight, staveWidth);
    if (measure === 0) {
      stave.addClef("treble").addTimeSignature(`${model.timeSignature.numerator}/${model.timeSignature.denominator}`);
    }
    stave.setContext(context).draw();
    const bounds = stave.getBoundingBox();
    staveBounds.set(measure, { x: bounds.getX(), y: bounds.getY(), width: bounds.getW(), height: bounds.getH() });

    const measureEvents = eventsAtMeasure(notation, measure);
    const notes: StaveNote[] = [];
    for (const event of measureEvents) {
      const symbol = SYMBOL_BY_SIZE[event.sizeSixteenths];
      const key = event.kind === "rest" ? "b/4" : midiPitchToKey(event.pitch ?? 60);
      const note = new StaveNote({
        keys: [key],
        duration: symbol.duration + (event.kind === "rest" ? "r" : ""),
        clef: "treble"
      });
      if (symbol.dots > 0 && event.kind === "note") {
        Dot.buildAndAttach([note], { all: true });
      }
      noteByTick.set(event.tick, note);
      notes.push(note);
    }

    // beam：同拍内相邻可连符号（装饰性最小集；失败容忍，不承担断言）。
    for (let i = 0; i + 1 < measureEvents.length; i++) {
      const a = measureEvents[i];
      const b = measureEvents[i + 1];
      const beamable = (event: NotationEvent) =>
        event.kind === "note" && (event.sizeSixteenths === 1 || event.sizeSixteenths === 2);
      const sameBeat =
        Math.floor(a.tick / 4) === Math.floor(b.tick / 4);
      if (beamable(a) && beamable(b) && sameBeat) {
        try {
          new Beam([notes[i], notes[i + 1]]).setContext(context).draw();
        } catch {
          // beam 是视觉增强；VF 对部分组合拒绝绘制不构成失败。
        }
      }
    }

    const voice = new Voice({
      numBeats: model.timeSignature.numerator,
      beatValue: model.timeSignature.denominator
    }).setMode(Voice.Mode.SOFT);
    voice.addTickables(notes);
    new Formatter().joinVoices([voice]).format([voice], staveWidth - (measure === 0 ? CLEF_WIDTH : 8));
    voice.draw(context, stave);

    const row: TickXRow = [];
    for (const event of measureEvents) {
      const note = noteByTick.get(event.tick);
      if (!note) {
        continue;
      }
      const x = typeof note.getAbsoluteX === "function" ? note.getAbsoluteX() : note.getBoundingBox().getX();
      row.push([event.tick, x]);
    }
    row.sort((a, b) => a[0] - b[0]);
    tickRows.set(measure, row);
  }

  // R3 延音链渲染（跨小节线 tie 两端 noteObj 都在场）。
  for (const group of notation.tieGroups) {
    const first = noteByTick.get(group[0]);
    const last = noteByTick.get(group[group.length - 1]);
    if (first && last) {
      new StaveTie({ firstNote: first, lastNote: last }).setContext(context).draw();
    }
  }

  const svg = container.innerHTML ?? "";
  return {
    svg,
    notation,
    tickRows,
    staveBounds,
    height: rowHeight * notation.measures + 8
  };
}

// ---- 命中检测（M2 点击放符/拖动共用）----

export type HitTarget = {
  measure: number;
  tick: number;
  event: NotationEvent | null;
};

// rowAtY：命中行定位（y → 小节行）。
export function rowAtY(result: RenderResult, y: number): number {
  let best = -1;
  for (const [measure, bounds] of result.staveBounds) {
    if (y >= bounds.y && y <= bounds.y + bounds.height + 18) {
      best = measure;
    }
  }
  return best;
}

// xToTickContinuous：行内 x → 连续 tick 估计（符号锚点分段线性，行内单调；
// 369 spike L3 亲证模式）。行外 x 夹逼到首尾符号。
export function xToTickContinuous(row: TickXRow, x: number): number {
  if (row.length === 0) {
    return 0;
  }
  if (x <= row[0][1]) {
    return row[0][0];
  }
  const last = row[row.length - 1];
  if (x >= last[1]) {
    return last[0];
  }
  for (let i = 1; i < row.length; i++) {
    if (row[i][1] >= x) {
      const [tickA, xa] = row[i - 1];
      const [tickB, xb] = row[i];
      if (xb <= xa) {
        return tickB;
      }
      return tickA + ((x - xa) / (xb - xa)) * (tickB - tickA);
    }
  }
  return last[0];
}

// snapToGrid：连续 tick 吸附到 16 分网格。
export function snapToGrid(tick: number): number {
  return Math.max(0, Math.round(tick));
}

// hitTest：面板点击 → {行, 网格 tick, 最近符号事件}。
export function hitTest(result: RenderResult, x: number, y: number): HitTarget | null {
  const measure = rowAtY(result, y);
  if (measure < 0) {
    return null;
  }
  const row = result.tickRows.get(measure) ?? [];
  const continuous = xToTickContinuous(row, x);
  const tick = snapToGrid(continuous);
  const event = result.notation.events.find((item) => item.measure === measure && item.tick === tick) ?? null;
  return { measure, tick, event };
}
