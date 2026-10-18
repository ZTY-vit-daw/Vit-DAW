# MIXER-LIVE-REFRESH-1：新建/删除轨道后 mixer strips/推子不刷新（GUI↔内核事件链缺口）

- 发卡：12:00-18:00 决策流 / 2026-10-18 午窗（依据=用户 2026-10-18 R2 手测观察：新建轨道后 mixer 推子不刷新；同期 kernel/agent/vsp hub 日志在场=GUI 已连内核，**非"未连内核"**）
- 派发确认：**已确认（用户 2026-10-18 手测反馈并口头要求调整；P1 修复）**
- 验收负责人：GLM 主管决策流（或当班决策流）
- 池序 141；目标仓库=前端仓（C 线，主整合前修）；**P1** / 预估 0.5 天 / 无依赖
- 模型分级：L1/L2 / PC 前端执行流

## 现象与机制（代码已核）

- `vit_dock/views/mixer_dock_adapter.gd`：`_bind_project_events()` 仅连接 `project_loaded` / `track_focus_changed`；`_rebuild_strips()` 触发点=setup_view / 组选择 / project_loaded / focus 变化；**无轨道增删/重命名/排序事件订阅**；`_bind_ipc()` 只用于命令完成后的电平/状态更新，未在 track 命令完成后重建 strips。
- 结果：**新建轨道后 mixer 不出现新通道条/推子**（fader 不刷新）；删除/重命名同理可能不同步。
- 证据：用户 R2 手测会话（14:30/14:43）主检出 `VitHeadlessServer2026-10-18_14-43-23.log`、`agent_last.log`（14:43:42）、`vsp_hub_last.log`（14:43:37）、baker 大日志同期在卷——**内核/agent/hub 均在跑，GUI 已连接**；属事件刷新缺口，非环境未连内核。是否 main 同源由本条修复卡取证（C 线 R2 前该文件与 main 差异极小，预期同源既有缺口）。

## 目标

- mixer 在**轨道增/删/重命名/排序**后自动重建 strips 与推子；新轨道默认推子 0dB、正确命名与颜色；不破坏虚拟化/横向滚动/组选择/预设功能。
- 事件源优先 `ProjectEventHub`（若缺轨道变更信号，则用 track_client / IPC 命令完成 + 轻量 pull；不得全量轮询硬刷）。

## 文件域

`vit_dock/views/mixer_dock_adapter.gd`（必要时同视图 tscn）；`ProjectEventHub`/`track_client` 只读引用（**若需给 ProjectEventHub 新增信号→先报 Lead**）；`tools/probe_fe_gui_shrink_phase_c.gd` 补钉；`coord/runs/MIXER-LIVE-REFRESH-1/`。越域即停。

## 验收标准

1. headless 探针（真实装配）：模拟/注入轨道新增事件后 mixer strip 数与推子状态更新（新通道条 + 0dB）、删除/重命名同步；虚拟化与滚动不退化；R1/R2 既有 319 钉不得回退。
2. 回归电池（Phase A 69/Phase B 119/vitnote 六/FE-RACK 76）全 exit 0；红线 9 文件零命中。
3. 用户复测：工程内新建轨道 → mixer 立即出现新通道条+推子；删除/重命名同步。
4. 桩证据：事件链锚点（哪个信号、谁发、mixer 如何重建）+ 若 main 同源问题如实登记。

## 停止条件

- 需改内核/VSP 协议或跨视图架构 → 先上报；不得以定时轮询掩盖事件缺失（允许事件+节流）；不得隐藏/删 mixer 功能凑通过。

## 关联

- 发现卡：FE-GUI-C-FLOATING（92，GUI 改版验收；实时链路不在视觉验收面）
- 后续：FE-GUI-MAINLINE-1（主整合时合并本条修复）

