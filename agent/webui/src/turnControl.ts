import type { AuthorityMode, JsonRecord } from "./types";

export const manualConfirmation: AuthorityMode = "manual_confirmation";
export const fullProjectAccess: AuthorityMode = "full_project_access";

export function isAgentTurnRunning(status: unknown, requestInFlight = false): boolean {
  const normalized = String(status ?? "").trim().toLowerCase();
  return requestInFlight || ["running", "processing", "executing", "cancelling"].includes(normalized);
}

export function authorityContext(mode: AuthorityMode): JsonRecord {
  return { authority_mode: mode, authority_mode_explicit: true };
}

export function checkoutBlockedByState(uiState: { checkout_blocked?: boolean } | null | undefined, runtimeStatus: { checkout_blocked?: boolean } | null | undefined): boolean {
  return Boolean(uiState?.checkout_blocked ?? runtimeStatus?.checkout_blocked);
}
