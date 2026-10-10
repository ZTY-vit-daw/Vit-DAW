# G3-PROBE-METER-RECON-1：probe 物理成本计量源勘察报告（只读，零代码）

- 卡：`coord/cards/doing/2026-10-10-G3-PROBE-METER-RECON-1.md`（池序 42，G3 终裁重开条件①输入）
- 勘察时间：2026-10-10 闲时车道；代码基线=origin/main f0dc8e77（本地 HEAD 一致，零源码触碰）
- **一句话结论：有源——agent 侧已存在可接入的物理计量（两路墙钟+一路 probe 级结构化测量），接线点在 pull_session.go:411-413 已预声明；最小接入面约 2 文件，重开条件①的计量前提可以满足，无需新内核面。**

## 0. 背景重述

G3 终裁维持 push 生产缺省，唯一实质差距=轮级 LLM 调用 17v6；重开条件①="probe 计量源接入后真实成本对比"。G3-ATTRIB-1 run 中 `max_probe_cost: 8` 已注入但执法以计量为前提（报告披露 ProbeCostKnown=false 如实），预算实际执法量= max_cycles。

## 1. unknown 构成清单（pull 账本记账位逐项）

| # | 记账位 | 锚点 | 现态与原因 |
|---|---|---|---|
| 1 | `pullLedger.probeSpent / probeCostKnown` | `agent/internal/agentloop/pull_session.go:44-55` | 账户字段**存在**；:147 初始 `probeCostKnown: true`（缺省已知零成本），:425-428 任一新回执即翻 `false` |
| 2 | 结算函数 `settle` | `pull_session.go:81-92` | 只计 `toolAttempts`；**回执上无成本字段可读**——注释明示"未知成本（现执行面无统一 ProbeCost 字段）不填 0" |
| 3 | 批结算 `settleBatch` | `pull_session.go:411-430` | **接线点已预声明**："现执行面无统一 probe 计量（提案 §5.1 取证点）——成本标 unknown，不填 0 冒充免费；**有真实计量源后在此接入**" |
| 4 | 执法门 `exhausted()` | `pull_session.go:57-65` | max_probe_cost 执法要求 `probeCostKnown && probeSpent>=max`——unknown 即停用（本 run 实况：执法量=max_cycles=1） |
| 5 | 设计面（账户/披露/止损） | `agent/internal/pullharness/budget.go:20-24,:60-87` | ObservationBudget.ProbeCost/MaxProbeCost+ProbeExhausted+Disclosure（动态区 budget_state 行）**齐备**；D5 成本分级语义在案（:17-19：index 级零成本不计，render/probe 级逐笔计入）——唯缺逐笔成本生产者 |
| 6 | 上游缺口 1：执行回执 | `agent/internal/agentloop/runner.go:558-567` | execRecord 九字段（status/tool_call_id/agent_action_id/tool/command_name/preview/undo_label/result/error）——**无 elapsed/cost 键** |
| 7 | 上游缺口 2：工具响应 | `agent/internal/harness/harness.go:161-173` | InvokeResponse 结构**无耗时字段**；计时只进日志（见 §2 源 1） |
| 8 | 工件面 | `coord/runs/G3-ATTRIB-1/20261009_214138_harnessab/harness_ab_metrics_pull.json` | cost 对象四键 `{"prefix_bytes_total":582912,"dynamic_bytes_total":15276,"messages_total":103,"llm_calls":17}`——**probe 成本键不存在**；"unknown"指账本 ProbeCostKnown=false（执法面），非工件字段值 |

## 2. 可计量源盘点（区分"已有未接线"与"根本没有"）

### 2a. Agent 侧（三源，全部现成）

| # | 源 | 锚点 | 形态 | 分类 |
|---|---|---|---|---|
| A1 | `Harness.Invoke` 每工具墙钟 | `harness.go:879-892`（`started:=time.Now()`+defer 日志 `total_ms`） | log-only（`[timing] harness.invoke total_ms=…`，run 日志实测在案如 pull 日志 :37） | **已有未接线·通用**（覆盖全部工具含 probe 面） |
| A2 | `message_loop.logTiming("message_loop.tool", …)` | `message_loop.go:66`（定义）；调用点 :314/:328/:408/:679/:798 | log-only（`[timing] message_loop.tool ms=… tool=…`） | **已有未接线·loop 级**——与 settleBatch 结算单元（批内工具回执）同粒度，最自然接线源 |
| A3 | `CollectL2RenderProbeBatch.ElapsedMS` | `harness/l2_probe_batch.go:38`（字段）+:47（`started:=time.Now()`） | **结构化** int64 `elapsed_ms`——L2 render probe 批整墙钟，正是 D5"render/probe 级"目标类 | **已有·结构化·组合层丢弃**（消费方 ccb_masking_observation.go / c1 :169/:501 / b4 :334 均不透传进工具结果 map——grep 零命中） |
| A4 | `mergeFrequencyAssemblyTiming` elapsed_ms | `mixboard/mixboard.go:1021-1028`，注入点 :251（AssembleFrequencyContext，:193 定义） | 结构化，进 FrequencyContext.Assembly（非持久化观察面；c1_frequency_cleanup_runtime.go:150 消费） | 已有·结构化（局部：频率装配面） |
| A5 | 反向确认（无源面） | fxm 包（types/projection 纯投影零计时）；audition 面零计时；mixboard WriteResult（:163-171）无耗时；kernel_prepared 物化器（harness.go:4262-4278）无计时 | — | 根本没有（这些面如需计量须新增） |

