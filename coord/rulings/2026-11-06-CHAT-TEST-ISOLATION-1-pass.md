# Ruling：CHAT-TEST-ISOLATION-1（259）验收裁定——pass（G1/G2/G4 关闭）

- 裁定：GLM 主管决策流 / 2026-11-06 晚窗
- 输入：卡面回执（done，3766ac38）+ impl 6dfeff09 + 工件（两轮 console+全仓 console+impl_diff）+ 主管交叉复跑（merge ec477b8f 后 build 全量 exit 0 + chat -count=1 159.9s ok）

## 一、分面裁定

| 面 | 裁定 | 依据 |
|---|---|---|
| G1 verdict store 隔离 | **pass** | `isolateUnboundMixboardRootForTest`（t.Setenv VIT_MIXBOARD_ROOT=t.TempDir，env 优先=包内既有 idiom）——**生产注入缝 0 行**（卡面上限 ≤5）；10 个无 Activate watch 族测试接入+TestMain 清扫本进程残留；修复后两轮 unbound 零新增 pid 目录（对照基线轮 pid 13384 有残留） |
| G2 顺序敏感 | **pass** | per-test store 在领取基线已存在（257 失败轮同形——257 后已修一半），本卡补 projectstore 绑定漂移检测断言（残留向量=外来异步 ActivateProjectStore 翻全局态时明确归因失败） |
| G4 harness | **pass（不复现记录）** | 隔离 0.15s PASS+全仓 harness 包内 ok——按卡面处置记录闭环 |
| 回归面 | **pass** | chat 两轮 141.126s/141.332s EXIT=0×2+全仓 96 包 ok/0 FAIL（基线 929d6cd9 含 258/263）；diff +43/-0 全测试文件（6 个）零生产零行为 |

## 二、域外观察处置

- chat 包非 watch 面测试向 unbound 路径写 mixboard 会话/acoustic_package_status（基线轮 pid 13384 实证）：**TestMain 清理已兜底防跨轮致敏，挂账 known-observation 不开卡**；若再现同族断言失败，按 §11 不稳定测试纪律开收纳卡（同一 helper 模式）。

## 三、验收动作

- 合入 ec477b8f 零冲突；主管交叉复跑 build+chat 全绿；257 G1/G2/G4 三项就此关闭（G1-G7 全景：G1/G2/G4→本卡 ✓、G3/G7→260 ✓、G5→261 ✓、G6→262 待跑）。
