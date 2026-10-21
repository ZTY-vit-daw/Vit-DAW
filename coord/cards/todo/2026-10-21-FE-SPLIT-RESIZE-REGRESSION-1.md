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

- 领取：（时间 / origin hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 定位结论 / 验收工件 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
