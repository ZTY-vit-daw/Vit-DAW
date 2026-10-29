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

## 回执

（2026-10-29 晚，doing→done 交主管验收；执行 owner=GLM-5.3 ZCode PC 量产执行会话）

- **实现（单文件域内）**：`app/kernel/autoloads/telemetry_manager.gd`（+9/−6）——`_on_vsp_realtime_status` 补恢复沿分支：`websocket_recovered → _perf_count("telemetry.hub_recovered_refresh_scheduled") + _schedule_project_refresh_from_delta("hub_recovered", "hub")`，与既有 stale 沿同形一处消费；`vsp_realtime_adapter.gd` 零改（沿语义代码读澄清：recovered 仅在 stale 锁存后包恢复时一次性发射，严格蕴含先前 stale 沿，无需去重门控）。
- **探针（新形态：场景模式）**：`tools/probe_telem_ws_recover_consumer.tscn`+`_scene.gd`——autoload 全家引擎生产装配（真 manager+真 adapter+真 perf monitor，零 preload 自建），emit 走真适配器生产信号；host 为单元语义替身（本卡=autoload 信号沿消费面，非 GUI 装配卡）。17 检：W1 生产绑定面/W2 stale 沿回归/W3 恢复沿/W4 沿精度（其余 10 status 零调度零计数）/W5 防抖合并（双沿同帧→单次 dispatch）/W6 源面。**首试 -s 模式不可行**（telemetry_manager 引用 autoload 标识符 `VitDebugFlags` 编译期不解析，r1 留证）——场景模式为 telemetry 域既定出路。
- **验收数据**：前端 commit=**af700bd**（port/fe-telem-wsrecover-1 已推，3 文件 +244/−6）；探针 **17/17×2 连续两轮 exit 0**（r1 perf enabled 门+标记缺失、r2 summary 周期 clear 干扰，均探针面留证）；**全家零回退**：26 个 -s 件全 PASS+file_reply 场景件 PASS+本卡探针 PASS（fam2 完整导入态终轮）；pipeline_selftest=headless 环境边界件按 211 成文预期 exit 1 非回归项。工件=coord/runs/FE-TELEM-WSRECOVER-CONSUMER-1/wsrec_20261029_1（RESULTS.md 六节+13 件日志，随 **port/fe-telem-wsrecover-1-stack**@151df498 入库）。
- **环境事故留证（代码面无关四步链）**：fam1 popup 探针 icon 红→隔离复跑仍红→基线 37dc4b6 detached worktree 全绿二分→归因本 worktree 首次 import 被 segfault 截断在 reimport 98%（209 成文「末段 segfault 无碍」在本例不成立——截断资源导入）→补完整 import 后同分支 111/111 全绿。209/207 先例均在本卷互证。
- **端测覆盖边界声明（离线级交付，无真栈腿）**：适配器 stale→recovered 沿生成时序面=173 真栈腿已证且本卡零改 adapter；真栈复现需 hub 窗口模式停摆 15s+保持 socket（hub 无故障注入面，合成假服务器低于生产装配保真度）；消费链以生产 autoload 装配+真信号驱动全链实证；防抖落达真实主控链=212 r8/r9 已证同一生产入口。用户手测重验收口（可选）：正常使用期间重启 VSP hub 制造断供恢复，机架/时间线应在恢复后自动回拉（此前需任意触发）。
- **上交主管知悉**：ⓐ残余缺口（域外不扩域）：硬重连路径（断供>30s 硬阈值→websocket_closed→重连）无 recovered 沿（adapter :724-729 清锁存不发），该窗 stale 沿拉取之后的 delta 无恢复回拉——是否补由主管裁定；ⓑW1 绑定面实证：manager 声明序先于 adapter、静态分析疑 `_ready` 绑定落空，生产装配实证**绑定实动**（静态疑点被推翻，在卷备查）；ⓒtelemetry 域离线探针形态先例落地（-s 编译不可行→场景模式）。
- **[等待验收]** 主管裁定 pass/rework；实现分支 port/fe-telem-wsrecover-1 待验收不自行 merge。

## 主管验收（2026-10-29 晚）

- pass 并入 main：diff 亲读（生产面）+工件/退出码亲核+探针结果复核；214 含用户手测观感通过（12:11 同版，终值=128 默认）；213/215 合并面共存验证=两场景探针在合并后 main 各 17/17+15/15 exit 0（主管亲跑）；210 离线证据接受（卡面明示离线可验）。
- 上交项处置见主仓 accept 提交（213 硬重连缺口折 P3 卡待确认；214 四项观察级知悉；215 领取时序 slip 记档+主管侧折卡未提交失误一并记档）。
