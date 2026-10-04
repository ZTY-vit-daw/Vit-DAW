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
- 领取：2026-10-04 10:15 / Godot=port/vitnote-notestream-2@4391c49（工作树 HEAD 即此）/ agent=main@d2862b59 / 分支名：两腿均 `port/vitnote-region-time-1`（agent 腿按并行流纪律开独立 worktree `D:\Vit_DAW_wt_region_time_1`；主树另有 VitApp workspace XML 两处运行时改动与并行流 REGION-OP-RECON-1/WEBUI-SESSION-SEMANTICS-1 各占 worktree，均不触碰）
- 回执：2026-10-04 执行侧自验完成
  - **commit**：Godot=`ecc722d`（port/vitnote-region-time-1，自 4391c49 切出，4 文件 +258/-1）；agent=`1770f3c9`+`0364bcc5`（port/vitnote-region-time-1，worktree D:\Vit_DAW_wt_region_time_1 自 main d2862b59 切出，2 文件 +326/-7）。均未 push、未合 main（待决策验收）。
  - **实锚申报**（卡面"载荷组装一带"具体落点）：Godot `vit_face_supplier_timeline.gd`（`vit_face_resolve` 增维：domain 顶层 `range_time_span`=框选左右缘×`timeline_seconds_at_global_point`（与 time_window 空命中路径同源同映射——**停止条件解除依据：像素↔时间换算在 resolve 层可得**，无需 lane 内部新状态）；每 entry `range_clip_start/range_clip_end`=clip∩范围交；新静态纯函数 `clip_range_time_intersection`）+ `vit_note_panel.gd`（`build_note_payload` v3.1：timeline 面 domain 的 span 提升到载荷顶层，异形/缺省键不加）；`vit_note_manager.gd` 零改动（domain 经 resolve_circle 透传）。agent `internal/chat/note_sessions.go`（`NoteChatPayload.RangeTimeSpan` 顶层线格式字段——补丁 0364bcc5 防 JSON 解码静默丢弃；`noteJurisdictionSnapshot` 快照段注入 `range_time_span`+`time_digest` 两键；`noteJurisdictionDigest` 摘要函数组：m:ss.d 格式化/「时间线 0:03.2–0:08.5：命中 N 个 clip：」头行/每 clip「全长 …（与范围相交段 …）」；系统段新增指令：回答辖区内容须带范围时间跨度与相交段、字段缺席不伪造）。
  - **探针与测试**：Godot `--import` 退出 0；face_resolve 探针新增时间维度套件（静态五例：全含/部分交/零交/零长贴边/逆序 clip；假 lane 真链：span=框选缘映射、与命中包络 time_window 语义分立、entry 三态、空命中仍带跨度、无 lane 键缺席降级）PASS；notestream 探针 **26 钉** 0 失败（新 F 面：顶层提升/旧快照缺省/异形 span 守卫）；container 141 / circle_state 52 / panel_summary 16 / input 全 PASS（input 退出 0，"取证跑"输出为其设计形态，与本卡无关）。Agent：note 钉（m:ss.d 格式化/时间交三例/fail-open 旧载荷/异形值防御/组装注入/线格式解码）+ **chat 全包 ok**（~90s）。一次探针期望修正记录：time_window 包络对无时间字段命中按 0.0 缺省入包络=既有契约，探针期望已对齐（产品代码无缺陷）。
  - **边界声明**（AGENTS §5）：未起真栈（无内核/Godot 前端/agent 进程），7878 往返与 Godot↔agent 端到端问答链未覆盖——探针/单测锁纯函数、组装面与载荷契约，端到端归用户手测复验（NOTESTREAM-2 同口径）。
  - **手测复验点**（用户真栈，按 Godot 拉起途径）：① 框选含 clip 的时间线区域→开便签→问「这个范围是什么内容」→回答应含范围时间段（如 0:03.2–0:08.5）与每 clip 相交段；② 框选只盖 clip 局部→相交段应只是被框住的那段（非 clip 全长）；③ 框选空时间线区或纯机架面→回答不伪造时间（缺省如实）。
  - 并行流注记：主树 VitApp workspace XML 两处运行时改动与 REGION-OP-RECON-1/WEBUI-SESSION-SEMANTICS-1 两并行流均未触碰；agent 腿按并行流纪律走独立 worktree。
- 验收：（裁定文件 / 验收 commit）
