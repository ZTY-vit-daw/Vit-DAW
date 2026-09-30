import type { ChatMessage } from "./types";

// OPT-OBSERVE-OUTPUT-1 P1（2026-09-30）：观察问答输出分层——长回复默认折叠。
// 设计锚：docs/OBSERVE_OUTPUT_LAYERING_V1_DESIGN.md §4.2/§5。P1 是 webui 单侧的
// 呈现重排（长度治理，不造摘要——真三层摘要由 P2 的 agent presentation 块产出），
// 判定只作用于纯文本正文；零 agent 改动，折叠是纯 UI 态（不进 content、不进
// 持久化，刷新/回放后默认态按本文件重算，v1 不记忆展开状态）。
//
// 保守原则（设计 §4.2-1，宁漏勿滥）：错折叠回执/交互卡=缺陷；漏折叠观察问答=
// 旧体验，可接受。故除长度阈值外，回执/交互卡族消息一概排除。

/** 折叠阈值：正文超过该字符数才考虑折叠（设计默认 600 字符）。 */
export const OBSERVE_OUTPUT_MAX_CHARS = 600;
/** 折叠阈值：正文超过该行数才考虑折叠（设计默认 14 行）。两个阈值满足其一即命中。 */
export const OBSERVE_OUTPUT_MAX_LINES = 14;
/** 折叠时默认可见的首部行数（首段超长时的截断行数）。 */
export const OBSERVE_OUTPUT_LEAD_LINES = 3;
/** 首段可直接作为可见段的最大行数；超过则退回首 LEAD_LINES 行。 */
export const OBSERVE_OUTPUT_LEAD_MAX_PARA_LINES = 4;

export interface ObserveOutputMessageShape {
  role: ChatMessage["role"];
  status?: ChatMessage["status"];
  message_kind?: ChatMessage["message_kind"];
  actions?: ChatMessage["actions"];
  content: string;
}

export interface ObserveOutputThresholds {
  maxChars: number;
  maxLines: number;
}

export const DEFAULT_OBSERVE_OUTPUT_THRESHOLDS: ObserveOutputThresholds = {
  maxChars: OBSERVE_OUTPUT_MAX_CHARS,
  maxLines: OBSERVE_OUTPUT_MAX_LINES
};

export interface ObserveOutputLayout {
  /** 默认可见的首段（或首 N 行）。 */
  lead: string;
  /** 折叠容器内的其余正文；展开后与现行渲染拼接即完整原文（无重复段）。 */
  rest: string;
  /** 折叠容器内的行数（summary 标签「展开全部 N 行」的 N）。 */
  hiddenLineCount: number;
}

// 回执/交互卡族：这些 kind 的正文是卡片体系的附言或回执文案，折叠命中=缺陷。
// message_kind 缺失（旧水合记录）按普通助手文本处理——fail-open 走 P1 呈现，
// 与设计 §4.3-2 对旧消息的退化口径一致。
export function isObserveOutputLayerableKind(messageKind: ChatMessage["message_kind"]): boolean {
  return messageKind === undefined || messageKind === null || messageKind === "assistant";
}

/**
 * 分层判定谓词（设计 §4.2-1）：
 * role=assistant ∧ 非 pending/error ∧ kind 属助手文本 ∧ 无交互卡 ∧ 正文过阈值
 * ∧ 能产出非空折叠段。
 */
export function shouldLayerObserveOutput(
  message: ObserveOutputMessageShape,
  thresholds: ObserveOutputThresholds = DEFAULT_OBSERVE_OUTPUT_THRESHOLDS
): boolean {
  if (message.role !== "assistant") {
    return false;
  }
  // pending（流式中）交回 TypingMessage 原样呈现，完成后才按本谓词定型；
  // error 文本属于故障面，不参与呈现治理。
  if (message.status === "pending" || message.status === "error") {
    return false;
  }
  if (!isObserveOutputLayerableKind(message.message_kind)) {
    return false;
  }
  // 交互卡族（含已结算卡）一概不动——卡片即该消息的呈现主体。
  if ((message.actions ?? []).length > 0) {
    return false;
  }
  const content = message.content ?? "";
  const charCount = content.length;
  const lineCount = content.split("\n").length;
  if (charCount <= thresholds.maxChars && lineCount <= thresholds.maxLines) {
    return false;
  }
  // 长度过线但切不出「首段+非空其余」（如无分段的长单行）时不折叠：折叠后无
  // 信息增益且首段=全文，属漏折叠侧，可接受。
  return observeOutputLayout(content) !== null;
}

/**
 * 折叠布局（设计 §4.2-2「首段（或首 N 行）可见 + details 展开全部」）：
 * 可见段=首段（按空行分段）；首段超过 LEAD_MAX_PARA_LINES 行时退回首 LEAD_LINES 行。
 * 其余正文去掉边界空行后整体进折叠容器。切不出非空其余时返回 null（不折叠）。
 */
export function observeOutputLayout(content: string): ObserveOutputLayout | null {
  const lines = (content ?? "").split("\n");
  let start = 0;
  while (start < lines.length && lines[start].trim() === "") {
    start += 1;
  }
  if (start >= lines.length) {
    return null;
  }
  let paragraphEnd = start;
  while (paragraphEnd < lines.length && lines[paragraphEnd].trim() !== "") {
    paragraphEnd += 1;
  }
  const leadEnd = paragraphEnd - start <= OBSERVE_OUTPUT_LEAD_MAX_PARA_LINES
    ? paragraphEnd
    : start + OBSERVE_OUTPUT_LEAD_LINES;
  const lead = lines.slice(start, leadEnd).join("\n");
  let restStart = leadEnd;
  while (restStart < lines.length && lines[restStart].trim() === "") {
    restStart += 1;
  }
  const restLines = lines.slice(restStart);
  const rest = restLines.join("\n");
  if (rest.trim() === "") {
    return null;
  }
  return { lead, rest, hiddenLineCount: restLines.length };
}
