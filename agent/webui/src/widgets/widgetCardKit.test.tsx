import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import {
  DeclarativeInteractionCard,
  declarativeCardFromAction,
  declarativeFieldValuesForPayload
} from "../interactionCardKit";
import type { JsonRecord } from "../types";

// WIDGET-KIT-1（379）：interactionCardKit 音乐控件挂载扩展测试。
// audio_player / compare_pair 注册 + 布局族（行/列/折叠）缺口补齐；
// 事件回流走既有交互回流通道（declarativeFieldValuesForPayload → payload.fields）。

function widgetCardPayload(overrides: JsonRecord = {}): JsonRecord {
  return {
    schema: "interaction_card.v1",
    title: "节奏比较练习",
    fields: [
      { id: "intro", type: "text", label: "说明", value: "先听 A，再听 B" },
      {
        id: "player",
        type: "audio_player",
        label: "示范旋律.wav",
        media: { artifact_id: "art-lesson-1" }
      },
      {
        id: "compare",
        type: "compare_pair",
        label: "两版节奏对比",
        value: "",
        context_id: "compare-lesson-1",
        reason_field: "reason",
        a: { label: "原版节奏", artifact_id: "art-a-1", duration_hint: 12 },
        b: { label: "修改节奏", src: "data:audio/wav;base64,AAAA", duration_hint: 30 }
      },
      { id: "reason", type: "text", label: "理由", value: "" }
    ],
    actions: [
      { action_id: "submit", label: "提交比较", style: "primary" },
      { action_id: "cancel", label: "取消", style: "secondary" }
    ],
    ...overrides
  };
}

function widgetInteractionAction(cardOverrides: JsonRecord = {}, actionOverrides: JsonRecord = {}): JsonRecord {
  return {
    _ui_source: "interaction",
    id: "interaction_widget_1",
    kind: "lesson_compare",
    type: "lesson_compare",
    status: "waiting",
    title: "节奏比较练习",
    body: "听听两个版本，说说你更喜欢哪一版。",
    actions: [
      { id: "submit", label: "提交比较", style: "primary" },
      { id: "cancel", label: "取消", style: "secondary" }
    ],
    payload: { card: widgetCardPayload(cardOverrides) },
    ...actionOverrides
  };
}

describe("declarativeCardFromAction：音乐控件字段解析", () => {
  it("audio_player 字段：media.artifact_id 收进 media.artifactID（走既有 preview URL 通道）", () => {
    const card = declarativeCardFromAction(widgetInteractionAction());
    expect(card).not.toBeNull();
    const field = card!.fields.find((item) => item.id === "player");
    expect(field?.type).toBe("audio_player");
    expect(field?.media).toEqual({ artifactID: "art-lesson-1", src: "" });
  });

  it("compare_pair 字段：a/b 两侧（label/artifactID/src/durationHint）+ context_id + reason_field", () => {
    const card = declarativeCardFromAction(widgetInteractionAction());
    const field = card!.fields.find((item) => item.id === "compare");
    expect(field?.type).toBe("compare_pair");
    expect(field?.sides?.a).toEqual({ label: "原版节奏", artifactID: "art-a-1", src: "", durationHint: 12 });
    expect(field?.sides?.b).toEqual({ label: "修改节奏", artifactID: "", src: "data:audio/wav;base64,AAAA", durationHint: 30 });
    expect(field?.contextID).toBe("compare-lesson-1");
    expect(field?.reasonField).toBe("reason");
  });

  it("负向：compare_pair 非法偏好声明值清空（不代选）；上下文声明里的实验身份字段不收编", () => {
    const card = declarativeCardFromAction(
      widgetInteractionAction({
        fields: (widgetCardPayload().fields as JsonRecord[]).map((field) =>
          field.id === "compare"
            ? { ...field, value: "b-bogus", turn_id: "experiment-turn-99", round_id: "round-9" }
            : field
        )
      })
    );
    const field = card!.fields.find((item) => item.id === "compare");
    expect(field?.value).toBe("");
    expect(JSON.stringify(card)).not.toContain("experiment-turn-99");
    expect(JSON.stringify(card)).not.toContain("round-9");
  });

  it("负向：未知控件类型字段收敛为 text（渲染降级不失败，既有语义不变）", () => {
    const card = declarativeCardFromAction(
      widgetInteractionAction({
        fields: [{ id: "mystery", type: "beat_pad", label: "节拍垫", value: "" }]
      })
    );
    expect(card!.fields[0].type).toBe("text");
    expect(card!.fields[0].media).toBeUndefined();
    expect(card!.fields[0].sides).toBeUndefined();
  });
});

