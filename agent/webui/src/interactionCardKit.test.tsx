import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { ActionCard } from "./App";
import {
  DeclarativeInteractionCard,
  declarativeCardFromAction,
  declarativeFieldValuesForPayload
} from "./interactionCardKit";
import type { JsonRecord } from "./types";

// 编舞确认卡的声明式契约镜像（与 agent 侧 automationChoreoDeclarativeCard
// 同构——interaction_card.v1：六字段+确认/取消两动作）。
function choreoCardPayload(): JsonRecord {
  return {
    candidate_id: "automation_choreo_t1",
    track_id: "track-1",
    category: "level",
    card: {
      schema: "interaction_card.v1",
      title: "段落包络写入方案（确认后执行）",
      fields: [
        {
          id: "shape",
          type: "select",
          label: "形状",
          value: "fade_out",
          options: [
            { value: "fade_in", label: "渐入（0.30 → 0.60）" },
            { value: "fade_out", label: "渐出（0.60 → 0.30）" }
          ],
          description: "渐入/渐出/保持/推进缺省形状库"
        },
        { id: "start_value", type: "number", label: "起点值", value: "0.60", min: 0, max: 1, step: 0.01, description: "电平类为音量 raw 值 [0,1]" },
        { id: "end_value", type: "number", label: "终点值", value: "0.30", min: 0, max: 1, step: 0.01 },
        { id: "curve", type: "range", label: "曲率 c", value: "0.00", min: -1, max: 1, step: 0.05, description: "[-1,1]" },
        { id: "region_start", type: "number", label: "区间起点", value: "0.0", min: 0, unit: "s" },
        { id: "region_end", type: "number", label: "区间终点", value: "30.0", min: 0, unit: "s", description: "跨度上限 300 秒" }
      ],
      actions: [
        { action_id: "approve", label: "确认写入", style: "primary" },
        { action_id: "cancel", label: "取消", style: "secondary" }
      ]
    }
  };
}

function choreoInteractionAction(overrides: JsonRecord = {}): JsonRecord {
  return {
    _ui_source: "interaction",
    id: "interaction_choreo_1",
    kind: "automation_choreo_confirmation",
    type: "automation_choreo_confirmation",
    workflow: "section_automation_choreo",
    stage: "pending_confirmation",
    title: "段落包络写入方案（确认后执行）",
    body: "把轨道 1 的音量在 0.0s → 30.0s 内渐出。确认前不会修改工程；形状、幅度和区间都可以在下面改。",
    status: "waiting",
    fields: [
      { id: "shape", label: "形状", kind: "choice", value: "fade_out" },
      { id: "start_value", label: "起点值", kind: "text", value: "0.60" }
    ],
    actions: [
      { id: "approve", label: "确认写入", style: "primary", recommended: true },
      { id: "cancel", label: "取消", style: "secondary" }
    ],
    review_items: [
      { id: "target", title: "目标参数", body: "轨道 1：音量（电平类）", status: "info" }
    ],
    payload: choreoCardPayload(),
    ...overrides
  };
}

const cardProps = {
  index: 0,
  respondingActionID: "",
  onInteractionAction: () => {},
  onInvoke: (async () => ({})) as never,
  onSelectArtifact: () => {},
  uiState: null,
  onMacroValuePreview: () => {},
  onMacroValueCommit: (async () => {}) as never
};

describe("declarativeCardFromAction（契约提取）", () => {
  it("无 payload.card 的既有卡返回 null（走原渲染分支）", () => {
    expect(declarativeCardFromAction({ id: "i1", kind: "confirmation", payload: { plan_id: "p1" } })).toBeNull();
    expect(declarativeCardFromAction({ id: "i2", payload: {} })).toBeNull();
    expect(declarativeCardFromAction({ id: "i3" })).toBeNull();
  });

  it("card 有字段但无 actions 返回 null（无可执行面不算声明卡）", () => {
    const action = choreoInteractionAction({ payload: { ...choreoCardPayload(), card: { schema: "interaction_card.v1", title: "t", fields: [{ id: "a", type: "text", label: "A", value: "1" }] } } });
    expect(declarativeCardFromAction(action)).toBeNull();
  });

  it("编舞卡契约完整解析：六字段类型/边界/单位+两动作", () => {
    const card = declarativeCardFromAction(choreoInteractionAction());
    expect(card).not.toBeNull();
    expect(card?.schema).toBe("interaction_card.v1");
    expect(card?.title).toBe("段落包络写入方案（确认后执行）");
    expect(card?.fields.map((field) => field.id)).toEqual(["shape", "start_value", "end_value", "curve", "region_start", "region_end"]);
    expect(card?.fields.map((field) => field.type)).toEqual(["select", "number", "number", "range", "number", "number"]);
    const curve = card?.fields.find((field) => field.id === "curve");
    expect(curve?.min).toBe(-1);
    expect(curve?.max).toBe(1);
    const regionEnd = card?.fields.find((field) => field.id === "region_end");
    expect(regionEnd?.unit).toBe("s");
    const shape = card?.fields.find((field) => field.id === "shape");
    expect(shape?.options.map((option) => option.value)).toEqual(["fade_in", "fade_out"]);
    expect(card?.actions).toEqual([
      { action_id: "approve", label: "确认写入", style: "primary" },
      { action_id: "cancel", label: "取消", style: "secondary" }
    ]);
  });

  it("未知类型收敛为 text、未知 style 收敛为 secondary（渲染降级不失败）", () => {
    const action = choreoInteractionAction({
      payload: {
        card: {
          schema: "interaction_card.v1",
          title: "t",
          fields: [{ id: "f1", type: "color", label: "颜色", value: "#fff" }],
          actions: [{ action_id: "go", label: "执行", style: "wild" }]
        }
      }
    });
    const card = declarativeCardFromAction(action);
    expect(card?.fields[0].type).toBe("text");
    expect(card?.actions[0].style).toBe("secondary");
  });
});

