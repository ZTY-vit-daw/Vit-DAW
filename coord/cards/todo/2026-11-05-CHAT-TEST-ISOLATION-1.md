# CHAT-TEST-ISOLATION-1：chat 包测试全局态隔离修复（G1/G2 族）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[257 pass 裁定](../../rulings/2026-11-05-AF-FAMILY-HEALTH-1-pass.md) G1/G2/G4）
- 派发确认：已确认；验收负责人：GLM 主管决策流
- 池序 259；目标=agent/internal/chat 测试面；P2 / 0.25 天 / 无依赖；L1 / PC 会话；零真栈

## 目标

1. G1：`loudnessWatchVerdictStorePath()` 全局 `%TEMP%` 路径无 per-test 隔离——改 t.TempDir 注入或 t.Cleanup 清残留（生产代码零改动优先，测试注入缝最小面）；
2. G2：`TestRecordFreeStateSettlementPersistsAcrossSessions` projectstore 全局 Activate 态顺序敏感——隔离（每测试自建 store 或 restore）；
3. G4：harness `TestV2ProjectStoreSurvivesRestartAndContinues` 若复现则并入同族处置，不复现记录即可。
- 验收=chat 全量包顺序复跑两轮全绿+全仓 `go test ./...` 绿；工件=修改 diff+两轮 console。

## 约束

文件域=chat 包测试文件（+必要时 verdict store 路径注入缝生产面 ≤5 行）；禁行为改动。

## 领取/回执/验收

- （执行侧回填）/（执行侧回填）/（主管回填）
