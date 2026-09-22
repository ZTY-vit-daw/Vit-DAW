# mac 跟随转交包：GATE-FRESHNESS / F2 / F5 / F3 四实现（2026-09-22 决策会话组装）

> 交 mac 执行流使用。PC 侧验收与合入状态见文末；SHA 均为分支原始 commit（coord 记录锚点），main 侧为 cherry-pick 等价 commit（commit message 含卡 ID，可直接 `git log --grep` 定位）。

## 一、四实现清单（每项：diff 指针 + 一句语义 + mac 侧动作）

### 1. FIX-GATE-FRESHNESS-1 — 分支 `fix/gate-freshness-1` @ 8653a25a（4 文件 +244/-17）
- **语义**：G 门新鲜域对齐——G6/G8（+G5）空选择/空前沿时回退读循环内新鲜观测域（`messageLoopFreshTrackObservationTarget` / `freeStateLedgerHasUsableScan`），消同切片观测被折叠前快照假弹回；选择非空时行为不变；G1/G2/G4/G7、防滥用、弹回词表零触碰。
- **红测试**：`agent/internal/agentloop/free_state_gate_freshness_test.go`（R4 复现与生产 gap 逐字一致+G5 场景+真拒绝保留×2+预算保护）。夹具更新两处（no_frontier 补删扫描、no_selected→dangling——语义变化=卡面目标）。
- **⚠ 合入状态：未入 main**——随 FIX-FRONTIER-FOLD-1 item1 冲突重放合入（执行中，2026-09-22 晚）。**mac 等 FOLD-1 合入后拉 main，勿直接从分支取**（FOLD-1 会解甲后冲突并出两侧测试组共存证明）。
- **mac 动作**：纯共享 Go 代码，拉 main 即双端生效；mac 脚本无 G 门/terminal 词表消费（2026-09-22 grep 核查=0），**无强制脚本跟随项**。验证口径：`go test ./internal/agentloop/... -count=1` + ④ 1 轮（存在性）。

### 2. FIX-F2-SURFACE-REPLY — 分支 `fix/f2-surface-reply` @ 1177a9c0（main 侧 b184e5d9，已入 main）
- **语义**：clarify-park 终态事件承载问句——`goalHasLiveContinuationOwner` 谓词对齐（parked armed checkpoint 不算活切片归属）使 turn.completed(body=问句) 得以发射；pending payload/durable pending 补 `reply`；④⑤ 脚本兜底（needs_clarification 且合成 reply 无问句特征→读 runtime status pending reply；⑤ 兜底在 F5 分支）。
- **mac 动作**：Go 侧拉 main 即生效。**建议移植脚本兜底**：`run_vit_product_path_smoke_mac.sh` 的 settle reply 取行（:533 `reply_candidates[-1]`）与 ④ 同款存在"抓工具步标题"风险类——按 PC ps1 同款条件兜底（问句特征=U+003F/U+FF1F，mac 无 PS5.1 ANSI 陷阱）。验证口径：移植后 ⑤ vocal clarify 段 1 轮（reply 含问句原文）。

### 3. FIX-F5-SNAPSHOT-FRESHNESS — 分支 `fix/f5-snapshot-freshness` @ 4e4d850d（main 侧 eeb589ac，已入 main）
- **语义**：声学桥快照前台铃标注+原子发布——落盘单点 `writeMixboardFeatureSnapshotFile` 同笔写入标注（current / stale+superseded_by_request / material_reuse）+tmp+rename；断言对象从"不存在分叉"改"不存在**未标注**分叉"+有界重读 600ms×2。
- **mac 动作（必需）**：`run_vit_product_path_smoke_mac.sh:770` 仍是严格 `request_id mismatch → fail` 旧语义——**拉取 F5 标注代码后，有标注的 stale 分叉行会在 mac ⑤ 误红**。移植 PC ps1 同款：有 `freshness` 字段的分叉行降为披露（warn），无标注分叉行照 fail；可选 `-SnapshotPath` 有界重读同款。验证口径：移植后 ⑤ 1-2 轮 exit 0（含一次声学桥双写窗观察）。

