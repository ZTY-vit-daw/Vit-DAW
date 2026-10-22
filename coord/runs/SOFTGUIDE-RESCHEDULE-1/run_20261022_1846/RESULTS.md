# SOFTGUIDE-RESCHEDULE-1 真栈腿结果（B2 臂 F-gate ×2 止损全停）

- 执行：2026-10-22 18:46–19:06 +0800；owner=GLM-5.3-flash L1 PC ZCode 执行会话（159 卡会话，续接领取 ec659193）
- 协议：[PREDECLARATION.md](../PREDECLARATION.md)（0078e2ec 跑前锁定，跑后未改）+ [146 §E 原协议](../../../runs/146-SOFT-GUIDE-EXP/20261020_b2arm/RESULTS.md) + [156 设计稿 §3](../../FS-FRONTIER-OBSERVE-GAP-1/DESIGN.md)
- 栈占用：登记 373e4064（18:44，本卡分支 push 面，登记通道偏差已在登记行声明）；释放=本 RESULTS 同提交（19:0x，端口/进程复核 0 在卷）
- run=**run_20261022_1846**（本目录）

## 0. 被测与环境（§9 契约）

- agent=worktree 构建 `D:/Vit_DAW_wt_sgr1/agent/bin/VitAgent.exe` SHA256 前 16=**E7AE99B31F5B01E1**（2026-10-22 10:17 构建；生产码=基线 e101e19a 零改动，157/152/158/154 已合入，惯例段生产在场 carriers `semantic_topology_convention`）；vsphub 前 16=**CBECC9897175BAB6**
- kernel=主检出 K3 Release SHA256=**722A79D7**A3F53797…（18:39 哈希亲核，只读复用；VIT_PROJECT_XML 指 run 目录 kernel_workspace 副本，主检出/staging workspace 零触碰）
- 手动起栈三进程（bringup_sgr1.ps1）：kernel 27120 / vsphub 25248 / agent 19180；19:06 拆栈，复核 tcp 5555/5556/7878/8787/4445+udp 4445/5555/5556 全 free、四类进程=0
- 模型=deepseek-flash：**42 次调用全 deepseek-flash、0 error**（遥测在卷）；≤90 预算内
- git：分支 port/softguide-reschedule-1@**373e4064**（生产码零 diff vs e101e19a）；工作树实现面=驱动/工件非生产码

## 1. 结论（机械计数，判读交主管）

- **止损②命中全停**：F-gate 累计 **2**（A2、A5，同签名 target_identity 边界自报）→ 预声明 §4.1「第 2 次 F-gate→全停」逐字触发。B2 臂停于 **6/10**（A1–A5、B1 已跑；B2/B3/B4/B5 未跑）；B1 对照臂 5 run 未跑（全停优先）。
- **主判据如实计数**：F-gate=2；提案面到达率=**4/6**（A1 插件选择卡、A3/A4/B1 总线执行；A2/A5 未达提案）。
- 硬判据（§E 逐字）：**0 幻觉拓扑**——全部装载/send 目标（1032/1022/1027/1037+Pro-R/Timeless 3）均为观察披露轨集或本提案新建；逐轮 track.send.list 读回亲核（send_list_after 在卷）。
- 总线拓扑执行：**3/6**（A3 Reverb Bus+Pro-R、A4 Delay Bus+Timeless 3、B1 Reverb Bus+Pro-R；均 @-6dB 默认发送量）——其中源轨正确 **1/3**（B1 vocals/1032），域漂移 2/3（A3 落 other/1022、A4 落 piano/1027）。
- 其余分类：F-inline 0（A1 为 inline 主张但零装载，主张面注记）、F-notopo 0、F-halluc 0、F-env 0；驱动缺陷轮 0（三批次驱动 exit 0 全捕获）。

## 2. F-gate 签名与机制面证据（157 修复失效证据候选，上交）

两轮同签名（events turn.completed 原文在卷）：「自由态推理申报了能力边界（…<指名轨>…没有出现在任何已披露的工程事实中/无法圈定…）任务在能力边界停止：没有执行任何实验或修改，也不会自动重试。」

