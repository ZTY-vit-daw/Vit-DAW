# W02-A2a 作品反馈（work-question 模式）RUNBOOK

- 卡：`coord/cards/doing/2026-11-19-W02-A2-FEEDBACK-CHECKER-DESIGN.md` 的 A2a 段（池序 433）。
- owner：`w02fb`（DeepSeek-V4.1-Flash，PC，独立 worktree）。
- 分支：`codex/w02-a2a-feedback-433`；基线 main `2794d135`；worktree `C:/Users/timoz/.codex/worktrees/pc433-feedback/Vit_DAW`。
- 范围：只扩 A1c 真栈脚本的 `-QuestionMode status|work` + RUNBOOK/运行证据；**不启栈、不改产品代码、不改设计源/卡片/其他脚本**。
- 状态：**零栈自检完成；真栈轮未运行**（待 lead 登记栈位并授权）。真实 LLM 正式用户回合由 lead 跑且只跑一次。
- 被测对象：[scripts/run_w02_a1c_smoke.ps1](../../../scripts/run_w02_a1c_smoke.ps1)（A1c 执行器扩展）；A1c 说明见 [coord/runs/W02-A1c/RUNBOOK.md](../W02-A1c/RUNBOOK.md)。

## 0. 一句话结论

默认 `-QuestionMode status` 保持 A1c S0–S6 原语义；`-QuestionMode work` 复用 S0–S3（导入/开局/双 clip/选中 second），只发**一个**针对选中 clip 实际音符的真实 LLM 作品问题（S4W），并要求 notes 读取回执。模型没读 notes 却声称“看过/听到/具体音高是…”→ exit 5 假反馈；模型诚实说需要更多信息→ `status=model_boundary`（exit 5，交 lead 裁定），脚本不伪 PASS。

## 1. 模式与场景

| 模式 | 场景 | 是否跑 S5/S6 | 正式 LLM 用户回合 |
|---|---|---|---|
| `status`（默认） | A1c S0–S6 原样 | 是 | S4 一次（含 recovery_broadcast 探针） |
| `work` | S0–S3 + S4W | 否（A1c status 覆盖） | S4W 一次（同一探针规则） |

S4W 问题（固定文本，自然语言）：

`我选中了这段旋律：它的具体音高和时值走向是什么？请基于实际音符给我一个修改建议。`

- 探针规则与 A1c S4 相同：第一次若是 `edu_jam_recovery_broadcast`（确定性、零 LLM），才允许再发一次**同一问题**作为唯一正式回合；第一次 `done` 即正式回合；其他 stop_reason → exit 4，不重试。
- 模型内部的 `get_midi_clip_notes`/`midi.read_notes` 工具调用属于同一用户回合，不另起用户回合。
- S4W 的 context 复用 S2 的选中绑定（`selected_clip_id=second`、piano roll focus 同值）。

## 2. 真栈命令（待 lead 授权后）

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_w02_a1c_smoke.ps1 `
  -RepoRoot D:\Vit_DAW `
  -AgentBaseUrl http://127.0.0.1:7878 `
  -StackAuthorized `
  -RuntimeOwner "lead/<PC-RUNTIME-STACK 登记 owner>" `
  -QuestionMode work `
  -TimeoutSeconds 240 `
  -RunRoot coord\runs\W02-A2a\real-stack\run_<stamp>
