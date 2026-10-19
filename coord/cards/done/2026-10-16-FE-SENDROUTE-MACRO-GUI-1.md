# FE-SENDROUTE-MACRO-GUI-1（K3）：send 节点+宏控制层前端实现（一次落地）

- 发卡：GLM 主管决策侧 / 2026-10-16 晚窗（依据=设计件 §9 定稿+用户「一起执行，并不着急」——**Vit_Graph_Rack.gd 同文件一轮改净，不与宏升级拆两轮**）
- 派发确认：**已确认（用户 2026-10-16 同上）**；验收负责人：GLM 主管决策流
- 池序 130；目标仓库=Godot 前端仓（D:\Godot\project\vit-daw-frontend）；P1 / 3-4 天 / 依赖 **已满足**（K1/127 合入 f4926543 + 129 定稿 2026-10-18 深夜）——**解锁可领取（2026-10-18 深夜）**；基线=frontend main 最新（含 R2/R3.1 启动页线）
- 模型分级：L2 / GLM 执行会话（前端线，PC/Mac 皆可——领取时与 Mac 线核对前端仓文件域）

## 规格（权威=设计件 §9 + 129 定稿两份设计稿；宏部分已回填 2026-10-18 深夜）

1. **send 节点渲染**：SendOut（穿通+副本岔出）/SendIn（湿声注入）串联节点——与效果器同构的机架图成员（v4 样图定稿形态：SEND ▸ / ◂ SEND tag、目标名、量、拖排序换位=改 tap/汇入点、右键成对移除、双击跳 FX 轨画布）；**源轨画布无跨轨实体 icon**；
2. **创建流程**：机架右键/菜单「新建发送」→目标选择器（已有轨分组+搜索/每轨侧链口展开/「＋新建 FX 轨」现场建轨命名默认 Reverb Bus 2 类）→选定即连（乐观 pending 边→内核确认回读，rack_pending_edge_rules 回滚模式）；
3. **控制面**：通道条 SEND 槽位（FX 名+量+通/断开关，主控面）；FX 轨画布顶部只读来源列表（点击跳源轨）；
4. **宏控制层**（129 定稿回填，权威=coord/runs/FE-RACK-MACRO-LAYER-DESIGN-1/{01,02}-design-draft）：
   - **两层模型**：信号实线/控制紫虚线（#a78bfa 系）；层 chip「●信号 ◆控制」**默认双显**，单显控制=信号压淡 15%、单显信号=控制隐藏，切层只改渲染；控制节点不占信号链、拓扑门不放行控制连线（绑定走独立手势通道）；
   - **rack_client 封装 control_* 九写命令**（缺口 G1）+宏节点数据源从 repository 文档切换为 control_graph 快照渲染；
   - **控件渲染器组件族**：第一批七静态类完整交互（knob/slider/numberbox/xypad/toggle/button/range——内核 kind 词表），时变三类（envelope/lfo/stepsequencer）渲染壳先行（无绑定交互，标注后续卡）；**混排宏卡**（一卡多控件+N 映射 badge+卡级/控件级输出锚）；xypad/range **双输出双绑定**（binding.source_output=out_x/out_y）；
   - **绑定手势**：输出锚拖出紫虚线→参数口高亮→绑定菜单（curve 四选 linear/invert/ease_in/ease_out+min/max）→control_add_binding；绑定把手点击=编辑卡；右键线=断开；
   - **参数口**：效果器卡+SendOut 卡（数据源=参数描述符 binding_target 投影；SendOut 发送量口显示走 track.send.list amount_db，绑定目标=target_param_id="send level"，寻径 getAutomatableParameterByID 原生；发现面二选一=前端硬接线 or 内核补 aux 描述符小披露，领取时报决策侧定）；
   - **宏值施加**=control_set_macro_values（批量）；**侧面板 macro_panel.gd 渲染器扩十词表**（switch/checkbox/trigger 别名兼容），施加统一同命令面；**类型建议规则表**（参数特征→建议 kind，前端确定性表+可下拉改）；
   - 启用 EFFECT_RETURN 枚举+轨列表角色徽标；
