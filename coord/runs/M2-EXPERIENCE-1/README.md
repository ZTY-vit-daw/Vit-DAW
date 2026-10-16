# M2-EXPERIENCE-1：体验场次首跑取证（2026-10-15 早窗）

- 场次：M2 时域线体验场次第一次尝试（用户经 Godot 拉起前端，09:52 入场，09:53:11 发话术）。
- 话术（主推版，与 run12/13 同构）：「有几轨听起来比较干，想给其中一条合适的轨加一点混响（衰减短些），不要用整体增益或 EQ 来解决。请先观察工程，再给一个有界的小步改进建议。」
- 结果：**链路未达 LLM**（journal_records=0 / observations_count=0，store 仅 391KB mixboard 快照）；卡死于 ask→slice 前置段 + user_stop 半落地不收敛。
- 基线：main 1f85627e；VitAgent 重建于 10-15 09:49（含 115 修复，排除旧二进制因素）；VitApp pid 28456 / VitAgent pid 21120。

## 时间线（亲证，来源=checkpoint/commit 时间戳 + /agent/state 轮询）

| 时刻(+08) | 事件 | 证据 |
|---|---|---|
| 09:52:30 | VitApp 启动（pid 28456） | 进程表 |
| 09:52:32 | 启动页建**第一个空 draft**（Unsaved.vit 75B） | drafts/draft_20261015T015232_041e9c3d |
| 09:52:32/33 | VitAgent 启动（pid 21120，09:49 构建）+ 建**第二个空 draft**（本场次实际进入，75B/0 轨） | 进程表 + drafts/draft_20261015T015233_4a3e9fef |
| 09:52:36 | agent 启动期写 2026-09 旧 draft 的 agent_runtime_state.json（**42MB**） | 旧 session 目录时间戳 |
| 09:52:59 | 内核 L3 特征日志（.vit_derived，257KB） | 文件时间戳 |
| 09:53:11 | 话术落账：Conversation ask checkpoint + baseline commit | c_20261015T015311_fca1c116 |
| 09:54:14 | mixboard 特征快照落盘（391KB，ask 后 **+63s** 的上下文组装活动） | mixboard_feature_snapshot.json |
| 09:55:13.794 | slice 1 与 turn 1 才启动（**距 ask 122s**）；turn **5ms** 后即 stopped | /agent/state turns/slices |
| 09:55:13.846 | 用户停止请求落账：checkpoint「Vit: 已停止当前 Turn」+ goal stop_requested=true/stop_reason=user_stop | c_20261015T015513_51e17b9d |
| 09:55:13→10:05 | **task=active / slice=running 永不收敛**（goal.updated_at 冻结 09:55:13.794） | state 快照×3 |
| 10:05:2x | 决策侧二次 POST /agent/turn/stop（goal 非 active → finalizeStoppedTurn 内联分支）→ 返回 ok 但 slice/task 仍 running，**设计内恢复路径无效** | 响应体 + after_second_stop 快照 |

## 关键状态（10:03-10:05 快照）

- plan/agent_plan=stopped；goal stop_requested=true、cancel_requested=false、stop_reason=user_stop
- task.status=active；slice 1 status=running、ended_at=zero；turn 1 status=stopped（started/ended 同毫秒）
- shadow：initialized=true、track_count=0、user_track_count=0（空工程实锤）；kernel connected=true（ping ok）
- /agent/events 需 conversation_id（本取证未取到 GUI 会话 conversation id，事件面未覆盖——后续卡可补）

## 工件

- agent_state_1003_check1/2.json、agent_state_1005_after_second_stop.json、runtime_status_1003.json
- 二次停止 POST 响应（见上表 10:05 行，checkpoint_ref=c_20261015T015311 回退到 ask checkpoint=未产出新收口 checkpoint）

## 【2026-10-15 晚窗勘误】「空工程」结论撤回——内核有真实音轨，shadow 计数失真

用户质询「我明确新建一个工程并加载了音轨」成立，原「空工程/误入」结论**撤回**：

