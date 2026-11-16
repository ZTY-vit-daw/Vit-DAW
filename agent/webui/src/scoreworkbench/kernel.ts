// SCORE-WORKBENCH-1 M3：谱面工作台↔内核桥（经 agent HTTP 生产缝）。
// 链路：webui → /agent/invoke（invokeAgent）→ harness → 内核 ZMQ 命令面。
// 读取缝=analyze_midi_transformations（RiskDirect，回显 music_context_echo：
// tempo/拍号/分段计数——与 edu 判定器同款 fail-honest 门）+get_midi_clip_notes/
// get_midi_clip_data；写入缝=apply_midi_note_patch[RiskConfirm]（显式 apply 单次
// 确认，UI 已过确认对话框后 confirmed:true 提交）。
// 红线：治理链不动；回读不一致=fail-honest（不面显已保存，A5）。

import { invokeAgent } from "../lib/api";
import type { AgentInvokeResponse, JsonRecord } from "../types";
import { GRID_PER_BEAT, beatsPerMeasure, type KernelNote, type ScoreModel } from "./model";

export type ApplyPatchOp = {
  op: "replace_region";
  region_start: number;
  region_length: number;
  time_unit: "beats";
  notes: Array<{ pitch: number; start: number; length: number }>;
};

export type KernelBridgeResult<T> = { ok: true; value: T } | { ok: false; error: string };

export type ScoreClipSnapshot = {
  model: ScoreModel;
  // clip 全长（拍）——apply 的 replace_region 覆盖范围（整段覆盖语义）。
  // SCORE-ENTRY-REACH-1 (D4)：桥面网格化——clip 秒边界换拍后不必落在 16 分网格
  //（取证 382：373 模板 clip 8.0s@80bpm=10.6667 拍），渲染/apply 用向上取整的
  // 网格值（音符完整性保住，clip 尾部至多多 1/16 空白）；raw 保留披露真值。
  clipLengthBeats: number;
  // 网格化前的精确换算值（烟测/显形用；与 clipLengthBeats 相等=恰在网格上）。
  clipLengthBeatsRaw?: number;
  // 换算依据披露（面板显示用；来源=music_context_echo）。
  tempoBPM: number;
  timeSignatureLabel: string;
  // clip 全局 beat 起点（M4 游标换算用；秒→beat 需要）。
  clipStartSeconds: number;
};

// snapClipLengthToGrid：clip 拍长→16 分网格（向上取整，浮点 eps 安全）。
// 音符网格校验不变（fail-closed）；clip 边界是容器不是音符——合法工程可有
// 非网格 clip 尾（tempo 改后/拍数值模板），桥层宽容到下一网格点。
export function snapClipLengthToGrid(beats: number): number {
  if (!Number.isFinite(beats) || beats <= 0) {
    throw new Error(`clip 拍长不可用（${beats}）——无法网格化`);
  }
  const ticks = beats * GRID_PER_BEAT;
  const snapped = Math.ceil(ticks - 1e-9);
  return snapped / GRID_PER_BEAT;
}

// ScoreKernelBridge：面板对内核的全部依赖面。
export interface ScoreKernelBridge {
  readonly kind: "agent" | "none";
  applyPatch(patch: ApplyPatchOp): Promise<KernelBridgeResult<ScoreClipSnapshot>>;
  reload(): Promise<KernelBridgeResult<ScoreClipSnapshot | null>>;
}

// createNoKernelBridge：本地模式（无 agent）——apply/reload fail-honest，不静默假成功。
export function createNoKernelBridge(): ScoreKernelBridge {
  return {
    kind: "none",
    applyPatch: async () => ({ ok: false, error: "未连接内核（本地模式）：写入需真实 agent 栈" }),
    reload: async () => ({ ok: false, error: "未连接内核（本地模式）：回读需真实 agent 栈" })
  };
}

// invoke 注入缝（测试用）；生产走 lib/api invokeAgent（/agent/invoke 生产缝）。
export type InvokeFn = (payload: { tool?: string; args?: JsonRecord; confirmed?: boolean; source?: string }) => Promise<AgentInvokeResponse>;

// 桥工厂：面板持有 clip 选择（多 clip 工程=常态——课程模板/演示工程多轨多 clip；
// 桥内 clipID 参与 harness 的 track/clip 目标解析，多 clip 无指定=harness 正确拒绝）。
export type ScoreKernelBridgeFactory = (options: AgentScoreBridgeOptions) => ScoreKernelBridge;

