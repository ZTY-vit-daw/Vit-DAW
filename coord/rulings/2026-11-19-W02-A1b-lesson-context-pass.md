# 裁定：W02-A1b 有界课程上下文注入 PASS

- 日期：2026-11-19 19:54 +0800
- 卡：432 W02-A1（A1b 切片；A1c 未做）
- 裁定：**A1b 技术 PASS**；432 保持 doing（A1c 真实栈选中 clip + LLM 反馈待发）。
- 实现：分支 `codex/w02-a1b-context-432` @ `a18bf801` + `405423dd`；验收合入 main `f0bdf590`/`7ef087be`；6 文件。
- 证据：[RECEIPT](../runs/W02-A1b/RECEIPT.md)、[go-test.log](../runs/W02-A1b/go-test.log)。

## 验收事实（主管亲跑）

- `cd agent; go test ./internal/contextruntime ./internal/chat -count=1` **exit 0**（contextruntime 1.008s / chat 140.128s）。
- `go build ./...` **exit 0**。
- `contextruntime` 新增 `LessonContextKey="edu_jam_lesson_context"` 与 `Snapshot.LessonContext map[string]any json:"lesson_context,omitempty"`；Build 从 request context 白名单重建/clone，重施 title 200 / goal 500 / assets+allowed 8 上界；无键/空/非 map 时 omitempty；旧快照反序列化兼容。
- chat 非命令轮且活跃局时注入；命令轮（如「写好了」）、无局、终态均不注入。
- 组装后的模型 user 节断言含 `lesson_context`、卡/节点/选中 clip，且不含 asset 文件路径；无键快照字节不变；截断时 `truncated=true`。
- track 优先级修正为 `piano_roll_focus_track_id > selected_clip_track_id > focused_track_id > selected_track_id > track_id`（与 `conversation/intent.go:462` 同序）。

## 边界

- 未做 A1c 真实栈/LLM 旅程；本轮零栈。
- context 约 1KB，不进 hot/warm 预算段；未观察到冻结快照/溢出风险。
- `assets` 只投 id/kind，不投 file/sha256/notes；整份设计源/原图永不进上下文。

## 下一步

- A1c：真实栈选中 clip + 真实 LLM 提问/反馈 + 失败不伪通过 + 重开恢复；候栈窗口。
