import { describe, expect, it } from "vitest";
import { auditionCanSelect, auditionPreparing, auditionSettlementOutcome, emptyAuditionState, reduceAuditionEvents } from "./audition";
import { emptyTrajectoryState, reduceTrajectoryEvents } from "./trajectory";
import type { AgentEvent } from "./types";

function event(type: string, status: string, candidates: Array<Record<string, unknown>>, seq = 1): AgentEvent {
  return { seq, type, item_id: "audition-1", status, payload: { schema_version: "vit.kernel_audition.v1", session: { session_id: "audition-1", status, candidates } } };
}

describe("audition reducer", () => {
  it("rejects selection before both previews are ready", () => {
    const state = reduceAuditionEvents(emptyAuditionState(), [event("audition.prepare", "preparing", [
      { id: "candidate-a", label: "Before", status: "ready", preview_ref: "preview:a" },
      { id: "candidate-b", label: "After", status: "preparing" }
    ])]);
    expect(auditionCanSelect(state.sessions["audition-1"], state.sessions["audition-1"].candidates[0])).toBe(false);
  });

  it("updates A/B state from audition.ready", () => {
    const state = reduceAuditionEvents(emptyAuditionState(), [event("audition.ready", "ready", [
      { id: "candidate-a", label: "Before", status: "ready", preview_ref: "preview:a" },
      { id: "candidate-b", label: "After", status: "ready", preview_ref: "preview:b" }
    ])]);
    expect(state.sessions["audition-1"].status).toBe("ready");
    expect(auditionCanSelect(state.sessions["audition-1"], state.sessions["audition-1"].candidates[1])).toBe(true);
  });
});


it("opens judgment only after a requested ready session and keeps A/B selection separate", () => {
  const ready = event("audition.ready", "ready", [
    { id: "candidate-a", label: "A", status: "ready", preview_ref: "preview:a" },
    { id: "candidate-b", label: "B", status: "ready", preview_ref: "preview:b" }
  ], 1);
  ready.payload!.session = { ...(ready.payload!.session as Record<string, unknown>), conversation_id: "conversation-1", turn_id: "turn-1", round_id: "round-1" };
  const requested: AgentEvent = { seq: 2, type: "trajectory.user_judgment.requested", conversation_id: "conversation-1", payload: { schema_version: "vit.observable_trajectory.v1", turn_id: "turn-1", round_id: "round-1", details: { audition_session_id: "audition-1" } } };
  const selected: AgentEvent = { seq: 3, type: "audition.selected", item_id: "audition-1", payload: { schema_version: "vit.kernel_audition.v1", session: { session_id: "audition-1", status: "ready", active_candidate_id: "candidate-b" } } };
  const state = reduceAuditionEvents(emptyAuditionState(), [ready, requested, selected]);
  expect(state.sessions["audition-1"].judgmentRequested).toBe(true);
  expect(state.sessions["audition-1"].judgmentRecorded).toBe(false);
  expect(state.sessions["audition-1"].activeCandidateId).toBe("candidate-b");
});

function settlementFixture(outcome: string): { audition: ReturnType<typeof reduceAuditionEvents>; trajectory: ReturnType<typeof reduceTrajectoryEvents> } {
  const ready = event("audition.ready", "ready", [
    { id: "candidate-a", label: "A", status: "ready", preview_ref: "preview:a" },
    { id: "candidate-b", label: "B", status: "ready", preview_ref: "preview:b" }
  ], 1);
  ready.payload!.session = { ...(ready.payload!.session as Record<string, unknown>), conversation_id: "conversation-1", turn_id: "turn-1", round_id: "round-1" };
  const recorded: AgentEvent = { seq: 2, type: "trajectory.user_judgment.recorded", conversation_id: "conversation-1", payload: { schema_version: "vit.observable_trajectory.v1", turn_id: "turn-1", round_id: "round-1", details: { audition_session_id: "audition-1", evidence: { id: "evidence-1", preference: "b" } } } };
  const audition = reduceAuditionEvents(emptyAuditionState(), [ready, recorded]);
  // The settlement node carries the raw outcome in details; the turn-level
  // evaluation outcome maps needs_user_judgment back to human_audition_ready.
  const settled: AgentEvent = { seq: 3, type: "trajectory.settled", conversation_id: "conversation-1", item_id: "trace-settle-1", payload: { schema_version: "vit.observable_trajectory.v1", turn_id: "turn-1", round_id: "round-1", trace_node_id: "trace-settle-1", node_kind: "settlement", summary: "experiment settled", details: { outcome } } };
  const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), [settled]);
  return { audition, trajectory };
}

