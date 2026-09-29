# MACRO-RECON-1 勘察报告：现状宏数据流 + 动态注册可行性 + 传输通道选项 + 控件类型需求面 + 参数级可观测性与写冲突面

- 卡片：`coord/cards/done/2026-09-29-MACRO-RECON-1.md`（领取记录：origin/main=cd4738cb / port/macro-recon-1）
- 勘察时 HEAD：**894c1a79**（port/macro-recon-1，=领取 commit；工作树零代码改动，仅运行时状态 `VitApp/Workspace/default_project.xml` 为领取前已有改动，未触碰）
- Godot 前端勘察基线：`D:\Godot\project\vit-daw-frontend` 工作树现状（仓库外，无版本锚；行号以当日文件为准）
- 性质：只读勘察，零代码改动。三态结论口径：**支持**（机制已在、可直接承载设计）/ **不支持**（机制缺失，设计需绕行或新建）/ **需新开面**（有机制但需新工作面或真栈验证才能承载设计）。

---

## §1 现状宏数据流全景

### 1.1 宏的定义与存储形态：**两套并存且互不感知**

**体系 A：前端 rack 宏（当前实际在用的体系）**
- 存储：project media 内 JSON 文件 `rack_control_macros.json`，schema v1。`vit_dock/core/project_repository.gd:11-13`（`RACK_CONTROL_MACROS_KEY`/`FILENAME`/`SCHEMA_VERSION`），工程打开时加载（`:28`、`:199`）。
- 文档结构：`{macros: [...]}`，每条宏含 macro_id/track_id/plugin_id/name/control_type（默认 slider）/value(0-1)/bindings/position。CRUD 全在 repository：`create_rack_control_macro`（`:963`）、`get_rack_control_macros`（`:905`）、`for_plugin`（`:910`）、`for_track`（`:925`）、stats（`:939/:944`）。
- binding 结构（agent 侧归一化定义）：binding_id/track_id/plugin_id/param_id/param_name/source_min/source_max/target_min/target_max/enabled，`agent/internal/macrocontrols/types.go:179-201`。

**体系 B：内核控制图宏（能力完整、当前零调用者）**
- 内核 Edit ValueTree 内的控制图：`VitApp/Source/Core/VitParamLinkGraph.h:11-66`（addControlNode/addMacroNode/addBinding/setNodeOutputValue/updateBinding/updateNodeLayoutAndRange/remove*）。
- 宏节点（macropanel，含 macro_count 个旋钮，默认 8）：`VitApp/Source/Core/VitMacroNode.h:8-17`；注册命令链 `CommandDispatcher.cpp:2804-2848` → `PluginRackControlService.cpp`（handleControlAddMacro `:2059-2107`、handleControlAddBinding `:2109+`、handleControlSetMacroValues `:2478-2545`，后者在内核侧应用绑定 `applyControlBindingsFromOutput` 并回 `applied_bindings`+graph revision）。
- **agent 与 Godot 均不调用这套命令**：agent 侧 `control_add_macro` 等 handler 是纯 ui_action 合成（见 1.4），Godot 侧无 `control_add_macro` 字符串（全前端 grep 无命中）。仓库内除 agent 自身实现/测试外无调用者。

三态：**支持**（A 体系数据流完整可描述）；**需新开面**（B 体系与 A 体系的关系需设计裁定——见"冲突事实"第 1 条）。

### 1.2 rack 宏 → agent 上行链

