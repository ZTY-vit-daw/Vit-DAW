# Vit 路由宪法（摘要）

**单行摘要：** 横线（`rack_connect_pins` / `rack_remove_connection`，含 `RACK_INPUT` / `RACK_OUTPUT` 母线锚点）只表达 Tracktion `RackType` 执行 DAG；竖线（Clip 占位 → 插件输入 + IPC `rack_set_node_clip_scope` 写入 `vit_clip_scope`）只表达 MIDI/音频事件从哪条 Clip 进入该机架节点；二者不得混用同一 IPC。

## 横线（机架 DAG）

- **语义：** 插件 ↔ 插件、**Rack In → 插件**（`source_id` 为空、`RACK_INPUT`）、**插件 → Rack Out**（`dest_id` 为空、`RACK_OUTPUT`）的音频/MIDI 针脚连接；持久化在 rack 的 `connections` 里，`get_project_state` 的 `edges` 使用字符串 `RACK_INPUT` / `RACK_OUTPUT` 标记母线端。
- **针脚约定：** Tracktion rack pin `0` 是 MIDI，pin `1+` 是音频；Godot 单槽音频节点虽然显示为 GraphEdit port `0`，发送 IPC 时必须映射到 rack pin `1`。Rack In/Out 锚点使用 UI 槽 `0=Audio`、`1=MIDI`，并映射为 `{0: 1, 1: 0}`。
- **`rack_add_node`：** 前端发送 **`auto_connect: true`**（与 `CommandDispatcher::handleRackAddNode` 默认一致），由 Tracktion 在插入时完成与 rack 母线的默认挂钩及（满足前提时）串链；用户**之后**可拆掉任意横线（含 Rack In↔首插件）。不要用 `auto_connect: false` 一刀切，否则会破坏轨→机架默认接线并可能导致 Clip/干声异常。
- **自动连接验收：** `handleRackAddNode` 应记录插入后 `RackType::getConnections()` 摘要；首个/第二个 Z3 音频插件的验收以 `get_project_state.rack.edges` 中 pin `1` 的 `RACK_INPUT -> plugin` / `plugin -> RACK_OUTPUT` 连接为准，而不是只看 UI 是否有线。
- **前端：** Godot `Vit_Graph_Rack` 左右 **Rack In / Rack Out** 锚点为纯 UI 节点，内核 ID 分别为 `RACK_INPUT`、`RACK_OUTPUT`，断连参数须与连接时一致。
- **刷新：** `rack_connect_pins`、`rack_remove_connection`、`delete_plugin`、`rack_add_node`、`rack_set_node_clip_scope` 成功后须合并最新 `get_project_state` 再 `sync_edges_from_server`（及竖线重建）。

## 竖线（clip_scope）

- **语义：** `vit_clip_scope` 为 `track`（默认，空/normalise 后同 track）或 `clip:<clip_id>`；决定 VitClipRouteRegistry 如何把 Clip MIDI/音频路由进该机架节点。
- **IPC：** `rack_set_node_clip_scope` — 字段：`track_id`、`rack_item_id`、`plugin_item_id`、`clip_scope`（空字符串表示清除为 `track`）。
- **前端：** Clip 占位输出 → 插件匹配端口类型的连线 **不** 调用 `rack_connect_pins`；连接发送 `rack_set_node_clip_scope`，断开发送空 `clip_scope` 回到 `track`。音频 Clip 使用占位节点第二输出槽（若存在）。
- **`rack_add_node`：** **不得**根据「Clip 作用域」选中数量自动写入 `clip:<id>`；否则会出现「未手动画竖线却显示竖线」而与全轨生效的听感不一致。绑定单 Clip 仅限用户手动画竖线。

## MIDI / 音频 / 叠音色（默认语义）

- **MIDI：** `track` 范围表示该轨上所有 MIDI Clip 可按既有规则进入链；`clip:<id>` 限制为单 Clip。
- **音频：** 竖线绑定音频 Clip 时走对应端口；未绑竖线时沿用全局/Z3 默认与其它 ROUTING 文档一致（不接链不等于删除 Clip 音频源）。
- **Z3 前端灰显：** Godot 机架可对 **Z3 的 `audio_fx` / `bus` 插件**按「仅沿 **AUDIO** 横线边、从 **RACK_INPUT** 锚点 BFS 是否可达」做弱对比度提示；后端 `createRackState` 可暴露 `audio_reachable_from_rack_input` / `vit_orphan_bypass_candidate` 供前端与快照对齐。**Z1/Z2** 的 MIDI 域与 `vit_clip_scope`/全轨 MIDI **不**用该规则判定（避免与音频 DAG 混淆）。
- **空置：** 横线断开后「空置」指**该插件实例**在约定路径上旁通/无有效接入，**不是**整轨静音；当前后端先暴露 orphan-bypass 候选，不直接改写插件 enabled 状态。若快照已不可达但听感仍染色，再在播放图构建层做 explicit bypass/过滤，避免破坏用户手动启用状态。
- **Z2 多乐器：** 叠音色与分区（Z1/Z2/Z3）约束仍以 `VitGraphValidator` / template_role 为准；竖线只改「事件从哪来」，不改横线 DAG 合法性情检。
