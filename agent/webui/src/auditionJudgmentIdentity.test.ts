import { describe, expect, it } from "vitest";
import { emptyAuditionState, reduceAuditionEvents, type AuditionSession } from "./audition";
import { noDifferenceJudgmentPayload, supplementJudgmentPayload, verdictJudgmentPayload } from "./trajectory/TrajectoryAuditionPanel";
import type { AgentEvent } from "./types";

// AB-JUDGMENT-CARD-1：判定 POST 的 turn_id 契约。真实栈两轮取证
// （M8-FORENSIC-20260930 / M1-RETEST-20261001 第四轮）实证的事件形状：
//   1. 内核 audition.ready 的 session 快照【没有】turn_id/round_id（只有
//      active_project_plane.project_revision 与 candidates）；
//   2. trajectory.user_judgment.requested 的 source_turn_id 是 run 级、
//      payload.turn_id 才是实验 turn 域；
// 服务端 recordFreeStateAuditionJudgment 按 loop.Experiment.ID 校验身份，
// 发 run id 必吃 409 "audition session identity mismatch"——而修复前 webui
// 恰好把挂靠域（run 级）的 turnID 喂给了 POST，且拒绝被静默吞=点击零痕迹。
// 本组钉死：挂靠域（turnID）与判定契约域（experimentTurnID）分账。

const conversationID = "webui_forensic";
const runID = "run_forensic0001";
const experimentTurnID = "turn:free_state_forensic01";
const roundID = "round-1-forensic01";
const sessionID = `audition:${experimentTurnID}:${roundID}`;

/** 内核原始快照形状（M8/R4 证据实录：无 turn_id/round_id/project_revision 顶层键） */
const kernelReady: AgentEvent = {
  seq: 1, type: "audition.ready", item_id: sessionID, conversation_id: conversationID,
  payload: { schema_version: "vit.kernel_audition.v1", command: "audition.prepare", session: {
    session_id: sessionID, conversation_id: conversationID, status: "ready", state_revision: 5,
    active_project_plane: { plane: "active_project", project_ref: "project/Unsaved.vit", project_revision: "39" },
    candidates: [
      { id: "candidate-a", label: "A", status: "ready", preview_ref: `audio-buffer://${sessionID}/candidate-a:2ch@44100Hz` },
      { id: "candidate-b", label: "B", status: "ready", preview_ref: `audio-buffer://${sessionID}/candidate-b:2ch@44100Hz` }
    ]
  } }
};

/** 判定请求事件形状（source_turn_id=run 级；payload.turn_id=实验域——两轮证据实录） */
const judgmentRequested: AgentEvent = {
  seq: 2, type: "trajectory.user_judgment.requested", conversation_id: conversationID, source_turn_id: runID,
  item_id: "trace-forensic01",
  payload: {
    schema_version: "vit.observable_trajectory.v1", turn_id: experimentTurnID, round_id: roundID,
    details: { audition_session_id: sessionID, summary: "A/B audition required" }
  }
};

/** 用户点击后的内核遥测（select/stop 族）：session 依旧无 turn_id——不得抹掉契约域 */
const kernelStopped: AgentEvent = {
  seq: 3, type: "audition.stopped", item_id: sessionID, conversation_id: conversationID,
  payload: { schema_version: "vit.kernel_audition.v1", session: {
    session_id: sessionID, conversation_id: conversationID, status: "stopped", state_revision: 7
  } }
};

function sessionFromEvents(events: AgentEvent[]): AuditionSession {
  const state = reduceAuditionEvents(emptyAuditionState(), events);
  const session = Object.values(state.sessions)[0];
  if (!session) throw new Error("fixture must produce one audition session");
  return session;
}

describe("判定 POST 身份契约（AB-JUDGMENT-CARD-1）", () => {
  it("两轮取证实录形状：turn_id 走实验域，绝不被 run 级挂靠键污染", () => {
    const session = sessionFromEvents([kernelReady, judgmentRequested, kernelStopped]);
    // 挂靠域保持 run 级（B9/WEBUI-MSG-ORDER-2 三级挂靠依赖它，不得回退）
    expect(session.turnID).toBe(runID);
    // 判定契约域 = 实验域（服务端 loop.Experiment.ID 同键校验）
    expect(session.experimentTurnID).toBe(experimentTurnID);
    expect(session.roundID).toBe(roundID);
    // 卡面可判（点击时按钮确实渲染可点——两轮实测用户能点）
    expect(session.judgmentRequested).toBe(true);
    expect(session.judgmentRecorded).toBe(false);
    expect(session.candidates).toHaveLength(2);
    expect(session.candidates.every((candidate) => candidate.status === "ready" && candidate.previewRef)).toBe(true);
    expect(["ready", "playing", "stopped"]).toContain(session.status);
  });

  it("三个判定席位的 POST turn_id 都是实验域（A 直选/听不出差别/补充输入）", () => {
    const session = sessionFromEvents([kernelReady, judgmentRequested, kernelStopped]);
    expect(verdictJudgmentPayload(session, "a").turn_id).toBe(experimentTurnID);
    expect(verdictJudgmentPayload(session, "b").turn_id).toBe(experimentTurnID);
    expect(noDifferenceJudgmentPayload(session).turn_id).toBe(experimentTurnID);
    expect(supplementJudgmentPayload(session, "低频有点糊").turn_id).toBe(experimentTurnID);
  });

  it("回归钉（RED on old behavior）：POST turn_id 不得等于 run 级 source_turn_id", () => {
    const session = sessionFromEvents([kernelReady, judgmentRequested, kernelStopped]);
    const payload = verdictJudgmentPayload(session, "a");
    expect(payload.turn_id).not.toBe(runID);
    expect(payload.turn_id).not.toBe(session.turnID);
    expect(payload.round_id).toBe(roundID);
    expect(payload.audition_session_id).toBe(sessionID);
  });

  it("mix-tick 席位：payload.turn_id=mixTickAuditionTurnID 域时照抄（服务端 record.TurnID 同键）", () => {
    const mixTickTurnID = "mixtick:webui_forensic:tick-1";
    const mixTickRequested: AgentEvent = {
      ...judgmentRequested,
      seq: 4,
      payload: { ...judgmentRequested.payload, turn_id: mixTickTurnID, round_id: "mixtick-round-1" }
    };
    const session = sessionFromEvents([kernelReady, mixTickRequested]);
    expect(session.experimentTurnID).toBe(mixTickTurnID);
    expect(verdictJudgmentPayload(session, "a").turn_id).toBe(mixTickTurnID);
  });

  it("富化快照（含 turn_id 的内核 session）原生域优先；后续原始事件不抹掉契约域", () => {
    const enrichedSession = { ...(kernelReady.payload?.session as Record<string, unknown>), turn_id: experimentTurnID, round_id: roundID, project_revision: "39" };
    const enrichedReady: AgentEvent = {
      ...kernelReady, seq: 5,
      payload: { ...kernelReady.payload, session: enrichedSession }
    };
    const session = sessionFromEvents([enrichedReady, kernelStopped]);
    expect(session.experimentTurnID).toBe(experimentTurnID);
    expect(session.turnID).toBe(experimentTurnID);
  });

  it("无判定请求事件的会话不产判定席（experimentTurnID 空串由服务端显式拒绝，不再发脏 id）", () => {
    const session = sessionFromEvents([kernelReady]);
    expect(session.judgmentRequested).toBe(false);
    expect(session.experimentTurnID).toBe("");
    expect(verdictJudgmentPayload(session, "a").turn_id).toBe("");
  });
});
