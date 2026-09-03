import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { TopStatusBar } from "./App";

function renderBar(extras: { authorityMode?: string; authorityLocked?: boolean }) {
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
      authorityMode={extras.authorityMode as never}
      authorityLocked={extras.authorityLocked ?? false}
      onAuthorityModeChange={() => {}}
    />
  );
}

function onButtonLabel(markup: string) {
  const match = markup.match(/<button[^>]*class="on"[^>]*>([^<]+)<\/button>/);
  return match?.[1] ?? null;
}

describe("权限开关头部化（GUI-T4 ②；GUI-F1 副注条已移除）", () => {
  it("普通档：分段开关 普通 高亮，无副注条文案", () => {
    const markup = renderBar({ authorityMode: "manual_confirmation" });
    expect(markup).toContain('class="perm"');
    expect(markup).toContain("普通");
    expect(markup).toContain("完全");
    expect(onButtonLabel(markup)).toBe("普通");
    expect(markup).not.toContain("每步试验都经你确认");
    expect(markup).not.toContain("连续执行试验步");
  });

  it("完全档：完全 高亮，无副注条文案", () => {
    const markup = renderBar({ authorityMode: "full_project_access" });
    expect(onButtonLabel(markup)).toBe("完全");
    expect(markup).not.toContain("每步试验都经你确认");
    expect(markup).not.toContain("连续执行试验步");
  });

  it("锁定态（切换中/回合运行中）双档按钮禁用", () => {
    const markup = renderBar({ authorityMode: "manual_confirmation", authorityLocked: true });
    expect(markup).toContain('class="perm"');
    const disabledCount = (markup.match(/<button[^>]*disabled[^>]*>/g) ?? []).length;
    expect(disabledCount).toBeGreaterThanOrEqual(2);
  });

  it("头部结构：.top-status 头部，头部下方无副注条", () => {
    const markup = renderBar({ authorityMode: "manual_confirmation" });
    expect(markup).toContain('<header class="top-status"');
    expect(markup).not.toContain('class="subnote"');
  });
});
