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
