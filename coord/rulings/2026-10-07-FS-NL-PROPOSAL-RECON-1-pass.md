# Ruling：FS-NL-PROPOSAL-RECON-1 验收 pass（2026-10-07，决策侧）

- 对象：自由态 NL 提案治理面抖动勘察（报告 coord/runs/FS-NL-PROPOSAL-RECON-1/REPORT.md，零代码改动）
- 裁定：**pass**——取证结论全部经决策侧亲证：

1. **核心锚点亲读**（goalrunner_chat.go:463-512 响应路由门）：capability_blocked 决策（loop 非 awaiting_action/awaiting_experiment）落入 plain 分支 `chatResponseFromAgentLoopResult`——散文终答当 done 投递；`improvementProposalResponse`（挂卡路径）仅在 awaitingExperiment 时可达。**H2 漏拦点定位与代码实态逐字吻合**。
2. **R3 receipt 亲读**（journey_nl_runtime_status_final.json）：`proposal_present: true, proposal_valid: true, status: capability_blocked, failed_gate_ids: [G6_target_evidence, G8_target_consistency]`——"模型给了有效结构化提案被门拒绝"的定性由原始工件坐实，非推断。
3. 采信序 **H2（主因）+H3（触发器）复合、H1 排除**；N=4 机制分解（成功 1/gate 拒绝 1/预算耗尽 1/驱动缺陷已修 1）与抖动率口径合理。
4. 修复卡按建议立卡：**FS-CAPABILITY-BLOCKED-SURFACE-1**（P2，主腿=capability_blocked 显式边界响应）。

## 后续

- J3 的 0/3 抖动根因就此闭合：不是链路坏，是失败形被吞。修复卡落地后 journey_free_state_nl 成功率预期回升（gate 拒绝形可重试）。
