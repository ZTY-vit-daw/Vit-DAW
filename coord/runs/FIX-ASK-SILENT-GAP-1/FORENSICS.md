# FIX-ASK-SILENT-GAP-1 取证件：ask→slice 122 秒静默空窗的阶段分解

- 执行：GLM-5.3 L2 PC 执行会话（ZCode，worktree D:/Vit_DAW_wt_asg1，分支 port/fix-ask-silent-gap-1-recon）
- 日期：2026-10-15 晚窗；领取提交 f04b28c2（origin/main e115fd8a 之上）；取证腿（零生产代码合入）
- 基线：[M2-EXPERIENCE-1](../../runs/M2-EXPERIENCE-1/README.md) 两轮样本（ask 09:53:11 落账 → slice 09:55:13.794 启动，122s 静默；12:38 复跑同签名）
- 证据等级：真栈两轮（空工程 A + 6 轨有轨 B）+ M2 现场 LLM 遥测原记录 + 代码锚点三方互证；122s 构成精确到毫秒（120.855s 遥测实测 = 代码常量推算值逐毫秒吻合）

## 一、结论（TL;DR）

**122s 空窗 = ask 前置语义入口分类器的一次 LLM 调用（`ordinary_agent_semantic_entry`，deepseek-flash）在服务退化下的重试环独占**：

```
60.000s  HTTP 超时（挂死请求，默认 client timeout=60s，可重试错误）
+0.250s  退避
+60.000s HTTP 超时（第二次挂死）
+0.500s  退避
+0.105s  第三次请求：HTTP 400 "Service is too busy"（不可重试，放弃）
=120.855s ← 遥测 total_ms=120855 实测值，与 llm/client.go 常量逐毫秒吻合
```

期间用户可见面零事件（turn.started 在 t+0 已发，下一个事件要等 turn 终态）；journal_records=0 是因为 journal 只计 message loop，**语义入口分类器的 LLM 调用不在 journal 里**——基线"LLM 一次未被调用"的推断需修正为"message loop 的 LLM 未被调用，语义入口调用发生且吞掉了 120.9s"。

**叠加缺陷（空工程放大器真相）**：语义入口失败走 fail-open（`semantic_entry_unavailable=true`）后，**跳过了本应执行的空工程容量短路**（`planObservationFirstCapabilityRoute` 只在 `err == nil` 分支运行）——空工程上本应 ~2 秒直接作答的请求，被放进 slice 任务管线。A 轮真栈对照实证了短路路径的存在与健康形态。空工程本身**不是**空窗的放大因素（语义入口载荷只有用户文本，与轨数无关）；它只在"失败分支跳过短路"这一点上与有轨工程产生行为差异。

**mixboard 快照 +63s 落盘与卡点无关**：它是内核波形遥测的后台摄取（`ingestKernelWaveformTelemetry`，harness.go:4180 起，`writeMixboardReadyProjectTrackWaveformSnapshot`）在等待期间顺带写入，基线把它读成"上下文组装活动"是误读。

## 二、三腿证据

### 腿 1：M2 现场遥测（退化态，历史原记录，未动现场）

`D:\Vit_DAW\VitApp\Workspace\Logs\agent_llm_telemetry.jsonl`（37MB 累计文件）窗口内唯一记录（详见本目录 `telemetry_extract.json`）：

```json
{"created_at":"2026-10-15T01:55:12.4214141Z","source":"ordinary_agent_semantic_entry",
 "conversation_id":"webui_mv8vwp4d","model":"deepseek-flash",
 "http_ms":104,"total_ms":120855,
 "error":"Service is too busy. We advise users to temporarily switch to alternative LLM API service providers. (request_id: c55354ff-873a-40f9-b179-cf0c5114e387)"}
```

- `total_ms=120855` 即 09:53:11.5 → 09:55:12.42 的等待主体；其后 1.4s 为失败分支收尾+prepare 链，09:55:13.794 slice/turn 开启（与 /agent/state 亲证时间戳吻合）。
- `http_ms=104` 是**末次尝试**的时长（Timings.HTTPMs 每次尝试覆盖写入），证明时间烧在重试环而非单次慢请求。
- 第二轮（12:38）同源：`total_ms=47863, http_ms=47863, error="context canceled"`——单次挂死请求被用户关场 cancel，47.9s。
- **复发模式**：按日聚合 10344 条语义入口记录，历史上多次出现 60–124s 级 total_ms（2026-09-08 max=124030ms、08-24 98873ms、08-23 89485ms、08-11 77111ms、09-07 64308ms）——服务退化期该前置调用周期性变成分钟级静默墙。
- 10-14 烟测（6 轨 fixture，run12/13 同链路）两条对照记录：6990ms / 8745ms 无错——**烟测无此空窗的原因是当晚服务健康，不是工程有轨**。

