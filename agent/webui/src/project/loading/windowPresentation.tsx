import { createContext, useCallback, useContext, useEffect, useReducer, useRef, useState, type ReactNode } from "react";
import { LoadingCard } from "./LoadingCard";
import { emptyLoadingView, reduceLoading, type LoadingCardView } from "./loadingData";
import "./windowPresentation.css";

const PHASES = ["waiting_governance", "saving", "closing_daw", "spawning", "waiting_launch_result", "opened", "failed", "canceled"] as const;
type LaunchPhase = typeof PHASES[number];
export interface LaunchProgress {
  requestId: string; seq: number; phase: LaunchPhase; detail: string;
  stage?: "workspace_ready" | "scene_accepted";
}
export interface NativeWindowObservation {
  foreground: "shell" | "daw" | "other" | "none";
  dawHwnd: string | null; observedAt: number;
}
export interface WindowPresentationState {
  requestId: string; lastSeq: number; view: LoadingCardView;
  terminal: "" | "ready" | "accepted" | "unconfirmed" | "failed" | "canceled";
  window: NativeWindowObservation | null;
}
export const emptyWindowPresentation = (): WindowPresentationState => ({ requestId: "", lastSeq: 0, view: emptyLoadingView(), terminal: "", window: null });
const recordOf = (value: unknown): Record<string, unknown> | null => value !== null && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : null;
const text = (value: unknown) => typeof value === "string" ? value : "";

export function parseLaunchProgress(value: unknown): LaunchProgress | null {
  const r = recordOf(value);
  if (!r || r.type !== "vit_launch_progress" || r.schema !== "vitshell.launch_progress.v1"
      || !text(r.request_id).trim() || !Number.isSafeInteger(r.seq) || Number(r.seq) <= 0
      || !PHASES.includes(r.phase as LaunchPhase) || typeof r.detail !== "string") return null;
  return { requestId: text(r.request_id), seq: Number(r.seq), phase: r.phase as LaunchPhase, detail: r.detail,
    stage: r.stage === "workspace_ready" || r.stage === "scene_accepted" ? r.stage : undefined };
}

export function parseWindowObservation(value: unknown): NativeWindowObservation | null {
  const r = recordOf(value);
  if (!r || r.schema !== "vitshell.window_state.v1" || !["shell", "daw", "other", "none"].includes(text(r.foreground))
      || typeof r.observed_at_tick_ms !== "number" || !Number.isFinite(r.observed_at_tick_ms) || r.observed_at_tick_ms < 0
      || !(r.daw_hwnd === null || (typeof r.daw_hwnd === "string" && /^[1-9]\d*$/.test(r.daw_hwnd)))) return null;
  // A DAW foreground claim with no observed DAW HWND is insufficient evidence.
  if (r.foreground === "daw" && r.daw_hwnd === null) return null;
  return { foreground: r.foreground as NativeWindowObservation["foreground"], dawHwnd: r.daw_hwnd as string | null, observedAt: r.observed_at_tick_ms };
}

export type WindowPresentationEvent =
  | { type: "begin"; requestId: string; title: string }
  | { type: "progress"; progress: LaunchProgress; replay: boolean }
  | { type: "window"; observation: NativeWindowObservation | null }
  | { type: "result"; requestId: string; status: string; message: string };

const STEP_LABELS: Partial<Record<LaunchPhase, string>> = {
  waiting_governance: "正在核对工程切换条件", saving: "正在保存当前工程", closing_daw: "正在关闭原工作台",
  spawning: "正在拉起工作台", waiting_launch_result: "正在等待工作台装载回执"
};
export function reduceWindowPresentation(state: WindowPresentationState, event: WindowPresentationEvent): WindowPresentationState {
  if (event.type === "window") {
    if (event.observation && state.window && event.observation.observedAt < state.window.observedAt) return state;
    return { ...state, window: event.observation };
  }
  if (event.type === "begin") {
    return { ...state, requestId: event.requestId, terminal: "",
      view: reduceLoading(reduceLoading(emptyLoadingView(), { type: "loadingStarted", title: event.title }), { type: "loadingStepCurrent", label: "请求已发出，等待处理" }) };
  }
  if (event.type === "result") {
    if (!state.requestId || event.requestId !== state.requestId || state.terminal) return state;
    if (event.status === "opened") return { ...state, terminal: "unconfirmed" }; // no stage in the legacy result message
    if (event.status === "canceled") return { ...state, terminal: "canceled", view: reduceLoading(state.view, { type: "loadingInterrupted", reason: event.message || "已取消打开工程" }) };
    return { ...state, terminal: "failed", view: reduceLoading(state.view, { type: "loadingFailed", reason: event.message || "工程动作未完成，请检查宿主状态后重试。" }) };
  }
  const p = event.progress;
  if (p.seq <= state.lastSeq || (state.requestId && p.requestId !== state.requestId) || (!state.requestId && !event.replay)) return state;
  // A late replay may restore a request after the App is mounted again. A
  // window release_reason never supplies a request's terminal result.
  let view = state.view.phase === "idle" ? reduceLoading(emptyLoadingView(), { type: "loadingStarted", title: "" }) : state.view;
  const base = { ...state, requestId: p.requestId, lastSeq: p.seq };
  if (state.terminal && state.terminal !== "unconfirmed") return base;
  if (p.phase === "opened") {
    if (p.stage === "workspace_ready") {
      view = { ...view, steps: view.steps.map(s => ({ ...s, status: "done" as const })) };
      return { ...base, terminal: "ready", view: reduceLoading(view, { type: "loadingSucceeded", note: "工作台已打开，工程内容已载入" }) };
    }
    return { ...base, terminal: p.stage === "scene_accepted" ? "accepted" : "unconfirmed", view };
  }
  if (p.phase === "failed" || p.phase === "canceled") {
    const canceled = p.phase === "canceled";
    return { ...base, terminal: canceled ? "canceled" : "failed", view: reduceLoading(view, canceled
      ? { type: "loadingInterrupted", reason: "已取消打开工程" }
      : { type: "loadingFailed", reason: p.detail || "工作台未完成装载，请检查宿主状态后重试。" }) };
  }
  if (state.terminal === "unconfirmed") return base;
  const label = STEP_LABELS[p.phase];
  // Replay can arrive before older queued events. Sequence filtering prevents
  // replay from becoming another step, and no missing steps are filled in.
  if (label && view.steps.at(-1)?.label !== label) view = reduceLoading(view, { type: "loadingStepCurrent", label });
  return { ...base, view };
}

