/*
 * TUTOR-SIM-3（池序 437）：工程面对话流内导师控件。
 *
 * 数据红线：
 * - TutorProgressCard 的 X/Y/completed 只来自服务端 workflow_data（start/chat 回执），
 *   不从 LLM 回复文本或旧状态字符串反推；
 * - TutorStepCard 的 prompt 只来自最近 assistant 回复（服务端 reply），本组件不接触
 *   服务端判定答案，也不显示任何原始 JSON/状态文本；
 * - completed 后输入与提交禁用（保留只读问题与完成态），不自动发送、无命令词。
 *
 * 呈现：纯展示 + 受控步骤答案输入（TUTOR-INPUT-TYPE-1：数字框/文本框按
 * workflow_data.answer_kind 自适应）；提交经 ProjectSurface 的既有 tutor 会话
 * 通道，本组件不发网络、不伪造步骤推进。
 */

import { useEffect, useRef, useState, type FormEvent } from "react";
import { AudioPlayer } from "../widgets/AudioPlayer";
import { renderScoreInto } from "../scoreworkbench/render";
import type { ScoreModel } from "../scoreworkbench/model";
import type { TutorMaterialView, TutorProgressView, TutorStepView } from "./projectData";
import "./TutorWidgets.css";

export interface TutorProgressCardProps {
  view: TutorProgressView;
}

/** 第 X/Y 步与完成态（结构化；无进度条/无原始状态文本）。 */
export function TutorProgressCard({ view }: TutorProgressCardProps) {
  const label = view.completed
    ? `第 ${view.stepIndex}/${view.stepTotal} 步 · 已完成`
    : `第 ${view.stepIndex}/${view.stepTotal} 步`;
  return (
    <div
      className="v5-project-tutor-progress"
      data-tutor-progress
      data-tutor-progress-step={view.stepIndex}
      data-tutor-progress-total={view.stepTotal}
      data-tutor-progress-completed={view.completed ? "true" : "false"}
    >
      <span className="v5-project-tutor-progress-label" data-tutor-progress-label>
        {label}
      </span>
    </div>
  );
}

export interface TutorStepCardProps {
  view: TutorStepView;
  /** 同一条 tutor 会话回复在途：禁用输入，防重复提交。 */
  pending?: boolean;
  onSubmit: (value: string) => void;
}

/** 当前步骤问题 + 步骤答案输入 + 提交；completed 后禁用并显示完成。
 * TUTOR-SIM-4：brief 教学简报折叠展示（默认收起，只来自 workflow_data）。
 * TUTOR-INPUT-TYPE-1（444）：输入型按 view.answerKind 自适应——"text" 渲染
 * 文本框（词答案「完成」「模进」等可输入），缺省/undefined/"number" 渲染
 * 数字框（v0 行为）。提交链路不变：仍走 onSubmit(文本)，服务端判定权威。 */
