# FE-RACK-MACRO-LAYER-DESIGN-1 设计草稿 v1：机架宏控制层（两层模型）

- 决策侧：GLM-5.3 主管决策流（ZCode，2026-10-18 早窗开线）
- 状态：**v1 草稿，待用户样图迭代**（样图=[macro_layer_v1.html](macro_layer_v1.html)，对话式迭代同 send GUI v1-v4 模式）
- 上游：[MIX-SENDRETURN-DESIGN-1 §9.4](../../runs/MIX-SENDRETURN-DESIGN-1/README.md)（两层模型框架）+ 卡 129 现状盘点
- 视觉语言基线：[B2-RACK-VIZ-SKETCH](../../runs/B2-RACK-VIZ-SKETCH/rack_viz.png)（暗色工程规范风）+ send GUI v4 节点族（SEND tag 同款）

---

## 1. 现状盘点核验（2026-10-18 早窗决策侧亲核，含新发现）

### 1.1 内核（已实现，v1 设计的地基不动）

- control graph 状态模型：`VIT_PARAM_LINK_GRAPH`（domain=control）挂在 edit.state，子节点三类
  `VIT_CONTROL_NODE` / `VIT_PARAM_BINDING` / `VIT_PARAM_SURFACE`（[VitParamLinkGraph.cpp:13-64](../../../VitApp/Source/Core/VitParamLinkGraph.cpp)；
  缺省 surface=main_surface, kind=macro_panel）；macropanel 类节点走 `VitMacroNode`（nodeId/name/x/y/macroCount）。
- binding schema（[VitParamBinding.h:11-22](../../../VitApp/Source/Core/VitParamBinding.h)）：
  `sourceNodeId + sourceOutput → targetKind(plugin_param|control_port) + targetPluginId/targetParamId（或 targetNodeId/targetInput）+ min/max + curve`；
  curve 词表=**linear(缺省)/invert/ease_in/ease_out**（PluginRackControlService.cpp:416-426、2459-2473）。
- 命令面 9 写+1 读全在册（PluginRackControlService.cpp:2328-2800）：control_add_node / control_add_macro /
  control_add_binding / control_update_node / control_set_node_value / control_set_macro_values /
  control_update_binding / control_remove_binding / control_remove_node + 每写回执附 `control_graph` 全量快照。
- 参数描述符已带 binding_target 投影（PluginRackControlService.cpp:626-650）——前端选参面可直接拿到
  `plugin_id + param_id + param_name`，作为绑定目标寻址无需二次解析。

### 1.2 前端（部分在位，两个关键缺口）

- 机架画布宏节点**已在渲染**：`KIND_MACRO_CONTROL`（Vit_Graph_Rack.gd:151-153）；
  数据源=project_repository `get_rack_control_macros` 文档（按 track_id 过滤，:3924-3942），
  渲染为 GraphNode 卡片（z2 区主题、可拖、可改名、内容含 "Mappings N" 状态行，:3897-4010）；
  变更事件 `rack_control_macros_changed` 带 source 去抖（值/位置变更只重绘，:2072-2079）。
- 侧面板 MacroPanel=**agent 推送会话视图**（macro_panel.gd：ingest macro_panel artifact；
  控制施加走 agent HTTP `plugin_grabber.apply_control`，:167-185）。
- 参数绑定工作台：rack_param_surface_panel_factory.gd（分页/筛选/选参/生成控制/profile，923 行）。
- **缺口 G1（新发现，2026-10-18）**：`rack_client.gd` 对 control_* 命令**零封装**——前端目前
  只能渲染 agent 推来的宏文档，不能自己建宏/建绑定/改曲线；控制层的持久编辑面完全缺命令通道。
- **缺口 G2**：宏节点与效果器节点混在同层画布，无层语义（无视觉区分、无层过滤、信号拓扑门未隔离控制连线）。

### 1.3 与 send（v4 定稿）的交点

