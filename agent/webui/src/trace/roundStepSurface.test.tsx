import { readFileSync } from "node:fs";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { chatMessageFromAgentEvent } from "../App";
import { emptyTrajectoryState, reduceTrajectoryEvents } from "../trajectory";
import type { AgentEvent, ChatMessage } from "../types";
import { buildMessageStreamRenderPlan } from "./renderPlan";
import { emptyRoundStepMap, reduceRoundSteps, roundActivityBoundKeys, roundTurnShell, type RoundStepMap } from "./roundSteps";
import { TraceBlock } from "./TraceBlock";
import { isUnboundActivity } from "./turnGroups";
import { reduceTurnEventMeta } from "./turnEventMeta";
import { reduceAgentEventActivities } from "../messageLifecycle";

// TRAJ-IMPL-2（设计 docs/TRAJECTORY_PRESENTATION_REDESIGN_V1.md §2.1 + §7 裁定 A）：
// **回合单活动面**的端到端钉（纯函数 → 渲染计划 → 渲染面 → 活动线）。
//
// 缺陷形态（F2）：纯 chat 回合（无 trajectory 事件）执行 31s，item.* 逐条在场，却
// 因为「容器存在性 = 实验轨迹节点非空」连块都没有，进度只能落在流底活动线。
// 修法：谓词放宽到「任一活动证据」+ item 步内联 + 人话标题 + 活动线去重；
// M12 settle_slice 隐藏谓词逐字保留（本文件含「settle_slice 仍隐藏」钉）。

const RUN = "run_chat_only_1";
const T0 = Date.parse("2026-09-14T10:00:00.000Z");
const at = (ms: number) => new Date(ms).toISOString();

function chat(partial: Partial<ChatMessage> & Pick<ChatMessage, "id" | "role" | "content">): ChatMessage {
  return { createdAt: 0, ...partial } as ChatMessage;
}

/** 纯 chat 回合（无任何 trajectory.* 事件）：turn.started → 三次工具调用 */
function chatOnlyTurnEvents(): AgentEvent[] {
  return [
    { seq: 1, type: "turn.started", conversation_id: "c1", goal_id: "goal_1", run_id: RUN, turn_id: RUN, source_turn_id: RUN, status: "running", created_at: at(T0) },
    { seq: 2, type: "item.started", conversation_id: "c1", goal_id: "goal_1", run_id: RUN, turn_id: RUN, source_turn_id: RUN, item_id: "tool_step_1", item_type: "daw_action", status: "running", created_at: at(T0 + 1_000), payload: { tool: "ccb.observation_catalog" } },
    { seq: 3, type: "item.completed", conversation_id: "c1", goal_id: "goal_1", run_id: RUN, turn_id: RUN, source_turn_id: RUN, item_id: "tool_step_1", item_type: "daw_action", status: "completed", created_at: at(T0 + 2_500), payload: { command_name: "ccb_observation_catalog" } },
    { seq: 4, type: "item.started", conversation_id: "c1", goal_id: "goal_1", run_id: RUN, turn_id: RUN, source_turn_id: RUN, item_id: "tool_step_1", item_type: "daw_action", status: "running", created_at: at(T0 + 4_000), payload: { tool: "mix_tick.pending" } },
    { seq: 5, type: "item.completed", conversation_id: "c1", goal_id: "goal_1", run_id: RUN, turn_id: RUN, source_turn_id: RUN, item_id: "tool_step_1", item_type: "daw_action", status: "completed", created_at: at(T0 + 6_000), payload: { command_name: "mix_tick" } },
    { seq: 6, type: "item.started", conversation_id: "c1", goal_id: "goal_1", run_id: RUN, turn_id: RUN, source_turn_id: RUN, item_id: "tool_step_1", item_type: "daw_action", status: "running", created_at: at(T0 + 8_000), payload: { tool: "weird.custom_tool" } }
  ] as AgentEvent[];
}

