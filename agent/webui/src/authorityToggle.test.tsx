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

// GUI-F3（2026-09-04 用户第三笔裁定）：权限切换是点开选择的下拉选择器
// （对齐 ZCode 模型选择器形态），不是两档左右分段开关。收起态只显示当前
// 档胶囊键；菜单在客户端交互时才展开（SSR 标记收起态结构与可访问性语义）。
describe("权限下拉选择器（AuthorityToggle）", () => {
  it("收起态：显示当前档名 + 箭头，不渲染两档分段开关", () => {
    const markup = renderToggle({ authorityMode: "manual_confirmation" });
    expect(markup).toContain('class="authority-select-button"');
    expect(markup).toContain("普通确认");
    expect(markup).not.toContain("完全访问");
    expect(markup).not.toContain('class="perm"');
  });

  it("完全访问档：胶囊键显示完全访问", () => {
    const markup = renderToggle({ authorityMode: "full_project_access" });
    expect(markup).toContain("完全访问");
  });

  it("锁定态（回合进行中）禁用并说明原因", () => {
    const markup = renderToggle({ authorityMode: "manual_confirmation", authorityLocked: true });
    expect(markup).toContain("任务运行中暂不能切换权限");
    expect(markup).toMatch(/<button[^>]*disabled[^>]*>/);
  });

  it("空闲态给出权限语义提示与菜单语义标记", () => {
    const markup = renderToggle({ authorityMode: "manual_confirmation" });
    expect(markup).toContain("选择工程动作的执行权限");
    expect(markup).toContain('aria-haspopup="menu"');
  });
});
