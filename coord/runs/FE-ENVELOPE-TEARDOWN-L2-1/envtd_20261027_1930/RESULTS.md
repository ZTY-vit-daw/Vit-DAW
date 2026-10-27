# FE-ENVELOPE-TEARDOWN-L2-1（204）真栈腿回执——envtd_20261027_1930

- 卡：[FE-ENVELOPE-TEARDOWN-L2-1](../../../cards/done/2026-10-27-FE-ENVELOPE-TEARDOWN-L2-1.md)（池序 204，重设计链卡 4；领取 4207650f@main）
- 执行：GLM-5.3 L2 PC 执行会话（ZCode）；2026-10-27 18:47–18:52
- 被测=前端 worktree `D:/Godot/project/vit-daw-frontend-envtd1` @ `port/fe-envelope-teardown-l2-1` @ **3140681**（已推远端）
- 交付面：特写窗包络模式整体退役（envelope_dock_adapter/canvas 删除+registry 撤条+vit_open_envelope_detail/旧口存根撤+⤢ 转发撤+view_type 参数化还原；特写窗保留给 3D 波形/频域——切换面机制保留清单恰 `["detail"]`）+机架徽章 191 默认撤（Vit_Graph_Rack 徽章行/刷新+display_rules 纯函数；数据面=envelope_point_count 行保留给车道 ●N）+探针退役 19 件+新探针 2 件（probe_envelope_teardown 离线+stack_probe_envelope_teardown 真栈）+发送量寻址探测腿（196 模式）。

## 一、离线探针（无栈腿，-s 模式，run 目录 offline_regression/）

- **新探针 `tools/probe_envelope_teardown.gd` 50/50 检全绿 exit 0**：A registry 面（无 envelope 键+get_view_descriptor 回空+spawnable 不含+六类生产视图在册）／B dock_root 源码面（两入口零在册+切换面清单恰 `["detail"]`+view_type 参数化零残留）／C timeline adapter 源码面（⤢ 转发三件零在册）／D 退役件零在场（vit_dock 5+探针 19 逐件）／E **全树零残留引用扫描**（res:// 下 .git/.godot/构建目录外全部文本文件对 7 禁词零命中：envelope_dock_adapter/envelope_closeup_canvas/vit_open_envelope_detail/vit_open_envelope_in_stage_slot/request_open_envelope_closeup/plugin_envelope_badge_state/PluginEnvelopeBadge；探针自身豁免=断言清单载体）／F 机架徽章撤除面（源码两文件零徽章件名+子类探针机架真实装配插件卡后零 PluginEnvelopeBadgeRow 实例断言）。
- **全家回归 21 件全绿**（保留探针全量）：车道族 6 件 **274 检与 203 验收基线逐件同数**（autolane_rebuild 73+autolane_edit 63+lane_ui 36+lane_auto 25+lane_toggle 27+datachan 50）；rack 族 5 件与 185 收据同数（wet_chain 36+popup 111+fader 35+realinput 15+sendroute_macro 313）；shrink a/b/c=87/119/342（与基线主检出对照逐件同数——旧收据 88/120/343 为 168 时代计数，基线亲跑排除回退）；start_feed 244+vitnote 六件全绿。**app/rack 改动零回退实证**（徽章撤除不伤湿链徽标/搜索弹窗/sendlevel/sendroute）。

## 二、探针全家处置清单（卡面验收⑤：退役/改写/保留）

| 探针 | 卡 | 处置 | 依据 |
|---|---|---|---|
| probe_envelope_closeup（81）+stack 对 | 194 | **退役删除** | 特写窗包络模式整体退役（裁定链）；编辑逻辑已由 203 迁入车道（autolane_edit 覆盖） |
| probe_envelope_curve（55）+stack 对 | 195 | **退役删除** | 同上（曲率编辑已迁移=autolane_edit K 系列） |
| probe_envelope_detail_home（52）+stack 对 | 200 | **退役删除** | vit_open_envelope_detail 入口撤除（F1=A 宿主结论作废在卷） |
| probe_envelope_first_point（41）+stack 对 | 199 | **退役删除** | 预载已删 adapter 场景；首点手势已由 autolane_rebuild L3+autolane_edit E2 覆盖 |
| probe_plugin_envelope_badge（33）+stack 对 | 191 | **退役删除** | 机架徽章 UI 随旧面默认撤（卡面②）；数据面=外部插件行保留（lane ●N 消费） |
| stack_probe_envelope_lane+lane_auto（对） | 189/192 | **退役删除** | 189 旧车道 API/192 在场性三态形态已随 202 重写作废（裁定链「在场性驱动废弃」）；新系统覆盖=autolane 栈探针 |
| probe_envelope_lane_ui（36） | 189→202 | 保留（202 已迁移重写） | 断言面=新车道 UI |
| probe_envelope_lane_auto（25） | 192→202 | 保留（202 已迁移重写） | 断言面=●N 消费 |
| probe_envelope_lane_toggle（27） | 198→202 | 保留（202 已迁移重写） | 断言面=两态开关 |
| probe_autolane_rebuild（73）/probe_autolane_edit（63） | 202/203 | 保留 | 现行系统 |
| probe_automation_datachan（50） | 189 | 保留 | 底座=automation 数据通道面（裁定链「底座保留」） |
| stack_probe_autolane_rebuild/edit（对） | 202/203 | 保留 | 现行系统真栈旅程 |
| probe_fe_gui_shrink_a/b/c、fx_wet_chain、rack 四件、start_feed、vitnote 六件 | — | 保留 | 域外零回退面（本轮全家回归全绿） |
| **probe_envelope_teardown（50）+stack_probe_envelope_teardown（44）** | 204 | **新增** | 本卡拆除面断言+send 寻址探测+三组端到端 |

