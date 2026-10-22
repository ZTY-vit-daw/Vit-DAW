# FE-SPLIT-DRAG-RESTORE-1：主区双窗分栏拖拽恢复（165 裁定 A=恢复选项继卡）

- 发卡：GLM 主管决策流 / 2026-10-22 晚窗（依据=165 上交裁定：用户确认主管推荐 A=恢复拖拽——用户原话诉求「就和之前一样」；授权链=用户 2026-10-22 晚窗「继续任务，可以发卡」确认推荐方向）
- 派发确认：已确认
- 验收负责人：GLM 主管决策流
- 池序 168；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P2 / 0.25-0.5 天 / 依赖=无（文件域与在飞 164 rack 域不交）；模型分级：L1 / flash 可接

## 背景（165 勘察结论，主管抽查证实）

双窗不可拖=FE-GUI-SHRINK Phase A（c56e38f）有意重设计：`vit_dock_root.gd` `_build_stage_grid_for` 用 HBoxContainer+stretch_ratio 0.58/0.42 钉死，现场无分隔条；退役前=default_arrange 预设经 viewport_split_tree_builder 建成**原生可拖 HSplitContainer**（用户记忆锚点）。用户裁定=恢复拖拽能力（选项 A，非 F2 占比档位）。

## 范围

①`_build_stage_grid_for` 2 位态 HBoxContainer→HSplitContainer（初值仍 0.58/0.42 折算 split_offset，grabber 可见可拖）；②拖拽后占比持久化（JSON+防抖，参照 legacy `vit_control_v_1.0.gd:210-237` transport split 模式）落盘重启保持；③F2 三/四位模式与视图帧 reparent/缓存复用链不动（141 mixer 重建链不相交）；④Phase A 退役契约探针 `tools/probe_fe_gui_shrink_phase_a.gd` 同步修订：钉语义收紧为「分割树管线（preset repository/builder 驱动 UI）不回潮」，恢复的 HSplit 拖拽=用户裁定新增能力、非回潮——**不得笼统删除退役钉**。

## 文件域

`vit_dock/scenes/vit_dock_root.gd`（grid 构建+比率+持久化三处邻域）+`tools/probe_fe_gui_shrink_phase_a.gd`（契约修订）。越域即停。

## 验收标准

组件/探针断言：2 位态树中出现 HSplitContainer+split_offset 变更生效+持久化往返（写→读回一致）；修订后 Phase A 探针 exit 0+既有回归（shrink a/b/c 及布局相关）零回退；拖拽手感与重启保持的窗口态视觉=用户手测目检项（AGENTS §5 途径）；端测边界声明。

## 停止条件

拆 HBox 牵动 F2 位态切换/视图帧缓存链超出预估文件域 → 锚点上交（不得为塞进域内砍功能）。

- 领取：2026-10-22 18:48 / 主仓 origin/main=9e3601bc（领取基线=前端仓本地 main 尖 bf9c10a——主管已验收合入 164/167，卡面基线 5f2a806 为其祖先；164 rack 域与本卡文件域核实不交）/ owner=GLM-5.3-flash（ZCode 执行会话，PC）/ 分支=port/fe-split-drag-restore-1（前端仓）/ worktree=D:\Godot\project\vit-daw-frontend-splitdrag1 / 领取提交=本提交（含卡池同步：165 blocked 裁定终态+168 发卡自主管本地 main f9f96ddf 摘取入共享 main——未推送区间 20 提交含实现合入，执行侧按 PROTOCOL §3 只同步 coord 面；stale todo/165 副本=bee74f6f 事故遗留，一并清除）
- 回执：（commit / 断言面 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