- 领取：2026-10-18 14:55 +0800 / origin/main=77a0b02f / owner=DSH PC 执行流 exec-fe-gui-c / 分支=port/fe-gui-shrink-c-floating（C 线连续 commit）/ worktree=D:/Godot/project/vit-daw-frontend-fe-gui-c / 领取提交=51a9882（R2）→ 任务开始 be6da5d / task-10；用户 2026-10-18 手测反馈确认派发
- 回执（2026-10-18 午窗，exec-fe-gui-c 执行，Lead 转写）：实现 commit **be6da5d**（父 51a9882；3 files +283：vit_dock_root.gd / mixer_dock_adapter.gd / C 探针；红线 9 文件零触碰；未 push main）；探针分支 **cf2f39a7**（工件 coord/runs/MIXER-LIVE-REFRESH-1/ + FE-GUI-C-FLOATING/20261018_145205/）。**事件链断点（取证）**：①广播链本身完好（root _apply_kernel_project_reply_and_emit→project_event_hub.project_loaded→mixer _on_project_loaded→_rebuild_strips）；②断点①=root _on_vit_ipc_command_completed_for_dock_repo 回拉白名单无 add/delete/folder_track.*（轨道结构命令完成不触发回拉广播）；③断点②=ProjectEventHub 无轨道集合变更信号、mixer 仅订阅 project_loaded/track_focus、_bind_ipc 仅处理 track.group.*；④旁路=GUI 外部（agent/内核直连）建删轨道收不到命令完成事件；⑤**main 同源核对**=Phase A main 同白名单缺失+mixer 同订阅面→缺陷同源、非 C 线引入，修复将随主整合入 main。**修复**：root 新增 TRACK_STRUCTURE_COMMANDS 白名单 + is_track_structure_command()，IPC 完成命中即 it_refresh_project_from_kernel()（回拉快照+广播 project_loaded→mixer/timeline/rack 全链路刷新）；mixer 在 pply_engine_level_telemetry 内做轨道集合变化检测→500ms 节流合并→触发结构重同步（覆盖 agent/外部建删轨道，非全量轮询）；IPC 结构命令分支仅 standalone 兜底（避免重复）；新增 mixer_state() 探针面；重建仍走 project_loaded→_rebuild_strips，虚拟化/滚动/组/预设逻辑未动。**验证**：Phase C **342 钉 PASS**（R2 319 全保留+新增 23=静态契约 8+真实装配 mixer 15：注入 2 轨→strip 数/轨道集合/命名/颜色/推子 0dB；加第 3 轨；改名；删轨；实时电平流 track 集合变化调度重同步；predicate 正反例）；回归 Phase A 69/B 119/vitnote 六/FE-RACK 76 全 exit 0；红线门 924318d..be6da5d（36 文件）零命中。决策侧亲核=远端 hash（be6da5d/cf2f39a7）+commit stat+Phase C log checks=342 failed=0 PROBE PASS+红线门亲跑。**端测边界**：headless 无真实内核（事件链以真实装配 mixer+root 广播链注入合成快照+静态契约 IPC 命令集覆盖）；真实内核 IPC/agent 外部建轨归用户真栈复测（本卡未占栈）。**待用户复测**：①真栈新建轨道→mixer 立即出新条/0dB/命名；②agent/对话建轨→自动刷新（实时流兜底）；③删轨→消失重排；④推子/组/预设/滚动/虚拟化无回归。
- 验收（2026-10-18 晚窗 GLM 主管决策流）：**pass（终验）**——用户复测四项全过（①真栈建轨即时出新条/0dB/命名 ②agent/对话建轨自动刷新 ③删轨消失重排 ④推子/组/预设/滚动/虚拟化无回归；「推子同步」面含于④）。修复 be6da5d 已在 frontend main（3ab4dfd 合入链）；实现+探针+回归+红线+取证链此前已亲核（BATCH 系列在卷），真栈四项闭环。用户收口语境登记：本缺陷属「新 GUI 未接全」一族——新 GUI 最终直接接回原项目，此类接线缺口随主整合（FE-GUI-MAINLINE-1 线）一并收敛。卡终态=done。
