# TRACKLIST-AUTHORITY-VERIFY-1（255）复验收口：139 缺陷形态在当前 main 的真栈复测

- owner：GLM-5.3 L1 PC 执行会话（ZCode，255 卡会话）；worktree `D:/Vit_DAW_wt_255impl`，分支 `port/tracklist-authority-verify-1`（领取提交 5733f17a）
- 复验对象：139 TRACK-LIST-GAP-1 诊断钉死的缺陷形态——**tool face 返回 shadow Summary 忽略 kernel reply；外部（GUI/内核直连）新增/导入轨只以 delta 到达 shadow，权威快照刷新前 tool face 读数=旧 shadow（孤儿轨）**
- 判定：**139 形态不在当前 main——已被 143 修复（88e88327，FIX-TRACKLIST-SHADOW-FRESHNESS-1 A+B）覆盖，且历经 10-18 → 11-05 三周演进（含 231 identity bind 等触碰 shadow 面的工作）仍然存活。走卡面分叉②：证据包收口，不开修复面。**

## 1. 复验方法（可比性）

- 探针=143 修复验收探针**原样复用零改动**（`coord/runs/FIX-TRACKLIST-SHADOW-FRESHNESS-1/run_tracklist_fix_probe.ps1` + `tracklist_fix_probe.py`）；其 A 面 silent-stale/undisclosed 断言直接对映 139 形态。
- kernel=stock `D:\Vit_DAW\Export\staging\runtime\VitApp.exe` SHA256=`B6565DCF85D1DA867E16333052A7B928A5413565940603ADBCF4ABD59DD3123B`——**与 139 诊断 run（run_20261018_175229）、143 修复验收 run（run_20261018_192615）同一字节**。唯一变量=agent 侧代码（139 时代 vs 当前 main 5733f17a）。
- 被测 agent/vsphub=本 worktree 亲手构建：VitAgent `EDD851A6E268C0F7F161186403273735285B1109E762A117E215759F42AE1002`、vsphub `3E43fab18f0f4db662303a8d0d4ca7a4fa4629ce71689af4bbf450cb630fdfc1`（run_meta.json 在卷；-SkipBuild 前提=二进制含当前 main 全部代码，亲手构建亲核）。
- run=verifyrun1（2026-11-05 20:55:47 起，kernel pid19648+vsphub pid3028+agent pid17932，不启 UI）；栈占用登记 20:54 推 main（b619c5b8）+≤60s 即时复核（20:55:38 TCP=0/PROCS=0）后启动。

## 2. 真栈结果（PS1_EXIT=0，probe PASS，七面全过）

run ID=`verifyrun1`（原落 `FIX-TRACKLIST-SHADOW-FRESHNESS-1/run_20261105_205547`，因 ps1:147 `$RunDir=""` 参数自清空缺陷原样移至本卡目录，见 MOVE-NOTE.md）；内核会话 VSP envelope=124 行、legacy list_tracks=30 行；LLM 调用=**0**（agent_last.log 零 llm 行亲证）≈¥0。

六面同帧数值（user-track 口径；kernel=ZMQ 直查，hub_cmd=VSP canonical，hub_state=VSP state snapshot，shadow=/agent/state，tool=/agent/invoke track.list）：

| 阶段 | kernel | hub_cmd | hub_state | shadow | tool | tool==shadow | tool==kernel | stale 披露 |
|---|---|---|---|---|---|---|---|---|
| S0 基线 | 0 | 0 | 0 | 0 | 0 | True | True | — |
| **S1 r1 外部新建+导入** | **6** | **6** | **6** | **0** | **6** | **False** | **True** | **shadow_stale=true（双计数 kernel=6/shadow=0）** |
| S1 r2 外部新建+导入 | 6 | 6 | 6 | 6 | 6 | True | True | — |
| S1_refresh（project.state） | 6 | 6 | 6 | 6 | 6 | True | True | — |
| S2 外部打开既有 2 轨 | 2 | 2 | 2 | 2 | 2 | True | True | — |
| S3 agent 路径写 | 3 | 3 | 3 | 3 | 3 | True | True | — |

### 2.1 vs 139 形态对照（决定性帧）

| 面 | 139 run_20261018_175229（S1 两轮） | 本次 verifyrun1 |
|---|---|---|
| kernel/hub_cmd/hub_state | 6/6/6 | 6/6/6（同） |
| shadow（捕获帧） | 0 | 0（S1 r1 同；r2 已被 B 面 heal 追平） |
| **tool** | **0（==shadow，≠kernel）** | **6（≠shadow，==kernel）** |
| silent stale | 是（139 缺陷本体） | **无**——silent-stale 阶段=[]、undisclosed=[]（探针断言面） |
| stale 披露 | 无此面 | S1 r1 显式 shadow_stale=true+双计数 |
| shadow heal | 无（直到 project.state 才自愈） | B 面结构 delta 去抖刷新：r1 0.26s/r2 0.01s 追平 |