### 腿 2：真栈 A 轮（空工程 0 轨 + 健康 LLM）——短路路径实证

栈=worktree 打点 agent（SHA256 669d2109bd1a0981）+主检出 VitApp.exe（B6565DCF85D1DA86）+Godot 主检出；LLM 预探测 0.83s HTTP 200（deepseek-flash）。话术=M2 原文（69 字符，码点文件构造）。工件：`empty_A1/`。

| t（自 chat POST） | 事件 | 证据 |
|---|---|---|
| +6ms | chat POST 发出 | timeline.json |
| ~+0.4s | ask_booked（ask checkpoint） | [asg1] 日志 |
| +0.48s | **turn.started 首个用户可见事件** | events 面 |
| +1s | classification_begin | [asg1] |
| +1→3.1s | 语义入口 LLM 1 次尝试 **2092ms 成功** route=open_semantic | [asg1] llm_attempt_ok |
| +3.1s | capability_route_done 14ms **short_circuit=true**（空工程容量事实） | [asg1] |
| +3.7s | turn.completed（`stop=capacity_fact_short_circuit`，AGENT-F3 短路作答） | [timing] agent_loop_chat total=2109ms |
| +4.0s | http.chat 完成 | [timing] |

**零 slice、零 message loop**：空工程+健康服务下这条话术 4 秒内作答。这是 M2 现场本应走到的路径。

### 腿 3：真栈 B 轮（6 轨 fixture + 健康 LLM）——有轨健康路径阶段分解

6 轨 fixture（spv1_p01，复制到 worktree artifacts 隔离目录后 `project.open`；shadow track_count=6 亲核）。工件：`tracks_B1/`。

| t（自 chat POST） | 事件 | 证据 |
|---|---|---|
| +0 | chat POST 发出 | timeline.json |
| **+4.80s** | **turn.started 首个用户可见事件** | events 面（⚠️见修复面 5） |
| +4.81s | ask_booked（前导段 ~4.4s：goal 绑定/工作区激活/工件装配） | [asg1] |
| +5s | classification_begin | [asg1] |
| +5→11.2s | 语义入口 LLM **6165ms 成功** route=open_semantic | [asg1] |
| +11.2s | capability_route_done 32ms short_circuit=false（有轨→自由态准入） | [asg1] |
| +11.2s | prepare_done（mix/closure/free-state 三 prepare 合计 <1s） | [asg1] |
| **+11.64s** | **slice_1/turn_1 开启**（messageLoop.Start→buildStartState 同步开） | state 面 slice started_at=18:24:15.3828167 |
| +15→19s | message loop 首个 LLM turn（3777ms，1 turn 后 limit_reached→waiting_continue） | [asg1]+[timing] |
| +25.6s | 探针 POST /agent/turn/stop（控预算），turn.stopped 收口 | timeline.json |

**ask→slice = 11.6s**，其中语义入口 6.2s + 前导段 4.4s + prepare/slice 开启 <1s。烟测"无空窗"的同构健康形态。

## 三、阶段构成总表（122s 的分解）

| 阶段 | M2 现场（退化） | B 轮（有轨健康） | A 轮（空工程健康） |
|---|---|---|---|
| chat POST→ask_booked 前导 | ~0.5s（空工程） | **~4.4s** | ~0.4s |
| 语义入口 LLM（含重试环） | **120.855s** | 6.2s | 2.1s |
| 失败分支收尾+容量路由 | 跳过（err≠nil 不进路由/短路） | 32ms（短路评估） | 14ms（短路命中） |
| prepare 三连+工具上下文 | ~1.4s | <1s | —（已短路） |
| messageLoop.Start→slice/turn 开 | 09:55:13.794（122.8s 处） | +0.3s | —（零 slice） |
| 首个用户可见事件（turn.started） | t+0（其后 122s 静默） | **t+4.8s** | t+0.48s |

## 四、修复面清单（交决策侧核对 116 文件域后派修复腿）

锚点行号=origin/main e115fd8a 版本。

1. **语义入口无 deadline（主修复面）** `agent/internal/chat/semantic_entry.go:132-140`
   `planSemanticEntry` 构造的 `llm.Request` 不带 `Timeout`，落到 `llm/client.go:157-161` 的默认 60s/次。它是有 fail-open 出路的**前置分类器**（失败后 `semantic_entry_unavailable=true` 继续走），却持有与主循环同级的超时预算。修复方向：给该 Request 设短 deadline（如 10–15s），或给整段分类设总预算。
