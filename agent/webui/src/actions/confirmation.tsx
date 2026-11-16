// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2a）：App.tsx 动作卡体系全域出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
// confirmation 确认域：摘要/预览/ActionDetails/交互 payload + 响应侧兜底解析。

import type { AgentEvent, ChatMessage, ChatResponse, JsonRecord } from "../types";
import { asRecord, compactJSON, firstArray, stableIDPart, textValue, truthy } from "../lib/util";
import { isSameMixBoardSession, mixBoardActionSessionID } from "../chat/merge";
import { agentEventAction, agentEventSourceID } from "../App";
import { artifactSummariesFrom } from "../media/artifacts";
import { actionRenderID, isCapabilityProposalInteraction, isComposerInteraction, isDisposableConfirmationPromptText } from "./StandardActionCard";
import { isMixBoardAction } from "./MixBoardActionCard";
import { commandSummaryLabel, localizeDisplayText } from "./labels";

export function debugConfirmation(stage: string, detail: unknown): void {
  if (!confirmationDebugEnabled()) {
    return;
  }
  const entry = {
    stage,
    at: new Date().toISOString(),
    detail
  };
  try {
    console.info("[AskVit confirmation]", entry);
  } catch {
    // Diagnostics only.
  }
  try {
    const key = "ask_vit_confirmation_debug";
    const previous = JSON.parse(window.localStorage.getItem(key) || "[]");
    const rows = Array.isArray(previous) ? previous : [];
    rows.push(entry);
    window.localStorage.setItem(key, JSON.stringify(rows.slice(-80)));
  } catch {
    // Diagnostics only.
  }
  try {
    void fetch("/agent/debug/confirmation", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(entry)
    }).catch(() => undefined);
  } catch {
    // Diagnostics only.
  }
}

function confirmationDebugEnabled(): boolean {
  if (typeof window === "undefined") {
    return false;
  }
  try {
    const params = new URLSearchParams(window.location.search);
    return params.get("debug_confirmation") === "1" || window.localStorage.getItem("ask_vit_confirmation_debug_enabled") === "true";
  } catch {
    return false;
  }
}

export function shouldDebugAgentEvents(events: AgentEvent[]): boolean {
  return events.some((event) => {
    const type = textValue(event.type, "").toLowerCase();
    const status = textValue(event.status, "").toLowerCase();
    return (
      type === "approval.requested" ||
      type === "turn.completed" ||
      type === "turn.failed" ||
      type === "item.completed" ||
      status.includes("confirm") ||
      status.includes("waiting") ||
      status.includes("fail") ||
      status.includes("error")
    );
  });
}

export function summarizeAgentEventForConfirmation(event: AgentEvent): JsonRecord {
  return {
    seq: event.seq,
    type: textValue(event.type, ""),
    status: textValue(event.status, ""),
    source_id: agentEventSourceID(event),
    conversation_id: textValue(event.conversation_id, ""),
    goal_id: textValue(event.goal_id, ""),
    run_id: textValue(event.run_id, ""),
    item_id: textValue(event.item_id, ""),
    item_type: textValue(event.item_type, ""),
    title: textValue(event.title, ""),
    body: textValue(event.body, "").slice(0, 180),
    action: summarizeActionForConfirmation(agentEventAction(event))
  };
}

export function summarizeChatResponseForConfirmation(response: ChatResponse): JsonRecord {
  const responseRecord = asRecord(response);
  const interactions = firstArray(response.interaction_requests).map(asRecord);
  return {
    conversation_id: textValue(response.conversation_id, ""),
    needs_confirmation: Boolean(response.needs_confirmation),
    plan_id: textValue(response.plan_id, ""),
    next_plan_id: textValue(responseRecord.next_plan_id, ""),
    goal_status: textValue(response.goal_status, ""),
    status: textValue(response.status, ""),
    reply: textValue(response.reply ?? response.message, "").slice(0, 180),
    preview_present: textValue(response.preview, "").length > 0,
    artifact_count: firstArray(response.artifacts).length,
    artifacts: artifactSummariesFrom(response.artifacts).map((artifact) => ({
      id: artifact.id,
      title: textValue(artifact.title, ""),
      kind: textValue(artifact.kind, ""),
      mime: textValue(artifact.mime, "")
    })),
    interaction_count: interactions.length,
    interactions: interactions.map(summarizeActionForConfirmation),
    command_count: firstArray(response.commands).length,
    executed_count: firstArray(response.executed_kernel_reply).length,
    project_result_card_count: firstArray(response.project_result_cards).length
  };
}

