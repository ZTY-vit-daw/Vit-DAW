// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2b）：App.tsx MessageStream+AssistantTextContent（H 域）出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
import { Activity, AlertTriangle, Bot, Loader2 } from "lucide-react";
import { Fragment, useEffect, useRef } from "react";
import type { AgentMode, AgentUIState, AuthorityMode, ChatMessage, JsonRecord, MacroControl } from "../types";
import type { DawInvoke } from "../daw/parse";
import { asRecord, textValue } from "../lib/util";
import type { AuditionJudgmentPayload } from "../lib/api";
import { auditionSessions, type AuditionSession, type AuditionState } from "../audition";
import { auditionSessionNativeTurnID } from "../messageLifecycle";
import { trajectoryTurns, type TrajectoryState } from "../trajectory";
import { buildMessageStreamRenderPlan, type MessageStreamEntry } from "../trace/renderPlan";
import type { TurnEventMetaMap } from "../trace/turnEventMeta";
import { roundActivityBoundKeys, roundTurnShell, type RoundStepMap } from "../trace/roundSteps";
import type { TurnReceipt } from "../trace/turnReceipts";
import { TurnReceiptRow } from "../trace/TurnReceiptRow";
import { laneVisibleActivities } from "../trace/turnGroups";
import { TraceBlock, OptimisticTraceBlock, shouldShowOptimisticTrace } from "../trace/TraceBlock";
import { hasLayeredPresentation, ObservePresentationView } from "../observePresentationLayered";
import { observeOutputLayout, shouldLayerObserveOutput } from "../observeOutputLayering";
import { AuditionJudgeCard } from "../trajectory/TrajectoryAuditionPanel";
import { ActionCards } from "../actions/ActionCards";
import { actionRenderID } from "../actions/StandardActionCard";
import { isConfirmationAction } from "../actions/confirmation";
import { isDisposableConfirmationPromptText } from "../actions/StandardActionCard";
import { isMixPlanningDiscussionAction } from "../actions/MixBoardActionCard";
import { shouldSuppressProposalContent } from "../actions/CapabilityProposalCard";
import { localizeDisplayText } from "../actions/labels";
import { ArtifactContextCard } from "../media/artifacts";

