import { useMemo, useState } from "react";
import type { AgentEvent } from "../types";
import { emptyTrajectoryState, reduceTrajectoryEvents } from "../trajectory";
import { mockMultiRoundTrajectoryEvents, mockRollbackTrajectoryEvents, mockStoppedTrajectoryEvents } from "../trajectoryMock";
import { TrajectoryView } from "./TrajectoryView";
import "./trajectory.css";

type MockVariant = "multi_round" | "rollback" | "stopped";

const variants: Array<{ key: MockVariant; label: string; hint: string; events: AgentEvent[] }> = [
  { key: "multi_round", label: "多轮实验", hint: "处理量不足 → 目标响应 → 收口", events: mockMultiRoundTrajectoryEvents },
  { key: "rollback", label: "回退", hint: "副作用过大 → 恢复支点", events: mockRollbackTrajectoryEvents },
  { key: "stopped", label: "停止", hint: "用户停止 → 保留稳定状态", events: mockStoppedTrajectoryEvents }
];

export function TrajectoryMockView() {
  const [variant, setVariant] = useState<MockVariant>("multi_round");
  const selected = variants.find((item) => item.key === variant) ?? variants[0];
  const state = useMemo(() => reduceTrajectoryEvents(emptyTrajectoryState(), selected.events), [selected.events]);
  return (
    <main className="trajectory-demo-shell">
      <div className="trajectory-demo-topbar">
        <div>
          <span className="trajectory-demo-mark">VIT</span>
          <strong>自由态实验轨迹 · Mock</strong>
        </div>
        <span>G2 / reducer preview</span>
      </div>
      <div className="trajectory-demo-layout">
        <aside className="trajectory-demo-scenarios" aria-label="轨迹 Mock 场景">
          <span className="trajectory-demo-section-label">场景</span>
          {variants.map((item) => (
            <button key={item.key} type="button" className={item.key === variant ? "is-selected" : ""} onClick={() => setVariant(item.key)}>
              <strong>{item.label}</strong>
              <small>{item.hint}</small>
            </button>
          ))}
          <div className="trajectory-demo-note">只显示可审计节点，不显示原始思维链。</div>
        </aside>
        <div className="trajectory-demo-content"><TrajectoryView state={state} title={selected.label} /></div>
      </div>
    </main>
  );
}