- **A2（钢琴）**：自身 loop state **无 user_named_target**（「钢琴」不在 157 闭集别名表——表仅人声/伴奏/鼓组三族，`free_state_reasoning_loop.go:754-758`）；本轮观察供应不含 piano 候选 → 模型拒绝猜测 → 边界。=**prepare 解析侧闭集边界**（157 设计内 fail-open）。
- **A5（人声）**：自身 loop state **user_named_target=1032 已解析**（prepare 侧修复生效），但 **A5 全部 4 行渲染 prompt 均无 user_named:1032 frontier 披露行**（对照 A1 同值 1032：渲染 rows1-2 明确在场 `user_named:1032 tracks=1032. The runtime resolves this frontier only through target-level evidence…`）→ 模型声明「人声轨的身份没有出现在任何已披露的工程事实中」→ 边界。=**fold/披露面未把已解析指名轨送达模型决策点**（A1 与 A5 同 unt 同 fixture，披露面一有一无）。
- 146 信号②（`free_state_final_gate` 弹回行）两轮均 **0 次**；admission receipt boundary=admitted（信号①不成立）——本签名走 152 后的诚实边界出口（信号③），非 146 的 4 连弹回形态；回复面文案为分层诚实边界陈述（152 生效面）。
- 判读交主管：157 修复「失效」或「闭集裁定内的已知剩余缺口+fold 供应不稳定」，属决策域；本回执只固定机械事实。

## 3. 分类总表（RUNBOOK §5 机械口径）

| 轮 | unt 解析 | 惯例段 | 终局 | 装载/拓扑 | 分类 |
|---|---|---|---|---|---|
| B2-A1 | 1032 ✓（渲染披露在场） | 在场×4 | waiting_clarification 停泊（选择卡应答后无候选续行） | 插件选择卡=提案面到达；inline 主张（插入人声）；零装载 | **到达+inline 主张+停泊注记** |
| B2-A2 | 缺（钢琴不在闭集） | 在场×7 | 能力边界自报，completed，零变更 | 无 | **F-gate #1**（target_identity） |
| B2-A3 | 1012 ✓（渲染披露未见） | 在场×11 | completed | Reverb Bus/1037+Pro-R；send **other/1022**→bus @-6 | **到达+总线执行+域漂移注记** |
| B2-A4 | 缺（吉他不在闭集） | 在场×15 | completed | Delay Bus/1037+Timeless 3；send **piano/1027**→bus @-6 | **到达+总线执行+域漂移注记** |
| B2-A5 | 1032 ✓（渲染披露未见） | 在场×19 | 能力边界自报，completed，零变更 | 无 | **F-gate #2**（target_identity）→**全停** |
| B2-B1 | 1032 ✓（events 面候选在场） | 在场×25 | completed | Reverb Bus/1037+Pro-R；send **vocals/1032**→bus @-6 | **到达+总线执行（源轨正确）** |

- 每轮惯例段在场=逐变体 prompt_render_snapshot 亲核（「总线拓扑」出现次数见表；146 attempt-2 补丁失活教训的机械面，B2 条件全程成立）。
- 未跑：B2-B2/B3/B4/B5（全停）；B1 对照臂（全停优先；本腿判读已锁硬规则分支，对照非判据面）。

## 4. §8 记账

- LLM=42 次调用（authority 探针+6 变体链）全 deepseek-flash、0 error、0 超时；fixture 构建零 LLM（确定性 stems+import+DAD ready 轮询）。
- 轮次：6 产品轮全部有效捕获，0 驱动缺陷轮，0 环境中断，0 原样重跑（A3 复核轮=预声明 §4.1 命令的新变体复核，非原样重跑）。
- 内核增量：run 目录 kernel_workspace 副本内（Logs 类），主检出/staging workspace 零触碰。

## 5. 诚实边界

