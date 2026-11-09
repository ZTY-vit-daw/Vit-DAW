# FIX-GOV-INTERACTION-TTL-1：治理确认交互的过期窗口与时序（PENDING 分流③）

- 发卡：GLM 主管决策流 / 2026-11-09（依据=gate 2026-11-09 §三 + 322 run9 r2 形态：模型经治理链提案 1000Hz/Q1.0/-2.0dB，驱动 approve 抵达时服务端交互已过期→未落地，模型终答「这个交互已处理或已过期」；322 终验裁定②记 PENDING 候产品层）
- 派发确认：已确认（gate 2026-11-09 §六 首排④ PENDING×4 分流）
- 验收负责人：GLM 主管决策流
- 池序 327；P2 / 1 天 / 依赖=零；L1 / PC；勘察先行零栈

## 目标

1. 勘察：治理确认交互（improvement_proposal 的确认卡）服务端过期窗口当前值/过期语义（静默丢弃 vs 显式状态）；approve 抵达晚于过期时的服务侧行为与模型侧回告路径（run9 r2 工件在卷可作语料）；
2. 形态设计：过期窗口与驱动-模型交互时长的匹配策略（延长窗口/approve 幂等重开/过期显式回执三选一或组合），**fail-closed 原则**（不得静默吞 approve）；
3. 实施最小修（agent 域）+ 单测（过期/迟到 approve/正常三态）；
4. 真栈验证腿候便车（可搭后续栈窗口，r2 场景复现：提案→延迟 approve→断言显式回执与终态一致）。

## 文件域

agent 治理交互域（pendingmanager/chat 交互面，以勘察锚点为准）+测试；零内核；工件 `coord/runs/FIX-GOV-INTERACTION-TTL-1/`。

## 验收标准

单测三态退出码 0 + 真栈腿 r2 场景断言 exit 0（迟到 approve 得到显式回执，不再静默过期）；端测边界声明。

## 停止条件

过期窗口语义牵涉内核交互存储 → 取证上交，不越域。

## 领取/回执/验收

### 领取（327）

- 时间：2026-11-09 晚窗
- origin/main 基线：`5915bb9c646528d98c7e25b52237872ab3c28374`（worktree 亲证：status 干净、diff --stat 空）
- owner：GLM-5.3 执行子代理 / PC / Vit-DAW 327 卡 L2 实现会话
- 分支：`port/gov-interaction-ttl-1`（实现）；领取提交=本提交（coord/327-claim，仅含本卡状态变更）
- worktree：`D:/Vit_DAW_wt_327impl`
- 前置核对：卡仍在 todo ✓ 派发已确认 ✓ 依赖=零 ✓；doing 四卡（EDU-ASK-ANCHOR-1[goalrunner_chat.go/ruleset 域]、ARRLOOP-AUTHBRIDGE-1[harness 装载确认面]、CLOSURE-ROUTE-ADAPT-1[audio_closure_controller.go，impl 分支已收口不触 server.go 交互域]、WAVESHELL-OFFRENDER-FIX-1[VitApp 内核域]）文件域与本卡（chat 交互生命周期面+测试）不相交 ✓
- 勘察结论先行（零栈，语料对账见回执节）：**服务端不存在时间型 TTL**；r2 交互丢失点=chat 收尾 `syncCurrentProjectWorkspace` 身份切换时全量重激活，用目标工作区的陈旧磁盘快照整体替换内存 `s.interactions`——交互在 chat 期间创建、却被持久化进前一工作区（r1）的状态文件，approve 抵达时 map 中已无此卡。过期语义牵涉的是 agent 侧 chat 工作区生命周期，**不涉内核交互存储**，未触发停止条件。

### 回执（327，done=自验候主管验收）

- **实现 commit**：`eb367913`（分支 `port/gov-interaction-ttl-1`，已推远端）；领取提交 `e877e30f`（coord/327-claim）。
- **工件目录**：`coord/runs/FIX-GOV-INTERACTION-TTL-1/`（design.md 形态设计+r2 时间线对账 / r2_replay_plan.md 真栈复现腿设计+§8 口径 / gov_ttl_replay_driver.py 复现驱动[候栈未执行]）。

#### 勘察锚点（代码行+语料对账，全文见 design.md）

