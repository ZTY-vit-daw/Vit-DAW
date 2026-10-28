# FE-LANE-CATALOG-REPO-WIRE-1（207）真栈腿回执 — lanecatalog_20261028_1

- owner：ZCode GLM-5.3 PC 执行会话（207 卡会话）；2026-10-28 晚窗
- run：`coord/runs/FE-LANE-CATALOG-REPO-WIRE-1/lanecatalog_20261028_1`（本目录，随 `port/fe-lane-catalog-repo-wire-1-stack` 入库）
- 被测：前端 worktree `D:/Godot/project/vit-daw-frontend-lanecatalog1@port/fe-lane-catalog-repo-wire-1` @ **3b5df90+cecff4f**（领取基线 3b5df90 亲核=origin/main）
- kernel：**206 自建二进制只读复用** `D:/Vit_DAW_wt_lgrow1_stack/VitApp/build/VitApp_artefacts/Release/VitApp.exe`（sha256 `c9a258de…` 与 206 回执同源；`535c98b4..264704a8` VitApp/+agent/ 零改动亲核——本卡全部改动在前端仓）pid 2340，本腿 ps1 `-StartKernel` 自启
- agent：stack worktree `D:/Vit_DAW_wt_207stack` 构建（Go 零改动，`go build ./cmd/vitagent` exit 0）pid 35140
- vsphub：**主仓 `D:/Vit_DAW/agent/bin/VspHub.exe` 只读复用**（sha256 `b10cc412…`=173 回执同源；默认 8787/5555/5556）pid 5860 本腿手启——**用户手测栈形态含 hub（GUI VitIpcClient 自启链），VSP 喂食路径复现保真所需**
- VIT_PROJECT_XML：run 目录 git 干净版隔离副本（blob `287956e7` 基线=202/203/205/206 首腿同基线，hash-object 亲核）显式透传
- ps1：`-RepoRoot D:/Vit_DAW_wt_207stack -StartKernel -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -KernelExe <206 exe 复用>`（console/lanecatalog_stack_leg.ps1，exit 0）

## 一、勘察结论（发卡根因修正——证据在卷）

1. **用户手测 GUI=VitDockRoot（dock 主场景），非 legacy**：`%APPDATA%/Godot/app_userdata/Vit_DAW_Frontend/logs/godot.log`（2026-10-28 18:29:16，用户手测节第三轮会话）亲读——`_ready (res://vit_dock/scenes/vit_dock_root.gd:234)` 在场+`[rack.drop] kernel_track_id=1007 … Pro-Q 3.vst3`（用户拖装现场）；三份 coord 在卷先证（VITNOTE-DOCK-MOUNT-1/FE-SPLIT-RESIZE-REGRESSION-1/SHRINK-RECON）：启动页进工程默认 Vit-Dock 工作区，legacy 仅 `VIT_USE_LEGACY_MAIN=1` 回退。
2. **「ProjectRepository 生产零实例化」不成立于用户 GUI 形态**：dock root `_ready → _ensure_project_services → ProjectDataManager.new()`，其 `_ensure_services` 以 `ProjectRepository.new()+add_child` 装配（project_data_manager.gd:132+，class_name 引用故路径 grep 不可见）。探针 P1b/P1c 亲证：repo/hub 组节点在场且为 dock 子树内实件。
3. **真实断供点①（喂食 schema）**：dock 喂食优先走 VSP `state_snapshot("project.timeline")`；内核紧凑快照 `normaliseTrackArray`（VitApp/Source/Service/VspKernelReference.cpp:612+）白名单**剥每轨 `plugins` 行**（rack 保留）——repo `plugins_by_track` 恒空 → 未挂轨 `get_volume_pan_plugin_item_id_for_track` 恒空 → 无默认音量参数、目录全空。161 卡注释（rack_send_route_rules.gd:191）在卷先证该剥列事实。
4. **真实断供点②（变化面重读）**：rack 拓扑命令回执后 dock root 已回拉重喂 repo（vit_dock_root.gd:1196 `rack_add_node→vit_refresh_project_from_kernel`，**该链本卡零改动**），但 lane 目录的 receipt 触发（:713 `_CATALOG_TRIGGERS`）同步读**旧 repo 快照**（dock 重喂为 async await），重喂完成后无任何重读触发 → 装后列表仍旧。
5. **卡面 :394 锚点（app/legacy/vit_control_v_1.0.gd）非用户链路**：legacy 场景仅显式 `VIT_USE_LEGACY_MAIN=1` 进入，本卡零改动（文件域内未触碰）。停止条件①②的处置：既有 repo 驱动设计被发现且**未重复造**（修复=补 schema 守卫+消费端重读钩子，非新建装配/喂食链）；锚点修正已在回执披露，交主管裁定 legacy 面是否另行接线。

## 二、实现（cecff4f，5 文件 +629/-1）

1. `vit_dock/scenes/vit_dock_root.gd`：`_refresh_project_repository_from_vsp` 加 `_vsp_snapshot_tracks_have_plugins` 守卫——任一轨行携带数组型 `plugins` 键才走 VSP 落库，缺列（紧凑面恒缺）回落既有 legacy IPC `get_project_state` 全量喂食路径（空轨集/异常形状保守回落）。内核侧白名单将来扩列后自动恢复 VSP 路。transport-only 面（:2866 `apply_kernel_transport_only`）零改动。
2. `app/tracks/track_scene/track_envelope_lane_2d.gd`：①repo 缺位一次性 `push_warning`（fail-loud，修复前静默 return）+`get_repo_missing_warn_count()` 探针面；②懒挂 `hub.project_loaded`（`_ensure_project_event_hub_wiring`，bind/展开两入口兜底）——展开态下 repo 快照更新→目录+●N 重拉（变化面收口）。
3. 探针两件：`tools/stack_probe_lane_catalog_repo_wire.gd/.tscn`（§5.1 装配面口径）+`tools/probe_lane_catalog_fail_loud.gd`（-s 离线）。

