// @vitest-environment jsdom
// SCORE-WORKBENCH-1 M2：面板交互测试（react-dom/client + React19 act；零新测试依赖）。
// jsdom 24 无 PointerEvent/ResizeObserver/setPointerCapture——事件用 MouseEvent 派发
// "pointerdown/move/up"（React 根委托按事件类型分发；面板代码 ?. 守卫 pointer capture）。
// 点击坐标由 renderScoreInto 同模型确定性布局反算（369 亲证：同输入同布局）。
import { createElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";

import { vi } from "vitest";
import { ScoreWorkbenchPanel } from "./ScoreWorkbenchPanel";

// M4 游标测试缝：mock /agent/ui/state 轮询（ui_context 白名单键族）。
const uiStateMock = vi.fn(async () => ({
  status: "ok",
  ui_context: { playhead_seconds: 1.0, is_playing: true }
}));
vi.mock("../lib/api", () => ({
  invokeAgent: async () => ({ status: "error", error: "not under test" }),
  fetchUIState: () => uiStateMock()
}));
import type { ScoreKernelBridge, ApplyPatchOp, KernelBridgeResult } from "./kernel";
import { deserializeModel, serializeModel, type ScoreModel } from "./model";
import { beatsPerMeasure } from "./model";
import { renderScoreInto } from "./render";

(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const fixture: ScoreModel = {
  timeSignature: { numerator: 2, denominator: 4 },
  notes: [
    { id: "k1", pitch: 72, startBeats: 0, lengthBeats: 0.75 },
    { id: "k2", pitch: 67, startBeats: 0.75, lengthBeats: 0.25 },
    { id: "k3", pitch: 71, startBeats: 1.5, lengthBeats: 0.5 }
  ]
};

// 相邻同音高对（延音建立 fixture）——b 跨拍线（tick4），合并后 a=[0,6) 构成延音链
const tieFixture: ScoreModel = {
  timeSignature: { numerator: 2, denominator: 4 },
  notes: [
    { id: "a", pitch: 72, startBeats: 0, lengthBeats: 0.5 },
    { id: "b", pitch: 72, startBeats: 0.5, lengthBeats: 1.0 }
  ]
};

function mount(element: React.ReactElement): { root: Root; container: HTMLElement } {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  act(() => {
    root.render(element);
  });
  return { root, container };
}

function fire(target: HTMLElement, type: string, options: { clientX?: number; clientY?: number; button?: number } = {}): void {
  const event = new MouseEvent(type, {
    bubbles: true,
    cancelable: true,
    clientX: options.clientX ?? 0,
    clientY: options.clientY ?? 0,
    button: options.button ?? 0
  });
  act(() => {
    target.dispatchEvent(event);
  });
}

// anchor：反算某 tick 符号的点击坐标（独立 scratch 渲染，确定性同布局）。
function anchorFor(model: ScoreModel, tick: number): { x: number; y: number } {
  const scratch = document.createElement("div");
  document.body.appendChild(scratch);
  const result = renderScoreInto(scratch, model);
  const measure = Math.floor(tick / (beatsPerMeasure(model.timeSignature) * 4));
  const row = result.tickRows.get(measure) ?? [];
  const point = row.find(([t]) => t === tick) ?? row[0];
  const bounds = result.staveBounds.get(measure);
  return { x: point[1] + 1, y: (bounds?.y ?? 0) + 20 };
}

function overlayOf(container: HTMLElement): HTMLElement {
  const overlay = container.querySelector<HTMLElement>("[data-score-overlay]");
  if (!overlay) throw new Error("overlay missing");
  return overlay;
}

function rootEl(container: HTMLElement): HTMLElement {
  const root = container.querySelector<HTMLElement>("[data-score-root]");
  if (!root) throw new Error("score root missing");
  return root;
}

function modelOf(container: HTMLElement): ScoreModel {
  return deserializeModel(rootEl(container).getAttribute("data-score-model") ?? "");
}


function clickAt(anchor: { x: number; y: number }): { clientX: number; clientY: number } {
  return { clientX: anchor.x, clientY: anchor.y };
}

function clickDur(container: HTMLElement, value: number): void {
  const button = container.querySelector<HTMLElement>(`[data-score-dur="${value}"]`);
  if (!button) throw new Error(`duration button ${value} missing`);
  fire(button, "click");
}

function clickButton(container: HTMLElement, attr: string): void {
  const button = container.querySelector<HTMLElement>(`[${attr}]`);
  if (!button) throw new Error(`button ${attr} missing`);
  fire(button, "click");
}

afterEach(() => {
  document.body.textContent = "";
});

describe("ScoreWorkbenchPanel interactions (M2)", () => {
  it("renders initial fixture: svg present, clean state, local kernel tag", () => {
    const { container } = mount(createElement(ScoreWorkbenchPanel, { initialModel: fixture }));
    const root = rootEl(container);
    expect(root.getAttribute("data-score-dirty")).toBe("false");
    expect(root.getAttribute("data-score-kernel")).toBe("local");
    expect(root.getAttribute("data-score-note-count")).toBe("3");
    expect(container.querySelector("[data-score-svg] svg")).not.toBeNull();
    expect(serializeModel(modelOf(container))).toBe(serializeModel(fixture));
  });

  it("click empty rest cell in note mode places a note (dirty, +1)", () => {
    const { container } = mount(createElement(ScoreWorkbenchPanel, { initialModel: fixture }));
    // tick 4 = beat 1.0（八分休止窗）
    const anchor = anchorFor(fixture, 4);
    fire(overlayOf(container), "pointerdown", { clientX: anchor.x, clientY: anchor.y });
    const model = modelOf(container);
    expect(model.notes).toHaveLength(4);
    const placed = model.notes.find((note) => note.startBeats === 1 && note.lengthBeats === 0.5);
    expect(placed).toBeDefined();
    expect(placed?.pitch).toBe(72); // 默认音高 C5
    expect(rootEl(container).getAttribute("data-score-dirty")).toBe("true");
  });

  it("rest mode click erases the clicked span (paint: full cover deletes, partial cover trims)", () => {
    const { container } = mount(createElement(ScoreWorkbenchPanel, { initialModel: fixture }));
    clickButton(container, "data-score-mode-rest");
    // 八分休止 @tick0：k1[0,3) 部分覆盖 → 截短为 [2,3)（十六分）
    const anchor = anchorFor(fixture, 0);
    fire(overlayOf(container), "pointerdown", { clientX: anchor.x, clientY: anchor.y });
    let model = modelOf(container);
    const k1 = model.notes.find((note) => note.id === "k1");
    expect(k1).toBeDefined();
    expect(k1?.startBeats).toBe(0.5);
    expect(k1?.lengthBeats).toBe(0.25);
    // 四分休止 @tick0：剩余 k1 段与 k2 全覆盖 → 删除
    clickDur(container, 4);
    const anchor2 = anchorFor({ timeSignature: fixture.timeSignature, notes: model.notes }, 0);
    fire(overlayOf(container), "pointerdown", { clientX: anchor2.x, clientY: anchor2.y });
    model = modelOf(container);
    expect(model.notes.some((note) => note.id === "k1")).toBe(false);
    expect(model.notes.some((note) => note.id === "k2")).toBe(false);
    expect(model.notes).toHaveLength(1);
  });

  it("duration palette switches placed size; with selection it resizes the note", () => {
    const { container } = mount(createElement(ScoreWorkbenchPanel, { initialModel: fixture }));
    clickDur(container, 4); // 四分
    const anchor = anchorFor(fixture, 4);
    fire(overlayOf(container), "pointerdown", { clientX: anchor.x, clientY: anchor.y });
    let model = modelOf(container);
    let placed = model.notes.find((note) => note.startBeats === 1);
    expect(placed?.lengthBeats).toBe(1);
    // 选中后切时值 → resize
    const noteAnchor = anchorFor(model, 4);
    fire(overlayOf(container), "pointerdown", { clientX: noteAnchor.x, clientY: noteAnchor.y });
    expect(rootEl(container).getAttribute("data-score-selection")).not.toBe("");
    clickDur(container, 2);
    model = modelOf(container);
    placed = model.notes.find((note) => note.id === placed?.id);
    expect(placed?.lengthBeats).toBe(0.5);
  });

  it("dot toggle on selected note cycles eighth <-> dotted eighth", () => {
    const { container } = mount(createElement(ScoreWorkbenchPanel, { initialModel: fixture }));
    const anchor = anchorFor(fixture, 6); // k3 起点 tick6
    fire(overlayOf(container), "pointerdown", { clientX: anchor.x, clientY: anchor.y });
    expect(rootEl(container).getAttribute("data-score-selection")).toBe("k3");
    clickButton(container, "data-score-toggle-dot");
    let model = modelOf(container);
    expect(model.notes.find((note) => note.id === "k3")?.lengthBeats).toBe(0.75);
    clickButton(container, "data-score-toggle-dot");
    model = modelOf(container);
    expect(model.notes.find((note) => note.id === "k3")?.lengthBeats).toBe(0.5);
  });

  it("tie establish merges adjacent same-pitch pair; tie release splits the chain", () => {
    const { container } = mount(createElement(ScoreWorkbenchPanel, { initialModel: tieFixture }));
    const anchor = anchorFor(tieFixture, 0);
    fire(overlayOf(container), "pointerdown", { clientX: anchor.x, clientY: anchor.y });
    clickButton(container, "data-score-tie-establish");
    let model = modelOf(container);
    expect(model.notes).toHaveLength(1);
    expect(model.notes[0].lengthBeats).toBe(1.5); // a+b=[0,6) ticks，跨拍线
    // 现在是延音链（八分+四分 tie）——选中链首符号再解除（在链首末边界断开）
    const chainAnchor = anchorFor(model, 0);
    fire(overlayOf(container), "pointerdown", { clientX: chainAnchor.x, clientY: chainAnchor.y });
    clickButton(container, "data-score-tie-release");
    model = modelOf(container);
    expect(model.notes).toHaveLength(2);
    expect(model.notes.map((note) => [note.startBeats, note.lengthBeats]).sort()).toEqual([
      [0, 1],
      [1, 0.5]
    ]);
  });

  it("delete removes the selected note", () => {
    const { container } = mount(createElement(ScoreWorkbenchPanel, { initialModel: fixture }));
    const anchor = anchorFor(fixture, 3); // k2
    fire(overlayOf(container), "pointerdown", { clientX: anchor.x, clientY: anchor.y });
    expect(rootEl(container).getAttribute("data-score-selection")).toBe("k2");
    clickButton(container, "data-score-delete");
    const model = modelOf(container);
    expect(model.notes.some((note) => note.id === "k2")).toBe(false);
  });

  it("drag moves a note by grid steps (pointerdown on note, pointerup elsewhere)", () => {
    const { container } = mount(createElement(ScoreWorkbenchPanel, { initialModel: fixture }));
    const down = anchorFor(fixture, 6); // k3 @tick6
    const modelAfterSelection: ScoreModel = fixture; // 拖动前模型不变
    const up = anchorFor(modelAfterSelection, 4); // 目标 tick4
    fire(overlayOf(container), "pointerdown", { clientX: down.x, clientY: down.y });
    fire(overlayOf(container), "pointerup", { clientX: up.x, clientY: down.y }); // 保持同行
    const model = modelOf(container);
    const moved = model.notes.find((note) => note.id === "k3");
    expect(moved?.startBeats).toBe(1);
    expect(moved?.lengthBeats).toBe(0.5);
  });

  it("apply without kernel is fail-honest: confirm shows error, no silent success", () => {
    const { container } = mount(createElement(ScoreWorkbenchPanel, { initialModel: fixture }));
    const anchor = anchorFor(fixture, 4);
    fire(overlayOf(container), "pointerdown", { clientX: anchor.x, clientY: anchor.y });
    clickButton(container, "data-score-apply");
    expect(container.querySelector("[data-score-confirm]")).not.toBeNull();
    clickButton(container, "data-score-confirm-apply");
    const root = rootEl(container);
    expect(root.getAttribute("data-score-apply-state")).toBe("error");
    expect(root.getAttribute("data-score-status")).toContain("未连接内核");
    expect(root.getAttribute("data-score-dirty")).toBe("true"); // 缓冲保留
  });

  it("discard reverts to the base model", () => {
    const { container } = mount(createElement(ScoreWorkbenchPanel, { initialModel: fixture }));
    fire(overlayOf(container), "pointerdown", clickAt(anchorFor(fixture, 4)));
    expect(rootEl(container).getAttribute("data-score-dirty")).toBe("true");
    clickButton(container, "data-score-discard");
    expect(serializeModel(modelOf(container))).toBe(serializeModel(fixture));
    expect(rootEl(container).getAttribute("data-score-dirty")).toBe("false");
  });

  it("mount with an agent bridge auto-loads the kernel snapshot (M3)", async () => {
    const kernelModel: ScoreModel = {
      timeSignature: { numerator: 4, denominator: 4 },
      notes: [
        { id: "kernel-1", pitch: 60, startBeats: 0, lengthBeats: 0.75 },
        { id: "kernel-2", pitch: 62, startBeats: 0.75, lengthBeats: 0.25 }
      ]
    };
    const snapshot = {
      model: kernelModel,
      clipLengthBeats: 8,
      tempoBPM: 120,
      timeSignatureLabel: "4/4",
      clipStartSeconds: 0
    };
    const bridge: ScoreKernelBridge = {
      kind: "agent",
      applyPatch: async () => ({ ok: false, error: "not under test" }),
      reload: async () => ({ ok: true, value: snapshot })
    };
    const { container } = mount(createElement(ScoreWorkbenchPanel, { initialModel: fixture, kernelFactory: () => bridge }));
    await act(async () => {
      await Promise.resolve();
    });
    const root = rootEl(container);
    expect(serializeModel(modelOf(container))).toBe(serializeModel(kernelModel));
    expect(root.getAttribute("data-score-dirty")).toBe("false");
    expect(root.getAttribute("data-score-timesig")).toBe("4/4");
    expect(root.getAttribute("data-score-tempo")).toBe("120");
  });

  it("mount load failure keeps the local model and surfaces the error (fail-honest)", async () => {
    const bridge: ScoreKernelBridge = {
      kind: "agent",
      applyPatch: async () => ({ ok: false, error: "not under test" }),
      reload: async () => ({ ok: false, error: "analyze_midi_transformations requires a connected kernel" })
    };
    const { container } = mount(createElement(ScoreWorkbenchPanel, { initialModel: fixture, kernelFactory: () => bridge }));
    await act(async () => {
      await Promise.resolve();
    });
    const root = rootEl(container);
    expect(serializeModel(modelOf(container))).toBe(serializeModel(fixture));
    expect(root.getAttribute("data-score-status")).toContain("装载失败");
    expect(root.getAttribute("data-score-dirty")).toBe("false");
  });

  it("apply with a kernel bridge: single replace_region payload, snapshot rebases, saved state", async () => {
    const applied: ApplyPatchOp[] = [];
    const baseModel: ScoreModel = {
      timeSignature: { numerator: 2, denominator: 4 },
      notes: [
        { id: "kernel-1", pitch: 72, startBeats: 0, lengthBeats: 0.75 },
        { id: "kernel-2", pitch: 67, startBeats: 0.75, lengthBeats: 0.25 },
        { id: "kernel-3", pitch: 71, startBeats: 1.5, lengthBeats: 0.5 }
      ]
    };
    const editedModel: ScoreModel = {
      timeSignature: { numerator: 2, denominator: 4 },
      notes: [
        { id: "kernel-1", pitch: 72, startBeats: 0, lengthBeats: 0.75 },
        { id: "kernel-2", pitch: 67, startBeats: 0.75, lengthBeats: 0.25 },
        { id: "kernel-3", pitch: 71, startBeats: 1.5, lengthBeats: 0.5 },
        { id: "kernel-4", pitch: 72, startBeats: 1, lengthBeats: 0.5 }
      ]
    };
    const baseSnapshot = { model: baseModel, clipLengthBeats: 8, tempoBPM: 90, timeSignatureLabel: "2/4", clipStartSeconds: 0 };
    const editedSnapshot = { model: editedModel, clipLengthBeats: 8, tempoBPM: 90, timeSignatureLabel: "2/4", clipStartSeconds: 0 };
    const bridge: ScoreKernelBridge = {
      kind: "agent",
      applyPatch: async (patch) => {
        applied.push(patch);
        return { ok: true, value: editedSnapshot };
      },
      reload: async () => ({ ok: true, value: baseSnapshot })
    };
    const { container } = mount(createElement(ScoreWorkbenchPanel, { initialModel: fixture, kernelFactory: () => bridge }));
    await act(async () => {
      await Promise.resolve();
    });
    expect(serializeModel(modelOf(container))).toBe(serializeModel(baseModel));
    // 在空档（tick4=beat1.0，baseModel 该处为休止）放一个八分 → dirty
    fire(overlayOf(container), "pointerdown", clickAt(anchorFor(baseModel, 4)));
    expect(rootEl(container).getAttribute("data-score-dirty")).toBe("true");
    clickButton(container, "data-score-apply");
    clickButton(container, "data-score-confirm-apply");
    await act(async () => {
      await Promise.resolve();
    });
    expect(applied).toHaveLength(1);
    expect(applied[0].op).toBe("replace_region");
    expect(applied[0].time_unit).toBe("beats");
    expect(applied[0].region_start).toBe(0);
    expect(applied[0].notes).toHaveLength(4);
    const root = rootEl(container);
    expect(root.getAttribute("data-score-apply-state")).toBe("saved");
    expect(root.getAttribute("data-score-dirty")).toBe("false");
    expect(serializeModel(modelOf(container))).toBe(serializeModel(editedModel));
    expect(root.getAttribute("data-score-kernel")).toBe("connected");
  });

  it("cursor renders from polled playhead with snapshot (M4): beat anchor + playing tag", async () => {
    const kernelModel: ScoreModel = {
      timeSignature: { numerator: 2, denominator: 4 },
      notes: [
        { id: "kernel-1", pitch: 72, startBeats: 0, lengthBeats: 0.75 },
        { id: "kernel-2", pitch: 67, startBeats: 0.75, lengthBeats: 0.25 },
        { id: "kernel-3", pitch: 71, startBeats: 1.5, lengthBeats: 0.5 }
      ]
    };
    const snapshot = { model: kernelModel, clipLengthBeats: 4, tempoBPM: 120, timeSignatureLabel: "2/4", clipStartSeconds: 0.5 };
    const bridge: ScoreKernelBridge = {
      kind: "agent",
      applyPatch: async () => ({ ok: false, error: "not under test" }),
      reload: async () => ({ ok: true, value: snapshot })
    };
    // playhead 1.0s、clip 起点 0.5s、120bpm → clip 内 1 拍
    const { container } = mount(createElement(ScoreWorkbenchPanel, { initialModel: fixture, kernelFactory: () => bridge }));
    await act(async () => {
      await Promise.resolve();
    });
    await act(async () => {
      await Promise.resolve();
    });
    const root = rootEl(container);
    expect(root.getAttribute("data-score-cursor-beat")).toBe("1.000");
    expect(root.getAttribute("data-score-cursor-playing")).toBe("true");
    const cursorEl = container.querySelector("[data-score-cursor]");
    expect(cursorEl).not.toBeNull();
    expect(cursorEl?.getAttribute("data-score-cursor-measure")).toBe("0");
    // 无快照（本地模式）不画游标、不轮询错位
    const localMount = mount(createElement(ScoreWorkbenchPanel, { initialModel: fixture }));
    expect(localMount.container.querySelector("[data-score-cursor]")).toBeNull();
    expect(rootEl(localMount.container).getAttribute("data-score-cursor-beat")).toBe("");
  });

  it("apply error keeps the buffer and surfaces the error", async () => {
    const bridge: ScoreKernelBridge = {
      kind: "agent",
      applyPatch: async () => ({ ok: false, error: "RiskConfirm rejected" }),
      reload: async () => ({ ok: false, error: "unreachable" })
    };
    const { container } = mount(createElement(ScoreWorkbenchPanel, { initialModel: fixture, kernelFactory: () => bridge }));
    await act(async () => {
      await Promise.resolve();
    });
    // 装载失败保留本地 fixture（fail-honest），编辑基于本地模型
    fire(overlayOf(container), "pointerdown", clickAt(anchorFor(fixture, 4)));
    clickButton(container, "data-score-apply");
    clickButton(container, "data-score-confirm-apply");
    await act(async () => {
      await Promise.resolve();
    });
    const root = rootEl(container);
    expect(root.getAttribute("data-score-apply-state")).toBe("error");
    expect(root.getAttribute("data-score-status")).toContain("RiskConfirm rejected");
    expect(root.getAttribute("data-score-dirty")).toBe("true");
  });
});
