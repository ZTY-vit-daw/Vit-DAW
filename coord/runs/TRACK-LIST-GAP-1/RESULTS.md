# TRACK-LIST-GAP-1（139）取证报告：track.list 0 轨同帧对照

- owner：DSH PC 执行流 exec-rendergain；worktree `D:/Vit_DAW_wt_tracklist1`，分支 `port/track-list-gap-1`（基线 origin/main=77a0b02f）
- 边界：**零生产代码改动**；诊断脚本/工件只入本目录；不启 Godot UI；不写主工作树；不 push main
- 状态：**准备完成，待 Lead 栈窗口 GO**（Lead 裁定 GO 暂缓至用户 C worktree mixer 复测拆栈后）；本文件先记录静态考古证据链，栈腿后补同帧实测。

## 0. 现象（118 归因 §5 原始证据复核）

round4（2026-10-16 18:17–18:18，用户真实工程+GUI 路径，VitApp pid 17000@18:16:31 / VitAgent pid 20056@18:16:33）events seq5-8：
- seq5-6 `track.list` 工具回包：`command_name=list_tracks`、`status=ok`、`command_ack={command:"project.tracks.list", legacy_command:"list_tracks", ack.message:"Track list fetched", stage:"completed", transaction_id:"tx_1792145856379896400_7"}`、`track_count=0`、`user_track_count=0`、`tracks=[]`。
- seq7 agent 自述「track.list 返回 0 条轨道」；seq8 `project.state` 工具回包：`track_count=6`、`tracks=6`（bass/drums/guitar/other/piano/vocals），`vsp.project_observe={track_count:6, project_uuid:"vitproj_6f495cdeecf342ba89df47415a506dc7", revision:2, source:"vsp.state.snapshot"}`。
- **关键**：seq6 回包里内核 VSP **command_ack 已在场**（内核确实被查询并返回了 "Track list fetched"），说明「VSP 命令没到内核 / 打到另一个内核」不是该回包的表面解释；0 轨来自回包 body 的组装侧。

## 1. 静态考古（已证，代码锚点）

### 1.1 注入面 A：工具返回值 = shadow Summary，忽略内核回包（已证）

- `agent/internal/harness/harness.go:12193-12202`：`publicResult` 的 `case "list_tracks":` 在 `h.shadow != nil` 时**直接**返回
  `{status:"ok", track_count: summary["track_count"], user_track_count: summary["user_track_count"], tracks: summary["tracks"]}`——
  **完全不读本命令刚从内核拿到的 `reply`**。
- `agent/internal/harness/harness.go:12179-12182`：`get_project_state` 同样 shadow-only（`userVisibleState(h.shadow.Summary())`）。
- 该 shadow-only 行为有既有测试背书（=设计意图，不是笔误）：`harness_test.go:4236 TestPublicResultSanitizesProjectState` 显式断言 get_project_state 的 publicResult 用 shadow 过滤（只留 user track 1007）。
- 引入时间：`git blame` 该 `list_tracks` 分支 = `15ccd479a`（2026-05-19）。

### 1.2 注入面 B：shadow 的 track 集合只来自权威快照，不来自 delta（已证）

- `agent/internal/shadow/shadow.go:164-209 Summary()`：`tracks := asArray(engine["tracks"])`（engine=state["engine_snapshot"]）、`track_count=len(userTracks)`、`tracks=userTracks`；`compactUserTracks`（`:419-431`）只遍历 `engine_snapshot.tracks`，delta 仅用于**修饰既有行**（`trackRowWithDeltaProperties :433-447`）。
- `shadow.go:227-306 applyDeltaLocked()`：delta 写入 `nodes_by_uid/delta_properties`、`recordChangeLocked`、未知 uid → `orphanDeltas`（`orphan_uid_keys` 计数）——**不会把新轨道并入 engine_snapshot.tracks**。`shadow_test.go:35 TestShadowTracksOrphanDeltas` 断言未知 uid delta 只成 orphan（=快照权威语义的设计意图）。
- 结论：**外部（GUI/内核直连）新增轨道若只以 delta 到达 shadow，Summary 永远保持旧轨道集合**，直到下一次权威快照刷新。