export function summarizeMessageForConfirmation(message: ChatMessage): JsonRecord {
  return {
    id: textValue(message.id, ""),
    source_id: textValue(message.source_id, ""),
    role: message.role,
    content: textValue(message.content, "").slice(0, 180),
    status: textValue(message.status, ""),
    artifact_count: message.artifacts?.length ?? 0,
    artifacts: (message.artifacts ?? []).map((artifact) => ({
      id: artifact.id,
      title: textValue(artifact.title, ""),
      kind: textValue(artifact.kind, ""),
      mime: textValue(artifact.mime, "")
    })),
    disposable_confirmation_prompt: isDisposableConfirmationPromptText(textValue(message.content, "")),
    action_count: message.actions?.length ?? 0,
    composer_actions: (message.actions ?? []).map(asRecord).filter(isComposerInteraction).map(summarizeActionForConfirmation),
    actions: (message.actions ?? []).map(asRecord).map(summarizeActionForConfirmation)
  };
}

export function summarizeMessagesForConfirmation(messages: ChatMessage[]): JsonRecord[] {
  return messages.map(summarizeMessageForConfirmation).filter((message) => {
    const content = textValue(message.content, "");
    return (
      Number(message.action_count ?? 0) > 0 ||
      Number(message.artifact_count ?? 0) > 0 ||
      content.toLowerCase().includes("confirm") ||
      content.includes("确认") ||
      Boolean(message.disposable_confirmation_prompt)
    );
  });
}

export function summarizeActionForConfirmation(action: unknown): JsonRecord | null {
  const record = asRecord(action);
  if (Object.keys(record).length === 0) {
    return null;
  }
  return {
    id: textValue(record.id ?? record.interaction_id, ""),
    render_id: actionRenderID(record),
    ui_source: textValue(record._ui_source, ""),
    synthetic: truthy(record._synthetic_confirmation),
    is_confirmation: isConfirmationAction(record),
    is_composer: isComposerInteraction(record),
    kind: textValue(record.kind, ""),
    type: textValue(record.type, ""),
    status: textValue(record.status ?? record.stage, ""),
    plan_id: textValue(record.plan_id ?? asRecord(record.payload).plan_id ?? asRecord(record.data).plan_id, ""),
    child_action_count: firstArray(record.actions).length,
    child_actions: firstArray(record.actions).map(asRecord).map((child) => ({
      id: textValue(child.id ?? child.action_id, ""),
      label: textValue(child.label ?? child.title, ""),
      style: textValue(child.style, "")
    }))
  };
}

export function isConfirmationAction(action: JsonRecord): boolean {
  if (isMixTreatmentPendingAction(action)) {
    return false;
  }
  const kind = textValue(action.kind, "").toLowerCase();
  const type = textValue(action.type, "").toLowerCase();
  const status = textValue(action.status ?? action.stage, "").toLowerCase();
  const payload = asRecord(action.payload);
  return (
    kind === "confirmation" ||
    type === "confirmation" ||
    type === "approval.requested" ||
    truthy(action.requires_confirmation) ||
    truthy(action.needs_confirmation) ||
    status.includes("waiting_confirmation") ||
    status.includes("needs_confirmation") ||
    textValue(action.plan_id ?? payload.plan_id, "") !== ""
  );
}

export function isPendingInteractionAction(action: JsonRecord): boolean {
  return isConfirmationAction(action) || isMixTreatmentPendingAction(action) || isComposerInteraction(action);
}

export function isMixTreatmentPendingAction(action: JsonRecord): boolean {
  const payload = interactionPayload(action);
  const kind = textValue(action.kind, "").toLowerCase();
  const type = textValue(action.type, "").toLowerCase();
  const workflow = textValue(action.workflow ?? payload.workflow, "").toLowerCase();
  return kind === "mix_treatment_confirmation" || type === "mix_treatment_confirmation" || workflow === "mix_treatment";
}

export function mixTreatmentPendingLabel(actionKind: string, processor: string): string {
  const kind = actionKind.trim().toLowerCase();
  const proc = processor.trim().toLowerCase();
  if (kind === "gain_balance") {
    return "电平平衡";
  }
  if (kind === "pan_balance") {
    return "声像调整";
  }
  if (kind === "plugin_treatment") {
    return proc ? `${localizeDisplayText(proc)} 处理` : "插件处理";
  }
  return localizeDisplayText(actionKind || processor || "混音处理");
}

export function mixTreatmentPendingValueLabel(actionKind: string, deltaDB: string, deltaPan: string, targetPan: string): string {
  const kind = actionKind.trim().toLowerCase();
  if (kind === "gain_balance" && deltaDB) {
    return `${formatSignedNumber(deltaDB)} dB`;
  }
  if (kind === "pan_balance") {
    if (targetPan) {
      return `目标 ${formatPanValue(targetPan)}`;
    }
    if (deltaPan) {
      return `变化 ${formatSignedNumber(deltaPan)}`;
    }
  }
  return "等待确认后执行";
}

function formatSignedNumber(value: string): string {
  const number = Number(value);
  if (!Number.isFinite(number)) {
    return value;
  }
  return number > 0 ? `+${number}` : String(number);
}

