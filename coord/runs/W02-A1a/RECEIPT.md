# W02-A1a 回执（选中 clip 绑定）

- 状态：A1a 完成（执行侧自验），commit 66ad1d66，分支 codex/w02-a1a-clip-432（基线 b9c9d943）。
- 改动文件（4）：agent/internal/chat/edu_jam_chat.go、agent/internal/chat/edu_jam_chat_test.go、agent/internal/edu/jam_state.go、agent/internal/edu/jam_state_clip_test.go（新增 edu 测试）。
- 实现：handleEduJamChat → serveEduJamCommand → eduJamAdvance/eduJamClosurePass → observeEduJam 透传 chatContext；选中 clip 解析优先级 piano_roll_focus_clip_id > selected_clip_id > selected_clip_ids[0]（chat 纪律 server.go:6583 同文）。当前节点 done_when 引用的判定槽（target.clip / clip_exists.clip / track_non_empty.track）内命中选中 clip → 读该 clip 且 clip_scope=selected；否则回落槽内首个 MIDI clip 并披露 selection_unmatched；无选中=first。reference.clip 参照槽不由选择改写。
- Observation additive：clip_ids（槽→实读 clip id）、clip_scope（selected|first|selection_unmatched），json omitempty，旧契约不变（doctor 测试：jam_state_clip_test.go）。
- workflow_data 披露：推进成功/失败、闭合 blocked/inquiry 均带 clip_scope、clip_ids、selected_clip_id（未匹配时必现）。
- 验收命令（agent/ 下，均 exit 0）：
  - go test ./internal/edu ./internal/chat -count=1（edu 0.764s；chat 151.609s）日志 coord/runs/W02-A1a/go-test-edu-chat.log
  - go build ./...
- 关键测试：TestEduJamAdvanceUsesSelectedClipInCurrentNodeSlot（双 clip 选第二→notes 来自第二、scope=selected）；TestEduJamAdvanceWithoutSelectionUsesFirstClip（无选中→first）；TestEduJamAdvanceCrossSlotSelectionDisclosesUnmatched（跨槽→unmatched 披露且回落首 clip 判过）；TestEduJamRequestSelectedClipIDResolution（三键解析/优先级）；TestObservationClipFieldsAdditiveJSON（additive/omitempty/往返）。
- 既有测试仅机械改签 observeEduJam 调用（新增 nodeID/requestContext 参数），断言未放宽或删除。
- 未解决/边界：未做真实栈（A1c 不在本卡）；用户可见回复文本未加 disclosure（按卡面落在 workflow_data）；piano_roll 优先于 selected_clip_id 系仓库 chat 纪律，若 lead 要求改序为一行调整。
- 工件：coord/runs/W02-A1a/go-test-edu-chat.log；本回执。
