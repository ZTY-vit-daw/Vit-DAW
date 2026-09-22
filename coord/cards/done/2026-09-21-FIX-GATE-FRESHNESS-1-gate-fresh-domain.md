# FIX-GATE-FRESHNESS-1：G 门新鲜域对齐——G6/G8（+G5）切片内观测假性弹回修复

- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 取证依据 PORT-PS1-N5-1 ④ R4（run4_R4_forensic_timeline.json）+ mac 侧同前缀核查一致（用户转述 2026-09-21）
- 模型分级：L2 / GLM-5.3（准入门核心路径，纯函数修复+红测试先行）
- **背景（双端取证一致）**：G 门两族读域不对称——G3/G4/G7 读循环内即时更新的观测台账（新鲜域），G5/G6/G8 读 `minimal_audio_closure` 快照（仅切片边界折叠时更新）。当目标级观测与提案同落一个切片，G6/G8 读到折叠前快照 → 假性 `no_selected_candidate` 弹回（R4 实证：模型两轮观测后提案被假弹，二次弹回语义反转，最终散文退化 F4）。防滥用计数器（新鲜域）被假弹回消耗，两次假弹+同指纹重提可致"模型全对被锁死"。
- 目标：
  1. **红测试先行**：①R4 复现——同切片观测+提案，G6/G8 假性失败（修前红/修后绿）；②G5 场景——同切片扫描+提案候选未折叠；③假弹回后重发不烧预算/不触终局锁
  2. **最小修复（方案 A）**：G6——`selected==""` 时 allowedTracks 取全体候选轨道并应用既有 `messageLoopFreeStateCandidateTargetObserved` 新鲜检查（progression 检查已示范的 OR 模式，free_state_reasoning.go:1392）；G8——断言 1 同款兜底（`selected==""` 且目标 ∈ 某候选轨道且新鲜观测指向同轨）；G5——快照候选为空时回退查新鲜域合格扫描收据。选择非空时行为全部不变（真实偏离候选的拒绝语义保留）
  3. 不动：G1/G2/G4/G7、content-blind 红线（advisory #5）、防滥用规则 2/3、相位机、双端弹回词表
- §8 预声明：红测试构造三个场景各 1 次跑通即验收；真栈 ④⑤ 各 1 轮 exit 0（存在性证明口径，§7）；同断点两败止损上交
- 文件域：`agent/internal/agentloop/free_state_gate.go` + 新测试文件 `free_state_gate_freshness_test.go`（如 G5 需要则同文件）；零脚本/断言改动；mac 跟随更新由用户转交
- 验收标准：①红测试三场景修前红/修后绿；②现有 free_state_gate 测试组全绿 + G0 健康检查五条 + webui 测试；③④⑤ 各 1 轮真栈 exit 0；④回执列 diff 与测试证据
- 停止条件：修复引入现有 gate 测试语义冲突（非本卡目标的行为变化）→ 暂停上交；需动相位机/防滥用才能绿 → 域外上报
- 领取：2026-09-21 / PC 执行侧（GLM-5.3） / origin/main=03dba626（领取前）/ 分支 fix/gate-freshness-1 / 用户已裁定方案 A、PC 先修 mac 跟随（2026-09-21 会话）
- 回执：2026-09-21 20:25 / PC 执行侧（GLM-5.3） / 分支 fix/gate-freshness-1 已推（8653a25a，4 文件 +244/-17）；coord 开卡 ea76cfa3。**结果：验收①②全绿；③ ④ 1/1 exit 0、⑤ 3 轮均红但红点全部在修复域外（vocal focus 提案面路径 3/3 绿）——⑤ "1 轮 exit 0" 未达，同断点两败止损上交，卡留 doing 待决策**。
  - 验收①（红测试三场景，`free_state_gate_freshness_test.go`）：R4 复现测试修前 `failed=[G6_target_evidence G8_target_consistency]` **与生产 gap 逐字一致**、修后绿；G5 场景（空前沿+新鲜扫描/目标收据）修前红修后绿；真拒绝保留×2（选定候选下偏轨目标 G8 仍拒、选定候选外观测 G6 仍拒）+ 预算保护（门不再支持的弹回不烧 2 棒计数）全绿。
  - 实现摘要：G6 删 `selected==""` 早退，空选择取全体候选轨道并叠加新 helper `messageLoopFreshTrackObservationTarget`（读 state.recentObservation 新鲜域，镜像 progression 检查的 targetObservedNow 救援）；G8 断言 1 同款兜底（空前沿且提案目标=新鲜观测轨→frontier 一致）；G5 空前沿回退查新鲜域合格扫描收据（G3 循环体提取为 `freeStateLedgerHasUsableScan` 共享）；选择非空时三门行为不变。零接触：G1/G2/G4/G7、content-blind 红线、防滥用规则 2/3、相位机、弹回词表。
  - **夹具更新两处（语义变化=卡面目标行为，供决策复核）**：`gateVariantNoFrontier` 补删扫描收据（旧夹具在新语义下就是 R4 滞后窗本尊）；`no_selected_candidate`→`dangling_selected_candidate`（旧"空选择即拒"正是本修复要放行的 R4 场景，保留的拒绝形态改为选择悬空引用）；refusal-reference 范围控制夹具同款补删。
  - 验收②（回归）：`go test ./... -count=1` exit 0（agent 全量，G0 五条所属包全含）+ webui `npm run test` exit 0。
  - 验收③（真栈，日志 coord/runs/FIX-GATE-FRESHNESS-1/ 不提交）：**④ 1/1 exit 0**——提案面→工具面（Track 1007 -0.50dB stored）→d1 终态回读 0→-0.5dB 全链（run1_mix_single_tick_fix.log）。**⑤ 3 轮均 exit 1，止损**：R1 红在 clarify-ask F2 中间态（「已完成 ccb_observation_request」，统计卡 ④R1 同形态）；R2 红在声学桥 request_id 竞态（F5，统计卡 ⑤R4 同形态）；R3 红在 clarify-ask goal=failed（F3，SYNC-3 与统计卡 ⑤R3 的 agent 侧取证对象复发）。R1/R3 同断点两败 → §8 止损。**三红全部为修复前已存在的已知族（修复只动 G 门读路径，三族在 PORT-PS1-N5-1 统计中均已入账）；vocal focus 阶段（走修复的提案准入路径）3/3 停提案面绿**。
  - §8 账目：④ 1 轮+⑤ 3 轮（第 3 轮触发同断点两败止损线，停止投入如实上报）；环境中断 0（各轮栈自清/轮后清理）；LLM key 零入工件；测试 HEAD=8653a25a（烟测二进制含本修复，构建于运行前）。
  - 上交决策：a) ⑤ exit-0 验收是否由"④ 全链+⑤ vocal focus 3/3+单测 R4 复现"替代认定（三红与修复域无交集的证据已列）；b) clarify-ask goal=failed 已三次复发（SYNC-3、统计卡、本卡），建议独立 agent 侧取证卡；c) mac 跟随更新（同一 diff 双端同步语义，无词表变更）。
- 验收：
