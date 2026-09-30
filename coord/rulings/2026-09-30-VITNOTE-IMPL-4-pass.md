# Ruling：VITNOTE-IMPL-4 — pass（2026-09-30 决策会话）

- 实现：75e57a63@port/vitnote-impl-4 → 合并 main 4583d31d（cherry-pick）
- 亲核项：
  1. diff 对应：4 文件 +1172（writelease.go 305 / writelease_test.go 395 / coordinator.go +25 / dev_agent_smoke.ps1 +448），与回执一致。
  2. **我复跑**：go test ./internal/executionruntime/ -count=1 于合并态 main——**ok**。
  3. 租约语义抽查（对照设计 §7.3 五要素）：DefaultProjectWriteLeaseTTL=120s 具名常量 ✓、等待者 FIFO 排队不拒绝 ✓、排队中 ctx 取消退出并归还已授予租约 ✓、Release 幂等 ✓、跨工程不互斥 ✓；事件工件面（VIT_WRITE_LEASE_EVENTS_PATH JSONL 纳秒时间戳）支撑烟测断言。
  4. 接口冻结遵守：Coordinator 以 nil 兼容字段+New 启用+defer 释放钩子式接入，既有签名语义未改；Reconcile（恢复路径同为变异面）纳入租约域——卡面"全部能力执行"语义内聚，申报在先，采信。
  5. 烟测工件亲读：write_lease_20260930_101123/summary.json——first_segment executed_verified 3394ms、second wait_started→authorized_execution_blocked（stale_project_cut）5ms、同纳秒交接零重叠；分层安全语义（租约串行化段+段内 CAS 拦截）实证成立，exit 0。
  6. 前提取证采信：领取时已锚六项能力执行全部经 Coordinator 单点（10 处调用锚），停止条件未触发。
- flake 处置采信（AGENTS §11 口径）：chat TestWorkspaceSwitchSettlesInFlightChainExplicitly（TempDir 清理竞态，与已知 TestProcessorCertificationStart* 同族但新名）+harness TestMixboardSnapshotConcurrentDualWrite（5s 截止超时）——隔离复跑 3/3 绿+失败路径零能力执行关联+主基线全量绿，归因负载敏感 flake 成立；**首次记录，重复出现即开修复卡**。race detector 不可用（无 gcc）申报采信，并发断言以 channel/mutex 时序化+真栈纳秒时间戳补位。
- 执行段时长申报：3394ms < 30s 阈值；等待者 3.07s 在 v1 接受面（卡面风险点条款）。
- **两项脚本体系问题上交（决策侧处置，不在本卡域）**：
  1. `-StartUI` Godot start-page 自 spawn VitAgent 与脚本 `-RestartAgent` 竞态双绑 UDP 4445 双亡——脚本体系既有竞态，待开脚本体系卡；
  2. 并行执行会话真栈烟测互抢固定端口栈实录（本卡烟测两次被 webui 域 E2E 栈阻断）——AGENTS §9 运行栈所有权规则在派发侧的执行缺口：**"并行域"判定必须含真栈需求，两卡都需真栈烟测时应错峰派发**（本日 FIX×IMPL-4 并行派发由决策侧把关失误，执行侧实测两轮阻断后错峰完成）。此教训记入当日 gate 报告，规则不新立（§9 已在），执行加严。
