# Ruling：JOURNEY-4 blocked 处置（2026-10-07，决策侧）——探针坐实停止条件①，卡面 parked 正确

- 对象：A/B 试听判定旅程（探针腿 run 20261007_210801 exit 0；报告 coord/runs/JOURNEY-1/20261007_210801/journey4_probe_report.md）
- 裁定：**blocked 状态维持（停止条件①执行正确）+ 复活条件已定**：

1. **探针证据亲核**：journey_first +45 行 record-only 事件探针段（diff 亲读，throw 命中仅在注释）入 main（8d283c3c）；B2 确定性链 5 事件流 audition=0 vs R4 自由态链干预后三事件（prepare.started→candidate.ready→ready）——**确定性装卡入口不存在于现有零 LLM 面**，与锚点清单 A1（prepareFreeStateAudition 唯一装配路径=自由态 runtime）一致。
2. **journey_first 探针段保留**：audition 面记录成为 J1 场景永久部分，J4-REV 直接复用。
3. **复活条件（二选一触发）**：①FS-CAPABILITY-BLOCKED-SURFACE-1 落地后，J3 链成功率回升 → 立 J4-REV 卡（改 J3 链承接：概率装卡按 §8 预算+确定性判定面 status→select→judgment 事件链+盲听腿）；②决策侧另裁确定性装卡驱动端点（agent 侧新面，独立 scope）。
4. 卡留 blocked/，不做 done（未达成判定腿=未完成，如实状态）。
