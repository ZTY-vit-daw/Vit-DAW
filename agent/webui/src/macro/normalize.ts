// T9-PRE-1A-1（施工图 UI-CONVERGE-RECON-1 §3.1 批 1 / 步 1c）：App.tsx 宏归一化/补丁函数群（N 域纯函数段）出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
// statusLabel 仍属 App 动作卡域（步 1e 未来落点），经 App 兼容层引用（函数声明，循环安全）；
// MacroValueOverride 类型仍声明于 App.tsx 头部类型区，此处 type-only 引入（编译期擦除，零运行时耦合）。

import type { AgentUIState, JsonRecord, MacroControl, MacroControlBinding } from "../types";
import {
  asRecord,
  clampNumber,
  finiteNumber,
  firstArray,
  numericValue,
  textValue,
  truthy,
  uniqueID
} from "../lib/util";
import { statusLabel, type MacroValueOverride } from "../App";

function macroControlsFromUIState(uiState: AgentUIState | null): MacroControl[] {
  const capabilities = asRecord(uiState?.capabilities);
  const uiContext = asRecord(uiState?.ui_context);
  return mergeMacroControls(
    firstArray(uiState?.macro_controls).map((item) => normalizeMacroControl(asRecord(item))),
    firstArray(capabilities.macro_controls, capabilities.macros, capabilities.macro_panels, capabilities.quick_actions).map((item) => normalizeMacroControl(asRecord(item))),
    macroRowsFromAny(uiContext.rack_control_macros).map((item) => normalizeMacroControl(asRecord(item))),
    macroRowsFromAny(uiContext.active_macro_controls).map((item) => normalizeMacroControl(asRecord(item)))
  );
}

function patchMacroControlValueInUIState(uiState: AgentUIState, macroID: string, value: number, macro?: MacroControl): AgentUIState {
  const capabilities = asRecord(uiState.capabilities);
  const uiContext = asRecord(uiState.ui_context);
  const rackControls = asRecord(uiContext.rack_control_macros);
  const shouldPatchTrackVolume = macro?.control === "track.volume" && Boolean(macro.track_id);
  return {
    ...uiState,
    tracks: shouldPatchTrackVolume ? patchTrackVolumeList(uiState.tracks, macro.track_id ?? "", value) as JsonRecord[] | undefined : uiState.tracks,
    selected_track: shouldPatchTrackVolume ? patchTrackVolumeRecord(uiState.selected_track, macro.track_id ?? "", value) as JsonRecord | null | undefined : uiState.selected_track,
    macro_controls: patchMacroControlValueList(uiState.macro_controls, macroID, value) as MacroControl[] | undefined,
    capabilities: {
      ...capabilities,
      macro_controls: patchMacroControlValueList(capabilities.macro_controls, macroID, value)
    },
    ui_context: {
      ...uiContext,
      active_macro_controls: patchMacroControlValueList(uiContext.active_macro_controls, macroID, value),
      rack_control_macros: {
        ...rackControls,
        macros: patchMacroControlValueList(rackControls.macros, macroID, value)
      }
    }
  };
}

function applyMacroValueOverridesToUIState(uiState: AgentUIState, overrides: Record<string, MacroValueOverride>): AgentUIState {
  const now = Date.now();
  let next = uiState;
  Object.entries(overrides).forEach(([macroID, override]) => {
    if (!override || override.expiresAt <= now) {
      delete overrides[macroID];
      return;
    }
    const trackVolume = uiStateTrackVolumeDB(next, override.macro.track_id ?? "");
    if (trackVolume != null && Math.abs(trackVolume - override.value) <= 0.05) {
      delete overrides[macroID];
      next = patchMacroControlValueInUIState(next, macroID, override.value, override.macro);
      return;
    }
    next = patchMacroControlValueInUIState(next, macroID, override.value, override.macro);
  });
  return next;
}

