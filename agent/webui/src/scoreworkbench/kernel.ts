// SCORE-WORKBENCH-1 M2/M3：谱面工作台与内核（经 agent HTTP）的桥接口。
// M2：接口形状 + fail-honest 未连接实现（本地模式）。M3：真实 agent bridge
// （/agent/invoke apply_midi_note_patch[confirmed] + get_midi_clip_notes 回读重建）。
// 红线：治理链不动——apply 走 RiskConfirm 语义（本地缓冲+显式单次确认，369 R2
// 主管工作假设），本模块不做任何绕确认写入。

import type { ScoreModel } from "./model";

export type ApplyPatchOp = {
  op: "replace_region";
  region_start: number;
  region_length: number;
  time_unit: "beats";
  notes: Array<{ pitch: number; start: number; length: number }>;
};

export type KernelBridgeResult<T> = { ok: true; value: T } | { ok: false; error: string };

// ScoreKernelBridge：面板对内核的全部依赖面（apply 后 reload 重建模型=回读回显）。
export interface ScoreKernelBridge {
  readonly kind: "agent" | "none";
  // applyPatch：显式 apply 单次确认（RiskConfirm 语义：调用方已过 UI 确认）。
  applyPatch(patch: ApplyPatchOp): Promise<KernelBridgeResult<true>>;
  // reload：get_midi_clip_notes 回读 → 模型重建（含拍号/tempo 披露）。
  reload(): Promise<KernelBridgeResult<ScoreModel | null>>;
}

// createNoKernelBridge：本地模式（无 agent）——apply fail-honest，不静默假成功。
export function createNoKernelBridge(): ScoreKernelBridge {
  return {
    kind: "none",
    applyPatch: async () => ({ ok: false, error: "未连接内核（本地模式）：写入需真实 agent 栈" }),
    reload: async () => ({ ok: false, error: "未连接内核（本地模式）：回读需真实 agent 栈" })
  };
}
