# JOURNEY-4-REV：A/B 试听判定旅程烟测——J3 链承接 + FS-CAP 增强后的确定性判定面（J4 复活条件①）

- 池序 24（J4 blocked 复活条件①落地：[J4 处置裁定](../rulings/2026-10-07-JOURNEY-4-blocked.md) + [FS-CAP pass](../rulings/2026-10-08-FS-CAPABILITY-BLOCKED-SURFACE-1-pass.md)；探针坐实确定性 B2 链不挂 audition，唯一装配路径=自由态 loop——本卡按裁定承接 J3 链）；目标仓库=D:\Vit_DAW（PC 执行侧）
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / J3 场景骨架（journey_free_state_nl，已合 main）+ FS-CAP（ad21313a——capability_blocked 止因可靠表面化，本卡分类面依赖它）
- 模型分级：L1 / **flash 可接**（判定腿断言面 HTTP+事件直驱零 LLM；NL 挂卡腿=真 LLM 概率面按 §8 计轮）
- **并行域：scripts 独占+真栈泊位（与任何真栈烟测互斥——错窗执行）**
- 已核实事实（J4 探针+J3 验收在案）：
  1. 装卡面：audition.candidate.ready 事件=candidate 在场组装面标志（R4 实证事件链 applied→prepare.started→candidate.ready→ready）；唯一装配路径=startFreeStateExperiment 自由态链（J4 探针 B1-B5 锚点清单，journey4_probe_report.md）。
  2. 判定面（J4 设计已定）：`/agent/audition/status`→select(A/B)→judgment POST→`user_judgment.requested`→`recorded` 事件链；盲听态（VIT_DAW_AUDITION_BLIND）判定不带标签偏置——判定腿全确定性。
  3. FS-CAP 增益：NL 轮若止于 capability_blocked，响应面现带 stop_reason=capability_blocked+free_state_admission_receipt（WorkflowData）——失败形可机器分类（此前散文终答不可分）。
- 目标：
  1. **场景**：`dev_agent_smoke.ps1` 加 `-Scenario journey_ab_judgment`（泊位族：-StartKernel 强制；J3 同款 fixture→open→authority→NL 轮骨架复用；不入 all——概率面按旅程调度）。
  2. **挂卡腿（概率面 §8）**：NL 轮驱动至 A/B 卡挂出（audition.candidate.ready）；**N=3 分轮调用**，成功=≥1 轮挂卡；未挂卡轮分类分记：no_candidate_found / capability_blocked（断言新边界响应面：stop_reason=capability_blocked+receipt 在 WorkflowData——FS-CAP 表面化的旅程面验证）/ 环境中断（有原始日志才可排除）。
  3. **判定腿（挂卡轮内，确定性）**：candidate 在场→`/agent/audition/status` 断言→select(A/B)→judgment POST→`user_judgment.requested`→`recorded` 事件链断言→`/agent/audition/status` 终态核对。
  4. **盲听腿**：`VIT_DAW_AUDITION_BLIND=1` 同泊位一轮（泊位内核/agent 继承注入）——判定请求不带 A/B 标签偏置断言（盲听态字段面按 AUDITION-PLAY-1/B12-1 既有语义实锚）。
  5. 断言表落卡+run 工件；exit-0 门只载确定性子面（挂卡轮的判定腿+盲听腿），概率腿结果如实分类记录不 throw。
- 文件域：scripts/dev_agent_smoke.ps1（场景段）；agent 代码改动仅限断言暴露缺口（停止条件管）。
- 约束：断言只落服务端/事件拥有面；泊位自起自拆申报（领取前 netstat 核 5555/7878/5556）；与其它真栈烟测错窗；**capability_blocked 分类断言只在该形自然触发时执行（不为凑分类人为构造拒绝）**。
- 验收标准：判定腿真栈 exit 0（≥1 挂卡轮）+盲听腿过+run 工件（分类分记）；决策侧复跑判定腿 1 轮。
- 停止条件：①N=3 零挂卡（分类清单+轮工件上交，J3 实况挂卡率 1/4——若 3 轮 miss 属概率实况非缺陷，上交后决策侧裁加轮或改直驱端点）；②判定断言需 agent 侧新观测端点→锚点清单上交。
- 领取：2026-10-08 晚窗 PC 执行侧（GLM-5.3 flash，ZCode 会话） / origin/main=509e37f9 / port/j4-rev（独立 worktree D:/Vit_DAW_wt_j4rev；卡状态移动待决策侧代行）
- 回执：（commit hash / N 轮分类结果 / 判定腿+盲听腿断言面 / run 工件 / 端测边界声明）
- 验收：**pass（2026-10-08 晚窗决策侧，rulings/2026-10-08-JOURNEY-4-REV-pass.md）**——diff 结构核验（6 hunk 既有段零触碰）+执行侧四轮工件亲读（C1 概率 miss 如实/C2 judgment_ok/B1 真实失败驱动加固/B2 judgment_ok_blind）+决策侧复跑 exit 0 直采且挂座走通全判定链（run 20261008_201949）；cherry-pick 16f7a762→main c937e27e；挂账三项随裁定（BOM 配置小修卡候选/D1 渲染 before-after 观察项归 B12 域/capability_blocked 真栈样本累计挂账）
