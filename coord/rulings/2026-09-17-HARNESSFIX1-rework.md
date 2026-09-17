# 裁定：PORT-HARNESS-FIX-1 rework（扩域续作，方案 A 维持）（2026-09-17，决策侧）

- 对象：blocked 上交（实现 `e4dc881` @`port/harness-fix-1`）；决策侧已独立复验（隔离 worktree）：FLAKY-1 两例红→绿 PASS、`TestPluginLoadToRackInstrumentDefaultsToZ2FromScannedPluginInventory`（harness_test.go:1925-1961）FAIL、整包**有且仅有** 1 例失败——上交主张准确，停止条件命中正确（未擅改测试语义，纪律合规）
- **裁定一（方案 A 维持）**：不变量「被拒命令零内核副作用」的架构价值高于无索引机器的 Z2 解析便利；不推翻、不改走方案 B
- **裁定二（扩域·改写不删）**：`harness_test.go` 文件域开放，**仅限 :1925-1961 该一例**——按方案 A 新语义改写规格断言并更名（索引缺失 → 默认 Z3 + `kernel.commands` 为空），fakeKernel 的 scan_plugins 应答桩随之移除或改设为"不应被调用"的哨兵；**不采纳删除**（兄弟例 :1844 只覆盖索引在场的 Z2 解析，删则索引缺失路径零覆盖）；其余测试不触碰
- **裁定三（端测边界）**：本卡单元级验收足够——改动限命令规范化阶段 zone 解析，HTTP/ZMQ 接口面无变化；真栈 zone 解析行为验证并入 C3（重探针/重认证，索引将经正路建立）
- 续作要求：同分支续作，整包须零失败；干净机器前提（无 `~/.vit/plugin_semantics.json`、不设 `VIT_PLUGIN_SEMANTICS_PATH`）保持；`Get()` 模糊兜底错配隐患仍为独立待裁项，不在本卡
