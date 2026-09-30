import { describe, expect, it } from "vitest";
import { fullAccessDirectExecutionTargets, stripFullAccessDirectExecutionActions, mergeMessageActions } from "./App";
import type { ChatMessage, ChatResponse, JsonRecord } from "./types";

// FIX-CONFIRM-CARD-1 ③（M1 手测缺陷③，2026-09-29 决策侧裁定）：full access 下
// RiskConfirm 级确认闸不出前置卡——webui 渲染前拦截（直执目标提取+剥卡），
// manual_confirmation 行为不变。本文件钉纯函数口径；端到端直执链（应答+
// 回执入流）由 E2E-WEBUI-1 K2 断言。

function confirmationInteraction(kind: string, extras: JsonRecord = {}): JsonRecord {
  return {
    _ui_source: "interaction",
    id: `interaction_${kind}_1`,
    interaction_id: `interaction_${kind}_1`,
    kind,
    type: kind,
    title: "需要确认",
    status: "waiting_for_user",
    actions: [
      { id: "approve", label: "确认执行", style: "primary", recommended: true },
      { id: "cancel", label: "取消", style: "secondary" }
    ],
    ...extras
  };
}

function chatResponse(extras: Partial<ChatResponse> = {}): ChatResponse {
  return {
    conversation_id: "conv_full_access",
    reply: "我准备把 Track 2 提升 1dB，需要你确认后执行。",
    needs_confirmation: true,
    plan_id: "plan_direct_1",
    goal_status: "waiting_confirmation",
    ...extras
  } as ChatResponse;
}

function carrierMessage(actions: JsonRecord[], content: string): ChatMessage {
  return {
    id: "reply_direct",
    role: "assistant",
    content,
    mode: "default",
    artifacts: [],
    actions: actions as ChatMessage["actions"],
    createdAt: Date.now(),
    status: "sent",
    lifecycle: "durable",
    persistence: "local",
    message_kind: "proposal"
  } as ChatMessage;
}

describe("fullAccessDirectExecutionTargets 直执目标提取", () => {
  it("通用 confirmation 与 mix 单步/处理确认闸命中（RiskConfirm 闸口径）", () => {
    const requests = [
      confirmationInteraction("confirmation"),
      confirmationInteraction("mix_tick_confirmation"),
      confirmationInteraction("mix_treatment_confirmation")
    ];
    const targets = fullAccessDirectExecutionTargets(chatResponse({ interaction_requests: requests as ChatResponse["interaction_requests"] }));
    expect(targets).toHaveLength(3);
  });

  it("无 interaction_requests 时退到合成确认卡（plan 确认路径）", () => {
    const targets = fullAccessDirectExecutionTargets(chatResponse());
    expect(targets).toHaveLength(1);
    expect(String(targets[0]?.kind)).toBe("confirmation");
    expect(String(targets[0]?.plan_id)).toBe("plan_direct_1");
  });

  it("capability proposal（proposal_approval）不进直执（GUI-T4 静默沉淀先例）", () => {
    const presentation = { schema_version: "vit.proposal_presentation.v1", proposal_id: "prop_1", title: "B2 方案" };
    const proposal = confirmationInteraction("proposal_approval", {
      workflow: "capability_runtime_v1",
      payload: { plan_id: "prop_1", proposal_presentation: presentation }
    });
    const targets = fullAccessDirectExecutionTargets(chatResponse({
      interaction_requests: [proposal] as ChatResponse["interaction_requests"],
      workflow: "capability_runtime_v1",
      workflow_data: { proposal_presentation: presentation } as unknown as ChatResponse["workflow_data"]
    }));
    expect(targets).toHaveLength(0);
  });

  it("表单/问答类与单钮/选卡类交互不自动执行", () => {
    const withFields = confirmationInteraction("confirmation", { fields: [{ id: "note", label: "备注", required: true }] });
    const selectionCard = confirmationInteraction("plugin_recommendation_selection", {
      actions: [{ id: "pick_a", label: "选 A" }, { id: "pick_b", label: "选 B" }]
    });
    // 不带 needs_confirmation/plan_id：两个请求都被排除后合成兜底也不触发
    const targets = fullAccessDirectExecutionTargets(chatResponse({
      needs_confirmation: false,
      plan_id: "",
      goal_status: "completed",
      reply: "已给出候选。",
      interaction_requests: [withFields, selectionCard] as ChatResponse["interaction_requests"]
    }));
    expect(targets).toHaveLength(0);
  });
});

describe("stripFullAccessDirectExecutionActions 前置卡剥离", () => {
  it("直执目标从入流消息摘除；一次性确认提示改写为直执通告", () => {
    const interaction = confirmationInteraction("mix_tick_confirmation");
    const message = carrierMessage([interaction], "这个操作需要你确认后才会执行。");
    const stripped = stripFullAccessDirectExecutionActions(message, [interaction]);
    expect(stripped.actions).toHaveLength(0);
    expect(stripped.content).toContain("完全访问已开启");
    expect(stripped.content).toContain("回执见下");
  });

  it("非一次性提示内容保留原文；同消息其它动作行不受波及", () => {
    const interaction = confirmationInteraction("confirmation");
    const executedRow = { _ui_source: "executed", id: "exec_1", status: "ok", label: "已应用" };
    const message = carrierMessage([interaction, executedRow], "已完成 2 项调整。");
    const stripped = stripFullAccessDirectExecutionActions(message, [interaction]);
    expect(stripped.actions).toHaveLength(1);
    expect(String((stripped.actions ?? [])[0]?._ui_source)).toBe("executed");
    expect(stripped.content).toBe("已完成 2 项调整。");
  });

  it("无目标时零变化（引用相等）", () => {
    const message = carrierMessage([], "已完成。");
    expect(stripFullAccessDirectExecutionActions(message, [])).toBe(message);
  });
});

// FIX-CONFIRM-CARD-1 ②（E2E K1/K2 实证的按钮复活根因）：已结算交互动作与
// 其等待形态同键合并时，不得从 incoming 复活子按钮——settle/盖章/台账消费
// 后的终稿是权威形态，复活即死卡可点击（点击命中 server 4022 过期口径）。
describe("mergeMessageActions 已结算动作不复活按钮", () => {
  const waitingCard = confirmationInteraction("mix_tick_confirmation");

  it("已结算（摘按钮+resolved_action_id）与等待版合并保持终稿不可交互", () => {
    const settled = { ...waitingCard, status: "completed", resolved_action_id: "turn_expired", actions: [] };
    const merged = mergeMessageActions([settled], [waitingCard]);
    expect(merged).toHaveLength(1);
    expect(String(merged[0]?.resolved_action_id)).toBe("turn_expired");
    expect(merged[0]?.actions).toEqual([]);
  });

  it("台账盖章版（resolved+guard）合并同样保持无按钮", () => {
    const stamped = { ...waitingCard, status: "resolved", resolved_action_id: "consumed_interaction_guard", actions: [] };
    const merged = mergeMessageActions([stamped], [waitingCard]);
    expect(String(merged[0]?.status)).toBe("resolved");
    expect(merged[0]?.actions).toEqual([]);
  });

  it("等待版与等待版合并照旧（按钮保留，不误伤未结算卡）", () => {
    const merged = mergeMessageActions([waitingCard], [{ ...waitingCard }]);
    expect((merged[0]?.actions as JsonRecord[])?.length).toBe(2);
  });
});