2. **默认重试结构把 60s 超时放大成 ~121s** `agent/internal/llm/client.go:157-174`
   小载荷 `attempts=3`（:167-170）+ 每次尝试各自 60s timeout + 250/500ms 退避（:173-179）= 最坏 ~121s，对任何未显式设 Timeout 的小请求成立。修复方向：per-request deadline 贯穿重试环（总预算而非每次预算），或退避上限与尝试数按 Timeout 比例收缩。
3. **fail-open 跳过空工程短路（叠加缺陷）** `agent/internal/chat/goalrunner_chat.go:233-262`
   `err != nil` 分支（:241-243）置 `semantic_entry_unavailable=true` 后继续；容量路由与 `capacity_fact_shortCircuit`（capability_routing.go:257-263）只在 `if err == nil`（:247）内运行。语义入口失败时，空工程/容量事实短路**不再有机会**把请求在秒级收束——M2 现场 0 轨工程因此进了 slice 管线。修复方向：入口失败时仍跑一次本地（零 LLM）容量事实评估，能短路则短路作答并声明"入口分类暂不可用"。
4. **分类期零用户反馈（反馈面，卡面目标 2）** `agent/internal/chat/server.go:2509-2523` + `goalrunner_chat.go:234`
   ask 落账到 slice 开启之间，transport 面只有 t+0 的 turn.started 一个事件；语义入口（健康 2-6s，退化 121s）期间无任何忙碌指示。修复方向：classification_begin/入口失败等节点发「正在理解请求…」级 trajectory/transport 事件（webui 已有事件消费面；反馈位改动归本卡 webui 反馈位文件域）。
5. **（次级，本轮新发现）前导段随工程规模膨胀** B 轮 POST→ask_booked ~4.4s（A 轮 0.4s），且**首个可见事件 turn.started 被推迟到 t+4.8s**——已超卡面 ≤3s 反馈预算。锚点区间=server.go handleChat 前导（goal 绑定/`uiContextSnapshot`/工作区激活/工件装配，约 :2459-2523）。建议修复腿先打点细分再决定是否单拆卡。

### 与 116 卡的文件域关系

本清单触面：`chat/semantic_entry.go`、`chat/goalrunner_chat.go`（分类段，非停止路径）、`chat/server.go`（事件面）、`llm/client.go`、（反馈位）webui。**未触 `chat/turn_control.go` 与 harness 停止路径**（116 域）。`goalrunner_chat.go` 两卡都可能碰：116 若在停止路径改 `finalizeStoppedTurn` 调用点（:464-471）与本卡分类段（:233-262）不同函数不相交，但同文件——按协议由决策侧核对后再派。

## 五、§8 运行记账（LLM 概率面）

- 计划轮数 2（空+有轨各一），实际 2，全部达成取证目标（阶段时间线+[asg1] 日志+events/state 面三方对齐）。无失败轮，无需止损。
- LLM 调用实耗 5 次：健康预探测 1（0.83s）+ A 轮语义入口 1（2.09s 成功）+ B 轮语义入口 1（6.16s 成功）+ B 轮 message loop 1（3.78s 成功，单 turn limit_reached 自然收口）+ 遥测归档核查 0。全部 err=false，0 次超时。
- B 轮探针在 slice 出现后 POST stop 控预算；stop 落地时 goal 已 waiting_continue（limit_reached），按 user_stop 收口，无残留任务。
- 退化态 live 复现**未执行**：需真实服务退化或改全局 config（纪律禁动），历史遥测两独立样本+常量逐毫秒吻合已构成充分证据，如实登记边界。

## 六、工件索引

- `empty_A1/`：A 轮 timeline/events/state（timeline 8 行：短路零 slice 形态）
- `tracks_B1/`：B 轮 timeline/events/state（12 行：完整 ask→slice→stop 形态）
- `agent_log_asg1_excerpt.log`：两轮 [asg1]+[timing] 打点原行（28 行）
- `telemetry_extract.json`：M2 两轮原记录 + 10-14 烟测对照 + 说明
- 重件留盘（不入库）：worktree `artifacts/asg1_forensics/`（fixture 6 轨隔离副本 fixture_project_B/、完整 run 目录）；worktree `VitApp/Workspace/Logs/agent_last.log`（环形 500 行，已摘录）
- 临时打点与探针：分支 `port/fix-ask-silent-gap-1-recon` 提交 0eb31135（四文件 [asg1] 日志 + scripts/asg1_ask_gap_forensics.ps1 + scripts/asg1_prompt_codes.txt），**不合入 main**，供修复腿按需 cherry-pick 打点
- 栈占用/释放：登记 9b817024（占用）→ 释放登记见资源记录同日条目

## 七、停止声明

取证腿到此交付停手。修复腿（deadline/短路/反馈面）由决策侧核对与 116 文件域后另行派发；本轮未改任何生产代码（worktree 打点仅存取证分支）。