### 1.3 注入面 C：外部变更后缺少权威快照刷新触发（已证）

- bridge 的权威刷新点有限：`bridge.go:466-491 refreshShadow()` 仅在 `refreshCh`（`recording_stopped` 主题，`:417-422`）触发；`:493-501 initializeShadowFromReply` 为启动/一次性路径；`bridge.go:400-416 normalizeTelemetry` 对 `delta_update` 只 `shadow.ApplyDelta`。
- harness 的权威刷新点：`refreshShadow`（`harness.go:13090-13165`，VSP state snapshot 优先）由**显式调用**触发（state summary 未初始化时 `:492-495`、写类命令后 `spec.RefreshAfter`、get_project_state 的 VSP 特判 `:1640-1657` + `:1672-1673` 等）；continuation scheduler 仅在 project_uuid 为空时做一次发现式刷新（`continuation_scheduler.go:798-806`）。
- 没有任何「外部轨道变更 delta → 触发一次权威快照」的路径。

### 1.4 完整机理链（静态可证）

1. 外部路径（GUI/内核直连）新建工程+导入音轨 → 内核 edit 有 6 轨，广播 delta；
2. agent bridge 收 delta → `shadow.ApplyDelta` → 新轨 uid 进 `nodes_by_uid`/`orphanDeltas`，**engine_snapshot.tracks 仍 0**；
3. `track.list` 工具 → 先经 VSP canonical `project.tracks.list` 查内核（回包正确、command_ack 在场）→ 但 `publicResult` 返回 **shadow Summary=0**；
4. 随后 `project.state` 工具 → VSP state snapshot → `shadow.Initialize(6 轨)` → 下一轮 `track.list`=6（自愈，与 118 round4 观测一致）。

## 2. 候选面裁决计划（栈腿同帧探针，见 RUNBOOK）

- **候选①（VSP 通道命中内核/编辑实例与 GUI 当前 edit 不同）**：每阶段比对 kernel 直查 `list_tracks`/`get_project_state` 与 hub canonical `project.tracks.list`（payload.legacy_reply.tracks）+ hub state snapshot（tracks/project_uuid/revision）的 track_id 列表与 project_uuid；hub `/vsp/status` 记录 kernel_req 端点/pid；内核会话日志统计 VSP envelope 行数。→ 若全部一致且 uuid 相同 = **排除**（与 §0 command_ack 证据同向）。
- **候选②（新建+导入路径轨道枚举差）**：S1「外部新建+导入」vs S2「外部打开既有工程」同签名则**排除路径特异性**；两者差别只应在于 shadow 是否被刷新（外部 vs agent 路径）。
- **根因面**：任一同帧点上 `agent 工具 tracks == shadow.tracks` 且 `≠ kernel.tracks` → 坐实工具面=shadow 投影；`project.state` 后 tool==kernel → 坐实自愈路径。
- 预期（待栈腿数据）：S1 外部新建+导入 → kernel=6/hub=6/state=6/shadow=0/tool=0；S1b → tool=6；S2 → tool≠kernel 同签名（路径特异性排除）。

## 3. 修复建议（另卡，本卡只取证不改代码）