function uiStateTrackVolumeDB(uiState: AgentUIState, trackID: string): number | null {
  if (!trackID) {
    return null;
  }
  for (const track of firstArray(uiState.tracks).map(asRecord)) {
    const id = textValue(track.track_id ?? track.id, "");
    if (id !== trackID) {
      continue;
    }
    const value = finiteNumber(track.volume_db ?? track.volumeDb ?? track.fader_db ?? track.faderDb ?? track.gain_db ?? track.gainDb ?? track.db, NaN);
    return Number.isNaN(value) ? null : value;
  }
  return null;
}

function patchTrackVolumeList(raw: unknown, trackID: string, value: number): unknown {
  if (!Array.isArray(raw) || !trackID) {
    return raw;
  }
  return raw.map((item) => patchTrackVolumeRecord(item, trackID, value));
}

function patchTrackVolumeRecord(raw: unknown, trackID: string, value: number): unknown {
  const record = asRecord(raw);
  if (Object.keys(record).length === 0 || !trackID) {
    return raw;
  }
  const id = textValue(record.track_id ?? record.id, "");
  if (id !== trackID) {
    return raw;
  }
  return { ...record, volume_db: value, gain_db: value, fader_db: value, db: value };
}

function patchMacroControlValueList(raw: unknown, macroID: string, value: number): unknown {
  if (!Array.isArray(raw)) {
    return raw;
  }
  return raw.map((item) => {
    const record = asRecord(item);
    const id = textValue(record.macro_id ?? record.id ?? record.control_id, "");
    if (id !== macroID) {
      return item;
    }
    return { ...record, value };
  });
}

function patchMacroControlNameInUIState(uiState: AgentUIState, macroID: string, name: string): AgentUIState {
  const capabilities = asRecord(uiState.capabilities);
  const uiContext = asRecord(uiState.ui_context);
  const rackControls = asRecord(uiContext.rack_control_macros);
  return {
    ...uiState,
    macro_controls: patchMacroControlNameList(uiState.macro_controls, macroID, name) as MacroControl[] | undefined,
    capabilities: {
      ...capabilities,
      macro_controls: patchMacroControlNameList(capabilities.macro_controls, macroID, name),
      macros: patchMacroControlNameList(capabilities.macros, macroID, name),
      macro_panels: patchMacroControlNameList(capabilities.macro_panels, macroID, name),
      quick_actions: patchMacroControlNameList(capabilities.quick_actions, macroID, name)
    },
    ui_context: {
      ...uiContext,
      active_macro_controls: patchMacroControlNameList(uiContext.active_macro_controls, macroID, name),
      rack_control_macros: {
        ...rackControls,
        macros: patchMacroControlNameList(rackControls.macros, macroID, name)
      }
    }
  };
}

function patchMacroControlNameList(raw: unknown, macroID: string, name: string): unknown {
  if (!Array.isArray(raw)) {
    return raw;
  }
  return raw.map((item) => {
    const record = asRecord(item);
    const id = textValue(record.macro_id ?? record.id ?? record.control_id, "");
    if (id !== macroID) {
      return item;
    }
    return { ...record, name, label: name, title: name };
  });
}

function macroRowsFromAny(raw: unknown): JsonRecord[] {
  if (Array.isArray(raw)) {
    return raw.map(asRecord).filter((item) => Object.keys(item).length > 0);
  }
  const record = asRecord(raw);
  if (Object.keys(record).length === 0) {
    return [];
  }
  const nested = firstArray(record.macros, record.macro_controls, record.controls, record.items);
  if (nested.length > 0) {
    return nested.map(asRecord).filter((item) => Object.keys(item).length > 0);
  }
  return record.macro_id || record.id ? [record] : [];
}

