import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { AgentEvent, ChatMessage } from "../types";
import { emptyTrajectoryState, reduceTrajectoryEvents, trajectoryTurns } from "../trajectory";
import { mockMultiRoundTrajectoryEvents, mockRollbackTrajectoryEvents } from "../trajectoryMock";
import { defaultCollapsedForStatus, isLiveStatus, OptimisticTraceBlock, shouldShowOptimisticTrace, TraceBlock } from "./TraceBlock";
import { groupMessagesByTurn, isUnboundActivity, latestRenderedTurnId, turnIsAnchored } from "./turnGroups";
import { reduceTurnEventMeta, type TurnEventMeta } from "./turnEventMeta";

function chat(partial: Partial<ChatMessage> & Pick<ChatMessage, "id" | "role" | "content">): ChatMessage {
  return { createdAt: 0, ...partial } as ChatMessage;
}

describe("TraceBlock 状态与类名", () => {
  it("已完成回合默认收起为回执条（顶部栏设计：终态灰 + 执行完成 + meta 步数时长）", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), mockMultiRoundTrajectoryEvents);
    const turn = trajectoryTurns(state)[0];
    const markup = renderToStaticMarkup(
      <TraceBlock state={state} turn={turn} activities={[]} />
    );
    expect(markup).toContain("trace-block is-terminal is-collapsed");
    expect(markup).toContain('class="th-label"');
    expect(markup).toContain("执行完成");
    expect(markup).toContain(" 步");
    expect(markup).toMatch(/data-turn-id="/);
    expect(markup).toContain('aria-expanded="false"');
    expect(markup).not.toContain("trace-think");
    expect(markup).not.toContain('aria-live="polite"');
  });

  it("进行中回合默认展开：is-active 转圈 + 思考行渲染最新活动文案 + 游标 + aria-live", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), [
      ...mockMultiRoundTrajectoryEvents,
      {
        seq: 9001,
        type: "trajectory.round.started",
        title: "第二轮观察",
        created_at: new Date().toISOString(),
        payload: {
          schema_version: "vit.observable_trajectory.v1",
          trace_node_id: "mock-live-1",
          turn_id: "mock-turn",
          round_id: "mock-round-3",
          node_kind: "observation",
          status: "running",
          summary: "正在观察掩蔽关系"
        }
      }
    ]);
    const turn = trajectoryTurns(state)[0];
    expect(isLiveStatus(turn.status)).toBe(true);
    const activities = [chat({ id: "a1", role: "system", content: "正在复核掩蔽关系…" })];
    const markup = renderToStaticMarkup(
      <TraceBlock state={state} turn={turn} activities={activities} />
    );
    expect(markup).toContain("trace-block is-active is-live");
    expect(markup).not.toContain("is-collapsed");
    expect(markup).toContain("正在处理");
    expect(markup).toContain('aria-live="polite"');
    expect(markup).toContain("正在复核掩蔽关系…");
    expect(markup).toContain("trace-cursor");
  });

  it("等待判断回合：is-waiting 琥珀钟形图标语义", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), mockMultiRoundTrajectoryEvents);
    const turn = { ...trajectoryTurns(state)[0], status: "waiting_for_user" };
    const markup = renderToStaticMarkup(
      <TraceBlock state={state} turn={turn} activities={[]} />
    );
    expect(markup).toContain("trace-block is-waiting is-collapsed");
    expect(markup).toContain("等待你的判断");
  });

  it("节点四态类名：完成 plain / 运行 spinner / 排队 pending / action 变更类蓝节点", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), mockRollbackTrajectoryEvents);
    const turn = trajectoryTurns(state)[0];
    const markup = renderToStaticMarkup(
      <TraceBlock state={state} turn={turn} activities={[]} />
    );
    expect(markup).toContain("trace-step is-plain");
    expect(markup).toContain("trace-act");
  });

  it("完全档下 action 节点带「完全档 · 直接执行」标签，普通档不带", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), mockMultiRoundTrajectoryEvents);
    const turn = trajectoryTurns(state)[0];
    const hasActionNode = turn.nodeIds.some((id) => state.nodes[id]?.kind === "action");
    const full = renderToStaticMarkup(
      <TraceBlock state={state} turn={turn} activities={[]} authorityMode="full_project_access" />
    );
    const manual = renderToStaticMarkup(
      <TraceBlock state={state} turn={turn} activities={[]} authorityMode="manual_confirmation" />
    );
    if (hasActionNode) {
      expect(full).toContain("完全档 · 直接执行");
      expect(manual).not.toContain("完全档 · 直接执行");
    }
    expect(full).toContain("trace-step");
  });
});

