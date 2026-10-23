# FE-SEND-LEVEL-CLICK-FIX-1：发送量按钮真实鼠标点击无响应（178 返工——场次⑥手测实证）

- 发卡：GLM 主管决策流 / 2026-10-23 深夜（依据=场次⑥用户手测：五点中四点过[AB 每轮必现✓/链位连线✓/对间电缆✓/重启保持✓]，**发送量点击依然无响应**；用户观察=点击只触发插件卡拖拽，事件疑似未穿透到按钮）
- 派发确认：已确认（用户手测现场反馈）
- 验收负责人：GLM 主管决策流
- 池序 182；目标仓库=**前端仓 D:\Godot\project\vit-daw-frontend**；**P1（178 主目标未达，返工）** / 0.25-0.5 天 / 依赖=178 已合入（9015cfd）；模型分级：L2 / PC 前端执行流

## 根因面（主管桌面取证已完成）

- **探针覆盖缺口坐实**：stack_probe L10 与 headless 探针的「点击→弹窗」断言实际是 `pressed.emit()` **直发信号**（stack_probe_sendroute_macro.gd:415），完全绕过真实鼠标事件管线（viewport pick→Button._gui_input）——断言名「点击」与覆盖面不符，178 端测边界声明已预留此缺口，本次手测实证坐实。**与 171「断到 DOM 未断交互」同族第 2 例。**
- handler 链本身通（emit 直调下弹窗/payload/读回全过）；对卡层级无覆盖层（Container 全 PASS/Button STOP）；`_refresh_interactive_descendants` 递归启用按钮。剩余候选根因需窗口态取证：
  ①鼠标 pick 未达 Button（GraphNode 拖拽接管 press/几何或 Z 序遮挡）——用户主诉方向；
  ②弹窗弹出位置错（anchor.get_global_rect() 在 GraphNode 变换/缩放下坐标错→弹到可视区外，用户感知=无响应；headless 曾出 grab_focus 警告）；
  ③真实外部写形态下 finder 未命中静默早退（探针形态已排除，真实形态 kernel_item 一致性待复核——handler `_find_plugin_graph_node_by_kernel_item` 未命中时**静默 return**，应加诊断日志）。

## 根因收敛（用户手测补证 2026-10-23 深夜二）

用户二次取证已把三候选砍剩一个：**「▣ 折叠」按钮与普通插件卡按钮均可点动**（整卡按钮层/GraphNode 吞事件排除）；**点击「发送量」后弹窗确有出现（高亮可见）**（按钮→handler→popup 链路通、位置在屏内）；**随后在弹窗上的点击/拖动直接移动了下层插件卡**——即弹窗可见但其区域鼠标事件完全穿透回画布，弹窗内 SpinBox/箭头收不到任何输入。与 headless 真栈跑出的 `grab_focus` 警告（popup 后 SpinBox「can't grab focus」）同族=**PopupPanel 输入/焦点层未正确建立**。首选排查方向：PopupPanel 作为 GraphEdit 子节点挂在 `add_child` 下，其输入层被 GraphEdit/GraphNode 事件处理路径抢占或 popup 层级未就绪；修复方向候选=弹窗挂载点移出 GraphEdit 子树（画布顶层 Control/CanvasLayer）或改用原生弹层（AcceptDialog/Window 族），保证输入层独立。

## 范围

①窗口态取证：真实鼠标事件注入（`Viewport.push_input` 构造 InputEventMouseButton 在按钮全局位置）观察事件走向（Button.gui_input 是否触发/GraphNode dragged 是否触发/弹窗 global 位置）——区分三候选；②修复（按取证结论：事件穿透修正/弹窗定位修正/handler 诊断与 fail-open 提示）；③**探针断言升级**：L10 与 K3 lvl178 断言改真实事件管线（push_input 于按钮 rect 中心），禁止 emit 直调冒充点击——该纪律升格为 FE 系探针通则；④窗口态手感=用户手测复验。

## 文件域

以取证结论固定（预期 `app/rack/Vit_Graph_Rack.gd` 邻域+`tools/stack_probe_sendroute_macro.gd`+`tools/probe_rack_sendroute_macro.gd`）。越域即停。

## 验收标准

窗口态真实鼠标注入断言（点击按钮→弹窗可见且在屏内→值可改→payload 读回）+既有探针零回退（K3 318/popup 111/shrink abc/start_feed 244）+真栈腿；工件入主仓 coord/runs/FE-SEND-LEVEL-CLICK-FIX-1/；用户手测复验点击可调。

## 停止条件

真实事件管线断言在 Godot 4.6 探针环境不可注入（headless 限制）→ 以窗口态 Godot 进程内自驱动取证脚本替代并如实声明边界。

- 领取：（时间 / origin hash / owner / 分支 / worktree / 领取提交）
- 回执：（根因裁定 / commit / 探针断言面 / 真栈 run ID / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
