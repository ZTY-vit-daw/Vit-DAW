# Mac 开仓回执（MAC-OPEN-1）

- 日期：2026-09-16；执行：Mac 执行会话（ZCode / GLM-5.3，MacBook Air，darwin arm64）
- 上依据：[MAC_SESSION_HANDOVER_2026-09-16.md](MAC_SESSION_HANDOVER_2026-09-16.md) §1 开仓自检、§4 协作协议
- 本分支同时承载首张卡 C1 的交付；本文件为开仓回执 + 阻断项上报，供决策侧验收与裁定

## 1. 开仓自检结果（2/3 通过）

| 项 | 结果 | 证据 |
|---|---|---|
| ① `git log --oneline -1` | ✅ | `ee0fac4`（=移交文档入库 commit，=origin/main） |
| ② `git submodule status` | ❌ **阻断** | pinned `0981e7ce` 不可达，详见 §2 |
| ③ `agent: go build ./...` | ✅ exit 0 | Go 1.24.1 darwin/arm64（go.mod 要求 1.24.1，见 §4 环境偏差） |

克隆方式：SSH 22 端口不通，走 HTTPS 匿名克隆（浅克隆重试后主仓完整、子模块见 §2）。

## 2. 阻断项上报（需决策侧裁定）

### B-1 tracktion_engine pin `0981e7ce` 公共上游不可达

- GitHub API `GET /repos/Tracktion/tracktion_engine/commits/0981e7ce...` → 422 "No commit found for SHA"（截短 SHA 同样 422）
- `git ls-remote origin` 全部 318 个 ref 中无此 tip；`git fetch <sha>` 被拒（"not our ref"）
- 浅克隆/解除浅克隆多次重试均失败；子模块当前停留在上游默认分支顶端 `150582e`（**非 pin**，工作树 `M tracktion_engine` 即此差异，未提交）
- 推断：pin 指向的 commit 在公共上游已不可达（上游历史改写或 pin 来自不可引用对象，如未合并分支/PR head）。与 PORT_AUDIT §1.1「pinned 0981e7ce，公共上游」的记载冲突——**该前提被证据推翻**，按 AGENTS §11 停止自行修补，上交裁定
- 建议决策侧：在 Windows 本地 `D:\Vit_DAW\tracktion_engine`（含该 commit）核对 `git log --oneline -1 0981e7ce` 的语义等价上游点后**重 pin**；或将该 commit 推到可达位置（fork/私有仓）供 Mac 获取
- 影响：层 A 内核线（A1 编译验证/A3/A4/A5）全部阻塞；Go 侧线（C1✅/A2/C4）不受影响

### B-2 主仓裸 gitlink 残留：`tmp_sodium_check` / `tmp_sodium_cmake`

- `git ls-files -s` 显示两个 mode 160000 条目（`77e1ce5d` / `efe978b5`），**不在 `.gitmodules`**，无 URL 映射
- 名称为 libsodium 验证临时目录，疑似 D1 历史清洗期误入索引的残留；本会话未触碰，上交裁定是否移除

## 3. 首卡 C1 交付回执（同分支前一 commit `c40c876`）

- `defaultPluginScanPaths` 平台化：Windows 语义零变化 / darwin 返回 mac VST3 标准双目录 / 其余平台显式 nil；消解 PORT_AUDIT §1.4 护栏盲区与 R5 已知实例
- 验证：`go build ./...` 与 GOOS=windows/linux/darwin 交叉编译均 exit 0；`go test ./internal/harness -run TestDefaultPluginScanPaths -count=1` exit 0
- 已知边界：harness 整包 2 例失败（`TestFullProjectAccessDoesNotDisplaceExplicitSelectionAuthorization`、`TestAgentProcessorLoadGateRechecksPCAAndBlocksBypass`）——在 main 基线 `ee0fac4` 干净 worktree 复跑同样失败，属 **mac 环境既有失败**，与本卡 diff 无关，建议另开排查卡
- 端测覆盖边界（按 handover §4 声明）：本卡验证止于单测+交叉编译层；mac 等价只读冒测（C4）就位前未做真实栈验证，扫描路径实际效果待 C3/C4 真机确认

## 4. 环境偏差记录（U5 落地情况 + 绕行）

移交文档 §5 的 `brew install` 路线在本机不可行（无 sudo 输入途径；ghcr.io 不通），已全部用无 sudo 方式绕行，功能等效：

| 工具 | 版本 | 位置/方式 |
|---|---|---|
| Xcode CLT | 系统自带 | `/Library/Developer/CommandLineTools`（含 python3 3.9/pip3） |
| Go | 1.24.1 | `~/opt/go`（go.dev tarball；`GOPROXY=https://goproxy.cn,direct` 已持久化） |
| gh | 2.101.0 | `~/opt/bin/gh`（已 `gh auth login`，账号 ZTY-vit-daw，scopes 含 repo） |
| CMake | 4.4.3 | `~/opt/bin/cmake`（Kitware 官方 tarball） |
| Node/npm | 22.21.1 | `~/opt/bin`（npmmirror tarball；registry 已配 npmmirror） |
| SCons | 4.11.1 | `~/opt/bin/scons`（pip --user + 清华镜像） |
| Godot | 4.6.stable | `~/Applications/Godot.app`（官方 universal；CLI 验证 `--version` OK） |
| Homebrew | 4.3+（解包版） | `~/homebrew`，**仅骨架**：bottle 源 ghcr.io 不通，未用于装包 |

网络注意（对后续会话有效）：SSH 22 不通；golang.org/proxy.golang.org/ghcr.io 被墙；本机有 Clash 系统代理 `127.0.0.1:7890`（终端工具需显式 `HTTPS_PROXY`，git 已按 `http.https://github.com.proxy` 配置仅代理 github.com）。Godot 前端工程仍在 Windows 侧（层 B 前需用户传输，见 handover §6）。

## 5. 待决策侧裁定的完整清单

1. B-1：tracktion_engine 重 pin 或提供可达源（阻塞层 A）
2. B-2：`tmp_sodium_*` 裸 gitlink 是否清理
3. harness 2 例 mac 既有失败是否开排查卡
4. C1 验收（本分支 `port/c1-plugin-scan-paths`，commits：`c40c876` C1 实现 + 本回执）
