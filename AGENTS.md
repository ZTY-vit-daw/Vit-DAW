# AGENTS.md — Vit-DAW 仓库会话入口

本文件是每个 AI 会话的入场必读。目标：把"每开一个会话都要重付一次的成本"固化在这里。
最后更新：2026-09-07（T7）。

## 0. 决策会话入口（2026-09-05 用户裁定）

用户在新对话中要求“负责派发任务 / 验收 / 做决策”时，该会话担当决策侧；读取调度中枢 `C:\Users\timoz\.zcode\workspace\default\AGENTS.md` 的最新 Vit-DAW 裁定，以及同目录 `queue\` 的任务与回执，再作决定。不要依赖上一段对话记忆，也不要默认承担已派卡的生产实现。

- 用户说“早上打卡 / 开工 / morning / morining”或“下午继续 / 晚间开工 / night”：先核对 doing、done 待审、blocked 和 todo 的依赖/优先级，再派当前可执行卡及可复制的转交提示词。此处是自然语言触发约定，不要求客户端安装斜杠指令。
- 用户交回报告或说“验收”：检查实际 diff、命令退出码、日志与工件，裁定通过、补证、返工或阻塞；必要时更新任务卡或拆修订卡。done 只代表执行侧自验完成，不能替代决策验收。
- 用户说“收工 / gate”：复核当日交付、记录未完与阻塞、更新下一轮排序及交接，不开新开发面。
- 执行模型可能较弱：每卡一个明确目标、固定文件域、可执行验收和停止条件；发现执行失败先判断是执行偏差、卡片缺陷还是假设失效，不能只让它原样重试。决策模型也必须允许证据推翻自己的方案。
- 用户负责转交任务卡与结果；未经另行请求，不自动发消息给执行会话或安排后台调度。具体协议以调度中枢最新裁定为准。

## 1. 常用命令

所有 Go 命令都在 `agent/` 目录下执行：

```powershell
cd agent
go build ./...                     # 全量编译
go test ./...                      # 全量测试
go build -o .\bin\VitAgent.exe .\cmd\vitagent   # 构建 agent 主程序
```

WebUI（React + Vite + vitest），在 `agent/webui/` 下：

```powershell
cd agent\webui
npm run test        # vitest run（含 messageLifecycle 等测试）
npm run build       # tsc --noEmit && vite build
npm run dev         # 本地开发服务器（127.0.0.1）
```

Release 脚本入口（仓库根目录执行）：

```powershell
powershell -ExecutionPolicy Bypass -File .\build_release.ps1
```

（CMake 编译 C++ 内核 → 复制运行文件 → Godot 无头导出 → 打 zip。可选 `-GodotExe` 指定 Godot 可执行文件。）

## 2. 目录地图

三层架构：

| 层 | 位置 | 说明 |
|---|---|---|
| VitApp 内核（C++/DAW） | `VitApp/` | CMake 工程，音频引擎与工程状态 |
| Go agent | `agent/` | Go 实现的桥接与 AI agent（`cmd/vitagent` 为主入口，另有 `vsphub`、`pcactl`、`pluginprobe` 等） |
| WebUI | `agent/webui/` | React 19 + Vite 前端（`src/App.tsx` 目前约 12k 行，拆分是答辩后任务 T9） |
| DAW 前端（Godot） | `D:\Godot\project\vit-daw-frontend`（仓库外） | Godot 4 工程，Vit-DAW 的 DAW 前端 |

`agent/internal` 重点包一览（按主题分组）：

- **循环与编排**：`agentloop`（主 agent 循环）、`orchestration*`（orchestration/controller/runtime/store）、`goalrunner`、`planner`、`harness`、`shadow`
- **观察投影（DAD 之上的 peer projections）**：`dom`、`mom`、`tim`、`tom`、`fxm`、`com`、`epm`、`rlm`、`acousticpackage`、`masking`；证据层相关：`audioclosure`、`frequencycleanup`、`probeaudio`
- **能力与执行**：`capabilityadapters` / `capabilitycontext` / `capabilityinteraction` / `capabilityruntime`、`executionports` / `executionruntime` / `executionverifiers`、`executor`、`processorregistry` / `processorintent` / `processorattestation` / `processorauthority`、`pluginsemantics` / `pluginprobe`、`vst3host`
- **VSP 通信**：`vsphub`（hub 侧，CAS/幂等执行协调）、`vspclient`（agent 客户端侧）
- **会话与状态**：`chat`、`history`（对话图/worktree/分支检出）、`journal`、`projectstore` / `projectworkspace` / `projectpackage` / `projectcut`、`rollback`、`taskstate`、`pendingmanager`
- **上下文与语义**：`contextruntime`、`conversation`、`promptruntime`、`semanticeffect`、`semanticorchestrator`、`mixboard` / `mixcontrolsurface` / `mixdiagnosis` / `mixstyle`
- **其它**：`llm`（LLM 客户端）、`bridge`、`daw`、`kernel`、`webtools` / `shelltools` / `tools` / `toolpolicy`、`policy`、`presenter`、`preview`、`trajectory`、`experiment`、`logx`

## 3. 缩写词典

观察层拓扑（权威定义见 `docs/OBSERVATION_PROJECTION_MANIFEST.md`）：DAD 是 Evidence Layer；DOM/MOM/TIM/TOM/FXM/COM/EPM/RLM 是 DAD 之上的**同级 peer 投影**（不是链！）；peer 投影 → CCB → Context Graph → LLM。

| 缩写 | 含义 |
|---|---|
| DAD | Evidence Layer：事实采集、派生、状态标注和 evidence refs |
| DOM | Dynamics Observation Model（`dom.projection.v1`） |
| MOM | 混音/多轨关系投影（frequency_relationship / static_level_relationship） |
| TIM | 技术完整性投影（`tim.projection.v0`） |
| TOM | 轨道组织投影（ID/命名/长度/mono-stereo/format/轻量波形特征，`tom.projection.v0_2`） |
| FXM | A/B 变换投影（同源同窗同路由 bypass/processed render 对比） |
| COM | Compression Observation Model（source_dynamics + paired 压缩观察） |
| EPM | 执行投影（`epm.projection.v0`） |
| RLM | 参考电平投影（`reference_level_model.projection.v0`） |
| CCB | 观察 view catalog + bounded disclosure 层：按显式 view 请求披露，不做自动 view 选择 |
| PCA | Plugin/processor 控制准入层（evidence-backed control admission，见 `docs/PCA_PLUGIN_LOAD_GATE_V1.md`） |
| VSP | agent 与内核间的执行协调协议/组件（`agent/internal/vsphub` + `vspclient`，CAS + 幂等 Execution Coordinator） |
| FS0–FS9 | 自由态（Free State）状态机层级，宿主为 audioclosure（统一收敛是答辩后任务 T10） |

## 4. 文档白名单

`docs/` 有 100+ 份文档，其中大量已被实现超越、会误导判断。规则：**只读标记为"现行"的文档**——现行清单以 `CURRENT-STATE.md`（仓库根目录，T6 产出）的索引为准。在 T6 完成前，可信赖的少数入口：

- `docs/OBSERVATION_PROJECTION_MANIFEST.md` — 观察投影拓扑 ground truth
- `docs/G0_C2_BASELINE_2026-08-17.md` — G0 基线与验证命令
- `docs/REPO_OPTIMIZATION_TODO_2026-08-22.md` — 本轮仓库优化任务清单
- `docs/FREE_STATE_MINIMUM_IMPROVEMENT_WORKFLOW_UPDATE_2026-08-22.md` — 自由态工作流修订
- `agent/README.md` — agent 构建/运行/端口契约

其余 docs（各类 `*_V1.md` 契约、验证记录、基线）按"历史记录"对待：可查证当时的设计决策，不得当作现状。

## 5. 已知陷阱

- **Windows 路径在 bash 中的引号**：本机常用 Git Bash 操作 Windows 路径。含空格/反斜杠的路径必须加引号并优先用正斜杠（`D:/Vit_DAW/agent`），否则会被 bash 拆词或吞反斜杠。
- **测试不得写源码树**：测试中的运行时状态一律写 `t.TempDir()` 或测试专属工作区，禁止相对路径/固定路径落盘（历史上曾泄漏 `agent/internal/chat/.vit_history/`，T3 已修）。
- **封存测试集纪律**：封存（sealed/holdout）测试不得硬编码轨道名等易变标识；失败时只记录失败类型（不把具体期望值写进代码或日志去"凑"通过）。
- **禁止 `git reset` / `git clean` / 丢弃工作树改动**：工作树中的实现工作被视为不可丢弃的权威输入（G0 基线已声明）。
- **投影是 peer 不是链**：任何实现/文档修改不得把 `DAD -> DOM -> MOM -> CCB` 写成管线；CCB 不做自动 view 选择，缺失/partial/stale 原样保留不升级 readiness。
- **Context Runtime 优先消费投影自带的 `LLMContext`**，不得展开 raw package（多数投影 `do_not_include_raw_package=true`）。
- **禁止 computer use 等主机控制指令**：agent 不得使用控制主机桌面/GUI 的手段（computer use、键鼠模拟截屏操作等）。
- **手测入口=Godot 前端（用户裁定 2026-09-12）**：人工手测/目检的唯一启动途径是**用户从 Godot 拉起 DAW 前端，再从前端启动页进入目标工程**（agent 由此链路拉起）。决策侧为手测写步骤清单时必须按此途径书写——**禁止把 `dev_agent_smoke.ps1 -StartUI` 之类的冒测脚本入口当手测途径交给用户**（脚本栈仅供自动化烟测）。手测需要环境变量（如 `VIT_DAW_AUDITION_BLIND`）时，指导用户在拉起 Godot 的**同一 shell** 先设变量，保证 Godot 拉起的 agent 进程继承。
- **agent 侧改动的验收门槛是端侧烟测**：Go/webui 单元测试全绿不等于可交付。交给用户验收前，改动必须通过按以下方式执行的端侧烟测：
  - **在真实运行栈上测**：由烟测脚本启动真实三件套——VitApp 内核、Godot 前端（`D:\Godot\project\vit-daw-frontend`）、构建并重启 Go agent（如 `scripts/dev_agent_smoke.ps1 -StartKernel -StartUI`）——然后通过 agent 的 HTTP/ZMQ 接口对运行中的栈做验证，不得只测 mock 或编译产物；
  - **复用现有 ps1 脚本体系**：新测试场景以扩展现有冒测脚本（加参数）或按同一模式新增脚本实现，不另起一套测试机制；只读检查参照 `scripts/g_runtime_readonly_smoke.ps1` 的白名单 GET 模式；
  - **通过标准**：脚本以退出码 0（PASS）结束，方视为达到用户验收门槛。

## 6. 健康检查命令

摘自 G0 基线记录（`docs/G0_C2_BASELINE_2026-08-17.md`）。前五条在 `agent/` 下执行：

```powershell
cd agent
go test ./internal/history -run 'Test(BranchCheckoutMakesCheckpointAdvanceActiveBranch|CheckoutCommitEntersDetachedState|WorktreeCreateAndListReturnsMetadata|ConversationGraphNodeCheckoutAndBranchFromNode|ConversationMessagesFollowActiveNodePath|ProjectForkCopiesConversationThenDiverges)$' -count=1
go test ./internal/harness -run 'TestProjectHistory(CheckoutReloadsKernelAndShadow|WorktreeCheckoutOpensTargetProject|WorktreeCheckoutAutosaveUsesKernelProjectPath)$' -count=1
go test ./internal/capabilitycontext -run 'Test.*FreeState.*Observation' -count=1
go test ./internal/mom -run 'Test.*(AB|RenderProbe|Projection)' -count=1
go test ./internal/chat -run 'Test.*(Event|Message|Lifecycle)' -count=1
```

WebUI 生命周期测试（在 `agent/webui/` 下）：

```powershell
npm run test
```

MOM/mixboard ABResult 冒烟（在仓库根目录；走仓库脚本以覆盖完整 mixboard/MOM/chat 模式）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\run_ab_result_smoke.ps1
```

