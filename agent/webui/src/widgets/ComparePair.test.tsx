// @vitest-environment jsdom
// WIDGET-KIT-1（379）：A/B 对比组合组件测试（三红线各至少一正一负）。
// jsdom + act 交互模式沿用 SCORE-WORKBENCH-1 M2 先例（react-dom/client + React19 act，
// 零新测试依赖）；其余渲染断言走 renderToStaticMarkup。
import { createElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  COMPARE_PREFERENCE_VALUES,
  COMPARE_POSITION_RULE,
  ComparePair,
  compareAuditionEvent,
  comparePreferenceEvent,
  compareReasonEvent,
  normalizeCompareContext,
  type CompareContext,
  type CompareEvent
} from "./ComparePair";

(globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const sideA = { label: "原版节奏", src: "data:audio/wav;base64,AAAA", durationHint: 12 };
const sideB = { label: "修改节奏", src: "data:audio/wav;base64,AAAA", durationHint: 30 };

function mount(element: React.ReactElement): { root: Root; container: HTMLElement } {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  act(() => {
    root.render(element);
  });
  return { root, container };
}

function fire(target: Element, type: string, eventInit: EventInit = {}): void {
  const event = type === "keydown"
    ? new KeyboardEvent(type, { bubbles: true, cancelable: true, key: "Enter", ...eventInit })
    : new MouseEvent(type, { bubbles: true, cancelable: true, ...eventInit });
  act(() => {
    target.dispatchEvent(event);
  });
}

// React 受控 input 的 value tracker：直接赋 .value 会被 React 判定未变化而去重，
// 必须经 prototype 原生 setter 写入再派发 input 事件，onChange 才会触发。
function typeInput(input: HTMLInputElement, value: string): void {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
  act(() => {
    setter?.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

afterEach(() => {
  document.body.innerHTML = "";
});

describe("红线①：比较上下文白名单（不伪造旧实验 turn/round 身份）", () => {
  it("正向：normalizeCompareContext 只收 compare 自身身份域", () => {
    const context = normalizeCompareContext({
      context_id: "compare-lesson-1",
      surface_id: "lesson-surface",
      material_ref: "material-7",
      turn_id: "experiment-turn-99",
      round_id: "round-9",
      experimentTurnID: "exp-t",
      source_turn_id: "run-1"
    });
    expect(context.contextID).toBe("compare-lesson-1");
    expect(context.surfaceID).toBe("lesson-surface");
    expect(context.materialRef).toBe("material-7");
    const keys = Object.keys(context);
    expect(keys).toEqual(expect.arrayContaining(["contextID"]));
    expect(keys.some((key) => key.toLowerCase().includes("turn"))).toBe(false);
    expect(keys.some((key) => key.toLowerCase().includes("round"))).toBe(false);
    expect(JSON.stringify(context)).not.toContain("experiment-turn-99");
    expect(JSON.stringify(context)).not.toContain("round-9");
  });

  it("负向：只喂 turn/round 实验身份 → 不收编为 contextID，生成组件自有身份", () => {
    const context = normalizeCompareContext({ turn_id: "experiment-turn-99", round_id: "round-9" }, "field-compare-1");
    expect(context.contextID).toBe("field-compare-1");
    expect(JSON.stringify(context)).not.toContain("experiment-turn-99");
    expect(JSON.stringify(context)).not.toContain("round-9");
    // 缺 fallback 时生成组件前缀自有身份，绝不借用实验键
    expect(normalizeCompareContext(null).contextID.startsWith("compare_pair-")).toBe(true);
  });

  it("事件构造器输出无 turn/round 键（audition/preference/reason 全事件族）", () => {
    const context: CompareContext = { contextID: "c1" };
    for (const event of [
      compareAuditionEvent(context, "a"),
      comparePreferenceEvent(context, "b", "B 的落音更稳"),
      compareReasonEvent(context, "", "还想再改")
    ]) {
      const keys = Object.keys(event);
      expect(keys.some((key) => key.toLowerCase().includes("turn"))).toBe(false);
      expect(keys.some((key) => key.toLowerCase().includes("round"))).toBe(false);
    }
  });
});

describe("红线②：偏好只记录偏好事件（不路由混音裁决落账）", () => {
  it("正向：点击偏好回流 kind=preference 事件（含偏好/理由/上下文/位置规则），onPreference 缝写回", () => {
    const events: CompareEvent[] = [];
    const onPreference = vi.fn();
    const { container } = mount(
      <ComparePair
        a={sideA}
        b={sideB}
        context={{ contextID: "compare-x" }}
        reason="理由初稿"
        onCompareEvent={(event) => events.push(event)}
        onPreference={onPreference}
      />
    );
    const pickA = container.querySelector('[data-compare-preference="a"]');
    expect(pickA).not.toBeNull();
    fire(pickA!, "click");
    const preferenceEvents = events.filter((event) => event.kind === "preference");
    expect(preferenceEvents.length).toBe(1);
    const event = preferenceEvents[0] as Extract<CompareEvent, { kind: "preference" }>;
    expect(event.preference).toBe("a");
    expect(event.contextID).toBe("compare-x");
    expect(event.positionRule).toBe("per-side");
    // 负向面（内嵌）：事件键族里没有任何裁决/落账语义字段
    expect(Object.keys(event).some((key) => ["judgment", "settle", "retain", "rollback", "adopt", "settlement"].includes(key))).toBe(false);
    expect(onPreference).toHaveBeenCalledWith("a");
  });

  it("正向：四态偏好齐备（A/B/都不喜欢/不确定），不缺『都不喜欢』兜底", () => {
    expect(COMPARE_PREFERENCE_VALUES).toEqual(["a", "b", "neither", "unsure"]);
    const markup = renderToStaticMarkup(<ComparePair a={sideA} b={sideB} />);
    for (const value of COMPARE_PREFERENCE_VALUES) {
      expect(markup).toContain(`data-compare-preference="${value}"`);
    }
    expect(markup).toContain("都不喜欢");
    expect(markup).toContain("不确定");
  });

  it("负向：比较面不带混音裁决路由痕迹（无 audition 会话/判定键/回滚保留措辞）", () => {
    const markup = renderToStaticMarkup(<ComparePair a={sideA} b={sideB} title="课堂比较" />);
    expect(markup).not.toContain("data-audition-session");
    expect(markup).not.toContain("data-act=");
    expect(markup).not.toContain("回滚");
    expect(markup).not.toContain("保留改动");
    expect(markup).not.toContain("判定");
    // 组件模块不提供任何 judgment 形状回调：可编程面只有 onCompareEvent/onPreference/onReason
    const reasonEvents: CompareEvent[] = [];
    const { container } = mount(
      <ComparePair a={sideA} b={sideB} onCompareEvent={(event) => reasonEvents.push(event)} />
    );
    const reasonInput = container.querySelector("[data-compare-reason]");
    expect(reasonInput).not.toBeNull();
    typeInput(reasonInput as HTMLInputElement, "两版都想再改");
    fire(reasonInput!, "keydown");
    const kinds = reasonEvents.map((event) => event.kind);
    expect(kinds).toContain("reason");
    expect(kinds.every((kind) => ["audition", "preference", "reason"].includes(kind))).toBe(true);
    const reasonEvent = reasonEvents.find((event) => event.kind === "reason") as Extract<CompareEvent, { kind: "reason" }>;
    // 理由事件不代选偏好（未点偏好时 preference 为空串）
    expect(reasonEvent.preference).toBe("");
    expect(reasonEvent.reason).toBe("两版都想再改");
  });
});

describe("红线③：异长版本各自绑定坐标（不强行共轴）", () => {
  it("正向：两实例独立坐标披露 + 事件/容器 positionRule=per-side", () => {
    const events: CompareEvent[] = [];
    const markup = renderToStaticMarkup(
      <ComparePair a={sideA} b={sideB} context={{ contextID: "c3" }} onCompareEvent={(event) => events.push(event)} />
    );
    expect(markup).toContain('data-compare-position-rule="per-side"');
    expect(COMPARE_POSITION_RULE).toBe("per-side");
    expect(markup).toContain('data-compare-side="a"');
    expect(markup).toContain('data-compare-side="b"');
    // 各自绑定坐标：两侧 duration-hint 独立披露（12 vs 30），互不映射
    expect(markup).toContain('data-audio-duration-hint="12"');
    expect(markup).toContain('data-audio-duration-hint="30"');
    expect(compareAuditionEvent({ contextID: "c3" }, "b").positionRule).toBe("per-side");
    expect(comparePreferenceEvent({ contextID: "c3" }, "unsure").positionRule).toBe("per-side");
  });

  it("负向：无共享时间轴标记（不渲染 synced/shared-axis 类共轴面）", () => {
    const markup = renderToStaticMarkup(<ComparePair a={sideA} b={sideB} layout="row" />);
    expect(markup).not.toContain("data-compare-synced");
    expect(markup).not.toContain("data-timeline-shared");
    expect(markup).not.toContain("data-shared-axis");
    expect(markup).not.toContain("data-compare-sync");
  });
});

describe("播放互斥（同刻只响一版）与试听事件", () => {
  it("A 侧起播 → 回流 audition 事件（side/contextID/per-side），B 侧收到暂停 nonce", () => {
    const events: CompareEvent[] = [];
    const { container } = mount(
      <ComparePair a={sideA} b={sideB} context={{ contextID: "c4" }} onCompareEvent={(event) => events.push(event)} />
    );
    const audioElements = container.querySelectorAll("audio[data-audio-element]");
    expect(audioElements.length).toBe(2);
    // jsdom 无真实解码：直接在生产 play 事件处理器路径派发 play 事件
    act(() => {
      audioElements[0].dispatchEvent(new Event("play"));
    });
    const auditionEvents = events.filter((event) => event.kind === "audition");
    expect(auditionEvents.length).toBe(1);
    const event = auditionEvents[0] as Extract<CompareEvent, { kind: "audition" }>;
    expect(event.side).toBe("a");
    expect(event.contextID).toBe("c4");
    expect(event.positionRule).toBe("per-side");
    // 互斥落点：B 侧 AudioPlayer 的 pauseNonce prop 已 bump（data-compare-active-side 标记 A 活跃）
    expect(container.querySelector('[data-compare-side="a"]')?.getAttribute("data-compare-active-side")).toBe("true");
    expect(container.querySelector('[data-compare-side="b"]')?.getAttribute("data-compare-active-side")).toBe("false");
  });
});

describe("卡内受控挂载（interactionCardKit 回流通道对齐）", () => {
  it("受控偏好/理由：写回经 onPreference/onReason 缝，内部态不越权", () => {
    const onPreference = vi.fn();
    const onReason = vi.fn();
    const { container, root } = mount(
      <ComparePair a={sideA} b={sideB} preference="" reason="" reasonEnabled onPreference={onPreference} onReason={onReason} />
    );
    fire(container.querySelector('[data-compare-preference="neither"]')!, "click");
    expect(onPreference).toHaveBeenCalledWith("neither");
    const reasonInput = container.querySelector("[data-compare-reason]") as HTMLInputElement;
    typeInput(reasonInput, "节奏还可以更紧");
    expect(onReason).toHaveBeenCalledWith("节奏还可以更紧");
    root.unmount();
  });

  it("reasonEnabled=false：卡内未声明理由字段时不渲染理由输入", () => {
    const markup = renderToStaticMarkup(<ComparePair a={sideA} b={sideB} reasonEnabled={false} />);
    expect(markup).not.toContain("data-compare-reason");
  });
});
