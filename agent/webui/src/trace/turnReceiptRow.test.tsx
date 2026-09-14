import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { TurnReceiptRow, turnReceiptText } from "./TurnReceiptRow";
import type { TurnReceipt } from "./turnReceipts";

// TRAJ-IMPL-3（设计 §2.3 分期一② + §7 裁定 C「一行先做出来看」）：**静态收起回执行行**的
// 渲染面钉——状态文案分支、冻结句式（traceMetaParts 单一出处）、静态形态（无按钮/无转圈/
// 无展开箭头/无游标）、只显摘要（正文与步骤标题一个字都不出现）。

function receipt(partial: Partial<TurnReceipt> & { turnId: string }): TurnReceipt {
  return { status: "completed", stepCount: 2, activityCount: 2, workMs: 30_000, parkMs: null, startedAt: 0, ...partial };
}

/** 去掉标签后的可见文本（渲染面的等价物：用户在这行上读到的东西） */
const visibleText = (markup: string): string => markup.replace(/<[^>]*>/g, "");

describe("回执行行（静态收起）", () => {
  it("完成态：执行完成 · N 步 · 执行时长（句式与终态块同源）", () => {
    const row = receipt({ turnId: "run_ok", stepCount: 2, workMs: 30_000 });
    const markup = renderToStaticMarkup(<TurnReceiptRow receipt={row} />);
    expect(turnReceiptText(row)).toBe("执行完成 · 2 步 · 30.0s");
    expect(visibleText(markup)).toBe("执行完成 · 2 步 · 30.0s");
    expect(markup).toContain('data-turn-id="run_ok"');
    expect(markup).toContain('data-receipt-status="completed"');
  });

  it("失败态 / 停止态文案分支：失败 → 执行失败；驻留段单列 → 执行 Xs · 等待续跑 Ys", () => {
    const failed = receipt({ turnId: "run_failed", status: "failed", stepCount: 3, workMs: 12_500 });
    expect(visibleText(renderToStaticMarkup(<TurnReceiptRow receipt={failed} />))).toBe("执行失败 · 3 步 · 12.5s");

    const stopped = receipt({ turnId: "run_stopped", status: "stopped", stepCount: 4, workMs: 55_000, parkMs: 203_000 });
    expect(turnReceiptText(stopped)).toBe("已停止 · 4 步 · 执行 55.0s · 等待续跑 203.0s");
    expect(visibleText(renderToStaticMarkup(<TurnReceiptRow receipt={stopped} />))).toBe(
      "已停止 · 4 步 · 执行 55.0s · 等待续跑 203.0s"
    );
  });

  it("静态：无按钮、无展开箭头、无转圈/游标（活态不是它的事）", () => {
    const markup = renderToStaticMarkup(<TurnReceiptRow receipt={receipt({ turnId: "run_static" })} />);
    expect(markup).not.toContain("<button");
    expect(markup).not.toContain("trace-chev");
    expect(markup).not.toContain("trace-cursor");
    expect(markup).not.toContain("spin");
    expect(markup).not.toContain("aria-live");
  });

  it("只显摘要：没有步数证据时显活动数，正文/标题一个字都不出现", () => {
    const activitiesOnly = receipt({ turnId: "run_activity", stepCount: 0, activityCount: 3, workMs: 142_000 });
    expect(visibleText(renderToStaticMarkup(<TurnReceiptRow receipt={activitiesOnly} />))).toBe("执行完成 · 3 项活动 · 142.0s");

    const dirty = { ...receipt({ turnId: "run_dirty" }), content: "我把低音轨的 EQ 调好了", title: "已完成 频率关系观察" } as TurnReceipt;
    const markup = renderToStaticMarkup(<TurnReceiptRow receipt={dirty} />);
    expect(visibleText(markup)).toBe("执行完成 · 2 步 · 30.0s");
    expect(markup).not.toContain("我把低音轨的 EQ 调好了");
    expect(markup).not.toContain("已完成 频率关系观察");
  });

  it("时长无证据（workMs = null）不虚报 0.0s：显 --（计数照显）", () => {
    const unknown = receipt({ turnId: "run_unknown", workMs: null });
    expect(turnReceiptText(unknown)).toBe("执行完成 · 2 步 · --");
  });
});
