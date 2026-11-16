import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { emptyAuditionState, reduceAuditionEvents, auditionSessions } from "./audition";
import { emptyTrajectoryState, reduceTrajectoryEvents, trajectoryTurns } from "./trajectory";
import { reduceAgentEventActivities } from "./messageLifecycle";
import { chatMessageFromAgentEvent } from "./App";
import { reduceRoundSteps, roundActivityBoundKeys } from "./trace/roundSteps";
import { isAuditionFamilyActivity, laneVisibleActivities } from "./trace/turnGroups";
import type { AgentEvent, ChatMessage } from "./types";

// FIX-AUDITION-TRAIL-1（2026-09-29 M1 手测缺陷①）回归：
// 处理期「kernel audition 的动态轨迹在输出内容底下重复出现好几条」。
//
// 取证定性（真栈证据链）：
//  - 内核 audition::Session（VitApp/Source/Service/AuditionPreviewState.h）无 turn_id
//    字段——遥测事件（audition.prepare.started / candidate.ready / ready）的会话快照
//    不带回合域（2026-09-05 fixture seq27/28 实证：src_turn/turn_id/sess.turn_id 全空）；
//  - agent 侧 enrich 事件（同会话首个带 turn 域者）在 mount 完成后才发——遥测先到，
//    以未绑定身份落流底活动线；
//  - 族内每个 eventType 各占一条逻辑消息（audition:{sid}:{type}，audition_events.go
//    emitAuditionEvent），upsertActivity 不折叠 → prepare.started / candidate.ready /
//    ready 同屏多行「已完成：Kernel audition」（处理期重复的可见面）；
//  - AUDITION-LANE-1 裁定该族呈现面=判定卡：修复=lane 按族身份排除
//    （laneVisibleActivities/isAuditionFamilyActivity），判定卡为唯一动态单表面
//    （每会话一卡、随状态更新、判定后定型）。

const fixturesDir = new URL("./trace/__fixtures__/", import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1");

function loadFixture(name: string): AgentEvent[] {
  const raw = JSON.parse(readFileSync(fixturesDir + name, "utf-8")) as { events?: AgentEvent[] };
  return raw.events ?? [];
}

const activityFactory = (event: AgentEvent) => chatMessageFromAgentEvent(event, "default");

/** App 渲染段的 lane 组合口径（MessageStream：轨迹回合 ∪ 会话回合 → 绑定域） */
function composeLane(activities: ChatMessage[], events: AgentEvent[]): ChatMessage[] {
  const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), events);
  const audition = reduceAuditionEvents(emptyAuditionState(), events);
  const roundSteps = reduceRoundSteps({}, events);
  const knownTurnIds = new Set(trajectoryTurns(trajectory).map((turn) => turn.id));
  const sessionBoundTurnIds = new Set(auditionSessions(audition).map((session) => session.turnID).filter(Boolean));
  return laneVisibleActivities(activities, new Set([...knownTurnIds, ...sessionBoundTurnIds]), roundActivityBoundKeys(roundSteps));
}

const laneTexts = (activities: ChatMessage[]): string[] =>
  activities.map((activity) => `${activity.content}`).filter(Boolean);