function formatPanValue(value: string): string {
  const number = Number(value);
  if (!Number.isFinite(number)) {
    return value;
  }
  if (Math.abs(number) < 0.0001) {
    return "居中";
  }
  return number < 0 ? `左 ${Math.abs(number)}` : `右 ${number}`;
}

export function confirmationActionSummary(action: JsonRecord): string {
  const commands = confirmationCommands(action);
  const preview = confirmationPreviewText(action);
  const directBody = textValue(action.body ?? action.message ?? action.description, "");
  const commandSummary = summarizeConfirmationCommands(commands, preview);
  if (commandSummary) {
    return commandSummary;
  }
  if (directBody && !looksLikeInternalInstruction(directBody)) {
    return directBody;
  }
  if (preview && !looksLikeInternalInstruction(preview)) {
    return preview;
  }
  return "这个操作会修改工程，需要你确认后执行中";
}

function confirmationCommands(action: JsonRecord): JsonRecord[] {
  const payload = asRecord(action.payload);
  const data = asRecord(action.data);
  const command = asRecord(action.command);
  const out: JsonRecord[] = [];
  const seen = new Set<string>();
  const pushCommand = (value: unknown) => {
    const record = asRecord(value);
    if (Object.keys(record).length === 0) {
      return;
    }
    if (!confirmationRecordHasCommandContent(record)) {
      return;
    }
    const key = compactJSON(record);
    if (seen.has(key)) {
      return;
    }
    seen.add(key);
    out.push(record);
  };
  const pushCommands = (value: unknown) => firstArray(value).forEach(pushCommand);
  pushCommands(action.commands);
  pushCommands(payload.commands);
  pushCommands(data.commands);
  pushCommands(command.commands);
  [action.command, payload.command, data.command, action.value, payload.value, data.value].forEach(pushCommand);
  firstArray(action.review_items).map(asRecord).forEach((item) => {
    const itemPayload = asRecord(item.payload);
    pushCommands(item.commands);
    pushCommands(itemPayload.commands);
    [item.command, itemPayload.command, item.value, itemPayload.value].forEach(pushCommand);
  });
  return out;
}

function confirmationRecordHasCommandContent(record: JsonRecord): boolean {
  if (textValue(record.name ?? record.command_name ?? record.tool ?? record.cmd ?? record.action ?? record.command, "") !== "") {
    return true;
  }
  if (Object.keys(asRecord(record.command)).length > 0 || Object.keys(asRecord(record.args ?? record.arguments)).length > 0) {
    return true;
  }
  return firstArray(record.operations, record.notes, record.midi_notes, record.note_rows).length > 0;
}

function confirmationPreviewText(action: JsonRecord): string {
  const payload = asRecord(action.payload);
  const command = asRecord(action.command);
  return textValue(action.preview ?? payload.preview ?? command.preview, "");
}

export function confirmationReviewItems(action: JsonRecord): JsonRecord[] {
  const commands = confirmationCommands(action);
  const preview = confirmationPreviewText(action);
  const rows: JsonRecord[] = [];
  commands.forEach((command, index) => {
    rows.push(...confirmationCommandReviewItems(command, index, preview));
  });
  if (rows.length === 0) {
    if (preview && !looksLikeInternalInstruction(preview)) {
      rows.push({ id: "confirmation-preview", title: "确认内容", body: preview });
    }
  }
  return rows;
}

export function mergeReviewItems(primary: JsonRecord[], secondary: JsonRecord[]): JsonRecord[] {
  const seen = new Set<string>();
  const out: JsonRecord[] = [];
  [...primary, ...secondary].forEach((item, index) => {
    const key = textValue(item.id ?? item.title, `review-${index}`);
    if (seen.has(key)) {
      return;
    }
    seen.add(key);
    out.push(item);
  });
  return out;
}

function confirmationCommandReviewItems(command: JsonRecord, index: number, preview = ""): JsonRecord[] {
  const payload = confirmationCommandPayload(command);
  const name = confirmationCommandName(command, payload);
  const rows: JsonRecord[] = [];
  if (name.includes("apply_note_patch") || name.includes("add_midi_notes") || name.includes("midi_note")) {
    const notes = confirmationNoteRows(payload);
    const visibleNotes = notes.length > 0 ? notes : confirmationPreviewNoteRows(preview);
    rows.push({ id: `confirm-${index}-task`, title: "Task", body: visibleNotes.length > 0 ? `Write ${visibleNotes.length} MIDI notes` : "Write MIDI notes" });
    rows.push(...confirmationTargetRows(payload, index));
    rows.push({ id: `confirm-${index}-unit`, title: "Time unit", body: textValue(payload.time_unit, "beats") });
    visibleNotes.slice(0, 32).forEach((note, noteIndex) => {
      rows.push({
        id: `confirm-${index}-note-${noteIndex}`,
        title: `Note ${noteIndex + 1}`,
        body: confirmationNoteLabel(note)
      });
    });
    return rows;
  }
  if (name.includes("create_midi_clip") || name.includes("insert_midi_clip")) {
    rows.push({ id: `confirm-${index}-task`, title: "Task", body: "Create MIDI clip" });
    rows.push(...confirmationTargetRows(payload, index));
    rows.push({ id: `confirm-${index}-start`, title: "Start", body: textValue(payload.start_seconds ?? payload.start ?? payload.start_beats, "0") });
    rows.push({ id: `confirm-${index}-length`, title: "Length", body: textValue(payload.length_seconds ?? payload.length ?? payload.duration, "4") });
    return rows;
  }
  rows.push({ id: `confirm-${index}-task`, title: "Task", body: "Run project operation" });
  rows.push(...confirmationTargetRows(payload, index));
  return rows;
}

