// SCORE-WORKBENCH-1 M1：beat-grid 记谱模型（kernel 形状，推导规则的输入）。
// 契约：DERIVATION_RULES.md §0——GRID_PER_BEAT=4（16 分网格），内核形状音符
// {id, pitch, startBeats, lengthBeats}（beats 相对 clip 起点，time_unit:"beats"），
// id 为内核稳定音符 ID（一等公民，编辑/延音/回读映射全挂它）。
// 确定性：beats↔tick 换算只用乘除 4，无浮点累积；网格外值 fail-honest 不取整。

export const GRID_PER_BEAT = 4;

export type TimeSignature = {
  numerator: number;
  denominator: number;
};

// 内核形状音符（get_midi_clip_notes 回读对齐）
export type KernelNote = {
  id: string;
  pitch: number;
  startBeats: number;
  lengthBeats: number;
};

export type ScoreModel = {
  timeSignature: TimeSignature;
  notes: KernelNote[];
};

export type ModelErrorCode =
  | "grid_misaligned"
  | "bad_pitch"
  | "negative_length"
  | "bad_timesig"
  | "empty_id";

export class ModelError extends Error {
  readonly code: ModelErrorCode;
  constructor(code: ModelErrorCode, detail: string) {
    super(`score model error [${code}]: ${detail}`);
    this.code = code;
  }
}

export function beatsToTicks(beats: number): number {
  return beats * GRID_PER_BEAT;
}

export function ticksToBeats(ticks: number): number {
  return ticks / GRID_PER_BEAT;
}

// beatsPerMeasure：v1 仅 denominator=4（DERIVATION_RULES §0/§6），其余 fail-honest。
export function beatsPerMeasure(timeSignature: TimeSignature): number {
  if (timeSignature.denominator !== 4) {
    throw new ModelError(
      "bad_timesig",
      `v1 supports denominator=4 only (got ${timeSignature.numerator}/${timeSignature.denominator})`
    );
  }
  if (!Number.isInteger(timeSignature.numerator) || timeSignature.numerator < 1 || timeSignature.numerator > 8) {
    throw new ModelError("bad_timesig", `unusable numerator ${timeSignature.numerator}`);
  }
  return timeSignature.numerator;
}

export function measureLengthTicks(timeSignature: TimeSignature): number {
  return beatsPerMeasure(timeSignature) * GRID_PER_BEAT;
}

// validateModel：模型入口校验（fail-honest；网格外/越界 pitch/负长度/空 id 显形）。
export function validateModel(model: ScoreModel): void {
  beatsPerMeasure(model.timeSignature);
  for (const note of model.notes) {
    if (!note.id || typeof note.id !== "string") {
      throw new ModelError("empty_id", `note id missing: ${JSON.stringify(note)}`);
    }
    if (!Number.isFinite(note.pitch) || !Number.isInteger(note.pitch) || note.pitch < 0 || note.pitch > 127) {
      throw new ModelError("bad_pitch", `note ${note.id} pitch ${note.pitch} outside 0-127`);
    }
    if (!(note.startBeats >= 0) || !(note.lengthBeats > 0)) {
      throw new ModelError("negative_length", `note ${note.id} start=${note.startBeats} length=${note.lengthBeats}`);
    }
    if (!onGrid(note.startBeats) || !onGrid(note.lengthBeats)) {
      throw new ModelError(
        "grid_misaligned",
        `note ${note.id} start=${note.startBeats} length=${note.lengthBeats} not on 16th grid (multiples of 0.25)`
      );
    }
  }
}

function onGrid(beats: number): boolean {
  const ticks = beats * GRID_PER_BEAT;
  return Number.isFinite(ticks) && Math.abs(ticks - Math.round(ticks)) < 1e-9;
}

// roundTripTicks：内核往返换算的唯一入口（读写两端共用，防漂移）。
export function beatsRoundedToTicks(beats: number): number {
  const ticks = beats * GRID_PER_BEAT;
  const rounded = Math.round(ticks);
  if (Math.abs(ticks - rounded) >= 1e-9) {
    throw new ModelError("grid_misaligned", `beats=${beats} not on grid`);
  }
  return rounded;
}

export function serializeModel(model: ScoreModel): string {
  return JSON.stringify({
    timeSignature: model.timeSignature,
    notes: model.notes.map((note) => ({
      id: note.id,
      pitch: note.pitch,
      startBeats: note.startBeats,
      lengthBeats: note.lengthBeats
    }))
  });
}

export function deserializeModel(json: string): ScoreModel {
  const parsed = JSON.parse(json) as ScoreModel;
  if (!parsed || typeof parsed !== "object" || !parsed.timeSignature || !Array.isArray(parsed.notes)) {
    throw new ModelError("bad_timesig", "serialized model shape mismatch");
  }
  const model: ScoreModel = {
    timeSignature: {
      numerator: parsed.timeSignature.numerator,
      denominator: parsed.timeSignature.denominator
    },
    notes: parsed.notes.map((note) => ({
      id: String(note.id),
      pitch: Number(note.pitch),
      startBeats: Number(note.startBeats),
      lengthBeats: Number(note.lengthBeats)
    }))
  };
  validateModel(model);
  return model;
}
