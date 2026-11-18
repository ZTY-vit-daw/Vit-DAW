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

import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react";
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
  projectThreadItemsFromOutcome,
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
// SHELL-6（池序 367）：工程面交互链复用 launcher 面的桥客户端与纯文案/视图件
//（import-only，零修改 launcher/）。同一 ask_vit_launcher 契约、同一
// vit_governance_required 页内治理载荷——宿主（VitShell LauncherBridge）对
// 整个 webview 生效，不区分 surface；工程面只差订阅与渲染。
import {
  createLauncherBridgeRequestID,
  isLauncherBridgeAvailable,
  postLauncherDirective,
  postLauncherGovernanceChoice,
  subscribeGovernanceRequests,
  subscribeLauncherResults,
  type LauncherBridgeResult,
  type LauncherGovernanceChoice
} from "../launcher/launcherBridge";
import {
  LAUNCHER_BRIDGE_UNAVAILABLE_TEXT,
  LAUNCHER_BRIDGE_WAITING_TEXT,
  LAUNCHER_GOVERNANCE_CANCEL_TEXT,
  LAUNCHER_GOVERNANCE_SAVE_TEXT,
  LAUNCHER_GOVERNANCE_SWITCH_TEXT,
  launcherDirectiveResultText,
  launcherDirectiveResultRetryable,
  launcherGovernanceAnsweredText,
  launcherGovernanceHeadline,
  type LauncherDirectiveState
} from "../launcher/launcherData";
import type { LauncherDirective } from "../api/launcher";
import "../styles/tokens.css";
import "../components/components.css";
import "./project.css";
import { NativeFocusHint, WindowLaunchFeedback, isWindowLaunchBusy, useWindowPresentation } from "./loading/windowPresentation";

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
  /** SCORE-ENTRY-REACH-1（382）：「打开工作台」入口回调——把谱面/媒体/历史/宏
   * 工作台接进工程面可达旅程（App 级 React 内部切 surface，零导航零重载）。
   * 未传（gallery fixture/独立渲染）=按钮不渲染，行为零变化。 */
  onOpenWorkbench?: () => void;
  /** 查看启动页的工程管理视图；工程保持打开，不能用作退出成功回执。 */
  onViewLauncher?: (projectName: string) => void;
  onOpenSettings?: () => void;
  /** 隐藏的工程页保留本地对话，但不消费工程管理视图的治理请求。 */
  active?: boolean;
}

/** 轨迹视图 → 344 StatusCard 三态映射（idle 由 surface 渲染为待机行）。 */
function statusCardState(view: TrajectoryView): "running" | "completed" | "failed" {
  if (view.state === "busy") return "running";
  if (view.state === "done") return "completed";
  return "failed";
}

/** 宿主回执 status → 桥执行态（与 launcher 面同表；未知一律 failed，不静默）。 */
function mapBridgeResultState(status: string): LauncherDirectiveState {
  switch (status) {
    case "opened":
      return "opened";
    case "canceled":
      return "canceled";
    case "unsupported":
      return "unsupported";
    case "dialog_shown":
      return "dialog_shown";
    case "invalid":
      return "invalid";
    case "failed":
      return "failed";
    default:
      return "failed";
  }
}

/** SHELL-6：桥执行块（单活跃执行；终态只来自宿主 vit_launcher_result 回执）。 */
interface ProjectBridgeExecState {
  requestId: string;
  action: string;
  state: LauncherDirectiveState;
  message: string;
}

