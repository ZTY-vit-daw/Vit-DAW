# FE-AUTOLANE-PRODCHAIN-FIX-1：车道生产链修复——轨道 id 懒创建绑定+弹层屏幕坐标（手测节发现）

- 发卡：GLM 主管决策流 / 2026-10-27 晚（依据=用户手测节实证三缺陷中两项+发卡侧代码亲核根因闭环）
- 派发确认：已确认（手测节发现链）
- 验收负责人：GLM 主管决策流
- 池序 205；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**（领取基线 main=f52b78f）；**P1** / 0.5 天 / 依赖=无
- 模型分级：L1 / PC 执行会话
- 并行与资源：真栈腿与用户手测节错峰（§2.2 增补③——用户现场可能仍在跑）。

## 缺陷事实（发卡侧代码亲核，2026-10-27 手测节实证）

1. **轨道 id 懒创建断链**：`get_track_envelope_lane()` 懒创建走 `_ensure_envelope_lane_2d_layer(false)`——创建时不同步轨道 id；行装配期绑定先于容器存在、`bind_virtual_track` 跟随（:2650/:2677，198 补丁在位）仅在容器存在后的重绑生效→生产 GUI 容器恒空 id→目录查询全空→选择器只剩发送占位组（用户实证：无音量/声相/插件组）。
2. **弹层坐标错置**：`_on_selector_pressed` 以 `_selector_button.global_position`（canvas 坐标）直接赋 PopupMenu（Window）`position`（屏幕坐标）→弹层出现在窗口顶部而非点击处（用户实证）。
3. **探针注入盲区（验收判定面缺口，主管记档）**：202/204 真栈腿目录经 `set_plugin_catalog`/`set_track_volume_plugin_id` 注入（stack_probe :353-354），生产 repo 目录链零覆盖——「E2 三组收口」非生产链证据。

## 目标

1. **id 绑定修复**：懒创建即推当前轨道 id（`_ensure_envelope_lane_2d_layer(true)` 或创建路径等效推 id）——覆盖「行已绑→后创建」时序；id 到达且展开态时目录刷新（既有 set_kernel_track_id 行为保持）。
2. **弹层定位修复**：选择器/曲率预设弹层改屏幕坐标映射（canvas→screen；对齐仓库既有弹层定位先例——轨道右键菜单/timeline_dock_adapter popup 模式）；headless 守卫保持。
3. **探针生产链断言（本卡核心义务）**：离线——真行装配（track_row.tscn）+真 repo 形态（带 rack_nodes 快照的 project_repository 或等价真路径）断言：开关展开→容器 id=行当前 id→**目录三组从生产源到达**（轨道组恰 volume/pan+插件组实载）——**禁 set_plugin_catalog/set_track_volume_plugin_id 注入**；既有注入态探针标注「注入边界」不删（它们测编辑/写回面仍有价值）。真栈腿同理禁注入，目录走真实 repo 链。
4. **回归**：全家探针零回退（202/203 面）。

## 文件域

`app/tracks/track_scene/Track_Row_Controller.gd`（懒创建 id 推送）+`app/tracks/track_scene/track_envelope_lane_2d.gd`（弹层坐标）+探针（新断言+注入边界标注）。不触 vit_dock/**、app/rack/**。越域即停。

## 验收标准

独立探针全绿（含生产链目录断言）+真栈两轮（**无注入**：装载 VST→展开→选择器三组在场→默认音量车道绑定→建点→内核读回一致）+弹层定位断言（屏幕坐标映射单测或等价断言面）+工件入 coord/runs/FE-AUTOLANE-PRODCHAIN-FIX-1/；端测边界声明。

## 停止条件

- 真 repo 快照形态与探针假设不符（rack_nodes 键型/结构漂移）→锚点上交；
- 弹层坐标映射在多 viewport/缩放场景仍有偏移（Godot 嵌入 viewport 变换复杂度超预期）→取证上交两案。
