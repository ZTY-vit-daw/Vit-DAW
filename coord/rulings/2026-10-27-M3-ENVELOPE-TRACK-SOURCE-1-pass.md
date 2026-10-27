# Ruling：M3-ENVELOPE-TRACK-SOURCE-1（196）——pass

- 验收人：GLM 主管决策流 / 2026-10-27 午后
- 合入：主仓 fa94eb8f（scripts 探测模式+run 工件+RESULTS.md）。

## 亲核面

1. **diff 亲读**：automation_curve_smoke.py 探测模式增量（--probe-tracks/--probe-params，默认路径零改动）+dev_agent_smoke.ps1 -TrackSourceProbe 腿；VitApp/ 生产源零改动亲核（RESULTS §一在卷）。
2. **判定面**：run tracksrc_20261027_1232 ps1 exit 0（含 190 回归面 13 checks=scripts 默认路径零改动佐证）+28 行逐字回执工件亲核——volume/pan 双轨全周期（空披露→三点→逐点回读 float32 容差内→跨轨 plugin_id 隔离→clear 回空）、master/trim/mute/gain+Level Meter 不可寻址逐字留证、descriptor 面 error 与 189 逐字一致（B 案前提复核）。
3. **工件契约（§9）**：完整命令/退出码/run ID/HEAD/隔离副本/登记释放链全在卷；前置缺陷轮 1229 留证（探针自身缺陷，内核无涉）。

## 裁定

- **整卡 pass**。探测目标全履行、边界如实（单轮实证无重复采样，§8 口径在卷）。
- **A/B 择一挂起随 F2 用户裁定**（volume/pan 一等公民确认后开消费实施卡族）。主管预备意见=**A 案先行**（零内核契约变更+本卡已钉死寻址细节；B 案作为后续描述符面统一化选项，动内核契约须主管明裁的约束 RESULTS 已列）。
