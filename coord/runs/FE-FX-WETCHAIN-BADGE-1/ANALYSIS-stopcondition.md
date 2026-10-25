# FE-FX-WETCHAIN-BADGE-1（185）停止条件触发上交件（2026-10-25 晚窗，执行侧）

**结论：卡面停止条件触发——「pluginList 回执面不含 FX 轨 rack 成员」。** 真栈首轮 L12 实证轨级 pluginList 只呈现 rack 容器实例（名 "New Rack"），rack 成员（TDR Nova）住在 rack DAG 内部。**同等数据已在另一既有回执字段（get_project_state 的 tracks[].rack.nodes）**，零新请求面约束可保持——是否换源继续，上交主管裁定。

## 一、触发证据（真栈首轮 run=sendroute_gui_20261025_193814，exit 1，81 断言 8 败）

L12 三败（族 2=设计面，本上交件主体）：

1. `L12 badge shows loaded plugin after rack_add_node | New Rack`——装载 TDR Nova 后徽标显示 **"New Rack"**（rack 容器实例名），非插件名。
2. `L12 expanded SendIn card badge matches loaded plugin | badge=New Rack`——同上（展开态 SendIn 卡）。
3. `L12 list_tracks receipt carries fx rack member (data source) | fx rack member missing in receipt`——**直接断言**：list_tracks 回执 FX 轨 plugins 不含 Nova。

首轮完整清单另有五败（族 1=环境面，见 §四）：L2/L4/L5/L6/L7 全部指向主检出 build 目录运行时工程预存边 `1032→1037@+6.0`（183 首轮 67/72 同款污染）。

## 二、根因（两侧源码亲核）

- **装载形态**：本仓插件装载走 `rack_add_node`（rack DAG 模式；`move_plugin` 在内核 DAG 模式已废弃，rack_send_route_rules.gd:57-60 注释同口径）。内核把插件装进 tracktion **RackInstance 容器**（`ensureRackInstanceOnTrack` 兜底建 rack，CommandDispatcher.cpp handleRackAddNode）。
- **轨级 pluginList 序列化**（TrackService.cpp:34-46 `createPluginArray` → `createTrackState`）：只序列化 `track.pluginList.getPlugins()` 顶层——FX 轨顶层=[receiver(auxreturn), 系统隐藏(volume/level), **rack 容器实例("New Rack"=tracktion 新建 rack 默认名)**, wetSend(auxsend)]。rack 成员不在顶层。176 勘察锚点「receiver@头+rack 成员+wetSend@尾」的轨级链序成立，但「rack 成员」在轨级回执里呈现为**单个容器节点**，成员名不可派生。
- 由此，卡面①预设的「徽标内容从同源 pluginList+rack 成员序派生」的字面路径不成立——**停止条件字面触发**。

## 三、替代数据面（已亲核，供裁定换源参考——零新请求面约束可保持）

- **内核侧**：`get_project_state` 回执每轨带 `rack: {rack_item_id, nodes, edges, ...}`（CommandDispatcher.cpp:1797 `createRackState`，3754 行挂轨行）；`nodes[]` = rack 容器内成员全字段：`node_id/plugin_item_id/item_id/id/plugin_id/name/plugin_name/type/enabled/x/y/zone_id/clip_scope/plugin_load_state...`——**成员名直接在场**（185 徽标所需全部信息）。
- **前端 repository 已缓存已消费**：project_repository.gd:106-114 把 `track.rack.nodes` 规整进 `_project_snapshot["rack_nodes"][track_id]`（rack.nodes 缺省回退轨级 plugins）；`get_rack_nodes_by_track()`（:287）已全轨可读——**167 目标选择器侧链口在用的同一面**（target_selector_rows 的 rack_nodes_by_track 参数）。即：换源=消费既有 get_project_state 回执的另一字段，不加任何新请求面（卡面①的实质约束满足）。
- **到达/刷新链（已核，挂点候选留裁定后实现定）**：repo 为纯被动缓存（无信号 emit）；真实 GUI 链=工程刷新（vit_control_v_1.0.gd:375-396 `vit_refresh_project_from_kernel`→get_project_state→`_apply_kernel_project_state_to_legacy_scene`→repo.apply）+ rack 视图既有外部变化面（161 挂的 project_loaded 广播→拓扑/快照双拉取）。徽标刷新可挂：a) project_loaded 外部变化面追加「repo rack_nodes 重读+轻量标签刷新」（与现实现同位）；b) 工程刷新链末端。候选 a 与 161 同构、改动最小。

## 四、族 1（预存边污染，复跑可消，非本卡缺陷）

- 根因：`dev_agent_smoke.ps1:1162-1207` 的 VIT_PROJECT_XML 隔离**只在 journey berth 场景设置**；SendRouteGuiSmoke 非旅程 → 内核以 exe 目录（主检出 build\...\Release）为 WorkingDirectory 启动 → 加载该目录 default_project.xml（含预存边 1032→1037@+6.0，183 首轮取证在卷的同一条边）。
- 修正（183 先例同款，无需改脚本）：复跑时调用方 shell 预设进程级 `VIT_PROJECT_XML` 指 git 干净版 default_project.xml 隔离副本（stack worktree 的 VitApp\Workspace\default_project.xml 复制到 run 目录），子进程内核继承。

## 五、当前工作状态（裁定后可快速续）

- 前端分支 `port/fe-fx-wetchain-badge-1`（worktree D:/Godot/project/vit-daw-frontend-wetchain1，基线 ee03262）已有实现：呈现面（合一 icon+SendIn 卡徽标行/166 chip 形态/fail-closed 语义/「（空）」形态/183 拖动门不互抢）+ rack_send_route_rules.gd 派生纯函数 + Vit_Graph_Rack.gd 刷新面 + 独立探针 probe_fx_wet_chain_badge（35/35）+ stack L12 腿 + 180 横幅笔误顺带修。
- **换源影响的仅是派生输入**（list_tracks 轨级 plugins → repo rack_nodes 快照）与刷新挂点（快照回执 → repo 更新到达面）；呈现层/探针断言结构/L12 腿骨架均可复用。预计 ≤0.25 天。
- headless 零回退全家绿已落（headless/ 目录 9 套：K3 313/popup 111/realinput 15/shrink abc/start_feed 244/fader 35/badge 35；popup 首败=worktree 缺 .godot/imported 产物，基线 ee03262 对照全绿+import 重建后全绿，环境因素非 diff）。

## 六、待裁点

1. **主裁定**：数据源换 get_project_state 的 rack.nodes（repo rack_nodes 快照面）继续本卡，还是另行处置（如要求内核在 list_tracks 补 rack 成员=改内核面，越出本卡文件域）。
2. 徽标成员序口径：rack.nodes 序列序即内核 rack 成员序（裁定 176 锚点的「成员序」在此面的等价物），是否认可。
3. 族 1 复跑修正（VIT_PROJECT_XML 显式透传）随裁定后复跑一并执行。

——执行侧：ZCode GLM-5.3 L1 PC 前端执行会话（185 卡会话），2026-10-25 晚窗。
