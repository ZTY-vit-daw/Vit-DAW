# G3-REOPEN-COSTCMP-1 对比报告：probe 计量真实成本对比轮（G3 重开条件①取证）

- 卡：2026-10-11-G3-REOPEN-COSTCMP-1（池序 73；G3 终裁重开条件①"probe 计量源接入后真实成本对比"）
- 执行：GLM-5.3 PC 执行流（ZCode 会话），2026-10-12 晚窗
- 被测代码：port/g3-reopen-costcmp-1 @ 5b55e064（脚本面：原始 chat 响应捕获 8e68f201 + RunArtifactsDir 相对路径硬化 5b55e064；**零生产 Go/C++ 代码改动**）
- run：`coord/runs/G3-REOPEN-COSTCMP-1/20261012_192737_run1` 与 `20261012_193106_run2`（均 **SCRIPT-LASTEXITCODE=0**）+ 三轮启栈取证腿（18:58/19:17/19:22，见 §5）
- 栈占用：登记 fcef17a1 → 释放 470d877f（两 run 脚本自 teardown；19:37 复核四端口 0 监听、VitApp/VitAgent 进程=0）；内核复用主检出 VitApp.exe B6565DCF85D1DA86（与 G3-ATTRIB-1 同版二进制，C++ 面零改动）
- 判定权：本报告只交证据与建议，G3 是否重开归主管

## 1. 运行设定（§8 纪律执行面）

- 两 run 同构：`dev_agent_smoke.ps1 -Scenario harness_ab -HarnessAbRounds 1 -StartKernel -KernelExe <主检出内核> -RunArtifactsDir <绝对路径>`；push（生产缺省）先、pull（`VIT_DAW_HARNESS=pull`）后，每 run 每模式 1 base 轮（J3 话术）+ 3 族轮（f1 响度平衡=J3 同源 / f2 低频清理 / f3 动态控制，G3-ATTRIB-1 话术族原样复用，每轮 fixture reopen 归一基线）+ pull 独有 confirm/budget 腿。**双模式各 2 run 达成卡面下限**。
- **失败分类分记**（卡面纪律）：
  - `脚本调用形态缺陷`×3（18:58/19:17/19:22 启栈腿，确定性）：相对 `-RunArtifactsDir` → 内核双段路径必死——根因、取证与修复见 §5。初始两轮曾按"环境中断"记录，第三轮包装器取证后**改记确定性脚本缺陷**（初始分类如实保留演进过程）。
  - `环境中断（API 额度耗尽）`：两 run 全部 LLM 面腿受 `预扣费额度失败, 用户剩余额度: 0.000190` 截断（与 M2-TIMEFX-1 白天 111841 轮同源，额度未恢复）。**差异性截断**：push 大上下文调用预扣 ~0.0249 全败、pull 小额入口调用偶过——两模式截断面不同，见 §2 有效性的影响。
  - `断言失败`：0（确定性门全过：fixture/open/authority/DAD/telemetry/轮分类/metrics 写盘）。
  - 模型随机分支：f3 两 run 两模式均挂确认卡（waiting_confirmation）；f1/f2 均被额度截断（不可归因话术）；无同话术换族操作。
- 止损线：未触发（无同模式连续 2 轮 infra_error_*；两 run 均健康退出）。

## 2. 三口径对比表（主交付）

### 2.1 口径③ probe 物理成本（墙钟）——**本轮完整有效**

数据源：原始 chat 响应捕获（`harness_ab_chat_raw_*.jsonl`）中 `executed_kernel_reply[].elapsed_ms`，按 PULL-PROBE-METER-1 分级表求和（本轮每族轮恰好一次 probe 级调用 `mix.observe`，零 index 级、零 unlisted——**probeCostKnown 语义下账本无翻 false 项**）。逐轮明细在 `analysis.json`（两 run 各一），原始捕获留盘可回指。

| 相 | push（6 族轮调用） | pull（6 族轮调用） |
|---|---|---|
| run1（f1/f2/f3） | 871 / 1045 / 890 ms | 854 / 1344 / 895 ms |
| run2（f1/f2/f3） | 841 / 832 / 837 ms | 899 / 868 / 840 ms |
| **族轮合计 / 均值** | **5316 ms / 886.0 ms** | **5700 ms / 950.0 ms** |
| base 轮（参考） | 1735 / 1719 ms（健康完成） | 831 / 841 ms（额度截断于首边） |
| 区间（族轮） | 832–1045 ms | 840–1344 ms |

