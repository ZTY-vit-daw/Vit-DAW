# FS-FRONTIER-OBSERVE-GAP-1 取证结论（卡 156）

- 执行：GLM-5.3 L2 PC 执行会话 ZCode（本会话）；基线=origin/main 171640d5（领取提交 5b6bd8ca）；2026-10-21 晚窗
- 输入：146 取证报告 §① L1 + 146-SOFT-GUIDE-EXP/20261020_b2arm 原始工件（prompt_render / message_loop_debug / events 双侧快照）+ 171640d5 干净 worktree 源码
- 本卡不改生产代码；设计件（三案对比+拆卡）见同目录 [DESIGN.md](DESIGN.md)

## TL;DR

1. **缺口①归因**：candidate frontier 的轨集是**混音内容派生、用户意图盲**的——candidates 只从三个 mix 扫描视图的 top-k band 重叠行派生（`freeStateCandidateScanViewIDs`），vocals/1032 在 A1/A3 两轮的 band 行里排不进任何行的 tracks 数组，于是既进不了 frontier，又被进度守卫禁止观察（指名轨连 track.* 观察都发不出），再被 G6/G8 双重锁死提案面。blocked 族是唯一合法出口——模型自报 capability_blocked 是诚实行为。
2. **供应不稳定性（新 finding）**：A1 与 A2 请求完全相同的视图，却消费了**正午相差 73 秒的两份持久观察**（obs_…T123522 vs obs_…T123635/123642），band 行内容不同（前者 mid/presence/air=[piano,guitar,other]，后者=[vocals,piano,guitar]）。frontier 内容是观察供应的非平稳函数——同一工程同一话术，能否到提案面取决于 ledger 供应了哪一份。
3. **缺口②归因**：结转断点在 `free_state_reasoning_loop.go:265-270` 的 switch——merge 只对 active 循环生效，终态词表之外的循环在下次 prepare 时走 `:277-292` 全新构造（空 ObservationLedger、新 LoopID）；且 blocked 族状态在空 ledger 下畅通（final gate 仅要求 status+summary）——首轮无强制观察的机制面坐实。
4. **两缺口独立、不可互替**：缺口②=延续/冷启动断层（M2 二轮型：有历史、新循环零结转）；缺口①=有观察但 frontier 用户盲（F-gate 型）。修复组合见 DESIGN.md。
5. **停止条件未触发**：三案均落在既有分层内（chat 侧 loop store / closure fold + agentloop 只读消费），不要求 FS0–FS9 统一收敛；B 案的 T10 邻接边界已写死（见 DESIGN §4）。

## ① candidate frontier 为何不含用户指名轨

### 生成面（代码锚点）

| # | 事实 | 锚点（agent/ 下） |
|---|---|---|
| F1 | frontier 构造入口：closure fold 时从 observations 重建 candidates | `internal/chat/audio_closure_controller.go:1039`（audioClosureFrontier）→ `:1067`（audioClosureCandidates） |
| F2 | candidates **只**来自三个扫描视图的行：multitrack / frequency / masking relationship；其它 view id 一律落空 | `internal/chat/audio_closure_controller.go:1270`（freeStateCandidateScanViewIDs）+ `:1139-1178`（audioClosureCandidateRows 的 switch 只有这三个 arm） |
| F3 | 行内容=MOM/工程包的 band 冲突行，每行 tracks 数组=该 band 的 top 轨（multitrack 按 unit_energy/energy_db 可排序取 top-k） | catalog 披露 `internal/tools/catalog.go:732`；A1 实测每行恰 3 轨（工件引证见下） |
| F4 | candidate.track_ids = 行的 tracks；允许轨集=（未选定时）全体 candidate 轨并集 | `internal/chat/audio_closure_controller.go:1083-1103` |
| F5 | 披露面：系统提示词逐字列出 `candidate:<id> tracks=<ids>` | `internal/agentloop/ccb_model_prompt.go:191-226`（:204 拼行） |
| F6 | 进度守卫：frontier 有 candidates 且无 selected 且本轮无 candidate 轨 target 观察时——零工具调用→弹回（:1444）；project.*/mix.* 重试→弹回（:1453）；target 不在允许轨集→弹回（:1458，实验签名原文）；needs_action→弹回（:1421）；**blocked 族→放行** | `internal/agentloop/free_state_reasoning.go:1395-1461`（messageLoopFreeStateCandidateProgressionIssue，由 :920 输出校验器在 :1005/:1032 调用；调试面=`internal/agentloop/message_loop.go:10870` stage=free_state_final_gate） |
| F7 | G6：target 级证据必须落在允许轨集上（selected 时=该 candidate 轨集）；:342 的 rescue 只救**空 frontier**场景，不救本题 | `internal/agentloop/free_state_gate.go:312-365` |
| F8 | G8 断言 1：提案目标必须是 candidate 轨——指名轨即使有提案也被拒 | `internal/agentloop/free_state_gate.go:444-484` |
| F9 | GATE PATH 软指令自述"disclosure only — no evaluator reads it"：运行时引导走"mix 扫描→frontier→target 观察→提案"路径，但无任何机器强制 | `internal/agentloop/ccb_model_prompt.go:230-283` |

### 工件回指（coord/runs/146-SOFT-GUIDE-EXP/20261020_b2arm/）

