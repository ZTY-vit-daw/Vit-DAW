import {
  Activity,
  AlertTriangle,
  Archive,
  Bot,
  Brain,
  CheckCircle2,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Clock,
  Eye,
  EyeOff,
  File,
  FileAudio,
  FileImage,
  FileText,
  Film,
  Music,
  Gauge,
  GitBranch,
  Globe,
  Link2,
  Loader2,
  Maximize2,
  Minus,
  Copy,
  MoreHorizontal,
  Play,
  Plug,
  Plus,
  Radio,
  RefreshCw,
  Search,
  Send,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
  Square,
  Upload,
  Volume2,
  Wand2,
  X,
  ZoomIn,
  ZoomOut
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { CSSProperties, RefObject } from "react";
import { FormEvent, Fragment, KeyboardEvent as ReactKeyboardEvent, MouseEvent as ReactMouseEvent, PointerEvent as ReactPointerEvent, WheelEvent as ReactWheelEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  artifactFileURL,
  captureBrowserPage,
  confirmPlan,
  deleteArtifact,
  fetchAgentEvents,
  fetchAgentConfig,
  fetchHealth,
  fetchRuntimeStatus,
  fetchUIState,
  invokeAgent,
  listArtifacts,
  readArtifact,
  registerResourceURL,
  revealArtifact,
  renameArtifact,
  renameMacroControl,
  respondInteraction,
  saveAgentConfig,
  selectAudition,
  setAuthorityMode,
  stopTurn,
  stopAudition,
  submitAuditionJudgment,
  type AuditionJudgmentPayload,
  scanDownloads,
  sendChat,
  setMacroControlValue,
  uploadArtifacts,
  watchDownloads
} from "./lib/api";
import {
  dismissActivitiesForTurn,
  dismissActivityByID,
  durableMessage,
  durableMessagesForStorage,
  historyMessageProtocol,
  inferMessageKind,
  isDurableMessage,
  isLegacyTransientMessage,
  messageProtocolIdentityKeys,
  mergeMessageCollections,
  proposalActionIdentity,
  proposalDecisionTranscript,
  reduceAgentEventActivities,
  resolveCompletedTurnProposals,
  resolveSupersededMessages,
  responseMessageProtocol,
  settleSupersededInteractionFamilies,
  settleTerminatedTurnInteractions,
  terminalTurnIDsFromEvents,
  transientMessage,
  TURN_EXPIRED_RESOLVED_ACTION,
  SUPERSEDED_BY_NEWER_RESOLVED_ACTION,
  upsertActivity,
  auditionSessionNativeTurnID
} from "./messageLifecycle";
import { emptyTrajectoryState, hasLiveTrajectoryTurn, reduceTrajectoryEvents, trajectoryTurns } from "./trajectory";
import type { TrajectoryState } from "./trajectory";
import { auditionSessions, emptyAuditionState, reduceAuditionEvents, type AuditionSession, type AuditionState } from "./audition";
import { emptyTaskTrajectoryState, reduceTaskTrajectoryForConversation } from "./taskTrajectory";
import { loadSessionFlowMainID, mainConversationDefaultTitle, saveSessionFlowMainID } from "./sessionFlow";
// UI-LAUNCHER-1（345）：启动台 surface。App.tsx 仅做 surface 分发最小改动——
// launcher 面的实现全部在 src/launcher/**（原 App 主体更名为 MainApp，行为零变化）。
// CONVERGE-SHELL-1（387）：launcher 面换挂收敛壳 ConvergedLauncher（数据流/桥协议
// 平移 + chat 域对话流复用 + 366 磁带卡落位）；旧 LauncherSurface 保留至
// CONVERGE-CLEANUP-1（gallery fixture 面仍消费它，施工图 §7.2 双轨）。
import { ConvergedLauncher } from "./launcher/ConvergedLauncher";
// UI-ENTRY-1：入口 surface 切换桥（宿主→React 单向 + React 取证回执）——进工程展开
// DAW 时宿主通知 React 切 project surface，走内部 setState 零导航零重载（343 不变量）。
import { postEntrySurfaceAck, subscribeEntrySurface } from "./launcher/entrySurfaceBridge";
// UI-PROJECT-1（346）：工程内对话主窗 surface。实现在 src/project/**，此处仅分发（345 同款最小改动，full/main/workbench 行为零变化）。
import { ProjectSurface } from "./project/ProjectSurface";
// UI-LOADING-1（347）：P2 装载门（消费既有 ui_state 探针；不新增 agent 语义，不改打开链）。
import { ProjectLoadingGate } from "./project/loading/ProjectLoadingGate";
import { authorityContext, checkoutBlockedByState, continuationChainLive, isAgentTurnRunning } from "./turnControl";
import { observeOutputLayout, shouldLayerObserveOutput } from "./observeOutputLayering";
import { hasLayeredPresentation, ObservePresentationView } from "./observePresentationLayered";
import { agentEventPollBusy, createAgentEventPollIdleGate } from "./eventPolling";
import { AuditionJudgeCard } from "./trajectory/TrajectoryAuditionPanel";
import { TraceBlock, OptimisticTraceBlock, shouldShowOptimisticTrace } from "./trace/TraceBlock";
import { appendChainResultMessages, chainResultMessagesFromEvents, hasChainTerminalDeliveryEvent, isChainResultChatMessage, settlementMessagesFromEvents } from "./trace/traceDelivery";
import {
  CONSUMED_INTERACTION_GUARD_ID,
  forgetConsumedInteraction,
  isConsumedInteractionStatus,
  loadConsumedInteractionIDs,
  pendingInteractionFromContinuations,
  recordConsumedInteractions,
  resolvedInteractionIdsFromEvents,
  stampConsumedInteractionActions
} from "./interactionGuard";
import { DeclarativeInteractionCard, declarativeCardFromAction } from "./interactionCardKit";
// SCORE-WORKBENCH-1（374）：谱面工作台页签装配（三点最小接线之一）。
import { ScoreWorkbenchPanel } from "./scoreworkbench/ScoreWorkbenchPanel";
import { createAgentScoreBridge } from "./scoreworkbench/kernel";
import { defaultScoreModel } from "./scoreworkbench/model";
import { SessionFlowSidebar } from "./SessionFlowSidebar";
import {
  classifyHistoryScopeChange,
  concreteWorkspacePath,
  historyScopeKeyFromParts,
  historyScopeKeyFromUIState,
  historyScopePartsFromUIState,
  shouldAdoptStoredConversationOnScopeEvolution
} from "./historyScope";
import type { HistoryScopeChangeKind, HistoryScopeParts } from "./historyScope";
import { buildMessageStreamRenderPlan, type MessageStreamEntry } from "./trace/renderPlan";
import { emptyTurnEventMetaMap, reduceTurnEventMeta, type TurnEventMetaMap } from "./trace/turnEventMeta";
// TRAJ-IMPL-2（设计 §2.1）：回合步账——item 工具步还原为回合内进度步，容器存在性与
// 实验准入解耦；活动线按「已归属回合」收窄（streamRoundBoundKeys）。
import {
  emptyRoundStepMap,
  reduceRoundSteps,
  roundActivityBoundKeys,
  roundTurnShell,
  type RoundStepMap
} from "./trace/roundSteps";
// TRAJ-IMPL-3（设计 §2.3 分期一② + §7 裁定 C）：终局回执行台账——只在**终态收口**
// 写入一行摘要（状态/步数/活动数/执行时长/等待时长/started_at，不复述正文），刷新后
// 为活态中不存在的回合渲染静态收起收据行（同键活态优先）。chat 域与协议零接触。
import {
  collectTerminalTurnReceipts,
  loadTurnReceipts,
  mergeTurnReceipts,
  persistTurnReceipts,
  saveTurnReceipts,
  turnReceiptsEqual,
  turnReceiptsStorageKey,
  type TurnReceipt
} from "./trace/turnReceipts";
import { TurnReceiptRow } from "./trace/TurnReceiptRow";
import { PlanBar } from "./composer/PlanBar";
import { laneVisibleActivities } from "./trace/turnGroups";
import type {
  AgentConfigResponse,
  AgentEvent,
  AgentInvokeResponse,
  AgentMode,
  AgentUIState,
  AuthorityMode,
  Artifact,
  ArtifactSummary,
  ChatMessage,
  ChatResponse,
  EngineConfig,
  JsonRecord,
  MacroControl,
  MacroControlBinding,
  MultimodalRouteConfig,
  RuntimeStatusResponse
} from "./types";
// T9-PRE-1A-1（施工图 §3.1 批 1 / 步 1f-A）：通用工具域出域至 src/lib/util.ts——
// 纯移动+re-export 兼容层：import 供 MainApp 既有引用零改动，export * 保持 App 命名面兼容。
import {
  numericValue,
  finiteNumber,
  asRecord,
  firstArray,
  textValue,
  compactJSON,
  parseJSONRecord,
  stableIDPart,
  stableTextHash,
  clampNumber,
  defaultComposerInteractionHeight,
  clampComposerInteractionHeight,
  truthy,
  formatBytes,
  lastPathPart,
  uniqueID,
  currentConversationTitle,
  conversationTitleCandidate,
  compactConversationTitle,
  panelWidthStorageKey,
  loadPanelWidth,
  savePanelWidth
} from "./lib/util";
export * from "./lib/util";
// T9-PRE-1A-1（施工图 §3.1 批 1）：出域模块 re-export 兼容层——import 供 MainApp 既有
// 引用零改动，export * 保持 App 命名面兼容；App 侧对出域函数的跨域引用（动作标签/
// confirmation 域）经此往返，1e 卡移动 actions 域后剪断。
import {
  multimodalRouteMeta,
  defaultSettingsConfig,
  mergeSettingsConfig,
  retainSecretInputs,
  settingsSavedStatus,
  buildChatContext,
  uploadMetadata,
  mediaScopeKeyFromUIState,
} from "./settings/config";
export * from "./settings/config";
// T9-PRE-1A-1（施工图 §3.1 批 1）：出域模块 re-export 兼容层——import 供 MainApp 既有
// 引用零改动，export * 保持 App 命名面兼容；App 侧对出域函数的跨域引用（动作标签/
// confirmation 域）经此往返，1e 卡移动 actions 域后剪断。
import {
  noteSessionSeedMessages,
  historyMessagesFromUIState,
  removeDisposableConfirmationPromptMessages,
  mergeRestoredChatMessages,
  mergeAssistantMessageIntoChat,
  upsertPersistentMixBoardMessage,
  messageMixBoardAction,
  isSameMixBoardSession,
  mixBoardActionSessionID,
} from "./chat/merge";
export * from "./chat/merge";
// T9-PRE-1A-1（施工图 §3.1 批 1）：出域模块 re-export 兼容层——import 供 MainApp 既有
// 引用零改动，export * 保持 App 命名面兼容；App 侧对出域函数的跨域引用（动作标签/
// confirmation 域）经此往返，1e 卡移动 actions 域后剪断。
import {
  initialMessage,
  resolveHistorySyncMessages,
  migrateStoredConversationScope,
  messagesOrIntro,
  hasMeaningfulChatMessages,
  createConversationID,
  scopedConversationRuntimeKey,
  loadStoredScopedConversationID,
  saveStoredScopedConversationID,
  serverConversationIDFromContinuations,
  resolveScopedConversationID,
  conversationMessagesStorageKey,
  loadStoredConversationMessages,
  saveStoredConversationMessages,
} from "./chat/storage";
export * from "./chat/storage";
// T9-PRE-1A-1（施工图 §3.1 批 1）：出域模块 re-export 兼容层——import 供 MainApp 既有
// 引用零改动，export * 保持 App 命名面兼容；App 侧对出域函数的跨域引用（动作标签/
// confirmation 域）经此往返，1e 卡移动 actions 域后剪断。
import {
  HistoryTreeNode,
  HistoryBranchRow,
  HistoryWorktreeRow,
  HistoryLoadedWorktree,
  HistoryNodeMenuState,
  historyTreeNodes,
  historyTreeLayout,
  historyBranchRows,
  historyWorktreeRows,
  historyWorktreeRowFromRecord,
  historyToolArgs,
  historyWorktreeCheckoutArgs,
  buildHistoryInvokeContext,
  historyProjectPath,
  historyActiveBranch,
  historyNodeTitle,
  historyNodeCanCreateFrom,
  historyStatusIsError,
  historyShortID,
  historySafeName,
} from "./history/tree";
export * from "./history/tree";
// T9-PRE-1A-1（施工图 §3.1 批 1）：出域模块 re-export 兼容层——import 供 MainApp 既有
// 引用零改动，export * 保持 App 命名面兼容；App 侧对出域函数的跨域引用（动作标签/
// confirmation 域）经此往返，1e 卡移动 actions 域后剪断。
import {
  macroControlsFromUIState,
  patchMacroControlValueInUIState,
  applyMacroValueOverridesToUIState,
  patchMacroControlNameInUIState,
  normalizeMacroControl,
  mergeMacroControls,
  macroControlID,
  macroControlLabel,
  macroControlValue,
  macroControlDisabledReason,
  macroControlMeta,
  macroBindingSummary,
  macroValueText,
  macroPaneSubtitle,
  activeWorkflowModeLabel,
  macroControlCardAction,
  isMacroControlAction,
  macroControlValueMutation,
  macroControlRenameMutation,
} from "./macro/normalize";
export * from "./macro/normalize";
// T9-PRE-1A-1（施工图 §3.1 批 1）：出域模块 re-export 兼容层——import 供 MainApp 既有
// 引用零改动，export * 保持 App 命名面兼容；App 侧对出域函数的跨域引用（动作标签/
// confirmation 域）经此往返，1e 卡移动 actions 域后剪断。
import {
  DawInvoke,
  DawClip,
  DawPlugin,
  DawTrack,
  dawTracksFromUIState,
  isSystemRackPluginRecord,
  selectedTrackIDFromUIState,
  selectedClipIDFromUIState,
  selectedClipIDsFromUIState,
  isMidiClip,
  focusDawTarget,
  auditionDawTarget,
  actionDawTarget,
  actionLooksLikeDawTarget,
  formatTimelineSeconds,
  numberFromKeys,
  textFromKeys,
} from "./daw/parse";
export * from "./daw/parse";
// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2a）：动作卡体系全域出域至 src/actions/*——
// 纯移动+re-export 兼容层：import 供 MainApp/MessageStream 既有引用零改动，export * 保持 App 命名面兼容。
import { ActionCard, ActionCards } from "./actions/ActionCards";
import { statusLabel, localizeDisplayText, agentEventDisplayTitle } from "./actions/labels";
import { shouldSuppressProposalContent } from "./actions/CapabilityProposalCard";
import { InteractionPager, buildInteractionPages, emptyInteractionPage } from "./actions/InteractionPager";
import { MixBoardActionCard, MixTreatmentPendingCard, isMixBoardAction, isMixPlanningDiscussionAction } from "./actions/MixBoardActionCard";
import { ProjectResultCard, isProjectResultAction, projectResultActionsFromResponse } from "./actions/ProjectResultCard";
import { StandardActionCard, actionRenderID, resolveInteractionInMessages, removeInteractionActionFromMessages, addInteractionDismissals, latestComposerInteraction, isComposerInteraction, isCapabilityProposalInteraction, hasPendingComposerInteraction, isDisposableConfirmationPromptMessage, isDisposableConfirmationPromptText } from "./actions/StandardActionCard";
import {
  isConfirmationAction,
  isPendingInteractionAction,
  mergeJsonRecords,
  interactionPayload,
  confirmationFallbackActionsFromResponse,
  responseNeedsConfirmationFallback,
  fullAccessDirectExecutionTargets,
  stripFullAccessDirectExecutionActions,
  interactionActionsFrom,
  debugConfirmation,
  shouldDebugAgentEvents,
  summarizeAgentEventForConfirmation,
  summarizeChatResponseForConfirmation,
  summarizeMessageForConfirmation,
  summarizeMessagesForConfirmation,
  summarizeActionForConfirmation,
  isFullAccessDirectExecutionInteraction,
  isSyntheticConfirmationInteraction,
  mixBoardActionFromPayload,
  mixBoardActionWithInteractions
} from "./actions/confirmation";
export * from "./actions/types";
export * from "./actions/labels";
export * from "./actions/InteractionPager";
export * from "./actions/ActionCards";
export * from "./actions/CapabilityProposalCard";
export * from "./actions/MixBoardActionCard";
export * from "./actions/ProjectResultCard";
export * from "./actions/StandardActionCard";
export * from "./actions/confirmation";
// T9-PRE-1B-1（386 / 施工图 §3.1 批 2 / 步 2b）：MessageStream+AssistantTextContent 出域至 src/chat/MessageStream.tsx——
// 纯移动+re-export 兼容层：import 供 MainApp 既有引用零改动，export * 保持 App 命名面兼容。
import { MessageStream, AssistantTextContent } from "./chat/MessageStream";
export * from "./chat/MessageStream";
// T9-PRE-1B-1（386 / 施工图 §3.1 批 2 / 步 2c）：Composer 出域至 src/chat/Composer.tsx——
// 纯移动+re-export 兼容层：import 供 MainApp 既有引用零改动，export * 保持 App 命名面兼容。
import { Composer } from "./chat/Composer";
export * from "./chat/Composer";
// T9-PRE-1B-1（386 / 施工图 §3.1 批 2 / 步 2d）：MediaPane/ArtifactPreview/artifact 工具域出域至 src/media/*——
// 纯移动+re-export 兼容层：import 供 MainApp 既有引用零改动，export * 保持 App 命名面兼容。
import { MediaPane } from "./media/MediaPane";
import { mergeArtifacts, artifactSummariesFrom } from "./media/artifacts";
export * from "./media/MediaPane";
export * from "./media/artifacts";
// T9-PRE-1B-1（386 / 施工图 §3.1 批 2 / 步 2e）：Macro 族组件出域至 src/macro/MacroPane.tsx——
// 纯移动+re-export 兼容层：import 供 MainApp 既有引用零改动，export * 保持 App 命名面兼容。
import { MacroPane, MacroContextCard } from "./macro/MacroPane";
export * from "./macro/MacroPane";
// T9-PRE-1B-1（386 / 施工图 §3.1 批 2 / 步 2f）：HistoryPane 出域至 src/history/HistoryPane.tsx——
// 纯移动+re-export 兼容层：import 供 MainApp 既有引用零改动，export * 保持 App 命名面兼容。
import { HistoryPane } from "./history/HistoryPane";
export * from "./history/HistoryPane";
// T9-PRE-1B-1（386 / 施工图 §3.1 批 2 / 步 2g）：BrowserPane+浏览器桥出域至 src/browser/BrowserPane.tsx——
// 纯移动+re-export 兼容层：import 供 MainApp 既有引用零改动，export * 保持 App 命名面兼容。
import { BrowserPane, postAgentMutationsFromChatResponse, postAgentMutationsFromInvokeResponse, postAgentMutations } from "./browser/BrowserPane";
export * from "./browser/BrowserPane";
// T9-PRE-1B-1（386 / 施工图 §3.1 批 2 / 步 2h）：Workbench 族/TopStatusBar/诊断件/SettingsPage 族出域至
// src/workbench/ + src/topbar/ + src/settings/ + src/daw/panes（§3.2 daw 2h 并入）——
// 纯移动+re-export 兼容层：import 供 MainApp 既有引用零改动，export * 保持 App 命名面兼容。
import { SettingsPage, SettingsTextField, RouteEditor } from "./settings/SettingsPage";
import { AuthorityToggle, authoritySwitchNotice } from "./topbar/AuthorityToggle";
import { TopStatusBar } from "./topbar/TopStatusBar";
import { buildDiagnosticsRows, buildDiagnosticsPayload, InfoGrid, KeyValue, EmptyState } from "./topbar/diagnostics";
import { Workbench, WorkbenchTabs, CollapsedWorkbench, PanelResizeHandle } from "./workbench/Workbench";
import { ProjectPane, TracksPane, PluginsPane } from "./daw/panes";
export * from "./topbar/diagnostics";
export * from "./topbar/AuthorityToggle";
export * from "./topbar/TopStatusBar";
export * from "./settings/SettingsPage";
export * from "./workbench/Workbench";
export * from "./daw/panes";









export const WEBUI_BUILD_MARK = "vit-confirmation-consumption-v1-20260801";
export const readonlyAuditMode = import.meta.env.VITE_READONLY_AUDIT === "1";

console.info(`[VitWebUI] loaded ${WEBUI_BUILD_MARK}`);

export const mixBoardUserNoteDrafts = new Map<string, string>();

export type ConnectionStatus = "loading" | "ready" | "offline";
export type WorkbenchTab = "history" | "media" | "macro" | "score";
type AppSurface = "full" | "main" | "workbench" | "launcher" | "project";
export type MacroValueOverride = { value: number; macro: MacroControl; expiresAt: number };

export const browserBridgeChannel = "ask_vit_browser";
export const externalBrowserChannel = "ask_vit_external_browser";
export const agentMutationBridgeChannel = "ask_vit_agent_mutations";
export const defaultExternalBrowserURL = "https://www.bing.com/";
const attachedArtifactStorageKey = "ask_vit_attached_artifact";
const defaultRightPanelWidth = 390;
const minRightPanelWidth = 280;

const modeItems: Array<{ key: AgentMode; label: string; hint: string; icon: LucideIcon }> = [
  { key: "default", label: "协作", hint: "自然对话", icon: Bot },
  { key: "plan", label: "计划", hint: "只读分析", icon: Brain },
  { key: "goal", label: "目标", hint: "长任务", icon: Wand2 }
];

/**
 * UI-LAUNCHER-1（345）：surface 分发入口。launcher 面独立组件（自带数据加载，
 * 不复用主对话的状态/轮询）；UI-PROJECT-1（346）工程内主窗同模式独立分发；
 * full/main/workbench 三 surface 原样走 MainApp，行为零变化。
 * UI-LOADING-1（347）：project 面包装载门——agent 未就绪期间显示 P2 装载状态卡
 * （输入禁用=children 不渲染），ui_state 就绪放行（打开链不变，渲染分支而已）。
 * UI-ENTRY-1：surface 状态化——初始值仍来自 URL 参数（浏览器 dev 直开不变）；
 * 宿主 vit_surface 消息（进工程展开 DAW）经 entrySurfaceBridge 订阅切换 surface，
 * 走 React 内部 setState **零导航零重载**（343 不变量：同一 HWND 同一页面，
 * navigate_count 不增，禁止整页导航）。合法载荷回执 vit_surface_ack（宿主日志
 * 留痕=React 真收证明）；桥不可用（浏览器 dev）=订阅空转+回执不发。
 */
const appSurfaces: ReadonlySet<AppSurface> = new Set<AppSurface>([
  "full",
  "main",
  "workbench",
  "launcher",
  "project"
]);

function App() {
  const [surface, setSurface] = useState<AppSurface>(() => appSurface());
  const [managedProject, setManagedProject] = useState<string | null>(() =>
    appSurface() === "project" ? "当前工程" : null
  );
  const [entrySettingsOpen, setEntrySettingsOpen] = useState(false);
  const [entryRuntime, setEntryRuntime] = useState<RuntimeStatusResponse | null>(null);
  const [entryUIState, setEntryUIState] = useState<AgentUIState | null>(null);
  const entrySettingsRef = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const dialog = entrySettingsRef.current;
    if (entrySettingsOpen && dialog && !dialog.open) dialog.showModal();
    if (!entrySettingsOpen && dialog?.open) dialog.close();
  }, [entrySettingsOpen]);
  useEffect(() => {
    if (!entrySettingsOpen) return;
    let current = true;
    void Promise.allSettled([fetchRuntimeStatus(), fetchUIState()]).then(([runtime, ui]) => {
      if (!current) return;
      setEntryRuntime(runtime.status === "fulfilled" ? runtime.value : null);
      setEntryUIState(ui.status === "fulfilled" ? ui.value : null);
    });
    return () => { current = false; };
  }, [entrySettingsOpen]);
  useEffect(() => {
    return subscribeEntrySurface((next) => {
      const target = next.toLowerCase() as AppSurface;
      if (!appSurfaces.has(target)) {
        return;
      }
      // 宿主消息仍是生命周期权威：DAW 退出发 launcher 时清除管理视图的工程标记。
      if (target === "launcher") setManagedProject(null);
      if (target === "project") setManagedProject((current) => current ?? "当前工程");
      setEntrySettingsOpen(false);
      setSurface(target);
      postEntrySurfaceAck(target);
    });
  }, []);
  if (surface === "launcher" || surface === "project") {
    return (
      <>
        <div hidden={surface !== "launcher"} style={{ height: "100%" }}>
          {surface === "launcher" ? (
            <ConvergedLauncher
              fill
              activeProjectName={managedProject ?? undefined}
              onReturnProject={managedProject ? () => {
                setSurface("project");
                postEntrySurfaceAck("project");
              } : undefined}
              onOpenSettings={() => setEntrySettingsOpen(true)}
            />
          ) : null}
        </div>
        {/* 管理视图只隐藏工程页，保留当前对话/输入；不关闭 DAW、不解绑工程。 */}
        <div hidden={surface !== "project"} style={{ height: "100%" }}>
          {managedProject || surface === "project" ? (
            <ProjectLoadingGate>
              <ProjectSurface
                fill
                active={surface === "project"}
                onOpenWorkbench={() => setSurface("workbench")}
                onViewLauncher={(name) => {
                  setManagedProject(name || "当前工程");
                  setSurface("launcher");
                  postEntrySurfaceAck("launcher");
                }}
                onOpenSettings={() => setEntrySettingsOpen(true)}
              />
            </ProjectLoadingGate>
          ) : null}
        </div>
        <dialog
          ref={entrySettingsRef}
          className="v5-design v5-entry-settings"
          aria-label="设置"
          data-entry-settings
          onCancel={() => setEntrySettingsOpen(false)}
          onClose={() => setEntrySettingsOpen(false)}
        >
          {entrySettingsOpen ? (
            <SettingsPage runtimeStatus={entryRuntime} uiState={entryUIState} onClose={() => setEntrySettingsOpen(false)} />
          ) : null}
        </dialog>
      </>
    );
  }
  return (
    <MainApp
      surface={surface}
      onExitWorkbench={surface === "workbench" ? () => setSurface("project") : undefined}
    />
  );
}

