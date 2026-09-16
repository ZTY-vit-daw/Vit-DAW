# 文献工程借点清单（开发动作域）— 2026-09-07

- 决策记录（2026-09-07 会话）：开发侧文档**只收录会改动 Vit 代码/契约/测试的开发动作**；论文论证、评测实验与答辩口径等一律归毕设项目（对照清单见毕设侧 `设计方向对照与偷学清单_2026-09-07.md`），本文件只留边界声明不重复收录。
- 状态口径：本文件为**现行参考清单**——条目状态（`待审计` / `待设计` / `已采纳` / `已落地` / `不采纳`）随卡片推进更新；条目落地并提交后更新状态，若不再需要保留为参考则整体迁 `docs/archive/` 并同步三态索引。
- 排期不以本文件为准：本文档第 3 节只记当时（2026-09-07）的排期建议；实际排期以 `CURRENT-STATE.md` 每轮 gate 现排为准。

## 1. 收录范围与边界

**收录**：直接导致开发仓库代码（Go / C++ 内核 / WebUI）、现行契约文档、测试与冒烟脚本变更的文献借鉴点。

**不收录（归毕设项目）**：

- 论文论证素材：DAWZY fxparam 数值换算背书、MixAssist 访谈主题、RIME 抽象分层结论等；
- 评测实验与指标设计：RIME AL0/AL1/AL2 分层实验、LLM2Fx 客观指标（Acc/MAE/MRS/DSP 特征）、MUSHRA 协议等；
- 产品/立场结论：防同质化论证、不采用参数规则化路线等；
- 未来方向储备：垂类后训练、哼唱输入、stem-aware 算子池扩展等。

上述内容的完整清单在毕设项目 `C:\Users\timoz\Documents\毕业设计\output\设计方向对照与偷学清单_2026-09-07.md`。若未来某条"评测/表达"需要在仓库落地基建（如评测脚本、指标代码），重新评估后以新条目加入本文档。

## 2. 候选条目

状态取值：`待审计`（先核实现状再定改动）/ `待设计`（需设计讨论后再开实现卡）/ `已采纳` / `已落地` / `不采纳`。

### L1 Apply 失败语义显式化 + 默认"第二提案再确认"

- 来源：RIME arXiv:2607.19605v2 §5.2 步 4；附录 D.1（Failure/Repair State 携带失败工具名、失败 assignment 与失败参数）；附录 B.2（逐工具默认参数全表，如 compressor threshold −18 dB / ratio 3、de-esser 4–10 kHz 等）。
- Vit 现状：执行失败已有回执与 Rollback，但无显式"失败类型 + 失败参数 + retry 预算"字段；无"失败后以默认建议值生成新提案"的语义。
- 建议动作（**设计红线**：默认值只作为新提案生成，必须再次走用户确认门，不得自动执行——RIME 的无确认兜底是明确不偷项）：
  1. 扩展 receipt：`failed_tool` / `failed_args` / `failure_class` / `retry_budget` 字段与机检规则；
  2. 失败后可选路径：agent 依 PCA 默认建议值生成"第二提案"，走确认门；确认拒绝则回合如实结束；
  3. 契约文档同步 + 失败注入 fixture 用例。
- 文件域：`agent/internal/experiment`（receipt）、`agent/internal/chat`（流程）、`agent/internal/agentloop`；`docs/FREE_STATE_IMPROVEMENT_EXECUTION_RECEIPT_V1.md`。
- 验收：receipt 机检单测（含失败字段与 ambiguous 不得 continue_once 等既有规则不回归）→ 测试/回放矩阵扩展 → 真栈烟测（脚本退出码 0）。
- 建议门：门 3（需设计讨论；与 L2 合并设计，实现可拆卡）。

### L2 PCA typed axis 契约单源化（range / step / 默认值元数据）

