// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2a）：App.tsx 动作卡体系全域出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
// ActionCards 出口栅格 + ActionCard 卡型路由。

import type { AgentUIState, AuthorityMode, JsonRecord, MacroControl } from "../types";
import type { DawInvoke } from "../daw/parse";
import { textValue } from "../lib/util";
import { DeclarativeInteractionCard, declarativeCardFromAction } from "../interactionCardKit";
import { isMacroControlAction } from "../macro/normalize";
import { MacroActionCard } from "../App";
import { actionTitle } from "./labels";
import { isMixTreatmentPendingAction } from "./confirmation";
import { CapabilityProposalCard } from "./CapabilityProposalCard";
import { MixBoardActionCard, MixTreatmentPendingCard, isMixBoardAction } from "./MixBoardActionCard";
import { ProjectResultCard, isProjectResultAction } from "./ProjectResultCard";
import { StandardActionCard, actionRenderID, isCapabilityProposalInteraction } from "./StandardActionCard";
import type { ActionCardProps } from "./types";

export function ActionCards({
  actions,
  respondingActionID,
  hiddenActionID = "",
  onInteractionAction,
  onInvoke,
  onSelectArtifact,
  uiState,
  onMacroValuePreview,
  onMacroValueCommit,
  authorityMode,
  onSendSupplement
}: {
  actions: JsonRecord[];
  respondingActionID: string;
  hiddenActionID?: string;
  onInteractionAction: (interaction: JsonRecord, action: JsonRecord, payload?: JsonRecord) => void;
  onInvoke: DawInvoke;
  onSelectArtifact: (id: string) => void;
  uiState: AgentUIState | null;
  onMacroValuePreview: (macro: MacroControl, value: number) => void;
  onMacroValueCommit: (macro: MacroControl, value: number) => Promise<void>;
  authorityMode?: AuthorityMode;
  onSendSupplement?: (text: string) => void;
}) {
  const visibleActions = actions
    .slice(0, 6)
    .filter((action) => !hiddenActionID || actionRenderID(action) !== hiddenActionID);
  if (visibleActions.length === 0) {
    return null;
  }
  return (
    <div className="action-list">
      {visibleActions.map((action, index) => (
        <ActionCard
          key={`${textValue(action._ui_source, "")}-${actionRenderID(action) || actionTitle(action, index)}-${index}`}
          action={action}
          index={index}
          respondingActionID={respondingActionID}
          onInteractionAction={onInteractionAction}
          onInvoke={onInvoke}
          onSelectArtifact={onSelectArtifact}
          uiState={uiState}
          onMacroValuePreview={onMacroValuePreview}
          onMacroValueCommit={onMacroValueCommit}
          authorityMode={authorityMode}
          onSendSupplement={onSendSupplement}
        />
      ))}
    </div>
  );
}
export function ActionCard(props: ActionCardProps) {
  if (isCapabilityProposalInteraction(props.action)) {
    return <CapabilityProposalCard {...props} />;
  }
  if (isMixBoardAction(props.action)) {
    return <MixBoardActionCard {...props} />;
  }
  if (isMixTreatmentPendingAction(props.action)) {
    return <MixTreatmentPendingCard {...props} />;
  }
  if (isProjectResultAction(props.action)) {
    return <ProjectResultCard {...props} />;
  }
  if (isMacroControlAction(props.action)) {
    return (
      <MacroActionCard
        action={props.action}
        uiState={props.uiState}
        respondingActionID={props.respondingActionID}
        onMacroValuePreview={props.onMacroValuePreview}
        onMacroValueCommit={props.onMacroValueCommit}
      />
    );
  }
  // 通用声明式交互卡（FE-INTERACTION-CARD-KIT-1）：payload 携带 interaction_
  // card.v1 契约的卡按声明渲染（字段控件+动作按钮），不再逐卡加分支。仅
  // 声明了 card 契约的卡走此路——既有特制卡分支零改动。
  const declarativeCard = declarativeCardFromAction(props.action);
  if (declarativeCard) {
    return (
      <DeclarativeInteractionCard
        action={props.action}
        card={declarativeCard}
        respondingActionID={props.respondingActionID}
        onInteractionAction={props.onInteractionAction}
      />
    );
  }
  return <StandardActionCard {...props} />;
}
