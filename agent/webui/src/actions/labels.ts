// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2a）：App.tsx 动作卡体系全域出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
// 动作卡共享标签/本地化纯函数域（无 JSX）。

import type { AgentEvent, JsonRecord } from "../types";
import { asRecord, firstArray, textValue } from "../lib/util";
import { isConsumedInteractionStatus } from "../interactionGuard";
import { agentEventAction } from "../App";
import { confirmationActionSummary, isConfirmationAction, looksLikeInternalInstruction } from "./confirmation";

// 导出供 composer/PlanBar 复用（GUI-T6）：goal/plan 英文机器状态 → 中文标签
export function statusLabel(value: unknown, fallback = ""): string {
  const status = textValue(value, "").toLowerCase();
  if (!status) {
    return fallback;
  }
  if (status === "none") {
    return "无";
  }
  if (["idle", "empty"].includes(status)) {
    return "空闲";
  }
  if (status === "ready") {
    return "就绪";
  }
  if (status === "offline") {
    return "未连接";
  }
  if (["play", "playing"].includes(status)) {
    return "播放中";
  }
  if (["stop", "stopped"].includes(status)) {
    return "停止";
  }
  if (["pause", "paused"].includes(status)) {
    return "暂停";
  }
  if (["running", "in_progress", "processing", "busy"].includes(status)) {
    return "执行中";
  }
  if (["pending", "queued"].includes(status)) {
    return "待处理";
  }
  if (["completed", "complete", "done", "ok", "success", "succeeded", "applied"].includes(status)) {
    return "已完成";
  }
  // F3：已消费/失效（server 4022 口径）——已决只读形态的可辨识标签
  if (isConsumedInteractionStatus(status)) {
    return "已处理";
  }
  if (status.includes("waiting") || status.includes("confirm") || status === "needs_confirmation" || status === "requires_confirmation") {
    return "待确认";
  }
  if (["cancel", "cancelled", "canceled"].includes(status)) {
    return "已取消";
  }
  if (["failed", "failure", "error", "kernel_error"].includes(status)) {
    return "失败";
  }
  return fallback;
}

function riskLabel(value: unknown): string {
  const risk = textValue(value, "").toLowerCase();
  if (!risk) {
    return "";
  }
  if (risk.includes("confirm") || risk.includes("approval")) {
    return "需确认";
  }
  if (risk.includes("high") || risk.includes("danger")) {
    return "高风险";
  }
  if (risk.includes("medium")) {
    return "中风险";
  }
  if (risk.includes("low") || risk.includes("safe")) {
    return "低风险";
  }
  return statusLabel(risk, "");
}

export function agentEventDisplayTitle(event: AgentEvent): string {
  const action = agentEventAction(event);
  const title = actionOperationTitle(action);
  if (title) {
    return title;
  }
  return localizeDisplayText(textValue(event.title, ""));
}

function actionOperationTitle(action: JsonRecord): string {
  const label = actionOperationLabel(action);
  if (!label) {
    return "";
  }
  const objectName = actionObjectName(action);
  return objectName ? `${label}: ${objectName}` : label;
}

function actionOperationLabel(action: JsonRecord): string {
  const names = actionNameCandidates(action);
  for (const name of names) {
    const label = operationLabelFromName(name);
    if (label) {
      return label;
    }
  }
  const kind = textValue(action.kind ?? action.type ?? action.item_type, "").toLowerCase();
  if (kind === "daw_action" || kind === "tool_call") {
    return "工程操作";
  }
  return "";
}

function actionNameCandidates(action: JsonRecord): string[] {
  const payload = asRecord(action.payload);
  const result = asRecord(action.result ?? payload.result);
  const command = asRecord(action.command);
  const args = asRecord(action.args ?? payload.args ?? command.args ?? command.arguments);
  const commandRaw = asRecord(payload.command_raw);
  const records = [action, payload, result, command, args, commandRaw];
  const values: string[] = [];
  records.forEach((record) => {
    [
      record.tool,
      record.command_name,
      record.name,
      record.cmd,
      record.command,
      record.action,
      record.kind,
      record.type
    ].forEach((value) => {
      const text = textValue(value, "");
      if (text) {
        values.push(text);
      }
    });
  });
  firstArray(action.commands, payload.commands, command.commands).map(asRecord).forEach((record) => {
    actionNameCandidates(record).forEach((name) => values.push(name));
  });
  return Array.from(new Set(values));
}

