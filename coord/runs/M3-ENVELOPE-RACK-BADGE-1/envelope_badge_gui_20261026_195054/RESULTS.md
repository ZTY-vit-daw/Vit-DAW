# M3-ENVELOPE-RACK-BADGE-1（191）真栈腿回执 RESULTS

- 结论一句话：**机架插件卡「参数有包络」徽章落地——独立探针 33/33、既有 10 件探针零回退、真栈腿 `stack_probe_plugin_envelope_badge` 21/21 连续两轮 exit 0（装载外部插件→拉描述符零包络隐藏→set_points 三点→Get Params 强刷徽章出现（文本精确「1参数·3点」）→clear→强刷徽章消失）；文件域=app/rack/ 两文件+探针新文件，stack_probe_sendroute_macro.gd 与 app/timeline、app/tracks 零触碰（192 卡域亲证）。**

## 交付物

| 仓库 | 分支 | commit | 内容 |
|---|---|---|---|
| 前端仓 | port/m3-envelope-rack-badge-1 | 08df199 | 实现：rack_node_display_rules.gd `plugin_envelope_badge_state` 纯函数（33 行）+ Vit_Graph_Rack.gd 卡头徽标行（60 行：PluginEnvelopeBadgeRow 工厂 body 后挂装+_refresh_plugin_envelope_badge+回执面刷新钩子+Get Params 强刷面擦缓存再拉）+ 独立探针 tools/probe_plugin_envelope_badge.gd + 真栈探针 tools/stack_probe_plugin_envelope_badge.gd/.tscn |
| 主仓 | port/m3-envelope-rack-badge-1-stack | （本 run 入库提交） | 真栈工件（本目录） |

- 前端 diff stat vs 领取基线 20873f8：5 文件 +872/-0（见 claim_state.txt），文件域零越界。

## 设计要点（勘察择型披露）

1. **徽章形态**：caption「包络」（灰）+chip「N参数·M点」（淡紫 0.45,0.38,0.60=包络/曲线域，与路由徽标青/灰区分）；零包络/描述符未达=**隐藏**（灰态零信息=添噪，勘察裁定隐藏）；复用 166 chip 构造改名独立寻址（185 湿链徽标同位先例）。
2. **数据源**：190 `envelope_point_count`，回执链=ctrl_params_（`_on_param_port_rows_reply`→`_param_port_rows_by_plugin`，卡面指定同源链）；纯函数对缺字段/非数值/负值/bool 一律 fail-closed 归零（pre-190 内核形态不造假）。
3. **刷新面**（零新增请求面）：①描述符回执到达面 `_refresh_param_port_holders`（holder 未展开也刷——徽章在卡头）；②节点重建路径 `_add_plugin_graph_node` 末尾按缓存显徽（拓扑/工程重建=185 同位先例轻刷新）；③**再拉触发=Get Params 既有强刷面**（⌗params 提示文案已明示「点击 Get Params 可强制刷新」）——擦 rows 缓存后走既有 `_request_param_port_rows`，命令仍是同一 get_plugin_parameters，无新命令面。
4. **已知边界（如实）**：内置轨级参数（volume/pan）不在 get_plugin_parameters 回执面（190 卡面边界，189 L1 活内核逐字在卷）——首版覆盖面=外部 rack VST 插件参数，轨级缺口显示为「无徽章」不造假。

## 判定面

