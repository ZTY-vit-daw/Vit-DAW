// T9-PRE-1A-1（施工图 UI-CONVERGE-RECON-1 §3.1 批 1 / 步 1a）：App.tsx DAW 状态解析层（F 域）出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
// statusLabel/mergeJsonRecords 仍属 App 动作卡域（§3.1 步 1e 未来落点），本模块经 App
// 兼容层引用（两侧均为函数声明，ESM 实例化期初始化，循环安全），1e 卡移动后剪断。

import type { AgentInvokeResponse, AgentUIState, JsonRecord } from "../types";
import {
  asRecord,
  clampNumber,
  finiteNumber,
  firstArray,
  numericValue,
  textValue,
  truthy
} from "../lib/util";
import { mergeJsonRecords, statusLabel } from "../App";

type DawInvoke = (tool: string, args?: JsonRecord, source?: string, confirmed?: boolean) => Promise<AgentInvokeResponse>;
type DawActivitySegment = { left: number; width: number; height: number };
type DawClip = {
  id: string;
  name: string;
  type: string;
  startSeconds: number;
  lengthSeconds: number;
  startBeats: number;
  lengthBeats: number;
  selected: boolean;
  noteCount: number;
  notes: JsonRecord[];
  activity: DawActivitySegment[];
  activityKnown: boolean;
};
type DawPlugin = {
  id: string;
  itemID: string;
  name: string;
  kind: string;
  role: string;
  clipScope: string;
  selected: boolean;
  bypass: boolean;
  path: string;
  vendor: string;
  source: string;
  deletable: boolean;
};
type DawTrack = {
  id: string;
  name: string;
  type: string;
  color: string;
  selected: boolean;
  mute: boolean;
  solo: boolean;
  armed: boolean;
  volumeDB: number | null;
  pan: number | null;
  levelDB: number | null;
  clips: DawClip[];
  plugins: DawPlugin[];
  rackEdges: JsonRecord[];
};

const dawTrackPalette = ["#1f55d8", "#0f8f6d", "#a35d00", "#7c3aed", "#b42318", "#2e6f95", "#8a6f00", "#4d66c7"];

function dawTracksFromUIState(uiState: AgentUIState | null): DawTrack[] {
  const selectedTrackID = selectedTrackIDFromUIState(uiState);
  const selectedPlugin = selectedPluginFromUIState(uiState);
  const selectedClipIDs = selectedClipIDsFromUIState(uiState);
  const bpm = bpmFromUIState(uiState);
  return (uiState?.tracks ?? [])
    .map(asRecord)
    .map((track, index) => dawTrackFromRecord(track, index, selectedTrackID, selectedPlugin, selectedClipIDs, bpm))
    .filter((track) => track.id !== "");
}

function dawTrackFromRecord(
  raw: JsonRecord,
  index: number,
  selectedTrackID: string,
  selectedPlugin: { id: string; trackID: string; name: string },
  selectedClipIDs: Set<string>,
  bpm: number
): DawTrack {
  const id = textFromKeys(raw, "track_id", "id", "uid", "kernel_track_id");
  const rack = asRecord(raw.rack);
  const rackNodes = firstArray(rack.nodes, raw.plugins, raw.devices, raw.rack_nodes, rack.items, rack.chain);
  const rackEdges = firstArray(rack.edges, raw.rack_edges, raw.edges).map(asRecord);
  const clips = firstArray(raw.clips, raw.clip_summaries)
    .map(asRecord)
    .map((clip) => dawClipFromRecord(clip, selectedClipIDs, bpm))
    .filter((clip) => clip.id !== "")
    .sort((left, right) => left.startSeconds - right.startSeconds);
  const plugins = rackNodes
    .map(asRecord)
    .filter((plugin) => !isSystemRackPluginRecord(plugin))
    .map((plugin, pluginIndex) => dawPluginFromRecord(plugin, pluginIndex, selectedPlugin))
    .filter((plugin) => plugin.id !== "");
  const selected = truthy(raw.selected ?? raw.is_selected) || (selectedTrackID !== "" && selectedTrackID === id) || selectedPlugin.trackID === id;
  return {
    id,
    name: textFromKeys(raw, "track_name", "name", "label") || id || `Track ${index + 1}`,
    type: statusLabel(textFromKeys(raw, "track_type", "type", "vit_type"), textFromKeys(raw, "track_type", "type", "vit_type") || "track"),
    color: textFromKeys(raw, "color", "track_color") || dawTrackPalette[index % dawTrackPalette.length],
    selected,
    mute: truthy(raw.mute ?? raw.muted ?? raw.is_muted),
    solo: truthy(raw.solo ?? raw.is_solo),
    armed: truthy(raw.armed ?? raw.is_armed ?? raw.record_armed),
    volumeDB: numberFromKeys(raw, "volume_db", "volumeDb", "fader_db", "faderDb", "gain_db", "gainDb"),
    pan: numberFromKeys(raw, "pan", "pan_value"),
    levelDB: trackLevelDB(raw),
    clips,
    plugins,
    rackEdges
  };
}

