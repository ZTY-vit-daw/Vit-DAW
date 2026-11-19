/*
 * UI-PROJECT-1（346）语义红线单测。
 * 覆盖：选择≠应用、applied 只经服务端成功事件、失败保留未应用事实、
 * 撤销绑定真实 ref、试听不产生应用、输入四态、轨迹四态、A/B 准备链。
 */
import { describe, expect, it } from "vitest";
import {
  composerStateOf,
  emptyProjectState,
  PROJECT_CONFIRM_APPROVE_TEXT,
  PROJECT_CONFIRM_CANCEL_TEXT,
  projectThreadItemsFromOutcome,
  reduceProject,
  replyAsksConfirmation,
  requestMacroApply,
  tutorProgressFromWorkflowData,
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

/* ── SHELL-6（池序 367）：回执载荷 → 交互卡映射 + 确认语义不变式 ── */

describe("SHELL-6：replyAsksConfirmation（LLM 兜底确认语义检测）", () => {
  it("紧口径命中：需要确认/请确认/要不要等确认语态", () => {
    for (const text of [
      "这个操作需要确认后再执行。",
      "要不要我先这样做？",
      "请确认后我再继续。",
      "我需要确认一下你的意图。",
      "是否确认切换工程？",
      "等你确认。",
      "确认后执行。"
    ]) {
      expect(replyAsksConfirmation(text)).toBe(true);
    }
  });

  it("普通陈述/指令/通知不命中", () => {
    for (const text of [
      "好——这就新建工程《下一段》并切换过去。",
      "已应用宏「温暖」。",
      "本轮没有候选，稍后再试。",
      "确认函已收到。", // 「确认」一词但非确认请求语态
      ""
    ]) {
      expect(replyAsksConfirmation(text)).toBe(false);
    }
  });
});

describe("SHELL-6：projectThreadItemsFromOutcome 不变式（需要确认⇒可点交互面）", () => {
  const nextID = (prefix: string) => `${prefix}-t`;

  it("needs_confirmation=true（chat 确认协议位）→ 必伴随确认承接卡（两键可点语义）", () => {
    const mapped = projectThreadItemsFromOutcome(
      { text: "当前操作仍在等待确认，请先确认执行或取消。", needsConfirmation: true },
      nextID
    );
    const confirm = mapped.items.find((item) => item.kind === "confirm");
    expect(confirm).toBeDefined();
    if (confirm?.kind === "confirm") {
      expect(confirm.view.approveText).toBe(PROJECT_CONFIRM_APPROVE_TEXT);
      expect(confirm.view.cancelText).toBe(PROJECT_CONFIRM_CANCEL_TEXT);
    }
  });

  it("LLM 纯文字确认语义（无 directive/卡兜底）→ 同样伴随确认承接卡", () => {
    const mapped = projectThreadItemsFromOutcome(
      { text: "这个改动比较大，需要确认一下再动手。" },
      nextID
    );
    expect(mapped.items.some((item) => item.kind === "confirm")).toBe(true);
    expect(mapped.autoDirective).toBeUndefined();
  });

  it("directive 回执 → directive 占位卡 + autoDirective（自动经桥执行语义）", () => {
    const mapped = projectThreadItemsFromOutcome(
      {
        text: "好——这就新建一个工程并切换过去。",
        directive: { action: "new_project", project_name: "新工程_1115_2330", requires_governance: true }
      },
      nextID
    );
    const directive = mapped.items.find((item) => item.kind === "directive");
    expect(directive).toBeDefined();
    expect(mapped.autoDirective?.action).toBe("new_project");
    // directive 轮的「切换确认」由宿主治理卡承接，不再叠确认承接卡。
    expect(mapped.items.some((item) => item.kind === "confirm")).toBe(false);
  });

  it("结果卡回执（多命中点选）→ cards 卡批，无 directive 无确认卡", () => {
    const mapped = projectThreadItemsFromOutcome(
      {
        text: "有好几个都像「混音」——点一张卡片直接切换：",
        cards: [
          { id: "c1", name: "鼓组混音v2（fixture）", filePath: "D:/x/鼓组混音v2.vit", directive: null },
          { id: "c2", name: "人声混音（fixture）", filePath: "D:/x/人声混音.vit", directive: null }
        ]
      },
      nextID
    );
    const cards = mapped.items.find((item) => item.kind === "cards");
    expect(cards && cards.kind === "cards" && cards.cards.length).toBe(2);
    expect(mapped.autoDirective).toBeUndefined();
    expect(mapped.items.some((item) => item.kind === "confirm")).toBe(false);
  });

  it("普通回复 → 仅消息条目", () => {
    const mapped = projectThreadItemsFromOutcome({ text: "已就绪。" }, nextID);
    expect(mapped.items.length).toBe(1);
    expect(mapped.items[0]?.kind).toBe("message");
  });
});

describe("TUTOR-SIM-3：workflow_data 进度解析", () => {
  it("只看结构化 step_index/step_total/completed", () => {
    expect(tutorProgressFromWorkflowData({ step_index: 1, step_total: 3, completed: false })).toEqual({
      stepIndex: 1,
      stepTotal: 3,
      completed: false
    });
    expect(tutorProgressFromWorkflowData({ step_index: 2, step_total: 3, completed: false })).toEqual({
      stepIndex: 2,
      stepTotal: 3,
      completed: false
    });
    expect(tutorProgressFromWorkflowData({ step_index: 3, step_total: 3, completed: true })).toEqual({
      stepIndex: 3,
      stepTotal: 3,
      completed: true
    });
  });

  it("缺字段/字符串数字/越界/非布尔 completed 一律 null（不猜测）", () => {
    const invalid: unknown[] = [
      null,
      undefined,
      {},
      [],
      "第 1/3 步",
      { step_index: 0, step_total: 3, completed: false },
      { step_index: 4, step_total: 3, completed: false },
      { step_index: 2, step_total: 0, completed: false },
      { step_index: "2", step_total: 3, completed: false },
      { step_index: 2, step_total: "3", completed: false },
      { step_index: 2, step_total: 3 },
      { step_index: 2, step_total: 3, completed: "false" },
      { step_index: 2.5, step_total: 3, completed: false }
    ];
    for (const value of invalid) {
      expect(tutorProgressFromWorkflowData(value)).toBeNull();
    }
  });
});

describe("SHELL-6：治理/确认卡 reducer", () => {
  it("governanceRequested 幂等（同一 request_id 不叠卡）；作答一次固定", () => {
    let s = emptyProjectState();
    const view = { requestID: "req-1", action: "new_project", currentProject: "测试工程甲（fixture）" };
    s = reduceProject(s, { type: "governanceRequested", id: "g1", view });
    s = reduceProject(s, { type: "governanceRequested", id: "g2", view });
    expect(s.thread.filter((item) => item.kind === "governance").length).toBe(1);
    s = reduceProject(s, { type: "governanceAnswered", id: "g1", choice: "switch" });
    s = reduceProject(s, { type: "governanceAnswered", id: "g1", choice: "cancel" });
    const gov = s.thread.find((item) => item.kind === "governance");
    expect(gov && gov.kind === "governance" && gov.answered).toBe("switch");
  });

  it("TUTOR-SIM-3：tutorViewUpdated 按工程隔离，空路径/非法进度零变化", () => {
    let s = emptyProjectState();
    s = reduceProject(s, {
      type: "tutorViewUpdated",
      projectPath: "D:/fixtures/tutor-a.vit",
      view: { stepIndex: 1, stepTotal: 3, completed: false, prompt: "第 1 步问题" }
    });
    s = reduceProject(s, {
      type: "tutorViewUpdated",
      projectPath: "D:/fixtures/tutor-b.vit",
      view: { stepIndex: 2, stepTotal: 3, completed: false, prompt: "B 工程" }
    });
    expect(s.tutorViews["D:/fixtures/tutor-a.vit"]).toEqual({
      stepIndex: 1,
      stepTotal: 3,
      completed: false,
      prompt: "第 1 步问题"
    });
    expect(s.tutorViews["D:/fixtures/tutor-b.vit"]?.stepIndex).toBe(2);

    // 空 prompt 保留上一次问题（服务端空回复不回空问题）。
    s = reduceProject(s, {
      type: "tutorViewUpdated",
      projectPath: "D:/fixtures/tutor-a.vit",
      view: { stepIndex: 2, stepTotal: 3, completed: false, prompt: "" }
    });
    expect(s.tutorViews["D:/fixtures/tutor-a.vit"]).toEqual({
      stepIndex: 2,
      stepTotal: 3,
      completed: false,
      prompt: "第 1 步问题"
    });

    const before = s;
    s = reduceProject(s, {
      type: "tutorViewUpdated",
      projectPath: "",
      view: { stepIndex: 3, stepTotal: 3, completed: true, prompt: "x" }
    });
    expect(s).toBe(before);
    s = reduceProject(s, {
      type: "tutorViewUpdated",
      projectPath: "D:/fixtures/tutor-a.vit",
      view: { stepIndex: 9, stepTotal: 3, completed: false, prompt: "越界" }
    });
    expect(s).toBe(before);
  });

  it("confirmAnswered 一次固定，不重放", () => {
    let s = emptyProjectState();
    s = reduceProject(s, {
      type: "outcomeItemsArrived",
      items: [{ kind: "confirm", id: "cf1", view: { note: "…", approveText: "确认执行", cancelText: "取消" } }]
    });
    s = reduceProject(s, { type: "confirmAnswered", id: "cf1", choice: "approve" });
    s = reduceProject(s, { type: "confirmAnswered", id: "cf1", choice: "cancel" });
    const confirm = s.thread.find((item) => item.kind === "confirm");
    expect(confirm && confirm.kind === "confirm" && confirm.answered).toBe("approve");
  });
});
