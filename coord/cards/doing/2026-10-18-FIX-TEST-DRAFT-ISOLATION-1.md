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
