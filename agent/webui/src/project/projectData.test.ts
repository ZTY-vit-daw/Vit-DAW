/*
 * UI-PROJECT-1（346）语义红线单测。
 * 覆盖：选择≠应用、applied 只经服务端成功事件、失败保留未应用事实、
 * 撤销绑定真实 ref、试听不产生应用、输入四态、轨迹四态、A/B 准备链。
 */
import { describe, expect, it } from "vitest";
import {
  composerStateOf,
  emptyProjectState,
  reduceProject,
  requestMacroApply,
  type MacroCardView,
  type ProjectSurfaceState,
  type ProjectThreadItem
} from "./projectData";
import type { MacroOption } from "../components";

const OPTIONS: MacroOption[] = [
  { id: "warm", name: "温暖", description: "中频略厚。", contour: [0.2, 0.5, 0.8] },
  { id: "bright", name: "明亮", description: "高频提起。", contour: [0.3, 0.5, 0.9] },
  { id: "tight", name: "紧实", description: "低频收短。", contour: [0.8, 0.5, 0.3] }
];

function seedMacro(view: Partial<MacroCardView> = {}): ProjectSurfaceState {
  const base = emptyProjectState();
  const item: ProjectThreadItem = {
    kind: "macro",
    id: "m1",
    view: {
      object: "鼓组总线",
      name: "染色",
      options: OPTIONS,
      selectedId: null,
      state: "default",
      ...view
    }
  };
  return { ...base, thread: [item] };
}

function macroView(state: ProjectSurfaceState, id = "m1"): MacroCardView {
  const item = state.thread.find((t) => t.kind === "macro" && t.id === id);
  if (!item || item.kind !== "macro") throw new Error("macro item missing");
  return item.view;
}

describe("宏卡：选择 ≠ 应用", () => {
  it("切换选择只更新 selectedId，state 保持 default", () => {
    const s1 = reduceProject(seedMacro(), { type: "macroSelect", id: "m1", optionId: "warm" });
    const view = macroView(s1);
    expect(view.selectedId).toBe("warm");
    expect(view.state).toBe("default");
    expect(view.appliedOptionId).toBeUndefined();
  });

  it("applied 态下切换选择不产生第二个已应用，也不清除既有应用事实", () => {
    const applied = seedMacro({
      state: "applied",
      selectedId: "warm",
      appliedOptionId: "warm",
      appliedSnapshotRef: "snap-1"
    });
    const s2 = reduceProject(applied, { type: "macroSelect", id: "m1", optionId: "bright" });
    const view = macroView(s2);
    expect(view.state).toBe("applied");
    expect(view.appliedOptionId).toBe("warm");
    expect(view.appliedSnapshotRef).toBe("snap-1");
  });
});

describe("宏卡：applied 只能经服务端成功事件", () => {
  it("未发起应用（无 pending）时成功事件被拒绝", () => {
    const s1 = reduceProject(seedMacro({ selectedId: "warm" }), {
      type: "macroApplySucceeded",
      id: "m1",
      optionId: "warm",
      snapshotRef: "snap-1",
      time: "14:07"
    });
    expect(macroView(s1).state).toBe("default");
  });

  it("pending 匹配但选择与成功回执不一致 → 拒绝", () => {
    const seeded = seedMacro({ selectedId: "warm" });
    // 模拟 surface 已发起请求（pending）
    const pending: ProjectSurfaceState = { ...seeded, pendingMacroId: "m1" };
    const s2 = reduceProject(pending, {
      type: "macroApplySucceeded",
      id: "m1",
      optionId: "bright",
      snapshotRef: "snap-1",
      time: "14:07"
    });
    expect(macroView(s2).state).toBe("default");
  });

  it("成功回执缺 snapshotRef → 拒绝（无伪造已应用）", () => {
    const pending: ProjectSurfaceState = { ...seedMacro({ selectedId: "warm" }), pendingMacroId: "m1" };
    const s2 = reduceProject(pending, {
      type: "macroApplySucceeded",
      id: "m1",
      optionId: "warm",
      snapshotRef: "  ",
      time: "14:07"
    });
    expect(macroView(s2).state).toBe("default");
    expect(s2.pendingMacroId).toBe("m1");
  });

  it("服务端成功 → applied + 快照回执追加，回执绑定同一 ref", () => {
    const pending: ProjectSurfaceState = { ...seedMacro({ selectedId: "warm" }), pendingMacroId: "m1" };
    const s2 = reduceProject(pending, {
      type: "macroApplySucceeded",
      id: "m1",
      optionId: "warm",
      snapshotRef: "snap-9",
      time: "14:07"
    });
    const view = macroView(s2);
    expect(view.state).toBe("applied");
    expect(view.appliedOptionId).toBe("warm");
    expect(view.appliedSnapshotRef).toBe("snap-9");
    expect(s2.pendingMacroId).toBeNull();
    const receipt = s2.thread.find((t) => t.kind === "snapshot");
    expect(receipt && receipt.kind === "snapshot" && receipt.view.snapshotRef === "snap-9").toBe(true);
  });
});

