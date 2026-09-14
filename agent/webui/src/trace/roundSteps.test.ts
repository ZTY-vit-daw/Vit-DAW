import { describe, expect, it } from "vitest";
import type { AgentEvent } from "../types";
import { emptyRoundStepMap, mergeTraceAndRoundSteps, reduceRoundSteps, roundActivityBoundKeys, roundStartSeq, roundStepAsTrajectoryNode, roundStepHasEvidence, roundStepTurnIds, roundTurnShell, roundTurnStatus, shouldRenderRoundContainer, unmappedStepIdentifiers, ROUND_STEP_WINDOW } from "./roundSteps";
import { stepLabel } from "./stepLabels";
import { reduceTurnEventMeta } from "./turnEventMeta";
import type { TrajectoryNode } from "../trajectory";

// TRAJ-IMPL-2（设计 docs/TRAJECTORY_PRESENTATION_REDESIGN_V1.md §2.1 + §7 裁定 A，
// 2026-09-14 定稿）：回合步归约器 + 人话映射 + 渲染谓词放宽的**纯函数**钉。
//
// 事件形态全部取自真栈取证（两份 fixture 的 item 事件原样）：
//   - 同一次 run 的多次工具调用**共用 item_id / logical_message_id**
//     （agent_item:<run>:tool_step_1），所以步身份只能由事件 seq 分开，
//     足迹键（与 turnEventMeta 同源）只用于起止配对与「本回合发生过」的判据；
//   - item.started 带 payload.tool，item.completed 带 payload.command_name
//     （ccb.observation_catalog 对 ccb_observation_catalog：同一动作两种写法）。

const RUN = "run_traj_impl_2";
const T0 = Date.parse("2026-09-14T10:00:00.000Z");
const at = (ms: number) => new Date(ms).toISOString();

function started(seq: number, atMs: number, tool: string, itemId = "tool_step_1"): AgentEvent {
  return {
    seq,
    type: "item.started",
    source_turn_id: RUN,
    item_id: itemId,
    item_type: "daw_action",
    status: "running",
    created_at: at(atMs),
    payload: { tool, command_raw: { tool } }
  } as AgentEvent;
}

function completed(seq: number, atMs: number, commandName: string, status = "completed", itemId = "tool_step_1"): AgentEvent {
  return {
    seq,
    type: "item.completed",
    source_turn_id: RUN,
    item_id: itemId,
    status,
    created_at: at(atMs),
    payload: { command_name: commandName }
  } as AgentEvent;
}

function turnStarted(seq: number, atMs: number): AgentEvent {
  return { seq, type: "turn.started", source_turn_id: RUN, status: "running", created_at: at(atMs) } as AgentEvent;
}

describe("钉1 回合键与起止折叠：与 trajectory/turnEventMeta 同键，一次调用一步", () => {
  it("source_turn_id 优先（turn_id/run_id 跨域也不改回合键）；item 起止折叠成一步", () => {
    const events: AgentEvent[] = [
      turnStarted(1, T0),
      { ...started(2, T0 + 1_000, "ccb.observation_catalog"), run_id: "run_other", goal_id: "goal_x", turn_id: "turn_chat_domain" },
      { ...completed(3, T0 + 2_000, "ccb_observation_catalog"), turn_id: "turn_chat_domain", logical_message_id: `agent_item:${RUN}:tool_step_1` }
    ];
    const rounds = reduceRoundSteps(emptyRoundStepMap(), events);
    expect(Object.keys(rounds)).toEqual([RUN]);
    const round = rounds[RUN];
    expect(round.steps).toHaveLength(1);
    expect(round.steps[0].status).toBe("completed");
    expect(round.steps[0].createdAt).toBe(T0 + 1_000);
    expect(round.steps[0].closeSeq).toBe(3);
    expect(round.startedAt).toBe(T0);
  });

  it("同键钉：meta 的 item 活动足迹与步账落在同一回合键上（B9「一轮一块」不破）", () => {
    const events: AgentEvent[] = [turnStarted(1, T0), started(2, T0 + 1_000, "track.volume"), completed(3, T0 + 1_500, "track_volume")];
    const rounds = reduceRoundSteps(emptyRoundStepMap(), events);
    const meta = reduceTurnEventMeta({}, events);
    expect(Object.keys(rounds)).toEqual([RUN]);
    expect(Object.keys(meta)).toEqual([RUN]);
    expect(meta[RUN].itemActivityCount).toBe(1);
  });
});

