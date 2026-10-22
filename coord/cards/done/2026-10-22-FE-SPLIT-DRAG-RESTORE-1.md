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

- 领取：（执行侧未回填——主管验收时补记：2026-10-22 晚窗 / 前端基线=main bf9c10a（含 164/167，正确依赖基线）/ worktree=D:/Godot/project/vit-daw-frontend-splitdrag1 @ port/fe-split-drag-restore-1 / 依据=worktree 存在+实现提交 912d4a8（19:14+0800））
- 回执：实现=port/fe-split-drag-restore-1@**912d4a8**（2 文件 +350/-35：vit_dock_root.gd +103（HSplit 替换+占比折算+防抖持久化）/probe_fe_gui_shrink_phase_a.gd +282 修订）。断言面=Phase A 契约探针修订：退役钉**全保持**（钉语义收紧「分割树管线不回潮」）+新增 168 能力钉，2 位断言改 HSplit 面（grabber 活性/dragged 驱动产品链防抖落盘写读一致/窗口改宽占比保持/同进程重实例重启往返）——Phase A 88 断言 exit 0+shrink b 120+c 343 零回退。**关键语义勘定**：Godot 4.6.1 split_offset=相对子项 stretch 等比分点的偏移量（非距左绝对像素；退役 builder 绝对公式当年即顶格故用户记忆只见可拖），折算带 -0.5 修正项（隔离探针实测）。端测边界=headless 探针断言到产品链（dragged 信号+防抖 Timer+user:// 落盘读写），拖拽手感与重启视觉=用户手测目检项（探针头声明）。
- 验收：**通过**（GLM 主管决策流 / 2026-10-22 晚窗）。核对：diff 亲读（占比 clamp 0.2-0.8 防拖没；坏 JSON/未知 schema fail-open 出厂 0.58；1/3/4 位态与 F2/视图帧 reparent/缓存链零改动；卡面红线「不得笼统删钉」遵守=退役钉全保持）；主管复跑 **Phase A/B/C 三探针 PASS+sendroute 合并态 255/255+popup 111/111 全 exit 0**（worktree splitdrag1；sendroute/popup 为执行侧未跑面主管补跑）。真栈腿裁定=**免**（零链路改动纯前端布局，headless 已断言产品链；窗口态视觉归手测）。验收 commit=前端 main **3988f04**（merge）。