describe("宏卡：失败保留未应用事实 + 重试", () => {
  it("失败 → failed + 原因，选择不清除", () => {
    const pending: ProjectSurfaceState = { ...seedMacro({ selectedId: "bright" }), pendingMacroId: "m1" };
    const s2 = reduceProject(pending, {
      type: "macroApplyFailed",
      id: "m1",
      reason: "音频设备被其他程序占用。"
    });
    const view = macroView(s2);
    expect(view.state).toBe("failed");
    expect(view.failureReason).toBe("音频设备被其他程序占用。");
    expect(view.selectedId).toBe("bright");
    expect(view.appliedOptionId).toBeUndefined();
  });

  it("重试 → default（原因清除、选择保留）", () => {
    const failed = seedMacro({ state: "failed", selectedId: "bright", failureReason: "x" });
    const s2 = reduceProject(failed, { type: "macroRetry", id: "m1" });
    const view = macroView(s2);
    expect(view.state).toBe("default");
    expect(view.failureReason).toBeUndefined();
    expect(view.selectedId).toBe("bright");
  });
});

describe("宏卡：撤销绑定真实数据", () => {
  const applied = seedMacro({
    state: "applied",
    selectedId: "warm",
    appliedOptionId: "warm",
    appliedSnapshotRef: "snap-1"
  });

  it("ref 不一致 → 拒绝撤销", () => {
    const s2 = reduceProject(applied, { type: "macroUndo", id: "m1", snapshotRef: "snap-other" });
    expect(macroView(s2).state).toBe("applied");
  });

  it("非 applied 态 → 拒绝撤销", () => {
    const s2 = reduceProject(seedMacro({ selectedId: "warm" }), {
      type: "macroUndo",
      id: "m1",
      snapshotRef: "snap-1"
    });
    expect(macroView(s2).state).toBe("default");
  });

  it("ref 一致 → 退回 default + 回执标记已撤销", () => {
    const withReceipt: ProjectSurfaceState = {
      ...applied,
      thread: [
        ...applied.thread,
        { kind: "snapshot", id: "snapshot-snap-1", view: { time: "14:07", snapshotRef: "snap-1", macroItemId: "m1" } }
      ]
    };
    const s2 = reduceProject(withReceipt, { type: "macroUndo", id: "m1", snapshotRef: "snap-1" });
    const view = macroView(s2);
    expect(view.state).toBe("default");
    expect(view.appliedSnapshotRef).toBeUndefined();
    const receipt = s2.thread.find((t) => t.kind === "snapshot");
    expect(receipt && receipt.kind === "snapshot" && receipt.view.undone).toBe(true);
  });
});

describe("宏卡：试听", () => {
  it("仅 default 可进入试听；applied/failed 不试听", () => {
    const s1 = reduceProject(seedMacro(), { type: "macroAuditionStart", id: "m1", optionId: "warm" });
    expect(macroView(s1).state).toBe("auditioning");
    const s2 = reduceProject(
      seedMacro({ state: "applied", appliedOptionId: "warm", appliedSnapshotRef: "s", selectedId: "warm" }),
      { type: "macroAuditionStart", id: "m1", optionId: "warm" }
    );
    expect(macroView(s2).state).toBe("applied");
  });

  it("取消试听回到 default，不产生应用事实", () => {
    const s1 = reduceProject(seedMacro(), { type: "macroAuditionStart", id: "m1", optionId: "warm" });
    const s2 = reduceProject(s1, { type: "macroAuditionCancel", id: "m1" });
    const view = macroView(s2);
    expect(view.state).toBe("default");
    expect(view.appliedOptionId).toBeUndefined();
    expect(view.appliedSnapshotRef).toBeUndefined();
  });
});