describe("FIX-AUDITION-TRAIL-1：真栈 fixture 回放（2026-09-05 mtny2v9x，含 audition 遥测族）", () => {
  const events = loadFixture("2026-09-05-webui-mtny2v9x-33events.json");
  const auditionEvents = events.filter((event) => String(event.type).startsWith("audition."));
  const sessionId = "audition:turn:free_state_6464f768689e59d9:round-1-600a22ee194f330b";

  it("取证前提复述：遥测事件（seq27/28/30）确实不带任何 turn 域——重复行的原始形态", () => {
    const sessionTurnOf = (event: AgentEvent): string => {
      const session = (event.payload as Record<string, unknown> | undefined)?.session;
      return String((session as Record<string, unknown> | undefined)?.turn_id ?? "");
    };
    const telemetry = auditionEvents.filter((event) => !sessionTurnOf(event));
    expect(telemetry.map((event) => event.type)).toEqual([
      "audition.prepare.started",
      "audition.candidate.ready",
      // seq30 是内核在 prepare 收尾时重发的 ready 遥测（seq29 是 agent 侧 enrich 版）
      "audition.ready"
    ]);
    // 每个 eventType 一条逻辑消息（upsert 键含 eventType → 处理期同屏多行的根因）
    expect(new Set(telemetry.map((event) => event.logical_message_id)).size).toBe(3);
  });

  it("处理期窗口（seq≤28，仅遥测到达）：活动行存在且未绑定，但 lane 按族身份排除——不再多条", () => {
    const window = events.filter((event) => Number(event.seq) <= 28);
    const activities = reduceAgentEventActivities([], window, activityFactory);
    // 缺陷形态本身仍在活动账里（遥测行、无回合域）——排除是渲染层按身份做的
    const auditionRows = activities.filter(isAuditionFamilyActivity);
    expect(auditionRows.length).toBeGreaterThanOrEqual(2);
    expect(auditionRows.every((row) => !(row.turn_id ?? "").trim())).toBe(true);
    // 修复后：lane 里一条 audition 行都没有
    const lane = composeLane(activities, window);
    expect(lane.filter(isAuditionFamilyActivity)).toHaveLength(0);
    expect(laneTexts(lane).some((text) => text.includes("Kernel audition"))).toBe(false);
  });

  it("判定卡是唯一动态表面：整流回放后该会话收敛为单会话（至多一条），lane 零 audition 行", () => {
    const activities = reduceAgentEventActivities([], events, activityFactory);
    const audition = reduceAuditionEvents(emptyAuditionState(), events);
    const sessions = auditionSessions(audition);
    expect(sessions).toHaveLength(1);
    expect(sessions[0]?.id).toBe(sessionId);
    // 会话回合域由 agent 侧 enrich 事件（seq29 payload.session.turn_id）建立
    expect(sessions[0]?.turnID).toBe("turn:free_state_6464f768689e59d9");
    const lane = composeLane(activities, events);
    expect(lane.filter(isAuditionFamilyActivity)).toHaveLength(0);
  });
});

