# Mac 会话移交文档（MAC-HANDOVER-1）

- 日期：2026-09-16；撰写：Windows 决策侧（ZCode / GLM-5.3）
- 读者：MacBook Air 上的 ZCode 执行会话，**首次开仓必读**；读完即可独立开工，不需要用户转述上下文
- 上游文档：仓库根目录 [AGENTS.md](../AGENTS.md)（协作纪律，约束力最高）、[CURRENT-STATE.md](../CURRENT-STATE.md)（docs 现行索引）、[PORT_AUDIT_2026-09.md](PORT_AUDIT_2026-09.md)（移植工作计划来源）
- 本文档是双端协作的锚点：Mac 侧对仓库状态有疑问时，以本文 + 实时 `git` 核对为准，不依赖会话记忆

## 0. 双端协作模型

- **Windows 侧（`D:\Vit_DAW`）= 决策侧 + 主开发线**：任务卡派发、验收、main 分支日常提交。
- **Mac 侧（本机 clone）= 移植执行侧**：按 PORT_AUDIT 层 A–D 执行移植卡，在 `port/*` 分支上工作。
- **git 私有仓是唯一同步通道**：`https://github.com/ZTY-vit-daw/Vit-DAW.git`（私有）。Mac 侧的一切交付物通过 branch + commit + push 到达 Windows 侧；Windows 侧的验收裁定通过 main（merge）或分支上的评审意见到达 Mac 侧。**用户只负责触发会话，不负责转述内容。**
- **coord/ 是双端协作现行权威（2026-09-16 起）**：卡片状态机（todo/doing/done/blocked）、轮询约定、裁定与决策记录见 [coord/PROTOCOL.md](../coord/PROTOCOL.md)；定时轮询模式下会话由双端各自的工作区自动化驱动，用户不再逐次触发。
- 用户在 Windows 侧说"验收"时，决策侧会看你在远程分支上的实际 diff、命令退出码与日志工件，不看自述。

## 1. 首次开仓（照抄执行）

```bash
# ① GitHub 认证（私有仓，二选一；没装 gh 就先 brew install gh）
gh auth login
# 或：配置 SSH key 后把下文 URL 换成 git@github.com:ZTY-vit-daw/Vit-DAW.git

# ② 克隆（--recurse-submodules 会从公共上游拉 tracktion_engine，仓库较大，耐心等）
git clone --recurse-submodules https://github.com/ZTY-vit-daw/Vit-DAW.git

# ③ 开仓自检（全部通过才算开仓成功，结果记入首个回执）
cd Vit-DAW
git log --oneline -1                     # 应为本文档入库的 commit 或其后续
git submodule status                     # tracktion_engine 应为 bae0331...（私有镜像 pin，无 + 前缀）
cd agent && go build ./... ; echo $?     # 应 exit 0（darwin 交叉编译已审计验证）
```

自检通过后，按 §4 顺序读文档，然后从 §5 领第一张卡。

## 2. 仓库状态快照（2026-09-16 私有仓部署日）

读取时必须重新核对 `git rev-parse HEAD` 与 `git status --short`，不得把本快照当实时状态。

- **私有仓于 2026-09-16 首推**。推送前做了一次历史清洗（`git filter-repo` 剥离 `VitApp/build/` 与全部 >50MB 历史 blob），因此**全部 commit 哈希与清洗前不同**；网上不存在旧哈希，不要试图找。
- 远程现有 30 个分支 + 10 个标签 + main 主干。**日常只基于 main 开工**；其余 `codex/*`、`backup/*` 分支是历史工作线的归档快照（已重写为干净历史），仅供参考，不要基于它们开发。
- `spectrum_data.exr`（227MB 旧测试频谱图）已移出跟踪（用户裁定）；`.gitignore` 的 `*.exr`/`*.wav` 规则接管。**永远不要再把大文件（>50MB）提交进仓**——GitHub 单文件 100MB 硬上限，且会重新污染历史。
- `tracktion_engine` 是 submodule，URL 指向**私有自包含镜像** `ZTY-vit-daw/tracktion_engine`（pinned `bae0331`，2026-09-16 起；原公共上游改写历史除名了旧 pin，事件与裁定见 `coord/decisions/`）。升级它需要决策侧重新制作快照并重 pin，执行侧不得自行改 pin。
- Windows 侧本地有一个 `archive/pre-filter-history` 分支（清洗前旧历史，纯本地保险）；它在远程**不存在也不该存在**，Mac 侧无需关心。