- **弹回记录**：`logs_sge1/agent_message_loop_debug.jsonl`——A1（conv softguide_b2_A1_20261020_203509）turns 4-7 共 4 次、A3（…_203711）turns 4-7 共 4 次、B1（…_204023）1 次，全部 stage=free_state_final_gate、error=「closure candidate frontier requires target_ref.kind=track and an id from the candidate track set」。模型第 7 轮（下一轮）自报 capability_blocked。
- **frontier 披露**（`logs_sge1/agent_prompt_render.jsonl`，messageLoopCandidateFrontierDirective 段）：
  - A1：4 candidates=`{1017,1022,1027}`×2+`{1007,1012,1022}`×2 → **并集 {1007,1012,1017,1022,1027}，无 1032**；
  - A3：2 candidates=`{1007,1022}`+`{1017,1027}` → **并集 {1007,1017,1022,1027}，无 1032、无 1012**；
  - 对照（到达提案面的轮次 A2/A4/A5/B4 与真 B2 两轮）：均有含 1032 的行 `tracks=1017,1027,1032`。
- **内容根因**（`driver_ctrlshape_8rounds_patch_inert/arm/{A1,A2,A3}/events.json`）：
  - A1 的 multitrack band 行：sub=[1012,1007,1022]、bass=[1007,1022,1012]、low_mid/mid/presence/air=[1027,1017,1022]——**vocals/1032 不在任何行的 tracks 数组**，只出现在 metric 排名事实（level_db rank1 -9.032、peak -6.021）；
  - A2 的 mid/presence/air=[1032,1027,1017]——vocals 在列且居首（energy_db 69.7）；
  - A3 请求的是 frequency_relationship（conflict_candidates 行），同样无 1032 于 tracks；
  - 观察包 ID：A1=obs_20261020T**123522**、A2=obs_20261020T**123635/123642**（正午 12:35/12:36 的持久观察，非 20:3x 会话现场产出）——**同请求、不同供应**：两份观察相差 73 秒、band 行不同，20:35/20:36 的会话各自 ledger 供应了其中一份。
- 判定：RESULTS §3 原「frontier 与用户指名轨冲突」精确化为——**frontier 语义里根本没有用户意图的耦合面**；指名轨能否进 frontier 取决于它在当时供应的 band 重叠行里排不排得进 top-k。F-gate 3/8 与惯例文本正交的判定维持成立。

### 设计推论（详见 DESIGN.md）

用户指名轨必须有一条**不依赖 mix 内容巧合**的入 frontier 通道（C 案），否则守卫+G6+G8 构成对用户意图的机械封锁。同时观察供应的非平稳性意味着「重试一次换个供应」不可作为修复依赖。

## ② 新循环 ledger 零结转 + 无首轮强制观察

| # | 事实 | 锚点 |
|---|---|---|
| L1 | merge 只在 persisted 循环 **active** 时发生；终态词表=completed/cancelled/blocked/capability_blocked/no_candidate_found/failed/stopped | `internal/chat/free_state_reasoning_loop.go:265-270`（switch）+ `:191-199`（freeStateLoopActive） |
| L2 | 前驱终态后，下次 prepare 全新构造循环：空 ObservationLedger、新 LoopID、新 CreatedAt；OriginalIntent=新话术 | `internal/chat/free_state_reasoning_loop.go:277-292` |
| L3 | loop 存储=单会话单槽 `s.freeStateLoops[conversationID]`，新循环覆盖前驱；prepare 在 `:263` 读旧、`:316` 存新——**前驱数据在覆盖前可捕获**（B 案落点） | `:648-677`（storeFreeStateLoop/freeStateLoop）；持久化面 `internal/chat/server.go:215`（agent_runtime_state.json 的 free_state_reasoning_loops） |
| L4 | 无首轮强制观察：blocked 族结算只要求 status+summary（空 ledger 畅通）；needs_action 要求 ≥1 成功观察（:1026-1031）但这是后置审计非前置步；needs_experiment 空账本会被 G3-G8 审计弹回；唯一引导=GATE PATH 软指令（F9） | `internal/agentloop/free_state_reasoning.go:212`（blocked case）、`:1026-1031`；`internal/agentloop/ccb_model_prompt.go:230-283` |
| L5 | M2 二轮工件（146 已钉，此处复核引用）：turn10 零工具调用、ledger boundary stored_receipts=0 / ccb_observations=1 / snapshot_loop=true continuation_loop=false、同 goal 复用（goal_98d6813ec6100767） | 146 取证报告 E1-E3（agent_last.log:492-495） |

**语义判定**：这不是 bug 而是设计缺面——loop 生命周期把「终态」当「血统终点」，没有后继链接机制；同时首轮的观察义务完全交给模型自觉（软指令）。两者合成「延续式模糊意图断在提案面之前」（146 L1 原文成立）。

## ③ 修复对 146 惯例线的意义（卡待证 3）

146-A 惯例段与 154 拓扑计划的解释/落笔面都只在**模型能到提案面**时可见。F-gate（缺口①）直接压制到达率（实验 3/8），观察断层（缺口②）断掉延续意图（M2 二轮）。本卡两缺口是模糊意图链在惯例/拓扑层**之前**的前置断层——修通后 146 实验重排（§E 原协议）才能测出惯例文本的真实路由效应，NL 旅程 F-gate 族记录面（156/E 上游）才可消解。

## 停止条件核对

- **T10（FS0–FS9 统一收敛）：未触发**。三案均不动状态机词表与门数量；agentloop 侧全部只读消费 closure/loop 上下文。B 案触及 loop 生命周期边界语义，其 T10 邻接红线（不统一两 ledger schema、只做 chat 侧搬运）已写入 DESIGN §4。
- 未发现需要内核新命令或跨层改写的面。
