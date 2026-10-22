# FS-NAMED-DISCLOSE-SUPPLY-1 真栈 chat 补证腿结果（run fs_disclose_chat_20261022_214407）

- 执行：GLM-5.3 L2 PC ZCode 执行会话（170 卡会话）；2026-10-22 21:45 占用登记（2f6c942d@coord/fnds170-claim 推共享 main，预检五端口/五类进程=0 在卷）→ 21:46 起栈 → 21:46–21:48 A5/B1 驱动+断言 → 21:49 拆栈复核（五端口 tcp=0、VitApp/VitAgent/vsphub/VitHeadless/Godot=0）。时戳=本会话机器钟（与 169 会话同钟系）。
- 被测（§9 契约）：agent=主检出 `agent/bin/VitAgent.exe` SHA256 前 8=**6CE34E54**（2026-10-22 21:40 构建，主管预备二进制；构建晚于 e5e53b31(169 合入 21:37:50)+3e72c5c4(170 merge 21:37:57)=**含 169+170**；169 别名表「钢琴/键盘/贝斯」特征 bytes 亲核在卷，170 注释不进二进制，以本腿行为面断言兜底）；vsphub=主检出 B10CC412；kernel=主检出 K3 Release 只读复用 bringup 哈希校验 **722A79D7A3F53797…** 在卷；VIT_PROJECT_XML 指 run 目录 kernel_workspace 副本，主检出 workspace 零触碰；不启 UI。
- 驱动：`softguide_exp.py`（157/159/169 留档原件复用，本卡目录留档）`--arm B1 --variants A5,B1`——A5=159 逐字「请先观察工程，再给一个有界的小步改进建议：给人声加一点板式混响」（159 A5 失败话术复刻）；B1=「给人声加点空间感（M2 场次原话术）」（用户派发指令口述话术，159 B1 原文）。fixture=159 六 stem 配方确定性重建（STEM_SPECS，轨 ID 1007/1012/1017/1022/1027/1032 与 159 同构）。
- 驱动 **exit 0** 两变体捕获：A5 settle=awaiting_user（goal waiting_confirmation 停泊确认/选择卡，hops=2）；B1 goal_terminal completed（hops=1）。断言器 **exit 0**（`driver/leg_assertions.json`）。

## 1. 判定面（卡面验收：unt=1032 已解析且渲染披露行在场，模型不再边界自报——159 A1 对照形态）

| # | 事实 | 证据 |
|---|---|---|
| 1 | **unt=1032 已解析（两话术）**：A5 与 B1 的 loop user_named_target 均=**1032** | `driver/arm/{A5,B1}/chat_1.json` workflow_data.free_state_reasoning_loop |
| 2 | **渲染 prompt 披露行在场（159 A1 对照形态逐字一致）**：两变体各 4 渲染轮，turn2/turn3 的 turn_directives 均含 `user_named:1032 tracks=1032`（frontier 行尾，`A closure candidate frontier is now available: …; user_named:1032 tracks=1032. The runtime resolves this frontier…`） | `logs/agent_prompt_render.jsonl` 按 conversation_id 过滤亲核 |
| 3 | **159 A5 分叉条件下行为翻转（最强对照）**：两变体终态 frontier 均为 5 候选=4 machine+`user_named:1032`，且 **machine 候选恰好覆盖 1032**（A5 `candidate:64f4f69ef9ce2d23 tracks=[1017,1027,1032]`、B1 `candidate:f2d5bd10891f6975 tracks=[1017,1027,1032]`——与 159 A5 抑制触发候选 `17ba… tracks=[1017,1027,1032]` 同形态）**而 user_named 行同时在场**——159 中该覆盖形态即抑制披露行致边界自报，本轮覆盖与披露行共存 | `driver/arm/{A5,B1}/chat_1.json` workflow_data.minimal_audio_closure.hypothesis_frontier |
| 4 | **模型不再能力边界自报**：两变体 reply+events 全文零命中 159 F-gate 签名标记（「没有出现在任何已披露的工程事实」「无法圈定」「任务在能力边界停止」）；终局 A5=停泊确认/选择卡（waiting_confirmation，157 A1 同族合法停泊）、B1=completed——159 A5 同话术为 completed+能力边界自报+零变更 | `driver/arm/{A5,B1}/{chat_1,events}.json`；`driver/leg_assertions.json` |
| 5 | **159 复刻对照成立**：159 A5（unt=1032 已解析、渲染 4 轮零 user_named 行、F-gate #2 全停）→ 本轮同话术三面全反（行在场+停泊非边界） | 159 RESULTS §2/§3 对照；本表 #1–#4 |

## 2. §8 记账

- LLM=**12 次全 deepseek-flash、0 error**（遥测 agent_llm_telemetry.jsonl 在卷；159 同模型）；fixture 构建零 LLM（确定性 stems+import）。
- 轮次：2 驱动变体轮全部有效捕获，0 驱动缺陷轮，0 环境中断，0 原样重跑（一次过）。
- 行为面（停泊/终局形态）为模型概率面：本腿判定主面=机械断言（#1–#3 确定性）；#4 无边界自报为单轮真栈事实，159 复刻对照增强但非稳定性证明（§7 口径）。
- 内核增量：run 目录 kernel_workspace 副本内；主检出/staging workspace 零触碰。

## 3. 端测边界声明

- 本腿=agent HTTP/ZMQ 面 chat 链真栈（真实内核+真实 deepseek 链+工程副本隔离）；**不含** Godot GUI 渲染面与 webui 渲染面断言（本卡改动=Go chat/fold 面，零渲染面文件）。
- A5 停泊在确认/选择卡（hops=2）、B1 completed——提案面到达/装载/总线执行非本卡判定面（159 判读域为披露供应面；169 同款边界声明先例）。
- 170 二进制行为面证据以本腿为界：未单独重跑 169 判定面（A2/A4 别名解析）——169 真栈腿已由该卡收口，本腿二进制含 169 为时间线+特征 bytes 推定。

## 4. 工件清单（随 port/fs-named-disclose-supply-1 入库；重件留盘 run 目录可回指）

- `driver/arm/{A5,B1}/`：prompt/chat_1/events/runtime_status_final/send_list 前后/project_state_after/walkhop/variant_summary
- `driver/{authority_switch,authority_probe,leg_assertions,session_summary,session_log}.json|txt`、`driver_console.txt`
- `driver/fixture/softguide_fixture.vit`（六 stem 工程件）；`logs/`（agent_last.log、遥测、prompt_render、vsphub、pid 三件、kernel_sha256.txt、MaterializeMetrics.log）
- 驱动/起栈/断言脚本在卡目录（softguide_exp.py=169 件复用、bringup_fnds170.ps1=169 件改造主检出二进制路径、assert 逻辑内联 leg_assertions 产出）
- **留盘不入库**：`driver/fixture/{fixture_stems,.vit_history,.vit_derived,.vit_agent}`（确定性可再生/运行时会话态）、`kernel_workspace/`（运行时副本）
