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
import type { TutorProgressView, TutorStepView } from "./projectData";
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

/** 当前步骤问题 + 数字输入 + 提交；completed 后禁用并显示完成。 */
export function TutorStepCard({ view, pending = false, onSubmit }: TutorStepCardProps) {
  const [value, setValue] = useState("");
  const disabled = view.completed || pending;
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
