# PORT-MAC-HARNESS-FLAKY-1：harness 2 例 darwin 失败根因排查

- 优先级 / 预估 / 依赖：P2 / 0.5-1 天（取证卡）/ 无
- 背景：Mac darwin/arm64 上 `TestFullProjectAccessDoesNotDisplaceExplicitSelectionAuthorization` 与 `TestAgentProcessorLoadGateRechecksPCAAndBlocksBypass` 失败；干净 worktree @`ee0fac4` 复跑同样失败（与 C1 无关）；Windows 同版本 PASS（决策侧 2026-09-16 复验）
- 目标：定位 mac 特有根因（候选方向：路径分隔符 / 大小写敏感 / 文件锁语义 / 时序 / TempDir 行为差异），产出取证报告；根因在共享代码 → 附最小修复建议转修复卡；属 mac 环境语义 → 文档化到 PORT_AUDIT §7 或独立风险条目
- 文件域：只读取证为主；如需实验性补丁，放独立分支不进 main
- 验收标准：失败原因有代码锚点或最小复现；原始失败输出已保存（AGENTS §11 不稳定测试纪律）
- 停止条件：2×20 分钟仍无法缩小假设面 → 上交已收集证据转 blocked
- 领取：
- 回执：
- 验收：
