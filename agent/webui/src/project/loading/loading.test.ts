/*
 * UI-LOADING-1（347）装载状态机单元测试（纯函数，无 DOM）。
 *
 * 红线断言（卡面）：错误路径必测（不只顺利完成）；无假倒计时（progress 只
 * 来自显式事件）；步骤 ≤3；输入禁用/恢复可断言；failed/interrupted 保留原因。
 */

import { describe, expect, it } from "vitest";
import {
  emptyLoadingView,
  loadingInputDisabled,
  LOADING_LIVE_STEP_LABEL,
  reduceLoading,
  type LoadingCardView
} from "./loadingData";
import { tapeReelAmounts } from "./tape";

function busyView(): LoadingCardView {
  return reduceLoading(reduceLoading(emptyLoadingView(), { type: "loadingStarted", title: "鼓组混音 v2" }), {
    type: "loadingStepCurrent",
    label: LOADING_LIVE_STEP_LABEL
  });
}

describe("装载状态机：进行中", () => {
  it("loadingStarted 建立装载态；空标题合法（无工程名时通用句式兜底，渲染层负责）", () => {
    const v = reduceLoading(emptyLoadingView(), { type: "loadingStarted", title: "鼓组混音 v2" });
    expect(v.phase).toBe("loading");
    expect(v.title).toBe("鼓组混音 v2");
    const blank = reduceLoading(emptyLoadingView(), { type: "loadingStarted", title: "   " });
    expect(blank.phase).toBe("loading");
    expect(blank.title).toBe("");
  });

  it("loadingStepCurrent：新 current 开工即旧 current 隐式转 done（轨迹保留）", () => {
    let v = busyView();
    expect(v.steps).toHaveLength(1);
    expect(v.steps[0]).toMatchObject({ label: LOADING_LIVE_STEP_LABEL, status: "current" });
    v = reduceLoading(v, { type: "loadingStepCurrent", label: "已恢复轨道" });
    expect(v.steps).toHaveLength(2);
    expect(v.steps[0]).toMatchObject({ label: LOADING_LIVE_STEP_LABEL, status: "done" });
    expect(v.steps[1]).toMatchObject({ label: "已恢复轨道", status: "current" });
  });

  it("loadingStepDone 只接受 loading 态与真实事件", () => {
    const done = reduceLoading(busyView(), { type: "loadingStepDone", label: "已读取工程文件" });
    expect(done.steps.some((s) => s.status === "done" && s.label === "已读取工程文件")).toBe(true);
    const rejected = reduceLoading({ ...busyView(), phase: "completed" }, { type: "loadingStepDone", label: "x" });
    expect(rejected.steps).toHaveLength(1);
  });

  it("无假倒计时：progress 只来自显式事件；reducer 不自增、不接受非有限值", () => {
    const v0 = busyView();
    expect(v0.progress).toBeNull(); // 未知进度 = null（不伪造）
    const v1 = reduceLoading(v0, { type: "loadingProgress", value: 0.64 });
    expect(v1.progress).toBeCloseTo(0.64);
    // 两次渲染之间无事件 → progress 不变（无自增/无定时器推进）
    expect(reduceLoading(v1, { type: "loadingStepDone", label: "x" }).progress).toBeCloseTo(0.64);
    // 非有限值拒绝
    expect(reduceLoading(v1, { type: "loadingProgress", value: Number.NaN }).progress).toBeCloseTo(0.64);
    // clamp 0..1
    expect(reduceLoading(v0, { type: "loadingProgress", value: 1.7 }).progress).toBe(1);
    expect(reduceLoading(v0, { type: "loadingProgress", value: -0.4 }).progress).toBe(0);
  });

  it("步骤同屏 ≤3：第 4 步进入时最早步骤出轨迹（保留最近 3 条）", () => {
    let v = busyView();
    v = reduceLoading(v, { type: "loadingStepCurrent", label: "步骤二" });
    v = reduceLoading(v, { type: "loadingStepCurrent", label: "步骤三" });
    expect(v.steps).toHaveLength(3);
    v = reduceLoading(v, { type: "loadingStepCurrent", label: "步骤四" });
    expect(v.steps).toHaveLength(3);
    // 4 条裁 3：最早一步（agent 连接步）出轨迹，保留最近 3 条
    expect(v.steps.map((s) => s.label)).toEqual(["步骤二", "步骤三", "步骤四"]);
    expect(v.steps[2]).toMatchObject({ label: "步骤四", status: "current" });
  });
});

