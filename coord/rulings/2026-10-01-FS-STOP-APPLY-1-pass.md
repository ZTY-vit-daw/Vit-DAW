# Ruling：FS-STOP-APPLY-1 — pass（2026-10-01 决策会话）

- 实现：28551ef2+6a338479+7c7a3370@port/fs-stop-apply-1 → 合并 main 15a10a5b/5b1805ba/dad07e16（cherry-pick）；卡片状态变更 8bd10faf 已由执行侧合规直推 main。
- 亲核项：
  1. **取证先行达标**（卡面目标 1）：Part A 执行点实锚=improvement_proposal_workflow.go:479-485（完全访问决策返回后直连 executePendingMixTickCandidate，在 runner 检查点体系之外——本轮询无检查点的盲区正是 M1 18:34 案发位）；停止标志可查询面=harness.RuntimeStatus→Goal.StopRequested（复用 server.go:4112 既有惯用式）。
  2. **diff 直读**（3 文件 +479，其中测试 329）：Part A——awaitingExperiment 分支停止闸门（goalrunner_chat.go:490-500），决策返回与应用之间重读停止闩，命中即不应用+诚实 skip（user_stop_pending_intervention_skipped+用户可读话术+mutation_performed=false），且**立即 finalizeStoppedTurn（连带 Part B 结算）而非拖到下一个 continuation**；Part B——settleStoppedTurnClosure（turn_control.go）：任务 EventOwnerTurnClosed 合法关闭→closure 经 audioClosureSettleFromResult 诚实结算（兜底 StopCancelled，**禁伪造 settle-success**）→owner 释放；own-closure 判定（state.GoalID≠goalID 不碰）。两处锚点日志（request received/settled）补齐取证盲区。
  3. **顺带实证修复采信**：TransitionTask StateCancelled 分支会把已停 goal 改写回 cancelled（task_runtime.go:361，M1 实测复现）——MarkGoalStopped 移到 stopFreeStateExperiment 之后，停止语义最后落地；方向正确且改动最小。
  4. **语义边界钉死**：stopped≠parked 保持（TestStoppedExperimentIsNotAdoptedByContinuation——FS-PARK adoption 语义零回退）；FS-PARK 全钉绿（judgment_park_continuation_test.go 10 函数）。
  5. **我方独立复跑**：go build + go test ./... -count=1 **87 包 0 FAIL**；三关键钉定向 PASS（skip-intervention/ownership-release/stopped-not-adopted）。
  6. **烟测工件亲读**（§9 纪律全达标）：命令+exit 0+status=stop_semantics_probe_pass；被测二进制双 SHA256 指纹+零 C++ 改动核对；一次环境失败如实隔离（bash 引号路径，非功能性）；探针五证据=parked goal→诚实 stopped、closure **fs9_terminal+cancelled（非 satisfied）**、owner **settled**、停止后 revision 不动（零新干预）、新输入开新 goal/run+回复落地（**M1 18:35 楔死面 green**）；锚点行 `[turn.stop] request received` 在 log。
  7. **泊位声明合规**：栈已拆除（三进程 pid+端口核验在案）。
- 上交接管（决策侧处置）：`requestFreeStateTurnStop` 置 loop.Status="stopping" 后全仓无读者（死代码候选）——登记清理候选，随下次 turn_control 域卡顺带移除或正名，不单独立卡。
- 收尾项：用户手测复验（取消即时生效+第二句正常+顺序正常）——**与 M1 第四轮同场**（新二进制+新 dist 拉栈后：观察→park→取消→新输入全链）。