describe("钉2 同一足迹的多次调用 = 多步（真栈形态：item 标识符按 run 复用）", () => {
  it("三次调用共用 item_id/logical_message_id 仍是三步，状态逐条（含失败）", () => {
    const events: AgentEvent[] = [
      turnStarted(1, T0),
      started(2, T0 + 1_000, "ccb.observation_catalog"),
      completed(3, T0 + 2_000, "ccb_observation_catalog"),
      started(4, T0 + 3_000, "plugin.search"),
      completed(5, T0 + 4_000, "plugin_search"),
      started(6, T0 + 5_000, "plugin.load"),
      completed(7, T0 + 6_000, "plugin_load", "failed")
    ];
    const round = reduceRoundSteps(emptyRoundStepMap(), events)[RUN];
    expect(round.steps).toHaveLength(3);
    expect(round.totalStepCount).toBe(3);
    expect(round.steps.map((step) => step.status)).toEqual(["completed", "completed", "failed"]);
    expect(round.steps.map((step) => step.title)).toEqual(["已完成 可用观察视图清单", "已完成 插件搜索", "执行失败 插件装载"]);
    // 步序按起始时刻（完成事件不把步往后挪）
    expect(round.steps.map((step) => step.createdAt)).toEqual([T0 + 1_000, T0 + 3_000, T0 + 5_000]);
  });
});

describe("钉3 幂等：轮询重复投递不虚增步数", () => {
  it("同一段事件重放两次，步账逐字段相同", () => {
    const events: AgentEvent[] = [
      turnStarted(1, T0),
      started(2, T0 + 1_000, "plugin.search"),
      completed(3, T0 + 2_000, "plugin_search")
    ];
    const once = reduceRoundSteps(emptyRoundStepMap(), events);
    const twice = reduceRoundSteps(once, events);
    expect(twice).toEqual(once);
    expect(twice[RUN].steps).toHaveLength(1);
    expect(twice[RUN].totalStepCount).toBe(1);
  });

  it("只见完成事件（轮询漏了起始）也独立成一步，不假装它没发生", () => {
    const round = reduceRoundSteps(emptyRoundStepMap(), [completed(9, T0 + 500, "plugin_search")])[RUN];
    expect(round.steps).toHaveLength(1);
    expect(round.steps[0].status).toBe("completed");
  });
});

describe("钉4 审批步与回合失败步", () => {
  it("approval.requested → pending 审批步（人话「等待你的权限确认」）；turn.failed → failed 回合步", () => {
    const events: AgentEvent[] = [
      { seq: 1, type: "approval.requested", source_turn_id: RUN, item_id: "approval_1", status: "pending", created_at: at(T0 + 1_000), payload: { tool: "approval.requested" } },
      { seq: 2, type: "turn.failed", source_turn_id: RUN, status: "failed", created_at: at(T0 + 2_000) }
    ] as AgentEvent[];
    const round = reduceRoundSteps(emptyRoundStepMap(), events)[RUN];
    expect(round.steps.map((step) => step.kind)).toEqual(["approval", "turn"]);
    expect(round.steps.map((step) => step.status)).toEqual(["pending", "failed"]);
    expect(round.steps[0].title).toBe("等待你的权限确认");
    expect(round.failed).toBe(true);
    expect(roundTurnStatus(round)).toBe("failed");
  });
});

describe("钉5 滚动窗口：留最近 50 步 + 总计数（设计 §6.2 步量爆炸对策）", () => {
  function manyCalls(count: number): AgentEvent[] {
    const events: AgentEvent[] = [turnStarted(1, T0)];
    for (let index = 0; index < count; index += 1) {
      const base = 10 + index * 2;
      events.push(started(base, T0 + (index + 1) * 1_000, "track.volume"));
      events.push(completed(base + 1, T0 + (index + 1) * 1_000 + 200, "track_volume"));
    }
    return events;
  }

  it("60 次调用 → 窗口 50 步、总计数 60、丢弃 10；保留的是最近的 50 步", () => {
    const round = reduceRoundSteps(emptyRoundStepMap(), manyCalls(60))[RUN];
    expect(ROUND_STEP_WINDOW).toBe(50);
    expect(round.steps).toHaveLength(50);
    expect(round.totalStepCount).toBe(60);
    expect(round.droppedStepCount).toBe(10);
    // 第 11 次调用（seq 30）是窗口里的第一步——窗口丢的是更早的
    expect(round.steps[0].seq).toBe(30);
    expect(roundStepHasEvidence(round)).toBe(true);
  });

  it("未超窗口时总计数 = 步数、丢弃 0", () => {
    const round = reduceRoundSteps(emptyRoundStepMap(), manyCalls(3))[RUN];
    expect(round.steps).toHaveLength(3);
    expect(round.totalStepCount).toBe(3);
    expect(round.droppedStepCount).toBe(0);
  });
});