describe("钉A 渲染谓词放宽：无实验回合的执行期也出容器（F2 可见性错绑修法）", () => {
  it("只有 item 事件的回合（trajectory 里没有它）→ 渲染计划给出 trace 条目并锚定在该回合用户消息之后", () => {
    const events = chatOnlyTurnEvents();
    // 用户消息时刻按真栈水合形态：服务端图节点盖章比回合起始晚 ~2ms（UI-FOLLOW-2 证据）
    const messages = [
      chat({ id: "u1", role: "user", content: "看一下当前工程有什么问题", createdAt: T0 + 2 }),
      chat({ id: "a1", role: "assistant", content: "我还在继续处理这个任务。", createdAt: T0 + 20_000 })
    ];
    const plan = buildMessageStreamRenderPlan({
      messages,
      trajectory: emptyTrajectoryState(),
      turnEventMeta: reduceTurnEventMeta({}, events),
      roundSteps: reduceRoundSteps(emptyRoundStepMap(), events)
    });
    expect(plan.entries.map((entry) => entry.kind === "trace"
      ? `trace:${entry.turnId}`
      : entry.kind === "receipt"
        ? `receipt:${entry.turnId}`
        : `messages:${entry.messages.map((message) => message.id).join(",")}`))
      .toEqual(["messages:u1", `trace:${RUN}`, "messages:a1"]);
    expect(plan.orphanTurnIds).toEqual([]);
  });

  it("轨迹回合的记录不受影响（同一次计划里两类候选并存，顺序按回合起始 seq）", () => {
    const itemEvents = chatOnlyTurnEvents();
    const trajectoryEvents: AgentEvent[] = [
      { seq: 50, type: "trajectory.turn.started", source_turn_id: "run_experiment_1", item_id: "turn:run_experiment_1", status: "running", created_at: at(T0 + 60_000), payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "turn:run_experiment_1", turn_id: "run_experiment_1", node_kind: "turn", status: "running" } },
      { seq: 51, type: "trajectory.observation.recorded", source_turn_id: "run_experiment_1", item_id: "obs-1", status: "completed", created_at: at(T0 + 61_000), payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "obs-1", turn_id: "run_experiment_1", node_kind: "observation", status: "completed" } }
    ] as AgentEvent[];
    const plan = buildMessageStreamRenderPlan({
      messages: [
        chat({ id: "u1", role: "user", content: "问", createdAt: T0 + 2 }),
        chat({ id: "u2", role: "user", content: "第二轮问", turn_id: "turn_chat_domain_2", createdAt: T0 + 60_000 })
      ],
      trajectory: reduceTrajectoryEvents(emptyTrajectoryState(), trajectoryEvents),
      turnEventMeta: reduceTurnEventMeta({}, [...itemEvents, ...trajectoryEvents]),
      roundSteps: reduceRoundSteps(emptyRoundStepMap(), itemEvents)
    });
    expect(plan.entries.filter((entry) => entry.kind === "trace").map((entry) => entry.kind === "trace" && entry.turnId))
      .toEqual([RUN, "run_experiment_1"]);
  });
});

describe("钉B 步内联（人话标题）：回合块出现且 item 步逐条内联（复用 TraceStep）", () => {
  it("执行期（无收口事件）渲染 live 回合块，item 步带人话标题与工具步脚注", () => {
    const events = chatOnlyTurnEvents();
    const rounds = reduceRoundSteps(emptyRoundStepMap(), events);
    const turn = roundTurnShell(rounds[RUN]);
    const markup = renderToStaticMarkup(
      <TraceBlock
        state={emptyTrajectoryState()}
        turn={turn!}
        activities={[]}
        turnMeta={reduceTurnEventMeta({}, events)[RUN]}
        itemSteps={rounds[RUN].steps}
        totalStepCount={rounds[RUN].totalStepCount}
      />
    );
    // 容器：执行期是 live 且展开（不是回执行坍缩）
    expect(markup).toContain("trace-block");
    expect(markup).toContain("is-live");
    expect(markup).not.toContain("is-collapsed");
    expect(markup).toContain("正在处理");
    // 步内联：三次调用三步，人话标题逐条（常见族命中 + 未命中原样）
    expect(markup).toContain("已完成 可用观察视图清单");
    expect(markup).toContain("已完成 混音调整");
    expect(markup).toContain("weird.custom_tool");
    expect(markup).toContain("工具步骤");
    expect(markup.match(/trace-step/g) ?? []).toHaveLength(3);
    expect(markup).toContain("3 步");
    // 未收口的最后一步在执行期是「进行中」，收口的两步是静态时长
    expect(markup).toContain("进行中");
  });

  it("滚动窗口截断在头部显式标注（总计数不缩水、省略数不隐瞒）", () => {
    const rounds = reduceRoundSteps(emptyRoundStepMap(), chatOnlyTurnEvents());
    const markup = renderToStaticMarkup(
      <TraceBlock
        state={emptyTrajectoryState()}
        turn={roundTurnShell(rounds[RUN])!}
        activities={[]}
        itemSteps={rounds[RUN].steps.slice(0, 2)}
        totalStepCount={rounds[RUN].totalStepCount}
      />
    );
    expect(markup).toContain("2 步（较早 1 步已省略）");
  });
});

