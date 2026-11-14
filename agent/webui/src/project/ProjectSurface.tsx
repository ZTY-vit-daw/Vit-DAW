/*
 * UI-PROJECT-1（346）P3 工程内对话主窗 surface（?surface=project）。
 *
 * 组成（卡面）：工程上下文头（标题栏 = i-project + 工程名，v5.1：V 标不进标题栏）
 * + 消息流 + 宏卡四态 + 快照回执/撤销 + A/B 动作行 + 执行轨迹 + 输入框四态。
 * 左缘抽屉（新建/打开）不在本卡目标清单，另卡接入。
 *
 * 语义红线（projectData.ts 同源）：选择 ≠ 应用；「已应用」只来自服务端真实
 * 成功回执；失败显示原因与重试并保留未应用事实；撤销绑定真实 snapshotRef；
 * A/B 只到 ready（开始方式待用户裁定），不伪造播放。
 *
 * 数据（卡面）：live 模式消费真实端点（ui_state / chat / stop_turn，见
 * projectService.ts）；宏/A/B 无 agent 协议 → live 不入口（service 抛错被拦截），
 * 仅 fixture 模式演示且显式标注（卡面 fixture 例外）。不硬编码示例工程名。
 *
 * fixture 接缝：props 传入 ProjectFixtureOverrides 时完全离线（gallery /
 * render-smoke 使用），真实端点零请求；不传即 live 模式。
 */

import { useCallback, useEffect, useReducer, useRef, useState } from "react";
import type { FormEvent } from "react";
import {
  Button,
  Icon,
  MacroCard,
  Message,
  ProjectBadge,
  SnapshotReceipt,
  StatusCard,
  StatusRing,
  TextField
} from "../components";
import {
  composerStateOf,
  emptyProjectState,
  nextProjectItemID,
  reduceProject,
  type ABActionView,
  type MacroCardView,
  type ProjectComposerState,
  type ProjectContextView,
  type ProjectEvent,
  type ProjectService,
  type ProjectSurfaceState,
  type ProjectThreadItem,
  type SnapshotReceiptView,
  type TrajectoryView
} from "./projectData";
import { createProjectLiveService } from "./projectService";
import { abReadyLabel } from "./projectFixtures";
import "../styles/tokens.css";
import "../components/components.css";
import "./project.css";

export interface ProjectFixtureOverrides {
  /** fixture 工程上下文（离线注入；source 必须为 "fixture"）。 */
  context?: ProjectContextView;
  /** fixture：上下文拉取失败文案（live 上下文不可用态样例）。 */
  contextErrorText?: string;
  /** fixture 初始线程（离线渲染各组件态）。 */
  thread?: ProjectThreadItem[];
  /** fixture 输入禁用（装载中样例）。 */
  inputDisabled?: boolean;
  /** fixture 输入区状态强制样例（仅 gallery 展示；live 不用）。 */
  forceComposerState?: ProjectComposerState;
  /** fixture 服务替身（离线交互；省略 = 纯静态展示）。 */
  service?: ProjectService;
}

export interface ProjectSurfaceProps {
  /** gallery / 渲染烟测 fixture 模式（离线，不出网）。 */
  fixture?: ProjectFixtureOverrides;
  /** 撑满视口（live surface 用）。 */
  fill?: boolean;
}

/** 轨迹视图 → 344 StatusCard 三态映射（idle 由 surface 渲染为待机行）。 */
function statusCardState(view: TrajectoryView): "running" | "completed" | "failed" {
  if (view.state === "busy") return "running";
  if (view.state === "done") return "completed";
  return "failed";
}

function TrajectoryItem({ view }: { view: TrajectoryView }) {
  if (view.state === "idle") {
    return (
      <p className="v5-project-traj-idle" data-project-trajectory-state="idle">
        <StatusRing state="idle" size={16} />
        <span>{view.title || "待机"}</span>
      </p>
    );
  }
  return (
    <div className="v5-project-traj" data-project-trajectory-state={view.state}>
      <StatusCard
        title={view.title}
        steps={view.steps}
        state={statusCardState(view)}
        failureReason={view.failureReason}
      />
    </div>
  );
}

