# VITNOTE-REGION-TIME-1：辖区上下文时间维度披露——范围时间跨度+clip∩范围时间段进载荷与回答（P2）

- 池序 6；目标仓库=D:\Godot\project\vit-daw-frontend+D:\Vit_DAW（跨 Godot 载荷腿+agent 模板腿，一卡串行）；来源=手测四号场用户反馈（2026-10-03：辖区识别已答 clip 但**未给 clip 在范围内的具体时间段**——用户场景：框选 clip 局部→要的就是只作用于该范围）
- 优先级 / 预估 / 依赖：P2 / 0.25-0.5 天 / NOTESTREAM-2 已转正（note 会话与注入链在位）；分支 Godot 自 port/vitnote-notestream-2@4391c49、agent 自 main 切出
- 模型分级：L1 / GLM 或 flash 可接（先例充分）
- 已核实事实：
  1. resolve_circle 已做空间相交判定（谁在范围内）；**时间交未进载荷**（裁定 3 方案 A 的轻摘要=轨名+clip 数+插件名，无时间界）——用户测试确认：能答有哪些 clip，答不出时间段。
  2. 时间信息在相交判定处**已经算过或近零成本可算**（范围 rect ∩ clip rect 的时间轴投影）。
- 目标：
  1. **载荷增维**：faces/辖区载荷（v3→v3.1）每 clip 条目带 `range_clip_start/range_clip_end`（clip∩范围时间界）+ 载荷顶层带范围时间跨度 `range_time_span`（时间线面框选映射的时间段；非时间线面无此项如实缺省）。
  2. **回答模板**：note 组装（buildNoteAssembly）辖区快照段落含时间维度——「时间线 0:03.2–0:08.5：贝斯 clip（与范围相交段 0:03.2–0:07.8）」式；prompt 指令要求回答辖区内容时给时间界。
  3. 兼容：旧载荷无时间字段→模板缺省不报错（fail-open）；探针断言面新增（时间交三例：全含/部分交/零交缺省）。
  4. 回归：NOTESTREAM-2 探针族+四回归+--import 零错（Godot）；Go note 钉+chat 全包（agent）；用户手测复验（问「这个范围是什么内容」回答含时间段）。
- 文件域：Godot `vit_note_manager.gd`/`vit_note_circle_resolver` 一带（载荷组装，实锚后申报）+agent `note_sessions.go`（模板+prompt）+测试。
- 验收标准：载荷/模板/回答三面落地+回归全绿+用户手测复验。
- 停止条件：时间线坐标系换算（像素↔时间）在 resolve 层不可得（需 lane 内部状态）→ 实锚上交定方案。
- 领取：（时间 / 两仓基线 / 分支名）
- 回执：（commit hash / 实锚 / 探针与测试 / 手测复验点）
- 验收：（裁定文件 / 验收 commit）
