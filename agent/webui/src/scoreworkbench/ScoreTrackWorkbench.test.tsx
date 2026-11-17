// @vitest-environment jsdom
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { AgentUIState } from "../types";
import { ScoreTrackWorkbench } from "./ScoreTrackWorkbench";
import { scoreCatalog, targetInCatalog } from "./binding";
import type { ApplyPatchOp, KernelBridgeResult, ScoreClipSnapshot, ScoreKernelBridgeFactory } from "./kernel";
import type { ScoreModel } from "./model";

const api = vi.hoisted(() => ({ fetchUIState: vi.fn() }));
vi.mock("../lib/api", () => ({ fetchUIState: api.fetchUIState, invokeAgent: vi.fn() }));
(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const model: ScoreModel = { timeSignature: { numerator: 2, denominator: 4 }, notes: [{ id: "n", pitch: 72, startBeats: 0, lengthBeats: .5 }] };
const snap = (value = model): ScoreClipSnapshot => ({ model: value, clipLengthBeats: 8, tempoBPM: 120, timeSignatureLabel: "2/4", clipStartSeconds: 0 });
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>((done) => { resolve = done; }); return { promise, resolve }; }
function fixture(path = "P:/first.vit"): AgentUIState {
  return { project: { project_path: path }, tracks: [
    { id: "a", name: "旋律甲", clips: [{ id: "a1", name: "主题甲", clip_type: "midi" }] },
    { id: "b", name: "旋律乙", clips: [{ id: "b1", name: "主题乙", clip_type: "midi" }] },
    { id: "many", name: "多段轨", clips: [{ id: "m1", name: "首段", clip_type: "midi" }, { id: "m2", name: "尾段", clip_type: "midi" }, { id: "wave2", clip_type: "wave" }] },
    { id: "empty", name: "空轨", clips: [] },
    { id: "audio", name: "音频轨", clips: [{ id: "wave", clip_type: "wave" }] },
    { id: "unknown", name: "未知类型", clips: [{ id: "u1" }] }
  ] };
}

let state: AgentUIState;
let root: Root;
let host: HTMLDivElement;
let writes: Array<{ clipID: string; patch: ApplyPatchOp }>;
let reloads: string[];
let load: (clipID: string) => Promise<KernelBridgeResult<ScoreClipSnapshot | null>>;
let apply: (clipID: string, patch: ApplyPatchOp) => Promise<KernelBridgeResult<ScoreClipSnapshot>>;
const factory: ScoreKernelBridgeFactory = ({ clipID = "", trackID }) => {
  expect(trackID).toBeTruthy();
  return { kind: "agent", reload: () => { reloads.push(clipID); return load(clipID); }, applyPatch: (patch) => { writes.push({ clipID, patch }); return apply(clipID, patch); } };
};
async function flush() { await act(async () => { await Promise.resolve(); }); }
async function mount() {
  host = document.createElement("div"); document.body.append(host); root = createRoot(host);
  await act(async () => root.render(createElement(ScoreTrackWorkbench, { kernelFactory: factory })));
}
function el<T extends HTMLElement = HTMLElement>(selector: string): T {
  const result = host.querySelector<T>(selector); if (!result) throw new Error(`Missing ${selector}`); return result;
}
async function click(selector: string) { await act(async () => el(selector).click()); }
async function select(value: string) { await act(async () => { const picker = el<HTMLSelectElement>("[data-score-clip-select]"); picker.value = value; picker.dispatchEvent(new Event("change", { bubbles: true })); }); }
async function dirty() {
  // Edit a real rendered note through the existing note-selection -> duration path.
  await act(async () => {
    const overlay = el("[data-score-overlay]");
    overlay.dispatchEvent(new MouseEvent("pointerdown", { bubbles: true, clientX: 20, clientY: 20 }));
  });
  // Tick zero geometry is published by the production renderer.
  const rows = el("[data-score-svg]").getAttribute("data-score-tick-rows")!;
  const x = Number(rows.match(/m0:0@(\d+)/)![1]);
  await act(async () => el("[data-score-overlay]").dispatchEvent(new MouseEvent("pointerdown", { bubbles: true, clientX: x + 1, clientY: 20 })));
  await click('[data-score-dur="4"]');
  expect(el("[data-score-root]").getAttribute("data-score-dirty")).toBe("true");
}
async function refresh() { await click("[data-score-refresh]"); }

