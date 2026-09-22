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
- 回执：2026-09-21 11:05 / PC 执行侧（GLM-5.3） / 分支 port/ps1-n5-1 已推（59ccc8e1，SMOKE_TESTS.md 统计口径两行——卡面"如需补一行"预期内文档补充，非缺陷修复）；coord 领取 7b8bb1cc 已推 main。**结果：④ 3/5 ✗、⑤ 3/5 ✗——两件均不达标（<4/5），逐轮族谱如下；但 vocal focus 提案面停车率 4/4（PC 历史口径 7/7 → 11/11），放大器假说标的在 PC 侧保持 100%，⑤ 的红轮均非 vocal focus 停车失败**。
  - **RUN_LEDGER ④ run_mix_single_tick_e2e.ps1 ×5**（run ID=observe conv 时间戳；console 日志与判族证据 coord/runs/PORT-PS1-N5-1/，不提交）：
    - R1 `20260921_101048` exit 1 **F2 中间态**——observe 绿（提案面）→vocal 守卫 settle 后 waiting_clarification 但 reply=「已完成 ccb_observation_request」无"哪条轨"问句（run1 log）。
    - R2 `20260921_101536` exit 0 **正路径**——提案面→hop-1 工具面（stored Track 1010 -0.50dB）→hop-2 d1_post_action_evaluation_required（回读 0→-0.5dB）。
    - R3 `20260921_101925` exit 0 **正路径**——同上双跳全链（Track 1007 -0.50dB）。
    - R4 `20260921_102317` exit 1 **F4 done 不停车**——observe 完整提案散文「建议候选是把 Track 2 降低 1 dB，等你在确认环节点头后再动…不涉及任何插件」但 stop_reason=done/needs_confirmation=false（run4 log+run4_..._seq12_evidence.json，目标终态应为提案面）。**【勘误 2026-09-22】本轮 F4 带 G 门滞后假弹回前缀**：取证时间线（run4_R4_forensic_timeline.json，FIX-GATE-FRESHNESS-1 卡背景引证）证明模型两轮观测后提案被 G6/G8 读折叠前快照假弹（R4 滞后窗），二次弹回语义反转后才散文退化 F4——归因=基础设施前缀+模型终态选择复合，非纯模型终态方差；修复=FIX-GATE-FRESHNESS-1（8653a25a，G5/G6/G8 空前沿兜底，随 FIX-FRONTIER-FOLD-1 合入 main）。
    - R5 `20260921_102626` exit 0 **正路径**——同上双跳全链（Track 1007 -0.50dB）。
    - **判定：3/5 绿，不达标**。红轮 F2×1+F4×1；F2=呈现路径缺陷（FIX-F2-SURFACE-REPLY 已修，模型零责），F4=门滞后假弹回前缀+模型终态选择复合（见 R4 行勘误）——两红均非纯模型方差，mac 拼表时 ④ F4 行请按"复合（门滞后前缀）"口径读取；无环境类、无同断点两败。
  - **RUN_LEDGER ⑤ run_vit_product_path_smoke.ps1 ×5**（run ID=工件目录 product_path_*；工件在 VitApp/Workspace/Artifacts/smoke/ 各轮新目录不覆盖）：
    - R1 `product_path_20260921_103115` exit 0 **正路径**——vocal focus settle→needs_confirmation 提案面（presence 频段提升提案）。
    - R2 `product_path_20260921_103431` exit 0 **正路径**——提案面（Lead Vocal presence 提升+A/B 提案）。
    - R3 `product_path_20260921_103919` exit 1 **F3 无可采纳终态**——vocal focus 本身绿（提案面）；红点=clarify ask durable continuation 链 7 轮后 goal=failed（agent_log_tail status=failed stop=failed；chat_vocal_clarify_ask.json stop=limit_reached reply=回合失败）。**注：SYNC-3 观察过的 goal=failed 形态复发（当时 ruling 列'未复发待观察'），如需取证属 agent 侧另开卡**。
    - R4 `product_path_20260921_104355` exit 1 **F5 复合与环境（单列不算失败族，红轮照计——镜像 MAC-4 白名单 F5 轮计入 0/5 口径）**——未及 vocal focus；Chinese multitrack observe 声学桥断言：band_energy_summary.request_id=mixboard_* vs latest_request=kernel_prepared_spectral_field_1016（kernel/mixboard 双备谱路径瞬态竞态；工件 authoritative 快照终态一致 ready，非模型终态族）。
    - R5 `product_path_20260921_104638` exit 0（pass line 锚定：passed+Summary+栈自清；该轮 shell 包装未挂 exit 捕获，日志终局无异常块）**正路径**——提案面（track 1007 presence band 提案）。
    - **判定：3/5 绿，不达标**。**vocal focus 停车率 4/4**（R1/R2/R3/R5 凡跑到该阶段全停提案面；R4 未及）→ PC 口径 7/7 延伸 11/11。
  - **三列对照表骨架**（行=族谱标签；MAC-4 列取自 mac 通报 §二，④ 的 3 红轮与 ⑤ 的 5 红轮各 1 轮族谱通报未点名、标 ? 待 mac RUN_LEDGER 拼入；**WL-1 列已由 mac 执行流 2026-09-22 拼入**——统计批 10 轮全 flash 遥测实证，run 级证据 `~/Documents/vit-wl1-artifacts/RUN_LEDGER.md`，假说判定=放大器成立：vocal focus 停车率 0/5→4/5、⑤ 通过率与 PC-N5 拉平 3/5=3/5；WL-1 批基线=领取时 HEAD 296ad46（agent 代码与 MAC-4 同树，白名单为唯一功能差量）——对照口径干净）：

    | 族谱标签 | PC-N5（flash+白名单在位） | MAC-4（flash+无白名单） | WL-1（flash+白名单，mac） |
    |---|---|---|---|
    | ④ 通过率 | **3/5** | 2/5 | **2/5** |
    | ④ 正路径 | 3 | 2 | **2** |
    | ④ F1 能力边界 | 0 | 2 | **0** |
    | ④ F2 中间态 | 1 | ? | **2**（observe settle 未收敛） |
    | ④ F3 无可采纳 | 0 | ? | **0** |
    | ④ F4 done 不停车 | 1（勘误：带门滞后假弹回前缀，复合归因） | 0（⑦ 传导 1 例非 ④⑤） | **1**（vocal 守卫轮 done） |
    | ④ F5 复合（单列） | 0 | 0 | **0**（环境批另计：CoreAudio×3+LLM>60s×5，见 WL-1 RUN_LEDGER §二） |
    | ⑤ 通过率 | **3/5** | 0/5 | **3/5** |
    | ⑤ 正路径（vocal focus 停车） | 3（停车率 4/4） | 0 | **3（停车率 4/5，R4 一轮 F2）** |
    | ⑤ F1 能力边界 | 0 | 0 | **0** |
    | ⑤ F2 中间态 | 0 | ≥1 | **2**（R3 clarify ask+R4 vocal focus） |
    | ⑤ F3 无可采纳 | 1（clarify ask 子回合） | 2 | **0** |
    | ⑤ F4 done 不停车 | 0 | 0 | **0** |
    | ⑤ F5 复合（单列） | 1（声学桥 request_id 竞态） | 1（白名单缺失） | **0** |

  - §8 账目：两件各 5 轮固定无加跑；轮内作废重跑 0（零脚本缺陷修复）；环境中断 0 轮（轮间 taskkill 清 ④ 栈残留×5 均为轮后处置，不影响退出码；⑤ 各轮栈自清）；连续环境故障 0（未触 ≥3 暂停线）；LLM key 零入工件（events JSON grep=0，工件均脚本生成 chat/tool JSON）；引擎=deepseek-v4-flash 全程（~/.vit/config.json 未动）。测试 HEAD=7b8bb1cc（=origin/main 117e9307 代码树，SMOKE_TESTS.md 行在 10 轮全完成后才写）。
  - 纪律记录：agent/、VitApp/、前端仓零触碰；default_project.xml 烟测改写未提交未还原（已知保留项）；断言零改动（双跳断言即被测契约，本卡 10 轮未改任何断言——R4 声学桥红也未改，如实记账）。
- **勘误（2026-09-22，决策会话补记，供 mac 拼三列对照表）**：④ R4 的 F4 归因由"模型终态选择类"增补为"**G 门滞后假弹回前缀+模型终态选择复合**"——取证=run4_R4_forensic_timeline.json（coord/runs/PORT-PS1-N5-1/）+ FIX-GATE-FRESHNESS-1 卡背景（模型两轮观测后提案被 G6/G8 假弹→二次弹回语义反转→散文退化 F4）；修复=FIX-GATE-FRESHNESS-1（G5/G6/G8 空前沿新鲜域兜底）。数字结论（④ 3/5、⑤ 3/5、vocal focus 4/4）不变，仅归因口径修正。另注：SMOKE_TESTS.md 统计口径两行在分支 port/ps1-n5-1（59ccc8e1）未落 main，如需落 main 另行裁定。
- 验收：
