// SCORE-WORKBENCH-1 M2：谱面工作台面板（React）。
// 治理粒度：本地编辑缓冲 + 显式 apply 单次确认（主管工作假设 369-R2；卡面 M2）。
// 所有编辑只改本地 buffer.working（edit.ts 确定性操作）；「写入内核」按钮弹
// 确认摘要（diff 计数+音符清单），确认后单次 replace_region 整段覆盖（A5）。
// 交互全集：点击放符/时值切换/附点开关/休止插入/延音建立解除/删除/拖动（16 分网格）。
// data-score-* 属性=渲染面烟测锚点（DG3 门槛 1/2 用）。

import { Component, useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { deriveNotation } from "./derive";
import {
  bufferDirty,
  bufferDiffSummary,
  buildApplyPatch,
  createEditBuffer,
  deleteNote,
  dragNote,
  insertRest,
  placeNote,
  setDuration,
  tieEstablish,
  tieRelease,
  toggleDot,
  type EditBuffer
} from "./edit";
import { GRID_PER_BEAT, serializeModel, type ScoreModel } from "./model";
import { hitTest, renderScoreInto, type RenderResult } from "./render";
import { cursorPosition, playheadClipBeats } from "./cursor";
import { createNoKernelBridge, type ScoreClipSnapshot, type ScoreKernelBridgeFactory } from "./kernel";
import { scoreTargetKey, type ScoreTarget, type ScoreSessionState } from "./binding";
import { fetchUIState } from "../lib/api";
import "./scoreworkbench.css";

function firstFinite(...values: unknown[]): number | null {
  for (const value of values) {
    if (typeof value === "number" && Number.isFinite(value)) {
      return value;
    }
  }
  return null;
}

export type ScoreDuration = 1 | 2 | 3 | 4;

export const DURATION_LABELS: Record<ScoreDuration, string> = {
  1: "十六分",
  2: "八分",
  3: "附点八分",
  4: "四分"
};

const PITCH_OPTIONS = [
  { value: 79, label: "G5" },
  { value: 76, label: "E5" },
  { value: 74, label: "D5" },
  { value: 72, label: "C5" },
  { value: 71, label: "B4" },
  { value: 67, label: "G4" },
  { value: 64, label: "E4" },
  { value: 60, label: "C4" }
];

type Selection = { kernelId: string; tick: number } | null;

type ScorePanelProps = {
  initialModel: ScoreModel;
  clipLengthBeats?: number;
  kernelFactory?: ScoreKernelBridgeFactory | null;
  target?: ScoreTarget | null;
  targetValid?: boolean;
  validateTarget?: (target: ScoreTarget) => Promise<boolean>;
  onSessionChange?: (state: ScoreSessionState) => void;
};

// SCORE-ENTRY-REACH-1 (D4) 渲染期错误边界：ModelError/渲染异常不再卸载整面板
//（377 实证：buildApplyPatch useMemo 渲染期抛 grid_misaligned → 无边界 → 整树
// 卸载白板）。降级=错误显形+「重试」（重挂子树清状态）；面板本体逻辑零变化。
export class ScorePanelErrorBoundary extends Component<
  { children: ReactNode },
  { error: Error | null; attempt: number }
> {
  constructor(props: { children: ReactNode }) {
    super(props);
    this.state = { error: null, attempt: 0 };
  }

  static getDerivedStateFromError(error: unknown): { error: Error | null; attempt: number } | null {
    return { error: error instanceof Error ? error : new Error(String(error)), attempt: Date.now() };
  }

  render(): ReactNode {
    if (this.state.error) {
      const message = this.state.error.message || String(this.state.error);
      return (
        <div
          className="score-workbench score-panel-error"
          data-score-error-boundary="true"
          data-score-error-message={message}
        >
          <p className="score-panel-error-title">谱面渲染中止（错误已捕获，面板未卸载）</p>
          <p className="score-panel-error-detail">{message}</p>
          <button
            type="button"
            className="v5-btn v5-btn--secondary"
            data-score-error-retry
            onClick={() => this.setState({ error: null })}
          >
            重试（重挂面板）
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}

export function ScoreWorkbenchPanel(props: ScorePanelProps) {
  return (
    <ScorePanelErrorBoundary>
      <ScorePanelBody {...props} />
    </ScorePanelErrorBoundary>
  );
}

function ScorePanelBody({
  initialModel,
  clipLengthBeats: fallbackClipLengthBeats = 8,
  kernelFactory,
  target = null,
  targetValid = true,
  validateTarget,
  onSessionChange
}: ScorePanelProps) {
  const targetKey = scoreTargetKey(target);
  const clipID = target?.clipID ?? "";
  const activeKernel = useMemo(
    () => (kernelFactory && target ? kernelFactory({ clipID: target.clipID, trackID: target.trackID }) : null),
    // Display names do not rebind the editor or discard its working buffer.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [kernelFactory, targetKey]
  );
  const [buffer, setBuffer] = useState<EditBuffer>(() => createEditBuffer(initialModel));
  const [mode, setMode] = useState<"note" | "rest">("note");
  const [duration, setDurationValue] = useState<ScoreDuration>(2);
  const [pitch, setPitch] = useState(72);
  const [selection, setSelection] = useState<Selection>(null);
  const [status, setStatus] = useState("就绪");
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [applyState, setApplyState] = useState<"idle" | "busy" | "saved" | "error">("idle");
  const [snapshot, setSnapshot] = useState<ScoreClipSnapshot | null>(null);
  const [loadedKey, setLoadedKey] = useState("");
  const [loading, setLoading] = useState(false);
  const [retry, setRetry] = useState(0);
  const generation = useRef(0);
  const applying = useRef(false);
  const live = useRef({ targetKey, targetValid });
  live.current = { targetKey, targetValid };
  const ready = !!activeKernel && targetValid && loadedKey === targetKey && !!targetKey;
  const editingLocked = !!kernelFactory && (!ready || loading || applyState === "busy");
  const [clipLengthBeats, setClipLengthBeats] = useState(fallbackClipLengthBeats);
  // M4 随谱游标：playhead 秒轮询（/agent/ui/state ui_context 白名单键）+单 tempo 门内换算。
  const [playheadSeconds, setPlayheadSeconds] = useState<number | null>(null);
  const [transportPlaying, setTransportPlaying] = useState(false);

  // 397: only a successful reload binds the buffer to this explicit target.
  // Invalidating a target preserves edits for inspection but revokes all writes.
  useEffect(() => {
    const token = ++generation.current;
    setLoadedKey("");
    setSnapshot(null);
    setConfirmOpen(false);
    setLoading(false);
    applying.current = false;
    setApplyState("idle");
    if (!activeKernel || !target || !targetValid) return;
    if (bufferDirty(buffer)) {
      setStatus("本地修改已保留；请先放弃修改，再重新装载片段");
      return;
    }
    const current = () => generation.current === token && live.current.targetKey === targetKey && live.current.targetValid;
    setLoading(true);
    setStatus("正在装载所选片段…");
    void (async () => {
      try {
        if (validateTarget && !(await validateTarget(target))) throw new Error("工程或片段已变化，请重新选择");
        if (!current()) return;
        const result = await activeKernel.reload();
        if (!current()) return;
        if (result.ok && result.value) {
          if (validateTarget && !(await validateTarget(target))) throw new Error("装载期间工程或片段已变化，请重新选择");
          if (!current()) return;
          setBuffer(createEditBuffer(result.value.model));
          setSnapshot(result.value);
          setLoadedKey(targetKey);
          setApplyState("idle");
          setClipLengthBeats(result.value.clipLengthBeats);
          // D4（SCORE-ENTRY-REACH-1）：raw 网格值不一致=clip 尾被桥面网格化，显形披露。
          const raw = result.value.clipLengthBeatsRaw;
          const gridNote =
            raw !== undefined && Math.abs(raw - result.value.clipLengthBeats) > 1e-9
              ? `，clip 拍长 ${raw.toFixed(4)} 非网格已上取整`
              : "";
          setStatus(`已从内核装载（${result.value.timeSignatureLabel} @ ${result.value.tempoBPM}bpm，clip ${result.value.clipLengthBeats} 拍${gridNote}）`);
        } else {
          setStatus(`内核装载失败：${result.ok ? "空模型" : result.error}`);
        }
      } catch (failure) {
        if (current()) setStatus(`内核装载失败：${failure instanceof Error ? failure.message : String(failure)}`);
      } finally {
        if (current()) setLoading(false);
      }
    })();
    return () => { generation.current++; };
    // target names are labels; identity and validation govern this effect.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeKernel, targetKey, targetValid, validateTarget, retry]);

  const svgHostRef = useRef<HTMLDivElement | null>(null);
  const overlayRef = useRef<HTMLDivElement | null>(null);
  const renderResultRef = useRef<RenderResult | null>(null);
  const dragRef = useRef<{ kernelId: string; lengthTicks: number; grabTick: number; pointerId: number } | null>(null);

  const dirty = bufferDirty(buffer);
  useEffect(() => {
    onSessionChange?.({ dirty, busy: loading || applyState === "busy", loaded: ready });
  }, [dirty, loading, applyState, ready, onSessionChange]);
  const diff = useMemo(() => bufferDiffSummary(buffer), [buffer]);
  const notation = useMemo(() => deriveNotation(buffer.working), [buffer.working]);
  const selectedNote = selection ? buffer.working.notes.find((note) => note.id === selection.kernelId) ?? null : null;
  const selectionChainTicks = useMemo(() => {
    if (!selectedNote) {
      return [];
    }
    return notation.events.filter((event) => event.kernelId === selectedNote.id).map((event) => event.tick);
  }, [notation, selectedNote]);

  // M4：游标轮询（250ms；快照在场才轮询——无 tempo/clip 起点不画游标=单 tempo
  // fail-honest，不静默错位）。读取缝=/agent/ui/state ui_context（服务端白名单
  // playhead/current_playhead/transport_position/is_playing 键族）。
  useEffect(() => {
    if (!snapshot) {
      return;
    }
    let stopped = false;
    const tickPlayhead = async () => {
      try {
        const state = await fetchUIState();
        const context = state.ui_context ?? {};
        const seconds = firstFinite(context.playhead_seconds, context.current_playhead_seconds, context.transport_position_seconds);
        if (!stopped && seconds !== null) {
          setPlayheadSeconds(seconds);
          setTransportPlaying(context.is_playing === true || context.playing === true);
        }
      } catch {
        // 轮询失败静默保留上一位置（面不可达由装载状态显形；不重复报错刷屏）。
      }
    };
    void tickPlayhead();
    const timer = window.setInterval(() => void tickPlayhead(), 250);
    return () => {
      stopped = true;
      window.clearInterval(timer);
    };
  }, [snapshot]);

  // 推导重绘：working 变化即重渲染（369 spike 模式：编辑→推导→重绘全链）。
  // data-score-tick-rows：符号锚点几何可见性（m<measure>:<tick>@<x>,...; 分号分行）——
  // 渲染面烟测/真栈腿据此换算真实指针点击坐标（点击仍走 pointer 事件管线）。
  useEffect(() => {
    const host = svgHostRef.current;
    if (!host) {
      return;
    }
    const width = host.clientWidth && host.clientWidth > 80 ? host.clientWidth : 340;
    const result = renderScoreInto(host, buffer.working, { width });
    renderResultRef.current = result;
    const rows = [...result.tickRows.entries()]
      .map(([measure, row]) => `m${measure}:${row.map(([tick, x]) => `${tick}@${Math.round(x)}`).join(",")}`)
      .join(";");
    host.setAttribute("data-score-tick-rows", rows);
  }, [buffer.working]);

  const localOffset = useCallback((event: { clientX: number; clientY: number }, element: HTMLElement | null) => {
    const rect = element?.getBoundingClientRect();
    const left = rect ? rect.left : 0;
    const top = rect ? rect.top : 0;
    return { x: event.clientX - left, y: event.clientY - top };
  }, []);

  const onOverlayPointerDown = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      if (editingLocked) return;
      const result = renderResultRef.current;
      if (!result || !overlayRef.current) {
        return;
      }
      const { x, y } = localOffset(event, overlayRef.current);
      const hit = hitTest(result, x, y);
      if (!hit) {
        return;
      }
      if (event.button !== 0) {
        return;
      }
      const hitEvent = hit.event;
      if (mode === "note" && hitEvent?.kind === "note" && hitEvent.kernelId) {
        setSelection({ kernelId: hitEvent.kernelId, tick: hit.tick });
        const note = buffer.working.notes.find((item) => item.id === hitEvent.kernelId);
        if (note) {
          dragRef.current = {
            kernelId: note.id,
            lengthTicks: Math.round(note.lengthBeats * GRID_PER_BEAT),
            grabTick: hit.tick,
            pointerId: event.pointerId
          };
          overlayRef.current.setPointerCapture?.(event.pointerId);
          setStatus(`已选中 ${note.id}（拖动移动，松开落格）`);
        }
        return;
      }
      if (mode === "rest") {
        setBuffer((current) => insertRest(current, hit.tick, duration));
        setStatus(`插入休止 @ ${hit.tick / GRID_PER_BEAT} 拍`);
        return;
      }
      setBuffer((current) => placeNote(current, pitch, hit.tick, duration));
      setStatus(`放置音符 @ ${hit.tick / GRID_PER_BEAT} 拍`);
    },
    [buffer.working, duration, localOffset, mode, pitch, editingLocked]
  );

  const onOverlayPointerMove = useCallback((event: React.PointerEvent<HTMLDivElement>) => {
    const drag = dragRef.current;
    if (!drag) {
      return;
    }
    const result = renderResultRef.current;
    if (!result || !overlayRef.current) {
      return;
    }
    const { x } = localOffset(event, overlayRef.current);
    const measure = rowOfTick(result, drag.grabTick);
    const row = result.tickRows.get(measure) ?? [];
    if (row.length === 0) {
      return;
    }
    // 轻量状态：拖动中只更新状态行（落格在 pointer up）。
    const hit = hitTest(result, x, (result.staveBounds.get(measure)?.y ?? 0) + 20);
    if (hit) {
      setStatus(`拖动中 → ${hit.tick / GRID_PER_BEAT} 拍（松开落格）`);
    }
  }, [localOffset]);

  const onOverlayPointerUp = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      const drag = dragRef.current;
      dragRef.current = null;
      if (editingLocked) return;
      const result = renderResultRef.current;
      if (!drag || !result || !overlayRef.current) {
        return;
      }
      const { x } = localOffset(event, overlayRef.current);
      const measure = rowOfTick(result, drag.grabTick);
      const rowY = (result.staveBounds.get(measure)?.y ?? 0) + 20;
      const hit = hitTest(result, x, rowY);
      if (!hit || hit.tick === drag.grabTick) {
        setStatus("拖动取消（未移动）");
        return;
      }
      const deltaTicks = hit.tick - drag.grabTick;
      const note = buffer.working.notes.find((item) => item.id === drag.kernelId);
      if (!note) {
        return;
      }
      const newStart = Math.max(0, Math.round(note.startBeats * GRID_PER_BEAT) + deltaTicks);
      setBuffer((current) => dragNote(current, drag.kernelId, newStart));
      setStatus(`移动 ${drag.kernelId} → ${newStart / GRID_PER_BEAT} 拍`);
    },
    [buffer.working, localOffset, editingLocked]
  );

  const onToggleDot = useCallback(() => {
    if (!selection) {
      setStatus("先点选一个音符再切换附点");
      return;
    }
    setBuffer((current) => toggleDot(current, selection.kernelId));
    setStatus(`附点切换 ${selection.kernelId}`);
  }, [selection]);

  const onSetDuration = useCallback(
    (next: ScoreDuration) => {
      setDurationValue(next);
      if (selection) {
        setBuffer((current) => setDuration(current, selection.kernelId, next));
        setStatus(`时值切换 ${selection.kernelId} → ${DURATION_LABELS[next]}`);
      }
    },
    [selection]
  );

  const onTieEstablish = useCallback(() => {
    if (!selection) {
      setStatus("先点选一个音符再建立延音");
      return;
    }
    const note = buffer.working.notes.find((item) => item.id === selection.kernelId);
    if (!note) {
      return;
    }
    const endTick = Math.round((note.startBeats + note.lengthBeats) * GRID_PER_BEAT);
    const nextNote = buffer.working.notes
      .filter((item) => item.pitch === note.pitch && Math.round(item.startBeats * GRID_PER_BEAT) === endTick)
      .sort((a, b) => a.id.localeCompare(b.id))[0];
    if (!nextNote) {
      setStatus("相邻同音高音符不存在：延音建立需要尾部相接的同音高音符");
      return;
    }
    setBuffer((current) => tieEstablish(current, selection.kernelId, nextNote.id));
    setStatus(`延音建立 ${selection.kernelId} + ${nextNote.id}`);
  }, [buffer.working, selection]);

  const onTieRelease = useCallback(() => {
    if (!selection) {
      setStatus("先点选延音链上的符号再解除延音");
      return;
    }
    if (selectionChainTicks.length <= 1) {
      setStatus("选中音符不是延音链（单符号）");
      return;
    }
    // 在点击符号之后断链：点的是链首符号→在其末符号边界断；否则在点击符号起点断。
    const idx = selectionChainTicks.indexOf(selection.tick);
    const splitTick = idx <= 0 ? selectionChainTicks[1] : selection.tick;
    setBuffer((current) => tieRelease(current, selection.kernelId, splitTick));
    setStatus(`延音解除 ${selection.kernelId} @ ${splitTick / GRID_PER_BEAT} 拍`);
  }, [selection, selectionChainTicks]);

  const onDelete = useCallback(() => {
    if (!selection) {
      setStatus("先点选一个音符再删除");
      return;
    }
    setBuffer((current) => deleteNote(current, selection.kernelId));
    setStatus(`删除 ${selection.kernelId}`);
    setSelection(null);
  }, [selection]);

  const onDiscard = useCallback(() => {
    if (applying.current || loading) return;
    setBuffer(createEditBuffer(buffer.base));
    setSelection(null);
    setConfirmOpen(false);
    setApplyState("idle");
    setStatus("已放弃本地修改（回到基线）");
    if (!ready) setRetry((value) => value + 1);
  }, [buffer.base, loading, ready]);

  const applyPatch = useMemo(() => buildApplyPatch(buffer, clipLengthBeats), [buffer, clipLengthBeats]);

  const onConfirmApply = useCallback(async () => {
    if (applying.current) return;
    if (!activeKernel || !target || !ready || loading) {
      setStatus(kernelFactory ? "尚未成功装载当前片段，无法写入；请重新选择或装载" : "未连接内核（本地模式）：apply 需真实 agent 栈");
      setApplyState("error");
      return;
    }
    const token = generation.current;
    const current = () => generation.current === token && live.current.targetKey === targetKey && live.current.targetValid;
    applying.current = true;
    setApplyState("busy");
    try {
      if (validateTarget && !(await validateTarget(target))) throw new Error("工程或片段已变化，未执行写入");
      if (!current()) return;
      // M3：applyPatch 内部已完成 回读+验证（A5：比对一致才返回 ok；桥即真栈链）。
      const applied = await activeKernel.applyPatch(applyPatch);
      if (!current()) return;
      if (!applied.ok) {
        setStatus(`写入失败：${applied.error}`);
        setApplyState("error");
        // 失败也关确认框：错误已在状态行显形，缓冲保留可继续编辑或放弃
        // （渲染面烟测实证：留开的模态会遮挡后续操作）。
        setConfirmOpen(false);
        return;
      }
      if (validateTarget && !(await validateTarget(target))) throw new Error("写入后工程或片段已变化，请重新装载核对结果");
      if (!current()) return;
      setBuffer(createEditBuffer(applied.value.model));
      setSnapshot(applied.value);
      setClipLengthBeats(applied.value.clipLengthBeats);
      setStatus("已写入内核并回读验证一致（已保存）");
      setApplyState("saved");
      setConfirmOpen(false);
      setSelection(null);
    } catch (failure) {
      if (current()) {
        setStatus(`写入未确认：${failure instanceof Error ? failure.message : String(failure)}`);
        setLoadedKey("");
        setApplyState("error");
        setConfirmOpen(false);
      }
    } finally {
      if (current()) applying.current = false;
    }
  }, [activeKernel, applyPatch, target, targetKey, ready, loading, validateTarget, kernelFactory]);

  const kernelTag = kernelFactory ? "connected" : "local";

  // M4 游标位置（快照+playhead 都在场才换算；clip 外或渲染范围外=不画）。
  const cursor =
    snapshot && playheadSeconds !== null && renderResultRef.current
      ? cursorPosition(
          renderResultRef.current,
          playheadClipBeats(playheadSeconds, snapshot.clipStartSeconds, snapshot.tempoBPM),
          buffer.working.timeSignature.numerator
        )
      : null;

  return (
    <div className="score-workbench" data-score-root data-score-mode={mode} data-score-duration={duration} data-score-dirty={dirty ? "true" : "false"} data-score-kernel={kernelTag} data-score-note-count={buffer.working.notes.length} data-score-selection={selection?.kernelId ?? ""} data-score-status={status} data-score-apply-state={applyState} data-score-model={serializeModel(buffer.working)} data-score-timesig={snapshot?.timeSignatureLabel ?? "local-2/4"} data-score-tempo={snapshot ? String(snapshot.tempoBPM) : ""} data-score-clip-start-seconds={snapshot?.clipStartSeconds ?? ""} data-score-cursor-beat={cursor ? cursor.beat.toFixed(3) : ""} data-score-cursor-playing={transportPlaying ? "true" : "false"} data-score-clip-id={clipID} data-score-loaded-target={loadedKey} data-score-write-ready={ready ? "true" : "false"}>
      <fieldset className="score-toolbar" disabled={editingLocked} aria-label="谱面编辑工具">
        <div className="score-tool-group" role="group" aria-label="输入模式">
          <button type="button" className={`score-tool ${mode === "note" ? "active" : ""}`} data-score-mode-note onClick={() => setMode("note")}>
            音符
          </button>
          <button type="button" className={`score-tool ${mode === "rest" ? "active" : ""}`} data-score-mode-rest onClick={() => setMode("rest")}>
            休止
          </button>
        </div>
        <div className="score-tool-group" role="group" aria-label="时值">
          {([4, 3, 2, 1] as ScoreDuration[]).map((value) => (
            <button key={value} type="button" className={`score-tool ${duration === value ? "active" : ""}`} data-score-dur={value} onClick={() => onSetDuration(value)}>
              {DURATION_LABELS[value]}
            </button>
          ))}
        </div>
        <label className="score-tool score-pitch">
          音高
          <select data-score-pitch value={pitch} onChange={(event) => setPitch(Number(event.currentTarget.value))}>
            {PITCH_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </label>
        <div className="score-tool-group" role="group" aria-label="符号操作">
          <button type="button" className="score-tool" data-score-toggle-dot onClick={onToggleDot} disabled={!selection}>
            附点
          </button>
          <button type="button" className="score-tool" data-score-tie-establish onClick={onTieEstablish} disabled={!selection}>
            延音接续
          </button>
          <button type="button" className="score-tool" data-score-tie-release onClick={onTieRelease} disabled={!selection || selectionChainTicks.length <= 1}>
            解除延音
          </button>
          <button type="button" className="score-tool" data-score-delete onClick={onDelete} disabled={!selection}>
            删除
          </button>
        </div>
      </fieldset>
      <div className="score-stage">
        <div className="score-svg-host" ref={svgHostRef} data-score-svg />
        {cursor && (
          <div
            className="score-cursor"
            data-score-cursor
            data-score-cursor-measure={cursor.measure}
            style={{ left: `${cursor.x}px`, top: `${cursor.y}px`, height: `${cursor.height}px` }}
          />
        )}
        <div
          className="score-overlay"
          ref={overlayRef}
          data-score-overlay
          aria-disabled={editingLocked}
          onPointerDown={onOverlayPointerDown}
          onPointerMove={onOverlayPointerMove}
          onPointerUp={onOverlayPointerUp}
        />
      </div>
      <div className="score-statusline">
        <span className="score-status-text" data-score-status-text>
          {status}
        </span>
        <span className="score-diff" data-score-diff={dirty ? "true" : "false"}>
          {dirty ? `未保存：+${diff.added} -${diff.removed} ~${diff.changed}` : ready ? "与内核一致" : kernelFactory ? "尚未装载当前片段" : "本地模型"}
        </span>
      </div>
      <div className="score-actions">
        <button type="button" className="score-tool secondary" data-score-discard onClick={onDiscard} disabled={!dirty || loading || applyState === "busy"}>
          放弃修改
        </button>
        <button type="button" className="score-tool primary" data-score-apply onClick={() => setConfirmOpen(true)} disabled={!dirty || (!!kernelFactory && !ready) || loading || applyState === "busy"}>
          写入内核…
        </button>
      </div>
      {kernelFactory && !ready && targetValid && !loading && !dirty && <button type="button" className="score-tool secondary" data-score-reload onClick={() => setRetry((value) => value + 1)}>重新装载片段</button>}
      {confirmOpen && (
        <div className="score-confirm" data-score-confirm role="dialog" aria-label="写入确认">
          <p>
            将以整段覆盖写入内核（replace_region，0–{applyPatch.region_length} 拍，{applyPatch.notes.length} 个音符，
            基线 +{diff.added} -{diff.removed} ~{diff.changed}）。此操作经治理确认链单次执行。
          </p>
          <div className="score-confirm-buttons">
            <button type="button" className="score-tool secondary" data-score-cancel-apply onClick={() => setConfirmOpen(false)} disabled={applyState === "busy"}>
              取消
            </button>
            <button type="button" className="score-tool primary" data-score-confirm-apply onClick={() => void onConfirmApply()} disabled={applyState === "busy"}>
              {applyState === "busy" ? "写入中…" : "确认写入"}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function rowOfTick(result: RenderResult, tick: number): number {
  for (const [measure, bounds] of result.staveBounds) {
    const measureTicks = (result.tickRows.get(measure) ?? []).map(([t]) => t);
    if (measureTicks.length === 0) {
      continue;
    }
    const min = Math.min(...measureTicks);
    const max = Math.max(...measureTicks);
    if (tick >= min && tick <= max) {
      return measure;
    }
  }
  return 0;
}

// 默认内核桥：无 kernel prop 时=本地模式（fail-honest apply）。
export { createNoKernelBridge };
