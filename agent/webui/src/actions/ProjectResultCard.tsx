// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2a）：App.tsx 动作卡体系全域出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
// ProjectResultCard 工程结果卡全家。

import { ChevronRight, Loader2, Play, Radio } from "lucide-react";
import { useState } from "react";
import type { AgentUIState, ChatResponse, JsonRecord } from "../types";
import { asRecord, finiteNumber, firstArray, numericValue, textValue, truthy, uniqueID } from "../lib/util";
import { actionDawTarget, actionLooksLikeDawTarget, auditionDawTarget, DawClip, DawPlugin, DawTrack, dawTracksFromUIState, focusDawTarget, formatTimelineSeconds, isMidiClip } from "../daw/parse";
import { agentExecutionSucceeded } from "../App";
import { mergeJsonRecords, responseNeedsConfirmationFallback } from "./confirmation";
import type { ActionCardProps, ProjectResultABView, ProjectResultReadiness, ProjectResultView } from "./types";

export function ProjectResultCard({ action, respondingActionID, onInvoke, uiState }: ActionCardProps) {
  const result = projectResultSummary(action, uiState);
  const [busyAction, setBusyAction] = useState("");
  if (!result) {
    return null;
  }
  const disabled = respondingActionID !== "" || busyAction !== "";
  const readinessTone =
    result.readiness === "ready" ? "success" : result.readiness === "silent_risk" || result.readiness === "affected_by_mute" ? "attention" : "info";
  return (
    <div className={`action-card project-result ${readinessTone}`}>
      <Radio size={16} />
      <div className="action-content">
        <div className="action-title-line">
          <strong>{result.title}</strong>
          <span>{result.badge}</span>
        </div>
        {result.body && <p className="action-body">{result.body}</p>}
        <div className="action-detail-grid project-result-grid">
          {result.details.map((item) => (
            <div className="action-detail" key={item.id}>
              <span>{item.title}</span>
              <strong>{item.body}</strong>
            </div>
          ))}
        </div>
        {result.hint && <p className="project-result-hint">{result.hint}</p>}
        <div className="action-buttons action-quick-buttons">
          <button
            className="action-button secondary"
            type="button"
            disabled={disabled}
            onClick={() => {
              setBusyAction("focus");
              void focusDawTarget(onInvoke, result.target)
                .catch((error) => console.warn("[AskVit] project result focus failed", error))
                .finally(() => setBusyAction(""));
            }}
          >
            {busyAction === "focus" ? <Loader2 className="spin" size={14} /> : <ChevronRight size={14} />}
            <span>跳转</span>
          </button>
          {result.canAudition && (
            <button
              className={result.readiness === "ready" ? "action-button primary" : "action-button secondary"}
              type="button"
              disabled={disabled}
              onClick={() => {
                setBusyAction("audition");
                void auditionDawTarget(onInvoke, result.target)
                  .catch((error) => console.warn("[AskVit] project result audition failed", error))
                  .finally(() => setBusyAction(""));
              }}
            >
              {busyAction === "audition" ? <Loader2 className="spin" size={14} /> : <Play size={14} />}
              <span>{result.auditionLabel}</span>
            </button>
          )}
        </div>
      </div>
    </div>
  );
}

export function isProjectResultAction(action: JsonRecord): boolean {
  return textValue(action.kind ?? action.type, "").toLowerCase() === "project_result";
}

function projectResultSummary(action: JsonRecord, uiState: AgentUIState | null): ProjectResultView | null {
  const executions = projectResultExecutions(action);
  if (executions.length === 0) {
    return null;
  }
  const target = projectResultTarget(action, executions, uiState);
  if (!target) {
    return null;
  }
  const track = projectResultTrack(uiState, textValue(target.track_id, ""));
  const clip = projectResultClip(uiState, textValue(target.clip_id, ""), textValue(target.track_id, ""));
  const operation = projectResultOperation(executions, target, clip);
  const readiness = projectResultReadiness(target, track, clip);
  const canAudition = readiness !== "not_auditionable";
  const abResult = projectResultABView(action);
  const details = projectResultDetails(target, track, clip, executions, abResult);
  return {
    title: operation.title,
    badge: projectResultReadinessBadge(readiness),
    body: projectResultBodyWithAB(operation.body, abResult),
    hint: projectResultReadinessHint(readiness, track, clip),
    readiness,
    canAudition,
    auditionLabel: readiness === "ready" ? "试听" : readiness === "not_auditionable" ? "" : "试听（可能无声）",
    target,
    details
  };
}

