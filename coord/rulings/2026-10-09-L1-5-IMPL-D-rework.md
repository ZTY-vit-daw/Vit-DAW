# Ruling：L1-5-IMPL-D 首轮裁定=补证返工（窄域）——2026-10-09 主管决策侧

- 裁定：**腿1-3 实现面+腿4 真栈执行面验收通过；P2 指标面补证返工**。卡移回 todo 待返工后再终审。
- 通过面（四层亲核）：
  1. 代码：Session 协议+A 占位退役+pullSession adapter+PullContinuation+入口路由全 diff 抽验；冻结语义逐条有测试对位（五不可丢全名在列/缺省 push 零变化/跨进程拒绝/旧 continuation fail-open/unknown 成本不填零/ctx 与用户取消分离/14 行映射表/Outcome 独立轴）。
  2. 我方复跑：pullharness+agentloop ok+全量 92 包 exit 0（D worktree）。
  3. 真栈工件亲读：summary outcome=pass；遥测 source=pullharness 真 LLM 轮（deepseek-v4-flash 真 HTTP）；动态区 834-916B=逐 turn 注入语义实证；栈占用登记规范。
  4. G3 裁定报告"不切换+差异归因"=诚实产出，方向采信。
- **缺陷（主管复核发现，证据如下）**：
  1. **P2 指标空洞**：dev_agent_smoke.ps1 的 `$abBreaks` 只收 `section_stats.breaks` 为 int/long/double 的值——breaks 是 JSON 对象数组，恒不命中 → `$abP2BreakCalls` 恒 0。G3-RULING §2.3 的"p2_break_calls 0/13 与 0/5"不是测量事实，是类型缺陷产物。
  2. **原始遥测与前缀断裂**：pull 侧 13 调用中 7 条记录带 `ruleset_changed:pullharness.protocol`（BreakReason.PrefixBreaking()=true 的断裂类）+history/snapshot 动态类；push 侧待同口径复算。模式=各 run 首批调用出现、run_078e4d67 交替出现——protocol 段内容在会话内变化，与"稳定前缀字节恒定"语义存在张力，需解释或修复。
- **补证返工授权（窄域四项，原分支继续）**：
  ① 修 P2 计算：按 breaks 数组非空计数（per-reason 分解可选）——脚本面测量修正。
  ② 离线复算：从既有工件 20261009_195558 遥测（两模式）重算 P2，**不要求新真栈轮**（遥测完备；如需端到端验证修正后指标可选跑一轮，独占栈）。
  ③ 解释 `ruleset_changed:pullharness.protocol`：渲染时机/内容变化源/为何不违稳定前缀语义——成文入 G3-RULING；若确属会话内稳定前缀突变（缺陷），在归因卡②范围登记具体修复项（修复本体非本返工域，除非两行内可证无害修正）。
  ④ G3-RULING §2.3 P2 行按复算结果改写+归因卡②范围更新+裁定逻辑复核（预期不切换结论不变，如实记录）。
- 门：脚本改动不触 Go 面——全量复跑仍须 0 FAIL（防连带）；复算工件落 coord/runs/L1-5-IMPL-D/（新目录，旧工件不覆盖）。
- 停止条件：protocol 段断裂解释需要改产品代码才能成立 → 上交主管裁（修复另立卡）。
