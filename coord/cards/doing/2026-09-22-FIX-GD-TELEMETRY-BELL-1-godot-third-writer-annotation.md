# FIX-GD-TELEMETRY-BELL-1：Godot telemetry_manager 第三写者前台铃标注补齐（preserve 分支）

- 优先级 / 预估 / 依赖：P2 / 0.5 天 / FIX-F5-SNAPSHOT-FRESHNESS 验收裁定 2026-09-22（第三写者上交开卡；agent 侧责任已闭环）
- 模型分级：L1 / GLM-5.3 flash 可接（单文件域+同款语义移植，锚点已在卡）
- **目标仓库（跨仓卡）**：`D:\Godot\project\vit-daw-frontend`（Godot 4 工程，Vit-DAW DAW 前端；**本卡不在 D:\Vit_DAW 执行**，回执仍写回本 coord）
- **背景（FIX-F5 回执上交+决策侧复核）**：mixboard 声学桥快照有三层写者。agent 侧经 FIX-F5 已落盘单点标注（current/stale+superseded_by_request/material_reuse）+tmp+rename 原子发布；Godot `app/kernel/autoloads/telemetry_manager.gd` 是 `kernel_prepared_waveform_envelope_<clip>` 铃的发明者：内存持有整份快照、750ms debounce、自身已 tmp+rename、flush 前 preserve 磁盘行（保留 agent 标注）。**残余缺口**：`_mixboard_should_keep_prepared_feature_row`（:2381）preserve 分支——磁盘 prepared 行胜过内存弱状态行时可落"行 request_id ≠ 铃且无 freshness 标注"形态（agent 未写过的行）；该快照面有 Godot 侧消费方（mix_client.gd / LLM_Chat_Controller.gd），无标注分叉可直达 UI/聊天控制器呈现。本轮真栈未触发（⑤ 三轮 exit 0），属有界潜伏风险。
- 目标：
  1. Godot flush 落盘前对 preserve 采纳的行补写同款前台铃标注（行 id==铃→current；行 id≠铃→stale+superseded_by_request=铃；无铃归属有材料身份→material_reuse——语义与 agent 侧 `annotateMixboardRowFreshness` 对齐，字段名保持一致供双端互认）
  2. 红测试（GDScript 或场景测试）：构造"磁盘 prepared 行 + 内存弱状态行"preserve 序列，修前落盘行无标注/修后有标注且保留原 request_id
  3. 不动：铃的发明与请求时序、750ms debounce、tmp+rename 落盘、agent 侧任何代码
- 约束：content-blind（标注只用 request_id/feature_type/status 等结构性字段）；不引入 agent 依赖（Godot 侧不读 agent 代码）；**mac 双端注记**——Godot 前端经 PORT-B4B 移植到 mac，本修复需同步 mac 前端仓（用户转交，见 2026-09-22 转交包附注）
- 验收：①红测试修前红/修后绿；②Godot 侧快照落盘抽查：preserve 采纳行全部携带标注（手测或自动皆可，按手测入口裁定从 Godot 拉起）；③回执写回 D:\Vit_DAW coord（本卡移 doing 时在两仓各自记 HEAD）
- 停止条件：preserve 分支语义与 agent 标注语义冲突（如 Godot 行模型无法承载 superseded_by_request）→ 上交扩域裁定
- 领取：2026-09-24，Godot 前端仓执行会话（ZCode/GLM-5.3）。Godot 仓 HEAD=c7bcd2978ead8c7dfe67c03bece85ede507a2481（工作树已有他人未提交改动：telemetry_manager.gd 的材料同源判定、global_settings/start_page/vit_dock 系列等，diff 归属见回执）；Vit_DAW 仓 HEAD=6774b9fba197386e771f2faf47b9c6d2b9c62e。
- 回执：2026-09-24 执行完成（Godot 前端仓，零 agent 侧改动）。
  - **实现**：`app/kernel/autoloads/telemetry_manager.gd` 新增①键集常量 `MIXBOARD_FRESHNESS_SINGLE_ROW_KEYS`/`MIXBOARD_FRESHNESS_MULTI_ROW_KEYS`（与 agent `annotateMixboardSnapshotFreshness` 键集逐项一致）②`_write_mixboard_feature_snapshot` 在 `_merge_existing_mixboard_feature_request` 之后、tmp 写入之前插入一行 `_annotate_mixboard_snapshot_freshness()` 调用③`_annotate_mixboard_row_freshness` 三段语义：行 id==铃→`freshness=current` 并清 `superseded_by_request`/`superseded_at`；行 id≠铃→`stale`+`superseded_by_request=铃`（指针变化才写 `superseded_at`，防时间戳抖动）；无铃归属有材料身份→`material_reuse`+`reused_for_request=铃`。占位行（status ""/missing）跳过；材料身份字段=source_revision/source_fingerprint/source_hash/clip_revision/source_path|file_path|current_source_path（与 agent `mixboardFeatureRowHasMaterialIdentity` 同款）。preserve 分支采纳的磁盘行与内存行统一走此 pass——第三写者与 agent 同笔写入同款标注。
  - **红绿证据**（工件 `coord/runs/FIX-GD-TELEMETRY-BELL-1/`）：红 `20260924_214150_red` exit=2，12 条失败全为标注断言（freshness/superseded_by_request/reused_for_request 缺失或 stale 残留），preserve 断言（原 request_id/status 保留）全过；绿 `20260924_214219_green` exit=0 全过，重复 `_green2` exit=0（确定性）。新测试 `tools/diagnostics/mixboard_snapshot_freshness_annotation_probe.gd`（headless SceneTree probe，6 场景：stale/current 清残留/material_reuse/多行 summary 新旧双行/track_waveform 复用/占位行不标注），断言只用结构性字段（content-blind），走真实 `_write_mixboard_feature_snapshot`（merge→tmp+rename）。落盘抽查：preserve 行带 `freshness:stale` 且 `request_id:agent_probe_prev_request` 原样保留。
  - **不动面确认**：铃发明时序（`_ensure_mixboard_kernel_feature_request` 未触碰）、750ms debounce（`_schedule_mixboard_feature_snapshot_write` 未触碰）、tmp+rename 落盘（仅插入标注调用）、agent 仓零改动。
  - **边界注记**：①键集镜像 agent 排除面——`l3_acoustic_*`/`realtime_*` 键 agent 侧即不标注，Godot 同样不标（preserve 这些键的行仍无标注，与 agent 现状一致；如需扩大标注面属决策侧扩域）。②消费方安全：Godot 读侧走 `.get()` 增量键（mix_client 经 agent 请求面不直读磁盘快照），无严格校验。③mac 双端：需同步 mac 前端仓（PORT-B4B 同源），待用户转交。④手测（Godot 拉起入口抽查真栈落盘）可选补做，验收②已按卡面"自动皆可"由 probe 覆盖。
  - **diff 归属**：Godot 仓 `telemetry_manager.gd` 领取前已有他人未提交改动（`_mixboard_feature_rows_same_material` 材料同源判定约 18 行，Save As 主题），本卡未触碰；两改动共存同文件，提交时需分账。本卡新增：telemetry_manager.gd 标注块（约 +83 行）+ 新 probe 文件（untracked）。
- 验收：
