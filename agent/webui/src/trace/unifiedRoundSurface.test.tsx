import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { emptyTrajectoryState, reduceTrajectoryEvents, trajectoryTurns } from "../trajectory";
import { chatMessageFromAgentEvent } from "../App";
import type { AgentEvent, ChatMessage } from "../types";
import { buildMessageStreamRenderPlan } from "./renderPlan";
import { chainResultMessagesFromEvents } from "./traceDelivery";
import { reduceTurnEventMeta } from "./turnEventMeta";
import { TraceBlock } from "./TraceBlock";

const fixturesDir = new URL("./__fixtures__/", import.meta.url).pathname.replace(/^\/([A-Za-z]:)/, "$1");

function loadFixture(name: string): AgentEvent[] {
  const raw = JSON.parse(readFileSync(fixturesDir + name, "utf-8")) as { events?: AgentEvent[] };
  return raw.events ?? [];
}

function chat(partial: Partial<ChatMessage> & Pick<ChatMessage, "id" | "role" | "content">): ChatMessage {
  return { createdAt: 0, ...partial } as ChatMessage;
}

// B9 统一面（2026-09-11 用户裁定）：一轮对话一个统一轨迹块，终局并入原块，
// 下一次输入开启新一轮后才出现新块。fixture 为真栈取证实锤（free-state 默认
// chat 链 91s 形态）：run_3d7968a724ed4ced（chat 轮壳+item 活动+调度分片+终局）
// 与 turn:free_state_ed748b62646b4687（实验轨迹 6 步）双域并存、全部事件带
// run 级 source_turn_id——归并键就是它。
const RUN_ID = "run_3d7968a724ed4ced";

describe("B9 统一面：mtwwegtp 真栈取证回放（free-state chat 链一轮一块）", () => {
  const events = loadFixture("2026-09-11-webui-mtwwegtp-19events.json");

  it("钉1：run 壳与实验轨迹归并为单一轮次回合（6 步），free_state 域不再独立成块", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), events);
    const turns = trajectoryTurns(state);
    expect(turns.map((turn) => turn.id)).toEqual([RUN_ID]);
    const turn = turns[0];
    expect(turn.roundScoped).toBe(true);
    const stepNodes = turn.nodeIds
      .map((id) => state.nodes[id])
      .filter((node): node is NonNullable<typeof node> => Boolean(node))
      .filter((node) => node.kind !== "turn")
      .sort((left, right) => left.seq - right.seq);
    expect(stepNodes).toHaveLength(6);
    expect(stepNodes.map((node) => node.kind)).toEqual(["intent", "hypothesis", "decision", "observation", "action", "observation"]);
  });

  it("钉2a：批量轨迹事件到达（seq15，终局未至）回合保持 live——完成态步节点不提前收块", () => {
    const partial = events.filter((event) => event.seq <= 15);
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), partial);
    const turn = state.turns[RUN_ID];
    expect(turn).toBeDefined();
    expect(turn.status).toBe("running");
    expect(turn.terminalStatus).toBe("");
  });

  it("钉2b：终局并入原块——trajectory.turn.completed 收口同一轮次，不再另出块", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), events);
    const turn = state.turns[RUN_ID];
    expect(turn.status).toBe("completed");
    expect(turn.terminalStatus).toBe("completed");
    // 全流只有一个轮次回合（旧形态的 run 壳空块+free_state 孤儿块都不复现）
    expect(trajectoryTurns(state)).toHaveLength(1);
    // 终局链消息只合成一条（scheduler_chain）
    const chainMessages = chainResultMessagesFromEvents(events, "default", chatMessageFromAgentEvent);
    expect(chainMessages).toHaveLength(1);
  });

  it("钉2c：下一次输入才出新块——第二轮（新 run 域）事件归并为第二个轮次回合", () => {
    const secondRound: AgentEvent[] = events.map((event) => {
      const payload = event.payload as Record<string, unknown> | undefined;
      return {
        ...event,
        seq: event.seq + 100,
        run_id: "run_second_round",
        turn_id: "run_second_round",
        trajectory_turn_id: "run_second_round",
        source_turn_id: "run_second_round",
        item_id: event.item_id ? `${event.item_id}_r2` : "",
        logical_message_id: event.logical_message_id ? `${event.logical_message_id}_r2` : "",
        payload: payload
          ? {
              ...payload,
              turn_id: "run_second_round",
              ...(payload.trace_node_id ? { trace_node_id: `${String(payload.trace_node_id)}_r2` } : {}),
              ...(payload.round_id ? { round_id: `${String(payload.round_id)}_r2` } : {})
            }
          : payload
      };
    }) as AgentEvent[];
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), [...events, ...secondRound]);
    const turnIds = trajectoryTurns(state).map((turn) => turn.id);
    expect(turnIds).toHaveLength(2);
    expect(turnIds).toContain(RUN_ID);
    expect(turnIds).toContain("run_second_round");
    // 两轮各自 6 步，互不吞并
    for (const id of turnIds) {
      const steps = state.turns[id].nodeIds
        .map((nodeId) => state.nodes[nodeId])
        .filter((node) => node && node.kind !== "turn");
      expect(steps).toHaveLength(6);
    }
  });

  it("钉3：终局后无空壳文案——块内 6 步渲染，不再出现「本回合尚未产生轨迹节点」", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), events);
    const meta = reduceTurnEventMeta({}, events);
    const turn = trajectoryTurns(state)[0];
    const markup = renderToStaticMarkup(
      <TraceBlock state={state} turn={turn} activities={[]} turnMeta={meta[RUN_ID]} />
    );
    expect(markup).not.toContain("本回合尚未产生轨迹节点");
    expect(markup).toContain("执行完成");
    expect(markup).toContain("6 步");
    expect(markup).toContain("实验意图已确立");
    expect(markup).toContain("类型化干预已应用");
  });

  it("钉4：渲染计划——用户消息 → 单一轨迹块（锚定）→ 中间汇报 → 终局回复，零孤儿块", () => {
    const meta = reduceTurnEventMeta({}, events);
    const trajectory = reduceTrajectoryEvents(emptyTrajectoryState(), events);
    const messages = [
      chat({ id: "u1", role: "user", content: "检查一下当前工程有什么问题吗" }),
      chat({ id: "a1", role: "assistant", content: "我还在继续处理这个任务，完成后再向你汇报。", turn_id: RUN_ID }),
      ...chainResultMessagesFromEvents(events, "default", chatMessageFromAgentEvent)
    ];
    const plan = buildMessageStreamRenderPlan({ messages, trajectory, turnEventMeta: meta });
    const traceEntries = plan.entries.filter((entry) => entry.kind === "trace");
    expect(traceEntries).toHaveLength(1);
    expect(traceEntries[0].turnId).toBe(RUN_ID);
    expect(plan.orphanTurnIds).toEqual([]);
    expect(plan.chainResultMessages).toHaveLength(1);
    // meta 记账与轮次键对齐（item 活动足迹落在 run 键上）
    expect(meta[RUN_ID]?.itemActivityCount).toBeGreaterThan(0);
    expect(meta[RUN_ID]?.turnKind).toBe("settle_slice");
  });
});
