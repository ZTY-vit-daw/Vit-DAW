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
 * 呈现：纯展示 + 受控数字输入；提交经 ProjectSurface 的既有 tutor 会话通道，
 * 本组件不发网络、不伪造步骤推进。
 */

import { useState, type FormEvent } from "react";
import { AudioPlayer } from "../widgets/AudioPlayer";
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

/** 当前步骤问题 + 数字输入 + 提交；completed 后禁用并显示完成。
 * TUTOR-SIM-4：brief 教学简报折叠展示（默认收起，只来自 workflow_data）。 */
export function TutorStepCard({ view, pending = false, onSubmit }: TutorStepCardProps) {
  const [value, setValue] = useState("");
  const [briefOpen, setBriefOpen] = useState(false);
  const disabled = view.completed || pending;
  const brief = (view.brief ?? "").trim();
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
          type="number"
          inputMode="numeric"
          value={value}
          disabled={disabled}
          placeholder={view.completed ? "实验已完成" : "输入数字答案"}
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
}

/** MIDI 材料卡：材料名 + 摘要（音符数/时长）+「装载到工程」。
 * 已装载/在途/摘要缺失时按钮禁用（防重复装载）；装载态只来自服务端回执。 */
export function MidiMaterialCard({ view, pending = false, onLoad }: MidiMaterialCardProps) {
  const disabled = view.loaded || view.loading || pending || Boolean(view.summaryError);
  const buttonLabel = pending ? "装载中…" : view.loaded ? "已装载" : "装载到工程";
  return (
    <div
      className="v5-project-tutor-material"
      data-tutor-material
      data-tutor-material-kind={view.kind}
      data-tutor-material-title={view.title}
      data-tutor-material-loaded={view.loaded ? "true" : "false"}
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
