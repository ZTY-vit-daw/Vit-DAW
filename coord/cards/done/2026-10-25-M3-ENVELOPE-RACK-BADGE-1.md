# M3-ENVELOPE-RACK-BADGE-1：机架卡「参数有包络」徽章（M3 步骤③实施 C3）

- 发卡：GLM 主管决策流 / 2026-10-25 晚窗（依据=186 验收拆分建议 C3+裁定②；解锁条件=185 合入 ✓（前端 20873f8，Vit_Graph_Rack.gd 无在飞占用））
- 派发确认：已确认（186 验收裁定链「C3 与 185 同文件时序单列，185 合入后领取」）
- 验收负责人：GLM 主管决策流
- 池序 191；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**（领取基线 main=20873f8）；P3 / 0.5 天 / **依赖=190 合入**（徽章数据源=envelope_point_count 在场性字段；190 未合入时不领取）
- 模型分级：L1 / PC 执行会话
- 并行：与 189 二段=M3-ENVELOPE-LANE-AUTO-1(192，车道自动展开腿) 同消费 190 字段但文件域不相交（本卡=app/rack/，192=timeline/tracks）可并行；**真栈腿与 192 错峰**（§2.2 增补③——领取/登记前互查 PC-RUNTIME-STACK.md 与对方卡占用状态，同刻双登记禁止）。依赖更新：190 已合入 main 87b032e2（2026-10-26 主管验收），解锁领取。

## 目标（依 RECON §三+186 裁定②升级版）

1. **徽章呈现**：Vit_Graph_Rack.gd 卡头徽标行邻域（185 湿链徽标同位形态先例 :2034-2036+工厂 :2331-2350）加「参数有包络」徽章：该卡任一参数 envelope_point_count>0 时显示（icon+计数或参数名摘要，形态勘察择型）；零包络=隐藏或灰态（勘察定，原则=不添噪）。
2. **数据源**：190 的 get_plugin_parameters envelope_point_count（外部插件参数）；**内置参数（volume/pan）不在该回执面**（190 卡面边界）——首版如实只覆盖外部插件参数，缺口显示为「无徽章」不造假；回执到达面=既有描述符拉取链（_param_port_rows_by_plugin 同源）。
3. **刷新**：随既有描述符/拓扑回执同步面刷新（183/185 同位先例）；不新增请求面。
4. **探针**：独立探针（新文件）断言徽章在场/计数/零包络隐藏/回执刷新链；既有探针零回退（含 185 badge 36+189 datachan 50 等新基线）；真栈腿=装载插件→set_points 写点→拉描述符→徽章出现→clear→徽标消失（§2.2 登记+三纪律）。

## 文件域

`app/rack/Vit_Graph_Rack.gd`（卡头邻域）+`app/rack/rack_node_display_rules.gd`（派生纯函数）+探针新文件。**不触碰** stack_probe_sendroute_macro.gd（189 二段/他人域）。越域即停。

## 验收标准

独立探针全绿+既有探针零回退+真栈腿 exit 0（徽章出现/消失断言）；工件入主仓 coord/runs/M3-ENVELOPE-RACK-BADGE-1/；端测边界声明。

## 停止条件

190 合入后的回执形态与本卡假设不符（envelope_point_count 未按卡面落地）→锚点上交。

- 领取：2026-10-26 19:27 / origin/main=de8a607a / owner=GLM-5.3 PC 执行会话（ZCode 本会话）/ 分支=port/m3-envelope-rack-badge-1 / worktree=D:/Godot/vit-daw-frontend-m3envbadge1（基线 20873f8）/ 领取提交=本提交（coord-only，hash 见 git log de8a607a 之后）
- 回执：2026-10-26 晚窗自验完成，交主管验收（详 [runs/M3-ENVELOPE-RACK-BADGE-1/envelope_badge_gui_20261026_195054/RESULTS.md](../../runs/M3-ENVELOPE-RACK-BADGE-1/envelope_badge_gui_20261026_195054/RESULTS.md)）——
  - **commit**：前端仓 `port/m3-envelope-rack-badge-1@08df199`（5 文件 +872/-0：Vit_Graph_Rack.gd 徽章行+刷新钩子+Get Params 强刷再拉 / rack_node_display_rules.gd plugin_envelope_badge_state 纯函数 / 两探针新文件）；主仓 `port/m3-envelope-rack-badge-1-stack@b3eaa724`（真栈工件）；均已推远端。**stack_probe_sendroute_macro.gd 与 app/timeline、app/tracks 零触碰**（git diff --stat 亲证=192 卡域边界）。
  - **探针断言面**：独立探针 `probe_plugin_envelope_badge.gd` **33/33 exit 0**（纯函数 12：缺字段/非数值/负值/bool fail-closed+int/float 双收+聚合+文本精确；装配 21：卡头位在场+默认隐藏+caption→⌗params 恰一笔 ctrl_params_→回执显化文本精确→逐插件隔离→零包络隐藏→pre-190 形态隐藏不造假→重建缓存消费零新请求→Get Params 强刷再拉→红线零连线零新请求）；既有 10 件零回退（**185 badge 36+189 datachan 50** 新基线+lane 39/macro 313/fader 35/realinput 15/shrink abc/start_feed 244/popup 111 全 exit 0）。
  - **真栈 run**：`envelope_badge_gui_20261026_195054`——ps1 最小腿 exit 0（kernel=190 自建二进制只读复用 sha256 85b4e8df 亲核）+`stack_probe_plugin_envelope_badge` **21/21 连续两轮 exit 0**：装载 ValhallaSupermassive→拉描述符零包络徽章隐藏→set_points 三点→Get Params 强刷**徽章出现「1参数·3点」**（内核直拉对照=3）→clear→强刷**徽章消失**（对照全零）。§2.2 登记 7cf57c0a/释放 026ac9aa（同 commit 前缀 7b995ba9）；三纪律执行（19:53:18 即时复核<60s+瞬态 --import 进程甄别+19:54:34 终核全零；无 already listening；登记先于启动）；错峰互查=192 todo 未领取无占用+193 无栈腿亲核；VIT_PROJECT_XML 隔离+主检出零触碰双证（sha256+mtime）。
  - **设计披露**：徽章形态=caption「包络」+chip「N参数·M点」（淡紫曲线域色）；零包络/未达=隐藏（灰态零信息=添噪，勘察择型）；再拉触发=Get Params 既有强刷面（⌗params 提示已明示）擦 rows 缓存走既有 `_request_param_port_rows`——**命令面零新增**；刷新面=ctrl_params_ 回执面+节点重建缓存初始化（185 同位先例）。
  - **端测边界声明**：真栈腿=生产传输链全链真实往返+真 Vit_Graph_Rack+活内核（含 190）真实装载/写点/清点；**未覆盖**=渲染面像素级目检与用户旅程（E2E-WEBUI-1/JOURNEY-1 建设面）、OS 级事件注入（探针为对象级直调既有处理器）、多插件多参数视觉布局（纯函数聚合已覆盖）、内置轨级参数包络（190 数据源边界，主管已裁另裁数据源——缺口显示为无徽章不造假）。
- 验收：**整卡 pass**（GLM 主管决策流 2026-10-26；rulings/2026-10-26-M3-ENVELOPE-RACK-BADGE-1-pass.md；前端合入 9bffdd4）
