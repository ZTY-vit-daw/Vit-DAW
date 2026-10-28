# FE-RACK-DELETE-REDRAW-1（212）真栈腿回执 — rackdel_20261028_1

- owner：ZCode GLM-5.3 PC 执行会话（212 卡会话）；2026-10-28 晚窗–10-29 凌晨（手测节第四解锁）
- run：`coord/runs/FE-RACK-DELETE-REDRAW-1/rackdel_20261028_1`（本目录，随 `port/fe-rack-delete-redraw-1-stack` 入库）
- 被测：前端 worktree `D:/Godot/project/vit-daw-frontend-wt-rackdel1@port/fe-rack-delete-redraw-1` @ 84396f1 基线 + 探针提交 76c7bae/900dc6d/3f4ab36（**零生产改动**——本卡结论=零修复）
- kernel：主检出重建二进制只读复用 `D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe`（2026-10-28 21:26 主管重建，含 III-A 190/201 面；209/211 同源复用；本卡 VitApp/+agent/ 零改动）pid 29988，ps1 `-StartKernel` 自启
- agent：stack worktree `D:/Vit_DAW_wt_212stack` 构建（Go 零改动）pid 23592
- vsphub：主仓 `D:/Vit_DAW/agent/bin/VspHub.exe` 只读复用 pid 6832 本腿手启（启前亲核 8787 空）
- VIT_PROJECT_XML：run 目录 git 干净版隔离副本（blob `287956e7` 基线=202-211 腿同基线，hash-object 亲核 `287956e71e03532139b35e8f616fb283f4dca1ca`；运行后 `da3d5655`=内核写回被隔离副本承接；stack worktree `VitApp/` 运行后 git 干净）

## 一、勘察结论（卡面前提被证据推翻——零修复裁定，209 同形）

**卡面前提**「删除效果器后机架节点不消失=删除回执重绘链断」在现行基线（前端 84396f1+内核 21:26 重建）**两条删除向量均不成立**，停止条件①（既定设计已在且工作）命中：

1. **手势删除路径全链工作（DIAG r8 亲证，事件管线口径）**：节点选中态+`Input.parse_input_event` KEY_DELETE 真实事件注入 → dock `_unhandled_input`（vit_dock_root.gd:2267-2276）→ shortcut_manager.dispatch_shortcut（:2288）→ viewport frame → adapter `handle_viewport_shortcut("delete_selection")`（graph_rack_dock_adapter.gd:166-180）→ rack `vit_delete_selected_nodes_from_shortcut` → `_on_delete_nodes_request` → `_ipc_delete_plugin_for_node`（Vit_Graph_Rack.gd:7648）→ delete_plugin IPC → 内核 removeFromParent（PluginRackControlService.cpp:3438）reply ok → **dock 回执处理器 delete_plugin 在册对称回拉（vit_dock_root.gd:1217-1224——卡面「add/delete 回拉不对称候选」不成立）** → `vit_refresh_project_from_kernel` → store VSP delta（transport=state.delta 亲证）→ repo rack_nodes/plugins_by_track 双面清除 → project_loaded 广播 → adapter `_on_project_loaded`→`_reload_graph_rack`（invalidate 签名+load_nodes）→ **画布节点消失+双面一致**。
2. **外部删除路径（agent 等价）亦工作（DIAG r9 亲证）**：经 VSP hub `legacy_command_request` delete_plugin（不经 VitIpcClient=无 GUI 回执）→ 内核 `ZmqGateway` PUB `delta_update(action=node_removed)`（ZmqGateway.cpp:45,VitDeltaProbe.cpp:123）→ hub websocket 推送 → 前端 `telemetry_manager._handle_delta_update`（telemetry_manager.gd:672-697）action 过滤命中 node_removed → 0.22s 防抖 `_schedule_project_refresh_from_delta` → `vit_refresh_project_from_kernel` → **repo+画布 8s 窗内双面清除**。即现行基线对删除已有**三层触发**（GUI 回执面/遥测 delta 面/173 stale 沿兜底）。
3. **add 对称基线活体（每轮 P2b-b）**：dock 在场后 rack_add_node → 画布无手势获得第二节点——add 回执→刷新→adapter 重绘链每轮绿。
4. **用户症状残余面（不可 headless 复现，上交主管裁定）**：
   - **缺口 A（代码锚点亲证，域外）**：实时 websocket 断供窗内丢失的 delta 无恢复补偿——`websocket_recovered` 沿（vsp_realtime_adapter.gd:718 发射）**无任何消费者**，`_on_vsp_realtime_status`（telemetry_manager.gd:665）只在 `websocket_stale` 沿触发一次回拉。若删除发生在断供窗内且无后续 GUI 命令/stale 沿，机架/repo 可无限期陈旧=用户症状的一种机制形态。修复（recovered 沿补一次防抖回拉）在 `app/kernel/autoloads/telemetry_manager.gd`=**本卡文件域外**→按卡面停止条件上交扩域裁定。
   - 缺口 B：用户实际删除向量未知（手势/agent 会话/其它）；全 GUI 并发与长会话态（VSP 游标漂移等）headless 不可复现——用户手测节在新栈重验=收口动作（209 先例同形）。
   - 探针面伴生披露：adapter 无选轨时**自动绑首轨**（graph_rack_dock_adapter.gd:830-844 `_select_initial_track`）——若用户机架绑轨与被删轨不同步，机架呈现不反映被删轨（探针 r7 轨错配亲证该行为；生产由轨焦点跟随链覆盖，非缺陷折卡面）。

