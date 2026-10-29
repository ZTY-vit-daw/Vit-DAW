# FE-TELEM-WSRECOVER-CONSUMER-1：websocket_recovered 沿无消费者——断供窗 delta 丢失无恢复回拉（212 勘察上交缺口）

- 发卡：GLM 主管决策流 / 2026-10-28 深夜（依据=212 勘察上交：vsp_realtime_adapter.gd:718 发射/telemetry_manager.gd:665 只吃 stale 沿）
- 派发确认：已确认（用户 2026-10-29 晚直接派发至 PC 量产执行会话：「本会话串行完成两张前端 telemetry 域小卡…先 213 后 215」）
- 验收负责人：GLM 主管决策流
- 池序 213；目标仓库=**前端仓 app/kernel/autoloads/ 域**；P2 / 0.25 天 / 依赖=无；L1 量产档可做

## 根因（212 勘察亲核）

`websocket_recovered` 信号发射（vsp_realtime_adapter.gd:718）无任何消费者（telemetry_manager.gd:665 的恢复沿只吃 stale）——VSP hub websocket 断供窗内 delta 丢失后无恢复回拉，机架/时间线可陈旧至下一任意触发。

## 目标

1. 恢复沿消费：websocket_recovered → 防抖回拉（对齐 telemetry 既有 stale 沿同形态，一处消费）；
2. 探针断言：断供→恢复→repo/机架重拉一致性（事件管线/时序面口径按适用）；既有全家回归零回退。

## 文件域

`app/kernel/autoloads/telemetry_manager.gd`（消费挂载）+`app/kernel/autoloads/vsp_realtime_adapter.gd`（仅当沿语义需澄清）+探针。越域即停。

## 领取

- 领取：2026-10-29 18:43 晚窗 / 主仓 origin/main=26a6ed75 / owner=GLM-5.3 ZCode PC 量产执行会话（本会话，L1 档）/ 分支=port/fe-telem-wsrecover-1 / worktree=D:/Godot/project/vit-daw-frontend-wsrec1 / 领取基线=前端 main 37dc4b6 亲核（origin/main 同点）
- 领取时前端仓基线记录：`git rev-parse HEAD`=37dc4b6da735f1e32fef3d091c8553195f5596c7；`git status --short`（tracked）=零改动（仅用户测试工程未跟踪目录）；`git diff --stat`=空
- 领取前核对：卡仍在 todo（fetch 重读后 main=26a6ed75 拉平）；doing 仅 GEN-SUNO（Mac 设计卡，无文件域/栈冲突）；文件域与在飞卡不相交
