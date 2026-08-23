# RUNBOOK.md — Vit-DAW 构建 / 运行 / 测试 / 排障手册

面向**从未接触过本仓库的人**（未来的你、答辩委员会、合作者、新 AI 会话）。只凭本文档应能完成构建 + 三进程启动 + 跑通健康检查。

环境假设：Windows 10/11 + PowerShell 5.1+，已安装 Go、CMake + Visual Studio（C++ 工作负载）、Node.js/npm、Godot 4.6（本机位于 `D:\Godot`）。若工具不在 PATH，各节均有指定绝对路径的示例。

架构总览（三层，详见 `AGENTS.md` §2）：

| 层 | 位置 | 进程/产物 |
|---|---|---|
| VitApp 内核（C++ DAW 引擎） | `VitApp/` | CMake 构建，产物 `VitApp\build\VitApp_artefacts\Release\VitApp.exe` |
| Go agent（桥接 + AI agent） | `agent/` | 产物 `agent\bin\VitAgent.exe`，HTTP `127.0.0.1:7878` |
| DAW 前端（Godot） | `D:\Godot\project\vit-daw-frontend`（仓库外） | Godot 4 工程 |
| WebUI（React 管理界面） | `agent/webui/` | 开发服务器 `npm run dev`，非运行三件套之一 |

---

## 1. 如何构建

### 1.1 Go agent（在 `agent/` 目录下）

```powershell
cd D:\Vit_DAW\agent
go build ./...                                      # 全量编译检查
go build -o .\bin\VitAgent.exe .\cmd\vitagent       # 产出主程序
```

注意：**所有 Go 命令必须在 `agent/` 下执行**，在仓库根执行会找不到 module。

### 1.2 VitApp 内核（CMake，在仓库根）

```powershell
cmake -S D:\Vit_DAW\VitApp -B D:\Vit_DAW\VitApp\build_release
cmake --build D:\Vit_DAW\VitApp\build_release --config Release
```

产物路径：`VitApp\build\VitApp_artefacts\Release\VitApp.exe`（dev 烟测另有 staging 拷贝，见 §2）。也可用 `-G "Visual Studio 17 2022"` 指定生成器。

### 1.3 WebUI（在 `agent/webui/` 下）

```powershell
cd D:\Vit_DAW\agent\webui
npm install        # 首次
npm run build      # tsc --noEmit && vite build
```

### 1.4 一键发布包（可选）

```powershell
powershell -ExecutionPolicy Bypass -File D:\Vit_DAW\build_release.ps1
# Godot 不在 PATH 时：-GodotExe "D:\Godot\Godot_v4.6.1-stable_win64_console.exe"
```

脚本依次完成：CMake 编译内核 → 复制 VitApp.exe/bridge 脚本/DLL → Godot 无头导出 → 打 zip 到 `Export\`。发版前还需 `scripts\bundle_python_embed.ps1` 打入内嵌 Python。

---

## 2. 如何运行（三进程启动顺序与依赖）

端口契约（权威来源 `agent/README.md` 与 `docs/VIT_IPC_CONTRACT.md`）：

| 端口 | 用途 |
|---|---|
| `tcp://127.0.0.1:5555` | VitApp 内核命令 ZMQ REQ |
| `tcp://127.0.0.1:5556` | VitApp 内核遥测 ZMQ SUB |
| `127.0.0.1:4445` / `:4444` | Godot 命令/遥测 UDP（经 bridge） |
| `http://127.0.0.1:7878` | VitAgent HTTP API |

有两种启动方式，按场景选择：

### 2.1 产品路径（日常手测推荐）

在 Godot 编辑器中打开 `D:\Godot\project\vit-daw-frontend`，按 **F5** 运行：GUI 启动后会**自行拉起内核和 agent**，无需手动启动另外两个进程。这是产品路径（烟测会验证它启动的是预期的 `agent\bin\VitAgent.exe`）。前提：`agent\bin\VitAgent.exe` 和内核运行文件已按 §1 构建就位。

### 2.2 脚本路径（自动化烟测）

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\dev_agent_smoke.ps1 -StartKernel -StartUI -MixSmoke
```

由脚本启动真实三件套（构建 agent → 启动内核 → 启动 Godot 前端 → HTTP/ZMQ 检查 + mix 冒烟），不依赖 GUI 交互。也可按下述命令手动分进程启动（调试单个进程时有用）：

```powershell
# ① 内核（dev 烟测使用的 staging 拷贝；也可用 §1.2 的构建产物）
D:\Vit_DAW\Export\staging\runtime\VitApp.exe

# ② agent（-verbose 打详细日志）
D:\Vit_DAW\agent\bin\VitAgent.exe -verbose

# ③ Godot 前端
D:\Godot\Godot_v4.6.1-stable_win64_console.exe --path D:\Godot\project\vit-daw-frontend
```

脚本默认复用已在跑的 agent；`-RestartAgent` 才会换新二进制重启（`scripts\restart_agent.ps1` 是其单进程版：停旧 PID、起新进程、等 7878 就绪）。

agent 的 LLM 配置读取 `%USERPROFILE%\.vit\config.json`：

```json
{ "baseUrl": "https://api.openai.com/v1", "apiKey": "...", "defaultModel": "..." }
```

无此文件时纯 IPC/观察类功能可用，真模型对话验收不可用（见 §3.4）。

运行中可用的 agent HTTP 端点：

- `GET /agent/state`、`GET /agent/tools`（DAW 命令目录）、`GET /agent/actions?limit=50`（动作日志）
- `POST /agent/invoke`，body 例：`{"tool":"daw.invoke","args":{"cmd":"get_project_state"},"source":"debug"}`

---

## 3. 如何测试（分层入口）

### 3.1 单元测试

```powershell
# Go（在 agent/ 下）
cd D:\Vit_DAW\agent
go test ./...