## 二、实现（本卡=零生产代码改动；交付探针一件）

`tools/stack_probe_rack_delete_redraw.gd/.tscn`（真栈，随 port/fe-rack-delete-redraw-1@3f4ab36）：
- **旅程面（211 新口径）**：删除走真实事件管线（选中态+`Input.parse_input_event` KEY_DELETE→Viewport 派发→dock `_unhandled_input`→shortcut 链），禁 handler 直调；headless 焦点面回退生产调用链 `dispatch_shortcut`（两路生效形态 SUMMARY 披露——三轮正式/取证轮均 event_pipeline 直达，回退未触发）。
- **§5.1 装配面**：VitDockRoot.tscn 实件+`_stage_push_view` 生产上台链；repo/hub/pdm 组节点 dock 子树亲证；**零 repo/rack 自建**（无 `_REPOSITORY_SCRIPT`/repo `.new()`/GraphEdit.new() 可 grep）；多 adapter 实例枚举+非空视口 id 判真身（r1 教训入探针）；生产 context 绑轨（set_view_context track_scope，r7 教训入探针）。
- **断言面**：P0 真实 VST 发现+装载（Supermassive）→P1 装配+绑轨→P2 基线双面→P2b add 活体对照→P3 手势删除（L1 回执/L2 project_loaded/L3 repo 双列/L4 画布四层遥测）→P4 双面一致主断言→P5 参数列表同步（plugins_by_track）→P3x 外部删除腿（VSP command=agent 等价）双面+一致→P6 store 传输取证（只读）→P7 清场。DIAG 模式（`VIT_PROBE_212_DIAG=1`）签名如实记录不预设断点层。

## 三、真栈结果（§8 声明：LLM=0 全确定性；成功=退出码 0；失败分类=assertion_fail/environment_interrupt/script_exception）