function MainApp({ surface: surfaceProp, onExitWorkbench }: { surface?: AppSurface; onExitWorkbench?: () => void }) {
  // SCORE-ENTRY-REACH-1（382）：surface 以 App() state 为准（URL 初值不变；
  // 工程面「打开工作台」等 React 内部切换〔零导航〕不再被 URL 旧值覆盖——
  // attempt1 实证：MainApp 自读 URL 时按钮点击后 workbench 壳层不渲染）。
  const urlSurface = useMemo(() => appSurface(), []);
  const surface: AppSurface = surfaceProp ?? urlSurface;
  const isWorkbenchSurface = surface === "workbench";
  const isMainSurface = surface === "main";
  const [connection, setConnection] = useState<ConnectionStatus>("loading");
  const [healthLabel, setHealthLabel] = useState("连接中");
  const [runtimeStatus, setRuntimeStatus] = useState<RuntimeStatusResponse | null>(null);
  const [uiState, setUIState] = useState<AgentUIState | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([initialMessage]);
  const [activities, setActivities] = useState<ChatMessage[]>([]);
  const [trajectoryState, setTrajectoryState] = useState(emptyTrajectoryState);
  // GUI-1/M12：item 活动足迹与 turn_kind 标记账本（活动会被回合清退、item 又不
  // 投影为轨迹节点，回合完成后只剩这里能证明「这个回合发生过什么」）
  const [turnEventMeta, setTurnEventMeta] = useState<TurnEventMetaMap>(emptyTurnEventMetaMap);
  // TRAJ-IMPL-2：回合内 item 步账（与 trajectory/turnEventMeta 同键增量归约）
  const [roundSteps, setRoundSteps] = useState<RoundStepMap>(emptyRoundStepMap);
  const [taskTrajectoryState, setTaskTrajectoryState] = useState(emptyTaskTrajectoryState);
  const [auditionState, setAuditionState] = useState(emptyAuditionState);
  const [auditionBusySessionID, setAuditionBusySessionID] = useState("");
  const auditionWaiting = useMemo(() => Object.values(auditionState.sessions).some((session) => session.status === "preparing"), [auditionState]);
  const [conversationID, setConversationID] = useState(initialConversationID);
  // WEBUI-SESSION-SEMANTICS-1：本 scope 的主对话流身份（「主对话流 · <工程名>」）。
  // 独立于 scoped conversation 锚定——新建/切换会话都会改写锚定，主会话身份只在
  // scope 物化时首见即钉（loadSessionFlowMainID 幂等），此后不再跟当前会话走。
  const [mainConversationID, setMainConversationID] = useState("");
  const [mode, setMode] = useState<AgentMode>("default");
  const [authorityMode, setAuthorityModeState] = useState<AuthorityMode>("manual_confirmation");
  const [authorityBusy, setAuthorityBusy] = useState(false);
  const [stopTurnBusy, setStopTurnBusy] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [input, setInput] = useState("");
  const [pendingArtifacts, setPendingArtifacts] = useState<ArtifactSummary[]>([]);
  const [pendingMacroRefs, setPendingMacroRefs] = useState<MacroControl[]>([]);
  const [activeTab, setActiveTab] = useState<WorkbenchTab>("media");
  const [rightPanelOpen, setRightPanelOpen] = useState(true);
  const [rightPanelWidths, setRightPanelWidths] = useState<Record<WorkbenchTab, number>>(() => loadRightPanelWidths());
  const [selectedArtifactID, setSelectedArtifactID] = useState<string>("");
  const [isSending, setIsSending] = useState(false);
  const [isUploading, setIsUploading] = useState(false);
  const [respondingActionID, setRespondingActionID] = useState("");
  const [dismissedInteractionIDs, setDismissedInteractionIDs] = useState<string[]>([]);
  const [messageBottomInset, setMessageBottomInset] = useState(156);
  const [agentEventPolling, setAgentEventPolling] = useState(false);
  // TRAJ-IMPL-3：终局回执行台账（本会话内存视图；权威副本在 localStorage，
  // 每次 scope 就绪时读一次，终态收口时写一行）
  const [turnReceipts, setTurnReceipts] = useState<TurnReceipt[]>([]);
  const [transportBusy, setTransportBusy] = useState(false);
  const [error, setError] = useState("");
  // AB-JUDGMENT-CARD-1：错误显形不被运行时轮询抹掉——refreshState 的成功分支
  // 只允许清掉自己写入的错误（连接面错误）。判定/试听等交互面错误若恰逢一次
  // 在飞 refreshState（挂载期或 8s 周期拍），会被迟到的无条件 setError("") 静默
  // 抹除（E2E J1 实测：点击后 +26ms 渲染、+706ms 被挂载期轮询清除）。
  const runtimeOwnedErrorRef = useRef("");
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const composerRef = useRef<HTMLFormElement | null>(null);
  // PLANBAR-1：输入框停靠列（规划条 + composer）。消息流要预留的底部空间是
  // 整列的高度，不是 composer 一家的高度——否则栏出现时会盖住最后一条内容。
  const composerDockRef = useRef<HTMLDivElement | null>(null);
  const noticeRef = useRef<HTMLDivElement | null>(null);
  const workspaceRef = useRef<HTMLElement | null>(null);
  const historyScopeRef = useRef("");
  // CONTRACT-3：演进/切换分离的状态线——最近具体工作区坐标与 uuid 只在已知时
  // 更新（"unsaved" 拍不回退坐标），保证 关A→空→开B 仍能判出真切换。
  const historyScopeConcreteRef = useRef("");
  const historyScopeUUIDRef = useRef("");
  const scopedConversationRef = useRef("");
  const pausedHistoryScopeRef = useRef("");
  const restoredMessageScopeRef = useRef("");
  const skipNextStoredMessageSaveRef = useRef("");
  // 台账水合只读一次（键 = conversationID + scope）：键变化即重读，同键不重复读
  const restoredReceiptScopeRef = useRef("");
  const turnReceiptsRef = useRef<TurnReceipt[]>([]);
  const mediaScopeRef = useRef("");
  const agentEventSeqRef = useRef(0);
  const macroValueOverridesRef = useRef<Record<string, MacroValueOverride>>({});

  // TRAJ-IMPL-3：台账视图的唯一写入口——ref 与 state 同步推进（写侧按 ref 比较是否
  // 有变化，渲染按 state；两者不同步会导致重复写 localStorage 或渲染滞后一拍）。
  const applyTurnReceipts = useCallback((next: TurnReceipt[]) => {
    turnReceiptsRef.current = next;
    setTurnReceipts(next);
  }, []);

  const refreshState = useCallback(async () => {
    const runtimeRequest = fetchRuntimeStatus().catch(async (runtimeError) => {
      const health = await fetchHealth();
      return {
        status: health.status,
        service: health.service,
        kernel: {
          connected: false,
          status: "unknown",
          error: runtimeError instanceof Error ? runtimeError.message : "运行状态不可用"
        }
      } satisfies RuntimeStatusResponse;
    });
    const [runtimeResult, uiResult] = await Promise.allSettled([runtimeRequest, fetchUIState()]);
    if (runtimeResult.status === "fulfilled") {
      setConnection("ready");
      setRuntimeStatus(runtimeResult.value);
      // WEBUI-SESSION-SEMANTICS-1：PlanBar 快照按 active session 绑定——全局
      // task_trajectory 投影的 conversation_id 与当前会话不符（note 会话/其他流
      // 的任务在跑）时即时清空，异会话的执行轨迹不得渲染到本会话输入框上方。
      setTaskTrajectoryState((current) =>
        reduceTaskTrajectoryForConversation(
          current,
          "task_trajectory" in runtimeResult.value ? runtimeResult.value.task_trajectory : null,
          conversationID
        )
      );
      setHealthLabel(runtimeResult.value.service ?? "VitAgent");
      // AB-JUDGMENT-CARD-1：成功只清运行时自己写入的错误——在飞轮询迟到的
      // 成功不得抹掉判定/试听等交互面刚设置的错误（挂载期竞态 E2E J1 实证）。
      setError((current) => (current === runtimeOwnedErrorRef.current ? "" : current));
      runtimeOwnedErrorRef.current = "";
    } else {
      setConnection("offline");
      setRuntimeStatus(null);
      setHealthLabel("未连接");
      const runtimeError = runtimeResult.reason instanceof Error ? runtimeResult.reason.message : "Agent 未连接";
      runtimeOwnedErrorRef.current = runtimeError;
      setError(runtimeError);
    }
    if (uiResult.status === "fulfilled") {
      setUIState(applyMacroValueOverridesToUIState(uiResult.value, macroValueOverridesRef.current));
    }
    // WEBUI-SESSION-SEMANTICS-1：会话切换后下一次轮询立即按新会话裁决快照归属
    //（依赖缺位会让异会话快照多活一个 8s 拍）。
  }, [conversationID]);

  useEffect(() => {
    void refreshState();
    const timer = window.setInterval(() => void refreshState(), 8000);
    return () => window.clearInterval(timer);
  }, [refreshState]);

  useEffect(() => {
    const restored = uiState?.authority_mode ?? runtimeStatus?.authority_mode;
    if (restored === "manual_confirmation" || restored === "full_project_access") {
      setAuthorityModeState(restored);
    }
  }, [runtimeStatus?.authority_mode, uiState?.authority_mode]);

  useEffect(() => {
    agentEventSeqRef.current = 0;
    setTrajectoryState(emptyTrajectoryState());
    setTurnEventMeta(emptyTurnEventMetaMap());
    setRoundSteps(emptyRoundStepMap());
    setTaskTrajectoryState(emptyTaskTrajectoryState());
    setAuditionState(emptyAuditionState());
    setAgentEventPolling(true);
    // TRAJ-IMPL-3：换会话即清台账视图（分桶键含 conversationID+scope，不会串桶；
    // 清空只为不在新会话里闪现上一会话的回执行）。
    turnReceiptsRef.current = [];
    setTurnReceipts([]);
  }, [conversationID]);

  // FIX-BUCKET-SAVE-RACE-1：restore 必须先于同桶首次 save——依赖含 messages。
  // reload 时 scope 物化提交把图消息同步 set 进流（scopedConversationRef 由同
  // 提交里后置的 scope 解析效应钉住），save 效应（依赖含 messages）下一提交即
  // 首跑；本效应原依赖只有 [conversationID, uiState]，URL 锚定会话两者都未变时
  // 要等下一轮询拍（~8s）才首跑——先到的 save 把仅存本地桶的末轮消息整桶覆写
  // （结构性丢失；mini_repro 反证：无图消息无覆写，restore 正常合并回屏）。本
  // 效应声明在 save 之前：依赖加 messages 后两者在同一提交重跑，restore 先读桶
  // 并布防 skipNextStoredMessageSaveRef，save 让位（既有 skip 机制），合并流再
  // 下一提交落盘。restoredMessageScopeRef 仍保证每桶键只真读一次，稳态零变化。
  useEffect(() => {
    const scope = historyScopeKeyFromUIState(uiState);
    if (!scope) {
      return;
    }
    if (scopedConversationRef.current !== scopedConversationRuntimeKey(scope, conversationID)) {
      return;
    }
    const restoreKey = conversationMessagesStorageKey(conversationID, scope);
    if (restoredMessageScopeRef.current === restoreKey) {
      return;
    }
    restoredMessageScopeRef.current = restoreKey;
    const storedMessages = stampConsumedInteractionActions(loadStoredConversationMessages(conversationID, scope), loadConsumedInteractionIDs());
    if (storedMessages.length === 0) {
      return;
    }
    skipNextStoredMessageSaveRef.current = restoreKey;
    setMessages((current) => mergeRestoredChatMessages(current, storedMessages));
  }, [conversationID, messages, uiState]);

  useEffect(() => {
    const scope = historyScopeKeyFromUIState(uiState);
    if (!scope) {
      return;
    }
    if (scopedConversationRef.current !== scopedConversationRuntimeKey(scope, conversationID)) {
      return;
    }
    const storageKey = conversationMessagesStorageKey(conversationID, scope);
    if (skipNextStoredMessageSaveRef.current === storageKey) {
      skipNextStoredMessageSaveRef.current = "";
      return;
    }
    if (!hasMeaningfulChatMessages(messages)) {
      return;
    }
    saveStoredConversationMessages(conversationID, scope, messages);
  }, [conversationID, messages, uiState]);

  const currentGoal = asRecord(uiState?.goal ?? runtimeStatus?.goal);
  const currentGoalStatus = textValue(currentGoal.status, "").toLowerCase();
  const liveContinuations = Array.isArray(runtimeStatus?.continuations) ? runtimeStatus.continuations : [];
  const agentTurnRunning =
    isAgentTurnRunning(currentGoalStatus, isSending) || continuationChainLive(liveContinuations, currentGoalStatus);
  // B9 症1：free-state chat 链不进 goal/continuations 投影——轮询忙态补「轨迹
  // 回合开放」位（壳节点自 trajectory.turn.started 开至终态事件），链执行期
  // 轮询不再休眠（mtwwegtp 取证：63s 空窗+终局一次性补渲染的根因①）。
  const trajectoryLive = hasLiveTrajectoryTurn(trajectoryState);
  // WEBUI-SESSION-SEMANTICS-1：PlanBar 链活/兜底的会话归属门控——全局 goal running
  // 只有在属主快照在位（归约已按 conversation_id 过滤）或本会话事件流轨迹回合开放
  // 时才视为"本会话有活链"；异会话任务在跑不再点亮输入框上方的规划条。
  const planBarOwnLive = trajectoryLive || (agentTurnRunning && taskTrajectoryState.snapshot !== null);

  useEffect(() => {
    if (!agentEventPolling) {
      return;
    }
    let cancelled = false;
    const idleGate = createAgentEventPollIdleGate();
    const poll = async () => {
      // GUI-F5：忙态（发送/动作响应/试听等待/链活/轨迹回合开放）任一为真时空轮询
      // 不累积空闲拍，链活期终局结果不漏取；全部空闲才按 4 拍（连续失败 3 拍）休眠。
      const pollBusy = agentEventPollBusy({ isSending, respondingActionID, auditionWaiting, agentTurnRunning, trajectoryLive });
      try {
        const requestedSince = agentEventSeqRef.current;
        const response = await fetchAgentEvents(conversationID, requestedSince, 120);
        if (cancelled) {
          return;
        }
        const events = (response.events ?? []).filter((event) => Number(event.seq) > agentEventSeqRef.current);
        if (typeof response.next_seq === "number") {
          agentEventSeqRef.current = Math.max(agentEventSeqRef.current, response.next_seq);
        } else {
          events.forEach((event) => {
            agentEventSeqRef.current = Math.max(agentEventSeqRef.current, Number(event.seq) || 0);
          });
        }
        if (events.length > 0) {
          idleGate.markActive();
          // F3 面②：server 权威撤卡事件（AGENT-F5）入持久已消费台账——重载后
          // 水合过滤不依赖本浏览器是否亲手应答过。终局投递事件（scheduler_chain
          // 三型）到达即刷新 runtime status，忙态（Stop Turn 锁）数秒内退场，
          // 不再等 8s 周期轮；终局消息入流仍走下方 GUI-F7 路径。
          const resolvedInteractionIDs = resolvedInteractionIdsFromEvents(events);
          if (resolvedInteractionIDs.length > 0) {
            recordConsumedInteractions(resolvedInteractionIDs);
            // MSG-REVIVE-1：台账只在水合边界盖章——裸开（storage 全灭）时水合
            // 先于回放到达，快照形态（waiting_for_user）已渲染成可交互卡，
            // 台账补齐后若不重盖已渲染流，已 resolve 的交互会复活成死卡
            //（点击报「交互已过期」）。resolved 事件是服务端权威撤卡信号，
            // 盖章幂等且未命中零拷贝，对正常形态无行为变化。
            setMessages((current) => stampConsumedInteractionActions(current, resolvedInteractionIDs));
          }
          // FIX-CONFIRM-CARD-1 ②：回合真终结（完成/失败/停止，waiting_continue
          // 等驻留态除外）时，该回合未应答的交互卡立即转入不可交互终态——
          // server 在无应答过期路径上不补发 interaction.resolved（4022 只在
          // 重复提交时兜底），呈现面按回合终结信号自行收卡，不再残留可点击
          // 的"过期"卡。
          const terminalTurnIDs = terminalTurnIDsFromEvents(events);
          if (terminalTurnIDs.length > 0) {
            setMessages((current) => settleTerminatedTurnInteractions(current, terminalTurnIDs));
          }
          if (hasChainTerminalDeliveryEvent(events)) {
            void refreshState();
          }
          if (shouldDebugAgentEvents(events)) {
            debugConfirmation("agent-events-polled", {
              conversation_id: conversationID,
              requested_since: requestedSince,
              next_seq: response.next_seq,
              is_sending: isSending,
              responding_action_id: respondingActionID,
              agent_turn_running: agentTurnRunning,
              events: events.map(summarizeAgentEventForConfirmation)
            });
          }
          setActivities((current) => reduceAgentEventActivities(
            current,
            events,
            (agentEvent) => chatMessageFromAgentEvent(agentEvent, mode)
          ));
          setTrajectoryState((current) => reduceTrajectoryEvents(current, events));
          setTurnEventMeta((current) => reduceTurnEventMeta(current, events));
          setRoundSteps((current) => reduceRoundSteps(current, events));
          setAuditionState((current) => reduceAuditionEvents(current, events));
          // GUI-F7：scheduler_chain 终局事件的回复直接入 messages（正式气泡）。
          // 不能走 activities——归约器对 turn.completed 只清场不产消息，且活动
          // 在回合结束后会被清退（2026-09-05 手测：终局回复整条丢失的根因）。
          const chainMessages = chainResultMessagesFromEvents(events, mode, chatMessageFromAgentEvent);
          if (chainMessages.length > 0) {
            setMessages((current) => appendChainResultMessages(current, chainMessages));
          }
          // SETTLE-DELIVER-1 症状 A：判定结算确认（judgment.settled +
          // settlement_reply）同 GUI-F7 语义直接入 messages——活动面不承载正式
          // 回复，刷新后由持久化 vit 节点水合接管（logical_message_id 同键合并）。
          const settleMessages = settlementMessagesFromEvents(events, mode, chatMessageFromAgentEvent);
          if (settleMessages.length > 0) {
            setMessages((current) => appendChainResultMessages(current, settleMessages));
          }
        } else if (idleGate.tickIdle(pollBusy)) {
          setAgentEventPolling(false);
        }
      } catch {
        if (!cancelled && idleGate.tickError(pollBusy)) {
          setAgentEventPolling(false);
        }
      }
    };
    void poll();
    // 发送等待期收紧到 250ms，让 turn.started 真块尽快接管乐观占位；链活期（GUI-F5）与空闲同为 500ms
    const timer = window.setInterval(() => void poll(), isSending ? 250 : 500);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [agentEventPolling, agentTurnRunning, auditionWaiting, conversationID, isSending, mode, refreshState, respondingActionID, trajectoryLive]);

  // GUI-F5：轮询被空闲休眠停掉后，链中途再活（续跑切片醒来）必须重新拉起轮询，
  // 否则终局结果落在服务端无人来取（2026-09-04 会话 webui_mtmwwfax 的漏取形态）。
  // B9：轨迹回合开放（含 free-state chat 链形态）同样要拉起。
  useEffect(() => {
    if (!agentTurnRunning && !trajectoryLive) {
      return;
    }
    setAgentEventPolling(true);
  }, [agentTurnRunning, trajectoryLive]);

  // CONTRACT-3（2026-09-05）：会话内 scope 演进 ≠ 工程切换。
  // 旧实现把 state_dir/worktree/branch 等晚物化字段编进 scope 键，链首批
  // checkpoint 落地后键中途变化被当作工程切换：换会话 id、事件 seq 归零、
  // 消息流重置为问候语（终审复现）。现在由 classifyHistoryScopeChange 判定：
  // 真工作区切换（工作区坐标变化/同路径 uuid 更换）才换会话+清流；同会话
  // 键演进保留消息流与事件 seq，重新锚定存储桶并迁移已存消息。
  useEffect(() => {
    const nextParts = historyScopePartsFromUIState(uiState);
    const nextScope = historyScopeKeyFromParts(nextParts);
    if (!nextScope || pausedHistoryScopeRef.current === nextScope) {
      return;
    }
    const historyMessages = stampConsumedInteractionActions(historyMessagesFromUIState(uiState), loadConsumedInteractionIDs());
    const changeKind = classifyHistoryScopeChange({
      previousKey: historyScopeRef.current,
      previousConcretePath: historyScopeConcreteRef.current,
      previousUUID: historyScopeUUIDRef.current,
      nextParts
    });
    const nextConcrete = concreteWorkspacePath(nextParts);
    if (nextConcrete) {
      historyScopeConcreteRef.current = nextConcrete;
    }
    if (nextParts.projectUUID) {
      historyScopeUUIDRef.current = nextParts.projectUUID;
    }
    if (changeKind === "switch") {
      setActivities([]);
    }
    if (changeKind === "switch" || changeKind === "initial") {
      const nextConversationID = resolveScopedConversationID({
        urlConversationID: conversationIDFromURL(),
        storedScopedConversationID: loadStoredScopedConversationID(nextScope),
        serverConversationID: serverConversationIDFromContinuations(runtimeStatus?.continuations, nextParts),
        freshConversationID: createConversationID()
      });
      historyScopeRef.current = nextScope;
      saveStoredScopedConversationID(nextScope, nextConversationID);
      // WEBUI-SESSION-SEMANTICS-1：主对话流出生即钉（裁定 1：每次拉栈自动建、同工程
      // 重开沿旧名）——scope 首次物化解析出的会话就是主对话流；已钉过则沿旧身份。
      if (!loadSessionFlowMainID(nextScope)) {
        saveSessionFlowMainID(nextScope, nextConversationID);
      }
      setMainConversationID(loadSessionFlowMainID(nextScope));
      scopedConversationRef.current = scopedConversationRuntimeKey(nextScope, nextConversationID);
      restoredMessageScopeRef.current = "";
      agentEventSeqRef.current = 0;
      setConversationID((currentConversationID) =>
        currentConversationID === nextConversationID ? currentConversationID : nextConversationID
      );
      setMessages((current) => {
        debugConfirmation("history-sync", {
          changeKind,
          current: summarizeMessagesForConfirmation(current),
          incoming: summarizeMessagesForConfirmation(historyMessages)
        });
        return resolveHistorySyncMessages({ changeKind, current, historyMessages });
      });
      return;
    }
    if (changeKind === "evolution") {
      // B9 症4（刷新后轨迹整体消失）：首拍 scope 未物化时 initial 分支已造新
      // 随机会话 id，此演进拍回读真实 scope 的锚定存档 id——当前流尚无有效
      // 消息（未劫持现役对话）即采纳，让 [conversationID] 效应重置游标并回放
      // /agent/events 重建轨迹块。终局消息的服务端图水合不受会话 id 影响，
      // 这正是「消息在、轨迹无」的分裂成因。
      //
      // CONV-ID-BOOT-1 勘察结论（2026-09-14，卡内要求查明本路径为何没兜住
      // 裸启动形态）：采纳守卫的「当前流无有效消息」被开机问候误触——
      // messagesOrIntro 克隆问候的 id 是 intro_<ts>_<rand>，而
      // hasMeaningfulChatMessages 旧排除只匹配整串 "intro"，问候被计成有效
      // 消息，守卫在裸启动上永远拒绝采纳，fall-through 迁移随即覆写真实
      // scope 桶（修复在 hasMeaningfulChatMessages + 迁移/写回双守卫）。
      //
      // REFRESH-VANISH-2（2026-09-15）：锚定候选在本地存档之外增加服务端兜底
      // 档——空 storage 裸开（webview 面板销毁重建丢 localStorage）时本地锚定
      // 缺失，改用 runtimeStatus.continuations 投影里该 scope 的权威会话身份；
      // 采纳后 [conversationID] 效应重置游标并以 since=0 全量回放 /agent/events，
      // 轨迹块/A-B 卡重建（本地锚定在位时候选=存档值，storage 正常形态零变化）。
      const anchorScopedConversationID = resolveScopedConversationID({
        urlConversationID: "",
        storedScopedConversationID: loadStoredScopedConversationID(nextScope),
        serverConversationID: serverConversationIDFromContinuations(runtimeStatus?.continuations, nextParts),
        freshConversationID: ""
      });
      if (
        shouldAdoptStoredConversationOnScopeEvolution({
          storedConversationID: anchorScopedConversationID,
          currentConversationID: conversationID,
          hasMeaningfulMessages: hasMeaningfulChatMessages(messages)
        })
      ) {
        historyScopeRef.current = nextScope;
        agentEventSeqRef.current = 0;
        saveStoredScopedConversationID(nextScope, anchorScopedConversationID);
        // WEBUI-SESSION-SEMANTICS-1：采纳的锚定会话就是本 scope 的主对话流（裸开/
        // 服务端兜底形态）——主身份缺位时补钉，命名随侧边栏幂等登记生效。
        if (!loadSessionFlowMainID(nextScope)) {
          saveSessionFlowMainID(nextScope, anchorScopedConversationID);
        }
        setMainConversationID(loadSessionFlowMainID(nextScope));
        scopedConversationRef.current = scopedConversationRuntimeKey(nextScope, anchorScopedConversationID);
        restoredMessageScopeRef.current = "";
        // MSG-REVIVE-1：采纳服务端会话身份时工程历史消息同批并入——裸开
        //（storage 全灭）形态此路径是消息的唯一恢复入口：首拍 scope 未物化
        // 时 initial 已在 unsaved 桶造了空流，此处跳过合并就是「轨迹/A-B 卡
        // 重建（回放）、消息全灭（桶+回放面都无正文）」的分裂形态。合并用
        // evolution 保流语义，不劫持现役对话（采纳守卫已保证当前流无有效
        // 消息）。
        setMessages((current) => resolveHistorySyncMessages({ changeKind: "evolution", current, historyMessages }));
        setConversationID(anchorScopedConversationID);
        return;
      }
      const previousScope = historyScopeRef.current;
      historyScopeRef.current = nextScope;
      scopedConversationRef.current = scopedConversationRuntimeKey(nextScope, conversationID);
      migrateStoredConversationScope(previousScope, nextScope, conversationID);
      // WEBUI-SESSION-SEMANTICS-1：scope 演进迁移路径上，被迁移的当前会话继承为
      // 新 scope 的主对话流（未保存→保存等演进形态主身份不落空）。
      if (!loadSessionFlowMainID(nextScope)) {
        saveSessionFlowMainID(nextScope, conversationID);
      }
      setMainConversationID(loadSessionFlowMainID(nextScope));
      restoredMessageScopeRef.current = "";
      setMessages((current) => {
        debugConfirmation("history-sync", {
          changeKind,
          previousScope,
          current: summarizeMessagesForConfirmation(current),
          incoming: summarizeMessagesForConfirmation(historyMessages)
        });
        return resolveHistorySyncMessages({ changeKind, current, historyMessages });
      });
      return;
    }
    setMessages((current) => {
      const hasPendingInteraction = hasPendingComposerInteraction(current);
      const baseMessages = hasPendingInteraction ? current : removeDisposableConfirmationPromptMessages(current);
      debugConfirmation("history-sync", {
        changeKind,
        current: summarizeMessagesForConfirmation(current),
        current_after_prompt_filter: summarizeMessagesForConfirmation(baseMessages),
        incoming: summarizeMessagesForConfirmation(historyMessages)
      });
      return resolveHistorySyncMessages({ changeKind, current, historyMessages });
    });
    // REFRESH-VANISH-2：服务端兜底身份来自 runtimeStatus（与 uiState 同一
    // refreshState 的 allSettled 提交到达）；入依赖保证晚到的一拍也能取到。
  }, [conversationID, runtimeStatus, uiState]);

  // TRAJ-IMPL-3（设计 §2.3）：**水合读取**终局回执行台账。
  //
  // 位置在 scope 解析效应**之后**：同一个提交里 scope 效应先把 scopedConversationRef
  // 钉到「本 scope × 本会话」，本效应紧接着就能读到台账——否则刷新后要多等一拍
  // （最多 8s 的 uiState 轮询）才把回执行画回来，而「刷新后立刻还在」正是本卡的验收面。
  //
  // 时序与消息存档同一条（scope 就绪、且当前会话就是该 scope 锚定的会话时读一次）：
  // 换会话/换工作区都会换键，因此自然重读，不需要额外的清理分支。台账缺失
  // （换浏览器/清缓存/从未写过）时读出空表——**不编造回执行**，正文气泡照旧
  // （设计 §2.3 边界声明）。
  useEffect(() => {
    const scope = historyScopeKeyFromUIState(uiState);
    if (!scope) {
      return;
    }
    if (scopedConversationRef.current !== scopedConversationRuntimeKey(scope, conversationID)) {
      return;
    }
    const restoreKey = turnReceiptsStorageKey(conversationID, scope);
    if (restoredReceiptScopeRef.current === restoreKey) {
      return;
    }
    restoredReceiptScopeRef.current = restoreKey;
    applyTurnReceipts(loadTurnReceipts(conversationID, scope));
  }, [applyTurnReceipts, conversationID, uiState]);

  // TRAJ-IMPL-3（设计 §2.3 + §4）：**终态收口才写台账**。收口判据复用 TRAJ-IMPL-2 的
  // 回合收口规则（roundTurnStatus：waiting_* 切片边界算没结束 → 不写；live 一律不写；
  // settle_slice 等 M12 隐藏回合同样不写，噪音不得经台账在刷新后复活）。
  //
  // 写侧走 saveTurnReceipts（读—并—写），权威副本是**落盘台账**而不是内存视图：
  // 刷新后的第一笔收口若只按内存合并，就会把上一页写下、活态已不存在的回执行整桶冲掉
  // ——而「那些行还在」正是台账存在的理由（首轮渲染面门实测命中：E1 水合相位红）。
  // 效应幂等：同一回合重复收口只刷新同一行（同键去重），无变化时不碰 localStorage。
  useEffect(() => {
    const scope = historyScopeKeyFromUIState(uiState);
    if (!scope || !conversationID) {
      return;
    }
    if (scopedConversationRef.current !== scopedConversationRuntimeKey(scope, conversationID)) {
      return;
    }
    const closed = collectTerminalTurnReceipts({ trajectory: trajectoryState, turnEventMeta, roundSteps });
    if (closed.length === 0) {
      return;
    }
    const merged = saveTurnReceipts(conversationID, scope, closed);
    if (turnReceiptsEqual(merged, turnReceiptsRef.current)) {
      return;
    }
    applyTurnReceipts(merged);
  }, [applyTurnReceipts, conversationID, roundSteps, trajectoryState, turnEventMeta, uiState]);

  useEffect(() => {
    const nextScope = mediaScopeKeyFromUIState(uiState);
    if (!nextScope) {
      return;
    }
    if (mediaScopeRef.current && mediaScopeRef.current !== nextScope) {
      setPendingArtifacts([]);
      setSelectedArtifactID("");
    }
    mediaScopeRef.current = nextScope;
  }, [uiState]);

  const artifacts = useMemo(() => {
    return mergeArtifacts(pendingArtifacts, uiState?.artifacts ?? []);
  }, [pendingArtifacts, uiState?.artifacts]);

  const selectedArtifact = useMemo(() => {
    return artifacts.find((artifact) => artifact.id === selectedArtifactID) ?? artifacts[0] ?? null;
  }, [artifacts, selectedArtifactID]);
  const conversationTitle = useMemo(() => currentConversationTitle(uiState, messages), [messages, uiState]);
  const rightPanelWidth = rightPanelWidths[activeTab] ?? defaultRightPanelWidth;
  const workspaceStyle = {
    "--right-panel-width": `${rightPanelWidth}px`
  } as CSSProperties;
  const composerInteraction = useMemo(
    () => latestComposerInteraction(messages, dismissedInteractionIDs) ?? pendingInteractionFromContinuations(runtimeStatus?.continuations ?? [], conversationID),
    [messages, dismissedInteractionIDs, runtimeStatus, conversationID]
  );
  const composerInteractionID = composerInteraction ? actionRenderID(composerInteraction) : "";

  useEffect(() => {
    const collapseForViewport = () => {
      if (window.innerWidth < 1120) {
        setRightPanelOpen(false);
      }
    };
    collapseForViewport();
    window.addEventListener("resize", collapseForViewport);
    return () => window.removeEventListener("resize", collapseForViewport);
  }, []);

  useEffect(() => {
    const measureBottomInset = () => {
      const composerHeight = composerRef.current?.getBoundingClientRect().height ?? 0;
      // PLANBAR-1：停靠列（规划条 + composer）才是底部浮层整体，取两者的
      // 最大值——栏出现/展开/收起都会触发 ResizeObserver，预留随之一致。
      const dockHeight = composerDockRef.current?.getBoundingClientRect().height ?? 0;
      const noticeHeight = noticeRef.current?.getBoundingClientRect().height ?? 0;
      const nextInset = Math.max(132, Math.ceil(Math.max(dockHeight, composerHeight) + noticeHeight + 36));
      setMessageBottomInset((current) => (Math.abs(current - nextInset) > 1 ? nextInset : current));
    };
    measureBottomInset();
    const observer = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(measureBottomInset);
    if (composerDockRef.current) {
      observer?.observe(composerDockRef.current);
    }
    if (composerRef.current) {
      observer?.observe(composerRef.current);
    }
    if (noticeRef.current) {
      observer?.observe(noticeRef.current);
    }
    const frameID = window.requestAnimationFrame(measureBottomInset);
    window.addEventListener("resize", measureBottomInset);
    return () => {
      window.cancelAnimationFrame(frameID);
      window.removeEventListener("resize", measureBottomInset);
      observer?.disconnect();
    };
  }, [composerInteractionID, error, pendingArtifacts.length, pendingMacroRefs.length, isSending, isUploading]);

  useEffect(() => {
    debugConfirmation("composer-selection", {
      selected: summarizeActionForConfirmation(composerInteraction),
      messages: summarizeMessagesForConfirmation(messages)
    });
  }, [composerInteractionID, messages]);

  // FIX-CONFIRM-CARD-1 ③ 呈现面②：composer 投影（后台驻留续跑/刷新后仍待
  // 应答的确认闸）在完全档下同样不出前置卡——命中直执谓词即自动应答，投影
  // 随 continuation 退役自愈消失。按交互 id 去重防重放；直执失败时投影卡
  // 保留（server 侧仍 pending），用户可手动确认。
  useEffect(() => {
    if (authorityMode !== "full_project_access" || !composerInteraction) {
      return;
    }
    if (!isFullAccessDirectExecutionInteraction(composerInteraction)) {
      return;
    }
    void fireFullAccessDirectExecution(composerInteraction);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- fireFullAccessDirectExecution 每渲染重建但仅按 id 幂等触发
  }, [composerInteractionID, authorityMode]);

  useEffect(() => {
    const scope = historyScopeKeyFromUIState(uiState);
    if (!scope) {
      return;
    }
    if (scopedConversationRef.current !== scopedConversationRuntimeKey(scope, conversationID)) {
      return;
    }
    // CONV-ID-BOOT-1（2026-09-14）：运行时写回不得覆写指向其他会话的锚定。
    // 显式「新会话」由 handleNewConversation 直接写入（用户意图优先），不经
    // 此路径；这里只做「当前锚定 == 运行会话或为空」的幂等同步。
    const anchoredConversationID = loadStoredScopedConversationID(scope);
    if (anchoredConversationID && anchoredConversationID !== conversationID) {
      return;
    }
    saveStoredScopedConversationID(scope, conversationID);
  }, [conversationID, uiState]);

  useEffect(() => {
    const handleStorage = (event: StorageEvent) => {
      if (event.key === attachedArtifactStorageKey && event.newValue && !isWorkbenchSurface) {
        try {
          const artifact = JSON.parse(event.newValue) as ArtifactSummary;
          if (artifact.id) {
            setPendingArtifacts((current) => mergeArtifacts(current, [artifact]));
          }
        } catch {
          // Ignore malformed cross-window state.
        }
      }
    };
    window.addEventListener("storage", handleStorage);
    return () => window.removeEventListener("storage", handleStorage);
  }, [isWorkbenchSurface]);

  const attachArtifact = useCallback((artifact: ArtifactSummary) => {
    setPendingArtifacts((current) => mergeArtifacts(current, [artifact]));
    try {
      window.localStorage.setItem(attachedArtifactStorageKey, JSON.stringify({ ...artifact, attached_at: Date.now() }));
    } catch {
      // Clipboard-style handoff is best effort.
    }
  }, []);

  const handleUpload = async (files: FileList | File[]) => {
    if (files.length === 0) {
      return;
    }
    setIsUploading(true);
    setError("");
    try {
      const result = await uploadArtifacts(files, uploadMetadata(conversationID, uiState));
      const uploaded = result.artifacts ?? [];
      setPendingArtifacts((current) => mergeArtifacts(current, uploaded));
      if (uploaded[0]) {
        setSelectedArtifactID(uploaded[0].id);
        setActiveTab("media");
      }
      await refreshState();
    } catch (uploadError) {
      setError(uploadError instanceof Error ? uploadError.message : "上传失败");
    } finally {
      setIsUploading(false);
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    }
  };

  const handleSend = async (event?: FormEvent, textOverride?: string) => {
    event?.preventDefault();
    const messageText = (textOverride ?? input).trim();
    if (isSending) {
      return;
    }
    if (!messageText && pendingArtifacts.length === 0 && pendingMacroRefs.length === 0) {
      return;
    }
    const attached = pendingArtifacts;
    const macroRefs = pendingMacroRefs;
    const userMessage: ChatMessage = {
      id: uniqueID("msg"),
      role: "user",
      content: messageText || (macroRefs.length > 0 ? "已引用宏控制" : "已附加资料"),
      mode,
      artifacts: attached,
      actions: macroRefs.map((macro) => macroControlCardAction(macro, "reference")),
      createdAt: Date.now(),
      status: "sent",
      lifecycle: "durable",
      persistence: "project_history",
      message_kind: "user"
    };
    setActivities([]);
    setMessages((current) => [...current, userMessage]);
    setInput("");
    setIsSending(true);
    setAgentEventPolling(true);
    setError("");
    try {
      const response = await sendChat({
        conversation_id: conversationID,
        message: messageText,
        artifact_refs: attached.map((artifact) => artifact.id),
        authority_mode: authorityMode,
        context: buildChatContext(mode, uiState, attached, macroRefs, authorityMode)
      });
      debugConfirmation("chat-response", summarizeChatResponseForConfirmation(response));
      postAgentMutationsFromChatResponse(response, "chat");
      // FIX-CONFIRM-CARD-1 ③：完全档下 RiskConfirm 确认闸不出前置卡——入流
      // 消息先剥除直执目标（一次性确认提示改写为直执通告），应答在流合并
      // 后执行，回执随 runFullAccessDirectExecution 入流。
      const directTargets = authorityMode === "full_project_access" ? fullAccessDirectExecutionTargets(response) : [];
      let assistantMessage = assistantMessageFromResponse(response);
      if (directTargets.length > 0) {
        assistantMessage = stripFullAccessDirectExecutionActions(assistantMessage, directTargets);
      }
      setActivities((current) => dismissActivitiesForTurn(current, response.turn_id || response.run_id || response.goal_id || ""));
      debugConfirmation("assistant-message", summarizeMessageForConfirmation(assistantMessage));
      setMessages((current) => mergeAssistantMessageIntoChat(current, assistantMessage));
      setPendingArtifacts([]);
      setPendingMacroRefs([]);
      if (response.side_panel_request?.artifact_id) {
        setSelectedArtifactID(response.side_panel_request.artifact_id);
        setActiveTab("media");
      }
      await refreshState();
      setMessages((current) => mergeAssistantMessageIntoChat(current, assistantMessage));
      for (const target of directTargets) {
        await fireFullAccessDirectExecution(target);
      }
    } catch (chatError) {
      const message = chatError instanceof Error ? chatError.message : "发送失败";
      setError(message);
      setActivities([]);
      setMessages((current) => [
        ...current,
        {
          id: uniqueID("err"),
          role: "system",
          content: message,
          createdAt: Date.now(),
          status: "error",
          lifecycle: "durable",
          persistence: "local",
          message_kind: "error"
        }
      ]);
      setInput(messageText);
    } finally {
      setIsSending(false);
    }
  };

  const handleAuthorityModeChange = async (nextMode: AuthorityMode) => {
    if (nextMode === authorityMode || authorityBusy) return;
    setAuthorityBusy(true);
    setError("");
    try {
      const response = await setAuthorityMode(nextMode);
      setAuthorityModeState(response.authority_mode ?? nextMode);
      await refreshState();
    } catch (authorityError) {
      setError(authoritySwitchNotice({ ok: false, error: authorityError }) ?? "权限模式切换失败");
    } finally {
      setAuthorityBusy(false);
    }
  };

  const handleStopTurn = async () => {
    if (stopTurnBusy || !agentTurnRunning) return;
    setStopTurnBusy(true);
    setError("");
    try {
      const activeTurnID = textValue(taskTrajectoryState.snapshot?.run.current_turn_id, "") || (Object.values(trajectoryState.turns).find((turn) => !turn.stopped && turn.status !== "completed" && turn.status !== "failed") ?? Object.values(trajectoryState.turns)[0])?.id;
      await stopTurn({
        conversation_id: conversationID,
        goal_id: textValue(currentGoal.goal_id, ""),
        run_id: textValue(currentGoal.run_id, ""),
        turn_id: activeTurnID || textValue(currentGoal.run_id, ""),
        reason: "user_stop"
      });
      setAgentEventPolling(true);
      await refreshState();
    } catch (stopError) {
      setError(stopError instanceof Error ? stopError.message : "停止 Turn 失败");
    } finally {
      setStopTurnBusy(false);
    }
  };

  const removePendingArtifact = (id: string) => {
    setPendingArtifacts((current) => current.filter((artifact) => artifact.id !== id));
  };

  const attachMacroControl = useCallback((macro: MacroControl) => {
    setPendingMacroRefs((current) => mergeMacroControls(current, [macro]));
    setInput((current) => current || `@macro:${macroControlID(macro)} `);
  }, []);

  const removePendingMacroControl = (id: string) => {
    setPendingMacroRefs((current) => current.filter((macro) => macroControlID(macro) !== id));
  };

  const insertMacroControlCard = useCallback((macro: MacroControl) => {
    setMessages((current) => [
      ...current,
      durableMessage({
        id: uniqueID("macro_card"),
        role: "assistant",
        content: "已放入宏控制卡片。",
        mode,
        actions: [macroControlCardAction(macro, "live_card")],
        createdAt: Date.now(),
        status: "sent"
      }, { kind: "assistant", persistence: "local" })
    ]);
  }, [mode]);

  const patchMacroControlValue = useCallback((macro: MacroControl, value: number) => {
    const macroID = macroControlID(macro);
    if (!macroID) {
      return;
    }
    macroValueOverridesRef.current[macroID] = {
      value,
      macro: normalizeMacroControl(asRecord(macro)),
      expiresAt: Date.now() + 120000
    };
    setUIState((current) => current ? patchMacroControlValueInUIState(current, macroID, value, macro) : current);
  }, []);

  const patchMacroControlName = useCallback((macro: MacroControl, name: string) => {
    const macroID = macroControlID(macro);
    if (!macroID) {
      return;
    }
    setUIState((current) => current ? patchMacroControlNameInUIState(current, macroID, name) : current);
    setPendingMacroRefs((current) => mergeMacroControls(current.map((item) => (
      macroControlID(item) === macroID ? { ...item, name } : item
    ))));
  }, []);

  const previewMacroControlValue = useCallback((macro: MacroControl, value: number) => {
    patchMacroControlValue(macro, value);
    postAgentMutations([macroControlValueMutation(macro, value, false, "webui_preview")], "macro_preview");
  }, [patchMacroControlValue]);

  const commitMacroControlValue = useCallback(async (macro: MacroControl, value: number) => {
    const macroID = macroControlID(macro);
    if (!macroID) {
      return;
    }
    patchMacroControlValue(macro, value);
    const pendingID = `macro:${macroID}`;
    setRespondingActionID(pendingID);
    try {
      const response = await setMacroControlValue({
        macro_id: macroID,
        value,
        macro,
        commit: true,
        source: "ask_vit_webui"
      });
      const posted = postAgentMutationsFromInvokeResponse(response, "macro_commit");
      if (!posted) {
        postAgentMutations([macroControlValueMutation(macro, value, true, "webui_commit")], "macro_commit");
      }
      await refreshState();
    } catch (macroError) {
      setError(macroError instanceof Error ? macroError.message : "宏控制提交失败");
    } finally {
      setRespondingActionID("");
    }
  }, [patchMacroControlValue, refreshState]);

  const renameMacroControlLabel = useCallback(async (macro: MacroControl, name: string) => {
    const macroID = macroControlID(macro);
    const cleanName = name.trim();
    const previousName = macroControlLabel(macro);
    if (!macroID || !cleanName || cleanName === previousName) {
      return;
    }
    const renamedMacro = { ...macro, name: cleanName };
    patchMacroControlName(renamedMacro, cleanName);
    setRespondingActionID(`macro:rename:${macroID}`);
    try {
      const response = await renameMacroControl({
        macro_id: macroID,
        name: cleanName,
        macro: renamedMacro,
        source: "ask_vit_webui"
      });
      const posted = postAgentMutationsFromInvokeResponse(response, "macro_rename");
      if (!posted) {
        postAgentMutations([macroControlRenameMutation(renamedMacro, cleanName, "webui_rename")], "macro_rename");
      }
      await refreshState();
    } catch (macroError) {
      patchMacroControlName(macro, previousName);
      setError(macroError instanceof Error ? macroError.message : "Macro rename failed");
    } finally {
      setRespondingActionID("");
    }
  }, [patchMacroControlName, refreshState]);

  const handleRenameArtifact = async (id: string, title: string) => {
    const result = await renameArtifact(id, title);
    const updated = result.artifact;
    if (updated) {
      setPendingArtifacts((current) => mergeArtifacts(current.filter((artifact) => artifact.id !== id), [updated]));
    }
    await refreshState();
  };

  const handleDeleteArtifact = async (id: string) => {
    await deleteArtifact(id);
    setPendingArtifacts((current) => current.filter((artifact) => artifact.id !== id));
    if (selectedArtifactID === id) {
      setSelectedArtifactID("");
    }
    await refreshState();
  };

  const handleNewConversation = () => {
    const currentScope = historyScopeKeyFromUIState(uiState);
    if (currentScope) {
      pausedHistoryScopeRef.current = currentScope;
      historyScopeRef.current = currentScope;
    }
    const nextConversationID = `webui_${Date.now().toString(36)}`;
    setConversationID(nextConversationID);
    if (currentScope) {
      scopedConversationRef.current = scopedConversationRuntimeKey(currentScope, nextConversationID);
      saveStoredScopedConversationID(currentScope, nextConversationID);
    }
    setMessages([{ ...initialMessage, id: uniqueID("intro"), createdAt: Date.now() }]);
    setActivities([]);
    setPendingArtifacts([]);
    setPendingMacroRefs([]);
    setDismissedInteractionIDs([]);
    setInput("");
    setError("");
  };

  // WEBUI-IA-REDESIGN-1：会话流侧边栏的切换通路——与 handleNewConversation 同语义
  //（scope 暂停+锚定改写+瞬态清空），目标会话的消息由既有恢复效应从本地桶水合、
  // 轨迹块由 [conversationID] 效应重放 /agent/events 重建。不建工作树/分支：
  // 会话键=conversation id，工程级深操作仍在历史界面。
  // VITNOTE-NOTESTREAM-2：切到 note 会话（note_ 前缀）时本地桶无消息（note 问答不经
  // webui 发送），改由 note_sessions 投影行种子消息——note 删除=归档可查（判据面：
  // 不入主流，但侧边栏可回看）。
  const handleSwitchConversation = (targetConversationID: string) => {
    const target = targetConversationID.trim();
    if (!target || target === conversationID) {
      return;
    }
    const currentScope = historyScopeKeyFromUIState(uiState);
    if (currentScope) {
      pausedHistoryScopeRef.current = currentScope;
      historyScopeRef.current = currentScope;
      scopedConversationRef.current = scopedConversationRuntimeKey(currentScope, target);
      saveStoredScopedConversationID(currentScope, target);
    }
    setConversationID(target);
    setMessages(noteSessionSeedMessages(target, runtimeStatus?.note_sessions));
    setActivities([]);
    setPendingArtifacts([]);
    setPendingMacroRefs([]);
    setDismissedInteractionIDs([]);
    setInput("");
    setError("");
  };

  // 侧边栏未命名会话的展示名推导：本地消息桶的首条用户消息（截断 40 字）。
  const sessionEntryTitleFor = useCallback((conversationID: string) => {
    const scope = historyScopeKeyFromUIState(uiState);
    if (!scope || !conversationID) {
      return "";
    }
    const cached = loadStoredConversationMessages(conversationID, scope);
    const firstUser = [...cached].find((message) => message.role === "user" && message.content.trim());
    return firstUser ? firstUser.content.replace(/\s+/g, " ").trim().slice(0, 40) : "";
  }, [uiState]);

  // WEBUI-SESSION-SEMANTICS-1（命名裁定 1）：主对话流默认名=「主对话流 · <工程名>」，
  // 工程名与状态栏同源（工程路径尾段；未保存工程沿 "Vit Project" 兜底）。
  const mainFlowTitle = useMemo(() => {
    const parts = historyScopePartsFromUIState(uiState);
    return mainConversationDefaultTitle(lastPathPart(parts.projectPath || parts.rootProjectPath || ""));
  }, [uiState]);

  const fetchLatestUIState = async (): Promise<AgentUIState | null> => {
    try {
      const latest = await fetchUIState();
      setUIState(latest);
      return latest;
    } catch {
      return uiState;
    }
  };

  const invokeDawAction = useCallback(async (
    tool: string,
    args: JsonRecord = {},
    source = "ask_vit_webui_daw_panel",
    confirmed = true
  ): Promise<AgentInvokeResponse> => {
    setError("");
    const response = await invokeAgent({
      tool,
      args,
      source,
      confirmed,
      context: buildChatContext(mode, uiState, [], macroControlsFromUIState(uiState), authorityMode)
    });
    postAgentMutationsFromInvokeResponse(response, source);
    if (response.status === "error") {
      const message = textValue(response.error, "DAW action failed");
      setError(message);
      throw new Error(message);
    }
    await refreshState();
    return response;
  }, [mode, refreshState, uiState]);

  const runTransportCommand = useCallback(async (tool: string, args: JsonRecord = {}) => {
    setTransportBusy(true);
    try {
      await invokeDawAction(tool, args, "ask_vit_webui_transport", true);
    } catch (transportError) {
      setError(transportError instanceof Error ? transportError.message : "Transport action failed");
    } finally {
      setTransportBusy(false);
    }
  }, [invokeDawAction]);

  const dismissInteractionCard = useCallback((interactionID: string, renderID: string, interaction?: JsonRecord) => {
    setDismissedInteractionIDs((current) => addInteractionDismissals(current, interactionID, renderID));
    setMessages((current) => {
      const next = removeInteractionActionFromMessages(current, interactionID, renderID, interaction);
      debugConfirmation("interaction-dismiss", {
        interaction_id: interactionID,
        render_id: renderID,
        is_confirmation: interaction ? isConfirmationAction(interaction) : false,
        before: summarizeMessagesForConfirmation(current),
        after: summarizeMessagesForConfirmation(next)
      });
      return next;
    });
  }, []);
  const handleInteractionAction = async (interaction: JsonRecord, action: JsonRecord, payload?: JsonRecord) => {
    const interactionID = textValue(interaction.id ?? interaction.interaction_id, "");
    const actionID = textValue(action.id ?? action.action_id ?? action.decision, "submit");
    if (!interactionID || !actionID || respondingActionID) {
      return;
    }
    const pendingID = interactionActionID(interactionID, actionID);
    const actionLabel = textValue(action.label ?? action.title ?? actionID, actionID);
    const transcriptText = isCapabilityProposalInteraction(interaction)
      ? proposalDecisionTranscript(actionID, actionLabel)
      : actionLabel;
    const renderID = actionRenderID(interaction);
    const processingMessageID = uniqueID("interaction_processing");
    const persistentMixBoard = isMixBoardAction(interaction);
    setRespondingActionID(pendingID);
    setAgentEventPolling(true);
    setError("");
    debugConfirmation("interaction-click", {
      interaction_id: interactionID,
      render_id: renderID,
      action_id: actionID,
      action_label: actionLabel,
      is_confirmation: isConfirmationAction(interaction),
      interaction: summarizeActionForConfirmation(interaction)
    });
    if (!persistentMixBoard) {
      dismissInteractionCard(interactionID, renderID, interaction);
      // F3 死卡守卫：点击即乐观记入持久已消费台账——56s 级慢响应在飞期间，
      // 8s 周期 refreshState 的历史水合不会把 waiting_for_user 快照复活成
      // 可交互卡（18:59 二次 approve 形态）。应答失败在 catch 里回滚。
      recordConsumedInteractions([interactionID, renderID]);
    }
    const progressMessage = processingChatMessage(processingMessageID, interactionProcessingText(interaction, actionID, actionLabel), mode);
    if (!persistentMixBoard) {
      setMessages((current) => [
        ...current,
        {
          id: uniqueID("interaction"),
          role: "user",
          content: transcriptText,
          mode,
          createdAt: Date.now(),
          status: "sent",
          lifecycle: "durable",
          persistence: "project_history",
          message_kind: "user"
        }
      ]);
      setActivities((current) => upsertActivity(current, progressMessage));
    }
    try {
      const decision = textValue(action.decision ?? actionID, actionID);
      const response = isSyntheticConfirmationInteraction(interaction) && !isCapabilityProposalInteraction(interaction)
        ? await confirmPlan({
            plan_id: textValue(interaction.plan_id ?? asRecord(interaction.payload).plan_id, ""),
            decision
          })
        : await respondInteraction({
            interaction_id: interactionID,
            action_id: actionID,
            decision,
            payload: payload ?? asRecord(action.value ?? action.payload)
          });
      debugConfirmation("interaction-response", summarizeChatResponseForConfirmation(response));
      postAgentMutationsFromChatResponse(response, "interaction");
      const assistantMessage = assistantMessageFromResponse(response);
      setActivities((current) => dismissActivityByID(
        dismissActivitiesForTurn(current, response.turn_id || response.run_id || response.goal_id || ""),
        processingMessageID
      ));
      debugConfirmation("assistant-message", summarizeMessageForConfirmation(assistantMessage));
      setMessages((current) => {
        const resolved = persistentMixBoard ? current : resolveInteractionInMessages(current, interactionID, actionID, response);
        const next = persistentMixBoard || messageMixBoardAction(assistantMessage)
          ? upsertPersistentMixBoardMessage(resolved, assistantMessage)
          : mergeAssistantMessageIntoChat(resolved, assistantMessage);
        debugConfirmation("interaction-response-replace", {
          interaction_id: interactionID,
          action_id: actionID,
          before: summarizeMessagesForConfirmation(current),
          after_resolve: summarizeMessagesForConfirmation(resolved),
          after_replace: summarizeMessagesForConfirmation(next)
        });
        return next;
      });
      if (response.side_panel_request?.artifact_id) {
        setSelectedArtifactID(response.side_panel_request.artifact_id);
        setActiveTab("media");
      }
      await refreshState();
      setMessages((current) => {
        const next = persistentMixBoard ? upsertPersistentMixBoardMessage(current, assistantMessage) : mergeAssistantMessageIntoChat(current, assistantMessage);
        debugConfirmation("interaction-final-merge", {
          interaction_id: interactionID,
          action_id: actionID,
          before: summarizeMessagesForConfirmation(current),
          after: summarizeMessagesForConfirmation(next)
        });
        return next;
      });
    } catch (interactionError) {
      const message = interactionError instanceof Error ? interactionError.message : "交互提交失败";
      setError(message);
      // F3 死卡守卫：应答失败回滚乐观台账——交互可能仍在 server 侧 pending，
      // 不得因台账残留把真待确认卡隐藏成幽灵。
      if (!persistentMixBoard) {
        forgetConsumedInteraction(interactionID, renderID);
      }
      setActivities((current) => dismissActivityByID(current, processingMessageID));
      setMessages((current) => [...current, {
          id: uniqueID("interaction_err"),
          role: "system",
          content: message,
          createdAt: Date.now(),
          status: "error",
          lifecycle: "durable",
          persistence: "local",
          message_kind: "error"
      }]);
    } finally {
      setRespondingActionID("");
    }
  };

  // FIX-CONFIRM-CARD-1 ③：完全档直执——对 fullAccessDirectExecutionTargets 命中
  // 的 RiskConfirm 确认闸，以 approve 决策自动应答（与用户点击确认执行同一
  // server 通路），应答回执并入对话流（事后回执：执行摘要+可回滚路径沿用
  // 既有结果卡）。失败时回滚台账并把原可交互卡补回流中，交还手动确认。
  const runFullAccessDirectExecution = async (interaction: JsonRecord): Promise<boolean> => {
    const interactionID = textValue(interaction.id ?? interaction.interaction_id, "");
    const renderID = actionRenderID(interaction);
    const synthetic = isSyntheticConfirmationInteraction(interaction);
    debugConfirmation("full-access-direct", {
      interaction_id: interactionID,
      render_id: renderID,
      synthetic,
      interaction: summarizeActionForConfirmation(interaction)
    });
    recordConsumedInteractions([interactionID, renderID]);
    try {
      const receipt = synthetic
        ? await confirmPlan({
            plan_id: textValue(interaction.plan_id ?? asRecord(interaction.payload).plan_id, ""),
            decision: "approve"
          })
        : await respondInteraction({
            interaction_id: interactionID,
            action_id: "approve",
            decision: "approve",
            payload: asRecord(interaction.payload ?? interaction.value)
          });
      debugConfirmation("full-access-direct-receipt", summarizeChatResponseForConfirmation(receipt));
      postAgentMutationsFromChatResponse(receipt, "full_access_direct");
      const receiptMessage = assistantMessageFromResponse(receipt);
      if (textValue(receiptMessage.content, "").trim() || (receiptMessage.actions ?? []).length > 0) {
        setMessages((current) => mergeAssistantMessageIntoChat(current, receiptMessage));
      }
      setAgentEventPolling(true);
      await refreshState();
      return true;
    } catch (directError) {
      const message = directError instanceof Error ? directError.message : "完全档直执失败";
      forgetConsumedInteraction(interactionID, renderID);
      setError(`完全访问直执失败（${message}）——已恢复确认卡，可手动确认。`);
      setMessages((current) => mergeAssistantMessageIntoChat(current, {
        id: uniqueID("full_access_direct"),
        role: "assistant",
        content: textValue(interaction.body ?? interaction.title, ""),
        mode,
        actions: [interaction],
        createdAt: Date.now(),
        status: "sent",
        lifecycle: "durable",
        persistence: "local",
        message_kind: "proposal"
      }));
      return false;
    }
  };

  const fullAccessDirectFiredRef = useRef<Set<string>>(new Set());
  const fireFullAccessDirectExecution = async (interaction: JsonRecord): Promise<boolean> => {
    const id = textValue(interaction.id ?? interaction.interaction_id, "") || actionRenderID(interaction);
    if (!id || fullAccessDirectFiredRef.current.has(id)) {
      return false;
    }
    fullAccessDirectFiredRef.current.add(id);
    return runFullAccessDirectExecution(interaction);
  };

  const handleAuditionSelect = async (sessionID: string, candidateID: string) => {
    setAuditionBusySessionID(sessionID);
    try {
      await selectAudition(conversationID, sessionID, candidateID);
      setAgentEventPolling(true);
    } catch (auditionError) {
      setError(auditionError instanceof Error ? auditionError.message : "A/B 选择失败");
    } finally {
      setAuditionBusySessionID("");
    }
  };

  const handleAuditionStop = async (sessionID: string) => {
    setAuditionBusySessionID(sessionID);
    try {
      await stopAudition(conversationID, sessionID);
      setAgentEventPolling(true);
    } catch (auditionError) {
      setError(auditionError instanceof Error ? auditionError.message : "停止试听失败");
    } finally {
      setAuditionBusySessionID("");
    }
  };

  const handleAuditionJudgment = async (payload: AuditionJudgmentPayload) => {
    // AB-JUDGMENT-CARD-1：判定 POST 的拒绝必须显形——此前无 catch，服务端 409
    // （identity mismatch）以未处理 rejection 形式被吞，用户点击零反馈零事件。
    try {
      await submitAuditionJudgment(payload);
      setAgentEventPolling(true);
    } catch (auditionError) {
      setError(auditionError instanceof Error ? auditionError.message : "判定提交失败");
    }
  };

  const hiddenFileInput = (
    <input
      ref={fileInputRef}
      className="hidden-input"
      type="file"
      multiple
      onChange={(event) => {
        if (event.currentTarget.files) {
          void handleUpload(event.currentTarget.files);
        }
      }}
    />
  );

  const openWorkbenchTab = (tab: WorkbenchTab) => {
    setActiveTab(tab);
    setRightPanelOpen(true);
  };

  const rightPanelMaximum = () => {
    const workspaceWidth = workspaceRef.current?.getBoundingClientRect().width ?? window.innerWidth;
    return Math.max(minRightPanelWidth, Math.min(workspaceWidth * 0.45, workspaceWidth - 460));
  };

  const updateRightPanelWidth = (width: number, persist = false) => {
    const next = Math.round(clampNumber(width, minRightPanelWidth, rightPanelMaximum()));
    setRightPanelWidths((current) => ({ ...current, [activeTab]: next }));
    if (persist) {
      savePanelWidth("right", activeTab, next);
    }
  };

  const beginRightPanelResize = (event: ReactPointerEvent<HTMLDivElement>) => {
    event.preventDefault();
    const startX = event.clientX;
    const startWidth = rightPanelWidth;
    let nextWidth = startWidth;
    document.body.classList.add("panel-resizing");
    const move = (moveEvent: PointerEvent) => {
      nextWidth = clampNumber(startWidth + startX - moveEvent.clientX, minRightPanelWidth, rightPanelMaximum());
      setRightPanelWidths((current) => ({ ...current, [activeTab]: Math.round(nextWidth) }));
    };
    const finish = () => {
      document.body.classList.remove("panel-resizing");
      document.removeEventListener("pointermove", move);
      document.removeEventListener("pointerup", finish);
      savePanelWidth("right", activeTab, Math.round(nextWidth));
    };
    document.addEventListener("pointermove", move);
    document.addEventListener("pointerup", finish, { once: true });
  };

  const workbenchPanel = (
    <aside className={`workbench-panel ${isWorkbenchSurface ? "standalone" : ""}`}>
      <WorkbenchTabs activeTab={activeTab} setActiveTab={openWorkbenchTab} onCollapse={isWorkbenchSurface ? undefined : () => setRightPanelOpen(false)} />
      <Workbench
        activeTab={activeTab}
        uiState={uiState}
        artifacts={artifacts}
        selectedArtifact={selectedArtifact}
        conversationID={conversationID}
        onSelectArtifact={setSelectedArtifactID}
        onAttachArtifact={attachArtifact}
        onRenameArtifact={handleRenameArtifact}
        onDeleteArtifact={handleDeleteArtifact}
        onUploadClick={() => fileInputRef.current?.click()}
        onModeChange={setMode}
        onAttachMacroControl={attachMacroControl}
        onInsertMacroControlCard={insertMacroControlCard}
        onMacroValuePreview={previewMacroControlValue}
        onMacroValueCommit={commitMacroControlValue}
        onMacroRename={renameMacroControlLabel}
        onRefresh={refreshState}
        checkoutBlocked={checkoutBlockedByState(uiState, runtimeStatus)}
      />
    </aside>
  );

  const conversationPanel = (
    <section className="conversation-panel">
      <div className="conversation-toolbar" aria-label="对话模式">
        <ModeSwitch value={mode} onChange={setMode} />
      </div>

      <MessageStream
        messages={messages}
        activities={activities}
        trajectory={trajectoryState}
        turnEventMeta={turnEventMeta}
        roundSteps={roundSteps}
        receipts={turnReceipts}
        audition={auditionState}
        auditionBusySessionID={auditionBusySessionID}
        onAuditionSelect={handleAuditionSelect}
        onAuditionStop={handleAuditionStop}
        onSubmitAuditionJudgment={handleAuditionJudgment}
        authorityMode={authorityMode}
        respondingActionID={respondingActionID}
        isSending={isSending}
        hiddenActionID={composerInteractionID}
        bottomInset={messageBottomInset}
        onInteractionAction={handleInteractionAction}
        onInvoke={invokeDawAction}
        onSelectArtifact={(id) => {
          setSelectedArtifactID(id);
          setActiveTab("media");
        }}
        uiState={uiState}
        onMacroValuePreview={previewMacroControlValue}
        onMacroValueCommit={commitMacroControlValue}
        onSendSupplement={(text) => {
          void handleSend(undefined, text);
        }}
      />

      {error && (
        <div className="notice warning" ref={noticeRef}>
          <AlertTriangle size={16} />
          <span>{error}</span>
        </div>
      )}

      {/* GUI-T6 设计位（PLANBAR-1 修复）：规划条与输入框同挂 .composer-dock 停靠
          列——列向 flex 保证栏底边恒在 composer 顶边之上；栏留在面板流内时会被
          composer 的绝对定位浮层压住（缺陷①的形态）。停靠列整体高度即消息流预留。 */}
      <div className="composer-dock" ref={composerDockRef}>
        {/* WEBUI-SESSION-SEMANTICS-1：PlanBar 的 goal/plan 全局兜底与链活信号按
            active session 归属门控——快照归约已保证 snapshot 非空 ⟺ 全局任务属于
            本会话；异会话任务在跑（note 流/其他 webui 流）时兜底置空，栏随快照
            清空立即卸载，不再以无快照兜底形态滞留或泄漏他流意图文本。本会话自身
            活动（事件流轨迹回合开放/链活且属主快照在位）保持既有兜底行为。 */}
        <PlanBar
          snapshot={taskTrajectoryState.snapshot}
          goal={planBarOwnLive ? asRecord(uiState?.goal) : null}
          plan={planBarOwnLive ? asRecord(uiState?.agent_plan) : null}
          chainLive={planBarOwnLive}
        />

        <Composer
          composerRef={composerRef}
          input={input}
          setInput={setInput}
          pendingArtifacts={pendingArtifacts}
          pendingMacroControls={pendingMacroRefs}
          mode={mode}
          agentTurnRunning={agentTurnRunning}
          stopTurnBusy={stopTurnBusy}
          isSending={isSending}
          isUploading={isUploading}
          interactionAction={composerInteraction}
          respondingActionID={respondingActionID}
          authorityMode={authorityMode}
          authorityLocked={authorityBusy || agentTurnRunning}
          onAuthorityModeChange={handleAuthorityModeChange}
          onSubmit={handleSend}
          onUploadClick={() => fileInputRef.current?.click()}
          onModeChange={setMode}
          onStopTurn={handleStopTurn}
          onInteractionAction={handleInteractionAction}
          onInvoke={invokeDawAction}
          onSelectArtifact={(id) => {
            setSelectedArtifactID(id);
            setActiveTab("media");
          }}
          onRemoveArtifact={removePendingArtifact}
          onRemoveMacroControl={removePendingMacroControl}
        />
      </div>

      {hiddenFileInput}
    </section>
  );

  if (isWorkbenchSurface) {
    return (
      <main
        className="app-shell app-shell-workbench"
        onDragOver={(event) => {
          if (event.dataTransfer.types.includes("Files")) {
            event.preventDefault();
          }
        }}
        onDrop={(event) => {
          if (!event.dataTransfer.types.includes("Files") || event.dataTransfer.files.length === 0) {
            return;
          }
          event.preventDefault();
          void handleUpload(event.dataTransfer.files);
        }}
      >
        {/* SCORE-ENTRY-REACH-1（382）：standalone 工作台的返回工程条（从工程面
            「打开工作台」进来的用户有回头路；宿主桥/浏览器 dev 均适用）。 */}
        {onExitWorkbench ? (
          <div className="workbench-return-bar" data-workbench-return-bar>
            <button type="button" className="workbench-return-button" data-workbench-exit-project onClick={onExitWorkbench}>
              ← 返回工程
            </button>
          </div>
        ) : null}
        {workbenchPanel}
        {hiddenFileInput}
      </main>
    );
  }

  return (
    <main
      className={`app-shell ${isMainSurface ? "app-shell-main-surface" : ""}`}
      onDragOver={(event) => {
        if (event.dataTransfer.types.includes("Files")) {
          event.preventDefault();
        }
      }}
      onDrop={(event) => {
        if (!event.dataTransfer.types.includes("Files") || event.dataTransfer.files.length === 0) {
          return;
        }
        event.preventDefault();
        void handleUpload(event.dataTransfer.files);
      }}
    >
      <TopStatusBar
        connection={connection}
        healthLabel={healthLabel}
        runtimeStatus={runtimeStatus}
        uiState={uiState}
        conversationTitle={conversationTitle}
        onOpenHistory={() => openWorkbenchTab("history")}
        onRefresh={refreshState}
        onTransportCommand={runTransportCommand}
        transportBusy={transportBusy}
      />

      <section className="main-workspace">
        <SessionFlowSidebar
          scopeKey={historyScopeKeyFromUIState(uiState)}
          scopeParts={historyScopePartsFromUIState(uiState)}
          currentConversationID={conversationID}
          continuations={runtimeStatus?.continuations}
          noteSessions={runtimeStatus?.note_sessions}
          mainConversationID={mainConversationID}
          mainFlowTitle={mainFlowTitle}
          settingsOpen={settingsOpen}
          onNewConversation={handleNewConversation}
          onSwitchConversation={handleSwitchConversation}
          onOpenSettings={() => setSettingsOpen(true)}
          readEntryTitle={sessionEntryTitleFor}
        />

        <section
          ref={workspaceRef}
          className={`workspace-grid project-aware ${isMainSurface ? "main-only" : ""} ${rightPanelOpen ? "right-open" : "right-collapsed"}`}
          style={workspaceStyle}
        >
          {settingsOpen ? (
            <SettingsPage
              runtimeStatus={runtimeStatus}
              uiState={uiState}
              onClose={() => setSettingsOpen(false)}
            />
          ) : (
            <>
              {conversationPanel}
              {!isMainSurface && rightPanelOpen && (
                <>
                  <PanelResizeHandle
                    side="right"
                    onPointerDown={beginRightPanelResize}
                    onReset={() => updateRightPanelWidth(defaultRightPanelWidth, true)}
                    onNudge={(delta) => updateRightPanelWidth(rightPanelWidth - delta, true)}
                  />
                  {workbenchPanel}
                </>
              )}
              {!isMainSurface && !rightPanelOpen && <CollapsedWorkbench activeTab={activeTab} onOpen={openWorkbenchTab} />}
            </>
          )}
        </section>
      </section>
    </main>
  );
}

function appSurface(): AppSurface {
  const value = new URLSearchParams(window.location.search).get("surface")?.toLowerCase();
  if (value === "main" || value === "workbench" || value === "launcher" || value === "project") {
    return value;
  }
  return "full";
}

function initialConversationID(): string {
  const fromURL = conversationIDFromURL();
  if (fromURL) {
    return fromURL;
  }
  return createConversationID();
}

function conversationIDFromURL(): string {
  return new URLSearchParams(window.location.search).get("conversation_id")?.trim() ?? "";
}

function ModeSwitch({ value, onChange }: { value: AgentMode; onChange: (mode: AgentMode) => void }) {
  return (
    <div className="mode-switch">
      {modeItems.map(({ key, label, hint, icon: Icon }) => (
        <button key={key} type="button" className={value === key ? "active" : ""} onClick={() => onChange(key)}>
          <Icon size={15} />
          <span>{label}</span>
          <small>{hint}</small>
        </button>
      ))}
    </div>
  );
}

export function chatMessageFromAgentEvent(event: AgentEvent, mode?: AgentMode | string): ChatMessage | null {
  const type = textValue(event.type, "");
  const status = textValue(event.status, "");
  // AGENT-F6：调度侧续跑链的终片 turn.completed/turn.failed/turn.stopped 带
  // scheduler_chain 标记且 body 装着最终回复——这是链的结果消息（此前被静默
  // 丢弃，多轮执行后用户看不到任何结果）。GUI-1：stopped 终局补齐合成（F7 扩
  // 展，CONTRACT-1 服务端已保证 stopped body 非空，UI 侧再兜底一次）。
  // HTTP 路径的 turn.completed 照旧不产消息（回复已由 HTTP 响应本身交付，再
  // 产会双份）。
  if (Boolean(asRecord(event.payload).scheduler_chain) && (type === "turn.completed" || type === "turn.failed" || type === "turn.stopped")) {
    const isError = type === "turn.failed" || status === "failed";
    const isStopped = type === "turn.stopped" || status === "stopped";
    const reply = textValue(event.body, "") || (isStopped ? "回合已停止。" : "");
    if (!reply) {
      return null;
    }
    const sourceID = `agent_event_${textValue(event.goal_id, "goal")}_chain_result`;
    return transientMessage({
      id: sourceID,
      source_id: sourceID,
      // stopped 是中性系统消息（回合被停止，非 assistant 自然回复、也非错误态）
      role: isError || isStopped ? "system" : "assistant",
      content: reply,
      mode,
      createdAt: agentEventCreatedAt(event),
      status: isError ? "error" : "sent"
    });
  }
  if (type === "turn.started" || type === "turn.completed") {
    return null;
  }
  const sourceID = agentEventSourceID(event);
  const content = agentEventMessageContent(event);
  if (!sourceID || !content) {
    return null;
  }
  const isRunning = type === "item.started" || status === "running" || status === "in_progress";
  const isError = type === "turn.failed" || status === "failed" || status === "error";
  return transientMessage({
    id: sourceID,
    source_id: sourceID,
    role: isError ? "system" : "assistant",
    content,
    mode,
    createdAt: agentEventCreatedAt(event),
    status: isRunning ? "pending" : isError ? "error" : "sent",
    actions: [agentEventAction(event)]
  });
}

export function agentEventSourceID(event: AgentEvent): string {
  const itemID = textValue(event.item_id, "");
  const goalID = textValue(event.goal_id, "");
  const type = textValue(event.type, "event");
  if (itemID) {
    return `agent_event_${goalID || "goal"}_${itemID}`;
  }
  if (type === "turn.failed") {
    return `agent_event_${goalID || "goal"}_failed_${Number(event.seq) || 0}`;
  }
  return "";
}

function agentEventMessageContent(event: AgentEvent): string {
  const type = textValue(event.type, "");
  const title = agentEventDisplayTitle(event);
  const body = localizeDisplayText(textValue(event.body, ""));
  const status = textValue(event.status, "");
  if (type === "item.started") {
    return title ? `正在执行：${title}` : "正在执行工程操作";
  }
  if (type === "approval.requested") {
    return body || title || "这个操作需要你确认后才会执行。";
  }
  if (type === "turn.failed") {
    return body || "执行失败。";
  }
  if (status === "failed" || status === "error") {
    return body || title || "执行失败。";
  }
  return title ? `已完成：${title}` : body || "工程操作已完成。";
}

export function agentEventAction(event: AgentEvent): JsonRecord {
  const sourceID = agentEventSourceID(event);
  const payload = asRecord(event.payload);
  return {
    ...payload,
    _ui_source: "event",
    id: `${sourceID}_action`,
    kind: textValue(event.item_type, "event"),
    type: textValue(event.type, "event"),
    title: textValue(event.title, ""),
    body: textValue(event.body, ""),
    status: textValue(event.status, ""),
    created_at: textValue(event.created_at, ""),
    seq: event.seq,
    payload
  };
}

function agentEventCreatedAt(event: AgentEvent): number {
  const createdAt = textValue(event.created_at, "");
  const parsed = createdAt ? Date.parse(createdAt) : NaN;
  return Number.isFinite(parsed) ? parsed : Date.now();
}

function processingChatMessage(id: string, content: string, mode?: AgentMode | string): ChatMessage {
  return transientMessage({
    id,
    role: "assistant",
    content,
    mode,
    createdAt: Date.now(),
    status: "pending"
  });
}

function interactionProcessingText(interaction: JsonRecord, actionID: string, actionLabel: string): string {
  const normalizedAction = actionID.trim().toLowerCase();
  if (normalizedAction === "cancel") {
    return "正在取消";
  }
  if (isConfirmationAction(interaction)) {
    return "正在执行确认的操作";
  }
  const label = actionLabel.trim();
  return label ? `正在处理：${label}` : "正在处理你的选择";
}

function assistantMessageFromResponse(response: ChatResponse): ChatMessage {
  const interactionActions = interactionActionsFrom(response.interaction_requests);
  const mixBoardAction = mixBoardActionWithInteractions(mixBoardActionFromPayload(response), interactionActions);
  const visibleInteractionActions = mixBoardAction ? interactionActions.filter((action) => !isMixBoardAction(action)) : interactionActions;
  const hasPendingInteractionAction = visibleInteractionActions.some(isPendingInteractionAction);
  const confirmationFallbackActions = hasPendingInteractionAction ? [] : confirmationFallbackActionsFromResponse(response);
  const projectResultActions = projectResultActionsFromResponse(response);
  const actions = [
    ...(mixBoardAction ? [mixBoardAction] : []),
    ...visibleInteractionActions,
    ...confirmationFallbackActions,
    ...projectResultActions,
    ...(response.commands ?? []).map((action) => ({ ...action, _ui_source: "command" })),
    ...(response.executed_kernel_reply ?? []).map((action) => ({ ...action, _ui_source: "executed" }))
  ];
  const content = response.error || response.reply || response.message || response.preview || "Vit 已返回。";
  return durableMessage({
    id: uniqueID("reply"),
    role: response.error ? "system" : "assistant",
    content,
    mode: response.agent_mode,
    artifacts: response.artifacts,
    actions,
    createdAt: Date.now(),
    status: response.error ? "error" : "sent",
    ...responseMessageProtocol(response, content)
  });
}

function invokeMessageFromResponse(response: AgentInvokeResponse, mode: AgentMode): ChatMessage {
	const result = asRecord(response.result);
	const responseRecord = asRecord(response);
	const mixSession = asRecord(result.mix_session);
	const hasMixSession = Object.keys(mixSession).length > 0;
	const hasError = textValue(response.error, "") !== "" || textValue(response.status, "").toLowerCase() === "error";
	const content = hasError
		? textValue(response.error, `${response.command_name || response.tool || "Agent"} failed.`)
		: hasMixSession
			? textValue(result.message ?? result.reply ?? mixSession.message ?? mixSession.summary ?? response.preview, "Mix task design is ready.")
			: textValue(result.message ?? result.reply ?? response.preview, "Agent command completed.");
  const resultInteractions = interactionActionsFrom(result.interaction_requests, responseRecord.interaction_requests);
  const resultArtifacts = artifactSummariesFrom(result.artifacts, responseRecord.artifacts);
  const mixBoardAction = mixBoardActionWithInteractions(mixBoardActionFromPayload(result, responseRecord), resultInteractions);
  const visibleResultInteractions = mixBoardAction ? resultInteractions.filter((action) => !isMixBoardAction(action)) : resultInteractions;
  const actions: JsonRecord[] = [
    ...(mixBoardAction ? [mixBoardAction] : []),
    ...visibleResultInteractions,
    {
      ...response,
      _ui_source: "executed",
      title: response.command_name || response.tool || (hasMixSession ? "Mix Session" : "Plugin Grabber"),
      body: textValue(response.preview ?? result.message ?? result.reply, ""),
      status: response.status,
      error: response.error
    }
  ];
  return durableMessage({
    id: uniqueID("invoke"),
    role: hasError ? "system" : "assistant",
    content,
    mode,
    artifacts: resultArtifacts,
    actions,
    createdAt: Date.now(),
    status: hasError ? "error" : "sent"
  }, { kind: hasError ? "error" : "assistant", persistence: "local" });
}

function loadRightPanelWidths(): Record<WorkbenchTab, number> {
  return {
    media: loadPanelWidth("right", "media", defaultRightPanelWidth, minRightPanelWidth),
    macro: loadPanelWidth("right", "macro", defaultRightPanelWidth, minRightPanelWidth),
    score: loadPanelWidth("right", "score", defaultRightPanelWidth, minRightPanelWidth),
    history: loadPanelWidth("right", "history", defaultRightPanelWidth, minRightPanelWidth)
  };
}

export function interactionActionID(interactionID: string, actionID: string): string {
  return `${interactionID}:${actionID}`;
}

export default App;
