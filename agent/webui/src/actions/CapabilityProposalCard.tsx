// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2a）：App.tsx 动作卡体系全域出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
// CapabilityProposalCard 提案确认卡全家。

import { Loader2 } from "lucide-react";
import { useState } from "react";
import type { ChatMessage, JsonRecord } from "../types";
import { asRecord, finiteNumber, firstArray, textValue, truthy } from "../lib/util";
import { CONSUMED_INTERACTION_GUARD_ID } from "../interactionGuard";
import { SUPERSEDED_BY_NEWER_RESOLVED_ACTION, TURN_EXPIRED_RESOLVED_ACTION } from "../messageLifecycle";
import { interactionActionID } from "../App";
import { localizeDisplayText } from "./labels";
import { interactionPayload, interactionPayloadForAction } from "./confirmation";
import { actionRenderID, isCapabilityProposalInteraction } from "./StandardActionCard";
import type { ActionCardProps } from "./types";

export function shouldSuppressProposalContent(message: ChatMessage): boolean {
  if (message.role !== "assistant" || !message.actions?.length) {
    return false;
  }
  return message.actions.map(asRecord).some(isCapabilityProposalInteraction);
}

export type ProposalCardOutcome = { tone: "blue" | "red" | "gray"; icon: "check" | "cross" | "undo"; text: string };

/** 确认卡沉淀结果条推导（GUI-T4）：批准/拒绝走 resolved_action_id 与 status，补充未采用走 supersedes 渲染 */
export function proposalCardOutcome(action: JsonRecord, resolved: boolean): ProposalCardOutcome | null {
  if (!resolved) {
    return null;
  }
  const status = textValue(action.status ?? action.stage, "").toLowerCase();
  const resolvedActionID = textValue(action.resolved_action_id, "").toLowerCase();
  const terminal = status.includes("complete") || status.includes("cancel") || status.includes("fail");
  if (!resolvedActionID && !terminal) {
    return null;
  }
  if (resolvedActionID === "superseded") {
    return { tone: "gray", icon: "undo", text: "已收到你的补充 · 卡面选项未采用" };
  }
  // FIX-CONFIRM-CARD-1 ② 终态口径：同族新卡替代（旧交互 id 已失效）、回合
  // 终结未应答、台账盖章（已应答/撤卡）——一律灰沉淀、不可交互，不残留
  // 可点击的"过期"形态。
  if (resolvedActionID === SUPERSEDED_BY_NEWER_RESOLVED_ACTION) {
    return { tone: "gray", icon: "undo", text: "已替代 · 以最新方案为准" };
  }
  if (resolvedActionID === TURN_EXPIRED_RESOLVED_ACTION) {
    return { tone: "gray", icon: "cross", text: "已失效 · 回合已结束" };
  }
  if (resolvedActionID === CONSUMED_INTERACTION_GUARD_ID) {
    return { tone: "gray", icon: "undo", text: "已处理 · 交互已关闭" };
  }
  if (resolvedActionID === "turn_terminal_receipt") {
    return status.includes("fail")
      ? { tone: "red", icon: "cross", text: "已失效 · 本回合执行失败" }
      : { tone: "blue", icon: "check", text: "已按对话处理 · 本回合执行完毕" };
  }
  if (status.includes("cancel")) {
    return { tone: "red", icon: "cross", text: "已拒绝 · 未做任何改动" };
  }
  if (status.includes("fail")) {
    return { tone: "red", icon: "cross", text: "已失效 · 未执行" };
  }
  const resolvedAt = finiteNumber(action.resolved_at, 0);
  return { tone: "blue", icon: "check", text: resolvedAt > 0 ? `已批准 · ${proposalClock(resolvedAt)}` : "已批准" };
}

function proposalClock(epochMS: number): string {
  const date = new Date(epochMS);
  return `${String(date.getHours()).padStart(2, "0")}:${String(date.getMinutes()).padStart(2, "0")}`;
}