### 4. FIX-F3-G4-SEMANTICS — 分支 `fix/f3-g4-semantics` @ 31d35f8d(乙)+51686265(甲)（main 侧 6d799909+58469f23，已入 main，分列可回滚）
- **语义（乙）**：锁死终局轮完整提案仅被证据完备类门（G3-G6）拒时→停车提案确认面+open_dimensions 披露由用户裁决（防滥用 2/3 不动，停车≠重入不计弹回）；fallback 措辞分裂新 stop reason `free_state_terminal_turn_gate_rejected`（旧 `free_state_terminal_turn_unparseable` 保留给真不可解析形态）。
- **语义（甲，用户裁定 2026-09-22「支持改呈或逻辑」）**：G4 对 improvement 类提案对齐脊柱 FS5 守卫 OR 三析取——已闭合维度 ∨ 可用交付扫描收据(G3) ∨ 台账目标级证据(G6)；diagnosticOnly 全量严格 G4；**门 ID/condition 词表零变动**（`no_closed_diagnostic_dimension` 逐字未动，仅 gap Binding 扩写）。裁定档=[2026-09-22-f3-plan-a-g4-or-semantics.md](../decisions/2026-09-22-f3-plan-a-g4-or-semantics.md)。
- **mac 动作**：纯共享 Go 代码，拉 main 即生效。**词表双端一致性核查结论（2026-09-22 决策侧 grep 实证）**：mac 两脚本（run_mix_single_tick_e2e_mac.sh / run_vit_product_path_smoke_mac.sh）仅消费 `needs_clarification`（未被触碰）；`terminal_turn_unparseable`/`gate_rejected`/门 condition 词表 mac 侧零消费——**新 stop reason 为纯新增，无 mac 词表同步义务**。验证口径：`go test ./internal/agentloop/... ./internal/chat/... -count=1`（M06/乙夹具同步随代码走）+ ④ 1 轮（提案面双跳）。

## 二、建议执行顺序

1. mac 先拉**当前 main**（F2/F5/F3 已在）：跑 §一.2/.3/.4 的 Go 测试与脚本移植；
2. 等 PC 侧 FOLD-1 合入推送后再拉一次（GATE-FRESHNESS 到位）：跑 §一.1；
3. 回归：mac ④⑤ 各 1 轮（存在性口径）+ `go test ./... -count=1`（mac 平台分支测试）。

## 三、附带事项

- **PORT-WL-1（doing/，mac 侧）**：回执回来后拼三列对照表第三列（N5-1 done 卡表格 WL-1 列）。
- **N5-1 勘误（2026-09-22 已落卡面）**：④ R4 的 F4 归因=「G 门滞后假弹回前缀+模型终态选择复合」非纯模型方差——mac 拼表 ④ F4 行按此口径；数字结论（PC ④ 3/5、⑤ 3/5）不变。
- **FIX-GD-TELEMETRY-BELL-1（todo/，新开卡）**：Godot 前端第三写者标注补齐——Godot 工程经 PORT-B4B 移植 mac，该卡执行时 mac 前端仓需同步（附注在卡内）。
- **FIX-FRONTIER-FOLD-1（doing/，PC 执行中）**：折叠确定性修复，回执后另行小转交。

## 四、PC 侧状态（本转交包组装时）

- main = e359260c + F2(b184e5d9) + F5(eeb589ac) + F3乙(6d799909) + F3甲(58469f23) + 本 coord 提交；共存树决策侧复跑 agent 84 包 ok/0 FAIL + webui 324/324。
- 四卡验收 pass 归档 done/（GATE-FRESHNESS-1 验收含 ⑤ exit-0 替代认定=成立）；8653a25a 合入归 FOLD-1。