describe("钉C M12 钉：settle_slice 仍隐藏（谓词放宽不得让结算切片噪音回归）", () => {
  it("settle_slice 切片回合（无步、带标记）不产生容器——有 roundSteps 输入也一样", () => {
    const sliceEvents: AgentEvent[] = [
      { seq: 1, type: "turn.started", goal_id: "goal_1", run_id: "run_slice_1", turn_id: "run_slice_1", source_turn_id: "run_slice_1", status: "running", created_at: at(T0) },
      { seq: 2, type: "turn.completed", goal_id: "goal_1", run_id: "run_slice_1", turn_id: "run_slice_1", source_turn_id: "run_slice_1", status: "completed", created_at: at(T0 + 128), payload: { turn_kind: "settle_slice" } }
    ] as AgentEvent[];
    const turnEventMeta = reduceTurnEventMeta({}, sliceEvents);
    const plan = buildMessageStreamRenderPlan({
      messages: [chat({ id: "u1", role: "user", content: "继续", createdAt: T0 - 1_000 })],
      trajectory: emptyTrajectoryState(),
      turnEventMeta,
      roundSteps: reduceRoundSteps(emptyRoundStepMap(), sliceEvents)
    });
    expect(plan.entries.map((entry) => entry.kind)).toEqual(["messages"]);
    // 隐藏 = 根本不是候选：既不出块，也不被当成孤儿兜到流尾（否则「仍隐藏」不成立）
    expect(plan.orphanTurnIds).toEqual([]);
    expect(plan.entries.some((entry) => entry.kind === "trace")).toBe(false);
  });

  it("轨迹侧的 M12 谓词逐字保留：带标记且有轨迹壳节点的切片回合仍不渲染（谓词零改动）", () => {
    const sliceEvents: AgentEvent[] = [
      { seq: 1, type: "trajectory.turn.started", source_turn_id: "run_slice_2", item_id: "turn:run_slice_2", created_at: at(T0), payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "turn:run_slice_2", turn_id: "run_slice_2", node_kind: "turn", status: "running" } },
      { seq: 2, type: "trajectory.turn.completed", source_turn_id: "run_slice_2", item_id: "turn:run_slice_2", created_at: at(T0 + 128), payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "turn:run_slice_2", turn_id: "run_slice_2", node_kind: "turn", status: "completed" } },
      { seq: 3, type: "turn.started", source_turn_id: "run_slice_2", created_at: at(T0) },
      { seq: 4, type: "turn.completed", source_turn_id: "run_slice_2", status: "completed", created_at: at(T0 + 128), payload: { turn_kind: "settle_slice" } }
    ] as AgentEvent[];
    const plan = buildMessageStreamRenderPlan({
      messages: [chat({ id: "u1", role: "user", content: "继续", createdAt: T0 - 1_000 })],
      trajectory: reduceTrajectoryEvents(emptyTrajectoryState(), sliceEvents),
      turnEventMeta: reduceTurnEventMeta({}, sliceEvents),
      roundSteps: reduceRoundSteps(emptyRoundStepMap(), sliceEvents)
    });
    expect(plan.entries.some((entry) => entry.kind === "trace")).toBe(false);
    expect(plan.orphanTurnIds).toEqual([]);
  });
});

