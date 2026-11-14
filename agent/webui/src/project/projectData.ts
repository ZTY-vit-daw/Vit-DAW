/*
 * UI-PROJECT-1（346）P3 工程内对话主窗 · 数据模型与语义状态机。
 *
 * 语义红线（卡面 + IMPLEMENTATION-HANDOFF「必须保留的控件语义」）：
 * 1. 选择 ≠ 应用：宏选项切换只产生「已选择」；「已应用」只能由 applySucceeded
 *    事件（服务端真实成功回执）进入，任何 UI 事件都无法直接把状态写成 applied。
 * 2. 失败显示原因与重试，保留未应用事实（failed 不清除选择，重试回到 default）。
 * 3. 快照/撤销绑定真实数据：撤销必须携带服务端签发的 snapshotRef；
 *    无 ref 的撤销事件被拒绝（无伪造撤销）。
 * 4. 未知进度不伪造：轨迹步骤只来自事件；不倒计时、不推测百分比。
 *
 * 数据来源：live 模式消费真实 agent 端点（ui_state / chat / stop_turn，经
 * ProjectService）；fixture 模式完全离线（gallery / render-smoke），fixture
 * 在接缝处扮演「服务端回执」，UI 状态机本身不做任何伪造。
 * 工程名等示例数据一律带（fixture）后缀并显式标注，生产代码零硬编码工程名。
 */

import type { MacroCardState, MacroOption, TraceStep } from "../components";

/* ------------------------------------------------------------------ */
/* 视图模型                                                            */
/* ------------------------------------------------------------------ */

/** 工程上下文头（P3 标题栏下方一行；来源显式标注，fixture 不冒充 agent 数据）。 */
export interface ProjectContextView {
  name: string;
  source: "agent" | "fixture";
  /** 可选的次要信息（一行元信息最多两项，§3）。 */
  detail?: string;
}

/** 执行轨迹（SPEC-V5 §4.2.1 / §8.2）：busy 有当前步，done/error 收成回执行。 */
export interface TrajectoryView {
  state: "idle" | "busy" | "done" | "error";
  /** 直接写正在做什么，如「正在应用宏「温暖」」。 */
  title: string;
  /** 已完成/当前步骤（StatusCard 最多显示最近 3 步）。 */
  steps: TraceStep[];
  failureReason?: string;
}

/** 宏卡线程视图：344 MacroCard 的四态 + 撤销所需的真实快照绑定。 */
export interface MacroCardView {
  object: string;
  name: string;
  options: MacroOption[];
  selectedId: string | null;
  state: MacroCardState;
  failureReason?: string;
  /** 服务端确认的已应用选项（applied 态必填）。 */
  appliedOptionId?: string;
  /** 服务端签发的快照引用（applied 态必填；撤销绑定此 ref）。 */
  appliedSnapshotRef?: string;
}

/** 快照回执（§8.5 一行）：撤销按钮消费真实 ref。 */
export interface SnapshotReceiptView {
  time: string;
  /** 服务端签发的快照引用（撤销绑定）。 */
  snapshotRef: string;
  /** 关联的宏卡线程 id（撤销后回写宏卡状态）。 */
  macroItemId?: string;
  undone?: boolean;
}

/** A/B 动作（开始方式待用户裁定，本卡只到 ready 为止；不伪造播放）。 */
export interface ABActionView {
  state: "idle" | "preparing" | "ready" | "failed";
  title: string;
  /** 服务端回执的真实渲染引用（ready 态在卷）。 */
  bypassRef?: string;
  processedRef?: string;
  failureReason?: string;
}

/** 会话线程条目（全部为数据呈现；宏/A/B 的执行结果只能来自服务事件）。 */
export type ProjectThreadItem =
  | { kind: "message"; id: string; author: "user" | "vit"; text: string }
  | { kind: "trajectory"; id: string; view: TrajectoryView }
  | { kind: "macro"; id: string; view: MacroCardView }
  | { kind: "snapshot"; id: string; view: SnapshotReceiptView }
  | { kind: "ab"; id: string; view: ABActionView };

/** 输入区状态（§8.6 四态）。 */
export type ProjectComposerState = "default" | "focus" | "disabled" | "replying";

/* ------------------------------------------------------------------ */
/* 语义事件（reducer 输入）                                             */
/* ------------------------------------------------------------------ */

