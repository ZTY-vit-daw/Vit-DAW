# FS-NAMED-ALIAS-COVER-1 真栈 chat 腿结果（run fs_alias_chat_20261022_2125）

- 执行：GLM-5.3 L2 PC ZCode 执行会话（169 卡会话）；2026-10-22 21:23 占用登记（4a2b49d3，171 已释放复核在卷）→ 21:24 起栈 → 21:24-21:26 A2/A4 驱动 → 21:27 ③ 披露探针 → 21:28 拆栈复核（四端口 tcp=0、VitApp/VitAgent/vsphub/VitHeadless=0）。时戳统一用本会话机器钟（与 171 回执时戳存在约 1 小时偏差，171 写 22:2x 时本机钟 21:2x，如实注记）。
- 被测（§9 契约）：agent=worktree D:/Vit_DAW_wt_fsnac1 构建 `agent/bin/VitAgent.exe` SHA256 前 16=**19DC2488ABC4ED91**（HEAD ab67d9a1@port/fs-named-alias-cover-1，含本卡改动，基线 origin/main 2bc46406）；vsphub=177359D7CBBFC6C2；kernel=主检出 K3 Release 只读复用，bringup 哈希校验 **722A79D7A3F53797…** 在卷（本卡零内核改动）；VIT_PROJECT_XML 指 run 目录 kernel_workspace 副本，主检出 workspace 零触碰。
- 驱动：`softguide_exp.py`（157 留档原件复用，本 run 目录留档）`--arm B1 --variants A2,A4`，话术=159 逐字（A2「给钢琴加一点混响」/A4「给这条吉他轨加一个轻微的延迟」，prompt.txt 在卷）；fixture=159 六 stem 配方确定性重建（STEM_SPECS：bass/drums/guitar/other/piano/vocals 正弦 stem，轨 ID 1007/1012/1017/1022/1027/1032 与 159 同构）。
- 驱动 **exit 0**，两变体均捕获（settle=awaiting_user、goal=waiting_confirmation、hops=2、零拓扑写入 sends 0->0 / fx 0->0——停泊在确认/选择卡，157 A1 同族停泊形态，提案/执行面非本卡判定面）。

## 1. 判定面（卡面验收：A2 话术 unt 解析命中非空，159 复刻对照）

| # | 事实 | 证据 |
|---|---|---|
| 1 | **A2 钢琴指名解析命中非空**：user_named_target=**1027**，轨名=piano | `driver/arm/A2/chat_1.json` workflow_data.free_state_reasoning_loop.user_named_target；project_state_after 轨名交叉核对（assert_leg.py） |
| 2 | **A4 吉他指名解析命中非空**：user_named_target=**1017**，轨名=guitar | `driver/arm/A4/chat_1.json` 同面 |
| 3 | **159 复刻对照成立**：159 同两话术 unt 缺失（A2→F-gate #1 能力边界自报 / A4→域漂移落 piano/1027）；本 run 两话术均解析命中，160 形态消除 | 159 RESULTS §2/§3 对照；本表 #1/#2 |
| 4 | **持久面在卷**：A2 conversation_graph 携带 unt=1027；A4 runtime_state free_state_reasoning_loops 携带 unt=1017（durable continuation context 同值） | `driver/persist_evidence/{a2_conversation_graph_unt_rows.json, a4_agent_runtime_state.json}`（分片抽取，原件留盘 .vit_history 会话工作区） |
| 5 | **prepare 解析链真栈在跑**：user_named_target_resolution 刷新 ×16 | `logs/agent_last.log` |
| 6 | **③ 披露面真栈命中（可选面，附带验证）**：弦乐指名（闭集别名、六 stem 无弦乐轨）→ unt=None fail-open + workflow_data.user_named_target_unresolved 在场 + 回复带机器常数行「指名轨『弦乐』未在轨集中识别，按自动选择处理。」 | `driver/disclosure_probe.json`（goal=waiting_continue，marker+行双源） |

断言器 `assert_leg.py` **exit 0**（unt 非空+id→轨名交叉核对+探针行核对），输出 `driver/leg_assertions.json`。

## 2. §8 记账

- LLM=**17 次全 deepseek-flash、0 error**（遥测 agent_llm_telemetry.jsonl 在卷）；fixture 构建零 LLM（确定性 stems+import）。
- 轮次：2 驱动变体轮+1 探针轮全部有效捕获，0 驱动缺陷轮，0 环境中断，0 原样重跑（一次过）。
- 内核增量：run 目录 kernel_workspace 副本内；主检出/staging workspace 零触碰。

## 3. 端测边界声明

- 本腿=agent HTTP/ZMQ 面 chat 链真栈（真实内核+真实 deepseek 链+工程副本隔离）；**不含** Godot GUI 渲染面与 webui 渲染面断言（本卡改动=Go chat 面，零渲染面文件）。
- A2/A4 轮停泊在确认/选择卡（交互面 2 hops 后停泊）；提案面到达/装载/总线执行非本卡判定面（157 A1 同停泊先例；观察供应与 fold 披露面属 156/170 卡域）。
- ③ 探针为可选面附带验证：单轮真实 LLM 链上 marker→包装面→回复行全链命中；主判定面仍为 #1/#2 解析命中。

## 4. 工件清单（随 port/fs-named-alias-cover-1 入库；重件留盘 run 目录可回指）

- `driver/arm/{A2,A4}/`：prompt/chat_1/events/runtime_status_final/send_list 前后/project_state_after/walkhop/variant_summary
- `driver/disclosure_probe.json`、`driver/leg_assertions.json`、`driver/session_summary.json`、`driver/session_log.txt`、`driver_console.txt`
- `driver/persist_evidence/`：A2 图谱 unt 行+A4 runtime_state 分片（判定面 #4）
- `driver/fixture/softguide_fixture.vit`（六 stem 工程件 11KB）；`logs/`（agent_last.log、遥测、prompt_render、vsphub、pid 三件、kernel_sha256.txt）
- 驱动/起栈/断言三脚本在上级卡目录（softguide_exp.py / bringup_fsnac1.ps1 / assert_leg.py）
- **留盘不入库**：`driver/fixture/fixture_stems/`（6 wav 1.2MB，配方确定性可再生）、`driver/fixture/.vit_history/`（6.9MB 会话态全量，证据分片已抽取）、`.vit_derived/`（320KB）、`.vit_agent/`（5.9MB）、`kernel_workspace/`（运行时副本）
