# K3.1 ① 空白右键菜单坐标取证（2026-10-19）

## 问题
用户手测（2026-10-18 深夜）：机架空白处右键菜单稳定出现在屏幕左侧固定位置非鼠标处；
send 选择器（同一坐标传导）弹在画布左缘被裁剪。

## 取证方法
`forensic_popup_coords_k31.gd`（headless + 窗口态两轮）：构造与生产同构的
偏移容器（300,100）> Vit_Graph_Rack（800x600），分别以局部/viewport 坐标
触发空白右键链路，读 PopupMenu 弹出后的窗口屏幕位置。

## 结论（窗口态 windowed_run.log，Godot 4.6.1）
1. **popup_on_parent 实测吃主窗口 viewport 坐标**：菜单屏幕位置 = 主窗口原点 +
   rect.position 原样（parent=GraphEdit 的全局换算未发生）：
   - 传局部 (400,300) → 菜单屏幕 = 窗口+(400,300)（`local-arg` 行，修前）
     = 系统性偏左 (300,100) = GraphEdit 全局偏移量 —— 与用户症状精确吻合。
   - 传 viewport (700,400) → 菜单屏幕 = 窗口+(700,400) = 鼠标真实位置。
2. **引擎 GraphEdit 摆放公式 = 仓库 `_graph_point_to_canvas`**（zoom=0.8、
   scroll=(50,30) 下 engine node.position 与公式输出逐位相等）；
   `_node_context_menu_viewport_position` 输出恒等于鼠标 GraphEdit 局部坐标。
   → 节点右键菜单（:6478）走 `_show_embedded_context_menu`（内嵌 Panel，
   吃局部坐标）所以一直正确；K3 空白右键换成 PopupMenu.popup_on_parent
   （吃 viewport 坐标）但第三参仍传局部坐标 → 错位。
3. **修法**（已实施）：Vit_Graph_Rack.gd :1037 第三参 `get_global_transform()
   * mb.position`（local→viewport 换算，对齐卡面钦定修法）；
   `_open_canvas_action_menu` 双轨——popup 吃 viewport 坐标，下游三入口
   （装载插件弹窗/发送选择器/新建宏）锚点存回局部坐标
   （`affine_inverse` 换算）。窗口态复跑（windowed_run.log 后两行，
   修后代码）：菜单屏幕位置 = 窗口+(700,400) = 鼠标位置 ±0px；
   `_canvas_action_menu_pos` 存回局部 (400,300)/(100,200) 正确。

## 登记
- `_open_node_context_menu` 的 PopupMenu fallback 分支（:6430，embedded
  panel 优先分支之外的旧路径）与 popup_on_parent 坐标口径同样不匹配——
  现状因 embedded 分支常驻不触发，K3.1 未动（域外，登记备查）。
