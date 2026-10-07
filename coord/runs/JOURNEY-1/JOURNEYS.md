# JOURNEY-1 旅程清单设计（demo 关键旅程烟测体系）

- 卡片：JOURNEY-1（2026-10-04 入池，M1 建设卡）；本文=卡目标 1"旅程清单设计"交付物
- 产出：2026-10-07，PC 执行侧（ZCode 值班会话）；状态=设计定稿+首旅程实现随本卡交付，决策侧复核
- 基线：origin/main=412820cc / 本地 HEAD=f126df68
- 依据：AGENTS §5（旅程门槛层，2026-09-13 用户裁定）+§8（概率性运行纪律）；SMOKE-SCEN-RANGE-1 场景泊位（裁定书明示"为 JOURNEY-1 直接复用资产"）；journey1_demo_journey_smoke.ps1（2026-09-13 老旅程门槛载体）装载勘定①与其 A2 确定性探针先例

## 0. 设计原则（从先例固化）

1. **泊位**：旅程模式复用 dev_agent_smoke.ps1 场景泊位——拒绝复用已监听栈、自起 kernel+agent、finally 必拆（不占用户栈，AGENTS §9）。不启动 Godot（见 §4 边界声明）。
2. **确定性口径**（§8）：每条旅程的判据必须落在**服务端/内核拥有的组装面**（invoke 状态、权威模式、canary_stage、确认卡结构、事件流、投影、持久化文件），不落在 LLM 回复文本。LLM 参与的旅程显式按概率性声明（运行次数/成功条件/失败分类/止损线）或**不进 exit 0 门**（只报告）。
3. **两半结构**（journey1 A2 先例）：真实 NL 旅程回合是模型分支依赖面——只报告；确定性判据由**直接探针/能力链直驱**承载。
4. **隔离契约**（§10/§12）：每 run 独立工件目录；被测工程=run 目录内自建 fixture（不碰用户工程）；内核默认工程重定向（VIT_PROJECT_XML→run 目录副本，journey1 同款）；agent 会话草稿重定向（VIT_HISTORY_DRAFT_ROOT→run 目录）。
5. **增量**：一旅程一卡；本卡只建首旅程，后续按 §2 清单立卡。

## 1. demo 六站点 → 旅程切分总表

demo 主线（卡片枚举）：工程打开→权限授予→自由态实验→插件装载→A/B 试听判定→结算报告。
烟测按"每旅程一个可独立 exit 0 的场景"切分；站点→旅程映射与确定性口径如下：

| # | 旅程 | 站点覆盖 | 起点态 | 关键断言点（面） | 确定性口径 | LLM 参与面 |
|---|------|---------|--------|----------------|-----------|-----------|
| J1 | 首旅程：工程打开→权限→实验单轮（**本卡**） | 站点1-3 的确定性骨架 | 泊位净栈（kernel 带隔离默认工程+agent 新起） | 见 §2 断言表 | **全腿零 LLM**；重试类预声明 | 无（NL 面归 J3） |
| J2 | 插件装载 | 站点4 | J1 同泊位，full access 已授 | rack.add_node 直驱探针：内核回执 plugin_id+plugin_instance_ready+graph_diff=node_add；UI 投影 plugin_count≥1（选轨后）；PCA 门拒绝增量=0 | 确定性（journey1 A2 修后 GREEN 3/3 实证；FULLACCESS-AUTONOMY-1 自主准入） | 无 |
| J3 | 自由态实验（NL 旅程回合） | 站点3 的自然语言面 | J1/J2 泊位或承接其工程 | 组装面：stop_reason（needs_confirmation/improvement_proposal_* 家族）、pending faces（mix_tick_confirmation）、events（capability/experiment 链）、A/B 卡挂出（audition candidate）；**绝不断言回复文本语义** | **§8 概率性声明**：运行次数 N=3、成功条件=至少 1 轮走通提案→确认→应用→A/B 卡挂出、失败分类分记（no_candidate_found/模型纯文本/链停滞/环境中断）、止损线=同代码版本连续 2 轮同形失败即停手上交；exit 0 门只含确定性子面（如确认路由 hop 的 stop_reason） | 有（核心面） |
| J4 | A/B 试听判定 | 站点5 | A/B candidate 在场（承接 J3 产物或实验执行产出的 candidate 面） | /agent/audition/status→select(A/B)→judgment：user_judgment.requested→recorded 事件链；盲听态（VIT_DAW_AUDITION_BLIND）下判定不带标签偏置 | 确定性（HTTP 驱动+事件面）；candidate 在场依赖=J3 或确定性执行产出——**若装卡本身无确定性入口，立卡时按停止条件上交锚点** | 无（判定=HTTP 驱动） |
| J5 | 结算报告 | 站点6 | 判定已 record（承接 J4） | judgment.settled 事件+结算确认消息入会话流（会话图 vit 节点 kind=assistant / logical=judgment_settle:*，SETTLE-DELIVER-1 修复面）+轨迹 settle 面 | 确定性（事件+持久化文件面） | 无 |
| J6 | 会话续写卫生 | demo 收尾（重开工程） | J1-J5 任一旅程结束态 | 重开工程（open_project 同文件）后：旧会话不复活为新活上下文（.vit_history/.sessions 新会话目录干净）、无 stall WARN（journey1 A4 先例；PROJ-OPEN-RESUME-1 域） | 确定性（文件系统+日志面） | 无 |

