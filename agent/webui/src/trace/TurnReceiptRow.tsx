import { StateIcon, terminalClass } from "../taskTrajectory/details";
import { traceMetaParts } from "./TraceBlock";
import { turnReceiptStatusLabel, type TurnReceipt } from "./turnReceipts";
import "./trace.css";

// TRAJ-IMPL-3（设计 §2.3 分期一② + §7 裁定 C「一行先做出来看」）：**静态收起回执行行**。
//
// 它是刷新后「活态缺失」的回合唯一的呈现形态——不是活块，不是占位：一行状态 + 步数 +
// 时长（明细展开属分期二）。三条呈现纪律：
//   ① **静态**：无按钮、无 chevron、无转圈、无游标、无计时器（活态不是它的事）；
//   ② **只显摘要**：状态/计数/时长，正文与步骤标题一个字都不复述（正文归气泡）；
//   ③ 句式与终态回合块**同源**：meta 走 TraceBlock.traceMetaParts（设计 §2.4-3 冻结的
//      终态句式，'执行 Xs · 等待续跑 Ys' 只有一个出处），不另造第二套写法。

/**
 * 回执行句式（正文之外的三个事实：状态、步数/活动数、时长）。
 *
 * 时长无证据（workMs = null）时**不虚报 0.0s**：traceMetaParts 的 formatSeconds 对
 * 非有限值输出 "--"，因此这里把「无证据」翻译成 NaN 而不是 0——行照样显示计数，
 * 只是不宣称一个它不知道的时长。
 */
export function turnReceiptMetaParts(receipt: TurnReceipt): string[] {
  const workMs = typeof receipt.workMs === "number" && Number.isFinite(receipt.workMs) ? receipt.workMs : null;
  return traceMetaParts({
    live: false,
    stepCount: receipt.stepCount,
    stepSpanMs: workMs === null ? Number.NaN : workMs,
    itemActivityCount: receipt.activityCount,
    itemActivitySpanMs: workMs,
    parkMs: receipt.parkMs
  });
}

/**
 * 回执行整行文案（状态 + ' · ' + 摘要）——**渲染文本的唯一出处**：组件按它排版，
 * 单元测试与渲染面断言按它核对，三者不各写一套分隔符。
 */
export function turnReceiptText(receipt: TurnReceipt): string {
  const label = turnReceiptStatusLabel(receipt.status);
  const parts = turnReceiptMetaParts(receipt);
  return `${label} · ${parts.join(" · ")}`;
}

export function TurnReceiptRow({ receipt }: { receipt: TurnReceipt }) {
  const label = turnReceiptStatusLabel(receipt.status);
  const parts = turnReceiptMetaParts(receipt);
  return (
    <section
      className={["trace-receipt", terminalClass(receipt.status)].filter(Boolean).join(" ")}
      data-turn-id={receipt.turnId}
      data-receipt-status={receipt.status}
      aria-label={`执行回执：${label}`}
    >
      <div className="trace-receipt-head" role="status">
        <StateIcon state={receipt.status} />
        <strong className="trace-receipt-label">{label}</strong>{" · "}
        <span className="trace-receipt-meta">{parts.join(" · ")}</span>
      </div>
    </section>
  );
}
