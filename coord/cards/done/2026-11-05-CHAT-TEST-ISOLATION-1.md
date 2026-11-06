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

- 领取：2026-11-06 午后 / origin/main=929d6cd9 / owner=GLM-5.3·PC·ZCode 执行会话（CHAT-TEST-ISOLATION-1 执行流）/ 分支 port/chat-test-isolation-1 / worktree D:/Vit_DAW_wt_259impl / 领取提交=964c8026（卡面首入库随领取协调提交）
- 回执：实现 commit **6dfeff09** @ port/chat-test-isolation-1（已推 origin）；工件 @ 同 commit `coord/runs/CHAT-TEST-ISOLATION-1/`（README+impl_diff.patch+chat 两轮 console+全仓 console+G4 隔离 console）。要点：①G1 新增 `isolateUnboundMixboardRootForTest`（`t.Setenv("VIT_MIXBOARD_ROOT", t.TempDir())`，DefaultRoot 中该 env 优先于全局绑定——包内既有 idiom，生产注入缝 **0 行**，卡面 ≤5 行上限内）接入 10 个无 Activate 的 watch 族测试（trigger×3/state×5/persist_race×2）+ TestMain 收尾清扫本进程 `%TEMP%/vit-daw-unbound/<pid>` 残留；②G2 该测试 per-test Deactivate+Activate 自建 store **在领取基线已存在**（257 失败轮同形），本卡补写后 projectstore 绑定漂移检测断言（残留向量=外来异步 `harness.ActivateProjectStore` 翻全局态，漂移时以明确归因失败）；③G4 隔离 PASS 0.15s + 全仓 harness 包内 ok——不复现，记录处置。验收：chat 两轮 141.126s/141.332s EXIT=0×2 + 全仓 96 包 ok/0 FAIL EXIT=0；修复后两轮 unbound 零新增 pid 目录（对照基线轮 pid 13384 有残留写入）。diff 面 +43/-0 全测试文件（6 个），零生产改动，零行为改动。**域外观察（上交不扩域）**：chat 包另有非 watch 面测试向 unbound 路径写 mixboard 会话/acoustic_package_status（基线轮 pid 13384 实证），TestMain 清理已兜底防跨轮致敏，若再现同族断言失败建议另开卡按同一 helper 模式收纳。
- 验收：（主管回填）
