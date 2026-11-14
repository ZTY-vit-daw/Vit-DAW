import { Icon } from "./icons";

/** 快照回执（SPEC-V5 §8.5）：一行「已保存快照 14:07 · 撤销」，不另起卡片。 */
export interface SnapshotReceiptProps {
  time: string;
  onUndo?: () => void;
  className?: string;
}

export function SnapshotReceipt({ time, onUndo, className }: SnapshotReceiptProps) {
  const cls = ["v5-snapshot-receipt", className].filter(Boolean).join(" ");
  return (
    <p className={cls} data-snapshot-receipt="true">
      <Icon name="i-snapshot" size={16} className="v5-snapshot-icon" />
      <span>已保存快照</span>
      <time className="v5-snapshot-time">{time}</time>
      <button type="button" className="v5-btn v5-btn--text v5-snapshot-undo" onClick={onUndo}>
        <span>撤销</span>
      </button>
    </p>
  );
}
