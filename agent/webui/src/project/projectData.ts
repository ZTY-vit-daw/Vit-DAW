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
// SHELL-6（池序 367）：工程面交互卡数据面。launcher 面已有的两个纯数据件复用
// （import-only，零修改 launcher/）——directive 占位视图：两面对同一 agent/宿主
// 契约（launcher_directive），视图构造不该各写一套。
import type { LauncherDirective, LauncherProjectCard } from "../api/launcher";
import { directivePlaceholder, type DirectivePlaceholderView } from "../launcher/launcherData";

/* ------------------------------------------------------------------ */
/* 视图模型                                                            */
/* ------------------------------------------------------------------ */

/** 工程上下文头（P3 标题栏下方一行；来源显式标注，fixture 不冒充 agent 数据）。 */
export interface ProjectContextView {
  name: string;
  source: "agent" | "fixture";
  /** 可选的次要信息（一行元信息最多两项，§3）。 */
  detail?: string;
  /** TUTOR-SIM-2（436）：工程文件绝对路径（live 来自 ui_state.project.project_path）。
   * 自动开场的「每工程一次」触发器；fixture/旧调用不传 = 不触发。 */
  path?: string;
}

/* ------------------------------------------------------------------ */
/* TUTOR-SIM-3（437）：导师进度/步骤输入卡视图                            */
/* ------------------------------------------------------------------ */

/** 导师实验进度（唯一数据源 = 服务端 workflow_data 的结构化字段；
 * 禁止从 LLM 回复文本或旧 eduJamRenderStatus 文本反推）。 */
export interface TutorProgressView {
  stepIndex: number;
  stepTotal: number;
  completed: boolean;
}

/** 步骤输入卡视图：步骤寻址 + 最近 assistant 回复作为当前问题/引导。
 * prompt 只来自服务端 reply；服务端判定答案永不下发、永不显示。
 * TUTOR-SIM-4（438）：brief 教学简报与材料列表（v1 实验字段；可选，v0 实验
 * 两字段缺省空 → UI 不渲染简报/材料卡）。 */
export interface TutorStepView extends TutorProgressView {
  prompt: string;
  brief?: string;
  materials?: TutorMaterialView[];
}

/** 材料视图（服务端 workflow_data.materials 的只读投影；kind 只认
 * midi_clip / audio，其余 fail-closed 丢弃）。 */
export interface TutorMaterialView {
  kind: "midi_clip" | "audio";
  title: string;
  path: string;
  noteCount?: number;
  durationBeats?: number;
  tempoBPM?: number;
  loaded: boolean;
  loading: boolean;
  summaryError?: string;
  instrumentAttached?: boolean;
  instrumentError?: string;
  loadError?: string;
}

/** workflow_data.materials → 材料视图（fail-closed：非数组返回 null 由调用方
 * 保留旧值；单个非法条目丢弃，不因个别脏数据丢整批）。 */
export function tutorMaterialsFromValue(value: unknown): TutorMaterialView[] | null {
  if (!Array.isArray(value)) return null;
  const out: TutorMaterialView[] = [];
  for (const item of value) {
    if (!item || typeof item !== "object" || Array.isArray(item)) continue;
    const row = item as Record<string, unknown>;
    const kind = row.kind;
    const title = typeof row.title === "string" ? row.title.trim() : "";
    if ((kind !== "midi_clip" && kind !== "audio") || !title) continue;
    const finiteNumber = (input: unknown): number | undefined =>
      typeof input === "number" && Number.isFinite(input) ? input : undefined;
    const text = (input: unknown): string | undefined =>
      typeof input === "string" && input.trim() !== "" ? input : undefined;
    out.push({
      kind,
      title,
      path: typeof row.path === "string" ? row.path : "",
      noteCount: finiteNumber(row.note_count),
      durationBeats: finiteNumber(row.duration_beats),
      tempoBPM: finiteNumber(row.tempo_bpm),
      loaded: row.loaded === true,
      loading: row.loading === true,
      summaryError: text(row.summary_error),
      instrumentAttached: typeof row.instrument_attached === "boolean" ? row.instrument_attached : undefined,
      instrumentError: text(row.instrument_error),
      loadError: text(row.load_error)
    });
  }
  return out;
}

/** 严格校验 workflow_data 的 step_index/step_total/completed；任何缺失或越界
 * 都不更新进度（保留上一次真实服务端状态，不用 LLM 文本补齐）。 */
export function tutorProgressFromSteps(
  stepIndex: unknown,
  stepTotal: unknown,
  completed: unknown
): TutorProgressView | null {
  if (typeof stepIndex !== "number" || !Number.isInteger(stepIndex)) return null;
  if (typeof stepTotal !== "number" || !Number.isInteger(stepTotal)) return null;
  if (stepTotal < 1 || stepIndex < 1 || stepIndex > stepTotal) return null;
  if (typeof completed !== "boolean") return null;
  return { stepIndex, stepTotal, completed };
}

