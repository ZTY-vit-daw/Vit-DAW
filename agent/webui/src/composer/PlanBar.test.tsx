import { renderToStaticMarkup } from "react-dom/server";
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { mockTaskTrajectorySnapshot } from "../trajectoryMock";
import { PlanBar, planBarDwellMs, planBarFadeMs, planBarPhase, planBarRetracted, planBarRetracts } from "./PlanBar";

describe("PlanBar 规划条（GUI-T6）", () => {
  it("有 snapshot：收起窄条含状态标签/意图摘要/当前步骤/Task id，默认收起", () => {
    const snapshot = mockTaskTrajectorySnapshot();
    expect(snapshot).not.toBeNull();
    const markup = renderToStaticMarkup(
      <PlanBar snapshot={snapshot} plan={{ current_step: "观察掩蔽关系" }} />
    );
    expect(markup).toContain('aria-label="任务规划"');
    expect(markup).toContain("正在观察工程");
    expect(markup).toContain("改善主唱清晰度，不明显增加亮度。");
    expect(markup).toContain("观察掩蔽关系");
    expect(markup).toContain("Task mock-task");
    expect(markup).toContain('aria-expanded="false"');
    expect(markup).not.toContain("is-open");
  });

  it("展开态：原始意图全文 + semantic summary + 容量评估 + 工程修订 + 证据引用", () => {
    const markup = renderToStaticMarkup(
      <PlanBar snapshot={mockTaskTrajectorySnapshot()} plan={{ current_step: "观察掩蔽关系" }} defaultOpen />
    );
    expect(markup).toContain('aria-expanded="true"');
    expect(markup).toContain("is-open");
    expect(markup).toContain("改善主唱清晰度，不明显增加亮度。");
    expect(markup).toContain("正在观察掩蔽关系与电平结构。");
    expect(markup).toContain("保留在自由态");
    expect(markup).toContain("within_free_state");
    expect(markup).toContain("工程修订 revision-3");
    expect(markup).toContain('aria-label="证据引用"');
    expect(markup).toContain("project.state:revision-3");
  });

  it("无 snapshot：goal/plan 回退窄条——英文状态走中文标签，current_step 本地化", () => {
    const markup = renderToStaticMarkup(
      <PlanBar
        snapshot={null}
        goal={{ status: "running", summary: "跟踪人声清晰度任务" }}
        plan={{ current_step: "观察掩蔽关系" }}
      />
    );
    expect(markup).toContain('aria-label="任务规划"');
    expect(markup).toContain("执行中");
    expect(markup).toContain("跟踪人声清晰度任务");
    expect(markup).toContain("观察掩蔽关系");
    expect(markup).not.toContain("Task ");
  });

  it("空任务：无 snapshot 且 goal/plan 无内容 → 整条不渲染（idle 视为空）", () => {
    expect(renderToStaticMarkup(<PlanBar snapshot={null} />)).toBe("");
    expect(renderToStaticMarkup(<PlanBar snapshot={null} goal={{ status: "idle" }} />)).toBe("");
    expect(renderToStaticMarkup(<PlanBar snapshot={null} goal={{ status: "idle" }} plan={{ current_step: "" }} />)).toBe("");
  });

  it("挂载与数据线：App.tsx 渲染 PlanBar 且喂 taskTrajectoryState.snapshot；taskSnapshot 链与 TaskTrajectoryView 死代码移除", () => {
    const appSource = readFileSync(new URL("../App.tsx", import.meta.url), "utf8");
    expect(appSource).toMatch(/<PlanBar\b/);
    expect(appSource).toContain("taskTrajectoryState.snapshot");
    expect(appSource).not.toContain("taskSnapshot");
    expect(appSource).not.toMatch(/<TaskTrajectoryView\b/);
  });
});