- **首选（用户面正确性）**：`harness.publicResult` 的 `list_tracks` 以**本命令内核回包**为准（shadow 仅作 enrichment/命名/分组；若 shadow 陈旧则在结果中显式 `shadow_stale:true` + 双计数，禁止静默返回旧快照）。文件域：`agent/internal/harness/harness.go`（publicResult list_tracks 分支）+ `harness_test.go` 对应测试；同样口径复核 `get_project_state`（至少披露 shadow 快照 revision 与内核 revision 差异）。
- **加固（时效性）**：shadow 收到「未知 uid 的新轨道 delta」时触发一次**去抖**权威快照刷新（VSP state snapshot 优先，参考 `harness.refreshShadow`），而不是把新轨静默留在 orphanDeltas。文件域：`agent/internal/shadow/shadow.go`（orphan 触发点）+ `agent/internal/bridge/bridge.go`（delta 处理处，`normalizeTelemetry`/`runShadowRefresh`）+ 单测（保留 `TestShadowTracksOrphanDeltas` 语义：orphan 仍然不直接并入快照，但触发刷新）。
- **测试防线**：新增 tools/harness 层用例「内核回包 6 轨 + shadow 空 → list_tracks 结果应为 6（或显式 stale 标注）」，防止回归到 shadow-only 静默。
- **影响面边界（静态评估，供修复卡定范围）**：
  - 消费面：`agentloop/execution_memory.go:229,373-376`（把 track.list/project.state 的 tracks 写入执行记忆）、`agentloop/message_loop.go:2681,3563-3684,7836,9253`（mix candidate/prompt 组装；注意 `:7836 if messageLoopListCount(projectState["tracks"]) == 0` 是另一处「0 轨即空工程」分支，陈旧 shadow 会同样误判）。
  - 待改测试口径：`harness_test.go:4236 TestPublicResultSanitizesProjectState`（get_project_state shadow-only 断言）、`shadow_test.go:35 TestShadowTracksOrphanDeltas`（orphan 语义需保留但新增触发刷新断言）、`chat/reply_test.go:19`（list_tracks risk 声明，无内容语义）。
- **建议修复卡（草案，交 Lead 决定是否立）**：`FIX-TRACKLIST-SHADOW-FRESHNESS-1`（P2/P1 视裁定）：A 工具面回包以 kernel reply 为准 + `shadow_stale` 披露；B 未知新轨 delta 去抖触发权威刷新；C 回归用例（S1 场景：外部新建+导入 6 轨、shadow 空 → tool 仍应返回 6 或显式 stale）；验收=`cd agent && go test ./internal/harness ./internal/shadow ./internal/bridge ./internal/agentloop ./internal/chat -count=1`（0 FAIL）+ 真栈 S1/S2 复跑；文件域=`agent/internal/harness/harness.go`、`agent/internal/shadow/shadow.go`、`agent/internal/bridge/bridge.go`(+对应 *_test.go)。

## 4. 栈腿同帧实测（待 GO 后补）

- run ID / 脚本 exit / 六面逐数值与 track_id 列表 / 候选①②裁决 / 最小复现次数（≥2 次确定性）/ 栈证据与拆栈复核 → 见后续追加节。

### 4.1 干跑记录（仅 harness 干跑，**不作验收证据**）

- `run_20261018_145825`：14:58:25–14:59:45 一次**未获 GO** 的 berth 启动（harness `-PreflightOnly` 开关替换 bug 导致意外执行；已自报 Lead，按要求仅作工程参考）。该 run 暴露并促成两项 harness 修复（`-PreflightOnly` 真生效 + user-track 投影比较口径 + 两轮确定性 S1），原始工件 slim 入库但不用于卡面结论；正式证据待 GO 后新 run。

## 5. 工件清单

- `RUNBOOK.md`（berth 命令/矩阵/§8 声明）、`run_tracklist_gap_probe.ps1`（kernel+vsphub+agent berth）、`tracklist_gap_probe.py`（同帧六面探针）、`_inspect_round4_events.py`（round4 events 复核脚本）
- 运行工件（GO 后）：`run_<ts>/probe_summary.json`、`probe_console.txt`、`raw/*.json`、`kernel_workspace/Workspace/Logs/`、`agent_last.log`、`vsphub_last.log`、`kernel_vsp_evidence.json`、`teardown.json`、`run_meta.json`