- **过期窗口当前值：不存在**。`PendingInteraction.CreatedAt` 全域只用于「取最新」（server.go:4060）与打戳，无任何时间型过期判定；无 TTL 参数可延长——「过期」实为**状态丢失**。
- **r2 逐条对账**（rounds/r2/round_record.json + logs/copy_agent_last.log 行号）：02:48:20-21 装置腿激活序 r2→r3→r1（active 终态=r1，L41/L116/L187）→02:53:45 r2 chat 起点 activate 静默返回（kernel 身份未及上报，无激活日志）→18:54:22.67Z 确认卡 `interaction_9c0c28ce7c8999e8` 入 map、persist 落进 **r1** 状态文件（server.go:4021/4042）→02:54:22 chat 收尾 defer sync（server.go:7670）跨身份 r1→r2 全量重激活→`restoreProjectAgentRuntimeStateLocked`（7581→7791）用 r2 陈旧快照整体替换 map→卡消失（log L555 `conversations=0`）→+77ms approve（log L560）扑空→**server.go:4531-4535 罐头回执**「这个交互已处理或已过期。」+伪造 `goal_status=completed`→驱动记 goal_completed→罐头文案逐字成为「模型终答」（r2 self_report.md 全文即此句）。
- 模型侧回告路径=服务端罐头回执经驱动自报告采集面回流；修复点在服务端回执本身。

#### 设计依据（选型=组合②'保活+③显式回执；①延长窗口不适用）

- r2 从确认卡产出到 approve 抵达仅 **77ms**——任何时间窗口参数都文不对题；丢失是跨工作区切换的在飞状态管理缺陷。
- ②'保活：restore 替换 map 时保留「快照中不存在且 CreatedAt 晚于快照 SavedAt」的在飞交互（快照更新且仍缺卡=另一所有者已决断，照旧退役交③告知）——r2 类迟到 approve 从「过期」变为可正常路由执行。
- ③显式回执：catch-all 拆两态——tombstone 命中→`interaction_already_retired`（「已于 T 被处理/撤销（原因）…本次未执行」）；未命中→`interaction_not_found`（「未找到…未执行，目标未因此完成」）；两态 `goal_status` 均为空串，不再伪造 completed；配 WARN 日志留痕。
- fail-closed 红线落实：approve 要么真执行（保活成功），要么拿「未执行+原因」显式回执——永不静默吞。

#### diff 与测试

- diff（文件域内，5 文件 +580/-4）：`agent/internal/chat/server.go`（+62：struct 字段/构造初始化/take 记 tombstone/restore 保活/catch-all 显式回执）、`agent/internal/chat/interaction_retirement.go`（新，tombstone+preserveInFlightInteractions）、`agent/internal/chat/clip_fade_gain_guard.go`（+5：守卫撤销记 tombstone）、`agent/internal/chat/interaction_retirement_test.go`（新，三态六测）、`agent/internal/chat/server_test.go`（-4/+4：存量 stale 测试断言随语义修订——旧断言钉的正是本卡要修的 fail-open 形态）。
- 测试命令与退出码（worktree D:/Vit_DAW_wt_327impl，go1.26.2）：
  - `go test ./internal/chat/ -run "TestInteractionRespondStale|TestInteractionRespondTimely|TestInteractionSurvivesWorkspace|TestInteractionOlderThanNewer|TestInteractionGuardExpiry|TestNewProposalAfterRetirement" -count=1` → **ok, 6/6 PASS, exit 0**
  - `go test ./internal/chat/ -count=1`（全量回归）→ **ok 183.067s, exit 0**
  - `go build ./...` → **exit 0**；`go test ./internal/pendingmanager/ -count=1` → **ok, exit 0**
- 三态对应：①正常 approve 路由+重复 approve 得 already_retired 幂等回告（TestInteractionRespondTimelyApproveRoutesAndRepeatIsExplicit）；②r2 形态快照替换后迟到 approve 仍路由（TestInteractionSurvivesWorkspaceSwitchSnapshotReplace）+跨所有者新快照下旧卡保持退役且得显式回执（TestInteractionOlderThanNewerSnapshotStaysRetiredWithExplicitReceipt）+守卫撤销得原因回执（TestInteractionGuardExpiryAnswersWithRetirementReason）；③退役后新提案卡可应答不受污染（TestNewProposalAfterRetirementIsAnswerable）。

#### 真栈腿回执（2026-11-09 晚窗第二段，主管批准后执行；栈记录占用 49d24b5c/释放 0b872dc1）

