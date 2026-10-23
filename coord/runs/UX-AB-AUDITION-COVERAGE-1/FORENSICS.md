# 取证报告：UX-AB-AUDITION-COVERAGE-1（卡 177）——场次⑤「AB 卡未出现」形态归因

- 执行：GLM-5.3 L2 PC 执行会话（ZCode，177 卡会话）/ 2026-10-23
- 基线：origin/main=08588c8c
- 性质：取证锚定（卡范围①），零生产码改动；证据摘录见同目录 evidence_extracts.md（源文件只读，行号锚定）

## 0. 结论（TL;DR）

1. **未出现那次的会话身份已锚定：`webui_mvjr15zw`（2026-10-23 00:21:36–00:23:42），不是卡面估计的 00:27–00:34 窗口**——该窗口实为内核2死亡（00:27）到内核3启动（00:34:52）的空档，两份 prompt render 日志、内核日志、message loop debug 均零记录；用户的话术第三次输入即 mvjr15zw（00:21–00:23），卡面时间窗系事后回忆偏差。三轮身份：mvjr15zw（00:21–00:23）/ mvjr5hcm（00:25–00:26:33，AB ✓）/ mvjriarj（00:35–00:36:52，AB ✓），与 174 回执记载的两次出现时刻（00:26:33/00:36:52）逐一对上。
2. **卡面怀疑的旁路路径实锤**：三轮（含两次「出现」的轮）的效果类结构突变**全部**走非 ticket 直发路径——`track.send.connect`（auto_create_fx_track="Reverb Bus"）+ `rack_add_node`（Pro-R 装载到 FX 轨），经 `executeSemanticTopologyBusLegs`（plugin_grabber_workflow.go:921 起）以 `harness.Invoke(Source:"chat", Confirmed:true)` 派发；journal source=chat；**零 audition bracket**。挂接单点：`executeSemanticTopologyBusLegs` 内两腿（connect+load 同组），其两个调用入口（server.go handleConfirm 确认续行 + full_access_semantic_chain.go:192 完全访问链）都被该单点覆盖。
3. **ticket 路径三轮都在场**：每轮直发两腿落地后，紧随一条 semantic_dynamic_execution ticket（dyn_3bb825c45a53c3bf / dyn_7d48faeeb0128811 / 轮3 同族）执行参数写并按 174 的 bracket 挂出 audition（内核侧 audition.prepare + 双 render 资产在卷，轮1 的 before_revision_10.wav/after_revision_11.wav 落盘于用户工程 artifacts）。**即：未被 bracket 覆盖的是「send 路由+插件装载」这两腿（听感主增量），参数写腿一直有 audition。**
4. **轮1 用户未见卡 ≠ bracket 缺失的唯一直接后果**：轮1 的 ticket audition 内核侧确实挂出（00:23:26）且后续有点播/重落座（00:23:29–42，含一次 `select refused … session_bound=false` 后 reseat 再发 audition.ready）；轮1 持久会话终态停在中间态措辞（「我还在继续处理这个任务…」），完成报告未落入会话流（174 回执已上交的同族投递边界问题：turn_trajectory_events.go:325-372 调度续跑边界只送 Reply 文本）。**live 事件流不落盘，webui 渲染差异（轮1 未见卡 vs 轮3 见卡）无法从盘面完全判定**；但结构事实独立成立：直发腿在每一轮都产生全链最大的听感增量（-6dB 默认发送 + Pro-R 装载）而**从未被任何 audition 覆盖**——若直发腿有等价 bracket，轮1 用户在 00:23:19 两腿落地当时即拿到 A/B 卡，不依赖后续 ticket 的事件投递形态。**修复目标因此成立且有效。**
5. 轮1 会话由 174 拆栈后的遗留 agent 进程服务（174 回执「活体已被未登记手测形态新占用 00:21 起」即此轮；其 agent_last.log 记录了 webui_mvjr15zw 的 loop completed stop=semantic_dynamic_execution_completed executed=3）。

## 1. 轮1（mvjr15zw）完整时间线（本地时间 +08:00，goal_04f63fd2b7f8ca38 / run_bc7ddfef5f890245）

