import { describe, expect, it } from "vitest";
import { reduceTurnEventMeta, trajectoryTurnIdOfEvent } from "./turnEventMeta";
import type { AgentEvent } from "../types";

function event(partial: Partial<AgentEvent> & Pick<AgentEvent, "seq" | "type">): AgentEvent {
  return { goal_id: "goal-1", ...partial } as AgentEvent;
}

describe("turnEventMeta 归约器（GUI-1：item 活动足迹与生命周期记账）", () => {
  it("item 活动（started/completed）按 turn 记数，turn 生命周期节点不计活动", () => {
    let meta = reduceTurnEventMeta({}, [
      event({ seq: 1, type: "turn.started", run_id: "run_1", turn_id: "run_1", created_at: "2026-09-05T15:00:00Z" }),
      event({ seq: 2, type: "trajectory.turn.started", run_id: "run_1", turn_id: "run_1", item_id: "turn:run_1" }),
      event({ seq: 3, type: "item.started", run_id: "run_1", turn_id: "run_1", item_id: "tool_step_1", logical_message_id: "agent_item:run_1:tool_step_1" }),
      event({ seq: 4, type: "item.completed", run_id: "run_1", turn_id: "run_1", item_id: "tool_step_1", logical_message_id: "agent_item:run_1:tool_step_1" })
    ]);
    expect(meta["run_1"]?.itemActivityCount).toBe(1);
    expect(meta["run_1"]?.startedAt).toBeDefined();
  });

  it("重复/迟到投递的 item 事件按 logical_message_id 去重，不虚增活动数", () => {
    const batch = [
      event({ seq: 1, type: "item.started", run_id: "run_1", turn_id: "run_1", item_id: "tool_step_1", logical_message_id: "agent_item:run_1:tool_step_1" }),
      event({ seq: 2, type: "item.completed", run_id: "run_1", turn_id: "run_1", item_id: "tool_step_1", logical_message_id: "agent_item:run_1:tool_step_1" })
    ];
    let meta = reduceTurnEventMeta({}, batch);
    meta = reduceTurnEventMeta(meta, batch);
    meta = reduceTurnEventMeta(meta, [...batch, ...batch.map((item) => ({ ...item, seq: item.seq + 100 }))]);
    expect(meta["run_1"]?.itemActivityCount).toBe(1);
  });

  it("无 logical_message_id 的 item 事件按 item_id 去重", () => {
    const batch = [
      event({ seq: 1, type: "item.started", run_id: "run_1", turn_id: "run_1", item_id: "tool_step_1" }),
      event({ seq: 2, type: "item.completed", run_id: "run_1", turn_id: "run_1", item_id: "tool_step_1" }),
      event({ seq: 3, type: "item.completed", run_id: "run_1", turn_id: "run_1", item_id: "tool_step_1" })
    ];
    const meta = reduceTurnEventMeta({}, batch);
    expect(meta["run_1"]?.itemActivityCount).toBe(1);
  });

  it("生命周期：turn.started→终局的 created_at 差可读；无终局时 endedAt 缺失", () => {
    let meta = reduceTurnEventMeta({}, [
      event({ seq: 1, type: "turn.started", run_id: "run_1", turn_id: "run_1", created_at: "2026-09-05T15:00:00.000Z" })
    ]);
    expect(meta["run_1"]?.startedAt).toBe(Date.parse("2026-09-05T15:00:00.000Z"));
    expect(meta["run_1"]?.endedAt).toBeUndefined();
    meta = reduceTurnEventMeta(meta, [
      event({ seq: 2, type: "turn.completed", run_id: "run_1", turn_id: "run_1", created_at: "2026-09-05T15:02:22.000Z" })
    ]);
    expect(meta["run_1"]?.endedAt).toBe(Date.parse("2026-09-05T15:02:22.000Z"));
  });

  it("turn_kind 标记从终局事件 payload 捕获（CONTRACT-1 settle_slice）", () => {
    const meta = reduceTurnEventMeta({}, [
      event({
        seq: 14,
        type: "turn.completed",
        run_id: "run_1",
        turn_id: "run_1",
        item_id: "chain_result",
        payload: { scheduler_chain: true, turn_kind: "settle_slice" }
      })
    ]);
    expect(meta["run_1"]?.turnKind).toBe("settle_slice");
  });

  it("C0 双读：轨迹归属键优先 trajectory_turn_id，缺失回退 turn_id/run_id", () => {
    expect(trajectoryTurnIdOfEvent(event({ seq: 1, type: "turn.started", run_id: "run_a", turn_id: "run_b", trajectory_turn_id: "run_c" }))).toBe("run_c");
    expect(trajectoryTurnIdOfEvent(event({ seq: 1, type: "turn.started", run_id: "run_a", turn_id: "run_b" }))).toBe("run_b");
    expect(trajectoryTurnIdOfEvent(event({ seq: 1, type: "turn.started", run_id: "run_a" }))).toBe("run_a");
  });

  it("增量归约保持既有足迹（分批轮询到达不丢账）", () => {
    let meta = reduceTurnEventMeta({}, [
      event({ seq: 1, type: "item.started", run_id: "run_1", turn_id: "run_1", item_id: "tool_step_1", logical_message_id: "agent_item:run_1:tool_step_1" })
    ]);
    meta = reduceTurnEventMeta(meta, [
      event({ seq: 2, type: "item.started", run_id: "run_1", turn_id: "run_1", item_id: "tool_step_2", logical_message_id: "agent_item:run_1:tool_step_2" })
    ]);
    expect(meta["run_1"]?.itemActivityCount).toBe(2);
  });
});