describe("钉6 人话映射（§7 裁定 A）：命中转中文，未命中原样", () => {
  it("ccb 观察族命中「已完成 频率关系观察」（裁定 A 的示例逐字）", () => {
    expect(stepLabel({ identifier: "ccb.observation_request", status: "completed" }).title).toBe("已完成 频率关系观察");
    expect(stepLabel({ identifier: "ccb.observation.request", status: "running" }).title).toBe("正在执行 频率关系观察");
    // 下划线写法与点写法归一化后同一条（真栈 started/completed 两种写法）
    expect(stepLabel({ identifier: "ccb_observation_catalog", status: "completed" }).mapped).toBe(true);
  });

  it("常见族逐族命中（mix_tick / trajectory / audition / approval），审批步读作整句", () => {
    expect(stepLabel({ identifier: "mix_tick.pending", status: "pending" }).title).toBe("混音调整等待你确认");
    expect(stepLabel({ identifier: "trajectory.observation.recorded", status: "completed" }).title).toBe("已完成 轨迹观察记录");
    expect(stepLabel({ identifier: "audition.prepare", status: "running" }).title).toBe("正在执行 A/B 试听准备");
    expect(stepLabel({ identifier: "approval.requested", status: "pending" }).title).toBe("等待你的权限确认");
  });

  it("未命中的标识符原样显示（不吞不改写）并进待补清单", () => {
    const label = stepLabel({ identifier: "weird.custom_tool", status: "completed" });
    expect(label.mapped).toBe(false);
    expect(label.title).toBe("weird.custom_tool");
    const round = reduceRoundSteps(emptyRoundStepMap(), [
      started(1, T0, "weird.custom_tool"),
      completed(2, T0 + 500, "weird.custom_tool")
    ])[RUN];
    expect(round.steps[0].title).toBe("weird.custom_tool");
    expect(unmappedStepIdentifiers({ [RUN]: round })).toEqual(["weird.custom_tool"]);
  });
});

describe("钉7 渲染谓词放宽 + M12 证据链重述（settle_slice 逐字仍隐藏）", () => {
  it("有 item 步 → 出容器（活动足迹证据始终优先，即便带 settle_slice 标记）", () => {
    const round = reduceRoundSteps(emptyRoundStepMap(), [started(1, T0, "plugin.search")])[RUN];
    expect(shouldRenderRoundContainer({ round, meta: { turnKind: "settle_slice", itemActivityCount: 0, itemActivityKeys: [], startedAt: T0, endedAt: T0 + 128 } })).toBe(true);
  });

  it("无步 + settle_slice 标记 → 隐藏（M12 条件②，标记是服务端权威）", () => {
    expect(shouldRenderRoundContainer({
      round: undefined,
      meta: { turnKind: "settle_slice", itemActivityCount: 0, itemActivityKeys: [], startedAt: T0, endedAt: T0 + 128 }
    })).toBe(false);
  });

  it("无步无标记：短寿命（128ms）隐藏、≥2s 渲染（M12 条件③阈值逐字沿用）", () => {
    const short = { turnKind: "", itemActivityCount: 0, itemActivityKeys: [], startedAt: T0, endedAt: T0 + 128 };
    const long = { turnKind: "", itemActivityCount: 0, itemActivityKeys: [], startedAt: T0, endedAt: T0 + 2_000 };
    expect(shouldRenderRoundContainer({ round: undefined, meta: short })).toBe(false);
    expect(shouldRenderRoundContainer({ round: undefined, meta: long })).toBe(true);
  });

  it("时间戳缺失（证据不足）不隐藏；meta 缺失同样保守渲染", () => {
    expect(shouldRenderRoundContainer({ round: undefined, meta: { turnKind: "", itemActivityCount: 0, itemActivityKeys: [] } })).toBe(true);
    expect(shouldRenderRoundContainer({ round: undefined, meta: undefined })).toBe(true);
  });
});

