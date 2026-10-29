# FE-TELEM-HARDRECOVER-GAP-1 工件（hardrec_20261029_1）

- 日期：2026-10-29 晚；机器=PC；执行流=GLM-5.3 Flash（ZCode PC 量产会话，L1）
- 仓库=前端仓；分支=`port/fe-telem-hardrecover-1`；领取基线=`4219618`（含 213/215）；worktree=`D:/Godot/project/vit-daw-frontend-hardrec1`
- 交付级=离线级（零真栈零端口，同 213/215 口径：watchdog 时序面 173 真栈已证+hub 无故障注入面）

## 1. 修复内容（执行侧定案形态，卡面候选①websocket_open 沿门控）

根因（213 上交）：`_check_ws_stale_watchdog` 硬阈值分支（断供>stale_sec×2，默认 30s）清 `_ws_stale_latched` 不发 recovered，STATE_OPEN 重连沿也静默清锁存——重连成功时无任何「曾有 stale」信息存活，窗内 stale 沿拉取之后的 delta 无恢复回拉。

定案：新增 `_ws_stale_recover_armed` 待补标记（`vsp_realtime_adapter.gd:63-66`）：

- stale 沿置位（`_ws_stale_latched=true` 同步）；软恢复沿消费即清（213 语义不变）；
- **硬阈值分支清 `_ws_stale_latched` 不动 armed**（跨硬清除存活——缺口根因面的修复本体）；
- STATE_OPEN 状态沿行为提为 `_handle_ws_opened()`（`_poll_ws` 调用，语义不变），按 armed 门控补发 `websocket_recovered` 后清位——重连成功沿（含 stale→网络真断→重连路径，同属「曾有 stale」门控语义）补拉窗内残余；
- armed 已清零发：初始 open、重连失败循环不产生恢复沿，沿一次性非轮询；
- 消费侧 `telemetry_manager.gd` **零改动**（复用 213 既有 recovered 分支），零新命令。

停止条件核查：勘察未发现硬重连自带全量重订阅回拉设计（重订阅恢复的是订阅面 delta 流，不重放断供窗丢失 delta；工程投影回拉唯一入口即 telemetry_manager 防抖拉取链）——零修复裁定不成立，按卡面目标①实现。

## 2. 探针（213 场景探针同族扩展）

`tools/probe_telem_ws_hardrecover.tscn`+`_scene.gd`（24 检）：真 autoload 全家引擎装配（真 adapter+真 manager+真 perf，零 preload 自建），MockRefreshHost 单元替身；沿生成**直调真生产方法** `_check_ws_stale_watchdog`/`_handle_ws_opened`（STATE_OPEN 沿行为提为方法后可离线驱动，非复制逻辑）。

- H0 env 阈值面+S0 注入回环自检；H1 硬阈值沿生成（stale 沿经真 manager 消费+硬清除 latched+**armed 存活新不变量**+防抖回拉恰一次）；H2 重连 open 沿补发（armed 即清+recovered 计数+1+**窗内残余回拉恰一次=缺口闭合主断言**）；H3 沿精度（armed 已清重复 open 零发零拉）；H4 软恢复沿 armed 联动回归（213 软路径零回退）；H5 源面（armed 变量/open 沿门控/卡标记/213 复用面）。
- env：`VIT_GUI_VSP_REALTIME_DISABLE=1 VIT_GUI_VSP_REALTIME_WS_DISABLE=1 VIT_GUI_LEGACY_UDP_TELEMETRY_DISABLE=1 VIT_PERF_MONITOR=1 VIT_GUI_VSP_REALTIME_WS_STALE_SEC=3`。阈值 env 边界：headless 进程 `Time.get_ticks_msec()` 自引擎启动起算，相位 age（3200/6500ms）须小于进程运行时长且大于阈值——默认 15s/30s 不可达（r1/r2 留证：负 age 撞 watchdog `<=0` 哨兵提前 return，全沿零发），经生产 env 优先面调 3.0s（clamp[3,120] 合法配置），默认阈值时序面 173 真栈亲证在案不重证。

## 3. 验收数据

- 探针 r3：**24/24 exit 0**（`probe_hardrecover_scene_r3.log`）；r4：**24/24 exit 0**（`probe_hardrecover_scene_r4.log`）——两轮稳定。
- 213 探针回归：**17/17 exit 0**（`probe_wsrecover_213_regression.log`）——软恢复沿消费零回退。
- 全家回归（`fam_full_table.log`）：**-s 件 26/26 exit 0** + **file_reply 场景件 exit 0**——零回退。
- pipeline_selftest（headless 环境边界件，预期 exit 1 非回归项）**环境中断留证**：本 worktree 下连续两次挂起（15min 无脚本自报输出，停引擎头后；对照 213 同探针正常打印自报行后 exit 1——`fam_probe_pipeline_selftest.log`/`_retry.log` 与 213 工件 `pipeline_selftest_headless_boundary_expected1.log` 对照），与本人改动面（adapter stale 恢复沿+新探针）零相关（该件测 UI input press 面）；按 §8 停止原样重跑，强制终止两进程。注意：原批表行 `EXIT=1` 为 taskkill 产物**非探针自产退出码**，不作为功能结论；该件在 213 跑批同按非回归项处理，全家判定不含它。
- diag 留证：`probe_hardrecover_scene_r1.log`（初版 age 注入负值，全沿零发）、`probe_hardrecover_diag_r2.log`（S0 回环 PASS+DIAG 定位）。

## 4. 文件域核对

- `app/kernel/autoloads/vsp_realtime_adapter.gd`（+25/−5 于 r3 时点，终值以 commit 为准）——卡面域内；
- `tools/probe_telem_ws_hardrecover.tscn`+`_scene.gd`（新增）——探针域内；
- `telemetry_manager.gd` 零改动（消费复用）。越域：无。

## 5. 端测覆盖边界声明（AGENTS §5）

离线级：探针在引擎生产装配的真 autoload 上驱动真生产 FSM 方法，但**未在真实运行三件套（VitApp 内核+Godot 前端+Go agent）上运行**；默认阈值（15/30s）时序面与 hub 停摆>30s+socket 保持的真实触发场景未在本卡真栈复现（取证类型=代码锚点亲读+探针语义面，卡面已注明「非运行时复现——触发需 hub 停摆>30s 且 socket 保持」）。消费挂载零改动，用户手测义务不新增。

## 6. 工件清单

- `RESULTS.md`（本文件）；`logs/`：probe_hardrecover_scene_r3/r4.log、probe_wsrecover_213_regression.log、probe_hardrecover_scene_r1.log、probe_hardrecover_diag_r2.log、fam_full_table.log、fam_probe_*.log（27 件）、fam_file_reply_face.log
- 入库分支：`port/fe-telem-hardrecover-1-stack`（§6.1 工件纪律，commit 于实现分支持平后）
