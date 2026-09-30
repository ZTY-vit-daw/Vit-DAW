# Ruling：FIX-CHAT-TMPDIR-FLAKE-1 — pass（2026-09-30 决策会话）

- 实现：0d5d0d2c@port/fix-chat-tmpdir-flake-1 → 合并 main 445d3688（cherry-pick）
- 亲核项：
  1. diff 对应：3 文件 +66/−8 **全为测试文件**（server_test_helpers_test.go 新增 58 行 helper+两名所在测试换用），零生产改动——与回执"文件域申报"一致，无越域。
  2. **我复跑**：go test ./internal/chat/ -count=1 于合并态 main——**ok（97.8s）**；两名隔离复跑各 1 轮 ok（执行侧验证=隔离各 3 轮+整包 2 轮+高压 60+60 轮全绿，全量 87 包 0 FAIL）。
  3. 根治面亲核：newChatServerForTest 收尾面 helper 族级根治（非逐测试补丁），对两测试名同源生效——卡面目标 1 达成。
  4. 停止条件核验：生产唯一宿主 cmd/vitagent 显式 Start()+defer Close()，wake 自动拉起是被测语义（scheduler 测试有意不经 helper）——非生产句柄泄漏定性成立，未触发。
- 附带处置：
  - 同族暴露面（5 个构造 workspace 的测试文件未换 helper）：**暂不推广**——均无失败留档且部分依赖中途调度行为；若该族 flake 在这些文件再现，扩展 helper 随当时修复卡走（本 ruling 记录在案）。
  - go vet unreachable code（semantic_treatment_strategy.go:1087/:1363）：main 基线预存申报属实（非本卡引入），留待独立清理卡。
- CURRENT-STATE 已知不稳定项状态：本裁定后，该族两名的 Windows TempDir 清理竞态已在测试收尾面根治——已知项条目可于下次 CURRENT-STATE 更新时改为"已修（FIX-CHAT-TMPDIR-FLAKE-1）"。
