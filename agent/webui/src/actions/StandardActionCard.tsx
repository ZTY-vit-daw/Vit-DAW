// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2a）：App.tsx 动作卡体系全域出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
// StandardActionCard 标准动作卡 + 交互解析/决议簿记。

import { AlertTriangle, CheckCircle2, ChevronRight, Loader2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import type { ChatMessage, ChatResponse, JsonRecord } from "../types";
import { asRecord, clampNumber, firstArray, textValue, truthy } from "../lib/util";
import { isConsumedInteractionStatus } from "../interactionGuard";
import { interactionActionID } from "../App";
import { actionBadge, actionBody, actionTitle, actionTone, isCompactActionCard } from "./labels";
import { InteractionPager, buildInteractionPages, emptyInteractionPage } from "./InteractionPager";
import { ActionDetails, confirmationReviewItems, hasMissingRequiredFields, initialFieldValues, interactionFields, interactionPayload, interactionPayloadForAction, interactionReviewItems, isCancelAction, isConfirmationAction, isMixTreatmentPendingAction, mergeReviewItems } from "./confirmation";
import type { ActionCardProps } from "./types";

export function StandardActionCard({
  action,
  index,
  respondingActionID,
  onInteractionAction,
  onSelectArtifact
}: ActionCardProps) {
  const source = textValue(action._ui_source, "");
  const status = textValue(action.status ?? action.risk ?? action.risk_level ?? action.kind, "pending");
  const title = actionTitle(action, index);
  const body = actionBody(action);
	const childActions = firstArray(action.actions)
		.map(asRecord)
		.filter((item) => Object.keys(item).length > 0);
  const interactionID = textValue(action.id ?? action.interaction_id, "");
  const renderID = actionRenderID(action);
	const isInteractive = interactionID !== "" && childActions.length > 0;
  const isConfirmation = isConfirmationAction(action);
  const fields = interactionFields(action);
  const reviewItems = isConfirmation
    ? mergeReviewItems(confirmationReviewItems(action), interactionReviewItems(action))
    : interactionReviewItems(action);
  const command = isConfirmation ? {} : asRecord(action.command);
	const pages = useMemo(
		() => buildInteractionPages({ reviewItems, fields, command }),
		[action, fields, reviewItems, command]
  );
  const [pageIndex, setPageIndex] = useState(0);
  const fieldSignature = fields.map((field) => `${textValue(field.id, "")}:${textValue(field.value, "")}`).join("|");
  const [fieldValues, setFieldValues] = useState<JsonRecord>(() => initialFieldValues(fields));

  useEffect(() => {
    setFieldValues(initialFieldValues(fields));
    setPageIndex(0);
  }, [interactionID, fieldSignature]);

  const tone = actionTone(action, isInteractive);
  const Icon = tone === "error" ? AlertTriangle : tone === "attention" ? AlertTriangle : tone === "success" ? CheckCircle2 : ChevronRight;
  const requiredMissing = hasMissingRequiredFields(fields, fieldValues);
  const currentPage = pages[Math.min(pageIndex, Math.max(0, pages.length - 1))] ?? emptyInteractionPage();
  const isLastPage = pageIndex >= pages.length - 1;
	const compact = isCompactActionCard(action, tone, isInteractive, false);

  return (
		<div className={`action-card ${tone} ${isInteractive ? "interactive" : ""} ${compact ? "compact" : ""}`}>
      <Icon size={16} />
      <div className="action-content">
        <div className="action-title-line">
          <strong>{title}</strong>
          <span>{actionBadge(action, source, status)}</span>
        </div>
        {body && !compact && <p className="action-body">{body}</p>}
        {pages.length > 1 && !compact && (
          <InteractionPager
            pageIndex={pageIndex}
            pageCount={pages.length}
            onPageChange={(nextPage) => setPageIndex(clampNumber(nextPage, 0, pages.length - 1))}
          />
        )}
        {!compact && (
          <ActionDetails
            reviewItems={currentPage.reviewItems}
            fields={currentPage.fields}
            command={currentPage.showCommand ? command : {}}
				fieldValues={fieldValues}
            onFieldChange={(fieldID, value) => setFieldValues((current) => ({ ...current, [fieldID]: value }))}
            onSelectArtifact={onSelectArtifact}
          />
        )}
        {isInteractive && isLastPage && (
          <div className="action-buttons">
            {childActions.map((child, childIndex) => {
              const actionID = textValue(child.id ?? child.action_id ?? child.decision, `action_${childIndex + 1}`);
              const label = textValue(child.label ?? child.title ?? actionID, actionID);
              const pendingID = interactionActionID(interactionID || renderID || "action", actionID);
              const isBusy = respondingActionID === pendingID;
              const style = textValue(child.style, "secondary");
              const disabled = respondingActionID !== "" || (requiredMissing && !isCancelAction(actionID));
              return (
                <button
                  key={`${actionID}-${childIndex}`}
                  className={`action-button ${style}`}
                  type="button"
                  disabled={disabled}
                  onClick={() => onInteractionAction(action, child, interactionPayloadForAction(action, child, fieldValues))}
                >
                  {isBusy && <Loader2 className="spin" size={14} />}
                  <span>{label}</span>
                </button>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

export function actionRenderID(action: JsonRecord): string {
  return textValue(action.id ?? action.interaction_id ?? action.plan_id ?? action.title ?? action.type ?? action.kind, "");
}

export function resolveInteractionInMessages(messages: ChatMessage[], interactionID: string, actionID: string, response: ChatResponse): ChatMessage[] {
  const id = interactionID.trim();
  if (!id) {
    return messages;
  }
  const resolvedStatus = interactionResolvedStatus(actionID, response);
  const resolvedAt = Date.now();
  let changed = false;
  const nextMessages = messages.map((message) => {
    if (!message.actions?.length) {
      return message;
    }
    let actionChanged = false;
    const actions = message.actions.map((action) => {
      if (textValue(action.id ?? action.interaction_id, "") !== id) {
        return action;
      }
      actionChanged = true;
      changed = true;
      return {
        ...action,
        status: resolvedStatus,
        stage: resolvedStatus,
        resolved_action_id: actionID,
        resolved_at: resolvedAt,
        actions: []
      };
    });
    return actionChanged ? { ...message, actions } : message;
  });
  return changed ? nextMessages : messages;
}

export function removeInteractionActionFromMessages(messages: ChatMessage[], interactionID: string, renderID: string, interaction?: JsonRecord): ChatMessage[] {
  const ids = interactionDismissalSet([interactionID, renderID]);
  if (ids.size === 0) {
    return messages;
  }
  const dismissesConfirmation = interaction ? isConfirmationAction(interaction) : false;
  let changed = false;
  const nextMessages = messages.flatMap((message) => {
    if (dismissesConfirmation && isDisposableConfirmationPromptMessage(message)) {
      changed = true;
      return [];
    }
    if (!message.actions?.length) {
      return [message];
    }
    const removedActions: JsonRecord[] = [];
    const actions = message.actions.filter((action) => {
      const record = asRecord(action);
      const actionID = textValue(record.id ?? record.interaction_id, "");
      const actionRenderIDValue = actionRenderID(record);
      const removed = ids.has(actionID) || ids.has(actionRenderIDValue);
      if (removed) {
        removedActions.push(record);
      }
      return !removed;
    });
    if (actions.length === message.actions.length) {
      return [message];
    }
    changed = true;
    if (isDisposableInteractionCarrierMessage(message, removedActions, actions, dismissesConfirmation)) {
      return [];
    }
    return [{ ...message, actions }];
  });
  return changed ? nextMessages : messages;
}

function isDisposableInteractionCarrierMessage(
  message: ChatMessage,
  removedActions: JsonRecord[],
  remainingActions: JsonRecord[],
  dismissesConfirmation: boolean
): boolean {
  if (message.artifacts?.length || remainingActions.length > 0) {
    return false;
  }
  if (removedActions.length === 0) {
    return false;
  }
  if (removedActions.some((action) => isComposerInteraction(action) || isConfirmationAction(action))) {
    return true;
  }
  return dismissesConfirmation && isDisposableConfirmationPromptText(message.content);
}

export function isDisposableConfirmationPromptMessage(message: ChatMessage): boolean {
  if (message.status === "error" || message.artifacts?.length) {
    return false;
  }
  if (!isDisposableConfirmationPromptText(message.content)) {
    return false;
  }
  const actions = (message.actions ?? []).map(asRecord);
  return actions.length === 0 || actions.some((action) => isApprovalPromptAction(action) || isConfirmationAction(action) || isComposerInteraction(action));
}

export function isDisposableConfirmationPromptText(content: string): boolean {
  const text = content.trim();
  if (!text) {
    return false;
  }
  const compact = text.replace(/\s+/g, " ");
  const genericChineseConfirmation = compact.length <= 160 && (
    /^(?:这个|此|该)?操作.*(?:需要|等待).*确认.*(?:执行|继续)[。.!！]?$/u.test(compact) ||
    /^(?:确认|批准)后(?:才会|将会|即可)?.*(?:执行|继续)[。.!！]?$/u.test(compact) ||
    /^(?:需要|等待)你(?:的)?确认(?:后才会执行)?[。.!！]?$/u.test(compact)
  );
  return (
    text === "This action requires confirmation before execution." ||
    text === "This action will modify the project and requires confirmation." ||
    (text.toLowerCase().includes("confirm") && text.length <= 120) ||
    genericChineseConfirmation
  );
}

function isApprovalPromptAction(action: JsonRecord): boolean {
  if (isMixTreatmentPendingAction(action)) {
    return false;
  }
  const type = textValue(action.type, "").toLowerCase();
  const status = textValue(action.status ?? action.stage, "").toLowerCase();
  return type === "approval.requested" || status.includes("waiting") || status.includes("confirm") || truthy(action.requires_confirmation);
}

export function addInteractionDismissals(current: string[], ...ids: string[]): string[] {
  const next = interactionDismissalSet(current);
  ids.forEach((id) => {
    const cleanID = id.trim();
    if (cleanID) {
      next.add(cleanID);
    }
  });
  return Array.from(next).slice(-80);
}

function interactionDismissalSet(ids: string[]): Set<string> {
  const out = new Set<string>();
  ids.forEach((id) => {
    const cleanID = id.trim();
    if (cleanID) {
      out.add(cleanID);
    }
  });
  return out;
}

function interactionResolvedStatus(actionID: string, response: ChatResponse): string {
  const normalizedAction = actionID.trim().toLowerCase();
  const goalStatus = textValue(response.goal_status, "").toLowerCase();
  if (normalizedAction === "cancel" || goalStatus.includes("cancel")) {
    return "cancelled";
  }
  if (response.error || goalStatus.includes("fail") || goalStatus.includes("error")) {
    return "failed";
  }
  return "completed";
}

export function latestComposerInteraction(messages: ChatMessage[], dismissedIDs: string[] = []): JsonRecord | null {
  const dismissed = interactionDismissalSet(dismissedIDs);
  for (let messageIndex = messages.length - 1; messageIndex >= 0; messageIndex -= 1) {
    const actions = messages[messageIndex].actions ?? [];
    for (let actionIndex = actions.length - 1; actionIndex >= 0; actionIndex -= 1) {
      const action = asRecord(actions[actionIndex]);
      const actionID = textValue(action.id ?? action.interaction_id, "");
      if (isComposerInteraction(action) && !dismissed.has(actionID) && !dismissed.has(actionRenderID(action))) {
        return action;
      }
    }
  }
  return null;
}

// GUI-F3/F3：后台驻留交互的浮卡投影移入 interactionGuard.pendingInteractionFromContinuations。
// 调度侧续跑切片 park 在 waiting_interaction 时没有 HTTP 响应通道（AGENT-F5 补了
// interaction.pending 事件），投影从 runtime status 的 continuation pending_interaction
// 兜底——覆盖事件未达与刷新竞态两个窗口，应答后 pending 消失即自动撤卡；
// F3 起按 conversation_id 归属过滤，他会话的 stale pending 卡不再挤占新会话 composer。

export function isComposerInteraction(action: JsonRecord): boolean {
	if (textValue(action._ui_source, "") !== "interaction") {
    return false;
  }
  if (isCapabilityProposalInteraction(action)) {
    return false;
  }
  const id = textValue(action.id ?? action.interaction_id, "");
  const status = textValue(action.status ?? action.stage, "").toLowerCase();
  const kind = textValue(action.kind, "").toLowerCase();
  const type = textValue(action.type, "").toLowerCase();
  const childActions = firstArray(action.actions).map(asRecord).filter((item) => Object.keys(item).length > 0);
  if (!id || childActions.length === 0) {
    return false;
  }
  if (
    // F3 死卡守卫：已消费/过期口径与 server "这个交互已处理或已过期"
    // （server.go:4022）对齐——水合盖章（resolved）与既有状态形态
    // （responded/expired 等）一律不再视为可交互。
    isConsumedInteractionStatus(status) ||
    status.includes("complete") ||
    status.includes("done") ||
    status.includes("cancel") ||
    status.includes("fail") ||
    status.includes("error") ||
    kind === "mode_boundary" ||
    type === "mode_boundary"
  ) {
    return false;
  }
  return true;
}

export function isCapabilityProposalInteraction(action: JsonRecord): boolean {
  const payload = interactionPayload(action);
  const presentation = asRecord(payload.proposal_presentation ?? action.proposal_presentation);
  const kind = textValue(action.kind, "").toLowerCase();
  const type = textValue(action.type, "").toLowerCase();
  return kind === "proposal_approval" || type === "proposal_approval" || textValue(presentation.schema_version, "") === "vit.proposal_presentation.v1";
}

export function hasPendingComposerInteraction(messages: ChatMessage[]): boolean {
  return messages.some((message) => (message.actions ?? []).map(asRecord).some(isComposerInteraction));
}