describe("装载状态机：完成回执", () => {
  it("loadingSucceeded 收成 completed；结果注可选；耗时只来自事件", () => {
    let v = reduceLoading(busyView(), { type: "loadingStepDone", label: "已连接音频设备", duration: "1.2s" });
    v = reduceLoading(v, { type: "loadingSucceeded", note: "3 条轨道已恢复" });
    expect(v.phase).toBe("completed");
    expect(v.resultNote).toBe("3 条轨道已恢复");
    expect(v.steps).toHaveLength(2);
    expect(v.steps[1].duration).toBe("1.2s");
  });

  it("loading 时输入禁用；completed/failed/interrupted 恢复（loadingInputDisabled）", () => {
    expect(loadingInputDisabled(busyView())).toBe(true);
    const done = reduceLoading(busyView(), { type: "loadingSucceeded" });
    expect(loadingInputDisabled(done)).toBe(false);
    const fail = reduceLoading(busyView(), { type: "loadingFailed", reason: "x" });
    expect(loadingInputDisabled(fail)).toBe(false);
    const interrupted = reduceLoading(busyView(), { type: "loadingInterrupted", reason: "x" });
    expect(loadingInputDisabled(interrupted)).toBe(false);
  });
});

describe("装载状态机：错误与中断路径（必测）", () => {
  it("loadingFailed 写明原因；空原因拒绝（不渲染无原因错误卡）", () => {
    const v = reduceLoading(busyView(), { type: "loadingFailed", reason: "音频设备被其他程序占用。" });
    expect(v.phase).toBe("failed");
    expect(v.failureReason).toBe("音频设备被其他程序占用。");
    expect(v.title).toBe("鼓组混音 v2"); // 保留标题
    expect(reduceLoading(busyView(), { type: "loadingFailed", reason: " " }).phase).toBe("loading");
  });

  it("loadingInterrupted 写明中断原因；与 failed 分态", () => {
    const v = reduceLoading(busyView(), { type: "loadingInterrupted", reason: "宿主窗口被关闭，装载中断。" });
    expect(v.phase).toBe("interrupted");
    expect(v.failureReason).toBe("宿主窗口被关闭，装载中断。");
  });

  it("终态才可重试：retry 清空步骤回 loading（标题保留）；loading/completed 不响应", () => {
    const fail = reduceLoading(busyView(), { type: "loadingFailed", reason: "x" });
    const retry = reduceLoading(fail, { type: "loadingRetry" });
    expect(retry.phase).toBe("loading");
    expect(retry.title).toBe("鼓组混音 v2");
    expect(retry.steps).toHaveLength(0);
    expect(retry.failureReason).toBeUndefined();
    expect(reduceLoading(busyView(), { type: "loadingRetry" }).phase).toBe("loading"); // loading 态 no-op
    const done = reduceLoading(busyView(), { type: "loadingSucceeded" });
    expect(reduceLoading(done, { type: "loadingRetry" }).phase).toBe("completed"); // 完成 no-op
    const interrupted = reduceLoading(busyView(), { type: "loadingInterrupted", reason: "x" });
    expect(reduceLoading(interrupted, { type: "loadingRetry" }).phase).toBe("loading");
  });

  it("终态后不再接受推进事件（failed 拒绝 step/progress/succeeded）", () => {
    const fail = reduceLoading(busyView(), { type: "loadingFailed", reason: "x" });
    expect(reduceLoading(fail, { type: "loadingProgress", value: 0.5 }).progress).toBeNull();
    expect(reduceLoading(fail, { type: "loadingStepCurrent", label: "y" }).steps).toHaveLength(1);
    expect(reduceLoading(fail, { type: "loadingSucceeded" }).phase).toBe("failed");
  });
});

describe("磁带带量映射（tapeReelAmounts）", () => {
  it("待机/未知进度 = 停待机帧 L=1 R=0.2（不伪造推进）", () => {
    expect(tapeReelAmounts("loading", null)).toEqual({ L: 1, R: 0.2 });
    expect(tapeReelAmounts("idle", null)).toEqual({ L: 1, R: 0.2 });
    expect(tapeReelAmounts("failed", null)).toEqual({ L: 1, R: 0.2 });
  });

  it("有进度 p → L=1-0.8p R=0.2+0.8p（SPEC §7.4 数值）；完成帧 L=0.2 R=1", () => {
    expect(tapeReelAmounts("loading", 0.64).L).toBeCloseTo(0.488);
    expect(tapeReelAmounts("loading", 0.64).R).toBeCloseTo(0.712);
    expect(tapeReelAmounts("loading", 0)).toEqual({ L: 1, R: 0.2 });
    expect(tapeReelAmounts("loading", 1).L).toBeCloseTo(0.2);
    expect(tapeReelAmounts("loading", 1).R).toBeCloseTo(1);
    expect(tapeReelAmounts("completed", null)).toEqual({ L: 0.2, R: 1 });
  });

  it("failed 冻结当前帧：有进度时保留进度帧，不回卷", () => {
    const frozen = tapeReelAmounts("failed", 0.5);
    expect(frozen.L).toBeCloseTo(0.6);
    expect(frozen.R).toBeCloseTo(0.6);
  });
});
