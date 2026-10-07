import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { AssistantTextContent } from "./App";
import {
  hasLayeredPresentation,
  ObservePresentationView
} from "./observePresentationLayered";
import {
  historyMessageProtocol,
  normalizeObservePresentation,
  responseMessageProtocol
} from "./messageLifecycle";
import type { ChatMessage, ChatResponse, ObservePresentation } from "./types";

// OPT-IMPL-2 P2（2026-10-07）：真三层渲染+fail-open 兼容反例。
// 设计锚 docs/OBSERVE_OUTPUT_LAYERING_V1_DESIGN.md §4.1/§4.3——有块走
// 摘要常显+证据折叠+完整原文深折叠；无块/未知 detail_mode/缺证据回落 P1。

function presentation(overrides: Partial<ObservePresentation> = {}): ObservePresentation {
  return {
    summary: "Lead Vocal 与 Bass 在 110-160Hz 存在约 3.1dB 掩蔽风险。",
    evidence_entries: [
      { text: "track_1007 lufs=-18.4", source: "mix.observe", ref: "obs_20261007T210000_ab12" },
      { text: "band energy summary: sub 偏高", source: "ccb.observation_request" }
    ],
    detail_mode: "layered",
    ...overrides
  };
}

function message(overrides: Partial<ChatMessage>): ChatMessage {
  return {
    id: "msg_test",
    role: "assistant",
    content: "",
    createdAt: 0,
    status: "sent",
    message_kind: "assistant",
    ...overrides
  };
}

const layeredContent =
  "Lead Vocal 与 Bass 在 110-160Hz 存在约 3.1dB 掩蔽风险。\n\n详细分析：频段能量显示 Bass 基频集中在 92Hz，Lead Vocal 低频延伸到 110Hz 以下。\n建议后续用 A/B 验证。";

describe("normalizeObservePresentation 兼容反例", () => {
  it("合法块原样归一（detail_mode=layered）", () => {
    const normalized = normalizeObservePresentation(presentation());
    expect(normalized?.summary).toContain("3.1dB");
    expect(normalized?.evidence_entries).toHaveLength(2);
    expect(normalized?.evidence_entries?.[0].ref).toBe("obs_20261007T210000_ab12");
  });

  it("旧消息无块/非对象/缺 summary/空证据 → undefined（不报错不升级）", () => {
    expect(normalizeObservePresentation(undefined)).toBeUndefined();
    expect(normalizeObservePresentation("layered")).toBeUndefined();
    expect(normalizeObservePresentation({ detail_mode: "layered" })).toBeUndefined();
    expect(normalizeObservePresentation({ summary: "结论", detail_mode: "layered", evidence_entries: [] })).toBeUndefined();
    expect(
      normalizeObservePresentation({ summary: "结论", detail_mode: "layered", evidence_entries: [{ text: "  " }] })
    ).toBeUndefined();
  });

  it("未知 detail_mode 按无块处理（设计 §4.3-2）", () => {
    expect(normalizeObservePresentation(presentation({ detail_mode: "v2_fancy" }))).toBeUndefined();
  });

  it("responseMessageProtocol / historyMessageProtocol 携带 presentation", () => {
    const response = { presentation: presentation() } as ChatResponse;
    expect(responseMessageProtocol(response, "内容").presentation?.detail_mode).toBe("layered");
    const row = { node_id: "n1", presentation: presentation() };
    expect(historyMessageProtocol(row, "assistant").presentation?.summary).toContain("3.1dB");
    expect(historyMessageProtocol({ node_id: "n2" }, "assistant").presentation).toBeUndefined();
  });
});

describe("AssistantTextContent 真三层渲染", () => {
  it("有块渲染摘要常显+证据折叠+完整原文深折叠（details 默认收起）", () => {
    const markup = renderToStaticMarkup(
      <AssistantTextContent message={message({ content: layeredContent, presentation: presentation() })} />
    );
    expect(markup).toContain("observe-presentation-layered");
    expect(markup).toContain("observe-presentation-summary");
    expect(markup).toContain("3.1dB");
    expect(markup).toContain("observe-presentation-evidence");
    expect(markup).toContain("观察证据 2 条");
    expect(markup).toContain("展开完整回复");
    expect(markup).toContain("详细分析");
    expect(markup).not.toMatch(/<details[^>]*\bopen\b/);
    // 追溯入口：source+ref 行在场。
    expect(markup).toContain("来源 mix.observe");
    expect(markup).toContain("引用 obs_20261007T210000_ab12");
  });

  it("有块优先于 P1 谓词（不再输出 observe-output-layered 容器）", () => {
    const longContent = layeredContent + "\n" + "细".repeat(600);
    const markup = renderToStaticMarkup(
      <AssistantTextContent message={message({ content: longContent, presentation: presentation() })} />
    );
    expect(markup).toContain("observe-presentation-layered");
    expect(markup).not.toContain("observe-output-layered");
  });

  it("无块回落既有 P1 谓词路径（长文本 observe-output-layered）", () => {
    const longContent = "结论首段。\n\n" + "细".repeat(600);
    const markup = renderToStaticMarkup(<AssistantTextContent message={message({ content: longContent })} />);
    expect(markup).toContain("observe-output-layered");
    expect(markup).not.toContain("observe-presentation-layered");
  });

  it("summary 之外的正文进「展开完整回复」，与摘要无重复段", () => {
    const markup = renderToStaticMarkup(
      <ObservePresentationView message={message({ content: layeredContent, presentation: presentation() })} />
    );
    const rest = layeredContent.slice(layeredContent.indexOf("\n\n") + 2);
    expect(markup).toContain(rest);
  });

  it("hasLayeredPresentation 谓词边界", () => {
    expect(hasLayeredPresentation({ presentation: presentation() })).toBe(true);
    expect(hasLayeredPresentation({})).toBe(false);
    expect(hasLayeredPresentation({ presentation: undefined })).toBe(false);
  });
});
