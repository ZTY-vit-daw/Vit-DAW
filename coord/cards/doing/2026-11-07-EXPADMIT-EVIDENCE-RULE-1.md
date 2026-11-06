# EXPADMIT-EVIDENCE-RULE-1：D1-S1 实验准入门证据引用规则修正+拒绝路径 park 化

- 发卡：GLM 主管决策流 / 2026-11-07 凌晨（依据=[262 verifyrun1 取证](../runs/MIXTICK-CONFIRM-CONTRACT-1/verifyrun1/evidence.md)——新形态三选项裁定②+①组合；262 终验转卡）
- 派发确认：已确认（主管发卡，自主批次）；验收负责人：GLM 主管决策流
- 池序 269；目标仓库=主仓 agent（free_state 准入链）；P1 / 0.5-1 天 / 依赖=262 工件在库；L1-L2 / PC；验证腿需真栈一次（mix_single_tick 复现场景）

## 背景（262 verifyrun1 钉死）

模型完全自主产出有效提案（proposal_valid=true，evidence_refs 引用同目标更早观察 obs_154552 basic_energy+seed），被准入门拒绝且**终局化**：门绑定 `loop.LatestObservation`=obs_154607 要求必含其 ID（`agent/internal/chat/free_state_experiment_runtime.go:176-179`）→`agent/internal/agentloop/free_state_reasoning_loop.go:1684-1693` 改写 blocked/`free_state_experiment_admission_invalid`→响应面终局 done→`turn_close_guard.go:56` 孤儿收口，确认卡未建。三因素叠加：模型引用选择（合理：basic_energy 对电平提案声学合理）×门最新绑定规则（把"新鲜"过度具体化为"最新"）×拒绝即终局。

## 目标

1. **②门校验放宽**（主修）：evidence_refs 校验从「必含 LatestObservation ID」放宽为「含**任一同目标的新鲜观察**」（新鲜=G7 revision-bound 既有语义，同 revision 内不区分先后）；LatestObservation 绑定语义的原始目的（证据新鲜度）不得削弱——同 revision 内更早观察与最新观察等效；
2. **①拒绝路径 park 化**（防御）：准入门拒绝不再终局化——改走 terminalAdjudicationParking 族既有先例机制（回喂或挂「方案乙 park」确认面），孤儿收口路径保留为最后兜底；
3. 配对单测：①同 revision 早期观察引用准入通过 ②跨 revision 陈旧引用仍拒（G7 语义不削）③拒绝路径产 park/确认面而非终局 done；
4. 验证腿：mix_single_tick 单 run 复现场景（≈¥1-3，§8 成文）——预期 observe turn 直停确认面（262 原断言目标）。

## 约束

文件域=free_state_experiment_runtime.go+free_state_reasoning_loop.go 拒绝分支+测试；禁动 G7 门本体（free_state_gate.go 的 revision-bound 判据零改动——本卡改的是引用选择规则不是新鲜度规则）；治理红线不变（确认链/零裸写）；256/258 断言面零回退。

## 停止条件

修复发现拒绝终局化是有意设计（有在案裁定）→取证上交转裁定；验证腿断言失败取证不重跑。

## 领取/回执/验收

