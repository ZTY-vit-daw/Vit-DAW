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

describe("权限开关头部化（GUI-T4 ②）", () => {
  it("普通档：分段开关 普通 高亮，副注为逐确认文案", () => {
    const markup = renderBar({ authorityMode: "manual_confirmation" });
    expect(markup).toContain('class="perm"');
    expect(markup).toContain('class="on"');
    expect(markup).toContain("普通");
    expect(markup).toContain("完全");
    expect(markup).toContain("每步试验都经你确认 · 每步可回滚");
    expect(markup).not.toContain("连续执行试验步");
  });

  it("完全档：完全 高亮，副注切换为连续执行文案", () => {
    const markup = renderBar({ authorityMode: "full_project_access" });
    expect(markup).toContain("连续执行试验步 · 每步仍可回滚");
    expect(markup).not.toContain("每步试验都经你确认");
  });

  it("锁定态（切换中/回合运行中）双档按钮禁用", () => {
    const markup = renderBar({ authorityMode: "manual_confirmation", authorityLocked: true });
    expect(markup).toContain('class="perm"');
    const disabledCount = (markup.match(/<button[^>]*disabled[^>]*>/g) ?? []).length;
    expect(disabledCount).toBeGreaterThanOrEqual(2);
  });

  it("头部结构：.top-status 头部 + 头部下方副注条（黄底墨字）", () => {
    const markup = renderBar({ authorityMode: "manual_confirmation" });
    expect(markup).toContain('<header class="top-status"');
    expect(markup).toContain('class="subnote"');
    expect(markup.indexOf('class="subnote"')).toBeGreaterThan(markup.indexOf("</header>"));
  });
});