export function MessageStream({
  messages,
  activities,
  trajectory,
  turnEventMeta,
  roundSteps,
  receipts,
  audition,
  auditionBusySessionID,
  authorityMode,
  respondingActionID,
  isSending,
  hiddenActionID,
  bottomInset,
  onInteractionAction,
  onInvoke,
  onSelectArtifact,
  uiState,
  onMacroValuePreview,
  onMacroValueCommit,
  onAuditionSelect,
  onAuditionStop,
  onSubmitAuditionJudgment,
  onSendSupplement
}: {
  messages: ChatMessage[];
  activities: ChatMessage[];
  trajectory: TrajectoryState;
  turnEventMeta: TurnEventMetaMap;
  /** TRAJ-IMPL-2：回合内 item 步账（容器证据 + 步混排 + 活动线去重） */
  roundSteps: RoundStepMap;
  /** TRAJ-IMPL-3：终局回执行台账行（活态缺失回合的水合收据，同键活态优先） */
  receipts: TurnReceipt[];
  audition: AuditionState;
  auditionBusySessionID: string;
  authorityMode: AuthorityMode;
  respondingActionID: string;
  isSending: boolean;
  hiddenActionID: string;
  bottomInset: number;
  onInteractionAction: (interaction: JsonRecord, action: JsonRecord, payload?: JsonRecord) => void;
  onInvoke: DawInvoke;
  onSelectArtifact: (id: string) => void;
  uiState: AgentUIState | null;
  onMacroValuePreview: (macro: MacroControl, value: number) => void;
  onMacroValueCommit: (macro: MacroControl, value: number) => Promise<void>;
  onAuditionSelect: (sessionID: string, candidateID: string) => Promise<void>;
  onAuditionStop: (sessionID: string) => Promise<void>;
  onSubmitAuditionJudgment?: (payload: AuditionJudgmentPayload) => Promise<void>;
  onSendSupplement?: (text: string) => void;
}) {
  const bottomRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const frameID = window.requestAnimationFrame(() => {
      bottomRef.current?.scrollIntoView({ block: "end", behavior: "smooth" });
    });
    return () => window.cancelAnimationFrame(frameID);
  }, [activities.length, messages.length, respondingActionID, bottomInset]);

  const renderMessage = (message: ChatMessage) => {
    const modeLabel = agentModeLabel(message.mode);
    const actionsBeforeContent = shouldRenderActionsBeforeContent(message);
    const suppressProposalContent = shouldSuppressProposalContent(message);
    const contentBlock = suppressProposalContent
      ? null
      : message.status === "pending" && message.role === "assistant"
        ? <TypingMessage content={message.content} />
        : <AssistantTextContent message={message} />;
    const actionCardsBlock = message.actions && message.actions.length > 0
      ? (
          <ActionCards
            actions={message.actions}
            respondingActionID={respondingActionID}
            hiddenActionID={hiddenActionID}
            onInteractionAction={onInteractionAction}
            onInvoke={onInvoke}
            onSelectArtifact={onSelectArtifact}
            uiState={uiState}
            onMacroValuePreview={onMacroValuePreview}
            onMacroValueCommit={onMacroValueCommit}
            authorityMode={authorityMode}
            onSendSupplement={onSendSupplement}
          />
        )
      : null;
    return (
      <article key={message.id} className={`message-row ${message.role} ${message.status ?? ""}`}>
        <div className="message-avatar">{message.role === "user" ? <Activity size={16} /> : <Bot size={16} />}</div>
        <div className="message-body">
          <div className="message-meta">
            <span>{messageRoleLabel(message.role)}</span>
            {modeLabel && <b>{modeLabel}</b>}
          </div>
          {actionsBeforeContent && actionCardsBlock}
          {contentBlock}
          {message.artifacts && message.artifacts.length > 0 && (
            <div className="artifact-context-list message-context-list">
              {message.artifacts.map((artifact) => (
                <ArtifactContextCard key={artifact.id} artifact={artifact} variant="message" onSelect={() => onSelectArtifact(artifact.id)} />
              ))}
            </div>
          )}
          {!actionsBeforeContent && actionCardsBlock}
        </div>
      </article>
    );
  };

  // 回合分组渲染（GUI-1/G2：分组+锚定+孤儿+终局顺序抽为纯函数 buildMessageStreamRenderPlan，
  // 固化 用户消息→中间汇报→轨迹块→…→终局回复 的顺序，设计基线 GUI-T2/T3 + GUI-F8）
  const visibleMessages = messages.filter((message) => !shouldHideMessageForComposerOverlay(message, hiddenActionID));
  // TRAJ-IMPL-3：台账行一并进计划——活态中不存在的回合渲染静态收起收据行（同键去重
  // 在计划入口完成），落位复用同一个两级锚定。
  const plan = buildMessageStreamRenderPlan({ messages: visibleMessages, trajectory, turnEventMeta, roundSteps, receipts });
  const knownTurnIds = new Set(trajectoryTurns(trajectory).map((turn) => turn.id));
  const turnActivities = (turnId: string) => activities.filter((activity) => (activity.turn_id ?? "").trim() === turnId);
  const sessions = auditionSessions(audition);
  // 活动线只承载非回合活动（上传/调用等）；回合内活动并入回合单活动面。TRAJ-IMPL-2
  // 把「归属」判据从回合 id 扩到**事件身份**（item 步账的钥匙）：item 已经落在某个
  // 回合容器里，就不该在流底 lane 再出现一次（设计 §2.1-5）；turn_id 跨命名空间的
  // 直连活动（如上传）照旧留在 lane。
  const roundBoundKeys = roundActivityBoundKeys(roundSteps);
  // AUDITION-LANE-1（2026-09-14）：audition.* 活动行归属会话回合域（判定卡锚定域，
  // messageLifecycle.activityTurnIDOfEvent 与 audition.ts:93 同键）——该族活动的呈现
  // 面是判定卡区域（准备/忙碌态卡面已显形，AUDITION-UNSTICK-1），不再落流底 lane；
  // 无回合归属的活动（上传等）照旧留 lane。
  // FIX-AUDITION-TRAIL-1（2026-09-29）：回合域匹配漏了无 turn 域的内核遥测行（快照
  // 无 turn_id，agent 侧 enrich 事件到达前未绑定）——处理期多条「Kernel audition」
  // 重复落 lane。排除改按族身份（laneVisibleActivities 内 isAuditionFamilyActivity），
  // 判定卡是族内唯一动态表面（每会话一卡、随状态更新、判定后定型）。
  const sessionBoundTurnIds = new Set(sessions.map((session) => session.turnID).filter(Boolean));
  const laneBoundTurnIds = new Set([...knownTurnIds, ...sessionBoundTurnIds]);
  const laneActivities = laneVisibleActivities(activities, laneBoundTurnIds, roundBoundKeys);

  const turnFirstSeq = (turnId: string): number => {
    const turn = trajectory.turns[turnId];
    if (!turn || turn.nodeIds.length === 0) return Number.MAX_SAFE_INTEGER;
    return Math.min(...turn.nodeIds.map((id) => trajectory.nodes[id]?.seq ?? Number.MAX_SAFE_INTEGER));
  };
  // 免选路径：用户越过待裁卡在底部输入框继续了对话 → 卡片沉淀「卡面选项未采用」（supersedes 语义的 UI 呈现）
  // WEBUI-MSG-ORDER-2：turn 域缺失的会话（内核 audition::Session 无该字段）按
  // session_id 内嵌的原生轨迹域回查 trajectory 的 nativeTurnIds 账，映射到拥有
  // 它的 B9 轮次——映射不中保持空串（无证据不猜，走流尾兜底）。
  const nativeTurnOwnerBySessionNative = new Map<string, string>();
  for (const turn of trajectoryTurns(trajectory)) {
    for (const native of turn.nativeTurnIds ?? []) {
      if (native && !nativeTurnOwnerBySessionNative.has(native)) {
        nativeTurnOwnerBySessionNative.set(native, turn.id);
      }
    }
  }
  const sessionTurnID = (session: AuditionSession): string => {
    if (session.turnID) {
      return session.turnID;
    }
    return nativeTurnOwnerBySessionNative.get(auditionSessionNativeTurnID(session.id)) ?? "";
  };
  const sessionSuperseded = (session: AuditionSession): boolean => {
    if (session.judgmentRecorded) return false;
    const turnID = sessionTurnID(session);
    const baseSeq = turnFirstSeq(turnID);
    if (baseSeq !== Number.MAX_SAFE_INTEGER && Object.values(trajectory.turns).some((other) => other.id !== turnID && turnFirstSeq(other.id) > baseSeq)) {
      return true;
    }
    const entryIndex = plan.entries.findIndex((entry) => entry.kind === "trace" && entry.turnId === turnID);
    return entryIndex >= 0 && plan.entries.slice(entryIndex + 1).some((entry) => entry.kind === "messages" && entry.messages.some((message) => message.role === "user"));
  };
  const renderJudgeCard = (session: AuditionSession) => (
    <AuditionJudgeCard
      key={`audition:${session.id}`}
      trajectory={trajectory}
      session={session}
      busySessionID={auditionBusySessionID}
      superseded={sessionSuperseded(session)}
      onSelect={onAuditionSelect}
      onStop={onAuditionStop}
      onSubmitJudgment={onSubmitAuditionJudgment}
    />
  );
  // 试听判定卡挂靠（WEBUI-MSG-ORDER-2 三级）：
  //   ① 组尾挂靠（既有语义）：会话回合域命中某消息组键（turn:<id>），卡跟组尾条目后；
  //   ② 轨迹块跟随（本卡新增）：会话回合域未命中组键但该回合在计划中有轨迹块条目
  //      （自由态实验块身份不同源、槽位被占或无消息组的形态），卡直接跟在该块后
  //      ——卡与块同沉浮，新消息组自然排在它们下面；
  //   ③ 流尾兜底（既有语义）：回合域无法解析或两者皆不命中，挂流尾防丢（不猜）。
  // 旧判据（visibleMessages 的 turn_id 集合）被 ① 的组键判据取代：组键由同一批消息
  // 归组而来，二者对「组锚定是否命中」等价，而组键判据与渲染落位（entries）同源。
  const entryGroupKey = (entry: MessageStreamEntry): string =>
    entry.key.startsWith("orphan:") ? "" : entry.key.replace(/:(user|trace|rest|receipt)$/, "");
  const groupKeyTurnIds = new Set(
    plan.entries
      .map((entry) => entryGroupKey(entry))
      .filter((key) => key.startsWith("turn:"))
      .map((key) => key.slice("turn:".length))
  );
  const planTraceTurnIds = new Set(plan.entries.filter((entry) => entry.kind === "trace").map((entry) => entry.turnId));
  const resolvedSessionTurnID = new Map(sessions.map((session) => [session.id, sessionTurnID(session)]));
  const traceAnchoredSessions = sessions.filter((session) => {
    const turnID = resolvedSessionTurnID.get(session.id) ?? "";
    return Boolean(turnID) && !groupKeyTurnIds.has(turnID) && planTraceTurnIds.has(turnID);
  });
  const unanchoredSessions = sessions.filter((session) => {
    const turnID = resolvedSessionTurnID.get(session.id) ?? "";
    return !turnID || (!groupKeyTurnIds.has(turnID) && !planTraceTurnIds.has(turnID));
  });
  const sessionsForGroupKey = (groupKey: string) => {
    if (!groupKey.startsWith("turn:")) {
      return [];
    }
    const turnId = groupKey.slice("turn:".length);
    return sessions.filter((session) => (resolvedSessionTurnID.get(session.id) ?? "") === turnId);
  };

  return (
    <div className="message-stream">
      {plan.entries.map((entry, index) => {
        const groupKey = entryGroupKey(entry);
        const isGroupEnd = groupKey !== "" && (index + 1 >= plan.entries.length || entryGroupKey(plan.entries[index + 1]) !== groupKey);
        return (
          <Fragment key={entry.key}>
            {entry.kind === "messages" ? entry.messages.map(renderMessage) : entry.kind === "receipt" ? (
              // TRAJ-IMPL-3 §2.3：活态缺失回合的静态收起回执行（台账水合产物）
              <TurnReceiptRow receipt={entry.receipt} />
            ) : (() => {
              // TRAJ-IMPL-2 §2.1-2：无 trajectory 回合记录的回合（纯 chat 执行期）由 item
              // 步账撑起同一个容器——回合壳只承载身份与状态，轨迹步为空是事实而非缺陷。
              const round = roundSteps[entry.turnId];
              const turn = trajectory.turns[entry.turnId] ?? roundTurnShell(round);
              return turn ? (
                <TraceBlock
                  state={trajectory}
                  turn={turn}
                  activities={turnActivities(entry.turnId)}
                  authorityMode={authorityMode}
                  turnMeta={turnEventMeta[entry.turnId]}
                  itemSteps={round?.steps}
                  totalStepCount={round?.totalStepCount}
                />
              ) : null;
            })()}
            {isGroupEnd && sessionsForGroupKey(groupKey).map(renderJudgeCard)}
            {/* WEBUI-MSG-ORDER-2 ②：会话回合域无组键但该回合有轨迹块条目（自由态
                实验块）——卡跟块后，与块同沉浮（时序插列后新消息组排在它们下面） */}
            {entry.kind === "trace" &&
              traceAnchoredSessions.filter((session) => (resolvedSessionTurnID.get(session.id) ?? "") === entry.turnId).map(renderJudgeCard)}
          </Fragment>
        );
      })}
      {/* GUI-F8：终局回复在整个条目序列（含孤儿实验轨迹块）之后渲染（实验轨迹块 → 终局回复） */}
      {plan.chainResultMessages.map(renderMessage)}
      {/* 发送即显的乐观轨迹条：真 turn.started 事件到达后由上方真块无缝接管 */}
      {shouldShowOptimisticTrace({ isSending, trajectory, messages: visibleMessages }) && (
        <OptimisticTraceBlock key="optimistic-trace" />
      )}
      {/* 未挂靠回合组的试听会话判定卡同样挂流尾防丢 */}
      {unanchoredSessions.map(renderJudgeCard)}
      {laneActivities.length > 0 && (
        <section className="activity-lane" aria-label="即时活动" aria-live="polite">
          {laneActivities.map((activity) => (
            <div key={activity.id} className={`activity-lane-item ${activity.status ?? "pending"}`}>
              {activity.status === "error" ? <AlertTriangle size={14} /> : <Loader2 className="spin" size={14} />}
              <span>{activity.content}</span>
              {activity.status !== "error" && <span className="typing-dots" aria-hidden="true"><i /><i /><i /></span>}
            </div>
          ))}
        </section>
      )}
      <div ref={bottomRef} className="message-scroll-anchor" style={{ height: `${bottomInset}px` }} />
    </div>
  );
}

