import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { buildMessageStreamRenderPlan } from "./renderPlan";
import { emptyTrajectoryState, reduceTrajectoryEvents, type TrajectoryState } from "../trajectory";
import { reduceTurnEventMeta, type TurnEventMeta } from "./turnEventMeta";
import type { AgentEvent, ChatMessage } from "../types";

function chat(partial: Partial<ChatMessage> & Pick<ChatMessage, "id" | "role" | "content">): ChatMessage {
  return { createdAt: 0, ...partial } as ChatMessage;
}

function chainResult(id: string, content: string): ChatMessage {
  return chat({ id, role: "assistant", content, source_id: id });
}

function meta(...turns: Array<{ id: string; itemActivityCount?: number }>): Record<string, TurnEventMeta> {
  const out: Record<string, TurnEventMeta> = {};
  for (const turn of turns) {
    out[turn.id] = {
      turnKind: "",
      itemActivityCount: turn.itemActivityCount ?? 0,
      itemActivityKeys: [],
      startedAt: 0,
      endedAt: 1000
    };
  }
  return out;
}

function stateWithTurn(id: string, events: AgentEvent[]): TrajectoryState {
  return reduceTrajectoryEvents(emptyTrajectoryState(), events);
}

describe("buildMessageStreamRenderPlan（GUI-1/G2：固化渲染顺序）", () => {
  it("no_candidate 流：用户消息 → 回合轨迹块 → 中间汇报 → 终局回复（轨迹完成后保留可回看）", () => {
    const messages = [
      chat({ id: "u1", role: "user", content: "检查一下当前工程有什么问题", turn_id: "run_1" }),
      chat({ id: "a1", role: "assistant", content: "我还在继续处理这个任务，完成后再向你汇报。", turn_id: "run_1" })
    ];
    const trajectory = stateWithTurn("run_1", [
      { seq: 1, type: "trajectory.turn.started", run_id: "run_1", turn_id: "run_1", item_id: "turn:run_1", payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "turn", turn_id: "run_1", trace_node_id: "turn:run_1" } },
      { seq: 9, type: "trajectory.turn.completed", run_id: "run_1", turn_id: "run_1", item_id: "turn:run_1", payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "turn", turn_id: "run_1", trace_node_id: "turn:run_1", status: "completed" } }
    ] as AgentEvent[]);
    const chain = chainResult("agent_event_goal_chain_result", "没有发现可信改善候选。");
    const plan = buildMessageStreamRenderPlan({
      messages: [...messages, chain],
      trajectory,
      turnEventMeta: meta({ id: "run_1", itemActivityCount: 5 })
    });
    const shape = plan.entries.map((entry) => entry.kind === "trace" ? `trace:${entry.turnId}` : "messages");
    expect(shape).toEqual(["messages", "trace:run_1", "messages"]);
    expect(plan.entries[0].kind === "messages" && plan.entries[0].messages[0]?.id).toBe("u1");
    expect(plan.entries[2].kind === "messages" && plan.entries[2].messages[0]?.id).toBe("a1");
    // 终局回复在孤儿块之后、且不入组内
    expect(plan.chainResultMessages).toHaveLength(1);
    expect(plan.entries.filter((entry) => entry.kind === "messages").at(-1)?.kind === "messages" &&
      plan.entries.filter((entry) => entry.kind === "messages").at(-1)!.messages[0]?.id).toBe("a1");
  });

  it("实验流：用户消息 → 中间汇报 → 实验轨迹块（孤儿）→ 终局回复", () => {
    const messages = [
      chat({ id: "u1", role: "user", content: "把军鼓往上提", turn_id: "run_1" }),
      chat({ id: "a1", role: "assistant", content: "正在实验。", turn_id: "run_1" }),
      chainResult("agent_event_goal_chain_result", "已提升 1.5dB。")
    ];
    const trajectory = stateWithTurn("turn:fs_1", [
      { seq: 2, type: "trajectory.intent.framed", run_id: "run_1", turn_id: "turn:fs_1", item_id: "n1", payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "intent", turn_id: "turn:fs_1", trace_node_id: "n1", status: "completed" } }
    ] as AgentEvent[]);
    const plan = buildMessageStreamRenderPlan({ messages, trajectory, turnEventMeta: meta({ id: "run_1", itemActivityCount: 2 }) });
    const shape = plan.entries.map((entry) => entry.kind === "trace" ? `trace:${entry.turnId}` : "messages");
    // 组内：用户消息条目 + 中间汇报条目（run_1 无轨迹 turn 记录不入块）；实验块为孤儿条目
    expect(shape).toEqual(["messages", "messages", "trace:turn:fs_1"]);
    expect(plan.orphanTurnIds).toEqual(["turn:fs_1"]);
    expect(plan.chainResultMessages.map((message) => message.id)).toEqual(["agent_event_goal_chain_result"]);
  });

  it("settle_slice 切片回合（0 步、无活动、带标记）不产生轨迹条目，终局回复保留", () => {
    const messages = [
      chat({ id: "u1", role: "user", content: "继续", turn_id: "run_1" }),
      chainResult("agent_event_goal_chain_result", "收尾完成。")
    ];
    const trajectory = stateWithTurn("run_1", [
      { seq: 1, type: "trajectory.turn.started", run_id: "run_1", turn_id: "run_1", item_id: "turn:run_1", payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "turn", turn_id: "run_1", trace_node_id: "turn:run_1" } },
      { seq: 2, type: "trajectory.turn.completed", run_id: "run_1", turn_id: "run_1", item_id: "turn:run_1", payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "turn", turn_id: "run_1", trace_node_id: "turn:run_1", status: "completed" } }
    ] as AgentEvent[]);
    const sliceMeta = meta({ id: "run_1" });
    sliceMeta["run_1"] = { ...sliceMeta["run_1"]!, turnKind: "settle_slice", startedAt: 0, endedAt: 128 };
    const plan = buildMessageStreamRenderPlan({ messages, trajectory, turnEventMeta: sliceMeta });
    expect(plan.entries.map((entry) => entry.kind)).toEqual(["messages"]);
    expect(plan.chainResultMessages).toHaveLength(1);
  });

  it("终局回复消息不进任何回合组（不产生 loose 碎片组）", () => {
    const plan = buildMessageStreamRenderPlan({
      messages: [chainResult("agent_event_goal_chain_result", "终局。")],
      trajectory: emptyTrajectoryState()
    });
    expect(plan.entries).toHaveLength(0);
    expect(plan.chainResultMessages).toHaveLength(1);
  });

  it("纯函数：相同输入产出相同计划（无隐藏状态）", () => {
    const messages = [chat({ id: "u1", role: "user", content: "检查", turn_id: "run_1" })];
    const trajectory = emptyTrajectoryState();
    const a = buildMessageStreamRenderPlan({ messages, trajectory });
    const b = buildMessageStreamRenderPlan({ messages, trajectory });
    expect(a).toEqual(b);
  });
});