describe("GUI-T6 轨迹只放轨迹：任务详情不进轨迹块", () => {
  it("轨迹块不含「任务详情」小节、meta 无 Task id（规划内容由输入框上方 PlanBar 承载）", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), mockMultiRoundTrajectoryEvents);
    const turn = trajectoryTurns(state)[0];
    const markup = renderToStaticMarkup(
      <TraceBlock state={state} turn={turn} activities={[]} />
    );
    expect(markup).not.toContain('aria-label="任务详情"');
    expect(markup).not.toContain("任务详情");
    expect(markup).not.toContain("Task mock-task");
    expect(markup).not.toContain("trace-task");
    expect(markup).not.toContain("Run 的 invocation 切片");
    expect(markup).not.toContain("状态变更记录");
    // 回执仍完整：状态 + 步数/耗时
    expect(markup).toContain("执行完成");
    expect(markup).toContain(" 步");
  });
});

describe("收起语义", () => {
  it("live 状态不收起；完成/等待/失败默认收起", () => {
    expect(defaultCollapsedForStatus("running")).toBe(false);
    expect(defaultCollapsedForStatus("pending")).toBe(false);
    expect(defaultCollapsedForStatus("completed")).toBe(true);
    expect(defaultCollapsedForStatus("waiting_for_user")).toBe(true);
    expect(defaultCollapsedForStatus("failed")).toBe(true);
    expect(defaultCollapsedForStatus("stopped")).toBe(true);
  });
});

describe("乐观占位条（GUI-F2）", () => {
  const optimisticMessages = [chat({ id: "u1", role: "user", content: "帮我压一下人声" })];

  function messages(...partial: Array<Partial<ChatMessage> & Pick<ChatMessage, "id" | "role" | "content">>): ChatMessage[] {
    return partial.map((item) => chat(item));
  }

  it("出现：发送中 + 无 live 真块 + 流尾是乐观用户消息", () => {
    expect(shouldShowOptimisticTrace({
      isSending: true,
      trajectory: emptyTrajectoryState(),
      messages: messages(...optimisticMessages)
    })).toBe(true);
  });

  it("接管：trajectory 出现 live turn 后占位撤下（真块同帧顶上）", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), mockMultiRoundTrajectoryEvents.slice(0, 1));
    const turn = trajectoryTurns(state)[0];
    expect(turn.status).toMatch(/running|pending/);
    expect(shouldShowOptimisticTrace({
      isSending: true,
      trajectory: state,
      messages: messages(
        { id: "u1", role: "user", content: "帮我压一下人声" },
        { id: "a1", role: "assistant", content: "好的", turn_id: turn.id }
      )
    })).toBe(false);
  });

  it("失败移除：发送结束即撤下占位（错误消息随流进入）", () => {
    expect(shouldShowOptimisticTrace({
      isSending: false,
      trajectory: emptyTrajectoryState(),
      messages: messages(
        { id: "u1", role: "user", content: "帮我压一下人声" },
        { id: "e1", role: "system", content: "发送失败", status: "error" }
      )
    })).toBe(false);
  });

  it("渲染：与真块同容器类 + 正在处理文案 + 呼吸游标 + aria-live", () => {
    const markup = renderToStaticMarkup(<OptimisticTraceBlock />);
    expect(markup).toContain("trace-block is-live is-optimistic");
    expect(markup).toContain("正在处理");
    expect(markup).toContain("trace-cursor");
    expect(markup).toContain('aria-live="polite"');
    expect(markup).not.toContain("is-collapsed");
  });
});