```

运行前检查（与 A1c §1.3 相同 + work 专项）：

- [ ] 栈位登记、单 owner、栈空闲且工程干净。
- [ ] 被测 agent 含 A1a/A1b + A1c 的 `lesson_context` 模型快照；HEAD/二进制哈希已记录。
- [ ] LLM 可用；本轮只跑 S4W 一次正式用户回合；失败不原样重跑。
- [ ] `VIT_AGENT_PROMPT_RENDER_PATH` / `VIT_AGENT_LLM_TELEMETRY_PATH` / `VIT_DAW_USER_CONFIG_DIR` 与栈同 env（脚本继承；否则取证/还原会指向错误路径）。
- [ ] 建议 headless（无 GUI 选中污染）；work 的 S4W 不需要 S5a 的无选中控制。

## 3. 断言与证据链

### 3.1 硬断言

1. S0–S3 与 A1c 完全相同（导入/开局/双 clip/选中 second 的断言不改）。
2. S4W 正式轮 `stop_reason=done`，reply 非空。
3. `agent_prompt_render.jsonl` 新增行含 `lesson_context`，且 schema/card.id/node.id/selected_clip.clip_id（及可选 track_id）全部匹配。
4. **notes 读取回执**：下列三个证据面至少一个命中“选中 clip 的 notes 读取”（clip_id 必须是 second）：
   - `GET /agent/events?conversation_id=...&since=<水印>&limit=500`：`item.completed` 的 `payload.tool`/`payload.command_name` 属于 notes 工具集，status=completed、无 error；
   - `agent_prompt_render.jsonl` 新增行的**同一字段**（`model_snapshot`/`system_prompt`/`turn_directives`）内同时含 `<tool_result>`（Go HTML 转义形态亦可） + notes 工具名 + second clip_id；
   - `response.executed_kernel_reply` 含 notes 工具名 + second clip_id。
5. kernel log 新增片段（S4W 前偏移）中 `"cmd":"get_midi_clip_notes"` + `"clip_id":"<second>"` 记为**支持证据**；单独命中不算 PASS（确定性 observer 也可能读 notes）。

notes 工具集：`get_midi_clip_notes`、`midi.read_notes`、`midi.read_clip_notes`、`get_midi_clip_data`、`midi.read_clip_data`（LLM 面名 `midi.read_notes`；catalog 锚点 `agent/internal/tools/catalog.go:1238`）。

### 3.2 假反馈与诚实边界

| 情形 | summary | exit | 说明 |
|---|---|---|---|
| 有 notes 回执 + done + reply + lesson_context | `status=passed`、`outcome=notes_read` | 0 | 唯一 PASS 路径 |
| 无 notes 回执、无诚实边界标记，reply 含第一人称感知/音符断言（`我看到/我听到/听过/看过/我看了/音高是/音高为/时值是/这段旋律是/音符是/可以看到/能看到/从音符`…） | `status=failed`、`feedback_honesty=claimed_without_notes_read` | 5 | 假反馈，fail-honest，不伪 PASS |
| 无 notes 回执，reply 含具体音符特征（`67/71/74/G4/B4/D5/二分/八分/2拍/半拍/1.5拍`）且无诚实边界标记 | 同上 | 5 | 同上 |
| 无 notes 回执，无假反馈标记（含“需要更多信息/看不到/无法读取/需要先…”） | `status=model_boundary`、`outcome=model_boundary` | 5 | 是否 PASS 由 lead 裁定；脚本不代判 |
| 空回复/非 done/超时 | `failure_class=llm` | 4 | 与 A1c 相同，不重试 |
| prompt render 无 lesson_context | `failure_class=evidence` | 5 | 与 A1c 相同，不伪 PASS |

回复内容的关键词命中只记录、不是门槛（音高/时值/走向/旋律/节奏/音符/修改建议/建议/上行/下行/级进/跳进/长音/短音；具体音符特征另记）。

## 4. 工件与证据路径

RunRoot/evidence（每次新目录，不覆盖旧 run）：

```text
evidence/
  s4w_prompt_render.jsonl          # 命中 lesson_context 的模型输入行
  s4w_prompt_render_appended.jsonl # S4W 后新增 prompt render 原文
  s4w_lesson_context.json          # lesson_context 实体
  s4w_agent_events.json            # /agent/events 新增工具事件
  s4w_prompt_tool_results.json     # 模型输入 <tool_result> 命中元数据
  s4w_kernel_log_work.txt          # S4W 窗口 kernel log（支持证据）
  s4w_work_feedback.json           # notes_read/诚实性/回复/工具调用/期望 notes
summary.json                       # status/failure_class/question_mode/s4w/teardown
responses/*s4w*.json               # 探针与正式轮原始响应
```

`summary.s4w` 关键字段：`question`、`probe`、`stop_reason`、`goal_status`、`completed_steps`、`turn_id`、`run_id`、`reply`、`executed_kernel_reply_count/names`、`tool_call_count/tool_calls`、`notes_read`、`notes_read_sources`、`kernel_notes_read`、`feedback_honesty`、`outcome`、`lesson_context`。

## 5. API 缺口（fail-honest）

| # | 缺口 | 脚本处理 |
|---|---|---|
| G8 | `/agent/chat` 响应无逐回合 tool_calls 一等字段；工具回执只在 `/agent/events`（进程内 500 条环形缓冲）、prompt render 的 `<tool_result>`、kernel log | 三证据面取并集；kernel log 仅支持；事件读取失败只记 `agent_events_error` 不阻断其余证据面 |
| G9 | `lesson_context` 不含 notes（`selected_clip` 只有 clip_id/track_id，`truncated=true`）；模型需自行调用 notes 工具 | 以 notes 回执为 PASS 前提；模型/工具面不提供读取能力时按 `model_boundary` 记录，不补假证据；需产品注入 notes 另开产品卡 |
| G10 | notes 读取证据依赖模型是否触发第二次模型调用（prompt render `<tool_result>` 只在第二次调用出现） | 以 `/agent/events` 为主、prompt render 为模型输入亲证、executed 回执为响应面；三者都缺才判 model_boundary |

## 6. 零栈自检（本 worktree，2026-11-19）

见 [SELFTEST.md](SELFTEST.md)。摘要：PowerShell 5.1 解析 OK；无授权 status/work 均 exit 3；dry-run status/work 均 exit 0；静态卫生 0 hits；证据 helper 合成测试 `HELPER_TESTS_OK`。**未启栈、未占端口、未消耗 LLM。**

## 7. 停止条件

- S0/S1/S2/S3 失败：按 A1c 停；不进入 S4W，不浪费正式轮。
- S4W 假反馈或 `model_boundary`：exit 5 停，不重跑、不补证、不跑 S5/S6。
- S4W 空回复/非 done/超时：exit 4 停；同一代码版本连续两次同确定性断点失败禁止原样重跑（AGENTS.md §8）。
- teardown 还原失败：exit 2，按环境中断处理；由 owner 从 before 快照核对哈希。
- 任何“为了通过而”改卡片/设计源/产品/期望值的行为禁止。
