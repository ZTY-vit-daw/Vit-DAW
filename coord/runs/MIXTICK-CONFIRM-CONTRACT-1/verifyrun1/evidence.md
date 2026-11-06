# verifyrun1 运行证据与判定（2026-11-06 23:45–23:47 +0800）

- run stamp：`20261106_234509`（PS1 启动）；conversation=`mix_single_tick_e2e_20261106_234522`；goal=`goal_18823d5b3c0d6284`；run_id=`run_01df31110fd06c31`
- 被测：worktree D:/Vit_DAW_wt_262impl @ port/mixtick-confirm-contract-1 0b686c80（A patch=72718892 在树）；agent=worktree 现场构建 `agent/bin/VitAgent.exe` SHA256 `B280318D4FFA55D2E6BC8FEB962F6A0A0A7492B16E0E182C2AAFCC017654F5E3`；kernel=wt_228 F9E3DA0A 只读复用；引擎=deepseek-flash（本机默认无注入）
- 结果：**PS1 exit 1**——observe turn stop_reason `got 'done'`（run_mix_single_tick_e2e.ps1:926 断言），后段腿未执行
- 判定：**③ 断言失败/新形态取证上交（单 run 无重跑兑现）**——非 ① 直停提案（响应面未到确认面）、非 ② 弹回族（零去重弹回、零切片、零排干）

## 一、响应面与计费

- observe turn：6 轮 LLM（T1 catalog→T2 project.structure→T3 track:1007 basic_energy→T4 track:1010 basic_energy→T5 track:1010 time_dynamics→T6 提案决策），观察工具 4 请求全 executed，**总时长 59.9s，无 180s 排干、无切片、无 final_gate 弹回**（agent_message_loop_debug.jsonl 本 run 零新增条目＝零 parse/弹回类诊断）。
- 响应：stop_reason=`done`、needs_confirmation=false、reply（285 字，runtime state conversations 亲证）：「整体先看了一眼：Track 1 还有约 6 dB 峰值余量，Track 2 峰值正好顶到 0 dBFS、基本没有余量，而且两条轨在低频到中低频存在重叠候选。建议只把 Track 2 音量小幅下调 0.5 dB，不改插件、不动别的参数；确认后我才执行。」
- 计费（telemetry_run1.jsonl，7 调用全 deepseek-flash）：input 101,603 + cache_read 22,784 / output 8,503 tok（含 semantic_entry 1 调用）≈ **¥0.1-0.2**，预算 ≈¥1-3 内远低于上界。
- 环境注记：agent 全程对 127.0.0.1:8787（VspHub）连接拒绝重试——mix 脚本族不起 vsphub，257 run 同款背景噪声；CCB 观察全部经内核 ZMQ 正常执行，与本失败无关，不计环境中断。

## 二、失败机制（全部在原始工件指印）

1. **模型侧契约达成**：T6 模型输出结构化 needs_experiment + `improvement_proposal.v1`（runtime state `latest_decision` 与 `pending_candidates` 亲证）：target=track:1010、action_kind=track_gain_adjust、delta_db=-0.5、hypothesis/expected_effect/verification_plan 齐备；`admission_receipt.proposal_present=true / proposal_valid=true / proposal_error=""`——**提案本体 schema 有效且在观察回合内一击产出**。
2. **D1-S1 实验准入门拒绝**：提案 evidence_refs=`[obs_154552(track:1010::basic_energy), obs_154526(seed mix.multitrack_relationship)]`；准入校验 `validateD1FreshObservedTarget` 绑定 `loop.LatestObservation`＝`obs_154607(track:1010::track.time_dynamics)`（T5 刚返回的最新目标级观察），要求 evidence_refs 必须包含该 ID（`free_state_experiment_runtime.go:176-179`「D1-S1 evidence_refs must include the target observation ID "obs_20261106T154607_2e9239af78ba"」）。模型引用的是**同一目标的更早一条目标级观察**（basic_energy，恰恰是音量决策最相关证据）而非最新一条 → 准入失败。
3. **blocked 改写→终局**：`free_state_reasoning_loop.go:1684-1693` 准入失败→`loop.Status="blocked"`、LastError=上句、决策被改写 status=blocked/stop_reason=`free_state_experiment_admission_invalid`；本轮 chat 响应面终局 `stop=done`（`[continuation.arm] status=completed stop=done requires_user_interaction=false`，agent_last.log 23:46:23）。
4. **孤儿收口**：`turn_close_guard.go:56` 将仍处 improvement_proposal 态的任务按 `owner_turn_closed`（"chat turn closed without governed experiment"）关闭（semantic_state history rev4 亲证）——确认卡从未建（pending_candidates 有候选记录但无确认 interaction 落库）。

## 三、对 262 卡前提的增量结论

- **裁定 A 的直接目标面（弹回族）本 run 未触发也未复发**：模型没有重复请求已返回 view set（A patch 新指令的触发条件＝去重门同判据的重复请求，从未发生）；turn_directives 逐轮核对（prompt render 6 行亲证）：A-patch 新指令与 :292 闭合句族**均未出现**——模型完全自主到达提案，措辞面没有机会起作用也没有被违反。
- **真实阻断点在 A patch 下游一层（域外）**：提案有效但被 D1-S1 准入门的「evidence_refs 必须含最新目标观察 ID」规则拒绝，且该拒绝路径**终局化**（blocked→done）而非回喂/挂确认面。这是「模型引用选择 × 门最新绑定规则 × 拒绝即终局」三因素叠加的新形态，与 257 取证的 L1（弹回）/L3（settle 读错面）均不同族。
- 257 对照：T32（排干切片后续跑）的提案通过准入并落卡（timing :321）；本 run 提案在观察回合内更早到达却因引用选择被拒——两次模型引用行为不同（随机分支），门的「最新观察绑定」规则未变，257 run 恰好引用了当时的最新观察。
- 处置建议（交主管）：本形态修补落点不在 262 文件域（chat 措辞面/scripts 断言面）——候选方向供裁定：①准入门拒绝改回喂/挂「方案乙 park」确认面（`free_state_reasoning_loop.go` terminalAdjudicationParking 族已有先例机制）；②门的引用校验放宽为「任一目标级新鲜观察」而非 LatestObservation 绑定；③措辞面教模型引用最新返回观察（262 域内可做但属新话术族）。**本卡边界内零代码改动兑现，未动手。**

## 四、工件清单

| 文件 | 内容 |
|---|---|
| RUNBOOK.md | §8 跑前口径（跑前成文，随 a05cea2b 已推） |
| console_20261106_234509.txt | PS1 全程 console（exit 1 断言行在尾） |
| telemetry_run1.jsonl | 本 conversation 7 条 LLM telemetry |
| agent_last_run1.log | agent 会话日志（observe turn 时间线 171-233 行段） |
| agent_runtime_state_run1.json | runtime state 全量（semantic_state history/latest_decision/pending_candidates/goal_runtime/free_state_reasoning_loops 亲证源） |
| agent_binary_sha256.txt | 被测 agent 哈希 B280318D… |