- 两轮 draft 的 `.vit_derived/.../l3_feature_log.jsonl` 各有 **18 条真实音频分析**（每条 analyzed_sample_count=1,764,000、16bit、真实 LUFS −24.46/−22.89、立体声平衡、频带动态）——**内核侧工程有真实音轨**，时间=入场即析（09:52:59 / 12:37）；
- mixboard 快照（391KB/400KB）含 track_waveform_envelopes / loudness_summaries / stereo_relation_summaries 等真实逐轨特征；
- 原「空工程」推断错误根因：只读了 shadow track_count=0/user_track_count=0（09:57/10:03 两次查询）与 75B Unsaved.vit（=从未保存的占位文件，不代表内核态），**未交叉核对派生特征数据**——决策侧取证漏审，如实登记；
- **修正后的核心异常=kernel↔shadow 轨道不同步**：内核有轨+特征已析，agent shadow 报 0 轨（delta_seq_gaps=0、last_delta_seq=209——deltas 流过但轨道计数为零）；
- 根因假设（取证腿验证）：**「新建工程+导入音轨」路径与「打开既有工程」路径的 shadow 同步差异**——烟测（run12/13 及 PORT-JOURNEY-1）全部走「打开既有工程」（shadow track_count=6、dad_fact ready=6）；新建+导入路径的轨道增量可能未进 shadow 订阅面或早于 SUB 附着。ask→slice 122s 空窗与 LLM 零调用和此假设自洽：前置组装等待轨道相关就绪（DAD facts），shadow 认为无轨→永等/超时；
- 环境事实（2026-10-15 晚窗核）：用户前端=D:/Godot/project/vit-daw-frontend @**port/fe-gui-shrink-1 c56e38f（10-11 Phase A 开发分支）**，工作树含大量未跟踪测试工程目录；主仓 agent/webui/dist=**10-07 构建**（09:49 Go 重建未含 webui 重建）——用户所见聊天面板为一周前 webui。两项与本故障簇的因果关系未判定，归 117/118 取证。

## 第二次尝试（2026-10-15 12:37，用户自发重试；场景已由用户关闭）

| 时刻(+08) | 事件 | 证据 |
|---|---|---|
| 12:37:31/32 | **「一秒双空 draft」二次复现**（Unsaved.vit 各 75B，与首跑 09:52:32/33 同签名） | drafts/draft_20261015T043731_c86c7a3b + draft_20261015T043732_236b4272 |
| 12:38:23 | 话术 ask 落账（goal_060587d6f1ae2c34 / run_cb7b27ec6aca7b88 / conversation webui_mv91t2mk） | commit c_20261015T043823_99b7c286「Conversation ask」 |
| 12:39:08 | 运行时状态保存（ask 后 +45s）；mixboard 快照再次落盘 | agent_runtime_state.json + mixboard_feature_snapshot.json |
| ~12:4x | 用户报告「问题还是没修」→ 关闭现场；**本次无停止 checkpoint**（仅 ask 一个 commit——未按停或停止未落账，磁盘不可分辨，如实登记） | commits 目录仅 1 件 |
| 12:5x | 决策侧复核现场：VitAgent/VitApp/Godot 进程=0，7878/5555/5556 监听=0 | 进程/端口亲核 |

二次尝试结论：**同版本二进制（09:49 构建）+ 未派发任何修复 → 复现符合预期**；「双空 draft+误入空工程」拿到第二个独立样本；新线索=第二个 draft 的 .vit_history 树内出现首轮工程 uuid（vitproj_69a775…）的 state/worktrees 登记——跨工程历史状态泄漏候选，已补入 118 卡。

## 派生 findings 卡

- 池序 116 FIX-TURNSTOP-WEDGE-1（P1）：user_stop 半落地不收敛 + 二次停止救不回
- 池序 117 FIX-ASK-SILENT-GAP-1（P1）：ask→slice 122s 静默空窗，LLM 前卡点未定位（二轮样本在卷）
- 池序 118 FE-START-EMPTY-DRAFT-1（P2）：启动页双空 draft + 误入空工程 +（附带）旧 42MB 状态启动加载——**双 draft 已二次复现 + 跨工程历史泄漏新线索**

## 第三轮（2026-10-15 21:00–21:04，二进制=20:15 重建含 117/119 修复+webui 重建；用户现场未关）