function normalizeMacroControl(raw: JsonRecord): MacroControl {
  const macroID = textValue(raw.macro_id ?? raw.id ?? raw.control_id, "");
  const name = textValue(raw.name ?? raw.label ?? raw.title, macroID || "Macro");
  const trackID = textValue(raw.track_id ?? raw.track, "");
  const control = inferMacroControlKind(raw, macroID, name);
  let min = finiteNumber(raw.min ?? raw.minimum, 0);
  let max = Math.max(min + 0.0001, finiteNumber(raw.max ?? raw.maximum, 1));
  if (control === "track.volume" && min === 0 && max === 1) {
    min = -60;
    max = 12;
  }
  const value = clampNumber(finiteNumber(raw.value ?? raw.default, min), min, max);
  const bindings = firstArray(raw.bindings, raw.targets).map((item) => normalizeMacroBinding(asRecord(item)));
  if (bindings.length === 0 && control === "track.volume" && trackID) {
    bindings.push({
      binding_id: `binding_${macroID || "macro"}_track_volume`,
      control: "track.volume",
      track_id: trackID,
      plugin_id: "",
      param_id: "track.volume",
      param_name: "Track volume",
      source_min: 0,
      source_max: 1,
      target_min: min,
      target_max: max,
      enabled: true,
      unit: textValue(raw.unit, "dB")
    });
  }
  return {
    macro_id: macroID,
    id: macroID,
    name,
    track_id: trackID,
    plugin_id: textValue(raw.plugin_id ?? raw.plugin ?? raw.node_id, ""),
    role: textValue(raw.role, ""),
    control,
    control_type: textValue(raw.control_type ?? raw.type, "slider"),
    value,
    min,
    max,
    unit: textValue(raw.unit, control === "track.volume" ? "dB" : ""),
    bindings: normalizeTrackVolumeMacroBindings(bindings, control, min, max, textValue(raw.unit, control === "track.volume" ? "dB" : "")),
    binding_count: Math.max(bindings.length, numericValue(raw.binding_count)),
    source: textValue(raw.source ?? raw.created_by, "godot_rack"),
    status: textValue(raw.status ?? raw.state, "ready"),
    enabled: raw.enabled === undefined ? true : truthy(raw.enabled),
    updated_at: textValue(raw.updated_at, ""),
    updated_at_unix: numericValue(raw.updated_at_unix)
  };
}

function normalizeMacroBinding(raw: JsonRecord) {
  return {
    binding_id: textValue(raw.binding_id ?? raw.id, ""),
    control: textValue(raw.control, ""),
    track_id: textValue(raw.track_id ?? raw.track, ""),
    plugin_id: textValue(raw.plugin_id ?? raw.plugin ?? raw.node_id, ""),
    param_id: textValue(raw.param_id ?? raw.parameter_id ?? raw.key, ""),
    param_name: textValue(raw.param_name ?? raw.parameter_name ?? raw.label ?? raw.name, ""),
    source_min: finiteNumber(raw.source_min, 0),
    source_max: finiteNumber(raw.source_max, 1),
    target_min: finiteNumber(raw.target_min ?? raw.min, 0),
    target_max: finiteNumber(raw.target_max ?? raw.max, 1),
    enabled: raw.enabled === undefined ? true : truthy(raw.enabled),
    unit: textValue(raw.unit, "")
  };
}

function normalizeTrackVolumeMacroBindings(bindings: MacroControlBinding[], control: string, min: number, max: number, unit: string): MacroControlBinding[] {
  if (control !== "track.volume") {
    return bindings;
  }
  return bindings.map((binding) => {
    const isTrackVolumeBinding = binding.control === "track.volume" || binding.param_id === "track.volume";
    if (!isTrackVolumeBinding) {
      return binding;
    }
    return {
      ...binding,
      control: "track.volume",
      param_id: binding.param_id || "track.volume",
      param_name: binding.param_name || "Track volume",
      target_min: min,
      target_max: max,
      unit: binding.unit || unit || "dB"
    };
  });
}

function inferMacroControlKind(raw: JsonRecord, macroID: string, name: string): string {
  const explicit = textValue(raw.control ?? raw.param_id ?? raw.parameter_id, "");
  if (explicit) {
    return explicit;
  }
  const probe = `${macroID} ${name} ${textValue(raw.role, "")}`.toLowerCase();
  if (probe.includes("track_volume") || probe.includes("track.volume")) {
    return "track.volume";
  }
  return "";
}