- Godot `vit_dock_root.gd`：每 0.75s tick 做变化检测（`:193-197`），payload JSON 变化即 POST `http://127.0.0.1:7878/agent/ui/context`（`:28`），5s 心跳强制重发（`:33`、`:597-611`）；payload 含 repository 全量 `rack_control_macros` 文档（`_agent_ui_context_payload`，`:637+`）。
- agent 接收：`chat/server.go:1188-1206`（handleUIContext，POST→sanitize→`s.uiContext`），白名单键见 `sanitizeUIContext` `:2150-2188`（含 `rack_control_macros`/`macro_controls`/`active_macro_controls`/`macro_control_summary`）。
- 归一化链：`uiMacroControls` `server.go:1995-2022` 从 5 个候选 key（`macro_controls`/`active_macro_controls`/`rack_control_macros`/`control_macros`/`macros`）取行 → `macrocontrols.NormalizeList/NormalizeControl`（`types.go:37-120`，source 缺省 `godot_rack` `:74`，control_type 缺省 slider `:82`）→ `enrichUIMacroControlFromState`（`server.go:2024-2060`，track.volume 类宏同步轨音量+补默认 binding）→ 按 macro_id 合并（`:2091+`）。
- 下发：GET `/agent/ui/state`（`server.go:1074-1117`）响应含 `macro_controls`（`:1102`）与 `capabilities.macro_control_ui`（`:1114`，声明 `source:"godot_rack"`、`supports_reference/live_cards:true`）。

三态：**支持**。

### 1.3 webui 宏面板与宏卡片

- 面板卡：`MacroControlPanelCard` `webui/src/App.tsx:9800-9893`（改名输入框 9849-9868、绑定摘要、引用/插卡按钮 9883-9888）；滑条 `MacroSlider` `:9937+`（HTML range，onChange→preview，释放→commit，含同值去重 `lastCommitRef`）。
- 对话流实时宏卡：`MacroActionCard` `:9895-9930`（"实时控件"=uiState 活宏 / "快照"=结果内嵌宏，`:9911-9920`）；引用到输入区 `@macro:` 前缀（`:993-999`）、插入 live card（`:1002-1009`）。
- webui 归一化镜像 `normalizeMacroControl` `:10210`（control_type 缺省 slider）。

三态：**支持**。

### 1.4 rack 宏 → 参数写路径（通道/数据形态/速率特征）

**写路径 ①（Godot rack 宏滑条，本源路径）**：`Vit_Graph_Rack.gd`
- 渲染：`_render_rack_macro_control_nodes`（`:3884-3899`）→ `_add_rack_macro_control_node`（`:3952+`）→ `_create_rack_macro_control_content`（`:3979-4031`，HSlider 0-1 step 0.001）。
- 拖动中每 tick：`slider.value_changed → _apply_rack_macro_control_value`（`:4018-4022`，`:4147-4171`）——对每个 enabled binding 算目标值（`_macro_binding_target_value` `:4174-4184`，source[0,1]→target 线性插值）→ 同步参数面板 UI（`_sync_param_surface_value_from_macro`）→ `rack_client.set_plugin_param(payload, request_id)`（`:4166-4168`）。
- 释放才持久化：`drag_ended → _commit_rack_macro_control_value`（`:4023-4026`，`:4101-4106`）→ `command_service.upsert_rack_control_macro(macro, "graph_rack_macro_value")`。
- 传输通道：`set_plugin_param` → `KernelCommandClient.send_no_wait`（`app/kernel/clients/rack_client.gd:43-44`、`kernel_command_client.gd:28-32`）→ `VitIpcClient` **UDP 127.0.0.1:4445 → godot_bridge → ZMQ REQ tcp://127.0.0.1:5555**（`vit_ipc_client.gd:2,7-8`；agent 侧转发 `agent/internal/bridge/bridge.go:228-248`）。应答经同一 UDP 套接字返回，单 active request + 队列（`vit_ipc_client.gd:15-16`），大应答走 file_reply（`:46-86`）。
- 速率特征：**每滑条 tick × 每 binding 一条 set_plugin_param**（无节流/合并）；持久化一次。

