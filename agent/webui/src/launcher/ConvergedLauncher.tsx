/*
 * CONVERGE-SHELL-1（387）：收敛壳 ConvergedLauncher（launcher surface 新门面）。
 *
 * 组成（卡面 + 施工图 UI-CONVERGE-RECON-1 §4.2 锚 3–5）：
 *  1. 桥面/数据流平移：LauncherSurface 的 greeting/recents 数据流、UI-BRIDGE-1
 *     directive 自动执行、SHELL-2 页内治理三键卡、烟测接缝（URL launcher_smoke=1）
 *     原样平移——桥协议五 type 零新增零改名（launcherBridge.ts 零改动）；
 *  2. 对话流复用：chat 域组件 import 即用（T9-PRE-1B-1/386 出域后可用）——
 *     文本消息经 MessageStream 渲染（LauncherThreadItem 文本段 → ChatMessage 映射，
 *     轨迹/台账/试听等 MainApp 专有面按空态 stub），输入区经 Composer 挂载；
 *     不复用 MainApp 实例。launcher 专有条目（结果卡/directive/治理卡）仍是本壳
 *     的渲染块，按线程序与 MessageStream 文本段交错（segmentThread）——
 *     「收敛切换瞬间完成交接」的呈现层落点（施工图 §7.2 双轨）；
 *  3. v5 令牌接入：根类 v5-design + v5-converged-launcher；tokens/components/
 *     launcher css 复用，壳专有装修在 converged.css（磁带卡/对话流覆盖/输入区
 *     作用域化）；
 *  4. 366 磁带卡落位（用户裁定 b 案）：recents 工程条目本身=磁带盒造型整卡
 *     （TapeCardFace：壳/窗/双带轮/底标结构节点，工程名/路径排布进卡结构；
 *     氛围色=工程名 FNV-1a 哈希 tapeAtmos 沿用）；对话流建议卡工程位同形态
 *     （compact 档）；TapeBadge 小尺寸档保留给 SPEC-V5 §7.3.1 场用位清单其余位。
 *  5. LAUNCHER-RECENTS-CLICK-1（383）：recents 磁带卡整卡可点=发起该工程 open
 *     （同对话流结果卡/「打开…」open 链；agent 卡 exists=false 置灰档）；
 *     烟测接缝扩展 launcher_smoke_followup=recents_click[:N]（生产零行为）。
 *
 * 冻结令牌（磁带卡）：main-window-v3/SPEC.md §3 逐字（TAPE_INK/WIN/HUB/ACC/TINT），
 * 带量公式 d=r*(0.34+0.5*fill)、毂比 0.32r、壳填充 rgba(c1,0.62) 逐字保留——
 * 数值镜像自 components/ProjectBadge.tsx（365），令牌定义收敛在 converged.css。
 *
 * fixture 接缝：props 传入 ConvergedLauncherFixtureOverrides 时完全离线（gallery /
 * render-smoke 使用），真实端点零请求、零桥调用；不传即 live 模式。烟测接缝与
 * LauncherSurface 同语义（生产 URL 无 launcher_smoke 参数 → 全零行为）。
 */

import { useCallback, useEffect, useId, useMemo, useRef, useState } from "react";
import type { CSSProperties, FormEvent, RefObject } from "react";
import {
  cardsToEntries,
  fetchLauncherGreeting,
  fetchLauncherRecentProjects,
  sendLauncherChat,
  type LauncherChatResult,
  type LauncherDirective,
  type LauncherGreeting,
  type LauncherProjectCard
} from "../api/launcher";
import {
  buildDirectNewDirective,
  cardPendingPlaceholder,
  createLauncherConversationID,
  directivePlaceholder,
  drawerCountLabel,
  greetingDisplayText,
  LAUNCHER_AGENT_UNAVAILABLE_TEXT,
  LAUNCHER_BRIDGE_UNAVAILABLE_TEXT,
  LAUNCHER_BRIDGE_WAITING_TEXT,
  LAUNCHER_GOVERNANCE_CANCEL_TEXT,
  LAUNCHER_GOVERNANCE_SAVE_TEXT,
  LAUNCHER_GOVERNANCE_SWITCH_TEXT,
  LAUNCHER_RECENTS_ERROR_TEXT,
  LAUNCHER_SUGGESTIONS,
  launcherDirectiveResultText,
  launcherDirectiveResultRetryable,
  launcherDisplayFileName,
  launcherGovernanceAnsweredText,
  launcherGovernanceHeadline,
  type DirectivePlaceholderView,
  type LauncherDirectiveState,
  type LauncherGreetingState,
  type LauncherGovernanceView
} from "./launcherData";
import {
  createLauncherBridgeRequestID,
  isLauncherBridgeAvailable,
  postLauncherChooseFile,
  postLauncherDirective,
  postLauncherGovernanceChoice,
  postLauncherResultAck,
  postLauncherSurfaceReady,
  subscribeGovernanceRequests,
  subscribeLauncherResults,
  type LauncherBridgeResult,
  type LauncherGovernanceChoice
} from "./launcherBridge";
// 366 氛围色哈希沿用（365 移植源：tape_atmosphere.gd 确定性移植；本壳只消费
// tapeAtmos，令牌数值在 converged.css 镜像）。
import { tapeAtmos } from "../components/ProjectBadge";
// ── 对话流复用（386 出域组件，import 即用；不复用 MainApp 实例）──
import { MessageStream } from "../chat/MessageStream";
import { Composer } from "../chat/Composer";
import { emptyTrajectoryState } from "../trajectory";
import { emptyTurnEventMetaMap } from "../trace/turnEventMeta";
import { emptyRoundStepMap } from "../trace/roundSteps";
import { emptyAuditionState } from "../audition";
import type { AgentInvokeResponse, ChatMessage } from "../types";
import type { DawInvoke } from "../daw/parse";
import "../styles/tokens.css";
import "../components/components.css";
import "../styles/launcher.css";
import "./converged.css";
import { NativeFocusHint, WindowLaunchFeedback, isWindowLaunchBusy, useWindowPresentation } from "../project/loading/windowPresentation";