- SendOut/SendIn=pluginList 里的 AuxSend/AuxReturn 插件呈名（§9.1 零聚合投影）。
- **关键推导**：SendOut 的发送量=AuxSend 插件的 send level 参数（0..1，tracktion_AuxSend.cpp:14-29）
  ——所以「send 量进宏绑定面」**不需要任何新内核机制**：绑定目标就是 SendOut 插件实例的 level 参数，
  与绑 ZamVerb 的 Decay 完全同构（plugin_param target）。
  **核对结论（2026-10-18 晚窗决策侧亲核，闭环）**：
  ① binding 寻径=通用 `plugin.getAutomatableParameterByID(paramId)`（PluginRackControlService.cpp:387-390），
  对内部插件参数原生可用（"master volume"/"master pan" 内部参数特判先例就在同函数）——
  `target_plugin_id=<SendOut 实例> + target_param_id="send level"`（tracktion_AuxSend.cpp:22 注册名）即可绑定；
  ② 参数口数值显示不必依赖描述符面：SendOut 卡的发送量显示走 `track.send.list` 的 amount_db（K1 已有）；
  ③ 唯一残留缺口=发现面：`get_plugin_parameters` 仅对 `te::ExternalPlugin` 开放（:3235-3237），
  aux 内部插件不产描述符——K3 二选一：前端硬接线该参数口（数据源=track.send.list），
  或 K1 面补一个 aux 参数描述符小披露（列入 K3 卡缺口 l 的两个实现选项，非设计阻塞）。

---

## 2. 两层模型（v1 提案）

```
信号层（实线，现状语义不变）：
  Clip 干声 ──▶ z1 预处理 ──▶ z2 空间 ──▶ z3 动态 ──▶ SEND ▸ ─┐
                                                              │（副本岔出，穿通继续）
  Rack Out ◀── ◂ SEND（湿声注入，汇入点=链位） ◀──────────────┘
控制层（虚线，新增语义）：
  [宏节点]·──·──▶ 参数口（插件参数 / SendOut 发送量口）
  ·控制层节点不占信号链：不参与 pluginList 序、不进拓扑门、不产音频
```

规则五条：

1. **层是视觉+交互语义，不是第二张图**：控制层实体与信号层同画布共存；控制线=紫罗兰虚线，
   信号线=现状灰实线（B2 语言：橙=时间限段、青=用户动作、**紫=控制**新增第三语义色）。
2. **控制节点零信号副作用**：`can_connect_rack_topology` 不放行任何「控制节点↔信号锚点」连线；
   控制连线是独立交互通道（绑定手势），不是 GraphEdit connection（详见 §3.4）。
3. **层过滤器**：画布右上角层 chip 组「[●信号] [◆控制]」——双显（默认）/单显控制（绑定期
   信号层压暗 15% 透明度）/单显信号（现状视图，控制层隐藏）。切层不改数据只改渲染。
4. **控制层坐标独立于泳道**：宏节点位置=x/y 自由摆放（内核 VitMacroNode 已是 x/y 模型），
   不进 z1/z2/z3 泳道列布局；效果器卡上的**参数口**是绑定的落点（见 §3.2）。
5. **快照与真相源**：控制层读侧=内核 `control_graph` 快照（每写命令回执自带）；现状
   repository 宏文档路径降级为 agent 会话投影的缓存（见 §5）。

---

## 3. 控制层交互规格 v1

### 3.1 宏节点形态（与 v4 节点语言同族）

- 卡片=同款圆角卡+标题栏，但**左侧竖条为紫色**（效果器卡现状用轨道色/区色）；
  尺寸沿用 `RACK_GRAPH_MACRO_NODE_MIN 236×116`。
- 内容：旋钮（dial，主控件）+当前值+名称（可改名，现状 rename dialog 保留）+
  「N 映射」badge（点击=展开该宏的全部绑定列表）。
- 右侧一个**输出锚**（绑定拖拽起点，紫色小圆）；下方可选 slot 排布多旋钮
  （macroCount>1 时一卡多旋钮，内核模型原生支持）。

### 3.2 参数口（绑定落点）

- 效果器卡/SendOut 卡右上角新增**参数口行**：小圆点+参数名（hover 展开）；缺省收起只显示
  「⌗ params」计数，点击展开该卡前 N 个参数口（或搜索）。
- SendOut 卡的「发送量」= 参数口之一（§1.3 推导）；SendIn 无参数口（湿声注入无用户参数）。

### 3.3 绑定线（控制虚线）

- 视觉：紫虚线（dash 6/4），线中点小菱形=绑定把手；hover 展开 tooltip：
  `宏名 → 插件名·参数名 | min..max | curve`。
