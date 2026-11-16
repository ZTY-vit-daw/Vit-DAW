import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { emptyAuditionState, reduceAuditionEvents, type AuditionSession } from "./audition";
import { emptyTrajectoryState, reduceTrajectoryEvents } from "./trajectory";
import { mockMultiRoundTrajectoryEvents, mockRollbackTrajectoryEvents } from "./trajectoryMock";
import { AuditionJudgeCard, TrajectoryAuditionPanel, auditionCardOutcome, auditionChangeSummary, auditionRoundBadge, supplementJudgmentPayload, verdictJudgmentPayload } from "./trajectory/TrajectoryAuditionPanel";
import type { AgentEvent } from "./types";

const trajectoryEvent: AgentEvent = { seq: 1, type: "trajectory.turn.started", item_id: "trace-1", payload: { schema_version: "vit.observable_trajectory.v1", turn_id: "turn-1", trace_node_id: "trace-1", node_kind: "turn", status: "running", summary: "started" } };
const auditionEvent: AgentEvent = { seq: 2, type: "audition.ready", item_id: "audition-1", payload: { schema_version: "vit.kernel_audition.v1", session: { session_id: "audition-1", status: "ready", candidates: [{ id: "candidate-a", label: "Before", status: "ready", preview_ref: "a" }, { id: "candidate-b", label: "After", status: "ready", preview_ref: "b" }] } } };

const ready: AgentEvent = { seq: 1, type: "audition.ready", item_id: "audition-ready", conversation_id: "conversation-1", payload: { schema_version: "vit.kernel_audition.v1", session: { session_id: "audition-ready", conversation_id: "conversation-1", turn_id: "turn-1", round_id: "round-1", project_revision: "rev-1", status: "ready", candidates: [{ id: "candidate-a", label: "A", status: "ready", preview_ref: "a" }, { id: "candidate-b", label: "B", status: "ready", preview_ref: "b" }] } } };
const requested: AgentEvent = { seq: 2, type: "trajectory.user_judgment.requested", conversation_id: "conversation-1", payload: { schema_version: "vit.observable_trajectory.v1", turn_id: "turn-1", round_id: "round-1", details: { audition_session_id: "audition-ready", summary: "static_eq 315Hz -5dB (Q 1.8)" } } };

function sessionFromEvents(events: AgentEvent[]): AuditionSession {
  const state = reduceAuditionEvents(emptyAuditionState(), events);
  const session = Object.values(state.sessions)[0];
  if (!session) throw new Error("test fixture must produce one audition session");
  return session;
}

function renderCard(session: AuditionSession, extras?: { trajectory?: ReturnType<typeof emptyTrajectoryState>; superseded?: boolean; busySessionID?: string }) {
  return renderToStaticMarkup(
    <AuditionJudgeCard
      trajectory={extras?.trajectory ?? emptyTrajectoryState()}
      session={session}
      busySessionID={extras?.busySessionID ?? ""}
      superseded={extras?.superseded ?? false}
      onSelect={async () => {}}
      onStop={async () => {}}
      onSubmitJudgment={async () => {}}
    />
  );
}

describe("live trajectory and audition projection", () => {
  it("renders real AgentEvent projections together", () => {
    const markup = renderToStaticMarkup(<TrajectoryAuditionPanel trajectory={reduceTrajectoryEvents(emptyTrajectoryState(), [trajectoryEvent])} audition={reduceAuditionEvents(emptyAuditionState(), [auditionEvent])} busySessionID="" onSelect={async () => {}} onStop={async () => {}} />);
    expect(markup).toContain("自由态实验轨迹");
    expect(markup).toContain("判定 · A/B 试听");
    expect(markup).toContain("Before");
    expect(markup).toContain("After");
  });
});


