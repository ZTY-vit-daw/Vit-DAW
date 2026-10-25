# M3-ENVELOPE-LANE-UI-1（C2，池序 189）执行回执

- 卡：[coord/cards/done/2026-10-25-M3-ENVELOPE-LANE-UI-1.md](../../../../coord/cards/done/2026-10-25-M3-ENVELOPE-LANE-UI-1.md)（领取 2026-10-25 20:21，前端基线 3413fe8）
- 执行：ZCode GLM-5.3 PC 执行会话（M3-ENVELOPE-LANE-UI-1 专属）；2026-10-25 晚窗
- 结论一句话：**C 案独立 TrackEnvelopeLane2D 落地（呈现+选中，点编辑归 C4）——独立探针 39/39、既有 9 件探针零回退、真栈腿 envelope_lane 23/23 连续两轮 exit 0（含车道呈现读回逐点一致）；vit_track_lane_2d.gd / rack_client.gd / dev_agent_smoke.ps1 全部零改。**

## 一、commit

| 仓 | 分支 | commit | 内容 |
|---|---|---|---|
| 前端仓 | port/m3-envelope-lane-ui-1 | ea9fa23 | 实现：新 lane 文件 track_envelope_lane_2d.gd + 工厂扩分支挂载（Track_Row_Controller `_ensure_lane_2d_layer`→`_ensure_envelope_lane_2d_layer`）+ 参数选中面 + TimelineViewMapping envelope 族 hit-kind + TimelineInputArbitrator 只选中/聚焦分派 + 独立探针 tools/probe_envelope_lane_ui.gd + 真栈探针 tools/stack_probe_envelope_lane.gd/.tscn |
| 前端仓 | 同上 | 4f9d8a5 | 真栈探针三轮缺陷修正（见 §四）|
| 主仓 | port/m3-envelope-lane-ui-1-stack | 本提交 | 真栈 run 工件（本目录）+ RESULTS.md |

## 二、实现面（对照卡面目标 1-4）

1. **车道挂载（C 案）**：`app/tracks/track_scene/track_envelope_lane_2d.gd`（TrackEnvelopeLane2D）新文件；`_ensure_lane_2d_layer`（Track_Row_Controller.gd:1033-1059 锚点亲核一致）尾部扩 `_ensure_envelope_lane_2d_layer`——底部固定高度叠加条（96px，PRESET_BOTTOM_WIDE，z_index 15）。行高形态=卡面允许的「车道条固定高度叠加」案：**行高/波形 lane 布局零改动**（行高联动另计，不动 timeline_dock_adapter/drop router 常量=零越域）。关闭态 visible=false+mouse_filter=IGNORE+hit 空=时间线既有行为零变化。
2. **曲线呈现**：经 C1 `AutomationClient.get_curve`（no-wait+kernel_command_signal_bus 回执路由，rack_client 生产同款面）按需拉取；t→x 映射复制 vit_track_lane_2d:1452-1482 同式（_visible_range 秒窗线性，权威=wrapper get_visible_timeline_range_seconds）；播放头=GuiTransportClock 三级回退链同式（:4043-4053）+节流重绘（>0.001s）；空曲线=empty 披露形态、加载失败=error+内核文案逐字透传、beats 非空曲线拒绝呈现（fail-closed 披露，防拍值按秒静默错位）。v→y 归一：描述符 min/max 优先，缺描述符 clamp01 兜底并标记 assumed（读回面 get_value_scale 可见，不静默）。
3. **参数选中面**：车头 plugin/param 两级 OptionButton+关闭钮。plugin 目录=project_repository rack_nodes 快照（`get_rack_nodes_by_track`——185 换源裁定面，repo 归一形态）；param 描述符=rack_client.get_plugin_parameters（与 Vit_Graph_Rack `_param_port_rows_by_plugin` 同一命令同源；独立 request_id env_lane_params_<plugin>，不读其私有缓存）；探针注入面 set_plugin_catalog/install_clients/handle_command_completed（回执同一处理体，零旁路）。
4. **交互分派**：hit-kind 词表（TimelineViewMapping.resolve_hit_target）在 clip 族之前加 envelope 族（envelope_lane/envelope_lane_point 近点 6px+point_index）；仲裁器 match（TimelineInputArbitrator.gd:503 match）新增分派臂=行选中+车道聚焦+清 clip 选择+消费——**只选中/聚焦**（4px 手势状态机不启用，点编辑归 C4）；车道关闭时行方法返回空→词表零变化。车头 OptionButton/Button 经 drop_router 既有让路链（_is_non_lane_interactive_timeline_control 认 OptionButton/BaseButton）原生可点，零路由改动。

## 三、判定面

