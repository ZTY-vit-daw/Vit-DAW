# MEMORY-RECON-1：memory 系统现存载体盘点报告（D14 前置勘察）

- 执行侧：Mac 夜间托管会话（纯只读勘察，零代码改动）
- 领取时 origin/main：`e9c6f59`（领取提交 `394a7cc`）
- 日期：2026-09-27；行号以本卡工作 HEAD 为准
- 路线图锚：D14（AGENTIC-OBSERVATION 蓝图收尾段——三记忆载体/原始流vs蒸馏/画像零基线/灭失点）
- 交叉输入：`coord/runs/L2-3-PROTOCOL-RECON-1/CHALLENGE_CONFIRM_INVENTORY.md`（M1-M4 固化路径分级——本报告 §1/§4 多处引用）

---

## 0. 执行摘要

1. **三类记忆只有"客观经验"有真载体**：PCA 认证台账（`~/.vit/processor_control_attestations.v1.json`）是全库唯一跨会话、按内容指纹组织、"一次认证终身可用"的持久经验面（与 L2-3 M4 结论一致）；用户偏好**无任何跨工程载体**；工程事实载体充分（三层 append-only）。
2. **"原始流永不删+蒸馏可重算"的现存基础成立**：原始流三层持久（journal jsonl 分片/mixboard observations/工程遥测日志），全部投影与 LLMContext 是可重算派生视图。
3. **贝叶斯画像零基线确认**：全库无任何计数/频率/统计驱动的用户行为聚合面——盲测判定有逐条记录（Turn 内持久化），但从不跨 run/跨工程汇总。
4. **灭失点清点 6 处**（验收线 ≥4）：capability packs、harness 渲染/特征收集器、vsphub 事件环、chat 内存 job 表、内核 meters 读即清、（边界项）投影缓存。
5. **环境指纹缺机器维度**：现有指纹全部是"插件二进制内容"粒度（机器无关）；机器/OS/版本标识无生成点——D14"按环境指纹"的键面需要新建。

---

## 1. 三种记忆的现存载体对照（D14 表）

### 1.1 用户偏好（跨工程随人走）

| 候选面 | 锚点 | 判定 |
|---|---|---|
| 引擎配置 | `agent/internal/config/config.go:147-151`（`~/.vit/config.json`，`Path()/Load()`） | **不是偏好记忆**——引擎级 knobs（模型/端口类），无任何用户判断沉淀语义；跨工程 ✓ 随人走 ✓ 但内容域不符 |
| 盲测判定历史 | `agent/internal/experiment/runtime.go:450`（`Turn.UserJudgmentEvidence []UserJudgmentEvidence`，json 字段 `user_judgment_evidence`）；恢复链 `agent/internal/chat/audition_events.go:222-235`（"User judgment restored" —— REFRESH/RECEIPT 修复链产物） | **最接近物，但作用域=单实验 run**：逐条持久化在 Turn 里、刷新可恢复，但**不外推**——下轮/下工程重问（L2-3 §2 同结论）；无跨工程聚合 |
| 用户手测反馈 | `coord/cards|rulings|reports`（人读 Markdown，仓内）+ `~/Documents/vit-handtest-*` 工件目录 | **运行时不可消费**：反馈以文档形态存在于协作层，无机器可读偏好存储 |
| 盲测开关配置 | `agent/internal/chat/audition_events.go:26`（`VIT_DAW_AUDITION_BLIND` 环境变量） | 环境变量=会话级开关，非记忆 |

**结论：用户偏好——无载体**。盲测判定是"工程内单 run 的偏好事实记录"，跨工程随人走的沉淀面为零。

### 1.2 客观经验（按环境指纹）

| 候选面 | 锚点 | 判定 |
|---|---|---|
| **PCA 认证台账** | `agent/internal/processorattestation/store.go:16,25-39`（`processor_control_attestations.v1.json`，`~/.vit/`，含备份恢复 `ReadReport.recovered_from_backup`）；v2 台账 `v2_store.go:29`；失效语义 `eligibility.go:49-53`（指纹不可得→stale） | **真载体**：跨会话持久、按 subject（插件路径+二进制指纹）组织、失效由指纹变化驱动。**全库唯一"一次成本终身享用"样板**（L2-3 §3 M4 行同判） |
| 环境指纹现状 | `agent/internal/processorattestation/fingerprint.go:16-40`（`FingerprintPath`=插件文件/bundle 的 SHA-256 内容哈希，路径作用域） | **粒度=插件二进制内容**，机器无关；**无机器/OS/版本标识生成点**（全库 grep `machine_id|host_id|env_fingerprint` 零命中——D1 runtime 的 "machine" 是累积剂量机注释） |
| knownPluginList 黑名单 | 内核 `VitApp/Workspace/Settings.xml`（JUCE `KnownPluginList`，crash-defence 死踩黑板；卫生清理面=PluginListHygiene，本夜卡②刚接披露） | **工作区级本地经验**：跨会话 ✓ 但随 workspace 不随人；黑名单是"踩坑记忆"的最原始形态 |
| 插件语义索引 | `agent/internal/pluginsemantics/index.go:66-74`（`~/.vit/plugin_semantics.json`） | 跨工程随人走 ✓——插件语义（参数面/能力面）客观知识，但来源是静态登记而非经验学习 |
| PCA 认证工件 | `agent/internal/chat/processor_certification_entry.go:585-593`（`~/.vit/pca_certifications/<jobID>/`） | 过程工件（快照/拓扑），经验结论回写 PCA 台账 |

