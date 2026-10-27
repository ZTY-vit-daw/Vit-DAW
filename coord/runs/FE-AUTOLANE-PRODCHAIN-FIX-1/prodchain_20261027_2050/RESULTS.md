# FE-AUTOLANE-PRODCHAIN-FIX-1（205）真栈腿回执——prodchain_20261027_2050

- 卡：[FE-AUTOLANE-PRODCHAIN-FIX-1](../../../cards/blocked/2026-10-27-FE-AUTOLANE-PRODCHAIN-FIX-1.md)（池序 205，手测节缺陷修复；领取 8791405c@main → 397176ea）
- 执行：GLM-5.3 PC 执行会话（ZCode）；2026-10-27 19:22 领取 – 21:2x 收口
- 被测=前端 worktree `D:/Godot/project/vit-daw-frontend-autofix1` @ `port/fe-autolane-prodchain-fix-1` @ **8295ca4**（已推远端；ed64f0f 实现 + 8295ca4 探针加固/诊断件）
- 交付面：①懒创建即推轨道 id（`get_track_envelope_lane()` 慢路径 `_ensure_envelope_lane_2d_layer(false→true)`——覆盖「行已绑→后创建」时序）；②弹层 canvas→屏幕坐标（选择器/曲率预设两处改 `_popup_menu_screen_position()`：embed 两态取实时鼠标位置，对齐 `timeline_dock_adapter._popup_menu_position_from_viewport_global` 既有先例；headless 守卫保持；`popup_position_policy` 纯函数=断言面）；③无注入生产链探针（本卡核心义务）；另发现并修复 `add_lane` 分组分歧（轨级 volume 插件优先轨道组——生产链探针发现，注入态探针目录不含 vp1 掩盖）+`get_kernel_track_id()` 读回面。

## 一、离线探针（无栈腿，-s 模式，offline_regression/ 26 件日志）

- **新探针 `tools/probe_autolane_prodchain.gd` 47/47 全绿 exit 0**（R 真 repo 生产链源：ProjectRepository 生产类+`apply_kernel_project_state` 生产写入路径+rack_nodes 快照+轨级插件 id 解析；A 真行装配+懒创建 id 时序（缺陷①回归面）；S 展开生产链三组（**禁注入**：目录唯一来源=真 repo；轨道组恰 volume/pan+发送占位禁用+插件组实载；描述符请求恰发 repo 源）；P 弹层定位断言（policy 两态纯函数+实例集成鼠标源≠canvas 旧源+headless 守卫×2）；W repo 生长→rack_add_node 总线回执→生产重读；E fail-closed（repo 在场轨无行→仅发送占位））。
- **基线负对照（红绿证据）**：`negctl_base.log`（f52b78f 纯基线上懒重建容器 kernel id=**空**=缺陷①实证）vs `negctl_fixed.log`（修复分支=**1001**）；主探针在基线亦结构性红（`prodchain_base_red.log`：`get_kernel_track_id` 不存在+S2 目录空链断言不可达）。
- **全家回归 23 件全绿零回退**（与 204 收据 21 件同集+新增本卡探针）：autolane_rebuild **73** / autolane_edit **63**（202/203 面，检数与 203 收据同数）/ automation_datachan 50 / lane_auto 25 / lane_toggle 27 / lane_ui 36 / envelope_teardown 50 / fe_gui_shrink a/b/c 87+119+342 / fx_wet_chain 36 / rack 四件 111+35+15+313 / start_feed 244 / vitnote 六件（input 与 204 基线逐字节一致；circle_state 仅运行时实例 id 差异）。
- **注入边界标注 8 件**（卡面：既有注入态探针不删）：probe_autolane_rebuild/edit、probe_envelope_lane_toggle/auto/ui、stack_probe_autolane_rebuild/edit、stack_probe_envelope_teardown——逐件在注入点标注【注入边界（205 标注）】+指向 prodchain 探针收口。

## 二、真栈腿（§2.2 登记+三纪律+188 守卫+VIT_PROJECT_XML 隔离+错峰）

- **登记**：76619335 推 main 成功+远端重读确认置顶（20:49 登记）；登记前只读核对 20:49 进程/端口全零+fetch 重读占用空（最新释放=204@18:52）+错峰核对 doing 仅本卡+GEN-SUNO（Mac 设计卡无栈腿）；注记用户主检出 Godot editor（14072）在场不占栈端口未动。
- **三纪律**：a) 登记 76619335 推送 20:49:42 后 **20:49:54 即时复核**（远端亲读置顶+进程/端口全零，12s<60s 窗口）+driver PREFLIGHT_CLEAN 二次预检（ps1_minimal.log）；b) 无 already listening（kernel 18076/agent 27524 均本腿 ps1 -StartKernel 自启，188 守卫版脚本）；c) 登记先于启动。
- **栈形态**：kernel=stack worktree `D:/Vit_DAW_wt_autofix1_stack`（397176ea=origin/main）自建 VitApp.exe（**内核面与 204 同源零改动亲核——4207650f..397176ea 零 VitApp/ 提交**（含 196 automation 三命令+201 轨级描述符行+undo 事务）；CMake configure+build exit 0，sha256 **8440636F**…；VIT_TRACKTION_ENGINE_DIR=主检出只读；libzmq/cppzmq=FETCHCONTENT_SOURCE_DIR 主检出 _deps 源只读复用零网络）；agent=stack worktree 构建（Go 零改动）；被测=前端 `autofix1@port/fe-autolane-prodchain-fix-1@8295ca4`。
- **腿程**：ps1 最小腿 exit 0 → R1 首试（19 检：核心面全绿，S4 空条崩溃挂看门狗 600s，`probe_r1.log` 留证）→ **探针侧加固**（S4 空条守卫+S4b 阻塞面披露+cleanup，零生产改动）→ **R1b 20 检干净收尾**：**17 绿+3 红（同一根因）**，exit 1=阻塞面如实红，无挂起（`probe_r1b_hardened.log`）→ rackshape 诊断 30 行逐字留证（`diag_rackshape.log`）。
- **核心面真栈实证（全绿）**：setup 装载 ValhallaSupermassive（plugin_search 真发现 fallback=false）+描述符预热；R1 生产写入路径 applied（真内核回执）；R2 repo rack_nodes 按轨键型含实载 VST；**A2 生产绑定 bind_virtual_track 在真 registry autoload 落册**；A3 198 补丁路径 id 到容器；**A5 懒重建即推行当前 id（205 修复面，修复前恒空=用户手测根因）**；S0/S1 开关生产入口展开；**S2 目录从真 repo 无注入到达（生产读链）**。
- **VIT_PROJECT_XML 隔离亲证**：run 副本 git-clean 基线 **287956e7**（=202/203 同基线，hash-object 亲核）→内核写入 **072ac158**（mtime 21:12=本腿运行期=隔离生效）；stack worktree `VitApp/`+`agent/` 运行后 git 干净；主检出 default_project.xml 78b828ee（mtime 19:10=本腿前既有值零触碰）。
- **拆栈**：owner 手拆 kernel 18076/agent 27524（路径亲核=worktree 自建）→21:21 复核 VitApp/VitAgent/VspHub=0、TCP 5555/5556/7878/8787/18787 listen=0、UDP 4444/4445=0；登记时在场的用户 editor（14072）于拆栈前已自行退出（非本腿处置）。释放记录 d7c8d2d8 随本回执同批推送。

