// @vitest-environment jsdom
/*
 * TUTOR-SIM-2（池序 436）：ProjectSurface 工程打开自动开场接线测试。
 *
 * 纪律：
 * - 只经 ProjectService 接缝驱动；fixture 服务替身在接缝处扮演服务端，不发网络。
 * - 真实 /agent/tutor/start 请求与 unavailable 降级映射由 projectService.test.ts 覆盖。
 * - 本文件所有工程路径/开场文案均为测试 fixture（不冒充真实工程/真实 LLM 输出）。
 */

import { StrictMode } from "react";
import type { ReactElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { ProjectSurface } from "./ProjectSurface";
import { createProjectFixtureService } from "./projectFixtures";
import { parseMidiScore } from "./midiScore";
import type { ScoreModel } from "../scoreworkbench/model";
import type { ProjectContextView, ProjectThreadItem, ProjectService, TutorStartOutcome } from "./projectData";

(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const PROJECT_PATH = "D:/fixtures/tutor-project-436.vit";
const OTHER_PATH = "D:/fixtures/ordinary-project-436.vit";
const OPENING = "第 1/3 步：先听这段，再告诉我你听到的频段。";
const OTHER_OPENING = "普通工程开场（fixture）。";

const CONTEXT: ProjectContextView = {
  name: "导师实验工程（fixture）",
  source: "fixture",
  path: PROJECT_PATH
};

const OTHER_CONTEXT: ProjectContextView = {
  name: "普通工程（fixture）",
  source: "fixture",
  path: OTHER_PATH
};

const OK_OPENING: TutorStartOutcome = {
  status: "ok",
  conversationID: "tutor_conv_436",
  reply: OPENING,
  stepIndex: 1,
  stepTotal: 3,
  stepID: "step-1"
};

interface Mounted {
  root: Root;
  container: HTMLElement;
}

const mounted: Mounted[] = [];

function mount(element: ReactElement): Mounted {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  act(() => {
    root.render(element);
  });
  const entry = { root, container };
  mounted.push(entry);
  return entry;
}

function unmount(entry: Mounted): void {
  const index = mounted.indexOf(entry);
  if (index >= 0) mounted.splice(index, 1);
  act(() => {
    entry.root.unmount();
  });
  entry.container.remove();
}

/** 冲掉 service 微任务 + React effect 调度（fixture latency=0）。 */
async function flush(): Promise<void> {
  for (let i = 0; i < 4; i += 1) {
    await act(async () => {
      await Promise.resolve();
    });
  }
}

function click(element: Element): void {
  act(() => {
    element.dispatchEvent(new MouseEvent("click", { bubbles: true, cancelable: true }));
  });
}

// React 受控 input：经 prototype 原生 setter 写入再派发 input，onChange 才会触发。
function typeInput(input: HTMLInputElement, value: string): void {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
  act(() => {
    setter?.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

function sendFromComposer(container: HTMLElement, text: string): void {
  const input = container.querySelector("input.v5-composer-input") as HTMLInputElement | null;
  const button = container.querySelector('button[aria-label="发送"]');
  expect(input).not.toBeNull();
  expect(button).not.toBeNull();
  typeInput(input as HTMLInputElement, text);
  click(button as Element);
}

function messages(container: HTMLElement): Element[] {
  return Array.from(container.querySelectorAll("[data-message-author]"));
}

/** TUTOR-SIM-3：进度卡的结构化属性（断言数据来自 workflow_data 而非回复文本）。 */
function progressAttrs(container: HTMLElement): { step: string | null; total: string | null; completed: string | null } {
  const node = container.querySelector("[data-tutor-progress]");
  return {
    step: node?.getAttribute("data-tutor-progress-step") ?? null,
    total: node?.getAttribute("data-tutor-progress-total") ?? null,
    completed: node?.getAttribute("data-tutor-progress-completed") ?? null
  };
}

function tutorStepInput(container: HTMLElement): HTMLInputElement {
  const input = container.querySelector("[data-tutor-step-input]") as HTMLInputElement | null;
  expect(input).not.toBeNull();
  return input as HTMLInputElement;
}

/** 步骤卡提交：写受控输入后派发 form submit（与真实按钮提交同一 React 通道）。 */
function submitTutorStep(container: HTMLElement, text: string): void {
  const input = tutorStepInput(container);
  expect(input.disabled).toBe(false);
  typeInput(input, text);
  const form = container.querySelector("[data-tutor-step-form]");
  expect(form).not.toBeNull();
  act(() => {
    form?.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
  });
}

function threadWith(...items: ProjectThreadItem[]): ProjectThreadItem[] {
  return items;
}

afterEach(() => {
  for (const entry of [...mounted]) unmount(entry);
  document.body.innerHTML = "";
});

describe("TUTOR-SIM-2：active 工程自动开场", () => {
  it("start=ok：每工程恰好 start 一次、开场作为 assistant 消息渲染、发送带导师 conversation_id", async () => {
    const tutorStartCalls: string[] = [];
    const sentMessages: string[] = [];
    const sentConversationIDs: (string | undefined)[] = [];
    const service = createProjectFixtureService({
      context: CONTEXT,
      tutorStart: OK_OPENING,
      tutorStartCalls,
      sentMessages,
      sentConversationIDs
    });
    const { root, container } = mount(
      <ProjectSurface fixture={{ context: CONTEXT, service }} />
    );
    await flush();

    expect(tutorStartCalls).toEqual([PROJECT_PATH]);
    expect(container.textContent).toContain(OPENING);
    const vitMessages = messages(container).filter((node) => node.getAttribute("data-message-author") === "vit");
    expect(vitMessages.some((node) => node.textContent?.includes(OPENING))).toBe(true);

    // 重渲染（同工程）不得重复 start。
    act(() => {
      root.render(<ProjectSurface fixture={{ context: { ...CONTEXT }, service }} />);
    });
    await flush();
    expect(tutorStartCalls).toHaveLength(1);

    // composer 后续消息走服务端签发的导师会话。
    sendFromComposer(container, "7");
    await flush();
    expect(sentMessages).toEqual(["7"]);
    expect(sentConversationIDs).toEqual(["tutor_conv_436"]);
  });

  it("StrictMode 双跑 effect 仍只 start 一次且开场可见", async () => {
    const tutorStartCalls: string[] = [];
    const service = createProjectFixtureService({
      context: CONTEXT,
      tutorStart: OK_OPENING,
      tutorStartCalls
    });
    const { container } = mount(
      <StrictMode>
        <ProjectSurface fixture={{ context: CONTEXT, service }} />
      </StrictMode>
    );
    await flush();
    expect(tutorStartCalls).toEqual([PROJECT_PATH]);
    expect(container.textContent).toContain(OPENING);
  });

  it("start=unavailable：静默、无错误、线程原样保留", async () => {
    const tutorStartCalls: string[] = [];
    const service = createProjectFixtureService({
      context: CONTEXT,
      tutorStartCalls,
      tutorStart: { status: "unavailable", reason: "sidecar_missing" }
    });
    const { container } = mount(
      <ProjectSurface
        fixture={{
          context: CONTEXT,
          service,
          thread: threadWith({ kind: "message", id: "m-existing", author: "vit", text: "工程已就绪。" })
        }}
      />
    );
    await flush();

    expect(tutorStartCalls).toEqual([PROJECT_PATH]);
    expect(container.textContent).toContain("工程已就绪。");
    expect(messages(container)).toHaveLength(1);
    expect(container.querySelector("[data-project-context-error]")).toBeNull();
    expect(container.querySelector("[data-project-notice]")).toBeNull();
  });

  it("active=false：不发送 start 请求", async () => {
    const tutorStartCalls: string[] = [];
    const service = createProjectFixtureService({
      context: CONTEXT,
      tutorStart: OK_OPENING,
      tutorStartCalls
    });
    const { container } = mount(<ProjectSurface active={false} fixture={{ context: CONTEXT, service }} />);
    await flush();
    expect(tutorStartCalls).toHaveLength(0);
    expect(container.textContent).not.toContain(OPENING);
  });

  it("普通工程（start=unavailable）：现有线程不变，发送仍用本 surface 普通会话", async () => {
    const tutorStartCalls: string[] = [];
    const sentConversationIDs: (string | undefined)[] = [];
    const service = createProjectFixtureService({
      context: OTHER_CONTEXT,
      tutorStartCalls,
      sentConversationIDs
    });
    const { container } = mount(
      <ProjectSurface
        fixture={{
          context: OTHER_CONTEXT,
          service,
          thread: threadWith({ kind: "message", id: "m-existing", author: "vit", text: "普通工程线程保留。" })
        }}
      />
    );
    await flush();

    expect(tutorStartCalls).toEqual([OTHER_PATH]);
    expect(messages(container)).toHaveLength(1);
    expect(container.textContent).toContain("普通工程线程保留。");
    // TUTOR-SIM-3：普通工程无导师进度/步骤卡。
    expect(container.querySelector("[data-project-tutor]")).toBeNull();
    expect(container.querySelector("[data-tutor-progress]")).toBeNull();
    expect(container.querySelector("[data-tutor-step]")).toBeNull();
    expect(container.textContent).not.toContain("第 1/3 步");
    expect(container.textContent).not.toContain("workflow_data");

    sendFromComposer(container, "你好");
    await flush();
    // 未记录导师会话 = 不传 conversation_id（live 服务回落到本 surface 普通会话）。
    expect(sentConversationIDs).toEqual([undefined]);
  });
});

describe("TUTOR-SIM-2：竞态与泄漏", () => {
  it("start 在途时重渲染不重复请求；回执到达只入一条开场", async () => {
    let resolveStart: (outcome: TutorStartOutcome) => void = () => undefined;
    const deferred = new Promise<TutorStartOutcome>((resolve) => {
      resolveStart = resolve;
    });
    const tutorStartCalls: string[] = [];
    const base = createProjectFixtureService({ context: CONTEXT });
    const service: ProjectService = {
      ...base,
      startTutorSim: (projectPath: string) => {
        tutorStartCalls.push(projectPath);
        return deferred;
      }
    };
    const { root, container } = mount(
      <ProjectSurface fixture={{ context: CONTEXT, service }} />
    );
    await flush();

    expect(tutorStartCalls).toEqual([PROJECT_PATH]);
    expect(container.textContent).not.toContain(OPENING);

    for (let i = 0; i < 3; i += 1) {
      act(() => {
        root.render(<ProjectSurface fixture={{ context: CONTEXT, service }} />);
      });
      await flush();
    }
    expect(tutorStartCalls).toHaveLength(1);

    await act(async () => {
      resolveStart(OK_OPENING);
      await deferred;
    });
    await flush();

    expect(container.textContent).toContain(OPENING);
    const vitMessages = messages(container).filter((node) => node.getAttribute("data-message-author") === "vit");
    expect(vitMessages).toHaveLength(1);

    // 回执落线程后再次重渲染，仍不重复请求。
    act(() => {
      root.render(<ProjectSurface fixture={{ context: CONTEXT, service }} />);
    });
    await flush();
    expect(tutorStartCalls).toHaveLength(1);
  });

  it("卸载/切换工程后旧工程回执不泄漏进新工程线程", async () => {
    let resolveOld: (outcome: TutorStartOutcome) => void = () => undefined;
    const oldDeferred = new Promise<TutorStartOutcome>((resolve) => {
      resolveOld = resolve;
    });
    const oldCalls: string[] = [];
    const oldService: ProjectService = {
      ...createProjectFixtureService({ context: CONTEXT }),
      startTutorSim: (projectPath: string) => {
        oldCalls.push(projectPath);
        return oldDeferred;
      }
    };
    const first = mount(<ProjectSurface fixture={{ context: CONTEXT, service: oldService }} />);
    await flush();
    expect(oldCalls).toEqual([PROJECT_PATH]);

    // 工程切换/组件卸载：旧请求晚到不得落进新工程线程。
    unmount(first);
    await act(async () => {
      resolveOld(OK_OPENING);
      await oldDeferred;
    });
    await flush();

    const newCalls: string[] = [];
    const newService = createProjectFixtureService({
      context: OTHER_CONTEXT,
      tutorStartCalls: newCalls,
      tutorStart: {
        status: "ok",
        conversationID: "tutor_conv_other_436",
        reply: OTHER_OPENING,
        stepIndex: 1,
        stepTotal: 1,
        stepID: "step-only"
      }
    });
    const second = mount(<ProjectSurface fixture={{ context: OTHER_CONTEXT, service: newService }} />);
    await flush();

    expect(newCalls).toEqual([OTHER_PATH]);
    expect(second.container.textContent).toContain(OTHER_OPENING);
    expect(second.container.textContent).not.toContain(OPENING);
    expect(messages(second.container)).toHaveLength(1);
  });
});

describe("TUTOR-SIM-3：进度卡与步骤输入卡", () => {
  it("start 1/3；错输入仍 1/3；7→2/3；42→3/3；123 completed/禁用；提交均带导师会话", async () => {
    const tutorStartCalls: string[] = [];
    const submits: Array<{ text: string; conversationID?: string }> = [];
    let stepIndex = 1;
    let completed = false;
    const base = createProjectFixtureService({
      context: CONTEXT,
      tutorStart: OK_OPENING,
      tutorStartCalls
    });
    const service: ProjectService = {
      ...base,
      async sendMessage(text: string, conversationID?: string) {
        submits.push({ text, conversationID });
        let passed = false;
        if (stepIndex === 1 && text === "7") {
          stepIndex = 2;
          passed = true;
        } else if (stepIndex === 2 && text === "42") {
          stepIndex = 3;
          passed = true;
        } else if (stepIndex === 3 && text === "123") {
          completed = true;
          passed = true;
        }
        // 回复文本故意不带步骤数字：进度只能来自 workflow_data。
        return {
          text: passed ? "通过，继续下一步。" : "还没通过，再试一次。",
          workflow: "tutor_sim",
          workflowData: { step_index: stepIndex, step_total: 3, completed }
        };
      }
    };
    const { container } = mount(<ProjectSurface fixture={{ context: CONTEXT, service }} />);
    await flush();

    // start ok → 1/3，步骤卡显示开场问题。
    expect(progressAttrs(container)).toEqual({ step: "1", total: "3", completed: "false" });
    expect(container.querySelector("[data-tutor-step-prompt]")?.textContent).toContain(OPENING);
    expect(tutorStepInput(container).disabled).toBe(false);

    // 错输入 → workflow_data 仍 1/3（不推进），输入保持可用。
    submitTutorStep(container, "6");
    await flush();
    expect(progressAttrs(container)).toEqual({ step: "1", total: "3", completed: "false" });
    expect(container.textContent).toContain("还没通过，再试一次。");
    expect(container.querySelector("[data-tutor-step-prompt]")?.textContent).toContain("还没通过，再试一次。");
    expect(tutorStepInput(container).disabled).toBe(false);

    // 7 → 2/3。
    submitTutorStep(container, "7");
    await flush();
    expect(progressAttrs(container)).toEqual({ step: "2", total: "3", completed: "false" });

    // 42 → 3/3。
    submitTutorStep(container, "42");
    await flush();
    expect(progressAttrs(container)).toEqual({ step: "3", total: "3", completed: "false" });

    // 123 → completed：进度 3/3 已完成，输入与提交禁用。
    submitTutorStep(container, "123");
    await flush();
    expect(progressAttrs(container)).toEqual({ step: "3", total: "3", completed: "true" });
    expect(container.textContent).toContain("实验已完成");
    expect(container.textContent).toContain("无需继续输入");
    expect(tutorStepInput(container).disabled).toBe(true);
    const submitButton = container.querySelector("[data-tutor-step-submit]") as HTMLButtonElement | null;
    expect(submitButton?.disabled).toBe(true);

    // 每次步骤提交都走同一个导师 conversation_id。
    expect(submits.map((item) => item.text)).toEqual(["6", "7", "42", "123"]);
    expect(submits.every((item) => item.conversationID === "tutor_conv_436")).toBe(true);
  });

  it("进度只认 workflow_data：回复文本写 3/3 也不改变 1/3", async () => {
    const base = createProjectFixtureService({ context: CONTEXT, tutorStart: OK_OPENING });
    const service: ProjectService = {
      ...base,
      async sendMessage() {
        return {
          text: "模型措辞：现在已经是第 3/3 步。",
          workflow: "tutor_sim",
          workflowData: { step_index: 1, step_total: 3, completed: false }
        };
      }
    };
    const { container } = mount(<ProjectSurface fixture={{ context: CONTEXT, service }} />);
    await flush();

    submitTutorStep(container, "6");
    await flush();
    expect(progressAttrs(container)).toEqual({ step: "1", total: "3", completed: "false" });
    expect(container.querySelector("[data-tutor-step-prompt]")?.textContent).toContain("第 3/3 步");
  });

  it("无 workflow 标记的普通回执不更新步骤卡/进度", async () => {
    const base = createProjectFixtureService({ context: CONTEXT, tutorStart: OK_OPENING });
    const service: ProjectService = {
      ...base,
      async sendMessage() {
        return { text: "普通回复，无 workflow。" };
      }
    };
    const { container } = mount(<ProjectSurface fixture={{ context: CONTEXT, service }} />);
    await flush();
    expect(progressAttrs(container)).toEqual({ step: "1", total: "3", completed: "false" });

    submitTutorStep(container, "7");
    await flush();
    expect(progressAttrs(container)).toEqual({ step: "1", total: "3", completed: "false" });
    expect(container.querySelector("[data-tutor-step-prompt]")?.textContent).toContain(OPENING);
  });
});

/* ------------------------------------------------------------------ */
/* TUTOR-SIM-4（438）：brief 折叠 + 材料卡装载链                          */
/* ------------------------------------------------------------------ */

const MATERIAL_START: TutorStartOutcome = {
  status: "ok",
  conversationID: "tutor_conv_438",
  reply: "第 1/4 步：先装载材料，从头听一遍。",
  stepIndex: 1,
  stepTotal: 4,
  stepID: "step-hear-theme",
  brief: "教学简报：点材料卡上的「装载到工程」，主题会以 MIDI clip 进工程。",
  materials: [
    {
      kind: "midi_clip",
      title: "小星星主题（fixture）",
      path: "assets/theme_twinkle_v0.mid",
      noteCount: 42,
      durationBeats: 48,
      loaded: false,
      loading: false
    }
  ]
};

describe("TUTOR-SIM-4：brief 与材料卡（surface 装载链）", () => {
  it("start 携带 brief/materials：简报默认收起，材料卡可点装载，成功后按钮置灰", async () => {
    const loadCalls: Array<{ conversationID: string; title: string }> = [];
    const service = createProjectFixtureService({
      context: CONTEXT,
      tutorStart: MATERIAL_START,
      loadMaterialCalls: loadCalls
    });
    const { container } = mount(<ProjectSurface fixture={{ context: CONTEXT, service }} />);
    await flush();

    // brief 默认收起：有折叠块与开关，无正文。
    expect(container.querySelector("[data-tutor-brief]")).not.toBeNull();
    expect(container.querySelector("[data-tutor-brief]")?.getAttribute("data-tutor-brief-open")).toBe("false");
    expect(container.querySelector("[data-tutor-brief-text]")).toBeNull();
    expect(container.textContent).toContain("查看简报");
    expect(container.textContent).not.toContain("教学简报：点材料卡");

    // 材料卡：摘要 + 可点装载按钮。
    const material = container.querySelector("[data-tutor-material]");
    expect(material).not.toBeNull();
    expect(material?.getAttribute("data-tutor-material-kind")).toBe("midi_clip");
    expect(material?.getAttribute("data-tutor-material-loaded")).toBe("false");
    expect(container.querySelector("[data-tutor-material-summary]")?.textContent).toContain("42 音");
    const loadButton = container.querySelector("[data-tutor-material-load]") as HTMLButtonElement | null;
    expect(loadButton?.disabled).toBe(false);

    click(loadButton as Element);
    await flush();
    expect(loadCalls).toEqual([{ conversationID: "tutor_conv_438", title: "小星星主题（fixture）" }]);
    const loadedButton = container.querySelector("[data-tutor-material-load]") as HTMLButtonElement | null;
    expect(loadedButton?.textContent).toContain("已装载");
    expect(loadedButton?.disabled).toBe(true);
    expect(container.querySelector("[data-tutor-material]")?.getAttribute("data-tutor-material-loaded")).toBe("true");
  });

  it("装载失败：notice 显示原因，按钮保持可点（不伪造已装载）", async () => {
    const loadCalls: Array<{ conversationID: string; title: string }> = [];
    const service = createProjectFixtureService({
      context: CONTEXT,
      tutorStart: MATERIAL_START,
      loadMaterialCalls: loadCalls,
      loadMaterialOutcome: { ok: false, reason: "kernel refused" }
    });
    const { container } = mount(<ProjectSurface fixture={{ context: CONTEXT, service }} />);
    await flush();
    click(container.querySelector("[data-tutor-material-load]") as Element);
    await flush();
    expect(loadCalls).toHaveLength(1);
    expect(container.querySelector("[data-project-notice]")?.textContent).toContain("kernel refused");
    const loadButton = container.querySelector("[data-tutor-material-load]") as HTMLButtonElement | null;
    expect(loadButton?.disabled).toBe(false);
  });

  it("服务端已装载（workflow_data loaded=true）：按钮置灰，不重复装载", async () => {
    const loadCalls: Array<{ conversationID: string; title: string }> = [];
    const base = createProjectFixtureService({
      context: CONTEXT,
      tutorStart: {
        ...MATERIAL_START,
        materials: [{ ...MATERIAL_START.materials![0], loaded: true, instrumentAttached: true }]
      },
      loadMaterialCalls: loadCalls
    });
    const { container } = mount(<ProjectSurface fixture={{ context: CONTEXT, service: base }} />);
    await flush();
    const loadButton = container.querySelector("[data-tutor-material-load]") as HTMLButtonElement | null;
    expect(loadButton?.disabled).toBe(true);
    expect(container.textContent).toContain("已挂音源");
    click(loadButton as Element);
    await flush();
    expect(loadCalls).toHaveLength(0);
  });

  it("audio 材料渲染 AudioPlayer（无真实音频时禁播降级）；v0 实验无材料/无简报零新增卡", async () => {
    const audioStart: TutorStartOutcome = {
      ...MATERIAL_START,
      materials: [{ kind: "audio", title: "音频材料位（fixture）", path: "", loaded: false, loading: false }]
    };
    const audioService = createProjectFixtureService({ context: CONTEXT, tutorStart: audioStart });
    const { container } = mount(<ProjectSurface fixture={{ context: CONTEXT, service: audioService }} />);
    await flush();
    expect(container.querySelector('[data-tutor-material-kind="audio"]')).not.toBeNull();
    expect(container.querySelector("[data-widget-audio-player]")).not.toBeNull();
    expect(container.querySelector("[data-tutor-material-load]")).toBeNull();

    // v0（无 brief/materials）：无简报块、无材料区（437 契约面不回归）。
    const v0Service = createProjectFixtureService({ context: CONTEXT, tutorStart: OK_OPENING });
    const v0 = mount(<ProjectSurface fixture={{ context: CONTEXT, service: v0Service }} />);
    await flush();
    expect(v0.container.querySelector("[data-tutor-brief]")).toBeNull();
    expect(v0.container.querySelector("[data-tutor-materials]")).toBeNull();
    expect(v0.container.querySelector("[data-tutor-material]")).toBeNull();
  });
});

/* ------------------------------------------------------------------ */
/* SIM-MUSIC-WIDGET-1（441）：材料卡谱面展开（fixture 注入 / 未注入降级）  */
/* ------------------------------------------------------------------ */

// 仓库装置 fixture（只读）：小星星主题 42 音 / 48 拍 / 4/4。
const SCORE_FIXTURE_REL = "../../coord/experiments/tutor-sim-2/assets/theme_twinkle_v0.mid";

function fixtureScoreModel(): ScoreModel {
  const text = readFileSync(join(process.cwd(), SCORE_FIXTURE_REL), "latin1");
  const bytes = new Uint8Array(text.length);
  for (let i = 0; i < text.length; i++) bytes[i] = text.charCodeAt(i) & 0xff;
  return parseMidiScore(bytes).model;
}

describe("SIM-MUSIC-WIDGET-1：材料卡谱面展开（surface 注入链）", () => {
  it("fixture 注入谱面 + 装载成功：谱面入口出现，展开渲染（42 音 / 4/4 SVG）", async () => {
    const service = createProjectFixtureService({
      context: CONTEXT,
      tutorStart: MATERIAL_START,
      loadMaterialCalls: []
    });
    const { container } = mount(
      <ProjectSurface
        fixture={{ context: CONTEXT, service, tutorScores: { "小星星主题（fixture）": fixtureScoreModel() } }}
      />
    );
    await flush();
    // 未装载：谱面入口不渲染（谱面=已装载材料的交互）。
    expect(container.querySelector("[data-tutor-score-toggle]")).toBeNull();

    click(container.querySelector("[data-tutor-material-load]") as Element);
    await flush();
    const material = container.querySelector("[data-tutor-material]");
    expect(material?.getAttribute("data-tutor-material-loaded")).toBe("true");
    expect(material?.getAttribute("data-tutor-score-available")).toBe("true");
    // 默认收起（不撑爆对话流），入口在。
    expect(container.querySelector("[data-tutor-score-toggle]")?.textContent).toContain("查看谱面");
    expect(container.querySelector("[data-tutor-score-svg]")).toBeNull();

    click(container.querySelector("[data-tutor-score-toggle]") as Element);
    await flush();
    const svgHost = container.querySelector("[data-tutor-score-svg]");
    expect(svgHost).not.toBeNull();
    expect(svgHost?.querySelector("svg")).not.toBeNull(); // 374 渲染层复用（VexFlow beat-grid）
    expect(container.querySelector("[data-tutor-score-notes]")?.textContent).toContain("42 音");
    expect(container.querySelector("[data-tutor-score-timesig]")?.textContent).toBe("4/4");
    expect(container.querySelector("[data-tutor-score]")?.getAttribute("data-tutor-score-open")).toBe("true");
  });

  it("未注入谱面：装载成功后无谱面入口（fail-honest 零回归面）", async () => {
    const service = createProjectFixtureService({ context: CONTEXT, tutorStart: MATERIAL_START });
    const { container } = mount(<ProjectSurface fixture={{ context: CONTEXT, service }} />);
    await flush();
    click(container.querySelector("[data-tutor-material-load]") as Element);
    await flush();
    expect(container.querySelector("[data-tutor-material]")?.getAttribute("data-tutor-material-loaded")).toBe("true");
    expect(container.querySelector("[data-tutor-score-toggle]")).toBeNull();
    expect(container.querySelector("[data-tutor-material]")?.getAttribute("data-tutor-score-available")).toBe("false");
  });
});
