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
- worktree：D:/Godot/project/vit-daw-frontend-wt-rackdel1（实际路径，与登记时卡面初填名差异=简写，此前提下零歧义）
- 领取提交：53def3cc（doing 移动+回填，coord/ only；与 211 并行领取竞争一次干净解决——其 doing 副本删除滞后入本提交，终态正确双方卡面完整）

## 回执

- **结论=零修复卡（停止条件①命中，209 先例同形）**：卡面前提「删除回执重绘链断」在现行基线（前端 84396f1+内核 21:26 重建）被双向量证据推翻——
  - **手势删除全链工作（DIAG r8）**：`Input.parse_input_event` KEY_DELETE 真实事件管线（211 新口径兑现，禁 handler 直调）→ dock `_unhandled_input`→shortcut 链→adapter→rack→delete_plugin IPC→内核 reply ok→**dock 回执处理器 delete_plugin 在册对称回拉（vit_dock_root.gd:1217-1224——卡面已核事实 2「add/delete 回拉不对称候选」不成立）**→store VSP delta（transport=state.delta 亲证）→repo rack_nodes+plugins_by_track 双面清除→project_loaded→adapter 重绘→画布节点消失+双面一致；
  - **外部删除（agent 等价，VSP hub command 不经 VitIpcClient）亦工作（DIAG r9）**：内核 `delta_update(action=node_removed)`→hub websocket→telemetry_manager 防抖回拉（telemetry_manager.gd:672-697）——现行基线删除有三层触发（GUI 回执/遥测 delta/173 stale 沿）；
  - 真栈正式轮 **29 PASS×2 连续 exit 0**（真实 Supermassive VST：双向量删除双面一致+add 活体对照）+离线全家 **26/26**；零生产改动（前端 port 分支仅探针件 7 commits@3f4ab36）。
- **探针**：`tools/stack_probe_rack_delete_redraw.gd/.tscn`（旅程面事件管线注入+§5.1 VitDockRoot 实件/`_stage_push_view` 生产上台链零自建+多 adapter 真身甄别[树内空视口实例亲证入探针]+生产 context 绑轨[自动绑首轨行为亲证入探针]+四层遥测+双面一致主断言+外部删除腿）。
- **上交三件**：①**[等待用户:手测重验]** 用户手测节在新栈重验删除效果器（手势+agent 两向量）=卡面手测闭环即收口动作；②**域外缺口 A 扩域裁定**：`websocket_recovered` 沿无消费者（vsp_realtime_adapter.gd:718 发射/telemetry_manager.gd:665 只吃 stale 沿）=断供窗 delta 丢失无恢复回拉，修复在 app/kernel/autoloads/ 域——建议折 P2 小卡（一处沿消费+防抖回拉，173 同形态）；③伴生披露：adapter 无选轨自动绑首轨（graph_rack_dock_adapter.gd:830-844，生产由轨焦点跟随覆盖）——用户手测若再现症状优先核对机架绑轨一致性。
- **run**：`coord/runs/FE-RACK-DELETE-REDRAW-1/rackdel_20261028_1`（工件随 `port/fe-rack-delete-redraw-1-stack@cbf96ed2` 入库；RESULTS.md 六节含 DIAG r1-r9 全程与探针迭代三案）；真栈三纪律+错峰（长轮询 211 释放后登记）+VIT_PROJECT_XML 隔离（287956e7→da3d5655）+拆栈亲核全零——UDP 4444 现由用户编辑器形态 Godot（pid 13612，非本腿启动）持有，§2.2 不擅动留痕披露。
- **端测覆盖边界声明（AGENTS §5）**：真栈探针=headless 生产装配（VitDockRoot 实件+生产上台/绑轨/事件注入链），非全 GUI 目检；渲染像素面与用户全 GUI 并发态未覆盖——残余面已上交（缺口 A+用户重验）。

## 验收

（待主管裁定）