export type ProjectEvent =
  | { type: "userSent"; id: string; text: string }
  | { type: "vitReplied"; id: string; text: string }
  | { type: "replyStarted" }
  | { type: "replySettled" }
  | { type: "inputDisabledSet"; disabled: boolean }
  | { type: "contextLoaded"; view: ProjectContextView }
  | { type: "turnStarted"; id: string; title: string }
  | { type: "turnStepDone"; id: string; label: string; duration?: string }
  | { type: "turnSucceeded"; id: string }
  | { type: "turnFailed"; id: string; reason: string }
  | { type: "macroSelect"; id: string; optionId: string }
  | { type: "macroAuditionStart"; id: string; optionId: string }
  | { type: "macroAuditionCancel"; id: string }
  | { type: "macroApplyStarted"; id: string }
  | { type: "macroApplySucceeded"; id: string; optionId: string; snapshotRef: string; time: string }
  | { type: "macroApplyFailed"; id: string; reason: string }
  | { type: "macroRetry"; id: string }
  | { type: "macroUndo"; id: string; snapshotRef: string }
  | { type: "abPrepareStarted"; id: string }
  | { type: "abPrepareSucceeded"; id: string; bypassRef: string; processedRef: string }
  | { type: "abPrepareFailed"; id: string; reason: string }
  | { type: "composerFocus"; focused: boolean };

export interface ProjectSurfaceState {
  context: ProjectContextView | null;
  thread: ProjectThreadItem[];
  /** 输入框禁用（装载中 / fixture 指定）。 */
  inputDisabled: boolean;
  /** Vit 回复中（发送键变停止键）。 */
  replying: boolean;
  composerFocused: boolean;
  /** 应用于途中的宏卡 id（防重复应用；不改变卡片四态）。 */
  pendingMacroId: string | null;
}

export function emptyProjectState(): ProjectSurfaceState {
  return {
    context: null,
    thread: [],
    inputDisabled: false,
    replying: false,
    composerFocused: false,
    pendingMacroId: null
  };
}

/** 输入区四态（优先级：disabled > replying > focus > default）。 */
export function composerStateOf(state: ProjectSurfaceState): ProjectComposerState {
  if (state.inputDisabled) return "disabled";
  if (state.replying) return "replying";
  if (state.composerFocused) return "focus";
  return "default";
}

function findMacro(items: ProjectThreadItem[], id: string): MacroCardView | null {
  for (const item of items) {
    if (item.kind === "macro" && item.id === id) return item.view;
  }
  return null;
}

function findAB(items: ProjectThreadItem[], id: string): ABActionView | null {
  for (const item of items) {
    if (item.kind === "ab" && item.id === id) return item.view;
  }
  return null;
}

function withMacro(items: ProjectThreadItem[], id: string, view: MacroCardView): ProjectThreadItem[] {
  return items.map((item) => (item.kind === "macro" && item.id === id ? { ...item, view } : item));
}

function withAB(items: ProjectThreadItem[], id: string, view: ABActionView): ProjectThreadItem[] {
  return items.map((item) => (item.kind === "ab" && item.id === id ? { ...item, view } : item));
}

/**
 * 语义 reducer。全部状态推进由显式事件驱动；红线断言（见 projectData.test.ts）：
 * - applied 只能经 macroApplySucceeded；
 * - 撤销必须带与卡片一致的 snapshotRef；
 * - failed 保留选择，retry 回 default。
 */