function ABItem({
  view,
  onPrepare,
  busy
}: {
  view: ABActionView;
  onPrepare: () => void;
  busy: boolean;
}) {
  const label = abReadyLabel(view);
  return (
    <div className="v5-project-ab" data-project-ab-state={view.state}>
      {view.state === "preparing" ? (
        <StatusRing state="busy" size={16} />
      ) : view.state === "ready" ? (
        <Icon name="i-done" size={16} />
      ) : view.state === "failed" ? (
        <StatusRing state="error" size={16} />
      ) : (
        <Icon name="i-ab" size={16} />
      )}
      <span className="v5-project-ab-label">{label}</span>
      {view.state === "failed" && view.failureReason ? (
        <span className="v5-project-ab-reason">{view.failureReason}</span>
      ) : null}
      {view.state === "idle" || view.state === "failed" ? (
        <Button variant="secondary" icon="i-ab" disabled={busy} onClick={onPrepare}>
          {view.state === "failed" ? "重试" : "对比 A/B"}
        </Button>
      ) : null}
      {view.state === "ready" ? (
        <span className="v5-project-ab-note">渲染引用已绑定，可随时对比。</span>
      ) : null}
    </div>
  );
}

function SnapshotItem({
  view,
  onUndo
}: {
  view: SnapshotReceiptView;
  onUndo: (ref: string) => void;
}) {
  if (view.undone) {
    return (
      <p className="v5-project-undone" data-project-snapshot-undone="true">
        <Icon name="i-undo" size={16} />
        <span>
          已撤销（{view.time} 的快照）
        </span>
      </p>
    );
  }
  return <SnapshotReceipt time={view.time} onUndo={() => onUndo(view.snapshotRef)} />;
}

function MacroItem({
  itemId,
  view,
  onEvent,
  pending,
  service,
  onError
}: {
  itemId: string;
  view: MacroCardView;
  onEvent: (event: ProjectEvent) => void;
  pending: boolean;
  service: ProjectService | null;
  onError: (text: string) => void;
}) {
  const apply = useCallback(() => {
    // 应用守卫（与 requestMacroApply 同语义，reducer 再验一次）：default + 已选择 + 无在途。
    if (pending || view.state !== "default" || !view.selectedId || !service) return;
    const optionId = view.selectedId;
    void (async () => {
      onEvent({ type: "macroApplyStarted", id: itemId });
      try {
        const outcome = await service.applyMacro(itemId, optionId);
        if (outcome.ok) {
          onEvent({
            type: "macroApplySucceeded",
            id: itemId,
            optionId,
            snapshotRef: outcome.snapshotRef,
            time: outcome.time
          });
        } else {
          onEvent({ type: "macroApplyFailed", id: itemId, reason: outcome.reason });
        }
      } catch (error) {
        onEvent({
          type: "macroApplyFailed",
          id: itemId,
          reason: error instanceof Error ? error.message : "应用请求失败"
        });
      }
    })();
  }, [pending, service, view, itemId, onEvent]);

  return (
    <MacroCard
      object={view.object}
      name={view.name}
      options={view.options}
      selectedId={view.selectedId}
      onSelect={(optionId) => onEvent({ type: "macroSelect", id: itemId, optionId })}
      state={view.state}
      failureReason={view.failureReason}
      onApply={apply}
      onUndo={() => {
        if (!view.appliedSnapshotRef || !service) return;
        const ref = view.appliedSnapshotRef;
        void (async () => {
          try {
            await service.undoSnapshot(ref);
            onEvent({ type: "macroUndo", id: itemId, snapshotRef: ref });
          } catch (error) {
            onError(error instanceof Error ? error.message : "撤销请求失败");
          }
        })();
      }}
      onAudition={(optionId) => onEvent({ type: "macroAuditionStart", id: itemId, optionId })}
      onCancelAudition={() => onEvent({ type: "macroAuditionCancel", id: itemId })}
      onRetry={() => onEvent({ type: "macroRetry", id: itemId })}
    />
  );
}