function bridgeExecStateLabel(state: ProjectBridgeExecState["state"]): string {
  switch (state) {
    case "waiting":
    case "pending":
      return "Vit 执行中…";
    case "opened":
      return "已完成";
    case "canceled":
      return "已取消";
    case "unsupported":
      return "暂未开放";
    case "dialog_shown":
      return "对话框已弹出";
    case "invalid":
      return "指令无效";
    default:
      return "未完成";
  }
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

export function ProjectSurface({ fixture, fill = false, onOpenWorkbench, onViewLauncher, onOpenSettings, active = true }: ProjectSurfaceProps) {
  const presentation = useWindowPresentation();
  const launchBusy = isWindowLaunchBusy(presentation.state);
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

  // ── SHELL-6：工程面交互链（桥执行 + 页内治理三键卡 + 确认承接卡）──
  const bridgeAvailable = useMemo(() => !fixture && isLauncherBridgeAvailable(), [fixture]);
  const [bridgeExec, setBridgeExec] = useState<ProjectBridgeExecState | null>(null);
  const lastDirectiveRef = useRef<LauncherDirective | null>(null);
  const sendRef = useRef<(text: string) => Promise<void>>(async () => {});

  const refreshContextQuietly = useCallback(() => {
    // 切换工程成功后（宿主回执 opened）刷新标题栏工程名；失败静默（标题栏
    // 旧名保持，下一次显式重试仍可用）。
    const service = serviceRef.current;
    if (!service) return;
    const run = ++contextRunRef.current;
    void service
      .fetchContext()
      .then((view) => {
        if (contextRunRef.current !== run) return;
        dispatch({ type: "contextLoaded", view });
      })
      .catch(() => undefined);
  }, []);

  // 桥执行：请求经宿主治理 + 公共链；终态只来自 vit_launcher_result（与
  // launcher 面同一诚实边界——React 不伪造「已切换」）。
  const executeDirective = useCallback(
    (directive: LauncherDirective) => {
      if (fixture) return;
      lastDirectiveRef.current = directive;
      if (!isLauncherBridgeAvailable()) {
        setBridgeExec({
          requestId: createLauncherBridgeRequestID(),
          action: directive.action ?? "",
          state: "failed",
          message: LAUNCHER_BRIDGE_UNAVAILABLE_TEXT
        });
        return;
      }
      const requestId = createLauncherBridgeRequestID();
      const path = directive.file_path?.replace(/\\/g, "/").split("/").pop() || "";
      presentation.begin(requestId, directive.project_name || path);
      setBridgeExec({ requestId, action: directive.action ?? "", state: "waiting", message: LAUNCHER_BRIDGE_WAITING_TEXT });
      const posted = postLauncherDirective(directive as unknown as Record<string, unknown>, requestId);
      if (!posted) {
        presentation.result(requestId, "failed", LAUNCHER_BRIDGE_UNAVAILABLE_TEXT);
        setBridgeExec({
          requestId,
          action: directive.action ?? "",
          state: "failed",
          message: LAUNCHER_BRIDGE_UNAVAILABLE_TEXT
        });
      }
    },
    [fixture, presentation.begin, presentation.result]
  );

  const retryLastDirective = useCallback(() => {
    const directive = lastDirectiveRef.current;
    if (directive) executeDirective(directive);
  }, [executeDirective]);

  // 宿主回执订阅：更新桥执行态；opened 后静默刷新工程上下文（切工程全链收口）。
  useEffect(() => {
    if (fixture) return;
    return subscribeLauncherResults((result: LauncherBridgeResult) => {
      setBridgeExec((prev) => {
        if (!prev) return prev;
        if (result.request_id && result.request_id !== prev.requestId) return prev;
        const status = result.status || "failed";
        const state = mapBridgeResultState(status);
        const message = launcherDirectiveResultText(status, result.message ?? "", result.action || prev.action);
        if (state === "opened") {
          refreshContextQuietly();
        }
        return { ...prev, state, message };
      });
    });
  }, [fixture, refreshContextQuietly]);

  // 页内治理三键卡（SHELL-2 语义全 surface 覆盖——工程面同样渲染；宿主
  // vit_governance_required → 线程卡；选择经 launcher_governance_choice 回投）。
  const answerGovernance = useCallback((itemId: string, requestID: string, choice: LauncherGovernanceChoice) => {
    dispatch({ type: "governanceAnswered", id: itemId, choice });
    postLauncherGovernanceChoice(requestID, choice);
  }, []);

  useEffect(() => {
    if (fixture || !active) return;
    return subscribeGovernanceRequests((request) => {
      dispatch({
        type: "governanceRequested",
        id: nextProjectItemID("governance"),
        view: {
          requestID: request.request_id,
          action: request.action,
          currentProject: request.current_project
        }
      });
    });
  }, [fixture, active]);

  useEffect(() => {
    if (!fixture && active) refreshContextQuietly();
  }, [fixture, active, refreshContextQuietly]);

  // 确认承接卡：按钮=把用户本可亲口说的确认/取消话术走同一对话通道（真实
  // 协议，不硬造）；已作答后固定呈报不重放。
  const answerConfirm = useCallback(
    (itemId: string, choice: "approve" | "cancel", approveText: string, cancelText: string) => {
      dispatch({ type: "confirmAnswered", id: itemId, choice });
      void sendRef.current?.(choice === "approve" ? approveText : cancelText);
    },
    []
  );

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
      if (!trimmed || state.inputDisabled || state.replying || launchBusy || !service) return;
      dispatch({ type: "userSent", id: nextProjectItemID("msg-user"), text: trimmed });
      setInput("");
      dispatch({ type: "replyStarted" });
      try {
        // SHELL-6：完整回执载荷 → 线程卡批（消息/结果卡/directive 占位/确认
        // 承接卡——映射单点在 projectThreadItemsFromOutcome，不变式在那里断言）。
        const outcome = await service.sendMessage(trimmed);
        const mapped = projectThreadItemsFromOutcome(outcome);
        dispatch({ type: "outcomeItemsArrived", items: mapped.items });
        // directive 到达即自动经桥执行（SHELL-5 语义平移到工程面；治理确认
        // 由宿主按 337 语义仅在需要时出页内三键卡）。
        if (mapped.autoDirective) {
          executeDirective(mapped.autoDirective);
        }
      } catch (error) {
        const message = error instanceof Error ? error.message : "请求失败";
        dispatch({ type: "vitReplied", id: nextProjectItemID("msg-err"), text: `这轮没送达（${message}）。确认 VitAgent 在运行后重发一次。` });
        setInput(trimmed); // 失败还原输入
      } finally {
        dispatch({ type: "replySettled" });
      }
    },
    [state.inputDisabled, state.replying, launchBusy, executeDirective]
  );

  // 确认卡/建议入口要调 send：以 ref 解回调环（send 定义于桥执行块之后）。
  useEffect(() => {
    sendRef.current = send;
  }, [send]);

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
  const composerDisabled = composerState === "disabled" || launchBusy;
  const composerBusy = composerState === "replying";

  const frameClass = ["v5-project-frame", fill ? "v5-project-frame--fill" : ""].filter(Boolean).join(" ");
  // 根容器 fill 修饰（UI-VISUAL-FIDELITY-1 W2）：同 launcher--fill，裁切修复。
  const rootClass = ["v5-design v5-project", fill ? "v5-project--fill" : ""].filter(Boolean).join(" ");
  const contextName = state.context?.name ?? "";
  const contextSource = state.context?.source ?? "none";

  return (
    <div
      className={rootClass}
      data-project-root
      data-project-state={contextLoading ? "loading" : contextError ? "error" : "ready"}
      data-project-context-source={contextSource}
      data-project-bridge={bridgeAvailable ? "available" : fixture ? "fixture" : "unavailable"}
      data-fixture={fixture ? "project-surface" : undefined}
      data-native-focus={presentation.state.window?.foreground ?? "unknown"}
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
          <NativeFocusHint />
          {onViewLauncher ? (
            <button type="button" className="v5-btn v5-btn--secondary v5-project-view-launcher" data-project-view-launcher onClick={() => onViewLauncher(contextName)} disabled={state.replying || bridgeExec?.state === "waiting" || bridgeExec?.state === "pending"} title="查看工程管理；当前工程保持打开">
              查看启动页
            </button>
          ) : null}
          {onOpenWorkbench ? (
            <button
              type="button"
              className="v5-btn v5-btn--secondary v5-project-open-workbench"
              data-project-open-workbench
              onClick={onOpenWorkbench}
            >
              打开工作台
            </button>
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
          <div className="v5-project-thread" data-project-thread data-scroll-intentional="thread">
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
              if (item.kind === "directive") {
                // SHELL-6：directive 占位卡（自动执行语态；终态以桥执行块为准）。
                return (
                  <div
                    key={item.id}
                    className="v5-project-directive"
                    data-project-directive
                    data-project-directive-state="auto"
                  >
                    <span className="v5-project-directive-headline">{item.view.headline}</span>
                    {item.view.detail ? <span className="v5-project-directive-detail">{item.view.detail}</span> : null}
                    <span className="v5-project-directive-note">{item.view.note}</span>
                    {item.directive && !bridgeAvailable ? (
                      <span className="v5-project-bridge-note" data-project-bridge-note>
                        {LAUNCHER_BRIDGE_UNAVAILABLE_TEXT}
                      </span>
                    ) : null}
                  </div>
                );
              }
              if (item.kind === "cards") {
                // SHELL-6：结果卡（多命中/最近清单）——点卡=执行该卡 directive。
                return (
                  <div key={item.id} className="v5-project-cards" data-project-cards>
                    {item.cards.map((card) => (
                      <button
                        key={card.id}
                        type="button"
                        className="v5-project-card"
                        data-project-card={card.id}
                        onClick={() => {
                          const cardDirective = card.directive ?? undefined;
                          const mapped = projectThreadItemsFromOutcome({
                            text: "",
                            directive: cardDirective ?? null
                          });
                          dispatch({ type: "outcomeItemsArrived", items: mapped.items });
                          if (mapped.autoDirective) {
                            executeDirective(mapped.autoDirective);
                          }
                        }}
                      >
                        <span className="v5-project-card-name">{card.name || card.filePath}</span>
                        {card.filePath ? <span className="v5-project-card-path">{card.filePath}</span> : null}
                      </button>
                    ))}
                  </div>
                );
              }
              if (item.kind === "governance") {
                // SHELL-6：页内治理三键卡（SHELL-2 语义全 surface 覆盖；选择经
                // 真实桥路径回投宿主执行；已作答后固定呈报不重放）。
                return (
                  <div
                    key={item.id}
                    className="v5-project-governance"
                    data-project-governance
                    data-project-governance-request={item.view.requestID}
                    data-project-governance-answered={item.answered ?? ""}
                  >
                    <span className="v5-project-governance-headline" data-project-governance-headline>
                      {launcherGovernanceHeadline(item.view)}
                    </span>
                    <span className="v5-project-governance-note">
                      请选择保存后切换、直接切换，或留在当前工程。
                    </span>
                    {item.answered ? (
                      <span className="v5-project-governance-answered" data-project-governance-answered-text>
                        {launcherGovernanceAnsweredText(item.answered)}
                      </span>
                    ) : (
                      <div className="v5-project-governance-actions" data-project-governance-actions>
                        <button
                          type="button"
                          className="v5-btn v5-btn--primary"
                          data-project-governance-save
                          disabled={!bridgeAvailable}
                          onClick={() => answerGovernance(item.id, item.view.requestID, "save")}
                        >
                          {LAUNCHER_GOVERNANCE_SAVE_TEXT}
                        </button>
                        <button
                          type="button"
                          className="v5-btn v5-btn--secondary"
                          data-project-governance-switch
                          disabled={!bridgeAvailable}
                          onClick={() => answerGovernance(item.id, item.view.requestID, "switch")}
                        >
                          {LAUNCHER_GOVERNANCE_SWITCH_TEXT}
                        </button>
                        <button
                          type="button"
                          className="v5-btn v5-btn--secondary"
                          data-project-governance-cancel
                          disabled={!bridgeAvailable}
                          onClick={() => answerGovernance(item.id, item.view.requestID, "cancel")}
                        >
                          {LAUNCHER_GOVERNANCE_CANCEL_TEXT}
                        </button>
                      </div>
                    )}
                  </div>
                );
              }
              if (item.kind === "confirm") {
                // SHELL-6：确认承接卡（不变式面——「需要确认」语义回复必伴随
                // 可点交互面；按钮=用户本可亲口说的确认/取消话术走同一通道）。
                return (
                  <div
                    key={item.id}
                    className="v5-project-confirm"
                    data-project-confirm
                    data-project-confirm-answered={item.answered ?? ""}
                  >
                    <span className="v5-project-confirm-note" data-project-confirm-note>{item.view.note}</span>
                    {item.answered ? (
                      <span className="v5-project-confirm-answered" data-project-confirm-answered-text>
                        {item.answered === "approve" ? "已发送「确认执行」。" : "已发送「取消」。"}
                      </span>
                    ) : (
                      <div className="v5-project-confirm-actions" data-project-confirm-actions>
                        <button
                          type="button"
                          className="v5-btn v5-btn--primary"
                          data-project-confirm-approve
                          onClick={() => answerConfirm(item.id, "approve", item.view.approveText, item.view.cancelText)}
                        >
                          {item.view.approveText}
                        </button>
                        <button
                          type="button"
                          className="v5-btn v5-btn--secondary"
                          data-project-confirm-cancel
                          onClick={() => answerConfirm(item.id, "cancel", item.view.approveText, item.view.cancelText)}
                        >
                          {item.view.cancelText}
                        </button>
                      </div>
                    )}
                  </div>
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
            <div data-project-bridge-exec={bridgeExec?.requestId === presentation.state.requestId ? "" : undefined}
              data-project-bridge-exec-state={bridgeExec?.requestId === presentation.state.requestId ? bridgeExec.state : undefined}>
              <WindowLaunchFeedback onRetry={bridgeExec && bridgeExec.requestId === presentation.state.requestId && lastDirectiveRef.current ? retryLastDirective : undefined} />
            </div>
            {bridgeExec && bridgeExec.requestId !== presentation.state.requestId ? (
              <div
                className={`v5-project-bridge-exec v5-project-bridge-exec--${
                  bridgeExec.state === "opened" || bridgeExec.state === "dialog_shown"
                    ? "ok"
                    : bridgeExec.state === "waiting" || bridgeExec.state === "pending"
                      ? "wait"
                      : "bad"
                }`}
                data-project-bridge-exec
                data-project-bridge-exec-state={bridgeExec.state}
              >
                <span className="v5-project-bridge-exec-label">{bridgeExecStateLabel(bridgeExec.state)}</span>
                <span className="v5-project-bridge-exec-message">{bridgeExec.message}</span>
                {bridgeExec.state !== "waiting" && bridgeExec.state !== "pending" && launcherDirectiveResultRetryable(bridgeExec.state) ? (
                  <button
                    type="button"
                    className="v5-btn v5-btn--secondary v5-project-bridge-retry"
                    onClick={retryLastDirective}
                    data-project-bridge-retry
                  >
                    重试
                  </button>
                ) : null}
              </div>
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
        {onOpenSettings ? (
          <footer className="v5-project-footer">
            <button type="button" className="v5-btn v5-btn--secondary" onClick={onOpenSettings} data-project-settings>
              设置
            </button>
          </footer>
        ) : null}
      </div>
    </div>
  );
}

export default ProjectSurface;
