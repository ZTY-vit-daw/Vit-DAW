// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2h）：App.tsx Workbench 族（K 域）出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
import { Archive, ChevronRight, Clock, Music, Wand2 } from "lucide-react";
import type { LucideIcon } from "lucide-react";
import type { PointerEvent as ReactPointerEvent } from "react";
import type { AgentMode, AgentUIState, ArtifactSummary, MacroControl } from "../types";
import type { WorkbenchTab } from "../App";
import { HistoryPane } from "../history/HistoryPane";
import { MacroPane } from "../macro/MacroPane";
import { MediaPane } from "../media/MediaPane";
import { ScoreTrackWorkbench } from "../scoreworkbench/ScoreTrackWorkbench";

export function PanelResizeHandle({
  side,
  onPointerDown,
  onReset,
  onNudge
}: {
  side: "left" | "right";
  onPointerDown: (event: ReactPointerEvent<HTMLDivElement>) => void;
  onReset: () => void;
  onNudge: (delta: number) => void;
}) {
  return (
    <div
      className={`panel-resize-handle ${side}`}
      role="separator"
      aria-label={`${side === "left" ? "左" : "右"}侧面板宽度`}
      aria-orientation="vertical"
      tabIndex={0}
      title="拖拽调整宽度；双击恢复默认"
      onPointerDown={onPointerDown}
      onDoubleClick={onReset}
      onKeyDown={(event) => {
        if (event.key === "ArrowLeft") {
          event.preventDefault();
          onNudge(-16);
        } else if (event.key === "ArrowRight") {
          event.preventDefault();
          onNudge(16);
        } else if (event.key === "Home") {
          event.preventDefault();
          onReset();
        }
      }}
    />
  );
}

export function CollapsedWorkbench({ activeTab, onOpen }: { activeTab: WorkbenchTab; onOpen: (tab: WorkbenchTab) => void }) {
  return (
    <aside className="workbench-collapsed" aria-label="右侧工程面板">
      {workbenchTabs.map(({ key, label, icon: Icon }) => (
        <button key={key} type="button" className={activeTab === key ? "active" : ""} title={`打开${label}`} onClick={() => onOpen(key)}>
          <Icon size={16} />
          <span>{label}</span>
        </button>
      ))}
    </aside>
  );
}

export function WorkbenchTabs({
  activeTab,
  setActiveTab,
  onCollapse
}: {
  activeTab: WorkbenchTab;
  setActiveTab: (tab: WorkbenchTab) => void;
  onCollapse?: () => void;
}) {
  return (
    <div className="workbench-tabs">
      {workbenchTabs.map(({ key, label, icon: Icon }) => (
        <button key={key} type="button" className={activeTab === key ? "active" : ""} data-workbench-tab={key} onClick={() => setActiveTab(key)}>
          <Icon size={15} />
          <span>{label}</span>
        </button>
      ))}
      {onCollapse && (
        <button className="collapse-workbench" type="button" title="收起右侧面板" onClick={onCollapse}>
          <ChevronRight size={16} />
          <span>收起</span>
        </button>
      )}
    </div>
  );
}

export function Workbench({
  activeTab,
  uiState,
  artifacts,
  selectedArtifact,
  conversationID,
  onSelectArtifact,
  onAttachArtifact,
  onRenameArtifact,
	onDeleteArtifact,
	onUploadClick,
	onModeChange,
	onAttachMacroControl,
  onInsertMacroControlCard,
  onMacroValuePreview,
  onMacroValueCommit,
  onMacroRename,
  onRefresh,
  checkoutBlocked
}: {
  activeTab: WorkbenchTab;
  uiState: AgentUIState | null;
  artifacts: ArtifactSummary[];
  selectedArtifact: ArtifactSummary | null;
  conversationID: string;
  onSelectArtifact: (id: string) => void;
  onAttachArtifact: (artifact: ArtifactSummary) => void;
  onRenameArtifact: (id: string, title: string) => Promise<void>;
  onDeleteArtifact: (id: string) => Promise<void>;
  onUploadClick: () => void;
  onModeChange: (mode: AgentMode) => void;
	onAttachMacroControl: (macro: MacroControl) => void;
  onInsertMacroControlCard: (macro: MacroControl) => void;
  onMacroValuePreview: (macro: MacroControl, value: number) => void;
  onMacroValueCommit: (macro: MacroControl, value: number) => Promise<void>;
  onMacroRename: (macro: MacroControl, name: string) => Promise<void>;
  onRefresh: () => Promise<void>;
  checkoutBlocked: boolean;
}) {
  if (activeTab === "history") {
    return <HistoryPane uiState={uiState} onRefresh={onRefresh} checkoutBlocked={checkoutBlocked} />;
  }
  if (activeTab === "score") {
    // 397: explicit track/clip identity and switching guard wrap the existing
    // score editor; its confirmed replace_region/readback bridge is unchanged.
    return <ScoreTrackWorkbench />;
  }
  if (activeTab === "macro") {
    return (
      <MacroPane
        uiState={uiState}
        onUploadClick={onUploadClick}
        onModeChange={onModeChange}
		onAttachMacroControl={onAttachMacroControl}
        onInsertMacroControlCard={onInsertMacroControlCard}
        onMacroValuePreview={onMacroValuePreview}
        onMacroValueCommit={onMacroValueCommit}
        onMacroRename={onMacroRename}
        onRefresh={onRefresh}
      />
    );
  }
  return (
    <MediaPane
      artifacts={artifacts}
      selectedArtifact={selectedArtifact}
      conversationID={conversationID}
      uiState={uiState}
      onSelectArtifact={onSelectArtifact}
      onAttachArtifact={onAttachArtifact}
      onRenameArtifact={onRenameArtifact}
      onDeleteArtifact={onDeleteArtifact}
      onUploadClick={onUploadClick}
      onRefresh={onRefresh}
    />
  );
}

const workbenchTabs: Array<{ key: WorkbenchTab; label: string; icon: LucideIcon }> = [
  { key: "media", label: "资料库", icon: Archive },
  { key: "macro", label: "宏控制", icon: Wand2 },
  { key: "score", label: "谱面", icon: Music },
  { key: "history", label: "历史树", icon: Clock }
];

