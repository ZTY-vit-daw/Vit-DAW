/*
 * UI-LAUNCHER-1 启动台 surface（?surface=launcher）。
 *
 * 组成（卡面）：左侧工程抽屉（真实最近工程）+ 问候语 + 输入框 + 3–4 个上下文建议；
 * 标题栏只写 "Vit" 文本（V 品牌标不进标题栏——v5.1 纪律，gallery 断言在卷）。
 *
 * 数据（卡面）：消费真实 /agent/launcher/greeting 与 launcher 聊天
 * （context.launcher=true，agent 端确定性意图路由零 LLM）。完整最近工程列表经
 * launcher 聊天 list 意图（"有哪些工程"）获取——greeting 只回执首项+计数。
 *
 * UI-BRIDGE-1 桥面（SHELL-5：directive 到达即自动执行）：launcher 确定性路由
 * 命中 新建/打开 → directive 自动经 ask_vit_launcher 通道发出（不再等
 * 「让 Vit 新建/打开」手动按钮；治理确认仍按 337 语义仅在需要时由宿主出）；
 * 终态一律以宿主 vit_launcher_result 回执呈现——React 不伪造「已打开」；
 * 桥不可用给诚实文案。选择文件请求经 launcher_choose_file（宿主原生文件
 * 对话框）。抽屉「＋新建」按钮=直连 directive（缺省草稿名，不经对话问询——
 * 点击即建并自动进工程）；「打开…」保持宿主原生文件对话框。
 * 呈现区分（SHELL-5）：执行轮=「正在…」占位+下方桥执行态回执；确定性问答
 * （列表/说明）=纯文本回执——用户可分辨「Vit 正在执行」与「预设应答」。
 *
 * SHELL-2 页内治理（跟随发起方裁定）：agent/对话发起的工程切换 → 宿主回投
 * vit_governance_required → 本 surface 在对话流内出三键确认卡（保存并切换/直接切换/
 * 取消，v5.1 样式），选择经 launcher_governance_choice 回投宿主执行；GUI（DAW 内）
 * 发起的切换仍走 Godot 弹窗（不变）。
 *
 * fixture 接缝：props 传入 LauncherFixtureOverrides 时完全离线（gallery /
 * render-smoke 使用），真实端点零请求、零桥调用；不传即 live 模式。
 * 烟测接缝：URL launcher_smoke=1（生产 URL 无参零行为）——自动发送
 * launcher_smoke_message（可 || 分隔多条，回执后延迟发下一条）并回执
 * launcher_result_ack（取证）；directive 自动执行是生产默认（SHELL-5 转正，
 * 不再限于烟测参数）；launcher_smoke_governance=save|switch|cancel 时治理卡
 * 出现即自动经真实桥路径回投选择（镜像 Godot 侧 VIT_SMOKE_LAUNCHER_GOVERNANCE
 * 模式，生产零行为）。
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { FormEvent } from "react";
import {
  cardsToEntries,
  fetchLauncherGreeting,
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
import { TextField } from "../components";
// UI-REC-TAPE-1（365）：启动台工程列表头像换磁带 cassette 造型（用户裁定第 1 条；
// i-project 简化标志保留给其余 ≤48px 场用位，见 SPEC-V5 §7.3.1 修订段）。
import { TapeBadge } from "../components/ProjectBadge";
import "../styles/tokens.css";
import "../components/components.css";
import "../styles/launcher.css";

/** 会话线程条目（directive 携带原始指令供桥执行；fixture 线程不带 directive）。
 * SHELL-2：governance=宿主页内治理请求（跟随发起方——对话发起的工程切换在页内确认）。 */
export type LauncherThreadItem =
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

export interface LauncherFixtureOverrides {
  /** fixture greeting（离线注入）。 */
  greeting?: LauncherGreeting;
  /** fixture：greeting 拉取失败文案（agent 不可用态样例）。 */
  greetingErrorText?: string;
  /** fixture 抽屉态与条目。 */
  drawerState?: "loading" | "ready" | "error" | "empty";
  drawerEntries?: LauncherProjectCard[];
  drawerErrorText?: string;
  /** fixture 初始线程。 */
  thread?: LauncherThreadItem[];
  /** fixture 输入框预填（仅展示）。 */
  initialInput?: string;
}

export interface LauncherSurfaceProps {
  /** gallery / 渲染烟测 fixture 模式（离线，不出网）。 */
  fixture?: LauncherFixtureOverrides;
  /** 撑满视口（live surface 用）。 */
  fill?: boolean;
}

type DrawerState = "loading" | "ready" | "error" | "empty";

/** list 意图词面（确定性零 LLM，见 agent launcherClassifyMessage）。 */
const RECENTS_LIST_MESSAGE = "有哪些工程";

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

