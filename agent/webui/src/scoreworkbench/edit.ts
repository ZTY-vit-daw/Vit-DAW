// SCORE-WORKBENCH-1 M2：本地编辑缓冲与编辑操作（治理粒度=本地缓冲+显式 apply
// 单次确认——主管工作假设，RiskConfirm 语义对齐：所有编辑只改本地 working 模型，
// 面板显式「写入内核」才产一次 replace_region 整段覆盖（A5 保存=整段覆盖语义）。
// 全部操作确定性、网格对齐（tick=16 分数）；paint 语义：新符号覆盖其跨度内旧音符
// 时间（无隐藏报错路径）。

import {
  GRID_PER_BEAT,
  beatsRoundedToTicks,
  serializeModel,
  validateModel,
  type KernelNote,
  type ScoreModel
} from "./model";

export type EditBuffer = {
  base: ScoreModel;
  working: ScoreModel;
  nextLocalID: number;
};

export function createEditBuffer(base: ScoreModel): EditBuffer {
  validateModel(base);
  return { base, working: cloneModel(base), nextLocalID: 1 };
}

function cloneModel(model: ScoreModel): ScoreModel {
  return {
    timeSignature: { ...model.timeSignature },
    notes: model.notes.map((note) => ({ ...note }))
  };
}

export function bufferDirty(buffer: EditBuffer): boolean {
  return serializeModel(buffer.base) !== serializeModel(buffer.working);
}

export function bufferDiffSummary(buffer: EditBuffer): { added: number; removed: number; changed: number } {
  const baseByID = new Map(buffer.base.notes.map((note) => [note.id, note]));
  const workingByID = new Map(buffer.working.notes.map((note) => [note.id, note]));
  let added = 0;
  let removed = 0;
  let changed = 0;
  for (const [id, note] of workingByID) {
    const before = baseByID.get(id);
    if (!before) {
      added++;
    } else if (before.pitch !== note.pitch || before.startBeats !== note.startBeats || before.lengthBeats !== note.lengthBeats) {
      changed++;
    }
  }
  for (const id of baseByID.keys()) {
    if (!workingByID.has(id)) {
      removed++;
    }
  }
  return { added, removed, changed };
}

function localID(buffer: EditBuffer): string {
  return `local-${buffer.nextLocalID++}`;
}

function notesOverlapping(model: ScoreModel, startTick: number, endTick: number): KernelNote[] {
  return model.notes.filter((note) => {
    const noteStart = beatsRoundedToTicks(note.startBeats);
    const noteEnd = beatsRoundedToTicks(note.startBeats + note.lengthBeats);
    return noteStart < endTick && startTick < noteEnd;
  });
}

// clearSpan：paint 语义核心——把 [startTick,endTick) 内的旧音符时间清掉：
// 完全被覆盖的音符删除；部分覆盖的音符被截短（截短到 0 = 删除）。确定性：按 id 稳定排序。
function clearSpan(model: ScoreModel, startTick: number, endTick: number): void {
  const victims = notesOverlapping(model, startTick, endTick).sort((a, b) => a.id.localeCompare(b.id));
  const keep: KernelNote[] = [];
  for (const note of model.notes) {
    const victim = victims.find((item) => item.id === note.id);
    if (!victim) {
      keep.push(note);
      continue;
    }
    const noteStart = beatsRoundedToTicks(note.startBeats);
    const noteEnd = beatsRoundedToTicks(note.startBeats + note.lengthBeats);
    if (noteStart >= startTick && noteEnd <= endTick) {
      continue; // 完全覆盖 → 删除
    }
    if (noteStart < startTick && noteEnd > endTick) {
      // 跨越整个新区间 → 拆两段（保留前段+后段）
      keep.push({
        ...note,
        lengthBeats: (startTick - noteStart) / GRID_PER_BEAT
      });
      keep.push({
        ...note,
        id: `${note.id}#split-${startTick}`,
        startBeats: endTick / GRID_PER_BEAT,
        lengthBeats: (noteEnd - endTick) / GRID_PER_BEAT
      });
      continue;
    }
    if (noteStart < startTick) {
      keep.push({ ...note, lengthBeats: (startTick - noteStart) / GRID_PER_BEAT });
    } else {
      keep.push({ ...note, startBeats: endTick / GRID_PER_BEAT, lengthBeats: (noteEnd - endTick) / GRID_PER_BEAT });
    }
  }
  const withoutEmpty = keep.filter((note) => note.lengthBeats > 0);
  // 拆段产生的合成 id 防碰撞（同 note 二次拆）：
  model.notes = withoutEmpty;
}

