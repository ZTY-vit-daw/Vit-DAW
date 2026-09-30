# CLIPSCOPE-AGENT-1：路由宪法修订——clip_scope 竖线向 agent 开放（RiskConfirm 确认+披露+可逆）

- 池序 6（用户裁定 2026-09-30：[decisions/2026-09-30-clipscope-open-and-automation-slot.md](../../decisions/2026-09-30-clipscope-open-and-automation-slot.md)）；设计关联=docs/VITNOTE_V1_DESIGN.md §7.5/F14。**并行域：docs+agent prompt/测试（本卡）× executionruntime+烟测脚本（VITNOTE-IMPL-4）× webui（FIX-CONFIRM-CARD-1）——不同文件域可并行**
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无代码依赖，领取即开工
- 模型分级：L1 / GLM 或 flash 可接（修订语义已由裁定给定）
- 已核实事实（决策侧 2026-09-30，勿重勘）：
  - 内核竖线机制在：`rack_set_node_clip_scope` IPC 写 `vit_clip_scope`（docs/ROUTING_CONSTITUTION.md §竖线）
  - agent 工具目录在册：agent/internal/tools/catalog.go:1047 `spec("rack_set_node_clip_scope", "rack.set_node_clip_scope", "rack", "Set rack node clip scope.", RiskConfirm, true, true, true, true)`——**命令面已通、风险级别 RiskConfirm**
  - 禁令承载：宪法 :19 附近（rack_add_node 不得按「Clip 作用域」选中数量自动写 `clip:<id>`；绑定单 Clip 仅限用户手动画竖线）；该语义在 agent prompt 组装层的具体承载文件**未实锚**（已知 mix_session_workflow.go / mixboard.go 有 clip_scope 消费命中，领取后第一步实锚）
  - 原禁令理由：防"未手动画竖线却显示竖线"与全轨生效的听感不一致
- 目标：
  1. **宪法修订**（docs/ROUTING_CONSTITUTION.md）：单 Clip 绑定从"仅限用户手动"修订为"agent 可经 RiskConfirm 确认流提案执行"。保留三条纪律不变：①竖线写入必伴随披露（回执可见、竖线重建显示同步）；②可逆（解绑路径明示于宪法与回执）；③rack_add_node 仍不得静默自动绑定——装载与绑定两步分离。
  2. **prompt/纪律层同步**：实锚宪法语义在 agent 侧 prompt 组装的承载点并同步修订（grep clip_scope/竖线/clip scope 于 chat/mixboard 工作流域）；若发现代码级拦截（非纯文本纪律）一并申报，不擅自绕过。
  3. **测试**：RiskConfirm 确认流对该命令的提案→确认→执行→回执链路回归用例（已有用例则复跑并补"agent 提案绑定单 clip 获确认后写入成功+竖线可见"边界用例）。
- 文件域：docs/ROUTING_CONSTITUTION.md + agent 侧 prompt 承载点（实锚后申报具体文件，预期 chat/mixboard 域 ≤2 文件）+ 相关 _test.go。**接口冻结：不动 tools/catalog.go 命令定义、RiskConfirm 级别、内核命令面。**
- 验收标准：相关 go test 包全绿 + 全量 0 FAIL + gofmt + 宪法 diff 语义自洽（决策侧复核）+ 回执附承载点锚点清单（宪法外每一处修订点的 文件:行）。
- 停止条件：取证发现竖线写入存在内核侧一致性约束或听感风险的技术根据（非纯产品纪律）→ 停下实证上交，由决策侧回炉裁定。
- 领取：（时间 / origin/main hash / 分支名）
- 回执：（commit hash / 承载点锚点清单 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