describe("B3 零步终态驻留消灭 + 步数流式增量", () => {
  // 真栈床 R3 取证（20260909_221101 全程 176 次目击）：chat 回合只有 kind=turn
  // 壳节点（turn.started/completed 伴生投影），终局（waiting/completed）后轨迹块
  // 以「0 步 0.0s」驻留全程。壳节点不是步——终态零步 meta 不得声称步数。
  function shellTurnEvents(terminalType: string, terminalStatus: string): AgentEvent[] {
    return [
      { seq: 1, type: "trajectory.turn.started", item_id: "turn:run-b3", status: "running", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "turn:run-b3", turn_id: "run-b3", node_kind: "turn", phase: "framing", status: "running" } },
      { seq: 2, type: terminalType, item_id: "turn:run-b3", status: terminalStatus, payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "turn:run-b3", turn_id: "run-b3", node_kind: "turn", phase: "completed", status: terminalStatus, summary: "提案已进入待确认" } }
    ];
  }

  function stepEvent(seq: number, running: boolean): AgentEvent {
    return {
      seq,
      type: "trajectory.observation.recorded",
      item_id: `trace-b3-${seq}`,
      status: running ? "running" : "completed",
      payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: `trace-b3-${seq}`, turn_id: "run-b3", node_kind: "observation", phase: "observing", status: running ? "running" : "completed", summary: `观察 ${seq}` }
    };
  }

  it("终态壳回合（仅 kind=turn 壳节点）不以「0 步 0.0s」终态驻留", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), shellTurnEvents("trajectory.turn.completed", "waiting_for_user"));
    const turn = trajectoryTurns(state)[0];
    expect(turn.nodeIds.length).toBe(1); // 壳节点在场（块保留），但它不是步
    const markup = renderToStaticMarkup(<TraceBlock state={state} turn={turn} activities={[]} />);
    expect(markup).toContain("等待你的判断");
    expect(markup).not.toContain("0 步");
    expect(markup).not.toContain("0.0s");
  });

  it("终态零步回合带 item 活动足迹时以「N 项活动+真实时长」呈现（M12 证据块不虚称 0 步）", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), shellTurnEvents("trajectory.turn.completed", "waiting_for_user"));
    const turn = trajectoryTurns(state)[0];
    const turnMeta: TurnEventMeta = { turnKind: "", itemActivityCount: 2, itemActivityKeys: ["i1", "i2"], startedAt: 1_000, endedAt: 61_000 };
    const markup = renderToStaticMarkup(<TraceBlock state={state} turn={turn} activities={[]} turnMeta={turnMeta} />);
    expect(markup).toContain("2 项活动");
    expect(markup).toContain("60.0s");
    expect(markup).not.toContain("0 步");
  });

  // CONT-STALL-1 口径钉：驻留等待不得计成执行时长，两者必须分开呈现。
  it("驻留终点在场时工作时长与等待续跑分行呈现（不再把驻留墙钟算成执行时长）", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), shellTurnEvents("trajectory.turn.completed", "waiting_for_user"));
    const turn = trajectoryTurns(state)[0];
    // 真栈形态（goal_5b9cb1a9e48ace5b）：工作 55.0s，其后驻留 203.5s。meta 走
    // 真实归约器，不手搓字段——归约口径一旦回退成「驻留并进 endedAt」，这里
    // 就会重新渲染出 258.5s 的执行时长。
    const turnMeta = reduceTurnEventMeta({}, [
      { seq: 1, type: "turn.started", source_turn_id: "run-b3", created_at: "2026-09-12T22:53:11.745Z" },
      { seq: 2, type: "item.started", source_turn_id: "run-b3", item_id: "i1", logical_message_id: "agent_item:run-b3:i1", created_at: "2026-09-12T22:53:26.993Z" },
      { seq: 3, type: "item.completed", source_turn_id: "run-b3", item_id: "i1", logical_message_id: "agent_item:run-b3:i1", created_at: "2026-09-12T22:53:27.018Z" },
      { seq: 4, type: "item.started", source_turn_id: "run-b3", item_id: "i2", logical_message_id: "agent_item:run-b3:i2", created_at: "2026-09-12T22:53:44.839Z" },
      { seq: 5, type: "turn.completed", source_turn_id: "run-b3", created_at: "2026-09-12T22:54:06.557Z" },
      { seq: 6, type: "trajectory.turn.stopped", source_turn_id: "run-b3", created_at: "2026-09-12T22:57:30.069Z" }
    ])["run-b3"];
    const markup = renderToStaticMarkup(<TraceBlock state={state} turn={turn} activities={[]} turnMeta={turnMeta} />);
    expect(markup).toContain("2 项活动");
    expect(markup).toContain("执行 54.8s");
    expect(markup).toContain("等待续跑 203.5s");
    // 旧口径的反向锁定：258.3s 的驻留墙钟不得作为执行时长出现。
    expect(markup).not.toContain("258.3s");
  });

  it("无驻留段的回合 meta 逐字不变（不多一个词、不虚报等待）", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), shellTurnEvents("trajectory.turn.completed", "waiting_for_user"));
    const turn = trajectoryTurns(state)[0];
    const turnMeta: TurnEventMeta = { turnKind: "", itemActivityCount: 2, itemActivityKeys: ["i1", "i2"], startedAt: 1_000, endedAt: 61_000 };
    const markup = renderToStaticMarkup(<TraceBlock state={state} turn={turn} activities={[]} turnMeta={turnMeta} />);
    expect(markup).toContain("60.0s");
    expect(markup).not.toContain("等待续跑");
    expect(markup).not.toContain("执行 ");
  });

  it("执行中步数流式增量：live 回合有步节点即显「N 步」（此前恒 --，步数只在终局可见）", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), [
      shellTurnEvents("trajectory.turn.completed", "completed")[0],
      stepEvent(3, false),
      stepEvent(4, true)
    ]);
    const turn = trajectoryTurns(state)[0];
    expect(isLiveStatus(turn.status)).toBe(true);
    const markup = renderToStaticMarkup(<TraceBlock state={state} turn={turn} activities={[]} />);
    expect(markup).toContain(">2 步</span>");
    expect(markup).not.toContain("0 步");
  });

  it("步数增量随节点单调：3 个步节点显 3 步（≥1 后不回落显示 0）", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), [
      shellTurnEvents("trajectory.turn.completed", "completed")[0],
      stepEvent(3, false),
      stepEvent(4, false),
      stepEvent(5, false)
    ]);
    const turn = trajectoryTurns(state)[0];
    const markup = renderToStaticMarkup(<TraceBlock state={state} turn={turn} activities={[]} />);
    expect(markup).toContain(">3 步</span>");
  });

  it("live 回合尚无步节点时保持 --，不虚报 0 步", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), [shellTurnEvents("trajectory.turn.completed", "completed")[0]]);
    const turn = trajectoryTurns(state)[0];
    expect(isLiveStatus(turn.status)).toBe(true);
    const markup = renderToStaticMarkup(<TraceBlock state={state} turn={turn} activities={[]} />);
    expect(markup).not.toContain("0 步");
    expect(markup).toContain(">--</span>");
  });

  it("终态有步回合回执语义不变：N 步 + 时长", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), mockMultiRoundTrajectoryEvents);
    const turn = trajectoryTurns(state)[0];
    const markup = renderToStaticMarkup(<TraceBlock state={state} turn={turn} activities={[]} />);
    expect(markup).toContain(" 步");
  });
});

