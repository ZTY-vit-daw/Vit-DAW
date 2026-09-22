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
- **mac 动作**：Go 侧拉 main 即生效。~~建议移植脚本兜底~~ **已移植（2026-09-22 晚，mac 提交 bbd0dbf2 经 PC 决策侧验收 ff 入 main）**：④⑤ 两脚本 SETTLE_PY 兜底（needs_clarification+无问句特征→runtime status pending reply，marker `clarify_reply_source` 双端同名）。验证口径：⑤ vocal clarify 段 1 轮（reply 含问句原文）——归入 §二.3 复测。

### 3. FIX-F5-SNAPSHOT-FRESHNESS — 分支 `fix/f5-snapshot-freshness` @ 4e4d850d（main 侧 eeb589ac，已入 main）
- **语义**：声学桥快照前台铃标注+原子发布——落盘单点 `writeMixboardFeatureSnapshotFile` 同笔写入标注（current / stale+superseded_by_request / material_reuse）+tmp+rename；断言对象从"不存在分叉"改"不存在**未标注**分叉"+有界重读 600ms×2。
- **mac 动作（必需）**：~~:770 旧严格语义~~ **已移植（2026-09-22 晚，bbd0dbf2）**：`row_fork_request_id` 谓词与 PC `Test-BridgeSnapshotRowFork` 同款——未标注分叉 fail（消息形态同 PC）/标注分叉 warn 披露。**边界（决策侧复核接受）**：有界重读未移植——mac 调用点为内存/HTTP 投影快照无文件路径，与 PC 响应投影场景同位（PC 该场景同样无重读源、未标注即 Fail 视为永久披露）；若日后 mac ⑤ 增加文件路径断言再随之移植。验证口径归入 §二.3 复测（含声学桥双写窗观察）。

### 4. FIX-F3-G4-SEMANTICS — 分支 `fix/f3-g4-semantics` @ 31d35f8d(乙)+51686265(甲)（main 侧 6d799909+58469f23，已入 main，分列可回滚）
- **语义（乙）**：锁死终局轮完整提案仅被证据完备类门（G3-G6）拒时→停车提案确认面+open_dimensions 披露由用户裁决（防滥用 2/3 不动，停车≠重入不计弹回）；fallback 措辞分裂新 stop reason `free_state_terminal_turn_gate_rejected`（旧 `free_state_terminal_turn_unparseable` 保留给真不可解析形态）。
- **语义（甲，用户裁定 2026-09-22「支持改呈或逻辑」）**：G4 对 improvement 类提案对齐脊柱 FS5 守卫 OR 三析取——已闭合维度 ∨ 可用交付扫描收据(G3) ∨ 台账目标级证据(G6)；diagnosticOnly 全量严格 G4；**门 ID/condition 词表零变动**（`no_closed_diagnostic_dimension` 逐字未动，仅 gap Binding 扩写）。裁定档=[2026-09-22-f3-plan-a-g4-or-semantics.md](../decisions/2026-09-22-f3-plan-a-g4-or-semantics.md)。
- **mac 动作**：纯共享 Go 代码，拉 main 即生效。**词表双端一致性核查结论（2026-09-22 决策侧 grep 实证）**：mac 两脚本（run_mix_single_tick_e2e_mac.sh / run_vit_product_path_smoke_mac.sh）仅消费 `needs_clarification`（未被触碰）；`terminal_turn_unparseable`/`gate_rejected`/门 condition 词表 mac 侧零消费——**新 stop reason 为纯新增，无 mac 词表同步义务**。验证口径：`go test ./internal/agentloop/... ./internal/chat/... -count=1`（M06/乙夹具同步随代码走）+ ④ 1 轮（提案面双跳）。

## 二、建议执行顺序

1. （2026-09-22 晚更新）FOLD-1 已验收合入 main——mac **拉一次 main 即五补丁全齐**（F2/F5/F3乙甲/GATE-FRESHNESS/FOLD）+ mac 脚本移植（bbd0dbf2）也在 main，先跑 `go test ./... -count=1` 确认并树绿（PC 决策侧已复跑 84 包 ok）；
2. ~~脚本移植~~ **已完成（bbd0dbf2，PC 决策侧 diff 审查+bash -n 复核通过 ff 入 main）**：§一.2 F2 settle 兜底 + §一.3 F5 断言语义；
3. ~~mac ④⑤ 真栈复测~~ **已完成（2026-09-23 回执，决策侧验收通过）**：④ **2/5→4/4**（红轮族全吸收，R4 环境批另计）；⑤ **3/5 持平但族迁移**（F3/F5复合清零；clarify 链失败×1+F4×1 新形态）；F2 修复直供实证（停车轮 reply 天然含问句、兜底零替换）；F5 断言实跑零误报零漏报。**后续裁定（2026-09-23）**：a) 新形态 clarify 链失败=开卡 **FORENSIC-MAC-CLARIFY-CHAIN-1**（todo/，mac 侧只读取证，P1——F2 修复面邻域回归优先排除）；b) pro 升线**资格成立但排在取证卡后**（R3 若判基础设施缺陷→先修复复测再测引擎差，判别力才干净；若判模型方差→pro 直接上）；c) ④ R4 chat 传输失败并入 mac 环境已知项；d) contextruntime /var 符号链接测试失败=开卡 **FIX-TEST-CTXSYMLINK-1**（todo/，P3 测试侧最小修）。数字详证见 [2026-09-23-mac-followup-f5f2-receipt.md](2026-09-23-mac-followup-f5f2-receipt.md)。

