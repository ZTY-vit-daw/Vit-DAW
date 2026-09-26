# L2-3-PROTOCOL-RECON-1：质询-确权协议现状盘点（D13 统一协议前置）

- 优先级 / 预估 / 依赖：P2 / 0.4 天 / 路线图 D13+L2-3 段（AGENTIC-OBSERVATION 蓝图）；与 L2-2-SEG/L2-3 实现无耦合，纯现状盘点；夜间托管卡池成员
- 模型分级：L1 / flash 可接（纯只读勘察+报告，零代码改动）
- **执行侧（mac 会话，夜间托管）**
- 背景：D13 质询三要素=假设+证据引用+低成本反馈形式（二选一/确认）；适用于段落命名/角色身份/知识缺口/memory 主观偏好。本卡盘现存"类质询"机制与 D13 的差距。
- 目标：
  1. **现存确认/审批机制全景**（≥4 处带锚点）：pendingmanager 的 pending 卡机制、capabilityinteraction 的交互卡（用户确认面）、PCA evidence-backed control admission 的准入流（processorauthority/processorattestation 侧）、盲测 A/B 用户判断回路（audition/judgment）——每处记录：触发条件/呈现形式（问什么怎么问）/证据引用有无/留痕位置/结果去向。
  2. **三要素差距矩阵**：每个机制对照 D13（假设明确？带证据引用？反馈形式低成本？）逐格判定。
  3. **固化路径现状**：确权/确认后的结果落到哪（工程状态/投影字段/账本/一次性丢弃）——"一次性成本终身享用、下次会话不重问"的现状基础评估。
  4. **惰性质询现状**：现有机制是冷启动批发式还是用到才问；冲突再确权机制有无。
  5. 报告落 `coord/runs/L2-3-PROTOCOL-RECON-1/CHALLENGE_CONFIRM_INVENTORY.md`。
- 约束：零代码改动；锚点带文件:行；机制之间的同构/异构关系显式说明（哪些是同一套东西的不同面）。
- 验收：①全景 ≥4 机制带锚点 ②三要素差距矩阵完整 ③固化路径清单+终身享用评估 ④惰性现状结论 ⑤报告入库
- 停止条件：某机制面复杂盘不完 → 成稿+剩余列明上交
- 领取：2026-09-27 夜间托管 / origin/main 6b4eba9 / 报告分支 port/l2-3-protocol-recon-1（卡回执直推 main）
- 回执：报告 coord/runs/L2-3-PROTOCOL-RECON-1/CHALLENGE_CONFIRM_INVENTORY.md @ port/l2-3-protocol-recon-1（自 main 91bf88d 拉出）；零代码改动（仅新增报告文件）。验收对照：①6 机制面（≥4）各带五要素锚点 ✓ ②三要素矩阵逐格 ✓ ③固化路径六行+PCA 唯一终身享用样板 ✓ ④惰性健康结论+三处冲突再确权+无批发式 ✓ ⑤报告入库 ✓。机检：12 处锚点 sed 抽验全命中。发现要点：盲测 A/B（audition judgment）是三要素唯一齐全机制（preference 二选一+evidence_id/project_revision+假设随实验轮存续）——D13 统一协议最优起点；ApprovalRequest 结构体无 EvidenceRefs 字段（typed_protocol.go:244-253）是审批面补齐缺口；段落命名/角色身份两域零现存确权机制。