/** workflow_data → 进度视图（只读结构化字段；不接受字符串数字/缺字段）。 */
export function tutorProgressFromWorkflowData(data: unknown): TutorProgressView | null {
  if (!data || typeof data !== "object" || Array.isArray(data)) return null;
  const record = data as Record<string, unknown>;
  return tutorProgressFromSteps(record.step_index, record.step_total, record.completed);
}

/** TUTOR-SIM-4：workflow_data.brief → 简报文本（缺省/非字符串 = 空串）。 */
export function tutorBriefFromWorkflowData(data: unknown): string {
  if (!data || typeof data !== "object" || Array.isArray(data)) return "";
  const brief = (data as Record<string, unknown>).brief;
  return typeof brief === "string" ? brief : "";
}

/** TUTOR-SIM-4：workflow_data → 材料视图列表（materials 键缺失/非数组 =
 * null，由调用方保留旧值）。 */
export function tutorMaterialsFromWorkflowData(data: unknown): TutorMaterialView[] | null {
  if (!data || typeof data !== "object" || Array.isArray(data)) return null;
  const record = data as Record<string, unknown>;
  return tutorMaterialsFromValue(record.materials);
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

/** 会话线程条目（全部为数据呈现；宏/A/B 的执行结果只能来自服务事件）。
 * SHELL-6 新增四类交互卡：directive（自动执行占位）/ cards（点卡切换）/
 * governance（宿主三键治理卡）/ confirm（确认语义承接卡）。 */
export type ProjectThreadItem =
  | { kind: "message"; id: string; author: "user" | "vit"; text: string }
  | { kind: "trajectory"; id: string; view: TrajectoryView }
  | { kind: "macro"; id: string; view: MacroCardView }
  | { kind: "snapshot"; id: string; view: SnapshotReceiptView }
  | { kind: "ab"; id: string; view: ABActionView }
  | { kind: "directive"; id: string; view: DirectivePlaceholderView; directive?: LauncherDirective }
  | { kind: "cards"; id: string; cards: LauncherProjectCard[] }
  | { kind: "governance"; id: string; view: ProjectGovernanceView; answered?: string }
  | { kind: "confirm"; id: string; view: ProjectConfirmView; answered?: "approve" | "cancel" };

/** SHELL-6：宿主页内治理三键卡视图（vit_governance_required 载荷镜像）。 */
export interface ProjectGovernanceView {
  requestID: string;
  action: string;
  /** 在开工程显示名（宿主回执原样；态未知=空）。 */
  currentProject: string;
}

/** SHELL-6：确认语义承接卡视图（协议位或检测命中共用）。 */
export interface ProjectConfirmView {
  /** 引导语态说明（点按钮或直接输入都行——按钮=用户本可亲口说的话）。 */
  note: string;
  approveText: string;
  cancelText: string;
}

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
  | { type: "outcomeItemsArrived"; items: ProjectThreadItem[] }
  | { type: "tutorViewUpdated"; projectPath: string; view: TutorStepView }
  | {
      type: "tutorMaterialLoaded";
      projectPath: string;
      title: string;
      trackID: string;
      clipID: string;
      instrumentAttached: boolean;
    }
  | { type: "governanceRequested"; id: string; view: ProjectGovernanceView }
  | { type: "governanceAnswered"; id: string; choice: string }
  | { type: "confirmAnswered"; id: string; choice: "approve" | "cancel" }
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
  /** TUTOR-SIM-3：按工程路径索引的导师控件视图。普通工程无条目 → 零 UI；
   * 切换工程只显示当前路径的条目，旧工程状态不串台。 */
  tutorViews: Record<string, TutorStepView>;
}

