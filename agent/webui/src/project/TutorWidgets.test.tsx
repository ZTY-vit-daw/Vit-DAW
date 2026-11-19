/*
 * TUTOR-SIM-3（池序 437）：导师进度卡 / 步骤输入卡的静态渲染断言。
 * 交互提交链（发 /agent/chat、进度推进）由 ProjectSurface.tutor.test.tsx 覆盖。
 */

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { TutorProgressCard, TutorStepCard } from "./TutorWidgets";
import type { TutorStepView } from "./projectData";

const ACTIVE_VIEW: TutorStepView = {
  stepIndex: 1,
  stepTotal: 3,
  completed: false,
  prompt: "听听这段，输入你听到的频段数字。"
};

const COMPLETED_VIEW: TutorStepView = {
  stepIndex: 3,
  stepTotal: 3,
  completed: true,
  prompt: "三步都通过了，实验完成。"
};

describe("TutorProgressCard", () => {
  it("第 1/3 步：结构化属性与文案在卷", () => {
    const markup = renderToStaticMarkup(<TutorProgressCard view={ACTIVE_VIEW} />);
    expect(markup).toContain("data-tutor-progress");
    expect(markup).toContain('data-tutor-progress-step="1"');
    expect(markup).toContain('data-tutor-progress-total="3"');
    expect(markup).toContain('data-tutor-progress-completed="false"');
    expect(markup).toContain("第 1/3 步");
  });

  it("completed：显示 3/3 · 已完成", () => {
    const markup = renderToStaticMarkup(<TutorProgressCard view={COMPLETED_VIEW} />);
    expect(markup).toContain('data-tutor-progress-completed="true"');
    expect(markup).toContain("第 3/3 步 · 已完成");
  });
});

describe("TutorStepCard", () => {
  it("未完成：当前问题 + 数字输入 + 可提交", () => {
    const markup = renderToStaticMarkup(<TutorStepCard view={ACTIVE_VIEW} onSubmit={() => undefined} />);
    expect(markup).toContain("data-tutor-step");
    expect(markup).toContain('data-tutor-step-index="1"');
    expect(markup).toContain('data-tutor-step-total="3"');
    expect(markup).toContain('data-tutor-step-completed="false"');
    expect(markup).toContain("data-tutor-step-prompt");
    expect(markup).toContain(ACTIVE_VIEW.prompt);
    expect(markup).toContain('type="number"');
    expect(markup).toContain("data-tutor-step-input");
    expect(markup).toContain("data-tutor-step-submit");
    expect(markup).toContain("提交");
    // 未提交前空输入不可提交（防空发送）。
    expect(markup).toMatch(/<button[^>]*data-tutor-step-submit[^>]*disabled/);
  });

  it("completed：输入与提交禁用，显示完成态", () => {
    const markup = renderToStaticMarkup(<TutorStepCard view={COMPLETED_VIEW} onSubmit={() => undefined} />);
    expect(markup).toContain('data-tutor-step-completed="true"');
    expect(markup).toContain("实验已完成");
    expect(markup).toMatch(/<input[^>]*data-tutor-step-input[^>]*disabled/);
    expect(markup).toMatch(/<button[^>]*data-tutor-step-submit[^>]*disabled/);
    expect(markup).toContain("无需继续输入");
  });

  it("pending：输入/提交禁用，按钮显示提交中", () => {
    const markup = renderToStaticMarkup(<TutorStepCard view={ACTIVE_VIEW} pending onSubmit={() => undefined} />);
    expect(markup).toContain('data-tutor-step-pending="true"');
    expect(markup).toContain("提交中…");
    expect(markup).toMatch(/<input[^>]*data-tutor-step-input[^>]*disabled/);
  });
});