- 领取（实现腿）：2026-11-07T00:11+08:00 / origin/main=f7204a25 / owner=GLM 主会话 L2-worker 子代理（PC，ZCode） / 分支=port/expadmit-evidence-rule-1（worktree=D:\Vit_DAW_wt_269impl，基线=f7204a25，领取时 status 净空、diff --stat 空） / 领取提交=本提交（仅含本卡状态，fast-forward → main）
- 回执（实现腿，2026-11-07T00:30+08:00，GLM 主会话 L2-worker 子代理，wt_269impl）：**完成，停止条件未触发**（拒绝终局化无在案有意设计裁定——方案乙 park 族先例 FIX-F3-G4-SEMANTICS 即同型收口）。impl commit=914e5c48（分支 port/expadmit-evidence-rule-1 已推，基线 f7204a25）。要点：
  - **②门校验放宽**：`validateD1FreshObservedTarget` 引用绑定从「必含 LatestObservation ID」放宽为「任一 cited ref 经台账行（receipts+available_views，G7 同源 resolution 族=observation_id/receipt_id/行 evidence_refs，镜像 refMatchesLedgerRow）解析到**同目标且与已验证最新观察同 revision** 的行」；新鲜度判据零新造（新 helper freeStateLedgerRowFreshAtRevision 镜像 G7 freeStateReceiptFreshRevisionBound：非 fresh class 拒、revision 相等才过、无 revision fail-closed）；**free_state_gate.go 零改动（git diff 亲核=0 行）**；
  - **①拒绝路径 park 化（选挂确认面）**：1684-1693 拒绝分支不再改写 blocked 终局——拒绝如实落 AdmissionReceipt（boundary=free_state_experiment_admission_invalid，较旧路径多记一笔——旧路径把 1427 的 receipt 留在原样）+LastError；决策保持 needs_experiment 形、loop 保持 awaiting_experiment，粘附 TerminalAdjudication（free_state_terminal_adjudication.v1，failed_gate_ids=[D1_experiment_admission_refused] 内容自由标记）→ improvementProposalResponse 出确认卡（full-access 也不自动授权，方案乙族同款）；后续成功准入清 stale latch；turn_close_guard 孤儿收口保留最后兜底（确认卡在=活 owner 不触发）。**选型理由**：回喂需重驱动 LLM 轮次+再循环风险，且 262 验证腿预期「直停确认面」；挂确认面确定性、零额外计费、与 terminalAdjudicationParking 先例完全同族，且是文件域内唯一不需改 improvement_proposal_workflow.go 的路径（确认面/防全访问绕过/披露挂载全部复用既有读取）；
  - **配对单测**：新增 free_state_admission_refusal_park_test.go 7 例（同 revision 早期引用过=262 形态/跨 revision 陈旧仍拒/G7 新鲜度不削四态：异目标同 rev、stale class、无 revision、不可解析/最新引用仍过/拒绝产 park 非终局/park 确认卡 full-access 下仍挂+interaction 落库/后续成功准入清 latch）+既有 1 例断言口径更新（TestD1S1AdmissionRequiresTargetObservationID：拒绝语义保留，错误文案随②变化）；
  - **回归**：go build ./... exit 0；go test ./internal/agentloop -count=1 **ok**（4.7s）；go test ./internal/chat -count=1 **ok**（137.4s）——258 断言面（agentloop G7/disclosure+chat Revalidate 族）零回退，256 断言面=scripts 面（零栈未触）；
  - 验证腿就绪：待主管栈调度 mix_single_tick 单 run（§8 口径沿用 262 RUNBOOK；预期 observe turn 直停确认面——262 形态经②直接准入，残余拒绝经①出确认卡而非终局 done）。
- **实现腿验收（2026-11-07 凌晨，主管）：pass（轻验，终验待验证腿）**——②放宽实现=「任一 cited ref 解析到同目标且同 revision」resolution+新鲜度两族镜像 G7 既有判据零新造（设计正确）；①park 化选**挂确认面不回喂**（理由充分：回喂耗预算+与 262 预期终态矛盾+park 族先例同族）；**G7 门本体 free_state_gate.go 相对领取基线 0 行 diff 主管亲证**；7 例配对单测+agentloop/chat 全绿+258 断言面零回退；主管交叉复跑 agentloop ok。验证腿进栈队列（276 段 3b 后）。
- 回执·验证腿（2026-11-07 00:51–01:00+08:00，GLM 主会话 L2-worker 子代理验证会话，wt_269impl）：**单 run `20261107_005156` PS1 exit 0——§8 预写①PASS 兑现，单 run 无重跑兑现**。被测=wt_269impl @ port/expadmit-evidence-rule-1 3ce30ce7（impl 914e5c48；origin/main 9e5af73c 合入**无冲突**——main 域内前进=goalrunner_chat.go 披露白名单[276 补丁]+server_test.go，与本卡 4 文件零重叠；合并后 build exit 0+agentloop[4.6s]/chat[115.4s] 全绿）；agent=现场构建 SHA256 2092F8C8…F1BD79；kernel=wt_228 F9E3DA0A 只读复用；引擎=deepseek-flash 无注入。**判定面**：observe turn 直停确认面（waiting_confirmation/improvement_proposal_confirmation_required，83.7s/5 steps，无排干无切片零弹回）+确认卡落库+双跳链全绿（提案确认→工具确认→已应用回读 0→-1dB→d1_post_action_evaluation_required）——262 原断言目标达成；**到达路径=②直接准入亲证**（loop experiment 非 nil rounds=1；①park 未触发=terminal_adjudication None+log 零 park 行；诚实注记：本随机分支引用面宽[obs_165235+obs_165307+seed]含当时最新观察，②放宽非本 run 决定性分支——262 精确形态复现依赖模型只引早期观察的随机分支，该面由 7 例配对单测钉死）；262 失败链 3/4 环（blocked 改写→终局 done→孤儿收口）零踪迹（goal=waiting_continue、continuation.owed 诚实 withhold）。计费 17 调用 flash in 255,214+cache 56,704/out 23,509 tok ≈¥0.25-0.5（预算 ≈¥1-3 内）。占栈协议全履约（净空 00:50:14→登记 24f7280a→≤60s 复核 00:51:45→单 run→TEARDOWN_ALL_ZERO 亲核 01:00:14[六口 LISTEN=0+UDP 0+四类进程+python 0]→释放登记本提交）。工件=port/expadmit-evidence-rule-1（324d295f）coord/runs/EXPADMIT-EVIDENCE-RULE-1/verifyrun1/（RUNBOOK+console+runtime state/status+agent log+telemetry+evidence 七件）。预期终验信息：262 G6 契约链在 269 修复面完整走通（observe 直停提案面→双跳→D1 终态），主管可终验。
