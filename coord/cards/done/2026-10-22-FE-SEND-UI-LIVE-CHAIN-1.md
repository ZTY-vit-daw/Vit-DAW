# FE-SEND-UI-LIVE-CHAIN-1：真实 UI 环境下外部写后 send 拓扑 GUI 消费链断裂（场次④手测取证）

- 发卡：GLM 主管决策流 / 2026-10-22 晚（依据=手测场次④用户反馈+主管取证：agent 建 send 后 GUI 悬空态+无发送量控件+无电缆；后端事实完整在卷）
- 派发确认：已确认（用户手测现场反馈+取证链）
- 验收负责人：GLM 主管决策流
- 池序 171；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**（嫌疑；断点也可能在 agent bridge 面——勘察后固定）；**P1（主旅程可感面：161/164/166 三卡修的面在真实 UI 环境整体不生效）** / 0.5-1 天 / 依赖=无；模型分级：L2 / GLM 执行会话

## 现象（2026-10-22 19:4x 手测实录）

用户从 Godot 前端拉起（主检出 main=2007cb1 含 164/166/167/168；agent=04D52490 含 162），话术「给人声加空间感」。**通过的**：旅程完整（19:44:32 track.send.connect confirmed=true+rack_add_node confirmed=true→19:44:36 chainEnded=true completed）；意图保真（162 生效——aux 对落在 vocals 轨，见取证①）。**失败的（GUI 三症状同源）**：①send 卡槽无发送量调节控件；②无接线（电缆）；③悬空态（166 悬空占位徽标=repository send 拓扑未落库形态）。

## 主管取证（2026-10-22 20:0x，工件=主检出日志，跑后栈已停只读取证）

1. **后端事实完整**：`Logs/BridgeReplies/20261022_194652_000000000_get_project_state_ipc_208.json` 在卷——vocals 轨 pluginList 含 **"S:Reverb Bus"(auxsend/1042)+"R:Reverb Bus Ret"(auxreturn/1043) 相邻对**（164 派生数据源在场；轨集=1002 Arranger+1007 bass/1012/1017/1022/1027/1032 六 stem，1032=vocals）。
2. **意图达成**：aux 对宿主轨 clip=vocals（模型推理文本显示其曾猜轨 1017/1032/1007——**170 缺口的现场形态**：模型看不到 user_named 披露行靠推理选轨；最终落 1032 或经 162 改写，source 断言留本卡真栈复刻时亲核）。
3. **探针面与真实 UI 面的覆盖缺口**：164/166 真栈腿=headless 场景装配（探针自建 ProjectEventHub+Repository+Vit_Graph_Rack 实件+注入命令），**不覆盖真实 UI 视图树**（Godot 完整拉起→启动页→工程打开→dock/viewport 装配→用户交互时序）。合并态探针 289 全绿在卷=修复面本身未被回退，断点在真实 UI 独有的链路段（project_loaded 广播到达/拉取触发时机/repo 落库/视图重建挂接/订阅装配之一）。
4. **疑点记录**：会话名 webui_mvjh2p5b（chat 面）；19:46:52 前端经 bridge 拉 get_project_state（ipc_208，**说明前端拉取链活着**）——前端拿到了含 aux 对的工程状态却仍悬空，嫌疑收敛到**track.send.list 拓扑拉取/落库环节或机架视图消费环节**，而非广播断链。

## 范围

①真实 UI 环境复现（Godot 拉起→工程→话术或注入等价写）+断点定位：project_loaded 广播→rack 双拉取（track.send.list+list_tracks）→条目缓存→load_nodes→repo 拓扑落库→send_topology_changed→卡/合一 icon 量控件/电缆消费链，找出真实 UI 与 headless 装配的分叉点；②修复断点；③防回归腿：探针补真实 UI 形态覆盖（或明确手测复核清单+驱动脚本按 dev_agent_smoke -StartUI 真栈模式）；④connect source 轨断言（1032 亲核）。

## 文件域

以勘察结论固定（预期 `app/rack/Vit_Graph_Rack.gd`/`vit_dock/core/project_repository.gd`/161 拉取挂接邻域/`app/kernel/clients/` 订阅面——真实 UI 装配链可能涉及 viewport_frame/dock，越界需上交扩域）；开工时写入卡面。越域即停。

## 验收标准