describe("钉D 活动线去重：已归属回合的 item 活动从流底 lane 消失（§2.1-5）", () => {
  it("同一段 item 事件：活动 id 落在步账钥匙里 → 不再算 unbound；无归属活动（上传）照旧", () => {
    const events = chatOnlyTurnEvents().filter((event) => event.type === "item.started");
    const activities = reduceAgentEventActivities([], events, (event) => chatMessageFromAgentEvent(event, "default"));
    expect(activities.length).toBeGreaterThan(0);
    const rounds: RoundStepMap = reduceRoundSteps(emptyRoundStepMap(), events);
    const boundKeys = roundActivityBoundKeys(rounds);
    const knownTurnIds = new Set<string>();  // 纯 chat 回合：轨迹域里没有这个回合（旧判据必然 unbound）
    const upload = chat({ id: "upload-1", role: "system", content: "正在上传素材…", createdAt: T0 + 1 });
    for (const activity of activities) {
      expect(isUnboundActivity(activity, knownTurnIds)).toBe(true);            // 旧判据：跨命名空间 → 落 lane
      expect(isUnboundActivity(activity, knownTurnIds, boundKeys)).toBe(false); // 新判据：已归属回合 → 消失
    }
    expect(isUnboundActivity(upload, knownTurnIds, boundKeys)).toBe(true);
  });

  it("收口后活动被清退（既有语义零回退）：步账仍在，容器仍有证据", () => {
    const events = chatOnlyTurnEvents();
    const activities = reduceAgentEventActivities([], events, (event) => chatMessageFromAgentEvent(event, "default"));
    const rounds = reduceRoundSteps(emptyRoundStepMap(), events);
    // 只有最后一次调用还挂着（前两次 item.completed 已清退）
    expect(activities.every((activity) => activity.turn_id === RUN)).toBe(true);
    expect(rounds[RUN].steps).toHaveLength(3);
  });
});

describe("钉E 数据线：App.tsx 把步账喂进计划/渲染/活动线（这条线断了会静默退回旧形态）", () => {
  const appSource = readFileSync(new URL("../App.tsx", import.meta.url), "utf8");

  it("归约接线 + 计划传参 + 渲染传参 + lane 过滤四段都在", () => {
    expect(appSource).toMatch(/setRoundSteps\(\(current\) => reduceRoundSteps\(current, events\)\)/);
    expect(appSource).toMatch(/buildMessageStreamRenderPlan\(\{\s*messages:[^)]*roundSteps/);
    expect(appSource).toMatch(/isUnboundActivity\(activity, knownTurnIds, roundBoundKeys\)/);
    expect(appSource).toMatch(/itemSteps=\{round\?\.steps\}/);
    expect(appSource).toMatch(/totalStepCount=\{round\?\.totalStepCount\}/);
    expect(appSource).toMatch(/trajectory\.turns\[entry\.turnId\] \?\? roundTurnShell\(round\)/);
  });
});

describe("钉F 零回退：既有轨迹回合在步账缺省时逐字不变", () => {
  it("不传 roundSteps：计划与渲染与 TRAJ-IMPL-2 之前一致（轨迹步照常、无 item 步）", () => {
    const events: AgentEvent[] = [
      { seq: 1, type: "trajectory.turn.started", source_turn_id: RUN, item_id: `turn:${RUN}`, created_at: at(T0), payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: `turn:${RUN}`, turn_id: RUN, node_kind: "turn", status: "running" } },
      { seq: 2, type: "trajectory.observation.recorded", source_turn_id: RUN, item_id: "obs-1", created_at: at(T0 + 1_000), payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "obs-1", turn_id: RUN, node_kind: "observation", status: "completed", summary: "频率关系观察" } }
    ] as AgentEvent[];
    const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), events);
    const plan = buildMessageStreamRenderPlan({
      messages: [chat({ id: "u1", role: "user", content: "问", createdAt: T0 - 1_000 })],
      trajectory,
      turnEventMeta: reduceTurnEventMeta({}, events)
    });
    expect(plan.entries.map((entry) => entry.kind)).toEqual(["messages", "trace"]);
    const markup = renderToStaticMarkup(
      <TraceBlock state={trajectory} turn={trajectory.turns[RUN]} activities={[]} turnMeta={reduceTurnEventMeta({}, events)[RUN]} />
    );
    expect(markup).toContain("1 步");
    expect(markup).toContain("频率关系观察");
    expect(markup).not.toContain("工具步骤");
  });
});