describe("renderPlan × fixtures 组合回放前的 meta 接线", () => {
  it("reduceTurnEventMeta 产出的 meta 直接可喂给渲染计划", () => {
    const events: AgentEvent[] = [
      { seq: 1, type: "item.started", run_id: "run_1", turn_id: "run_1", item_id: "t1", logical_message_id: "agent_item:run_1:t1" } as AgentEvent
    ];
    const turnEventMeta = reduceTurnEventMeta({}, events);
    const plan = buildMessageStreamRenderPlan({
      messages: [chat({ id: "u1", role: "user", content: "检查", turn_id: "run_1" })],
      trajectory: stateWithTurn("run_1", [
        { seq: 2, type: "trajectory.turn.completed", run_id: "run_1", turn_id: "run_1", item_id: "turn:run_1", payload: { schema_version: "vit.observable_trajectory.v1", node_kind: "turn", turn_id: "run_1", trace_node_id: "turn:run_1", status: "completed" } }
      ] as AgentEvent[]),
      turnEventMeta
    });
    const shape = plan.entries.map((entry) => entry.kind === "trace" ? `trace:${entry.turnId}` : "messages");
    expect(shape).toEqual(["messages", "trace:run_1"]);
  });
});

// UI-FOLLOW-1（2026-09-12 用户产品裁定）：轨迹块与状态行跟随对话——轨迹块是
// 「每回合消息的附属组件」（回合内实时更新、回合结束定格为该回合的终态行），
// 只有「确实没有回合附属位」时才允许流尾兜底。
//
// 缺陷形态（真栈手测命中）：轨迹回合的身份键与消息携带的 turn_id 常不在同一
// 命名空间（轮次域 run_/turn:free_state_ 对 chat 的 turn_ 域，失败链更可能只有
// 一条不带 id 的乐观用户消息），身份匹配失败时旧实现把轨迹块一律追加到条目
// 序列尾——「执行完成 N 步」这一行连着步数轨迹坠到整条对话流最底部、被输入框
// 浮层压住。修法：新增回合槽位锚定（以用户消息为界切出的对话回合，取起始时刻
// 之前的最后一个用户消息所在组），身份锚定优先级不变。
describe("UI-FOLLOW-1 回合附属锚定：身份不匹配时按回合槽位入位", () => {
  const T0 = Date.parse("2026-09-13T10:00:00.000Z");

  function turnStarted(turnId: string, seq: number, at: number): AgentEvent {
    return { seq, type: "turn.started", source_turn_id: turnId, created_at: new Date(at).toISOString() } as AgentEvent;
  }

  function traceEvents(turnId: string, seqBase: number, at: number): AgentEvent[] {
    const at0 = new Date(at).toISOString();
    const at1 = new Date(at + 1000).toISOString();
    const at2 = new Date(at + 5000).toISOString();
    return [
      { seq: seqBase, type: "trajectory.turn.started", item_id: `turn:${turnId}`, created_at: at0, payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: `turn:${turnId}`, turn_id: turnId, node_kind: "turn", phase: "framing", status: "running" } },
      { seq: seqBase + 1, type: "trajectory.observation.recorded", item_id: `obs:${turnId}`, created_at: at1, payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: `obs:${turnId}`, turn_id: turnId, node_kind: "observation", phase: "observing", status: "completed", summary: "观察已完成" } },
      { seq: seqBase + 2, type: "trajectory.turn.failed", item_id: `turn:${turnId}`, created_at: at2, payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: `turn:${turnId}`, turn_id: turnId, node_kind: "turn", phase: "failed", status: "failed" } }
    ] as AgentEvent[];
  }

  const shapeOf = (plan: ReturnType<typeof buildMessageStreamRenderPlan>) =>
    plan.entries.map((entry) => entry.kind === "trace"
      ? `trace:${entry.turnId}`
      : `messages:${entry.messages.map((message) => message.id).join(",")}`);

  it("失败链形态（消息不带轨迹回合 id）：轨迹块挂到该回合的用户消息之后（新输出上方），不再坠流尾", () => {
    const turnId = "run_failed_chain";
    const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), traceEvents(turnId, 2, T0 + 800));
    const meta = reduceTurnEventMeta({}, [turnStarted(turnId, 1, T0 + 500)]);
    const messages = [
      chat({ id: "u1", role: "user", content: "把当前工程混得更好一点", createdAt: T0 }),
      chat({ id: "e1", role: "system", content: "这条后台续跑链在执行中失败并已停止", status: "error", createdAt: T0 + 20_000 })
    ];
    const plan = buildMessageStreamRenderPlan({ messages, trajectory, turnEventMeta: meta });
    expect(shapeOf(plan)).toEqual(["messages:u1", `trace:${turnId}`, "messages:e1"]);
    expect(plan.orphanTurnIds).toEqual([]);
  });

  it("两轮跟随：第二轮轨迹块随第二轮用户消息下移，第一轮轨迹块留在自己的回合位（不迁移）", () => {
    const turn1 = "run_round_1";
    const turn2 = "run_round_2";
    const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), [
      ...traceEvents(turn1, 2, T0 + 800),
      ...traceEvents(turn2, 11, T0 + 60_800)
    ]);
    const meta = reduceTurnEventMeta({}, [turnStarted(turn1, 1, T0 + 500), turnStarted(turn2, 10, T0 + 60_500)]);
    const messages = [
      chat({ id: "u1", role: "user", content: "第一问", createdAt: T0 }),
      chat({ id: "a1", role: "assistant", content: "第一轮中间汇报", turn_id: "turn_x1", createdAt: T0 + 30_000 }),
      chat({ id: "u2", role: "user", content: "第二问", createdAt: T0 + 60_000 }),
      chat({ id: "a2", role: "assistant", content: "第二轮中间汇报", turn_id: "turn_x2", createdAt: T0 + 90_000 })
    ];
    const plan = buildMessageStreamRenderPlan({ messages, trajectory, turnEventMeta: meta });
    expect(shapeOf(plan)).toEqual([
      "messages:u1", `trace:${turn1}`, "messages:a1",
      "messages:u2", `trace:${turn2}`, "messages:a2"
    ]);
    expect(plan.orphanTurnIds).toEqual([]);
  });

  it("数据线：App.tsx 把 turnEventMeta 喂给渲染计划（这条线断了锚定会静默退化回流尾）", () => {
    const appSource = readFileSync(new URL("../App.tsx", import.meta.url), "utf8");
    expect(appSource).toMatch(/buildMessageStreamRenderPlan\(\{\s*messages:[^)]*turnEventMeta/);
    expect(appSource).toMatch(/<MessageStream[\s\S]*?turnEventMeta=\{turnEventMeta\}/);
  });

  it("无起始时刻证据：不猜归属，保持流尾兜底（缺证据时零回退）", () => {
    const turnId = "run_no_timing";
    const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), traceEvents(turnId, 2, T0 + 800));
    const messages = [chat({ id: "u1", role: "user", content: "继续", createdAt: T0 })];
    const plan = buildMessageStreamRenderPlan({ messages, trajectory });
    expect(shapeOf(plan)).toEqual(["messages:u1", `trace:${turnId}`]);
    expect(plan.orphanTurnIds).toEqual([turnId]);
  });
});

