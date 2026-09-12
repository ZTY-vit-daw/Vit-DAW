import { useState } from "react";
import { auditionCanSelect, auditionSessions, auditionSettlementOutcome, type AuditionCandidate, type AuditionSession, type AuditionState, type AuditionSettlementOutcome } from "../audition";
import type { TrajectoryState } from "../trajectory";
import { trajectoryRounds, trajectoryTurns } from "../trajectory";
import type { AuditionJudgmentPayload } from "../lib/api";
import { TrajectoryView } from "./TrajectoryView";
import "./trajectory.css";

/** A/B 两裁决（定案 2026-09-03）：直选即判定+执行意向，隐含「能听出差别」 */
export function verdictJudgmentPayload(session: AuditionSession, preference: "a" | "b"): AuditionJudgmentPayload {
  return judgmentPayload(session, "yes", preference, "");
}

/** 卡面补充输入：「听不出差别/想折中/另有想法」走 free_text，卡面选项未采用 */
export function supplementJudgmentPayload(session: AuditionSession, freeText: string): AuditionJudgmentPayload {
  return judgmentPayload(session, "unsure", "unsure", freeText.trim());
}

/** 卡面次级按钮「听不出差别」：heard=no 直报（效果不足信号，走既有重校准/终止路径，非终局判词） */
export function noDifferenceJudgmentPayload(session: AuditionSession): AuditionJudgmentPayload {
  return judgmentPayload(session, "no", "unsure", "");
}

/** 判定落账后的解盲文案（B12-1）：只有承载 A/B 动作结果的披露才生成文案，其余结果条沿用既有措辞 */
export function auditionBlindDisclosureText(session: AuditionSession): string {
  const action = text(session.blindDisclosure?.action);
  if (action !== "retain" && action !== "rollback") return "";
  return text(session.blindDisclosure?.summary);
}

function judgmentPayload(session: AuditionSession, heardDifference: "yes" | "no" | "unsure", preference: "a" | "b" | "neither" | "equal" | "unsure", freeText: string): AuditionJudgmentPayload {
  return {
    conversation_id: session.conversationID,
    turn_id: session.turnID,
    round_id: session.roundID,
    audition_session_id: session.id,
    project_revision: session.projectRevision,
    heard_difference: heardDifference,
    preference,
    reason_tags: [],
    free_text: freeText
  };
}

export type AuditionCardTone = "yellow" | "blue" | "gray" | "red";

export interface AuditionCardOutcome {
  tone: AuditionCardTone;
  icon: "undo" | "check" | "cross";
  text: string;
}

/** 沉淀结果条推导：结算节点 > 已记录判断 > 判定模糊 > 底部输入框绕过卡面 */
export function auditionCardOutcome(session: AuditionSession, settlement: AuditionSettlementOutcome, superseded: boolean): AuditionCardOutcome | null {
  const preference = text(session.judgmentEvidence?.preference);
  // 盲态会话的顺序是随机的：结果条必须按解盲披露（物理指派 + 实际动作）说话，
  // 不能再用「A=改动前」的固定文案。
  // 披露只可能在判定落账后存在（后端在动作落地后才生成/发事件/落快照），
  // 故此处无需再按 judgmentRecorded 设闸：即便 WebUI 重载后先收到恢复快照，
  // 结果条也不会退回「A=改动前」的固定文案。
  const disclosure = auditionBlindDisclosureText(session);
  if (disclosure) {
    const action = text(session.blindDisclosure?.action);
    return { tone: action === "rollback" ? "yellow" : "blue", icon: action === "rollback" ? "undo" : "check", text: disclosure };
  }
  if (settlement === "rolled_back") return { tone: "yellow", icon: "undo", text: "已裁决 · A 更好 → 已回滚到改动前" };
  if (settlement === "improved") return { tone: "blue", icon: "check", text: "已裁决 · B 更好 · 保留改动后" };
  if (session.judgmentRecorded && preference === "a") return { tone: "yellow", icon: "undo", text: "已裁决 · A 更好 → 已回滚到改动前" };
  if (session.judgmentRecorded && preference === "b") return { tone: "blue", icon: "check", text: "已裁决 · B 更好 · 保留改动后" };
  if (session.judgmentRecorded) return { tone: "gray", icon: "undo", text: "已收到你的补充 · 卡面选项未采用" };
  if (settlement === "needs_user_judgment") return { tone: "gray", icon: "undo", text: "判定模糊 · 实验已终止，未执行进一步变更" };
  if (superseded) return { tone: "gray", icon: "undo", text: "卡面选项未采用 · 你在对话中继续了" };
  return null;
}

