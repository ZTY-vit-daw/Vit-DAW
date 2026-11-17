import { useCallback, useEffect, useRef, useState } from "react";
import { fetchUIState } from "../lib/api";
import { ScoreWorkbenchPanel } from "./ScoreWorkbenchPanel";
import { createAgentScoreBridge, type ScoreKernelBridgeFactory } from "./kernel";
import { defaultScoreModel } from "./model";
import { scoreCatalog, scoreTargetKey, targetFor, targetInCatalog, type ScoreCatalog, type ScoreSessionState, type ScoreTarget } from "./binding";

type Choice = { projectKey: string; trackID: string; target: ScoreTarget | null };
const EMPTY_SESSION: ScoreSessionState = { dirty: false, busy: false, loaded: false };

// 397: selection belongs to this workbench. GUI selection following is 398.
export function ScoreTrackWorkbench({ kernelFactory = createAgentScoreBridge }: { kernelFactory?: ScoreKernelBridgeFactory }) {
  const [catalog, setCatalog] = useState<ScoreCatalog | null>(null);
  const [choice, setChoice] = useState<Choice | null>(null);
  const [pending, setPending] = useState<Choice | null>(null);
  const [session, setSession] = useState(EMPTY_SESSION);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const mounted = useRef(true);
  const request = useRef(0);
  const received = useRef(0);

  // A refresh checks identity and existence only; it never auto-applies or
  // follows selected_track from Godot. The next poll waits for this one to end.
  const readCatalog = useCallback(async () => {
    const id = ++request.current;
    try {
      const next = scoreCatalog(await fetchUIState());
      if (id < received.current) throw new Error("轨道状态已更新，请重试");
      if (mounted.current && id >= received.current) {
        received.current = id;
        setCatalog(next);
        setError("");
      }
      return next;
    } catch (failure) {
      if (mounted.current && id >= received.current) {
        received.current = id;
        setError(failure instanceof Error ? failure.message : "轨道读取失败，请刷新重试");
      }
      throw failure;
    }
  }, []);

  useEffect(() => {
    mounted.current = true;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    const refresh = async () => {
      try { await readCatalog(); } catch { /* visible error, fail closed */ }
      if (!stopped) timer = setTimeout(() => void refresh(), 1000);
    };
    void refresh();
    return () => { stopped = true; mounted.current = false; clearTimeout(timer); };
  }, [readCatalog]);

  const validateTarget = useCallback(async (target: ScoreTarget) => targetInCatalog(await readCatalog(), target), [readCatalog]);
  const selectedTrack = catalog?.projectKey === choice?.projectKey ? catalog?.tracks.find((track) => track.trackID === choice?.trackID) : undefined;
  const target = choice?.target ?? null;
  const valid = !error && targetInCatalog(catalog, target);
  const choose = (next: Choice) => {
    if (session.busy) { setNotice("正在装载或写入，请稍后切换"); return; }
    if (next.projectKey === choice?.projectKey && next.trackID === choice?.trackID && scoreTargetKey(next.target) === scoreTargetKey(target)) return;
    if (session.dirty) { setPending(next); return; }
    setChoice(next); setSession(EMPTY_SESSION); setNotice("");
  };
  const emptyText = selectedTrack
    ? selectedTrack.unknownClipCount ? "片段类型不可得，无法安全编辑；请刷新轨道"
      : selectedTrack.otherClipCount ? "该轨道只有音频或其他片段，请选择含 MIDI 片段的轨道"
      : "该轨道还没有 MIDI 片段"
    : "选择左侧轨道，开始编辑 MIDI 片段";

  return (
    <section className="score-track-workbench" data-score-binding-root data-score-project-key={catalog?.projectKey ?? ""}>
      <aside className="score-track-list" aria-label="谱面轨道">
        <div className="score-track-list-heading"><strong>轨道</strong><button type="button" className="score-tool secondary" data-score-refresh onClick={() => void readCatalog().catch(() => {})}>刷新</button></div>
        {catalog?.tracks.map((track) => (
          <button type="button" key={track.trackID} className="score-track-item" data-score-track-id={track.trackID}
            aria-pressed={selectedTrack?.trackID === track.trackID} disabled={session.busy || !!pending}
            onClick={() => choose({ projectKey: catalog.projectKey, trackID: track.trackID, target: track.clips.length === 1 ? targetFor(catalog, track, track.clips[0].clipID) : null })}>
            <span>{track.name}</span><small>{track.clips.length ? `${track.clips.length} 个 MIDI 片段` : track.unknownClipCount ? "片段类型未知" : track.otherClipCount ? "无 MIDI 片段" : "空轨道"}</small>
          </button>
        ))}
        {catalog && !catalog.tracks.length && <p className="score-binding-message">工程中还没有轨道</p>}
      </aside>
      <div className="score-track-editor" inert={!!pending}>
        <header className="score-binding-heading" data-score-editing-target data-score-track-id={target?.trackID ?? ""} data-score-bound-clip-id={target?.clipID ?? ""}>
          <small>{target ? "正在编辑" : "谱面编辑"}</small>
          <strong>{target ? `${target.trackName} · ${target.clipName}` : selectedTrack?.name ?? "尚未选择轨道"}</strong>
          {target && <span>{session.busy ? "处理中…" : !valid ? "目标已失效，写入已锁定" : session.dirty ? "有未写入的修改" : session.loaded ? "已装载" : "等待装载"}</span>}
        </header>
        {error && <p role="alert" className="score-binding-message">轨道读取失败：{error}。写入已锁定，请刷新重试。</p>}
        {notice && <p role="status" className="score-binding-message">{notice}</p>}
        {target && !valid && !error && <p role="alert" className="score-binding-message">工程已切换或片段已移除，保留的谱面仅供查看。请选择当前工程的 MIDI 片段。</p>}
        {selectedTrack && selectedTrack.clips.length > 0 && (
          <label className="score-clip-picker">编辑片段
            <select data-score-clip-select aria-label="编辑片段" value={valid ? target?.clipID ?? "" : ""} disabled={session.busy}
              onChange={(event) => choose({ projectKey: catalog!.projectKey, trackID: selectedTrack.trackID, target: targetFor(catalog!, selectedTrack, event.currentTarget.value) })}>
              <option value="">选择 MIDI 片段</option>
              {selectedTrack.clips.map((clip) => <option key={clip.clipID} value={clip.clipID}>{clip.name}</option>)}
            </select>
          </label>
        )}
        {target ? <ScoreWorkbenchPanel key={scoreTargetKey(target)} initialModel={defaultScoreModel()} kernelFactory={kernelFactory}
          target={target} targetValid={valid} validateTarget={validateTarget} onSessionChange={setSession} />
          : <p className="score-binding-empty" data-score-binding-empty>{selectedTrack?.clips.length ? "请选择要编辑的 MIDI 片段" : emptyText}</p>}
      </div>
      {pending && (
        <div className="score-binding-switch" role="dialog" aria-modal="true" aria-label="切换编辑目标" data-score-switch-confirm>
          <p>「{target?.trackName} · {target?.clipName}」有未写入的修改。</p>
          <p>切换到「{pending.target ? `${pending.target.trackName} · ${pending.target.clipName}` : catalog?.tracks.find((track) => track.trackID === pending.trackID)?.name ?? "所选轨道"}」前，需要放弃这些修改。</p>
          <div className="score-confirm-buttons">
            <button type="button" className="score-tool secondary" data-score-switch-cancel autoFocus onClick={() => setPending(null)}>继续编辑</button>
            <button type="button" className="score-tool primary" data-score-switch-discard onClick={() => { setChoice(pending); setPending(null); setSession(EMPTY_SESSION); setNotice(""); }}>放弃修改并切换</button>
          </div>
        </div>
      )}
    </section>
  );
}
