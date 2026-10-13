# FIX-C2-TIMEFX-VIEWS-1：c2VerificationViews 补 reverb/delay 验证视图 case

- 发卡：GLM 主管决策侧 / 2026-10-13 晚（依据=FIX-TIMEFX-AXISFILTER-1 执行侧域外上交，[2026-10-13 裁定 §三](../../rulings/2026-10-13-FIX-TIMEFX-AXISFILTER-1-pass.md)采纳）
- 派发确认：已确认（主管依已核实的上交锚点发卡：`agent/internal/chat/c2_dynamic_control_runtime.go` c2VerificationViews :903 default 返回 nil）
- 验收负责人：GLM 主管决策流
- 池序 107；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无（FIX-106 已合入 main 2c241ca3）
- 模型分级：L1 / GLM 或 flash 执行会话

## 背景与缺陷画像

TIMEFX v2（reverb/delay）已注册为受治理动态族（registry/CoverageProofs/v2 认证/白名单全在，FIX-106 已收 D1 链上七处消费面漏接）。C2 能力会话面（capability_runtime_v1 动态控制）的 `c2VerificationViews(family)` 是"每族用什么观察视图验证干预效果"的对照表：compressor→track.time_dynamics、limiter→track.peak_structure、gate→activity_structure/time_dynamics、de-esser→frequency_time_events、transient→transient_structure、multiband→band_dynamics——**唯独缺 reverb/delay**。若 NL 请求经 C2 路径控制混响/延迟，验证步拿不到视图（nil）→C2 在受治理边界停下（fail-safe 停，不误写；与 D1 链不同路径，故 FIX-106 两轮复现未触发）。

## 任务

1. **取证前置（视图选型）**：对照 CCB view catalog 与既有六族选型语义，裁定 reverb/delay 各自应返回的观察视图（候选方向：时域效果的可感验证面——如 FXM A/B 变换观察、wet/dry 相关性类视图；**选型须有依据注记**，不得随手占位）。
2. **补 case + 测试**：c2VerificationViews 加两族 case；单测断言两族返回非 nil 且与选型一致；确认 C2 会话面到该函数的可达路径（若当前无可达触发路径，如实登记端测边界）。
3. `go test ./... -count=1` 0 FAIL + gofmt 净（LF blob 口径）。

## 文件域

agent/internal/chat/c2_dynamic_control_runtime.go + 同包新测试文件。

## 验收标准

diff 亲读+选型依据在卷+测试全绿；真栈腿可选（无可达路径时按 §5 显式声明端测边界即可）。

## 停止条件

视图选型需要新观察投影/view catalog 契约变更（超出补 case 范围）→ 带选型方案上交决策侧定契约。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 选型依据 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
