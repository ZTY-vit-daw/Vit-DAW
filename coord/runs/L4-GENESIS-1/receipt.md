# L4-GENESIS-1 回执：工程账本 genesis 头部段接线——工程打开时投影摘要行一次性入账

- 执行侧：PC（GLM-5.3 flash）；执行方式：独立 worktree `D:/Vit_DAW_wt_genesis`，分支 `port/l4-genesis-1`
- 本回执随分支提交；run 工件目录不入版本（与既有 coord/runs 纪律一致）。

## 1. 领取基线

- 领取时间：2026-10-08 21:2x（晚窗）
- 领取时主仓 HEAD：`9e30ac5f3da8a90f51b420b66c7a66f0b8cf2a9c`（= origin/main，fetch 后无新提交）
- worktree 基线：origin/main @ `9e30ac5f`，工作树净
- 领取前主仓已有 diff（本卡零触碰）：`VitApp/Workspace/Settings/Settings.xml`、`VitApp/Workspace/default_project.xml`、`agent/internal/contextruntime/history_refs.go`（修改）+ coord/runs 未跟踪工件

## 2. 锚点实核（领取时回查，卡面行号零漂移）

1. 写入 API：`agent/internal/contextruntime/carriers/ledger.go:223`（GenesisFacts{TracksSummary/BusTopology/DeliveryTargets}）+`:232`（AppendGenesis——空账本写入、非空幂等跳过、空语句组跳过）✅ 与卡面一致，零改动。
2. 挂点：`agent/internal/harness/harness.go:2694`（version_project_opened case）+`:2750`（applyExternalProjectOpened）✅；挂接点=refresh 之后 `:2797`，一行尾挂 `h.appendProjectLedgerGenesis(projectPath, result)`（harness.go:2800，diff +3 行）。
3. 投影摘要行消费面（红线实核=无需扩投影接口，停止条件不触发）：
   - TOM：`capabilitycontext.BuildProjectTOMProjection`（chat 侧既有消费面，capability_runtime_canary.go:287 同源）→ `llm_context.summary_md` 概览行 + `full_assignment_manifest` 逐轨行；只读投影 map，不回读 raw rows、不代发 observe。
   - 总线拓扑：内核项目状态快照 track 行（`parent_track_id`/`parent_folder_track_id`/`is_submix_folder`/`routing_bus_enabled`/`vit_type`，VitApp TrackService.cpp:65-91 序列化在案）在接线层文本化——与 TOM 消费面同一快照源，未动任何投影包接口。
   - 交付目标：`rlm.BuiltinDeliveryProfiles()` + `h.RenderProfileBindingsSnapshot()`（harness render_profile.go 既有只读面）。

## 3. 挂点 diff 形态

| 文件 | 变更 |
|---|---|
| `agent/internal/harness/harness.go` | +3 行（挂点一行+注释两行，applyExternalProjectOpened refresh 之后） |
| `agent/internal/harness/project_genesis.go` | 新增 345 行（事实渲染+fail-open+账本目录解析） |
| `agent/internal/harness/project_genesis_test.go` | 新增 215 行（幂等/fail-open/路由文本化四测试） |
| `agent/internal/contextruntime/carrier_dirs_test.go` | 新增 88 行（REVIEW-1 G-2） |
| `scripts/dev_agent_smoke.ps1` | +190/-3（l4_genesis 场景：注册+三腿+工件落盘） |
| `coord/cards/todo/2026-10-08-L4-GENESIS-1.md` | 卡面回填（领取/回执两行） |

AppendGenesis/GenesisFacts 零改动（幂等语义保持）；账本只追加语义未触碰。

## 4. 测试（幂等/fail-open 形态名）

- `TestProjectLedgerGenesisAppendsOnceThenIdempotent`——幂等形态：真栈路径形态（project_path=.vit 文件、账本落父目录）首次入账 + 再开零新增 + 文件字节不变。
- `TestProjectLedgerGenesisFailOpenWithoutReadableProjection`——fail-open 三形态：shadow 不可用 / 影子身份不可证（串工程防护）/ 空状态，均不报错不落盘警告可见。
- `TestProjectLedgerGenesisFailOpenOnCorruptLedger`——损坏账本前缀不被改写、警告可见、不阻塞。
- `TestGenesisBusTopologyRendering`——路由树文本化（父边/总线标记/不可解析父丢弃）。
- REVIEW-1 G-2：`TestDefaultCarrierWorkspaceDirProbesWorkspaceRoot`（仓库根/agent 内层/VitApp 内层三分支）+ `TestDefaultCarrierWorkspaceDirFallsBackToCwdOutsideRepo`（非仓库 cwd 拼接回落 + detectWorkspaceRoot 空串语义）。
- carriers 层既有 `TestGenesisRendersOnceAndTopoDeltaKind` 零改动持续锁定幂等。

## 5. 全量门

- `cd agent && go test ./... -count=1`：见 §8 终态行（回执落盘以终态复跑为准；中途两轮均 90 包 0 FAIL）。
- G0 五条健康检查：全过（history/harness/capabilitycontext/mom/chat）。
- gofmt：本卡三个新文件 LF 级净（`gofmt -l` 零报警）；全树 900+ 报警经 LF 化甄别为 autocrlf 工作树噪声（与 REFSCHEMA-M2 记录同源），harness.go 仅 +3 行 hunk、报警先于本卡存在。

## 6. 真栈烟测（隔离泊位，三腿全过 ×2 轮）