function dawClipFromRecord(raw: JsonRecord, selectedClipIDs: Set<string>, bpm: number): DawClip {
  const id = textFromKeys(raw, "clip_id", "id", "uid");
  const type = textFromKeys(raw, "clip_type", "type", "media_type", "kind") || "clip";
  const beatSeconds = 60 / Math.max(1, bpm);
  const startBeats = finiteNumber(valueFromKeys(raw, "start_beats", "startBeat", "beat"), finiteNumber(valueFromKeys(raw, "start_seconds", "start_time", "start"), 0) / beatSeconds);
  const startSeconds = finiteNumber(valueFromKeys(raw, "start_seconds", "start_time_seconds", "start_time", "startSeconds"), startBeats * beatSeconds);
  const notesRaw = firstArray(raw.notes, raw.midi_notes, raw.note_summary);
  const notes = notesRaw.map(asRecord).filter((note) => Object.keys(note).length > 0);
  const noteCount = Math.max(numericValue(raw.note_count ?? raw.notes_count), notes.length);
  const endBeats = finiteNumber(valueFromKeys(raw, "end_beats", "endBeat"), NaN);
  const endSeconds = finiteNumber(valueFromKeys(raw, "end_seconds", "end_time_seconds", "end_time", "endSeconds"), NaN);
  let lengthBeats = finiteNumber(valueFromKeys(raw, "length_beats", "duration_beats", "lengthBeats"), Number.isFinite(endBeats) ? endBeats - startBeats : NaN);
  let lengthSeconds = finiteNumber(valueFromKeys(raw, "length_seconds", "duration_seconds", "lengthSeconds", "duration"), Number.isFinite(endSeconds) ? endSeconds - startSeconds : NaN);
  if (!Number.isFinite(lengthBeats) || lengthBeats <= 0) {
    lengthBeats = Number.isFinite(lengthSeconds) && lengthSeconds > 0 ? lengthSeconds / beatSeconds : midiNoteEndBeats(notes);
  }
  if (!Number.isFinite(lengthSeconds) || lengthSeconds <= 0) {
    lengthSeconds = Number.isFinite(lengthBeats) && lengthBeats > 0 ? lengthBeats * beatSeconds : 1;
  }
  lengthBeats = Math.max(0.25, lengthBeats);
  lengthSeconds = Math.max(0.25, lengthSeconds);
  const activity = clipActivitySegments(raw, notes, lengthBeats);
  const activityKnown = hasArrayValue(raw, "notes", "midi_notes", "note_summary", "activity", "activity_segments", "peaks", "waveform_peaks", "waveform", "rms", "amplitudes");
  return {
    id,
    name: textFromKeys(raw, "name", "clip_name", "title") || id || "Clip",
    type,
    startSeconds,
    lengthSeconds,
    startBeats,
    lengthBeats,
    selected: selectedClipIDs.has(id) || truthy(raw.selected ?? raw.is_selected),
    noteCount,
    notes,
    activity,
    activityKnown
  };
}