function confirmationCommandPayload(command: JsonRecord): JsonRecord {
  const records: JsonRecord[] = [];
  const visit = (value: unknown, depth = 0) => {
    const record = asRecord(value);
    if (Object.keys(record).length === 0 || depth > 4) {
      return;
    }
    records.push(record);
    visit(record.command, depth + 1);
    visit(record.payload, depth + 1);
    visit(record.value, depth + 1);
    visit(record.args, depth + 1);
    visit(record.arguments, depth + 1);
    visit(record.params, depth + 1);
    visit(record.parameters, depth + 1);
  };
  visit(command);
  return mergeJsonRecords(...records);
}

function confirmationCommandName(command: JsonRecord, payload: JsonRecord): string {
  return textValue(command.name ?? command.command_name ?? payload.name ?? payload.command_name ?? command.tool ?? command.cmd ?? payload.tool ?? payload.cmd ?? payload.command ?? payload.action, "").toLowerCase();
}

export function mergeJsonRecords(...records: JsonRecord[]): JsonRecord {
  const out: JsonRecord = {};
  records.forEach((record) => {
    Object.entries(record).forEach(([key, value]) => {
      if (value !== undefined && value !== null) {
        out[key] = value;
      }
    });
  });
  return out;
}

function confirmationTargetRows(payload: JsonRecord, index: number): JsonRecord[] {
  const rows: JsonRecord[] = [];
  const trackID = textValue(payload.track_id ?? payload.target_track_id, "");
  const clipID = textValue(payload.clip_id ?? payload.target_clip_id, "");
  if (trackID) {
    rows.push({ id: `confirm-${index}-track`, title: "目标轨道", body: trackID });
  }
  if (clipID) {
    rows.push({ id: `confirm-${index}-clip`, title: "目标片段", body: clipID });
  }
  return rows;
}

function confirmationNoteRows(payload: JsonRecord): JsonRecord[] {
  const patch = asRecord(payload.patch ?? payload.note_patch ?? payload.midi_patch);
  const operations = firstArray(payload.operations, payload.note_operations, payload.midi_operations, patch.operations).map(asRecord);
  const rows: JsonRecord[] = [];
  operations.forEach((operation) => {
    const op = textValue(operation.op ?? operation.action ?? operation.type ?? operation.kind, "").toLowerCase();
    const nestedNotes = firstArray(operation.notes, operation.midi_notes, operation.note_rows, operation.note_events, operation.events, operation.insert_notes)
      .map(asRecord)
      .filter(confirmationLooksLikeNote);
    if (nestedNotes.length > 0) {
      nestedNotes.forEach((note) => rows.push({ ...note, operation: op || textValue(operation.operation, "") }));
      return;
    }
    if (op === "insert_note" || op === "add_note" || op === "create_note" || op === "note" || (!op && confirmationLooksLikeNote(operation))) {
      rows.push(operation);
    }
  });
  if (rows.length > 0) {
    return rows;
  }
  return firstArray(payload.notes, payload.midi_notes, payload.note_rows, payload.note_events, payload.events, patch.notes)
    .map(asRecord)
    .filter(confirmationLooksLikeNote);
}

function confirmationLooksLikeNote(note: JsonRecord): boolean {
  return textValue(note.pitch ?? note.note ?? note.name ?? note.pitch_name ?? note.midi_note, "") !== "";
}

function confirmationPreviewNoteRows(preview: string): JsonRecord[] {
  const rows: JsonRecord[] = [];
  const linePattern = /^\s*\d+\.\s*(?:insert_note|add_note|create_note|note)\b([^\n]*)$/gim;
  let lineMatch: RegExpExecArray | null;
  while ((lineMatch = linePattern.exec(preview)) !== null) {
    const row: JsonRecord = {};
    const fieldPattern = /\b(pitch|note|start|length|duration|velocity)=([^\s,;]+)/gi;
    let fieldMatch: RegExpExecArray | null;
    while ((fieldMatch = fieldPattern.exec(lineMatch[1])) !== null) {
      row[fieldMatch[1].toLowerCase()] = fieldMatch[2];
    }
    if (confirmationLooksLikeNote(row)) {
      rows.push(row);
    }
  }
  return rows;
}

