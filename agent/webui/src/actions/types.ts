// T9-PRE-1B-1（386 / 施工图 UI-CONVERGE-RECON-1 §3.1 批 2 / 步 2a）：App.tsx 动作卡体系全域出域。
// 纯移动+逐字搬运，行为零变化；App.tsx 经 re-export 兼容层保持命名面不变。
// 动作卡 props/视图类型（跨卡共享，无运行时依赖）。

import type { AgentUIState, AuthorityMode, JsonRecord, MacroControl } from "../types";
import type { DawInvoke } from "../daw/parse";

export type ActionCardProps = {
  action: JsonRecord;
  index: number;
  respondingActionID: string;
  onInteractionAction: (interaction: JsonRecord, action: JsonRecord, payload?: JsonRecord) => void;
  onInvoke: DawInvoke;
  onSelectArtifact: (id: string) => void;
  uiState: AgentUIState | null;
  onMacroValuePreview: (macro: MacroControl, value: number) => void;
  onMacroValueCommit: (macro: MacroControl, value: number) => Promise<void>;
  /** 完全档下确认卡静默沉淀（GUI-T4 ③ UI 层兜底，agent 侧仍发 proposal） */
  authorityMode?: AuthorityMode;
  /** 卡面补充输入直发对话（免选路径，发送后卡面选项未采用） */
  onSendSupplement?: (text: string) => void;
};

export type ProjectResultReadiness = "ready" | "silent_risk" | "affected_by_mute" | "unknown" | "not_auditionable";
export type ProjectResultView = {
  title: string;
  badge: string;
  body: string;
  hint: string;
  readiness: ProjectResultReadiness;
  canAudition: boolean;
  auditionLabel: string;
  target: JsonRecord;
  details: Array<{ id: string; title: string; body: string }>;
};

export type ProjectResultABView = {
  title: string;
  body: string;
  status: string;
  tapPoint: string;
  renderMode: string;
  deltaSummary: string;
};
