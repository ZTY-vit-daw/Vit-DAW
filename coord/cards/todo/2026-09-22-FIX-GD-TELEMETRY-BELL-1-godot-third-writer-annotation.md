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
- 领取：
- 回执：
- 验收：
