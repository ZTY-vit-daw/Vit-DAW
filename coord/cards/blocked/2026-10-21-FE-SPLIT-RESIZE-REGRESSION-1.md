# FE-SPLIT-RESIZE-REGRESSION-1：双窗分栏占比不能拖拽调节（回归）

- 发卡：GLM 主管决策流 / 2026-10-21 深夜（依据=M2 场次③用户手测反馈：「双窗两边不能拉伸，需要可以拉伸调节两窗的占比，就和之前一样」）
- 派发确认：已确认（用户手测现场反馈）
- 验收负责人：GLM 主管决策流
- 池序 165；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P2 / 0.25-0.5 天 / 依赖=无；模型分级：L1 / PC 前端执行流

## 现象

主布局双窗分栏（用户表述「双窗两边」）拖拽分隔条不生效，无法调节两窗占比；用户记忆中此前可用（「就和之前一样」）。具体指哪一组分栏以复现时用户描述为准（嫌疑面：`app/legacy/vit_control_v_1.0.gd` 的 main_layout/right_main_core/app_vertical_root split_offset 家族或 `vit_dock/core/window_manager.gd` 窗口布局面）。

## 范围

①复现+二分定位：回归引入点在哪个合入面（嫌疑窗=K3/K3.1/K3.2 合入 0e244f0 与 161 c5cac57 之间，也含更早；git checkout 二分或代码勘察）；②恢复拖拽调节；③split_offset 持久化（:210-237 既有 save/load 面）不破坏；④如涉及 vit_dock 窗口布局，与 141 的重建链兼容。

## 文件域

以勘察结论固定（预期 `app/legacy/vit_control_v_1.0.gd` / `vit_dock/core/window_manager.gd` / 主场景 tscn 邻域）；开工时把精确文件域写入卡面。越域即停。

## 验收标准

真栈/窗口态腿：拖拽分隔条两窗占比实时变化+重启后保持（如既有持久化面）；探针或用户手测复核（窗口态视觉=用户目检项，按 AGENTS §5 手测途径）；既有布局测试零回退。端测边界声明。

## 停止条件

分栏结构已被有意重设计（非回归而是设计变更）→ 上交用户裁定（恢复 vs 新设计）。

- 领取：（2026-10-22 晚窗 / Vit_DAW origin/main=1ae57f23 / GLM-5.3-Flash（ZCode PC 执行会话，用户口令指定领取本卡）/ 分支=port/fe-split-resize-regression-1（前端仓独立 worktree D:/Godot/project/vit-daw-frontend-fe-split-resize，基线=前端 main c5cac57 即含 161 合入面）/ 领取提交=本提交；领取核对：卡在 todo 且派发已确认、依赖无、doing 唯一在飞卡 GEN-SUNO-WEB-DESIGN-1 为 Mac 侧文件域零冲突、前端仓 main=c5cac57）
- 回执：（**未实现——停止条件命中，证据上交待裁定**。定位结论=非回归，属 FE-GUI-SHRINK-IMPL-1 Phase A 有意重设计；验收项（拖拽实时生效/重启保持）不适用，留待裁定后继卡承接。**端测边界声明**：本轮零真栈运行、零代码改动，全部结论来自静态代码勘验+git 史二分+退役契约探针原文核对；worktree port/fe-split-resize-regression-1（基线 c5cac57）零 commit 零 diff，回执推送后即删）
- 验收：（裁定文件 / 验收 commit）

## 勘察结论（2026-10-22 执行流 GLM-5.3-Flash，停止条件命中——上交用户裁定）

**定性：非回归。双窗分栏不可拖拽=FE-GUI-SHRINK-IMPL-1 Phase A（c56e38f，2026-10-11）有意重设计的结果，非 K3→161 嫌疑窗内引入。**

证据链：

1. **嫌疑窗布局面零变更**：`git diff --stat 0e244f0 c5cac57` 全仓仅 6 文件（app/rack/Vit_Graph_Rack.gd、rack_send_route_rules.gd 新增、app/kernel/clients/track_client.gd、vit_dock/views/mixer_dock_adapter.gd、两探针），无一触及分栏/布局/输入面；`vit_control_v_1.0.gd`、主 tscn、window_manager.gd 在窗内零提交。
2. **「双窗」实指**：默认主工作区=vit_dock/scenes/VitDockRoot.tscn（start_page.gd:5 `MAIN_WORKSPACE_SCENE`；legacy「Vit control V1.0.tscn」仅 `VIT_USE_LEGACY_MAIN=1` 时进入）。主区双台前位（左=音轨 timeline、右=机架 graph_rack，vp_timeline_main/vp_rack_main）由 vit_dock_root.gd `_rebuild_stage_slots`/`_build_stage_grid_for` 装配：**HBoxContainer（separation 6px）+ size_flags_stretch_ratio 0.58/0.42 钉死**——现场不存在可拖拽分隔条；用户拖的「分隔条」是容器间隙，无输入面，拖动自然无效。
3. **用户记忆锚点（「就和之前一样」）真实存在**：Phase A 之前的布局=default_arrange 预设（workspace_preset_repository.gd，split_ratio 0.58）经 viewport_split_tree_builder.gd 建成**原生可拖拽的 HSplitContainer**（SplitContainer 自带 grabber）。用户记忆的拖拽能力=该 UI，2026-10-11 随 Phase A 退役。
4. **有意重设计实证**：c56e38f 提交明言「预设/分割树管线整体退役（workspace_layout_manager/workspace_preset_repository 文件保留为数据层不驱动 UI，viewport_split_tree_builder 删除）」；vit_dock_root.gd:211「主区=固定双台前位……不再走预设/分割树铺排」；Phase B（1c61482）延续固定钉比（「58/42 钉在帧上，Phase A 钉」）；tools/probe_fe_gui_shrink_phase_a.gd 把退役钉成**静态契约**（69 钉：split tree builder preload retired / split_requested retired / ViewportTitleBar retired 等，防回潮）。
5. **卡目标③自然满足**：split_offset 持久化面（vit_control_v_1.0.gd :210-237 JSON 保存/防抖）属 legacy 场景链，与用户所见双窗无关，现状未破坏、无需改动。

**上交点（用户裁定：恢复 vs 新设计）**：

- **恢复选项（实现代价小，贴合「就和之前一样」）**：把 `_build_stage_grid_for` 2 位态的 HBoxContainer 换成 HSplitContainer（初值 0.58/0.42 沿用），拖拽后经新 persistence 小面落盘重启保持（可参照 legacy transport split 的 JSON+防抖模式）；3/4 位态同法可选；视图帧 reparent/缓存复用链与 F2 四位模式切换不动，141 mixer 重建链不相交。预估文件域：`vit_dock/scenes/vit_dock_root.gd`（grid 构建+比率+持久化三处），并复查 probe_fe_gui_shrink_phase_a.gd 静态契约是否需随裁定修订。
- **新设计选项**：F2 调度面板加占比档位（如 50/50、58/42、67/33）等受控切换——改动面更小但非用户原话诉求。

裁定后建议开继卡承接实现（本卡转 blocked 供追溯）；若裁定恢复，继卡文件域以上行预估为基线。