function mergeMacroControls(...groups: MacroControl[][]): MacroControl[] {
  const byID = new Map<string, MacroControl>();
  groups.flat().forEach((macro) => {
    const normalized = normalizeMacroControl(asRecord(macro));
    const id = macroControlID(normalized);
    if (!id) {
      return;
    }
    byID.set(id, mergeMacroControlRecords(byID.get(id), normalized));
  });
  return Array.from(byID.values()).sort((a, b) => macroControlLabel(a).localeCompare(macroControlLabel(b)));
}

function mergeMacroControlRecords(previous: MacroControl | undefined, next: MacroControl): MacroControl {
  if (!previous) {
    return next;
  }
  const previousBindings = previous.bindings ?? [];
  const nextBindings = next.bindings ?? [];
  const merged: MacroControl = { ...previous, ...next };
  if (previousBindings.length > 0 && nextBindings.length === 0) {
    merged.bindings = previousBindings;
    merged.binding_count = Math.max(previous.binding_count ?? 0, previousBindings.length);
    if (isDefaultMacroRange(next) && !isDefaultMacroRange(previous)) {
      merged.min = previous.min;
      merged.max = previous.max;
      merged.unit = previous.unit;
      merged.value = previous.value;
    }
    if (!merged.control && previous.control) {
      merged.control = previous.control;
    }
    if (!merged.role && previous.role) {
      merged.role = previous.role;
    }
  }
  return normalizeMacroControl(asRecord(merged));
}

function isDefaultMacroRange(macro: MacroControl): boolean {
  return finiteNumber(macro.min, 0) === 0 && finiteNumber(macro.max, 1) === 1;
}

function macroControlID(macro: MacroControl): string {
  return textValue(macro.macro_id ?? macro.id, "");
}

function macroControlLabel(macro: MacroControl): string {
  return textValue(macro.name, macroControlID(macro) || "Macro");
}

function macroControlValue(macro: MacroControl): number {
  const min = finiteNumber(macro.min, 0);
  const max = Math.max(min + 0.0001, finiteNumber(macro.max, 1));
  return clampNumber(finiteNumber(macro.value, min), min, max);
}

function macroControlDisabledReason(macro: MacroControl): string {
  if (!macroControlID(macro)) {
    return "缺少宏控制 ID";
  }
  if (macro.enabled === false) {
    return "宏控制已禁用";
  }
  if ((macro.bindings ?? []).filter((binding) => binding.enabled !== false).length === 0) {
    return "暂无参数绑定，只能引用";
  }
  return "";
}

function macroControlMeta(macro: MacroControl): string {
  const parts = [
    macro.track_id ? `轨道 ${macro.track_id}` : "",
    macro.plugin_id ? `插件 ${macro.plugin_id}` : "",
    statusLabel(macro.status, textValue(macro.status, "ready")),
    textValue(macro.source, "")
  ].filter(Boolean);
  return parts.join(" · ") || "工程宏控制";
}

function macroBindingSummary(macro: MacroControl): string {
  const bindings = macro.bindings ?? [];
  if (bindings.length === 0) {
    return "暂无参数绑定";
  }
  const names = bindings
    .slice(0, 2)
    .map((binding) => textValue(binding.param_name ?? binding.param_id, "参数"))
    .filter(Boolean);
  const suffix = bindings.length > names.length ? ` +${bindings.length - names.length}` : "";
  return `${bindings.length} 个绑定${names.length > 0 ? `，${names.join("、")}${suffix}` : ""}`;
}

function macroValueText(value: number, macro?: MacroControl): string {
  const unit = textValue(macro?.unit, "");
  if (macro?.control === "track.volume" || unit.toLowerCase() === "db") {
    return `${value.toFixed(2)} dB`;
  }
  if (value >= 0 && value <= 1) {
    return `${Math.round(value * 100)}%`;
  }
  return unit ? `${value.toFixed(2)} ${unit}` : value.toFixed(2);
}