- 合计：退役 19 文件（5 离线+7 栈探针对）、保留 21 离线+3 栈探针对、新增 1+1。
- 退役形态=直接删除（FE-GUI-SHRINK-IMPL-1 Phase A「删除性工程」先例）；历史断言面归档=本 RESULTS 清单+203/202/191 等卡回执在卷。

## 三、真栈腿（§2.2 登记+三纪律+188 守卫+VIT_PROJECT_XML 隔离+错峰）

- **登记**：cf5d74af 推 main 成功+远端重读确认置顶（18:47 登记）；登记前只读核对 18:46:11 进程/端口全零+fetch 重读占用空（最新释放=203@17:46）+错峰核对无在飞真栈卡（doing 仅本卡）。
- **三纪律**：a) 登记推送后**即时复核**（18:48:43 远端亲读置顶+进程/端口全零，<60s 窗口）+driver PREFLIGHT_CLEAN 二次预检（18:48:56=复核后 13s，ps1_minimal.log 首行自检）；b) 无 already listening（kernel pid 19240/agent pid 11824 均本腿 ps1 -StartKernel/-RestartAgent 自启，188 守卫版脚本）；c) 登记先于启动。
- **栈形态**：kernel=stack worktree `D:/Vit_DAW_wt_envtd1_stack`（4207650f=origin/main；内核面与 203 同源零改动亲核——97d47c0b..4207650f 零 VitApp/ 提交，含 196 automation 三命令+201 轨级描述符行+undo 事务；CMake configure+build exit 0，sha256 `19215a25`3…；VIT_TRACKTION_ENGINE_DIR=主检出只读；cppzmq/libzmq=find_package 主检出 _deps 构建产物只读复用零网络）；agent=stack worktree 构建（Go 零改动）；被测=前端 `envtd1@port/fe-envelope-teardown-l2-1@3140681`。
- **driver**：`powershell -NoProfile -ExecutionPolicy Bypass -File console\envtd_stack_leg.ps1`（203 edit1_stack_leg.ps1 同款；完整命令在件内）→ **ps1 最小腿 exit 0**。
- **探针**：`stack_probe_envelope_teardown.tscn` 44 检。**连续两轮全绿：R1 44/44 + R2 44/44 均 exit 0（SUMMARY_JSON checks_failed=0 双轮，零修正轮）**：
  - T0 拆除面运行时断言：registry 实例无 envelope 描述符+spawnable 清单不含+六类生产视图在册；dock root 场景实例 has_method(vit_open_envelope_detail/in_stage_slot) 均否；
  - S0 发送量寻址探测（196 模式，§四详述）；
  - E1 装载链：plugin_search 真发现（fallback 未触发）→rack_add_node 装载 ValhallaSupermassive→描述符预热→真行装配→展开→默认音量车道生产链落定（描述符+get_curve EMPTY）；
  - E2 选择器三组收口（④）：轨道组恰 volume/pan 两行（201 实面镜像）；发送组恰 1 项占位禁用（标签「发送（待接入）」）；插件组=实装载 VST 实参数（190/201/202 面端到端）；
  - E3/E4/E5 三组各画一条（生产 _gui_input 手势链）：EMPTY 按下即建→拖动零写（bus 计数=0）→**释放恰一笔（bus automation.set_points 完成==1）+回显门+内核 get_curve 读回逐点一致（t,v,c）**——音量/声相（选择器条目绑定）/插件参数（dry level 锚点）三组逐组；
  - E6 undo 事务：Ctrl+Z（生产按键链→内核 undo 事务）→**插件参数车道回 EMPTY+内核 get_curve 空（末笔 LIFO 回退）+音量/声相车道 1 点保持+内核读回一致**；
  - cleanup clear_project。
