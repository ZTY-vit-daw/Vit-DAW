# FS-STOP-APPLY-1：停止语义双缺陷——停止挂起期间干预仍被应用 + user_stop 在 park 态楔死会话（M1 复验实测，P1）

- 池序 22（P1 用户面）；目标仓库=D:\Vit_DAW（PC 执行侧，agent Go 域）；来源=M1 复验第三轮活栈取证（[runs/M1-RETEST-20261001/FORENSIC-NOTE.md](../../runs/M1-RETEST-20261001/FORENSIC-NOTE.md)+evidence 五件，勿重取证）
- 优先级 / 预估 / 依赖：P1 / 取证 0.5 天+修复 1 天 / 无（与 FS-PARK-TURNFAIL-1 已合入修复无冲突——本案是其姊妹缺口：那是 park+新输入，这是 park+**取消后**新输入+停止时序）
- 模型分级：L2 / GLM 首选（停止时序语义+closure 状态机所有权面；FS-PARK 同域先例）
- 已核实事实（2026-10-01 18:30-18:36 活栈，会话 webui_mupe9yh1，栈二进制=main 修复态）：
  1. **Part A 症状（停止挂起期间干预仍被应用）**：用户在 CCB 观察运行中（18:32:18–18:33:58 空窗内）按取消；取消=handleTurnStop→RequestGoalStop 异步标记（turn_control.go:326-336）；在途轮继续「LLM 决策→18:34:04 EQ 应用→18:34:06 试听准备→park 建立」→18:34:07 停止才落地（trajectory.turn.stopped, summary=user_stop, checkpoint c_20261001T103407）。**用户已按停止，干预仍被写入工程**。
  2. **Part B 症状（取消后第二句立即失败+会话楔死）**：goal_6b07c186 于 18:35:32 turn.started→**同秒 turn.failed**：`stop_reason=audio_closure_controller_failure, error="conversation is already owned by minimal_audio_closure controller audio_closure_997396f72e46902c"`。机制链全锚点：finalizeStoppedTurn（turn_control.go:246-271）置 Experiment=Stopped（freeStateJudgmentBoundary 因 StatusStopped 返 false——free_state_reasoning_loop.go:1276-1278——FS-PARK Part A 采纳正确排除）**但不触碰 audioclosure 状态机**；closure 停 fs7 非终态→settleAudioClosureOwner 仅认 Terminal（audio_closure_controller.go:1390-1392）→controllerOwners 不释放→新 goal 的 ensureAudioClosureOwner（:236-240）遇异 ID 活跃 owner 即拒。turn_close_guard 孤儿结算路径（turn_close_guard.go:42-68→206-248）被「任务 human_judgment_required+判定交互在飞」前置条件挡住。
  3. 取消按下时刻无日志（handleTurnStop 零日志——取证盲区，本卡补锚点）；用户证词在案（CCB 观察运行中按下）。
- 目标：
  1. **Part A 修复**：停止请求挂起期间不得应用新干预——消息循环在「LLM 决策已返回、干预尚未执行」之间检查 goal 停止标记；命中则跳过应用并诚实记录（skip 原因=user_stop pending；不回滚已应用部分——有界可回滚语义不变，但本轮剩余动作不再发生）。取证先行：领取后实锚决策后应用的执行点与停止标记的可查询面（harness.RequestGoalStop 的标志位读取路径）。
  2. **Part B 修复**：user_stop 收尾释放会话——finalizeStoppedTurn（或其调用链）对「closure 非终态+Experiment 已 Stopped」的会话：任务语义走合法关闭迁移（EventOwnerTurnClosed 族）、closure 以诚实停因结算（StopInsufficientEvidence 类既有停因，**不得伪造成 settle 成功**）、settleAudioClosureOwner 释放所有权。修复后：同会话新输入正常开新 goal（复刻本案第二句→应正常跑，不再是 audio_closure_controller_failure）。**注意边界**：不复活 adoption（stopped≠parked，FS-PARK 语义保持）；显式 judgment POST 通道对已停轮的行为如受影响须如实申报。
  3. **锚点补齐**：handleTurnStop 记录请求到达（conversation/goal/时刻/reason 一行 INFO）——消除取证盲区。
  4. **回归**：复刻三场景——a) 观察运行中取消→干预不应用+轮诚实停止；b) park 态取消→新输入正常新 goal（修复前红=本案）；c) 正常 park+新输入（FS-PARK 既有语义零回退，其六钉必须仍绿）。真栈烟测沿 run_free_state_d1_smoke.ps1 体系扩场景（-StopPendingProbe/-StopOwnershipProbe 或合并一个探针），exit 0 方算交付（AGENTS §5）。