**结论③：probe 物理成本两模式对称（差异 64ms/均值≈7%，n=6/组，在调度噪音带内；同机同 fixture 同夜配对）。** 该计量的语义=agent 观测墙钟（含 OS 调度、ZMQ 传输、内核渲染全程——**调度噪音声明：非纯内核成本**，与 PULL-PROBE-METER-1 勘察注记一致）。pull 账本（probeSpent/probeCostKnown，ms 口径）消费的正是同一 execRecord.elapsed_ms 源、同一分级表——物理口径下 push 与 pull 各自的 probe 支出无结构差异。

### 2.2 口径① 轮级 LLM 调用数——**被额度差异性截断，本轮不构成有效对比**

| 相 | push | pull |
|---|---|---|
| run1 记录数（base+3族+额外腿） | 7（base 2 / f1 2 / f2 2 / f3 1） | 6（六轮各 1，全截断于首边后） |
| run2 同 | 7 | 6 |
| 脚本 metric（message_loop/pullharness 源） | 3 / 3 | 5 / 5 |

读数解读：push 每族轮"首次全上下文调用成功+终答调用被额度拒"（2 条记录）；pull"语义入口小调用成功+pullharness 循环调用被额度拒"（1 条记录，循环调用未产生 section_stats）。**截断面随模式调用价格结构不同而不同（差异性删失），任何方向的双模式差都是额度墙伪象**。健康链的轮级调用对比仍以 G3-ATTRIB-1 的 17 vs 6（含 confirm/budget 腿）为现行证据。

### 2.3 口径② 逐 turn 注入——**同受差异性截断**

- push base 轮（两 run 唯一健康完成轮）：prefix_bytes=22,946、dynamic_bytes≈22,060–22,491/调用（2 调用/轮）。
- pull：pullharness 循环调用全部被额度拒于预扣费 → **零 section_stats 记录**，注入面本轮无 pull 读数。
- 结论：注入口径本轮只有 push 单侧读数，不构成对比；健康链证据仍为 G3-ATTRIB-1（pull 首调 prefix 34,316–34,476 含冷启动快照）。

## 3. 账本位核对（停止条件核对）

- **形态核对**：`execRecord.elapsed_ms`（runner.go executeTool 墙钟 ms）+ `probeToolTier` 分级（probe=ccb.observation_request/mix.observe/mix.request_observation/clip.warm_waveform_bake；index=ref.query/ref.diff/ccb.observation_catalog/project.state 不计；unlisted 翻 false）+ `settleBatch→settleRecordProbeCost` 逐笔累加 `ledger.probeSpent`——与 PULL-PROBE-METER-1+TIER-EXT-1 合入形态一致，**停止条件（形态不符）未触发**。
- **账本位披露面缺口（如实记录）**：本轮无任何 pull 披露窗开过——①budget 腿死于额度而非 `observation_budget_exhausted`（无 probe_spent=X/Y 错误面）；②pull f3 的 Suspend 走 interaction 卡路径，盘上 `goal_continuations` 未见 `pull_checkpoint` 持久化（两 run `agent_runtime_state.json` 全查零命中）；③轮次均死于 turn-1 响应边界，无 turn-2 `budget_state` 注入窗。故**账本聚合值的直接读数（probeSpent 披露行）本轮未取得**；已取得的物理计量=账本同源逐笔和（同单位 ms、同分级过滤），账本聚合语义由 PULL-PROBE-METER-1 单测背书。此缺口归因额度环境中断，非账本位形态问题。

## 4. mix_report 随轮断言（M1-5 挂账补证腿）

两 run 收尾各调一次 `/agent/invoke mix_report`（工件 `harness_ab_mix_report*.json`）：