function confirmationNoteLabel(note: JsonRecord): string {
  const pitch = textValue(note.pitch ?? note.note ?? note.name ?? note.pitch_name, "-");
  const pitchName = midiPitchName(Number(pitch));
  const start = textValue(note.start ?? note.start_beat ?? note.start_beats ?? note.start_time_beats ?? note.position ?? note.beat, "0");
  const length = textValue(note.length ?? note.duration ?? note.length_beats ?? note.duration_beats ?? note.duration_beats_float, "1");
  const velocity = textValue(note.velocity, "100");
  return `Pitch ${pitch}${pitchName ? ` (${pitchName})` : ""}, start ${start}, length ${length}, velocity ${velocity}`;
}

function midiPitchName(pitch: number): string {
  if (!Number.isFinite(pitch)) {
    return "";
  }
  const names = ["C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"];
  const rounded = Math.round(pitch);
  const name = names[((rounded % 12) + 12) % 12];
  const octave = Math.floor(rounded / 12) - 1;
  return `${name}${octave}`;
}

function summarizeConfirmationCommands(commands: JsonRecord[], preview: string): string {
  const names = commands
    .map((command) => textValue(command.name ?? command.command_name ?? command.cmd ?? command.tool, "").toLowerCase())
    .filter(Boolean);
  const joined = `${names.join(" ")} ${preview}`.toLowerCase();
  if (joined.includes("apply_note_patch") || joined.includes("apply_midi_note_patch") || joined.includes("add_midi_notes") || joined.includes("insert_note")) {
    const noteCount = confirmationNoteCount(commands, preview);
    return noteCount > 0 ? `Ready to write ${noteCount} MIDI notes` : "Ready to write MIDI notes";
  }
  if (joined.includes("create_midi_clip") || joined.includes("midi clip")) {
    return "Ready to create MIDI clip";
  }
  if (joined.includes("clip.gain.set") || joined.includes("clip_gain_set")) {
    return "Ready to modify clip gain";
  }
  if (joined.includes("clip.fade.set") || joined.includes("clip_fade_set")) {
    return "Ready to modify clip fade";
  }
  if (joined.includes("create_track") || joined.includes("track")) {
    return "Ready to modify track structure";
  }
  if (commands.length > 1) {
    return `Ready to run ${commands.length} project operations after confirmation`;
  }
  if (commands.length === 1) {
    return "Ready to run 1 project operation after confirmation";
  }
  return "";
}

function confirmationNoteCount(commands: JsonRecord[], preview: string): number {
  let count = 0;
  commands.forEach((command) => {
    count += confirmationNoteRows(confirmationCommandPayload(command)).length;
  });
  if (count > 0) {
    return count;
  }
  count = confirmationPreviewNoteRows(preview).length;
  if (count > 0) {
    return count;
  }
  const match = preview.match(/(\d+)\s*(?:个)?\s*(?:notes?|音符)/i);
  return match ? Number(match[1]) : 0;
}

export function looksLikeInternalInstruction(value: string): boolean {
  const text = value.trim();
  if (!text) {
    return false;
  }
  if (text.includes("{") && text.includes("}")) {
    return true;
  }
  return /\b(NEEDS_CONFIRMATION|tool|tool_call|command|cmd|workflow|plan_id|midi\.apply_note_patch|apply_midi_note_patch|add_midi_notes|create_midi_clip)\b/i.test(text);
}

export function ActionDetails({
  reviewItems,
  fields,
  command,
	fieldValues,
  onFieldChange,
  onSelectArtifact
}: {
  reviewItems: JsonRecord[];
  fields: JsonRecord[];
  command: JsonRecord;
	fieldValues: JsonRecord;
  onFieldChange: (fieldID: string, value: string) => void;
  onSelectArtifact: (id: string) => void;
}) {
  const hasCommand = Object.keys(command).length > 0;
	if (reviewItems.length === 0 && fields.length === 0 && !hasCommand) {
    return null;
  }
  return (
    <div className="action-detail-grid">
      {reviewItems.map((item, index) => (
        <div className="action-detail" key={textValue(item.id, `review-${index}`)}>
          <span>{localizeDisplayText(textValue(item.title ?? item.status, `项目 ${index + 1}`))}</span>
          <strong>{localizeDisplayText(textValue(item.body ?? item.value, "-")) || "-"}</strong>
        </div>
      ))}
      {fields.length > 0 && (
        <div className="interaction-fields">
          {fields.slice(0, 32).map((field, index) => {
            const fieldID = textValue(field.id, `field-${index}`);
            const label = textValue(field.label ?? field.kind, `Field ${index + 1}`);
            const description = textValue(field.description, "");
            return (
              <label className="interaction-field" key={fieldID}>
                <span>{label}</span>
                <InteractionFieldControl field={field} value={textValue(fieldValues[fieldID], "")} onChange={(value) => onFieldChange(fieldID, value)} />
                {description && <small>{description}</small>}
              </label>
            );
          })}
        </div>
      )}
      {hasCommand && (
        <div className="action-detail command">
          <span>操作</span>
          <strong>{commandSummaryLabel(command)}</strong>
        </div>
      )}
    </div>
  );
}

