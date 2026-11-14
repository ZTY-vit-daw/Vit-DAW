/*
 * UI-LOADING-1（347）P2 装载 / 执行生命周期 · 数据模型与语义状态机。
 *
 * 语义红线（卡面 + SPEC-V5 §4.2.1/§7.4/§8.2 + IMPLEMENTATION-HANDOFF 语义 7）：
 * 1. 步骤最多 3 步（同屏可见），当前步骤小面积黄标记；变化中的主表达是执行
 *    轨迹，黄色只落当前步 chip，不做整卡黄底。
 * 2. 进度只用一种表达：磁带带量（tapeReelAmounts）。未知进度不伪造——progress
 *    只来自显式 loadingProgress 事件；reducer 不自增、不倒计时、无时间驱动。
 * 3. 完成收一行回执（轨迹可展开）；错误/中断写明原因与恢复入口。
 * 4. 输入禁用/恢复：loading 态禁用输入（loadingInputDisabled），completed/failed/
 *    interrupted 恢复（完成后可继续输入）。
 * 5. 步骤只来自显式事件（live 消费既有 ui_state 往返 = 真实单步；三步细分仅
 *    组件能力 + fixture 样张，不在 live 伪造）。
 */

/** 同 StatusCard/TraceStep 形状的装载步骤（≤3 条同屏）。 */
export interface LoadingStep {
  id: string;
  label: string;
  status: "done" | "current";
  /** 已完成步骤的耗时（真实回执携带；无真实耗时就不显示，不编造）。 */
  duration?: string;
}

export type LoadingPhase = "idle" | "loading" | "completed" | "failed" | "interrupted";

/** 装载状态卡视图（reducer 输出 / 组件输入）。 */
export interface LoadingCardView {
  phase: LoadingPhase;
  /** 直接写正在做什么，如「正在打开《鼓组混音 v2》」。 */
  title: string;
  steps: LoadingStep[];
  /** 真实进度 0..1（只来自显式事件；null = 未知进度，不伪造）。 */
  progress: number | null;
  /** failed/interrupted 的原因（写明原因，不美化）。 */
  failureReason?: string;
  /** 完成回执行的结果注（可选；如「3 条轨道已恢复」）。 */
  resultNote?: string;
}

export type LoadingEvent =
  | { type: "loadingStarted"; title: string }
  | { type: "loadingStepDone"; label: string; duration?: string }
  | { type: "loadingStepCurrent"; label: string }
  | { type: "loadingProgress"; value: number }
  | { type: "loadingSucceeded"; note?: string }
  | { type: "loadingFailed"; reason: string }
  | { type: "loadingInterrupted"; reason: string }
  | { type: "loadingRetry" };

export function emptyLoadingView(): LoadingCardView {
  return { phase: "idle", title: "", steps: [], progress: null };
}

/**
 * 装载语义 reducer。全部推进由显式事件驱动（loadingData.test.ts 断言红线）：
 * - progress 只来自 loadingProgress（reducer 不自增）；
 * - 步骤同屏 ≤3（超出保留最近 3 步轨迹）；
 * - failed/interrupted 保留原因，retry 回 loading（title 保留）。
 */
export function reduceLoading(view: LoadingCardView, event: LoadingEvent): LoadingCardView {
  switch (event.type) {
    case "loadingStarted": {
      // 空标题合法（无工程名时渲染层兜底「正在打开工程…」，SPEC §3 通用句式）；
      // 只去首尾空白，不拒绝空串——否则 live probe（尚无工程名）会卡在 idle。
      return { phase: "loading", title: event.title.trim(), steps: [], progress: null };
    }
    case "loadingStepDone": {
      if (view.phase !== "loading") return view;
      if (!event.label.trim()) return view;
      return {
        ...view,
        steps: clampSteps([
          ...view.steps,
          { id: nextLoadingStepID(), label: event.label.trim(), status: "done", duration: event.duration }
        ])
      };
    }
    case "loadingStepCurrent": {
      if (view.phase !== "loading") return view;
      const label = event.label.trim();
      if (!label) return view;
      // 新 current 开工 = 旧 current 步完成（隐式转 done，轨迹保留）。
      const settled = view.steps.map((s) => (s.status === "current" ? { ...s, status: "done" as const } : s));
      return {
        ...view,
        steps: clampSteps([...settled, { id: nextLoadingStepID(), label, status: "current" }])
      };
    }
    case "loadingProgress": {
      if (view.phase !== "loading") return view;
      if (!Number.isFinite(event.value)) return view; // 非有限值不接受（不伪造）
      return { ...view, progress: Math.min(1, Math.max(0, event.value)) };
    }
    case "loadingSucceeded": {
      if (view.phase !== "loading") return view;
      return { ...view, phase: "completed", resultNote: event.note?.trim() || undefined };
    }
    case "loadingFailed": {
      if (view.phase !== "loading") return view;
      const reason = event.reason.trim();
      if (!reason) return view;
      return { ...view, phase: "failed", failureReason: reason };
    }
    case "loadingInterrupted": {
      if (view.phase !== "loading") return view;
      const reason = event.reason.trim();
      if (!reason) return view;
      return { ...view, phase: "interrupted", failureReason: reason };
    }
    case "loadingRetry": {
      if (view.phase !== "failed" && view.phase !== "interrupted") return view;
      // 重试：保留标题与已完成轨迹清空重走（步骤事件重新喂入）。
      return { ...view, phase: "loading", steps: [], progress: null, failureReason: undefined };
    }
    default:
      return view;
  }
}

/** 同屏最多 3 步（SPEC §8.2）；超出时保留最近 3 条。 */
function clampSteps(steps: LoadingStep[]): LoadingStep[] {
  return steps.slice(-3);
}

let loadingStepIDCounter = 0;
function nextLoadingStepID(): string {
  loadingStepIDCounter += 1;
  return `loading-step-${loadingStepIDCounter}`;
}

/** 装载期间输入禁用；终态（完成/失败/中断）恢复（「打开完成后可继续输入」）。 */
export function loadingInputDisabled(view: LoadingCardView): boolean {
  return view.phase === "loading";
}

/**
 * live 探针接缝（消费既有端点，不新增 agent 语义）：
 * fetchUIState 往返 → 装载事件映射。pending = 当前步「正在连接 Vit agent」；
 * 成功 = loadingSucceeded；失败 = loadingFailed（原因原样）。
 */
export const LOADING_LIVE_STEP_LABEL = "正在连接 Vit agent";
