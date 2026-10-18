# TRACK-LIST-GAP-1：track.list 0 轨取证（新建+导入路径；同帧对照探针）

- 发卡：12:00-18:00 决策流 / 2026-10-18 午窗（依据=FE-START-EMPTY-DRAFT-1(118) [ATTRIBUTION.md](../../runs/FE-START-EMPTY-DRAFT-1/ATTRIBUTION.md) §2.5，round4 M2 体验样本）
- 派发确认：**待用户确认**（P2 取证；需内核栈窗口）
- 验收负责人：GLM 主管决策流
- 池序 139；目标仓库=D:\Vit_DAW；P2 / 取证 0.5 天 / 无依赖
- 模型分级：L2 / GLM 执行会话（内核/传输取证）

## 现象（round4 events seq5-6 实证）

- `list_tracks` 工具回包 `command_name=list_tracks/status=ok/ack="Track list fetched"/tracks=[]/track_count=0`；同窗口 seq8 `get_project_state`=6 轨；shadow/engine 均 6 轨（agent 以 get_project_state 自愈）。列表工具面失真。
- 映射链：agent `list_tracks`→VSP canonical `project.tracks.list`（harness.go:1771-1772）；内核 `VspKernelReference.cpp:87/2551-2572` 只转 legacy `list_tracks`；`CommandDispatcher.cpp:2670-2674`→`TrackService.cpp:444-466` `getEdit()`+`te::getAllTracks`。ack 成功排除「空 edit」（会报 No active edit loaded）。

## 候选面（同帧对照探针逐一排除/坐实）

1. VSP 通道命中的内核/编辑实例与 GUI 当前 edit 不同（hub 陈旧 kernel 连接/另一内核实例）；
2. 「新建+导入」路径的轨道枚举差（与「打开既有工程」路径对比）。

## 目标

栈内**同帧对照探针**（同一时刻 `list_tracks` vs `get_project_state` vs kernel 直查 `te::getAllTracks` vs shadow），定位 0 轨来源并提供最小复现（新建工程+导入音轨后 list_tracks）。零生产代码改动。

## 文件域

`coord/runs/TRACK-LIST-GAP-1/` 诊断脚本+工件。不改产品代码/既有冒测脚本（若需扩展现有脚本先上报）。越域即停。

## 验收标准

- 最小复现（新建+导入后 list_tracks=0 轨）在真栈确定性呈现（至少 2 次）或证据化说明前提；
- 同帧证据链（四面对照时间戳/实例标识）坐实/排除三候选面；
- 修复建议与文件域锚点上交决策侧（另立修复卡）。
- 栈纪律：PC-RUNTIME-STACK 登记后启栈、不启 Godot UI（如同帧需要 UI 则上报）、拆净复核=0。

## 停止条件

- 需内核/传输层改造 → 只取证上交另卡；不得伪造结论或改产品代码。

## 关联

- 发现卡：FE-START-EMPTY-DRAFT-1（118，done；§2.5 待证）
- 与 118 双 draft/shadow 0 轨为不同代码路径，不合并归因。

- 领取：2026-10-18 14:55 +0800 / origin/main=3abe0cfb / owner=DSH PC 执行流 exec-rendergain（模型+机器+会话）/ 分支=port/track-list-gap-1（自 3abe0cfb）/ worktree=D:/Vit_DAW_wt_tracklist1（新建）/ 领取提交=待建；task-13；用户 2026-10-18 确认派发；零生产代码

- 回执（2026-10-18 晚窗，exec-rendergain 执行，Lead 转写）：分支 **port/track-list-gap-1@6bce31fe**（已推；5 commits：探针 runner+静态链+harness 修复+修复卡草案+正式 run；**零生产代码**，仅 coord/runs/TRACK-LIST-GAP-1/）。**正式 run=run_20261018_175229**（17:52:20 预检六端口/进程=0→起栈 kernel15144+vsphub11892+agent5264→17:52:44 拆栈只杀自起 pid→17:52:49 复核六端口=0/四类进程=0）脚本 exit 0（probe PASS，§8 LLM=0，零 assertion_fail）。**同帧六面（user-track 口径）**：S0 全 0；**S1 round1/round2 外部新建+导入 kernel=6/hub_cmd=6/hub_state=6 而 shadow=0/tool=0（tool≡shadow，tool≠kernel）**；S1_refresh（project.state 后）全 6（自愈）；S2 外部打开既有 2 轨 kernel/hub=2 而 shadow/tool=6；S3 agent 路径写 add_track 全 3。**候选面结论**：①VSP 通道命中不同内核/编辑实例→**排除**（6 阶段 user-track ids 全一致、kernel↔hub_state project_uuid 一致、hub kernel_req=127.0.0.1:5555、116 条 VSP envelope 同证）；②新建+导入路径枚举差→**排除为路径特异**（S1×2 与 S2 同签名，S3 agent 路径反而同步）；**根因坐实**=工具面 list_tracks 返回 shadow Summary（每回包含内核 VSP command_ack 却返回 shadow 行；6/6 阶段 tool≡shadow、3 个 stale 阶段 tool≠kernel）；最小复现 **2/2**；自愈链=project.state→VSP state snapshot→shadow.Initialize→tool=6。被测=stock kernel B6565DCF…/worktree VitAgent C7A01057…/vsphub A1811FA5…；工件 coord/runs/TRACK-LIST-GAP-1/run_20261018_175229/。**修复卡草案**（FIX-TRACKLIST-SHADOW-FRESHNESS-1）：A 工具面以本命令内核回包为准（shadow 仅 enrichment；陈旧显式 shadow_stale+双计数）=harness.go:12193-12202（get_project_state 同 12179-12182）；B shadow 未知新轨 delta 去抖触发权威快照刷新=shadow.go:227-306+bridge.go:400-491（保留 shadow_test.go:35 orphan 语义）；C 回归用例+消费面锚点（execution_memory.go:229,373-376、message_loop.go:7836）。**偏差/边界**：14:58 未登记未 GO 启动事故已自报（run_20261018_145825 仅干跑不作证据；整改 75f3caf2 -PreflightOnly 实测只预检不启栈+比较口径修正）；kernel-only berth+vsphub（VSP canonical 必需，不启 UI）；用户 Godot 全程未碰；主检出只读。
- 验收：**pass（取证卡）**——[rulings/2026-10-18-BATCH8-rulings.md](../../rulings/2026-10-18-BATCH8-rulings.md) §二；决策侧亲核=run_meta/probe_summary/teardown 亲读（exit_code=0/remaining=0/0）+六面数值自洽检查（tool≡shadow、S1 2/2、S1_refresh 自愈、S2 反向 stale）+分支提交范围（零生产代码）+远端 hash 亲核+事故纪律记录在案；候选①②排除与根因结论**采信**；修复卡 **FIX-TRACKLIST-SHADOW-FRESHNESS-1（池序 143，P1 候选，待用户确认派发）** 已立（todo）；cherry-pick 入 main（619f76b9/00c87e8a/a8f53176/84b8ad16/7333bb48）；卡归 done。