export type AgentScoreBridgeOptions = {
  clipID?: string;
  trackID?: string;
  invoke?: InvokeFn;
};

const GRID_EPSILON_BEATS = 1e-4;

function asNumber(value: unknown): number | null {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

function resultOf(response: AgentInvokeResponse): JsonRecord {
  if ((response.status ?? "") !== "ok") {
    throw new Error(response.error || `invoke 状态 ${response.status || "unknown"}`);
  }
  const result = response.result;
  if (!result || typeof result !== "object") {
    throw new Error("invoke 返回缺 result");
  }
  return result;
}

export function createAgentScoreBridge(options: AgentScoreBridgeOptions = {}): ScoreKernelBridge {
  const invoke: InvokeFn = options.invoke ?? ((payload) => invokeAgent(payload));
  const clipArgs = (): JsonRecord => {
    const args: JsonRecord = {};
    if (options.clipID) {
      args.clip_id = options.clipID;
    }
    if (options.trackID) {
      args.track_id = options.trackID;
    }
    return args;
  };
  const invokeTool = async (tool: string, args: JsonRecord, confirmed = false): Promise<JsonRecord> =>
    resultOf(await invoke({ tool, args, confirmed, source: "ask_vit_webui_score" }));

  // readMusicContext：经 analyze_midi_transformations 的 music_context_echo 披露
  // （与 edu 判定器同款换算依据；多段 tempo/拍号 fail-honest）。
  const readMusicContext = async (): Promise<{
    tempoBPM: number;
    numerator: number;
    denominator: number;
    label: string;
  }> => {
    const analysis = await invokeTool("analyze_midi_transformations", { ...clipArgs() });
    const echo = (analysis.music_context_echo ?? {}) as JsonRecord;
    const tempo = asNumber(echo.tempo_bpm);
    const numerator = asNumber(echo.time_signature_numerator);
    const denominator = asNumber(echo.time_signature_denominator);
    const tempoCount = asNumber(echo.tempo_setting_count);
    const timeSigCount = asNumber(echo.time_sig_setting_count);
    if (tempo === null || numerator === null || denominator === null) {
      throw new Error("music_context 不可用（tempo/拍号未披露）——谱面工作台需恒定 tempo 与单拍号工程");
    }
    if (tempoCount !== null && tempoCount > 1) {
      throw new Error(`工程 tempo_setting_count=${tempoCount}（多段 tempo）——beat 换算非线性，fail-honest 不渲染`);
    }
    if (timeSigCount !== null && timeSigCount > 1) {
      throw new Error(`工程 time_sig_setting_count=${timeSigCount}（多段拍号）——bar 换算不可靠，fail-honest 不渲染`);
    }
    if (tempo <= 0 || denominator !== 4 || numerator < 1 || numerator > 8) {
      throw new Error(`拍号/tempo 不可用（${numerator}/${denominator} @ ${tempo}bpm）——v1 支持 denominator=4`);
    }
    return {
      tempoBPM: tempo,
      numerator: numerator,
      denominator: denominator,
      label: `${numerator}/${denominator}`
    };
  };

  // clipLength 独立小步（readMusicContext 拿 clip 数据，reload/apply 共用）。
  // SCORE-ENTRY-REACH-1 (D4)：返回精确换算值（raw）；网格化在 reload 组装点统一做。
  const readClipGeometry = async (
    tempoBPM: number
  ): Promise<{ clipStartSeconds: number; clipLengthBeatsRaw: number }> => {
    const clipData = await invokeTool("get_midi_clip_data", { ...clipArgs() });
    const startSeconds = asNumber(clipData.start_seconds);
    const lengthSeconds = asNumber(clipData.length_seconds);
    if (startSeconds === null || lengthSeconds === null || lengthSeconds <= 0) {
      throw new Error("get_midi_clip_data 未披露 start_seconds/length_seconds——clip 覆盖范围不可得");
    }
    const clipStartSeconds: number = startSeconds;
    return { clipStartSeconds, clipLengthBeatsRaw: (lengthSeconds * tempoBPM) / 60 };
  };

  const reload = async (): Promise<KernelBridgeResult<ScoreClipSnapshot | null>> => {
    try {
      const context = await readMusicContext();
      const notesReply = await invokeTool("get_midi_clip_notes", { ...clipArgs() });
      const rows = Array.isArray(notesReply.notes) ? (notesReply.notes as JsonRecord[]) : null;
      if (rows === null) {
        throw new Error("get_midi_clip_notes 返回缺 notes[]");
      }
      const notes: KernelNote[] = [];
      for (const row of rows) {
        const id = typeof row.id === "string" ? row.id : "";
        const pitch = asNumber(row.pitch);
        const start = asNumber(row.start);
        const length = asNumber(row.length);
        if (!id || pitch === null || start === null || length === null) {
          throw new Error("音符行缺 id/pitch/start/length（fail-closed）");
        }
        const startTicks = start * GRID_PER_BEAT;
        const lengthTicks = length * GRID_PER_BEAT;
        if (
          Math.abs(startTicks - Math.round(startTicks)) > GRID_EPSILON_BEATS * GRID_PER_BEAT ||
          Math.abs(lengthTicks - Math.round(lengthTicks)) > GRID_EPSILON_BEATS * GRID_PER_BEAT
        ) {
          throw new Error(`音符 ${id} 不在 16 分网格（start=${start} length=${length}）——v1 不支持网格外时值`);
        }
        notes.push({
          id,
          pitch: Math.round(pitch),
          startBeats: Math.round(startTicks) / GRID_PER_BEAT,
          lengthBeats: Math.round(lengthTicks) / GRID_PER_BEAT
        });
      }
      // 单声部守门（DERIVATION_RULES §6）：同 tick 起音多音符=和弦，v1 拒绝。
      const byTick = new Map<number, number>();
      for (const note of notes) {
        const tick = Math.round(note.startBeats * GRID_PER_BEAT);
        byTick.set(tick, (byTick.get(tick) ?? 0) + 1);
      }
      for (const [tick, count] of byTick) {
        if (count > 1) {
          throw new Error(`clip 非单声部（tick ${tick} 有 ${count} 个同刻音符）——v1 不支持和弦`);
        }
      }
      const geometry = await readClipGeometry(context.tempoBPM);
      const model: ScoreModel = {
        timeSignature: { numerator: context.numerator, denominator: context.denominator },
        notes
      };
      const measureBeats = beatsPerMeasure(model.timeSignature);
      // SCORE-ENTRY-REACH-1 (D4)：raw 精确值先落小节下限再向上网格化——
      // 非网格 clip 尾（如 10.6667 拍）不再让 buildApplyPatch 抛 grid_misaligned。
      const clipLengthBeats = Math.max(
        snapClipLengthToGrid(geometry.clipLengthBeatsRaw),
        measureBeats
      );
      return {
        ok: true,
        value: {
          model,
          clipLengthBeats,
          clipLengthBeatsRaw: geometry.clipLengthBeatsRaw,
          tempoBPM: context.tempoBPM,
          timeSignatureLabel: context.label,
          clipStartSeconds: geometry.clipStartSeconds
        }
      };
    } catch (error) {
      return { ok: false, error: error instanceof Error ? error.message : String(error) };
    }
  };

  const applyPatch = async (patch: ApplyPatchOp): Promise<KernelBridgeResult<ScoreClipSnapshot>> => {
    try {
      // 写入：单次 confirmed 提交（UI 确认对话框后；治理链 RiskConfirm 语义）。
      await invokeTool(
        "apply_midi_note_patch",
        {
          ...clipArgs(),
          time_unit: "beats",
          operations: [patch]
        },
        true
      );
      // 回读验证（A5：回读比对一致后才允许面显已保存）。
      const readback = await reload();
      if (!readback.ok || !readback.value) {
        return { ok: false, error: `写入成功但回读失败：${readback.ok ? "空模型" : readback.error}` };
      }
      const expected = [...patch.notes]
        .map((note) => [note.pitch, Math.round(note.start * GRID_PER_BEAT), Math.round(note.length * GRID_PER_BEAT)] as const)
        .sort((a, b) => a[1] - b[1] || a[0] - b[0]);
      const actual = readback.value.model.notes
        .map((note) => [note.pitch, Math.round(note.startBeats * GRID_PER_BEAT), Math.round(note.lengthBeats * GRID_PER_BEAT)] as const)
        .sort((a, b) => a[1] - b[1] || a[0] - b[0]);
      const same =
        expected.length === actual.length &&
        expected.every((row, index) => row[0] === actual[index][0] && row[1] === actual[index][1] && row[2] === actual[index][2]);
      if (!same) {
        return { ok: false, error: "回读不一致（写入音符与内核回读集合不等）——不面显已保存" };
      }
      return { ok: true, value: readback.value };
    } catch (error) {
      return { ok: false, error: error instanceof Error ? error.message : String(error) };
    }
  };

  return { kind: "agent", applyPatch, reload };
}