/** 会话线程条目（directive 携带原始指令供桥执行；fixture 线程不带 directive）。
 * SHELL-2：governance=宿主页内治理请求（跟随发起方——对话发起的工程切换在页内确认）。
 * 与 LauncherSurface.LauncherThreadItem 同构（退役迁移前两壳并存）。 */
export type ConvergedThreadItem =
  | { kind: "assistant"; text: string }
  | { kind: "user"; text: string }
  | { kind: "cards"; cards: LauncherProjectCard[] }
  | { kind: "directive"; view: DirectivePlaceholderView; directive?: LauncherDirective }
  | { kind: "governance"; view: LauncherGovernanceView; answered?: LauncherGovernanceChoice };

/** 桥执行块（单活跃执行，对应宿主 _launcher_directive_in_flight 单飞语义）。 */
interface BridgeExecState {
  requestId: string;
  action: string;
  state: LauncherDirectiveState;
  message: string;
}

export interface ConvergedLauncherFixtureOverrides {
  /** fixture greeting（离线注入）。 */
  greeting?: LauncherGreeting;
  /** fixture：greeting 拉取失败文案（agent 不可用态样例）。 */
  greetingErrorText?: string;
  /** fixture 抽屉态与条目。 */
  drawerState?: "loading" | "ready" | "error" | "empty";
  drawerEntries?: LauncherProjectCard[];
  drawerErrorText?: string;
  /** fixture 初始线程。 */
  thread?: ConvergedThreadItem[];
  /** fixture 输入框预填（仅展示）。 */
  initialInput?: string;
}

export interface ConvergedLauncherProps {
  /** gallery / 渲染烟测 fixture 模式（离线，不出网）。 */
  fixture?: ConvergedLauncherFixtureOverrides;
  /** 撑满视口（live surface 用）。 */
  fill?: boolean;
  /** 工程管理视图：工程仍在宿主打开，不是退出/解绑回执。 */
  activeProjectName?: string;
  onReturnProject?: () => void;
  onOpenSettings?: () => void;
}

type DrawerState = "loading" | "ready" | "error" | "empty";

/** 映射 ChatMessage 的单调时刻基线（只服务 trace 渲染计划的稳定性，不落盘）。 */
const CONV_MSG_TIME_BASE = 1700000000000;

/** MessageStream 专有面的空态/中性 stub（launcher 卷零轨迹/零台账/零试听/零宏）。 */
const stubTrajectory = emptyTrajectoryState();
const stubTurnEventMeta = emptyTurnEventMetaMap();
const stubRoundSteps = emptyRoundStepMap();
const stubAudition = emptyAuditionState();
const stubInvoke: DawInvoke = async () => ({}) as AgentInvokeResponse;