// ---------------------------------------------------------------- PLANBAR-1
// 2026-09-13（手测命中·用户裁定）：缺陷①栏渲染在输入框浮层后方（设计=输入框
// 上方）②终态常驻不清场。这里的断言把两条修法钉成机器可判的契约：
//   * 位置契约——栏与 composer 同挂 .composer-dock 停靠列、栏在 DOM 序上先于
//     composer。列向 flex 容器的渲染序即几何序，故 bar.bottom + gap <= composer.top
//     由布局保证（真实视口坐标由渲染面交付门量取，见 scripts/webui_rendered_dom_smoke.mjs）。
//   * 生命周期——相位分类 + 有限时间收起时间表；无活链支撑的非终态不得带 live 呈现。
describe("PlanBar 位置契约（PLANBAR-1 缺陷①：恢复 T6 设计位=输入框上方）", () => {
  const appSource = readFileSync(new URL("../App.tsx", import.meta.url), "utf8");
  const css = readFileSync(new URL("./planbar.css", import.meta.url), "utf8");
  const baseCss = readFileSync(new URL("../styles.css", import.meta.url), "utf8");

  function cssRule(source: string, selector: string): string {
    const pattern = new RegExp(selector.replace(/[.*+?^${}()|[\]\\]/g, "\\$&") + "\\s*\\{");
    const match = source.match(pattern);
    expect(match, selector + " 规则缺失").not.toBeNull();
    const start = (match as RegExpMatchArray).index! + (match as RegExpMatchArray)[0].length;
    return source.slice(start, source.indexOf("}", start));
  }

  it("栏与 composer 同挂 .composer-dock 停靠列，栏在列中先于 composer（几何序由此保证）", () => {
    const dockStart = appSource.indexOf('<div className="composer-dock"');
    expect(dockStart).toBeGreaterThan(-1);
    const openEnd = appSource.indexOf(">", dockStart) + 1;
    const planIndex = appSource.indexOf("<PlanBar", openEnd);
    const composerIndex = appSource.indexOf("<Composer", openEnd);
    const dockEnd = appSource.indexOf("\n      </div>", composerIndex);
    // 栏不再是 .conversation-panel 里 composer 之外的流内兄弟（那正是被浮层压住的形态）
    expect(planIndex).toBeGreaterThan(openEnd);
    expect(composerIndex).toBeGreaterThan(planIndex);
    expect(dockEnd).toBeGreaterThan(composerIndex);
    expect(appSource.split("<PlanBar").length - 1).toBe(1);
    expect(appSource).toContain("chainLive={agentTurnRunning || trajectoryLive}");
  });

  it("停靠列承担底部浮层定位，composer 的绝对定位被中性化（栏底边恒在 composer 顶边之上）", () => {
    const dock = cssRule(css, ".composer-dock");
    expect(dock).toMatch(/position:\s*absolute/);
    expect(dock).toMatch(/flex-direction:\s*column/);
    // composer 本体的视口位置不变：停靠列沿用 styles.css .composer 的 bottom 偏移
    expect(cssRule(baseCss, ".composer")).toMatch(/bottom:\s*22px/);
    expect(dock).toMatch(/bottom:\s*22px/);
    const gap = dock.match(/gap:\s*(\d+(?:\.\d+)?)px/);
    expect(gap, "停靠列必须有非负 gap（栏与输入框的间距）").not.toBeNull();
    expect(Number(gap![1])).toBeGreaterThanOrEqual(0);
    const composerOverride = cssRule(css, ".composer-dock > .composer");
    expect(composerOverride).toMatch(/position:\s*relative/);
    expect(composerOverride).toMatch(/bottom:\s*auto/);
  });

  it("底部预留量测随停靠列（栏出现时消息流同步多留出栏高，避免新的遮挡）", () => {
    expect(appSource).toContain("composerDockRef");
    expect(appSource).toMatch(/Math\.max\(dockHeight, composerHeight\)|const dockHeight = composerDockRef\.current/);
  });
});

