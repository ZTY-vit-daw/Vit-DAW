import { useCallback, useEffect, useRef, useState } from "react";
import { fetchUIState, invokeAgent } from "../lib/api";
import { ScoreWorkbenchPanel } from "./ScoreWorkbenchPanel";
import { createAgentScoreBridge, type ScoreKernelBridgeFactory } from "./kernel";
import { defaultScoreModel } from "./model";
import { followTargetFor, scoreCatalog, scoreTargetKey, targetFor, targetInCatalog, type ScoreCatalog, type ScoreSessionState, type ScoreTarget } from "./binding";
import { FOLLOW_CONFIRM_POLLS, guiSelectedClipID, guiSelectedTrackID, initialFollowState, markSelect, nextFollow, type FollowState } from "./selection";
import type { AgentUIState, JsonRecord } from "../types";

type Choice = { projectKey: string; trackID: string; target: ScoreTarget | null };
const EMPTY_SESSION: ScoreSessionState = { dirty: false, busy: false, loaded: false };

// 397: selection belongs to this workbench. 398: GUI selection following is the
// edge-triggered follow machine in selection.ts (change-follows-once semantics;
// dirty pending targets and busy sessions are never bypassed by the poller).
export function ScoreTrackWorkbench({ kernelFactory = createAgentScoreBridge }: { kernelFactory?: ScoreKernelBridgeFactory }) {
  const [catalog, setCatalog] = useState<ScoreCatalog | null>(null);
  const [choice, setChoice] = useState<Choice | null>(null);
  const [pending, setPending] = useState<Choice | null>(null);
  const [session, setSession] = useState(EMPTY_SESSION);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [guiFollow, setGuiFollow] = useState<"" | "pending" | "confirmed">("");
  const mounted = useRef(true);
  const request = useRef(0);
  const received = useRef(0);
  const follow = useRef<FollowState>(initialFollowState());
  const uiStateRef = useRef<AgentUIState | null>(null);
  const catalogRef = useRef<ScoreCatalog | null>(null);
  const choiceRef = useRef<Choice | null>(null);
  const sessionRef = useRef<ScoreSessionState>(session);
  const chooseRef = useRef<(next: Choice) => void>(() => {});
  const followNoticeShown = useRef(false);

  // 反向链路：谱面选轨 commit 后经 /agent/invoke 复用 harness 的 select_track
  //（RiskDirect，ui_action=focus_track）。回执 track_id 核对 + GUI 状态回声确认。
  const selectTrack = useCallback(async (trackID: string, projectKey: string) => {
    try {
      const response = await invokeAgent({ tool: "select_track", args: { track_id: trackID }, source: "ask_vit_webui_score" });
      const result = (response.result ?? {}) as JsonRecord;
      if ((response.status ?? "") !== "ok") throw new Error(response.error || `invoke 状态 ${response.status || "unknown"}`);
      const echoed = typeof result.track_id === "string" || typeof result.track_id === "number" ? String(result.track_id).trim() : "";
      if (echoed && echoed !== trackID) throw new Error(`select_track 回执目标不符（${echoed} ≠ ${trackID}）`);
      if (catalogRef.current?.projectKey !== projectKey) return; // 提交期间工程已切：旧目标不再登记（跨工程清失效引用）
      follow.current = markSelect(follow.current, trackID, projectKey);
      setGuiFollow("pending");
    } catch (failure) {
      setGuiFollow("");
      setNotice(`GUI 选轨未同步：${failure instanceof Error ? failure.message : String(failure)}`);
    }
  }, []);

  // GUI 选轨边沿评估：只在被接受的最新回包上跑（receipt guard 之内）。
  const evaluateFollow = useCallback((next: ScoreCatalog) => {
    const raw = uiStateRef.current;
    if (!raw) return;
    const outcome = nextFollow(follow.current, {
      projectKey: next.projectKey,
      guiTrackID: guiSelectedTrackID(raw),
      guiClipID: guiSelectedClipID(raw),
      choiceProjectKey: choiceRef.current?.projectKey ?? "",
      choiceTrackID: choiceRef.current?.trackID ?? "",
      busy: sessionRef.current.busy
    });
    follow.current = outcome.state;
    if (outcome.state.awaitConfirm && outcome.state.awaitPolls >= FOLLOW_CONFIRM_POLLS && !followNoticeShown.current) {
      followNoticeShown.current = true;
      setNotice(`GUI 选轨未同步：内核状态连续 ${FOLLOW_CONFIRM_POLLS} 次未确认所选轨道，可在轨道列表重试`);
    }
    if (!outcome.state.awaitConfirm) followNoticeShown.current = false;
    if (outcome.decision.kind === "confirm") { setGuiFollow("confirmed"); return; }
    if (outcome.decision.kind !== "follow") return;
    const followed = outcome.decision;
    if (!next.tracks.some((track) => track.trackID === followed.trackID)) return;
    chooseRef.current({ projectKey: next.projectKey, trackID: followed.trackID, target: followTargetFor(next, followed.trackID, followed.clipID) });
    setGuiFollow("");
  }, []);

  // Poll: identity/existence refresh + 398 GUI-follow edge evaluation; it never
  // auto-applies, never bypasses the dirty/busy guards, and waits for itself.
  // validateTarget-driven reads pass { evaluateFollow: false }: a mid-load
  // identity recheck must never consume a GUI edge (the next poll does that
  // under the busy/defer guards).
  const readCatalog = useCallback(async (options?: { evaluateFollow?: boolean }) => {
    const id = ++request.current;
    try {
      const raw = await fetchUIState();
      const next = scoreCatalog(raw);
      if (id < received.current) throw new Error("轨道状态已更新，请重试");
      if (mounted.current && id >= received.current) {
        received.current = id;
        uiStateRef.current = raw;
        setCatalog(next);
        setError("");
        if (options?.evaluateFollow !== false) evaluateFollow(next);
      }
      return next;
    } catch (failure) {
      if (mounted.current && id >= received.current) {
        received.current = id;
        setError(failure instanceof Error ? failure.message : "轨道读取失败，请刷新重试");
      }
      throw failure;
    }
  }, [evaluateFollow]);

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

  const validateTarget = useCallback(async (target: ScoreTarget) => targetInCatalog(await readCatalog({ evaluateFollow: false }), target), [readCatalog]);
  const selectedTrack = catalog?.projectKey === choice?.projectKey ? catalog?.tracks.find((track) => track.trackID === choice?.trackID) : undefined;
  const target = choice?.target ?? null;
  const valid = !error && targetInCatalog(catalog, target);
  const choose = (next: Choice) => {
    if (session.busy) { setNotice("正在装载或写入，请稍后切换"); return; }
    if (next.projectKey === choice?.projectKey && next.trackID === choice?.trackID && scoreTargetKey(next.target) === scoreTargetKey(target)) return;
    if (session.dirty) { setPending(next); return; }
    commitChoice(next);
  };
  // 398：选择真正落地（显式点击或 GUI 跟随通过全部守卫后）才回写 GUI 选轨。
  const commitChoice = (next: Choice) => {
    setChoice(next); setSession(EMPTY_SESSION); setNotice("");
    void selectTrack(next.trackID, next.projectKey);
  };
  choiceRef.current = choice;
  sessionRef.current = session;
  catalogRef.current = catalog;
  chooseRef.current = choose;
  const emptyText = selectedTrack
    ? selectedTrack.unknownClipCount ? "片段类型不可得，无法安全编辑；请刷新轨道"
      : selectedTrack.otherClipCount ? "该轨道只有音频或其他片段，请选择含 MIDI 片段的轨道"
      : "该轨道还没有 MIDI 片段"
    : "选择左侧轨道，开始编辑 MIDI 片段";

  return (
    <section className="score-track-workbench" data-score-binding-root data-score-project-key={catalog?.projectKey ?? ""} data-score-gui-follow={guiFollow}>
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
            <button type="button" className="score-tool primary" data-score-switch-discard onClick={() => { const next = pending; setPending(null); if (next) commitChoice(next); }}>放弃修改并切换</button>
          </div>
        </div>
      )}
    </section>
  );
}
