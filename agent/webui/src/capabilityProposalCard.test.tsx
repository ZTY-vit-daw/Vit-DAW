import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { CapabilityProposalCard, proposalCardOutcome } from "./App";
import type { AgentInvokeResponse, JsonRecord } from "./types";

const presentation: JsonRecord = {
  schema_version: "vit.proposal_presentation.v1",
  proposal_id: "proposal-1",
  proposal_revision: 0,
  title: "B2 静态主次与音量平衡方案",
  conclusion: "已分析 8 条轨道，推荐以 均衡 策略调整 4 条轨道。",
  recommendation: "推荐候选“均衡”，置信度高，最大单轨变化 1.50 dB。",
  action_count: 4,
  analyzed_tracks: 8,
  risk: "low",
  reversible: true,
  change_groups: [],
  actions: [],
  analysis_summary: ["角色覆盖 92%。"],
  limitations: []
};

const waitingAction: JsonRecord = {
  _ui_source: "interaction",
  _synthetic_confirmation: true,
  id: "confirmation_plan-1",
  kind: "proposal_approval",
  type: "proposal_approval",
  status: "waiting_for_user",
  payload: { plan_id: "plan-1", proposal_presentation: presentation },
  actions: [
    { id: "approve", label: "确认执行", style: "primary", recommended: true },
    { id: "cancel", label: "取消", style: "secondary" }
  ]
};

function renderCard(action: JsonRecord, extras?: { authorityMode?: string; onSendSupplement?: (text: string) => void }) {
  return renderToStaticMarkup(
    <CapabilityProposalCard
      action={action}
      index={0}
      respondingActionID=""
      onInteractionAction={() => {}}
      onInvoke={async () => ({}) as AgentInvokeResponse}
      onSelectArtifact={() => {}}
      uiState={null}
      onMacroValuePreview={() => {}}
      onMacroValueCommit={async () => {}}
      authorityMode={extras?.authorityMode as never}
      onSendSupplement={extras?.onSendSupplement}
    />
  );
}

describe("确认卡 Mondrian 卡族重皮（GUI-T4 ①）", () => {
  it("待裁态：白底卡壳+蓝底页签+改动/假设/预期行+批准/拒绝双席+卡面补充输入", () => {
    const markup = renderCard(waitingAction, { onSendSupplement: () => {} });
    expect(markup).toContain('class="card capability-proposal-card"');
    expect(markup).not.toContain("settled");
    expect(markup).toContain("确认 · 第 1 轮试验步");
    expect(markup).toContain('<span class="chip">待裁</span>');
    expect(markup).toContain('<span class="k">改动</span>');
    expect(markup).toContain('<span class="k">假设</span>');
    expect(markup).toContain('<span class="k">预期</span>');
    expect(markup).toContain("可一键回滚");
    expect(markup).toContain('class="btn approve"');
    expect(markup).toContain('class="btn reject"');
    expect(markup).toContain("确认执行");
    expect(markup).toContain("取消");
    expect(markup).toContain('class="c-ask"');
    expect(markup).toContain("不用选项？直接补充你的要求…");
    expect(markup).toContain("已分析 8 轨");
  });

  it("settled 已批准：灰沉淀壳+tone-blue 结果条，按钮与补充输入隐藏", () => {
    const action = { ...waitingAction, status: "completed", stage: "completed", resolved_action_id: "approve", resolved_at: 1_800_000_000_000, actions: [] };
    const markup = renderCard(action);
    expect(markup).toContain("card capability-proposal-card settled");
    expect(markup).toContain('class="outcome tone-blue"');
    expect(markup).toContain("已批准");
    expect(markup).not.toContain('class="c-ask"');
    expect(markup).not.toContain('class="btn approve"');
  });

  it("settled 已拒绝：tone-red 结果条", () => {
    const action = { ...waitingAction, status: "cancelled", stage: "cancelled", resolved_action_id: "cancel", actions: [] };
    const markup = renderCard(action);
    expect(markup).toContain("card capability-proposal-card settled");
    expect(markup).toContain('class="outcome tone-red"');
    expect(markup).toContain("已拒绝 · 未做任何改动");
  });

  it("settled 补充未采用：supersedes 渲染走 tone-gray 结果条", () => {
    const action = { ...waitingAction, status: "completed", stage: "completed", resolved_action_id: "superseded", actions: [] };
    const markup = renderCard(action);
    expect(markup).toContain("card capability-proposal-card settled");
    expect(markup).toContain('class="outcome tone-gray"');
    expect(markup).toContain("已收到你的补充 · 卡面选项未采用");
  });

  it("完全档下待裁卡静默沉淀为「完全档 · 已直接执行」（UI 层兜底，agent 侧仍发 proposal）", () => {
    const markup = renderCard(waitingAction, { authorityMode: "full_project_access" });
    expect(markup).toContain("card capability-proposal-card settled");
    expect(markup).toContain('class="outcome tone-gray"');
    expect(markup).toContain("完全档 · 已直接执行");
    expect(markup).not.toContain('class="btn approve"');
    expect(markup).not.toContain('class="c-ask"');
  });
});

describe("proposalCardOutcome 推导", () => {
  it("无终态标记时不伪造结果条（无按钮的 waiting 卡保持原样）", () => {
    expect(proposalCardOutcome({ status: "waiting_for_user" }, true)).toBeNull();
    expect(proposalCardOutcome(waitingAction, false)).toBeNull();
  });

  it("turn_terminal_receipt 按本回合收尾渲染，失败落红", () => {
    expect(proposalCardOutcome({ status: "completed", resolved_action_id: "turn_terminal_receipt" }, true)?.tone).toBe("blue");
    expect(proposalCardOutcome({ status: "failed", resolved_action_id: "turn_terminal_receipt" }, true)?.tone).toBe("red");
  });
});
