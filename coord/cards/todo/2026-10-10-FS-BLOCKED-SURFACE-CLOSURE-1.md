# FS-BLOCKED-SURFACE-CLOSURE-1：capability_blocked 边界面闭包分支扩面（LEDGER 顺带核对发现）

- 发卡：GLM 主管决策侧 / 2026-10-10 夜（依据=[NIGHT-BATCH rulings §5-2](../../rulings/2026-10-10-NIGHT-BATCH-rulings.md)+LEDGER 回执顺带核对：机制已精确定位）
- 派发确认：已确认（主管裁定）
- 验收负责人：GLM 主管决策流
- 池序 54；目标仓库=D:\Vit_DAW
- 优先级 / 预估 / 依赖：P3 / 0.5 天 / 无（可与 HARNESS-STRUCT-NORMALIZE-1 并行，文件域不相交）
- 模型分级：L1 / flash 可接

## 缺口（机制已定位，勿重复取证）

goalrunner_chat.go 的 `capabilityBlockedBoundaryResponse`（FS-CAPABILITY-BLOCKED-SURFACE-1 产物）检查位于 `!audioClosureActive` 分支内；**闭包激活（自由态标准配置）时 audioClosureResponse 先返回，blocked 边界面永不触发**——run 203348 期间实证 stop=done+goal_status=completed 与 fs_loop capability_blocked 并存形态（修复后机会面已改变，需重新观察）。既有测试只钉非闭包路径（free_state_capability_blocked_surface_test.go）。

## 目标

闭包激活路径补 blocked 边界检查（对齐 FS-CAPABILITY-BLOCKED-SURFACE-1 的边界面语义：blocked 收据在场时终局呈现不冒充完成）；补闭包路径测试（blocked 收据+closure 激活→边界面触发断言）。

## 文件域

`agent/internal/chat/goalrunner_chat.go` + 测试；越域即停。

## 验收标准

`go build ./...`+chat 包+全量 0 FAIL+gofmt 净；新测试绿+既有非闭包路径测试零改动全绿。

## 停止条件

边界面语义与 FS-CAPABILITY-BLOCKED-SURFACE-1 契约冲突（如闭包态本就另有呈现约定）→ 契约引用上交裁定。

- 领取：（时间 / origin/main hash / owner / 分支 / 领取提交）
- 回执：（commit / 测试名）
- 验收：（裁定文件 / 验收 commit）
