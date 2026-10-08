# FS-CAPABILITY-BLOCKED-SURFACE-1：capability_blocked 显式边界响应——gate 拒绝不再被散文终答吞没（RECON-1 修复卡）

- 池序 19（FS-NL-PROPOSAL-RECON-1 验收裁定立卡；JOURNEY-4 复活条件①）；目标仓库=D:\Vit_DAW（PC 执行侧）
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无（取证证据全在案）
- 模型分级：L1 / **flash 可接**（锚点精确+测试面清晰；chat 单域）
- 依据：[RECON-1 报告](../../runs/FS-NL-PROPOSAL-RECON-1/REPORT.md)（H2 主因+R3 receipt 证据）+ [rulings/2026-10-07-FS-NL-PROPOSAL-RECON-1-pass.md](../rulings/2026-10-07-FS-NL-PROPOSAL-RECON-1-pass.md)
- 已核实事实（决策侧亲证，实现前回查）：
  1. R3 receipt：`proposal_present=true, proposal_valid=true, status=capability_blocked, failed_gate_ids=[G6_target_evidence, G8_target_consistency]`（journey_nl_runtime_status_final.json 工件）——模型给了有效结构化提案被 G 门拒绝。
  2. 漏拦点：`agent/internal/chat/goalrunner_chat.go` 响应路由门——capability_blocked 决策（loop 非 awaiting_action/awaiting_experiment）落 plain 分支 `chatResponseFromAgentLoopResult`，散文终答当 done 投递；`improvementProposalResponse`（挂卡路径）仅 awaitingExperiment 可达。
  3. 自由态 admission receipt 位置：`free_state_admission_receipt`（boundary=admission_gate_failed 时含 failed_gate_ids 全量）在 res/loop 面可用（R3 工件实证）。
- 目标：
  1. **主腿（响应兜底）**：路由门 plain 分支前加 capability_blocked 显式边界——当 freeStateActive 且 LatestDecision.Status=capability_blocked（或 admission receipt boundary=admission_gate_failed）时：stop_reason=`capability_blocked`、WorkflowData 携带 `free_state_admission_receipt`（含 failed_gate_ids）、回复文本=明确的 capability 边界陈述（不承诺执行、不假装成功——RECON-1 建议：goalrunner_chat.go:478 分支，改动面≈1 个分支）。**语义红线**：不自动重试、不绕过 G 门（G 门拒绝本身是正确执法，修的是"失败形表面化"，不是"让拒绝变通过"）。
  2. **可选腿（nudge 兜底）**：capability_blocked 形一轮确定性重试 nudge（journey1 证据工具先例）——若主腿+测试余量足则做，否则留后续卡。
  3. **测试**：构造 gate 拒绝形态（G6/G8 fail 的 admission receipt）的响应路由单测——断言 stop_reason/WorkflowData receipt/文本边界三面；既有 awaitingAction/awaitingExperiment/plain 三路径回归零变化。
  4. **复现轮**：`-Scenario journey_free_state_nl -StartKernel` 1-2 轮（§8 口径：成功=capability_blocked 形显式表面化【若触发】或全链走通；失败分类分记；止损=2 轮同形即停）。
- 文件域：`agent/internal/chat/goalrunner_chat.go`+测试（预计 ≤3 文件，越域先申报）。
- 约束：G1-G8 门逻辑零改动；improvement_proposal_workflow.go 零改动（挂卡路径行为不变）；R3 的散文终答文本本身不回填断言（仍是组装面断言）。
- 验收标准：单测三路径回归+新边界路径全绿+`go build ./...`+`go test ./... -count=1` 全量 0 FAIL+gofmt 净+复现轮工件（§8 如实记录）；决策侧复核复现轮。
- 停止条件：路由门实态与取证锚点不符（行漂移或分支重构）→ 新锚点+差异清单上交。
- 领取：2026-10-08 早窗派发 flash（用户转交） / origin/main=d154cffc / port/fs-capability-blocked-surface-1（独立 worktree，决策侧代行移动）
- 回执：（commit hash / 改动面申报 / 新测试名 / 复现轮工件与分类 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
