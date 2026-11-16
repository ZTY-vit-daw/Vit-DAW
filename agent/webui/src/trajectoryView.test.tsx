import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { emptyTrajectoryState, reduceTrajectoryEvents, trajectoryNodesForRound, trajectoryRounds, trajectoryTurns } from "./trajectory";
import { mockMultiRoundTrajectoryEvents, mockRollbackTrajectoryEvents } from "./trajectoryMock";
import { RoundContainer, TrajectoryTurnView, TrajectoryView } from "./trajectory/TrajectoryView";

function multiRoundState() {
  return reduceTrajectoryEvents(emptyTrajectoryState(), mockMultiRoundTrajectoryEvents);
}

describe("observable trajectory GUI projection", () => {
  it("renders the Turn header, context, collapsed rounds, and Settlement from mock events", () => {
    const state = multiRoundState();
    const markup = renderToStaticMarkup(<TrajectoryView state={state} title="多轮实验" />);

    expect(markup).toContain("改善主唱清晰度");
    expect(markup).toContain("任务上下文");
    expect(markup).toContain("Settlement / 收口");
    expect(markup).toContain("处理已保留");
    expect(markup).toContain("vit.observable_trajectory.v1");
    expect(markup.match(/class="trajectory-round-header"[^>]*aria-expanded="false"/g)).toHaveLength(2);
    expect(markup).not.toContain("class=\"trajectory-round-body\"");
    expect(markup).not.toContain("执行动态让位");
  });

  it("reveals Materiality and Rollback trace nodes when a Round is expanded", () => {
    const materialityState = multiRoundState();
    const materialityRound = trajectoryRounds(materialityState, "mock-turn")[0];
    const materialityMarkup = renderToStaticMarkup(
      <RoundContainer
        round={materialityRound}
        nodes={trajectoryNodesForRound(materialityState, materialityRound.id)}
        expanded
        onToggle={() => undefined}
      />
    );
    expect(materialityMarkup).toContain("变化量");
    expect(materialityMarkup).toContain("处理量不足");
    expect(materialityMarkup).toContain("查看审计详情");

    const rollbackState = reduceTrajectoryEvents(emptyTrajectoryState(), mockRollbackTrajectoryEvents);
    const rollbackRound = trajectoryRounds(rollbackState, "mock-turn")[0];
    const rollbackMarkup = renderToStaticMarkup(
      <RoundContainer
        round={rollbackRound}
        nodes={trajectoryNodesForRound(rollbackState, rollbackRound.id)}
        expanded
        onToggle={() => undefined}
      />
    );
    expect(rollbackMarkup).toContain("回退");
    expect(rollbackMarkup).toContain("副作用过大，已回退");
    expect(rollbackMarkup).toContain("已回退");
  });

  it("keeps every Round collapsed by default, including a running Round", () => {
    const state = multiRoundState();
    const turn = trajectoryTurns(state)[0];
    const markup = renderToStaticMarkup(<TrajectoryTurnView state={state} turn={turn} />);

    expect(markup.match(/class="trajectory-round-header"[^>]*aria-expanded="false"/g)).toHaveLength(2);
    expect(markup).not.toContain("class=\"trajectory-round-body\"");
  });
});
