# FIX-LW-CONFIG-PERSIST-1：loudness watch 配置关（enabled=false）在 free-state post-run 上下文被还原

- 发卡：12:00-18:00 决策流 / 2026-10-18 午窗（依据=LOUDNESS-B-TRIGGER(90) 修订腿 run2 gate4 缺陷上交；证据 [coord/runs/LOUDNESS-B-TRIGGER-1/run2_20261018_121008_slim/](../../runs/LOUDNESS-B-TRIGGER-1/run2_20261018_121008_slim/) 的 `gate4_config_off_repro.{txt,json}`/`gate4_agent_log_excerpt.txt`）
- 派发确认：**已确认（用户 2026-10-18 午窗选择「现在派发第三执行流」；由 12:00-18:00 决策流派发）**
- 验收负责人：GLM 主管决策流（或当班决策流）
- 池序 133；目标仓库=D:\Vit_DAW；P1（阻断 90 终验；loudness watch 配置面状态持久化缺陷）/ 预估 0.5-1 天 / 无代码依赖
- 模型分级：L2 / GLM 执行会话（agent Go 运行时状态与并发/ownership 面）
- 领取（2026-10-18 午窗）：2026-10-18 12:29 +0800 / origin/main=f946e271（派发提交；修复分支自最新 origin/main 建）/ owner=**DSH PC 执行流 exec-lw-fix**（本机 DSH 团队会话；用户指名派发第三流）/ 分支 **port/fix-lw-config-persist-1** / worktree **D:/Vit_DAW_wt_lwfix1** / 领取提交=（本提交）。文件域=`agent/internal/chat/server.go`+同包测试（+`coord/runs/FIX-LW-CONFIG-PERSIST-1/` 工件）；真栈复验腿经 Lead 授权复用 D:/Vit_DAW_wt_loudness_watch_b 的 revision-leg 脚本（只读：不在该 worktree 提交 git 改动，只把本卡构建的 gitignored agent 二进制拷入其 agent/bin），真栈窗口排队等 92 释放（PC-RUNTIME-STACK 排队条）。协作模式（DSH 团队）：协调面由 12-18 决策流单写，实现面由执行流在独立 worktree 写。

## 缺陷（真栈实证，2026-10-18 午窗）

D1 post-run 上下文（run2=20261018_121008，free-state park 后续跑活跃）：

- `loudness_watch_configure {enabled:false}` HTTP 返回 ok+enabled=false（off digest `75d39682f703540698a859655fceb6ca2a6096c28ddc5c47a22d6e42f978913f`），**下一请求即恢复 enabled=true**（on digest `0079aa99…`）；持久化 state 文件被复写 enabled=true；随后 observe 使 checks_total 8→9（「配置关静默」gate4 FAIL）。
- live 复测 2s/5s 间隙均复现；invoke 延迟 8,298–14,775ms（普通上下文 33–45ms）；agent log `[continuation.stall] reason=invocations-active`（12:11:48/12:13:24）；run2 工程激活 `recovered=true conversations=1`（12:25:18）。
- **对照**：普通（非 D1）工程上下文同序列 immediate/+2s/+5s 均 enabled=false（state 文件值稳定）→ 缺陷限定在 **free-state post-run/续跑请求边界 ownership** 上下文，非 LOUDNESS-A 通用持久化。
- trigger 单测已证 disabled→Check 早退静默零成本（loudnesswatch 13 测），故修复面在 chat runtime state 同步/恢复，不在 trigger。

## 代码锚点（初核，执行侧须自行复核）

- `agent/internal/chat/server.go:1658 handleInvoke`（前置 `activateCurrentProjectWorkspace`+`persistCurrentProjectWorkspace` :1675-1677、`defer syncCurrentProjectWorkspace` :1678）
- `server.go:7449 syncCurrentProjectWorkspace` → `:7284 activateCurrentProjectWorkspace` → `:7524 restoreProjectAgentRuntimeStateLocked`（`:7539-7554` 从持久 state 重设 `s.loudnessWatchConfig`）
- `server.go:1592 invokeLoudnessWatchConfigure`（写入 `s.loudnessWatchConfig`）

## 目标

free-state post-run 上下文中的 configure off 语义稳定：configure 返回后，后续 status/observe/续跑持久化不得把 enabled 还原为 true；in-flight 旧快照写入不得覆盖新配置（或恢复语义须保留显式用户配置优先）。

## 文件域

`agent/internal/chat/server.go`（runtime state sync/restore 路径）；必要时 `agent/internal/chat/*_test.go`（新增回归测）。不扩到 loudnesswatch trigger 语义；不动 C++/前端/主仓 scripts。越域=停并上报（AGENTS §11）。

## 验收标准

