/*
 * TUTOR-SIM-3（池序 437）：导师进度卡 / 步骤输入卡的静态渲染断言。
 * 交互提交链（发 /agent/chat、进度推进）由 ProjectSurface.tutor.test.tsx 覆盖。
 */

import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { AudioMaterialCard, MidiMaterialCard, TutorProgressCard, TutorStepCard } from "./TutorWidgets";
import type { TutorMaterialView, TutorStepView } from "./projectData";

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

/* ------------------------------------------------------------------ */
/* TUTOR-SIM-4（438）：brief 折叠 / MidiMaterialCard / AudioMaterialCard  */
/* ------------------------------------------------------------------ */

const BRIEF_VIEW: TutorStepView = {
  ...ACTIVE_VIEW,
  brief: "教学简报：先装载材料，再从头听一遍主题。"
};

describe("TutorStepCard brief（TUTOR-SIM-4）", () => {
  it("有 brief：默认收起（无正文），可点开", () => {
    const markup = renderToStaticMarkup(<TutorStepCard view={BRIEF_VIEW} onSubmit={() => undefined} />);
    expect(markup).toContain("data-tutor-brief");
    expect(markup).toContain('data-tutor-brief-open="false"');
    expect(markup).toContain("查看简报");
    // 默认收起：不渲染正文。
    expect(markup).not.toContain("data-tutor-brief-text");
    expect(markup).not.toContain("教学简报：先装载材料");
  });

  it("无 brief：不渲染简报块（v0 实验契约面）", () => {
    const markup = renderToStaticMarkup(<TutorStepCard view={ACTIVE_VIEW} onSubmit={() => undefined} />);
    expect(markup).not.toContain("data-tutor-brief");
    expect(markup).not.toContain("查看简报");
  });
});

const MIDI_MATERIAL: TutorMaterialView = {
  kind: "midi_clip",
  title: "小星星主题（42 音）",
  path: "assets/theme_twinkle_v0.mid",
  noteCount: 42,
  durationBeats: 48,
  tempoBPM: 100,
  loaded: false,
  loading: false
};

describe("MidiMaterialCard（TUTOR-SIM-4）", () => {
  it("未装载：材料名 + 摘要 + 可点装载按钮", () => {
    const markup = renderToStaticMarkup(
      <MidiMaterialCard view={MIDI_MATERIAL} onLoad={() => undefined} />
    );
    expect(markup).toContain("data-tutor-material");
    expect(markup).toContain('data-tutor-material-kind="midi_clip"');
    expect(markup).toContain('data-tutor-material-title="小星星主题（42 音）"');
    expect(markup).toContain('data-tutor-material-loaded="false"');
    expect(markup).toContain("data-tutor-material-summary");
    expect(markup).toContain("42 音");
    expect(markup).toContain("48 拍");
    expect(markup).toContain("装载到工程");
    expect(markup).not.toMatch(/<button[^>]*data-tutor-material-load[^>]*disabled/);
  });

  it("已装载：按钮禁用、显示已装载与音源状态", () => {
    const markup = renderToStaticMarkup(
      <MidiMaterialCard view={{ ...MIDI_MATERIAL, loaded: true, instrumentAttached: true }} onLoad={() => undefined} />
    );
    expect(markup).toContain('data-tutor-material-loaded="true"');
    expect(markup).toContain("已装载");
    expect(markup).toMatch(/<button[^>]*data-tutor-material-load[^>]*disabled/);
    expect(markup).toContain('data-tutor-material-instrument-attached="true"');
    expect(markup).toContain("已挂音源");
  });

  it("音源挂载被拒：如实显示原因，不算未装载", () => {
    const markup = renderToStaticMarkup(
      <MidiMaterialCard
        view={{ ...MIDI_MATERIAL, loaded: true, instrumentAttached: false, instrumentError: "plugin not found" }}
        onLoad={() => undefined}
      />
    );
    expect(markup).toContain('data-tutor-material-instrument-attached="false"');
    expect(markup).toContain("音源未挂");
    expect(markup).toContain("plugin not found");
    expect(markup).toMatch(/<button[^>]*data-tutor-material-load[^>]*disabled/);
  });

  it("摘要缺失/在途：按钮禁用（防不可用装载与重复提交）", () => {
    const unavailable = renderToStaticMarkup(
      <MidiMaterialCard view={{ ...MIDI_MATERIAL, noteCount: undefined, durationBeats: undefined, summaryError: "missing" }} onLoad={() => undefined} />
    );
    expect(unavailable).toContain("材料不可用");
    expect(unavailable).toMatch(/<button[^>]*data-tutor-material-load[^>]*disabled/);
    const pending = renderToStaticMarkup(
      <MidiMaterialCard view={MIDI_MATERIAL} pending onLoad={() => undefined} />
    );
    expect(pending).toContain("装载中…");
    expect(pending).toMatch(/<button[^>]*data-tutor-material-load[^>]*disabled/);
  });
});

describe("AudioMaterialCard（TUTOR-SIM-4 schema 位）", () => {
  it("audio 材料复用 AudioPlayer；无真实音频时诚实降级（禁播）", () => {
    const markup = renderToStaticMarkup(
      <AudioMaterialCard view={{ kind: "audio", title: "音频材料位", path: "", loaded: false, loading: false }} />
    );
    expect(markup).toContain('data-tutor-material-kind="audio"');
    expect(markup).toContain("data-widget-audio-player");
    expect(markup).toContain("data-audio-unavailable");
  });
});
