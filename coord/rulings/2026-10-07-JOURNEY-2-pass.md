# Ruling：JOURNEY-2 验收 pass（2026-10-07，决策侧）

- 对象：旅程烟测 J2 插件装载（实现 **813f3ea6**，main）
- 裁定：**pass**。

## 证据链（决策侧亲核）

1. **实现面**：`scripts/dev_agent_smoke.ps1` 单文件 +约 470 行（场景段+泊位条件泛化），agent 代码零改动属实；`-JourneyPluginIdentifier` 默认 J1 同源插件（VST3-bx_hybrid），未新增插件依赖——卡面约束遵守。
2. **断言表**（卡面①-⑤逐项落位+实测列）：rack.add_node 回执三件（plugin_id=1020/plugin_instance_ready=true/graph_last_diff_kind=node_add）+选轨基线 plugin_count=0→装载后 ≥1+rack 行对象与回执 id 一致+PCA 门拒绝增量=0（≤0 保守断言+实测 delta=0，计数 3 读取最大防撕裂）+栈健康。零 LLM、断言全落服务端/内核拥有面——设计口径遵守。
3. **执行侧证据**：验收轮 2 轮 exit 0（104819/104851，退出码无管道直采）+midi_register 共享段回归 exit 0（104937）+泊位 4+1 轮自起自拆、拆后三端口零监听。
4. **我方复跑**：`-Scenario journey_plugin_load -StartKernel` **exit 0**（run **20261007_110716**，main HEAD=813f3ea6）：LEG5 实测 plugin_count=1/rack.track=Bass/rack_plugin_rows=1/rack_plugin_ids=[1020]，teardown ok——断言面与执行侧两轮一致。
5. **过程记录（诚实边界）采信**：首两轮 LEG5 读数缺陷（PowerShell 单元素数组经函数输出流展开）根因取证+离线最小复现+域内修复（不动共享 helper）；104630 中继轮真实 exit 1 如实记录；退出码采集从管道掩盖改直采——红过程不粉饰，方法学正确。
6. **归属核**：本轮验收以决策侧 110716 复跑为据（完整 HEAD）；执行侧验收轮 HEAD=40035e0d+未提交 scripts（后即 813f3ea6 内容）——scripts-only 卡归属间隙由复跑闭合。

## 后续

- J3（自由态 NL 旅程）立卡时按设计 §1 概率口径写卡（N=3/失败分类/止损线）；J4 立卡前先核 A/B candidate 确定性装卡入口（设计 §3 既定）。
- 旅程门槛层覆盖面：站点 1-4 已有确定性回归（J1+J2）。