- **独立探针**（假传输零栈）：`probe_plugin_envelope_badge.gd` **33/33 PASS exit 0**——纯函数 12（fail-closed 族/int+float 双收/聚合/文本精确）+装配 21（在场+卡头位+默认隐藏+caption→⌗params 展开恰一笔 ctrl_params_→回执显化+文本精确→逐插件隔离→零包络隐藏→pre-190 形态隐藏→重建缓存消费零新请求→Get Params 强刷再拉→红线：徽章显化零新增画布连线+徽章链零新增请求）。
- **既有探针零回退**（worktree m3envbadge1@08df199，.godot import 后跑）：185 badge **36**/189 datachan **50**/189 lane **39**/sendroute_macro 313/fader 35/realinput 15/shrink a+b+c/start_feed 244/popup 111——全 exit 0。
- **真栈腿**（生产传输链 VitIpcClient UDP 4445→agent→内核 ZMQ）：`stack_probe_plugin_envelope_badge.tscn` **21/21 连续两轮 exit 0**（r1 19:55:52/r2 19:56:1x，SUMMARY_JSON pass）：
  - setup：clear_project→add_track→plugin_search 发现外部插件（ValhallaSupermassive）→rack_add_node 装载（190 冒测同款面，pgtdc 先例不走 PCA 门）；
  - L1：描述符就绪（探针侧直拉）+**190 字段在场亲核+基线全零**（本卡前提）；
  - L2：真 Vit_Graph_Rack 实例+生产 load_nodes→插件卡在场+徽章行在场（卡头邻域 index=1）+**隐藏 fail-closed**；
  - L3：⌗params 展开（既有拉取面）→ctrl_params_ 回执经生产传输到达→缓存全零（与内核直拉对照一致）→**徽章仍隐藏**；
  - L4：automation.set_points 三平坦点（v=当前值不动数值面）written=3；
  - L5：Get Params 强刷→**徽章出现+文本精确「1参数·3点」**+内核直拉目标行=3 其余 0 对照一致；
  - L6：automation.clear_region removed_points=3→Get Params 强刷→**徽章消失**+内核直拉全零对照。
  - 180 警告横幅双源在场（STACK_TOUCH_WARNING+SUMMARY_JSON stack_warning）。

## 栈记录（§2.2）

- 登记 7cf57c0a（19:52-53 推送）→即时复核 19:53:18（<60s：栈端口/进程全零；发现一对 19:53:03 起的 Godot `--headless --import` 瞬态进程——甄别=纯导入建缓存非栈操作，40s 后自退亲核，193 勘察会话同期在飞，无端口占用）→终核 19:54:34 全零→ps1 启动。
- ps1 最小腿 `-StartKernel -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -KernelExe <190 二进制> -GodotProjectRoot <前端 worktree m3envbadge1> -GodotExe <4.6.1 console>` **exit 0**；kernel=**190 卡自建二进制只读复用**（sha256 85b4e8df…38c9 与 190 回执亲核一致=含 envelope_point_count 改动；本卡零内核改动）；agent=stack worktree D:/Vit_DAW_wt_m3badge1_stack 构建 pid 33320；kernel pid 23640 **本腿自启，无 already listening**（纪律②）。
- 拆栈 19:56:2x owner 手拆 VitAgent 33320/VitApp 23640→复核 VitApp/VitAgent/VspHub/Godot=0、TCP 5555/5556/7878/8787/18787 listen=0、UDP 4444/4445=0。
- **VIT_PROJECT_XML 隔离生效亲证**：隔离副本（git 干净版）末次写 19:56:xx=r2 期内核自动保存；主检出 default_project.xml sha256=1fef9ce9…（与 189 回执运行后记录逐字节一致）+末次写 2026-10-25 19:39:01（187 事故留档时刻）=**本腿零触碰双证**。
- 错峰互查（§2.2 增补③）：登记前 fetch 亲核 PC-RUNTIME-STACK 当前占用=空（190 已释放 18:57）+192 卡在 todo 未领取无占用+193 勘察卡领取注记「无真栈腿不启栈」亲读。

## 端测边界声明

- 独立探针=假传输零栈（fake client 直供回执）；真栈腿=生产 GUI 传输链全链真实往返+真 Vit_Graph_Rack 实例+活内核（含 190）真实装载/写点/清点。
- **未覆盖**：渲染面像素级目检与用户旅程（E2E-WEBUI-1/JOURNEY-1 建设面；徽章观感随手测场次）；用户手动点按 Get Params/⌗params 的 OS 级事件注入（探针为对象级直调既有处理器）；多插件多参数同时有包络的视觉布局（纯函数聚合已覆盖，真栈腿单插件面）；内置轨级参数包络（190 数据源边界，主管已裁另裁数据源）。

## 运行前声明（§8）

LLM=0 全确定性；成功=进程退出码 0；失败分类=assertion_fail/environment_interrupt/script_exception；真栈两轮均 exit 0（r1+r2 连续绿=189 先例口径）。
