# 裁定：W02-A1a 选中 clip 绑定 PASS

- 日期：2026-11-19 18:56 +0800
- 卡：432 W02-A1（A1a 切片；A1b/A1c 未做）
- 裁定：**A1a 技术 PASS**；432 保持 doing（A1b 课程上下文注入、A1c 真实栈旅程待发）。
- 实现：分支 `codex/w02-a1a-clip-432` @ `66ad1d66`（父 `b9c9d943`），验收合入 main `c73f53b2`；4 文件（edu_jam_chat.go / edu_jam_chat_test.go / edu/jam_state.go / edu/jam_state_clip_test.go）。
- 证据：[RECEIPT.md](../runs/W02-A1a/RECEIPT.md)、[go-test-edu-chat.log](../runs/W02-A1a/go-test-edu-chat.log)。

## 验收事实（主管亲跑）

- `cd agent; go test ./internal/edu ./internal/chat -count=1` **exit 0**（edu 0.586s，chat 119.996s）。
- `go build ./...` **exit 0**。
- 单测覆盖：双 clip 选中第二→notes 来自第二、`clip_scope=selected`；无选中→first；跨槽选中→`selection_unmatched` 披露并回落首 clip；三键解析优先级；`Observation.clip_ids/clip_scope` additive/omitempty 往返。
- 既有测试只机械改签 `observeEduJam` 参数；无断言删除/放宽。

## 实现口径

- `chatContext` 全链透传：`handleEduJamChat → serveEduJamCommand → eduJamAdvance/eduJamClosurePass → observeEduJam`。
- 选中解析优先级 `piano_roll_focus_clip_id > selected_clip_id > selected_clip_ids[0]`（仓库 chat 纪律同文）。
- 仅当前节点 `done_when` 引用的判定槽内命中选中 clip 才采用；`reference.clip` 参照槽不由选择改写。
- `Observation` 新增 `clip_ids`（槽→实读 id）、`clip_scope`（selected|first|selection_unmatched），additive/omitempty，旧 JSON 契约不变。
- 推进/闭合的 `workflow_data` 披露 `clip_scope`、`clip_ids`、`selected_clip_id`。

## 边界（必须随卡记录）

1. **跨槽选中当前为披露+回落首 clip**（卡面 A1a 口径），不是 fail-closed；真实旅程若要求「选中不在本关轨道即不判过」，留 A1c/后续裁定。
2. 用户可见回复文本未加 disclosure，披露只在 `workflow_data`；UI/文案留 A1b/A1c。
3. `ClipScope` 是观察级单值；多槽时逐槽实读 id 见 `clip_ids`。
4. 未做真实栈/真实选中 API 腿（A1c）。

## 下一步

- A1b：有界课程上下文注入（先定 prompt 渲染路径）；A1c：真实栈选中 clip + LLM 反馈 + 恢复。
