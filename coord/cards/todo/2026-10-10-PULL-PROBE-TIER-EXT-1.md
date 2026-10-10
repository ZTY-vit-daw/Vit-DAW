# PULL-PROBE-TIER-EXT-1：probe 分级表扩 clip.warm_waveform_bake（主管裁定落地）

- 发卡：GLM 主管决策侧 / 2026-10-10 晚窗（依据=[EVENING-BATCH rulings §3 分级裁定](../../rulings/2026-10-10-EVENING-BATCH-rulings.md)：clip.warm_waveform_bake 归 probe 级——D5 语义 render/probe 逐笔计入，波形烘焙=真实渲染物理工作（L2-2 实证串行瓶颈）；warm 预热属性不改变物理成本事实）
- 派发确认：已确认（裁定即授权）
- 验收负责人：GLM 主管决策流
- 池序 52；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P3 / 0.25h / PULL-PROBE-METER-1 已合入
- 模型分级：L0 / flash 可接

## 目标

`agent/internal/agentloop/pull_session.go` probeToolTier 分级表 probe 案增 `clip.warm.waveform.bake`（名称归一后形态——下划线/点号归一已由既有 ReplaceAll 处理，领取时以实测形态为准）；补单测：该工具回执 elapsed_ms 入账 probeSpent。

## 文件域

`agent/internal/agentloop/pull_session.go` + `pull_probe_meter_test.go`；越域即停。

## 验收标准

`go build ./...`+agentloop 包+全量 0 FAIL+gofmt 净；新单测绿。

## 停止条件

领取时该工具名/归一形态与预期不符 → 实测形态上交。

- 领取：（时间 / origin/main hash / owner / 分支 / 领取提交）
- 回执：（commit / 测试名）
- 验收：（裁定文件 / 验收 commit）