/** 烟测 URL 参数（生产 URL 无 launcher_smoke → 全零行为）。 */
function parseSmokeParams(): {
  enabled: boolean;
  messages: string[];
  delayMs: number;
  followup: string;
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
  // followup=choose_file：回执后下一段改发 launcher_choose_file（dock 降级腿——
  // open/new directive 经 agent 门（340：进程内 user_entered_project 后关闭，
  // 跨工程意图归 C5），choose_file 不经 agent 门，纯桥请求可驱动 dock 降级面）。
  const followup = params.get("launcher_smoke_followup") === "choose_file" ? "choose_file" : "message";
  // SHELL-2：治理卡自动应答（save|switch|cancel；出现即经真实桥路径回投）。
  const rawGovernance = (params.get("launcher_smoke_governance") ?? "").toLowerCase();
  const governance =
    rawGovernance === "save" || rawGovernance === "switch" || rawGovernance === "cancel"
      ? (rawGovernance as LauncherGovernanceChoice)
      : "";
  return { enabled, messages, delayMs, followup, governance };
}

export function LauncherSurface({ fixture, fill = false }: LauncherSurfaceProps) {
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
  const [thread, setThread] = useState<LauncherThreadItem[]>(fixtureThread ?? []);
  const [input, setInput] = useState(fixture?.initialInput ?? "");
  const [busy, setBusy] = useState(false);
  const [conversationID] = useState(() => createLauncherConversationID());
  const loadRunRef = useRef(0);

  // ── UI-BRIDGE-1：桥可用性（一次性判定）+ 烟测参数（URL 一次性解析）+ 执行态 ──
  const bridgeAvailable = useMemo(() => !fixture && isLauncherBridgeAvailable(), [fixture]);
  // fixture 模式（node 渲染/无 window）完全离线：烟测参数不解析。
  const smokeParams = useMemo(
    () =>
      fixture
        ? { enabled: false, messages: [] as string[], delayMs: 8000, followup: "message", governance: "" as LauncherGovernanceChoice | "" }
        : parseSmokeParams(),
    [fixture]
  );
  const [bridgeExec, setBridgeExec] = useState<BridgeExecState | null>(null);
  const lastDirectiveRef = useRef<LauncherDirective | null>(null);
  const isSmokeRef = useRef(false);
  const smokeDelayRef = useRef(8000);
  const smokeFollowupRef = useRef("message");
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
      setBridgeExec({ requestId, action: directive.action ?? "", state: "waiting", message: LAUNCHER_BRIDGE_WAITING_TEXT });
      const posted = postLauncherDirective(directive as unknown as Record<string, unknown>, requestId);
      if (!posted) {
        setBridgeExec({
          requestId,
          action: directive.action ?? "",
          state: "failed",
          message: LAUNCHER_BRIDGE_UNAVAILABLE_TEXT
        });
      }
    },
    [fixture]
  );

  const chooseFileViaBridge = useCallback(() => {
    if (fixture || !isLauncherBridgeAvailable()) return;
    const requestId = createLauncherBridgeRequestID();
    setBridgeExec({ requestId, action: "choose_file", state: "waiting", message: LAUNCHER_BRIDGE_WAITING_TEXT });
    const posted = postLauncherChooseFile(requestId);
    if (!posted) {
      setBridgeExec({ requestId, action: "choose_file", state: "failed", message: LAUNCHER_BRIDGE_UNAVAILABLE_TEXT });
    }
  }, [fixture]);

  const retryLastDirective = useCallback(() => {
    const directive = lastDirectiveRef.current;
    if (directive) executeDirective(directive);
  }, [executeDirective]);

  // ── SHELL-5：抽屉「＋新建」=直连 directive（不经对话问询）──
  // 点击即建（缺省草稿名=MMDD_HHmm 时间戳，进工程后可改；363 承接名实相符）
  // 并自动进工程；对话流内呈报「正在新建《X》…」+ 桥执行态回执。
  const directNewProject = useCallback(() => {
    if (fixture) return;
    const directive = buildDirectNewDirective();
    const view = directivePlaceholder(directive);
    if (view) {
      setThread((prev) => [...prev, { kind: "directive", view, directive }]);
    }
    executeDirective(directive);
  }, [executeDirective, fixture]);

  const loadRecents = useCallback(async (run: number) => {
    setDrawerState("loading");
    setDrawerError("");
    try {
      const result = await sendLauncherChat(conversationID, RECENTS_LIST_MESSAGE);
      if (loadRunRef.current !== run) return;
      const entries = cardsToEntries(result.project_result_cards);
      if (entries.length > 0) {
        setDrawerEntries(entries);
        setDrawerState("ready");
        return;
      }
      // 无卡回执：按回执文案区分空态与失败（不虚构工程名）。
      const reply = result.reply?.trim() ?? "";
      if (result.error || /连不上内核|暂时/.test(reply)) {
        setDrawerState("error");
        setDrawerError(reply || LAUNCHER_RECENTS_ERROR_TEXT);
      } else {
        setDrawerEntries([]);
        setDrawerState("empty");
      }
    } catch (error) {
      if (loadRunRef.current !== run) return;
      setDrawerState("error");
      setDrawerError(error instanceof Error ? error.message : LAUNCHER_RECENTS_ERROR_TEXT);
    }
  }, [conversationID]);

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
    if (fixture || greetingThreadSeededRef.current) return;
    if (greetingState !== "ready") return;
    greetingThreadSeededRef.current = true;
    setThread((prev) => (prev.length > 0 ? prev : [{ kind: "assistant", text: greetingDisplayText(greeting) }]));
  }, [fixture, greeting, greetingState]);

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
      if (!trimmed || busy || fixture) return;
      setInput("");
      setThread((prev) => [...prev, { kind: "user", text: trimmed }]);
      setBusy(true);
      try {
        const result: LauncherChatResult = await sendLauncherChat(conversationID, trimmed);
        const items: LauncherThreadItem[] = [];
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
        // SHELL-5：directive 到达即自动执行（生产默认——烟测接缝的 auto-execute
        // 语义转正；治理确认仍由宿主按 337 语义仅在需要时出）。
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
    [busy, conversationID, fixture, executeDirective]
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
      // 队列先行（首条+多段消息）；队列排空后 followup=choose_file 才发一次桥
      // choose_file 请求（done 守卫防每次 result 重发）。
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

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void send(input);
  };

  const drawerViewLabel = drawerCountLabel({ state: drawerState, entries: drawerEntries, count: drawerEntries.length, errorText: drawerError });

  const frameClass = ["v5-launcher-frame", fill ? "v5-launcher-frame--fill" : ""].filter(Boolean).join(" ");
  // 根容器 fill 修饰（UI-VISUAL-FIDELITY-1 W2）：#root 视口高 → 根容器显式 100%，
  // frame--fill 的 height:100% 不再按 auto 父级退化为内容高度（裁切修复）。
  const rootClass = ["v5-design v5-launcher", fill ? "v5-launcher--fill" : ""].filter(Boolean).join(" ");
  const rootState = fixture ? (fixture.greetingErrorText ? "error" : "ready") : greetingState;
  const bridgeBusy = bridgeExec?.state === "waiting";

  const renderBridgeExec = () => {
    if (!bridgeExec) return null;
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

  return (
    <div
      className={rootClass}
      data-launcher-root
      data-launcher-state={rootState}
      data-launcher-bridge={bridgeAvailable ? "available" : fixture ? "fixture" : "unavailable"}
      data-launcher-kernel={
        greeting?.kernel_unavailable ? "unavailable" : greetingState === "ready" ? "online" : undefined
      }
      data-fixture={fixture ? "launcher-surface" : undefined}
    >
      <div className={frameClass} data-launcher-frame>
        <header className="v5-launcher-titlebar" data-launcher-titlebar>
          <span className="v5-launcher-titlebar-brand">Vit</span>
        </header>
        <div className="v5-launcher-body">
          <aside
            className="v5-launcher-drawer"
            data-launcher-drawer
            data-drawer-state={drawerState}
            aria-label="最近工程"
          >
            <div className="v5-launcher-drawer-head">
              <span className="v5-launcher-drawer-title">最近工程</span>
              <span className="v5-launcher-drawer-count" data-launcher-recents-count={drawerEntries.length}>
                {drawerViewLabel}
              </span>
            </div>
            {/* SHELL-2 抽屉手动按钮；SHELL-5 语义收口：「＋新建」=直连 directive
                （缺省草稿名，点击即建并自动进工程，不经对话问询）；「打开…」=
                宿主原生文件对话框（348 choose_file 语义宿主版，行为不变）。
                空 recents 态两按钮主路径突出（data-drawer-primary 修饰）。 */}
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
                {drawerEntries.map((entry) => (
                  <li key={entry.id} className="v5-launcher-recent" data-launcher-recent>
                    {/* UI-REC-TAPE-1（365）：工程列表磁带 cassette 头像（氛围色=工程名哈希） */}
                    <TapeBadge size={24} name={entry.name || launcherDisplayFileName(entry.filePath)} />
                    <span>
                      <span className="v5-launcher-recent-name">{entry.name || launcherDisplayFileName(entry.filePath)}</span>
                      {entry.filePath ? <span className="v5-launcher-recent-path">{entry.filePath}</span> : null}
                    </span>
                  </li>
                ))}
              </ul>
            ) : null}
          </aside>
          <section className="v5-launcher-main" aria-label="Vit 启动台对话">
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
                  {greetingDisplayText(greeting)}
                </p>
                <div className="v5-launcher-thread" data-launcher-thread data-scroll-intentional="thread">
                  {thread.map((item, index) => {
                    if (item.kind === "user") {
                      return (
                        <p key={index} className="v5-launcher-msg v5-launcher-msg--user" data-launcher-msg="user">
                          {item.text}
                        </p>
                      );
                    }
                    if (item.kind === "assistant") {
                      return (
                        <p key={index} className="v5-launcher-msg v5-launcher-msg--assistant" data-launcher-msg="assistant">
                          {item.text}
                        </p>
                      );
                    }
                    if (item.kind === "cards") {
                      return (
                        <div key={index} className="v5-launcher-cards" data-launcher-cards>
                          {item.cards.map((card) => (
                            <button
                              key={card.id}
                              type="button"
                              className="v5-launcher-card"
                              data-launcher-card={card.id}
                              onClick={() => {
                                // 卡片点选 = 自动执行该卡 directive（SHELL-5：点卡即开，
                                // 不再出「让 Vit 打开」手动按钮等待）。
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
                              {/* UI-REC-TAPE-1（365）：建议卡工程位磁带 cassette 头像 */}
                              <TapeBadge size={20} name={card.name || launcherDisplayFileName(card.filePath)} />
                              <span>
                                <span className="v5-launcher-card-name">{card.name || launcherDisplayFileName(card.filePath)}</span>
                                {card.filePath ? <span className="v5-launcher-card-path">{card.filePath}</span> : null}
                              </span>
                            </button>
                          ))}
                        </div>
                      );
                    }
                    if (item.kind === "governance") {
                      // SHELL-2 页内治理三键卡（对话流内 v5.1 样式；选择经真实桥
                      // 路径回投宿主执行；已作答后固定呈报不重放）。
                      return (
                        <div
                          key={index}
                          className="v5-launcher-governance"
                          data-launcher-governance
                          data-launcher-governance-request={item.view.requestID}
                          data-launcher-governance-answered={item.answered ?? ""}
                        >
                          <span className="v5-launcher-governance-headline" data-launcher-governance-headline>
                            {launcherGovernanceHeadline(item.view)}
                          </span>
                          <span className="v5-launcher-governance-note">
                            治理确认跟随发起方：本次切换由对话发起，确认在此页内完成——选择后 Vit 才执行切换（保存由 Vit 完成）。
                          </span>
                          {item.answered ? (
                            <span className="v5-launcher-governance-answered" data-launcher-governance-answered-text>
                              {launcherGovernanceAnsweredText(item.answered)}
                            </span>
                          ) : (
                            <div className="v5-launcher-governance-actions" data-launcher-governance-actions>
                              <button
                                type="button"
                                className="v5-btn v5-btn--primary"
                                data-launcher-governance-save
                                disabled={!bridgeAvailable}
                                onClick={() => answerGovernance(item.view.requestID, "save")}
                              >
                                {LAUNCHER_GOVERNANCE_SAVE_TEXT}
                              </button>
                              <button
                                type="button"
                                className="v5-btn v5-btn--secondary"
                                data-launcher-governance-switch
                                disabled={!bridgeAvailable}
                                onClick={() => answerGovernance(item.view.requestID, "switch")}
                              >
                                {LAUNCHER_GOVERNANCE_SWITCH_TEXT}
                              </button>
                              <button
                                type="button"
                                className="v5-btn v5-btn--secondary"
                                data-launcher-governance-cancel
                                disabled={!bridgeAvailable}
                                onClick={() => answerGovernance(item.view.requestID, "cancel")}
                              >
                                {LAUNCHER_GOVERNANCE_CANCEL_TEXT}
                              </button>
                            </div>
                          )}
                        </div>
                      );
                    }
                    return (
                      <div key={index} className="v5-launcher-directive" data-launcher-directive data-launcher-directive-state="auto">
                        <span className="v5-launcher-directive-headline">{item.view.headline}</span>
                        {item.view.detail ? <span className="v5-launcher-directive-detail">{item.view.detail}</span> : null}
                        <span className="v5-launcher-directive-note">{item.view.note}</span>
                        {item.directive && !bridgeAvailable ? (
                          <span className="v5-launcher-bridge-note" data-launcher-bridge-note>
                            {LAUNCHER_BRIDGE_UNAVAILABLE_TEXT}
                          </span>
                        ) : null}
                      </div>
                    );
                  })}
                  {renderBridgeExec()}
                </div>
              </>
            ) : null}
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
            <form className="v5-launcher-composer" data-launcher-input onSubmit={onSubmit}>
              <TextField
                value={input}
                placeholder={busy ? "Vit 回复中…" : "说出想做的事——例如新建工程、继续上次的工程"}
                disabled={fixture ? false : greetingState === "loading"}
                onChange={setInput}
                onSend={() => void send(input)}
              />
            </form>
          </section>
        </div>
      </div>
    </div>
  );
}

export default LauncherSurface;
