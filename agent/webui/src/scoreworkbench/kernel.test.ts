// SCORE-WORKBENCH-1 M3：agent 桥单元测试（invoke 注入缝；fail-honest 门全量）。
// 断言契约：music_context_echo 门（缺披露/多段 tempo/多段拍号/非 4 分母拒绝）、
// 回读解析（网格吸附/缺字段 fail-closed/同刻多音=非单声部拒绝）、clip 几何换算、
// apply=单次 confirmed replace_region 载荷、回读验证（不一致不面显已保存）。
import { describe, expect, it } from "vitest";

import { createAgentScoreBridge, type InvokeFn } from "./kernel";
import type { AgentInvokeResponse, JsonRecord } from "../types";

type Call = { tool: string; args: JsonRecord; confirmed: boolean };

function okReply(result: JsonRecord): AgentInvokeResponse {
  return { status: "ok", result };
}

const ECHO_4_4 = {
  tempo_bpm: 120,
  time_signature: "4/4",
  time_signature_numerator: 4,
  time_signature_denominator: 4,
  tempo_setting_count: 1,
  time_sig_setting_count: 1
};

const KERNEL_NOTES = [
  { id: "n-a", pitch: 72, start: 0, length: 0.75 },
  { id: "n-b", pitch: 67, start: 0.7499999, length: 0.25 } // 吸附到 0.75
];

function makeInvoke(overrides?: Partial<Record<string, unknown>>): { invoke: InvokeFn; calls: Call[] } {
  const calls: Call[] = [];
  const invoke: InvokeFn = async (payload) => {
    calls.push({ tool: payload.tool ?? "", args: payload.args ?? {}, confirmed: payload.confirmed ?? false });
    const tool = payload.tool ?? "";
    const custom = overrides?.[tool];
    if (custom !== null && typeof custom === "object" && "error" in custom) {
      const errorReply: AgentInvokeResponse = { status: "error", error: String(custom.error) };
      return errorReply;
    }
    switch (tool) {
      case "analyze_midi_transformations":
        return okReply({ status: "ok", music_context_echo: (custom as JsonRecord) ?? ECHO_4_4, limitations: [] });
      case "get_midi_clip_data":
        return okReply({ status: "ok", ...((custom as JsonRecord) ?? {}), start_seconds: 0, length_seconds: 4 });
      case "get_midi_clip_notes":
        return okReply({ status: "ok", notes: (custom as unknown[]) ?? KERNEL_NOTES });
      case "apply_midi_note_patch":
        return okReply({ status: "ok", applied: (custom as JsonRecord) ?? {} });
      default:
        return { status: "error", error: `unexpected tool ${tool}` };
    }
  };
  return { invoke, calls };
}