function projectResultExecutions(action: JsonRecord): JsonRecord[] {
  return firstArray(action.executions, action.executed_kernel_reply)
    .map(asRecord)
    .filter((entry) => Object.keys(entry).length > 0);
}

function projectResultABView(action: JsonRecord): ProjectResultABView | null {
  const ab = asRecord(action.ab_result ?? action.mom_ab_result ?? action.result_ab);
  if (Object.keys(ab).length === 0) {
    return null;
  }
  const status = textValue(ab.status, "missing").toLowerCase();
  const title = textValue(ab.display_title, status === "ready" ? "AB Result：可信" : "AB Result：不可信");
  const body = textValue(ab.display_body, "");
  return {
    title,
    body,
    status,
    tapPoint: textValue(ab.tap_point, ""),
    renderMode: textValue(ab.render_mode, ""),
    deltaSummary: textValue(ab.delta_summary, "")
  };
}

function projectResultBodyWithAB(body: string, abResult: ProjectResultABView | null): string {
  if (!abResult) {
    return body;
  }
  const abText = abResult.title;
  return body ? `${body} ${abText}` : abText;
}

export function projectResultActionsFromResponse(response: ChatResponse): JsonRecord[] {
  if (responseNeedsConfirmationFallback(response)) {
    return [];
  }
  const explicitCards = firstArray(response.project_result_cards)
    .map(asRecord)
    .filter((card) => Object.keys(card).length > 0)
    .map((card, index) => ({
      ...card,
      _ui_source: "project_result",
      kind: "project_result",
      type: "project_result",
      id: textValue(card.id, `project_result_${textValue(response.goal_id ?? response.run_id ?? response.conversation_id, String(index))}`)
    }));
  if (explicitCards.length > 0) {
    return explicitCards;
  }
  const executions = firstArray(response.executed_kernel_reply)
    .map(asRecord)
    .filter(projectResultExecutionCandidate);
  if (executions.length === 0) {
    return [];
  }
  return [
    {
      _ui_source: "project_result",
      kind: "project_result",
      id: `project_result_${textValue(response.goal_id ?? response.run_id ?? response.conversation_id, uniqueID("project_result"))}`,
      executions
    }
  ];
}

function projectResultExecutionCandidate(entry: JsonRecord): boolean {
  const result = asRecord(entry.result);
  const payload = Object.keys(result).length > 0 ? result : entry;
  if (!agentExecutionSucceeded(entry, payload)) {
    return false;
  }
  const commandName = projectResultCommandName(entry, payload);
  if (projectResultReadOnlyCommand(commandName)) {
    return false;
  }
  const uiAction = textValue(payload.ui_action ?? entry.ui_action, "").toLowerCase();
  return actionLooksLikeDawTarget(commandName, uiAction, mergeJsonRecords(entry, payload));
}

function projectResultCommandName(entry: JsonRecord, payload: JsonRecord): string {
  const verification = asRecord(entry.verification);
  return textValue(
    entry.command_name ??
      entry.tool ??
      entry.cmd ??
      entry.command ??
      payload.command_name ??
      payload.tool ??
      payload.cmd ??
      payload.command ??
      verification.command_name ??
      verification.tool,
    ""
  ).toLowerCase();
}

function projectResultReadOnlyCommand(commandName: string): boolean {
  if (!commandName) {
    return false;
  }
  return (
    commandName.startsWith("get_") ||
    commandName.startsWith("list_") ||
    commandName.includes(".list") ||
    commandName.includes(".read") ||
    commandName.includes("read_clip") ||
    commandName.includes("project.state") ||
    commandName.includes("artifact.") ||
    commandName.includes("media.index") ||
    commandName.includes("media.register") ||
    commandName.includes("browser.")
  );
}

function projectResultTarget(action: JsonRecord, executions: JsonRecord[], uiState: AgentUIState | null): JsonRecord | null {
  let target: JsonRecord = asRecord(action.target);
  executions.forEach((entry) => {
    const next = actionDawTarget({ ...entry, _ui_source: "executed" }, uiState);
    if (!next) {
      return;
    }
    target = mergeProjectTargets(target, next);
  });
  if (!textValue(target.track_id ?? target.clip_id ?? target.plugin_id, "")) {
    return null;
  }
  return target;
}