export function reduceProject(state: ProjectSurfaceState, event: ProjectEvent): ProjectSurfaceState {
  switch (event.type) {
    case "userSent": {
      const text = event.text.trim();
      if (!text) return state;
      return {
        ...state,
        thread: [...state.thread, { kind: "message", id: event.id, author: "user", text }]
      };
    }
    case "vitReplied": {
      const text = event.text.trim();
      if (!text) return state;
      return {
        ...state,
        thread: [...state.thread, { kind: "message", id: event.id, author: "vit", text }]
      };
    }
    case "replyStarted":
      return state.replying ? state : { ...state, replying: true };
    case "replySettled":
      return state.replying ? { ...state, replying: false } : state;
    case "inputDisabledSet":
      return { ...state, inputDisabled: event.disabled };
    case "contextLoaded":
      return { ...state, context: event.view };
    case "turnStarted": {
      const item: ProjectThreadItem = {
        kind: "trajectory",
        id: event.id,
        view: { state: "busy", title: event.title, steps: [] }
      };
      return { ...state, thread: [...state.thread, item] };
    }
    case "turnStepDone": {
      return {
        ...state,
        thread: state.thread.map((item) => {
          if (item.kind !== "trajectory" || item.id !== event.id) return item;
          if (item.view.state !== "busy") return item;
          return {
            ...item,
            view: {
              ...item.view,
              steps: [...item.view.steps, { id: `${event.id}-${item.view.steps.length + 1}`, label: event.label, status: "done", duration: event.duration }]
            }
          };
        })
      };
    }
    case "turnSucceeded": {
      return {
        ...state,
        thread: state.thread.map((item) =>
          item.kind === "trajectory" && item.id === event.id && item.view.state === "busy"
            ? { ...item, view: { ...item.view, state: "done" } }
            : item
        )
      };
    }
    case "turnFailed": {
      return {
        ...state,
        thread: state.thread.map((item) =>
          item.kind === "trajectory" && item.id === event.id && item.view.state === "busy"
            ? { ...item, view: { ...item.view, state: "error", failureReason: event.reason } }
            : item
        )
      };
    }
    case "macroSelect": {
      const macro = findMacro(state.thread, event.id);
      if (!macro) return state;
      if (!macro.options.some((o) => o.id === event.optionId)) return state;
      // 选择 ≠ 应用：任何状态下切换选择都不进入/退出 applied。
      const next: MacroCardView = { ...macro, selectedId: event.optionId };
      return { ...state, thread: withMacro(state.thread, event.id, next) };
    }
    case "macroAuditionStart": {
      const macro = findMacro(state.thread, event.id);
      if (!macro || macro.state !== "default") return state; // applied/failed 不试听
      if (!macro.options.some((o) => o.id === event.optionId)) return state;
      return {
        ...state,
        thread: withMacro(state.thread, event.id, { ...macro, state: "auditioning", selectedId: event.optionId })
      };
    }
    case "macroAuditionCancel": {
      const macro = findMacro(state.thread, event.id);
      if (!macro || macro.state !== "auditioning") return state;
      // 试听取消回到 default；不产生任何「已应用」事实。
      return { ...state, thread: withMacro(state.thread, event.id, { ...macro, state: "default" }) };
    }
    case "macroApplyStarted": {
      // 与 requestMacroApply 同一守卫（default + 已选择 + 无在途）；通过才记在途。
      if (state.pendingMacroId) return state;
      const macro = findMacro(state.thread, event.id);
      if (!macro || macro.state !== "default" || !macro.selectedId) return state;
      return { ...state, pendingMacroId: event.id };
    }
    case "macroApplySucceeded": {
      if (state.pendingMacroId !== event.id) return state; // 只接受在途应用的成功
      const macro = findMacro(state.thread, event.id);
      if (!macro || !macro.selectedId || macro.selectedId !== event.optionId) return state;
      if (!event.snapshotRef.trim()) return state; // 无服务端 ref 不入 applied
      const next: MacroCardView = {
        ...macro,
        state: "applied",
        appliedOptionId: event.optionId,
        appliedSnapshotRef: event.snapshotRef,
        failureReason: undefined
      };
      const snapshotItem: ProjectThreadItem = {
        kind: "snapshot",
        id: `snapshot-${event.snapshotRef}`,
        view: { time: event.time, snapshotRef: event.snapshotRef, macroItemId: event.id }
      };
      return {
        ...state,
        pendingMacroId: null,
        thread: [...withMacro(state.thread, event.id, next), snapshotItem]
      };
    }
    case "macroApplyFailed": {
      if (state.pendingMacroId !== event.id) return state;
      const macro = findMacro(state.thread, event.id);
      if (!macro) return state;
      // 保留未应用事实：state=failed + 原因；选择不清除。
      return {
        ...state,
        pendingMacroId: null,
        thread: withMacro(state.thread, event.id, { ...macro, state: "failed", failureReason: event.reason })
      };
    }
    case "macroRetry": {
      const macro = findMacro(state.thread, event.id);
      if (!macro || macro.state !== "failed") return state;
      // 重试回到 default（选择保留），由调用方重新发起真实应用请求。
      return { ...state, thread: withMacro(state.thread, event.id, { ...macro, state: "default", failureReason: undefined }) };
    }
    case "macroUndo": {
      const macro = findMacro(state.thread, event.id);
      if (!macro || macro.state !== "applied") return state;
      // 撤销绑定真实数据：ref 必须与卡片持有的服务端 ref 一致。
      if (!macro.appliedSnapshotRef || macro.appliedSnapshotRef !== event.snapshotRef) return state;
      const next: MacroCardView = {
        ...macro,
        state: "default",
        appliedOptionId: undefined,
        appliedSnapshotRef: undefined,
        failureReason: undefined
      };
      // 回执行同步标记已撤销（真实链路：回执与卡片消费同一 ref）。
      const thread = state.thread.map((item) => {
        if (item.kind === "snapshot" && item.view.snapshotRef === event.snapshotRef) {
          return { ...item, view: { ...item.view, undone: true } };
        }
        if (item.kind === "trajectory" && item.view.state === "done" && item.view.title.includes("撤销")) {
          return item;
        }
        return item;
      });
      return { ...state, thread: withMacro(thread, event.id, next) };
    }
    case "abPrepareStarted": {
      const ab = findAB(state.thread, event.id);
      // idle（首次）/ failed（重试）才可进入 preparing；ready 不重跑。
      if (!ab || (ab.state !== "idle" && ab.state !== "failed")) return state;
      return { ...state, thread: withAB(state.thread, event.id, { ...ab, state: "preparing", failureReason: undefined }) };
    }
    case "abPrepareSucceeded": {
      const ab = findAB(state.thread, event.id);
      if (!ab || ab.state !== "preparing") return state;
      return {
        ...state,
        thread: withAB(state.thread, event.id, {
          ...ab,
          state: "ready",
          bypassRef: event.bypassRef,
          processedRef: event.processedRef,
          failureReason: undefined
        })
      };
    }
    case "abPrepareFailed": {
      const ab = findAB(state.thread, event.id);
      if (!ab || ab.state !== "preparing") return state;
      return { ...state, thread: withAB(state.thread, event.id, { ...ab, state: "failed", failureReason: event.reason }) };
    }
    case "composerFocus":
      return { ...state, composerFocused: event.focused };
    default:
      return state;
  }
}

