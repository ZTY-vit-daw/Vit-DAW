/*
 * UI-LAUNCHER-1 launcher 面纯数据助手（无副作用，供单元测试与渲染面复用）。
 *
 * 纪律（卡面）：不硬编码示例工程名——建议词全是动词/通用句式，
 * 占位文案不冒充「已打开」；fixture 数据只存在于 gallery/render-smoke。
 */

import type { LauncherDirective, LauncherGreeting, LauncherProjectCard } from "../api/launcher";

/** 输入框上方的上下文建议（3–4 个，确定性意图词面，见 launcher_flow.go 分类器）。 */
export const LAUNCHER_SUGGESTIONS: ReadonlyArray<{ text: string; hint: string }> = [
  { text: "看看最近的工程", hint: "列出最近工程卡片" },
  { text: "新建一个工程", hint: "新建意图，agent 会追问工程名" },
  { text: "继续", hint: "接着上一次的工程" }
];

/** launcher 聊天会话 ID（与 App 主面对齐的 webui_<ts36> 形态，自持不引 App）。 */
export function createLauncherConversationID(now: number = Date.now()): string {
  return `webui_launcher_${now.toString(36)}`;
}

/** 路径 → 显示名（去扩展名；与 agent 侧 launcherProjectDisplayName 同语义）。 */
export function launcherDisplayFileName(filePath: string): string {
  const base = filePath.replace(/\\/g, "/").split("/").pop() ?? filePath;
  const dot = base.lastIndexOf(".");
  return dot > 0 ? base.slice(0, dot) : base;
}

export type LauncherGreetingState = "loading" | "ready" | "error";

/** greeting 回执 → 问候展示（reply 原样；空回执给确定性文案，不编造工程名）。 */
export function greetingDisplayText(greeting: LauncherGreeting | null): string {
  const reply = greeting?.reply?.trim();
  return reply || "我在。想从哪开始？";
}

export interface DrawerRecentsView {
  state: "loading" | "ready" | "error" | "empty";
  /** 真实最近工程条目（ready 时非空）。 */
  entries: LauncherProjectCard[];
  /** 抽屉标题里的真实计数（ready 时 = entries.length）。 */
  count: number;
  errorText?: string;
}

/** 问候语右侧 / 抽屉头部计数副注（仅陈述真实计数，不虚构）。 */
export function drawerCountLabel(view: DrawerRecentsView): string {
  if (view.state === "loading") return "正在读取最近工程…";
  if (view.state === "error") return "最近工程读取失败";
  if (view.state === "empty") return "还没有最近工程记录";
  return `最近工程 · ${view.count} 个`;
}

export type LauncherPendingAction = "open" | "new" | "unknown";

export interface DirectivePlaceholderView {
  action: LauncherPendingAction;
  /** 占位标题：「准备打开《X》…」/「准备新建《X》…」。 */
  headline: string;
  /** 数据行：文件路径 / 工程名（原样呈报）。 */
  detail: string;
  /** 固定说明：本卡只做数据展示，执行留待工程桥接。 */
  note: string;
}

/**
 * launcher_directive → 数据展示占位（卡面硬要求：不执行 open/new、不虚报已打开）。
 * agent 侧 requires_governance 恒 true；占位文案明确「未执行」。
 */
export function directivePlaceholder(directive: LauncherDirective | null | undefined): DirectivePlaceholderView | null {
  if (!directive || !directive.action) return null;
  const note = "占位展示——打开/新建动作待工程桥接（UI-BRIDGE-1）后执行，本页不执行。";
  if (directive.action === "open_project") {
    const name = directive.project_name?.trim() || launcherDisplayFileName(directive.file_path ?? "");
    return {
      action: "open",
      headline: name ? `准备打开《${name}》…` : "准备打开工程…",
      detail: directive.file_path?.trim() ?? "",
      note
    };
  }
  if (directive.action === "new_project") {
    const name = directive.project_name?.trim();
    return {
      action: "new",
      headline: name ? `准备新建《${name}》…` : "准备新建工程…",
      detail: directive.reason?.trim() ?? "",
      note
    };
  }
  return { action: "unknown", headline: "准备执行 launcher 指令…", detail: "", note };
}

/** 结果卡点击（仅数据展示态切换）时的占位视图。 */
export function cardPendingPlaceholder(card: LauncherProjectCard): DirectivePlaceholderView {
  const fromDirective = directivePlaceholder(card.directive);
  if (fromDirective) return fromDirective;
  return {
    action: "open",
    headline: card.name ? `准备打开《${card.name}》…` : "准备打开工程…",
    detail: card.filePath,
    note: "占位展示——打开/新建动作待工程桥接（UI-BRIDGE-1）后执行，本页不执行。"
  };
}

/** agent 不可用（greeting 拉取失败）的固定文案。 */
export const LAUNCHER_AGENT_UNAVAILABLE_TEXT = "连不上 Vit agent——启动台暂时不可用。确认 VitAgent 已启动后重试。";
/** 最近工程列表拉取失败的固定文案。 */
export const LAUNCHER_RECENTS_ERROR_TEXT = "最近工程列表暂时读不到（内核未就绪或 agent 异常）。";
