import { describe, expect, it } from "vitest";
import { authorityContext, checkoutBlockedByState, isAgentTurnRunning } from "./turnControl";

 describe("turn control", () => {
  it("maps explicit full access into Agent context without implicit action flags", () => {
    expect(authorityContext("full_project_access")).toEqual({ authority_mode: "full_project_access", authority_mode_explicit: true });
  });

  it("shows Stop Turn only for active execution states or an in-flight request", () => {
    for (const status of ["running", "processing", "executing", "cancelling"]) expect(isAgentTurnRunning(status)).toBe(true);
    for (const status of ["stopped", "completed", "failed", "waiting_confirmation"]) expect(isAgentTurnRunning(status)).toBe(false);
    expect(isAgentTurnRunning("", true)).toBe(true);
  });

  it("projects checkout guard from authoritative server state", () => {
    expect(checkoutBlockedByState({ checkout_blocked: true }, null)).toBe(true);
    expect(checkoutBlockedByState(null, { checkout_blocked: false })).toBe(false);
  });
});
