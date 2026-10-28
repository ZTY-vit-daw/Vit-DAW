# FE-RACK-DELETE-REDRAW-1：删除效果器后机架节点不消失——delete 回执重绘链断（手测节第三轮缺陷卡 V）

- 发卡：GLM 主管决策流 / 2026-10-28 晚（依据=用户手测：删除效果器后 GUI 依然挂着卡片，参数列表已移除=数据面删除成功）
- 派发确认：待用户确认（手测节当场折卡惯例）
- 验收负责人：GLM 主管决策流
- 池序 212；目标仓库=**前端仓 D:\Godot/project/vit-daw-frontend**；P1 / 0.5 天 / 依赖=无（基线=前端 main 84396f1）；**与 211 文件域不相交可并行**
- 模型分级：L2 / PC 执行会话

## 已核事实（发卡侧）

1. **删除链注释亲证**（Vit_Graph_Rack.gd:7645-7647 `_ipc_delete_plugin_for_node`）：「由内核 removeFromParent；**成功后由主控 get_project_state 重绘机架，禁止本地 queue_free**」——机架节点消失完全依赖删除后的重绘触发。
2. **回拉锚点不对称候选**（207 勘察在卷）：dock root 拓扑回执回拉链锚点挂在 `rack_add_node`（vit_dock_root.gd:1196 区域）——**delete_plugin 回执是否挂同样回拉待勘察**；用户侧 repo 已更新（参数列表移除=lane hub 重读生效）而机架未重绘——重绘触发/机架重绘消费信号两层候选断点。
3. 数据面删除成功（内核 removeFromParent 生效）——本卡纯 GUI 刷新链。

## 目标

1. 勘察定断点：delete_plugin 回执→（dock 回拉/机架重绘信号）→机架节点移除，断在哪层；
2. 修复=删除后机架节点消失（与 add 对称的重绘触发或回执本地处置——按 Phase 3.1 既定设计优先补触发，不破坏「禁止本地 queue_free」契约，如须破坏需说明理由）；
3. 手测闭环：用户删除效果器卡片消失+参数列表同步（双向一致性）。

## 探针义务（§5.1+时序面+旅程面口径延续）

- 事件管线注入（211 新口径）适用面：删除动作若经右键/快捷键 GUI 触发，探针须走真实事件或生产调用链（勿直调内部函数跳过回执等待）；
- 断言=删除后机架节点集合与 repo rack_nodes 一致（双面）+重绘触发在卷；全家回归零回退。

## 文件域

`app/rack/Vit_Graph_Rack.gd`（重绘消费面）+`vit_dock/scenes/vit_dock_root.gd`（delete 回执回拉挂载面——若须改）+探针。**域外需改→上交扩域裁定**。

## 停止条件

- Phase 3.1「主控重绘」设计的既有触发方（主控层）已存在但被其他条件短路→上交锚点；
- 修复须动 agent/内核→上交。

## 领取

- 时间：2026-10-28 22:38；origin/main hash：186070e7
- owner：ZCode GLM-5.3 PC 执行会话（212 卡会话）
- 分支：port/fe-rack-delete-redraw-1（前端仓 D:/Godot/project/vit-daw-frontend，基线=main 84396f1）
- worktree：D:/Godot/project/vit-daw-frontend-wt-fe-rack-delete-redraw-1
- 领取提交：本提交（doing 移动+回填，coord/ only）

## 回执

（待回填）

## 验收

（待主管裁定）