export function emptyProjectState(): ProjectSurfaceState {
  return {
    context: null,
    thread: [],
    inputDisabled: false,
    replying: false,
    composerFocused: false,
    pendingMacroId: null,
    tutorViews: {}
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
    case "outcomeItemsArrived": {
      // 追加一批交互卡（消息/卡/directive/确认卡）；空批零变化。
      if (event.items.length === 0) return state;
      return { ...state, thread: [...state.thread, ...event.items] };
    }
    case "tutorViewUpdated": {
      // TUTOR-SIM-3：每工程保留一份最新结构化视图；空路径/非法步骤不落状态。
      // TUTOR-SIM-4：brief/materials 随事件更新（缺省回退旧值）；两者皆空时
      // 不落键——v0 实验的存储视图保持 437 原形状（严格相等面不回归）。
      const path = event.projectPath.trim();
      if (!path) return state;
      const progress = tutorProgressFromSteps(event.view.stepIndex, event.view.stepTotal, event.view.completed);
      if (!progress) return state;
      const previous = state.tutorViews[path];
      const prompt = event.view.prompt.trim() || previous?.prompt || "";
      const brief = (event.view.brief ?? "").trim() || previous?.brief || "";
      // materials 已是投影后的视图数组（raw workflow_data 的校验投影单点在
      // tutorMaterialsFromWorkflowData / tutorStartOutcomeFromResponse）。
      const materials = event.view.materials ?? previous?.materials ?? [];
      const view: TutorStepView = { ...progress, prompt };
      if (brief) view.brief = brief;
      if (materials.length > 0) view.materials = materials;
      return {
        ...state,
        tutorViews: { ...state.tutorViews, [path]: view }
      };
    }
    case "tutorMaterialLoaded": {
      // TUTOR-SIM-4：装载成功回执只把当前工程同名材料置为已装载（真实服务端
      // track/clip 证据由回执带出；无会话/无材料时零变化）。
      const path = event.projectPath.trim();
      if (!path) return state;
      const current = state.tutorViews[path];
      if (!current?.materials?.length) return state;
      return {
        ...state,
        tutorViews: {
          ...state.tutorViews,
          [path]: {
            ...current,
            materials: current.materials.map((material) =>
              material.title === event.title && !material.loaded
                ? { ...material, loaded: true, loading: false, instrumentAttached: event.instrumentAttached }
                : material
            )
          }
        }
      };
    }
    case "governanceRequested": {
      // 幂等：同一 request_id 只挂一张卡（宿主重发不叠卡）。
      const exists = state.thread.some(
        (item) => item.kind === "governance" && item.view.requestID === event.view.requestID
      );
      if (exists) return state;
      return { ...state, thread: [...state.thread, { kind: "governance", id: event.id, view: event.view }] };
    }
    case "governanceAnswered": {
      return {
        ...state,
        thread: state.thread.map((item) =>
          item.kind === "governance" && item.id === event.id && !item.answered
            ? { ...item, answered: event.choice }
            : item
        )
      };
    }
    case "confirmAnswered": {
      return {
        ...state,
        thread: state.thread.map((item) =>
          item.kind === "confirm" && item.id === event.id && !item.answered
            ? { ...item, answered: event.choice }
            : item
        )
      };
    }
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

/** SHELL-6：一句对话的完整回执载荷（消息文本 + 交互面）。 */
export interface ProjectChatOutcome {
  /** 呈现文本（回执 reply；空由调用方给占位）。 */
  text: string;
  /** launcher_directive（agent 确定性意图路由回执；到达即自动经桥执行）。 */
  directive?: LauncherDirective | null;
  /** project_result_cards（点卡=执行同指令切换）。 */
  cards?: LauncherProjectCard[];
  /** chat 确认协议位（needs_confirmation）：必须伴随可点确认卡（不变式）。 */
  needsConfirmation?: boolean;
  /** TUTOR-SIM-3：服务端 workflow 标记（"tutor_sim" 命中时更新步骤控件）。 */
  workflow?: string | null;
  /** TUTOR-SIM-3：导师进度结构化载荷（只读投影；不渲染原始 JSON/状态文本）。 */
  workflowData?: Record<string, unknown> | null;
}

/** TUTOR-SIM-2（436）：自动开场接缝回执。
 * unavailable = 工程无 sidecar / LLM 配置缺失 / 请求失败——UI 必须静默。
 * TUTOR-SIM-4（438）：ok 变体携带当前步 brief/materials（v0 实验缺省空）。 */
export type TutorStartOutcome =
  | {
      status: "ok";
      conversationID: string;
      reply: string;
      stepIndex: number;
      stepTotal: number;
      stepID: string;
      brief?: string;
      materials?: TutorMaterialView[];
    }
  | { status: "unavailable"; reason?: string };

/** TUTOR-SIM-4（438）：材料装载接缝回执。track/clip 就位即 ok（音源挂载是
 * best-effort：instrumentAttached=false + instrumentError 如实带回，不算失败）。 */
export type TutorMaterialLoadOutcome =
  | {
      ok: true;
      alreadyLoaded: boolean;
      trackID: string;
      clipID: string;
      pluginID: string;
      instrumentAttached: boolean;
      instrumentError?: string;
    }
  | { ok: false; reason: string };

export interface ProjectService {
  /** 工程上下文（live: GET /agent/ui_state → project）。 */
  fetchContext(): Promise<ProjectContextView>;
  /** 发送一句对话（live: POST /agent/chat，本 surface 会话）——SHELL-6 起返回完整回执载荷。
   * TUTOR-SIM-2：传 conversationID 时用它寻址（导师会话）；省略 = 本 surface 普通会话。 */
  sendMessage(text: string, conversationID?: string): Promise<ProjectChatOutcome>;
  /** TUTOR-SIM-2：自动开场（live: POST /agent/tutor/start）。不可用/失败必须返回
   * unavailable，不抛错、不发假消息；fixture 默认实现不触网。 */
  startTutorSim(projectPath: string): Promise<TutorStartOutcome>;
  /** TUTOR-SIM-4：装载当前步材料（live: POST /agent/tutor/load_material）。
   * 失败返回 ok:false + 原因，不抛错、不伪造已装载。 */
  loadTutorMaterial(conversationID: string, title: string): Promise<TutorMaterialLoadOutcome>;
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

/* ------------------------------------------------------------------ */
/* SHELL-6：回执载荷 → 线程交互卡（纯函数；不变式断言面）                  */
/* ------------------------------------------------------------------ */

/** 确认/取消按钮发送的话术=用户本可亲口说的既有确认协议话术（不硬造协议）。 */
export const PROJECT_CONFIRM_APPROVE_TEXT = "确认执行";
export const PROJECT_CONFIRM_CANCEL_TEXT = "取消";

/**
 * 纯文字确认语义检测（紧口径——仅 LLM 兜底用：协议位 needs_confirmation 之外，
 * 意图路由未命中而 LLM 产出确认语态的文字）。误检代价=多一张可点卡，不伪造动作。
 */
const CONFIRMATION_TEXT_PATTERNS = [
  "需要确认",
  "要确认",
  "请确认",
  "确认一下",
  "是否确认",
  "要不要",
  "等你确认",
  "确认后执行"
];

export function replyAsksConfirmation(text: string): boolean {
  const trimmed = text.trim();
  if (!trimmed) return false;
  return CONFIRMATION_TEXT_PATTERNS.some((pattern) => trimmed.includes(pattern));
}

/** 确认承接卡视图（protocol=true：chat 确认协议位；false：LLM 纯文字兜底）。 */
export function buildProjectConfirmView(protocol: boolean): ProjectConfirmView {
  return {
    note: protocol
      ? "Vit 正在等一句确认——点下面的按钮，或直接输入确认话术，效果相同。"
      : "这条回复带着确认的意思但没有给出可点的动作——点下面的按钮（等同你亲口说），或换个更直接的说法（例如「新建工程」）。",
    approveText: PROJECT_CONFIRM_APPROVE_TEXT,
    cancelText: PROJECT_CONFIRM_CANCEL_TEXT
  };
}

/** projectThreadItemsFromOutcome 的产出：线程卡批 + 自动执行指令。 */
export interface OutcomeProjectItems {
  items: ProjectThreadItem[];
  /** directive 到达即自动经桥执行（SHELL-6 语义平移；执行态/终态只认宿主回执）。 */
  autoDirective?: LauncherDirective;
}

/**
 * 回执载荷 → 线程卡批（纯函数，render 与单测共用一条映射）。
 * SHELL-6 不变式（卡面断言面）：任何「需要确认」语义回复必须伴随可点交互面——
 *  - needs_confirmation=true → confirm 卡（按钮走既有确认话术协议）；
 *  - 纯文字确认语义（无 directive/卡兜底）→ 同一张 confirm 卡（引导语态）；
 *  - directive 回执 → directive 占位卡 + autoDirective（UI 自动经桥执行，
 *    治理确认由宿主按需出三键卡）。
 */
export function projectThreadItemsFromOutcome(
  outcome: ProjectChatOutcome,
  nextID: (prefix: string) => string = nextProjectItemID
): OutcomeProjectItems {
  const items: ProjectThreadItem[] = [];
  const text = outcome.text.trim();
  if (text) {
    items.push({ kind: "message", id: nextID("msg-vit"), author: "vit", text });
  }
  const cards = (outcome.cards ?? []).filter((card) => card && (card.name || card.filePath));
  if (cards.length > 0) {
    items.push({ kind: "cards", id: nextID("cards"), cards });
  }
  let autoDirective: LauncherDirective | undefined;
  if (outcome.directive && outcome.directive.action) {
    autoDirective = outcome.directive;
    const view = directivePlaceholder(outcome.directive);
    if (view) {
      items.push({ kind: "directive", id: nextID("directive"), view, directive: outcome.directive });
    }
  }
  const confirmationSemantic =
    Boolean(outcome.needsConfirmation) ||
    (!autoDirective && cards.length === 0 && replyAsksConfirmation(text));
  if (confirmationSemantic) {
    items.push({
      kind: "confirm",
      id: nextID("confirm"),
      view: buildProjectConfirmView(Boolean(outcome.needsConfirmation))
    });
  }
  return { items, autoDirective };
}
