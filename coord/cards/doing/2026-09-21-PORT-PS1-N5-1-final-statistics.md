# PORT-PS1-N5-1：PC ④⑤ 收官统计（flash 口径 N=5，镜像 MAC-4）

- 优先级 / 预估 / 依赖：P1 / 0.5 天 / 无（断言已双端同步；响应 mac 跨端对照请求 3；**协议与 mac PORT-WL-1 复跑段对齐 2026-09-21**——达标线/族谱/账目/flash 口径一致，结果可 1:1 拼三列对照表）
- 模型分级：L1 / GLM-5.3 flash 可接（复跑统计卡：驱动+记账，断言零改动）
- **背景**：MAC-4 mac 侧 ④ 2/5、⑤ 0/5（flash）不达标上交；PC 决策侧历史清点（[reports/2026-09-21-pc-45-reply.md](../../reports/2026-09-21-pc-45-reply.md)）显示 PC ⑤ vocal focus 09-20 后 7/7 提案面停车——本卡把 PC 侧数字补成 MAC-4 同口径统计，一次测净"引擎差（已排除，双端 flash）/双端差"两个问题
- 目标：
  1. **④ `run_mix_single_tick_e2e.ps1` ×5**、**⑤ `run_vit_product_path_smoke.ps1` ×5**，全 flash 口径（config 不动），断言零改动（SYNC-3 双跳断言即被测契约）
  2. 达标线镜像 MAC-4：**≥4/5 绿为达标**；<4/5 逐轮定性上交。**终态族谱与 mac WL-1/MAC-4 统一（对齐 2026-09-21，供跨端 1:1 对照）**——正路径=**提案面停车**（needs_confirmation/improvement_proposal 面）；失败族五形态：**F1 能力边界声明**（done+边界措辞）/ **F2 中间态标记**（settle 窗内未收敛的进行时）/ **F3 无可采纳终态**（strengthened retry 后 failed）/ **F4 done 不停车**（散文/完整提案文本但 stop_reason=done）/ **F5 复合与环境**（白名单类机器复合、文件锁竞态等，单独列不算失败族）
  3. RUN_LEDGER 式逐轮账目（run ID/退出码/终态+族谱标记/环境事件），工件新目录不覆盖；**交付三列对照表骨架**：本卡 PC-N5（flash+白名单在位）vs MAC-4 基线（flash+无白名单，mac）vs WL-1 复跑（flash+白名单，mac——第三列由 mac 回执拼入），族谱标签同上一致
- §8 预声明：两件各 5 轮固定（统计卡不加跑；轮内脚本缺陷修复后该轮作废重跑须注明）；环境中断带证据排除；LLM key 零入工件
- 文件域：**零代码改动预期**（若轮内暴露脚本缺陷，最小修复走独立 commit 并在回执列明）；SMOKE_TESTS.md 如需补 N=5 统计口径一行
- 验收标准：①10 轮 RUN_LEDGER（逐轮 run ID/退出码/终态）；②两件达标判定（≥4/5）与不达标轮族谱定性；③§8 账目合规；④工件齐备
- 停止条件：连续环境故障（端口栈/文件锁）≥3 次暂停上交；发现需改断言/agent 才能绿 → 域外上报（本卡是统计卡不是修复卡）
- 领取：2026-09-21 10:08 / PC 执行侧（GLM-5.3） / origin/main=117e9307f0cacfd2246e75442951ec08af912e8d / 分支 port/ps1-n5-1 / 领取时工作树：M VitApp/Workspace/default_project.xml（烟测已知保留项）、?? coord/runs/ + extension/ 构建产物 + godot-cpp/（均域外不动）；引擎口径已核：~/.vit/config.json defaultModel=deepseek-v4-flash（不动）；~/.vit/free_state_experiment_plugins.json v5 在位（本卡"白名单在位"列前提实证）
- 回执：
- 验收：