describe("declarativeCardFromAction：布局族（行/列/折叠）", () => {
  it("column + collapse 组解析：字段引用按白名单过滤，collapse 首组默认展开", () => {
    const card = declarativeCardFromAction(
      widgetInteractionAction({
        layout: {
          type: "column",
          groups: [
            { id: "g-compare", title: "对比区", fields: ["player", "compare", "ghost-field"] },
            { id: "g-input", title: "输入区", fields: ["reason"], open: false }
          ]
        }
      })
    );
    expect(card!.layout).toBeDefined();
    expect(card!.layout?.type).toBe("column");
    expect(card!.layout?.groups[0].collapsible).toBe(false);
    // 悬空引用 ghost-field 被过滤
    expect(card!.layout?.groups[0].fieldIDs).toEqual(["player", "compare"]);
    expect(card!.layout?.groups[1].collapsible).toBe(false);
    expect(card!.layout?.groups[1].defaultOpen).toBe(false);
    // intro/reason 未编入组 → 隐式尾组不丢字段
    expect(card!.layout?.groups.at(-1)?.id).toBe("group-rest");
    expect(card!.layout?.groups.at(-1)?.fieldIDs).toEqual(["intro"]);
  });

  it("collapse 布局：组可折叠且默认首组展开；row 布局组不可折叠", () => {
    const collapse = declarativeCardFromAction(
      widgetInteractionAction({ layout: { type: "collapse", groups: [{ id: "fold-1", title: "点开播放器", fields: ["player", "compare"] }] } })
    );
    expect(collapse!.layout?.type).toBe("collapse");
    expect(collapse!.layout?.groups[0].collapsible).toBe(true);
    expect(collapse!.layout?.groups[0].defaultOpen).toBe(true);
    const row = declarativeCardFromAction(
      widgetInteractionAction({ layout: { type: "row", groups: [{ id: "r-1", fields: ["player", "compare"] }] } })
    );
    expect(row!.layout?.type).toBe("row");
    expect(row!.layout?.groups.every((group) => !group.collapsible)).toBe(true);
  });

  it("负向：layout 类型未知 / groups 无有效字段 → undefined（回退平铺，渲染降级不失败）", () => {
    const badType = declarativeCardFromAction(widgetInteractionAction({ layout: { type: "tabs", groups: [{ id: "t", fields: ["intro"] }] } }));
    expect(badType!.layout).toBeUndefined();
    const dangling = declarativeCardFromAction(
      widgetInteractionAction({ layout: { type: "row", groups: [{ id: "t", fields: ["ghost-only"] }] } })
    );
    expect(dangling!.layout).toBeUndefined();
    const noLayout = declarativeCardFromAction(widgetInteractionAction());
    expect(noLayout!.layout).toBeUndefined();
  });
});