## 三、停止条件分支触发（锚点上交，主管裁决）

**卡面停止条件①「真 repo 快照形态与探针假设不符（rack_nodes 键型/结构漂移）」命中。**

- **缺陷链（真栈实证+逐字留证）**：`rack_add_node` 装载 VST 后，内核 `get_project_state` 轨行**同时含** plugins 数组（`New Rack/rack` + `Volume & Pan Plugin/volume` + `Level Meter/level`）与 rack 字典（nodes=VST）；`vit_dock/core/project_repository.gd` `apply_kernel_project_state`（:106-116 一带）rack 分支**独占**→`rack_nodes[track]`=仅 VST→`get_volume_pan_plugin_item_id_for_track`（:879-910 一带）candidates 非空不再走 plugins 兜底，且兜底源 `tracks_out` 无 plugins 键=**死路**→**轨级 volume 插件 id 解析恒空**。
- **消费面（两处，均域内只读亲核）**：①车道目录轨道组+默认音量车道（`track_envelope_lane_2d.gd` `_set_catalog`→`_track_volume_plugin_id` 空→无轨道组/无默认车道——**用户手测「无音量/声相组」症状的第二层根因**，与缺陷①（容器空 id→全空）复合）；②轨道头声相推子 IPC（`Track_Row_Controller.gd` `_volume_pan_plugin_id_for_this_row`→`_header_send_pan_ipc` 空 id 即静默 no-op）。
- **锚点**：`vit_dock/core/project_repository.gd` `apply_kernel_project_state`（rack 分支）+`get_volume_pan_plugin_item_id_for_track`（candidates/兜底）；最小复现件=`tools/stack_diag_205_rackshape.gd/.tscn`（真栈可重复，须 kernel+agent 在 5555/4445；输出 30 行逐字留证 `console/diag_rackshape.log`）。
- **域判定**：修复点在 `vit_dock/**`=卡面明令不触域（「不触 vit_dock/**、app/rack/**。越域即停」）→按 AGENTS §11+卡面停止条件**锚点上交**，不越域修补。
- **对验收的影响**：卡面真栈验收「装载 VST→展开→选择器三组在场→默认音量车道绑定→建点→内核读回一致」的「三组在场+默认音量车道」链在 racked 轨上被该域外缺陷阻塞（无注入生产链如实红）；其余面（缺陷①修复真栈实证+缺陷②离线断言+离线生产链三组+全家回归）已交付。两轮全绿需主管裁决（扩域授权单点修复 project_repository.gd 或另开卡）后重放本探针（R1b 形态已就绪，repo 修复后预期 S 面/建点腿全绿）。

## 四、端测边界声明

- 渲染面（弹层真实窗口落点/弹层观感）未过自动化断言（headless 无窗口面）——弹层定位以 policy 两态纯函数+鼠标源集成断言交付（卡面「屏幕坐标映射单测或等价断言面」），真窗口落点留用户手测节目检。
- 探针断言到内部状态+真内核读回（W 建点腿因阻塞未达）；R1b 三红=同一域外根因的如实披露，非探针缺陷（R1 首试挂起=探针缺陷轮已加固留证）。
- 手测面（选择器弹层点击落点/车道交互手感）留用户手测节。

## 五、工件清单

- `console/`：`prodchain_stack_leg.ps1`（driver，188 守卫+PREFLIGHT）+ `ps1_minimal.log`（最小腿 exit 0）+ `probe_r1.log`（首试留证：核心面绿+S4 挂起）+ `probe_r1b_hardened.log`（加固后 17/20 干净收尾）+ `diag_rackshape.log`（停止条件形状留证 30 行）。
- `offline_regression/`：26 件（全家 23 件全绿日志+负对照 3 件红绿证据）。
- `project/default_project.xml`：隔离副本（287956e7→072ac158 内核写入=隔离生效亲证）。
- 工件入库分支：**port/fe-autolane-prodchain-fix-1-stack**（主仓）；实现分支：**port/fe-autolane-prodchain-fix-1@8295ca4**（前端仓，已推远端）。
