# FE-SEND-CHAIN-POSITION-1：send 对卡与源轨插件链的链位连线（场次⑤手测裁定=需要）

- 发卡：GLM 主管决策流 / 2026-10-23 凌晨（依据=场次⑤用户手测反馈：「send 卡之间的确有连线，但 rack in 和 rack out 之间没有连线——机架 GUI 的显示逻辑是加载插件后它会自动连接到合适的位置，然后两端连线，现在 send 卡依然是悬空的」；164/166 端测边界此前已如实声明该面未做待裁定）
- 派发确认：已确认（用户手测现场反馈=需要）
- 验收负责人：GLM 主管决策流
- 池序 176；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；P1（主旅程可感面） / 0.5 天 / 依赖=无；模型分级：L1/L2 / PC 前端执行流

## 现状与缺口（既有声明在卷）

- **已有**：SendOut↔SendIn 对卡之间的信号电缆（164 派生边）；合一 icon 折叠呈现（166）；普通插件装载后自动连接到源轨插件链的合适位置并与上下游两端连线（机架既有装载逻辑——用户描述的预期行为）。
- **缺**：send 对卡/合一 icon 在**源轨插件链上的链位连线**——SendOut 应像普通插件一样接入源轨链的 tap 位置（上下游两端连线），当前悬空孤立。数据源在：pluginList 相邻对（164 已证内核保序回执，SendOut/SendIn 的 pluginIndexOf 即链位事实）。
- 邻接未做面（166 声明）：FX 轨画布内部湿链（receiver→rack→wet）——本卡一并勘察其数据源可派生性，可做则做、不可做如实上交另拆。

## 范围

①取证：普通插件的自动连线下游（装载后接入链位+两端连线的既有机制）与 send 对卡为何不走该机制（send 对卡非 rack 成员=164 已证，链位连线需从 pluginList 序派生）；②实现：send 对卡/展开对卡与源轨链的链位连线（相邻插件间连线派生，同 164 派生边纪律——内核确认事实派生+确认集+防幽灵线门零放松）；③折叠态合一 icon 的链位表达（两端连线入折叠形态或入口提示保持）；④探针断言（外部写后链位连线零操作出现+断开消失+防幽灵线门保持）；⑤窗口态视觉=用户手测目检项。

## 文件域

以勘察固定（预期 `app/rack/Vit_Graph_Rack.gd` 边集/连线派生邻域+`rack_send_route_rules.gd` 纯函数+探针件）。越域即停。

## 验收标准

探针+真栈腿（外部写后 send 对卡+对间电缆+**链位连线**三层零操作同现）；K3/161/164/166 既有探针零回退（合并态基线 289）；端测边界声明；工件入主仓 coord/runs/FE-SEND-CHAIN-POSITION-1/。

## 停止条件

链位连线与 GraphEdit 布局语义冲突（连线拓扑不可表达）→ 锚点上交再裁定。

