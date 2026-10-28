# FE-LANE-POINT-CLICK-DEAD-1：车道画布真实鼠标点击无效——建点/交互死区（手测节第三轮缺陷卡 IV）

- 发卡：GLM 主管决策流 / 2026-10-28 晚（依据=用户手测：音量/声相可选✓+效果器参数可选✓后，画布单击无法生成节点、鼠标操作无效——编辑链被阻塞）
- 派发确认：已确认（用户 2026-10-28 22:37 本执行会话当场指派领取=派发确认落实）
- 验收负责人：GLM 主管决策流
- 池序 211；目标仓库=**前端仓 D:\Godot/project/vit-daw-frontend**；P1 / 0.5 天 / 依赖=209 已合入（基线=前端 main 84396f1）
- 模型分级：L2 / PC 执行会话
- 并行与资源：真栈腿错峰手测节

## 已核事实（发卡侧）

1. **用户会话进展=III-A/209 闭环**：音量/声相参数可选（新内核 190/201 面生效）+效果器 348 参数可选（信封通道）——目录与描述符链全通；唯余**画布交互死区**。
2. **strip 编辑逻辑真栈绿**：203 编辑手势机经 206/209 两轮探针复跑（建点/拖点/曲率/undo 30/30）——但探针=**handler/事件构造直调**（205 披露ⓔ起一直声明的未覆盖面），从未走真实事件投递管线。
3. **事件接收面静态勘察**：lane 容器展开态 STOP（:276/:280）、_body/_scroll/_stack PASS（:1089/:1101/:1109）、strip（内部类 EnvelopeParamLane）STOP（:1259）——静态链通；断点候选=①strip 在 206 行内布局流（HBox[spacer, ScrollContainer[VBox]]）新几何下的真实命中/裁剪；②事件到达 strip 但坐标/时间换算（timeline_seconds_at_global_point 改 _stack 变换：905-910）在建点路径上失效；③上游仲裁层（drop_router/行级 gui_input）在 wrapper 外新区域先收/吞事件（207 披露ⓐ的 mapping 边界变体）。
4. 内部类 EnvelopeParamLane 无独立文件（track_envelope_lane_2d.gd:1195 内部类）。

## 目标

1. 全 GUI 环境复现与定位（真实鼠标事件管线）：断点在哪一层（命中裁剪/坐标换算/仲裁先收/其他）——取证锚点回执披露；
2. 修复=单点建点链恢复（单击建点/拖点/hover 全交互面）；
3. 手测闭环：用户画线/拖点/曲率/undo 全套（206 挂起至今的验收面）。

## 探针义务（旅程面新口径：事件管线注入）

- **事件管线注入断言必入**：以 `Input.parse_input_event`（或等价真实投递管线）构造鼠标事件投递到场景——**禁止以 handler 直调替代真实事件路由验证**（本卡=该盲区成文案例：203/206/209 三轮事件构造探针全绿而真实鼠标死区）。
- §5.1 装配面+时序面+体积面延续；全家回归零回退。

## 文件域

`app/tracks/track_scene/track_envelope_lane_2d.gd`（含内部类 strip）+事件路由涉及文件（勘察定：drop_router/行控制器命中面——**域外文件需改→上交扩域裁定**）+探针。

## 停止条件

- 断点位于 Godot 引擎层行为（SubViewport/裁剪特性）非脚本可修→上交两案；
- 修复须改 wrapper/3D 层结构（右 3D 区布局）→上交扩域裁定。

- 领取：2026-10-28 22:37 / origin/main=186070e7 / owner=GLM-5.3 ZCode 执行会话（PC，本会话，用户当场指派=派发确认）/ 分支=port/fe-lane-point-click-dead-1 / worktree=D:/Godot/project/vit-daw-frontend-laneclick1（基线=前端 main 84396f1 亲核）/ 领取提交=0ad40a7b