## 三、探针义务兑现（PROTOCOL §5.1 本卡成文案例）

- **装配面断言必入**：探针零 repo 自建（无 `_REPOSITORY_SCRIPT`、无 repo `.new()`，可 grep 亲证）——P1 以 `VitDockRoot.tscn` **实件**入场，断言 project_data_manager/project_repository/project_event_hub 组节点在场**且均为 dock 子树内**（P1c `is_ancestor_of` 亲证）；P2 喂食=dock root 自身 deferred 快照链（零探针侧喂食调用）；P3 目录非空+vp id 可解析；P4 lane 生产消费（真行装配+`bind_virtual_track` 生产绑定+生产开关展开）。
- **禁注入（205 口径）延续**：全程无 `set_plugin_catalog`/`set_track_volume_plugin_id` 调用（可 grep）。
- **DIAG/正式双模式**：`VIT_PROBE_207_DIAG=1` 时 P3-P5 断言取缺陷签名（复现轮），默认断言修复态。

## 四、真栈结果（§8 声明：LLM=0 全确定性；成功=exit 0；失败分类=assertion_fail/environment_interrupt/script_exception）

| 轮 | 命令形态 | 结果 | 退出码 |
|---|---|---|---|
| DIAG r1 | `VIT_PROBE_207_DIAG=1` 探针场景 | **23/23 缺陷签名全绿**（P3a vp id 恒空+P4e 目录全空+P4f 无默认车道+P5c 目录不跟随——用户手测三症状逐一复现；P5b repo 重喂正常=断点在消费端） | 0 |
| 正式 R1 | 探针场景（默认模式） | **24/24 PASS**（P1 守卫活语义+P3a vp id 可解析+P4e/f/g 目录到达+默认车道落定+轨道组恰 volume/pan+P5b/c repo 重喂→目录跟随） | 0 |
| 正式 R2 | 同上复跑 | **24/24 PASS** | 0 |

离线：`probe_lane_catalog_fail_loud` **18/18 PASS**（F1 fail-loud 一次性+注入面不因开合被清+无 repo 时目录保持；F2 transport-only 单元语义——全量喂食后增量不清 rack_nodes/plugins_by_track、仅更新 transport 面、vp id 保持可解析=卡面「transport-only 不得误触发全量重喂」语义面；F3 源标记）。

## 五、离线全家回归（24 件+新增 1 件）

- 24 件全家（206 同清单）：**23 件首轮全绿 exit 0**；`rack_plugin_search_popup` 首轮 exit 1（110/111，「leaf icon null」）——**非本卡 diff**：基线 worktree（零改动）111/111 绿+基线+本卡两文件二分 111/111 绿；清 `.godot` 重导入后本卡 worktree 复跑 **111/111 绿 exit 0**（`_post` 日志在卷）。定性=worktree 导入缓存态（首轮 import 与后续全 UI 栈探针运行混态），代码面无关留证。
- 新增 fail-loud 探针 18/18 绿。

## 六、三纪律与隔离亲证

- 登记 `00eb75c8` 推送 19:06 后 **19:06:47 即时复核**（远端亲读置顶+进程/端口全零，<60s 窗口）+driver `PREFLIGHT_CLEAN` 二次预检；无 already listening（kernel/agent=ps1 自启；hub=本腿手启，启动前 8787 亲核空）；登记先于启动。
- VIT_PROJECT_XML 隔离三面：run 副本 `287956e7→2efe87b0`（内核写入=隔离生效）；stack worktree VitApp/ 运行后 git 干净（本体仍 287956e7）；主检出 default_project.xml `44587011` mtime 18:29:08=用户会话遗留值**零触碰**。
- 拆栈=owner 手拆 hub 5860/agent 35140/kernel 2340（三 pid 路径逐一亲核=本腿所属）→19:18:13 复核 VitApp/VitAgent/VspHub=0、TCP 5555/5556/7878/8787/18787 listen=0、UDP 4444/4445=0。

## 七、披露

1. **发卡根因叙事修正**：「生产零实例化」与「:394 锚点=用户链路」两前提被运行时证据推翻（§一）；修复落在卡面文件域预期内的「主场景引导」文件（vit_dock_root.gd）+lane 文件；legacy 文件域零触碰——legacy 面（显式 env 回退形态）repo 缺位现状维持，现以 fail-loud warning 留痕，是否另行接线交主管裁定。
2. **喂食路径权衡**：守卫触发时 VSP 快照被取后弃用、追加一次 legacy `get_project_state`（每 refresh 双调用）——换正确性（全量 schema），与 legacy 主控时代每 refresh 单次 get_project_state 相比多一次 VSP 快照取；内核侧白名单扩列后自动回单次路径。
3. **vsphub 补位**：ps1 最小腿不启 hub，本腿按用户栈形态手启（173 同源二进制只读复用）——ps1 是否应内置 hub 腿交主管参考。
4. **端测边界**：本腿=agent HTTP/ZMQ 面+Godot headless 生产树断言，**不含 webui 渲染面与用户旅程面**；手测闭环义务（卡面目标⑤：用户手测重验选择参数列表=轨道组+已装插件在场）待用户手测节执行。
5. **popup 探针缓存态事故**：首轮红非代码面（§五），缓存清理后绿——worktree 首轮 import 与栈探针全 UI 运行的缓存混态值得后续留意，未另开修复卡（非缺陷）。
