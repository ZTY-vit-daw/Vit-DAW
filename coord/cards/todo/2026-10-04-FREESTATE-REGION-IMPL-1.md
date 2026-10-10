# FREESTATE-REGION-IMPL-1：自由态范围 goal 两轮编舞——loop 状态 range_goal 字段+轮 1 断言 settle+轮 2 衔接（P2）

> **排序裁定（2026-10-04 晚用户）：推迟至自动化曲线（C4 段落自动化批次——2026-10-10 用户勘误：原 B2 系误记，按 A-F 官方文档 §6.1 自动化=C4；解冻条件随此更正）**——届时范围 goal 编舞按单轮形态（写包络=单变更）直接做终态，本卡两轮制为 a 路径过渡形态不再单独实现；卡与 DESIGN.md 保留为 B2 批次设计输入。**不可在此批领取。**
> **竞态处置（2026-10-04 晚）**：一执行会话开工早于上条裁定（口令排队竞态），完成后发现裁定即停工——未领取未提交、工作树复原，实现成果（build/test 全绿+range_goal 字段+武装+测试）保全于 [runs/FREESTATE-REGION-IMPL-1-STOP/](../../runs/FREESTATE-REGION-IMPL-1-STOP/)。处置：登记为 B2 批次单轮编舞参考输入（两轮逻辑届时重写，字段/武装/测试骨架可评估复用）；代码不进库，裁定效力优先。

- 池序 7（设计卡 FREESTATE-REGION-GOAL-1 已收口）；目标仓库=D:\Vit_DAW（PC 执行侧）；设计=[DESIGN.md](../../runs/FREESTATE-REGION-GOAL-1/DESIGN.md)（锚点地图 D1-D7+协议全文，实现以它为准）
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / main c1f95286；REGION-INTENT-WIRE-1 已合入（split 切序先例）
- 模型分级：L1 / **flash 可接**（先例=goalrunner 轮状态机+INTENT-WIRE）
- 已核实事实（DESIGN 锚点，实现前回查）：
  1. ranges 已达 agentloop 组装（helpers.go:143-145 白名单）；beginChatGoal 整体收 requestContext（goalrunner_chat.go:32）。
  2. 轮状态机挂点=free_state_reasoning_loop（goalrunner_chat.go:280-330 分支族+audio_closure_controller.go admit/record）；D1 prompt（ccb_model_prompt.go:131/133）**零改动**。
  3. split 子 id 回传=ClipService left/right_clip_id（RECON A2）；切序先终点后起点（INTENT-WIRE 已合 main）。
- 目标：
  1. **loop 状态增 range_goal 可选字段**（DESIGN 问 1 结构体：clip_id/track_id/range_start_s/range_end_s/phase[split_pending|split_done|treating]/sub_clip_id）——fail-open：旧 loop 状态无键=普通 goal 零影响；持久化兼容测试（AGENTS §11：旧 JSON 反序列化+往返）。
  2. **轮 1 编舞**：goal 创建时（目标话术+ranges 在场，话术分界判定本卡用最简形态：含 treatment 类动词且 ranges 在场——IMPL-2 再精化）置 range_goal.phase=split_pending；轮 1 mutation=split×2（复用 INTENT-WIRE 命令形态）；post 断言=子 clip id 回传+时长≈range 宽（容差 0.02s）；断言过→phase=split_done+记 sub_clip_id+自动开轮 2；不过→blocked（证据上交）。
  3. **轮 2 编舞**：操作目标=sub_clip_id（treatment 命令的 clip_id 解析优先取 range_goal.sub_clip_id）；判定/结算走既有链零改动。
  4. **轮间中止**：split_done 后会话中断/goal 停止→子 clip 留存，goal Summary 记"已拆分待调改"；不自动 revert。
  5. 测试：range_goal 字段兼容反例+两轮编舞单测（拆分断言过/不过/中止留存三轮态）+既有 chat/goalrunner 回归全绿。
- 文件域：agent/internal/chat/goalrunner_chat.go+audio_closure_controller.go（如需）+测试文件；**ccb_model_prompt.go 不碰**；intent.go/webui/Godot 零改动（IMPL-2 域）。
- 约束：D1 prompt 文案零改动（红线）；loop 状态 JSON 兼容义务；真栈泊位（E2E 归 IMPL-2 扩组）。
- 验收标准：`go build ./...`+`go test ./internal/chat ./internal/conversation -count=1` 全绿+新用例+gofmt blob 净；回执附两轮编舞锚点清单。
- 停止条件：轮状态机挂点与设计假设不符（free_state_reasoning_loop 不可承载 per-goal 字段）→ 锚点+形态清单上交。
- 领取：（时间 / origin/main hash / 分支名）
- 回执：（commit hash / 编舞锚点 / 新用例 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）
