# M3-ENVELOPE-DATACHAN-1（C1，池序 187）回执工件

- 卡：coord/cards/doing/2026-10-25-M3-ENVELOPE-DATACHAN-1.md（领取 e5739152）
- 执行：ZCode GLM-5.3 PC 执行会话 @ D:\Vit_DAW_wt_m3dc1（主仓）+ D:\Godot\project\vit-daw-frontend-m3dc1（前端仓 port/m3-envelope-datachan-1）
- 实现提交：前端仓 75bdef38（automation_client.gd+两探针）+ f461438（真栈探针预初始化修复）；主仓 port/m3-envelope-datachan-1-stack（本分支：dev_agent_smoke.ps1 扩展+本工件目录）

## 一、交付面（automation_client.gd，新文件）

1. **三方法**（rack_client track_send_* 同款 no-wait 形态，kernel_command_client→VitIpcClient UDP 4445 legacy 面；rack_client.gd 零改——git diff 亲证）：`get_curve`/`set_points`/`clear_region`；读超时 5000ms/写 10000ms；payload 构造纯函数三件（get_curve_payload/set_points_payload/clear_region_payload，region 缺省=全替换）。
2. **回执解析**：get_curve→points[{t,v,c}] float 归一+time_base+bypass+point_count；set_points→mode(region|full)+region?+written+point_count+time_base；clear_region→region+removed_points+point_count。**形态核**：clear_region 回执实际无 mode/written/time_base（PluginRackControlService.cpp:2247-2257，与 186 勘察对 set_points 回执的描述互补不矛盾；解析器不虚构字段）——186 锚点全一致，停止条件未触发。
3. **错误分类**（文案逐字透传内核串，呈现归 C2）：卡面三主类 beats_curve_rejected/empty_points_rejected/target_missing + 伴生类 malformed_region/point_out_of_domain/no_active_edit/unknown_error。
4. **工作副本底座**（纯函数，C4 直接消费）：`apply_region_edit`（区域替换：移除 [start,end) 半开区间点——镜像引擎 removePoints tracktion_AutomationCurve.cpp:587-601；区域内新点 [start,end] 闭接收——镜像内核 :2161-2163 拒收规则；两端语义差如实镜像不抹平）、`apply_region_clear`、`apply_full_replace`（=完整点集发包体，裁定①案 A：UDP 直连+写后回读对账，183 同姿态零 CAS）；坏点/坏 region 预检 fail-closed；零副作用。**单点 add/move 面不存在（有意省略）不造**。
5. **变更通知形态（裁定①③落地面）**：首版=按需拉取+每笔写回执到达后回读+既有工程刷新链覆盖；未接 delta_update 事件通知（依据见 §四核定）。

## 二、探针断言面

- **独立探针** `tools/probe_automation_datachan.gd`（headless `-s` 脚本模式，注入 FakeIpcClient，零 autoload 零 UDP 零栈）：**50/50 PASS exit 0**——A 传输面 15（cmd/command 补齐/寻址/payload 深拷贝/region 形态/超时分档/注入件不解析 VitIpcClient/custom request_id）；B 回执解析 9（归一/字段/full 无 region 键/clear_region 不虚构/错误回执形态）；C 错误分类 14（内核串逐字取自 PluginRackControlService.cpp 亲核原文+透传断言）；D 工作副本纯函数 12（半开镜像/闭接收/越界拒收/坏 region/坏点/c 缺省/区域清除/全替换排序/C4 组合链/零副作用）。
- **既有探针零回退 7/7 全绿**（headless）：K3 probe_rack_sendroute_macro 313 / popup 111 / realinput 15 / fader 35 / shrink_a PASS / shrink_b PASS / start_feed 244（shrink_c 未跑=179 域已知不稳定，本卡零既有文件改动）。

## 三、真栈腿（两轮）

- **第二轮（收口）run=datachan_gui_20261025_194233：脚本 exit 0，stack probe 16/16 PASS**——setup（clear_project/add_track/get_project_state 发现 volume 插件）+L1 空曲线披露（point_count=0+time_base=time+bypass）+L2 set_points 三点 mode=full/written=3→逐点回读一致（t,v,c 1e-6）+L3 **区域替换对账：apply_region_edit 纯函数预测 vs 活内核逐点一致**（set_points region 模式 written=2）+L4 clear_region removed_points 对账→回读空+L5 错误透传活栈（目标缺失+空点集，分类+文案原样）。被测通道=生产 GUI 传输链（VitIpcClient UDP 4445→agent bridge→内核 ZMQ），AutomationClient no-wait+kernel_command_signal_bus 回执路由（rack 生产同款）；180 横幅双源在场（STACK_TOUCH_WARNING+SUMMARY_JSON stack_warning）。自启 kernel pid 9044+VIT_PROJECT_XML 隔离副本被写 19:42:51（隔离生效亲证）。
- **第一轮（留证）run=datachan_gui_20261025_193812：exit 1**——双重缺陷：①探针 `_automation` 惰性初始化时序缺陷（script_exception→watchdog 150s 超时）——已修（f461438 预初始化），复跑=修复后首跑；②**撞用户在飞栈事故**（详见 §五）。

## 四、delta_update 覆盖面核定结论（卡面⑤，结论不定案不扩面）

