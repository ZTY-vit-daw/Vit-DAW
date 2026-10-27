# FE-AUTOLANE-PRODCHAIN-FIX-1（205）扩域续作真栈回执——prodchain_20261027_2155

- 卡：[FE-AUTOLANE-PRODCHAIN-FIX-1](../../../cards/done/2026-10-27-FE-AUTOLANE-PRODCHAIN-FIX-1.md)（池序 205；首腿 8791405c 领取→blocked aa1c5939→主管裁决 9b7885ff 扩域→续作领取 a3341b25）
- 执行：GLM-5.3 PC 执行会话（ZCode，原 owner 续作）；2026-10-27 21:36 领取 – 22:0x 收口
- 被测=前端 worktree `D:/Godot/project/vit-daw-frontend-autofix1` @ `port/fe-autolane-prodchain-fix-1` @ **d611b21**（已推远端；ed64f0f 首腿实现+8295ca4 探针加固/诊断件+d611b21 扩域续作）
- 续作范围（主管裁决 2026-10-27）：只做扩域面+重放；首腿已绿交付面①②③不重做。

## 一、扩域修复（vit_dock/core/project_repository.gd 单点，主管裁决授权域）

- **并源形态=解析器兜底+纯增键**（裁决两案中取最小面）：`apply_kernel_project_state` 轨行 plugins 全量留存于新快照键 **`plugins_by_track`**（rack 分支/无 rack 分支均留存；既有键零破坏——`rack_nodes`/`tracks`/`clips`/`rack_by_track` 语义与内容不变）；`get_volume_pan_plugin_item_id_for_track` 在 rack 候选后追加 `plugins_by_track` 候选（同一过滤；legacy 空候选 `get_track().plugins` 路径保持原样）——racked 轨（rack 字典独占 rack_nodes）轨级 Volume & Pan 插件经并源可达。
- **既有消费方零破坏亲证**：rack 宏测试 `app/rack/tests/test_project_repository_rack_macros.gd` PASS（含 rack 投影 apply→get_rack_item_id/get_rack_nodes 断言面，`rack_macro_test.log`）；离线 K1 断言 racked 形态 rack_nodes 仍= rack.nodes 恰 VST（rack 分支语义保持）。
- **两消费面收口**（裁决回归义务）：①车道轨道组+默认音量车道；②轨道头声相推子 IPC（racked 轨解析恒空=预存静默 no-op 缺陷随修收口）。

## 二、离线红绿（offline_regression/ 27 件）

- **红（修复前，d611b21 前工作树仅含 K 段探针）**：`prodchain_k_red.log`——**K2 并源解析空/K3 无 plugins_by_track 键/K6 车道轨级插件 id 空/K7 无默认音量车道四面红**+K 段中止被段标记捕获，exit 1（56 检 5 失败）。
- **绿（修复后）**：`probe_autolane_prodchain` **65/65 exit 0**（首腿 47+K 段 18）。K 段=racked 真内核形态（rack 字典+plugins 并存，stack_diag_205_rackshape 证据同构）+rack 分支零破坏+三组在场（轨道恰 volume/pan+发送占位+插件组=VST 实参）+默认音量车道落定（轨道·音量）+**声相门红绿**（K14 解析=3004 修复前空；K15 空解析=零派发 no-op 形态；K16 解析达=set_plugin_param{plugin_id:3004,param_id:pan,value:0.625} 派发）。
- **全家回归 23 件全绿零回退**（与首腿同集：rebuild 73/edit 63/datachan 50/lane_auto 25/lane_toggle 27/lane_ui 36/teardown 50/shrink a/b/c 87+119+342/fx_wet 36/rack 四件 111+35+15+313/start_feed 244/vitnote 六件；三件 verdict 形态=start_feed/input 与首腿逐字节一致+circle_state 仅运行时实例 id 差异）。

## 三、真栈腿（§2.2 登记+三纪律+188 守卫+VIT_PROJECT_XML 隔离+错峰手测节）