/**
 * 试听中状态文案（AUDITION-PLAY-1）：内核预览平面在出声时把会话状态置为
 * playing 并回传 active_candidate_id，用户不再需要猜「点了到底响没响」。
 * 盲态只报标签（A/B），绝不出现物理指派措辞。
 */
export function auditionPlaybackStatus(session: AuditionSession): string {
  const tapes = session.candidates.map((candidate, index) => ({ candidate, side: tapeSide(candidate, index) }));
  const side = sideOfCandidate(session.activeCandidateId, tapes);
  const label = side ? side.toUpperCase() : "";
  if (session.status === "playing") {
    const candidate = session.candidates.find((item) => item.id === session.activeCandidateId);
    const name = text(candidate?.label);
    const shown = label && name && name.toUpperCase() !== label ? `候选 ${label} · ${name}` : label ? `候选 ${label}` : "当前候选";
    return `试听中 · ${shown}`;
  }
  if (session.status === "stopped") return "已停止试听";
  if (session.status === "ready" && session.activeCandidateId) {
    return label ? `已选候选 ${label} · 待播放` : "已选候选 · 待播放";
  }
  return "";
}

/** round 徽标（第 N/M 轮），源自 trajectory rounds；无法定位时留空 */
export function auditionRoundBadge(trajectory: TrajectoryState, session: AuditionSession): string {
  if (!session.roundID) return "";
  const rounds = trajectoryRounds(trajectory, session.turnID);
  const index = rounds.findIndex((round) => round.id === session.roundID);
  if (index < 0) return "";
  return `第 ${index + 1}/${rounds.length} 轮`;
}

/** mono 摘要：优先 user_judgment 请求事件携带的 summary，回退本回合 action 节点摘要 */
export function auditionChangeSummary(trajectory: TrajectoryState, session: AuditionSession): string {
  const nodes = Object.values(trajectory.nodes);
  const requested = nodes.find((node) => node.kind === "user_judgment" && text(node.details?.audition_session_id) === session.id && Boolean(text(node.details?.summary)));
  if (requested) return text(requested.details?.summary);
  const actions = nodes
    .filter((node) => node.roundId === session.roundID && node.kind === "action")
    .sort((left, right) => left.seq - right.seq);
  const action = actions[actions.length - 1];
  return action?.summary || action?.title || "";
}

// 磁带装饰波形（无回放进度事件，不做假进度蓝染；条高按模板同款 LCG 确定性生成）
const waveSeeds: Record<"a" | "b", number[]> = {
  a: seededHeights(20177),
  b: seededHeights(40503)
};

function seededHeights(seed: number, count = 34): number[] {
  const heights: number[] = [];
  let x = seed;
  for (let index = 0; index < count; index += 1) {
    x = (x * 48271) % 2147483647;
    heights.push(4 + Math.floor((x / 2147483647) * 15));
  }
  return heights;
}