function dawPluginFromRecord(raw: JsonRecord, index: number, selectedPlugin: { id: string; trackID: string; name: string }): DawPlugin {
  const id = textFromKeys(raw, "plugin_id", "id", "node_id", "item_id", "plugin_item_id");
  const itemID = textFromKeys(raw, "plugin_item_id", "item_id", "plugin_id", "id", "node_id") || id;
  const scope = textFromKeys(raw, "clip_scope", "scope", "target_scope", "lane").toLowerCase();
  const role = textFromKeys(raw, "role", "slot", "lane", "chain", "chain_id", "group");
  const kind = textFromKeys(raw, "type", "plugin_type", "kind") || "plugin";
  return {
    id,
    itemID,
    name: textFromKeys(raw, "plugin_name", "name", "display_name", "label") || id || `Plugin ${index + 1}`,
    kind,
    role,
    clipScope: scope.includes("clip") || textFromKeys(raw, "clip_id", "target_clip_id") ? scope || "clip" : "track",
    selected: selectedPlugin.id !== "" && selectedPlugin.id === id,
    bypass: truthy(raw.bypass ?? raw.bypassed ?? raw.disabled ?? raw.is_bypassed),
    path: textFromKeys(raw, "plugin_path", "path", "file_path", "module_path", "vst_path", "binary_path"),
    vendor: textFromKeys(raw, "vendor", "manufacturer", "maker"),
    source: textFromKeys(raw, "track_id", "source_track_id", "owner_track_id"),
    deletable: !truthy(raw.system ?? raw.builtin ?? raw.internal ?? raw.locked ?? raw.protected)
  };
}

function isSystemRackPluginRecord(raw: JsonRecord): boolean {
  if (truthy(raw.system ?? raw.builtin ?? raw.internal ?? raw.hidden ?? raw.is_default ?? raw.default_plugin)) {
    return true;
  }
  if (raw.user_visible === false || raw.visible === false) {
    return true;
  }
  const label = normalizeRackPluginText(textFromKeys(raw, "plugin_name", "name", "display_name", "label"));
  const kind = normalizeRackPluginText(textFromKeys(raw, "type", "plugin_type", "kind", "role", "slot"));
  if (label === "volumepanplugin" || label === "volumepan" || label === "trackvolumepan") {
    return true;
  }
  if (label === "levelmeter" || label === "levelmeterlevel" || label === "tracklevelmeter") {
    return true;
  }
  if ((kind.includes("system") || kind.includes("builtin") || kind.includes("internal")) && (label.includes("meter") || label.includes("volumepan"))) {
    return true;
  }
  return false;
}

function normalizeRackPluginText(value: string): string {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, "");
}

function selectedTrackIDFromUIState(uiState: AgentUIState | null): string {
  const selectedTrack = asRecord(uiState?.selected_track);
  const context = asRecord(uiState?.ui_context);
  const selectedPlugin = asRecord(uiState?.selected_plugin);
  return textValue(
    selectedTrack.track_id ??
      selectedTrack.id ??
      context.selected_track_id ??
      context.selected_clip_track_id ??
      context.selected_plugin_track_id ??
      selectedPlugin.track_id,
    ""
  );
}

function selectedClipIDFromUIState(uiState: AgentUIState | null): string {
  const context = asRecord(uiState?.ui_context);
  const selectedClip = asRecord(asRecord(uiState).selected_clip);
  const ids = firstArray(context.selected_clip_ids, selectedClip.selected_clip_ids);
  return textValue(selectedClip.clip_id ?? selectedClip.id ?? context.selected_clip_id ?? ids[0], "");
}

function selectedMidiClipIDFromUIState(uiState: AgentUIState | null): string {
  const context = asRecord(uiState?.ui_context);
  return textValue(context.piano_roll_focus_clip_id ?? context.active_midi_clip_id ?? selectedClipIDFromUIState(uiState), "");
}

function selectedMidiTrackIDFromUIState(uiState: AgentUIState | null): string {
  const context = asRecord(uiState?.ui_context);
  return textValue(context.piano_roll_focus_track_id ?? context.active_midi_track_id ?? context.selected_clip_track_id ?? selectedTrackIDFromUIState(uiState), "");
}

function selectedClipIDsFromUIState(uiState: AgentUIState | null): Set<string> {
  const context = asRecord(uiState?.ui_context);
  const selected = selectedClipIDFromUIState(uiState);
  const ids = firstArray(context.selected_clip_ids)
    .map((id) => textValue(id, ""))
    .filter(Boolean);
  if (selected) {
    ids.push(selected);
  }
  return new Set(ids);
}