**写路径 ②（webui 宏滑条，双段式）**：
- preview：本地 patch + `postAgentMutations`（`App.tsx:1041-1044`）→ WebView2 bridge `window.chrome.webview.postMessage`（`:9286-9310`，channel `ask_vit_agent_mutations` `:202`）→ Godot `vit_dock_root.gd:42,2630,2704-2706` → `_apply_agent_webui_rack_macro_value_mutation`（`:2804-2822`：**未知 macro_id 忽略并警告** `:2813-2815`；注释明确"值 mutation 不得用紧凑 WebUI payload 覆盖已持久化的 bindings" `:2811`）→ `_apply_agent_webui_rack_macro_bindings`（`:2881-2906`，同样逐 binding `set_plugin_param`）+ rack 滑条 UI 同步（`:2825-2846`，set_block_signals 防回环）；commit 标志决定是否 upsert 持久化（`:2819-2822`）。
- commit：HTTP `/agent/invoke` tool=`control_set_macro_values`（`webui/src/lib/api.ts:158-177`）→ agent `setMacroValuesResult`（`harness.go:3045-3157`）：
  - **track.volume binding：agent 直写内核**——组 `set_volume` kernel 命令经 `executeKernelCommand`（VSP 通道，回执含 `execution.vsp`，`harness.go:3095-3128`）；
  - **插件参数 binding：agent 不写内核**——只计算 `applied_parameters` 目标值并返回 `ui_action:"rack_macro_value_changed"`（`:3134-3156`），实际写由 webui 把响应转 bridge mutation（`App.tsx:1062`、`:9484-9502`）交 Godot 执行（fallback：响应无 mutation 时 webui 自行补发 `webui_commit` mutation，`App.tsx:1064`）。
- 速率特征：preview 每 onChange 一条 bridge mutation（进程内同帧，快）；commit 一次 HTTP 往返 + 一次 bridge 回流。

**写路径 ③（agent 宏 CRUD 命令）**：`control_add_macro`/`control_rename_macro`/`control_add_binding` 在 harness 分发处即返回合成 ui_action（`harness.go:2489-2496` → `upsertMacroControlResult :2913-2963`、`renameMacroControlResult :2965-2995`、`addMacroBindingResult :2997-3043`），**无任何内核调用**；目标消歧逻辑 `resolveMacroTargets`（`:11099-11127`）。命令目录与别名 `tools/catalog.go:104-108,897-906`；prompt 面指引 `server.go:5735-5736`。mix workflow 合成同形执行条目 `mix_session_workflow.go:3645-3696`。

三态：**支持**（链路各环锚点齐全）；**不支持**项=agent 宏 CRUD 无持久化落点（ui_action fire-and-forget，前端仅消费已知 macro_id 的值 mutation）。

---

## §2 动态注册可行性

### 2.1 kernel 宏注册/变更接口

- **存在且完整**：`control_add_macro`（x/y/name/macro_count，undo 事务+saveProject+graph_revision 回执，`PluginRackControlService.cpp:2059-2107`）、`control_add_node`（`:2015-2057`）、`control_add_binding`（`:2109+`）、`control_update_node`/`control_set_node_value`/`control_update_binding`/`control_remove_binding`/`control_remove_node`（`CommandDispatcher.cpp:2819-2843`）、`control_set_macro_values`（`:2478-2545`，macropanel 输出批量置值+内核侧绑定应用）。
- 图变更统一走 `VitGraphSwapCoordinator::publishGraphChange` 并回 4 个 revision 字段（`PluginRackControlService.cpp:1387-1397`：graph_revision/active/pending/last_retired）。
- **现状零调用**（见 §1.1）。

三态：**支持**（接口在）；**需新开面**（接线到 agent 是新工作）。

### 2.2 Godot 宏 UI 渲染来源与刷新机制

- 渲染来源=repository 的 rack_control_macros 文档（§1.4）。
- 刷新=事件驱动：`project_event_hub.rack_control_macros_changed` 信号 → `_on_rack_control_macros_changed`（`Vit_Graph_Rack.gd:2055-2068`）：source 为 VALUE/POSITION（自反馈）仅重绘，**其余来源全量重渲染宏节点**（`_render_rack_macro_control_nodes`）。rack load_nodes 时也渲染（`:1850/:1878`）。
- 用户创建入口：dock 工具条"创建宏"按钮（`graph_rack_dock_adapter.gd:22,320-324`）与参数面板"绑定"按钮（`rack_param_surface_panel_factory.gd:549+` `_append_macro_bind_button`）。
- 侧边宏面板（Godot 侧）：`app/shell/side_panel/macro_panel.gd`，从 agent 回复 artifacts 摄入（`LLM_Chat_Controller.gd:7288-7289`）。

