# M3-ENVELOPE-LANE-UI-1：包络车道 UI 呈现+选中（M3 步骤③实施 C2）

- 发卡：GLM 主管决策流 / 2026-10-25 晚窗（依据=[M3-ENVELOPE-LANE-RECON-1 验收](../done/2026-10-25-M3-ENVELOPE-LANE-RECON-1.md) 标注点裁定④=C 案+拆分建议 C2；依赖 C1=M3-ENVELOPE-DATACHAN-1 已验收合入 ✓）
- 派发确认：已确认（M3 步骤③实施序列，186 验收裁定链）
- 验收负责人：GLM 主管决策流
- 池序 189；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**（领取基线 main=3413fe8 含 C1）；P2 / 1-1.5 天 / 依赖=C1 合入（✓）
- **依赖补充（2026-10-25 用户设计裁定后，主管补注——执行侧 fetch 后按本节+设计裁定节执行）**：自动展开腿依赖 M3-ENVELOPE-PRESENCE-1（190，内核在场性字段，即开）——190 未合入时本卡先交「手动开关+选中呈现+刷新回读」面，自动展开腿随 190 合入后补验（同卡两段式，不拆新卡）。
- 模型分级：L2 / PC 执行会话（时间线主场核心面+手势词表扩展）
- 并行：与 185 续跑（app/rack 域）/188（主仓 scripts）文件域不相交可并行；与 C4（写回闭环）**同文件域串卡**——本卡先呈现+选中，C4 后写回。

## 设计裁定（用户 2026-10-25 形态讨论确认——本卡呈现面的权威约束，优先级高于下方目标条目的形态自由度）

1. **形态=子车道**（Cubase/Logic/tracktion 主流派）：波形下方矮条车道，包络与波形同屏对照（=186 裁定④ C 案，用户确认，推翻权行使期结束）。
2. **显示时机=按需+手动混合**（用户原话「按需出现和手动开关应该都需要」）：①手动开关在轨道头（开=显示车道含空带、关=隐藏；首版三态「自动/开/关」默认自动，勘察后可简化须给理由）；②自动展开=刷新链发现该轨任一参数有包络→车道展开（数据源=190 的 envelope_point_count；用户显式隐藏后，同参数新包络到达再次触发）。**诚实边界**：无实时推送（C1 已证 delta 不携带曲线），自动发现时机=工程刷新/rack 状态拉取时刻，非写入即弹。
3. **参数呈现=单车道+切换器**：一条车道显示当前选中参数，车道头切换（左右步进+下拉选参）；多车道堆叠=后续增强不入本卡。
4. **曲线细节**：点+折线呈现，值域标签随参数描述符单位（vol/pan=dB/%，插件参数=归一值或描述符单位）；c 曲率首版只呈现不编辑（编辑归 C4）；空曲线=fail-closed 灰条+「（空）」。

## 目标（依 RECON §一/§五 C2 行+裁定④=C 案+上节用户设计裁定）

1. **车道挂载（C 案定案）**：新文件 `app/tracks/track_scene/track_envelope_lane_2d.gd`（TrackEnvelopeLane2D），`_ensure_lane_2d_layer`（Track_Row_Controller.gd:1033-1063）扩分支挂载；行高联动按勘察 1.4 评估（案 C「行高联动另计」——首版可=车道条固定高度叠加或行高+定值，形态执行侧勘察定，原则=波形 lane 零行为改变）。
2. **曲线呈现**：经 C1 `AutomationClient.get_curve`（automation_client.gd）按需拉取；points[{t,v,c}] 时间→x 映射复用 `vit_track_lane_2d.gd:1470-1482` 同式（`_visible_range()` 秒窗）；播放头联动复用 GuiTransportClock 三级回退链（RECON 1.3）；空曲线/加载失败 fail-closed 形态。
3. **参数选中面（单车道切换器）**：车道头参数选择器（左右步进+下拉，消费 `get_plugin_parameters` 描述符缓存 `Vit_Graph_Rack.gd:347/_param_port_rows_by_plugin` 同源面或 repo 快照——形态勘察定）；选中 (track, plugin, param) 后 get_curve 拉取呈现；切换器状态入 store（预览态面）。
4. **交互分派**：hit-kind 词表（TimelineViewMapping.gd:116-228）加 envelope 族 kind；仲裁器 match（TimelineInputArbitrator.gd:503-550）分派到车道；**本卡只做选中/聚焦+呈现**，点编辑（拖点/加点）归 C4；4px 手势状态机复用。
5. **探针**：独立探针（新文件）断言车道在场/曲线点绘制读回（绘制面以 lane 内部状态断言）/参数选中切换/get_curve 真实管线；真栈腿=装载插件→get_curve 空曲线披露→C1 L2 同款三点写入（探针侧直调 client）→车道呈现读回一致（§2.2 登记+188 落地前按 185 裁定③三纪律执行）。

## 文件域

`app/timeline/`（TimelineViewMapping/TimelineInputArbitrator/TimelineEditStore）+`app/tracks/track_scene/`（新 lane 文件+Track_Row_Controller 挂载）+探针新文件；vit_track_lane_2d.gd **预期零改**（C 案定义性边界——要改先上交）。越域即停。

## 验收标准