## 三、附带事项

- **PORT-WL-1（已验收 pass，2026-09-22 晚；第三列已由 mac 自行拼入 N5-1 表）**：假说判定=放大器成立（停车 0/5→4/5、⑤ 与 PC 拉平 3/5=3/5）。验收四裁定：⑤ F2 残差**不开新卡**——由本包 §一.2 的 F2 修复（已在 main）+ settle 兜底移植吸收（该项由"建议"**升级为必做**，mac ④ 残差主族 F2×2 即此），复测仍现族再开卡；pro 对照**暂缓一步**（先吸收五补丁+脚本移植后复测，有残差再升一线）；static_eq/broadband 补齐=开卡 **PORT-WL-EQ-1**（todo/，mac 侧）；LLM 60s 超时+CoreAudio 卡顿=登记 mac 环境已知项。**mac 下一步顺序：拉 main（29efd6fa 或其后）→ §二 脚本移植 → ④⑤ 复测（对照 WL-1 基线出修复前后差）**。
- **N5-1 勘误（2026-09-22 已落卡面）**：④ R4 的 F4 归因=「G 门滞后假弹回前缀+模型终态选择复合」非纯模型方差——WL-1 列的 ④ F4×1（R5）同样按此口径读取（mac 基线 296ad46 在 GATE-FRESHNESS 修复前）；数字结论（PC ④ 3/5、⑤ 3/5；WL-1 ④ 2/5、⑤ 3/5）不变。
- **FIX-GD-TELEMETRY-BELL-1（todo/，新开卡）**：Godot 前端第三写者标注补齐——Godot 工程经 PORT-B4B 移植 mac，该卡执行时 mac 前端仓需同步（附注在卡内）。
- **FIX-FRONTIER-FOLD-1（已验收合入 main，2026-09-22 晚增补）**：折叠确定性修复 + GATE-FRESHNESS 合入，随 main 一次性到达 mac——
  - 语义：①G 门 G5/G6/G8 空前沿/空选择新鲜域兜底（FIX-GATE-FRESHNESS-1，8653a25a 经 056b6d88 甲后重放，与 F3 甲 OR 语义共存证明在卡回执）；②折叠生成/选择不对称修复（3cd08f21）：轨道级 usable 观察（target_ref.kind=track）结论行无内嵌轨道 ID 时仍折候选（轨道=观察 target，派生 ID 式零变动）——"证据已入账=事实已登记"。
  - mac 动作：均为共享 Go 代码，拉 main 即生效；**无脚本跟随项**（词表零变动、mac 脚本零消费，2026-09-22 grep 复核）。验证口径：`go test ./internal/agentloop/... ./internal/chat/... -count=1`（含 free_state_gate_freshness_test.go 共存红测试组随代码走）+ ④⑤ 各 1 轮（存在性）。
  - 交互注记：FOLD 修后"轨道级观察→候选折叠→提案准入"链路对齐，mac ⑤ 若曾见 gap=[G5,G6,G8] 形态，拉 main 后应显著减少（PC 真栈 20260922_182036→204302 前后对照在案）。

## 四、PC 侧状态（2026-09-22 晚 WL-1 验收后更新）

- main = e359260c + F2(b184e5d9) + F5(eeb589ac) + F3乙(6d799909) + F3甲(58469f23) + 验收/勘误 coord 提交 + FOLD-1 merge cac20769（含 8653a25a GATE-FRESHNESS 与 3cd08f21 折叠修复）+ mac 侧 WL-1 回执/拼表（a42ea04e/29efd6fa）+ WL-1 验收提交；五补丁并树决策侧复跑 agent 84 包 ok/0 FAIL + webui 324/324。
- 六卡验收 pass 全归档 done/（F2/F5/F3乙甲/GATE-FRESHNESS-1/FOLD-1/WL-1）；GATE-FRESHNESS-1 合并后 ⑤ 复核由 FOLD-1 轮 ⑤ exit 0 满足。todo/ 待领：FIX-GD-TELEMETRY-BELL-1（Godot 仓）、PORT-WL-EQ-1（mac 侧）。
