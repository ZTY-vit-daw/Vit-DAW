# FIX-TRACKLIST-SHADOW-FRESHNESS-1：轨道列表工具面返回 shadow 旧快照（stale）——内核回包权威 + shadow 时效性修复

- 发卡：12:00-18:00 决策流 / 2026-10-18 晚窗（依据=TRACK-LIST-GAP-1(139) 正式 run `run_20261018_175229` 同帧六面证据 + 118 round4 seq5-8 同签名）
- 派发确认：**待用户确认**（P1 候选；工具面数据正确性，影响 agent 判断与用户 M2 体验）
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

## 关联

- 取证卡：TRACK-LIST-GAP-1（139，done）；修复锚点/草案见 `coord/runs/TRACK-LIST-GAP-1/RESULTS.md` §3
- 发现来源：FE-START-EMPTY-DRAFT-1（118）round4 seq5-8 用户 M2 体验样本
