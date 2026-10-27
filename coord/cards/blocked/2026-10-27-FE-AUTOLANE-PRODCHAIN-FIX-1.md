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

## 领取

- 2026-10-27 19:22 / origin/main=8791405c / owner=GLM-5.3 PC 执行会话（ZCode，本卡会话，PC 机）/ 前端仓分支 port/fe-autolane-prodchain-fix-1@基线 main=f52b78f / worktree D:/Godot/project/vit-daw-frontend-autofix1 / 主仓协调分支 coord/fe-autolane-prodchain-fix-1-claim（worktree D:/Vit_DAW_wt_autofix1_coord）/ 领取提交=397176ea

## 回执

- **状态：blocked——停止条件①命中，锚点上交主管裁决（2026-10-27 21:2x）**
- 实现：前端仓 `port/fe-autolane-prodchain-fix-1` @ **8295ca4**（已推远端；ed64f0f 实现+8295ca4 探针加固/诊断件）。①懒创建即推轨道 id（`get_track_envelope_lane()` 慢路径 false→true）+②弹层 canvas→屏幕坐标（选择器/曲率预设两处 embed 两态取实时鼠标，对齐 timeline_dock_adapter 先例；headless 守卫保持）+add_lane 轨道优先分组（生产链探针发现的注入盲区分歧）+get_kernel_track_id 读回面。
- 离线：新探针 `probe_autolane_prodchain` **47/47**（真行装配+真 ProjectRepository 生产写入路径+禁注入三组+懒创建时序+弹层定位断言）；**基线负对照红绿证据**（f52b78f 懒重建容器 id=空 vs 修复=行当前 id）；全家回归 **23 件全绿零回退**（rebuild 73/edit 63 与 203 收据同数）；注入边界标注 8 件（不删）。
- 真栈：§2.2 登记 76619335（20:49）+12s 即时复核+三纪律+VIT_PROJECT_XML 隔离亲证（287956e7→072ac158）；ps1 最小腿 exit 0；`stack_probe_autolane_prodchain` **R1b 17/20（核心面全绿）**：A5 懒创建即推 id **修复面真栈实证**（修复前恒空=用户手测根因）+A2 生产绑定 registry 落册+**S2 目录从真 repo 无注入到达**；3 红=同一根因（下述阻塞）。
- **停止条件①命中（锚点上交）**：racked 轨（rack_add_node 装载 VST 后）内核轨行 rack 字典**独占** `rack_nodes` 快照→轨级 Volume & Pan 插件行被排除→`get_volume_pan_plugin_item_id_for_track` 恒空（plugins 兜底=死路：tracks_out 无 plugins 键）；消费面①车道轨道组+默认音量车道（用户「无音量/声相组」第二层根因，与缺陷①复合）②轨道头声相推子 IPC 静默 no-op。**修复点 `vit_dock/core/project_repository.gd`（apply_kernel_project_state rack 分支+get_volume_pan_plugin_item_id_for_track 兜底）=卡面明令不触域**→按 AGENTS §11 锚点上交；最小复现件 `tools/stack_diag_205_rackshape.gd/.tscn`（真栈 30 行留证）。真栈验收「三组在场+默认音量车道→建点→读回」链在 racked 轨被阻塞，两轮全绿需主管裁决（扩域单点修复或另开卡）后重放探针（R1b 形态就绪）。
- 工件：run=`coord/runs/FE-AUTOLANE-PRODCHAIN-FIX-1/prodchain_20261027_2050/`（**分支 port/fe-autolane-prodchain-fix-1-stack @ 326dcd9e**，RESULTS.md+R1/R1b/diag 日志+离线回归 26 件+隔离副本）；栈登记 76619335–释放 d7c8d2d8（21:21 拆栈全零）。
- 端测边界：弹层真窗口落点未过自动化断言（headless 无窗口面；policy 单测+鼠标源集成断言=卡面等价断言面），留用户手测节；渲染面/手感面同留手测。

## 验收

（待主管裁决：blocked 处置——扩域授权单点修复 project_repository.gd 后重放真栈探针收口，或另开 repo 修复卡+本卡部分验收）