- 回执（2026-10-28 23:4x，doing→done 交主管验收）：
  - **勘察结论（断点定案，DIAG r2-r4 取证锚点全在 run 工件）**：卡面三候选中**候选③坐实为复合形态**——①候选①（命中裁剪/新几何）**排除**（P0 生产装配+strip 真实几何 rect=(315,332,2237×96) 全绿）；②真实断链两层：**GUI 派发层=TimelineDropOverlay 网关截获**（r4 全链路由指认：parse_input_event 注入 press 后路由=[TimelineDropOverlay(PASS)→adapter]，strip 零收——overlay=画布区全幅 PASS 网关[timeline_dock_adapter `_ensure_drop_overlay`+`_has_point` 动态认领+`vit_timeline_overlay_forward_pointer_event` 显式转发链]，206 行内布局流后车道画布落入网关之下）；**_input 路由层=转发链无人认领**（r3 P1z0：fast_row 命中行而 mapping resolve kind=none——`_row_accepts_global_point` 的 wrapper rect 判定[TimelineViewMapping.gd:278-283]对 wrapper 外车道新区域 false=「207 披露ⓐ mapping 边界变体」坐实，仲裁器随之 IGNORE）；③**"按钮可用与画布死并存"自洽**：按钮=BaseButton 走 overlay 既有 interactive 让路面（用户"可选✓"），画布=网关认领后转发链无人管（死区）——两症状同源不同面；④**203/206/209 三轮绿而真死区原因**=探针裸 host（无 overlay/dock 生产宿主）+`_gui_input` 直调，生产 GUI 派发管线（本卡探针首次覆盖）从未被测——旅程面新口径成文案例坐实。
  - **修复（两文件，仲裁器/mapping/drop_router 零改）**：`vit_dock/views/timeline_dock_adapter.gd`（事件路由文件）`vit_timeline_overlay_should_accept_pointer_at_global` 加车道编辑面让路（`_global_point_targets_envelope_lane_edit_surface`：fast 行→`envelope_lane_hit_test_global` 非空→网关不认领→**GUI 派发穿透直达 strip._gui_input**，203 领土隔离设计在生产装配首次可达；车道收起恒空=既有行为零变化）+`app/tracks/track_scene/track_envelope_lane_2d.gd`（域内）strip press 行选中联动（`_notify_lane_edit_press`——原仲裁器 envelope 分支 timeline_select_this_track 语义编辑面内补齐；裸 host 探针装配安全跳过=203 离线件零回退亲证）。
  - **验收数据**：前端 commit=**6ade42d**（port/fe-lane-point-click-dead-1 已推，6 文件 +695：修复 35 行+探针 660 行）；真栈 run=coord/runs/FE-LANE-POINT-CLICK-DEAD-1/pointclick_20261028_1（工件随 **port/fe-lane-point-click-dead-1-stack** 分支入库，RESULTS.md 断点定案+验证矩阵+四披露）；栈登记 8600e965/释放 67415d67（三纪律全守：登记先于启动+推送后即时复核+vsphub 手启前亲核+拆栈三 pid 路径逐一亲核）。探针 stack_probe_lane_point_click_pipeline=**旅程面新口径义务兑现**（Input.parse_input_event 管线注入+§5.1 装配面=timeline_dock_adapter.tscn 生产 dock 实件首次入场零自建）：**正式轮 30/30 PASS×2 连续 exit 0**（P0 装配+几何/P1 overlay 让路+仲裁器不拦/P2 管线建首点全链[press 抵手势机→drag→恰一笔→回显门→内核读回逐点一致]/P3 折线加点/P4 拖点改值/P5 曲率柄 c=-1.0/P6 hover 零副作用/P7 Ctrl+Z/Y undo-redo 往返）+DIAG r2-r4 基线死区签名+**离线全家 27/27 exit 0**+真栈族既有 3 件零回退（203 edit 30/30+206 grow 16/16+202 rebuild 23/23）。
  - **异常如实分类**：探针迭代三轮全为探针面非生产面——v1 发现函数轨行键漏 track_id 候选（203 原版对照修正）；P7 选位（两点曲线右延伸无命中面=194 设计边界披露→段中点恰为曲率柄位进曲率拖拽→0.38 段位沿曲线采样值修正）；r3 一次 gp_press 声明序 Parse Error（209 事故同款，timeout 干净退出未触栈浪费）。**环境边界成文**（probe_pipeline_selftest v2 入库留证）：headless 下 GUI 派发不达（_input 通 gui_input 零收）→旅程面探针须窗口模式；注入序列 motion 先行建立 hover 态否则不路由。
  - **端测覆盖边界声明**：探针窗口模式=真实 GUI 派发管线（与用户手测同构），但非用户键鼠；栈为探针最小腿（-StartKernel -RestartAgent，无 UI 全旅程）。
  - **[等待用户:手测闭环]**——用户从 Godot 前端拉起进入工程，画线（空车道单击建点）/拖点/曲率（柄拖+右键预设）/undo 全套（206 挂起至今的验收面）；主管验收含上件。
  - **上交主管知悉**：让路后原仲裁器 envelope 分支的"清 clip 选择+聚焦边框高亮"两联动未在编辑面复刻（行选中已补；低频边缘面，是否补齐由主管裁定）；两点曲线右延伸区无加点命中面=194 既有设计边界（探针 P7 披露），如需"右延伸加点"为独立小卡。
- 验收：（待主管 ruling）
