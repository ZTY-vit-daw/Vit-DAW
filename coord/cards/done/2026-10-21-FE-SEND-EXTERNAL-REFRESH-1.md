# FE-SEND-EXTERNAL-REFRESH-1：agent/外部写命令后 send 拓扑 GUI 不刷新（机架 send 边+SEND 槽位盲区）

- 发卡：GLM 主管决策流 / 2026-10-21 晚（依据=M2 手测场次②用户观察+内核/前端双面取证）
- 派发确认：已确认（用户手测现场确认「GUI 没有显示 send」）
- 验收负责人：GLM 主管决策流
- 池序 161；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P1（M2 主旅程可感面）/ 0.5 天 / 依赖=无；模型分级：L1/L2 / PC 前端执行流

## 现象（2026-10-21 21:02 实录，栈在卷）

agent 在 full-access 下执行 `track.send.connect`（source=guitar/1017 → auto_create Reverb Bus/1037 @-6.0dB，内核回执 `created_fx_track=true`、`source_count=1`、`sends[]` 在卷=**内核拓扑已接通**）+ `rack_add_node`（Pro-R 落 1037）。GUI：新轨 1037 与 Pro-R **可见**（141 轨道级刷新生效），但吉他轨 SEND 槽位/机架 send 边**不显示**（用户目检确认）。

## 机制（代码已核，前端仓 main=0e244f0）

- send 拓扑唯一读源=`track.send.list` 回执（`app/rack/rack_send_route_rules.gd:8-9`；`Vit_Graph_Rack.gd:2117` 注释同款）。
- 刷新触发=`_on_send_control_ipc_completed`（`Vit_Graph_Rack.gd:2204+`）按 `_SEND_REQUEST_PREFIXES`（:338）匹配 **GUI 自身 rack_client 请求的 request_id**；连接类回执另触发 `_request_project_state_refresh_single_flight()`（:2184-2186）——全部以「GUI 发过命令」为前提。
- agent 经 VSP→内核的写命令不产生 GUI request_id → 零触发。轨道级刷新（141 卡）已解决外部建轨可见，send 拓扑面缺同款处理。

## 范围

agent/外部写后 send 拓扑自动呈现：①触发面——外部变化信号（内核 delta 广播若可达 GUI 则订阅之；否则 ProjectEventHub/既有轨道级刷新链挂轻量 `track.send.list` pull；**不得全量轮询硬刷**，141 同约束）；②呈现面——机架 send 边+轨 SEND 槽位+send 电平数值显示（:2014）三处一并核对；③`stack_probe_sendroute_macro.tscn` 探针补「外部写→GUI 自动出现」腿（K3 六腿零回退）。

## 文件域

`app/rack/Vit_Graph_Rack.gd`（send 拓扑刷新触发邻域）+事件源挂接点（ProjectEventHub/track_client/mixer_dock_adapter 同款只读引用，新增信号先报）+探针件；前端 worktree+port 分支。越域即停。

## 验收标准

真栈腿：agent 侧（或注入）建 send 后 GUI **无需任何用户操作**出现 send 边+槽位+数值；手动 K3 六腿回归零回退；探针新腿 exit 0；工件入 coord/runs/FE-SEND-EXTERNAL-REFRESH-1/。端测边界声明。

## 停止条件

内核→GUI 无任何外部变化广播通道且轻量 pull 无可挂接的既有事件链（需新建轮询）→ 锚点上交裁定刷新策略。

## 诊断结论（用户已测，2026-10-21 21:4x）

重新点选吉他轨+切走切回机架视图后 send **仍不出现**——「纯外部通知缺口」假设收窄：**视图重建路径同样不拉取 send 拓扑**（刷新唯一挂点=GUI 自身命令完成）。修复面=外部变化信号 + 视图构建/重建两处都要挂 `track.send.list` 拉取；仅补通知不补重建挂点不够。判定面证据见 [M2 场次②实录](../../runs/M2-SESSION2-20261021/SESSION.md)。

- 领取：2026-10-21 晚窗 / origin/main=39326111（612beb18 之上 158/160 两卡领取提交，文件域不交）/ owner=GLM-5.3 L2 PC ZCode 执行会话（161 卡会话）/ 分支 port/fe-send-external-refresh-1（前端仓，基线 0e244f0）/ worktree D:/Godot/project-vit-daw-frontend-sendext1 / 领取提交=本卡 todo→doing 提交（领取核对：卡仍在 todo、派发已确认、依赖=无、doing 三卡[158/160/GEN-SUNO] 文件域不交）
- 回执：实现=前端仓 port/fe-send-external-refresh-1@**10324d8**（6 文件 +465/-3，全在卡域；零新增 ProjectEventHub 信号，复用既有 project_loaded/send_topology_changed；无全量轮询，全部冷却防抖）。根因比卡面深一层=双断点：①外部写零拉取触发（卡面既有判断）+②VSP-first 快照链剥每轨 plugins（内核 normaliseTrackArray 白名单），auxsend 只在 legacy plugins——视图重建无数据可生成卡。修复=rack 订阅 project_loaded 双拉取（track.send.list+list_tracks 插件条目快照）+视图构建/重建同挂+条目并入 load_nodes；mixer adapter 同链拉取落库广播（机架不在场时 SEND 槽位也刷新）。验证=K3 探针 206→**231 exit 0**+9 套回归零回退；真栈 run=**sendext1_gui_20261021_220842**（占用登记 2cc1ce1e）`dev_agent_smoke -SendRouteGuiSmoke` **exit 0 探针 33/33**——K3 六腿零回退+L7 外部写腿 8 断言全过（零用户操作自动呈现+电平数值+repository 落库+VSP 形态盲区复现基线）。端测边界：真栈腿为 headless 组件装配（project_loaded=真实 dock root 下游汇聚信号，141 先例），未覆盖窗口态渲染视觉与 delta→VSP 回拉段（141 既有链非本卡面）；窗口态建议随 M2 场次③手测顺带复核。事故自报在卷（21:5x 干跑触碰 158 占用栈，损害限其隔离 run 副本，登记 c57e5f05）。工件=主仓 coord/runs/FE-SEND-EXTERNAL-REFRESH-1/（随 port/fe-send-external-refresh-1-stack@86e854ba 入库待验收合入；RESULTS.md 含全部证据与边界）。
- 验收：2026-10-21 深夜 / GLM 主管决策流 / **整卡 pass**：前端 10324d8 合入前端 main（merge c5cac57 已推）；工件 86e854ba cherry-pick 入主仓 main（5b3574f1）；根因双断点采信（外部写零拉取+内核 normaliseTrackArray 白名单剥每轨 plugins 致视图重建无数据——比卡面深一层，VSP 形态盲区基线在真栈复现为证）；约束核对=零新增 ProjectEventHub 信号+零轮询（全冷却防抖）；真栈 run sendext1_gui_20261021_220842 亲核 SUMMARY_JSON 33/33 checks_failed=0——K3 六腿零回退+L7 八断言含零操作自动呈现/-6.0 电平/repository 行；K3 探针 231+9 套回归零回退；端测边界采信（headless 组件装配=141 先例；窗口态目检随 M2 场次③顺带复核，非门槛）；**事故裁定**：21:5x 未登记触碰 158 占用栈=执行纪律违规（自报 c57e5f05 在卷，134/139 同类先例）——损害限 158 隔离 run 副本且 158 判定面经时戳自证无扰动，产品证据零损；免返工，记流程警示（任何探针跑前核对栈占用登记，§2.2 既有义务）；验收 commit=本提交