全部通过即健康（与 G0 记录结果一致：五条 Go 测试 + webui 13 个测试 + AB 冒烟均 PASS）。

## 7. 证据等级与验收口径

- `go build` 或 TypeScript 编译通过，只证明产物可构建，不证明运行链路可用。
- 单元测试证明局部契约；合成集成测试证明跨模块逻辑；二者不能替代真实栈验收。
- agent 侧生产改动必须按 §5 的真实栈烟测执行，并以脚本退出码 0 作为交付门槛。
- 日志出现某个阶段名、候选或 mutation，只证明链路进度；不能替代 evaluator 断言、持久化核对或最终退出码。
- 里程碑报告必须分别写明“链路进度”和“最终验收”；不得把真实插件装载、proposal accepted 或局部 PASS 写成端到端完成。

## 8. 概率性运行、止损与失败分类

- LLM 参与的运行必须在执行前写明运行次数、成功条件、失败分类和止损线；不得在看到偶然成功后倒推验收标准。
- `no_candidate_found`、`capability_blocked`、断言失败、崩溃、环境中断必须分开记录。环境中断只有有原始日志或退出证据时才可排除出有效轮次。
- 一次真实栈 exit 0 只证明存在一条成功路径；若要证明稳定性，必须另行规定重复成功比例和样本数。
- 同一代码版本在同一确定性断点连续失败两次后，禁止原样重跑；必须先补代码锚点、缩小假设或拆出取证/修复卡。只有明确属于环境中断或前段模型随机分支时，才可继续使用剩余轮次。
- 止损线只能停止继续投入，不能把未发生的成功改写成成功，也不能把未排除的基础设施错误归因于模型能力。

