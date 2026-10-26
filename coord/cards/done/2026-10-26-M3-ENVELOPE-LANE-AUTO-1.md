# M3-ENVELOPE-LANE-AUTO-1：包络车道三态自动档+自动展开腿（189 二段）
- 发卡：GLM 主管决策流 / 2026-10-26 晚（依据=189 验收裁定两段式——三态「自动」档+自动展开腿随 190 补验；190 已合入 main 87b032e2 ✓ 数据源就位；用户 2026-10-25 形态裁定「按需+手动混合」为此腿裁定源）
- 派发确认：已确认（189 验收裁定链+用户形态裁定）
- 验收负责人：GLM 主管决策流
- 池序 192；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**（领取基线 main=20873f8）；P3 / 0.5 天 / 依赖=190 合入 ✓（envelope_point_count 在场性字段）
- 模型分级：L1 / PC 执行会话
- 并行与资源：与 191 M3-ENVELOPE-RACK-BADGE-1（app/rack/）文件域不相交可并行；**真栈腿与 191 错峰**（§2.2 增补③——领取/登记前互查 PC-RUNTIME-STACK.md 与对方卡占用状态，同刻双登记禁止）；C4 勘察卡未开卡，无在飞冲突。

## 目标
1. **三态档位**：车道开关从两态（开/关）扩三态（关/手动/自动）——189 第一段已留口；手动档=现有行为；自动档=可见性由在场性驱动。
2. **自动展开腿**：描述符刷新链（rack_client.get_plugin_parameters 同源，189 车头参数选择器同链）拉取回执→任一参数 envelope_point_count>0→自动档下该轨车道自动展开；全部归零→自动收起；用户手动展开优先（手动档/手动展开不被自动收起，形态细节勘察定案）。
3. **覆盖面如实**：数据源=外部 rack VST 插件参数（190 覆盖面）；轨级参数（volume/pan）不在回执面=已知缺口（主管 2026-10-26 已裁定另裁数据源）——轨级参数不触发自动展开如实呈现，不硬造。
4. **回退**：默认档=现两态行为（自动档需用户显式选）；旧工程/旧状态零行为变化（AGENTS §11 缺省语义）。

## 文件域
`app/timeline/`+`app/tracks/track_scene/`（track_envelope_lane_2d.gd 三态档+Track_Row_Controller.gd 档位邻域）+探针新文件。**不触碰** `app/rack/**`（191 域）与 stack_probe_sendroute_macro.gd。越域即停。

## 验收标准
独立探针全绿（三态切换+自动展开/收起+手动优先+回退零变化）+既有探针零回退（189 envelope_lane 39/datachan 50、185 badge 36 新基线）+真栈腿 exit 0（§2.2 登记+三纪律+188 守卫）：装载插件→set_points 写点→自动档车道自动展开→clear_region→自动收起；工件入主仓 coord/runs/M3-ENVELOPE-LANE-AUTO-1/；端测边界声明。

## 停止条件
- 190 回执未按卡面到达前端消费链（envelope_point_count 缺失）→锚点上交；
- 三态档与既有让路链/仲裁器冲突超勘察预期→上交。

