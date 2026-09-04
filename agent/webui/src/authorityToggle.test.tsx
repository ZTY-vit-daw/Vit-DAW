import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { AuthorityToggle } from "./App";

function renderToggle(extras: { authorityMode: string; authorityLocked?: boolean }) {
  return renderToStaticMarkup(
    <AuthorityToggle
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

// GUI-F3（2026-09-04 用户裁定）：权限开关从顶栏迁到对话输入框 + 号旁，
// 对齐 DSH/ZCode 等 agent 的常驻入口习惯；锁定态给出原因提示。
describe("权限开关（输入框旁常驻，AuthorityToggle）", () => {
  it("普通档：普通 高亮", () => {
    const markup = renderToggle({ authorityMode: "manual_confirmation" });
    expect(markup).toContain('class="perm"');
    expect(markup).toContain("普通");
    expect(markup).toContain("完全");
    expect(onButtonLabel(markup)).toBe("普通");
  });

  it("完全档：完全 高亮", () => {
    const markup = renderToggle({ authorityMode: "full_project_access" });
    expect(onButtonLabel(markup)).toBe("完全");
  });

  it("锁定态（回合进行中/切换中）双档禁用并说明原因", () => {
    const markup = renderToggle({ authorityMode: "manual_confirmation", authorityLocked: true });
    expect(markup).toContain('class="perm"');
    expect(markup).toContain("任务运行中暂不能切换权限");
    const disabledCount = (markup.match(/<button[^>]*disabled[^>]*>/g) ?? []).length;
    expect(disabledCount).toBeGreaterThanOrEqual(2);
  });

  it("空闲态给出权限语义提示", () => {
    const markup = renderToggle({ authorityMode: "manual_confirmation" });
    expect(markup).toContain("控制可逆工程动作是否逐项请求确认");
  });
});