function InteractionFieldControl({ field, value, onChange }: { field: JsonRecord; value: string; onChange: (value: string) => void }) {
  const kind = textValue(field.kind ?? field.type, "text").toLowerCase();
  const placeholder = textValue(field.placeholder, "");
  const options = firstArray(field.options).map(asRecord).filter((option) => Object.keys(option).length > 0);
  if (kind === "choice" || kind === "select") {
    return (
      <div className="choice-list">
        {options.length === 0 && <span className="choice-empty">{placeholder || "-"}</span>}
        {options.map((option, index) => {
          const optionValue = textValue(option.value ?? option.id ?? option.key ?? option.label, `option_${index + 1}`);
          const label = textValue(option.label ?? option.title ?? option.name ?? optionValue, optionValue);
          const description = textValue(option.description ?? option.hint, "");
          const selected = value === optionValue;
          return (
            <button
              key={`${optionValue}-${index}`}
              className={`choice-option ${selected ? "selected" : ""}`}
              type="button"
              onClick={() => onChange(optionValue)}
            >
              <span className="choice-index">{index + 1}.</span>
              <span className="choice-main">
                <strong>{label}</strong>
                {description && <small>{description}</small>}
              </span>
            </button>
          );
        })}
      </div>
    );
  }
  if (kind === "textarea" || kind === "long_text" || kind === "multiline") {
    return <textarea value={value} placeholder={placeholder} rows={3} onChange={(event) => onChange(event.currentTarget.value)} />;
  }
  return <input value={value} placeholder={placeholder} onChange={(event) => onChange(event.currentTarget.value)} />;
}





export function interactionFields(action: JsonRecord): JsonRecord[] {
  return [...firstArray(action.questions), ...firstArray(action.fields)]
    .map(asRecord)
    .filter((field) => Object.keys(field).length > 0);
}

export function interactionReviewItems(action: JsonRecord): JsonRecord[] {
  return firstArray(action.review_items).map(asRecord).filter((item) => Object.keys(item).length > 0);
}

export function initialFieldValues(fields: JsonRecord[]): JsonRecord {
  const out: JsonRecord = {};
  fields.forEach((field, index) => {
    const fieldID = textValue(field.id, `field-${index}`);
    out[fieldID] = textValue(field.value, "");
  });
  return out;
}

export function hasMissingRequiredFields(fields: JsonRecord[], values: JsonRecord): boolean {
  return fields.some((field, index) => {
    if (!truthy(field.required)) {
      return false;
    }
    const fieldID = textValue(field.id, `field-${index}`);
    return textValue(values[fieldID], "") === "";
  });
}

export function isCancelAction(actionID: string): boolean {
  return ["cancel", "done", "close"].includes(actionID.trim().toLowerCase());
}

export function interactionPayloadForAction(interaction: JsonRecord, action: JsonRecord, fieldValues: JsonRecord): JsonRecord {
  const payload: JsonRecord = { ...asRecord(action.value ?? action.payload) };
  const fields = interactionFields(interaction);
  if (fields.length === 0) {
    return payload;
  }
  const regularFields: JsonRecord = {};
  const displayDomainReviews: JsonRecord[] = [];
  fields.forEach((field, index) => {
    const fieldID = textValue(field.id, `field-${index}`);
    const value = textValue(fieldValues[fieldID], "");
    if (!value) {
      return;
    }
    if (truthy(field._plugin_display_domain_review)) {
      displayDomainReviews.push({
        ...asRecord(field.payload),
        display_domain_text: value,
        provenance_kind: "user_review",
        observation: "Confirmed from Ask Vit WebUI review card."
      });
      return;
    }
    regularFields[fieldID] = value;
  });
  if (Object.keys(regularFields).length > 0) {
    payload.fields = regularFields;
  }
  if (displayDomainReviews.length > 0) {
    payload.display_domain_reviews = displayDomainReviews;
  }
  return payload;
}