describe("钉8 回合壳状态（收口双轨的显示侧口径，设计 §4）", () => {
  function roundWith(events: AgentEvent[]) {
    return reduceRoundSteps(emptyRoundStepMap(), [started(1, T0, "plugin.search"), ...events])[RUN];
  }

  it("未收口 / 切片边界（waiting_continue）→ running（live，容器接管乐观占位）", () => {
    expect(roundTurnStatus(roundWith([]))).toBe("running");
    expect(roundTurnStatus(roundWith([
      { seq: 2, type: "turn.completed", source_turn_id: RUN, status: "waiting_continue", created_at: at(T0 + 5_000) }
    ] as AgentEvent[]))).toBe("running");
  });

  it("completed → completed；turn.failed → failed；turn.stopped → stopped", () => {
    expect(roundTurnStatus(roundWith([
      { seq: 2, type: "turn.completed", source_turn_id: RUN, status: "completed", created_at: at(T0 + 5_000) }
    ] as AgentEvent[]))).toBe("completed");
    expect(roundTurnStatus(roundWith([
      { seq: 2, type: "turn.failed", source_turn_id: RUN, status: "failed", created_at: at(T0 + 5_000) }
    ] as AgentEvent[]))).toBe("failed");
    expect(roundTurnStatus(roundWith([
      { seq: 2, type: "turn.stopped", source_turn_id: RUN, status: "stopped", created_at: at(T0 + 5_000) }
    ] as AgentEvent[]))).toBe("stopped");
  });

  it("壳节点身份与状态齐备（TraceBlock 只消费身份/状态；nodeIds 空是事实）", () => {
    const round = roundWith([]);
    const shell = roundTurnShell(round);
    expect(shell?.id).toBe(RUN);
    expect(shell?.status).toBe("running");
    expect(shell?.nodeIds).toEqual([]);
    expect(roundTurnShell(undefined)).toBeNull();
  });
});

describe("钉9 步序混排与活动线去重钥匙", () => {
  it("item 步与轨迹步按 createdAt 混排（同刻按 seq 稳定）", () => {
    const traceNode = (id: string, createdAt: number, seq: number): TrajectoryNode => ({
      id, turnId: RUN, roundId: "", parentId: "", kind: "observation", phase: "", status: "completed",
      title: "轨迹观察", summary: "", createdAt, seq, materiality: "", targetResponse: "", outcome: "",
      nextDecision: "", evidenceRefs: [], actionRefs: [], projectRevision: "", checkpointRef: "", branchRef: "",
      worktreeRef: "", details: {}, eventType: "trajectory.observation.recorded"
    });
    const round = reduceRoundSteps(emptyRoundStepMap(), [
      started(1, T0 + 2_000, "plugin.search"),
      completed(2, T0 + 2_500, "plugin_search")
    ])[RUN];
    const merged = mergeTraceAndRoundSteps([traceNode("obs-1", T0 + 1_000, 10), traceNode("obs-2", T0 + 3_000, 20)], round.steps);
    expect(merged.map((node) => node.id)).toEqual(["obs-1", `round-step:${round.steps[0].key}`, "obs-2"]);
    // item 步行走形态：kind=activity（行外壳复用 TraceStep 的既有分支）
    expect(roundStepAsTrajectoryNode(round.steps[0]).kind).toBe("activity");
  });

  it("无 item 步时原样返回轨迹步数组（既有调用面零回退）", () => {
    const nodes: TrajectoryNode[] = [];
    expect(mergeTraceAndRoundSteps(nodes, [])).toBe(nodes);
  });

  it("活动线去重钥匙覆盖 step.key / 足迹 / 活动 id / 逻辑消息 id", () => {
    const round = reduceRoundSteps(emptyRoundStepMap(), [
      { ...started(1, T0, "plugin.search"), goal_id: "goal_1", logical_message_id: `agent_item:${RUN}:tool_step_1` }
    ])[RUN];
    const keys = roundActivityBoundKeys({ [RUN]: round });
    expect(keys.has(round.steps[0].key)).toBe(true);
    expect(keys.has("tool_step_1")).toBe(true);
    expect(keys.has("agent_event_goal_1_tool_step_1")).toBe(true);
    expect(keys.has(`agent_item:${RUN}:tool_step_1`)).toBe(true);
  });

  it("渲染候选排序：按回合起始 seq（与轨迹节点 seq 同域）", () => {
    const later = reduceRoundSteps(emptyRoundStepMap(), [
      { ...started(90, T0 + 9_000, "plugin.search"), source_turn_id: "run_later" } as AgentEvent
    ]);
    const both = reduceRoundSteps(later, [started(5, T0 + 1_000, "plugin.search")]);
    expect(roundStepTurnIds(both)).toEqual([RUN, "run_later"]);
    expect(roundStartSeq(both[RUN])).toBe(5);
  });
});
