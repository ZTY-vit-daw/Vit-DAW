// SCORE-GUI-FOLLOW-1 (398)：Godot GUI 选轨 ↔ 谱面工作台 双向同步的纯状态机。
//
// 方向一（跟随）：GUI 真实选轨变化 → 谱面跟随。读源=/agent/ui/state 的 resolved
// selected_track 行（ui_context.selected_track_id 仅为读序回退，同 daw/parse.ts
// 先例）；ui_context 注入只用于零栈局部消费测试，不冒充 GUI 装配验证。
// 跟随是**边沿触发**：只对 GUI 选轨的"变化"反应一次（lastSeenTrackID 消费边沿），
// 之后相同 GUI 值不再触发——用户在 dirty 对话框点「继续编辑」后，轮询看到同一个
// GUI 目标也不会反复抢回（卡面"dirty 待切目标防轮询抢回"）；busy 期间边沿延迟
// 不丢弃（busy 结束后仍跟随）。
//
// 方向二（回写）：谱面选轨 commit 后经 /agent/invoke 调 harness 的 select_track
// （RiskDirect，ui_action=focus_track）；回执 track_id 核对 + awaitConfirm 等
// GUI 状态回声确认（连续 FOLLOW_CONFIRM_POLLS 轮未确认=显形 notice）。
//
// 跨工程：工程身份变化即整体重置簿记（边沿基线/awaitConfirm 全清）——旧工程的
// 失效引用不会在新工程触发假确认或假跟随。

import type { AgentUIState, JsonRecord } from "../types";

export type FollowDecision =
  | { kind: "none" }
  | { kind: "confirm" }
  | { kind: "defer" }
  | { kind: "follow"; trackID: string; clipID: string };

export type FollowState = {
  projectKey: string;
  lastSeenTrackID: string;
  awaitConfirm: string;
  awaitProjectKey: string;
  awaitPolls: number;
};

export const FOLLOW_CONFIRM_POLLS = 3;

export function initialFollowState(projectKey = ""): FollowState {
  return { projectKey, lastSeenTrackID: "", awaitConfirm: "", awaitProjectKey: "", awaitPolls: 0 };
}

function trim(value: unknown): string {
  return typeof value === "string" || typeof value === "number" ? String(value).trim() : "";
}

// GUI 真实选轨读序：resolved selected_track 行为准；ui_context.selected_track_id
// 是回退读键（服务器投影把两路都并进 selected_track 行，行缺席时读 context）。
export function guiSelectedTrackID(state: AgentUIState): string {
  const row = (state.selected_track ?? null) as JsonRecord | null;
  const fromRow = row ? trim(row.track_id ?? row.id) : "";
  if (fromRow) return fromRow;
  const context = (state.ui_context ?? {}) as JsonRecord;
  return trim(context.selected_track_id);
}

export function guiSelectedClipID(state: AgentUIState): string {
  const context = (state.ui_context ?? {}) as JsonRecord;
  return trim(context.selected_clip_id);
}

export type FollowInput = {
  projectKey: string;
  guiTrackID: string;
  guiClipID: string;
  choiceProjectKey: string;
  choiceTrackID: string;
  busy: boolean;
};

export function nextFollow(
  prev: FollowState,
  input: FollowInput
): { state: FollowState; decision: FollowDecision } {
  // 跨工程：基线重置（首个新工程轮次只建立基线，不跟随）。
  if (prev.projectKey !== input.projectKey) {
    return {
      state: { projectKey: input.projectKey, lastSeenTrackID: input.guiTrackID, awaitConfirm: "", awaitProjectKey: "", awaitPolls: 0 },
      decision: { kind: "none" }
    };
  }
  if (!input.guiTrackID) {
    // GUI 未披露选轨（或瞬时清空）：保留 lastSeen，防瞬时空值伪造边沿。
    const awaitPolls = prev.awaitConfirm ? prev.awaitPolls + 1 : 0;
    return { state: { ...prev, awaitPolls }, decision: { kind: "none" } };
  }
  if (prev.awaitConfirm && prev.awaitProjectKey === input.projectKey && input.guiTrackID === prev.awaitConfirm) {
    // 核对 GUI 状态：内核/GUI 回声了我们 select 的轨。
    return {
      state: { ...prev, lastSeenTrackID: input.guiTrackID, awaitConfirm: "", awaitProjectKey: "", awaitPolls: 0 },
      decision: { kind: "confirm" }
    };
  }
  const awaitPolls = prev.awaitConfirm ? prev.awaitPolls + 1 : 0;
  if (input.guiTrackID === prev.lastSeenTrackID) {
    return { state: { ...prev, awaitPolls }, decision: { kind: "none" } };
  }
  if (input.busy) {
    // busy：边沿保留不消费（defer），busy 结束后下一轮仍跟随。
    return { state: { ...prev, awaitPolls }, decision: { kind: "defer" } };
  }
  if (input.choiceProjectKey === input.projectKey && input.guiTrackID === input.choiceTrackID) {
    // GUI 落在谱面已持有的轨（我们的 select 生效，或用户在 Godot 点了同一轨）：
    // 确认而非切换。
    return {
      state: { ...prev, lastSeenTrackID: input.guiTrackID, awaitConfirm: "", awaitProjectKey: "", awaitPolls: 0 },
      decision: { kind: "confirm" }
    };
  }
  // 用户驱动的变化：跟随恰好一次（边沿被 lastSeen 消费）。
  return {
    state: { ...prev, lastSeenTrackID: input.guiTrackID, awaitConfirm: "", awaitProjectKey: "", awaitPolls: 0 },
    decision: { kind: "follow", trackID: input.guiTrackID, clipID: input.guiClipID }
  };
}

// markSelect：谱面侧 select_track 提交成功后登记待确认目标（带工程身份防跨工程假确认）。
export function markSelect(state: FollowState, trackID: string, projectKey: string): FollowState {
  return { ...state, awaitConfirm: trackID, awaitProjectKey: projectKey, awaitPolls: 0 };
}
