# JOURNEY-3 N=3 轮跨轮汇总（2026-10-07，PC 执行侧）

- 卡：JOURNEY-3（自由态 NL 旅程回合，概率口径 N=3 / 成功=至少 1 轮走通提案→确认→应用→A/B 卡挂出 / 止损=同代码版本连续 2 轮同形失败）
- 场景：`dev_agent_smoke.ps1 -Scenario journey_free_state_nl -StartKernel`（每轮一次调用=独立泊位+独立 fixture；话术固定 B64，话术原文见各轮 run 目录 journey_nl_prompt.txt）
- 被测 HEAD：9485ccd9（工作树，仅 scripts/dev_agent_smoke.ps1 改动）+ nudge/分类器两轮驱动面修订（见下"驱动版本"）
- LLM 面：真实 LLM（agent 生产链路，非 mock）

## 结果总览

| 轮 | run 目录 | 驱动版本 | exit | 确定性子面 | NL 链形态 | 分类 | 提案卡 | hop | 应用 | A/B 卡 |
|----|----------|---------|------|-----------|----------|------|--------|-----|------|--------|
| R1 | 20261007_194840 | v1 | 0 | 全绿 | trajectory 进入（intent.framed→hypothesis.proposed→observation.recorded）后停在续轮会话边界（goal=waiting_continue，continuations 全 completed，无卡） | chain_stall（驱动缺陷：无 nudge 机械） | 无 | 未驱动 | 无 | 无 |
| R2 | 20261007_200603 | v2(+nudge) | 0 | 全绿 | capacity_assessment + 2×ccb.observation_request（人声 1012/Bass 1007）+ 7 slices → goal completed | 脚本分类 other_terminal:limit_reached；**修正读法=终态无治理面提案（no_candidate_found 族）** | 无 | 未驱动 | 无 | 无 |
| R3 | 20261007_201400 | v2(+分类器 v3) | 0 | 全绿 | trajectory.turn + ccb_observation_catalog + 2×ccb.observation_request → goal completed；**终答文本交付了完整有界实验提案（约 350Hz、-3.0dB、Q≈1.0 主唱削减+执行后 A/B 试听指示）** | no_candidate_found（治理面上无候选） | 无 | 未驱动 | 无 | 无 |

- **成功条件：0/3，未达成**（无任何一轮挂出治理面提案卡，确认 hop 无从驱动）。
- **止损线：R2+R3 同形（终态无治理面提案）×2 连续（同代码版本 v2）→ 触发，停手上交。** R1 的 chain_stall 属驱动面缺陷（v1 无 nudge 机械），修订后不计入 v2 序列。
- 失败分类计数：chain_stall=1（R1，驱动缺陷）；终态无治理面提案=2（R2/R3）；no_candidate_found（显式 stop_reason 族）=0；模型纯文本（零链活动的纯文本轮）=0；环境中断=0。
- exit 0 门（确定性子面）：3/3 正确——fixture DAD 烘焙/工程打开/权限授予+输入链探针/栈健康全绿；hop 门=not_driven_no_card（无卡时不驱动不误炸，符合设计）。

## 驱动版本（执行侧脚本自修，均在 scripts 域内）

- v1：单轮 chat + settle。R1 暴露缺陷——自由态链在 trajectory 轮后停在**续轮会话边界**（journey1 证据工具当时用固定 nudge 驱动同一面）。
- v2：LEG 3 重写为有界 nudge 驱动（固定话术"继续执行"，≤3 次；journey1_demo_journey_smoke.ps1 先例）；边界判定=goal waiting_continue + 无 active continuation + 无 pending 卡。
- v3（分类器）：链有活动（trajectory.* 事件或 item.completed command_name）而终态无卡 → no_candidate_found；零链活动+有回复 → model_pure_text。

## 关键取证：提案以文本交付、治理面未挂出（R3）

R3 事件流 seq10（turn.completed body，服务端拥有的事件面）：

> 我准备对主唱轨做一次有界实验：在约 350 Hz（低中频）做一个约 -3.0 dB、Q≈1.0 的削减，用于缓解主唱低中频堆积，让它和伴奏的分离更清楚。执行后请直接 A/B 试听……

即：模型形成了完整的有界实验提案意图，但它只出现在回复文本里——`improvement_proposal_confirmation` 卡 / pending face / stop_reason 家族（improvement_proposal_*）/ mix_tick 面全部未挂出，goal 以 completed 结束。R2 同形（链跑完观察后终态 completed，事件流无治理面挂出）。**按卡面纪律（绝不断言回复文本语义），该文本仅作失败时间线诊断证据记录，不作为任何断言依据。**

锚点（供决策侧处置参考，本卡未改 agent 代码）：
- 提案挂出面：`agent/internal/chat/improvement_proposal_workflow.go`（挂出条件=自由态 loop 提交结构化 improvement_proposal；FS7 由 `free_state_reasoning_loop.go` 推进，`free_state_improvement_proposal_missing` 即 needs_experiment 无结构化提案时的 stop_reason）
- 本轮观测：模型终答走会话文本而非结构化提案提交——与 OPT-IMPL-2 回执诚实边界的同族现象（agentloop 腿未注入 prompt 指令，自然达标与否依赖模型分支）
- full access 自动授权链（B6③，improvement_proposal_workflow.go:476-486）：native domain 提案确认后会直接执行并挂 audition；但前提仍是结构化提案先挂出/被路由

## 工件索引

- 每轮：`coord/runs/JOURNEY-3/<时间戳>/`（journey_nl_summary.json=轮报告含 timeline；journey_nl_chat_*.json、journey_nl_events*.json、journey_nl_runtime_status_final.json、fixture/open/authority 工件、head.txt、git_status.txt）
- 控制台日志：`coord/runs/JOURNEY-3_round{1,2,3}_console.log`（另 round1 首次调用为调用侧路径失误瞬败，无栈启动，不计轮次）
- 机器可读汇总：`summary_N3.json`