**结论：delta_update 不携带 plugin state（automation 曲线）变更——首版不接事件通知的裁定①形态成立；接通知须内核侧扩面（另开卡域，本卡不扩）。**

亲核证据链（全部当日源码亲读）：
1. delta ops 生产面 `buildStateDeltaOps`（VspKernelReference.cpp:924-980）只产三类路径：`/project` replace、`/tracks/<id>` add/remove/replace（buildTrackCore=track 行减 clips）、clip 级 ops（appendClipDeltaOps）——**无 plugin state/automation 任何专用 op 路径**。
2. 入快照/_delta 的 track 行（normaliseTrackArray :603-676）携带=track 核心字段+`plugin_param_digest`+`rack`（结构图）+clip 行。其中：
   - `plugin_param_digest`（CommandDispatcher.cpp:3662-3706）=plugin_id|param_id|**getCurrentNormalisedValue() 当前值位**哈希——**不折叠曲线点集**；曲线写入只在恰好改变参数当前值（播放头相关、偶发）时才间接变 digest；
   - `rack`（normaliseRackState :571-601）=nodes/edges/clip 路由结构图，零参数零曲线。
3. 即便曲线写入偶发触发 digest 变化→`replace /tracks/<id>` op，其载荷=track 行（无曲线数据、无 per-param 信息）——消费面**不可能**从 delta 得知"哪条曲线变了/变成什么"。
4. 前端消费面 project_store.gd:187-212 只认 track/clip 路径（186 勘察同判）。
→ 曲线写回显只能靠「写后回读对账+按需拉取+工程刷新链」（本卡已落地面），与 RECON §2.3 存疑一致、现已证伪"delta 覆盖 plugin state"。

## 五、第一轮撞栈事故披露（待主管裁定）

- **事实**：占用前检查（~19:36，四端口/进程=0）与 ps1 启动（19:38:22）之间的空窗内，**用户自行拉起在飞栈**（agent pid 17316+kernel，7 轨 stems 工程，shadow track_count=7）；第一轮 ps1 走既有"Start or reuse"分支**静默复用**该栈（ps1 trail 留证："kernel command port already listening: 5555"/"agent HTTP port already listening: 7878 pid=17316"）；本卡探针 setup **clear_project/add_track 在用户活内存工程上执行**（第一轮 console PASS 行在卷）。
- **盘上零损失**：repo VitApp/Workspace/default_project.xml 末次写 19:39:01、内容=完整 stems 工程（6 stems+Reverb Bus+Pro-R）——时序=ps1 前置腿（range context）触发的**清场前**自动保存，用户态先于 clear_project 落盘；此后该文件零写入（mtime 不变）。用户下次开工程从盘重载=无损。exe 目录无独立 Workspace（VitPaths getProjectRootDirectory 从 CWD/exe 爬升至 repo VitApp，两轮内核同一路径）。
- **用户栈下线**：19:41:3x 复核已无 VitApp/VitAgent（非本 owner 手拆——ps1 throw 路径零 teardown 输出；本 owner 全程仅拆自录 pid：22736 遗留 Godot/9044 二轮 kernel/24084 二轮 agent，路径均亲核自起）。
- **根因**：①占用登记后~启动前缺一次**即时**进程/端口复核（19:36 检查 vs 19:38:22 启动，空窗 2 分钟）；②dev_agent_smoke.ps1"Start or reuse"设计对既有栈静默复用，无 owner 校验。候选整改（供主管，不定案）：冒测脚本对"already listening"分支加显式中止或 owner 核对；执行侧纪律补"启动前 30 秒内复核"条款。
- **归类**：环境中断（用户栈窗口）+流程缺陷（复核空窗）叠加，非功能失败；第一轮 exit 1 的功能面缺陷（探针时序）独立可修已修。

## 六、端测边界声明

1. **独立探针**=注入式假传输（FakeIpcClient），零 autoload 零 UDP 零栈——payload 形态/回执解析/错误分类/纯函数语义全确定性断言；不触栈（180 警告面对应的是场景模式栈探针，本件无生产传输路径）。
2. **真栈腿**=生产 GUI 传输链全链（UDP 4445→agent bridge→内核 ZMQ）真实往返：三命令+区域替换对账+错误透传；**未覆盖**：C2 车道 UI 呈现面（本卡无 UI 改动）、渲染面与用户旅程（不适用——零 webui/零 GUI 组件改动，探针为 headless 命令面断言）、beats 曲线活栈反例（内核 beats 拒收文案在独立探针逐字断言+内核源码锚点在卷，活栈造 beats 曲线需 tempo 绑定面超出本卡域）。
3. **工作副本纯函数与活内核一致性**：L3 对账腿证区域替换语义逐点一致（[start,end) 半开+闭接收双端）；全替换（mode=full）语义=L2 三点腿（clear+整批写入逐点回读）。
4. rack_client.gd 零改（git diff 亲证）；stack_probe_sendroute_macro.gd 零触碰（禁触令遵守）；主仓 dev_agent_smoke.ps1 扩展=AGENTS §5「扩展现有冒测脚本（加参数）」形态（-AutomationDatachanGuiSmoke，SendRouteGuiSmoke 同模式）。
