# SCORE-DG1A-COMPLETE-1：补完 DG1-A 的 A7/A8 出声+差分断言（微卡）

- 发卡：主管 2026-11-16 晨（承接 374 验收遗留——A7/A8 当时 skipped-pending-373，现 373/375 已合入解锁）
- 池序 377；P1；依赖=373/375 已合入✓；真栈候栈位
- 目标：score_workbench_smoke.mjs 补 A7（谱面编辑→内核出声探针非静音，复用 373 DG1-B 判据）+A8（编辑前后 WAV 差分）断言；真栈轮 exit 0；三门不回退。
- 文件域：scripts/score_workbench_smoke.mjs+dev_agent_smoke 透传缝只读+coord 工件。
- 验收：DG1-A 全断言含 A7/A8 真栈 PASS 亲证。

## 领取/回执/验收：待回填