- 领取：2026-10-23 09:13 本会话机器钟 / origin/main=8cc1e5e90cfedc2abcbef083694daa5739fe817c（领取前 fetch 亲核=当前 main HEAD） / owner=GLM-5.3 L2 PC 前端执行会话（ZCode，176/178 卡会话，机器=PC） / 分支=port/fe-send-chain-position-1 / worktree=D:/Godot/project/vit-daw-frontend-sendchain1（前端仓领取基线 eee2fcd=本地 main HEAD；远端 origin/main=c5cac577 落后本地 6 个已合入 commit 主管合入未推送——按卡面基线 eee2fcd 开工，如实记录差量；协调分支 coord/fe-send-chain-position-1-claim@D:/Vit_DAW_wt_sendchain1_coord） / 领取提交=本 commit
- 回执：**2026-10-23 10:1x 本会话机器钟执行侧自验完成，done 待主管验收**
  - **commit**：前端 port/fe-send-chain-position-1@**7c82260**（4 文件 +314/-9：rack_send_route_rules.gd 纯函数/Vit_Graph_Rack.gd 派生并入+合一端口+手势门/两探针件）；主仓工件 port/fe-send-chain-position-1-stack@**27297279**（run 目录）。
  - **实现面**：①`RackSendRouteRules.aux_chain_edges_from_plugin_entries` 纯函数——相邻 auxsend→auxreturn 对（与 164 同一相邻判定）派生上下游两端连线：上游=链上前一可见插件（无=RACK_INPUT 总线锚，与内核边序列化同构；系统隐藏 volume/level/meter 邻居跳过），下游=后一可见（无=RACK_OUTPUT）；折叠态端点=`sendpair:<out_item>` 代键（合一 icon plugin_id，邻居为折叠对成员时同样折叠——跨对链位 sendpair:→sendpair: 连续）；非相邻/同 id/畸形 fail-closed 零产边。②`_rack_server_edges_for_bound_track` 与 164 电缆同源并入同一 `sync_edges_from_server` 确认集（防幽灵线门零放松；折叠端点集读当前画布合一 icon 态=无独立状态源）。③合一 icon 补音频端口=链位呈现挂接面；`_on_connection_request` 对合一端点拒绝（166 红线 2 保持=操作面仍展开态）；send 端点边删除统一分派 `_dispatch_send_endpoint_edge_removal`：同对电缆（SendOut→SendIn 同对）仍改道 track.send.disconnect（键盘删除路径补齐同语义）、链位边/跨对边/合一端点边=派生呈现拒绝（线保持零发包）。
  - **取证结论（范围①）**：普通插件自动连链=内核 `rack_add_node` 默认 `auto_connect=true`+`vitTryChainNewRackPluginSerial`（PluginRackControlService.cpp:3420-3465，串尾拆 RACK_OUTPUT 边改道穿新插件）；send 对经 `ensureSourceSendPair` 直接插 pluginList（SendRouteService.cpp:498，插入位=sourcePairInsertIndex=内容插件后监听尾巴前）绕过 DAG 边——悬空根因。内核边序列化总线端点=RACK_INPUT/RACK_OUTPUT（CommandDispatcher.cpp:1877-1880），前端 `_vit_kernel_id_for_graph_node` 既有解析直接复用。
  - **附带勘察（FX 轨画布内部湿链）——数据源可派生但画布表达需裁定，另拆**：内核 `normaliseFxChain` 反环门 2（SendRouteService.cpp:365-408）保证 FX 轨 pluginList 形态=receiver(AuxReturn send_bus)@头 + rack 成员 + wetSend(AuxSend return_bus)@rack 后（每次 rack 变更后规整）——receiver→rack→wet 拓扑可从同源 pluginList+rack 成员序派生；**但**画布表达与既有锚点拓扑门冲突：湿链形态=receiver→RACK_IN 锚→FX→RACK_OUT 锚→wetSend，而 `RackTopologyRules.can_connect_rack_topology` 既有门禁 in 锚为目的端（to_anchor=="in" 拒）与 out 锚为源端（from_anchor=="out" 拒，防用户画环）——需 UX 层呈现形态裁定（新锚点语义或徽标化呈现），不建议本卡顺手扩门。上交决策侧拆卡。
  - **探针断言面**：K3 `tools/probe_rack_sendroute_macro.gd` **289→309**（+20 chain176：纯函数 10=双对上下游/折叠代键端点/半折叠邻居折叠/系统隐藏跳过/中链 prev-next/非相邻/侧链孤 auxsend/畸形/同 id；场景 10=折叠态链位键入确认集+锚到锚画布连线在场+内核确认门/合一端点手势连线拒绝零发包/展开态键替换+连线/链位边拖断拒绝线保持零 IPC/跨对拖断不误删路由/清空回执链位键消失）；popup **111/111**、shrink a/b/c PASS、start_feed **244/244** 零回退。
  - **真栈 run**：**chainpos_gui_20261023_0957**（dev_agent_smoke -StartKernel -RestartAgent -SendRouteGuiSmoke，kernel=主检出 K3 722A79D7 只读，被测=worktree@7c82260）**脚本 exit 0，stack probe 56/56**（SUMMARY_JSON checks_failed=0；166 基线 51+L9 腿 5 断言：折叠态链位键入确认集+锚到锚连线画布在场/展开态链位键+连线/外部断开链位键同链消失）。占用登记 100eb072（09:5x）→拆栈复核进程=0、5555/5556/4445/7878/8787/18787 tcp+udp=0。
  - **端测边界声明**：headless/真栈断言到画布连接表/确认集/节点 meta/命令 payload 与零发包计数，**不覆盖渲染像素**——链位连线视觉观感（锚到锚走线形态/折叠 icon 带端口的视觉呈现/线色区分）与窗口态交互手感=**用户手测目检项**（入口=Godot 前端拉起，建议目检点：源轨带 FX 插件+send 对时链位两端连线、折叠/展开切换连线跟随、拖拽链位线被拒的反馈感）。§8：本卡断言面 LLM=0（脚本自带 chat/confirm 冒测面 1 次 deepseek-flash 调用，非本卡断言参与，如实注记）。
- 验收：**整卡通过**（GLM 主管决策流 / 2026-10-23 晚窗）。diff 亲读红线全守（纯函数独立新增+164 既有 aux_pair_edges 零触碰；链位边与电缆同源并入同一确认集=防幽灵线门零放松；合一节点仍零 kernel_plugin_item_id；合一端点手势连线拒绝+删除统一分派——同对电缆 track.send.disconnect 改道保持、链位边/跨对边拒绝零发包）。真栈工件亲核（27297279 SUMMARY_JSON checks_failed=0 checks_total=56）；主管复跑合并态 K3 309/popup 111/shrink a·b·c/start_feed 244 全绿 exit 0。取证结论采纳（悬空根因=ensureSourceSendPair 插 pluginList 绕过 DAG 边，内核锚点在卷）。FX 轨湿链勘察上交=数据源可派生（normaliseFxChain 反环门 2 规整形态）但画布表达与 can_connect_rack_topology 锚点门冲突——另拆卡待用户裁定呈现形态（gate 风险位在案）。验收 commit=前端 787150f+主仓 b091f573+本收口。
