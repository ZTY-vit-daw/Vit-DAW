# FIX-TRACKLIST-SHADOW-FRESHNESS-1（143）回执：track.list 工具面 shadow 旧快照修复

- owner：GLM-5.3 L2 PC ZCode 执行会话（本卡会话，用户口令指定领取）；worktree `D:/Vit_DAW_wt_trackfix1`，分支 `port/fix-tracklist-shadow-freshness-1`（领取基线 origin/main=d7079a64a → 领取提交 e4d113f3 → 实现提交 **372ab2c9**）
- 方案：**A+B 组合**（卡面推荐项）

## 1. 实现（commit 372ab2c9，文件域=卡面授权域，零越域）

### A 面：工具面内核回包权威（`agent/internal/harness/harness.go`）

- `publicResult` 的 `list_tracks` 分支改为 `publicListTracksResult(h, reply)`：**行与计数取本命令内核回包**（VSP canonical `project.tracks.list` 的 legacy_reply 或 legacy 直查回包，经 `shadow.CompactUserTrackRows` 用与 Summary 相同的 isUserTrack 规则过滤渲染，行形=既有紧凑行形，含 user_track_index）；shadow 仅做不一致比对。
- 不一致（user-track ID 集合不等）时显式披露：`shadow_stale=true` + `warning` + 双计数 `kernel_user_track_count`/`shadow_user_track_count`；一致时不打 stale 标。
- 回包缺 `tracks` 载荷的异常形态：回落 shadow 投影但必标 `tracks_source=shadow_snapshot`+warning，不得冒充权威。
- `get_project_state` 分支：行为保持（内核权威按构造成立——`afterKernelReply`/`afterKernelReplyVSP` 在 publicResult 前已用本命令回包 `shadow.Initialize`，投影即本回包投影）；分支内补契约注释；回归测试覆盖「kernel=6/shadow 空 → 返回 6 且 shadow 被回包初始化」。

### B 面：shadow 结构 delta 去抖权威刷新（`shadow.go` + `bridge.go`）

- `shadow.applyDeltaLocked`：结构 delta（`node_added` 且 uid 不在 bootstrap / `node_removed` 且 uid 为已知轨）置一次性 `structureRefreshPending` 提示；**orphan 记账语义原样保留**（结构 delta 仍进 orphanDeltas，`TestShadowTracksOrphanDeltas` 不动并新增断言）。属性 delta（`property_changed:*`）不触发。
- `shadow.ConsumeStructureRefreshHint()`：原子读清。
- bridge：`normalizeTelemetry` 在 `ApplyDelta` 后消费提示→投递 `structural_delta` 刷新信号（容量 1 天然合并）；`runShadowRefresh` 对结构信号**去抖 1s**（突发 node_added 合并为一次刷新，刷新后仍 pending 的提示再武装一轮兜竞态），`recording_stopped` 保持即时；`refreshShadow` 升级为 **VSP state snapshot 优先**（共享 kernel.Client 的 VSP 通道），legacy `get_project_state` 回退。非轮询：零结构变更零刷新。
- 内核侧事实（只读取证）：结构变更广播 `node_added`/`node_removed`（`VitValueTreeDeltaListener`，VitDeltaProbe.cpp:110-128），属性变更 `property_changed:<prop>`；`new_project` 后 delta hub 重挂新 edit 根（VitHeadlessService.cpp:1160）。

## 2. 单元/回归测试（`cd agent && go build ./... && go test ./... -count=1` → **exit 0，95 包 ok，0 FAIL**；gofmt 净——CRLF 归一化后 6 个触碰文件 diff=0）

新增（验收①）：
- `harness_test.go` `TestPublicListTracksKernelAuthoritativeDisclosesStaleShadow`：S1 正向 kernel=6/shadow=0 → 6+stale+双计数；S2 反向 shadow=6/kernel=2 → 2+stale+双计数；一致不打标。
- `harness_test.go` `TestInvokeListTracksVSPKernelReplyAuthoritative`（VSP 执行路径，S1 场景，断言走 canonical `project.tracks.list`）+ `TestInvokeListTracksLegacyKernelReplyAuthoritative`（legacy 路径反向）+ `TestInvokeGetProjectStateInitializesShadowFromKernelReply`（project.state 回包初始化后返回 6）。
- `shadow_test.go` `TestStructureDeltaRaisesRefreshHint`（未知 uid node_added/已知轨 node_removed 触发、一次性消费、orphan 语义保留）+ `TestCompactUserTrackRowsFiltersKernelRows`（内核行过滤+值原样透传）。
- `bridge_test.go` `TestNormalizeTelemetryStructuralDeltaSignalsRefresh`（结构信号投递/属性不投递/已知 uid 回放不投递）+ `TestRunShadowRefreshDebouncesStructuralBurst`（突发合并 1 次+recording 即时+竞态再武装）。

既有口径：`TestPublicResultSanitizesProjectState` 保持通过（get_project_state 语义不变）；`TestShadowTracksOrphanDeltas` 原样保留通过；bridge 既有 telemetry 测试全部兼容（nil-safety）。139 探针口径未弱化（见 §3）。

## 3. 真栈验收（AGENTS §5；占用登记 a1593143 → 释放 c074be36，先登先用，无栈冲突）

