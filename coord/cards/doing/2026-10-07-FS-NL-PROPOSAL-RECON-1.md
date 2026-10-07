# FS-NL-PROPOSAL-RECON-1：自由态 NL 提案治理面挂出抖动勘察（JOURNEY-3 止损上交产物）

- 池序 17（JOURNEY-3 验收裁定立卡：执行侧 N=3 成功 0/3（R2/R3 同形终态"无治理面提案"）；**决策侧复跑 R4（20261007_202322）完整走通严格链**（提案→确认 hop→应用→A/B 卡，success_strict/policy 双 True）——缺口定性从"阻塞"降级为"抖动"（聚合成功率 1/4），P2）；目标仓库=D:\Vit_DAW（PC 执行侧）
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无（证据已在案，含 R4 成功样本）
- 模型分级：L1 / flash 可接（取证卡：工件亲读+锚点核对+一次受控复现）
- **来源证据（不重跑，先读）**：
  1. R3 run `coord/runs/JOURNEY-3/20261007_201400/`（journey_nl_summary.json + chat/events/runtime 工件）：链走完 trajectory intent→observation catalog→2×ccb.observation_request→goal completed；**终态 turn.completed 正文以文本交付完整有界实验提案**（约 350Hz/-3.0dB/Q≈1.0 主唱削减+执行后 A/B 试听指示）而 improvement_proposal 卡/pending face/stop_reason 家族零挂出（11 事件中 audition=0/mix_tick=0/applied_like=0）。
  2. R2 run `coord/runs/JOURNEY-3/20261007_200603/`：同形（capacity assessment+观察后 goal completed 无卡）；R1 chain_stall 系驱动缺陷已修（v2 nudge），不计入本缺口。
  3. 执行侧止损上交锚点：`agent/internal/chat/improvement_proposal_workflow.go`（结构化 improvement_proposal 提交→卡/自动授权路由的挂出条件）+`agent/internal/chat/free_state_reasoning_loop.go`（FS7 推进与 `free_state_improvement_proposal_missing` 分支=needs_experiment 无模型结构化提案时的路径）。
- 待证假设（勘察前不预判）：
  - H1 入口协议缺口：该 NL 话术走 trajectory/chat 入口的终轮 prompt 未含（或未强约束）"needs_experiment+improvement_proposal.v1 结构化返回"指令——模型合规地给了终答文本；
  - H2 协议修复面漏拦：messageLoop 的 model protocol repair/终轮门对"散文式提案终答"无兜底（improvement_proposal_missing 分支未被触发的机制原因）；
  - H3 模型概率行为：prompt 面完整、修复面在位，纯模型分支（→修 prompt 措辞或加一次 nudge 重试即可）。**R4 证据（20261007_202322，同代码同话术走通全链）使 H3 先验显著升高——勘察重点转为"抖动率是否可接受/最低成本加固"**。
- 目标：
  1. **工件取证**：R3 的 turn.completed 原文+该轮请求的实际 prompt 面（agent log/上下文快照工件）逐层读——终轮模型看到什么指令、返回什么 JSON 形态、loop 如何解析为 final。
  2. **锚点核对**：两锚点文件的挂出条件与 R3 实际路径对齐——结构化提交在链中的触发点是哪一步、为什么没走到。
  3. **受控复现一轮**（可选，若 1/2 已定因则跳过）：`-Scenario journey_free_state_nl -StartKernel` 单轮（决策侧已复跑过的命令），若复现同形则取证结论升级。
  4. 产出：根因假设裁定（H1/H2/H3 采信序）+ 修复建议（prompt 注入点/修复面兜底/话术调整 三选一或组合）+ 是否需要修复卡的明确建议——**零生产代码改动**（本卡纯取证）。
- 文件域：只读 agent/chat 域+coord/runs 工件；写入仅报告（coord/runs/FS-NL-PROPOSAL-RECON-1/）。
- 验收标准：三假设逐条有证据支持或排除（引用工件行/代码锚点）；根因裁定+修复建议落报告；决策侧复核。
- 停止条件：工件不足以下结论（如终轮 prompt 面未留痕）→ 缺口清单上交（此时修复卡按现有证据先立 prompt 注入腿）。
- 领取：（2026-10-07 22:5x 前后 / origin/main=a42c87e1 / main，主树单流；真栈泊位 5555/7878/5556，领取时未核监听，跑前 netstat 申报）
- 回执：（报告路径 / 假设裁定 / 修复建议 / 是否需修复卡）
- 验收：（裁定文件 / 验收 commit）
