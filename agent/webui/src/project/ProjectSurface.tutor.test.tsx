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

import { ProjectSurface } from "./ProjectSurface";
import { createProjectFixtureService } from "./projectFixtures";
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