5. **命令面**：rack_client 封装 5 命令（track.send.*）+快照面扩展 send 边行（get_rack_edges_for_track 同构缺口 d）。

## 文件域

前端仓 app/rack/{Vit_Graph_Rack.gd 等}、app/shell/side_panel、rack_client.gd、vit_daw_track_kind.gd、rack_ipc_payload_rules.gd；GH001 推送限制照旧（补丁/清单存档主仓 run 目录）。

## 验收标准

- 渲染面烟测（FE 系列脚本模式扩展）：send 节点渲染/选择器流程/SEND 槽位/宏控制层 DOM 级断言；
- 真栈（K1/K2 已合入）：GUI 建发送→track.send.list 读回一致→调量/换位/断开回读；
- **[等待用户] 手测**：Godot 拉起→进工程→机架新建发送→听感干湿（手测入口裁定：从 Godot 拉起，环境变量同 shell 设置）；
- 前端仓测试（tests/test_project_repository_rack_macros.gd 等）零回退。

## 停止条件

- 129 宏设计未定稿不得领取本卡 send+宏合并实现（可申请拆 send 先行的例外，须用户单点确认）；
- 快照面/IPC payload 扩展与内核 schema 偏差 → 停手对齐 K1。

- 领取：（2026-10-18 晚窗 / 前端仓 origin/main=4bdbf51 + 主仓 origin/main=611305f5（领取提交前再核）/ owner=GLM-5.3 L2 PC ZCode 执行会话（K3/130）/ 分支 port/fe-sendroute-macro-gui-1 / worktree D:/Godot/project-vit-daw-frontend-sendmacro / 领取提交=本提交）
- **领取时决策点上报（卡面 §规格 4 参数口）**：① aux send level 发现面选 **前端硬接线**——SendOut 卡发送量口数据源=track.send.list amount_db，绑定走 target_kind=plugin_param + target_plugin_id=<SendOut 插件实例> + target_param_id="send level"（129 设计件 v1 §1.3 亲核寻径 getAutomatableParameterByID 原生可用）；理由=K1 已合入无需内核改动（内核域 126 在飞）、设计件明示两选项非设计阻塞；报主管复核，若裁定内核补 aux 描述符另拆小卡。② 文件域核对（2026-10-18 晚窗亲核 doing 三卡）：126=VitApp/Source/Service（内核仓）、131=coord/runs 设计件（零生产代码）、138=agent/internal/{history,chat}——前端仓零在飞卡，本卡文件域无冲突。本卡 SEND 槽位落 vit_dock/views/mixer/vit_mixer_channel_strip.gd、轨列表徽标落 app/tracks/track_scene/Track_Row_Controller.gd（卡面 §规格 3/4「通道条 SEND 槽位」「轨列表角色徽标」的文件域落点，属卡面「等」域内）。
- 回执：（**done 2026-10-18 深夜，执行侧自验完成待主管验收**）
  - **前端实现 commits**（port/fe-sendroute-macro-gui-1@worktree D:/Godot/project-vit-daw-frontend-sendmacro，基线 4bdbf51，GH001 照旧不推远端——补丁存档主仓 run 目录 patches/0001-0003）：`31e4f0e` 主实现（send 节点+创建选择器+SEND 槽位+FX 来源列表+FX 徽标+两层模型/控制层/控件族/绑定手势/参数口/建议表/侧面板十词表+快照面）+`8cc6787`+`6404a52` 真栈探针。新文件：rack_send_route_rules/rack_send_target_selector/rack_control_layer_rules/rack_control_widget_factory+两探针；改：Vit_Graph_Rack（+1751 行）/rack_client（14 命令封装=缺口 G1）/rack_ipc_payload_rules（17 构造器）/project_repository+event_hub（send_topology+control_graph 快照面=缺口 d/h 落点，领取时已登记域内）/vit_mixer_channel_strip/Track_Row_Controller/macro_panel/probe_rack_plugin_search_popup（入口断言随三入口菜单更新，111/111 保持）。
  - **主仓 commits**（port/fe-sendroute-macro-stack@worktree D:/Vit_DAW_wt_fesendmacro_stack）：`1c4d3fec`+`42fbf110` dev_agent_smoke 新腿 -SendRouteGuiSmoke；`8314841e`+`7a735344` 工件+RESULTS+手测清单+前端补丁存档。
  - **run ID**：run1=sendroute_gui_20261018_2250（dev_agent_smoke -StartKernel -RestartAgent -SendRouteGuiSmoke **exit 0**，真栈探针 **25/25 PASS**：connect auto_create 回执拓扑/list 读回一致/state aux 插件对/set_amount 双回读/宏三步+control_graph 快照+send dB 联动（实测 +1.5371=fader 换算 20·ln(0.8)+6 公式亲核）/disconnect 归零+fx_released——GUI 同源 payload 经生产传输链 VitIpcClient UDP 4445→agent bridge→内核 ZMQ）；run2=window_boot_20261018_2258（-StartUI -MixSmoke **exit 0**+窗口态启动证据 D3D12 RTX4070/零 SCRIPT ERROR）。被测 kernel=本腿自建 main 基线 Release SHA256=722A79D7A3F53797F5713209D812D8CE9F2888CB2A7E743286FA56B99629DFEC（stock B6565DCF 不含 K1 不可用）。渲染面探针 `tools/probe_rack_sendroute_macro.gd` 三段式 **151/151 exit 0**（契约 66+纯函数+场景 DOM/命令面）；前端仓既有 **9 套测试零回退**。栈登记 cd23b1c0/释放 c28f3c19（拆栈复核净，§8 零 LLM，三轮探针自缺陷迭代如实记账于 RESULTS）。
  - **端测边界声明**（AGENTS §5/§7）：已覆盖=GUI 命令构造面×生产传输链×内核真实落链回读+headless DOM 形态+窗口态启动；**未覆盖**=窗口内真实指针交互（选择器点击/绑定拖拽/旋钮转动）与听感干湿——归 **[等待用户] 手测**（清单=coord/runs/FE-SENDROUTE-MACRO-GUI-1/MANUAL_TEST_CHECKLIST.md，按 Godot 拉起入口书写）。「拖排序换位」无内核命令面（move_plugin 已在 DAG rack 模式废弃，亲核拒绝路径）——send 链位=连线语义与效果器同构，如实登记非缺陷。
  - 决策点①（aux send level 发现面=前端硬接线）真栈验证通过：binding target_param_id="send level"+target_plugin_id=<SendOut 实例> 寻径原生可用（L5 腿），amount 数值显示走 track.send.list——**报主管复核确认或另拆内核描述符小卡**。
