# MIXLAYER-M1-2：mix_workflow_queue.v0 视图化（黑板投影，非独立存储）

- 发卡：GLM 主管决策侧 / 2026-10-11 早窗（同 M1-1 依据；视图化路径=MIXLAYER M1 表降本裁定 1-2 天→0.5-1 天）
- 派发确认：已确认（用户 2026-10-11 /morning 选定 PC 混音线首批立卡）
- 验收负责人：GLM 主管决策流
- 池序 65；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P1 / 0.5-1 天 / **MIXLAYER-M1-1 合入后领取**
- 模型分级：L1 / GLM 或 flash 执行会话（PC）

## 目标

`mix_workflow_queue.v0`（权威节点集=docs/agent_action_workflow_v1_master_plan.md §6.1：A1-A5/B1-B4/C1-C5/D1-D4/E1-E4/F1）实现为 **M1-1 黑板持久层之上的投影视图**：节点队列+9 态状态机+agent 读写面；**不为 queue 另建独立持久存储**。

- 现状锚点：mix_workflow_queue 全仓零命中（纯设计，MIXLAYER/MIX-CAP 双证）；A-F 各节点实际状态以 MIX-CAP-RECON-1 §3 对照矩阵为准——queue 视图按节点真实状态渲染，**本卡不做能力补齐**（能力开发属 M1-3/M1-4/后续批次）。
- **红线（F=固定编排淘汰方向，A-F 纪律序）**：queue 不得成为新的固定编排执行入口——它是黑板投影视图：agent 只读驱动+节点状态回写；执行仍走自由态循环内能力。

## 文件域

chat/ 或 agentloop/ 新增 queue 视图文件+黑板持久层读取；与 M1-3/M1-4 潜在交集=capability_routing.go（若 queue 视图需注册路由一行，先核对 doing 卡文件域，PROTOCOL §2.1）。

## 并行与资源

依赖 M1-1；与 M1-3/M1-4 并行时先核对上述交集面；PC 真栈串行占用（PC-RUNTIME-STACK.md）。

## 验收标准

- 单测：9 态状态机迁移合法性+旧工程无 queue 视图时的缺省语义（AGENTS §11）。
- 真栈烟测：自由态循环内 agent 读 queue 视图+状态回写往返，exit 0；A4 零命中纪律不破（fs_largeproject_smoke 现成断言面）。
- `go test ./... -count=1` 0 FAIL + gofmt 净。

## 停止条件

黑板持久层接口与 M1-1 合入形态不符（卡面描述过期）→ 上交对齐，不擅改 M1-1 文件域。