| 检查点 | 结果 |
|---|---|
| 发送后反馈 | ✅ item.started 即刻（21:01:19，117 修复现场生效） |
| shadow 轨道计数 | ✅ 6/6（今早失同步**未复现**） |
| 观察链 | ✅ catalog→view 请求×2 完成（mix.multitrack_relationship 带 MOM 投影、track.stereo_space 带 balance 5.945dB 实值） |
| 轨迹链 | ✅ 假设框到 1017 guitar、round 启动、observation.recorded |
| 编排器入口 | ❌ `semantic progressive disclosure observation audit is not successful: partial` → stop_reason=semantic_treatment_capability_boundary 诚实收口（task closed，无楔死，goal 干净 completed） |
| 双 draft | ⚠️ 第三次复现（21:00:48/51）但 shadow 未失同步——两缺陷解耦 |
| 终态 | task=settled、goal=completed、4 slice 全收口——**现场干净可关可续** |

两个 partial 源：obs#1 limitations=["project.structure: evidence status is partial"]；obs#2 omission_reasons=["track.time_dynamics: source evidence is missing"]。派生卡=池序 120 M2-EXP-TIMEDYN-GAP-1。工件：events_round3_2100.json / runtime_status_round3_2100.json。

## 第四轮（2026-10-16 18:16–18:18，二进制=18:15 重建含 120 修复①a；用户现场未关，决策侧入场取证）

| 检查点 | 结果 |
|---|---|
| 话术 | 「帮我在vocals轨道加载一个混响并调整一下」（18:17:24 ask 落账） |
| 链路 | ✅ classification→观察→list_tracks→get_project_state→插件搜索→装载→settle 一气呵成，18:18:38 done（ask→done 约 74s，2 turn） |
| shadow 轨道计数 | ✅ 6/6（engine_track_count=6，last_delta_seq=471） |
| 编排器入口/partial 死签 | ✅ **零复现——120 修复在用户真实工程+Godot GUI 路径成立**（goal completed、stop_reason=done、task settled，事件面无 partial 审计拒收签名） |
| 插件装载 | ✅ Abbey Road Plates Stereo（VST3/Waves WaveShell 壳，id=1040）装载 vocals(1032) 机架 Z3，processing=true、零 load_error |
| typed 参数写入 | ❌ `plugin_grabber.inspect_reverb` 拒绝：`unresolved_reverb_surface`（seq14）——decay 可证核心只认 decay/size/length 词族（reverb_topology.go:169-174），板式 Damper 不在词表→hasWet+hasCharacter 无 tail 核心即拒（:378-383）；agent 诚实不绕道（**本轮零参数写入**），开插件编辑器+给起点值；explain_controls 读到通用参数外壳（Damper/BassCut/Predelay/Wet/Dry 等 normalized_role=other） |
| track.list | ⚠️ 返回 0 轨（seq7 body 自述），agent 以 get_project_state 自愈——列表工具面与 shadow/权威态不一致新样本（118 归因腿域） |
| 双 draft | ⚠️ **第四次复现**（18:16:33/34，draft_20261016T101633+T101634，1 秒间隔；shadow 正常） |
| l3_feature_log | 18 条=6 轨×3 族（band/stereo/loudness），**无 time_energy**——与 ①b（122 卡 KERNEL-L3-TIMEENERGY-1）未实现一致，旅程靠 ①a 波形绑定走通 |

结论：**M2 旅程在用户真实工程+GUI 路径首次完整走通**（120 修复真机成立）；「成功加载」属实、「调整」实为 agent 开编辑器交用户手调——受控写入被板式混响词表缺位诚实拒绝。新发现派生卡=池序 123 PLUGGRABBER-TIMEDYN-COMPAT-1（当晚经用户范围升级指令自 PLUGGRABBER-REVERB-PLATE-1 扩为 Waves+FabFilter 时域批量兼容 pass）+ 池序 124 MIX-SENDRETURN-DESIGN-1（用户 bus 发送质询派生）。工件：events_round4_1817.json / agent_state_round4_1820.json / runtime_status_round4_1820.json（只读取证：/agent/state、/agent/runtime/status、/agent/events?conversation_id=webui_mvatcjnw；进程 pid VitAgent=20056@18:16:33/VitApp=17000@18:16:31）。