- **独立探针** `tools/probe_envelope_lane_ui.gd`（headless -s 零栈，假传输注入）：**39/39 exit 0**——A 车道在场/工厂幂等/波形 lane 实例与视图模式零改/关闭态 hit 空+IGNORE/固定高度叠加几何；B 曲线读回逐点（t,v,c）+t→x 秒窗线性+v→y 描述符归一；C 换参/换 plugin 重置/目录外拒选；D payload 寻址+cmd/command 补齐；E fail-closed 全形态（空目录/空曲线披露/内核文案逐字/beats 拒呈现/队列失败）。**段完成标记机制**：GDScript 运行时错误中止函数但不中止 _run——探针以段标记统一收尾判 FAIL，杜绝静默跳段假 PASS（本卡探针工程自纠项）。
- **既有探针零回退 9/9**：K3 313 / popup 111 / realinput 15 / shrink a·b·c（c 亦绿）/ start_feed 244 / fader 35 / datachan 50，全部 exit 0。
- **真栈腿**（§2.2 登记 caaad069 21:25→释放 b15a1564）：ps1 栈生命周期（main 前守卫版，-StartKernel -RestartAgent -NoChatSmoke -NoStripSilenceSmoke，**ps1 零改**——188 未合入，探针场景由本 owner 手动驱动=ps1 datachan 腿同款 cmd 形态）exit 0；`stack_probe_envelope_lane.tscn` **23/23 连续两轮全绿 exit 0**（r4 21:50:29 + r5 21:51:50）：setup 清场/建轨/发现 volume 插件（get_project_state 同款面）+真 track_row.tscn 挂载+kernel track id 绑定→L1 车道空曲线披露+内核对照一致→L2 set_points 三点（探针侧直调 client，C1 L2 同款）→**车道呈现读回逐点一致（t 0.0/1.5/3.0, v 0.8/0.5/0.2, c 0/0.25/0）**→L3 换参+bogus 参数活内核错误文案逐字透传（target_missing）→L4 envelope_lane/envelope_lane_point 命中+仲裁只选中/聚焦分派（行选中+车道聚焦+清 clip 选择+消费+近点索引= C4 消费面）+关闭态既有词表零变化。180 警告横幅在场（STACK_TOUCH_WARNING）。
- **VIT_PROJECT_XML 隔离生效**：副本（git 干净版 sha 4e899fb6）末次写 21:51:51=r5 探针 cleanup 前内核自动保存；主检出 default_project.xml 运行前后 sha256 一致（1fef9ce9）零触碰。

## 四、三轮探针缺陷与修正（过程留证，全部为探针侧缺陷，lane 实现零返工）

1. r1：stack_probe `var _` 非法变量名（GDScript）→ 命名修复。
2. r2 前置：协程 `_await_lane_state` 七处缺 await（--import 面不检协程，场景模式编译报）→ 全部 await 化。
3. r2：探针顺序缺陷——目录注入先于开车道，`set_envelope_lane_open(true)` 的 refresh_plugin_catalog（生产源=repo）覆盖注入目录（探针场景无 repo autoload）→ 注入后置。**此为探针环境差非实现缺陷**：生产面 repo 在位，重开车道同目录插件保留选择（_set_catalog 保留逻辑）。
4. r3：L4 呈现恢复调用排在断言之后（bogus 错误态清空点集→近点必败）+关→开 toggle 在无 repo 环境清点集 → 断言顺序重排（点断言先行、关态检查后置）。
5. r3：params 等待窗 20s < rack_client PARAM_GRABBER_TIMEOUT_MS=30s 已知慢档 → 35s+探针侧直调 get_plugin_parameters 披露内核真实回执。

## 五、披露与边界

1. **get_plugin_parameters 描述符面边界（活内核实证）**：对**内置 volume 插件**，内核回 `error "Plugin parameters are only available for external plugins"`（L1 直调逐字披露）——描述符面仅外部（rack VST）插件可用。车道以 fail-closed 形态如实呈现（param 口显示 params error+v→y 走 assumed clamp01 兜底，读回面可见标记）；曲线呈现/选中/读回不依赖描述符（L2 读回一致在此形态下达成）。**生产主用例=rack 外部插件（Vit_Graph_Rack 同命令面日常消费），描述符管线对其成立**（本腿未装载 VST——装载外部插件入此腿超本卡文件域，留 C4/后续卡若需可扩）。
2. **端测边界声明**：①独立探针=headless -s 假传输注入（零栈零 UDP，180 警告域外）；②真栈腿=生产 GUI 传输链全链真实往返（UDP 4445→agent→内核 ZMQ）+真 track_row.tscn 场景+真 mapping/仲裁器分派（autoload 在位编译）；**未覆盖**=渲染面像素级目检与用户旅程（E2E-WEBUI-1/JOURNEY-1 两层仍属建设面）、外部插件描述符真实管线（见上）、C4 写回链（本卡不发 set_points 生产路径——L2 的 set_points 为探针侧直调 client 装载曲线，非车道 UI 编辑写回）；③车道开关/参数选择的鼠标操作=仲裁器/词表对象级断言（_begin_gesture 直调），非 OS 级事件注入。
3. **vit_track_lane_2d.gd 零改**（git diff --name-only 亲证）；rack_client.gd 零改；主仓 scripts/dev_agent_smoke.ps1 零触碰（188 文件域纪律）。
4. **拆栈注记（不处置，留档待核）**：拆栈复核时发现 4 个非本腿 Godot 进程（GUI 13284+console 35376@21:28:13、GUI 23476+console 29712@21:35:16；13284 持 UDP 4444 遥测口）——非本 owner 所启，按 §2.2 不擅自终止；对本腿断言面零影响（命令走 4445，4444 仅遥测，且 r4/r5 两轮全绿与其共存）。

## 六、§8 运行声明复核

独立探针/真栈探针均为 LLM=0 全确定性；成功=进程退出码 0（达成）；失败分类=assertion_fail（r1-r3 探针缺陷三轮，逐条修正留证）/script_exception（r1 解析失败属此类）/environment_interrupt（无）。真栈 exit 0 两轮=存在稳定成功路径的重复证据（非稳定性比例声明）。

## 七、工件清单（本目录）

- `ps1_console.log`——ps1 栈生命周期全程（无 already listening，kernel 36420+agent 自启亲证）
- `envelope_lane_console.txt`——r5 终轮（23/23+L1 描述符逐字披露+SUMMARY_JSON）
- `envelope_lane_console_r2/r3/r4.txt`——过程轮（r2=18/22 顺序缺陷、r3=21/23、r4=23/23 首个全绿）
- `headless_probe_envelope_lane_ui.txt`——终态独立探针 39/39
- `default_project.xml`——隔离副本（被内核写=隔离生效证据，非 git 干净版）