三态：**支持**（新宏入库即自动重渲染）。

### 2.3 用户手工改插件参数的写路径（含插件自有 GUI）

- 参数面板（rack param surface）：控件 → `_on_param_surface_control_value_changed`（`Vit_Graph_Rack.gd:2979-3016`）→ `set_plugin_param`（同 UDP→ZMQ 通道）→ 完成回执后 **get_plugin_parameters 回读校验**（`:3018-3030`）。面板滑条**仅 drag_ended 提交**（`rack_param_surface_panel_factory.gd:541-544`），checkbox/toggle 即时（`:387/:521`）。
- 插件自有 GUI：打开命令存在可达——`open_plugin_ui`（`rack_client.gd:35-36`；内核 `PluginRackControlService.h:56` handleOpenPluginUI）。用户越过裁定已允许（d53ad1e1），无禁开需求。
- 可观测性：GUI 参数变更最终落在 tracktion AutomatableParameter→Edit ValueTree；内核有 ValueTree 深度监听探针（§5.1）。**但 GUI 编辑到 ValueTree 的 flush 时机无实测证据**——IPC 写路径是显式 flush（`PluginRackControlService.cpp:1938-1941`），这一显式调用本身暗示非所有路径自动 flush。

三态：面板路径**支持**；插件 GUI 可达**支持**、可观测**需新开面**（真栈实测 GUI 拖动→delta_update 到达时延与覆盖率）。

---

## §3 传输通道选项（2026-09-29 扩为通用控制车道）

### 3.1 agent 页面交互控件盘点（webui）

| 控件 | 位置 | 传输路径 | 备注 |
|---|---|---|---|
| transport Play/RTZ/Stop/Seek | `App.tsx:2202-2210,2184` | HTTP POST `/agent/invoke`（`runTransportCommand :1178-1186`，busy 串行） | 每击一次完整 HTTP 往返+agent→kernel ZMQ REQ |
| A/B 试听 select/stop/judgment | `App.tsx:1327-1346`；`api.ts:83-88` | HTTP POST `/agent/audition/*`（`server.go:544-548`） | 试听触发播放也走 `transport.play` invoke（`App.tsx:3812`） |
| 宏滑条 preview | `App.tsx:1041-1044` | WebView2 bridge postMessage（同帧进程内）→Godot→UDP/ZMQ | 仅嵌 Godot WebView 时可用（`browserBridge` `:9286-9289`）；独立浏览器打开时 preview 落空（`postAgentMutations` 返回 false `:9341-9344`） |
| 宏滑条 commit / 改名 | `api.ts:158-196` | HTTP POST `/agent/invoke` | 串行确认式 |
| 轨道 M/S/R toggle | `App.tsx:2561-2563` | HTTP POST `/agent/invoke`（track.mute/solo/arm） | |
| 状态/事件获取 | `App.tsx:528`（250ms 忙/500ms 闲轮询 `/agent/events`）；`:367`（8s `/agent/ui/state` 全刷） | HTTP GET 轮询 | **无 SSE/WebSocket**，webui 侧纯轮询 |

- **实测延迟：本卡未跑真栈，无新测量数据**（零改动约束）。可复用的既有测量点：agent `[timing] kernel.send_command ms=…` 日志（`harness.go:12549`）。结构性判断：transport/试听类点击延迟=HTTP RTT+agent 处理+ZMQ REQ RTT；宏 preview=bridge 同帧+Godot→kernel 一跳（最短路径）。
- agent HTTP 全路由面：`server.go:537-575`（chat/confirm/interaction/events/state/ui.state/ui.context/invoke/actions/audition 族/artifacts 族）。

### 3.2 HTTP/ZMQ/websocket 面对"控制类事件流"的适配度