**结论：客观经验——部分载体**。"按插件内容指纹"的维度在位（PCA 台账+语义索引）；D14 字面的"按环境指纹"（机器/版本）键面**缺失**。

### 1.3 工程事实（按工程）

| 候选面 | 锚点 | 判定 |
|---|---|---|
| projectstore | `agent/internal/projectstore/store.go:106-267`（`Resolve/Ensure/Activate`，manifest 备份恢复 `:187-210`、重定位重绑 `:224-242`）；evidence/gates 同包 | **真载体**：按工程 UUID 组织的持久根+证据门 |
| mixboard ledger | `agent/internal/mixboard/mixboard.go:1104-1106`（`<sessionDir>/observations/*.json` 逐观察落盘）；读回 `:3683`（readPreviousObservation）`:3756`（readLatestObservation）、`:382`（glob 恢复） | **真载体**：观察票 append-only 面板（B1 `obs_` ID 按文件名读回） |
| 行动日志 | `agent/internal/journal/journal.go:78-112`（`NewPersistent`+`persistLocked`）；v2 分片 `journal_v2.go:51`（活动 jsonl 分片+折入 `archive.jsonl`） | **真载体**：append-only 动作流，带回滚标记（`MarkRollback :144`） |
| 对话图 | `agent/internal/history/history.go:19`（`.vit_history`）、`:1547/:1889`（conversationGraph 读写）、worktree/分支检出 | **真载体**：会话历史图+工程 fork |
| 工程遥测日志 | `agent/internal/projectworkspace/l3_feature_log.go:132`（`BuildL3AcousticStatuses` 折叠 append-only 工程遥测日志） | **真载体**：特征就绪状态的原始流 |
| 决策账本 | 无专用载体——决策双轨：人读 `coord/decisions/`（仓内）+机器读 journal actions | **缺口**：工程内机器可读的"决策事件流"只有 journal 的动作面，无决策语义层 |

**结论：工程事实——载体充分**（五层持久面），决策语义层薄。

---

## 2. 原始记忆流 vs 蒸馏文档的现存对应

