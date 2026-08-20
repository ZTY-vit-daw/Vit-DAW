import { useState } from "react";
import { auditionCanInspect, auditionCanSelect, auditionJudgmentPrefers, auditionSessions, type AuditionState, type HeardDifference, type JudgmentPreference } from "../audition";
import type { TrajectoryState } from "../trajectory";
import { trajectoryTurns } from "../trajectory";
import { TrajectoryView } from "./TrajectoryView";
import "./trajectory.css";

const reasonOptions = ["更清晰", "更自然", "更有力度", "更稳定", "更少刺耳", "更宽", "其他"];

export function TrajectoryAuditionPanel({
  trajectory,
  audition,
  busySessionID,
  onSelect,
  onStop,
  onSubmitJudgment,
  onInspect,
  onApply
}: {
  trajectory: TrajectoryState;
  audition: AuditionState;
  busySessionID: string;
  onSelect: (sessionID: string, candidateID: string) => Promise<void>;
  onStop: (sessionID: string) => Promise<void>;
  onInspect?: (sessionID: string, candidateID: string) => Promise<void>;
  onApply?: (sessionID: string, candidateID: string, evidenceID: string) => Promise<void>;
  onSubmitJudgment?: (payload: {
    conversation_id: string;
    turn_id: string;
    round_id: string;
    audition_session_id: string;
    project_revision: string;
    heard_difference: HeardDifference;
    preference: JudgmentPreference;
    reason_tags: string[];
    free_text: string;
  }) => Promise<void>;
}) {
  const turns = trajectoryTurns(trajectory);
  const sessions = auditionSessions(audition);
  const [heard, setHeard] = useState<Record<string, HeardDifference>>({});
  const [preference, setPreference] = useState<Record<string, JudgmentPreference>>({});
  const [reasons, setReasons] = useState<Record<string, string[]>>({});
  const [freeText, setFreeText] = useState<Record<string, string>>({});
  const [submitting, setSubmitting] = useState("");
  if (turns.length === 0 && sessions.length === 0) return null;

  return (
    <section className="trajectory-live-panel" aria-label="实验轨迹与 A/B 试听">
      {turns.length > 0 && <TrajectoryView state={trajectory} title="自由态实验轨迹" />}
      {sessions.map((session) => {
        const heardValue = heard[session.id] ?? "";
        const preferenceValue = preference[session.id] ?? (heardValue === "yes" ? "" : heardValue ? "unsure" : "");
        const canJudge = Boolean(onSubmitJudgment) && (session.status === "ready" || session.status === "playing" || session.status === "stopped") && session.candidates.length === 2 && session.candidates.every((candidate) => candidate.status === "ready" && Boolean(candidate.previewRef)) && session.judgmentRequested && !session.judgmentRecorded;
        const submitLabel = "记录判断（不会自动采用）";
        const submit = async () => {
          if (!heardValue || !preferenceValue || !canJudge) return;
          setSubmitting(session.id);
          try {
            await onSubmitJudgment?.({
              conversation_id: session.conversationID,
              turn_id: session.turnID,
              round_id: session.roundID,
              audition_session_id: session.id,
              project_revision: session.projectRevision,
              heard_difference: heardValue,
              preference: preferenceValue,
              reason_tags: reasons[session.id] ?? [],
              free_text: freeText[session.id] ?? ""
            });
          } finally {
            setSubmitting("");
          }
        };
        return (
          <section className="audition-session" key={session.id} data-status={session.status}>
            <div className="audition-session-head">
              <div><span>A/B AUDITION</span><strong>{session.status}</strong></div>
              <button type="button" disabled={busySessionID === session.id || session.status === "stopped"} onClick={() => void onStop(session.id)}>停止</button>
            </div>
            <div className="audition-candidates">
              {session.candidates.map((candidate) => {
                const selectable = auditionCanSelect(session, candidate);
                return (
                  <div className={`audition-candidate-card ${session.inspectedCandidateId === candidate.id ? "inspected" : ""} ${session.adoptedCandidateId === candidate.id ? "adopted" : ""}`} key={candidate.id}>
                    <button type="button" className={session.activeCandidateId === candidate.id ? "active" : ""} disabled={!selectable || busySessionID === session.id} aria-label={`试听 ${candidate.label}`} onClick={() => void onSelect(session.id, candidate.id)}>
                      <strong>{candidate.label}</strong><span>{candidate.status || "preparing"}</span>
                    </button>
                    <div className="audition-candidate-source">
                      <span>{candidate.engineeringSourceKind || (candidate.worktreeRef ? "worktree" : candidate.branchRef ? "branch" : "checkpoint")}</span>
                      <strong>{candidate.worktreeRef || candidate.branchRef || candidate.checkpointRef || candidate.engineeringSourceRef || "-"}</strong>
                      {candidate.ownerAgentID && <small>{candidate.ownerAgentID}{candidate.reservationID ? ` · ${candidate.reservationID}` : ""}</small>}
                    </div>
                    <div className="audition-candidate-actions">
                      <button type="button" disabled={!auditionCanInspect(candidate) || busySessionID === session.id} onClick={() => void onInspect?.(session.id, candidate.id)}>查看</button>
                      <button type="button" disabled={!auditionJudgmentPrefers(session, candidate.id) || busySessionID === session.id || session.adoptionStatus === "applied"} onClick={() => void onApply?.(session.id, candidate.id, String(session.judgmentEvidence?.id ?? ""))}>采用</button>
                    </div>
                  </div>
                );
              })}
            </div>
            {canJudge && (
              <form className="audition-judgment" onSubmit={(event) => { event.preventDefault(); void submit(); }}>
                <fieldset>
                  <legend>你能听出 A 和 B 的区别吗？</legend>
                  <div className="audition-choice-row">
                    {([["yes", "能"], ["no", "不能"], ["unsure", "不确定"]] as const).map(([value, label]) => (
                      <label key={value}><input type="radio" name={`heard-${session.id}`} value={value} checked={heardValue === value} onChange={() => { setHeard((current) => ({ ...current, [session.id]: value })); if (value !== "yes") setPreference((current) => ({ ...current, [session.id]: "unsure" })); }} />{label}</label>
                    ))}
                  </div>
                </fieldset>
                <fieldset>
                  <legend>如果能听出，你更偏好哪个？</legend>
                  <div className="audition-choice-row">
                    {([["a", "A"], ["b", "B"], ["equal", "都差不多"], ["neither", "都不喜欢"], ["unsure", "不确定"]] as const).map(([value, label]) => (
                      <label key={value}><input type="radio" name={`preference-${session.id}`} value={value} checked={preferenceValue === value} disabled={heardValue !== "yes"} onChange={() => setPreference((current) => ({ ...current, [session.id]: value }))} />{label}</label>
                    ))}
                  </div>
                </fieldset>
                <fieldset>
                  <legend>可选原因</legend>
                  <div className="audition-choice-row audition-reasons">
                    {reasonOptions.map((reason) => { const checked = (reasons[session.id] ?? []).includes(reason); return <label key={reason}><input type="checkbox" checked={checked} onChange={() => setReasons((current) => ({ ...current, [session.id]: checked ? (current[session.id] ?? []).filter((item) => item !== reason) : [...(current[session.id] ?? []), reason] }))} />{reason}</label>; })}
                  </div>
                </fieldset>
                <textarea value={freeText[session.id] ?? ""} onChange={(event) => setFreeText((current) => ({ ...current, [session.id]: event.target.value }))} placeholder="补充说明（可选）" rows={2} />
                <button className="audition-submit" type="submit" disabled={!heardValue || !preferenceValue || submitting === session.id}>{submitting === session.id ? "记录中…" : submitLabel}</button>
              </form>
            )}
            {session.judgmentRecorded && <div className="audition-recorded">已记录用户判断证据{session.judgmentEvidence ? ` · ${String(session.judgmentEvidence.preference ?? "")}` : ""}。请使用“采用”明确改变工程。</div>}
            {session.inspectedCandidateId && <div className="audition-inspected">当前查看：{session.inspectedCandidateId === "candidate-a" ? "A" : "B"}（已切换 Active Project Plane）</div>}
            {session.adoptedCandidateId && <div className="audition-adopted">当前采用：{session.adoptedCandidateId === "candidate-a" ? "A" : "B"} · {session.adoptionStatus}</div>}
            {session.error && <p className="audition-error">{session.error}</p>}
          </section>
        );
      })}
    </section>
  );
}
