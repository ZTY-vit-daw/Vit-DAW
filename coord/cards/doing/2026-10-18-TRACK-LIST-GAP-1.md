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