it("shows the A/B verdict card only after Runtime requests a ready session", () => {
  const beforeRequest = renderCard(sessionFromEvents([ready]));
  const afterRequest = renderCard(sessionFromEvents([ready, requested]));
  expect(beforeRequest).not.toContain("A 更好 · 回滚");
  expect(beforeRequest).not.toContain("听不出差别、想折中或另有想法？直接补充…");
  expect(beforeRequest).not.toContain("待判定");
  expect(beforeRequest).toContain("A/B 快速对比");
  expect(afterRequest).toContain("A 更好 · 回滚");
  expect(afterRequest).toContain("B 更好 · 保留");
  expect(afterRequest).toContain("听不出差别、想折中或另有想法？直接补充…");
  expect(afterRequest).toContain("待判定");
  expect(afterRequest).toContain("这是试验步，可一键回滚");
});

it("does not show verdict buttons while either candidate is not ready", () => {
  const preparing: AgentEvent = { seq: 1, type: "audition.prepare", item_id: "audition-preparing", payload: { schema_version: "vit.kernel_audition.v1", session: { session_id: "audition-preparing", conversation_id: "conversation-1", turn_id: "turn-1", round_id: "round-1", project_revision: "rev-1", status: "preparing", candidates: [{ id: "candidate-a", label: "A", status: "ready", preview_ref: "a" }, { id: "candidate-b", label: "B", status: "preparing" }] } } };
  const preparingRequested: AgentEvent = { seq: 2, type: "trajectory.user_judgment.requested", payload: { schema_version: "vit.observable_trajectory.v1", turn_id: "turn-1", round_id: "round-1", details: { audition_session_id: "audition-preparing" } } };
  const markup = renderCard(sessionFromEvents([preparing, preparingRequested]));
  expect(markup).not.toContain("A 更好 · 回滚");
  expect(markup).not.toContain("发送补充");
});

