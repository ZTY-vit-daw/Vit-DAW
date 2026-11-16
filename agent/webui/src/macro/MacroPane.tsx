// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2e）：App.tsx Macro 族组件（N 域）出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
import { Activity, Brain, Link2, RefreshCw, Send, SlidersHorizontal, Upload, Wand2, X } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { AgentMode, AgentUIState, JsonRecord, MacroControl } from "../types";
import { asRecord, numericValue, textValue } from "../lib/util";
import {
  macroBindingSummary,
  macroControlDisabledReason,
  macroControlID,
  macroControlLabel,
  macroControlMeta,
  macroControlValue,
  macroControlsFromUIState,
  macroPaneSubtitle,
  macroValueText,
  normalizeMacroControl,
  activeWorkflowModeLabel
} from "./normalize";
import { EmptyState } from "../App";

export function MacroPane({
	uiState,
	onUploadClick,
	onModeChange,
	onAttachMacroControl,
  onInsertMacroControlCard,
  onMacroValuePreview,
  onMacroValueCommit,
  onMacroRename,
  onRefresh
}: {
  uiState: AgentUIState | null;
  onUploadClick: () => void;
  onModeChange: (mode: AgentMode) => void;
	onAttachMacroControl: (macro: MacroControl) => void;
  onInsertMacroControlCard: (macro: MacroControl) => void;
  onMacroValuePreview: (macro: MacroControl, value: number) => void;
  onMacroValueCommit: (macro: MacroControl, value: number) => Promise<void>;
  onMacroRename: (macro: MacroControl, name: string) => Promise<void>;
  onRefresh: () => Promise<void>;
}) {
  const macros = macroControlsFromUIState(uiState);
  const summary = asRecord(asRecord(uiState?.ui_context).macro_control_summary);
  const selectedPlugin = asRecord(uiState?.selected_plugin);
  const activeMode = activeWorkflowModeLabel(uiState);
  return (
    <div className="pane-body">
      <section className="work-section">
        <div className="macro-pane-header">
          <div>
            <h2>宏控制</h2>
            <span>{macroPaneSubtitle(macros.length, summary, selectedPlugin)}</span>
          </div>
          <button className="icon-button refresh" type="button" title="刷新宏控制" onClick={() => void onRefresh()}>
            <RefreshCw size={15} />
          </button>
        </div>
		<MacroModeStrip activeMode={activeMode} onUploadClick={onUploadClick} onModeChange={onModeChange} />
        {macros.length === 0 && <EmptyState label="暂无宏控制" />}
        <div className="macro-list">
          {macros.map((macro) => (
            <MacroControlPanelCard
              key={macroControlID(macro)}
              macro={macro}
              onAttach={() => onAttachMacroControl(macro)}
              onInsertCard={() => onInsertMacroControlCard(macro)}
              onPreviewValue={(value) => onMacroValuePreview(macro, value)}
              onCommitValue={(value) => onMacroValueCommit(macro, value)}
              onRename={(name) => onMacroRename(macro, name)}
            />
          ))}
        </div>
      </section>
    </div>
  );
}

export function MacroModeStrip({
	activeMode,
	onUploadClick,
	onModeChange
}: {
	activeMode: string;
	onUploadClick: () => void;
	onModeChange: (mode: AgentMode) => void;
}) {
  const items: Array<{ key: string; label: string; enabled: boolean; title: string; Icon: LucideIcon; action: () => void }> = [
    { key: "add_file", label: "添加文件", enabled: true, title: "从输入区添加文件资料", Icon: Upload, action: onUploadClick },
    { key: "plan", label: "计划模式", enabled: true, title: "切换到计划模式", Icon: Brain, action: () => onModeChange("plan") },
    { key: "goal", label: "目标模式", enabled: true, title: "切换到目标模式", Icon: Wand2, action: () => onModeChange("goal") },
  ];
  return (
    <div className="macro-mode-strip">
      <div className="macro-mode-status">
        <Activity size={14} />
        <span>{activeMode}</span>
      </div>
      <div className="macro-mode-actions">
        {items.map(({ key, label, enabled, title, Icon, action }) => (
          <button key={key} type="button" disabled={!enabled} title={enabled ? title : `${label} 暂不可用`} onClick={action}>
            <Icon size={14} />
            <span>{label}</span>
          </button>
        ))}
      </div>
    </div>
  );
}

