# AGENTS.md — Vit-DAW 仓库会话入口

本文件是每个 AI 会话的入场必读。目标：把"每开一个会话都要重付一次的成本"固化在这里。
最后更新：2026-08-23（T5）。

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
