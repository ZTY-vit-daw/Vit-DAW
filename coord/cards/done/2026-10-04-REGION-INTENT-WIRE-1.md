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
- 回执：
  - **commit**：`bd60a799`（分支 `port/region-intent-wire-1`，工作树 D:/Vit_DAW_wt_region_intent_wire_1；基线 HEAD dce9c3b8=origin/main，领取时主树仅 VitApp workspace 运行态改动+coord/runs 未跟踪残留，与本卡文件域无重叠）
  - **验收命令实测**：`go build ./...` OK；`go test ./internal/conversation ./internal/chat -count=1` → `ok 0.298s` / `ok 88.578s`（含 chat 全量回归）；gofmt 按卡口径 `git show HEAD:<file> | gofmt -d` 两文件均 0 行（工作树 CRLF 为 autocrlf=true 检出产物，blob 已验 LF clean）。diff --stat：intent.go +122/-1、intent_test.go +219（新文件），无越域文件。
  - **实锚路由形态**：
    1. **不新增意图分支**——`conversation/intent.go` 既有 split case 条件扩为 `isClipSplitText(text) || len(rangeSplitCommands) > 0`；范围形态全部守卫在 `clipRangeSplitCommands`：话语含"这段/这个范围/框选"+split 动词（既有 isClipSplitText 词表或**拆出/拆开/拆分**）+无口述秒数+无播放头指涉+`selected_clip_ranges` 在场且可解析。
    2. **内核命令组合=两次 split（同 clip_id，先终点后起点）**：锚 `VitApp ClipService.cpp handleSplitClip(1489-1580)`——split_time 为全局秒时间线位置、须严格在 clip 内且两侧留材（kMinimumSurvivingClipLengthSeconds=0.01）、拆后**原 clip_id 保留为左半**（response left_clip_id/right_clip_id）。故先切 range 终点再切起点，框选段独立成 clip，两命令对拆分前几何各自有效，经 `policy.Analyze→RiskConfirm PendingPlan` 按序执行（server.go chatResponseForCommands），确认卡呈现两条、零治理改动。
    3. **边界退化**：range 贴 clip 起点→只切终点；贴终点→只切起点；整 clip 框选→不切。异形（缺 clip_id/track_id、start/end 非数值、start≥end、缺 clip_start/end_seconds、越界）→ fail-open 不取范围形态。
    4. **申报的形态决定（均在卡面授权内自查）**：(a) 播放头守卫——话语同时指涉播放头/这里/当前位置时保留既有 playhead 形态，ranges 不参与（卡面条件"无口述时间"之外保守加的既有切点信号）；(b) 动词门——旅程话术"把这段拆出来"不含现有 split 词表，经 expand 别名（这段→"this clip selected clip"）过 mentionsClip 门，split 路由扩入 拆出/拆开/拆分（仅 ranges 在场生效；"拆掉"不入表防误撞删除意图）；(c) goal 1 落地=selectedClipArgs 镜像 server `contextClipRangeRows` 口径解析（须 clip_id+键白名单，异形跳过），注记 `selected_clip_range_count`+首行入 `args.selected_clip_range`，ids 缺席时首行 clip_id 兜底为选择目标（口述时间路径同样享受兜底+注记，切点仍按口述值）；(d) 已知边界——仅含"框选"而无 clip 信号词（这个/这段/选中/split 动词）的话语被既有 mentionsClip 门挡（与今日一致，未扩）。
  - **新用例清单**（intent_test.go，8 条）：CutsAtRangeBoundaries（两次切序=先终点后起点）/ WithoutRangesUnchanged（无 ranges 零变化：拆出话术回落既有 select 路由、切开话术旧行为）/ MalformedRangesIgnored（6 种异形 fail-open）/ MultipleRangesUsesFirst / SpokenTimeWins（口述时间优先）/ BoundaryRanges（贴界单切×2+整 clip 不切）/ PlayheadKeepsExistingPath / SelectedClipArgsConsumesRanges（兜底/ids 优先/count 注记/异形无注记）。
  - **端测边界声明**：未起真栈（内核/Godot/agent 三件套未动，纯 Go 意图层逻辑+单测；不涉 webui 渲染面）。真栈手测复验点（卡面原列）：前端 range 工具框选 clip 局部→主对话说"把这段拆出来"→确认卡（两条 clip.split）→拆分发生在框选边界。补充复验点：口述秒数切分仍按口述值；无框选时旧话术行为不变。
  - message_loop 未接线（无需）；server 白名单/结构校验、catalog、webui、Godot 零改动。
- 验收：conditional pass（[rulings/2026-10-04-REGION-INTENT-WIRE-1-conditional.md](../../rulings/2026-10-04-REGION-INTENT-WIRE-1-conditional.md)，2026-10-04 晚）/ 验收 commit=merge c2edb0d6（合 main）+binary 18:12:21 收口重建；我方复跑 verify_pc（build+conversation/chat 全绿+gofmt blob 补跑 0 行）；两次 split 顺序论证核实成立。~~conditional 维持——转正条件修订（2026-10-04 晚，SMOKE 场景 B 证据）~~ → **转正 pass（2026-10-06 晚窗）**：转正条件①FIX-1 合入 main（ed45a072）②场景 B 严格判据 exit 0（执行侧两轮 20261004_223710/223757+决策侧独立复跑 20261006_185750）全部达成，见 [rulings/2026-10-06-INTENT-WIRE-FIX-1-pass.md](../../rulings/2026-10-06-INTENT-WIRE-FIX-1-pass.md)；口述秒数/旧话术反例覆盖不受影响。范围线收官。