interface PresentationContext {
  state: WindowPresentationState; sidebarOpen: boolean; setSidebarOpen: (open: boolean) => void;
  begin: (requestId: string, title: string) => void;
  result: (requestId: string, status: string, message: string) => void;
}
const emptyContext: PresentationContext = { state: emptyWindowPresentation(), sidebarOpen: true, setSidebarOpen: () => {}, begin: () => {}, result: () => {} };
const Context = createContext<PresentationContext>(emptyContext);
type Webview = { postMessage: (message: unknown) => void; addEventListener?: (type: "message", listener: (event: MessageEvent) => void) => void; removeEventListener?: (type: "message", listener: (event: MessageEvent) => void) => void };

export function WindowPresentationProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(reduceWindowPresentation, undefined, emptyWindowPresentation);
  const [sidebarOpen, setSidebarOpen] = useState(true);
  useEffect(() => {
    const webview = (window as unknown as { chrome?: { webview?: Webview } }).chrome?.webview;
    if (!webview?.addEventListener) return;
    const subscriptionId = `window-presentation-${crypto.randomUUID()}`;
    const listener = (event: MessageEvent) => {
      const r = recordOf(event.data);
      if (!r || (r.replay === true && r.subscription_id !== subscriptionId)) return;
      if (r.type === "vit_window_state") dispatch({ type: "window", observation: parseWindowObservation(r.window_state) });
      const progress = parseLaunchProgress(r);
      if (progress) dispatch({ type: "progress", progress, replay: r.replay === true });
      if (r.type === "vit_launcher_result" && text(r.request_id)) dispatch({ type: "result", requestId: text(r.request_id), status: text(r.status), message: text(r.message) });
    };
    webview.addEventListener("message", listener);
    try {
      webview.postMessage({ channel: "ask_vit_shell", type: "window_presentation_ready", subscription_id: subscriptionId });
    } catch {
      // A partial/unavailable host remains unknown. No browser focus or
      // synthetic progress substitutes for its missing observation.
    }
    return () => webview.removeEventListener?.("message", listener);
  }, []);
  const begin = useCallback((requestId: string, title: string) => dispatch({ type: "begin", requestId, title }), []);
  const result = useCallback((requestId: string, status: string, message: string) => dispatch({ type: "result", requestId, status, message }), []);
  return <Context.Provider value={{ state, sidebarOpen, setSidebarOpen, begin, result }}>{children}</Context.Provider>;
}
export const useWindowPresentation = () => useContext(Context);
export const isWindowLaunchBusy = (state: WindowPresentationState) => Boolean(state.requestId && !state.terminal && state.view.phase === "loading");

export function NativeFocusHint() {
  const { state } = useWindowPresentation();
  return state.window?.foreground === "daw" ? <span className="v5-window-focus-hint" data-native-focus-hint>工程已在前台</span> : null;
}

export function WindowLaunchFeedback({ onRetry }: { onRetry?: () => void }) {
  const { state } = useWindowPresentation();
  const feedbackRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const element = feedbackRef.current;
    if (element?.getClientRects().length) element.scrollIntoView({ block: "nearest", behavior: "instant" });
  }, [state.requestId, state.lastSeq, state.terminal]);
  if (!state.requestId) return null;
  const limited = state.terminal === "accepted" || state.terminal === "unconfirmed";
  return <div ref={feedbackRef} className="v5-window-launch-feedback" data-window-launch-feedback data-window-launch-request={state.requestId}
    data-window-launch-seq={state.lastSeq} data-window-launch-terminal={state.terminal || "pending"} data-window-launch-progress="unknown" role="status" aria-live="polite">
    {limited ? <p className="v5-window-launch-limited" data-window-launch-limited>{state.terminal === "accepted" ? "工作区切换已受理；完整装载状态尚未确认。" : "打开请求已返回；工作台就绪状态尚未确认。"}</p>
      : <LoadingCard view={state.view} onRetry={state.terminal === "failed" || state.terminal === "canceled" ? onRetry : undefined} />}
  </div>;
}
