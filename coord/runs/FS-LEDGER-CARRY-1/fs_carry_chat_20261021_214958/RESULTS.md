# FS-LEDGER-CARRY-1 真栈 chat 腿判定件（run fs_carry_chat_20261021_214958）

- 执行：GLM-5.3 L2 PC ZCode 执行会话（158 卡）；2026-10-21 21:49 登记（fc7f690c）→ 21:50 起栈 → 22:05 拆栈复核（四端口 tcp 监听=0、VitApp/VitAgent/VitHeadless/vsphub=0；用户 Godot 编辑器未动）
- 被测：agent=worktree D:/Vit_DAW_wt_carry1 构建 `agent/bin/VitAgent.exe`（SHA256 前 16=CDB3BB97D23CD973，基线 612beb18+本卡改动）；内核=K3 只读复用主检出 Release `VitApp.exe` SHA256=722A79D7A3F53797F5713209D812D8CE9F2888CB2A7E743286FA56B99629DFEC（21:49 亲核一致，本卡零内核改动）
- 栈：手动起栈（157 先例形态：kernel 2004 + agent 26420；VIT_PROJECT_XML 指 run 目录工程副本、VIT_HISTORY_DRAFT_ROOT 指 run 目录，主检出 workspace 零触碰）。注：vsphub（8787）未起——agent 以 VSP pending 告警降级运行，种子观察的 revision 绑定经 kernel ZMQ 快照正常（见判定面 1），非本腿阻断面
- 驱动：`scripts/fs_ledger_carry_chat.py`（本卡新增，softguide_exp.py 同族）：M2 六轨合成 fixture（bass/drums/guitar/other/piano/vocals，dev_agent_smoke fullaccess LEG A 逐字配方）→ full_project_access → 同会话双轮；确定性判定面=prepare 边界日志线（`[free-state.seed]`/`[free-state.carry]`），回复语义不断言（AGENTS §8）
- LLM：三轮全真 deepseek 链；§8 预声明在占用登记（1 run/次；round1 未终态=llm_shape 允许一次重跑；同确定性断点×2=止损）

## 三轮经过（同一栈，逐会话新 conversation）

| run | 轮 2 话术 | 驱动 exit | 分类 | 经过 |
|---|---|---|---|---|
| run1 | M2 场次①轮 1 主话术（逐字） | 4 | gate_face_missing_carry | 种子面过；轮 2 被语义入口合法路由到 **147 全接入动态链**（semantic_dynamic_execution_completed，guitar→auto_create Reverb Bus→Pro-R rack 装载全执行）——非自由态路由，无新循环=结转面不适用。**驱动话术选择缺陷**（非产品缺陷） |
| run2 | 146 真 B2「这条钢琴听着太干了，帮我弄润一点」 | 4 | gate_face_missing_carry | 种子面过；轮 1 循环因 nudge「可以执行」被 mix-tick 流截走 goal 完成而**仍处活跃**（observing），轮 2 正确合并延续（D1 实验执行+回读在卷）——前驱非终态=结转面不适用。**驱动 settle 判据缺陷**（nudge 过早，fs_largeproject 判据未先等调度链排空） |
| run3 | 同 run2（B2）+ 修正 settle 轮询 | **0** | **pass** | 双判定面全过（下表） |

## 判定面（run3，conv=fs_carry_chat_20261021_215857）