function operationLabelFromName(rawName: string): string {
  const name = rawName.trim().toLowerCase();
  if (!name) {
    return "";
  }
  if (name.includes("track.add") || name.includes("create_track") || name.includes("add_track") || name === "track") {
    return "新建轨道";
  }
  if (name.includes("track.delete") || name.includes("delete_track") || name.includes("remove_track")) {
    return "删除轨道";
  }
  if (name.includes("track.rename") || name.includes("rename_track")) {
    return "重命名轨道";
  }
  if (name.includes("track.select") || name.includes("select_track")) {
    return "选择轨道";
  }
  if (name.includes("midi.create_clip") || name.includes("midi.insert_clip") || name.includes("create_midi_clip") || name.includes("insert_midi_clip")) {
    return "新建 MIDI 片段";
  }
  if (name.includes("midi.apply_note_patch") || name.includes("apply_midi_note_patch") || name.includes("add_midi_notes") || name.includes("mutate_midi_notes")) {
    return "写入 MIDI 音符";
  }
  if (name.includes("midi.read_clip_notes") || name.includes("read_midi") || name.includes("read_clip_notes")) {
    return "读取 MIDI 音符";
  }
  if (name.includes("midi.import") || name.includes("import_midi")) {
    return "导入 MIDI";
  }
  if (name.includes("warm_waveform_bake") || name.includes("mix.request_observation") || name.includes("mix_request_observation")) {
    return "音频观察";
  }
  if (name.includes("mix.apply_tick") || name.includes("mix_apply_tick")) {
    return "混音步骤已执行";
  }
  if (name.includes("mix.propose_tick") || name.includes("mix_propose_tick")) {
    return "混音步骤建议";
  }
  if (name.includes("mix.rollback_tick") || name.includes("mix_rollback_tick")) {
    return "混音步骤回滚";
  }
  if (name.includes("plugin.load") || name.includes("load_plugin") || name.includes("rack_add_node") || name.includes("rack.add") || name.includes("rack_node")) {
    return "加载插件";
  }
  if (name.includes("plugin") && name.includes("learn")) {
    return "插件学习";
  }
  if (name.includes("clip.import") || name.includes("import_media")) {
    return "导入媒体";
  }
  if (name.includes("artifact") || name.includes("media")) {
    return "资料操作";
  }
  if (name.includes("browser")) {
    return "浏览器操作";
  }
  if (name.includes("macro")) {
    return "宏操作";
  }
  return "";
}

function actionObjectName(action: JsonRecord): string {
  const payload = asRecord(action.payload);
  const result = asRecord(action.result ?? payload.result);
  const command = asRecord(action.command);
  const args = asRecord(action.args ?? payload.args ?? command.args ?? command.arguments);
  const records = [action, payload, result, args, command];
  const specificNameKeys = ["track_name", "clip_name", "plugin_name", "display_name"];
  for (const record of records) {
    for (const key of specificNameKeys) {
      const value = textValue(record[key], "");
      if (isSafeObjectDisplayName(value)) {
        return value;
      }
    }
  }
  const nestedRecords = [
    asRecord(result.track),
    asRecord(result.clip),
    asRecord(result.plugin),
    asRecord(payload.track),
    asRecord(payload.clip),
    asRecord(payload.plugin),
    asRecord(args.track),
    asRecord(args.clip),
    asRecord(args.plugin)
  ];
  const nestedNameKeys = ["track_name", "clip_name", "plugin_name", "display_name", "title", "name", "label"];
  for (const record of nestedRecords) {
    for (const key of nestedNameKeys) {
      const value = textValue(record[key], "");
      if (isSafeObjectDisplayName(value)) {
        return value;
      }
    }
  }
  for (const record of records) {
    for (const key of ["name", "label"]) {
      const value = textValue(record[key], "");
      if (isSafeObjectDisplayName(value)) {
        return value;
      }
    }
  }
  const idKeys = ["new_track_id", "created_track_id", "track_id", "new_clip_id", "created_clip_id", "clip_id", "plugin_id"];
  for (const record of records) {
    for (const key of idKeys) {
      const value = textValue(record[key], "");
      if (isSafeObjectDisplayName(value)) {
        return value;
      }
    }
  }
  return "";
}

