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
import { createPortal } from "react-dom";
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
import { useEntryRail } from "../entry/entryRail";
// CONV-RAIL-1（418，B2a）：对话流左栏树 + 只读预览 + 继续/新对话。
// 数据源=App 注入的 /agent/ui/state 投影；动作走生产缝 /agent/invoke（version.checkout /
// version.branch_create），参数/上下文复用 history/tree 的 historyToolArgs /
// buildHistoryInvokeContext（与 HistoryPane 同一套组装，避免第二套协议）。
import { ConversationRail } from "../entry/ConversationRail";
import {
  conversationActiveBranchName,
  conversationActiveNodeCommitID,
  conversationActiveNodeId,
  conversationBranchMessages,
  conversationHasProjectIdentity,
  nextConversationBranchName,
  type ConversationBranchTarget,
  type ConversationWorktreeStatus,
  type ConversationWorktreeStatusMap
} from "../entry/conversationData";
import { invokeAgent } from "../lib/api";
import { buildHistoryInvokeContext, historyToolArgs } from "../history/tree";
import { asRecord, textValue } from "../lib/util";
import type { AgentInvokeResponse, AgentUIState } from "../types";

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
  /** CONV-RAIL-1（418）：live App 注入的 /agent/ui/state 投影（含 project_history
   * 对话图 / checkout_blocked）。fixture/gallery/静态渲染不传 → 对话流左栏树与只读
   * 预览不渲染，既有渲染树逐字不变。 */
  conversationUIState?: AgentUIState | null;
  /** CONV-RAIL-1（418）：动作成功后刷新对话流数据（App 重新 fetchUIState；不传=不刷新，
   * 动作成功事实不受刷新失败影响）。 */
  onRefreshConversationState?: () => void | Promise<unknown>;
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

/**
 * CONV-RAIL-CROSS-1（421）：version.conversation_open 失败文案——如实披露回执里的
 * target_materialized（目标副本是否已物化）与 branch_warning，不静默、不美化失败。
 */
