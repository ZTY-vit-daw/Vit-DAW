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