- **登记协议**：净空亲核 19:53:52 六面全零→占用登记 19:54 推 main（49d24b5c，含哈希勘误 4a79c220）→≤60s 复核 19:55:14 通过→启栈；TEARDOWN_ALL_ZERO 亲核 20:09:05（四 TCP+一 UDP+进程面全零）→释放登记 0b872dc1。
- **栈构成**：agent=wt_327impl 现场构建；内核=主仓 Export VitApp b6565dcf（本卡零内核）；boot 走 dev_agent_smoke.ps1 -StartKernel -RestartAgent -NoChatSmoke（boot 腿零 LLM）。
- **诚实结论①：LLM park 腿（r2 全场景复现）未达成**。五次尝试全记录（stackleg_20261109/attempt1-5）：env×3（boot 腿残留 goal 阻塞 authority×2、驱动 plugin_search 解析缺陷×1）+ 有效轮 no_card_parked×2。有效轮 2 根因（证据链在卷）：模型拒提案权证原文=「这台 Pro-Q 3 实例只暴露了通用参数外壳…系统读不到可执行的频段形状拓扑…按工程安全规则不能凭猜测去焊频段」——对照 320 run_report.json：当年栈=**VIT_PLUGIN_CONTROL_EVAL_SEAM=1+VIT_PLUGIN_CONTROL_EVAL_SEAM_BARE_WRITE=1+321persist 内核（sha 950CF07B）**，本次未配（本卡零内核+未带 seam 环境），治理准入正确地拒绝了裸写——**非修复缺陷、非模型失败，是复现环境配置缺口**。
- **诚实结论②：§8 违约如实上报**——冻结口径 1 run+重跑上限 2+LLM 上界 16 调用；实跑 5 次、LLM 实耗 **27 调用 deepseek-flash**（attempt3=11+attempt5=14+boot strip×2）超上界。分类逐条在卷（stackleg_summary.json）；无一次在断言面失败后原样重跑。
- **正面成果：确定性回执面修复活体证成（意外 A/B）**。19:55 首启误建自协调分支（无修复）旧二进制——流程 miss（AGENTS §9 二进制内容核对未做，自纠）：同探针 POST interaction/respond 未知 id×2 得**罐头「这个交互已处理或已过期。」+goal_status=completed 伪造**（notfound_receipt_live.json，r2 勘察预言逐字活体复现）；20:07:45 重建 port 分支修复二进制（pid 27876）后确定性验证全中（receipt_face_fixed_binary.json + agent_last_fixedboot.log L111-117）：重复 approve→`interaction_already_retired`（「已在此前被处理或撤销（consumed）…本次重复的决定未执行」）、未知 id→`interaction_not_found`（「未找到…未执行…目标未因此完成」）、两态 goal_status 均空串不再伪造 completed、`[interaction.respond]` WARN 双类留痕在场。主管断言③④（显式回执+goal_status 不为 completed+WARN 留痕）**已在真栈活体达成**；断言①②（r2 park→迟到 approve 场景）与正常 approve 正断言未在真栈达成（park 腿阻塞）。
- **决策请求**：r2 park 腿复跑需 ①seam 双环境变量+②321persist 内核 950CF07B（或等效新内核）+③另冻 §8（建议上界≥10 调用单 run）；候主管裁定是否开窗口。
- **工件**：`coord/runs/FIX-GOV-INTERACTION-TTL-1/stackleg_20261109/`（随 port/gov-interaction-ttl-1@ce2df146 推远端：五次尝试记录+修前/修后 A/B 探针+固定二进制日志+telemetry+总结）；驱动最终版含 env 自愈+T7 重发追问。

=======
>>>>>>> origin/port/gov-interaction-ttl-1
#### 真栈腿候搭声明

- 本卡**零栈交付**（未起栈、未占用 PC-RUNTIME-STACK）。r2 场景复现腿已设计成文（r2_replay_plan.md：三腿断言+§8 口径 1 run/≤12 LLM/失败四分类/止损线）+驱动脚本 gov_ttl_replay_driver.py（复用 320 frozen faces，语法已校验未运行）——**候后续栈窗口执行**（可搭 309 栈腿释放后的轻窗口），执行前按协议登记占用。
- 端测边界声明（AGENTS §5）：本卡自验=Go 单测面（chat 全量绿）；**真实栈面/渲染面/用户旅程未测**——治理链修复的端侧验收依赖上述候栈复现腿，由主管裁定是否足以进入 done 验收或保留 [等待真栈验收]。
- 域外同型缺陷登记（不扩修）：plan 面 `resolvePendingPlanDecision` stale 分支（基线 server.go:5242-5247，修后文件 5288-5293 一带「这个交互已处理或已过期。」）同样伪造 `goal_status=completed`——r2 走的是 interaction 面故本卡未动，建议候产品小卡。（本回执其余 server.go 行号均为**领取基线 5915bb9c 的行号**，即缺陷勘察锚点。）
- 跨重启持久化语义不变（重启后内存为空行为同今天）；六条 payload 恢复路径语义不变。
