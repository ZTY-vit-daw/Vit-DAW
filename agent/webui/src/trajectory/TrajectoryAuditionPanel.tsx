import { auditionCanSelect, auditionSessions, type AuditionState } from "../audition";
import type { TrajectoryState } from "../trajectory";
import { trajectoryTurns } from "../trajectory";
import { TrajectoryView } from "./TrajectoryView";
import "./trajectory.css";

export function TrajectoryAuditionPanel({
  trajectory,
  audition,
  busySessionID,
  onSelect,
  onStop
}: {
  trajectory: TrajectoryState;
  audition: AuditionState;
  busySessionID: string;
  onSelect: (sessionID: string, candidateID: string) => Promise<void>;
  onStop: (sessionID: string) => Promise<void>;
}) {
  const turns = trajectoryTurns(trajectory);
  const sessions = auditionSessions(audition);
  if (turns.length === 0 && sessions.length === 0) return null;
  return (
    <section className="trajectory-live-panel" aria-label="实验轨迹与 A/B 试听">
      {turns.length > 0 && <TrajectoryView state={trajectory} title="自由态实验轨迹" />}
      {sessions.map((session) => (
        <section className="audition-session" key={session.id} data-status={session.status}>
          <div className="audition-session-head">
            <div><span>A/B AUDITION</span><strong>{session.status}</strong></div>
            <button type="button" disabled={busySessionID === session.id || session.status === "stopped"} onClick={() => void onStop(session.id)}>停止</button>
          </div>
          <div className="audition-candidates">
            {session.candidates.map((candidate) => {
              const selectable = auditionCanSelect(session, candidate);
              return (
                <button
                  type="button"
                  key={candidate.id}
                  className={session.activeCandidateId === candidate.id ? "active" : ""}
                  disabled={!selectable || busySessionID === session.id}
                  aria-label={`选择 ${candidate.label}`}
                  onClick={() => void onSelect(session.id, candidate.id)}
                >
                  <strong>{candidate.label}</strong>
                  <span>{candidate.status || "preparing"}</span>
                </button>
              );
            })}
          </div>
          {session.error && <p className="audition-error">{session.error}</p>}
        </section>
      ))}
    </section>
  );
}