1. B2 臂 6/10 中断：到达率 4/6 与总线 3/6 为**中断面计数**，非 10 变体完成面统计；不构成对惯例路由效应的任何判读（预声明 §2）。
2. agent_last.log 为 800 行滚动日志，起栈至 A1 早期行可能已转出（快照=19:06 时点 78KB 在卷）；判定面证据以逐轮 chat/events/runtime_status/prompt_render 快照+遥测为准（146 §5.4 同界）。
3. message_loop_debug 未启用（未设 env）：弹回/循环内部面证据以 events+render 快照为界。
4. 域漂移 2 轮（A3/A4 源轨错位）归因面（157 闭集 vs 162 缺位[本分支基线 e101e19a 不含 1495bb92] vs 观察供应非平稳）交主管；本分支按领取基线运行，未混入 162。
5. A3/A4/B1 的 walk 面应答出现「这个交互已处理或已过期」（驱动对 runtime 停泊卡的兜底应答落在过期卡上）；执行证据以内核读回（fx_tracks/send_edges/rack after）为准，不以 walk 应答文本为据。
6. 端测边界（预声明 §8）：本腿=agent HTTP/ZMQ 面 chat 链真栈（真实内核+真实 deepseek 链+真实工程副本）；**不含** Godot GUI 渲染面与 webui 渲染面断言；提案面到达判定以 chat 响应/events/内核回执工件为据。
7. 提案面到达的「终答计划文本」形态本轮未单独判定（到达轮全部为卡/执行形态），如实声明。

## 6. 工件索引

- `driver/arm_B2/arm/{A1,A2,A3,A4,A5,B1}/`：prompt.txt / chat_1.json / hop_*.json / walkhop_*.json / events.json / send_list_{before,after}.json / project_state_after.json / runtime_status_final.json / variant_summary.json / prompt_render_snapshot.jsonl
- `driver/batch1_A1A2_console.txt`、`driver/batch2_A3_check_console.txt`、`driver/batch2_A4_console.txt`、`driver/batch3_A5B1_console.txt`
- `driver/fixture/`：六轨 stems+softguide_fixture.vit（M2 同源配方，146 §一.B）
- `logs/`：agent_last.log（+`.snapshot_1906.txt` 时点快照）、agent_llm_telemetry.jsonl、agent_prompt_render.jsonl、vsphub_last.log、MaterializeMetrics.log、pid 三件、kernel_sha256.txt
- **留盘不入库（RUNBOOK §4 重件面）**：`logs/BridgeReplies/`（14 件×1.3MB 重复 plugin_list 桥回执转储，非判定面）、`driver/fixture/fixture_stems/`（六轨 wav 1.2MB，配方=驱动 STEM_SPECS 确定性可再生）、`driver/fixture/.vit_history/`（运行时会话态 1.8MB）、`driver/fixture/.vit_derived/`（派生分析缓存 320KB）——均在本 run 目录原样保留可回指
- prep 件：PREDECLARATION.md（0078e2ec）/ RUNBOOK.md（07c3d814+fbbee7a9）/ bringup_sgr1.ps1（373e4064 修复）/ kernel_expected_sha256.txt

## 7. 执行侧缺陷与偏差如实记录

1. bringup_sgr1.ps1 两处点火前修复（373e4064，零副作用）：①Start 动作 `$logs` 先用后赋致点火必中止（内核未启即抛参数绑定错）；②未显式设 `VIT_AGENT_PROMPT_RENDER_PATH`（`messageLoopPromptRenderPath` 走 `DefaultTelemetryPath()` 忽略 telemetry override，不补则逐变体快照采集落点与 RUNBOOK 不符）。
2. RUNBOOK §3 批次示例 out-dir 笔误（B2 批示例写 `arm_b1`）：执行按 `arm_B2`/`arm_B1` 区分，无协议影响。
3. 栈登记通道偏差：PROTOCOL §2.2 要求「成功推 main」，主检出 main 与 origin/main 真分叉（本地 20 未推/origin 10 未含，含主管验收 f9f96ddf），本卡不可处置——按用户直派口令以本卡分支 push 为登记可见面（登记行已声明），释放/验收时随分支合入 main。
4. RUNBOOK 批次序执行偏差：批 2 拆为 A3（止损复核轮）+A4 两跑——预声明 §4.1 的暂停-复核流程所致，非自由裁量。
