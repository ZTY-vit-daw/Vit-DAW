# GENSHELL-GUI-1：模板面+GUI（实施④）

- 发卡：Mac 决策侧 / 2026-10-12（依据=件五 卡4）
- 派发确认：已确认（用户 2026-10-12「可以发卡」）
- 验收负责人：Mac 决策侧先行裁定+PC 主管复核留档
- 池序 98；**规格全文=[件五 卡4](../../runs/GEN-PLUGIN-SHELL-DESIGN-1/05-impl-card-specs.md)（权威）**
- 归属：Mac 生成线；P2 / 1-2 天 / GENSHELL-AGENT-1（dry-run 可用）
- 模型分级：L2 / GLM 执行会话

## 要点摘录（以规格为准）

壳 C++ 侧模板引擎（vit.genshell.template.v1 解析+compose 纯函数+四类型控件+内置四模板+用户模板 fail-closed）+GUI 六区业务接线。验收=compose 确定性单测+模板文件负例+真栈数据面烟测（free_text 编排行在场+用户原文置尾+预览=实发逐字节一致）+手测清单交付（[等待用户] 六区视觉目检）。停止条件=VSTGUI 控件集不足→范围上交。端测边界：GUI 视觉=手测清单不入自动化门。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / run ID / 手测清单 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
