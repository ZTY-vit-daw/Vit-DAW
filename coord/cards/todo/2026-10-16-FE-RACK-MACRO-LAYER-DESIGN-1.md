# FE-RACK-MACRO-LAYER-DESIGN-1：机架宏控制层设计（send×宏 整合，129）

- 发卡：GLM 主管决策侧 / 2026-10-16 晚窗（依据=用户「机架视窗里之前已经设计了宏控件相关内容，顺便把宏控件一起升级，整理到一块」）
- 派发确认：**已确认（用户 2026-10-16 同上+「一起执行」）**；验收负责人：GLM 主管决策流
- 池序 129；目标=设计件（coord/runs）；P2 / 设计 0.5-1 天 / 无依赖（与 K1/K2 并行）
- 产出方式：**决策侧对话式样图迭代**（send GUI v1-v4 同模式），用户过目定稿后方可进 K3

## 现状盘点（2026-10-16 决策侧勘察在卷）

- 内核 control graph **已实现**：macropanel 节点（x/y 坐标）+到插件参数 binding+`control_set_macro_values`，11 条 control_* 命令在册（PluginRackControlService.cpp:2360-2800）；
- 机架画布**已有** `KIND_MACRO_CONTROL` 节点类型（Vit_Graph_Rack.gd:151-165，宏节点可放画布/重命名/位置 meta）——但与效果器混在同层，无层语义；
- 侧面板 MacroPanel=agent 推送会话视图（macro_panel.gd，ingest macro_panel artifact）；
- 参数绑定工作台：rack_param_surface_panel_factory.gd（分页/筛选/选参/生成控制/profile）；
- B2-RACK-VIZ-SKETCH 草稿在案（coord/runs/B2-RACK-VIZ-SKETCH/rack_viz.png+html，2026-10-04）——设计时对照。

## 设计目标（框架=设计件 §9.4，细化待样图迭代）

1. **机架两层模型**：信号层（rack in→效果器串联→SendOut/SendIn→rack out，实线）+**控制层**（宏节点+binding 虚线到参数口）——控制层节点不占信号链，线型区分（信号实线/控制虚线）；
2. **send amount 进宏绑定面**：发送量成为 control graph 可绑定参数（宏旋钮同时带 send 量+混响 decay 的经典混音宏）；
3. **宏节点形态与 v4 节点语言统一**（SEND tag 同款视觉族）；binding 线交互（从宏拖到参数口/断开/映射曲线）；
4. 侧面板 MacroPanel 角色重定位：agent 会话视图保留，持久编辑归机架控制层；
5. 参数绑定工作台与控制层的关系（工作台是否收编为控制层入口之一）。

## 验收标准

- 设计件（coord/runs/FE-RACK-MACRO-LAYER-DESIGN-1/）：现状盘点+两层模型+宏节点/binding 线/send 绑定交互规格+与 K3 的实现清单对接；
- **样图经用户过目定稿**+决策侧评审 pass 后放行 K3（130）；
- 零生产代码改动。

## 停止条件

- 现状宏节点/参数面板的功能面盘点发现与本设计前提冲突（如 control graph 语义与样图假设不符）→ 上交对齐再继续。

- 领取：2026-10-18 早窗 / 决策侧对话迭代启动（用户 /morning 四线确认指定 129 开线）/ main=b61a773e / 现状勘察已复核并补新发现（rack_client 对 control_* 零封装=缺口 G1；宏节点渲染链/绑定 schema/curve 词表锚点见 [01-design-draft-v1.md](../../runs/FE-RACK-MACRO-LAYER-DESIGN-1/01-design-draft-v1.md) §1）
- **v2 迭代（2026-10-18 深夜）**：用户反馈=方向对+要 MAX/MSP 式多类型控件库+agent 侧同步生成控件双面考虑 → [02-design-draft-v2.md](../../runs/FE-RACK-MACRO-LAYER-DESIGN-1/02-design-draft-v2.md)+[macro_layer_v2.html](../../runs/FE-RACK-MACRO-LAYER-DESIGN-1/macro_layer_v2.html) 已出。新核验=内核原生十类控件词表（VitParamSurface supportedControlKinds）+xypad/range 双输出绑定 schema 原生+macropanel macro_count 混排容器；新增规格=控件族谱三组十类/混排宏卡/双面架构（kind 十词表统一内核-artifact-渲染器三处）/类型建议确定性规则/K3 缺口 m-n-o。**待用户表态 5 问（v2 三问+v1 遗留两问）后定稿放行 K3**。
- 回执：（设计件路径 / 样图定稿记录）
- 验收：（用户定稿记录 / 评审 commit）
