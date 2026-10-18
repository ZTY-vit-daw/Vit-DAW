# FIX-TRACKLIST-SHADOW-FRESHNESS-1：轨道列表工具面返回 shadow 旧快照（stale）——内核回包权威 + shadow 时效性修复

- 发卡：12:00-18:00 决策流 / 2026-10-18 晚窗（依据=TRACK-LIST-GAP-1(139) 正式 run `run_20261018_175229` 同帧六面证据 + 118 round4 seq5-8 同签名）
- 派发确认：**已确认（用户 2026-10-18 晚窗）**（P1；工具面数据正确性，影响 agent 判断与用户 M2 体验）
- 验收负责人：GLM 主管决策流（或当班决策流）
- 池序 143；目标仓库=D:\Vit_DAW；P1 候选 / 预估 0.5-1 天 / 前置=无（139 取证已闭合）
- 模型分级：L2 / GLM 执行会话（agent harness/shadow/bridge）

## 根因（139 已取证，勿重复）

- 工具面 `harness.publicResult` 的 `list_tracks` 分支**直接返回 shadow Summary、忽略本命令内核回包**（`agent/internal/harness/harness.go:12193-12202`；`get_project_state` 同口径 :12179-12182）；既有测试 `TestPublicResultSanitizesProjectState` 背书 shadow-only。
- shadow 轨道集合只来自权威快照（`shadow.go:164-209 Summary()`）；`applyDeltaLocked:227-306` 对新 uid 只进 `nodes_by_uid`/`orphanDeltas`（`shadow_test.go:35` 断言 orphan 语义）；bridge 权威刷新只在 `recording_stopped`（`bridge.go:466-491`），**外部新轨 delta 无权威快照刷新触发**。
- 139 同帧六面：S1 外部新建+导入 kernel/hub_cmd/hub_state=6 而 shadow/tool=0（tool≡shadow，每回包含内核 `project.tracks.list` command_ack）；S1_refresh（project.state→VSP state snapshot）后自愈；S2 反向 stale（kernel=2/shadow=6）；S3 agent 路径写同步（经 agent 工具路径触发刷新）。

## 目标（三选一由执行侧取证后给最小方案，Lead 裁定）

1. **A（首选）**：工具面以本命令内核回包为准（shadow 仅 enrichment）；若 shadow 与内核不一致，显式披露 `shadow_stale` + 双计数（内核 user tracks / shadow tracks），不得静默返回旧快照。
2. **B（加固）**：shadow 收到未知 uid 新轨 delta 时去抖触发权威快照刷新（VSP state snapshot 优先；保留 orphan 语义；不得全量轮询）。
3. A+B 组合（推荐若成本可控）。

## 文件域

`agent/internal/harness/harness.go`、`agent/internal/shadow/shadow.go`、`agent/internal/bridge/bridge.go`（必要时）+同包测试；消费面只读核查（`agentloop/execution_memory.go:229,373-376`、`agentloop/message_loop.go:7836` 等）。越域先上报；消费面若需修另卡。

## 验收标准

1. 回归用例（新增）：内核回包 user=6 + shadow 空 → `track.list`/`project.state` 返回 6 或显式 `shadow_stale`（不得返回 0 静默）；反向（shadow=6/kernel=2）同样显式；S1 外部新建+导入、S3 agent 路径两场景。
2. 既有测试口径更新（`TestPublicResultSanitizesProjectState` 等）与新契约一致；`cd agent && go build ./... && go test ./... -count=1` 0 FAIL + gofmt 净。
3. 真栈（AGENTS §5，PC-RUNTIME-STACK 登记后）：复用 139 harness 复刻 S1×2/S2/S3 六面断言——tool≡kernel 或显式 stale、无静默旧快照；run ID/工件/被测 SHA 入回执。
4. 消费面影响清单（0 轨误判空工程路径）核对，必要时另卡。

## 停止条件

- 需改内核/VSP 协议或跨层架构 → 先上报；不得用 WARN/静默降级掩盖 stale；不得弱化 139 探针口径；不得删 orphan 语义测试。

## 领取

- （2026-10-18 晚窗 / origin/main=d7079a64a715cadd0d64cb90f5320d18c4bc967d / GLM-5.3 L2 PC 执行会话 ZCode（用户口令指定领取，本会话）/ 分支=port/fix-tracklist-shadow-freshness-1（worktree D:/Vit_DAW_wt_trackfix1）/ 领取提交=本提交）
- 领取时核对：卡在 todo 派发已确认（用户 2026-10-18 晚窗，commit d27f6dbc）；依赖 139 已闭合；doing 三卡（FE-GUI-START-FEED/GEN-SUNO/MIXER-LIVE-REFRESH）文件域均为前端仓/Godot/coord-runs，与本卡 agent/internal/{harness,shadow,bridge} 零冲突；真栈亲核 2026-10-18 晚窗：六端口（5555/5556/5557/7878/8787/4445）全 free、四类进程（VitApp/VitAgent/vsphub/Godot 含变体）=0，PC-RUNTIME-STACK 当前占用=空闲。
- 方案裁定：A+B 组合（按卡面推荐，139 §3 草案）。