# WebUI（在 agent/webui/ 下，含 messageLifecycle 等测试）
cd D:\Vit_DAW\agent\webui
npm run test
```

### 3.2 健康检查（G0 基线摘录，全绿即健康）

前五条在 `agent/` 下执行（权威来源 `docs/G0_C2_BASELINE_2026-08-17.md`）：

```powershell
go test ./internal/history -run 'Test(BranchCheckoutMakesCheckpointAdvanceActiveBranch|CheckoutCommitEntersDetachedState|WorktreeCreateAndListReturnsMetadata|ConversationGraphNodeCheckoutAndBranchFromNode|ConversationMessagesFollowActiveNodePath|ProjectForkCopiesConversationThenDiverges)$' -count=1
go test ./internal/harness -run 'TestProjectHistory(CheckoutReloadsKernelAndShadow|WorktreeCheckoutOpensTargetProject|WorktreeCheckoutAutosaveUsesKernelProjectPath)$' -count=1
go test ./internal/capabilitycontext -run 'Test.*FreeState.*Observation' -count=1
go test ./internal/mom -run 'Test.*(AB|RenderProbe|Projection)' -count=1
go test ./internal/chat -run 'Test.*(Event|Message|Lifecycle)' -count=1
```

再跑 `npm run test`（webui）和仓库根的 AB 冒烟：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\run_ab_result_smoke.ps1
```

### 3.3 端侧烟测（agent 侧改动的验收门槛）

单元测试全绿 ≠ 可交付。验收必须在**真实三件套**（内核 + Godot 前端 + 重启后的 Go agent）上通过脚本验证，且脚本以**退出码 0（PASS）**结束：

```powershell
# 全栈冒烟：起真实内核/UI，经 HTTP/ZMQ 验证
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\dev_agent_smoke.ps1 -StartKernel -StartUI -MixSmoke

# 只读检查（白名单 GET 模式，参照此脚本新增只读场景）
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW\scripts\g_runtime_readonly_smoke.ps1
```

新测试场景以扩展现有 ps1 脚本（加参数）或按同一模式新增脚本实现，不另起测试机制。脚本目录的稳定入口总览见 `scripts/SMOKE_TESTS.md`。

### 3.4 契约 / 回放 / 真模型验收

- **契约烟测**：`scripts/` 下 `run_*_smoke.ps1` 系列（如 `run_observation_v1_acceptance_smoke.ps1` 观察系统验收、`run_ab_result_smoke.ps1` MOM/mixboard/chat 模式），多数在真实栈上运行并落盘 summary 到 `VitApp\Workspace\Artifacts\smoke\`。
- **回放/离线实验**：`run_mix_acoustic_lab.ps1` 构建 `cmd/mixlab`，用仓库测试音频合成全工程观察，无需内核。
- **真模型验收**：需要 `%USERPROFILE%\.vit\config.json` 配好真实 LLM，然后走 `dev_agent_smoke.ps1` 的 chat 冒烟（不加 `-NoChatSmoke`）。

---

## 4. 常见故障与处置

1. **bash 下 Windows 路径被拆词/吞反斜杠**：含空格/反斜杠的路径必须加引号且优先用正斜杠（`"D:/Vit_DAW/agent"`）。本机默认 shell 是 Git Bash，这是最高频的坑。
2. **Go 命令在仓库根执行报 module not found**：所有 Go 命令必须在 `agent/` 目录下执行。
3. **agent 起不来 / 7878 未监听**：用 `scripts/restart_agent.ps1`（会停旧 PID 并等端口就绪）；确认 `agent\bin\VitAgent.exe` 已按 §1.1 重新构建；`-verbose` 看日志。
4. **烟测报 kernel exe not found**：dev 烟测使用 `Export\staging\runtime\VitApp.exe`，缺失时先按 §1.2 构建内核或运行 `build_release.ps1` / staging 组装脚本刷新拷贝。
5. **Godot 未找到**：给 `dev_agent_smoke.ps1` 显式传 `-GodotExe "D:\Godot\Godot_v4.6.1-stable_win64_console.exe"`；脚本只自动探测 PATH 和少数固定路径。
6. **测试后源码树出现 `.vit_history/` 等运行时残留**：说明有测试写了相对路径（历史泄漏已在 T3 修复）。原则：测试运行时状态一律写 `t.TempDir()`；发现新泄漏按此修复，不要提交残留。
7. **误读过期文档导致实现方向错误**：`docs/` 多数文档是历史记录。只读 `CURRENT-STATE.md`（仓库根）索引中标记为「现行」的文档。
8. **工作树"脏"不敢清理**：纪律是**禁止 `git reset` / `git clean` / 丢弃任何工作树改动**——树中包含不可丢弃的实现工作（G0 基线声明）。改动按主题分组提交，而不是丢弃。

---

## 5. 相关入口

- `AGENTS.md` — AI 会话入场必读（命令、目录地图、缩写词典、陷阱、健康检查）
- `CURRENT-STATE.md` — docs/ 三态索引（只信「现行」）
- `agent/README.md` — 端口契约与 dev smoke 用法
- `scripts/SMOKE_TESTS.md` — 烟测入口总览
- `docs/G0_C2_BASELINE_2026-08-17.md` — G0 基线与健康检查权威来源
