# Ruling：REFSCHEMA-M3 pass（2026-10-09 决策侧）

- 裁定：**pass**。cherry-pick b9d7efc9 → main 72a358b2。
- 亲核四层：
  1. diff 亲读（dom/rlm 全文+fxm/com 抽验）：sha256 种子输入集逐字节不动（dom/com 剥 ProjectionID/GeneratedAt/LLMContext=内容身份；fxm 剥 ProjectionID/LLMContext 保 GeneratedAt、rlm 七字段拼接保 GeneratedAt=实例身份——与 REF_SCHEMA_V1 §7 明文一致，非偏离）；`FormatRef` 全程走文法面，F3 16hex 截断、scope=targetScope 先例（rlm=project:current）、snapshot=observation_id（rlm=GeneratedAt 实例代）；注册表/文法零改动核实（agentprotocol 包 diff 为空）。
  2. 卡外唯一改动核实（mixboard persistence_v2_test.go:127 断言两侧小写）：**决策侧强化断言实验**——worktree 内临时改回原文精确大小写 `Contains(string(data), ProjectionID)` 复跑 `TestProjectObservationPersistsCompactCOMProjectionAndCatalog` **同样通过**，证明盘上 ID 原文精确在、小写化为外观性非弱化（实验后 worktree 还原干净）。备注：后续若触此面可顺手采精确断言形态（更优非必须）。
  3. 我方复跑：dom/fxm/com/rlm/agentprotocol/mixboard 六包 -count=1 全 ok。
  4. 全量门：执行侧 90 包 0 FAIL+预算门未触发（M2 教训预警项核对过）；决策侧 main 合入后全量复跑（见 decision-log）。
- 消费面清单采信：回执 §3 十消费点逐列（chat 同源校验/capabilitycontext 透传/materialize/mixboard/queryengine 路由/harness 同源断言），核对结论与 diff 一致。
- G1 迁移账：M3 落地，A 类四包归一完成；M4/M5/M6/M7/M8 余量（M2X-1 在飞 Mac）。
