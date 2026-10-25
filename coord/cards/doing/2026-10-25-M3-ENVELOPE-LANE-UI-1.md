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
- 回执：（commit / 探针断言面 / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