- **判定链**：同 kernel 字节+同探针体系+S1 同构突变（外部 new_project+add_track×6+import，不经 agent 工具）下，tool face 读数已从「旧 shadow（0）」变为「本命令内核回包（6）」，且 shadow 陈旧时显式披露、随后 0.26s 内权威刷新追平——139 三面缺陷形态（A 工具面 shadow-only / B delta 孤儿 / C 无刷新触发）在当前 main 均已不可复现。
- 与 143 验收 run 的形态差异（时序方差，非覆盖缺口）：143 run 的 S2 捕获帧 shadow=6（反向陈旧）→ 本次 S2 heal 0.0s 在捕获前完成（shadow=2）。反向 stale 披露面由单测 `TestPublicListTracksKernelAuthoritativeDisclosesStaleShadow`（S2 反向形态）静态钉住+本次 A 面/披露面断言全阶段通过；如实记录，不计为缺口。

## 3. 静态复核（三面锚点在当前 main 亲核）

- **A 面**：`harness.go` publicResult `case "list_tracks": return publicListTracksResult(h, reply)`（:12285）；`publicListTracksResult`（:12406）=kernel reply 行权威+`shadow_stale`/双计数披露+异常形态 `tracks_source=shadow_snapshot` 显式回落。
- **B 面**：`shadow.go:318` 结构 delta 判定（`node_added` 未知 uid / `node_removed` 已知轨）→ 一次性刷新提示；orphan 语义保留（`TestShadowTracksOrphanDeltas` 不动）。
- **C 面**：`bridge.go:97-98` `shadowRefreshReasonStructural`+去抖 1s；`:460` 结构信号投递；`:537-556` 去抖刷新循环（VSP state snapshot 优先）。
- VSP-SHADOW-REFRESH-1（`ee4a991e`，VSP 执行路径 spec.RefreshAfter 触发刷新）亦在 main 祖先链。

## 4. 分支残余定性（卡面目标 2 收口件）

- **port/track-list-gap-1（ahead5，worktree wt_tracklist1 保留中）**：`git diff main...branch` 21 文件 +6678 行**全部在 coord/**（诊断工件+草稿文档），**零生产代码**；其中 `coord/runs/TRACK-LIST-GAP-1/` 与 main **内容完全一致**（两点 diff 空——c96049e0 cherry-pick 已全量入册），其余差异仅为卡片目录的旧状态快照（历史形态，无独有价值）。
- 分支上的 fix 草稿卡（30abfa0a，tool face kernel-reply authority+shadow delta refresh trigger）**已被 143 实现（88e88327，含单测+真栈验收）超集覆盖**，本次复验证明其存活至今。
- **建议**：分支与 wt_tracklist1 可关线清理（决策侧裁定执行；139 证据链以 main 在册件为准）。

## 5. 偏差与诚实边界

- `-RunDir` 参数未生效：ps1:147 在 try 块前 `$RunDir = ""` 无条件清空参数（脚本固有缺陷；独立 argtest 证明参数绑定正常传入）。工件原样移至本卡目录，MOVE-NOTE.md 留痕；不影响被测栈与判定。该缺陷属 143 探针脚本文件域，本卡不修（越域即停），上交决策侧知悉。
- S1 r2/S2 捕获帧 shadow 已 heal（139 时代 r2 仍 0）——B 面刷新时序在两次 S1 间无重置，属修复后预期行为，非口径放宽（探针判据含 B 面 heal poll 独立断言，r1 捕获帧保留了 shadow=0 的决定性对照）。
- 本次为单 run 复验（§8 跑前成文口径）；证明「存在一条成功路径+139 形态不复现」，未做多样本稳定性声明。
- 端测边界：本复验覆盖 agent HTTP/ZMQ 接口面（探针口径），不涉 webui 渲染面与用户旅程（无相关改动，E2E-WEBUI-1/JOURNEY-1 债务不变）。

## 6. 工件清单

- `RUNBOOK.md`（§8 跑前成文）、`verifyrun1/`（probe_summary.json/probe_console.txt/raw/、kernel_workspace/Workspace/Logs/VitHeadlessServer2026-11-05_20-55-47.log、agent_last.log、vsphub_last.log、kernel_vsp_evidence.json、teardown.json、run_meta.json、MOVE-NOTE.md）
- 拆栈：teardown.json remaining_listeners=0/remaining_processes=0 + 独立复核 20:57:10 TCP=0/PROCS=0（记录于栈释放条目 5642a66f）。
