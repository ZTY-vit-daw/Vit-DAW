# FIX-TEST-DRAFT-ISOLATION-1：既有测试写真实 draft 根（AGENTS §10 隔离违规）

- 发卡：12:00-18:00 决策流 / 2026-10-18 午窗（依据=FE-START-EMPTY-DRAFT-1(118) 执行测试时发现：[test_isolation_finding.md](../../runs/FE-START-EMPTY-DRAFT-1/evidence/test_isolation_finding.md)）
- 派发确认：**待用户确认**（P2 hygiene；建议与 118 返工/下一窗口合并派发）
- 验收负责人：GLM 主管决策流
- 池序 140；目标仓库=D:\Vit_DAW；P2 / 0.5 天 / 无依赖
- 模型分级：L1 / PC 执行流

## 现象（118 取证）

- 首跑全量 `go test ./...` 时，仓库既有测试把 **4 个 draft 写进真实 draft 根**（`C:\Users\timoz\AppData\Local\Vit\ProjectHistory\drafts\`，内容=测试工程历史、无用户数据）。
- 执行侧已移出到隔离区未删除：`C:\Users\timoz\AppData\Local\Temp\fedraft1\test_pollution_quarantine\`（明细 evidence/test_isolation_finding.md）；真实 draft 根复核 0 遗留。
- 违反 AGENTS §10「测试不得写源码树/固定历史/共享真实目录」。

## 目标

定位未隔离测试（history 包及其他以默认 draft 根运行的测试）→ 改为 `t.TempDir()` 或 `VIT_HISTORY_DRAFT_ROOT` 隔离；补回归：全量测试前后真实 draft 根目录数不变。

## 文件域

`agent/**/*_test.go`（按定位结果最小面）；必要时 `agent/internal/history` 测试辅助函数；`coord/runs/FIX-TEST-DRAFT-ISOLATION-1/` 工件。**不改产品代码**（除测试辅助）；越域即停。

## 验收标准

1. 定位报告：哪些测试、为何写入真实 draft 根（逐条 file:line）。
2. 修复后：`cd agent && go test ./... -count=1` 前后真实 draft 根目录数不变（附前后清单+时间戳），95 包 0 FAIL + gofmt 净。
3. 新增/修改测试不得依赖共享固定路径（grep 证据）。

## 停止条件

- 修复需改产品代码/全局环境注入 → 先上报；不得删除隔离区证据（保留到验收后由决策侧处置）。

## 关联

- 发现卡：FE-START-EMPTY-DRAFT-1（118）
- 说明：隔离区 4 个污染 draft 由决策侧在验收后处置（删除或留档）。

- 领取：2026-10-18 14:55 +0800 / origin/main=3abe0cfb / owner=DSH PC 执行流 exec-start-draft（模型+机器+会话）/ 分支=port/fix-test-draft-isolation-1（自 3abe0cfb）/ worktree=D:/Vit_DAW_wt_testiso1（新建）/ 领取提交=待建；task-14，**串行在 task-12(137) 之后**（避免 history 测试文件域交叉）；用户 2026-10-18 确认派发

- 回执（2026-10-18 晚窗，exec-start-draft 执行，Lead 转写）：分支 **port/fix-test-draft-isolation-1@ead01a47**（已推，worktree 干净；**实际基线 origin/main=3e60df15**——卡面领取行 3abe0cfb 为发卡时点，验收以 3e60df15 为准）。**定位**（真实根未设 env）：逐包串行扫描 107 包 → 仅 internal/chat（+2 目录）/internal/harness（+1 目录）写真实根，其余 105 包 0 新增 0 失败；逐测试定位 chat **9 个**（free_state_owed_round_guidance_test.go:126、fs_adopt_closure_test.go:28、server_test.go:2397/:3675、traj_auto_settle_test.go:441/:547/:729、free_state_arm_slot_restore_test.go:87/:136）+ harness **1 个**（loudness_watch_hook_test.go:101）；根因=defaultDraftProjectPath 进程内缓存——每包首个无 env 写路径物化一次真实 draft，逐测试隔离才根治（LOCALIZATION.md + per_test_logs_internal_chat 108 日志 + pollution maps 在卷）。**修复（零产品代码）**：chat server_test.go 既有 TestMain 与 harness 	estmain_test.go 既有 TestMain——未外部提供 VIT_HISTORY_DRAFT_ROOT 时 os.MkdirTemp 建 per-run 临时根并设置、m.Run() 后删除（外部 CI env 优先；测试内既有 t.Setenv 继续生效）。**验证**：go build ./... exit 0；go test ./... -count=1 exit 0（95 包 ok/12 no test/0 FAIL）；**真实 draft 根 10185→10185（added=0/removed=0，前后清单+时间戳 evidence/）**；changed files gofmt 净（cmd/* 既有未格式化文件非本卡改动）。**边界**：定位窗口（17:56–18:10）与 139 真栈腿重叠，真实根出现 4 个带 .vit_agent/session 的**运行时 draft**（非测试污染；已由 Lead 隔离到 %TEMP%\fedraft1\stack_leg_orphan_drafts\，见 BATCH9 §二）；本卡定位过程自身 11 个测试污染目录已移入 %TEMP%\fedraft1\task14_test_quarantine\（未删）；118 隔离区 4 个 draft 未删。
- 验收：**pass（2026-10-18 晚窗）**——[rulings/2026-10-18-BATCH9-rulings.md](../../rulings/2026-10-18-BATCH9-rulings.md) §一；决策侧独立复验=diff 亲读（仅两处 TestMain 测试辅助；零产品代码）+**独立全量复跑（未设隔离 env）**：go build exit 0、go test ./... exit 0（118.8s）、真实 draft 根 **10185→10185（delta=0）**、changed files gofmt 净、远端 hash 亲核；边界采信并另立卫生卡 **FIX-STACK-DRAFT-ROOT-ISOLATION-1（池序 144，P2，待用户确认）**；cherry-pick 入 main；卡归 done。