export function AuditionJudgeCard({
  trajectory,
  session,
  busySessionID = "",
  superseded = false,
  onSelect,
  onStop,
  onSubmitJudgment
}: {
  trajectory: TrajectoryState;
  session: AuditionSession;
  busySessionID?: string;
  /** 免选路径：用户越过卡面在底部输入框继续了对话 → 卡片沉淀灰条（supersedes 语义的 UI 呈现） */
  superseded?: boolean;
  onSelect: (sessionID: string, candidateID: string) => Promise<void>;
  onStop: (sessionID: string) => Promise<void>;
  onSubmitJudgment?: (payload: AuditionJudgmentPayload) => Promise<void>;
}) {
  const [supplement, setSupplement] = useState("");
  const [supplementOpen, setSupplementOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const settlement = auditionSettlementOutcome(trajectory, session);
  const outcome = auditionCardOutcome(session, settlement, superseded);
  const settled = outcome !== null;
  const busy = busySessionID === session.id;
  const pending = session.judgmentRequested && !session.judgmentRecorded;
  // canJudge 守卫整体保留：结算未发生 + 双候选 ready + runtime 已请求判断（表单简化不动门）
  const canJudge = !settled
    && Boolean(onSubmitJudgment)
    && (session.status === "ready" || session.status === "playing" || session.status === "stopped")
    && session.candidates.length === 2
    && session.candidates.every((candidate) => candidate.status === "ready" && Boolean(candidate.previewRef))
    && session.judgmentRequested
    && !session.judgmentRecorded;
  const badge = auditionRoundBadge(trajectory, session);
  const summary = auditionChangeSummary(trajectory, session);
  const tapes = session.candidates
    .map((candidate, index) => ({ candidate, side: tapeSide(candidate, index) }))
    .sort((left, right) => (left.side === right.side ? 0 : left.side === "a" ? -1 : 1));
  const activeSide = sideOfCandidate(session.activeCandidateId, tapes) ?? "a";

  const submit = async (payload: AuditionJudgmentPayload) => {
    if (!canJudge || submitting || busy) return;
    setSubmitting(true);
    try {
      await onSubmitJudgment?.(payload);
    } finally {
      setSubmitting(false);
    }
  };
  const sendSupplement = async () => {
    const value = supplement.trim();
    if (!value) return;
    await submit(supplementJudgmentPayload(session, value));
  };
  // 「说不清/另有想法」：展开既有 free_text 兜底并聚焦，判定口径不变（heard/preference 取中性 unsure）
  const openSupplement = () => {
    setSupplementOpen(true);
    window.setTimeout(() => {
      document.querySelector<HTMLInputElement>(`[data-audition-session="${session.id}"] input[aria-label="自定义补充输入"]`)?.focus();
    }, 0);
  };
  const selectCandidate = (candidate: AuditionCandidate) => {
    if (settled || busy || !auditionCanSelect(session, candidate)) return;
    void onSelect(session.id, candidate.id);
  };
  const playing = session.status === "playing";
  const playbackStatus = auditionPlaybackStatus(session);
  const isPlayingSide = (side: "a" | "b") => playing && activeSide === side;
  // 盲态：顺序随机，判定前不得出现「改动前/改动后」这类物理指派措辞
  const blind = session.blind && !settled;

  return (
    <section
      className={["card", settled ? "settled" : "", session.error ? "has-error" : ""].filter(Boolean).join(" ")}
      data-audition-session={session.id}
      data-status={session.status}
      aria-label="判定 · A/B 试听"
    >
      <div className="tab">判定 · A/B 试听</div>
      <div className="c-head">
        {badge && <span className="rtag">{badge}</span>}
        {summary && <span className="csum">{summary}</span>}
        {pending && <span className="chip">待判定</span>}
        {blind && <span className="chip">盲测 · 顺序随机</span>}
        {playing && playbackStatus && <span className="chip chip-audition" data-audition-state="playing">{playbackStatus}</span>}
      </div>
      <span className="sr-only" role="status" aria-live="polite" data-audition-playback-status={session.status}>
        {playbackStatus}
      </span>
      {!settled && (
        <>
          <div className="trust">
            <UndoIcon />
            <span>这是试验步，可一键回滚</span>
          </div>
          <div className="ab">
            <div className="ab-head">
              <span className="ab-cap">A/B 快速对比</span>
              <div className="ab-sw" role="group" aria-label="A/B 快速对比切换">
                {(["a", "b"] as const).map((side) => {
                  const tape = tapes.find((item) => item.side === side);
                  const switchable = tape ? !settled && !busy && auditionCanSelect(session, tape.candidate) : false;
                  return (
                    <button key={side} type="button" className={activeSide === side ? "on" : ""} disabled={!switchable} onClick={() => tape && selectCandidate(tape.candidate)}>
                      {side.toUpperCase()}
                    </button>
                  );
                })}
              </div>
            </div>
            {tapes.map(({ candidate, side }) => {
              const selectable = !settled && !busy && auditionCanSelect(session, candidate);
              const active = activeSide === side;
              const isPlaying = isPlayingSide(side);
              const stoppable = !busy && session.status !== "stopped";
              const label = blind
                ? `${side.toUpperCase()} · ${candidate.label || "候选"}`
                : `改动${side === "a" ? "前" : "后"} · ${candidate.label || (side === "a" ? "original" : "processed")}`;
              return (
                <div
                  key={candidate.id || side}
                  className={["tape", active ? "active" : "", isPlaying ? "playing" : ""].filter(Boolean).join(" ")}
                  data-side={side}
                  role={selectable ? "button" : undefined}
                  tabIndex={selectable ? 0 : undefined}
                  aria-label={label}
                  onClick={selectable ? () => selectCandidate(candidate) : undefined}
                  onKeyDown={selectable ? (event) => {
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      selectCandidate(candidate);
                    }
                  } : undefined}
                >
                  <span className="tp-tag">{side.toUpperCase()}</span>
                  <button
                    type="button"
                    className="pbtn"
                    aria-label={`${isPlaying ? "停止" : "播放"} ${label}`}
                    disabled={isPlaying ? !stoppable : !selectable}
                    onClick={(event) => {
                      event.stopPropagation();
                      if (isPlaying) {
                        if (stoppable) void onStop(session.id);
                        return;
                      }
                      selectCandidate(candidate);
                    }}
                  >
                    <PlayIcon />
                    <PauseIcon />
                  </button>
                  <span className="tp-label">{label}</span>
                  <div className="wave" aria-hidden="true">
                    {waveSeeds[side].map((height, index) => <i key={index} style={{ height: `${height}px` }} />)}
                  </div>
                  <span className="tp-time">{tapeTime(candidate, isPlaying)}</span>
                </div>
              );
            })}
          </div>
        </>
      )}
      {canJudge && (
        <div className="vgrid">
          <button type="button" className="vbtn" data-act="pickA" disabled={submitting || busy} onClick={() => { void submit(verdictJudgmentPayload(session, "a")); }}>{blind ? "A 更好" : "A 更好 · 回滚"}</button>
          <button type="button" className="vbtn" data-act="pickB" disabled={submitting || busy} onClick={() => { void submit(verdictJudgmentPayload(session, "b")); }}>{blind ? "B 更好" : "B 更好 · 保留"}</button>
        </div>
      )}
      {canJudge && (
        <div className="vsec">
          <button type="button" className="sbtn" data-act="noDifference" disabled={submitting || busy} onClick={() => { void submit(noDifferenceJudgmentPayload(session)); }}>听不出差别</button>
          <button type="button" className="sbtn" data-act="explain" disabled={submitting || busy} onClick={openSupplement}>说不清 / 另有想法</button>
        </div>
      )}
      {canJudge && (
        <form className={["c-ask", supplementOpen ? "supplement-open" : ""].filter(Boolean).join(" ")} onSubmit={(event) => { event.preventDefault(); void sendSupplement(); }}>
          <input
            type="text"
            value={supplement}
            onChange={(event) => setSupplement(event.target.value)}
            placeholder="听不出差别、想折中或另有想法？直接补充…"
            aria-label="自定义补充输入"
          />
          <button type="submit" className="askb" aria-label="发送补充" disabled={submitting || busy}>
            <SendIcon />
          </button>
        </form>
      )}
      {outcome && (
        <div className={`outcome tone-${outcome.tone}`}>
          {outcome.icon === "undo" ? <UndoIcon /> : outcome.icon === "check" ? <CheckIcon /> : <CrossIcon />}
          <span>{outcome.text}</span>
        </div>
      )}
      {session.error && (
        <div className="outcome tone-red">
          <CrossIcon />
          <span>{session.error}</span>
        </div>
      )}
    </section>
  );
}