| 时刻 | 事件 | 证据 |
|---|---|---|
| 00:21:36 | Godot 前端连内核1（realtime_adapter session） | 内核1日志 |
| 00:21:41 | 新 draft 工程 draft_20261022T162141_61d037c5（Unsaved.vit） | .vit_agent manifest |
| 00:22:19–00:22:47 | 观察轮（ccb.observation_request / catalog） | journal |
| 00:22:35 | final gate 拒 needs_observation（closure frontier 规则） | message_loop_debug |
| 00:22:57 | final gate 拒 needs_experiment（materiality 对象缺失） | message_loop_debug |
| 00:23:06 | version.checkpoint（free_state_experiment，confirmed） | journal |
| 00:23:10 | semantic_topology_plan: track.send.list + plugin.semantic_search("reverb") | journal + 内核1 |
| **00:23:19** | **直发 track.send.connect（auto_create_fx_track="Reverb Bus"，source=chat）+ rack_add_node（Pro-R→track 1037）** | 内核1 line 292393/297383 + journal（revision 8→9） |
| 00:23:25 | semantic ticket dyn_3bb825c45a53c3bf 执行 plugin.set_params_batch（decay） | 内核1 line 380348 + journal（revision 10→11） |
| 00:23:26 | ticket bracket 挂出 audition:dynamic:dyn_3bb825c45a53c3bf；loop completed stop=semantic_dynamic_execution_completed | 174 遗留 agent_last.log line 484/485 |
| 00:23:29–00:42 | audition.select a / stop / select b；一次 `select refused … session_bound=false` → reseat → audition.ready 再发 | 内核1 line 400454-402087 + agent_last.log line 489/490 |
| 00:23:42 | 持久会话终态=中间态措辞（完成报告未落入会话流） | agent_runtime_state.json saved_at 00:23:42 |

轮2（mvjr5hcm，内核2）同形态：00:26:26 直发两腿（goal_3d4245383bfd6d72）→ 00:26:32 ticket dyn_7d48faeeb0128811 → 00:26:33 audition（=174 回执「00:26:33 在卷」）。轮3（mvjriarj，内核3，goal_1a0ee9e570e6b5bb）同族（00:35–00:36:52）。

## 2. 旁路路径清单（效果类突变 × 执行路径覆盖面）

### 2.1 本卡修复面（取证实证行使 3/3 轮）

| # | 路径 | 命令 | 挂点 | 状态 |
|---|---|---|---|---|
| P1 | plugin_grabber bus 拓扑两腿（finish 期派发） | track.send.connect(auto_create) + rack_add_node | `agent/internal/chat/plugin_grabber_workflow.go` `executeSemanticTopologyBusLegs`（单点覆盖 server.go:5020 确认续行与 full_access_semantic_chain.go:192 完全访问两入口） | **零 audition（本卡修复）** |

### 2.2 未行使面（本轮不修，逐条上交主管裁定是否另开卡/扩域）

| # | 路径 | 锚点 | 说明 |
|---|---|---|---|
| P2 | plugin_grabber 单轨 insert 装载（decisions 执行期，非 bus 模式） | server.go handleConfirm executeDecisions / full_access_semantic_chain.go pluginGrabberLoadFullAccessResponse executeDecisions；finish 期只做只读抓参 | 装载发生在 finish 之前，before render 挂点=executeDecisions 前（Workflow==pluginGrabberLoadCommand 分支判定可行）；场次⑤未行使 |
| P3 | sendTopologyConnectWorkflow 直连 | server.go:5016 finishSendTopologyConnectWorkflow | send 路由突变非 ticket 路径；场次⑤未行使 |
| P4 | C1/B4/C3/C2 运行时装载 | c1_frequency_cleanup_runtime.go:361 / b4_eq_runtime.go:254 / c2_dynamic_plugin_load.go:123 / c3_space_depth_runtime.go | 各族治理装载，contract 版本串里带 plugin-load:rack_add_node；场次⑤未行使 |
| P5 | goalrunner 装载面 | goalrunner_chat.go:1978 plugin.load_to_rack 族 | 场次⑤未行使 |
| P6 | agentloop 原始工具直发 | agentloop/message_loop.go 等（track.send.connect/rack_add_node 工具面） | 场次⑤实际经 chat 面而非该面；与 P1 命令面重叠、入口不同 |

P2–P6 每条的 before render 时机均有可挂点（无「时机不可逆且无等价挂点」形态）——**停止条件未触发**，但按卡面「文件域以取证结论固定」纪律，本轮只修 P1，其余逐条上交。

## 3. 文件域（依取证结论固定）

- `agent/internal/chat/plugin_grabber_workflow.go`（P1 挂点+回复条款）
- `agent/internal/chat/semantic_dynamic_audition.go`（bracket/plan 复用件邻域，必要时抽公共构造）
- `agent/internal/chat/semantic_topology_audition_test.go`（新增测试：P1 路径覆盖防回归断言族）
- 零内核/webui/Godot 改动；`semantic_treatment_strategy.go` 只读（topology plan 字段消费）。

## 4. 边界与诚实声明

- live AgentEvent 流（audition.ready 的 webui 投递）不落盘，轮1「未见卡」vs 轮3「见卡」的 webui 渲染面差异无法从盘面复判；本报告只锚定结构事实（直发腿零 bracket）与修复有效性论证（直发腿 bracket → 两腿落地即出卡）。
- 轮1 的 ticket audition 会话归属（audition:dynamic:dyn_3bb825c45a53c3bf）在 174 回执中被记作其 resume 期第二次执行——按本取证更正：该会话由 mvjr15zw 的 ticket 挂出（goal/run/工程路径三重对齐 mvjr15zw；174 round4 用 aba174 kernel_workspace 工程），174 回执的归属记载有误但不影响其验收面。
- 取证全程零 LLM 复现轮（卡面预声明 ≥5 轮未启用——history 面可回指，四源证据链闭合）。
