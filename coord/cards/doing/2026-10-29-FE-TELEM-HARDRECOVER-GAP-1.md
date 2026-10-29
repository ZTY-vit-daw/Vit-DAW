# FE-TELEM-HARDRECOVER-GAP-1：硬重连路径（断供>30s）无恢复沿——窗内 delta 无恢复回拉（213 上交残余缺口）

- 发卡：GLM 主管决策流 / 2026-10-29 晚（依据=213 执行侧上交第 1 件：vsp_realtime_adapter.gd:724-729 硬阈值分支清锁存不发 recovered，主管代码读采纳）
- 派发确认：已确认（用户 2026-10-29 晚「可以发卡」）
- 验收负责人：GLM 主管决策流
- 池序 216；目标仓库=**前端仓 app/kernel/autoloads/vsp_realtime_adapter.gd**；P3 / 0.125 天 / 依赖=213 已合入；L1 量产档可做
- 取证类型：执行侧代码锚点亲读+主管复核（非运行时复现——触发需 hub 停摆>30s 且 socket 保持）

## 根因（213 上交亲核）

软恢复沿（`websocket_recovered`）已由 213 消费（stale 沿同形回拉）；但 `_check_ws_stale_watchdog` 的**硬阈值分支**（断供>30s→`websocket_closed`→硬重连重订阅）清 `_ws_stale_latched` 不发 recovered——该窗内 stale 沿拉取之后的 delta 无恢复回拉，等下一任意触发（与 213 修复前的软窗缺口同形）。

## 目标

1. 硬重连成功后补回拉：候选形态（执行侧定案回执披露）——`websocket_open`/重订阅完成沿按"曾有 stale"门控发 recovered（或等价一处消费形态，对齐 213 既有恢复沿语义，不新增命令）。
2. 探针：213 场景探针同族扩展（硬阈值路径沿生成+消费断言）；全家回归零回退。离线级交付边界可接受（同 213 口径：时序面已证+hub 无故障注入面）。

## 文件域

`app/kernel/autoloads/vsp_realtime_adapter.gd`+（消费挂载如需）`telemetry_manager.gd`+探针。越域即停。

## 停止条件

- 硬重连沿语义勘察发现设计意图即如此（如硬重连自带全量重订阅回拉）→上交证据零修复裁定。

## 领取

- 时间：2026-10-29 晚；owner=GLM-5.3 Flash（ZCode PC 量产执行会话，L1 档，机器=PC）
- 基线：主仓 origin/main `635c0592`；前端仓 origin/main=本地=`4219618`（含 213/215），工作树 tracked diff 空（仅未跟踪运行时目录，不涉文件域）
- 分支：`port/fe-telem-hardrecover-1`；worktree：`D:/Godot/project/vit-daw-frontend-hardrec1`
- 领取提交=本 mv 提交（同步共享 main 后开始实现）
