# M3-ENVELOPE-LANE-AUTO-1（卡 192，189 二段）真栈腿回执

- run：`coord/runs/M3-ENVELOPE-LANE-AUTO-1/envelope_lane_auto_20261026_2012`
- owner：ZCode GLM-5.3 PC 执行会话（ZCode，192 卡会话）
- 领取：主仓 claim 提交 62993f54（origin/main=de8a607a 时点，与 191 领取 b88f3a1f 同刻竞态→§2.1.3 重放于其上成功）；前端仓领取基线 main=20873f8
- 实现：前端仓 `port/m3-envelope-lane-auto-1@6b66f9a`（5 文件：lane 三态档+在场性扫描+controller 行 API+两探针；vit_track_lane_2d.gd/rack_client.gd/dev_agent_smoke.ps1 零改；不触 app/rack/**=191 域遵守）
- 栈占用：登记 dbe40d88（20:11）–释放 a34a6d91（20:13）；与 191 错峰互查兑现（其 19:56 释放记录在案，登记前 fetch 重读当前占用空）
- kernel：stack worktree D:/Vit_DAW_wt_envauto1 自建 VitApp.exe（源=main tip e70bafc7 含 190 合入 87b032e2——PluginRackControlService.cpp:652 appendEnvelopePointCountsToParameterDescriptors 亲核在源码；CMake configure exit 0+build exit 0，sha256 5D31056A0306292D0334913CE54D595B8FFD0BF49D0F2CAE6B9BC214663DEB9A；190 卡 85b4e8df worktree 已清理不可复用→按卡面「须重建」路径自建）；agent=stack worktree 构建 pid 22708；kernel pid 1904 本腿 ps1 自启
- 构建 note：worktree 缺 .godot/导入与类缓存（首轮 -s 探针 parse error 复现后 `--import` 生成）；CMake FetchContent libzmq 下载网络受限→`FETCHCONTENT_SOURCE_DIR_LIBZMQ/CPPZMQ` 指主检出已拉源树只读复用（主检出 build 目录零写入）

## 判定面

### ① 独立探针（无栈 -s，零 autoload 零 UDP）

`tools/probe_envelope_lane_auto.gd` **41/41 PASS exit 0**（五段：A 三态状态机+回退基线默认 off/关；B 在场性扫描求值——rid 命名空间 env_lane_presence_*、pending 不落结论、任一行 count>0 自动展开、全部成功且全零自动收起、timeout=不完整正证据保留、字段缺席=如实零+field_seen=false；C 手动优先——auto 内手动展开/收起双粘性+关闭按钮同路径+档位切换重置后恢复跟随；D 触发链——automation 写回执触发重扫、rack 拓扑回执重读目录、描述符回执自身不触发防环、off/manual 零触发；E 两态 API 回退零变化——开=manual/关=off、车头 env_lane_params_* 命名空间不受影响）。

### ② 既有探针零回退（11/11 PASS exit 0）

189 envelope_lane **39**/datachan **50**/185 badge **36**（卡面三项基线）+ K3 sendroute_macro 313/popup 111/realinput 15/fader 35/start_feed 244/shrink a·b·c（189 全家口径）。

### ③ 真栈腿（生产 GUI 传输链全链）

ps1 最小腿 exit 0（-StartKernel -RestartAgent -NoChatSmoke -NoStripSilenceSmoke，188 守卫版，无 already listening=kernel 自启亲证）；`stack_probe_envelope_lane_auto.tscn` **24/24 连续两轮 exit 0**（r1 20:12:31/r2 20:12:5x，SUMMARY_JSON status=pass 双源在卷）：

- setup：clear_project→add_track→plugin_search ValhallaSupermassive（真发现非回退）→rack_add_node 装载外部 VST→描述符预热→目标参数 dry level（190 锚点）
- **L1**：auto 档进场→车道自发在场性扫描（rack_client.get_plugin_parameters 同链同源）→**envelope_point_count 到达前端消费链（field_seen=true，卡面停止条件反证）**→全零=车道保持收起（visible=false+IGNORE+hit 空）+内核直拉对照全零一致
- **L2**：探针侧 automation.set_points 三平坦点（v=当前值，190 同款）→kernel_command_signal_bus 全量回执（VitIpcClient.command_completed）→车道自动重扫→**在场=true→车道自动展开**（visible+hit 词表 envelope_lane+override=0 纯自动非手动）+内核直拉对照目标行 envelope_point_count==3
- **L3**：automation.clear_region 全域→车道自动重扫→**全零→车道自动收起**（visible=false+hit 空+IGNORE+override=0）+内核对照全行回零
- **L4**（活栈手动优先）：auto 档内手动展开（两态 API）→全零重扫不收起（粘性覆盖 override=1）
- **L5**（活栈手动关）：手动收起→再写点在场=true 重扫不重开（粘性覆盖 override=-1）
- cleanup clear_project

### §2.2 三纪律+188 守卫

a) 登记 dbe40d88 推送后 20:11:43 即时复核进程/端口全零（距 ps1 启动 <60s）；b) 无 already listening（kernel 1904 本腿 ps1 自启）；c) 登记先于启动；188 守卫版 ps1（守卫①外来栈显式中止在卷）。VIT_PROJECT_XML 隔离生效亲证：隔离副本 sha 947E4D00→33F2889C（末次写 20:12:46=内核自动保存）；worktree Workspace/default_project.xml 运行前后同哈希 947E4D00（mtime 19:27:26=零触碰）+Settings.xml 未动。拆栈=owner 手拆 VitAgent 22708/VitApp 1904（路径亲核归属）→20:13:17 复核四类进程=0、五端口 TCP listen=0、UDP 4444/4445=0。

## §8 记账

LLM=0（全确定性腿）；运行次数=真栈 1 轮预约 2 轮全 PASS（189 连续两轮先例）；失败分类=无；独立探针三轮迭代（r1 环境=worktree 缺 .godot 缓存 parse error；r2 探针侧流程缺陷 5 败[A4 计数面/B7 混入带字段行/C7·D1 manual 开档清注入目录时序]，全部探针侧修正零实现返工；r3 41/41）。

## 端测边界声明

- 独立探针=假传输零栈（FakeIpcClient 注入回执）；真栈腿=生产 GUI 传输链全链真实往返（VitIpcClient UDP 4445→agent bridge→内核 ZMQ）+真 track_row.tscn+真 autoload bus 面
- **未覆盖**：渲染面像素级目检与用户旅程（E2E-WEBUI-1/JOURNEY-1 建设面；车道观感随 M3 手测场次）；C4 写回链（set_points/clear_region 为探针侧直调，车道 UI 本卡零写回=189 边界保持）；OS 级事件注入；多轨并发在场性；轨级参数（volume/pan）触发自动展开=已知缺口（主管 2026-10-26 裁定另裁数据源，本卡如实不硬造——空目录/缺字段均完整零收起形态）
- 生产面目录新鲜度边界（与 189 同界）：rack 拓扑触发时重读 repo 目录（回执时点快照，repo 快照更新由 project_loaded 链驱动不在本卡域）；探针场景无 repo autoload=注入面（189 同款）
- 设计定案披露（卡面「形态细节勘察定案」义务）：手动展开/手动收起均为粘性覆盖直至反向手动动作或档位切换（对称语义，自动只在无覆盖时驱动）；错误/超时=不完整扫描保持现态（正证据优先展开、负结论须完整扫描）——防抖动 fail-closed
