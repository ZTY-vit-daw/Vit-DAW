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

- 领取：2026-10-22 早窗 / 主仓 origin/main=1ae57f23（前端仓基线 origin/main=c5cac57，161 已合入）/ owner=GLM-5.3 L2 PC ZCode 执行会话（164 卡会话）/ 分支=port/fe-send-cable-external-1（前端仓）/ worktree=D:/Godot/project/project-vit-daw-frontend-cableext / 领取提交=a8dfe20b+哈希回填 184b119b（重放于 162/165/163 三笔领取之上，本卡零冲突）。**事故记录**：bee74f6f（162 占用登记，2026-10-22）从过期工作树构建提交，FF 推送把本卡 doing 移动与领取栏静默回退到 todo 模板态（父链含 184b119b 仍回退=stale-tree overwrite，非并行竞态）；本提交一并修复卡态（todo→done）——交决策侧知悉（协议 §2.1.3 只拦非 FF，FF 内容回退无检测）
- 回执：实现 commit=**8fb415c** @ port/fe-send-cable-external-1（前端仓 worktree D:/Godot/project/project-vit-daw-frontend-cableext，基线 c5cac57）；工件+RESULTS=coord/runs/FE-SEND-CABLE-EXTERNAL-1/ 随主仓 **port/fe-send-cable-external-1-stack@fb251fb7**（86e854ba=161 stack 分支基底——SendRouteGuiSmoke ps1 复用，决策侧合入序=161 stack 先行）；真栈 run=**cableext_gui_20261022_1028**（脚本 exit 0，探针 **41/41**，占用登记 2699d1d2→释放 3acede67，错峰序=162 chat 腿释放后）。探针断言面：K3 探针 231→247（纯函数 7：相邻对/形状/非相邻/FX 乱序/尾部悬空/畸形输入/同 id + 场景 12.5 九断言：卡+电缆同现+过确认门+确认集 key+无双轨重复卡+拖断改道 track.send.disconnect 且 rack.disconnect_pins 零泄漏+清空回执电缆/确认集同链消失）+回归零回退（popup 111/111、shrink a/b/c、start_feed 244/244）；真栈 L8 八断言全过（foreign id connect→卡+电缆零操作同现→确认门保持→恰一卡→外部 disconnect→同链消失+确认集回零；2s 快照冷却按生产纪律等待）。**锚点结论（卡①）**：内核回执族无任何一份携带显式 auxsend→auxreturn 边（rack 快照 createRackState 只枚举 rack 实例轨的 rack DAG；aux 对住 track pluginList）——停止条件未触发；内核确认事实=SendRouteService::ensureSourceSendPair 成对插入源轨 pluginList（SendIn 紧邻 SendOut）+list_tracks 保序回执，电缆由该事实派生（aux_pair_edges_from_plugin_entries）入同一 sync_edges_from_server 确认集，防幽灵线门零放松。端测边界声明：headless 断言到 DOM/命令面（连接表+确认集+卡节点），**渲染像素（电缆视觉）归用户手测**（入口=Godot 前端，既有裁定；场次④清单可加「agent 建 send→卡与电缆同时出现」）；GUI 拖缆**新建** send 路径未覆盖（现行入口=SEND 槽位/目标选择器/空白右键）；FX 轨画布内部 receiver→rack→wet 湿链连线未派生（非 rack 成员且乱序，属 166 合一呈现卡域）；167（FE-MIXER-FX-STRIP-AUDIT-1）在同探针文件有 231→239 分支待合入——本卡 247 增量与其无函数/块交叠，合并时注意上下文冲突。§8：LLM=0（chat 腿白名单话术非验收面）；一次过零重跑。
- 验收：（裁定文件 / 验收 commit）
