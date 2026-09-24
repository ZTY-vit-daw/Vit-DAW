# FIX-TEST-CTXSYMLINK-1：contextruntime 快照路径测试 darwin /var 符号链接兼容

- 优先级 / 预估 / 依赖：P3 / 0.2 天 / mac 跟随回执 2026-09-23 裁定请求 4（§11 登记→开卡处置；确定性平台既有失败，mac 全量测试常红一项）
- 模型分级：L0-L1 / GLM-5.3 flash 可接（单测试函数最小修）
- **背景（mac 回执 §一）**：`internal/contextruntime` `TestDefaultSnapshotPathFindsVitAppFromNestedWorkdir`（context_test.go:537）在 macOS 失败——`/var`→`/private/var` 符号链接致 `DefaultSnapshotPath()` 返回值与 want 字面比较不等（chdir/t.TempDir 解析差）。PC 侧同测试绿（84 包一致）；该包近史无代码变更、五补丁域不含——确定性平台既有问题，非本轮回归。证据 gotest_full_5678a79.log（mac 工件）。
- 目标：路径比较改符号链接等价判定——两侧经 `filepath.EvalSymlinks` 归一后比较（或 `os.SameFile` 等价判据）；**不得弱化测试语义**（嵌套 workdir 向上找到 VitApp/Workspace 的行为断言保持）；EvalSymlinks 失败时的回退语义明确
- 文件域：`agent/internal/contextruntime/context_test.go`（单文件，测试侧）；零生产代码改动
- 验收：①PC `go test ./internal/contextruntime -count=1` 绿；②mac 同命令绿（mac 执行侧复跑或回执带退出码）；③其余路径断言不回归（包内其它测试绿）
- 停止条件：发现生产代码 DefaultSnapshotPath 本身依赖未解析路径语义 → 上交（说明为何测试侧等价判据不适用）
- 领取：2026-09-24 PC 执行会话 A（GLM-5.3，随 FIX-PCA-CERTAUTH-TOKEN-1 搭车，分支 port/fix-certauth-token）
- 回执：已完成。改动仅 `agent/internal/contextruntime/context_test.go` 单测试函数 `TestDefaultSnapshotPathFindsVitAppFromNestedWorkdir`：`want` 改为从 `nested` 目录经 `filepath.EvalSymlinks` 归一后拼接（macOS t.TempDir 位于 /var→/private/var 符号链接后，chdir 后 Getwd 已解析而字面量未解析导致不等）；EvalSymlinks 失败时回退原始 nested 路径（回退语义注释明确）；断言语义不变——仍断言嵌套 workdir 向上发现 VitApp/Workspace 并取其 Logs/agent_context_snapshots.jsonl，仅路径拼写经符号链接归一。零生产代码改动。PC 验收：`go test ./internal/contextruntime -count=1` → ok（exit 0，包内其它测试同步绿，含本函数）。mac 侧验收②待 mac 执行侧复跑同命令回执退出码（PC 会话无法执行 darwin）。
- 验收：**pass（2026-09-24 PC 决策会话）**——单测试函数改动审查通过（EvalSymlinks 归一+回退语义明确，断言语义不变）；PC 侧并入决策侧 84 包全量复跑绿；mac 一条命令复验（`go test ./internal/contextruntime -count=1`）随 mac 转交包带回执退出码即闭环②。
