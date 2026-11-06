# MIXTICK-CONFIRM-CONTRACT-1 证据摘录（来源：257 worktree 原始日志，零改写）

来源文件：`D:/Vit_DAW_wt_257impl/VitApp/Workspace/Logs/agent_message_loop_debug.jsonl`（conversation=mix_single_tick_e2e_20261105_224459 共 29 条）
来源文件：`D:/Vit_DAW_wt_257impl/VitApp/Workspace/Logs/agent_prompt_render.jsonl`（同会话 31 条，turn 4 无渲染记录）

## 一、message_loop_debug 条目清单（stage / error 摘要）

```
#1  parse_failed          Agent 返回的计划格式不是有效 JSON            (T1)
#2  repair_succeeded      (无)                                       (T2)
#3  free_state_final_gate the requested project-level CCB view set already returned usable evidence; choose a different cataloged view or target instead of repeating it   (T5)
#4-#29 同错误串 ×27（T6-T31）
```

## 二、原始输出摘录（逐字节，仅截断不改动）

### debug #2（T2 输出：请求 Track 1 basic_energy，被接受执行）

```json
{"final":false,"reply":"已看到工程里有两条轨道（Track 1、Track 2），也有了它们之间的频段重叠线索；但要给\"小幅音量\"建议，还缺每条轨道本身的电平证据。我先取 Track 1 的电平结构，随后再看 Track 2 做对比。","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"工程结构已确认（Track 1=1007、Track 2=1010），但现有线索只有频段重叠候选，缺少可供音量对比的轨道电平事实，因此先取 Track 1 的基础电平证据。","requested_view_ids":["track.basic_energy"]},"tool_calls":[{"tool":"ccb.observation_request","args":{"view_ids":["track.basic_energy"],"target_ref":{"kind":"track","id":"1007","label":"Track 1"}},"reason":"获取 Track 1 的电平与峰值结构，作为判断哪条轨道需要小幅音量调整的对比依据"}]}
```

### debug #3（T5 输出：首次被弹回——重请求 project 级 set）

```json
{"final":false,"reply":"已拿到 Track 1 的电平数据；还需要工程内的轨道清单和两条轨道之间的相对电平对比，才能指出唯一一处小幅音量调整。","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"现有证据只有 Track 1 的峰值/RMS/动态余量，缺少另一条轨道的身份与两条轨道之间的相对响度关系，无法确定唯一一处小幅音量调整的目标","requested_view_ids":["project.structure","mix.multitrack_relationship"]},"tool_calls":[{"tool":"ccb.observation_request","args":{"view_ids":["project.structure","mix.multitrack_relationship"],"target_ref":{"kind":"track","id":"1007","label":"Track 1"}},"reason":"工程结构与轨道间电平关系可同时披露全部可见轨道身份及彼此相对响度，从而定位唯一需要小幅音量调整的轨道"}]}
```

### debug #15（T17 输出：死锁中段形态——收窄为单 view 重请求）

```json
{"final":false,"reply":"我已经看到第 1 轨的电平结构（峰值约 -6 dBFS、RMS 约 -9 dBFS）。要判断整体混音里哪一轨相对偏响、值不值得做一次小幅音量调整，我还需要看两条轨道之间的相对电平关系。","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"目前只有第 1 轨的单轨电平，缺少工程内两条轨道之间的相对电平对比，无法确定小幅音量调整应落在哪一轨、朝哪个方向","requested_view_ids":["mix.multitrack_relationship"]},"tool_calls":[{"tool":"ccb.observation_request","args":{"view_ids":["mix.multitrack_relationship"],"target_ref":{"kind":"track","id":"1007","label":"Track 1"}},"reason":"获取两条轨道之间的相对电平与对比关系，作为小幅音量调整候选目标的判断依据"}]}
```

### debug #29（T31 输出：切片前最后一轮，仍同形态）