// placeNote：点击放符（paint 语义：覆盖跨度内旧音符时间）。
export function placeNote(buffer: EditBuffer, pitch: number, startTick: number, sizeTicks: number): EditBuffer {
  const next = { ...buffer, working: cloneModel(buffer.working) };
  clearSpan(next.working, startTick, startTick + sizeTicks);
  next.working.notes.push({
    id: localID(buffer),
    pitch,
    startBeats: startTick / GRID_PER_BEAT,
    lengthBeats: sizeTicks / GRID_PER_BEAT
  });
  validateModel(next.working);
  return next;
}

// insertRest：休止插入=抹掉跨度内的音符时间（休止在推导层自动出现）。
export function insertRest(buffer: EditBuffer, startTick: number, sizeTicks: number): EditBuffer {
  const next = { ...buffer, working: cloneModel(buffer.working) };
  clearSpan(next.working, startTick, startTick + sizeTicks);
  validateModel(next.working);
  return next;
}

// setDuration：时值切换（size 1/2/4；3=附点八分、6=附点四分由 toggleDot 产生）。
export function setDuration(buffer: EditBuffer, kernelId: string, sizeTicks: number): EditBuffer {
  const note = buffer.working.notes.find((item) => item.id === kernelId);
  if (!note) {
    return buffer;
  }
  const startTick = beatsRoundedToTicks(note.startBeats);
  const next = { ...buffer, working: cloneModel(buffer.working) };
  const target = next.working.notes.find((item) => item.id === kernelId);
  if (!target) {
    return buffer;
  }
  next.working.notes = next.working.notes.filter((item) => item.id !== kernelId);
  clearSpan(next.working, startTick, startTick + sizeTicks);
  next.working.notes.push({ ...target, lengthBeats: sizeTicks / GRID_PER_BEAT });
  validateModel(next.working);
  return next;
}

// 附点开关表（DERIVATION_RULES 之上的编辑语义层）：2↔3、4↔6；1 无附点格（1.5 不在网格）→ 原样返回。
const DOT_TOGGLE: Record<number, number> = { 2: 3, 3: 2, 4: 6, 6: 4 };

export function toggleDot(buffer: EditBuffer, kernelId: string): EditBuffer {
  const note = buffer.working.notes.find((item) => item.id === kernelId);
  if (!note) {
    return buffer;
  }
  const currentTicks = beatsRoundedToTicks(note.lengthBeats);
  const targetTicks = DOT_TOGGLE[currentTicks];
  if (!targetTicks) {
    return buffer;
  }
  return setDuration(buffer, kernelId, targetTicks);
}

// tieEstablish：延音建立——相邻（尾部=头部）同音高两音符合并为一个内核音符
// （推导层自然产生延音链）。不满足相邻/同音高 → 原样返回（fail-honest 无隐藏副作用）。
export function tieEstablish(buffer: EditBuffer, kernelIdA: string, kernelIdB: string): EditBuffer {
  const a = buffer.working.notes.find((item) => item.id === kernelIdA);
  const b = buffer.working.notes.find((item) => item.id === kernelIdB);
  if (!a || !b || a.pitch !== b.pitch) {
    return buffer;
  }
  const aEnd = beatsRoundedToTicks(a.startBeats + a.lengthBeats);
  const bStart = beatsRoundedToTicks(b.startBeats);
  const bEnd = beatsRoundedToTicks(b.startBeats + b.lengthBeats);
  const aStart = beatsRoundedToTicks(a.startBeats);
  if (aEnd !== bStart && bEnd !== aStart) {
    return buffer;
  }
  const [first, second] = aEnd === bStart ? [a, b] : [b, a];
  const next = { ...buffer, working: cloneModel(buffer.working) };
  next.working.notes = next.working.notes.filter((item) => item.id !== first.id && item.id !== second.id);
  next.working.notes.push({
    id: first.id,
    pitch: first.pitch,
    startBeats: first.startBeats,
    lengthBeats: first.lengthBeats + second.lengthBeats
  });
  validateModel(next.working);
  return next;
}