1. 回归单测（新）：持久 state 中 enabled=true + 显式 configure off 后，后续 invoke/恢复路径不得将配置还原（覆盖 restore 覆盖与 defer sync 两条路径；D1 类似场景可用 fake state/runtime 建模）；`cd agent && go build ./... && go test ./... -count=1` 0 FAIL + gofmt 净。
2. 真栈证明（本卡自证或由 90 补证腿承担，二选一由决策侧验收时认定）：D1 post-run 上下文 configure off → status immediate/+2s/+5s 均 enabled=false 且 observe 不推进 checks_total；提供 run/工件（新目录，登记 PC-RUNTIME-STACK）。
3. 回执含 root cause（restore 覆盖 vs state ownership 竞争）、改动 diff、回归结果、边界。

## 停止条件

- 若根因=free-state 续跑/request-boundary ownership 的更大设计问题（需改 runtime state 同步协议）→ 先上报决策侧拆卡；修复需改动其他层（goalrunner/continuation）先上交。
- **不得**用「gate4 改 WARN / 放宽 smoke 期望 / 跳过配置关断言」凑通过；不得改 sealed/holdout 或既有 gate 文案降口径。

## 关联与边界

- 触发卡：LOUDNESS-B-TRIGGER(90)（doing；终验=本卡修复合入后补证腿，仅需重跑 LW 门）。
- 边界候选（不并入本卡）：feature snapshot `loudness_summaries` 行累积重复（19 行=6 clip×3+1）致 watch measured 重复计入（P2 候选，另行裁定）。
- 决策侧记录：A 收口裁定见 90 卡面「裁定（修订腿·2026-10-18 午窗）」；本卡不改变 90 已证面（D1 PASS+LW 门①②③+E1 入账）。
- 回执（2026-10-18 午窗，exec-lw-fix 执行，Lead 转写）：分支 **port/fix-lw-config-persist-1@2097653b**（实现 36323529：仅 `agent/internal/chat/server.go`+新增 `loudness_watch_persist_race_test.go`，+241/−14；证据提交 30ead248/ba7bb407/2097653b）。**root cause=请求边界 ownership 缺失+deferred persist 时序**（configure 分支 writeJSON 先行、defer syncCurrentProjectWorkspace 后持久化；D1 post-run 上下文 continuation scheduler 每 250ms disk replay 在「内存已 off、deferred persist 未落地」窗口把旧持久快照 enabled=true 整体恢复内存、随后 persist 回写；普通上下文 state 小 replay 毫秒级不出现；D1 state≈20.8MB，configure 14775ms/status 8298ms vs 普通 33-45ms；同族先例 AUTHORITY-LOST-1）。**修复（最小面）**=仿 authorityModeExplicit 加 `loudnessWatchConfigExplicit` latch（显式 configure 置位；restore 在 latch 下保留内存显式配置、其余状态照常重放；workspace identity 切换清 latch 不跨工程泄漏；不持久化、重启以磁盘为准；enabled=true 正常持久化/恢复不变）。**单测**：新增 2 测试双向覆盖 restore replay+deferred sync 两路径，pre-fix 确定性失败签名（`after scheduler replay/deferred sync: enabled=true, want false` 反向亦同）→ post-fix PASS；`go build ./...` exit 0；`go test ./... -count=1` **95 包 0 FAIL exit 0**；gofmt 净。**真栈**：run1=20261018_124040 D1 腿模型随机分支失败（materiality missing/budget exhausted，非本卡面；§8 未烧第三轮）→ run2=**20261018_124639 exit 0**：D1 PASS + **LW 四门+E1 全 PASS，gate4 FAIL→PASS（config-off silent：checks 8→8 不推进）**，E1=`[settle/project_change] n=1 median=18.041ms bakes=0 fresh=19 stale=0 reuse=1.00`；被测 agent SHA256 5A25452ADCCE89…（fix 构建）/kernel B6565DCF…；脚本=90 worktree 4f97bc7e 只读复用；拆栈 12:52:54 六端口=0+四类进程=0。工件 coord/runs/FIX-LW-CONFIG-PERSIST-1/（随本分支入 main）。
- 验收：**pass（终验·2026-10-18 午窗）**——[rulings/2026-10-18-BATCH2-rulings.md](../../rulings/2026-10-18-BATCH2-rulings.md) §一；决策侧独立复验=diff 亲读（最小面/无越域/latch 与 AUTHORITY-LOST-1 同族）+单测 pre/post 证据链亲核+95 包 0 FAIL 日志+真栈 gates/E1 工件亲读（gate_results 六项全 true/config_off_silent=true/runner_exit=0）+12:54 栈现场=0；修复四笔 cherry-pick 入 main（8b8bc70e/ed8dce98/3ce0d088/7dd7854e）。边界采信：真栈成功样本 n=1（存在性证据，稳定性由确定性单测+既有基线背书）；run1 D1 模型随机分支失败已如实取证。卡归 done。
