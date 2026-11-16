// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2a）：App.tsx 动作卡体系全域出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
// MixTreatmentPendingCard + MixBoardActionCard 混音卡全家。

import { Activity, Loader2, SlidersHorizontal } from "lucide-react";
import { useRef, useState } from "react";
import type { JsonRecord } from "../types";
import { asRecord, firstArray, textValue } from "../lib/util";
import { numberFromKeys, textFromKeys } from "../daw/parse";
import { isSameMixBoardSession, mixBoardActionSessionID } from "../chat/merge";
import { WEBUI_BUILD_MARK, mixBoardUserNoteDrafts, interactionActionID, KeyValue } from "../App";
import { actionRenderID } from "./StandardActionCard";
import { interactionPayload, interactionPayloadForAction, isConfirmationAction, mixTreatmentPendingLabel, mixTreatmentPendingValueLabel } from "./confirmation";
import { localizeDisplayText, localizeMixTreatmentCardText } from "./labels";
import type { ActionCardProps } from "./types";

export function MixTreatmentPendingCard({ action, respondingActionID, onInteractionAction }: ActionCardProps) {
  const payload = interactionPayload(action);
  const display = asRecord(payload.display ?? action.display);
  const targetRef = textValue(display.target_ref ?? payload.display_target_ref ?? payload.target_ref ?? action.target_ref, "当前对象");
  const actionKind = textValue(payload.action_kind ?? action.action_kind, "");
  const processor = textValue(payload.processor_type ?? action.processor_type, "");
  const deltaDB = textValue(payload.delta_db ?? action.delta_db, "");
  const deltaPan = textValue(payload.delta_pan ?? action.delta_pan, "");
  const targetPan = textValue(payload.target_pan ?? action.target_pan, "");
  const confidence = textValue(display.confidence ?? payload.display_confidence ?? payload.confidence ?? action.confidence, "");
  const reason = textValue(display.reasoning_summary ?? payload.display_reasoning_summary ?? payload.reasoning_summary ?? action.reasoning_summary ?? action.body, "");
  const childActions = firstArray(action.actions)
    .map(asRecord)
    .filter((item) => Object.keys(item).length > 0);
  const interactionID = textValue(action.id ?? action.interaction_id, "");
  const renderID = actionRenderID(action);
  const pendingLabel = textValue(display.action_kind, "") || mixTreatmentPendingLabel(actionKind, processor);
  const valueLabel = mixTreatmentPendingValueLabel(actionKind, deltaDB, deltaPan, targetPan);
  const rows = [
    ["对象", targetRef],
    ["建议", pendingLabel],
    ["幅度", valueLabel],
    ["置信度", confidence ? localizeDisplayText(confidence) : "待确认"]
  ].filter(([, value]) => value !== "");

  return (
    <div className="action-card mix-treatment-pending-card">
      <SlidersHorizontal size={16} />
      <div className="action-content">
        <div className="action-title-line">
          <strong>混音建议待确认</strong>
          <span>待确认</span>
        </div>
        {reason && <p className="action-body">{localizeMixTreatmentCardText(reason)}</p>}
        <div className="action-detail-grid mix-treatment-pending-grid">
          {rows.map(([label, value]) => (
            <div className="action-detail" key={label}>
              <span>{label}</span>
              <strong>{value}</strong>
            </div>
          ))}
        </div>
        {childActions.length > 0 && (
          <div className="action-buttons">
            {childActions.map((child, childIndex) => {
              const actionID = textValue(child.id ?? child.action_id ?? child.decision, `action_${childIndex + 1}`);
              const label = localizeMixTreatmentCardText(textValue(child.label ?? child.title ?? actionID, actionID));
              const pendingID = interactionActionID(interactionID || renderID || "mix_treatment", actionID);
              const isBusy = respondingActionID === pendingID;
              const style = textValue(child.style, "secondary");
              return (
                <button
                  key={`${actionID}-${childIndex}`}
                  className={`action-button ${style}`}
                  type="button"
                  disabled={respondingActionID !== ""}
                  onClick={() => onInteractionAction(action, child, interactionPayloadForAction(action, child, {}))}
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

export function MixBoardActionCard({ action, respondingActionID, onInteractionAction }: ActionCardProps) {
  const payload = interactionPayload(action);
  const session = asRecord(payload.mix_session ?? action.mix_session);
  const observationEnvelope = asRecord(payload.mix_observation ?? action.mix_observation);
  const observation = asRecord(observationEnvelope.observation);
  const mixboard = asRecord(observationEnvelope.mixboard);
  const contextPack = asRecord(observationEnvelope.context_pack);
  const sessionHeader = asRecord(contextPack.session_header);
  const latestObservation = asRecord(contextPack.latest_observation);
  const target = asRecord(session.target_ref ?? observation.target_ref ?? mixboard.target_ref ?? sessionHeader.target_ref);
  const mixObjects = firstArray(session.mix_objects, observation.mix_objects, mixboard.mix_objects, sessionHeader.mix_objects).map(asRecord);
  const listenScope = asRecord(session.listen_scope ?? observation.listen_scope ?? mixboard.listen_scope ?? sessionHeader.listen_scope);
  const timeRuler = asRecord(observation.time_ruler ?? asRecord(contextPack.latest_observation).time_ruler);
  const sourceCapabilities = asRecord(observation.source_capabilities ?? latestObservation.source_capabilities);
  const globalSummary = asRecord(observation.global_summary ?? latestObservation.global_summary);
  const environmentPackage = asRecord(observation.environment_package ?? latestObservation.environment_package);
  const mixPackage = asRecord(observation.mix_package ?? latestObservation.mix_package);
  const deepPackage = asRecord(observation.deep_package ?? latestObservation.deep_package);
  const mixSourceCapabilities = asRecord(mixPackage.source_capabilities);
  const deepSourceCapabilities = asRecord(deepPackage.source_capabilities);
  const featureSnapshot = asRecord(globalSummary.feature_snapshot);
  const latestFeatureRequest = asRecord(featureSnapshot.latest_request);
  const snapshotWaveform = asRecord(featureSnapshot.waveform_envelope);
  const snapshotSpectrum = asRecord(featureSnapshot.spectrogram_tiles);
  const activeProblems = firstArray(mixboard.active_problem_map, contextPack.active_problem_map, observation.hotspots);
  const openBlockers = firstArray(mixboard.open_blockers, contextPack.open_blockers);
  const state = textValue(session.state ?? mixboard.status ?? observation.status, "unknown");
  const baseAcousticReady = textValue(sourceCapabilities.waveform_envelope, "").toLowerCase() === "ready";
  const effectiveBlockers = baseAcousticReady
    ? openBlockers.filter((item) => {
        const text = textValue(item, "").toLowerCase();
        return !(text.includes("waveform ready") && text.includes("spectrum"));
      })
    : openBlockers;
  const rawObservationStatus = textValue(observationEnvelope.status ?? observation.status ?? mixboard.status, state);
  const observationStatus = baseAcousticReady && rawObservationStatus === "partial" ? "ready" : rawObservationStatus;
  const blocker = textValue(effectiveBlockers[0] ?? (baseAcousticReady ? "" : session.blocking_point), "");
  const boardPath = textValue(observationEnvelope.board_path, "");
  const observationPath = textValue(observationEnvelope.observation_path, "");
  const contextPackPath = textValue(observationEnvelope.context_pack_path, "");
  const duration = numberFromKeys(timeRuler, "duration_seconds");
  const segment = numberFromKeys(timeRuler, "segment_seconds");
  const goalText = textValue(session.goal_text ?? mixboard.goal_text ?? sessionHeader.goal_text ?? payload.goal_text, "");
  const targetLabel = mixBoardTargetLabel(target);
  const statusLabel = mixObservationStatusLabel(observationStatus || state);
  const tone = observationStatus === "ready" ? "success" : observationStatus === "partial" ? "attention" : observationStatus === "unavailable" ? "error" : "info";
  const childActions = firstArray(action.actions)
    .map(asRecord)
    .filter((item) => Object.keys(item).length > 0)
    .filter((item) => textValue(item.id ?? item.action_id ?? item.decision, "") !== "done");
  const interactionID = textValue(action.id ?? action.interaction_id, "");
  const renderID = actionRenderID(action);
  const sessionID = mixBoardActionSessionID(action);
  const draftKey = sessionID || renderID || interactionID || "latest";
  const existingUserNote = mixBoardUserNoteLabel(session, mixboard, payload);
  const existingDraft = mixBoardUserNoteDrafts.get(draftKey);
  const [userNoteDraft, setUserNoteDraft] = useState(
    existingDraft ?? (existingUserNote === "等待用户听感，可在下一轮开始前补充" ? "" : existingUserNote)
  );
  const userNoteInputRef = useRef<HTMLTextAreaElement | null>(null);
  const latestTurn = mixBoardLatestTuneTurn(mixboard, observation, payload);
  const tuneRows = mixBoardTuneRows(latestTurn, mixboard);
  const payloadForMixAction = (): JsonRecord => {
    const note = (userNoteInputRef.current?.value ?? mixBoardUserNoteDrafts.get(draftKey) ?? userNoteDraft).trim();
    const outgoing: JsonRecord = {
      ...payload,
      mix_session: session,
      mix_observation: observationEnvelope,
      request_context: asRecord(payload.request_context),
      conversation_id: textValue(action.conversation_id, ""),
      goal_id: textValue(action.goal_id, ""),
      run_id: textValue(action.run_id, ""),
      _mixboard_client_build: WEBUI_BUILD_MARK
    };
    if (note) {
      outgoing.user_note = note;
      outgoing.fields = {
        ...asRecord(outgoing.fields),
        user_note: note
      };
    }
    console.info("[VitWebUI] MixBoard submit", {
      build: WEBUI_BUILD_MARK,
      session_id: sessionID,
      interaction_id: interactionID,
      draft_key: draftKey,
      user_note: note,
      payload: outgoing
    });
    return outgoing;
  };
  const mainRows = [
    ["混音目标", goalText || "等待当前轮次目标"],
    ["混音对象", mixBoardObjectsLabel(mixObjects, target)],
    ["试听范围", mixBoardListenScopeLabel(listenScope, duration)],
    ["用户听感 / 干预", existingUserNote],
    ["当前判断", mixBoardJudgementLabel(activeProblems, observationStatus, blocker)],
    ["当前动作", mixBoardCurrentActionLabel(session, mixboard, observation, payload, observationStatus)],
    ["下一步", mixBoardNextStepLabel(session, mixboard, payload, observationStatus, blocker)]
  ];
  const traffic = mixBoardTrafficState(observationStatus || state);
  const diagnostics = [
    ["UI", WEBUI_BUILD_MARK],
    ["Session", textValue(session.mix_session_id ?? observation.mix_session_id ?? mixboard.mix_session_id, "-")],
    ["Observation", textValue(observation.observation_id ?? observationEnvelope.observation_id ?? mixboard.latest_observation_id, "-")],
    ["时间尺", duration !== null && duration > 0 ? `${duration.toFixed(2)}s` : "待接入"],
    ["分段", segment !== null && segment > 0 ? `${segment.toFixed(2)}s` : "-"],
    ["环境包", mixBoardPackageStatusLabel(textValue(environmentPackage.status, "-"))],
    ["混音包", mixBoardPackageStatusLabel(textValue(mixPackage.status, "-"))],
    ["深度包", mixBoardPackageStatusLabel(textValue(deepPackage.status, "-"))],
    ["Waveform", mixObservationStatusLabel(textValue(sourceCapabilities.waveform_envelope, "missing"))],
    ["实时频段", mixObservationStatusLabel(textValue(mixSourceCapabilities.band_energy ?? sourceCapabilities.band_energy, "missing"))],
    ["声像/相干", mixObservationStatusLabel(textValue(mixSourceCapabilities.stereo_correlation ?? sourceCapabilities.stereo_relation, "missing"))],
    ["调参复查", mixObservationStatusLabel(textValue(mixSourceCapabilities.before_after_delta, "missing"))],
    ["深度频谱(异步)", mixObservationStatusLabel(textValue(deepSourceCapabilities.deep_band_observation ?? sourceCapabilities.spectrogram_tiles, "missing"))],
    ["Request", mixBoardFeatureRequestLabel(latestFeatureRequest)],
    ["Feature", textValue(featureSnapshot.updated_at, "-")],
    ["Peak/RMS", mixBoardPeakRmsLabel(globalSummary)],
    ["Board", boardPath || "-"],
    ["Observation", observationPath || "-"],
    ["Context", contextPackPath || "-"]
  ];

  console.info("[VitWebUI] MixBoardActionCard render", {
    build: WEBUI_BUILD_MARK,
    state,
    observationStatus,
    target: targetLabel,
    rows: mainRows.map(([label, value]) => ({ label, value })),
    boardPath,
    observationPath,
    contextPackPath
  });

  return (
    <div className={`action-card ${tone} mix-board-card`}>
      <Activity size={16} />
      <div className="action-content">
        <div className="action-title-line">
          <strong>MixBoard</strong>
          <span>{statusLabel}</span>
        </div>
        <p className="action-body">
          {targetLabel} 路 {mixSessionStateLabel(state)}
        </p>
        <label className="mix-board-intervention">
          <span>本轮听感干预</span>
          <textarea
            ref={userNoteInputRef}
            value={userNoteDraft}
            placeholder="例如：不要再降低音量，低频有点糊"
            rows={2}
            disabled={respondingActionID !== ""}
            onChange={(event) => {
              const next = event.currentTarget.value;
              mixBoardUserNoteDrafts.set(draftKey, next);
              setUserNoteDraft(next);
            }}
          />
        </label>
        <div className="action-detail-grid project-result-grid mix-board-main-grid">
          {mainRows.map(([label, value]) => (
            <div className="action-detail" key={label}>
              <span>{label}</span>
              <strong>{value}</strong>
            </div>
          ))}
          <div className="action-detail mix-board-status-detail">
            <span>观察状态</span>
            <strong>
              <i className={`mix-board-status-dot ${traffic}`} aria-hidden="true" />
              {mixBoardTrafficLabel(observationStatus || state, blocker)}
            </strong>
          </div>
        </div>
        {tuneRows.length > 0 && (
          <div className="mix-board-tune-panel">
            <div className="mix-board-panel-title">
              <SlidersHorizontal size={14} />
              <strong>调参轮次</strong>
            </div>
            <div className="action-detail-grid project-result-grid">
              {tuneRows.map(([label, value]) => (
                <div className="action-detail" key={label}>
                  <span>{label}</span>
                  <strong>{value}</strong>
                </div>
              ))}
            </div>
          </div>
        )}
        {childActions.length > 0 && (
          <div className="action-buttons mix-board-actions">
            {childActions.map((child, index) => {
              const actionID = textValue(child.id ?? child.action_id ?? child.decision, `action_${index + 1}`);
              const label = textValue(child.label ?? child.title ?? actionID, actionID);
              const style = textValue(child.style, "secondary");
              const pendingID = interactionActionID(interactionID || renderID || "mixboard", actionID);
              const isBusy = respondingActionID === pendingID;
              const disabled = respondingActionID !== "";
              return (
                <button
                  key={`${actionID}-${index}`}
                  className={`action-button ${style}`}
                  type="button"
                  disabled={disabled}
                  onClick={() => onInteractionAction(action, child, payloadForMixAction())}
                >
                  {isBusy && <Loader2 className="spin" size={14} />}
                  <span>{label}</span>
                </button>
              );
            })}
          </div>
        )}
        <details className="mix-board-diagnostics">
          <summary>诊断信息</summary>
          <div className="action-detail-grid project-result-grid">
            {diagnostics.map(([label, value], index) => (
              <KeyValue label={label} value={value} key={`${label}-${index}`} />
            ))}
          </div>
        </details>
      </div>
    </div>
  );
}

function mixBoardLatestTuneTurn(mixboard: JsonRecord, observation: JsonRecord, payload: JsonRecord): JsonRecord {
  const rows = firstArray(mixboard.auto_tune_turns, observation.auto_tune_turns, payload.auto_tune_turns).map(asRecord);
  return rows.length > 0 ? rows[rows.length - 1] : {};
}

function mixBoardTuneRows(turn: JsonRecord, mixboard: JsonRecord): Array<[string, string]> {
  const rows: Array<[string, string]> = [];
  const turnNumber = numberFromKeys(turn, "turn");
  const control = textValue(turn.control_label, "") || mixBoardControlLabel(textValue(turn.control, ""));
  const target = textValue(turn.target_label ?? turn.track_label ?? turn.track_id, "");
  const before = numberFromKeys(turn, "before_db");
  const after = numberFromKeys(turn, "target_db", "after_db");
  const step = numberFromKeys(turn, "step_db");
  const status = textValue(turn.status, "");
  const review = textValue(turn.review_status ?? mixboard.review_status, "");
  rows.push(["执行器", mixBoardExecutorLabel(textValue(turn.executor_type ?? mixboard.executor_type, ""), textValue(turn.executor_version ?? mixboard.executor_version, ""))]);
  if (turnNumber !== null) {
    rows.push(["轮次", `第 ${turnNumber} 轮`]);
  }
  if (target) {
    rows.push(["调控对象", target]);
  }
  if (control) {
    rows.push(["参数", control]);
  }
  if (before !== null && after !== null) {
    rows.push(["参数变化", `${before.toFixed(2)} dB -> ${after.toFixed(2)} dB`]);
  }
  if (step !== null) {
    rows.push(["步进", `${step > 0 ? "+" : ""}${step.toFixed(2)} dB`]);
  }
  const direction = textValue(turn.direction ?? mixboard.mix_tuning_direction, "");
  if (direction) {
    rows.push(["方向", mixBoardTuneDirectionLabel(direction)]);
  }
  const source = textValue(turn.direction_source ?? mixboard.mix_tuning_direction_source, "");
  if (source) {
    rows.push(["方向来源", mixBoardTuneDirectionSourceLabel(source)]);
  }
  const userNote = textValue(turn.user_note ?? mixboard.user_note ?? mixboard.mix_tuning_user_note, "");
  if (userNote) {
    rows.push(["用户干预", userNote]);
  }
  if (status) {
    rows.push(["执行状态", mixBoardTuneStatusLabel(status)]);
  }
  if (review) {
    rows.push(["复查状态", mixBoardReviewStatusLabel(review)]);
  }
  if (rows.length === 0) {
    const summary = textValue(mixboard.last_action_summary, "");
    if (summary) {
      rows.push(["最近动作", summary]);
    }
  }
  return rows;
}

function mixBoardControlLabel(value: string): string {
  const control = value.toLowerCase();
  if (control === "track.volume") {
    return "轨道音量";
  }
  return value;
}

function mixBoardExecutorLabel(type: string, version: string): string {
  const normalized = type.toLowerCase();
  if (normalized === "fallback_executor") {
    return version ? `Fallback 小步调参 · ${version}` : "Fallback 小步调参";
  }
  if (normalized === "llm_executor") {
    return version ? `LLM 调参执行器 · ${version}` : "LLM 调参执行器";
  }
  return version || type || "未指定";
}

function mixBoardTuneDirectionLabel(value: string): string {
  const direction = value.toLowerCase();
  if (direction === "raise_volume") {
    return "增大音量";
  }
  if (direction === "lower_volume") {
    return "降低音量";
  }
  if (direction === "hold") {
    return "保持不动";
  }
  return localizeDisplayText(value) || value;
}

function mixBoardTuneDirectionSourceLabel(value: string): string {
  const source = value.toLowerCase();
  if (source === "user_note") {
    return "用户干预";
  }
  if (source === "goal_text") {
    return "混音目标";
  }
  if (source === "explicit_step_db") {
    return "鏄惧紡步进";
  }
  if (source === "fallback_default") {
    return "默认小步";
  }
  if (source === "fallback_rule_or_user_note") {
    return "规则判断";
  }
  return localizeDisplayText(value) || value;
}

function mixBoardTuneStatusLabel(value: string): string {
  const status = value.toLowerCase();
  if (status === "ok") {
    return "已执行";
  }
  if (status === "failed" || status === "error") {
    return "执行失败";
  }
  return localizeDisplayText(value) || value;
}

function mixBoardReviewStatusLabel(value: string): string {
  const status = value.toLowerCase();
  if (status === "waiting_review") {
    return "等待复查";
  }
  if (status === "reviewed") {
    return "已复查";
  }
  if (status === "effective") {
    return "变化有效";
  }
  if (status === "not_obvious") {
    return "变化不明显";
  }
  if (status === "rolled_back") {
    return "已撤回";
  }
  if (status === "failed") {
    return "复查失败";
  }
  return localizeDisplayText(value) || value;
}

function mixBoardPeakRmsLabel(summary: JsonRecord): string {
  const peak = textValue(summary.peak_dbfs, "");
  const rms = textValue(summary.rms_dbfs, "");
  if (peak || rms) {
    return `Peak ${peak || "-"} dBFS / RMS ${rms || "-"} dBFS`;
  }
  const snap = asRecord(summary.feature_snapshot);
  const waveform = asRecord(snap.waveform_envelope);
  return textValue(waveform.status, "未接入");
}

export function isMixBoardAction(action: JsonRecord): boolean {
  const kind = textValue(action.kind ?? action.type, "").toLowerCase();
  const type = textValue(action.type, "").toLowerCase();
  const workflow = textValue(action.workflow, "").toLowerCase();
  const source = textValue(action.source, "").toLowerCase();
  const payload = interactionPayload(action);
  const payloadKind = textValue(payload.kind ?? payload.type, "").toLowerCase();
  const payloadType = textValue(payload.type, "").toLowerCase();
  const identifiers = [kind, type, payloadKind, payloadType].filter(Boolean);
  const hasMixBoardKind = identifiers.some((value) =>
    value === "mix_board" ||
    value === "mixboard" ||
    value === "mixboard_status" ||
    value.includes("mixboard")
  );
  const hasMixBoardPayload =
    Object.keys(asRecord(action.mix_observation)).length > 0 ||
    Object.keys(asRecord(payload.mix_observation)).length > 0;
  if ((kind === "mix_session_entry" || type === "mix_session_entry") || (isConfirmationAction(action) && !hasMixBoardKind && !hasMixBoardPayload)) {
    return false;
  }
  return hasMixBoardKind ||
    hasMixBoardPayload ||
    (source === "mix_session" && workflow === "mix_session_entry" && type === "mixboard_status");
}

export function isMixPlanningDiscussionAction(action: JsonRecord): boolean {
  const kind = textValue(action.kind ?? action.type, "").toLowerCase();
  const type = textValue(action.type, "").toLowerCase();
  return kind === "mix_planning_discussion" || type === "mix_planning_discussion";
}

function mixSessionStateLabel(value: unknown): string {
  const state = textValue(value, "").toLowerCase();
  if (state === "observation_ready") {
    return "声学观察已就绪";
  }
  if (state === "observation_partial") {
    return "声学观察受限";
  }
  if (state === "observation_unavailable") {
    return "等待声学观察";
  }
  if (state === "ready_for_observation") {
    return "等待观察";
  }
  if (state === "tuning_running") {
    return "自动调控中";
  }
  if (state === "waiting_review") {
    return "等待试听复查";
  }
  if (state === "tuning_paused") {
    return "调控已暂停";
  }
  if (state === "rollback_available") {
    return "可撤回上一轮";
  }
  if (state === "tuning_complete") {
    return "本轮调控完成";
  }
  if (state === "tuning_stopped") {
    return "调控已暂停";
  }
  return localizeDisplayText(state) || state || "未知";
}

function mixObservationStatusLabel(value: unknown): string {
  const status = textValue(value, "").toLowerCase();
  if (status === "ready") {
    return "就绪";
  }
  if (status === "partial") {
    return "受限";
  }
  if (status === "requested") {
    return "读取中";
  }
  if (status === "blocked") {
    return "等待可分析片段";
  }
  if (status === "unavailable") {
    return "等待观察";
  }
  if (status === "missing") {
    return "缺失";
  }
  return localizeDisplayText(status) || status || "-";
}

function mixBoardFeatureRequestLabel(request: JsonRecord): string {
  const status = textValue(request.status, "").toLowerCase();
  const count = firstArray(request.requested_features).length;
  if (status === "requested") {
    return count > 0 ? `已请求 ${count} 项声学特征` : "已请求声学特征";
  }
  if (status === "blocked") {
    return mixBoardBlockerLabel(textValue(request.reason, "声学特征请求受限"));
  }
  return status || "-";
}

function mixBoardPackageStatusLabel(value: string): string {
  const status = value.toLowerCase();
  if (status === "ready") {
    return "就绪";
  }
  if (status === "partial") {
    return "部分就绪";
  }
  if (status === "baseline_ready") {
    return "基础闭环就绪";
  }
  if (status === "limited") {
    return "指标受限";
  }
  if (status === "async_available") {
    return "异步可用";
  }
  if (status === "async_missing") {
    return "异步待补齐";
  }
  return value || "-";
}

function mixBoardTargetLabel(target: JsonRecord): string {
  const label = textValue(target.label, "");
  if (label) {
    return label;
  }
  const kind = textValue(target.kind, "").toLowerCase();
  if (kind === "track") {
    return "当前轨道";
  }
  if (kind === "clip") {
    return "当前片段";
  }
  if (kind === "project" || kind === "song") {
    return "全曲";
  }
  return textValue(target.id, "当前混音对象");
}

function mixBoardObjectsLabel(objects: JsonRecord[], target: JsonRecord): string {
  if (objects.length === 0) {
    return mixBoardTargetLabel(target);
  }
  const labels = objects.slice(0, 3).map((object) => {
    const label = textValue(object.label, "") || mixBoardObjectKindLabel(textValue(object.kind, ""));
    const scope = mixBoardEffectScopeLabel(textValue(object.effect_scope, ""));
    return scope ? `${label}（${scope}）` : label;
  });
  const extra = objects.length > labels.length ? ` 等 ${objects.length} 个对象` : "";
  return `${labels.join("、")}${extra}`;
}

function mixBoardObjectKindLabel(kindValue: string): string {
  const kind = kindValue.toLowerCase();
  if (kind === "track") {
    return "轨道";
  }
  if (kind === "clip") {
    return "片段";
  }
  if (kind === "bus") {
    return "鎬荤嚎";
  }
  return kindValue || "混音对象";
}

function mixBoardEffectScopeLabel(scopeValue: string): string {
  const scope = scopeValue.toLowerCase();
  if (scope === "track_scope") {
    return "轨道处理";
  }
  if (scope === "clip_scope") {
    return "片段处理";
  }
  return "";
}

function mixBoardListenScopeLabel(scope: JsonRecord, duration: number | null): string {
  const time = asRecord(scope.time);
  const source = asRecord(scope.source);
  const timeMode = textValue(time.mode, "").toLowerCase();
  let timeLabel = "全曲";
  if (timeMode === "manual_time") {
    const start = numberFromKeys(time, "start_seconds");
    const end = numberFromKeys(time, "end_seconds");
    timeLabel = start !== null && end !== null ? `${start.toFixed(2)}s - ${end.toFixed(2)}s` : "选定时间段";
  } else if (timeMode === "full_song" && duration !== null && duration > 0) {
    timeLabel = `全曲 ${duration.toFixed(2)}s`;
  } else if (timeMode) {
    timeLabel = localizeDisplayText(timeMode) || timeMode;
  }

  const sourceMode = textValue(source.mode, "").toLowerCase();
  let sourceLabel = "全轨混音";
  const focusCount = firstArray(source.focus_ids).length;
  if (sourceMode === "target_only" || sourceMode === "focused_targets") {
    sourceLabel = focusCount > 1 ? `${focusCount} 个选中对象` : "混音对象";
  } else if (sourceMode === "selected_tracks") {
    sourceLabel = focusCount > 1 ? `${focusCount} 条选中轨道` : "选中轨道";
  } else if (sourceMode === "selected_clips") {
    sourceLabel = focusCount > 1 ? `${focusCount} 个选中片段` : "选中片段";
  } else if (sourceMode && sourceMode !== "full_mix_context") {
    sourceLabel = localizeDisplayText(sourceMode) || sourceMode;
  }
  return `${sourceLabel} 路 ${timeLabel}`;
}

function mixBoardUserNoteLabel(session: JsonRecord, mixboard: JsonRecord, payload: JsonRecord): string {
  return (
    textValue(session.user_note ?? session.user_feedback ?? session.subjective_note, "") ||
    textValue(mixboard.user_note ?? mixboard.user_feedback ?? mixboard.subjective_note, "") ||
    textValue(payload.user_note ?? payload.user_feedback ?? payload.subjective_note, "") ||
    "等待用户听感，可在下一轮开始前补充"
  );
}

function mixBoardJudgementLabel(problems: unknown[], status: string, blocker: string): string {
  const problemLabels = problems
    .map(asRecord)
    .map((problem) => textFromKeys(problem, "summary", "label", "problem", "tag", "kind"))
    .filter((value) => value.length > 0)
    .slice(0, 2);
  if (problemLabels.length > 0) {
    return problemLabels.join("；");
  }
  if (blocker) {
    return mixBoardBlockerLabel(blocker);
  }
  const normalized = status.toLowerCase();
  if (normalized === "requested") {
    return "声学特征已请求，等待内核回传波形与频谱";
  }
  if (normalized === "blocked") {
    return blocker ? mixBoardBlockerLabel(blocker) : "需要可分析的音频片段后才能读取声学特征";
  }
  if (normalized === "partial") {
    return "声学观察部分就绪，先按已接入特征与时间尺判断";
  }
  if (normalized === "unavailable") {
    return "缺少时间尺或声学特征，暂不做音色判断";
  }
  return "暂无突出问题";
}

function mixBoardCurrentActionLabel(session: JsonRecord, mixboard: JsonRecord, observation: JsonRecord, payload: JsonRecord, status: string): string {
  const actionText =
    textValue(session.current_action ?? session.action, "") ||
    textValue(mixboard.current_action ?? mixboard.action, "") ||
    textValue(observation.current_action ?? observation.action, "") ||
    textValue(payload.current_action ?? payload.action, "");
  if (actionText) {
    return actionText;
  }
  if (status === "requested") {
    return "已向内核请求波形包络与频谱场，等待回传";
  }
  if (status === "blocked") {
    return "等待选择可分析的音频片段或恢复内核连接";
  }
  return status === "ready" ? "等待根据目标执行效果器微调" : "建立 MixBoard，等待声学特征与下一轮调控";
}

function mixBoardNextStepLabel(session: JsonRecord, mixboard: JsonRecord, payload: JsonRecord, status: string, blocker: string): string {
  const nextText =
    textValue(session.next_step ?? session.next_action, "") ||
    textValue(mixboard.next_step ?? mixboard.next_action, "") ||
    textValue(payload.next_step ?? payload.next_action, "");
  if (nextText) {
    return nextText;
  }
  if (blocker) {
    return "接入声学特征读取后刷新观察，再进入参数调控轮";
  }
  if (status === "requested") {
    return "等待特征回传后刷新观察";
  }
  if (status === "blocked") {
    return "先指定含音频片段的混音对象";
  }
  return status === "ready" ? "进入监听、调参、复查循环" : "先确认混音对象与试听范围";
}

function mixBoardTrafficLabel(value: unknown, blocker: string): string {
  const status = textValue(value, "").toLowerCase();
  if (status === "ready") {
    return "声学观察就绪";
  }
  if (status === "requested") {
    return "声学特征已请求，等待回传";
  }
  if (status === "blocked") {
    return blocker ? mixBoardBlockerLabel(blocker) : "需要可分析的音频片段";
  }
  if (status === "partial") {
    return blocker ? mixBoardBlockerLabel(blocker) : "工程与时间尺已就绪，声学观察受限";
  }
  if (status === "unavailable") {
    return blocker ? mixBoardBlockerLabel(blocker) : "等待声学观察";
  }
  return mixObservationStatusLabel(status);
}

function mixBoardTrafficState(value: unknown): string {
  const status = textValue(value, "").toLowerCase();
  if (status === "ready" || status === "ok" || status === "observation_ready" || status === "tuning_complete") {
    return "ready";
  }
  if (status === "partial" || status === "requested" || status === "observation_partial" || status === "tuning_running" || status === "tuning_stopped") {
    return "partial";
  }
  if (status === "blocked" || status === "unavailable" || status === "observation_unavailable" || status === "missing" || status === "failed" || status === "error") {
    return "unavailable";
  }
  return status ? "partial" : "unavailable";
}

function mixBoardBlockerLabel(value: string): string {
  const blocker = value.toLowerCase();
  if (blocker.includes("waveform ready") && blocker.includes("spectrum")) {
    return "基础声学包已接入，深度频段观察可稍后补充";
  }
  if (blocker.includes("spectrum ready") && blocker.includes("waveform")) {
    return "频谱观察已接入，波形包络待接入";
  }
  if (blocker.includes("partially connected")) {
    return "声学观察部分接入，仍需补齐特征包";
  }
  if (blocker.includes("audio_feature_request_pending")) {
    return "声学特征已请求，等待内核回传";
  }
  if (blocker.includes("audio_feature_request_blocked")) {
    return "声学特征请求受限，需要明确音频片段源";
  }
  if (blocker.includes("clip_source_required")) {
    return "当前特征提取需要明确的音频片段源";
  }
  if (blocker.includes("kernel_client_unavailable")) {
    return "内核连接不可用，暂不能读取声学特征";
  }
  if (blocker.includes("all_feature_requests_skipped")) {
    return "声学特征请求未成功发送";
  }
  if (blocker === "audio_feature_reader_not_connected" || blocker.includes("audio feature")) {
    return "声学特征读取尚未接通，当前只能按工程和时间尺判断";
  }
  if (blocker === "feature_source_unavailable") {
    return "声学特征来源暂不可用";
  }
  return localizeDisplayText(value) || value;
}
