# EDU-SYMBOL-CORE-1 段 3 smokerun1 取证——chat 披露层缺口（域外），命令面本身健康

- run：`20261107_002241`（edu_transform_smoke stamp）；PS1 exit **1**（=py exit 3=LLM 行为未达类，按 §8 预写③如实记录，无重跑）
- 计费：9 调用 deepseek-flash（全属本腿对话 `edu_transform_smoke_20261107_002241`），in 142,076（含 cache_read 68,992）/ out 6,658 tok ≈**¥0.15-0.25**（预算 ≈¥0.5-2 内）
- 占栈协议全链：占用登记 82ca20c3（净空亲核 00:19:27 全零+≤60s 复核 00:21:20 全零）→ TEARDOWN_ALL_ZERO 亲核 **00:27:21** 全零 → 释放登记见 PC-RUNTIME-STACK.md
- 被测 agent SHA256 `FD34516D9DB1260293556184F1C8D7C05427035531067BFE7A0219612ECD30E0`；kernel=wt_228 `F9E3DA0A…E0D67A` 只读复用

## 现象

chat 回合（消息点名工具+clip_id+key，agent_mode=chat）内模型调了 `midi.read_clip_notes`/`midi.read_clip_data`（两 reads 均返回 42 音全量，见 chat_response.json executed 行），随后停在 `needs_clarification`，回复明言：**「当前运行时工具目录里没有 midi.analyze_transformations（也没有任何变换分析类工具）」**并给出 A/B 选项——诚实边界申报，零虚构（262 族纪律正确应用）。

## 根因（代码锚点）

chat 能力门控轮的工具允许列表是**第四处注册面**，卡面「三处注册」前提与段 2 均未覆盖：

1. `agent/internal/chat/goalrunner_chat.go:4935-4942` `agentLoopMidiTools()`——硬编码 midi 工具白名单，**无** `midi.analyze_transformations`；
2. 消费链：`goalrunner_chat.go:4375` `agentLoopToolContext` → `toolNamesForAgentLoopCapabilities`（:4408，capability=midi 时取 agentLoopMidiTools）→ `planner.go:176`（AllowedTools=模型可调工具宇宙，系统提示明文「Use only tools from Allowed tools」）；`capability_pack.go:85` 将 pack.Tools 与 allowed 求交后渲染——段 2 的 capability_pack.go 注册行因此对 midi 能力轮**不可见**；
3. 触发条件：`goalrunner_chat.go:4487` `agentLoopCapabilityNames`——消息含「MIDI/音符/旋律/和弦…」即选 midi 能力（本腿消息天然命中——这正是该功能的目标用户话术）；不命中任何能力时走全量 ModelCatalogSummary（296 工具含新工具），但那是偶发逃逸路径而非产品答案。

证据：`logs/agent_prompt_render.jsonl` 渲染目录含 midi pack 工具段但无 analyze 行；`forensics_direct_invoke.json`（拆栈前零 LLM 只读探针）**证明命令面全链健康**。

## 命令面健康证据（direct invoke，拆栈前）

`/agent/invoke {tool:"midi.analyze_transformations", args:{clip_id:"1010", key:{tonic:0, mode:"major"}}, confirmed:true}` → `status=ok`，**38 findings 全量**，黄金三条精确命中：

- 乐句2↔3 调内模进：proto beats 8（bar 3-4）→ match 16（bar 5-6），shift_semitones=2、diatonic_steps=1、label「模进（调内，上移 2 半音）」✓
- 乐句3↔4 原样重复：proto 16（bar 5-6）→ match 24（bar 7-8），shift=0、label「原样重复」✓
- A 段整体再现：proto 0（bar 1-4）→ match 32（bar 9-12）repeat ✓
- analyzed_note_count=42 ✓；真实内核 bar 守门换算正确（tempo_setting_count=1、4/4）

即：catalog→policy direct→harness invokeLocal→edu→内核三条只读命令全链在真栈正确，**与段 1/2 单测期望逐项一致**；失败仅在 chat 披露层。

## 次要观察（记录不扩修）

- 模型澄清回复称「已读回的 22 个音符」——executed 行实际两 reads 均返回 42 音；属模型自行概括失误（口述计数不准），非工具面缺陷；
- 模型给出 A/B 选项（只读分析 vs 写入编辑）属合理澄清行为。

## 修复方向（交主管裁定，三选项）

- **A（最小扩域）**：`agentLoopMidiTools()` 加一行 `"midi.analyze_transformations"`（agent/internal/chat/goalrunner_chat.go 域外一行，263 立规「执行必需挂点卡面预写」的第四处同类面）+ 复跑 chat 腿；
- **B**：另开小卡（chat 披露白名单补齐）+ 本卡段 3 挂其上，先合段 1/2；
- **C**：重构合并双列表（agentLoopMidiTools 与 capability_pack.Tools 冗余是结构性隐患）——更大动作，另行裁定。

## 工件清单

`console_ps1.txt`（ps1 全程）、`edu_transform_console.txt`/`edu_transform_summary.json`/`chat_response.json`（驱动三轮）、`forensics_direct_invoke.json`（探针）、`agent_last.log`、`logs/agent_llm_telemetry.jsonl`+`agent_prompt_render.jsonl`（计费+渲染证据）、`edu_transform_fixture_project.vit`（fixture 工程）、`agent_binary_sha256.txt`、`default_project.xml`+`gui_project_isolation.json`（隔离标记）、`RUNBOOK.md`（跑前成文）