- **vsp_hub WebSocket 先例（现成可参照）**：`ws://127.0.0.1:8787/vsp/stream`（`scripts/vsp_hub_websocket_smoke.ps1:2-3`；`agent/internal/vsphub/websocket.go:153-193`）。已具备设计所需的三类机制：
  - 类型化事件面：vsp envelope（channel/type/schema，`kernel/vsp.go:222-247`）+ hub 内 channel 分发（`websocket.go:189` event.subscribe；`hub.go:159-226` realtime.subscribe/extension.register 等）；
  - **合并与丢弃策略**：realtime `latest_only` 模式按 key 只留最新帧（`realtime.go:185-187`）、每流 MaxHz 节流缺省 30Hz（`websocket.go:111-134`）、`drop_old:true`（`realtime.go:616-618`）——正是"macro 预览流丢弃旧值"的现成语义；
  - role/session 绑定（`websocket.go:75-108`）。
- 与既有对话/消息路径的分界现状：对话走 `/agent/chat` HTTP 请求响应+`/agent/events` 轮询（可靠、串行、终局语义）；控制类事件目前**没有专用车道**（transport 也是 invoke 串行、preview 借道 WebView bridge）——新 WS 控制车道若建，应独立于两者并沿用 vsp envelope 的类型化面。
- **kernel 侧控制速率通道：不支持**。控制=ZMQ REQ/REP 单命令（5555，`agent/internal/kernel/client.go:24,39-45`；Godot 经 UDP 4445→bridge，`bridge.go:228-248` 带 ReqMaxRetries）；telemetry=ZMQ PUB 5556 单向事件流（`ZmqGateway.cpp:123,271-283`；`bridge.go:279-322` 转发 UDP 4444 给 Godot）。无 kernel 侧低延迟控制订阅面。

三态：盘点与 WS 先例**支持**；webui 独立 WS 控制车道与 kernel 控制速率通道**不支持（需新建）**；实测延迟**需新开面**（真栈测量卡，建议挂 agent timing 日志+脚本断言）。

---

## §4 控件类型需求清单

### 4.1 webui 现有交互组件盘点

- 滑条：HTML `<input type="range">`（MacroSlider `App.tsx:9937+`；轨道音量 `:2472`）。
- 开关：checkbox（`App.tsx:1986`）、toggle 按钮 M/S/R（`:2561-2563`）。
- 文本输入：宏改名（`:9849-9868`）。
- 段选：focus 模式切换（`:218,1442`——界面模式切换，非参数控件）。
- **无 XY pad、无 dial/knob、无参数级离散段选组件**。
- `control_type` 字段链路存在（归一化缺省 slider，`macrocontrols/types.go:82`、`App.tsx:10210`）但 **webui 渲染无类型分支**（只有 MacroSlider 一种）。

### 4.2 参数元数据面（"参数类型→控件类型"映射的可得性）

- **内核 `get_plugin_parameters`（富元数据，映射输入基本齐备）**：`VitPluginGrabber::buildParameterDescriptors`（`VitPluginGrabber.cpp:117-172`）每参数给出：id/name/value/normalized_value/value_text、**min/max**（`:150-151`）、**is_discrete**（`:152`）、**num_steps**（`:153`）、**is_boolean**（`:157`，注释明确两态≠布尔、不得凭 label 升格）、normalized_role/display_group/control_relevance+priority、supports_automation、display_probe（`:57-93`：unit=label、**discrete_labels=index/value/label 枚举行**（`:40-54`）+all_labels、0/25/50/75/100% 归一化采样文本）。响应另有 recommended_groups/quick_controls/control_shell（`PluginRackControlService.cpp:2912+` 段）。
- **pluginprobe 离线探测面**：`SurfaceParameter`（`pluginprobe/types.go:45-58`）含 id/name/normalized/default/display_text/unit/automation/host_controllable/DisplayDomain{text,unit,min,max,scale}+Observed——**无显式 param_type/枚举值字段**（离散性需从 DisplayDomain/Observed 组合推断；运行面 kernel 描述符更全）。
- pluginsemantics：语义角色映射（`pluginsemantics/index.go`，normalized_role 消费侧）。
- agent 侧面板结构体已预留控件类型承载：`macrocontrols.Panel.Controls[]Control{Type,Min,Max,Default,Unit,Control,TargetTemplate}`（`types.go:25-35,122-132`）。