```json
{"final":false,"reply":"已看到轨道 1 的电平，但还缺两条轨道之间的相对响度对比；补取一次多轨关系证据后再给建议。","free_state":{"schema_version":"free_state_decision.v1","status":"needs_observation","evidence_status":"insufficient","summary":"当前只有轨道 1 的电平数据（RMS -9.0 dBFS、峰值 -6.0 dBFS、余量约 6.0 dB），工程共两条轨道，但缺少另一条轨道的响度以及两条轨道之间的相对关系，无法判断哪条轨道值得小幅调整或调多少。","requested_view_ids":["mix.multitrack_relationship"]},"tool_calls":[{"tool":"ccb.observation_request","args":{"view_ids":["mix.multitrack_relationship"],"target_ref":{"kind":"track","id":"1007","label":"Track 1"}},"reason":"该视图披露多轨之间的电平与关系差异，是判断两条轨道中是否存在响度失衡、进而给出一个小幅音量调整建议所需的对比证据"}]}
```

**判读**：29 条输出全部为结构化 free_state 决策 JSON + tool_calls，无一条终局文字建议——「观察→终局文字建议不挂卡」分支形态在原始输出层面不存在。

## 三、prompt_render 指纹面（turn / fingerprint 前 12 位 / sys_len / dir_len）

```
1  20c8305adad1 17670 21616
2  063f76f758ac 17670 21863
3  f2c2ebc886d8 17670 21863
5  bbef34dfdbb4 17670 21863
6  fd94d36acf7d 17670 21863   ┐
7  fd94d36acf7d 17670 21863   │
…（8-30 全同）                │ ×26 恒定（弹回反馈未进入重建请求）
31 fd94d36acf7d 17670 21863   ┘
32 d842ad0eec3f 17670 22496   ← 续跑 settle 注入 +633
```

（turn 4 无渲染记录；telemetry 计费面同指纹序列，turn 6-31 input_tokens 恒 16822。）

## 四、turn 32 vs turn 6 turn_directives 唯一差异（difflib.SequenceMatcher insert 块，逐字节）

```
A target-level observation for the closure candidate has just returned in this turn. Further observations consume window budget (closure rounds remaining: 4/6; continuations remaining: 5/6). The evidence you hold is sufficient for a bounded proposal. Use needs_action only when the returned target evidence supports a deterministic governed action; when it plausibly relates to the user's listening goal but cannot prove an objective defect, use needs_experiment with one bounded improvement_proposal.v1 citing the returned observation. Use blocked only for a concrete capability, freshness, authorization, or observation bound
```

（现行代码位：agent/internal/agentloop/ccb_model_prompt.go:292。）

## 五、agent_last.log 关键行（坐标=工件 agent_last.log 行号）

```
:169 22:45:02 [free-state.seed] booked … revision=6 scan_satisfied=true
:187-199 T1-T3 观察工具腿（catalog 12ms / request 152ms / request 887ms）
:190-193 [tim.assert] track=1010 clipping_headroom fail value=0 / level_ceiling fail value=0（×4 行，22:45:21/42）
:293 22:48:06 [free-state.ledger] boundary executed=3 ccb_observations=2 … stored_receipts=0
:295 22:48:06 [continuation.arm] status=waiting_continue stop=limit_reached durable=cont_9474da5d… scheduled=true
:296 22:48:06 [continuation.stall] reason=invocations-active drivable=1
:297 22:48:06 [timing] agent_loop_chat total_ms=186939 … status=waiting_continue stop=limit_reached
:304 22:48:07 [continuation.claim] claimed id=cont_9474da5d…
:316 22:48:17 [timing] message_loop.llm turn=32 … chars=61802
:317 22:48:17 [free-state.ledger] boundary executed=0 … stored_receipts=2
:319 22:48:17 [continuation.arm] status=completed stop=done … continuation=false requires_user_interaction=false owes_outcome=false
:321 22:48:17 [timing] agent_loop_chat total_ms=10126 … status=waiting_confirmation stop=improvement_proposal_confirmation_required
:322 22:48:17 [f6.gate] chainEnded=true status=waiting_interaction respGoal=waiting_confirmation runtimeGoal=waiting_confirmation
:324+ 22:48:19/26/33 [continuation.park] answerable park holds the chain non_terminal=1 skipped_status=1
```

## 六、复算方法（零 LLM，可重放）

- 指纹/轮次表：python 遍历两 jsonl，按 conversation_id 过滤后打印 turn/fingerprint/长度（本会话命令见 262impl 会话记录）。
- T6/T31 请求恒定：prompt_render fingerprint 全等 `fd94d36acf7d`（prompt_fingerprint 为请求全文哈希）。
- directives 恒定：r5==r6 逐字节比较 True；T32 差异=difflib opcodes 唯一 insert。
