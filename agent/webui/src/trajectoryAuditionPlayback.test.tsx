import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { emptyAuditionState, reduceAuditionEvents, type AuditionSession } from "./audition";
import { emptyTrajectoryState } from "./trajectory";
import type { AgentEvent } from "./types";
import { AuditionJudgeCard, auditionPlaybackStatus } from "./trajectory/TrajectoryAuditionPanel";

// AUDITION-PLAY-1：select 之后内核预览平面自己出声（与工程 transport 解耦）。
// 卡面必须显示「试听中 · 候选 X」，用户不再需要猜自己点的候选到底响没响。

function selectedEvent(seq: number, status: string, activeCandidateID: string, extra: Record<string, unknown> = {}): AgentEvent {
  return {
    seq,
    type: "audition.selected",
    item_id: "audition-1",
    conversation_id: "conversation-1",
    payload: {
      schema_version: "vit.kernel_audition.v1",
      session: {
        session_id: "audition-1",
        conversation_id: "conversation-1",
        status,
        active_candidate_id: activeCandidateID,
        candidates: [
          { id: "candidate-a", label: "A", status: "ready", preview_ref: "audio-buffer://s/candidate-a:1ch@48000Hz" },
          { id: "candidate-b", label: "B", status: "ready", preview_ref: "audio-buffer://s/candidate-b:1ch@48000Hz" }
        ],
        ...extra
      }
    }
  };
}

function sessionFromEvents(events: AgentEvent[]): AuditionSession {
  const state = reduceAuditionEvents(emptyAuditionState(), events);
  const session = Object.values(state.sessions)[0];
  if (!session) throw new Error("fixture must produce one audition session");
  return session;
}

function renderCard(session: AuditionSession): string {
  return renderToStaticMarkup(
    <AuditionJudgeCard trajectory={emptyTrajectoryState()} session={session} busySessionID="" onSelect={async () => {}} onStop={async () => {}} onSubmitJudgment={async () => {}} />
  );
}

describe("A/B 试听中状态提示（AUDITION-PLAY-1）", () => {
  it("playing 会话报出当前候选", () => {
    const session = sessionFromEvents([selectedEvent(1, "playing", "candidate-b")]);
    expect(auditionPlaybackStatus(session)).toBe("试听中 · 候选 B");
  });

  it("ready 且有选中候选时报待播放，未选中时不报状态", () => {
    const ready = sessionFromEvents([selectedEvent(1, "ready", "candidate-a")]);
    expect(auditionPlaybackStatus(ready)).toBe("已选候选 A · 待播放");
    const fresh = sessionFromEvents([selectedEvent(1, "ready", "")]);
    expect(auditionPlaybackStatus(fresh)).toBe("");
  });

  it("playing 卡面渲染「试听中」标签与无障碍活区", () => {
    const markup = renderCard(sessionFromEvents([selectedEvent(1, "playing", "candidate-a")]));
    expect(markup).toContain("试听中 · 候选 A");
    expect(markup).toContain('data-audition-state="playing"');
    expect(markup).toContain('aria-live="polite"');
  });

  it("ready 卡面不谎报试听中，但活区仍声明当前选择", () => {
    const markup = renderCard(sessionFromEvents([selectedEvent(1, "ready", "candidate-b")]));
    expect(markup).not.toContain("试听中");
    expect(markup).toContain("已选候选 B · 待播放");
  });

  it("盲态 playing 只出现标签，不出现改动前后措辞", () => {
    const session = sessionFromEvents([selectedEvent(1, "playing", "candidate-a", { blind: true })]);
    const status = auditionPlaybackStatus(session);
    expect(status).toBe("试听中 · 候选 A");
    const markup = renderCard(session);
    expect(markup).toContain("试听中 · 候选 A");
    expect(markup).not.toContain("改动前");
    expect(markup).not.toContain("改动后");
  });
});
