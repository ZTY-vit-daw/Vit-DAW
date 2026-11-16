import { renderToStaticMarkup } from "react-dom/server";
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { mockTaskTrajectorySnapshot } from "../trajectoryMock";
import { PlanBar, planBarDwellMs, planBarFadeMs, planBarPhase, planBarRetracted, planBarRetracts } from "./PlanBar";

describe("PlanBar 规划条（GUI-T6）", () => {
  it("有 snapshot：收起窄条含状态标签/意图摘要/当前步骤，默认收起；Task id 不上可见面（STATUSBAR-ID-1）", () => {
    const snapshot = mockTaskTrajectorySnapshot();
    expect(snapshot).not.toBeNull();
    const markup = renderToStaticMarkup(
      <PlanBar snapshot={snapshot} plan={{ current_step: "观察掩蔽关系" }} />
    );
    expect(markup).toContain('aria-label="任务规划"');
    expect(markup).toContain("正在观察工程");
    expect(markup).toContain("改善主唱清晰度，不明显增加亮度。");
    expect(markup).toContain("观察掩蔽关系");
    // STATUSBAR-ID-1（并入 WEBUI-IA-REDESIGN-1）：哈希类任务 id 不再渲染可见
    // 文本；完整 id 保留在 data-task-id 属性，调试走 DOM 面板查。
    expect(markup).not.toContain("Task mock-task");
    expect(markup).toContain('data-task-id="mock-task"');
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
    // WEBUI-SESSION-SEMANTICS-1：链活/兜底按会话归属门控（planBarOwnLive）——
    // 全局 goal running 只有属主快照在位或本会话轨迹回合开放时才算本会话活链，
    // 异会话任务在跑不得点亮输入框上方的规划条。
    expect(appSource).toContain("chainLive={planBarOwnLive}");
    expect(appSource).toContain(
      "const planBarOwnLive = trajectoryLive || (agentTurnRunning && taskTrajectoryState.snapshot !== null);"
    );
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
    // 栏的其余信息面（意图/步骤）保留；Task id 仅存 data-task-id（STATUSBAR-ID-1），
    // 收起与否由时间表决定
    expect(stale).toContain("改善主唱清晰度，不明显增加亮度。");
    expect(stale).toContain('data-task-id="mock-task"');
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

// --------------------------------------------- WEBUI-PLANBAR-COMPLETED-1
// 2026-10-21 场次③（用户目检+日志对照在卷）：动态链正常收工（goal completed+
// chainEnded=true+journal 入账）后链活信号自然熄灭，PlanBar 误标灰态「已中断」
// ——链活启发式不读 goal 终态。三态契约钉死：
//   * 活链=现行态；正常完成=读快照 task.goal_status 终态（completed/stable）
//     → terminal「已完成」，不降灰；真中断=无终态证据才 stale（fail-open）。
//   * 数据源=trajectory 投影自带字段（task_runtime_trajectory.go 的 task.goal_status），
//     缺字段的旧载荷维持 stale 现状，零回退。
describe("PlanBar 三态（WEBUI-PLANBAR-COMPLETED-1：链正常完成不误标已中断）", () => {
  function snapshotWithGoalStatus(goalStatus: string) {
    const snapshot = mockTaskTrajectorySnapshot();
    expect(snapshot).not.toBeNull();
    return { ...snapshot!, task: { ...snapshot!.task, goal_status: goalStatus } };
  }

  it("相位分类：无活链非终态读 goal 终态——completed/stable=terminal，其余缺证据维持 stale", () => {
    expect(planBarPhase("observation_in_progress", false, "completed")).toBe("terminal");
    expect(planBarPhase("observation_in_progress", false, "stable")).toBe("terminal");
    expect(planBarPhase("observation_in_progress", false, "running")).toBe("stale");
    expect(planBarPhase("observation_in_progress", false, "")).toBe("stale");
    // 缺参（旧调用形）=缺终态字段：fail-open 维持 stale（既有语义零回退）
    expect(planBarPhase("observation_in_progress", false)).toBe("stale");
    // 既有优先级不变：警示/等待/终态词汇压过 goal 终态，活链压过一切
    expect(planBarPhase("settled", false, "")).toBe("terminal");
    expect(planBarPhase("failed", true, "completed")).toBe("warning");
    expect(planBarPhase("human_judgment_required", false, "completed")).toBe("waiting");
    expect(planBarPhase("observation_in_progress", true, "completed")).toBe("live");
  });

  it("链正常完成（goal completed+语义态停非终态）：「已完成」终态呈现，不降灰已中断", () => {
    const markup = renderToStaticMarkup(
      <PlanBar snapshot={snapshotWithGoalStatus("completed")} chainLive={false} />
    );
    expect(markup).toContain('data-plan-phase="terminal"');
    expect(markup).toContain("is-terminal");
    expect(markup).toContain("已完成");
    expect(markup).not.toContain("已中断");
    expect(markup).not.toContain("正在观察工程");
    expect(markup).not.toContain('class="spin"');
    // 完成态沿用既有终态生命周期：有限时间收起，非常驻
    expect(planBarRetracts("terminal")).toBe(true);
  });

  it("stable（agent 侧与 completed 同映射 TaskStatusSettled）同享完成呈现", () => {
    const markup = renderToStaticMarkup(
      <PlanBar snapshot={snapshotWithGoalStatus("stable")} chainLive={false} />
    );
    expect(markup).toContain('data-plan-phase="terminal"');
    expect(markup).toContain("已完成");
    expect(markup).not.toContain("已中断");
  });

  it("真中断（快照无 goal 终态/终态未到）：维持灰态「已中断」既有语义", () => {
    for (const goalStatus of ["", "running"]) {
      const markup = renderToStaticMarkup(
        <PlanBar snapshot={snapshotWithGoalStatus(goalStatus)} chainLive={false} />
      );
      expect(markup).toContain('data-plan-phase="stale"');
      expect(markup).toContain("已中断");
      expect(markup).not.toContain("已完成");
    }
  });

  it("活链：现行态不受 goal 终态字段影响", () => {
    const live = renderToStaticMarkup(
      <PlanBar snapshot={snapshotWithGoalStatus("running")} chainLive />
    );
    expect(live).toContain('data-plan-phase="live"');
    expect(live).toContain("正在观察工程");
    expect(live).not.toContain("已中断");
  });
});
