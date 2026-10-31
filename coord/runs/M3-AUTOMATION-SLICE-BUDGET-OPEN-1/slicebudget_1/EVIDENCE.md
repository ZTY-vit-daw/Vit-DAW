# 232 真栈重验腿证据锚点（slicebudget_1）

run：coord/runs/M3-AUTOMATION-SLICE-BUDGET-OPEN-1/slicebudget_1（2026-10-31 20:04-20:24 UTC+8）
ps1 exit 0；kernel=wt_228 VitApp.exe sha256 f9e3da0a；被测=wt_232impl @ port/m3-automation-slice-budget-open-1（96a9b224）。

## 本卡两标的生效证据

1. 生产面（片预算放开）：
   - 五个 capability_blocked 轮全部为**同步终局**（agent_last.log timing 行：status=completed stop=capability_blocked，total_ms 80,842-184,395；completed_steps 3-6 / executed 3-8）——无一轮被 turn 预算切断。
   - 全 run 唯一一次 limit_reached=g1_r4（Timeout 180s 切片，LimitType 语义），异步续跑由 250ms scheduler 正常接力，等待窗 3 polls 后观测到终局。
   - 231 形态（2-3 工具即 limit_reached→排队文本淹没）消失：模型每轮稳定跑完 观察→catalog→（get_parameters）→终局申报 链。
2. 仪器面（终态等待保险丝）：
   - g1_r4 全链在卷：limit_reached→「继续」→automatic_continuation_already_scheduled 排队文本→等待窗（terminal_wait polls=3）→goal 终局 failed→按终局分类（无排队文本误判 no_route）。
   - choreo-pending 不干扰腿 group_1=pending_intact（确认卡机制在预算放开后不受扰）。

## 229 第四承未达——两个越域生产缺陷（锚点+结构化证据）

### A. FS 准入门 G6/G8 拒绝（capability_boundary 结构化披露，非模型话术）

g1_r1/g1_r3/g2_r1-r3 的 capability_blocked 均携带 workflow_data.capability_boundary + free_state_admission_receipt：

- code=admission_gate_failed, class=free_state, track_id=1010
- gate_results：G1_project_binding/G2_capacity_assessed/G3_project_scan/G4_dimension_closed/G5_frontier_established/G7_fresh_revision_bound_refs 全 **pass**；**G6_target_evidence fail + G8_target_consistency fail**
- proposal_present=true, proposal_valid=true, target_evidence_ref=obs_20261031T120646_69938d440c7d（观察 ref 在场仍被判 G6 不可用——判定/view 家族匹配问题）
- unavailable_reasons=["certification_entry_unavailable_for_family","family_unresolved","no_loaded_instance_for_manual_guidance"]

锚点：
- agent/internal/agentloop/free_state_gate.go:312（gateG6：frontier candidate track_ids + usable target-level track.* 观察）
- agent/internal/agentloop/free_state_gate.go:444（gateG8：proposal target ∈ frontier + ledger usable receipt 同轨绑定）
- agent/internal/capabilityboundary/resolver.go:18（certification_entry_unavailable_for_family 等 unavailable_reasons 渲染）
- agent/internal/chat/free_state_reasoning_loop.go:3602（chat 侧 gate_results 表）

### B. taskstate 重复转移拒绝（durable 续跑 settle 冲突）

g1_r4 终局 error（terminal_wait.final_goal.error）：
`durable continuation failed: minimal audio closure controller failed: semantic transition capability_blocked is not allowed from capability_blocked`

锚点：agent/internal/taskstate/state.go:515-517（invalidTransition）。closure 已处于 capability_blocked 后续跑再次 settle capability_blocked 被状态机拒绝→goal failed。仪器面按终局兜底 no_route 分类（goal failed 无写入/确认/能力面）——组 1 no_route×3 的第三个实为该缺陷的表现。

## 组级结果（summary 原口径）

- group_1：5 轮 [capability_blocked, no_route, capability_blocked, no_route, no_route]，successes=0，group_pass=false，止损未触发（无同型三连），choreo_pending=pending_intact
- group_2：3 轮 [capability_blocked ×3]，successes=0，止损触发（同型三连）
- group_3：skipped（stop_loss_in_earlier_group，220 原口径）
- prompt_guidance_rendered=true；status=COMPLETED

## §8 计费

deepseek-flash 103 调用，1,512,840 input / 160,974 output tokens（轮窗放大后单轮链内调用数上升——229/231 同腿约 30 调用量级）。