| 轮 | 形态 | 结果 | 退出码 |
|---|---|---|---|
| DIAG r1 | 探针 v1 | P2b 假阴性：按名深度优先枚举拿到空视口 adapter（树内多实例亲证）——**探针缺陷**，迭代 | 1 |
| DIAG r2-r5 | 探针 v2-v5 | 逐层取证：单 adapter 确认/锚点对=重建走完 spawn 段/repo 行形状/rack 实收数组 `[Volume&Pan,LevelMeter]`——层层收敛 | 1 |
| DIAG r6 | 探针 v5.1 | 场景脚本解析错（print 位置在 vst1_id 赋值前）→场景未载入空转超时——**探针缺陷**非被测面 | 124(kill) |
| DIAG r7 | 探针 v6 | 断点定谳=探针面轨错配（adapter 自动绑首轨 1007≠探针轨 1070，rack 实收内建件数组）——**探针缺陷**，加生产 context 绑轨 | 1 |
| DIAG r8 | 探针 v7 `VIT_PROBE_212_DIAG=1` | **手势路径全链绿**（事件管线直达内核+双面清除+一致）；P4b 缺陷签名断言 FAIL=签名未复现的如实披露 | 1 |
| DIAG r9 | 探针 v8 同上 | 手势绿+**外部路径亦绿**（遥测 delta 链双面清除）；P3x-d/e 签名断言 FAIL=签名未复现；P3x-c 状态解析小错（信封 type/payload.status 双口径修正） | 1 |
| **正式 R1** | 探针 v9（修复态=基线即工作态） | **29 PASS 全绿**（P0-P7+P3x 全过：手势+外部双向量双面一致） | **0** |
| **正式 R2** | 同上复跑 | **29 PASS 全绿**（同形） | **0** |

## 四、离线全家回归

**26/26 全绿 exit 0**（offline_regression/ 目录 26 份 log；209 同族清单+file_reply_volume_face；调用形态=.gd 件 `-s tools/probe_x.gd` 脚本模式+file_reply_volume_face 场景模式）。零回退。

## 五、三纪律与隔离

- 登记 `f1749d70` 推送后 **23:44:13/23:44:21 即时复核**（远端亲读置顶+进程/VitApp/VitAgent/VspHub=0+TCP 5555/5556/7878/8787/18787+UDP 4444/4445 全零，<60s 窗口）+driver `PREFLIGHT_CLEAN` 预检（188 守卫）；vsphub 手启 6832 前亲核 8787 空；登记先于启动。
- 错峰：211 真栈腿 23:08 登记 → 本腿长轮询至其 23:38 释放（67415d67）后 23:41 登记。
- 拆栈：owner 手拆 VspHub 6832/VitAgent 23592/VitApp 29988（三 pid 路径逐一亲核=本腿所属）→ 复核进程 0、本腿 TCP/UDP 全零。**留痕**：UDP 4444 现由 pid 13612 `Godot_v4.6.1-stable_win64.exe`（窗口版=用户编辑器形态，非本腿启动；23:44 预检时 4444 空=测试窗内用户开启）持有——§2.2 不擅自终止，原样保留并披露。该占用使探针端 telemetry_manager legacy UDP 4444 bind 失败（日志在卷），遥测走 websocket 路（外部删除腿正是经此路验证=不影响结论）。
- VIT_PROJECT_XML 隔离生效（287956e7→da3d5655）；stack worktree `VitApp/` git 干净；主检出 `VitApp/Workspace/` 零触碰。

## 六、上交主管裁定项

1. **[等待用户:手测重验]** 用户手测节在新栈重验删除效果器（手势+agent 两条向量）——卡面手测闭环即收口动作。
2. **域外缺口 A 扩域裁定**：`websocket_recovered` 沿无消费者（telemetry_manager.gd:665 只吃 stale 沿）——断供窗 delta 丢失无恢复回拉，修复=app/kernel/autoloads/ 域（本卡域外）。建议折小卡（P2，一处沿消费+防抖回拉同 173 形态）。
3. 探针面伴生披露：adapter 无选轨自动绑首轨（graph_rack_dock_adapter.gd:830-844）——生产由轨焦点跟随覆盖；若用户手测再现症状，优先核对机架绑轨与被删轨一致性。