// tieRelease：延音解除——在指定 tick 处把一个内核音符拆成两个（推导层延音链消失）。
// atTick 必须严格在音符内部；否则原样返回。
export function tieRelease(buffer: EditBuffer, kernelId: string, atTick: number): EditBuffer {
  const note = buffer.working.notes.find((item) => item.id === kernelId);
  if (!note) {
    return buffer;
  }
  const startTick = beatsRoundedToTicks(note.startBeats);
  const endTick = beatsRoundedToTicks(note.startBeats + note.lengthBeats);
  if (atTick <= startTick || atTick >= endTick) {
    return buffer;
  }
  const next = { ...buffer, working: cloneModel(buffer.working) };
  next.working.notes = next.working.notes.filter((item) => item.id !== kernelId);
  next.working.notes.push({
    id: kernelId,
    pitch: note.pitch,
    startBeats: note.startBeats,
    lengthBeats: (atTick - startTick) / GRID_PER_BEAT
  });
  next.working.notes.push({
    id: `${kernelId}#split-${atTick}`,
    pitch: note.pitch,
    startBeats: atTick / GRID_PER_BEAT,
    lengthBeats: (endTick - atTick) / GRID_PER_BEAT
  });
  validateModel(next.working);
  return next;
}

// deleteNote：删除（空档由推导层自动落休止）。
export function deleteNote(buffer: EditBuffer, kernelId: string): EditBuffer {
  const next = { ...buffer, working: cloneModel(buffer.working) };
  next.working.notes = next.working.notes.filter((item) => item.id !== kernelId);
  validateModel(next.working);
  return next;
}

// dragNote：拖动（16 分网格步进；paint 语义落位）。
export function dragNote(buffer: EditBuffer, kernelId: string, toStartTick: number): EditBuffer {
  const note = buffer.working.notes.find((item) => item.id === kernelId);
  if (!note || toStartTick < 0) {
    return buffer;
  }
  const lengthTicks = beatsRoundedToTicks(note.lengthBeats);
  const next = { ...buffer, working: cloneModel(buffer.working) };
  const target = next.working.notes.find((item) => item.id === kernelId);
  if (!target) {
    return buffer;
  }
  next.working.notes = next.working.notes.filter((item) => item.id !== kernelId);
  clearSpan(next.working, toStartTick, toStartTick + lengthTicks);
  next.working.notes.push({ ...target, startBeats: toStartTick / GRID_PER_BEAT });
  validateModel(next.working);
  return next;
}

// buildApplyPatch：显式 apply 的载荷（A5 保存=整段覆盖：单 replace_region）。
// notes 不带 id——内核 replace_region 自assign 新 id，回读后重绑（A2 映射稳定由
// 回读重建链保证，非 id 保号）。
export function buildApplyPatch(buffer: EditBuffer, clipLengthBeats: number): {
  op: "replace_region";
  region_start: number;
  region_length: number;
  time_unit: "beats";
  notes: Array<{ pitch: number; start: number; length: number }>;
} {
  const lengthTicks = beatsRoundedToTicks(clipLengthBeats);
  const notes = [...buffer.working.notes]
    .sort((a, b) => beatsRoundedToTicks(a.startBeats) - beatsRoundedToTicks(b.startBeats))
    .map((note) => ({
      pitch: note.pitch,
      start: note.startBeats,
      length: note.lengthBeats
    }));
  return {
    op: "replace_region",
    region_start: 0,
    region_length: lengthTicks / GRID_PER_BEAT,
    time_unit: "beats",
    notes
  };
}
