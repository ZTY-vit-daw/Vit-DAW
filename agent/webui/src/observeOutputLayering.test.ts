import { describe, expect, it } from "vitest";
import {
  DEFAULT_OBSERVE_OUTPUT_THRESHOLDS,
  observeOutputLayout,
  shouldLayerObserveOutput,
  type ObserveOutputMessageShape
} from "./observeOutputLayering";

// OPT-OBSERVE-OUTPUT-1 P1：判定谓词与折叠布局的边界用例（设计 §7——长度阈值/
// 交互卡排除/回执排除）。保守口径：错折叠=缺陷，漏折叠可接受。

function shape(overrides: Partial<ObserveOutputMessageShape>): ObserveOutputMessageShape {
  return {
    role: "assistant",
    status: "sent",
    message_kind: "assistant",
    content: "",
    ...overrides
  };
}

// 首段 + 空行 + 指定长度的第二段：保证 layout 切得出非空 rest。
function layeredContent(secondParaChars: number): string {
  const lead = "结论首段。";
  return lead + "\n\n" + "细".repeat(secondParaChars);
}

describe("shouldLayerObserveOutput 长度阈值边界", () => {
  it("短文本不折叠（字符与行数均低于阈值）", () => {
    expect(shouldLayerObserveOutput(shape({ content: "结论首段。\n\n证据细节。" }))).toBe(false);
  });

  it("正文恰为 600 字符不折叠（阈值边界，闭口）", () => {
    const content = layeredContent(600 - 5 - 2);
    expect(content.length).toBe(600);
    expect(shouldLayerObserveOutput(shape({ content }))).toBe(false);
  });

  it("正文 601 字符折叠（阈值边界，开口）", () => {
    const content = layeredContent(601 - 5 - 2);
    expect(content.length).toBe(601);
    expect(shouldLayerObserveOutput(shape({ content }))).toBe(true);
  });

  it("正文恰 14 行不折叠（行数阈值边界，字符低于阈值）", () => {
    const content = Array.from({ length: 14 }, (_, index) => `行${index}`).join("\n");
    expect(content.length).toBeLessThan(DEFAULT_OBSERVE_OUTPUT_THRESHOLDS.maxChars);
    expect(shouldLayerObserveOutput(shape({ content }))).toBe(false);
  });

  it("正文 15 行折叠（行数阈值边界，字符低于阈值）", () => {
    const content = Array.from({ length: 15 }, (_, index) => `行${index}`).join("\n");
    expect(shouldLayerObserveOutput(shape({ content }))).toBe(true);
  });

  it("长度过线但切不出非空折叠段（无分段长单行）不折叠——漏折叠侧可接受", () => {
    expect(shouldLayerObserveOutput(shape({ content: "长".repeat(601) }))).toBe(false);
  });
});

describe("shouldLayerObserveOutput 消息面排除（宁漏勿滥）", () => {
  const content = layeredContent(600);

  it("非 assistant 角色不折叠", () => {
    expect(shouldLayerObserveOutput(shape({ role: "user", content }))).toBe(false);
    expect(shouldLayerObserveOutput(shape({ role: "system", content }))).toBe(false);
  });

  it("pending（流式中）与 error 不折叠", () => {
    expect(shouldLayerObserveOutput(shape({ status: "pending", content }))).toBe(false);
    expect(shouldLayerObserveOutput(shape({ status: "error", content }))).toBe(false);
  });

  it("回执/交互卡族 message_kind 一概不折叠", () => {
    for (const message_kind of ["proposal", "execution_receipt", "verification", "warning", "error", "activity", "system"] as const) {
      expect(shouldLayerObserveOutput(shape({ message_kind, content })), message_kind).toBe(false);
    }
  });

  it("message_kind 缺失（旧水合记录）按助手文本 fail-open 处理", () => {
    expect(shouldLayerObserveOutput(shape({ message_kind: undefined, content }))).toBe(true);
  });

  it("携带交互卡（actions 非空）的长文本不折叠", () => {
    expect(shouldLayerObserveOutput(shape({
      content,
      actions: [{ id: "approve", status: "waiting_for_user" }]
    }))).toBe(false);
  });
});

describe("observeOutputLayout 折叠布局", () => {
  it("首段作可见段，其余进折叠容器，拼接还原完整原文", () => {
    const content = "首段第一行\n首段第二行\n\n证据 A\n证据 B\n证据 C";
    const layout = observeOutputLayout(content);
    expect(layout).not.toBeNull();
    expect(layout!.lead).toBe("首段第一行\n首段第二行");
    expect(layout!.rest).toBe("证据 A\n证据 B\n证据 C");
    expect(layout!.hiddenLineCount).toBe(3);
    expect(layout!.lead + "\n\n" + layout!.rest).toBe(content);
  });

  it("边界空行归入折叠侧并去除，不产生空白首行", () => {
    const layout = observeOutputLayout("首段\n\n\n证据 A");
    expect(layout!.lead).toBe("首段");
    expect(layout!.rest).toBe("证据 A");
  });

  it("首段超长时退回首 3 行作为可见段", () => {
    const layout = observeOutputLayout("一\n二\n三\n四\n五\n\n证据 A");
    expect(layout!.lead).toBe("一\n二\n三");
    expect(layout!.rest.startsWith("四")).toBe(true);
  });

  it("切不出非空折叠段（单段全文）返回 null", () => {
    expect(observeOutputLayout("单段全文")).toBeNull();
    expect(observeOutputLayout("")).toBeNull();
    expect(observeOutputLayout("\n\n")).toBeNull();
  });
});
