# REGION-INTENT-WIRE-1：意图层接线框选范围——selected_clip_ranges 进 selectedClipArgs/split 快速意图（P2）

- 池序 5（RECON 分期 1 近期小改，2026-10-04 裁定承接卡）；目标仓库=D:\Vit_DAW（PC 执行侧）；来源=[decisions/2026-10-04-region-op-path-and-freestate.md](../../decisions/2026-10-04-region-op-path-and-freestate.md) 裁定 1+[RECON.md](../../runs/REGION-OP-RECON-1/RECON.md) 问 4 形态一
- 优先级 / 预估 / 依赖：P2 / 0.25-0.5 天 / main b3b814d0（无依赖卡）
- 模型分级：L1 / **flash 可接**（先例=message_loop.go:1268 strip_silence 的 selected_ranges 接法）
- 已核实事实（RECON 锚点，领取时可回查）：
  1. `intent.go:248-259` selectedClipArgs 只取 clip id 键（selected_clip_ids/piano_roll_focus_clip_id 等），**不取 selected_clip_ranges**——split 快速意图切点来自口述秒数（firstLocalSeconds）或播放头，与框选无关（intent.go:93-112）。
  2. 前端 range 工具产物已入主对话上下文：`server.go:2171-2175` 白名单+`server.go:5119-5128` 结构校验；结构=range_id/clip_id/track_id/start_seconds/end_seconds/duration_seconds/clip_start_seconds/clip_end_seconds/clip_local_start_seconds/clip_local_end_seconds/source（全局+clip 内双时间界，vit_track_lane_2d.gd:2016-2024+TimelineEditStore `_normalize_clip_range`）。
- 目标：
  1. **selectedClipArgs 增 ranges 消费**：上下文有 `selected_clip_ranges` 时解析出框选（沿用 server 侧结构校验口径，异形/空=fail-open 忽略）；多 range 取首个并在参数面注记 ranges 数。
  2. **split 快速意图接线**：话语含"这段/这个范围/框选的"且无口述时间且 ranges 在场 → 切点=range 全局界（最小面：单 clip 单 range 语义——"把框住的这段拆出来"=两次 split 或 split+resize，实现取内核命令最小组合，实锚后申报）；无 ranges 时行为与现状零变化（回归面）。
  3. 普通确认制执行：split 走既有 RiskConfirm 提案面，零治理改动（本卡不碰自由态/harness）。
  4. 测试：意图解析新单测（ranges 在场命中/缺席零变化/异形忽略/多 range 取首+注记/口述时间优先于 ranges）+既有 conversation/chat 回归。
- 文件域：agent/internal/conversation/intent.go+intent 测试文件（如需 message_loop 接线，同域最小面+回执申报）。
- 约束：不改 server 白名单/结构校验（已含 ranges）；不改 catalog 工具面；不碰 webui/Godot。
- 验收标准：`go test ./internal/conversation ./internal/chat -count=1` 全绿+新用例+`go build ./...`+gofmt（git show HEAD:<file> | gofmt -d 为空）。
- 停止条件：实锚发现 split 快速意图路由不承载范围形态（需新意图分支）→ 锚点+形态清单上交定方案，不扩域自决。
- 领取：2026-10-04 12:57 / origin/main dce9c3b81c5d427231a288acad59727e68771739 / 分支 port/region-intent-wire-1（独立工作树 D:/Vit_DAW_wt_region_intent_wire_1——主树有 VitApp workspace 运行态未提交改动与当日冒测 coord/runs 残留，按协议 §3 不共用主工作树）
- 回执：（commit hash / 实锚路由形态 / 新用例清单 / 端测边界声明——未起真栈；真栈手测复验点=前端 range 工具框选 clip 局部→主对话说"把这段拆出来"→确认卡→拆分发生在框选边界）
- 验收：（裁定文件 / 验收 commit）