## 9. 真实运行栈所有权与工件契约

- 同一时刻只能有一个执行会话拥有同一套 VitApp 内核、Godot 前端和 Go agent 进程；不同会话不得并行重启或复用同一端口栈。
- 真栈回执至少记录：完整命令、退出码、run ID、测试时 HEAD 与工作树状态、`d1_smoke_report.json`、agent 原始日志、关键开关；使用 `-SkipBuild` 时还要记录被测二进制的时间或哈希。
- 每次运行使用新的工件目录，不覆盖旧 run。报告中的关键结论必须能回指原始工件。
- 使用 `-SkipBuild` 前必须确认二进制包含待测改动；不能把旧二进制的结果归给当前工作树。
- CPU、端口、残留进程等环境故障先按环境中断记录，再决定是否重跑；不得把环境中断的下游断言失败当作功能结论。

## 10. 测试、冒测与权威输入防污染

- 单元测试、集成测试、烟测脚本和调试脚本都不得直接修改 sealed/holdout fixture、仓库样例工程、用户真实工程或权威基线；写入必须先复制到独立临时工作区。
- 关键 fixture 或样例工程在运行前后应核对哈希；发现污染时先定位写入者并隔离影响，再判断功能结果。
- 测试运行时状态写入 `t.TempDir()`、专用临时目录或脚本创建的隔离工作区，禁止落到源码树、固定历史目录或共享真实工程。
- 不得通过弱化 G7/G8、改变 sealed 期望、硬编码轨道标识或加入领域动词来凑通过。