## 3. 必读文件与顺序

1. [AGENTS.md](../AGENTS.md) —— 仓库会话纪律，全部条目对 Mac 会话同等生效。特别注意：§5 已知陷阱（测试不写源码树、投影是 peer 不是链、禁 computer use）、§7 证据等级、§8 概率运行止损、§11 越域处理、§12 Git 纪律。
2. [CURRENT-STATE.md](../CURRENT-STATE.md) —— docs/ 三态索引；**只把"现行"文档当现状读**。
3. [PORT_AUDIT_2026-09.md](PORT_AUDIT_2026-09.md) —— 移植工作计划：六项勘察结论、层 A–D 工时表、风险 R1–R9、§5 修正后的开工顺序、§7 真机待办。
4. 本文档 §4–§6（协作协议与环境）。

## 4. 协作协议（Mac 侧执行规则）

- **分支**：每张卡一个分支，命名 `port/<卡ID>-<短slug>`（如 `port/a3-mac-cmake`）。不直接推 main；不对任何远程分支 force-push 或改写历史。
- **提交**：遵守 AGENTS.md §12——commit message 带卡 ID、写明验证命令与退出码；一个 commit 对应一张卡或一个可独立回滚的逻辑变更；运行时状态、构建产物、浏览器目录不进 commit（`.gitignore` 已覆盖大部分，新增产物要配 ignore）。
- **回执**：卡片完成 = 远程分支上可见完整 diff + 回执信息。回执写在 commit message（简短卡）或 `docs/` 下报告文件（里程碑卡，文件名带日期与卡 ID，并同步索引进 CURRENT-STATE.md）。回执必须区分"链路进度"与"最终验收"（AGENTS.md §7）。
- **验收门槛**：Go/webui 单测全绿 ≠ 可交付。agent 侧改动的门槛是真实栈烟测；在 mac 等价烟测脚本就位（PORT_AUDIT A5/C4）之前，涉运行链路的改动必须在回执中显式声明端测覆盖边界，由决策侧裁定。
- **冲突处理**：main 由 Windows 侧推进。开工前 `git fetch origin && git rebase origin/main`（只 rebase 自己的 port 分支）；发现卡片前提被证据推翻时停止实现并上交证据（AGENTS.md §11），不擅自修补卡外文件域。
- **决策归属**：U1–U5 类用户确认项、依赖升级、跨层架构取舍，一律上交决策侧，不自行裁定。

## 5. 第一张卡之前：环境（PORT_AUDIT U5）

```bash
xcode-select --install          # Xcode CLT（内核 clang 编译必需）
brew install go cmake gh node   # Go agent / 内核构建 / GitHub / webui
brew install scons              # vit_extension（Godot GDExtension）构建
# Godot 4.6 官方 mac 版：手动下载安装（CEF 集成时再处理 godot_cef mac 产物）
```

环境就绪后建议的首批卡（顺序按 PORT_AUDIT §5 修正）：

1. **A3 mac CMake 试编译整备**——首个内核构建；R1（libsodium GLOB）在此消解。
2. **C1 `defaultPluginScanPaths` 平台化**——半小时级，阻塞整个校准层，可并行。
3. **A2 Go 侧 `shm_darwin.go`**——与 A1（内核 ISharedMemorySegment 抽象）成对，A1 开工后跟进。

## 6. 不在仓库里的东西（不要找，也不要自己造）

| 内容 | 说明 |
|---|---|
| Godot 前端工程 | 在 Windows 机器 `D:\Godot\project\vit-daw-frontend`，仓库外。层 B 开工前需用户单独传输（建议届时同样入私有仓） |
| 调度中枢 queue\ 与用户级 skills | Windows 机器本地（`C:\Users\...`），Mac 侧任务来源是本文档 + PORT_AUDIT + 用户派卡，不读 queue |
| `~/.vit/` 机器本地状态 | PCA 白名单 / plugin_semantics 等按设计机器本地；Mac 侧走重校准（层 C），不从 Windows 拷贝 |
| AGENTS.md §1 的 PowerShell 命令 | Windows 侧专用；mac 等价入口本身就是移植卡（A5/C4），不照抄 ps1 |

## 7. 维护规则

- 本文档归"现行"；双端协作协议变更时由发起侧更新并注明日期，重大变更（如新增分支策略、验收门槛）需决策侧 commit。
- 移植完成后本文档降级为历史记录，届时双端协作以 AGENTS.md 为唯一权威。
