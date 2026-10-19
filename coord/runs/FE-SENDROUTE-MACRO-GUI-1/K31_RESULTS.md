# FE-SENDROUTE-MACRO-GUI-1（130）K3.1 返工收口（2026-10-19）

范围=卡面「K3.1 范围」：修①空白右键菜单坐标 + 复验②selector 可见性 + ③侧栏宏调取第一入口。
分支续 port/fe-sendroute-macro-gui-1，前端实现 commit **1e33284**（基线 6404a52）。

## ① 空白右键菜单坐标（根因钉死→运行时取证定谳→修复→验证）

- **取证**（k31_forensic/，headless+窗口态两轮，Godot 4.6.1）：`popup_on_parent` 实测把
  rect.position 当**主窗口 viewport 坐标**（菜单屏幕位置=主窗口原点+arg 原样，
  parent=GraphEdit 的全局换算未发生）。:1037 传 GraphEdit 局部坐标 → 菜单系统性
  偏左 GraphEdit 全局偏移量 = 用户症状「屏幕左侧固定位置」精确吻合。引擎
  GraphEdit 摆放公式与 `_graph_point_to_canvas` 逐位相等（zoom=0.8/scroll≠0 验证）；
  节点右键菜单一直正确因其优先走 `_show_embedded_context_menu`（内嵌 Panel 吃局部系）。
- **修复**（Vit_Graph_Rack.gd）：:1037 第三参 `get_global_transform() * mb.position`
  （local→viewport，对齐 :6478 先例式）；`_open_canvas_action_menu` 双轨——popup 收
  viewport 坐标，下游三入口（装载插件弹窗/发送选择器/新建宏）锚点 `affine_inverse`
  换算存回局部系（`_create_macro_at` 的 (pos+scroll)/zoom 语义不变）。
- **验证**：窗口态取证复跑（k31_forensic/windowed_run.log 末两行，修后代码）——菜单
  屏幕位置=主窗口+(700,400)=鼠标 viewport 坐标 **±0px**；`_canvas_action_menu_pos`
  存回局部 (400,300)/(100,200) 正确。
- 登记备查：`_open_node_context_menu` 的 PopupMenu fallback 分支（:6430）与
  popup_on_parent 口径同样不匹配，现状 embedded 分支常驻不触发——域外未动。

## ② send 选择器可见性（①传导症状，复验通过）

`rack_send_target_selector.open_at` 与 `rack_plugin_search_popup`（用户手测过的先例）
完全同模式（普通子 Panel position=clamp(局部)），无独立缺陷；修①后锚点回到正确局部
坐标。探针断言：selector 经画布动作菜单打开+position 在 GraphEdit 矩形内+近锚点
（±16px）三项全绿。

## ③ 侧栏宏调取第一入口（用户架构裁定）

- **Vit_Graph_Rack.gd 新桥方法**（与画布右键同一 control_* 命令面）：
  `vit_sidebar_create_macro`（control_add_macro，宏名 Macro N=快照 macropanel 数+1，
  落视野中心）/`vit_sidebar_create_control`（control_add_node，十词表 kind）/
  `vit_sidebar_control_nodes`（control_graph 快照读侧列表：node_id/name/kind/映射数）/
  `vit_sidebar_focus_control_node`（定位=选中+滚动居中）/
  `vit_sidebar_reapply_control_node`（施加=宏卡 control_set_macro_values 批量当前值/
  单控件卡 control_set_node_value 首输出）/`vit_sidebar_suggest_control_kind`
  （复用 rack_control_layer_rules.suggest_control_kind，选中插件首个选中参数特征）。
- **graph_rack_dock_adapter.gd**：「机架控件类型」三选横排按钮 → 十词表 OptionButton
  （中英标签）+建议行（建议 类型：参数名（原因）+「采用建议类型」）；「宏控件」区 →
  「创建宏（混排宏卡）」+「创建控件卡」+「调取已有控制」列表（点击定位/施加按钮）；
  统计走 control_graph；新增 control_graph_changed 监听刷新；旧链路
  （CONTROL_TYPE_CHOICES/RACK_MACRO_ACTIONS/_create_macro_for_track→
  create_rack_control_macro）移除——repository/command_service 宏文档层保留
  （vit_dock_root 2 处读侧+Vit_Graph_Rack 参数面绑定宏+回归测试仍在消费=「仅留兼容读」
  裁量，不触发停止条件）。
- view_state `rack_control_pref` 值域扩展（旧 slider/knob/toggle 均在十词表内，兼容）。

## 验收证据

- **探针**（tools/probe_rack_sendroute_macro.gd）：151→**178 断言全绿 exit 0**
  （+27：k31 坐标链路+锚点±2px/selector 可见性三项/侧栏桥 13 项/adapter 契约 8 项）。
- **回归电池 9 套零回退**：4 rack 测试（param_surface_panel_factory/
  project_repository_rack_macros/ipc_payload_rules/layout_rules）exit 0 +
  probe_rack_plugin_search_popup **111/111** + fe_gui_shrink a/b/c PASS。
- **真栈**：run=k31_sendroute_gui_20261019_1135，dev_agent_smoke -StartKernel
  -RestartAgent -SendRouteGuiSmoke **exit 0**，SUMMARY_JSON
  `{"checks_failed":0,"checks_total":25,"status":"pass"}`（六腿：connect auto_create/
  list+edge 读回/state aux 插件对/set_amount 双回读/宏五步+宏驱动 send amount/
  disconnect 归零+fx_released）+chat/confirm 腿过。被测 kernel=K3 自建
  **SHA256=722A79D7A3F53797F5713209D812D8CE9F2888CB2A7E743286FA56B99629DFEC**
  （11:29 复核一致）+腿内 go build agent+前端 @1e33284。
- **栈登记**：占用 4062a98d（11:31 预检六端口 tcp+udp=0/四类进程=0）→ 释放登记
  （11:4x，拆栈只杀本腿 VitAgent 27560/VitApp 7440，复核进程=0/六端口=0）。
- §8 记账：LLM=0；首轮真栈命令 bash 吞反斜杠未起栈即失败（无栈副作用，立即修正
  重跑，如实记录）。

## 端测边界声明（AGENTS §5/§7）

已覆盖=K3.1 三返工点的命令面/锚点换算链（headless 178 断言）+GUI 同源 payload 经
生产传输链×内核真实落链回读（真栈 25/25）+坐标修复的窗口态取证实证（菜单屏幕位置=
鼠标 viewport 坐标 ±0px）；**未覆盖**=窗口内真实指针交互下的菜单视觉确认（三入口
菜单位置/selector 弹出/侧栏工具页交互/调取列表点击）——归 **[等待用户] 手测复测**
（清单 K3.1 段已追加：coord/runs/FE-SENDROUTE-MACRO-GUI-1/MANUAL_TEST_CHECKLIST.md）。

## 工件清单

- k31_sendroute_gui_20261019_1135/（console+run_console）
- k31_forensic/（取证脚本+headless/windowed 两轮日志+README 结论）
- K31_RESULTS.md（本文件）
- patches/0001-…-K3.1（前端补丁，GH001 推送限制照旧）
