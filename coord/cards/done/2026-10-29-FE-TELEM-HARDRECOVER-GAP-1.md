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
- 领取提交=领取 mv 提交（09365d36，已同步共享 main）

## 回执（2026-10-29 晚，执行侧自验完成）

- **实现 commit**：前端仓 `port/fe-telem-hardrecover-1`@`8345687`（3 文件 +286/−5：adapter+探针 tscn/gd），已推远端。
- **定案形态**（卡面候选①）：新增 `_ws_stale_recover_armed` 待补标记——stale 沿置位、软恢复沿清（213 语义不变）、**硬阈值分支清 `_ws_stale_latched` 不动 armed**（跨硬清除存活=缺口修复本体）；STATE_OPEN 状态沿行为提为 `_handle_ws_opened()`（`_poll_ws` 调用，语义不变）按 armed 门控补发 `websocket_recovered` 后清位。armed 已清零发（初始 open/重连失败循环零发，沿一次性非轮询）。**消费侧 telemetry_manager.gd 零改动**（复用 213 recovered 分支），零新命令。
- **形态边界披露**：STATE_OPEN 沿门控顺带覆盖「stale→网络真断→重连成功」路径（STATE_CLOSED 沿不清 armed）——同属卡面「websocket_open 沿按曾有 stale 门控」授权语义，非越域扩面。
- **停止条件核查**：勘察未发现硬重连自带全量重订阅回拉设计（重订阅恢复订阅面 delta 流，不重放断供窗丢失 delta；工程投影回拉唯一入口=telemetry_manager 防抖拉取链）——零修复裁定不成立，按目标①实现。
- **验收数据**：本卡场景探针 `probe_telem_ws_hardrecover`（24 检：真 autoload 装配直调真生产 watchdog/opened 方法）**24/24×2 exit 0**；213 探针回归 **17/17 exit 0**；全家 **-s 件 26/26 exit 0+file_reply 场景件 exit 0** 零回退。
- **工件**：`coord/runs/FE-TELEM-HARDRECOVER-GAP-1/hardrec_20261029_1/`（RESULTS.md 六节+35 件日志），随主仓 **`port/fe-telem-hardrecover-1-stack`**@`caff6de6` 入库已推。
- **端测覆盖边界声明（AGENTS §5）**：离线级交付（零真栈零端口，同 213/215 口径——watchdog 时序面 173 真栈已证+hub 无故障注入面）；探针阈值经生产 env 优先面调 3.0s（headless 进程 tick 时长约束，r1/r2 诊断留证：负 age 撞 watchdog `<=0` 哨兵提前 return），默认 15/30s 阈值时序面不在此重证。
- **上交主管知悉**：
  1. `probe_pipeline_selftest`（headless 边界件）在本 worktree **连续两次挂起**（15min 无脚本自报输出，对照 213 worktree 正常自报后 exit 1），与本人改动面（adapter stale 恢复沿+新探针）零相关（该件测 UI input press 面）；按 §8 停止原样重跑强制终止，全家判定不含它（213 跑批同按非回归项处理）；原批表行 `EXIT=1` 为 taskkill 产物非探针自产退出码，不作功能结论。
  2. 残留进程上交：PID 7520（`godot -s probe_telem_ws_recover_consumer.gd`，2026-10-29 18:52 启动挂死至今）——判定为 213 执行会话 `-s` 模式首试实验残留（213 卡「首试 -s 模式不可行」留证现场），非本卡产物，未动，请主管处置。
  3. 主检出本地 main 指针落后于 origin/main（unstaged 工件挡 rebase，领取与卡片提交均走临时协调 checkout 路径，§2.1-3/§3 纪律内）。
- 验收负责人：GLM 主管决策流（待裁定；不自行 merge）