describe("PlanBar 生命周期（PLANBAR-1 缺陷②：执行结束后停掉 / 无活链不显 live 标签）", () => {
  it("相位分类：终态=terminal、无活链非终态=stale、活链非终态=live", () => {
    expect(planBarPhase("settled", false)).toBe("terminal");
    expect(planBarPhase("completed", true)).toBe("terminal");
    expect(planBarPhase("no_candidate_found", false)).toBe("terminal");
    expect(planBarPhase("cancelled", false)).toBe("terminal");
    expect(planBarPhase("observation_in_progress", true)).toBe("live");
    expect(planBarPhase("observation_in_progress", false)).toBe("stale");
    expect(planBarPhase("needs_experiment", false)).toBe("stale");
    expect(planBarPhase("human_judgment_required", false)).toBe("waiting");
    expect(planBarPhase("capability_blocked", true)).toBe("warning");
  });

  it("终态与陈旧态在有限时间内收起；live/waiting/warning 不自动清场", () => {
    expect(Number.isFinite(planBarDwellMs)).toBe(true);
    expect(planBarDwellMs).toBeGreaterThan(0);
    expect(planBarDwellMs).toBeLessThanOrEqual(15000);
    expect(planBarRetracts("terminal")).toBe(true);
    expect(planBarRetracts("stale")).toBe(true);
    expect(planBarRetracts("live")).toBe(false);
    expect(planBarRetracts("waiting")).toBe(false);
    expect(planBarRetracts("warning")).toBe(false);
    // 时间表：先淡出（仍在屏上），过淡出窗后卸载——不是常驻
    expect(planBarRetracted("terminal", 0)).toBe(false);
    expect(planBarRetracted("terminal", planBarDwellMs)).toBe(false);
    expect(planBarRetracted("terminal", planBarDwellMs + planBarFadeMs)).toBe(true);
    expect(planBarRetracted("stale", planBarDwellMs + planBarFadeMs)).toBe(true);
    expect(planBarRetracted("live", planBarDwellMs * 100)).toBe(false);
  });

  it("无活链的非终态：灰态「已中断」，不带 live 标签与旋转图标", () => {
    const snapshot = mockTaskTrajectorySnapshot();
    const live = renderToStaticMarkup(<PlanBar snapshot={snapshot} plan={{ current_step: "观察掩蔽关系" }} chainLive />);
    expect(live).toContain("正在观察工程");
    expect(live).toContain("plan-bar is-active");
    expect(live).toContain('data-plan-phase="live"');

    const stale = renderToStaticMarkup(<PlanBar snapshot={snapshot} plan={{ current_step: "观察掩蔽关系" }} chainLive={false} />);
    expect(stale).not.toContain("正在观察工程");
    expect(stale).not.toContain('class="spin"');
    expect(stale).not.toContain("is-active");
    expect(stale).toContain("is-stale");
    expect(stale).toContain("已中断");
    expect(stale).toContain('data-plan-phase="stale"');
    // 栏的其余信息面（意图/步骤/Task id）保留，收起与否由时间表决定
    expect(stale).toContain("改善主唱清晰度，不明显增加亮度。");
    expect(stale).toContain("Task mock-task");
  });

  it("终态：灰态无 live 标签，且进入收起时间表", () => {
    const snapshot = mockTaskTrajectorySnapshot();
    const settled = { ...snapshot!, semantic: { ...snapshot!.semantic, state: "settled" } };
    const markup = renderToStaticMarkup(<PlanBar snapshot={settled} chainLive={false} />);
    expect(markup).toContain("is-terminal");
    expect(markup).toContain("任务已完成");
    expect(markup).not.toContain('class="spin"');
    expect(markup).toContain('data-plan-phase="terminal"');
    expect(planBarRetracts("terminal")).toBe(true);
  });

  it("收起动画类与样式在位（淡出后卸载，尊重 reduced-motion）", () => {
    const css = readFileSync(new URL("./planbar.css", import.meta.url), "utf8");
    const barSource = readFileSync(new URL("./PlanBar.tsx", import.meta.url), "utf8");
    expect(css).toContain(".plan-bar.is-retiring");
    expect(css).toMatch(/prefers-reduced-motion[\s\S]*is-retiring/);
    expect(barSource).toContain("is-retiring");
    expect(barSource).toMatch(/if \(planBarRetracted\(phase, elapsedMs\)\) return null/);
  });
});