- **登记**：c13a7f78 推 main 成功+远端重读确认置顶（21:55 登记）；登记前只读核对 21:55 进程/端口全零+fetch 重读占用空（最新释放=205 首腿@21:21）+错峰核对 doing 仅本卡+GEN-SUNO（无栈腿）。
- **三纪律**：a) 登记 c13a7f78 推送 21:56:50 后 **21:57:07 即时复核**（远端亲读置顶+进程/端口全零，17s<60s 窗口）+driver PREFLIGHT_CLEAN 二次预检（ps1_minimal.log）；b) 无 already listening（kernel 11408/agent 16528 均本腿 ps1 -StartKernel 自启，188 守卫版脚本）；c) 登记先于启动。
- **栈形态**：kernel=**205 首腿自建二进制只读复用**（sha256 **8440636F**…与首腿回执同源；扩域零内核改动亲核——续作全部改动在前端仓）；agent=stack worktree 构建（Go 零改动）；被测=前端 `autofix1@port/fe-autolane-prodchain-fix-1@d611b21`。
- **R1b 重放连续两轮全绿**：**R1 34/34 + R2 34/34 均 exit 0（零修正轮）**——主管裁决验收面全通：
  - setup 装载 ValhallaSupermassive（真发现 fallback=false）+描述符预热；
  - R 真 repo（真内核回执生产写入路径）+**R3 轨级 volume 插件解析**（首腿红面→绿）；
  - A 真行装配+生产绑定 registry 落册+**A5 懒重建即推 id（205 修复面）**；
  - S 展开生产链三组（禁注入）：目录从真 repo 到达+**S4/S5 默认音量车道生产链落定（描述符 bus 真实回执+get_curve EMPTY 披露）**+S7 轨道组恰 volume/pan+S8 发送占位+S9 插件组=实载 VST 实参+弹层定位断言面 S10-S13；
  - W 建点读回：W1 手势零写+W2 释放恰一笔+W3 回显门+**W4 内核 get_curve 读回逐点一致（t,v,c）**；
  - **W5-W6 声相推子 IPC 写读对照（消费面②真栈收口）**：生产路径 `_header_send_pan_ipc`→repo 并源解析→`set_plugin_param`→内核 `get_plugin_parameters` pan 行 **normalized_value=0.625 读回一致**（修复前=静默 no-op 恒中心位）；
  - cleanup clear_project。
- **VIT_PROJECT_XML 隔离亲证**：run 副本 git-clean 基线 **287956e7**（=202/203/205 首腿同基线，hash-object 亲核）→内核写入 **c6105261**（mtime 21:58=本腿运行期=隔离生效）；stack worktree `VitApp/`+`agent/` 运行后 git 干净；主检出 default_project.xml 78b828ee（mtime 19:10=本腿前既有值零触碰）。
- **拆栈**：owner 手拆 kernel 11408/agent 16528（路径亲核=worktree 自建）→21:59:00 复核 VitApp/VitAgent/VspHub/Godot=0、TCP 5555/5556/7878/8787/18787 listen=0、UDP 4444/4445=0。释放记录 869ce03a。

## 四、端测边界声明（承首腿，扩域面无新增豁免）

- 渲染面（弹层真实窗口落点/观感）未过自动化断言（headless 无窗口面；policy 单测+鼠标源集成断言=卡面等价断言面），留用户手测节目检；手感面（车道编辑/弹层点击/声相推子拖动手感）留用户手测节。
- 声相写读=数值面断言（normalized_value），推子 UI 拖动链（slider→handler）经生产方法直调驱动——真实拖动手感留手测。
- 首腿端测边界（弹层落点/渲染面）不变。

## 五、工件清单

- `console/`：`prodchain_stack_leg.ps1`（driver，188 守卫+PREFLIGHT）+ `ps1_minimal.log`（最小腿 exit 0）+ `probe_r1.log`（34/34）+ `probe_r2.log`（34/34）。
- `offline_regression/`：27 件（全家 23 件全绿日志+K 红绿三件 `prodchain_k_red/green/green2`+repo 消费方 `rack_macro_test`）。
- `project/default_project.xml`：隔离副本（287956e7→c6105261 内核写入=隔离生效亲证）。
- 工件入库分支：**port/fe-autolane-prodchain-fix-1-stack**（主仓，续首腿 326dcd9e）；实现分支：**port/fe-autolane-prodchain-fix-1@d611b21**（前端仓，已推远端）。
- 首腿工件：`../prodchain_20261027_2050/`（原 run 不覆盖）。