### 2b. 内核侧（VitApp，两处日志形态计时+反向确认）

| # | 源 | 锚点 | 形态 | 分类 |
|---|---|---|---|---|
| K1 | L3 源读租约等待/reader open 耗时 | `VitApp/Source/Service/L3AcousticAnalyzer.cpp:952-961`（`writeDiagLog` wait_ms/elapsed_ms） | diag 日志（非 VSP/JSON 遥测） | 已有·日志形态 |
| K2 | WaveformEnvelopeBake 各阶段耗时 | `VitApp/Source/Service/WaveformEnvelopeBaker.cpp:672-693`（`logPerf stage_ms=…`） | perf 日志 | 已有·日志形态 |
| K3 | VitDeltaProbe 事件时间戳 | `VitApp/Source/Core/VitDeltaProbe.cpp:106-126`（DeltaEvent.timestamp） | 事件时序非成本计量 | 不适用 |
| K4 | 反向确认（无源面） | AuditionPreviewService 仅媒体 `duration_seconds`（:30）非执行成本；JobEventService 零时长字段；ClipService/ImportService 命中为参数名（min_strip_duration_ms 等）非计时；VSP 回执/内核遥测 JSON 结构化输出 grep `elapsed_ms/duration_ms/render_ms` 零命中（附录 B 口径） | — | **根本没有**（内核渲染物理成本入 VSP 回执=新面，见 §3 分层） |

## 3. 重开条件①边界结论

**结论：有源，最小接入面成立。** "probe 物理成本"在本架构呈两层，非互斥争议而是粒度层级：

- **层 (a) agent 工具墙钟（充分层）**：对每个工具执行记录 `elapsed_ms`（A2 的 started/time.Since 同款计时已存在，只差写入 execRecord），settleBatch 按 D5 分级把 probe 类工具（ccb.observation_request / mix.observe 等）耗时逐笔累加进 `ledger.probeSpent`，`probeCostKnown` 保持 true。**改动面≈2 文件**（`runner.go` execRecord 增键 + `pull_session.go` settleBatch 接入；budget.go 零改动——账户/执法/披露面已齐）。此层即可支撑 G3 重开条件①的"真实成本对比"：两模式同工具集同粒度墙钟可比。
- **层 (b) probe 级结构化测量（精化层，可选）**：A3 的 ElapsedMS 已是 probe 类结构化测量，组合层透传（返回 map 增 `probe_elapsed_ms`）即可让特定 probe 工具携带真测量；与层 (a) 叠加可校准"等待/传输 vs 内核工作"占比。
- **层 (c) 内核渲染物理成本（扩展层，非必要）**：K1/K2 的 perf 日志已量化源读/bake，但"音频渲染 CPU/时长"入 VSP 回执需新内核遥测字段——列机会面，**非重开条件①必要项**。

**口径分层注记（停止条件检查）**：层 (a) 是层 (b)(c) 的包含上界（墙钟=内核工作+等待+传输），三层递进无冲突，不构成"物理成本定义存疑"的设计争议——不触发停止条件，本报告如实分层呈现即边界结论。G3 重开条件①的另一半（轮级调用优化=会话内压缩/摘要策略）不在本卡范围。

**如实边界**：层 (a) 计入的是 agent 观测的调用墙钟，含 LLM 网络往返以外的调度噪音；若未来需要"纯内核渲染"口径，走层 (c) 新遥测，勿用层 (a) 冒称（与账本"不填 0 冒充免费"同一诚实原则）。

## 附录：可复跑命令（Git Bash，2026-10-10 实跑口径）

```bash
# A. 账本与接线点
cd /d/Vit_DAW/agent
grep -n "probeCostKnown\|probeSpent" internal/agentloop/pull_session.go | head
sed -n '411,430p' internal/agentloop/pull_session.go   # settleBatch 预声明注释
grep -n "ProbeCost\|Disclosure" internal/pullharness/budget.go | head

# B. 内核结构化时长面反向确认（零命中=无源）
grep -rn "elapsed_ms\|duration_ms\|render_ms" /d/Vit_DAW/VitApp/Source --include="*.cpp" --include="*.h" | grep -v "min_strip\|min_silence" 
#   实跑命中：L3AcousticAnalyzer.cpp:960（diag 日志）+ WaveformEnvelopeBaker（perf 日志）两文件，无 VSP/JSON 结构化面

# C. agent 结构化 elapsed 生产点
grep -rn "ElapsedMS\|elapsed_ms" internal/harness internal/mixboard --include="*.go" | grep -v "_test"
#   实跑命中：l2_probe_batch.go:38 + mixboard.go:1021（另有 strip_silence 参数名）

# D. G3-ATTRIB-1 工件 cost 对象
python -c "import json;d=json.load(open('coord/runs/G3-ATTRIB-1/20261009_214138_harnessab/harness_ab_metrics_pull.json',encoding='utf-8-sig'));print(json.dumps(d['modes']['pull']['cost']))"
```