## 11. 持久化兼容、不稳定测试与越域处理

- 新增持久化字段必须定义旧状态缺省语义，并补旧记录反序列化/往返测试；未知枚举值须明确 fail-closed 或兼容策略。
- 不得用新增字段悄悄改变旧记录含义。修改 project/workspace/history 格式时，至少验证一份旧工件可以加载或明确记录不兼容边界。
- 不稳定测试首次失败要保存原始输出；只有隔离复跑、整包复跑、失败类型和当前 diff 都排除关联后，才能标记为 known flaky。重复出现应开修复卡，不能永久豁免。
- 执行中若证据推翻任务卡前提，停止实现并上交证据；不得为完成原卡擅自修补其他层。新缺陷位于卡片文件域之外时，提供代码锚点和最小复现，由决策侧决定扩域或另开卡。
- `AGENTS.md` 只保留长期规则；具体 run ID、一次性故障经过和当日排序写入 `CURRENT-STATE.md`、任务卡或 gate 报告。

## 12. Git 提交与跨卡工作树纪律

- 工作树中的已有改动是权威输入，禁止使用 `git reset --hard`、`git clean`、覆盖式 checkout 或其他方式丢弃、隐藏未知改动。
- 卡片状态分为：执行完成、决策验收通过、已提交。执行侧 `done` 和自验通过不能替代决策验收，也不能自动视为已提交。
- 每张实现卡记录领取时的 `git rev-parse HEAD`、`git status --short` 和 `git diff --stat`，用于区分本卡新增 diff 与领取前已有 diff。
- 一张实现卡经决策侧验收通过后，应在领取下一张会修改相同文件域的卡之前形成 Git commit。原则上一个 commit 对应一张卡或一个可独立回滚的逻辑变更；依赖卡按依赖顺序形成连续 commit。
- 领取卡前若发现其他卡或人工改动，不得擅自整理、覆盖、回退或 stash；先记录文件域和冲突，再继续原卡或上交决策侧。
- 同一文件已叠加多个未提交任务时，不再领取重叠文件域的新卡；先完成 diff 归属核对、必要测试和按依赖顺序提交。
- 未经决策侧明确授权，不得使用 `git rebase`、`git commit --amend` 或改写公共历史。
- 提交消息包含任务卡 ID；提交后核对 `git show --stat --oneline HEAD` 与 `git status --short`，回执记录 commit hash、测试命令及退出码，适用时记录真实栈 run ID。
- commit 不得混入运行时工程状态、临时工件、构建产物、浏览器目录或本地缓存。验收报告和长期工件按各自目录保存，不因代码提交自动纳入版本。
