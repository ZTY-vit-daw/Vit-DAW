# FE-SEND-EXTERNAL-REFRESH-1（池序 161）执行回执

- owner=GLM-5.3 L2 PC ZCode 执行会话（161 卡会话）；领取=主仓 05a33d65（todo→doing，基线 origin/main=39326111）
- 前端仓分支 port/fe-send-external-refresh-1@**10324d8**（基线 0e244f0=main）；worktree D:/Godot/project-vit-daw-frontend-sendext1
- 主仓构建/工件 worktree D:/Vit_DAW_wt_sendext1_stack@port/fe-send-external-refresh-1-stack（基于 K3.2 腿分支 f39f595e 续，Go 生产零改动）
- 真栈占用登记 2cc1ce1e；run=**sendext1_gui_20261021_220842**

## 根因（代码取证，比卡面诊断深一层）

1. **外部写零触发**：`track.send.list` 拉取只挂在 GUI 自身 send 命令回执（`_on_send_control_ipc_completed`，`_SEND_REQUEST_PREFIXES:338`）与机架 `load_nodes` 的 deferred 拉取；agent 经 VSP→内核的写不产生任何 GUI request_id。`send_topology_changed` hub 信号唯一发射者=机架 `_apply_send_topology`——无机架视图时 SEND 槽位/轨 FX 徽标永不刷新。
2. **更深的断点（用户「视图重建仍不显示」的机制解释）**：GUI 工程快照刷新走 VSP-first（`_refresh_project_repository_from_vsp`→agent hub 8787→内核 `makeCompactProjectSnapshot`）——内核 `normaliseTrackArray`（VspKernelReference.cpp:603）白名单**剥掉每轨 `plugins`**，而 auxsend/auxreturn 只存在于 legacy `plugins` 数组（SendRouteService 插入 track.pluginList；rack DAG `createRackState` 的 nodes 不含 auxsend）。⇒ 外部刷新后 repository `rack_nodes[rack-less 源轨]` 恒空，SEND 卡即使重建视图也无数据可生成。

## 修复（全在卡面文件域；零新增 ProjectEventHub 信号，复用既有 project_loaded/send_topology_changed；禁全量轮询，全部冷却防抖）

- `app/rack/Vit_Graph_Rack.gd`：订阅 `project_loaded`（外部变化到达信号，141 同款链）→ `track.send.list`+`list_tracks` 双只读拉取；视图构建/重建（`_apply_k3_snapshots`）同挂两拉取；list_tracks 回执缓存每轨 send 路由插件条目（JSON 签名去抖）经 `merge_send_route_entries` 并入 `load_nodes`（按 plugin_item_id 去重，repository 行优先）；外部断开清空同链移除卡片；`_vit_track_client()` 解析点同 `_vit_rack_client` 探针注入面。
- `app/rack/rack_send_route_rules.gd`：纯函数 `send_route_plugin_entries_from_track_states`（fail-open）+ `merge_send_route_entries`（探针直断面）。
- `app/kernel/clients/track_client.gd`：`list_track_plugin_states`（list_tracks no-wait 只读拉取）。
- `vit_dock/views/mixer_dock_adapter.gd`：`project_loaded`→`track_send_list` 拉取（外部前缀 request_id）→回执 normalize+repository 落库+`send_topology_changed` 广播——机架视图不在场时 SEND 槽位/轨徽标也自动刷新。
- 探针：`tools/probe_rack_sendroute_macro.gd` 206→231 断言（合同 5+纯函数 6+场景 12 外部写→机架自动呈现含幂等/清空+场景 13 mixer 落库广播去抖）；`tools/stack_probe_sendroute_macro.gd` 新 L7 腿（foreign request_id connect=agent 写同构→真实组件装配→project_loaded→生产 transport 拉取→SEND 卡零用户操作出现）。

## 呈现面三处核对（卡面②）

| 面 | 机制 | 证据 |
|---|---|---|
| 机架 send 边/SEND 卡 | auxsend 条目经 list_tracks 快照并入 load_nodes 生成 SendOut/SendIn 卡+链位 | 真栈 L7 "SendOut card auto-present (zero user action)" PASS |
| 轨 SEND 槽位（mixer strip） | 机架或 mixer adapter 拉取→repository 落库→`send_topology_changed`→strip `_refresh_send_slots` | 场景 13（adapter 落库+广播+去抖）PASS；真栈 L7 "topology auto-applied to repository" PASS |
| send 电平数值（:2014 面） | SendOut 卡 amount label/参数口走 `_send_topology` amount_db | 真栈 L7 "SendOut card amount label"（-6.0）PASS；本地场景 12 参数口注册断言 PASS |

## 验收证据

- **真栈腿（卡面验收）**：`dev_agent_smoke -StartKernel -RestartAgent -SendRouteGuiSmoke -KernelExe <K3 722A79D7 22:0x 复核> -GodotProjectRoot <前端 worktree@10324d8>` **脚本 exit 0**；探针 **33/33 PASS**（SUMMARY_JSON checks_failed=0/status=pass）——K3 六腿 L1-L6 零回退 + **L7 外部写腿 8 断言全过**（含 VSP 形态盲区复现基线 "baseline no send node (VSP-shaped repo)"=修复前状态在真栈复现，随后自动呈现）。chat/confirm 腿过（既有两条 Strip Silence warn 非验收面，与既往形态一致）。§8 记账：LLM=0（探针腿全确定性；chat 腿白名单话术非验收面）；一次过零重跑（首轮命令行 bash 吞反斜杠未起栈零副作用，正斜杠重跑即过）。
- **本地回归**：K3 探针 231/231 exit 0；9 套回归零回退（4 rack 测试+plugin_search_popup 111/111+shrink a/b/c PASS）。
- 拆栈复核：六端口 tcp+udp=0、VitApp/VitAgent/vsphub=0（脚本 finally 后残留本腿 VitApp 16432/VitAgent 12260 路径亲核后清杀，145 卡同款）；用户 Godot 编辑器 14284 全程未动。

## 事故自报（登记 c57e5f05）

21:5x 在 158 卡占用期内，本会话对 stack 探针做「干跑语法验证」误判 headless 不触栈——实际场景模式即生产传输链，对 158 在跑栈执行了完整探针腿。损害限定 158 隔离 run 副本（clear clips+新增 K3 探针轨/send/宏；158 自建 1032→1037 实测存活）；主检出/staging 零触碰。即停触碰，真栈腿等 158 释放（22:04）后按 §2.2 另行登记执行。

## 端测边界声明

- 真栈腿为 headless 场景模式：GUI 组件（ProjectEventHub/ProjectRepository/Vit_Graph_Rack 实件）在探针内装配，`project_loaded` 广播即真实 dock root 应用外部快照后的同一下游汇聚信号（141 探针先例）；**未覆盖**真实 dock root 的 delta_update→VSP 快照回拉段（该段为 141 既有链，非本卡改动面）与窗口态渲染视觉。
- 窗口态用户旅程（agent 建轨+send 后用户目检机架/混音台自动出现）建议随 M2 场次③手测顺带复核——非本卡验收门槛（卡面验收=探针腿 exit 0）。
