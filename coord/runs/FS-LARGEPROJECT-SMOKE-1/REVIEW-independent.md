# FS-LARGEPROJECT-SMOKE-1 独立复核报告（L2 复核腿，只读）

- 复核对象：执行会话对卡 FS-LARGEPROJECT-SMOKE-1 的脚本实现与两轮功能运行
- 复核腿：general-purpose 独立 agent（与实现不同会话），2026-10-10；复核过程零文件修改
- 复核材料：卡面（main 上 doing/ 版，领取提交 d4ee5f2a）、scripts/fs_largeproject_smoke.ps1、四个 run 目录、PC-RUNTIME-STACK（占用登记 de1d441b）、PROTOCOL §2/§2.2/§3、AGENTS §5/§8/§9/§10、J3/G3 先例工件、agent Go 源（法条核对）

前置说明：任务书给的是 worktree 内 coord/cards/doing/ 路径，实际该 worktree 中卡面在 todo/（视图滞后）；权威卡面在协调 worktree D:\Vit_DAW_wt_fsLPS1_coord\coord\cards\doing\，领取栏已按 PROTOCOL §2 回填并推 main。协议面领取合规。

## A. 四断言组脚本实现 vs 卡面验收标准

**结论：pass（附两条低危覆盖面备注）**

- A1（pullharness+指纹，脚本 pullharness_with_fingerprint>=1 逐条 census）：忠实。
- A2（站图事件 trajectory.round.started/observation.recorded/(hypothesis.proposed|mix_tick)，hops 后快照）：与 J3 先例 20261007_202322 事件型谱逐一对得上（该 run 实测 round=1/observation=2/hypothesis=1/mix_tick.pending=2）；事件型在 agent/internal/trajectory/types.go:24-26 有权威定义。
- A3（cardMounted AND approveAnswered AND audition>=1 AND reobserved+completed 扫全部 chat/hop 响应）：与 G3 harness_ab_confirm_roundtrip.json 断言词汇同构，全链覆盖。
- A4（五标记 semantic_eq_batch/eq_plugin_load_batch/dynamic_plugin_load_batch/dynamic_parameter_batch/c2_dynamic_plugin_selection，扫响应 JSON/事件快照/agent log，显式豁免 fastpath）：C1/C2/B4 主干全覆盖；标记与 Go 源实存子串可命中（b4_eq_runtime.go:26-27、c1_frequency_cleanup_runtime.go:27-28、c2_dynamic_batch.go:26-27、c2_dynamic_control_runtime.go:625）。

低危备注（不构成 fail）：①源中另有受控命令 c2.dynamic_plugin_load.governed（单轨装载）与 plugin_grabber.apply_eq_edits.governed，前者不落入 _batch 形标记子串，卡面措辞下算轻微欠覆盖；②A4 扫描面未含 telemetry source 面（固定编排规划器有独立 LLM source 标如 b4_project_eq_planner）；③EventsLimit=500 是 /agent/events 服务端上限，超长链会截尾（J3/G3 先例同面，本轮无实际影响）。

## B. 两轮功能运行的失败分类与止损纪律

**结论：pass（附一条分类字段关注项）**

- run 184500 = model_protocol_failure（infra_error_llm）：nl_chat_1.json 原文证实 fs_loop last_error=model_protocol_failure、boundary=proposal_missing（G4/G6/G7/G8 fail 是提案缺失下游）、顶层 stop_reason=failed；telemetry 含 1 条 message_loop_repair。A1 部分证据成立（3 条 pullharness 全带指纹）。分记诚实。
- run 185519 = capability_blocked（准入收据）：run_report 记录 fs_loop_admission_status=capability_blocked、fs_loop_last_error=free_state_admission_gate_failed；receipt 原文 status=capability_blocked, boundary=admission_gate_failed, G3-G8 fail / G1-G2 pass。
- 关注项：run 185519 run_report.classification="other" 与人工分类 capability_blocked 不一致——脚本分类阶梯只查 stop reasons 与 fsLoopLastError 字面，未消费已提取的 fsLoopAdmissionStatus。原始字段已如实入报告可回溯，不属隐瞒；机器/人工分类分歧在回执中说明，分类阶梯补 receipt 通道另立 hygiene 卡（不在本卡文件域）。
- 止损纪律：两轮断点不同（决策段 model_protocol_failure vs 准入门 capability_blocked），非同断点连续两败，正确未触发止损、未借机改写结论；≤2 轮预算用尽后如实停止。两次工具性中止（183305/183914）均未起 LLM 轮（无 telemetry_pull.jsonl、无 nl_chat 文件），不计入概率预算，与 AGENTS §8 "LLM 参与的运行"口径一致。

## C. E:\ 源只读红线

**结论：pass**

三份 manifest（before/mid/after）逐字节一致（61 条 name/size/mtime_utc_ticks 全等，python 全量比对）。脚本写入面全部锚定 RunRoot（报告/telemetry/drafts 重定向 VIT_HISTORY_DRAFT_ROOT/隔离工程/agent log 副本）；对 E:\ 仅有 Get-ChildItem/Resolve-Path 读操作，导入命令以 folder_path 传内核读源。无任何向 E:\ 的写路径。行为与代码面双向印证。

## D. §9 回执面

**结论：concern（主体齐备，四处缺口）**