真实 UI 真栈腿（-StartUI 形态）：话术后卡+**量控件**+**电缆**+**目标名徽标**（非悬空）零操作同现，断开同链消失；既有探针（289 面）零回退；用户手测复核（窗口态视觉=目检项）；工件入主仓 coord/runs/FE-SEND-UI-LIVE-CHAIN-1/。端测边界声明。

## 停止条件

断点在 agent 面（bridge 广播/快照剥数据族——161 根因②同族残留）而非前端仓 → 锚点上交拆 agent 面卡。

- 领取：（2026-10-22 20:1x +0800 / origin/main=2b3a9b05 / GLM-5.3 L2 PC ZCode 执行会话（171 卡会话，用户口令指定领取） / 分支=port/fe-send-ui-live-chain-1（前端仓 worktree D:/Godot/project/project-vit-daw-frontend-senduichain1@基线 2007cb1）+ 协调分支 coord/fe-send-ui-live-chain-1-claim / 领取提交=本提交；卡面自主管本地 main ee0050de 摘取入共享 main（只同步 coord 面，同 2b3a9b05 先例）；领取时核对=卡在 todo 派发已确认（手测现场反馈+主管取证链）、依赖无、doing 卡 GEN-SUNO-WEB-DESIGN-1 零生产代码无文件域冲突、真栈空闲（四类栈进程=0，用户 Godot 编辑器 30564 --editor 在跑不动；5555/5556/7878/7879/8787 tcp/udp 全 free）；真栈腿占用随本提交同批登记（今晚错峰序首位）；文件域按卡面预期域开工，勘察收敛后随回执固定）
- 回执：（**2026-10-22 22:2x 执行侧自验完成**｜断点定位结论/commit/真栈 run/端测边界全四项）
  - **断点定位结论**：前端消费链功能面完好——内核日志行号排序还原（connect=82120/19:44:32 后 GUI 多轮 track.send.list 回执均完整含 1032→1037@-6.0），真实 UI 窗口态驱动四轮实证：写后合一卡+徽标零操作同现、按钮展开=对卡+量控件+电缆、断开同链消失。用户三症状机制=①**VSP hub 停摆致写通知延迟 2m16s**（vsp_hub_last.log 19:43:12→19:45:56 空窗+agent POST deadline+发布者 19:46:52 重注册；悬空=停摆窗非持久缺陷；**停止条件命中=agent 面锚点上交**：建议 hub/agent 实时发布停摆韧性+GUI 写通知断供兜底卡，归主管裁定）②折叠态电缆/量控件=设计归属展开面（Vit_Graph_Rack _send_route_cable_color 注释「合一态无对卡连接」）③**展开/折叠重建 freed-object 引擎错误**（用户 4 次/本卡每轮 2 次确定性复现）=本卡修复对象。
  - **卡④亲核**：内核原始载荷行 82120 source_track_id=1032 auto_create_fx_track=Reverb Bus（vocals 断言成立）。
  - **修复**：Vit_Graph_Rack.gd `_clear_graph_canvas_for_rebuild` n.free()→remove_child+queue_free（信号安全清树，同步契约不变；首版 deferred 被探针 9 断言合法打回后改 v2）。实现 **1516611@port/fe-send-ui-live-chain-1（前端仓，已推 origin）**。
  - **真栈 run**（窗口态真 UI，内核 K3 722A79D7 只读+agent 04D52490=用户会话同构建）：九 run 见 coord/runs/FE-SEND-UI-LIVE-CHAIN-1/（终验=sendui1_final_20261022_2210：B 卡+徽标 Reverb Bus 非悬空/B2 对卡+ParamPortRow_send_level+电缆 1042→1043/B3 折叠回/C 断开全消+读回 0/**freed=0**）；K3 探针 **289/289 PASS**+popup **111/111 PASS** 零回退（log 在卷）。驱动=env 门控 sendui1_repro_driver autoload（防回归腿：真实 UI 形态可复跑，bringup ps1+run 目录）。
  - **端测边界声明**：程序化生产链入口（start_page._create_new_project/顶栏导入链/按钮 pressed.emit）非鼠标旅程，渲染像素/窗口态视觉=用户手测目检项；mixer 视图默认 2 位态未实例化未采样（161 腿有 ext161 13/13 覆盖）；话术腿未跑（写=agent 同构注入，164 L8 先例），是否补话术腿归主管裁定。
  - 文件域固定：app/rack/Vit_Graph_Rack.gd+project.godot（autoload 注册）+app/kernel/autoloads/sendui1_repro_driver.gd（新增）——越域未发生。

- 验收：（裁定文件 / 验收 commit）
