# VITNOTE-CONTAINER-4：图钉回归视窗（大/半透明/固定/代表范围）+ 便签列表入左侧资料库列 + 底部图钉带退役（P2，用户裁定 2026-10-03 二号）

- 池序 7；目标仓库=D:\Godot\project\vit-daw-frontend；来源=[decisions/2026-10-03-note-pin-design-v2.md](../../decisions/2026-10-03-note-pin-design-v2.md)（修订 CONTAINER-3 设计段 B）；分支自 port/vitnote-container-3@82635c1 切出
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / CONTAINER-3 已 conditional pass（把手/描边/管理器基线在位）
- 模型分级：L1 / GLM 或 flash 可接（纯 Godot 域，CONTAINER-3 先例充分）
- 已核实事实（用户裁定原文要点，详见 decision）：
  1. 用户肯定 CONTAINER-3 方向（"设计更好了"）：把手、面板拖动、作用域概念均保留。
  2. **推翻底部图钉带落点**：图钉应**留在视窗上**（代表其圈选范围处），可比旧小圆点更大，压内容上方时**半透明**，**不可移动**。
  3. 便签列表如需罗列：**左侧资料库区**专门一列（与 group/plugin 列表同级同模式）。
- 目标：
  1. **图钉回归视窗**：收起态图钉渲染于圈选范围处（overlay 顶层，轨道头之上结构性不遮）；尺寸可放大（≥旧 12px 的 1.5-2 倍，实测定值）；与内容重叠时半透明（alpha 阈值实锚）；固定不可拖（CONTAINER-3 断言面沿用）；单击重开/右键删除语义不变；悬停 peek 作用域描边沿用。
  2. **底部图钉带退役**：删除 pin tray；空带隐藏逻辑随之退役。
  3. **便签列（左侧资料库）**：资料库抽屉内新增便签列（group/plugin 列表同款交互模式）：条目=图钉 glyph+序号+摘要缩写；单击=重开对应 note；右键=删除；无便签时空态文案。若本卡取简可先做"列内只读罗列+单击重开"，新建仍走 Q 圈选（不入本卡）。
  4. 回归：CONTAINER-3 探针族迁移（pin tray 断言改视窗图钉断言：位置=范围处/不可拖/半透明 alpha/不被轨道头遮）+四回归+`--import` 零 SCRIPT ERROR；探针输出落 runs/。
  5. 用户手测复验（图钉在视窗/大而可辨/半透明/固定；资料库便签列；底部无停靠带），过则 CONTAINER-3+CONTAINER-4 同场转正（连带 CONTAINER-2/IMPL-D）。
- 文件域：Godot `vit_note_panel.gd`+`vit_note_manager.gd`（+资料库面板挂列如需，实锚后申报 ≤4 文件+工具）。
- 约束：不动 main；headless 探针不占端口；与 NOTESTREAM-2 Godot 腿串行（本卡先——同文件域）。
- 验收标准：目标 1-3 落地+探针/回归全绿+用户手测复验。
- 停止条件：资料库面板结构不容新列（需动 dock/抽屉架构）→ 上交裁定；图钉半透明与 overlay 渲染层耦合超预期 → 方案上交。
- 领取：2026-10-03 19:53 PC 执行侧（ZCode GLM-5.3）/ Vit-DAW 仓 origin/main=48ee12c28afc486b8a9a1a25289220e1a47a679a / Godot 仓 port/vitnote-container-4 自 port/vitnote-container-3@82635c1 切出
- 回执：（commit hash / 实锚申报 / 探针结果 / 前后截图）
- 验收：（裁定文件 / 验收 commit）