function mergeProjectTargets(current: JsonRecord, incoming: JsonRecord): JsonRecord {
  const out = { ...current };
  const incomingClip = textValue(incoming.clip_id, "");
  const currentClip = textValue(out.clip_id, "");
  const incomingPlugin = textValue(incoming.plugin_id, "");
  const currentPlugin = textValue(out.plugin_id, "");
  if (incomingClip || !currentClip) {
    copyProjectTargetValue(out, incoming, "clip_id");
  }
  if (incomingPlugin || (!incomingClip && !currentPlugin)) {
    copyProjectTargetValue(out, incoming, "plugin_id");
  }
  for (const key of ["track_id", "start_seconds", "start_beats", "can_audition"]) {
    if (incoming[key] !== undefined && (out[key] === undefined || incomingClip || incomingPlugin)) {
      out[key] = incoming[key];
    }
  }
  return out;
}

function copyProjectTargetValue(out: JsonRecord, incoming: JsonRecord, key: string): void {
  const value = textValue(incoming[key], "");
  if (value) {
    out[key] = value;
  }
}

function projectResultTrack(uiState: AgentUIState | null, trackID: string): DawTrack | null {
  if (!trackID) {
    return null;
  }
  return dawTracksFromUIState(uiState).find((track) => track.id === trackID) ?? null;
}

function projectResultClip(uiState: AgentUIState | null, clipID: string, trackID = ""): DawClip | null {
  if (!clipID) {
    return null;
  }
  for (const track of dawTracksFromUIState(uiState)) {
    if (trackID && track.id !== trackID) {
      continue;
    }
    const clip = track.clips.find((item) => item.id === clipID);
    if (clip) {
      return clip;
    }
  }
  return null;
}

function projectResultOperation(executions: JsonRecord[], target: JsonRecord, clip: DawClip | null): { title: string; body: string } {
  const commands = executions.map((entry) => {
    const result = asRecord(entry.result);
    return projectResultCommandName(entry, Object.keys(result).length > 0 ? result : entry);
  });
  const joined = commands.join(" ");
  const noteCount = projectResultNoteCount(executions);
  const hasClip = textValue(target.clip_id, "") !== "";
  const hasPlugin = textValue(target.plugin_id, "") !== "";
  const hasTrack = textValue(target.track_id, "") !== "";
  if (joined.includes("apply_midi_note_patch") || joined.includes("add_midi_notes") || noteCount > 0) {
    const title = joined.includes("create_midi_clip") || joined.includes("midi.create_clip") ? "MIDI 片段已创建并写入" : "MIDI 片段已更新";
    return { title, body: noteCount > 0 ? `已写入或更新 ${noteCount} 个音符。` : "已更新 MIDI 音符。" };
  }
  if (joined.includes("create_midi_clip") || joined.includes("midi.create_clip")) {
    return { title: "MIDI 片段已创建", body: "已在工程中创建 MIDI 片段。" };
  }
  if (joined.includes("import_media") || joined.includes("import_audio") || joined.includes("clip.import")) {
    return { title: "媒体片段已导入", body: "已把素材放入工程时间线。" };
  }
  if (joined.includes("mix.apply_tick") || joined.includes("mix_apply_tick")) {
    if (joined.includes("pan")) {
      return { title: "混音声像已更新", body: "已执行一次可撤销的轨道声像调整。" };
    }
    if (joined.includes("gain") || joined.includes("volume")) {
      return { title: "混音电平已更新", body: "已执行一次可撤销的轨道电平调整。" };
    }
    return { title: "混音步骤已执行", body: "已执行一次可撤销的小型混音调整。" };
  }
  if (hasClip) {
    return { title: clip && isMidiClip(clip) ? "MIDI 片段已更新" : "片段已更新", body: "工程片段已有可定位结果。" };
  }
  if (hasPlugin || joined.includes("plugin") || joined.includes("rack") || joined.includes("control")) {
    return { title: "声音处理已更新", body: "已修改插件、机架或宏控制相关设置。" };
  }
  if (hasTrack) {
    return { title: joined.includes("add_track") ? "轨道已创建" : "轨道已更新", body: "工程轨道已有可定位结果。" };
  }
  return { title: "工程已更新", body: "本次任务已修改工程。" };
}

function projectResultNoteCount(executions: JsonRecord[]): number {
  return executions.reduce((total, entry) => {
    const result = asRecord(entry.result);
    const payload = Object.keys(result).length > 0 ? result : entry;
    const direct = numericValue(payload.inserted_count ?? payload.added_count ?? payload.mutated_count ?? payload.note_count ?? payload.notes_count);
    const notes = firstArray(payload.notes, payload.midi_notes, payload.note_rows);
    return total + Math.max(direct, notes.length);
  }, 0);
}

