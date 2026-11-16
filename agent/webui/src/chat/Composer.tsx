import { Brain, Loader2, Plus, Send, Square, Upload, Wand2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { CSSProperties, FormEvent, PointerEvent as ReactPointerEvent, RefObject } from "react";
import type { AgentMode, ArtifactSummary, AuthorityMode, JsonRecord, MacroControl } from "../types";
import type { DawInvoke } from "../daw/parse";
import { clampComposerInteractionHeight, defaultComposerInteractionHeight } from "../lib/util";
import { macroControlID } from "../macro/normalize";
import { ActionCard } from "../actions/ActionCards";
import { ArtifactContextCard, AuthorityToggle, MacroContextCard } from "../App";

export function Composer({
  composerRef,
  input,
  setInput,
  pendingArtifacts,
  pendingMacroControls,
	mode,
	agentTurnRunning,
  stopTurnBusy,
  isSending,
  isUploading,
  interactionAction,
  respondingActionID,
  authorityMode,
  authorityLocked,
  onAuthorityModeChange,
  onSubmit,
  onUploadClick,
  onModeChange,
	onStopTurn,
  onInteractionAction,
  onInvoke,
  onSelectArtifact,
  onRemoveArtifact,
  onRemoveMacroControl
}: {
  composerRef: RefObject<HTMLFormElement | null>;
  input: string;
  setInput: (value: string) => void;
  pendingArtifacts: ArtifactSummary[];
  pendingMacroControls: MacroControl[];
  mode: AgentMode;
  agentTurnRunning: boolean;
  stopTurnBusy: boolean;
  isSending: boolean;
  isUploading: boolean;
	interactionAction: JsonRecord | null;
  respondingActionID: string;
  authorityMode: AuthorityMode;
  /** authorityBusy || agentTurnRunning：回合进行中锁定档位（服务端权威规则同源） */
  authorityLocked: boolean;
  onAuthorityModeChange: (mode: AuthorityMode) => void;
  onSubmit: (event?: FormEvent) => void;
  onUploadClick: () => void;
  onModeChange: (mode: AgentMode) => void;
  onStopTurn: () => void;
  onInteractionAction: (interaction: JsonRecord, action: JsonRecord, payload?: JsonRecord) => void;
  onInvoke: DawInvoke;
  onSelectArtifact: (id: string) => void;
  onRemoveArtifact: (id: string) => void;
  onRemoveMacroControl: (id: string) => void;
}) {
  const [menuOpen, setMenuOpen] = useState(false);
  const [interactionHeight, setInteractionHeight] = useState(defaultComposerInteractionHeight);
  const menuRef = useRef<HTMLDivElement | null>(null);
  const interactionShellRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!menuOpen) {
      return;
    }
    const handlePointerDown = (event: MouseEvent) => {
      if (!menuRef.current?.contains(event.target as Node)) {
        setMenuOpen(false);
      }
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [menuOpen]);

  const chooseMode = (nextMode: AgentMode) => {
    onModeChange(mode === nextMode ? "default" : nextMode);
    setMenuOpen(false);
  };

  const beginInteractionResize = (event: ReactPointerEvent<HTMLDivElement>) => {
    event.preventDefault();
    const startY = event.clientY;
    const startHeight = interactionShellRef.current?.getBoundingClientRect().height ?? interactionHeight;
    const handlePointerMove = (moveEvent: PointerEvent) => {
      const nextHeight = clampComposerInteractionHeight(startHeight + startY - moveEvent.clientY);
      setInteractionHeight(nextHeight);
    };
    const handlePointerUp = () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", handlePointerUp);
      window.localStorage?.setItem("ask_vit_composer_interaction_height", String(interactionShellRef.current?.getBoundingClientRect().height ?? interactionHeight));
    };
    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", handlePointerUp);
  };

  return (
    <form className="composer" ref={composerRef} onSubmit={onSubmit}>
      {interactionAction && (
        <div className="composer-interaction-shell" ref={interactionShellRef} style={{ height: `${interactionHeight}px` }}>
          <div className="composer-resize-handle" role="separator" aria-orientation="horizontal" title="拖拽调整交互面板高度" onPointerDown={beginInteractionResize} />
          <ActionCard
            action={interactionAction}
            index={0}
            respondingActionID={respondingActionID}
            onInteractionAction={onInteractionAction}
            onInvoke={onInvoke}
            onSelectArtifact={onSelectArtifact}
            uiState={null}
            onMacroValuePreview={() => undefined}
            onMacroValueCommit={async () => undefined}
          />
        </div>
      )}
      {pendingArtifacts.length > 0 && (
        <div className="pending-row artifact-context-list">
          {pendingArtifacts.map((artifact) => (
            <ArtifactContextCard
              key={artifact.id}
              artifact={artifact}
              variant="compact"
              onSelect={() => onSelectArtifact(artifact.id)}
              onRemove={() => onRemoveArtifact(artifact.id)}
            />
          ))}
        </div>
      )}
      {pendingMacroControls.length > 0 && (
        <div className="pending-row macro-context-list">
          {pendingMacroControls.map((macro) => (
            <MacroContextCard
              key={macroControlID(macro)}
              macro={macro}
              onRemove={() => onRemoveMacroControl(macroControlID(macro))}
            />
          ))}
        </div>
      )}
      <div className="composer-main">
        <textarea
          value={input}
          rows={1}
          placeholder="和 Vit 说..."
          onChange={(event) => setInput(event.currentTarget.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) {
              event.preventDefault();
              onSubmit();
            }
          }}
        />
        <div className="composer-toolbar">
          <div className="composer-menu-host" ref={menuRef}>
            <button
              className={`icon-button upload ${menuOpen ? "active" : ""}`}
              type="button"
              title="添加上下文、文件或模式选项"
              onClick={() => setMenuOpen((current) => !current)}
              disabled={isUploading}
            >
              {isUploading ? <Loader2 className="spin" size={18} /> : <Plus size={18} />}
            </button>
            {menuOpen && (
              <div className="composer-menu" role="menu">
                <button type="button" role="menuitem" onClick={() => { onUploadClick(); setMenuOpen(false); }}>
                  <Upload size={17} />
                  <span>添加照片和文件</span>
                </button>
                <div className="composer-menu-separator" />
                <button type="button" role="menuitemcheckbox" aria-checked={mode === "plan"} onClick={() => chooseMode("plan")}>
                  <Brain size={17} />
                  <span>计划模式</span>
                  <i className={mode === "plan" ? "on" : ""} />
                </button>
                <button type="button" role="menuitemcheckbox" aria-checked={mode === "goal"} onClick={() => chooseMode("goal")}>
                  <Wand2 size={17} />
                  <span>追求目标</span>
                  <i className={mode === "goal" ? "on" : ""} />
                </button>
              </div>
            )}
          </div>
          <AuthorityToggle authorityMode={authorityMode} authorityLocked={authorityLocked} onAuthorityModeChange={onAuthorityModeChange} />
          <div className="composer-toolbar-spacer" />
          {agentTurnRunning ? (
            <button className="send-button stop-turn-button" type="button" title="Stop Turn" disabled={stopTurnBusy} onClick={onStopTurn}>
              {stopTurnBusy ? <Loader2 className="spin" size={18} /> : <Square size={17} />}
              <span>Stop Turn</span>
            </button>
          ) : (
            <button className="send-button" type="submit" title="Send" disabled={isSending}>
              {isSending ? <Loader2 className="spin" size={18} /> : <Send size={18} />}
              <span>发送</span>
            </button>
          )}
        </div>
      </div>
    </form>
  );
}

