# Ruling：VITNOTE-CONTAINER-3 — conditional pass（2026-10-03 决策会话）

## 验收

1. **diff 亲核**（82635c1@port/vitnote-container-3，Godot 4 文件=2 产品+2 工具，+879/-221）：设计段 A/B/C 逐条落地——8 把手入框（边 8px 单轴延展/角 12px 等比=锁比+对角锚点+触界同率截断，`resolve_edge_resize`/`resolve_corner_scale` 纯函数族）；图钉带=视窗下方固定停靠（旧锚点小圆点及"拖动=移动"路径退役，推翻 10-02 旧裁定落实）；作用域描边 overlay 顶层 IGNORE。头注文档链与常量表与设计段一致。
2. **我方独立复跑**（Godot 仓亲跑，输出落 runs/VITNOTE-CONTAINER-3/verify_pc/）：container 探针 **116/116 EXIT=0**；face_resolve **127/127**；circle_state **52/52**；panel_summary **16/16**；input dock EXIT=0；`--import` 零 SCRIPT ERROR——与回执逐面吻合。
3. **工件亲读**：前后截图四张+放大件在档；目检发现并修复 open_note 首绘缺描边（诚实留档）；图钉 glyph 观感（渲染过小、分析器未辨）如实申报归用户手测——存在性由断言面锁定，边界划分正确。
4. **设计偏差 4 项采信**（空带隐藏/自绘 glyph/40px 握持/对角投影等比）——均属设计段未定自由量，解释符合"不遮内容"精神，回写卡面为既定设计。
5. 泊位/工作树声明核实：零真栈零端口；与并行 flash 会话的工作树碰撞一次自愈披露（领取提交误落分支→指针还原+移 main，该分支零污染）——过程风险记 gate，无工件损失。

## 裁定

- **conditional pass**：探针面全过；**用户手测复验（回执自验清单 7 项）**为转正条件——同场转正 VITNOTE-CONTAINER-2 与 VITNOTE-V2-IMPL-D（两 conditional 卡）。
- Godot 分支 `port/vitnote-container-3`@82635c1 为现 HEAD——**NOTESTREAM-2 Godot 腿基线自此切出**。