/** 宿主回执 status → 桥执行态（未知一律 failed，不静默）。 */
function mapResultState(status: string): LauncherDirectiveState {
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

/** 烟测 followup 模式（LAUNCHER-RECENTS-CLICK-1 扩展 recents_click 档）。 */
export type LauncherSmokeFollowup =
  | "message"
  | "choose_file"
  | { kind: "recents_click"; index: number };

/** 烟测 URL 参数（生产 URL 无 launcher_smoke → 全零行为）。 */
function parseSmokeParams(): {
  enabled: boolean;
  messages: string[];
  delayMs: number;
  followup: LauncherSmokeFollowup;
  governance: LauncherGovernanceChoice | "";
} {
  const params = new URLSearchParams(window.location.search);
  const enabled = params.get("launcher_smoke") === "1";
  const rawMessage = params.get("launcher_smoke_message") ?? "";
  const messages = enabled
    ? rawMessage
        .split("||")
        .map((part) => part.trim())
        .filter((part) => part.length > 0)
    : [];
  const delayRaw = Number.parseInt(params.get("launcher_smoke_followup_delay_ms") ?? "8000", 10);
  const delayMs = Number.isFinite(delayRaw) && delayRaw > 0 ? delayRaw : 8000;
  // followup：choose_file=回执后下一段改发桥 choose_file 请求（dock 降级腿）；
  // recents_click[:N]=greeting+recents 就绪且消息轮收尾后，延迟点磁带卡 N（默认 0，
  // LAUNCHER-RECENTS-CLICK-1 列表可点真栈腿——经同一 openRecentProject 链）。
  const rawFollowup = params.get("launcher_smoke_followup") ?? "";
  let followup: LauncherSmokeFollowup = "message";
  if (rawFollowup === "choose_file") {
    followup = "choose_file";
  } else if (/^recents_click(:\d+)?$/.test(rawFollowup)) {
    const indexPart = rawFollowup.split(":")[1];
    const parsedIndex = indexPart === undefined ? 0 : Number.parseInt(indexPart, 10);
    followup = { kind: "recents_click", index: Number.isFinite(parsedIndex) && parsedIndex >= 0 ? parsedIndex : 0 };
  }
  // SHELL-2：治理卡自动应答（save|switch|cancel；出现即经真实桥路径回投）。
  const rawGovernance = (params.get("launcher_smoke_governance") ?? "").toLowerCase();
  const governance =
    rawGovernance === "save" || rawGovernance === "switch" || rawGovernance === "cancel"
      ? (rawGovernance as LauncherGovernanceChoice)
      : "";
  return { enabled, messages, delayMs, followup, governance };
}

// ── 线程分段：文本段（→ MessageStream）与 launcher 专有块按线程序交错 ──

type ThreadSegment =
  | { kind: "messages"; key: string; messages: ChatMessage[] }
  | { kind: "cards"; key: string; cards: LauncherProjectCard[] }
  | { kind: "directive"; key: string; view: DirectivePlaceholderView; directive?: LauncherDirective }
  | { kind: "governance"; key: string; view: LauncherGovernanceView; answered?: LauncherGovernanceChoice };

/** 线程 → 渲染段（连续 user/assistant 文本合并为一段 MessageStream 消息组）。 */
function segmentThread(thread: ConvergedThreadItem[]): ThreadSegment[] {
  const segments: ThreadSegment[] = [];
  let textRun: Array<{ kind: "user" | "assistant"; text: string }> = [];
  const flush = () => {
    if (textRun.length === 0) return;
    const base = segments.length;
    segments.push({
      kind: "messages",
      key: `seg-messages-${base}`,
      messages: textRun.map((item, index) => {
        const message: ChatMessage = {
          id: `conv-${base}-${index}`,
          role: item.kind === "user" ? "user" : "assistant",
          content: item.text,
          createdAt: CONV_MSG_TIME_BASE + base * 1000 + index,
          status: "sent"
        };
        return message;
      })
    });
    textRun = [];
  };
  thread.forEach((item) => {
    if (item.kind === "user" || item.kind === "assistant") {
      textRun.push({ kind: item.kind, text: item.text });
      return;
    }
    flush();
    if (item.kind === "cards") {
      segments.push({ kind: "cards", key: `seg-cards-${segments.length}`, cards: item.cards });
    } else if (item.kind === "directive") {
      segments.push({
        kind: "directive",
        key: `seg-directive-${segments.length}`,
        view: item.view,
        directive: item.directive
      });
    } else {
      segments.push({
        kind: "governance",
        key: `seg-governance-${segments.length}`,
        view: item.view,
        answered: item.answered
      });
    }
  });
  flush();
  return segments;
}

/**
 * 366 磁带卡面板（壳/窗/双带轮/底标结构节点；工程名/路径排布进卡结构）。
 * 结构节点锚点：data-tape-shell / data-tape-window / data-tape-reel（每卡三类
 * 节点齐备，双带轮）；data-tape-label=底标文本位（主标=工程名，副标=路径）。
 * 氛围色=工程名 FNV-1a 哈希（tapeAtmos 沿用）；无工程名=中性灰壳（冻结源 null
 * 氛围语义）。带量公式/毂比/壳填充与冻结源逐字同源。
 */
export function TapeCardFace({
  name,
  filePath,
  compact = false
}: {
  name?: string;
  filePath?: string;
  compact?: boolean;
}) {
  const display = (name ?? "").trim() || launcherDisplayFileName(filePath ?? "");
  const atmos = display ? tapeAtmos(display) : null;
  const shellFill = atmos
    ? `rgba(${atmos.c1[0]},${atmos.c1[1]},${atmos.c1[2]},0.62)`
    : "var(--tape-tint)";
  // 带量（冻结公式 d=r*(0.34+0.5*fill)，盘径/环径比=0.34+0.5*fill；无氛围=满盘）。
  const discL = Math.round((atmos ? 0.34 + 0.5 * atmos.L : 1) * 100);
  const discR = Math.round((atmos ? 0.34 + 0.5 * atmos.R : 1) * 100);
  const shellClass = compact ? "v5-tape-card-shell v5-tape-card-shell--compact" : "v5-tape-card-shell";
  const shellStyle = { "--tape-shell-fill": shellFill } as CSSProperties;
  return (
    <span className={shellClass} data-tape-shell data-graphic="tape" style={shellStyle}>
      <span className="v5-tape-card-window" data-tape-window data-graphic="tape">
        <span className="v5-tape-card-reel" data-tape-reel data-tape-side="l" data-graphic="tape">
          <span className="v5-tape-card-reel-disc" style={{ width: `${discL}%`, height: `${discL}%` }} />
          <span className="v5-tape-card-reel-hub" />
        </span>
        <span className="v5-tape-card-reel" data-tape-reel data-tape-side="r" data-graphic="tape">
          <span className="v5-tape-card-reel-disc" style={{ width: `${discR}%`, height: `${discR}%` }} />
          <span className="v5-tape-card-reel-hub" />
        </span>
      </span>
      <span className="v5-tape-card-label" data-tape-label>
        <span className="v5-tape-card-name">{display}</span>
        {filePath ? <span className="v5-tape-card-path">{filePath}</span> : null}
      </span>
    </span>
  );
}

export function ConvergedLauncher({ fixture, fill = false, activeProjectName, onReturnProject, onOpenSettings }: ConvergedLauncherProps) {
  const presentation = useWindowPresentation();
  const launchBusy = isWindowLaunchBusy(presentation.state);
  const sidebarId = useId();
  const fixtureGreeting = fixture?.greeting;
  const fixtureThread = fixture?.thread;
  const [greetingState, setGreetingState] = useState<LauncherGreetingState>(
    fixture ? (fixture.greetingErrorText ? "error" : "ready") : "loading"
  );
  const [greeting, setGreeting] = useState<LauncherGreeting | null>(fixtureGreeting ?? null);
  const [greetingError, setGreetingError] = useState(fixture?.greetingErrorText ?? "");
  const [drawerState, setDrawerState] = useState<DrawerState>(
    fixture ? (fixture.drawerState ?? (fixture.drawerEntries?.length ? "ready" : "empty")) : "loading"
  );
  const [drawerEntries, setDrawerEntries] = useState<LauncherProjectCard[]>(fixture?.drawerEntries ?? []);
  const [drawerError, setDrawerError] = useState(fixture?.drawerErrorText ?? "");
  const [thread, setThread] = useState<ConvergedThreadItem[]>(fixtureThread ?? []);
  const [input, setInput] = useState(fixture?.initialInput ?? "");
  const [busy, setBusy] = useState(false);
  const [conversationID] = useState(() => createLauncherConversationID());
  const loadRunRef = useRef(0);
  // Composer 挂载 ref（chat/Composer 契约 prop；launcher 卷无浮层定位依赖）。
  const composerRef = useRef<HTMLFormElement | null>(null);

  // ── UI-BRIDGE-1：桥可用性（一次性判定）+ 烟测参数（URL 一次性解析）+ 执行态 ──
  const bridgeAvailable = useMemo(() => !fixture && isLauncherBridgeAvailable(), [fixture]);
  // fixture 模式（node 渲染/无 window）完全离线：烟测参数不解析。
  const smokeParams = useMemo(
    () =>
      fixture
        ? { enabled: false, messages: [] as string[], delayMs: 8000, followup: "message" as LauncherSmokeFollowup, governance: "" as LauncherGovernanceChoice | "" }
        : parseSmokeParams(),
    [fixture]
  );
  const [bridgeExec, setBridgeExec] = useState<BridgeExecState | null>(null);
  const lastDirectiveRef = useRef<LauncherDirective | null>(null);
  const isSmokeRef = useRef(false);
  const smokeDelayRef = useRef(8000);
  const smokeFollowupRef = useRef<LauncherSmokeFollowup>("message");
  const smokeGovernanceRef = useRef<LauncherGovernanceChoice | "">("");
  const smokeQueueRef = useRef<string[]>([]);
  const smokeStartedRef = useRef(false);
  const smokeFollowupDoneRef = useRef(false);
  const smokeNextRef = useRef<() => void>(() => {});
  useEffect(() => {
    isSmokeRef.current = smokeParams.enabled;
    smokeDelayRef.current = smokeParams.delayMs;
    smokeFollowupRef.current = smokeParams.followup;
    smokeGovernanceRef.current = smokeParams.governance;
  }, [smokeParams]);

  // ── SHELL-2：页内治理三键卡（跟随发起方——宿主 vit_governance_required → 页内
  // 确认卡；选择经 launcher_governance_choice 真实桥路径回投）──
  const answerGovernance = useCallback((requestID: string, choice: LauncherGovernanceChoice) => {
    setThread((prev) =>
      prev.map((item) =>
        item.kind === "governance" && item.view.requestID === requestID && !item.answered
          ? { ...item, answered: choice }
          : item
      )
    );
    postLauncherGovernanceChoice(requestID, choice);
  }, []);

  useEffect(() => {
    if (fixture) return;
    return subscribeGovernanceRequests((request) => {
      const view: LauncherGovernanceView = {
        requestID: request.request_id,
        action: request.action,
        currentProject: request.current_project
      };
      const autoChoice = smokeGovernanceRef.current;
      setThread((prev) => [
        ...prev,
        { kind: "governance", view, answered: autoChoice || undefined }
      ]);
      if (autoChoice) {
        // 烟测自动应答：走同一真实桥路径（生产 URL 无参零行为）。
        postLauncherGovernanceChoice(request.request_id, autoChoice);
      }
    });
  }, [fixture]);

  // 启动台就绪握手：宿主日志留痕（React launcher surface 已加载且桥在卷）。
  useEffect(() => {
    if (fixture) return;
    postLauncherSurfaceReady();
  }, [fixture]);

  // 宿主结果回执订阅（live only）：更新执行态 + 烟测 ack/排队。
  const handleBridgeResult = useCallback((result: LauncherBridgeResult) => {
    const status = result.status || "failed";
    if (isSmokeRef.current && result.request_id) {
      postLauncherResultAck(result.request_id, status);
    }
    setBridgeExec((prev) => {
      if (!prev) return prev;
      if (result.request_id && result.request_id !== prev.requestId) return prev;
      const state = mapResultState(status);
      const message = launcherDirectiveResultText(status, result.message ?? "", result.action || prev.action);
      return { ...prev, state, message };
    });
    if (isSmokeRef.current) {
      const delay = smokeDelayRef.current;
      window.setTimeout(() => smokeNextRef.current(), delay);
    }
  }, []);

  useEffect(() => {
    if (fixture) return;
    return subscribeLauncherResults(handleBridgeResult);
  }, [fixture, handleBridgeResult]);

  // ── 桥执行：请求经宿主治理 + 公共链；终态只来自 vit_launcher_result ──
  const executeDirective = useCallback(
    (directive: LauncherDirective) => {
      if (fixture) return;
      if (!isLauncherBridgeAvailable()) {
        lastDirectiveRef.current = directive;
        setBridgeExec({
          requestId: createLauncherBridgeRequestID(),
          action: directive.action ?? "",
          state: "failed",
          message: LAUNCHER_BRIDGE_UNAVAILABLE_TEXT
        });
        return;
      }
      lastDirectiveRef.current = directive;
      const requestId = createLauncherBridgeRequestID();
      presentation.begin(requestId, directive.project_name || launcherDisplayFileName(directive.file_path || ""));
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

  const chooseFileViaBridge = useCallback(() => {
    if (fixture || !isLauncherBridgeAvailable()) return;
    // The native dialog has not returned a path yet. A retry must not replay
    // a different directive retained from a previous recents/chat request.
    lastDirectiveRef.current = null;
    const requestId = createLauncherBridgeRequestID();
    presentation.begin(requestId, "");
    setBridgeExec({ requestId, action: "choose_file", state: "waiting", message: LAUNCHER_BRIDGE_WAITING_TEXT });
    const posted = postLauncherChooseFile(requestId);
    if (!posted) {
      presentation.result(requestId, "failed", LAUNCHER_BRIDGE_UNAVAILABLE_TEXT);
      setBridgeExec({ requestId, action: "choose_file", state: "failed", message: LAUNCHER_BRIDGE_UNAVAILABLE_TEXT });
    }
  }, [fixture, presentation.begin, presentation.result]);

  const retryLastDirective = useCallback(() => {
    const directive = lastDirectiveRef.current;
    if (directive) executeDirective(directive);
  }, [executeDirective]);

  // ── SHELL-5：抽屉「＋新建」=直连 directive（不经对话问询）──
  const directNewProject = useCallback(() => {
    if (fixture) return;
    const directive = buildDirectNewDirective();
    const view = directivePlaceholder(directive);
    if (view) {
      setThread((prev) => [...prev, { kind: "directive", view, directive }]);
    }
    executeDirective(directive);
  }, [executeDirective, fixture]);

  // ── LAUNCHER-RECENTS-CLICK-1（383）：抽屉磁带卡点选 = 该工程 open——走与对话流
  // 结果卡/抽屉「打开…」相同的 open 链（executeDirective → 宿主治理 → 内核公共链；
  // requires_governance 恒真语义沿用）。agent 卡内嵌 directive 优先；缺省防御性本地
  // 构造同 schema。线程内落 directive 占位（「正在打开《X》…」+桥执行态）=装载反馈面。
  const openRecentProject = useCallback(
    (entry: LauncherProjectCard) => {
      if (fixture) return;
      const directive: LauncherDirective = entry.directive ?? {
        action: "open_project",
        file_path: entry.filePath,
        requires_governance: true
      };
      const view = directivePlaceholder(directive) ?? cardPendingPlaceholder(entry);
      setThread((prev) => [...prev, { kind: "directive", view, directive }]);
      executeDirective(directive);
    },
    [executeDirective, fixture]
  );

  const loadRecents = useCallback(async (run: number) => {
    setDrawerState("loading");
    setDrawerError("");
    try {
      const entries = await fetchLauncherRecentProjects();
      if (loadRunRef.current !== run) return;
      if (entries.length > 0) {
        setDrawerEntries(entries);
        setDrawerState("ready");
        return;
      }
      setDrawerEntries([]);
      setDrawerState("empty");
    } catch (error) {
      if (loadRunRef.current !== run) return;
      setDrawerState("error");
      setDrawerError(error instanceof Error ? error.message : LAUNCHER_RECENTS_ERROR_TEXT);
    }
  }, []);

  const loadGreeting = useCallback(async (run: number) => {
    setGreetingState("loading");
    setGreetingError("");
    try {
      const data = await fetchLauncherGreeting();
      if (loadRunRef.current !== run) return;
      setGreeting(data);
      setGreetingState("ready");
    } catch (error) {
      if (loadRunRef.current !== run) return;
      setGreeting(null);
      setGreetingError(error instanceof Error ? error.message : LAUNCHER_AGENT_UNAVAILABLE_TEXT);
      setGreetingState("error");
    }
  }, []);

  // live 模式：问候 + 最近工程各自独立加载（互不阻塞，各自有重试）。
  useEffect(() => {
    if (fixture) return;
    const run = ++loadRunRef.current;
    void loadGreeting(run);
    void loadRecents(run);
    return () => {
      loadRunRef.current += 1;
    };
  }, [fixture, loadGreeting, loadRecents]);

  // 问候就绪后把开场白作为首条 Vit 消息入线程（live 模式仅一次）。
  const greetingThreadSeededRef = useRef(false);
  useEffect(() => {
    if (fixture || activeProjectName || greetingThreadSeededRef.current) return;
    if (greetingState !== "ready") return;
    greetingThreadSeededRef.current = true;
    setThread((prev) => (prev.length > 0 ? prev : [{ kind: "assistant", text: greetingDisplayText(greeting) }]));
  }, [fixture, activeProjectName, greeting, greetingState]);

  const retryAll = useCallback(() => {
    const run = ++loadRunRef.current;
    void loadGreeting(run);
    void loadRecents(run);
  }, [loadGreeting, loadRecents]);

  const retryDrawer = useCallback(() => {
    const run = ++loadRunRef.current;
    void loadRecents(run);
  }, [loadRecents]);

  const send = useCallback(
    async (message: string) => {
      const trimmed = message.trim();
      if (!trimmed || busy || launchBusy || fixture || activeProjectName) return;
      setInput("");
      setThread((prev) => [...prev, { kind: "user", text: trimmed }]);
      setBusy(true);
      try {
        const result: LauncherChatResult = await sendLauncherChat(conversationID, trimmed);
        const items: ConvergedThreadItem[] = [];
        const reply = result.reply?.trim();
        if (reply) items.push({ kind: "assistant", text: reply });
        const cards = cardsToEntries(result.project_result_cards);
        if (cards.length > 0) items.push({ kind: "cards", cards });
        const directive = result.launcher_directive;
        const placeholder = directivePlaceholder(directive);
        if (placeholder) items.push({ kind: "directive", view: placeholder, directive: directive ?? undefined });
        if (items.length === 0) {
          items.push({ kind: "assistant", text: result.error?.trim() || "Vit 没有返回内容——换个说法再试一次。" });
        }
        setThread((prev) => [...prev, ...items]);
        // SHELL-5：directive 到达即自动执行（生产默认）。
        if (directive && directive.action) {
          executeDirective(directive);
        }
      } catch (error) {
        const text = error instanceof Error ? error.message : "launcher 聊天请求失败";
        setThread((prev) => [
          ...prev,
          { kind: "assistant", text: `这轮没送达（${text}）。确认 VitAgent 在运行后重发一次。` }
        ]);
        setInput(trimmed); // 失败还原输入，避免用户重打长句
      } finally {
        setBusy(false);
      }
    },
    [busy, launchBusy, conversationID, fixture, activeProjectName, executeDirective]
  );

  // 烟测消息队列：greeting 就绪后发第一条；每条 vit_launcher_result 后延迟发下一条
  // （followup=choose_file 时改发桥 choose_file 请求——dock 降级腿，不经 agent 门）。
  const sendNextSmokeMessage = useCallback(() => {
    if (!isSmokeRef.current) return;
    const next = smokeQueueRef.current.shift();
    if (next === undefined) return;
    void send(next);
  }, [send]);
  useEffect(() => {
    smokeNextRef.current = () => {
      const next = smokeQueueRef.current.shift();
      if (next !== undefined) {
        void send(next);
        return;
      }
      if (smokeFollowupRef.current === "choose_file" && !smokeFollowupDoneRef.current) {
        smokeFollowupDoneRef.current = true;
        chooseFileViaBridge();
      }
    };
  }, [sendNextSmokeMessage, chooseFileViaBridge]);
  useEffect(() => {
    if (fixture || !smokeParams.enabled || smokeStartedRef.current) return;
    if (greetingState !== "ready" || smokeParams.messages.length === 0) return;
    smokeStartedRef.current = true;
    smokeQueueRef.current = [...smokeParams.messages];
    smokeNextRef.current();
  }, [fixture, greetingState, smokeParams, sendNextSmokeMessage]);

  // 383 真栈腿接缝：followup=recents_click[:N] 时，greeting+recents 就绪且烟测消息
  // 轮收尾（!busy）后，按 FOLLOWUP_MS 延迟经同一 openRecentProject 链点磁带卡 N
  // （恰好一次；**跳过 exists=false 条目**——烟测驱动尊重 disabled 语义，不模拟
  // 点不可点按钮；生产 URL 无 launcher_smoke → 全零行为）。
  const recentsClickFiredRef = useRef(false);
  useEffect(() => {
    if (fixture || !smokeParams.enabled) return;
    const followup = smokeParams.followup;
    if (typeof followup !== "object" || followup.kind !== "recents_click") return;
    if (recentsClickFiredRef.current || !smokeStartedRef.current || busy) return;
    if (greetingState !== "ready" || drawerState !== "ready" || drawerEntries.length === 0) return;
    const entry = drawerEntries[followup.index];
    if (!entry || entry.exists === false) return;
    recentsClickFiredRef.current = true;
    const delay = smokeDelayRef.current;
    window.setTimeout(() => openRecentProject(entry), delay);
  }, [fixture, smokeParams, busy, greetingState, drawerState, drawerEntries, openRecentProject]);

  const onSubmit = (event?: FormEvent) => {
    event?.preventDefault();
    void send(input);
  };

  const drawerViewLabel = drawerCountLabel({ state: drawerState, entries: drawerEntries, count: drawerEntries.length, errorText: drawerError });

  const frameClass = ["v5-launcher-frame", fill ? "v5-launcher-frame--fill" : ""].filter(Boolean).join(" ");
  // 根容器 fill 修饰（UI-VISUAL-FIDELITY-1 W2）：#root 视口高 → 根容器显式 100%。
  const rootClass = ["v5-design v5-launcher v5-converged-launcher", fill ? "v5-launcher--fill" : ""]
    .filter(Boolean)
    .join(" ");
  const rootState = fixture ? (fixture.greetingErrorText ? "error" : "ready") : greetingState;
  const bridgeBusy = bridgeExec?.state === "waiting" || launchBusy;
  // Composer 输入门（对齐旧壳语义：greeting 读取中输入不可用——呈现为 pointer-events
  // 门 + 半透明，chat/Composer 无 disabled prop，作用域 CSS 落 converged.css）。
  const composerGated = !fixture && (greetingState === "loading" || bridgeBusy);

  const segments = useMemo(() => segmentThread(thread), [thread]);

  const renderBridgeExec = () => {
    if (!bridgeExec) return <WindowLaunchFeedback />;
    if (presentation.state.requestId === bridgeExec.requestId) return (
      <div data-launcher-bridge-exec data-launcher-bridge-exec-state={bridgeExec.state}>
        <WindowLaunchFeedback onRetry={lastDirectiveRef.current ? retryLastDirective : undefined} />
      </div>
    );
    const terminal = bridgeExec.state !== "waiting" && bridgeExec.state !== "pending";
    const stateClass = `v5-launcher-bridge-exec--${
      bridgeExec.state === "opened" || bridgeExec.state === "dialog_shown"
        ? "ok"
        : bridgeExec.state === "waiting"
          ? "wait"
          : "bad"
    }`;
    return (
      <div
        className={`v5-launcher-bridge-exec ${stateClass}`}
        data-launcher-bridge-exec
        data-launcher-bridge-exec-state={bridgeExec.state}
      >
        <span className="v5-launcher-bridge-exec-label">
          {bridgeExec.state === "waiting"
            ? "Vit 执行中…"
            : bridgeExec.state === "opened"
              ? "已打开"
              : bridgeExec.state === "canceled"
                ? "已取消"
                : bridgeExec.state === "unsupported"
                  ? "暂未开放"
                  : bridgeExec.state === "dialog_shown"
                    ? "对话框已弹出"
                    : "未完成"}
        </span>
        <span className="v5-launcher-bridge-exec-message">{bridgeExec.message}</span>
        {terminal && launcherDirectiveResultRetryable(bridgeExec.state) ? (
          <button
            type="button"
            className="v5-btn v5-btn--secondary v5-launcher-retry"
            onClick={retryLastDirective}
            data-launcher-bridge-retry
          >
            重试
          </button>
        ) : null}
      </div>
    );
  };

  const renderThreadSegment = (segment: ThreadSegment) => {
    if (segment.kind === "messages") {
      // 对话流复用（386 出域组件）：文本消息经 MessageStream 渲染；MainApp 专有面
      // （轨迹/台账/试听/宏/活动线）按空态 stub——launcher 卷零这些证据是事实而非缺失。
      return (
        <MessageStream
          key={segment.key}
          messages={segment.messages}
          activities={[]}
          trajectory={stubTrajectory}
          turnEventMeta={stubTurnEventMeta}
          roundSteps={stubRoundSteps}
          receipts={[]}
          audition={stubAudition}
          auditionBusySessionID=""
          authorityMode="manual_confirmation"
          respondingActionID=""
          isSending={busy}
          hiddenActionID=""
          bottomInset={0}
          onInteractionAction={() => undefined}
          onInvoke={stubInvoke}
          onSelectArtifact={() => undefined}
          uiState={null}
          onMacroValuePreview={() => undefined}
          onMacroValueCommit={async () => undefined}
          onAuditionSelect={async () => undefined}
          onAuditionStop={async () => undefined}
        />
      );
    }
    if (segment.kind === "cards") {
      return (
        <div key={segment.key} className="v5-launcher-cards" data-launcher-cards>
          {segment.cards.map((card) => (
            <button
              key={card.id}
              type="button"
              className="v5-launcher-card v5-converged-card"
              data-launcher-card={card.id}
              onClick={() => {
                // 卡片点选 = 自动执行该卡 directive（SHELL-5：点卡即开）。
                const cardDirective = card.directive ?? undefined;
                setThread((prev) => [
                  ...prev,
                  { kind: "directive", view: cardPendingPlaceholder(card), directive: cardDirective }
                ]);
                if (cardDirective && cardDirective.action) {
                  executeDirective(cardDirective);
                }
              }}
            >
              {/* 366：对话流建议卡工程位=磁带卡整卡形态（compact 档，同形核对） */}
              <TapeCardFace name={card.name} filePath={card.filePath} compact />
            </button>
          ))}
        </div>
      );
    }
    if (segment.kind === "governance") {
      // SHELL-2 页内治理三键卡（对话流内 v5.1 样式；选择经真实桥路径回投宿主执行）。
      return (
        <div
          key={segment.key}
          className="v5-launcher-governance"
          data-launcher-governance
          data-launcher-governance-request={segment.view.requestID}
          data-launcher-governance-answered={segment.answered ?? ""}
        >
          <span className="v5-launcher-governance-headline" data-launcher-governance-headline>
            {launcherGovernanceHeadline(segment.view)}
          </span>
          <span className="v5-launcher-governance-note">
            治理确认跟随发起方：本次切换由对话发起，确认在此页内完成——选择后 Vit 才执行切换（保存由 Vit 完成）。
          </span>
          {segment.answered ? (
            <span className="v5-launcher-governance-answered" data-launcher-governance-answered-text>
              {launcherGovernanceAnsweredText(segment.answered)}
            </span>
          ) : (
            <div className="v5-launcher-governance-actions" data-launcher-governance-actions>
              <button
                type="button"
                className="v5-btn v5-btn--primary"
                data-launcher-governance-save
                disabled={!bridgeAvailable}
                onClick={() => answerGovernance(segment.view.requestID, "save")}
              >
                {LAUNCHER_GOVERNANCE_SAVE_TEXT}
              </button>
              <button
                type="button"
                className="v5-btn v5-btn--secondary"
                data-launcher-governance-switch
                disabled={!bridgeAvailable}
                onClick={() => answerGovernance(segment.view.requestID, "switch")}
              >
                {LAUNCHER_GOVERNANCE_SWITCH_TEXT}
              </button>
              <button
                type="button"
                className="v5-btn v5-btn--secondary"
                data-launcher-governance-cancel
                disabled={!bridgeAvailable}
                onClick={() => answerGovernance(segment.view.requestID, "cancel")}
              >
                {LAUNCHER_GOVERNANCE_CANCEL_TEXT}
              </button>
            </div>
          )}
        </div>
      );
    }
    return (
      <div key={segment.key} className="v5-launcher-directive" data-launcher-directive data-launcher-directive-state="auto">
        <span className="v5-launcher-directive-headline">{segment.view.headline}</span>
        {segment.view.detail ? <span className="v5-launcher-directive-detail">{segment.view.detail}</span> : null}
        <span className="v5-launcher-directive-note">{segment.view.note}</span>
        {segment.directive && !bridgeAvailable ? (
          <span className="v5-launcher-bridge-note" data-launcher-bridge-note>
            {LAUNCHER_BRIDGE_UNAVAILABLE_TEXT}
          </span>
        ) : null}
      </div>
    );
  };

  return (
    <div
      className={rootClass}
      data-launcher-root
      data-launcher-state={rootState}
      data-launcher-bridge={bridgeAvailable ? "available" : fixture ? "fixture" : "unavailable"}
      data-launcher-kernel={
        greeting?.kernel_unavailable ? "unavailable" : greetingState === "ready" ? "online" : undefined
      }
      data-fixture={fixture ? "converged-launcher" : undefined}
      data-launcher-mode={activeProjectName ? "project-management" : "startup"}
      data-native-focus={presentation.state.window?.foreground ?? "unknown"}
    >
      <div className={frameClass} data-launcher-frame>
        <header className="v5-launcher-titlebar" data-launcher-titlebar>
          <span className="v5-launcher-titlebar-brand">Vit</span>
          <button type="button" className="v5-btn v5-btn--secondary v5-window-sidebar-toggle" data-window-sidebar-toggle
            aria-controls={sidebarId} aria-expanded={presentation.sidebarOpen} onClick={() => presentation.setSidebarOpen(!presentation.sidebarOpen)}>
            {presentation.sidebarOpen ? "收起工程列表" : "展开工程列表"}
          </button>
          <NativeFocusHint />
        </header>
        <div className="v5-launcher-body">
          <aside
            className="v5-launcher-drawer"
            data-launcher-drawer
            data-drawer-state={drawerState}
            id={sidebarId} data-window-sidebar-slot="recents" data-sidebar-open={presentation.sidebarOpen ? "true" : "false"}
            aria-hidden={!presentation.sidebarOpen} inert={!presentation.sidebarOpen}
            aria-label="最近工程"
          >
            <div className="v5-converged-projects" data-launcher-projects data-scroll-intentional="projects">
            {activeProjectName ? (
              <section className="v5-converged-active-project" aria-label="当前打开的工程" data-launcher-active-project>
                <span>工程保持打开</span>
                <strong data-launcher-active-project-name>{activeProjectName}</strong>
                <button type="button" className="v5-btn v5-btn--secondary" onClick={onReturnProject} disabled={!onReturnProject} data-launcher-return-project>
                  返回工程
                </button>
                <p>这里用于查找、新建与打开工程；切换工程仍需确认。</p>
              </section>
            ) : null}
            <div className="v5-launcher-drawer-head">
              <span className="v5-launcher-drawer-title">最近工程</span>
              <span className="v5-launcher-drawer-count" data-launcher-recents-count={drawerEntries.length}>
                {drawerViewLabel}
              </span>
            </div>
            {/* SHELL-2 抽屉手动按钮；SHELL-5 语义收口：「＋新建」=直连 directive；
                「打开…」=宿主原生文件对话框。空 recents 态两按钮主路径突出。 */}
            <div
              className="v5-launcher-drawer-actions"
              data-launcher-drawer-actions
              data-drawer-primary={drawerState === "empty" ? "true" : undefined}
            >
              <button
                type="button"
                className="v5-btn v5-btn--primary v5-launcher-new"
                data-launcher-new
                data-launcher-new-mode="direct"
                disabled={busy || bridgeBusy || (!fixture && greetingState !== "ready")}
                onClick={directNewProject}
                title="直接新建工程并进入（不经对话问询；缺省名进工程后可改）"
              >
                ＋新建
              </button>
              {bridgeAvailable ? (
                <button
                  type="button"
                  className="v5-btn v5-btn--secondary v5-launcher-choose-file v5-launcher-open"
                  data-launcher-choose-file
                  onClick={chooseFileViaBridge}
                  disabled={bridgeBusy}
                  title="由 Vit 弹出原生文件对话框选择工程"
                >
                  打开…
                </button>
              ) : null}
            </div>
            {drawerState === "loading" ? (
              <p className="v5-launcher-drawer-note" data-launcher-drawer-loading>
                正在读取最近工程…
              </p>
            ) : null}
            {drawerState === "error" ? (
              <div className="v5-launcher-drawer-error" data-launcher-drawer-error>
                <span>{drawerError || LAUNCHER_RECENTS_ERROR_TEXT}</span>
                <button type="button" className="v5-btn v5-btn--secondary v5-launcher-retry" onClick={retryDrawer} data-launcher-drawer-retry>
                  重试
                </button>
              </div>
            ) : null}
            {drawerState === "empty" ? (
              <p className="v5-launcher-drawer-note" data-launcher-drawer-empty>
                还没有最近工程记录。点「＋新建」直接新建一个工程，或点「打开…」选择工程文件。
              </p>
            ) : null}
            {drawerEntries.length > 0 ? (
              <ul className="v5-launcher-recents" data-launcher-recents>
                {drawerEntries.map((entry) => {
                  // 366（用户裁定 b 案）：工程条目本身=磁带盒造型整卡（非图标+文字行）——
                  // 工程名/路径排布进磁带卡结构（底标位），氛围色=工程名哈希沿用。
                  // 383（LAUNCHER-RECENTS-CLICK-1）：整卡可点=发起该工程 open（同对话流
                  // 结果卡链）；agent os.Stat 实测缺失（exists===false）→置灰不可点+如实
                  // 注记（健康度标注最小档；字段缺省=旧 agent 未知，按可点，点后失败走
                  // 宿主降级文案，不虚报）。禁用门与「＋新建」同语义（busy/桥在飞/未就绪）。
                  const missing = entry.exists === false;
                  const displayName = entry.name || launcherDisplayFileName(entry.filePath);
                  return (
                    <li
                      key={entry.id}
                      className="v5-launcher-recent v5-converged-recent"
                      data-launcher-recent
                      data-tape-card
                      data-tape-card-mode={entry.name ? "named" : "derived"}
                      data-launcher-recent-missing={missing ? "true" : undefined}
                    >
                      <button
                        type="button"
                        className="v5-launcher-recent-open"
                        data-launcher-recent-open={entry.id}
                        disabled={missing || busy || bridgeBusy || (!fixture && greetingState !== "ready")}
                        onClick={() => openRecentProject(entry)}
                        title={
                          missing
                            ? `《${displayName}》的工程文件已不在原位置——检查文件后重试`
                            : `打开工程《${displayName}》（${entry.filePath}）`
                        }
                        aria-label={
                          missing
                            ? `工程 ${displayName}（文件缺失，不可打开）`
                            : `打开工程 ${displayName}`
                        }
                      >
                        <TapeCardFace name={entry.name} filePath={entry.filePath} />
                      </button>
                      {missing ? (
                        <span className="v5-launcher-recent-missing-note" data-launcher-recent-missing-note>
                          文件已不在原位置
                        </span>
                      ) : null}
                    </li>
                  );
                })}
              </ul>
            ) : null}
            </div>
            {onOpenSettings ? (
              <footer className="v5-converged-drawer-footer">
                <button type="button" className="v5-btn v5-btn--secondary" onClick={onOpenSettings} data-launcher-settings>
                  设置
                </button>
              </footer>
            ) : null}
          </aside>
          <section className="v5-launcher-main" aria-label={activeProjectName ? "Vit 工程管理对话" : "Vit 启动台对话"}>
            {greetingState === "loading" ? (
              <p className="v5-launcher-greeting-status" data-launcher-greeting-loading>
                正在连接 Vit agent…
              </p>
            ) : null}
            {greetingState === "error" ? (
              <div className="v5-launcher-agent-error" data-launcher-agent-error>
                <span>{greetingError || LAUNCHER_AGENT_UNAVAILABLE_TEXT}</span>
                <button type="button" className="v5-btn v5-btn--secondary v5-launcher-retry" onClick={retryAll} data-launcher-retry>
                  重试
                </button>
              </div>
            ) : null}
            {greetingState === "ready" ? (
              <>
                <p className="v5-launcher-greeting" data-launcher-greeting>
                  {activeProjectName ? "工程管理" : greetingDisplayText(greeting)}
                </p>
                <div className="v5-launcher-thread v5-converged-thread" data-launcher-thread data-scroll-intentional="thread">
                  {segments.map((segment) => renderThreadSegment(segment))}
                  {renderBridgeExec()}
                </div>
              </>
            ) : null}
            {activeProjectName ? (
              <p className="v5-converged-management-note" data-launcher-management-note>
                这里用于管理工程。要继续当前工程的对话，请点「返回工程」。
              </p>
            ) : (
              <>
            <ul className="v5-launcher-suggestions" data-launcher-suggestions aria-label="上下文建议">
              {LAUNCHER_SUGGESTIONS.map((suggestion) => (
                <li key={suggestion.text}>
                  <button
                    type="button"
                    className="v5-launcher-suggestion"
                    data-launcher-suggestion={suggestion.text}
                    title={suggestion.hint}
                    onClick={() => void send(suggestion.text)}
                  >
                    {suggestion.text}
                  </button>
                </li>
              ))}
            </ul>
            <fieldset className="v5-converged-composer v5-window-composer-fieldset" data-launcher-input data-composer-gated={composerGated ? "true" : undefined} disabled={composerGated}>
              <Composer
                composerRef={composerRef}
                input={input}
                setInput={setInput}
                pendingArtifacts={[]}
                pendingMacroControls={[]}
                mode="default"
                agentTurnRunning={false}
                stopTurnBusy={false}
                isSending={busy || bridgeBusy}
                isUploading={false}
                interactionAction={null}
                respondingActionID=""
                authorityMode="manual_confirmation"
                authorityLocked={false}
                onAuthorityModeChange={() => undefined}
                onSubmit={onSubmit}
                onUploadClick={() => undefined}
                onModeChange={() => undefined}
                onStopTurn={() => undefined}
                onInteractionAction={() => undefined}
                onInvoke={stubInvoke}
                onSelectArtifact={() => undefined}
                onRemoveArtifact={() => undefined}
                onRemoveMacroControl={() => undefined}
              />
            </fieldset>
              </>
            )}
          </section>
        </div>
      </div>
    </div>
  );
}
