import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { ChatMessage } from "../types";
import { emptyTrajectoryState, reduceTrajectoryEvents, trajectoryTurns } from "../trajectory";
import { mockMultiRoundTrajectoryEvents, mockRollbackTrajectoryEvents } from "../trajectoryMock";
import { defaultCollapsedForStatus, isLiveStatus, OptimisticTraceBlock, shouldShowOptimisticTrace, TraceBlock } from "./TraceBlock";
import { groupMessagesByTurn, isUnboundActivity, turnIsAnchored } from "./turnGroups";

function chat(partial: Partial<ChatMessage> & Pick<ChatMessage, "id" | "role" | "content">): ChatMessage {
  return { createdAt: 0, ...partial } as ChatMessage;
}

describe("TraceBlock 状态与类名", () => {
  it("已完成回合默认收起为回执条（标题含步数与总时长，无思考行）", () => {
    const state = reduceTrajectoryEvents(emptyTrajectoryState(), mockMultiRoundTrajectoryEvents);
    const turn = trajectoryTurns(state)[0];
    const markup = renderToStaticMarkup(
      <TraceBlock state={state} turn={turn} activities={[]} />
    );
    expect(markup).toContain("trace-block is-collapsed");
    expect(markup).toContain("执行完成 · ");
    expect(markup).toContain(" 步");
    expect(markup).toMatch(/data-turn-id="/);
    expect(markup).not.toContain("trace-think");
  });

  it("进行中回合默认展开：思考行渲染最新活动文案 + 游标", () => {
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
    expect(markup).toContain("trace-block is-live");
    expect(markup).not.toContain("is-collapsed");
    expect(markup).toContain("正在复核掩蔽关系…");
    expect(markup).toContain("trace-cursor");
    expect(markup).toContain("进行中");
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
      expect(full).toContain("自主执行");
      expect(full).toContain("完全档 · 直接执行");
      expect(manual).not.toContain("完全档 · 直接执行");
    }
    expect(full).toContain("trace-step");
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
});
