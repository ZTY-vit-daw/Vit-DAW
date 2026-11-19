# W02-A1b 回执（有界课程上下文注入）

- 状态：完成（执行侧自验 + 主管验收）。
- 分支 `codex/w02-a1b-context-432` @ `a18bf801` + `405423dd`；合入 main `f0bdf590`/`7ef087be`。
- 6 文件：`contextruntime/context.go`(+test)、`chat/edu_jam_chat.go`(+test)、`chat/edu_jam_lesson_context.go`(+test)。
- 实现：contextruntime `LessonContextKey` + `Snapshot.LessonContext` additive/omitempty；Build 白名单 clone/裁剪（title 200/goal 500/assets+allowed 8）；chat 非命令轮有活跃局注入；命令/无局/终态不注入。
- track 优先级：`piano_roll_focus_track_id > selected_clip_track_id > focused_track_id > selected_track_id > track_id`（与 `conversation.intent` 同序）。
- 主管亲跑：`go test ./internal/contextruntime ./internal/chat -count=1` exit 0（1.008s / 140.128s）；`go build ./...` exit 0。
- 关键断言：组装后的模型 user 节含 `lesson_context` 及 card/node/clip，且不含 asset 文件路径；旧快照无键字节不变；上界截断 `truncated`。
- 边界：未做 A1c 真实栈/LLM；context 约 1KB，不进 hot/warm 预算段。
- 工件：本回执、`go-test.log`。
