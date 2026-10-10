# SMOKE-SCRIPT-HYGIENE-1：fs_largeproject_smoke.ps1 五项修补（复核报告 D/E 节+分类分歧）

- 发卡：GLM 主管决策侧 / 2026-10-10 晚窗（依据=[EVENING-BATCH rulings §5](../../rulings/2026-10-10-EVENING-BATCH-rulings.md) §9 缺口登记+[REVIEW-independent.md](../../runs/FS-LARGEPROJECT-SMOKE-1/REVIEW-independent.md) B/D 节）
- 派发确认：已确认（主管裁定）
- 验收负责人：GLM 主管决策流
- 池序 50；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P3 / 0.5 天 / 无；可与 FS-LEDGER-PERSIST-1 并行（不同文件域：本卡仅 scripts/）
- 模型分级：L1 / flash 可接（纯脚本修补）

## 五项目标（复核报告建议逐条）

1. **分类阶梯消费 fsLoopAdmissionStatus**：run_report.classification 阶梯增加已提取的准入态字段通道（修复"机器分类 other vs 人工 capability_blocked"分歧——run 185519 实证）。
2. **A4 补标记**：`c2.dynamic_plugin_load.governed`（单轨装载受控命令，现 _batch 子串覆盖不到）+telemetry source 面扫描（固定编排规划器独立 LLM source 标如 `b4_project_eq_planner`）。
3. **run_report 补字段**：command_line（$MyInvocation.Line 为空时改记录完整参数拼接或 PSCommandPath+Args）；exit code 数值化；agent_binary 死字段修复或删除。
4. **run_report.json 落盘健壮性**：Phase 6 类崩溃（$pid 只读变量）不吞 run_report——全局 try/catch 兜底落盘（184500 教训）。
5. 回归：既有断言组与清场约束零弱化。

## 文件域

`scripts/fs_largeproject_smoke.ps1` 单文件；越域即停。

## 验收标准

脚本语法过+干跑（-WhatIf 或早退模式如支持；无则静态审查）+复核报告五项逐条对照入回执；不占真栈（真栈回归归 FS-LEDGER-PERSIST-1）。

## 停止条件

无。

- 领取：（时间 / origin/main hash / owner / 分支 / 领取提交）
- 回执：（五项对照 / 修改锚点行）
- 验收：（裁定文件 / 验收 commit）