describe("audition settlement outcome", () => {
  it("derives the D1 settlement from the trajectory settlement node details", () => {
    for (const outcome of ["improved", "rolled_back", "needs_user_judgment"]) {
      const { audition, trajectory } = settlementFixture(outcome);
      expect(auditionSettlementOutcome(trajectory, audition.sessions["audition-1"])).toBe(outcome);
    }
  });

  it("reports no settlement before the settlement node exists", () => {
    const ready = event("audition.ready", "ready", [
      { id: "candidate-a", label: "A", status: "ready", preview_ref: "preview:a" },
      { id: "candidate-b", label: "B", status: "ready", preview_ref: "preview:b" }
    ], 1);
    ready.payload!.session = { ...(ready.payload!.session as Record<string, unknown>), conversation_id: "conversation-1", turn_id: "turn-1", round_id: "round-1" };
    const audition = reduceAuditionEvents(emptyAuditionState(), [ready]);
    expect(auditionSettlementOutcome(emptyTrajectoryState(), audition.sessions["audition-1"])).toBe("");
  });
});

// AUDITION-UNSTICK-1（卡 2026-09-14）：stopped 态锁死选择的单元钉。
// 用户事件流铁证（seq20-30）：点 A 播放 → audition.stopped ×2 → 会话 stopped，
// 旧 auditionCanSelect 只认 ready|playing——A/B 全部不可选且无反馈=「点 B 卡住」。
// 修复后 stopped 放行（点击即经服务端同参 audition.prepare 重落座回 ready 再
// select 重启播放）；stale/failed 仍拒；候选 preparing 仍拒。
describe("AUDITION-UNSTICK-1 选择门", () => {
  const readyCandidates = [
    { id: "candidate-a", label: "A", status: "ready", preview_ref: "preview:a" },
    { id: "candidate-b", label: "B", status: "ready", preview_ref: "preview:b" }
  ];

  it("stopped 会话可选择候选（点击即重启播放）", () => {
    const state = reduceAuditionEvents(emptyAuditionState(), [
      event("audition.ready", "ready", readyCandidates, 1),
      event("audition.stopped", "stopped", readyCandidates, 2)
    ]);
    const session = state.sessions["audition-1"];
    expect(session.status).toBe("stopped");
    for (const candidate of session.candidates) {
      expect(auditionCanSelect(session, candidate)).toBe(true);
    }
  });

  it("stale / failed 会话仍不可选（终态不放行）", () => {
    for (const terminal of ["stale", "failed"]) {
      const state = reduceAuditionEvents(emptyAuditionState(), [
        event("audition.ready", "ready", readyCandidates, 1),
        event("audition." + terminal, terminal, readyCandidates, 2)
      ]);
      const session = state.sessions["audition-1"];
      expect(session.status).toBe(terminal);
      expect(auditionCanSelect(session, session.candidates[0])).toBe(false);
    }
  });

  it("候选 preparing 时不可选（准备期选择门保持）", () => {
    const state = reduceAuditionEvents(emptyAuditionState(), [
      event("audition.prepare", "preparing", [
        { id: "candidate-a", label: "A", status: "ready", preview_ref: "preview:a" },
        { id: "candidate-b", label: "B", status: "preparing" }
      ], 1)
    ]);
    const session = state.sessions["audition-1"];
    expect(auditionCanSelect(session, session.candidates[0])).toBe(false);
    expect(auditionCanSelect(session, session.candidates[1])).toBe(false);
  });
});

// 准备期显形判定（目标 3 的数据面）：会话或任一候选仍在 preparing 即为准备期。
const unstuckReadyCandidates = [
  { id: "candidate-a", label: "A", status: "ready", preview_ref: "preview:a" },
  { id: "candidate-b", label: "B", status: "ready", preview_ref: "preview:b" }
];

describe("AUDITION-UNSTICK-1 准备期判定", () => {
  it("会话 preparing 或任一候选 preparing 都算准备期", () => {
    const preparing = reduceAuditionEvents(emptyAuditionState(), [
      event("audition.prepare", "preparing", [
        { id: "candidate-a", label: "A", status: "preparing" },
        { id: "candidate-b", label: "B", status: "preparing" }
      ], 1)
    ]);
    expect(auditionPreparing(preparing.sessions["audition-1"])).toBe(true);

    const halfReady = reduceAuditionEvents(emptyAuditionState(), [
      event("audition.prepare", "preparing", [
        { id: "candidate-a", label: "A", status: "ready", preview_ref: "preview:a" },
        { id: "candidate-b", label: "B", status: "preparing" }
      ], 1)
    ]);
    expect(auditionPreparing(halfReady.sessions["audition-1"])).toBe(true);
  });

  it("双候选 ready 的会话不算准备期", () => {
    const state = reduceAuditionEvents(emptyAuditionState(), [event("audition.ready", "ready", unstuckReadyCandidates, 1)]);
    expect(auditionPreparing(state.sessions["audition-1"])).toBe(false);
  });
});