- 验收：（裁定文件 / 验收 commit）
- 回执（2026-10-18 深夜，K3 执行会话，验收侧补记）：实现 commit **31e4f0e**（20 文件 +4605/-56：Vit_Graph_Rack +1751/rack_send_route_rules/rack_send_target_selector/rack_control_layer_rules/rack_control_widget_factory/rack_ipc_payload_rules/rack_client control_* 封装/macro_panel 十词表复用 factory/Track_Row 徽标/mixer SEND 槽位/repository send_topology+control_graph 存取/event_hub 双信号）+探针 commit **8cc6787**（probe_rack_sendroute_macro 745 行三段式 151 断言+stack_probe_sendroute_macro 场景六腿 autoload 生产传输链）+真栈修正 commit **6404a52**（fader dB 期望式/FX 名时间戳/残留隔离）。分支 port/fe-sendroute-macro-gui-1@6404a52 **未推 origin**；主仓回执原未回填（本段=验收侧代记，流程瑕疵登记）。决策点①上报=aux 发现面前端硬接线（send level 参数口+amount_db 数据源）。
- 验收（2026-10-18 深夜 GLM 主管决策流·第一次）：**部分采信——实现面+headless 面 pass，真栈面证据不足退回补证**。①实现面=diff 抽查锚点全在（control_* 封装/层规则紫 a78bfa+双显+压淡 0.85/kind 归一+静态类与壳判定+xypad 双输出 out_x/out_y/绑定手势独立通道 _binding_drag 非 connection:1635/SendOut 硬接线 send level 口:2011/侧面板复用 RackControlWidgetFactory 同组件族=129 §4 双面架构正解）；②headless 探针=决策侧独立复跑 **PROBE PASS checks_total=151 exit 0**；③诚实登记采信=拖排序换位降级为连线语义（内核 move_plugin 已废弃无换位命令面=域外事实；后续若需拖换位须内核补命令另卡）；决策点①（前端硬接线）=**复核通过**（与 129 定稿一致）。**④真栈面=不采信**：提交正文自述「25/25 PASS（活栈手跑）」但零 run 工件（主仓/worktree/内核日志均无 22:1x 痕迹）、零栈占用登记（PC-RUNTIME-STACK 最新=21:36 的 138 释放）、零被测 SHA 记录——按 AGENTS §9 回执须可回指原始工件，本轮真栈证据链缺失。**补证要求**：栈登记后以 8cc6787 已建 stack_probe_sendroute_macro 场景标准包装重跑一轮（run 目录工件+SUMMARY_JSON+被测 SHA+六腿全绿 exit 0），并将分支推 origin+补回执。补证过+手测清单后终验。
- 验收（终审修正·2026-10-18 深夜 GLM 主管决策流）：**一审「真栈零工件零登记」结论撤回——证据在构建 worktree（D:/Vit_DAW_wt_fesendmacro_stack，占用登记 cd23b1c0 内披露），验收侧搜索时机早于执行侧推送所致。真栈面采信**：占用/释放登记时间线核正（21:5x 预检 GO 先于跑，c28f3c19 拆栈复核净）；run 工件亲核=sendroute_gui_20261018_2250 **SUMMARY_JSON checks_total=25 checks_failed=0 status=pass**（L5 宏×send 联动核心断言全绿：add_macro→add_binding target=send level→set_macro_values→macro drives send amount 实测 +1.5371dB=20·ln(0.8)+6→control_graph 快照；L6 disconnect 归零）+window_boot_20261018_2258 窗口态证据；被测 kernel SHA 722A79D7（main 基线含 K1）+§8 零 LLM。**K3 内部验收=pass**（实现 diff 锚点全在+headless 151 断言独立复跑 exit 0+真栈 25/25 工件亲核）；诚实登记采信=拖排序换位降级连线语义（内核 move_plugin 废弃，域外，后续若需另卡）；决策点①（前端硬接线 send level 口）复核通过。验收侧补救三件：工件/补丁/手测清单自构建 worktree 补入主仓 main；栈文件占用区顺序修正（执行侧 23:1x 收口记录保留）；一审裁定撤回如实登记。**终验=用户手测**（[MANUAL_TEST_CHECKLIST.md](../../runs/FE-SENDROUTE-MACRO-GUI-1/MANUAL_TEST_CHECKLIST.md)：A send 创建与听感 7 步/B SEND 槽位/C 宏控制层；入口=Godot 拉起，被测=worktree D:/Godot/project-vit-daw-frontend-sendmacro——注意内核须含 K1 非 stock）；手测过后前端合 main（补丁三件在 patches/ 已存档，GH001 限制下走补丁/清单）。
- **K3 用户手测（2026-10-18 深夜）：两缺陷+一架构指正，卡重开 doing 立 K3.1 返工**：
  ①**空白右键菜单位置 bug**（稳定出现在屏幕左侧固定位置非鼠标处）——根因钉死：`_gui_input` 空白右键分支把 GraphEdit **局部坐标**直接当 viewport 坐标传入（Vit_Graph_Rack.gd:1037 `_rack_begin_right_pan(mb.position, null, mb.position)`；对照节点右键 :6478 走 `_node_context_menu_viewport_position` 正确换算）→ `_open_canvas_action_menu(popup_on_parent(viewport 坐标))` 收到局部坐标=系统性偏左固定量。修法=空白分支坐标换算对齐 :6478 先例（local→viewport），一处改。
  ②**新建 send 点击后无视觉变化**——根因=①坐标错误传导：`_canvas_action_menu_pos`（错坐标）作为 `local_pos` 传 `_open_send_target_selector` → selector `position=clamp(错误坐标)`（rack_send_target_selector.gd:88 open_at）弹在画布左缘/被裁剪=看不见。修①后复验 selector 弹出可见性；如仍有独立症状按现场再钉。
  ③**架构指正（用户裁定方向）：宏调取第一入口=机架左侧工具栏**——现状该栏「工具」页宏控件区为旧链路（graph_rack_dock_adapter.gd:15-23 CONTROL_TYPE_CHOICES 仅 slider/knob/toggle 三选+「创建宏」走 repository 宏文档 `_create_macro_for_track`），与 K3 新 control_graph 链路两套并存。返工=侧栏宏控件区升级为新链路第一入口：创建宏走 control_add_macro、控件类型选择复用 rack_control_layer_rules 十词表+suggest_control_kind 建议、增加「调取已有控制」列表（control_graph 快照读侧，点击定位/施加）；画布右键「新建宏」保留为快捷路径（同一命令面）。旧 repository 文档链路降级移除或仅留兼容读。