依赖序：J2 独立；J3 独立（泊位自起）；J4→J3（或确定性 candidate 入口）；J5→J4；J6 独立。
每旅程一卡、一 Scenario 值；`all` 不自动含旅程族（旅程含执行/渲染重腿，独立调度）。

## 2. 首旅程（J1）详设——`-Scenario journey_first`

### 2.1 fixture（run 目录内自建，零外部工程依赖）

WriteLeaseSmoke（VITNOTE-IMPL-4）同款配方：

1. `project.new` → `import_folder_as_stems`（2 条合成 stem：Lead Vocal 440Hz/0.5、Bass 110Hz/0.35，各 2.2s——staticbalance 名字推断词表口径）→ `project.audio_analysis_start` → 轮询 DAD ready（预算 240s）；
2. **analysis ready 后** `project.save_as`（run 目录 project/journey_fixture.vit）——analysis manifest 以 `vit_analysis_manifest_json` 嵌工程状态持久化（CommandDispatcher.cpp:3396），重开可带回；
3. `project.new`（回到空白未命名态=演示"启动页"起点态）→ **open_project(fixture.vit)** = 工程打开腿的真断言对象。

工程打开走 agent 工具面 `POST /agent/invoke {tool:"project.open", confirmed:true}`——journey1 装载勘定①（决策侧已验收）：与 Godot 启动页 project_client.open_project 同一组 agent 侧调用（harness.go:12645 内核 open_project → applyProjectLifecycle "open" → BindProjectIdentity → OpenWorkingSessionAsGeneration → ActivateProjectStore），无需 GUI。

### 2.2 断言表（exit 0 = 全绿）

| 腿 | # | 断言 | 面 |
|----|---|------|----|
| 工程打开 | P1.1 | open invoke status=ok | /agent/invoke |
| | P1.2 | ui/state 轨道=Lead Vocal+Bass 恰 2 条 | /agent/ui/state |
| | P1.3 | project_uuid 非空且两连读稳定（新身份） | /agent/state shadow |
| | P1.4 | 会话证据：project/.vit_history/.sessions 下 ≥1 会话目录（OpenWorkingSessionAsGeneration 落地） | 文件系统 |
| 权限授予 | P2.1 | authority_mode=full_project_access | POST /agent/authority |
| | P2.2 | hold 窗（8s activation 请求+等待）后仍 full_project_access（AUTHORITY-LOST-1 口径） | /agent/authority + /agent/runtime/status |
| | P2.3 | `/smoke authority` 探针：stop_reason=authority_smoke_ok + reply 含 bound=full_project_access + 无确认卡（输入链绑定，服务端拥有，零 LLM 随机） | /agent/chat |
| 实验单轮 | P3.1 | mix.observe 预检循环至 MOM multitrack+static_level ready（预算 240s） | /agent/invoke mix.observe |
| | P3.2 | B2 propose（capability_id=static_mix.static_balance.v0, interaction_mode=propose）：canary_stage=proposal + needs_confirmation + workflow=capability_runtime_v1 + session_id + proposal_approval interaction id；readiness_blocked=可重试类（≤10 探） | /agent/chat |
| | P3.3 | approve（/agent/interaction/respond）→ canary_stage ∈ {executed_verified, executed_needs_review} | /agent/interaction/respond |
| | P3.4 | 执行后读回：observation_only mix.observe status=ok（再观察面） | /agent/invoke mix.observe |
| | P3.5 | 栈健康：/agent/state status=ok | /agent/state |

