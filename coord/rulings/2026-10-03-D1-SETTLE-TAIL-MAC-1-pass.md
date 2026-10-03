# Ruling：D1-SETTLE-TAIL-MAC-1 — pass；D1-EQ-READBACK-550A-1 终验闭合（conditional 转正）（2026-10-03 决策会话）

## 验收核验

1. **取证亲读**（coord/runs/D1-SETTLE-TAIL-MAC-1/FORENSICS.md）：事件链十时点全部可回指（round_1 报告+kernel 摘录+durable 状态+环形日志首尾行）；三假设分诊含否定证据（①饥饿：调度全程 250ms/拍+地板已授；③超时重试：无重试在等）——**假设②（等待条件永真）成立**：mix-tick 可答 park 先落（10:26:09.7）钳住 goal→apply（10:26:12）后两条武装边界（applied 边界 goal==completed 门/recordGoalResult completed-over-owed 门）均不可达，调度器按设计握 park 十分钟。"Mac 时序敏感"重定性为**模型提案流方差**（park 先于 apply 的窗口期交错），嫌疑不成立；**轮2 不同根**（准入门 G6/G8 拒案=设计内 fail-closed）定性正确。日志能见度受损（AUTH-RESTORE 刷屏代价）如实声明，取证改走持久化状态+kernel 日志合规。
2. **修复 diff 亲核**（1662943）：applied 边界门扩展——`StatusCompleted` → 加 `shouldResumeGoalFromStatus`（parked-resumable 全集），running 仍拒、completed 形态维持原修正、parked 不动用户可见 park 面（不降级 waiting_confirmation）；arm 核心槽位移——不可答 legacy shell 按 recalibration 配方取消（防 restore 回灌）+answerable park 存活（按 interaction_id 退役与槽无关）+runnable/裸占用仍拒；exactly-once settle-marker 闩守卫沿用。红先行三钉（OverAnswerablePark/RefusedOverRunnableArmOccupant/DisplacesLegacyShellOccupant）+189 行测试亲在读。未放宽等待未吞错。文件域=goalrunner_chat.go+测试，卡面域内。
3. **与 PC 同域修复重叠核对**：PC JUDGMENT-SETTLE-STALL-1（334d8f4：audition_events/free_state_reasoning_loop/judgment_park_continuation/user_judgment）与 SETTLE-DELIVER-1（d8b7e2d：audio_closure_controller/audition_events）**零文件交集**；语义面正交（本卡=settle 检查点武装边界，PC=判定结算后续链/驻留边界）；cherry-pick 到含双修复的当前 main 零冲突，全量复跑绿（下条）——并存无冲突成立。
4. **我方复跑**：cherry-pick 1662943 于 origin/main=3c9e4f1 干净落地；go build exit 0；**全量 87 包 0 FAIL**（chat 62s 含三钉）；blob gofmt 2/2。
5. **终验冒测亲读**（coord/runs/D1-SETTLE-TAIL-MAC-1/d1_550a_20261003_180509/）：final_exit=0（round1 exit=1=无关模型方差断点，round2 exit=0）；被测=修复件 1662943 干净树（run_meta 亲核）；round2 报告独立复核——validation pass、static_eq@1017 rev6→7、readback=-2 verified=True（我方直接读 interventions[0]：technical_application=applied、双通道 requested==actual==0.400000005960464）、materiality/target_response 落地非 null、fs_settle_terminal_park **judgment_park=true**（closure fs7_improvement_proposal，泊位形态正确）、两条 durable settle 检查点 completed。SMOKE_RUN.md 诚实边界（特定交错由单测钉住、不冒认端到端走过该交错）+环境类失败五例留痕+A/B 内核算证（二进制无罪）+机器态复原记录——全部合规。
6. 泊位声明（自动跑完泊人工判定边界、无机器越权收口）与端测边界声明（agent HTTP 面驱动，渲染面未涉、无 webui 改动）采信。

## 裁定

**pass**——取证报告+修复 diff+chat 包全绿+全量 0 FAIL+d1 冒测 exit-0（工件可回指）+泊位声明，验收标准全项达成，无条件通过。实现随本裁定 cherry-pick 合 main。

## D1-EQ-READBACK-550A-1 终验闭合（conditional 转正）

2026-10-02 conditional ruling 的终验条件（"D1-SETTLE-TAIL-MAC-1 落地后 550A 锁定场景复跑 exit-0"）由本卡 run `d1_550a_20261003_180509` 达成（readback verified on locked 550A scenario, exit-0）——**D1-EQ-READBACK-550A-1 conditional 转正为 pass（全条件闭合）**。

## 遗留移交

- 执行 worktree ~/Documents/Vit-DAW-d1settle-mac1 由决策侧本次验收后清理（port 分支已推远端）。
- 原始工件目录 ~/Documents/vit-d1eq550a-artifacts/ 保留（回指锚）。
- AUTH-RESTORE-LOGSPAM-1（池序 12，P3）仍在池——本卡取证再次实证其取证能见度代价（环形日志被刷出窗外），建议下轮发卡优先。
- SMOKE_RUN.md 教训移交（代理 env 污染整栈/WaveShell1 探测余量）已在案，后续 Mac 冒测卡提示词注意。

## PC 接收复核 concur（2026-10-03 PC 决策会话，独立验证后并入本裁定）

PC 侧对同一卡独立完成验收四层（时间窗与 Mac 验收并行，rebase 收敛时合并）：cherry-pick 1662943 至含 PC 同域双修复（334d8f4 判定通道/d8b7e2d 结算投递+所有权门）的当前 main **零冲突**；三新钉 PASS；chat 全包 84.7s PASS；全仓 `go test -count=1` EXIT=0；SMOKE_RUN.md 诚实边界（特定交错由单测钉住、不冒认 run 内发生）亲读合格。结论与 Mac 裁定一致——**pass 维持**；PC 侧重复实现提交（d8ce072c）经 rebase 补丁去重自动跳过，以 Mac 链 cherry-pick（abc92402）为准。