function macroPaneSubtitle(count: number, summary: JsonRecord, selectedPlugin: JsonRecord): string {
  const bindingCount = numericValue(summary.binding_count);
  const activeCount = numericValue(summary.active_macro_count);
  const pluginName = textValue(selectedPlugin.plugin_name, "");
  if (pluginName) {
    return `${pluginName} / ${activeCount || count} 个相关宏控 / ${bindingCount} 个绑定`;
  }
  return `${count} 个工程宏控 / ${bindingCount} 个绑定`;
}

function activeWorkflowModeLabel(uiState: AgentUIState | null): string {
  const uiContext = asRecord(uiState?.ui_context);
  const goal = asRecord(uiState?.goal);
  const raw = textValue(uiContext.active_workflow_mode ?? uiContext.active_mode_status ?? goal.status, "即时模式");
  if (raw === "plan") {
    return "计划模式";
  }
  if (raw === "goal") {
    return "目标模式";
  }
  if (raw === "auto_learn") {
    return "插件学习自动学习";
  }
  if (raw === "teach") {
    return "插件学习教学模式";
  }
  return statusLabel(raw, raw);
}

function macroControlCardAction(macro: MacroControl, source: string): JsonRecord {
  return {
    _ui_source: source,
    kind: "macro_control_card",
    type: "macro_control_card",
    id: `macro_card_${macroControlID(macro) || uniqueID("macro")}`,
    macro_id: macroControlID(macro),
    title: macroControlLabel(macro),
    status: macroControlDisabledReason(macro) ? "attention" : "ready",
    macro: normalizeMacroControl(asRecord(macro))
  };
}

function isMacroControlAction(action: JsonRecord): boolean {
  const kind = textValue(action.kind ?? action.type, "").toLowerCase();
  const commandName = textValue(action.command_name ?? action.cmd ?? action.command, "").toLowerCase();
  const result = asRecord(action.result);
  return (
    kind === "macro_control_card" ||
    kind === "macro_control" ||
    kind === "rack_macro_upserted" ||
    kind === "rack_macro_renamed" ||
    commandName === "control.set_macro_values" ||
    commandName === "control_set_macro_values" ||
    commandName === "control.rename_macro" ||
    commandName === "control_rename_macro" ||
    commandName === "plugin_map_macro_to_params" ||
    Boolean(action.macro_id && (asRecord(action.macro).macro_id || asRecord(result.macro).macro_id))
  );
}

function macroControlValueMutation(macro: MacroControl, value: number, commit: boolean, source: string): JsonRecord {
  const normalized = normalizeMacroControl(asRecord({ ...macro, value }));
  return {
    kind: "rack_macro_value_changed",
    macro_id: macroControlID(normalized),
    value,
    commit,
    source,
    requires_refresh: commit
  };
}

function macroControlRenameMutation(macro: MacroControl, name: string, source: string): JsonRecord {
  const normalized = normalizeMacroControl(asRecord({ ...macro, name }));
  return {
    kind: "rack_macro_renamed",
    macro_id: macroControlID(normalized),
    name,
    macro: normalized,
    source,
    requires_refresh: true
  };
}

export {
  macroControlsFromUIState,
  patchMacroControlValueInUIState,
  applyMacroValueOverridesToUIState,
  uiStateTrackVolumeDB,
  patchTrackVolumeList,
  patchTrackVolumeRecord,
  patchMacroControlValueList,
  patchMacroControlNameInUIState,
  patchMacroControlNameList,
  macroRowsFromAny,
  normalizeMacroControl,
  normalizeMacroBinding,
  normalizeTrackVolumeMacroBindings,
  inferMacroControlKind,
  mergeMacroControls,
  mergeMacroControlRecords,
  isDefaultMacroRange,
  macroControlID,
  macroControlLabel,
  macroControlValue,
  macroControlDisabledReason,
  macroControlMeta,
  macroBindingSummary,
  macroValueText,
  macroPaneSubtitle,
  activeWorkflowModeLabel,
  macroControlCardAction,
  isMacroControlAction,
  macroControlValueMutation,
  macroControlRenameMutation
};
