# FE-GUI-SHRINK-RECON-1：前端 GUI 收缩盘点（只读，GUI 翻新讨论线第一输入）

- 发卡：GLM 主管决策侧 / 2026-10-10
- 派发确认：已确认（用户 2026-10-10 裁定 GUI 走收缩归拢路线+讨论对话流工作流：讨论→案例样图（HTML/PNG）→统一→动工→测试；盘点为第一轮讨论输入）
- 验收负责人：GLM 主管决策流
- 池序 47；目标仓库=**D:\Godot\project\vit-daw-frontend**（仓库外，只读）；工件落 D:\Vit_DAW\coord\runs\FE-GUI-SHRINK-RECON-1\
- 优先级 / 预估 / 依赖：P2 / 1-2h / 无；**只读勘察，零代码**
- 模型分级：L0 / **任意引擎（含闲时）**

## 目标（报告五节）

1. **GUI 面清单**：app/ 各模块（browser/clips/mixer/piano_roll/rack/resource_hub/settings/shell/startup/state/timeline/tracks/transport/ui/visual/legacy）逐模块盘点——面板/窗口/弹层清单（场景或脚本锚点+一句话功能+可见入口）。
2. **交互入口清单**：每面主要操作入口（菜单/右键/拖拽源与落点/快捷键）——重点标注跨面长路径交互（如资料库→机架的插件拖拽横跨）。
3. **遥测写者红线面**：证据链写者清点（mix_client.gd/telemetry_manager.gd 等产 mixboard_/kernel_prepared_ refs 的面，参照 coord/runs/REFSCHEMA-M8/report.md 附录口径）——标注"收缩不可动"。
4. **legacy/ 清点**：遗留面清单（vit_control_v_1.0.gd 等）——退役候选肥肉，含被引用情况（谁还在调）。
5. **收缩候选分级草案**：每面三级（归拢/合并/退役）草案+证据锚点——**草案仅供主管与用户讨论定夺，非结论**；使用频率证据限于已插装面（遥测/事件流），未插装面如实标"无使用证据"。

## 文件域

零代码改动；报告+可复跑盘点命令附录（M8 报告附录风格）。

## 验收标准

五节齐+每条有锚点；分级草案明确标注"讨论输入非裁定"；红线面清单与 M8 报告交叉核对零遗漏。

## 停止条件

无（纯只读）；发现 GUI 结构性异常（如 legacy 与新面双轨并存的隐藏耦合）如实记录不展开。

## 并行与资源

只读零占用；与全部在池卡并行安全。

- 领取：（时间 / 前端仓 HEAD / owner）
- 回执：（报告链接）
- 验收：（裁定文件）