三态：元数据**支持**（bool→开关、离散→段选（枚举值+label 直接可得）、连续→滑条（min/max/unit/display_domain））；XY pad/复合控件**不支持**（无两参数组合语义与元数据配对）；webui 控件类型分化渲染**需新开面**（组件建设）。

---

## §5 参数级状态可观测性与写冲突面（2026-09-29 讨论补）

### 5.1 "插件 GUI 手改单参数"这类宏外写的可观测性

- **机制存在（事件驱动、非轮询）**：Edit 根 ValueTree 深度监听 `VitValueTreeDeltaListener`（attach `edit->state`，`VitHeadlessService.cpp:530,651`；`VitDeltaProbe.cpp:66-75`）→ 属性变更/子节点增删产生 DeltaEvent（`:99-127`）→ `VitDeltaRingBuffer`（**容量 4096**，`VitDeltaProbe.h:28`；满则丢+计数 `:11-27`）→ `ZmqGateway` 以 `delta_update{seq_id, action:"property_changed:<prop>", target_uid, value, timestamp}` 发布 ZMQ PUB 5556（`ZmqGateway.cpp:42-52,323`）→ agent bridge（`bridge.go:279-322`，seq gap 检测 `:407-417`）→ **shadow 泛化记录**：`nodes_by_uid[uid].delta_properties[prop]=value`（`shadow.go:227-284`，transport 噪声过滤、Before=null）+ 转 UDP 4444 给 Godot。
- 粒度：**ValueTree 属性级**（属性名+节点 uid+新值）；频率：事件驱动，无固定周期；**不携带旧值**（漂移检测需消费端自存期望值）。
- 覆盖确定性：IPC 写路径必触发（`handleSetPluginParam` 显式 flush `PluginRackControlService.cpp:1938-1944`）；**插件 GUI 路径的 flush 时机未实测**（同 §2.3，需真栈验证）。
- 参数全量读（get_plugin_parameters）全部为按需调用（各 plugin_*_apply 前后读、grabber 工作流），**无周期性参数轮询**。

三态：机制**支持**；GUI 路径实测**需新开面**；周期轮询**不支持**（也不必要，事件流已在——消费端缺一个把 delta_properties 用起来的漂移检测面，属**需新开面**）。

### 5.2 现有写路径的 revision/基线

| 写路径 | revision/基线 | 语义 |
|---|---|---|
| Godot rack 宏 per-tick 直写（set_plugin_param） | **无**（`handleSetPluginParam :1835-1956` 无校验、响应不带 graph_revision） | 绝对置值、最后写赢 |
| agent 直发 legacy 命令 | 同上 | 绝对置值 |
| **VSP typed 写**（executionports 实验线） | **CAS 存在**：feature flag `command.base_revision_cas`（`VspKernelReference.cpp:1054`）；写类命令带 `base_revision` → `validateCommandBaseRevision`（`:2499-2545`）不匹配即 **`stale_project_cut` 拒绝**；状态通道 `state.delta_request(base_revision)` → ops+resync（`:2327-2374`；`kernel/vsp.go:169-174,265-293`） | 乐观并发 |
| revision 本体 | 内容哈希驱动：快照 hash 变化才 `++revision`（`VspKernelReference.cpp:754-775`）；**rack 内插件写也推进 revision**（`:654-657` 注释明示） | 全工程级、非参数级 |
| 内核控制图命令（control_*） | 响应附 graph_revision/active/pending/last_retired（`:1387-1397`） | 图级 revision 回执（无写前校验） |

- executionports 先例（agent 侧组合增量写的完整范本）：`StaticEQVSPPort.writeParameterBatch` 带 `base_revision`+`readback:true`+幂等键（`staticeq_delta.go:67-82`）；`rebaseAfterWrite` 每写后快照、断言 revision 前进+epoch 不变（`:52-65`）；**探针式增量组合**——小步可逆 probe 写测斜率→恢复→算归一化目标（`:84-152`），CAS 重试窗口=每写多一次快照 RTT、epoch 变化即 fail-closed。trackpan/staticbalance 同级实现（`trackpan_vsp.go:15`、`staticeq_vsp.go:18,199`）。

### 5.3 参数写语义：绝对 or 增量

