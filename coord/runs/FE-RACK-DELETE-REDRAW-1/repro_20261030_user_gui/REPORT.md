# 212 用户手测重验收口——失败复现取证（2026-10-30 晚）

- 取证会话：GLM 主管决策流（用户报「删除后卡片保留在 GUI 画布节点」，现场保留中——**只读取证，零栈操作**）
- 现场形态：用户从 Godot 编辑器拉起 start_page（合规手测链，pid 37784 / 18:10:43），自拉栈 VitApp 19836（18:10:44）+ VspHub 26096 + VitAgent 32108；内核日志 `VitHeadlessServer2026-10-30_18-10-45.log`；前端日志 `godot.log`（user://logs/，覆盖写当前会话）
- 用户操作流（内核日志重建）：18:11:00 拖入 Pro-Q 3（rack_add_node → revision 1 node_add, rack_item 1012 / node 1013 / track 1007）→ 18:11:12 删除（revision 2 node_remove）→ 画布节点未消失 → 用户重删（Plugin not found）→ 其后用户新建空白工程（graph_revision 归零 "Blank project created"）→ 尾部 [rack.drop] 再拖 Pro-Q 3

## 证据链（六段）

1. **内核删除链全对**（212 探针结论维持）：delete_plugin 到达 ZmqGateway → 执行 ok（node_remove diff + graph_revision 2）→ 工程落盘 `default_project.xml`（核对 `plugin_item_id="1013"` 0 次出现）。第二次删除返回 "Plugin not found"=内核状态确已删。
2. **dock 回执回拉启动**：delete reply ok → `_on_vit_ipc_command_completed_for_dock_repo`（vit_dock_root.gd:1204，request_id="delete_plugin" 在册列表）→ `vit_refresh_project_from_kernel` → 内核连续收到 VSP 通道请求：`state.snapshot_request`（req_29250225_33, client_id=godot.gui.project_repository）+ `state.delta_request`（req_29299549_50, base_revision=4）。
3. **两级守卫都命中（设计内回落）**：
   - 207 快照守卫：VSP 紧凑快照剥轨行 plugins 列（normaliseTrackArray 白名单）→ `_vsp_snapshot_tracks_have_plugins` false → `_refresh_project_repository_from_vsp` 返回 false → 落 store（vit_dock_root.gd:1129）✓
   - 208 delta 守卫：前端日志唯一运行时告警 `FE-LANE-CATALOG-VSPDELTA-GUARD-1: VSP delta applied but project_state tracks lack array-typed plugins rows; falling back to legacy get_project_state full feed`（backtrace: project_store.gd:185 ← :65 ← vsp_client.gd:101）→ `_apply_vsp_delta_reply` 判非 ok ✓
4. **断点=第三跳 legacy 回落未发出**：`refresh_project_state`（project_store.gd:32）guard error 后应 `_refresh_project_state_from_legacy` → `project_client.get_project_state` → `KernelCommandClient.send` → `VitIpcClient.send_command_async_await`（UDP 队列，request_id 自动生成 ipc_N）。**两次删除之间内核收到的全部命令清单（ZmqGateway 全量核对）：零次裸形态 get_project_state**——legacy 请求从未到达内核（ipc_N/无 id 两种形态均零；对照 ipc_33 get_audio_device_status 有记录，通道记录面无盲区）。
5. **结构性 root cause 候选（UDP 串行队列）**：`vit_ipc_client.gd:126 _process_queue`——`_active_request` 非空时队列完全不调度（**严格单请求在飞**）；软超时（PROJECT_TIMEOUT_SEC=8s）对排队中未发出请求同样生效丢弃。删除时刻 envelope lane 周期轮询在飞：`env_lane_params_1008` / **`env_lane_params_1013`（已删插件仍在被轮询，track_envelope_lane_2d.gd）**，param 族超时 30s（PARAM_GRABBER_TIMEOUT_MS）。**时序竞争：回拉 legacy get_project_state（8s 软超时）入队被在飞/排队的 param 轮询堵塞 → 未发出即丢弃 → await 返回 error → dock `_project_reply_ok` false 静默 return → `_apply_kernel_project_reply_and_emit` 不执行 → repo/project_loaded/画布重绘全链停摆**。内核侧 param 对已删插件返回 error 小包（"Plugin is not on the specified track"，不挂起）——堵塞面在前端 UDP 队列调度，非内核。
6. **辅助证据**：`_on_delete_nodes_request` 注释（Vit_Graph_Rack.gd:7632「Phase 3.1：成功后由主控 get_project_state 重绘机架，禁止本地 queue_free」）——画布节点消失 100% 依赖回拉全量，无本地回退路径，与症状一致。前一会话日志 `godot2026-10-30T18.10.43.log`（更早一轮，已停栈 31852/24104/21640）尾部有**同一条 guard 告警**——跨会话稳定复现，非偶发。

## 212 探针为何未覆盖（差异归因）

- 212 r8 断言链止于「VSP delta → repo 双面清除 → project_loaded → 重绘」（delta 面成功形态）；**用户时序=「add 后参数面板/车道轮询激活 → 删除」**，回拉落进 207+208 双守卫 + legacy 一跳——探针无此竞争窗口（headless 无 envelope lane 轮询、无 param 面板在飞）。
- 208 真栈探针 P4c/P6 验证过「守卫回落补齐」（store 层 vp 恢复），但 208 探针是**车道参数目录面**，未断言 rack 画布节点消失；且回落在探针形态下 UDP 队列无竞争。

## 处置

- 212 收口动作（用户手测重验）**失败**：探针向量（KEY_DELETE/agent 命令）与用户惯用入口同链（`_on_delete_nodes_request`），失败非向量差异，是回拉链第三跳断裂——212 卡面记录，收口挂起。
- 折精准修复卡 **2026-10-30-FE-RACK-DELETE-GUI-REFRESH-1（池序 221）**：插桩定案 UDP 堵塞假说 + 修复。
- 现场保全：未动任何进程；用户后续可自由关闭或继续（工程已新建空白+新 Pro-Q 3，1013 事故现场已翻篇，取证依赖的日志文件均已落盘）。

## 原始工件指针

- 内核日志：`D:/Vit_DAW/VitApp/Workspace/Logs/VitHeadlessServer2026-10-30_18-10-45.log`（第一次删除 L247842 / 第二次 L251918 / 空白工程 L31040 区）
- 前端日志（当前会话）：`%APPDATA%/Godot/app_userdata/Vit_DAW_Frontend/logs/godot.log`（guard 告警+ [rack.drop] 尾行）
- 前端日志（前一会话）：同目录 `godot2026-10-30T18.10.43.log`（同告警）
- hub 日志：`D:/Vit_DAW/VitApp/Workspace/Logs/vsp_hub_last.log`（websocket 会话 8787 全程仅 realtime_adapter 一路，无异常）