export function interactionPayload(action: JsonRecord): JsonRecord {
  return asRecord(action.payload ?? action.data);
}
export function confirmationFallbackActionsFromResponse(response: ChatResponse): JsonRecord[] {
  const planID = textValue(response.plan_id, "");
  if (!planID || !responseNeedsConfirmationFallback(response)) {
    return [];
  }
  const body = textValue(response.reply ?? response.message, "这个操作需要你确认后才会执行。");
  const preview = textValue(response.preview, "");
  const workflowData = asRecord(response.workflow_data);
  const proposalPresentation = asRecord(response.proposal_presentation ?? workflowData.proposal_presentation);
  const isCapabilityProposal = textValue(response.workflow, "").toLowerCase() === "capability_runtime_v1" && Object.keys(proposalPresentation).length > 0;
  const payload: JsonRecord = {
    plan_id: planID,
    preview,
    commands: response.commands ?? []
  };
  if (isCapabilityProposal) {
    ["session_id", "capability_id", "proposal_id", "proposal_revision", "action_set_hash", "project_cut_hash", "approval_mode"].forEach((key) => {
      if (workflowData[key] !== undefined) {
        payload[key] = workflowData[key];
      }
    });
    payload.proposal_presentation = proposalPresentation;
  }
  return [
    {
      _ui_source: "interaction",
      _synthetic_confirmation: true,
      id: `confirmation_${planID}`,
      kind: isCapabilityProposal ? "proposal_approval" : "confirmation",
      type: isCapabilityProposal ? "proposal_approval" : "confirmation",
      source: "vit_agent",
      title: isCapabilityProposal ? textValue(proposalPresentation.title, "方案等待确认") : "需要确认",
      body,
      status: "waiting_for_user",
      plan_id: planID,
      workflow: response.workflow,
      payload,
      data: payload,
      command: {
        preview,
        commands: response.commands ?? []
      },
      actions: [
        { id: "approve", label: "确认执行", style: "primary", recommended: true },
        { id: "cancel", label: "取消", style: "secondary" }
      ]
    }
  ];
}

export function responseNeedsConfirmationFallback(response: ChatResponse): boolean {
  const status = textValue(response.goal_status ?? response.status, "").toLowerCase();
  return Boolean(response.needs_confirmation) || status.includes("waiting_confirmation") || status.includes("needs_confirmation") || status.includes("confirm");
}

// FIX-CONFIRM-CARD-1 ③（2026-09-29 决策侧裁定）：full access 的产品语义=用户
// 授予完全访问——RiskConfirm 级确认闸（通用 confirmation / mix 单步
// mix_tick_confirmation / mix_treatment_confirmation）不再出前置确认卡，webui
// 渲染前拦截并自动直执，保留事后回执（执行摘要入对话流）。capability
// proposal（proposal_approval，GUI-T4 先例：完全档静默沉淀）与选卡/表单类
// 交互不在直执范围；manual_confirmation 行为不变。
// 注意：此文案不得含英文 "confirm"（小写 ≤120 字符会被
// isDisposableConfirmationPromptText 判为一次性确认提示而在 history sync
// 时被 removeDisposableConfirmationPromptMessages 删除——E2E K2 实证）。
const FULL_ACCESS_DIRECT_EXECUTION_NOTICE = "完全访问已开启 · 已直接执行该动作，执行回执见下。";

export function isFullAccessDirectExecutionInteraction(action: JsonRecord): boolean {
  if (isCapabilityProposalInteraction(action)) {
    return false;
  }
  const kind = textValue(action.kind, "").toLowerCase();
  const type = textValue(action.type, "").toLowerCase();
  const kindOrType = [kind, type];
  const isConfirmationGate =
    kindOrType.includes("confirmation") ||
    kindOrType.includes("mix_tick_confirmation") ||
    kindOrType.includes("mix_treatment_confirmation") ||
    type === "approval.requested";
  if (!isConfirmationGate) {
    return false;
  }
  // 表单/问答类交互（需用户输入）永不自动执行；直执仅覆盖二元批准闸。
  if (firstArray(action.questions).length > 0 || firstArray(action.fields).length > 0) {
    return false;
  }
  const childActions = firstArray(action.actions).map(asRecord).filter((item) => Object.keys(item).length > 0);
  if (childActions.length === 0) {
    return false;
  }
  const childIDs = childActions.map((child) => textValue(child.id ?? child.action_id ?? child.decision, "").toLowerCase());
  const hasApprove = childIDs.some((id) => ["approve", "confirm", "allow", "yes", "ok", "apply"].includes(id));
  const hasAbort = childIDs.some((id) => ["cancel", "reject", "deny", "dismiss", "no"].includes(id));
  return hasApprove && hasAbort;
}

// 一次 chat 响应中可被完全档直执的确认闸集合：优先取服务端 interaction_
// requests；无 requests 而带 needs_confirmation 时退到合成确认卡（plan 确认
// 路径），capability proposal 形态的合成卡（proposal_approval）不进直执。
export function fullAccessDirectExecutionTargets(response: ChatResponse): JsonRecord[] {
  const fromRequests = firstArray(response.interaction_requests)
    .map(asRecord)
    .filter((action) => Object.keys(action).length > 0 && isFullAccessDirectExecutionInteraction(action));
  if (fromRequests.length > 0) {
    return fromRequests;
  }
  return confirmationFallbackActionsFromResponse(response)
    .filter((action) => isFullAccessDirectExecutionInteraction(action) && !isCapabilityProposalInteraction(action));
}