- 来源：LLM2Fx-Tools arXiv:2512.01559v2 §3.1 与附录 G（26 参数各带 `range + step`，且分 coarse/fine 双档采样区间）；官方仓库 SonyResearch/LLM2Fx README（数据集记录内嵌 `tools` schema 列表——模型可见的工具描述与校验使用同一数据源）。
- Vit 现状：PCA v2 身份无关 typed 词汇表已实现（`docs/PROCESSOR_CONTROL_ATTESTATION_V2.md`、`agent/internal/processorauthority`），以语义 axes 为界；axes 尚无 range/step/默认建议值元数据；"模型可见受限工具描述 / 越界校验 / 对外文档表格"是否同一数据源生成**待查**。
- 建议动作：
  1. 在 **typed axis 层**加元数据：operating range、discrete step（可 coarse/fine 双档）、default suggestion；不把 raw param_id / 插件滑杆映射暴露给模型（PCA v2 边界不破）；
  2. 由该权威源生成（a）受限工具描述、（b）运行时校验器、（c）PCA 文档/附录表格；
  3. 作为 L1 默认建议值的数据底座。
- 文件域：`agent/internal/processorauthority`（或 `processorregistry`）；`docs/PROCESSOR_CONTROL_ATTESTATION_V2.md` 后续扩展；数据-描述-校验一致性测试。
- 验收：一致性测试（同一权威源产出的描述/校验/文档互不漂移）+ PCA 既有测试全绿 + 真栈烟测。
- 建议门：门 3（与 L1 合并设计）。

### L3 VSP Apply 入口 freshness 不变式

- 来源：DAWZY arXiv:2512.03289 摘要原文 "It maintains grounding by refreshing state before mutation"（变更前刷新工程状态）。
- Vit 现状：needs_experiment 门 G4/G7 已把 ledger `project_binding` / freshness 作为生产数据源透传（`docs/FREE_STATE_NEEDS_EXPERIMENT_GATE_V1.md`）；**执行入口（VSP Apply）是否强制复核 freshness 待查**。
- 建议动作：审计 `vsphub` / `executionruntime` 的 Apply 路径；若入口无强制新鲜度校验，补强制核对 + 负例测试（过期 binding 的 Apply 必须拒绝并返回明确错误类型）。
- 文件域：`agent/internal/vsphub`（CAS/幂等执行协调）、`agent/internal/executionruntime`；相应单测。
- 验收：负例测试（stale binding → 拒绝）+ VSP 既有一致性测试不回归 + 真栈烟测。
- 建议门：门 2（审计 + 堵口小卡）。

### L4 护栏现状审计（代码级 vs prompt 级）

- 来源：RIME arXiv:2607.19605v2 附录 D.2（分离/混回 branch 开闭闸、强制分离、branch 重组闸、强制待办处理、强制返回——全部系统级强制，非 prompt 约束）。
- Vit 现状：G1–G7（`agent/internal/agentloop/free_state_gate.go`）、CCB bounded disclosure、PCA 准入均已是代码门；**是否残留"应当/必须"类的 prompt 级机制约束待审计**。
- 建议动作：审计自由态/语义入口的系统提示词与模板；凡属机制性约束的一律改代码门或显式标注为"仅建议措辞"；输出审计记录。
- 文件域：`agent/internal/agentloop`、`agent/internal/promptruntime`、`agent/internal/chat` 提示词；审计记录文档。
- 验收：审计记录（机制约束无 prompt 残留）+ 相关回归测试。
- 建议门：门 2（验证性，不引入新机制）。

### L5 agent 结构化输出 fail-closed 硬化

- 来源：LLM2Fx-Tools arXiv:2512.01559v2 附录 A（`<tool_call>{'name':..., 'arguments':{...}}</tool_call>` 结构化序列化）及其评测反例：Qwen2.5-Omni 因无法生成正确 JSON 格式、工具调用成功率 0.2%——自由文本承载结构化调用不可靠。
- Vit 现状：执行面走 VSP 结构化传输；agent 文本侧（诊断叙事 → 提案结构）的解析/校验是否 fail-closed **待查**（涉及 `semanticeffect` / `chat` 层）。
- 建议动作：审计解析层；非结构化/越界/字段缺失输出一律 fail-closed（产生显式失败事件，不做宽松修复或静默默认）；补解析边界测试样本集。
- 文件域：`agent/internal/semanticeffect`、`agent/internal/chat`、`agent/internal/executionports`；相应测试。
- 验收：畸形输出样本集测试全绿 + 消息生命周期等既有测试不回归。
- 建议门：门 2（审计 + 硬化小卡）。

