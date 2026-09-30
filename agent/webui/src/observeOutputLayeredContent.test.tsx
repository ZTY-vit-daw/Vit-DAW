import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { AssistantTextContent } from "./App";
import type { ChatMessage } from "./types";

// OPT-OBSERVE-OUTPUT-1 P1：折叠容器渲染形态（命中谓词=details 默认收起；
// 未命中=现行单 <p>）。谓词边界本身在 observeOutputLayering.test.ts。

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

const longLayered = "结论首段。\n\n" + "细".repeat(600);

describe("AssistantTextContent 折叠容器", () => {
  it("命中谓词渲染为 lead + details（默认不带 open 属性），summary 含展开行数", () => {
    const markup = renderToStaticMarkup(<AssistantTextContent message={message({ content: longLayered })} />);
    expect(markup).toContain("observe-output-layered");
    expect(markup).toContain("observe-output-lead");
    expect(markup).toContain("observe-output-details");
    expect(markup).not.toMatch(/<details[^>]*\bopen\b/);
    expect(markup).toContain("展开全部");
    expect(markup).toContain("结论首段。");
    expect(markup).toContain("细");
  });

  it("折叠容器内正文为其余段落，lead+rest 拼回原文（无重复段）", () => {
    const markup = renderToStaticMarkup(<AssistantTextContent message={message({ content: longLayered })} />);
    expect(markup).toContain(longLayered.slice("结论首段。".length + 2));
  });

  it("未命中谓词（短文本）渲染为现行单 <p>", () => {
    const markup = renderToStaticMarkup(<AssistantTextContent message={message({ content: "短回复" })} />);
    expect(markup).toContain("<p>短回复</p>");
    expect(markup).not.toContain("observe-output");
  });

  it("回执/交互卡形态（kind 排除或 actions 非空）渲染为现行单 <p>", () => {
    const receipt = renderToStaticMarkup(
      <AssistantTextContent message={message({ content: longLayered, message_kind: "execution_receipt" })} />
    );
    expect(receipt).toContain("<p>");
    expect(receipt).not.toContain("observe-output");
    const interactive = renderToStaticMarkup(
      <AssistantTextContent message={message({ content: longLayered, actions: [{ id: "approve" }] })} />
    );
    expect(interactive).toContain("<p>");
    expect(interactive).not.toContain("observe-output");
  });
});
