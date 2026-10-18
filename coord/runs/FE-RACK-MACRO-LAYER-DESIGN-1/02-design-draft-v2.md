# FE-RACK-MACRO-LAYER-DESIGN-1 设计草稿 v2：控件族谱 + 双面架构

- 决策侧：GLM-5.3 主管决策流（ZCode，2026-10-18 深夜 v2 迭代）
- 状态：**v2 草稿，待用户迭代**（样图=[macro_layer_v2.html](macro_layer_v2.html)）
- v1 反馈吸收（用户 2026-10-18 深夜）：①方向对/要 MAX·MSP 式多类型控件库；②须考虑 agent 侧同步生成控件的双面效果
- v1 不变项：两层模型/紫虚线绑定/层 chip/创建三步流/绑定编辑卡——见 [01-design-draft-v1.md](01-design-draft-v1.md)

---

## 1. 新核验事实（2026-10-18 深夜决策侧亲核）

- **内核原生控件词表共十类**：`slider / knob / xypad / toggle / button / numberbox / range / envelope / lfo / stepsequencer`
  （+容器 `macropanel`）——[VitParamSurface.cpp:19-23](../../../VitApp/Source/Core/VitParamSurface.cpp) `supportedControlKinds()`；
  `normaliseControlKind`（:6-17）未知类型 **fail-open 归 slider**。
  → 控件族谱不是新机制：把内核已有词汇画全、前端补渲染器即可。
- 控件节点带 kind+x/y+value（`control_set_node_value` 可写）；binding 带 `source_output`
  （[VitParamBinding.h:11-22](../../../VitApp/Source/Core/VitParamBinding.h)）→ **xypad/range 一控件双输出绑两参数在 schema 上原生成立**。
- macropanel 容器节点带 `macro_count`（VitMacroNode.cpp createSnapshot）→ 一卡多控件混排有内核模型背书。
- agent 侧现状：macro_panel.gd `_control_row` 只认 toggle/switch/checkbox、button/trigger、缺省 slider 三类
  （[macro_panel.gd:106-114](../../../agent/../frontend 参考)）→ 双面统一需扩词表。

## 2. 控件族谱规格（三组十类）

| 组 | kind | 适用参数特征 | 绑定语义 | K3 分期 |
|---|---|---|---|---|
| 连续量 | knob | 连续单参数（缺省首选） | 静态映射+curve | 第一批 |
| 连续量 | slider | 连续·长行程精细 | 静态映射+curve | 第一批 |
| 连续量 | numberbox | 精确值·步进·单位敏感 | 静态映射（线性） | 第一批 |
| 连续量 | xypad | 双相关参数联动 | **双输出双绑定**（out_x/out_y） | 第一批 |
| 开关触发 | toggle | 两态/枚举二值 | min/max 两点映射 | 第一批 |
| 开关触发 | button | 瞬时触发无保持值 | 脉冲（无静态绑定） | 第一批 |
| 时变形 | range | 双端 min+max 成对 | 双输出双绑定 | 第一批 |
| 时变形 | envelope | 包络（ADSR 拖点） | 动态调制 | **渲染壳先行，绑定另卡** |
| 时变形 | lfo | 低频振荡调制源 | 动态调制 | 同上 |
| 时变形 | stepsequencer | 8-16 步节奏门 | 动态调制 | 同上 |

- 选型规则（**确定性，零 LLM**）：创建绑定时按目标参数特征给建议控件类型，用户可下拉改任意类型。
- 兼容：unknown kind→slider（内核归一已定）；旧 macro_panel artifact 的 switch/checkbox/trigger 别名前端渲染器保留。

## 3. 混排宏卡

- 一张宏卡=一个容器（macropanel 语义），内部**多控件混排**（MAX/MSP patch 习惯）：
  例「空间感」=knob（空间量）+numberbox（decay 直显）+toggle（发送通断）。
- 卡级仍有「N 映射」badge（该卡全部绑定的列表入口）与卡级输出锚；控件级输出锚（如 xypad 的 out_x/out_y）直接出线。

## 4. 双面架构（机架控制层 ⇄ agent 侧面板）

**同一套控件词汇（kind 十词表）贯穿三处**：

1. 内核 control_graph 节点 kind（真源）；
2. agent 推送 macro_panel artifact 的 control.type（生成面数据）；
3. 前端渲染器组件族（两个面共用同一套控件绘制组件）。

- **机架控制层=持久编辑面**（建宏/绑定/曲线/换型，v1 语义不变）；
- **侧面板 MacroPanel=agent 会话生成面**：渲染器从三类扩到十词表（含别名兼容）；
  施加统一走 `control_set_macro_values`（与机架同命令面）；
- 数据流：agent 生成物经 control_* 落库 → 出现在机架控制层（同一真源）；机架侧用户建的宏，
  agent 会话视图可引用呈现——不建第二真相源。

## 5. K3（130）缺口清单增补（v1 的 g-l 之后续）

- m）**控件渲染器组件族**：十类型绘制+交互（第一批七静态类完整交互；时变三类渲染壳）；
- n）**侧面板渲染器扩展**：macro_panel.gd 扩十词表——**文件域增补 app/shell/side_panel/**（若用户选侧面板另卡则移出）；
- o）**类型建议规则表**：参数特征→建议 kind（前端确定性表，无 LLM）。

## 6. 开放问题（样图 QA 区同步列出）

1. 第一批控件范围=七静态类（时变三类渲染壳先行）？；
2. agent 侧面板渲染器扩展归 K3 一并做（+side_panel 域）还是另卡？；
3. 类型建议规则（§2 表）够否/要不要「按已绑定参数自动换型」提示；
4. （v1 遗留）控制层默认双显？
5. （v1 遗留）控制色=紫？
