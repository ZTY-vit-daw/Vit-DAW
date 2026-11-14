/*
 * UI-LOADING-1（347）P2 装载门（live 接入；App.tsx surface=project 分支包裹）。
 *
 * 消费既有端点（零 agent 改动）：fetchUIState() GET /agent/ui/state 单发探测。
 * - probing：渲染装载状态卡（loading 态，当前步=「正在连接 Vit agent」真实
 *   事件：请求已发出）。children 不渲染 → 输入禁用语义成立（无输入框在场）。
 * - ok：放行 children（ProjectSurface 自行拉上下文、渲染 composer = 输入恢复）。
 *   ui_state 成功但无工程上下文时同样放行（工程内 surface 已有 346 错误态语义，
 *   门不重复报错）。
 * - error：装载卡 failed 态（原因原样 + 重试入口）。
 *
 * 红线：不新增 agent 语义、不改启动/打开链（渲染分支而已）；live 不伪造三步
 * 推进（只有真实存在的单步事件；三步细分见 gallery fixture 样张）；完成回执
 * 态作为组件能力由 gallery 呈现，live 成功即放行（工程内主窗上下文头即完成
 * 证据，不人为插帧）。
 */

import { useCallback, useEffect, useReducer, useRef, useState, type ReactNode } from "react";
import { fetchUIState } from "../../lib/api";
import { LoadingCard } from "./LoadingCard";
import { emptyLoadingView, LOADING_LIVE_STEP_LABEL, reduceLoading, type LoadingCardView } from "./loadingData";

export interface ProjectLoadingGateFixture {
  /** fixture 探针终态（省略 = live 模式，真发 ui_state 探测）。 */
  probe?: "ok" | "error";
  /** fixture 失败原因文案。 */
  errorText?: string;
}

export interface ProjectLoadingGateProps {
  children: ReactNode;
  fixture?: ProjectLoadingGateFixture;
}

function lastPathPart(path: string): string {
  const normalized = path.replace(/\\/g, "/");
  const parts = normalized.split("/").filter(Boolean);
  return parts.length > 0 ? parts[parts.length - 1] : path;
}

/** ui_state → 工程名（project_path 尾段；无 path 返回空串=「正在打开工程」）。 */
export function loadingProjectNameFromUIState(uiState: { project?: { project_path?: unknown } | null } | null | undefined): string {
  const path = uiState?.project?.project_path;
  return typeof path === "string" ? lastPathPart(path) : "";
}

export function ProjectLoadingGate({ children, fixture }: ProjectLoadingGateProps) {
  // fixture 终态在初始化期推导（渲染期确定，静态渲染/gallery 可见；零网络）。
  const [view, dispatch] = useReducer(
    (s: LoadingCardView, e: Parameters<typeof reduceLoading>[1]) => reduceLoading(s, e),
    undefined,
    (): LoadingCardView => {
      if (fixture?.probe !== "error") return emptyLoadingView();
      let v = emptyLoadingView();
      v = reduceLoading(v, { type: "loadingStarted", title: "" });
      v = reduceLoading(v, { type: "loadingStepCurrent", label: LOADING_LIVE_STEP_LABEL });
      v = reduceLoading(v, { type: "loadingFailed", reason: fixture.errorText ?? "fixture：装载失败。" });
      return v;
    }
  );
  const [gateState, setGateState] = useState<"probing" | "ok" | "error">(() =>
    fixture ? (fixture.probe === "error" ? "error" : "ok") : "probing"
  );
  const runRef = useRef(0);

  const probe = useCallback(() => {
    if (fixture) {
      // fixture：重放离线终态（零网络；error 态重试入口重放，语义不变）。
      if (fixture.probe === "error") {
        dispatch({ type: "loadingRetry" });
        dispatch({ type: "loadingFailed", reason: fixture.errorText ?? "fixture：装载失败。" });
        setGateState("error");
      } else {
        setGateState("ok");
      }
      return;
    }
    const run = ++runRef.current;
    setGateState("probing");
    dispatch({ type: "loadingStarted", title: "" });
    dispatch({ type: "loadingStepCurrent", label: LOADING_LIVE_STEP_LABEL });
    void fetchUIState()
      .then((uiState) => {
        if (runRef.current !== run) return;
        // 成功即放行（工程名由 children 呈现；成功事件写入视图供语义完整）。
        dispatch({ type: "loadingStarted", title: loadingProjectNameFromUIState(uiState) });
        dispatch({ type: "loadingSucceeded" });
        setGateState("ok");
      })
      .catch((error: unknown) => {
        if (runRef.current !== run) return;
        const message = error instanceof Error ? error.message : "装载请求失败";
        dispatch({ type: "loadingFailed", reason: `连不上 Vit agent——${message}。确认 VitAgent 在运行后重试。` });
        setGateState("error");
      });
  }, [fixture]);

  useEffect(() => {
    if (!fixture) {
      probe();
    }
    return () => {
      runRef.current += 1;
    };
  }, [probe, fixture]);

  const showCard = gateState !== "ok";
  return (
    <div
      className="v5-design v5-loading-gate"
      data-loading-gate
      data-loading-gate-state={gateState}
      data-fixture={fixture ? "loading-gate" : undefined}
    >
      {showCard ? (
        <LoadingCard
          view={view}
          onRetry={gateState === "error" ? probe : undefined}
          className="v5-loading-card--gate"
        />
      ) : (
        children
      )}
    </div>
  );
}
