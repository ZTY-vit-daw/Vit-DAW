import {
  Activity,
  AlertTriangle,
  Archive,
  Bot,
  Brain,
  CheckCircle2,
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
  Gauge,
  GitBranch,
  Globe,
  Layers,
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
import { FormEvent, KeyboardEvent as ReactKeyboardEvent, MouseEvent as ReactMouseEvent, PointerEvent as ReactPointerEvent, WheelEvent as ReactWheelEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
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
  inspectAuditionCandidate,
  applyAuditionCandidate,
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
  transientMessage,
  upsertActivity
} from "./messageLifecycle";
import { emptyTrajectoryState, reduceTrajectoryEvents } from "./trajectory";
import { emptyAuditionState, reduceAuditionEvents } from "./audition";
import { authorityContext, checkoutBlockedByState, isAgentTurnRunning } from "./turnControl";
import { TrajectoryAuditionPanel } from "./trajectory/TrajectoryAuditionPanel";
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

const WEBUI_BUILD_MARK = "vit-confirmation-consumption-v1-20260801";

console.info(`[VitWebUI] loaded ${WEBUI_BUILD_MARK}`);

const mixBoardUserNoteDrafts = new Map<string, string>();

type ConnectionStatus = "loading" | "ready" | "offline";
type FocusMode = "dialogue" | "tracks" | "rack" | "midi" | "mixer";
type WorkbenchTab = "history" | "media" | "macro";
type SettingsTab = "agent" | "llm" | "multimodal" | "browser" | "diagnostics";
type MediaContextMenuState = { artifact: ArtifactSummary; x: number; y: number };
type AppSurface = "full" | "main" | "workbench";
type MacroValueOverride = { value: number; macro: MacroControl; expiresAt: number };
type BrowserBridgeState = {
  ready?: boolean;
  loading?: boolean;
  url?: string;
  title?: string;
  last_error?: string;
  can_go_back?: boolean;
  can_go_forward?: boolean;
  visible?: boolean;
  profile?: string;
};

const browserBridgeChannel = "ask_vit_browser";
const externalBrowserChannel = "ask_vit_external_browser";
const agentMutationBridgeChannel = "ask_vit_agent_mutations";
const defaultExternalBrowserURL = "https://www.bing.com/";
const conversationMessagesStoragePrefix = "ask_vit_conversation_messages";
const conversationScopedIDStoragePrefix = "ask_vit_conversation_id_scope";
const attachedArtifactStorageKey = "ask_vit_attached_artifact";
const panelWidthStoragePrefix = "ask_vit_project_workbench_width";
const defaultRightPanelWidth = 390;
const minRightPanelWidth = 280;

const modeItems: Array<{ key: AgentMode; label: string; hint: string; icon: LucideIcon }> = [
  { key: "default", label: "协作", hint: "自然对话", icon: Bot },
  { key: "plan", label: "计划", hint: "只读分析", icon: Brain },
  { key: "goal", label: "目标", hint: "长任务", icon: Wand2 }
];

const focusItems: Array<{ key: FocusMode; label: string; icon: LucideIcon }> = [
  { key: "dialogue", label: "对话", icon: Bot },
  { key: "tracks", label: "轨道", icon: Layers },
  { key: "rack", label: "机架", icon: Plug },
  { key: "midi", label: "MIDI", icon: Activity },
  { key: "mixer", label: "混音台", icon: Gauge }
];

const workbenchTabs: Array<{ key: WorkbenchTab; label: string; icon: LucideIcon }> = [
  { key: "media", label: "资料库", icon: Archive },
  { key: "macro", label: "宏控制", icon: Wand2 },
  { key: "history", label: "历史树", icon: Clock }
];

const settingsTabs: Array<{ key: SettingsTab; label: string; icon: LucideIcon }> = [
  { key: "agent", label: "Agent", icon: Bot },
  { key: "llm", label: "LLM API", icon: Brain },
  { key: "multimodal", label: "多模态路由", icon: Archive },
  { key: "browser", label: "浏览器", icon: Globe },
  { key: "diagnostics", label: "诊断", icon: Activity }
];

const multimodalRouteMeta: Array<{ key: string; label: string; hint: string }> = [
  { key: "text", label: "文本 LLM", hint: "Agent 对话与规划" },
  { key: "image_understanding", label: "图片理解", hint: "图片、截图、视觉素材分析" },
  { key: "image_generation", label: "图片生成", hint: "生成图像与设计稿" },
  { key: "audio_understanding", label: "音频理解", hint: "音频内容和素材分析" },
  { key: "transcription", label: "音频转写", hint: "语音、采样说明、口述转写" },
  { key: "video_browser", label: "视频/浏览器理解", hint: "WebView2 页面与视频上下文" },
  { key: "embeddings", label: "Embedding / 检索", hint: "历史、媒体和工程语义检索" }
];

const initialMessage: ChatMessage = {
  id: "intro",
  role: "assistant",
  content: "Ask Vit 就绪。",
  createdAt: Date.now(),
  lifecycle: "durable",
  persistence: "local",
  message_kind: "assistant"
};

function App() {
  const surface = useMemo(() => appSurface(), []);
  const isWorkbenchSurface = surface === "workbench";
  const isMainSurface = surface === "main";
  const [connection, setConnection] = useState<ConnectionStatus>("loading");
  const [healthLabel, setHealthLabel] = useState("连接中");
  const [runtimeStatus, setRuntimeStatus] = useState<RuntimeStatusResponse | null>(null);
  const [uiState, setUIState] = useState<AgentUIState | null>(null);
  const [messages, setMessages] = useState<ChatMessage[]>([initialMessage]);
  const [activities, setActivities] = useState<ChatMessage[]>([]);
  const [trajectoryState, setTrajectoryState] = useState(emptyTrajectoryState);
  const [auditionState, setAuditionState] = useState(emptyAuditionState);
  const [auditionBusySessionID, setAuditionBusySessionID] = useState("");
  const auditionWaiting = useMemo(() => Object.values(auditionState.sessions).some((session) => session.status === "preparing"), [auditionState]);
  const [conversationID, setConversationID] = useState(initialConversationID);
  const [mode, setMode] = useState<AgentMode>("default");
  const [authorityMode, setAuthorityModeState] = useState<AuthorityMode>("manual_confirmation");
  const [authorityBusy, setAuthorityBusy] = useState(false);
  const [stopTurnBusy, setStopTurnBusy] = useState(false);
  const [activeFocus, setActiveFocus] = useState<FocusMode>("dialogue");
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
  const [transportBusy, setTransportBusy] = useState(false);
  const [error, setError] = useState("");
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const composerRef = useRef<HTMLFormElement | null>(null);
  const noticeRef = useRef<HTMLDivElement | null>(null);
  const workspaceRef = useRef<HTMLElement | null>(null);
  const historyScopeRef = useRef("");
  const scopedConversationRef = useRef("");
  const pausedHistoryScopeRef = useRef("");
  const restoredMessageScopeRef = useRef("");
  const skipNextStoredMessageSaveRef = useRef("");
  const mediaScopeRef = useRef("");
  const agentEventSeqRef = useRef(0);
  const macroValueOverridesRef = useRef<Record<string, MacroValueOverride>>({});

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
      setHealthLabel(runtimeResult.value.service ?? "VitAgent");
      setError("");
    } else {
      setConnection("offline");
      setRuntimeStatus(null);
      setHealthLabel("未连接");
      setError(runtimeResult.reason instanceof Error ? runtimeResult.reason.message : "Agent 未连接");
    }
    if (uiResult.status === "fulfilled") {
      setUIState(applyMacroValueOverridesToUIState(uiResult.value, macroValueOverridesRef.current));
    }
  }, []);

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
    setAuditionState(emptyAuditionState());
    setAgentEventPolling(true);
  }, [conversationID]);

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
    const storedMessages = loadStoredConversationMessages(conversationID, scope);
    if (storedMessages.length === 0) {
      return;
    }
    skipNextStoredMessageSaveRef.current = restoreKey;
    setMessages((current) => {
      if (!hasMeaningfulChatMessages(current)) {
        return messagesOrIntro(storedMessages);
      }
      return mergeChatMessages(storedMessages, current);
    });
  }, [conversationID, uiState]);

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

  useEffect(() => {
    if (!agentEventPolling) {
      return;
    }
    let cancelled = false;
    let idleTicks = 0;
    const poll = async () => {
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
          idleTicks = 0;
          if (shouldDebugAgentEvents(events)) {
            debugConfirmation("agent-events-polled", {
              conversation_id: conversationID,
              requested_since: requestedSince,
              next_seq: response.next_seq,
              is_sending: isSending,
              responding_action_id: respondingActionID,
              events: events.map(summarizeAgentEventForConfirmation)
            });
          }
          setActivities((current) => reduceAgentEventActivities(
            current,
            events,
            (agentEvent) => chatMessageFromAgentEvent(agentEvent, mode)
          ));
          setTrajectoryState((current) => reduceTrajectoryEvents(current, events));
          setAuditionState((current) => reduceAuditionEvents(current, events));
        } else if (!isSending && !respondingActionID && !auditionWaiting) {
          idleTicks += 1;
          if (idleTicks >= 4) {
            setAgentEventPolling(false);
          }
        }
      } catch {
        if (!cancelled && !isSending && !respondingActionID && !auditionWaiting) {
          idleTicks += 1;
          if (idleTicks >= 3) {
            setAgentEventPolling(false);
          }
        }
      }
    };
    void poll();
    const timer = window.setInterval(() => void poll(), 500);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [agentEventPolling, auditionWaiting, conversationID, isSending, mode, respondingActionID]);

  useEffect(() => {
    const nextScope = historyScopeKeyFromUIState(uiState);
    if (!nextScope || pausedHistoryScopeRef.current === nextScope) {
      return;
    }
    const historyMessages = historyMessagesFromUIState(uiState);
    const scopeChanged = historyScopeRef.current !== nextScope;
    if (scopeChanged) {
      setActivities([]);
    }
    setMessages((current) => {
      const hasPendingInteraction = hasPendingComposerInteraction(current);
      const baseMessages = hasPendingInteraction ? current : removeDisposableConfirmationPromptMessages(current);
      debugConfirmation("history-sync", {
        scopeChanged,
        current: summarizeMessagesForConfirmation(current),
        current_after_prompt_filter: summarizeMessagesForConfirmation(baseMessages),
        incoming: summarizeMessagesForConfirmation(historyMessages)
      });
      if (scopeChanged) {
        historyScopeRef.current = nextScope;
        const nextConversationID = conversationIDFromURL() || loadStoredScopedConversationID(nextScope) || createConversationID();
        saveStoredScopedConversationID(nextScope, nextConversationID);
        scopedConversationRef.current = scopedConversationRuntimeKey(nextScope, nextConversationID);
        restoredMessageScopeRef.current = "";
        agentEventSeqRef.current = 0;
        setConversationID((currentConversationID) =>
          currentConversationID === nextConversationID ? currentConversationID : nextConversationID
        );
        return messagesOrIntro(historyMessages);
      }
      return mergeChatMessages(baseMessages, historyMessages);
    });
  }, [uiState]);

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
  const composerInteraction = useMemo(() => latestComposerInteraction(messages, dismissedInteractionIDs), [messages, dismissedInteractionIDs]);
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
      const noticeHeight = noticeRef.current?.getBoundingClientRect().height ?? 0;
      const nextInset = Math.max(132, Math.ceil(composerHeight + noticeHeight + 36));
      setMessageBottomInset((current) => (Math.abs(current - nextInset) > 1 ? nextInset : current));
    };
    measureBottomInset();
    const observer = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(measureBottomInset);
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

  useEffect(() => {
    const scope = historyScopeKeyFromUIState(uiState);
    if (!scope) {
      return;
    }
    if (scopedConversationRef.current !== scopedConversationRuntimeKey(scope, conversationID)) {
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

  const handleSend = async (event?: FormEvent) => {
    event?.preventDefault();
    const messageText = input.trim();
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
        context: buildChatContext(mode, activeFocus, uiState, attached, macroRefs, authorityMode)
      });
      debugConfirmation("chat-response", summarizeChatResponseForConfirmation(response));
      postAgentMutationsFromChatResponse(response, "chat");
      const assistantMessage = assistantMessageFromResponse(response);
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
      setError(authorityError instanceof Error ? authorityError.message : "权限模式切换失败");
    } finally {
      setAuthorityBusy(false);
    }
  };

  const currentGoal = asRecord(uiState?.goal ?? runtimeStatus?.goal);
  const currentGoalStatus = textValue(currentGoal.status, "").toLowerCase();
  const agentTurnRunning = isAgentTurnRunning(currentGoalStatus, isSending);

  const handleStopTurn = async () => {
    if (stopTurnBusy || !agentTurnRunning) return;
    setStopTurnBusy(true);
    setError("");
    try {
      const activeTurn = Object.values(trajectoryState.turns).find((turn) => !turn.stopped && turn.status !== "completed" && turn.status !== "failed") ?? Object.values(trajectoryState.turns)[0];
      await stopTurn({
        conversation_id: conversationID,
        goal_id: textValue(currentGoal.goal_id, ""),
        run_id: textValue(currentGoal.run_id, ""),
        turn_id: activeTurn?.id ?? textValue(currentGoal.run_id, ""),
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
    setActiveFocus("dialogue");
    setInput("");
    setError("");
  };

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
      context: buildChatContext(mode, activeFocus, uiState, [], macroControlsFromUIState(uiState), authorityMode)
    });
    postAgentMutationsFromInvokeResponse(response, source);
    if (response.status === "error") {
      const message = textValue(response.error, "DAW action failed");
      setError(message);
      throw new Error(message);
    }
    await refreshState();
    return response;
  }, [activeFocus, mode, refreshState, uiState]);

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
    await submitAuditionJudgment(payload);
    setAgentEventPolling(true);
  };

  const handleAuditionInspect = async (sessionID: string, candidateID: string) => {
    setAuditionBusySessionID(sessionID);
    try {
      await inspectAuditionCandidate(conversationID, sessionID, candidateID);
      setAgentEventPolling(true);
      await refreshState();
    } catch (inspectError) {
      setError(inspectError instanceof Error ? inspectError.message : "查看候选失败");
    } finally {
      setAuditionBusySessionID("");
    }
  };

  const handleAuditionApply = async (sessionID: string, candidateID: string, evidenceID: string) => {
    setAuditionBusySessionID(sessionID);
    try {
      await applyAuditionCandidate(conversationID, sessionID, candidateID, evidenceID);
      setAgentEventPolling(true);
      await refreshState();
    } catch (applyError) {
      setError(applyError instanceof Error ? applyError.message : "采用候选失败");
    } finally {
      setAuditionBusySessionID("");
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

  const handleFocusChange = (nextFocus: FocusMode) => {
    setSettingsOpen(false);
    setActiveFocus(nextFocus);
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

      <TrajectoryAuditionPanel
        trajectory={trajectoryState}
        audition={auditionState}
        busySessionID={auditionBusySessionID}
        onSelect={handleAuditionSelect}
        onStop={handleAuditionStop}
        onSubmitJudgment={handleAuditionJudgment}
        onInspect={handleAuditionInspect}
        onApply={handleAuditionApply}
      />

      <MessageStream
        messages={messages}
        activities={activities}
        respondingActionID={respondingActionID}
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
      />

      {error && (
        <div className="notice warning" ref={noticeRef}>
          <AlertTriangle size={16} />
          <span>{error}</span>
        </div>
      )}

      <Composer
        composerRef={composerRef}
        input={input}
        setInput={setInput}
        pendingArtifacts={pendingArtifacts}
        pendingMacroControls={pendingMacroRefs}
        mode={mode}
        authorityMode={authorityMode}
        authorityBusy={authorityBusy}
        agentTurnRunning={agentTurnRunning}
        stopTurnBusy={stopTurnBusy}
        isSending={isSending}
        isUploading={isUploading}
        interactionAction={composerInteraction}
        respondingActionID={respondingActionID}
        onSubmit={handleSend}
        onUploadClick={() => fileInputRef.current?.click()}
        onModeChange={setMode}
        onAuthorityModeChange={handleAuthorityModeChange}
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
        <SideRail
          activeFocus={activeFocus}
          settingsOpen={settingsOpen}
          onFocusChange={handleFocusChange}
          onNewConversation={handleNewConversation}
          onOpenSettings={() => setSettingsOpen(true)}
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
              {activeFocus === "dialogue" ? (
                conversationPanel
              ) : (
                <DawFocusPanel
                  activeFocus={activeFocus}
                  uiState={uiState}
                  connection={connection}
                  onInvoke={invokeDawAction}
                  onRefresh={refreshState}
                  onClose={() => handleFocusChange("dialogue")}
                />
              )}
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

function SideRail({
  activeFocus,
  settingsOpen,
  onFocusChange,
  onNewConversation,
  onOpenSettings
}: {
  activeFocus: FocusMode;
  settingsOpen: boolean;
  onFocusChange: (mode: FocusMode) => void;
  onNewConversation: () => void;
  onOpenSettings: () => void;
}) {
  return (
    <nav className="side-rail" aria-label="Ask Vit modes">
      <div className="rail-brand">
        <strong>Ask</strong>
        <span>Vit</span>
      </div>
      <div className="rail-main">
        {focusItems.map(({ key, label, icon: Icon }) => (
          <button
            key={key}
            type="button"
            className={`rail-button ${activeFocus === key ? "active" : ""}`}
            title={label}
            onClick={() => onFocusChange(key)}
          >
            <Icon size={20} />
            <span>{label}</span>
          </button>
        ))}
      </div>
      <div className="rail-footer">
        <button className="rail-button compact" type="button" title="新建对话" onClick={onNewConversation}>
          <Plus size={19} />
          <span>新建</span>
        </button>
        <button className={`rail-button compact ${settingsOpen ? "active" : ""}`} type="button" title="设置" onClick={onOpenSettings}>
          <Settings size={19} />
          <span>设置</span>
        </button>
      </div>
    </nav>
  );
}

function focusTitle(activeFocus: FocusMode): string {
  switch (activeFocus) {
    case "tracks":
      return "轨道";
    case "rack":
      return "机架";
    case "midi":
      return "MIDI";
    case "mixer":
      return "混音台";
    default:
      return "对话";
  }
}

function appSurface(): AppSurface {
  const value = new URLSearchParams(window.location.search).get("surface")?.toLowerCase();
  if (value === "main" || value === "workbench") {
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

function FocusSummary({ activeFocus, uiState }: { activeFocus: FocusMode; uiState: AgentUIState | null }) {
  const tracks = uiState?.tracks ?? [];
  const selectedTrack = asRecord(uiState?.selected_track);
  const rack = asRecord(uiState?.plugin_rack);
  const transport = asRecord(uiState?.transport);
  const pluginCount = firstArray(rack.plugins, rack.items, rack.chain, rack.rack).length;
  const rows: Array<[string, string]> = [];
  if (activeFocus === "tracks") {
    rows.push(["轨道数", String(tracks.length)]);
    rows.push(["当前轨道", textValue(selectedTrack.name ?? selectedTrack.track_name, "未选择")]);
  } else if (activeFocus === "rack") {
    rows.push(["当前轨道", textValue(selectedTrack.name ?? selectedTrack.track_name, "未选择")]);
    rows.push(["机架节点", String(pluginCount)]);
  } else if (activeFocus === "midi") {
    rows.push(["当前轨道", textValue(selectedTrack.name ?? selectedTrack.track_name, "未选择")]);
    rows.push(["编辑语境", "MIDI"]);
  } else if (activeFocus === "mixer") {
    rows.push(["播放状态", statusLabel(transport.play_state ?? transport.state ?? transport.status, "空闲")]);
    rows.push(["速度", `${textValue(transport.bpm ?? transport.tempo, "--")} BPM`]);
  }
  return (
    <section className="focus-summary">
      {rows.map(([label, value]) => (
        <div key={label}>
          <span>{label}</span>
          <strong>{value}</strong>
        </div>
      ))}
    </section>
  );
}

function SettingsPage({
  runtimeStatus,
  uiState,
  onClose
}: {
  runtimeStatus: RuntimeStatusResponse | null;
  uiState: AgentUIState | null;
  onClose: () => void;
}) {
  const [activeTab, setActiveTab] = useState<SettingsTab>("llm");
  const [config, setConfig] = useState<EngineConfig>(() => defaultSettingsConfig());
  const [configResponse, setConfigResponse] = useState<AgentConfigResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [status, setStatus] = useState("");

  const loadConfig = useCallback(async () => {
    setLoading(true);
    setStatus("读取配置中...");
    try {
      const response = await fetchAgentConfig();
      setConfigResponse(response);
      setConfig(mergeSettingsConfig(response.config));
      setStatus(response.path ? `配置文件：${response.path}` : "配置已读取");
    } catch (configError) {
      setStatus(configError instanceof Error ? configError.message : "配置读取失败");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadConfig();
  }, [loadConfig]);

  const saveConfig = async () => {
    setSaving(true);
    setStatus("保存中...");
    try {
      const response = await saveAgentConfig(config);
      setConfigResponse(response);
      setConfig(mergeSettingsConfig(response.config));
      setStatus("已保存。下一条消息会使用新配置。");
    } catch (saveError) {
      setStatus(saveError instanceof Error ? saveError.message : "保存失败");
    } finally {
      setSaving(false);
    }
  };

  const setConfigField = <K extends keyof EngineConfig>(key: K, value: EngineConfig[K]) => {
    setConfig((current) => ({ ...current, [key]: value }));
  };

  const updateRoute = (routeKey: string, patch: Partial<MultimodalRouteConfig>) => {
    setConfig((current) => ({
      ...current,
      multimodalRoutes: {
        ...(current.multimodalRoutes ?? {}),
        [routeKey]: {
          ...(current.multimodalRoutes?.[routeKey] ?? {}),
          ...patch
        }
      }
    }));
  };

  const browser = config.browser ?? {};
  const kernel = asRecord(runtimeStatus?.kernel);
  const project = asRecord(uiState?.project);
  const diagnosticsRows = buildDiagnosticsRows(runtimeStatus, uiState);
  const diagnosticsPayload = buildDiagnosticsPayload(runtimeStatus, uiState, configResponse);

  return (
    <section className="settings-page">
      <div className="settings-header">
        <div>
          <p className="eyebrow">Ask Vit</p>
          <h1>设置</h1>
        </div>
        <div className="settings-actions">
          <button className="tool-button" type="button" onClick={() => void loadConfig()} disabled={loading || saving}>
            <RefreshCw size={16} />
            <span>重载</span>
          </button>
          <button className="tool-button primary" type="button" onClick={() => void saveConfig()} disabled={loading || saving}>
            {saving ? <Loader2 className="spin" size={16} /> : <CheckCircle2 size={16} />}
            <span>保存</span>
          </button>
          <button className="icon-button" type="button" title="关闭设置" onClick={onClose}>
            <X size={16} />
          </button>
        </div>
      </div>

      <div className="settings-grid">
        <nav className="settings-nav" aria-label="Settings sections">
          {settingsTabs.map(({ key, label, icon: Icon }) => (
            <button key={key} type="button" className={activeTab === key ? "active" : ""} onClick={() => setActiveTab(key)}>
              <Icon size={16} />
              <span>{label}</span>
            </button>
          ))}
        </nav>

        <div className="settings-body">
          {activeTab === "agent" && (
            <section className="settings-section">
              <h2>Agent</h2>
              <InfoGrid
                rows={[
                  ["服务", runtimeStatus?.service ?? "VitAgent"],
                  ["PID", textValue(runtimeStatus?.pid, "-")],
                  ["内核", Boolean(kernel.connected) ? "已连接" : "未连接"],
                  ["工程", lastPathPart(textValue(project.project_path, "Vit Project"))]
                ]}
              />
              <KeyValue label="配置" value={configResponse?.path ?? "~/.vit/config.json"} />
              <KeyValue label="状态" value={statusLabel(status, status || "空闲")} />
            </section>
          )}

          {activeTab === "llm" && (
            <section className="settings-section">
              <h2>LLM API</h2>
              <SettingsTextField
                label="Base URL"
                value={config.baseUrl ?? ""}
                placeholder="https://api.openai.com/v1"
                onChange={(value) => setConfigField("baseUrl", value)}
              />
              <SettingsTextField
                label="API Key"
                value={config.apiKey ?? ""}
                type="password"
                placeholder={configResponse?.hasApiKey ? "API Key 已保存；留空保持不变" : "API Key"}
                onChange={(value) => setConfigField("apiKey", value)}
              />
              <SettingsTextField
                label="默认模型"
                value={config.defaultModel ?? ""}
                placeholder="渚涘簲鍟嗘ā鍨?ID"
                onChange={(value) => setConfigField("defaultModel", value)}
              />
            </section>
          )}

          {activeTab === "multimodal" && (
            <section className="settings-section">
              <h2>多模态路由</h2>
              <div className="route-list">
                {multimodalRouteMeta.map((meta) => (
                  <RouteEditor
                    key={meta.key}
                    meta={meta}
                    route={config.multimodalRoutes?.[meta.key] ?? {}}
                    hasAPIKey={Boolean(configResponse?.routeApiKeys?.[meta.key])}
                    onChange={(patch) => updateRoute(meta.key, patch)}
                  />
                ))}
              </div>
            </section>
          )}

          {activeTab === "browser" && (
            <section className="settings-section">
              <h2>浏览器</h2>
              <SettingsCheckboxField
                label="启用 WebView2 companion"
                checked={Boolean(browser.enabled)}
                onChange={(checked) => setConfigField("browser", { ...browser, enabled: checked })}
              />
              <SettingsTextField
                label="模式"
                value={browser.mode ?? "webview2_companion"}
                onChange={(value) => setConfigField("browser", { ...browser, mode: value })}
              />
              <SettingsTextField
                label="Profile Path"
                value={browser.profilePath ?? ""}
                placeholder="留空使用默认浏览器 profile"
                onChange={(value) => setConfigField("browser", { ...browser, profilePath: value })}
              />
              <SettingsCheckboxField
                label="保留登录状态"
                checked={Boolean(browser.rememberSession)}
                onChange={(checked) => setConfigField("browser", { ...browser, rememberSession: checked })}
              />
            </section>
          )}

          {activeTab === "diagnostics" && (
            <section className="settings-section">
              <h2>诊断</h2>
              <InfoGrid rows={diagnosticsRows} />
              <pre className="settings-json">
                {JSON.stringify(diagnosticsPayload, null, 2)}
              </pre>
            </section>
          )}
        </div>
      </div>
    </section>
  );
}

function SettingsTextField({
  label,
  value,
  placeholder,
  type = "text",
  onChange
}: {
  label: string;
  value: string;
  placeholder?: string;
  type?: string;
  onChange: (value: string) => void;
}) {
  return (
    <label className="settings-field">
      <span>{label}</span>
      <input type={type} value={value} placeholder={placeholder} onChange={(event) => onChange(event.currentTarget.value)} />
    </label>
  );
}

function SettingsCheckboxField({
  label,
  checked,
  onChange
}: {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label className="settings-check">
      <input type="checkbox" checked={checked} onChange={(event) => onChange(event.currentTarget.checked)} />
      <span>{label}</span>
    </label>
  );
}

function RouteEditor({
  meta,
  route,
  hasAPIKey,
  onChange
}: {
  meta: { key: string; label: string; hint: string };
  route: MultimodalRouteConfig;
  hasAPIKey: boolean;
  onChange: (patch: Partial<MultimodalRouteConfig>) => void;
}) {
  return (
    <section className="route-card">
      <div className="route-card-header">
        <div>
          <h3>{meta.label}</h3>
          <span>{meta.hint}</span>
        </div>
        <SettingsCheckboxField label="启用" checked={Boolean(route.enabled)} onChange={(enabled) => onChange({ enabled })} />
      </div>
      <div className="route-fields">
        <SettingsTextField label="Provider" value={route.provider ?? ""} placeholder="default / openai / local" onChange={(provider) => onChange({ provider })} />
        <SettingsTextField label="Model" value={route.model ?? ""} placeholder="model id" onChange={(model) => onChange({ model })} />
        <SettingsTextField label="Base URL" value={route.baseUrl ?? ""} placeholder="留空继承 LLM API" onChange={(baseUrl) => onChange({ baseUrl })} />
        <SettingsTextField
          label="API Key"
          value={route.apiKey ?? ""}
          type="password"
          placeholder={hasAPIKey ? "API Key 已保存；留空保持不变" : "留空继承 LLM API key"}
          onChange={(apiKey) => onChange({ apiKey })}
        />
        <SettingsTextField label="Fallback" value={route.fallback ?? ""} placeholder="fallback route key" onChange={(fallback) => onChange({ fallback })} />
        <label className="settings-field">
          <span>Priority</span>
          <input
            type="number"
            value={route.priority ?? 0}
            onChange={(event) => onChange({ priority: Number(event.currentTarget.value) || 0 })}
          />
        </label>
      </div>
    </section>
  );
}

function TopStatusBar({
  connection,
  healthLabel,
  runtimeStatus,
  uiState,
  conversationTitle,
  onOpenHistory,
  onRefresh,
  onTransportCommand,
  transportBusy
}: {
  connection: ConnectionStatus;
  healthLabel: string;
  runtimeStatus: RuntimeStatusResponse | null;
  uiState: AgentUIState | null;
  conversationTitle: string;
  onOpenHistory: () => void;
  onRefresh: () => Promise<void>;
  onTransportCommand: (tool: string, args?: JsonRecord) => Promise<void>;
  transportBusy: boolean;
}) {
  const [seekDraft, setSeekDraft] = useState("");
  const project = asRecord(uiState?.project);
  const transport = asRecord(uiState?.transport);
  const projectPath = textValue(project.project_path, "Vit Project");
  const projectName = lastPathPart(projectPath);
  const rawPlayState = textValue(transport.play_state ?? transport.state ?? transport.status, connection === "ready" ? "ready" : "offline");
  const playState = statusLabel(rawPlayState, rawPlayState);
  const timecode = textValue(transport.timecode ?? transport.position ?? transport.time, "00:00:00");
  const bpm = textValue(transport.bpm ?? transport.tempo, "--");
  const sampleRate = textValue(transport.sample_rate ?? transport.sampleRate, "--");
  const diagnosticLine = runtimeDiagnosticLine(healthLabel, runtimeStatus, connection);
  const diagnosticTitle = runtimeDiagnosticTitle(runtimeStatus);
  const transportDisabled = connection !== "ready" || transportBusy;
  const submitSeek = () => {
    const seconds = Number(seekDraft);
    if (!Number.isFinite(seconds) || seconds < 0) {
      return;
    }
    void onTransportCommand("transport.seek", { time: seconds });
    setSeekDraft("");
  };

  return (
    <header className="top-status">
      <div className="brand-block">
        <div className="vit-mondrian-mark" aria-label="Vit"><strong>V</strong><i /><b /></div>
        <div className="conversation-identity">
          <button type="button" className="conversation-title-button" title="打开历史树切换对话流" onClick={onOpenHistory}>
            <strong>{conversationTitle}</strong>
            <ChevronRight size={14} />
          </button>
          <span title={`${projectPath} · ${diagnosticTitle}`}><i className={`connection-dot ${connection}`} />{projectName} · {diagnosticLine}</span>
        </div>
      </div>

      <div className="transport-strip" aria-label="Transport">
        <button className="icon-button" type="button" title="Play" disabled={transportDisabled} onClick={() => void onTransportCommand("transport.play")}>
          <Play size={16} />
        </button>
        <button className="icon-button" type="button" title="Return to zero" disabled={transportDisabled} onClick={() => void onTransportCommand("transport.return_to_zero")}>
          <ChevronLeft size={16} />
        </button>
        <button className="icon-button" type="button" title="Stop" disabled={transportDisabled} onClick={() => void onTransportCommand("transport.stop")}>
          <Square size={15} />
        </button>
        <span className="timecode">{timecode}</span>
        <input
          className="transport-seek-input"
          type="number"
          min="0"
          step="0.25"
          value={seekDraft}
          placeholder="sec"
          title="Seek seconds"
          disabled={transportDisabled}
          onChange={(event) => setSeekDraft(event.currentTarget.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              submitSeek();
            }
          }}
          onBlur={() => {
            if (seekDraft.trim()) {
              submitSeek();
            }
          }}
        />
        <span className="mini-meter">
          <i />
          <i />
          <i />
          <i />
          <i />
        </span>
      </div>

      <div className="status-cells">
        <StatusCell icon={Radio} label={playState} tone={connection === "ready" ? "ok" : "warn"} />
        <StatusCell icon={Gauge} label={`${bpm} BPM`} />
        <StatusCell icon={Volume2} label={sampleRate} />
        <button className="icon-button refresh" type="button" title="Refresh" onClick={() => void onRefresh()}>
          <RefreshCw size={16} />
        </button>
      </div>
    </header>
  );
}

type DawInvoke = (tool: string, args?: JsonRecord, source?: string, confirmed?: boolean) => Promise<AgentInvokeResponse>;
type DawActivitySegment = { left: number; width: number; height: number };
type DawMidiClipEntry = { track: DawTrack; clip: DawClip };
type DawMidiGhostClip = DawMidiClipEntry & { notes: JsonRecord[]; offsetBeats: number };
type DawClip = {
  id: string;
  name: string;
  type: string;
  startSeconds: number;
  lengthSeconds: number;
  startBeats: number;
  lengthBeats: number;
  selected: boolean;
  noteCount: number;
  notes: JsonRecord[];
  activity: DawActivitySegment[];
  activityKnown: boolean;
};
type ClipDragState = {
  clipID: string;
  trackID: string;
  targetTrackID: string;
  startSeconds: number;
  startClientX: number;
  startClientY: number;
  deltaX: number;
  deltaY: number;
  moved: boolean;
};
type DawPlugin = {
  id: string;
  itemID: string;
  name: string;
  kind: string;
  role: string;
  clipScope: string;
  selected: boolean;
  bypass: boolean;
  path: string;
  vendor: string;
  source: string;
  deletable: boolean;
};
type DawTrack = {
  id: string;
  name: string;
  type: string;
  color: string;
  selected: boolean;
  mute: boolean;
  solo: boolean;
  armed: boolean;
  volumeDB: number | null;
  pan: number | null;
  levelDB: number | null;
  clips: DawClip[];
  plugins: DawPlugin[];
  rackEdges: JsonRecord[];
};
type RackPluginContextMenuState = { track: DawTrack; plugin: DawPlugin; x: number; y: number };
type RackParamSnapshot = { count: number; labels: string[]; updatedAt: number };

function DawFocusPanel({
  activeFocus,
  uiState,
  connection,
  onInvoke,
  onRefresh,
  onClose
}: {
  activeFocus: Exclude<FocusMode, "dialogue">;
  uiState: AgentUIState | null;
  connection: ConnectionStatus;
  onInvoke: DawInvoke;
  onRefresh: () => Promise<void>;
  onClose: () => void;
}) {
  const tracks = useMemo(() => dawTracksFromUIState(uiState), [uiState]);
  const title = focusTitle(activeFocus);
  const subtitle = dawPanelSubtitle(activeFocus, tracks, uiState);

  return (
    <section className={`daw-focus-panel daw-${activeFocus}`}>
      <header className="daw-panel-header">
        <div>
          <p className="eyebrow">DAW</p>
          <h1>{title}</h1>
          <span>{subtitle}</span>
        </div>
        <div className="daw-panel-actions">
          <button className="ghost-button" type="button" onClick={() => void onRefresh()}>
            <RefreshCw size={15} />
            刷新
          </button>
          <button className="icon-button" type="button" title="收起左侧面板" onClick={onClose}>
            <X size={15} />
          </button>
        </div>
      </header>

      {connection !== "ready" && (
        <div className="notice warning">
          <AlertTriangle size={16} />
          <span>Agent 未连接，DAW 面板暂不可操作。</span>
        </div>
      )}

      {activeFocus === "tracks" && <DawTracksPanel tracks={tracks} onInvoke={onInvoke} />}
      {activeFocus === "rack" && <DawRackPanelV2 tracks={tracks} uiState={uiState} onInvoke={onInvoke} />}
      {activeFocus === "midi" && <DawMidiPanel tracks={tracks} uiState={uiState} onInvoke={onInvoke} />}
      {activeFocus === "mixer" && <DawMixerPanel tracks={tracks} onInvoke={onInvoke} />}
    </section>
  );
}

function DawTracksPanel({ tracks, onInvoke }: { tracks: DawTrack[]; onInvoke: DawInvoke }) {
  const projectEnd = Math.max(8, ...tracks.flatMap((track) => track.clips.map((clip) => clip.startSeconds + clip.lengthSeconds)));
  const timelineEnd = Math.max(8, projectEnd + Math.min(12, Math.max(2, projectEnd * 0.08)));
  const fitPixelsPerSecond = clampNumber(720 / timelineEnd, 0.05, 42);
  const minPixelsPerSecond = Math.min(1.5, fitPixelsPerSecond);
  const [pixelsPerSecond, setPixelsPerSecond] = useState(() => clampNumber(fitPixelsPerSecond, minPixelsPerSecond, 18));
  const [snapSeconds, setSnapSeconds] = useState(0.5);
  const [clipDrag, setClipDrag] = useState<ClipDragState | null>(null);
  const userZoomTouchedRef = useRef(false);
  const suppressClipClickRef = useRef(false);
  useEffect(() => {
    if (!userZoomTouchedRef.current) {
      setPixelsPerSecond(clampNumber(fitPixelsPerSecond, minPixelsPerSecond, 18));
    }
  }, [fitPixelsPerSecond, minPixelsPerSecond]);
  const safePixelsPerSecond = clampNumber(pixelsPerSecond, minPixelsPerSecond, 80);
  const timelineWidth = Math.max(720, Math.ceil(timelineEnd * safePixelsPerSecond));
  const displayEndSeconds = timelineWidth / safePixelsPerSecond;
  const setTimelineZoom = (next: number) => {
    userZoomTouchedRef.current = true;
    setPixelsPerSecond(clampNumber(next, minPixelsPerSecond, 80));
  };
  const beginClipDrag = (event: ReactPointerEvent<HTMLButtonElement>, track: DawTrack, clip: DawClip) => {
    if (event.button !== 0) {
      return;
    }
    event.stopPropagation();
    event.currentTarget.setPointerCapture(event.pointerId);
    suppressClipClickRef.current = false;
    setClipDrag({
      clipID: clip.id,
      trackID: track.id,
      targetTrackID: track.id,
      startSeconds: clip.startSeconds,
      startClientX: event.clientX,
      startClientY: event.clientY,
      deltaX: 0,
      deltaY: 0,
      moved: false
    });
  };
  const updateClipDrag = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (!clipDrag) {
      return;
    }
    const deltaX = event.clientX - clipDrag.startClientX;
    const moved = clipDrag.moved || Math.abs(deltaX) >= 4;
    if (moved) {
      event.stopPropagation();
      suppressClipClickRef.current = true;
    }
    setClipDrag({
      ...clipDrag,
      deltaX,
      deltaY: event.clientY - clipDrag.startClientY,
      targetTrackID: trackIDFromPointer(event.clientX, event.clientY) || clipDrag.targetTrackID,
      moved
    });
  };
  const finishClipDrag = (event: ReactPointerEvent<HTMLButtonElement>, track: DawTrack, clip: DawClip) => {
    if (!clipDrag || clipDrag.clipID !== clip.id) {
      return;
    }
    event.stopPropagation();
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    const targetTrackID = trackIDFromPointer(event.clientX, event.clientY) || clipDrag.targetTrackID || track.id;
    const nextStart = draggedClipStart(clipDrag, safePixelsPerSecond, snapSeconds);
    const didMove = clipDrag.moved && Math.abs(nextStart - clipDrag.startSeconds) >= 0.02;
    const changedTrack = targetTrackID !== clipDrag.trackID;
    setClipDrag(null);
    if (!didMove && !changedTrack) {
      return;
    }
    suppressClipClickRef.current = true;
    void onInvoke("clip.move", {
      source_track_id: clipDrag.trackID,
      target_track_id: targetTrackID,
      track_id: targetTrackID,
      clip_id: clip.id,
      new_start: nextStart,
      new_start_seconds: nextStart,
      time_unit: "seconds"
    }, "ask_vit_webui_tracks");
  };
  const cancelClipDrag = () => {
    setClipDrag(null);
  };
  return (
    <div className="daw-panel-body">
      <div className="daw-toolbar">
        <button className="primary-button compact" type="button" onClick={() => void onInvoke("track.add", {}, "ask_vit_webui_tracks")}>
          <Plus size={15} />
          新建轨道
        </button>
        <div className="timeline-zoom-controls" aria-label="Timeline zoom">
          <button type="button" title="Zoom out" onClick={() => setTimelineZoom(safePixelsPerSecond / 1.25)}>
            <ZoomOut size={14} />
          </button>
          <input
            type="range"
            min={minPixelsPerSecond}
            max="80"
            step={minPixelsPerSecond < 1 ? 0.05 : 0.5}
            value={safePixelsPerSecond}
            title="Timeline zoom"
            onChange={(event) => setTimelineZoom(Number(event.currentTarget.value))}
          />
          <button type="button" title="Zoom in" onClick={() => setTimelineZoom(safePixelsPerSecond * 1.25)}>
            <ZoomIn size={14} />
          </button>
          <button type="button" title="Fit timeline" onClick={() => setTimelineZoom(fitPixelsPerSecond)}>
            <Maximize2 size={14} />
          </button>
          <button type="button" title="Compact overview" onClick={() => setTimelineZoom(minPixelsPerSecond)}>
            <Minus size={14} />
          </button>
        </div>
        <select
          className="timeline-snap-select"
          value={snapSeconds}
          title="Snap"
          onChange={(event) => setSnapSeconds(Number(event.currentTarget.value))}
        >
          <option value={0}>Snap off</option>
          <option value={0.25}>1/4s</option>
          <option value={0.5}>1/2s</option>
          <option value={1}>1s</option>
          <option value={5}>5s</option>
        </select>
        <span>{tracks.length} tracks</span>
      </div>
      {tracks.length === 0 ? (
        <EmptyState label="当前工程没有可编辑轨道" />
      ) : (
        <div className="daw-timeline" style={{ "--timeline-width": `${timelineWidth}px` } as React.CSSProperties}>
          <div className="timeline-ruler">
            <span>0s</span>
            <span>{formatTimelineSeconds(displayEndSeconds / 2)}</span>
            <span>{formatTimelineSeconds(displayEndSeconds)}</span>
          </div>
          {tracks.map((track) => (
            <div className={`daw-track-lane ${track.selected ? "selected" : ""} ${clipDrag?.moved && clipDrag.targetTrackID === track.id ? "drop-target" : ""}`} key={track.id}>
              <TrackLaneHeader track={track} onInvoke={onInvoke} />
              <div className="track-lane-canvas" data-track-id={track.id} onClick={() => void focusDawTarget(onInvoke, { track_id: track.id })}>
                {track.clips.length === 0 && <span className="lane-empty">No clips</span>}
                {track.clips.map((clip) => (
                  <button
                    className={`clip-block ${clip.selected ? "selected" : ""} ${clip.activityKnown ? "" : "unknown-activity"} ${clipDrag?.clipID === clip.id ? "dragging" : ""}`}
                    key={clip.id}
                    type="button"
                    style={clipBlockStyle(clip, safePixelsPerSecond, timelineWidth, clipDrag?.clipID === clip.id ? clipDrag.deltaX : 0, clipDrag?.clipID === clip.id ? clipDrag.deltaY : 0)}
                    title={clipTitle(track, clip)}
                    onPointerDown={(event) => beginClipDrag(event, track, clip)}
                    onPointerMove={updateClipDrag}
                    onPointerUp={(event) => finishClipDrag(event, track, clip)}
                    onPointerCancel={cancelClipDrag}
                    onClick={(event) => {
                      event.stopPropagation();
                      if (suppressClipClickRef.current) {
                        suppressClipClickRef.current = false;
                        return;
                      }
                      void focusDawTarget(onInvoke, { track_id: track.id, clip_id: clip.id, start_seconds: clip.startSeconds });
                    }}
                  >
                    <span>{clip.name}</span>
                    {clipDrag?.clipID === clip.id && clipDrag.moved && <em>{formatTimelineSeconds(draggedClipStart(clipDrag, safePixelsPerSecond, snapSeconds))}</em>}
                    <ClipActivityStrip clip={clip} />
                  </button>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function TrackLaneHeader({ track, onInvoke }: { track: DawTrack; onInvoke: DawInvoke }) {
  return (
    <div className="track-lane-header">
      <button className="track-name-button" type="button" onClick={() => void focusDawTarget(onInvoke, { track_id: track.id })}>
        <span className="track-color" style={{ background: track.color }} />
        <strong>{track.name}</strong>
        <small>{track.type}</small>
      </button>
      <div className="track-mini-actions">
        <button className={track.mute ? "toggle on red" : "toggle"} type="button" onClick={() => void onInvoke("track.mute", { track_id: track.id, mute: !track.mute }, "ask_vit_webui_tracks")}>M</button>
        <button className={track.solo ? "toggle on yellow" : "toggle"} type="button" onClick={() => void onInvoke("track.solo", { track_id: track.id, solo: !track.solo }, "ask_vit_webui_tracks")}>S</button>
        <button className={track.armed ? "toggle on blue" : "toggle"} type="button" onClick={() => void onInvoke("track.arm", { track_id: track.id, is_armed: !track.armed }, "ask_vit_webui_tracks")}>R</button>
      </div>
    </div>
  );
}

function ClipActivityStrip({ clip }: { clip: DawClip }) {
  if (!clip.activityKnown) {
    return <i className="clip-activity unknown" />;
  }
  return (
    <i className="clip-activity">
      {clip.activity.map((segment, index) => (
        <b
          key={`${clip.id}-activity-${index}`}
          style={{
            left: `${segment.left}%`,
            width: `${segment.width}%`,
            height: `${segment.height}%`
          }}
        />
      ))}
    </i>
  );
}

function DawRackPanel({ tracks, uiState, onInvoke }: { tracks: DawTrack[]; uiState: AgentUIState | null; onInvoke: DawInvoke }) {
  const selectedPlugin = selectedPluginFromUIState(uiState);
  return (
    <div className="daw-panel-body rack-lanes">
      {tracks.length === 0 && <EmptyState label="当前工程没有可显示的机架轨道" />}
      {tracks.map((track) => {
        const lanes = rackLanesForTrack(track);
        return (
          <section className={`rack-track-row ${track.selected ? "selected" : ""}`} key={track.id}>
            <button className="rack-track-title" type="button" onClick={() => void focusDawTarget(onInvoke, { track_id: track.id })}>
              <span className="track-color" style={{ background: track.color }} />
              <strong>{track.name}</strong>
              <small>{track.plugins.length} plugins</small>
            </button>
            <RackPluginLane label="Track Chain" plugins={lanes.trackChain} track={track} onInvoke={onInvoke} />
            {lanes.parallel.length > 0 && <RackPluginLane label="Parallel" plugins={lanes.parallel} track={track} onInvoke={onInvoke} />}
            {lanes.clipFx.length > 0 && <RackPluginLane label="Clip FX" plugins={lanes.clipFx} track={track} onInvoke={onInvoke} clipLane />}
          </section>
        );
      })}
      {selectedPlugin.id && (
        <aside className="rack-inspector">
          <div>
            <span>Selected plugin</span>
            <strong>{selectedPlugin.name || selectedPlugin.id}</strong>
          </div>
          <button type="button" onClick={() => void onInvoke("plugin.open", { track_id: selectedPlugin.trackID, plugin_id: selectedPlugin.id }, "ask_vit_webui_rack")}>Open UI</button>
          <button type="button" onClick={() => void onInvoke("plugin.get_parameters", { track_id: selectedPlugin.trackID, plugin_id: selectedPlugin.id }, "ask_vit_webui_rack")}>Get Params</button>
        </aside>
      )}
    </div>
  );
}

function RackPluginLane({ label, plugins, track, onInvoke, clipLane = false }: { label: string; plugins: DawPlugin[]; track: DawTrack; onInvoke: DawInvoke; clipLane?: boolean }) {
  return (
    <div className={`rack-plugin-lane ${clipLane ? "clip-fx" : ""}`}>
      <span className="rack-lane-label">{label}</span>
      <div className="rack-plugin-chain">
        {plugins.length === 0 ? (
          <span className="rack-empty">Empty</span>
        ) : (
          plugins.map((plugin, index) => (
            <div className="rack-plugin-wrap" key={plugin.id || `${track.id}-plugin-${index}`}>
              {index > 0 && <i className="rack-connector" />}
              <article className={`rack-plugin-card ${plugin.selected ? "selected" : ""} ${plugin.bypass ? "bypassed" : ""}`}>
                <button className="rack-plugin-main" type="button" onClick={() => void focusDawTarget(onInvoke, { track_id: track.id, plugin_id: plugin.id })}>
                  <strong>{plugin.name}</strong>
                  <span>{plugin.clipScope === "track" ? plugin.role || plugin.kind : plugin.clipScope}</span>
                </button>
                <div>
                  <button type="button" onClick={() => void onInvoke("plugin.open", { track_id: track.id, plugin_id: plugin.id }, "ask_vit_webui_rack")}>Open UI</button>
                  <button type="button" onClick={() => void onInvoke("plugin.get_parameters", { track_id: track.id, plugin_id: plugin.id }, "ask_vit_webui_rack")}>Get Params</button>
                </div>
              </article>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

function DawRackPanelV2({ tracks, uiState, onInvoke }: { tracks: DawTrack[]; uiState: AgentUIState | null; onInvoke: DawInvoke }) {
  const selectedPlugin = selectedPluginFromUIState(uiState);
  const [contextMenu, setContextMenu] = useState<RackPluginContextMenuState | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<{ track: DawTrack; plugin: DawPlugin } | null>(null);
  const [busyPluginKey, setBusyPluginKey] = useState("");
  const [rackMessage, setRackMessage] = useState("");
  const [paramSnapshots, setParamSnapshots] = useState<Record<string, RackParamSnapshot>>({});
  const selectedPluginEntry = tracks
    .flatMap((track) => track.plugins.map((plugin) => ({ track, plugin })))
    .find(({ track, plugin }) => plugin.id === selectedPlugin.id && (!selectedPlugin.trackID || selectedPlugin.trackID === track.id || selectedPlugin.trackID === plugin.source));

  useEffect(() => {
    if (!contextMenu) {
      return undefined;
    }
    const close = () => setContextMenu(null);
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        close();
      }
    };
    window.addEventListener("click", close);
    window.addEventListener("contextmenu", close);
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("click", close);
      window.removeEventListener("contextmenu", close);
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [contextMenu]);

  const copyRackValue = async (label: string, value: string) => {
    const text = value.trim();
    if (!text) {
      setRackMessage(`${label}不可用`);
      return;
    }
    try {
      await navigator.clipboard.writeText(text);
      setRackMessage(`已复制${label}`);
    } catch {
      setRackMessage(`无法复制${label}`);
    }
    setContextMenu(null);
  };

  const invokeRackPlugin = async (tool: string, track: DawTrack, plugin: DawPlugin) => {
    const key = rackPluginKey(track.id, plugin.id);
    setBusyPluginKey(key);
    setRackMessage("");
    try {
      const args = tool === "plugin.get_parameters"
        ? { ...rackPluginArgs(track, plugin), include_parameters: true }
        : rackPluginArgs(track, plugin);
      const response = await onInvoke(tool, args, "ask_vit_webui_rack", true);
      if (tool === "plugin.get_parameters") {
        const snapshot = rackParamSnapshotFromInvoke(response);
        setParamSnapshots((current) => ({ ...current, [key]: snapshot }));
        setRackMessage(snapshot.count > 0 ? `已读取 ${plugin.name} 的 ${snapshot.count} 个参数` : `${plugin.name} 暂无可显示参数`);
      } else if (tool === "plugin.delete") {
        setRackMessage(`已删除 ${plugin.name}`);
      }
      return response;
    } catch (error) {
      setRackMessage(error instanceof Error ? error.message : "机架操作失败");
    } finally {
      setBusyPluginKey("");
    }
  };

  const openPlugin = (track: DawTrack, plugin: DawPlugin) => {
    void invokeRackPlugin("plugin.open", track, plugin);
  };

  const readPluginParams = (track: DawTrack, plugin: DawPlugin) => {
    void invokeRackPlugin("plugin.get_parameters", track, plugin);
  };

  const confirmDeletePlugin = async () => {
    if (!deleteTarget) {
      return;
    }
    const target = deleteTarget;
    setDeleteTarget(null);
    await invokeRackPlugin("plugin.delete", target.track, target.plugin);
  };

  return (
    <div className="daw-panel-body rack-lanes">
      {tracks.length === 0 && <EmptyState label="当前工程没有可显示的机架轨道" />}
      {tracks.map((track) => {
        const lanes = rackLanesForTrack(track);
        return (
          <section className={`rack-track-row ${track.selected ? "selected" : ""}`} key={track.id}>
            <button className="rack-track-title" type="button" onClick={() => void focusDawTarget(onInvoke, { track_id: track.id })}>
              <span className="track-color" style={{ background: track.color }} />
              <strong>{track.name}</strong>
              <small>{track.plugins.length} plugins</small>
            </button>
            <RackPluginLaneV2
              label="Track Chain"
              plugins={lanes.trackChain}
              track={track}
              onInvoke={onInvoke}
              onOpenPlugin={openPlugin}
              onReadParams={readPluginParams}
              onContextMenu={setContextMenu}
              busyPluginKey={busyPluginKey}
              paramSnapshots={paramSnapshots}
            />
            {lanes.parallel.length > 0 && (
              <RackPluginLaneV2
                label="Parallel"
                plugins={lanes.parallel}
                track={track}
                onInvoke={onInvoke}
                onOpenPlugin={openPlugin}
                onReadParams={readPluginParams}
                onContextMenu={setContextMenu}
                busyPluginKey={busyPluginKey}
                paramSnapshots={paramSnapshots}
              />
            )}
            {lanes.clipFx.length > 0 && (
              <RackPluginLaneV2
                label="Clip FX"
                plugins={lanes.clipFx}
                track={track}
                onInvoke={onInvoke}
                onOpenPlugin={openPlugin}
                onReadParams={readPluginParams}
                onContextMenu={setContextMenu}
                busyPluginKey={busyPluginKey}
                paramSnapshots={paramSnapshots}
                clipLane
              />
            )}
          </section>
        );
      })}
      {selectedPluginEntry && (
        <aside className="rack-inspector">
          <div>
            <span>Selected plugin</span>
            <strong>{selectedPluginEntry.plugin.name || selectedPluginEntry.plugin.id}</strong>
          </div>
          <button type="button" disabled={busyPluginKey === rackPluginKey(selectedPluginEntry.track.id, selectedPluginEntry.plugin.id)} onClick={() => openPlugin(selectedPluginEntry.track, selectedPluginEntry.plugin)}>Open UI</button>
          <button type="button" disabled={busyPluginKey === rackPluginKey(selectedPluginEntry.track.id, selectedPluginEntry.plugin.id)} onClick={() => readPluginParams(selectedPluginEntry.track, selectedPluginEntry.plugin)}>Get Params</button>
        </aside>
      )}
      {rackMessage && <div className="rack-status-note">{rackMessage}</div>}
      {contextMenu && (
        <div
          className="media-context-menu rack-context-menu"
          style={{ left: contextMenu.x, top: contextMenu.y }}
          role="menu"
          onClick={(event) => event.stopPropagation()}
          onContextMenu={(event) => {
            event.preventDefault();
            event.stopPropagation();
          }}
        >
          <button type="button" role="menuitem" onClick={() => void copyRackValue("插件引用", rackPluginReference(contextMenu.track, contextMenu.plugin))}>
            引用插件
          </button>
          <button type="button" role="menuitem" disabled={!contextMenu.plugin.path} onClick={() => void copyRackValue("插件路径", contextMenu.plugin.path)}>
            复制路径
          </button>
          <button type="button" role="menuitem" onClick={() => void copyRackValue("插件 ID", contextMenu.plugin.id)}>
            复制 ID
          </button>
          <div className="media-menu-separator" />
          <button type="button" role="menuitem" onClick={() => {
            openPlugin(contextMenu.track, contextMenu.plugin);
            setContextMenu(null);
          }}>
            Open UI
          </button>
          <button type="button" role="menuitem" onClick={() => {
            readPluginParams(contextMenu.track, contextMenu.plugin);
            setContextMenu(null);
          }}>
            Get Params
          </button>
          <div className="media-menu-separator" />
          <button
            className="danger"
            type="button"
            role="menuitem"
            disabled={!contextMenu.plugin.deletable}
            onClick={() => {
              setDeleteTarget({ track: contextMenu.track, plugin: contextMenu.plugin });
              setContextMenu(null);
            }}
          >
            删除插件
          </button>
        </div>
      )}
      {deleteTarget && (
        <div className="media-dialog-backdrop rack-dialog-backdrop" onClick={() => setDeleteTarget(null)}>
          <div className="media-dialog rack-delete-dialog" onClick={(event) => event.stopPropagation()}>
            <h3>删除插件</h3>
            <p>{deleteTarget.plugin.name}</p>
            <span>这会从 {deleteTarget.track.name} 的机架链中移除该插件实例。</span>
            <div className="media-dialog-actions">
              <button type="button" onClick={() => setDeleteTarget(null)} disabled={Boolean(busyPluginKey)}>
                取消
              </button>
              <button type="button" className="danger" onClick={() => void confirmDeletePlugin()} disabled={Boolean(busyPluginKey)}>
                删除
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function RackPluginLaneV2({
  label,
  plugins,
  track,
  onInvoke,
  onOpenPlugin,
  onReadParams,
  onContextMenu,
  busyPluginKey,
  paramSnapshots,
  clipLane = false
}: {
  label: string;
  plugins: DawPlugin[];
  track: DawTrack;
  onInvoke: DawInvoke;
  onOpenPlugin: (track: DawTrack, plugin: DawPlugin) => void;
  onReadParams: (track: DawTrack, plugin: DawPlugin) => void;
  onContextMenu: (state: RackPluginContextMenuState) => void;
  busyPluginKey: string;
  paramSnapshots: Record<string, RackParamSnapshot>;
  clipLane?: boolean;
}) {
  return (
    <div className={`rack-plugin-lane ${clipLane ? "clip-fx" : ""}`}>
      <span className="rack-lane-label">{label}</span>
      <div className="rack-plugin-chain">
        {plugins.length === 0 ? (
          <span className="rack-empty">Empty</span>
        ) : (
          plugins.map((plugin, index) => {
            const key = rackPluginKey(track.id, plugin.id);
            const snapshot = paramSnapshots[key];
            return (
              <div className="rack-plugin-wrap" key={plugin.id || `${track.id}-plugin-${index}`}>
                {index > 0 && <i className="rack-connector" />}
                <article
                  className={`rack-plugin-card ${plugin.selected ? "selected" : ""} ${plugin.bypass ? "bypassed" : ""}`}
                  onContextMenu={(event) => {
                    event.preventDefault();
                    event.stopPropagation();
                    onContextMenu({ track, plugin, x: event.clientX, y: event.clientY });
                  }}
                >
                  <button
                    className="rack-plugin-menu-button"
                    type="button"
                    title="插件操作"
                    onClick={(event) => {
                      event.stopPropagation();
                      const rect = event.currentTarget.getBoundingClientRect();
                      onContextMenu({ track, plugin, x: rect.left, y: rect.bottom + 4 });
                    }}
                  >
                    <MoreHorizontal size={15} />
                  </button>
                  <button className="rack-plugin-main" type="button" onClick={() => void focusDawTarget(onInvoke, { track_id: track.id, plugin_id: plugin.id })}>
                    <strong>{plugin.name}</strong>
                    <span>{plugin.vendor || (plugin.clipScope === "track" ? plugin.role || plugin.kind : plugin.clipScope)}</span>
                  </button>
                  <div>
                    <button type="button" disabled={busyPluginKey === key} onClick={() => onOpenPlugin(track, plugin)}>Open UI</button>
                    <button type="button" disabled={busyPluginKey === key} onClick={() => onReadParams(track, plugin)}>Get Params</button>
                  </div>
                  {snapshot && <small className="rack-param-summary">{rackParamSnapshotText(snapshot)}</small>}
                </article>
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}

function DawMidiPanel({ tracks, uiState, onInvoke }: { tracks: DawTrack[]; uiState: AgentUIState | null; onInvoke: DawInvoke }) {
  const midiClips = tracks.flatMap((track) => track.clips.filter((clip) => isMidiClip(clip)).map((clip) => ({ track, clip })));
  const [notesByClip, setNotesByClip] = useState<Record<string, JsonRecord[]>>({});
  const [noteReadErrorsByClip, setNoteReadErrorsByClip] = useState<Record<string, string>>({});
  const [loadingClipIDs, setLoadingClipIDs] = useState<Record<string, boolean>>({});
  const [hiddenGhostTrackIDs, setHiddenGhostTrackIDs] = useState<Record<string, boolean>>({});
  const [localSelectedClipID, setLocalSelectedClipID] = useState("");
  const onInvokeRef = useRef(onInvoke);
  const loadingClipIDsRef = useRef<Set<string>>(new Set());
  const mountedRef = useRef(true);
  const lastContextFocusRef = useRef("");
  const midiClipIDsKey = midiClips.map(({ track, clip }) => `${track.id}:${clip.id}`).join("|");
  const displayMidiTracks = tracks
    .map((track) => ({
      track,
      clips: track.clips.filter((clip) => isMidiClip(clip)).map((clip) => midiClipWithCachedNotes(clip, notesByClip))
    }))
    .filter((entry) => entry.clips.length > 0);
  const displayMidiClips = displayMidiTracks.flatMap(({ track, clips }) => clips.map((clip) => ({ track, clip })));
  const contextSelectedClipID = selectedMidiClipIDFromUIState(uiState);
  const contextSelectedTrackID = selectedMidiTrackIDFromUIState(uiState);
  const contextFocusKey = `${contextSelectedTrackID}::${contextSelectedClipID}`;
  const localSelected = displayMidiClips.find((entry) => entry.clip.id === localSelectedClipID) ?? null;
  const contextSelected = contextSelectedClipID ? displayMidiClips.find((entry) => entry.clip.id === contextSelectedClipID) ?? null : null;
  const contextTrackSelected = contextSelectedTrackID ? displayMidiClips.find((entry) => entry.track.id === contextSelectedTrackID) ?? null : null;
  const selected = localSelected ?? contextSelected ?? contextTrackSelected ?? displayMidiClips[0] ?? null;
  const currentTrackClips = selected ? displayMidiTracks.find((entry) => entry.track.id === selected.track.id)?.clips ?? [] : [];
  const ghostClips: DawMidiGhostClip[] = selected
    ? displayMidiTracks
      .filter(({ track }) => track.id !== selected.track.id && !hiddenGhostTrackIDs[track.id])
      .map(({ track, clips }) => {
        const clip = midiClipIntersectingWindow(clips, selected.clip);
        if (!clip) {
          return null;
        }
        const notes = clip.notes.length > 0 ? clip.notes : notesByClip[clip.id] ?? [];
        return { track, clip, notes, offsetBeats: clip.startBeats - selected.clip.startBeats };
      })
      .filter((entry): entry is DawMidiGhostClip => entry != null)
    : [];
  const selectedNotes = selected ? (selected.clip.notes.length > 0 ? selected.clip.notes : notesByClip[selected.clip.id] ?? []) : [];
  const selectedReadError = selected ? noteReadErrorsByClip[selected.clip.id] ?? "" : "";
  const selectedClipLoadID = selected?.clip.id ?? "";
  const selectedTrackLoadID = selected?.track.id ?? "";
  const selectedInlineNoteCount = selected?.clip.notes.length ?? 0;
  const selectedCachedNotes = selectedClipLoadID ? notesByClip[selectedClipLoadID] : undefined;
  const ghostLoadKey = ghostClips
    .map(({ track, clip }) => `${track.id}:${clip.id}:${clip.notes.length}:${midiClipHasLoadedNotes(notesByClip, clip.id) ? "loaded" : "pending"}`)
    .join("|");

  useEffect(() => {
    onInvokeRef.current = onInvoke;
  }, [onInvoke]);

  useEffect(() => {
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const readMidiClipNotes = useCallback((clipID: string, trackID: string) => {
    if (!clipID || loadingClipIDsRef.current.has(clipID)) {
      return;
    }
    loadingClipIDsRef.current.add(clipID);
    setLoadingClipIDs((current) => ({ ...current, [clipID]: true }));
    setNoteReadErrorsByClip((current) => {
      if (!current[clipID]) {
        return current;
      }
      const next = { ...current };
      delete next[clipID];
      return next;
    });
    void onInvokeRef.current("midi.read_clip_notes", { clip_id: clipID, track_id: trackID }, "ask_vit_webui_midi", true)
      .then((response) => {
        const result = asRecord(response.result);
        const responseStatus = textValue(response.status, "").toLowerCase();
        const resultStatus = textValue(result.status, "").toLowerCase();
        if ((responseStatus && responseStatus !== "ok" && responseStatus !== "success") || resultStatus === "error") {
          throw new Error(textValue(response.error ?? result.error ?? result.message, "Unable to read MIDI notes"));
        }
        const notes = midiReadNotesFromResult(result);
        if (mountedRef.current) {
          setNotesByClip((current) => ({ ...current, [clipID]: notes }));
        }
      })
      .catch((error: unknown) => {
        if (mountedRef.current) {
          const message = error instanceof Error ? error.message : "Unable to read MIDI notes";
          setNoteReadErrorsByClip((current) => ({ ...current, [clipID]: message }));
          setNotesByClip((current) => ({ ...current, [clipID]: [] }));
        }
      })
      .finally(() => {
        loadingClipIDsRef.current.delete(clipID);
        if (mountedRef.current) {
          setLoadingClipIDs((current) => {
            if (!current[clipID]) {
              return current;
            }
            const next = { ...current };
            delete next[clipID];
            return next;
          });
        }
      });
  }, []);

  useEffect(() => {
    if (contextFocusKey === lastContextFocusRef.current) {
      return;
    }
    lastContextFocusRef.current = contextFocusKey;
    const next =
      (contextSelectedClipID ? displayMidiClips.find((entry) => entry.clip.id === contextSelectedClipID) ?? null : null) ??
      (contextSelectedTrackID ? displayMidiClips.find((entry) => entry.track.id === contextSelectedTrackID) ?? null : null);
    if (next) {
      setLocalSelectedClipID(next.clip.id);
    }
  }, [contextFocusKey, contextSelectedClipID, contextSelectedTrackID, displayMidiClips]);

  useEffect(() => {
    if (localSelectedClipID && !midiClips.some((entry) => entry.clip.id === localSelectedClipID)) {
      setLocalSelectedClipID("");
    }
  }, [localSelectedClipID, midiClips, midiClipIDsKey]);

  useEffect(() => {
    if (!selectedClipLoadID || selectedInlineNoteCount > 0 || selectedCachedNotes || loadingClipIDsRef.current.has(selectedClipLoadID)) {
      return;
    }
    readMidiClipNotes(selectedClipLoadID, selectedTrackLoadID);
  }, [readMidiClipNotes, selectedCachedNotes, selectedClipLoadID, selectedInlineNoteCount, selectedTrackLoadID]);

  useEffect(() => {
    ghostClips.forEach(({ track, clip }) => {
      if (clip.notes.length > 0 || midiClipHasLoadedNotes(notesByClip, clip.id) || loadingClipIDsRef.current.has(clip.id)) {
        return;
      }
      readMidiClipNotes(clip.id, track.id);
    });
  }, [ghostLoadKey, ghostClips, notesByClip, readMidiClipNotes]);

  const focusMidiClip = (track: DawTrack, clip: DawClip) => {
    setLocalSelectedClipID(clip.id);
    void focusDawTarget(onInvoke, { track_id: track.id, clip_id: clip.id, start_seconds: clip.startSeconds, start_beats: clip.startBeats });
  };

  const activateTrack = (track: DawTrack, clips: DawClip[]) => {
    const clip = midiClipForTrackActivation(clips, selected?.clip ?? null);
    if (clip) {
      focusMidiClip(track, clip);
    }
  };

  const toggleGhostTrack = (trackID: string) => {
    setHiddenGhostTrackIDs((current) => {
      const next = { ...current };
      if (next[trackID]) {
        delete next[trackID];
      } else {
        next[trackID] = true;
      }
      return next;
    });
  };

  return (
    <div className="daw-panel-body midi-workspace">
      <aside className="midi-track-list">
        <h2>Tracks</h2>
        {displayMidiTracks.map(({ track, clips }) => {
          const active = selected?.track.id === track.id;
          const ghostVisible = !active && !hiddenGhostTrackIDs[track.id];
          const targetClip = midiClipForTrackActivation(clips, selected?.clip ?? null);
          return (
            <div className={active ? "midi-track-row active" : "midi-track-row"} key={track.id}>
              <button
                className="midi-ghost-toggle"
                disabled={active}
                title={active ? "Current track" : ghostVisible ? "Hide ghost notes" : "Show ghost notes"}
                type="button"
                onClick={() => toggleGhostTrack(track.id)}
              >
                {ghostVisible || active ? <Eye size={15} /> : <EyeOff size={15} />}
              </button>
              <button className="midi-track-main" disabled={!targetClip} type="button" onClick={() => activateTrack(track, clips)}>
                <span className="track-color" style={{ background: track.color }} />
                <strong>{track.name}</strong>
                <small>{active ? "Current" : ghostVisible ? "Ghost" : "Hidden"} / {clips.length} clips</small>
              </button>
            </div>
          );
        })}
        {midiClips.length === 0 && <EmptyState label="当前工程没有 MIDI clip" />}
        {false && displayMidiClips.map(({ track, clip }) => (
          <button className={selected?.clip.id === clip.id ? "midi-clip-row selected" : "midi-clip-row"} key={clip.id} type="button" onClick={() => void focusDawTarget(onInvoke, { track_id: track.id, clip_id: clip.id, start_seconds: clip.startSeconds })}>
            <strong>{clip.name}</strong>
            <span>{track.name} 路 {clip.noteCount || clip.notes.length || "?"} notes</span>
          </button>
        ))}
      </aside>
      <section className="midi-editor-preview">
        {selected ? (
          <>
            <div className="midi-editor-header">
              <div>
                <span>{selected.track.name}</span>
                <strong>{selected.clip.name}</strong>
              </div>
              <button type="button" onClick={() => void focusDawTarget(onInvoke, { track_id: selected.track.id, clip_id: selected.clip.id, start_seconds: selected.clip.startSeconds })}>跳到 Clip</button>
            </div>
            <PianoRollPreview notes={selectedNotes} clip={selected.clip} loading={Boolean(loadingClipIDs[selected.clip.id])} error={selectedReadError} ghosts={ghostClips} />
          </>
        ) : (
          <EmptyState label="选择一个 MIDI clip 查看音符" />
        )}
      </section>
      <aside className="midi-track-clips">
        <div className="midi-track-clips-header">
          <h2>Clips</h2>
          <span>{selected?.track.name ?? "No track"}</span>
        </div>
        {currentTrackClips.length === 0 && <EmptyState label="No clips on current track" />}
        {currentTrackClips.map((clip) => (
          <button className={selected?.clip.id === clip.id ? "midi-clip-row selected" : "midi-clip-row"} key={clip.id} type="button" onClick={() => selected && focusMidiClip(selected.track, clip)}>
            <strong>{clip.name}</strong>
            <span>{midiClipNoteCountLabel(clip, notesByClip)}</span>
          </button>
        ))}
      </aside>
    </div>
  );
}

function PianoRollPreview({ notes, clip, loading, error = "", ghosts = [] }: { notes: JsonRecord[]; clip: DawClip; loading: boolean; error?: string; ghosts?: DawMidiGhostClip[] }) {
  const rows = 12;
  const length = Math.max(1, clip.lengthBeats || 4);
  const ghostNoteCount = ghosts.reduce((total, ghost) => total + ghost.notes.length, 0);
  return (
    <div className="piano-roll-preview">
      {Array.from({ length: rows }).map((_, index) => <span className="piano-row" key={`row-${index}`} />)}
      {loading && <span className="piano-empty">Reading notes...</span>}
      {!loading && error && <span className="piano-empty">{error}</span>}
      {!loading && !error && notes.length === 0 && ghostNoteCount === 0 && <span className="piano-empty">No note data</span>}
      {ghosts.map((ghost) => ghost.notes.map((note, index) => {
        const pitch = finiteNumber(note.pitch ?? note.note, 60);
        const start = finiteNumber(note.start ?? note.start_beats ?? note.beat, 0) + ghost.offsetBeats;
        const duration = Math.max(0.1, finiteNumber(note.length ?? note.length_beats ?? note.duration, 0.25));
        const row = rows - 1 - Math.abs(Math.round(pitch) % rows);
        const left = start / length * 100;
        const width = duration / length * 100;
        if (left > 100 || left + width < 0) {
          return null;
        }
        return (
          <i
            className="midi-note ghost"
            key={`ghost-${ghost.track.id}-${ghost.clip.id}-${index}`}
            style={{
              background: ghost.track.color,
              borderColor: ghost.track.color,
              left: `${Math.max(0, Math.min(100, left))}%`,
              width: `${Math.max(2, Math.min(100, width))}%`,
              top: `${row / rows * 100}%`
            }}
          />
        );
      }))}
      {notes.map((note, index) => {
        const pitch = finiteNumber(note.pitch ?? note.note, 60);
        const start = finiteNumber(note.start ?? note.start_beats ?? note.beat, 0);
        const duration = Math.max(0.1, finiteNumber(note.length ?? note.length_beats ?? note.duration, 0.25));
        const row = rows - 1 - Math.abs(Math.round(pitch) % rows);
        return (
          <i
            className="midi-note"
            key={`note-${index}`}
            style={{
              left: `${Math.max(0, Math.min(100, start / length * 100))}%`,
              width: `${Math.max(2, Math.min(100, duration / length * 100))}%`,
              top: `${row / rows * 100}%`
            }}
          />
        );
      })}
    </div>
  );
}

function DawMixerPanel({ tracks, onInvoke }: { tracks: DawTrack[]; onInvoke: DawInvoke }) {
  const [editingMixerVolumeDB, setEditingMixerVolumeDB] = useState<Record<string, number>>({});

  useEffect(() => {
    setEditingMixerVolumeDB((current) => {
      const trackIDs = new Set(tracks.map((track) => track.id));
      let changed = false;
      const next = { ...current };
      Object.keys(next).forEach((trackID) => {
        const track = tracks.find((entry) => entry.id === trackID);
        if (!trackIDs.has(trackID) || (track?.volumeDB != null && Math.abs(track.volumeDB - next[trackID]) < 0.05)) {
          delete next[trackID];
          changed = true;
        }
      });
      return changed ? next : current;
    });
  }, [tracks]);

  const setPendingMixerVolume = useCallback((trackID: string, db: number) => {
    const nextDB = clampMixerVolumeDB(db);
    setEditingMixerVolumeDB((current) => ({ ...current, [trackID]: nextDB }));
    return nextDB;
  }, []);

  const clearPendingMixerVolume = useCallback((trackID: string, db: number) => {
    window.setTimeout(() => {
      setEditingMixerVolumeDB((current) => {
        if (Math.abs((current[trackID] ?? db) - db) > 0.05) {
          return current;
        }
        const next = { ...current };
        delete next[trackID];
        return next;
      });
    }, 1200);
  }, []);

  const commitMixerVolume = useCallback((trackID: string, db: number) => {
    const nextDB = clampMixerVolumeDB(db);
    void onInvoke("track.volume", { track_id: trackID, db: nextDB }, "ask_vit_webui_mixer").finally(() => {
      clearPendingMixerVolume(trackID, nextDB);
    });
  }, [clearPendingMixerVolume, onInvoke]);

  const updateMixerVolumeFromPointer = useCallback((event: ReactPointerEvent<HTMLElement>, trackID: string) => {
    event.preventDefault();
    return setPendingMixerVolume(trackID, mixerDBFromPointer(event));
  }, [setPendingMixerVolume]);

  const updateMixerVolumeFromKey = useCallback((event: ReactKeyboardEvent<HTMLElement>, trackID: string, currentDB: number) => {
    const nextDB = mixerDBFromKeyboardEvent(event, currentDB);
    if (nextDB == null) {
      return;
    }
    event.preventDefault();
    const committedDB = setPendingMixerVolume(trackID, nextDB);
    commitMixerVolume(trackID, committedDB);
  }, [commitMixerVolume, setPendingMixerVolume]);

  return (
    <div className="daw-panel-body mixer-strips">
      {tracks.length === 0 && <EmptyState label="当前工程没有可混音轨道" />}
      {tracks.map((track) => {
        const displayVolumeDB = editingMixerVolumeDB[track.id] ?? track.volumeDB;
        const volumeEnabled = displayVolumeDB != null;
        const meterKnown = track.levelDB != null;
        const faderTop = mixerFaderTopPercent(displayVolumeDB);
        return (
          <article className={`mixer-strip-card ${track.selected ? "selected" : ""}`} key={track.id}>
            <button className="mixer-strip-name" type="button" onClick={() => void focusDawTarget(onInvoke, { track_id: track.id })}>
              <span className="track-color" style={{ background: track.color }} />
              <strong>{track.name}</strong>
            </button>
            <div className="mixer-zone-stack" aria-label={`${track.name} rack zones`}>
              <span>Z1 MIDI FX</span>
              <span>Z2 乐器</span>
              <span>Z3 音频效果</span>
            </div>
            <div className="mixer-strip-core">
              <div
                aria-disabled={!volumeEnabled}
                aria-label={`${track.name} volume fader`}
                aria-valuemax={mixerFaderMaxDB}
                aria-valuemin={mixerFaderMinDB}
                aria-valuenow={volumeEnabled ? Number(displayVolumeDB.toFixed(1)) : undefined}
                aria-valuetext={mixerVolumeLabel(displayVolumeDB)}
                className={volumeEnabled ? "mixer-fader" : "mixer-fader disabled"}
                onKeyDown={(event) => {
                  if (!volumeEnabled) {
                    return;
                  }
                  updateMixerVolumeFromKey(event, track.id, displayVolumeDB);
                }}
                onPointerCancel={(event) => {
                  if (event.currentTarget.hasPointerCapture(event.pointerId)) {
                    event.currentTarget.releasePointerCapture(event.pointerId);
                  }
                }}
                onPointerDown={(event) => {
                  if (!volumeEnabled) {
                    void focusDawTarget(onInvoke, { track_id: track.id });
                    return;
                  }
                  void focusDawTarget(onInvoke, { track_id: track.id });
                  event.currentTarget.setPointerCapture(event.pointerId);
                  updateMixerVolumeFromPointer(event, track.id);
                }}
                onPointerMove={(event) => {
                  if (!volumeEnabled || (event.buttons & 1) !== 1 || !event.currentTarget.hasPointerCapture(event.pointerId)) {
                    return;
                  }
                  updateMixerVolumeFromPointer(event, track.id);
                }}
                onPointerUp={(event) => {
                  if (event.currentTarget.hasPointerCapture(event.pointerId)) {
                    event.currentTarget.releasePointerCapture(event.pointerId);
                  }
                  if (!volumeEnabled) {
                    return;
                  }
                  const nextDB = updateMixerVolumeFromPointer(event, track.id);
                  commitMixerVolume(track.id, nextDB);
                }}
                role="slider"
                tabIndex={volumeEnabled ? 0 : -1}
                title={volumeEnabled ? `Volume ${mixerVolumeLabel(displayVolumeDB)}` : "No volume data"}
              >
                <div className="mixer-scale" aria-hidden="true">
                  {mixerFaderTicks.map((db) => (
                    <span key={db} style={{ top: `${mixerFaderTickTopPercent(db)}%` }}>{mixerFaderTickLabel(db)}</span>
                  ))}
                </div>
                <div className="mixer-groove" aria-hidden="true">
                  <span className="mixer-rail" />
                  <span className="mixer-knob" style={{ top: `${faderTop}%` }} />
                </div>
              </div>
              <div
                className={meterKnown ? "mixer-meter" : "mixer-meter unknown"}
                onPointerDown={() => void focusDawTarget(onInvoke, { track_id: track.id })}
                title={mixerMeterTitle(track.levelDB)}
              >
                <span className="mixer-meter-fill" style={{ height: `${meterHeight(track.levelDB)}%` }} />
              </div>
            </div>
            <div className="mixer-volume">
              <span>VOL</span>
              <strong>{mixerVolumeLabel(displayVolumeDB)}</strong>
            </div>
            <div className="track-mini-actions">
              <button className={track.mute ? "toggle on red" : "toggle"} type="button" onClick={() => void onInvoke("track.mute", { track_id: track.id, mute: !track.mute }, "ask_vit_webui_mixer")}>M</button>
              <button className={track.solo ? "toggle on yellow" : "toggle"} type="button" onClick={() => void onInvoke("track.solo", { track_id: track.id, solo: !track.solo }, "ask_vit_webui_mixer")}>S</button>
              <button className={track.armed ? "toggle on blue" : "toggle"} type="button" onClick={() => void onInvoke("track.arm", { track_id: track.id, is_armed: !track.armed }, "ask_vit_webui_mixer")}>R</button>
            </div>
          </article>
        );
      })}
    </div>
  );
}

const mixerFaderMinDB = -60;
const mixerFaderMaxDB = 12;
const mixerFaderTicks = [-60, -15, -5, 0, 3, 12];
const mixerFaderScalePoints = [
  { db: -60, y: 1 },
  { db: -15, y: 0.65 },
  { db: 0, y: 0.3 },
  { db: 3, y: 0.15 },
  { db: 12, y: 0 }
];

const dawTrackPalette = ["#1f55d8", "#0f8f6d", "#a35d00", "#7c3aed", "#b42318", "#2e6f95", "#8a6f00", "#4d66c7"];

function dawTracksFromUIState(uiState: AgentUIState | null): DawTrack[] {
  const selectedTrackID = selectedTrackIDFromUIState(uiState);
  const selectedPlugin = selectedPluginFromUIState(uiState);
  const selectedClipIDs = selectedClipIDsFromUIState(uiState);
  const bpm = bpmFromUIState(uiState);
  return (uiState?.tracks ?? [])
    .map(asRecord)
    .map((track, index) => dawTrackFromRecord(track, index, selectedTrackID, selectedPlugin, selectedClipIDs, bpm))
    .filter((track) => track.id !== "");
}

function dawTrackFromRecord(
  raw: JsonRecord,
  index: number,
  selectedTrackID: string,
  selectedPlugin: { id: string; trackID: string; name: string },
  selectedClipIDs: Set<string>,
  bpm: number
): DawTrack {
  const id = textFromKeys(raw, "track_id", "id", "uid", "kernel_track_id");
  const rack = asRecord(raw.rack);
  const rackNodes = firstArray(rack.nodes, raw.plugins, raw.devices, raw.rack_nodes, rack.items, rack.chain);
  const rackEdges = firstArray(rack.edges, raw.rack_edges, raw.edges).map(asRecord);
  const clips = firstArray(raw.clips, raw.clip_summaries)
    .map(asRecord)
    .map((clip) => dawClipFromRecord(clip, selectedClipIDs, bpm))
    .filter((clip) => clip.id !== "")
    .sort((left, right) => left.startSeconds - right.startSeconds);
  const plugins = rackNodes
    .map(asRecord)
    .filter((plugin) => !isSystemRackPluginRecord(plugin))
    .map((plugin, pluginIndex) => dawPluginFromRecord(plugin, pluginIndex, selectedPlugin))
    .filter((plugin) => plugin.id !== "");
  const selected = truthy(raw.selected ?? raw.is_selected) || (selectedTrackID !== "" && selectedTrackID === id) || selectedPlugin.trackID === id;
  return {
    id,
    name: textFromKeys(raw, "track_name", "name", "label") || id || `Track ${index + 1}`,
    type: statusLabel(textFromKeys(raw, "track_type", "type", "vit_type"), textFromKeys(raw, "track_type", "type", "vit_type") || "track"),
    color: textFromKeys(raw, "color", "track_color") || dawTrackPalette[index % dawTrackPalette.length],
    selected,
    mute: truthy(raw.mute ?? raw.muted ?? raw.is_muted),
    solo: truthy(raw.solo ?? raw.is_solo),
    armed: truthy(raw.armed ?? raw.is_armed ?? raw.record_armed),
    volumeDB: numberFromKeys(raw, "volume_db", "volumeDb", "fader_db", "faderDb", "gain_db", "gainDb"),
    pan: numberFromKeys(raw, "pan", "pan_value"),
    levelDB: trackLevelDB(raw),
    clips,
    plugins,
    rackEdges
  };
}

function dawClipFromRecord(raw: JsonRecord, selectedClipIDs: Set<string>, bpm: number): DawClip {
  const id = textFromKeys(raw, "clip_id", "id", "uid");
  const type = textFromKeys(raw, "clip_type", "type", "media_type", "kind") || "clip";
  const beatSeconds = 60 / Math.max(1, bpm);
  const startBeats = finiteNumber(valueFromKeys(raw, "start_beats", "startBeat", "beat"), finiteNumber(valueFromKeys(raw, "start_seconds", "start_time", "start"), 0) / beatSeconds);
  const startSeconds = finiteNumber(valueFromKeys(raw, "start_seconds", "start_time_seconds", "start_time", "startSeconds"), startBeats * beatSeconds);
  const notesRaw = firstArray(raw.notes, raw.midi_notes, raw.note_summary);
  const notes = notesRaw.map(asRecord).filter((note) => Object.keys(note).length > 0);
  const noteCount = Math.max(numericValue(raw.note_count ?? raw.notes_count), notes.length);
  const endBeats = finiteNumber(valueFromKeys(raw, "end_beats", "endBeat"), NaN);
  const endSeconds = finiteNumber(valueFromKeys(raw, "end_seconds", "end_time_seconds", "end_time", "endSeconds"), NaN);
  let lengthBeats = finiteNumber(valueFromKeys(raw, "length_beats", "duration_beats", "lengthBeats"), Number.isFinite(endBeats) ? endBeats - startBeats : NaN);
  let lengthSeconds = finiteNumber(valueFromKeys(raw, "length_seconds", "duration_seconds", "lengthSeconds", "duration"), Number.isFinite(endSeconds) ? endSeconds - startSeconds : NaN);
  if (!Number.isFinite(lengthBeats) || lengthBeats <= 0) {
    lengthBeats = Number.isFinite(lengthSeconds) && lengthSeconds > 0 ? lengthSeconds / beatSeconds : midiNoteEndBeats(notes);
  }
  if (!Number.isFinite(lengthSeconds) || lengthSeconds <= 0) {
    lengthSeconds = Number.isFinite(lengthBeats) && lengthBeats > 0 ? lengthBeats * beatSeconds : 1;
  }
  lengthBeats = Math.max(0.25, lengthBeats);
  lengthSeconds = Math.max(0.25, lengthSeconds);
  const activity = clipActivitySegments(raw, notes, lengthBeats);
  const activityKnown = hasArrayValue(raw, "notes", "midi_notes", "note_summary", "activity", "activity_segments", "peaks", "waveform_peaks", "waveform", "rms", "amplitudes");
  return {
    id,
    name: textFromKeys(raw, "name", "clip_name", "title") || id || "Clip",
    type,
    startSeconds,
    lengthSeconds,
    startBeats,
    lengthBeats,
    selected: selectedClipIDs.has(id) || truthy(raw.selected ?? raw.is_selected),
    noteCount,
    notes,
    activity,
    activityKnown
  };
}

function dawPluginFromRecord(raw: JsonRecord, index: number, selectedPlugin: { id: string; trackID: string; name: string }): DawPlugin {
  const id = textFromKeys(raw, "plugin_id", "id", "node_id", "item_id", "plugin_item_id");
  const itemID = textFromKeys(raw, "plugin_item_id", "item_id", "plugin_id", "id", "node_id") || id;
  const scope = textFromKeys(raw, "clip_scope", "scope", "target_scope", "lane").toLowerCase();
  const role = textFromKeys(raw, "role", "slot", "lane", "chain", "chain_id", "group");
  const kind = textFromKeys(raw, "type", "plugin_type", "kind") || "plugin";
  return {
    id,
    itemID,
    name: textFromKeys(raw, "plugin_name", "name", "display_name", "label") || id || `Plugin ${index + 1}`,
    kind,
    role,
    clipScope: scope.includes("clip") || textFromKeys(raw, "clip_id", "target_clip_id") ? scope || "clip" : "track",
    selected: selectedPlugin.id !== "" && selectedPlugin.id === id,
    bypass: truthy(raw.bypass ?? raw.bypassed ?? raw.disabled ?? raw.is_bypassed),
    path: textFromKeys(raw, "plugin_path", "path", "file_path", "module_path", "vst_path", "binary_path"),
    vendor: textFromKeys(raw, "vendor", "manufacturer", "maker"),
    source: textFromKeys(raw, "track_id", "source_track_id", "owner_track_id"),
    deletable: !truthy(raw.system ?? raw.builtin ?? raw.internal ?? raw.locked ?? raw.protected)
  };
}

function isSystemRackPluginRecord(raw: JsonRecord): boolean {
  if (truthy(raw.system ?? raw.builtin ?? raw.internal ?? raw.hidden ?? raw.is_default ?? raw.default_plugin)) {
    return true;
  }
  if (raw.user_visible === false || raw.visible === false) {
    return true;
  }
  const label = normalizeRackPluginText(textFromKeys(raw, "plugin_name", "name", "display_name", "label"));
  const kind = normalizeRackPluginText(textFromKeys(raw, "type", "plugin_type", "kind", "role", "slot"));
  if (label === "volumepanplugin" || label === "volumepan" || label === "trackvolumepan") {
    return true;
  }
  if (label === "levelmeter" || label === "levelmeterlevel" || label === "tracklevelmeter") {
    return true;
  }
  if ((kind.includes("system") || kind.includes("builtin") || kind.includes("internal")) && (label.includes("meter") || label.includes("volumepan"))) {
    return true;
  }
  return false;
}

function normalizeRackPluginText(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, "");
}

function dawPanelSubtitle(activeFocus: FocusMode, tracks: DawTrack[], uiState: AgentUIState | null): string {
  const selectedTrack = tracks.find((track) => track.selected);
  if (activeFocus === "tracks") {
    const clipCount = tracks.reduce((total, track) => total + track.clips.length, 0);
    return `${tracks.length} tracks / ${clipCount} clips`;
  }
  if (activeFocus === "rack") {
    const pluginCount = tracks.reduce((total, track) => total + track.plugins.length, 0);
    return selectedTrack ? `${selectedTrack.name} selected / ${pluginCount} plugins` : `${pluginCount} plugins`;
  }
  if (activeFocus === "midi") {
    const midiCount = tracks.reduce((total, track) => total + track.clips.filter(isMidiClip).length, 0);
    const selectedClip = selectedClipIDFromUIState(uiState);
    return selectedClip ? `${midiCount} MIDI clips / selected ${selectedClip}` : `${midiCount} MIDI clips`;
  }
  if (activeFocus === "mixer") {
    return selectedTrack ? `${selectedTrack.name} selected` : `${tracks.length} mixer channels`;
  }
  return "";
}

function selectedTrackIDFromUIState(uiState: AgentUIState | null): string {
  const selectedTrack = asRecord(uiState?.selected_track);
  const context = asRecord(uiState?.ui_context);
  const selectedPlugin = asRecord(uiState?.selected_plugin);
  return textValue(
    selectedTrack.track_id ??
      selectedTrack.id ??
      context.selected_track_id ??
      context.selected_clip_track_id ??
      context.selected_plugin_track_id ??
      selectedPlugin.track_id,
    ""
  );
}

function selectedClipIDFromUIState(uiState: AgentUIState | null): string {
  const context = asRecord(uiState?.ui_context);
  const selectedClip = asRecord(asRecord(uiState).selected_clip);
  const ids = firstArray(context.selected_clip_ids, selectedClip.selected_clip_ids);
  return textValue(selectedClip.clip_id ?? selectedClip.id ?? context.selected_clip_id ?? ids[0], "");
}

function selectedMidiClipIDFromUIState(uiState: AgentUIState | null): string {
  const context = asRecord(uiState?.ui_context);
  return textValue(context.piano_roll_focus_clip_id ?? context.active_midi_clip_id ?? selectedClipIDFromUIState(uiState), "");
}

function selectedMidiTrackIDFromUIState(uiState: AgentUIState | null): string {
  const context = asRecord(uiState?.ui_context);
  return textValue(context.piano_roll_focus_track_id ?? context.active_midi_track_id ?? context.selected_clip_track_id ?? selectedTrackIDFromUIState(uiState), "");
}

function selectedClipIDsFromUIState(uiState: AgentUIState | null): Set<string> {
  const context = asRecord(uiState?.ui_context);
  const selected = selectedClipIDFromUIState(uiState);
  const ids = firstArray(context.selected_clip_ids)
    .map((id) => textValue(id, ""))
    .filter(Boolean);
  if (selected) {
    ids.push(selected);
  }
  return new Set(ids);
}

function selectedPluginFromUIState(uiState: AgentUIState | null): { id: string; trackID: string; name: string } {
  const plugin = asRecord(uiState?.selected_plugin);
  const context = asRecord(uiState?.ui_context);
  const id = textValue(plugin.plugin_id ?? plugin.id ?? plugin.node_id ?? plugin.plugin_item_id ?? context.selected_plugin_id, "");
  return {
    id,
    trackID: textValue(plugin.track_id ?? plugin.selected_plugin_track_id ?? context.selected_plugin_track_id ?? context.selected_track_id, ""),
    name: textValue(plugin.plugin_name ?? plugin.name ?? context.selected_plugin_name, "")
  };
}

function rackLanesForTrack(track: DawTrack): { trackChain: DawPlugin[]; parallel: DawPlugin[]; clipFx: DawPlugin[] } {
  const clipFx: DawPlugin[] = [];
  const parallel: DawPlugin[] = [];
  const trackChain: DawPlugin[] = [];
  track.plugins.forEach((plugin) => {
    const lane = `${plugin.clipScope} ${plugin.role} ${plugin.kind}`.toLowerCase();
    if (plugin.clipScope !== "track" || lane.includes("clip")) {
      clipFx.push(plugin);
      return;
    }
    if (lane.includes("parallel") || lane.includes("send") || lane.includes("aux") || lane.includes("branch")) {
      parallel.push(plugin);
      return;
    }
    trackChain.push(plugin);
  });
  if (parallel.length === 0 && hasBranchingRackEdges(track.rackEdges)) {
    track.plugins.forEach((plugin) => {
      if (!trackChain.includes(plugin) && !clipFx.includes(plugin) && !parallel.includes(plugin)) {
        parallel.push(plugin);
      }
    });
  }
  return { trackChain, parallel, clipFx };
}

function hasBranchingRackEdges(edges: JsonRecord[]): boolean {
  const outgoing = new Map<string, number>();
  edges.forEach((edge) => {
    const source = textFromKeys(edge, "source", "source_id", "from", "from_id", "src");
    if (!source) {
      return;
    }
    outgoing.set(source, (outgoing.get(source) ?? 0) + 1);
  });
  return Array.from(outgoing.values()).some((count) => count > 1);
}

function rackPluginKey(trackID: string, pluginID: string): string {
  return `${trackID}::${pluginID}`;
}

function rackPluginArgs(track: DawTrack, plugin: DawPlugin): JsonRecord {
  return {
    track_id: track.id,
    plugin_id: plugin.id,
    plugin_item_id: plugin.itemID || plugin.id,
    item_id: plugin.itemID || plugin.id,
    plugin_name: plugin.name,
    plugin_path: plugin.path
  };
}

function rackPluginReference(track: DawTrack, plugin: DawPlugin): string {
  return `@plugin(track_id="${track.id}", plugin_id="${plugin.id}", name="${plugin.name}")`;
}

function rackParamSnapshotFromInvoke(response: AgentInvokeResponse): RackParamSnapshot {
  const result = asRecord(response.result);
  const plugin = asRecord(result.plugin ?? result.snapshot ?? result.plugin_snapshot);
  const rows = firstArray(
    result.parameters,
    result.params,
    result.plugin_parameters,
    result.controls,
    result.quick_controls,
    plugin.parameters,
    plugin.params,
    plugin.controls
  ).map(asRecord);
  const labels = rows
    .map((row) => textFromKeys(row, "label", "name", "param_name", "raw_param_name", "param_id", "id"))
    .filter(Boolean)
    .slice(0, 4);
  const declaredCount = numericValue(result.parameter_count ?? result.param_count ?? result.count);
  return {
    count: declaredCount || rows.length,
    labels,
    updatedAt: Date.now()
  };
}

function rackParamSnapshotText(snapshot: RackParamSnapshot): string {
  if (snapshot.count <= 0) {
    return "No params";
  }
  return snapshot.labels.length > 0 ? `${snapshot.count} params 路 ${snapshot.labels.join(", ")}` : `${snapshot.count} params`;
}

function midiReadNotesFromResult(result: JsonRecord): JsonRecord[] {
  const clip = asRecord(result.clip ?? result.midi_clip);
  const data = asRecord(result.data ?? result.payload);
  return firstArray(
    result.notes,
    result.midi_notes,
    result.note_rows,
    result.note_events,
    result.events,
    clip.notes,
    clip.midi_notes,
    data.notes,
    data.midi_notes,
    data.note_rows,
    data.note_events
  )
    .map(asRecord)
    .filter((note) => Object.keys(note).length > 0);
}

function midiClipHasLoadedNotes(notesByClip: Record<string, JsonRecord[]>, clipID: string): boolean {
  return Object.prototype.hasOwnProperty.call(notesByClip, clipID);
}

function midiClipWithCachedNotes(clip: DawClip, notesByClip: Record<string, JsonRecord[]>): DawClip {
  if (!midiClipHasLoadedNotes(notesByClip, clip.id) || clip.notes.length > 0) {
    return clip;
  }
  const notes = notesByClip[clip.id] ?? [];
  return { ...clip, notes, noteCount: Math.max(clip.noteCount, notes.length) };
}

function midiClipNotesForDisplay(clip: DawClip, notesByClip: Record<string, JsonRecord[]>): JsonRecord[] {
  if (clip.notes.length > 0) {
    return clip.notes;
  }
  return notesByClip[clip.id] ?? [];
}

function midiClipNoteCountLabel(clip: DawClip, notesByClip: Record<string, JsonRecord[]>): string {
  if (clip.noteCount > 0) {
    return `${clip.noteCount} notes`;
  }
  if (clip.notes.length > 0) {
    return `${clip.notes.length} notes`;
  }
  if (midiClipHasLoadedNotes(notesByClip, clip.id)) {
    return `${notesByClip[clip.id]?.length ?? 0} notes`;
  }
  return "? notes";
}

function midiClipIntersectingWindow(clips: DawClip[], focusClip: DawClip | null): DawClip | null {
  if (clips.length === 0) {
    return null;
  }
  if (!focusClip) {
    return clips[0];
  }
  const start = focusClip.startBeats;
  const end = focusClip.startBeats + Math.max(0.125, focusClip.lengthBeats);
  return clips.find((clip) => midiRangesIntersect(clip.startBeats, clip.startBeats + Math.max(0.125, clip.lengthBeats), start, end)) ?? null;
}

function midiClipForTrackActivation(clips: DawClip[], focusClip: DawClip | null): DawClip | null {
  return midiClipIntersectingWindow(clips, focusClip) ?? clips[0] ?? null;
}

function midiRangesIntersect(leftStart: number, leftEnd: number, rightStart: number, rightEnd: number): boolean {
  return leftStart < rightEnd && rightStart < leftEnd;
}

function isMidiClip(clip: DawClip): boolean {
  const type = clip.type.toLowerCase();
  return type.includes("midi") || type.includes("sequencer") || clip.notes.length > 0 || clip.noteCount > 0;
}

async function focusDawTarget(onInvoke: DawInvoke, target: JsonRecord): Promise<void> {
  const trackID = textValue(target.track_id ?? target.target_track_id, "");
  const clipID = textValue(target.clip_id ?? target.selected_clip_id, "");
  const pluginID = textValue(target.plugin_id ?? target.node_id ?? target.plugin_item_id, "");
  if (pluginID) {
    await onInvoke("select_plugin", { track_id: trackID, plugin_id: pluginID }, "ask_vit_webui_focus", true);
  } else if (clipID) {
    await onInvoke("select_clip", { track_id: trackID, clip_id: clipID, start_seconds: target.start_seconds, start_beats: target.start_beats }, "ask_vit_webui_focus", true);
  } else if (trackID) {
    await onInvoke("select_track", { track_id: trackID }, "ask_vit_webui_focus", true);
  }
  const seekSeconds = finiteNumber(target.start_seconds ?? target.start_time ?? target.time, NaN);
  if (clipID && Number.isFinite(seekSeconds) && seekSeconds >= 0) {
    await onInvoke("transport.seek", { time: seekSeconds }, "ask_vit_webui_focus_seek", true);
  }
}

async function auditionDawTarget(onInvoke: DawInvoke, target: JsonRecord): Promise<void> {
  await focusDawTarget(onInvoke, target);
  await onInvoke("transport.play", {}, "ask_vit_webui_audition", true);
}

function actionDawTarget(action: JsonRecord, uiState: AgentUIState | null): JsonRecord | null {
  const source = textValue(action._ui_source, "");
  if (source !== "executed" && source !== "event") {
    return null;
  }
  const command = asRecord(action.command);
  const args = asRecord(command.args ?? action.args);
  const payload = asRecord(action.payload);
  const result = asRecord(action.result);
  const verification = asRecord(action.verification);
  const evidence = asRecord(verification.evidence);
  const merged = mergeJsonRecords(action, command, args, payload, result, verification, evidence);
  const commandName = textValue(
    action.command_name ??
      action.tool ??
      action.cmd ??
      action.command ??
      payload.command_name ??
      payload.tool ??
      payload.cmd ??
      payload.command ??
      result.command_name ??
      result.tool ??
      result.cmd ??
      verification.command_name ??
      verification.tool,
    ""
  ).toLowerCase();
  const uiAction = textValue(result.ui_action ?? payload.ui_action ?? action.ui_action, "").toLowerCase();
  if (!actionLooksLikeDawTarget(commandName, uiAction, merged)) {
    return null;
  }

  let trackID = textValue(
    merged.track_id ?? merged.target_track_id ?? merged.selected_track_id ?? merged.source_track_id ?? merged.observed_track_id ?? merged.expected_track_id,
    ""
  );
  const clipID = textValue(
    merged.clip_id ?? merged.target_clip_id ?? merged.selected_clip_id ?? merged.new_clip_id ?? merged.created_clip_id ?? merged.observed_clip_id ?? merged.expected_clip_id,
    ""
  );
  const pluginID = textValue(
    merged.plugin_id ?? merged.plugin_item_id ?? merged.node_id ?? merged.selected_plugin_id ?? merged.observed_plugin_id ?? merged.expected_plugin_id ?? merged.observed_node_id ?? merged.expected_node_id,
    ""
  );
  const clipContext = clipTargetFromUIState(uiState, clipID, trackID);
  if (!trackID) {
    trackID = textValue(clipContext.track_id, "") || pluginTrackIDFromUIState(uiState, pluginID);
  }
  const startSeconds = actionTargetStartSeconds(merged, clipContext, uiState);
  const startBeats = finiteNumber(merged.start_beats ?? merged.start_beat ?? merged.beat, finiteNumber(clipContext.start_beats, NaN));
  if (!trackID && !clipID && !pluginID) {
    return null;
  }
  return {
    track_id: trackID,
    clip_id: clipID,
    plugin_id: pluginID,
    start_seconds: Number.isFinite(startSeconds) ? startSeconds : undefined,
    start_beats: Number.isFinite(startBeats) ? startBeats : undefined,
    can_audition: actionTargetCanAudition(commandName, uiAction, { track_id: trackID, clip_id: clipID, plugin_id: pluginID })
  };
}

function actionLooksLikeDawTarget(commandName: string, uiAction: string, payload: JsonRecord): boolean {
  if (uiAction || textValue(payload.track_id ?? payload.clip_id ?? payload.plugin_id ?? payload.new_clip_id ?? payload.created_clip_id, "")) {
    return true;
  }
  return /^(track|midi|clip|plugin|rack|control|transport)\b/.test(commandName) || /(track|midi|clip|plugin|rack|control|transport)/.test(commandName);
}

function actionTargetCanAudition(commandName: string, uiAction: string, target: JsonRecord): boolean {
  if (textValue(target.clip_id ?? target.plugin_id, "")) {
    return true;
  }
  if (!textValue(target.track_id, "")) {
    return false;
  }
  const joined = `${commandName} ${uiAction}`;
  if (/add_track|create_track|rename_track|delete_track|select_track/.test(joined)) {
    return false;
  }
  return /mute|solo|arm|volume|pan|mixer|plugin|rack|control/.test(joined);
}

function actionTargetStartSeconds(payload: JsonRecord, clipContext: JsonRecord, uiState: AgentUIState | null): number {
  const directSeconds = finiteNumber(
    payload.start_seconds ??
      payload.start_time_seconds ??
      payload.position_seconds ??
      payload.playhead_seconds ??
      payload.new_start_seconds ??
      payload.time,
    NaN
  );
  if (Number.isFinite(directSeconds)) {
    return directSeconds;
  }
  const bpm = bpmFromUIState(uiState);
  const beatSeconds = 60 / Math.max(1, bpm);
  const timeUnit = textValue(payload.time_unit ?? payload.unit, "").toLowerCase();
  const startTime = finiteNumber(payload.start_time ?? payload.start, NaN);
  if (Number.isFinite(startTime)) {
    return timeUnit.includes("beat") ? startTime * beatSeconds : startTime;
  }
  const newStart = finiteNumber(payload.new_start, NaN);
  if (Number.isFinite(newStart)) {
    return timeUnit.includes("beat") ? newStart * beatSeconds : newStart;
  }
  const startBeats = finiteNumber(payload.start_beats ?? payload.start_beat ?? payload.beat, NaN);
  if (Number.isFinite(startBeats)) {
    return startBeats * beatSeconds;
  }
  return finiteNumber(clipContext.start_seconds, NaN);
}

function clipTargetFromUIState(uiState: AgentUIState | null, clipID: string, trackID = ""): JsonRecord {
  if (!clipID) {
    return {};
  }
  for (const track of dawTracksFromUIState(uiState)) {
    if (trackID && track.id !== trackID) {
      continue;
    }
    const clip = track.clips.find((item) => item.id === clipID);
    if (clip) {
      return {
        track_id: track.id,
        clip_id: clip.id,
        start_seconds: clip.startSeconds,
        start_beats: clip.startBeats
      };
    }
  }
  return {};
}

function pluginTrackIDFromUIState(uiState: AgentUIState | null, pluginID: string): string {
  if (!pluginID) {
    return "";
  }
  for (const track of dawTracksFromUIState(uiState)) {
    if (track.plugins.some((plugin) => plugin.id === pluginID || plugin.itemID === pluginID)) {
      return track.id;
    }
  }
  return "";
}

function clipBlockStyle(clip: DawClip, pixelsPerSecond: number, timelineWidth: number, dragDeltaX = 0, dragDeltaY = 0): React.CSSProperties {
  const safeScale = clampNumber(pixelsPerSecond, 0.05, 80);
  const left = Math.max(0, clip.startSeconds * safeScale);
  const width = clampNumber(clip.lengthSeconds * safeScale, 18, Math.max(18, timelineWidth - left));
  return {
    left: `${left}px`,
    width: `${width}px`,
    transform: dragDeltaX !== 0 || dragDeltaY !== 0 ? `translate(${dragDeltaX}px, ${dragDeltaY}px)` : undefined
  };
}

function draggedClipStart(drag: ClipDragState, pixelsPerSecond: number, snapSeconds: number): number {
  const rawStart = Math.max(0, drag.startSeconds + drag.deltaX / Math.max(0.05, pixelsPerSecond));
  if (snapSeconds <= 0) {
    return rawStart;
  }
  return Math.max(0, Math.round(rawStart / snapSeconds) * snapSeconds);
}

function trackIDFromPointer(clientX: number, clientY: number): string {
  if (typeof document === "undefined") {
    return "";
  }
  const lanes = Array.from(document.querySelectorAll<HTMLElement>(".track-lane-canvas[data-track-id]"));
  for (const lane of lanes) {
    const rect = lane.getBoundingClientRect();
    if (clientY >= rect.top && clientY <= rect.bottom) {
      return lane.dataset.trackId ?? "";
    }
  }
  const element = document.elementFromPoint(clientX, clientY);
  if (!(element instanceof HTMLElement)) {
    return "";
  }
  return element.closest<HTMLElement>("[data-track-id]")?.dataset.trackId ?? "";
}

function formatTimelineSeconds(seconds: number): string {
  const safe = Math.max(0, Math.round(seconds));
  if (safe < 60) {
    return `${safe}s`;
  }
  const minutes = Math.floor(safe / 60);
  const rest = safe % 60;
  return `${minutes}:${String(rest).padStart(2, "0")}`;
}

function clipTitle(track: DawTrack, clip: DawClip): string {
  const end = clip.startSeconds + clip.lengthSeconds;
  const noteSuffix = isMidiClip(clip) ? ` / ${clip.noteCount || clip.notes.length} notes` : "";
  return `${track.name} / ${clip.name}\n${clip.startSeconds.toFixed(2)}s - ${end.toFixed(2)}s${noteSuffix}`;
}

function meterHeight(levelDB: number | null): number {
  if (levelDB == null || !Number.isFinite(levelDB)) {
    return 0;
  }
  return clampNumber(((levelDB + 60) / 60) * 100, 0, 100);
}

function clampMixerVolumeDB(db: number): number {
  return clampNumber(db, mixerFaderMinDB, mixerFaderMaxDB);
}

function mixerVolumeLabel(db: number | null): string {
  if (db == null || !Number.isFinite(db)) {
    return "-- dB";
  }
  if (db <= mixerFaderMinDB + 0.05) {
    return "-inf";
  }
  return `${db.toFixed(1)} dB`;
}

function mixerMeterTitle(levelDB: number | null): string {
  return levelDB == null || !Number.isFinite(levelDB) ? "No meter data" : `Meter ${levelDB.toFixed(1)} dB`;
}

function mixerFaderTickLabel(db: number): string {
  if (db <= mixerFaderMinDB) {
    return "-inf";
  }
  if (db > 0) {
    return `+${Math.round(db)}`;
  }
  return `${Math.round(db)}`;
}

function mixerFaderTopPercent(db: number | null): number {
  return mixerDBToUIPercent(db ?? 0) * 100;
}

function mixerFaderTickTopPercent(db: number): number {
  return clampNumber(mixerFaderTopPercent(db), 4, 96);
}

function mixerDBToUIPercent(db: number): number {
  const safeDB = clampMixerVolumeDB(db);
  for (let index = 0; index < mixerFaderScalePoints.length - 1; index += 1) {
    const current = mixerFaderScalePoints[index];
    const next = mixerFaderScalePoints[index + 1];
    if (safeDB <= next.db) {
      const span = next.db - current.db;
      const t = Math.abs(span) < 1e-9 ? 0 : (safeDB - current.db) / span;
      return current.y + (next.y - current.y) * t;
    }
  }
  return mixerFaderScalePoints[mixerFaderScalePoints.length - 1].y;
}

function mixerUIPercentToDB(percent: number): number {
  const safePercent = clampNumber(percent, 0, 1);
  for (let index = 0; index < mixerFaderScalePoints.length - 1; index += 1) {
    const current = mixerFaderScalePoints[index];
    const next = mixerFaderScalePoints[index + 1];
    const high = Math.max(current.y, next.y);
    const low = Math.min(current.y, next.y);
    if (safePercent <= high && safePercent >= low) {
      const span = current.y - next.y;
      const t = Math.abs(span) < 1e-9 ? 0 : (current.y - safePercent) / span;
      return clampMixerVolumeDB(current.db + (next.db - current.db) * t);
    }
  }
  return mixerFaderMaxDB;
}

function mixerDBFromPointer(event: ReactPointerEvent<HTMLElement>): number {
  const rect = event.currentTarget.getBoundingClientRect();
  if (rect.height <= 1) {
    return 0;
  }
  return mixerUIPercentToDB((event.clientY - rect.top) / rect.height);
}

function mixerDBFromKeyboardEvent(event: ReactKeyboardEvent<HTMLElement>, currentDB: number): number | null {
  const step = event.shiftKey ? 1 : 0.5;
  switch (event.key) {
    case "ArrowUp":
    case "ArrowRight":
      return clampMixerVolumeDB(currentDB + step);
    case "ArrowDown":
    case "ArrowLeft":
      return clampMixerVolumeDB(currentDB - step);
    case "PageUp":
      return clampMixerVolumeDB(currentDB + 3);
    case "PageDown":
      return clampMixerVolumeDB(currentDB - 3);
    case "Home":
      return mixerFaderMaxDB;
    case "End":
      return mixerFaderMinDB;
    default:
      return null;
  }
}

function clipActivitySegments(raw: JsonRecord, notes: JsonRecord[], lengthBeats: number): DawActivitySegment[] {
  if (notes.length > 0) {
    return midiActivitySegments(notes, lengthBeats);
  }
  const declared = firstArray(raw.activity, raw.activity_segments).map(asRecord);
  if (declared.length > 0) {
    return declared
      .map((segment) => ({
        left: clampNumber(finiteNumber(segment.left ?? segment.start_percent ?? segment.start, 0), 0, 100),
        width: clampNumber(finiteNumber(segment.width ?? segment.duration_percent ?? segment.length, 2), 0.5, 100),
        height: clampNumber(finiteNumber(segment.height ?? segment.value ?? segment.amplitude, 45), 8, 100)
      }))
      .filter((segment) => segment.width > 0);
  }
  const peaks = numericSeriesFromAny(
    firstPresentArray(raw.peaks, raw.waveform_peaks, raw.waveform, raw.rms, raw.amplitudes, asRecord(raw.waveform_data).peaks)
  );
  if (peaks.length === 0) {
    return [];
  }
  const bucketCount = Math.min(64, peaks.length);
  const bucketSize = Math.max(1, Math.ceil(peaks.length / bucketCount));
  const segments: DawActivitySegment[] = [];
  for (let bucket = 0; bucket < bucketCount; bucket += 1) {
    const slice = peaks.slice(bucket * bucketSize, (bucket + 1) * bucketSize);
    if (slice.length === 0) {
      continue;
    }
    const amplitude = slice.reduce((max, value) => Math.max(max, Math.abs(value)), 0);
    if (amplitude <= 0.001) {
      continue;
    }
    segments.push({
      left: (bucket / bucketCount) * 100,
      width: Math.max(0.8, 100 / bucketCount),
      height: clampNumber(amplitude * 100, 8, 100)
    });
  }
  return segments;
}

function midiActivitySegments(notes: JsonRecord[], lengthBeats: number): DawActivitySegment[] {
  const safeLength = Math.max(0.25, lengthBeats);
  return notes.slice(0, 120).map((note) => {
    const start = finiteNumber(note.start ?? note.start_beats ?? note.beat, 0);
    const duration = Math.max(0.05, finiteNumber(note.length ?? note.length_beats ?? note.duration, 0.25));
    const velocity = clampNumber(finiteNumber(note.velocity ?? note.vel, 96), 1, 127);
    return {
      left: clampNumber((start / safeLength) * 100, 0, 100),
      width: clampNumber((duration / safeLength) * 100, 0.7, 100),
      height: clampNumber((velocity / 127) * 100, 14, 100)
    };
  });
}

function midiNoteEndBeats(notes: JsonRecord[]): number {
  return notes.reduce((end, note) => {
    const start = finiteNumber(note.start ?? note.start_beats ?? note.beat, 0);
    const length = Math.max(0.05, finiteNumber(note.length ?? note.length_beats ?? note.duration, 0.25));
    return Math.max(end, start + length);
  }, 0);
}

function numericSeriesFromAny(value: unknown): number[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map((item) => {
      if (Array.isArray(item)) {
        const values = item.map(Number).filter(Number.isFinite);
        return values.length > 0 ? Math.max(...values.map((next) => Math.abs(next))) : NaN;
      }
      const record = asRecord(item);
      if (Object.keys(record).length > 0) {
        return finiteNumber(record.peak ?? record.rms ?? record.value ?? record.amplitude ?? record.height, NaN);
      }
      return Number(item);
    })
    .filter(Number.isFinite)
    .map((value) => Math.abs(value));
}

function bpmFromUIState(uiState: AgentUIState | null): number {
  const transport = asRecord(uiState?.transport);
  const project = asRecord(uiState?.project);
  return Math.max(1, finiteNumber(transport.bpm ?? transport.tempo ?? project.bpm ?? project.tempo, 120));
}

function trackLevelDB(raw: JsonRecord): number | null {
  const direct = numberFromKeys(raw, "level_db", "levelDb", "meter_db", "current_db", "peak_db", "peakDb", "meter_peak_db", "meter_level_db");
  if (direct != null) {
    return direct;
  }
  const stereo = [
    numberFromKeys(raw, "left_level_db", "leftLevelDb", "left_peak_db", "leftPeakDb", "level_l_db", "peak_l_db"),
    numberFromKeys(raw, "right_level_db", "rightLevelDb", "right_peak_db", "rightPeakDb", "level_r_db", "peak_r_db")
  ].filter((value): value is number => value != null);
  if (stereo.length > 0) {
    return Math.max(...stereo);
  }
  const meters = asRecord(raw.meter ?? raw.levels ?? raw.telemetry);
  const nestedDirect = numberFromKeys(meters, "level_db", "levelDb", "meter_db", "current_db", "peak_db", "peakDb", "meter_peak_db", "meter_level_db", "left_db", "rms_db");
  if (nestedDirect != null) {
    return nestedDirect;
  }
  const nestedStereo = [
    numberFromKeys(meters, "left_level_db", "leftLevelDb", "left_peak_db", "leftPeakDb", "level_l_db", "peak_l_db"),
    numberFromKeys(meters, "right_level_db", "rightLevelDb", "right_peak_db", "rightPeakDb", "level_r_db", "peak_r_db")
  ].filter((value): value is number => value != null);
  return nestedStereo.length > 0 ? Math.max(...nestedStereo) : null;
}

function numberFromKeys(record: JsonRecord, ...keys: string[]): number | null {
  for (const key of keys) {
    if (!(key in record)) {
      continue;
    }
    const value = Number(record[key]);
    if (Number.isFinite(value)) {
      return value;
    }
  }
  return null;
}

function textFromKeys(record: JsonRecord, ...keys: string[]): string {
  for (const key of keys) {
    const value = textValue(record[key], "");
    if (value) {
      return value;
    }
  }
  return "";
}

function valueFromKeys(record: JsonRecord, ...keys: string[]): unknown {
  for (const key of keys) {
    if (key in record && record[key] !== undefined && record[key] !== null && textValue(record[key], "") !== "") {
      return record[key];
    }
  }
  return undefined;
}

function hasArrayValue(record: JsonRecord, ...keys: string[]): boolean {
  return keys.some((key) => Array.isArray(record[key]));
}

function firstPresentArray(...values: unknown[]): unknown[] {
  for (const value of values) {
    if (Array.isArray(value)) {
      return value;
    }
  }
  return [];
}

function StatusCell({ icon: Icon, label, tone = "neutral" }: { icon: LucideIcon; label: string; tone?: "neutral" | "ok" | "warn" }) {
  return (
    <span className={`status-cell ${tone}`}>
      <Icon size={15} />
      {label}
    </span>
  );
}

function runtimeDiagnosticLine(healthLabel: string, runtimeStatus: RuntimeStatusResponse | null, connection: ConnectionStatus): string {
  if (connection !== "ready" || !runtimeStatus) {
    return healthLabel;
  }
  const kernel = asRecord(runtimeStatus.kernel);
  const shadow = asRecord(runtimeStatus.shadow);
  const kernelConnected = Boolean(kernel.connected);
  const pieces = [healthLabel, kernelConnected ? "内核已连接" : "内核未连接"];
  const trackCount = textValue(shadow.track_count ?? shadow.user_track_count, "");
  if (trackCount) {
    pieces.push(`${trackCount} 条轨道`);
  }
  const deltaSeq = textValue(shadow.last_delta_seq, "");
  if (deltaSeq) {
    pieces.push(`状态 #${deltaSeq}`);
  }
  return pieces.join(" 路 ");
}

function runtimeDiagnosticTitle(runtimeStatus: RuntimeStatusResponse | null): string {
  if (!runtimeStatus) {
    return "Agent 运行状态不可用";
  }
  const kernel = asRecord(runtimeStatus.kernel);
  const shadow = asRecord(runtimeStatus.shadow);
  const lines = [
    `Agent: ${runtimeStatus.service ?? "VitAgent"}${runtimeStatus.pid ? ` pid=${runtimeStatus.pid}` : ""}`,
    `内核: ${Boolean(kernel.connected) ? "已连接" : "未连接"}${kernel.endpoint ? ` ${String(kernel.endpoint)}` : ""}`,
    `影子状态: ${Boolean(shadow.initialized) ? "已初始化" : "未初始化"}`
  ];
  const trackCount = textValue(shadow.track_count ?? shadow.user_track_count, "");
  if (trackCount) {
    lines.push(`轨道: ${trackCount}`);
  }
  const deltaSeq = textValue(shadow.last_delta_seq, "");
  if (deltaSeq) {
    lines.push(`最新状态版本 ${deltaSeq}`);
  }
  const kernelError = textValue(kernel.error, "");
  if (kernelError) {
    lines.push(`鍐呮牳閿欒: ${kernelError}`);
  }
  return lines.join("\n");
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

function MessageStream({
  messages,
  activities,
  respondingActionID,
  hiddenActionID,
  bottomInset,
  onInteractionAction,
  onInvoke,
  onSelectArtifact,
  uiState,
  onMacroValuePreview,
  onMacroValueCommit
}: {
  messages: ChatMessage[];
  activities: ChatMessage[];
  respondingActionID: string;
  hiddenActionID: string;
  bottomInset: number;
  onInteractionAction: (interaction: JsonRecord, action: JsonRecord, payload?: JsonRecord) => void;
  onInvoke: DawInvoke;
  onSelectArtifact: (id: string) => void;
  uiState: AgentUIState | null;
  onMacroValuePreview: (macro: MacroControl, value: number) => void;
  onMacroValueCommit: (macro: MacroControl, value: number) => Promise<void>;
}) {
  const bottomRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const frameID = window.requestAnimationFrame(() => {
      bottomRef.current?.scrollIntoView({ block: "end", behavior: "smooth" });
    });
    return () => window.cancelAnimationFrame(frameID);
  }, [activities.length, messages.length, respondingActionID, bottomInset]);

  return (
    <div className="message-stream">
      {messages.map((message) => {
        if (shouldHideMessageForComposerOverlay(message, hiddenActionID)) {
          return null;
        }
        const modeLabel = agentModeLabel(message.mode);
        const actionsBeforeContent = shouldRenderActionsBeforeContent(message);
        const suppressProposalContent = shouldSuppressProposalContent(message);
        const contentBlock = suppressProposalContent
          ? null
          : message.status === "pending" && message.role === "assistant"
            ? <TypingMessage content={message.content} />
            : <p>{message.content}</p>;
        const actionCardsBlock = message.actions && message.actions.length > 0
          ? (
              <ActionCards
                actions={message.actions}
                respondingActionID={respondingActionID}
                hiddenActionID={hiddenActionID}
                onInteractionAction={onInteractionAction}
                onInvoke={onInvoke}
                onSelectArtifact={onSelectArtifact}
                uiState={uiState}
                onMacroValuePreview={onMacroValuePreview}
                onMacroValueCommit={onMacroValueCommit}
              />
            )
          : null;
        return (
          <article key={message.id} className={`message-row ${message.role} ${message.status ?? ""}`}>
            <div className="message-avatar">{message.role === "user" ? <Activity size={16} /> : <Bot size={16} />}</div>
            <div className="message-body">
              <div className="message-meta">
                <span>{messageRoleLabel(message.role)}</span>
                {modeLabel && <b>{modeLabel}</b>}
              </div>
              {actionsBeforeContent && actionCardsBlock}
              {contentBlock}
              {message.artifacts && message.artifacts.length > 0 && (
                <div className="artifact-context-list message-context-list">
                  {message.artifacts.map((artifact) => (
                    <ArtifactContextCard key={artifact.id} artifact={artifact} variant="message" onSelect={() => onSelectArtifact(artifact.id)} />
                  ))}
                </div>
              )}
              {!actionsBeforeContent && actionCardsBlock}
            </div>
          </article>
        );
      })}
      {activities.length > 0 && (
        <section className="activity-lane" aria-label="即时活动" aria-live="polite">
          {activities.map((activity) => (
            <div key={activity.id} className={`activity-lane-item ${activity.status ?? "pending"}`}>
              {activity.status === "error" ? <AlertTriangle size={14} /> : <Loader2 className="spin" size={14} />}
              <span>{activity.content}</span>
              {activity.status !== "error" && <span className="typing-dots" aria-hidden="true"><i /><i /><i /></span>}
            </div>
          ))}
        </section>
      )}
      <div ref={bottomRef} className="message-scroll-anchor" style={{ height: `${bottomInset}px` }} />
    </div>
  );
}

function shouldHideMessageForComposerOverlay(message: ChatMessage, hiddenActionID: string): boolean {
  if (!hiddenActionID || message.artifacts?.length || !isDisposableConfirmationPromptText(message.content)) {
    return false;
  }
  return (message.actions ?? []).map(asRecord).some((action) => actionRenderID(action) === hiddenActionID && isConfirmationAction(action));
}

function shouldRenderActionsBeforeContent(message: ChatMessage): boolean {
  if (message.role !== "assistant") {
    return false;
  }
  return (message.actions ?? []).map(asRecord).some(isMixPlanningDiscussionAction);
}

function TypingMessage({ content }: { content: string }) {
  return (
    <p className="typing-message">
      <span>{content}</span>
      <span className="typing-dots" aria-hidden="true">
        <i />
        <i />
        <i />
      </span>
    </p>
  );
}

function ActionCards({
  actions,
  respondingActionID,
  hiddenActionID = "",
  onInteractionAction,
  onInvoke,
  onSelectArtifact,
  uiState,
  onMacroValuePreview,
  onMacroValueCommit
}: {
  actions: JsonRecord[];
  respondingActionID: string;
  hiddenActionID?: string;
  onInteractionAction: (interaction: JsonRecord, action: JsonRecord, payload?: JsonRecord) => void;
  onInvoke: DawInvoke;
  onSelectArtifact: (id: string) => void;
  uiState: AgentUIState | null;
  onMacroValuePreview: (macro: MacroControl, value: number) => void;
  onMacroValueCommit: (macro: MacroControl, value: number) => Promise<void>;
}) {
  const visibleActions = actions
    .slice(0, 6)
    .filter((action) => !hiddenActionID || actionRenderID(action) !== hiddenActionID);
  if (visibleActions.length === 0) {
    return null;
  }
  return (
    <div className="action-list">
      {visibleActions.map((action, index) => (
        <ActionCard
          key={`${textValue(action._ui_source, "")}-${actionRenderID(action) || actionTitle(action, index)}-${index}`}
          action={action}
          index={index}
          respondingActionID={respondingActionID}
          onInteractionAction={onInteractionAction}
          onInvoke={onInvoke}
          onSelectArtifact={onSelectArtifact}
          uiState={uiState}
          onMacroValuePreview={onMacroValuePreview}
          onMacroValueCommit={onMacroValueCommit}
        />
      ))}
    </div>
  );
}

type ActionCardProps = {
  action: JsonRecord;
  index: number;
  respondingActionID: string;
  onInteractionAction: (interaction: JsonRecord, action: JsonRecord, payload?: JsonRecord) => void;
  onInvoke: DawInvoke;
  onSelectArtifact: (id: string) => void;
  uiState: AgentUIState | null;
  onMacroValuePreview: (macro: MacroControl, value: number) => void;
  onMacroValueCommit: (macro: MacroControl, value: number) => Promise<void>;
};

type ProjectResultReadiness = "ready" | "silent_risk" | "affected_by_mute" | "unknown" | "not_auditionable";
type ProjectResultView = {
  title: string;
  badge: string;
  body: string;
  hint: string;
  readiness: ProjectResultReadiness;
  canAudition: boolean;
  auditionLabel: string;
  target: JsonRecord;
  details: Array<{ id: string; title: string; body: string }>;
};

type ProjectResultABView = {
  title: string;
  body: string;
  status: string;
  tapPoint: string;
  renderMode: string;
  deltaSummary: string;
};

function ActionCard(props: ActionCardProps) {
  if (isCapabilityProposalInteraction(props.action)) {
    return <CapabilityProposalCard {...props} />;
  }
  if (isMixBoardAction(props.action)) {
    return <MixBoardActionCard {...props} />;
  }
  if (isMixTreatmentPendingAction(props.action)) {
    return <MixTreatmentPendingCard {...props} />;
  }
  if (isProjectResultAction(props.action)) {
    return <ProjectResultCard {...props} />;
  }
  if (isMacroControlAction(props.action)) {
    return (
      <MacroActionCard
        action={props.action}
        uiState={props.uiState}
        respondingActionID={props.respondingActionID}
        onMacroValuePreview={props.onMacroValuePreview}
        onMacroValueCommit={props.onMacroValueCommit}
      />
    );
  }
  return <StandardActionCard {...props} />;
}

function shouldSuppressProposalContent(message: ChatMessage): boolean {
  if (message.role !== "assistant" || !message.actions?.length) {
    return false;
  }
  return message.actions.map(asRecord).some(isCapabilityProposalInteraction);
}

function CapabilityProposalCard({ action, respondingActionID, onInteractionAction }: ActionCardProps) {
  const payload = interactionPayload(action);
  const presentation = asRecord(payload.proposal_presentation ?? action.proposal_presentation);
  const childActions = firstArray(action.actions)
    .map(asRecord)
    .filter((item) => Object.keys(item).length > 0);
  const interactionID = textValue(action.id ?? action.interaction_id, "");
  const renderID = actionRenderID(action);
  const title = textValue(presentation.title ?? action.title, "方案等待确认");
  const conclusion = textValue(presentation.conclusion ?? action.body, "");
  const recommendation = textValue(presentation.recommendation, "");
  const revision = finiteNumber(presentation.proposal_revision ?? payload.proposal_revision, 0);
  const actionCount = finiteNumber(presentation.action_count, 0);
  const analyzedTracks = finiteNumber(presentation.analyzed_tracks, 0);
  const risk = textValue(presentation.risk, "");
  const reversible = truthy(presentation.reversible);
  const status = textValue(action.status ?? action.stage, "waiting_for_user").toLowerCase();
  const resolved = childActions.length === 0 || status.includes("complete") || status.includes("cancel") || status.includes("fail");
  const readiness = firstArray(presentation.readiness).map(asRecord).filter((item) => Object.keys(item).length > 0);
  const groups = firstArray(presentation.change_groups).map(asRecord).filter((item) => Object.keys(item).length > 0);
  const previews = firstArray(presentation.actions).map(asRecord).filter((item) => Object.keys(item).length > 0);
  const summaries = firstArray(presentation.analysis_summary).map((item) => textValue(item, "")).filter(Boolean);
  const limitations = firstArray(presentation.limitations).map((item) => textValue(item, "")).filter(Boolean);
  const badge = resolved
    ? status.includes("cancel") ? "已取消" : status.includes("fail") ? "已失效" : "已确认"
    : `Proposal${revision > 0 ? ` · r${revision}` : ""}`;

  return (
    <div className={`action-card capability-proposal-card ${resolved ? "resolved" : "attention"}`}>
      <SlidersHorizontal size={16} />
      <div className="action-content">
        <div className="action-title-line capability-proposal-title">
          <strong>{title}</strong>
          <span>{badge}</span>
        </div>
        {conclusion && <p className="capability-proposal-conclusion">{conclusion}</p>}
        <div className="capability-proposal-facts" aria-label="方案摘要">
          {analyzedTracks > 0 && <span>已分析 {analyzedTracks} 轨</span>}
          <span>{actionCount} 项修改</span>
          {risk && <span>风险 {localizeDisplayText(risk)}</span>}
          <span>{reversible ? "可回滚" : "不可回滚"}</span>
        </div>
        {groups.length > 0 && (
          <div className="capability-proposal-groups">
            {groups.slice(0, 6).map((group, index) => {
              const label = textValue(group.label ?? group.role ?? group.function, `分组 ${index + 1}`);
              const count = finiteNumber(group.move_count ?? group.track_count, 0);
              const unit = textValue(group.unit, "");
              const min = finiteNumber(group.min_value, 0);
              const max = finiteNumber(group.max_value, 0);
              const range = min === max ? formatProposalValue(min, unit) : `${formatProposalValue(min, unit)} ～ ${formatProposalValue(max, unit)}`;
              return <span key={`${textValue(group.id, label)}-${index}`}><strong>{label}</strong>{count > 0 ? ` · ${count} 项` : ""}{unit ? ` · ${range}` : ""}</span>;
            })}
          </div>
        )}
        {(summaries.length > 0 || recommendation || readiness.length > 0 || previews.length > 0 || limitations.length > 0) && (
          <details className="capability-proposal-details">
            <summary>查看分析依据与逐轨修改</summary>
            {summaries.length > 0 && (
              <section>
                <h4>分析摘要</h4>
                <ul>{summaries.map((item, index) => <li key={`${item}-${index}`}>{item}</li>)}</ul>
              </section>
            )}
            {recommendation && <section><h4>推荐理由</h4><p>{recommendation}</p></section>}
            {readiness.length > 0 && (
              <section>
                <h4>证据覆盖</h4>
                <div className="capability-proposal-metrics">
                  {readiness.map((metric, index) => (
                    <span key={`${textValue(metric.id, "metric")}-${index}`}>
                      <strong>{textValue(metric.label, "指标")}</strong>
                      {textValue(metric.value, "—")}
                    </span>
                  ))}
                </div>
              </section>
            )}
            {previews.length > 0 && (
              <section>
                <h4>逐轨修改</h4>
                <div className="capability-proposal-preview-list">
                  {previews.map((preview, index) => {
                    const unit = textValue(preview.unit, "");
                    const before = finiteNumber(preview.before, 0);
                    const target = finiteNumber(preview.target, 0);
                    const delta = finiteNumber(preview.delta, 0);
                    return (
                      <div key={`${textValue(preview.action_id ?? preview.track_id, "change")}-${index}`}>
                        <strong>{textValue(preview.track_name ?? preview.track_id, `轨道 ${index + 1}`)}</strong>
                        <span>{formatProposalValue(before, unit)} → {formatProposalValue(target, unit)} ({formatProposalDelta(delta, unit)})</span>
                        {textValue(preview.reason, "") && <small>{textValue(preview.reason, "")}</small>}
                      </div>
                    );
                  })}
                </div>
              </section>
            )}
            {limitations.length > 0 && (
              <section className="capability-proposal-limitations">
                <h4>限制与风险</h4>
                <ul>{limitations.map((item, index) => <li key={`${item}-${index}`}>{item}</li>)}</ul>
              </section>
            )}
          </details>
        )}
        {!resolved && <p className="capability-proposal-hint">可直接回复“执行这个方案”，也可以继续提问、排除轨道或修改数值。</p>}
        {!resolved && childActions.length > 0 && (
          <div className="action-buttons capability-proposal-actions">
            {childActions.map((child, childIndex) => {
              const actionID = textValue(child.id ?? child.action_id ?? child.decision, `action_${childIndex + 1}`);
              const label = textValue(child.label ?? child.title ?? actionID, actionID);
              const pendingID = interactionActionID(interactionID || renderID || "proposal", actionID);
              const isBusy = respondingActionID === pendingID;
              return (
                <button
                  key={`${actionID}-${childIndex}`}
                  className={`action-button ${textValue(child.style, "secondary")}`}
                  type="button"
                  disabled={respondingActionID !== ""}
                  onClick={() => onInteractionAction(action, child, capabilityProposalInteractionPayload(action, child))}
                >
                  {isBusy && <Loader2 className="spin" size={14} />}
                  <span>{label}</span>
                </button>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

function formatProposalValue(value: number, unit: string): string {
  const suffix = unit.trim() ? ` ${unit.trim()}` : "";
  return `${value.toFixed(2)}${suffix}`;
}

function capabilityProposalInteractionPayload(interaction: JsonRecord, action: JsonRecord): JsonRecord {
  const parent = interactionPayload(interaction);
  return {
    ...interactionPayloadForAction(interaction, action, {}),
    workflow: "capability_runtime_v1",
    approval_mode: textValue(parent.approval_mode, "conversational"),
    conversation_id: textValue(interaction.conversation_id ?? parent.conversation_id, ""),
    goal_id: textValue(interaction.goal_id ?? parent.goal_id, ""),
    run_id: textValue(interaction.run_id ?? parent.run_id, ""),
    session_id: textValue(parent.session_id, ""),
    capability_id: textValue(parent.capability_id, ""),
    proposal_id: textValue(parent.proposal_id ?? interaction.plan_id, ""),
    proposal_revision: finiteNumber(parent.proposal_revision, 0),
    action_set_hash: textValue(parent.action_set_hash, ""),
    project_cut_hash: textValue(parent.project_cut_hash, "")
  };
}

function formatProposalDelta(value: number, unit: string): string {
  return `${value > 0 ? "+" : ""}${formatProposalValue(value, unit)}`;
}

function MixTreatmentPendingCard({ action, respondingActionID, onInteractionAction }: ActionCardProps) {
  const payload = interactionPayload(action);
  const display = asRecord(payload.display ?? action.display);
  const targetRef = textValue(display.target_ref ?? payload.display_target_ref ?? payload.target_ref ?? action.target_ref, "当前对象");
  const actionKind = textValue(payload.action_kind ?? action.action_kind, "");
  const processor = textValue(payload.processor_type ?? action.processor_type, "");
  const deltaDB = textValue(payload.delta_db ?? action.delta_db, "");
  const deltaPan = textValue(payload.delta_pan ?? action.delta_pan, "");
  const targetPan = textValue(payload.target_pan ?? action.target_pan, "");
  const confidence = textValue(display.confidence ?? payload.display_confidence ?? payload.confidence ?? action.confidence, "");
  const reason = textValue(display.reasoning_summary ?? payload.display_reasoning_summary ?? payload.reasoning_summary ?? action.reasoning_summary ?? action.body, "");
  const childActions = firstArray(action.actions)
    .map(asRecord)
    .filter((item) => Object.keys(item).length > 0);
  const interactionID = textValue(action.id ?? action.interaction_id, "");
  const renderID = actionRenderID(action);
  const pendingLabel = textValue(display.action_kind, "") || mixTreatmentPendingLabel(actionKind, processor);
  const valueLabel = mixTreatmentPendingValueLabel(actionKind, deltaDB, deltaPan, targetPan);
  const rows = [
    ["对象", targetRef],
    ["建议", pendingLabel],
    ["幅度", valueLabel],
    ["置信度", confidence ? localizeDisplayText(confidence) : "待确认"]
  ].filter(([, value]) => value !== "");

  return (
    <div className="action-card mix-treatment-pending-card">
      <SlidersHorizontal size={16} />
      <div className="action-content">
        <div className="action-title-line">
          <strong>混音建议待确认</strong>
          <span>待确认</span>
        </div>
        {reason && <p className="action-body">{localizeMixTreatmentCardText(reason)}</p>}
        <div className="action-detail-grid mix-treatment-pending-grid">
          {rows.map(([label, value]) => (
            <div className="action-detail" key={label}>
              <span>{label}</span>
              <strong>{value}</strong>
            </div>
          ))}
        </div>
        {childActions.length > 0 && (
          <div className="action-buttons">
            {childActions.map((child, childIndex) => {
              const actionID = textValue(child.id ?? child.action_id ?? child.decision, `action_${childIndex + 1}`);
              const label = localizeMixTreatmentCardText(textValue(child.label ?? child.title ?? actionID, actionID));
              const pendingID = interactionActionID(interactionID || renderID || "mix_treatment", actionID);
              const isBusy = respondingActionID === pendingID;
              const style = textValue(child.style, "secondary");
              return (
                <button
                  key={`${actionID}-${childIndex}`}
                  className={`action-button ${style}`}
                  type="button"
                  disabled={respondingActionID !== ""}
                  onClick={() => onInteractionAction(action, child, interactionPayloadForAction(action, child, {}))}
                >
                  {isBusy && <Loader2 className="spin" size={14} />}
                  <span>{label}</span>
                </button>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

function MixBoardActionCard({ action, respondingActionID, onInteractionAction }: ActionCardProps) {
  const payload = interactionPayload(action);
  const session = asRecord(payload.mix_session ?? action.mix_session);
  const observationEnvelope = asRecord(payload.mix_observation ?? action.mix_observation);
  const observation = asRecord(observationEnvelope.observation);
  const mixboard = asRecord(observationEnvelope.mixboard);
  const contextPack = asRecord(observationEnvelope.context_pack);
  const sessionHeader = asRecord(contextPack.session_header);
  const latestObservation = asRecord(contextPack.latest_observation);
  const target = asRecord(session.target_ref ?? observation.target_ref ?? mixboard.target_ref ?? sessionHeader.target_ref);
  const mixObjects = firstArray(session.mix_objects, observation.mix_objects, mixboard.mix_objects, sessionHeader.mix_objects).map(asRecord);
  const listenScope = asRecord(session.listen_scope ?? observation.listen_scope ?? mixboard.listen_scope ?? sessionHeader.listen_scope);
  const timeRuler = asRecord(observation.time_ruler ?? asRecord(contextPack.latest_observation).time_ruler);
  const sourceCapabilities = asRecord(observation.source_capabilities ?? latestObservation.source_capabilities);
  const globalSummary = asRecord(observation.global_summary ?? latestObservation.global_summary);
  const environmentPackage = asRecord(observation.environment_package ?? latestObservation.environment_package);
  const mixPackage = asRecord(observation.mix_package ?? latestObservation.mix_package);
  const deepPackage = asRecord(observation.deep_package ?? latestObservation.deep_package);
  const mixSourceCapabilities = asRecord(mixPackage.source_capabilities);
  const deepSourceCapabilities = asRecord(deepPackage.source_capabilities);
  const featureSnapshot = asRecord(globalSummary.feature_snapshot);
  const latestFeatureRequest = asRecord(featureSnapshot.latest_request);
  const snapshotWaveform = asRecord(featureSnapshot.waveform_envelope);
  const snapshotSpectrum = asRecord(featureSnapshot.spectrogram_tiles);
  const activeProblems = firstArray(mixboard.active_problem_map, contextPack.active_problem_map, observation.hotspots);
  const openBlockers = firstArray(mixboard.open_blockers, contextPack.open_blockers);
  const state = textValue(session.state ?? mixboard.status ?? observation.status, "unknown");
  const baseAcousticReady = textValue(sourceCapabilities.waveform_envelope, "").toLowerCase() === "ready";
  const effectiveBlockers = baseAcousticReady
    ? openBlockers.filter((item) => {
        const text = textValue(item, "").toLowerCase();
        return !(text.includes("waveform ready") && text.includes("spectrum"));
      })
    : openBlockers;
  const rawObservationStatus = textValue(observationEnvelope.status ?? observation.status ?? mixboard.status, state);
  const observationStatus = baseAcousticReady && rawObservationStatus === "partial" ? "ready" : rawObservationStatus;
  const blocker = textValue(effectiveBlockers[0] ?? (baseAcousticReady ? "" : session.blocking_point), "");
  const boardPath = textValue(observationEnvelope.board_path, "");
  const observationPath = textValue(observationEnvelope.observation_path, "");
  const contextPackPath = textValue(observationEnvelope.context_pack_path, "");
  const duration = numberFromKeys(timeRuler, "duration_seconds");
  const segment = numberFromKeys(timeRuler, "segment_seconds");
  const goalText = textValue(session.goal_text ?? mixboard.goal_text ?? sessionHeader.goal_text ?? payload.goal_text, "");
  const targetLabel = mixBoardTargetLabel(target);
  const statusLabel = mixObservationStatusLabel(observationStatus || state);
  const tone = observationStatus === "ready" ? "success" : observationStatus === "partial" ? "attention" : observationStatus === "unavailable" ? "error" : "info";
  const childActions = firstArray(action.actions)
    .map(asRecord)
    .filter((item) => Object.keys(item).length > 0)
    .filter((item) => textValue(item.id ?? item.action_id ?? item.decision, "") !== "done");
  const interactionID = textValue(action.id ?? action.interaction_id, "");
  const renderID = actionRenderID(action);
  const sessionID = mixBoardActionSessionID(action);
  const draftKey = sessionID || renderID || interactionID || "latest";
  const existingUserNote = mixBoardUserNoteLabel(session, mixboard, payload);
  const existingDraft = mixBoardUserNoteDrafts.get(draftKey);
  const [userNoteDraft, setUserNoteDraft] = useState(
    existingDraft ?? (existingUserNote === "等待用户听感，可在下一轮开始前补充" ? "" : existingUserNote)
  );
  const userNoteInputRef = useRef<HTMLTextAreaElement | null>(null);
  const latestTurn = mixBoardLatestTuneTurn(mixboard, observation, payload);
  const tuneRows = mixBoardTuneRows(latestTurn, mixboard);
  const payloadForMixAction = (): JsonRecord => {
    const note = (userNoteInputRef.current?.value ?? mixBoardUserNoteDrafts.get(draftKey) ?? userNoteDraft).trim();
    const outgoing: JsonRecord = {
      ...payload,
      mix_session: session,
      mix_observation: observationEnvelope,
      request_context: asRecord(payload.request_context),
      conversation_id: textValue(action.conversation_id, ""),
      goal_id: textValue(action.goal_id, ""),
      run_id: textValue(action.run_id, ""),
      _mixboard_client_build: WEBUI_BUILD_MARK
    };
    if (note) {
      outgoing.user_note = note;
      outgoing.fields = {
        ...asRecord(outgoing.fields),
        user_note: note
      };
    }
    console.info("[VitWebUI] MixBoard submit", {
      build: WEBUI_BUILD_MARK,
      session_id: sessionID,
      interaction_id: interactionID,
      draft_key: draftKey,
      user_note: note,
      payload: outgoing
    });
    return outgoing;
  };
  const mainRows = [
    ["混音目标", goalText || "等待当前轮次目标"],
    ["混音对象", mixBoardObjectsLabel(mixObjects, target)],
    ["试听范围", mixBoardListenScopeLabel(listenScope, duration)],
    ["用户听感 / 干预", existingUserNote],
    ["当前判断", mixBoardJudgementLabel(activeProblems, observationStatus, blocker)],
    ["当前动作", mixBoardCurrentActionLabel(session, mixboard, observation, payload, observationStatus)],
    ["下一步", mixBoardNextStepLabel(session, mixboard, payload, observationStatus, blocker)]
  ];
  const traffic = mixBoardTrafficState(observationStatus || state);
  const diagnostics = [
    ["UI", WEBUI_BUILD_MARK],
    ["Session", textValue(session.mix_session_id ?? observation.mix_session_id ?? mixboard.mix_session_id, "-")],
    ["Observation", textValue(observation.observation_id ?? observationEnvelope.observation_id ?? mixboard.latest_observation_id, "-")],
    ["时间尺", duration !== null && duration > 0 ? `${duration.toFixed(2)}s` : "待接入"],
    ["分段", segment !== null && segment > 0 ? `${segment.toFixed(2)}s` : "-"],
    ["环境包", mixBoardPackageStatusLabel(textValue(environmentPackage.status, "-"))],
    ["混音包", mixBoardPackageStatusLabel(textValue(mixPackage.status, "-"))],
    ["深度包", mixBoardPackageStatusLabel(textValue(deepPackage.status, "-"))],
    ["Waveform", mixObservationStatusLabel(textValue(sourceCapabilities.waveform_envelope, "missing"))],
    ["实时频段", mixObservationStatusLabel(textValue(mixSourceCapabilities.band_energy ?? sourceCapabilities.band_energy, "missing"))],
    ["声像/相干", mixObservationStatusLabel(textValue(mixSourceCapabilities.stereo_correlation ?? sourceCapabilities.stereo_relation, "missing"))],
    ["调参复查", mixObservationStatusLabel(textValue(mixSourceCapabilities.before_after_delta, "missing"))],
    ["深度频谱(异步)", mixObservationStatusLabel(textValue(deepSourceCapabilities.deep_band_observation ?? sourceCapabilities.spectrogram_tiles, "missing"))],
    ["Request", mixBoardFeatureRequestLabel(latestFeatureRequest)],
    ["Feature", textValue(featureSnapshot.updated_at, "-")],
    ["Peak/RMS", mixBoardPeakRmsLabel(globalSummary)],
    ["Board", boardPath || "-"],
    ["Observation", observationPath || "-"],
    ["Context", contextPackPath || "-"]
  ];

  console.info("[VitWebUI] MixBoardActionCard render", {
    build: WEBUI_BUILD_MARK,
    state,
    observationStatus,
    target: targetLabel,
    rows: mainRows.map(([label, value]) => ({ label, value })),
    boardPath,
    observationPath,
    contextPackPath
  });

  return (
    <div className={`action-card ${tone} mix-board-card`}>
      <Activity size={16} />
      <div className="action-content">
        <div className="action-title-line">
          <strong>MixBoard</strong>
          <span>{statusLabel}</span>
        </div>
        <p className="action-body">
          {targetLabel} 路 {mixSessionStateLabel(state)}
        </p>
        <label className="mix-board-intervention">
          <span>本轮听感干预</span>
          <textarea
            ref={userNoteInputRef}
            value={userNoteDraft}
            placeholder="例如：不要再降低音量，低频有点糊"
            rows={2}
            disabled={respondingActionID !== ""}
            onChange={(event) => {
              const next = event.currentTarget.value;
              mixBoardUserNoteDrafts.set(draftKey, next);
              setUserNoteDraft(next);
            }}
          />
        </label>
        <div className="action-detail-grid project-result-grid mix-board-main-grid">
          {mainRows.map(([label, value]) => (
            <div className="action-detail" key={label}>
              <span>{label}</span>
              <strong>{value}</strong>
            </div>
          ))}
          <div className="action-detail mix-board-status-detail">
            <span>观察状态</span>
            <strong>
              <i className={`mix-board-status-dot ${traffic}`} aria-hidden="true" />
              {mixBoardTrafficLabel(observationStatus || state, blocker)}
            </strong>
          </div>
        </div>
        {tuneRows.length > 0 && (
          <div className="mix-board-tune-panel">
            <div className="mix-board-panel-title">
              <SlidersHorizontal size={14} />
              <strong>调参轮次</strong>
            </div>
            <div className="action-detail-grid project-result-grid">
              {tuneRows.map(([label, value]) => (
                <div className="action-detail" key={label}>
                  <span>{label}</span>
                  <strong>{value}</strong>
                </div>
              ))}
            </div>
          </div>
        )}
        {childActions.length > 0 && (
          <div className="action-buttons mix-board-actions">
            {childActions.map((child, index) => {
              const actionID = textValue(child.id ?? child.action_id ?? child.decision, `action_${index + 1}`);
              const label = textValue(child.label ?? child.title ?? actionID, actionID);
              const style = textValue(child.style, "secondary");
              const pendingID = interactionActionID(interactionID || renderID || "mixboard", actionID);
              const isBusy = respondingActionID === pendingID;
              const disabled = respondingActionID !== "";
              return (
                <button
                  key={`${actionID}-${index}`}
                  className={`action-button ${style}`}
                  type="button"
                  disabled={disabled}
                  onClick={() => onInteractionAction(action, child, payloadForMixAction())}
                >
                  {isBusy && <Loader2 className="spin" size={14} />}
                  <span>{label}</span>
                </button>
              );
            })}
          </div>
        )}
        <details className="mix-board-diagnostics">
          <summary>诊断信息</summary>
          <div className="action-detail-grid project-result-grid">
            {diagnostics.map(([label, value], index) => (
              <KeyValue label={label} value={value} key={`${label}-${index}`} />
            ))}
          </div>
        </details>
      </div>
    </div>
  );
}

function mixBoardLatestTuneTurn(mixboard: JsonRecord, observation: JsonRecord, payload: JsonRecord): JsonRecord {
  const rows = firstArray(mixboard.auto_tune_turns, observation.auto_tune_turns, payload.auto_tune_turns).map(asRecord);
  return rows.length > 0 ? rows[rows.length - 1] : {};
}

function mixBoardTuneRows(turn: JsonRecord, mixboard: JsonRecord): Array<[string, string]> {
  const rows: Array<[string, string]> = [];
  const turnNumber = numberFromKeys(turn, "turn");
  const control = textValue(turn.control_label, "") || mixBoardControlLabel(textValue(turn.control, ""));
  const target = textValue(turn.target_label ?? turn.track_label ?? turn.track_id, "");
  const before = numberFromKeys(turn, "before_db");
  const after = numberFromKeys(turn, "target_db", "after_db");
  const step = numberFromKeys(turn, "step_db");
  const status = textValue(turn.status, "");
  const review = textValue(turn.review_status ?? mixboard.review_status, "");
  rows.push(["执行器", mixBoardExecutorLabel(textValue(turn.executor_type ?? mixboard.executor_type, ""), textValue(turn.executor_version ?? mixboard.executor_version, ""))]);
  if (turnNumber !== null) {
    rows.push(["轮次", `第 ${turnNumber} 轮`]);
  }
  if (target) {
    rows.push(["调控对象", target]);
  }
  if (control) {
    rows.push(["参数", control]);
  }
  if (before !== null && after !== null) {
    rows.push(["参数变化", `${before.toFixed(2)} dB -> ${after.toFixed(2)} dB`]);
  }
  if (step !== null) {
    rows.push(["步进", `${step > 0 ? "+" : ""}${step.toFixed(2)} dB`]);
  }
  const direction = textValue(turn.direction ?? mixboard.mix_tuning_direction, "");
  if (direction) {
    rows.push(["方向", mixBoardTuneDirectionLabel(direction)]);
  }
  const source = textValue(turn.direction_source ?? mixboard.mix_tuning_direction_source, "");
  if (source) {
    rows.push(["方向来源", mixBoardTuneDirectionSourceLabel(source)]);
  }
  const userNote = textValue(turn.user_note ?? mixboard.user_note ?? mixboard.mix_tuning_user_note, "");
  if (userNote) {
    rows.push(["用户干预", userNote]);
  }
  if (status) {
    rows.push(["执行状态", mixBoardTuneStatusLabel(status)]);
  }
  if (review) {
    rows.push(["复查状态", mixBoardReviewStatusLabel(review)]);
  }
  if (rows.length === 0) {
    const summary = textValue(mixboard.last_action_summary, "");
    if (summary) {
      rows.push(["最近动作", summary]);
    }
  }
  return rows;
}

function mixBoardControlLabel(value: string): string {
  const control = value.toLowerCase();
  if (control === "track.volume") {
    return "轨道音量";
  }
  return value;
}

function mixBoardExecutorLabel(type: string, version: string): string {
  const normalized = type.toLowerCase();
  if (normalized === "fallback_executor") {
    return version ? `Fallback 小步调参 · ${version}` : "Fallback 小步调参";
  }
  if (normalized === "llm_executor") {
    return version ? `LLM 调参执行器 · ${version}` : "LLM 调参执行器";
  }
  return version || type || "未指定";
}

function mixBoardTuneDirectionLabel(value: string): string {
  const direction = value.toLowerCase();
  if (direction === "raise_volume") {
    return "增大音量";
  }
  if (direction === "lower_volume") {
    return "降低音量";
  }
  if (direction === "hold") {
    return "保持不动";
  }
  return localizeDisplayText(value) || value;
}

function mixBoardTuneDirectionSourceLabel(value: string): string {
  const source = value.toLowerCase();
  if (source === "user_note") {
    return "用户干预";
  }
  if (source === "goal_text") {
    return "混音目标";
  }
  if (source === "explicit_step_db") {
    return "鏄惧紡步进";
  }
  if (source === "fallback_default") {
    return "默认小步";
  }
  if (source === "fallback_rule_or_user_note") {
    return "规则判断";
  }
  return localizeDisplayText(value) || value;
}

function mixBoardTuneStatusLabel(value: string): string {
  const status = value.toLowerCase();
  if (status === "ok") {
    return "已执行";
  }
  if (status === "failed" || status === "error") {
    return "执行失败";
  }
  return localizeDisplayText(value) || value;
}

function mixBoardReviewStatusLabel(value: string): string {
  const status = value.toLowerCase();
  if (status === "waiting_review") {
    return "等待复查";
  }
  if (status === "reviewed") {
    return "已复查";
  }
  if (status === "effective") {
    return "变化有效";
  }
  if (status === "not_obvious") {
    return "变化不明显";
  }
  if (status === "rolled_back") {
    return "已撤回";
  }
  if (status === "failed") {
    return "复查失败";
  }
  return localizeDisplayText(value) || value;
}

function mixBoardPeakRmsLabel(summary: JsonRecord): string {
  const peak = textValue(summary.peak_dbfs, "");
  const rms = textValue(summary.rms_dbfs, "");
  if (peak || rms) {
    return `Peak ${peak || "-"} dBFS / RMS ${rms || "-"} dBFS`;
  }
  const snap = asRecord(summary.feature_snapshot);
  const waveform = asRecord(snap.waveform_envelope);
  return textValue(waveform.status, "未接入");
}

function isMixBoardAction(action: JsonRecord): boolean {
  const kind = textValue(action.kind ?? action.type, "").toLowerCase();
  const type = textValue(action.type, "").toLowerCase();
  const workflow = textValue(action.workflow, "").toLowerCase();
  const source = textValue(action.source, "").toLowerCase();
  const payload = interactionPayload(action);
  const payloadKind = textValue(payload.kind ?? payload.type, "").toLowerCase();
  const payloadType = textValue(payload.type, "").toLowerCase();
  const identifiers = [kind, type, payloadKind, payloadType].filter(Boolean);
  const hasMixBoardKind = identifiers.some((value) =>
    value === "mix_board" ||
    value === "mixboard" ||
    value === "mixboard_status" ||
    value.includes("mixboard")
  );
  const hasMixBoardPayload =
    Object.keys(asRecord(action.mix_observation)).length > 0 ||
    Object.keys(asRecord(payload.mix_observation)).length > 0;
  if ((kind === "mix_session_entry" || type === "mix_session_entry") || (isConfirmationAction(action) && !hasMixBoardKind && !hasMixBoardPayload)) {
    return false;
  }
  return hasMixBoardKind ||
    hasMixBoardPayload ||
    (source === "mix_session" && workflow === "mix_session_entry" && type === "mixboard_status");
}

function isMixPlanningDiscussionAction(action: JsonRecord): boolean {
  const kind = textValue(action.kind ?? action.type, "").toLowerCase();
  const type = textValue(action.type, "").toLowerCase();
  return kind === "mix_planning_discussion" || type === "mix_planning_discussion";
}

function mixSessionStateLabel(value: unknown): string {
  const state = textValue(value, "").toLowerCase();
  if (state === "observation_ready") {
    return "声学观察已就绪";
  }
  if (state === "observation_partial") {
    return "声学观察受限";
  }
  if (state === "observation_unavailable") {
    return "等待声学观察";
  }
  if (state === "ready_for_observation") {
    return "等待观察";
  }
  if (state === "tuning_running") {
    return "自动调控中";
  }
  if (state === "waiting_review") {
    return "等待试听复查";
  }
  if (state === "tuning_paused") {
    return "调控已暂停";
  }
  if (state === "rollback_available") {
    return "可撤回上一轮";
  }
  if (state === "tuning_complete") {
    return "本轮调控完成";
  }
  if (state === "tuning_stopped") {
    return "调控已暂停";
  }
  return localizeDisplayText(state) || state || "未知";
}

function mixObservationStatusLabel(value: unknown): string {
  const status = textValue(value, "").toLowerCase();
  if (status === "ready") {
    return "就绪";
  }
  if (status === "partial") {
    return "受限";
  }
  if (status === "requested") {
    return "读取中";
  }
  if (status === "blocked") {
    return "等待可分析片段";
  }
  if (status === "unavailable") {
    return "等待观察";
  }
  if (status === "missing") {
    return "缺失";
  }
  return localizeDisplayText(status) || status || "-";
}

function mixBoardFeatureRequestLabel(request: JsonRecord): string {
  const status = textValue(request.status, "").toLowerCase();
  const count = firstArray(request.requested_features).length;
  if (status === "requested") {
    return count > 0 ? `已请求 ${count} 项声学特征` : "已请求声学特征";
  }
  if (status === "blocked") {
    return mixBoardBlockerLabel(textValue(request.reason, "声学特征请求受限"));
  }
  return status || "-";
}

function mixBoardPackageStatusLabel(value: string): string {
  const status = value.toLowerCase();
  if (status === "ready") {
    return "就绪";
  }
  if (status === "partial") {
    return "部分就绪";
  }
  if (status === "baseline_ready") {
    return "基础闭环就绪";
  }
  if (status === "limited") {
    return "指标受限";
  }
  if (status === "async_available") {
    return "异步可用";
  }
  if (status === "async_missing") {
    return "异步待补齐";
  }
  return value || "-";
}

function mixBoardTargetLabel(target: JsonRecord): string {
  const label = textValue(target.label, "");
  if (label) {
    return label;
  }
  const kind = textValue(target.kind, "").toLowerCase();
  if (kind === "track") {
    return "当前轨道";
  }
  if (kind === "clip") {
    return "当前片段";
  }
  if (kind === "project" || kind === "song") {
    return "全曲";
  }
  return textValue(target.id, "当前混音对象");
}

function mixBoardObjectsLabel(objects: JsonRecord[], target: JsonRecord): string {
  if (objects.length === 0) {
    return mixBoardTargetLabel(target);
  }
  const labels = objects.slice(0, 3).map((object) => {
    const label = textValue(object.label, "") || mixBoardObjectKindLabel(textValue(object.kind, ""));
    const scope = mixBoardEffectScopeLabel(textValue(object.effect_scope, ""));
    return scope ? `${label}（${scope}）` : label;
  });
  const extra = objects.length > labels.length ? ` 等 ${objects.length} 个对象` : "";
  return `${labels.join("、")}${extra}`;
}

function mixBoardObjectKindLabel(kindValue: string): string {
  const kind = kindValue.toLowerCase();
  if (kind === "track") {
    return "轨道";
  }
  if (kind === "clip") {
    return "片段";
  }
  if (kind === "bus") {
    return "鎬荤嚎";
  }
  return kindValue || "混音对象";
}

function mixBoardEffectScopeLabel(scopeValue: string): string {
  const scope = scopeValue.toLowerCase();
  if (scope === "track_scope") {
    return "轨道处理";
  }
  if (scope === "clip_scope") {
    return "片段处理";
  }
  return "";
}

function mixBoardListenScopeLabel(scope: JsonRecord, duration: number | null): string {
  const time = asRecord(scope.time);
  const source = asRecord(scope.source);
  const timeMode = textValue(time.mode, "").toLowerCase();
  let timeLabel = "全曲";
  if (timeMode === "manual_time") {
    const start = numberFromKeys(time, "start_seconds");
    const end = numberFromKeys(time, "end_seconds");
    timeLabel = start !== null && end !== null ? `${start.toFixed(2)}s - ${end.toFixed(2)}s` : "选定时间段";
  } else if (timeMode === "full_song" && duration !== null && duration > 0) {
    timeLabel = `全曲 ${duration.toFixed(2)}s`;
  } else if (timeMode) {
    timeLabel = localizeDisplayText(timeMode) || timeMode;
  }

  const sourceMode = textValue(source.mode, "").toLowerCase();
  let sourceLabel = "全轨混音";
  const focusCount = firstArray(source.focus_ids).length;
  if (sourceMode === "target_only" || sourceMode === "focused_targets") {
    sourceLabel = focusCount > 1 ? `${focusCount} 个选中对象` : "混音对象";
  } else if (sourceMode === "selected_tracks") {
    sourceLabel = focusCount > 1 ? `${focusCount} 条选中轨道` : "选中轨道";
  } else if (sourceMode === "selected_clips") {
    sourceLabel = focusCount > 1 ? `${focusCount} 个选中片段` : "选中片段";
  } else if (sourceMode && sourceMode !== "full_mix_context") {
    sourceLabel = localizeDisplayText(sourceMode) || sourceMode;
  }
  return `${sourceLabel} 路 ${timeLabel}`;
}

function mixBoardUserNoteLabel(session: JsonRecord, mixboard: JsonRecord, payload: JsonRecord): string {
  return (
    textValue(session.user_note ?? session.user_feedback ?? session.subjective_note, "") ||
    textValue(mixboard.user_note ?? mixboard.user_feedback ?? mixboard.subjective_note, "") ||
    textValue(payload.user_note ?? payload.user_feedback ?? payload.subjective_note, "") ||
    "等待用户听感，可在下一轮开始前补充"
  );
}

function mixBoardJudgementLabel(problems: unknown[], status: string, blocker: string): string {
  const problemLabels = problems
    .map(asRecord)
    .map((problem) => textFromKeys(problem, "summary", "label", "problem", "tag", "kind"))
    .filter((value) => value.length > 0)
    .slice(0, 2);
  if (problemLabels.length > 0) {
    return problemLabels.join("；");
  }
  if (blocker) {
    return mixBoardBlockerLabel(blocker);
  }
  const normalized = status.toLowerCase();
  if (normalized === "requested") {
    return "声学特征已请求，等待内核回传波形与频谱";
  }
  if (normalized === "blocked") {
    return blocker ? mixBoardBlockerLabel(blocker) : "需要可分析的音频片段后才能读取声学特征";
  }
  if (normalized === "partial") {
    return "声学观察部分就绪，先按已接入特征与时间尺判断";
  }
  if (normalized === "unavailable") {
    return "缺少时间尺或声学特征，暂不做音色判断";
  }
  return "暂无突出问题";
}

function mixBoardCurrentActionLabel(session: JsonRecord, mixboard: JsonRecord, observation: JsonRecord, payload: JsonRecord, status: string): string {
  const actionText =
    textValue(session.current_action ?? session.action, "") ||
    textValue(mixboard.current_action ?? mixboard.action, "") ||
    textValue(observation.current_action ?? observation.action, "") ||
    textValue(payload.current_action ?? payload.action, "");
  if (actionText) {
    return actionText;
  }
  if (status === "requested") {
    return "已向内核请求波形包络与频谱场，等待回传";
  }
  if (status === "blocked") {
    return "等待选择可分析的音频片段或恢复内核连接";
  }
  return status === "ready" ? "等待根据目标执行效果器微调" : "建立 MixBoard，等待声学特征与下一轮调控";
}

function mixBoardNextStepLabel(session: JsonRecord, mixboard: JsonRecord, payload: JsonRecord, status: string, blocker: string): string {
  const nextText =
    textValue(session.next_step ?? session.next_action, "") ||
    textValue(mixboard.next_step ?? mixboard.next_action, "") ||
    textValue(payload.next_step ?? payload.next_action, "");
  if (nextText) {
    return nextText;
  }
  if (blocker) {
    return "接入声学特征读取后刷新观察，再进入参数调控轮";
  }
  if (status === "requested") {
    return "等待特征回传后刷新观察";
  }
  if (status === "blocked") {
    return "先指定含音频片段的混音对象";
  }
  return status === "ready" ? "进入监听、调参、复查循环" : "先确认混音对象与试听范围";
}

function mixBoardTrafficLabel(value: unknown, blocker: string): string {
  const status = textValue(value, "").toLowerCase();
  if (status === "ready") {
    return "声学观察就绪";
  }
  if (status === "requested") {
    return "声学特征已请求，等待回传";
  }
  if (status === "blocked") {
    return blocker ? mixBoardBlockerLabel(blocker) : "需要可分析的音频片段";
  }
  if (status === "partial") {
    return blocker ? mixBoardBlockerLabel(blocker) : "工程与时间尺已就绪，声学观察受限";
  }
  if (status === "unavailable") {
    return blocker ? mixBoardBlockerLabel(blocker) : "等待声学观察";
  }
  return mixObservationStatusLabel(status);
}

function mixBoardTrafficState(value: unknown): string {
  const status = textValue(value, "").toLowerCase();
  if (status === "ready" || status === "ok" || status === "observation_ready" || status === "tuning_complete") {
    return "ready";
  }
  if (status === "partial" || status === "requested" || status === "observation_partial" || status === "tuning_running" || status === "tuning_stopped") {
    return "partial";
  }
  if (status === "blocked" || status === "unavailable" || status === "observation_unavailable" || status === "missing" || status === "failed" || status === "error") {
    return "unavailable";
  }
  return status ? "partial" : "unavailable";
}

function mixBoardBlockerLabel(value: string): string {
  const blocker = value.toLowerCase();
  if (blocker.includes("waveform ready") && blocker.includes("spectrum")) {
    return "基础声学包已接入，深度频段观察可稍后补充";
  }
  if (blocker.includes("spectrum ready") && blocker.includes("waveform")) {
    return "频谱观察已接入，波形包络待接入";
  }
  if (blocker.includes("partially connected")) {
    return "声学观察部分接入，仍需补齐特征包";
  }
  if (blocker.includes("audio_feature_request_pending")) {
    return "声学特征已请求，等待内核回传";
  }
  if (blocker.includes("audio_feature_request_blocked")) {
    return "声学特征请求受限，需要明确音频片段源";
  }
  if (blocker.includes("clip_source_required")) {
    return "当前特征提取需要明确的音频片段源";
  }
  if (blocker.includes("kernel_client_unavailable")) {
    return "内核连接不可用，暂不能读取声学特征";
  }
  if (blocker.includes("all_feature_requests_skipped")) {
    return "声学特征请求未成功发送";
  }
  if (blocker === "audio_feature_reader_not_connected" || blocker.includes("audio feature")) {
    return "声学特征读取尚未接通，当前只能按工程和时间尺判断";
  }
  if (blocker === "feature_source_unavailable") {
    return "声学特征来源暂不可用";
  }
  return localizeDisplayText(value) || value;
}


function ProjectResultCard({ action, respondingActionID, onInvoke, uiState }: ActionCardProps) {
  const result = projectResultSummary(action, uiState);
  const [busyAction, setBusyAction] = useState("");
  if (!result) {
    return null;
  }
  const disabled = respondingActionID !== "" || busyAction !== "";
  const readinessTone =
    result.readiness === "ready" ? "success" : result.readiness === "silent_risk" || result.readiness === "affected_by_mute" ? "attention" : "info";
  return (
    <div className={`action-card project-result ${readinessTone}`}>
      <Radio size={16} />
      <div className="action-content">
        <div className="action-title-line">
          <strong>{result.title}</strong>
          <span>{result.badge}</span>
        </div>
        {result.body && <p className="action-body">{result.body}</p>}
        <div className="action-detail-grid project-result-grid">
          {result.details.map((item) => (
            <div className="action-detail" key={item.id}>
              <span>{item.title}</span>
              <strong>{item.body}</strong>
            </div>
          ))}
        </div>
        {result.hint && <p className="project-result-hint">{result.hint}</p>}
        <div className="action-buttons action-quick-buttons">
          <button
            className="action-button secondary"
            type="button"
            disabled={disabled}
            onClick={() => {
              setBusyAction("focus");
              void focusDawTarget(onInvoke, result.target)
                .catch((error) => console.warn("[AskVit] project result focus failed", error))
                .finally(() => setBusyAction(""));
            }}
          >
            {busyAction === "focus" ? <Loader2 className="spin" size={14} /> : <ChevronRight size={14} />}
            <span>跳转</span>
          </button>
          {result.canAudition && (
            <button
              className={result.readiness === "ready" ? "action-button primary" : "action-button secondary"}
              type="button"
              disabled={disabled}
              onClick={() => {
                setBusyAction("audition");
                void auditionDawTarget(onInvoke, result.target)
                  .catch((error) => console.warn("[AskVit] project result audition failed", error))
                  .finally(() => setBusyAction(""));
              }}
            >
              {busyAction === "audition" ? <Loader2 className="spin" size={14} /> : <Play size={14} />}
              <span>{result.auditionLabel}</span>
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

function isProjectResultAction(action: JsonRecord): boolean {
  return textValue(action.kind ?? action.type, "").toLowerCase() === "project_result";
}

function projectResultSummary(action: JsonRecord, uiState: AgentUIState | null): ProjectResultView | null {
  const executions = projectResultExecutions(action);
  if (executions.length === 0) {
    return null;
  }
  const target = projectResultTarget(action, executions, uiState);
  if (!target) {
    return null;
  }
  const track = projectResultTrack(uiState, textValue(target.track_id, ""));
  const clip = projectResultClip(uiState, textValue(target.clip_id, ""), textValue(target.track_id, ""));
  const operation = projectResultOperation(executions, target, clip);
  const readiness = projectResultReadiness(target, track, clip);
  const canAudition = readiness !== "not_auditionable";
  const abResult = projectResultABView(action);
  const details = projectResultDetails(target, track, clip, executions, abResult);
  return {
    title: operation.title,
    badge: projectResultReadinessBadge(readiness),
    body: projectResultBodyWithAB(operation.body, abResult),
    hint: projectResultReadinessHint(readiness, track, clip),
    readiness,
    canAudition,
    auditionLabel: readiness === "ready" ? "试听" : readiness === "not_auditionable" ? "" : "试听（可能无声）",
    target,
    details
  };
}

function projectResultExecutions(action: JsonRecord): JsonRecord[] {
  return firstArray(action.executions, action.executed_kernel_reply)
    .map(asRecord)
    .filter((entry) => Object.keys(entry).length > 0);
}

function projectResultABView(action: JsonRecord): ProjectResultABView | null {
  const ab = asRecord(action.ab_result ?? action.mom_ab_result ?? action.result_ab);
  if (Object.keys(ab).length === 0) {
    return null;
  }
  const status = textValue(ab.status, "missing").toLowerCase();
  const title = textValue(ab.display_title, status === "ready" ? "AB Result：可信" : "AB Result：不可信");
  const body = textValue(ab.display_body, "");
  return {
    title,
    body,
    status,
    tapPoint: textValue(ab.tap_point, ""),
    renderMode: textValue(ab.render_mode, ""),
    deltaSummary: textValue(ab.delta_summary, "")
  };
}

function projectResultBodyWithAB(body: string, abResult: ProjectResultABView | null): string {
  if (!abResult) {
    return body;
  }
  const abText = abResult.title;
  return body ? `${body} ${abText}` : abText;
}

function projectResultActionsFromResponse(response: ChatResponse): JsonRecord[] {
  if (responseNeedsConfirmationFallback(response)) {
    return [];
  }
  const explicitCards = firstArray(response.project_result_cards)
    .map(asRecord)
    .filter((card) => Object.keys(card).length > 0)
    .map((card, index) => ({
      ...card,
      _ui_source: "project_result",
      kind: "project_result",
      type: "project_result",
      id: textValue(card.id, `project_result_${textValue(response.goal_id ?? response.run_id ?? response.conversation_id, String(index))}`)
    }));
  if (explicitCards.length > 0) {
    return explicitCards;
  }
  const executions = firstArray(response.executed_kernel_reply)
    .map(asRecord)
    .filter(projectResultExecutionCandidate);
  if (executions.length === 0) {
    return [];
  }
  return [
    {
      _ui_source: "project_result",
      kind: "project_result",
      id: `project_result_${textValue(response.goal_id ?? response.run_id ?? response.conversation_id, uniqueID("project_result"))}`,
      executions
    }
  ];
}

function projectResultExecutionCandidate(entry: JsonRecord): boolean {
  const result = asRecord(entry.result);
  const payload = Object.keys(result).length > 0 ? result : entry;
  if (!agentExecutionSucceeded(entry, payload)) {
    return false;
  }
  const commandName = projectResultCommandName(entry, payload);
  if (projectResultReadOnlyCommand(commandName)) {
    return false;
  }
  const uiAction = textValue(payload.ui_action ?? entry.ui_action, "").toLowerCase();
  return actionLooksLikeDawTarget(commandName, uiAction, mergeJsonRecords(entry, payload));
}

function projectResultCommandName(entry: JsonRecord, payload: JsonRecord): string {
  const verification = asRecord(entry.verification);
  return textValue(
    entry.command_name ??
      entry.tool ??
      entry.cmd ??
      entry.command ??
      payload.command_name ??
      payload.tool ??
      payload.cmd ??
      payload.command ??
      verification.command_name ??
      verification.tool,
    ""
  ).toLowerCase();
}

function projectResultReadOnlyCommand(commandName: string): boolean {
  if (!commandName) {
    return false;
  }
  return (
    commandName.startsWith("get_") ||
    commandName.startsWith("list_") ||
    commandName.includes(".list") ||
    commandName.includes(".read") ||
    commandName.includes("read_clip") ||
    commandName.includes("project.state") ||
    commandName.includes("artifact.") ||
    commandName.includes("media.index") ||
    commandName.includes("media.register") ||
    commandName.includes("browser.")
  );
}

function projectResultTarget(action: JsonRecord, executions: JsonRecord[], uiState: AgentUIState | null): JsonRecord | null {
  let target: JsonRecord = asRecord(action.target);
  executions.forEach((entry) => {
    const next = actionDawTarget({ ...entry, _ui_source: "executed" }, uiState);
    if (!next) {
      return;
    }
    target = mergeProjectTargets(target, next);
  });
  if (!textValue(target.track_id ?? target.clip_id ?? target.plugin_id, "")) {
    return null;
  }
  return target;
}

function mergeProjectTargets(current: JsonRecord, incoming: JsonRecord): JsonRecord {
  const out = { ...current };
  const incomingClip = textValue(incoming.clip_id, "");
  const currentClip = textValue(out.clip_id, "");
  const incomingPlugin = textValue(incoming.plugin_id, "");
  const currentPlugin = textValue(out.plugin_id, "");
  if (incomingClip || !currentClip) {
    copyProjectTargetValue(out, incoming, "clip_id");
  }
  if (incomingPlugin || (!incomingClip && !currentPlugin)) {
    copyProjectTargetValue(out, incoming, "plugin_id");
  }
  for (const key of ["track_id", "start_seconds", "start_beats", "can_audition"]) {
    if (incoming[key] !== undefined && (out[key] === undefined || incomingClip || incomingPlugin)) {
      out[key] = incoming[key];
    }
  }
  return out;
}

function copyProjectTargetValue(out: JsonRecord, incoming: JsonRecord, key: string): void {
  const value = textValue(incoming[key], "");
  if (value) {
    out[key] = value;
  }
}

function projectResultTrack(uiState: AgentUIState | null, trackID: string): DawTrack | null {
  if (!trackID) {
    return null;
  }
  return dawTracksFromUIState(uiState).find((track) => track.id === trackID) ?? null;
}

function projectResultClip(uiState: AgentUIState | null, clipID: string, trackID = ""): DawClip | null {
  if (!clipID) {
    return null;
  }
  for (const track of dawTracksFromUIState(uiState)) {
    if (trackID && track.id !== trackID) {
      continue;
    }
    const clip = track.clips.find((item) => item.id === clipID);
    if (clip) {
      return clip;
    }
  }
  return null;
}

function projectResultOperation(executions: JsonRecord[], target: JsonRecord, clip: DawClip | null): { title: string; body: string } {
  const commands = executions.map((entry) => {
    const result = asRecord(entry.result);
    return projectResultCommandName(entry, Object.keys(result).length > 0 ? result : entry);
  });
  const joined = commands.join(" ");
  const noteCount = projectResultNoteCount(executions);
  const hasClip = textValue(target.clip_id, "") !== "";
  const hasPlugin = textValue(target.plugin_id, "") !== "";
  const hasTrack = textValue(target.track_id, "") !== "";
  if (joined.includes("apply_midi_note_patch") || joined.includes("add_midi_notes") || noteCount > 0) {
    const title = joined.includes("create_midi_clip") || joined.includes("midi.create_clip") ? "MIDI 片段已创建并写入" : "MIDI 片段已更新";
    return { title, body: noteCount > 0 ? `已写入或更新 ${noteCount} 个音符。` : "已更新 MIDI 音符。" };
  }
  if (joined.includes("create_midi_clip") || joined.includes("midi.create_clip")) {
    return { title: "MIDI 片段已创建", body: "已在工程中创建 MIDI 片段。" };
  }
  if (joined.includes("import_media") || joined.includes("import_audio") || joined.includes("clip.import")) {
    return { title: "媒体片段已导入", body: "已把素材放入工程时间线。" };
  }
  if (joined.includes("mix.apply_tick") || joined.includes("mix_apply_tick")) {
    if (joined.includes("pan")) {
      return { title: "混音声像已更新", body: "已执行一次可撤销的轨道声像调整。" };
    }
    if (joined.includes("gain") || joined.includes("volume")) {
      return { title: "混音电平已更新", body: "已执行一次可撤销的轨道电平调整。" };
    }
    return { title: "混音步骤已执行", body: "已执行一次可撤销的小型混音调整。" };
  }
  if (hasClip) {
    return { title: clip && isMidiClip(clip) ? "MIDI 片段已更新" : "片段已更新", body: "工程片段已有可定位结果。" };
  }
  if (hasPlugin || joined.includes("plugin") || joined.includes("rack") || joined.includes("control")) {
    return { title: "声音处理已更新", body: "已修改插件、机架或宏控制相关设置。" };
  }
  if (hasTrack) {
    return { title: joined.includes("add_track") ? "轨道已创建" : "轨道已更新", body: "工程轨道已有可定位结果。" };
  }
  return { title: "工程已更新", body: "本次任务已修改工程。" };
}

function projectResultNoteCount(executions: JsonRecord[]): number {
  return executions.reduce((total, entry) => {
    const result = asRecord(entry.result);
    const payload = Object.keys(result).length > 0 ? result : entry;
    const direct = numericValue(payload.inserted_count ?? payload.added_count ?? payload.mutated_count ?? payload.note_count ?? payload.notes_count);
    const notes = firstArray(payload.notes, payload.midi_notes, payload.note_rows);
    return total + Math.max(direct, notes.length);
  }, 0);
}

function projectResultReadiness(target: JsonRecord, track: DawTrack | null, clip: DawClip | null): ProjectResultReadiness {
  if (!truthy(target.can_audition)) {
    return "not_auditionable";
  }
  if (track?.mute) {
    return "affected_by_mute";
  }
  if (clip) {
    if (!isMidiClip(clip)) {
      return "ready";
    }
    if (!track) {
      return "unknown";
    }
    return trackHasLikelyInstrument(track) ? "ready" : "silent_risk";
  }
  if (track) {
    if (track.clips.length === 0) {
      return "not_auditionable";
    }
    if (track.clips.some((item) => !isMidiClip(item))) {
      return "ready";
    }
    return trackHasLikelyInstrument(track) ? "ready" : "silent_risk";
  }
  return "unknown";
}

function projectResultReadinessBadge(readiness: ProjectResultReadiness): string {
  switch (readiness) {
    case "ready":
      return "可试听";
    case "silent_risk":
      return "可能无声";
    case "affected_by_mute":
      return "受静音影响";
    case "not_auditionable":
      return "可跳转";
    default:
      return "状态未知";
  }
}

function projectResultReadinessHint(readiness: ProjectResultReadiness, track: DawTrack | null, clip: DawClip | null): string {
  if (readiness === "silent_risk" && clip && isMidiClip(clip)) {
    return "当前轨道没有检测到乐器插件，MIDI 片段播放时可能没有声音。";
  }
  if (readiness === "affected_by_mute") {
    return "目标轨道当前处于静音状态，试听前可能需要取消静音。";
  }
  if (readiness === "unknown") {
    return "可以定位并尝试试听，但当前路由或音源状态无法完全确认。";
  }
  if (readiness === "not_auditionable" && track?.clips.length === 0) {
    return "当前目标没有可播放片段，已提供跳转入口。";
  }
  return "";
}

function projectResultDetails(
  target: JsonRecord,
  track: DawTrack | null,
  clip: DawClip | null,
  executions: JsonRecord[],
  abResult: ProjectResultABView | null
): Array<{ id: string; title: string; body: string }> {
  const details: Array<{ id: string; title: string; body: string }> = [];
  if (abResult) {
    details.push({ id: "ab_result", title: "AB Result", body: abResult.status === "ready" ? "可信" : "不可信" });
    if (abResult.deltaSummary) {
      details.push({ id: "ab_delta", title: "AB 变化", body: abResult.deltaSummary });
    }
    if (abResult.tapPoint) {
      details.push({ id: "ab_tap", title: "观测点", body: abResult.tapPoint });
    }
    if (abResult.renderMode) {
      details.push({ id: "ab_render", title: "观测方式", body: abResult.renderMode });
    }
  }
  const trackLabel = track?.name || textValue(target.track_id, "");
  const clipLabel = clip?.name || textValue(target.clip_id, "");
  if (trackLabel) {
    details.push({ id: "track", title: "轨道", body: trackLabel });
  }
  if (clipLabel) {
    details.push({ id: "clip", title: "片段", body: clipLabel });
  }
  const startSeconds = finiteNumber(target.start_seconds, NaN);
  if (Number.isFinite(startSeconds)) {
    details.push({ id: "start", title: "起点", body: formatTimelineSeconds(startSeconds) });
  }
  if (clip) {
    details.push({ id: "duration", title: "长度", body: formatTimelineSeconds(clip.lengthSeconds) });
  }
  const noteCount = projectResultNoteCount(executions);
  if (noteCount > 0) {
    details.push({ id: "notes", title: "音符", body: `${noteCount} 个` });
  }
  return details.slice(0, 6);
}

function trackHasLikelyInstrument(track: DawTrack): boolean {
  if (track.plugins.length === 0) {
    return false;
  }
  return track.plugins.some(pluginLooksLikeInstrument);
}

function pluginLooksLikeInstrument(plugin: DawPlugin): boolean {
  const text = [plugin.name, plugin.kind, plugin.role, plugin.path, plugin.vendor, plugin.source].join(" ").toLowerCase();
  if (!text.trim()) {
    return false;
  }
  if (/(^|\b)(eq|equalizer|compressor|limiter|reverb|delay|chorus|flanger|phaser|distortion|saturat|gate|filter|meter|analyzer|utility|gain|clipper|tape)(\b|$)/.test(text)) {
    return false;
  }
  return /(instrument|synth|sampler|rompler|piano|keys|keyboard|organ|drum|kit|bass|guitar|strings|brass|kontakt|serum|massive|spire|omnisphere|vital|dexed|pigments|labs|sine)/.test(text);
}

function StandardActionCard({
  action,
  index,
  respondingActionID,
  onInteractionAction,
  onSelectArtifact
}: ActionCardProps) {
  const source = textValue(action._ui_source, "");
  const status = textValue(action.status ?? action.risk ?? action.risk_level ?? action.kind, "pending");
  const title = actionTitle(action, index);
  const body = actionBody(action);
	const childActions = firstArray(action.actions)
		.map(asRecord)
		.filter((item) => Object.keys(item).length > 0);
  const interactionID = textValue(action.id ?? action.interaction_id, "");
  const renderID = actionRenderID(action);
	const isInteractive = interactionID !== "" && childActions.length > 0;
  const isConfirmation = isConfirmationAction(action);
  const fields = interactionFields(action);
  const reviewItems = isConfirmation
    ? mergeReviewItems(confirmationReviewItems(action), interactionReviewItems(action))
    : interactionReviewItems(action);
  const command = isConfirmation ? {} : asRecord(action.command);
	const pages = useMemo(
		() => buildInteractionPages({ reviewItems, fields, command }),
		[action, fields, reviewItems, command]
  );
  const [pageIndex, setPageIndex] = useState(0);
  const fieldSignature = fields.map((field) => `${textValue(field.id, "")}:${textValue(field.value, "")}`).join("|");
  const [fieldValues, setFieldValues] = useState<JsonRecord>(() => initialFieldValues(fields));

  useEffect(() => {
    setFieldValues(initialFieldValues(fields));
    setPageIndex(0);
  }, [interactionID, fieldSignature]);

  const tone = actionTone(action, isInteractive);
  const Icon = tone === "error" ? AlertTriangle : tone === "attention" ? AlertTriangle : tone === "success" ? CheckCircle2 : ChevronRight;
  const requiredMissing = hasMissingRequiredFields(fields, fieldValues);
  const currentPage = pages[Math.min(pageIndex, Math.max(0, pages.length - 1))] ?? emptyInteractionPage();
  const isLastPage = pageIndex >= pages.length - 1;
	const compact = isCompactActionCard(action, tone, isInteractive, false);

  return (
		<div className={`action-card ${tone} ${isInteractive ? "interactive" : ""} ${compact ? "compact" : ""}`}>
      <Icon size={16} />
      <div className="action-content">
        <div className="action-title-line">
          <strong>{title}</strong>
          <span>{actionBadge(action, source, status)}</span>
        </div>
        {body && !compact && <p className="action-body">{body}</p>}
        {pages.length > 1 && !compact && (
          <InteractionPager
            pageIndex={pageIndex}
            pageCount={pages.length}
            onPageChange={(nextPage) => setPageIndex(clampNumber(nextPage, 0, pages.length - 1))}
          />
        )}
        {!compact && (
          <ActionDetails
            reviewItems={currentPage.reviewItems}
            fields={currentPage.fields}
            command={currentPage.showCommand ? command : {}}
				fieldValues={fieldValues}
            onFieldChange={(fieldID, value) => setFieldValues((current) => ({ ...current, [fieldID]: value }))}
            onSelectArtifact={onSelectArtifact}
          />
        )}
        {isInteractive && isLastPage && (
          <div className="action-buttons">
            {childActions.map((child, childIndex) => {
              const actionID = textValue(child.id ?? child.action_id ?? child.decision, `action_${childIndex + 1}`);
              const label = textValue(child.label ?? child.title ?? actionID, actionID);
              const pendingID = interactionActionID(interactionID || renderID || "action", actionID);
              const isBusy = respondingActionID === pendingID;
              const style = textValue(child.style, "secondary");
              const disabled = respondingActionID !== "" || (requiredMissing && !isCancelAction(actionID));
              return (
                <button
                  key={`${actionID}-${childIndex}`}
                  className={`action-button ${style}`}
                  type="button"
                  disabled={disabled}
                  onClick={() => onInteractionAction(action, child, interactionPayloadForAction(action, child, fieldValues))}
                >
                  {isBusy && <Loader2 className="spin" size={14} />}
                  <span>{label}</span>
                </button>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

function actionRenderID(action: JsonRecord): string {
  return textValue(action.id ?? action.interaction_id ?? action.plan_id ?? action.title ?? action.type ?? action.kind, "");
}

function resolveInteractionInMessages(messages: ChatMessage[], interactionID: string, actionID: string, response: ChatResponse): ChatMessage[] {
  const id = interactionID.trim();
  if (!id) {
    return messages;
  }
  const resolvedStatus = interactionResolvedStatus(actionID, response);
  const resolvedAt = Date.now();
  let changed = false;
  const nextMessages = messages.map((message) => {
    if (!message.actions?.length) {
      return message;
    }
    let actionChanged = false;
    const actions = message.actions.map((action) => {
      if (textValue(action.id ?? action.interaction_id, "") !== id) {
        return action;
      }
      actionChanged = true;
      changed = true;
      return {
        ...action,
        status: resolvedStatus,
        stage: resolvedStatus,
        resolved_action_id: actionID,
        resolved_at: resolvedAt,
        actions: []
      };
    });
    return actionChanged ? { ...message, actions } : message;
  });
  return changed ? nextMessages : messages;
}

function removeInteractionActionFromMessages(messages: ChatMessage[], interactionID: string, renderID: string, interaction?: JsonRecord): ChatMessage[] {
  const ids = interactionDismissalSet([interactionID, renderID]);
  if (ids.size === 0) {
    return messages;
  }
  const dismissesConfirmation = interaction ? isConfirmationAction(interaction) : false;
  let changed = false;
  const nextMessages = messages.flatMap((message) => {
    if (dismissesConfirmation && isDisposableConfirmationPromptMessage(message)) {
      changed = true;
      return [];
    }
    if (!message.actions?.length) {
      return [message];
    }
    const removedActions: JsonRecord[] = [];
    const actions = message.actions.filter((action) => {
      const record = asRecord(action);
      const actionID = textValue(record.id ?? record.interaction_id, "");
      const actionRenderIDValue = actionRenderID(record);
      const removed = ids.has(actionID) || ids.has(actionRenderIDValue);
      if (removed) {
        removedActions.push(record);
      }
      return !removed;
    });
    if (actions.length === message.actions.length) {
      return [message];
    }
    changed = true;
    if (isDisposableInteractionCarrierMessage(message, removedActions, actions, dismissesConfirmation)) {
      return [];
    }
    return [{ ...message, actions }];
  });
  return changed ? nextMessages : messages;
}

function isDisposableInteractionCarrierMessage(
  message: ChatMessage,
  removedActions: JsonRecord[],
  remainingActions: JsonRecord[],
  dismissesConfirmation: boolean
): boolean {
  if (message.artifacts?.length || remainingActions.length > 0) {
    return false;
  }
  if (removedActions.length === 0) {
    return false;
  }
  if (removedActions.some((action) => isComposerInteraction(action) || isConfirmationAction(action))) {
    return true;
  }
  return dismissesConfirmation && isDisposableConfirmationPromptText(message.content);
}

function isDisposableConfirmationPromptMessage(message: ChatMessage): boolean {
  if (message.status === "error" || message.artifacts?.length) {
    return false;
  }
  if (!isDisposableConfirmationPromptText(message.content)) {
    return false;
  }
  const actions = (message.actions ?? []).map(asRecord);
  return actions.length === 0 || actions.some((action) => isApprovalPromptAction(action) || isConfirmationAction(action) || isComposerInteraction(action));
}

function isDisposableConfirmationPromptText(content: string): boolean {
  const text = content.trim();
  if (!text) {
    return false;
  }
  const compact = text.replace(/\s+/g, " ");
  const genericChineseConfirmation = compact.length <= 160 && (
    /^(?:这个|此|该)?操作.*(?:需要|等待).*确认.*(?:执行|继续)[。.!！]?$/u.test(compact) ||
    /^(?:确认|批准)后(?:才会|将会|即可)?.*(?:执行|继续)[。.!！]?$/u.test(compact) ||
    /^(?:需要|等待)你(?:的)?确认(?:后才会执行)?[。.!！]?$/u.test(compact)
  );
  return (
    text === "This action requires confirmation before execution." ||
    text === "This action will modify the project and requires confirmation." ||
    (text.toLowerCase().includes("confirm") && text.length <= 120) ||
    genericChineseConfirmation
  );
}

function isApprovalPromptAction(action: JsonRecord): boolean {
  if (isMixTreatmentPendingAction(action)) {
    return false;
  }
  const type = textValue(action.type, "").toLowerCase();
  const status = textValue(action.status ?? action.stage, "").toLowerCase();
  return type === "approval.requested" || status.includes("waiting") || status.includes("confirm") || truthy(action.requires_confirmation);
}

function chatMessageFromAgentEvent(event: AgentEvent, mode?: AgentMode | string): ChatMessage | null {
  const type = textValue(event.type, "");
  const status = textValue(event.status, "");
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

function agentEventSourceID(event: AgentEvent): string {
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

function agentEventAction(event: AgentEvent): JsonRecord {
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

function addInteractionDismissals(current: string[], ...ids: string[]): string[] {
  const next = interactionDismissalSet(current);
  ids.forEach((id) => {
    const cleanID = id.trim();
    if (cleanID) {
      next.add(cleanID);
    }
  });
  return Array.from(next).slice(-80);
}

function interactionDismissalSet(ids: string[]): Set<string> {
  const out = new Set<string>();
  ids.forEach((id) => {
    const cleanID = id.trim();
    if (cleanID) {
      out.add(cleanID);
    }
  });
  return out;
}

function interactionResolvedStatus(actionID: string, response: ChatResponse): string {
  const normalizedAction = actionID.trim().toLowerCase();
  const goalStatus = textValue(response.goal_status, "").toLowerCase();
  if (normalizedAction === "cancel" || goalStatus.includes("cancel")) {
    return "cancelled";
  }
  if (response.error || goalStatus.includes("fail") || goalStatus.includes("error")) {
    return "failed";
  }
  return "completed";
}

function latestComposerInteraction(messages: ChatMessage[], dismissedIDs: string[] = []): JsonRecord | null {
  const dismissed = interactionDismissalSet(dismissedIDs);
  for (let messageIndex = messages.length - 1; messageIndex >= 0; messageIndex -= 1) {
    const actions = messages[messageIndex].actions ?? [];
    for (let actionIndex = actions.length - 1; actionIndex >= 0; actionIndex -= 1) {
      const action = asRecord(actions[actionIndex]);
      const actionID = textValue(action.id ?? action.interaction_id, "");
      if (isComposerInteraction(action) && !dismissed.has(actionID) && !dismissed.has(actionRenderID(action))) {
        return action;
      }
    }
  }
  return null;
}

function isComposerInteraction(action: JsonRecord): boolean {
	if (textValue(action._ui_source, "") !== "interaction") {
    return false;
  }
  if (isCapabilityProposalInteraction(action)) {
    return false;
  }
  const id = textValue(action.id ?? action.interaction_id, "");
  const status = textValue(action.status ?? action.stage, "").toLowerCase();
  const kind = textValue(action.kind, "").toLowerCase();
  const type = textValue(action.type, "").toLowerCase();
  const childActions = firstArray(action.actions).map(asRecord).filter((item) => Object.keys(item).length > 0);
  if (!id || childActions.length === 0) {
    return false;
  }
  if (
    status.includes("complete") ||
    status.includes("done") ||
    status.includes("cancel") ||
    status.includes("fail") ||
    status.includes("error") ||
    kind === "mode_boundary" ||
    type === "mode_boundary"
  ) {
    return false;
  }
  return true;
}

function isCapabilityProposalInteraction(action: JsonRecord): boolean {
  const payload = interactionPayload(action);
  const presentation = asRecord(payload.proposal_presentation ?? action.proposal_presentation);
  const kind = textValue(action.kind, "").toLowerCase();
  const type = textValue(action.type, "").toLowerCase();
  return kind === "proposal_approval" || type === "proposal_approval" || textValue(presentation.schema_version, "") === "vit.proposal_presentation.v1";
}

function hasPendingComposerInteraction(messages: ChatMessage[]): boolean {
  return messages.some((message) => (message.actions ?? []).map(asRecord).some(isComposerInteraction));
}

function debugConfirmation(stage: string, detail: unknown): void {
  if (!confirmationDebugEnabled()) {
    return;
  }
  const entry = {
    stage,
    at: new Date().toISOString(),
    detail
  };
  try {
    console.info("[AskVit confirmation]", entry);
  } catch {
    // Diagnostics only.
  }
  try {
    const key = "ask_vit_confirmation_debug";
    const previous = JSON.parse(window.localStorage.getItem(key) || "[]");
    const rows = Array.isArray(previous) ? previous : [];
    rows.push(entry);
    window.localStorage.setItem(key, JSON.stringify(rows.slice(-80)));
  } catch {
    // Diagnostics only.
  }
  try {
    void fetch("/agent/debug/confirmation", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(entry)
    }).catch(() => undefined);
  } catch {
    // Diagnostics only.
  }
}

function confirmationDebugEnabled(): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  try {
    const params = new URLSearchParams(window.location.search);
    return params.get("debug_confirmation") === "1" || window.localStorage.getItem("ask_vit_confirmation_debug_enabled") === "true";
  } catch {
    return false;
  }
}

function shouldDebugAgentEvents(events: AgentEvent[]): boolean {
  return events.some((event) => {
    const type = textValue(event.type, "").toLowerCase();
    const status = textValue(event.status, "").toLowerCase();
    return (
      type === "approval.requested" ||
      type === "turn.completed" ||
      type === "turn.failed" ||
      type === "item.completed" ||
      status.includes("confirm") ||
      status.includes("waiting") ||
      status.includes("fail") ||
      status.includes("error")
    );
  });
}

function summarizeAgentEventForConfirmation(event: AgentEvent): JsonRecord {
  return {
    seq: event.seq,
    type: textValue(event.type, ""),
    status: textValue(event.status, ""),
    source_id: agentEventSourceID(event),
    conversation_id: textValue(event.conversation_id, ""),
    goal_id: textValue(event.goal_id, ""),
    run_id: textValue(event.run_id, ""),
    item_id: textValue(event.item_id, ""),
    item_type: textValue(event.item_type, ""),
    title: textValue(event.title, ""),
    body: textValue(event.body, "").slice(0, 180),
    action: summarizeActionForConfirmation(agentEventAction(event))
  };
}

function summarizeChatResponseForConfirmation(response: ChatResponse): JsonRecord {
  const responseRecord = asRecord(response);
  const interactions = firstArray(response.interaction_requests).map(asRecord);
  return {
    conversation_id: textValue(response.conversation_id, ""),
    needs_confirmation: Boolean(response.needs_confirmation),
    plan_id: textValue(response.plan_id, ""),
    next_plan_id: textValue(responseRecord.next_plan_id, ""),
    goal_status: textValue(response.goal_status, ""),
    status: textValue(response.status, ""),
    reply: textValue(response.reply ?? response.message, "").slice(0, 180),
    preview_present: textValue(response.preview, "").length > 0,
    artifact_count: firstArray(response.artifacts).length,
    artifacts: artifactSummariesFrom(response.artifacts).map((artifact) => ({
      id: artifact.id,
      title: textValue(artifact.title, ""),
      kind: textValue(artifact.kind, ""),
      mime: textValue(artifact.mime, "")
    })),
    interaction_count: interactions.length,
    interactions: interactions.map(summarizeActionForConfirmation),
    command_count: firstArray(response.commands).length,
    executed_count: firstArray(response.executed_kernel_reply).length,
    project_result_card_count: firstArray(response.project_result_cards).length
  };
}

function summarizeMessageForConfirmation(message: ChatMessage): JsonRecord {
  return {
    id: textValue(message.id, ""),
    source_id: textValue(message.source_id, ""),
    role: message.role,
    content: textValue(message.content, "").slice(0, 180),
    status: textValue(message.status, ""),
    artifact_count: message.artifacts?.length ?? 0,
    artifacts: (message.artifacts ?? []).map((artifact) => ({
      id: artifact.id,
      title: textValue(artifact.title, ""),
      kind: textValue(artifact.kind, ""),
      mime: textValue(artifact.mime, "")
    })),
    disposable_confirmation_prompt: isDisposableConfirmationPromptText(textValue(message.content, "")),
    action_count: message.actions?.length ?? 0,
    composer_actions: (message.actions ?? []).map(asRecord).filter(isComposerInteraction).map(summarizeActionForConfirmation),
    actions: (message.actions ?? []).map(asRecord).map(summarizeActionForConfirmation)
  };
}

function summarizeMessagesForConfirmation(messages: ChatMessage[]): JsonRecord[] {
  return messages.map(summarizeMessageForConfirmation).filter((message) => {
    const content = textValue(message.content, "");
    return (
      Number(message.action_count ?? 0) > 0 ||
      Number(message.artifact_count ?? 0) > 0 ||
      content.toLowerCase().includes("confirm") ||
      content.includes("确认") ||
      Boolean(message.disposable_confirmation_prompt)
    );
  });
}

function summarizeActionForConfirmation(action: unknown): JsonRecord | null {
  const record = asRecord(action);
  if (Object.keys(record).length === 0) {
    return null;
  }
  return {
    id: textValue(record.id ?? record.interaction_id, ""),
    render_id: actionRenderID(record),
    ui_source: textValue(record._ui_source, ""),
    synthetic: truthy(record._synthetic_confirmation),
    is_confirmation: isConfirmationAction(record),
    is_composer: isComposerInteraction(record),
    kind: textValue(record.kind, ""),
    type: textValue(record.type, ""),
    status: textValue(record.status ?? record.stage, ""),
    plan_id: textValue(record.plan_id ?? asRecord(record.payload).plan_id ?? asRecord(record.data).plan_id, ""),
    child_action_count: firstArray(record.actions).length,
    child_actions: firstArray(record.actions).map(asRecord).map((child) => ({
      id: textValue(child.id ?? child.action_id, ""),
      label: textValue(child.label ?? child.title, ""),
      style: textValue(child.style, "")
    }))
  };
}

type InteractionPage = {
  reviewItems: JsonRecord[];
  fields: JsonRecord[];
  showCommand: boolean;
};

function emptyInteractionPage(): InteractionPage {
  return { reviewItems: [], fields: [], showCommand: false };
}

function buildInteractionPages({
  reviewItems,
  fields,
  command
}: {
  reviewItems: JsonRecord[];
  fields: JsonRecord[];
  command: JsonRecord;
}): InteractionPage[] {
  const hasCommand = Object.keys(command).length > 0;
  const commandWeight = hasCommand ? 2 : 0;
  const totalWeight =
    reviewItems.length +
    commandWeight +
    fields.reduce((sum, field) => sum + interactionFieldWeight(field), 0);
  if (totalWeight <= 6) {
    return [{ reviewItems, fields, showCommand: hasCommand }];
  }

  const pages: InteractionPage[] = [];
  let current = emptyInteractionPage();
  let weight = 0;
  const flush = () => {
    if (current.reviewItems.length > 0 || current.fields.length > 0 || current.showCommand) {
      pages.push(current);
      current = emptyInteractionPage();
      weight = 0;
    }
  };
  const reserve = (nextWeight: number) => {
    if (weight > 0 && weight + nextWeight > 6) {
      flush();
    }
    weight += nextWeight;
  };

  reviewItems.forEach((item) => {
    reserve(1);
    current.reviewItems.push(item);
  });
  fields.forEach((field) => {
    const fieldWeight = interactionFieldWeight(field);
    reserve(fieldWeight);
    current.fields.push(field);
  });
  if (hasCommand) {
    reserve(commandWeight);
    current.showCommand = true;
  }
  flush();
  return pages.length > 0 ? pages : [emptyInteractionPage()];
}

function interactionFieldWeight(field: JsonRecord): number {
  const kind = textValue(field.kind ?? field.type, "text").toLowerCase();
  const optionCount = firstArray(field.options).length;
  if (kind === "choice" || kind === "select") {
    return Math.max(2, Math.min(5, 1 + Math.ceil(optionCount / 2)));
  }
  if (kind === "textarea" || kind === "long_text" || kind === "multiline") {
    return 2;
  }
  return 1.25;
}

function InteractionPager({
  pageIndex,
  pageCount,
  onPageChange
}: {
  pageIndex: number;
  pageCount: number;
  onPageChange: (pageIndex: number) => void;
}) {
  const safePage = clampNumber(pageIndex, 0, Math.max(0, pageCount - 1));
  return (
    <div className="interaction-pager">
      <button type="button" disabled={safePage <= 0} onClick={() => onPageChange(safePage - 1)}>
        上一页      </button>
      <span>{safePage + 1} / {pageCount}</span>
      <button type="button" disabled={safePage >= pageCount - 1} onClick={() => onPageChange(safePage + 1)}>
        下一页      </button>
    </div>
  );
}

function messageRoleLabel(role: ChatMessage["role"]): string {
  if (role === "user") {
    return "用户";
  }
  if (role === "system") {
    return "系统";
  }
  return "Vit";
}

function agentModeLabel(mode?: AgentMode | string): string {
  const value = textValue(mode, "").toLowerCase();
  if (value === "default") {
    return "";
  }
  if (value === "plan") {
    return "计划";
  }
  if (value === "goal") {
    return "目标";
  }
  return localizeDisplayText(value);
}

function statusLabel(value: unknown, fallback = ""): string {
  const status = textValue(value, "").toLowerCase();
  if (!status) {
    return fallback;
  }
  if (status === "none") {
    return "无";
  }
  if (["idle", "empty"].includes(status)) {
    return "空闲";
  }
  if (status === "ready") {
    return "就绪";
  }
  if (status === "offline") {
    return "未连接";
  }
  if (["play", "playing"].includes(status)) {
    return "播放中";
  }
  if (["stop", "stopped"].includes(status)) {
    return "停止";
  }
  if (["pause", "paused"].includes(status)) {
    return "暂停";
  }
  if (["running", "in_progress", "processing", "busy"].includes(status)) {
    return "执行中";
  }
  if (["pending", "queued"].includes(status)) {
    return "待处理";
  }
  if (["completed", "complete", "done", "ok", "success", "succeeded", "applied"].includes(status)) {
    return "已完成";
  }
  if (status.includes("waiting") || status.includes("confirm") || status === "needs_confirmation" || status === "requires_confirmation") {
    return "待确认";
  }
  if (["cancel", "cancelled", "canceled"].includes(status)) {
    return "已取消";
  }
  if (["failed", "failure", "error", "kernel_error"].includes(status)) {
    return "失败";
  }
  return fallback;
}

function riskLabel(value: unknown): string {
  const risk = textValue(value, "").toLowerCase();
  if (!risk) {
    return "";
  }
  if (risk.includes("confirm") || risk.includes("approval")) {
    return "需确认";
  }
  if (risk.includes("high") || risk.includes("danger")) {
    return "高风险";
  }
  if (risk.includes("medium")) {
    return "中风险";
  }
  if (risk.includes("low") || risk.includes("safe")) {
    return "低风险";
  }
  return statusLabel(risk, "");
}

function agentEventDisplayTitle(event: AgentEvent): string {
  const action = agentEventAction(event);
  const title = actionOperationTitle(action);
  if (title) {
    return title;
  }
  return localizeDisplayText(textValue(event.title, ""));
}

function actionOperationTitle(action: JsonRecord): string {
  const label = actionOperationLabel(action);
  if (!label) {
    return "";
  }
  const objectName = actionObjectName(action);
  return objectName ? `${label}: ${objectName}` : label;
}

function actionOperationLabel(action: JsonRecord): string {
  const names = actionNameCandidates(action);
  for (const name of names) {
    const label = operationLabelFromName(name);
    if (label) {
      return label;
    }
  }
  const kind = textValue(action.kind ?? action.type ?? action.item_type, "").toLowerCase();
  if (kind === "daw_action" || kind === "tool_call") {
    return "工程操作";
  }
  return "";
}

function actionNameCandidates(action: JsonRecord): string[] {
  const payload = asRecord(action.payload);
  const result = asRecord(action.result ?? payload.result);
  const command = asRecord(action.command);
  const args = asRecord(action.args ?? payload.args ?? command.args ?? command.arguments);
  const commandRaw = asRecord(payload.command_raw);
  const records = [action, payload, result, command, args, commandRaw];
  const values: string[] = [];
  records.forEach((record) => {
    [
      record.tool,
      record.command_name,
      record.name,
      record.cmd,
      record.command,
      record.action,
      record.kind,
      record.type
    ].forEach((value) => {
      const text = textValue(value, "");
      if (text) {
        values.push(text);
      }
    });
  });
  firstArray(action.commands, payload.commands, command.commands).map(asRecord).forEach((record) => {
    actionNameCandidates(record).forEach((name) => values.push(name));
  });
  return Array.from(new Set(values));
}

function operationLabelFromName(rawName: string): string {
  const name = rawName.trim().toLowerCase();
  if (!name) {
    return "";
  }
  if (name.includes("track.add") || name.includes("create_track") || name.includes("add_track") || name === "track") {
    return "新建轨道";
  }
  if (name.includes("track.delete") || name.includes("delete_track") || name.includes("remove_track")) {
    return "删除轨道";
  }
  if (name.includes("track.rename") || name.includes("rename_track")) {
    return "重命名轨道";
  }
  if (name.includes("track.select") || name.includes("select_track")) {
    return "选择轨道";
  }
  if (name.includes("midi.create_clip") || name.includes("midi.insert_clip") || name.includes("create_midi_clip") || name.includes("insert_midi_clip")) {
    return "新建 MIDI 片段";
  }
  if (name.includes("midi.apply_note_patch") || name.includes("apply_midi_note_patch") || name.includes("add_midi_notes") || name.includes("mutate_midi_notes")) {
    return "写入 MIDI 音符";
  }
  if (name.includes("midi.read_clip_notes") || name.includes("read_midi") || name.includes("read_clip_notes")) {
    return "读取 MIDI 音符";
  }
  if (name.includes("midi.import") || name.includes("import_midi")) {
    return "导入 MIDI";
  }
  if (name.includes("warm_waveform_bake") || name.includes("mix.request_observation") || name.includes("mix_request_observation")) {
    return "音频观察";
  }
  if (name.includes("mix.apply_tick") || name.includes("mix_apply_tick")) {
    return "混音步骤已执行";
  }
  if (name.includes("mix.propose_tick") || name.includes("mix_propose_tick")) {
    return "混音步骤建议";
  }
  if (name.includes("mix.rollback_tick") || name.includes("mix_rollback_tick")) {
    return "混音步骤回滚";
  }
  if (name.includes("plugin.load") || name.includes("load_plugin") || name.includes("rack_add_node") || name.includes("rack.add") || name.includes("rack_node")) {
    return "加载插件";
  }
  if (name.includes("plugin") && name.includes("learn")) {
    return "插件学习";
  }
  if (name.includes("clip.import") || name.includes("import_media")) {
    return "导入媒体";
  }
  if (name.includes("artifact") || name.includes("media")) {
    return "资料操作";
  }
  if (name.includes("browser")) {
    return "浏览器操作";
  }
  if (name.includes("macro")) {
    return "宏操作";
  }
  return "";
}

function actionObjectName(action: JsonRecord): string {
  const payload = asRecord(action.payload);
  const result = asRecord(action.result ?? payload.result);
  const command = asRecord(action.command);
  const args = asRecord(action.args ?? payload.args ?? command.args ?? command.arguments);
  const records = [action, payload, result, args, command];
  const specificNameKeys = ["track_name", "clip_name", "plugin_name", "display_name"];
  for (const record of records) {
    for (const key of specificNameKeys) {
      const value = textValue(record[key], "");
      if (isSafeObjectDisplayName(value)) {
        return value;
      }
    }
  }
  const nestedRecords = [
    asRecord(result.track),
    asRecord(result.clip),
    asRecord(result.plugin),
    asRecord(payload.track),
    asRecord(payload.clip),
    asRecord(payload.plugin),
    asRecord(args.track),
    asRecord(args.clip),
    asRecord(args.plugin)
  ];
  const nestedNameKeys = ["track_name", "clip_name", "plugin_name", "display_name", "title", "name", "label"];
  for (const record of nestedRecords) {
    for (const key of nestedNameKeys) {
      const value = textValue(record[key], "");
      if (isSafeObjectDisplayName(value)) {
        return value;
      }
    }
  }
  for (const record of records) {
    for (const key of ["name", "label"]) {
      const value = textValue(record[key], "");
      if (isSafeObjectDisplayName(value)) {
        return value;
      }
    }
  }
  const idKeys = ["new_track_id", "created_track_id", "track_id", "new_clip_id", "created_clip_id", "clip_id", "plugin_id"];
  for (const record of records) {
    for (const key of idKeys) {
      const value = textValue(record[key], "");
      if (isSafeObjectDisplayName(value)) {
        return value;
      }
    }
  }
  return "";
}

function isSafeObjectDisplayName(value: string): boolean {
  const text = value.trim();
  if (!text) {
    return false;
  }
  if (statusLabel(text, "") || operationLabelFromName(text) || looksLikeInternalInstruction(text)) {
    return false;
  }
  if (/^[a-z]+[._][a-z0-9_.-]+$/i.test(text) || /^[a-z0-9]+_[a-z0-9_]+$/i.test(text)) {
    return false;
  }
  if (/^(已完成|正在执行)\s+/i.test(text)) {
    return false;
  }
  return true;
}

function commandSummaryLabel(command: JsonRecord): string {
  return actionOperationTitle(command) || "工程操作";
}

function localizeDisplayText(value: string): string {
  const text = value.trim();
  if (!text) {
    return "";
  }
  const status = statusLabel(text, "");
  if (status) {
    return status;
  }
  const directOperation = operationLabelFromName(text);
  if (directOperation) {
    return directOperation;
  }
  const completedMatch = text.match(/^已完成\s+(.+)$/i);
  if (completedMatch) {
    const label = operationLabelFromName(completedMatch[1]);
    return label ? `已完成：${label}` : "工程操作已完成";
  }
  const runningMatch = text.match(/^正在执行\s+(.+)$/i);
  if (runningMatch) {
    const label = operationLabelFromName(runningMatch[1]);
    return label ? `正在执行：${label}` : "正在执行工程操作";
  }
  if (/^track added$/i.test(text)) {
    return "轨道已创建";
  }
  if (/^clip added$/i.test(text) || /^midi clip added$/i.test(text)) {
    return "MIDI 片段已创建";
  }
  const insertedMatch = text.match(/^updated midi clip:\s*inserted\s*(\d+)/i);
  if (insertedMatch) {
    return `MIDI 片段已更新：写入 ${insertedMatch[1]} 个音符。`;
  }
  if (looksLikeInternalInstruction(text)) {
    return "";
  }
  return text;
}

function localizeMixTreatmentCardText(value: string): string {
  const raw = value.trim();
  if (looksLikeEnglishMixTreatmentReason(raw)) {
    return "已根据当前频段能量观察生成一个保守的低频/低中频 EQ 处理候选；确认前不会写入任何插件参数。";
  }
  let text = localizeDisplayText(raw);
  if (!text) {
    return "";
  }
  const exact = text.toLowerCase();
  const exactMap: Record<string, string> = {
    pending: "待确认",
    pending_confirmation: "待确认",
    waiting_for_user: "等待确认",
    request_more_observation: "需要更多观察",
    low: "低",
    medium: "中",
    high: "高",
    plugin_treatment: "插件处理",
    gain_balance: "电平平衡",
    pan_balance: "声像调整",
    eq: "EQ",
    utility: "工具"
  };
  if (exactMap[exact]) {
    return exactMap[exact];
  }
  text = text
    .replace(/\brequest_more_observation\b/gi, "需要更多观察")
    .replace(/\bpending_confirmation\b/gi, "待确认")
    .replace(/\bwaiting_for_user\b/gi, "等待确认")
    .replace(/\bconfidence\b/gi, "置信度")
    .replace(/\bstrategy\b/gi, "策略")
    .replace(/\blow[-_\s]?mid\b/gi, "低中频")
    .replace(/\bsub\b/gi, "超低频")
    .replace(/\bbass\b/gi, "低频")
    .replace(/\bmid\b/gi, "中频")
    .replace(/\bpresence\b/gi, "存在感频段")
    .replace(/\bair\b/gi, "空气感频段")
    .replace(/\bpeak\s*\/\s*rms\b/gi, "峰值/RMS")
    .replace(/\blow\b/gi, "低")
    .replace(/\bmedium\b/gi, "中")
    .replace(/\bhigh\b/gi, "高");
  return text;
}

function looksLikeEnglishMixTreatmentReason(text: string): boolean {
  const compact = text.trim();
  if (!compact) {
    return false;
  }
  const lower = compact.toLowerCase();
  if (!/(sub|bass|low[-\s]?mid|band energ|presence|peak\/rms|plugin|parameter)/.test(lower)) {
    return false;
  }
  const asciiLetters = (compact.match(/[a-z]/gi) ?? []).length;
  const cjk = (compact.match(/[\u4e00-\u9fff]/g) ?? []).length;
  return asciiLetters >= 20 && cjk === 0;
}

function isCompactActionCard(
  action: JsonRecord,
  tone: "info" | "attention" | "success" | "error",
  isInteractive: boolean,
  isCompletion: boolean
): boolean {
  if (isInteractive || isCompletion || tone !== "success") {
    return false;
  }
  const source = textValue(action._ui_source, "");
  if (source !== "event" && source !== "executed") {
    return false;
  }
  const status = statusLabel(action.status ?? action.stage ?? asRecord(action.payload).status, "");
  return status === "已完成";
}

function actionTitle(action: JsonRecord, index: number): string {
  if (isConfirmationAction(action)) {
    return "需要确认";
  }
  const operationTitle = actionOperationTitle(action);
  if (operationTitle) {
    return operationTitle;
  }
  const command = asRecord(action.command);
  const rawTitle = textValue(
    action.title ?? action.command_name ?? action.name ?? action.tool ?? command.tool ?? command.cmd ?? action.kind,
    ""
  );
  return localizeDisplayText(rawTitle) || `Project action ${index + 1}`;
}

function actionBody(action: JsonRecord): string {
  if (isConfirmationAction(action)) {
    return confirmationActionSummary(action);
  }
  const payload = asRecord(action.payload);
  const rawBody = textValue(action.body ?? action.preview ?? payload.preview ?? action.message ?? action.reason ?? action.description, "");
  const body = localizeDisplayText(rawBody);
  if (!body || looksLikeInternalInstruction(body)) {
    return "";
  }
  const objectName = actionObjectName(action);
  if (objectName && body === objectName) {
    return "";
  }
  return body;
}

function actionBadge(action: JsonRecord, source: string, status: string): string {
  if (isConfirmationAction(action)) {
    const resolvedStatus = textValue(action.status ?? action.stage, "").toLowerCase();
    if (resolvedStatus.includes("complete") || resolvedStatus.includes("done")) {
      return "已确认";
    }
    if (resolvedStatus.includes("cancel")) {
      return "已取消";
    }
    if (resolvedStatus.includes("fail") || resolvedStatus.includes("error")) {
      return "失败";
    }
    return "待确认";
  }
  if (source === "interaction") {
    const label = statusLabel(action.status ?? action.stage ?? status, "");
    return label || "Interaction";
  }
  if (source === "command") {
    return riskLabel(action.risk ?? action.risk_level ?? status) || "待确认";
  }
  if (source === "executed") {
    return statusLabel(action.status ?? status, "Executed");
  }
  if (source === "event") {
    return statusLabel(status, "Status");
  }
  return statusLabel(status, "");
}

function actionTone(action: JsonRecord, isInteractive: boolean): "info" | "attention" | "success" | "error" {
  const source = textValue(action._ui_source, "");
  const status = textValue(action.status ?? action.goal_status ?? "", "").toLowerCase();
  const risk = textValue(action.risk ?? action.risk_level ?? "", "").toLowerCase();
  const stopReason = textValue(action.stop_reason ?? "", "").toLowerCase();
  const error = textValue(action.error ?? action.failure_reason, "");
  if (error || status.includes("error") || status.includes("fail") || stopReason === "failed") {
    return "error";
  }
  if (status === "completed" || status === "complete" || status === "done") {
    return "success";
  }
  if (isInteractive || risk === "confirm" || status.includes("waiting") || status.includes("confirm")) {
    return "attention";
  }
  if (source === "executed" || status === "ok" || status === "done" || status === "success" || status === "applied") {
    return "success";
  }
  return "info";
}

function isConfirmationAction(action: JsonRecord): boolean {
  if (isMixTreatmentPendingAction(action)) {
    return false;
  }
  const kind = textValue(action.kind, "").toLowerCase();
  const type = textValue(action.type, "").toLowerCase();
  const status = textValue(action.status ?? action.stage, "").toLowerCase();
  const payload = asRecord(action.payload);
  return (
    kind === "confirmation" ||
    type === "confirmation" ||
    type === "approval.requested" ||
    truthy(action.requires_confirmation) ||
    truthy(action.needs_confirmation) ||
    status.includes("waiting_confirmation") ||
    status.includes("needs_confirmation") ||
    textValue(action.plan_id ?? payload.plan_id, "") !== ""
  );
}

function isPendingInteractionAction(action: JsonRecord): boolean {
  return isConfirmationAction(action) || isMixTreatmentPendingAction(action) || isComposerInteraction(action);
}

function isMixTreatmentPendingAction(action: JsonRecord): boolean {
  const payload = interactionPayload(action);
  const kind = textValue(action.kind, "").toLowerCase();
  const type = textValue(action.type, "").toLowerCase();
  const workflow = textValue(action.workflow ?? payload.workflow, "").toLowerCase();
  return kind === "mix_treatment_confirmation" || type === "mix_treatment_confirmation" || workflow === "mix_treatment";
}

function mixTreatmentPendingLabel(actionKind: string, processor: string): string {
  const kind = actionKind.trim().toLowerCase();
  const proc = processor.trim().toLowerCase();
  if (kind === "gain_balance") {
    return "电平平衡";
  }
  if (kind === "pan_balance") {
    return "声像调整";
  }
  if (kind === "plugin_treatment") {
    return proc ? `${localizeDisplayText(proc)} 处理` : "插件处理";
  }
  return localizeDisplayText(actionKind || processor || "混音处理");
}

function mixTreatmentPendingValueLabel(actionKind: string, deltaDB: string, deltaPan: string, targetPan: string): string {
  const kind = actionKind.trim().toLowerCase();
  if (kind === "gain_balance" && deltaDB) {
    return `${formatSignedNumber(deltaDB)} dB`;
  }
  if (kind === "pan_balance") {
    if (targetPan) {
      return `目标 ${formatPanValue(targetPan)}`;
    }
    if (deltaPan) {
      return `变化 ${formatSignedNumber(deltaPan)}`;
    }
  }
  return "等待确认后执行";
}

function formatSignedNumber(value: string): string {
  const number = Number(value);
  if (!Number.isFinite(number)) {
    return value;
  }
  return number > 0 ? `+${number}` : String(number);
}

function formatPanValue(value: string): string {
  const number = Number(value);
  if (!Number.isFinite(number)) {
    return value;
  }
  if (Math.abs(number) < 0.0001) {
    return "居中";
  }
  return number < 0 ? `左 ${Math.abs(number)}` : `右 ${number}`;
}

function confirmationActionSummary(action: JsonRecord): string {
  const commands = confirmationCommands(action);
  const preview = confirmationPreviewText(action);
  const directBody = textValue(action.body ?? action.message ?? action.description, "");
  const commandSummary = summarizeConfirmationCommands(commands, preview);
  if (commandSummary) {
    return commandSummary;
  }
  if (directBody && !looksLikeInternalInstruction(directBody)) {
    return directBody;
  }
  if (preview && !looksLikeInternalInstruction(preview)) {
    return preview;
  }
  return "这个操作会修改工程，需要你确认后执行中";
}

function confirmationCommands(action: JsonRecord): JsonRecord[] {
  const payload = asRecord(action.payload);
  const data = asRecord(action.data);
  const command = asRecord(action.command);
  const out: JsonRecord[] = [];
  const seen = new Set<string>();
  const pushCommand = (value: unknown) => {
    const record = asRecord(value);
    if (Object.keys(record).length === 0) {
      return;
    }
    if (!confirmationRecordHasCommandContent(record)) {
      return;
    }
    const key = compactJSON(record);
    if (seen.has(key)) {
      return;
    }
    seen.add(key);
    out.push(record);
  };
  const pushCommands = (value: unknown) => firstArray(value).forEach(pushCommand);
  pushCommands(action.commands);
  pushCommands(payload.commands);
  pushCommands(data.commands);
  pushCommands(command.commands);
  [action.command, payload.command, data.command, action.value, payload.value, data.value].forEach(pushCommand);
  firstArray(action.review_items).map(asRecord).forEach((item) => {
    const itemPayload = asRecord(item.payload);
    pushCommands(item.commands);
    pushCommands(itemPayload.commands);
    [item.command, itemPayload.command, item.value, itemPayload.value].forEach(pushCommand);
  });
  return out;
}

function confirmationRecordHasCommandContent(record: JsonRecord): boolean {
  if (textValue(record.name ?? record.command_name ?? record.tool ?? record.cmd ?? record.action ?? record.command, "") !== "") {
    return true;
  }
  if (Object.keys(asRecord(record.command)).length > 0 || Object.keys(asRecord(record.args ?? record.arguments)).length > 0) {
    return true;
  }
  return firstArray(record.operations, record.notes, record.midi_notes, record.note_rows).length > 0;
}

function confirmationPreviewText(action: JsonRecord): string {
  const payload = asRecord(action.payload);
  const command = asRecord(action.command);
  return textValue(action.preview ?? payload.preview ?? command.preview, "");
}

function confirmationReviewItems(action: JsonRecord): JsonRecord[] {
  const commands = confirmationCommands(action);
  const preview = confirmationPreviewText(action);
  const rows: JsonRecord[] = [];
  commands.forEach((command, index) => {
    rows.push(...confirmationCommandReviewItems(command, index, preview));
  });
  if (rows.length === 0) {
    if (preview && !looksLikeInternalInstruction(preview)) {
      rows.push({ id: "confirmation-preview", title: "确认内容", body: preview });
    }
  }
  return rows;
}

function mergeReviewItems(primary: JsonRecord[], secondary: JsonRecord[]): JsonRecord[] {
  const seen = new Set<string>();
  const out: JsonRecord[] = [];
  [...primary, ...secondary].forEach((item, index) => {
    const key = textValue(item.id ?? item.title, `review-${index}`);
    if (seen.has(key)) {
      return;
    }
    seen.add(key);
    out.push(item);
  });
  return out;
}

function confirmationCommandReviewItems(command: JsonRecord, index: number, preview = ""): JsonRecord[] {
  const payload = confirmationCommandPayload(command);
  const name = confirmationCommandName(command, payload);
  const rows: JsonRecord[] = [];
  if (name.includes("apply_note_patch") || name.includes("add_midi_notes") || name.includes("midi_note")) {
    const notes = confirmationNoteRows(payload);
    const visibleNotes = notes.length > 0 ? notes : confirmationPreviewNoteRows(preview);
    rows.push({ id: `confirm-${index}-task`, title: "Task", body: visibleNotes.length > 0 ? `Write ${visibleNotes.length} MIDI notes` : "Write MIDI notes" });
    rows.push(...confirmationTargetRows(payload, index));
    rows.push({ id: `confirm-${index}-unit`, title: "Time unit", body: textValue(payload.time_unit, "beats") });
    visibleNotes.slice(0, 32).forEach((note, noteIndex) => {
      rows.push({
        id: `confirm-${index}-note-${noteIndex}`,
        title: `Note ${noteIndex + 1}`,
        body: confirmationNoteLabel(note)
      });
    });
    return rows;
  }
  if (name.includes("create_midi_clip") || name.includes("insert_midi_clip")) {
    rows.push({ id: `confirm-${index}-task`, title: "Task", body: "Create MIDI clip" });
    rows.push(...confirmationTargetRows(payload, index));
    rows.push({ id: `confirm-${index}-start`, title: "Start", body: textValue(payload.start_seconds ?? payload.start ?? payload.start_beats, "0") });
    rows.push({ id: `confirm-${index}-length`, title: "Length", body: textValue(payload.length_seconds ?? payload.length ?? payload.duration, "4") });
    return rows;
  }
  rows.push({ id: `confirm-${index}-task`, title: "Task", body: "Run project operation" });
  rows.push(...confirmationTargetRows(payload, index));
  return rows;
}

function confirmationCommandPayload(command: JsonRecord): JsonRecord {
  const records: JsonRecord[] = [];
  const visit = (value: unknown, depth = 0) => {
    const record = asRecord(value);
    if (Object.keys(record).length === 0 || depth > 4) {
      return;
    }
    records.push(record);
    visit(record.command, depth + 1);
    visit(record.payload, depth + 1);
    visit(record.value, depth + 1);
    visit(record.args, depth + 1);
    visit(record.arguments, depth + 1);
    visit(record.params, depth + 1);
    visit(record.parameters, depth + 1);
  };
  visit(command);
  return mergeJsonRecords(...records);
}

function confirmationCommandName(command: JsonRecord, payload: JsonRecord): string {
  return textValue(command.name ?? command.command_name ?? payload.name ?? payload.command_name ?? command.tool ?? command.cmd ?? payload.tool ?? payload.cmd ?? payload.command ?? payload.action, "").toLowerCase();
}

function mergeJsonRecords(...records: JsonRecord[]): JsonRecord {
  const out: JsonRecord = {};
  records.forEach((record) => {
    Object.entries(record).forEach(([key, value]) => {
      if (value !== undefined && value !== null) {
        out[key] = value;
      }
    });
  });
  return out;
}

function confirmationTargetRows(payload: JsonRecord, index: number): JsonRecord[] {
  const rows: JsonRecord[] = [];
  const trackID = textValue(payload.track_id ?? payload.target_track_id, "");
  const clipID = textValue(payload.clip_id ?? payload.target_clip_id, "");
  if (trackID) {
    rows.push({ id: `confirm-${index}-track`, title: "目标轨道", body: trackID });
  }
  if (clipID) {
    rows.push({ id: `confirm-${index}-clip`, title: "目标片段", body: clipID });
  }
  return rows;
}

function confirmationNoteRows(payload: JsonRecord): JsonRecord[] {
  const patch = asRecord(payload.patch ?? payload.note_patch ?? payload.midi_patch);
  const operations = firstArray(payload.operations, payload.note_operations, payload.midi_operations, patch.operations).map(asRecord);
  const rows: JsonRecord[] = [];
  operations.forEach((operation) => {
    const op = textValue(operation.op ?? operation.action ?? operation.type ?? operation.kind, "").toLowerCase();
    const nestedNotes = firstArray(operation.notes, operation.midi_notes, operation.note_rows, operation.note_events, operation.events, operation.insert_notes)
      .map(asRecord)
      .filter(confirmationLooksLikeNote);
    if (nestedNotes.length > 0) {
      nestedNotes.forEach((note) => rows.push({ ...note, operation: op || textValue(operation.operation, "") }));
      return;
    }
    if (op === "insert_note" || op === "add_note" || op === "create_note" || op === "note" || (!op && confirmationLooksLikeNote(operation))) {
      rows.push(operation);
    }
  });
  if (rows.length > 0) {
    return rows;
  }
  return firstArray(payload.notes, payload.midi_notes, payload.note_rows, payload.note_events, payload.events, patch.notes)
    .map(asRecord)
    .filter(confirmationLooksLikeNote);
}

function confirmationLooksLikeNote(note: JsonRecord): boolean {
  return textValue(note.pitch ?? note.note ?? note.name ?? note.pitch_name ?? note.midi_note, "") !== "";
}

function confirmationPreviewNoteRows(preview: string): JsonRecord[] {
  const rows: JsonRecord[] = [];
  const linePattern = /^\s*\d+\.\s*(?:insert_note|add_note|create_note|note)\b([^\n]*)$/gim;
  let lineMatch: RegExpExecArray | null;
  while ((lineMatch = linePattern.exec(preview)) !== null) {
    const row: JsonRecord = {};
    const fieldPattern = /\b(pitch|note|start|length|duration|velocity)=([^\s,;]+)/gi;
    let fieldMatch: RegExpExecArray | null;
    while ((fieldMatch = fieldPattern.exec(lineMatch[1])) !== null) {
      row[fieldMatch[1].toLowerCase()] = fieldMatch[2];
    }
    if (confirmationLooksLikeNote(row)) {
      rows.push(row);
    }
  }
  return rows;
}

function confirmationNoteLabel(note: JsonRecord): string {
  const pitch = textValue(note.pitch ?? note.note ?? note.name ?? note.pitch_name, "-");
  const pitchName = midiPitchName(Number(pitch));
  const start = textValue(note.start ?? note.start_beat ?? note.start_beats ?? note.start_time_beats ?? note.position ?? note.beat, "0");
  const length = textValue(note.length ?? note.duration ?? note.length_beats ?? note.duration_beats ?? note.duration_beats_float, "1");
  const velocity = textValue(note.velocity, "100");
  return `Pitch ${pitch}${pitchName ? ` (${pitchName})` : ""}, start ${start}, length ${length}, velocity ${velocity}`;
}

function midiPitchName(pitch: number): string {
  if (!Number.isFinite(pitch)) {
    return "";
  }
  const names = ["C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"];
  const rounded = Math.round(pitch);
  const name = names[((rounded % 12) + 12) % 12];
  const octave = Math.floor(rounded / 12) - 1;
  return `${name}${octave}`;
}

function summarizeConfirmationCommands(commands: JsonRecord[], preview: string): string {
  const names = commands
    .map((command) => textValue(command.name ?? command.command_name ?? command.cmd ?? command.tool, "").toLowerCase())
    .filter(Boolean);
  const joined = `${names.join(" ")} ${preview}`.toLowerCase();
  if (joined.includes("apply_note_patch") || joined.includes("apply_midi_note_patch") || joined.includes("add_midi_notes") || joined.includes("insert_note")) {
    const noteCount = confirmationNoteCount(commands, preview);
    return noteCount > 0 ? `Ready to write ${noteCount} MIDI notes` : "Ready to write MIDI notes";
  }
  if (joined.includes("create_midi_clip") || joined.includes("midi clip")) {
    return "Ready to create MIDI clip";
  }
  if (joined.includes("clip.gain.set") || joined.includes("clip_gain_set")) {
    return "Ready to modify clip gain";
  }
  if (joined.includes("clip.fade.set") || joined.includes("clip_fade_set")) {
    return "Ready to modify clip fade";
  }
  if (joined.includes("create_track") || joined.includes("track")) {
    return "Ready to modify track structure";
  }
  if (commands.length > 1) {
    return `Ready to run ${commands.length} project operations after confirmation`;
  }
  if (commands.length === 1) {
    return "Ready to run 1 project operation after confirmation";
  }
  return "";
}

function confirmationNoteCount(commands: JsonRecord[], preview: string): number {
  let count = 0;
  commands.forEach((command) => {
    count += confirmationNoteRows(confirmationCommandPayload(command)).length;
  });
  if (count > 0) {
    return count;
  }
  count = confirmationPreviewNoteRows(preview).length;
  if (count > 0) {
    return count;
  }
  const match = preview.match(/(\d+)\s*(?:个)?\s*(?:notes?|音符)/i);
  return match ? Number(match[1]) : 0;
}

function looksLikeInternalInstruction(value: string): boolean {
  const text = value.trim();
  if (!text) {
    return false;
  }
  if (text.includes("{") && text.includes("}")) {
    return true;
  }
  return /\b(NEEDS_CONFIRMATION|tool|tool_call|command|cmd|workflow|plan_id|midi\.apply_note_patch|apply_midi_note_patch|add_midi_notes|create_midi_clip)\b/i.test(text);
}

function ActionDetails({
  reviewItems,
  fields,
  command,
	fieldValues,
  onFieldChange,
  onSelectArtifact
}: {
  reviewItems: JsonRecord[];
  fields: JsonRecord[];
  command: JsonRecord;
	fieldValues: JsonRecord;
  onFieldChange: (fieldID: string, value: string) => void;
  onSelectArtifact: (id: string) => void;
}) {
  const hasCommand = Object.keys(command).length > 0;
	if (reviewItems.length === 0 && fields.length === 0 && !hasCommand) {
    return null;
  }
  return (
    <div className="action-detail-grid">
      {reviewItems.map((item, index) => (
        <div className="action-detail" key={textValue(item.id, `review-${index}`)}>
          <span>{localizeDisplayText(textValue(item.title ?? item.status, `项目 ${index + 1}`))}</span>
          <strong>{localizeDisplayText(textValue(item.body ?? item.value, "-")) || "-"}</strong>
        </div>
      ))}
      {fields.length > 0 && (
        <div className="interaction-fields">
          {fields.slice(0, 32).map((field, index) => {
            const fieldID = textValue(field.id, `field-${index}`);
            const label = textValue(field.label ?? field.kind, `Field ${index + 1}`);
            const description = textValue(field.description, "");
            return (
              <label className="interaction-field" key={fieldID}>
                <span>{label}</span>
                <InteractionFieldControl field={field} value={textValue(fieldValues[fieldID], "")} onChange={(value) => onFieldChange(fieldID, value)} />
                {description && <small>{description}</small>}
              </label>
            );
          })}
        </div>
      )}
      {hasCommand && (
        <div className="action-detail command">
          <span>操作</span>
          <strong>{commandSummaryLabel(command)}</strong>
        </div>
      )}
    </div>
  );
}

function InteractionFieldControl({ field, value, onChange }: { field: JsonRecord; value: string; onChange: (value: string) => void }) {
  const kind = textValue(field.kind ?? field.type, "text").toLowerCase();
  const placeholder = textValue(field.placeholder, "");
  const options = firstArray(field.options).map(asRecord).filter((option) => Object.keys(option).length > 0);
  if (kind === "choice" || kind === "select") {
    return (
      <div className="choice-list">
        {options.length === 0 && <span className="choice-empty">{placeholder || "-"}</span>}
        {options.map((option, index) => {
          const optionValue = textValue(option.value ?? option.id ?? option.key ?? option.label, `option_${index + 1}`);
          const label = textValue(option.label ?? option.title ?? option.name ?? optionValue, optionValue);
          const description = textValue(option.description ?? option.hint, "");
          const selected = value === optionValue;
          return (
            <button
              key={`${optionValue}-${index}`}
              className={`choice-option ${selected ? "selected" : ""}`}
              type="button"
              onClick={() => onChange(optionValue)}
            >
              <span className="choice-index">{index + 1}.</span>
              <span className="choice-main">
                <strong>{label}</strong>
                {description && <small>{description}</small>}
              </span>
            </button>
          );
        })}
      </div>
    );
  }
  if (kind === "textarea" || kind === "long_text" || kind === "multiline") {
    return <textarea value={value} placeholder={placeholder} rows={3} onChange={(event) => onChange(event.currentTarget.value)} />;
  }
  return <input value={value} placeholder={placeholder} onChange={(event) => onChange(event.currentTarget.value)} />;
}





function interactionFields(action: JsonRecord): JsonRecord[] {
  return [...firstArray(action.questions), ...firstArray(action.fields)]
    .map(asRecord)
    .filter((field) => Object.keys(field).length > 0);
}

function interactionReviewItems(action: JsonRecord): JsonRecord[] {
  return firstArray(action.review_items).map(asRecord).filter((item) => Object.keys(item).length > 0);
}

function initialFieldValues(fields: JsonRecord[]): JsonRecord {
  const out: JsonRecord = {};
  fields.forEach((field, index) => {
    const fieldID = textValue(field.id, `field-${index}`);
    out[fieldID] = textValue(field.value, "");
  });
  return out;
}

function hasMissingRequiredFields(fields: JsonRecord[], values: JsonRecord): boolean {
  return fields.some((field, index) => {
    if (!truthy(field.required)) {
      return false;
    }
    const fieldID = textValue(field.id, `field-${index}`);
    return textValue(values[fieldID], "") === "";
  });
}

function isCancelAction(actionID: string): boolean {
  return ["cancel", "done", "close"].includes(actionID.trim().toLowerCase());
}

function interactionPayloadForAction(interaction: JsonRecord, action: JsonRecord, fieldValues: JsonRecord): JsonRecord {
  const payload: JsonRecord = { ...asRecord(action.value ?? action.payload) };
  const fields = interactionFields(interaction);
  if (fields.length === 0) {
    return payload;
  }
  const regularFields: JsonRecord = {};
  const displayDomainReviews: JsonRecord[] = [];
  fields.forEach((field, index) => {
    const fieldID = textValue(field.id, `field-${index}`);
    const value = textValue(fieldValues[fieldID], "");
    if (!value) {
      return;
    }
    if (truthy(field._plugin_display_domain_review)) {
      displayDomainReviews.push({
        ...asRecord(field.payload),
        display_domain_text: value,
        provenance_kind: "user_review",
        observation: "Confirmed from Ask Vit WebUI review card."
      });
      return;
    }
    regularFields[fieldID] = value;
  });
  if (Object.keys(regularFields).length > 0) {
    payload.fields = regularFields;
  }
  if (displayDomainReviews.length > 0) {
    payload.display_domain_reviews = displayDomainReviews;
  }
  return payload;
}




function interactionPayload(action: JsonRecord): JsonRecord {
  return asRecord(action.payload ?? action.data);
}


function Composer({
  composerRef,
  input,
  setInput,
  pendingArtifacts,
  pendingMacroControls,
	mode,
	authorityMode,
	authorityBusy,
	agentTurnRunning,
	stopTurnBusy,
	isSending,
	isUploading,
	interactionAction,
  respondingActionID,
  onSubmit,
  onUploadClick,
  onModeChange,
	onAuthorityModeChange,
	onStopTurn,
	onInteractionAction,
  onInvoke,
  onSelectArtifact,
  onRemoveArtifact,
  onRemoveMacroControl
}: {
  composerRef: RefObject<HTMLFormElement | null>;
  input: string;
  setInput: (value: string) => void;
  pendingArtifacts: ArtifactSummary[];
  pendingMacroControls: MacroControl[];
  mode: AgentMode;
  authorityMode: AuthorityMode;
  authorityBusy: boolean;
  agentTurnRunning: boolean;
  stopTurnBusy: boolean;
  isSending: boolean;
  isUploading: boolean;
	interactionAction: JsonRecord | null;
  respondingActionID: string;
  onSubmit: (event?: FormEvent) => void;
  onUploadClick: () => void;
  onModeChange: (mode: AgentMode) => void;
  onAuthorityModeChange: (mode: AuthorityMode) => void;
  onStopTurn: () => void;
	onInteractionAction: (interaction: JsonRecord, action: JsonRecord, payload?: JsonRecord) => void;
  onInvoke: DawInvoke;
  onSelectArtifact: (id: string) => void;
  onRemoveArtifact: (id: string) => void;
  onRemoveMacroControl: (id: string) => void;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [interactionHeight, setInteractionHeight] = useState(defaultComposerInteractionHeight);
  const menuRef = useRef<HTMLDivElement | null>(null);
  const interactionShellRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!menuOpen) {
      return;
    }
    const handlePointerDown = (event: MouseEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [menuOpen]);

  const chooseMode = (nextMode: AgentMode) => {
    onModeChange(mode === nextMode ? "default" : nextMode);
    setMenuOpen(false);
  };

  const beginInteractionResize = (event: ReactPointerEvent<HTMLDivElement>) => {
    event.preventDefault();
    const startY = event.clientY;
    const startHeight = interactionShellRef.current?.getBoundingClientRect().height ?? interactionHeight;
    const handlePointerMove = (moveEvent: PointerEvent) => {
      const nextHeight = clampComposerInteractionHeight(startHeight + startY - moveEvent.clientY);
      setInteractionHeight(nextHeight);
    };
    const handlePointerUp = () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", handlePointerUp);
      window.localStorage?.setItem("ask_vit_composer_interaction_height", String(interactionShellRef.current?.getBoundingClientRect().height ?? interactionHeight));
    };
    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", handlePointerUp);
  };

  return (
    <form className="composer" ref={composerRef} onSubmit={onSubmit}>
      {interactionAction && (
        <div className="composer-interaction-shell" ref={interactionShellRef} style={{ height: `${interactionHeight}px` }}>
          <div className="composer-resize-handle" role="separator" aria-orientation="horizontal" title="拖拽调整交互面板高度" onPointerDown={beginInteractionResize} />
          <ActionCard
            action={interactionAction}
            index={0}
            respondingActionID={respondingActionID}
            onInteractionAction={onInteractionAction}
            onInvoke={onInvoke}
            onSelectArtifact={onSelectArtifact}
            uiState={null}
            onMacroValuePreview={() => undefined}
            onMacroValueCommit={async () => undefined}
          />
        </div>
      )}
      {pendingArtifacts.length > 0 && (
        <div className="pending-row artifact-context-list">
          {pendingArtifacts.map((artifact) => (
            <ArtifactContextCard
              key={artifact.id}
              artifact={artifact}
              variant="compact"
              onSelect={() => onSelectArtifact(artifact.id)}
              onRemove={() => onRemoveArtifact(artifact.id)}
            />
          ))}
        </div>
      )}
      {pendingMacroControls.length > 0 && (
        <div className="pending-row macro-context-list">
          {pendingMacroControls.map((macro) => (
            <MacroContextCard
              key={macroControlID(macro)}
              macro={macro}
              onRemove={() => onRemoveMacroControl(macroControlID(macro))}
            />
          ))}
        </div>
      )}
      <div className="composer-main">
        <div className="composer-menu-host" ref={menuRef}>
          <button
            className={`icon-button upload ${menuOpen ? "active" : ""}`}
            type="button"
            title="添加上下文、文件或模式选项"
            onClick={() => setMenuOpen((current) => !current)}
            disabled={isUploading}
          >
            {isUploading ? <Loader2 className="spin" size={18} /> : <Plus size={18} />}
          </button>
          {menuOpen && (
            <div className="composer-menu" role="menu">
              <button type="button" role="menuitem" onClick={() => { onUploadClick(); setMenuOpen(false); }}>
                <Upload size={17} />
                <span>添加照片和文件</span>
              </button>
              <div className="composer-menu-separator" />
              <button type="button" role="menuitemcheckbox" aria-checked={mode === "plan"} onClick={() => chooseMode("plan")}>
                <Brain size={17} />
                <span>计划模式</span>
                <i className={mode === "plan" ? "on" : ""} />
              </button>
              <button type="button" role="menuitemcheckbox" aria-checked={mode === "goal"} onClick={() => chooseMode("goal")}>
                <Wand2 size={17} />
                <span>追求目标</span>
                <i className={mode === "goal" ? "on" : ""} />
              </button>
            </div>
          )}
        </div>
        <label className="authority-mode-control" title="控制可逆工程动作是否逐项请求确认">
          <span className="sr-only">Agent 权限模式</span>
          <select
            aria-label="Agent 权限模式"
            value={authorityMode}
            disabled={authorityBusy || agentTurnRunning}
            onChange={(event) => onAuthorityModeChange(event.currentTarget.value as AuthorityMode)}
          >
            <option value="manual_confirmation">Manual Confirmation</option>
            <option value="full_project_access">Full Project Access</option>
          </select>
        </label>
        <textarea
          value={input}
          rows={1}
          placeholder="和 Vit 说..."
          onChange={(event) => setInput(event.currentTarget.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) {
              event.preventDefault();
              onSubmit();
            }
          }}
        />
        {agentTurnRunning ? (
          <button className="send-button stop-turn-button" type="button" title="Stop Turn" disabled={stopTurnBusy} onClick={onStopTurn}>
            {stopTurnBusy ? <Loader2 className="spin" size={18} /> : <Square size={17} />}
            <span>Stop Turn</span>
          </button>
        ) : (
          <button className="send-button" type="submit" title="Send" disabled={isSending}>
            {isSending ? <Loader2 className="spin" size={18} /> : <Send size={18} />}
            <span>发送</span>
          </button>
        )}
      </div>
    </form>
  );
}

function PanelResizeHandle({
  side,
  onPointerDown,
  onReset,
  onNudge
}: {
  side: "left" | "right";
  onPointerDown: (event: ReactPointerEvent<HTMLDivElement>) => void;
  onReset: () => void;
  onNudge: (delta: number) => void;
}) {
  return (
    <div
      className={`panel-resize-handle ${side}`}
      role="separator"
      aria-label={`${side === "left" ? "左" : "右"}侧面板宽度`}
      aria-orientation="vertical"
      tabIndex={0}
      title="拖拽调整宽度；双击恢复默认"
      onPointerDown={onPointerDown}
      onDoubleClick={onReset}
      onKeyDown={(event) => {
        if (event.key === "ArrowLeft") {
          event.preventDefault();
          onNudge(-16);
        } else if (event.key === "ArrowRight") {
          event.preventDefault();
          onNudge(16);
        } else if (event.key === "Home") {
          event.preventDefault();
          onReset();
        }
      }}
    />
  );
}

function CollapsedWorkbench({ activeTab, onOpen }: { activeTab: WorkbenchTab; onOpen: (tab: WorkbenchTab) => void }) {
  return (
    <aside className="workbench-collapsed" aria-label="右侧工程面板">
      {workbenchTabs.map(({ key, label, icon: Icon }) => (
        <button key={key} type="button" className={activeTab === key ? "active" : ""} title={`打开${label}`} onClick={() => onOpen(key)}>
          <Icon size={16} />
          <span>{label}</span>
        </button>
      ))}
    </aside>
  );
}

function WorkbenchTabs({
  activeTab,
  setActiveTab,
  onCollapse
}: {
  activeTab: WorkbenchTab;
  setActiveTab: (tab: WorkbenchTab) => void;
  onCollapse?: () => void;
}) {
  return (
    <div className="workbench-tabs">
      {workbenchTabs.map(({ key, label, icon: Icon }) => (
        <button key={key} type="button" className={activeTab === key ? "active" : ""} onClick={() => setActiveTab(key)}>
          <Icon size={15} />
          <span>{label}</span>
        </button>
      ))}
      {onCollapse && (
        <button className="collapse-workbench" type="button" title="收起右侧面板" onClick={onCollapse}>
          <ChevronRight size={16} />
          <span>收起</span>
        </button>
      )}
    </div>
  );
}

function Workbench({
  activeTab,
  uiState,
  artifacts,
  selectedArtifact,
  conversationID,
  onSelectArtifact,
  onAttachArtifact,
  onRenameArtifact,
	onDeleteArtifact,
	onUploadClick,
	onModeChange,
	onAttachMacroControl,
  onInsertMacroControlCard,
  onMacroValuePreview,
  onMacroValueCommit,
  onMacroRename,
  onRefresh,
  checkoutBlocked
}: {
  activeTab: WorkbenchTab;
  uiState: AgentUIState | null;
  artifacts: ArtifactSummary[];
  selectedArtifact: ArtifactSummary | null;
  conversationID: string;
  onSelectArtifact: (id: string) => void;
  onAttachArtifact: (artifact: ArtifactSummary) => void;
  onRenameArtifact: (id: string, title: string) => Promise<void>;
  onDeleteArtifact: (id: string) => Promise<void>;
  onUploadClick: () => void;
  onModeChange: (mode: AgentMode) => void;
	onAttachMacroControl: (macro: MacroControl) => void;
  onInsertMacroControlCard: (macro: MacroControl) => void;
  onMacroValuePreview: (macro: MacroControl, value: number) => void;
  onMacroValueCommit: (macro: MacroControl, value: number) => Promise<void>;
  onMacroRename: (macro: MacroControl, name: string) => Promise<void>;
  onRefresh: () => Promise<void>;
  checkoutBlocked: boolean;
}) {
  if (activeTab === "history") {
    return <HistoryPane uiState={uiState} onRefresh={onRefresh} checkoutBlocked={checkoutBlocked} />;
  }
  if (activeTab === "macro") {
    return (
      <MacroPane
        uiState={uiState}
        onUploadClick={onUploadClick}
        onModeChange={onModeChange}
		onAttachMacroControl={onAttachMacroControl}
        onInsertMacroControlCard={onInsertMacroControlCard}
        onMacroValuePreview={onMacroValuePreview}
        onMacroValueCommit={onMacroValueCommit}
        onMacroRename={onMacroRename}
        onRefresh={onRefresh}
      />
    );
  }
  return (
    <MediaPane
      artifacts={artifacts}
      selectedArtifact={selectedArtifact}
      conversationID={conversationID}
      uiState={uiState}
      onSelectArtifact={onSelectArtifact}
      onAttachArtifact={onAttachArtifact}
      onRenameArtifact={onRenameArtifact}
      onDeleteArtifact={onDeleteArtifact}
      onUploadClick={onUploadClick}
      onRefresh={onRefresh}
    />
  );
}

function ProjectPane({ uiState }: { uiState: AgentUIState | null }) {
  const project = asRecord(uiState?.project);
  const goal = asRecord(uiState?.goal);
  const plan = asRecord(uiState?.agent_plan);
  const capabilities = asRecord(uiState?.capabilities);
  return (
    <div className="pane-body">
      <InfoGrid
        rows={[
          ["工程", lastPathPart(textValue(project.project_path, "Vit Project"))],
          ["轨道", textValue(project.user_track_count ?? project.track_count, "0")],
          ["图修订", textValue(project.graph_revision, "-")],
          ["工具", textValue(capabilities.tools, "-")]
        ]}
      />
      <section className="work-section">
        <h2>目标</h2>
        <KeyValue label="状态" value={statusLabel(goal.status ?? plan.status, "空闲")} />
        <KeyValue label="步骤" value={localizeDisplayText(textValue(goal.current_step ?? plan.current_step, "无")) || "无"} />
        <KeyValue label="停止" value={localizeDisplayText(textValue(goal.stop_reason ?? plan.stop_reason, "无")) || "无"} />
      </section>
    </div>
  );
}

function TracksPane({ uiState }: { uiState: AgentUIState | null }) {
  const tracks = uiState?.tracks ?? [];
  return (
    <div className="pane-body">
      <section className="work-section">
        <h2>轨道</h2>
        <div className="track-list">
          {tracks.length === 0 && <EmptyState label="暂无轨道" />}
          {tracks.map((track, index) => (
            <TrackRow key={textValue(track.id ?? track.track_id, `track-${index}`)} track={track} />
          ))}
        </div>
      </section>
    </div>
  );
}

function TrackRow({ track }: { track: JsonRecord }) {
  const name = textValue(track.name ?? track.track_name, "Track");
  const type = textValue(track.type ?? track.kind, "audio");
  const selected = Boolean(track.selected);
  return (
    <div className={`track-row ${selected ? "selected" : ""}`}>
      <span className="track-color" />
      <div>
        <strong>{name}</strong>
        <span>{type}</span>
      </div>
      <div className="track-flags">
        <b className={truthy(track.mute) ? "on red" : ""}>M</b>
        <b className={truthy(track.solo) ? "on yellow" : ""}>S</b>
        <b className={truthy(track.arm ?? track.armed) ? "on blue" : ""}>R</b>
      </div>
    </div>
  );
}

function PluginsPane({ uiState }: { uiState: AgentUIState | null }) {
  const rack = asRecord(uiState?.plugin_rack);
  const plugins = firstArray(rack.plugins, rack.items, rack.chain, rack.rack);
  return (
    <div className="pane-body">
      <section className="work-section">
        <h2>Plugin Rack</h2>
        {plugins.length === 0 && <EmptyState label="No plugins loaded" />}
        {plugins.map((raw, index) => {
          const plugin = asRecord(raw);
          return (
            <div className="plugin-row" key={textValue(plugin.id ?? plugin.plugin_id, `plugin-${index}`)}>
              <Plug size={16} />
              <div>
                <strong>{textValue(plugin.name ?? plugin.title, `Plugin ${index + 1}`)}</strong>
                <span>{textValue(plugin.vendor ?? plugin.status ?? plugin.kind, "rack node")}</span>
              </div>
            </div>
          );
        })}
      </section>
    </div>
  );
}

function MediaPane({
  artifacts,
  selectedArtifact,
  conversationID,
  uiState,
  onSelectArtifact,
  onAttachArtifact,
  onRenameArtifact,
  onDeleteArtifact,
  onUploadClick,
  onRefresh
}: {
  artifacts: ArtifactSummary[];
  selectedArtifact: ArtifactSummary | null;
  conversationID: string;
  uiState: AgentUIState | null;
  onSelectArtifact: (id: string) => void;
  onAttachArtifact: (artifact: ArtifactSummary) => void;
  onRenameArtifact: (id: string, title: string) => Promise<void>;
  onDeleteArtifact: (id: string) => Promise<void>;
  onUploadClick: () => void;
  onRefresh: () => Promise<void>;
}) {
  const [contextMenu, setContextMenu] = useState<MediaContextMenuState | null>(null);
  const [menuStatus, setMenuStatus] = useState("");
  const [listOpen, setListOpen] = useState(true);
  const [resourceURL, setResourceURL] = useState("");
  const [resourceBusy, setResourceBusy] = useState(false);
  const [renameTarget, setRenameTarget] = useState<ArtifactSummary | null>(null);
  const [renameDraft, setRenameDraft] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<ArtifactSummary | null>(null);
  const [artifactActionBusy, setArtifactActionBusy] = useState(false);

  useEffect(() => {
    if (!contextMenu) {
      return;
    }
    const closeMenu = () => setContextMenu(null);
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        closeMenu();
      }
    };
    window.addEventListener("click", closeMenu);
    window.addEventListener("keydown", closeOnEscape);
    return () => {
      window.removeEventListener("click", closeMenu);
      window.removeEventListener("keydown", closeOnEscape);
    };
  }, [contextMenu]);

  const metadata = useMemo(() => uploadMetadata(conversationID, uiState), [conversationID, uiState]);
  const metadataRef = useRef(metadata);
  const downloadChangeTokenRef = useRef("");

  useEffect(() => {
    metadataRef.current = metadata;
  }, [metadata]);

  const scanRecentDownloads = useCallback(async (sinceMinutes = 30, quiet = false, force = false) => {
    if (!quiet) {
      setResourceBusy(true);
    }
    if (!quiet) {
      setMenuStatus("正在扫描下载文件...");
    }
    try {
      const result = await scanDownloads({
        ...metadataRef.current,
        since_minutes: sinceMinutes,
        limit: 80,
        change_token: downloadChangeTokenRef.current,
        only_if_changed: quiet && !force && Boolean(downloadChangeTokenRef.current)
      });
      if (result.change_token) {
        downloadChangeTokenRef.current = result.change_token;
      }
      if (quiet && result.changed === false) {
        return;
      }
      const rows = result.artifacts ?? [];
      if (rows[0]) {
        onSelectArtifact(rows[0].id);
      }
      if (!quiet || rows.length > 0) {
        setMenuStatus(rows.length > 0 ? `发现 ${rows.length} 个新下载` : "没有新的下载文件");
      }
      if (!quiet || rows.length > 0) {
        await onRefresh();
      }
    } catch (scanError) {
      if (!quiet) {
        setMenuStatus(scanError instanceof Error ? scanError.message : "扫描下载失败");
      }
    } finally {
      if (!quiet) {
        setResourceBusy(false);
      }
    }
  }, [onRefresh, onSelectArtifact]);

  useEffect(() => {
    let cancelled = false;
    const wait = (ms: number) => new Promise<void>((resolve) => window.setTimeout(resolve, ms));
    const watchLoop = async () => {
      await scanRecentDownloads(30, true, true);
      while (!cancelled) {
        try {
          const result = await watchDownloads({
            change_token: downloadChangeTokenRef.current,
            timeout_ms: 25000,
            poll_ms: 1500
          });
          if (cancelled) {
            return;
          }
          if (result.changed) {
            await scanRecentDownloads(30, true, true);
          } else if (result.change_token) {
            downloadChangeTokenRef.current = result.change_token;
          }
        } catch {
          await wait(15000);
        }
      }
    };
    void watchLoop();
    return () => {
      cancelled = true;
    };
  }, [scanRecentDownloads]);

  const openExternalURL = async () => {
    const target = normalizeBrowserURL(resourceURL) || defaultExternalBrowserURL;
    setResourceURL(target);
    const didPost = postExternalBrowserMessage("open_url", { url: target });
    if (!didPost) {
      window.open(target, "_blank", "noopener,noreferrer");
    }
    setMenuStatus("已在系统浏览器打开");
  };

  const saveResourceURL = async () => {
    const target = normalizeBrowserURL(resourceURL);
    if (!target) {
      setMenuStatus("请先输入网址");
      return;
    }
    setResourceBusy(true);
    setMenuStatus("");
    try {
      const result = await registerResourceURL({ ...metadataRef.current, url: target, title: target });
      if (result.artifact) {
        onSelectArtifact(result.artifact.id);
      }
      setMenuStatus("链接已保存到资料库");
      await onRefresh();
    } catch (saveError) {
      setMenuStatus(saveError instanceof Error ? saveError.message : "保存链接失败");
    } finally {
      setResourceBusy(false);
    }
  };

  const openContextMenu = (artifact: ArtifactSummary, event: ReactMouseEvent<HTMLElement>) => {
    event.preventDefault();
    event.stopPropagation();
    const menuWidth = 220;
    const menuHeight = 250;
    const x = Math.max(12, Math.min(event.clientX, window.innerWidth - menuWidth));
    const y = Math.max(12, Math.min(event.clientY, window.innerHeight - menuHeight));
    setContextMenu({ artifact, x, y });
  };

  const copyArtifactValue = async (label: string, value: string) => {
    const text = value.trim();
    if (!text) {
      setMenuStatus(`${label}暂无内容`);
      setContextMenu(null);
      return;
    }
    try {
      await navigator.clipboard.writeText(text);
      setMenuStatus(`已复制${label}`);
    } catch {
      setMenuStatus("复制失败，请手动选择文本");
    }
    setContextMenu(null);
  };

  const openRenameDialog = (artifact: ArtifactSummary) => {
    setRenameTarget(artifact);
    setRenameDraft(artifactLabel(artifact));
    setContextMenu(null);
  };

  const submitRename = async (event: FormEvent) => {
    event.preventDefault();
    if (!renameTarget) {
      return;
    }
    const title = renameDraft.trim();
    if (!title) {
      setMenuStatus("名称不能为空");
      return;
    }
    setArtifactActionBusy(true);
    try {
      await onRenameArtifact(renameTarget.id, title);
      setMenuStatus("已重命名");
      setRenameTarget(null);
    } catch (renameError) {
      setMenuStatus(renameError instanceof Error ? renameError.message : "重命名失败");
    } finally {
      setArtifactActionBusy(false);
    }
  };

  const confirmDelete = async () => {
    if (!deleteTarget) {
      return;
    }
    setArtifactActionBusy(true);
    try {
      await onDeleteArtifact(deleteTarget.id);
      setMenuStatus("已从资料库移除");
      setDeleteTarget(null);
    } catch (deleteError) {
      setMenuStatus(deleteError instanceof Error ? deleteError.message : "删除失败");
    } finally {
      setArtifactActionBusy(false);
    }
  };

  return (
    <div className="pane-body media-pane">
      <div className="media-toolbar">
        <button className="media-pool-title-button" type="button" onClick={() => setListOpen((open) => !open)}>
          <Archive size={18} />
          <span>
            <strong>资料库</strong>
            <small>{artifacts.length} 个文件</small>
          </span>
        </button>
        <div className="media-toolbar-actions">
          <button className="tool-button" type="button" onClick={onUploadClick}>
            <Upload size={16} />
            <span>上传</span>
          </button>
        </div>
      </div>
      <form
        className="media-resource-bar"
        onSubmit={(event) => {
          event.preventDefault();
          void openExternalURL();
        }}
      >
        <label className="media-url-field">
          <Globe size={15} />
          <input
            value={resourceURL}
            spellCheck={false}
            onChange={(event) => setResourceURL(event.currentTarget.value)}
            placeholder="输入网址；留空打开浏览器"
          />
        </label>
        <button className="tool-button primary media-open-button" type="submit" disabled={resourceBusy} title="打开系统浏览器">
          <Globe size={16} />
          <span>打开</span>
        </button>
        <button
          className="tool-button media-icon-button"
          type="button"
          title="保存链接到资料库"
          aria-label="保存链接到资料库"
          onClick={saveResourceURL}
          disabled={resourceBusy}
        >
          <Plus size={16} />
        </button>
        <button
          className="tool-button media-icon-button"
          type="button"
          title="扫描本次会话下载"
          aria-label="扫描本次会话下载"
          onClick={() => scanRecentDownloads(30, false)}
          disabled={resourceBusy}
        >
          {resourceBusy ? <Loader2 className="spin" size={16} /> : <RefreshCw size={16} />}
        </button>
      </form>
      <div className={`media-status-line ${menuStatus ? "" : "empty"}`}>{menuStatus}</div>
      <div className={`media-grid ${listOpen ? "list-open" : "list-closed"}`}>
        <section className="artifact-list" aria-label="资料库文件列表">
          <div className="artifact-list-header">
            <span>文件</span>
            <small>{formatBytes(artifacts.reduce((total, artifact) => total + Number(artifact.size_bytes ?? 0), 0))}</small>
            <button className="artifact-list-close" type="button" title="隐藏文件列表" onClick={() => setListOpen(false)}>
              <X size={14} />
            </button>
          </div>
          <div className="artifact-list-scroll">
            {artifacts.length === 0 && <EmptyState label="暂无资料" />}
            {artifacts.map((artifact) => (
              <div
                key={artifact.id}
                role="button"
                tabIndex={0}
                className={`artifact-list-item ${selectedArtifact?.id === artifact.id ? "active" : ""}`}
                onClick={() => onSelectArtifact(artifact.id)}
                onContextMenu={(event) => openContextMenu(artifact, event)}
                onKeyDown={(event) => {
                  if (event.key === "Enter" || event.key === " ") {
                    event.preventDefault();
                    onSelectArtifact(artifact.id);
                  }
                }}
              >
                <div className="artifact-type-icon">{artifactIcon(artifact)}</div>
                <div className="artifact-row-main">
                  <strong>{artifactLabel(artifact)}</strong>
                  <span>{artifact.kind ?? "artifact"} 路 {formatBytes(artifact.size_bytes)}</span>
                </div>
                <button
                  className="artifact-more-button"
                  type="button"
                  title="更多操作"
                  onClick={(event) => openContextMenu(artifact, event)}
                >
                  <MoreHorizontal size={15} />
                </button>
              </div>
            ))}
          </div>
        </section>
        <ArtifactPreview selectedArtifact={selectedArtifact} onAttachArtifact={onAttachArtifact} />
      </div>
      {contextMenu && (
        <div
          className="media-context-menu"
          style={{ left: contextMenu.x, top: contextMenu.y }}
          role="menu"
          onClick={(event) => event.stopPropagation()}
        >
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              onSelectArtifact(contextMenu.artifact.id);
              setContextMenu(null);
            }}
          >
            预览
          </button>
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              onAttachArtifact(contextMenu.artifact);
              setContextMenu(null);
            }}
          >
            附加到对话          </button>
          <div className="media-menu-separator" />
          <button type="button" role="menuitem" onClick={() => copyArtifactValue("地址", artifactAddress(contextMenu.artifact))}>
            复制地址
          </button>
          <button type="button" role="menuitem" onClick={() => copyArtifactValue("文件名", artifactLabel(contextMenu.artifact))}>
            复制文件名          </button>
          <button type="button" role="menuitem" onClick={() => copyArtifactValue("ID", contextMenu.artifact.id)}>
            复制 ID
          </button>
          <div className="media-menu-separator" />
          <button type="button" role="menuitem" onClick={() => openRenameDialog(contextMenu.artifact)}>
            重命名          </button>
          <button
            className="danger"
            type="button"
            role="menuitem"
            onClick={() => {
              setDeleteTarget(contextMenu.artifact);
              setContextMenu(null);
            }}
          >
            从列表删除          </button>
        </div>
      )}
      {renameTarget && (
        <div className="media-dialog-backdrop" onClick={() => setRenameTarget(null)}>
          <form className="media-dialog" onSubmit={submitRename} onClick={(event) => event.stopPropagation()}>
            <h3>重命名</h3>
            <input value={renameDraft} onChange={(event) => setRenameDraft(event.currentTarget.value)} autoFocus />
            <div className="media-dialog-actions">
              <button type="button" onClick={() => setRenameTarget(null)} disabled={artifactActionBusy}>
                取消
              </button>
              <button type="submit" className="primary" disabled={artifactActionBusy}>
                保存
              </button>
            </div>
          </form>
        </div>
      )}
      {deleteTarget && (
        <div className="media-dialog-backdrop" onClick={() => setDeleteTarget(null)}>
          <div className="media-dialog" onClick={(event) => event.stopPropagation()}>
            <h3>从列表删除</h3>
            <p>{artifactLabel(deleteTarget)}</p>
            <span>只会从资料库移除此记录，不会删除硬盘上的原文件。</span>
            <div className="media-dialog-actions">
              <button type="button" onClick={() => setDeleteTarget(null)} disabled={artifactActionBusy}>
                取消
              </button>
              <button type="button" className="danger" onClick={confirmDelete} disabled={artifactActionBusy}>
                删除
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function ArtifactPreview({
  selectedArtifact,
  onAttachArtifact
}: {
  selectedArtifact: ArtifactSummary | null;
  onAttachArtifact: (artifact: ArtifactSummary) => void;
}) {
  const [artifact, setArtifact] = useState<Artifact | null>(null);
  const [loading, setLoading] = useState(false);
  const [previewError, setPreviewError] = useState("");
  const [copyStatus, setCopyStatus] = useState("");
  const [imageZoom, setImageZoom] = useState(1);

  useEffect(() => {
    if (!selectedArtifact?.id) {
      setArtifact(null);
      return;
    }
    setLoading(true);
    setPreviewError("");
    void readArtifact(selectedArtifact.id)
      .then((response) => setArtifact(response.artifact ?? null))
      .catch((readError) => setPreviewError(readError instanceof Error ? readError.message : "读取失败"))
      .finally(() => setLoading(false));
  }, [selectedArtifact?.id]);

  useEffect(() => {
    setImageZoom(1);
  }, [selectedArtifact?.id]);

  if (!selectedArtifact) {
    return <EmptyState label="尚未选择资料" />;
  }
  const current = artifact ?? selectedArtifact;
  const isImagePreview = artifactIsImage(current);
  const preview = renderArtifactPreview(current, { imageZoom });
  const location = artifactAddress(current);
  const handlePreviewWheel = (event: ReactWheelEvent<HTMLDivElement>) => {
    if (!isImagePreview) {
      return;
    }
    event.preventDefault();
    const direction = event.deltaY > 0 ? -1 : 1;
    setImageZoom((value) => clampNumber(Math.round((value + direction * 0.12) * 100) / 100, 0.35, 4));
  };
  const copyLocation = async () => {
    try {
      await navigator.clipboard.writeText(location);
      setCopyStatus("已复制地址");
    } catch {
      setCopyStatus("复制失败");
    }
  };
  const revealLocation = async () => {
    try {
      await revealArtifact(current.id);
      setCopyStatus("已打开文件位置");
    } catch (revealError) {
      await copyLocation();
      setCopyStatus(revealError instanceof Error ? `无法打开，已复制地址：${revealError.message}` : "无法打开，已复制地址");
    }
  };
  return (
    <section className="artifact-preview">
      <div className="preview-header">
        <div>
          <h3>{artifactLabel(current)}</h3>
          <span>{artifactMetaLine(current)}</span>
        </div>
        <div className="preview-actions">
          <button className="icon-button" type="button" title="复制地址" onClick={copyLocation}>
            <Copy size={16} />
          </button>
          <button className="icon-button" type="button" title="附加到对话" onClick={() => onAttachArtifact(selectedArtifact)}>
            <Plus size={16} />
          </button>
        </div>
      </div>
      <button className="artifact-location" type="button" title="打开本地文件位置" onClick={revealLocation}>
        {location}
      </button>
      {copyStatus && <div className="preview-copy-status">{copyStatus}</div>}
      {loading && <div className="loading-line"><Loader2 className="spin" size={16} />Loading</div>}
      {previewError && <div className="notice warning"><AlertTriangle size={16} />{previewError}</div>}
      <div
        className={`artifact-preview-body ${isImagePreview ? "image-mode" : ""}`}
        onWheel={handlePreviewWheel}
        onDragStart={(event) => event.preventDefault()}
        onDoubleClick={() => {
          if (isImagePreview) {
            setImageZoom(1);
          }
        }}
      >
        {preview}
      </div>
      {isImagePreview && (
        <div className="image-zoom-status">
          <span>{Math.round(imageZoom * 100)}%</span>
          <button type="button" onClick={() => setImageZoom(1)}>复位</button>
        </div>
      )}
      {current.summary && <p className="artifact-summary">{current.summary}</p>}
    </section>
  );
}

function BrowserPane({
  conversationID,
  uiState,
  onCaptured,
  onRefresh
}: {
  conversationID: string;
  uiState: AgentUIState | null;
  onCaptured: (artifact: ArtifactSummary) => void;
  onRefresh: () => Promise<void>;
}) {
  const [address, setAddress] = useState("https://www.bing.com");
  const [bridgeReady, setBridgeReady] = useState(() => hasBrowserBridge());
  const [browserState, setBrowserState] = useState<BrowserBridgeState>({});
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const viewportRef = useRef<HTMLDivElement | null>(null);
  const captureRequestRef = useRef("");
  const addressRef = useRef(address);
  const browserStateRef = useRef<BrowserBridgeState>({});
  const conversationIDRef = useRef(conversationID);
  const uiStateRef = useRef<AgentUIState | null>(uiState);
  const onCapturedRef = useRef(onCaptured);
  const onRefreshRef = useRef(onRefresh);
  const scopeKey = mediaScopeKeyFromUIState(uiState) || historyScopeKeyFromUIState(uiState) || "unsaved";

  useEffect(() => {
    addressRef.current = address;
  }, [address]);

  useEffect(() => {
    browserStateRef.current = browserState;
  }, [browserState]);

  useEffect(() => {
    conversationIDRef.current = conversationID;
    uiStateRef.current = uiState;
    onCapturedRef.current = onCaptured;
    onRefreshRef.current = onRefresh;
  }, [conversationID, onCaptured, onRefresh, uiState]);

  const postBrowserMessage = useCallback((type: string, payload: JsonRecord = {}) => {
    const didPost = postToBrowserBridge(type, {
      ...payload,
      scope_key: scopeKey
    });
    setBridgeReady(didPost);
    return didPost;
  }, [scopeKey]);

  const syncBounds = useCallback(() => {
    const viewport = viewportRef.current;
    if (!viewport) {
      return;
    }
    const rect = viewport.getBoundingClientRect();
    postBrowserMessage("bounds", {
      visible: rect.width > 8 && rect.height > 8,
      rect: {
        x: rect.left,
        y: rect.top,
        screen_x: window.screenX + rect.left,
        screen_y: window.screenY + rect.top,
        width: rect.width,
        height: rect.height,
        dpr: window.devicePixelRatio || 1
      }
    });
  }, [postBrowserMessage]);

  const saveCapturedPayload = useCallback(async (payload: JsonRecord) => {
    setBusy(true);
    setMessage("");
    try {
      const currentBrowserState = browserStateRef.current;
      const result = await captureBrowserPage({
        url: textValue(payload.url ?? currentBrowserState.url, ""),
        title: textValue(payload.title ?? currentBrowserState.title, ""),
        text: textValue(payload.text, ""),
        selection_text: textValue(payload.selection_text ?? payload.selected_text, ""),
        ...uploadMetadata(conversationIDRef.current, uiStateRef.current)
      });
      if (result.artifact) {
        onCapturedRef.current(result.artifact);
        setMessage("已捕获到资料库，并附加到当前对话");
        await onRefreshRef.current();
      } else {
        setMessage("捕获完成，但没有返回媒体记录");
      }
    } catch (captureError) {
      setMessage(captureError instanceof Error ? captureError.message : "捕获失败");
    } finally {
      setBusy(false);
    }
  }, []);

  useEffect(() => {
    const bridge = browserBridge();
    setBridgeReady(Boolean(bridge));
    if (!bridge) {
      return;
    }
    const handleMessage = (event: MessageEvent) => {
      const data = browserBridgePayload(event.data);
      if (textValue(data.channel, "") !== browserBridgeChannel) {
        return;
      }
      const type = textValue(data.type, "");
      if (type === "state") {
        const state = asRecord(data.state);
        setBrowserState(state as BrowserBridgeState);
        const nextURL = textValue(state.url, "");
        if (nextURL) {
          setAddress(nextURL);
        }
        const errorText = textValue(state.last_error, "");
        if (errorText) {
          setMessage(errorText);
        }
      }
      if (type === "capture_result") {
        const requestID = textValue(data.request_id, "");
        if (requestID && requestID !== captureRequestRef.current) {
          return;
        }
        const errorText = textValue(data.error, "");
        if (errorText) {
          setBusy(false);
          setMessage(errorText);
          return;
        }
        void saveCapturedPayload(asRecord(data.payload));
      }
    };
    bridge.addEventListener?.("message", handleMessage);
    postToBrowserBridge("mount", { url: addressRef.current, scope_key: scopeKey, visible: true });
    window.requestAnimationFrame(syncBounds);
    return () => {
      bridge.removeEventListener?.("message", handleMessage);
      postToBrowserBridge("hide", { scope_key: scopeKey });
    };
  }, [saveCapturedPayload, scopeKey, syncBounds]);

  useEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport || !bridgeReady) {
      return;
    }
    const resizeObserver = new ResizeObserver(() => syncBounds());
    resizeObserver.observe(viewport);
    window.addEventListener("resize", syncBounds);
    window.addEventListener("scroll", syncBounds, true);
    const timer = window.setInterval(syncBounds, 1200);
    syncBounds();
    return () => {
      resizeObserver.disconnect();
      window.removeEventListener("resize", syncBounds);
      window.removeEventListener("scroll", syncBounds, true);
      window.clearInterval(timer);
    };
  }, [bridgeReady, syncBounds]);

  const navigate = (rawURL: string) => {
    const target = normalizeBrowserURL(rawURL);
    if (!target) {
      return;
    }
    setAddress(target);
    setMessage("");
    postBrowserMessage("navigate", { url: target });
    window.requestAnimationFrame(syncBounds);
  };

  const capture = () => {
    if (!bridgeReady || !browserState.ready) {
      setMessage("WebView2 companion 尚未就绪");
      return;
    }
    const requestID = uniqueID("browser_capture");
    captureRequestRef.current = requestID;
    setBusy(true);
    setMessage("");
    postBrowserMessage("capture", { request_id: requestID });
  };

  const loading = Boolean(browserState.loading);
  const lastError = textValue(browserState.last_error, "");
  const statusLabel = bridgeReady
    ? browserState.ready
      ? loading
        ? "加载中"
        : "就绪"
      : "等待 WebView2"
    : "需要 Godot WebView2";

  return (
    <div className="pane-body browser-pane">
      <section className="browser-shell">
        <form
          className="browser-toolbar"
          onSubmit={(event) => {
            event.preventDefault();
            navigate(address);
          }}
        >
          <button className="icon-button" type="button" title="后退" onClick={() => postBrowserMessage("back")} disabled={!browserState.can_go_back}>
            <ChevronLeft size={16} />
          </button>
          <button className="icon-button" type="button" title="鍓嶈繘" onClick={() => postBrowserMessage("forward")} disabled={!browserState.can_go_forward}>
            <ChevronRight size={16} />
          </button>
          <button className="icon-button" type="button" title={loading ? "停止加载" : "刷新"} onClick={() => postBrowserMessage(loading ? "stop" : "reload")} disabled={!browserState.ready}>
            {loading ? <Square size={14} /> : <RefreshCw size={15} />}
          </button>
          <label className="browser-address">
            <Globe size={15} />
            <input
              value={address}
              spellCheck={false}
              onFocus={() => postBrowserMessage("focus")}
              onChange={(event) => setAddress(event.currentTarget.value)}
              placeholder="搜索或输入网址"
              disabled={!bridgeReady}
            />
          </label>
          <button className="tool-button primary" type="submit" disabled={!bridgeReady}>
            <Search size={16} />
            <span>打开</span>
          </button>
        </form>
        <div className="browser-status-line">
          <span className={`browser-status-dot ${browserState.ready ? "ready" : ""} ${lastError ? "error" : ""}`} />
          <strong>{statusLabel}</strong>
          <span>{textValue(browserState.title, "") || textValue(browserState.url, "") || "WebView2 companion browser"}</span>
          <button className="tool-button" type="button" onClick={capture} disabled={busy || !bridgeReady || !browserState.ready}>
            {busy ? <Loader2 className="spin" size={16} /> : <Archive size={16} />}
            <span>捕获到资料库</span>
          </button>
        </div>
        <div ref={viewportRef} className="browser-viewport" onClick={() => postBrowserMessage("focus")}>
          <div className="browser-viewport-placeholder">
            <Globe size={36} />
            <strong>{bridgeReady ? "WebView2 浏览器区域" : "等待 Godot WebView2 companion"}</strong>
            <span>{bridgeReady ? "真实浏览器会覆盖此区域；页面内容不会自动进入 Agent 上下文。" : "请从 Godot 内打开 Ask Vit。开发浏览器里不会伪装登录浏览器。"}</span>
          </div>
        </div>
        {(message || lastError) && <div className={`notice ${lastError ? "warning" : ""}`}>{message || lastError}</div>}
      </section>
    </div>
  );
}

type BrowserWebViewBridge = {
  postMessage: (message: unknown) => void;
  addEventListener?: (type: "message", listener: (event: MessageEvent) => void) => void;
  removeEventListener?: (type: "message", listener: (event: MessageEvent) => void) => void;
};

function browserBridge(): BrowserWebViewBridge | null {
  const hostWindow = window as unknown as { chrome?: { webview?: BrowserWebViewBridge } };
  return hostWindow.chrome?.webview ?? null;
}

function hasBrowserBridge(): boolean {
  return Boolean(browserBridge()?.postMessage);
}

function postToBrowserBridge(type: string, payload: JsonRecord = {}): boolean {
  const bridge = browserBridge();
  if (!bridge?.postMessage) {
    return false;
  }
  try {
    bridge.postMessage({
      ...payload,
      channel: browserBridgeChannel,
      type
    });
    return true;
  } catch {
    return false;
  }
}

function postExternalBrowserMessage(type: string, payload: JsonRecord = {}): boolean {
  const bridge = browserBridge();
  if (!bridge?.postMessage) {
    return false;
  }
  try {
    bridge.postMessage({
      ...payload,
      channel: externalBrowserChannel,
      type
    });
    return true;
  } catch {
    return false;
  }
}

function postAgentMutationsFromChatResponse(response: ChatResponse, source: string): boolean {
  return postAgentMutations(agentMutationsFromChatResponse(response), source);
}

function postAgentMutationsFromInvokeResponse(response: AgentInvokeResponse, source: string): boolean {
  return postAgentMutations(agentMutationsFromInvokeResponse(response), source);
}

function postAgentMutations(mutations: JsonRecord[], source: string): boolean {
  if (mutations.length === 0) {
    return false;
  }
  const bridge = browserBridge();
  if (!bridge?.postMessage) {
    return false;
  }
  try {
    bridge.postMessage({
      channel: agentMutationBridgeChannel,
      type: "agent_kernel_result",
      source,
      mutation_count: mutations.length,
      mutations
    });
    return true;
  } catch {
    return false;
  }
}

function agentMutationsFromChatResponse(response: ChatResponse): JsonRecord[] {
  const mutations: JsonRecord[] = [];
  firstArray(response.executed_kernel_reply).forEach((entry) => {
    appendAgentMutationsFromExecution(mutations, asRecord(entry));
  });
  return compactAgentMutations(mutations);
}

function agentMutationsFromInvokeResponse(response: AgentInvokeResponse): JsonRecord[] {
  const result = asRecord(response.result);
  if (Object.keys(result).length === 0) {
    return [];
  }
  const entry: JsonRecord = {
    status: response.status,
    command_name: response.command_name,
    tool: response.tool,
    risk_level: response.risk_level,
    result
  };
  const mutations: JsonRecord[] = [];
  appendAgentMutationsFromExecution(mutations, entry);
  return compactAgentMutations(mutations);
}

function appendAgentMutationsFromExecution(mutations: JsonRecord[], entry: JsonRecord): void {
  const result = asRecord(entry.result);
  const payload = Object.keys(result).length > 0 ? result : entry;
  if (!agentExecutionSucceeded(entry, payload)) {
    return;
  }
  const commandName = textValue(entry.command_name ?? entry.tool ?? entry.cmd ?? entry.command ?? payload.command_name ?? payload.tool ?? payload.cmd ?? payload.command, "").toLowerCase();
  const uiAction = textValue(payload.ui_action ?? entry.ui_action, "").toLowerCase();
  const trackID = textValue(payload.track_id ?? payload.target_track_id ?? entry.track_id, "");
  const clipID = textValue(payload.clip_id ?? payload.new_clip_id ?? payload.created_clip_id ?? entry.clip_id, "");
  const notes = firstArray(payload.notes).map(asRecord);
  const selectedExistingMacro = uiAction === "rack_macro_selected" || truthy(payload.selected_existing_macro);

  if (uiAction === "select_clip" || commandName === "select_clip" || hasCreatedClipPayload(payload)) {
    if (clipID) {
      mutations.push({
        kind: "select_clip",
        track_id: trackID,
        clip_id: clipID,
        created_clip_ids: firstArray(payload.created_clip_ids),
        source_command: commandName,
        requires_refresh: true
      });
    }
  }

  if (uiAction === "focus_track" || uiAction === "select_track" || commandName === "select_track") {
    if (trackID) {
      mutations.push({
        kind: "focus_track",
        track_id: trackID,
        source_command: commandName
      });
    }
  }

  if (uiAction === "focus_plugin" || uiAction === "select_plugin" || commandName === "select_plugin") {
    const pluginID = textValue(payload.plugin_id ?? payload.plugin_item_id ?? payload.node_id ?? entry.plugin_id, "");
    if (pluginID) {
      mutations.push({
        kind: "focus_plugin",
        track_id: trackID,
        plugin_id: pluginID,
        source_command: commandName
      });
    }
  }

  if (isTrackStateRefreshCommand(commandName, uiAction)) {
    mutations.push({
      kind: trackID ? "track_state_changed" : "project_state_changed",
      track_id: trackID,
      mute: payload.mute ?? payload.muted,
      solo: payload.solo,
      armed: payload.is_armed ?? payload.armed,
      volume_db: payload.volume_db ?? payload.db,
      source_command: commandName,
      requires_refresh: true
    });
  }

  if (isClipStateRefreshCommand(commandName, uiAction)) {
    mutations.push({
      kind: "project_state_changed",
      track_id: trackID,
      clip_id: clipID,
      source_command: commandName,
      requires_refresh: true
    });
  }

  if (uiAction === "remove_clips" || commandName === "remove_clips") {
    mutations.push({
      kind: "remove_clips",
      clip_ids: firstArray(payload.removed_clip_ids, payload.requested_clip_ids, payload.clip_ids),
      source_command: commandName,
      requires_refresh: true
    });
  }

  if (uiAction === "midi_note_patch" || isMidiNoteWriteCommand(commandName)) {
    if (clipID || notes.length > 0) {
      const insertedCount = numericValue(payload.inserted_count ?? payload.added_count);
      const mutatedCount = numericValue(payload.mutated_count);
      const quantizedCount = numericValue(payload.quantized_count);
      mutations.push({
        kind: "midi_notes_changed",
        track_id: trackID,
        clip_id: clipID,
        notes,
        inserted_count: insertedCount,
        mutated_count: mutatedCount,
        deleted_count: payload.deleted_count ?? payload.removed_count,
        quantized_count: quantizedCount,
        source_command: commandName,
        fetch_if_missing: notes.length === 0 && (insertedCount > 0 || mutatedCount > 0 || quantizedCount > 0)
      });
    }
  }

  if (uiAction === "rack_macro_value_changed" || commandName === "control.set_macro_values" || commandName === "control_set_macro_values") {
    const macro = normalizeMacroControl(asRecord(payload.macro ?? payload.control ?? payload));
    const macroID = macroControlID(macro) || textValue(payload.macro_id ?? entry.macro_id, "");
    if (macroID) {
      mutations.push(macroControlValueMutation({ ...macro, macro_id: macroID }, finiteNumber(payload.value ?? macro.value, macroControlValue(macro)), true, "agent_invoke"));
    }
    firstArray(payload.applied_parameters).map(asRecord).forEach((applied) => {
      if (textValue(applied.control ?? applied.param_id, "") !== "track.volume") {
        return;
      }
      const appliedTrackID = textValue(applied.track_id, "");
      const targetValue = finiteNumber(applied.target_value, NaN);
      if (!appliedTrackID || Number.isNaN(targetValue)) {
        return;
      }
      mutations.push({
        kind: "track_state_changed",
        track_id: appliedTrackID,
        volume_db: targetValue,
        source_command: commandName,
        requires_refresh: true
      });
    });
  }

  if (!selectedExistingMacro && (uiAction === "rack_macro_upserted" || commandName === "plugin_map_macro_to_params" || commandName === "control_add_macro")) {
    const macro = normalizeMacroControl(asRecord(payload.macro ?? payload));
    const macroID = macroControlID(macro) || textValue(payload.macro_id ?? entry.macro_id, "");
    if (macroID) {
      mutations.push({
        kind: "rack_macro_upserted",
        macro_id: macroID,
        macro: { ...macro, macro_id: macroID },
        source_command: commandName,
        requires_refresh: true
      });
    }
  }

  if (uiAction === "rack_macro_renamed" || commandName === "control.rename_macro" || commandName === "control_rename_macro") {
    const macro = normalizeMacroControl(asRecord(payload.macro ?? payload));
    const macroID = macroControlID(macro) || textValue(payload.macro_id ?? entry.macro_id, "");
    const name = textValue(payload.name ?? payload.new_name ?? macro.name, "");
    if (macroID && name) {
      mutations.push({
        kind: "rack_macro_renamed",
        macro_id: macroID,
        name,
        macro: { ...macro, macro_id: macroID, name },
        source_command: commandName,
        requires_refresh: true
      });
    }
  }

  if (uiAction === "rack_macro_binding_added" || commandName === "control.add_binding" || commandName === "control_add_binding") {
    const macroID = textValue(payload.macro_id ?? entry.macro_id, "");
    const binding = asRecord(payload.binding ?? payload);
    if (macroID && Object.keys(binding).length > 0) {
      mutations.push({
        kind: "rack_macro_binding_added",
        macro_id: macroID,
        binding,
        source_command: commandName,
        requires_refresh: true
      });
    }
  }
}

function agentExecutionSucceeded(entry: JsonRecord, payload: JsonRecord): boolean {
  const entryStatus = textValue(entry.status, "").toLowerCase();
  const payloadStatus = textValue(payload.status, "").toLowerCase();
  if (
    entryStatus === "error" ||
    entryStatus === "failed" ||
    entryStatus === "failure" ||
    payloadStatus === "error" ||
    payloadStatus === "failed" ||
    payloadStatus === "failure"
  ) {
    return false;
  }
  const status = entryStatus || payloadStatus;
  return status === "" || status === "ok" || status === "success" || status === "completed";
}

function hasCreatedClipPayload(payload: JsonRecord): boolean {
  return textValue(payload.new_clip_id ?? payload.created_clip_id, "") !== "" || firstArray(payload.created_clip_ids).length > 0;
}

function isMidiNoteWriteCommand(commandName: string): boolean {
  return [
    "apply_midi_note_patch",
    "add_midi_notes",
    "add_midi_notes_bulk",
    "mutate_midi_notes",
    "delete_midi_notes",
    "midi.apply_note_patch",
    "midi.add_notes",
    "midi.add_notes_bulk"
  ].includes(commandName);
}

function isTrackStateRefreshCommand(commandName: string, uiAction: string): boolean {
  if (uiAction === "track_state_changed" || uiAction === "project_state_changed") {
    return true;
  }
  return [
    "add_track",
    "track.add",
    "add_audio_track",
    "track.add_audio",
    "append_ghost_track",
    "track.append_ghost",
    "delete_track",
    "track.delete",
    "rename_track",
    "track.rename",
    "set_mute",
    "track.mute",
    "set_solo",
    "track.solo",
    "arm_track",
    "track.arm",
    "set_volume",
    "track.volume",
    "rack_add_node",
    "rack.add_node",
    "instantiate_plugin",
    "plugin.load_to_rack",
    "delete_plugin",
    "plugin.delete",
    "freeze_track",
    "track.freeze",
    "unfreeze_track",
    "track.unfreeze"
  ].includes(commandName);
}

function isClipStateRefreshCommand(commandName: string, uiAction: string): boolean {
  if (uiAction === "clip_state_changed" || uiAction === "project_state_changed") {
    return true;
  }
  return [
    "move_clip",
    "clip.move",
    "resize_clip",
    "clip.resize",
    "split_clip",
    "clip.split",
    "clone_clip",
    "clip.clone",
    "add_audio_clip",
    "clip.add_audio",
    "import_audio",
    "clip.import_audio",
    "import_media_to_track",
    "clip.import_media_to_track",
    "insert_midi_clip",
    "midi.insert_clip",
    "create_midi_clip",
    "midi.create_clip"
  ].includes(commandName);
}

function numericValue(value: unknown): number {
  const n = Number(value);
  return Number.isFinite(n) ? n : 0;
}

function compactAgentMutations(mutations: JsonRecord[]): JsonRecord[] {
  const seen = new Set<string>();
  const out: JsonRecord[] = [];
  mutations.forEach((mutation, index) => {
    const key = [
      textValue(mutation.kind, ""),
      textValue(mutation.track_id, ""),
      textValue(mutation.clip_id, ""),
      textValue(mutation.macro_id, ""),
      textValue(mutation.source_command, ""),
      index
    ].join(":");
    if (seen.has(key)) {
      return;
    }
    seen.add(key);
    out.push(mutation);
  });
  return out;
}

function browserBridgePayload(raw: unknown): JsonRecord {
  if (typeof raw === "string") {
    try {
      return asRecord(JSON.parse(raw));
    } catch {
      return {};
    }
  }
  return asRecord(raw);
}

function normalizeBrowserURL(rawURL: string): string {
  const value = rawURL.trim();
  if (!value) {
    return "";
  }
  const lower = value.toLowerCase();
  if (
    lower.startsWith("http://") ||
    lower.startsWith("https://") ||
    lower.startsWith("file://") ||
    lower.startsWith("data:") ||
    lower.startsWith("about:")
  ) {
    return value;
  }
  if (/^(localhost|127\.0\.0\.1|\[::1\])(:\d+)?(\/.*)?$/i.test(value)) {
    return `http://${value}`;
  }
  if (value.includes(".") && !/\s/.test(value)) {
    return `https://${value}`;
  }
  return `https://www.bing.com/search?q=${encodeURIComponent(value)}`;
}

function MacroPane({
	uiState,
	onUploadClick,
	onModeChange,
	onAttachMacroControl,
  onInsertMacroControlCard,
  onMacroValuePreview,
  onMacroValueCommit,
  onMacroRename,
  onRefresh
}: {
  uiState: AgentUIState | null;
  onUploadClick: () => void;
  onModeChange: (mode: AgentMode) => void;
	onAttachMacroControl: (macro: MacroControl) => void;
  onInsertMacroControlCard: (macro: MacroControl) => void;
  onMacroValuePreview: (macro: MacroControl, value: number) => void;
  onMacroValueCommit: (macro: MacroControl, value: number) => Promise<void>;
  onMacroRename: (macro: MacroControl, name: string) => Promise<void>;
  onRefresh: () => Promise<void>;
}) {
  const macros = macroControlsFromUIState(uiState);
  const summary = asRecord(asRecord(uiState?.ui_context).macro_control_summary);
  const selectedPlugin = asRecord(uiState?.selected_plugin);
  const activeMode = activeWorkflowModeLabel(uiState);
  return (
    <div className="pane-body">
      <section className="work-section">
        <div className="macro-pane-header">
          <div>
            <h2>宏控制</h2>
            <span>{macroPaneSubtitle(macros.length, summary, selectedPlugin)}</span>
          </div>
          <button className="icon-button refresh" type="button" title="刷新宏控制" onClick={() => void onRefresh()}>
            <RefreshCw size={15} />
          </button>
        </div>
		<MacroModeStrip activeMode={activeMode} onUploadClick={onUploadClick} onModeChange={onModeChange} />
        {macros.length === 0 && <EmptyState label="暂无宏控制" />}
        <div className="macro-list">
          {macros.map((macro) => (
            <MacroControlPanelCard
              key={macroControlID(macro)}
              macro={macro}
              onAttach={() => onAttachMacroControl(macro)}
              onInsertCard={() => onInsertMacroControlCard(macro)}
              onPreviewValue={(value) => onMacroValuePreview(macro, value)}
              onCommitValue={(value) => onMacroValueCommit(macro, value)}
              onRename={(name) => onMacroRename(macro, name)}
            />
          ))}
        </div>
      </section>
    </div>
  );
}

function MacroModeStrip({
	activeMode,
	onUploadClick,
	onModeChange
}: {
	activeMode: string;
	onUploadClick: () => void;
	onModeChange: (mode: AgentMode) => void;
}) {
  const items: Array<{ key: string; label: string; enabled: boolean; title: string; Icon: LucideIcon; action: () => void }> = [
    { key: "add_file", label: "添加文件", enabled: true, title: "从输入区添加文件资料", Icon: Upload, action: onUploadClick },
    { key: "plan", label: "计划模式", enabled: true, title: "切换到计划模式", Icon: Brain, action: () => onModeChange("plan") },
    { key: "goal", label: "目标模式", enabled: true, title: "切换到目标模式", Icon: Wand2, action: () => onModeChange("goal") },
  ];
  return (
    <div className="macro-mode-strip">
      <div className="macro-mode-status">
        <Activity size={14} />
        <span>{activeMode}</span>
      </div>
      <div className="macro-mode-actions">
        {items.map(({ key, label, enabled, title, Icon, action }) => (
          <button key={key} type="button" disabled={!enabled} title={enabled ? title : `${label} 暂不可用`} onClick={action}>
            <Icon size={14} />
            <span>{label}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

function MacroControlPanelCard({
  macro,
  onAttach,
  onInsertCard,
  onPreviewValue,
  onCommitValue,
  onRename
}: {
  macro: MacroControl;
  onAttach: () => void;
  onInsertCard: () => void;
  onPreviewValue: (value: number) => void;
  onCommitValue: (value: number) => Promise<void>;
  onRename: (name: string) => Promise<void>;
}) {
  const [busy, setBusy] = useState(false);
  const label = macroControlLabel(macro);
  const [draftName, setDraftName] = useState(label);
  const disabledReason = macroControlDisabledReason(macro);
  const disabled = Boolean(disabledReason);
  useEffect(() => {
    setDraftName(label);
  }, [label, macroControlID(macro)]);
  const commit = async (value: number) => {
    setBusy(true);
    try {
      await onCommitValue(value);
    } finally {
      setBusy(false);
    }
  };
  const commitName = async () => {
    const cleanName = draftName.trim();
    if (!cleanName || cleanName === label) {
      setDraftName(label);
      return;
    }
    setBusy(true);
    try {
      await onRename(cleanName);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className={`macro-panel-card ${disabled ? "disabled" : ""}`}>
      <div className="macro-card-head">
        <SlidersHorizontal size={16} />
        <div>
          <input
            className="macro-name-input"
            type="text"
            value={draftName}
            title="Macro name"
            aria-label="Macro name"
            disabled={busy || !macroControlID(macro)}
            onChange={(event) => setDraftName(event.currentTarget.value)}
            onBlur={() => void commitName()}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                event.currentTarget.blur();
              }
              if (event.key === "Escape") {
                event.preventDefault();
                setDraftName(label);
                event.currentTarget.blur();
              }
            }}
          />
          <span>{macroControlMeta(macro)}</span>
        </div>
        <em>{macroValueText(macroControlValue(macro), macro)}</em>
      </div>
      <MacroSlider
        macro={macro}
        disabled={disabled || busy}
        onPreviewValue={onPreviewValue}
        onCommitValue={commit}
      />
      <div className="macro-card-foot">
        <span>{disabledReason || macroBindingSummary(macro)}</span>
        <div>
          <button type="button" title="引用到输入区" onClick={onAttach}>
            <Link2 size={14} />
          </button>
          <button type="button" title="放入对话流控件" onClick={onInsertCard}>
            <Send size={14} />
          </button>
        </div>
      </div>
    </div>
  );
}

function MacroActionCard({
  action,
  uiState,
  respondingActionID,
  onMacroValuePreview,
  onMacroValueCommit
}: {
  action: JsonRecord;
  uiState: AgentUIState | null;
  respondingActionID: string;
  onMacroValuePreview: (macro: MacroControl, value: number) => void;
  onMacroValueCommit: (macro: MacroControl, value: number) => Promise<void>;
}) {
  const result = asRecord(action.result);
  const actionMacro = normalizeMacroControl(asRecord(action.macro ?? action.control ?? result.macro ?? action.payload ?? result));
  const macroID = macroControlID(actionMacro) || textValue(action.macro_id ?? action.id, "");
  const liveMacro = macroControlsFromUIState(uiState).find((macro) => macroControlID(macro) === macroID);
  const macro = liveMacro ?? actionMacro;
  const isBusy = respondingActionID === `macro:${macroControlID(macro)}`;
  return (
    <div className={`action-card macro-action-card ${macroControlDisabledReason(macro) ? "attention" : "info"}`}>
      <SlidersHorizontal size={16} />
      <div className="action-content">
        <div className="action-title-line">
          <strong>{macroControlLabel(macro)}</strong>
          <span>{liveMacro ? "实时控件" : "快照"}</span>
        </div>
        <MacroSlider
          macro={macro}
          disabled={Boolean(macroControlDisabledReason(macro)) || isBusy}
          onPreviewValue={(value) => onMacroValuePreview(macro, value)}
          onCommitValue={(value) => onMacroValueCommit(macro, value)}
        />
        <div className="macro-inline-meta">
          <span>{macroBindingSummary(macro)}</span>
          <b>{macroValueText(macroControlValue(macro), macro)}</b>
        </div>
      </div>
    </div>
  );
}

function MacroSlider({
  macro,
  disabled,
  onPreviewValue,
  onCommitValue
}: {
  macro: MacroControl;
  disabled?: boolean;
  onPreviewValue: (value: number) => void;
  onCommitValue: (value: number) => void | Promise<void>;
}) {
  const min = numericValue(macro.min ?? 0);
  const max = numericValue(macro.max ?? 1) || 1;
  const value = macroControlValue(macro);
  const [draft, setDraft] = useState(value);
  const draftRef = useRef(value);
  const lastCommitRef = useRef("");
  useEffect(() => {
    setDraft(value);
    draftRef.current = value;
  }, [macroControlID(macro), value]);
  const commit = () => {
    if (!disabled) {
      const next = draftRef.current;
      const commitKey = `${macroControlID(macro)}:${next.toFixed(4)}`;
      if (lastCommitRef.current === commitKey) {
        return;
      }
      lastCommitRef.current = commitKey;
      void onCommitValue(next);
    }
  };
  return (
    <div className="macro-slider-row">
      <input
        type="range"
        min={min}
        max={max}
        step={0.001}
        value={draft}
        disabled={disabled}
        onChange={(event) => {
          const next = Number(event.currentTarget.value);
          setDraft(next);
          draftRef.current = next;
          onPreviewValue(next);
        }}
        onPointerUp={commit}
        onMouseUp={commit}
        onTouchEnd={commit}
        onBlur={commit}
      />
      <span>{macroValueText(draft, macro)}</span>
    </div>
  );
}

function MacroContextCard({ macro, onRemove }: { macro: MacroControl; onRemove?: () => void }) {
  return (
    <button className="macro-context-card" type="button" title={macroBindingSummary(macro)}>
      <SlidersHorizontal size={14} />
      <span>{macroControlLabel(macro)}</span>
      <small>{macroValueText(macroControlValue(macro), macro)}</small>
      {onRemove && (
        <i
          role="button"
          aria-label="移除宏控制引用"
          onClick={(event) => {
            event.stopPropagation();
            onRemove();
          }}
        >
          <X size={13} />
        </i>
      )}
    </button>
  );
}

function macroControlsFromUIState(uiState: AgentUIState | null): MacroControl[] {
  const capabilities = asRecord(uiState?.capabilities);
  const uiContext = asRecord(uiState?.ui_context);
  return mergeMacroControls(
    firstArray(uiState?.macro_controls).map((item) => normalizeMacroControl(asRecord(item))),
    firstArray(capabilities.macro_controls, capabilities.macros, capabilities.macro_panels, capabilities.quick_actions).map((item) => normalizeMacroControl(asRecord(item))),
    macroRowsFromAny(uiContext.rack_control_macros).map((item) => normalizeMacroControl(asRecord(item))),
    macroRowsFromAny(uiContext.active_macro_controls).map((item) => normalizeMacroControl(asRecord(item)))
  );
}

function patchMacroControlValueInUIState(uiState: AgentUIState, macroID: string, value: number, macro?: MacroControl): AgentUIState {
  const capabilities = asRecord(uiState.capabilities);
  const uiContext = asRecord(uiState.ui_context);
  const rackControls = asRecord(uiContext.rack_control_macros);
  const shouldPatchTrackVolume = macro?.control === "track.volume" && Boolean(macro.track_id);
  return {
    ...uiState,
    tracks: shouldPatchTrackVolume ? patchTrackVolumeList(uiState.tracks, macro.track_id ?? "", value) as JsonRecord[] | undefined : uiState.tracks,
    selected_track: shouldPatchTrackVolume ? patchTrackVolumeRecord(uiState.selected_track, macro.track_id ?? "", value) as JsonRecord | null | undefined : uiState.selected_track,
    macro_controls: patchMacroControlValueList(uiState.macro_controls, macroID, value) as MacroControl[] | undefined,
    capabilities: {
      ...capabilities,
      macro_controls: patchMacroControlValueList(capabilities.macro_controls, macroID, value)
    },
    ui_context: {
      ...uiContext,
      active_macro_controls: patchMacroControlValueList(uiContext.active_macro_controls, macroID, value),
      rack_control_macros: {
        ...rackControls,
        macros: patchMacroControlValueList(rackControls.macros, macroID, value)
      }
    }
  };
}

function applyMacroValueOverridesToUIState(uiState: AgentUIState, overrides: Record<string, MacroValueOverride>): AgentUIState {
  const now = Date.now();
  let next = uiState;
  Object.entries(overrides).forEach(([macroID, override]) => {
    if (!override || override.expiresAt <= now) {
      delete overrides[macroID];
      return;
    }
    const trackVolume = uiStateTrackVolumeDB(next, override.macro.track_id ?? "");
    if (trackVolume != null && Math.abs(trackVolume - override.value) <= 0.05) {
      delete overrides[macroID];
      next = patchMacroControlValueInUIState(next, macroID, override.value, override.macro);
      return;
    }
    next = patchMacroControlValueInUIState(next, macroID, override.value, override.macro);
  });
  return next;
}

function uiStateTrackVolumeDB(uiState: AgentUIState, trackID: string): number | null {
  if (!trackID) {
    return null;
  }
  for (const track of firstArray(uiState.tracks).map(asRecord)) {
    const id = textValue(track.track_id ?? track.id, "");
    if (id !== trackID) {
      continue;
    }
    const value = finiteNumber(track.volume_db ?? track.volumeDb ?? track.fader_db ?? track.faderDb ?? track.gain_db ?? track.gainDb ?? track.db, NaN);
    return Number.isNaN(value) ? null : value;
  }
  return null;
}

function patchTrackVolumeList(raw: unknown, trackID: string, value: number): unknown {
  if (!Array.isArray(raw) || !trackID) {
    return raw;
  }
  return raw.map((item) => patchTrackVolumeRecord(item, trackID, value));
}

function patchTrackVolumeRecord(raw: unknown, trackID: string, value: number): unknown {
  const record = asRecord(raw);
  if (Object.keys(record).length === 0 || !trackID) {
    return raw;
  }
  const id = textValue(record.track_id ?? record.id, "");
  if (id !== trackID) {
    return raw;
  }
  return { ...record, volume_db: value, gain_db: value, fader_db: value, db: value };
}

function patchMacroControlValueList(raw: unknown, macroID: string, value: number): unknown {
  if (!Array.isArray(raw)) {
    return raw;
  }
  return raw.map((item) => {
    const record = asRecord(item);
    const id = textValue(record.macro_id ?? record.id ?? record.control_id, "");
    if (id !== macroID) {
      return item;
    }
    return { ...record, value };
  });
}

function patchMacroControlNameInUIState(uiState: AgentUIState, macroID: string, name: string): AgentUIState {
  const capabilities = asRecord(uiState.capabilities);
  const uiContext = asRecord(uiState.ui_context);
  const rackControls = asRecord(uiContext.rack_control_macros);
  return {
    ...uiState,
    macro_controls: patchMacroControlNameList(uiState.macro_controls, macroID, name) as MacroControl[] | undefined,
    capabilities: {
      ...capabilities,
      macro_controls: patchMacroControlNameList(capabilities.macro_controls, macroID, name),
      macros: patchMacroControlNameList(capabilities.macros, macroID, name),
      macro_panels: patchMacroControlNameList(capabilities.macro_panels, macroID, name),
      quick_actions: patchMacroControlNameList(capabilities.quick_actions, macroID, name)
    },
    ui_context: {
      ...uiContext,
      active_macro_controls: patchMacroControlNameList(uiContext.active_macro_controls, macroID, name),
      rack_control_macros: {
        ...rackControls,
        macros: patchMacroControlNameList(rackControls.macros, macroID, name)
      }
    }
  };
}

function patchMacroControlNameList(raw: unknown, macroID: string, name: string): unknown {
  if (!Array.isArray(raw)) {
    return raw;
  }
  return raw.map((item) => {
    const record = asRecord(item);
    const id = textValue(record.macro_id ?? record.id ?? record.control_id, "");
    if (id !== macroID) {
      return item;
    }
    return { ...record, name, label: name, title: name };
  });
}

function macroRowsFromAny(raw: unknown): JsonRecord[] {
  if (Array.isArray(raw)) {
    return raw.map(asRecord).filter((item) => Object.keys(item).length > 0);
  }
  const record = asRecord(raw);
  if (Object.keys(record).length === 0) {
    return [];
  }
  const nested = firstArray(record.macros, record.macro_controls, record.controls, record.items);
  if (nested.length > 0) {
    return nested.map(asRecord).filter((item) => Object.keys(item).length > 0);
  }
  return record.macro_id || record.id ? [record] : [];
}

function normalizeMacroControl(raw: JsonRecord): MacroControl {
  const macroID = textValue(raw.macro_id ?? raw.id ?? raw.control_id, "");
  const name = textValue(raw.name ?? raw.label ?? raw.title, macroID || "Macro");
  const trackID = textValue(raw.track_id ?? raw.track, "");
  const control = inferMacroControlKind(raw, macroID, name);
  let min = finiteNumber(raw.min ?? raw.minimum, 0);
  let max = Math.max(min + 0.0001, finiteNumber(raw.max ?? raw.maximum, 1));
  if (control === "track.volume" && min === 0 && max === 1) {
    min = -60;
    max = 12;
  }
  const value = clampNumber(finiteNumber(raw.value ?? raw.default, min), min, max);
  const bindings = firstArray(raw.bindings, raw.targets).map((item) => normalizeMacroBinding(asRecord(item)));
  if (bindings.length === 0 && control === "track.volume" && trackID) {
    bindings.push({
      binding_id: `binding_${macroID || "macro"}_track_volume`,
      control: "track.volume",
      track_id: trackID,
      plugin_id: "",
      param_id: "track.volume",
      param_name: "Track volume",
      source_min: 0,
      source_max: 1,
      target_min: min,
      target_max: max,
      enabled: true,
      unit: textValue(raw.unit, "dB")
    });
  }
  return {
    macro_id: macroID,
    id: macroID,
    name,
    track_id: trackID,
    plugin_id: textValue(raw.plugin_id ?? raw.plugin ?? raw.node_id, ""),
    role: textValue(raw.role, ""),
    control,
    control_type: textValue(raw.control_type ?? raw.type, "slider"),
    value,
    min,
    max,
    unit: textValue(raw.unit, control === "track.volume" ? "dB" : ""),
    bindings: normalizeTrackVolumeMacroBindings(bindings, control, min, max, textValue(raw.unit, control === "track.volume" ? "dB" : "")),
    binding_count: Math.max(bindings.length, numericValue(raw.binding_count)),
    source: textValue(raw.source ?? raw.created_by, "godot_rack"),
    status: textValue(raw.status ?? raw.state, "ready"),
    enabled: raw.enabled === undefined ? true : truthy(raw.enabled),
    updated_at: textValue(raw.updated_at, ""),
    updated_at_unix: numericValue(raw.updated_at_unix)
  };
}

function normalizeMacroBinding(raw: JsonRecord) {
  return {
    binding_id: textValue(raw.binding_id ?? raw.id, ""),
    control: textValue(raw.control, ""),
    track_id: textValue(raw.track_id ?? raw.track, ""),
    plugin_id: textValue(raw.plugin_id ?? raw.plugin ?? raw.node_id, ""),
    param_id: textValue(raw.param_id ?? raw.parameter_id ?? raw.key, ""),
    param_name: textValue(raw.param_name ?? raw.parameter_name ?? raw.label ?? raw.name, ""),
    source_min: finiteNumber(raw.source_min, 0),
    source_max: finiteNumber(raw.source_max, 1),
    target_min: finiteNumber(raw.target_min ?? raw.min, 0),
    target_max: finiteNumber(raw.target_max ?? raw.max, 1),
    enabled: raw.enabled === undefined ? true : truthy(raw.enabled),
    unit: textValue(raw.unit, "")
  };
}

function normalizeTrackVolumeMacroBindings(bindings: MacroControlBinding[], control: string, min: number, max: number, unit: string): MacroControlBinding[] {
  if (control !== "track.volume") {
    return bindings;
  }
  return bindings.map((binding) => {
    const isTrackVolumeBinding = binding.control === "track.volume" || binding.param_id === "track.volume";
    if (!isTrackVolumeBinding) {
      return binding;
    }
    return {
      ...binding,
      control: "track.volume",
      param_id: binding.param_id || "track.volume",
      param_name: binding.param_name || "Track volume",
      target_min: min,
      target_max: max,
      unit: binding.unit || unit || "dB"
    };
  });
}

function inferMacroControlKind(raw: JsonRecord, macroID: string, name: string): string {
  const explicit = textValue(raw.control ?? raw.param_id ?? raw.parameter_id, "");
  if (explicit) {
    return explicit;
  }
  const probe = `${macroID} ${name} ${textValue(raw.role, "")}`.toLowerCase();
  if (probe.includes("track_volume") || probe.includes("track.volume")) {
    return "track.volume";
  }
  return "";
}

function mergeMacroControls(...groups: MacroControl[][]): MacroControl[] {
  const byID = new Map<string, MacroControl>();
  groups.flat().forEach((macro) => {
    const normalized = normalizeMacroControl(asRecord(macro));
    const id = macroControlID(normalized);
    if (!id) {
      return;
    }
    byID.set(id, mergeMacroControlRecords(byID.get(id), normalized));
  });
  return Array.from(byID.values()).sort((a, b) => macroControlLabel(a).localeCompare(macroControlLabel(b)));
}

function mergeMacroControlRecords(previous: MacroControl | undefined, next: MacroControl): MacroControl {
  if (!previous) {
    return next;
  }
  const previousBindings = previous.bindings ?? [];
  const nextBindings = next.bindings ?? [];
  const merged: MacroControl = { ...previous, ...next };
  if (previousBindings.length > 0 && nextBindings.length === 0) {
    merged.bindings = previousBindings;
    merged.binding_count = Math.max(previous.binding_count ?? 0, previousBindings.length);
    if (isDefaultMacroRange(next) && !isDefaultMacroRange(previous)) {
      merged.min = previous.min;
      merged.max = previous.max;
      merged.unit = previous.unit;
      merged.value = previous.value;
    }
    if (!merged.control && previous.control) {
      merged.control = previous.control;
    }
    if (!merged.role && previous.role) {
      merged.role = previous.role;
    }
  }
  return normalizeMacroControl(asRecord(merged));
}

function isDefaultMacroRange(macro: MacroControl): boolean {
  return finiteNumber(macro.min, 0) === 0 && finiteNumber(macro.max, 1) === 1;
}

function macroControlID(macro: MacroControl): string {
  return textValue(macro.macro_id ?? macro.id, "");
}

function macroControlLabel(macro: MacroControl): string {
  return textValue(macro.name, macroControlID(macro) || "Macro");
}

function macroControlValue(macro: MacroControl): number {
  const min = finiteNumber(macro.min, 0);
  const max = Math.max(min + 0.0001, finiteNumber(macro.max, 1));
  return clampNumber(finiteNumber(macro.value, min), min, max);
}

function macroControlDisabledReason(macro: MacroControl): string {
  if (!macroControlID(macro)) {
    return "缺少宏控制 ID";
  }
  if (macro.enabled === false) {
    return "宏控制已禁用";
  }
  if ((macro.bindings ?? []).filter((binding) => binding.enabled !== false).length === 0) {
    return "暂无参数绑定，只能引用";
  }
  return "";
}

function macroControlMeta(macro: MacroControl): string {
  const parts = [
    macro.track_id ? `轨道 ${macro.track_id}` : "",
    macro.plugin_id ? `插件 ${macro.plugin_id}` : "",
    statusLabel(macro.status, textValue(macro.status, "ready")),
    textValue(macro.source, "")
  ].filter(Boolean);
  return parts.join(" · ") || "工程宏控制";
}

function macroBindingSummary(macro: MacroControl): string {
  const bindings = macro.bindings ?? [];
  if (bindings.length === 0) {
    return "暂无参数绑定";
  }
  const names = bindings
    .slice(0, 2)
    .map((binding) => textValue(binding.param_name ?? binding.param_id, "参数"))
    .filter(Boolean);
  const suffix = bindings.length > names.length ? ` +${bindings.length - names.length}` : "";
  return `${bindings.length} 个绑定${names.length > 0 ? `，${names.join("、")}${suffix}` : ""}`;
}

function macroValueText(value: number, macro?: MacroControl): string {
  const unit = textValue(macro?.unit, "");
  if (macro?.control === "track.volume" || unit.toLowerCase() === "db") {
    return `${value.toFixed(2)} dB`;
  }
  if (value >= 0 && value <= 1) {
    return `${Math.round(value * 100)}%`;
  }
  return unit ? `${value.toFixed(2)} ${unit}` : value.toFixed(2);
}

function macroPaneSubtitle(count: number, summary: JsonRecord, selectedPlugin: JsonRecord): string {
  const bindingCount = numericValue(summary.binding_count);
  const activeCount = numericValue(summary.active_macro_count);
  const pluginName = textValue(selectedPlugin.plugin_name, "");
  if (pluginName) {
    return `${pluginName} / ${activeCount || count} 个相关宏控 / ${bindingCount} 个绑定`;
  }
  return `${count} 个工程宏控 / ${bindingCount} 个绑定`;
}

function activeWorkflowModeLabel(uiState: AgentUIState | null): string {
  const uiContext = asRecord(uiState?.ui_context);
  const goal = asRecord(uiState?.goal);
  const raw = textValue(uiContext.active_workflow_mode ?? uiContext.active_mode_status ?? goal.status, "即时模式");
  if (raw === "plan") {
    return "计划模式";
  }
  if (raw === "goal") {
    return "目标模式";
  }
  if (raw === "auto_learn") {
    return "插件学习自动学习";
  }
  if (raw === "teach") {
    return "插件学习教学模式";
  }
  return statusLabel(raw, raw);
}

function macroControlCardAction(macro: MacroControl, source: string): JsonRecord {
  return {
    _ui_source: source,
    kind: "macro_control_card",
    type: "macro_control_card",
    id: `macro_card_${macroControlID(macro) || uniqueID("macro")}`,
    macro_id: macroControlID(macro),
    title: macroControlLabel(macro),
    status: macroControlDisabledReason(macro) ? "attention" : "ready",
    macro: normalizeMacroControl(asRecord(macro))
  };
}

function isMacroControlAction(action: JsonRecord): boolean {
  const kind = textValue(action.kind ?? action.type, "").toLowerCase();
  const commandName = textValue(action.command_name ?? action.cmd ?? action.command, "").toLowerCase();
  const result = asRecord(action.result);
  return (
    kind === "macro_control_card" ||
    kind === "macro_control" ||
    kind === "rack_macro_upserted" ||
    kind === "rack_macro_renamed" ||
    commandName === "control.set_macro_values" ||
    commandName === "control_set_macro_values" ||
    commandName === "control.rename_macro" ||
    commandName === "control_rename_macro" ||
    commandName === "plugin_map_macro_to_params" ||
    Boolean(action.macro_id && (asRecord(action.macro).macro_id || asRecord(result.macro).macro_id))
  );
}

function macroControlValueMutation(macro: MacroControl, value: number, commit: boolean, source: string): JsonRecord {
  const normalized = normalizeMacroControl(asRecord({ ...macro, value }));
  return {
    kind: "rack_macro_value_changed",
    macro_id: macroControlID(normalized),
    value,
    commit,
    source,
    requires_refresh: commit
  };
}

function macroControlRenameMutation(macro: MacroControl, name: string, source: string): JsonRecord {
  const normalized = normalizeMacroControl(asRecord({ ...macro, name }));
  return {
    kind: "rack_macro_renamed",
    macro_id: macroControlID(normalized),
    name,
    macro: normalized,
    source,
    requires_refresh: true
  };
}

function finiteNumber(value: unknown, fallback: number): number {
  const next = Number(value);
  return Number.isFinite(next) ? next : fallback;
}

type HistoryTreeNode = {
  index: number;
  id: string;
  nodeID: string;
  parentID: string;
  parentIndex: number;
  kind: string;
  label: string;
  meta: string;
  summary: string;
  branch: string;
  commitID: string;
  createdAt: string;
  active: boolean;
  depth: number;
  raw: JsonRecord;
};

type HistoryBranchRow = {
  name: string;
  head: string;
  active: boolean;
};

type HistoryWorktreeRow = {
  key: string;
  name: string;
  label: string;
  path: string;
  projectFilePath: string;
  commitID: string;
  active: boolean;
  isRoot: boolean;
  raw: JsonRecord;
};

type HistoryLoadedWorktree = {
  key: string;
  history: JsonRecord;
};

type HistoryNodeMenuState = {
  node: HistoryTreeNode;
  x: number;
  y: number;
};

type HistoryTreeLayout = {
  nodes: Array<
    HistoryTreeNode & {
      x: number;
      y: number;
    }
  >;
  connections: Array<{ id: string; points: string }>;
  width: number;
  height: number;
};

function HistoryPane({ uiState, onRefresh, checkoutBlocked }: { uiState: AgentUIState | null; onRefresh: () => Promise<void>; checkoutBlocked: boolean }) {
  const activeHistory = asRecord(uiState?.project_history);
  const collaboration = asRecord(uiState?.collaboration);
  const reservations = useMemo(() => firstArray(collaboration.reservations).map(asRecord), [collaboration]);
  const childTasks = useMemo(() => firstArray(collaboration.child_tasks).map(asRecord), [collaboration]);
  const worktrees = useMemo(() => historyWorktreeRows(activeHistory), [activeHistory]);
  const activeWorktreeKey = worktrees.find((worktree) => worktree.active)?.key ?? worktrees[0]?.key ?? "";
  const [selectedWorktreeKey, setSelectedWorktreeKey] = useState("");
  const [loadedWorktree, setLoadedWorktree] = useState<HistoryLoadedWorktree | null>(null);
  const selectedWorktree = worktrees.find((worktree) => worktree.key === selectedWorktreeKey) ?? worktrees.find((worktree) => worktree.key === activeWorktreeKey) ?? worktrees[0] ?? null;
  const selectedWorktreeIsActive = selectedWorktree?.active ?? true;
  const displayHistory = selectedWorktreeIsActive
    ? activeHistory
    : loadedWorktree?.key === selectedWorktree?.key
      ? loadedWorktree.history
      : {};
  const branches = useMemo(() => historyBranchRows(displayHistory), [displayHistory]);
  const nodes = useMemo(() => historyTreeNodes(displayHistory), [displayHistory]);
  const treeLayout = useMemo(() => historyTreeLayout(nodes), [nodes]);
  const [selectedBranchName, setSelectedBranchName] = useState("");
  const [selectedNodeID, setSelectedNodeID] = useState("");
  const [busyAction, setBusyAction] = useState("");
  const [status, setStatus] = useState("");
  const [nodeMenu, setNodeMenu] = useState<HistoryNodeMenuState | null>(null);
  const activeNodeID = textValue(displayHistory.active_node_id ?? asRecord(displayHistory.conversation_graph).active_node_id, "");
  const selectedNode =
    nodes.find((node) => node.id === selectedNodeID) ??
    nodes.find((node) => node.id === activeNodeID) ??
    nodes[nodes.length - 1] ??
    null;
  const activeBranch = historyActiveBranch(displayHistory);
  const projectPath = historyProjectPath(displayHistory) || selectedWorktree?.projectFilePath || historyProjectPath(activeHistory);
  const selectedReservation = selectedWorktree
    ? reservations.find((reservation) => textValue(reservation.worktree_ref, "") === selectedWorktree.name || textValue(reservation.worktree_project_path, "") === selectedWorktree.projectFilePath) ?? null
    : null;
  const selectedChildTask = selectedReservation
    ? childTasks.find((task) => textValue(task.reservation_id, "") === textValue(selectedReservation.id, "")) ?? null
    : null;
  const loadingSelectedWorktree = Boolean(selectedWorktree && !selectedWorktreeIsActive && busyAction === `inspect:${selectedWorktree.key}`);

  useEffect(() => {
    if (!selectedWorktreeKey || !worktrees.some((worktree) => worktree.key === selectedWorktreeKey)) {
      setSelectedWorktreeKey(activeWorktreeKey);
      setLoadedWorktree(null);
    }
  }, [activeWorktreeKey, selectedWorktreeKey, worktrees]);

  useEffect(() => {
    setSelectedBranchName(historyActiveBranch(displayHistory));
  }, [displayHistory]);

  useEffect(() => {
    if (nodes.length === 0) {
      setSelectedNodeID("");
      return;
    }
    if (selectedNodeID && nodes.some((node) => node.id === selectedNodeID)) {
      return;
    }
    setSelectedNodeID(activeNodeID || nodes[nodes.length - 1].id);
  }, [activeNodeID, nodes, selectedNodeID]);

  useEffect(() => {
    if (!nodeMenu) {
      return;
    }
    const closeMenu = () => setNodeMenu(null);
    window.addEventListener("click", closeMenu);
    window.addEventListener("blur", closeMenu);
    return () => {
      window.removeEventListener("click", closeMenu);
      window.removeEventListener("blur", closeMenu);
    };
  }, [nodeMenu]);

  const inspectWorktree = async (worktree: HistoryWorktreeRow) => {
    if (!worktree.projectFilePath) {
      setLoadedWorktree(null);
      setStatus("这个工作树还没有可读取的工程文件。");
      return;
    }
    setBusyAction(`inspect:${worktree.key}`);
    setStatus("");
    try {
      const response = await invokeAgent({
        tool: "version.status",
        args: historyToolArgs(activeHistory, { project_path: worktree.projectFilePath }),
        confirmed: false,
        source: "ask_vit_webui_history",
        context: buildHistoryInvokeContext(uiState, activeHistory)
      });
      if (response.error || response.status === "error") {
        throw new Error(response.error || "读取工作树历史失败。");
      }
      const result = asRecord(response.result ?? response.project_history);
      setLoadedWorktree({ key: worktree.key, history: result });
      setStatus(`正在查看工作树：${worktree.label}`);
    } catch (historyError) {
      setLoadedWorktree(null);
      setStatus(historyError instanceof Error ? historyError.message : "读取工作树历史失败。");
    } finally {
      setBusyAction("");
    }
  };

  const selectWorktree = async (worktree: HistoryWorktreeRow) => {
    setSelectedWorktreeKey(worktree.key);
    setSelectedNodeID("");
    setNodeMenu(null);
    if (worktree.active) {
      setLoadedWorktree(null);
      setStatus("");
      return;
    }
    await inspectWorktree(worktree);
  };

  const activateSelectedWorktree = async () => {
    if (!selectedWorktree || selectedWorktree.active || checkoutBlocked) return;
    if (!window.confirm(`打开工作树 ${selectedWorktree.label}？当前 Active Project Plane 将切换。`)) return;
    const args = historyWorktreeCheckoutArgs(selectedWorktree);
    if (Object.keys(args).length === 0) {
      setStatus("这个工作树没有可打开的工程路径。");
      return;
    }
    await runHistoryTool(`worktree-open:${selectedWorktree.key}`, "version.worktree_checkout", args, `已打开工作树 ${selectedWorktree.label}`);
  };

  const runHistoryTool = async (
    actionKey: string,
    tool: string,
    args: JsonRecord,
    successMessage: string,
    options: { checkoutCreatedWorktree?: boolean } = {}
  ) => {
    if (busyAction) {
      return null;
    }
    if (checkoutBlocked && ["version.checkout", "version.node_checkout", "version.worktree_checkout", "version.branch_create"].includes(tool)) {
      setStatus("Agent Turn 正在运行；请先 Stop Turn 再切换工程历史。");
      return null;
    }
    setBusyAction(actionKey);
    setStatus("");
    try {
      const response = await invokeAgent({
        tool,
        args: historyToolArgs(displayHistory, args),
        confirmed: true,
        source: "ask_vit_webui_history",
        context: buildHistoryInvokeContext(uiState, displayHistory)
      });
      if (response.error || response.status === "error") {
        throw new Error(response.error || "Project History action failed");
      }
      if (options.checkoutCreatedWorktree) {
        const createdWorktree = asRecord(response.result);
        const checkoutArgs = historyWorktreeCheckoutArgs(historyWorktreeRowFromRecord(createdWorktree, displayHistory));
        if (Object.keys(checkoutArgs).length > 0) {
          const checkout = await invokeAgent({
            tool: "version.worktree_checkout",
            args: historyToolArgs(displayHistory, checkoutArgs),
            confirmed: true,
            source: "ask_vit_webui_history",
            context: buildHistoryInvokeContext(uiState, displayHistory)
          });
          if (checkout.error || checkout.status === "error") {
            throw new Error(checkout.error || "Worktree created, but checkout failed");
          }
        }
      }
      setStatus(successMessage);
      await onRefresh();
      if (selectedWorktree && !selectedWorktree.active && !options.checkoutCreatedWorktree) {
        await inspectWorktree(selectedWorktree);
      }
      return response;
    } catch (historyError) {
      setStatus(historyError instanceof Error ? historyError.message : "Project History action failed");
      return null;
    } finally {
      setBusyAction("");
    }
  };

  const checkoutBranch = async (branch: HistoryBranchRow) => {
    setSelectedBranchName(branch.name);
    if (!selectedWorktreeIsActive || !branch.name || branch.active) {
      return;
    }
    if (!window.confirm(`切换到分支 ${branch.name}？当前工程会恢复到该分支 HEAD。`)) {
      return;
    }
    await runHistoryTool(`branch:${branch.name}`, "version.checkout", { branch: branch.name }, `已切换到分支 ${branch.name}`);
  };

  const createBranchFromNode = async (node: HistoryTreeNode) => {
    if (!historyNodeCanCreateFrom(node)) {
      setStatus("这个节点还没有可用的回溯哈希。");
      return;
    }
    const name = window.prompt("从此节点新建分支", `branch-${historyShortID(node.commitID)}`);
    const cleanName = historySafeName(name ?? "");
    if (!cleanName) {
      return;
    }
    await runHistoryTool(
      `branch-create:${node.id}`,
      "version.branch_create",
      { name: cleanName, commit_id: node.commitID, from_node_id: node.nodeID },
      `已从节点创建分支 ${cleanName}`
    );
  };

  const createWorktreeFromNode = async (node: HistoryTreeNode) => {
    if (!historyNodeCanCreateFrom(node)) {
      setStatus("这个节点还没有可用的回溯哈希。");
      return;
    }
    const name = window.prompt("从此节点新建工作树", `worktree-${historyShortID(node.commitID)}`);
    const cleanName = historySafeName(name ?? "");
    if (!cleanName) {
      return;
    }
    await runHistoryTool(
      `worktree-create:${node.id}`,
      "version.worktree_create",
      { name: cleanName, commit_id: node.commitID, parent_node_id: node.nodeID },
      `已从节点创建非激活工作树 ${cleanName}`
    );
  };

  const deleteHistoryNode = async (node: HistoryTreeNode) => {
    if (!node.nodeID) {
      setStatus("这个节点没有可删除的节点 ID。");
      return;
    }
    if (!window.confirm(`删除节点 ${historyNodeTitle(node)} 及其子节点？这会修改工程历史树。`)) {
      return;
    }
    await runHistoryTool(`delete:${node.id}`, "version.node_delete", { node_id: node.nodeID }, `已删除节点 ${historyShortID(node.nodeID)}`);
  };

  const openNodeMenu = (event: ReactMouseEvent, node: HistoryTreeNode) => {
    event.preventDefault();
    event.stopPropagation();
    setSelectedNodeID(node.id);
    setNodeMenu({ node, x: event.clientX, y: event.clientY });
  };

  return (
    <div className="pane-body history-pane" onContextMenu={(event) => event.preventDefault()}>
      <section className="history-rail-panel">
        <div className="history-title-line">
          <div>
            <h2>工程历史</h2>
            <span>{lastPathPart(projectPath || textValue(activeHistory.project_label, "未保存工程"))}</span>
          </div>
          <button className="history-icon-button" type="button" title="刷新历史" onClick={() => void onRefresh()} disabled={Boolean(busyAction)}>
            <RefreshCw size={15} />
          </button>
        </div>

        <div className="history-rail-group">
          <div className="history-section-heading">
          <h2>工作树</h2>
          <span>{worktrees.length}</span>
        </div>
          <div className="history-list">
          {worktrees.length === 0 && <EmptyState label="暂无工作树" />}
          {worktrees.map((worktree) => (
            <button
                className={`history-list-row ${worktree.active ? "active" : ""} ${selectedWorktree?.key === worktree.key ? "selected" : ""}`}
              key={worktree.key}
              type="button"
                onClick={() => void selectWorktree(worktree)}
                disabled={Boolean(busyAction) && busyAction !== `inspect:${worktree.key}`}
              title={worktree.projectFilePath || worktree.path || worktree.label}
            >
                <span className="history-row-dot" />
              <span>
                <strong>{worktree.label}</strong>
                  <small>{historyShortID(worktree.commitID) || lastPathPart(worktree.projectFilePath || worktree.path) || "-"}</small>
              </span>
                <em>{worktree.active ? "当前" : selectedWorktree?.key === worktree.key ? "已选中" : "可查看"}</em>
            </button>
          ))}
          </div>
          {selectedWorktree && !selectedWorktree.active && (
            <button className="history-open-worktree" type="button" disabled={Boolean(busyAction) || checkoutBlocked} onClick={() => void activateSelectedWorktree()}>
              {checkoutBlocked ? "运行中不可切换" : `打开 ${selectedWorktree.label}`}
            </button>
          )}
          {selectedReservation && (
            <div className={`history-reservation ${textValue(selectedReservation.status, "")}`}>
              <strong>{textValue(selectedReservation.display_name, "Reservation")}</strong>
              <span>{textValue(selectedReservation.status, "unknown")}{textValue(selectedReservation.disposition, "") ? ` / ${textValue(selectedReservation.disposition, "")}` : ""} · {textValue(selectedReservation.owner_agent_id, "-")}</span>
              <small>{textValue(selectedReservation.purpose, "未记录目的")}</small>
              {selectedChildTask && <small>子任务：{textValue(selectedChildTask.status, "-")} · {textValue(selectedChildTask.agent_id, "-")}</small>}
            </div>
          )}
        </div>

        <div className="history-rail-group">
          <div className="history-section-heading">
          <h2>分支</h2>
          <span>{branches.length}</span>
        </div>
          <div className="history-list">
            {loadingSelectedWorktree && <EmptyState label="正在读取工作树历史..." />}
          {branches.length === 0 && <EmptyState label="暂无分支" />}
          {branches.map((branch) => (
            <button
                className={`history-list-row ${branch.active ? "active" : ""} ${selectedBranchName === branch.name ? "selected" : ""}`}
              key={branch.name}
              type="button"
              onClick={() => void checkoutBranch(branch)}
                disabled={Boolean(busyAction) || checkoutBlocked || !branch.head}
              title={branch.head}
            >
                <span className="history-row-dot" />
              <span>
                <strong>{branch.name}</strong>
                <small>{historyShortID(branch.head) || "-"}</small>
              </span>
                <em>{branch.active ? "当前" : selectedWorktreeIsActive ? "可切换" : "可查看"}</em>
            </button>
          ))}
          </div>
        </div>

        <div className="history-rail-group history-tree-group">
          <div className="history-section-heading">
          <h2>Ask Vit 历史树</h2>
          <span>{nodes.length}</span>
        </div>
          <div className="history-tree-scroll">
            {nodes.length === 0 && <EmptyState label={loadingSelectedWorktree ? "正在读取工作树历史..." : "暂无历史节点"} />}
            {nodes.length > 0 && (
              <div className="history-tree-canvas" style={{ width: treeLayout.width, height: treeLayout.height }}>
                <svg className="history-tree-lines" width={treeLayout.width} height={treeLayout.height} aria-hidden="true">
                  {treeLayout.connections.map((connection) => (
                    <polyline key={connection.id} points={connection.points} />
                  ))}
                </svg>
                {treeLayout.nodes.map((node) => (
                  <button
                    className={`history-node-card ${node.kind} ${node.active ? "active" : ""} ${selectedNode?.id === node.id ? "selected" : ""}`}
                    key={node.id}
                    type="button"
                    style={{ left: node.x, top: node.y }}
                    onClick={() => setSelectedNodeID(node.id)}
                    onContextMenu={(event) => openNodeMenu(event, node)}
                    title={node.summary}
                  >
                    <strong>{historyNodeTitle(node)}</strong>
                    <span>{historyShortID(node.commitID) || node.meta || "-"}</span>
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
        {status && <div className={`history-status ${historyStatusIsError(status) ? "error" : ""}`}>{status}</div>}
      </section>

      {nodeMenu && (
        <div className="history-context-menu" style={{ left: nodeMenu.x, top: nodeMenu.y }} onClick={(event) => event.stopPropagation()}>
          <button type="button" onClick={() => void createBranchFromNode(nodeMenu.node)} disabled={!historyNodeCanCreateFrom(nodeMenu.node) || checkoutBlocked || Boolean(busyAction)}>
            <GitBranch size={14} />
            <span>从此节点新建分支</span>
          </button>
          <button type="button" onClick={() => void createWorktreeFromNode(nodeMenu.node)} disabled={!historyNodeCanCreateFrom(nodeMenu.node) || Boolean(busyAction)}>
            <Plus size={14} />
            <span>新建工作树</span>
          </button>
          <button type="button" className="danger" onClick={() => void deleteHistoryNode(nodeMenu.node)} disabled={!nodeMenu.node.nodeID || Boolean(busyAction)}>
            <X size={14} />
            <span>删除节点</span>
          </button>
        </div>
      )}
    </div>
  );
}

function historyTreeNodes(history: JsonRecord): HistoryTreeNode[] {
  const graph = asRecord(history.conversation_graph);
  const rawNodes = firstArray(graph.nodes).map(asRecord).filter((node) => textValue(node.id, "") && textValue(node.commit_id, ""));
  const activeNodeID = textValue(history.active_node_id ?? graph.active_node_id, "");
  const activeBranch = historyActiveBranch(history);
  const byID = new Map<string, JsonRecord>();
  rawNodes.forEach((node) => byID.set(textValue(node.id, ""), node));
  const childrenByParent = new Map<string, JsonRecord[]>();
  rawNodes.forEach((node) => {
    const parentID = textValue(node.parent_node_id, "");
    if (!parentID || !byID.has(parentID)) {
      return;
    }
    const children = childrenByParent.get(parentID) ?? [];
    children.push(node);
    childrenByParent.set(parentID, children);
  });
  const ordered: Array<Omit<HistoryTreeNode, "index" | "parentIndex">> = [];
  const visited = new Set<string>();
  const appendNode = (node: JsonRecord, depth: number) => {
    const id = textValue(node.id, "");
    if (!id || visited.has(id)) {
      return;
    }
    visited.add(id);
    const kind = textValue(node.kind, "checkpoint").toLowerCase();
    const commitID = textValue(node.commit_id, "");
    const branch = textValue(node.branch, activeBranch);
    ordered.push({
      id,
      nodeID: id,
      parentID: textValue(node.parent_node_id, ""),
      kind,
      label: historyKindLabel(kind),
      meta: historyShortID(commitID),
      summary: historyNodeSummary(node, kind),
      branch,
      commitID,
      createdAt: textValue(node.created_at, ""),
      active: id === activeNodeID,
      depth,
      raw: node
    });
    (childrenByParent.get(id) ?? []).forEach((child) => appendNode(child, depth + 1));
  };
  rawNodes.forEach((node) => {
    const parentID = textValue(node.parent_node_id, "");
    if (!parentID || !byID.has(parentID)) {
      appendNode(node, 0);
    }
  });
  rawNodes.forEach((node) => appendNode(node, 0));
  const indexByID = new Map<string, number>();
  ordered.forEach((node, index) => indexByID.set(node.id, index));
  return ordered.map((node, index) => ({
    ...node,
    index,
    parentIndex: node.parentID ? indexByID.get(node.parentID) ?? -1 : -1
  }));
}

function historyTreeLayout(nodes: HistoryTreeNode[]): HistoryTreeLayout {
  if (nodes.length === 0) {
    return { nodes: [], connections: [], width: 260, height: 160 };
  }
  const cardWidth = 108;
  const cardHeight = 42;
  const rowHeight = 72;
  const laneGap = 92;
  const pad = 8;
  const children = new Map<number, number[]>();
  nodes.forEach((node, index) => {
    if (node.parentIndex < 0 || node.parentIndex >= nodes.length) {
      return;
    }
    const rows = children.get(node.parentIndex) ?? [];
    rows.push(index);
    children.set(node.parentIndex, rows);
  });
  const roots = nodes.map((node, index) => ({ node, index })).filter(({ node }) => node.parentIndex < 0 || node.parentIndex >= nodes.length).map(({ index }) => index);
  const layout = new Map<number, { lane: number; depth: number }>();
  const visiting = new Set<number>();
  let nextLane = 0;
  const assign = (index: number, depth: number): number => {
    const existing = layout.get(index);
    if (existing) {
      return existing.lane;
    }
    if (visiting.has(index)) {
      const lane = nextLane++;
      layout.set(index, { lane, depth });
      return lane;
    }
    visiting.add(index);
    const childRows = children.get(index) ?? [];
    let lane = 0;
    if (childRows.length === 0) {
      lane = nextLane++;
    } else {
      const childLanes = childRows.map((childIndex) => assign(childIndex, depth + 1));
      lane = (childLanes[0] + childLanes[childLanes.length - 1]) / 2;
    }
    visiting.delete(index);
    layout.set(index, { lane, depth });
    return lane;
  };
  (roots.length > 0 ? roots : [0]).forEach((index) => assign(index, 0));
  nodes.forEach((_, index) => {
    if (!layout.has(index)) {
      assign(index, 0);
    }
  });
  const maxDepth = Math.max(0, ...Array.from(layout.values()).map((row) => row.depth));
  const leafCount = Math.max(1, Math.ceil(nextLane));
  const naturalWidth = cardWidth + Math.max(0, leafCount - 1) * laneGap;
  const width = Math.max(260, pad * 2 + naturalWidth);
  const height = Math.max(160, pad * 2 + cardHeight + maxDepth * rowHeight);
  const horizontalOffset = Math.max(pad, (width - naturalWidth) / 2);
  const positioned = nodes.map((node, index) => {
    const row = layout.get(index) ?? { lane: 0, depth: node.depth };
    return {
      ...node,
      x: horizontalOffset + row.lane * laneGap,
      y: pad + row.depth * rowHeight
    };
  });
  const connections = positioned
    .filter((node) => node.parentIndex >= 0 && positioned[node.parentIndex])
    .map((node) => {
      const parent = positioned[node.parentIndex];
      const startX = parent.x + cardWidth / 2;
      const startY = parent.y + cardHeight;
      const endX = node.x + cardWidth / 2;
      const endY = node.y;
      const midY = startY + (endY - startY) * 0.52;
      return {
        id: `${parent.id}:${node.id}`,
        points: `${startX},${startY} ${startX},${midY} ${endX},${midY} ${endX},${endY}`
      };
    });
  return { nodes: positioned, connections, width, height };
}

function historyBranchRows(history: JsonRecord): HistoryBranchRow[] {
  const activeBranch = historyActiveBranch(history);
  const branches = asRecord(history.branches);
  const names = Object.keys(branches).sort((left, right) => left.localeCompare(right));
  if (names.length === 0 && activeBranch) {
    return [{ name: activeBranch, head: textValue(history.head, ""), active: true }];
  }
  return names.map((name) => ({
    name,
    head: textValue(branches[name], ""),
    active: name === activeBranch
  }));
}

function historyWorktreeRows(history: JsonRecord): HistoryWorktreeRow[] {
  const rows: HistoryWorktreeRow[] = [];
  const rootProjectPath = textValue(history.root_project_path, "") || textValue(history.project_path, "");
  if (rootProjectPath) {
    const rootPath = rootProjectPath.replace(/[/\\][^/\\]+$/, "");
    rows.push({
      key: historyWorktreeKey(rootProjectPath, rootPath, "main"),
      name: "",
      label: "主工作树",
      path: rootPath,
      projectFilePath: rootProjectPath,
      commitID: textValue(history.head, ""),
      active: historyActiveWorktree(history) === "main",
      isRoot: true,
      raw: { project_file_path: rootProjectPath, is_root: true }
    });
  }
  const seen = new Set(rows.map((row) => row.key));
  firstArray(history.worktrees).map(asRecord).forEach((row) => {
    const item = historyWorktreeRowFromRecord(row, history);
    if (!item.key || seen.has(item.key)) {
      return;
    }
    seen.add(item.key);
    rows.push(item);
  });
  return rows;
}

function historyWorktreeRowFromRecord(row: JsonRecord, history: JsonRecord): HistoryWorktreeRow {
  const name = textValue(row.name, "");
  const projectFilePath = textValue(row.project_file_path, "");
  const path = textValue(row.path, "");
  const commitID = textValue(row.worktree_head_commit_id ?? row.head ?? row.commit_id ?? row.origin_commit_id, "");
  const isRoot = truthy(row.is_root);
  const label = textValue(row.label, "") || (isRoot ? "主工作树" : name || lastPathPart(projectFilePath || path) || "工作树");
  const key = historyWorktreeKey(projectFilePath, path, isRoot ? "main" : name || label);
  const activeWorktree = historyActiveWorktree(history);
  const active = isRoot ? activeWorktree === "main" : Boolean(name) && name === textValue(history.active_worktree, "");
  return { key, name, label, path, projectFilePath, commitID, active, isRoot, raw: row };
}

function historyWorktreeKey(projectFilePath: string, path: string, name: string): string {
  return (projectFilePath || path || name).replace(/\\/g, "/").trim().toLowerCase();
}

function historyToolArgs(history: JsonRecord, args: JsonRecord): JsonRecord {
  const out: JsonRecord = { ...args };
  if (!textValue(out.project_path, "")) {
    const projectPath = historyProjectPath(history);
    if (projectPath) {
      out.project_path = projectPath;
    }
  }
  return out;
}

function historyWorktreeCheckoutArgs(worktree: HistoryWorktreeRow): JsonRecord {
  const args: JsonRecord = {};
  if (worktree.name && !worktree.isRoot) {
    args.name = worktree.name;
  }
  if (worktree.projectFilePath) {
    args.project_file_path = worktree.projectFilePath;
  }
  return args;
}

function buildHistoryInvokeContext(uiState: AgentUIState | null, sourceHistory?: JsonRecord): JsonRecord {
  const history = sourceHistory && Object.keys(sourceHistory).length > 0 ? sourceHistory : asRecord(uiState?.project_history);
  return {
    project_history: history,
    history_scope_key: historyScopeKeyFromUIState(uiState),
    project_path: historyProjectPath(history),
    root_project_path: textValue(history.root_project_path, ""),
    active_branch: historyActiveBranch(history),
    active_worktree: textValue(history.active_worktree, ""),
    active_node_id: textValue(history.active_node_id, "")
  };
}

function historyProjectPath(history: JsonRecord): string {
  return textValue(history.project_path ?? history.current_project_path ?? history.root_project_path, "");
}

function historyActiveBranch(history: JsonRecord): string {
  if (truthy(history.detached)) {
    return "detached";
  }
  return textValue(history.active_branch, "main");
}

function historyActiveWorktree(history: JsonRecord): string {
  return textValue(history.active_worktree, "") || "main";
}

function historyKindLabel(kind: string): string {
  switch (kind.toLowerCase()) {
    case "ask":
      return "Ask";
    case "vit":
      return "Vit";
    case "branch_marker":
      return "分支点";
    case "checkpoint":
      return "Saved";
    default:
      return kind || "鑺傜偣";
  }
}

function historyNodeSummary(node: JsonRecord, kind: string): string {
  const summary = textValue(node.text_preview ?? node.text ?? node.summary ?? node.message, "");
  if (summary) {
    return summary.length > 92 ? `${summary.slice(0, 89)}...` : summary;
  }
  return `${historyKindLabel(kind)} saved state`;
}

function historyNodeTitle(node: HistoryTreeNode): string {
  const label = node.label || historyKindLabel(node.kind);
  const branch = node.branch && node.branch !== "main" ? ` / ${node.branch}` : "";
  return `${label}${branch}`;
}

function historyNodeCanCreateFrom(node: HistoryTreeNode): boolean {
  return Boolean(node.commitID) && node.kind === "vit";
}

function historyStatusIsError(status: string): boolean {
  const lower = status.toLowerCase();
  return lower.includes("error") || lower.includes("failed") || status.includes("失败") || status.includes("不可用");
}

function historyShortID(value: string): string {
  const clean = value.trim();
  if (!clean || clean === "-") {
    return "";
  }
  return clean.length > 10 ? clean.slice(0, 10) : clean;
}

function historySafeName(value: string): string {
  return value
    .trim()
    .replace(/[\\/:*?"<>|]+/g, "-")
    .replace(/\s+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 64);
}

function historyMessageRoleLabel(role: string): string {
  const clean = role.toLowerCase();
  if (clean === "user" || clean === "ask") {
    return "用户";
  }
  if (clean === "assistant" || clean === "vit") {
    return "Vit";
  }
  return role || "消息";
}

function InfoGrid({ rows }: { rows: Array<[string, string]> }) {
  return (
    <div className="info-grid">
      {rows.map(([label, value]) => (
        <div key={label}>
          <span>{label}</span>
          <strong>{value}</strong>
        </div>
      ))}
    </div>
  );
}

function KeyValue({ label, value }: { label: string; value: string }) {
  return (
    <div className="key-value">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function buildDiagnosticsRows(runtimeStatus: RuntimeStatusResponse | null, uiState: AgentUIState | null): Array<[string, string]> {
  const kernel = asRecord(runtimeStatus?.kernel);
  const shadow = asRecord(runtimeStatus?.shadow);
  const project = asRecord(uiState?.project);
  const history = asRecord(uiState?.project_history);
  const selectedPlugin = asRecord(uiState?.selected_plugin);
  const uiContext = asRecord(uiState?.ui_context);
  return [
    ["Agent", runtimeStatus?.service ?? "未连接"],
    ["PID", textValue(runtimeStatus?.pid, "-")],
    ["内核", truthy(kernel.connected) || textValue(kernel.status, "").toLowerCase() === "ok" ? "已连接" : textValue(kernel.error ?? kernel.status, "未知")],
    ["影子状态", truthy(shadow.initialized) || truthy(project.initialized) ? "已初始化" : "未初始化"],
    ["轨道", textValue(project.user_track_count ?? project.track_count ?? uiState?.tracks?.length, "0")],
    ["工程", lastPathPart(textValue(history.project_path ?? project.project_path, "未保存工程"))],
    ["历史作用域", historyScopeKeyFromUIState(uiState) || "-"],
    ["当前节点", textValue(history.active_node_id, "-")],
    ["工作树", textValue(history.active_worktree, "main")],
    ["选中插件", textValue(selectedPlugin.plugin_name ?? uiContext.selected_plugin_name, "-")],
    ["插件 ID", textValue(selectedPlugin.plugin_id ?? uiContext.selected_plugin_id, "-")],
    ["状态版本", textValue(shadow.last_delta_seq ?? project.last_delta_seq, "-")],
    ["版本缺口", textValue(shadow.delta_seq_gaps ?? project.delta_seq_gaps, "0")]
  ];
}

function buildDiagnosticsPayload(
  runtimeStatus: RuntimeStatusResponse | null,
  uiState: AgentUIState | null,
  configResponse: AgentConfigResponse | null
): JsonRecord {
  const project = asRecord(uiState?.project);
  const history = asRecord(uiState?.project_history);
  const uiContext = asRecord(uiState?.ui_context);
  return {
    runtime: runtimeStatus,
    history_scope_key: historyScopeKeyFromUIState(uiState),
    project: {
      project_path: project.project_path,
      track_count: project.track_count,
      user_track_count: project.user_track_count,
      graph_revision: project.graph_revision,
      last_delta_seq: project.last_delta_seq,
      delta_seq_gaps: project.delta_seq_gaps
    },
    project_history: {
      project_path: history.project_path,
      root_project_path: history.root_project_path,
      active_branch: history.active_branch,
      active_node_id: history.active_node_id,
      active_worktree: history.active_worktree,
      head: history.head,
      draft: history.draft,
      initialized: history.initialized,
      commit_count: history.commit_count,
      worktree_count: history.worktree_count,
      warnings: history.warnings
    },
    selected_track: uiState?.selected_track ?? null,
    selected_plugin: uiState?.selected_plugin ?? null,
    ui_context: uiContext,
    configPath: configResponse?.path,
    hasApiKey: configResponse?.hasApiKey,
    routeApiKeys: configResponse?.routeApiKeys
  };
}

function EmptyState({ label }: { label: string }) {
  return <div className="empty-state">{label}</div>;
}

type ArtifactContextVariant = "compact" | "message";

function ArtifactContextCard({
  artifact,
  variant = "message",
  onSelect,
  onRemove
}: {
  artifact: ArtifactSummary;
  variant?: ArtifactContextVariant;
  onSelect: () => void;
  onRemove?: () => void;
}) {
  const presentation = artifactPresentation(artifact);
  const isImage = artifactIsImage(artifact);
  const handleKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onSelect();
    }
  };
  return (
    <div
      className={`artifact-context-card ${variant} ${presentation.kind}`}
      role="button"
      tabIndex={0}
      title={artifactAddress(artifact)}
      onClick={onSelect}
      onKeyDown={handleKeyDown}
    >
      <div className="artifact-context-thumb" aria-hidden="true">
        {isImage ? (
          <img src={artifactFileURL(artifact.id)} alt="" loading="lazy" />
        ) : (
          <presentation.Icon size={variant === "compact" ? 16 : 18} />
        )}
      </div>
      <div className="artifact-context-main">
        <span className="artifact-context-type">{presentation.label}</span>
        <strong>{artifactLabel(artifact)}</strong>
        <small>{presentation.detail}</small>
      </div>
      {onRemove && (
        <button
          className="artifact-context-remove"
          type="button"
          title="Remove"
          onClick={(event) => {
            event.stopPropagation();
            onRemove();
          }}
        >
          <X size={14} />
        </button>
      )}
    </div>
  );
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

function confirmationFallbackActionsFromResponse(response: ChatResponse): JsonRecord[] {
  const planID = textValue(response.plan_id, "");
  if (!planID || !responseNeedsConfirmationFallback(response)) {
    return [];
  }
  const body = textValue(response.reply ?? response.message, "这个操作需要你确认后才会执行。");
  const preview = textValue(response.preview, "");
  const workflowData = asRecord(response.workflow_data);
  const proposalPresentation = asRecord(response.proposal_presentation ?? workflowData.proposal_presentation);
  const isCapabilityProposal = textValue(response.workflow, "").toLowerCase() === "capability_runtime_v1" && Object.keys(proposalPresentation).length > 0;
  const payload: JsonRecord = {
    plan_id: planID,
    preview,
    commands: response.commands ?? []
  };
  if (isCapabilityProposal) {
    ["session_id", "capability_id", "proposal_id", "proposal_revision", "action_set_hash", "project_cut_hash", "approval_mode"].forEach((key) => {
      if (workflowData[key] !== undefined) {
        payload[key] = workflowData[key];
      }
    });
    payload.proposal_presentation = proposalPresentation;
  }
  return [
    {
      _ui_source: "interaction",
      _synthetic_confirmation: true,
      id: `confirmation_${planID}`,
      kind: isCapabilityProposal ? "proposal_approval" : "confirmation",
      type: isCapabilityProposal ? "proposal_approval" : "confirmation",
      source: "vit_agent",
      title: isCapabilityProposal ? textValue(proposalPresentation.title, "方案等待确认") : "需要确认",
      body,
      status: "waiting_for_user",
      plan_id: planID,
      workflow: response.workflow,
      payload,
      data: payload,
      command: {
        preview,
        commands: response.commands ?? []
      },
      actions: [
        { id: "approve", label: "确认执行", style: "primary", recommended: true },
        { id: "cancel", label: "取消", style: "secondary" }
      ]
    }
  ];
}

function responseNeedsConfirmationFallback(response: ChatResponse): boolean {
  const status = textValue(response.goal_status ?? response.status, "").toLowerCase();
  return Boolean(response.needs_confirmation) || status.includes("waiting_confirmation") || status.includes("needs_confirmation") || status.includes("confirm");
}

function isSyntheticConfirmationInteraction(interaction: JsonRecord): boolean {
  return truthy(interaction._synthetic_confirmation);
}

function isCurrentPluginTargetText(value: string): boolean {
  const normalized = value.trim().toLowerCase();
  return (
    ["当前", "当前插件", "选中插件", "使用当前插件", "current", "current plugin", "selected plugin"].includes(normalized) ||
    normalized.includes("当前") ||
    normalized.includes("选中") ||
    normalized.includes("已选中") ||
    normalized.includes("selected") ||
    normalized.includes("current")
  );
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

function mixBoardActionFromPayload(...values: unknown[]): JsonRecord | null {
  for (const value of values) {
    const record = asRecord(value);
    const workflowData = asRecord(record.workflow_data);
    const result = asRecord(record.result);
    const interactionRequests = firstArray(record.interaction_requests, workflowData.interaction_requests, result.interaction_requests).map(asRecord);
    const hasMixBoardInteraction = interactionRequests.some(isMixBoardAction);
    const mixSession = firstNonEmptyRecord(asRecord(record.mix_session), asRecord(workflowData.mix_session), asRecord(result.mix_session));
    const mixObservation = firstNonEmptyRecord(asRecord(record.mix_observation), asRecord(workflowData.mix_observation), asRecord(result.mix_observation));
    if (Object.keys(mixObservation).length > 0 || hasMixBoardInteraction) {
      const sessionID = textValue(mixSession.mix_session_id ?? asRecord(mixObservation.observation).mix_session_id ?? mixObservation.mix_session_id, "");
      return {
        _ui_source: "mix_board",
        kind: "mix_board",
        type: "mix_board",
        id: `mix_board_${stableIDPart(sessionID || textValue(asRecord(mixObservation.observation).observation_id ?? mixObservation.observation_id, "latest"))}`,
        status: textValue(mixObservation.status ?? asRecord(mixObservation.observation).status ?? mixSession.state, ""),
        mix_session: mixSession,
        mix_observation: mixObservation,
        payload: {
          mix_session: mixSession,
          mix_observation: mixObservation
        }
      };
    }
  }
  return null;
}

function mixBoardActionWithInteractions(mixBoardAction: JsonRecord | null, interactions: JsonRecord[]): JsonRecord | null {
  if (!mixBoardAction) {
    return null;
  }
  const match = interactions.find((interaction) => isSameMixBoardSession(interaction, mixBoardAction, mixBoardActionSessionID(mixBoardAction))) ??
    interactions.find(isMixBoardAction);
  if (!match) {
    return mixBoardAction;
  }
  const payload = interactionPayload(match);
  const mixPayload = interactionPayload(mixBoardAction);
  return {
    ...match,
    ...mixBoardAction,
    id: textValue(match.id ?? match.interaction_id, "") || textValue(mixBoardAction.id, ""),
    interaction_id: textValue(match.id ?? match.interaction_id, ""),
    _ui_source: "mix_board",
    kind: "mix_board",
    type: "mix_board",
    title: textValue(match.title ?? mixBoardAction.title, "MixBoard"),
    body: textValue(match.body ?? mixBoardAction.body, ""),
    actions: firstArray(match.actions),
    payload: {
      ...payload,
      ...mixPayload,
      request_context: asRecord(payload.request_context ?? mixPayload.request_context)
    },
    data: {
      ...asRecord(match.data),
      ...asRecord(mixBoardAction.data),
      ...mixPayload
    }
  };
}

function firstNonEmptyRecord(...records: JsonRecord[]): JsonRecord {
  return records.find((record) => Object.keys(record).length > 0) ?? {};
}


function interactionActionsFrom(...values: unknown[]): JsonRecord[] {
  const seen = new Set<string>();
  const out: JsonRecord[] = [];
  values.forEach((value) => {
    firstArray(value).forEach((raw, index) => {
      const record = asRecord(raw);
      if (Object.keys(record).length === 0) {
        return;
      }
      const id = textValue(record.id ?? record.interaction_id, "");
      const key = id || `${textValue(record.type ?? record.kind ?? record.title, "interaction")}-${index}`;
      if (seen.has(key)) {
        return;
      }
      seen.add(key);
      out.push({ ...record, _ui_source: "interaction" });
    });
  });
  return out;
}

function artifactSummariesFrom(...values: unknown[]): ArtifactSummary[] {
  const seen = new Set<string>();
  const out: ArtifactSummary[] = [];
  values.forEach((value) => {
    firstArray(value).forEach((raw) => {
      const record = asRecord(raw);
      const id = textValue(record.id, "");
      if (!id || seen.has(id)) {
        return;
      }
      seen.add(id);
      out.push({ ...record, id } as ArtifactSummary);
    });
  });
  return out;
}

function historyScopeKeyFromUIState(uiState: AgentUIState | null): string {
  if (!uiState) {
    return "";
  }
  const projectHistory = asRecord(uiState.project_history);
  const project = asRecord(uiState.project);
  const uiContext = asRecord(uiState.ui_context);
  const projectPath = textValue(
    projectHistory.project_path ?? projectHistory.current_project_path ?? project.project_path ?? uiContext.project_path ?? uiContext.current_project_path,
    ""
  );
  const rootProjectPath = textValue(projectHistory.root_project_path, "");
  const stateDir = textValue(projectHistory.state_dir, "");
  const historyDir = textValue(projectHistory.history_dir, "");
  const activeWorktree = textValue(projectHistory.active_worktree, "");
  const activeBranch = textValue(projectHistory.active_branch, "");
  const projectIdentity = projectPath || rootProjectPath || textValue(project.project_path, "") || "unsaved";
  return [
    projectIdentity,
    rootProjectPath || "root",
    stateDir || historyDir || "state",
    activeWorktree || "main",
    activeBranch || "main"
  ].join("::");
}

function messagesOrIntro(messages: ChatMessage[]): ChatMessage[] {
  if (messages.length > 0) {
    return messages;
  }
  return [{ ...initialMessage, id: uniqueID("intro"), createdAt: Date.now() }];
}

function hasMeaningfulChatMessages(messages: ChatMessage[]): boolean {
  return messages.some((message) => {
    if (message.id === "intro" && message.role === "assistant") {
      return false;
    }
    return textValue(message.content, "").trim() !== "" || (message.actions?.length ?? 0) > 0 || (message.artifacts?.length ?? 0) > 0;
  });
}

function createConversationID(): string {
  return `webui_${Date.now().toString(36)}`;
}

function conversationScopedIDStorageKey(scope: string): string {
  return `${conversationScopedIDStoragePrefix}:${stableIDPart(scope)}`;
}

function scopedConversationRuntimeKey(scope: string, conversationID: string): string {
  return `${stableIDPart(scope)}:${stableIDPart(conversationID || "latest")}`;
}

function loadStoredScopedConversationID(scope: string): string {
  if (typeof window === "undefined" || !scope) {
    return "";
  }
  try {
    return window.localStorage.getItem(conversationScopedIDStorageKey(scope))?.trim() ?? "";
  } catch {
    return "";
  }
}

function saveStoredScopedConversationID(scope: string, conversationID: string): void {
  if (typeof window === "undefined" || !scope || !conversationID) {
    return;
  }
  try {
    window.localStorage.setItem(conversationScopedIDStorageKey(scope), conversationID);
  } catch {
    // Scoped local history is best-effort; the backend Project History remains authoritative.
  }
}

function conversationMessagesStorageKey(conversationID: string, scope = ""): string {
  const id = stableIDPart(conversationID || "latest");
  const scoped = scope ? `:${stableIDPart(scope)}` : "";
  return `${conversationMessagesStoragePrefix}:${id}${scoped}`;
}

function loadStoredConversationMessages(conversationID: string, scope = ""): ChatMessage[] {
  if (typeof window === "undefined" || !conversationID || !scope) {
    return [];
  }
  try {
    const raw = window.localStorage.getItem(conversationMessagesStorageKey(conversationID, scope));
    if (!raw) {
      return [];
    }
    const parsed = JSON.parse(raw);
    return firstArray(asRecord(parsed).messages ?? parsed).map(sanitizeStoredChatMessage).filter(Boolean) as ChatMessage[];
  } catch {
    // Ignore corrupt local history and let Project History hydrate what it can.
    return [];
  }
}

function saveStoredConversationMessages(conversationID: string, scope: string, messages: ChatMessage[]): void {
  if (typeof window === "undefined" || !conversationID || !scope) {
    return;
  }
  const rows = durableMessagesForStorage(messages)
    .filter((message) => message.id !== "intro")
    .slice(-120)
    .map(sanitizeChatMessageForStorage);
  if (rows.length === 0) {
    return;
  }
  const payload = JSON.stringify({
    schema_version: "ask_vit_conversation_messages.v2",
    conversation_id: conversationID,
    scope,
    saved_at: new Date().toISOString(),
    messages: rows
  });
  try {
    window.localStorage.setItem(conversationMessagesStorageKey(conversationID, scope), payload);
  } catch {
    // Local history is best-effort; Project History remains the durable backend record.
  }
}

function sanitizeChatMessageForStorage(message: ChatMessage): ChatMessage {
  return durableMessage({
    id: textValue(message.id, uniqueID("stored_msg")),
    source_id: textValue(message.source_id, ""),
    role: message.role,
    content: textValue(message.content, ""),
    mode: message.mode,
    artifacts: message.artifacts ?? [],
    actions: (message.actions ?? []).map(asRecord).filter((action) => Object.keys(action).length > 0),
    createdAt: Number.isFinite(message.createdAt) ? message.createdAt : Date.now(),
    status: message.status,
    lifecycle: message.lifecycle,
    persistence: message.persistence,
    message_kind: message.message_kind,
    turn_id: message.turn_id,
    logical_message_id: message.logical_message_id,
    supersedes: message.supersedes
  });
}

function sanitizeStoredChatMessage(value: unknown): ChatMessage | null {
  const row = asRecord(value);
  const roleText = textValue(row.role, "assistant");
  const role: ChatMessage["role"] = roleText === "user" || roleText === "system" ? roleText : "assistant";
  const content = textValue(row.content, "");
  const actions = firstArray(row.actions).map(asRecord).filter((action) => Object.keys(action).length > 0);
  const artifacts = artifactSummariesFrom(row.artifacts);
  if (!content && actions.length === 0 && artifacts.length === 0) {
    return null;
  }
  const createdAt = Number(row.createdAt ?? row.created_at);
  const candidate: ChatMessage = {
    id: textValue(row.id, uniqueID("stored_msg")),
    source_id: textValue(row.source_id, ""),
    role,
    content,
    mode: textValue(row.mode, "") as AgentMode,
    artifacts,
    actions,
    createdAt: Number.isFinite(createdAt) ? createdAt : Date.now(),
    status: chatMessageStatusFromStored(row.status),
    lifecycle: textValue(row.lifecycle, "") as ChatMessage["lifecycle"],
    persistence: textValue(row.persistence, "") as ChatMessage["persistence"],
    message_kind: textValue(row.message_kind, "") as ChatMessage["message_kind"],
    turn_id: textValue(row.turn_id, ""),
    logical_message_id: textValue(row.logical_message_id, ""),
    supersedes: firstArray(row.supersedes).map((item) => textValue(item, "")).filter(Boolean)
  };
  if (isLegacyTransientMessage(candidate) || !isDurableMessage(candidate)) {
    return null;
  }
  return durableMessage(candidate);
}

function chatMessageStatusFromStored(value: unknown): ChatMessage["status"] {
  const status = textValue(value, "sent").toLowerCase();
  if (status === "pending" || status === "error") {
    return status;
  }
  return "sent";
}

function historyMessagesFromUIState(uiState: AgentUIState | null): ChatMessage[] {
  const projectHistory = asRecord(uiState?.project_history);
  const rows = firstArray(projectHistory.conversation_messages);
  const artifacts = uiState?.artifacts ?? [];
  const messages: ChatMessage[] = [];
  rows.forEach((raw, index) => {
    const row = asRecord(raw);
    const content = textValue(row.content ?? row.text ?? row.text_preview, "");
    if (!content) {
      return;
    }
    const sourceID = textValue(row.node_id ?? row.id, `history_${index}`);
    const roleText = textValue(row.role ?? row.kind, "assistant").toLowerCase();
    const role: ChatMessage["role"] = roleText === "user" || roleText === "ask" ? "user" : "assistant";
    if (role === "assistant" && isDisposableConfirmationPromptText(content)) {
      return;
    }
    const created = Date.parse(textValue(row.created_at, ""));
    const messageData = asRecord(row.message_data);
    const workflowData = asRecord(messageData.workflow_data);
    const messageInteractions = interactionActionsFrom(messageData.interaction_requests);
    const historyResponse: ChatResponse = {
      reply: content,
      needs_confirmation: truthy(messageData.needs_confirmation),
      plan_id: textValue(messageData.plan_id, ""),
      preview: textValue(messageData.preview, ""),
      workflow: textValue(messageData.workflow, ""),
      workflow_data: workflowData,
      proposal_presentation: asRecord(messageData.proposal_presentation ?? workflowData.proposal_presentation),
      interaction_requests: firstArray(messageData.interaction_requests).map(asRecord),
      message_kind: textValue(row.message_kind, "") as ChatResponse["message_kind"],
      lifecycle: "durable",
      persistence: "project_history"
    };
    const historyFallbackActions = messageInteractions.some(isPendingInteractionAction)
      ? []
      : confirmationFallbackActionsFromResponse(historyResponse);
    const actions =
      role === "assistant"
        ? [
            ...messageInteractions,
            ...historyFallbackActions,
            ...firstArray(row.project_result_cards)
              .map(asRecord)
              .filter((card) => Object.keys(card).length > 0)
              .map((card, cardIndex) => ({
                ...card,
                _ui_source: "project_result",
                kind: "project_result",
                type: "project_result",
                id: textValue(card.id, `history_project_result_${sourceID}_${cardIndex}`)
              }))
          ]
        : [];
    const messageArtifacts = artifactSummariesFrom(row.artifacts);
    const protocol = historyMessageProtocol(row, role);
    const historyMessage = durableMessage({
      id: `history_${sourceID}`,
      role,
      content,
      artifacts: messageArtifacts,
      actions,
      createdAt: Number.isFinite(created) ? created : Date.now() + index,
      status: "sent",
      ...protocol
    }, { persistence: "project_history" });
    if (isDurableMessage(historyMessage)) {
      messages.push(historyMessage);
    }
  });
  return resolveCompletedTurnProposals(resolveSupersededMessages(messages));
}

function removeDisposableConfirmationPromptMessages(messages: ChatMessage[]): ChatMessage[] {
  return messages.filter((message) => !isDisposableConfirmationPromptMessage(message));
}

function mergeChatMessages(current: ChatMessage[], incoming: ChatMessage[]): ChatMessage[] {
  if (incoming.length === 0) {
    return current;
  }
  const base = current.length === 1 && current[0].id === "intro" ? [] : current;
  return resolveCompletedTurnProposals(mergeMessageCollections(base, incoming, chatMessageKeys, mergeChatMessage));
}

function mergeAssistantMessageIntoChat(current: ChatMessage[], incoming: ChatMessage): ChatMessage[] {
  return messageMixBoardAction(incoming) ? upsertPersistentMixBoardMessage(current, incoming) : mergeChatMessages(current, [incoming]);
}

function upsertPersistentMixBoardMessage(current: ChatMessage[], incoming: ChatMessage): ChatMessage[] {
  const incomingAction = messageMixBoardAction(incoming);
  if (!incomingAction) {
    return mergeChatMessages(current, [incoming]);
  }
  const incomingSessionID = mixBoardActionSessionID(incomingAction);
  let replaced = false;
  let inserted = false;
  const next = current.flatMap((message) => {
    const actions = (message.actions ?? []).map(asRecord);
    if (actions.length === 0) {
      return [message];
    }
    const mixActions = actions.filter(isMixBoardAction);
    if (mixActions.length === 0) {
      return [message];
    }
    const sessionMatches = mixActions.some((action) => isSameMixBoardSession(action, incomingAction, incomingSessionID));
    const shouldReplaceHere = !replaced && (sessionMatches || !incomingSessionID);
    if (!shouldReplaceHere) {
      const remaining = actions.filter((action) => !isMixBoardAction(action));
      if (remaining.length === 0 && !message.artifacts?.length && !message.content.trim()) {
        return [];
      }
      return [{ ...message, actions: remaining }];
    }
    replaced = true;
    inserted = true;
    const mergedActions = actions.map((action) => (isMixBoardAction(action) ? incomingAction : action));
    return [{
      ...message,
      content: incoming.content || message.content,
      mode: incoming.mode || message.mode,
      artifacts: mergeArtifacts(message.artifacts ?? [], incoming.artifacts ?? []),
      actions: mergedActions,
      status: incoming.status
    }];
  });
  if (inserted) {
    return next;
  }
  const stripped = next.filter((message) => {
    const actions = (message.actions ?? []).map(asRecord);
    return !actions.some(isMixBoardAction);
  });
  return mergeChatMessages(stripped, [incoming]);
}

function messageMixBoardAction(message: ChatMessage): JsonRecord | null {
  return firstArray(message.actions).map(asRecord).find(isMixBoardAction) ?? null;
}

function isSameMixBoardSession(action: JsonRecord, incomingAction: JsonRecord, incomingSessionID: string): boolean {
  if (!isMixBoardAction(action)) {
    return false;
  }
  const existingSessionID = mixBoardActionSessionID(action);
  if (incomingSessionID && existingSessionID) {
    return incomingSessionID === existingSessionID;
  }
  return actionRenderID(action) === actionRenderID(incomingAction);
}

function mixBoardActionSessionID(action: JsonRecord): string {
  const payload = interactionPayload(action);
  const session = asRecord(payload.mix_session ?? action.mix_session);
  const observationEnvelope = asRecord(payload.mix_observation ?? action.mix_observation);
  const observation = asRecord(observationEnvelope.observation);
  const mixboard = asRecord(observationEnvelope.mixboard);
  return textValue(
    session.mix_session_id ??
      observation.mix_session_id ??
      mixboard.mix_session_id ??
      payload.mix_session_id ??
      action.mix_session_id,
    ""
  );
}

function mergeChatMessage(existing: ChatMessage, incoming: ChatMessage): ChatMessage {
  return {
    ...incoming,
    ...existing,
    source_id: existing.source_id || incoming.source_id,
    lifecycle: existing.lifecycle === "durable" || incoming.lifecycle === "durable" ? "durable" : existing.lifecycle ?? incoming.lifecycle,
    persistence: existing.persistence === "project_history" || incoming.persistence === "project_history"
      ? "project_history"
      : existing.persistence ?? incoming.persistence,
    message_kind: inferMessageKind(existing) === "proposal" || inferMessageKind(incoming) === "proposal"
      ? "proposal"
      : existing.message_kind ?? incoming.message_kind,
    turn_id: existing.turn_id || incoming.turn_id,
    logical_message_id: existing.logical_message_id || incoming.logical_message_id,
    supersedes: Array.from(new Set([...(existing.supersedes ?? []), ...(incoming.supersedes ?? [])])),
    artifacts: mergeArtifacts(existing.artifacts ?? [], incoming.artifacts ?? []),
    actions: mergeMessageActions(existing.actions ?? [], incoming.actions ?? []),
    createdAt: Math.min(existing.createdAt, incoming.createdAt),
    status: existing.status === "error" || incoming.status === "error" ? "error" : existing.status ?? incoming.status
  };
}

function mergeMessageActions(current: JsonRecord[], incoming: JsonRecord[]): JsonRecord[] {
  if (current.length === 0) {
    return incoming;
  }
  if (incoming.length === 0) {
    return current;
  }
  const byKey = new Map<string, JsonRecord>();
  const order: string[] = [];
  [...current, ...incoming].forEach((action, index) => {
    const key = actionMergeKey(action, index);
    const existing = byKey.get(key);
    if (existing) {
      byKey.set(key, mergeActionRecord(existing, action));
      return;
    }
    byKey.set(key, action);
    order.push(key);
  });
  return order.map((key) => byKey.get(key)).filter((action): action is JsonRecord => Boolean(action));
}

function mergeActionRecord(existing: JsonRecord, incoming: JsonRecord): JsonRecord {
  if (isComposerInteraction(existing) && !isComposerInteraction(incoming)) {
    return existing;
  }
  return {
    ...incoming,
    ...existing,
    actions: firstArray(existing.actions).length > 0 ? existing.actions : incoming.actions
  };
}

function actionMergeKey(action: JsonRecord, index: number): string {
  if (isCapabilityProposalInteraction(action)) {
    const proposalID = proposalActionIdentity(action);
    if (proposalID) {
      return `proposal:${proposalID}`;
    }
  }
  return actionRenderID(action) || stableActionIdentity(action) || `${textValue(action._ui_source, "action")}:${index}`;
}

function stableActionIdentity(action: JsonRecord): string {
  const source = textValue(action._ui_source, "action");
  const payload = asRecord(action.result ?? action.payload);
  const tool = textValue(action.tool ?? payload.tool, "");
  const command = textValue(action.command_name ?? action.cmd ?? action.command ?? payload.command_name ?? payload.cmd ?? payload.command, "");
  const status = textValue(action.status ?? payload.status, "");
  const objectID = textValue(
    action.tool_call_id ??
      action.agent_action_id ??
      payload.tool_call_id ??
      payload.agent_action_id ??
      payload.track_id ??
      payload.clip_id ??
      payload.plugin_id ??
      payload.new_clip_id ??
      payload.created_clip_id,
    ""
  );
  if (objectID) {
    return `${source}:${tool || command || "action"}:${objectID}:${status}`;
  }
  const preview = textValue(
    action.preview ??
      action.body ??
      action.title ??
      payload.message ??
      payload.summary ??
      payload.preview,
    ""
  ).slice(0, 120);
  if (tool || command || status || preview) {
    return `${source}:${tool || command || "action"}:${status}:${preview}`;
  }
  return "";
}

function chatMessageKeys(message: ChatMessage): string[] {
  const actions = (message.actions ?? []).map(asRecord);
  const actionKeys = actions
    .filter(isMessageMergeKeyAction)
    .map((action) => actionRenderID(action))
    .filter(Boolean);
  const keys: string[] = messageProtocolIdentityKeys(message);
  if (message.id) {
    keys.push(`id:${message.id}`);
  }
  if (actionKeys.length > 0) {
    keys.push(`actions:${message.role}:${actionKeys.join("|")}`);
  }
  if (!actions.some(isProjectResultAction) && (actionKeys.length === 0 || inferMessageKind(message) === "proposal")) {
    keys.push(`text:${message.role}:${message.content}`);
  }
  return Array.from(new Set(keys));
}

function isMessageMergeKeyAction(action: JsonRecord): boolean {
  if (isProjectResultAction(action)) {
    return false;
  }
  const source = textValue(action._ui_source, "");
  return source === "interaction" || isComposerInteraction(action) || isConfirmationAction(action);
}

function defaultSettingsConfig(): EngineConfig {
  return {
    baseUrl: "",
    apiKey: "",
    defaultModel: "",
    multimodalRoutes: Object.fromEntries(
      multimodalRouteMeta.map((meta, index) => [
        meta.key,
        {
          enabled: meta.key === "text",
          provider: "default",
          baseUrl: "",
          apiKey: "",
          model: "",
          priority: (index + 1) * 10,
          fallback: ""
        } satisfies MultimodalRouteConfig
      ])
    ),
    browser: {
      enabled: false,
      mode: "webview2_companion",
      profilePath: "",
      rememberSession: true
    }
  };
}

function mergeSettingsConfig(config?: EngineConfig): EngineConfig {
  const defaults = defaultSettingsConfig();
  return {
    ...defaults,
    ...(config ?? {}),
    multimodalRoutes: {
      ...(defaults.multimodalRoutes ?? {}),
      ...(config?.multimodalRoutes ?? {})
    },
    browser: {
      ...(defaults.browser ?? {}),
      ...(config?.browser ?? {})
    },
    apiKey: ""
  };
}

function selectedPluginContextFromUIState(uiState: AgentUIState | null): { track_id: string; plugin_id: string; plugin_name: string } {
  const selectedPlugin = asRecord(uiState?.selected_plugin);
  const uiContext = asRecord(uiState?.ui_context);
  const rack = asRecord(uiState?.plugin_rack);
  const rackTrack = asRecord(rack.track);
  const selectedTrack = asRecord(uiState?.selected_track);
  const plugins = firstArray(rack.plugins, rack.items, rack.chain, rack.rack)
    .map(asRecord)
    .filter((item) => Object.keys(item).length > 0 && !isSystemRackPluginRecord(item));
  const rackSelection = plugins.find((plugin) => truthy(plugin.selected) || truthy(plugin.current) || truthy(plugin.active) || truthy(plugin.focused)) ??
    (plugins.length === 1 ? plugins[0] : {});
  return {
    track_id: textValue(selectedPlugin.track_id ?? uiContext.selected_plugin_track_id ?? uiContext.selected_track_id ?? rackTrack.track_id ?? rackTrack.id ?? selectedTrack.track_id ?? selectedTrack.id, ""),
    plugin_id: textValue(selectedPlugin.plugin_id ?? uiContext.selected_plugin_id ?? rackSelection.plugin_id ?? rackSelection.id ?? rackSelection.plugin_item_id ?? rackSelection.item_id, ""),
    plugin_name: textValue(selectedPlugin.plugin_name ?? uiContext.selected_plugin_name ?? rackSelection.plugin_name ?? rackSelection.name ?? rackSelection.title, "")
  };
}

function buildChatContext(mode: AgentMode, activeFocus: FocusMode, uiState: AgentUIState | null, artifacts: ArtifactSummary[], macroRefs: MacroControl[] = [], authorityMode: AuthorityMode = "manual_confirmation"): JsonRecord {
  const selectedTrack = asRecord(uiState?.selected_track);
  const uiContext = asRecord(uiState?.ui_context);
  const selectedTrackID = selectedTrackIDFromUIState(uiState);
  const selectedTrackName = textValue(selectedTrack.name ?? selectedTrack.track_name ?? uiContext.selected_track_name, "");
  const selectedClipID = selectedClipIDFromUIState(uiState);
  const selectedClipIDs = Array.from(selectedClipIDsFromUIState(uiState));
  const selectedClipTrackID = textValue(uiContext.selected_clip_track_id ?? selectedTrackID, "");
  const pianoRollFocusClipID = textValue(uiContext.piano_roll_focus_clip_id, "");
  const pianoRollFocusTrackID = textValue(uiContext.piano_roll_focus_track_id, "");
  const playheadSeconds = uiContext.playhead_seconds ?? uiContext.current_playhead_seconds ?? uiContext.transport_position_seconds;
  const currentSelection = compactChatContextRecord({
    selected_track_id: selectedTrackID,
    selected_track_name: selectedTrackName,
    selected_clip_id: selectedClipID,
    selected_clip_ids: selectedClipIDs,
    selected_clip_track_id: selectedClipTrackID,
    piano_roll_focus_clip_id: pianoRollFocusClipID,
    piano_roll_focus_track_id: pianoRollFocusTrackID,
    playhead_seconds: playheadSeconds,
    current_playhead_seconds: uiContext.current_playhead_seconds ?? playheadSeconds,
    transport_position_seconds: uiContext.transport_position_seconds ?? playheadSeconds
  });
  const mergedUIContext = compactChatContextRecord({
    ...uiContext,
    ...currentSelection
  });
  const pluginTarget = selectedPluginContextFromUIState(uiState);
  const project = asRecord(uiState?.project);
  const projectHistory = asRecord(uiState?.project_history);
  const scope = artifactScopeMetadata(uiState);
  const macroControls = macroControlsFromUIState(uiState);
  return compactChatContextRecord({
    agent_mode: mode,
    ...authorityContext(authorityMode),
    active_focus: activeFocus,
    history_scope_key: scope.history_scope_key,
    media_scope_key: scope.media_scope_key,
    project_path: textValue(projectHistory.project_path ?? projectHistory.current_project_path ?? project.project_path, ""),
    root_project_path: textValue(projectHistory.root_project_path, ""),
    active_branch: textValue(projectHistory.active_branch, ""),
    active_node_id: textValue(projectHistory.active_node_id, ""),
    active_worktree: textValue(projectHistory.active_worktree, ""),
    selected_track_id: selectedTrackID,
    selected_track_name: selectedTrackName,
    selected_clip_id: selectedClipID,
    selected_clip_ids: selectedClipIDs,
    selected_clip_track_id: selectedClipTrackID,
    piano_roll_focus_clip_id: pianoRollFocusClipID,
    piano_roll_focus_track_id: pianoRollFocusTrackID,
    playhead_seconds: playheadSeconds,
    current_playhead_seconds: currentSelection.current_playhead_seconds,
    transport_position_seconds: currentSelection.transport_position_seconds,
    current_selection: currentSelection,
    ui_context: mergedUIContext,
    selected_plugin_track_id: pluginTarget.track_id,
    selected_plugin_id: pluginTarget.plugin_id,
    selected_plugin_name: pluginTarget.plugin_name,
    macro_refs: macroRefs.map(compactMacroForContext),
    available_macro_controls: macroControls.slice(0, 12).map(compactMacroForContext),
    artifacts: artifacts.map((artifact) => ({
      id: artifact.id,
      kind: artifact.kind,
      title: artifactLabel(artifact),
      mime: artifact.mime
    }))
  });
}

function compactChatContextRecord(record: JsonRecord): JsonRecord {
  return Object.fromEntries(
    Object.entries(record).filter(([, value]) => {
      if (Array.isArray(value)) {
        return value.length > 0;
      }
      if (value && typeof value === "object") {
        return Object.keys(value as JsonRecord).length > 0;
      }
      return textValue(value, "") !== "";
    })
  );
}

function compactMacroForContext(macro: MacroControl): JsonRecord {
  const normalized = normalizeMacroControl(asRecord(macro));
  return {
    macro_id: macroControlID(normalized),
    name: macroControlLabel(normalized),
    track_id: normalized.track_id,
    plugin_id: normalized.plugin_id,
    role: normalized.role,
    control: normalized.control,
    value: macroControlValue(normalized),
    binding_count: normalized.bindings?.length ?? 0,
    bindings: (normalized.bindings ?? []).slice(0, 6).map((binding) => ({
      control: binding.control,
      track_id: binding.track_id,
      plugin_id: binding.plugin_id,
      param_id: binding.param_id,
      param_name: binding.param_name,
      target_min: binding.target_min,
      target_max: binding.target_max,
      enabled: binding.enabled !== false
    }))
  };
}

function uploadMetadata(conversationID: string, uiState: AgentUIState | null): {
  conversation_id: string;
  goal_id?: string;
  run_id?: string;
  project_path?: string;
  root_project_path?: string;
  active_worktree?: string;
  active_branch?: string;
  active_node_id?: string;
  history_scope_key?: string;
  media_scope_key?: string;
} {
  const goal = asRecord(uiState?.goal);
  const plan = asRecord(uiState?.agent_plan);
  return {
    conversation_id: conversationID,
    goal_id: textValue(goal.goal_id ?? plan.goal_id, ""),
    run_id: textValue(goal.run_id ?? plan.run_id, ""),
    ...artifactScopeMetadata(uiState)
  };
}

function artifactScopeMetadata(uiState: AgentUIState | null): {
  project_path?: string;
  root_project_path?: string;
  active_worktree?: string;
  active_branch?: string;
  active_node_id?: string;
  history_scope_key?: string;
  media_scope_key?: string;
} {
  const project = asRecord(uiState?.project);
  const projectHistory = asRecord(uiState?.project_history);
  const uiContext = asRecord(uiState?.ui_context);
  const projectPath = textValue(
    projectHistory.project_path ?? projectHistory.current_project_path ?? project.project_path ?? uiContext.project_path ?? uiContext.current_project_path,
    ""
  );
  const rootProjectPath = textValue(projectHistory.root_project_path, "");
  const activeWorktree = textValue(projectHistory.active_worktree, "");
  const activeBranch = textValue(projectHistory.active_branch, "");
  const activeNodeID = textValue(projectHistory.active_node_id, "");
  const historyScopeKey = historyScopeKeyFromUIState(uiState);
  return compactMetadata({
    project_path: projectPath,
    root_project_path: rootProjectPath,
    active_worktree: activeWorktree,
    active_branch: activeBranch,
    active_node_id: activeNodeID,
    history_scope_key: historyScopeKey,
    media_scope_key: mediaScopeKeyFromParts(projectPath, rootProjectPath, activeWorktree, activeBranch, historyScopeKey)
  });
}

function mediaScopeKeyFromUIState(uiState: AgentUIState | null): string {
  return artifactScopeMetadata(uiState).media_scope_key ?? "";
}

function mediaScopeKeyFromParts(projectPath: string, rootProjectPath: string, activeWorktree: string, activeBranch: string, historyScopeKey: string): string {
  if (rootProjectPath) {
    return ["root", rootProjectPath, activeWorktree || "main", activeBranch || "main"].join("::");
  }
  if (projectPath) {
    return `project::${projectPath}`;
  }
  if (historyScopeKey) {
    return `history::${historyScopeKey}`;
  }
  return "";
}

function compactMetadata<T extends Record<string, string>>(metadata: T): Partial<T> {
  return Object.fromEntries(Object.entries(metadata).filter(([, value]) => value.trim() !== "")) as Partial<T>;
}

function renderArtifactPreview(artifact: ArtifactSummary | Artifact, options: { imageZoom?: number } = {}) {
  const kind = (artifact.kind ?? "").toLowerCase();
  const mime = (artifact.mime ?? "").toLowerCase();
  const fileURL = artifactFileURL(artifact.id);
  if (kind === "audio" || mime.startsWith("audio/")) {
    return <audio className="media-player" controls src={fileURL} />;
  }
  if (kind === "video" || mime.startsWith("video/")) {
    return <video className="media-player video" controls src={fileURL} />;
  }
  if (artifactIsImage(artifact)) {
    const zoom = clampNumber(options.imageZoom ?? 1, 0.35, 4);
    const imageStyle = zoom <= 1
      ? { maxWidth: "100%", maxHeight: "100%" }
      : { width: `${zoom * 100}%`, maxWidth: "none", maxHeight: "none" };
    return (
      <img
        className="image-preview"
        src={fileURL}
        alt={artifactLabel(artifact)}
        draggable={false}
        onDragStart={(event) => event.preventDefault()}
        style={imageStyle}
      />
    );
  }
  if ("text" in artifact && artifact.text) {
    return <pre className="text-preview">{artifact.text}</pre>;
  }
  if (mime === "application/pdf" || mime === "text/html" || mime.startsWith("text/")) {
    return <iframe className="document-frame" title={artifactLabel(artifact)} src={fileURL} />;
  }
  return (
    <div className="file-preview">
      <File size={36} />
      <span>{artifact.mime || artifact.kind || "artifact"}</span>
    </div>
  );
}

function artifactIsImage(artifact: ArtifactSummary): boolean {
  const kind = (artifact.kind ?? "").toLowerCase();
  const mime = (artifact.mime ?? "").toLowerCase();
  return kind === "image" || mime.startsWith("image/");
}

function artifactPresentation(artifact: ArtifactSummary): { kind: string; label: string; detail: string; Icon: LucideIcon } {
  const kind = (artifact.kind ?? "").toLowerCase();
  const mime = (artifact.mime ?? "").toLowerCase();
  const extension = artifactFileExtension(artifact);
  const source = artifactSourceLabel(artifact);
  const size = formatBytes(artifact.size_bytes);
  const detail = [source, size].filter((part) => part !== "").join(" / ");
  if (artifactIsImage(artifact)) {
    return { kind: "image", label: "Image", detail, Icon: FileImage };
  }
  if (kind === "audio" || mime.startsWith("audio/")) {
    return { kind: "audio", label: "Audio", detail, Icon: FileAudio };
  }
  if (kind === "video" || mime.startsWith("video/")) {
    return { kind: "video", label: "Video", detail, Icon: Film };
  }
  if (kind === "midi" || extension === ".mid" || extension === ".midi") {
    return { kind: "midi", label: "MIDI", detail, Icon: Activity };
  }
  if (artifactLooksLikeAdapter(artifact)) {
    return { kind: "adapter", label: "Adapter", detail, Icon: Plug };
  }
  if (kind === "web_page" || textValue(artifact.url, "") !== "") {
    return { kind: "web", label: "Web page", detail, Icon: Globe };
  }
  if (kind === "document" || kind === "text" || mime.includes("pdf") || mime.startsWith("text/")) {
    return { kind: "document", label: mime.includes("pdf") ? "PDF" : "Document", detail, Icon: FileText };
  }
  if (kind === "archive" || [".zip", ".rar", ".7z", ".tar", ".gz"].includes(extension)) {
    return { kind: "archive", label: "Archive", detail, Icon: Archive };
  }
  return { kind: "file", label: kind || "File", detail, Icon: File };
}

function artifactIcon(artifact: ArtifactSummary) {
  const Icon = artifactPresentation(artifact).Icon;
  return <Icon size={16} />;
}

function artifactLabel(artifact: ArtifactSummary): string {
  return (
    textValue(artifact.title, "") ||
    textValue(artifact.url, "") ||
    lastPathPart(textValue(artifact.path, "")) ||
    artifact.id
  );
}

function artifactAddress(artifact: ArtifactSummary): string {
  return textValue(artifact.path, "") || textValue(artifact.url, "") || artifactFileURL(artifact.id);
}

function artifactMetaLine(artifact: ArtifactSummary): string {
  return [artifactPresentation(artifact).label, artifact.status ?? "ready", formatBytes(artifact.size_bytes)]
    .filter((part) => textValue(part, "") !== "")
    .join(" 路 ");
}

function artifactSourceLabel(artifact: ArtifactSummary): string {
  const source = textValue(artifact.source, "").toLowerCase();
  if (source.includes("download")) {
    return "Downloads";
  }
  if (source.includes("upload")) {
    return "Uploaded";
  }
  if (source.includes("browser")) {
    return "Browser";
  }
  if (source.includes("plugin") || source.includes("grabber")) {
    return "Plugin";
  }
  return source || textValue(artifact.status, "");
}

function artifactFileExtension(artifact: ArtifactSummary): string {
  const value = textValue(artifact.path, "") || textValue(artifact.url, "") || textValue(artifact.title, "");
  const clean = value.split(/[?#]/)[0] ?? "";
  const name = lastPathPart(clean).toLowerCase();
  const dot = name.lastIndexOf(".");
  return dot >= 0 ? name.slice(dot) : "";
}

function artifactLooksLikeAdapter(artifact: ArtifactSummary): boolean {
  const metadata = asRecord(artifact.metadata);
  const fields = [
    artifact.kind,
    artifact.source,
    artifact.path,
    artifact.title,
    artifact.mime,
    metadata.resource_origin,
    metadata.profile_path,
    metadata.skill_path
  ]
    .map((value) => textValue(value, "").toLowerCase())
    .join(" ");
  return (
    fields.includes("adapter") ||
    fields.includes("skill") ||
    fields.includes("profile") ||
    fields.includes("plugin_grabber") ||
    fields.includes("capability_pack")
  );
}

function mergeArtifacts(...groups: ArtifactSummary[][]): ArtifactSummary[] {
  const byID = new Map<string, ArtifactSummary>();
  groups.flat().forEach((artifact) => {
    if (artifact.id) {
      byID.set(artifact.id, { ...byID.get(artifact.id), ...artifact });
    }
  });
  return Array.from(byID.values()).sort((left, right) => textValue(right.created_at, "").localeCompare(textValue(left.created_at, "")));
}

function asRecord(value: unknown): JsonRecord {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as JsonRecord) : {};
}

function firstArray(...values: unknown[]): unknown[] {
  for (const value of values) {
    if (Array.isArray(value)) {
      return value;
    }
  }
  return [];
}

function textValue(value: unknown, fallback: string): string {
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (trimmed === "<nil>" || trimmed === "null" || trimmed === "undefined") {
      return fallback;
    }
    return trimmed || fallback;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  return fallback;
}

function compactJSON(value: unknown): string {
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

function parseJSONRecord(value: unknown): JsonRecord {
  const text = textValue(value, "");
  if (!text) {
    return {};
  }
  try {
    return asRecord(JSON.parse(text));
  } catch {
    return {};
  }
}

function stableIDPart(value: string): string {
  const clean = value.trim().replace(/[^a-zA-Z0-9_-]+/g, "_").replace(/^_+|_+$/g, "");
  return clean.slice(0, 96) || stableTextHash(value);
}

function stableTextHash(value: string): string {
  let hash = 2166136261;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return (hash >>> 0).toString(36);
}

function clampNumber(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function defaultComposerInteractionHeight(): number {
  if (typeof window === "undefined") {
    return 560;
  }
  const saved = Number(window.localStorage?.getItem("ask_vit_composer_interaction_height"));
  if (Number.isFinite(saved) && saved > 0) {
    return clampComposerInteractionHeight(saved);
  }
  return clampComposerInteractionHeight(window.innerHeight * 0.74);
}

function clampComposerInteractionHeight(value: number): number {
  if (typeof window === "undefined") {
    return clampNumber(value, 340, 880);
  }
  return clampNumber(value, 320, Math.max(420, window.innerHeight - 124));
}

function truthy(value: unknown): boolean {
  return value === true || value === "true" || value === 1 || value === "1";
}

function formatBytes(value: unknown): string {
  const bytes = typeof value === "number" ? value : 0;
  if (bytes <= 0) {
    return "-";
  }
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  if (bytes < 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

function lastPathPart(path: string): string {
  const normalized = path.replace(/\\/g, "/");
  const parts = normalized.split("/").filter(Boolean);
  return parts[parts.length - 1] || path || "Vit Project";
}

function uniqueID(prefix: string): string {
  return `${prefix}_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
}

function currentConversationTitle(uiState: AgentUIState | null, messages: ChatMessage[]): string {
  const history = asRecord(uiState?.project_history);
  const graph = asRecord(history.conversation_graph);
  const nodes = firstArray(graph.nodes).map(asRecord);
  const activeNodeID = textValue(history.active_node_id ?? graph.active_node_id, "");
  const byID = new Map(nodes.map((node) => [textValue(node.id, ""), node]));
  const activePath: JsonRecord[] = [];
  const visited = new Set<string>();
  let cursor = activeNodeID ? byID.get(activeNodeID) : undefined;
  while (cursor) {
    const id = textValue(cursor.id, "");
    if (!id || visited.has(id)) {
      break;
    }
    visited.add(id);
    activePath.unshift(cursor);
    cursor = byID.get(textValue(cursor.parent_node_id, ""));
  }
  const latestTaskAsk = [...activePath].reverse().find((node) => {
    if (textValue(node.kind, "").toLowerCase() !== "ask") {
      return false;
    }
    return conversationTitleCandidate(textValue(node.text_preview ?? node.text ?? node.message, ""));
  });
  const latestTaskText = latestTaskAsk ? textValue(latestTaskAsk.text_preview ?? latestTaskAsk.text ?? latestTaskAsk.message, "") : "";
  if (latestTaskText) {
    return compactConversationTitle(latestTaskText);
  }
  const localTaskMessage = [...messages].reverse().find((message) => message.role === "user" && conversationTitleCandidate(textValue(message.content, "")));
  return localTaskMessage ? compactConversationTitle(localTaskMessage.content) : "新对话";
}

function conversationTitleCandidate(value: string): boolean {
  const clean = value.replace(/\s+/g, " ").trim().toLowerCase();
  if (clean.length < 3) {
    return false;
  }
  return !/^(可以|可以执行|执行|确认|确定|同意|继续|好的|好|ok|okay|yes|取消|不用了)[。.!！\s]*$/i.test(clean);
}

function compactConversationTitle(value: string): string {
  const clean = value.replace(/\s+/g, " ").replace(/^[#>*`\-\s]+/, "").trim();
  if (!clean) {
    return "新对话";
  }
  return clean.length > 28 ? `${clean.slice(0, 27)}…` : clean;
}

function panelWidthStorageKey(side: "left" | "right", panel: string): string {
  return `${panelWidthStoragePrefix}_${side}_${panel}`;
}

function loadPanelWidth(side: "left" | "right", panel: string, fallback: number, minimum: number): number {
  if (typeof window === "undefined") {
    return fallback;
  }
  try {
    const stored = Number(window.localStorage.getItem(panelWidthStorageKey(side, panel)));
    return Number.isFinite(stored) && stored >= minimum ? stored : fallback;
  } catch {
    return fallback;
  }
}

function savePanelWidth(side: "left" | "right", panel: string, width: number): void {
  try {
    window.localStorage.setItem(panelWidthStorageKey(side, panel), String(Math.round(width)));
  } catch {
    // Panel sizing is best-effort and must never block the workbench.
  }
}

function loadRightPanelWidths(): Record<WorkbenchTab, number> {
  return {
    media: loadPanelWidth("right", "media", defaultRightPanelWidth, minRightPanelWidth),
    macro: loadPanelWidth("right", "macro", defaultRightPanelWidth, minRightPanelWidth),
    history: loadPanelWidth("right", "history", defaultRightPanelWidth, minRightPanelWidth)
  };
}

function interactionActionID(interactionID: string, actionID: string): string {
  return `${interactionID}:${actionID}`;
}

export default App;