/** 判定卡 + 轨迹面板整体（面板挂载在 App.tsx 对话流内时逐卡使用 AuditionJudgeCard） */
export function TrajectoryAuditionPanel({
  trajectory,
  audition,
  busySessionID,
  showTrajectory = true,
  onSelect,
  onStop,
  onSubmitJudgment
}: {
  trajectory: TrajectoryState;
  audition: AuditionState;
  busySessionID: string;
  /** false = 轨迹由对话流内 TraceBlock 呈现（GUI-T2），本面板只保留 audition 部分 */
  showTrajectory?: boolean;
  onSelect: (sessionID: string, candidateID: string) => Promise<void>;
  onStop: (sessionID: string) => Promise<void>;
  onSubmitJudgment?: (payload: AuditionJudgmentPayload) => Promise<void>;
}) {
  const turns = trajectoryTurns(trajectory);
  const sessions = auditionSessions(audition);
  if (turns.length === 0 && sessions.length === 0) return null;
  return (
    <section className="trajectory-live-panel" aria-label="实验轨迹与 A/B 试听">
      {showTrajectory && turns.length > 0 && <TrajectoryView state={trajectory} title="自由态实验轨迹" />}
      {sessions.map((session) => (
        <AuditionJudgeCard key={session.id} trajectory={trajectory} session={session} busySessionID={busySessionID} onSelect={onSelect} onStop={onStop} onSubmitJudgment={onSubmitJudgment} />
      ))}
    </section>
  );
}