function conversationOpenFailureText(response: AgentInvokeResponse): string {
  const result = asRecord(response.result);
  const parts = [response.error || "打开工作树并继续对话失败。"];
  const materialized = result.target_materialized;
  if (materialized === true || materialized === false) {
    parts.push(`目标副本已物化：${materialized ? "是" : "否"}`);
  } else if (typeof materialized === "string" && materialized.trim()) {
    parts.push(`目标副本物化状态：${materialized.trim()}`);
  }
  const branchWarning = textValue(result.branch_warning, "");
  if (branchWarning) {
    parts.push(`分支提示：${branchWarning}`);
  }
  const switchWarning = textValue(result.switch_autosave_warning, "");
  if (switchWarning) {
    parts.push(`切换前保存提示：${switchWarning}`);
  }
  return parts.join("；");
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

export function ProjectSurface({
  fixture,
  fill = false,
  onOpenWorkbench,
  onViewLauncher,
  onOpenSettings,
  active = true,
  conversationUIState,
  onRefreshConversationState
}: ProjectSurfaceProps) {
  const presentation = useWindowPresentation();
  const launchBusy = isWindowLaunchBusy(presentation.state);
  // CONV-RAIL-1（418，B2a）：live 入口壳 rail context + App 注入的 ui state → 左栏对话流树。
  // 无 context（fixture/gallery/静态渲染）或未注入数据 = 不渲染树 → 既有渲染树逐字不变。
  const entryRail = useEntryRail();
  const railHost = entryRail?.host ?? null;
  const conversationLive =
    !fixture && Boolean(railHost) && entryRail?.surface === "project" && conversationUIState !== undefined;
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

  // ── CONV-RAIL-1（418，B2a）/ CONV-RAIL-CROSS-1（421，B2b）：对话流左栏 + 只读预览 ──
  // 数据源=App 注入的 /agent/ui/state（project_history 对话图 + checkout_blocked）；
  // 其它工作树展开时懒加载 version.status（只读），缓存于本组件（跨 surface 切换不丢）。
  // 浏览一律不写：当前工作树分支走 version.checkout，其它工作树走 version.conversation_open。
  const conversationHistory = asRecord(conversationUIState?.project_history);
  const conversationActiveBranch = conversationActiveBranchName(conversationHistory);
  const conversationActiveNode = conversationActiveNodeId(conversationHistory);
  const conversationCheckoutBlocked = Boolean(conversationUIState?.checkout_blocked);
  const [previewTarget, setPreviewTarget] = useState<ConversationBranchTarget | null>(null);
  const [conversationBusy, setConversationBusy] = useState(false);
  const [conversationError, setConversationError] = useState("");
  const [expandedWorktreeKeys, setExpandedWorktreeKeys] = useState<string[]>([]);
  const [worktreeStatus, setWorktreeStatus] = useState<ConversationWorktreeStatusMap>({});
  const worktreeStatusRef = useRef<ConversationWorktreeStatusMap>({});
  const worktreeRequestRef = useRef<Set<string>>(new Set());
  const previewing = Boolean(previewTarget);
  const previewMessages = useMemo(
    () => (previewTarget ? conversationBranchMessages(previewTarget.history, previewTarget.branch) : []),
    [previewTarget]
  );
  // 离开工程面（切回管理视图）时清掉只读预览：下次进入工程面仍在当前对话。
  useEffect(() => {
    if (!active) {
      setPreviewTarget(null);
      setConversationError("");
    }
  }, [active]);

  const applyWorktreeStatus = useCallback((key: string, next: ConversationWorktreeStatus) => {
    worktreeStatusRef.current = { ...worktreeStatusRef.current, [key]: next };
    setWorktreeStatus(worktreeStatusRef.current);
  }, []);

  /**
   * 懒加载其它工作树 version.status（只读，不 checkout/不 open_project/不重载 DAW）。
   * 展开一次只取一次：已缓存或已失败时不自动重取（失败由用户显式点重试）；force=手动刷新。
   */
  const loadWorktreeStatus = useCallback(
    async (key: string, projectFilePath: string, force = false) => {
      if (!key || !projectFilePath || worktreeRequestRef.current.has(key)) {
        return;
      }
      const current = worktreeStatusRef.current[key];
      if (!force && (current?.history || current?.error)) {
        return;
      }
      worktreeRequestRef.current.add(key);
      applyWorktreeStatus(key, { loading: true, error: "", history: current?.history ?? null });
      try {
        const response = await invokeAgent({
          tool: "version.status",
          args: { project_path: projectFilePath },
          confirmed: false,
          source: "ask_vit_webui_conversation"
        });
        if (response.error || response.status === "error") {
          throw new Error(response.error || "读取工作树历史失败。");
        }
        const result = asRecord(response.result ?? response.project_history);
        applyWorktreeStatus(key, { loading: false, error: "", history: result });
      } catch (error) {
        applyWorktreeStatus(key, {
          loading: false,
          error: error instanceof Error ? error.message : "读取工作树历史失败。",
          history: current?.history ?? null
        });
      } finally {
        worktreeRequestRef.current.delete(key);
      }
    },
    [applyWorktreeStatus]
  );

  const toggleWorktree = useCallback(
    (key: string, projectFilePath: string) => {
      setExpandedWorktreeKeys((previous) =>
        previous.includes(key) ? previous.filter((item) => item !== key) : [...previous, key]
      );
      const current = worktreeStatusRef.current[key];
      if (!current?.history && !current?.loading && !current?.error) {
        void loadWorktreeStatus(key, projectFilePath);
      }
    },
    [loadWorktreeStatus]
  );

  const refreshWorktree = useCallback(
    (key: string, projectFilePath: string) => {
      void loadWorktreeStatus(key, projectFilePath, true);
    },
    [loadWorktreeStatus]
  );

  const refreshConversationState = useCallback(async () => {
    try {
      await onRefreshConversationState?.();
    } catch {
      // 刷新失败不改变动作成功的事实；左栏保持旧数据，用户可再次操作刷新。
    }
  }, [onRefreshConversationState]);

  const exitConversationPreview = useCallback(() => {
    setPreviewTarget(null);
    setConversationError("");
  }, []);

  const selectConversationBranch = useCallback(
    (target: ConversationBranchTarget) => {
      setConversationError("");
      // 点当前分支=清预览；点其它分支（含其它工作树/未归属分支）=只读预览，
      // 浏览本身不调任何写工具、不重载工程。
      if (target.isActiveWorktree && target.branch === conversationActiveBranch) {
        setPreviewTarget(null);
        return;
      }
      setPreviewTarget(target);
    },
    [conversationActiveBranch]
  );

  const continueConversation = useCallback(
    async (target: ConversationBranchTarget) => {
      if (!target || conversationBusy) {
        return;
      }
      if (target.isActiveWorktree && target.branch === conversationActiveBranch) {
        return;
      }
      if (conversationCheckoutBlocked) {
        setConversationError("Agent Turn 正在运行；请先 Stop Turn 再继续该对话。");
        return;
      }
      if (target.readOnly) {
        setConversationError("该分支未归属任何可打开的工作树，仅支持只读浏览。");
        return;
      }
      // 继续=单次工程重载；跨工作树=组合缝 version.conversation_open（一次 reload）。
      // 先显式确认「将回到/打开该对话对应的工程状态」。
      const confirmText = target.isActiveWorktree
        ? "继续此对话会回到该对话对应的工程状态（当前工程未保存的改动按既有自动保存语义处理）。是否继续？"
        : `打开工作树「${target.worktreeLabel}」并继续此对话？这会切换到该工作树对应的工程状态（当前工程未保存的改动按既有自动保存语义处理）。是否继续？`;
      if (!window.confirm(confirmText)) {
        return;
      }
      setConversationBusy(true);
      setConversationError("");
      try {
        if (target.isActiveWorktree) {
          const response = await invokeAgent({
            tool: "version.checkout",
            args: historyToolArgs(conversationHistory, { branch: target.branch }),
            confirmed: true,
            source: "ask_vit_webui_conversation",
            context: buildHistoryInvokeContext(conversationUIState ?? null, conversationHistory)
          });
          if (response.error || response.status === "error") {
            throw new Error(response.error || "继续对话失败。");
          }
        } else {
          // 组合缝：目标工作树副本 + 分支 + head 节点；agent 侧做校验/autosave/物化/
          // 恰一次内核 reload。失败时回执仍披露 target_materialized/branch_warning。
          const response = await invokeAgent({
            tool: "version.conversation_open",
            args: {
              project_file_path: target.projectFilePath,
              branch: target.branch,
              node_id: target.headNodeId
            },
            confirmed: true,
            source: "ask_vit_webui_conversation"
          });
          if (response.error || response.status === "error") {
            throw new Error(conversationOpenFailureText(response));
          }
        }
        // 先等 ui state 刷新到位再清预览：loading 期间仍保留「正在切换工程状态」的
        // 可读态，避免旧树/旧高亮短暂复现。
        await refreshConversationState();
        setPreviewTarget(null);
      } catch (error) {
        // 失败保留只读预览并显示原因；不自动重试（重试=用户再次点击动作按钮）。
        setConversationError(error instanceof Error ? error.message : "继续对话失败。");
      } finally {
        setConversationBusy(false);
      }
    },
    [
      conversationActiveBranch,
      conversationBusy,
      conversationCheckoutBlocked,
      conversationHistory,
      conversationUIState,
      refreshConversationState
    ]
  );

  const startNewConversation = useCallback(async () => {
    if (conversationBusy) {
      return;
    }
    if (conversationCheckoutBlocked) {
      setConversationError("Agent Turn 正在运行；请先 Stop Turn 再新建对话。");
      return;
    }
    // SHELL-PROJECT-STREAM-FIX-1（427）：无活动节点不再提前报错。只要工程身份
    // （project_path/active branch/head）存在，就按当前 HEAD 分叉——agent 侧
    // version.branch_create 省略 from_node_id/commit_id 时即按当前 HEAD 建分支。
    if (!conversationActiveNode && !conversationHasProjectIdentity(conversationHistory)) {
      setConversationError("当前工程还没有可用的工程状态，暂时不能新建对话。");
      return;
    }
    // 新对话=从当前活动节点分叉（分支名确定性生成，避免与既有分支撞名）。
    const name = nextConversationBranchName(Object.keys(asRecord(conversationHistory.branches)));
    const commitID = conversationActiveNode ? conversationActiveNodeCommitID(conversationHistory) : "";
    setConversationBusy(true);
    setConversationError("");
    try {
      const response = await invokeAgent({
        tool: "version.branch_create",
        // 有活动节点：from_node_id + commit_id 精确落在该节点；无活动节点：工具字段只带
        // name（historyToolArgs 仅补 project_path 这一全部历史工具共用的寻址参数）。
        args: historyToolArgs(
          conversationHistory,
          conversationActiveNode
            ? {
                name,
                from_node_id: conversationActiveNode,
                ...(commitID ? { commit_id: commitID } : {})
              }
            : { name }
        ),
        confirmed: true,
        source: "ask_vit_webui_conversation",
        context: buildHistoryInvokeContext(conversationUIState ?? null, conversationHistory)
      });
      if (response.error || response.status === "error") {
        throw new Error(response.error || "新建对话失败。");
      }
      // 生产实现 version.branch_create 会物化并切到新分支（写 refs/HEAD + active_branch）；
      // 若回执证明未自动 checkout，再补一次 checkout（卡面「若未自动 checkout 再补」）。
      const created = asRecord(response.result);
      const createdBranch = textValue(created.branch, "");
      const checkedOutTo = textValue(created.checked_out_to, "");
      if (!createdBranch || createdBranch !== name || !checkedOutTo) {
        const checkout = await invokeAgent({
          tool: "version.checkout",
          args: historyToolArgs(conversationHistory, { branch: name }),
          confirmed: true,
          source: "ask_vit_webui_conversation",
          context: buildHistoryInvokeContext(conversationUIState ?? null, conversationHistory)
        });
        if (checkout.error || checkout.status === "error") {
          throw new Error(checkout.error || "新对话已创建，但切换工程状态失败。");
        }
      }
      await refreshConversationState();
      setPreviewTarget(null);
    } catch (error) {
      setConversationError(error instanceof Error ? error.message : "新建对话失败。");
    } finally {
      setConversationBusy(false);
    }
  }, [
    conversationActiveNode,
    conversationBusy,
    conversationCheckoutBlocked,
    conversationHistory,
    conversationUIState,
    refreshConversationState
  ]);

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
        // 427：工程上下文就绪后同步刷新左栏对话流数据（active node/branches/新对话
        // 可用性跟上服务端 ui state；失败静默保留旧数据）。
        void refreshConversationState();
      })
      .catch((error: unknown) => {
        if (contextRunRef.current !== run) return;
        setContextError(error instanceof Error ? error.message : "工程上下文暂不可用");
        setContextLoading(false);
      });
    return () => {
      contextRunRef.current += 1;
    };
  }, [fixture, fixtureService, refreshConversationState]);

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
        void refreshConversationState();
      })
      .catch((error: unknown) => {
        if (contextRunRef.current !== run) return;
        setContextError(error instanceof Error ? error.message : "工程上下文暂不可用");
        setContextLoading(false);
      });
  }, [fixture, refreshConversationState]);

  const send = useCallback(
    async (text: string) => {
      const trimmed = text.trim();
      const service = serviceRef.current;
      // 只读预览期间输入被蒙版禁用：即使事件绕过 UI 也不得发送。
      if (!trimmed || previewing || state.inputDisabled || state.replying || launchBusy || !service) return;
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
        // 427：发送完成后刷新 /agent/ui/state，让左栏 active node/branches/新对话
        // 状态跟上（失败静默保留旧数据，不打断对话）。
        void refreshConversationState();
      }
    },
    [state.inputDisabled, state.replying, launchBusy, executeDirective, previewing, refreshConversationState]
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

  // CONV-RAIL-CROSS-1（421，B2b）：工程面左栏内容 = 工作区→工作树→分支对话流树
  //（数据=App 注入的 ui state + 其它工作树 version.status 懒加载缓存）；点击任何
  // 非当前分支只进入只读预览，不调 checkout/open_project、不重载工程。
  const conversationRailPortal =
    conversationLive && railHost
      ? createPortal(
          <ConversationRail
            uiState={conversationUIState ?? null}
            expandedWorktreeKeys={expandedWorktreeKeys}
            worktreeStatus={worktreeStatus}
            previewTarget={previewTarget ? { worktreeKey: previewTarget.worktreeKey, branch: previewTarget.branch } : null}
            checkoutBlocked={conversationCheckoutBlocked}
            busy={conversationBusy}
            error={conversationError}
            onSelectBranch={selectConversationBranch}
            onToggleWorktree={toggleWorktree}
            onRefreshWorktree={refreshWorktree}
            onNewConversation={() => void startNewConversation()}
          />,
          railHost
        )
      : null;

  // CONV-RAIL-1（418，B2a）/ CONV-RAIL-CROSS-1（421，B2b）：只读预览内容——
  // 上方动作卡（继续/打开此工作树并继续 + 确认/守卫/失败原因/重试）+ 分支 head
  // 回溯 parent 链的消息路径（Message 组件只读渲染）。
  const previewKind = previewTarget
    ? previewTarget.readOnly
      ? "readonly"
      : previewTarget.isActiveWorktree
        ? "current"
        : "cross"
    : "";
  const conversationPreview = previewTarget ? (
    <div
      className="v5-project-preview"
      data-project-preview
      data-project-preview-branch={previewTarget.branch}
      data-project-preview-worktree={previewTarget.worktreeLabel}
      data-project-preview-kind={previewKind}
    >
      <div className="v5-project-preview-card" data-project-preview-card>
        <div className="v5-project-preview-head">
          <span className="v5-project-preview-title" data-project-preview-title>
            {previewKind === "current"
              ? "正在只读浏览历史对话"
              : previewKind === "cross"
                ? `正在只读浏览「${previewTarget.worktreeLabel}」的历史对话`
                : "正在只读浏览未归属分支"}
          </span>
          <span className="v5-project-preview-branch" data-project-preview-branch-name>
            {previewKind === "current" ? previewTarget.branch : `${previewTarget.worktreeLabel} · ${previewTarget.branch}`}
          </span>
        </div>
        <p className="v5-project-preview-note" data-project-preview-note>
          {previewKind === "current"
            ? "浏览历史对话不切换工程、不重载 DAW，输入已暂停。继续此对话会回到该对话对应的工程状态（当前工程未保存的改动按既有自动保存语义处理）。"
            : previewKind === "cross"
              ? "浏览其它工作树不切换工程、不重载 DAW，输入已暂停。打开此工作树并继续此对话会切换到该工作树的工程状态（当前工程未保存的改动按既有自动保存语义处理）。"
              : "该分支未归属任何可打开的工作树，仅供只读浏览；输入已暂停。"}
        </p>
        {conversationError ? (
          <p className="v5-project-preview-error" data-project-preview-error>
            {conversationError}
          </p>
        ) : null}
        <div className="v5-project-preview-actions" data-project-preview-actions>
          {previewKind === "readonly" ? (
            <span className="v5-project-preview-readonly-note" data-project-preview-readonly-note>
              未归属分支仅支持只读浏览
            </span>
          ) : (
            <button
              type="button"
              className="v5-btn v5-btn--primary"
              data-project-preview-continue
              data-project-preview-action={previewKind}
              disabled={conversationBusy || conversationCheckoutBlocked}
              onClick={() => void continueConversation(previewTarget)}
            >
              {previewKind === "current" ? "继续此对话" : "打开此工作树并继续此对话"}
            </button>
          )}
          <button
            type="button"
            className="v5-btn v5-btn--secondary"
            data-project-preview-return
            disabled={conversationBusy}
            onClick={exitConversationPreview}
          >
            返回当前对话
          </button>
          {conversationCheckoutBlocked ? (
            <span className="v5-project-preview-blocked" data-project-preview-blocked>
              Agent Turn 正在运行；请先 Stop Turn 再继续。
            </span>
          ) : null}
          {conversationBusy ? (
            <span className="v5-project-preview-busy" data-project-preview-busy>
              {previewKind === "current" ? "正在切换工程状态…" : "正在打开工作树并切换工程状态…"}
            </span>
          ) : null}
        </div>
      </div>
      <div className="v5-project-preview-thread" data-project-preview-thread>
        {previewMessages.length === 0 ? (
          <p className="v5-project-preview-empty" data-project-preview-empty>
            该对话流还没有可预览的消息。
          </p>
        ) : (
          previewMessages.map((message) => (
            <Message key={message.id} author={message.author}>
              {message.text}
            </Message>
          ))
        )}
      </div>
    </div>
  ) : null;

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
      {conversationRailPortal}
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
            {/* B2a：只读预览优先——浏览非当前对话流时整个线程切换为该分支路径消息，
                实时线程/桥执行块暂时让位；返回或继续后恢复。 */}
            {conversationPreview}
            {conversationPreview ? null : (
              <>
            {/* SHELL-PROJECT-STREAM-FIX-1（427）：工程加载卡（WindowLaunchFeedback/
                LoadingCard）是对话历史里的「装载记录」，固定在消息列表之前——用户/
                agent 消息一律排在它之后；同一时刻只渲染这一张卡（全文件唯一）。 */}
            <div data-project-bridge-exec={bridgeExec?.requestId === presentation.state.requestId ? "" : undefined}
              data-project-bridge-exec-state={bridgeExec?.requestId === presentation.state.requestId ? bridgeExec.state : undefined}>
              <WindowLaunchFeedback onRetry={bridgeExec && bridgeExec.requestId === presentation.state.requestId && lastDirectiveRef.current ? retryLastDirective : undefined} />
            </div>
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
              </>
            )}
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
                  previewing
                    ? "只读浏览中，返回当前对话后可继续输入"
                    : composerDisabled
                      ? "装载完成后可继续输入"
                      : composerBusy
                        ? "Vit 正在回复…"
                        : "告诉 Vit 下一步做什么"
                }
                disabled={composerDisabled || previewing}
                disabledHint="返回当前对话后可继续输入"
                busy={composerBusy}
                onChange={setInput}
                onSend={() => void send(input)}
                onStop={stopReply}
              />
              {/* B2a/B2b：只读预览时输入蒙版——禁用输入并保留「返回当前对话」入口。 */}
              {previewTarget ? (
                <div className="v5-project-composer-mask" data-project-composer-mask>
                  <span className="v5-project-composer-mask-text" data-project-composer-mask-text>
                    正在只读浏览历史对话，输入已暂停
                  </span>
                  <button
                    type="button"
                    className="v5-btn v5-btn--secondary"
                    data-project-composer-mask-return
                    onClick={exitConversationPreview}
                  >
                    返回当前对话
                  </button>
                </div>
              ) : null}
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