describe("declarativeFieldValuesForPayload（点击回传载荷）", () => {
  it("字段编辑值组成 payload.fields——id 为键、字符串值（server 侧 executeAutomationChoreo 读同键）", () => {
    const card = declarativeCardFromAction(choreoInteractionAction());
    expect(card).not.toBeNull();
    const payload = declarativeFieldValuesForPayload(card!, {
      shape: "fade_out",
      start_value: "0.55",
      end_value: "0.20",
      curve: "-0.30",
      region_start: "0.0",
      region_end: "45.0"
    });
    expect(payload).toEqual({
      fields: {
        shape: "fade_out",
        start_value: "0.55",
        end_value: "0.20",
        curve: "-0.30",
        region_start: "0.0",
        region_end: "45.0"
      }
    });
  });

  it("空值字段不进入回传载荷", () => {
    const card = declarativeCardFromAction(choreoInteractionAction());
    const payload = declarativeFieldValuesForPayload(card!, { shape: "", start_value: "0.5" });
    expect(payload).toEqual({ fields: { start_value: "0.5" } });
  });
});

describe("DeclarativeInteractionCard 渲染（组件级）", () => {
  it("编舞卡按声明渲染：标题+六字段（类型锚点）+确认/取消按钮（样式与 action_id）", () => {
    const action = choreoInteractionAction();
    const card = declarativeCardFromAction(action);
    const markup = renderToStaticMarkup(
      <DeclarativeInteractionCard action={action} card={card!} respondingActionID="" onInteractionAction={() => {}} />
    );
    expect(markup).toContain('data-card-schema="interaction_card.v1"');
    expect(markup).toContain('data-interaction-id="interaction_choreo_1"');
    expect(markup).toContain("段落包络写入方案（确认后执行）");
    expect(markup).toContain('data-field-id="shape"');
    expect(markup).toContain('data-field-type="select"');
    expect(markup).toContain('data-field-id="curve"');
    expect(markup).toContain('data-field-type="range"');
    expect(markup).toContain('data-field-id="region_end"');
    expect(markup).toContain('data-action-id="approve"');
    expect(markup).toContain('data-action-id="cancel"');
    expect(markup).toContain("action-button primary");
    expect(markup).toContain("action-button secondary");
    expect(markup).toContain("确认写入");
    expect(markup).toContain("取消");
    // 区间单位披露在场
    expect(markup).toContain("declarative-unit");
  });

  it("settle 态（已确认）不出按钮——只读沉淀形态", () => {
    const settled = choreoInteractionAction({ status: "completed", resolved_action_id: "approve", actions: [] });
    const card = declarativeCardFromAction(settled);
    expect(card).not.toBeNull();
    const markup = renderToStaticMarkup(
      <DeclarativeInteractionCard action={settled} card={card!} respondingActionID="" onInteractionAction={() => {}} />
    );
    expect(markup).toContain("已确认");
    expect(markup).not.toContain('data-action-id="approve"');
    expect(markup).not.toContain("action-buttons");
  });

  it("settle 态（已取消）徽标正确", () => {
    const cancelled = choreoInteractionAction({ status: "cancelled", resolved_action_id: "cancel", actions: [] });
    const card = declarativeCardFromAction(cancelled);
    const markup = renderToStaticMarkup(
      <DeclarativeInteractionCard action={cancelled} card={card!} respondingActionID="" onInteractionAction={() => {}} />
    );
    expect(markup).toContain("已取消");
  });
});

describe("ActionCard 分发（接入面）", () => {
  it("带 card 契约的编舞卡走通用声明式渲染器", () => {
    const markup = renderToStaticMarkup(<ActionCard {...cardProps} action={choreoInteractionAction()} />);
    expect(markup).toContain('data-card-schema="interaction_card.v1"');
    expect(markup).toContain('data-action-id="approve"');
  });

  it("既有特制卡零回退：无 card 契约的 mix 处理卡仍走原分支（非声明式渲染）", () => {
    const mixAction: JsonRecord = {
      _ui_source: "interaction",
      id: "interaction_mix_1",
      kind: "mix_treatment_confirmation",
      type: "mix_treatment_confirmation",
      status: "waiting",
      actions: [{ id: "approve", label: "确认", style: "primary" }],
      payload: { kind: "gain_balance", delta_db: "-1.5" }
    };
    expect(declarativeCardFromAction(mixAction)).toBeNull();
    const markup = renderToStaticMarkup(<ActionCard {...cardProps} action={mixAction} />);
    expect(markup).not.toContain("data-card-schema");
  });

  it("既有通用确认卡（合成 confirmation）零回退", () => {
    const synthetic: JsonRecord = {
      _ui_source: "interaction",
      _synthetic_confirmation: true,
      id: "confirmation_plan-9",
      kind: "confirmation",
      type: "confirmation",
      status: "waiting_for_user",
      plan_id: "plan-9",
      actions: [
        { id: "approve", label: "确认执行", style: "primary" },
        { id: "cancel", label: "取消", style: "secondary" }
      ],
      payload: { plan_id: "plan-9" }
    };
    expect(declarativeCardFromAction(synthetic)).toBeNull();
    const markup = renderToStaticMarkup(<ActionCard {...cardProps} action={synthetic} />);
    expect(markup).toContain("需要确认");
    expect(markup).not.toContain("data-card-schema");
  });
});