function shouldHideMessageForComposerOverlay(message: ChatMessage, hiddenActionID: string): boolean {
  if (!hiddenActionID || message.artifacts?.length || !isDisposableConfirmationPromptText(message.content)) {
    return false;
  }
  return (message.actions ?? []).map(asRecord).some((action) => actionRenderID(action) === hiddenActionID && isConfirmationAction(action));
}

function shouldRenderActionsBeforeContent(message: ChatMessage): boolean {
  if (message.role !== "assistant") {
    return false;
  }
  return (message.actions ?? []).map(asRecord).some(isMixPlanningDiscussionAction);
}

function TypingMessage({ content }: { content: string }) {
  return (
    <p className="typing-message">
      <span>{content}</span>
      <span className="typing-dots" aria-hidden="true">
        <i />
        <i />
        <i />
      </span>
    </p>
  );
}

// OPT-OBSERVE-OUTPUT-1 P1（2026-09-30）：观察问答输出分层——长助手文本默认折叠。
// 命中 observeOutputLayering 谓词的消息渲染为「首段可见 + details 展开全部」；
// 展开后 lead+rest 拼回完整原文（无重复段），与现行 <p> pre-wrap 渲染同构。
// 谓词与阈值集中在 observeOutputLayering.ts（设计 §5），此处只做折叠容器；
// details 原生收纳展开态（纯 UI 态，不进 content/持久化，刷新后默认态重算）。
// OPT-IMPL-2 P2（2026-10-07）：有效 presentation 块优先——真三层（摘要常显/
// 证据折叠/完整原文深折叠，observePresentationLayered.tsx）；无块回落 P1 谓词。
export function AssistantTextContent({ message }: { message: ChatMessage }) {
  if (hasLayeredPresentation(message)) {
    return <ObservePresentationView message={message} />;
  }
  if (!shouldLayerObserveOutput(message)) {
    return <p>{message.content}</p>;
  }
  const layout = observeOutputLayout(message.content);
  if (!layout) {
    return <p>{message.content}</p>;
  }
  return (
    <div className="observe-output-layered">
      <p className="observe-output-lead">{layout.lead}</p>
      <details className="observe-output-details">
        <summary>展开全部 {layout.hiddenLineCount} 行</summary>
        <p>{layout.rest}</p>
      </details>
    </div>
  );
}
function messageRoleLabel(role: ChatMessage["role"]): string {
  if (role === "user") {
    return "用户";
  }
  if (role === "system") {
    return "系统";
  }
  return "Vit";
}

function agentModeLabel(mode?: AgentMode | string): string {
  const value = textValue(mode, "").toLowerCase();
  if (value === "default") {
    return "";
  }
  if (value === "plan") {
    return "计划";
  }
  if (value === "goal") {
    return "目标";
  }
  return localizeDisplayText(value);
}