function selectedPluginFromUIState(uiState: AgentUIState | null): { id: string; trackID: string; name: string } {
  const plugin = asRecord(uiState?.selected_plugin);
  const context = asRecord(uiState?.ui_context);
  const id = textValue(plugin.plugin_id ?? plugin.id ?? plugin.node_id ?? plugin.plugin_item_id ?? context.selected_plugin_id, "");
  return {
    id,
    trackID: textValue(plugin.track_id ?? plugin.selected_plugin_track_id ?? context.selected_plugin_track_id ?? context.selected_track_id, ""),
    name: textValue(plugin.plugin_name ?? plugin.name ?? context.selected_plugin_name, "")
  };
}

function isMidiClip(clip: DawClip): boolean {
  const type = clip.type.toLowerCase();
  return type.includes("midi") || type.includes("sequencer") || clip.notes.length > 0 || clip.noteCount > 0;
}

async function focusDawTarget(onInvoke: DawInvoke, target: JsonRecord): Promise<void> {
  const trackID = textValue(target.track_id ?? target.target_track_id, "");
  const clipID = textValue(target.clip_id ?? target.selected_clip_id, "");
  const pluginID = textValue(target.plugin_id ?? target.node_id ?? target.plugin_item_id, "");
  if (pluginID) {
    await onInvoke("select_plugin", { track_id: trackID, plugin_id: pluginID }, "ask_vit_webui_focus", true);
  } else if (clipID) {
    await onInvoke("select_clip", { track_id: trackID, clip_id: clipID, start_seconds: target.start_seconds, start_beats: target.start_beats }, "ask_vit_webui_focus", true);
  } else if (trackID) {
    await onInvoke("select_track", { track_id: trackID }, "ask_vit_webui_focus", true);
  }
  const seekSeconds = finiteNumber(target.start_seconds ?? target.start_time ?? target.time, NaN);
  if (clipID && Number.isFinite(seekSeconds) && seekSeconds >= 0) {
    await onInvoke("transport.seek", { time: seekSeconds }, "ask_vit_webui_focus_seek", true);
  }
}

async function auditionDawTarget(onInvoke: DawInvoke, target: JsonRecord): Promise<void> {
  await focusDawTarget(onInvoke, target);
  await onInvoke("transport.play", {}, "ask_vit_webui_audition", true);
}

function actionDawTarget(action: JsonRecord, uiState: AgentUIState | null): JsonRecord | null {
  const source = textValue(action._ui_source, "");
  if (source !== "executed" && source !== "event") {
    return null;
  }
  const command = asRecord(action.command);
  const args = asRecord(command.args ?? action.args);
  const payload = asRecord(action.payload);
  const result = asRecord(action.result);
  const verification = asRecord(action.verification);
  const evidence = asRecord(verification.evidence);
  const merged = mergeJsonRecords(action, command, args, payload, result, verification, evidence);
  const commandName = textValue(
    action.command_name ??
      action.tool ??
      action.cmd ??
      action.command ??
      payload.command_name ??
      payload.tool ??
      payload.cmd ??
      payload.command ??
      result.command_name ??
      result.tool ??
      result.cmd ??
      verification.command_name ??
      verification.tool,
    ""
  ).toLowerCase();
  const uiAction = textValue(result.ui_action ?? payload.ui_action ?? action.ui_action, "").toLowerCase();
  if (!actionLooksLikeDawTarget(commandName, uiAction, merged)) {
    return null;
  }

  let trackID = textValue(
    merged.track_id ?? merged.target_track_id ?? merged.selected_track_id ?? merged.source_track_id ?? merged.observed_track_id ?? merged.expected_track_id,
    ""
  );
  const clipID = textValue(
    merged.clip_id ?? merged.target_clip_id ?? merged.selected_clip_id ?? merged.new_clip_id ?? merged.created_clip_id ?? merged.observed_clip_id ?? merged.expected_clip_id,
    ""
  );
  const pluginID = textValue(
    merged.plugin_id ?? merged.plugin_item_id ?? merged.node_id ?? merged.selected_plugin_id ?? merged.observed_plugin_id ?? merged.expected_plugin_id ?? merged.observed_node_id ?? merged.expected_node_id,
    ""
  );
  const clipContext = clipTargetFromUIState(uiState, clipID, trackID);
  if (!trackID) {
    trackID = textValue(clipContext.track_id, "") || pluginTrackIDFromUIState(uiState, pluginID);
  }
  const startSeconds = actionTargetStartSeconds(merged, clipContext, uiState);
  const startBeats = finiteNumber(merged.start_beats ?? merged.start_beat ?? merged.beat, finiteNumber(clipContext.start_beats, NaN));
  if (!trackID && !clipID && !pluginID) {
    return null;
  }
  return {
    track_id: trackID,
    clip_id: clipID,
    plugin_id: pluginID,
    start_seconds: Number.isFinite(startSeconds) ? startSeconds : undefined,
    start_beats: Number.isFinite(startBeats) ? startBeats : undefined,
    can_audition: actionTargetCanAudition(commandName, uiAction, { track_id: trackID, clip_id: clipID, plugin_id: pluginID })
  };
}