function projectResultReadiness(target: JsonRecord, track: DawTrack | null, clip: DawClip | null): ProjectResultReadiness {
  if (!truthy(target.can_audition)) {
    return "not_auditionable";
  }
  if (track?.mute) {
    return "affected_by_mute";
  }
  if (clip) {
    if (!isMidiClip(clip)) {
      return "ready";
    }
    if (!track) {
      return "unknown";
    }
    return trackHasLikelyInstrument(track) ? "ready" : "silent_risk";
  }
  if (track) {
    if (track.clips.length === 0) {
      return "not_auditionable";
    }
    if (track.clips.some((item) => !isMidiClip(item))) {
      return "ready";
    }
    return trackHasLikelyInstrument(track) ? "ready" : "silent_risk";
  }
  return "unknown";
}

function projectResultReadinessBadge(readiness: ProjectResultReadiness): string {
  switch (readiness) {
    case "ready":
      return "可试听";
    case "silent_risk":
      return "可能无声";
    case "affected_by_mute":
      return "受静音影响";
    case "not_auditionable":
      return "可跳转";
    default:
      return "状态未知";
  }
}

function projectResultReadinessHint(readiness: ProjectResultReadiness, track: DawTrack | null, clip: DawClip | null): string {
  if (readiness === "silent_risk" && clip && isMidiClip(clip)) {
    return "当前轨道没有检测到乐器插件，MIDI 片段播放时可能没有声音。";
  }
  if (readiness === "affected_by_mute") {
    return "目标轨道当前处于静音状态，试听前可能需要取消静音。";
  }
  if (readiness === "unknown") {
    return "可以定位并尝试试听，但当前路由或音源状态无法完全确认。";
  }
  if (readiness === "not_auditionable" && track?.clips.length === 0) {
    return "当前目标没有可播放片段，已提供跳转入口。";
  }
  return "";
}

function projectResultDetails(
  target: JsonRecord,
  track: DawTrack | null,
  clip: DawClip | null,
  executions: JsonRecord[],
  abResult: ProjectResultABView | null
): Array<{ id: string; title: string; body: string }> {
  const details: Array<{ id: string; title: string; body: string }> = [];
  if (abResult) {
    details.push({ id: "ab_result", title: "AB Result", body: abResult.status === "ready" ? "可信" : "不可信" });
    if (abResult.deltaSummary) {
      details.push({ id: "ab_delta", title: "AB 变化", body: abResult.deltaSummary });
    }
    if (abResult.tapPoint) {
      details.push({ id: "ab_tap", title: "观测点", body: abResult.tapPoint });
    }
    if (abResult.renderMode) {
      details.push({ id: "ab_render", title: "观测方式", body: abResult.renderMode });
    }
  }
  const trackLabel = track?.name || textValue(target.track_id, "");
  const clipLabel = clip?.name || textValue(target.clip_id, "");
  if (trackLabel) {
    details.push({ id: "track", title: "轨道", body: trackLabel });
  }
  if (clipLabel) {
    details.push({ id: "clip", title: "片段", body: clipLabel });
  }
  const startSeconds = finiteNumber(target.start_seconds, NaN);
  if (Number.isFinite(startSeconds)) {
    details.push({ id: "start", title: "起点", body: formatTimelineSeconds(startSeconds) });
  }
  if (clip) {
    details.push({ id: "duration", title: "长度", body: formatTimelineSeconds(clip.lengthSeconds) });
  }
  const noteCount = projectResultNoteCount(executions);
  if (noteCount > 0) {
    details.push({ id: "notes", title: "音符", body: `${noteCount} 个` });
  }
  return details.slice(0, 6);
}

function trackHasLikelyInstrument(track: DawTrack): boolean {
  if (track.plugins.length === 0) {
    return false;
  }
  return track.plugins.some(pluginLooksLikeInstrument);
}

function pluginLooksLikeInstrument(plugin: DawPlugin): boolean {
  const text = [plugin.name, plugin.kind, plugin.role, plugin.path, plugin.vendor, plugin.source].join(" ").toLowerCase();
  if (!text.trim()) {
    return false;
  }
  if (/(^|\b)(eq|equalizer|compressor|limiter|reverb|delay|chorus|flanger|phaser|distortion|saturat|gate|filter|meter|analyzer|utility|gain|clipper|tape)(\b|$)/.test(text)) {
    return false;
  }
  return /(instrument|synth|sampler|rompler|piano|keys|keyboard|organ|drum|kit|bass|guitar|strings|brass|kontakt|serum|massive|spire|omnisphere|vital|dexed|pigments|labs|sine)/.test(text);
}