| 层 | 形态 | 锚点 | 重算性 |
|---|---|---|---|
| journal actions | **原始流**（jsonl 分片 append-only，永不改写；archive 折叠保留） | journal.go:90-163、journal_v2.go:51 | 源 |
| mixboard observations | **原始流**（逐观察 JSON 文件，board 引用不内联） | mixboard.go:1091-1106 | 源 |
| 工程遥测日志 | **原始流**（append-only，折叠出状态视图） | projectworkspace/l3_feature_log.go:132 | 源 |
| history 对话图 | **原始流**（图节点+消息路径） | history.go:19,1547 | 源 |
| 九投影+DAD | **蒸馏**（Build 纯函数从上述流+工程状态派生，不落盘） | 各投影包（如 tim/projection.go Build） | **可重算** |
| LLMContext | **蒸馏**（投影自带，`do_not_include_raw_package=true`） | 各投影 types（如 tim/types.go:137-146） | **可重算** |
| ContextPack/报告 | **蒸馏**（coord/runs 人读+机读工件） | coord/runs/* | 部分可重算（依赖当时 HEAD） |
| vsphub 事件环 | **非持久缓存**（`recent []CachedTelemetryEvent`，`eventReplayMax` 截断） | vsphub/event.go:14,112-118 | 挥发性中间层——**不是**原始流（细节灭失，见 §4-③） |

**结论**：D14"原始流永不删+蒸馏可重算"的结构前提**现成**——原始流全部 append-only 持久，蒸馏层全部派生可重算；唯一灰色带是 vsphub 事件环（缓存不是流）。

---

## 3. 画像/偏好的量化面现状

- 盲测判定**有逐条记录**（`Turn.UserJudgmentEvidence`，runtime.go:450）——但是**列举不是统计**：全库 grep 无任何对该数组的计数/频率/比率聚合（无 confirm-rate、无判定统计面）。
- 挑战-确认面同判：L2-3-PROTOCOL-RECON §2 逐面盘点后结论"留痕均单点、无跨 run 汇总"。
- 唯一的"统计驱动"面是**工程健康类**（project_health/TechSummary 计数，如 tim types.go TechnicalSummary）——那是工程状态统计，不是用户行为画像。

**结论：贝叶斯画像零基线确认** ✅——无任何计数/频率驱动的用户行为聚合面；D14 若建画像，是从零起步（第一块基石=把盲测判定从"列举"升"聚合"，且需先决断作用域：工程内 vs 跨工程）。

---

## 4. 跨会话记忆的灭失点（清单 ≥4，实际 6）

| # | 灭失点 | 锚点 | 灭失内容 | 缓解 |
|---|---|---|---|---|
| ① | capability packs | `agent/internal/capabilitycontext/pack.go`（`stablePackID :108`——pack 仅内存组装，包内无落盘/读盘函数；L1-1 §2 B5 同证） | 能力上下文包 | 按输入可重建（确定性 ID）——**可重建型灭失** |
| ② | harness 运行态 | `agent/internal/harness/harness.go:66,73`（`featureMu`+`renderResults map`、`h.waveforms/h.spectrals` 收集器） | 渲染结果等待表+特征收集器 | 部分可由 observations 重放；等待中的 waiter 通道直接丢 |
| ③ | vsphub 事件环 | `agent/internal/vsphub/event.go:112-118`（`eventReplayMax` 截断的内存 ring） | 事件细节（落盘面只收聚合后的 observations/日志） | 无——**真灭失**（原始事件不等价于任何持久面） |
| ④ | chat 内存 job 表 | `agent/internal/chat/processor_certification_entry.go`（`updateProcessorCertificationJob` 内存 job map；同类：audition session 运行态） | 认证/试听任务的进行时进度 | 结论回写 PCA 台账后可恢复语义，进行中进度丢 |
| ⑤ | 内核 meters 读即清 | `VitApp/Source/Service/VitHeadlessService.cpp:991`（`getAndClearOverload`/`getAndClearPeak`） | 轮询间隔间的削波事件 | 无——GAPS Item 7 已登记（粘性计数器=后续卡） |
| ⑥ | （边界项）投影实例 | 各投影 Build 产物不落盘（内存用后即弃） | TIM/MOM 等 | **非灭失**（§2 判定=可重算蒸馏）——列此明示边界 |

**与 REFRESH/RECEIPT 修复链的关系**：该链已修复的盲测判定恢复（audition_events.go:222-235 "restored:judgment"）与回执恢复属"灭失点收窄"先例；当前剩余边界=上表 ①-⑤，其中 ③⑤ 是不可重建的真灭失。

---

## 5. 验收对照

| 卡面验收项 | 状态 |
|---|---|
| ① 三记忆载体对照表（缺的明示"无载体"） | ✅ §1 三表——用户偏好**无载体**（明示）、客观经验部分载体（PCA 台账在位/环境指纹维度缺）、工程事实五层载体 |
| ② 流vs蒸馏对照 | ✅ §2 八行表——原始流四层持久/蒸馏三层可重算/事件环定性为挥发性中间层 |
| ③ 画像零基线结论 | ✅ §3——列举≠统计，全库无行为聚合面，零基线确认 |
| ④ 灭失点清单 ≥4 处 | ✅ §4 六处（含一处边界项明示），标注可重建/真灭失分型 |
| ⑤ 报告入库 | ✅ 本文件 `coord/runs/MEMORY-RECON-1/MEMORY_INVENTORY.md` |
| 零代码改动 | ✅ 全程只读 |
| 与 L2-3-PROTOCOL-RECON 交叉引用 | ✅ §0/§1.2/§3 引其 M4 固化样板与"不外推"结论 |

## 6. 给 D14 的排程输入（非本卡义务，仅浓缩）

1. 偏好载体若建：作用域先决断（工程内 vs 跨工程）；跨工程面可复用 `~/.vit/` 布局先例（config/attestations/semantics 三店同构）。
2. 环境指纹若建：键面需含机器/OS/agent 版本——现无生成点；PCA 的"指纹变化→stale"失效语义是现成范式。
3. 真灭失优先级：⑤（meters，已排 GAPS Item 7）>③（事件环，落盘面扩白名单语义需决策）。
