# FE-LANE-POINT-CLICK-DEAD-1（211）真栈 run 工件：pointclick_20261028_1

- owner=ZCode GLM-5.3 PC 执行会话（211 卡会话）；栈登记 8600e965（23:08）/释放 67415d67（23:38）
- kernel=主检出重建二进制只读复用（2026-10-28 21:26 主管重建 III-A 面，209 同源）；agent=D:/Vit_DAW_wt_211stack 构建（Go 零改动 exit 0）；vsphub=主仓 bin 只读复用（17308）
- VIT_PROJECT_XML=run 目录隔离副本 project_iso.xml（287956e7，hash-object 亲核）
- 前端被测 worktree=D:/Godot/project/vit-daw-frontend-laneclick1@port/fe-lane-point-click-dead-1（基线 84396f1+本卡修复）

## 断点定案（DIAG r2-r4 取证锚点）

1. **P0（r2-r4 全绿）**：timeline_dock_adapter.tscn 生产宿主装配（drop_router/TimelineInputArbitrator/mapping/store 实件）+行经 adapter 生产路径创建+strip 行内布局真实几何 rect=(315,332,2237×96)——**卡面断点候选①（命中裁剪/新几何）排除**。
2. **r4 全链路由指认（决定性）**：parse_input_event 注入左键 press 后 GUI 路由=`[TimelineDropOverlay←press1(filter=PASS); adapter←press1(filter=STOP)]`——**strip 零收**。TimelineDropOverlay=画布区 GUI 网关（timeline_dock_adapter `_ensure_drop_overlay` 全幅 PASS+`_has_point` 动态认领+`vit_timeline_overlay_forward_pointer_event` 显式转发链[ruler→TrackList.vit_forward_timeline_pointer_event→仲裁器]）。
3. **r3 P1z0**：`_resolve_row_at_global_point_fast(gp)` 命中行（row_rect=(0,76,2560×358) 含车道区）而 `mapping.resolve_hit_target(gp)=kind:none`——`row_at_global_point`→`_row_accepts_global_point` 的 **wrapper rect 命中判定（TimelineViewMapping.gd:278-283）对 wrapper 外的车道新区域（206 行内布局流）返回 false**=卡面断点候选③"wrapper 外新区域（207 披露ⓐ mapping 边界变体）"坐实。仲裁器因此 IGNORE（不拦）——但 overlay 网关仍截获 GUI 派发=死区完整根因。
4. **死区与"按钮可用"并存自洽**：strip 头部按钮/＋添加按钮=BaseButton，走 overlay 的 `_global_point_targets_interactive_control` 让路面（网关不认领→GUI 穿透按钮）——用户手测"音量/声相可选✓"与"画布单击死区"同源不同面。
5. **203/206/209 三轮探针绿而真实死区的原因**：探针=裸 host（无 overlay/dock 生产宿主）+`strip._gui_input` 直调——生产 GUI 派发管线（本探针首次覆盖）从未被测。

## 修复（两文件）

- `vit_dock/views/timeline_dock_adapter.gd`：`vit_timeline_overlay_should_accept_pointer_at_global` 加车道编辑面让路（`_global_point_targets_envelope_lane_edit_surface`：fast 行→`envelope_lane_hit_test_global` 非空→网关不认领→**GUI 派发穿透直达 strip._gui_input**，203 领土隔离设计在生产装配首次可达）。车道收起恒空=既有行为零变化。
- `app/tracks/track_scene/track_envelope_lane_2d.gd`：strip `_on_left_pressed` 过守卫后 `_notify_lane_edit_press()`（容器→`_track_row.timeline_select_this_track`——原仲裁器 envelope 分支行选中语义在编辑面内补齐；裸 host 装配安全跳过）。
- 仲裁器/mapping/drop_router **零改**（车道区维持非认领=勘察定案维持）。

## 验证矩阵

| 面 | 结果 | 工件 |
|---|---|---|
| 正式轮×2（旅程面管线注入） | **30/30 PASS×2 连续 exit 0** | formal_r5/r6_probe.log |
| DIAG 取证轮 r2-r4（基线死区签名） | 签名坐实（见上） | diag_r2/r3/r4_probe.log |
| 离线全家 27 件 | 27/27 exit 0（26 -s 件+file_reply 场景件 7/7） | offline_family.log |
| 真栈族既有 3 件（203 edit/206 grow/202 rebuild） | 30+16+23 断言全绿零回退 | （stdout，随会话记录） |

## 旅程面断言明细（正式轮 P2-P7）

P2 管线建首点（press→drag→motion→release→恰一笔→回显门→内核读回逐点一致）／P3 折线加点（内核 2 点）／P4 拖点改值（v=0.3 读回）／P5 曲率柄（c=-1.0 读回）／P6 hover 零副作用／P7 Ctrl+Z/Y 键链（undo 后=编辑前+redo 后=3 点，车道+内核对齐）。

## 环境边界成文（probe_pipeline_selftest v2 留档）

- **headless 下 GUI 派发不达**：`_input` 阶段收到而 `gui_input` 信号零收（v1 双模式亲证）——旅程面管线注入探针须**窗口模式**跑。
- **注入序列义务**：press 前须 motion 先行（建立 hover/mouse_over 态），否则 GUI 派发不路由（v2 PASS vs v1 FAIL）。

## 披露

1. 探针 v1 发现函数轨行键名漏 `track_id` 候选（203 原版对照修正）——探针面。
2. P7 选位三轮迭代：两点曲线右延伸区无命中面（194 设计边界，非缺陷）→段中点=曲率柄位（进曲率拖拽零位移=零写回）→**0.38 段位沿曲线采样值**命中折线——探针面，产品行为边界如实披露。
3. 让路后仲裁器 envelope 分支的"清 clip 选择+聚焦高亮"联动不在编辑面内复刻（行选中已补；清 clip/聚焦边框为低频边缘面，上交主管知悉）。
4. `[等待用户:手测闭环]`——用户窗口手测画线/拖点/曲率/undo 全套（206 挂起至今的验收面）。