- **invoke ok + schema_version=mix_report.v1**（两 run 均 ✓）；
- 盘上结论板：两 run 均查无 `free_state_conclusions.json`（判定链被额度截断，从未 settle）→ 走**诚实无记录分支**：消息含"最近 settle 结论：无记录（本工程自由态实验尚未收口…）"，queue 行 23 节点，双折叠一致（决策账本 0 条 = 自由态结论 0 条）；
- **有结论分支（latest_conclusion_id 非空+与盘上一致）未行使**——阻断面=额度环境中断（判定链无法收口），非断言面缺陷；断言基础设施（含盘上板一致性核对）已就位并跑通负分支。M1-5 正面真栈证据仍缺，与 M2-TIMEFX-1 额度挂账腿同源同 blockers。
- 脚本侧 `timeline_count` 字段存在 PS 空值计数伪影（记 1，实际 null）；以 `harness_ab_mix_report_response.json` 原文为准。

## 5. 取证腿：dev_agent_smoke 相对路径缺陷（确定性，已修复）

- 现象：三轮启栈腿内核 ~1.6s 干净退出（exit code 0）、零端口绑定、agent 首拨即拒；`shadow.initialized=False`。
- 手段（§8 两同断点止损后）：内核包装器（捕获退出码+继承环境回显）+ 300ms 外部进程/端口观察器（工件 `.kernel_probe/`）。
- 根因（部署目录内核日志原文）：journey berth 把 `VIT_PROJECT_XML` 以**相对路径**透传（`coord\runs\...`），内核按自身 CWD（run 目录 `kernel_workspace`）解析成**双段不存在路径** → `VitPaths: failed to create default project XML` → `project XML not found` → `service startup failed` → 干净退出。包装器回显 `env VIT_PROJECT_XML=coord\runs\...` 钉死输入面。
- G3-ATTRIB-1（10-09）同调用形态跑通的机制：其部署日志 healthy——该轮 Start-Process 相对 WorkingDirectory 未被 ShellExecute 按预期解析，内核继承了 PS 进程 CWD（工作树根），相对 env 恰好命中真文件=**偶然躲过**，非当时路径形态正确。
- 修复（5b55e064）：`-RunArtifactsDir` 非 IsPathRooted 时 GetFullPath 归一（默认分支 Join-Path $RepoRoot 本就绝对，不受影响）；修复后两 run 一次通过。**建议主管追认该脚本修复**（影响所有 journey 类 berth 调用者）。

## 6. 覆盖边界与诚实声明

1. 两 run 均为额度截断轮：LLM 面（口径①②）无有效双模式对比——本报告不把任何被截断读数当对比结论；
2. 口径③（probe 物理成本）证据链完整：原始捕获（留盘）→ analysis.json（入库）→ 本表；墙钟含调度噪音已声明；
3. f1/f2 族轮的 outcome 分类（judgment_failed）全部为额度截断伪象，不进入话术族等价性讨论（G3-ATTRIB-1 结论不受影响）；
4. mix_report 正面分支未行使（§4）；pull 账本披露窗未开（§3）——两者同源于额度，建议随额度恢复一并补；
5. 重工件（raw 捕获 jsonl 7MB/run、工程存储 9.2MB/run、stems、kernel_workspace）按 2026-09-29 政策留盘不入库，可回指本机 worktree；
6. 渲染面零改动；chat 直连管线未迁移（OQ-H3 不变）；C++ 面零改动。

## 7. 给主管的重开判定输入（建议，非裁定）

1. **条件①的物理成本腿可以收口**：probe 物理成本 push/pull 对称（886 vs 950ms/次，n=6/组，调度噪音带内）——**pull 不存在"probe 物理成本更低"的反转空间**；G3 成本门槛的实质变量仍在 LLM 面（轮级调用数+注入），而该面今晚无有效新证据，G3-ATTRIB-1 的 17 vs 6 仍是现行基线。
2. **若要完成条件①原定全貌**（LLM 面健康链对比），需额度恢复后复跑一轮同构对比（脚本已就绪：绝对路径修复+响应捕获+mix_report 腿全在 port 分支）——建议立小卡（~0.25h，占用与 M2-TIMEFX-2 额度挂账腿可并窗）或并入该卡复跑窗。
3. **随轮顺带产出**：①dev_agent_smoke 相对路径硬化（请追认）；②M1-5 有结论分支真栈证据仍缺（额度阻断面，非断言面缺陷）。
4. 判定归主管：本卡不自行切换 harness 缺省，不据截断读数建议任何方向。
