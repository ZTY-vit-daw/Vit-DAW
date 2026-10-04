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
- 领取：2026-10-04 / `4de389a5900c54ec485d2802e6ecc32c72097b30`（Windows 主工作树；领取时工作树已有与本卡无关的 Settings.xml/default_project.xml 改动及 coord/runs 未跟踪目录，未触碰）
- 回执：`coord/runs/MIXLAYER-RECON-1/RECON.md`——四问全答+锚点可回查+A-F×实现度 23 节点矩阵+M1 内工作量估算（6-10 天分块，拆卡权留决策侧）。核心读数：账本+mix.report 已实现且 B2/B3/B4/C1/C2 五族读写接线；mix_workflow_queue.v0 代码零命中纯设计；黑板=每回合现算无持久层且 `static_mix_capabilities` 等三个状态键全仓无生产者；自由态 settle 与账本/黑板零桥接（conversation 键控 vs project_uuid 键控）；typed route 七项中采样率/电平/声像在位，宽度/automation/clip 静音写/bus-send/导出暴露缺。执行侧未提交 commit（按 §12 等决策验收；且主工作树有他卡前置改动）——卡片移动+RECON.md 为 coord/ 变更，提交由决策侧处置。
- 端测边界声明：本卡为纯只读勘察，无运行链路改动，不适用端侧烟测门槛；结论以源码锚点与全仓 grep 判定为据。
- 验收：pass（[rulings/2026-10-04-MIXLAYER-RECON-1-pass.md](../../rulings/2026-10-04-MIXLAYER-RECON-1-pass.md)，2026-10-04 晚）——锚点抽查 3/3 属实（queue 零命中/三键无生产者/impact 契约五 ID）；M1 拆卡建议五序见裁定。