export function TutorStepCard({ view, pending = false, onSubmit }: TutorStepCardProps) {
  const [value, setValue] = useState("");
  const [briefOpen, setBriefOpen] = useState(false);
  const disabled = view.completed || pending;
  const brief = (view.brief ?? "").trim();
  const textInput = view.answerKind === "text";
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const text = value.trim();
    if (!text || disabled) return;
    onSubmit(text);
    setValue("");
  };
  return (
    <div
      className="v5-project-tutor-step"
      data-tutor-step
      data-tutor-step-index={view.stepIndex}
      data-tutor-step-total={view.stepTotal}
      data-tutor-step-completed={view.completed ? "true" : "false"}
      data-tutor-step-pending={pending ? "true" : "false"}
    >
      <div className="v5-project-tutor-step-head">
        <span className="v5-project-tutor-step-title" data-tutor-step-title>
          第 {view.stepIndex}/{view.stepTotal} 步
        </span>
        {view.completed ? (
          <span className="v5-project-tutor-step-done" data-tutor-step-done>
            实验已完成
          </span>
        ) : null}
      </div>
      {view.prompt ? (
        <p className="v5-project-tutor-step-prompt" data-tutor-step-prompt>
          {view.prompt}
        </p>
      ) : null}
      {brief ? (
        <div
          className="v5-project-tutor-brief"
          data-tutor-brief
          data-tutor-brief-open={briefOpen ? "true" : "false"}
        >
          <button
            type="button"
            className="v5-project-tutor-brief-toggle"
            data-tutor-brief-toggle
            aria-expanded={briefOpen}
            onClick={() => setBriefOpen((open) => !open)}
          >
            {briefOpen ? "收起简报" : "查看简报"}
          </button>
          {briefOpen ? (
            <p className="v5-project-tutor-brief-text" data-tutor-brief-text>
              {brief}
            </p>
          ) : null}
        </div>
      ) : null}
      <form className="v5-project-tutor-step-form" data-tutor-step-form onSubmit={submit}>
        <input
          className="v5-project-tutor-step-input"
          data-tutor-step-input
          data-tutor-step-input-kind={textInput ? "text" : "number"}
          type={textInput ? "text" : "number"}
          inputMode={textInput ? "text" : "numeric"}
          value={value}
          disabled={disabled}
          placeholder={view.completed ? "实验已完成" : textInput ? "输入文本答案" : "输入数字答案"}
          aria-label="步骤答案"
          onChange={(event) => setValue(event.target.value)}
        />
        <button
          type="submit"
          className="v5-btn v5-btn--primary v5-project-tutor-step-submit"
          data-tutor-step-submit
          disabled={disabled || !value.trim()}
        >
          {pending ? "提交中…" : "提交"}
        </button>
      </form>
      {view.completed ? (
        <p className="v5-project-tutor-step-note" data-tutor-step-note>
          导师实验已完成，无需继续输入。
        </p>
      ) : null}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* TUTOR-SIM-4（438）：材料卡（midi_clip 装载 / audio schema 位）         */
/* ------------------------------------------------------------------ */

function midiMaterialSummary(view: TutorMaterialView): string {
  if (view.summaryError) {
    return "材料不可用（解析失败）";
  }
  const parts: string[] = [];
  if (typeof view.noteCount === "number") parts.push(`${view.noteCount} 音`);
  if (typeof view.durationBeats === "number") parts.push(`${Math.round(view.durationBeats * 10) / 10} 拍`);
  if (parts.length === 0) return "MIDI 材料";
  return parts.join(" · ");
}

export interface MidiMaterialCardProps {
  view: TutorMaterialView;
  /** 装载请求在途（同名材料）：按钮禁用、显示装载中。 */
  pending?: boolean;
  /** 发起装载（调用方经 ProjectService 接缝；组件不发网络）。 */
  onLoad: () => void;
  /** SIM-MUSIC-WIDGET-1（441）：已装载材料的谱面模型（音符序列）。
   * 缺省/undefined/null 或材料未装载=谱面入口不渲染（fail-honest 不伪造）；
   * 有值且已装载=「查看谱面」可展开（默认收起，不撑爆对话流）。数据源接缝：
   * 装置侧材料文件只读解析（midiScore.ts）或 live 装载链内核回读
   * （createAgentScoreBridge.reload，374 既有协议；调用方接线）。 */
  score?: ScoreModel | null;
}

/** MIDI 材料卡：材料名 + 摘要（音符数/时长）+「装载到工程」。
 * 已装载/在途/摘要缺失时按钮禁用（防重复装载）；装载态只来自服务端回执。
 * SIM-MUSIC-WIDGET-1（441）：带谱面数据时显示「查看谱面」——对话流内联展开
 * 谱面渲染（与 brief 折叠同模式，默认收起）。 */
export function MidiMaterialCard({ view, pending = false, onLoad, score }: MidiMaterialCardProps) {
  const disabled = view.loaded || view.loading || pending || Boolean(view.summaryError);
  const buttonLabel = pending ? "装载中…" : view.loaded ? "已装载" : "装载到工程";
  const [scoreOpen, setScoreOpen] = useState(false);
  return (
    <div
      className="v5-project-tutor-material"
      data-tutor-material
      data-tutor-material-kind={view.kind}
      data-tutor-material-title={view.title}
      data-tutor-material-loaded={view.loaded ? "true" : "false"}
      data-tutor-score-available={score && view.loaded ? "true" : "false"}
    >
      <span className="v5-project-tutor-material-name" data-tutor-material-name>
        {view.title}
      </span>
      <span className="v5-project-tutor-material-summary" data-tutor-material-summary>
        {midiMaterialSummary(view)}
      </span>
      <button
        type="button"
        className="v5-btn v5-btn--primary v5-project-tutor-material-load"
        data-tutor-material-load
        disabled={disabled}
        onClick={onLoad}
      >
        {buttonLabel}
      </button>
      {view.loaded ? (
        <span
          className="v5-project-tutor-material-instrument"
          data-tutor-material-instrument
          data-tutor-material-instrument-attached={
            view.instrumentAttached === false ? "false" : "true"
          }
        >
          {view.instrumentAttached === false
            ? `已装载；音源未挂：${view.instrumentError ?? "内核拒绝"}`
            : "已挂音源"}
        </span>
      ) : null}
      {view.loadError && !view.loaded ? (
        <span className="v5-project-tutor-material-error" data-tutor-material-load-error>
          {view.loadError}
        </span>
      ) : null}
      {score && view.loaded ? (
        <div
          className="v5-project-tutor-material-score"
          data-tutor-score
          data-tutor-score-open={scoreOpen ? "true" : "false"}
        >
          <button
            type="button"
            className="v5-btn v5-btn--text v5-project-tutor-material-score-toggle"
            data-tutor-score-toggle
            aria-expanded={scoreOpen}
            onClick={() => setScoreOpen((open) => !open)}
          >
            {scoreOpen ? "收起谱面" : "查看谱面"}
          </button>
          {scoreOpen ? <MaterialScoreView model={score} /> : null}
        </div>
      ) : null}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* SIM-MUSIC-WIDGET-1（441）：材料卡内联谱面视图                          */
/* ------------------------------------------------------------------ */

/** 谱面视图：复用 374 SCORE-WORKBENCH-1 的 beat-grid 渲染适配层
 * （scoreworkbench/render.ts renderScoreInto——纯函数，VexFlow 唯一入口；
 * ScoreWorkbenchPanel 容器强耦合工作台编辑器，本卡只挂其渲染层）。
 * 渲染异常 fail-honest（错误文本显形，不卸载对话流）。
 * AudioPlayer 挂载留位：谱面下方（音频形态候内核渲染 MIDI→wav，后续卡，
 * 接缝形态同 AudioMaterialCard 的 AudioPlayer 复用面）。 */
export function MaterialScoreView({ model }: { model: ScoreModel }) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const [renderError, setRenderError] = useState("");
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    setRenderError("");
    try {
      renderScoreInto(container, model, { width: 300, rowHeight: 110 });
    } catch (failure) {
      setRenderError(failure instanceof Error ? failure.message : String(failure));
    }
  }, [model]);
  return (
    <div className="v5-project-tutor-score" data-tutor-score-view>
      <div className="v5-project-tutor-score-meta" data-tutor-score-meta>
        <span className="v5-project-tutor-score-notes" data-tutor-score-notes>
          {model.notes.length} 音
        </span>
        <span className="v5-project-tutor-score-timesig" data-tutor-score-timesig>
          {model.timeSignature.numerator}/{model.timeSignature.denominator}
        </span>
      </div>
      {renderError ? (
        <p className="v5-project-tutor-score-error" data-tutor-score-error>
          谱面渲染中止：{renderError}
        </p>
      ) : (
        <div className="v5-project-tutor-score-svg" data-tutor-score-svg ref={containerRef} />
      )}
      {/* AudioPlayer 挂载接缝（441 留位）：内核渲染 MIDI→wav 后在此复用
          widgets/AudioPlayer（同 AudioMaterialCard 形态），本卡不动实现。 */}
    </div>
  );
}

/** audio 材料（本轮 schema 位，无真实内容）：只读复用 widgets/AudioPlayer。
 * 无有效音频引用时播放器诚实降级（禁播占位），不伪造可播状态。 */
export function AudioMaterialCard({ view }: { view: TutorMaterialView }) {
  return (
    <div
      className="v5-project-tutor-material v5-project-tutor-material--audio"
      data-tutor-material
      data-tutor-material-kind={view.kind}
      data-tutor-material-title={view.title}
      data-tutor-material-loaded={view.loaded ? "true" : "false"}
    >
      <AudioPlayer title={view.title} />
    </div>
  );
}