/** childActions 批准/拒绝席位：取消/拒绝语义占拒绝位（白底 hover 红），其余（approve/primary/recommended）占批准位（蓝底） */
function isProposalApproveAction(child: JsonRecord, actionID: string): boolean {
  const id = actionID.toLowerCase();
  if (["cancel", "reject", "deny", "dismiss"].includes(id)) {
    return false;
  }
  const label = textValue(child.label ?? child.title, "").toLowerCase();
  if (["取消", "拒绝", "不执行"].some((word) => label.includes(word))) {
    return false;
  }
  return true;
}

export function CapabilityProposalCard({ action, respondingActionID, onInteractionAction, authorityMode, onSendSupplement }: ActionCardProps) {
  const [supplementText, setSupplementText] = useState("");
  const [supplementSent, setSupplementSent] = useState(false);
  const payload = interactionPayload(action);
  const presentation = asRecord(payload.proposal_presentation ?? action.proposal_presentation);
  const childActions = firstArray(action.actions)
    .map(asRecord)
    .filter((item) => Object.keys(item).length > 0);
  const interactionID = textValue(action.id ?? action.interaction_id, "");
  const renderID = actionRenderID(action);
  const title = textValue(presentation.title ?? action.title, "方案等待确认");
  const conclusion = textValue(presentation.conclusion ?? action.body, "");
  const recommendation = textValue(presentation.recommendation, "");
  const revision = finiteNumber(presentation.proposal_revision ?? payload.proposal_revision, 0);
  const actionCount = finiteNumber(presentation.action_count, 0);
  const analyzedTracks = finiteNumber(presentation.analyzed_tracks, 0);
  const risk = textValue(presentation.risk, "");
  const reversible = truthy(presentation.reversible);
  const status = textValue(action.status ?? action.stage, "waiting_for_user").toLowerCase();
  const resolved = childActions.length === 0 || status.includes("complete") || status.includes("cancel") || status.includes("fail");
  const readiness = firstArray(presentation.readiness).map(asRecord).filter((item) => Object.keys(item).length > 0);
  const groups = firstArray(presentation.change_groups).map(asRecord).filter((item) => Object.keys(item).length > 0);
  const previews = firstArray(presentation.actions).map(asRecord).filter((item) => Object.keys(item).length > 0);
  const summaries = firstArray(presentation.analysis_summary).map((item) => textValue(item, "")).filter(Boolean);
  const limitations = firstArray(presentation.limitations).map((item) => textValue(item, "")).filter(Boolean);
  const outcome = proposalCardOutcome(action, resolved)
    ?? (supplementSent ? { tone: "gray" as const, icon: "undo" as const, text: "已收到你的补充 · 卡面选项未采用" } : null)
    ?? (authorityMode === "full_project_access" ? { tone: "gray" as const, icon: "check" as const, text: "完全档 · 已直接执行" } : null);
  const settled = outcome !== null;

  const sendSupplement = () => {
    const value = supplementText.trim();
    if (!value || !onSendSupplement) {
      return;
    }
    setSupplementSent(true);
    setSupplementText("");
    onSendSupplement(value);
  };

  return (
    <section className={["card", "capability-proposal-card", settled ? "settled" : ""].filter(Boolean).join(" ")} aria-label="确认 · 试验步方案">
      <div className="tab">确认 · 第 {revision + 1} 轮试验步</div>
      <div className="c-head">
        <span className="csum">{title}</span>
        {!settled && <span className="chip">待裁</span>}
      </div>
      {(conclusion || title) && (
        <div className="c-row"><span className="k">改动</span><span className="v">{conclusion || title}</span></div>
      )}
      {recommendation && (
        <div className="c-row"><span className="k">假设</span><span className="v">{recommendation}</span></div>
      )}
      <div className="c-row">
        <span className="k">预期</span>
        <span className="v">{actionCount} 项修改确认后一次生效 · {reversible ? "可一键回滚" : "执行后不可回滚"}</span>
      </div>
      <div className="capability-proposal-facts" aria-label="方案摘要">
        {analyzedTracks > 0 && <span>已分析 {analyzedTracks} 轨</span>}
        <span>{actionCount} 项修改</span>
        {risk && <span>风险 {localizeDisplayText(risk)}</span>}
        <span>{reversible ? "可回滚" : "不可回滚"}</span>
      </div>
      {groups.length > 0 && (
        <div className="capability-proposal-groups">
          {groups.slice(0, 6).map((group, index) => {
            const label = textValue(group.label ?? group.role ?? group.function, `分组 ${index + 1}`);
            const count = finiteNumber(group.move_count ?? group.track_count, 0);
            const unit = textValue(group.unit, "");
            const min = finiteNumber(group.min_value, 0);
            const max = finiteNumber(group.max_value, 0);
            const range = min === max ? formatProposalValue(min, unit) : `${formatProposalValue(min, unit)} ～ ${formatProposalValue(max, unit)}`;
            return <span key={`${textValue(group.id, label)}-${index}`}><strong>{label}</strong>{count > 0 ? ` · ${count} 项` : ""}{unit ? ` · ${range}` : ""}</span>;
          })}
        </div>
      )}
      {(summaries.length > 0 || recommendation || readiness.length > 0 || previews.length > 0 || limitations.length > 0) && (
        <details className="capability-proposal-details">
          <summary>查看分析依据与逐轨修改</summary>
          {summaries.length > 0 && (
            <section>
              <h4>分析摘要</h4>
              <ul>{summaries.map((item, index) => <li key={`${item}-${index}`}>{item}</li>)}</ul>
            </section>
          )}
          {recommendation && <section><h4>推荐理由</h4><p>{recommendation}</p></section>}
          {readiness.length > 0 && (
            <section>
              <h4>证据覆盖</h4>
              <div className="capability-proposal-metrics">
                {readiness.map((metric, index) => (
                  <span key={`${textValue(metric.id, "metric")}-${index}`}>
                    <strong>{textValue(metric.label, "指标")}</strong>
                    {textValue(metric.value, "—")}
                  </span>
                ))}
              </div>
            </section>
          )}
          {previews.length > 0 && (
            <section>
              <h4>逐轨修改</h4>
              <div className="capability-proposal-preview-list">
                {previews.map((preview, index) => {
                  const unit = textValue(preview.unit, "");
                  const before = finiteNumber(preview.before, 0);
                  const target = finiteNumber(preview.target, 0);
                  const delta = finiteNumber(preview.delta, 0);
                  return (
                    <div key={`${textValue(preview.action_id ?? preview.track_id, "change")}-${index}`}>
                      <strong>{textValue(preview.track_name ?? preview.track_id, `轨道 ${index + 1}`)}</strong>
                      <span>{formatProposalValue(before, unit)} → {formatProposalValue(target, unit)} ({formatProposalDelta(delta, unit)})</span>
                      {textValue(preview.reason, "") && <small>{textValue(preview.reason, "")}</small>}
                    </div>
                  );
                })}
              </div>
            </section>
          )}
          {limitations.length > 0 && (
            <section className="capability-proposal-limitations">
              <h4>限制与风险</h4>
              <ul>{limitations.map((item, index) => <li key={`${item}-${index}`}>{item}</li>)}</ul>
            </section>
          )}
        </details>
      )}
      {!settled && childActions.length > 0 && (
        <div className="c-actions capability-proposal-actions">
          {childActions.map((child, childIndex) => {
            const actionID = textValue(child.id ?? child.action_id ?? child.decision, `action_${childIndex + 1}`);
            const label = textValue(child.label ?? child.title ?? actionID, actionID);
            const pendingID = interactionActionID(interactionID || renderID || "proposal", actionID);
            const isBusy = respondingActionID === pendingID;
            const approveSeat = isProposalApproveAction(child, actionID);
            return (
              <button
                key={`${actionID}-${childIndex}`}
                className={`btn ${approveSeat ? "approve" : "reject"}`}
                type="button"
                disabled={respondingActionID !== ""}
                onClick={() => onInteractionAction(action, child, capabilityProposalInteractionPayload(action, child))}
              >
                {isBusy ? <Loader2 className="spin" size={14} /> : approveSeat ? <ProposalCheckIcon /> : <ProposalCrossIcon />}
                <span>{label}</span>
              </button>
            );
          })}
        </div>
      )}
      {!settled && onSendSupplement && (
        <form
          className="c-ask"
          title="可直接回复“执行这个方案”，也可以继续提问、排除轨道或修改数值。"
          onSubmit={(event) => {
            event.preventDefault();
            sendSupplement();
          }}
        >
          <input
            type="text"
            value={supplementText}
            onChange={(event) => setSupplementText(event.currentTarget.value)}
            placeholder="不用选项？直接补充你的要求…"
            aria-label="方案补充输入"
          />
          <button type="submit" className="askb" aria-label="发送补充" disabled={respondingActionID !== ""}>
            <ProposalSendIcon />
          </button>
        </form>
      )}
      {outcome && (
        <div className={`outcome tone-${outcome.tone}`}>
          {outcome.icon === "undo" ? <ProposalUndoIcon /> : outcome.icon === "cross" ? <ProposalCrossIcon /> : <ProposalCheckIcon />}
          <span>{outcome.text}</span>
        </div>
      )}
    </section>
  );
}

