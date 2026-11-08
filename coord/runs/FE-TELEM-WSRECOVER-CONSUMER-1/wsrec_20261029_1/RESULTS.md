# RESULTS — FE-TELEM-WSRECOVER-CONSUMER-1（213）wsrec_20261029_1

执行：GLM-5.3 ZCode PC 量产执行会话（L1 档）/ 2026-10-29 晚窗
被测：前端仓 worktree `D:/Godot/project/vit-daw-frontend-wsrec1` @ `port/fe-telem-wsrecover-1`（基线=前端 main 37dc4b6，领取记录见卡面）
性质：**离线卡（零真栈腿、零端口触碰）**——栈记录无占用登记义务（未启 kernel/agent/hub/Godot GUI，全部探针 headless）

## 一、根因与修复面

`vsp_realtime_adapter.gd:718` 发射的 `websocket_recovered`（stale 锁存解除沿，一次性非轮询）全仓零消费者（grep 亲证：`vsp_realtime_status_changed` 唯一连接点=telemetry_manager.gd:171-172；`websocket_recovered` 仅 :718 发射+一处注释）。断供窗内丢失的 delta 不被 hub 重放，stale 沿拉取只覆盖其前缺口。

修复（`app/kernel/autoloads/telemetry_manager.gd` 单文件 +9/−6）：`_on_vsp_realtime_status` 补恢复沿分支——`websocket_recovered → _perf_count("telemetry.hub_recovered_refresh_scheduled") + _schedule_project_refresh_from_delta("hub_recovered", "hub")`，与既有 stale 沿（:664-668）完全同形，一处消费。`vsp_realtime_adapter.gd` 零改（沿语义代码读已澄清：recovered 仅在 `_ws_stale_latched==true` 后包恢复时发射一次，严格蕴含先前 stale 沿，无需去重门控）。

## 二、探针面（新形态：场景模式）

`tools/probe_telem_ws_recover_consumer.tscn` + `_scene.gd`。**首试 -s 脚本模式不可行**：telemetry_manager.gd:219 引用 autoload 标识符 `VitDebugFlags`，`-s` 模式 GDScript 编译器不解析（r1 留证 `probe_dash_s_mode_compile_block_r1.log`）——改场景模式（`probe_file_reply_volume_face` 同族先例），保真度更高：**autoload 全家由引擎按 project.godot 生产装配**（真 VitTelemetryManager+真 VspRealtimeAdapter+真 VitPerfMonitor），emit 走真适配器生产信号，零 preload 自建；刷新 host 为单元语义替身（被测面=autoload 信号沿消费→防抖回拉链，非 GUI 装配卡，§5.1 不适用）。

断言面 17 检：W1 生产绑定面（adapter 信号→manager 生产 handler 恰一连接）；W2 stale 沿回归（perf `hub_stale_refresh_scheduled`+1→防抖后 host `_request_refresh_project_state_after_edit` 恰一次=repo/机架重拉生产入口）；W3 恢复沿（同形 +1+恰一次）；W4 沿精度（status 其余 10 值零调度零计数——一处消费非轮询）；W5 防抖合并（stale+recovered 同帧连发→单次 dispatch 双沿计数）；W6 源面（卡标记+分支在案）。

真计数 monitor 经生产开关 `VIT_PERF_MONITOR=1` 启用；探针内调 `VitDebugFlags.per_summary_interval_ms`→60s 防启用态 2s 周期 clear 干扰跨相位断言（探针本地稳定化）。迭代轮留证：r1=perf `enabled` 门+卡标记缺失（7 败）、r2=summary 周期 clear 干扰 W5 计数读取（2 败）、r3/r4=**17/17 全绿 exit 0 连续两轮**。

## 三、全家回归零回退

完整导入态终轮（fam2，`fam2_full_table.log`）：**26 个 -s 件全 PASS + file_reply 场景件 PASS + 本卡新探针 PASS，零回退**。`probe_pipeline_selftest`=headless 环境边界件，按 211 成文预期呈现 `input_stage=true unhandled=true press_at_box=false`→exit 1（其退出语义 `quit(0 if _got_press else 1)`，真断言面需窗口模式——**非回归项不计入 -s 全家**，输出签名与 211 文档一致）。

## 四、环境事故与二分留证（与代码面无关实证）

fam1 首轮 `probe_rack_plugin_search_popup` 1/111 红（`scene tree leaf carries library-same icon | leaf icon null`）：

1. 本卡 worktree 隔离复跑仍红（同断言）；
2. 基线二分：detached worktree@37dc4b6 完整 import 后 111/111 PASS（`popup_probe_baseline_37dc4b6_pass.log`）；
3. 归因：本 worktree 首次 `--import` 尾段 segfault 发生在 reimport **98% 进行中**（`wsrec1_first_import_truncated.log`，209 成文的"末段 segfault 无碍"在本例不成立——截断了剩余资源导入，图标纹理缺失→icon null；207 同探针「缓存态红」同类环境面）；
4. 补完整 import（rc=0）后同分支 111/111 PASS（`popup_probe_after_import2_pass.log`）。

结论：环境面（导入不完整），与生产 diff 零关联，四步证据链在卷。

## 五、边界声明（AGENTS §5 义务）

- **本卡离线级交付，无真栈腿**。理由：①适配器 stale→recovered 沿生成时序面已由 173 真栈腿亲证且本卡零改 adapter；②真栈复现"断供→恢复"需 hub 在窗口模式停摆 15s+ 且保持 socket——hub 无故障注入面，合成假 WS 服务器反而低于生产装配保真度；③场景探针以生产 autoload 装配+真信号驱动消费全链。残余风险=防抖回拉落达真实主控（`main_ui_controller` 组）后的 UI 呈现，该链为 212 r8/r9 已证生产链（同一 `_request_refresh_project_state_after_edit` 入口）。用户手测重验收口（可选）：拉起 Godot 进工程，正常使用期间重启 VSP hub 制造断供恢复，观察机架/时间线在恢复后自动回拉（此前需任意触发才刷新）。
- 探针进程全部 headless 自退，无残留（UDP 4444 未触碰：场景腿 `VIT_GUI_LEGACY_UDP_TELEMETRY_DISABLE=1`，-s 腿无 manager 实例）。

## 六、上交主管知悉

1. **残余缺口披露（域外，不扩域）**：硬重连路径（断供>30s 硬阈值→`websocket_closed`→重连重订阅）无 recovered 沿——`_check_ws_stale_watchdog` 硬阈值分支清锁存不发 recovered（vsp_realtime_adapter.gd:724-729），该窗内 stale 沿拉取之后的 delta 无恢复回拉，等下一任意触发。是否补该路径（如 websocket_open 后按"曾有 stale"门控回拉）由主管裁定，本卡不动。
2. **W1 绑定面实证**：manager（project.godot 第 8 位）先于 adapter（第 11 位）声明，静态顺序分析提示 `_ready` 绑定可能落空；场景探针生产装配实证**绑定实动**（W1a 恰一连接指向 manager 生产 handler）——静态疑点被生产证据推翻，在卷备查。
3. **探针形态先例**：telemetry 域离线探针首次落地，`-s` 模式对引用 autoload 标识符的生产脚本不可行（编译期），场景模式为既定出路——后续 telemetry 域探针可直接复用本形态。
