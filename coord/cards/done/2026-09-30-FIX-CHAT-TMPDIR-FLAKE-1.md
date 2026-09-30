# FIX-CHAT-TMPDIR-FLAKE-1：chat 包 TempDir 清理竞态族根治（Windows）

- 池序 9；来源=AGENTS §11 重复出现规则触发——2026-09-30 同日两卡全量 run 命中：VITNOTE-IMPL-4（早窗，首见）+ CLIPSCOPE-AGENT-1（晚窗，再现）；裁定见 [rulings/2026-09-30-CLIPSCOPE-AGENT-1-pass.md](../../rulings/2026-09-30-CLIPSCOPE-AGENT-1-pass.md)
- 优先级 / 预估 / 依赖：P2 / 0.5 天 / 无；agent chat 测试域
- 模型分级：L1 / flash 可接
- 已核实事实（两次失败原始输出均在两卡回执留档）：
  - 失败形态：`TempDir RemoveAll cleanup: unlinkat ...TestWorkspaceSwitchSettlesInFlightChainExplicitly<rand>\001\.vit_agent\vitproj_b5_switch_b: The directory is not empty`——Windows 目录清理竞态，非功能断言失败。
  - 两次均隔离复跑 PASS+整包复跑 PASS。
  - 同族已知项：`TestProcessorCertificationStart*`（CURRENT-STATE 已知不稳定项在案，Windows TempDir 清理竞态同族）。
- 目标：
  1. 族根治：`.vit_agent` 工作区目录的测试句柄释放与清理时序修复——方向（领取后实锚定案）：测试收尾显式关闭/释放内核句柄后再交还 TempDir 清理；或状态目录改用测试专用 TempDir 子目录+清理重试；或 t.Cleanup 顺序修正。要求对两测试名同源生效，不逐测试打补丁。
  2. 验证：`TestWorkspaceSwitchSettlesInFlightChainExplicitly` 与 `TestProcessorCertificationStart*` 两名隔离各 3 轮+整包 2 轮全绿。
- 文件域：agent/internal/chat/ 测试文件（若需生产侧目录参数化小改，越域申报）。
- 验收标准：上述验证轮次全绿+gofmt+全量 0 FAIL；回执附各轮退出码。
- 停止条件：取证发现竞态源于生产代码句柄泄漏（非测试清理路径）→ 上交转生产缺陷卡。
- 领取：2026-09-30 19:32 / origin/main=c4a10d54398ed3b12a7596d0e68d58892116c824 / port/fix-chat-tmpdir-flake-1（独立 worktree D:/Vit_DAW_worktrees/fix-chat-tmpdir-flake-1）；领取时工作树预存改动=VitApp/Workspace/{Settings.xml,default_project.xml}（运行时工程态，非本卡，不动）+暂存区并行卡 FIX-BUCKET-SAVE-RACE-1 领取 rename（他卡会话，不纳入本卡提交）
- 回执：commit=0d5d0d2c（port/fix-chat-tmpdir-flake-1，3 文件 +66/−8：server_test_helpers_test.go 新增 58 / workspace_switch_safety_test.go 换用 1+gofmt 对齐 3 / processor_certification_entry_test.go 换用 4）。**根治方案一句话**：同包测试 helper `newChatServerForTest`——构造即熄灭 continuation scheduler（抢先消耗 schedulerOnce+cancel 上下文，此后 wake 无法自动拉起），t.Cleanup 里 Close()+轮询 certification job 终态，保证全部后台写盘先于 t.TempDir() RemoveAll（cleanup LIFO）执行；零生产代码改动。
  - **族根实锚**：测试构造 `New()` 后存在不被等待的后台 goroutine——①`wakeContinuationScheduler` 在「有 pending 链+workspace 非空」时自动拉起 scheduler（250ms ticker，drive 链多处 persist 写 active workspace 的 `.vit_agent`）；②confirmed certification start 拉起 `runProcessorCertificationJob` goroutine（写 `$HOME/.vit/pca_certifications`，测试把 HOME Setenv 进 TempDir）。测试返回后写盘与 RemoveAll 交错 → Windows "The directory is not empty"。
  - **取证**：目标测试 60 轮复现命中 3 失败——2 次卡面同形清理竞态 + 1 次**功能断言失败**（scheduler 中途抢先续跑 pending 链为 completed：Attempt:1、UpdatedAt 晚 5s——同根因的另一面，证明 goroutine 在测试期间真实活动）；失败残留目录铁证=`...\.vit_agent\vitproj_b5_switch_b\manifest.v2.json`（RemoveAll 窗口内最后写入物）。修复后同规模 60+60 轮全绿，单轮耗时 14s→5.5s（scheduler 竞跑空转消失）。
  - **验证轮次记录**：`TestWorkspaceSwitchSettlesInFlightChainExplicitly` 隔离 3 轮 exit 0×3；`TestProcessorCertificationStart*` 隔离 3 轮 exit 0×3；chat 整包 2 轮 exit 0（89.0s / 85.4s）；gofmt 触碰 3 文件 `-l` 零输出（wss 的 CRLF 检出坑已 `gofmt -w` 归一为 LF，core.autocrlf=true 入库内容不受影响）；全量 `go test ./... -count=1` exit 0（87 包 ok，0 FAIL）。额外高压自验：两测试名各 `-count=60` 均 exit 0。
  - **停止条件未触发声明**：非生产句柄泄漏——生产唯一宿主 `cmd/vitagent/main.go` 显式 `Start()+defer Close()`；wake 自动拉起是被测语义（continuation_scheduler_test.go:362/:1286 注释锁定，其 :1286 还留有同模式 17:20 烟测失败先例记录）。scheduler 续跑语义零影响（语义测试直接构造不经 helper）。
  - **文件域申报**：全在 agent/internal/chat/ 测试文件内（含 1 新增 helper 文件），零生产代码改动，无越域。
  - **同族暴露面申报（供决策侧裁定是否推广）**：另有 5 个构造 workspace 的测试文件未换 helper（cont_stall_arming / g_runtime_recovery_smoke / improvement_proposal_workflow / project_workspace_persistence / turn_control；continuation_scheduler_test.go 属有意测 scheduler 语义不适用）——均无失败留档，且部分依赖中途调度行为；helper 机制对其同源可用。
  - **附注**：`go vet ./internal/chat` 报 semantic_treatment_strategy.go:1087/:1363 unreachable code——main 基线预存（主仓工作树同报），非本卡引入，未处理。
- 验收：**pass（2026-09-30 决策会话）**——裁定=[rulings/2026-09-30-FIX-CHAT-TMPDIR-FLAKE-1-pass.md](../../rulings/2026-09-30-FIX-CHAT-TMPDIR-FLAKE-1-pass.md)；合并 commit 445d3688（cherry-pick）；复跑 chat 整包 ok+两名隔离 ok+零生产改动亲核；同族 5 文件暂不推广（无失败留档，再现随当时卡扩展）；CURRENT-STATE 已知项下次更新改"已修"。