- 文件域：agent/internal/chat/（turn_control.go/audio_closure_controller.go/turn_close_guard.go 一带+测试）+ agent/internal/agentloop/（停止标记检查点——实锚后申报）——FS-PARK 同域先例可参照。
- 约束：真栈 §9 所有权（与其他卡烟测错峰）；泊位声明必附；FS-PARK 六钉零回退是硬门槛。
- 验收标准：三场景红绿+全量 0 FAIL+FS-PARK 六钉绿+真栈烟测 exit 0+锚点清单。
- 停止条件：停止标记在 agentloop 无可查询面（需改 harness 契约）或 closure 结算与既有 FS9/settle 语义冲突超出 chat 域 → 实证上交定扩域。
- 领取：2026-10-01 19:05 / origin/main=701e2c27 / port/fs-stop-apply-1（worktree D:/Vit_DAW_worktrees/fs-stop-apply-1）
- 回执：
  - 实现 commit：port/fs-stop-apply-1 @ **28551ef2**（修复）+ **6a338479**（StopSemanticsProbe 烟测探针）+ **7c7a3370**（烟测工件入库）；分支已推 origin，等验收 cherry-pick
  - 取证补充（实锚申报，卡面「取证先行」项）：
    - Part A 执行点：improvement_proposal_workflow.go:479-485——完全访问模式下 LLM 决策（改善提案）返回后直接 `executePendingMixTickCandidate` 应用，整链在 runner checkpoint 体系之外；runner 检查点只有 before_plan/before_tool/after_tool（含 message_loop 变体），模型调用返回后无检查点
    - 停止标志可查询面：`harness.RuntimeStatus(goalID)` → `Goal.StopRequested`（RequestStop 置位，runtime.go:341）；chat 层既有惯用式 server.go:4112 `StatusStopped || StopRequested` 本卡复用
    - 顺带实锚（未修，越域不扩）：turn_control.go `requestFreeStateTurnStop` 置 `loop.Status="stopping"` 后**无任何读者**（全仓唯一写入点）——死代码候选，留决策侧处置
  - 修复面：goalrunner_chat.go awaitingExperiment 分支停止闸门（Part A）+ turn_control.go settleStoppedTurnClosure（Part B：任务 EventOwnerTurnClosed 合法关闭→closure 经 audioClosureSettleFromResult 诚实结算→settleAudioClosureOwner 释放）+ **顺带实证修复**：TransitionTask 的 StateCancelled 分支会把已停 goal 改写回 cancelled（task_runtime.go:361，M1 实测复现）——MarkGoalStopped 移到 stopFreeStateExperiment 之后，停止语义最后落地；stopped≠parked 保持（Experiment Stopped 永不被 continuation 采纳，TestStoppedExperimentIsNotAdoptedByContinuation 钉死）
  - 三场景红绿（agent/internal/chat/fs_stop_apply_test.go，四测全绿；红验证=闸门+结算中性化后精确翻红）：
    - a) 观察运行中取消→干预不应用+轮诚实停止：GREEN（TestStopPendingDecisionSkipsInterventionApplication：skip_reason=user_stop_pending、无 pending mix tick 落库、closure 结算+owner 释放同边界完成）
    - b) park 态取消→新输入正常新 goal：GREEN（TestUserStopReleasesClosureOwnershipForNextGoal；**修复前红=M1 楔死精确复现**：中性化实测 closure settlement=nil+owner 永_active，新 goal closure 准入即「already owned」）
    - c) 正常 park+新输入零回退：FS-PARK 全钉绿（judgment_park_continuation_test.go 10 函数 + goalrunner 扩展钉）
  - 全量：`go build ./...` ok；`go test ./... -count=1` **87 包 0 FAIL**
  - 真栈烟测：`run_free_state_d1_smoke.ps1 -StopSemanticsProbe` **exit 0**，run 工件=coord/runs/FS-STOP-APPLY-1/smoke-20261001_193304/（报告+agent_last.log+README 含命令/二进制指纹/一次启动环境失败记录）。探针证据：parked goal_53d57021→stop→goal 诚实 stopped、closure **fs9_terminal+reason=cancelled**（非 satisfied）、owner **settled**、停止后 revision 不动、`[turn.stop] request received` 锚点行在 log、新输入开新 goal_935f529d/run_61ca87d44（M1 18:35 楔死面 green）
  - 端测覆盖边界声明（§5）：改动全在 agent Go 域+烟测脚本；无 webui 渲染面改动——渲染面/旅程门槛不适用；真栈烟测覆盖停止语义的服务端面（用户旅程「取消」一环），webui 停止按钮的 UI 呈现不在本卡覆盖内
  - **泊位声明：运行栈已拆除**（VitApp pid5616 / Godot pid17900 / VitAgent pid25156 已停；7878/5555/5556 空闲；2026-10-01 19:40 核验）
- 验收：（裁定文件 / 验收 commit）