function isSafeObjectDisplayName(value: string): boolean {
  const text = value.trim();
  if (!text) {
    return false;
  }
  if (statusLabel(text, "") || operationLabelFromName(text) || looksLikeInternalInstruction(text)) {
    return false;
  }
  if (/^[a-z]+[._][a-z0-9_.-]+$/i.test(text) || /^[a-z0-9]+_[a-z0-9_]+$/i.test(text)) {
    return false;
  }
  if (/^(已完成|正在执行)\s+/i.test(text)) {
    return false;
  }
  return true;
}

export function commandSummaryLabel(command: JsonRecord): string {
  return actionOperationTitle(command) || "工程操作";
}

// 导出供 composer/PlanBar 复用（GUI-T6）：规划条 current_step 本地化
export function localizeDisplayText(value: string): string {
  const text = value.trim();
  if (!text) {
    return "";
  }
  const status = statusLabel(text, "");
  if (status) {
    return status;
  }
  const directOperation = operationLabelFromName(text);
  if (directOperation) {
    return directOperation;
  }
  const completedMatch = text.match(/^已完成\s+(.+)$/i);
  if (completedMatch) {
    const label = operationLabelFromName(completedMatch[1]);
    return label ? `已完成：${label}` : "工程操作已完成";
  }
  const runningMatch = text.match(/^正在执行\s+(.+)$/i);
  if (runningMatch) {
    const label = operationLabelFromName(runningMatch[1]);
    return label ? `正在执行：${label}` : "正在执行工程操作";
  }
  if (/^track added$/i.test(text)) {
    return "轨道已创建";
  }
  if (/^clip added$/i.test(text) || /^midi clip added$/i.test(text)) {
    return "MIDI 片段已创建";
  }
  const insertedMatch = text.match(/^updated midi clip:\s*inserted\s*(\d+)/i);
  if (insertedMatch) {
    return `MIDI 片段已更新：写入 ${insertedMatch[1]} 个音符。`;
  }
  if (looksLikeInternalInstruction(text)) {
    return "";
  }
  return text;
}

export function localizeMixTreatmentCardText(value: string): string {
  const raw = value.trim();
  if (looksLikeEnglishMixTreatmentReason(raw)) {
    return "已根据当前频段能量观察生成一个保守的低频/低中频 EQ 处理候选；确认前不会写入任何插件参数。";
  }
  let text = localizeDisplayText(raw);
  if (!text) {
    return "";
  }
  const exact = text.toLowerCase();
  const exactMap: Record<string, string> = {
    pending: "待确认",
    pending_confirmation: "待确认",
    waiting_for_user: "等待确认",
    request_more_observation: "需要更多观察",
    low: "低",
    medium: "中",
    high: "高",
    plugin_treatment: "插件处理",
    gain_balance: "电平平衡",
    pan_balance: "声像调整",
    eq: "EQ",
    utility: "工具"
  };
  if (exactMap[exact]) {
    return exactMap[exact];
  }
  text = text
    .replace(/\brequest_more_observation\b/gi, "需要更多观察")
    .replace(/\bpending_confirmation\b/gi, "待确认")
    .replace(/\bwaiting_for_user\b/gi, "等待确认")
    .replace(/\bconfidence\b/gi, "置信度")
    .replace(/\bstrategy\b/gi, "策略")
    .replace(/\blow[-_\s]?mid\b/gi, "低中频")
    .replace(/\bsub\b/gi, "超低频")
    .replace(/\bbass\b/gi, "低频")
    .replace(/\bmid\b/gi, "中频")
    .replace(/\bpresence\b/gi, "存在感频段")
    .replace(/\bair\b/gi, "空气感频段")
    .replace(/\bpeak\s*\/\s*rms\b/gi, "峰值/RMS")
    .replace(/\blow\b/gi, "低")
    .replace(/\bmedium\b/gi, "中")
    .replace(/\bhigh\b/gi, "高");
  return text;
}