function tapeSide(candidate: AuditionCandidate, index: number): "a" | "b" {
  if (candidate.id === "candidate-a") return "a";
  if (candidate.id === "candidate-b") return "b";
  return index === 0 ? "a" : "b";
}

function sideOfCandidate(candidateID: string, tapes: Array<{ candidate: AuditionCandidate; side: "a" | "b" }>): "a" | "b" | undefined {
  if (!candidateID) return undefined;
  return tapes.find((tape) => tape.candidate.id === candidateID)?.side;
}

function tapeTime(candidate: AuditionCandidate, isPlaying: boolean): string {
  if (isPlaying) return "播放中";
  if (candidate.status === "preparing" || !candidate.previewRef) return "准备中";
  if (candidate.status === "ready") return "待播放";
  return candidate.status || "--";
}

function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : value == null ? "" : String(value).trim();
}

function UndoIcon() {
  return (
    <svg viewBox="0 0 14 14" aria-hidden="true">
      <path d="M2.6 5.6h5.1a3.6 3.6 0 1 1-3.5 4.6" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="square" />
      <path d="M5 3.3V7.9L1.4 5.6Z" fill="currentColor" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg viewBox="0 0 12 12" aria-hidden="true">
      <path d="M1.8 6.4 4.8 9.2 10.2 2.8" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />
    </svg>
  );
}

function CrossIcon() {
  return (
    <svg viewBox="0 0 12 12" aria-hidden="true">
      <path d="M2.5 2.5 9.5 9.5M9.5 2.5 2.5 9.5" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />
    </svg>
  );
}

function PlayIcon() {
  return (
    <svg className="ic-play" viewBox="0 0 12 12" aria-hidden="true">
      <path d="M2.5 1.5 10.5 6 2.5 10.5Z" fill="currentColor" />
    </svg>
  );
}

function PauseIcon() {
  return (
    <svg className="ic-pause" viewBox="0 0 12 12" aria-hidden="true">
      <path d="M2.5 1.5h2.6v9H2.5zM6.9 1.5h2.6v9H6.9z" fill="currentColor" />
    </svg>
  );
}

function SendIcon() {
  return (
    <svg viewBox="0 0 12 12" aria-hidden="true">
      <path d="M1.5 1 11 6 1.5 11Z" fill="currentColor" />
    </svg>
  );
}
