# FIX-START-DRAFT-SIDE-EFFECT-1：启动读路径不得物化 draft + 跨工程 uuid 泄漏守卫（118 归因拆出）

- 发卡：12:00-18:00 决策流 / 2026-10-18 午窗（依据=FE-START-EMPTY-DRAFT-1(118) [ATTRIBUTION.md](../../runs/FE-START-EMPTY-DRAFT-1/ATTRIBUTION.md) §2.1/§2.4 + 真栈 D/E 轮 reader 实证）
- 派发确认：**待用户确认**（P1；118 最小修复已合入，本卡为其根因面；建议下一可用窗口派发）
- 验收负责人：GLM 主管决策流（或当班决策流）
- 池序 137；目标仓库=D:\Vit_DAW；**P1** / 预估 0.5-1 天 / 无依赖
- 模型分级：L2 / GLM 执行会话（agent Go 启动/历史面）

## 根因与证据（118 已取证，勿重复）

1. **启动读路径物化 draft**（已证）：`Server.Start()`→scheduler 首 tick `recoverContinuationWorkspace`（continuation_scheduler.go:778-811）→`CurrentProjectIdentity`（harness.go:518-530）→`BindProjectIdentity("", 内核uuid)`（history.go:131-155）→`defaultDraftProjectPath`（history.go:2260-2288）建 75B `Unsaved.vit` 空壳 draft。读路径不应有落盘副作用。
2. **启动期自主历史 reader**（已证，118 真栈 D/E 轮）：agent activation 后约 **1.5–2s** 出现无外部请求来源的 `harness.invoke tool=version.list / version.worktree_list`，把隐式 draft 的 `.vit_history/vitproj_<旧uuid>/state/worktrees` 物化，使 pristine 前置丢失。因此 118 最小修复仅在「通知早于 reader 落盘」时生效（用户真实时序=是；首装空根仍双 draft）。证据：`coord/runs/FE-START-EMPTY-DRAFT-1/realstack_d_clean_20261018_1421/probe_race/`、`agent_last_snapshot.txt`。
3. **跨工程 uuid 泄漏竞态**（round2 已证，1/5 命中）：`ProjectNew` 后 shadow 刷新完成前并发读路径用旧 uuid 再走 `BindProjectIdentity("", 旧uuid)`→`Open`→`migrateLegacyHistoryWorkspace`（history.go:196-223/2804-2858）在**新 draft 内**物化旧 uuid workspace。

## 目标

- 读路径（identity probe / 自主 reader / shadow 刷新）**零落盘、零 draft 物化**；
- reader 不得破坏 draft pristine、不得跨工程物化历史；
- 泄漏守卫：路径/uuid 绑定校验（旧 uuid 不得在新 draft 内落地）；
- 118 的最小修复（空壳 draft 复用）保留为第二道防线，不回退。

## 文件域

`agent/internal/history/{history.go,working_session.go}`；`agent/internal/harness/harness.go`（CurrentProjectIdentity/reader 调用面）；`agent/internal/chat/server.go`（如恢复路径必要）；同包测试；`coord/runs/FIX-START-DRAFT-SIDE-EFFECT-1/` 工件。越域即停并上报。

## 验收标准

1. 单测：①identity probe/reader 调用后 draft 根**零新增目录/文件**；②pristine draft 在 reader 路径后仍 pristine（或 reader 不再物化）；③并发旧 uuid 绑定不泄漏（复刻 round2 竞态）；④既有 118 两测试与全量测试保持绿。
2. `cd agent && go build ./... && go test ./... -count=1` 0 FAIL + gofmt 净。
3. 真栈（AGENTS §5，PC-RUNTIME-STACK 登记后）：复刻 118 realstack 场景（含 reader 抢跑）断言 `version.project_new` 后 draft 根新增=0、无旧 uuid 泄漏；run ID/工件/被测 agent SHA 入回执。
4. 用户手测 4 条（新建工程 draft 根只新增 1 个 / 打开既有工程不落空壳 / 导入后问轨数 / track.list 轨数一致）由用户排期。

## 停止条件

- 需改内核/前端协议或跨层设计 → 先上报拆卡；
- 不得用弱化 draft 判定/跳过 reader 场景凑通过；
- 不得删除 118 已合入的最小修复。

## 关联

- 发现卡：FE-START-EMPTY-DRAFT-1（118，done；最小修复+适用边界在卡面/REALSTACK_REPORT）
- 相邻：FIX-START-STATE-RECOVERY-1（139 前置语义）、TRACK-LIST-GAP-1（不同根因）

- 领取：2026-10-18 14:55 +0800 / origin/main=3abe0cfb / owner=DSH PC 执行流 exec-start-draft（模型+机器+会话）/ 分支=port/fix-start-draft-side-effect-1（自 3abe0cfb）/ worktree=D:/Vit_DAW_wt_startside1（新建）/ 领取提交=待建；task-12；用户 2026-10-18 确认派发
