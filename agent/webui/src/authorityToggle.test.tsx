import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { authoritySwitchNotice, AuthorityToggle } from "./App";

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

// UX-1（2026-09-05 任务卡）：权限切换收到 409 authority_mode_change_blocked
// （含 stop_turn_prevents_new_action 等占用类错误码）时，复用既有 error 横幅
// 呈现人话原因；其它错误原样透传不清吞；正常切换不产生任何提示。
describe("权限切换被拒的人话提示（UX-1）", () => {
  it("409 占用类拒绝：呈现人话原因而非机器报错", () => {
    const blocked = "任务占用工程面中，结束后可切换";
    expect(
      authoritySwitchNotice({ ok: false, error: new Error("authority mode cannot change while an Agent Turn is running") })
    ).toBe(blocked);
    expect(
      authoritySwitchNotice({ ok: false, error: new Error("authority_mode_change_blocked: authority mode cannot change") })
    ).toBe(blocked);
    expect(authoritySwitchNotice({ ok: false, error: new Error("stop_turn_prevents_new_action") })).toBe(blocked);
  });

  it("其它错误不清吞：原样透传原始信息", () => {
    expect(authoritySwitchNotice({ ok: false, error: new Error("网络中断") })).toBe("网络中断");
    expect(authoritySwitchNotice({ ok: false, error: "invalid JSON" })).toBe("invalid JSON");
  });

  it("正常切换：无提示", () => {
    expect(authoritySwitchNotice({ ok: true })).toBeNull();
  });
});
