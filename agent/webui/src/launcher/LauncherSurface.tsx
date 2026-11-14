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
 * 硬边界（卡面）：结果卡 / directive 只做数据展示，不执行 open/new；
 * directive 一律渲染「准备打开…/准备新建…」占位。不硬编码示例工程名。
 *
 * fixture 接缝：props 传入 LauncherFixtureOverrides 时完全离线（gallery /
 * render-smoke 使用），真实端点零请求；不传即 live 模式。
 */

import { useCallback, useEffect, useRef, useState } from "react";
import type { FormEvent } from "react";
import {
  cardsToEntries,
  fetchLauncherGreeting,
  sendLauncherChat,
  type LauncherChatResult,
  type LauncherGreeting,
  type LauncherProjectCard
} from "../api/launcher";
import {
  cardPendingPlaceholder,
  createLauncherConversationID,
  directivePlaceholder,
  drawerCountLabel,
  greetingDisplayText,
  LAUNCHER_AGENT_UNAVAILABLE_TEXT,
  LAUNCHER_RECENTS_ERROR_TEXT,
  LAUNCHER_SUGGESTIONS,
  launcherDisplayFileName,
  type DirectivePlaceholderView,
  type LauncherGreetingState
} from "./launcherData";
import { ProjectBadge, TextField } from "../components";
import "../styles/tokens.css";
import "../components/components.css";
import "../styles/launcher.css";

/** 会话线程条目（问候之后的所有展示均为数据呈现，无执行副作用）。 */
export type LauncherThreadItem =
  | { kind: "assistant"; text: string }
  | { kind: "user"; text: string }
  | { kind: "cards"; cards: LauncherProjectCard[] }
  | { kind: "directive"; view: DirectivePlaceholderView };

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
        const placeholder = directivePlaceholder(result.launcher_directive);
        if (placeholder) items.push({ kind: "directive", view: placeholder });
        if (items.length === 0) {
          items.push({ kind: "assistant", text: result.error?.trim() || "Vit 没有返回内容——换个说法再试一次。" });
        }
        setThread((prev) => [...prev, ...items]);
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
    [busy, conversationID, fixture]
  );

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void send(input);
  };

  const drawerViewLabel = drawerCountLabel({ state: drawerState, entries: drawerEntries, count: drawerEntries.length, errorText: drawerError });

  const frameClass = ["v5-launcher-frame", fill ? "v5-launcher-frame--fill" : ""].filter(Boolean).join(" ");
  const rootState = fixture ? (fixture.greetingErrorText ? "error" : "ready") : greetingState;

  return (
    <div
      className="v5-design v5-launcher"
      data-launcher-root
      data-launcher-state={rootState}
      data-fixture={fixture ? "launcher-surface" : undefined}
    >
      <div className={frameClass} data-launcher-frame>
        <header className="v5-launcher-titlebar" data-launcher-titlebar>
          <span className="v5-launcher-titlebar-brand">Vit</span>
          <span className="v5-launcher-titlebar-sub">启动台</span>
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
                还没有最近工程记录。可以说「新建一个工程」，或从磁带流里选文件打开。
              </p>
            ) : null}
            {drawerEntries.length > 0 ? (
              <ul className="v5-launcher-recents" data-launcher-recents>
                {drawerEntries.map((entry) => (
                  <li key={entry.id} className="v5-launcher-recent" data-launcher-recent>
                    <ProjectBadge size={20} />
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
                <div className="v5-launcher-thread" data-launcher-thread>
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
                                // 仅切换占位展示（数据呈现），不执行任何 open/new。
                                setThread((prev) => [...prev, { kind: "directive", view: cardPendingPlaceholder(card) }]);
                              }}
                            >
                              <ProjectBadge size={20} />
                              <span>
                                <span className="v5-launcher-card-name">{card.name || launcherDisplayFileName(card.filePath)}</span>
                                {card.filePath ? <span className="v5-launcher-card-path">{card.filePath}</span> : null}
                              </span>
                            </button>
                          ))}
                        </div>
                      );
                    }
                    return (
                      <div key={index} className="v5-launcher-directive" data-launcher-directive data-launcher-directive-state="pending">
                        <span className="v5-launcher-directive-headline">{item.view.headline}</span>
                        {item.view.detail ? <span className="v5-launcher-directive-detail">{item.view.detail}</span> : null}
                        <span className="v5-launcher-directive-note">{item.view.note}</span>
                      </div>
                    );
                  })}
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
