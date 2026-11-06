# ARR-SYMLINK-1：符号分析接入编曲——analysis 引导进 pack + 消费链验证（注册链⑤）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[258 报告](../../reports/2026-11-05-arr-recon-1.md) 点③ + taxonomy §三-3）
- 派发确认：已确认（用户 2026-11-05 批准入池）
- 验收负责人：GLM 主管决策流
- 池序 283；**注册链第 ⑤ 位（前置=276+282 已验收提交）**；P1 / 0.5-1 天；L1 / PC；零栈

## 目标

1. **编曲 pack 补 analysis 衔接引导**（277 留的占位）：编排次序="结构/变换族问题先 `midi.analyze_transformations`，再决定 marker/编辑动作"（消费链=pack→CatalogSummary→system prompt，message_loop.go:4278-4291 既有面零改动——258 点③已锚定）。
2. **消费链验证**（数据级）：构造 fixture——分析返回的 note_ids 与编辑命令（apply_note_patch region/transpose）目标同域衔接的单测/脚本验证（LLM 符号分析→编辑 MIDI 链路形态，taxonomy §三-3）。
3. 引导覆盖编曲动词族映射提示（"移调这段/倒影重写"→对应变换 op 提示，文本级）。

## 约束

- 文件域=capability_pack.go（277 所建 pack 的引导段扩展）+测试/脚本；零内核；不触 M3 域。

## 验收标准

- 引导段进 pack 且既有行为零变化；衔接验证数据级通过（输出样例入回执）；commit hash。

## 停止条件

- 276 命令返回形态与引导假设不符→对齐后重写文本（不阻塞）。

## 领取

- （执行侧回填）

## 回执

- （执行侧回填）

## 验收

- （主管回填）
