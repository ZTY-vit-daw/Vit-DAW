# MIXLAYER-M1-5：F1/queue/黑板三视图对齐（mix.report 纳入自由态结论）

- 发卡：GLM 主管决策侧 / 2026-10-11 早窗（同 M1-1 依据；MIXLAYER Q4 第⑤件事+M1 表）
- 派发确认：已确认（用户 2026-10-11 /morning 选定 PC 混音线首批立卡）
- 验收负责人：GLM 主管决策流
- 池序 68；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / **M1-1+M1-2 合入后领取**
- 模型分级：L1 / GLM 或 flash 执行会话（PC）

## 目标

F1 混音审查视图对齐（现状：mix_report.v1 账本汇总+Mixboard 决策账本现行，但看不见实验线与节点队列）：

1. mix.report 时间线纳入自由态 settle 结论（MIXLAYER Q4 第⑤件事——否则 F1 审查看不见实验线）；
2. mix.report 纳入 queue 节点状态视图（M1-2 产出）。

## 文件域

mixboard/mix.report 工具面（chat catalog 注册处）+ 对应单测；不越入 M1-1/M1-2 已合入文件的重构。

## 并行与资源

依赖 M1-1+M1-2；PC 真栈串行占用（PC-RUNTIME-STACK.md）。

## 验收标准

- 真栈烟测：mix.report 输出含最近 settle 结论行+queue 节点状态行，exit 0。
- `go test ./... -count=1` 0 FAIL + gofmt 净。

## 停止条件

M1-1/M1-2 合入形态与本卡接口假设不符 → 上交对齐。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