function actionLooksLikeDawTarget(commandName: string, uiAction: string, payload: JsonRecord): boolean {
  if (uiAction || textValue(payload.track_id ?? payload.clip_id ?? payload.plugin_id ?? payload.new_clip_id ?? payload.created_clip_id, "")) {
    return true;
  }
  return /^(track|midi|clip|plugin|rack|control|transport)\b/.test(commandName) || /(track|midi|clip|plugin|rack|control|transport)/.test(commandName);
}

function actionTargetCanAudition(commandName: string, uiAction: string, target: JsonRecord): boolean {
  if (textValue(target.clip_id ?? target.plugin_id, "")) {
    return true;
  }
  if (!textValue(target.track_id, "")) {
    return false;
  }
  const joined = `${commandName} ${uiAction}`;
  if (/add_track|create_track|rename_track|delete_track|select_track/.test(joined)) {
    return false;
  }
  return /mute|solo|arm|volume|pan|mixer|plugin|rack|control/.test(joined);
}

function actionTargetStartSeconds(payload: JsonRecord, clipContext: JsonRecord, uiState: AgentUIState | null): number {
  const directSeconds = finiteNumber(
    payload.start_seconds ??
      payload.start_time_seconds ??
      payload.position_seconds ??
      payload.playhead_seconds ??
      payload.new_start_seconds ??
      payload.time,
    NaN
  );
  if (Number.isFinite(directSeconds)) {
    return directSeconds;
  }
  const bpm = bpmFromUIState(uiState);
  const beatSeconds = 60 / Math.max(1, bpm);
  const timeUnit = textValue(payload.time_unit ?? payload.unit, "").toLowerCase();
  const startTime = finiteNumber(payload.start_time ?? payload.start, NaN);
  if (Number.isFinite(startTime)) {
    return timeUnit.includes("beat") ? startTime * beatSeconds : startTime;
  }
  const newStart = finiteNumber(payload.new_start, NaN);
  if (Number.isFinite(newStart)) {
    return timeUnit.includes("beat") ? newStart * beatSeconds : newStart;
  }
  const startBeats = finiteNumber(payload.start_beats ?? payload.start_beat ?? payload.beat, NaN);
  if (Number.isFinite(startBeats)) {
    return startBeats * beatSeconds;
  }
  return finiteNumber(clipContext.start_seconds, NaN);
}

function clipTargetFromUIState(uiState: AgentUIState | null, clipID: string, trackID = ""): JsonRecord {
  if (!clipID) {
    return {};
  }
  for (const track of dawTracksFromUIState(uiState)) {
    if (trackID && track.id !== trackID) {
      continue;
    }
    const clip = track.clips.find((item) => item.id === clipID);
    if (clip) {
      return {
        track_id: track.id,
        clip_id: clip.id,
        start_seconds: clip.startSeconds,
        start_beats: clip.startBeats
      };
    }
  }
  return {};
}

function pluginTrackIDFromUIState(uiState: AgentUIState | null, pluginID: string): string {
  if (!pluginID) {
    return "";
  }
  for (const track of dawTracksFromUIState(uiState)) {
    if (track.plugins.some((plugin) => plugin.id === pluginID || plugin.itemID === pluginID)) {
      return track.id;
    }
  }
  return "";
}

