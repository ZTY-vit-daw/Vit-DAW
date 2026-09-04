import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { TopStatusBar } from "./App";

function renderBar() {
  return renderToStaticMarkup(
    <TopStatusBar
      connection="ready"
      healthLabel="ok"
      runtimeStatus={null}
      uiState={null}
      conversationTitle="对话"
      onOpenHistory={() => {}}
      onRefresh={async () => {}}
      onTransportCommand={async () => {}}
      transportBusy={false}
    />
  );
}

describe("顶栏（GUI-F3 起权限开关迁至输入框 + 号旁，顶栏不再承载）", () => {
  it("头部结构：.top-status 头部，无副注条", () => {
    const markup = renderBar();
    expect(markup).toContain('<header class="top-status"');
    expect(markup).not.toContain('class="subnote"');
  });

  it("权限开关不再出现在顶栏（迁至 Composer）", () => {
    const markup = renderBar();
    expect(markup).not.toContain('class="perm"');
    expect(markup).not.toContain("普通");
    expect(markup).not.toContain("完全");
  });
});
