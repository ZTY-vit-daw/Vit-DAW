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
- 回执：
- 验收：
