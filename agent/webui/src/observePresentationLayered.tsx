import type { ChatMessage, ObservePresentation } from "./types";

// observePresentationLayered.tsx — OPT-IMPL-2 P2（2026-10-07）：观察问答输出
// 真三层渲染。设计锚 docs/OBSERVE_OUTPUT_LAYERING_V1_DESIGN.md §4.1/§4.3：
//   摘要层（≤3 行结论）常显；证据层（观察条目，含 source/ref 追溯入口）折叠；
//   完整回复原文（摘要段之后的正文）最深折叠——P1「展开全部」精神保留，零信息丢失。
// 有 presentation 且有效（detail_mode=layered）的消息走本渲染；否则回落既有
// P1 谓词路径（observeOutputLayering.ts）。折叠是纯 UI 态，不进 content/持久化。

export function hasLayeredPresentation(
  message: Pick<ChatMessage, "presentation">
): boolean {
  return message.presentation !== undefined && message.presentation !== null;
}

function restAfterSummary(content: string, summary: string): string {
  const normalized = content.replace(/\r\n/g, "\n");
  const index = normalized.indexOf(summary);
  if (index >= 0) {
    return normalized.slice(index + summary.length).trim();
  }
  return normalized.trim();
}

export function ObservePresentationView({
  message
}: {
  message: ChatMessage;
}) {
  const presentation: ObservePresentation = message.presentation!;
  const rest = restAfterSummary(message.content, presentation.summary);
  const entries = presentation.evidence_entries ?? [];
  return (
    <div className="observe-presentation-layered">
      <p className="observe-presentation-summary">{presentation.summary}</p>
      <details className="observe-presentation-evidence">
        <summary>
          观察证据 {entries.length} 条（点选展开）
        </summary>
        <ul className="observe-presentation-evidence-list">
          {entries.map((entry, index) => (
            <li key={index} className="observe-presentation-evidence-item">
              <span className="observe-presentation-evidence-text">{entry.text}</span>
              {entry.source || entry.ref ? (
                <span className="observe-presentation-evidence-source">
                  {[
                    entry.source ? `来源 ${entry.source}` : "",
                    entry.ref ? `引用 ${entry.ref}` : ""
                  ]
                    .filter(Boolean)
                    .join(" · ")}
                </span>
              ) : null}
            </li>
          ))}
        </ul>
      </details>
      {rest !== "" ? (
        <details className="observe-presentation-fulltext">
          <summary>展开完整回复</summary>
          <p>{rest}</p>
        </details>
      ) : null}
    </div>
  );
}
