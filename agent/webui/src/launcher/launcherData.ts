/*
 * UI-LAUNCHER-1 launcher 面纯数据助手（无副作用，供单元测试与渲染面复用）。
 *
 * 纪律（卡面）：不硬编码示例工程名——建议词全是动词/通用句式，
 * 占位文案不冒充「已打开」；fixture 数据只存在于 gallery/render-smoke。
 */

import type { LauncherDirective, LauncherGreeting, LauncherProjectCard } from "../api/launcher";

/** 输入框上方的上下文建议（3–4 个，确定性意图词面，见 launcher_flow.go 分类器）。
 * SHELL-5：新建建议=直接执行语义（agent 出缺省草稿名 directive，不再问询循环）。 */
export const LAUNCHER_SUGGESTIONS: ReadonlyArray<{ text: string; hint: string }> = [
  { text: "看看最近的工程", hint: "列出最近工程卡片" },
  { text: "新建一个工程", hint: "直接新建并进入工程（缺省名，进去后可改）" },
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

/** UI-BRIDGE-1：桥执行态（waiting 起于请求发出；终态一律来自宿主 vit_launcher_result）。 */
export type LauncherDirectiveState =
  | "pending"
  | "waiting"
  | "opened"
  | "failed"
  | "canceled"
  | "unsupported"
  | "invalid"
  | "dialog_shown";

export interface DirectivePlaceholderView {
  action: LauncherPendingAction;
  /** 占位标题：「正在打开《X》…」/「正在新建《X》…」（SHELL-5 自动执行语态）。 */
  headline: string;
  /** 数据行：文件路径 / 工程名（原样呈报）。 */
  detail: string;
  /** 请求提交后的说明；不把占位说明当作成功回执。 */
  note: string;
}

/**
 * launcher_directive → 自动执行回执占位（SHELL-5：directive 到达即自动经
 * Vit 桥执行——不再等「让 Vit 新建/打开」手动按钮；本视图只呈报执行中事实，
 * 终态一律以下方桥执行态（宿主 vit_launcher_result）为准，不虚报已打开）。
 * agent 侧 requires_governance 恒 true；治理确认与原生对话框归属宿主
 * （VitShell 版：Win32 对话框 + 页内治理卡；Godot 旧宿主同文案成立——归属
 * 都是「宿主」，不点名具体宿主实现）。
 */
export function directivePlaceholder(directive: LauncherDirective | null | undefined): DirectivePlaceholderView | null {
  if (!directive || !directive.action) return null;
  const note = "工程操作请求已提交，进度见下方；切换工程前会请你确认。";
  if (directive.action === "open_project") {
    const name = directive.project_name?.trim() || launcherDisplayFileName(directive.file_path ?? "");
    return {
      action: "open",
      headline: name ? `正在打开《${name}》…` : "正在打开工程…",
      detail: directive.file_path?.trim() ?? "",
      note
    };
  }
  if (directive.action === "new_project") {
    const name = directive.project_name?.trim();
    return {
      action: "new",
      headline: name ? `正在新建《${name}》…` : "正在新建工程…",
      detail: directive.reason?.trim() ?? "",
      note
    };
  }
  return { action: "unknown", headline: "正在执行 launcher 指令…", detail: "", note };
}

/** 结果卡点击（自动执行同一指令）时的占位视图。 */
export function cardPendingPlaceholder(card: LauncherProjectCard): DirectivePlaceholderView {
  const fromDirective = directivePlaceholder(card.directive);
  if (fromDirective) return fromDirective;
  return {
    action: "open",
    headline: card.name ? `正在打开《${card.name}》…` : "正在打开工程…",
    detail: card.filePath,
    note: "工程操作请求已提交，进度见下方；切换工程前会请你确认。"
  };
}

/** 缺省新工程名（SHELL-5：MMDD_HHmm 时间戳草稿名；agent 侧
 * launcherDefaultNewProjectName 同格式镜像——名实相符的内核承接是 363 域）。 */
export function defaultNewProjectName(now: number = Date.now()): string {
  const d = new Date(now);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `新工程_${pad(d.getMonth() + 1)}${pad(d.getDate())}_${pad(d.getHours())}${pad(d.getMinutes())}`;
}

/** 启动台「＋新建」按钮的直连 directive（不经对话问询——点击即建并进入）。 */
export function buildDirectNewDirective(now: number = Date.now()): LauncherDirective {
  return {
    action: "new_project",
    project_name: defaultNewProjectName(now),
    reason: "新建工程（启动台「＋新建」按钮）",
    requires_governance: true
  };
}

/** agent 不可用（greeting 拉取失败）的固定文案。 */
export const LAUNCHER_AGENT_UNAVAILABLE_TEXT = "连不上 Vit agent——启动台暂时不可用。确认 VitAgent 已启动后重试。";
/** 最近工程列表拉取失败的固定文案。 */
export const LAUNCHER_RECENTS_ERROR_TEXT = "最近工程列表暂时读不到（内核未就绪或 agent 异常）。";

// ── UI-BRIDGE-1：Vit 桥呈现文案（全 honest——终态只陈述宿主回执，不虚报已打开）──

/** 桥不可用（非 Vit 窗口/chrome.webview 缺失）的固定文案。 */
export const LAUNCHER_BRIDGE_UNAVAILABLE_TEXT =
  "Vit 桥不可用（当前不在 Vit 窗口内）——打开/新建请在 Vit 窗口的启动台操作，本页不会假装执行。";

/** 桥请求已发出（waiting 态）的固定文案。 */
export const LAUNCHER_BRIDGE_WAITING_TEXT =
  "已发送到 Vit——等待宿主执行（有在开工程时会弹出治理确认，打开工程会走原生对话框）。";

/** choose_file 回执 dialog_shown 的固定文案（原生 FileDialog 归属 Godot）。 */
export const LAUNCHER_BRIDGE_DIALOG_SHOWN_TEXT =
  "Vit 已弹出原生文件对话框——请在对话框中选择工程文件（取消则不打开）。";

/**
 * 宿主回执 → 终态文案（只陈述回执事实；未知 status 按 failed 呈现原始消息，
 * 不静默、不美化）。
 */
export function launcherDirectiveResultText(status: string, message: string, action: string): string {
  const verbOpen = action === "new_project" ? "新建" : "打开";
  if (status === "opened") return message ? `Vit 已完成${verbOpen}（${message}）` : `Vit 已完成${verbOpen}。`;
  if (status === "canceled") return "已在治理确认中取消——未切换工程。";
  if (status === "unsupported") return message || "当前在工程内——跨工程打开/新建暂未开放（宿主降级）。";
  if (status === "dialog_shown") return LAUNCHER_BRIDGE_DIALOG_SHOWN_TEXT;
  return message ? `${verbOpen}未完成（${message}）` : `${verbOpen}未完成（宿主未给出原因）。`;
}

/** 终态是否允许重试（canceled/unsupported/dialog_shown 无需重试钮）。 */
export function launcherDirectiveResultRetryable(status: string): boolean {
  return status === "failed" || status === "invalid";
}

// ── SHELL-2：页内治理确认卡（跟随发起方——agent/对话发起 → React 三键卡）──

/** 页内治理卡数据视图（宿主 vit_governance_required → 本视图；对话流内呈现）。 */
export interface LauncherGovernanceView {
  requestID: string;
  action: string;
  /** 在开工程显示名（宿主回执原样；态未知=空）。 */
  currentProject: string;
}

/** 治理卡标题（v5.1 口径：保守确认，宁多勿漏；显示名只陈述宿主回执）。 */
export function launcherGovernanceHeadline(view: { currentProject: string }): string {
  const name = view.currentProject.trim();
  return name
    ? `当前工程《${name}》还在进行中——切换前要处理它吗？`
    : "当前可能有工程在进行（状态未知）——切换前要处理它吗？";
}

/** 三键固定文案（与 Godot LauncherGovernanceDialog 同语义三键）。 */
export const LAUNCHER_GOVERNANCE_SAVE_TEXT = "保存并切换";
export const LAUNCHER_GOVERNANCE_SWITCH_TEXT = "直接切换";
export const LAUNCHER_GOVERNANCE_CANCEL_TEXT = "取消";

/** 已作答后的固定呈现（不重放选择，不虚报宿主已执行）。 */
export function launcherGovernanceAnsweredText(choice: string): string {
  if (choice === "save") return "已选择「保存并切换」——等待 Vit 执行。";
  if (choice === "switch") return "已选择「直接切换」——等待 Vit 执行。";
  return "已取消——未切换工程。";
}