- 点击绑定把手（或 hover 后点开）=**绑定编辑卡**：curve 四选（linear/invert/ease_in/ease_out，
  内核词表原生）+min/max 数字对+「断开绑定」。
- 右键绑定线=断开（control_remove_binding）。

### 3.4 创建流（三步）

1. 画布右键 →「新建宏」（control_add_macro，x/y=落点）→ 宏节点就地出现，进入命名；
2. 从宏输出锚**拖出虚线**（前端手势，不触发 GraphEdit connection_request）→ 悬停任意参数口
   高亮 → 松手=弹出绑定小菜单（curve+范围缺省 linear / 0..1）→ 确认=control_add_binding；
3. 内核回执 control_graph 快照 → 前端重渲染绑定线（乐观虚线 pending → 确认替换，
   rack_pending_edge_rules 同款回滚模式）。

### 3.5 宏值施加

- 旋钮转动（drag_ended 提交，中间帧本地）→ `control_set_macro_values`（批量：一卡多旋钮一次提交）；
  内核按 binding 分发到各目标参数。
- 与侧面板/agent 施加同一命令面（不再经 plugin_grabber.apply_control 的 agent HTTP 旁路——
  那条保留给 agent 会话视图，见 §5）。

---

## 4. 层切换与视觉规范（样图 v1 呈现）

| 元素 | 信号层 | 控制层 |
|---|---|---|
| 线 | 灰实线（现状 _draw 电缆） | 紫虚线 #a78bfa 系 |
| 节点 | 效果器卡/SendOut/SendIn/Bus | 宏卡（紫竖条+旋钮） |
| 层 chip | ● 信号（灰） | ◆ 控制（紫） |
| 拓扑门 | can_connect_rack_topology | 不参与（独立手势） |

---

## 5. 侧面板 MacroPanel 与参数工作台的角色（v1 提案）

- **侧面板 MacroPanel=agent 会话视图（保留，只读化倾向）**：agent 推 macro_panel artifact 的
  呈现面不变；其「施加」动作从 agent HTTP 旁路改为直调 control_set_macro_values（与机架同命令面，
  一致性与审计单一化）——若 agent 需要经手，保留 plugin_grabber.apply_control 作为其工具面，
  GUI 不再双轨。
- **持久编辑归机架控制层**：建宏/绑定/曲线/范围全部在机架画布（§3），侧面板不提供编辑。
- **参数绑定工作台（rack_param_surface_panel_factory）收编为控制层入口之一**：其「选参生成控制」
  的产物从「agent profile 文档」改为直接落控制层（生成宏节点+批量绑定，走 control_* 命令）；
  工作台 UI 本身可后续迁入画布参数口面板（K3 范围外，登记为后续）。

---

## 6. K3（130）实现清单对接（缺口字母续 K3 卡既有清单）

- **g）rack_client 封装 control_* 命令**（缺口 G1）：add_macro/add_binding/update_binding/
  remove_binding/set_macro_values/update_node/remove_node+graph 读回；
- **h）宏节点数据源切换**：repository 文档路径 → control_graph 快照渲染（§2 规则 5）；
- **i）层过滤器+控制虚线渲染**：层 chip+控制线自绘（复用 _draw 电缆管线的虚线变体）；
- **j）绑定手势通道**：输出锚拖拽+参数口高亮+绑定菜单（不经 connection_request）；
- **k）参数口行**：效果器卡+SendOut 卡的参数口展开（数据源=参数描述符 binding_target 投影 §1.1）；
- **l）K1 面核对项**：aux 插件 send level 参数披露（§1.3）。

---

## 7. 开放问题（用户迭代点，样图内也标注）

1. 控制层默认显示 vs 默认隐藏（v1=默认双显，宏少不碍事；若嫌乱可改默认只信号、按需开）；
2. 紫色作为控制语义色是否接受（备选：绿=Bitwig 调制色习惯 / 黄=MIDI learn 习惯）；
3. 宏卡多旋钮形态：一卡一旋钮（简单）vs 一卡多旋钮 slot（内核 macroCount 原生，v1 样图按多旋钮画）；
4. 绑定编辑入口：线中点把手点击（v1）vs 全部收进宏卡「N 映射」列表内编辑（两处都开亦可）。