beforeEach(() => {
  vi.spyOn(HTMLCanvasElement.prototype, "getContext").mockReturnValue({ measureText: () => ({ width: 0 }) } as unknown as CanvasRenderingContext2D);
  state = fixture(); writes = []; reloads = [];
  load = async () => ({ ok: true, value: snap() });
  apply = async (_clipID, patch) => ({ ok: true, value: snap({ ...model, notes: patch.notes.map((note, i) => ({ id: `saved-${i}`, pitch: note.pitch, startBeats: note.start, lengthBeats: note.length })) }) });
  api.fetchUIState.mockImplementation(async () => state);
});
afterEach(() => { if (root) act(() => root.unmount()); document.body.textContent = ""; vi.restoreAllMocks(); });

describe("397 production score binding", () => {
  it("requires an explicit track, shows only MIDI clips, and keeps empty/audio/unknown states honest", async () => {
    await mount(); expect(reloads).toEqual([]);
    await click('[data-score-track-id="many"]');
    expect(reloads).toEqual([]);
    expect([...el<HTMLSelectElement>("[data-score-clip-select]").options].map((option) => option.value)).toEqual(["", "m1", "m2"]);
    await select("m2"); expect(reloads).toEqual(["m2"]);
    expect(el("[data-score-editing-target]").textContent).toContain("多段轨 · 尾段");
    for (const [id, message] of [["empty", "还没有 MIDI"], ["audio", "只有音频"], ["unknown", "类型不可得"]]) {
      await click(`[data-score-track-id="${id}"]`); expect(el("[data-score-binding-empty]").textContent).toContain(message);
      expect(host.querySelector("[data-score-apply]")).toBeNull();
    }
    expect(writes).toEqual([]);
  });

  it("dirty A -> B cancel preserves identity; explicit discard loads failed B with no old buffer/write", async () => {
    await mount(); await click('[data-score-track-id="a"]'); await dirty();
    const before = el("[data-score-root]").getAttribute("data-score-model");
    await click('[data-score-track-id="b"]'); expect(host.querySelector("[data-score-switch-confirm]")).not.toBeNull();
    await click("[data-score-switch-cancel]");
    expect(el("[data-score-root]").getAttribute("data-score-clip-id")).toBe("a1");
    expect(el("[data-score-root]").getAttribute("data-score-model")).toBe(before);
    load = async () => ({ ok: false, error: "unsupported B" });
    await click('[data-score-track-id="b"]'); await click("[data-score-switch-discard]");
    expect(el("[data-score-root]").getAttribute("data-score-clip-id")).toBe("b1");
    expect(el("[data-score-root]").getAttribute("data-score-model")).not.toBe(before);
    expect(el("[data-score-root]").getAttribute("data-score-status")).toContain("unsupported B");
    expect(el<HTMLButtonElement>("[data-score-apply]").disabled).toBe(true);
    expect(writes).toEqual([]);
  });

  it("locks track changes and editing during apply, then ignores the old reply after a project switch", async () => {
    const result = deferred<KernelBridgeResult<ScoreClipSnapshot>>(); apply = () => result.promise;
    await mount(); await click('[data-score-track-id="a"]'); await dirty();
    await click("[data-score-apply]"); await click("[data-score-confirm-apply]");
    expect(writes.map((write) => write.clipID)).toEqual(["a1"]);
    expect(el<HTMLButtonElement>('[data-score-track-id="b"]').disabled).toBe(true);
    expect(el<HTMLFieldSetElement>(".score-toolbar").disabled).toBe(true);
    state = fixture("P:/second.vit"); await refresh();
    expect(el("[data-score-root]").getAttribute("data-score-write-ready")).toBe("false");
    await click('[data-score-track-id="b"]'); await click("[data-score-switch-discard]");
    expect(el("[data-score-root]").getAttribute("data-score-clip-id")).toBe("b1");
    await act(async () => result.resolve({ ok: true, value: snap({ ...model, notes: [{ ...model.notes[0], pitch: 12 }] }) }));
    expect(el("[data-score-root]").getAttribute("data-score-clip-id")).toBe("b1");
    expect(el("[data-score-root]").getAttribute("data-score-model")).not.toContain('"pitch":12');
    expect(el("[data-score-root]").getAttribute("data-score-apply-state")).toBe("idle");
  });

  it("ignores a delayed load after project identity changes even when track and clip IDs repeat", async () => {
    const result = deferred<KernelBridgeResult<ScoreClipSnapshot | null>>(); load = () => result.promise;
    await mount(); await click('[data-score-track-id="a"]');
    state = fixture("P:/second.vit"); await refresh();
    await act(async () => result.resolve({ ok: true, value: snap() }));
    expect(el("[data-score-root]").getAttribute("data-score-write-ready")).toBe("false");
    expect(el("[data-score-root]").getAttribute("data-score-note-count")).toBe("0");
  });

  it("revokes a loaded target on deletion or type change and does not overwrite dirty edits on recovery", async () => {
    await mount(); await click('[data-score-track-id="a"]'); await dirty();
    const before = el("[data-score-root]").getAttribute("data-score-model");
    state.tracks![0].clips = []; await refresh();
    expect(el<HTMLButtonElement>("[data-score-apply]").disabled).toBe(true);
    state = fixture(); await refresh();
    expect(el("[data-score-root]").getAttribute("data-score-model")).toBe(before);
    expect(el<HTMLButtonElement>("[data-score-apply]").disabled).toBe(true);
    await click("[data-score-discard]");
    expect(el("[data-score-root]").getAttribute("data-score-write-ready")).toBe("true");
    state.tracks![0].clips = [{ id: "a1", clip_type: "wave" }]; await refresh();
    expect(el<HTMLButtonElement>("[data-score-apply]").disabled).toBe(true); expect(writes).toEqual([]);
  });

  it("checks fresh state immediately before apply and rejects a project change before the poll sees it", async () => {
    await mount(); await click('[data-score-track-id="a"]'); await dirty(); await click("[data-score-apply]");
    state = fixture("P:/second.vit"); await click("[data-score-confirm-apply]");
    expect(writes).toEqual([]); expect(el<HTMLButtonElement>("[data-score-apply]").disabled).toBe(true);
  });

  it("retains the dirty buffer and blocks apply when the state endpoint fails", async () => {
    await mount(); await click('[data-score-track-id="a"]'); await dirty();
    api.fetchUIState.mockRejectedValue(new Error("offline")); await refresh();
    expect(el("[data-score-root]").getAttribute("data-score-dirty")).toBe("true");
    expect(el<HTMLButtonElement>("[data-score-apply]").disabled).toBe(true); expect(writes).toEqual([]);
  });

  it("writes only the explicit loaded clip and rebases after successful readback", async () => {
    await mount(); await click('[data-score-track-id="a"]'); await dirty();
    await click("[data-score-apply]"); await click("[data-score-confirm-apply]");
    expect(writes.map((write) => write.clipID)).toEqual(["a1"]);
    expect(el("[data-score-root]").getAttribute("data-score-apply-state")).toBe("saved");
    expect(el("[data-score-root]").getAttribute("data-score-dirty")).toBe("false");
  });

  it("catalog refresh does not follow GUI selection or overwrite a locally selected clip (398 boundary)", async () => {
    await mount(); await click('[data-score-track-id="a"]');
    state.ui_context = { selected_track_id: "b", selected_clip_id: "b1" }; await refresh();
    expect(el("[data-score-root]").getAttribute("data-score-clip-id")).toBe("a1");
    expect(reloads).toEqual(["a1"]);
  });

  it("disallows duplicate confirmation while a write is pending and preserves the buffer if apply rejects", async () => {
    const result = deferred<KernelBridgeResult<ScoreClipSnapshot>>(); apply = () => result.promise;
    await mount(); await click('[data-score-track-id="a"]'); await dirty();
    await click("[data-score-apply]"); await click("[data-score-confirm-apply]"); await click("[data-score-confirm-apply]");
    expect(writes).toHaveLength(1);
    await act(async () => result.resolve({ ok: false, error: "fixture write rejected" }));
    expect(el("[data-score-root]").getAttribute("data-score-dirty")).toBe("true");
    expect(el("[data-score-root]").getAttribute("data-score-status")).toContain("fixture write rejected");
    expect(host.querySelector("[data-score-confirm]")).toBeNull();
  });

  it("fails closed on missing project identity and never infers MIDI from absent type", () => {
    expect(() => scoreCatalog({ tracks: [] })).toThrow("工程身份不可得");
    const catalog = scoreCatalog(fixture());
    expect(catalog.tracks.find((track) => track.trackID === "unknown")!.clips).toHaveLength(0);
    expect(targetInCatalog(catalog, { projectKey: "P:/other.vit", trackID: "a", clipID: "a1", trackName: "", clipName: "" })).toBe(false);
  });
});
