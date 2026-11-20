// @vitest-environment jsdom
/*
 * TUTOR-SIM-3（池序 437）：导师进度卡 / 步骤输入卡的静态渲染断言。
 * 交互提交链（发 /agent/chat、进度推进）由 ProjectSurface.tutor.test.tsx 覆盖。
 * SIM-MUSIC-WIDGET-1（441）：MidiMaterialCard 谱面展开交互（jsdom + fixture 解析喂入）。
 */

import { StrictMode, act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { AudioMaterialCard, MidiMaterialCard, TutorProgressCard, TutorStepCard } from "./TutorWidgets";
import { parseMidiScore } from "./midiScore";
import type { ScoreModel } from "../scoreworkbench/model";
import type { TutorMaterialView, TutorStepView } from "./projectData";

(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

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

/* ------------------------------------------------------------------ */
/* SIM-MUSIC-WIDGET-1（441）：材料卡谱面展开（查看谱面 / 收起）           */
/* ------------------------------------------------------------------ */

// 仓库装置 fixture（只读）：小星星主题 42 音 / 48 拍 / 4/4 / 100 BPM。
const FIXTURE_REL = "../../coord/experiments/tutor-sim-2/assets/theme_twinkle_v0.mid";

function fixtureScoreModel(): ScoreModel {
  const text = readFileSync(join(process.cwd(), FIXTURE_REL), "latin1");
  const bytes = new Uint8Array(text.length);
  for (let i = 0; i < text.length; i++) bytes[i] = text.charCodeAt(i) & 0xff;
  return parseMidiScore(bytes).model;
}

const LOADED_MATERIAL: TutorMaterialView = {
  ...MIDI_MATERIAL,
  loaded: true,
  instrumentAttached: true
};

describe("MidiMaterialCard 谱面入口（SIM-MUSIC-WIDGET-1）", () => {
  it("无谱面数据：无谱面入口（fail-honest，不伪造）", () => {
    const markup = renderToStaticMarkup(<MidiMaterialCard view={LOADED_MATERIAL} onLoad={() => undefined} />);
    expect(markup).not.toContain("data-tutor-score-toggle");
    expect(markup).toContain('data-tutor-score-available="false"');
  });

  it("有谱面数据但材料未装载：无谱面入口（谱面=已装载材料的交互）", () => {
    const markup = renderToStaticMarkup(
      <MidiMaterialCard view={MIDI_MATERIAL} onLoad={() => undefined} score={fixtureScoreModel()} />
    );
    expect(markup).not.toContain("data-tutor-score-toggle");
    expect(markup).toContain('data-tutor-score-available="false"');
  });

  it("已装载 + 有谱面数据：默认收起（入口在，谱面不在）", () => {
    const markup = renderToStaticMarkup(
      <MidiMaterialCard view={LOADED_MATERIAL} onLoad={() => undefined} score={fixtureScoreModel()} />
    );
    expect(markup).toContain("data-tutor-score-toggle");
    expect(markup).toContain("查看谱面");
    expect(markup).toContain('data-tutor-score-open="false"');
    expect(markup).not.toContain("data-tutor-score-svg");
    expect(markup).toContain('data-tutor-score-available="true"');
  });
});

describe("MidiMaterialCard 谱面展开交互（SIM-MUSIC-WIDGET-1，jsdom）", () => {
  const mounted: Array<{ root: Root; container: HTMLElement }> = [];

  afterEach(() => {
    for (const entry of mounted.splice(0)) {
      act(() => entry.root.unmount());
      entry.container.remove();
    }
  });

  function mountScoreCard(score: ScoreModel | null): HTMLElement {
    const container = document.createElement("div");
    document.body.appendChild(container);
    const root = createRoot(container);
    act(() => {
      root.render(
        <StrictMode>
          <MidiMaterialCard view={LOADED_MATERIAL} onLoad={() => undefined} score={score} />
        </StrictMode>
      );
    });
    mounted.push({ root, container });
    return container;
  }

  it("展开：VexFlow 谱面渲染 + 音符数/拍号来自装置 fixture（42 音 / 4/4）；收起：谱面移除", () => {
    const container = mountScoreCard(fixtureScoreModel());
    const toggle = container.querySelector<HTMLElement>("[data-tutor-score-toggle]");
    expect(toggle).not.toBeNull();
    expect(container.querySelector("[data-tutor-score-svg]")).toBeNull();

    act(() => {
      toggle!.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
    const svgHost = container.querySelector<HTMLElement>("[data-tutor-score-svg]");
    expect(svgHost).not.toBeNull();
    expect(svgHost!.querySelector("svg")).not.toBeNull(); // VexFlow SVG 在卷（374 渲染层复用）
    expect(container.querySelector("[data-tutor-score-notes]")?.textContent).toContain("42 音");
    expect(container.querySelector("[data-tutor-score-timesig]")?.textContent).toBe("4/4");
    expect(container.querySelector("[data-tutor-score]")?.getAttribute("data-tutor-score-open")).toBe("true");

    act(() => {
      container.querySelector<HTMLElement>("[data-tutor-score-toggle]")!.dispatchEvent(
        new MouseEvent("click", { bubbles: true })
      );
    });
    expect(container.querySelector("[data-tutor-score-svg]")).toBeNull();
    expect(container.querySelector("[data-tutor-score]")?.getAttribute("data-tutor-score-open")).toBe("false");
    expect(container.querySelector("[data-tutor-score-toggle]")!.textContent).toContain("查看谱面");
  });
});
