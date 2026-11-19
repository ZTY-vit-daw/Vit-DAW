# W02-A1：每轮课程上下文与选中 clip 绑定（最小切片设计）

- 池序 432；P1；PC；发卡=lead（2026-11-19）；派发确认=**已确认（用户 2026-11-19「可以开始」；lead 发卡）**；验收负责人=lead。
- 依赖：W01（425 真栈 PASS，已收口）；374/375 已合入；W08 的拍号面由 375 覆盖；W18 未建（本切片不依赖）。
- 目标：让真实 LLM 轮看到当前课程节点/材料/选中 clip 的**有界**上下文；推进检查读取学生**选中 clip**而非轨道首个 clip；不改冻结契约 R1–R6。
- 文件域（A1a）：`agent/internal/chat/edu_jam_chat.go`、`agent/internal/edu/jam_state.go`、`agent/internal/chat/edu_jam_chat_test.go`。

## 审计发现（2026-11-19）

1. `edu_jam_chat.go:876-883` 每个 runtime slot 取**首个 MIDI clip**；请求上下文已有 `selected_clip_id`/`selected_clip_ids`/`piano_roll_focus_clip_id`，当前未消费。
2. `chatContext[edu_jam_pullback]` 只在 chat 层写入；`contextruntime.Build` 只投影固定字段，任意 chatContext 键不会自动进入模型快照/提示。W02 的上下文注入不能照搬该面。
3. 非命令轮交给既有 agent loop；没有课程上下文时模型不知道当前节点/材料。确定性「写好了」是模板回话，不是 LLM 反馈。
4. W18 未建；374 的谱面模型在 webui，Go 侧无纯转换包。本切片用 MIDI note start/length（拍）做结构检查，不做记谱拼写。
5. 设计源材料/资产已编译进卡（`assets`/`notes`），可按 ID 引用；不得注入整份 `design.v1.yaml`。

## 切片（建议分两小卡）

### A1a 选中 clip 绑定（零栈，先做）
- 变更：`observeEduJam` 接受请求上下文；若选中 clip 在当前节点 `clip` 槽内，使用该 clip；否则保留 first-clip 并披露 `selection_unmatched`；`edu.Observation` 增加 `clip_ids`/`clip_scope`（additive，omitempty）；推进回执披露所用 clip id/scope。
- 验收：fake kernel 双 clip 用例——选中第二 clip 时 notes 来自第二、scope=selected；无选中时 first+scope=first；跨槽选中→unmatched 披露；`go test ./internal/edu ./internal/chat` PASS。

### A1b 有界课程上下文注入（需先定 prompt 路径）
- 文件域：`agent/internal/chat/goalrunner_chat.go`、`agent/internal/agentloop`（新增 lesson context 渲染面；二选一：promptruntime section / contextruntime additive field）。
- 内容上限：card id/title、node id/title/goal、选中 clip/track、材料/资产 ID+purpose（≤N）、允许组件；不注入全文/原图/整份设计源。
- 验收：模型快照/提示中出现 `<lesson_context>`（或契约字段），字节上限；无会话零注入；非命令轮模型引用节点/材料；`go test` + 渲染断言。

### A1c 真实栈+LLM 旅程（候栈）
- 一局内：选中 clip→「写好了」用选中 clip 判定；自然提问得到引用当前节点的 LLM 反馈；失败不伪通过；重开恢复。

## 停止条件

- A1b 若需改冻结契约/新增设计源 schema，停手上交；A1a 独立价值先落。
- 真实栈选中 clip API 不可达时记录依赖缺口，不以 first-clip 冒充选中 clip 通过。

## 建议

先派 **A1a**（零栈、独立价值、可单测）；A1b 单独设计 prompt 路径后再实现。A1a 通过后再决定是否直接进 A1c 真实栈。

## 领取（2026-11-19 18:42）

- owner=`w02clip`（DeepSeek-V4.1-Flash，PC，独立 worktree）；分支 `codex/w02-a1a-clip-432`；worktree `C:/Users/timoz/.codex/worktrees/pc432-clip/Vit_DAW`；基线 main `b9c9d943`。
- **本轮范围=A1a**（选中 clip 绑定 + 单测）；A1b/A1c 留后续。
- 文件域：`agent/internal/chat/edu_jam_chat.go`、`agent/internal/edu/jam_state.go`、`agent/internal/chat/edu_jam_chat_test.go`（如需新测试文件限 `agent/internal/edu/*_test.go`）。
- 执行侧只推 `codex/*` 分支，不合入 main；回执≤15行。

## A1a 验收（2026-11-19）

- commit `66ad1d66`（分支 `codex/w02-a1a-clip-432`）→ main `c73f53b2`；4 文件。
- 主管亲跑：`go test ./internal/edu ./internal/chat -count=1` exit 0（edu 0.586s / chat 119.996s）；`go build ./...` exit 0。
- 证据：[RECEIPT](../../runs/W02-A1a/RECEIPT.md)、[裁定](../../rulings/2026-11-19-W02-A1a-selected-clip-pass.md)。
- 边界：跨槽选中=披露+回落首 clip（卡面口径）；用户可见文案未加 disclosure；真实栈 A1c 未做。
- 432 保持 doing；下一步 A1b（有界课程上下文注入，先定 prompt 路径）或 A1c。