- 泊位：`-Scenario l4_genesis`（本卡新增，注册进 journeyBerth 名单=独占 kernel+agent，拒绝已监听栈，§9 单栈所有权）；内核=`D:/Vit_DAW/Export/staging/runtime/VitApp.exe`（主仓 staged 导出，2026-10-06 构建，内核侧本卡零改动，如实记录被测内核二进制非本卡构建）；agent=worktree 现构建（21:39/21:45 两次构建均含本卡钩子，二进制内钩子串 grep 3 处在案）。
- 被测事件面：`version.project_opened`（=Godot 启动页真实驱动面，证据：`D:/Godot/project/vit-daw-frontend/A5完成后/.vit_agent/.../journal/000001.jsonl` 中 `source=start_page_open_project, tool=version.project_opened`）。
- 腿 1 fail-open：内核持空白工程时通知打开 fixture → 打开照常 ok（不阻塞）+ agent 日志 WARN 可见（"shadow snapshot identity does not match opened project; genesis skipped"）+ 账本零落盘。
- 腿 2 genesis：内核持 fixture 时通知打开 → 账本 2 条 `topology_delta/genesis`：`tom_overview:`（TOM v0.2 status=ready tracks=2 … manifest=2/2 complete | Lead Vocal → Vocals [high] | Bass → Bass [high]）+ `delivery_targets:`（五内建 profile 引用行）；链式 prev_hash 在案。
- 腿 3 幂等：同通知再发 → 账本字节级不变（逐字节比对）。
- Run 工件：`coord/runs/L4-GENESIS-1/20261008_213943`（首轮，LEG1 断言失败轮，见 §7-②）/`20261008_214506`（PASS）/`20261008_214549`（PASS，`SCRIPT-LASTEXITCODE=0`）。
- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File D:/Vit_DAW_wt_genesis/scripts/dev_agent_smoke.ps1 -RepoRoot D:/Vit_DAW_wt_genesis -Scenario l4_genesis -StartKernel -KernelExe D:/Vit_DAW/Export/staging/runtime/VitApp.exe` → 退出码 0（run3 显式捕获）。

## 7. 执行中的证据发现（上交决策侧）

**① 账本目录解析缺陷（跨域，本卡内最小解+锚点上交）**
真栈 project_path 是 `.vit` **文件**路径（sidecar `.vit_agent`/`.vit_history` 与其同级，A5 真实工程盘面+projectstore.Resolve:106 的 ProjectDir=文件父目录语义为证）。`carriers.AppendLedgerEntry` 在 `<projectDir>/ledger/` 下 MkdirAll——直喂 .vit 文件路径必然失败，且 `AppendGenesis` 的 write 闭包（ledger.go:245-248）**静默吞写入错误**→genesis 永远无声 0 条。
- 本卡内解法：挂点侧 `genesisLedgerDir` 解析（project_path 为已存在目录→原样；否则→父目录），对齐 projectstore ProjectDir 语义；单测以真栈文件形态锁定。
- **跨域锚点（未动，待裁定）**：装配读取面（`agentloop/message_loop.go:4147` ProjectDir=runProjectDirFromState 裸路径、`chat/server.go:5802` 同族）与 retain 写入面（`agentloop/exit_retain.go:69`）同样直喂 .vit 文件路径→真栈上 L4 装配读账本永远 absent、retain 写账本永远无声失败。L1-4-IMPL-D 回执端测边界②"retain 真栈落盘样本待下一旅程轮采集"的原因即此。需决策侧裁定统一解析点归属（扩域修或另开卡）；裁定前装配面渲染不到账本属已知边界，非本卡回归。

**② host 生命周期响应传输白名单（记录，未动）**
`chat/server.go` `compactHostLifecycleInvokeResponseForTransport` 对 version.project_opened/saved/new 响应做 13 键白名单压缩（既有行为，`project_package_restore` 同样被剥）→本卡 result 注记键不外传。卡面"WARN 可见"由日志面承载（logx.Warn，烟测腿 1 断言日志锚点）；传输面是否要加键由决策侧另议。

## 8. 端测覆盖边界声明（AGENTS §5）

1. 真栈三腿覆盖 **host 通知开面**（version.project_opened）。**agent 工具开面**（`project.open` → applyProjectLifecycle open 腿）无 genesis 挂点——卡面挂点候选仅列 opened 事件面，两开面并存属实，agent 发起的 open 不入 genesis（如实申报；若需两面对齐由决策侧裁定）。
2. bus_topology 条目：fixture 工程无路由→该组留空不入账（AppendGenesis 空语句跳过语义）；真栈断言只锁 tom_overview+delivery_targets 在场；带路由文本化由单测覆盖。
3. fail-open 的账本损坏/空状态/shadow 不可用三形态由单测覆盖；身份不匹配形态真栈复现（腿 1）。
4. G-2 的 getwd 失败回落分支：Windows 实测删除 cwd 后 os.Getwd 仍成功（探针在案），不可移植触发；该分支为单行 `return ""`，代码读核覆盖（测试文件头注释同记）。
5. 真栈断言面全部落在 server/kernel 持有面（invoke 状态/账本文件/agent 日志），零 LLM、零回复文本断言（§8 纪律）。

## 9. 稳定性口径（§8）

同代码版本真栈连续 2 轮 PASS（20261008_214506 / 20261008_214549），失败分类零（无环境中断/断言失败残留；首轮 LEG1 失败=断言面选错（响应白名单），属执行偏差非功能缺陷，修正后两轮过）。单元面：harness/contextruntime 触及包多轮复跑全绿。

## 10. 提交

- commit：见分支 `port/l4-genesis-1` HEAD（提交后回填）。
- 提交后核对：`git show --stat --oneline HEAD` 与 `git status --short`（运行工件不入版本）。