export function MacroControlPanelCard({
  macro,
  onAttach,
  onInsertCard,
  onPreviewValue,
  onCommitValue,
  onRename
}: {
  macro: MacroControl;
  onAttach: () => void;
  onInsertCard: () => void;
  onPreviewValue: (value: number) => void;
  onCommitValue: (value: number) => Promise<void>;
  onRename: (name: string) => Promise<void>;
}) {
  const [busy, setBusy] = useState(false);
  const label = macroControlLabel(macro);
  const [draftName, setDraftName] = useState(label);
  const disabledReason = macroControlDisabledReason(macro);
  const disabled = Boolean(disabledReason);
  useEffect(() => {
    setDraftName(label);
  }, [label, macroControlID(macro)]);
  const commit = async (value: number) => {
    setBusy(true);
    try {
      await onCommitValue(value);
    } finally {
      setBusy(false);
    }
  };
  const commitName = async () => {
    const cleanName = draftName.trim();
    if (!cleanName || cleanName === label) {
      setDraftName(label);
      return;
    }
    setBusy(true);
    try {
      await onRename(cleanName);
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className={`macro-panel-card ${disabled ? "disabled" : ""}`}>
      <div className="macro-card-head">
        <SlidersHorizontal size={16} />
        <div>
          <input
            className="macro-name-input"
            type="text"
            value={draftName}
            title="Macro name"
            aria-label="Macro name"
            disabled={busy || !macroControlID(macro)}
            onChange={(event) => setDraftName(event.currentTarget.value)}
            onBlur={() => void commitName()}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                event.preventDefault();
                event.currentTarget.blur();
              }
              if (event.key === "Escape") {
                event.preventDefault();
                setDraftName(label);
                event.currentTarget.blur();
              }
            }}
          />
          <span>{macroControlMeta(macro)}</span>
        </div>
        <em>{macroValueText(macroControlValue(macro), macro)}</em>
      </div>
      <MacroSlider
        macro={macro}
        disabled={disabled || busy}
        onPreviewValue={onPreviewValue}
        onCommitValue={commit}
      />
      <div className="macro-card-foot">
        <span>{disabledReason || macroBindingSummary(macro)}</span>
        <div>
          <button type="button" title="引用到输入区" onClick={onAttach}>
            <Link2 size={14} />
          </button>
          <button type="button" title="放入对话流控件" onClick={onInsertCard}>
            <Send size={14} />
          </button>
        </div>
      </div>
    </div>
  );
}

export function MacroActionCard({
  action,
  uiState,
  respondingActionID,
  onMacroValuePreview,
  onMacroValueCommit
}: {
  action: JsonRecord;
  uiState: AgentUIState | null;
  respondingActionID: string;
  onMacroValuePreview: (macro: MacroControl, value: number) => void;
  onMacroValueCommit: (macro: MacroControl, value: number) => Promise<void>;
}) {
  const result = asRecord(action.result);
  const actionMacro = normalizeMacroControl(asRecord(action.macro ?? action.control ?? result.macro ?? action.payload ?? result));
  const macroID = macroControlID(actionMacro) || textValue(action.macro_id ?? action.id, "");
  const liveMacro = macroControlsFromUIState(uiState).find((macro) => macroControlID(macro) === macroID);
  const macro = liveMacro ?? actionMacro;
  const isBusy = respondingActionID === `macro:${macroControlID(macro)}`;
  return (
    <div className={`action-card macro-action-card ${macroControlDisabledReason(macro) ? "attention" : "info"}`}>
      <SlidersHorizontal size={16} />
      <div className="action-content">
        <div className="action-title-line">
          <strong>{macroControlLabel(macro)}</strong>
          <span>{liveMacro ? "实时控件" : "快照"}</span>
        </div>
        <MacroSlider
          macro={macro}
          disabled={Boolean(macroControlDisabledReason(macro)) || isBusy}
          onPreviewValue={(value) => onMacroValuePreview(macro, value)}
          onCommitValue={(value) => onMacroValueCommit(macro, value)}
        />
        <div className="macro-inline-meta">
          <span>{macroBindingSummary(macro)}</span>
          <b>{macroValueText(macroControlValue(macro), macro)}</b>
        </div>
      </div>
    </div>
  );
}

export function MacroSlider({
  macro,
  disabled,
  onPreviewValue,
  onCommitValue
}: {
  macro: MacroControl;
  disabled?: boolean;
  onPreviewValue: (value: number) => void;
  onCommitValue: (value: number) => void | Promise<void>;
}) {
  const min = numericValue(macro.min ?? 0);
  const max = numericValue(macro.max ?? 1) || 1;
  const value = macroControlValue(macro);
  const [draft, setDraft] = useState(value);
  const draftRef = useRef(value);
  const lastCommitRef = useRef("");
  useEffect(() => {
    setDraft(value);
    draftRef.current = value;
  }, [macroControlID(macro), value]);
  const commit = () => {
    if (!disabled) {
      const next = draftRef.current;
      const commitKey = `${macroControlID(macro)}:${next.toFixed(4)}`;
      if (lastCommitRef.current === commitKey) {
        return;
      }
      lastCommitRef.current = commitKey;
      void onCommitValue(next);
    }
  };
  return (
    <div className="macro-slider-row">
      <input
        type="range"
        min={min}
        max={max}
        step={0.001}
        value={draft}
        disabled={disabled}
        onChange={(event) => {
          const next = Number(event.currentTarget.value);
          setDraft(next);
          draftRef.current = next;
          onPreviewValue(next);
        }}
        onPointerUp={commit}
        onMouseUp={commit}
        onTouchEnd={commit}
        onBlur={commit}
      />
      <span>{macroValueText(draft, macro)}</span>
    </div>
  );
}

export function MacroContextCard({ macro, onRemove }: { macro: MacroControl; onRemove?: () => void }) {
  return (
    <button className="macro-context-card" type="button" title={macroBindingSummary(macro)}>
      <SlidersHorizontal size={14} />
      <span>{macroControlLabel(macro)}</span>
      <small>{macroValueText(macroControlValue(macro), macro)}</small>
      {onRemove && (
        <i
          role="button"
          aria-label="移除宏控制引用"
          onClick={(event) => {
            event.stopPropagation();
            onRemove();
          }}
        >
          <X size={13} />
        </i>
      )}
    </button>
  );
}


