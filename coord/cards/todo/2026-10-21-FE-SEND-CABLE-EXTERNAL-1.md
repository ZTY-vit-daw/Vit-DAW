# FE-SEND-CABLE-EXTERNAL-1：外部（agent）建 aux 对后机架卡在但信号电缆缺失

- 发卡：GLM 主管决策流 / 2026-10-21 深夜（依据=M2 场次③用户手测：agent 建 piano→Reverb Bus 后，源轨 SEND ▸ 与 FX 轨 ◂ SEND 卡渲染、中间无信号实线；GUI 重复接线得「已接线」警告+听感验证=后端通、纯显示缺线）
- 派发确认：已确认（用户手测现场反馈）
- 验收负责人：GLM 主管决策流
- 池序 164；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P1（主旅程可感面）/ 0.5 天 / 依赖=无；模型分级：L1/L2 / PC 前端执行流

## 现象（场次③实录）

agent 经内核 `track.send.connect`（piano/1027→Reverb Bus/1037 @-6.0，内核回执 sends[] 在卷）后：SEND ▸ / ◂ SEND **卡对渲染正常**（161 生效），但两卡之间的**信号实线（电缆）不画**。用户经 GUI 再次拖接同一边 → 内核「已接线」警告（正确拒绝重复）+听感确认信号通——**边在内核存在、GUI 电缆层缺失**。

## 机制（代码已核，卡与线双链分离）

- 卡=插件条目链（161 修复面：list_tracks 拉取→merge_send_route_entries→load_nodes）。
- 线=`_draw()` 只画 GraphEdit connections 且逐条过 `_rack_graph_connection_is_kernel_confirmed_fast`（防幽灵线：边必须在内核确认边集）；auxsend→auxreturn 的连接应在边同步链（`_vit_edge_sync`/rack DAG 状态）建立。**外部写后该链未被喂到/未刷新**——疑似 161 根因②同族（VSP 快照剥插件族数据剥掉了 aux 边），161 只修了卡面。

## 范围

①取证锚定：外部建 send 后内核 rack DAG/边回执里 aux 边的可用面（list_tracks 回执/边同步数据源里哪一份含 auxsend→auxreturn 边）；②外部变化信号（project_loaded/141 链）挂**边同步/重建**；③真·重复卡核对：确认无双轨同插件重复卡（merge 去重 plugin_item_id 覆盖 GUI 侧建的卡），有则一并修；④探针补「外部写→电缆出现」腿（K3 231+161 L7 之上加边断言）。**修复须保持防幽灵线门**：外部边也走内核确认集，不得为画线放松确认语义。

## 文件域

`app/rack/Vit_Graph_Rack.gd`（边同步/确认集邻域）+`app/rack/rack_send_route_rules.gd`（如需边派生纯函数）+探针件；前端 worktree+port 分支。越域即停。

## 验收标准

真栈腿：agent 侧（或注入）建 send 后 GUI **卡+电缆同时零操作出现**；断开同链消失；K3/161 既有探针零回退；新腿 exit 0；工件入主仓 coord/runs/FE-SEND-CABLE-EXTERNAL-1/。端测边界声明。

## 停止条件

内核回执族里找不到携带 aux 边的只读数据源（需内核新命令）→ 锚点上交（内核面另卡）。

- 领取：（时间 / origin hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / 真栈 run ID / 探针断言面 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