独立探针全绿+既有探针零回退（K3/popup/realinput/shrink abc/start_feed/fader 35/datachan 50）+真栈腿 exit 0（呈现读回一致）；渲染面断言（车道 DOM/绘制态）入探针；工件入主仓 coord/runs/M3-ENVELOPE-LANE-UI-1/；端测边界声明。

## 停止条件

- 车道工厂/仲裁词表与 186 锚点不符（已重构）→锚点上交；
- 行高联动须改 vit_track_lane_2d.gd 才能成立（C 案侵入假设失效）→上交再裁（可能转 A 案）。

- 领取：2026-10-25 20:21 晚窗 / 主仓 origin/main=6009b37c / 前端仓领取基线 main=3413fe8（C1 合入亲核；worktree git status --short 干净+git diff --stat 空）/ owner=ZCode GLM-5.3 PC 执行会话（M3-ENVELOPE-LANE-UI-1 专属）/ 前端分支 port/m3-envelope-lane-ui-1 / worktree D:/Godot/project/vit-daw-frontend-m3envui1 / 领取提交=本提交
- 回执：2026-10-25 晚窗自验完成，交主管验收（详 [runs/M3-ENVELOPE-LANE-UI-1/envelope_lane_gui_20261025_2048/RESULTS.md](../../runs/M3-ENVELOPE-LANE-UI-1/envelope_lane_gui_20261025_2048/RESULTS.md)）——
  - **commit**：前端仓 `port/m3-envelope-lane-ui-1@ea9fa23`（实现：lane 新文件+工厂扩分支+参数选中面+词表/仲裁分派+两探针）`+4f9d8a5`（真栈探针三轮缺陷修正，全探针侧零实现返工）；主仓 `port/m3-envelope-lane-ui-1-stack`（真栈工件+RESULTS.md）；均已推远端。**vit_track_lane_2d.gd / rack_client.gd / dev_agent_smoke.ps1 全部零改**（git diff 亲证；188 文件域纪律遵守）。
  - **实现面**：C 案独立 TrackEnvelopeLane2D（底部 96px 固定高度叠加条，关闭态 visible=false+IGNORE+hit 空=零行为变化；行高/波形 lane 布局零改）；呈现经 C1 AutomationClient.get_curve（t→x 复用 :1452-1482 同式+GuiTransportClock 三级回退+空/错/beats 全 fail-closed 形态，内核文案逐字透传）；参数选中面=repo rack_nodes 目录+rack_client.get_plugin_parameters 同源描述符（独立 request_id 不读 Vit_Graph_Rack 私有缓存，车头按钮经既有让路链原生可点零路由改）；hit-kind 加 envelope 族（envelope_lane/envelope_lane_point 近点 6px+索引）在 clip 族前抢占；仲裁器分派臂=只选中/聚焦（行选中+车道聚焦+清 clip 选择+消费，4px 手势不启用=C4 边界）。
  - **探针断言面**：独立探针 `probe_envelope_lane_ui.gd`（headless -s 零栈）**39/39 exit 0**（含段完成标记机制防静默跳段假 PASS——本卡探针工程自纠项）；既有探针零回退 **9/9**（K3 313/popup 111/realinput 15/shrink a·b·c/start_feed 244/fader 35/datachan 50，c 亦绿）；渲染面断言（车道在场/绘制态读回）入探针。
  - **真栈 run**：`envelope_lane_gui_20261025_2048`——ps1 栈生命周期 exit 0（main 前守卫版零 ps1 改动，-NoChatSmoke -NoStripSilenceSmoke 最小腿+探针场景手动驱动=ps1 datachan 腿同款形态）+`stack_probe_envelope_lane` **23/23 连续两轮 exit 0**（r4 21:50:29/r5 21:51:50）：装载插件（C1 同款发现面）→车道空曲线披露+内核对照一致→**C1 L2 同款三点写入（探针侧直调 client）→车道呈现读回逐点一致**→换参+bogus 活内核错误逐字→envelope 命中+仲裁只选中/聚焦+关闭态词表零变化。§2.2 登记 caaad069/释放 b15a1564；三纪律执行（21:27:08 即时复核全零<60s、无 already listening、登记先于启动）；VIT_PROJECT_XML 隔离生效+主检出零触碰 sha256 亲证。
  - **披露**：①内核 get_plugin_parameters 对**内置 volume 插件**回 error「only available for external plugins」（L1 直调逐字披露）——描述符面仅外部插件；车道 fail-closed 呈现（assumed scale 读回可见），曲线读回一致在此形态下达成；外部 VST 描述符真实管线未入本腿（超文件域，C4/后续可扩）。②拆栈时 4 个非本腿 Godot 进程在飞（21:28/21:35 两对，13284 持 UDP 4444 遥测口）——§2.2 不拆他人栈留档待核，对本腿断言零影响（命令走 4445）。③三轮探针缺陷全为探针侧（var _/await 缺失/注入顺序/断言顺序/等待窗 30s 档），逐条修正留证。
  - **端测边界声明**：独立探针=假传输零栈；真栈腿=生产 GUI 传输链全链真实往返+真 track_row.tscn+真 mapping/仲裁器分派；**未覆盖**=渲染面像素级目检与用户旅程（E2E-WEBUI-1/JOURNEY-1 建设面）、外部插件描述符管线、C4 写回链（L2 set_points 为探针侧直调装载非车道 UI 编辑写回）、OS 级事件注入（仲裁断言为 _begin_gesture 对象级直调）。
- 验收：（裁定文件 / 验收 commit）