function looksLikeEnglishMixTreatmentReason(text: string): boolean {
  const compact = text.trim();
  if (!compact) {
    return false;
  }
  const lower = compact.toLowerCase();
  if (!/(sub|bass|low[-\s]?mid|band energ|presence|peak\/rms|plugin|parameter)/.test(lower)) {
    return false;
  }
  const asciiLetters = (compact.match(/[a-z]/gi) ?? []).length;
  const cjk = (compact.match(/[\u4e00-\u9fff]/g) ?? []).length;
  return asciiLetters >= 20 && cjk === 0;
}

export function isCompactActionCard(
  action: JsonRecord,
  tone: "info" | "attention" | "success" | "error",
  isInteractive: boolean,
  isCompletion: boolean
): boolean {
  if (isInteractive || isCompletion || tone !== "success") {
    return false;
  }
  const source = textValue(action._ui_source, "");
  if (source !== "event" && source !== "executed") {
    return false;
  }
  const status = statusLabel(action.status ?? action.stage ?? asRecord(action.payload).status, "");
  return status === "已完成";
}

export function actionTitle(action: JsonRecord, index: number): string {
  if (isConfirmationAction(action)) {
    return "需要确认";
  }
  const operationTitle = actionOperationTitle(action);
  if (operationTitle) {
    return operationTitle;
  }
  const command = asRecord(action.command);
  const rawTitle = textValue(
    action.title ?? action.command_name ?? action.name ?? action.tool ?? command.tool ?? command.cmd ?? action.kind,
    ""
  );
  return localizeDisplayText(rawTitle) || `Project action ${index + 1}`;
}

export function actionBody(action: JsonRecord): string {
  if (isConfirmationAction(action)) {
    return confirmationActionSummary(action);
  }
  const payload = asRecord(action.payload);
  const rawBody = textValue(action.body ?? action.preview ?? payload.preview ?? action.message ?? action.reason ?? action.description, "");
  const body = localizeDisplayText(rawBody);
  if (!body || looksLikeInternalInstruction(body)) {
    return "";
  }
  const objectName = actionObjectName(action);
  if (objectName && body === objectName) {
    return "";
  }
  return body;
}

export function actionBadge(action: JsonRecord, source: string, status: string): string {
  if (isConfirmationAction(action)) {
    const resolvedStatus = textValue(action.status ?? action.stage, "").toLowerCase();
    // F3 死卡守卫：已消费卡的已决只读形态要可辨识（对齐 server 4022 口径）
    if (isConsumedInteractionStatus(resolvedStatus)) {
      return "已处理";
    }
    if (resolvedStatus.includes("complete") || resolvedStatus.includes("done")) {
      return "已确认";
    }
    if (resolvedStatus.includes("cancel")) {
      return "已取消";
    }
    if (resolvedStatus.includes("fail") || resolvedStatus.includes("error")) {
      return "失败";
    }
    return "待确认";
  }
  if (source === "interaction") {
    const label = statusLabel(action.status ?? action.stage ?? status, "");
    return label || "Interaction";
  }
  if (source === "command") {
    return riskLabel(action.risk ?? action.risk_level ?? status) || "待确认";
  }
  if (source === "executed") {
    return statusLabel(action.status ?? status, "Executed");
  }
  if (source === "event") {
    return statusLabel(status, "Status");
  }
  return statusLabel(status, "");
}

export function actionTone(action: JsonRecord, isInteractive: boolean): "info" | "attention" | "success" | "error" {
  const source = textValue(action._ui_source, "");
  const status = textValue(action.status ?? action.goal_status ?? "", "").toLowerCase();
  const risk = textValue(action.risk ?? action.risk_level ?? "", "").toLowerCase();
  const stopReason = textValue(action.stop_reason ?? "", "").toLowerCase();
  const error = textValue(action.error ?? action.failure_reason, "");
  if (error || status.includes("error") || status.includes("fail") || stopReason === "failed") {
    return "error";
  }
  if (status === "completed" || status === "complete" || status === "done") {
    return "success";
  }
  if (isInteractive || risk === "confirm" || status.includes("waiting") || status.includes("confirm")) {
    return "attention";
  }
  if (source === "executed" || status === "ok" || status === "done" || status === "success" || status === "applied") {
    return "success";
  }
  return "info";
}