function ProposalCheckIcon() {
  return (
    <svg viewBox="0 0 12 12" aria-hidden="true">
      <path d="M1.8 6.4 4.8 9.2 10.2 2.8" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />
    </svg>
  );
}

function ProposalCrossIcon() {
  return (
    <svg viewBox="0 0 12 12" aria-hidden="true">
      <path d="M2.5 2.5 9.5 9.5M9.5 2.5 2.5 9.5" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="square" />
    </svg>
  );
}

function ProposalUndoIcon() {
  return (
    <svg viewBox="0 0 14 14" aria-hidden="true">
      <path d="M2.6 5.6h5.1a3.6 3.6 0 1 1-3.5 4.6" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="square" />
      <path d="M5 3.3V7.9L1.4 5.6Z" fill="currentColor" />
    </svg>
  );
}

function ProposalSendIcon() {
  return (
    <svg viewBox="0 0 12 12" aria-hidden="true">
      <path d="M1.5 1 11 6 1.5 11Z" fill="currentColor" />
    </svg>
  );
}

function formatProposalValue(value: number, unit: string): string {
  const suffix = unit.trim() ? ` ${unit.trim()}` : "";
  return `${value.toFixed(2)}${suffix}`;
}

function capabilityProposalInteractionPayload(interaction: JsonRecord, action: JsonRecord): JsonRecord {
  const parent = interactionPayload(interaction);
  return {
    ...interactionPayloadForAction(interaction, action, {}),
    workflow: "capability_runtime_v1",
    approval_mode: textValue(parent.approval_mode, "conversational"),
    conversation_id: textValue(interaction.conversation_id ?? parent.conversation_id, ""),
    goal_id: textValue(interaction.goal_id ?? parent.goal_id, ""),
    run_id: textValue(interaction.run_id ?? parent.run_id, ""),
    session_id: textValue(parent.session_id, ""),
    capability_id: textValue(parent.capability_id, ""),
    proposal_id: textValue(parent.proposal_id ?? interaction.plan_id, ""),
    proposal_revision: finiteNumber(parent.proposal_revision, 0),
    action_set_hash: textValue(parent.action_set_hash, ""),
    project_cut_hash: textValue(parent.project_cut_hash, "")
  };
}

function formatProposalDelta(value: number, unit: string): string {
  return `${value > 0 ? "+" : ""}${formatProposalValue(value, unit)}`;
}