function clipBlockStyle(clip: DawClip, pixelsPerSecond: number, timelineWidth: number, dragDeltaX = 0, dragDeltaY = 0): React.CSSProperties {
  const safeScale = clampNumber(pixelsPerSecond, 0.05, 80);
  const left = Math.max(0, clip.startSeconds * safeScale);
  const width = clampNumber(clip.lengthSeconds * safeScale, 18, Math.max(18, timelineWidth - left));
  return {
    left: `${left}px`,
    width: `${width}px`,
    transform: dragDeltaX !== 0 || dragDeltaY !== 0 ? `translate(${dragDeltaX}px, ${dragDeltaY}px)` : undefined
  };
}

function formatTimelineSeconds(seconds: number): string {
  const safe = Math.max(0, Math.round(seconds));
  if (safe < 60) {
    return `${safe}s`;
  }
  const minutes = Math.floor(safe / 60);
  const rest = safe % 60;
  return `${minutes}:${String(rest).padStart(2, "0")}`;
}

function clipActivitySegments(raw: JsonRecord, notes: JsonRecord[], lengthBeats: number): DawActivitySegment[] {
  if (notes.length > 0) {
    return midiActivitySegments(notes, lengthBeats);
  }
  const declared = firstArray(raw.activity, raw.activity_segments).map(asRecord);
  if (declared.length > 0) {
    return declared
      .map((segment) => ({
        left: clampNumber(finiteNumber(segment.left ?? segment.start_percent ?? segment.start, 0), 0, 100),
        width: clampNumber(finiteNumber(segment.width ?? segment.duration_percent ?? segment.length, 2), 0.5, 100),
        height: clampNumber(finiteNumber(segment.height ?? segment.value ?? segment.amplitude, 45), 8, 100)
      }))
      .filter((segment) => segment.width > 0);
  }
  const peaks = numericSeriesFromAny(
    firstPresentArray(raw.peaks, raw.waveform_peaks, raw.waveform, raw.rms, raw.amplitudes, asRecord(raw.waveform_data).peaks)
  );
  if (peaks.length === 0) {
    return [];
  }
  const bucketCount = Math.min(64, peaks.length);
  const bucketSize = Math.max(1, Math.ceil(peaks.length / bucketCount));
  const segments: DawActivitySegment[] = [];
  for (let bucket = 0; bucket < bucketCount; bucket += 1) {
    const slice = peaks.slice(bucket * bucketSize, (bucket + 1) * bucketSize);
    if (slice.length === 0) {
      continue;
    }
    const amplitude = slice.reduce((max, value) => Math.max(max, Math.abs(value)), 0);
    if (amplitude <= 0.001) {
      continue;
    }
    segments.push({
      left: (bucket / bucketCount) * 100,
      width: Math.max(0.8, 100 / bucketCount),
      height: clampNumber(amplitude * 100, 8, 100)
    });
  }
  return segments;
}

function midiActivitySegments(notes: JsonRecord[], lengthBeats: number): DawActivitySegment[] {
  const safeLength = Math.max(0.25, lengthBeats);
  return notes.slice(0, 120).map((note) => {
    const start = finiteNumber(note.start ?? note.start_beats ?? note.beat, 0);
    const duration = Math.max(0.05, finiteNumber(note.length ?? note.length_beats ?? note.duration, 0.25));
    const velocity = clampNumber(finiteNumber(note.velocity ?? note.vel, 96), 1, 127);
    return {
      left: clampNumber((start / safeLength) * 100, 0, 100),
      width: clampNumber((duration / safeLength) * 100, 0.7, 100),
      height: clampNumber((velocity / 127) * 100, 14, 100)
    };
  });
}

function midiNoteEndBeats(notes: JsonRecord[]): number {
  return notes.reduce((end, note) => {
    const start = finiteNumber(note.start ?? note.start_beats ?? note.beat, 0);
    const length = Math.max(0.05, finiteNumber(note.length ?? note.length_beats ?? note.duration, 0.25));
    return Math.max(end, start + length);
  }, 0);
}

function numericSeriesFromAny(value: unknown): number[] {
  if (!Array.isArray(value)) {
    return [];
  }
  return value
    .map((item) => {
      if (Array.isArray(item)) {
        const values = item.map(Number).filter(Number.isFinite);
        return values.length > 0 ? Math.max(...values.map((next) => Math.abs(next))) : NaN;
      }
      const record = asRecord(item);
      if (Object.keys(record).length > 0) {
        return finiteNumber(record.peak ?? record.rms ?? record.value ?? record.amplitude ?? record.height, NaN);
      }
      return Number(item);
    })
    .filter(Number.isFinite)
    .map((value) => Math.abs(value));
}