// 直执目标的呈现面治理：从入流消息上摘除前置确认卡（不渲染可交互形态）；
// 消息若只剩一次性确认提示文本，则改写为直执通告行（回执随后入流）。
export function stripFullAccessDirectExecutionActions(message: ChatMessage, targets: JsonRecord[]): ChatMessage {
  if (targets.length === 0) {
    return message;
  }
  const removeIDs = new Set(targets.map((action) => actionRenderID(action)));
  const actions = (message.actions ?? []).filter((value) => {
    const action = asRecord(value);
    return !removeIDs.has(actionRenderID(action));
  });
  let content = message.content;
  if (actions.length === 0 && isDisposableConfirmationPromptText(content)) {
    content = FULL_ACCESS_DIRECT_EXECUTION_NOTICE;
  }
  return { ...message, actions, content };
}

export function isSyntheticConfirmationInteraction(interaction: JsonRecord): boolean {
  return truthy(interaction._synthetic_confirmation);
}

function isCurrentPluginTargetText(value: string): boolean {
  const normalized = value.trim().toLowerCase();
  return (
    ["当前", "当前插件", "选中插件", "使用当前插件", "current", "current plugin", "selected plugin"].includes(normalized) ||
    normalized.includes("当前") ||
    normalized.includes("选中") ||
    normalized.includes("已选中") ||
    normalized.includes("selected") ||
    normalized.includes("current")
  );
}
export function mixBoardActionFromPayload(...values: unknown[]): JsonRecord | null {
  for (const value of values) {
    const record = asRecord(value);
    const workflowData = asRecord(record.workflow_data);
    const result = asRecord(record.result);
    const interactionRequests = firstArray(record.interaction_requests, workflowData.interaction_requests, result.interaction_requests).map(asRecord);
    const hasMixBoardInteraction = interactionRequests.some(isMixBoardAction);
    const mixSession = firstNonEmptyRecord(asRecord(record.mix_session), asRecord(workflowData.mix_session), asRecord(result.mix_session));
    const mixObservation = firstNonEmptyRecord(asRecord(record.mix_observation), asRecord(workflowData.mix_observation), asRecord(result.mix_observation));
    if (Object.keys(mixObservation).length > 0 || hasMixBoardInteraction) {
      const sessionID = textValue(mixSession.mix_session_id ?? asRecord(mixObservation.observation).mix_session_id ?? mixObservation.mix_session_id, "");
      return {
        _ui_source: "mix_board",
        kind: "mix_board",
        type: "mix_board",
        id: `mix_board_${stableIDPart(sessionID || textValue(asRecord(mixObservation.observation).observation_id ?? mixObservation.observation_id, "latest"))}`,
        status: textValue(mixObservation.status ?? asRecord(mixObservation.observation).status ?? mixSession.state, ""),
        mix_session: mixSession,
        mix_observation: mixObservation,
        payload: {
          mix_session: mixSession,
          mix_observation: mixObservation
        }
      };
    }
  }
  return null;
}

export function mixBoardActionWithInteractions(mixBoardAction: JsonRecord | null, interactions: JsonRecord[]): JsonRecord | null {
  if (!mixBoardAction) {
    return null;
  }
  const match = interactions.find((interaction) => isSameMixBoardSession(interaction, mixBoardAction, mixBoardActionSessionID(mixBoardAction))) ??
    interactions.find(isMixBoardAction);
  if (!match) {
    return mixBoardAction;
  }
  const payload = interactionPayload(match);
  const mixPayload = interactionPayload(mixBoardAction);
  return {
    ...match,
    ...mixBoardAction,
    id: textValue(match.id ?? match.interaction_id, "") || textValue(mixBoardAction.id, ""),
    interaction_id: textValue(match.id ?? match.interaction_id, ""),
    _ui_source: "mix_board",
    kind: "mix_board",
    type: "mix_board",
    title: textValue(match.title ?? mixBoardAction.title, "MixBoard"),
    body: textValue(match.body ?? mixBoardAction.body, ""),
    actions: firstArray(match.actions),
    payload: {
      ...payload,
      ...mixPayload,
      request_context: asRecord(payload.request_context ?? mixPayload.request_context)
    },
    data: {
      ...asRecord(match.data),
      ...asRecord(mixBoardAction.data),
      ...mixPayload
    }
  };
}

function firstNonEmptyRecord(...records: JsonRecord[]): JsonRecord {
  return records.find((record) => Object.keys(record).length > 0) ?? {};
}


export function interactionActionsFrom(...values: unknown[]): JsonRecord[] {
  const seen = new Set<string>();
  const out: JsonRecord[] = [];
  values.forEach((value) => {
    firstArray(value).forEach((raw, index) => {
      const record = asRecord(raw);
      if (Object.keys(record).length === 0) {
        return;
      }
      const id = textValue(record.id ?? record.interaction_id, "");
      const key = id || `${textValue(record.type ?? record.kind ?? record.title, "interaction")}-${index}`;
      if (seen.has(key)) {
        return;
      }
      seen.add(key);
      out.push({ ...record, _ui_source: "interaction" });
    });
  });
  return out;
}
