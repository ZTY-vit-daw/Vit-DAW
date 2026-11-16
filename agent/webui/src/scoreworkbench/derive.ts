// SCORE-WORKBENCH-1 M1：确定性记谱推导（DERIVATION_RULES.md v1 的实现）。
// 内核平音符 → 记谱符号+延音链。R1 切分点/R2 段内贪心/R3 延音链——编号与
// DERIVATION_RULES.md 一一对应，改规则先改文档。全部整数 tick 运算，同输入同输出。
// 369 spike 亲证基础（27 断言）+ 教材语汇精化（拍内附点归并=段长3单符号、
// 弱起延音链=R1.4、休止填充表=R2 表）。

import {
  GRID_PER_BEAT,
  measureLengthTicks,
  beatsPerMeasure,
  beatsRoundedToTicks,
  validateModel,
  type ScoreModel
} from "./model";

export type SymbolSize = 1 | 2 | 3 | 4;

export type NotationEvent = {
  measure: number;
  tick: number;
  sizeSixteenths: SymbolSize;
  kind: "note" | "rest";
  kernelId: string | null;
  pitch: number | null;
  tieNext?: true;
  tiePrev?: true;
};

export type Notation = {
  events: NotationEvent[];
  tieGroups: Array<number[]>;
  measures: number;
};

// R2 符号表：段长 → 记谱时值（DERIVATION_RULES §2；段长 ≤4 由 R1.3 保证）。
export const SYMBOL_BY_SIZE: Record<SymbolSize, { duration: "16" | "8" | "q"; dots: 0 | 1 }> = {
  1: { duration: "16", dots: 0 },
  2: { duration: "8", dots: 0 },
  3: { duration: "8", dots: 1 },
  4: { duration: "q", dots: 0 }
};

type Span = { kernelId: string | null; pitch: number | null; startTick: number; endTick: number };

// deriveNotation：R1 切分 → 段落收集（音符段+休止段）→ R2 符号化 → R3 延音链。
export function deriveNotation(model: ScoreModel): Notation {
  validateModel(model);
  const measureTicks = measureLengthTicks(model.timeSignature);
  const lastTick = model.notes.reduce(
    (max, note) => Math.max(max, beatsRoundedToTicks(note.startBeats + note.lengthBeats)),
    0
  );
  const measures = Math.max(1, Math.ceil(lastTick / measureTicks));
  const totalTicks = measures * measureTicks;

  // R1 切分点：小节线（R1.1）+ 拍线（R1.2）。
  const cutPoints = new Set<number>();
  for (let m = 0; m <= measures; m++) {
    cutPoints.add(m * measureTicks);
    for (let b = 1; b < beatsPerMeasure(model.timeSignature); b++) {
      cutPoints.add(m * measureTicks + b * GRID_PER_BEAT);
    }
  }
  const nextCutAfter = (tick: number): number => {
    let best = totalTicks;
    for (const cut of cutPoints) {
      if (cut > tick && cut < best) {
        best = cut;
      }
    }
    return best;
  };

  // 音符跨度按切分点切段（R1.3/R1.4：弱起跨拍即产生多段，交 R3 连延音链）。
  const spans: Span[] = [];
  const covered = new Set<number>();
  for (const note of model.notes) {
    let cursor = beatsRoundedToTicks(note.startBeats);
    const end = beatsRoundedToTicks(note.startBeats + note.lengthBeats);
    while (cursor < end) {
      const segEnd = Math.min(nextCutAfter(cursor), end);
      spans.push({ kernelId: note.id, pitch: note.pitch, startTick: cursor, endTick: segEnd });
      for (let tick = cursor; tick < segEnd; tick++) {
        covered.add(tick);
      }
      cursor = segEnd;
    }
  }
  // 空档跨度（休止填充表同 R2 表）：同样按切分点切段。
  for (let tick = 0; tick < totalTicks; ) {
    if (covered.has(tick)) {
      tick++;
      continue;
    }
    let end = tick;
    while (end < totalTicks && !covered.has(end)) {
      end++;
    }
    let cursor = tick;
    while (cursor < end) {
      const segEnd = Math.min(nextCutAfter(cursor), end);
      spans.push({ kernelId: null, pitch: null, startTick: cursor, endTick: segEnd });
      cursor = segEnd;
    }
    tick = end;
  }
  spans.sort((a, b) => a.startTick - b.startTick);

  // R2 段内贪心：段长 ∈ {1..4} → 单符号（表驱动；确定性无歧义）。
  const events: NotationEvent[] = [];
  for (const span of spans) {
    let cursor = span.startTick;
    while (cursor < span.endTick) {
      const room = span.endTick - cursor;
      let size: SymbolSize = 1;
      for (const candidate of [4, 3, 2] as const) {
        if (candidate <= room) {
          size = candidate;
          break;
        }
      }
      events.push({
        measure: Math.floor(cursor / measureTicks),
        tick: cursor,
        sizeSixteenths: size,
        kind: span.kernelId ? "note" : "rest",
        kernelId: span.kernelId,
        pitch: span.pitch
      });
      cursor += size;
    }
  }

  // R3 延音链：同 kernelId 相邻符号 tieNext/tiePrev；休止不参与（R3.3）。
  const byKernel = new Map<string, NotationEvent[]>();
  for (const event of events) {
    if (!event.kernelId) {
      continue;
    }
    const bucket = byKernel.get(event.kernelId);
    if (bucket) {
      bucket.push(event);
    } else {
      byKernel.set(event.kernelId, [event]);
    }
  }
  const tieGroups: Array<number[]> = [];
  for (const bucket of byKernel.values()) {
    bucket.sort((a, b) => a.tick - b.tick);
    if (bucket.length > 1) {
      tieGroups.push(bucket.map((event) => event.tick));
      for (let i = 0; i + 1 < bucket.length; i++) {
        bucket[i].tieNext = true;
        bucket[i + 1].tiePrev = true;
      }
    }
  }

  return { events, tieGroups, measures };
}

// eventsAtMeasure：渲染/命中检测按小节取事件（保持推导顺序）。
export function eventsAtMeasure(notation: Notation, measure: number): NotationEvent[] {
  return notation.events.filter((event) => event.measure === measure);
}