describe("判定卡映射（GUI-T3：两裁决 + 补充输入 → 既有 audition 协议）", () => {
  it("pickA → preference a、pickB → preference b，直选隐含 heard_difference yes", () => {
    const session = sessionFromEvents([ready, requested]);
    expect(verdictJudgmentPayload(session, "a")).toMatchObject({
      conversation_id: "conversation-1", turn_id: "turn-1", round_id: "round-1",
      audition_session_id: "audition-ready", project_revision: "rev-1",
      heard_difference: "yes", preference: "a", free_text: "", reason_tags: []
    });
    expect(verdictJudgmentPayload(session, "b")).toMatchObject({ heard_difference: "yes", preference: "b", free_text: "" });
  });

  it("补充输入 → free_text（heard/preference 取中性 unsure，卡面选项未采用）", () => {
    const session = sessionFromEvents([ready, requested]);
    expect(supplementJudgmentPayload(session, "  听不出差别，下一轮加码 ")).toMatchObject({
      heard_difference: "unsure", preference: "unsure", free_text: "听不出差别，下一轮加码", reason_tags: []
    });
  });

  it("补充判定记录后卡片沉淀灰条「已收到你的补充 · 卡面选项未采用」", () => {
    const recorded: AgentEvent = { seq: 3, type: "trajectory.user_judgment.recorded", payload: { schema_version: "vit.observable_trajectory.v1", turn_id: "turn-1", round_id: "round-1", details: { audition_session_id: "audition-ready", evidence: { preference: "unsure", free_text: "听不出差别，下一轮加码" } } } };
    const session = sessionFromEvents([ready, requested, recorded]);
    const markup = renderCard(session);
    expect(markup).toContain("card settled");
    expect(markup).toContain("tone-gray");
    expect(markup).toContain("已收到你的补充 · 卡面选项未采用");
    expect(markup).not.toContain("A 更好 · 回滚");
  });

  it("A 裁决沉淀 yellow+回滚文案，B 裁决沉淀 blue+保留文案", () => {
    const recordedA: AgentEvent = { seq: 3, type: "trajectory.user_judgment.recorded", payload: { schema_version: "vit.observable_trajectory.v1", turn_id: "turn-1", round_id: "round-1", details: { audition_session_id: "audition-ready", evidence: { preference: "a" } } } };
    const recordedB: AgentEvent = { seq: 3, type: "trajectory.user_judgment.recorded", payload: { schema_version: "vit.observable_trajectory.v1", turn_id: "turn-1", round_id: "round-1", details: { audition_session_id: "audition-ready", evidence: { preference: "b" } } } };
    const markupA = renderCard(sessionFromEvents([ready, requested, recordedA]));
    expect(markupA).toContain("tone-yellow");
    expect(markupA).toContain("已裁决 · A 更好 → 已回滚到改动前");
    const markupB = renderCard(sessionFromEvents([ready, requested, recordedB]));
    expect(markupB).toContain("tone-blue");
    expect(markupB).toContain("已裁决 · B 更好 · 保留改动后");
  });

  it("轨迹结算节点直接驱动结果条（rolled_back → yellow）", () => {
    // 真实 settled 事件把 outcome 放 details.outcome（runtime.go Settle：details={"outcome":…}）
    const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), [
      ...mockRollbackTrajectoryEvents,
      { seq: 9001, type: "trajectory.settled", item_id: "mock-settlement-real", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "mock-settlement-real", turn_id: "mock-turn", node_kind: "settlement", status: "completed", details: { outcome: "rolled_back", summary: "本轮未保留" } } }
    ]);
    const boundReady: AgentEvent = { seq: 1, type: "audition.ready", item_id: "audition-bound", payload: { schema_version: "vit.kernel_audition.v1", session: { session_id: "audition-bound", conversation_id: "conversation-1", turn_id: "mock-turn", round_id: "mock-round-1", project_revision: "rev-1", status: "ready", candidates: [{ id: "candidate-a", label: "A", status: "ready", preview_ref: "a" }, { id: "candidate-b", label: "B", status: "ready", preview_ref: "b" }] } } };
    const session = sessionFromEvents([boundReady]);
    const markup = renderCard(session, { trajectory });
    expect(markup).toContain("tone-yellow");
    expect(markup).toContain("已回滚到改动前");
    expect(auditionCardOutcome(session, "improved", false)).toMatchObject({ tone: "blue", icon: "check" });
  });

  it("免选路径：底部输入框继续对话 → 卡片沉淀灰条（superseded）", () => {
    const session = sessionFromEvents([ready, requested]);
    const markup = renderCard(session, { superseded: true });
    expect(markup).toContain("card settled");
    expect(markup).toContain("tone-gray");
    expect(markup).toContain("卡面选项未采用 · 你在对话中继续了");
    expect(markup).not.toContain("A 更好 · 回滚");
  });

  it("错误态红条呈现且不隐藏可重试的卡面", () => {
    const failed: AgentEvent = { seq: 3, type: "audition.failed", item_id: "audition-ready", payload: { schema_version: "vit.kernel_audition.v1", session: { session_id: "audition-ready" }, message: "渲染中断" } };
    const session = sessionFromEvents([ready, requested, failed]);
    const markup = renderCard(session);
    expect(markup).toContain("tone-red");
    expect(markup).toContain("渲染中断");
    expect(markup).toContain("A 更好 · 回滚");
  });
});

describe("round 徽标与 mono 摘要（源自 trajectory rounds / user_judgment 事件）", () => {
  it("round 徽标 = 第 N/M 轮，摘要取 user_judgment 请求携带的 summary", () => {
    const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), [
      ...mockMultiRoundTrajectoryEvents,
      { seq: 9001, type: "trajectory.user_judgment.requested", item_id: "mock-judgment-2", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "mock-judgment-2", turn_id: "mock-turn", round_id: "mock-round-2", node_kind: "user_judgment", details: { audition_session_id: "audition-round2", summary: "static_eq 315Hz -8dB (Q 1.8)" } } }
    ]);
    const round2Ready: AgentEvent = { seq: 1, type: "audition.ready", item_id: "audition-round2", payload: { schema_version: "vit.kernel_audition.v1", session: { session_id: "audition-round2", conversation_id: "conversation-1", turn_id: "mock-turn", round_id: "mock-round-2", project_revision: "rev-2", status: "ready", candidates: [{ id: "candidate-a", label: "A", status: "ready", preview_ref: "a" }, { id: "candidate-b", label: "B", status: "ready", preview_ref: "b" }] } } };
    const session = sessionFromEvents([round2Ready]);
    expect(auditionRoundBadge(trajectory, session)).toBe("第 2/2 轮");
    expect(auditionChangeSummary(trajectory, session)).toBe("static_eq 315Hz -8dB (Q 1.8)");
    const markup = renderCard(session, { trajectory });
    expect(markup).toContain("第 2/2 轮");
  });

  it("无法定位 round / summary 时留空，不编造", () => {
    const session = sessionFromEvents([ready]);
    expect(auditionRoundBadge(emptyTrajectoryState(), session)).toBe("");
    expect(auditionChangeSummary(emptyTrajectoryState(), session)).toBe("");
  });
});