- **run=run_20261018_192615，脚本 exit 0，probe PASS**（§8 LLM=0）。被测：agent SHA256=`C57EB91797BF27DC…`（本分支构建）/vsphub `D35519BCB0A3DBD8…`/kernel=stock `B6565DCF85D1DA86…`（只读）；worktree HEAD=372ab2c9。栈=kernel pid15776+vsphub 16240+agent 21196（不启 UI）；起栈前/拆栈后六端口与四类栈进程均=0（teardown.json exit_code=0/remaining_listeners=0/remaining_processes=0）。
- 探针=139 berth+六面口径原样复刻（`tracklist_fix_probe.py`，同 F1-F6 面、同 user-track 投影、同 raw 存档），仅阶段期望改为修复契约。
- 六面结果（user-track 口径）：

| 阶段 | kernel | hub_cmd | hub_state | shadow | tool | tool≡kernel | 备注 |
|---|---|---|---|---|---|---|---|
| S0 基线 | 0 | 0 | 0 | 0 | 0 | ✓ | |
| S1 r1 外部新建+导入 | 6 | 6 | 6 | 6 | 6 | ✓ | B 面在捕获前已 heal（捕获序 kernel→hub→state→shadow 耗时>去抖窗） |
| S1 r2 外部新建+导入 | 6 | 6 | 6 | 6 | 6 | ✓ | 同上；heal poll 确认 |
| S1_refresh（project.state） | 6 | 6 | 6 | 6 | 6 | ✓ | 自愈面保持 |
| **S2 外部打开 2 轨** | **2** | 2 | 2 | **6** | **2** | ✓ | **反向陈旧面：tool=kernel 2 行 + shadow_stale=true + 双计数 kernel=2/shadow=6；零静默** |
| S3 agent 路径 add_track | 3 | 3 | 3 | 3 | 3 | ✓ | |

- 断言面全过：sanity（kernel/hub/hub_state 三面 ID+uuid 全阶段一致）；A 面 tool≡kernel 全 6 阶段、silent-stale 空、未披露非权威空；stale 披露面（tool 自带双计数无竞态判据）全过；B 面 S1 两轮 shadow heal=True；自愈面 ✓；S3 面 ✓；同帧链（track.list 回包均带内核 VSP command_ack）✓。
- **B 面直接证据**：agent_last.log 三条 `[shadow] refreshed after structural_delta source=vsp.state.snapshot revision=2/3/4`（S1 r1/r2/S2 各一次，VSP 优先刷新落地）；内核会话 VSP envelope 128 行（139 同栈 116，差额=B 面刷新的 state.snapshot 请求）。
- S2 的 B 面 heal 亦发生（0.26s，观察项非门）——超出预测（fixture 预填充预期无结构 delta），机制为 load 后 ensureMonitoring/rack 生成节点的 node_added，如实记录。

## 4. 消费面影响清单（只读核查，验收④，零改动）

- `agentloop/execution_memory.go:229,373-376`（track.list/project.state 结果→执行记忆/TOM 重建）：读 `result["tracks"]` 行的 track_id/track_name（clips 可缺省有降级路径）——内核行兼容；**S1 类 0 轨误绑（空工程误判）随修复消除**。
- `agentloop/message_loop.go:7836`（`messageLoopListCount(projectState["tracks"])==0` 空工程分支）：计数现反映内核真值，陈旧 shadow 不再把非空工程误判为空；2681/3563-3684/9253 处走 project.state 结果或通用行读取，兼容。
- `chat/reply_test.go:19`：list_tracks 风险声明，无内容语义，全量测试通过。
- 其余 `user_track_count` 读者（project_blackboard_report / audio_closure_controller:1561 / chat server / contextruntime）：均读 shadow Summary 或 project.state 面，语义未变。
- 工具回包新增字段：`shadow_stale`（仅不一致时）、`kernel_user_track_count`/`shadow_user_track_count`、异常形态 `tracks_source`——纯增量，无既有字段移除。

## 5. 边界与偏差

- S1 即时捕获未观测到「披露中」瞬态（B 面 heal 快于捕获序到达 shadow 读点）；披露路径由 S2 真栈面+单测正反向双覆盖。
- bridge 新增 `shadowRefresh` 注入缝（kernel.Client 具体类型无法伪造 VSP 回包，测试注调用计数）；生产路径恒等 `(*Bridge).refreshShadow`。
- `node_added` 触发含未知 uid 的 clip/plugin 节点（GUI 编辑期结构事件）——受 1s 去抖+容量 1 合并约束，事件驱动非轮询，零结构变更零刷新。
- 主工作树既有未提交改动（Settings.xml/default_project.xml/烟测脚本）未触碰（AGENTS §12）。

## 6. 工件

- 本目录：`run_tracklist_fix_probe.ps1`、`tracklist_fix_probe.py`、`run_20261018_192615/`（probe_summary.json/probe_console.txt/raw/*.json/kernel_workspace 日志/agent_last.log/vsphub_last.log/kernel_vsp_evidence.json/teardown.json/run_meta.json）
- 实现+测试：分支 `port/fix-tracklist-shadow-freshness-1` commit 372ab2c9（8 文件，+1779/−33）
