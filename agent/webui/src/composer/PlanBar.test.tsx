import { renderToStaticMarkup } from "react-dom/server";
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { mockTaskTrajectorySnapshot } from "../trajectoryMock";
import { PlanBar } from "./PlanBar";

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
