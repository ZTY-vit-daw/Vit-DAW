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

- 领取：（时间 / origin hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 真栈 run ID / 回归工件 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
