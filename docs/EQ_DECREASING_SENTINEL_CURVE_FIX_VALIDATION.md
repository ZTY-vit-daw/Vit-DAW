# EQ 递减频率曲线与 inactive sentinel 修复验证

日期：2026-07-28
分支：`codex/eq-structural-matrix-unification`

## 结论

Waves Channel Strip 的 High Cut 控件存在一种通用结构：归一化值增大时，活动频率值严格递减，并且归一化端点是 `Off` / `Out` 一类无物理频率的 inactive sentinel。识别器此前丢弃整条 probe 曲线，planner 因而沿错误方向修正物理值。

本次修复不依赖插件名称。它保留 sentinel 之外、至少三个样本组成的严格单调活动曲线，同时支持物理值递增和递减。真实 Godot → Kernel → Agent 烟测已证明该规则跨两个独立 Waves 型号生效；历史 16 个 capture 与 Waves 50 个 capture 均无 capability 回退。

## 根因

`SSLChannel Stereo` 的 LP Frequency 五点 probe 为：

| Normalized | Display |
|---:|---:|
| 0.00 | `Out KHz` |
| 0.25 | 10.7 kHz |
| 0.50 | 5.0 kHz |
| 0.75 | 3.7 kHz |
| 1.00 | 3.0 kHz |

旧实现无法把 `Out KHz` 解析为物理频率，因此丢弃整条曲线。后续只剩不带方向信息的 3.0–10.7 kHz domain，8 kHz 请求被当成递增映射，并在 readback correction 中最终推到 normalized 1.0 / 3.0 kHz。

## 修复

### 曲线事实层

`eqCurveFromProbe` 现在：

1. 只跳过明确的 inactive sentinel；
2. 要求剩余至少三个活动样本；
3. 要求 normalized 轴严格递增；
4. 允许物理轴严格递增或严格递减；
5. 不推测 sentinel 端未被 probe 证明的物理值。

因此 8 kHz 可以从实测的 10.7→5.0 kHz 区间反解；12 kHz 仍超出已证明活动域并安全拒绝。

### 事务副作用守卫

交叉型号烟测发现 `SSL EV2 Channel Stereo` 在写 LP Frequency 和 LP On/Off 时，会自行把父级 `EQ Bypass` 参数 10 从 On 改为 Off。只读参数面不能证明这种父子依赖，因此通用控制器不能把它当成获准写入。

执行器现在保存完整数字参数快照，在写入和物理修正后检查所有未计划参数。发现联动时：

- 恢复计划参数及联动参数；
- 最后恢复 side-effect 参数，避免父级状态再次被子级写入触发；
- 返回 typed rejection `unplanned_parameter_change`；
- 不把事务报告为成功。

这仍然没有引入父级自动激活、插件名分支或音频探测。

## 验证结果

### 离线 replay

| 数据集 | Recognized | Executable | 与修复前 capability diff |
|---|---:|---:|---:|
| 历史 capture 16 | 14 | 14 | 0 lost / 0 gained |
| Waves capture 50 | 44 | 38 | 0 lost / 0 gained |

证据：

- `artifacts/eq_auxiliary_exclusion_fix/20260728/historical_sentinel_curve_fix.json`
- `artifacts/eq_auxiliary_exclusion_fix/20260728/historical_sentinel_curve_diff.md`
- `artifacts/eq_auxiliary_exclusion_fix/20260728/waves_sentinel_curve_fix.json`
- `artifacts/eq_auxiliary_exclusion_fix/20260728/waves_sentinel_curve_diff.md`

### 真实烟测

| 型号 | 指令目标 | 结果 |
|---|---|---|
| Q10 Stereo | Low Cut 80 Hz | 成功、exact、恢复成功 |
| SSLChannel Stereo | High Cut 8 kHz | 成功、exact、恢复成功 |
| SSLGChannel Stereo | High Cut 8 kHz | 成功、exact、恢复成功 |
| SSL EV2 Channel Stereo | High Cut 8 kHz | 频率映射到 8 kHz；检测父级参数 10 联动，拒绝并完整恢复 |

三条成功路径汇总：

- `artifacts/eq_compat_matrix/20260728_014908/capture/summary.json`

typed rejection 定点复验：

- `artifacts/eq_compat_matrix/20260728_015412/capture/summary.json`
- 返回：`rejection_code=unplanned_parameter_change`
- 消息：`unplanned parameter changes: 10; full preimage restored`

该定点 runner 的总体 status 为 failed 是预期行为：runner 把任何 HTTP 400 记作失败，而本用例正是在验收生产控制器必须拒绝未授权父级联动。Godot 测试链随后关闭，端口 5555、5556、7878、8787 均释放，受保护运行时文件保持 byte-identical。

### Go 验证

- `go test ./internal/chat ./internal/workflows/plugingrabber`
- `go vet ./internal/chat ./internal/workflows/plugingrabber`
- `go test ./...`
- `git diff --check`

全部通过。

## 决策边界

本修复解决的是可由参数面证明的递减连续频率映射，不授权推断未知物理端点，也不授权自动改变父级 EQ/Bypass。`SSL EV2 Channel Stereo` 因此不是“High Cut 映射失败”，而是“插件存在未声明联动，事务被安全拒绝”。若未来希望使 EV2 可执行，需要先从通用参数事实中证明父子依赖和恢复顺序；不能以型号白名单绕过守卫。

Marvel GEQ 的无频率标签 ISO anchor 推断不在本次范围内，继续搁置。