describe("turnGroups 分组", () => {
  it("无 turn_id 的用户消息自成独立组，顺序在回合组之前", () => {
    const groups = groupMessagesByTurn([
      chat({ id: "u1", role: "user", content: "帮我听一下" }),
      chat({ id: "a1", role: "assistant", content: "好", turn_id: "turn-9" }),
      chat({ id: "a2", role: "assistant", content: "结论", turn_id: "turn-9" }),
      chat({ id: "u2", role: "user", content: "再改一轮" }),
    ]);
    expect(groups).toHaveLength(3);
    expect(groups[0].turnId).toBe("");
    expect(groups[0].messages.map((message) => message.id)).toEqual(["u1"]);
    expect(groups[1].turnId).toBe("turn-9");
    expect(groups[1].messages).toHaveLength(2);
    expect(groups[2].turnId).toBe("");
    expect(turnIsAnchored(groups, "turn-9")).toBe(true);
    expect(turnIsAnchored(groups, "turn-x")).toBe(false);
  });

  it("相邻无 turn_id 消息并入同一独立组，避免碎片化", () => {
    const groups = groupMessagesByTurn([
      chat({ id: "u1", role: "user", content: "一" }),
      chat({ id: "s1", role: "system", content: "系统提示" }),
    ]);
    expect(groups).toHaveLength(1);
    expect(groups[0].messages).toHaveLength(2);
  });

  it("活动线过滤：回合内活动不留线，非回合活动保留", () => {
    const bound = chat({ id: "b1", role: "system", content: "回合内", turn_id: "turn-9" });
    const loose = chat({ id: "l1", role: "system", content: "正在上传…" });
    const unknownTurn = chat({ id: "k1", role: "system", content: "未知回合", turn_id: "turn-404" });
    const known = new Set(["turn-9"]);
    expect(isUnboundActivity(bound, known)).toBe(false);
    expect(isUnboundActivity(loose, known)).toBe(true);
    expect(isUnboundActivity(unknownTurn, known)).toBe(true);
  });

  it("任务详情锚定位：chat turn_ 域回复组不入序列，锚定最后一个会渲染块的回合", () => {
    // 真实栈双 id 域：user 消息回填 run_ 域 id（对齐 trajectory turn），assistant 回复带 turn_ 域 id
    const groups = groupMessagesByTurn([
      chat({ id: "u1", role: "user", content: "轨道数量", turn_id: "run_1" }),
      chat({ id: "a1", role: "assistant", content: "1 条", turn_id: "turn_x1" }),
      chat({ id: "u2", role: "user", content: "采样率", turn_id: "run_2" }),
      chat({ id: "a2", role: "assistant", content: "48k", turn_id: "turn_x2" })
    ]);
    const known = new Set(["run_1", "run_2"]);
    // turn_x2 组在消息序最后但不渲染块——锚定位必须仍是 run_2
    expect(latestRenderedTurnId(groups, known, [])).toBe("run_2");
    // 孤儿兜底块在渲染序列尾时锚定孤儿
    expect(latestRenderedTurnId(groups, known, ["run_3"])).toBe("run_3");
    // 空序列不锚定
    expect(latestRenderedTurnId([], new Set(), [])).toBe("");
  });
});

