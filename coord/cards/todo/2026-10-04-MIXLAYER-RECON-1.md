# MIXLAYER-RECON-1：A-F 能力层实现度盘点——黑板/账本/queue/mix.report+契约接线+typed route 缺口（只读勘察）

- 池序 4（M1 harness 更新批内前置盘点）；目标仓库=D:\Vit_DAW（只读）；依据=[decisions/2026-10-04-capability-layer-harness-ruling.md](../../decisions/2026-10-04-capability-layer-harness-ruling.md) 裁定 3
- 优先级 / 预估 / 依赖：P2 / 0.25 天只读 / 无
- 模型分级：L1 / **flash 可接**（纯勘察）
- 盘点问题（每条带代码/文档锚点回答）：
  1. **黑板/账本/queue 实现度**：`mix_workflow_queue.v0` / `mix_decision_record.v1` / `mix.report` / `project.blackboard.status_report.v0` 四件——各是实现、部分实现还是纯设计？（查 agent/internal/mixboard 一带+MIXBOARD_DECISION_LEDGER_V1.md 对照实现）；needs_review 影响联动是否存在。
  2. **A-F 各族契约接线现状**：A1-A5/B1-B4/C1-C5/D1-D4/E1-E4/F1 逐节点标注：投影/命令面已备（只需契约接线）/部分/缺（需开发）——重点核对 B 族（static_mix 契约 v0 后有多少 Session 形态落地）与 C 族（C1/C2/C3 观察在但能力 Session 呢）。
  3. **typed route 缺口核对**：master_plan §6.3 七项（采样率/电平微调/声像宽度/automation 读写/clip gain-fade-静音/bus-send-FX return/reference-export-stems）逐项在内核命令表+agent catalog 的现状（在位/部分/缺）。
  4. **自由态 settle→黑板供血路径现状**：settle 报告现在写到哪（会话/单图/账本？），距"黑板数据源"差什么。
- 产出：`coord/runs/MIXLAYER-RECON-1/RECON.md`（四问各带锚点+一张 A-F×实现度矩阵+M1 内 A-F 工作量估算表）。
- 约束：只读（零代码改动零探针写源码树）；不动 sealed fixture。
- 验收标准：四问全答+锚点可回查+矩阵覆盖 23 节点+工作量估算明确（不代决拆卡——决策侧复核后拆）。
- 领取：（时间 / 基线）
- 回执：（RECON.md 路径）
- 验收：（裁定文件）
