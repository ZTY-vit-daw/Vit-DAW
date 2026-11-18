# SHELL-WIN-BEHAVIOR-1（391）终验：用户手测 PASS（附两项后续）

- 卡：`2026-11-17-SHELL-WIN-BEHAVIOR-1`（池序 391）；发卡/验收=PC 主管（lead）。
- 自动前置：409 产品窗口验收 + 410/411 producer/readiness + 412 呈现 + 413 入口刷新 + 415 stage 解析修复 + 414 组合真栈 attempt03 PASS（`coord/runs/SHELL-WIN-COMBO-1/run_20261118_190253`：409 分类器 PASS、395 呈现 terminal=ready/completed/steps=3、chain_detail=workspace_ready、零 LLM、卫生还原）。
- 用户手测（2026-11-18 晚，产品入口 `启动VitShell.cmd`，产物 VitShell `F8FD36AB…`/DLL `B270362D…`/agent `27D09FA0…`/webui `0886058E…`）：**效果非常好，判定 PASS**。手测覆盖：打开工程时对话窗不退后、DAW 在对话窗之后、真实装载反馈可见、窗口内行为正常。

## 后续项（不阻塞本卡 PASS）

1. 加载卡黄色状态环「绕轴环绕」视觉缺陷 → **416 SHELL-RING-SPIN-FIX-1 已修复并验收 PASS**（轴心居中/原地自转；webui dist 重建为 `0C6A1691…`）。
2. 工程态左侧“对话流列表”（工作区→工作树→分支 + 只读预览 + 上下文动作卡）→ 新需求线 **417/418/419**（[模型决定](../decisions/2026-11-18-entry-rail-conversation-model.md)），不属于 391 判据。

## 边界

- 自动证据只证明 shell 自然前台单次成功路径；用户手测为体验终判。
- 导出模板/`template_release` DLL 未刷新；396 retry 微卡未并入；395-B launcherData 文案未合入——均不属 391 判据。
- 本卡收口后，窗口线（391 家族）归档；后续窗口/入口 UI 演进由 417-419 及新卡承接。
