import { describe, expect, it } from "vitest";
import { reduceTurnEventMeta, trajectoryTurnIdOfEvent, turnDurationSplit } from "./turnEventMeta";
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

  it("C0 双读（B9 轮次键）：归属键优先 source_turn_id，缺失回退 trajectory_turn_id / turn_id / run_id", () => {
    expect(trajectoryTurnIdOfEvent(event({ seq: 1, type: "turn.started", run_id: "run_a", turn_id: "run_b", trajectory_turn_id: "run_c", source_turn_id: "run_d" }))).toBe("run_d");
    expect(trajectoryTurnIdOfEvent(event({ seq: 1, type: "turn.started", run_id: "run_a", turn_id: "run_b", trajectory_turn_id: "run_c" }))).toBe("run_c");
    expect(trajectoryTurnIdOfEvent(event({ seq: 1, type: "turn.started", run_id: "run_a", turn_id: "run_b" }))).toBe("run_b");
    expect(trajectoryTurnIdOfEvent(event({ seq: 1, type: "turn.started", run_id: "run_a" }))).toBe("run_a");
  });

  // CONT-STALL-1 主钉（2026-09-12 22:53 真栈时间线逐字，工件
  // artifacts/b12_1_blind/forensics/events6.json）：切片 22:53:11 开始、
  // 22:54:06 以 waiting_continue 结束并驻留，用户 22:57:30 手动停止才补上
  // trajectory.turn.stopped。该事件与 turn.completed 同属一个 turn，
  // 此前被并进 endedAt 取 max，于是 258.3 s 的驻留墙钟被当成执行时长呈现。
  describe("CONT-STALL-1 工作/驻留分离", () => {
    const realStackTimeline = (): AgentEvent[] => [
      event({ seq: 1, type: "turn.started", source_turn_id: "run_park", created_at: "2026-09-12T22:53:11.745Z" }),
      event({ seq: 2, type: "trajectory.turn.started", source_turn_id: "run_park", created_at: "2026-09-12T22:53:11.745Z" }),
      event({ seq: 3, type: "item.started", source_turn_id: "run_park", item_id: "tool_step_1", logical_message_id: "agent_item:run_park:tool_step_1", created_at: "2026-09-12T22:53:26.993Z" }),
      event({ seq: 4, type: "item.completed", source_turn_id: "run_park", item_id: "tool_step_1", logical_message_id: "agent_item:run_park:tool_step_1", created_at: "2026-09-12T22:53:27.018Z" }),
      event({ seq: 5, type: "turn.completed", source_turn_id: "run_park", created_at: "2026-09-12T22:54:06.557Z" }),
      event({ seq: 6, type: "trajectory.turn.stopped", source_turn_id: "run_park", created_at: "2026-09-12T22:57:30.069Z" })
    ];

    it("turn.stopped 不计入工作终点：endedAt 停在 turn.completed，驻留终点单独记", () => {
      const meta = reduceTurnEventMeta({}, realStackTimeline())["run_park"];
      expect(meta?.endedAt).toBe(Date.parse("2026-09-12T22:54:06.557Z"));
      expect(meta?.residencyEndedAt).toBe(Date.parse("2026-09-12T22:57:30.069Z"));
    });

    it("拆分口径：执行 54.8s、等待续跑 203.5s（卡面「驻留墙钟计成执行时长」的反向锁定）", () => {
      const meta = reduceTurnEventMeta({}, realStackTimeline())["run_park"];
      const split = turnDurationSplit(meta);
      expect(split.workMs).toBe(Date.parse("2026-09-12T22:54:06.557Z") - Date.parse("2026-09-12T22:53:11.745Z"));
      expect(split.parkMs).toBe(Date.parse("2026-09-12T22:57:30.069Z") - Date.parse("2026-09-12T22:54:06.557Z"));
      // 旧口径（endedAt 取 max 后相减）会得到 258.3s 的总时长；工作片不得包含驻留。
      expect(split.workMs! + split.parkMs!).toBe(Date.parse("2026-09-12T22:57:30.069Z") - Date.parse("2026-09-12T22:53:11.745Z"));
      expect(split.workMs! < split.parkMs!).toBe(true);
    });

    it("无驻留终点的回合 parkMs 保持 null（不虚报等待）", () => {
      const meta = reduceTurnEventMeta({}, [
        event({ seq: 1, type: "turn.started", source_turn_id: "run_ok", created_at: "2026-09-05T15:00:00.000Z" }),
        event({ seq: 2, type: "turn.completed", source_turn_id: "run_ok", created_at: "2026-09-05T15:02:22.000Z" })
      ])["run_ok"];
      expect(meta?.residencyEndedAt).toBeUndefined();
      expect(turnDurationSplit(meta)).toEqual({ workMs: 142_000, parkMs: null });
    });

    it("真·运行中被停止（无工作终局）：整段算工作，不虚构驻留段", () => {
      const meta = reduceTurnEventMeta({}, [
        event({ seq: 1, type: "turn.started", source_turn_id: "run_stop", created_at: "2026-09-05T15:00:00.000Z" }),
        event({ seq: 2, type: "turn.stopped", source_turn_id: "run_stop", created_at: "2026-09-05T15:00:12.000Z" })
      ])["run_stop"];
      expect(turnDurationSplit(meta)).toEqual({ workMs: 12_000, parkMs: null });
    });

    it("迟到/重复的驻留事件取最大时间戳，不重复记账", () => {
      const base = reduceTurnEventMeta({}, realStackTimeline());
      const again = reduceTurnEventMeta(base, realStackTimeline());
      expect(again["run_park"]?.residencyEndedAt).toBe(Date.parse("2026-09-12T22:57:30.069Z"));
      expect(again["run_park"]?.itemActivityCount).toBe(1);
    });
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