- 领取：2026-10-26 晚窗 / 主仓 origin/main=de8a607a / 前端仓领取基线 main=20873f8（worktree git status --short 干净亲核）/ owner=ZCode GLM-5.3 PC 执行会话（M3-ENVELOPE-LANE-AUTO-1 卡 192 专属）/ 前端分支 port/m3-envelope-lane-auto-1 / worktree D:/Godot/project/vit-daw-frontend-envauto1 / 领取提交=62993f54（首推与 191 领取同刻竞态被拒→§2.1.3 fetch 重读重放于 b88f3a1f 之上成功，顺序以 git 史为准）；领取前核对：191 尚在 todo（其领取后落）、doing 仅 GEN-SUNO-WEB-DESIGN-1（域不相交）、PC-RUNTIME-STACK 当前占用=空（最新释放 190@18:57）。
- 回执：2026-10-26 晚窗自验完成，交主管验收（详 [runs/M3-ENVELOPE-LANE-AUTO-1/envelope_lane_auto_20261026_2012/RESULTS.md](../../runs/M3-ENVELOPE-LANE-AUTO-1/envelope_lane_auto_20261026_2012/RESULTS.md)）——
  - **commit**：前端仓 `port/m3-envelope-lane-auto-1@6b66f9a`（5 文件：track_envelope_lane_2d.gd 三态档+在场性扫描+timeout 面 / Track_Row_Controller.gd 三态行 API / 独立探针+真栈探针 gd+tscn；**vit_track_lane_2d.gd/rack_client.gd/dev_agent_smoke.ps1 零改；不触 app/rack/**=191 域遵守**）；主仓 `port/m3-envelope-lane-auto-1-stack@036d61d9`（真栈工件+RESULTS.md）；均已推远端。
  - **实现面**：三态档 off/manual/auto——两态 `set_envelope_lane_open` 语义保持（开=manual/关=off，旧调用/旧工程零行为变化=卡面④）；auto 档可见性=envelope_point_count 在场性驱动：目录内每 rack 插件经 rack_client.get_plugin_parameters（189 车头同链同源，独立 rid `env_lane_presence_*`）拉回执，任一行 count>0→自动展开，全部成功且全零（含空目录）→自动收起（空目录=完整零=**轨级参数不在回执面如实不触发不硬造**=卡面③）；错误/超时=扫描不完整保持现态（正证据优先展开，负结论须完整扫描）；手动粘性覆盖（auto 档内手动展开/手动收起不被自动反向推翻，档位切换重置后恢复跟随——「形态细节勘察定案」落案）；重扫触发=进 auto 档/目录变化/automation 写回执/rack 拓扑回执（kernel_command_signal_bus 全量回执面，仅 auto 档，manual/off 零自动行为）；读回面 `get_auto_presence_state`（present/override/scan 计数/field_seen 披露）。
  - **探针断言面**：独立探针 `probe_envelope_lane_auto.gd`（headless -s 零栈）**41/41 exit 0**（五段：A 三态状态机+回退基线 / B 扫描求值[pending 不落结论·在场展开·全零收起·timeout 正证据保留·字段缺席如实零] / C 手动优先双粘性+档位切换重置 / D 触发链+防环+off/manual 零触发 / E 两态 API 回退零变化+rid 命名空间分离）；既有探针零回退 **11/11 exit 0**（卡面三项：189 envelope_lane **39**/datachan **50**/185 badge **36**+189 全家：K3 313/popup 111/realinput 15/fader 35/start_feed 244/shrink a·b·c）。
  - **真栈 run**：`envelope_lane_auto_20261026_2012`——ps1 最小腿 exit 0（188 守卫版，kernel 自启无 already listening）+`stack_probe_envelope_lane_auto.tscn` **24/24 连续两轮 exit 0**（r1/r2 SUMMARY_JSON pass）：装载 ValhallaSupermassive（plugin_search 真发现非回退）→**L1 field_seen=true（190 envelope_point_count 到达前端消费链=停止条件反证锚）+全零车道收起+内核对照一致**→L2 set_points 三平坦点（190 同款）→bus 全量回执→车道自动重扫→**自动展开（visible+hit 词表+override=0 纯自动）+内核对照目标行 count==3**→L3 clear_region→**自动收起+内核对照回零**→L4/L5 手动粘性双向（活栈）。kernel=stack worktree D:/Vit_DAW_wt_envauto1 自建 VitApp.exe sha256 5D31056A…（源=main tip e70bafc7 含 190 合入 87b032e2 源码亲核；190 的 85b4e8df worktree 已清理→按卡面「须重建」路径 CMake 自建 exit 0）；§2.2 登记 dbe40d88→即时复核（20:11:43 全零<60s）→释放 a34a6d91（20:13:17 复核全零）三纪律全执行；VIT_PROJECT_XML 隔离亲证（隔离副本被内核写 20:12:46；worktree 工程同哈希零触碰）；**与 191 错峰互查兑现**（其 19:56 释放记录在案）。
  - **披露**：①构建 note=worktree 缺 .godot 缓存（`--import` 生成）+CMake libzmq 下载网络受限（`FETCHCONTENT_SOURCE_DIR_*` 指主检出已拉源树只读复用，主检出零写入）；②独立探针三轮迭代=r1 环境（缺缓存 parse error）/r2 探针侧流程缺陷 5 败（计数面/混入带字段行/manual 开档清注入目录时序）/r3 全绿——**零实现返工**；③生产面目录新鲜度=与 189 同界（rack 触发回执时点快照，repo 快照更新链 project_loaded 不在本卡域）。
  - **端测边界声明**：独立探针=假传输零栈（FakeIpcClient 注入）；真栈=生产 GUI 传输链全链真实往返（VitIpcClient UDP 4445→agent bridge→内核 ZMQ）+真 track_row.tscn+真 autoload bus 面；**未覆盖**=渲染面像素级目检与用户旅程（E2E-WEBUI-1/JOURNEY-1 建设面）、C4 写回链（set_points/clear_region 为探针侧直调，车道本卡零写回=189 边界保持）、OS 级事件注入、多轨并发在场性、轨级参数触发（主管已裁另裁数据源）。
- 验收：**整卡 pass**（GLM 主管决策流 2026-10-26；rulings/2026-10-26-M3-ENVELOPE-LANE-AUTO-1-pass.md；前端合入 53293a5；189 二段义务收口=189 卡全链闭环）