- **K3.1 范围**：修①（坐标一处）+复验②+③侧栏入口统一（vit_dock/views/graph_rack_dock_adapter.gd 宏控件区+Vit_Graph_Rack 坐标行）。验收=探针补钉（空白右键菜单坐标=鼠标 viewport 坐标 ±2px/selector 弹出位置可见/侧栏创建宏走 control_add_macro 命令断言）+回归电池+真栈 SendRouteGuiSmoke 复跑 exit 0+用户手测复测三问题。分支续 port/fe-sendroute-macro-gui-1。
- 回执（**2026-10-19 K3.1 done，执行侧自验完成待主管验收**）：
  - **前端实现 commit**：`1e33284`（基线 6404a52，+436/-123 三文件——Vit_Graph_Rack.gd/graph_rack_dock_adapter.gd/probe_rack_sendroute_macro.gd；GH001 照旧不推远端，补丁存主仓 run 目录 patches/0001-fix-…-K3.1）。
  - **①坐标修复**（根因运行时取证定谳，工件 k31_forensic/ 两轮日志+README）：Godot 4.6.1 `popup_on_parent` 实测把 rect.position 当**主窗口 viewport 坐标**（菜单屏幕=主窗口原点+arg 原样，parent=GraphEdit 全局换算未发生）→ :1037 局部坐标直传=系统性偏左 GraphEdit 全局偏移量（与用户症状精确吻合）；修=第三参 `get_global_transform() * mb.position`（local→viewport，对齐 :6478 先例式）+`_open_canvas_action_menu` 双轨（popup 吃 viewport，下游三入口锚点 affine_inverse 存回局部系——`_create_macro_at`/plugin 弹窗/selector 语义不变）。窗口态修后复跑：**菜单屏幕位置=鼠标 viewport 坐标 ±0px**。节点右键一直正确的机理同证（优先走内嵌 Panel 吃局部系）；`_open_node_context_menu` 的 PopupMenu fallback 分支同口径不匹配（embedded 常驻不触发，域外未动，登记备查）。
  - **②复验**：selector `open_at` 与 plugin_popup（手测过的先例）完全同模式（普通子 Panel 吃局部系），确认纯传导症状无独立缺陷；探针钉 selector 经画布动作菜单打开+position 在 GraphEdit 矩形内+近锚点 ±16px 三断言全绿。
  - **③侧栏第一入口**：Vit_Graph_Rack 新六桥方法（vit_sidebar_create_macro=control_add_macro/vit_sidebar_create_control=control_add_node/vit_sidebar_control_nodes=control_graph 快照读侧/vit_sidebar_focus_control_node=定位选中滚动/vit_sidebar_reapply_control_node=宏卡 set_macro_values 批量+单控件 set_node_value/vit_sidebar_suggest_control_kind=建议表）；adapter 工具页=十词表 OptionButton（中英标签）+建议行+创建宏/创建控件卡+调取已有控制列表（点击定位/施加）+control_graph_changed 监听；旧链路 CONTROL_TYPE_CHOICES 三选/create_rack_control_macro 写路径移除，repository/command_service 宏文档层保留（dock_root 2 读侧+参数面绑定宏+回归测试在消费=「仅留兼容读」裁量，停止条件未触发）。画布右键「新建宏」保留同命令面 ✓。view_state rack_control_pref 值域扩展（旧三值兼容）。
  - **探针**：151→**178 断言全绿 exit 0**（+27=坐标链路/锚点±2px/selector 三项/侧栏桥 13/adapter 契约 8）。
  - **回归电池 9 套零回退**：4 rack 测试 exit 0+probe_rack_plugin_search_popup 111/111+fe_gui_shrink a/b/c PASS。
  - **真栈**：run=**k31_sendroute_gui_20261019_1135**（dev_agent_smoke -StartKernel -RestartAgent -SendRouteGuiSmoke **exit 0**，SUMMARY_JSON checks_total=25 checks_failed=0 status=pass：L1 connect auto_create/FX 创建/默认 -6.0→L2 list+edge 读回→L3 state aux 插件对→L4 set_amount -12.5 双回读→L5 宏五步+control_graph 快照+宏驱动 send amount→L6 disconnect 归零+fx_released；+chat/confirm 腿过）。被测 kernel=**复用 K3 自建 SHA256=722A79D7…11:29 复核一致**（含 K1）+腿内 go build agent+前端 @1e33284。栈登记 4062a98d（11:31 预检六端口 tcp+udp=0/四类进程=0）→释放复核净（只杀本腿 VitAgent 27560/VitApp 7440，进程=0/六端口=0）。§8：LLM=0；首轮真栈命令 bash 吞反斜杠未起栈即失败（无栈副作用，修正重跑，如实记录）。
  - **端测边界声明**（AGENTS §5/§7）：已覆盖=三返工点命令面/锚点换算链（headless 178）+GUI 同源 payload×生产传输链×内核落链回读（真栈 25/25）+坐标修复窗口态取证实证（±0px）；**未覆盖**=窗口内真实指针交互的视觉确认（菜单/selector/侧栏工具页/调取列表）——归 **[等待用户] 手测复测三问题**（清单 K3.1 段已追加至 MANUAL_TEST_CHECKLIST.md）。
  - 主仓 stack worktree commits：`4062a98d`（栈占用登记）+`2a19f2e3`（工件收口：run 目录+取证+K31_RESULTS+清单+补丁）。主仓工作树遗留（开工前已存在，非本卡改动未动）：VitApp/Workspace/{Settings.xml,default_project.xml} M+VitApp/build-feguik3/ untracked。