已齐备：run ID、repo HEAD（与 head.txt 一致）、branch/worktree、git_status（仅 ?? scripts/ + ?? coord/runs/，与零 Go 改动互证；git diff origin/main -- agent/ 为空）、kernel exe+SHA256、关键开关（pull_env.json 三 env+注入时刻）、telemetry census+原始 jsonl、preheat 轮询史、断言块+前后站图 census、teardown（torn_down=true, remaining=[]）、agent 原始日志副本 127KB。构建非 SkipBuild。

缺口：①command_line 字段为空串（-File 调用下 $MyInvocation.Line 为空）——完整命令不可从工件复原；②退出码未数值化记录（仅 verdict FAIL）；③agent_binary 字段恒空串（死字段）；④run 184500 因 Phase 6 $pid 崩溃无 run_report.json（RUN_NOTE+console 原文+残留工件作替代回执，该轮 §9 面不完整是事实）；⑤流程项：PC-RUNTIME-STACK 当时仍标占用中（验收前须补释放记录，见回执必办项）。

## E. 脚本缺陷修复史（四次迭代）定性

**结论：pass**

四次迭代均工具性缺陷、各有原始证据，非功能失败凑数：183305（early save_as 撞 draft 会话竞态+Invoke-WebRequest 非 2xx 抛异常死于 teardown 前）、183914（preflight result.summary 提取面错误）、184500（Phase 6 赋值只读自动变量 $pid 崩溃，LLM 轮已完成 A1 已成立后崩溃）、185519（干净跑全流程）。两个 ABORTED 目录保留 NOTE+console+现场，四目录互不覆盖，满足证据保留义务。缺陷-修复对应关系可核（Invoke-ToolJson 容错面、save 重排+重试、$pid 改名、全局 try/catch、result.summary 提取）。

## F. 高价值发现的独立判读

**结论：发现成立且比执行侧上交口径更有价值，但事实句须修正一处（concern）**

1. 执行侧原句"模型只请求了 track 级 view"**不完整**：nl_chat_1.json executed_kernel_reply[1]（ccb.observation_request，obs_20261010T105649_06b2e7251024，早于最终 track 观察 18 秒）requested_views=[mix.multitrack_relationship, mix.frequency_relationship]、audit_receipt requested_by=model scope=full_project；mix.multitrack_relationship **实际交付**（28.5KB MOM 投影事实：rankings.level/peak、relationship_inputs、risks.headroom，status=partial），仅 mix.frequency_relationship 被 disclosure budget 裁掉（27245/65536）。模型行为面没有"不会要 mix view"的问题。
2. 真正的缺口在**台账持久化**：终态 observation_ledger.receipts 只剩 track 级一张（receipt_count=1），mix 回执未存活到准入评估；fs_loop cycle=0（同 run ≥4 个 pullharness 模型轮）。按法条 agent/internal/agentloop/free_state_gate.go:137,139-146,185-215（freeStateReceiptUsable 接受 partial、disclosure 剔除只针对被裁 view），若 mix 回执在台账中 G3 本应 pass；chat/free_state_reasoning_loop.go:817-840 表明每张观察应逐张并入台账。mix 回执丢失指向跨模型轮的 loop/continuation 持久化缺陷（或 loop 激活时序），非模型选择。归因修正为："61 轨大工程上，准入门在存在一张合法已交付的全曲 scan 收据时仍判 capability_blocked"。
3. receipt project_revision="" 属实可定位：构造点 free_state_reasoning_loop.go:738 只读 LatestProjectChange 与 observation_binding.project_binding，而该 CCB bundle 的 revision "4" 落在 freshness.project_revision；更宽回退链（:1865-1871 freeStateObservationProjectRevision）未在此处使用。回执构造面窄回退缺陷，不改门结果，另立 hygiene 卡。
4. "surface done+completed despite blocked receipt"属实：顶层 stop_reason=done, goal_status=completed, workflow 键缺席，fs_loop status=capability_blocked——FS-CAPABILITY-BLOCKED-SURFACE-1 边界面在 fs2_capacity_assessed 阶段未接合。已正确声明出本卡文件域。
5. admission_receipt 与原始观察证据互相印证（proposal_present=true/proposal_valid=true/target_evidence_ref 非空），收据诚实；模型 prose（锁定 track 1037、−1.5dB@5kHz 提案、请求 A/B 试听）与提案字段一致。

## 总体结论

**支持将本卡以"verdict FAIL（A1/A4 pass，A2/A3 fail）+ 高价值发现上交"形态交验收**。验收前必办：①上交文本按 F-2 修正发现归因（mix view 已请求已交付、台账丢收据、cycle=0）；②PC-RUNTIME-STACK 补释放记录推 main；③回执显式说明 classification="other" 与人工分类 capability_blocked 的分歧及原始字段出处。建议另立卡：分类阶梯消费 fsLoopAdmissionStatus（脚本侧 hygiene）；台账跨轮持久化/cycle 计数 Go 侧锚点取证卡（free_state_reasoning_loop.go:817-840 合并路径 + cycle=0 现象）；receipt project_revision 窄回退（:738）hygiene；A4 补 c2.dynamic_plugin_load.governed 标记与 telemetry source 面；run_report 补 command_line/exit code。

本卡自身文件域（scripts/fs_largeproject_smoke.ps1 + coord/runs/FS-LARGEPROJECT-SMOKE-1/）零越界：git diff origin/main -- agent/ 为空。