"实验单轮"定义声明：B2 能力运行时**单轮 propose→approve→执行→验证**（executed_verified）即一旅程实验轮——这是治理实验链的机械本体（提案卡→用户批准→写租约内执行→验证读回），全链服务端拥有、零 LLM。NL 话术驱动的自由态实验（产品演示的对话面）= J3，按 §8 概率口径另卡。

### 2.3 重试类与失败分类（§8 预声明）

- **可重试（滞后类）**：MOM/DAD readiness（P3.1 循环内消化）；B2 propose readiness_blocked（P3.2 探针环 ≤10）。
- **不可重试（真失败→throw）**：open/authority/smoke 探针/propose 契约/approve 段任一断言红；泊位起栈失败。
- 运行次数口径：本卡验收=同二进制 exit 0 ≥1 轮+决策侧复跑 1 轮（存在性证明，不主张稳定性比例；重复成功比例若需另行规定）。

### 2.4 泊位与隔离实现（对 dev_agent_smoke.ps1 的改动面）

- `$Scenario` 允许值 +`journey_first`（不入 `all`）；`-Scenario journey_first` 强制要求 `-StartKernel`（旅程拥有整栈，早期明确报错）。
- 旅程泊位 kernel 隔离：WorkingDirectory=run 目录 kernel_workspace + VIT_PROJECT_XML=run 目录 default_project.xml 副本 + Settings/Logs 目录（journey1 隔离契约，其余场景零改动）。
- 旅程泊位 agent 隔离：VIT_HISTORY_DRAFT_ROOT=run 目录 agent_drafts（泊位 agent 继承）。
- run 工件默认根：coord/runs/JOURNEY-1/<时间戳>/（其余场景仍默认 SMOKE-SCEN-RANGE-1/）。
- 拆除：既有场景 finally（agent→kernel Stop-Process）零改动复用。

## 3. 后续旅程增量（每旅程一卡，本卡不开工）

按 §1 表 J2→J6 顺序立卡；J4 立卡前先核"A/B candidate 确定性装卡入口"——无则按卡面停止条件上交锚点清单（涉 agent 侧新观测/驱动端点=越 scripts 域）。

## 4. 边界与声明

- **Godot 不在泊位内（端测覆盖边界声明）**：旅程层断言 agent↔kernel 活链路（HTTP/ZMQ 面）；工程打开的 Godot 启动页等价性由 journey1 勘定①承担（同一组 agent 侧调用，决策侧已验收）；webui 渲染面归 E2E-WEBUI-1，Godot 前端点击路径归用户手测（AGENTS §5 手测入口裁定）。若决策侧要求真 Godot 三件套形态，需另立卡（GUI 进程生命周期管理超本卡 scripts 域既有模式）。
- **不新增 agent 端点**：本设计全部断言面已存在（invoke/authority/chat smoke 探针/interaction respond/ui state/state/events/文件系统）——未触发卡面停止条件。
- **概率面隔离**：本门零 LLM；J3 的概率口径已预声明（§1 表），未混入本卡。
- 旧 journey1_demo_journey_smoke.ps1 保留不动（历史门槛载体+NL 旅程证据工具），旅程模式为新增体系非替换。
