# 2026-09-30：clip_scope 向 agent 开放 + 自动化排序锚（用户裁定）

- 背景：VITNOTE-DESIGN 落笔（0b19eee6）后的边界质询——vit note 的辖区执行（"只处理框选这一段"）是否依赖 clip 级效果与自动化两项未开发能力。决策侧核查（2026-09-30 早窗）：
  1. **clip 级效果=机制半在，agent 执行被纪律层禁**：内核竖线机制在（`rack_set_node_clip_scope` IPC 写 `vit_clip_scope`，ROUTING_CONSTITUTION §竖线）；agent 工具目录在册且为 RiskConfirm 级（agent/internal/tools/catalog.go:1047）——命令面已通；禁令（rack_add_node 不得按 Clip 选中自动写竖线、单 Clip 绑定仅限用户手动）为宪法文档/prompt 纪律层，非命令缺失。原禁令理由=防"未手动画竖线却显示竖线"与全轨生效的听感不一致。
  2. **自动化=零实现**：agent 侧仅关键词路由+`AutomationLaneCount` 计数占位（capability_routing.go，数据源不存在恒 0）；内核无包络/写参时间线。
- **用户裁定 1**：clip_scope 自动绑定禁令向 agent 放开——"这个禁止自动是最初没有开发 agent 时的设计"。开放落地=宪法修订卡 CLIPSCOPE-AGENT-1（RiskConfirm 确认流+披露+可逆语义保留；命令面与风险级别不动）。
- **用户裁定 2**：自动化控制按混音能力层既定序位开发=空间与深度（时域混响/delay/send，总规划 C3）之后的 C4 段落自动化；不因 vit note 提前。
- **排序结论（决策侧）**：现池序维持——vit note v1（观察问答闭环，M8 判据）不依赖两项；clip_scope 开放为独立小卡入池（池序 6，与 vit note IMPL 序列无依赖）；自动化等能力线 C3 落地后开。设计侧落 docs/VITNOTE_V1_DESIGN.md §7.5（辖区执行面边界与演进）+ 基线 F14。