describe("应用请求守卫 requestMacroApply", () => {
  it("未选择 / 非 default / 已有在途 → null", () => {
    expect(requestMacroApply(seedMacro(), "m1")).toBeNull();
    expect(requestMacroApply(seedMacro({ state: "auditioning", selectedId: "warm" }), "m1")).toBeNull();
    expect(requestMacroApply(seedMacro({ state: "applied", selectedId: "warm", appliedSnapshotRef: "s" }), "m1")).toBeNull();
    const pending: ProjectSurfaceState = { ...seedMacro({ selectedId: "warm" }), pendingMacroId: "m1" };
    expect(requestMacroApply(pending, "m1")).toBeNull();
  });

  it("default + 已选择 → 返回请求", () => {
    const s = seedMacro({ selectedId: "tight" });
    expect(requestMacroApply(s, "m1")).toEqual({ macroId: "m1", optionId: "tight" });
  });
});

describe("输入区四态", () => {
  it("优先级 disabled > replying > focus > default", () => {
    const base = emptyProjectState();
    expect(composerStateOf(base)).toBe("default");
    expect(composerStateOf({ ...base, composerFocused: true })).toBe("focus");
    expect(composerStateOf({ ...base, replying: true })).toBe("replying");
    expect(composerStateOf({ ...base, replying: true, composerFocused: true })).toBe("replying");
    expect(composerStateOf({ ...base, inputDisabled: true, replying: true })).toBe("disabled");
  });
});

describe("执行轨迹四态", () => {
  it("busy → 步骤累积（仅 busy 时）→ done 收成回执", () => {
    let s = emptyProjectState();
    s = reduceProject(s, { type: "turnStarted", id: "t1", title: "正在应用宏「温暖」" });
    s = reduceProject(s, { type: "turnStepDone", id: "t1", label: "写入 6 个绑定参数", duration: "0.2s" });
    s = reduceProject(s, { type: "turnStepDone", id: "t1", label: "渲染对比 A/B", duration: "3.1s" });
    s = reduceProject(s, { type: "turnSucceeded", id: "t1" });
    // done 后步骤事件不再追加
    s = reduceProject(s, { type: "turnStepDone", id: "t1", label: "迟到的步骤" });
    const traj = s.thread.find((t) => t.kind === "trajectory");
    expect(traj && traj.kind === "trajectory" && traj.view.state).toBe("done");
    expect(traj && traj.kind === "trajectory" && traj.view.steps.length).toBe(2);
  });

  it("busy → error 带原因", () => {
    let s = emptyProjectState();
    s = reduceProject(s, { type: "turnStarted", id: "t1", title: "正在渲染对比 A/B" });
    s = reduceProject(s, { type: "turnFailed", id: "t1", reason: "音频设备被占用。" });
    const traj = s.thread.find((t) => t.kind === "trajectory");
    expect(traj && traj.kind === "trajectory" && traj.view.state).toBe("error");
    expect(traj && traj.kind === "trajectory" && traj.view.failureReason).toBe("音频设备被占用。");
  });
});

describe("A/B 准备链", () => {
  function seedAB(): ProjectSurfaceState {
    const base = emptyProjectState();
    return {
      ...base,
      thread: [{ kind: "ab", id: "ab1", view: { state: "preparing", title: "鼓组总线 · 染色 对比" } }]
    };
  }
  it("preparing → ready 携带真实渲染引用", () => {
    const s = reduceProject(seedAB(), {
      type: "abPrepareSucceeded",
      id: "ab1",
      bypassRef: "render-bypass-1",
      processedRef: "render-processed-1"
    });
    const item = s.thread.find((t) => t.kind === "ab");
    expect(item && item.kind === "ab" && item.view.state).toBe("ready");
    expect(item && item.kind === "ab" && item.view.bypassRef).toBe("render-bypass-1");
  });
  it("preparing → failed 带原因；非 preparing 拒绝成功事件", () => {
    const s1 = reduceProject(seedAB(), { type: "abPrepareFailed", id: "ab1", reason: "渲染失败" });
    const item1 = s1.thread.find((t) => t.kind === "ab");
    expect(item1 && item1.kind === "ab" && item1.view.state).toBe("failed");
    const s2 = reduceProject(s1, {
      type: "abPrepareSucceeded",
      id: "ab1",
      bypassRef: "b",
      processedRef: "p"
    });
    const item2 = s2.thread.find((t) => t.kind === "ab");
    expect(item2 && item2.kind === "ab" && item2.view.state).toBe("failed");
  });
});

describe("消息线程", () => {
  it("空文本不产生消息条目", () => {
    let s = emptyProjectState();
    s = reduceProject(s, { type: "userSent", id: "u1", text: "   " });
    expect(s.thread.length).toBe(0);
    s = reduceProject(s, { type: "userSent", id: "u1", text: "把鼓组压紧一点" });
    expect(s.thread.length).toBe(1);
  });
});
