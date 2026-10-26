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
- 回执：（commit / 探针断言面 / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