export function ProjectSurface({ fixture, fill = false }: ProjectSurfaceProps) {
  const fixtureService = fixture?.service ?? null;
  const [state, dispatch] = useReducer(
    (s: ProjectSurfaceState, e: ProjectEvent) => reduceProject(s, e),
    undefined,
    (): ProjectSurfaceState => ({
      ...emptyProjectState(),
      context: fixture?.context ?? null,
      thread: fixture?.thread ?? [],
      inputDisabled: fixture?.inputDisabled ?? false
    })
  );
  const [contextError, setContextError] = useState(fixture?.contextErrorText ?? "");
  const [contextLoading, setContextLoading] = useState(!fixture);
  const [input, setInput] = useState("");
  const [notice, setNotice] = useState("");
  const serviceRef = useRef<ProjectService | null>(fixtureService);
  const contextRunRef = useRef(0);

  // live 模式：服务与上下文各就位（fixture 模式零网络）。
  useEffect(() => {
    if (fixture) {
      serviceRef.current = fixtureService;
      return;
    }
    serviceRef.current = createProjectLiveService();
    const run = ++contextRunRef.current;
    setContextLoading(true);
    setContextError("");
    const service = serviceRef.current;
    void service
      .fetchContext()
      .then((view) => {
        if (contextRunRef.current !== run) return;
        dispatch({ type: "contextLoaded", view });
        setContextLoading(false);
      })
      .catch((error: unknown) => {
        if (contextRunRef.current !== run) return;
        setContextError(error instanceof Error ? error.message : "工程上下文暂不可用");
        setContextLoading(false);
      });
    return () => {
      contextRunRef.current += 1;
    };
  }, [fixture, fixtureService]);

  const retryContext = useCallback(() => {
    if (fixture) return;
    const service = serviceRef.current ?? createProjectLiveService();
    serviceRef.current = service;
    const run = ++contextRunRef.current;
    setContextLoading(true);
    setContextError("");
    void service
      .fetchContext()
      .then((view) => {
        if (contextRunRef.current !== run) return;
        dispatch({ type: "contextLoaded", view });
        setContextLoading(false);
      })
      .catch((error: unknown) => {
        if (contextRunRef.current !== run) return;
        setContextError(error instanceof Error ? error.message : "工程上下文暂不可用");
        setContextLoading(false);
      });
  }, [fixture]);

  const send = useCallback(
    async (text: string) => {
      const trimmed = text.trim();
      const service = serviceRef.current;
      if (!trimmed || state.inputDisabled || state.replying || !service) return;
      dispatch({ type: "userSent", id: nextProjectItemID("msg-user"), text: trimmed });
      setInput("");
      dispatch({ type: "replyStarted" });
      try {
        const reply = await service.sendMessage(trimmed);
        dispatch({ type: "vitReplied", id: nextProjectItemID("msg-vit"), text: reply });
      } catch (error) {
        const message = error instanceof Error ? error.message : "请求失败";
        dispatch({ type: "vitReplied", id: nextProjectItemID("msg-err"), text: `这轮没送达（${message}）。确认 VitAgent 在运行后重发一次。` });
        setInput(trimmed); // 失败还原输入
      } finally {
        dispatch({ type: "replySettled" });
      }
    },
    [state.inputDisabled, state.replying]
  );

  const stopReply = useCallback(() => {
    const service = serviceRef.current;
    if (!service || !state.replying) return;
    void service
      .stopReply()
      .catch(() => undefined)
      .finally(() => dispatch({ type: "replySettled" }));
  }, [state.replying]);

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void send(input);
  };

  const composerState: ProjectComposerState =
    fixture?.forceComposerState ?? composerStateOf(state);
  const composerDisabled = composerState === "disabled";
  const composerBusy = composerState === "replying";

  const frameClass = ["v5-project-frame", fill ? "v5-project-frame--fill" : ""].filter(Boolean).join(" ");
  const contextName = state.context?.name ?? "";
  const contextSource = state.context?.source ?? "none";

  return (
    <div
      className="v5-design v5-project"
      data-project-root
      data-project-state={contextLoading ? "loading" : contextError ? "error" : "ready"}
      data-project-context-source={contextSource}
      data-fixture={fixture ? "project-surface" : undefined}
    >
      <div className={frameClass} data-project-frame>
        <header className="v5-project-titlebar" data-project-titlebar>
          <ProjectBadge size={20} />
          <span className="v5-project-titlebar-name" data-project-context-name={contextName}>
            {contextName || "…"}
          </span>
          {state.context?.detail ? (
            <span className="v5-project-titlebar-detail">{state.context.detail}</span>
          ) : null}
        </header>
        <div className="v5-project-main">
          {contextLoading && !fixture ? (
            <p className="v5-project-context-status" data-project-context-loading>
              正在读取工程上下文…
            </p>
          ) : null}
          {contextError ? (
            <div className="v5-project-context-error" data-project-context-error>
              <span>{contextError || "工程上下文暂不可用"}</span>
              <button type="button" className="v5-btn v5-btn--secondary v5-project-retry" onClick={retryContext} data-project-retry>
                重试
              </button>
            </div>
          ) : null}
          <div className="v5-project-thread" data-project-thread>
            {state.thread.map((item) => {
              if (item.kind === "message") {
                return (
                  <Message key={item.id} author={item.author === "user" ? "user" : "vit"}>
                    {item.text}
                  </Message>
                );
              }
              if (item.kind === "trajectory") {
                return <TrajectoryItem key={item.id} view={item.view} />;
              }
              if (item.kind === "macro") {
                return (
                  <div key={item.id} className="v5-project-macro" data-project-macro-id={item.id}>
                    <MacroItem
                      itemId={item.id}
                      view={item.view}
                      onEvent={dispatch}
                      pending={state.pendingMacroId === item.id}
                      service={serviceRef.current}
                      onError={setNotice}
                    />
                  </div>
                );
              }
              if (item.kind === "snapshot") {
                return (
                  <SnapshotItem
                    key={item.id}
                    view={item.view}
                    onUndo={(ref) => {
                      const service = serviceRef.current;
                      const macroItem = state.thread.find(
                        (t) => t.kind === "macro" && t.view.appliedSnapshotRef === ref
                      );
                      if (!service || !macroItem || macroItem.kind !== "macro") return;
                      void (async () => {
                        try {
                          await service.undoSnapshot(ref);
                          dispatch({ type: "macroUndo", id: macroItem.id, snapshotRef: ref });
                        } catch (error) {
                          setNotice(error instanceof Error ? error.message : "撤销请求失败");
                        }
                      })();
                    }}
                  />
                );
              }
              return (
                <ABItem
                  key={item.id}
                  view={item.view}
                  busy={false}
                  onPrepare={() => {
                    const service = serviceRef.current;
                    if (!service) return;
                    dispatch({ type: "abPrepareStarted", id: item.id });
                    void (async () => {
                      try {
                        const result = await service.prepareAB(item.id);
                        dispatch({ type: "abPrepareSucceeded", id: item.id, bypassRef: result.bypassRef, processedRef: result.processedRef });
                      } catch (error) {
                        dispatch({
                          type: "abPrepareFailed",
                          id: item.id,
                          reason: error instanceof Error ? error.message : "A/B 准备失败"
                        });
                      }
                    })();
                  }}
                />
              );
            })}
            {notice ? (
              <p className="v5-project-notice" data-project-notice>
                {notice}
                <button type="button" className="v5-btn v5-btn--text" onClick={() => setNotice("")}>
                  知道了
                </button>
              </p>
            ) : null}
          </div>
          <form className="v5-project-composer" data-project-composer onSubmit={onSubmit}>
            <div
              className="v5-project-composer-slot"
              data-project-composer-state={composerState}
              onFocusCapture={() => dispatch({ type: "composerFocus", focused: true })}
              onBlurCapture={() => dispatch({ type: "composerFocus", focused: false })}
            >
              <TextField
                value={input}
                placeholder={
                  composerDisabled
                    ? "装载完成后可继续输入"
                    : composerBusy
                      ? "Vit 正在回复…"
                      : "告诉 Vit 下一步做什么"
                }
                disabled={composerDisabled}
                disabledHint="打开完成后可继续输入"
                busy={composerBusy}
                onChange={setInput}
                onSend={() => void send(input)}
                onStop={stopReply}
              />
            </div>
          </form>
        </div>
      </div>
    </div>
  );
}

export default ProjectSurface;