// AUDITION-UNSTICK-1（卡 2026-09-14）：stopped 态锁死 + 准备期无显形的渲染钉。
// 用户手测：「AB卡我可以播放A，但是再点B就卡住了无法点击」——stopped 后 A/B
// 控件全部 disabled 且无原因；prepare 期界面空白像死机。修复后：stopped 可点
// （点击经服务端重落座重启播放）；准备期显「正在准备 A/B 试听…」且按钮禁用带
// 原因；在途显「正在切换…」。
describe("AUDITION-UNSTICK-1 A/B 卡控制面", () => {
  const stoppedEvents = [
    ready,
    requested,
    { ...ready, seq: 3, type: "audition.selected", payload: { schema_version: "vit.kernel_audition.v1", session: { ...((ready.payload as { session: Record<string, unknown> }).session), status: "playing", active_candidate_id: "candidate-a" } } },
    { ...ready, seq: 4, type: "audition.stopped", payload: { schema_version: "vit.kernel_audition.v1", session: { ...((ready.payload as { session: Record<string, unknown> }).session), status: "stopped", active_candidate_id: "candidate-a" } } }
  ];

  it("stopped 会话的 A/B 切换与播放按钮不再禁用（点击即重启播放）", () => {
    const markup = renderCard(sessionFromEvents(stoppedEvents));
    expect(markup).toContain('data-status="stopped"');
    const switchButtons = markup.match(/<button[^>]*type="button"[^>]*>\s*[AB]\s*<\/button>/g) || [];
    expect(switchButtons.length).toBeGreaterThanOrEqual(2);
    for (const button of switchButtons) {
      expect(button).not.toContain("disabled");
    }
    const playButtons = markup.match(/class="pbtn"[^>]*/g) || [];
    for (const button of playButtons) {
      expect(button).not.toContain("disabled");
    }
  });

  it("stopped 会话的卡片显式陈述已停止状态", () => {
    const markup = renderCard(sessionFromEvents(stoppedEvents));
    expect(markup).toContain("已停止试听");
  });

  it("准备期显「正在准备 A/B 试听…」且 A/B 按钮禁用带原因", () => {
    const preparingEvent: AgentEvent = {
      ...ready, seq: 1, type: "audition.prepare", status: "preparing",
      payload: {
        schema_version: "vit.kernel_audition.v1",
        session: {
          session_id: "audition-ready", conversation_id: "conversation-1", turn_id: "turn-1", round_id: "round-1",
          status: "preparing",
          candidates: [
            { id: "candidate-a", label: "A", status: "preparing" },
            { id: "candidate-b", label: "B", status: "preparing" }
          ]
        }
      }
    };
    const markup = renderCard(sessionFromEvents([preparingEvent]));
    expect(markup).toContain("正在准备 A/B 试听…");
    const switchButtons = markup.match(/<button[^>]*type="button"[^>]*>\s*[AB]\s*<\/button>/g) || [];
    expect(switchButtons.length).toBeGreaterThanOrEqual(2);
    for (const button of switchButtons) {
      expect(button).toContain("disabled");
      expect(button).toContain("正在准备");
    }
  });

  it("选择在途显 busy（正在切换…）", () => {
    const markup = renderCard(sessionFromEvents([ready, requested]), { busySessionID: "audition-ready" });
    expect(markup).toContain("正在切换…");
  });
});