| # | 事实 | 证据 |
|---|---|---|
| 1 | **A1 种子观察真栈入账**：轮 1 冷启动新循环在 store 前由 server 发 mix.multitrack_relationship 入账，revision 真绑定（=22） | `agent_last.log:1607` `[free-state.seed] booked conversation=…215857 observation=obs_20261021T135908_f7ac6b3f132a receipt=ccbr_obs_…f7ac6b3f132a revision=22 scan_satisfied=true` |
| 2 | **G3 由种子满足**：轮 1 从第 1 拍起 G3 合格（M2 场次②轮 1 断点「整项目扫描凭据未交付+观察窗口耗尽」不再出现——模型照常观察人声/吉他轨并拿到目标级证据） | 判定面 1 的 scan_satisfied=true + 轮 1 ledger boundary 链（log:1614→1643：model 又发 1 次 track 观察 obs_…58f9088 无弹回）；对照 run1 同话术持久面（`persisted_state_run1_conv215005.json`：ledger=种子扫描凭据+track:1032 双视图，status awaiting_experiment——目标级证据到手） |
| 3 | **B 结转真栈生效**：轮 2 新循环由终态前驱播种，`carried_from_loop_id` 血统持久 | log:1671 `[free-state.carry] conversation=…215857 carried_from_loop=free_state_2319aeed5675fc7f receipts=2 scan_satisfied=true`；持久面 `persisted_state_run3_conv215857.json`：轮 2 loop=free_state_d8b1a258…，**carried_from_loop_id=free_state_2319aeed…**，receipts=2（前驱种子扫描凭据 ccbr_obs_…f7ac6b3f[mix.multitrack_relationship ready]+前驱轨级观察 ccbr_obs_…58f9088[track.basic_energy+stereo_space]） |
| 4 | **二轮 ledger 非空、不再零观察结算**：轮 2 出生即带 2 凭据+可用扫描（146 L5 工件形态=turn10 零工具调用 stored_receipts=0 的反转） | 判定面 3 receipts=2 + 轮 2 settle（goal=completed，log:1698 boundary stored_receipts≥1） |
| 5 | **种子谓词与预算语义**：轮 2 结转账本已含合格扫描→不再发种子（无第二条 seed 线） | log 全文 `free-state.seed` 仅 1 条/conv；持久面 run3 loop `seed_observation` 缺省 |
| 6 | **run1 附带面**：M2 场次②轮 2 主话术在其合法路由（147 全接入动态链）上端到端复刻成功——Pro-R 自主选择+send.connect(guitar→auto_create Reverb Bus)+rack 装载零交互卡 | `report_run1.json` rounds.round2 + `driver_round2_chat_1.json`（semantic_dynamic_execution_completed，回复=Pro-R 自主选择文案+63/30 计数准入句）；内核铁证 log（21:51:00 track.send.connect status=ok→tracks 6→7；21:51:01 rack_add_node） |

## 诚实边界（如实记录，非本卡缺陷）

- run1/run2 的 exit 4 两次均为**驱动侧话术/settle 判据缺陷**（§8 允许的重跑类），非产品断言失败；两轮中种子面均已过，产品行为（语义链路由/活跃循环延续）均正确。
- M2 场次②轮 2 主话术在当前栈路由到 147 动态链而非自由态——「二轮型」结转面以 146 真 B2 两轮工件（156 RESULTS L5）为复刻载体；主话术自身经其合法路由端到端成功（判定面 6）。
- 轮 2 持久面 status=blocked（模型诚实自报边界）——终态家族成员，不影响判定面（结转发生在轮 2 出生时点）。
- vsphub 未起栈：agent 以 pending 告警降级，种子观察经 kernel ZMQ 快照正常绑定 revision；本腿不涉 CAS 执行协调面。

## 对 161 会话事故自报的判读回应（PC-RUNTIME-STACK.md 21:5x 自报在卷）

161 执行会话在本腿占用期内未登记触碰了本栈隔离 run 副本（clear_project+K3 探针轨/send/宏，主检出零触碰）。判读：本腿三轮全部带时戳（run1=21:50-51、run2=21:56-57、run3=21:58-22:00）；run3 每轮 `project.open` 从磁盘 .vit 重置副本状态后开跑；**种子/结转判定面=prepare 边界确定性事实**（循环机器+账本+前驱终态），与副本拓扑零耦合；run3 判定面日志线（21:59:08 seed/21:59:40 carry）与持久面（22:03 快照）均在无外部扰动窗内自洽。161 探针腿的具体时戳窗以其自报与 VitIpcClient 4445 日志为准。

## 工件清单（随 port/fs-ledger-carry-1 入库；重件留盘本目录）

- `evidence/`：agent_last.log（全量）+ 三轮 report/chat/prompt/console + 两份持久面 agent_runtime_state.json（run1 conv215005 / run3 conv215857）
- `driver{,_run2,_run3}/`：三轮原始工件（含 fixture/ 与 .vit_history，重件留盘不入库）
- `logs/`：起栈 pid 件与驱动 console；`kernel_workspace/`：隔离内核工作区

## 结论

卡面真栈验收两项判定面（轮 1 种子=G3 满足不再断在扫描凭据；M2 二轮型=结转后二轮 ledger 非空、G3 satisfied、不再零观察结算）在 run3 全部命中，驱动 exit 0。