- 验收（2026-10-19 早窗 GLM 主管决策流·K3.1）：**K3.1 内部验收 pass；终验=用户手测复测三问题**。①三返工点亲核（diff 亲读：:1037 第三参 get_global_transform()\*mb.position+锚点 affine_inverse 双轨在位；vit_sidebar_\* 六桥方法全在；旧链路移除探针断言亲见「old repository macro write path removed」/「old three-choice constant removed」）；②探针独立复跑 **PROBE PASS checks_total=178 exit 0**；③9 套回归独立复跑全绿（4 rack 测试+popup 111/111+shrink a/b/c 全 exit 0）；④真栈工件亲核：k31_sendroute_gui_20261019_1135 SUMMARY_JSON checks_failed=0/checks_total=25/status=pass 六腿全绿+占用/释放登记（4062a98d/2a19f2e3）时间线核正+验收时点栈面亲核净（进程=0/六端口 tcp+udp=0）；⑤取证链采信（popup_on_parent viewport 口径 headless+窗口态两轮实证、修后 ±0px、域外 :6430 fallback 分支登记备查不扩大面）。主仓工件随本裁定补入 main（run 目录+取证+K31_RESULTS+清单 K3.1 段+补丁 0001-…-K3.1）。
- 终验（2026-10-19 晚窗）：**用户手测三轮全过（首轮两缺陷一指正→环境修复复测→K3.2 四点复测），send+宏主线（K1+K2+K3）全链收官**。首轮两缺陷经 K3.1（145 卡前身段）+主检出二进制追平源码（内核 B6565DCF→722A79D7 陷阱修复）后消解；一指正落地为 K3.1③ 侧栏第一入口+K3.2 收口。前端合 main=**0e244f0**（与 K3.1/K3.2 整线合入，已推 origin；合并后 main 探针 206/206 exit 0）。