function bpmFromUIState(uiState: AgentUIState | null): number {
  const transport = asRecord(uiState?.transport);
  const project = asRecord(uiState?.project);
  return Math.max(1, finiteNumber(transport.bpm ?? transport.tempo ?? project.bpm ?? project.tempo, 120));
}

function trackLevelDB(raw: JsonRecord): number | null {
  const direct = numberFromKeys(raw, "level_db", "levelDb", "meter_db", "current_db", "peak_db", "peakDb", "meter_peak_db", "meter_level_db");
  if (direct != null) {
    return direct;
  }
  const stereo = [
    numberFromKeys(raw, "left_level_db", "leftLevelDb", "left_peak_db", "leftPeakDb", "level_l_db", "peak_l_db"),
    numberFromKeys(raw, "right_level_db", "rightLevelDb", "right_peak_db", "rightPeakDb", "level_r_db", "peak_r_db")
  ].filter((value): value is number => value != null);
  if (stereo.length > 0) {
    return Math.max(...stereo);
  }
  const meters = asRecord(raw.meter ?? raw.levels ?? raw.telemetry);
  const nestedDirect = numberFromKeys(meters, "level_db", "levelDb", "meter_db", "current_db", "peak_db", "peakDb", "meter_peak_db", "meter_level_db", "left_db", "rms_db");
  if (nestedDirect != null) {
    return nestedDirect;
  }
  const nestedStereo = [
    numberFromKeys(meters, "left_level_db", "leftLevelDb", "left_peak_db", "leftPeakDb", "level_l_db", "peak_l_db"),
    numberFromKeys(meters, "right_level_db", "rightLevelDb", "right_peak_db", "rightPeakDb", "level_r_db", "peak_r_db")
  ].filter((value): value is number => value != null);
  return nestedStereo.length > 0 ? Math.max(...nestedStereo) : null;
}

function numberFromKeys(record: JsonRecord, ...keys: string[]): number | null {
  for (const key of keys) {
    if (!(key in record)) {
      continue;
    }
    const value = Number(record[key]);
    if (Number.isFinite(value)) {
      return value;
    }
  }
  return null;
}

function textFromKeys(record: JsonRecord, ...keys: string[]): string {
  for (const key of keys) {
    const value = textValue(record[key], "");
    if (value) {
      return value;
    }
  }
  return "";
}

function valueFromKeys(record: JsonRecord, ...keys: string[]): unknown {
  for (const key of keys) {
    if (key in record && record[key] !== undefined && record[key] !== null && textValue(record[key], "") !== "") {
      return record[key];
    }
  }
  return undefined;
}

function hasArrayValue(record: JsonRecord, ...keys: string[]): boolean {
  return keys.some((key) => Array.isArray(record[key]));
}

function firstPresentArray(...values: unknown[]): unknown[] {
  for (const value of values) {
    if (Array.isArray(value)) {
      return value;
    }
  }
  return [];
}

export type {
  DawInvoke,
  DawActivitySegment,
  DawClip,
  DawPlugin,
  DawTrack
};

export {
  dawTracksFromUIState,
  dawTrackFromRecord,
  dawClipFromRecord,
  dawPluginFromRecord,
  isSystemRackPluginRecord,
  normalizeRackPluginText,
  selectedTrackIDFromUIState,
  selectedClipIDFromUIState,
  selectedMidiClipIDFromUIState,
  selectedMidiTrackIDFromUIState,
  selectedClipIDsFromUIState,
  selectedPluginFromUIState,
  isMidiClip,
  focusDawTarget,
  auditionDawTarget,
  actionDawTarget,
  actionLooksLikeDawTarget,
  actionTargetCanAudition,
  actionTargetStartSeconds,
  clipTargetFromUIState,
  pluginTrackIDFromUIState,
  clipBlockStyle,
  formatTimelineSeconds,
  clipActivitySegments,
  midiActivitySegments,
  midiNoteEndBeats,
  numericSeriesFromAny,
  bpmFromUIState,
  trackLevelDB,
  numberFromKeys,
  textFromKeys,
  valueFromKeys,
  hasArrayValue,
  firstPresentArray
};
