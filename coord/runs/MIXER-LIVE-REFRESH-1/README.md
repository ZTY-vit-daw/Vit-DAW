# MIXER-LIVE-REFRESH-1 工件索引（池序 141 / task-10）

- owner=DSH PC 执行流 exec-fe-gui-c；前端仓分支 **port/fe-gui-shrink-c-floating**（worktree D:/Godot/project/vit-daw-frontend-fe-gui-c，基线 f6abe20→51a9882→本卡 commit）；主仓探针分支 **port/fe-gui-shrink-c-probe-1**。
- 现象：用户 R2 复测发现**新建轨道后 mixer 不出新通道条/推子不刷新**（同期 kernel/agent/vsp hub 在跑，非未连内核）。

## 事件链断点（取证）

1. 工程数据广播链：`vit_dock_root.gd::_apply_kernel_project_reply_and_emit()` → `project_event_hub.project_loaded.emit()`；mixer `_on_project_loaded()` → `_rebuild_strips()`（该段本身完好）。
2. 断点①：`vit_dock_root.gd::_on_vit_ipc_command_completed_for_dock_repo()` 的"命令完成后回拉工程快照"白名单只含插件/机架/clip 类命令，**缺轨道结构命令**（`add_track`/`delete_track`/`folder_track.*`）→ 命令完成不触发回拉/广播 → mixer 停在旧轨道集合。
3. 断点②：`ProjectEventHub` 无"轨道集合变更"信号（只有 project_loaded/track_focus 等）；`mixer_dock_adapter.gd::_bind_project_events()` 仅订阅 project_loaded/track_focus；`_bind_ipc()` 的 completion 只处理 `track.group.*`。
4. GUI 外部（agent/内核直连）建删轨道时 GUI 无命令完成事件 → 需要既有实时事件流兜底。
5. main 同源核对：主检出（D:/Godot/project/vit-daw-frontend @ c56e38f Phase A）的 `_on_vit_ipc_command_completed_for_dock_repo` 白名单同样缺轨道结构命令、mixer 同样只订阅 project_loaded → **缺陷同源**；修复随 C 线分支由 FE-GUI-MAINLINE-1 带入 main。

## 修复实现（最小面）

1. `vit_dock_root.gd`：新增 `TRACK_STRUCTURE_COMMANDS`（add_track/delete_track/folder_track.create/folder_track.set_routing_bus_enabled + 防御性 rename_track/set_track_name/update_track/move_track/reorder_tracks/set_track_order）+ `is_track_structure_command()`；在 IPC 完成处理器中命中即 `await vit_refresh_project_from_kernel()`（回拉快照+广播 project_loaded）。→ mixer/timeline/rack 等所有订阅视图受益。
2. `mixer_dock_adapter.gd`：
   - IPC completion 增加轨道结构命令分支（仅 standalone/无 main_ui_controller 场景触发，dock 内不重复刷新）；
   - 实时电平事件流 `apply_engine_level_telemetry()` 中做**轨道集合变化检测**（与已知 track id 集合比较），变化即调度一次**节流结构重同步**（500ms 合并窗口），覆盖 agent/内核外部建删轨道；
   - 新增 `mixer_state()` 探针面（strip 数/轨道集合/推子 dB/仓库命名/颜色/0dB/重同步状态）。
   - 重建仍走 `project_loaded → _on_project_loaded → _rebuild_strips`，保留虚拟化/滚动/组/预设逻辑；无全量轮询（仅事件+合并窗口）。

## 探针/电池（Phase C 342 钉 PASS）

- 新增 23 钉：静态契约 8（root 命令集/predicate/广播链、mixer fallback/检测/节流/rebuild 保持/探针面）+ 装配 15（真实装配 mixer 后经 root project_loaded 链注入：2 轨→strip 数/轨道集合/命名/颜色/0dB 推子；加第 3 轨→3 strips+0dB；改名同步；删轨→2 strips；实时电平流 track 集合变化→调度重同步；predicate 接受/拒绝命令）。
- 回归：Phase A 69 / Phase B 119 / vitnote 六 / FE-RACK 76 全 exit 0（电池目录 `../FE-GUI-C-FLOATING/20261018_145205/`）；红线门 924318d..worktree 9 文件零命中。
- 端测边界：headless 无真实内核 —— 事件链以「真实装配 mixer + root 广播链注入合成内核工程快照」+「静态契约钉 IPC 命令集」覆盖；真实内核 IPC 回拉/agent 外部建轨由用户真栈手测闭合（本卡不占栈）。

## 需用户复测

① 真栈下新建轨道（timeline + 轨道按钮/右键新建）→ mixer 立即出现新通道条、推子 0dB、命名正确；② 通过 agent/对话新建轨道 → mixer 亦自动刷新（实时事件流兜底）；③ 删除轨道 → 通道条消失、其余重排；④ 混音器既有操作（推子/组/预设/滚动/虚拟化）无回归。