describe("DeclarativeInteractionCard 渲染：控件挂载 + 布局容器", () => {
  it("控件按声明渲染：卡内出现轻量播放器与 A/B 对比组合（含偏好四态与两侧媒体）", () => {
    const action = widgetInteractionAction();
    const card = declarativeCardFromAction(action)!;
    const markup = renderToStaticMarkup(
      <DeclarativeInteractionCard action={action} card={card} respondingActionID="" onInteractionAction={() => {}} />
    );
    expect(markup).toContain('data-card-schema="interaction_card.v1"');
    expect(markup).toContain('data-field-type="audio_player"');
    expect(markup).toContain('data-field-type="compare_pair"');
    expect(markup).toContain('data-widget-audio-player="示范旋律.wav"');
    expect(markup).toContain("/agent/artifact/file?id=art-lesson-1");
    expect(markup).toContain('data-widget-compare-pair="两版节奏对比"');
    expect(markup).toContain('data-compare-context="compare-lesson-1"');
    expect(markup).toContain('data-compare-side="a"');
    expect(markup).toContain('data-compare-side="b"');
    expect(markup).toContain('data-compare-preference="neither"');
    // 理由字段已声明 → 理由输入在场（回流目标）
    expect(markup).toContain("data-compare-reason");
    // 位置规则披露（红线③容器面）
    expect(markup).toContain('data-compare-position-rule="per-side"');
  });

  it("布局声明渲染：data-card-layout + 组容器（collapse 用 details 折叠语义）", () => {
    const action = widgetInteractionAction({
      layout: {
        type: "collapse",
        groups: [
          { id: "fold", title: "点开对比", fields: ["player", "compare"] }
        ]
      }
    });
    const card = declarativeCardFromAction(action)!;
    const markup = renderToStaticMarkup(
      <DeclarativeInteractionCard action={action} card={card} respondingActionID="" onInteractionAction={() => {}} />
    );
    expect(markup).toContain('data-card-layout="collapse"');
    expect(markup).toContain('data-layout-group="fold"');
    expect(markup).toContain("interaction-collapse");
    expect(markup).toContain("<summary");
    expect(markup).toContain("点开对比");
    // 隐式尾组：intro 未编入 → 不丢字段
    expect(markup).toContain('data-layout-group="group-rest"');
    expect(markup).toContain('data-field-id="intro"');
  });

  it("负向：无 layout 声明回退既有平铺（interaction-fields 在场、无布局容器标记）", () => {
    const action = widgetInteractionAction();
    const card = declarativeCardFromAction(action)!;
    const markup = renderToStaticMarkup(
      <DeclarativeInteractionCard action={action} card={card} respondingActionID="" onInteractionAction={() => {}} />
    );
    expect(markup).toContain("interaction-fields");
    expect(markup).not.toContain("data-card-layout=");
    expect(markup).not.toContain("interaction-layout");
  });

  it("负向：compare_pair 未声明理由字段 → 不渲染理由输入（回流面诚实披露）", () => {
    const payload = widgetCardPayload();
    const fields = (payload.fields as JsonRecord[]).map((field) =>
      field.id === "compare" ? { ...field, reason_field: "" } : field
    );
    const action = widgetInteractionAction({ fields });
    const card = declarativeCardFromAction(action)!;
    const compareField = card.fields.find((field) => field.id === "compare");
    expect(compareField?.reasonField).toBeUndefined();
    const markup = renderToStaticMarkup(
      <DeclarativeInteractionCard action={action} card={card} respondingActionID="" onInteractionAction={() => {}} />
    );
    expect(markup).toContain('data-widget-compare-pair="两版节奏对比"');
    expect(markup).not.toContain("data-compare-reason");
  });

  it("负向：audio_player 无 media 声明 → 诚实降级占位（不渲染可播面）", () => {
    const payload = widgetCardPayload();
    const fields = (payload.fields as JsonRecord[]).map((field) =>
      field.id === "player" ? { id: "player", type: "audio_player", label: "示范旋律.wav", value: "" } : field
    );
    const action = widgetInteractionAction({ fields });
    const card = declarativeCardFromAction(action)!;
    const markup = renderToStaticMarkup(
      <DeclarativeInteractionCard action={action} card={card} respondingActionID="" onInteractionAction={() => {}} />
    );
    expect(markup).toContain('data-widget-unavailable="audio_player"');
    // 降级面：该字段不渲染可播播放器（同卡 compare_pair 两侧自身的 audio 元素不受影响）
    expect(markup).not.toContain('data-widget-audio-player="示范旋律.wav"');
  });
});

describe("事件回流：偏好/理由经既有 payload.fields 通道上行", () => {
  it("declarativeFieldValuesForPayload 携带 compare_pair 偏好值与理由字段值", () => {
    const card = declarativeCardFromAction(widgetInteractionAction())!;
    const payload = declarativeFieldValuesForPayload(card, {
      intro: "先听 A，再听 B",
      player: "",
      compare: "a",
      reason: "A 的重拍更稳"
    });
    expect(payload).toEqual({
      fields: {
        intro: "先听 A，再听 B",
        compare: "a",
        reason: "A 的重拍更稳"
      }
    });
    // 偏好四态均可经既有通道上行
    for (const preference of ["b", "neither", "unsure"]) {
      expect(declarativeFieldValuesForPayload(card, { compare: preference })).toEqual({ fields: { compare: preference } });
    }
  });
});