// UI-FOLLOW-2（2026-09-13 真栈水合实测 + E2E-WEBUI-1 渲染面红，两卡互为验收）：
// 轨迹块坠底有**两个独立成因**，修法必须同时覆盖，且都不放松既有语义
// （身份锚定优先 / 无证据不猜回落流尾 / M12 谓词 / B9 一组一块）。
//
// 证据锚点（取自用户现场，逐字）：
//   - 事件流 trajectory.turn.started.created_at = 2026-09-13T12:26:17.1403545+08:00；
//   - 会话图用户节点 n_20260913T042617_dea1c7cc.created_at =
//     2026-09-13T04:26:17.1423557Z（比回合起始**晚 2 ms**，水合路径的服务端盖章）；
//   - 同一 run_id（run_bab2dcdacb41bdc1）挂三条 ask 节点：
//     12:26:17.142 / 12:27:15.808 / 12:27:50.303（同一 goal 的连续追问共用 run）。
describe("UI-FOLLOW-2 轨迹块坠底：水合时刻晚 2 ms（成因①）+ 同 run 多用户消息（成因②）", () => {
  const T0 = Date.parse("2026-09-13T12:00:00.000+08:00");
  // 真实现场时刻（毫秒）——两种写法都在现场证据里出现过（事件流 +08:00，图节点 Z）。
  const REAL_TURN_STARTED_MS = Date.parse("2026-09-13T12:26:17.1403545+08:00");
  const REAL_OPEN_ASK_MS = Date.parse("2026-09-13T04:26:17.1423557Z");
  const REAL_ASK2_MS = Date.parse("2026-09-13T12:27:15.8085506+08:00");
  const REAL_ASK3_MS = Date.parse("2026-09-13T12:27:50.303323+08:00");
  const REAL_RUN = "run_bab2dcdacb41bdc1";
  // 判据的唯一放宽量，按契约钉死为 50 ms（renderPlan.TURN_SLOT_ANCHOR_TOLERANCE_MS）——
  // 这里写死字面值是为了让边界测试独立约束该契约：模块把容差改成别的值，边界两侧必有一侧失败。
  const TOLERANCE_MS = 50;

  function turnStarted(turnId: string, seq: number, at: number): AgentEvent {
    return { seq, type: "turn.started", source_turn_id: turnId, created_at: new Date(at).toISOString() } as AgentEvent;
  }

  function traceEvents(turnId: string, seqBase: number, at: number): AgentEvent[] {
    const at0 = new Date(at).toISOString();
    const at1 = new Date(at + 1000).toISOString();
    const at2 = new Date(at + 5000).toISOString();
    return [
      { seq: seqBase, type: "trajectory.turn.started", item_id: `turn:${turnId}`, created_at: at0, payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: `turn:${turnId}`, turn_id: turnId, node_kind: "turn", phase: "framing", status: "running" } },
      { seq: seqBase + 1, type: "trajectory.observation.recorded", item_id: `obs:${turnId}`, created_at: at1, payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: `obs:${turnId}`, turn_id: turnId, node_kind: "observation", phase: "observing", status: "completed", summary: "观察已完成" } },
      { seq: seqBase + 2, type: "trajectory.turn.failed", item_id: `turn:${turnId}`, created_at: at2, payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: `turn:${turnId}`, turn_id: turnId, node_kind: "turn", phase: "failed", status: "failed" } }
    ] as AgentEvent[];
  }

  const shapeOf = (plan: ReturnType<typeof buildMessageStreamRenderPlan>) =>
    plan.entries.map((entry) => entry.kind === "trace"
      ? `trace:${entry.turnId}`
      : `messages:${entry.messages.map((message) => message.id).join(",")}`);

  /** E2E-WEBUI-1 断言 1 的语义：块之后必须还有用户消息（它没离开自己的回合槽位） */
  const hasUserMessageAfterBlock = (plan: ReturnType<typeof buildMessageStreamRenderPlan>): boolean => {
    const index = plan.entries.findIndex((entry) => entry.kind === "trace");
    return index >= 0 && plan.entries.slice(index + 1).some((entry) =>
      entry.kind === "messages" && entry.messages.some((message) => message.role === "user"));
  };

  it("成因①（真栈实测 +2 ms）：水合用户消息晚于回合起始，块仍锚定在该消息下方而非坠流尾", () => {
    const turnId = "run_hydrated_late";
    // 现场原始差值：图节点比 trajectory.turn.started 晚 2 ms。
    expect(REAL_OPEN_ASK_MS - REAL_TURN_STARTED_MS).toBe(2);
    const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), traceEvents(turnId, 2, REAL_TURN_STARTED_MS + 800));
    const turnEventMeta = reduceTurnEventMeta({}, [turnStarted(turnId, 1, REAL_TURN_STARTED_MS)]);
    const messages = [
      chat({ id: "hydrated_ask", role: "user", content: "帮低音轨做个均衡实验然后让我试听", createdAt: REAL_OPEN_ASK_MS }),
      chat({ id: "e1", role: "system", content: "这条后台续跑链在执行中失败并已停止", status: "error", createdAt: REAL_OPEN_ASK_MS + 20_000 })
    ];
    const plan = buildMessageStreamRenderPlan({ messages, trajectory, turnEventMeta });
    expect(shapeOf(plan)).toEqual(["messages:hydrated_ask", `trace:${turnId}`, "messages:e1"]);
    expect(plan.orphanTurnIds).toEqual([]);
  });

  it("成因①续（多回合）：滞后 2 ms 不把块往前滑一格——第二个回合的块仍跟第二问", () => {
    const turn1 = "run_round_1";
    const turn2 = "run_round_2";
    const start1 = T0;
    const start2 = T0 + 60_500;
    const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), [
      ...traceEvents(turn1, 2, start1 + 800),
      ...traceEvents(turn2, 11, start2 + 800)
    ]);
    const turnEventMeta = reduceTurnEventMeta({}, [turnStarted(turn1, 1, start1), turnStarted(turn2, 10, start2)]);
    const messages = [
      chat({ id: "u1", role: "user", content: "第一问", createdAt: T0 - 1000 }),
      chat({ id: "a1", role: "assistant", content: "第一轮中间汇报", turn_id: "turn_x1", createdAt: start1 + 30_000 }),
      chat({ id: "u2", role: "user", content: "第二问", createdAt: start2 + 2 }),
      chat({ id: "a2", role: "assistant", content: "第二轮中间汇报", turn_id: "turn_x2", createdAt: start2 + 30_000 })
    ];
    const plan = buildMessageStreamRenderPlan({ messages, trajectory, turnEventMeta });
    expect(shapeOf(plan)).toEqual([
      "messages:u1", `trace:${turn1}`, "messages:a1",
      "messages:u2", `trace:${turn2}`, "messages:a2"
    ]);
    expect(plan.orphanTurnIds).toEqual([]);
  });

  it("成因②（水合后的实际形态）：同 run 两条追问并成一组——块跟开启该回合的第一条，不越过第二条坠底", () => {
    const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), traceEvents(REAL_RUN, 2, REAL_TURN_STARTED_MS + 800));
    const turnEventMeta = reduceTurnEventMeta({}, [turnStarted(REAL_RUN, 1, REAL_TURN_STARTED_MS)]);
    const messages = [
      // 前一日同文本的提问：水合去重后留下的是它（携带自己的 run 域 id），本回合的
      // 开启 ask 因此不在这一组里——组内只剩同 run 的两条追问（真栈 DOM 的产出形态）。
      chat({ id: "u_prev", role: "user", content: "帮低音轨做个均衡实验然后让我试听", turn_id: "run_ea4ad10fbb3a3f99", createdAt: Date.parse("2026-09-12T15:01:44.6960078Z") }),
      chat({ id: "a_prev", role: "assistant", content: "低音轨（bass）目前是空的", turn_id: "turn_dd897b886a5503a7", createdAt: Date.parse("2026-09-13T04:26:53.5162306Z") }),
      chat({ id: "u_ask2", role: "user", content: "我选C", turn_id: REAL_RUN, createdAt: REAL_ASK2_MS }),
      chat({ id: "a_mid", role: "assistant", content: "我看了一下：bass 轨目前是空的", turn_id: "turn_2b17483cc48c2786", createdAt: Date.parse("2026-09-13T04:27:33.6995897Z") }),
      chat({ id: "u_ask3", role: "user", content: "AMEK EQ 200", turn_id: REAL_RUN, createdAt: REAL_ASK3_MS }),
      chat({ id: "a_last", role: "assistant", content: "抱歉，加载没成功", turn_id: "turn_56e88f464b3d02d7", createdAt: Date.parse("2026-09-13T04:28:00.991157Z") })
    ];
    const plan = buildMessageStreamRenderPlan({ messages, trajectory, turnEventMeta });
    expect(shapeOf(plan)).toEqual([
      "messages:u_prev", "messages:a_prev",
      "messages:u_ask2", `trace:${REAL_RUN}`, "messages:u_ask3",
      "messages:a_mid", "messages:a_last"
    ]);
    expect(plan.orphanTurnIds).toEqual([]);
    expect(hasUserMessageAfterBlock(plan)).toBe(true);
  });

  it("成因②（三条 ask 原形）：同 run 三条用户消息——块跟第一条开启消息，其余追问留在块之下", () => {
    const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), traceEvents(REAL_RUN, 2, REAL_TURN_STARTED_MS + 800));
    const turnEventMeta = reduceTurnEventMeta({}, [turnStarted(REAL_RUN, 1, REAL_TURN_STARTED_MS)]);
    const messages = [
      chat({ id: "u_open", role: "user", content: "帮低音轨做个均衡实验然后让我试听", turn_id: REAL_RUN, createdAt: REAL_OPEN_ASK_MS }),
      chat({ id: "a_prev", role: "assistant", content: "低音轨（bass）目前是空的", turn_id: "turn_dd897b886a5503a7", createdAt: Date.parse("2026-09-13T04:26:53.5162306Z") }),
      chat({ id: "u_ask2", role: "user", content: "我选C", turn_id: REAL_RUN, createdAt: REAL_ASK2_MS }),
      chat({ id: "a_mid", role: "assistant", content: "我看了一下：bass 轨目前是空的", turn_id: "turn_2b17483cc48c2786", createdAt: Date.parse("2026-09-13T04:27:33.6995897Z") }),
      chat({ id: "u_ask3", role: "user", content: "AMEK EQ 200", turn_id: REAL_RUN, createdAt: REAL_ASK3_MS }),
      chat({ id: "a_last", role: "assistant", content: "抱歉，加载没成功", turn_id: "turn_56e88f464b3d02d7", createdAt: Date.parse("2026-09-13T04:28:00.991157Z") })
    ];
    const plan = buildMessageStreamRenderPlan({ messages, trajectory, turnEventMeta });
    // 块紧跟在「开启该回合的那条用户消息」之后（该消息在容差内 → 认它，而不是组内最后一条）。
    const traceIndex = plan.entries.findIndex((entry) => entry.kind === "trace");
    const beforeTrace = plan.entries[traceIndex - 1];
    expect(beforeTrace.kind === "messages" ? beforeTrace.messages.map((message) => message.id) : []).toEqual(["u_open"]);
    expect(hasUserMessageAfterBlock(plan)).toBe(true);
    expect(plan.orphanTurnIds).toEqual([]);
  });

  it("反例保持：无起始时刻证据 → 不猜归属，块仍回落流尾（既有语义零回退）", () => {
    const turnId = "run_no_timing_probe";
    const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), traceEvents(turnId, 2, REAL_TURN_STARTED_MS + 800));
    const messages = [chat({ id: "u1", role: "user", content: "继续", createdAt: REAL_OPEN_ASK_MS })];
    const plan = buildMessageStreamRenderPlan({ messages, trajectory });
    expect(shapeOf(plan)).toEqual(["messages:u1", `trace:${turnId}`]);
    expect(plan.orphanTurnIds).toEqual([turnId]);
  });

  it("反例保持：回合起始早于全部用户消息且超出容差 → 不认领，仍回落流尾", () => {
    const turnId = "run_before_all_users";
    const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), traceEvents(turnId, 2, T0 + 800));
    const turnEventMeta = reduceTurnEventMeta({}, [turnStarted(turnId, 1, T0)]);
    const messages = [chat({ id: "u1", role: "user", content: "继续", createdAt: T0 + 5_000 })];
    const plan = buildMessageStreamRenderPlan({ messages, trajectory, turnEventMeta });
    expect(shapeOf(plan)).toEqual(["messages:u1", `trace:${turnId}`]);
    expect(plan.orphanTurnIds).toEqual([turnId]);
  });

  it("容差边界被钉死：回合起始 +50 ms 内的水合滞后认作同一回合，超出 1 ms 即不认", () => {
    const turnId = "run_tolerance_boundary";
    const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), traceEvents(turnId, 2, T0 + 800));
    const turnEventMeta = reduceTurnEventMeta({}, [turnStarted(turnId, 1, T0)]);
    const anchored = buildMessageStreamRenderPlan({
      messages: [chat({ id: "u1", role: "user", content: "继续", createdAt: T0 + TOLERANCE_MS })],
      trajectory,
      turnEventMeta
    });
    expect(anchored.orphanTurnIds).toEqual([]);
    expect(shapeOf(anchored)).toEqual(["messages:u1", `trace:${turnId}`]);
    const rejected = buildMessageStreamRenderPlan({
      messages: [chat({ id: "u1", role: "user", content: "继续", createdAt: T0 + TOLERANCE_MS + 1 })],
      trajectory,
      turnEventMeta
    });
    expect(rejected.orphanTurnIds).toEqual([turnId]);
  });
});