- **VIT_PROJECT_XML 隔离亲证**：run 副本 git-clean 基线（blob **287956e7**=202/203 同基线，hash-object 亲核）被内核写入（SHA1 **ce0f6543**，mtime 18:50:22=本腿运行期=隔离生效）；stack worktree `VitApp/`+`agent/` 运行后 git 干净；主检出 default_project.xml mtime 14:01:21（本腿前既有值零触碰）。
- **拆栈**：owner 手拆 kernel 19240/agent 11824（路径亲核=worktree 自建）→18:52:03 复核 VitApp/VitAgent/VspHub/Godot=0、TCP 5555/5556/7878/8787/18787 listen=0、UDP 4444/4445=0。
- **释放记录**：2aee5e0d（登记行收口 cf5d74af 登记–18:52 释放，随本回执同批推送）。

## 四、S0 发送量寻址探测裁决（196 模式：如实披露，证据上交）

- **探测面**（真栈逐字留证，probe_r1/r2.log SUMMARY_JSON `send_addressability` 字段）：
  - 发现面：track.send.connect（source 1036 轨→fx 1038 轨）ok+track.send.set_amount（amount_db=-6）ok；get_project_state source 轨 pluginList 四行：`S:Track 8`（type=**auxsend**，id 1043）+`R:Track 8 Ret`（auxreturn）+`Volume & Pan Plugin`+`Level Meter`——**AuxSend 插件在 get_project_state 可枚举**（plugin_id=ItemID 1043）；
  - **automation 面：可寻址**。`automation.get_curve(plugin_id=1043, param_id="send level")` → **ok**（空曲线 points=0 如实披露）；`set_points` 两点写+get_curve 读回往返 **ok**（S0h 双轮过）。param_id 锚点=tracktion 引擎源 `tracktion_AuxSend.cpp:22 addParam("send level", …, {0..1})`；候选族其余四项（send_level/send amount/gain/level）均 `Parameter not found` 逐字留证；
  - **描述符面：拒绝**。`get_plugin_parameters(track_id, plugin_id=1043)` → error `Plugin parameters are only available for external plugins`（PluginRackControlService.cpp 内置非 volume 插件拒绝面——201 B 案边界）。
- **裁决**：send 量 **automation 三命令面全通（可寻址+有曲线挂载）**，但选择器接入需要描述符面（值域/显示名/单位/envelope_point_count=●N 数据源）——**当前内核面缺席，启用发送组需内核扩面**（按 201 B 案先例=描述符面扩 auxsend 内置插件行，单一数据源原则）。**停止条件分支触发→证据上交另裁，不在本卡扩内核**：选择器发送组保持占位禁用（E2b 断言=当前实现如实），不硬造发送参数知识（201 B 案用户原则：不靠前端背参数知识）。
- 两轮探针 send_addressability 裁决一致（addressable=true/描述符面拒绝文案逐字同）。

## 五、停止条件核对

- 发送量寻址需内核扩面 → **触发**：证据（§四）随本回执上交主管另裁；本卡其余面不受阻（卡面该分支仅约束「不扩内核」，选择器占位禁用+如实披露=分支内处置）。
- 拆除发现 vit_dock 之外残留引用（结构漂移）→ **未触发**：全树 7 禁词扫描零命中（离线 E 段+拆除面在 vit_dock/app 之外仅历史注释提及，已改写为卡号出处引用）。

## 六、端测边界声明

- 拆除断言到 registry/dock_root 实例+源码+全树扫描三面；渲染面（特写窗 Detail 单项切换观感/拆除后布局零漂移目检）未过浏览器级 DOM 断言（E2E-WEBUI-1 未就位）；车道三组绘制为 headless 生产 _gui_input 事件注入（无真实键鼠焦点路径）。
- 用户手测节点=本链合入后（裁定链「实施后用户手测节点终验」）：特写窗打开路径（方块按钮/剪辑/母带）目检+车道三组绘制手感+Ctrl+Z/Y undo 手感。
- S0 裁决的 send 组启用实现（若主管裁定扩内核）归后续卡；本卡交付=占位禁用+探测证据。

## 七、工件清单（随 port/fe-envelope-teardown-l2-1-stack 分支入库）

- `console/envtd_stack_leg.ps1`（driver）+`envtd_check_stack.ps1`（核对脚本）+`ps1_minimal.log`（最小腿 exit 0+PREFLIGHT_CLEAN 18:48:56）+`probe_r1.log`/`probe_r2.log`（44/44×2 全量含 SUMMARY_JSON+send_addressability 裁决）
- `offline_regression/`（22 件离线探针日志：新拆除探针 50 检+全家回归 21 件）
- `project/default_project.xml`（VIT_PROJECT_XML 隔离副本=内核写入留证 SHA1 ce0f6543）
- RESULTS.md（本件）