describe("agent score bridge (M3)", () => {
  it("reload builds the model with grid snap, timesig from echo, clip length from seconds*tempo/60", async () => {
    const { invoke } = makeInvoke();
    const bridge = createAgentScoreBridge({ invoke });
    const result = await bridge.reload();
    expect(result.ok).toBe(true);
    if (!result.ok || !result.value) throw new Error("unreachable");
    expect(result.value.model.timeSignature).toEqual({ numerator: 4, denominator: 4 });
    expect(result.value.model.notes.map((note) => [note.id, note.pitch, note.startBeats, note.lengthBeats])).toEqual([
      ["n-a", 72, 0, 0.75],
      ["n-b", 67, 0.75, 0.25]
    ]);
    expect(result.value.clipLengthBeats).toBe(8); // 4s * 120/60
    expect(result.value.tempoBPM).toBe(120);
    expect(result.value.timeSignatureLabel).toBe("4/4");
    expect(result.value.clipStartSeconds).toBe(0);
  });

  it("reload fails honest when music context is not disclosed", async () => {
    const { invoke } = makeInvoke({ analyze_midi_transformations: {} });
    const result = await createAgentScoreBridge({ invoke }).reload();
    expect(result.ok).toBe(false);
    if (result.ok) throw new Error("unreachable");
    expect(result.error).toContain("music_context");
  });

  it("reload fails honest on multi-segment tempo / time signature (fail-honest, no silent misalign)", async () => {
    const multi: { invoke: InvokeFn } = makeInvoke({
      analyze_midi_transformations: { ...ECHO_4_4, tempo_setting_count: 3 }
    });
    const tempoResult = await createAgentScoreBridge(multi).reload();
    expect(tempoResult.ok).toBe(false);
    if (tempoResult.ok) throw new Error("unreachable");
    expect(tempoResult.error).toContain("多段 tempo");

    const multiSig: { invoke: InvokeFn } = makeInvoke({
      analyze_midi_transformations: { ...ECHO_4_4, time_sig_setting_count: 2 }
    });
    const sigResult = await createAgentScoreBridge(multiSig).reload();
    expect(sigResult.ok).toBe(false);
    if (sigResult.ok) throw new Error("unreachable");
    expect(sigResult.error).toContain("多段拍号");
  });

  it("reload fails honest on non-4 denominator (v1 boundary)", async () => {
    const { invoke } = makeInvoke({
      analyze_midi_transformations: { ...ECHO_4_4, time_signature_numerator: 6, time_signature_denominator: 8 }
    });
    const result = await createAgentScoreBridge({ invoke }).reload();
    expect(result.ok).toBe(false);
    if (result.ok) throw new Error("unreachable");
    expect(result.error).toContain("denominator=4");
  });

  it("reload fails closed on missing note fields and off-grid values", async () => {
    const missing = makeInvoke({ get_midi_clip_notes: [{ id: "x", pitch: 60, start: 0 }] });
    const missingResult = await createAgentScoreBridge(missing).reload();
    expect(missingResult.ok).toBe(false);
    if (missingResult.ok) throw new Error("unreachable");
    expect(missingResult.error).toContain("fail-closed");

    const offGrid = makeInvoke({ get_midi_clip_notes: [{ id: "y", pitch: 60, start: 0.1, length: 0.5 }] });
    const offGridResult = await createAgentScoreBridge(offGrid).reload();
    expect(offGridResult.ok).toBe(false);
    if (offGridResult.ok) throw new Error("unreachable");
    expect(offGridResult.error).toContain("16 分网格");
  });

  it("reload rejects non-single-voice clips (same-tick chord)", async () => {
    const { invoke } = makeInvoke({
      get_midi_clip_notes: [
        { id: "c1", pitch: 60, start: 0, length: 0.5 },
        { id: "c2", pitch: 64, start: 0, length: 0.5 }
      ]
    });
    const result = await createAgentScoreBridge({ invoke }).reload();
    expect(result.ok).toBe(false);
    if (result.ok) throw new Error("unreachable");
    expect(result.error).toContain("非单声部");
  });

  it("applyPatch sends one confirmed replace_region and verifies readback (ok on match)", async () => {
    const { invoke, calls } = makeInvoke();
    const bridge = createAgentScoreBridge({ invoke, clipID: "clip-1" });
    const result = await bridge.applyPatch({
      op: "replace_region",
      region_start: 0,
      region_length: 8,
      time_unit: "beats",
      notes: KERNEL_NOTES.map((note) => ({ pitch: note.pitch, start: note.id === "n-b" ? 0.75 : 0, length: note.length }))
    });
    expect(result.ok).toBe(true);
    const applyCall = calls.find((call) => call.tool === "apply_midi_note_patch");
    expect(applyCall).toBeDefined();
    expect(applyCall?.confirmed).toBe(true);
    expect(applyCall?.args.clip_id).toBe("clip-1");
    expect(applyCall?.args.time_unit).toBe("beats");
    expect(applyCall?.args.operations).toHaveLength(1);
    expect((applyCall?.args.operations as JsonRecord[])[0].op).toBe("replace_region");
  });

  it("applyPatch fails honest when readback set differs from the patch (no fake saved state)", async () => {
    // 读回的音长与写入不同 → 回读不一致
    const drifted = makeInvoke({
      get_midi_clip_notes: [
        { id: "n-a", pitch: 72, start: 0, length: 0.5 },
        { id: "n-b", pitch: 67, start: 0.75, length: 0.25 }
      ]
    });
    const result = await createAgentScoreBridge(drifted).applyPatch({
      op: "replace_region",
      region_start: 0,
      region_length: 8,
      time_unit: "beats",
      notes: [
        { pitch: 72, start: 0, length: 0.75 },
        { pitch: 67, start: 0.75, length: 0.25 }
      ]
    });
    expect(result.ok).toBe(false);
    if (result.ok) throw new Error("unreachable");
    expect(result.error).toContain("回读不一致");
  });

  it("applyPatch surfaces the invoke error verbatim (governance rejections included)", async () => {
    const { invoke } = makeInvoke({ apply_midi_note_patch: { error: "authority_mode_not_active" } });
    const result = await createAgentScoreBridge({ invoke }).applyPatch({
      op: "replace_region",
      region_start: 0,
      region_length: 8,
      time_unit: "beats",
      notes: [{ pitch: 60, start: 0, length: 1 }]
    });
    expect(result.ok).toBe(false);
    if (result.ok) throw new Error("unreachable");
    expect(result.error).toContain("authority_mode_not_active");
  });
});