## 回执

- （2026-10-18 晚窗；commit=**372ab2c9**（分支 port/fix-tracklist-shadow-freshness-1，8 文件 +1779/−33）；报告=[RESULTS.md](../../runs/FIX-TRACKLIST-SHADOW-FRESHNESS-1/RESULTS.md)）
- 方案=A+B 组合：A=`publicListTracksResult` 内核回包权威+`shadow_stale` 双计数披露（get_project_state 按构造已权威，补契约注释+回归）；B=shadow 结构 delta（未知 uid `node_added`/已知轨 `node_removed`）一次性刷新提示（orphan 语义原样保留）+bridge 去抖 1s、VSP state snapshot 优先权威刷新、事件驱动非轮询。
- 验收①②：新增回归 8 例（S1 正反向 publicResult/VSP/legacy、project.state 回包初始化、结构提示/orphan 保留、内核行过滤、bridge 信号/去抖/竞态再武装）；既有 `TestPublicResultSanitizesProjectState`/`TestShadowTracksOrphanDeltas` 原样通过；`go build ./... && go test ./... -count=1` **exit 0（95 包 ok，0 FAIL）**+gofmt 净（CRLF 归一后 diff=0）。
- 验收③真栈：占用登记 a1593143→释放 c074be36；**run=run_20261018_192615 脚本 exit 0 probe PASS**（§8 LLM=0）；被测 agent SHA256=C57EB91797BF27DC…/vsphub D35519BCB0A3DBD8…/kernel stock B6565DCF…（只读），HEAD=372ab2c9；六面 S0/S1×2/S1_refresh/S2/S3 **全阶段 tool≡kernel、零静默旧快照**；S2 反向面 tool=2（kernel 行）+shadow_stale=true+双计数 2/6；B 面直接证据=agent 日志 3 条 `refreshed after structural_delta source=vsp.state.snapshot`（revision=2/3/4）+S1 两轮 heal 确认；拆栈复核六端口/四类进程=0。139 探针六面口径原样复刻未弱化。
- 验收④消费面（只读核查零改动，详见报告 §4）：execution_memory/TOM 重建、message_loop:7836 空工程分支——内核行兼容且 0 轨误判消除；新增字段纯增量。无需另卡。
- 边界声明：S1 即时捕获未观测到披露瞬态（B heal 快于捕获序，披露路径由 S2 真栈+单测双覆盖）；bridge 加 `shadowRefresh` 测试注入缝（生产路径恒等 refreshShadow）；`node_added` 含 clip/plugin 节点受 1s 去抖约束。

## 关联

- 取证卡：TRACK-LIST-GAP-1（139，done）；修复锚点/草案见 `coord/runs/TRACK-LIST-GAP-1/RESULTS.md` §3
- 发现来源：FE-START-EMPTY-DRAFT-1（118）round4 seq5-8 用户 M2 体验样本

## 验收（2026-10-18 晚窗 GLM 主管决策流）：pass

- diff 亲读：A 面 publicListTracksResult 内核回包权威+异常回包诚实降级（tracks_source=shadow_snapshot+warning 不冒充权威）+userTrackIDSetsDiffer 集合比对正确；B 面 structureRefreshPending 触发条件=未知 uid node_added/已知轨 node_removed（orphan 记账原样）+bridge 去抖 1s 再武装（防去抖窗口吞改）+VSP 快照优先 legacy 回退+shadowRefresh 测试注入缝（生产恒等）。语义变更与停止条件零越界。
- 决策侧独立复跑：worktree trackfix1 `go build ./...` exit 0 + `go test ./... -count=1` **95 包 ok / 0 FAIL**。
- 真栈工件亲核：run_20261018_192615 probe_summary **pass=True**——六面全阶段 tool≡kernel（a_face ok/silent_stale=∅/undisclosed=∅）、stale_disclosure violations=∅、B 面 S1 两轮 heal 0.01-0.02s/S2 0.26s、agent 日志 3 条 `refreshed after structural_delta source=vsp.state.snapshot`（rev 2/3/4）实证；run_meta 被测 SHA 齐（worktree agent C57EB917/vsphub D35519BC+stock kernel 只读）。
- 消费面清单（execution_memory/TOM/message_loop 空工程分支）核查采信，无需另卡。
- impl 372ab2c9 已 cherry-pick 入 main=**88e88327**；卡终态=done。
