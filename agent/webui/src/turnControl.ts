import type { AuthorityMode, JsonRecord } from "./types";

export const manualConfirmation: AuthorityMode = "manual_confirmation";
export const fullProjectAccess: AuthorityMode = "full_project_access";

export function isAgentTurnRunning(status: unknown, requestInFlight = false): boolean {
  const normalized = String(status ?? "").trim().toLowerCase();
  return requestInFlight || ["running", "processing", "executing", "cancelling"].includes(normalized);
}

// GUI-F3：切片制下首条 HTTP 响应只是第 1 片的边界（goal=waiting_continue +
// 自动续跑排队），链在后台继续跑。goal 停在这些边界态且仍有活 continuation
// （pending/claimed/running/waiting_interaction）时，回合仍在进行——输入锁、
// 状态条与权限锁都要跟随，否则链跑期间 UI 判定"回合已结束"提前解锁。
const chainBoundGoalStatuses = ["running", "processing", "executing", "waiting_continue", "waiting_confirmation", "waiting_clarification"];
const liveContinuationStatuses = ["pending", "claimed", "running", "waiting_interaction"];

export function continuationChainLive(
  continuations: Array<{ status?: unknown }> | null | undefined,
  goalStatus: unknown
): boolean {
  const status = String(goalStatus ?? "").trim().toLowerCase();
  if (!chainBoundGoalStatuses.includes(status)) {
    return false;
  }
  return Boolean(
    (continuations ?? []).some((row) => liveContinuationStatuses.includes(String(row?.status ?? "").trim().toLowerCase()))
  );
}

export function authorityContext(mode: AuthorityMode): JsonRecord {
  return { authority_mode: mode, authority_mode_explicit: true };
}

export function checkoutBlockedByState(uiState: { checkout_blocked?: boolean } | null | undefined, runtimeStatus: { checkout_blocked?: boolean } | null | undefined): boolean {
  return Boolean(uiState?.checkout_blocked ?? runtimeStatus?.checkout_blocked);
}