// UI-FOLLOW-1（2026-09-12 用户产品裁定③）：执行轨迹的转圈/排队不得在终局后仍
// 悬置。回合终态是服务端权威事实（turn 家族终局事件），终态回合里仍标着
// running/pending 的步节点是「没等到自己终局」的悬置标记——终局后它们必须
// 降级为静态「未收口」，不得继续转圈。
describe("UI-FOLLOW-1 终局定格：回合终态后不留转圈/排队标记", () => {
  function turnEvents(withTerminal: boolean): AgentEvent[] {
    const base = [
      { seq: 1, type: "trajectory.turn.started", item_id: "turn:run-follow", status: "running", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "turn:run-follow", turn_id: "run-follow", node_kind: "turn", phase: "framing", status: "running" } },
      { seq: 2, type: "trajectory.observation.recorded", item_id: "obs-live", status: "running", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "obs-live", turn_id: "run-follow", node_kind: "observation", phase: "observing", status: "running", summary: "正在观察掩蔽关系" } },
      { seq: 3, type: "trajectory.round.started", item_id: "queue-pending", status: "pending", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "queue-pending", turn_id: "run-follow", node_kind: "decision", phase: "deciding", status: "pending", summary: "等待排队执行" } }
    ];
    if (!withTerminal) {
      return base as AgentEvent[];
    }
    return [...base, { seq: 4, type: "trajectory.turn.completed", item_id: "turn:run-follow", status: "completed", payload: { schema_version: "vit.observable_trajectory.v1", trace_node_id: "turn:run-follow", turn_id: "run-follow", node_kind: "turn", phase: "completed", status: "completed" } }] as AgentEvent[];
  }

  it("终态回合里未收到终局的步：呈现「未收口」静态标记，不再转圈/排队", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), turnEvents(true));
    const turn = trajectoryTurns(state)[0];
    expect(isLiveStatus(turn.status)).toBe(false);
    const markup = renderToStaticMarkup(<TraceBlock state={state} turn={turn} activities={[]} />);
    expect(markup).toContain("trace-block is-terminal is-collapsed");
    expect(markup).toContain("执行完成");
    expect(markup).not.toContain("trace-step is-running");
    expect(markup).not.toContain("trace-step is-pending");
    expect(markup).not.toContain("trace-dur is-live");
    expect(markup).not.toContain("进行中");
    expect(markup).not.toContain("排队中");
    expect(markup).not.toContain("trace-think");
    expect(markup).toContain("trace-step is-unresolved");
    expect(markup).toContain("未收口");
  });

  it("live 回合零回退：步节点照旧转圈/排队（终态定格不误伤进行中回合）", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), turnEvents(false));
    const turn = trajectoryTurns(state)[0];
    expect(isLiveStatus(turn.status)).toBe(true);
    const markup = renderToStaticMarkup(<TraceBlock state={state} turn={turn} activities={[]} />);
    expect(markup).toContain("trace-step is-running");
    expect(markup).toContain("trace-step is-pending");
    expect(markup).toContain("trace-dur is-live");
    expect(markup).toContain("进行中");
    expect(markup).toContain("排队中");
  });
});