describe("FIX-AUDITION-TRAIL-1：M1 mix-tick 形态（遥测无域 → agent 侧带合成域后到）", () => {
  const conversationId = "webui_m1_probe";
  const runId = "run_m1_mixtick";
  const sid = "audition:mix_tick:tick_m1";
  const mixTickTurn = "mix_tick_turn:tick_m1";
  const at = (offset: number) => new Date(Date.now() - 30_000 + offset).toISOString();
  // 形态取自 Go 发射源：遥测（AuditionPreviewService.publishStateEvent——无 turn 域）
  // 与 agent 侧 mount（mix_tick_audition.go mountMixTickAudition——session.turn_id 合成域）
  const telemetryShape = (type: string, session: Record<string, unknown>, seq: number): AgentEvent => ({
    seq, type, conversation_id: conversationId, item_id: sid, item_type: "audition",
    status: String(session.status ?? "preparing"), title: "Kernel audition",
    logical_message_id: `audition:${sid}:${type}`, created_at: at(seq * 100),
    payload: { schema_version: "vit.kernel_audition.v1", session }
  } as AgentEvent);
  const candidates = (aStatus: string, bStatus: string) => [
    { id: "candidate-a", label: "A", status: aStatus },
    { id: "candidate-b", label: "B", status: bStatus }
  ];

  it("处理期（mount 未完成）：lane 无 audition 行；判定卡单会话且随状态推进", () => {
    const processing: AgentEvent[] = [
      { seq: 1, type: "turn.started", conversation_id: conversationId, goal_id: runId, run_id: runId, turn_id: runId, source_turn_id: runId, item_type: "turn", status: "running", created_at: at(0) },
      telemetryShape("audition.prepare.started", { session_id: sid, conversation_id: conversationId, status: "preparing", candidates: candidates("preparing", "preparing") }, 2),
      telemetryShape("audition.candidate.ready", { session_id: sid, conversation_id: conversationId, status: "preparing", candidates: candidates("ready", "preparing") }, 3),
      telemetryShape("audition.candidate.ready", { session_id: sid, conversation_id: conversationId, status: "preparing", candidates: candidates("ready", "ready") }, 4)
    ];
    const activities = reduceAgentEventActivities([], processing, activityFactory);
    // 遥测行按族身份排除——处理期输出内容底下不再出现多条「已完成：Kernel audition」
    const lane = composeLane(activities, processing);
    expect(lane.filter(isAuditionFamilyActivity)).toHaveLength(0);
    // 判定卡数据面：单会话、preparing（正在准备 A/B 试听的单表面显形）
    const audition = reduceAuditionEvents(emptyAuditionState(), processing);
    const sessions = auditionSessions(audition);
    expect(sessions).toHaveLength(1);
    expect(sessions[0]?.status).toBe("preparing");
  });

  it("mount 完成（agent 侧带合成域）→ 判定 → 落账：仍单会话、判定已记录（结束定型），lane 恒零 audition 行", () => {
    const full: AgentEvent[] = [
      { seq: 1, type: "turn.started", conversation_id: conversationId, goal_id: runId, run_id: runId, turn_id: runId, source_turn_id: runId, item_type: "turn", status: "running", created_at: at(0) },
      telemetryShape("audition.prepare.started", { session_id: sid, conversation_id: conversationId, status: "preparing", candidates: candidates("preparing", "preparing") }, 2),
      telemetryShape("audition.candidate.ready", { session_id: sid, conversation_id: conversationId, status: "preparing", candidates: candidates("ready", "preparing") }, 3),
      telemetryShape("audition.candidate.ready", { session_id: sid, conversation_id: conversationId, status: "preparing", candidates: candidates("ready", "ready") }, 4),
      { seq: 5, type: "audition.ready", conversation_id: conversationId, item_id: sid, item_type: "audition", status: "ready", title: "Kernel audition", logical_message_id: `audition:${sid}:audition.ready`, created_at: at(500),
        payload: { schema_version: "vit.kernel_audition.v1", command: "audition.prepare", mix_tick: "tick_m1",
          session: { session_id: sid, conversation_id: conversationId, turn_id: mixTickTurn, round_id: "mix_tick_round:tick_m1", status: "ready", candidates: candidates("ready", "ready").map((row) => ({ ...row, preview_ref: row.id })) } } } as AgentEvent,
      { seq: 6, type: "trajectory.user_judgment.requested", conversation_id: conversationId, goal_id: runId, run_id: runId, source_turn_id: runId, item_id: "mix_tick_judgment:" + sid, status: "waiting_for_user", created_at: at(600),
        payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "mix_tick_judgment:" + sid, turn_id: mixTickTurn, round_id: "mix_tick_round:tick_m1", node_kind: "user_judgment", phase: "user_judgment", status: "waiting_for_user", details: { audition_session_id: sid } } } as AgentEvent,
      { seq: 7, type: "trajectory.user_judgment.recorded", conversation_id: conversationId, goal_id: runId, run_id: runId, source_turn_id: runId, item_id: "judgment:" + sid, status: "completed", created_at: at(700),
        payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "judgment:" + sid, turn_id: mixTickTurn, node_kind: "user_judgment", phase: "user_judgment", status: "completed",
          details: { audition_session_id: sid, evidence: { preference: "b", heard_difference: "yes" } } } } as AgentEvent
    ];
    const activities = reduceAgentEventActivities([], full, activityFactory);
    const lane = composeLane(activities, full);
    expect(lane.filter(isAuditionFamilyActivity)).toHaveLength(0);
    // 判定卡定型输入：单会话、ready、判定已记录（卡面沉淀结果条）
    const audition = reduceAuditionEvents(emptyAuditionState(), full);
    const sessions = auditionSessions(audition);
    expect(sessions).toHaveLength(1);
    expect(sessions[0]?.status).toBe("ready");
    expect(sessions[0]?.judgmentRecorded).toBe(true);
    expect(String(sessions[0]?.judgmentEvidence?.preference ?? "")).toBe("b");
  });
});

describe("FIX-AUDITION-TRAIL-1：排除不误伤（无回合归属的非 audition 活动照旧留 lane）", () => {
  it("上传形态活动仍在 lane 可见", () => {
    const upload: ChatMessage = {
      id: "agent_event_goal_upload_1", source_id: "agent_event_goal_upload_1", role: "assistant",
      content: "正在上传素材", createdAt: Date.now(), status: "pending"
    } as ChatMessage;
    expect(isAuditionFamilyActivity(upload)).toBe(false);
    const lane = laneVisibleActivities([upload], new Set());
    expect(lane).toHaveLength(1);
  });

  it("audition 活动行即使带未知回合域也不落 lane（族身份优先于绑定判定）", () => {
    const auditionRow: ChatMessage = {
      id: "agent_event_goal_audition:probe:1", source_id: "agent_event_goal_audition:probe:1", role: "assistant",
      content: "已完成：Kernel audition", createdAt: Date.now(), status: "sent",
      logical_message_id: "audition:audition:probe:audition.ready"
    } as ChatMessage;
    const lane = laneVisibleActivities([auditionRow], new Set());
    expect(lane).toHaveLength(0);
  });
});
