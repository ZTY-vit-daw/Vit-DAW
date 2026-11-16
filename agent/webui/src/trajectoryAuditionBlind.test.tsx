import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { emptyAuditionState, reduceAuditionEvents, type AuditionSession } from "./audition";
import { emptyTrajectoryState } from "./trajectory";
import type { AgentEvent } from "./types";
import { AuditionJudgeCard, auditionBlindDisclosureText, auditionCardOutcome, noDifferenceJudgmentPayload } from "./trajectory/TrajectoryAuditionPanel";

// B12-1：盲态会话的顺序是随机的。判定前面板不得出现「改动前/改动后」这类物理指派
// 措辞；判定落账后按 blind_disclosure 的物理指派 + 实际动作生成解盲条文案。

const blindReady: AgentEvent = {
  seq: 1, type: "audition.ready", item_id: "audition-blind", conversation_id: "conversation-1",
  payload: { schema_version: "vit.kernel_audition.v1", session: {
    session_id: "audition-blind", conversation_id: "conversation-1", turn_id: "turn-1", round_id: "round-1",
    project_revision: "rev-1", status: "ready", blind: true,
    candidates: [
      { id: "candidate-a", label: "A", status: "ready", preview_ref: "a" },
      { id: "candidate-b", label: "B", status: "ready", preview_ref: "b" }
    ]
  } }
};
const blindRequested: AgentEvent = { seq: 2, type: "trajectory.user_judgment.requested", conversation_id: "conversation-1", payload: { schema_version: "vit.observable_trajectory.v1", turn_id: "turn-1", round_id: "round-1", details: { audition_session_id: "audition-blind" } } };
const blindRecorded: AgentEvent = { seq: 3, type: "trajectory.user_judgment.recorded", conversation_id: "conversation-1", payload: { schema_version: "vit.observable_trajectory.v1", turn_id: "turn-1", round_id: "round-1", details: { audition_session_id: "audition-blind", blind: true, evidence: { id: "evidence-blind", preference: "a" } } } };
const blindDisclosure: AgentEvent = { seq: 4, type: "audition.blind_disclosure", item_id: "audition-blind", conversation_id: "conversation-1", payload: { schema_version: "vit.kernel_audition.v1", blind_disclosure: {
  schema_version: "vit.audition_blind_disclosure.v1", blind: true,
  candidate_a_physical: "after", candidate_b_physical: "before", mapping_source: "render_revision",
  selected: "a", selected_physical: "after", action: "retain", summary: "你选的 A 是改动后状态 · 已保留"
}, session: {
  session_id: "audition-blind", conversation_id: "conversation-1", turn_id: "turn-1", round_id: "round-1",
  project_revision: "rev-1", status: "ready", blind: true
} } };

function sessionFromEvents(events: AgentEvent[]): AuditionSession {
  const state = reduceAuditionEvents(emptyAuditionState(), events);
  const session = Object.values(state.sessions)[0];
  if (!session) throw new Error("blind fixture must produce one audition session");
  return session;
}

function renderCard(session: AuditionSession): string {
  return renderToStaticMarkup(
    <AuditionJudgeCard trajectory={emptyTrajectoryState()} session={session} busySessionID="" onSelect={async () => {}} onStop={async () => {}} onSubmitJudgment={async () => {}} />
  );
}

describe("盲测判定卡（B12-1）", () => {
  it("reducer 记下盲态标志，判定前没有解盲披露", () => {
    const session = sessionFromEvents([blindReady, blindRequested]);
    expect(session.blind).toBe(true);
    expect(session.blindDisclosure).toBeNull();
    expect(auditionBlindDisclosureText(session)).toBe("");
  });

  it("判定前不出现「改动前/改动后」措辞，裁决按钮也不预支动作方向", () => {
    const markup = renderCard(sessionFromEvents([blindReady, blindRequested]));
    expect(markup).toContain("盲测 · 顺序随机");
    expect(markup).not.toContain("改动前");
    expect(markup).not.toContain("改动后");
    expect(markup).toContain("A 更好");
    expect(markup).not.toContain("A 更好 · 回滚");
    expect(markup).not.toContain("B 更好 · 保留");
  });

  it("次级按钮：听不出差别（heard=no 直报）与说不清/另有想法（展开 free_text）", () => {
    const session = sessionFromEvents([blindReady, blindRequested]);
    const markup = renderCard(session);
    expect(markup).toContain("听不出差别");
    expect(markup).toContain("说不清 / 另有想法");
    expect(noDifferenceJudgmentPayload(session)).toMatchObject({
      conversation_id: "conversation-1", turn_id: "turn-1", round_id: "round-1",
      audition_session_id: "audition-blind", project_revision: "rev-1",
      heard_difference: "no", preference: "unsure", free_text: ""
    });
  });

  it("解盲披露驱动沉淀条：按物理指派 + 实际动作说话", () => {
    const session = sessionFromEvents([blindReady, blindRequested, blindRecorded, blindDisclosure]);
    expect(session.blindDisclosure?.candidate_a_physical).toBe("after");
    expect(auditionBlindDisclosureText(session)).toBe("你选的 A 是改动后状态 · 已保留");
    expect(auditionCardOutcome(session, "improved", false)).toMatchObject({ tone: "blue", icon: "check", text: "你选的 A 是改动后状态 · 已保留" });
    // 恢复快照先到（尚未收到 judgment.recorded）时也必须按披露说话，不得退回标签级文案
    const restored = { ...session, judgmentRecorded: false };
    expect(auditionCardOutcome(restored, "rolled_back", false)).toMatchObject({ tone: "blue", icon: "check", text: "你选的 A 是改动后状态 · 已保留" });
    const markup = renderCard(session);
    expect(markup).toContain("你选的 A 是改动后状态 · 已保留");
    expect(markup).not.toContain("A 更好 · 回滚");
  });

  it("非盲会话零回退：仍用「改动前/改动后」与既有裁决文案", () => {
    const ready: AgentEvent = { seq: 1, type: "audition.ready", item_id: "audition-plain", conversation_id: "conversation-1", payload: { schema_version: "vit.kernel_audition.v1", session: {
      session_id: "audition-plain", conversation_id: "conversation-1", turn_id: "turn-1", round_id: "round-1", project_revision: "rev-1", status: "ready",
      candidates: [{ id: "candidate-a", label: "A", status: "ready", preview_ref: "a" }, { id: "candidate-b", label: "B", status: "ready", preview_ref: "b" }]
    } } };
    const requested: AgentEvent = { seq: 2, type: "trajectory.user_judgment.requested", conversation_id: "conversation-1", payload: { schema_version: "vit.observable_trajectory.v1", turn_id: "turn-1", round_id: "round-1", details: { audition_session_id: "audition-plain" } } };
    const session = sessionFromEvents([ready, requested]);
    expect(session.blind).toBe(false);
    const markup = renderCard(session);
    expect(markup).toContain("改动前 · A");
    expect(markup).toContain("改动后 · B");
    expect(markup).toContain("A 更好 · 回滚");
    expect(markup).toContain("B 更好 · 保留");
    expect(markup).toContain("听不出差别、想折中或另有想法？直接补充…");
    expect(markup).not.toContain("盲测 · 顺序随机");
  });
});
