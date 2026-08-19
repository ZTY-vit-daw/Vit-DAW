import { describe, expect, it } from "vitest";
import { auditionCanSelect, emptyAuditionState, reduceAuditionEvents } from "./audition";
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