- **内核：仅绝对设置**。`handleSetPluginParam` 三种值形态（raw value / normalized_value / value_text 经 `param->stringToValue`，`:1842-1905`）最终都走 `setParameter`/`setNormalisedParameter` 绝对置值（`:1923-1936`）；**无 delta/相对写命令**（全命令面无此形态）。
- agent 侧组合：**可行且有先例**（上述 staticeq 斜率探针法）；代价=探针两次额外写+每写 rebase 快照；失败模式明确（目标超归一化域即 fail-closed `:136-138`）。

三态：CAS 与 revision **支持**（VSP 通道）；内核增量写**不支持**（agent 侧组合可行=已验证范式）；delta 旧值**不支持**；漂移检测消费端**需新开面**。

---

## 总三态结论表

| 事实面 | 支持 | 不支持 | 需新开面 |
|---|---|---|---|
| §1 宏数据流 | A 体系（前端 JSON 宏）存储/上行/归一化/渲染/三条写路径全链可述 | agent 宏 CRUD 无持久化落点 | A/B 两套宏体系归一裁定 |
| §2 动态注册 | kernel 控制图命令面完整；Godot 事件驱动重渲染；参数面板写+回读；open_plugin_ui 可达 | — | 内核宏命令接线；插件 GUI→delta flush 真栈实测 |
| §3 传输通道 | 控件盘点+vsp_hub WS 先例（类型化/合并丢弃/节流） | webui 独立 WS 控制车道；kernel 控制速率通道 | 真栈延迟测量 |
| §4 控件类型 | 参数元数据（bool/离散枚举/min-max-unit）基本齐备 | XY pad/复合控件；webui 控件类型分化渲染 | webui 组件建设 |
| §5 可观测与写冲突 | 属性级事件流+VSP base_revision CAS+revision 覆盖插件写+agent 增量组合先例 | 内核增量写；delta 旧值；参数周期轮询 | GUI flush 实测；漂移检测消费端 |

## 与设计假设冲突/需设计侧裁定的事实（本卡核心产出）

1. **两套宏体系并存且互不感知**：内核控制图（ValueTree+bindings 内核侧应用+graph_revision 回执）vs 前端 rack_control_macros.json（Godot 侧应用 bindings）。agent 生成宏控件的设计必须先裁定落点；现状 agent 的 control_add_macro 是 ui_action 纸面闭环——前端对未知 macro_id 的值 mutation 直接忽略（`vit_dock_root.gd:2813-2815`），"agent 建宏"当前到不了持久层。
2. **插件参数的实际写者是前端（Godot），不是 agent**：`control_set_macro_values` 对插件参数 binding 只算值返回 ui_action，实写由 Godot 执行。若宏提交写要带 CAS，只包 agent→kernel 段盖不住 Godot per-tick 直写段；而 Godot 直写路径（set_plugin_param）无 revision 无校验。
3. track.volume 是唯一 agent 直写内核的宏 binding（走 VSP，带幂等键）——音量类宏的乐观并发可直接复用此通路。
4. webui 宏 preview 依赖 WebView2 bridge，仅嵌 Godot 时有效；独立浏览器打开 webui 时 preview 落空（`App.tsx:9341-9344`）——通用控制车道若以 webui 为面，需覆盖此场景。
5. 无实测延迟数据（本卡零改动约束）；结构性最快路径=bridge 同帧+Godot→kernel 一跳，最慢=HTTP invoke 全往返。建议后续用 agent 既有 `[timing]` 日志面出测量卡。
6. delta_update 不带旧值且环形缓冲满即丢（4096）：宏漂移检测若以 delta 流为基，需消费端缓存期望值并接受丢弃语义（或改用快照 hash 比对兜底）。

## 勘察边界声明

- 只读勘察，未运行真实栈；§2.3/§5.1 的插件 GUI flush 时延、§3.1 的交互延迟均无实测数据，已标"需新开面"。
- Godot 前端在仓库外（D:\Godot\project\vit-daw-frontend），行号为当日工作树快照。
- 本报告不构成设计决定；A/B 体系裁定、车道建设、控件分化均属 DESIGN 卡范围。