### 明确不采纳 / 不涉及（防重开讨论）

- Diff-MSTC（arXiv:2411.06576）DAW 集成路线（SKI 私有 SDK 插件 + TorchScript）——路线不同不采纳；作为论文侧反例素材。
- MixAssist（arXiv:2507.06329）判官校准协议、对话数据形态——评测/数据研究，归毕设侧；若未来在 agent 循环内加"文本质检门"，以新条目另行评估。
- RIME 无确认自动兜底（默认参数直接执行）、DAWZY LLM 自由生成代码执行——与 Vit 确认门 + 受控执行制度冲突，明确不采纳（立场见毕设侧清单 §4）。

## 3. 排期建议（2026-09-07 快照；实际以 CURRENT-STATE 每轮 gate 为准）

- **门 0（当前）**：只登记不开发。原因：与未提交的 PCA-1 / 模型 B / DIAG3 代码线文件域重叠（AGENTS §12 diff 归属纪律）。
- **门 1**：PCA-1 有效复跑证据（p03 三轮 + p01 回归）→ 模型 B 端到端（evaluator 通过 + 真栈 exit 0）→ COMMIT-1 分线提交。
- **门 2**（门 1 后首批，审计/堵口小卡，一周级）：L3、L4、L5。
- **门 3**（答辩材料冻结后可排）：L1 + L2 设计讨论 → 拆实现卡。
- 若答辩周期占用队列，门 3 整体后置；本文档状态不因此失效。

## 4. 预期工程收益（含验收口径）

- **L1**：失败回合有出路 + 失败可复盘（带类型/参数的收据）；自由态循环完成率提升——可用 fixture 失败注入做"前后对比"断言；失败不再只能落 needs_observation 死胡同。
- **L2**：模型可见面 / 运行时校验 / 文档表格同源，消灭"文档说能调、运行时拒绝"类不一致；为 L1 提供默认建议值数据底座；为 WebUI 提案粒度与后续轴级工具提示留数据基础。
- **L3**：防"对象身份竞态"——提案生成后工程被外部改动时，Apply 在入口被拒绝（负例测试可验证），守住受控执行的边界声明。
- **L4**：机制约束全部落在代码门，消除 prompt 级约束漂移风险（验证性收益 + 审计记录）。
- **L5**：LLM 输出解析错误从"静默误解"变为"可见失败收据"，证据链诚实（对齐 AGENTS §8 失败分类纪律）。
- 诚实声明：以上均不在毕设演示关键路径上；价值在工程健壮性与下一轮迭代地基。

## 5. 证据与更新规则

- 原文取证（2026-09-07 会话抓取留档，逐条引用以留档文本为准）：
  - RIME：`C:\Users\timoz\Documents\毕业设计\lit_review_tmp\rime_arxiv_html.txt`（arXiv:2607.19605v2 官方 HTML 全文）；
  - DAWZY：`...\lit_review_tmp\dawzy_ar5iv.txt`（ar5iv 全文；论文代码级信息稀薄、官方匿名仓库不可访问，未作为证据）；
  - LLM2Fx-Tools：`...\lit_review_tmp\llm2fx_arxiv_html.txt`（arXiv:2512.01559v2 HTML 全文）+ 官方仓库 `github.com/SonyResearch/LLM2Fx`；
  - Diff-MSTC：`...\lit_review_tmp\diffmstc_arxiv_html.txt`（arXiv:2411.06576）；
  - MixAssist：`...\lit_review_tmp\mixassist.txt`（arXiv:2507.06329）。
- 更新规则：条目被采纳时在 CURRENT-STATE 对应 gate 记录登记；实现完成并提交后更新本文档状态为 `已落地`；全部条目关闭或不再参考时整体迁 `docs/archive/` 并更新三态索引。
