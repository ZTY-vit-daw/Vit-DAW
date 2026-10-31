# M3-FS-GATE-EVIDENCE-WIRE-1：FS 准入 G6/G8 证据接线+capabilityboundary family 断线+taskstate 幂等重入（229 验收门第五承，三合一修复）

- 发卡：GLM 主管决策流 / 2026-10-31（依据=232 验收定案+用户裁定「发 233 三合一修复卡」：两生产缺陷主管独立亲证——A=G6/G8 对在场的 target_evidence_ref 判不可用+capability_boundary family=None 断线（g1_r1 回执亲读：G1-G5+G7 全 pass+proposal_present/valid=true 仍 fail；工具名匹配面已排除低级错误）；B=taskstate capability_blocked→capability_blocked 幂等重入被拒→goal failed（state.go:515 亲核）。模型行为面五卡零反例，G1-G5+G7 全过=只差最后一道接线的门）
- 派发确认：已确认（用户 2026-10-31）
- 验收负责人：GLM 主管决策流
- 池序 233；目标仓库=**主仓 agent agentloop/capabilityboundary/taskstate 域**；P1 / 0.5-1 天 / 依赖=232 已合入（e32de775）✓；L2 / PC 执行会话（准入语义+状态机判读）

## 标的（232 定案，不重考）

G6（frontier 候选轨覆盖+可用轨级观察）与 G8（提案目标∈frontier+ledger 同轨绑定）对在场证据误判不可用：target_evidence_ref=obs_…非空、proposal_valid=true 仍 fail，frontier 折空 blockers=[G6,G8]；capability_boundary 输入 family=None（提案为音量自动化，家族字段应解析为 volume 族）→family_unresolved+certification_entry_unavailable+no_loaded_instance 三连锁全因断线；taskstate 重复 settle 同型终态被 invalidTransition 拒绝把 goal 标 failed。锚点：`agent/internal/agentloop/free_state_gate.go:312`（gateG6）/`:444`（gateG8）、`agent/internal/agentloop/free_state_reasoning.go:2687`（CCB 观察名匹配——已排除）、`agent/internal/capabilityboundary/resolver.go:18`（unavailable 渲染，输入断线在上游）、`agent/internal/chat/free_state_reasoning_loop.go:3602`（chat 侧 gate_results 审计表）、`agent/internal/taskstate/state.go:515-517`（invalidTransition）。232 run=coord/runs/M3-AUTOMATION-SLICE-BUDGET-OPEN-1/slicebudget_1（失败轮结构化回执在卷，g1_r1/g1_r3/g2_r1-r3 皆有 admission receipt）。

## 目标

1. **A1 诊断+修复（先诊断后修，断点成文）**：以 slicebudget_1 g1_r1 的实际回执形态为 fixture 复现 G6/G8 fail，钉死精确断点——候选 A：`usableTargetTrackID` 对观察 summary 的形状要求（status/target_ref 形态）与实际 CCB 观察回执不匹配；候选 B：frontier 候选的 track_ids 覆盖与提案目标轨（1010）不对账。断点+修复+单测（用真实回执形状做往返，不得为过门改测试期望）。**红线：G6/G8 的 fail-closed 语义不弱化——修的是接线/对账，不是删检查**。
2. **A2 family 断线修复**：勘察 `capabilityboundary.Resolve` 的 `Input.Family` 来源链，从提案（音量自动化→volume 族）正确解析填充；family 有值后 unavailable_reasons 三连锁应收敛为真实语义（有同族候选则给候选，无则如实 unavailable）。单测：提案→family 解析往返。
3. **B taskstate 幂等重入**：同型终态重复 settle（capability_blocked→capability_blocked）改为幂等 no-op（成功返回，不产生新转移），**非同型的转移表语义零改动**；单测：重复 settle 往返+异型转移仍拒绝。
4. **重验腿（本卡验收门，229 原口径第五承）**：组 1「这段让它越来越突出一点」——**组 1 no_route 消失+set_points 确认链（确认卡→确认→写入→get_curve 读回）首次端到端全绿**；组 2/3 按 220 口径跑满。232 的仪器面（等待窗/确认卡驱动/in_progress_timeout）已就位，判定逻辑零改动。

## 文件域

`agent/internal/agentloop/`（free_state_gate.go/free_state_reasoning.go）+`agent/internal/capabilityboundary/`+`agent/internal/taskstate/`+`agent/internal/chat/free_state_reasoning_loop.go`（审计表同步）+测试。scripts 零改动。越域即停上交。

## 安全面（硬约束）

- G6/G8 fail-closed 语义、治理门、每轮一 mutation 限速、MaxClosureRounds、223 stamp、232 预算 floor 零改动；
- taskstate 仅同型幂等重入放行，转移表其余语义零回退；
- §11：taskstate 若涉持久化格式，旧工件缺省语义+往返测试。

## 停止条件

- 诊断发现 G6/G8 语义要求 automation 族结构上无法提供的东西（设计意图如此）→锚点+两案上交设计裁定（不得强改过门）；
- B 修复与状态机不变量冲突→两案上交；
- 修复后组 1 证据齐（G1-G8 全过+确认卡弹出）但确认驱动/写入/读回任一环失败→write_fail 如实锚点上交；
- 全修后组 1 仍 capability_blocked/no_route 三连→**模型能力面终局上交**（五卡证据链+本轮全量工件，转模型/指引设计讨论，不再折卡）；
- LLM 面栈不可用→环境中断如实记录。

## 领取

（待领取：记录 HEAD/status/diff --stat；真栈错峰登记 PC-RUNTIME-STACK.md；§8 计费披露）