/* ------------------------------------------------------------------ */
/* 应用请求发起（surface 侧守卫）                                        */
/* ------------------------------------------------------------------ */

/**
 * 发起应用：仅在 default 态 + 已选择 + 无在途应用时有效。
 * 返回 null 表示本次点击不构成请求（UI 层防重复/防未选择应用）。
 */
export function requestMacroApply(
  state: ProjectSurfaceState,
  macroId: string
): { macroId: string; optionId: string } | null {
  if (state.pendingMacroId) return null;
  const macro = findMacro(state.thread, macroId);
  if (!macro || macro.state !== "default" || !macro.selectedId) return null;
  return { macroId, optionId: macro.selectedId };
}

/* ------------------------------------------------------------------ */
/* 服务接缝（live = 真实端点；fixture = 离线服务端替身）                  */
/* ------------------------------------------------------------------ */

/** 应用结果（服务端语义）：成功必须带 snapshotRef。 */
export type MacroApplyOutcome =
  | { ok: true; snapshotRef: string; time: string }
  | { ok: false; reason: string };

export interface ProjectService {
  /** 工程上下文（live: GET /agent/ui_state → project）。 */
  fetchContext(): Promise<ProjectContextView>;
  /** 发送一句对话（live: POST /agent/chat，本 surface 会话）。 */
  sendMessage(text: string): Promise<string>;
  /** 停止当前回复（live: POST /agent/stop_turn）。 */
  stopReply(): Promise<void>;
  /** 应用宏（fixture：接缝回执；live 无快照协议 → 必须由 fixture/后续卡接入，不伪造）。 */
  applyMacro(macroId: string, optionId: string): Promise<MacroApplyOutcome>;
  /** 撤销快照（绑定真实 ref；服务端拒绝时抛错）。 */
  undoSnapshot(snapshotRef: string): Promise<void>;
  /** 准备对比 A/B 渲染。 */
  prepareAB(abId: string): Promise<{ bypassRef: string; processedRef: string }>;
}

let idCounter = 0;
export function nextProjectItemID(prefix: string): string {
  idCounter += 1;
  return `${prefix}-${Date.now().toString(36)}-${idCounter}`;
}