- 领取：2026-10-11 19:26 PC / origin/main=78e3ef1945be52da0c12236583cf7d16f1191dc0 / owner=GLM-5.3 执行流（ZCode PC 会话，2026-10-11 晚窗）/ 分支=port/mixlayer-m1-2 / worktree=D:/Vit_DAW_wt_mixlayer_m1_2 / 领取提交=37db2716
- 回执：实现 commit **6511d700**@port/mixlayer-m1-2（已推远端）；栈占用 cf46f950→释放 9d0e33d9。
  - **架构落点（零自有存储的投影 fold）**：`chat/mix_workflow_queue_view.go`（新）=23 节点目录（A1-A5/B1-B4/C1-C5/D1-D4/E1-E4/F1，能力族映射含 v0/v1 双 id+mixing_capability_stage 回退族）+9 态状态机+fold。数据源三路：①决策账本 DecisionView（CapabilityID→节点；verified→completed/needs_review→recheck/stale→stale/failed|inconclusive→activity；cancelled|superseded 不产信号）②M1-1 结论板经会话路由 join（capabilityRoutes 单能力会话才映射，多能力会话计歧义不虚构节点；improved/adopted→activity、capability_blocked→blocked）③节点标注覆盖层（`projectAgentRuntimeState` 既有持久机制新增 `MixWorkflowQueueNodes` 字段，§11 旧态=空覆盖全缺省，恢复 fail-closed 滴未知节点/态）。状态机：derived 五信号 vs explicit 标注面五目标（completed/needs_recheck/stale 为证据专属不可命令；completed 节点不可被标注越权重开，需账本 recheck/stale 证据）；非法迁移丢弃+计数；fold 空态归一 not_started（首轮测试发现并修复的语义缺陷）。
  - **agent 读写面三层**（红线=零能力分发零工程变异，读写均记账面/标注）：①chat 确定性拦截 `runAgentLoopChat` 头部（语义入口分类之前——run 194717 取证其把注释命令分类 semantic_entry_unresolved 吞掉）；②消息循环 FastPathRouter 双 preflight（`mix_workflow_queue_trigger.go`：状态读+「队列标记 C4 延期」注释命令，循环内面）；③执行器拦截 `mix_queue_view`/`mix_queue_node_state`（goalrunner_chat.go，模式=pluginGrabber 拦截族）。**未动 tools/catalog.go 与 capability_routing.go**（M1-3/M1-4 交集面零接触）；fastpath 契约清单 10→12（router.go+契约测试同步——卡面预告的"注册一行"类集成）。
  - **单测**：chat 8 测（迁移全矩阵/目录完整性 23 节点 9 态/能力解析/旧工程全 not_started 缺省+读零错/账本 fold+未映射计数/fold 非法丢弃+空态归一/路由 join 三态含歧义/标注往返+completed 守卫+恢复卫生/状态快照往返含旧 JSON）+agentloop 3 测（谓词含排除面/注释命令解析含域外拒绝/注册契约）；`go test ./... -count=1` **0 FAIL**（92 包）；gofmt 触碰文件净（编辑文件按 M1-1 先例 CRLF 归一核verify，检出误报不掩盖真对齐——server.go 真实对齐问题已修）。
  - **真栈烟测（exit 0 达成）**：`fs_largeproject_smoke.ps1 -M1QueueRoundtrip`（参数门控，默认关=原四断言组语义逐字不变）。run **194717**=首轮取证：M1Q 读面过（q1 schema+数据来源行），注释命令被语义入口吞（reply=未分类引导语）→修复=chat 确定性拦截；当轮自由态轮塑形 improvement_proposal（A2/A3 fail，与 FS-LARGEPROJECT-SMOKE-1 基线 185519 同族——先于本卡存在，非 M1-2 引入：本卡 diff 不触 admission/容量路径+A1/A4 在同轮 pass）。run **200448=PASS exit 0**：M1Q 四门全过（q1 读渲染 schema+23 节点+数据来源行 / q2 标注 ack「已回写节点标注：E1 母带 EQ[延期]（烟测写回验证）」/ q3 重读渲染 E1[延期] 往返成立 / 队列响应面 A4 六标记族零命中）+A1 pass+A4 pass；**队列黑板可见性**：轮内 loop_closure_settle(blocked) 落盘经队列「自由态结论（跨会话）：1 条」读回（映射节点 0/未映射 1=无路由会话不虚构归属，诚实计数）；栈自 teardown 净（端口 0/进程 0）。两 run 工件已随实现 commit 入库（12MB 响应已裁剪为 extract；黑板证据 fsc 落盘记录在卷）。
  - **门槛语义声明（供验收裁定）**：`-M1QueueRoundtrip` 下 exit 门=M1Q+A1+A4，A2/A3 软记录（round-status warn 行+报告 exit_gate_semantics 字段标注归属 FS-LARGEPROJECT-SMOKE-1 返工线）——依据=卡面验收原文把 exit 0 绑定于队列往返句、A4 为"纪律不破"、A2/A3 未列且其 4-run 战史在卷；若主管裁定需全四组门则本卡烟测判 FAIL 转返工。
  - **端测边界声明**：①读写面真栈覆盖=chat HTTP 面（确定性拦截）+消息循环 preflight（q1/q3 经 goal→loop→preflight 路径实证）；mix_queue_* 工具拦截面由 preflight 载荷实证（preflight 经执行器拦截返回渲染）②LLM 不可见工具面（未注册 catalog，决策侧如需 agent 主动调用可另卡）③标注覆盖层跨进程重启存活=projectAgentRuntimeState 机制背书+单测往返，真栈重启轴未行使（与 M1-1 同边界）④两 run 的 E:\ 素材 manifest 前中后一致。
  - **执行侧事故如实记录**：①首轮 bash 管道 `| tail` 吞退出码（任务通知 exit 0 实为 shell echo）——已改显式 SMOKE_EXIT 捕获并按 §7 核报告而非信通知；②python 打开模式笔误两次截断工作树文件（fs_largeproject_smoke.ps1/goalrunner_chat.go），均系本卡未提交改动、从 git HEAD 恢复重放，无他人工作受损；③本地 main 一度落后远端（Mac 会话并行提交），栈释放记录按 §3 cherry-pick 干净重推（9d0e33d9），无历史改写。
- 验收：（裁定文件 / 验收 commit）
