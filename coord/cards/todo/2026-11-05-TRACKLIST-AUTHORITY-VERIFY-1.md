# TRACKLIST-AUTHORITY-VERIFY-1：内核回复权威缺陷复验与修复（观察完整性地基）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[2026-11-05-af-line-survey.md](../../reports/2026-11-05-af-line-survey.md) §二/§三）
- 派发确认：已确认（主管发卡；用户 2026-11-05「先勘察，发卡」口头授权）
- 验收负责人：GLM 主管决策流
- 池序 255；目标仓库=主仓 agent；P1 / 0.5-1 天 / 依赖=无；L1 / PC 执行会话；复验优先，修复腿视复验结果

## 背景（勘察锚点）

- 139 TRACK-LIST-GAP-1 诊断（已在 main：coord/runs/TRACK-LIST-GAP-1）钉死形态：**tool face 返回 shadow Summary 忽略 kernel reply；delta-only 轨道在权威快照刷新前成孤儿**（六面探针同帧 tool==shadow 全阶段）。
- 分支 port/track-list-gap-1（ahead5，worktree wt_tracklist1 保留中）有：fix 草稿卡（tool face kernel-reply authority+shadow delta refresh trigger）+harness 修复两件+确定性外部 new/import 两轮+official GO run PASS（run_20261018_175229）。
- 未知项：此后 main 上的 fix-tracklist-shadow-freshness-1 / VSP-SHADOW-REFRESH 系工作**是否已覆盖**此缺陷——须先复验再动手。

## 目标

1. **复验（先行，只读）**：在当前 main 真栈或最小复现面复测 139 诊断形态（外部新增/导入轨→tool face 读数是否反映 kernel reply 而非旧 shadow）——分支 GO run 探针（tracklist_gap_probe.py 已在 main runs 内）可改造复用；
2. 分叉：**缺陷仍在**→按分支草稿实现修（文件域=harness/shadow 投影刷新面，kernel-reply 权威化+delta 触发刷新），补单测+真栈复验 run；**已被覆盖**→证据包收口（复验输出 vs 139 形态对照）关线，分支残余一并定性；
3. 真栈验证（修复腿时）：外部 new+import 两轮确定性探针，tool face 读数与 kernel reply 一致=过；§8 口径跑前成文（预算 ≈¥0.5-2，修复腿 LLM 参与若为零则更低）。

## 约束

- 复验先行：未复验不得动修复面；
- 文件域=harness/shadow 投影面（按草稿），**不触 chat 治理链/loudnesswatch**（与 254 不相交）；
- 探针复用 139 先例（确定性轮，无 LLM 依赖优先）；
- 禁 git reset/clean；分支取材只读。

## 停止条件

- 复验发现形态与 139 诊断不同型→如实分列上交（可能是新缺陷面）；
- 修复根因超出草稿域→取证转卡；
- 真栈断言失败→取证包不重跑。

## 领取

- （执行侧回填：会话/worktree/分支/领取基线 HEAD+status+diff --stat/栈登记）

## 回执

- （执行侧回填：复验结论[在/不在+形态对照]/修复 diff+测试/命令退出码/run ID/计费）

## 验收

- （主管回填）
